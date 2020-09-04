package ecspresso_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/kayac/ecspresso"
)

type desiredCountTestCase struct {
	sv       *ecs.Service
	opt      ecspresso.DeployOption
	expected *int64
}

var desiredCountTestSuite = []desiredCountTestCase{
	{
		sv:       &ecs.Service{DesiredCount: nil},
		opt:      ecspresso.DeployOption{DesiredCount: nil},
		expected: nil,
	},
	{
		sv:       &ecs.Service{DesiredCount: nil, SchedulingStrategy: aws.String("DAEMON")},
		opt:      ecspresso.DeployOption{DesiredCount: aws.Int64(10)},
		expected: nil,
	},
	{
		sv:       &ecs.Service{DesiredCount: aws.Int64(2)},
		opt:      ecspresso.DeployOption{DesiredCount: nil},
		expected: aws.Int64(2),
	},
	{
		sv:       &ecs.Service{DesiredCount: aws.Int64(1)},
		opt:      ecspresso.DeployOption{DesiredCount: aws.Int64(3)},
		expected: aws.Int64(3),
	},
	{
		sv:       &ecs.Service{DesiredCount: aws.Int64(1)},
		opt:      ecspresso.DeployOption{DesiredCount: aws.Int64(ecspresso.KeepDesiredCount)},
		expected: nil,
	},
	{
		sv:       &ecs.Service{DesiredCount: nil},
		opt:      ecspresso.DeployOption{DesiredCount: aws.Int64(5)},
		expected: aws.Int64(5),
	},
}

func TestCalcDesiredCount(t *testing.T) {
	for _, c := range desiredCountTestSuite {
		count := ecspresso.CalcDesiredCount(c.sv, c.opt)
		if count == nil && c.expected == nil {
			// ok
		} else if count != nil && c.expected == nil {
			t.Errorf("unexpected desired count:%d expected:nil", *count)
		} else if count == nil && c.expected != nil {
			t.Errorf("unexpected desired count:nil expected:%d", *c.expected)
		} else if *count != *c.expected {
			t.Errorf("unexpected desired count:%d expected:%d", *count, *c.expected)
		} else {
			// ok
		}
	}
}
