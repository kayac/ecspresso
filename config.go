package ecspresso

import (
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/kayac/ecspresso/appspec"
	gc "github.com/kayac/go-config"
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
	if c.ServiceDefinitionPath != "" {
		c.ServiceDefinitionPath = filepath.Join(c.dir, c.ServiceDefinitionPath)
	}
	if c.TaskDefinitionPath != "" {
		c.TaskDefinitionPath = filepath.Join(c.dir, c.TaskDefinitionPath)
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
