package ecspresso_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/fatih/color"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v2"
)

const testHookDetails = `{
  "baseUrl": "https://example.com",
  "X-Header": "Keep-Case",
  "ratio": 0.5,
  "count": 3,
  "enabled": true,
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

// checkHookDetails checks the lifecycle hooks of tests/sv-hookdetails.json in b.
func checkHookDetails(t *testing.T, b []byte) {
	t.Helper()
	m, _ := decodeJSON(t, b).(map[string]any)
	dc, _ := m["deploymentConfiguration"].(map[string]any)
	hooks, _ := dc["lifecycleHooks"].([]any)
	if len(hooks) != 2 {
		t.Fatalf("expected 2 lifecycleHooks, got %d in %s", len(hooks), string(b))
	}
	want := decodeJSON(t, []byte(testHookDetails))
	if diff := cmp.Diff(want, hooks[0].(map[string]any)["hookDetails"]); diff != "" {
		t.Errorf("hookDetails mismatch (-want +got):\n%s", diff)
	}
	if v, ok := hooks[1].(map[string]any)["hookDetails"]; ok {
		t.Errorf("unexpected hookDetails in the hook without hookDetails: %v", v)
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

func TestExtractHookDetails(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    string // the same as src if empty
		details map[int]any
	}{
		{
			name: "no deploymentConfiguration",
			src:  `{"desiredCount": 1, "tags": [{"key": "hookDetails", "value": "x"}]}`,
		},
		{
			name: "no hookDetails",
			src:  `{"deploymentConfiguration": {"lifecycleHooks": [{"roleArn": "a"}]}}`,
		},
		{
			name: "null hookDetails",
			src:  `{"deploymentConfiguration": {"lifecycleHooks": [{"hookDetails": null}]}}`,
		},
		{
			name: "invalid JSON",
			src:  `{"deploymentConfiguration": `,
		},
		{
			name: "hookDetails",
			src: `{"desiredCount": 1, "deploymentConfiguration": {"lifecycleHooks": [
				{"roleArn": "a"},
				{"roleArn": "b", "hookDetails": {"X-Key": [1, {"y": null}]}},
				{"hookDetails": "str"}
			]}}`,
			want: `{"deploymentConfiguration":{"lifecycleHooks":[{"roleArn":"a"},{"roleArn":"b"},{}]},"desiredCount":1}`,
			details: map[int]any{
				1: map[string]any{"X-Key": []any{float64(1), map[string]any{"y": nil}}},
				2: "str",
			},
		},
		{
			name: "keys are case-sensitive",
			src:  `{"deploymentConfiguration":{"lifecycleHooks":[{"HookDetails":{"a":1}}]},"DeploymentConfiguration":{"LifecycleHooks":[{"hookDetails":{"a":1}}]}}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, details, err := ecspresso.ExtractHookDetails([]byte(c.src))
			if err != nil {
				t.Fatal(err)
			}
			want := c.want
			if want == "" {
				want = c.src
			}
			if diff := cmp.Diff(want, string(b)); diff != "" {
				t.Errorf("source mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(c.details, details); diff != "" {
				t.Errorf("hookDetails mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadServiceDefinitionHookDetails(t *testing.T) {
	for _, path := range []string{"tests/sv-hookdetails.json", "tests/sv-hookdetails.jsonnet"} {
		t.Run(path, func(t *testing.T) {
			sv := loadHookDetailsServiceDefinition(t, path)
			b, err := ecspresso.MarshalJSONForAPI(sv)
			if err != nil {
				t.Fatal(err)
			}
			checkHookDetails(t, b)
		})
	}
}

func TestMarshalJSONForAPIHookDetails(t *testing.T) {
	sv := loadHookDetailsServiceDefinition(t, "tests/sv-hookdetails.json")
	in := &ecs.UpdateServiceInput{DeploymentConfiguration: sv.DeploymentConfiguration}
	for name, v := range map[string]any{
		"*Service":                sv,
		"*ServiceForDiff":         &ecspresso.ServiceForDiff{UpdateServiceInput: in},
		"*ecs.UpdateServiceInput": in,
	} {
		t.Run(name, func(t *testing.T) {
			b, err := ecspresso.MarshalJSONForAPI(v)
			if err != nil {
				t.Fatal(err)
			}
			checkHookDetails(t, b)
		})
	}
	for name, v := range map[string]any{
		"nil *Service":          (*ecspresso.Service)(nil),
		"empty *ServiceForDiff": &ecspresso.ServiceForDiff{},
		"*TaskDefinitionInput":  &ecspresso.TaskDefinitionInput{},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ecspresso.MarshalJSONForAPI(v); err != nil {
				t.Error(err)
			}
		})
	}
}

// newHookDetailsTestClient returns an ECS client which sends no requests.
// It stores the request body into sent if sent is not nil,
// and lets the SDK deserialize response as the response body.
func newHookDetailsTestClient(sent *[]byte, response string) *ecs.Client {
	return ecs.New(ecs.Options{
		Region:      "us-east-1",
		Credentials: aws.AnonymousCredentials{},
		APIOptions: []func(*middleware.Stack) error{
			func(stack *middleware.Stack) error {
				return stack.Deserialize.Add(middleware.DeserializeMiddlewareFunc(
					"test",
					func(ctx context.Context, in middleware.DeserializeInput, _ middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
						if sent != nil {
							b, err := io.ReadAll(in.Request.(*smithyhttp.Request).GetStream())
							if err != nil {
								return middleware.DeserializeOutput{}, middleware.Metadata{}, err
							}
							*sent = b
						}
						return middleware.DeserializeOutput{
							RawResponse: &smithyhttp.Response{Response: &http.Response{
								StatusCode: http.StatusOK,
								Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
								Body:       io.NopCloser(strings.NewReader(response)),
							}},
						}, middleware.Metadata{}, nil
					},
				), middleware.After)
			},
		},
	})
}

func TestHookDetailsRequestBody(t *testing.T) {
	sv := loadHookDetailsServiceDefinition(t, "tests/sv-hookdetails.json")
	var sent []byte
	c := newHookDetailsTestClient(&sent, "{}")

	t.Run("UpdateService", func(t *testing.T) {
		if _, err := c.UpdateService(t.Context(), &ecs.UpdateServiceInput{
			Service:                 aws.String("test"),
			DeploymentConfiguration: sv.DeploymentConfiguration,
		}); err != nil {
			t.Fatal(err)
		}
		checkHookDetails(t, sent)
		// integers are not sent as floating point numbers
		for _, n := range []string{`"count":3`, `"status":200`} {
			if !bytes.Contains(sent, []byte(n)) {
				t.Errorf("expected request body to contain %s, got %s", n, string(sent))
			}
		}
	})
	t.Run("CreateService", func(t *testing.T) {
		if _, err := c.CreateService(t.Context(), &ecs.CreateServiceInput{
			ServiceName:             aws.String("test"),
			DeploymentConfiguration: sv.DeploymentConfiguration,
		}); err != nil {
			t.Fatal(err)
		}
		checkHookDetails(t, sent)
	})
}

// describeRemoteService returns the service of path as the SDK reads from DescribeServices,
// with hookDetails of the first lifecycle hook replaced.
func describeRemoteService(t *testing.T, path string, hookDetails any) *ecspresso.Service {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sv := decodeJSON(t, src).(map[string]any)
	hook := sv["deploymentConfiguration"].(map[string]any)["lifecycleHooks"].([]any)[0].(map[string]any)
	if hookDetails == nil {
		delete(hook, "hookDetails")
	} else {
		hook["hookDetails"] = hookDetails
	}
	sv["serviceName"] = "test"
	sv["serviceArn"] = "arn:aws:ecs:us-east-1:123456789012:service/default/test"
	b, err := json.Marshal(map[string]any{"services": []any{sv}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := newHookDetailsTestClient(nil, string(b)).DescribeServices(t.Context(), &ecs.DescribeServicesInput{
		Services: []string{"test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &ecspresso.Service{Service: out.Services[0]}
}

func TestDiffServicesHookDetails(t *testing.T) {
	noColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = noColor })
	const path = "tests/sv-hookdetails.json"
	same := decodeJSON(t, []byte(testHookDetails))
	changed := decodeJSON(t, []byte(strings.Replace(testHookDetails, "/health_check", "/old_health_check", 1)))

	// init writes the remote service definition with MarshalJSONForAPI
	t.Run("remote service definition", func(t *testing.T) {
		b, err := ecspresso.MarshalJSONForAPI(describeRemoteService(t, path, same))
		if err != nil {
			t.Fatal(err)
		}
		checkHookDetails(t, b)
	})

	cases := []struct {
		name        string
		remote      any
		jsonnet     bool
		wantDiff    bool
		wantContain string
	}{
		{name: "same", remote: same},
		{name: "same in jsonnet", remote: same, jsonnet: true},
		{name: "changed", remote: changed, wantDiff: true, wantContain: "/old_health_check"},
		{name: "changed in jsonnet", remote: changed, jsonnet: true, wantDiff: true, wantContain: "/old_health_check"},
		{name: "not in remote", remote: nil, wantDiff: true, wantContain: "hookDetails"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := new(bytes.Buffer)
			opt := &ecspresso.DiffOption{Jsonnet: c.jsonnet}
			opt.SetWriter(b)
			local := loadHookDetailsServiceDefinition(t, path)
			remote := describeRemoteService(t, path, c.remote)
			diff, err := ecspresso.DiffServices(t.Context(), local, remote, path, opt)
			if err != nil {
				t.Fatal(err)
			}
			if diff != c.wantDiff {
				t.Errorf("expected diff %v, got %v:\n%s", c.wantDiff, diff, b.String())
			}
			if c.wantContain != "" && !strings.Contains(b.String(), c.wantContain) {
				t.Errorf("expected diff to contain %q, got:\n%s", c.wantContain, b.String())
			}
		})
	}
}
