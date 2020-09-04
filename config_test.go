package ecspresso_test

import (
	"os"
	"path/filepath"
	"testing"

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
	sv, err := app.LoadServiceDefinition(c.ServiceDefinitionPath)
	if err != nil || sv == nil {
		t.Errorf("%s load failed: %s", c.ServiceDefinitionPath, err)
	}

	if *sv.ServiceName != "test" ||
		*sv.DesiredCount != 2 ||
		*sv.LoadBalancers[0].TargetGroupArn != "arn:aws:elasticloadbalancing:us-east-1:1111111111:targetgroup/test/12345678" ||
		*sv.LaunchType != "EC2" ||
		*sv.SchedulingStrategy != "REPLICA" {
		t.Errorf("unexpected service definition %s", sv.String())
	}
}

func TestLoadConfigWithPlugin(t *testing.T) {
	dir, _ := os.Getwd()
	defer os.Chdir(dir)
	os.Chdir(filepath.Join(dir, "tests"))
	os.Setenv("TAG", "testing")
	os.Setenv("JSON", `{"foo":"bar"}`)

	conf := &ecspresso.Config{}
	err := conf.Load("config.yaml")
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
