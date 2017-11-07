package ecspresso

import (
	"fmt"
	"strings"
)

type ServiceContainer struct {
	Services []Service `json:"services"`
}

type Service struct {
	Deployments []Deployments `json:"deployments"`
}

type Deployments struct {
	ID             string  `json:"id"`
	DesiredCount   int     `json:"desiredCount"`
	PendingCount   int     `json:"pendingCount"`
	RunningCount   int     `json:"runningCount"`
	Status         string  `json:"status"`
	TaskDefinition string  `json:"taskDefinition"`
	CreatedAt      float64 `json:"createdAt"`
	UpdatedAt      float64 `json:"updatedAt"`
}

func (d Deployments) String() string {
	td := strings.Split(d.TaskDefinition, "/")
	return fmt.Sprintf(
		"%8s %s desired:%d pending:%d running:%d",
		d.Status,
		td[len(td)-1],
		d.DesiredCount, d.PendingCount, d.RunningCount,
	)
}
