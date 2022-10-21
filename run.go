package ecspresso

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type RunOption struct {
	DryRun               *bool
	TaskDefinition       *string
	NoWait               *bool
	TaskOverrideStr      *string
	TaskOverrideFile     *string
	SkipTaskDefinition   *bool
	Count                *int32
	WatchContainer       *string
	LatestTaskDefinition *bool
	PropagateTags        *string
	Tags                 *string
	WaitUntil            *string
	Revision             *int64
}

func (opt RunOption) waitUntilRunning() bool {
	return aws.ToString(opt.WaitUntil) == "running"
}

func (opt RunOption) DryRunString() string {
	if *opt.DryRun {
		return ""
	}
	return ""
}

func (d *App) Run(ctx context.Context, opt RunOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	d.Log("Running task %s", opt.DryRunString())
	ov := types.TaskOverride{}
	if ovStr := aws.ToString(opt.TaskOverrideStr); ovStr != "" {
		if err := json.Unmarshal([]byte(ovStr), &ov); err != nil {
			return fmt.Errorf("invalid overrides: %w", err)
		}
	} else if ovFile := aws.ToString(opt.TaskOverrideFile); ovFile != "" {
		src, err := d.readDefinitionFile(ovFile)
		if err != nil {
			return fmt.Errorf("failed to read overrides-file %s: %w", ovFile, err)
		}
		if err := d.unmarshalJSON(src, &ov, ovFile); err != nil {
			return fmt.Errorf("failed to read overrides-file %s: %w", ovFile, err)
		}
	}
	d.Log("[DEBUG] Overrides: %v", ov)

	tdArn, watchContainer, err := d.taskDefinitionForRun(ctx, opt)
	if err != nil {
		return err
	}
	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	task, err := d.RunTask(ctx, tdArn, &ov, &opt)
	if err != nil {
		return err
	}
	if *opt.NoWait {
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

	tags, err := parseTags(*opt.Tags)
	if err != nil {
		return nil, fmt.Errorf("failed to run task. invalid tags: %w", err)
	}

	in := &ecs.RunTaskInput{
		Cluster:                  aws.String(d.Cluster),
		TaskDefinition:           aws.String(tdArn),
		NetworkConfiguration:     sv.NetworkConfiguration,
		LaunchType:               sv.LaunchType,
		Overrides:                ov,
		Count:                    opt.Count,
		CapacityProviderStrategy: sv.CapacityProviderStrategy,
		PlacementConstraints:     sv.PlacementConstraints,
		PlacementStrategy:        sv.PlacementStrategy,
		PlatformVersion:          sv.PlatformVersion,
		Tags:                     tags,
		EnableECSManagedTags:     sv.EnableECSManagedTags,
		EnableExecuteCommand:     sv.EnableExecuteCommand,
	}

	switch aws.ToString(opt.PropagateTags) {
	case "SERVICE":
		out, err := d.ecs.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: sv.ServiceArn,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list tags for service: %w", err)
		}
		d.Log("[DEBUG] propagate tags from service %s", *sv.ServiceArn, out)
		in.Tags = append(in.Tags, out.Tags...)
	case "":
		in.PropagateTags = types.PropagateTagsNone
	default:
		in.PropagateTags = types.PropagateTagsTaskDefinition
	}
	d.Log("[DEBUG] run task input %v", in)

	out, err := d.ecs.RunTask(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("failed to run task: %w", err)
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		if f.Arn != nil {
			d.Log("Task ARN: " + *f.Arn)
		}
		return nil, fmt.Errorf("failed to run task: %s %s", *f.Reason, *f.Detail)
	}

	task := out.Tasks[0]
	d.Log("Task ARN: %s", *task.TaskArn)
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
		waiter := ecs.NewTasksRunningWaiter(d.ecs)
		if err := waiter.Wait(ctx, d.DescribeTasksInput(task), d.Timeout()); err != nil {
			return err
		}
		d.Log("Task ID %s is running", id)
		return nil
	}

	d.Log("Waiting for task ID %s until stopped", id)
	waiter := ecs.NewTasksStoppedWaiter(d.ecs)
	if err := waiter.Wait(ctx, d.DescribeTasksInput(task), d.Timeout()); err != nil {
		return fmt.Errorf("failed to wait task: %w", err)
	}
	return nil
}

func (d *App) taskDefinitionForRun(ctx context.Context, opt RunOption) (tdArn string, watchContainer *types.ContainerDefinition, err error) {
	tdPath := aws.ToString(opt.TaskDefinition)
	if tdPath == "" {
		tdPath = d.config.TaskDefinitionPath
	}
	var td *TaskDefinitionInput
	td, err = d.LoadTaskDefinition(tdPath)
	if err != nil {
		return
	}
	family := *td.Family
	defer func() {
		if err != nil {
			return
		}
		watchContainer = containerOf(td, opt.WatchContainer)
		d.Log("Task definition ARN: %s", tdArn)
		d.Log("Watch container: %s", *watchContainer.Name)
	}()

	if *opt.LatestTaskDefinition {
		tdArn, err = d.findLatestTaskDefinitionArn(ctx, family)
		return
	} else if *opt.SkipTaskDefinition {
		if aws.ToInt64(opt.Revision) != 0 {
			tdArn = fmt.Sprintf("%s:%d", family, aws.ToInt64(opt.Revision))
			return
		}
		if d.config.Service != "" {
			if sv, _err := d.DescribeServiceStatus(ctx, 0); _err != nil {
				err = _err
			} else {
				tdArn = *sv.TaskDefinition
				td, err = d.DescribeTaskDefinition(ctx, tdArn)
			}
			return
		} else {
			d.Log("Revision is not specified. Use latest task definition")
			tdArn, err = d.findLatestTaskDefinitionArn(ctx, family)
			return
		}
	} else {
		// register
		if *opt.DryRun {
			err = nil
			return
		}
		if newTd, _err := d.RegisterTaskDefinition(ctx, td); _err != nil {
			err = _err
			return
		} else {
			tdArn = *newTd.TaskDefinitionArn
			td, err = d.DescribeTaskDefinition(ctx, tdArn)
			return
		}
	}
}
