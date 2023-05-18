package ecspresso

import (
	"context"
	"os"
)

type RegisterOption struct {
	DryRun bool `help:"dry run" default:"false"`
	Output bool `help:"output the registered task definition as JSON" default:"false"`
}

func (opt RegisterOption) DryRunString() string {
	if opt.DryRun {
		return dryRunStr
	}
	return ""
}

func (d *App) Register(ctx context.Context, opt RegisterOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	d.Log("Starting register task definition %s", opt.DryRunString())
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}
	if opt.DryRun {
		d.Log("task definition:")
		if err := d.OutputJSONForAPI(os.Stdout, td); err != nil {
			return err
		}
		d.Log("DRY RUN OK")
		return nil
	}

	newTd, err := d.RegisterTaskDefinition(ctx, td)
	if err != nil {
		return err
	}

	if opt.Output {
		return d.OutputJSONForAPI(os.Stdout, newTd)
	}
	return nil
}
