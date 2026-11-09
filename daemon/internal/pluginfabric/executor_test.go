package pluginfabric

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeFactory struct {
	runtime ExecutorRuntime
	err     error
}

func (f *fakeFactory) New(context.Context, *Manifest, Limits, []string, []string) (ExecutorRuntime, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.runtime, nil
}

type fakeRuntime struct {
	invoke func(context.Context, Invocation) (any, error)
	closed atomic.Bool
}

func (r *fakeRuntime) Invoke(ctx context.Context, req Invocation) (any, error) {
	if r.invoke != nil {
		return r.invoke(ctx, req)
	}
	return map[string]any{"ok": true}, nil
}

func (r *fakeRuntime) Close() error {
	r.closed.Store(true)
	return nil
}

func manifest(id string) *Manifest {
	return &Manifest{Name: id, Version: "1.0.0", Coursive: Coursive{ID: id, Title: id}}
}

func adapter(runtime ExecutorRuntime, limits Limits) *MoeJSAdapter {
	return &MoeJSAdapter{Factory: &fakeFactory{runtime: runtime}, Limits: limits, HostAPIs: []string{"library.read"}, Modules: []string{"json"}}
}

func TestMoeJSAdapterInvokeAndMetrics(t *testing.T) {
	rt := &fakeRuntime{invoke: func(context.Context, Invocation) (any, error) { return map[string]any{"cards": 3}, nil }}
	a := adapter(rt, Limits{MaxOutputBytes: 1024, MaxDeadline: time.Second})
	if err := a.Prepare(context.Background(), manifest("demo")); err != nil {
		t.Fatal(err)
	}
	res, err := a.Invoke(context.Background(), Invocation{PluginID: "demo", Action: "review.normalize", HostAPIs: []string{"library.read"}, Modules: []string{"json"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output == nil || res.Duration <= 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	m := a.Metrics()
	if m.Invocations != 1 || m.Errors != 0 || m.P50 <= 0 || m.P95 <= 0 {
		t.Fatalf("unexpected metrics: %+v", m)
	}
}

func TestMoeJSAdapterRejectsUnpreparedAndDeniedInputs(t *testing.T) {
	a := adapter(&fakeRuntime{}, Limits{})
	if _, err := a.Invoke(context.Background(), Invocation{PluginID: "missing"}); !errors.Is(err, ErrPluginNotPrepared) {
		t.Fatalf("unprepared: %v", err)
	}
	if err := a.Prepare(context.Background(), manifest("demo")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Invoke(context.Background(), Invocation{PluginID: "demo", HostAPIs: []string{"vault.read"}}); !errors.Is(err, ErrHostAPIDenied) {
		t.Fatalf("host API: %v", err)
	}
	if _, err := a.Invoke(context.Background(), Invocation{PluginID: "demo", Modules: []string{"fs"}}); !errors.Is(err, ErrModuleDenied) {
		t.Fatalf("module: %v", err)
	}
}

func TestMoeJSAdapterTimeoutAndCancel(t *testing.T) {
	rt := &fakeRuntime{invoke: func(ctx context.Context, _ Invocation) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	a := adapter(rt, Limits{MaxDeadline: 20 * time.Millisecond})
	if err := a.Prepare(context.Background(), manifest("demo")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Invoke(context.Background(), Invocation{PluginID: "demo", Deadline: 10 * time.Millisecond}); !errors.Is(err, ErrInvocationTimeout) {
		t.Fatalf("timeout: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.Invoke(ctx, Invocation{PluginID: "demo"}); !errors.Is(err, ErrInvocationCancelled) {
		t.Fatalf("cancel: %v", err)
	}
	if m := a.Metrics(); m.Cancelled != 2 {
		t.Fatalf("cancelled metrics: %+v", m)
	}
}

func TestMoeJSAdapterOutputLimit(t *testing.T) {
	rt := &fakeRuntime{invoke: func(context.Context, Invocation) (any, error) {
		return "0123456789", nil
	}}
	a := adapter(rt, Limits{MaxOutputBytes: 5})
	if err := a.Prepare(context.Background(), manifest("demo")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Invoke(context.Background(), Invocation{PluginID: "demo"}); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("output limit: %v", err)
	}
}

func TestMoeJSAdapterDisposeAndPoolLimit(t *testing.T) {
	rt := &fakeRuntime{}
	a := adapter(rt, Limits{MaxPoolSize: 1})
	if err := a.Prepare(context.Background(), manifest("one")); err != nil {
		t.Fatal(err)
	}
	if err := a.Prepare(context.Background(), manifest("two")); !errors.Is(err, ErrExecutorBusy) {
		t.Fatalf("pool limit: %v", err)
	}
	if err := a.Dispose(context.Background(), "one"); err != nil {
		t.Fatal(err)
	}
	if !rt.closed.Load() {
		t.Fatal("runtime was not closed")
	}
	if _, err := a.Invoke(context.Background(), Invocation{PluginID: "one"}); !errors.Is(err, ErrPluginNotPrepared) {
		t.Fatalf("after dispose: %v", err)
	}
}

func TestMoeJSAdapterConcurrencyLimit(t *testing.T) {
	var active, maxActive atomic.Int32
	rt := &fakeRuntime{invoke: func(context.Context, Invocation) (any, error) {
		n := active.Add(1)
		for {
			old := maxActive.Load()
			if n <= old || maxActive.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return "ok", nil
	}}
	a := adapter(rt, Limits{MaxConcurrent: 2, MaxDeadline: time.Second})
	if err := a.Prepare(context.Background(), manifest("demo")); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := a.Invoke(context.Background(), Invocation{PluginID: "demo"}); err != nil {
				t.Errorf("invoke: %v", err)
			}
		}()
	}
	wg.Wait()
	if maxActive.Load() > 2 {
		t.Fatalf("max concurrency %d > 2", maxActive.Load())
	}
	if m := a.Metrics(); m.Invocations != 6 {
		t.Fatalf("invocations: %+v", m)
	}
}

func BenchmarkMoeJSAdapterInvoke(b *testing.B) {
	rt := &fakeRuntime{invoke: func(context.Context, Invocation) (any, error) { return "ok", nil }}
	a := adapter(rt, Limits{MaxDeadline: time.Second})
	if err := a.Prepare(context.Background(), manifest("demo")); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := a.Invoke(context.Background(), Invocation{PluginID: "demo"}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestMoeJSAdapterDisposeWaitsForInflight(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	rt := &fakeRuntime{invoke: func(context.Context, Invocation) (any, error) {
		close(started)
		<-release
		return "ok", nil
	}}
	a := adapter(rt, Limits{MaxDeadline: time.Second})
	if err := a.Prepare(context.Background(), manifest("demo")); err != nil {
		t.Fatal(err)
	}
	go func() {
		_, _ = a.Invoke(context.Background(), Invocation{PluginID: "demo"})
	}()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := a.Dispose(ctx, "demo"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("dispose should wait for in-flight work: %v", err)
	}
	close(release)
	if err := a.Dispose(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if !rt.closed.Load() {
		t.Fatal("runtime was not closed after in-flight work finished")
	}
}
