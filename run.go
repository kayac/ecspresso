package ecspresso

import (
	"context"
	"encoding/json"
	"errors"
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

	d.LogInfo("Running task")
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
	d.LogInfo("[DEBUG] Overrides")
	d.LogJSON(ov)

	tdForRun, err := d.resolveTaskDefinitionForRun(ctx, opt)
	if err != nil {
		return err
	}
	if tdForRun.Arn == "" && tdForRun.TaskDefinitionInput != nil {
		d.LogInfo("task definition will be registered", "family", aws.ToString(tdForRun.TaskDefinitionInput.Family))
	} else {
		d.LogInfo("task definition", "task_definition_arn", tdForRun.Arn)
		var err error
		td, err := d.DescribeTaskDefinition(ctx, tdForRun.Arn)
		if err != nil {
			return err
		}
		tdForRun.TaskDefinitionInput = td
	}
	watchContainer := containerOf(tdForRun.TaskDefinitionInput, &opt.WatchContainer)
	if watchContainer == nil {
		return fmt.Errorf("container %s not found in the task definition", opt.WatchContainer)
	}
	d.LogInfo("watch container", "container", aws.ToString(watchContainer.Name))

	if opt.DryRun {
		d.LogInfo("DRY RUN OK")
		return nil
	}

	task, err := d.RunTask(ctx, tdForRun.Arn, &ov, &opt)
	if err != nil {
		return err
	}
	if !opt.Wait {
		d.LogInfo("Run task invoked")
		return nil
	}
	if err := d.WaitRunTask(ctx, task, watchContainer, time.Now(), opt.waitUntilRunning()); err != nil {
		return err
	}
	if err := d.DescribeTaskStatus(ctx, task, watchContainer); err != nil {
		return err
	}
	d.LogInfo("Run task completed!")

	return nil
}

func (d *App) RunTask(ctx context.Context, tdArn string, ov *types.TaskOverride, opt *RunOption) (*types.Task, error) {
	d.LogInfo("running task", "task_definition", tdArn)

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
		d.LogInfo("[DEBUG] propagate tags from service", "service_arn", *sv.ServiceArn)
		d.LogJSON(out)
		in.Tags = append(in.Tags, out.Tags...)
	case "", "NONE":
		// XXX ECS says > InvalidParameterException: Invalid value for propagateTags
		// in.PropagateTags = types.PropagateTagsNone
		in.PropagateTags = ""
	default:
		in.PropagateTags = types.PropagateTagsTaskDefinition
	}
	d.LogInfo("[DEBUG] run task input")
	d.LogJSON(in)

	out, err := d.ecs.RunTask(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("failed to run task: %w", err)
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		if f.Arn != nil {
			d.LogInfo("task failure", "task_arn", *f.Arn)
		}
		return nil, fmt.Errorf("failed to run task: %s %s", aws.ToString(f.Reason), aws.ToString(f.Detail))
	}

	if len(out.Tasks) == 0 {
		return nil, fmt.Errorf("failed to run task: no tasks run")
	}
	task := out.Tasks[0]
	d.LogInfo("task started", "task_arn", aws.ToString(task.TaskArn))
	return &task, nil
}

func (d *App) WaitRunTask(ctx context.Context, task *types.Task, watchContainer *types.ContainerDefinition, startedAt time.Time, untilRunning bool) error {
	d.LogInfo("Waiting for run task...(it may take a while)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	lc := watchContainer.LogConfiguration
	if lc == nil || lc.LogDriver != types.LogDriverAwslogs || lc.Options["awslogs-stream-prefix"] == "" {
		d.LogInfo("awslogs not configured")
		if err := d.waitTask(ctx, task, untilRunning); err != nil {
			return err
		}
		return nil
	}

	d.LogInfo("watching container", "container", *watchContainer.Name)
	logGroup, logStream := d.GetLogInfo(task, watchContainer)
	sleepContext(ctx, 3*time.Second) // wait for log stream

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		var nextToken *string
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-ticker.C:
				token, err := d.GetLogEvents(waitCtx, logGroup, logStream, startedAt, nextToken)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return
					}
					if errors.Is(err, ErrPermissionDenied) {
						d.LogWarn("failed to get log events: check logs:GetLogEvents permission", "error", err.Error())
						return
					}
					if !errors.Is(err, ErrNotFound) {
						d.LogWarn("failed to get log events", "error", err.Error())
					}
					continue
				}
				nextToken = token
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
		d.LogInfo("waiting for task until running", "task_id", id)
		waiter := ecs.NewTasksRunningWaiter(d.ecs, func(o *ecs.TasksRunningWaiterOptions) {
			o.MaxDelay = waiterMaxDelay
		})
		if err := waiter.Wait(ctx, d.DescribeTasksInput(task), d.Timeout()); err != nil {
			return err
		}
		d.LogInfo("task is running", "task_id", id)
		return nil
	}

	d.LogInfo("waiting for task until stopped", "task_id", id)
	waiter := ecs.NewTasksStoppedWaiter(d.ecs, func(o *ecs.TasksStoppedWaiterOptions) {
		o.MaxDelay = waiterMaxDelay
	})
	if err := waiter.Wait(ctx, d.DescribeTasksInput(task), d.Timeout()); err != nil {
		return fmt.Errorf("failed to wait task: %w", err)
	}
	return nil
}

type taskDefinitionForRun struct {
	Arn                 string
	TaskDefinitionInput *TaskDefinitionInput
}

func (d *App) resolveTaskDefinitionForRun(ctx context.Context, opt RunOption) (*taskDefinitionForRun, error) {
	switch {
	case *opt.Revision > 0:
		if opt.LatestTaskDefinition {
			return nil, fmt.Errorf("revision and latest-task-definition are exclusive: %w", ErrConflictOptions)
		}
		family, _, err := d.resolveTaskdefinition(ctx)
		if err != nil {
			return nil, err
		}
		return &taskDefinitionForRun{Arn: fmt.Sprintf("%s:%d", family, *opt.Revision)}, nil
	case opt.LatestTaskDefinition:
		family, _, err := d.resolveTaskdefinition(ctx)
		if err != nil {
			return nil, err
		}
		d.LogInfo("revision not specified, using latest task definition", "family", family)
		latestTdArn, err := d.findLatestTaskDefinitionArn(ctx, family)
		if err != nil {
			return nil, err
		}
		return &taskDefinitionForRun{Arn: latestTdArn}, nil
	case opt.SkipTaskDefinition:
		family, rev, err := d.resolveTaskdefinition(ctx)
		if err != nil {
			return nil, err
		}
		if rev != "" {
			return &taskDefinitionForRun{Arn: fmt.Sprintf("%s:%s", family, rev)}, nil
		}
		d.LogInfo("revision not specified, using latest task definition", "family", family)
		latestTdArn, err := d.findLatestTaskDefinitionArn(ctx, family)
		if err != nil {
			return nil, err
		}
		return &taskDefinitionForRun{Arn: latestTdArn}, nil
	default:
		tdPath := opt.TaskDefinition
		if tdPath == "" {
			tdPath = d.config.TaskDefinitionPath
		}
		in, err := d.LoadTaskDefinition(tdPath)
		if err != nil {
			return nil, err
		}
		{
			b, _ := MarshalJSONForAPI(in)
			d.LogInfo("[DEBUG] task definition", "definition", string(b))
		}
		if opt.DryRun {
			return &taskDefinitionForRun{Arn: "", TaskDefinitionInput: in}, nil
		}
		newTd, err := d.RegisterTaskDefinition(ctx, in)
		if err != nil {
			return nil, err
		}
		return &taskDefinitionForRun{Arn: *newTd.TaskDefinitionArn}, nil
	}
}

func (d *App) resolveTaskdefinition(ctx context.Context) (family string, revision string, err error) {
	if d.config.Service != "" {
		d.LogInfo("[DEBUG] loading service")
		sv, err := d.DescribeService(ctx)
		if err != nil {
			return "", "", err
		}
		tdArn := aws.ToString(sv.TaskDefinition)
		p := strings.SplitN(arnToName(tdArn), ":", 2)
		if len(p) != 2 {
			return "", "", fmt.Errorf("invalid task definition arn: %s", tdArn)
		}
		return p[0], p[1], nil
	} else {
		d.LogInfo("[DEBUG] loading task definition")
		td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
		if err != nil {
			return "", "", err
		}
		family = *td.Family
		return family, "", nil
	}
}
