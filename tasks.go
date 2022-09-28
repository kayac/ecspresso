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
	ID     *string
	Output *string
	Find   *bool
	Stop   *bool
	Force  *bool
	Trace  *bool
}

func (o TasksOption) taskID() string {
	return aws.ToString(o.ID)
}

func (d *App) Tasks(ctx context.Context, opt TasksOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	ecstaApp, err := d.NewEcsta(ctx)
	if err != nil {
		return err
	}
	ecstaApp.Config.Set("output", aws.ToString(opt.Output))

	if aws.ToBool(opt.Find) {
		return ecstaApp.RunDescribe(ctx, &ecsta.DescribeOption{
			ID: opt.taskID(),
		})
	} else if aws.ToBool(opt.Stop) {
		return ecstaApp.RunStop(ctx, &ecsta.StopOption{
			ID:    opt.taskID(),
			Force: aws.ToBool(opt.Force),
		})
	} else if aws.ToBool(opt.Trace) {
		return ecstaApp.RunTrace(ctx, &ecsta.TraceOption{
			ID:       opt.taskID(),
			Duration: time.Minute,
		})
	} else {
		family, err := d.taskDefinitionFamily(ctx)
		if err != nil {
			return err
		}
		return ecstaApp.RunList(ctx, &ecsta.ListOption{
			Family: family,
		})
	}
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
