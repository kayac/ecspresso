package ecspresso

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
)

func (d *App) Run(opt RunOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Running task", opt.DryRunString())
	ov := ecs.TaskOverride{}
	if ovStr := aws.StringValue(opt.TaskOverrideStr); ovStr != "" {
		if err := json.Unmarshal([]byte(ovStr), &ov); err != nil {
			return errors.Wrap(err, "invalid overrides")
		}
	} else if ovFile := aws.StringValue(opt.TaskOverrideFile); ovFile != "" {
		src, err := d.readDefinitionFile(ovFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read overrides-file %s", ovFile)
		}
		if err := d.unmarshalJSON(src, &ov, ovFile); err != nil {
			return errors.Wrapf(err, "failed to read overrides-file %s", ovFile)
		}
	}
	d.DebugLog("Overrides:", ov.String())

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

func (d *App) RunTask(ctx context.Context, tdArn string, ov *ecs.TaskOverride, opt *RunOption) (*ecs.Task, error) {
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

	switch aws.StringValue(opt.PropagateTags) {
	case "SERVICE":
		out, err := d.ecs.ListTagsForResourceWithContext(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: sv.ServiceArn,
		})
		if err != nil {
			return nil, err
		}
		d.DebugLog("propagate tags from service", *sv.ServiceArn, out.String())
		for _, tag := range out.Tags {
			in.Tags = append(in.Tags, tag)
		}
	case "":
		in.PropagateTags = nil
	default:
		in.PropagateTags = opt.PropagateTags
	}
	d.DebugLog("run task input", in.String())

	out, err := d.ecs.RunTaskWithContext(ctx, in)
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
	return task, nil
}

func (d *App) WaitRunTask(ctx context.Context, task *ecs.Task, watchContainer *ecs.ContainerDefinition, startedAt time.Time, untilRunning bool) error {
	d.Log("Waiting for run task...(it may take a while)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	lc := watchContainer.LogConfiguration
	if lc == nil || *lc.LogDriver != "awslogs" || lc.Options["awslogs-stream-prefix"] == nil {
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

func (d *App) waitTask(ctx context.Context, task *ecs.Task, untilRunning bool) error {
	// Add an option WithWaiterDelay and request.WithWaiterMaxAttempts for a long timeout.
	// SDK Default is 10 min (MaxAttempts=100 * Delay=6sec) at now.
	const delay = 6 * time.Second
	attempts := int((d.config.Timeout / delay)) + 1
	if (d.config.Timeout % delay) > 0 {
		attempts++
	}

	id := arnToName(*task.TaskArn)
	if untilRunning {
		d.Log(fmt.Sprintf("Waiting for task ID %s until running", id))
		if err := d.ecs.WaitUntilTasksRunningWithContext(
			ctx,
			d.DescribeTasksInput(task),
			request.WithWaiterDelay(request.ConstantWaiterDelay(delay)),
			request.WithWaiterMaxAttempts(attempts),
		); err != nil {
			return err
		}
		d.Log(fmt.Sprintf("Task ID %s is running", id))
		return nil
	}

	d.Log(fmt.Sprintf("Waiting for task ID %s until stopped", id))
	return d.ecs.WaitUntilTasksStoppedWithContext(
		ctx, d.DescribeTasksInput(task),
		request.WithWaiterDelay(request.ConstantWaiterDelay(delay)),
		request.WithWaiterMaxAttempts(attempts),
	)
}

func (d *App) taskDefinitionForRun(ctx context.Context, opt RunOption) (tdArn string, watchContainer *ecs.ContainerDefinition, err error) {
	tdPath := aws.StringValue(opt.TaskDefinition)
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
		if aws.Int64Value(opt.Revision) != 0 {
			tdArn = fmt.Sprintf("%s:%d", family, aws.Int64Value(opt.Revision))
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
	return
}
