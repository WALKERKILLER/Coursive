package pluginfabric

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Executor errors are deliberately distinct so the caller can tell a plugin
// fault from a host policy decision and from cancellation.
var (
	ErrExecutorUnavailable = errors.New("plugin executor unavailable")
	ErrPluginNotPrepared   = errors.New("plugin executor not prepared")
	ErrInvocationTimeout   = errors.New("plugin invocation timed out")
	ErrInvocationCancelled = errors.New("plugin invocation cancelled")
	ErrOutputTooLarge      = errors.New("plugin output exceeds limit")
	ErrHostAPIDenied       = errors.New("host API not allowed")
	ErrModuleDenied        = errors.New("module not allowed")
	ErrExecutorBusy        = errors.New("plugin executor is busy")
)

// ExecutorCapabilities describes one optional runtime family. The host uses
// this to decide whether a contribution can run on this device.
type ExecutorCapabilities struct {
	Kind                 string
	SupportsCancellation bool
	MaxConcurrent        int
	MaxMemoryBytes       int64
	MaxDeadline          time.Duration
}

// Executor is the runtime-neutral seam for an optional plugin engine. The
// Node Pi host keeps its own path; this interface is for bounded executors
// such as the MoeJS prototype.
type Executor interface {
	Prepare(ctx context.Context, manifest *Manifest) error
	Invoke(ctx context.Context, req Invocation) (InvocationResult, error)
	Dispose(ctx context.Context, pluginID string) error
	Capabilities() ExecutorCapabilities
}

// Invocation is one bounded call into a plugin runtime. HostAPIs and Modules
// are requests, not grants: the adapter checks them against its allowlists.
type Invocation struct {
	PluginID string
	Action   string
	Input    any
	HostAPIs []string
	Modules  []string
	Deadline time.Duration
	TraceID  string
}

// Result is a successful invocation. Output is JSON-marshalled once to enforce
// the output bound before the caller sees it.
type InvocationResult struct {
	Output   any
	Duration time.Duration
}

// Limits are the host's hard bounds for an executor. A zero field means the
// adapter's default applies.
type Limits struct {
	MaxMemoryBytes int64
	MaxOutputBytes int
	MaxDeadline    time.Duration
	MaxPoolSize    int
	MaxConcurrent  int
}

// Runtime is one isolated execution context. A real MoeJS engine implements
// this behind RuntimeFactory; tests use a fake.
type ExecutorRuntime interface {
	Invoke(ctx context.Context, req Invocation) (any, error)
	Close() error
}

// RuntimeFactory creates a runtime for one prepared plugin. Compilation and
// engine setup belong here so the adapter can reuse the compiled runtime.
type RuntimeFactory interface {
	New(ctx context.Context, manifest *Manifest, limits Limits, hostAPIs, modules []string) (ExecutorRuntime, error)
}

// MetricsSnapshot is a copy safe to hand to diagnostics or a benchmark.
type MetricsSnapshot struct {
	Invocations  int64
	Errors       int64
	Cancelled    int64
	Average      time.Duration
	P50          time.Duration
	P95          time.Duration
	RSSDeltaMB   float64
	CPUTimeMS    float64
	LastDuration time.Duration
}

type metrics struct {
	mu          sync.Mutex
	invocations int64
	errors      int64
	cancelled   int64
	total       time.Duration
	last        time.Duration
	durations   []time.Duration
	rssDeltaMB  float64
	cpuTimeMS   float64
}

func (m *metrics) record(d time.Duration, err error, cancelled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invocations++
	m.total += d
	m.last = d
	m.durations = append(m.durations, d)
	if err != nil {
		m.errors++
	}
	if cancelled {
		m.cancelled++
	}
}

func (m *metrics) snapshot() MetricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := MetricsSnapshot{Invocations: m.invocations, Errors: m.errors, Cancelled: m.cancelled,
		RSSDeltaMB: m.rssDeltaMB, CPUTimeMS: m.cpuTimeMS, LastDuration: m.last}
	if m.invocations > 0 {
		out.Average = m.total / time.Duration(m.invocations)
	}
	if len(m.durations) > 0 {
		sorted := append([]time.Duration(nil), m.durations...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		out.P50 = percentile(sorted, 0.50)
		out.P95 = percentile(sorted, 0.95)
	}
	return out
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := int(float64(len(sorted)-1) * p)
	return sorted[index]
}

type preparedPlugin struct {
	manifest *Manifest
	runtime  ExecutorRuntime
	sem      chan struct{}
	wg       sync.WaitGroup
	closing  atomic.Bool
	hostAPIs map[string]bool
	modules  map[string]bool
}

// MoeJSAdapter is the optional bounded executor prototype. It owns a runtime
// per prepared plugin, a concurrency semaphore and the host allowlists. It
// never holds a daemon token, the vault or a database handle.
type MoeJSAdapter struct {
	Factory  RuntimeFactory
	Limits   Limits
	HostAPIs []string
	Modules  []string

	mu      sync.Mutex
	plugins map[string]*preparedPlugin
	metrics metrics
}

const (
	defaultMaxOutputBytes = 1 << 20
	defaultMaxDeadline    = 5 * time.Second
	defaultMaxConcurrent  = 4
	defaultMaxPoolSize    = 32
)

func (a *MoeJSAdapter) limits() Limits {
	l := a.Limits
	if l.MaxOutputBytes <= 0 {
		l.MaxOutputBytes = defaultMaxOutputBytes
	}
	if l.MaxDeadline <= 0 {
		l.MaxDeadline = defaultMaxDeadline
	}
	if l.MaxConcurrent <= 0 {
		l.MaxConcurrent = defaultMaxConcurrent
	}
	if l.MaxPoolSize <= 0 {
		l.MaxPoolSize = defaultMaxPoolSize
	}
	return l
}

func (a *MoeJSAdapter) Capabilities() ExecutorCapabilities {
	l := a.limits()
	return ExecutorCapabilities{Kind: "moejs", SupportsCancellation: true, MaxConcurrent: l.MaxConcurrent,
		MaxMemoryBytes: l.MaxMemoryBytes, MaxDeadline: l.MaxDeadline}
}

// Prepare compiles one plugin runtime. It is idempotent for the same plugin id.
func (a *MoeJSAdapter) Prepare(ctx context.Context, manifest *Manifest) error {
	if a.Factory == nil {
		return ErrExecutorUnavailable
	}
	if manifest == nil || manifest.Coursive.ID == "" {
		return fmt.Errorf("plugin manifest has no id")
	}
	id := manifest.Coursive.ID
	a.mu.Lock()
	if a.plugins == nil {
		a.plugins = map[string]*preparedPlugin{}
	}
	if _, ok := a.plugins[id]; ok {
		a.mu.Unlock()
		return nil
	}
	l := a.limits()
	if len(a.plugins) >= l.MaxPoolSize {
		a.mu.Unlock()
		return fmt.Errorf("%w: runtime pool is full", ErrExecutorBusy)
	}
	hostAPIs := setOf(a.HostAPIs)
	modules := setOf(a.Modules)
	a.mu.Unlock()

	runtime, err := a.Factory.New(ctx, manifest, l, a.HostAPIs, a.Modules)
	if err != nil {
		return fmt.Errorf("prepare plugin %s: %w", id, err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.plugins[id]; ok {
		_ = runtime.Close()
		return nil
	}
	a.plugins[id] = &preparedPlugin{manifest: manifest, runtime: runtime, sem: make(chan struct{}, l.MaxConcurrent), hostAPIs: hostAPIs, modules: modules}
	return nil
}

// Invoke runs one action with a deadline, concurrency bound and output bound.
// A cancelled or timed-out call never records a successful result.
func (a *MoeJSAdapter) Invoke(ctx context.Context, req Invocation) (InvocationResult, error) {
	a.mu.Lock()
	p := a.plugins[req.PluginID]
	a.mu.Unlock()
	if p == nil || p.closing.Load() {
		return InvocationResult{}, ErrPluginNotPrepared
	}
	for _, api := range req.HostAPIs {
		if !p.hostAPIs[api] {
			return InvocationResult{}, fmt.Errorf("%w: %s", ErrHostAPIDenied, api)
		}
	}
	for _, module := range req.Modules {
		if !p.modules[module] {
			return InvocationResult{}, fmt.Errorf("%w: %s", ErrModuleDenied, module)
		}
	}

	l := a.limits()
	deadline := req.Deadline
	if deadline <= 0 || deadline > l.MaxDeadline {
		deadline = l.MaxDeadline
	}
	callCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	select {
	case p.sem <- struct{}{}:
		defer func() { <-p.sem }()
		p.wg.Add(1)
		defer p.wg.Done()
	case <-callCtx.Done():
		a.metrics.record(0, callCtx.Err(), true)
		return InvocationResult{}, invocationContextError(callCtx)
	}

	start := time.Now()
	out, err := p.runtime.Invoke(callCtx, req)
	duration := time.Since(start)
	if callCtx.Err() != nil {
		a.metrics.record(duration, callCtx.Err(), true)
		return InvocationResult{}, invocationContextError(callCtx)
	}
	if err != nil {
		a.metrics.record(duration, err, false)
		return InvocationResult{}, err
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		a.metrics.record(duration, err, false)
		return InvocationResult{}, fmt.Errorf("plugin output is not JSON: %w", err)
	}
	if len(encoded) > l.MaxOutputBytes {
		a.metrics.record(duration, ErrOutputTooLarge, false)
		return InvocationResult{}, fmt.Errorf("%w: %d > %d", ErrOutputTooLarge, len(encoded), l.MaxOutputBytes)
	}
	a.metrics.record(duration, nil, false)
	return InvocationResult{Output: out, Duration: duration}, nil
}

func invocationContextError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", ErrInvocationTimeout, ctx.Err())
	}
	return fmt.Errorf("%w: %v", ErrInvocationCancelled, ctx.Err())
}

// Dispose closes one plugin runtime. It is safe to call more than once.
func (a *MoeJSAdapter) Dispose(ctx context.Context, pluginID string) error {
	a.mu.Lock()
	p := a.plugins[pluginID]
	a.mu.Unlock()
	if p == nil {
		return nil
	}
	p.closing.Store(true)
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		a.mu.Lock()
		if a.plugins[pluginID] == p {
			delete(a.plugins, pluginID)
		}
		a.mu.Unlock()
		return p.runtime.Close()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Metrics returns a copy of the adapter's counters.
func (a *MoeJSAdapter) Metrics() MetricsSnapshot { return a.metrics.snapshot() }
