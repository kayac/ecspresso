package ecspresso

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type CreateOption struct {
	DryRun       *bool
	DesiredCount *int32
	NoWait       *bool
}

func (opt CreateOption) getDesiredCount() *int32 {
	return opt.DesiredCount
}

func (opt CreateOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

func (d *App) Create(opt CreateOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting create service %s", opt.DryRunString())
	svd, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
	if err != nil {
		return err
	}
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}

	count := calcDesiredCount(svd, opt)
	if count == nil && (svd.SchedulingStrategy != "" && svd.SchedulingStrategy == types.SchedulingStrategyReplica) {
		count = aws.Int32(0) // Must provide desired count for replica scheduling strategy
	}

	if *opt.DryRun {
		d.Log("task definition:")
		d.LogJSON(td)
		d.Log("service definition:")
		d.LogJSON(svd)
		d.Log("DRY RUN OK")
		return nil
	}

	newTd, err := d.RegisterTaskDefinition(ctx, td)
	if err != nil {
		return err
	}
	createServiceInput := &ecs.CreateServiceInput{
		Cluster:                       aws.String(d.config.Cluster),
		CapacityProviderStrategy:      svd.CapacityProviderStrategy,
		DeploymentConfiguration:       svd.DeploymentConfiguration,
		DeploymentController:          svd.DeploymentController,
		DesiredCount:                  count,
		EnableECSManagedTags:          svd.EnableECSManagedTags,
		EnableExecuteCommand:          svd.EnableExecuteCommand,
		HealthCheckGracePeriodSeconds: svd.HealthCheckGracePeriodSeconds,
		LaunchType:                    svd.LaunchType,
		LoadBalancers:                 svd.LoadBalancers,
		NetworkConfiguration:          svd.NetworkConfiguration,
		PlacementConstraints:          svd.PlacementConstraints,
		PlacementStrategy:             svd.PlacementStrategy,
		PlatformVersion:               svd.PlatformVersion,
		PropagateTags:                 svd.PropagateTags,
		SchedulingStrategy:            svd.SchedulingStrategy,
		ServiceName:                   svd.ServiceName,
		ServiceRegistries:             svd.ServiceRegistries,
		Tags:                          svd.Tags,
		TaskDefinition:                newTd.TaskDefinitionArn,
	}
	if _, err := d.ecs.CreateService(ctx, createServiceInput); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	d.Log("Service is created")

	if *opt.NoWait {
		return nil
	}

	start := time.Now()
	time.Sleep(delayForServiceChanged) // wait for service created
	if err := d.WaitServiceStable(ctx, start); err != nil {
		return fmt.Errorf("failed to wait service stable: %w", err)
	}

	d.Log("Service is stable now. Completed!")
	return nil
}
