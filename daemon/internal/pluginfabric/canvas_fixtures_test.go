package pluginfabric

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// canvasFixtures are the shared canvas-object cases. The app validates the same
// file (app/test/study_plugin_objects_test.dart), so an object the canvas
// commits is an object the daemon saves.
type canvasFixtures struct {
	Schema string         `json:"schema"`
	Base   map[string]any `json:"base"`
	Cases  []struct {
		Name string         `json:"name"`
		Set  map[string]any `json:"set"`
		Code string         `json:"code"`
		Path string         `json:"path"`
	} `json:"cases"`
}

// fixtureObject applies a case's dotted-path changes to a copy of base; a null
// value removes the field.
func fixtureObject(t *testing.T, base, set map[string]any) CanvasObject {
	t.Helper()
	b, _ := json.Marshal(base)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for key, value := range set {
		parts := strings.Split(key, ".")
		at := m
		for _, p := range parts[:len(parts)-1] {
			at = at[p].(map[string]any)
		}
		if value == nil {
			delete(at, parts[len(parts)-1])
		} else {
			at[parts[len(parts)-1]] = value
		}
	}
	b, _ = json.Marshal(m)
	var o CanvasObject
	if err := json.Unmarshal(b, &o); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return o
}

func TestCanvasObjectSharedFixtures(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "canvas", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f canvasFixtures
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if want := canvasSubject(); !strings.HasSuffix(want, f.Schema) {
		t.Fatalf("fixtures are for %q, this build reads %q", f.Schema, want)
	}
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			err := fixtureObject(t, f.Base, c.Set).Validate()
			if c.Code == "" {
				if err != nil {
					t.Fatalf("Validate: %v", err)
				}
				return
			}
			var problems *Error
			if !errors.As(err, &problems) {
				t.Fatalf("Validate = %v, want %s at %s", err, c.Code, c.Path)
			}
			if got := problems.Problems[0]; got.Code != c.Code || got.Path != c.Path {
				t.Fatalf("first problem %s at %s, want %s at %s", got.Code, got.Path, c.Code, c.Path)
			}
		})
	}
}
