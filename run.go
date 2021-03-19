package ecspresso

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/morikuni/aec"
	"github.com/pkg/errors"
)

func (d *App) Run(opt RunOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Running task", opt.DryRunString())
	var ov ecs.TaskOverride
	if ovStr := aws.StringValue(opt.TaskOverrideStr); ovStr != "" {
		if err := json.Unmarshal([]byte(ovStr), &ov); err != nil {
			return errors.Wrap(err, "invalid overrides")
		}
	} else if ovFile := aws.StringValue(opt.TaskOverrideFile); ovFile != "" {
		if err := d.loader.LoadWithEnvJSON(&ov, ovFile); err != nil {
			return errors.Wrap(err, "failed to read overrides-file")
		}
	}
	d.DebugLog("Overrides:", ov.String())

	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return errors.Wrap(err, "failed to describe service status")
	}

	var tdArn string
	var watchContainer *ecs.ContainerDefinition

	if *opt.LatestTaskDefinition {
		family := strings.Split(arnToName(*sv.TaskDefinition), ":")[0]
		var err error
		tdArn, err = d.findLatestTaskDefinitionArn(ctx, family)
		if err != nil {
			return errors.Wrap(err, "failed to load latest task definition")
		}

		td, tdTags, err := d.DescribeTaskDefinition(ctx, tdArn)
		if err != nil {
			return errors.Wrap(err, "failed to describe task definition")
		}
		watchContainer = containerOf(td, opt.WatchContainer)
		if *opt.DryRun {
			d.Log("task definition:")
			d.LogJSON(td)
			d.Log("task definition tags:")
			d.LogJSON(tdTags)
		}
	} else if *opt.SkipTaskDefinition {
		td, tdTags, err := d.DescribeTaskDefinition(ctx, *sv.TaskDefinition)
		if err != nil {
			return errors.Wrap(err, "failed to describe task definition")
		}
		tdArn = *(td.TaskDefinitionArn)
		watchContainer = containerOf(td, opt.WatchContainer)
		if *opt.DryRun {
			d.Log("task definition:")
			d.LogJSON(td)
			d.Log("task definition tags:")
			d.LogJSON(tdTags)
		}
	} else {
		td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
		if err != nil {
			return errors.Wrap(err, "failed to load task definition")
		}
		tdTags, err := d.LoadTaskDefinitionTags(d.config.TaskDefinitionPath)
		if err != nil {
			return errors.Wrap(err, "failed to load task definition tags")
		}

		if len(*opt.TaskDefinition) > 0 {
			d.Log("Loading task definition")
			runTd, err := d.LoadTaskDefinition(*opt.TaskDefinition)
			if err != nil {
				return errors.Wrap(err, "failed to load task definition")
			}
			td = runTd

			runTdTags, err := d.LoadTaskDefinitionTags(*opt.TaskDefinition)
			if err != nil {
				return errors.Wrap(err, "failed to load task definition tags")
			}
			tdTags = runTdTags
		}
		watchContainer = containerOf(td, opt.WatchContainer)

		var newTd *TaskDefinition
		if *opt.DryRun {
			d.Log("task definition:")
			d.LogJSON(td)
		} else {
			newTd, err = d.RegisterTaskDefinition(ctx, td, tdTags)
			if err != nil {
				return errors.Wrap(err, "failed to register task definition")
			}
			tdArn = *newTd.TaskDefinitionArn
		}
	}
	if watchContainer == nil {
		return fmt.Errorf("container %s is not found in task definition", *opt.WatchContainer)
	}
	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	task, err := d.RunTask(ctx, tdArn, sv, &ov, &opt)
	if err != nil {
		return errors.Wrap(err, "failed to run task")
	}
	if *opt.NoWait {
		d.Log("Run task invoked")
		return nil
	}
	d.Log(fmt.Sprintf("Watching container: %s", *watchContainer.Name))
	if err := d.WaitRunTask(ctx, task, watchContainer, time.Now()); err != nil {
		return errors.Wrap(err, "failed to run task")
	}
	if err := d.DescribeTaskStatus(ctx, task, watchContainer); err != nil {
		return err
	}
	d.Log("Run task completed!")

	return nil
}

func (d *App) RunTask(ctx context.Context, tdArn string, sv *ecs.Service, ov *ecs.TaskOverride, opt *RunOption) (*ecs.Task, error) {
	d.Log("Running task")

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
		d.Log("Task ARN: " + *f.Arn)
		return nil, errors.New(*f.Reason)
	}

	task := out.Tasks[0]
	d.Log("Task ARN:", *task.TaskArn)
	return task, nil
}

func (d *App) WaitRunTask(ctx context.Context, task *ecs.Task, watchContainer *ecs.ContainerDefinition, startedAt time.Time) error {
	d.Log("Waiting for run task...(it may take a while)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	lc := watchContainer.LogConfiguration
	if lc == nil || *lc.LogDriver != "awslogs" || lc.Options["awslogs-stream-prefix"] == nil {
		d.Log("awslogs not configured")
		if err := d.WaitUntilTaskStopped(ctx, task); err != nil {
			return errors.Wrap(err, "failed to run task")
		}
		return nil
	}

	logGroup, logStream := d.GetLogInfo(task, watchContainer)
	time.Sleep(3 * time.Second) // wait for log stream

	go func() {
		tick := time.Tick(5 * time.Second)
		var lines int
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-tick:
				if isTerminal {
					for i := 0; i < lines; i++ {
						fmt.Print(aec.EraseLine(aec.EraseModes.All), aec.PreviousLine(1))
					}
				}
				lines, _ = d.GetLogEvents(waitCtx, logGroup, logStream, startedAt)
			}
		}
	}()

	if err := d.WaitUntilTaskStopped(ctx, task); err != nil {
		return errors.Wrap(err, "failed to run task")
	}
	return nil
}

func (d *App) WaitUntilTaskStopped(ctx context.Context, task *ecs.Task) error {
	// Add an option WithWaiterDelay and request.WithWaiterMaxAttempts for a long timeout.
	// SDK Default is 10 min (MaxAttempts=100 * Delay=6sec) at now.
	const delay = 6 * time.Second
	attempts := int((d.config.Timeout / delay)) + 1
	if (d.config.Timeout % delay) > 0 {
		attempts++
	}
	return d.ecs.WaitUntilTasksStoppedWithContext(
		ctx, d.DescribeTasksInput(task),
		request.WithWaiterDelay(request.ConstantWaiterDelay(delay)),
		request.WithWaiterMaxAttempts(attempts),
	)
}
