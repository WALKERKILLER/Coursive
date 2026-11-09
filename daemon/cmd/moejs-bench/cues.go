package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var srtTime = regexp.MustCompile(`(\d+):(\d+):(\d+)[,.](\d+)\s*-->\s*(\d+):(\d+):(\d+)[,.](\d+)`)

// readCues reads a {cues} JSON file or an SRT file into pilot cues.
func readCues(file string) []map[string]any {
	b, err := os.ReadFile(file)
	must(err)
	if strings.HasSuffix(strings.ToLower(file), ".json") {
		var doc struct {
			Cues []map[string]any `json:"cues"`
		}
		must(json.Unmarshal(b, &doc))
		return doc.Cues
	}
	var cues []map[string]any
	at := func(m []string, i int) float64 {
		h, _ := strconv.Atoi(m[i])
		mi, _ := strconv.Atoi(m[i+1])
		s, _ := strconv.Atoi(m[i+2])
		f, _ := strconv.Atoi(m[i+3])
		return float64(((h*60+mi)*60+s)*1000 + f)
	}
	for _, block := range regexp.MustCompile(`\r?\n\s*\r?\n`).Split(strings.TrimSpace(string(b)), -1) {
		lines := strings.Split(strings.ReplaceAll(block, "\r", ""), "\n")
		for i, line := range lines {
			if m := srtTime.FindStringSubmatch(line); m != nil {
				cues = append(cues, map[string]any{"start": at(m, 1), "end": at(m, 5), "text": strings.Join(lines[i+1:], "\n")})
				break
			}
		}
	}
	if len(cues) == 0 {
		must(errors.New("no cues in " + file))
	}
	return cues
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "moejs-bench:", err)
		os.Exit(1)
	}
}
