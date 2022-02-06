package ecspresso_test

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
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

type goPlatform struct {
	arch string
	os   string
}

var testRuntimePlatforms = []struct {
	platform                *ecs.RuntimePlatform
	requiredCompatibilities []*string
	want                    goPlatform
}{
	{
		requiredCompatibilities: []*string{
			aws.String(ecs.CompatibilityEc2),
		},
		want: goPlatform{
			arch: "",
			os:   "",
		},
	},
	{
		requiredCompatibilities: []*string{
			aws.String(ecs.CompatibilityFargate),
			aws.String(ecs.CompatibilityEc2),
		},
		want: goPlatform{
			arch: "amd64",
			os:   "linux",
		},
	},
	{
		requiredCompatibilities: []*string{
			aws.String(ecs.CompatibilityFargate),
		},
		platform: &ecs.RuntimePlatform{
			CpuArchitecture: aws.String(ecs.CPUArchitectureArm64),
		},
		want: goPlatform{
			arch: "arm64",
			os:   "linux",
		},
	},
	{
		requiredCompatibilities: []*string{
			aws.String(ecs.CompatibilityFargate),
		},
		platform: &ecs.RuntimePlatform{
			OperatingSystemFamily: aws.String(ecs.OSFamilyWindowsServer2019Core),
		},
		want: goPlatform{
			arch: "amd64",
			os:   "windows",
		},
	},
	{
		requiredCompatibilities: []*string{
			aws.String(ecs.CompatibilityEc2),
			aws.String(ecs.CompatibilityExternal),
		},
		want: goPlatform{
			arch: "",
			os:   "",
		},
	},
}

func TestNormalizePlatform(t *testing.T) {
	for _, p := range testRuntimePlatforms {
		arch, os := ecspresso.NormalizePlatform(p.platform, p.requiredCompatibilities)
		if arch != p.want.arch || os != p.want.os {
			t.Errorf("want arch/os %s/%s but got %s/%s", p.want.arch, p.want.os, arch, os)
		}
	}
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
