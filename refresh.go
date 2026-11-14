package ecspresso

type RefreshOption struct {
	DryRun bool `help:"dry run" default:"false"`
	Wait   bool `help:"wait for the deployment to complete" default:"true" negatable:""`
}

func (o *RefreshOption) DeployOption() DeployOption {
	return DeployOption{
		DryRun:               o.DryRun,
		DesiredCount:         nil,
		SkipTaskDefinition:   true,
		ForceNewDeployment:   true,
		Wait:                 o.Wait,
		WaitUntil:            string(waitUntilDeployed),
		RollbackEvents:       "",
		UpdateService:        false,
		LatestTaskDefinition: false,
	}
}
