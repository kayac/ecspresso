package ecspresso

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
)

type TasksOption struct {
	ID     *string
	Output *string
}

func (opt TasksOption) Formatter() taskFormatter {
	switch *opt.Output {
	case "json":
		return newTaskFormatterJSON(os.Stdout)
	case "tsv":
		return newTaskFormatterTSV(os.Stdout)
	}
	return newTaskFormatterTable(os.Stdout)
}

func (d *App) tasks(ctx context.Context, id *string) ([]*ecs.Task, error) {
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
		for _, desiredStatus := range []string{"RUNNING", "STOPPED"} {
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

	tasks, err := d.tasks(ctx, opt.ID)
	if err != nil {
		return err
	}

	formatter := opt.Formatter()
	for _, task := range tasks {
		formatter.AddTask(task)
	}
	formatter.Close()
	return nil
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

func newTaskFormatterTSV(w io.Writer) *taskFormatterTSV {
	t := &taskFormatterTSV{w: w}
	fmt.Fprintln(t.w, strings.Join(taskFormatterColumns, "\t"))
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
