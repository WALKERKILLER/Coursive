package pluginfabric

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fixtures is the shared case list in testdata: the daemon, the assistant host
// and the app all validate these files, so the accepted shape cannot drift
// between them.
type fixtures struct {
	Schema string `json:"schema"`
	Cases  []struct {
		File string `json:"file"`
		Code string `json:"code"`
		Path string `json:"path"`
	} `json:"cases"`
}

func readFixtures(t *testing.T) fixtures {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "cases.json"))
	if err != nil {
		t.Fatalf("read cases.json: %v", err)
	}
	var f fixtures
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("decode cases.json: %v", err)
	}
	if f.Schema != SchemaVersion {
		t.Fatalf("cases.json is for %q, want %q", f.Schema, SchemaVersion)
	}
	return f
}

func TestFixtures(t *testing.T) {
	f := readFixtures(t)
	listed := map[string]bool{}
	for _, c := range f.Cases {
		listed[c.File] = true
		b, err := os.ReadFile(filepath.Join("testdata", c.File))
		if err != nil {
			t.Fatalf("%s: %v", c.File, err)
		}
		m, err := Parse(b)
		if c.Code == "" {
			if err != nil {
				t.Errorf("%s: want a valid manifest, got %v", c.File, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: want %s, got a valid manifest", c.File, c.Code)
			continue
		}
		if got := Code(err); got != c.Code {
			t.Errorf("%s: first code = %q, want %q (%v)", c.File, got, c.Code, err)
		}
		var pe *Error
		if !errors.As(err, &pe) || len(pe.Problems) == 0 {
			t.Errorf("%s: want a manifest error, got %T", c.File, err)
			continue
		}
		if got := pe.Problems[0].Path; got != c.Path {
			t.Errorf("%s: first path = %q, want %q", c.File, got, c.Path)
		}
		if m != nil {
			t.Errorf("%s: Parse returned a manifest together with an error", c.File)
		}
	}

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	for _, e := range entries {
		if name := e.Name(); !e.IsDir() && name != "cases.json" && !listed[name] {
			t.Errorf("testdata/%s is not listed in cases.json", name)
		}
	}
}

func TestRules(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{
			"version": "1.0.0",
			"coursive": map[string]any{
				"id": "demo", "title": "演示", "summary": "演示插件", "api": "^3.0.0",
			},
		}
	}
	plugin := func(m map[string]any) map[string]any { return m["coursive"].(map[string]any) }

	cases := []struct {
		name string
		edit func(map[string]any)
		code string
		path string
	}{
		{"version not semver", func(m map[string]any) { m["version"] = "v1" }, "version-invalid", "version"},
		{"id missing", func(m map[string]any) { delete(plugin(m), "id") }, "id-missing", "coursive.id"},
		{"title too long", func(m map[string]any) {
			plugin(m)["title"] = "一二三四五六七八九十一二三四五六七八九十一二三四五"
		}, "title-invalid", "coursive.title"},
		{"summary missing", func(m map[string]any) { delete(plugin(m), "summary") }, "summary-invalid", "coursive.summary"},
		{"api not a range", func(m map[string]any) { plugin(m)["api"] = "3.0.0-rc1" }, "api-invalid", "coursive.api"},
		{"unknown category", func(m map[string]any) { plugin(m)["categories"] = []string{"gaming"} }, "category-unknown", "coursive.categories[0]"},
		{"runtime without entry", func(m map[string]any) {
			plugin(m)["runtime"] = []any{map[string]any{"kind": "moejs"}}
		}, "runtime-entry", "coursive.runtime[0].entry"},
		{"skills on a non-pi runtime", func(m map[string]any) {
			plugin(m)["runtime"] = []any{map[string]any{"kind": "moejs", "entry": "logic/index.js", "skills": []string{"skills"}}}
		}, "runtime-skills", "coursive.runtime[0].skills"},
		{"absolute runtime entry", func(m map[string]any) {
			plugin(m)["runtime"] = []any{map[string]any{"kind": "pi", "entry": "/etc/passwd"}}
		}, "runtime-path", "coursive.runtime[0].entry"},
		{"provided version is a range", func(m map[string]any) {
			plugin(m)["provides"] = []any{map[string]any{"service": "review.generate", "version": "^1"}}
		}, "provides-invalid", "coursive.provides[0].version"},
		{"duplicate permission", func(m map[string]any) {
			plugin(m)["permissions"] = []string{"views", "views"}
		}, "permission-duplicate", "coursive.permissions[1]"},
		{"exec permission without commands", func(m map[string]any) {
			plugin(m)["permissions"] = []string{"exec"}
		}, "exec-undeclared", "coursive.exec"},
		{"network permission without hosts", func(m map[string]any) {
			plugin(m)["permissions"] = []string{"network"}
		}, "hosts-undeclared", "coursive.hosts"},
		{"bad contribution id", func(m map[string]any) {
			plugin(m)["contributes"] = map[string]any{"views": []any{map[string]any{"id": "Bad View"}}}
		}, "contribution-invalid", "coursive.contributes.views[0].id"},
		{"duplicate tool", func(m map[string]any) {
			plugin(m)["contributes"] = map[string]any{"tools": []string{"review.generate", "review.generate"}}
		}, "contribution-duplicate", "coursive.contributes.tools[1]"},
		{"v1 engine field", func(m map[string]any) { plugin(m)["engine"] = "^1.0.0" }, "field-unknown", "coursive.engine"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := base()
			c.edit(m)
			b, err := json.Marshal(m)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			_, err = Parse(b)
			if err == nil {
				t.Fatalf("want %s, got a valid manifest", c.code)
			}
			if got := Code(err); got != c.code {
				t.Fatalf("code = %q, want %q (%v)", got, c.code, err)
			}
			var pe *Error
			if !errors.As(err, &pe) {
				t.Fatalf("want a manifest error, got %T", err)
			}
			if got := pe.Problems[0].Path; got != c.path {
				t.Fatalf("path = %q, want %q", got, c.path)
			}
		})
	}
}

func TestParseDecodesTheManifest(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "valid-full.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	m, err := Parse(b)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	c := m.Coursive
	if c.ID != "virtual-lab" || c.API != "^3.0.0" || m.Version != "1.2.3" {
		t.Fatalf("identity decoded as %q %q %q", c.ID, c.API, m.Version)
	}
	if len(c.Runtime) != 3 || c.Runtime[0].Kind != "pi" || c.Runtime[0].Entry.Path != "agent/index.mjs" {
		t.Fatalf("runtime decoded as %+v", c.Runtime)
	}
	if got := c.Runtime[0].Skills; len(got) != 1 || got[0] != "skills" {
		t.Fatalf("skills decoded as %v", got)
	}
	if len(c.Requires) != 2 || c.Requires[1].Service != "llm.generate" || !c.Requires[1].Optional {
		t.Fatalf("requires decoded as %+v", c.Requires)
	}
	if len(c.Provides) != 1 || c.Provides[0].Version != "1.0.0" {
		t.Fatalf("provides decoded as %+v", c.Provides)
	}
	if len(c.Contributes.Actions) != 1 || c.Contributes.Actions[0].ID != "raytrace.open" {
		t.Fatalf("contributions decoded as %+v", c.Contributes)
	}
	if len(c.Contributes.Tools) != 1 || c.Contributes.Tools[0] != "acoustics.raytrace" {
		t.Fatalf("tools decoded as %v", c.Contributes.Tools)
	}
}

func TestSourceExecutableIsAPlatformMap(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "valid-source-provider.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	m, err := Parse(b)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(m.Coursive.Runtime) != 1 {
		t.Fatalf("runtime decoded as %+v", m.Coursive.Runtime)
	}
	if got := m.Coursive.Runtime[0].Entry.Platforms["linux-x64"]; got != "bin/provider-linux-x64" {
		t.Fatalf("process entry decoded as %+v", m.Coursive.Runtime[0].Entry)
	}
	if len(m.Coursive.Contributes.Sources) != 1 || m.Coursive.Contributes.Sources[0].Protocol != "1" {
		t.Fatalf("sources decoded as %+v", m.Coursive.Contributes.Sources)
	}
}

// TestSchemaAgreesWithValidator keeps docs/schemas/plugin-manifest.v3.schema.json
// and this package from drifting: the closed sets live in the schema and the
// validator reads the same lists.
func TestSchemaAgreesWithValidator(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "schemas", "plugin-manifest.v3.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	defs := at(t, schema, "$defs")
	pluginDef := at(t, defs, "plugin")

	for _, c := range []struct {
		what string
		got  any
		want []string
	}{
		{"permissions", at(t, pluginDef, "properties", "permissions", "items", "enum"), Permissions},
		{"runtime kinds", at(t, defs, "runtime", "properties", "kind", "enum"), RuntimeKinds},
		{"categories", at(t, pluginDef, "properties", "categories", "items", "enum"), Categories},
	} {
		got := stringsOf(t, c.got)
		if !sameSet(got, c.want) {
			t.Errorf("schema %s = %v, validator has %v", c.what, got, c.want)
		}
	}

	keys := mapKeys(t, at(t, defs, "contributes", "properties"))
	if !sameSet(keys, ContributionGroups) {
		t.Errorf("schema contribution groups = %v, validator has %v", keys, ContributionGroups)
	}

	required := stringsOf(t, at(t, pluginDef, "required"))
	if !sameSet(required, []string{"id", "title", "summary", "api"}) {
		t.Errorf("schema requires %v", required)
	}
}

func at(t *testing.T, v any, keys ...string) any {
	t.Helper()
	for _, key := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("%q: %T is not an object", key, v)
		}
		v, ok = m[key]
		if !ok {
			t.Fatalf("%q: missing", key)
		}
	}
	return v
}

func stringsOf(t *testing.T, v any) []string {
	t.Helper()
	list, ok := v.([]any)
	if !ok {
		t.Fatalf("%T is not an array", v)
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("%v is not a string", item)
		}
		out = append(out, s)
	}
	return out
}

func mapKeys(t *testing.T, v any) []string {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%T is not an object", v)
	}
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, v := range a {
		seen[v] = true
	}
	for _, v := range b {
		if !seen[v] {
			return false
		}
	}
	return true
}
