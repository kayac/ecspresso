package ecspresso

import "fmt"

type ConfigPlugin struct {
	Name   string `yaml:"name"`
	Config map[string]interface{}
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
