package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kayac/ecspresso"
	config "github.com/kayac/go-config"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var Version = "current"

func main() {
	os.Exit(_main())
}

func _main() int {
	kingpin.Command("version", "show version")

	conf := kingpin.Flag("config", "config file").String()

	deploy := kingpin.Command("deploy", "deploy service")
	deployOption := ecspresso.DeployOption{
		DryRun:             deploy.Flag("dry-run", "dry-run").Bool(),
		DesiredCount:       deploy.Flag("tasks", "desired count of tasks").Default("-1").Int64(),
		SkipTaskDefinition: deploy.Flag("skip-task-definition", "skip register a new task definition").Bool(),
		ForceNewDeployment: deploy.Flag("force-new-deployment", "force a new deployment of the service").Bool(),
	}

	create := kingpin.Command("create", "create service")
	createOption := ecspresso.CreateOption{
		DryRun:       create.Flag("dry-run", "dry-run").Bool(),
		DesiredCount: create.Flag("tasks", "desired count of tasks").Default("1").Int64(),
	}

	status := kingpin.Command("status", "show status of service")
	statusOption := ecspresso.StatusOption{
		Events: status.Flag("events", "show events num").Default("2").Int(),
	}

	rollback := kingpin.Command("rollback", "rollback service")
	rollbackOption := ecspresso.RollbackOption{
		DryRun: rollback.Flag("dry-run", "dry-run").Bool(),
		DeregisterTaskDefinition: rollback.Flag("dereginster-task-definition", "deregister rolled back task definition").Bool(),
	}

	delete := kingpin.Command("delete", "delete service")
	deleteOption := ecspresso.DeleteOption{
		DryRun: delete.Flag("dry-run", "dry-run").Bool(),
		Force:  delete.Flag("force", "force delete. not confirm").Bool(),
	}

	run := kingpin.Command("run", "run task")
	runOption := ecspresso.RunOption{
		DryRun:         run.Flag("dry-run", "dry-run").Bool(),
		TaskDefinition: run.Flag("task-def", "task definition json for run task").String(),
	}

	scheduler := kingpin.Command("scheduler", "task scheduler")
	schedulerOption := ecspresso.SchedulerOption{
		SchedulerDefinition: scheduler.Flag("scheduler-def", "scheduler definition json").String(),
	}

	schedulerPut := scheduler.Command("put", "put task scheduler")
	schedulerPutOption := ecspresso.SchedulerPutOption{
		SchedulerOption:    schedulerOption,
		DryRun:             schedulerPut.Flag("dry-run", "dry-run").Bool(),
		SkipTaskDefinition: schedulerPut.Flag("skip-task-definition", "skip register a new task definition").Bool(),
	}

	schedulerDelete := scheduler.Command("delete", "delete task scheduler")
	schedulerDeleteOption := ecspresso.SchedulerDeleteOption{
		SchedulerOption: schedulerOption,
		DryRun:          schedulerDelete.Flag("dry-run", "dry-run").Bool(),
		Force:           schedulerDelete.Flag("force", "force delete. not confirm").Bool(),
	}

	schedulerEnable := scheduler.Command("enable", "enable task scheduler")
	schedulerEnableOption := ecspresso.SchedulerEnableOption{
		SchedulerOption: schedulerOption,
		DryRun:          schedulerEnable.Flag("dry-run", "dry-run").Bool(),
	}

	schedulerDisable := scheduler.Command("disable", "disable task scheduler")
	schedulerDisableOption := ecspresso.SchedulerDisableOption{
		SchedulerOption: schedulerOption,
		DryRun:          schedulerDisable.Flag("dry-run", "dry-run").Bool(),
	}

	sub := kingpin.Parse()
	if sub == "version" {
		fmt.Println("ecspresso", Version)
		return 0
	}

	c := ecspresso.NewDefaultConfig()
	if err := config.Load(c, *conf); err != nil {
		log.Println("Cloud not load config file", conf, err)
		kingpin.Usage()
		return 1
	}
	app, err := ecspresso.NewApp(c)
	if err != nil {
		log.Println(err)
		return 1
	}

	switch sub {
	case "deploy":
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
	case "scheduler put":
		err = app.SchedulerPut(schedulerPutOption)
	case "scheduler delete":
		err = app.SchedulerDelete(schedulerDeleteOption)
	case "scheduler enable":
		err = app.SchedulerEnable(schedulerEnableOption)
	case "scheduler disable":
		err = app.SchedulerDisable(schedulerDisableOption)
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
