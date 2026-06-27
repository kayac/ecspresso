package ecspresso

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	cdTypes "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/kayac/ecspresso/v2/appspec"
	"github.com/samber/lo"
	"github.com/shogo82148/go-retry"
)

type RollbackOption struct {
	DryRun                   bool   `help:"dry run" default:"false"`
	DeregisterTaskDefinition bool   `help:"deregister the rolled-back task definition. not works with --no-wait" default:"true" negatable:""`
	Wait                     bool   `help:"wait for the service stable" default:"true" negatable:""`
	WaitUntil                string `help:"Choose whether to wait for service stable or the deployment finishes. (stable|deployed)" default:"stable" enum:"stable,deployed"`
	RollbackEvents           string `help:"roll back when specified events happened (DEPLOYMENT_FAILURE,DEPLOYMENT_STOP_ON_ALARM,DEPLOYMENT_STOP_ON_REQUEST,...) CodeDeploy only." default:""`
	PreviousTaskDef          bool   `help:"find the previous task definition revision and deploy it, regardless of active deployments" default:"false"`
}

func (opt RollbackOption) DryRunString() string {
	if opt.DryRun {
		return dryRunStr
	}
	return ""
}

func (d *App) Rollback(ctx context.Context, opt RollbackOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	if opt.DeregisterTaskDefinition && !opt.Wait {
		return fmt.Errorf("--deregister-task-definition not works with --no-wait together. Please use --no-deregister-task-definition with --no-wait")
	}

	d.LogInfo("Starting rollback", withDryRun(opt.DryRun)...)
	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return err
	}
	d.LogInfo("deployment controller", "type", string(sv.DeploymentController.Type))

	if opt.PreviousTaskDef {
		return d.rollbackByPreviousTaskDef(ctx, sv, opt)
	}

	doRollback, err := d.RollbackFunc(sv)
	if err != nil {
		return err
	}
	targetArn, err := d.FindRollbackTarget(ctx, aws.ToString(sv.TaskDefinition))
	if err != nil {
		return err
	}

	result, err := doRollback(ctx, sv, targetArn, opt)
	if err != nil {
		return err
	}

	if opt.DryRun {
		if err := d.rollbackTaskDefinition(ctx, result.taskDefinitionArn, opt); err != nil {
			return err
		}
		d.LogInfo("DRY RUN OK")
		return nil
	}

	if !opt.Wait {
		d.LogInfo("Service is rolled back.")
		return nil
	}

	doWait, err := d.WaitFunc(sv, d.confirmPrimaryTD(targetArn), waitUntil(opt.WaitUntil), result.deploymentArn)
	if err != nil {
		return err
	}

	sleepContext(ctx, delayForServiceChanged) // wait for service updated
	if err := doWait(ctx, sv); err != nil {
		if errors.Is(err, ErrNotFound) {
			d.LogInfo(err.Error())
			return d.rollbackTaskDefinition(ctx, result.taskDefinitionArn, opt)
		}
		return err
	}

	d.LogInfo(waitUntil(opt.WaitUntil).doneMessage())

	return d.rollbackTaskDefinition(ctx, result.taskDefinitionArn, opt)
}

func (d *App) rollbackByPreviousTaskDef(ctx context.Context, sv *Service, opt RollbackOption) error {
	currentArn := aws.ToString(sv.TaskDefinition)
	targetArn, err := d.FindRollbackTarget(ctx, currentArn)
	if err != nil {
		return err
	}

	result, err := d.RollbackServiceTasks(ctx, sv, targetArn, opt)
	if err != nil {
		return err
	}

	if opt.DryRun {
		if err := d.rollbackTaskDefinition(ctx, result.taskDefinitionArn, opt); err != nil {
			return err
		}
		d.LogInfo("DRY RUN OK")
		return nil
	}

	if !opt.Wait {
		d.LogInfo("Service is rolled back.")
		return nil
	}

	doWait, err := d.WaitFunc(sv, d.confirmPrimaryTD(targetArn), waitUntil(opt.WaitUntil))
	if err != nil {
		return err
	}

	sleepContext(ctx, delayForServiceChanged)
	if err := doWait(ctx, sv); err != nil {
		if errors.Is(err, ErrNotFound) {
			d.LogInfo(err.Error())
			return d.rollbackTaskDefinition(ctx, result.taskDefinitionArn, opt)
		}
		return err
	}

	d.LogInfo(waitUntil(opt.WaitUntil).doneMessage())
	return d.rollbackTaskDefinition(ctx, result.taskDefinitionArn, opt)
}

func (d *App) rollbackTaskDefinition(ctx context.Context, rollbackedTdArn string, opt RollbackOption) error {
	if !opt.DeregisterTaskDefinition {
		return nil
	}
	if opt.DryRun {
		d.LogInfo("task definition will be deregistered", "task_definition", arnToName(rollbackedTdArn))
		return nil
	}

	d.LogInfo("deregistering rolled-back task definition", "task_definition", arnToName(rollbackedTdArn))
	_, err := d.ecs.DeregisterTaskDefinition(
		ctx,
		&ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: &rollbackedTdArn,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to deregister task definition: %w", err)
	}
	d.LogInfo("task definition deregistered successfully", "task_definition", arnToName(rollbackedTdArn))
	return nil
}

func (d *App) RollbackServiceTasks(ctx context.Context, sv *Service, targetArn string, opt RollbackOption) (*rollbackResult, error) {
	currentArn := aws.ToString(sv.TaskDefinition)

	d.LogInfo("rolling back", withDryRun(opt.DryRun, "target", arnToName(targetArn))...)
	if opt.DryRun {
		return &rollbackResult{taskDefinitionArn: currentArn}, nil
	}

	if err := d.DeployByECS(
		ctx,
		&ecs.UpdateServiceInput{
			Service: sv.ServiceName,
			Cluster: aws.String(d.Cluster),
		},
		targetArn,
		nil,
		sv,
		DeployOption{
			ForceNewDeployment: false,
			UpdateService:      false,
		},
	); err != nil {
		return nil, err
	}
	return &rollbackResult{taskDefinitionArn: currentArn}, nil
}

func (d *App) RollbackExpressService(ctx context.Context, sv *Service, _ string, opt RollbackOption) (*rollbackResult, error) {
	deploymentArn, err := d.findActiveECSDeploymentArn(ctx, 0, false)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, errors.New("no active service deployment found")
		}
		return nil, err
	}

	d.LogInfo("active deployment found, rolling back", withDryRun(opt.DryRun, "deployment", arnToName(deploymentArn))...)
	return d.rollbackActiveECSDeployment(ctx, sv, deploymentArn, opt)
}

func (d *App) RollbackECSService(ctx context.Context, sv *Service, targetArn string, opt RollbackOption) (*rollbackResult, error) {
	deploymentArn, err := d.findActiveECSDeploymentArn(ctx, 0, false)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("no active deployment found. Use --previous-task-def to rollback by deploying the previous task definition revision")
		}
		return nil, err
	}

	d.LogInfo("active deployment found, rolling back", withDryRun(opt.DryRun, "deployment", arnToName(deploymentArn))...)
	return d.rollbackActiveECSDeployment(ctx, sv, deploymentArn, opt)
}

func (d *App) RollbackByCodeDeploy(ctx context.Context, sv *Service, targetArn string, opt RollbackOption) (*rollbackResult, error) {
	dp, err := d.findDeploymentInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find deployment info: %w", err)
	}

	ld, err := d.codedeploy.ListDeployments(ctx, &codedeploy.ListDeploymentsInput{
		ApplicationName:     dp.ApplicationName,
		DeploymentGroupName: dp.DeploymentGroupName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	if len(ld.Deployments) == 0 {
		return nil, fmt.Errorf("no deployments are found: %w", ErrNotFound)
	}

	out, err := d.codedeploy.GetDeployment(ctx, &codedeploy.GetDeploymentInput{
		DeploymentId: &ld.Deployments[0], // latest deployment
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}
	currentDeployment := out.DeploymentInfo

	d.LogInfo("current deployment", "deployment_id", *currentDeployment.DeploymentId)

	switch currentDeployment.Status {
	case cdTypes.DeploymentStatusSucceeded, cdTypes.DeploymentStatusFailed, cdTypes.DeploymentStatusStopped:
		currentTdArn := aws.ToString(sv.TaskDefinition)
		d.LogInfo("deployment not in progress, creating new deployment", withDryRun(opt.DryRun, "target", targetArn)...)
		if opt.DryRun {
			return &rollbackResult{taskDefinitionArn: currentTdArn}, nil
		}
		if err := d.createDeployment(ctx, sv, targetArn, opt.RollbackEvents); err != nil {
			return nil, fmt.Errorf("failed to create deployment: %w", err)
		}
		return &rollbackResult{taskDefinitionArn: currentTdArn}, nil
	default: // If the deployment is not yet complete
		d.LogInfo("deployment in progress, stopping", withDryRun(opt.DryRun, "deployment_id", *currentDeployment.DeploymentId)...)
		tdArn, err := d.findTaskDefinitionOfDeployment(ctx, currentDeployment)
		if err != nil {
			return nil, fmt.Errorf("failed to find task definition of deployment: %w", err)
		}
		if opt.DryRun {
			return &rollbackResult{taskDefinitionArn: tdArn}, nil
		}
		if _, err := d.codedeploy.StopDeployment(ctx, &codedeploy.StopDeploymentInput{
			DeploymentId:        currentDeployment.DeploymentId,
			AutoRollbackEnabled: aws.Bool(true),
		}); err != nil {
			return nil, fmt.Errorf("failed to roll back the deployment: %w", err)
		}
		if err := d.waitForCodeDeployRollback(ctx, *currentDeployment.DeploymentId); err != nil {
			return nil, fmt.Errorf("failed to wait for deployment rollback: %w", err)
		}
		return &rollbackResult{taskDefinitionArn: tdArn}, nil
	}
}

func (d *App) FindRollbackTarget(ctx context.Context, taskDefinitionArn string) (string, error) {
	var found bool
	var nextToken *string
	family := strings.Split(arnToName(taskDefinitionArn), ":")[0]
	for {
		out, err := d.ecs.ListTaskDefinitions(ctx,
			&ecs.ListTaskDefinitionsInput{
				NextToken:    nextToken,
				FamilyPrefix: aws.String(family),
				MaxResults:   aws.Int32(100),
				Sort:         types.SortOrderDesc,
			},
		)
		if err != nil {
			return "", fmt.Errorf("failed to list task definitions: %w", err)
		}
		if len(out.TaskDefinitionArns) == 0 {
			return "", fmt.Errorf("rollback target is not found for family %s: %w", family, ErrNotFound)
		}
		for _, tdArn := range out.TaskDefinitionArns {
			if found {
				return tdArn, nil
			}
			if tdArn == taskDefinitionArn {
				found = true
			}
		}
		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return "", fmt.Errorf("rollback target is not found: %w", ErrNotFound)
}

type rollbackResult struct {
	taskDefinitionArn string
	deploymentArn     string
}

type rollbackFunc func(ctx context.Context, sv *Service, targetArn string, opt RollbackOption) (*rollbackResult, error)

func (d *App) RollbackFunc(sv *Service) (rollbackFunc, error) {
	defaultFunc := d.RollbackServiceTasks
	if sv == nil || sv.DeploymentController == nil {
		return defaultFunc, nil
	}
	if sv.isExpressMode() {
		return d.RollbackExpressService, nil
	}
	if dc := sv.DeploymentController; dc != nil {
		switch dc.Type {
		case types.DeploymentControllerTypeCodeDeploy:
			return d.RollbackByCodeDeploy, nil
		case types.DeploymentControllerTypeEcs:
			return d.RollbackECSService, nil
		default:
			return nil, fmt.Errorf("unsupported deployment controller type: %s", dc.Type)
		}
	}
	return defaultFunc, nil
}

func (d *App) findTaskDefinitionOfDeployment(ctx context.Context, dp *cdTypes.DeploymentInfo) (string, error) {
	resRev, err := d.codedeploy.GetApplicationRevision(ctx, &codedeploy.GetApplicationRevisionInput{
		ApplicationName: dp.ApplicationName,
		Revision:        dp.Revision,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get application revision: %w", err)
	}
	spec, err := appspec.Unmarsal([]byte(*resRev.Revision.AppSpecContent.Content))
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal appspec: %w", err)
	}
	return *spec.Resources[0].TargetService.Properties.TaskDefinition, nil
}

func (d *App) waitForCodeDeployRollback(ctx context.Context, id string) error {
	p := retry.Policy{
		MinDelay: time.Second,
		MaxDelay: 10 * time.Second,
		MaxCount: 10,
	}
	return p.Do(ctx, func() error {
		out, err := d.codedeploy.GetDeployment(ctx, &codedeploy.GetDeploymentInput{
			DeploymentId: aws.String(id),
		})
		if err != nil {
			return fmt.Errorf("failed to get deployment: %w", err)
		}
		status := out.DeploymentInfo.Status
		rbinfo := out.DeploymentInfo.RollbackInfo
		if status == cdTypes.DeploymentStatusStopped && rbinfo != nil && rbinfo.RollbackDeploymentId != nil {
			d.LogInfo("deployment stopped", "deployment_id", id)
			d.LogInfo("rollback deployment created", "deployment_id", *rbinfo.RollbackDeploymentId)
			return nil
		}
		return fmt.Errorf("deployment %s is not stopped yet", id)
	})
}

func (d *App) findActiveECSDeploymentArn(ctx context.Context, timeout time.Duration, afterStartedAt bool) (string, error) {
	d.LogDebug("finding active ECS service deployment...")
	tm := time.NewTimer(timeout)
	defer tm.Stop()
	activeDeployments := make([]types.ServiceDeploymentBrief, 0)
	for {
		resp, err := d.ecs.ListServiceDeployments(ctx, &ecs.ListServiceDeploymentsInput{
			Cluster: &d.Cluster,
			Service: &d.Service,
			Status: []types.ServiceDeploymentStatus{
				types.ServiceDeploymentStatusInProgress,
			},
		})
		if err != nil {
			return "", fmt.Errorf("failed to list service deployments: %w", err)
		}
		d.LogDebug("found %d service deployments", len(resp.ServiceDeployments))
		for _, sd := range resp.ServiceDeployments {
			d.LogInfo("service deployment",
				"arn", arnToName(aws.ToString(sd.ServiceDeploymentArn)),
				"created_at", sd.CreatedAt,
				"status", sd.Status,
			)
		}
		if afterStartedAt {
			activeDeployments = append(activeDeployments,
				lo.Filter(resp.ServiceDeployments, func(item types.ServiceDeploymentBrief, _ int) bool {
					return item.CreatedAt.After(d.startedAt)
				})...,
			)
		} else {
			activeDeployments = append(activeDeployments, resp.ServiceDeployments...)
		}
		if len(activeDeployments) > 0 {
			d.LogDebug("found %d active service deployments", len(activeDeployments))
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-tm.C: // Timeout reached
			return "", fmt.Errorf("no active service deployments found: %w", ErrNotFound)
		default:
			d.LogInfo("no service deployments found, waiting...")
			sleepContext(ctx, delayForServiceChanged)
		}
	}

	// Find the most recent active deployment
	deployment := lo.MaxBy(activeDeployments, func(item types.ServiceDeploymentBrief, max types.ServiceDeploymentBrief) bool {
		return item.CreatedAt.After(*max.CreatedAt)
	})
	d.LogInfo("found deployment",
		"arn", arnToName(aws.ToString(deployment.ServiceDeploymentArn)),
		"created_at", deployment.CreatedAt,
		"status", deployment.Status,
	)
	return aws.ToString(deployment.ServiceDeploymentArn), nil
}

func (d *App) rollbackActiveECSDeployment(ctx context.Context, sv *Service, deploymentArn string, opt RollbackOption) (*rollbackResult, error) {
	currentTaskDefinition := aws.ToString(sv.TaskDefinition)
	res := &rollbackResult{taskDefinitionArn: currentTaskDefinition, deploymentArn: deploymentArn}

	// Check if the deployment is paused at a lifecycle hook
	hookId, err := d.findPausedLifecycleHook(ctx, deploymentArn)
	if err != nil {
		return nil, err
	}

	if hookId != "" {
		d.LogInfo("deployment is paused, rolling back via ContinueServiceDeployment",
			withDryRun(opt.DryRun, "deployment", arnToName(deploymentArn), "hook_id", hookId)...)
		if opt.DryRun {
			return res, nil
		}
		if _, err := d.ecs.ContinueServiceDeployment(ctx, &ecs.ContinueServiceDeploymentInput{
			ServiceDeploymentArn: &deploymentArn,
			HookId:               &hookId,
			Action:               types.DeploymentLifecycleHookActionRollback,
		}); err != nil {
			return nil, fmt.Errorf("failed to rollback paused deployment: %w", err)
		}
		d.LogInfo("Rollback triggered successfully")
		return res, nil
	}

	d.LogInfo("stopping deployment with rollback", withDryRun(opt.DryRun, "deployment", arnToName(deploymentArn))...)
	if opt.DryRun {
		return res, nil
	}
	if _, err := d.ecs.StopServiceDeployment(ctx, &ecs.StopServiceDeploymentInput{
		ServiceDeploymentArn: &deploymentArn,
		StopType:             types.StopServiceDeploymentStopTypeRollback,
	}); err != nil {
		return nil, fmt.Errorf("failed to stop service deployment: %w", err)
	}
	d.LogInfo("Rollback triggered successfully")
	return res, nil
}

func (d *App) findPausedLifecycleHook(ctx context.Context, deploymentArn string) (string, error) {
	resp, err := d.ecs.DescribeServiceDeployments(ctx, &ecs.DescribeServiceDeploymentsInput{
		ServiceDeploymentArns: []string{deploymentArn},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe service deployments: %w", err)
	}
	if len(resp.ServiceDeployments) == 0 {
		return "", nil
	}
	for _, hook := range resp.ServiceDeployments[0].LifecycleHookDetails {
		if hook.TargetType == types.DeploymentLifecycleHookTargetTypePause &&
			hook.Status == types.DeploymentLifecycleHookStatusAwaitingAction {
			return aws.ToString(hook.HookId), nil
		}
	}
	return "", nil
}
