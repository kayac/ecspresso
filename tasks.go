package ecspresso

import (
	"context"
	"time"

	ecsv2 "github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ecsv2Types "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/fujiwara/ecsta"
	"github.com/pkg/errors"
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
	return aws.StringValue(o.ID)
}

func (d *App) Tasks(opt TasksOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	ecstaApp, err := d.NewEcsta(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create ecsta")
	}
	ecstaApp.Config.Set("output", aws.StringValue(opt.Output))

	if aws.BoolValue(opt.Find) {
		return ecstaApp.RunDescribe(ctx, &ecsta.DescribeOption{
			ID: opt.taskID(),
		})
	} else if aws.BoolValue(opt.Stop) {
		return ecstaApp.RunStop(ctx, &ecsta.StopOption{
			ID:    opt.taskID(),
			Force: aws.BoolValue(opt.Force),
		})
	} else if aws.BoolValue(opt.Trace) {
		return ecstaApp.RunTrace(ctx, &ecsta.TraceOption{
			ID:       opt.taskID(),
			Duration: time.Minute,
		})
	} else {
		family, err := d.taskDefinitionFamily(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get task definition family")
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
		family = aws.StringValue(td.Family)
	} else {
		td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
		if err != nil {
			return "", err
		}
		family = aws.StringValue(td.Family)
	}
	return family, nil
}

func (d *App) listTasks(ctx context.Context) ([]ecsv2Types.Task, error) {
	tasks := []types.Task{}
	family, err := d.taskDefinitionFamily(ctx)
	if err != nil {
		return nil, err
	}
	for _, status := range []ecsv2Types.DesiredStatus{ecsv2Types.DesiredStatusRunning, ecsv2Types.DesiredStatusStopped} {
		tp := ecsv2.NewListTasksPaginator(
			d.ecsv2,
			&ecsv2.ListTasksInput{
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
			out, err := d.ecsv2.DescribeTasks(ctx, &ecsv2.DescribeTasksInput{
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
