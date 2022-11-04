package ecspresso_test

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso"
)

var cliTests = []struct {
	args   []string
	sub    string
	option interface{}
}{
	{
		args: []string{"status"},
		sub:  "status",
		option: &ecspresso.StatusOption{
			Events: aws.Int(2),
		},
	},
}

func TestParseCLI(t *testing.T) {
	for _, tt := range cliTests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			sub, opt, err := ecspresso.ParseCLI(tt.args)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if sub != tt.sub {
				t.Errorf("unexpected subcommand: expected %s, got %s", tt.sub, sub)
			}
			if diff := cmp.Diff(opt.ForSubCommand(sub), tt.option); diff != "" {
				t.Errorf("unexpected option: diff %s", diff)
			}
		})
	}
}
