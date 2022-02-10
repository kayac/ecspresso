package registry

import (
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

func (c *Repository) HasImage(tag string) (bool, error) {
	ref, err := name.ParseReference(c.name + ":" + tag)
	if err != nil {
		return false, err
	}
	options := []remote.Option{}
	if c.user != "" || c.password != "" {
		basic := &authn.Basic{
			Username: c.user,
			Password: c.password,
		}
		options = append(options, remote.WithAuth(basic))
	}
	if _, err := remote.Image(ref, options...); err != nil {
		return false, err
	}
	return true, nil
}
