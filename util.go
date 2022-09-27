package ecspresso

import (
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/google/go-jsonnet"
)

func isLongArnFormat(a string) (bool, error) {
	an, err := arn.Parse(a)
	if err != nil {
		return false, err
	}
	rs := strings.Split(an.Resource, "/")
	switch rs[0] {
	case "container-instance", "service", "task":
		return len(rs) >= 3, nil
	default:
		return false, nil
	}
}

func (d *App) readDefinitionFile(path string) ([]byte, error) {
	switch filepath.Ext(path) {
	case jsonnetExt:
		vm := jsonnet.MakeVM()
		for k, v := range d.option.ExtStr {
			vm.ExtVar(k, v)
		}
		for k, v := range d.option.ExtCode {
			vm.ExtCode(k, v)
		}
		jsonStr, err := vm.EvaluateFile(path)
		if err != nil {
			return nil, err
		}
		return d.loader.ReadWithEnvBytes([]byte(jsonStr))
	}
	return d.loader.ReadWithEnv(path)
}
