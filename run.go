package ecspresso

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
)

func (d *App) Run(opt RunOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Running task", opt.DryRunString())
	var ov ecs.TaskOverride
	if ovStr := *opt.TaskOverrideStr; ovStr != "" {
		if err := json.Unmarshal([]byte(ovStr), &ov); err != nil {
			return errors.Wrap(err, "invalid overrides")
		}
	}

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

		td, err := d.DescribeTaskDefinition(ctx, tdArn)
		if err != nil {
			return errors.Wrap(err, "failed to describe task definition")
		}
		watchContainer = containerOf(td, opt.WatchContainer)
		if *opt.DryRun {
			d.Log("task definition:")
			d.LogJSON(td)
		}
	} else if *opt.SkipTaskDefinition {
		td, err := d.DescribeTaskDefinition(ctx, *sv.TaskDefinition)
		if err != nil {
			return errors.Wrap(err, "failed to describe task definition")
		}
		tdArn = *(td.TaskDefinitionArn)
		watchContainer = containerOf(td, opt.WatchContainer)
		if *opt.DryRun {
			d.Log("task definition:")
			d.LogJSON(td)
		}
	} else {
		td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
		if err != nil {
			return errors.Wrap(err, "failed to load task definition")
		}

		if len(*opt.TaskDefinition) > 0 {
			d.Log("Loading task definition")
			runTd, err := d.LoadTaskDefinition(*opt.TaskDefinition)
			if err != nil {
				return errors.Wrap(err, "failed to load task definition")
			}
			td = runTd
		}

		var newTd *ecs.TaskDefinition
		_ = newTd

		if *opt.DryRun {
			d.Log("task definition:")
			d.LogJSON(td)
		} else {
			newTd, err = d.RegisterTaskDefinition(ctx, td)
			if err != nil {
				return errors.Wrap(err, "failed to register task definition")
			}
			tdArn = *newTd.TaskDefinitionArn
			watchContainer = containerOf(td, opt.WatchContainer)
		}
	}
	if watchContainer == nil {
		return fmt.Errorf("container %s is not found in task definition", *opt.WatchContainer)
	}
	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	task, err := d.RunTask(ctx, tdArn, sv, &ov, *opt.Count)
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
