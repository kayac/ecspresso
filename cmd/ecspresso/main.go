package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/kayac/ecspresso"
)

func main() {
	var (
		service, cluster, path string
		timeout                int
	)

	flag.StringVar(&service, "service", "", "ECS service name(required)")
	flag.StringVar(&cluster, "cluster", "", "ECS cluster name(required)")
	flag.StringVar(&path, "task-definition", "", "task definition path(required)")
	flag.IntVar(&timeout, "timeout", 300, "timeout (sec)")
	flag.Parse()

	if service == "" || cluster == "" || path == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	err := ecspresso.Run(
		service, cluster, path,
		time.Duration(timeout)*time.Second,
	)
	if err != nil {
		log.Println("FAILED:", err)
		os.Exit(1)
	}
}
