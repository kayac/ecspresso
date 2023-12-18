package ecspresso_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/kayac/ecspresso/v2"
)

func TestLoadServiceDefinition(t *testing.T) {
	ctx := context.Background()
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
	ctx := context.Background()
	loader := ecspresso.NewConfigLoader(nil, nil)
	_, err := loader.Load(ctx, "tests/config_duplicate_plugins.yaml", "")
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
	for _, ext := range []string{".yml", ".yaml", ".json", ".jsonnet"} {
		testLoadConfigWithPlugin(t, "tests/ecspresso"+ext)
	}
}

func testLoadConfigWithPlugin(t *testing.T, path string) {
	t.Setenv("TAG", "testing")
	t.Setenv("JSON", `{"foo":"bar"}`)
	t.Setenv("AWS_REGION", "ap-northeast-1")
	ctx := context.Background()
	app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{ConfigFilePath: path})
	if err != nil {
		t.Error(err)
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
	t.Log(svd)
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

	td, err := app.LoadTaskDefinition(conf.TaskDefinitionPath)
	if err != nil {
		t.Error(err)
	}
	t.Log(td)
	image := *td.ContainerDefinitions[0].Image
	if image != "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/app:testing" {
		t.Errorf("unexpected image got:%s", image)
	}
	env := td.ContainerDefinitions[0].Environment[0]
	if *env.Name != "JSON" || *env.Value != `{"foo":"bar"}` {
		t.Errorf("unexpected JSON got:%s", *env.Value)
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
	ctx := context.Background()
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
	ctx := context.Background()
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

	ctx := context.Background()
	loader := ecspresso.NewConfigLoader(nil, nil)
	conf, err := loader.Load(ctx, "tests/notimeout.yml", "")
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
	ctx := context.Background()
	loader := ecspresso.NewConfigLoader(nil, nil)
	for _, ext := range []string{"yml", "json", "jsonnet"} {
		name := "tests/config_codedeploy." + ext
		conf, err := loader.Load(ctx, name, "")
		if err != nil {
			t.Error(err)
		}
		if conf.CodeDeploy.ApplicationName != "myapp" {
			t.Errorf("expected application name, but %v", conf.CodeDeploy.ApplicationName)
		}
		if conf.CodeDeploy.DeploymentGroupName != "mydeployment" {
			t.Errorf("expected deployment group name, but %v", conf.CodeDeploy.DeploymentGroupName)
		}
	}
}

var FilterCommandTests = []struct {
	Env      string
	Expected string
}{
	{"", "fzf"},
	{"peco", "peco"},
}

func TestFilterCommandDeprecated(t *testing.T) {
	ctx := context.Background()
	for _, ts := range FilterCommandTests {
		app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{
			ConfigFilePath: "tests/filter_command.yml",
			FilterCommand:  ts.Env,
		})
		if err != nil {
			t.Error(err)
		}
		if app.FilterCommand() != ts.Expected {
			t.Errorf("expected %s, but got %s", ts.Expected, app.FilterCommand())
		}
	}
}
