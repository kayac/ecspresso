package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/kayac/ecspresso"

	config "github.com/kayac/go-config"
)

var Version = "current"

func main() {
	os.Exit(_main())
}

func _main() int {
	kingpin.Command("version", "show version")

	conf := kingpin.Flag("config", "config file").Required().String()
	debug := kingpin.Flag("debug", "enable debug log").Bool()

	var isSetSuspendAutoScaling bool
	deploy := kingpin.Command("deploy", "deploy service")
	deployOption := ecspresso.DeployOption{
		DryRun:             deploy.Flag("dry-run", "dry-run").Bool(),
		DesiredCount:       deploy.Flag("tasks", "desired count of tasks").Default("-1").Int64(),
		SkipTaskDefinition: deploy.Flag("skip-task-definition", "skip register a new task definition").Bool(),
		ForceNewDeployment: deploy.Flag("force-new-deployment", "force a new deployment of the service").Bool(),
		NoWait:             deploy.Flag("no-wait", "exit ecspresso immediately after just deployed without waiting for service stable").Bool(),
		SuspendAutoScaling: deploy.Flag("suspend-auto-scaling", "set suspend to auto-scaling attached with the ECS service").IsSetByUser(&isSetSuspendAutoScaling).Bool(),
	}

	create := kingpin.Command("create", "create service")
	createOption := ecspresso.CreateOption{
		DryRun:       create.Flag("dry-run", "dry-run").Bool(),
		DesiredCount: create.Flag("tasks", "desired count of tasks").Default("1").Int64(),
		NoWait:       create.Flag("no-wait", "exit ecspresso immediately after just created without waiting for service stable").Bool(),
	}

	status := kingpin.Command("status", "show status of service")
	statusOption := ecspresso.StatusOption{
		Events: status.Flag("events", "show events num").Default("2").Int(),
	}

	rollback := kingpin.Command("rollback", "rollback service")
	rollbackOption := ecspresso.RollbackOption{
		DryRun: rollback.Flag("dry-run", "dry-run").Bool(),
		DeregisterTaskDefinition: rollback.Flag("deregister-task-definition", "deregister rolled back task definition").Bool(),
		NoWait: rollback.Flag("no-wait", "exit ecspresso immediately after just rollbacked without waiting for service stable").Bool(),
	}

	delete := kingpin.Command("delete", "delete service")
	deleteOption := ecspresso.DeleteOption{
		DryRun: delete.Flag("dry-run", "dry-run").Bool(),
		Force:  delete.Flag("force", "force delete. not confirm").Bool(),
	}

	run := kingpin.Command("run", "run task")
	runOption := ecspresso.RunOption{
		DryRun:             run.Flag("dry-run", "dry-run").Bool(),
		TaskDefinition:     run.Flag("task-def", "task definition json for run task").String(),
		NoWait:             run.Flag("no-wait", "exit ecspresso after task run").Bool(),
		TaskOverrideStr:    run.Flag("overrides", "task overrides JSON string").Default("").String(),
		SkipTaskDefinition: run.Flag("skip-task-definition", "skip register a new task definition").Bool(),
		Count:              run.Flag("count", "the number of tasks (max 10)").Default("1").Int64(),
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
		Region:                init.Flag("region", "AWS region name").Required().String(),
		Cluster:               init.Flag("cluster", "cluster name").Default("default").String(),
		Service:               init.Flag("service", "service name").Required().String(),
		TaskDefinitionPath:    init.Flag("task-definition-path", "output task definition file path").Default("ecs-task-def.json").String(),
		ServiceDefinitionPath: init.Flag("service-definition-path", "output service definition file path").Default("ecs-service-def.json").String(),
	}

	sub := kingpin.Parse()
	if sub == "version" {
		fmt.Println("ecspresso", Version)
		return 0
	}

	c := ecspresso.NewDefaultConfig()
	if sub == "init" {
		c.Region = *initOption.Region
		c.Cluster = *initOption.Cluster
		c.Service = *initOption.Service
		c.TaskDefinitionPath = *initOption.TaskDefinitionPath
		c.ServiceDefinitionPath = *initOption.ServiceDefinitionPath
		initOption.ConfigFilePath = conf
	} else {
		if err := config.LoadWithEnv(c, *conf); err != nil {
			log.Println("Cloud not load config file", conf, err)
			kingpin.Usage()
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
		err = app.Deploy(deployOption)
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
