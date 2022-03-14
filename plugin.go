package ecspresso

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/fatih/color"
	"github.com/fujiwara/cfn-lookup/cfn"
	"github.com/fujiwara/tfstate-lookup/tfstate"
)

type ConfigPlugins []ConfigPlugin

func (ps ConfigPlugins) Setup(c *Config) error {
	ps.check()
	for _, p := range ps {
		if err := p.Setup(c); err != nil {
			return err
		}
	}
	return nil
}

func (ps ConfigPlugins) check() {
	indexesByID := make(map[string][]int, len(ps))
	for i, p := range ps {
		id := strings.Join([]string{
			strings.ToLower(p.FuncPrefix),
			strings.ToLower(p.Name),
		}, ";")
		indexesByID[id] = append(indexesByID[id], i)
	}
	for _, indexes := range indexesByID {
		if len(indexes) > 1 {
			i := indexes[0]
			fmt.Fprintln(
				os.Stderr,
				color.YellowString(
					"WARNING: plugin `%s` in %v are duplicates. Please specify `func_prefix` to avoid duplication.",
					ps[i].Name, indexes,
				),
			)
		}
	}
}

type ConfigPlugin struct {
	Name       string                 `yaml:"name"`
	Config     map[string]interface{} `yaml:"config"`
	FuncPrefix string                 `yaml:"func_prefix"`
}

func (p ConfigPlugin) Setup(c *Config) error {
	switch strings.ToLower(p.Name) {
	case "tfstate":
		return setupPluginTFState(p, c)
	case "cloudformation":
		return setupPluginCFn(p, c)
	default:
		return fmt.Errorf("plugin %s is not available", p.Name)
	}
}

func (p ConfigPlugin) ModifyFuncMap(funcMap template.FuncMap) template.FuncMap {
	modified := make(template.FuncMap, len(funcMap))
	for funcName, f := range funcMap {
		modified[strings.ToLower(p.FuncPrefix)+funcName] = f
	}
	return modified
}

func setupPluginTFState(p ConfigPlugin, c *Config) error {
	var loc string
	if p.Config["path"] != nil {
		path, ok := p.Config["path"].(string)
		if !ok {
			return errors.New("tfstate plugin requires path for tfstate file as a string")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.dir, path)
		}
		loc = path
	} else if p.Config["url"] != nil {
		u, ok := p.Config["url"].(string)
		if !ok {
			return errors.New("tfstate plugin requires url for tfstate URL as a string")
		}
		loc = u
	} else {
		return errors.New("tfstate plugin requires path or url for tfstate location")
	}
	funcs, err := tfstate.FuncMap(loc)
	if err != nil {
		return err
	}
	funcs = p.ModifyFuncMap(funcs)
	c.templateFuncs = append(c.templateFuncs, funcs)
	return nil
}

func setupPluginCFn(p ConfigPlugin, c *Config) error {
	funcs, err := cfn.FuncMap(c.sess)
	if err != nil {
		return err
	}
	funcs = p.ModifyFuncMap(funcs)
	c.templateFuncs = append(c.templateFuncs, funcs)
	return nil
}
