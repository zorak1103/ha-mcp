// Package jsonorder records the key order of JSON documents in a shadow tree
// and re-emits generic Go values (map[string]any / []any) in that order.
//
// Go's encoding/json writes map keys sorted, which destroys the document order
// of objects whose order is semantically meaningful (for example Home
// Assistant renders an automation's "variables" sequentially). Keeping every
// config as a map and carrying the order separately keeps walkers and
// matchers untouched while still letting the writer reproduce the original
// order. A missing or partial Order degrades to sorted keys, which is exactly
// what json.Marshal produces.
package jsonorder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
)

// Order is a shadow tree describing key order for one JSON value.
//
// An object node carries Keys (document order) and Fields (sub-orders of the
// members that are objects or arrays). An array node carries Items, one entry
// per element (nil for scalars). Scalars have no node at all (nil *Order).
// Only exported fields exist so that go-cmp can compare trees (it panics on
// unexported ones); keep it that way.
type Order struct {
	Keys   []string          // object: keys in document order
	Fields map[string]*Order // object members that are objects/arrays
	Items  []*Order          // array elements (nil for scalars)
}

// Capture walks data with json.Decoder.Token and records the key order of
// every object. A scalar document yields a nil node. For duplicate keys the
// first position and the last subtree win, matching what decoding into a map
// keeps (last value) while pinning the key where it first appeared.
func Capture(data []byte) (*Order, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	o, err := captureValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("capturing key order: unexpected data after top-level value")
		}
		return nil, fmt.Errorf("capturing key order: %w", err)
	}
	return o, nil
}

func captureValue(dec *json.Decoder) (*Order, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("capturing key order: %w", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil, nil //nolint:nilnil // a nil node is the documented representation of a scalar
	}
	if delim == '{' {
		return captureObject(dec)
	}
	return captureArray(dec)
}

func captureObject(dec *json.Decoder) (*Order, error) {
	o := &Order{}
	seen := make(map[string]struct{})
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("capturing key order: %w", err)
		}
		key, _ := keyTok.(string) // Decoder.Token only yields strings in key position
		sub, err := captureValue(dec)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[key]; !dup {
			seen[key] = struct{}{}
			o.Keys = append(o.Keys, key)
		}
		if sub == nil {
			delete(o.Fields, key)
			continue
		}
		if o.Fields == nil {
			o.Fields = make(map[string]*Order)
		}
		o.Fields[key] = sub
	}
	if _, err := dec.Token(); err != nil { // closing '}'
		return nil, fmt.Errorf("capturing key order: %w", err)
	}
	if len(o.Fields) == 0 {
		o.Fields = nil
	}
	return o, nil
}

func captureArray(dec *json.Decoder) (*Order, error) {
	o := &Order{}
	for dec.More() {
		sub, err := captureValue(dec)
		if err != nil {
			return nil, err
		}
		o.Items = append(o.Items, sub)
	}
	if _, err := dec.Token(); err != nil { // closing ']'
		return nil, fmt.Errorf("capturing key order: %w", err)
	}
	return o, nil
}

// Field returns the order of the object member key, or nil. Safe on a nil
// receiver.
func (o *Order) Field(key string) *Order {
	if o == nil {
		return nil
	}
	return o.Fields[key]
}

// Item returns the order of array element i, or nil when i is out of range.
// Safe on a nil receiver.
func (o *Order) Item(i int) *Order {
	if o == nil || i < 0 || i >= len(o.Items) {
		return nil
	}
	return o.Items[i]
}

// Clone returns a deep copy of the tree; a nil receiver yields nil.
func (o *Order) Clone() *Order {
	if o == nil {
		return nil
	}
	c := &Order{Keys: slices.Clone(o.Keys)}
	if o.Fields != nil {
		c.Fields = make(map[string]*Order, len(o.Fields))
		for k, v := range o.Fields {
			c.Fields[k] = v.Clone()
		}
	}
	if o.Items != nil {
		c.Items = make([]*Order, len(o.Items))
		for i, v := range o.Items {
			c.Items[i] = v.Clone()
		}
	}
	return c
}

// WithField returns a copy of o whose member key carries sub (a nil sub
// removes the member's order). It is copy-on-write: o is never modified and
// untouched subtrees stay shared with it, so callers must Clone before
// mutating a shared subtree in place. A nil receiver yields a new root node.
// WithField does not add key to Keys: a member attached to a key that is not
// already in Keys is emitted in the sorted tail, so append the key to Keys
// when its position matters.
func (o *Order) WithField(key string, sub *Order) *Order {
	c := &Order{}
	if o != nil {
		c.Keys = slices.Clone(o.Keys)
		c.Items = slices.Clone(o.Items)
		if o.Fields != nil {
			c.Fields = make(map[string]*Order, len(o.Fields)+1)
			for k, v := range o.Fields {
				c.Fields[k] = v
			}
		}
	}
	if sub == nil {
		delete(c.Fields, key)
	} else {
		if c.Fields == nil {
			c.Fields = make(map[string]*Order, 1)
		}
		c.Fields[key] = sub
	}
	if len(c.Fields) == 0 {
		c.Fields = nil
	}
	return c
}

// Marshal encodes v like json.Marshal, except that map[string]any keys listed
// in o come first in o.Keys order; remaining keys follow sorted. A nil o, or
// an o whose shape does not match v, falls back to json.Marshal's output.
// Everything that is not a map[string]any or []any is encoded by
// json.Marshal, so escaping and number formatting are unchanged.
func Marshal(v any, o *Order) ([]byte, error) {
	var buf bytes.Buffer
	if err := encode(&buf, v, o); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encode(buf *bytes.Buffer, v any, o *Order) error {
	switch t := v.(type) {
	case map[string]any:
		if t == nil {
			buf.WriteString("null")
			return nil
		}
		return encodeMap(buf, t, o)
	case []any:
		if t == nil {
			buf.WriteString("null")
			return nil
		}
		return encodeSlice(buf, t, o)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshaling value: %w", err)
		}
		buf.Write(b)
		return nil
	}
}

func encodeMap(buf *bytes.Buffer, m map[string]any, o *Order) error {
	keys := orderedKeys(m, o)
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k) // marshaling a string cannot fail
		buf.Write(kb)
		buf.WriteByte(':')
		if err := encode(buf, m[k], o.Field(k)); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

func encodeSlice(buf *bytes.Buffer, s []any, o *Order) error {
	buf.WriteByte('[')
	for i, e := range s {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encode(buf, e, o.Item(i)); err != nil {
			return err
		}
	}
	buf.WriteByte(']')
	return nil
}

// orderedKeys returns the keys of m: recorded keys present in m first (in
// o.Keys order), then the remaining keys sorted.
func orderedKeys(m map[string]any, o *Order) []string {
	keys := make([]string, 0, len(m))
	placed := make(map[string]struct{}, len(m))
	if o != nil {
		for _, k := range o.Keys {
			if _, ok := m[k]; !ok {
				continue
			}
			if _, dup := placed[k]; dup {
				continue
			}
			placed[k] = struct{}{}
			keys = append(keys, k)
		}
	}
	rest := make([]string, 0, len(m)-len(keys))
	for k := range m {
		if _, ok := placed[k]; !ok {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(keys, rest...)
}
