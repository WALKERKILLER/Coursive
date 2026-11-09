package moejsexec

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/pluginfabric"
)

// plugin is a manifest whose moejs entry is index.js in testdata/<dir>.
func plugin(id string, permissions ...string) *pluginfabric.Manifest {
	return &pluginfabric.Manifest{Name: id, Version: "0.1.0", Coursive: pluginfabric.Coursive{
		ID:          id,
		Permissions: permissions,
		Runtime:     []pluginfabric.Runtime{{Kind: "moejs", Entry: pluginfabric.Entry{Path: "index.js"}}},
	}}
}

// adapter serves the plugins in testdata, by id, through the real engine.
func adapter(limits pluginfabric.Limits, granted map[string][]string, reads *atomic.Int64) *pluginfabric.MoeJSAdapter {
	f := &Factory{
		Root: func(m *pluginfabric.Manifest) (string, error) {
			return filepath.Abs(filepath.Join("testdata", m.Coursive.ID))
		},
		APIs: map[string]HostAPI{
			"library.read": {Permission: "library.read", Call: func(_ context.Context, args []any) (any, error) {
				reads.Add(1)
				return map[string]any{"path": args[0], "text": "讲义"}, nil
			}},
			"clock": {Call: func(context.Context, []any) (any, error) { return 42, nil }},
		},
		HostModules: map[string]string{"text": `export const shout = (s) => s.toUpperCase();`},
		Granted:     func(id string) []string { return granted[id] },
	}
	return &pluginfabric.MoeJSAdapter{Factory: f, Limits: limits, HostAPIs: []string{"library.read", "clock"}, Modules: []string{"text"}}
}

func prepare(t *testing.T, a *pluginfabric.MoeJSAdapter, m *pluginfabric.Manifest) {
	t.Helper()
	if err := a.Prepare(context.Background(), m); err != nil {
		t.Fatalf("Prepare %s: %v", m.Coursive.ID, err)
	}
	t.Cleanup(func() { _ = a.Dispose(context.Background(), m.Coursive.ID) })
}

func invoke(a *pluginfabric.MoeJSAdapter, id, action string, input any, deadline time.Duration) (any, error) {
	res, err := a.Invoke(context.Background(), pluginfabric.Invocation{PluginID: id, Action: action, Input: input, Deadline: deadline})
	return res.Output, err
}

// normalized is v as encoding/json reads it back, so Go and Node results
// compare by value.
func normalized(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func readJSON(t *testing.T, name string, out any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "pilot", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func TestPilotMatchesNode(t *testing.T) {
	var cases struct {
		Cases []struct {
			Name, Action, InputFile string
			Input                   map[string]any
			Merge                   map[string]any
		}
	}
	readJSON(t, "cases.json", &cases)
	var golden map[string]any
	readJSON(t, "expected.json", &golden)
	a := adapter(pluginfabric.Limits{MaxMemoryBytes: 64 << 20}, nil, &atomic.Int64{})
	prepare(t, a, plugin("pilot"))
	for _, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			input := c.Input
			if c.InputFile != "" {
				readJSON(t, c.InputFile, &input)
				for k, v := range c.Merge {
					input[k] = v
				}
			}
			out, err := invoke(a, "pilot", c.Action, input, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := normalized(t, out), golden[c.Name]; !reflect.DeepEqual(got, want) {
				g, _ := json.Marshal(got)
				w, _ := json.Marshal(want)
				t.Fatalf("MoeJS and Node differ\n got %s\nwant %s", g, w)
			}
		})
	}
}

func TestInfiniteLoopTimesOutAndTheRuntimeRecovers(t *testing.T) {
	a := adapter(pluginfabric.Limits{MaxConcurrent: 1}, nil, &atomic.Int64{})
	prepare(t, a, plugin("isolation"))
	start := time.Now()
	if _, err := invoke(a, "isolation", "spin", nil, 100*time.Millisecond); !errors.Is(err, pluginfabric.ErrInvocationTimeout) {
		t.Fatalf("spin: %v", err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("the interrupt took %v", took)
	}
	out, err := invoke(a, "isolation", "settled", map[string]any{"n": 21}, 0)
	if err != nil || normalized(t, out).(map[string]any)["value"] != 42.0 {
		t.Fatalf("after a timeout: %v %v", out, err)
	}
}

func TestCancellationStopsARunningScript(t *testing.T) {
	a := adapter(pluginfabric.Limits{}, nil, &atomic.Int64{})
	prepare(t, a, plugin("isolation"))
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	_, err := a.Invoke(ctx, pluginfabric.Invocation{PluginID: "isolation", Action: "spin"})
	if !errors.Is(err, pluginfabric.ErrInvocationCancelled) {
		t.Fatalf("cancel: %v", err)
	}
	if m := a.Metrics(); m.Cancelled != 1 {
		t.Fatalf("metrics: %+v", m)
	}
}

func TestMemoryLimitStopsTheScriptAndReports(t *testing.T) {
	a := adapter(pluginfabric.Limits{MaxMemoryBytes: 8 << 20, MaxConcurrent: 1}, nil, &atomic.Int64{})
	prepare(t, a, plugin("isolation"))
	for _, action := range []string{"grow", "growCaught"} {
		if _, err := invoke(a, "isolation", action, nil, 10*time.Second); !errors.Is(err, ErrMemoryLimit) {
			t.Fatalf("%s: %v", action, err)
		}
	}
	if _, err := invoke(a, "isolation", "settled", map[string]any{"n": 1}, 0); err != nil {
		t.Fatalf("a fresh runtime after the limit: %v", err)
	}
}

func TestAScriptReachesNoFileNetworkOrProcess(t *testing.T) {
	var reads atomic.Int64
	a := adapter(pluginfabric.Limits{}, nil, &reads)
	prepare(t, a, plugin("isolation"))
	out, err := invoke(a, "isolation", "probe", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := normalized(t, out).(map[string]any)
	for name, kind := range got["kinds"].(map[string]any) {
		if kind != "undefined" {
			t.Errorf("%s is %v inside the runtime", name, kind)
		}
	}
	// library.read needs a declared and granted permission; clock needs none.
	if !reflect.DeepEqual(got["coursive"], []any{"clock"}) || got["library"] != "undefined" {
		t.Errorf("host API: %v", got)
	}
	for _, action := range []string{"importFs", "evalCode", "newFunction", "readLibrary", "fails"} {
		if _, err := invoke(a, "isolation", action, map[string]any{"path": "/etc/passwd"}, 0); !errors.Is(err, ErrScript) {
			t.Errorf("%s: %v", action, err)
		}
	}
	if reads.Load() != 0 {
		t.Fatalf("the host function ran %d times without the permission", reads.Load())
	}
}

func TestHostAPINeedsADeclaredAndGrantedPermission(t *testing.T) {
	var reads atomic.Int64
	for name, c := range map[string]struct {
		declared, granted bool
	}{"declared only": {true, false}, "granted only": {false, true}, "both": {true, true}} {
		t.Run(name, func(t *testing.T) {
			reads.Store(0)
			var permissions []string
			if c.declared {
				permissions = []string{"library.read"}
			}
			granted := map[string][]string{}
			if c.granted {
				granted["isolation"] = []string{"library.read"}
			}
			a := adapter(pluginfabric.Limits{}, granted, &reads)
			prepare(t, a, plugin("isolation", permissions...))
			out, err := invoke(a, "isolation", "readLibrary", map[string]any{"path": "第 1 讲.pdf"}, 0)
			if c.declared && c.granted {
				if err != nil || normalized(t, out).(map[string]any)["text"] != "讲义" || reads.Load() != 1 {
					t.Fatalf("granted read: %v %v (%d)", out, err, reads.Load())
				}
				return
			}
			if !errors.Is(err, ErrScript) || reads.Load() != 0 {
				t.Fatalf("ungranted read: %v %v (%d)", out, err, reads.Load())
			}
		})
	}
}

func TestImportsStayInThePackageOrTheAllowlist(t *testing.T) {
	a := adapter(pluginfabric.Limits{}, nil, &atomic.Int64{})
	for _, id := range []string{"escape", "bare"} {
		if err := a.Prepare(context.Background(), plugin(id)); !errors.Is(err, pluginfabric.ErrModuleDenied) {
			t.Errorf("%s: %v", id, err)
		}
	}
	// The isolation package imports the allowlisted host module "text".
	denied := adapter(pluginfabric.Limits{}, nil, &atomic.Int64{})
	denied.Modules = nil
	if err := denied.Prepare(context.Background(), plugin("isolation")); !errors.Is(err, pluginfabric.ErrModuleDenied) {
		t.Errorf("host module outside the allowlist: %v", err)
	}
}

func TestOutputBoundAndUnknownActions(t *testing.T) {
	a := adapter(pluginfabric.Limits{MaxOutputBytes: 1 << 20}, nil, &atomic.Int64{})
	prepare(t, a, plugin("isolation"))
	if _, err := invoke(a, "isolation", "big", nil, 0); !errors.Is(err, pluginfabric.ErrOutputTooLarge) {
		t.Fatalf("big: %v", err)
	}
	for _, action := range []string{"missing", "constructor.prototype", ""} {
		if _, err := invoke(a, "isolation", action, nil, 0); !errors.Is(err, ErrScript) {
			t.Errorf("%q: %v", action, err)
		}
	}
	if _, err := invoke(a, "isolation", "now", nil, 0); err != nil {
		t.Fatalf("a host function without a permission: %v", err)
	}
}

func TestConcurrentCallsShareTheCompiledModule(t *testing.T) {
	a := adapter(pluginfabric.Limits{MaxConcurrent: 4}, nil, &atomic.Int64{})
	prepare(t, a, plugin("pilot"))
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			out, err := invoke(a, "pilot", "onEvent", map[string]any{"type": "lecture.progress", "payload": map[string]any{"percent": 95}}, 0)
			if err == nil && normalized(t, out).(map[string]any)["action"] != "tag" {
				err = errors.New("wrong result")
			}
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if m := a.Metrics(); m.Invocations != 32 || m.Errors != 0 {
		t.Fatalf("metrics: %+v", m)
	}
}
