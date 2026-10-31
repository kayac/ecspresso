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
		name       string
		exitCodes  []int32
		wantErrors []string
	}{
		{name: "single task", exitCodes: []int32{0}},
		{name: "all tasks succeed", exitCodes: []int32{0, 0}},
		{name: "second task fails", exitCodes: []int32{0, 42}, wantErrors: []string{"task-1", "exit code: 42"}},
		{name: "first task fails", exitCodes: []int32{42, 0}, wantErrors: []string{"task-0", "exit code: 42"}},
		{name: "multiple tasks fail", exitCodes: []int32{1, 42, 0}, wantErrors: []string{"task-0", "task-1", "exit code: 1", "exit code: 42"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tasks := runTestTasks("STOPPED", tc.exitCodes...)
			app, described := newRunTestApp(t, tasks)

			err := app.Run(t.Context(), ecspresso.RunOption{
				Revision: aws.Int64(1), Count: int32(len(tasks)), Wait: true, WaitUntil: "stopped",
			})

			if len(tc.wantErrors) == 0 {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("expected task failures, got nil")
				}
				for _, want := range tc.wantErrors {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not contain %q", err, want)
					}
				}
			}
			for i, calls := range described {
				if calls != 2 { // Waiter and exit status check.
					t.Errorf("task %d: got %d status checks, want 2", i, calls)
				}
			}
		})
	}
}

func TestRunWaitUntilRunning(t *testing.T) {
	app, described := newRunTestApp(t, runTestTasks("RUNNING", 0, 0))

	err := app.Run(t.Context(), ecspresso.RunOption{
		Revision: aws.Int64(1), Count: 2, Wait: true, WaitUntil: "running",
	})

	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]int{2, 2}, described); diff != "" {
		t.Errorf("task status checks mismatch (-want +got):\n%s", diff)
	}
}

func TestRunNoWait(t *testing.T) {
	app, described := newRunTestApp(t, runTestTasks("PENDING", 0, 0))

	err := app.Run(t.Context(), ecspresso.RunOption{
		Revision: aws.Int64(1), Count: 2, Wait: false,
	})

	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]int{0, 0}, described); diff != "" {
		t.Errorf("unexpected task status checks (-want +got):\n%s", diff)
	}
}

func TestRunTaskCompatibility(t *testing.T) {
	tasks := runTestTasks("PENDING", 0, 0)
	app, described := newRunTestApp(t, tasks)

	var task *types.Task
	task, err := app.RunTask(t.Context(), "test:1", &types.TaskOverride{}, &ecspresso.RunOption{Count: 2})

	if err != nil {
		t.Fatal(err)
	}
	if task == nil {
		t.Fatal("RunTask returned nil without an error")
	}
	if diff := cmp.Diff(tasks[0].TaskArn, task.TaskArn); diff != "" {
		t.Errorf("first task mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]int{0, 0}, described); diff != "" {
		t.Errorf("unexpected task status checks (-want +got):\n%s", diff)
	}
}

func runTestTasks(status string, exitCodes ...int32) []types.Task {
	tasks := make([]types.Task, len(exitCodes))
	for i, code := range exitCodes {
		tasks[i] = types.Task{
			TaskArn:    aws.String(fmt.Sprintf("arn:aws:ecs:ap-northeast-1:123456789012:task/default/task-%d", i)),
			LastStatus: aws.String(status),
			Containers: []types.Container{{Name: aws.String("app")}},
		}
		if status == "STOPPED" {
			tasks[i].Containers[0].ExitCode = aws.Int32(code)
		}
	}
	return tasks
}

// The returned counts may be read after Run or RunTask returns.
// AWS configuration is global, so callers must not use t.Parallel.
func newRunTestApp(t *testing.T, tasks []types.Task) (*ecspresso.App, []int) {
	t.Helper()
	var mu sync.Mutex
	described := make([]int, len(tasks))
	mock := func(stack *middleware.Stack) error {
		return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("test-run", func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
			var result any
			switch input := in.Parameters.(type) {
			case *ecs.DescribeTaskDefinitionInput:
				result = &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &types.TaskDefinition{
					TaskDefinitionArn: aws.String("test:1"), Family: aws.String("test"),
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
						if arn == aws.ToString(task.TaskArn) {
							mu.Lock()
							described[i]++
							mu.Unlock()
							found = append(found, task)
						}
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
	app, err := ecspresso.New(t.Context(), &ecspresso.CLIOptions{ConfigFilePath: "tests/run-without-sv.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	app.Config().Timeout.Duration = 5 * time.Second
	return app, described
}
