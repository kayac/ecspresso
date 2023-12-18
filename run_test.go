package ecspresso_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go/middleware"
	"github.com/kayac/ecspresso/v2"
)

var v2_1_OrLater = false // TODO: set true if v2.1

type taskDefinitionArnForRunSuite struct {
	opts     []string
	td       string
	raiseErr bool
}

var testTaskDefinitionArnForRunSuite = map[string][]taskDefinitionArnForRunSuite{
	"tests/run-with-sv.yaml": {
		{
			opts: []string{"--skip-task-definition"},
			td:   "katsubushi:39",
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
			opts: []string{"--latest-task-definition", "--skip-task-definition"},
			td:   "katsubushi:45",
		},
		{
			opts:     []string{"--latest-task-definition", "--skip-task-definition", "--revision=41"},
			td:       "katsubushi:41",
			raiseErr: true, // latest-task-definition and revision are exclusive
		},
		{
			opts:     []string{"--latest-task-definition", "--revision=41"},
			td:       "katsubushi:41",
			raiseErr: true, // latest-task-definition and revision are exclusive
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
			opts: []string{"--skip-task-definition"},
			td:   "katsubushi:45",
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
			opts: []string{"--latest-task-definition", "--skip-task-definition"},
			td:   "katsubushi:45",
		},
		{
			opts:     []string{"--latest-task-definition", "--revision=42"},
			td:       "katsubushi:42",
			raiseErr: true, // latest-task-definition and revision are exclusive
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
		config.WithAPIOptions([]func(*middleware.Stack) error{
			SDKTestingMiddleware("katsubushi"), // tests/td.json .taskDefinition.family
		}),
	})
	defer ecspresso.ResetAWSV2ConfigLoadOptionsFunc()
	for config, suites := range testTaskDefinitionArnForRunSuite {
		app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{ConfigFilePath: config})
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
			if v2_1_OrLater && s.raiseErr {
				if err == nil {
					t.Errorf("%s %s expected error, but got nil", config, args)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s %s unexpected error: %s", config, args, err)
				continue
			}
			if td := ecspresso.ArnToName(tdArn); td != s.td {
				t.Errorf("%s %s expected %s, got %s", config, args, s.td, td)
			}
		}
	}
}
