package ecspresso_test

import (
	"testing"
	"time"

	"github.com/kayac/ecspresso"
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
