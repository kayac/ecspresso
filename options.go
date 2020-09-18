package ecspresso

const dryRunStr = "DRY RUN"

type DryRunnable interface {
	DryRunString() bool
}

type CreateOption struct {
	DryRun       *bool
	DesiredCount *int64
	NoWait       *bool
}

func (opt CreateOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

type DeployOption struct {
	DryRun               *bool
	DesiredCount         *int64
	SkipTaskDefinition   *bool
	ForceNewDeployment   *bool
	NoWait               *bool
	SuspendAutoScaling   *bool
	RollbackEvents       *string
	UpdateService        *bool
	LatestTaskDefinition *bool
}

func (opt DeployOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

type StatusOption struct {
	Events *int
}

type RollbackOption struct {
	DryRun                   *bool
	DeregisterTaskDefinition *bool
	NoWait                   *bool
}

func (opt RollbackOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

type DeleteOption struct {
	DryRun *bool
	Force  *bool
}

func (opt DeleteOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

type RunOption struct {
	DryRun             *bool
	TaskDefinition     *string
	NoWait             *bool
	TaskOverrideStr    *string
	SkipTaskDefinition *bool
	Count              *int64
	WatchContainer     *string
}

func (opt RunOption) DryRunString() string {
	if *opt.DryRun {
		return ""
	}
	return ""
}

type WaitOption struct {
}

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

type InitOption struct {
	Region                *string
	Cluster               *string
	Service               *string
	TaskDefinitionPath    *string
	ServiceDefinitionPath *string
	ConfigFilePath        *string
}

type DiffOption struct {
}
