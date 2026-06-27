package ecspresso_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kayac/ecspresso/v2"
)

func TestLoadServiceDefinition(t *testing.T) {
	ctx := t.Context()
	app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{ConfigFilePath: "tests/test.yaml"})
	if err != nil {
		t.Error(err)
	}
	c := app.Config()
	for _, ext := range []string{"", "net"} {
		sv, err := app.LoadServiceDefinition(c.ServiceDefinitionPath + ext)
		if err != nil || sv == nil {
			t.Errorf("%s load failed: %s", c.ServiceDefinitionPath, err)
		}

		if *sv.ServiceName != "test" ||
			aws.ToInt32(sv.DesiredCount) != 2 ||
			aws.ToString(sv.LoadBalancers[0].TargetGroupArn) != "arn:aws:elasticloadbalancing:us-east-1:1111111111:targetgroup/test/12345678" ||
			sv.LaunchType != types.LaunchTypeEc2 ||
			sv.SchedulingStrategy != types.SchedulingStrategyReplica ||
			sv.PropagateTags != types.PropagateTagsService ||
			*sv.Tags[0].Key != "cluster" ||
			*sv.Tags[0].Value != "default2" {
			t.Errorf("unexpected service definition %#v", sv)
		}
		if dc := sv.DeploymentConfiguration; dc == nil {
			t.Error("deployment configuration is nil")
		} else {
			if *dc.MaximumPercent != 200 || *dc.MinimumHealthyPercent != 50 {
				t.Errorf("unexpected deployment configuration %#v", dc)
			}
			if dc.Alarms == nil {
				t.Errorf("deployment configuration alarms is nil")
			} else {
				if len(dc.Alarms.AlarmNames) != 1 || dc.Alarms.AlarmNames[0] != "HighResponseLatencyAlarm" {
					t.Errorf("unexpected alarms %#v", dc.Alarms)
				}
			}
		}
	}
}

func TestLoadConfigWithPluginAbsPath(t *testing.T) {
	testLoadConfigWithPlugin(t, "tests/config_abs.yaml")
}

func TestLoadConfigWithPluginMultiple(t *testing.T) {
	testLoadConfigWithPlugin(t, "tests/config_multiple_plugins.yaml")
}

func TestLoadConfigWithPluginDuplicate(t *testing.T) {
	t.Setenv("TAG", "testing")
	t.Setenv("JSON", `{"foo":"bar"}`)
	ctx := t.Context()
	loader := ecspresso.NewConfigLoader(nil, nil)
	_, err := loader.Load(ctx, "tests/config_duplicate_plugins.yaml", "", nil)
	if err == nil {
		t.Log("expected an error to occur, but it didn't.")
		t.FailNow()
	}
	expectedEnds := "already exists. set func_prefix to tfstate plugin"
	if !strings.HasSuffix(err.Error(), expectedEnds) {
		t.Log("unexpected error message")
		t.Log("expected ends:", expectedEnds)
		t.Log("actual:  ", err.Error())
		t.FailNow()
	}
}

func TestLoadConfigWithPlugin(t *testing.T) {
	// .yml / .yaml are excluded: the two-pass loader runs yaml.YAMLToJSON
	// on the raw file to extract `plugins`, so unquoted `{{ ... }}` in
	// YAML scalars is no longer accepted. See
	// TestLoadConfigYAMLUnquotedTemplateIsIncompatible for the regression.
	for _, ext := range []string{".json", ".jsonnet"} {
		t.Run("tests/ecspresso"+ext, func(t *testing.T) {
			testLoadConfigWithPlugin(t, "tests/ecspresso"+ext)
		})
	}
}

// Unquoted `{{ ... }}` at the start of a YAML scalar is parsed by YAML
// as a flow-mapping opener, so the two-pass config loader can no longer
// read these files. This is a documented incompatibility from the
// switch to two-pass plugin loading. The test pins the behaviour to a
// graceful error (not a panic).
func TestLoadConfigYAMLUnquotedTemplateIsIncompatible(t *testing.T) {
	t.Setenv("AWS_REGION", "ap-northeast-1")
	for _, ext := range []string{".yml", ".yaml"} {
		t.Run("tests/ecspresso"+ext, func(t *testing.T) {
			app, err := ecspresso.New(t.Context(), &ecspresso.CLIOptions{
				ConfigFilePath: "tests/ecspresso" + ext,
			})
			if err == nil {
				t.Fatalf("expected error for unquoted YAML template, got app=%v", app)
			}
			if app != nil {
				t.Errorf("expected nil app on error, got %v", app)
			}
		})
	}
}

func testLoadConfigWithPlugin(t *testing.T, path string) {
	t.Setenv("TAG", "testing")
	t.Setenv("JSON", `{"foo":"bar"}`)
	t.Setenv("AWS_REGION", "ap-northeast-1")
	ctx := t.Context()
	app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{ConfigFilePath: path})
	if err != nil {
		t.Fatal(err)
	}
	if app.Name() != "test/default" {
		t.Errorf("unexpected name got %s", app.Name())
	}
	conf := app.Config()
	if conf.Timeout.Duration != time.Minute*10 {
		t.Errorf("unexpected timeout got %s expected %s", conf.Timeout.Duration, time.Minute*10)
	}

	svd, err := app.LoadServiceDefinition(conf.ServiceDefinitionPath)
	if err != nil {
		t.Error(err)
	}
	t.Log(str(svd))
	sgID := svd.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups[0]
	subnetID := svd.NetworkConfiguration.AwsvpcConfiguration.Subnets[0]
	if sgID != "sg-12345678" {
		t.Errorf("unexpected sg id got:%s", sgID)
	}
	if subnetID != "subnet-07ac54af5e41a4fc4" {
		t.Errorf("unexpected subnet id got:%s", subnetID)
	}
	cb := *svd.DeploymentConfiguration.DeploymentCircuitBreaker
	if !cb.Enable {
		t.Errorf("unexpected deploymentCircuitBreaker.enable got:%v", cb.Enable)
	}
	if !cb.Rollback {
		t.Errorf("unexpected deploymentCircuitBreaker.rollback got:%v", cb.Rollback)
	}
	if len(svd.Tags) != 1 {
		t.Errorf("unexpected tags got:%s", str(svd.Tags))
	}
	if tag := svd.Tags[0]; *tag.Key != "Name" || *tag.Value != "test" {
		t.Errorf("unexpected tag got:%s", str(tag))
	}

	td, err := app.LoadTaskDefinition(conf.TaskDefinitionPath)
	if err != nil {
		t.Error(err)
	}
	t.Log(str(td))
	image := *td.ContainerDefinitions[0].Image
	if image != "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/app:testing" {
		t.Errorf("unexpected image got:%s", image)
	}
	env := td.ContainerDefinitions[0].Environment[0]
	if *env.Name != "JSON" || *env.Value != `{"foo":"bar"}` {
		t.Errorf("unexpected JSON got:%s", *env.Value)
	}
	if len(td.Tags) != 1 {
		t.Errorf("unexpected tags got:%s", str(td.Tags))
	}
	if tag := td.Tags[0]; *tag.Key != "Name" || *tag.Value != "test" {
		t.Errorf("unexpected tag got:%s", str(tag))
	}
}

func TestRestrictConfigWithRequiredVersion(t *testing.T) {
	cases := []struct {
		RequiredVersion string
		CurrentVersion  string
	}{
		{
			RequiredVersion: ">= v1.0.0",
			CurrentVersion:  "v1.2.1",
		},
		{
			RequiredVersion: "= v1.0.0",
			CurrentVersion:  "1.0.0",
		},
		{
			RequiredVersion: "~> v1.1.0",
			CurrentVersion:  "1.1.5",
		},
		{
			RequiredVersion: "~> v1.0",
			CurrentVersion:  "1.2.1",
		},
		{
			RequiredVersion: ">= v1, < v2",
			CurrentVersion:  "1.2.1",
		},
		{
			RequiredVersion: ">= v1.2.1, < v2",
			CurrentVersion:  "v1.2.1+3-g04fdc8e",
		},
		{
			RequiredVersion: ">= v1",
			CurrentVersion:  "current",
		},
	}
	for _, c := range cases {
		t.Run(c.CurrentVersion+":"+c.RequiredVersion, func(t *testing.T) {
			conf := ecspresso.NewDefaultConfig()
			conf.RequiredVersion = c.RequiredVersion

			if err := conf.ValidateVersion(c.CurrentVersion); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestConfigWithRequiredVersionUnsatisfied(t *testing.T) {
	cases := []struct {
		RequiredVersion string
		CurrentVersion  string
		ErrorMessage    string
	}{
		{
			RequiredVersion: "= v1.0.0",
			CurrentVersion:  "v1.2.1",
			ErrorMessage:    "does not satisfy constraints",
		},
		{
			RequiredVersion: "~> v1.1.0",
			CurrentVersion:  "v1.2.0",
			ErrorMessage:    "does not satisfy constraints",
		},
		{
			RequiredVersion: ">= v1.2.2, < v2",
			CurrentVersion:  "v1.2.1+3-g04fdc8e",
			ErrorMessage:    "does not satisfy constraints",
		},
		{
			RequiredVersion: ">= v0, <v1",
			CurrentVersion:  "v1.2.1",
			ErrorMessage:    "does not satisfy constraints",
		},
	}
	ctx := t.Context()
	for _, c := range cases {
		t.Run(c.CurrentVersion+":"+c.RequiredVersion, func(t *testing.T) {
			conf := ecspresso.NewDefaultConfig()
			conf.RequiredVersion = c.RequiredVersion
			if err := conf.Restrict(ctx); err != nil {
				t.Error(err)
				return
			}
			err := conf.ValidateVersion(c.CurrentVersion)
			if err == nil {
				t.Error("expected any error, but no error")
				return
			}
			if !strings.Contains(err.Error(), c.ErrorMessage) {
				t.Errorf("unexpected error got:%s", err)
			}
		})
	}
}

func TestConfigWithInvalidRequiredVersion(t *testing.T) {
	cases := []struct {
		RequiredVersion string
		CurrentVersion  string
		ErrorMessage    string
	}{
		{
			RequiredVersion: "hoge",
			CurrentVersion:  "v1.2.1",
			ErrorMessage:    "invalid format",
		},
	}
	ctx := t.Context()
	for _, c := range cases {
		t.Run(c.CurrentVersion+":"+c.RequiredVersion, func(t *testing.T) {
			conf := ecspresso.NewDefaultConfig()
			conf.RequiredVersion = c.RequiredVersion
			err := conf.Restrict(ctx)
			if err == nil {
				t.Error("expected any error, but no error")
				return
			}
			if !strings.Contains(err.Error(), c.ErrorMessage) {
				t.Errorf("unexpected error got:%s", err)
			}
		})
	}
}

func TestLoadConfigWithoutTimeout(t *testing.T) {
	t.Setenv("AWS_REGION", "ap-northeast-2")

	ctx := t.Context()
	loader := ecspresso.NewConfigLoader(nil, nil)
	conf, err := loader.Load(ctx, "tests/notimeout.yml", "", nil)
	if err != nil {
		t.Log("unexpected an error", err)
		t.FailNow()
	}
	if conf.Timeout == nil {
		t.Error("expected default timeout, but nil")
	}
	if conf.Timeout.Duration != ecspresso.DefaultTimeout {
		t.Errorf("expected default timeout, but %v", conf.Timeout.Duration)
	}

	if conf.Region != "ap-northeast-2" {
		t.Errorf("expected region from AWS_REGION, but %v", conf.Region)
	}
}

func TestLoadConfigForCodeDeploy(t *testing.T) {
	ctx := t.Context()
	loader := ecspresso.NewConfigLoader(nil, nil)
	for _, ext := range []string{"yml", "json", "jsonnet"} {
		name := "tests/config_codedeploy." + ext
		conf, err := loader.Load(ctx, name, "", nil)
		if err != nil {
			t.Error(err)
		}
		if conf.CodeDeploy.ApplicationName != "myapp" {
			t.Errorf("expected application name=myapp, but %v", conf.CodeDeploy.ApplicationName)
		}
		if conf.CodeDeploy.DeploymentGroupName != "mydeployment" {
			t.Errorf("expected deployment group name=mydeployment, but %v", conf.CodeDeploy.DeploymentGroupName)
		}
		if conf.CodeDeploy.DeploymentConfigName != "myConfigName" {
			t.Errorf("expected deployment config name=myConfigName, but %v", conf.CodeDeploy.DeploymentConfigName)
		}
	}
}

func TestFilterCommandFromCLIOption(t *testing.T) {
	ctx := t.Context()
	for _, want := range []string{"", "peco", "fzf"} {
		app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{
			ConfigFilePath: "tests/test.yaml",
			FilterCommand:  want,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := app.FilterCommand(); got != want {
			t.Errorf("FilterCommand: want %q, got %q", want, got)
		}
	}
}

var ConfigIgnoreTests = []struct {
	name         string
	ignore       *ecspresso.ConfigIgnore
	resourceTags []types.Tag
	expectedTags []types.Tag
}{
	{
		name:   "ignore all",
		ignore: &ecspresso.ConfigIgnore{Tags: []string{"foo", "bar", "baz"}},
		resourceTags: []types.Tag{
			{Key: ptr("foo"), Value: ptr("x")},
			{Key: ptr("bar"), Value: ptr("y")},
			{Key: ptr("baz"), Value: ptr("z")},
		},
		expectedTags: []types.Tag{},
	},
	{
		name:   "ignore some",
		ignore: &ecspresso.ConfigIgnore{Tags: []string{"foo", "bar"}},
		resourceTags: []types.Tag{
			{Key: ptr("foo"), Value: ptr("x")},
			{Key: ptr("bar"), Value: ptr("y")},
			{Key: ptr("baz"), Value: ptr("z")},
		},
		expectedTags: []types.Tag{
			{Key: ptr("baz"), Value: ptr("z")},
		},
	},
	{
		name:   "ignore nil",
		ignore: nil,
		resourceTags: []types.Tag{
			{Key: ptr("foo"), Value: ptr("x")},
			{Key: ptr("bar"), Value: ptr("y")},
		},
		expectedTags: []types.Tag{
			{Key: ptr("foo"), Value: ptr("x")},
			{Key: ptr("bar"), Value: ptr("y")},
		},
	},
	{
		name:   "ignore case sensitive",
		ignore: &ecspresso.ConfigIgnore{Tags: []string{"Foo"}},
		resourceTags: []types.Tag{
			{Key: ptr("foo"), Value: ptr("x")},
			{Key: ptr("Foo"), Value: ptr("y")},
		},
		expectedTags: []types.Tag{
			{Key: ptr("foo"), Value: ptr("x")},
		},
	},
}

func TestConfigIgnore(t *testing.T) {
	opt := cmpopts.IgnoreUnexported(types.Tag{})
	for _, tt := range ConfigIgnoreTests {
		t.Run(tt.name, func(t *testing.T) {
			tags := tt.ignore.FilterTags(tt.resourceTags)
			if diff := cmp.Diff(tt.expectedTags, tags, opt); diff != "" {
				t.Errorf("unexpected tags (-want +got):\n%s", diff)
			}
		})
	}
}

// When the tfstate plugin has optional=true and the configured path/url
// cannot be read, ecspresso must continue with an empty state instead of
// failing config load.
func TestLoadConfigWithTFStatePluginOptional(t *testing.T) {
	t.Setenv("AWS_REGION", "ap-northeast-1")
	ctx := t.Context()
	app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{
		ConfigFilePath: "tests/config_tfstate_optional.yaml",
	})
	if err != nil {
		t.Fatalf("expected New to succeed with optional tfstate plugin, got: %s", err)
	}
	if app == nil {
		t.Fatal("app is nil")
	}
}

// Verifies that the tfstate plugin can be used in top-level config
// fields (cluster / service / region / etc.), not only in task and
// service definition templates. ecspresso evaluates the file in two
// passes: pass 1 extracts only `plugins`, pass 2 re-reads the whole
// file with the tfstate function registered. Same fixture content
// across jsonnet / yaml / json to confirm the unified extraction
// works for every supported format.
func TestLoadConfigWithTFStateInConfig(t *testing.T) {
	for _, ext := range []string{".jsonnet", ".yaml", ".json"} {
		t.Run(ext, func(t *testing.T) {
			t.Setenv("AWS_REGION", "ap-northeast-1")
			ctx := t.Context()
			app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{
				ConfigFilePath: "tests/config_tfstate_in_config" + ext,
			})
			if err != nil {
				t.Fatalf("New failed: %s", err)
			}
			// tests/terraform.tfstate has an aws_ecs_cluster.main
			// resource whose name is "test-cluster"; every fixture
			// sets cluster: tfstate("aws_ecs_cluster.main.name").
			if got, want := app.Config().Cluster, "test-cluster"; got != want {
				t.Errorf("cluster = %q, want %q (tfstate-resolved value)", got, want)
			}
			if got, want := app.Name(), "test/test-cluster"; got != want {
				t.Errorf("app.Name() = %q, want %q", got, want)
			}
		})
	}
}

// A caller-supplied *tfstate.TFState injected via WithPluginInstance
// must drive `tfstate(...)` lookups in the config even when the
// config file has no `plugins:` block at all. Confirms the loader
// registers funcMaps from pre-provided instances that match no
// config plugin entry.
func TestLoadConfigWithPluginInstanceNoConfigEntry(t *testing.T) {
	t.Setenv("AWS_REGION", "ap-northeast-1")
	state := tfstate.Empty()
	state.SetOverrides(map[string]any{
		"aws_ecs_cluster.main.name": "injected-cluster",
		"aws_ecs_service.main.name": "injected-service",
	})
	ctx := t.Context()
	app, err := ecspresso.New(ctx,
		&ecspresso.CLIOptions{ConfigFilePath: "tests/config_tfstate_injected.yaml"},
		ecspresso.WithPluginInstance("tfstate", "", state),
	)
	if err != nil {
		t.Fatalf("New failed: %s", err)
	}
	if got, want := app.Config().Cluster, "injected-cluster"; got != want {
		t.Errorf("cluster = %q, want %q (pre-provided override)", got, want)
	}
	if got, want := app.Config().Service, "injected-service"; got != want {
		t.Errorf("service = %q, want %q (pre-provided override)", got, want)
	}
	if inst := app.PluginInstance("tfstate", ""); inst == nil {
		t.Errorf("PluginInstance(tfstate, \"\") = nil, want the injected *tfstate.TFState")
	}
}

// A pre-provided tfstate instance wins over a matching `plugins:`
// entry in the config: the entry's Setup is skipped (so a broken
// `path:` does not fail Load), and lookups resolve from the
// in-memory overrides.
func TestLoadConfigWithPluginInstanceWinsOverConfigEntry(t *testing.T) {
	t.Setenv("AWS_REGION", "ap-northeast-1")
	state := tfstate.Empty()
	state.SetOverrides(map[string]any{
		"aws_ecs_cluster.main.name": "from-override",
	})
	ctx := t.Context()
	app, err := ecspresso.New(ctx,
		// This fixture has plugins: [{name: tfstate, config: {path: terraform.tfstate}}].
		// The on-disk tfstate file's value is "test-cluster"; the
		// override below is "from-override". If the pre-provided
		// instance correctly wins, we see "from-override".
		&ecspresso.CLIOptions{ConfigFilePath: "tests/config_tfstate_in_config.yaml"},
		ecspresso.WithPluginInstance("tfstate", "", state),
	)
	if err != nil {
		t.Fatalf("New failed: %s", err)
	}
	if got, want := app.Config().Cluster, "from-override"; got != want {
		t.Errorf("cluster = %q, want %q (pre-provided override wins)", got, want)
	}
}

// optional must be a bool. Other types (e.g. the string "true") are
// rejected so users do not silently get the empty-state fallback by typo.
func TestLoadConfigWithTFStatePluginOptionalInvalidType(t *testing.T) {
	t.Setenv("AWS_REGION", "ap-northeast-1")
	ctx := t.Context()
	_, err := ecspresso.New(ctx, &ecspresso.CLIOptions{
		ConfigFilePath: "tests/config_tfstate_optional_invalid.yaml",
	})
	if err == nil {
		t.Fatal("expected error for non-bool optional, got nil")
	}
	if !strings.Contains(err.Error(), "optional must be a bool") {
		t.Errorf("unexpected error message: %s", err)
	}
}

func TestLoadServiceDefinitionWithMonitoring(t *testing.T) {
	src, err := os.ReadFile("tests/sv-monitoring.json")
	if err != nil {
		t.Fatal(err)
	}
	var sv ecspresso.Service
	if err := ecspresso.UnmarshalJSONForStruct(src, &sv, "tests/sv-monitoring.json"); err != nil {
		t.Fatal(err)
	}
	if sv.Monitoring == nil {
		t.Fatal("monitoring is nil")
	}
	if len(sv.Monitoring.MetricConfigurations) != 1 {
		t.Fatalf("unexpected metricConfigurations length: %d", len(sv.Monitoring.MetricConfigurations))
	}
	mc := sv.Monitoring.MetricConfigurations[0]
	if diff := cmp.Diff(mc.MetricNames, []string{"CPUUtilization", "MemoryUtilization"}); diff != "" {
		t.Errorf("unexpected metricNames (-want +got):\n%s", diff)
	}
	if aws.ToInt32(mc.ResolutionSeconds) != 20 {
		t.Errorf("unexpected resolutionSeconds: %d", aws.ToInt32(mc.ResolutionSeconds))
	}
}
