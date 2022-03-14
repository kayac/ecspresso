package ecspresso

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/fatih/color"
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
	RequiredVersion       string           `yaml:"required_version,omitempty"`
	Region                string           `yaml:"region"`
	Cluster               string           `yaml:"cluster"`
	Service               string           `yaml:"service"`
	ServiceDefinitionPath string           `yaml:"service_definition"`
	TaskDefinitionPath    string           `yaml:"task_definition"`
	Timeout               time.Duration    `yaml:"timeout"`
	Plugins               ConfigPlugins    `yaml:"plugins,omitempty"`
	AppSpec               *appspec.AppSpec `yaml:"appspec,omitempty"`
	FilterCommand         string           `yaml:"filter_command,omitempty"`

	templateFuncs      []template.FuncMap
	dir                string
	versionConstraints gv.Constraints
	sess               *session.Session
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
	if c.RequiredVersion != "" {
		constraints, err := gv.NewConstraint(c.RequiredVersion)
		if err != nil {
			return errors.Wrap(err, "required_version has invalid format")
		}
		c.versionConstraints = constraints
	}
	var err error
	c.sess, err = session.NewSessionWithOptions(session.Options{
		Config:            aws.Config{Region: aws.String(c.Region)},
		SharedConfigState: session.SharedConfigEnable,
	})
	return err
}

func (c *Config) setupPlugins() error {
	return c.Plugins.Setup(c)
}

// ValidateVersion validates a version satisfies required_version.
func (c *Config) ValidateVersion(version string) error {
	if c.versionConstraints == nil {
		return nil
	}
	v, err := gv.NewVersion(version)
	if err != nil {
		fmt.Fprintln(
			os.Stderr,
			color.YellowString("WARNING: Invalid version format \"%s\". Skip checking required_version.", version),
		)
		// invalid version string (e.g. "current") always allowed
		return nil
	}
	if !c.versionConstraints.Check(v) {
		return errors.Errorf("version %s does not satisfy constraints required_version: %s", version, c.versionConstraints)
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
