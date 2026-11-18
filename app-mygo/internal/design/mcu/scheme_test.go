package mcu

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// flutterSchemes is what Flutter's ColorScheme.fromSeed computes, written by
// app/test/design_system_export_test.dart.
type flutterSchemes struct {
	Roles   []string `json:"roles"`
	Schemes []struct {
		Seed     string   `json:"seed"`
		Variant  string   `json:"variant"`
		Contrast float64  `json:"contrast"`
		Dark     bool     `json:"dark"`
		Argb     []string `json:"argb"`
	} `json:"schemes"`
}

var variantNames = map[string]Variant{
	"tonalSpot": TonalSpot, "neutral": Neutral, "vibrant": Vibrant, "expressive": Expressive,
	"fidelity": Fidelity, "content": Content, "monochrome": Monochrome,
}

func hex(argb uint32) string { return fmt.Sprintf("#%08X", argb) }

func TestSchemesMatchFlutter(t *testing.T) {
	data, err := os.ReadFile("testdata/flutter_schemes.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc flutterSchemes
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Schemes) == 0 {
		t.Fatal("no schemes")
	}
	field := func(s *Scheme, role string) uint32 {
		name := string(role[0]-'a'+'A') + role[1:]
		v := reflect.ValueOf(s).Elem().FieldByName(name)
		if !v.IsValid() {
			t.Fatalf("no role %s", name)
		}
		return uint32(v.Uint())
	}
	mismatches := 0
	for _, want := range doc.Schemes {
		v, ok := variantNames[want.Variant]
		if !ok {
			t.Fatalf("unknown variant %s", want.Variant)
		}
		var seed uint32
		fmt.Sscanf(want.Seed, "#%08X", &seed)
		got := FromSeed(seed, v, want.Dark, want.Contrast)
		for i, role := range doc.Roles {
			if g := hex(field(&got, role)); g != want.Argb[i] {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("%s %s contrast %v dark %v: %s = %s, Flutter %s", want.Seed, want.Variant, want.Contrast, want.Dark, role, g, want.Argb[i])
				}
			}
		}
	}
	if mismatches > 0 {
		t.Errorf("%d roles differ from Flutter", mismatches)
	}
}
