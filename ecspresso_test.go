package ecspresso_test

import (
	"testing"
	"time"

	"github.com/kayac/ecspresso"
)

func TestLoadTaskDefinition(t *testing.T) {
	for _, path := range []string{"tests/td.json", "tests/td-plain.json"} {
		c := &ecspresso.Config{
			Region:             "ap-northeast-1",
			Timeout:            600 * time.Second,
			Service:            "test",
			Cluster:            "default",
			TaskDefinitionPath: path,
		}
		if err := c.Restrict(); err != nil {
			t.Error(err)
		}
		app, err := ecspresso.NewApp(c)
		if err != nil {
			t.Error(err)
		}
		td, err := app.LoadTaskDefinition(path)
		if err != nil || td == nil {
			t.Errorf("%s load failed: %s", path, err)
		}
	}
}
