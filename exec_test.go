package ecspresso_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v2"
)

func TestResolveEcstaFilters(t *testing.T) {
	tests := []struct {
		name            string
		config          string
		expectedFamily  *string
		expectedService *string
	}{
		{
			name:            "when service is configured, family is nil",
			config:          "tests/run-with-sv.yaml",
			expectedFamily:  nil,
			expectedService: aws.String("test"),
		},
		{
			name:            "when service is empty, family is loaded from task definition",
			config:          "tests/run-without-sv.yaml",
			expectedFamily:  aws.String("katsubushi"),
			expectedService: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			app, err := ecspresso.New(ctx, &ecspresso.CLIOptions{ConfigFilePath: tt.config})
			if err != nil {
				t.Fatal(err)
			}
			family, service, err := app.ResolveEcstaFilters(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if diff := cmp.Diff(tt.expectedFamily, family); diff != "" {
				t.Errorf("family mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.expectedService, service); diff != "" {
				t.Errorf("service mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
