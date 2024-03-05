package ecspresso

import (
	"context"
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

var noopWaitFunc = func() error { return nil }

type rollbackResult struct {
	waitID            string // deployment id for waiting
	taskDefinitionArn string // Rollbacked task definition arn
	waitFunc          func() error
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

	res, err := doRollback(ctx, sv, opt)
	if err != nil {
		return err
	}

	if !opt.Wait {
		d.Log("Service is rolled back.")
		return nil
	}

	time.Sleep(delayForServiceChanged) // wait for service updated

	if fn := res.waitFunc; fn != nil {
		policy := retry.Policy{
			MinDelay: 1 * time.Second,
			MaxDelay: 5 * time.Second,
			MaxCount: 10,
		}
		if err := policy.Do(ctx, fn); err != nil {
			return err
		}
	} else {
		// wait for service stable
		doWait, err := d.WaitFunc(sv)
		if err != nil {
			return err
		}
		if err := doWait(ctx, sv, res.waitID); err != nil {
			return err
		}
	}

	d.Log("Service is stable now. Completed!" + opt.DryRunString())

	return d.rollbackTaskDefinition(ctx, res.taskDefinitionArn, opt)
}

func (d *App) rollbackTaskDefinition(ctx context.Context, tdArn string, opt RollbackOption) error {
	if !opt.DeregisterTaskDefinition {
		return nil
	}
	d.Log("Deregistering the rolled-back task definition %s %s", tdArn, opt.DryRunString())
	if opt.DryRun {
		return nil
	}
	_, err := d.ecs.DeregisterTaskDefinition(
		ctx,
		&ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String(tdArn),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to deregister task definition: %w", err)
	}
	d.Log("%s was deregistered successfully", arnToName(tdArn))
	return nil
}

func (d *App) RollbackServiceTasks(ctx context.Context, sv *Service, opt RollbackOption) (*rollbackResult, error) {
	currentTdArn := *sv.TaskDefinition
	tdArn, err := d.FindRollbackTarget(ctx, currentTdArn)
	if err != nil {
		return nil, err
	}
	id, err := d.UpdateServiceTasks(
		ctx,
		tdArn,
		nil,
		sv,
		DeployOption{
			ForceNewDeployment: false,
			UpdateService:      false,
		},
	)
	if err != nil {
		return nil, err
	}
	return &rollbackResult{
		waitID:            id,
		taskDefinitionArn: currentTdArn,
	}, nil
}

func (d *App) RollbackByCodeDeploy(ctx context.Context, sv *Service, opt RollbackOption) (*rollbackResult, error) {
	dp, err := d.findDeploymentInfo(ctx)
	if err != nil {
		return nil, err
	}

	ld, err := d.codedeploy.ListDeployments(ctx, &codedeploy.ListDeploymentsInput{
		ApplicationName:     dp.ApplicationName,
		DeploymentGroupName: dp.DeploymentGroupName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	if len(ld.Deployments) == 0 {
		return nil, ErrNotFound("no deployments are found")
	}

	dpID := ld.Deployments[0] // latest deployment id

	dep, err := d.codedeploy.GetDeployment(ctx, &codedeploy.GetDeploymentInput{
		DeploymentId: &dpID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	switch dep.DeploymentInfo.Status {
	// If the deployment is already complete, create a new deployment for rollback
	case cdTypes.DeploymentStatusSucceeded, cdTypes.DeploymentStatusFailed, cdTypes.DeploymentStatusStopped:
		currentTdArn := *sv.TaskDefinition
		tdArn, err := d.FindRollbackTarget(ctx, currentTdArn)
		if err != nil {
			return nil, err
		}
		res := &rollbackResult{
			taskDefinitionArn: currentTdArn,
		}
		d.Log("Deployment in progress is not found. Creating a new deployment using %s %s", tdArn, opt.DryRunString())
		if opt.DryRun {
			res.waitFunc = noopWaitFunc
			return res, nil
		}
		id, err := d.createDeployment(ctx, sv, tdArn, opt.RollbackEvents)
		if err != nil {
			return nil, err
		}
		res.waitID = id
		return res, nil
	default: // If the deployment is not yet complete, stop the deployment
		d.Log("Found deployment in progress, stopping the deployment %s %s", dpID, opt.DryRunString())
		tdArn, err := d.FindRollbackTargetOfDeployment(ctx, dep.DeploymentInfo)
		if err != nil {
			return nil, err
		}
		res := &rollbackResult{
			waitID:            dpID,
			taskDefinitionArn: tdArn,
		}
		if opt.DryRun {
			res.waitFunc = noopWaitFunc
			return res, nil
		}
		_, err = d.codedeploy.StopDeployment(ctx, &codedeploy.StopDeploymentInput{
			DeploymentId:        &dpID,
			AutoRollbackEnabled: aws.Bool(true),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to roll back the deployment: %w", err)
		}
		res.waitFunc = func() error {
			d.Log("Waiting for the deployment to be stopped %s", dpID)
			dp, err := d.codedeploy.GetDeployment(ctx, &codedeploy.GetDeploymentInput{
				DeploymentId: &dpID,
			})
			if err != nil {
				return err
			}
			if dp.DeploymentInfo.Status == cdTypes.DeploymentStatusStopped {
				d.Log("Deployment %s is rolled back on CodeDeploy", dpID)
				return nil
			}
			return fmt.Errorf("deployment is not stopped yet")
		}
		return res, err
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

type rollbackFunc func(ctx context.Context, sv *Service, opt RollbackOption) (*rollbackResult, error)

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

func (d *App) FindRollbackTargetOfDeployment(ctx context.Context, dp *cdTypes.DeploymentInfo) (string, error) {
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
