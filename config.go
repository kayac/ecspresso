package ecspresso

import (
	"errors"
	"log"
	"os"
	"time"
)

const (
	DefaultClusterName = "default"
	DefaultTimeout     = 10 * time.Minute
)

type Config struct {
	Region            string                   `yaml:"region"`
	Cluster           string                   `yaml:"cluster"`
	Service           string                   `yaml:"service"`
	ServiceDefinition *ConfigServiceDefinition `yaml:"service_definition"`
	TaskDefinition    *ConfigTaskDefinition    `yaml:"task_definition"`
	Timeout           time.Duration            `yaml:"timeout"`
}

type configServiceDefinition struct {
	Path string            `yaml:"path"`
	Tags map[string]string `yaml:"tags"`
}

type ConfigServiceDefinition struct {
	configServiceDefinition
}

func (c *ConfigServiceDefinition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&c.Path); err == nil {
		return nil
	}
	if err := unmarshal(&c.configServiceDefinition); err != nil {
		return err
	}
	return nil
}

func (c *ConfigServiceDefinition) MarshalYAML() (interface{}, error) {
	if c.Tags == nil || len(c.Tags) == 0 {
		return c.Path, nil
	}
	return c.configServiceDefinition, nil
}

type configTaskDefinition struct {
	Path string            `yaml:"path"`
	Tags map[string]string `yaml:"tags"`
}

type ConfigTaskDefinition struct {
	configTaskDefinition
}

func (c *ConfigTaskDefinition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&c.Path); err == nil {
		return nil
	}
	if err := unmarshal(&c.configTaskDefinition); err != nil {
		return err
	}
	log.Printf("%#v", c)
	return nil
}

func (c *ConfigTaskDefinition) MarshalYAML() (interface{}, error) {
	if c.Tags == nil || len(c.Tags) == 0 {
		return c.Path, nil
	}
	return c.configTaskDefinition, nil
}

func (c *Config) Validate() error {
	if c.Cluster == "" {
		c.Cluster = DefaultClusterName
	}
	if c.TaskDefinition == nil || c.TaskDefinition.Path == "" {
		return errors.New("task_definition is not defined")
	}
	return nil
}

func NewDefaultConfig() *Config {
	return &Config{
		Region:  os.Getenv("AWS_REGION"),
		Timeout: DefaultTimeout,
	}
}
