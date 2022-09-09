package ecspresso_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/kayac/ecspresso"
)

type desiredCountTestCase struct {
	sv       *ecspresso.Service
	opt      ecspresso.DeployOption
	expected *int32
}

var desiredCountTestSuite = []desiredCountTestCase{
	{
		sv:       &ecspresso.Service{DesiredCount: 0},
		opt:      ecspresso.DeployOption{DesiredCount: nil},
		expected: nil,
	},
	{
		sv:       &ecspresso.Service{DesiredCount: 0, SchedulingStrategy: types.SchedulingStrategyDaemon},
		opt:      ecspresso.DeployOption{DesiredCount: aws.Int32(10)},
		expected: nil,
	},
	{
		sv:       &ecspresso.Service{DesiredCount: 2},
		opt:      ecspresso.DeployOption{DesiredCount: nil},
		expected: nil,
	},
	{
		sv:       &ecspresso.Service{DesiredCount: 1},
		opt:      ecspresso.DeployOption{DesiredCount: aws.Int32(3)},
		expected: aws.Int32(3),
	},
	{
		sv:       &ecspresso.Service{DesiredCount: 1},
		opt:      ecspresso.DeployOption{DesiredCount: aws.Int32(ecspresso.DefaultDesiredCount)},
		expected: aws.Int32(1),
	},
	{
		sv:       &ecspresso.Service{DesiredCount: 0},
		opt:      ecspresso.DeployOption{DesiredCount: aws.Int32(5)},
		expected: aws.Int32(5),
	},
	{
		sv:       &ecspresso.Service{DesiredCount: 0},
		opt:      ecspresso.DeployOption{DesiredCount: aws.Int32(ecspresso.DefaultDesiredCount)},
		expected: nil,
	},
}

func TestCalcDesiredCount(t *testing.T) {
	for n, c := range desiredCountTestSuite {
		count := ecspresso.CalcDesiredCount(c.sv, c.opt)
		if count == nil && c.expected == nil {
			// ok
		} else if count != nil && c.expected == nil {
			t.Errorf("case %d unexpected desired count:%d expected:nil", n, *count)
		} else if count == nil && c.expected != nil {
			t.Errorf("case %d unexpected desired count:nil expected:%d", n, *c.expected)
		} else if *count != *c.expected {
			t.Errorf("case %d unexpected desired count:%d expected:%d", n, *count, *c.expected)
		} else {
			// ok
		}
	}
}
