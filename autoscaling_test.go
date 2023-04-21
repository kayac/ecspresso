package ecspresso_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v2"
)

func TestModifyAutoScalingParams_String(t *testing.T) {
	testCases := []struct {
		name     string
		params   ecspresso.ModifyAutoScalingParams
		expected string
	}{
		{
			name: "all values set",
			params: ecspresso.ModifyAutoScalingParams{
				Suspend:     aws.Bool(false),
				MinCapacity: aws.Int32(1),
				MaxCapacity: aws.Int32(5),
			},
			expected: "MaxCapacity=5,MinCapacity=1,Suspend=false",
		},
		{
			name: "only Suspend set",
			params: ecspresso.ModifyAutoScalingParams{
				Suspend: aws.Bool(true),
			},
			expected: "Suspend=true",
		},
		{
			name: "only MinCapacity set",
			params: ecspresso.ModifyAutoScalingParams{
				MinCapacity: aws.Int32(1),
			},
			expected: "MinCapacity=1",
		},
		{
			name: "only MaxCapacity set",
			params: ecspresso.ModifyAutoScalingParams{
				MaxCapacity: aws.Int32(5),
			},
			expected: "MaxCapacity=5",
		},
		{
			name:     "all values nil",
			params:   ecspresso.ModifyAutoScalingParams{},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.params.String()
			if result != tc.expected {
				t.Errorf("Expected '%s', but got '%s'", tc.expected, result)
			}
		})
	}
}

func TestModifyAutoScalingParams(t *testing.T) {
	tests := []struct {
		name     string
		opt      ecspresso.DeployOption
		expected *ecspresso.ModifyAutoScalingParams
	}{
		{
			name: "SuspendAutoScaling=true",
			opt: ecspresso.DeployOption{
				SuspendAutoScaling: aws.Bool(true),
			},
			expected: &ecspresso.ModifyAutoScalingParams{
				Suspend:     aws.Bool(true),
				MinCapacity: nil,
				MaxCapacity: nil,
			},
		},
		{
			name: "ResumeAutoScaling=true",
			opt: ecspresso.DeployOption{
				ResumeAutoScaling: aws.Bool(true),
			},
			expected: &ecspresso.ModifyAutoScalingParams{
				Suspend:     aws.Bool(false),
				MinCapacity: nil,
				MaxCapacity: nil,
			},
		},
		{
			name: "AutoScalingMin and AutoScalingMax are specified",
			opt: ecspresso.DeployOption{
				AutoScalingMin: aws.Int32(1),
				AutoScalingMax: aws.Int32(10),
			},
			expected: &ecspresso.ModifyAutoScalingParams{
				Suspend:     nil,
				MinCapacity: aws.Int32(1),
				MaxCapacity: aws.Int32(10),
			},
		},
		{
			name: "Default values",
			opt:  ecspresso.DeployOption{},
			expected: &ecspresso.ModifyAutoScalingParams{
				Suspend:     nil,
				MinCapacity: nil,
				MaxCapacity: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.opt.ModifyAutoScalingParams()

			if diff := cmp.Diff(p, tt.expected); diff != "" {
				t.Errorf("ModifyAutoScalingParams() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
