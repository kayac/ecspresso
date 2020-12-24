package ecspresso

import (
	"os"
	"path/filepath"
	"strings"
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
			return errors.Wrap(err, "required_version is invalid format")
		}
		onlyConstraintLessThan := true
		for _, constraint := range constraints {
			if !strings.HasPrefix(strings.Trim(constraint.String(), " "), "<") {
				onlyConstraintLessThan = false
				break
			}
		}
		if onlyConstraintLessThan {
			return errors.New("required_version cannot only be less than a constraint")
		}
		if !checkRequiredVersion(currentVersion, constraints) {
			return errors.New("the current version does not meet the required_version")
		}
	}

	return nil
}

func checkRequiredVersion(currentVersion string, constraints gv.Constraints) bool {
	if currentVersion == "current" {
		// if only GreaterThan constraint, pass check required version
		for _, constraint := range constraints {
			if !strings.HasPrefix(strings.Trim(constraint.String(), " "), ">") {
				return false
			}
		}
		return true
	}
	v, err := gv.NewVersion(currentVersion)
	if err != nil {
		return false
	}
	return constraints.Check(v)
}

func NewDefaultConfig() *Config {
	return &Config{
		Region:  os.Getenv("AWS_REGION"),
		Timeout: DefaultTimeout,
	}
}
