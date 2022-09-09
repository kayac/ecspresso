package ecspresso

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/pkg/errors"
)

type DeregisterOption struct {
	DryRun   *bool
	Keeps    *int
	Revision *int64
	Force    *bool
}

func (opt DeregisterOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

func (d *App) Deregister(opt DeregisterOption) error {
	ctx, cancel := d.Start()
	defer cancel()
	d.Log("Starting deregister task definition", opt.DryRunString())

	inUse, err := d.inUseRevisions(ctx)
	if err != nil {
		return err
	}

	if aws.ToInt64(opt.Revision) > 0 {
		return d.deregiserRevision(ctx, opt, inUse)
	} else if aws.ToInt(opt.Keeps) > 0 {
		return d.deregisterKeeps(ctx, opt, inUse)
	}
	return errors.New("--revision or --keeps required")
}

func (d *App) deregiserRevision(ctx context.Context, opt DeregisterOption, inUse map[string]string) error {
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "failed to load task definition")
	}
	name := fmt.Sprintf("%s:%d", aws.ToString(td.Family), aws.ToInt64(opt.Revision))

	if s := inUse[name]; s != "" {
		return errors.Errorf("%s is in use by %s", name, s)
	}

	if aws.ToBool(opt.DryRun) {
		d.Log(fmt.Sprintf("task definition %s will be deregistered", name))
		d.Log("DRY RUN OK")
		return nil
	}
	if aws.ToBool(opt.Force) || prompter.YesNo(fmt.Sprintf("Deregister %s ?", name), false) {
		d.Log(fmt.Sprintf("Deregistring %s", name))
		if _, err := d.ecs.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: aws.String(name),
		}); err != nil {
			return errors.Wrap(err, "failed to deregister task definition")
		}
		d.Log(fmt.Sprintf("%s was deregistered successfully", name))
	} else {
		d.Log("Aborted")
		return errors.New("confirmation failed")
	}
	return nil
}

func (d *App) deregisterKeeps(ctx context.Context, opt DeregisterOption, inUse map[string]string) error {
	keeps := aws.ToInt(opt.Keeps)
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "failed to load task definition")
	}
	names := []string{}
	var nextToken *string
	for {
		res, err := d.ecs.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			FamilyPrefix: td.Family,
			NextToken:    nextToken,
		})
		if err != nil {
			return errors.Wrap(err, "failed to list task definitions")
		}
		for _, a := range res.TaskDefinitionArns {
			name, err := taskDefinitionToName(a)
			if err != nil {
				continue
			}
			if s := inUse[name]; s != "" {
				d.Log(fmt.Sprintf("%s is in use by %s. skip", name, s))
			} else {
				d.DebugLog(fmt.Sprintf("%s is marked to deregister", name))
				names = append(names, name)
			}
		}
		if nextToken = res.NextToken; nextToken == nil {
			break
		}
	}

	deregs := []string{}
	idx := len(names) - keeps
	if idx <= 0 {
		d.Log("No need to deregister task definitions")
		return nil
	}
	for i, name := range names {
		if i < idx {
			d.Log(fmt.Sprintf("%s will be deregistered", name))
			deregs = append(deregs, name)
		}
	}
	if aws.ToBool(opt.DryRun) {
		d.Log("DRY RUN OK")
		return nil
	}

	deregistered := 0
	if aws.ToBool(opt.Force) || prompter.YesNo(fmt.Sprintf("Deregister %d revisons?", len(deregs)), false) {
		for _, name := range deregs {
			d.Log(fmt.Sprintf("Deregistring %s", name))
			if _, err := d.ecs.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
				TaskDefinition: aws.String(name),
			}); err != nil {
				return errors.Wrap(err, "failed to deregister task definition")
			}
			d.Log(fmt.Sprintf("%s was deregistered successfully", name))
			time.Sleep(time.Second)
			deregistered++
		}
	} else {
		d.Log("Aborted")
		return errors.New("confirmation failed")
	}
	d.Log(fmt.Sprintf("%d task definitions were deregistered", deregistered))

	return nil
}

func (d *App) inUseRevisions(ctx context.Context) (map[string]string, error) {
	inUse := make(map[string]string)
	tasks, err := d.listTasks(ctx)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		name, _ := taskDefinitionToName(*task.TaskDefinitionArn)
		st := aws.ToString(task.LastStatus)
		if st == "" {
			st = aws.ToString(task.DesiredStatus)
		}
		if st != "STOPPED" {
			// ignore STOPPED tasks for in use
			inUse[name] = st + " task"
		}
		d.DebugLog(fmt.Sprintf("%s is in use by tasks", name))
	}

	if d.config.Service != "" {
		sv, err := d.DescribeService(ctx)
		if err != nil {
			return nil, err
		}
		for _, dp := range sv.Deployments {
			name, _ := taskDefinitionToName(*dp.TaskDefinition)
			inUse[name] = fmt.Sprintf("%s deployment", *dp.Status)
			d.DebugLog(fmt.Sprintf("%s is in use by deployments", name))
		}
	}
	return inUse, nil
}

func taskDefinitionToName(a string) (string, error) {
	an, err := arn.Parse(a)
	if err != nil {
		return "", err
	}
	n := strings.SplitN(an.Resource, "/", 2)
	return n[1], nil
}
