package ecspresso

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ecs"
)

var timezone, _ = time.LoadLocation("Local")

func arnToName(s string) string {
	ns := strings.Split(s, "/")
	return ns[len(ns)-1]
}

func formatDeployment(d *ecs.Deployment) string {
	return fmt.Sprintf(
		"%8s %s desired:%d pending:%d running:%d",
		*d.Status,
		arnToName(*d.TaskDefinition),
		*d.DesiredCount, *d.PendingCount, *d.RunningCount,
	)
}

func formatEvent(e *ecs.ServiceEvent) string {
	return fmt.Sprintf("%s: %s",
		e.CreatedAt.In(timezone).Format(time.RFC3339),
		*e.Message,
	)
}
