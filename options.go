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
	DryRun *bool
}
