package ecspresso_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go/middleware"
	"github.com/kayac/ecspresso/v2"
)

var testTaskDefinitionArnForRunSuite = []struct {
	opts []string
	td   string
}{
	{
		opts: []string{"--skip-task-definition"},
		td:   "test:39",
	},
	{
		opts: []string{"--skip-task-definition", "--revision=42"},
		td:   "test:42",
	},
}

func TestTaskDefinitionArnForRun(t *testing.T) {
	ctx := context.TODO()

	// mock aws sdk
	ecspresso.SetAWSV2ConfigLoadOptionsFunc([]func(*config.LoadOptions) error{
		config.WithRegion("ap-northeast-1"),
		config.WithAPIOptions([]func(*middleware.Stack) error{SDKTestingMiddleware()}),
	})
	defer ecspresso.ResetAWSV2ConfigLoadOptionsFunc()

	app, err := ecspresso.New(ctx, &ecspresso.Option{ConfigFilePath: "tests/run-with-sv.yaml"})
	if err != nil {
		t.Error(err)
	}

	for _, s := range testTaskDefinitionArnForRunSuite {
		args := []string{"run", "--dry-run"}
		args = append(args, s.opts...)
		_, cliopts, _, err := ecspresso.ParseCLIv2(args)
		if err != nil {
			t.Error(err)
		}
		opts := *cliopts.Run
		tdArn, err := app.TaskDefinitionArnForRun(ctx, opts)
		if err != nil {
			t.Errorf("%s unexpected error: %s", args, err)
		}
		if ecspresso.ArnToName(tdArn) != s.td {
			t.Errorf("%s expected %s, got %s", args, s.td, tdArn)
		}
	}
}
