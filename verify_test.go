package ecspresso_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"
	"github.com/fatih/color"
	"github.com/kayac/ecspresso/v2"
)

var testRoleArns = []struct {
	arn      string
	roleName string
	isValid  bool
}{
	{
		arn:      "arn:aws:iam::123456789012:role/ecsTaskRole",
		roleName: "ecsTaskRole",
		isValid:  true,
	},
	{
		arn:      "arn:aws:iam::123456789012:role/path/to/ecsTaskRole",
		roleName: "ecsTaskRole",
		isValid:  true,
	},
	{
		arn: "arn:aws:iam::123456789012:foo",
	},
	{
		arn: "arn:aws:iam::123456789012:policy/ecsTaskRole",
	},
	{
		arn: "arn:aws:ec2::123456789012:foo/bar",
	},
	{
		arn: "ecsTaskRole",
	},
}

var testImagesIsECR = []struct {
	image string
	isECR bool
}{
	{
		image: "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage",
		isECR: true,
	},
	{
		image: "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage:latest",
		isECR: true,
	},
	{
		image: "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage@sha256:xxx",
		isECR: true,
	},
	{
		image: "ubuntu:latest",
		isECR: false,
	},
	{
		image: "ubuntu@sha256:8f6a88feef3ed01a300dafb87f208977f39dccda1fd120e878129463f7fa3b8f",
		isECR: false,
	},
	{
		image: "123456789012.dkr.ecr.cn-north-1.amazonaws.com.cn/my-app:latest",
		isECR: true,
	},
}

type goPlatform struct {
	arch string
	os   string
}

var testRuntimePlatforms = []struct {
	platform  types.RuntimePlatform
	isFargate bool
	want      goPlatform
}{
	{
		isFargate: false,
		want: goPlatform{
			arch: "amd64",
			os:   "linux",
		},
	},
	{
		platform: types.RuntimePlatform{
			CpuArchitecture: types.CPUArchitectureArm64,
		},
		isFargate: true,
		want: goPlatform{
			arch: "arm64",
			os:   "linux",
		},
	},
	{
		platform: types.RuntimePlatform{
			OperatingSystemFamily: types.OSFamilyWindowsServer2019Core,
		},
		isFargate: true,
		want: goPlatform{
			arch: "amd64",
			os:   "windows",
		},
	},
	{
		platform: types.RuntimePlatform{
			OperatingSystemFamily: types.OSFamilyWindowsServer2022Full,
		},
		isFargate: true,
		want: goPlatform{
			arch: "amd64",
			os:   "windows",
		},
	},
	{
		platform: types.RuntimePlatform{
			CpuArchitecture: types.CPUArchitectureX8664,
		},
		isFargate: false,
		want: goPlatform{
			arch: "amd64",
			os:   "linux",
		},
	},
	{
		platform: types.RuntimePlatform{
			OperatingSystemFamily: types.OSFamilyWindowsServer2019Core,
		},
		isFargate: false,
		want: goPlatform{
			arch: "amd64",
			os:   "windows",
		},
	},
}

func TestNormalizePlatform(t *testing.T) {
	for _, p := range testRuntimePlatforms {
		arch, os := ecspresso.NormalizePlatform(&p.platform, p.isFargate)
		if arch != p.want.arch || os != p.want.os {
			t.Errorf("want arch/os %s/%s but got %s/%s", p.want.arch, p.want.os, arch, os)
		}
	}
}

func TestParseRoleArn(t *testing.T) {
	for _, s := range testRoleArns {
		name, err := ecspresso.ExtractRoleName(s.arn)
		if s.isValid {
			if name != s.roleName {
				t.Errorf("invalid roleName got:%s expected:%s", name, s.roleName)
			}
			if err != nil {
				t.Error("unexpected error", err)
			}
		} else if err == nil {
			t.Errorf("must be failed valdation for %s", s.arn)
		}
	}
}

var imageURLTestCases = []struct {
	imageURL            string
	expectedImageName   string
	expectedTagOrDigest string
}{
	{
		imageURL:            "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage",
		expectedImageName:   "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage",
		expectedTagOrDigest: "latest",
	},
	{
		imageURL:            "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage:foobar",
		expectedImageName:   "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage",
		expectedTagOrDigest: "foobar",
	},
	{
		imageURL:            "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage@sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		expectedImageName:   "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage",
		expectedTagOrDigest: "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
	},
	{
		imageURL:            "example.com:443/repo/image:tag",
		expectedImageName:   "example.com:443/repo/image",
		expectedTagOrDigest: "tag",
	},
	{
		imageURL:            "example.com:443/repo/image@sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		expectedImageName:   "example.com:443/repo/image",
		expectedTagOrDigest: "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
	},
	{
		imageURL:            "public.ecr.aws/nginx/nginx:1.27@sha256:df80ece7fa07f8ed4f3b70e883fe322732954d0287e562f1fdc790018f7d2c66",
		expectedImageName:   "public.ecr.aws/nginx/nginx",
		expectedTagOrDigest: "sha256:df80ece7fa07f8ed4f3b70e883fe322732954d0287e562f1fdc790018f7d2c66",
	},
	{
		imageURL:            "example.com:443/repo/image:tag@sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		expectedImageName:   "example.com:443/repo/image",
		expectedTagOrDigest: "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
	},
}

func TestParseImageURL(t *testing.T) {
	for _, tc := range imageURLTestCases {
		t.Run(tc.imageURL, func(t *testing.T) {
			imageName, tagOrDigest := ecspresso.ParseImageURL(tc.imageURL)
			if imageName != tc.expectedImageName {
				t.Errorf("unexpected imageName got:%s expected:%s", imageName, tc.expectedImageName)
			}
			if tagOrDigest != tc.expectedTagOrDigest {
				t.Errorf("unexpected tagOrDigest got:%s expected:%s", tagOrDigest, tc.expectedTagOrDigest)
			}
		})
	}
}

func TestIsECRImage(t *testing.T) {
	for _, s := range testImagesIsECR {
		isECR := ecspresso.ECRImageURLRegex.MatchString(s.image)
		if isECR != s.isECR {
			t.Errorf("invalid detect ECR image %s got:%t expected:%t", s.image, isECR, s.isECR)
		}
	}
}

func TestVerifyOKResource(t *testing.T) {
	color.NoColor = true
	for _, cache := range []bool{false, true} {
		vs := ecspresso.NewVerifyState(cache)
		for i := range 3 {
			r, err := vs.VerifyResource(context.TODO(), "ok resource", func(_ context.Context) error {
				return nil
			})
			if err != nil {
				t.Error("unexpected error for ok resource", err)
			}
			if r.Name != "ok resource" {
				t.Error("unexpected output for ok resource", r.Name)
			}
			if r.Result != "OK" {
				t.Error("unexpected output [OK] for ok resource", r.Result)
			}
			if cache && i >= 1 {
				if !r.Cached {
					t.Error("unexpected output (cached) for ok resource", r.Cached)
				}
			}
		}
	}
}

func TestVerifyNGResource(t *testing.T) {
	color.NoColor = true
	for _, cache := range []bool{false, true} {
		vs := ecspresso.NewVerifyState(cache)
		for i := range 3 {
			r, err := vs.VerifyResource(context.TODO(), "ng resource", func(_ context.Context) error {
				return errors.New("XXX")
			})
			if err == nil {
				t.Error("error must be returned for ng resource")
			}
			if r.Name != "ng resource" {
				t.Error("unexpected output for ng resource", r.Name)
			}
			if r.Result != "NG" {
				t.Error("unexpected output [NG] for ng resource", r.Result)
			}
			if r.Error == "" {
				t.Error("error must be returned for ng resource", r.Error)
			}
			if cache && i >= 1 {
				if !r.Cached {
					t.Error("unexpected output (cached) for ng resource", r.Cached)
				}
			}
		}
	}
}

func TestVerifySkipResource(t *testing.T) {
	color.NoColor = true
	for _, cache := range []bool{false, true} {
		vs := ecspresso.NewVerifyState(cache)
		for i := range 3 {
			r, err := vs.VerifyResource(context.TODO(), "skip resource", func(_ context.Context) error {
				return ecspresso.ErrSkipVerify("hello")
			})
			if err != nil {
				t.Error("unexpected error for skip resource", err)
			}
			if r.Result != "SKIP" {
				t.Error("unexpected output [SKIP] for skip resource", r.Result)
			}
			if cache && i >= 1 {
				if !r.Cached {
					t.Error("unexpected output (cached) for skip resource", r.Cached)
				}
			}
		}
	}
}

func TestVerifierIsAssumed(t *testing.T) {
	cfg1 := aws.Config{}
	cfg2 := aws.Config{}
	var testCases = []struct {
		exec      *aws.Config
		app       *aws.Config
		isAssumed bool
	}{
		{&cfg1, &cfg2, true},
		{&cfg1, &cfg1, false},
		{&cfg2, &cfg2, false},
		{&cfg2, &cfg1, true},
	}
	for i, c := range testCases {
		v := ecspresso.NewVerifier(c.exec, c.app, &ecspresso.VerifyOption{})
		if v.IsAssumed() != c.isAssumed {
			t.Errorf("unexpected IsAssumed %d expected:%v got:%v", i, c.isAssumed, v.IsAssumed())
		}
	}
}

type mockAPIError struct {
	code string
}

func (e mockAPIError) Error() string {
	return "mock error: " + e.code
}

func (e mockAPIError) ErrorCode() string {
	return e.code
}

func (e mockAPIError) ErrorMessage() string {
	return "mock error: " + e.code
}

func (e mockAPIError) ErrorFault() smithy.ErrorFault {
	return smithy.FaultClient
}

func TestIsPermissionError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "AccessDeniedException",
			err:      mockAPIError{code: "AccessDeniedException"},
			expected: true,
		},
		{
			name:     "UnauthorizedException",
			err:      mockAPIError{code: "UnauthorizedException"},
			expected: true,
		},
		{
			name:     "Forbidden",
			err:      mockAPIError{code: "Forbidden"},
			expected: true,
		},
		{
			name:     "AccessDenied",
			err:      mockAPIError{code: "AccessDenied"},
			expected: true,
		},
		{
			name:     "InvalidUserID.NotFound",
			err:      mockAPIError{code: "InvalidUserID.NotFound"},
			expected: true,
		},
		{
			name: "AccessDeniedException wrapped in OperationError",
			err: &smithy.OperationError{
				ServiceID:     "IAM",
				OperationName: "GetRole",
				Err:           mockAPIError{code: "AccessDeniedException"},
			},
			expected: true,
		},
		{
			name: "Other error wrapped in OperationError",
			err: &smithy.OperationError{
				ServiceID:     "IAM",
				OperationName: "GetRole",
				Err:           mockAPIError{code: "ValidationException"},
			},
			expected: false,
		},
		{
			name:     "Other error",
			err:      mockAPIError{code: "ValidationException"},
			expected: false,
		},
		{
			name:     "Regular error",
			err:      errors.New("regular error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ecspresso.IsPermissionError(tc.err)
			if result != tc.expected {
				t.Errorf("isPermissionError(%v) = %v, expected %v", tc.err, result, tc.expected)
			}
		})
	}
}

func TestVerifyWarnResource(t *testing.T) {
	color.NoColor = true
	for _, cache := range []bool{false, true} {
		vs := ecspresso.NewVerifyState(cache)
		for i := range 3 {
			r, err := vs.VerifyResource(context.TODO(), "warn resource", func(_ context.Context) error {
				// Simulate AWS permission error
				mockErr := mockAPIError{code: "AccessDeniedException"}
				return ecspresso.WrapPermissionError(mockErr)
			})
			if err != nil {
				t.Error("unexpected error for warn resource", err)
			}
			if r.Name != "warn resource" {
				t.Error("unexpected output for warn resource", r.Name)
			}
			if r.Result != "WARN" {
				t.Error("unexpected output [WARN] for warn resource", r.Result)
			}
			if r.Error == "" {
				t.Error("error message must be set for warn resource", r.Error)
			}
			if cache && i >= 1 {
				if !r.Cached {
					t.Error("unexpected output (cached) for warn resource", r.Cached)
				}
			}
		}
	}
}

func TestWrapPermissionError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		wantType string
	}{
		{
			name:     "Permission error gets wrapped",
			err:      mockAPIError{code: "AccessDeniedException"},
			wantType: "ErrPermissionDenied",
		},
		{
			name:     "Non-permission error stays unchanged",
			err:      mockAPIError{code: "ValidationException"},
			wantType: "mockAPIError",
		},
		{
			name:     "Nil error stays nil",
			err:      nil,
			wantType: "nil",
		},
		{
			name:     "Regular error stays unchanged",
			err:      errors.New("regular error"),
			wantType: "*errors.errorString",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ecspresso.WrapPermissionError(tc.err)

			switch tc.wantType {
			case "nil":
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			case "ErrPermissionDenied":
				var permErr ecspresso.ErrPermissionDenied
				if !errors.As(result, &permErr) {
					t.Errorf("expected ErrPermissionDenied, got %T", result)
				}
			case "mockAPIError":
				var mockErr mockAPIError
				if !errors.As(result, &mockErr) {
					t.Errorf("expected mockAPIError, got %T", result)
				}
			case "*errors.errorString":
				if result.Error() != tc.err.Error() {
					t.Errorf("expected same error, got %v", result)
				}
			}
		})
	}
}

func TestParseIAMPolicyDocument(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		containsService string
		containsAction  string
		wantErr         bool
	}{
		{
			name: "string format for Service and Action",
			input: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "ecs-tasks.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`,
			containsService: "ecs-tasks.amazonaws.com",
			containsAction:  "sts:AssumeRole",
			wantErr:         false,
		},
		{
			name: "array format for Service and Action",
			input: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": ["ecs-tasks.amazonaws.com"]},
					"Action": ["sts:AssumeRole"]
				}]
			}`,
			containsService: "ecs-tasks.amazonaws.com",
			containsAction:  "sts:AssumeRole",
			wantErr:         false,
		},
		{
			name: "array format with multiple services",
			input: `{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": ["ecs-tasks.amazonaws.com", "ec2.amazonaws.com"]},
					"Action": ["sts:AssumeRole", "sts:TagSession"]
				}]
			}`,
			containsService: "ecs-tasks.amazonaws.com",
			containsAction:  "sts:AssumeRole",
			wantErr:         false,
		},
		{
			name:            "URL encoded input",
			input:           `%7B%22Version%22%3A%222012-10-17%22%2C%22Statement%22%3A%5B%7B%22Effect%22%3A%22Allow%22%2C%22Principal%22%3A%7B%22Service%22%3A%22ecs-tasks.amazonaws.com%22%7D%2C%22Action%22%3A%22sts%3AAssumeRole%22%7D%5D%7D`,
			containsService: "ecs-tasks.amazonaws.com",
			containsAction:  "sts:AssumeRole",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := ecspresso.ParseIAMPolicyDocument(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseIAMPolicyDocument() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(doc.Statement) == 0 {
				t.Error("ParseIAMPolicyDocument() returned empty statement")
				return
			}

			stmt := doc.Statement[0]

			if !ecspresso.StringOrSlice(stmt.Principal.Service).Contains(tt.containsService) {
				t.Errorf("ParseIAMPolicyDocument() Service does not contain %q, got %v", tt.containsService, stmt.Principal.Service)
			}
			if !ecspresso.StringOrSlice(stmt.Action).Contains(tt.containsAction) {
				t.Errorf("ParseIAMPolicyDocument() Action does not contain %q, got %v", tt.containsAction, stmt.Action)
			}
		})
	}
}

func TestVerifyDeploymentConfigurationLifecycleHooks(t *testing.T) {
	color.NoColor = true
	app := &ecspresso.App{}

	t.Run("nil deployment configuration", func(t *testing.T) {
		ctx := ecspresso.ContextWithVerifyState(t.Context(), ecspresso.NewVerifyState(false))
		if err := app.VerifyDeploymentConfiguration(ctx, nil); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
	})

	t.Run("pause hook without roleArn and hookTargetArn", func(t *testing.T) {
		ctx := ecspresso.ContextWithVerifyState(t.Context(), ecspresso.NewVerifyState(false))
		dc := &types.DeploymentConfiguration{
			LifecycleHooks: []types.DeploymentLifecycleHook{
				{
					TargetType:      types.DeploymentLifecycleHookTargetTypePause,
					LifecycleStages: []types.DeploymentLifecycleHookStage{types.DeploymentLifecycleHookStagePostScaleUp},
				},
			},
		}
		if err := app.VerifyDeploymentConfiguration(ctx, dc); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
	})

	t.Run("lambda hook without hookTargetArn", func(t *testing.T) {
		ctx := ecspresso.ContextWithVerifyState(t.Context(), ecspresso.NewVerifyState(false))
		dc := &types.DeploymentConfiguration{
			LifecycleHooks: []types.DeploymentLifecycleHook{
				{
					// targetType is not specified, so it defaults to AWS_LAMBDA
					LifecycleStages: []types.DeploymentLifecycleHookStage{types.DeploymentLifecycleHookStagePostScaleUp},
				},
			},
		}
		err := app.VerifyDeploymentConfiguration(ctx, dc)
		if err == nil {
			t.Fatal("error must be returned for a lambda hook without hookTargetArn")
		}
		if !strings.Contains(err.Error(), "hookTargetArn is required") {
			t.Errorf("unexpected error: %s", err)
		}
	})
}

func TestVerifyDeploymentConfigurationEarlySuccessCriteria(t *testing.T) {
	color.NoColor = true
	app := &ecspresso.App{}
	valid := func() *types.DeploymentConfiguration {
		return &types.DeploymentConfiguration{
			Strategy:              types.DeploymentStrategyRolling,
			MinimumHealthyPercent: aws.Int32(50),
			EarlySuccessCriteria: &types.DeploymentEarlySuccessCriteria{
				Enable:                       true,
				HealthyPercent:               aws.Int32(90),
				SourceServiceRevisionCleanup: types.ServiceRevisionCleanupDeferred,
			},
		}
	}
	tests := []struct {
		name    string
		modify  func(dc *types.DeploymentConfiguration)
		wantErr string
	}{
		{name: "valid"},
		{
			name:   "strategy is not set",
			modify: func(dc *types.DeploymentConfiguration) { dc.Strategy = "" },
		},
		{
			name:   "minimumHealthyPercent is not set",
			modify: func(dc *types.DeploymentConfiguration) { dc.MinimumHealthyPercent = nil },
		},
		{
			name: "disabled criteria is not validated",
			modify: func(dc *types.DeploymentConfiguration) {
				dc.EarlySuccessCriteria = &types.DeploymentEarlySuccessCriteria{Enable: false}
			},
		},
		{
			name:    "blue/green strategy",
			modify:  func(dc *types.DeploymentConfiguration) { dc.Strategy = types.DeploymentStrategyBlueGreen },
			wantErr: "only supported by the ROLLING deployment strategy",
		},
		{
			name:    "healthyPercent is missing",
			modify:  func(dc *types.DeploymentConfiguration) { dc.EarlySuccessCriteria.HealthyPercent = nil },
			wantErr: "healthyPercent is required",
		},
		{
			name:    "healthyPercent is over 100",
			modify:  func(dc *types.DeploymentConfiguration) { dc.EarlySuccessCriteria.HealthyPercent = aws.Int32(101) },
			wantErr: "must be between 0 and 100",
		},
		{
			name:    "healthyPercent is less than minimumHealthyPercent",
			modify:  func(dc *types.DeploymentConfiguration) { dc.EarlySuccessCriteria.HealthyPercent = aws.Int32(49) },
			wantErr: "greater than or equal to minimumHealthyPercent",
		},
		{
			name:    "sourceServiceRevisionCleanup is missing",
			modify:  func(dc *types.DeploymentConfiguration) { dc.EarlySuccessCriteria.SourceServiceRevisionCleanup = "" },
			wantErr: "sourceServiceRevisionCleanup is required",
		},
		{
			name: "sourceServiceRevisionCleanup is invalid",
			modify: func(dc *types.DeploymentConfiguration) {
				dc.EarlySuccessCriteria.SourceServiceRevisionCleanup = "LATER"
			},
			wantErr: "must be BLOCKING or DEFERRED",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ecspresso.ContextWithVerifyState(t.Context(), ecspresso.NewVerifyState(false))
			dc := valid()
			if tt.modify != nil {
				tt.modify(dc)
			}
			err := app.VerifyDeploymentConfiguration(ctx, dc)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("error containing %q must be returned", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}
