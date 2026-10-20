// Package jsobj holds the values JSON.parse and the YAML reader produce, with JavaScript's
// object semantics: an Object enumerates integer-like keys in ascending order, then the
// other keys in the order they were added.
//
// A value is nil (null), bool, float64, string, []any or *Object.
package jsobj

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
)

// Object is a plain JS object.
type Object struct {
	order  []string
	values map[string]any
}

// New returns an empty Object.
func New() *Object {
	return &Object{values: map[string]any{}}
}

// Get returns the value for key and whether the object has it.
func (o *Object) Get(key string) (any, bool) {
	if o == nil {
		return nil, false
	}
	v, ok := o.values[key]
	return v, ok
}

// Set adds or replaces key. A replaced key keeps its place.
func (o *Object) Set(key string, value any) {
	if _, ok := o.values[key]; !ok {
		o.order = append(o.order, key)
	}
	o.values[key] = value
}

// Len returns the number of keys.
func (o *Object) Len() int {
	if o == nil {
		return 0
	}
	return len(o.order)
}

// Keys returns the keys in Object.keys order.
func (o *Object) Keys() []string {
	if o == nil {
		return nil
	}
	var indexes, names []string
	for _, k := range o.order {
		if isArrayIndex(k) {
			indexes = append(indexes, k)
		} else {
			names = append(names, k)
		}
	}
	slices.SortStableFunc(indexes, func(a, b string) int {
		if len(a) != len(b) {
			return len(a) - len(b)
		}
		return strings.Compare(a, b)
	})
	return append(indexes, names...)
}

// isArrayIndex reports whether key is the canonical form of an integer below 2^32 - 1,
// which JS enumerates before other keys.
func isArrayIndex(key string) bool {
	if key == "" || len(key) > 10 || (len(key) > 1 && key[0] == '0') {
		return false
	}
	n, err := strconv.ParseUint(key, 10, 64)
	return err == nil && n < 1<<32-1
}

// Clone returns a shallow copy, as {...o} makes.
func (o *Object) Clone() *Object {
	c := New()
	for _, k := range o.Keys() {
		c.Set(k, o.values[k])
	}
	return c
}

// DeepClone returns a copy that shares no objects or arrays with o.
func (o *Object) DeepClone() *Object {
	c := New()
	for _, k := range o.Keys() {
		c.Set(k, deepClone(o.values[k]))
	}
	return c
}

func deepClone(v any) any {
	switch v := v.(type) {
	case *Object:
		return v.DeepClone()
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = deepClone(e)
		}
		return out
	}
	return v
}

// Truthy reports whether v is truthy to JavaScript.
func Truthy(v any) bool {
	switch v := v.(type) {
	case nil:
		return false
	case bool:
		return v
	case float64:
		return v == v && v != 0
	case string:
		return v != ""
	default:
		return true
	}
}

// ToString returns String(v).
func ToString(v any) string {
	switch v := v.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return jsnum.Format(v)
	case string:
		return v
	case []any:
		parts := make([]string, len(v))
		for i, e := range v {
			if e != nil {
				parts[i] = ToString(e)
			}
		}
		return strings.Join(parts, ",")
	default:
		return "[object Object]"
	}
}
