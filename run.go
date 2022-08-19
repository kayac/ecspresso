package ecspresso

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/pkg/errors"
)

func (d *App) Run(opt RunOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Running task", opt.DryRunString())
	ov := types.TaskOverride{}
	if ovStr := aws.ToString(opt.TaskOverrideStr); ovStr != "" {
		if err := json.Unmarshal([]byte(ovStr), &ov); err != nil {
			return errors.Wrap(err, "invalid overrides")
		}
	} else if ovFile := aws.ToString(opt.TaskOverrideFile); ovFile != "" {
		src, err := d.readDefinitionFile(ovFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read overrides-file %s", ovFile)
		}
		if err := d.unmarshalJSON(src, &ov, ovFile); err != nil {
			return errors.Wrapf(err, "failed to read overrides-file %s", ovFile)
		}
	}
	d.DebugLog("Overrides:", ov)

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
		return errors.Wrap(err, "failed to run task")
	}
	if *opt.NoWait {
		d.Log("Run task invoked")
		return nil
	}
	if err := d.WaitRunTask(ctx, task, watchContainer, time.Now(), opt.waitUntilRunning()); err != nil {
		return errors.Wrap(err, "failed to run task")
	}
	if err := d.DescribeTaskStatus(ctx, task, watchContainer); err != nil {
		return err
	}
	d.Log("Run task completed!")

	return nil
}

func (d *App) RunTask(ctx context.Context, tdArn string, ov *types.TaskOverride, opt *RunOption) (*types.Task, error) {
	d.Log("Running task with", tdArn)

	sv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
	if err != nil {
		return nil, err
	}

	tags, err := parseTags(*opt.Tags)
	if err != nil {
		return nil, err
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
		out, err := d.ecsv2.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: sv.ServiceArn,
		})
		if err != nil {
			return nil, err
		}
		d.DebugLog("propagate tags from service", *sv.ServiceArn, out)
		in.Tags = append(in.Tags, out.Tags...)
	case "":
		in.PropagateTags = types.PropagateTagsNone
	default:
		in.PropagateTags = types.PropagateTagsTaskDefinition
	}
	d.DebugLog("run task input", in)

	out, err := d.ecsv2.RunTask(ctx, in)
	if err != nil {
		return nil, err
	}
	if len(out.Failures) > 0 {
		f := out.Failures[0]
		if f.Arn != nil {
			d.Log("Task ARN: " + *f.Arn)
		}
		return nil, errors.New(*f.Reason)
	}

	task := out.Tasks[0]
	d.Log("Task ARN:", *task.TaskArn)
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
			return errors.Wrap(err, "failed to run task")
		}
		return nil
	}

	d.Log(fmt.Sprintf("Watching container: %s", *watchContainer.Name))
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
		return errors.Wrap(err, "failed to run task")
	}
	return nil
}

func (d *App) waitTask(ctx context.Context, task *types.Task, untilRunning bool) error {
	id := arnToName(*task.TaskArn)
	if untilRunning {
		d.Log(fmt.Sprintf("Waiting for task ID %s until running", id))
		waiter := ecs.NewTasksRunningWaiter(d.ecsv2)
		if err := waiter.Wait(ctx, d.DescribeTasksInput(task), d.config.Timeout); err != nil {
			return err
		}
		d.Log(fmt.Sprintf("Task ID %s is running", id))
		return nil
	}

	d.Log(fmt.Sprintf("Waiting for task ID %s until stopped", id))
	waiter := ecs.NewTasksStoppedWaiter(d.ecsv2)
	return waiter.Wait(ctx, d.DescribeTasksInput(task), d.config.Timeout)
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
		watchContainer = containerOf(td, opt.WatchContainer)
		d.Log("Task definition ARN:", tdArn)
		d.Log("Watch container:", *watchContainer.Name)
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
