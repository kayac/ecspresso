package registry

import (
	"log"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Repository represents a repository using Docker Registry API v2.
type Repository struct {
	client   *http.Client
	name     string
	user     string
	password string
}

// New creates a client for a repository.
func New(name, user, password string) *Repository {
	return &Repository{
		client:   &http.Client{},
		name:     name,
		user:     user,
		password: password,
	}
}

func (c *Repository) ref(tag string) (name.Reference, []remote.Option, error) {
	ref, err := name.ParseReference(c.name + ":" + tag)
	if err != nil {
		return nil, nil, err
	}
	options := []remote.Option{}
	if c.user != "" || c.password != "" {
		basic := &authn.Basic{
			Username: c.user,
			Password: c.password,
		}
		options = append(options, remote.WithAuth(basic))
	}
	return ref, options, nil
}

func (c *Repository) HasImage(tag string) (bool, error) {
	ref, opt, err := c.ref(tag)
	if err != nil {
		return false, err
	}
	if _, err := remote.Image(ref, opt...); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Repository) HasImageFor(tag, os, arch string) (bool, error) {
	ref, opt, err := c.ref(tag)
	if err != nil {
		return false, err
	}
	idx, err := remote.Index(ref, opt...)
	if err != nil {
		log.Println("index failed fallback to image")
		if img, err := remote.Image(ref, opt...); err == nil {
			log.Println("image also errored")
			return false, err
		} else {
			m, err := img.Manifest()
			if err != nil {
				return false, err
			}
			if m.Config.Platform.OS == os || m.Config.Platform.Architecture == arch {
				return true, nil
			}
		}
	}
	im, err := idx.IndexManifest()
	if err != nil {
		return false, err
	}
	for _, m := range im.Manifests {
		if m.Platform.OS == os && m.Platform.Architecture == arch {
			return true, nil
		}
	}
	return false, nil
}
