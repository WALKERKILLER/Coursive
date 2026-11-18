// Package core is the Flutter client's lib/core in Go: the daemon process,
// its HTTP API and event stream, the models over its JSON, the app state
// the views read, and the small services beside them (preferences, the
// profile, formatting, opening files).
package core

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// J is a decoded JSON object, as Dart's Map<String, dynamic>. Its getters
// coerce leniently, as models.dart's _i, _d, _s and _b do.
type J map[string]any

// Int is _i: numbers truncate, strings parse, anything else is 0.
func Int(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
		f, _ := n.Float64()
		return int(f)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

// Float is _d.
func Float(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}

// Str is _s: nil is "", anything else prints as Dart's toString would.
func Str(v any) string {
	switch s := v.(type) {
	case nil:
		return ""
	case string:
		return s
	case float64:
		if s == math.Trunc(s) && math.Abs(s) < 1e15 {
			return strconv.FormatInt(int64(s), 10)
		}
		return strconv.FormatFloat(s, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(s)
	}
	return fmt.Sprint(v)
}

// Bool is _b: true, 1 and "1".
func Bool(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case float64:
		return b == 1
	case int:
		return b == 1
	case string:
		return b == "1"
	}
	return false
}

// Obj is v as an object, or nil.
func Obj(v any) J {
	switch m := v.(type) {
	case J:
		return m
	case map[string]any:
		return J(m)
	}
	return nil
}

// List is jsonList: Go encodes empty slices as null, which is an empty list.
func List(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

// Objs is the objects of a list.
func Objs(v any) []J {
	var out []J
	for _, e := range List(v) {
		if m := Obj(e); m != nil {
			out = append(out, m)
		}
	}
	return out
}

// Strs is a list's elements as strings.
func Strs(v any) []string {
	var out []string
	for _, e := range List(v) {
		out = append(out, Str(e))
	}
	return out
}

func (j J) I(k string) int       { return Int(j[k]) }
func (j J) F(k string) float64   { return Float(j[k]) }
func (j J) S(k string) string    { return Str(j[k]) }
func (j J) B(k string) bool      { return Bool(j[k]) }
func (j J) O(k string) J         { return Obj(j[k]) }
func (j J) L(k string) []any     { return List(j[k]) }
func (j J) Os(k string) []J      { return Objs(j[k]) }
func (j J) Ss(k string) []string { return Strs(j[k]) }
func (j J) Has(k string) bool    { _, ok := j[k]; return ok }
func (j J) IsTrue(k string) bool { return j[k] == true }
func (j J) Or(k, def string) string {
	if v, ok := j[k]; ok && v != nil {
		return Str(v)
	}
	return def
}

// Copy is a shallow copy of j.
func (j J) Copy() J {
	out := make(J, len(j))
	for k, v := range j {
		out[k] = v
	}
	return out
}

// Decode parses JSON text into plain Go values (objects as map[string]any).
func Decode(b []byte) (any, error) {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return normalize(v), nil
}

// normalize turns map[string]any into J all the way down.
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, e := range t {
			t[k] = normalize(e)
		}
		return J(t)
	case []any:
		for i, e := range t {
			t[i] = normalize(e)
		}
		return t
	}
	return v
}

// Encode is jsonEncode.
func Encode(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}
