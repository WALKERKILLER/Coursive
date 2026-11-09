//go:build !unix

package main

import (
	"fmt"
	"os"
)

// main reports that the benchmark reads CPU and memory through Unix APIs.
func main() {
	fmt.Fprintln(os.Stderr, "moejs-bench runs on Linux and macOS")
	os.Exit(2)
}
