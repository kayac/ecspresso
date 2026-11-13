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
	"github.com/kayac/ecspresso/v3"
)

const (
	testTaskDefinitionArnPrefix = "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test:"
	testSourceRevisionArn       = "arn:aws:ecs:ap-northeast-1:123456789012:service-revision/default/test/1"
)

// newRollbackTestApp returns an App whose ECS client answers the operations
// in outputs, and records the operations it received in calls.
func newRollbackTestApp(t *testing.T, outputs map[string]any) (*ecspresso.App, *[]string) {
	t.Helper()
	calls := []string{}
	ecspresso.SetAWSV2ConfigLoadOptionsFunc([]func(*config.LoadOptions) error{
		config.WithRegion("ap-northeast-1"),
		config.WithAPIOptions([]func(*middleware.Stack) error{
			func(stack *middleware.Stack) error {
				return stack.Finalize.Add(
					middleware.FinalizeMiddlewareFunc(
						"test",
						func(ctx context.Context, in middleware.FinalizeInput, handler middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
							req := in.Request.(*smithyhttp.Request)
							target := strings.SplitN(req.Header.Get("X-Amz-Target"), ".", 2)[1]
							calls = append(calls, target)
							if out, ok := outputs[target]; ok {
								return middleware.FinalizeOutput{Result: out}, middleware.Metadata{}, nil
							}
							return middleware.FinalizeOutput{}, middleware.Metadata{}, errors.New("unexpected target " + target)
						},
					),
					middleware.Before,
				)
			},
		}),
	})
	t.Cleanup(ecspresso.ResetAWSV2ConfigLoadOptionsFunc)

	app, err := ecspresso.New(t.Context(), &ecspresso.CLIOptions{ConfigFilePath: "tests/run-with-sv.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	return app, &calls
}

func TestRollbackTargetOfServiceDeployment(t *testing.T) {
	source := types.ServiceRevisionSummary{Arn: aws.String(testSourceRevisionArn)}
	cases := []struct {
		name      string
		dp        *types.ServiceDeployment
		want      string
		wantCalls []string
	}{
		{
			// The task definition of the source service revision, even if
			// newer revisions that were never deployed are registered.
			name:      "one source revision",
			dp:        &types.ServiceDeployment{SourceServiceRevisions: []types.ServiceRevisionSummary{source}},
			want:      testTaskDefinitionArnPrefix + "2",
			wantCalls: []string{"DescribeServiceRevisions"},
		},
		{
			name:      "no source revision",
			dp:        &types.ServiceDeployment{},
			wantCalls: []string{},
		},
		{
			name:      "multiple source revisions",
			dp:        &types.ServiceDeployment{SourceServiceRevisions: []types.ServiceRevisionSummary{source, source}},
			wantCalls: []string{},
		},
		{
			name:      "deployment not found",
			wantCalls: []string{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app, calls := newRollbackTestApp(t, map[string]any{
				"DescribeServiceRevisions": &ecs.DescribeServiceRevisionsOutput{
					ServiceRevisions: []types.ServiceRevision{
						{
							ServiceRevisionArn: aws.String(testSourceRevisionArn),
							TaskDefinition:     aws.String(testTaskDefinitionArnPrefix + "2"),
						},
					},
				},
			})
			got, err := app.RollbackTargetOfServiceDeployment(t.Context(), c.dp)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("unexpected target: want %q, got %q", c.want, got)
			}
			if diff := cmp.Diff(c.wantCalls, *calls); diff != "" {
				t.Errorf("calls mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDeregisterRolledBack(t *testing.T) {
	rolledBack := testTaskDefinitionArnPrefix + "7"
	cases := []struct {
		name      string
		target    string
		opt       ecspresso.RollbackOption
		wantCalls []string
	}{
		{
			name:      "rolled back to another task definition",
			target:    testTaskDefinitionArnPrefix + "2",
			opt:       ecspresso.RollbackOption{DeregisterTaskDefinition: true},
			wantCalls: []string{"DeregisterTaskDefinition"},
		},
		{
			// e.g. a deployment that changed only the service definition
			name:      "rolled back to the same task definition",
			target:    rolledBack,
			opt:       ecspresso.RollbackOption{DeregisterTaskDefinition: true},
			wantCalls: []string{},
		},
		{
			name:      "target is unknown",
			opt:       ecspresso.RollbackOption{DeregisterTaskDefinition: true},
			wantCalls: []string{},
		},
		{
			name:      "dry run",
			target:    testTaskDefinitionArnPrefix + "2",
			opt:       ecspresso.RollbackOption{DeregisterTaskDefinition: true, DryRun: true},
			wantCalls: []string{},
		},
		{
			name:      "no deregister",
			target:    testTaskDefinitionArnPrefix + "2",
			opt:       ecspresso.RollbackOption{},
			wantCalls: []string{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app, calls := newRollbackTestApp(t, map[string]any{
				"DeregisterTaskDefinition": &ecs.DeregisterTaskDefinitionOutput{},
			})
			if err := app.DeregisterRolledBack(t.Context(), rolledBack, c.target, c.opt); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.wantCalls, *calls); diff != "" {
				t.Errorf("calls mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
