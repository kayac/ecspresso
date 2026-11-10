package ecspresso_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v2"
)

const wantHookDetails = `{
  "baseUrl": "https://example.com",
  "X-Header": "Keep-Case",
  "ratio": 0.5,
  "count": 3,
  "weight": 1,
  "id": 9007199254740993,
  "enabled": true,
  "empty": [],
  "checks": [{"path": "/health_check", "status": 200}]
}`

func decodeJSON(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("failed to decode %s: %s", string(b), err)
	}
	return v
}

func lifecycleHooksOf(t *testing.T, b []byte) []map[string]any {
	t.Helper()
	m, _ := decodeJSON(t, b).(map[string]any)
	dc, _ := m["deploymentConfiguration"].(map[string]any)
	hs, _ := dc["lifecycleHooks"].([]any)
	var hooks []map[string]any
	for _, h := range hs {
		hook, _ := h.(map[string]any)
		hooks = append(hooks, hook)
	}
	if len(hooks) != 2 {
		t.Fatalf("expected 2 lifecycleHooks, got %d in %s", len(hooks), string(b))
	}
	return hooks
}

func checkHookDetails(t *testing.T, b []byte) {
	t.Helper()
	hooks := lifecycleHooksOf(t, b)
	want := decodeJSON(t, []byte(wantHookDetails))
	if diff := cmp.Diff(want, hooks[0]["hookDetails"]); diff != "" {
		t.Errorf("hookDetails mismatch (-want +got):\n%s", diff)
	}
	if v, ok := hooks[1]["hookDetails"]; ok {
		t.Errorf("unexpected hookDetails in the hook without hookDetails: %v", v)
	}
}

func TestExtractHookDetails(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
		// hookDetails by the index of the hook, in JSON. Numbers are kept as written, in strings.
		details map[int]string
	}{
		{
			name: "no deploymentConfiguration",
			src:  `{"desiredCount": 1, "tags": [{"key": "hookDetails", "value": "x"}]}`,
			want: `{"desiredCount": 1, "tags": [{"key": "hookDetails", "value": "x"}]}`,
		},
		{
			name: "no hookDetails",
			src:  `{"deploymentConfiguration": {"lifecycleHooks": [{"roleArn": "a"}]}}`,
			want: `{"deploymentConfiguration": {"lifecycleHooks": [{"roleArn": "a"}]}}`,
		},
		{
			name: "not a service definition",
			src:  `[{"deploymentConfiguration": {"lifecycleHooks": [{"hookDetails": 1}]}}]`,
			want: `[{"deploymentConfiguration": {"lifecycleHooks": [{"hookDetails": 1}]}}]`,
		},
		{
			name: "the rest is kept as it is",
			src: `{ "desiredCount":1, "DesiredCount":2,
  "deploymentConfiguration" : {"lifecycleHooks": [
    {"roleArn": "a"},
    {"roleArn": "b", "RoleArn": "c", "hookDetails" :  {"x": [1.0, {"y": null}]} , "targetType": "AWS_LAMBDA"},
    {"hookDetails": "str"}, {"hookDetails": null}
  ]}, "hookDetails": {"top": "level"}}`,
			want: `{ "desiredCount":1, "DesiredCount":2,
  "deploymentConfiguration" : {"lifecycleHooks": [
    {"roleArn": "a"},
    {"roleArn": "b", "RoleArn": "c", "hookDetails" :  null , "targetType": "AWS_LAMBDA"},
    {"hookDetails": null}, {"hookDetails": null}
  ]}, "hookDetails": {"top": "level"}}`,
			details: map[int]string{1: `{"x":["1.0",{"y":null}]}`, 2: `"str"`},
		},
		{
			name:    "keys are case-insensitive",
			src:     `{"DeploymentConfiguration":{"LIFECYCLEHOOKS":[{"HookDetails":{"a":1}}]}}`,
			want:    `{"DeploymentConfiguration":{"LIFECYCLEHOOKS":[{"HookDetails":null}]}}`,
			details: map[int]string{0: `{"a":"1"}`},
		},
		{
			name:    "escaped key",
			src:     `{"deploymentConfiguration":{"lifecycleHooks":[{"hook\u0044etails":{"a":1}}]}}`,
			want:    `{"deploymentConfiguration":{"lifecycleHooks":[{"hook\u0044etails":null}]}}`,
			details: map[int]string{0: `{"a":"1"}`},
		},
		{
			name:    "the later hookDetails wins",
			src:     `{"deploymentConfiguration":{"lifecycleHooks":[{"hookDetails":{"a":1},"HookDetails":{"b":2}},{"hookDetails":{"c":3},"HookDetails":null}]}}`,
			want:    `{"deploymentConfiguration":{"lifecycleHooks":[{"hookDetails":null,"HookDetails":null},{"hookDetails":null,"HookDetails":null}]}}`,
			details: map[int]string{0: `{"b":"2"}`},
		},
		{
			name:    "the later lifecycleHooks is merged by index",
			src:     `{"deploymentConfiguration":{"lifecycleHooks":[{"hookDetails":1},{"hookDetails":2},{"hookDetails":3}]},"DeploymentConfiguration":{"lifecycleHooks":[{"roleArn":"a"},{"hookDetails":4}]}}`,
			want:    `{"deploymentConfiguration":{"lifecycleHooks":[{"hookDetails":null},{"hookDetails":null},{"hookDetails":null}]},"DeploymentConfiguration":{"lifecycleHooks":[{"roleArn":"a"},{"hookDetails":null}]}}`,
			details: map[int]string{0: `"1"`, 1: `"4"`},
		},
		{
			name: "the later null clears deploymentConfiguration",
			src:  `{"deploymentConfiguration":{"lifecycleHooks":[{"hookDetails":{"a":1}}]},"DeploymentConfiguration":null}`,
			want: `{"deploymentConfiguration":{"lifecycleHooks":[{"hookDetails":null}]},"DeploymentConfiguration":null}`,
		},
		{
			name: "the later null clears lifecycleHooks",
			src:  `{"deploymentConfiguration":{"lifecycleHooks":[{"hookDetails":{"a":1}}],"LifecycleHooks":null}}`,
			want: `{"deploymentConfiguration":{"lifecycleHooks":[{"hookDetails":null}],"LifecycleHooks":null}}`,
		},
		{
			name:    "the later deploymentConfiguration without lifecycleHooks keeps them",
			src:     `{"deploymentConfiguration":{"lifecycleHooks":[{"hookDetails":{"a":1}}]},"DeploymentConfiguration":{"strategy":"BLUE_GREEN"}}`,
			want:    `{"deploymentConfiguration":{"lifecycleHooks":[{"hookDetails":null}]},"DeploymentConfiguration":{"strategy":"BLUE_GREEN"}}`,
			details: map[int]string{0: `{"a":"1"}`},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, details, err := ecspresso.ExtractHookDetails([]byte(c.src))
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, string(b)); diff != "" {
				t.Errorf("source mismatch (-want +got):\n%s", diff)
			}
			got := map[int]string{}
			for i, v := range details {
				d, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				got[i] = string(d)
			}
			want := c.details
			if want == nil {
				want = map[int]string{}
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("hookDetails mismatch (-want +got):\n%s", diff)
			}
		})
	}
	for _, src := range []string{``, `{`, `{"deploymentConfiguration":{"lifecycleHooks":[{"hookDetails":}]}}`} {
		if _, _, err := ecspresso.ExtractHookDetails([]byte(src)); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}

func loadHookDetailsServiceDefinition(t *testing.T, path string) *ecspresso.Service {
	t.Helper()
	app, err := ecspresso.New(t.Context(), &ecspresso.CLIOptions{ConfigFilePath: "tests/test.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	sv, err := app.LoadServiceDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	return sv
}

type fakeECSServer struct {
	*httptest.Server
	mu     sync.Mutex
	bodies map[string][]byte
}

func newFakeECSServer(t *testing.T) *fakeECSServer {
	t.Helper()
	s := &fakeECSServer{bodies: map[string][]byte{}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, op, _ := strings.Cut(r.Header.Get("X-Amz-Target"), ".")
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.bodies[op] = b
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		_, _ = io.WriteString(w, "{}")
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *fakeECSServer) body(op string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bodies[op]
}

func (s *fakeECSServer) client() *ecs.Client {
	return ecs.New(ecs.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(s.URL),
		Credentials:  aws.AnonymousCredentials{},
	})
}

func TestHookDetailsRequestBody(t *testing.T) {
	sv := loadHookDetailsServiceDefinition(t, "tests/sv-hookdetails.json")
	s := newFakeECSServer(t)
	c := s.client()

	t.Run("UpdateService", func(t *testing.T) {
		in := ecspresso.SvToUpdateServiceInput(sv)
		in.Service = aws.String("test")
		if _, err := c.UpdateService(t.Context(), in); err != nil {
			t.Fatal(err)
		}
		body := s.body("UpdateService")
		checkHookDetails(t, body)
		// numbers are sent as written
		for _, n := range []string{`"weight":1.0`, `"id":9007199254740993`} {
			if !bytes.Contains(body, []byte(n)) {
				t.Errorf("expected request body to contain %s, got %s", n, string(body))
			}
		}
	})
	t.Run("CreateService", func(t *testing.T) {
		in := &ecs.CreateServiceInput{
			ServiceName:             aws.String("test"),
			DeploymentConfiguration: sv.DeploymentConfiguration,
		}
		if _, err := c.CreateService(t.Context(), in); err != nil {
			t.Fatal(err)
		}
		checkHookDetails(t, s.body("CreateService"))
	})
}
