package ecspresso

import (
	"os"
	"path/filepath"
	"text/template"
	"time"

	gv "github.com/hashicorp/go-version"
	"github.com/kayac/ecspresso/appspec"
	gc "github.com/kayac/go-config"
	"github.com/pkg/errors"
)

const (
	DefaultClusterName = "default"
	DefaultTimeout     = 10 * time.Minute
)

// Config represents a configuration.
type Config struct {
	RequiredVersion       string           `yaml:"requried_version,omitempty"`
	Region                string           `yaml:"region"`
	Cluster               string           `yaml:"cluster"`
	Service               string           `yaml:"service"`
	ServiceDefinitionPath string           `yaml:"service_definition"`
	TaskDefinitionPath    string           `yaml:"task_definition"`
	Timeout               time.Duration    `yaml:"timeout"`
	Plugins               []ConfigPlugin   `yaml:"plugins,omitempty"`
	AppSpec               *appspec.AppSpec `yaml:"appspec,omitempty"`

	templateFuncs []template.FuncMap
	dir           string
}

// Load loads configuration file from file path.
func (c *Config) Load(path string) error {
	if err := gc.LoadWithEnv(c, path); err != nil {
		return err
	}
	c.dir = filepath.Dir(path)
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
	if c.ServiceDefinitionPath != "" && !filepath.IsAbs(c.ServiceDefinitionPath) {
		c.ServiceDefinitionPath = filepath.Join(c.dir, c.ServiceDefinitionPath)
	}
	if c.TaskDefinitionPath != "" && !filepath.IsAbs(c.TaskDefinitionPath) {
		c.TaskDefinitionPath = filepath.Join(c.dir, c.TaskDefinitionPath)
	}

	for _, p := range c.Plugins {
		if err := p.Setup(c); err != nil {
			return err
		}
	}
	return nil
}

// ValidateVersion validates current version satisfies required_version.
func (c *Config) ValidateVersion(current string) error {
	if c.RequiredVersion == "" {
		return nil
	}
	constraints, err := gv.NewConstraint(c.RequiredVersion)
	if err != nil {
		return errors.Wrap(err, "required_version has invalid format")
	}
	v, err := gv.NewVersion(current)
	if err != nil {
		// invalid version string (e.g. "current") always allowed
		return nil
	}
	if !constraints.Check(v) {
		return errors.Errorf("version %s does not satisfy constraints required_version: %s", current, constraints)
	}

	return nil
}

// NewDefaultConfig creates a default configuration.
func NewDefaultConfig() *Config {
	return &Config{
		Region:  os.Getenv("AWS_REGION"),
		Timeout: DefaultTimeout,
	}
}
