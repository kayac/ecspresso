package ecspresso

import (
	"bufio"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v2"
)

type RenderOption struct {
	ConfigFile        *bool
	ServiceDefinition *bool
	TaskDefinition    *bool
}

func (d *App) Render(opt RenderOption) error {
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	if aws.BoolValue(opt.ConfigFile) {
		return yaml.NewEncoder(out).Encode(d.config)
	}

	if aws.BoolValue(opt.ServiceDefinition) {
		sv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
		if err != nil {
			return err
		}
		b, _ := MarshalJSONForAPI(sv)
		_, err = out.Write(b)
		return err
	}

	if aws.BoolValue(opt.TaskDefinition) {
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
