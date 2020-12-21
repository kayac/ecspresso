package ecspresso

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/kayac/ecspresso/cloudformation"
)

type ConfigPlugin struct {
	Name   string                 `yaml:"name"`
	Config map[string]interface{} `yaml:"config"`
}

func (p ConfigPlugin) Setup(c *Config) error {
	switch p.Name {
	case "tfstate":
		return setupPluginTFState(p, c)
	case "cfn_output":
		return setupPluginCFnOutput(p, c)
	default:
		return fmt.Errorf("plugin %s is not available", p.Name)
	}
}

func setupPluginTFState(p ConfigPlugin, c *Config) error {
	path, ok := p.Config["path"].(string)
	if !ok {
		return errors.New("tfstate plugin requires path for tfstate file as string")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(c.dir, path)
	}
	funcs, err := tfstate.FuncMap(path)
	if err != nil {
		return err
	}
	c.templateFuncs = append(c.templateFuncs, funcs)
	return nil
}

func setupPluginCFnOutput(p ConfigPlugin, c *Config) error {
	c.templateFuncs = append(c.templateFuncs, cloudformation.NewFuncs(c.sess))
	return nil
}
