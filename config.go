package ecspresso

import (
	"errors"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/service/ecs"
)

const DefaultClusterName = "default"

type Config struct {
	Region                string        `yaml:"region"`
	Service               string        `yaml:"service"`
	Cluster               string        `yaml:"cluster"`
	TaskDefinitionPath    string        `yaml:"task_definition"`
	ServiceDefinitionPath string        `yaml:"service_definition"`
	Timeout               time.Duration `yaml:"timeout"`
}

type ServiceDefinition struct {
	DeploymentConfiguration       *ecs.DeploymentConfiguration `json:"deploymentConfiguration"`
	DesiredCount                  *int64                       `json:"desiredCount"`
	HealthCheckGracePeriodSeconds *int64                       `json:"healthCheckGracePeriod_seconds"`
	LaunchType                    *string                      `json:"launchType"`
	LoadBalancers                 []*ecs.LoadBalancer          `json:"loadBalancers"`
	NetworkConfiguration          *ecs.NetworkConfiguration    `json:"networkConfiguration"`
	PlacementConstraints          []*ecs.PlacementConstraint   `json:"placementConstraints"`
	PlacementStrategy             []*ecs.PlacementStrategy     `json:"placementStrategy"`
	PlatformVersion               *string                      `json:"platformVersion"`
	Role                          *string                      `json:"role"`
	SchedulingStrategy            *string                      `json:"schedulingStrategy"`
	ServiceRegistries             []*ecs.ServiceRegistry       `json:"serviceRegistries"`
}

func (c *Config) Validate() error {
	if c.Cluster == "" {
		c.Cluster = DefaultClusterName
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
