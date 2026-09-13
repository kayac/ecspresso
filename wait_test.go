package ecspresso_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kayac/ecspresso/v2"
)

func TestPausedHookIDs(t *testing.T) {
	dp := &types.ServiceDeployment{
		LifecycleHookDetails: []types.DeploymentLifecycleHookDetail{
			{
				HookId:     aws.String("hook-awaiting"),
				TargetType: types.DeploymentLifecycleHookTargetTypePause,
				Status:     types.DeploymentLifecycleHookStatusAwaitingAction,
			},
			{
				HookId:     aws.String("hook-in-progress"),
				TargetType: types.DeploymentLifecycleHookTargetTypePause,
				Status:     types.DeploymentLifecycleHookStatusInProgress,
			},
			{
				HookId:     aws.String("hook-lambda"),
				TargetType: types.DeploymentLifecycleHookTargetTypeAwsLambda,
				Status:     types.DeploymentLifecycleHookStatusAwaitingAction,
			},
		},
	}
	want := []string{"hook-awaiting"}
	if diff := cmp.Diff(want, ecspresso.PausedHookIDs(dp)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}

	if got := ecspresso.PausedHookIDs(&types.ServiceDeployment{}); got != nil {
		t.Errorf("expected nil for no hooks, got %v", got)
	}
}

func TestLifecycleStageIndex(t *testing.T) {
	// The waiter compares positions instead of equality, so pin the full
	// ordering of the stages within the deployment lifecycle.
	expected := []types.ServiceDeploymentLifecycleStage{
		"RECONCILE_SERVICE",
		"PRE_SCALE_UP",
		"SCALE_UP",
		"POST_SCALE_UP",
		"TEST_TRAFFIC_SHIFT",
		"POST_TEST_TRAFFIC_SHIFT",
		"PRODUCTION_TRAFFIC_SHIFT",
		"POST_PRODUCTION_TRAFFIC_SHIFT",
		"BAKE_TIME",
		"CLEAN_UP",
	}
	for i, stage := range expected {
		if got := ecspresso.LifecycleStageIndex(stage); got != i {
			t.Errorf("lifecycleStageIndex(%s) = %d, expected %d", stage, got, i)
		}
	}

	// Every stage known to the SDK must have a position, so that an SDK update
	// introducing a new stage fails this test instead of silently indexing to
	// -1 (which never satisfies a target).
	for _, stage := range types.ServiceDeploymentLifecycleStage("").Values() {
		if got := ecspresso.LifecycleStageIndex(stage); got < 0 {
			t.Errorf("lifecycleStageIndex(%s) = %d; add the new stage to lifecycleStages in its lifecycle position", stage, got)
		}
	}

	// A rolling deployment reports no lifecycle stage, which must never satisfy
	// a target stage.
	for _, stage := range []types.ServiceDeploymentLifecycleStage{"", "NO_SUCH_STAGE"} {
		if got := ecspresso.LifecycleStageIndex(stage); got != -1 {
			t.Errorf("lifecycleStageIndex(%q) = %d, expected -1", stage, got)
		}
	}
}

func TestValidateLifecycleStageSupported(t *testing.T) {
	stage := types.ServiceDeploymentLifecycleStageBakeTime

	for _, strategy := range []types.DeploymentStrategy{
		types.DeploymentStrategyBlueGreen,
		types.DeploymentStrategyLinear,
		types.DeploymentStrategyCanary,
		"",
	} {
		sv := &ecspresso.Service{
			Service: types.Service{
				DeploymentConfiguration: &types.DeploymentConfiguration{Strategy: strategy},
			},
		}
		if err := ecspresso.ValidateLifecycleStageSupported(sv, stage); err != nil {
			t.Errorf("strategy %q should be supported, got %v", strategy, err)
		}
	}

	// ROLLING never reports a lifecycle stage, so waiting for one would block
	// until the timeout expires. Fail fast instead.
	sv := &ecspresso.Service{
		Service: types.Service{
			DeploymentConfiguration: &types.DeploymentConfiguration{
				Strategy: types.DeploymentStrategyRolling,
			},
		},
	}
	if err := ecspresso.ValidateLifecycleStageSupported(sv, stage); err == nil {
		t.Error("ROLLING strategy should not be supported")
	}

	if err := ecspresso.ValidateLifecycleStageSupported(nil, stage); err != nil {
		t.Errorf("nil service should be allowed, got %v", err)
	}
}

func TestWaitFuncForECSLifecycleStage(t *testing.T) {
	app := &ecspresso.App{}
	app.SetLogger(ecspresso.NewLogger(new(bytes.Buffer)))
	sv := &ecspresso.Service{
		Service: types.Service{
			DeploymentController: &types.DeploymentController{
				Type: types.DeploymentControllerTypeEcs,
			},
		},
	}

	if _, err := app.WaitFunc(sv, nil, "ecs:BAKE_TIME"); err != nil {
		t.Errorf("ecs:BAKE_TIME should be supported by the ECS deployment controller, got %v", err)
	}
	if _, err := app.WaitFunc(sv, nil, "ecs:CLEAN_UP"); err != nil {
		t.Errorf("ecs:CLEAN_UP should be supported by the ECS deployment controller, got %v", err)
	}

	// The lifecycle stage of an ECS deployment has no meaning for CodeDeploy.
	cd := &ecspresso.Service{
		Service: types.Service{
			DeploymentController: &types.DeploymentController{
				Type: types.DeploymentControllerTypeCodeDeploy,
			},
		},
	}
	if _, err := app.WaitFunc(cd, nil, "ecs:BAKE_TIME"); err == nil {
		t.Error("ecs:* should not be accepted for the CodeDeploy deployment controller")
	}

	// ECS is the default deployment controller: a service without an explicit
	// one must not fall back to the service-stable waiter silently.
	noController := &ecspresso.Service{Service: types.Service{}}
	doWait, err := app.WaitFunc(noController, nil, "ecs:BAKE_TIME")
	if err != nil {
		t.Errorf("ecs:BAKE_TIME should be supported without an explicit deployment controller, got %v", err)
	}
	// The returned waiter must reject a rolling service before polling AWS.
	rolling := &ecspresso.Service{
		Service: types.Service{
			DeploymentConfiguration: &types.DeploymentConfiguration{
				Strategy: types.DeploymentStrategyRolling,
			},
		},
	}
	if err := doWait(t.Context(), rolling); err == nil {
		t.Error("waiting for a lifecycle stage on a ROLLING service should fail")
	}
}

func TestWaitServiceDeployLifecycleStageUnknownStage(t *testing.T) {
	app := &ecspresso.App{}
	sv := &ecspresso.Service{
		Service: types.Service{
			DeploymentConfiguration: &types.DeploymentConfiguration{
				Strategy: types.DeploymentStrategyBlueGreen,
			},
		},
	}
	// An unknown stage would index to -1 and be satisfied immediately, so the
	// waiter must reject it before polling anything.
	for _, stage := range []types.ServiceDeploymentLifecycleStage{"", "NO_SUCH_STAGE"} {
		if err := app.WaitServiceDeployLifecycleStage(stage, "")(t.Context(), sv); err == nil {
			t.Errorf("unknown lifecycle stage %q should be rejected", stage)
		}
	}
}

func TestDeploymentPaused(t *testing.T) {
	app := &ecspresso.App{}
	app.SetLogger(ecspresso.NewLogger(new(bytes.Buffer)))

	tests := []struct {
		name  string
		hooks []types.DeploymentLifecycleHookDetail
		want  bool
	}{
		{
			name: "no hooks",
			want: false,
		},
		{
			name: "pause hook awaiting action",
			hooks: []types.DeploymentLifecycleHookDetail{
				{
					HookId:     aws.String("hook-1"),
					TargetType: types.DeploymentLifecycleHookTargetTypePause,
					Status:     types.DeploymentLifecycleHookStatusAwaitingAction,
				},
			},
			want: true,
		},
		{
			name: "pause hook not awaiting action yet",
			hooks: []types.DeploymentLifecycleHookDetail{
				{
					HookId:     aws.String("hook-1"),
					TargetType: types.DeploymentLifecycleHookTargetTypePause,
					Status:     types.DeploymentLifecycleHookStatusInProgress,
				},
			},
			want: false,
		},
		{
			name: "non-pause hook awaiting action",
			hooks: []types.DeploymentLifecycleHookDetail{
				{
					HookId:     aws.String("hook-1"),
					TargetType: types.DeploymentLifecycleHookTargetTypeAwsLambda,
					Status:     types.DeploymentLifecycleHookStatusAwaitingAction,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dp := &types.ServiceDeployment{LifecycleHookDetails: tt.hooks}
			if got := app.DeploymentPaused(dp); got != tt.want {
				t.Errorf("deploymentPaused = %v, expected %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateDeploymentStatus(t *testing.T) {
	reached := func(dp *types.ServiceDeployment) bool {
		return ecspresso.LifecycleStageIndex(dp.LifecycleStage) >= ecspresso.LifecycleStageIndex(types.ServiceDeploymentLifecycleStageBakeTime)
	}
	tests := []struct {
		name    string
		dp      types.ServiceDeployment
		done    func(*types.ServiceDeployment) bool
		want    ecspresso.WaitDeploymentResult
		wantErr bool
	}{
		{
			name: "successful",
			dp:   types.ServiceDeployment{Status: types.ServiceDeploymentStatusSuccessful},
			want: ecspresso.WaitDeploymentCompleted,
		},
		{
			name: "rollback is completed when waiting for the deployment",
			dp:   types.ServiceDeployment{Status: types.ServiceDeploymentStatusRollbackSuccessful},
			want: ecspresso.WaitDeploymentCompleted,
		},
		{
			name:    "rollback fails when waiting for a done condition",
			dp:      types.ServiceDeployment{Status: types.ServiceDeploymentStatusRollbackSuccessful},
			done:    reached,
			wantErr: true,
		},
		{
			name:    "stopped",
			dp:      types.ServiceDeployment{Status: types.ServiceDeploymentStatusStopped},
			wantErr: true,
		},
		{
			name: "in progress before the target stage",
			dp: types.ServiceDeployment{
				Status:         types.ServiceDeploymentStatusInProgress,
				LifecycleStage: types.ServiceDeploymentLifecycleStageScaleUp,
			},
			done: reached,
			want: ecspresso.WaitDeploymentContinue,
		},
		{
			name: "in progress at the target stage",
			dp: types.ServiceDeployment{
				Status:         types.ServiceDeploymentStatusInProgress,
				LifecycleStage: types.ServiceDeploymentLifecycleStageBakeTime,
			},
			done: reached,
			want: ecspresso.WaitDeploymentDone,
		},
		{
			name: "the stage during a rollback does not satisfy the target",
			dp: types.ServiceDeployment{
				Status:         types.ServiceDeploymentStatusRollbackInProgress,
				LifecycleStage: types.ServiceDeploymentLifecycleStageBakeTime,
			},
			done: reached,
			want: ecspresso.WaitDeploymentContinue,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ecspresso.EvaluateDeploymentStatus(&tt.dp, tt.done)
			if tt.wantErr {
				if err == nil {
					t.Error("expected an error")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("evaluateDeploymentStatus = %v, expected %v", got, tt.want)
			}
		})
	}
}

// waiterOf returns the code pointer of a waiter to identify which method
// WaitFunc selected. It only works without a confirm func, which would wrap
// the waiter in a new closure.
func waiterOf(f any) uintptr {
	return reflect.ValueOf(f).Pointer()
}

func TestWaitFuncSelectsWaiter(t *testing.T) {
	app := &ecspresso.App{}
	withController := &ecspresso.Service{
		Service: types.Service{
			DeploymentController: &types.DeploymentController{
				Type: types.DeploymentControllerTypeEcs,
			},
		},
	}
	// ECS is the default deployment controller, so a service definition
	// without one must select the same waiter as an explicit ECS controller.
	noController := &ecspresso.Service{Service: types.Service{}}
	earlySuccess := &ecspresso.Service{
		Service: types.Service{
			DeploymentConfiguration: &types.DeploymentConfiguration{
				EarlySuccessCriteria: &types.DeploymentEarlySuccessCriteria{
					Enable:                       true,
					HealthyPercent:               aws.Int32(90),
					SourceServiceRevisionCleanup: types.ServiceRevisionCleanupDeferred,
				},
			},
		},
	}
	// The deployed and paused waiters are closures built by WaitFunc, so take
	// the ones selected for an explicit ECS controller as the reference.
	waiterFor := func(until string) uintptr {
		f, err := app.WaitFunc(withController, nil, ecspresso.WaitUntil(until))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return waiterOf(f)
	}
	deployed := waiterFor("deployed")
	paused := waiterFor("paused")
	stable := waiterOf(app.WaitServiceStable)
	if deployed == stable || paused == stable || deployed == paused {
		t.Fatal("reference waiters must be distinguishable")
	}
	tests := []struct {
		name  string
		sv    *ecspresso.Service
		until string
		want  uintptr
	}{
		{"deployed with ECS controller", withController, "deployed", deployed},
		{"deployed without controller", noController, "deployed", deployed},
		{"paused without controller", noController, "paused", paused},
		{"stable with ECS controller", withController, "stable", stable},
		{"stable without controller", noController, "stable", stable},
		{"empty without controller", noController, "", stable},
		{"deployed with early success criteria", earlySuccess, "deployed", deployed},
		{"stable with early success criteria", earlySuccess, "stable", stable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doWait, err := app.WaitFunc(tt.sv, nil, ecspresso.WaitUntil(tt.until))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := waiterOf(doWait); got != tt.want {
				t.Errorf("unexpected waiter selected for --wait-until=%q", tt.until)
			}
		})
	}

	if _, err := app.WaitFunc(noController, nil, "no-such-value"); err == nil {
		t.Error("an unknown waitUntil should be rejected without an explicit deployment controller")
	}
}

func TestEarlySuccessCriteria(t *testing.T) {
	enabled := &types.DeploymentEarlySuccessCriteria{
		Enable:                       true,
		HealthyPercent:               aws.Int32(80),
		SourceServiceRevisionCleanup: types.ServiceRevisionCleanupBlocking,
	}
	tests := []struct {
		name string
		dc   *types.DeploymentConfiguration
		want *types.DeploymentEarlySuccessCriteria
	}{
		{"no deployment configuration", nil, nil},
		{"no early success criteria", &types.DeploymentConfiguration{}, nil},
		{
			"disabled",
			&types.DeploymentConfiguration{
				EarlySuccessCriteria: &types.DeploymentEarlySuccessCriteria{Enable: false, HealthyPercent: aws.Int32(80)},
			},
			nil,
		},
		{"enabled", &types.DeploymentConfiguration{EarlySuccessCriteria: enabled}, enabled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dp := &types.ServiceDeployment{DeploymentConfiguration: tt.dc}
			if diff := cmp.Diff(tt.want, ecspresso.EarlySuccessCriteriaOf(dp), cmpopts.IgnoreUnexported(types.DeploymentEarlySuccessCriteria{})); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
			sv := &ecspresso.Service{Service: types.Service{DeploymentConfiguration: tt.dc}}
			if got, want := sv.EarlySuccessCriteriaEnabled(), tt.want != nil; got != want {
				t.Errorf("EarlySuccessCriteriaEnabled = %v, want %v", got, want)
			}
		})
	}
	if ecspresso.EarlySuccessCriteriaOf(nil) != nil {
		t.Error("nil deployment should not have early success criteria")
	}
	var nilSv *ecspresso.Service
	if nilSv.EarlySuccessCriteriaEnabled() {
		t.Error("nil service should not enable early success criteria")
	}
}

func TestWarnEarlySuccessCriteriaWait(t *testing.T) {
	app := &ecspresso.App{}
	logs := new(bytes.Buffer)
	app.SetLogger(ecspresso.NewLogger(logs))
	esc := &types.DeploymentEarlySuccessCriteria{
		Enable:                       true,
		HealthyPercent:               aws.Int32(90),
		SourceServiceRevisionCleanup: types.ServiceRevisionCleanupDeferred,
	}
	enabled := &ecspresso.Service{
		Service: types.Service{
			DeploymentConfiguration: &types.DeploymentConfiguration{EarlySuccessCriteria: esc},
		},
	}
	codeDeploy := &ecspresso.Service{
		Service: types.Service{
			DeploymentController:    &types.DeploymentController{Type: types.DeploymentControllerTypeCodeDeploy},
			DeploymentConfiguration: &types.DeploymentConfiguration{EarlySuccessCriteria: esc},
		},
	}
	disabled := &ecspresso.Service{
		Service: types.Service{
			DeploymentConfiguration: &types.DeploymentConfiguration{
				EarlySuccessCriteria: &types.DeploymentEarlySuccessCriteria{Enable: false},
			},
		},
	}
	tests := []struct {
		name  string
		sv    *ecspresso.Service
		until string
		want  bool
	}{
		{"stable defeats early success criteria", enabled, "stable", true},
		{"empty means stable", enabled, "", true},
		{"deployed returns early", enabled, "deployed", false},
		{"lifecycle stage", enabled, "ecs:BAKE_TIME", false},
		{"disabled criteria", disabled, "stable", false},
		{"no deployment configuration", &ecspresso.Service{}, "stable", false},
		{"nil service", nil, "stable", false},
		{"CodeDeploy never uses the criteria", codeDeploy, "stable", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs.Reset()
			got := app.WarnEarlySuccessCriteriaWait(tt.sv, ecspresso.WaitUntil(tt.until))
			if got != tt.want {
				t.Errorf("warned = %v, want %v", got, tt.want)
			}
			if logged := strings.Contains(logs.String(), "early success criteria is enabled but waiting for service stable"); logged != tt.want {
				t.Errorf("warning logged = %v, want %v", logged, tt.want)
			}
		})
	}
}

func TestEarlySuccessCriteriaCompletedMessage(t *testing.T) {
	criteria := func(cleanup types.ServiceRevisionCleanup) *types.DeploymentConfiguration {
		return &types.DeploymentConfiguration{
			EarlySuccessCriteria: &types.DeploymentEarlySuccessCriteria{
				Enable:                       true,
				HealthyPercent:               aws.Int32(50),
				SourceServiceRevisionCleanup: cleanup,
			},
		}
	}
	tests := []struct {
		name string
		dp   types.ServiceDeployment
		want string
	}{
		{
			name: "deferred cleanup",
			dp:   types.ServiceDeployment{Status: types.ServiceDeploymentStatusSuccessful, DeploymentConfiguration: criteria(types.ServiceRevisionCleanupDeferred)},
			want: "early success criteria is enabled; remaining tasks are launched and the previous tasks are removed in the background",
		},
		{
			name: "blocking cleanup",
			dp:   types.ServiceDeployment{Status: types.ServiceDeploymentStatusSuccessful, DeploymentConfiguration: criteria(types.ServiceRevisionCleanupBlocking)},
			want: "early success criteria is enabled; remaining tasks are launched in the background",
		},
		{
			// A rollback also ends the wait, but nothing continues in the background.
			name: "rolled back",
			dp:   types.ServiceDeployment{Status: types.ServiceDeploymentStatusRollbackSuccessful, DeploymentConfiguration: criteria(types.ServiceRevisionCleanupDeferred)},
		},
		{
			name: "no criteria",
			dp:   types.ServiceDeployment{Status: types.ServiceDeploymentStatusSuccessful, DeploymentConfiguration: &types.DeploymentConfiguration{}},
		},
		{
			name: "disabled criteria",
			dp: types.ServiceDeployment{Status: types.ServiceDeploymentStatusSuccessful, DeploymentConfiguration: &types.DeploymentConfiguration{
				EarlySuccessCriteria: &types.DeploymentEarlySuccessCriteria{Enable: false},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ecspresso.EarlySuccessCriteriaCompletedMessage(&tt.dp); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
