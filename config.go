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

type Config struct {
	RequiredVersion       string           `yaml:"requried_version"`
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
func (c *Config) Load(currentVersion string, p string) error {
	if err := gc.LoadWithEnv(c, p); err != nil {
		return err
	}
	c.dir = filepath.Dir(p)
	return c.Restrict(currentVersion)
}

// Restrict restricts a configuration.
func (c *Config) Restrict(currentVersion string) error {
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

	if c.RequiredVersion != "" {
		constraints, err := gv.NewConstraint(c.RequiredVersion)
		if err != nil {
			return errors.Wrap(err, "required_version has invalid format")
		}
		if !checkRequiredVersion(currentVersion, constraints) {
			return errors.Errorf("version %s does not satisfy constraints required_version: %s", currentVersion, constraints)
		}
	}

	return nil
}

func checkRequiredVersion(currentVersion string, constraints gv.Constraints) bool {
	v, err := gv.NewVersion(currentVersion)
	if err != nil {
		return true
	}
	return constraints.Check(v)
}

func NewDefaultConfig() *Config {
	return &Config{
		Region:  os.Getenv("AWS_REGION"),
		Timeout: DefaultTimeout,
	}
}
