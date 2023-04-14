package ecspresso_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
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
			expected: "Suspend: false, MinCapacity: 1, MaxCapacity: 5",
		},
		{
			name: "only Suspend set",
			params: ecspresso.ModifyAutoScalingParams{
				Suspend: aws.Bool(true),
			},
			expected: "Suspend: true, MinCapacity: nil, MaxCapacity: nil",
		},
		{
			name: "only MinCapacity set",
			params: ecspresso.ModifyAutoScalingParams{
				MinCapacity: aws.Int32(1),
			},
			expected: "Suspend: nil, MinCapacity: 1, MaxCapacity: nil",
		},
		{
			name: "only MaxCapacity set",
			params: ecspresso.ModifyAutoScalingParams{
				MaxCapacity: aws.Int32(5),
			},
			expected: "Suspend: nil, MinCapacity: nil, MaxCapacity: 5",
		},
		{
			name:     "all values nil",
			params:   ecspresso.ModifyAutoScalingParams{},
			expected: "Suspend: nil, MinCapacity: nil, MaxCapacity: nil",
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
