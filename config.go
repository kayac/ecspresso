package ecspresso

import (
	"errors"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/service/ecs"
)

type Config struct {
	Region                string        `yaml:"region"`
	Service               string        `yaml:"service"`
	Cluster               string        `yaml:"cluster"`
	TaskDefinitionPath    string        `yaml:"task_definition"`
	ServiceDefinitionPath string        `yaml:"service_definition"`
	RuleDefinitionPath    string        `yaml:"rule_definition"`
	TargetDefinitionPath  string        `yaml:"target_definition"`
	Timeout               time.Duration `yaml:"timeout"`
}

type ServiceDefinition struct {
	DeploymentConfiguration *ecs.DeploymentConfiguration `json:"deploymentConfiguration"`
	LaunchType              *string                      `json:"launchType"`
	LoadBalancers           []*ecs.LoadBalancer          `json:"loadBalancers"`
	NetworkConfiguration    *ecs.NetworkConfiguration    `json:"networkConfiguration"`
	PlacementConstraints    []*ecs.PlacementConstraint   `json:"placementConstraints"`
	PlacementStrategy       []*ecs.PlacementStrategy     `json:"placementStrategy"`
	Role                    *string                      `json:"role"`
	DesiredCount            *int64                       `json:"desiredCount"`
}

func (c *Config) Validate() error {
	if c.Service == "" {
		return errors.New("service is not defined")
	}
	if c.Cluster == "" {
		return errors.New("cluster is not defined")
	}
	if c.TaskDefinitionPath == "" {
		return errors.New("task_definition is not defined")
	}
	return nil
}

func NewDefaultConfig() *Config {
	return &Config{
		Region:  os.Getenv("AWS_REGION"),
		Timeout: 300 * time.Second,
	}
}
