package ecspresso

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	Region             string        `yaml:"region"`
	Service            string        `yaml:"service"`
	Cluster            string        `yaml:"cluster"`
	TaskDefinitionPath string        `yaml:"task_definition"`
	Timeout            time.Duration `yaml:"timeout"`
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
