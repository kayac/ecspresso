package ecspresso_test

import (
	"testing"
	"time"

	"github.com/kayac/ecspresso"
	"github.com/kayac/go-config"
)

func TestLoadServiceDefinition(t *testing.T) {
	path := "tests/sv.json"
	td := &ecspresso.ConfigTaskDefinition{}
	td.Path = "tests/td.json"
	c := &ecspresso.Config{
		Region:         "ap-northeast-1",
		Timeout:        300 * time.Second,
		Service:        "test",
		Cluster:        "default2",
		TaskDefinition: td,
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

func TestLoadConfig(t *testing.T) {
	path := "tests/config_simple.yaml"
	var c ecspresso.Config
	if err := config.LoadWithEnv(&c, path); err != nil {
		t.Error(err)
	}
}

func TestLoadConfigWithTags(t *testing.T) {
	path := "tests/config_tags.yaml"
	var c ecspresso.Config
	if err := config.LoadWithEnv(&c, path); err != nil {
		t.Error(err)
	}
}
