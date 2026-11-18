//go:build ignore

// generate writes strings_gen.go from the Flutter client's ARB file.
package main

import (
	"log"
	"os"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n/gen"
)

func main() {
	arb, err := os.ReadFile("../../../app/lib/l10n/app_zh.arb")
	if err != nil {
		log.Fatal(err)
	}
	src, err := gen.Generate(arb)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("strings_gen.go", src, 0o644); err != nil {
		log.Fatal(err)
	}
}
