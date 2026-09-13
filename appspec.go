package ecspresso

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/kayac/ecspresso/v2/appspec"
)

type AppSpecOption struct {
	TaskDefinition string `help:"use task definition arn in AppSpec (latest, current or Arn)" default:"latest"`
	UpdateService  bool   `help:"update service definition with task definition arn" default:"true" negatable:""`
}

func (d *App) AppSpec(ctx context.Context, opt AppSpecOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()
	var taskDefinitionArn string

	sv, err := d.DescribeService(ctx)
	if err != nil {
		return err
	}
	switch opt.TaskDefinition {
	case "current":
		taskDefinitionArn = aws.ToString(sv.TaskDefinition)
	case "latest":
		family, _, _ := strings.Cut(arnToName(aws.ToString(sv.TaskDefinition)), ":")
		taskDefinitionArn, err = d.findLatestTaskDefinitionArn(ctx, family)
		if err != nil {
			return err
		}
	default:
		if !strings.HasPrefix(opt.TaskDefinition, "arn:aws:ecs:") {
			return fmt.Errorf("--task-definition requires current, latest or a valid task definition arn")
		}
	}
	if opt.UpdateService {
		newSv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
		if err != nil {
			return err
		}
		sv = newSv
	}

	spec, err := appspec.NewWithService(&sv.Service, taskDefinitionArn)
	if err != nil {
		return fmt.Errorf("failed to create appspec: %w", err)
	}
	if d.config.AppSpec != nil {
		spec.Hooks = d.config.AppSpec.Hooks
	}

	_, err = WriteOutput(spec)
	return err
}
