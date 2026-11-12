package ecspresso_test

import (
	"testing"

	"github.com/kayac/ecspresso/v3"
)

func TestLoadExpressDefinition(t *testing.T) {
	ctx := t.Context()
	for _, path := range []string{
		"tests/ecs-express-def.json",
		"tests/ecs-express-def.jsonnet",
	} {
		app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{
			ConfigFilePath: "tests/express-config.jsonnet",
		})
		if err != nil {
			t.Error(err)
		}
		ex, err := app.LoadExpressDefinition(path)
		if err != nil || ex == nil {
			t.Errorf("%s load failed: %s", path, err)
		}
	}
}
