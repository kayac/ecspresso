package ecspresso

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
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

func formatEvent(e *ecs.ServiceEvent, chars int) []string {
	line := fmt.Sprintf("%s %s",
		e.CreatedAt.In(timezone).Format("2006/01/02 15:04:05"),
		*e.Message,
	)
	lines := []string{}
	n := len(line)/chars + 1
	for i := 0; i < n; i++ {
		if i == n-1 {
			lines = append(lines, line[i*chars:])
		} else {
			lines = append(lines, line[i*chars:(i+1)*chars])
		}
	}
	return lines
}

func formatLogEvent(e *cloudwatchlogs.OutputLogEvent, chars int) []string {
	t := time.Unix((*e.Timestamp / int64(1000)), 0)
	line := fmt.Sprintf("%s %s",
		t.In(timezone).Format("2006/01/02 15:04:05"),
		*e.Message,
	)
	lines := []string{}
	n := len(line)/chars + 1
	for i := 0; i < n; i++ {
		if i == n-1 {
			lines = append(lines, line[i*chars:])
		} else {
			lines = append(lines, line[i*chars:(i+1)*chars])
		}
	}
	return lines
}
