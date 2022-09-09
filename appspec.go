package ecspresso

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/kayac/ecspresso/appspec"
	"github.com/pkg/errors"
)

func (d *App) AppSpec(opt AppSpecOption) error {
	ctx, cancel := d.Start()
	defer cancel()
	var taskDefinitionArn string

	sv, err := d.DescribeService(ctx)
	if err != nil {
		return err
	}
	switch *opt.TaskDefinition {
	case "current":
		taskDefinitionArn = *sv.TaskDefinition
	case "latest":
		family := strings.Split(arnToName(*sv.TaskDefinition), ":")[0]
		taskDefinitionArn, err = d.findLatestTaskDefinitionArn(ctx, family)
		if err != nil {
			return err
		}
	default:
		taskDefinitionArn = *opt.TaskDefinition
		if !strings.HasPrefix(taskDefinitionArn, "arn:aws:ecs:") {
			return errors.New("--task-definition requires current, latest or a valid task definition arn")
		}
	}
	if aws.BoolValue(opt.UpdateService) {
		newSv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
		if err != nil {
			return err
		}
		sv = newSv
	}

	spec, err := appspec.NewWithService(&sv.Service, taskDefinitionArn)
	if err != nil {
		return errors.Wrap(err, "failed to create appspec")
	}
	if d.config.AppSpec != nil {
		spec.Hooks = d.config.AppSpec.Hooks
	}

	fmt.Print(spec.String())
	return nil
}
