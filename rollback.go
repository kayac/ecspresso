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
)

type RollbackOption struct {
	DryRun                   *bool
	DeregisterTaskDefinition *bool
	NoWait                   *bool
	RollbackEvents           *string
}

func (opt RollbackOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

func (d *App) Rollback(ctx context.Context, opt RollbackOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	if aws.ToBool(opt.DeregisterTaskDefinition) && aws.ToBool(opt.NoWait) {
		return fmt.Errorf("--deregister-task-definition not works with --no-wait together. Please use --no-deregister-task-definition with --no-wait")
	}

	d.Log("Starting rollback %s", opt.DryRunString())
	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return err
	}

	currentArn := *sv.TaskDefinition
	targetArn, err := d.FindRollbackTarget(ctx, currentArn)
	if err != nil {
		return err
	}

	d.Log("Rolling back to %s", arnToName(targetArn))
	if *opt.DryRun {
		if *opt.DeregisterTaskDefinition {
			d.Log("%s will be deregistered", arnToName(currentArn))
		} else {
			d.Log("%s will not be deregistered", arnToName(currentArn))
		}
		d.Log("DRY RUN OK")
		return nil
	}

	d.Log("deployment controller: %s", sv.DeploymentController.Type)
	if sv.DeploymentController != nil && sv.DeploymentController.Type == types.DeploymentControllerTypeCodeDeploy {
		return d.RollbackByCodeDeploy(ctx, sv, targetArn, opt)
	}

	if err := d.UpdateServiceTasks(
		ctx,
		targetArn,
		nil,
		sv,
		DeployOption{
			ForceNewDeployment: aws.Bool(false),
			UpdateService:      aws.Bool(false),
		},
	); err != nil {
		return err
	}

	if *opt.NoWait {
		d.Log("Service is rolled back.")
		return nil
	}

	time.Sleep(delayForServiceChanged) // wait for service updated
	if err := d.WaitServiceStable(ctx, time.Now()); err != nil {
		return err
	}

	d.Log("Service is stable now. Completed!")

	if *opt.DeregisterTaskDefinition {
		d.Log("Deregistering the rolled-back task definition %s", arnToName(currentArn))
		_, err := d.ecs.DeregisterTaskDefinition(
			ctx,
			&ecs.DeregisterTaskDefinitionInput{
				TaskDefinition: &currentArn,
			},
		)
		if err != nil {
			return fmt.Errorf("failed to deregister task definition: %w", err)
		}
		d.Log("%s was deregistered successfully", arnToName(currentArn))
	}

	return nil
}

func (d *App) RollbackByCodeDeploy(ctx context.Context, sv *Service, tdArn string, opt RollbackOption) error {
	dp, err := d.findDeploymentInfo(ctx)
	if err != nil {
		return err
	}

	ld, err := d.codedeploy.ListDeployments(ctx, &codedeploy.ListDeploymentsInput{
		ApplicationName:     dp.ApplicationName,
		DeploymentGroupName: dp.DeploymentGroupName,
	})
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}
	if len(ld.Deployments) == 0 {
		return ErrNotFound("no deployments are found")
	}

	dpID := ld.Deployments[0] // latest deployment id

	dep, err := d.codedeploy.GetDeployment(ctx, &codedeploy.GetDeploymentInput{
		DeploymentId: &dpID,
	})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	if *opt.DryRun {
		d.Log("deployment id: %s", dpID)
		d.Log("DRY RUN OK")
		return nil
	}

	switch dep.DeploymentInfo.Status {
	case cdTypes.DeploymentStatusSucceeded, cdTypes.DeploymentStatusFailed, cdTypes.DeploymentStatusStopped:
		return d.createDeployment(ctx, sv, tdArn, opt.RollbackEvents)
	default: // If the deployment is not yet complete
		_, err = d.codedeploy.StopDeployment(ctx, &codedeploy.StopDeploymentInput{
			DeploymentId:        &dpID,
			AutoRollbackEnabled: aws.Bool(true),
		})
		if err != nil {
			return fmt.Errorf("failed to roll back the deployment: %w", err)
		}

		d.Log("Deployment %s is rolled back on CodeDeploy:", dpID)
		return nil
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
		nextToken = out.NextToken
		for _, tdArn := range out.TaskDefinitionArns {
			if found {
				return tdArn, nil
			}
			if tdArn == taskDefinitionArn {
				found = true
			}
		}
	}
}
