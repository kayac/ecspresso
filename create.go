package ecspresso

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func (d *App) createService(ctx context.Context, opt DeployOption) error {
	d.LogInfo("Starting create service", withDryRun(opt.DryRun)...)
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

	// fail fast before creating the service
	until := waitUntil(opt.WaitUntil)
	doWait, err := d.WaitFunc(svd, nil, until)
	if err != nil {
		return err
	}
	if until.forECSLifecycleStage() {
		stage := types.ServiceDeploymentLifecycleStage(until.ecsLifecycleStage())
		if err := validateLifecycleStageSupported(svd, stage); err != nil {
			return err
		}
	}
	if opt.Wait {
		d.warnEarlySuccessCriteriaWait(svd, until)
	}

	if opt.DryRun {
		d.LogInfo("task definition:")
		OutputJSONForAPI(os.Stdout, td)
		d.LogInfo("service definition:")
		OutputJSONForAPI(os.Stdout, svd)
		d.LogInfo("DRY RUN OK")
		return nil
	}

	var tdArn string
	if opt.LatestTaskDefinition || opt.SkipTaskDefinition {
		var err error
		tdArn, err = d.findLatestTaskDefinitionArn(ctx, aws.ToString(td.Family))
		if err != nil {
			return err
		}
		d.LogInfo("using latest task definition", "task_definition", tdArn)
	} else {
		newTd, err := d.RegisterTaskDefinition(ctx, td)
		if err != nil {
			return err
		}
		tdArn = *newTd.TaskDefinitionArn
	}

	createServiceInput := &ecs.CreateServiceInput{
		AvailabilityZoneRebalancing:   svd.AvailabilityZoneRebalancing,
		Cluster:                       aws.String(d.config.Cluster),
		CapacityProviderStrategy:      svd.CapacityProviderStrategy,
		DeploymentConfiguration:       svd.DeploymentConfiguration,
		DeploymentController:          svd.DeploymentController,
		DesiredCount:                  count,
		EnableECSManagedTags:          svd.EnableECSManagedTags,
		EnableExecuteCommand:          svd.EnableExecuteCommand,
		GatewayConnectors:             svd.GatewayConnectors,
		HealthCheckGracePeriodSeconds: svd.HealthCheckGracePeriodSeconds,
		LaunchType:                    svd.LaunchType,
		LoadBalancers:                 svd.LoadBalancers,
		NetworkConfiguration:          svd.NetworkConfiguration,
		PlacementConstraints:          svd.PlacementConstraints,
		PlacementStrategy:             svd.PlacementStrategy,
		PlatformVersion:               svd.PlatformVersion,
		PropagateTags:                 svd.PropagateTags,
		SchedulingStrategy:            svd.SchedulingStrategy,
		Monitoring:                    svd.Monitoring,
		ServiceConnectConfiguration:   svd.ServiceConnectConfiguration,
		ServiceName:                   svd.ServiceName,
		ServiceRegistries:             svd.ServiceRegistries,
		Tags:                          svd.Tags,
		TaskDefinition:                aws.String(tdArn),
		VolumeConfigurations:          svd.VolumeConfigurations,
		VpcLatticeConfigurations:      svd.VpcLatticeConfigurations,
	}
	if _, err := d.ecs.CreateService(ctx, createServiceInput); err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	d.LogInfo("Service is created")

	if !opt.Wait {
		return nil
	}

	sleepContext(ctx, delayForServiceChanged) // wait for service created

	sv, err := d.DescribeService(ctx)
	if err != nil {
		return err
	}

	if err := doWait(ctx, sv); err != nil {
		if errors.Is(err, ErrNotFound) && sv.isCodeDeploy() {
			d.LogInfo(err.Error())
			return d.WaitTaskSetStable(ctx, sv)
		}
		return err
	}

	if msg := until.doneMessage(); msg != "" {
		d.LogInfo(msg)
	}
	return nil
}
