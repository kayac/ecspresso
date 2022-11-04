package ecspresso

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/fatih/color"
	isatty "github.com/mattn/go-isatty"
)

type CLIOptions struct {
	Option *Option

	AppSpecOption    *AppSpecOption
	DeleteOption     *DeleteOption
	DeployOption     *DeployOption
	DeregisterOption *DeregisterOption
	DiffOption       *DiffOption
	ExecOption       *ExecOption
	InitOption       *InitOption
	RefreshOption    *DeployOption
	RegisterOption   *RegisterOption
	RenderOption     *RenderOption
	RevisionsOption  *RevisionsOption
	RollbackOption   *RollbackOption
	RunOption        *RunOption
	ScaleOption      *DeployOption
	StatusOption     *StatusOption
	TasksOption      *TasksOption
	VerifyOption     *VerifyOption
	WaitOption       *WaitOption
}

func (opts *CLIOptions) ForSubCommand(sub string) interface{} {
	switch sub {
	case "appspec":
		return opts.AppSpecOption
	case "delete":
		return opts.DeleteOption
	case "deploy":
		return opts.DeployOption
	case "deregister":
		return opts.DeregisterOption
	case "diff":
		return opts.DiffOption
	case "exec":
		return opts.ExecOption
	case "init":
		return opts.InitOption
	case "refresh":
		return opts.RefreshOption
	case "register":
		return opts.RegisterOption
	case "render":
		return opts.RenderOption
	case "revisions":
		return opts.RevisionsOption
	case "rollback":
		return opts.RollbackOption
	case "run":
		return opts.RunOption
	case "scale":
		return opts.ScaleOption
	case "status":
		return opts.StatusOption
	case "tasks":
		return opts.TasksOption
	case "verify":
		return opts.VerifyOption
	case "wait":
		return opts.WaitOption
	default:
		return nil
	}
}

func dispatchCLI(ctx context.Context, sub string, opts *CLIOptions) error {
	if sub == "" {
		// usage already printed
		return nil
	} else if sub == "version" {
		fmt.Println("ecspresso", Version)
		return nil
	}

	app, err := New(ctx, opts.Option)
	if err != nil {
		return err
	}

	switch sub {
	case "deploy":
		return app.Deploy(ctx, *opts.DeployOption)
	case "refresh":
		return app.Deploy(ctx, *opts.RefreshOption)
	case "scale":
		return app.Deploy(ctx, *opts.ScaleOption)
	case "status":
		return app.Status(ctx, *opts.StatusOption)
	case "rollback":
		return app.Rollback(ctx, *opts.RollbackOption)
	case "create":
		return fmt.Errorf("create command is deprecated. use deploy command instead")
	case "delete":
		return app.Delete(ctx, *opts.DeleteOption)
	case "run":
		return app.Run(ctx, *opts.RunOption)
	case "wait":
		return app.Wait(ctx, *opts.WaitOption)
	case "register":
		return app.Register(ctx, *opts.RegisterOption)
	case "deregister":
		return app.Deregister(ctx, *opts.DeregisterOption)
	case "revisions":
		return app.Revesions(ctx, *opts.RevisionsOption)
	case "init":
		return app.Init(ctx, *opts.InitOption)
	case "diff":
		return app.Diff(ctx, *opts.DiffOption)
	case "appspec":
		return app.AppSpec(ctx, *opts.AppSpecOption)
	case "verify":
		return app.Verify(ctx, *opts.VerifyOption)
	case "render":
		return app.Render(ctx, *opts.RenderOption)
	case "tasks":
		return app.Tasks(ctx, *opts.TasksOption)
	case "exec":
		return app.Exec(ctx, *opts.ExecOption)
	default:
		kingpin.Usage()
	}
	return nil
}

func CLI(ctx context.Context) (int, error) {
	sub, opts, err := ParseCLI(os.Args[1:])
	if err != nil {
		return 1, err
	}
	if err := dispatchCLI(ctx, sub, opts); err != nil {
		return 1, err
	}
	return 0, nil
}

func ParseCLI(args []string) (string, *CLIOptions, error) {
	opts := &CLIOptions{}

	kingpin.Command("version", "show version")

	configFilePath := kingpin.Flag("config", "config file").String()
	debug := kingpin.Flag("debug", "enable debug log").Bool()
	envFiles := kingpin.Flag("envfile", "environment files").Strings()
	extStr := kingpin.Flag("ext-str", "external string values for Jsonnet").StringMap()
	extCode := kingpin.Flag("ext-code", "external code values for Jsonnet").StringMap()

	colorDefault := "false"
	if isatty.IsTerminal(os.Stdout.Fd()) {
		colorDefault = "true"
	}
	colorOpt := kingpin.Flag("color", "enable colored output").Default(colorDefault).Bool()

	var isSetSuspendAutoScaling, isSetResumeAutoScaling bool
	deploy := kingpin.Command("deploy", "deploy service")
	deploy.Flag("resume-auto-scaling", "resume application auto-scaling attached with the ECS service").IsSetByUser(&isSetResumeAutoScaling).Bool()
	opts.DeployOption = &DeployOption{
		DryRun:               deploy.Flag("dry-run", "dry-run").Bool(),
		DesiredCount:         deploy.Flag("tasks", "desired count of tasks").Default("-1").Int32(),
		SkipTaskDefinition:   deploy.Flag("skip-task-definition", "skip register a new task definition").Bool(),
		ForceNewDeployment:   deploy.Flag("force-new-deployment", "force a new deployment of the service").Bool(),
		NoWait:               deploy.Flag("no-wait", "exit ecspresso immediately after just deployed without waiting for service stable").Bool(),
		SuspendAutoScaling:   deploy.Flag("suspend-auto-scaling", "suspend application auto-scaling attached with the ECS service").IsSetByUser(&isSetSuspendAutoScaling).Bool(),
		RollbackEvents:       deploy.Flag("rollback-events", " roll back when specified events happened (DEPLOYMENT_FAILURE,DEPLOYMENT_STOP_ON_ALARM,DEPLOYMENT_STOP_ON_REQUEST,...) CodeDeploy only.").String(),
		UpdateService:        deploy.Flag("update-service", "update service attributes by service definition").Default("true").Bool(),
		LatestTaskDefinition: deploy.Flag("latest-task-definition", "deploy with latest task definition without registering new task definition").Default("false").Bool(),
	}

	scale := kingpin.Command("scale", "scale service. equivalent to deploy --skip-task-definition --no-update-service")
	scale.Flag("resume-auto-scaling", "resume application auto-scaling attached with the ECS service").IsSetByUser(&isSetResumeAutoScaling).Bool()
	opts.ScaleOption = &DeployOption{
		DryRun:               scale.Flag("dry-run", "dry-run").Bool(),
		DesiredCount:         scale.Flag("tasks", "desired count of tasks").Default("-1").Int32(),
		SkipTaskDefinition:   boolp(true),
		SuspendAutoScaling:   scale.Flag("suspend-auto-scaling", "suspend application auto-scaling attached with the ECS service").IsSetByUser(&isSetSuspendAutoScaling).Bool(),
		ForceNewDeployment:   boolp(false),
		NoWait:               scale.Flag("no-wait", "exit ecspresso immediately after just deployed without waiting for service stable").Bool(),
		UpdateService:        boolp(false),
		LatestTaskDefinition: boolp(false),
	}

	refresh := kingpin.Command("refresh", "refresh service. equivalent to deploy --skip-task-definition --force-new-deployment --no-update-service")
	opts.RefreshOption = &DeployOption{
		DryRun:               refresh.Flag("dry-run", "dry-run").Bool(),
		DesiredCount:         nil,
		SkipTaskDefinition:   boolp(true),
		ForceNewDeployment:   boolp(true),
		NoWait:               refresh.Flag("no-wait", "exit ecspresso immediately after just deployed without waiting for service stable").Bool(),
		UpdateService:        boolp(false),
		LatestTaskDefinition: boolp(false),
	}

	create := kingpin.Command("create", "[DEPRECATED] use deploy command instead")
	{
		// for backward compatibility
		create.Flag("dry-run", "dry-run").Bool()
		create.Flag("tasks", "desired count of tasks").Default("-1").Int32()
		create.Flag("no-wait", "exit ecspresso immediately after just created without waiting for service stable").Bool()
	}

	status := kingpin.Command("status", "show status of service")
	opts.StatusOption = &StatusOption{
		Events: status.Flag("events", "show events num").Default("2").Int(),
	}

	rollback := kingpin.Command("rollback", "roll back a service")
	opts.RollbackOption = &RollbackOption{
		DryRun:                   rollback.Flag("dry-run", "dry-run").Bool(),
		DeregisterTaskDefinition: rollback.Flag("deregister-task-definition", "deregister a rolled-back task definition. not works with --no-wait").Default("true").Bool(),
		NoWait:                   rollback.Flag("no-wait", "exit ecspresso immediately after just rolled back without waiting for service stable").Bool(),
		RollbackEvents:           rollback.Flag("rollback-events", " roll back when specified events happened (DEPLOYMENT_FAILURE,DEPLOYMENT_STOP_ON_ALARM,DEPLOYMENT_STOP_ON_REQUEST,...) CodeDeploy only.").String(),
	}

	delete := kingpin.Command("delete", "delete service")
	opts.DeleteOption = &DeleteOption{
		DryRun: delete.Flag("dry-run", "dry-run").Bool(),
		Force:  delete.Flag("force", "delete without confirmation").Bool(),
	}

	run := kingpin.Command("run", "run task")
	opts.RunOption = &RunOption{
		DryRun:               run.Flag("dry-run", "dry-run").Bool(),
		TaskDefinition:       run.Flag("task-def", "task definition json for run task").String(),
		NoWait:               run.Flag("no-wait", "exit ecspresso after task run").Bool(),
		WaitUntil:            run.Flag("wait-until", "wait until invoked tasks status reached to (running or stopped)").Default("stopped").Enum("running", "stopped"),
		TaskOverrideStr:      run.Flag("overrides", "task overrides JSON string").Default("").String(),
		TaskOverrideFile:     run.Flag("overrides-file", "task overrides JSON file path").Default("").String(),
		SkipTaskDefinition:   run.Flag("skip-task-definition", "skip register a new task definition").Bool(),
		Count:                run.Flag("count", "the number of tasks (max 10)").Default("1").Int32(),
		WatchContainer:       run.Flag("watch-container", "the container name to watch exit code").String(),
		LatestTaskDefinition: run.Flag("latest-task-definition", "run with latest task definition without registering new task definition").Default("false").Bool(),
		PropagateTags:        run.Flag("propagate-tags", "propagate the tags for the task (SERVICE or TASK_DEFINITION)").Default("").Enum("SERVICE", "TASK_DEFINITION", ""),
		Tags:                 run.Flag("tags", "tags for the task: format is KeyFoo=ValueFoo,KeyBar=ValueBar").String(),
		Revision:             run.Flag("revision", "revision of the task definition to run when --skip-task-definition").Default("0").Int64(),
	}

	register := kingpin.Command("register", "register task definition")
	opts.RegisterOption = &RegisterOption{
		DryRun: register.Flag("dry-run", "dry-run").Bool(),
		Output: register.Flag("output", "output registered task definition").Bool(),
	}

	deregister := kingpin.Command("deregister", "deregister task definition")
	opts.DeregisterOption = &DeregisterOption{
		DryRun:   deregister.Flag("dry-run", "dry-run").Bool(),
		Revision: deregister.Flag("revision", "revision number to deregister").Int64(),
		Keeps:    deregister.Flag("keeps", "numbers of keep latest revisions except in-use").Int(),
		Force:    deregister.Flag("force", "deregister without confirmation").Bool(),
	}

	revisions := kingpin.Command("revisions", "show revisions of task definitions")
	opts.RevisionsOption = &RevisionsOption{
		Output:   revisions.Flag("output", "output format (table|json|tsv)").Default("table").Enum("table", "json", "tsv"),
		Revision: revisions.Flag("revision", "revision number to output task definition as JSON").Int64(),
	}

	_ = kingpin.Command("wait", "wait until service stable")
	opts.WaitOption = &WaitOption{}

	init := kingpin.Command("init", "create service/task definition files by existing ECS service")
	opts.InitOption = &InitOption{
		Region:                init.Flag("region", "AWS region name").Default(os.Getenv("AWS_REGION")).String(),
		Cluster:               init.Flag("cluster", "cluster name").Default("default").String(),
		Service:               init.Flag("service", "service name").Required().String(),
		TaskDefinitionPath:    init.Flag("task-definition-path", "output task definition file path").Default("ecs-task-def.json").String(),
		ServiceDefinitionPath: init.Flag("service-definition-path", "output service definition file path").Default("ecs-service-def.json").String(),
		ForceOverwrite:        init.Flag("force-overwrite", "force overwrite files").Bool(),
		Jsonnet:               init.Flag("jsonnet", "format as jsonnet to generate definition files").Bool(),
		ConfigFilePath:        configFilePath,
	}

	diff := kingpin.Command("diff", "display diff for task definition compared with latest one on ECS")
	opts.DiffOption = &DiffOption{
		Unified: diff.Flag("unified", "display diff in unified format").Default("t").Bool(),
	}

	appspec := kingpin.Command("appspec", "output AppSpec YAML for CodeDeploy to STDOUT")
	opts.AppSpecOption = &AppSpecOption{
		TaskDefinition: appspec.Flag("task-definition", "use task definition arn in AppSpec (latest, current or Arn)").Default("latest").String(),
		UpdateService:  appspec.Flag("update-service", "update service attributes by service definition").Default("true").Bool(),
	}

	verify := kingpin.Command("verify", "verify resources in configurations")
	opts.VerifyOption = &VerifyOption{
		GetSecrets: verify.Flag("get-secrets", "get secrets from ParameterStore or SecretsManager").Default("true").Bool(),
		PutLogs:    verify.Flag("put-logs", "put verification logs to CloudWatch Logs").Default("true").Bool(),
	}

	render := kingpin.Command("render", "render config, service definition or task definition file to stdout")
	opts.RenderOption = &RenderOption{
		Targets: render.Arg("targets", "render targets (config, servicedef, taskdef)").Required().Enums(
			"config",
			"servicedef", "service-definition",
			"taskdef", "task-definition",
		),
	}

	tasks := kingpin.Command("tasks", "list tasks that are in a service or having the same family")
	opts.TasksOption = &TasksOption{
		ID:     tasks.Flag("id", "task ID").Default("").String(),
		Output: tasks.Flag("output", "output format (table|json|tsv)").Default("table").Enum("table", "json", "tsv"),
		Find:   tasks.Flag("find", "find a task from tasks list and dump it as JSON").Bool(),
		Stop:   tasks.Flag("stop", "stop a task").Bool(),
		Force:  tasks.Flag("force", "stop a task without confirmation").Bool(),
		Trace:  tasks.Flag("trace", "trace a task").Bool(),
	}

	exec := kingpin.Command("exec", "execute command in a task")
	opts.ExecOption = &ExecOption{
		ID:          exec.Flag("id", "task ID").Default("").String(),
		Command:     exec.Flag("command", "command").Default("sh").String(),
		Container:   exec.Flag("container", "container name").String(),
		LocalPort:   exec.Flag("local-port", "local port number").Default("0").Int(),
		Port:        exec.Flag("port", "remote port number (required for --port-forward)").Default("0").Int(),
		PortForward: exec.Flag("port-forward", "enable port forward").Default("false").Bool(),
	}

	sub := kingpin.MustParse(kingpin.CommandLine.Parse(args))
	if sub == "" {
		kingpin.Usage()
		return "", nil, nil
	}

	color.NoColor = !*colorOpt
	for _, envFile := range *envFiles {
		if err := ExportEnvFile(envFile); err != nil {
			return sub, opts, fmt.Errorf("failed to load envfile: %w", err)
		}
	}

	opts.Option = &Option{
		ConfigFilePath: *configFilePath,
		Debug:          *debug,
		ExtStr:         *extStr,
		ExtCode:        *extCode,
	}
	if sub == "init" {
		opts.Option.InitOption = opts.InitOption
	}

	if !isSetSuspendAutoScaling {
		opts.DeployOption.SuspendAutoScaling = nil
		opts.ScaleOption.SuspendAutoScaling = nil
	}
	if isSetResumeAutoScaling {
		opts.DeployOption.SuspendAutoScaling = boolp(false)
		opts.ScaleOption.SuspendAutoScaling = boolp(false)
	}

	return sub, opts, nil
}

func boolp(b bool) *bool {
	return &b
}
