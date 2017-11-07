package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/kayac/ecspresso"
	config "github.com/kayac/go-config"
)

func main() {
	os.Exit(_main())
}

func _main() int {
	var (
		conf, service, cluster, path string
		timeout                      int
	)

	flag.StringVar(&conf, "config", "", "Config file")
	flag.StringVar(&service, "service", "", "ECS service name(required)")
	flag.StringVar(&cluster, "cluster", "", "ECS cluster name(required)")
	flag.StringVar(&path, "task-definition", "", "task definition path(required)")
	flag.IntVar(&timeout, "timeout", 300, "timeout (sec)")
	flag.Parse()

	c := ecspresso.Config{
		Service:            service,
		Cluster:            cluster,
		TaskDefinitionPath: path,
		Timeout:            time.Duration(timeout) * time.Second,
	}
	if conf != "" {
		if err := config.Load(&c, conf); err != nil {
			log.Println("Cloud not load config file", conf, err)
			return 1
		}
	}
	if err := (&c).Validate(); err != nil {
		log.Println(err)
		flag.PrintDefaults()
		return 1
	}

	if err := ecspresso.Run(&c); err != nil {
		log.Println("FAILED:", err)
		return 1
	}

	return 0
}
