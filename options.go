package ecspresso

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/pkg/errors"
)

const dryRunStr = "DRY RUN"

type DryRunnable interface {
	DryRunString() bool
}

type optWithDesiredCount interface {
	getDesiredCount() *int64
}

type DeployOption struct {
	DryRun               *bool
	DesiredCount         *int64
	SkipTaskDefinition   *bool
	ForceNewDeployment   *bool
	NoWait               *bool
	SuspendAutoScaling   *bool
	RollbackEvents       *string
	UpdateService        *bool
	LatestTaskDefinition *bool
}

func (opt DeployOption) getDesiredCount() *int64 {
	return opt.DesiredCount
}

func (opt DeployOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

type StatusOption struct {
	Events *int
}

type RollbackOption struct {
	DryRun                   *bool
	DeregisterTaskDefinition *bool
	NoWait                   *bool
	RollbackEvents           *string
}

func (opt RollbackOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

type DeleteOption struct {
	DryRun *bool
	Force  *bool
}

func (opt DeleteOption) DryRunString() string {
	if *opt.DryRun {
		return dryRunStr
	}
	return ""
}

type RunOption struct {
	DryRun               *bool
	TaskDefinition       *string
	NoWait               *bool
	TaskOverrideStr      *string
	TaskOverrideFile     *string
	SkipTaskDefinition   *bool
	Count                *int64
	WatchContainer       *string
	LatestTaskDefinition *bool
	PropagateTags        *string
	Tags                 *string
	WaitUntil            *string
	Revision             *int64
}

func (opt RunOption) waitUntilRunning() bool {
	return aws.StringValue(opt.WaitUntil) == "running"
}

func (opt RunOption) DryRunString() string {
	if *opt.DryRun {
		return ""
	}
	return ""
}

func parseTags(s string) ([]*ecs.Tag, error) {
	tags := make([]*ecs.Tag, 0)
	if s == "" {
		return tags, nil
	}

	tagsStr := strings.Split(s, ",")
	for _, tag := range tagsStr {
		if tag == "" {
			continue
		}
		pair := strings.SplitN(tag, "=", 2)
		if len(pair) != 2 {
			return tags, errors.Errorf("invalid tag format. Key=Value is required: %s", tag)
		}
		if len(pair[0]) == 0 {
			return tags, errors.Errorf("tag Key is required")
		}
		tags = append(tags, &ecs.Tag{
			Key:   aws.String(pair[0]),
			Value: aws.String(pair[1]),
		})
	}
	return tags, nil
}

type WaitOption struct {
}

type DiffOption struct {
	Unified *bool
}

type AppSpecOption struct {
	TaskDefinition *string
	UpdateService  *bool
}
