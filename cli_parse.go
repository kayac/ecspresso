package ecspresso

import (
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
)

func ParseCLI(args []string) (string, *CLIOptions, func(), error) {
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

	// Kong allocates all cmd:"" pointer fields during parsing.
	// Clear inactive subcommand pointers so nil checks work for dispatch.
	clearInactiveSubcommands(&opts, c.Command())

	for _, envFile := range opts.Envfile {
		if err := exportEnvFile(envFile); err != nil {
			return sub, &opts, nil, fmt.Errorf("failed to load envfile: %w", err)
		}
	}

	if opts.ExtStr == nil {
		opts.ExtStr = map[string]string{}
	}
	if opts.ExtCode == nil {
		opts.ExtCode = map[string]string{}
	}
	color.NoColor = !opts.Color
	return sub, &opts, func() { c.PrintUsage(true) }, nil
}

func clearInactiveSubcommands(opts *CLIOptions, cmd string) {
	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return
	}
	primary, subcmd := fields[0], fields[1]

	switch primary {
	case "tasks":
		t := opts.Tasks
		if t == nil {
			return
		}
		if subcmd != "list" {
			t.List = nil
		}
		if subcmd != "find" {
			t.Find = nil
		}
		if subcmd != "stop" {
			t.Stop = nil
		}
		if subcmd != "trace" {
			t.Trace = nil
		}
		if subcmd != "logs" {
			t.Logs = nil
		}
	case "exec":
		e := opts.Exec
		if e == nil {
			return
		}
		if subcmd != "run" {
			e.Run = nil
		}
		if subcmd != "portforward" {
			e.Portforward = nil
		}
		if subcmd != "cp" {
			e.Cp = nil
		}
	case "skills":
		s := opts.Skills
		if s == nil {
			return
		}
		if subcmd != "list" {
			s.List = nil
		}
		if subcmd != "install" {
			s.Install = nil
		}
		if subcmd != "update" {
			s.Update = nil
		}
		if subcmd != "reinstall" {
			s.Reinstall = nil
		}
		if subcmd != "uninstall" {
			s.Uninstall = nil
		}
		if subcmd != "status" {
			s.Status = nil
		}
	}
}
