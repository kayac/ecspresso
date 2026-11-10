package ecspresso

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs/document"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	smithydocument "github.com/aws/smithy-go/document"
)

// DeploymentLifecycleHook.HookDetails is a document.Interface, which encoding/json can neither decode nor encode.
// So hookDetails is handled separately from the rest of the service definition.
// See https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs/document

// eachChild calls fn for each member or element of the JSON object or array in b, with the offset of the value in b.
func eachChild(b []byte, fn func(key string, index int, value []byte, offset int) error) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	t, err := dec.Token()
	if err != nil {
		return err
	}
	open, _ := t.(json.Delim)
	if open != '{' && open != '[' {
		return nil
	}
	for i := 0; dec.More(); i++ {
		var key string
		if open == '{' {
			t, err := dec.Token()
			if err != nil {
				return err
			}
			key, _ = t.(string)
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return err
		}
		if err := fn(key, i, v, int(dec.InputOffset())-len(v)); err != nil {
			return err
		}
	}
	return nil
}

// extractHookDetails replaces hookDetails of the lifecycle hooks in src with null, and returns
// them by the index of the hook. The rest of src is kept as it is.
// Keys are matched case-insensitively and the later one wins, as encoding/json decodes structs.
func extractHookDetails(src []byte) ([]byte, map[int]any, error) {
	type span struct{ start, end int }
	var spans []span
	details := map[int]any{}
	err := eachChild(src, func(key string, _ int, dc []byte, dcOffset int) error {
		if !strings.EqualFold(key, "deploymentConfiguration") {
			return nil
		}
		if string(dc) == "null" {
			clear(details)
			return nil
		}
		return eachChild(dc, func(key string, _ int, hooks []byte, hooksOffset int) error {
			if !strings.EqualFold(key, "lifecycleHooks") {
				return nil
			}
			var n int
			err := eachChild(hooks, func(_ string, i int, hook []byte, hookOffset int) error {
				n = i + 1
				return eachChild(hook, func(key string, _ int, v []byte, offset int) error {
					if !strings.EqualFold(key, "hookDetails") {
						return nil
					}
					start := dcOffset + hooksOffset + hookOffset + offset
					spans = append(spans, span{start, start + len(v)})
					dec := json.NewDecoder(bytes.NewReader(v))
					dec.UseNumber()
					var x any
					if err := dec.Decode(&x); err != nil {
						return err
					}
					if x == nil {
						delete(details, i)
						return nil
					}
					d, err := toDocumentValue(x)
					if err != nil {
						return fmt.Errorf("failed to load hookDetails of lifecycleHooks[%d]: %w", i, err)
					}
					details[i] = d
					return nil
				})
			})
			// lifecycleHooks in another case replaces the hooks, but keeps the fields of the hooks at the same index.
			maps.DeleteFunc(details, func(i int, _ any) bool { return i >= n })
			return err
		})
	})
	if err != nil {
		return nil, nil, err
	}
	if len(spans) == 0 {
		return src, nil, nil
	}
	for _, i := range slices.Sorted(maps.Keys(details)) {
		for _, n := range inexactNumbers(details[i]) {
			LogWarn("number in hookDetails is not exactly representable as float64: render and diff show it rounded, so diff may not detect its change and deploy may not update it. use a string for such a value",
				"lifecycleHook", i, "number", n)
		}
	}
	b := make([]byte, 0, len(src))
	var pos int
	for _, sp := range spans {
		b = append(b, src[pos:sp.start]...)
		b = append(b, "null"...)
		pos = sp.end
	}
	b = append(b, src[pos:]...)
	return b, details, nil
}

// The document encoder writes json.Number as a JSON string.
func toDocumentValue(v any) (any, error) {
	switch v := v.(type) {
	case json.Number:
		if _, err := v.Float64(); err != nil {
			// The SDK fails to read DescribeServices responses with such a number.
			return nil, fmt.Errorf("number %s is out of the range of float64", v)
		}
		return smithydocument.Number(v), nil
	case map[string]any:
		for k, e := range v {
			if k == "" {
				// The document encoder of the SDK fails with it when the request is sent.
				return nil, fmt.Errorf("key must not be empty")
			}
			d, err := toDocumentValue(e)
			if err != nil {
				return nil, err
			}
			v[k] = d
		}
	case []any:
		for i, e := range v {
			d, err := toDocumentValue(e)
			if err != nil {
				return nil, err
			}
			v[i] = d
		}
	}
	return v, nil
}

func inexactNumbers(v any) []string {
	var ns []string
	switch v := v.(type) {
	case smithydocument.Number:
		if !isFloat64Number(json.Number(v)) {
			ns = append(ns, string(v))
		}
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(v)) {
			ns = append(ns, inexactNumbers(v[k])...)
		}
	case []any:
		for _, e := range v {
			ns = append(ns, inexactNumbers(e)...)
		}
	}
	return ns
}

func isFloat64Number(n json.Number) bool {
	f, err := n.Float64()
	if err != nil {
		return false
	}
	want, ok := new(big.Rat).SetString(n.String())
	if !ok {
		return false
	}
	if exact := new(big.Rat).SetFloat64(f); exact != nil && want.Cmp(exact) == 0 {
		return true
	}
	// e.g. 0.1 is not the exact value of a float64, but is the shortest form which is read back as it.
	shortest, ok := new(big.Rat).SetString(strconv.FormatFloat(f, 'g', -1, 64))
	return ok && want.Cmp(shortest) == 0
}

func setHookDetails(dc *types.DeploymentConfiguration, details map[int]any) error {
	var n int
	if dc != nil {
		n = len(dc.LifecycleHooks)
	}
	for _, i := range slices.Sorted(maps.Keys(details)) {
		if i >= n {
			return fmt.Errorf("failed to set hookDetails of lifecycleHooks[%d]: %d lifecycle hooks are loaded", i, n)
		}
		dc.LifecycleHooks[i].HookDetails = document.NewLazyDocument(details[i])
	}
	return nil
}

var deploymentConfigurationType = reflect.TypeFor[*types.DeploymentConfiguration]()

// deploymentConfigurationOf finds the DeploymentConfiguration field of v or of its embedded structs.
func deploymentConfigurationOf(v reflect.Value) *types.DeploymentConfiguration {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Name == "DeploymentConfiguration" && f.Type == deploymentConfigurationType {
			dc, _ := v.Field(i).Interface().(*types.DeploymentConfiguration)
			return dc
		}
	}
	for i := range t.NumField() {
		if t.Field(i).Anonymous {
			if dc := deploymentConfigurationOf(v.Field(i)); dc != nil {
				return dc
			}
		}
	}
	return nil
}

// Numbers are read as float64 as the SDK reads DescribeServices responses,
// so that diff compares local and remote values in the same form.
func hookDetailsValue(d document.Interface) (any, error) {
	b, err := d.MarshalSmithyDocument()
	if err != nil {
		return nil, err
	}
	var x any
	if err := json.Unmarshal(b, &x); err != nil {
		return nil, err
	}
	return x, nil
}

// fillHookDetailsJSON runs after walkMap to keep the keys in hookDetails as they are.
func fillHookDetailsJSON(v any, m map[string]any) error {
	dc := deploymentConfigurationOf(reflect.ValueOf(v))
	if dc == nil {
		return nil
	}
	dcm, _ := m["deploymentConfiguration"].(map[string]any)
	hooks, _ := dcm["lifecycleHooks"].([]any)
	for i, h := range dc.LifecycleHooks {
		if i >= len(hooks) {
			break
		}
		hook, ok := hooks[i].(map[string]any)
		if !ok {
			continue
		}
		delete(hook, "hookDetails")
		if h.HookDetails == nil {
			continue
		}
		x, err := hookDetailsValue(h.HookDetails)
		if err != nil {
			return fmt.Errorf("failed to marshal hookDetails of lifecycleHooks[%d]: %w", i, err)
		}
		if x != nil {
			hook["hookDetails"] = x
		}
	}
	return nil
}
