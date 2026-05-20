package ecspresso_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go/middleware"
	"github.com/kayac/ecspresso/v2"
)

type taskDefinitionArnForRunSuite struct {
	opts     []string
	arn      string
	family   string
	raiseErr bool
}

var testTaskDefinitionArnForRunSuite = map[string][]taskDefinitionArnForRunSuite{
	"tests/run-with-sv.yaml": {
		{
			opts: []string{"--skip-task-definition"},
			arn:  "katsubushi:39",
		},
		{
			opts: []string{"--skip-task-definition", "--revision=42"},
			arn:  "katsubushi:42",
		},
		{
			opts: []string{"--latest-task-definition"},
			arn:  "katsubushi:45",
		},
		{
			opts: []string{"--latest-task-definition", "--skip-task-definition"},
			arn:  "katsubushi:45",
		},
		{
			opts:     []string{"--latest-task-definition", "--skip-task-definition", "--revision=41"},
			arn:      "katsubushi:41",
			raiseErr: true, // latest-task-definition and revision are exclusive
		},
		{
			opts:     []string{"--latest-task-definition", "--revision=41"},
			arn:      "katsubushi:41",
			raiseErr: true, // latest-task-definition and revision are exclusive
		},
		{
			opts:   nil,
			arn:    "",
			family: "katsubushi",
		},
		{
			opts:   []string{"--task-def=tests/run-test-td.json"},
			arn:    "",
			family: "test",
		},
	},
	"tests/run-without-sv.yaml": {
		{
			opts: []string{"--skip-task-definition"},
			arn:  "katsubushi:45",
		},
		{
			opts: []string{"--skip-task-definition", "--revision=42"},
			arn:  "katsubushi:42",
		},
		{
			opts: []string{"--latest-task-definition"},
			arn:  "katsubushi:45",
		},
		{
			opts: []string{"--latest-task-definition", "--skip-task-definition"},
			arn:  "katsubushi:45",
		},
		{
			opts:     []string{"--latest-task-definition", "--revision=42"},
			arn:      "katsubushi:42",
			raiseErr: true, // latest-task-definition and revision are exclusive
		},
		{
			opts:   nil,
			arn:    "",
			family: "katsubushi",
		},
		{
			opts:   []string{"--task-def=tests/run-test-td.json"},
			arn:    "",
			family: "test",
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
			_, cliopts, _, err := ecspresso.ParseCLI(args)
			if err != nil {
				t.Error(err)
			}
			opts := *cliopts.Run
			td, err := app.ResolveTaskDefinitionForRun(ctx, opts)
			if s.raiseErr {
				if err == nil {
					t.Errorf("%s %s expected error, but got nil", config, args)
				}
				continue
			}
			if err != nil {
				t.Errorf("%s %s unexpected error: %s", config, args, err)
				continue
			}
			if td.Arn != "" {
				if name := ecspresso.ArnToName(td.Arn); name != s.arn {
					t.Errorf("%s %s expected %s, got %s", config, args, s.arn, name)
				}
			} else {
				family := aws.ToString(td.TaskDefinitionInput.Family)
				if family != s.family {
					t.Errorf("%s %s expected %s, got %s", config, args, s.family, family)
				}
			}
		}
	}
}
