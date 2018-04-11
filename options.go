package ecspresso

type CreateOption struct {
	DryRun       *bool
	DesiredCount *int64
}

type DeployOption struct {
	DryRun             *bool
	DesiredCount       *int64
	SkipTaskDefinition *bool
	ForceNewDeployment *bool
}

type StatusOption struct {
	Events *int
}

type RollbackOption struct {
	DryRun                   *bool
	DeregisterTaskDefinition *bool
}

type DeleteOption struct {
	DryRun *bool
	Force  *bool
}

type RunOption struct {
	DryRun         *bool
	TaskDefinition *string
}

type SchedulerOption struct {
	SchedulerDefinition *string
}

type SchedulerPutOption struct {
	SchedulerOption
	DryRun             *bool
	SkipTaskDefinition *bool
}

type SchedulerDeleteOption struct {
	SchedulerOption
	DryRun *bool
	Force  *bool
}

type SchedulerEnableOption struct {
	SchedulerOption
	DryRun *bool
}

type SchedulerDisableOption struct {
	SchedulerOption
	DryRun *bool
}
