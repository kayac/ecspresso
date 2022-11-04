package registry_test

import (
	"context"
	"testing"

	"github.com/kayac/ecspresso/registry"
)

var testImages = []struct {
	image string
	tag   string
	arch  string
	os    string
}{
	{image: "debian", tag: "latest", arch: "arm64", os: "linux"},
	{image: "katsubushi/katsubushi", tag: "v1.6.0", arch: "amd64", os: "linux"},
	{image: "public.ecr.aws/mackerel/mackerel-container-agent", tag: "plugins", arch: "amd64", os: "linux"},
	{image: "gcr.io/kaniko-project/executor", tag: "v0.10.0", arch: "amd64", os: "linux"},
	{image: "ghcr.io/github/super-linter", tag: "v3", arch: "amd64", os: "linux"},
	{image: "mcr.microsoft.com/windows/servercore/iis", tag: "windowsservercore-ltsc2019", arch: "amd64", os: "windows"},
}

var testFailImages = []struct {
	image string
	tag   string
	arch  string
	os    string
}{
	{image: "debian", tag: "xxx", arch: "arm64", os: "linux"},
	{image: "katsubushi/katsubushi", tag: "xxx", arch: "amd64", os: "linux"},
	{image: "public.ecr.aws/mackerel/mackerel-container-agent", tag: "xxx", arch: "amd64", os: "linux"},
	{image: "gcr.io/kaniko-project/executor", tag: "xxx", arch: "amd64", os: "linux"},
	{image: "ghcr.io/github/super-linter", tag: "xxx", arch: "amd64", os: "linux"},
	{image: "mcr.microsoft.com/windows/servercore/iis", tag: "xxx", arch: "amd64", os: "windows"},
	{image: "xxx", tag: "xxx", arch: "xxx", os: "xxx"},
}

func TestImages(t *testing.T) {
	for _, c := range testImages {
		t.Logf("testing %s:%s", c.image, c.tag)
		client := registry.New(c.image, "", "")
		ctx := context.Background()
		if ok, err := client.HasImage(ctx, c.tag); err != nil {
			t.Errorf("%s:%s error %s", c.image, c.tag, err)
		} else if !ok {
			t.Errorf("%s:%s not found", c.image, c.tag)
		}
		if ok, err := client.HasPlatformImage(ctx, c.tag, c.arch, c.os); err != nil {
			t.Errorf("%s:%s %s/%s error %s", c.image, c.tag, c.arch, c.os, err)
		} else if !ok {
			t.Errorf("%s:%s %s/%s not found", c.image, c.tag, c.arch, c.os)
		}
	}
}

func TestFailImages(t *testing.T) {
	for _, c := range testFailImages {
		t.Logf("testing (will be fail) %s:%s", c.image, c.tag)
		client := registry.New(c.image, "", "")
		ctx := context.Background()
		if ok, err := client.HasImage(ctx, c.tag); err != nil {
			t.Errorf("%s:%s error %s", c.image, c.tag, err)
		} else if ok {
			t.Errorf("%s:%s should not be found", c.image, c.tag)
		}
		if ok, err := client.HasPlatformImage(ctx, c.tag, c.arch, c.os); err != nil {
			t.Errorf("%s:%s %s/%s error %s", c.image, c.tag, c.arch, c.os, err)
		} else if ok {
			t.Errorf("%s:%s %s/%s should not be found", c.image, c.tag, c.arch, c.os)
		}
	}
}
