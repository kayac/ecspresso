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

	var opts CLIOptions
	parser, err := kong.New(&opts, kong.Vars{"version": Version})
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to new kong: %w", err)
	}
	c, err := parser.Parse(args)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to parse args: %w", err)
	}
	sub := strings.Fields(c.Command())[0]

	for _, envFile := range opts.Envfile {
		if err := ExportEnvFile(envFile); err != nil {
			return sub, &opts, nil, fmt.Errorf("failed to load envfile: %w", err)
		}
	}

	if opts.ExtStr == nil {
		opts.ExtStr = map[string]string{}
	}
	if opts.ExtCode == nil {
		opts.ExtCode = map[string]string{}
	}
	return sub, &opts, func() { c.PrintUsage(true) }, nil
}
