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
		arn:      "arn:aws:iam::123456789012:role/path/to/ecsTaskRole",
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

var testImagesIsECR = []struct {
	image string
	isECR bool
}{
	{
		image: "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage",
		isECR: true,
	},
	{
		image: "123456789012.dkr.ecr.ap-northeast-1.amazonaws.com/myimage:latest",
		isECR: true,
	},
	{
		image: "ubuntu:latest",
		isECR: false,
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

func TestIsECRImage(t *testing.T) {
	for _, s := range testImagesIsECR {
		isECR := ecspresso.ECRImageURLRegex.MatchString(s.image)
		if isECR != s.isECR {
			t.Errorf("invalid detect ECR image %s got:%t expected:%t", s.image, isECR, s.isECR)
		}
	}
}
