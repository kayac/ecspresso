package ecspresso

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	cdTypes "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/morikuni/aec"
)

type WaitOption struct {
}

func (d *App) Wait(ctx context.Context, opt WaitOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	d.Log("Waiting for the service stable")

	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return err
	}
	if sv.DeploymentController != nil && sv.DeploymentController.Type == types.DeploymentControllerTypeCodeDeploy {
		err := d.WaitForCodeDeploy(ctx, sv)
		if err != nil {
			return fmt.Errorf("failed to wait for a deployment successfully: %w", err)
		}
	} else {
		if err := d.WaitServiceStable(ctx, time.Now()); err != nil {
			return err
		}
	}
	d.Log("Service is stable now. Completed!")
	return nil
}

func (d *App) WaitServiceStable(ctx context.Context, startedAt time.Time) error {
	d.Log("Waiting for service stable...(it will take a few minutes)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tick := time.NewTicker(10 * time.Second)
	go func() {
		var lines int
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-tick.C:
				if isTerminal {
					for i := 0; i < lines; i++ {
						fmt.Print(aec.EraseLine(aec.EraseModes.All), aec.PreviousLine(1))
					}
				}
				lines, _ = d.DescribeServiceDeployments(waitCtx, startedAt)
			}
		}
	}()

	waiter := ecs.NewServicesStableWaiter(d.ecs)
	if err := waiter.Wait(ctx, d.DescribeServicesInput(), d.Timeout()); err != nil {
		return fmt.Errorf("failed to wait for service stable: %w", err)
	}
	return nil
}

func (d *App) WaitForCodeDeploy(ctx context.Context, sv *Service) error {
	dp, err := d.findDeploymentInfo(ctx)
	if err != nil {
		return err
	}
	out, err := d.codedeploy.ListDeployments(
		ctx,
		&codedeploy.ListDeploymentsInput{
			ApplicationName:     dp.ApplicationName,
			DeploymentGroupName: dp.DeploymentGroupName,
			IncludeOnlyStatuses: []cdTypes.DeploymentStatus{
				cdTypes.DeploymentStatusCreated,
				cdTypes.DeploymentStatusQueued,
				cdTypes.DeploymentStatusInProgress,
			},
		},
	)
	if err != nil {
		return err
	}
	if len(out.Deployments) == 0 {
		return ErrNotFound("no deployments found in progress")
	}
	dpID := out.Deployments[0]
	d.Log("Waiting for a deployment successful ID: " + dpID)
	waiter := codedeploy.NewDeploymentSuccessfulWaiter(d.codedeploy)
	return waiter.Wait(
		ctx,
		&codedeploy.GetDeploymentInput{DeploymentId: &dpID},
		d.Timeout(),
	)
}
