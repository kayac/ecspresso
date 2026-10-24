package registry_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kayac/ecspresso/v2/registry"
)

var testImages = []struct {
	image       string
	tagOrDigest string
	arch        string
	os          string
}{
	{image: "debian", tagOrDigest: "latest", arch: "arm64", os: "linux"},
	{image: "debian", tagOrDigest: "sha256:8f6a88feef3ed01a300dafb87f208977f39dccda1fd120e878129463f7fa3b8f", arch: "arm64", os: "linux"},
	{image: "katsubushi/katsubushi", tagOrDigest: "v1.6.0", arch: "amd64", os: "linux"},
	{image: "katsubushi/katsubushi", tagOrDigest: "sha256:9426deec3ed0c1f41f070509c4e9729ac32d65920b919f0adc09809812c0f247", arch: "amd64", os: "linux"},
	{image: "public.ecr.aws/mackerel/mackerel-container-agent", tagOrDigest: "plugins", arch: "amd64", os: "linux"},
	{image: "public.ecr.aws/mackerel/mackerel-container-agent", tagOrDigest: "sha256:c7d960d3f87cc9f9b6e5afee8ad2fe5d0af14ec70d5c089ec1a77304ccb40e2d", arch: "amd64", os: "linux"},
	{image: "gcr.io/kaniko-project/executor", tagOrDigest: "v0.10.0", arch: "amd64", os: "linux"},
	{image: "gcr.io/kaniko-project/executor", tagOrDigest: "sha256:78d44ec4e9cb5545d7f85c1924695c89503ded86a59f92c7ae658afa3cff5400", arch: "amd64", os: "linux"},
	{image: "ghcr.io/github/super-linter", tagOrDigest: "v3", arch: "amd64", os: "linux"},
	{image: "ghcr.io/github/super-linter", tagOrDigest: "sha256:b1a1417c30483783987536fd8beb64d85f2509d27b5fb72afbe327a22f0ed59b", arch: "amd64", os: "linux"},
	{image: "mcr.microsoft.com/windows/servercore/iis", tagOrDigest: "windowsservercore-ltsc2019", arch: "amd64", os: "windows"},
	{image: "mcr.microsoft.com/windows/servercore/iis", tagOrDigest: "sha256:c077419bb006eb5c9a0fe3e5a9cdf018cdcc2b50c5ea11ad39ffbecf93fd3f07", arch: "amd64", os: "windows"},
}

var testFailImages = []struct {
	image       string
	tagOrDigest string
	arch        string
	os          string
}{
	{image: "debian", tagOrDigest: "xxx", arch: "arm64", os: "linux"},
	{image: "debian", tagOrDigest: "sha256:xxx", arch: "arm64", os: "linux"},
	{image: "katsubushi/katsubushi", tagOrDigest: "xxx", arch: "amd64", os: "linux"},
	{image: "public.ecr.aws/mackerel/mackerel-container-agent", tagOrDigest: "xxx", arch: "amd64", os: "linux"},
	{image: "gcr.io/kaniko-project/executor", tagOrDigest: "xxx", arch: "amd64", os: "linux"},
	{image: "ghcr.io/github/super-linter", tagOrDigest: "xxx", arch: "amd64", os: "linux"},
	{image: "mcr.microsoft.com/windows/servercore/iis", tagOrDigest: "xxx", arch: "amd64", os: "windows"},
	{image: "xxx", tagOrDigest: "xxx", arch: "xxx", os: "xxx"},
}

func TestImages(t *testing.T) {
	for _, c := range testImages {
		var fullImage string
		if strings.Contains(c.tagOrDigest, ":") {
			fullImage = fmt.Sprintf("%s@%s", c.image, c.tagOrDigest)
		} else {
			fullImage = fmt.Sprintf("%s:%s", c.image, c.tagOrDigest)
		}
		t.Logf("testing %s", fullImage)
		client := registry.New(c.image, "", "")
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		if ok, err := client.HasImage(ctx, c.tagOrDigest); err != nil {
			if isTemporary(err) {
				t.Logf("skip testing for %s: %s", c.image, err)
				continue
			}
			t.Errorf("%s error %s", fullImage, err)
		} else if !ok {
			t.Errorf("%s not found", fullImage)
		}
		if ok, err := client.HasPlatformImage(ctx, c.tagOrDigest, c.arch, c.os); err != nil {
			if isTemporary(err) {
				t.Logf("skip testing for %s: %s", c.image, err)
				continue
			}
			t.Errorf("%s %s/%s error %s", fullImage, c.arch, c.os, err)
		} else if !ok {
			t.Errorf("%s %s/%s not found", fullImage, c.arch, c.os)
		}
	}
}

func TestFailImages(t *testing.T) {
	for _, c := range testFailImages {
		var fullImage string
		if strings.Contains(c.tagOrDigest, ":") {
			fullImage = fmt.Sprintf("%s@%s", c.image, c.tagOrDigest)
		} else {
			fullImage = fmt.Sprintf("%s:%s", c.image, c.tagOrDigest)
		}
		t.Logf("testing (will be fail) %s", fullImage)
		client := registry.New(c.image, "", "")
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		if ok, err := client.HasImage(ctx, c.tagOrDigest); err == nil {
			t.Errorf("HasImage %s error %s", fullImage, err)
		} else if ok {
			t.Errorf("HasImage %s should not be found", fullImage)
		}
		if ok, err := client.HasPlatformImage(ctx, c.tagOrDigest, c.arch, c.os); err == nil {
			t.Errorf("HasPlatformImage %s %s/%s error %s", fullImage, c.arch, c.os, err)
		} else if ok {
			t.Errorf("HasPlatformImage %s %s/%s should not be found", fullImage, c.arch, c.os)
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
	if errors.Is(err, registry.ErrPullRateLimitExceeded) {
		// shared CI runner IPs often hit anonymous pull rate limits
		return true
	}
	return false
}
