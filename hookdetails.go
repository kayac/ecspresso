package ecspresso

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
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
					} else {
						details[i] = toDocumentValue(x)
					}
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
func toDocumentValue(v any) any {
	switch v := v.(type) {
	case json.Number:
		return smithydocument.Number(v)
	case map[string]any:
		for k, e := range v {
			v[k] = toDocumentValue(e)
		}
	case []any:
		for i, e := range v {
			v[i] = toDocumentValue(e)
		}
	}
	return v
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
