package pluginfabric

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version is a service version, compared field by field.
type Version struct {
	Major int
	Minor int
	Patch int
}

var versionRE = regexp.MustCompile(`^(\d{1,6})(?:\.(\d{1,6}))?(?:\.(\d{1,6}))?$`)

// ParseVersion reads "1", "1.2" or "1.2.3".
func ParseVersion(s string) (Version, error) {
	m := versionRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return Version{}, fmt.Errorf("版本 %q 应为 1、1.2 或 1.2.3", s)
	}
	part := func(i int) int {
		n, _ := strconv.Atoi(m[i])
		return n
	}
	return Version{Major: part(1), Minor: part(2), Patch: part(3)}, nil
}

// String renders the shortest form: "1", "1.2" or "1.2.3".
func (v Version) String() string {
	switch {
	case v.Patch != 0:
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	case v.Minor != 0:
		return fmt.Sprintf("%d.%d", v.Major, v.Minor)
	}
	return strconv.Itoa(v.Major)
}

// Compare returns -1, 0 or 1.
func (v Version) Compare(o Version) int {
	for _, d := range [][2]int{{v.Major, o.Major}, {v.Minor, o.Minor}, {v.Patch, o.Patch}} {
		if d[0] != d[1] {
			if d[0] < d[1] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Range is the versions a requirement accepts: "1.2.3" exactly, "^1" for the
// same major and at least that version, or ">=1" for that version and later.
type Range struct {
	Op      string
	Version Version
}

var rangeRE = regexp.MustCompile(`^(\^|>=|=)?\s*(\d{1,6}(?:\.\d{1,6}){0,2})$`)

// ParseRange reads a requirement's version constraint.
func ParseRange(s string) (Range, error) {
	m := rangeRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return Range{}, fmt.Errorf("版本要求 %q 应为 1.2.3、^1 或 >=1", s)
	}
	v, err := ParseVersion(m[2])
	if err != nil {
		return Range{}, err
	}
	op := m[1]
	if op == "" {
		op = "="
	}
	return Range{Op: op, Version: v}, nil
}

// Matches reports whether a provider's version satisfies the range.
func (r Range) Matches(v Version) bool {
	switch r.Op {
	case "^":
		return v.Major == r.Version.Major && v.Compare(r.Version) >= 0
	case ">=":
		return v.Compare(r.Version) >= 0
	}
	return v.Compare(r.Version) == 0
}

// String renders the range the way a manifest writes it.
func (r Range) String() string {
	if r.Op == "=" {
		return r.Version.String()
	}
	return r.Op + r.Version.String()
}
