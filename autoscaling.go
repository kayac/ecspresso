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

func (p *modifyAutoScalingParams) String() string {
	m := map[string]string{}
	if p.Suspend != nil {
		m["Suspend"] = fmt.Sprintf("%v", *p.Suspend)
	}
	if p.MinCapacity != nil {
		m["MinCapacity"] = fmt.Sprintf("%d", *p.MinCapacity)
	}
	if p.MaxCapacity != nil {
		m["MaxCapacity"] = fmt.Sprintf("%d", *p.MaxCapacity)
	}
	return map2str(m)
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

func (d *App) modifyAutoScaling(ctx context.Context, opt DeployOption) error {
	p := opt.ModifyAutoScalingParams()
	if p.isEmpty() {
		return nil
	}
	d.Log("[INFO] Modify auto scaling settings %s", p.String())

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

	if opt.DryRun {
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
