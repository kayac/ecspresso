package ecspresso

import (
	"context"
	"fmt"
	"os"
	"time"
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

	Appspec    *AppSpecOption    `cmd:"" help:"output AppSpec YAML for CodeDeploy to STDOUT"`
	Delete     *DeleteOption     `cmd:"" help:"delete service"`
	Deploy     *DeployOption     `cmd:"" help:"deploy service"`
	Deregister *DeregisterOption `cmd:"" help:"deregister task definition"`
	Diff       *DiffOption       `cmd:"" help:"show diff between task definition, service definition with current running service and task definition"`
	Exec       *ExecOption       `cmd:"" help:"execute command on task"`
	Init       *InitOption       `cmd:"" help:"create configuration files from existing ECS service"`
	Refresh    *RefreshOption    `cmd:"" help:"refresh service. equivalent to deploy --skip-task-definition --force-new-deployment --no-update-service"`
	Register   *RegisterOption   `cmd:"" help:"register task definition"`
	Render     *RenderOption     `cmd:"" help:"render config, service definition or task definition file to STDOUT"`
	Revisions  *RevisionsOption  `cmd:"" help:"show revisions of task definitions"`
	Rollback   *RollbackOption   `cmd:"" help:"rollback service"`
	Run        *RunOption        `cmd:"" help:"run task"`
	Scale      *ScaleOption      `cmd:"" help:"scale service. equivalent to deploy --skip-task-definition --no-update-service"`
	Status     *StatusOption     `cmd:"" help:"show status of service"`
	Tasks      *TasksOption      `cmd:"" help:"list tasks that are in a service or having the same family"`
	Verify     *VerifyOption     `cmd:"" help:"verify resources in configurations"`
	Wait       *WaitOption       `cmd:"" help:"wait until service stable"`
	Version    struct{}          `cmd:"" help:"show version"`
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

func (opts *CLIOptions) ForSubCommand(sub string) interface{} {
	switch sub {
	case "appspec":
		return opts.Appspec
	case "delete":
		return opts.Delete
	case "deploy":
		return opts.Deploy
	case "deregister":
		return opts.Deregister
	case "diff":
		return opts.Diff
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
	default:
		return nil
	}
}

func dispatchCLI(ctx context.Context, sub string, usage func(), opts *CLIOptions) error {
	switch sub {
	case "version", "":
		fmt.Println("ecspresso", Version)
		return nil
	}
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
	app.Log("[DEBUG] dispatching subcommand: %s", sub)
	switch sub {
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
	case "create":
		return fmt.Errorf("create command is deprecated. use deploy command instead")
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
		return app.Revesions(ctx, *opts.Revisions)
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
	}
	return nil
}

type CLIParseFunc func([]string) (string, *CLIOptions, func(), error)

func CLI(ctx context.Context, parse CLIParseFunc) (int, error) {
	sub, opts, usage, err := parse(os.Args[1:])
	if err != nil {
		return 1, err
	}
	if err := dispatchCLI(ctx, sub, usage, opts); err != nil {
		return 1, err
	}
	return 0, nil
}
