package ecspresso

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

const dryRunStr = "DRY RUN"

type DryRunnable interface {
	DryRunString() bool
}

type DeployOption struct {
	DryRun               *bool
	DesiredCount         *int32
	SkipTaskDefinition   *bool
	ForceNewDeployment   *bool
	NoWait               *bool
	SuspendAutoScaling   *bool
	RollbackEvents       *string
	UpdateService        *bool
	LatestTaskDefinition *bool
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
	Count                *int32
	WatchContainer       *string
	LatestTaskDefinition *bool
	PropagateTags        *string
	Tags                 *string
	WaitUntil            *string
	Revision             *int64
}

func (opt RunOption) waitUntilRunning() bool {
	return aws.ToString(opt.WaitUntil) == "running"
}

func (opt RunOption) DryRunString() string {
	if *opt.DryRun {
		return ""
	}
	return ""
}

func parseTags(s string) ([]types.Tag, error) {
	tags := make([]types.Tag, 0)
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
			return tags, fmt.Errorf("invalid tag format. Key=Value is required: %s", tag)
		}
		if len(pair[0]) == 0 {
			return tags, fmt.Errorf("tag Key is required")
		}
		tags = append(tags, types.Tag{
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
