package study

import "unicode/utf16"

// utf16Len is a Dart String's length.
func utf16Len(r []rune) int {
	n := 0
	for _, c := range r {
		n += utf16.RuneLen(c)
	}
	return n
}

// truncUTF16 keeps the runes within n UTF-16 units.
func truncUTF16(r []rune, n int) []rune {
	used := 0
	for i, c := range r {
		used += utf16.RuneLen(c)
		if used > n {
			return r[:i]
		}
	}
	return r
}

// Len16 is the Dart length of s.
func Len16(s string) int { return utf16Len([]rune(s)) }

// Sub16 is s.substring(start, end) in UTF-16 units, clamped.
func Sub16(s string, start, end int) string {
	u := utf16.Encode([]rune(s))
	start, end = max(0, min(start, len(u))), max(0, min(end, len(u)))
	if start >= end {
		return ""
	}
	return string(utf16.Decode(u[start:end]))
}
