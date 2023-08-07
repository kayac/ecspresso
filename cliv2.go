package ecspresso

import (
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
)

func ParseCLIv2(args []string) (string, *CLIOptions, func(), error) {
	// compatible with v1
	if len(args) == 0 || len(args) > 0 && args[0] == "help" {
		args = []string{"--help"}
	}

	opts := CLIOptions{}
	opts.ConfigOverrides = &ConfigOverrides{}

	parser, err := kong.New(&opts, kong.Vars{"version": Version})
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to new kong: %w", err)
	}
	c, err := parser.Parse(args)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to parse args: %w", err)
	}
	sub := strings.Fields(c.Command())[0]

	for _, envFile := range opts.Option.Envfile {
		if err := ExportEnvFile(envFile); err != nil {
			return sub, &opts, nil, fmt.Errorf("failed to load envfile: %w", err)
		}
	}
	if opts.Option.ExtStr == nil {
		opts.Option.ExtStr = map[string]string{}
	}
	if opts.Option.ExtCode == nil {
		opts.Option.ExtCode = map[string]string{}
	}
	switch sub {
	case "init":
		opts.Init.ConfigFilePath = opts.ConfigFilePath
		opts.Option.InitOption = opts.Init
	}
	return sub, &opts, func() { c.PrintUsage(true) }, nil
}
