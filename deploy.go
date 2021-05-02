package ecspresso

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kayac/ecspresso/appspec"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/codedeploy"
	"github.com/aws/aws-sdk-go/service/ecs"
	isatty "github.com/mattn/go-isatty"
	"github.com/pkg/errors"
)

const (
	CodeDeployConsoleURLFmt = "https://%s.console.aws.amazon.com/codesuite/codedeploy/deployments/%s?region=%s"
)

func calcDesiredCount(sv *ecs.Service, opt optWithDesiredCount) *int64 {
	if aws.StringValue(sv.SchedulingStrategy) == "DAEMON" {
		return nil
	}
	if oc := opt.getDesiredCount(); oc != nil {
		if *oc == DefaultDesiredCount {
			return sv.DesiredCount
		}
		return oc // --tasks
	}
	return nil
}

func (d *App) Deploy(opt DeployOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	var sv *ecs.Service
	d.Log("Starting deploy", opt.DryRunString())
	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return errors.Wrap(err, "failed to describe current service status")
	}

	var tdArn string
	if *opt.LatestTaskDefinition {
		family := strings.Split(arnToName(*sv.TaskDefinition), ":")[0]
		var err error
		tdArn, err = d.findLatestTaskDefinitionArn(ctx, family)
		if err != nil {
			return errors.Wrap(err, "failed to load latest task definition")
		}
	} else if *opt.SkipTaskDefinition {
		tdArn = *sv.TaskDefinition
	} else {
		td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
		if err != nil {
			return errors.Wrap(err, "failed to load task definition")
		}
		if *opt.DryRun {
			d.Log("task definition:")
			d.LogJSON(td)
		} else {
			newTd, err := d.RegisterTaskDefinition(ctx, td)
			if err != nil {
				return errors.Wrap(err, "failed to register task definition")
			}
			tdArn = *newTd.TaskDefinitionArn
		}
	}

	var count *int64
	if d.config.ServiceDefinitionPath != "" && aws.BoolValue(opt.UpdateService) {
		newSv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
		if err != nil {
			return errors.Wrap(err, "failed to load service definition")
		}
		ds, err := diffServices(sv, newSv)
		if err != nil {
			return errors.Wrap(err, "failed to diff of service definitions")
		}
		if ds != "" {
			if err = d.UpdateServiceAttributes(ctx, newSv, opt); err != nil {
				return errors.Wrap(err, "failed to update service attributes")
			}
			sv = newSv // updated
		} else {
			d.Log("service attributes will not change")
		}
		count = calcDesiredCount(newSv, opt)
	} else {
		count = calcDesiredCount(sv, opt)
	}
	if count != nil {
		d.Log("desired count:", *count)
	} else {
		d.Log("desired count: unchanged")
	}

	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	// manage auto scaling only when set option --suspend-auto-scaling or --no-suspend-auto-scaling explicitly
	if suspendState := opt.SuspendAutoScaling; suspendState != nil {
		if err := d.suspendAutoScaling(*suspendState); err != nil {
			return err
		}
	}

	// detect controller
	if dc := sv.DeploymentController; dc != nil {
		switch t := *dc.Type; t {
		case "CODE_DEPLOY":
			return d.DeployByCodeDeploy(ctx, tdArn, count, sv, opt)
		default:
			return fmt.Errorf("could not deploy a service using deployment controller type %s", t)
		}
	}

	// rolling deploy (ECS internal)
	if err := d.UpdateServiceTasks(ctx, tdArn, count, opt); err != nil {
		return errors.Wrap(err, "failed to update service tasks")
	}

	if *opt.NoWait {
		d.Log("Service is deployed.")
		return nil
	}

	if err := d.WaitServiceStable(ctx, time.Now()); err != nil {
		return errors.Wrap(err, "failed to wait service stable")
	}

	d.Log("Service is stable now. Completed!")
	return nil
}

func (d *App) UpdateServiceTasks(ctx context.Context, taskDefinitionArn string, count *int64, opt DeployOption) error {
	in := &ecs.UpdateServiceInput{
		Service:            aws.String(d.Service),
		Cluster:            aws.String(d.Cluster),
		TaskDefinition:     aws.String(taskDefinitionArn),
		DesiredCount:       count,
		ForceNewDeployment: opt.ForceNewDeployment,
	}
	msg := "Updating service tasks"
	if *opt.ForceNewDeployment {
		msg = msg + " with force new deployment"
	}
	msg = msg + "..."
	d.Log(msg)
	d.DebugLog(in.String())

	_, err := d.ecs.UpdateServiceWithContext(ctx, in)
	if err != nil {
		return err
	}
	time.Sleep(delayForServiceChanged) // wait for service updated
	return nil
}

func svToUpdateServiceInput(sv *ecs.Service) *ecs.UpdateServiceInput {
	in := &ecs.UpdateServiceInput{
		CapacityProviderStrategy:      sv.CapacityProviderStrategy,
		DeploymentConfiguration:       sv.DeploymentConfiguration,
		DesiredCount:                  sv.DesiredCount,
		EnableExecuteCommand:          sv.EnableExecuteCommand,
		HealthCheckGracePeriodSeconds: sv.HealthCheckGracePeriodSeconds,
		NetworkConfiguration:          sv.NetworkConfiguration,
		PlacementConstraints:          sv.PlacementConstraints,
		PlacementStrategy:             sv.PlacementStrategy,
		PlatformVersion:               sv.PlatformVersion,
	}
	if aws.StringValue(sv.SchedulingStrategy) == "DAEMON" {
		in.PlacementConstraints = nil
	}
	return in
}

func (d *App) UpdateServiceAttributes(ctx context.Context, sv *ecs.Service, opt DeployOption) error {
	in := svToUpdateServiceInput(sv)
	if isCodeDeploy(sv.DeploymentController) {
		// unable to update attributes below with a CODE_DEPLOY deployment controller.
		in.NetworkConfiguration = nil
		in.PlatformVersion = nil
		in.ForceNewDeployment = nil
	} else {
		in.ForceNewDeployment = opt.ForceNewDeployment
	}
	in.Service = aws.String(d.Service)
	in.Cluster = aws.String(d.Cluster)

	if *opt.DryRun {
		d.Log("update service input:")
		d.LogJSON(in)
		return nil
	}
	d.Log("Updating service attributes...")
	d.DebugLog(in.String())

	if _, err := d.ecs.UpdateServiceWithContext(ctx, in); err != nil {
		return err
	}
	time.Sleep(delayForServiceChanged) // wait for service updated
	return nil
}

func (d *App) DeployByCodeDeploy(ctx context.Context, taskDefinitionArn string, count *int64, sv *ecs.Service, opt DeployOption) error {
	if count != nil {
		d.Log("updating desired count to", *count)
	}
	_, err := d.ecs.UpdateServiceWithContext(
		ctx,
		&ecs.UpdateServiceInput{
			Service:      aws.String(d.Service),
			Cluster:      aws.String(d.Cluster),
			DesiredCount: count,
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to update service")
	}
	if aws.BoolValue(opt.SkipTaskDefinition) && !aws.BoolValue(opt.UpdateService) && !aws.BoolValue(opt.ForceNewDeployment) {
		// no need to create new deployment.
		return nil
	}

	return d.createDeployment(ctx, sv, taskDefinitionArn, opt.RollbackEvents)
}

func (d *App) findDeploymentInfo() (*codedeploy.DeploymentInfo, error) {
	// search deploymentGroup in CodeDeploy
	d.DebugLog("find all applications in CodeDeploy")
	la, err := d.codedeploy.ListApplications(&codedeploy.ListApplicationsInput{})
	if err != nil {
		return nil, err
	}
	if len(la.Applications) == 0 {
		return nil, errors.New("no any applications in CodeDeploy")
	}

	apps, err := d.codedeploy.BatchGetApplications(&codedeploy.BatchGetApplicationsInput{
		ApplicationNames: la.Applications,
	})
	if err != nil {
		return nil, err
	}
	for _, info := range apps.ApplicationsInfo {
		d.DebugLog("application", info.String())
		if *info.ComputePlatform != "ECS" {
			continue
		}
		lg, err := d.codedeploy.ListDeploymentGroups(&codedeploy.ListDeploymentGroupsInput{
			ApplicationName: info.ApplicationName,
		})
		if err != nil {
			return nil, err
		}
		if len(lg.DeploymentGroups) == 0 {
			d.DebugLog("no deploymentGroups in application", *info.ApplicationName)
			continue
		}
		groups, err := d.codedeploy.BatchGetDeploymentGroups(&codedeploy.BatchGetDeploymentGroupsInput{
			ApplicationName:      info.ApplicationName,
			DeploymentGroupNames: lg.DeploymentGroups,
		})
		if err != nil {
			return nil, err
		}
		for _, dg := range groups.DeploymentGroupsInfo {
			d.DebugLog("deploymentGroup", dg.String())
			for _, ecsService := range dg.EcsServices {
				if *ecsService.ClusterName == d.config.Cluster && *ecsService.ServiceName == d.config.Service {
					return &codedeploy.DeploymentInfo{
						ApplicationName:      aws.String(*info.ApplicationName),
						DeploymentGroupName:  aws.String(*dg.DeploymentGroupName),
						DeploymentConfigName: aws.String(*dg.DeploymentConfigName),
					}, nil
				}
			}
		}
	}
	return nil, fmt.Errorf(
		"failed to find CodeDeploy Application/DeploymentGroup for ECS service %s on cluster %s",
		d.config.Service,
		d.config.Cluster,
	)
}

func isCodeDeploy(dc *ecs.DeploymentController) bool {
	if dc != nil && dc.Type != nil && *dc.Type == "CODE_DEPLOY" {
		return true
	}
	return false
}

func (d *App) createDeployment(ctx context.Context, sv *ecs.Service, taskDefinitionArn string, rollbackEvents *string) error {

	spec, err := appspec.NewWithService(sv, taskDefinitionArn)
	if err != nil {
		return errors.Wrap(err, "failed to create appspec")
	}
	if d.config.AppSpec != nil {
		spec.Hooks = d.config.AppSpec.Hooks
	}
	d.DebugLog("appSpecContent:", spec.String())

	// deployment
	dp, err := d.findDeploymentInfo()
	if err != nil {
		return err
	}
	dd := &codedeploy.CreateDeploymentInput{
		ApplicationName:      dp.ApplicationName,
		DeploymentGroupName:  dp.DeploymentGroupName,
		DeploymentConfigName: dp.DeploymentConfigName,
		Revision: &codedeploy.RevisionLocation{
			RevisionType: aws.String("AppSpecContent"),
			AppSpecContent: &codedeploy.AppSpecContent{
				Content: aws.String(spec.String()),
			},
		},
	}
	if ev := aws.StringValue(rollbackEvents); ev != "" {
		var events []*string
		for _, ev := range strings.Split(ev, ",") {
			events = append(events, aws.String(ev))
		}
		dd.AutoRollbackConfiguration = &codedeploy.AutoRollbackConfiguration{
			Enabled: aws.Bool(true),
			Events:  events,
		}
	}

	d.DebugLog("creating a deployment to CodeDeploy", dd.String())

	res, err := d.codedeploy.CreateDeploymentWithContext(ctx, dd)
	if err != nil {
		return errors.Wrap(err, "failed to create deployment")
	}
	id := *res.DeploymentId
	u := fmt.Sprintf(
		CodeDeployConsoleURLFmt,
		d.config.Region,
		id,
		d.config.Region,
	)
	d.Log(fmt.Sprintf("Deployment %s is created on CodeDeploy:", id))
	d.Log(u)

	if isatty.IsTerminal(os.Stdout.Fd()) {
		if err := exec.Command("open", u).Start(); err != nil {
			d.Log("Couldn't open URL", u)
		}
	}
	return nil
}
