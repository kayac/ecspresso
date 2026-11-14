package ecspresso_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/smithy-go/middleware"
	"github.com/kayac/ecspresso/v3"
)

// deploy applies --wait-until also when it creates a new service, so a value
// the service does not support fails before the service is created.
func TestDeployCreateServiceRejectsUnsupportedWaitUntil(t *testing.T) {
	mock := func(stack *middleware.Stack) error {
		return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("test-create", func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
			switch input := in.Parameters.(type) {
			case *ecs.DescribeServicesInput:
				// the service does not exist
				return middleware.InitializeOutput{Result: &ecs.DescribeServicesOutput{}}, middleware.Metadata{}, nil
			default:
				return middleware.InitializeOutput{}, middleware.Metadata{}, fmt.Errorf("unexpected request %T", input)
			}
		}), middleware.Before)
	}
	ecspresso.SetAWSV2ConfigLoadOptionsFunc([]func(*config.LoadOptions) error{
		config.WithRegion("ap-northeast-1"),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
		config.WithAPIOptions([]func(*middleware.Stack) error{mock}),
	})
	t.Cleanup(ecspresso.ResetAWSV2ConfigLoadOptionsFunc)
	app, err := ecspresso.New(t.Context(), &ecspresso.CLIOptions{ConfigFilePath: "tests/run-with-sv.yaml"})
	if err != nil {
		t.Fatal(err)
	}

	// tests/sv.json uses the ECS deployment controller.
	err = app.Deploy(t.Context(), ecspresso.DeployOption{
		Wait:      true,
		WaitUntil: "codedeploy:AfterAllowTraffic",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported waitUntil") {
		t.Errorf("deploy must fail with unsupported waitUntil before creating the service, got %v", err)
	}
}
