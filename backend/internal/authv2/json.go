package authv2

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

// Small closed protocol parser. Duplicate keys (including escaped aliases),
// depth, bytes and unknown fields are rejected before interpreting authority.
func decode(raw []byte, max int, out any, required ...string) error {
	if len(raw) == 0 || len(raw) > max || !utf8.Valid(raw) {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, err := value(d, 0)
	if err != nil {
		return err
	}
	if _, err = d.Token(); err != io.EOF {
		return ErrInvalid
	}
	m, ok := v.(map[string]any)
	if !ok {
		return ErrInvalid
	}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			return ErrInvalid
		}
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return ErrInvalid
	}
	return nil
}
func value(d *json.Decoder, depth int) (any, error) {
	if depth > 16 {
		return nil, ErrInvalid
	}
	t, err := d.Token()
	if err != nil {
		return nil, ErrInvalid
	}
	switch t {
	case json.Delim('{'):
		m := map[string]any{}
		for d.More() {
			k, err := d.Token()
			name, ok := k.(string)
			if err != nil || !ok {
				return nil, ErrInvalid
			}
			if _, exists := m[name]; exists {
				return nil, ErrInvalid
			}
			v, err := value(d, depth+1)
			if err != nil {
				return nil, err
			}
			m[name] = v
		}
		if t, err := d.Token(); err != nil || t != json.Delim('}') {
			return nil, ErrInvalid
		}
		return m, nil
	case json.Delim('['):
		var a []any
		for d.More() {
			v, err := value(d, depth+1)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		if t, err := d.Token(); err != nil || t != json.Delim(']') {
			return nil, ErrInvalid
		}
		return a, nil
	default:
		if _, ok := t.(json.Delim); ok {
			return nil, ErrInvalid
		}
		return t, nil
	}
}
