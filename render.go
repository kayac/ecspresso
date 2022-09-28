package ecspresso

import (
	"bufio"
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"gopkg.in/yaml.v2"
)

type RenderOption struct {
	ConfigFile        *bool
	ServiceDefinition *bool
	TaskDefinition    *bool
}

func (d *App) Render(ctx context.Context, opt RenderOption) error {
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	if aws.ToBool(opt.ConfigFile) {
		return yaml.NewEncoder(out).Encode(d.config)
	}

	if aws.ToBool(opt.ServiceDefinition) {
		sv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
		if err != nil {
			return err
		}
		b, _ := MarshalJSONForAPI(sv)
		_, err = out.Write(b)
		return err
	}

	if aws.ToBool(opt.TaskDefinition) {
		td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
		if err != nil {
			return err
		}
		b, _ := MarshalJSONForAPI(td)
		_, err = out.Write(b)
		return err
	}

	return nil
}
