package ecspresso_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kayac/ecspresso"
	"github.com/kayac/go-config"
)

func TestLoadServiceDefinition(t *testing.T) {
	path := "tests/sv.json"
	c := &ecspresso.Config{
		Region:             "ap-northeast-1",
		Timeout:            300 * time.Second,
		Service:            "test",
		Cluster:            "default2",
		TaskDefinitionPath: "tests/td.json",
	}
	app, err := ecspresso.NewApp(c)
	if err != nil {
		t.Error(err)
	}
	sv, err := app.LoadServiceDefinition(path)
	if err != nil || sv == nil {
		t.Errorf("%s load failed: %s", path, err)
	}

	if *sv.Cluster != "default2" ||
		*sv.ServiceName != "test" ||
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

	var conf ecspresso.Config
	err := config.LoadWithEnv(&conf, "config.yaml")
	if err != nil {
		t.Error(err)
	}
	app, err := ecspresso.NewApp(&conf)
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
}
