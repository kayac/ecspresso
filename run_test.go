package ecspresso_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/google/go-cmp/cmp"
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
			_, cliopts, _, err := ecspresso.ParseCLIv2(args)
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

func TestRunMultipleTasks(t *testing.T) {
	for _, tc := range []struct {
		name          string
		exitCodes     []int32
		waitUntil     string
		noWait        bool
		directRunTask bool
	}{
		{name: "single task", exitCodes: []int32{0}},
		{name: "RunTask compatibility", exitCodes: []int32{0, 0}, noWait: true, directRunTask: true},
		{name: "all tasks succeed", exitCodes: []int32{0, 0}},
		{name: "second task fails", exitCodes: []int32{0, 42}},
		{name: "first task fails", exitCodes: []int32{42, 0}},
		{name: "multiple tasks fail", exitCodes: []int32{1, 42, 0}},
		{name: "wait until running", exitCodes: []int32{0, 0}, waitUntil: "running"},
		{name: "no wait", exitCodes: []int32{0, 0}, noWait: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const tdArn = "arn:aws:ecs:ap-northeast-1:123456789012:task-definition/test:1"
			var mu sync.Mutex
			described := make([]int, len(tc.exitCodes))
			tasks := make([]types.Task, len(tc.exitCodes))
			for i := range tasks {
				tasks[i].TaskArn = aws.String(fmt.Sprintf("arn:aws:ecs:ap-northeast-1:123456789012:task/default/task-%d", i))
			}
			mock := func(stack *middleware.Stack) error {
				return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("test-run", func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
					var result any
					switch input := in.Parameters.(type) {
					case *ecs.DescribeTaskDefinitionInput:
						result = &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &types.TaskDefinition{
							TaskDefinitionArn: aws.String(tdArn), Family: aws.String("test"),
							ContainerDefinitions: []types.ContainerDefinition{{Name: aws.String("app"), Essential: aws.Bool(true)}},
						}}
					case *ecs.RunTaskInput:
						if got, want := aws.ToInt32(input.Count), int32(len(tasks)); got != want {
							t.Errorf("task count = %d, want %d", got, want)
						}
						result = &ecs.RunTaskOutput{Tasks: tasks}
					case *ecs.DescribeTasksInput:
						var found []types.Task
						for _, arn := range input.Tasks {
							for i, task := range tasks {
								if arn != aws.ToString(task.TaskArn) {
									continue
								}
								mu.Lock()
								described[i]++
								mu.Unlock()
								task.LastStatus = aws.String("STOPPED")
								task.Containers = []types.Container{{Name: aws.String("app"), ExitCode: aws.Int32(tc.exitCodes[i])}}
								if tc.waitUntil == "running" {
									task.LastStatus = aws.String("RUNNING")
									task.Containers[0].ExitCode = nil
								}
								found = append(found, task)
							}
						}
						result = &ecs.DescribeTasksOutput{Tasks: found}
					default:
						return middleware.InitializeOutput{}, middleware.Metadata{}, fmt.Errorf("unexpected request %T", input)
					}
					return middleware.InitializeOutput{Result: result}, middleware.Metadata{}, nil
				}), middleware.Before)
			}
			ecspresso.SetAWSV2ConfigLoadOptionsFunc([]func(*config.LoadOptions) error{
				config.WithRegion("ap-northeast-1"),
				config.WithCredentialsProvider(aws.AnonymousCredentials{}),
				config.WithAPIOptions([]func(*middleware.Stack) error{mock}),
			})
			t.Cleanup(ecspresso.ResetAWSV2ConfigLoadOptionsFunc)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{ConfigFilePath: "tests/run-without-sv.yaml"})
			if err != nil {
				t.Fatal(err)
			}
			until := tc.waitUntil
			if until == "" {
				until = "stopped"
			}
			opt := ecspresso.RunOption{
				Revision: aws.Int64(1), Count: int32(len(tasks)), Wait: !tc.noWait, WaitUntil: until,
			}
			if tc.directRunTask {
				var task *types.Task
				task, err = app.RunTask(ctx, tdArn, &types.TaskOverride{}, &opt)
				if err == nil {
					if task == nil {
						t.Fatal("RunTask returned nil without an error")
					}
					if diff := cmp.Diff(tasks[0].TaskArn, task.TaskArn); diff != "" {
						t.Errorf("first task mismatch (-want +got):\n%s", diff)
					}
				}
			} else {
				err = app.Run(ctx, opt)
			}
			wantError := false
			for i, code := range tc.exitCodes {
				if code == 0 || tc.noWait || until == "running" {
					continue
				}
				wantError = true
				if err == nil || !strings.Contains(err.Error(), aws.ToString(tasks[i].TaskArn)) || !strings.Contains(err.Error(), fmt.Sprintf("exit code: %d", code)) {
					t.Errorf("expected failure for task %d with exit code %d, got %v", i, code, err)
				}
			}
			if !wantError && err != nil {
				t.Fatal(err)
			}
			wantDescribed := make([]int, len(tasks))
			if !tc.noWait {
				for i := range wantDescribed {
					wantDescribed[i] = 2
				} // Waiter and exit status check.
			}
			if diff := cmp.Diff(wantDescribed, described); diff != "" {
				t.Errorf("task status checks mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
