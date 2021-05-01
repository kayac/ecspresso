package ecspresso_test

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/kayac/ecspresso"
)

func TestLoadTaskDefinition(t *testing.T) {
	for _, path := range []string{"tests/td.json", "tests/td-plain.json", "tests/td-in-tags.json", "tests/td-plain-in-tags.json"} {
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
		if s := aws.Int64Value(td.EphemeralStorage.SizeInGiB); s != 25 {
			t.Errorf("EphemeralStorage.SizeInGiB expected %d got %d", 25, s)
		}
	}
}

func TestLoadTaskDefinitionTags(t *testing.T) {
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
		_, err = app.LoadTaskDefinition(path)
		if err != nil {
			t.Errorf("%s load failed: %s", path, err)
		}
	}

	for _, path := range []string{"tests/td-in-tags.json", "tests/td-plain-in-tags.json"} {
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
		if err != nil || len(td.Tags) == 0 {
			t.Errorf("%s load failed: %s", path, err)
		}
	}
}
