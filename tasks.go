package ecspresso

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
)

type TasksOption struct {
	ID     *string
	Output *string
	Find   *bool
	Stop   *bool
	Force  *bool
}

func (o TasksOption) taskID() string {
	return aws.StringValue(o.ID)
}

func (opt TasksOption) newFormatter() taskFormatter {
	switch *opt.Output {
	case "json":
		return newTaskFormatterJSON(os.Stdout)
	case "tsv":
		return newTaskFormatterTSV(os.Stdout, true)
	}
	return newTaskFormatterTable(os.Stdout)
}

func (d *App) listTasks(ctx context.Context, id *string, desiredStatuses ...string) ([]*ecs.Task, error) {
	if len(desiredStatuses) == 0 {
		desiredStatuses = []string{"RUNNING", "STOPPED"}
	}
	var taskIDs []*string
	if aws.StringValue(id) != "" {
		taskIDs = []*string{id}
	} else {
		sv, err := d.DescribeService(ctx)
		if err != nil {
			return nil, err
		}
		td, err := d.DescribeTaskDefinition(ctx, *sv.TaskDefinition)
		if err != nil {
			return nil, err
		}
		for _, desiredStatus := range desiredStatuses {
			var nextToken *string
			for {
				out, err := d.ecs.ListTasks(&ecs.ListTasksInput{
					Cluster:       &d.config.Cluster,
					Family:        td.Family,
					DesiredStatus: aws.String(desiredStatus),
					NextToken:     nextToken,
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to list tasks")
				}
				for _, id := range out.TaskArns {
					taskIDs = append(taskIDs, aws.String(arnToName(*id)))
				}
				if nextToken = out.NextToken; nextToken == nil {
					break
				}
			}
		}
	}
	if len(taskIDs) == 0 {
		return []*ecs.Task{}, nil
	}

	in := &ecs.DescribeTasksInput{
		Cluster: aws.String(d.Cluster),
		Tasks:   taskIDs,
	}
	out, err := d.ecs.DescribeTasksWithContext(ctx, in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to describe tasks")
	}
	if len(out.Tasks) == 0 && len(in.Tasks) != 0 {
		return nil, errors.Errorf("task ID %s is not found", *in.Tasks[0])
	}
	return out.Tasks, nil
}

func (d *App) Tasks(opt TasksOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	tasks, err := d.listTasks(ctx, opt.ID)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		d.Log("tasks not found")
		return nil
	}

	if !aws.BoolValue(opt.Find) && !aws.BoolValue(opt.Stop) {
		formatter := opt.newFormatter()
		for _, task := range tasks {
			formatter.AddTask(task)
		}
		formatter.Close()
		return nil
	}

	task, err := d.findTask(opt, tasks)
	if err != nil {
		return err
	}

	if aws.BoolValue(opt.Find) {
		f := newTaskFormatterJSON(os.Stdout)
		f.AddTask(task)
		f.Close()
		return nil
	} else if aws.BoolValue(opt.Stop) {
		stop := aws.BoolValue(opt.Force) ||
			prompter.YN(fmt.Sprintf("Stop task %s?", arnToName(*task.TaskArn)), false)
		if !stop {
			return nil
		}
		d.Log("Request stop task ID " + arnToName(*task.TaskArn))
		_, err := d.ecs.StopTask(&ecs.StopTaskInput{
			Cluster: task.ClusterArn,
			Task:    task.TaskArn,
			Reason:  aws.String("Request stop task by user action."),
		})
		if err != nil {
			return errors.Wrap(err, "failed to stop task")
		}
	}
	return nil
}

func (d *App) findTask(opt taskFinderOption, tasks []*ecs.Task) (*ecs.Task, error) {
	if len(tasks) == 1 && opt.taskID() == arnToName(*tasks[0].TaskArn) {
		return tasks[0], nil
	}
	buf := new(bytes.Buffer)
	tasksDict := make(map[string]*ecs.Task)
	formatter := newTaskFormatterTSV(buf, false)
	for _, task := range tasks {
		task := task
		formatter.AddTask(task)
		tasksDict[arnToName(*task.TaskArn)] = task
	}
	formatter.Close()
	result, err := d.runFilter(buf, "task ID")
	if err != nil {
		return nil, err
	}
	taskID := strings.Fields(string(result))[0]
	if task, found := tasksDict[taskID]; found {
		return task, nil
	}
	return nil, errors.Errorf("task ID %s is not found", taskID)
}

type taskFormatter interface {
	AddTask(*ecs.Task)
	Close()
}

var taskFormatterColumns = []string{
	"ID",
	"TaskDefinition",
	"Instance",
	"LastStatus",
	"DesiredStatus",
	"CreatedAt",
	"Group",
	"Type",
}

func taskToColumns(task *ecs.Task) []string {
	return []string{
		arnToName(*task.TaskArn),
		arnToName(*task.TaskDefinitionArn),
		arnToName(aws.StringValue(task.ContainerInstanceArn)),
		aws.StringValue(task.LastStatus),
		aws.StringValue(task.DesiredStatus),
		task.CreatedAt.In(time.Local).Format(time.RFC3339),
		aws.StringValue(task.Group),
		aws.StringValue(task.LaunchType),
	}
}

type taskFormatterTable struct {
	table *tablewriter.Table
}

func newTaskFormatterTable(w io.Writer) *taskFormatterTable {
	t := &taskFormatterTable{
		table: tablewriter.NewWriter(w),
	}
	t.table.SetHeader(taskFormatterColumns)
	t.table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	return t
}

func (t *taskFormatterTable) AddTask(task *ecs.Task) {
	t.table.Append(taskToColumns(task))
}

func (t *taskFormatterTable) Close() {
	t.table.Render()
}

type taskFormatterTSV struct {
	w io.Writer
}

func newTaskFormatterTSV(w io.Writer, header bool) *taskFormatterTSV {
	t := &taskFormatterTSV{w: w}
	if header {
		fmt.Fprintln(t.w, strings.Join(taskFormatterColumns, "\t"))
	}
	return t
}

func (t *taskFormatterTSV) AddTask(task *ecs.Task) {
	fmt.Fprintln(t.w, strings.Join(taskToColumns(task), "\t"))
}

func (t *taskFormatterTSV) Close() {
}

type taskFormatterJSON struct {
	w io.Writer
}

func newTaskFormatterJSON(w io.Writer) *taskFormatterJSON {
	return &taskFormatterJSON{w: w}
}

func (t *taskFormatterJSON) AddTask(task *ecs.Task) {
	b, _ := marshalJSON(task)
	t.w.Write(b.Bytes())
}

func (t *taskFormatterJSON) Close() {
}
