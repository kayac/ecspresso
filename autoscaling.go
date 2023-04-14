package ecspresso

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	aasTypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
)

type modifyAutoScalingParams struct {
	Suspend     *bool
	MinCapacity *int32
	MaxCapacity *int32
}

func (m *modifyAutoScalingParams) String() string {
	var suspendStr, minCapStr, maxCapStr string

	if m.Suspend == nil {
		suspendStr = "nil"
	} else {
		suspendStr = fmt.Sprintf("%v", *m.Suspend)
	}

	if m.MinCapacity == nil {
		minCapStr = "nil"
	} else {
		minCapStr = fmt.Sprintf("%d", *m.MinCapacity)
	}

	if m.MaxCapacity == nil {
		maxCapStr = "nil"
	} else {
		maxCapStr = fmt.Sprintf("%d", *m.MaxCapacity)
	}

	return fmt.Sprintf("Suspend: %s, MinCapacity: %s, MaxCapacity: %s", suspendStr, minCapStr, maxCapStr)
}

func (p *modifyAutoScalingParams) isEmpty() bool {
	return p.Suspend == nil && p.MinCapacity == nil && p.MaxCapacity == nil
}

func (p *modifyAutoScalingParams) SuspendState() *aasTypes.SuspendedState {
	if p.Suspend == nil {
		return nil
	}
	return &aasTypes.SuspendedState{
		DynamicScalingInSuspended:  p.Suspend,
		DynamicScalingOutSuspended: p.Suspend,
		ScheduledScalingSuspended:  p.Suspend,
	}
}

func (d *App) modifyAutoScaling(ctx context.Context, p *modifyAutoScalingParams) error {
	if p.isEmpty() {
		return nil
	}
	d.Log("[INFO] Modifying auto scaling settings %s", p.String())

	resourceId := fmt.Sprintf("service/%s/%s", d.Cluster, d.Service)
	out, err := d.autoScaling.DescribeScalableTargets(
		ctx,
		&applicationautoscaling.DescribeScalableTargetsInput{
			ResourceIds:       []string{resourceId},
			ServiceNamespace:  aasTypes.ServiceNamespaceEcs,
			ScalableDimension: aasTypes.ScalableDimensionECSServiceDesiredCount,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to describe scalable targets: %w", err)
	}
	if len(out.ScalableTargets) == 0 {
		d.Log("[WARNING] No scalable target for %s", resourceId)
		d.Log("[INFO] Skip modifying auto scaling settings")
		return nil
	}

	for _, target := range out.ScalableTargets {
		d.Log("[INFO] Register scalable target %s %s", *target.ResourceId, p.String())
		_, err := d.autoScaling.RegisterScalableTarget(
			ctx,
			&applicationautoscaling.RegisterScalableTargetInput{
				ServiceNamespace:  target.ServiceNamespace,
				ScalableDimension: target.ScalableDimension,
				ResourceId:        target.ResourceId,
				SuspendedState:    p.SuspendState(),
				MinCapacity:       p.MinCapacity,
				MaxCapacity:       p.MaxCapacity,
			},
		)
		if err != nil {
			return fmt.Errorf("failed to register scalable target %s %w", *target.ResourceId, err)
		}
	}
	return nil
}
