package registry_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kayac/ecspresso/v2/registry"
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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if ok, err := client.HasImage(ctx, c.tag); err != nil {
			if isTemporary(err) {
				t.Logf("skip testing for %s: %s", c.image, err)
				continue
			}
			t.Errorf("%s:%s error %s", c.image, c.tag, err)
		} else if !ok {
			t.Errorf("%s:%s not found", c.image, c.tag)
		}
		if ok, err := client.HasPlatformImage(ctx, c.tag, c.arch, c.os); err != nil {
			if isTemporary(err) {
				t.Logf("skip testing for %s: %s", c.image, err)
				continue
			}
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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if ok, err := client.HasImage(ctx, c.tag); err == nil {
			t.Errorf("HasImage %s:%s error %s", c.image, c.tag, err)
		} else if ok {
			t.Errorf("HasImage %s:%s should not be found", c.image, c.tag)
		}
		if ok, err := client.HasPlatformImage(ctx, c.tag, c.arch, c.os); err == nil {
			t.Errorf("HasPlatformImage %s:%s %s/%s error %s", c.image, c.tag, c.arch, c.os, err)
		} else if ok {
			t.Errorf("HasPlatformImage %s:%s %s/%s should not be found", c.image, c.tag, c.arch, c.os)
		}
	}
}

func isTemporary(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		// timed out
		return true
	}
	if strings.Contains(err.Error(), "503") {
		return true
	}
	return false
}
