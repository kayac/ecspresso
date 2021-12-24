package ecspresso

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/pkg/errors"
)

const SessionManagerPluginBinary = "session-manager-plugin"

type taskFinderOption interface {
	taskID() string
}

type ExecOption struct {
	ID        *string
	Command   *string
	Container *string

	PortForward *bool
	LocalPort   *int
	Port        *int
}

func (o ExecOption) taskID() string {
	return aws.StringValue(o.ID)
}

func (d *App) Exec(opt ExecOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	if _, err := exec.LookPath(SessionManagerPluginBinary); err != nil {
		return errors.Wrapf(err, "%s is not installed.\nSee also https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html\n", SessionManagerPluginBinary)
	}

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
			return errors.Wrap(err, "failed to execute filter")
		}
		targetContainer = aws.String(strings.Fields(string(result))[0])
	}

	if aws.BoolValue(opt.PortForward) {
		return d.portForward(ctx, task, targetContainer, *opt.LocalPort, *opt.Port)
	}

	out, err := d.ecs.ExecuteCommand(&ecs.ExecuteCommandInput{
		Cluster:     task.ClusterArn,
		Interactive: aws.Bool(true),
		Task:        task.TaskArn,
		Command:     opt.Command,
		Container:   targetContainer,
	})
	if err != nil {
		return errors.Wrap(err, "failed to execute command. See also https://github.com/aws-containers/amazon-ecs-exec-checker")
	}
	sess, _ := json.Marshal(out.Session)
	ssmReq, err := d.buildSsmRequestParameters(task, targetContainer)
	if err != nil {
		return errors.Wrap(err, "failed to build ssm request parameters")
	}

	cmd := exec.Command(
		SessionManagerPluginBinary,
		string(sess),
		d.config.Region,
		"StartSession",
		"",
		ssmReq.String(),
		d.ecs.Endpoint,
	)
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

/* buildSsmRequestParameters and getContainerRuntimeID are ported from aws-cli.
https://github.com/aws/aws-cli/blob/054e0f194cfe9cf3642994b583fe438c56a97dfc/awscli/customizations/ecs/executecommand.py#L45

# Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You
# may not use this file except in compliance with the License. A copy of
# the License is located at
#
#     http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is
# distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
# ANY KIND, either express or implied. See the License for the specific
# language governing permissions and limitations under the License.
*/

type ssmRequestParameters struct {
	Target string
}

func (p *ssmRequestParameters) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func (d *App) buildSsmRequestParameters(task *ecs.Task, targetContainer *string) (*ssmRequestParameters, error) {
	values := strings.Split(*task.TaskArn, "/")
	clusterName := values[1]
	taskID := values[2]
	runtimeID, err := d.getContainerRuntimeID(task, targetContainer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get container runtime ID")
	}
	return &ssmRequestParameters{
		Target: fmt.Sprintf("ecs:%s_%s_%s", clusterName, taskID, *runtimeID),
	}, nil
}

func (d *App) getContainerRuntimeID(task *ecs.Task, targetContainer *string) (*string, error) {
	output, err := d.ecs.DescribeTasks(&ecs.DescribeTasksInput{
		Cluster: task.ClusterArn,
		Tasks:   []*string{task.TaskArn},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to describe tasks")
	}
	for _, t := range output.Tasks {
		for _, c := range t.Containers {
			if aws.StringValue(c.Name) == aws.StringValue(targetContainer) {
				return c.RuntimeId, nil
			}
		}
	}
	return nil, errors.Errorf("container %s is not found in task %s", *targetContainer, *task.TaskArn)
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
			if item == input {
				found = []string{item}
				break
			} else if strings.HasPrefix(item, input) {
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

func (d *App) portForward(ctx context.Context, task *ecs.Task, targetContainer *string, localPort, remotePort int) error {
	if remotePort == 0 {
		return fmt.Errorf("--port is required")
	}

	ssmclient := ssm.New(d.sess)
	ssmReq, err := d.buildSsmRequestParameters(task, targetContainer)
	if err != nil {
		return err
	}
	res, err := ssmclient.StartSession(&ssm.StartSessionInput{
		Target:       aws.String(ssmReq.Target),
		DocumentName: aws.String("AWS-StartPortForwardingSession"),
		Parameters: map[string][]*string{
			"portNumber":      {aws.String(strconv.Itoa(remotePort))},
			"localPortNumber": {aws.String(strconv.Itoa(localPort))},
		},
		Reason: aws.String("port forwarding by ecspresso"),
	})
	if err != nil {
		return errors.Wrap(err, "failed to start SSM session")
	}
	ssmSess, _ := json.Marshal(res)
	d.DebugLog(SessionManagerPluginBinary, string(ssmSess), d.config.Region, "StartSession", "", ssmReq.String(), d.ecs.Endpoint)

	cmd := exec.Command(
		SessionManagerPluginBinary,
		string(ssmSess),
		d.config.Region,
		"StartSession",
		"",
		ssmReq.String(),
		d.ecs.Endpoint,
	)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
