package ecspresso

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func (d *App) createService(ctx context.Context, opt DeployOption) error {
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

	if opt.DryRun {
		d.Log("task definition:")
		d.OutputJSONForAPI(os.Stderr, td)
		d.Log("service definition:")
		d.OutputJSONForAPI(os.Stderr, svd)
		d.Log("DRY RUN OK")
		return nil
	}

	var tdArn string
	if opt.LatestTaskDefinition || opt.SkipTaskDefinition {
		var err error
		tdArn, err = d.findLatestTaskDefinitionArn(ctx, aws.ToString(td.Family))
		if err != nil {
			return err
		}
		d.Log("Using latest task definition %s", tdArn)
	} else {
		newTd, err := d.RegisterTaskDefinition(ctx, td)
		if err != nil {
			return err
		}
		tdArn = *newTd.TaskDefinitionArn
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
		ServiceConnectConfiguration:   svd.ServiceConnectConfiguration,
		ServiceName:                   svd.ServiceName,
		ServiceRegistries:             svd.ServiceRegistries,
		Tags:                          svd.Tags,
		TaskDefinition:                aws.String(tdArn),
		VolumeConfigurations:          svd.VolumeConfigurations,
	}
	if _, err := d.ecs.CreateService(ctx, createServiceInput); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	d.Log("Service is created")

	if !opt.Wait {
		return nil
	}

	time.Sleep(delayForServiceChanged) // wait for service created

	sv, err := d.DescribeService(ctx)
	if err != nil {
		return err
	}

	doWait, err := d.WaitFunc(sv)
	if err != nil {
		return err
	}

	if err := doWait(ctx, sv); err != nil {
		if errors.As(err, &errNotFound) && sv.isCodeDeploy() {
			d.Log("[INFO] %s", err)
			return d.WaitTaskSetStable(ctx, sv)
		}
		return err
	}

	d.Log("Service is stable now. Completed!")
	return nil
}
