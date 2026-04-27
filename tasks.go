package ecspresso

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fujiwara/ecsta"
)

type TasksOption struct {
	ID     string `help:"task ID" default:""`
	Output string `help:"output format" enum:"table,json,tsv" default:"table"`

	List  *TasksListOption  `cmd:"" default:"withargs" help:"list tasks"`
	Find  *TasksFindOption  `cmd:"" help:"find a task from tasks list and dump it as JSON"`
	Stop  *TasksStopOption  `cmd:"" help:"stop a task"`
	Trace *TasksTraceOption `cmd:"" help:"trace a task"`
	Logs  *TasksLogsOption  `cmd:"" help:"show logs of a task"`
}

// TasksListOption is the default subcommand for tasks.
// Deprecated flags are kept as hidden for backward compatibility.
type TasksListOption struct {
	DeprecatedFind  bool `name:"find" hidden:"" default:"false"`
	DeprecatedStop  bool `name:"stop" hidden:"" default:"false"`
	DeprecatedForce bool `name:"force" hidden:"" default:"false"`
	DeprecatedTrace bool `name:"trace" hidden:"" default:"false"`
}

type TasksFindOption struct{}

type TasksStopOption struct {
	Force bool `help:"stop the task without confirmation" default:"false"`
}

type TasksTraceOption struct{}

type TasksLogsOption struct {
	Follow    bool          `help:"follow logs" short:"f" default:"false"`
	Duration  time.Duration `help:"duration of logs" short:"d" default:"1m"`
	StartTime string        `help:"start time of logs" short:"s" default:""`
	Container string        `help:"container name" default:""`
}

func (o TasksOption) taskID() string {
	return o.ID
}

func (d *App) Tasks(ctx context.Context, opt TasksOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	ecstaApp, err := d.NewEcsta(ctx)
	if err != nil {
		return err
	}
	ecstaApp.Config.Set("output", opt.Output)

	family, service, err := d.resolveEcstaFilters(ctx)
	if err != nil {
		return err
	}

	switch {
	case opt.Find != nil:
		return ecstaApp.RunDescribe(ctx, &ecsta.DescribeOption{
			ID:      opt.taskID(),
			Family:  family,
			Service: service,
		})
	case opt.Stop != nil:
		return ecstaApp.RunStop(ctx, &ecsta.StopOption{
			ID:      opt.taskID(),
			Force:   opt.Stop.Force,
			Family:  family,
			Service: service,
		})
	case opt.Trace != nil:
		return ecstaApp.RunTrace(ctx, &ecsta.TraceOption{
			ID:       opt.taskID(),
			Duration: time.Minute,
			Family:   family,
			Service:  service,
		})
	case opt.Logs != nil:
		return ecstaApp.RunLogs(ctx, &ecsta.LogsOption{
			ID:        opt.taskID(),
			Follow:    opt.Logs.Follow,
			Duration:  opt.Logs.Duration,
			StartTime: opt.Logs.StartTime,
			Container: opt.Logs.Container,
			Family:    family,
			Service:   service,
			JSON:      logFormat == logFormatJSON,
		})
	case opt.List != nil:
		list := opt.List
		switch {
		case list.DeprecatedFind:
			LogWarn("--find flag is deprecated, use 'tasks find' subcommand instead")
			return ecstaApp.RunDescribe(ctx, &ecsta.DescribeOption{
				ID:      opt.taskID(),
				Family:  family,
				Service: service,
			})
		case list.DeprecatedStop:
			LogWarn("--stop flag is deprecated, use 'tasks stop' subcommand instead")
			return ecstaApp.RunStop(ctx, &ecsta.StopOption{
				ID:      opt.taskID(),
				Force:   list.DeprecatedForce,
				Family:  family,
				Service: service,
			})
		case list.DeprecatedTrace:
			LogWarn("--trace flag is deprecated, use 'tasks trace' subcommand instead")
			return ecstaApp.RunTrace(ctx, &ecsta.TraceOption{
				ID:       opt.taskID(),
				Duration: time.Minute,
				Family:   family,
				Service:  service,
			})
		default:
			return ecstaApp.RunList(ctx, &ecsta.ListOption{
				Family:  family,
				Service: service,
			})
		}
	}
	return nil
}

func (d *App) taskDefinitionFamily(ctx context.Context) (string, error) {
	var family string
	if d.config.Service != "" {
		sv, err := d.DescribeService(ctx)
		if err != nil {
			return "", err
		}
		tdArn := sv.TaskDefinition
		td, err := d.DescribeTaskDefinition(ctx, *tdArn)
		if err != nil {
			return "", err
		}
		family = aws.ToString(td.Family)
	} else {
		td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
		if err != nil {
			return "", err
		}
		family = aws.ToString(td.Family)
	}
	return family, nil
}

func (d *App) listTasks(ctx context.Context) ([]types.Task, error) {
	tasks := []types.Task{}
	family, err := d.taskDefinitionFamily(ctx)
	if err != nil {
		return nil, err
	}
	for _, status := range []types.DesiredStatus{types.DesiredStatusRunning, types.DesiredStatusStopped} {
		tp := ecs.NewListTasksPaginator(
			d.ecs,
			&ecs.ListTasksInput{
				Cluster:       &d.config.Cluster,
				Family:        &family,
				DesiredStatus: status,
			},
		)
		for tp.HasMorePages() {
			to, err := tp.NextPage(ctx)
			if err != nil {
				return nil, err
			}
			if len(to.TaskArns) == 0 {
				continue
			}
			out, err := d.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{
				Cluster: &d.config.Cluster,
				Tasks:   to.TaskArns,
				Include: []types.TaskField{"TAGS"},
			})
			if err != nil {
				return nil, err
			}
			tasks = append(tasks, out.Tasks...)
		}
	}
	return tasks, nil
}
