package ecspresso_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go/middleware"
	"github.com/kayac/ecspresso/v2"
)

var testTaskDefinitionArnForRunSuite = []struct {
	config   string
	opts     []string
	td       string
	raiseErr bool
}{
	{
		config: "tests/run-with-sv.yaml",
		opts:   []string{"--skip-task-definition"},
		td:     "test:39",
	},
	{
		config: "tests/run-with-sv.yaml",
		opts:   []string{"--skip-task-definition", "--revision=42"},
		td:     "test:42",
	},
	{
		config: "tests/run-with-sv.yaml",
		opts:   []string{"--latest-task-definition"},
		td:     "test:45",
	},
	{
		config: "tests/run-with-sv.yaml",
		opts:   []string{"--latest-task-definition", "--skip-task-definition"},
		td:     "test:45",
	},
	{
		config: "tests/run-with-sv.yaml",
		opts:   []string{"--latest-task-definition", "--skip-task-definition", "--revision=42"},
		td:     "test:42",
	},
	{
		config: "tests/run-with-sv.yaml",
		opts:   nil,
		td:     "family katsubushi will be registered",
	},
	{
		config: "tests/run-with-sv.yaml",
		opts:   []string{"--task-def=tests/run-test-td.json"},
		td:     "family test will be registered",
	},
	{
		config:   "tests/run-without-sv.yaml",
		opts:     []string{"--skip-task-definition"},
		raiseErr: true, // without service, --skip-task-definition is not allowed
	},
	{
		config: "tests/run-without-sv.yaml",
		opts:   []string{"--skip-task-definition", "--revision=42"},
		td:     "katsubushi:42",
	},
	{
		config: "tests/run-without-sv.yaml",
		opts:   []string{"--latest-task-definition"},
		td:     "katsubushi:45",
	},
	{
		config:   "tests/run-without-sv.yaml",
		opts:     []string{"--latest-task-definition", "--skip-task-definition"},
		raiseErr: true, // without service, --skip-task-definition is not allowed
	},
	{
		config: "tests/run-without-sv.yaml",
		opts:   []string{"--latest-task-definition", "--revision=42"},
		td:     "katsubushi:42",
	},
	{
		config: "tests/run-without-sv.yaml",
		opts:   nil,
		td:     "family katsubushi will be registered",
	},
	{
		config: "tests/run-without-sv.yaml",
		opts:   []string{"--task-def=tests/run-test-td.json"},
		td:     "family test will be registered",
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

	for _, s := range testTaskDefinitionArnForRunSuite {
		app, err := ecspresso.New(ctx, &ecspresso.Option{ConfigFilePath: s.config})
		if err != nil {
			t.Error(err)
			continue
		}
		args := []string{"run", "--dry-run"}
		args = append(args, s.opts...)
		_, cliopts, _, err := ecspresso.ParseCLIv2(args)
		if err != nil {
			t.Error(err)
		}
		opts := *cliopts.Run
		tdArn, err := app.TaskDefinitionArnForRun(ctx, opts)
		if err != nil {
			t.Errorf("%s %s unexpected error: %s", s.config, args, err)
		}
		if ecspresso.ArnToName(tdArn) != s.td {
			t.Errorf("%s %s expected %s, got %s", s.config, args, s.td, tdArn)
		}
	}
}
