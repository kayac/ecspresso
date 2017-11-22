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
	Timeout               time.Duration `yaml:"timeout"`
}

type ServiceDefinition struct {
	DeploymentConfiguration *ecs.DeploymentConfiguration `json:"deploymentConfiguration"`
	Role                    *string                      `json:"role"`
	LoadBalancers           []*ecs.LoadBalancer          `json:"loadBalancers"`
	PlacementConstraints    []*ecs.PlacementConstraint   `json:"placementConstraints"`
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
