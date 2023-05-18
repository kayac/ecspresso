package ecspresso

type ScaleOption struct {
	DryRun             bool   `help:"dry run" default:"false"`
	DesiredCount       *int32 `name:"tasks" help:"desired count of tasks" default:"-1"`
	Wait               bool   `help:"wait for service stable" default:"true" negatable:""`
	SuspendAutoScaling *bool  `help:"suspend application auto-scaling attached with the ECS service"`
	ResumeAutoScaling  *bool  `help:"resume application auto-scaling attached with the ECS service"`
	AutoScalingMin     *int32 `help:"set minimum capacity of application auto-scaling attached with the ECS service"`
	AutoScalingMax     *int32 `help:"set maximum capacity of application auto-scaling attached with the ECS service"`
}

func (o *ScaleOption) DeployOption() DeployOption {
	return DeployOption{
		DesiredCount:         o.DesiredCount,
		DryRun:               o.DryRun,
		SkipTaskDefinition:   true,
		ForceNewDeployment:   false,
		Wait:                 o.Wait,
		RollbackEvents:       "",
		UpdateService:        false,
		LatestTaskDefinition: false,
		SuspendAutoScaling:   o.SuspendAutoScaling,
		ResumeAutoScaling:    o.ResumeAutoScaling,
		AutoScalingMin:       o.AutoScalingMin,
		AutoScalingMax:       o.AutoScalingMax,
	}
}
