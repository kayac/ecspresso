package ecspresso_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-jsonnet"
	"github.com/kayac/ecspresso/v3"
)

var testSrcJsonnet = `
local env = std.native("env");
local must_env = std.native("must_env");
{
  foo: env("FOO", "default"),
  bar: must_env("BAR"),
}
`

var testCaseJsonnetNativeFuncs = []struct {
	name        string
	env         map[string]string
	expected    map[string]string
	errExpected bool
}{
	{
		name: "env FOO not set",
		env: map[string]string{
			"BAR": "bar",
		},
		expected: map[string]string{
			"foo": "default",
			"bar": "bar",
		},
	},
	{
		name: "env FOO set",
		env: map[string]string{
			"FOO": "foo",
			"BAR": "bar",
		},
		expected: map[string]string{
			"foo": "foo",
			"bar": "bar",
		},
	},
	{
		name: "must_env BAR not set",
		env: map[string]string{
			"FOO": "foo",
		},
		errExpected: true,
	},
}

func TestJsonnetNativeFuncs(t *testing.T) {
	vm := jsonnet.MakeVM()
	for _, f := range ecspresso.DefaultJsonnetNativeFuncs() {
		vm.NativeFunction(f)
	}

	for _, c := range testCaseJsonnetNativeFuncs {
		t.Run(c.name, func(t *testing.T) {
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			out, err := vm.EvaluateAnonymousSnippet("test.jsonnet", testSrcJsonnet)
			if c.errExpected {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			} else if err != nil {
				t.Fatal(err)
			}
			var got map[string]string
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.expected, got); diff != "" {
				t.Errorf("(-expected, +got)\n%s", diff)
			}
		})
	}
}
