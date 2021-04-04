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
	"strings"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
)

type ExecOption struct {
	ID        *string
	Command   *string
	Container *string
}

func (d *App) Exec(opt ExecOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	// find a task to exec
	tasks, err := d.tasks(ctx, opt.ID, "RUNNING")
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	formatter := newTaskFormatterTSV(buf, false)
	for _, task := range tasks {
		formatter.AddTask(task)
	}
	formatter.Close()
	result, err := d.runFinder(buf, "task ID")
	if err != nil {
		return err
	}
	taskID := strings.Fields(string(result))[0]
	var targetTask *ecs.Task
	for _, task := range tasks {
		task := task
		if arnToName(*task.TaskArn) == taskID {
			targetTask = task
			break
		}
	}

	// find a container to exec
	var targetContainer *string
	if len(targetTask.Containers) == 1 {
		targetContainer = targetTask.Containers[0].Name
	} else if aws.StringValue(opt.Container) != "" {
		targetContainer = opt.Container
	} else {
		// select a container to execute
		buf := new(bytes.Buffer)
		for _, container := range targetTask.Containers {
			fmt.Fprintln(buf, string(*container.Name))
		}
		result, err := d.runFinder(buf, "container name")
		if err != nil {
			return errors.Wrap(err, "failed to exucute finder")
		}
		targetContainer = aws.String(strings.Fields(string(result))[0])
	}

	out, err := d.ecs.ExecuteCommand(&ecs.ExecuteCommandInput{
		Cluster:     aws.String(d.Cluster),
		Interactive: aws.Bool(true),
		Task:        aws.String(taskID),
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

func (d *App) runFinder(src io.Reader, title string) (string, error) {
	command := d.config.FinderCommand
	if command == "" {
		return runInternalFinder(src, title)
	}
	finder := exec.Command(command)
	finder.Stderr = os.Stderr
	p, _ := finder.StdinPipe()
	go func() {
		io.Copy(p, src)
		p.Close()
	}()
	b, err := finder.Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to execute finder command")
	}
	return string(b), nil
}

func runInternalFinder(src io.Reader, title string) (string, error) {
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
