package ecspresso

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/pkg/errors"
)

type ConfigPluginTFState struct {
	Path string `yaml:"path"`
}

func (p ConfigPluginTFState) Enabled() bool {
	return p.Path != ""
}

func (p ConfigPluginTFState) Setup(c *Config) error {
	funcs, err := tfstatePluginFuncs(p.Path)
	if err != nil {
		return err
	}
	c.templateFuncs = append(c.templateFuncs, funcs)
	return nil
}

func tfstatePluginFuncs(path string) (template.FuncMap, error) {
	f, err := os.Open(path)
	if err != nil {
		errors.Wrapf(err, "failed to read tfstate: %s", path)
	}
	state, err := tfstate.Read(f)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read tfstate: %s", path)
	}
	return template.FuncMap{
		"tfstate": func(addrs string) string {
			if strings.Contains(addrs, "'") {
				addrs = strings.ReplaceAll(addrs, "'", "\"")
			}
			attrs, err := state.Lookup(addrs)
			if err != nil {
				return ""
			}
			if attrs.Value == nil {
				panic(fmt.Sprintf("%s is not found in tfstate", addrs))
			}
			return attrs.String()
		},
	}, nil
}
