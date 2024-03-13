package ecspresso

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	cdTypes "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/schollz/progressbar/v3"
)

type waitFunc func(ctx context.Context, sv *Service) error

func (d *App) WaitFunc(sv *Service) (waitFunc, error) {
	defaultFunc := d.WaitServiceStable
	if sv == nil || sv.DeploymentController == nil {
		return defaultFunc, nil
	}
	if dc := sv.DeploymentController; dc != nil {
		switch dc.Type {
		case types.DeploymentControllerTypeCodeDeploy:
			return d.WaitForCodeDeploy, nil
		case types.DeploymentControllerTypeEcs:
			return d.WaitServiceStable, nil
		default:
			return nil, fmt.Errorf("unsupported deployment controller type: %s", dc.Type)
		}
	}
	return defaultFunc, nil
}

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
	d.LogJSON(sv.DeploymentController)
	doWait, err := d.WaitFunc(sv)
	if err != nil {
		return err
	}
	if err := doWait(ctx, sv); err != nil {
		if errors.As(err, &errNotFound) && sv.isCodeDeploy() {
			d.Log("[INFO] %s", err)
			return d.WaitTaskSetStable(ctx, sv)
		}
		return err
	}

	d.Log("Service is stable now. Completed!")
	return nil
}

func (d *App) WaitServiceStable(ctx context.Context, sv *Service) error {
	d.Log("Waiting for service stable...(it will take a few minutes)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tick := time.NewTicker(10 * time.Second)
	st := &showState{lastEventAt: time.Now()}
	go func() {
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

	waiter := ecs.NewServicesStableWaiter(d.ecs, func(o *ecs.ServicesStableWaiterOptions) {
		o.MaxDelay = waiterMaxDelay
	})
	if err := waiter.Wait(ctx, d.DescribeServicesInput(), d.Timeout()); err != nil {
		return fmt.Errorf("failed to wait for service stable: %w", err)
	}
	cancel() // stop the showServiceStatus

	<-time.After(delayForServiceChanged)
	// show the service status once more (correct all logs)
	if err := d.showServiceStatus(ctx, st); err != nil {
		d.Log("[WARNING] %s", err.Error())
	}
	return nil
}

func (d *App) WaitForCodeDeploy(ctx context.Context, sv *Service) error {
	d.Log("[DEBUG] wait for CodeDeploy")
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
				cdTypes.DeploymentStatusReady,
			},
		},
	)
	if err != nil {
		return err
	}
	if len(out.Deployments) == 0 {
		return ErrNotFound("No deployments found in progress on CodeDeploy")
	}

	dpID := out.Deployments[0]
	d.Log("Waiting for a deployment successful ID: " + dpID)
	go d.codeDeployProgressBar(ctx, dpID)

	waiter := codedeploy.NewDeploymentSuccessfulWaiter(d.codedeploy, func(o *codedeploy.DeploymentSuccessfulWaiterOptions) {
		o.MaxDelay = waiterMaxDelay
	})
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

func (d *App) codeDeployProgressBar(ctx context.Context, dpID string) error {
	bar := progressbar.NewOptions(100,
		progressbar.OptionSetDescription("Traffic shifted"),
		progressbar.OptionSetWidth(20),
	)
	t := time.NewTicker(10 * time.Second)
	lcEvents := map[string]cdTypes.LifecycleEventStatus{}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		out, err := d.codedeploy.GetDeploymentTarget(ctx, &codedeploy.GetDeploymentTargetInput{
			DeploymentId: &dpID,
			TargetId:     aws.String(d.Cluster + ":" + d.Service),
		})
		if err != nil {
			d.Log("[WARNING] %s", err.Error())
			continue
		}
		dep := out.DeploymentTarget
		d.Log("[DEBUG] status: %s, %s", dep.EcsTarget.Status, *dep.EcsTarget.LastUpdatedAt)
		if dep.EcsTarget.Status != "InProgress" {
			break
		}
		for _, ev := range dep.EcsTarget.LifecycleEvents {
			name := *ev.LifecycleEventName
			if lcEvents[name] != ev.Status {
				if ev.Status != cdTypes.LifecycleEventStatusPending {
					d.Log("%s: %s", name, ev.Status)
				}
				lcEvents[name] = ev.Status
			}
		}
		for _, element := range dep.EcsTarget.TaskSetsInfo {
			d.Log("[DEBUG] taskset: %s, %s, %f", element.TaskSetLabel, *element.Status, element.TrafficWeight)
			if *element.Status == "ACTIVE" {
				bar.Set(int(element.TrafficWeight))
			}
		}
	}
	bar.Set(100)
	fmt.Println()
	return nil
}

func (d *App) WaitTaskSetStable(ctx context.Context, sv *Service) error {
	var prev types.StabilityStatus
	for {
		sv, err := d.DescribeService(ctx)
		if err != nil {
			return err
		}
		switch n := len(sv.TaskSets); n {
		case 0:
			d.Log("Waiting task sets available")
		default:
			ts := sv.TaskSets[0]
			if aws.ToString(ts.Status) == "PRIMARY" {
				if prev != ts.StabilityStatus {
					d.Log("Waiting a task set PRIMARY stable: %s", ts.StabilityStatus)
					if n > 1 {
						d.Log("Waiting a PRIMARY taskset available only")
					}
				}
				if ts.StabilityStatus == types.StabilityStatusSteadyState && n == 1 {
					d.Log("Service is stable now. Completed!")
					return nil
				}
				prev = ts.StabilityStatus
			}
		}
		time.Sleep(10 * time.Second)
	}
}
