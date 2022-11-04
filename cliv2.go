package ecspresso

import (
	"fmt"

	"github.com/alecthomas/kong"
)

func ParseCLIv2(args []string) (string, *CLIOptions, error) {
	var opts CLIOptions
	parser, err := kong.New(&opts, kong.Vars{"version": Version})
	if err != nil {
		return "", nil, err
	}
	c, err := parser.Parse(args)
	if err != nil {
		parser.FatalIfErrorf(err)
		return "", nil, err
	}
	sub := c.Command()

	for _, envFile := range opts.Envfile {
		if err := ExportEnvFile(envFile); err != nil {
			return sub, &opts, fmt.Errorf("failed to load envfile: %w", err)
		}
	}

	opts.Option = &Option{
		ConfigFilePath: opts.Config,
		Debug:          opts.Debug,
		ExtStr:         opts.ExtStr,
		ExtCode:        opts.ExtCode,
	}
	if sub == "init" {
		opts.Option.InitOption = opts.Init
	}
	return sub, &opts, nil
}
