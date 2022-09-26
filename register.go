package ecspresso

type RegisterOption struct {
	DryRun *bool
	Output *bool
}

func (opt RegisterOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

func (d *App) Register(opt RegisterOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting register task definition", opt.DryRunString())
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}
	if *opt.DryRun {
		d.Log("task definition:")
		d.LogJSON(td)
		d.Log("DRY RUN OK")
		return nil
	}

	newTd, err := d.RegisterTaskDefinition(ctx, td)
	if err != nil {
		return err
	}

	if *opt.Output {
		d.LogJSON(newTd)
	}
	return nil
}
