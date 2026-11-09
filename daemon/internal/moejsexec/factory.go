// Package moejsexec runs the `moejs` runtime component of a plugin on the
// MoeJS engine (github.com/MoeForge/moejs, Apache-2.0) behind
// pluginfabric.RuntimeFactory. It is an optional, experimental executor:
// nothing on the daemon's default paths imports it, so a build without it has
// the same attack surface (decision 0036).
//
// A runtime sees only what the factory gives it: the plugin's own package
// modules, the host modules the adapter allowlists, and the host functions
// under the global `coursive` whose permission the plugin declared and holds.
// It has no file, network, process or timer API, eval and new Function are
// disabled, and it never holds a daemon token, the vault or a database handle.
package moejsexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/MoeForge/moejs"

	"github.com/walkerkiller/coursive/daemon/internal/pluginfabric"
)

var (
	// ErrMemoryLimit means a call allocated past the runtime's memory limit.
	// The runtime is discarded; the next call gets a fresh one.
	ErrMemoryLimit = errors.New("plugin exceeded its memory limit")
	// ErrScript is a JavaScript throw or rejection, with its stack.
	ErrScript = errors.New("plugin script failed")
	// ErrNoEntry means the manifest declares no moejs runtime component.
	ErrNoEntry = errors.New("plugin has no moejs runtime")

	actionRE = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]{0,63}$`)
)

// HostAPI is one host function a script calls as `coursive.<name>(...)`; a
// dotted name such as library.read nests (`coursive.library.read`).
type HostAPI struct {
	// Permission is the manifest permission the plugin must declare and hold;
	// empty needs none.
	Permission string
	// Call receives the arguments as JSON-shaped Go values and the context of
	// the invocation, which is cancelled with it.
	Call func(ctx context.Context, args []any) (any, error)
}

// Factory compiles a plugin's moejs entry once and serves its invocations from
// a pool of runtimes that loaded the compiled module.
type Factory struct {
	// Root is the read-only package folder of an installed plugin. Modules are
	// read through os.Root, so no path, link or `..` leaves it.
	Root func(m *pluginfabric.Manifest) (string, error)
	// APIs are the host functions this daemon offers to scripts.
	APIs map[string]HostAPI
	// HostModules are ES module sources a script may import by bare name,
	// when the adapter also allowlists the name.
	HostModules map[string]string
	// Granted reports the permissions the user granted the plugin; nil grants
	// none, so only host functions without a permission are installed.
	Granted func(pluginID string) []string
}

// New compiles and links the plugin's moejs entry. It reads every module the
// entry imports now, so a missing, escaping or unlisted import fails here and
// never at call time.
func (f *Factory) New(_ context.Context, m *pluginfabric.Manifest, limits pluginfabric.Limits, hostAPIs, modules []string) (pluginfabric.ExecutorRuntime, error) {
	entry := ""
	for _, r := range m.Coursive.Runtime {
		if r.Kind == "moejs" {
			entry = r.Entry.Path
			break
		}
	}
	if entry == "" {
		return nil, ErrNoEntry
	}
	if f.Root == nil {
		return nil, pluginfabric.ErrExecutorUnavailable
	}
	dir, err := f.Root(m)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	allowed := map[string]bool{}
	for _, name := range modules {
		allowed[name] = true
	}
	compiled := map[string]*moejs.Module{}
	compile := func(name, source string) (*moejs.Module, error) {
		if mod, ok := compiled[name]; ok {
			return mod, nil
		}
		mod, err := moejs.Compile(name, source)
		if err != nil {
			return nil, err
		}
		compiled[name] = mod
		return mod, nil
	}
	read := func(rel string) (*moejs.Module, error) {
		if !pluginPath(rel) {
			return nil, fmt.Errorf("%w: %s leaves the plugin package", pluginfabric.ErrModuleDenied, rel)
		}
		b, err := root.ReadFile(rel)
		if err != nil {
			return nil, err
		}
		return compile(rel, string(b))
	}
	resolve := func(ref moejs.Referrer, specifier string) (*moejs.Module, error) {
		from := ref.Name()
		if strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") {
			if strings.HasPrefix(from, "host:") {
				return nil, fmt.Errorf("%w: host module %s imports %s", pluginfabric.ErrModuleDenied, from, specifier)
			}
			return read(path.Join(path.Dir(from), specifier))
		}
		source, ok := f.HostModules[specifier]
		if !ok || !allowed[specifier] {
			return nil, fmt.Errorf("%w: %s", pluginfabric.ErrModuleDenied, specifier)
		}
		return compile("host:"+specifier, source)
	}
	mod, err := read(path.Clean(entry))
	if err != nil {
		return nil, err
	}
	if mod, err = moejs.Link(mod, resolve); err != nil {
		return nil, err
	}

	size := limits.MaxConcurrent
	if size <= 0 {
		size = 1
	}
	return &runtime{
		module: mod,
		apis:   f.installable(m, hostAPIs),
		limits: limits,
		idle:   make(chan *slot, size),
		free:   make(chan struct{}, size),
		hooks:  map[string]moejs.Hook{},
	}, nil
}

// installable is the host functions a plugin's runtimes get: offered by the
// daemon, allowlisted by the adapter, and declared and granted when they need
// a permission.
func (f *Factory) installable(m *pluginfabric.Manifest, allowlist []string) map[string]HostAPI {
	declared := map[string]bool{}
	for _, p := range m.Coursive.Permissions {
		declared[p] = true
	}
	granted := map[string]bool{}
	if f.Granted != nil {
		for _, p := range f.Granted(m.Coursive.ID) {
			granted[p] = true
		}
	}
	out := map[string]HostAPI{}
	for _, name := range allowlist {
		api, ok := f.APIs[name]
		if ok && (api.Permission == "" || (declared[api.Permission] && granted[api.Permission])) {
			out[name] = api
		}
	}
	return out
}

// pluginPath reports whether rel is a clean path inside the package.
func pluginPath(rel string) bool {
	return rel != "" && rel != "." && !path.IsAbs(rel) && path.Clean(rel) == rel && rel != ".." && !strings.HasPrefix(rel, "../")
}

// runtime is one prepared plugin: the compiled module and a pool of runtimes
// that loaded it. free bounds how many exist; idle holds the ones not in use.
type runtime struct {
	module *moejs.Module
	apis   map[string]HostAPI
	limits pluginfabric.Limits
	idle   chan *slot
	free   chan struct{}

	mu     sync.Mutex
	hooks  map[string]moejs.Hook
	closed bool
}

// slot is one engine runtime and the context of the call it is running, which
// its host functions read.
type slot struct {
	rt  *moejs.Runtime
	ctx context.Context
}

func (r *runtime) hook(action string) (moejs.Hook, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.hooks[action]; ok {
		return h, nil
	}
	h, err := r.module.Hook(action)
	if err != nil {
		return moejs.Hook{}, err
	}
	r.hooks[action] = h
	return h, nil
}

// acquire takes an idle runtime or makes one while the pool has room.
func (r *runtime) acquire(ctx context.Context) (*slot, error) {
	select {
	case s := <-r.idle:
		return s, nil
	default:
	}
	select {
	case s := <-r.idle:
		return s, nil
	case r.free <- struct{}{}:
		s, err := r.newSlot()
		if err != nil {
			<-r.free
		}
		return s, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *runtime) newSlot() (*slot, error) {
	s := &slot{}
	s.rt = moejs.NewRuntime(moejs.Options{
		MemoryLimit:        r.limits.MaxMemoryBytes,
		DisableDynamicCode: true,
		TimeZone:           time.UTC,
	})
	api := map[string]any{}
	for name, h := range r.apis {
		// library.read is installed as coursive.library.read.
		at, parts := api, strings.Split(name, ".")
		for _, p := range parts[:len(parts)-1] {
			next, ok := at[p].(map[string]any)
			if !ok {
				next = map[string]any{}
				at[p] = next
			}
			at = next
		}
		at[parts[len(parts)-1]] = moejs.NativeFunc(func(realm *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			in := make([]any, len(args))
			for i, a := range args {
				in[i] = realm.ToGo(a)
			}
			out, err := h.Call(s.ctx, in)
			if err != nil {
				return moejs.Undefined(), err
			}
			return realm.FromGo(out)
		})
	}
	if err := s.rt.SetGlobal("coursive", api); err != nil {
		return nil, err
	}
	if err := s.rt.Load(r.module); err != nil {
		return nil, err
	}
	return s, nil
}

// release returns a runtime to the pool, or drops it when it can no longer be
// trusted to be in a clean state.
func (r *runtime) release(s *slot, keep bool) {
	s.ctx = nil
	s.rt.ClearInterrupt()
	s.rt.ReleaseCallData()
	r.mu.Lock()
	closed := r.closed
	r.mu.Unlock()
	if keep && !closed {
		r.idle <- s
		return
	}
	<-r.free
}

// Invoke calls the module's exported function named by req.Action with
// req.Input, interrupting it when ctx ends. A promise is read as the jobs the
// call ran left it, so an async function that awaits only settled work
// returns its value.
func (r *runtime) Invoke(ctx context.Context, req pluginfabric.Invocation) (any, error) {
	if !actionRE.MatchString(req.Action) {
		return nil, fmt.Errorf("%w: action %q is not an exported function name", ErrScript, req.Action)
	}
	h, err := r.hook(req.Action)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrScript, err)
	}
	input, err := json.Marshal(req.Input)
	if err != nil {
		return nil, fmt.Errorf("input is not JSON: %w", err)
	}
	s, err := r.acquire(ctx)
	if err != nil {
		return nil, err
	}
	s.ctx = ctx
	stop := context.AfterFunc(ctx, func() { s.rt.Interrupt(context.Cause(ctx)) })
	out, err := r.call(s, h, input)
	stop()
	r.release(s, err == nil || errors.Is(err, ErrScript) || errors.Is(err, pluginfabric.ErrOutputTooLarge))
	return out, err
}

func (r *runtime) call(s *slot, h moejs.Hook, input []byte) (any, error) {
	arg, err := s.rt.ParseJSON(input)
	if err != nil {
		return nil, err
	}
	// A limit hit inside some built-ins (String.prototype.repeat) surfaces as
	// a RangeError the script can catch; the counter still moves, and any hit
	// fails the call.
	hits := s.rt.Stats().MemoryLimitHits
	res, err := s.rt.Call(h, arg)
	if s.rt.Stats().MemoryLimitHits > hits {
		return nil, fmt.Errorf("%w: %v", ErrMemoryLimit, s.rt.Stats().LastMemoryLimitError)
	}
	if err != nil {
		return nil, r.fail(s, err)
	}
	if state, value, ok := moejs.PromiseResult(res); ok {
		switch state {
		case moejs.PromisePending:
			return nil, fmt.Errorf("%w: the promise is still pending when the call returns", ErrScript)
		case moejs.PromiseRejected:
			b, _ := s.rt.AppendJSON(nil, value)
			return nil, fmt.Errorf("%w: rejected with %s", ErrScript, b)
		}
		res = value
	}
	out, err := s.rt.ToGoWith(res, moejs.DecodeOptions{MaxBytes: r.limits.MaxOutputBytes})
	if errors.Is(err, moejs.ErrTooLarge) {
		return nil, fmt.Errorf("%w: %v", pluginfabric.ErrOutputTooLarge, err)
	}
	if err != nil {
		return nil, r.fail(s, err)
	}
	return out, nil
}

// fail names an engine error: a memory-limit hit, an interrupt (the adapter
// reports the timeout or cancellation from its context) or a script error.
func (r *runtime) fail(s *slot, err error) error {
	if errors.Is(err, moejs.ErrMemoryLimit) {
		return fmt.Errorf("%w: %v", ErrMemoryLimit, err)
	}
	var interrupted *moejs.InterruptedError
	if errors.As(err, &interrupted) {
		return err
	}
	var exc *moejs.Exception
	if errors.As(err, &exc) {
		return fmt.Errorf("%w: %s", ErrScript, s.rt.StackTrace(exc))
	}
	return fmt.Errorf("%w: %v", ErrScript, err)
}

// Close drops the pool; runtimes in use are dropped when their call returns.
func (r *runtime) Close() error {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	for {
		select {
		case <-r.idle:
			<-r.free
		default:
			return nil
		}
	}
}
