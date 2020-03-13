package ecspresso_test

import (
	"testing"
	"time"

	"github.com/kayac/ecspresso"
)

func TestLoadTaskDefinition(t *testing.T) {
	for _, path := range []string{"tests/td.json", "tests/td-plain.json"} {
		td := &ecspresso.ConfigTaskDefinition{}
		td.Path = path
		c := &ecspresso.Config{
			Region:         "ap-northeast-1",
			Timeout:        600 * time.Second,
			Service:        "test",
			Cluster:        "default",
			TaskDefinition: td,
		}
		app, err := ecspresso.NewApp(c)
		if err != nil {
			t.Error(err)
		}
		loadTd, err := app.LoadTaskDefinition(td.Path)
		if err != nil || loadTd == nil {
			t.Errorf("%s load failed: %s", path, err)
		}
	}
}
