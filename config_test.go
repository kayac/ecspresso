package ecspresso_test

import (
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/kayac/ecspresso"
)

func TestLoadServiceDefinition(t *testing.T) {
	c := &ecspresso.Config{}
	err := c.Load("tests/test.yaml")
	if err != nil {
		t.Error(err)
	}
	app, err := ecspresso.NewApp(c)
	if err != nil {
		t.Error(err)
	}
	for _, ext := range []string{"", "net"} {
		sv, err := app.LoadServiceDefinition(c.ServiceDefinitionPath + ext)
		if err != nil || sv == nil {
			t.Errorf("%s load failed: %s", c.ServiceDefinitionPath, err)
		}

		if *sv.ServiceName != "test" ||
			*sv.DesiredCount != 2 ||
			*sv.LoadBalancers[0].TargetGroupArn != "arn:aws:elasticloadbalancing:us-east-1:1111111111:targetgroup/test/12345678" ||
			*sv.LaunchType != "EC2" ||
			*sv.SchedulingStrategy != "REPLICA" ||
			*sv.PropagateTags != "SERVICE" ||
			*sv.Tags[0].Key != "cluster" ||
			*sv.Tags[0].Value != "default2" {
			t.Errorf("unexpected service definition %s", sv.String())
		}
	}
}

func TestLoadDeploymentDefinition(t *testing.T) {
	c := &ecspresso.Config{}
	err := c.Load("tests/test.yaml")
	if err != nil {
		t.Error(err)
	}
	app, err := ecspresso.NewApp(c)
	if err != nil {
		t.Error(err)
	}
	for _, ext := range []string{"", "net"} {
		dd, err := app.LoadDeploymentDefinition(c.DeploymentDefinitionPath + ext)
		if err != nil || dd == nil {
			t.Errorf("%s load failed: %s", c.DeploymentDefinitionPath, err)
		}

		if *dd.ApplicationName != "ecs-test-app" ||
			*dd.DeploymentConfigName != "CodeDeployDefault.ECSAllAtOnce" ||
			*dd.DeploymentGroupName != "ecs-test-deployment-group" {
			t.Errorf("unexpected deployment definition")
		}
	}
}

func TestLoadConfigWithPluginAbsPath(t *testing.T) {
	testLoadConfigWithPlugin(t, "tests/config_abs.yaml")
}

func TestLoadConfigWithPlugin(t *testing.T) {
	testLoadConfigWithPlugin(t, "tests/ecspresso.yml")
}

func testLoadConfigWithPlugin(t *testing.T, path string) {
	os.Setenv("TAG", "testing")
	os.Setenv("JSON", `{"foo":"bar"}`)

	conf := &ecspresso.Config{}
	err := conf.Load(path)
	if err != nil {
		t.Error(err)
	}
	app, err := ecspresso.NewApp(conf)
	if err != nil {
		t.Error(err)
	}
	if app.Name() != "test/default" {
		t.Errorf("unexpected name got %s", app.Name())
	}

	svd, err := app.LoadServiceDefinition(conf.ServiceDefinitionPath)
	if err != nil {
		t.Error(err)
	}
	t.Log(svd.String())
	sgID := *svd.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups[0]
	subnetID := *svd.NetworkConfiguration.AwsvpcConfiguration.Subnets[0]
	if sgID != "sg-12345678" {
		t.Errorf("unexpected sg id got:%s", sgID)
	}
	if subnetID != "subnet-07ac54af5e41a4fc4" {
		t.Errorf("unexpected subnet id got:%s", subnetID)
	}
	cb := *svd.DeploymentConfiguration.DeploymentCircuitBreaker
	if !aws.BoolValue(cb.Enable) {
		t.Errorf("unexpected deploymentCircuitBreaker.enable got:%v", *cb.Enable)
	}
	if !aws.BoolValue(cb.Rollback) {
		t.Errorf("unexpected deploymentCircuitBreaker.rollback got:%v", *cb.Rollback)
	}

	td, err := app.LoadTaskDefinition(conf.TaskDefinitionPath)
	if err != nil {
		t.Error(err)
	}
	t.Log(td.String())
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
	for _, c := range cases {
		t.Run(c.CurrentVersion+":"+c.RequiredVersion, func(t *testing.T) {
			conf := ecspresso.NewDefaultConfig()
			conf.RequiredVersion = c.RequiredVersion
			if err := conf.Restrict(); err != nil {
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
	for _, c := range cases {
		t.Run(c.CurrentVersion+":"+c.RequiredVersion, func(t *testing.T) {
			conf := ecspresso.NewDefaultConfig()
			conf.RequiredVersion = c.RequiredVersion
			err := conf.Restrict()
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
