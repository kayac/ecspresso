package ecspresso

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/goccy/go-yaml"
	"github.com/kayac/ecspresso/v2/duration"
)

var CreateFileMode = os.FileMode(0644)

type InitOption struct {
	Region                string `help:"AWS region" env:"AWS_REGION" default:""`
	Cluster               string `help:"ECS cluster name" default:"default"`
	Service               string `help:"ECS service name" required:"" xor:"FROM"`
	TaskDefinition        string `help:"ECS task definition name:revision" required:"" xor:"FROM"`
	TaskDefinitionPath    string `help:"path to output task definition file" default:"ecs-task-def.json"`
	ServiceDefinitionPath string `help:"path to output service definition file" default:"ecs-service-def.json"`
	ExpressDefinitionPath string `help:"path to output express service definition file" default:"ecs-express-def.json"`
	Sort                  bool   `help:"sort elements in task definition" default:"false" negatable:""`
	ForceOverwrite        bool   `help:"overwrite existing files" default:"false"`
	Jsonnet               bool   `help:"output files as jsonnet format" default:"false"`
	Express               *bool  `help:"import express service definition" negatable:""`
}

func (opt *InitOption) NewConfig(ctx context.Context, configFilePath string) (*Config, error) {
	conf := NewDefaultConfig()
	conf.path = configFilePath
	conf.Region = opt.Region
	conf.Cluster = opt.Cluster
	conf.Service = opt.Service
	conf.TaskDefinitionPath = opt.TaskDefinitionPath
	conf.ServiceDefinitionPath = opt.ServiceDefinitionPath
	conf.ExpressDefinitionPath = opt.ExpressDefinitionPath
	if err := conf.Restrict(ctx); err != nil {
		return nil, err
	}
	return conf, nil
}

const (
	jsonnetExt = ".jsonnet"
	jsonExt    = ".json"
	ymlExt     = ".yml"
	yamlExt    = ".yaml"
)

func (d *App) Init(ctx context.Context, opt InitOption) error {
	conf := d.config
	// when --task-definition is not empty, --service is empty because these flags are exclusive.
	tdOnly := opt.TaskDefinition != ""

	d.LogJSON(opt)
	d.LogJSON(conf)
	if opt.Jsonnet {
		if ext := filepath.Ext(conf.ServiceDefinitionPath); ext == jsonExt {
			conf.ServiceDefinitionPath = strings.TrimSuffix(conf.ServiceDefinitionPath, ext) + jsonnetExt
		}
		if ext := filepath.Ext(conf.TaskDefinitionPath); ext == jsonExt {
			conf.TaskDefinitionPath = strings.TrimSuffix(conf.TaskDefinitionPath, ext) + jsonnetExt
		}
		if ext := filepath.Ext(conf.ExpressDefinitionPath); ext == jsonExt {
			conf.ExpressDefinitionPath = strings.TrimSuffix(conf.ExpressDefinitionPath, ext) + jsonnetExt
		}
		if ext := filepath.Ext(conf.path); ext == ymlExt || ext == yamlExt {
			conf.path = strings.TrimSuffix(conf.path, ext) + jsonnetExt
		}
	}

	if tdOnly {
		tdArn := opt.TaskDefinition
		td, err := d.initTaskDefinition(ctx, opt, tdArn)
		if err != nil {
			return err
		}
		return d.initConfigurationFile(ctx, conf.path, opt, nil, td, nil)
	}

	// initialize with service
	sv, err := d.DescribeService(ctx)
	if err != nil {
		return err
	}

	if sv.isExpressMode() {
		d.LogInfo("Express Gateway Service detected", "service", aws.ToString(sv.ServiceName))
		if opt.Express == nil || *opt.Express {
			ex, sv, err := d.initExpressGatewayService(ctx, sv, opt)
			if err != nil {
				return err
			}
			return d.initConfigurationFile(ctx, conf.path, opt, sv, nil, ex)
		}
	} else if opt.Express != nil && *opt.Express {
		return fmt.Errorf("service %s is not an Express Gateway Service", aws.ToString(sv.ServiceName))
	}

	sv, tdArn, err := d.initServiceDefinition(ctx, sv, opt)
	if err != nil {
		return err
	}

	td, err := d.initTaskDefinition(ctx, opt, tdArn)
	if err != nil {
		return err
	}
	return d.initConfigurationFile(ctx, conf.path, opt, sv, td, nil)
}

func (d *App) initConfigurationFile(ctx context.Context, configFilePath string, opt InitOption, sv *Service, td *TaskDefinitionInput, ex *ExpressGatewayService) error {
	conf := d.config
	d.LogInfo("initializing configuration file", "path", configFilePath)
	if ex != nil {
		// express
		conf.Service = aws.ToString(sv.ServiceName)
		conf.TaskDefinitionPath = ""
		conf.ServiceDefinitionPath = ""
	} else if td != nil && sv == nil {
		// tdOnly
		conf.Service = ""
		conf.ServiceDefinitionPath = ""
		conf.ExpressDefinitionPath = ""
	} else {
		// normal
		conf.ExpressDefinitionPath = ""
		if sv.isCodeDeploy() {
			info, err := d.findDeploymentInfo(ctx)
			if err != nil {
				LogWarn("failed to find CodeDeploy deployment info", "error", err.Error())
				LogWarn("you need to set config.codedeploy section manually")
			} else {
				conf.CodeDeploy = &ConfigCodeDeploy{
					ApplicationName:     *info.ApplicationName,
					DeploymentGroupName: *info.DeploymentGroupName,
				}
			}
		}
	}

	// Express Mode uses CANARY deployment, so requires increasing timeout
	if sv != nil && sv.isExpressMode() {
		conf.Timeout = &duration.Duration{Duration: DefaultTimeout * 2}
	}

	{
		var b []byte
		var err error
		if opt.Jsonnet {
			b, err = json.MarshalIndent(conf, "", "  ")
			if err != nil {
				return fmt.Errorf("unable to marshal config to JSON: %w", err)
			}
			out, err := toJsonnetString(string(b), configFilePath)
			if err != nil {
				return fmt.Errorf("unable to format config as Jsonnet: %w", err)
			}
			b = []byte(out)
		} else {
			b, err = yaml.Marshal(conf)
			if err != nil {
				return fmt.Errorf("unable to marshal config to YAML: %w", err)
			}
		}
		d.LogInfo("saving config", "path", configFilePath)
		if err := d.saveFile(configFilePath, b, CreateFileMode, opt.ForceOverwrite); err != nil {
			return err
		}
	}
	return nil
}

func (d *App) initServiceDefinition(ctx context.Context, sv *Service, opt InitOption) (*Service, string, error) {
	conf := d.config
	d.LogInfo("initializing service definition", "service", aws.ToString(sv.ServiceName))

	svArn := aws.ToString(sv.ServiceArn)
	if long, _ := isLongArnFormat(svArn); long {
		// Long arn format must be used for tagging operations
		lt, err := d.ecs.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: sv.ServiceArn,
		})
		if err != nil {
			return nil, "", fmt.Errorf("failed to list tags for service: %w", err)
		}
		sv.Tags = lt.Tags
	}
	tdArn, err := sv.getTaskDefinitionArn()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get task definition arn: %w", err)
	}
	treatmentServiceDefinition(sv)
	// remove unnecessary fields
	if b, err := MarshalJSONForAPI(sv, "del(.runningCount, .pendingCount)"); err != nil {
		return nil, "", fmt.Errorf("unable to marshal service definition to JSON: %w", err)
	} else {
		if opt.Jsonnet {
			out, err := toJsonnetString(string(b), conf.ServiceDefinitionPath)
			if err != nil {
				return nil, "", fmt.Errorf("unable to format service definition as Jsonnet: %w", err)
			}
			b = []byte(out)
		}
		d.LogInfo("saving service definition", "service_arn", svArn, "path", conf.ServiceDefinitionPath)
		if err := d.saveFile(conf.ServiceDefinitionPath, b, CreateFileMode, opt.ForceOverwrite); err != nil {
			return nil, "", err
		}
	}
	return sv, tdArn, nil
}

func (d *App) initTaskDefinition(ctx context.Context, opt InitOption, tdArn string) (*TaskDefinitionInput, error) {
	conf := d.config
	d.LogInfo("initializing task definition", "task_definition", tdArn)

	td, err := d.DescribeTaskDefinition(ctx, tdArn)
	if err != nil {
		return nil, err
	}
	if opt.Sort {
		sortTaskDefinition(td)
	}
	if b, err := MarshalJSONForAPI(td); err != nil {
		return nil, fmt.Errorf("unable to marshal task definition to JSON: %w", err)
	} else {
		if opt.Jsonnet {
			out, err := toJsonnetString(string(b), conf.TaskDefinitionPath)
			if err != nil {
				return nil, fmt.Errorf("unable to format task definition as Jsonnet: %w", err)
			}
			b = []byte(out)
		}
		d.LogInfo("saving task definition", "task_definition", tdArn, "path", conf.TaskDefinitionPath)
		if err := d.saveFile(conf.TaskDefinitionPath, b, CreateFileMode, opt.ForceOverwrite); err != nil {
			return nil, err
		}
	}
	return td, nil
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
	sv.CurrentServiceRevisions = nil
	sv.CurrentServiceDeployment = nil

	if sv.PropagateTags != types.PropagateTagsService && sv.PropagateTags != types.PropagateTagsTaskDefinition {
		sv.PropagateTags = types.PropagateTagsNone
	}

	// Express Mode cannot use DeploymentConfiguration and LoadBalancers
	if sv.isExpressMode() {
		sv.DeploymentConfiguration = nil
		sv.LoadBalancers = nil
	}
}

func (d *App) saveFile(path string, b []byte, mode os.FileMode, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		ok := prompter.YN(fmt.Sprintf("Overwrite existing file %s?", path), false)
		if !ok {
			d.LogInfo("skipping file", "path", path)
			return nil
		}
	}
	if err := os.WriteFile(path, b, mode); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	return nil
}
