package ecspresso

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codedeploy"
	cdTypes "github.com/aws/aws-sdk-go-v2/service/codedeploy/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/schollz/progressbar/v3"
)

type waitUntil string

const (
	waitUntilStable           waitUntil = "stable"
	waitUntilDeployed         waitUntil = "deployed"
	waitUntilPaused           waitUntil = "paused"
	waitUntilCodeDeployPrefix           = "codedeploy:"
)

func (u waitUntil) Validate() error {
	if u.forECSDeployment() {
		return nil
	}
	if u.forCodeDeployLifecycle() && len(u) > len(waitUntilCodeDeployPrefix) {
		return nil
	}
	return fmt.Errorf("invalid waitUntil value: %s (expected: stable, deployed, paused, or codedeploy:*)", u)
}

func (u waitUntil) forECSDeployment() bool {
	return u == waitUntilStable || u == waitUntilDeployed || u == waitUntilPaused
}

func (u waitUntil) forCodeDeployLifecycle() bool {
	return strings.HasPrefix(string(u), waitUntilCodeDeployPrefix)
}

func (u waitUntil) doneMessage() string {
	switch {
	case u.forECSDeployment():
		if u == waitUntilPaused {
			return "service deployment paused"
		}
		return "service deployment completed"
	case u.forCodeDeployLifecycle():
		return fmt.Sprintf("CodeDeploy lifecycle event %s completed", u.codeDeployLifecycleEvent())
	default:
		return "service deployment completed"
	}
}

func (u waitUntil) codeDeployLifecycleEvent() string {
	if u.forCodeDeployLifecycle() {
		return strings.TrimPrefix(string(u), waitUntilCodeDeployPrefix)
	}
	return ""
}

type waitFunc func(ctx context.Context, sv *Service) error

type confirmFunc func(ctx context.Context) error

func (confirm confirmFunc) wrap(wait waitFunc) waitFunc {
	if confirm == nil {
		return wait
	}
	return func(ctx context.Context, sv *Service) error {
		if err := wait(ctx, sv); err != nil {
			return err
		}
		return confirm(ctx)
	}
}

func (d *App) WaitFunc(sv *Service, confirm confirmFunc, until waitUntil, deploymentArn ...string) (waitFunc, error) {
	defaultFunc := confirm.wrap(d.WaitServiceStable)
	if sv == nil || sv.DeploymentController == nil {
		return defaultFunc, nil
	}
	var knownArn string
	if len(deploymentArn) > 0 {
		knownArn = deploymentArn[0]
	}
	if dc := sv.DeploymentController; dc != nil {
		switch dc.Type {
		case types.DeploymentControllerTypeCodeDeploy:
			if until.forCodeDeployLifecycle() {
				return d.WaitForCodeDeployLifecycle(until.codeDeployLifecycleEvent()), nil
			}
			return d.WaitForCodeDeploy, nil
		case types.DeploymentControllerTypeEcs:
			switch until {
			case waitUntilDeployed:
				return confirm.wrap(func(ctx context.Context, sv *Service) error {
					return d.waitServiceDeployment(ctx, knownArn, false)
				}), nil
			case waitUntilPaused:
				return func(ctx context.Context, sv *Service) error {
					return d.waitServiceDeployment(ctx, knownArn, true)
				}, nil
			case waitUntilStable, "":
				return defaultFunc, nil
			default:
				return nil, fmt.Errorf("unsupported waitUntil: %s", until)
			}
		default:
			return nil, fmt.Errorf("unsupported deployment controller type: %s", dc.Type)
		}
	}
	return defaultFunc, nil
}

func (d *App) confirmPrimaryTD(tdArn string) confirmFunc {
	return func(ctx context.Context) error {
		sv, err := d.DescribeService(ctx)
		if err != nil {
			return err
		}
		if dp, ok := sv.PrimaryDeployment(); ok {
			current := aws.ToString(dp.TaskDefinition)
			d.LogDebug("checking primary deployment %s %s == %s", *dp.Id, current, tdArn)
			if arnToName(current) != arnToName(tdArn) {
				return fmt.Errorf("task definition %s is not deployed yet. PRIMARY deployment is %s", tdArn, current)
			}
			d.LogDebug("task definition %s is deployed", tdArn)
			return nil
		}
		return fmt.Errorf("no primary deployment found")
	}
}

type WaitOption struct {
	WaitUntil string `aliases:"until" help:"Choose whether to wait for service stable, the deployment finishes, or a lifecycle hook pauses. (stable|deployed|paused)" default:"stable" enum:"stable,deployed,paused"`
}

func (d *App) Wait(ctx context.Context, opt WaitOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	until := waitUntil(opt.WaitUntil)
	d.LogInfo("waiting for service", "until", string(until))

	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return err
	}
	d.LogJSON(sv.DeploymentController)
	doWait, err := d.WaitFunc(sv, nil, until)
	if err != nil {
		return err
	}
	if err := doWait(ctx, sv); err != nil {
		if errors.Is(err, ErrNotFound) && sv.isCodeDeploy() {
			d.LogInfo(err.Error())
			return d.WaitTaskSetStable(ctx, sv)
		}
		return err
	}

	d.LogInfo(until.doneMessage())
	return nil
}

func (d *App) WaitServiceStable(ctx context.Context, sv *Service) error {
	d.LogInfo("Waiting for service stable...(it will take a few minutes)")
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tick := time.NewTicker(refreshInterval)
	defer tick.Stop()
	st := &showState{lastEventAt: time.Now()}
	go func() {
		for {
			select {
			case <-waitCtx.Done():
				return
			case <-tick.C:
				if err := d.showServiceStatus(waitCtx, st); err != nil {
					d.LogWarn(err.Error())
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
		d.LogWarn(err.Error())
	}
	return nil
}

func serviceRevisionsSummaries(dp *types.ServiceDeployment) []string {
	lines := []string{}
	revs := []struct {
		types.ServiceRevisionSummary
		revType string
	}{}
	if dp.TargetServiceRevision != nil {
		revs = append(revs, struct {
			types.ServiceRevisionSummary
			revType string
		}{*dp.TargetServiceRevision, "TARGET"})
	}

	for _, rev := range dp.SourceServiceRevisions {
		revs = append(revs, struct {
			types.ServiceRevisionSummary
			revType string
		}{rev, "SOURCE"})
	}
	for _, rev := range revs {
		if rev.RequestedProductionTrafficWeight == nil && rev.RequestedTestTrafficWeight == nil {
			// rolling  ECS deployment without traffic shifting
			lines = append(lines,
				fmt.Sprintf("%s %s pending:%d running:%d",
					rev.revType, arnToName(*rev.Arn), rev.PendingTaskCount, rev.RunningTaskCount,
				),
			)
		} else {
			lines = append(lines,
				fmt.Sprintf("%s %s pending:%d running:%d production:%.1f%% test:%.1f%%",
					rev.revType, arnToName(*rev.Arn), rev.PendingTaskCount, rev.RunningTaskCount,
					aws.ToFloat64(rev.RequestedProductionTrafficWeight), aws.ToFloat64(rev.RequestedTestTrafficWeight),
				),
			)
		}
	}
	return lines
}

func (d *App) waitServiceDeployment(ctx context.Context, knownDeploymentArn string, waitForPause bool) error {
	if waitForPause {
		d.LogInfo("Waiting for service deployment paused...(it will take a few minutes)")
	} else {
		d.LogInfo("Waiting for service deployed...(it will take a few minutes)")
	}
	deploymentArn := knownDeploymentArn
	if deploymentArn == "" {
		var err error
		deploymentArn, err = d.findActiveECSDeploymentArn(ctx, time.Second*10, true)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				d.LogInfo("No active deployment found")
				return nil
			}
			return err
		}
	}
	d.LogInfo("waiting for service deployment", "deployment", arnToName(deploymentArn))

	if waitForPause {
		initResp, err := d.ecs.DescribeServiceDeployments(ctx, &ecs.DescribeServiceDeploymentsInput{
			ServiceDeploymentArns: []string{deploymentArn},
		})
		if err == nil && len(initResp.ServiceDeployments) > 0 {
			if dc := initResp.ServiceDeployments[0].DeploymentConfiguration; dc != nil && dc.Strategy == types.DeploymentStrategyRolling {
				d.LogWarn("--wait-until=paused is not effective for rolling deployments. Pause lifecycle hooks are only supported with blue/green, linear, and canary strategies. Falling back to waiting for deployment completion.")
			}
		}
	}

	tick := time.NewTicker(refreshInterval)
	defer tick.Stop()
	st := &showState{lastEventAt: time.Now()}
	var prevStatus types.ServiceDeploymentStatus
	var prevRevisionSummaryOutput string
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		if err := d.showServiceStatus(ctx, st); err != nil {
			d.LogWarn(err.Error())
			continue
		}

		resp, err := d.ecs.DescribeServiceDeployments(ctx, &ecs.DescribeServiceDeploymentsInput{
			ServiceDeploymentArns: []string{deploymentArn},
		})
		if err != nil {
			return fmt.Errorf("failed to describe service deployments: %w", err)
		}
		if len(resp.ServiceDeployments) == 0 {
			return fmt.Errorf("service deployment not found: %s: %w", deploymentArn, ErrNotFound)
		}
		dp := resp.ServiceDeployments[0]

		// show service revision summaries
		lines := serviceRevisionsSummaries(&dp)
		revisionSummaryOutput := strings.Join(lines, "\n")
		if revisionSummaryOutput != prevRevisionSummaryOutput {
			for _, line := range lines {
				d.LogInfo(line)
			}
			prevRevisionSummaryOutput = revisionSummaryOutput
		}

		// check if a pause lifecycle hook is awaiting action
		if waitForPause {
			for _, hook := range dp.LifecycleHookDetails {
				if hook.TargetType == types.DeploymentLifecycleHookTargetTypePause &&
					hook.Status == types.DeploymentLifecycleHookStatusAwaitingAction {
					d.LogInfo("deployment paused at lifecycle hook",
						"hook_id", aws.ToString(hook.HookId),
						"lifecycle_stage", string(dp.LifecycleStage),
					)
					return nil
				}
			}
		}

		// check deployment status
		status := dp.Status
		if status != prevStatus {
			d.LogInfo("service deployment status", "status", string(status))
			prevStatus = status
		}
		switch status {
		case types.ServiceDeploymentStatusSuccessful, types.ServiceDeploymentStatusRollbackSuccessful:
			d.LogInfo("service deployment completed", "status", string(status))
			return nil
		case types.ServiceDeploymentStatusStopped, types.ServiceDeploymentStatusRollbackFailed, types.ServiceDeploymentStatusStopRequested:
			return fmt.Errorf("Service deployment failed %s", status)
		default:
			d.LogDebug("Deployment %s, waiting...", status)
		}
	}
}

func (d *App) WaitServiceDeployCompleted(ctx context.Context, sv *Service) error {
	return d.waitServiceDeployment(ctx, "", false)
}

func (d *App) WaitServiceDeployPaused(ctx context.Context, sv *Service) error {
	return d.waitServiceDeployment(ctx, "", true)
}

func (d *App) getCodeDeployDeploymentID(ctx context.Context) (string, error) {
	dp, err := d.findDeploymentInfo(ctx)
	if err != nil {
		return "", err
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
		return "", err
	}
	if len(out.Deployments) == 0 {
		return "", fmt.Errorf("No deployments found in progress on CodeDeploy: %w", ErrNotFound)
	}

	return out.Deployments[0], nil
}

func (d *App) WaitForCodeDeploy(ctx context.Context, sv *Service) error {
	d.LogDebug("wait for CodeDeploy")
	dpID, err := d.getCodeDeployDeploymentID(ctx)
	if err != nil {
		return err
	}
	d.LogInfo("waiting for deployment success", "deployment_id", dpID)
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

func (d *App) WaitForCodeDeployLifecycle(targetLifecycleEvent string) waitFunc {
	return func(ctx context.Context, sv *Service) error {
		d.LogDebug("wait for CodeDeploy lifecycle event: %s", targetLifecycleEvent)
		dpID, err := d.getCodeDeployDeploymentID(ctx)
		if err != nil {
			return err
		}
		d.LogInfo("waiting for deployment lifecycle event", "event", targetLifecycleEvent, "deployment_id", dpID)

		t := time.NewTicker(refreshInterval)
		defer t.Stop()
		lifecycle2Status := map[string]cdTypes.LifecycleEventStatus{}
		targetEventFound := false

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
			}

			target, err := d.codedeploy.GetDeploymentTarget(ctx, &codedeploy.GetDeploymentTargetInput{
				DeploymentId: &dpID,
				TargetId:     aws.String(d.Cluster + ":" + d.Service),
			})
			if err != nil {
				d.LogWarn(err.Error())
				continue
			}

			dep := target.DeploymentTarget
			d.LogDebug("deployment target status: %s", dep.EcsTarget.Status)

			// Check lifecycle events
			for _, ev := range dep.EcsTarget.LifecycleEvents {
				lifecycleEvent := *ev.LifecycleEventName
				if lifecycle2Status[lifecycleEvent] != ev.Status {
					if ev.Status != cdTypes.LifecycleEventStatusPending {
						d.LogInfo("lifecycle event status", "event", lifecycleEvent, "status", string(ev.Status))
					}
					lifecycle2Status[lifecycleEvent] = ev.Status
				}

				if lifecycleEvent == targetLifecycleEvent {
					targetEventFound = true
					switch ev.Status {
					case cdTypes.LifecycleEventStatusSucceeded:
						d.LogInfo("lifecycle event completed", "event", targetLifecycleEvent)
						return nil
					case cdTypes.LifecycleEventStatusFailed:
						return fmt.Errorf("lifecycle event %s failed", targetLifecycleEvent)
					case cdTypes.LifecycleEventStatusSkipped:
						d.LogInfo("lifecycle event skipped", "event", targetLifecycleEvent)
						return nil
					default:
						// NOP for "Pending", "InProgress", and "Unknown"
						d.LogDebug("Lifecycle event %s is ignored", ev.Status)
					}
				}
			}

			// Check deployment status
			if dep.EcsTarget.Status != cdTypes.TargetStatusInProgress {
				if !targetEventFound {
					return fmt.Errorf("lifecycle event %s not found in deployment", targetLifecycleEvent)
				}
				d.LogInfo("deployment completed, lifecycle event incomplete", "event", targetLifecycleEvent)
				return nil
			}
		}
	}
}

type showState struct {
	lastEventAt     time.Time
	deploymentsHash []byte
}

func (d *App) showServiceStatus(ctx context.Context, st *showState) error {
	out, err := d.ecs.DescribeServices(ctx, d.DescribeServicesInput())
	if err != nil {
		return fmt.Errorf("failed to describe services: %w", err)
	}
	if len(out.Services) == 0 {
		return fmt.Errorf("service %s is not found: %w", d.Service, ErrNotFound)
	}
	sv := out.Services[0]

	// show events
	sort.SliceStable(sv.Events, func(i, j int) bool {
		return sv.Events[i].CreatedAt.Before(*sv.Events[j].CreatedAt)
	})
	for _, event := range sv.Events {
		if (*event.CreatedAt).After(st.lastEventAt) {
			WriteOutput(serviceEvent(event))
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
			d.LogInfo(line)
		}
	}
	st.deploymentsHash = hash
	return nil
}

func (d *App) codeDeployProgressBar(ctx context.Context, dpID string) error {
	opts := []progressbar.Option{
		progressbar.OptionSetDescription("Traffic shifted"),
		progressbar.OptionSetWidth(20),
	}
	if logFormat == logFormatJSON {
		// disable progress bar in JSON format
		opts = append(opts, progressbar.OptionSetWriter(io.Discard))
	} else {
		opts = append(opts, progressbar.OptionSetWriter(os.Stdout))
		defer func() {
			// append new line after progress bar
			os.Stdout.Write([]byte("\n"))
		}()
	}
	bar := progressbar.NewOptions(100, opts...)
	defer bar.Finish()
	t := time.NewTicker(refreshInterval)
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
			d.LogWarn(err.Error())
			continue
		}
		dep := out.DeploymentTarget
		d.LogDebug("status: %s, %s", dep.EcsTarget.Status, *dep.EcsTarget.LastUpdatedAt)
		if dep.EcsTarget.Status != "InProgress" {
			return nil
		}
		for _, ev := range dep.EcsTarget.LifecycleEvents {
			name := *ev.LifecycleEventName
			if lcEvents[name] != ev.Status {
				if ev.Status != cdTypes.LifecycleEventStatusPending {
					d.LogInfo("lifecycle event status", "event", name, "status", string(ev.Status))
				}
				lcEvents[name] = ev.Status
			}
		}
		for _, element := range dep.EcsTarget.TaskSetsInfo {
			d.LogDebug("taskset: %s, %s, %f", element.TaskSetLabel, *element.Status, element.TrafficWeight)
			if *element.Status == "ACTIVE" {
				bar.Set(int(element.TrafficWeight))
			}
		}
	}
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
			d.LogInfo("Waiting task sets available")
		default:
			ts := sv.TaskSets[0]
			if aws.ToString(ts.Status) == "PRIMARY" {
				if prev != ts.StabilityStatus {
					d.LogInfo("waiting for PRIMARY task set stable", "stability_status", string(ts.StabilityStatus))
					if n > 1 {
						d.LogInfo("Waiting a PRIMARY taskset available only")
					}
				}
				if ts.StabilityStatus == types.StabilityStatusSteadyState && n == 1 {
					d.LogInfo("Service is stable now. Completed!")
					return nil
				}
				prev = ts.StabilityStatus
			}
		}
		sleepContext(ctx, 10*time.Second)
	}
}
