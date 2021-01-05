package registry_test

import (
	"testing"

	"github.com/kayac/ecspresso/registry"
)

var testImages = []struct {
	image string
	tag   string
}{
	{image: "debian", tag: "latest"},
	{image: "katsubushi/katsubushi", tag: "v1.6.0"},
	{image: "public.ecr.aws/mackerel/mackerel-container-agent", tag: "plugins"},
	{image: "gcr.io/kaniko-project/executor", tag: "v0.10.0"},
	{image: "ghcr.io/github/super-linter", tag: "v3"},
}

func TestImages(t *testing.T) {
	for _, c := range testImages {
		t.Logf("testing %s:%s", c.image, c.tag)
		client := registry.New(c.image, "", "")
		if ok, err := client.HasImage(c.tag); err != nil {
			t.Errorf("%s:%s error %s", c.image, c.tag, err)
		} else if !ok {
			t.Errorf("%s:%s not found", c.image, c.tag)
		}
	}
}
