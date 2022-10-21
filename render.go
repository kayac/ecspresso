package ecspresso

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

type RenderOption struct {
	Targets *[]string
}

func (d *App) Render(ctx context.Context, opt RenderOption) error {
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	for _, target := range *opt.Targets {
		switch target {
		case "config":
			if err := yaml.NewEncoder(out).Encode(d.config); err != nil {
				return err
			}
		case "service-definition", "servicedef":
			sv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
			if err != nil {
				return err
			}
			b, _ := MarshalJSONForAPI(sv)
			if _, err = out.Write(b); err != nil {
				return err
			}
		case "task-definition", "taskdef":
			td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
			if err != nil {
				return err
			}
			b, _ := MarshalJSONForAPI(td)
			if _, err := out.Write(b); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown target: %s", target)
		}
	}
	return nil
}
