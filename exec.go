package ecspresso

import (
	"encoding/json"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
)

type ExecOption struct {
	ID      *string
	Command *string
}

func (d *App) Exec(opt ExecOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	tasks, err := d.tasks(ctx, opt.ID)
	if err != nil {
		return err
	}

	finder := exec.Command("sh", "-c", "peco")
	finder.Stderr = os.Stderr
	p, _ := finder.StdinPipe()

	go func() {
		formatter := newTaskFormatterTSV(p)
		for _, task := range tasks {
			formatter.AddTask(task)
		}
		formatter.Close()
		p.Close()
	}()

	var taskID string
	if result, err := finder.Output(); err != nil {
		return errors.Wrap(err, "failed to exucute finder command")
	} else {
		taskID = strings.Fields(string(result))[0]
	}

	out, err := d.ecs.ExecuteCommand(&ecs.ExecuteCommandInput{
		Cluster:     &d.Cluster,
		Interactive: aws.Bool(true),
		Task:        aws.String(taskID),
		Command:     opt.Command,
	})
	if err != nil {
		return err
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
