package ecspresso

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
)

type taskFinderOption interface {
	taskID() string
}

type ExecOption struct {
	ID        *string
	Command   *string
	Container *string
}

func (o ExecOption) taskID() string {
	return aws.StringValue(o.ID)
}

func (d *App) Exec(opt ExecOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	// find a task to exec
	tasks, err := d.listTasks(ctx, opt.ID, "RUNNING")
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		d.Log("tasks not found")
		return nil
	}

	task, err := d.findTask(opt, tasks)
	if err != nil {
		return err
	}

	// find a container to exec
	var targetContainer *string
	if len(task.Containers) == 1 {
		targetContainer = task.Containers[0].Name
	} else if aws.StringValue(opt.Container) != "" {
		targetContainer = opt.Container
	} else {
		// select a container to execute
		buf := new(bytes.Buffer)
		sort.SliceStable(task.Containers, func(i, j int) bool {
			return aws.StringValue(task.Containers[i].Name) < aws.StringValue(task.Containers[j].Name)
		})
		for _, container := range task.Containers {
			fmt.Fprintln(buf, string(*container.Name))
		}
		result, err := d.runFilter(buf, "container name")
		if err != nil {
			return errors.Wrap(err, "failed to exucute filter")
		}
		targetContainer = aws.String(strings.Fields(string(result))[0])
	}

	out, err := d.ecs.ExecuteCommand(&ecs.ExecuteCommandInput{
		Cluster:     task.ClusterArn,
		Interactive: aws.Bool(true),
		Task:        task.TaskArn,
		Command:     opt.Command,
		Container:   targetContainer,
	})
	if err != nil {
		return errors.Wrap(err, "failed to execute command")
	}
	sess, _ := json.Marshal(out.Session)
	cmd := exec.Command("session-manager-plugin", string(sess), d.config.Region, "StartSession")
	signal.Ignore(os.Interrupt)
	defer signal.Reset(os.Interrupt)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (d *App) runFilter(src io.Reader, title string) (string, error) {
	command := d.config.FilterCommand
	if command == "" {
		return runInternalFilter(src, title)
	}
	var f *exec.Cmd
	if strings.Contains(command, " ") {
		f = exec.Command("sh", "-c", command)
	} else {
		f = exec.Command(command)
	}
	f.Stderr = os.Stderr
	p, _ := f.StdinPipe()
	go func() {
		io.Copy(p, src)
		p.Close()
	}()
	b, err := f.Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to execute filter command")
	}
	return string(b), nil
}

func runInternalFilter(src io.Reader, title string) (string, error) {
	var items []string
	s := bufio.NewScanner(src)
	for s.Scan() {
		fmt.Println(s.Text())
		items = append(items, strings.Fields(s.Text())[0])
	}

	var input string
	for {
		input = prompter.Prompt("Enter "+title, "")
		if input == "" {
			continue
		}
		var found []string
		for _, item := range items {
			item := item
			if strings.HasPrefix(item, input) {
				found = append(found, item)
			}
		}

		switch len(found) {
		case 0:
			fmt.Printf("no such item %s\n", input)
		case 1:
			fmt.Printf("%s=%s\n", title, found[0])
			return found[0], nil
		default:
			fmt.Printf("%s is ambiguous\n", input)
		}
	}
}
