package dockerhub

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Repositry represents DockeHub repositry
type Repositry struct {
	client *http.Client
	token  string
	repo   string
}

// New creates a client for a repositry.
func New(repo string) (*Repositry, error) {
	c := &Repositry{
		client: &http.Client{},
		repo:   repo,
	}
	err := c.login()
	return c, err
}

func (c *Repositry) login() error {
	u := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", c.repo)
	resp, err := c.client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	var body struct {
		Token string
	}
	if err := dec.Decode(&body); err != nil {
		return err
	}
	if body.Token == "" {
		return errors.New("response does not contains token")
	}
	c.token = body.Token
	return nil
}

// HasImage returns an image tag exists or not in the repository.
func (c *Repositry) HasImage(tag string) (bool, error) {
	u := fmt.Sprintf("https://registry.hub.docker.com/v2/%s/manifests/%s", c.repo, tag)
	req, _ := http.NewRequest(http.MethodHead, u, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, errors.New(resp.Status)
}
