package ecspresso

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/google/go-jsonnet/formatter"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

var CreateFileMode = os.FileMode(0644)

type InitOption struct {
	Region                *string
	Cluster               *string
	Service               *string
	TaskDefinitionPath    *string
	ServiceDefinitionPath *string
	ConfigFilePath        *string
	ForceOverwrite        *bool
	Jsonnet               *bool
}

var (
	jsonnetExt = ".jsonnet"
	jsonExt    = ".json"
)

func (d *App) Init(opt InitOption) error {
	config := d.config
	ctx := context.Background()

	if *opt.Jsonnet {
		if ext := filepath.Ext(config.ServiceDefinitionPath); ext == jsonExt {
			config.ServiceDefinitionPath = strings.TrimSuffix(config.ServiceDefinitionPath, ext) + jsonnetExt
		}
		if ext := filepath.Ext(config.TaskDefinitionPath); ext == jsonExt {
			config.TaskDefinitionPath = strings.TrimSuffix(config.TaskDefinitionPath, ext) + jsonnetExt
		}
	}

	out, err := d.ecs.DescribeServices(ctx, d.DescribeServicesInput())
	if err != nil {
		return errors.Wrap(err, "failed to describe service")
	}
	if len(out.Services) == 0 {
		return errors.New("service is not found")
	}

	sv := newServiceFromTypes(out.Services[0])
	td, err := d.DescribeTaskDefinition(ctx, *sv.TaskDefinition)
	if err != nil {
		return errors.Wrap(err, "failed to describe task definition")
	}

	if long, _ := isLongArnFormat(*sv.ServiceArn); long {
		// Long arn format must be used for tagging operations
		lt, err := d.ecs.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: sv.ServiceArn,
		})
		if err != nil {
			return errors.Wrap(err, "failed to list tags for service")
		}
		sv.Tags = lt.Tags
	}

	// service-def
	treatmentServiceDefinition(sv)
	if b, err := MarshalJSON(sv); err != nil {
		return errors.Wrap(err, "unable to marshal service definition to JSON")
	} else {
		if *opt.Jsonnet {
			out, err := formatter.Format(config.ServiceDefinitionPath, string(b), formatter.DefaultOptions())
			if err != nil {
				return errors.Wrap(err, "unable to format service definition as Jsonnet")
			}
			b = []byte(out)
		}
		d.Log("save service definition to", config.ServiceDefinitionPath)
		if err := d.saveFile(config.ServiceDefinitionPath, b, CreateFileMode, *opt.ForceOverwrite); err != nil {
			return errors.Wrap(err, "failed to write file")
		}
	}

	// task-def
	if b, err := MarshalJSON(td); err != nil {
		return errors.Wrap(err, "unable to marshal task definition to JSON")
	} else {
		if *opt.Jsonnet {
			out, err := formatter.Format(config.TaskDefinitionPath, string(b), formatter.DefaultOptions())
			if err != nil {
				return errors.Wrap(err, "unable to format task definition as Jsonnet")
			}
			b = []byte(out)
		}
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

func treatmentServiceDefinition(sv *Service) *Service {
	sv.ClusterArn = nil
	sv.CreatedAt = nil
	sv.CreatedBy = nil
	sv.Deployments = nil
	sv.Events = nil
	sv.PendingCount = 0
	sv.RunningCount = 0
	sv.Status = nil
	sv.TaskDefinition = nil
	sv.TaskSets = nil
	sv.ServiceArn = nil
	sv.RoleArn = nil
	sv.ServiceName = nil

	if sv.PropagateTags != types.PropagateTagsService && sv.PropagateTags != types.PropagateTagsTaskDefinition {
		sv.PropagateTags = types.PropagateTagsNone
	}

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
