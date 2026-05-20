package ecspresso

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/goccy/go-yaml"
	"github.com/google/go-jsonnet"
	goVersion "github.com/hashicorp/go-version"
	"github.com/kayac/ecspresso/v2/appspec"
	goConfig "github.com/kayac/go-config"
	"github.com/samber/lo"
)

const (
	DefaultClusterName = "default"
	DefaultTimeout     = 10 * time.Minute
)

var awsv2ConfigLoadOptionsFunc []func(*awsConfig.LoadOptions) error

type configLoader struct {
	*goConfig.Loader
	VM *jsonnet.VM
}

func newConfigLoader(extStr, extCode map[string]string) *configLoader {
	vm := jsonnet.MakeVM()
	for k, v := range extStr {
		vm.ExtVar(k, v)
	}
	for k, v := range extCode {
		vm.ExtCode(k, v)
	}
	for _, f := range DefaultJsonnetNativeFuncs() {
		vm.NativeFunction(f)
	}
	return &configLoader{
		Loader: goConfig.New(),
		VM:     vm,
	}
}

// Config represents a configuration.
type Config struct {
	RequiredVersion       string            `yaml:"required_version,omitempty" json:"required_version,omitempty"`
	Region                string            `yaml:"region" json:"region"`
	Cluster               string            `yaml:"cluster" json:"cluster"`
	Service               string            `yaml:"service" json:"service"`
	ServiceDefinitionPath string            `yaml:"service_definition,omitempty" json:"service_definition,omitempty"`
	TaskDefinitionPath    string            `yaml:"task_definition,omitempty" json:"task_definition,omitempty"`
	ExpressDefinitionPath string            `yaml:"express_definition,omitempty" json:"express_definition,omitempty"`
	Plugins               []ConfigPlugin    `yaml:"plugins,omitempty" json:"plugins,omitempty"`
	AppSpec               *appspec.AppSpec  `yaml:"appspec,omitempty" json:"appspec,omitempty"`
	Timeout               *Duration         `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	CodeDeploy            *ConfigCodeDeploy `yaml:"codedeploy,omitempty" json:"codedeploy,omitempty"`
	Ignore                *ConfigIgnore     `yaml:"ignore,omitempty" json:"ignore,omitempty"`

	path               string
	templateFuncs      []template.FuncMap
	jsonnetNativeFuncs []*jsonnet.NativeFunction
	dir                string
	versionConstraints goVersion.Constraints
	awsv2Config        aws.Config

	// pluginInstances records the runtime instance each plugin's Setup
	// produces, when the plugin has something callers may want to access
	// after setup. Currently only the tfstate plugin uses this (storing
	// its *tfstate.TFState so callers can inject overrides via
	// App.TFState). Other plugins resolve at template-render time and
	// register nothing here.
	pluginInstances []pluginInstance

	// pluginsConfigured is set once plugin Setup has run. The config
	// loader runs a two-pass evaluation so that the tfstate / cfn / ssm
	// jsonnet native functions and template funcs declared by plugins
	// are available throughout the config (not just in task and service
	// definitions): pass 1 extracts only the `plugins` field, plugin
	// Setup runs, then pass 2 re-reads the whole file with plugin funcs
	// available. This flag tells Restrict to skip a second setup.
	pluginsConfigured bool
}

// pluginsOnly is used by extractPlugins to peel just the plugins
// section (plus the region literal) off a YAML / JSON config. Other
// fields are silently dropped — they may reference plugin-provided
// functions that are not yet available on the first pass. Region is
// pulled out so AWS-dependent plugins (ssm / secretsmanager / cfn)
// are initialised against the region configured in the file, not the
// AWS_REGION env fallback.
type pluginsOnly struct {
	Plugins []ConfigPlugin `yaml:"plugins" json:"plugins"`
	Region  string         `yaml:"region" json:"region"`
}

type pluginInstance struct {
	name       string
	funcPrefix string
	value      any
}

type ConfigCodeDeploy struct {
	ApplicationName      string `yaml:"application_name,omitempty" json:"application_name,omitempty"`
	DeploymentGroupName  string `yaml:"deployment_group_name,omitempty" json:"deployment_group_name,omitempty"`
	DeploymentConfigName string `yaml:"deployment_config_name,omitempty" json:"deployment_config_name,omitempty"`
}

// Load loads configuration file from file path.
//
// Loading runs in two passes so that the tfstate / cfn / ssm functions
// declared by plugins are available throughout the config (not just in
// task / service definitions):
//
//  1. extractPlugins pulls just the `plugins` section out of the file
//     in a format-appropriate way (lazy field eval for jsonnet, raw
//     unmarshal for YAML / JSON). The result is template-rendered with
//     the default funcs (env, must_env) and decoded into []ConfigPlugin.
//  2. prepareAWSAndPlugins loads AWS config and runs plugin Setup, which
//     registers each plugin's template funcs and jsonnet native funcs.
//     The loader carries them onto its shared template loader and VM
//     before the full file is re-read.
//
// The `plugins` section itself cannot reference plugin-provided
// functions — that would be a chicken-and-egg loop.
func (l *configLoader) Load(ctx context.Context, path string, version string) (*Config, error) {
	conf := &Config{path: path}
	ext := filepath.Ext(path)
	switch ext {
	case ymlExt, yamlExt, jsonExt, jsonnetExt:
		// supported; continue.
	default:
		return nil, fmt.Errorf("unsupported config file extension: %s", ext)
	}

	// Pass 1.
	plugins, region, err := l.extractPlugins(path, ext)
	if err != nil {
		return nil, err
	}
	pre := &Config{path: path, dir: filepath.Dir(path), Plugins: plugins, Region: region}
	if err := pre.prepareAWSAndPlugins(ctx); err != nil {
		return nil, err
	}
	for _, f := range pre.jsonnetNativeFuncs {
		l.VM.NativeFunction(f)
	}
	for _, f := range pre.templateFuncs {
		l.Funcs(f)
	}

	// Pass 2.
	switch ext {
	case ymlExt, yamlExt:
		b, err := l.ReadWithEnv(path)
		if err != nil {
			return nil, err
		}
		if err := unmarshalYAML(b, conf, path); err != nil {
			return nil, fmt.Errorf("failed to parse yaml: %w", err)
		}
	case jsonExt, jsonnetExt:
		jsonStr, err := l.VM.EvaluateFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate jsonnet file: %w", err)
		}
		b, err := l.ReadWithEnvBytes([]byte(jsonStr))
		if err != nil {
			return nil, fmt.Errorf("failed to read template file: %w", err)
		}
		if err := unmarshalJSON(b, conf, path); err != nil {
			return nil, fmt.Errorf("failed to unmarshal json: %w", err)
		}
	}

	// Carry the pre-setup state into the final Config so Restrict does
	// not re-run plugin setup.
	conf.awsv2Config = pre.awsv2Config
	conf.pluginInstances = pre.pluginInstances
	conf.templateFuncs = pre.templateFuncs
	conf.jsonnetNativeFuncs = pre.jsonnetNativeFuncs
	conf.pluginsConfigured = true

	conf.dir = filepath.Dir(path)
	if err := conf.Restrict(ctx); err != nil {
		return nil, err
	}
	if err := conf.ValidateVersion(version); err != nil {
		return nil, err
	}
	return conf, nil
}

// extractPlugins returns the parsed `plugins` section of a config file
// plus a best-effort `region` value, with `{{ env / must_env }}` template
// literals expanded. The rest of the config is dropped — it may
// reference plugin-provided functions that have not been set up yet.
//
// The shape per format is:
//
//   - jsonnet: `(import "<abs-path>").plugins` (and `.region`) —
//     jsonnet evaluates object fields lazily, so `tfstate()` calls in
//     `cluster` etc. are not reached. Region is extracted separately
//     so a `region: tfstate(...)` expression failing to evaluate does
//     not lose the plugin list.
//   - JSON: jsonnet VM evaluates the file (.json is a jsonnet subset
//     with no native-function calls in pure JSON), then plain
//     json.Unmarshal into pluginsOnly{} drops every other field. Any
//     `{{ tfstate ... }}` template literals in those dropped fields
//     never reach the template engine.
//   - YAML: yaml.YAMLToJSON + json.Unmarshal into pluginsOnly{}
//     (matches the main config loader's YAML→JSON→struct flow).
//     Template literals everywhere except the plugins / region fields
//     are dropped before template rendering sees them.
//
// Plugins and region strings are then template-rendered through the
// default funcs (env / must_env). Per-field rendering (rather than
// JSON / YAML marshal round-tripping) avoids the marshaler's quote-
// escape trap — `{{ env "FOO" }}` would otherwise be serialised as
// `\"FOO\"` and break the Go template parser.
//
// If the extracted region cannot be rendered with the default funcs
// (typically because it references a plugin-provided function), the
// returned region is empty and prepareAWSAndPlugins falls back to the
// AWS_REGION env var. The chicken-and-egg case (`region: tfstate(...)`
// combined with cfn / ssm plugins) is documented as a limitation.
func (l *configLoader) extractPlugins(path, ext string) ([]ConfigPlugin, string, error) {
	var po pluginsOnly
	switch ext {
	case jsonnetExt:
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve config path: %w", err)
		}
		pluginsOut, err := l.VM.EvaluateAnonymousSnippet(
			"<plugins-only>",
			fmt.Sprintf(
				`local c = import %q; if std.objectHas(c, "plugins") then c.plugins else null`,
				absPath,
			),
		)
		if err != nil {
			return nil, "", fmt.Errorf("failed to extract plugins from jsonnet: %w", err)
		}
		if trimmed := bytes.TrimSpace([]byte(pluginsOut)); len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
			if err := json.Unmarshal([]byte(pluginsOut), &po.Plugins); err != nil {
				return nil, "", fmt.Errorf("failed to parse plugins from jsonnet: %w", err)
			}
		}
		// Region is extracted by a separate snippet so its evaluation
		// failing (e.g. `region: tfstate(...)`) does not lose the
		// plugins we just got. Best-effort: any error here is treated
		// as "region not pre-resolvable, fall back to env".
		regionOut, err := l.VM.EvaluateAnonymousSnippet(
			"<region-only>",
			fmt.Sprintf(
				`local c = import %q; if std.objectHas(c, "region") then c.region else null`,
				absPath,
			),
		)
		if err == nil {
			_ = json.Unmarshal([]byte(regionOut), &po.Region)
		}
	case jsonExt:
		out, err := l.VM.EvaluateFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to evaluate json file: %w", err)
		}
		if err := json.Unmarshal([]byte(out), &po); err != nil {
			return nil, "", fmt.Errorf("failed to parse plugins from json: %w", err)
		}
	case ymlExt, yamlExt:
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read config file: %w", err)
		}
		asJSON, err := yaml.YAMLToJSON(raw)
		if err != nil {
			return nil, "", fmt.Errorf("failed to parse plugins from yaml: %w", err)
		}
		if err := json.Unmarshal(asJSON, &po); err != nil {
			return nil, "", fmt.Errorf("failed to parse plugins from yaml: %w", err)
		}
	default:
		return nil, "", nil
	}
	for i := range po.Plugins {
		rendered, err := po.Plugins[i].Render(l.renderString)
		if err != nil {
			return nil, "", fmt.Errorf("plugins[%d]: %w", i, err)
		}
		po.Plugins[i] = rendered
	}
	region, err := l.renderString(po.Region)
	if err != nil {
		// Region references a plugin-provided function we cannot yet
		// resolve. prepareAWSAndPlugins will fall back to AWS_REGION.
		region = ""
	}
	return po.Plugins, region, nil
}

// renderString runs s through the loader's template engine (env /
// must_env / json_escape and any plugin-provided funcs registered
// later). Strings without `{{` are returned as-is so values like plain
// S3 URLs are not paid for through the template parser.
func (l *configLoader) renderString(s string) (string, error) {
	if !strings.Contains(s, "{{") {
		return s, nil
	}
	b, err := l.ReadWithEnvBytes([]byte(s))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Config) OverrideByCLIOptions(opt *CLIOptions) {
	if opt.Timeout != nil {
		c.Timeout = &Duration{*opt.Timeout}
	}
}

// Restrict restricts a configuration.
func (c *Config) Restrict(ctx context.Context) error {
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
	if c.ExpressDefinitionPath != "" && !filepath.IsAbs(c.ExpressDefinitionPath) {
		c.ExpressDefinitionPath = filepath.Join(c.dir, c.ExpressDefinitionPath)
	}
	if c.RequiredVersion != "" {
		constraints, err := goVersion.NewConstraint(c.RequiredVersion)
		if err != nil {
			return fmt.Errorf("required_version has invalid format: %w", err)
		}
		c.versionConstraints = constraints
	}
	if c.Timeout == nil {
		c.Timeout = &Duration{Duration: DefaultTimeout}
	}
	if err := c.loadAWSConfig(ctx); err != nil {
		return err
	}
	if !c.pluginsConfigured {
		if err := c.setupPlugins(ctx); err != nil {
			return fmt.Errorf("failed to setup plugins: %w", err)
		}
		c.pluginsConfigured = true
	}
	return nil
}

// loadAWSConfig builds c.awsv2Config from c.Region (falling back to the
// AWS_REGION env var). Idempotent — Restrict re-runs it after the two-
// pass jsonnet eval so the final c.awsv2Config reflects the Region the
// config resolved to (which may have come from a tfstate lookup).
// Plugins set up earlier keep the awsv2Config that was current at the
// time of their Setup call.
func (c *Config) loadAWSConfig(ctx context.Context) error {
	if c.Region == "" {
		c.Region = os.Getenv("AWS_REGION")
	}
	var optsFunc []func(*awsConfig.LoadOptions) error
	if len(awsv2ConfigLoadOptionsFunc) == 0 {
		optsFunc = []func(*awsConfig.LoadOptions) error{
			awsConfig.WithRegion(c.Region),
		}
	} else {
		optsFunc = awsv2ConfigLoadOptionsFunc
	}
	var err error
	c.awsv2Config, err = awsConfig.LoadDefaultConfig(ctx, optsFunc...)
	if err != nil {
		return fmt.Errorf("failed to load aws config: %w", err)
	}
	return nil
}

// prepareAWSAndPlugins loads c.awsv2Config and runs plugin setup. Used
// by the jsonnet two-pass loader before the second evaluation so plugin-
// supplied native functions are registered on the VM.
func (c *Config) prepareAWSAndPlugins(ctx context.Context) error {
	if err := c.loadAWSConfig(ctx); err != nil {
		return err
	}
	if err := c.setupPlugins(ctx); err != nil {
		return fmt.Errorf("failed to setup plugins: %w", err)
	}
	c.pluginsConfigured = true
	return nil
}

func (c *Config) AssumeRole(assumeRoleARN string) {
	if assumeRoleARN == "" {
		return
	}
	LogInfo("assuming role", "role", assumeRoleARN)
	stsClient := sts.NewFromConfig(c.awsv2Config)
	assumeRoleProvider := stscreds.NewAssumeRoleProvider(stsClient, assumeRoleARN)
	c.awsv2Config.Credentials = aws.NewCredentialsCache(assumeRoleProvider)
}

func (c *Config) setupPlugins(ctx context.Context) error {
	plugins := []ConfigPlugin{}
	for _, name := range defaultPluginNames {
		plugins = append(plugins, ConfigPlugin{Name: name})
	}
	plugins = append(plugins, c.Plugins...)
	for _, p := range plugins {
		if err := p.Setup(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// ValidateVersion validates a version satisfies required_version.
func (c *Config) ValidateVersion(version string) error {
	if c.versionConstraints == nil {
		return nil
	}
	v, err := goVersion.NewVersion(version)
	if err != nil {
		LogWarn("invalid version format, skipping required_version check", "version", version)
		// invalid version string (e.g. "current") always allowed
		return nil
	}
	if !c.versionConstraints.Check(v) {
		return fmt.Errorf("version %s does not satisfy constraints required_version: %s", version, c.versionConstraints)
	}

	return nil
}

func (c *Config) isExpressMode() bool {
	return c.ExpressDefinitionPath != ""
}

// NewDefaultConfig creates a default configuration.
func NewDefaultConfig() *Config {
	return &Config{
		Region:  os.Getenv("AWS_REGION"),
		Timeout: &Duration{DefaultTimeout},
	}
}

type ConfigIgnore struct {
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

type hasTags interface {
	GetTags() []types.Tag
	SetTags([]types.Tag)
}

func (i *ConfigIgnore) filterTags(tags []types.Tag) []types.Tag {
	if i == nil || len(i.Tags) == 0 {
		return tags
	}
	return lo.Filter(tags, func(tag types.Tag, _ int) bool {
		return !lo.Contains(i.Tags, aws.ToString(tag.Key))
	})
}

func (i *ConfigIgnore) Apply(v hasTags) error {
	v.SetTags(i.filterTags(v.GetTags()))
	return nil
}
