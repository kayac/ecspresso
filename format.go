package ecspresso

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	aasTypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	logsTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

var EventTimeFormat = "2006/01/02 15:04:05"

func formatDeployment(dp types.Deployment) string {
	return fmt.Sprintf(
		"%8s %s desired:%d pending:%d running:%d %s(%s)",
		aws.ToString(dp.Status),
		arnToName(aws.ToString(dp.TaskDefinition)),
		dp.DesiredCount, dp.PendingCount, dp.RunningCount,
		dp.RolloutState, aws.ToString(dp.RolloutStateReason),
	)
}

func formatTaskSet(ts types.TaskSet) string {
	return fmt.Sprintf(
		"%8s %s desired:%d pending:%d running:%d %s",
		aws.ToString(ts.Status),
		arnToName(aws.ToString(ts.TaskDefinition)),
		ts.ComputedDesiredCount, ts.PendingCount, ts.RunningCount,
		ts.StabilityStatus,
	)
}

func formatEvent(e types.ServiceEvent) string {
	return fmt.Sprintf("%s %s",
		e.CreatedAt.In(time.Local).Format(EventTimeFormat),
		*e.Message,
	)
}

func formatLogEvent(e logsTypes.OutputLogEvent) string {
	t := time.Unix((*e.Timestamp / int64(1000)), 0)
	return fmt.Sprintf("%s %s",
		t.In(time.Local).Format(EventTimeFormat),
		*e.Message,
	)
}

func formatScalableTarget(t aasTypes.ScalableTarget) string {
	return strings.Join([]string{
		fmt.Sprintf(
			spcIndent+"Capacity min:%d max:%d",
			*t.MinCapacity,
			*t.MaxCapacity,
		),
		fmt.Sprintf(
			spcIndent+"Suspended in:%t out:%t scheduled:%t",
			*t.SuspendedState.DynamicScalingInSuspended,
			*t.SuspendedState.DynamicScalingOutSuspended,
			*t.SuspendedState.ScheduledScalingSuspended,
		),
	}, "\n")
}

func formatScalingPolicy(p aasTypes.ScalingPolicy) string {
	return fmt.Sprintf("  Policy name:%s type:%s", *p.PolicyName, p.PolicyType)
}
