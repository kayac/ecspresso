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
	deployDryRun := deploy.Flag("dry-run", "dry-run").Bool()

	status := kingpin.Command("status", "show status of service")
	statusEvents := status.Flag("events", "show events num").Default("2").Int()

	rollback := kingpin.Command("rollback", "rollback service")
	rollbackDryRun := rollback.Flag("dry-run", "dry-run").Bool()

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
		err = app.Deploy(*deployDryRun)
	case "status":
		err = app.Status(*statusEvents)
	case "rollback":
		err = app.Rollback(*rollbackDryRun)
	}
	if err != nil {
		log.Printf("%s FAILED. %s", sub, err)
		return 1
	}

	return 0
}
