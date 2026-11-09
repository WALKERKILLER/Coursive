package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const pilotDir = "../../internal/moejsexec/testdata/pilot"

func readPilot(t *testing.T, name string, out any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(pilotDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatal(err)
	}
}

// The Go baseline gives the golden results Node and MoeJS give, so the
// benchmark compares equal work.
func TestNativeMatchesTheGoldenResults(t *testing.T) {
	var cases struct {
		Cases []struct {
			Name, Action, InputFile string
			Input, Merge            map[string]any
		}
	}
	readPilot(t, "cases.json", &cases)
	var golden map[string]any
	readPilot(t, "expected.json", &golden)
	for _, c := range cases.Cases {
		input := c.Input
		if c.InputFile != "" {
			readPilot(t, c.InputFile, &input)
			for k, v := range c.Merge {
				input[k] = v
			}
		}
		b, _ := json.Marshal(native[c.Action](input))
		var got any
		_ = json.Unmarshal(b, &got)
		if !reflect.DeepEqual(got, golden[c.Name]) {
			w, _ := json.Marshal(golden[c.Name])
			t.Errorf("%s:\n got %s\nwant %s", c.Name, b, w)
		}
	}
}

func TestReadCuesParsesSRT(t *testing.T) {
	file := filepath.Join(t.TempDir(), "a.srt")
	srt := "1\r\n00:00:01,000 --> 00:00:02,500\r\n嗯，今天讲哈希表\r\n\r\n2\n00:01:02.250 --> 00:01:04,000\nkey mod m\n第二行\n"
	if err := os.WriteFile(file, []byte(srt), 0o600); err != nil {
		t.Fatal(err)
	}
	got := readCues(file)
	want := []map[string]any{
		{"start": 1000.0, "end": 2500.0, "text": "嗯，今天讲哈希表"},
		{"start": 62250.0, "end": 64000.0, "text": "key mod m\n第二行"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
}
