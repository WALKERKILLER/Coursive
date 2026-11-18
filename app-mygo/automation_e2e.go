//go:build e2e

package main

// Built with -tags e2e, the client listens on the socket MYGO_AUTOMATION
// names, for driving and capturing its screens from outside (see
// scripts/mygo-drive.py). Release builds leave it out.
//
// With COURSIVE_READONLY set, it refuses every request but GET and HEAD,
// so a run that plays a lecture or opens an editor leaves the library,
// playback positions and statistics as they were.
import (
	"errors"
	"net/http"
	"os"

	_ "github.com/egoist/mygo/plugins/automation"
)

type readOnly struct{ next http.RoundTripper }

func (r readOnly) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return nil, errors.New("read-only run: " + req.Method + " " + req.URL.Path)
	}
	return r.next.RoundTrip(req)
}

func init() {
	if os.Getenv("COURSIVE_READONLY") != "" {
		http.DefaultTransport = readOnly{http.DefaultTransport}
	}
}
