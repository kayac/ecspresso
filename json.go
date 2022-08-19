package ecspresso

import (
	"encoding/json"
	"strings"
)

func MarshalJSONForAPI(v interface{}) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	m := map[string]interface{}{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	walkMap(m, jsonKeyForAPI)
	bs, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	bs = append(bs, '\n')
	return bs, nil
}

func (d *App) UnmarshalJSONForStruct(src []byte, v interface{}, path string) error {
	m := map[string]interface{}{}
	if err := json.Unmarshal(src, &m); err != nil {
		return err
	}
	walkMap(m, jsonKeyForStruct)
	if b, err := json.Marshal(m); err != nil {
		return err
	} else {
		return d.unmarshalJSON(b, v, path)
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
		if value != nil {
			m[fn(key)] = value
		}
		switch value := value.(type) {
		case map[string]interface{}:
			walkMap(value, fn)
		case []interface{}:
			if len(value) > 0 {
				walkArray(value, fn)
			} else {
				delete(m, fn(key))
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
