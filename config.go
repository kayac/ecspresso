package ecspresso

import (
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/kayac/ecspresso/appspec"
	gc "github.com/kayac/go-config"
	"github.com/pkg/errors"
)

const (
	DefaultClusterName = "default"
	DefaultTimeout     = 10 * time.Minute
)

type Config struct {
	Region                string           `yaml:"region"`
	Cluster               string           `yaml:"cluster"`
	Service               string           `yaml:"service"`
	ServiceDefinitionPath string           `yaml:"service_definition"`
	TaskDefinitionPath    string           `yaml:"task_definition"`
	Timeout               time.Duration    `yaml:"timeout"`
	Plugins               []ConfigPlugin   `yaml:"plugins"`
	AppSpec               *appspec.AppSpec `yaml:"appspec"`

	templateFuncs []template.FuncMap
	dir           string
}

// Load loads configuration file from file path.
func (c *Config) Load(p string) error {
	if err := gc.LoadWithEnv(c, p); err != nil {
		return err
	}
	c.dir = filepath.Dir(p)
	return c.Restrict()
}

// Restrict restricts a configuration.
func (c *Config) Restrict() error {
	if c.Cluster == "" {
		c.Cluster = DefaultClusterName
	}
	if c.dir == "" {
		c.dir = "."
	}
	c.ServiceDefinitionPath = filepath.Join(c.dir, c.ServiceDefinitionPath)
	if _, err := os.Stat(c.ServiceDefinitionPath); err != nil {
		return errors.Wrapf(err, "service_definition:%s is not found", c.ServiceDefinitionPath)
	}

	c.TaskDefinitionPath = filepath.Join(c.dir, c.TaskDefinitionPath)
	if _, err := os.Stat(c.TaskDefinitionPath); err != nil {
		return errors.Wrapf(err, "task_definition:%s is not found", c.TaskDefinitionPath)
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
