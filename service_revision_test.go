package ecspresso_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kayac/ecspresso/v2"
)

const testServiceRevisionArn = "arn:aws:ecs:ap-northeast-1:123456789012:service-revision/default2/test/1234567890123456789"

func serviceRevisionMiddleware(describeServiceRevisions func() (any, error)) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Finalize.Add(
			middleware.FinalizeMiddlewareFunc(
				"test",
				func(ctx context.Context, in middleware.FinalizeInput, handler middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
					req := in.Request.(*smithyhttp.Request)
					target := strings.SplitN(req.Header.Get("X-Amz-Target"), ".", 2)[1]
					switch target {
					case "DescribeServices":
						return middleware.FinalizeOutput{
							Result: &ecs.DescribeServicesOutput{
								Services: []types.Service{
									{
										ServiceName:    aws.String("test"),
										Status:         aws.String("ACTIVE"),
										TaskDefinition: aws.String("arn:aws:ecs:ap-northeast-1:123456789012:task-definition/katsubushi:39"),
										Deployments: []types.Deployment{
											{Id: aws.String("ecs-svc/1"), Status: aws.String("PRIMARY")},
										},
										CurrentServiceRevisions: []types.ServiceCurrentRevisionSummary{
											{Arn: aws.String(testServiceRevisionArn)},
										},
									},
								},
							},
						}, middleware.Metadata{}, nil
					case "DescribeServiceRevisions":
						out, err := describeServiceRevisions()
						return middleware.FinalizeOutput{Result: out}, middleware.Metadata{}, err
					}
					return middleware.FinalizeOutput{}, middleware.Metadata{}, errors.New("unexpected target " + target)
				},
			),
			middleware.Before,
		)
	}
}

func TestDescribeServiceRevision(t *testing.T) {
	monitoring := &types.MonitoringConfiguration{
		MetricConfigurations: []types.MetricConfiguration{
			{MetricNames: []string{"CPUUtilization"}, ResolutionSeconds: aws.Int32(60)},
		},
	}
	cases := []struct {
		name           string
		revisions      func() (any, error)
		wantErr        string
		wantMonitoring *types.MonitoringConfiguration
		wantGateway    []types.GatewayConnector
	}{
		{
			name: "success",
			revisions: func() (any, error) {
				return &ecs.DescribeServiceRevisionsOutput{
					ServiceRevisions: []types.ServiceRevision{
						{
							ServiceRevisionArn: aws.String(testServiceRevisionArn),
							Monitoring:         monitoring,
							GatewayConnectors:  testGatewayConnectors(),
						},
					},
				}, nil
			},
			wantMonitoring: monitoring,
			wantGateway:    testGatewayConnectors(),
		},
		{
			name: "api error",
			revisions: func() (any, error) {
				return nil, errors.New("AccessDeniedException")
			},
			wantErr: "failed to describe service revision",
		},
		{
			name: "not found",
			revisions: func() (any, error) {
				return &ecs.DescribeServiceRevisionsOutput{
					Failures: []types.Failure{
						{Arn: aws.String(testServiceRevisionArn), Reason: aws.String("MISSING")},
					},
				}, nil
			},
			wantErr: "is not found: MISSING",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ecspresso.SetAWSV2ConfigLoadOptionsFunc([]func(*config.LoadOptions) error{
				config.WithRegion("ap-northeast-1"),
				config.WithAPIOptions([]func(*middleware.Stack) error{
					serviceRevisionMiddleware(c.revisions),
				}),
			})
			defer ecspresso.ResetAWSV2ConfigLoadOptionsFunc()

			app, err := ecspresso.New(t.Context(), &ecspresso.CLIOptions{ConfigFilePath: "tests/run-with-sv.yaml"})
			if err != nil {
				t.Fatal(err)
			}
			sv, err := app.DescribeService(t.Context())
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("expected error containing %q, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.wantMonitoring, sv.Monitoring, cmpopts.IgnoreUnexported(types.MonitoringConfiguration{}, types.MetricConfiguration{})); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(c.wantGateway, sv.GatewayConnectors, ignoreGatewayConnectorUnexported); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
