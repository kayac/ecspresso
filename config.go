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
	DeploymentConfiguration       *ecs.DeploymentConfiguration `json:"deployment_configuration"`
	DesiredCount                  *int64                       `json:"desired_count"`
	HealthCheckGracePeriodSeconds *int64                       `json:"health_check_grace_period_seconds"`
	LaunchType                    *string                      `json:"launch_type"`
	LoadBalancers                 []*ecs.LoadBalancer          `json:"load_balancers"`
	NetworkConfiguration          *ecs.NetworkConfiguration    `json:"network_configuration"`
	PlacementConstraints          []*ecs.PlacementConstraint   `json:"placement_constraints"`
	PlacementStrategy             []*ecs.PlacementStrategy     `json:"placement_strategy"`
	PlatformVersion               *string                      `json:"platform_version"`
	Role                          *string                      `json:"role"`
	SchedulingStrategy            *string                      `json:"scheduling_strategy"`
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
