package ecspresso

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
)

type DeregisterOption struct {
	DryRun   *bool
	Keeps    *int64
	Revision *int64
}

func (opt DeregisterOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

func (d *App) Deregister(opt DeregisterOption) error {
	ctx, cancel := d.Start()
	defer cancel()
	d.Log("Starting deregister task definition", opt.DryRunString())

	if opt.Revision != nil {
		return d.deregiserRevision(ctx, opt)
	}
	return nil
}

func (d *App) deregiserRevision(ctx context.Context, opt DeregisterOption) error {
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "failed to load task definition")
	}
	name := fmt.Sprintf("%s:%d", aws.StringValue(td.Family), aws.Int64Value(opt.Revision))

	if aws.BoolValue(opt.DryRun) {
		d.Log(fmt.Sprintf("task definition %s will be deregistered", name))
		d.Log("DRY RUN OK")
		return nil
	}

	d.Log(fmt.Sprintf("Deregistring %s", name))
	if _, err := d.ecs.DeregisterTaskDefinitionWithContext(ctx, &ecs.DeregisterTaskDefinitionInput{
		TaskDefinition: aws.String(name),
	}); err != nil {
		return errors.Wrap(err, "failed to deregister task definition")
	}
	d.Log(fmt.Sprintf("task definition %s is deregistered successfully", name))
	return nil
}
