package ecspresso

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/fatih/color"
	"github.com/pkg/errors"
)

func (d *App) Rollback(opt RollbackOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	if aws.BoolValue(opt.DeregisterTaskDefinition) && aws.BoolValue(opt.NoWait) {
		fmt.Fprintln(
			os.Stderr,
			color.YellowString("WARNING: --deregister-task-definition not works with --no-wait together"),
		)
	}

	d.Log("Starting rollback", opt.DryRunString())
	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return errors.Wrap(err, "failed to describe service status")
	}

	currentArn := *sv.TaskDefinition
	targetArn, err := d.FindRollbackTarget(ctx, currentArn)
	if err != nil {
		return errors.Wrap(err, "failed to find rollback target")
	}

	if isCodeDeploy(sv.DeploymentController) {
		return d.RollbackByCodeDeploy(ctx, sv, targetArn, opt)
	}

	d.Log("Rolling back to", arnToName(targetArn))
	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	if err := d.UpdateServiceTasks(
		ctx,
		targetArn,
		nil,
		DeployOption{
			ForceNewDeployment: aws.Bool(false),
			UpdateService:      aws.Bool(false),
		},
	); err != nil {
		return errors.Wrap(err, "failed to update service")
	}

	if *opt.NoWait {
		d.Log("Service is rolled back.")
		return nil
	}

	time.Sleep(delayForServiceChanged) // wait for service updated
	if err := d.WaitServiceStable(ctx, time.Now()); err != nil {
		return errors.Wrap(err, "failed to wait service stable")
	}

	d.Log("Service is stable now. Completed!")

	if *opt.DeregisterTaskDefinition {
		d.Log("Deregistering the rolled-back task definition", arnToName(currentArn))
		_, err := d.ecs.DeregisterTaskDefinitionWithContext(
			ctx,
			&ecs.DeregisterTaskDefinitionInput{
				TaskDefinition: &currentArn,
			},
		)
		if err != nil {
			return errors.Wrap(err, "failed to deregister task definition")
		}
		d.Log(arnToName(currentArn), "was deregistered successfully")
	}

	return nil
}
