package ecspresso

import (
	"errors"
	"os"
	"time"
)

const DefaultClusterName = "default"

type Config struct {
	Region                   string        `yaml:"region"`
	Cluster                  string        `yaml:"cluster"`
	Service                  string        `yaml:"service"`
	ServiceDefinitionPath    string        `yaml:"service_definition"`
	TaskDefinitionPath       string        `yaml:"task_definition"`
	DeploymentDefinitionPath string        `yaml:"deployment_definition"`
	Timeout                  time.Duration `yaml:"timeout"`
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
