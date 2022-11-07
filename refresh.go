package ecspresso

type RefreshOption struct {
	DryRun *bool `help:"dry run" default:"false"`
	NoWait *bool `help:"exit ecspresso immediately after just deployed without waiting for service stable" default:"false"`
}

func (o *RefreshOption) DeployOption() DeployOption {
	return DeployOption{
		DryRun:               o.DryRun,
		DesiredCount:         nil,
		SkipTaskDefinition:   ptr(true),
		ForceNewDeployment:   ptr(true),
		NoWait:               o.NoWait,
		RollbackEvents:       ptr(""),
		UpdateService:        ptr(false),
		LatestTaskDefinition: ptr(false),
	}
}
