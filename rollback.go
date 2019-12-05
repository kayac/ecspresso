package ecspresso

import (
	"time"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
)

func (d *App) Rollback(opt RollbackOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	d.Log("Starting rollback")
	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return errors.Wrap(err, "failed to describe service status")
	}

	if sv.DeploymentController != nil && *sv.DeploymentController.Type == "CODE_DEPLOY" {
		return errors.New("could not rollback service using deployment controller CODE_DEPLOY")
	}

	currentArn := *sv.TaskDefinition
	targetArn, err := d.FindRollbackTarget(ctx, currentArn)
	if err != nil {
		return errors.Wrap(err, "failed to find rollback target")
	}
	d.Log("Rollbacking to", arnToName(targetArn))
	if *opt.DryRun {
		d.Log("DRY RUN OK")
		return nil
	}

	f := false // Set ForceNewDeployment to false
	if err := d.UpdateServiceTasks(ctx, targetArn, sv.DesiredCount, DeployOption{ForceNewDeployment: &f}); err != nil {
		return errors.Wrap(err, "failed to update service")
	}

	if *opt.NoWait {
		d.Log("Service is rollbacked.")
		return nil
	}

	time.Sleep(delayForServiceChanged) // wait for service updated
	if err := d.WaitServiceStable(ctx, time.Now()); err != nil {
		return errors.Wrap(err, "failed to wait service stable")
	}

	d.Log("Service is stable now. Completed!")

	if *opt.DeregisterTaskDefinition {
		d.Log("Deregistering rolled back task definition", arnToName(currentArn))
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
