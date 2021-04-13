package ecspresso_test

import (
	"testing"

	"github.com/kayac/ecspresso"
)

var testRoleArns = []struct {
	arn      string
	roleName string
	isValid  bool
}{
	{
		arn:      "arn:aws:iam::123456789012:role/ecsTaskRole",
		roleName: "ecsTaskRole",
		isValid:  true,
	},
	{
		arn: "arn:aws:iam::123456789012:foo",
	},
	{
		arn: "arn:aws:iam::123456789012:policy/ecsTaskRole",
	},
	{
		arn: "arn:aws:ec2::123456789012:foo/bar",
	},
	{
		arn: "ecsTaskRole",
	},
}

func TestParseRoleArn(t *testing.T) {
	for _, s := range testRoleArns {
		name, err := ecspresso.ParseRoleArn(s.arn)
		if s.isValid {
			if name != s.roleName {
				t.Errorf("invalid roleName got:%s expected:%s", name, s.roleName)
			}
			if err != nil {
				t.Error("unexpected error", err)
			}
		} else if err == nil {
			t.Errorf("must be failed valdation for %s", s.arn)
		}
	}
}
