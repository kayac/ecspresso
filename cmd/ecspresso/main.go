package main

import (
	"log"
	"os"

	"github.com/kayac/ecspresso"
	config "github.com/kayac/go-config"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	os.Exit(_main())
}

func _main() int {
	conf := kingpin.Flag("config", "config file").Required().String()

	deploy := kingpin.Command("deploy", "deploy service")
	deployOption := ecspresso.DeployOption{
		DryRun: deploy.Flag("dry-run", "dry-run").Bool(),
	}

	status := kingpin.Command("status", "show status of service")
	statusOption := ecspresso.StatusOption{
		Events: status.Flag("events", "show events num").Default("2").Int(),
	}

	rollback := kingpin.Command("rollback", "rollback service")
	rollbackOption := ecspresso.RollbackOption{
		DryRun: rollback.Flag("dry-run", "dry-run").Bool(),
	}

	sub := kingpin.Parse()

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
	}
	if err != nil {
		log.Printf("%s FAILED. %s", sub, err)
		return 1
	}

	return 0
}
