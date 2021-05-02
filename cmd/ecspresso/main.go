package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/aws/aws-sdk-go/aws"
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

	conf := kingpin.Flag("config", "config file").String()
	debug := kingpin.Flag("debug", "enable debug log").Bool()
	envFiles := kingpin.Flag("envfile", "environment files").Strings()

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
		DesiredCount:         deploy.Flag("tasks", "desired count of tasks").Default("-1").Int64(),
		SkipTaskDefinition:   deploy.Flag("skip-task-definition", "skip register a new task definition").Bool(),
		ForceNewDeployment:   deploy.Flag("force-new-deployment", "force a new deployment of the service").Bool(),
		NoWait:               deploy.Flag("no-wait", "exit ecspresso immediately after just deployed without waiting for service stable").Bool(),
		SuspendAutoScaling:   deploy.Flag("suspend-auto-scaling", "suspend application auto-scaling attached with the ECS service").IsSetByUser(&isSetSuspendAutoScaling).Bool(),
		RollbackEvents:       deploy.Flag("rollback-events", " rollback when specified events happened (DEPLOYMENT_FAILURE,DEPLOYMENT_STOP_ON_ALARM,DEPLOYMENT_STOP_ON_REQUEST,...) CodeDeploy only.").String(),
		UpdateService:        deploy.Flag("update-service", "update service attributes by service definition").Default("true").Bool(),
		LatestTaskDefinition: deploy.Flag("latest-task-definition", "deploy with latest task definition without registering new task definition").Default("false").Bool(),
	}

	scale := kingpin.Command("scale", "scale service. equivalent to deploy --skip-task-definition --no-update-service")
	scale.Flag("resume-auto-scaling", "resume application auto-scaling attached with the ECS service").IsSetByUser(&isSetResumeAutoScaling).Bool()
	scaleOption := ecspresso.DeployOption{
		DryRun:               scale.Flag("dry-run", "dry-run").Bool(),
		DesiredCount:         scale.Flag("tasks", "desired count of tasks").Default("-1").Int64(),
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

	create := kingpin.Command("create", "create service")
	createOption := ecspresso.CreateOption{
		DryRun:       create.Flag("dry-run", "dry-run").Bool(),
		DesiredCount: create.Flag("tasks", "desired count of tasks").Default("-1").Int64(),
		NoWait:       create.Flag("no-wait", "exit ecspresso immediately after just created without waiting for service stable").Bool(),
	}

	status := kingpin.Command("status", "show status of service")
	statusOption := ecspresso.StatusOption{
		Events: status.Flag("events", "show events num").Default("2").Int(),
	}

	rollback := kingpin.Command("rollback", "rollback service")
	rollbackOption := ecspresso.RollbackOption{
		DryRun:                   rollback.Flag("dry-run", "dry-run").Bool(),
		DeregisterTaskDefinition: rollback.Flag("deregister-task-definition", "deregister rolled back task definition").Bool(),
		NoWait:                   rollback.Flag("no-wait", "exit ecspresso immediately after just rollbacked without waiting for service stable").Bool(),
		RollbackEvents:           rollback.Flag("rollback-events", " rollback when specified events happened (DEPLOYMENT_FAILURE,DEPLOYMENT_STOP_ON_ALARM,DEPLOYMENT_STOP_ON_REQUEST,...) CodeDeploy only.").String(),
	}

	delete := kingpin.Command("delete", "delete service")
	deleteOption := ecspresso.DeleteOption{
		DryRun: delete.Flag("dry-run", "dry-run").Bool(),
		Force:  delete.Flag("force", "force delete. not confirm").Bool(),
	}

	run := kingpin.Command("run", "run task")
	runOption := ecspresso.RunOption{
		DryRun:               run.Flag("dry-run", "dry-run").Bool(),
		TaskDefinition:       run.Flag("task-def", "task definition json for run task").String(),
		NoWait:               run.Flag("no-wait", "exit ecspresso after task run").Bool(),
		TaskOverrideStr:      run.Flag("overrides", "task overrides JSON string").Default("").String(),
		TaskOverrideFile:     run.Flag("overrides-file", "task overrides JSON file path").Default("").String(),
		SkipTaskDefinition:   run.Flag("skip-task-definition", "skip register a new task definition").Bool(),
		Count:                run.Flag("count", "the number of tasks (max 10)").Default("1").Int64(),
		WatchContainer:       run.Flag("watch-container", "the container name to watch exit code").String(),
		LatestTaskDefinition: run.Flag("latest-task-definition", "run with latest task definition without registering new task definition").Default("false").Bool(),
		PropagateTags:        run.Flag("propagate-tags", "propagate the tags for the task (SERVICE or TASK_DEFINITION)").Default("").Enum("SERVICE", "TASK_DEFINITION", ""),
		Tags:                 run.Flag("tags", "tags for the task: format is KeyFoo=ValueFoo,KeyBar=ValueBar").String(),
	}

	register := kingpin.Command("register", "register task definition")
	registerOption := ecspresso.RegisterOption{
		DryRun: register.Flag("dry-run", "dry-run").Bool(),
		Output: register.Flag("output", "output registered task definition").Bool(),
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
	}

	_ = kingpin.Command("diff", "display diff for task definition compared with latest one on ECS")
	diffOption := ecspresso.DiffOption{}

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
		ServiceDefinition: render.Flag("service-definition", "render service definition").Bool(),
		TaskDefinition:    render.Flag("task-definition", "render task definition").Bool(),
		ConfigFile:        render.Flag("config-file", "render config file").Bool(),
	}

	tasks := kingpin.Command("tasks", "list tasks that are in a service or having the same family")
	tasksOption := ecspresso.TasksOption{
		ID:     tasks.Flag("id", "task ID").Default("").String(),
		Output: tasks.Flag("output", "output format (table|json|tsv)").Default("table").Enum("table", "json", "tsv"),
		Find:   tasks.Flag("find", "find a task from tasks list and dump it as JSON").Bool(),
		Stop:   tasks.Flag("stop", "stop a task").Bool(),
		Force:  tasks.Flag("force", "stop a task without confirmation prompt").Bool(),
	}

	exec := kingpin.Command("exec", "execute command in a task")
	execOption := ecspresso.ExecOption{
		ID:        exec.Flag("id", "task ID").Default("").String(),
		Command:   exec.Flag("command", "command").Default("sh").String(),
		Container: exec.Flag("container", "container name").String(),
	}

	sub := kingpin.Parse()
	if sub == "version" {
		fmt.Println("ecspresso", Version)
		return 0
	}

	color.NoColor = !*colorOpt
	for _, envFile := range *envFiles {
		if err := ecspresso.ExportEnvFile(envFile); err != nil {
			log.Println("Failed to load envfile", err)
			return 1
		}
	}

	c := ecspresso.NewDefaultConfig()
	if sub == "init" {
		c.Region = *initOption.Region
		c.Cluster = *initOption.Cluster
		c.Service = *initOption.Service
		c.TaskDefinitionPath = *initOption.TaskDefinitionPath
		c.ServiceDefinitionPath = *initOption.ServiceDefinitionPath
		initOption.ConfigFilePath = conf
		if err := c.Restrict(); err != nil {
			log.Println("Could not init config", err)
			return 1
		}
	} else {
		if err := c.Load(*conf); err != nil {
			log.Println("Could not load config file", *conf, err)
			kingpin.Usage()
			return 1
		}
		if err := c.ValidateVersion(Version); err != nil {
			log.Println(err.Error())
			return 1
		}
	}

	app, err := ecspresso.NewApp(c)
	if err != nil {
		log.Println(err)
		return 1
	}
	app.Debug = *debug

	switch sub {
	case "deploy":
		if !isSetSuspendAutoScaling {
			deployOption.SuspendAutoScaling = nil
		}
		if isSetResumeAutoScaling {
			deployOption.SuspendAutoScaling = aws.Bool(false)
		}
		err = app.Deploy(deployOption)
	case "refresh":
		err = app.Deploy(refreshOption)
	case "scale":
		if !isSetSuspendAutoScaling {
			scaleOption.SuspendAutoScaling = nil
		}
		if isSetResumeAutoScaling {
			scaleOption.SuspendAutoScaling = aws.Bool(false)
		}
		err = app.Deploy(scaleOption)
	case "status":
		err = app.Status(statusOption)
	case "rollback":
		err = app.Rollback(rollbackOption)
	case "create":
		err = app.Create(createOption)
	case "delete":
		err = app.Delete(deleteOption)
	case "run":
		err = app.Run(runOption)
	case "wait":
		err = app.Wait(waitOption)
	case "register":
		err = app.Register(registerOption)
	case "init":
		err = app.Init(initOption)
	case "diff":
		err = app.Diff(diffOption)
	case "appspec":
		err = app.AppSpec(appspecOption)
	case "verify":
		err = app.Verify(verifyOption)
	case "render":
		err = app.Render(renderOption)
	case "tasks":
		err = app.Tasks(tasksOption)
	case "exec":
		err = app.Exec(execOption)
	default:
		kingpin.Usage()
		return 1
	}
	if err != nil {
		log.Printf("%s FAILED. %s", sub, err)
		return 1
	}

	return 0
}

func boolp(b bool) *bool {
	return &b
}

func int64p(i int64) *int64 {
	return &i
}
