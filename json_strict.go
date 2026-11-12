package ecspresso

import (
	"bytes"
	"encoding"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// unmarshalJSON decodes src into v, which is a pointer to a struct.
//
// Keys of JSON objects are matched to the struct fields case-sensitively.
// A field without a json tag is matched by its name with the first letter in
// lower case, as the AWS APIs define (e.g. "desiredCount" for DesiredCount).
//
// A key that matches a field only case-insensitively is an error.
// Other unknown keys are ignored with a warning.
func unmarshalJSON(src []byte, v any, path string) error {
	b, err := normalizeJSONKeys(src, v, path)
	if err != nil {
		return err
	}
	return jsonv2.Unmarshal(b, v, jsonv2.RejectUnknownMembers(true))
}

// normalizeJSONKeys validates the keys in src to be decoded into v, and
// returns the JSON whose keys are rewritten to the names that
// encoding/json/v2 matches to the struct fields, without the unknown keys.
func normalizeJSONKeys(src []byte, v any, path string) ([]byte, error) {
	n := &keyNormalizer{dec: jsontext.NewDecoder(bytes.NewReader(src))}
	var buf bytes.Buffer
	n.enc = jsontext.NewEncoder(&buf)
	if err := n.normalize(reflect.TypeOf(v)); err != nil {
		return nil, err
	}
	if _, err := n.dec.ReadToken(); err != io.EOF {
		if err == nil {
			err = errors.New("unexpected value after the top-level value")
		}
		return nil, err
	}
	if len(n.miscased) > 0 {
		return nil, errors.Join(n.miscased...)
	}
	for _, ptr := range n.unknown {
		LogWarn("unknown field in definition", "field", ptr, "path", path)
	}
	return buf.Bytes(), nil
}

// keyNormalizer copies a JSON value from dec to enc, rewriting the keys of
// JSON objects to the names that encoding/json/v2 matches to the struct fields.
// Unknown keys are not copied.
type keyNormalizer struct {
	dec      *jsontext.Decoder
	enc      *jsontext.Encoder
	miscased []error  // keys that differ from a field only in case
	unknown  []string // JSON pointers to the keys that match no field
}

// normalize copies a JSON value which is to be decoded into t.
func (n *keyNormalizer) normalize(t reflect.Type) error {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || hasCustomUnmarshaler(t) {
		return n.copyValue()
	}
	switch kind := n.dec.PeekKind(); {
	case kind == '{' && t.Kind() == reflect.Struct:
		fields := jsonFieldsOf(t)
		return n.copyObject(func(key string) (string, reflect.Type, bool) {
			if f, ok := fields[key]; ok {
				return f.name, f.typ, true
			}
			ptr := string(n.dec.StackPointer())
			for name := range fields {
				if strings.EqualFold(name, key) {
					n.miscased = append(n.miscased, fmt.Errorf("unknown field %q at %s: keys are case-sensitive, use %q", key, ptr, name))
					return "", nil, false
				}
			}
			n.unknown = append(n.unknown, ptr)
			return "", nil, false
		})
	case kind == '{' && t.Kind() == reflect.Map:
		return n.copyObject(func(key string) (string, reflect.Type, bool) {
			return key, t.Elem(), true
		})
	case kind == '[' && (t.Kind() == reflect.Slice || t.Kind() == reflect.Array):
		return n.copyArray(t.Elem())
	default:
		return n.copyValue()
	}
}

// copyObject copies a JSON object. resolve returns the key to write and the
// type of the value for a key, or false to drop the member.
func (n *keyNormalizer) copyObject(resolve func(key string) (string, reflect.Type, bool)) error {
	if err := n.copyToken(); err != nil { // {
		return err
	}
	for n.dec.PeekKind() != '}' {
		tok, err := n.dec.ReadToken()
		if err != nil {
			return err
		}
		name, typ, ok := resolve(tok.String())
		if !ok {
			if err := n.dec.SkipValue(); err != nil {
				return err
			}
			continue
		}
		if err := n.enc.WriteToken(jsontext.String(name)); err != nil {
			return err
		}
		if err := n.normalize(typ); err != nil {
			return err
		}
	}
	return n.copyToken() // }
}

func (n *keyNormalizer) copyArray(elem reflect.Type) error {
	if err := n.copyToken(); err != nil { // [
		return err
	}
	for n.dec.PeekKind() != ']' {
		if err := n.normalize(elem); err != nil {
			return err
		}
	}
	return n.copyToken() // ]
}

func (n *keyNormalizer) copyToken() error {
	tok, err := n.dec.ReadToken()
	if err != nil {
		return err
	}
	return n.enc.WriteToken(tok)
}

func (n *keyNormalizer) copyValue() error {
	v, err := n.dec.ReadValue()
	if err != nil {
		return err
	}
	return n.enc.WriteValue(v)
}

var customUnmarshalerTypes = []reflect.Type{
	reflect.TypeFor[jsonv2.UnmarshalerFrom](),
	reflect.TypeFor[json.Unmarshaler](),
	reflect.TypeFor[encoding.TextUnmarshaler](),
}

// hasCustomUnmarshaler reports whether t decodes JSON by itself,
// so that the keys in the value must be kept as they are.
func hasCustomUnmarshaler(t reflect.Type) bool {
	pt := reflect.PointerTo(t)
	for _, u := range customUnmarshalerTypes {
		if t.Implements(u) || pt.Implements(u) {
			return true
		}
	}
	return false
}

type jsonField struct {
	name string // name that encoding/json/v2 matches to the field
	typ  reflect.Type
}

// jsonFieldsOf returns the fields of the struct type t by the keys in a
// definition file.
func jsonFieldsOf(t reflect.Type) map[string]jsonField {
	fields := map[string]jsonField{}
	for _, f := range reflect.VisibleFields(t) {
		tag, tagged := f.Tag.Lookup("json")
		name, _, _ := strings.Cut(tag, ",")
		switch {
		case tagged && tag == "-":
			continue
		case f.Anonymous && name == "":
			continue // the fields of an embedded struct are promoted
		case !f.IsExported():
			continue
		}
		key := name
		if name == "" {
			name = f.Name
			key = strings.ToLower(name[:1]) + name[1:]
		}
		fields[key] = jsonField{name: name, typ: f.Type}
	}
	return fields
}
