package ecspresso

type CreateOption struct {
	DryRun *bool
}

type DeployOption struct {
	DryRun *bool
}

type StatusOption struct {
	Events *int
}

type RollbackOption struct {
	DryRun *bool
}
