package ecspresso

type RefreshOption struct {
	DryRun bool `help:"dry run" default:"false"`
	Wait   bool `help:"wait for service stable" default:"true"`
}

func (o *RefreshOption) DeployOption() DeployOption {
	return DeployOption{
		DryRun:               o.DryRun,
		DesiredCount:         nil,
		SkipTaskDefinition:   true,
		ForceNewDeployment:   true,
		Wait:                 o.Wait,
		RollbackEvents:       ptr(""),
		UpdateService:        false,
		LatestTaskDefinition: false,
	}
}
