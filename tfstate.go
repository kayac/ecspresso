package ecspresso

import (
	"os"
	"text/template"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/pkg/errors"
)

func NewTFStatePluginFuncs(path string) (template.FuncMap, error) {
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
			attrs, err := state.Lookup(addrs)
			if err != nil {
				return ""
			}
			return attrs.String()
		},
	}, nil
}
