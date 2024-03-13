package ecspresso

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type RunOption struct {
	DryRun                 bool    `help:"dry run" default:"false"`
	TaskDefinition         string  `name:"task-def" help:"task definition file for run task" default:""`
	Wait                   bool    `help:"wait for task to complete" default:"true" negatable:""`
	TaskOverrideStr        string  `name:"overrides" help:"task override JSON string" default:""`
	TaskOverrideFile       string  `name:"overrides-file" help:"task override JSON file path" default:""`
	SkipTaskDefinition     bool    `help:"skip register a new task definition" default:"false"`
	Count                  int32   `help:"number of tasks to run (max 10)" default:"1"`
	WatchContainer         string  `help:"container name for watching exit code" default:""`
	LatestTaskDefinition   bool    `help:"use the latest task definition without registering a new task definition" default:"false"`
	PropagateTags          string  `help:"propagate the tags for the task (SERVICE or TASK_DEFINITION)" default:""`
	Tags                   string  `help:"tags for the task: format is KeyFoo=ValueFoo,KeyBar=ValueBar" default:""`
	WaitUntil              string  `help:"wait until invoked tasks status reached to (running or stopped)" default:"stopped" enum:"running,stopped"`
	Revision               *int64  `help:"revision of the task definition to run when --skip-task-definition" default:"0"`
	ClientToken            *string `help:"unique token that identifies a request, useful for idempotency"`
	EBSDeleteOnTermination *bool   `help:"whether to delete the EBS volume when the task is stopped" default:"true" negatable:""`
}

func (opt RunOption) waitUntilRunning() bool {
	return opt.WaitUntil == "running"
}

func (opt RunOption) DryRunString() string {
	if opt.DryRun {
		return ""
	}
	return ""
}

func (d *App) Run(ctx context.Context, opt RunOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	d.Log("Running task %s", opt.DryRunString())
	ov := types.TaskOverride{}
	if opt.TaskOverrideStr != "" {
		if err := json.Unmarshal([]byte(opt.TaskOverrideStr), &ov); err != nil {
			return fmt.Errorf("invalid overrides: %w", err)
		}
	} else if ovFile := opt.TaskOverrideFile; ovFile != "" {
		src, err := d.readDefinitionFile(ovFile)
		if err != nil {
			return fmt.Errorf("failed to read overrides-file %s: %w", ovFile, err)
		}
		if err := unmarshalJSON(src, &ov, ovFile); err != nil {
			return fmt.Errorf("failed to read overrides-file %s: %w", ovFile, err)
		}
	}
	d.Log("[DEBUG] Overrides")
	d.LogJSON(ov)

	tdArn, err := d.taskDefinitionArnForRun(ctx, opt)
	if err != nil {
		return err
	}
	d.Log("Task definition ARN: %s", tdArn)
	if opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}
	td, err := d.DescribeTaskDefinition(ctx, tdArn)
	if err != nil {
		return err
	}
	watchContainer := containerOf(td, &opt.WatchContainer)
	d.Log("Watch container: %s", *watchContainer.Name)

	task, err := d.RunTask(ctx, tdArn, &ov, &opt)
	if err != nil {
		return err
	}
	if !opt.Wait {
		d.Log("Run task invoked")
		return nil
	}
	if err := d.WaitRunTask(ctx, task, watchContainer, time.Now(), opt.waitUntilRunning()); err != nil {
		return err
	}
	if err := d.DescribeTaskStatus(ctx, task, watchContainer); err != nil {
		return err
	}
	d.Log("Run task completed!")

	return nil
}

func (d *App) RunTask(ctx context.Context, tdArn string, ov *types.TaskOverride, opt *RunOption) (*types.Task, error) {
	d.Log("Running task with %s", tdArn)

	sv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
	if err != nil {
		return nil, err
	}

	tags, err := parseTags(opt.Tags)
	if err != nil {
		return nil, fmt.Errorf("failed to run task. invalid tags: %w", err)
	}

	in := &ecs.RunTaskInput{
		Cluster:                  aws.String(d.Cluster),
		TaskDefinition:           aws.String(tdArn),
		NetworkConfiguration:     sv.NetworkConfiguration,
		LaunchType:               sv.LaunchType,
		Overrides:                ov,
		Count:                    &opt.Count,
		CapacityProviderStrategy: sv.CapacityProviderStrategy,
		PlacementConstraints:     sv.PlacementConstraints,
		PlacementStrategy:        sv.PlacementStrategy,
		PlatformVersion:          sv.PlatformVersion,
		Tags:                     tags,
		EnableECSManagedTags:     sv.EnableECSManagedTags,
		EnableExecuteCommand:     sv.EnableExecuteCommand,
		ClientToken:              opt.ClientToken,
		VolumeConfigurations: serviceVolumeConfigurationsToTask(
			sv.VolumeConfigurations,
			opt.EBSDeleteOnTermination,
		),
	}

	switch opt.PropagateTags {
	case "SERVICE":
		out, err := d.ecs.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: sv.ServiceArn,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list tags for service: %w", err)
		}
		d.Log("[DEBUG] propagate tags from service %s", *sv.ServiceArn)
		d.LogJSON(out)
		in.Tags = append(in.Tags, out.Tags...)
	case "", "NONE":
		// XXX ECS says > InvalidParameterException: Invalid value for propagateTags
		// in.PropagateTags = types.PropagateTagsNone
		in.PropagateTags = ""
	default:
		in.PropagateTags = types.PropagateTagsTaskDefinition
	}
	d.Log("[DEBUG] run task input")
	d.LogJSON(in)

	out, err := d.ecs.RunTask(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("failed to run task: %w", err)
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		if f.Arn != nil {
			d.Log("Task ARN: %s", *f.Arn)
		}
		return nil, fmt.Errorf("failed to run task: %s %s", aws.ToString(f.Reason), aws.ToString(f.Detail))
	}

	if len(out.Tasks) == 0 {
		return nil, fmt.Errorf("failed to run task: no tasks run")
	}
	task := out.Tasks[0]
	d.Log("Task ARN: %s", aws.ToString(task.TaskArn))
	return &task, nil
}

func (d *App) WaitRunTask(ctx context.Context, task *types.Task, watchContainer *types.ContainerDefinition, startedAt time.Time, untilRunning bool) error {
	d.Log("Waiting for run task...(it may take a while)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	lc := watchContainer.LogConfiguration
	if lc == nil || lc.LogDriver != types.LogDriverAwslogs || lc.Options["awslogs-stream-prefix"] == "" {
		d.Log("awslogs not configured")
		if err := d.waitTask(ctx, task, untilRunning); err != nil {
			return err
		}
		return nil
	}

	d.Log("Watching container: %s", *watchContainer.Name)
	logGroup, logStream := d.GetLogInfo(task, watchContainer)
	time.Sleep(3 * time.Second) // wait for log stream

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		var nextToken *string
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-ticker.C:
				nextToken, _ = d.GetLogEvents(waitCtx, logGroup, logStream, startedAt, nextToken)
			}
		}
	}()

	if err := d.waitTask(ctx, task, untilRunning); err != nil {
		return err
	}
	return nil
}

func (d *App) waitTask(ctx context.Context, task *types.Task, untilRunning bool) error {
	id := arnToName(*task.TaskArn)
	if untilRunning {
		d.Log("Waiting for task ID %s until running", id)
		waiter := ecs.NewTasksRunningWaiter(d.ecs, func(o *ecs.TasksRunningWaiterOptions) {
			o.MaxDelay = waiterMaxDelay
		})
		if err := waiter.Wait(ctx, d.DescribeTasksInput(task), d.Timeout()); err != nil {
			return err
		}
		d.Log("Task ID %s is running", id)
		return nil
	}

	d.Log("Waiting for task ID %s until stopped", id)
	waiter := ecs.NewTasksStoppedWaiter(d.ecs, func(o *ecs.TasksStoppedWaiterOptions) {
		o.MaxDelay = waiterMaxDelay
	})
	if err := waiter.Wait(ctx, d.DescribeTasksInput(task), d.Timeout()); err != nil {
		return fmt.Errorf("failed to wait task: %w", err)
	}
	return nil
}

func (d *App) taskDefinitionArnForRun(ctx context.Context, opt RunOption) (string, error) {
	switch {
	case *opt.Revision > 0:
		if opt.LatestTaskDefinition {
			err := ErrConflictOptions("revision and latest-task-definition are exclusive")
			// TODO: v2.1 raise error
			d.Log("[WARNING] %s", err)
		}
		family, _, err := d.resolveTaskdefinition(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s:%d", family, *opt.Revision), nil
	case opt.LatestTaskDefinition:
		family, _, err := d.resolveTaskdefinition(ctx)
		if err != nil {
			return "", err
		}
		d.Log("Revision is not specified. Use latest task definition family" + family)
		latestTdArn, err := d.findLatestTaskDefinitionArn(ctx, family)
		if err != nil {
			return "", err
		}
		return latestTdArn, nil
	case opt.SkipTaskDefinition:
		family, rev, err := d.resolveTaskdefinition(ctx)
		if err != nil {
			return "", err
		}
		if rev != "" {
			return fmt.Sprintf("%s:%s", family, rev), nil
		}
		d.Log("Revision is not specified. Use latest task definition family" + family)
		latestTdArn, err := d.findLatestTaskDefinitionArn(ctx, family)
		if err != nil {
			return "", err
		}
		return latestTdArn, nil
	default:
		tdPath := opt.TaskDefinition
		if tdPath == "" {
			tdPath = d.config.TaskDefinitionPath
		}
		in, err := d.LoadTaskDefinition(tdPath)
		if err != nil {
			return "", err
		}
		{
			b, _ := MarshalJSONForAPI(in)
			d.Log("[DEBUG] task definition: %s", string(b))
		}
		if opt.DryRun {
			return fmt.Sprintf("family %s will be registered", *in.Family), nil
		}
		newTd, err := d.RegisterTaskDefinition(ctx, in)
		if err != nil {
			return "", err
		}
		return *newTd.TaskDefinitionArn, nil
	}
}

func (d *App) resolveTaskdefinition(ctx context.Context) (family string, revision string, err error) {
	if d.config.Service != "" {
		d.Log("[DEBUG] loading service")
		sv, err := d.DescribeService(ctx)
		if err != nil {
			return "", "", err
		}
		tdArn := *sv.TaskDefinition
		p := strings.SplitN(arnToName(tdArn), ":", 2)
		if len(p) != 2 {
			return "", "", fmt.Errorf("invalid task definition arn: %s", tdArn)
		}
		return p[0], p[1], nil
	} else {
		d.Log("[DEBUG] loading task definition")
		td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
		if err != nil {
			return "", "", err
		}
		family = *td.Family
		return family, "", nil
	}
}
