package ecspresso

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/itchyny/gojq"
)

func (d *App) OutputJSONForAPI(w io.Writer, v interface{}) error {
	b, err := MarshalJSONForAPI(v)
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	_, err = w.Write(b)
	return err
}

func MustMarshalJSONStringForAPI(v interface{}) string {
	b, err := MarshalJSONForAPI(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func MarshalJSONForAPI(v interface{}, queries ...string) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	m := map[string]interface{}{}
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

func UnmarshalJSONForStruct(src []byte, v interface{}, path string) error {
	m := map[string]interface{}{}
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

func walkMap(m map[string]interface{}, fn func(string) string) {
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
		case map[string]interface{}:
			switch strings.ToLower(key) {
			case "dockerlabels", "options":
				walkMap(value, nil) // do not rewrite keys for map[string]string
			default:
				walkMap(value, fn)
			}
		case []interface{}:
			if len(value) > 0 {
				walkArray(value, fn)
			} else {
				delete(m, newKey)
			}
		default:
		}
	}
}

func walkArray(a []interface{}, fn func(string) string) {
	for _, value := range a {
		switch value := value.(type) {
		case map[string]interface{}:
			walkMap(value, fn)
		case []interface{}:
			walkArray(value, fn)
		default:
		}
	}
}

func jqFilter(m map[string]interface{}, q string) (map[string]interface{}, error) {
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
		if m, ok = v.(map[string]interface{}); !ok {
			return nil, fmt.Errorf("query result is not map[string]interface{}: %v", v)
		}
	}
	return m, nil
}
