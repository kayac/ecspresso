package registry_test

import (
	"testing"

	"github.com/kayac/ecspresso/registry"
)

var testImages = []struct {
	image string
	tag   string
	os    string
	arch  string
}{
	{image: "debian", tag: "latest", os: "linux", arch: "amd64"},
	{image: "ubuntu", tag: "latest", os: "linux", arch: "arm64"},
	{image: "katsubushi/katsubushi", tag: "v1.6.0", os: "linux", arch: "amd64"},
	{image: "ghcr.io/kayac/go-katsubushi", tag: "v1.6.2", os: "linux", arch: "arm64"},
	{image: "public.ecr.aws/mackerel/mackerel-container-agent", tag: "plugins", os: "linux", arch: "arm64"},
	{image: "gcr.io/kaniko-project/executor", tag: "v1.7.0", os: "linux", arch: "amd64"},
	{image: "ghcr.io/github/super-linter", tag: "v4", os: "linux", arch: "amd64"},
	{image: "mcr.microsoft.com/windows/servercore/iis", tag: "windowsservercore-ltsc2019", os: "windows", arch: "amd64"},
}

func TestImages(t *testing.T) {
	for _, c := range testImages {
		client := registry.New(c.image, "", "")
		if ok, err := client.HasImage(c.tag); err != nil {
			t.Errorf("NG %s:%s error %s", c.image, c.tag, err)
		} else if !ok {
			t.Errorf("NG %s:%s not found", c.image, c.tag)
		} else {
			t.Logf("OK %s:%s", c.image, c.tag)
		}

		if ok, err := client.HasImageFor(c.tag, c.os, c.arch); err != nil {
			t.Errorf("NG %s:%s for %s/%s error %s", c.image, c.tag, c.os, c.arch, err)
		} else if !ok {
			t.Errorf("NG %s:%s for %s/%s not found", c.image, c.tag, c.os, c.arch)
		} else {
			t.Logf("OK %s:%s for %s/%s", c.image, c.tag, c.os, c.arch)
		}
	}
}
