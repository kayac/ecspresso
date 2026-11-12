package registry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type fakeRegistry struct {
	// challenge is "Basic" or "Bearer"
	challenge string
	// user and password required by the registry (Basic) or the token endpoint (Bearer).
	// Empty means anonymous access is allowed.
	user, password string

	tokenAuth    []string // Authorization headers sent to the token endpoint
	manifestAuth []string // Authorization headers sent to the manifests endpoint
}

const fakeToken = "fake-token"

func (f *fakeRegistry) handler(srv **httptest.Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		f.tokenAuth = append(f.tokenAuth, r.Header.Get("Authorization"))
		if f.user != "" {
			if u, p, ok := r.BasicAuth(); !ok || u != f.user || p != f.password {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		w.Write([]byte(`{"token":"` + fakeToken + `"}`))
	})
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		f.manifestAuth = append(f.manifestAuth, auth)
		authorized := false
		switch f.challenge {
		case "Basic":
			u, p, ok := r.BasicAuth()
			authorized = ok && u == f.user && p == f.password
			if !authorized {
				w.Header().Set("Www-Authenticate", `Basic realm="`+(*srv).URL+`/",service="fake"`)
			}
		case "Bearer":
			authorized = auth == "Bearer "+fakeToken
			if !authorized {
				w.Header().Set("Www-Authenticate", `Bearer realm="`+(*srv).URL+`/token",service="fake",scope="aws"`)
			}
		}
		if !authorized {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
		w.Write([]byte(`{"manifests":[{"platform":{"architecture":"amd64","os":"linux"}}]}`))
	})
	return mux
}

func newFakeRepository(t *testing.T, f *fakeRegistry, user, password string) *Repository {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewTLSServer(f.handler(&srv))
	t.Cleanup(srv.Close)
	c := New(strings.TrimPrefix(srv.URL, "https://")+"/foo/bar", user, password)
	c.client = srv.Client()
	return c
}

func TestHasImageAuth(t *testing.T) {
	tests := []struct {
		name             string
		registry         *fakeRegistry
		user, password   string
		wantOK           bool
		wantTokenAuth    []string
		wantManifestAuth []string
	}{
		{
			name:             "basic challenge with credentials",
			registry:         &fakeRegistry{challenge: "Basic", user: "AWS", password: "secret"},
			user:             "AWS",
			password:         "secret",
			wantOK:           true,
			wantManifestAuth: []string{"", "Basic QVdTOnNlY3JldA=="},
		},
		{
			name:     "basic challenge without credentials",
			registry: &fakeRegistry{challenge: "Basic", user: "AWS", password: "secret"},
		},
		{
			name:             "bearer challenge with credentials",
			registry:         &fakeRegistry{challenge: "Bearer", user: "AWS", password: "secret"},
			user:             "AWS",
			password:         "secret",
			wantOK:           true,
			wantTokenAuth:    []string{"Basic QVdTOnNlY3JldA=="},
			wantManifestAuth: []string{"", "Bearer " + fakeToken},
		},
		{
			name:             "bearer challenge anonymous",
			registry:         &fakeRegistry{challenge: "Bearer"},
			wantOK:           true,
			wantTokenAuth:    []string{""},
			wantManifestAuth: []string{"", "Bearer " + fakeToken},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newFakeRepository(t, tt.registry, tt.user, tt.password)
			ok, err := c.HasImage(t.Context(), "latest")
			if tt.wantOK {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				if !ok {
					t.Fatal("image should be found")
				}
			} else if err == nil && ok {
				t.Fatal("image should not be found")
			}
			if tt.wantTokenAuth != nil {
				if diff := cmp.Diff(tt.wantTokenAuth, tt.registry.tokenAuth); diff != "" {
					t.Errorf("token auth mismatch (-want +got):\n%s", diff)
				}
			}
			if tt.wantManifestAuth != nil {
				if diff := cmp.Diff(tt.wantManifestAuth, tt.registry.manifestAuth); diff != "" {
					t.Errorf("manifest auth mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

// HasPlatformImage must authenticate by itself without calling HasImage first.
func TestHasPlatformImageAuth(t *testing.T) {
	tests := []struct {
		name             string
		registry         *fakeRegistry
		wantManifestAuth []string
	}{
		{
			name:             "basic challenge",
			registry:         &fakeRegistry{challenge: "Basic", user: "AWS", password: "secret"},
			wantManifestAuth: []string{"", "Basic QVdTOnNlY3JldA=="},
		},
		{
			name:             "bearer challenge",
			registry:         &fakeRegistry{challenge: "Bearer", user: "AWS", password: "secret"},
			wantManifestAuth: []string{"", "Bearer " + fakeToken},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newFakeRepository(t, tt.registry, "AWS", "secret")
			ok, err := c.HasPlatformImage(t.Context(), "latest", "amd64", "linux")
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if !ok {
				t.Fatal("platform image should be found")
			}
			if diff := cmp.Diff(tt.wantManifestAuth, tt.registry.manifestAuth); diff != "" {
				t.Errorf("manifest auth mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
