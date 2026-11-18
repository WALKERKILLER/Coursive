package study

import (
	"encoding/json"
	"math"
	"strconv"
)

// M is a JSON object as decoded into any.
type M = map[string]any

// str is Dart's '${v}': null reads "null".
func str(v any) string {
	switch v := v.(type) {
	case nil:
		return "null"
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// strOr is '${v ?? ”}'.
func strOr(v any) string {
	if v == nil {
		return ""
	}
	return str(v)
}

// num is a finite number or fallback (_n).
func num(v any, fallback float64) float64 {
	if f, ok := v.(float64); ok && finite(f) {
		return f
	}
	if i, ok := v.(int); ok {
		return float64(i)
	}
	return fallback
}

// toInt is (v as num? ?? 0).toInt().
func toInt(v any) int {
	switch v := v.(type) {
	case float64:
		if finite(v) {
			return int(v)
		}
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}

// isInt is Dart's `is int` for decoded JSON: an integral number.
func isInt(v any) (int, bool) {
	switch v := v.(type) {
	case float64:
		if finite(v) && v == math.Trunc(v) {
			return int(v), true
		}
	case int:
		return v, true
	case int64:
		return int(v), true
	}
	return 0, false
}

func obj(v any) M {
	m, _ := v.(M)
	return m
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}

func strs(v any) []string {
	var out []string
	for _, s := range list(v) {
		out = append(out, str(s))
	}
	return out
}

// Clone deep-copies decoded JSON through its encoding.
func Clone(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

// Encode is jsonEncode.
func Encode(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
