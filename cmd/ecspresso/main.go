package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/alecthomas/kingpin"
	"github.com/fatih/color"
	"github.com/kayac/ecspresso"
	"github.com/mattn/go-isatty"
)

var Version = "current"

func main() {
	os.Exit(_main())
}

func _main() int {
	kingpin.Command("version", "show version")

	conf := kingpin.Flag("config", "config file").Default("ecspresso.yml").String()
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
	deployOption := ecspresso.DeployOption{
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
	scaleOption := ecspresso.DeployOption{
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
	refreshOption := ecspresso.DeployOption{
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
	statusOption := ecspresso.StatusOption{
		Events: status.Flag("events", "show events num").Default("2").Int(),
	}

	rollback := kingpin.Command("rollback", "roll back a service")
	rollbackOption := ecspresso.RollbackOption{
		DryRun:                   rollback.Flag("dry-run", "dry-run").Bool(),
		DeregisterTaskDefinition: rollback.Flag("deregister-task-definition", "deregister a rolled-back task definition. not works with --no-wait").Default("true").Bool(),
		NoWait:                   rollback.Flag("no-wait", "exit ecspresso immediately after just rolled back without waiting for service stable").Bool(),
		RollbackEvents:           rollback.Flag("rollback-events", " roll back when specified events happened (DEPLOYMENT_FAILURE,DEPLOYMENT_STOP_ON_ALARM,DEPLOYMENT_STOP_ON_REQUEST,...) CodeDeploy only.").String(),
	}

	delete := kingpin.Command("delete", "delete service")
	deleteOption := ecspresso.DeleteOption{
		DryRun: delete.Flag("dry-run", "dry-run").Bool(),
		Force:  delete.Flag("force", "delete without confirmation").Bool(),
	}

	run := kingpin.Command("run", "run task")
	runOption := ecspresso.RunOption{
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
	registerOption := ecspresso.RegisterOption{
		DryRun: register.Flag("dry-run", "dry-run").Bool(),
		Output: register.Flag("output", "output registered task definition").Bool(),
	}

	deregister := kingpin.Command("deregister", "deregister task definition")
	deregisterOption := ecspresso.DeregisterOption{
		DryRun:   deregister.Flag("dry-run", "dry-run").Bool(),
		Revision: deregister.Flag("revision", "revision number to deregister").Int64(),
		Keeps:    deregister.Flag("keeps", "numbers of keep latest revisions except in-use").Int(),
		Force:    deregister.Flag("force", "deregister without confirmation").Bool(),
	}

	revisions := kingpin.Command("revisions", "show revisions of task definitions")
	revisionsOption := ecspresso.RevisionsOption{
		Output:   revisions.Flag("output", "output format (table|json|tsv)").Default("table").Enum("table", "json", "tsv"),
		Revision: revisions.Flag("revision", "revision number to output task definition as JSON").Int64(),
	}

	_ = kingpin.Command("wait", "wait until service stable")
	waitOption := ecspresso.WaitOption{}

	init := kingpin.Command("init", "create service/task definition files by existing ECS service")
	initOption := ecspresso.InitOption{
		Region:                init.Flag("region", "AWS region name").Default(os.Getenv("AWS_REGION")).String(),
		Cluster:               init.Flag("cluster", "cluster name").Default("default").String(),
		Service:               init.Flag("service", "service name").Required().String(),
		TaskDefinitionPath:    init.Flag("task-definition-path", "output task definition file path").Default("ecs-task-def.json").String(),
		ServiceDefinitionPath: init.Flag("service-definition-path", "output service definition file path").Default("ecs-service-def.json").String(),
		ForceOverwrite:        init.Flag("force-overwrite", "force overwrite files").Bool(),
		Jsonnet:               init.Flag("jsonnet", "format as jsonnet to generate definition files").Bool(),
	}

	diff := kingpin.Command("diff", "display diff for task definition compared with latest one on ECS")
	diffOption := ecspresso.DiffOption{
		Unified: diff.Flag("unified", "display diff in unified format").Default("t").Bool(),
	}

	appspec := kingpin.Command("appspec", "output AppSpec YAML for CodeDeploy to STDOUT")
	appspecOption := ecspresso.AppSpecOption{
		TaskDefinition: appspec.Flag("task-definition", "use task definition arn in AppSpec (latest, current or Arn)").Default("latest").String(),
		UpdateService:  appspec.Flag("update-service", "update service attributes by service definition").Default("true").Bool(),
	}

	verify := kingpin.Command("verify", "verify resources in configurations")
	verifyOption := ecspresso.VerifyOption{
		GetSecrets: verify.Flag("get-secrets", "get secrets from ParameterStore or SecretsManager").Default("true").Bool(),
		PutLogs:    verify.Flag("put-logs", "put verification logs to CloudWatch Logs").Default("true").Bool(),
	}

	render := kingpin.Command("render", "render config, service definition or task definition file to stdout")
	renderOption := ecspresso.RenderOption{
		Targets: render.Arg("targets", "render targets").Required().Enums(
			"config",
			"servicedef", "service-definition",
			"taskdef", "task-definition",
		),
	}

	tasks := kingpin.Command("tasks", "list tasks that are in a service or having the same family")
	tasksOption := ecspresso.TasksOption{
		ID:     tasks.Flag("id", "task ID").Default("").String(),
		Output: tasks.Flag("output", "output format (table|json|tsv)").Default("table").Enum("table", "json", "tsv"),
		Find:   tasks.Flag("find", "find a task from tasks list and dump it as JSON").Bool(),
		Stop:   tasks.Flag("stop", "stop a task").Bool(),
		Force:  tasks.Flag("force", "stop a task without confirmation").Bool(),
		Trace:  tasks.Flag("trace", "trace a task").Bool(),
	}

	exec := kingpin.Command("exec", "execute command in a task")
	execOption := ecspresso.ExecOption{
		ID:          exec.Flag("id", "task ID").Default("").String(),
		Command:     exec.Flag("command", "command").Default("sh").String(),
		Container:   exec.Flag("container", "container name").String(),
		LocalPort:   exec.Flag("local-port", "local port number").Default("0").Int(),
		Port:        exec.Flag("port", "remote port number (required for --port-forward)").Default("0").Int(),
		PortForward: exec.Flag("port-forward", "enable port forward").Default("false").Bool(),
	}

	sub := kingpin.Parse()
	if sub == "version" {
		fmt.Println("ecspresso", Version)
		return 0
	}

	color.NoColor = !*colorOpt
	for _, envFile := range *envFiles {
		if err := ecspresso.ExportEnvFile(envFile); err != nil {
			ecspresso.Log("[ERROR] Failed to load envfile: %s", err)
			return 1
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), trapSignals...)
	defer stop()

	c := ecspresso.NewDefaultConfig()
	if sub == "init" {
		c.Region = *initOption.Region
		c.Cluster = *initOption.Cluster
		c.Service = *initOption.Service
		c.TaskDefinitionPath = *initOption.TaskDefinitionPath
		c.ServiceDefinitionPath = *initOption.ServiceDefinitionPath
		initOption.ConfigFilePath = conf
		if err := c.Restrict(ctx); err != nil {
			ecspresso.Log("[ERROR] Could not init config: %s", err)
			return 1
		}
	} else {
		if err := c.Load(ctx, *conf); err != nil {
			ecspresso.Log("[ERROR] Could not load config file %s: %s", *conf, err)
			kingpin.Usage()
			return 1
		}
		if err := c.ValidateVersion(Version); err != nil {
			ecspresso.Log("[ERROR] %s", err.Error())
			return 1
		}
	}

	opt := &ecspresso.Option{
		Debug:   *debug,
		ExtStr:  *extStr,
		ExtCode: *extCode,
	}
	app, err := ecspresso.New(c, opt)
	if err != nil {
		ecspresso.Log("[ERROR] %s", err)
		return 1
	}

	switch sub {
	case "deploy":
		if !isSetSuspendAutoScaling {
			deployOption.SuspendAutoScaling = nil
		}
		if isSetResumeAutoScaling {
			deployOption.SuspendAutoScaling = boolp(false)
		}
		err = app.Deploy(ctx, deployOption)
	case "refresh":
		err = app.Deploy(ctx, refreshOption)
	case "scale":
		if !isSetSuspendAutoScaling {
			scaleOption.SuspendAutoScaling = nil
		}
		if isSetResumeAutoScaling {
			scaleOption.SuspendAutoScaling = boolp(false)
		}
		err = app.Deploy(ctx, scaleOption)
	case "status":
		err = app.Status(ctx, statusOption)
	case "rollback":
		err = app.Rollback(ctx, rollbackOption)
	case "create":
		err = fmt.Errorf("create command is deprecated. use deploy command instead")
	case "delete":
		err = app.Delete(ctx, deleteOption)
	case "run":
		err = app.Run(ctx, runOption)
	case "wait":
		err = app.Wait(ctx, waitOption)
	case "register":
		err = app.Register(ctx, registerOption)
	case "deregister":
		err = app.Deregister(ctx, deregisterOption)
	case "revisions":
		err = app.Revesions(ctx, revisionsOption)
	case "init":
		err = app.Init(ctx, initOption)
	case "diff":
		err = app.Diff(ctx, diffOption)
	case "appspec":
		err = app.AppSpec(ctx, appspecOption)
	case "verify":
		err = app.Verify(ctx, verifyOption)
	case "render":
		err = app.Render(ctx, renderOption)
	case "tasks":
		err = app.Tasks(ctx, tasksOption)
	case "exec":
		err = app.Exec(ctx, execOption)
	default:
		kingpin.Usage()
		return 1
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			ecspresso.Log("[WARNING] Interrupted")
		} else {
			ecspresso.Log("[ERROR] %s FAILED. %s", sub, err)
		}
		return 1
	}

	return 0
}

func boolp(b bool) *bool {
	return &b
}
