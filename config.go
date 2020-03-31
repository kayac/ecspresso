package ecspresso

import (
	"errors"
	"os"
	"text/template"
	"time"
)

const (
	DefaultClusterName = "default"
	DefaultTimeout     = 10 * time.Minute
)

type Config struct {
	Region                string         `yaml:"region"`
	Cluster               string         `yaml:"cluster"`
	Service               string         `yaml:"service"`
	ServiceDefinitionPath string         `yaml:"service_definition"`
	TaskDefinitionPath    string         `yaml:"task_definition"`
	Timeout               time.Duration  `yaml:"timeout"`
	Plugins               []ConfigPlugin `yaml:"plugins"`

	templateFuncs []template.FuncMap
}

func (c *Config) Validate() error {
	if c.Cluster == "" {
		c.Cluster = DefaultClusterName
	}
	if c.TaskDefinitionPath == "" {
		return errors.New("task_definition is not defined")
	}
	for _, p := range c.Plugins {
		if err := p.Setup(c); err != nil {
			return err
		}
	}
	return nil
}

func NewDefaultConfig() *Config {
	return &Config{
		Region:  os.Getenv("AWS_REGION"),
		Timeout: DefaultTimeout,
	}
}
