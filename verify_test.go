package ecspresso_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fatih/color"
	"github.com/kayac/ecspresso/v2"
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
	platform  types.RuntimePlatform
	isFargate bool
	want      goPlatform
}{
	{
		isFargate: false,
		want: goPlatform{
			arch: "amd64",
			os:   "linux",
		},
	},
	{
		platform: types.RuntimePlatform{
			CpuArchitecture: types.CPUArchitectureArm64,
		},
		isFargate: true,
		want: goPlatform{
			arch: "arm64",
			os:   "linux",
		},
	},
	{
		platform: types.RuntimePlatform{
			OperatingSystemFamily: types.OSFamilyWindowsServer2019Core,
		},
		isFargate: true,
		want: goPlatform{
			arch: "amd64",
			os:   "windows",
		},
	},
	{
		platform: types.RuntimePlatform{
			OperatingSystemFamily: types.OSFamilyWindowsServer2022Full,
		},
		isFargate: true,
		want: goPlatform{
			arch: "amd64",
			os:   "windows",
		},
	},
	{
		platform: types.RuntimePlatform{
			CpuArchitecture: types.CPUArchitectureX8664,
		},
		isFargate: false,
		want: goPlatform{
			arch: "amd64",
			os:   "linux",
		},
	},
	{
		platform: types.RuntimePlatform{
			OperatingSystemFamily: types.OSFamilyWindowsServer2019Core,
		},
		isFargate: false,
		want: goPlatform{
			arch: "amd64",
			os:   "windows",
		},
	},
}

func TestNormalizePlatform(t *testing.T) {
	for _, p := range testRuntimePlatforms {
		arch, os := ecspresso.NormalizePlatform(&p.platform, p.isFargate)
		if arch != p.want.arch || os != p.want.os {
			t.Errorf("want arch/os %s/%s but got %s/%s", p.want.arch, p.want.os, arch, os)
		}
	}
}

func TestParseRoleArn(t *testing.T) {
	for _, s := range testRoleArns {
		name, err := ecspresso.ExtractRoleName(s.arn)
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

func TestVerifyOKResource(t *testing.T) {
	color.NoColor = true
	for _, cache := range []bool{false, true} {
		ecspresso.InitVerifyState(cache)
		for i := 0; i < 3; i++ {
			out := extractStdout(t, func() {
				err := ecspresso.VerifyResource(context.TODO(), "ok resource", func(_ context.Context) error {
					return nil
				})
				if err != nil {
					t.Error("unexpected error for ok resource", err)
				}
			})
			if !bytes.Contains(out, []byte("ok resource")) {
				t.Error("unexpected output for ok resource")
			}
			if !bytes.Contains(out, []byte("[OK]")) {
				t.Error("unexpected output [OK] for ok resource")
			}
			if cache && i >= 1 {
				if !bytes.Contains(out, []byte("(cached)")) {
					t.Error("unexpected output (cached) for ok resource")
				}
			}
		}
	}
}

func TestVerifyNGResource(t *testing.T) {
	color.NoColor = true
	for _, cache := range []bool{false, true} {
		ecspresso.InitVerifyState(cache)
		for i := 0; i < 3; i++ {
			out := extractStdout(t, func() {
				err := ecspresso.VerifyResource(context.TODO(), "ng resource", func(_ context.Context) error {
					return errors.New("XXX")
				})
				if err == nil {
					t.Error("error must be returned for ng resource")
				}
			})
			if !bytes.Contains(out, []byte("ng resource")) {
				t.Error("unexpected output for ng resource")
			}
			if cache && i >= 1 {
				if !bytes.Contains(out, []byte("[NG](cached) XXX")) {
					t.Errorf("unexpected output (cached) for ng resource")
				}
			} else {
				if !bytes.Contains(out, []byte("[NG] XXX")) {
					t.Error("unexpected output [NG] for ng resource")
				}
			}
		}
	}
}

func TestVerifySkipResource(t *testing.T) {
	color.NoColor = true
	for _, cache := range []bool{false, true} {
		ecspresso.InitVerifyState(cache)
		for i := 0; i < 3; i++ {
			out := extractStdout(t, func() {
				err := ecspresso.VerifyResource(context.TODO(), "skip resource", func(_ context.Context) error {
					return ecspresso.ErrSkipVerify("hello")
				})
				if err != nil {
					t.Error("unexpected error for skip resource", err)
				}
			})
			if !bytes.Contains(out, []byte("skip resource")) {
				t.Error("unexpected output for skip resource")
			}
			if cache && i >= 1 {
				if !bytes.Contains(out, []byte("[SKIP](cached) hello")) {
					t.Error("unexpected output (cached) for skip resource")
				}
			} else {
				if !bytes.Contains(out, []byte("[SKIP] hello")) {
					t.Error("unexpected output [SKIP] for skip resource")
				}
			}
		}
	}
}

func TestVerifierIsAssumed(t *testing.T) {
	cfg1 := aws.Config{}
	cfg2 := aws.Config{}
	var testCases = []struct {
		exec      *aws.Config
		app       *aws.Config
		isAssumed bool
	}{
		{&cfg1, &cfg2, true},
		{&cfg1, &cfg1, false},
		{&cfg2, &cfg2, false},
		{&cfg2, &cfg1, true},
	}
	for i, c := range testCases {
		v := ecspresso.NewVerifier(c.exec, c.app, &ecspresso.VerifyOption{})
		if v.IsAssumed() != c.isAssumed {
			t.Errorf("unexpected IsAssumed %d expected:%v got:%v", i, c.isAssumed, v.IsAssumed())
		}
	}
}
