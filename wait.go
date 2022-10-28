package ecspresso

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	cdTypes "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
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
		st := &showState{lastEventAt: startedAt}
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-tick.C:
				if err := d.showServiceStatus(waitCtx, st); err != nil {
					d.Log("[WARNING] %s", err.Error())
					continue
				}
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

type showState struct {
	lastEventAt     time.Time
	deploymentsHash []byte
}

func (d *App) showServiceStatus(ctx context.Context, st *showState) error {
	out, err := d.ecs.DescribeServices(ctx, d.DescribeServicesInput())
	if err != nil {
		return fmt.Errorf("failed to describe service deployments: %w", err)
	}
	if len(out.Services) == 0 {
		return ErrNotFound(fmt.Sprintf("service %s is not found", d.Service))
	}
	sv := out.Services[0]

	// show events
	sort.SliceStable(sv.Events, func(i, j int) bool {
		return sv.Events[i].CreatedAt.Before(*sv.Events[j].CreatedAt)
	})
	for _, event := range sv.Events {
		if (*event.CreatedAt).After(st.lastEventAt) {
			fmt.Println(formatEvent(event))
			st.lastEventAt = *event.CreatedAt
		}
	}

	// show deployments
	h := sha256.New()
	lines := make([]string, 0, len(sv.Deployments))
	for _, dep := range sv.Deployments {
		line := formatDeployment(dep)
		lines = append(lines, line)
		h.Write([]byte(line))
	}
	hash := h.Sum(nil)
	// if the deployments are not changed, do not show the deployments.
	if !bytes.Equal(st.deploymentsHash, hash) {
		for _, line := range lines {
			d.Log(line)
		}
	}
	st.deploymentsHash = hash
	return nil
}
