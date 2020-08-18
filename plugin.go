package ecspresso

import (
	"errors"
	"fmt"

	"github.com/fujiwara/tfstate-lookup/tfstate"
)

type ConfigPlugin struct {
	Name   string                 `yaml:"name"`
	Config map[string]interface{} `yaml:"config"`
}

func (p ConfigPlugin) Setup(c *Config) error {
	switch p.Name {
	case "tfstate":
		return setupPluginTFState(p, c)
	default:
		return fmt.Errorf("plugin %s is not available", p.Name)
	}
	return nil
}

func setupPluginTFState(p ConfigPlugin, c *Config) error {
	path, ok := p.Config["path"].(string)
	if !ok {
		return errors.New("tfstate plugin requires path for tfstate file as string")
	}
	funcs, err := tfstate.FuncMap(path)
	if err != nil {
		return err
	}
	c.templateFuncs = append(c.templateFuncs, funcs)
	return nil
}
