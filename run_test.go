package ecspresso_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go/middleware"
	"github.com/kayac/ecspresso/v2"
)

type taskDefinitionArnForRunSuite struct {
	opts     []string
	td       string
	raiseErr bool
}

var testTaskDefinitionArnForRunSuite = map[string][]taskDefinitionArnForRunSuite{
	"tests/run-with-sv.yaml": {
		{
			opts: []string{"--skip-task-definition"},
			td:   "test:39",
		},
		{
			opts: []string{"--skip-task-definition", "--revision=42"},
			td:   "test:42",
		},
		{
			opts: []string{"--latest-task-definition"},
			td:   "test:45",
		},
		{
			opts:     []string{"--latest-task-definition", "--skip-task-definition"},
			raiseErr: true,
		},
		{
			opts:     []string{"--latest-task-definition", "--skip-task-definition", "--revision=42"},
			raiseErr: true,
		},
		{
			opts: nil,
			td:   "family katsubushi will be registered",
		},
		{
			opts: []string{"--task-def=tests/run-test-td.json"},
			td:   "family test will be registered",
		},
	},
	"tests/run-without-sv.yaml": {
		{
			opts:     []string{"--skip-task-definition"},
			raiseErr: true, // without service, --skip-task-definition is not allowed
		},
		{
			opts: []string{"--skip-task-definition", "--revision=42"},
			td:   "katsubushi:42",
		},
		{
			opts: []string{"--latest-task-definition"},
			td:   "katsubushi:45",
		},
		{
			opts:     []string{"--latest-task-definition", "--skip-task-definition"},
			raiseErr: true, // without service, --skip-task-definition is not allowed
		},
		{
			opts: []string{"--latest-task-definition", "--revision=42"},
			td:   "katsubushi:42",
		},
		{
			opts: nil,
			td:   "family katsubushi will be registered",
		},
		{
			opts: []string{"--task-def=tests/run-test-td.json"},
			td:   "family test will be registered",
		},
	},
}

func TestTaskDefinitionArnForRun(t *testing.T) {
	ctx := context.TODO()

	// mock aws sdk
	ecspresso.SetAWSV2ConfigLoadOptionsFunc([]func(*config.LoadOptions) error{
		config.WithRegion("ap-northeast-1"),
		config.WithAPIOptions([]func(*middleware.Stack) error{SDKTestingMiddleware("test")}),
	})
	defer ecspresso.ResetAWSV2ConfigLoadOptionsFunc()
	for config, suites := range testTaskDefinitionArnForRunSuite {
		app, err := ecspresso.New(ctx, &ecspresso.Option{ConfigFilePath: config})
		if err != nil {
			t.Error(err)
			continue
		}
		for _, s := range suites {
			args := []string{"run", "--dry-run"}
			args = append(args, s.opts...)
			_, cliopts, _, err := ecspresso.ParseCLIv2(args)
			if err != nil {
				t.Error(err)
			}
			opts := *cliopts.Run
			tdArn, err := app.TaskDefinitionArnForRun(ctx, opts)
			if s.raiseErr && err == nil {
				t.Errorf("%s %s expected error, but got nil", config, args)
			} else if err != nil {
				t.Errorf("%s %s unexpected error: %s", config, args, err)
			} else if td := ecspresso.ArnToName(tdArn); td != s.td {
				t.Errorf("%s %s expected %s, got %s", config, args, s.td, td)
			}
		}
	}
}
