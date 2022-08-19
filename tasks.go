package ecspresso

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
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

func (d *App) listTasks(ctx context.Context, id *string, desiredStatuses ...string) ([]*ecs.Task, error) {
	if len(desiredStatuses) == 0 {
		desiredStatuses = []string{"RUNNING", "STOPPED"}
	}
	if aws.StringValue(id) != "" {
		in := &ecs.DescribeTasksInput{
			Cluster: aws.String(d.Cluster),
			Tasks:   []*string{id},
			Include: []*string{aws.String("TAGS")},
		}
		out, err := d.ecs.DescribeTasksWithContext(ctx, in)
		if err != nil {
			return nil, errors.Wrap(err, "failed to describe tasks")
		}
		if len(out.Tasks) == 0 && len(in.Tasks) != 0 {
			return nil, errors.Errorf("task ID %s is not found", *id)
		}
		return out.Tasks, nil
	}

	var tasks []*ecs.Task
	var family string
	if d.config.Service != "" {
		sv, err := d.DescribeService(ctx)
		if err != nil {
			return nil, err
		}
		tdArn := sv.TaskDefinition
		td, err := d.DescribeTaskDefinition(ctx, *tdArn)
		if err != nil {
			return nil, err
		}
		family = aws.StringValue(td.Family)
	} else {
		td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
		if err != nil {
			return nil, err
		}
		family = aws.StringValue(td.Family)
	}
	for _, desiredStatus := range desiredStatuses {
		var nextToken *string
		for {
			out, err := d.ecs.ListTasks(&ecs.ListTasksInput{
				Cluster:       &d.config.Cluster,
				Family:        &family,
				DesiredStatus: aws.String(desiredStatus),
				NextToken:     nextToken,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to list tasks")
			}
			if len(out.TaskArns) == 0 {
				break
			}
			in := &ecs.DescribeTasksInput{
				Cluster: aws.String(d.Cluster),
				Tasks:   out.TaskArns,
				Include: []*string{aws.String("TAGS")},
			}
			taskOut, err := d.ecs.DescribeTasksWithContext(ctx, in)
			if err != nil {
				return nil, errors.Wrap(err, "failed to describe tasks")
			}
			for _, task := range taskOut.Tasks {
				tasks = append(tasks, task)
			}
			if nextToken = out.NextToken; nextToken == nil {
				break
			}
		}
	}
	return tasks, nil
}
