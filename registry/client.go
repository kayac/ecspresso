package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/shogo82148/go-retry"
)

const (
	dockerHubHost                      = "registry-1.docker.io"
	mediaTypeDockerSchema2ManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	mediaTypeDockerSchema2Manifest     = "application/vnd.docker.distribution.manifest.v2+json"
)

var (
	ErrDeprecatedManifest    = fmt.Errorf("deprecated image manifest")
	ErrPullRateLimitExceeded = fmt.Errorf("image pull rate limit exceeded")

	retryPolicy = retry.Policy{
		MinDelay: time.Second,
		MaxDelay: 5 * time.Second,
		MaxCount: 3,
	}
)

// Repository represents a repository using Docker Registry API v2.
type Repository struct {
	client   *http.Client
	host     string
	repo     string
	user     string
	password string
	token    string
}

// New creates a client for a repository.
func New(image, user, password string) *Repository {
	c := &Repository{
		client:   &http.Client{},
		user:     user,
		password: password,
	}
	p := strings.SplitN(image, "/", 2)
	if strings.Contains(p[0], ".") && len(p) >= 2 {
		// Docker registry v2 API
		c.host = p[0]
		c.repo = p[1]
	} else {
		// DockerHub
		if !strings.Contains(image, "/") {
			image = "library/" + image
		}
		c.host = dockerHubHost
		c.repo = image
	}
	return c
}

func (c *Repository) login(ctx context.Context, endpoint, service, scope string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	u.RawQuery = strings.Join([]string{
		"service=" + url.QueryEscape(service),
		"scope=" + url.QueryEscape(scope),
	}, "&")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	if c.user != "" && c.password != "" {
		req.SetBasicAuth(c.user, c.password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed %s", resp.Status)
	}
	dec := json.NewDecoder(resp.Body)
	var body struct {
		Token string `json:"Token"`
	}
	if err := dec.Decode(&body); err != nil {
		return err
	}
	if body.Token == "" {
		return fmt.Errorf("response does not contains token")
	}
	c.token = body.Token
	return nil
}

func (c *Repository) fetchManifests(ctx context.Context, method, tag string) (*http.Response, error) {
	u := fmt.Sprintf("https://%s/v2/%s/manifests/%s", c.host, c.repo, tag)
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", strings.Join([]string{
		mediaTypeDockerSchema2ManifestList,
		ocispec.MediaTypeImageIndex,
		mediaTypeDockerSchema2Manifest,
		ocispec.MediaTypeImageManifest}, ", "))
	c.setAuthHeader(req)
	return c.client.Do(req)
}

func (c *Repository) getAvailability(ctx context.Context, tag string) (*http.Response, error) {
	resp, err := c.fetchManifests(ctx, http.MethodHead, tag)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	return resp, nil
}

func (c *Repository) getManifests(ctx context.Context, tag string) (mediaType string, _ io.ReadCloser, _ error) {
	var resp *http.Response
	err := retryPolicy.Do(ctx, func() error {
		var err error
		resp, err = c.fetchManifests(ctx, http.MethodGet, tag)
		if err != nil {
			return err
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return ErrPullRateLimitExceeded
		} else if resp.StatusCode != http.StatusOK {
			return fmt.Errorf(resp.Status)
		}
		return nil
	})
	if err != nil || resp == nil {
		return "", nil, fmt.Errorf("failed to fetch manifests: %w", err)
	}
	mediaType = parseContentType(resp.Header.Get("Content-Type"))
	return mediaType, resp.Body, nil
}

func (c *Repository) getImageConfig(ctx context.Context, digest string) (io.ReadCloser, error) {
	u := fmt.Sprintf("https://%s/v2/%s/blobs/%s", c.host, c.repo, digest)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.container.image.v1+json",
		ocispec.MediaTypeImageConfig,
	}, ", "))
	c.setAuthHeader(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf(resp.Status)
	}
	return resp.Body, err
}

func (c *Repository) setAuthHeader(req *http.Request) {
	if c.user == "AWS" && c.password != "" {
		// ECR
		req.Header.Set("Authorization", "Basic "+c.password)
	} else if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func parseContentType(contentType string) (mediaType string) {
	mediaType = contentType
	if i := strings.IndexByte(contentType, ';'); i != -1 {
		mediaType = contentType[0:i]
	}
	return
}

func match(want, got string) bool {
	// if want is empty, skip verify
	return want == "" || want == got
}

// HasPlatformImage returns an image tag for arch/os exists or not in the repository.
func (c *Repository) HasPlatformImage(ctx context.Context, tag, arch, os string) (bool, error) {
	mediaType, rc, err := c.getManifests(ctx, tag)
	if err != nil {
		return false, err
	}
	defer rc.Close()
	dec := json.NewDecoder(rc)
	switch mediaType {
	case
		ocispec.MediaTypeImageIndex,
		mediaTypeDockerSchema2ManifestList:
		var manifestList ocispec.Index
		if err := dec.Decode(&manifestList); err != nil {
			return false, fmt.Errorf("manifest list decode error: %w", err)
		}
		// https://github.com/opencontainers/image-spec/blob/main/image-index.md#image-index-property-descriptions
		for _, desc := range manifestList.Manifests {
			p := desc.Platform
			if p == nil {
				// regard as non platform-specific image
				return true, nil
			}
			if match(arch, p.Architecture) && match(os, p.OS) {
				return true, nil
			}
		}
	case
		mediaTypeDockerSchema2Manifest,
		ocispec.MediaTypeImageManifest:
		var manifest ocispec.Manifest
		if err := dec.Decode(&manifest); err != nil {
			return false, fmt.Errorf("manifest decode error: %w", err)
		}
		if p := manifest.Config.Platform; p != nil {
			if match(arch, p.OS) && match(os, p.Architecture) {
				return true, nil
			}
		}

		// fallback to image config
		// https://github.com/opencontainers/image-spec/blob/main/config.md#properties
		rc, err := c.getImageConfig(ctx, manifest.Config.Digest.String())
		if err != nil {
			return false, err
		}
		defer rc.Close()
		var image ocispec.Image
		if err := json.NewDecoder(rc).Decode(&image); err != nil {
			return false, fmt.Errorf("image config decode error: %w", err)
		}
		if match(arch, image.Architecture) && match(os, image.OS) {
			return true, nil
		}
	case
		// https://docs.docker.com/registry/spec/deprecated-schema-v1/
		"application/vnd.docker.distribution.manifest.v1+prettyjws",
		"application/vnd.docker.distribution.manifest.v1+json":
		return false, ErrDeprecatedManifest
	default:
		return false, fmt.Errorf("unknown MediaType %s", mediaType)
	}
	// not found
	return false, nil
}

// HasImage returns an image tag exists or not in the repository.
func (c *Repository) HasImage(ctx context.Context, tag string) (bool, error) {
	tries := 2
	for tries > 0 {
		tries--
		resp, err := c.getAvailability(ctx, tag)
		if err != nil {
			return false, err
		}
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			h := resp.Header.Get("Www-Authenticate")
			if strings.HasPrefix(h, "Bearer ") {
				auth := strings.SplitN(h, " ", 2)[1]
				e, svc, scope := parseAuthHeader(auth)
				if err := c.login(ctx, e, svc, scope); err != nil {
					return false, err
				}
			}
		case http.StatusNotFound:
			return false, nil
		case http.StatusOK:
			return true, nil
		default:
			return false, fmt.Errorf(resp.Status)
		}
	}
	return false, fmt.Errorf("aborted")
}

var (
	partRegexp = regexp.MustCompile(`[a-zA-Z0-9_]+="[^"]*"`)
)

func parseAuthHeader(bearer string) (endpoint, service, scope string) {
	parsed := make(map[string]string, 3)
	for _, part := range partRegexp.FindAllString(bearer, -1) {
		kv := strings.SplitN(part, "=", 2)
		parsed[kv[0]] = kv[1][1 : len(kv[1])-1]
	}
	return parsed["realm"], parsed["service"], parsed["scope"]
}
