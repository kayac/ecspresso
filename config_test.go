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
		Cluster:            "default",
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
	if *sv.SchedulingStrategy != "REPLICA" {
		t.Errorf("unexpected SchedulingStrategy: %s", *sv.SchedulingStrategy)
	}
}
