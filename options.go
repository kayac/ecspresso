package ecspresso

type CreateOption struct {
	DryRun       *bool
	DesiredCount *int64
	NoWait       *bool
}

type DeployOption struct {
	DryRun             *bool
	DesiredCount       *int64
	SkipTaskDefinition *bool
	ForceNewDeployment *bool
	NoWait             *bool
}

type StatusOption struct {
	Events *int
}

type RollbackOption struct {
	DryRun                   *bool
	DeregisterTaskDefinition *bool
	NoWait                   *bool
}

type DeleteOption struct {
	DryRun *bool
	Force  *bool
}

type RunOption struct {
	DryRun             *bool
	TaskDefinition     *string
	NoWait             *bool
	TaskOverrideStr    *string
	SkipTaskDefinition *bool
	Count              *int64
}

type WaitOption struct {
}
