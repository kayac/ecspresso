package ecspresso

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/itchyny/gojq"
)

func OutputJSONForAPI(w io.Writer, v any) (int, error) {
	b, err := MarshalJSONForAPI(v)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal json: %w", err)
	}
	return w.Write(b)
}

func MustMarshalJSONStringForAPI(v any) string {
	b, err := MarshalJSONForAPI(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func MarshalJSONForAPI(v any, queries ...string) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	walkMap(m, jsonKeyForAPI)
	if len(queries) > 0 {
		for _, q := range queries {
			if m, err = jqFilter(m, q); err != nil {
				return nil, err
			}
		}
	}
	bs, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	bs = append(bs, '\n')
	return bs, nil
}

func UnmarshalJSONForStruct(src []byte, v any, path string) error {
	m := map[string]any{}
	if err := json.Unmarshal(src, &m); err != nil {
		return err
	}
	walkMap(m, jsonKeyForStruct)
	if b, err := json.Marshal(m); err != nil {
		return err
	} else {
		return unmarshalJSON(b, v, path)
	}
}

func jsonKeyForAPI(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func jsonKeyForStruct(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func walkMap(m map[string]any, fn func(string) string) {
	for key, value := range m {
		delete(m, key)
		newKey := key
		if fn != nil {
			newKey = fn(key)
		}
		if value != nil {
			m[newKey] = value
		}
		switch value := value.(type) {
		case map[string]any:
			switch strings.ToLower(key) {
			case "dockerlabels", "options":
				walkMap(value, nil) // do not rewrite keys for map[string]string
			default:
				walkMap(value, fn)
			}
		case []any:
			if len(value) > 0 {
				walkArray(value, fn)
			} else {
				delete(m, newKey)
			}
		default:
		}
	}
}

func walkArray(a []any, fn func(string) string) {
	for _, value := range a {
		switch value := value.(type) {
		case map[string]any:
			walkMap(value, fn)
		case []any:
			walkArray(value, fn)
		default:
		}
	}
}

func jqFilter(m map[string]any, q string) (map[string]any, error) {
	query, err := gojq.Parse(q)
	if err != nil {
		return nil, err
	}
	iter := query.Run(m)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, err
		}
		if m, ok = v.(map[string]any); !ok {
			return nil, fmt.Errorf("query result is not map[string]any: %v", v)
		}
	}
	return m, nil
}
