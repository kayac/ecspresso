package ecspresso

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
)

func formatDeployment(d *ecs.Deployment) string {
	td := strings.Split(*d.TaskDefinition, "/")
	return fmt.Sprintf(
		"%8s %s desired:%d pending:%d running:%d",
		*d.Status,
		td[len(td)-1],
		*d.DesiredCount, *d.PendingCount, *d.RunningCount,
	)
}
