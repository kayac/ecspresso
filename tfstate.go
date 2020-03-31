package ecspresso

import (
	"errors"

	"github.com/kayac/go-config/tfstate"
)

func setupPluginTFState(p ConfigPlugin, c *Config) error {
	path, ok := p.Config["path"].(string)
	if !ok {
		return errors.New("tfstate plugin requires path for tfstate file as string")
	}
	funcs, err := tfstate.Load(path)
	if err != nil {
		return err
	}
	c.templateFuncs = append(c.templateFuncs, funcs)
	return nil
}
