package ecspresso

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
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
	waitUntilCodeDeployPrefix           = "codedeploy:"
	waitUntilECSPrefix                  = "ecs:"
)

func (u waitUntil) Validate() error {
	if u == waitUntilStable || u == waitUntilDeployed {
		return nil
	}
	if strings.HasPrefix(string(u), waitUntilCodeDeployPrefix) && len(u) > len(waitUntilCodeDeployPrefix) {
		return nil
	}
	if strings.HasPrefix(string(u), waitUntilECSPrefix) {
		stage := types.ServiceDeploymentLifecycleStage(u.ecsLifecycleStage())
		if lifecycleStageIndex(stage) < 0 {
			return fmt.Errorf("invalid waitUntil value: %s (expected one of %s)", u, strings.Join(lifecycleStageNames(), ", "))
		}
		return nil
	}
	return fmt.Errorf("invalid waitUntil value: %s (expected: stable, deployed, codedeploy:*, or ecs:*)", u)
}

func (u waitUntil) forCodeDeployLifecycle() bool {
	return strings.HasPrefix(string(u), waitUntilCodeDeployPrefix)
}

func (u waitUntil) codeDeployLifecycleEvent() string {
	if u.forCodeDeployLifecycle() {
		return strings.TrimPrefix(string(u), waitUntilCodeDeployPrefix)
	}
	return ""
}

func (u waitUntil) forECSLifecycleStage() bool {
	return strings.HasPrefix(string(u), waitUntilECSPrefix)
}

func (u waitUntil) ecsLifecycleStage() string {
	if u.forECSLifecycleStage() {
		return strings.TrimPrefix(string(u), waitUntilECSPrefix)
	}
	return ""
}

// lifecycleStages are the deployment lifecycle stages in the order they occur
// during a deployment, as documented in "Deployment lifecycle stages" at
// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/blue-green-deployment-how-it-works.html#blue-green-deployment-stages
// The waiter compares stage positions, so this must not be derived from
// types.ServiceDeploymentLifecycleStage.Values(), whose ordering is documented
// as not guaranteed to be stable across SDK updates. When the SDK introduces a
// new stage (TestLifecycleStageIndex fails), insert it here at the position
// the document above describes.
var lifecycleStages = []types.ServiceDeploymentLifecycleStage{
	types.ServiceDeploymentLifecycleStageReconcileService,
	types.ServiceDeploymentLifecycleStagePreScaleUp,
	types.ServiceDeploymentLifecycleStageScaleUp,
	types.ServiceDeploymentLifecycleStagePostScaleUp,
	types.ServiceDeploymentLifecycleStageTestTrafficShift,
	types.ServiceDeploymentLifecycleStagePostTestTrafficShift,
	types.ServiceDeploymentLifecycleStageProductionTrafficShift,
	types.ServiceDeploymentLifecycleStagePostProductionTrafficShift,
	types.ServiceDeploymentLifecycleStageBakeTime,
	types.ServiceDeploymentLifecycleStageCleanUp,
}

// lifecycleStageIndex returns the position of the stage in the deployment
// lifecycle, or -1 when the stage is unknown (including an empty stage, which
// is what a rolling deployment reports).
func lifecycleStageIndex(stage types.ServiceDeploymentLifecycleStage) int {
	return slices.Index(lifecycleStages, stage)
}

func lifecycleStageNames() []string {
	names := make([]string, 0, len(lifecycleStages))
	for _, s := range lifecycleStages {
		names = append(names, waitUntilECSPrefix+string(s))
	}
	return names
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

func (d *App) WaitFunc(sv *Service, confirm confirmFunc, until waitUntil) (waitFunc, error) {
	defaultFunc := confirm.wrap(d.WaitServiceStable)
	if sv == nil {
		return defaultFunc, nil
	}
	// ECS is the default deployment controller when the service doesn't set
	// one explicitly, so it must behave the same as an explicit ECS controller
	// instead of falling back to the service-stable waiter silently.
	controllerType := types.DeploymentControllerTypeEcs
	if dc := sv.DeploymentController; dc != nil {
		controllerType = dc.Type
	}
	switch controllerType {
	case types.DeploymentControllerTypeCodeDeploy:
		if until.forCodeDeployLifecycle() {
			return d.WaitForCodeDeployLifecycle(until.codeDeployLifecycleEvent()), nil
		}
		if until.forECSLifecycleStage() {
			return nil, fmt.Errorf("unsupported waitUntil: %s (a deployment lifecycle stage is only reported by the ECS deployment controller)", until)
		}
		return d.WaitForCodeDeploy, nil
	case types.DeploymentControllerTypeEcs:
		if until.forECSLifecycleStage() {
			stage := types.ServiceDeploymentLifecycleStage(until.ecsLifecycleStage())
			return d.WaitServiceDeployLifecycleStage(stage), nil
		}
		switch until {
		case waitUntilDeployed:
			return confirm.wrap(d.WaitServiceDeployCompleted), nil
		case waitUntilStable, "":
			return defaultFunc, nil
		default:
			return nil, fmt.Errorf("unsupported waitUntil: %s", until)
		}
	default:
		return nil, fmt.Errorf("unsupported deployment controller type: %s", controllerType)
	}
}

// warnEarlySuccessCriteriaWait warns when waiting for service stable defeats
// early success criteria. The service-stable waiter waits until all tasks are
// running and the previous tasks are removed, so the deployment completing
// early gains nothing. It reports whether a warning was logged.
func (d *App) warnEarlySuccessCriteriaWait(sv *Service, until waitUntil) bool {
	if sv == nil || sv.isCodeDeploy() || !sv.earlySuccessCriteriaEnabled() {
		return false
	}
	switch until {
	case waitUntilStable, "":
		d.LogWarn("early success criteria is enabled but waiting for service stable; the wait continues until all tasks are running and the previous tasks are removed, use --wait-until=deployed to return when the deployment completes")
		return true
	}
	return false
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
	WaitUntil string `aliases:"until" help:"Choose whether to wait for service stable or the deployment finishes. (stable|deployed)" default:"stable" enum:"stable,deployed"`
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
	d.warnEarlySuccessCriteriaWait(sv, until)
	if err := doWait(ctx, sv); err != nil {
		if errors.As(err, &errNotFound) && sv.isCodeDeploy() {
			d.LogInfo(err.Error())
			return d.WaitTaskSetStable(ctx, sv)
		}
		return err
	}

	d.LogInfo("service completed", "status", string(until))
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

func (d *App) WaitServiceDeployCompleted(ctx context.Context, sv *Service) error {
	d.LogInfo("Waiting for service deployed...(it will take a few minutes)")
	return d.waitServiceDeployment(ctx, nil)
}

// WaitServiceDeployLifecycleStage waits until the deployment reaches the target
// lifecycle stage, instead of waiting for the whole deployment to finish. This
// allows returning before a long bakeTimeInMinutes elapses.
func (d *App) WaitServiceDeployLifecycleStage(stage types.ServiceDeploymentLifecycleStage) waitFunc {
	return func(ctx context.Context, sv *Service) error {
		if lifecycleStageIndex(stage) < 0 {
			// An unknown stage would index to -1 and be satisfied by any
			// deployment state immediately, so reject it up front.
			return fmt.Errorf("unknown lifecycle stage: %s (expected one of %s)", stage, strings.Join(lifecycleStageNames(), ", "))
		}
		if err := validateLifecycleStageSupported(sv, stage); err != nil {
			return err
		}
		if sv == nil || sv.DeploymentConfiguration == nil || sv.DeploymentConfiguration.Strategy == "" {
			d.LogWarn("deployment strategy is not set; a ROLLING deployment reports no lifecycle stage, so this waits until the deployment completes")
		}
		d.LogInfo("Waiting for service deployment lifecycle stage...", "stage", string(stage))
		target := lifecycleStageIndex(stage)
		notified := map[string]struct{}{}
		// Stages such as PRODUCTION_TRAFFIC_SHIFT are transient and can be
		// skipped between polls, so compare positions rather than equality.
		return d.waitServiceDeployment(ctx, func(dp *types.ServiceDeployment) bool {
			if lifecycleStageIndex(dp.LifecycleStage) >= target {
				return true
			}
			// A pause lifecycle hook awaiting action blocks the deployment, so
			// the target stage never arrives until the deployment is continued.
			for _, id := range pausedHookIDs(dp) {
				if _, ok := notified[id]; ok {
					continue
				}
				notified[id] = struct{}{}
				d.LogWarn("deployment is paused at a lifecycle hook; the target lifecycle stage will not be reached until the deployment is continued (e.g., by `aws ecs continue-service-deployment`)",
					"hook_id", id,
					"lifecycle_stage", string(dp.LifecycleStage),
					"target_stage", string(stage),
				)
			}
			return false
		})
	}
}

// earlySuccessCriteriaOf returns the enabled early success criteria of the
// deployment, or nil when the deployment doesn't use it.
func earlySuccessCriteriaOf(dp *types.ServiceDeployment) *types.DeploymentEarlySuccessCriteria {
	if dp == nil || dp.DeploymentConfiguration == nil {
		return nil
	}
	if esc := dp.DeploymentConfiguration.EarlySuccessCriteria; esc != nil && esc.Enable {
		return esc
	}
	return nil
}

// earlySuccessCriteriaCompletedMessage returns the message to log when a
// deployment with early success criteria completes successfully, describing
// what ECS continues to do outside of the deployment. It returns an empty
// string when the message doesn't apply (e.g., the deployment was rolled back).
func earlySuccessCriteriaCompletedMessage(dp *types.ServiceDeployment) string {
	if dp.Status != types.ServiceDeploymentStatusSuccessful {
		return ""
	}
	esc := earlySuccessCriteriaOf(dp)
	if esc == nil {
		return ""
	}
	if esc.SourceServiceRevisionCleanup == types.ServiceRevisionCleanupDeferred {
		return "early success criteria is enabled; remaining tasks are launched and the previous tasks are removed in the background"
	}
	return "early success criteria is enabled; remaining tasks are launched in the background"
}

// pausedHookIDs returns the IDs of pause lifecycle hooks of the deployment
// that are awaiting action.
func pausedHookIDs(dp *types.ServiceDeployment) []string {
	var ids []string
	for _, hook := range dp.LifecycleHookDetails {
		if hook.TargetType == types.DeploymentLifecycleHookTargetTypePause &&
			hook.Status == types.DeploymentLifecycleHookStatusAwaitingAction {
			ids = append(ids, aws.ToString(hook.HookId))
		}
	}
	return ids
}

// validateLifecycleStageSupported rejects a lifecycle stage target on a service
// whose deployment never reports one, which would otherwise wait until timeout.
func validateLifecycleStageSupported(sv *Service, stage types.ServiceDeploymentLifecycleStage) error {
	if sv == nil || sv.DeploymentConfiguration == nil {
		return nil
	}
	switch strategy := sv.DeploymentConfiguration.Strategy; strategy {
	case types.DeploymentStrategyBlueGreen, types.DeploymentStrategyLinear, types.DeploymentStrategyCanary:
		return nil
	case "":
		return nil // not set by the service definition, let the deployment decide
	default:
		return fmt.Errorf(
			"waiting for lifecycle stage %s requires a traffic shifting deployment strategy, but the service uses %s",
			stage, strategy,
		)
	}
}

// waitServiceDeployment polls the active service deployment until done reports
// true, or until the deployment reaches a terminal status. A nil done waits for
// the deployment to finish.
func (d *App) waitServiceDeployment(ctx context.Context, done func(*types.ServiceDeployment) bool) error {
	deploymentArn, err := d.findActiveECSDeploymentArn(ctx, time.Second*10)
	if err != nil {
		if errors.As(err, &errNotFound) {
			d.LogInfo("No active deployment found")
			return nil // no active deployment, nothing to wait for
		}
		return err
	}
	d.LogInfo("waiting for service deployment", "deployment", arnToName(deploymentArn))

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
			return ErrNotFound("service deployment not found: " + deploymentArn)
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

		// check deployment status
		status := dp.Status
		if status != prevStatus {
			d.LogInfo("service deployment status", "status", string(status))
			prevStatus = status
		}
		result, err := evaluateDeploymentStatus(&dp, done)
		if err != nil {
			return err
		}
		switch result {
		case waitDeploymentCompleted:
			if reason := aws.ToString(dp.StatusReason); reason != "" {
				d.LogInfo("service deployment completed", "status", string(status), "reason", reason)
			} else {
				d.LogInfo("service deployment completed", "status", string(status))
			}
			if msg := earlySuccessCriteriaCompletedMessage(&dp); msg != "" {
				esc := dp.DeploymentConfiguration.EarlySuccessCriteria
				d.LogInfo(msg,
					"healthy_percent", aws.ToInt32(esc.HealthyPercent),
					"source_service_revision_cleanup", string(esc.SourceServiceRevisionCleanup),
				)
			}
			return nil
		case waitDeploymentReachedStage:
			d.LogInfo("service deployment reached the lifecycle stage", "stage", string(dp.LifecycleStage))
			return nil
		default:
			d.LogDebug("Deployment %s, waiting...", status)
		}
	}
}

type waitDeploymentResult int

const (
	waitDeploymentContinue waitDeploymentResult = iota
	waitDeploymentCompleted
	waitDeploymentReachedStage
)

// evaluateDeploymentStatus decides whether polling the service deployment can
// stop. A non-nil done means the caller waits for a lifecycle stage of this
// deployment, so a rollback is a failure instead of a terminal success.
func evaluateDeploymentStatus(dp *types.ServiceDeployment, done func(*types.ServiceDeployment) bool) (waitDeploymentResult, error) {
	switch dp.Status {
	case types.ServiceDeploymentStatusSuccessful:
		return waitDeploymentCompleted, nil
	case types.ServiceDeploymentStatusRollbackSuccessful:
		if done != nil {
			if reason := aws.ToString(dp.StatusReason); reason != "" {
				return waitDeploymentContinue, fmt.Errorf("service deployment has been rolled back before reaching the target lifecycle stage: %s", reason)
			}
			return waitDeploymentContinue, fmt.Errorf("service deployment has been rolled back before reaching the target lifecycle stage")
		}
		return waitDeploymentCompleted, nil
	case types.ServiceDeploymentStatusStopped, types.ServiceDeploymentStatusRollbackFailed, types.ServiceDeploymentStatusStopRequested:
		return waitDeploymentContinue, fmt.Errorf("Service deployment failed %s", dp.Status)
	case types.ServiceDeploymentStatusPending, types.ServiceDeploymentStatusInProgress:
		// A lifecycle stage target is only meaningful while the deployment is
		// progressing. During a rollback the stage can still read as the
		// target, so keep waiting for the rollback to reach a terminal status.
		if done != nil && done(dp) {
			return waitDeploymentReachedStage, nil
		}
	}
	return waitDeploymentContinue, nil
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
		return "", ErrNotFound("No deployments found in progress on CodeDeploy")
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
		return ErrNotFound(fmt.Sprintf("service %s is not found", d.Service))
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
