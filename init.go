package ecspresso

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

var CreateFileMode = os.FileMode(0644)

func (d *App) Init(opt InitOption) error {
	config := d.config
	ctx := context.Background()

	out, err := d.ecs.DescribeServicesWithContext(ctx, d.DescribeServicesInput())
	if err != nil {
		return errors.Wrap(err, "failed to describe service")
	}
	if len(out.Services) == 0 {
		return errors.New("service is not found")
	}

	sv := out.Services[0]
	td, err := d.DescribeTaskDefinition(ctx, *sv.TaskDefinition)
	if err != nil {
		return errors.Wrap(err, "failed to describe task definition")
	}
	lt, err := d.ecs.ListTagsForResourceWithContext(ctx, &ecs.ListTagsForResourceInput{
		ResourceArn: sv.ServiceArn,
	})
	if err != nil {
		return errors.Wrap(err, "failed to list tags for service")
	}
	sv.Tags = lt.Tags

	// service-def
	treatmentServiceDefinition(sv)
	if b, err := MarshalJSON(sv); err != nil {
		return errors.Wrap(err, "unable to marshal service definition to JSON")
	} else {
		d.Log("save service definition to", config.ServiceDefinitionPath)
		if err := d.saveFile(config.ServiceDefinitionPath, b, CreateFileMode, *opt.ForceOverwrite); err != nil {
			return errors.Wrap(err, "failed to write file")
		}
	}

	// task-def
	if b, err := MarshalJSON(td); err != nil {
		return errors.Wrap(err, "unable to marshal task definition to JSON")
	} else {
		d.Log("save task definition to", config.TaskDefinitionPath)
		if err := d.saveFile(config.TaskDefinitionPath, b, CreateFileMode, *opt.ForceOverwrite); err != nil {
			return errors.Wrap(err, "failed to write file")
		}
	}

	// config
	if b, err := yaml.Marshal(config); err != nil {
		return errors.Wrap(err, "unable to marshal config to YAML")
	} else {
		d.Log("save config to", *opt.ConfigFilePath)
		if err := d.saveFile(*opt.ConfigFilePath, b, CreateFileMode, *opt.ForceOverwrite); err != nil {
			return errors.Wrap(err, "failed to write file")
		}
	}

	return nil
}

func treatmentServiceDefinition(sv *ecs.Service) *ecs.Service {
	sv.ClusterArn = nil
	sv.CreatedAt = nil
	sv.CreatedBy = nil
	sv.Deployments = nil
	sv.Events = nil
	sv.PendingCount = nil
	sv.PropagateTags = nil
	sv.RunningCount = nil
	sv.Status = nil
	sv.TaskDefinition = nil
	sv.TaskSets = nil
	sv.ServiceArn = nil
	sv.RoleArn = nil
	sv.ServiceName = nil
	return sv
}

func (d *App) saveFile(path string, b []byte, mode os.FileMode, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		ok := prompter.YN(fmt.Sprintf("Overwrite existing file %s?", path), false)
		if !ok {
			d.Log("skip", path)
			return nil
		}
	}
	return ioutil.WriteFile(path, b, mode)
}
