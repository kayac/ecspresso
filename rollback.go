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
	"github.com/shogo82148/go-retry"
)

type RollbackOption struct {
	DryRun                   bool   `help:"dry run" default:"false"`
	DeregisterTaskDefinition bool   `help:"deregister the rolled-back task definition. not works with --no-wait" default:"true" negatable:""`
	Wait                     bool   `help:"wait for the service stable" default:"true" negatable:""`
	RollbackEvents           string `help:"roll back when specified events happened (DEPLOYMENT_FAILURE,DEPLOYMENT_STOP_ON_ALARM,DEPLOYMENT_STOP_ON_REQUEST,...) CodeDeploy only." default:""`
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

	d.Log("Starting rollback %s", opt.DryRunString())
	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return err
	}

	d.Log("deployment controller: %s", sv.DeploymentController.Type)
	doRollback, err := d.RollbackFunc(sv)
	if err != nil {
		return err
	}

	doWait, err := d.WaitFunc(sv)
	if err != nil {
		return err
	}

	// doRollback returns the task definition arn to be rolled back
	rollbackedTdArn, err := doRollback(ctx, sv, opt)
	if err != nil {
		return err
	}

	if opt.DryRun {
		if err := d.rollbackTaskDefinition(ctx, rollbackedTdArn, opt); err != nil {
			return err
		}
		d.Log("DRY RUN OK")
		return nil
	}

	if !opt.Wait {
		d.Log("Service is rolled back.")
		return nil
	}

	time.Sleep(delayForServiceChanged) // wait for service updated
	if err := doWait(ctx, sv); err != nil {
		if errors.As(err, &errNotFound) {
			d.Log("[INFO] %s", err)
			return d.rollbackTaskDefinition(ctx, rollbackedTdArn, opt)
		}
		return err
	}

	d.Log("Service is stable now. Completed!")

	return d.rollbackTaskDefinition(ctx, rollbackedTdArn, opt)
}

func (d *App) rollbackTaskDefinition(ctx context.Context, rollbackedTdArn string, opt RollbackOption) error {
	if !opt.DeregisterTaskDefinition {
		return nil
	}
	if opt.DryRun {
		d.Log("task definition %s will be deregistered", arnToName(rollbackedTdArn))
		return nil
	}

	d.Log("Deregistering the rolled-back task definition %s", arnToName(rollbackedTdArn))
	_, err := d.ecs.DeregisterTaskDefinition(
		ctx,
		&ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: &rollbackedTdArn,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to deregister task definition: %w", err)
	}
	d.Log("%s was deregistered successfully", arnToName(rollbackedTdArn))
	return nil
}

func (d *App) RollbackServiceTasks(ctx context.Context, sv *Service, opt RollbackOption) (string, error) {
	currentArn := *sv.TaskDefinition
	targetArn, err := d.FindRollbackTarget(ctx, currentArn)
	if err != nil {
		return "", err
	}

	d.Log("Rolling back to %s %s", arnToName(targetArn), opt.DryRunString())
	if opt.DryRun {
		return currentArn, nil
	}

	if err := d.UpdateServiceTasks(
		ctx,
		targetArn,
		nil,
		sv,
		DeployOption{
			ForceNewDeployment: false,
			UpdateService:      false,
		},
	); err != nil {
		return "", err
	}
	return currentArn, nil
}

func (d *App) RollbackByCodeDeploy(ctx context.Context, sv *Service, opt RollbackOption) (string, error) {
	dp, err := d.findDeploymentInfo(ctx)
	if err != nil {
		return "", err
	}

	ld, err := d.codedeploy.ListDeployments(ctx, &codedeploy.ListDeploymentsInput{
		ApplicationName:     dp.ApplicationName,
		DeploymentGroupName: dp.DeploymentGroupName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list deployments: %w", err)
	}
	if len(ld.Deployments) == 0 {
		return "", ErrNotFound("no deployments are found")
	}

	out, err := d.codedeploy.GetDeployment(ctx, &codedeploy.GetDeploymentInput{
		DeploymentId: &ld.Deployments[0], // latest deployment
	})
	if err != nil {
		return "", fmt.Errorf("failed to get deployment: %w", err)
	}
	currentDeployment := out.DeploymentInfo

	d.Log("current deployment id: %s", *currentDeployment.DeploymentId)

	switch currentDeployment.Status {
	case cdTypes.DeploymentStatusSucceeded, cdTypes.DeploymentStatusFailed, cdTypes.DeploymentStatusStopped:
		currentTdArn := *sv.TaskDefinition
		targetArn, err := d.FindRollbackTarget(ctx, currentTdArn)
		if err != nil {
			return "", err
		}
		d.Log("the deployment in progress is not found, creating a new deployment with %s %s", targetArn, opt.DryRunString())
		if opt.DryRun {
			return currentTdArn, nil
		}
		if err := d.createDeployment(ctx, sv, targetArn, opt.RollbackEvents); err != nil {
			return "", err
		}
		return currentTdArn, nil
	default: // If the deployment is not yet complete
		d.Log("the deployment in progress found, stopping the deployment %s %s", *currentDeployment.DeploymentId, opt.DryRunString())
		tdArn, err := d.findTaskDefinitionOfDeployment(ctx, currentDeployment)
		if err != nil {
			return "", err
		}
		if opt.DryRun {
			return tdArn, nil
		}
		if _, err := d.codedeploy.StopDeployment(ctx, &codedeploy.StopDeploymentInput{
			DeploymentId:        currentDeployment.DeploymentId,
			AutoRollbackEnabled: aws.Bool(true),
		}); err != nil {
			return "", fmt.Errorf("failed to roll back the deployment: %w", err)
		}
		if err := d.waitForDeploymentRollbacked(ctx, *currentDeployment.DeploymentId); err != nil {
			return "", err
		}
		return tdArn, nil
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
			return "", fmt.Errorf("failed to list taskdefinitions: %w", err)
		}
		if len(out.TaskDefinitionArns) == 0 {
			return "", ErrNotFound(fmt.Sprintf("rollback target is not found: %s", err))
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
	return "", ErrNotFound("rollback target is not found")
}

type rollbackFunc func(ctx context.Context, sv *Service, opt RollbackOption) (string, error)

func (d *App) RollbackFunc(sv *Service) (rollbackFunc, error) {
	defaultFunc := d.RollbackServiceTasks
	if sv == nil || sv.DeploymentController == nil {
		return defaultFunc, nil
	}
	if dc := sv.DeploymentController; dc != nil {
		switch dc.Type {
		case types.DeploymentControllerTypeCodeDeploy:
			return d.RollbackByCodeDeploy, nil
		case types.DeploymentControllerTypeEcs:
			return d.RollbackServiceTasks, nil
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

func (d *App) waitForDeploymentRollbacked(ctx context.Context, id string) error {
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
			d.Log("Deployment %s is stopped", id)
			d.Log("Rollback deployment created: %s", *rbinfo.RollbackDeploymentId)
			return nil
		}
		return fmt.Errorf("deployment %s is not stopped yet", id)
	})
}
