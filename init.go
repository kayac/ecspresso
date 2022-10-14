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

func (d *App) Init(ctx context.Context, opt InitOption) error {
	config := d.config

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
		return fmt.Errorf("failed to describe service: %w", err)
	}
	if len(out.Services) == 0 {
		return ErrNotFound("service is not found")
	}

	sv := newServiceFromTypes(out.Services[0])
	td, err := d.DescribeTaskDefinition(ctx, *sv.TaskDefinition)
	if err != nil {
		return err
	}

	if long, _ := isLongArnFormat(*sv.ServiceArn); long {
		// Long arn format must be used for tagging operations
		lt, err := d.ecs.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: sv.ServiceArn,
		})
		if err != nil {
			return fmt.Errorf("failed to list tags for service: %w", err)
		}
		sv.Tags = lt.Tags
	}

	// service-def
	treatmentServiceDefinition(sv)
	if b, err := MarshalJSONForAPI(sv); err != nil {
		return fmt.Errorf("unable to marshal service definition to JSON: %w", err)
	} else {
		if *opt.Jsonnet {
			out, err := formatter.Format(config.ServiceDefinitionPath, string(b), formatter.DefaultOptions())
			if err != nil {
				return fmt.Errorf("unable to format service definition as Jsonnet: %w", err)
			}
			b = []byte(out)
		}
		d.Log("save service definition to %s", config.ServiceDefinitionPath)
		if err := d.saveFile(config.ServiceDefinitionPath, b, CreateFileMode, *opt.ForceOverwrite); err != nil {
			return err
		}
	}

	// task-def
	if b, err := MarshalJSONForAPI(td); err != nil {
		return fmt.Errorf("unable to marshal task definition to JSON: %w", err)
	} else {
		if *opt.Jsonnet {
			out, err := formatter.Format(config.TaskDefinitionPath, string(b), formatter.DefaultOptions())
			if err != nil {
				return fmt.Errorf("unable to format task definition as Jsonnet: %w", err)
			}
			b = []byte(out)
		}
		d.Log("save task definition to %s", config.TaskDefinitionPath)
		if err := d.saveFile(config.TaskDefinitionPath, b, CreateFileMode, *opt.ForceOverwrite); err != nil {
			return err
		}
	}

	// config
	if b, err := yaml.Marshal(config); err != nil {
		return fmt.Errorf("unable to marshal config to YAML: %w", err)
	} else {
		d.Log("save config to %s", *opt.ConfigFilePath)
		if err := d.saveFile(*opt.ConfigFilePath, b, CreateFileMode, *opt.ForceOverwrite); err != nil {
			return err
		}
	}

	return nil
}

func treatmentServiceDefinition(sv *Service) {
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
}

func (d *App) saveFile(path string, b []byte, mode os.FileMode, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		ok := prompter.YN(fmt.Sprintf("Overwrite existing file %s?", path), false)
		if !ok {
			d.Log("skip %s", path)
			return nil
		}
	}
	if err := ioutil.WriteFile(path, b, mode); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	return nil
}
