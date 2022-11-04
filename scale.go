package ecspresso

type ScaleOption struct {
	DryRun             *bool  `help:"dry run" default:"false"`
	DesiredCount       *int32 `name:"tasks" help:"desired count of tasks" default:"-1"`
	NoWait             *bool  `help:"exit ecspresso immediately after just deployed without waiting for service stable" default:"false"`
	SuspendAutoScaling *bool  `help:"suspend application auto-scaling attached with the ECS service"`
}

func (o *ScaleOption) DeployOption() DeployOption {
	return DeployOption{
		DryRun:               o.DryRun,
		SkipTaskDefinition:   ptr(true),
		ForceNewDeployment:   ptr(false),
		NoWait:               o.NoWait,
		RollbackEvents:       ptr(""),
		UpdateService:        ptr(false),
		LatestTaskDefinition: ptr(false),
	}
}
