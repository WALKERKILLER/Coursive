//go:build unix

// Command moejs-bench measures the MoeJS executor against the other ways a
// light plugin task can run (decision 0036): a Go native function in the
// daemon, MoeJS in the daemon, the same module inline in Node as a Pi
// extension runs it, and a Node worker over JSON-lines RPC as a process
// plugin runs. Each backend runs in its own child process, so the memory and
// CPU it reports are its own; a tool-call pass then times the paths the
// assistant host would take to reach each one.
//
//	cd daemon && go run ./cmd/moejs-bench -node "$(command -v node)" [-n 2000] [-subtitles lecture.srt] [-out report.json]
//
// -subtitles takes a real .srt or a {"cues": [...]} JSON file; without it the
// committed synthetic lecture fixture is used. The report prints no subtitle
// text. It is a development tool and is not part of the daemon build.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/moejsexec"
	"github.com/walkerkiller/coursive/daemon/internal/pluginfabric"
)

// task is one pilot call the backends all run.
type task struct {
	Name   string         `json:"name"`
	Action string         `json:"action"`
	Input  map[string]any `json:"input"`
}

type plan struct {
	Pilot string `json:"pilot"`
	N     int    `json:"n"`
	Tasks []task `json:"tasks"`
}

// timing is one task on one backend.
type timing struct {
	Task      string  `json:"task"`
	FirstMs   float64 `json:"first_action_ms"`
	P50Ms     float64 `json:"invoke_p50_ms"`
	P95Ms     float64 `json:"invoke_p95_ms"`
	Calls     int     `json:"calls"`
	Errors    int     `json:"errors"`
	LastError string  `json:"last_error,omitempty"`
}

// backendReport is what a child prints: setup cost, then each task.
type backendReport struct {
	Backend    string   `json:"backend"`
	StartupMs  float64  `json:"startup_ms"`
	RSSDeltaMB float64  `json:"rss_delta_mb"`
	CPUMs      float64  `json:"cpu_ms"`
	Tasks      []timing `json:"tasks"`
}

type cancelReport struct {
	Backend   string  `json:"backend"`
	Runs      int     `json:"runs"`
	Stopped   int     `json:"stopped"`
	MaxOverMs float64 `json:"max_overrun_ms"`
	RecoverMs float64 `json:"recover_ms"`
	Note      string  `json:"note,omitempty"`
}

type report struct {
	Environment map[string]string `json:"environment"`
	Input       map[string]any    `json:"input"`
	Backends    []backendReport   `json:"backends"`
	ToolCalls   []json.RawMessage `json:"tool_calls"`
	Cancel      []cancelReport    `json:"cancellation"`
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func main() {
	child := flag.String("child", "", "internal: run one backend from a plan file")
	planFile := flag.String("plan", "", "internal: the plan a child runs")
	node := flag.String("node", "node", "the Node binary")
	n := flag.Int("n", 2000, "timed calls per task and backend")
	pilot := flag.String("pilot", "internal/moejsexec/testdata/pilot", "the pilot module folder")
	subtitles := flag.String("subtitles", "", "a .srt or {cues} .json file (default: the synthetic fixture)")
	out := flag.String("out", "", "write the JSON report here")
	flag.Parse()

	if *child != "" {
		var p plan
		b, err := os.ReadFile(*planFile)
		must(err)
		must(json.Unmarshal(b, &p))
		var r backendReport
		switch *child {
		case "go-native":
			r = runNative(p)
		case "moejs":
			r = runMoeJS(p)
		case "process-rpc":
			r = runRPC(p, *node)
		default:
			must(fmt.Errorf("unknown backend %s", *child))
		}
		must(json.NewEncoder(os.Stdout).Encode(r))
		return
	}

	dir, err := filepath.Abs(*pilot)
	must(err)
	p, input := buildPlan(dir, *n, *subtitles)
	tmp, err := os.MkdirTemp("", "moejs-bench-")
	must(err)
	defer os.RemoveAll(tmp)
	planPath := filepath.Join(tmp, "plan.json")
	b, _ := json.Marshal(p)
	must(os.WriteFile(planPath, b, 0o600))

	rep := report{Environment: environment(*node), Input: input}
	self, err := os.Executable()
	must(err)
	for _, backend := range []string{"go-native", "moejs", "process-rpc"} {
		var r backendReport
		must(runJSON(exec.Command(self, "-child", backend, "-plan", planPath, "-node", *node), &r))
		rep.Backends = append(rep.Backends, r)
	}
	var inline backendReport
	must(runJSON(exec.Command(*node, filepath.Join(benchDir(), "node-inline.mjs"), planPath), &inline))
	rep.Backends = append(rep.Backends, inline)

	rep.ToolCalls = toolCalls(planPath, *node, p)
	rep.Cancel = []cancelReport{cancelMoeJS(dir), cancelRPC(dir, *node), {Backend: "node-inline",
		Note: "not measured: a synchronous loop blocks the host's event loop, so only a worker thread or a process can stop it"}}

	printReport(rep)
	if *out != "" {
		b, _ := json.MarshalIndent(rep, "", "  ")
		must(os.WriteFile(*out, append(b, '\n'), 0o644))
	}
}

// benchDir is where the Node halves of this command live.
func benchDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func runJSON(cmd *exec.Cmd, out any) error {
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.Join(cmd.Args, " "), err)
	}
	return json.Unmarshal(b, out)
}

func environment(node string) map[string]string {
	v, _ := exec.Command(node, "--version").Output()
	return map[string]string{"os": runtime.GOOS + "/" + runtime.GOARCH, "cpus": strconv.Itoa(runtime.NumCPU()),
		"go": runtime.Version(), "node": strings.TrimSpace(string(v)), "moejs": "v0.1.0"}
}

// buildPlan reads the pilot cases: the subtitle task from the given file or
// the fixture, the other two from cases.json.
func buildPlan(dir string, n int, subtitles string) (plan, map[string]any) {
	var cases struct {
		Cases []struct {
			Name, Action string
			Input        map[string]any
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	must(err)
	must(json.Unmarshal(b, &cases))
	p := plan{Pilot: dir, N: n}
	source := "synthetic fixture (testdata/pilot/subtitles.json)"
	if subtitles == "" {
		subtitles = filepath.Join(dir, "subtitles.json")
	} else {
		source = "a local subtitle file given with -subtitles"
	}
	cues := readCues(subtitles)
	p.Tasks = append(p.Tasks, task{Name: "subtitles", Action: "normalizeSubtitles", Input: map[string]any{"cues": cues}})
	for _, c := range cases.Cases {
		switch c.Name {
		case "notes":
			p.Tasks = append(p.Tasks, task{Name: "json-transform", Action: c.Action, Input: c.Input})
		case "watched":
			p.Tasks = append(p.Tasks, task{Name: "event-hook", Action: c.Action, Input: c.Input})
		}
	}
	chars := 0
	for _, c := range cues {
		chars += len([]rune(c["text"].(string)))
	}
	return p, map[string]any{"subtitles": source, "cues": len(cues), "subtitle_chars": chars, "calls_per_task": n}
}

// usage is this process's CPU time and resident memory.
func usage() (cpu time.Duration, rssKB int64) {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano()), rss(os.Getpid())
}

// rss reads VmRSS of a process, in KiB (Linux; 0 elsewhere).
func rss(pid int) int64 {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			v, _ := strconv.ParseInt(strings.Fields(line)[1], 10, 64)
			return v
		}
	}
	return 0
}

// measure times the first call and n more of one task.
func measure(name string, n int, call func() error) timing {
	t := timing{Task: name}
	start := time.Now()
	if err := call(); err != nil {
		t.Errors, t.LastError = 1, err.Error()
	}
	t.FirstMs = ms(time.Since(start))
	d := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		err := call()
		d = append(d, time.Since(start))
		if err != nil {
			t.Errors++
			t.LastError = err.Error()
		}
	}
	t.Calls = n + 1
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	if n > 0 {
		t.P50Ms, t.P95Ms = ms(d[(n-1)*50/100]), ms(d[(n-1)*95/100])
	}
	return t
}

func run(backend string, p plan, setup func() func(task) error) backendReport {
	cpu0, rss0 := usage()
	start := time.Now()
	call := setup()
	r := backendReport{Backend: backend, StartupMs: ms(time.Since(start))}
	for _, t := range p.Tasks {
		r.Tasks = append(r.Tasks, measure(t.Name, p.N, func() error { return call(t) }))
	}
	cpu1, rss1 := usage()
	r.CPUMs, r.RSSDeltaMB = ms(cpu1-cpu0), float64(rss1-rss0)/1024
	return r
}

func runNative(p plan) backendReport {
	return run("go-native", p, func() func(task) error {
		return func(t task) error {
			_, err := json.Marshal(native[t.Action](t.Input))
			return err
		}
	})
}

func pilotAdapter(dir string, limits pluginfabric.Limits) *pluginfabric.MoeJSAdapter {
	return &pluginfabric.MoeJSAdapter{Limits: limits, Factory: &moejsexec.Factory{
		Root: func(*pluginfabric.Manifest) (string, error) { return dir, nil },
	}}
}

var pilotManifest = &pluginfabric.Manifest{Name: "pilot", Version: "0.1.0", Coursive: pluginfabric.Coursive{
	ID: "pilot", Runtime: []pluginfabric.Runtime{{Kind: "moejs", Entry: pluginfabric.Entry{Path: "index.js"}}}}}

func runMoeJS(p plan) backendReport {
	return run("moejs", p, func() func(task) error {
		a := pilotAdapter(p.Pilot, pluginfabric.Limits{MaxMemoryBytes: 64 << 20, MaxConcurrent: 1})
		must(a.Prepare(context.Background(), pilotManifest))
		return func(t task) error {
			_, err := a.Invoke(context.Background(), pluginfabric.Invocation{PluginID: "pilot", Action: t.Action, Input: t.Input})
			return err
		}
	})
}

// worker is a Node process plugin speaking JSON lines.
type worker struct {
	cmd *exec.Cmd
	in  *json.Encoder
	out *bufio.Scanner
}

func startWorker(node, dir string) (*worker, time.Duration) {
	start := time.Now()
	cmd := exec.Command(node, filepath.Join(benchDir(), "rpc-worker.mjs"), dir)
	stdin, err := cmd.StdinPipe()
	must(err)
	stdout, err := cmd.StdoutPipe()
	must(err)
	cmd.Stderr = os.Stderr
	must(cmd.Start())
	w := &worker{cmd: cmd, in: json.NewEncoder(stdin), out: bufio.NewScanner(stdout)}
	w.out.Buffer(make([]byte, 1<<20), 64<<20)
	if !w.out.Scan() {
		must(errors.New("the RPC worker did not start"))
	}
	return w, time.Since(start)
}

func (w *worker) call(id int, t task) error {
	if err := w.in.Encode(map[string]any{"id": id, "action": t.Action, "input": t.Input}); err != nil {
		return err
	}
	if !w.out.Scan() {
		return errors.New("the RPC worker closed")
	}
	var res struct {
		ID    int    `json:"id"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.out.Bytes(), &res); err != nil {
		return err
	}
	if res.Error != "" || res.ID != id {
		return fmt.Errorf("rpc %d: %s", res.ID, res.Error)
	}
	return nil
}

func runRPC(p plan, node string) backendReport {
	var w *worker
	id := 0
	r := run("process-rpc", p, func() func(task) error {
		w, _ = startWorker(node, p.Pilot)
		return func(t task) error {
			id++
			return w.call(id, t)
		}
	})
	// The worker's own memory and CPU count against this backend.
	r.RSSDeltaMB += float64(rss(w.cmd.Process.Pid)) / 1024
	_ = w.cmd.Process.Signal(syscall.SIGTERM)
	_ = w.cmd.Wait()
	if ru, ok := w.cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
		r.CPUMs += ms(time.Duration(ru.Utime.Nano() + ru.Stime.Nano()))
	}
	return r
}

// cancelMoeJS runs an endless loop under a 50 ms deadline and checks that
// each run stops, how late, and that the next call still works.
func cancelMoeJS(dir string) cancelReport {
	a := pilotAdapter(dir, pluginfabric.Limits{MaxConcurrent: 1})
	must(a.Prepare(context.Background(), pilotManifest))
	r := cancelReport{Backend: "moejs", Runs: 30}
	var recover time.Duration
	for i := 0; i < r.Runs; i++ {
		start := time.Now()
		_, err := a.Invoke(context.Background(), pluginfabric.Invocation{PluginID: "pilot", Action: "spin", Deadline: 50 * time.Millisecond})
		if over := ms(time.Since(start)) - 50; errors.Is(err, pluginfabric.ErrInvocationTimeout) {
			r.Stopped++
			r.MaxOverMs = max(r.MaxOverMs, over)
		}
		start = time.Now()
		if _, err := a.Invoke(context.Background(), pluginfabric.Invocation{PluginID: "pilot", Action: "onEvent", Input: map[string]any{"type": "x"}}); err == nil {
			recover = max(recover, time.Since(start))
		}
	}
	r.RecoverMs = ms(recover)
	return r
}

// cancelRPC stops an endless loop in a worker by killing it at the deadline,
// and measures the restart a process plugin then needs.
func cancelRPC(dir, node string) cancelReport {
	r := cancelReport{Backend: "process-rpc", Runs: 10, Note: "stopped by killing the worker; recover is a fresh worker's start"}
	var recover time.Duration
	for i := 0; i < r.Runs; i++ {
		w, _ := startWorker(node, dir)
		start := time.Now()
		done := make(chan error, 1)
		go func() { done <- w.call(1, task{Action: "spin"}) }()
		select {
		case <-done:
		case <-time.After(50 * time.Millisecond):
			_ = w.cmd.Process.Kill()
			<-done
			_ = w.cmd.Wait()
			r.Stopped++
			r.MaxOverMs = max(r.MaxOverMs, ms(time.Since(start))-50)
		}
		next, took := startWorker(node, dir)
		recover = max(recover, took)
		_ = next.cmd.Process.Kill()
		_ = next.cmd.Wait()
	}
	r.RecoverMs = ms(recover)
	return r
}

// toolCalls serves the Go native and MoeJS backends on a loopback route shaped
// like a daemon tool route (bearer token, JSON in and out) and lets the Node
// half time every path the assistant host could take to run a tool.
func toolCalls(planPath, node string, p plan) []json.RawMessage {
	a := pilotAdapter(p.Pilot, pluginfabric.Limits{MaxMemoryBytes: 64 << 20, MaxConcurrent: 4})
	must(a.Prepare(context.Background(), pilotManifest))
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	token := hex.EncodeToString(key)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tools/{backend}", func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Action string         `json:"action"`
			Input  map[string]any `json:"input"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var out any
		switch req.PathValue("backend") {
		case "go-native":
			out = native[body.Action](body.Input)
		case "moejs":
			res, err := a.Invoke(req.Context(), pluginfabric.Invocation{PluginID: "pilot", Action: body.Action, Input: body.Input})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			out = res.Output
		default:
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"output": out})
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	must(err)
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	cmd := exec.Command(node, filepath.Join(benchDir(), "tool-call.mjs"), planPath, "http://"+ln.Addr().String())
	cmd.Env = append(os.Environ(), "BENCH_TOKEN="+token)
	var rows []json.RawMessage
	must(runJSON(cmd, &rows))
	return rows
}

func printReport(r report) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "Environment: %s, %s CPUs, %s, Node %s, MoeJS %s\n", r.Environment["os"], r.Environment["cpus"], r.Environment["go"], r.Environment["node"], r.Environment["moejs"])
	fmt.Fprintf(&b, "Input: %v cues (%v characters) from %v; %v timed calls per task\n\n", r.Input["cues"], r.Input["subtitle_chars"], r.Input["subtitles"], r.Input["calls_per_task"])
	fmt.Fprintln(&b, "| backend | startup_ms | rss_delta_mb | cpu_ms | task | first_action_ms | invoke_p50_ms | invoke_p95_ms | errors |")
	fmt.Fprintln(&b, "|---|---:|---:|---:|---|---:|---:|---:|---:|")
	for _, be := range r.Backends {
		for i, t := range be.Tasks {
			head := fmt.Sprintf("| %s | %.2f | %.1f | %.0f", be.Backend, be.StartupMs, be.RSSDeltaMB, be.CPUMs)
			if i > 0 {
				head = "| | | |"
			}
			fmt.Fprintf(&b, "%s | %s | %.3f | %.4f | %.4f | %d/%d |\n", head, t.Task, t.FirstMs, t.P50Ms, t.P95Ms, t.Errors, t.Calls)
		}
	}
	fmt.Fprintln(&b, "\nTool call from the assistant host:")
	fmt.Fprintln(&b, "| path | task | first_ms | p50_ms | p95_ms | errors |")
	fmt.Fprintln(&b, "|---|---|---:|---:|---:|---:|")
	for _, raw := range r.ToolCalls {
		var row struct {
			Path    string  `json:"path"`
			Task    string  `json:"task"`
			FirstMs float64 `json:"first_action_ms"`
			P50Ms   float64 `json:"invoke_p50_ms"`
			P95Ms   float64 `json:"invoke_p95_ms"`
			Errors  int     `json:"errors"`
			Calls   int     `json:"calls"`
		}
		_ = json.Unmarshal(raw, &row)
		fmt.Fprintf(&b, "| %s | %s | %.3f | %.4f | %.4f | %d/%d |\n", row.Path, row.Task, row.FirstMs, row.P50Ms, row.P95Ms, row.Errors, row.Calls)
	}
	fmt.Fprintln(&b, "\nCancellation of an endless loop (50 ms deadline):")
	fmt.Fprintln(&b, "| backend | stopped | max_overrun_ms | recover_ms | note |")
	fmt.Fprintln(&b, "|---|---:|---:|---:|---|")
	for _, c := range r.Cancel {
		if c.Runs == 0 {
			fmt.Fprintf(&b, "| %s | – | – | – | %s |\n", c.Backend, c.Note)
			continue
		}
		fmt.Fprintf(&b, "| %s | %d/%d | %.2f | %.2f | %s |\n", c.Backend, c.Stopped, c.Runs, c.MaxOverMs, c.RecoverMs, c.Note)
	}
	os.Stdout.Write(b.Bytes())
}
