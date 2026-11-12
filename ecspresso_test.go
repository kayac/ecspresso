package ecspresso_test

import (
	"encoding/json"
	"testing"

	"github.com/kayac/ecspresso/v2"
)

func str[T any](v T) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestLoadTaskDefinition(t *testing.T) {
	ctx := t.Context()
	for _, path := range []string{
		"tests/td.json",
		"tests/td-plain.json",
		"tests/td-in-tags.json",
		"tests/td-plain-in-tags.json",
		"tests/td.jsonnet",
	} {
		app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{
			ConfigFilePath: "tests/td-config.yml",
			ExtStr:         map[string]string{"WorkerID": "3"},
			ExtCode:        map[string]string{"EphemeralStorage": "24 + 1"}, // == 25
		})
		if err != nil {
			t.Error(err)
		}

		td, err := app.LoadTaskDefinition(path)
		if err != nil || td == nil {
			t.Errorf("%s load failed: %s", path, err)
		}
		if s := td.EphemeralStorage.SizeInGiB; s != 25 {
			t.Errorf("EphemeralStorage.SizeInGiB expected %d got %d", 25, s)
		}
		if td.ContainerDefinitions[0].DockerLabels["name"] != "katsubushi" {
			t.Errorf("unexpected DockerLabels unexpected got %v", td.ContainerDefinitions[0].DockerLabels)
		}
		if td.ContainerDefinitions[0].LogConfiguration.Options["awslogs-group"] != "fargate" {
			t.Errorf("unexpected LogConfiguration.Options got %v", td.ContainerDefinitions[0].LogConfiguration.Options)
		}
	}
}

func TestLoadTaskDefinitionTags(t *testing.T) {
	ctx := t.Context()
	for _, path := range []string{"tests/td.json", "tests/td-plain.json", "tests/td.jsonnet"} {
		app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{
			ConfigFilePath: "tests/td-config.yml",
			ExtStr:         map[string]string{"WorkerID": "3"},
			ExtCode:        map[string]string{"EphemeralStorage": "24 + 1"}, // == 25
		})
		if err != nil {
			t.Error(err)
		}
		td, err := app.LoadTaskDefinition(path)
		if err != nil {
			t.Errorf("%s load failed: %s", path, err)
		}
		if td.Tags != nil {
			t.Errorf("%s tags must be null %v", path, td.Tags)
		}
	}

	for _, path := range []string{"tests/td-in-tags.json", "tests/td-plain-in-tags.json"} {
		app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{ConfigFilePath: "tests/td-config.yml"})
		if err != nil {
			t.Error(err)
		}
		td, err := app.LoadTaskDefinition(path)
		if err != nil || len(td.Tags) == 0 {
			t.Errorf("%s load failed: %s", path, err)
		}
	}
}
