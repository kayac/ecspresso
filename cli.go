package ecspresso

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/kayac/ecspresso/v2/skillscmd"
)

type CLIOptions struct {
	Envfile        []string          `help:"environment files" env:"ECSPRESSO_ENVFILE"`
	Debug          bool              `help:"enable debug log" env:"ECSPRESSO_DEBUG"`
	ExtStr         map[string]string `help:"external string values for Jsonnet" env:"ECSPRESSO_EXT_STR"`
	ExtCode        map[string]string `help:"external code values for Jsonnet" env:"ECSPRESSO_EXT_CODE"`
	ConfigFilePath string            `name:"config" help:"config file" default:"ecspresso.yml" env:"ECSPRESSO_CONFIG"`
	AssumeRoleARN  string            `help:"the ARN of the role to assume" default:"" env:"ECSPRESSO_ASSUME_ROLE_ARN"`
	Timeout        *time.Duration    `help:"timeout. Override in a configuration file." env:"ECSPRESSO_TIMEOUT"`
	FilterCommand  string            `help:"filter command" env:"ECSPRESSO_FILTER_COMMAND"`
	Color          bool              `help:"enable colorized output" env:"ECSPRESSO_COLOR" default:"true" negatable:""`
	LogFormat      string            `help:"log format" env:"ECSPRESSO_LOG_FORMAT" default:"text" enum:"text,json"`

	Appspec    *AppSpecOption      `cmd:"" help:"output AppSpec YAML for CodeDeploy to STDOUT"`
	Continue   *ContinueOption     `cmd:"" help:"continue a paused service deployment"`
	Delete     *DeleteOption       `cmd:"" help:"delete service"`
	Deploy     *DeployOption       `cmd:"" help:"deploy service"`
	Deregister *DeregisterOption   `cmd:"" help:"deregister task definition"`
	Diff       *DiffOption         `cmd:"" help:"show diff between task definition, service definition with current running service and task definition"`
	Docs       *DocsOption         `cmd:"" help:"show documentation for ecspresso"`
	Exec       *ExecOption         `cmd:"" help:"execute command on task"`
	Init       *InitOption         `cmd:"" help:"create configuration files from existing ECS service"`
	Refresh    *RefreshOption      `cmd:"" help:"refresh service. equivalent to deploy --skip-task-definition --force-new-deployment --no-update-service"`
	Register   *RegisterOption     `cmd:"" help:"register task definition"`
	Render     *RenderOption       `cmd:"" help:"render config, service definition or task definition file to STDOUT"`
	Revisions  *RevisionsOption    `cmd:"" help:"show revisions of task definitions"`
	Rollback   *RollbackOption     `cmd:"" help:"rollback service"`
	Run        *RunOption          `cmd:"" help:"run task"`
	Scale      *ScaleOption        `cmd:"" help:"scale service. equivalent to deploy --skip-task-definition --no-update-service"`
	Status     *StatusOption       `cmd:"" help:"show status of service"`
	Tasks      *TasksOption        `cmd:"" help:"list tasks that are in a service or having the same family"`
	Verify     *VerifyOption       `cmd:"" help:"verify resources in configurations"`
	Wait       *WaitOption         `cmd:"" help:"wait until service stable"`
	Skills     *skillscmd.Commands `cmd:"" help:"manage agent skills"`
	Version    struct{}            `cmd:"" help:"show version"`
}

func (opt *CLIOptions) resolveConfigFilePath() (path string) {
	path = DefaultConfigFilePath
	defer func() {
		opt.ConfigFilePath = path
	}()
	if opt.ConfigFilePath != "" && opt.ConfigFilePath != DefaultConfigFilePath {
		path = opt.ConfigFilePath
		return
	}
	for _, ext := range []string{ymlExt, yamlExt, jsonExt, jsonnetExt} {
		if _, err := os.Stat("ecspresso" + ext); err == nil {
			path = "ecspresso" + ext
			return
		}
	}
	return
}

func (opts *CLIOptions) forSubCommand(sub string) any {
	switch sub {
	case "appspec":
		return opts.Appspec
	case "continue":
		return opts.Continue
	case "delete":
		return opts.Delete
	case "deploy":
		return opts.Deploy
	case "deregister":
		return opts.Deregister
	case "diff":
		return opts.Diff
	case "docs":
		return opts.Docs
	case "exec":
		return opts.Exec
	case "init":
		return opts.Init
	case "refresh":
		return opts.Refresh
	case "register":
		return opts.Register
	case "render":
		return opts.Render
	case "revisions":
		return opts.Revisions
	case "rollback":
		return opts.Rollback
	case "run":
		return opts.Run
	case "scale":
		return opts.Scale
	case "status":
		return opts.Status
	case "tasks":
		return opts.Tasks
	case "verify":
		return opts.Verify
	case "wait":
		return opts.Wait
	case "skills":
		return opts.Skills
	default:
		return nil
	}
}

// dispatchCLI routes the subcommand to the appropriate handler.
// Commands that do not require AWS credentials or config are handled
// directly. All others are delegated to dispatchApp.
func dispatchCLI(ctx context.Context, sub string, usage func(), opts *CLIOptions) error {
	switch sub {
	case "version", "":
		return showVersion()
	case "docs":
		return dispatchDocs(ctx, opts.Docs)
	case "skills":
		return dispatchSkills(ctx, opts.Skills)
	default:
		return dispatchApp(ctx, sub, usage, opts)
	}
}

func showVersion() error {
	_, err := WriteOutput("ecspresso " + Version)
	return err
}

// dispatchApp handles subcommands that require an App instance
// (AWS credentials and config).
func dispatchApp(ctx context.Context, sub string, usage func(), opts *CLIOptions) error {
	var appOpts []AppOption
	if sub == "init" {
		config, err := opts.Init.NewConfig(ctx, opts.ConfigFilePath)
		if err != nil {
			return err
		}
		appOpts = append(appOpts, WithConfig(config))
	}
	app, err := New(ctx, opts, appOpts...)
	if err != nil {
		return err
	}
	app.LogDebug("dispatching subcommand: %s", sub)
	switch sub {
	case "continue":
		return app.Continue(ctx, *opts.Continue)
	case "deploy":
		return app.Deploy(ctx, *opts.Deploy)
	case "refresh":
		return app.Deploy(ctx, opts.Refresh.DeployOption())
	case "scale":
		return app.Deploy(ctx, opts.Scale.DeployOption())
	case "status":
		return app.Status(ctx, *opts.Status)
	case "rollback":
		return app.Rollback(ctx, *opts.Rollback)
	case "delete":
		return app.Delete(ctx, *opts.Delete)
	case "run":
		return app.Run(ctx, *opts.Run)
	case "wait":
		return app.Wait(ctx, *opts.Wait)
	case "register":
		return app.Register(ctx, *opts.Register)
	case "deregister":
		return app.Deregister(ctx, *opts.Deregister)
	case "revisions":
		return app.Revisions(ctx, *opts.Revisions)
	case "init":
		return app.Init(ctx, *opts.Init)
	case "diff":
		return app.Diff(ctx, *opts.Diff)
	case "appspec":
		return app.AppSpec(ctx, *opts.Appspec)
	case "verify":
		return app.Verify(ctx, *opts.Verify)
	case "render":
		return app.Render(ctx, *opts.Render)
	case "tasks":
		return app.Tasks(ctx, *opts.Tasks)
	case "exec":
		return app.Exec(ctx, *opts.Exec)
	default:
		usage()
		return nil
	}
}

func CLI(ctx context.Context, parse func([]string) (string, *CLIOptions, func(), error)) (int, error) {
	sub, opts, usage, err := parse(os.Args[1:])
	if err != nil {
		return 1, err
	}
	// set log format and level
	setLogFormat(opts.LogFormat)
	if opts.Debug {
		logLevel.Set(slog.LevelDebug)
	} else {
		logLevel.Set(slog.LevelInfo)
	}

	if err := dispatchCLI(ctx, sub, usage, opts); err != nil {
		return 1, err
	}
	return 0, nil
}
