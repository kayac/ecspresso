package ecspresso

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	aasTypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	logsTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fujiwara/sloghandler"
)

var EventTimeFormat = sloghandler.TimeFormat

type genericLogEvent struct {
	Time   time.Time `json:"time"`
	Level  string    `json:"level"`
	Msg    string    `json:"msg"`
	TaskID string    `json:"task_id,omitempty"`
}

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

func formatExpress(ex *types.ECSExpressGatewayService) string {
	var b strings.Builder
	fmt.Fprintf(&b, spcIndent+"Status: %s %s\n", ex.Status.StatusCode, aws.ToString(ex.Status.StatusReason))
	if len(ex.ActiveConfigurations) > 0 {
		ac := ex.ActiveConfigurations[0]
		if len(ac.IngressPaths) > 0 {
			fmt.Fprintln(&b, spcIndent+"IngressPaths:")
		}
		for _, p := range ac.IngressPaths {
			fmt.Fprintf(&b, spcIndent+spcIndent+"%s %s\n", p.AccessType, aws.ToString(p.Endpoint))
		}
	}
	return b.String()
}

type serviceEvent types.ServiceEvent

func (e serviceEvent) String() string {
	return fmt.Sprintf("%s %s",
		e.CreatedAt.In(time.Local).Format(EventTimeFormat),
		*e.Message,
	)
}

func (e serviceEvent) JSON() string {
	t := e.CreatedAt.In(time.Local)
	b, _ := json.Marshal(genericLogEvent{
		Time:  t,
		Level: slog.LevelInfo.String(),
		Msg:   *e.Message,
	})
	return string(b)
}

// logEvent is a log event of a task container. taskID is set to distinguish logs of multiple tasks.
type logEvent struct {
	event  logsTypes.OutputLogEvent
	taskID string
}

func (e logEvent) String() string {
	t := time.Unix((*e.event.Timestamp / int64(1000)), 0).In(time.Local).Format(EventTimeFormat)
	if e.taskID != "" {
		// prefix every line, as a multiline event (e.g. a stack trace) has no other way to tell its task
		prefix := fmt.Sprintf("%s [%s] ", t, e.taskID)
		msg := strings.TrimSuffix(*e.event.Message, "\n")
		return prefix + strings.ReplaceAll(msg, "\n", "\n"+prefix)
	}
	return fmt.Sprintf("%s %s", t, *e.event.Message)
}

func (e logEvent) JSON() string {
	t := time.Unix((*e.event.Timestamp / int64(1000)), 0).In(time.Local)
	b, _ := json.Marshal(genericLogEvent{
		Time:   t,
		Level:  slog.LevelInfo.String(),
		Msg:    *e.event.Message,
		TaskID: e.taskID,
	})
	return string(b)
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
