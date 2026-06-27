package ecspresso

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type ContinueOption struct {
	Wait      bool   `help:"wait for service stable after continue" default:"true" negatable:""`
	WaitUntil string `help:"Choose whether to wait for service stable, the deployment finishes, or a lifecycle hook pauses. (stable|deployed|paused)" default:"deployed" enum:"stable,deployed,paused"`
}

func (d *App) Continue(ctx context.Context, opt ContinueOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	d.LogInfo("Starting continue")
	sv, err := d.DescribeServiceStatus(ctx, 0)
	if err != nil {
		return err
	}

	deploymentArn, err := d.findActiveECSDeploymentArn(ctx, 0, false)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("no active deployment found")
		}
		return err
	}

	hookId, err := d.findPausedLifecycleHook(ctx, deploymentArn)
	if err != nil {
		return err
	}
	if hookId == "" {
		return fmt.Errorf("no paused lifecycle hook found in deployment %s", arnToName(deploymentArn))
	}

	d.LogInfo("continuing deployment",
		"deployment", arnToName(deploymentArn),
		"hook_id", hookId,
	)

	if _, err := d.ecs.ContinueServiceDeployment(ctx, &ecs.ContinueServiceDeploymentInput{
		ServiceDeploymentArn: &deploymentArn,
		HookId:               &hookId,
		Action:               types.DeploymentLifecycleHookActionContinue,
	}); err != nil {
		return fmt.Errorf("failed to continue service deployment: %w", err)
	}
	d.LogInfo("deployment continued successfully")

	if !opt.Wait {
		return nil
	}

	doWait, err := d.WaitFunc(sv, nil, waitUntil(opt.WaitUntil), deploymentArn)
	if err != nil {
		return err
	}
	if err := doWait(ctx, sv); err != nil {
		return err
	}
	d.LogInfo(waitUntil(opt.WaitUntil).doneMessage())
	return nil
}
