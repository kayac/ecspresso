package ecspresso_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/kayac/ecspresso/v3"
)

// wait finds a service deployment started by another command, also while it
// is rolling back.
func TestFindOngoingServiceDeploymentInRollback(t *testing.T) {
	const arn = "arn:aws:ecs:ap-northeast-1:123456789012:service-deployment/default/test/xxxx"
	var requested *ecs.ListServiceDeploymentsInput
	mock := func(stack *middleware.Stack) error {
		return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("test-find", func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
			switch input := in.Parameters.(type) {
			case *ecs.ListServiceDeploymentsInput:
				requested = input
				return middleware.InitializeOutput{Result: &ecs.ListServiceDeploymentsOutput{
					ServiceDeployments: []types.ServiceDeploymentBrief{{
						ServiceDeploymentArn: aws.String(arn),
						Status:               types.ServiceDeploymentStatusRollbackInProgress,
						CreatedAt:            aws.Time(time.Now().Add(-time.Hour)),
					}},
				}}, middleware.Metadata{}, nil
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

	got, err := app.FindOngoingServiceDeployment(t.Context(), &ecspresso.Service{})
	if err != nil {
		t.Fatal(err)
	}
	if got != arn {
		t.Errorf("unexpected deployment arn: %s", got)
	}
	if requested == nil {
		t.Fatal("ListServiceDeployments is not called")
	}
	if requested.CreatedAt != nil {
		t.Errorf("the deployment must be found regardless of when it started, got createdAt %v", requested.CreatedAt)
	}
	for _, status := range []types.ServiceDeploymentStatus{
		types.ServiceDeploymentStatusInProgress,
		types.ServiceDeploymentStatusRollbackRequested,
		types.ServiceDeploymentStatusRollbackInProgress,
	} {
		if !slices.Contains(requested.Status, status) {
			t.Errorf("status %s is not requested: %v", status, requested.Status)
		}
	}
}
