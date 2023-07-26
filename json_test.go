package ecspresso_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v2"
)

type testJSONStruct = struct {
	FooBarBaz string
	Options   map[string]string
	Nested    struct {
		FooBar string
	}
}

var testJSONValue = testJSONStruct{
	FooBarBaz: "foo",
	Options: map[string]string{
		"Foo": "xxx",
	},
	Nested: struct {
		FooBar string
	}{
		FooBar: "foobar",
	},
}

var testSuiteJSONForAPI = []struct {
	name    string
	json    string
	queries []string
}{
	{
		name: "no transform",
		json: `{"fooBarBaz": "foo","nested":{"fooBar":"foobar"},"options":{"Foo":"xxx"}}`,
	},
	{
		name:    "options only",
		json:    `{"Foo":"xxx"}`,
		queries: []string{".options"},
	},
	{
		name:    "del options",
		json:    `{"fooBarBaz": "foo","nested":{"fooBar":"foobar"}}`,
		queries: []string{"del(.options)"},
	},
	{
		name:    "multiple del queries",
		json:    `{"fooBarBaz": "foo"}`,
		queries: []string{"del(.options)", "del(.nested)"},
	},
}

func TestMarshalJSONForAPI(t *testing.T) {
	for _, s := range testSuiteJSONForAPI {
		t.Run(s.name, func(t *testing.T) {
			b, err := ecspresso.MarshalJSONForAPI(testJSONValue, s.queries...)
			if err != nil {
				t.Fatal(err)
			}
			var expected bytes.Buffer
			json.Indent(&expected, []byte(s.json), "", "  ")
			expected.WriteByte('\n') // json.MarshalIndent does not append newline
			if diff := cmp.Diff(expected.String(), string(b)); diff != "" {
				t.Errorf("unexpected json: %s", diff)
			}
		})
	}
}

func TestUnmarshalJSON(t *testing.T) {
	for _, s := range testSuiteJSONForAPI {
		if s.queries != nil {
			continue
		}
		t.Run(s.name, func(t *testing.T) {
			var v testJSONStruct
			err := ecspresso.UnmarshalJSONForStruct([]byte(s.json), &v, "")
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(v, testJSONValue); diff != "" {
				t.Errorf("unexpected json: %s", diff)
			}
		})
	}
}
