package ecspresso_test

import (
	"context"
	"fmt"
	"strings"
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
			app := newRunTestApp(t, tasks, nil)

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
		})
	}
}

func TestRunWaitsForAllTasks(t *testing.T) {
	for _, tc := range []struct {
		until  string
		status string
	}{
		{until: "stopped", status: "STOPPED"},
		{until: "running", status: "RUNNING"},
	} {
		t.Run(tc.until, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			tasks := runTestTasks(tc.status, 0, 0)
			secondStarted := make(chan struct{}, 1)
			releaseSecond := make(chan struct{})
			app := newRunTestApp(t, tasks, func(ctx context.Context, task types.Task) (types.Task, error) {
				if aws.ToString(task.TaskArn) == aws.ToString(tasks[1].TaskArn) {
					select {
					case secondStarted <- struct{}{}:
					default:
					}
					select {
					case <-releaseSecond:
					case <-ctx.Done():
						return types.Task{}, ctx.Err()
					}
				}
				return task, nil
			})

			done := make(chan error, 1)
			go func() {
				done <- app.Run(ctx, ecspresso.RunOption{
					Revision: aws.Int64(1), Count: 2, Wait: true, WaitUntil: tc.until,
				})
			}()

			select {
			case <-secondStarted:
			case err := <-done:
				t.Fatalf("Run returned without waiting for the second task: %v", err)
			case <-ctx.Done():
				t.Fatal("second task was not checked")
			}
			select {
			case err := <-done:
				t.Fatalf("Run returned while the second task was still waiting: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			close(releaseSecond)
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("Run did not finish after both tasks reached the target status")
			}
		})
	}
}

func TestRunNoWait(t *testing.T) {
	app := newRunTestApp(t, runTestTasks("PENDING", 0, 0), func(ctx context.Context, task types.Task) (types.Task, error) {
		t.Error("--no-wait must not describe task status")
		return types.Task{}, fmt.Errorf("unexpected DescribeTasks call")
	})

	err := app.Run(t.Context(), ecspresso.RunOption{
		Revision: aws.Int64(1), Count: 2, Wait: false,
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestRunTaskCompatibility(t *testing.T) {
	tasks := runTestTasks("PENDING", 0, 0)
	app := newRunTestApp(t, tasks, nil)

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

// AWS configuration is global, so callers must not use t.Parallel.
func newRunTestApp(t *testing.T, tasks []types.Task, describeTask func(context.Context, types.Task) (types.Task, error)) *ecspresso.App {
	t.Helper()
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
					for _, task := range tasks {
						if arn == aws.ToString(task.TaskArn) {
							if describeTask != nil {
								var err error
								task, err = describeTask(ctx, task)
								if err != nil {
									return middleware.InitializeOutput{}, middleware.Metadata{}, err
								}
							}
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
	return app
}
