package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// daemonState is the stage the daemon reached, as the window shows it.
type daemonState int

const (
	stateSetup daemonState = iota
	stateStarting
	stateReady
	stateFailed
)

func (s daemonState) String() string {
	switch s {
	case stateSetup:
		return "setup"
	case stateStarting:
		return "starting"
	case stateReady:
		return "ready"
	default:
		return "failed"
	}
}

// daemon owns the coursive-daemon child process: it starts it with the token
// the client authenticates with, reads the port the daemon reports on stdout,
// restarts it when it exits, and closes it with the window.
//
// The contract is the Flutter client's (app/lib/core/daemon_service.dart):
// the token goes in COURSIVE_TOKEN, the child answers on stdout with
// "COURSIVE_READY <json>" carrying "port" and "library", and
// -exit-on-stdin-eof makes it exit when the client closes stdin.
type daemon struct {
	mu       sync.Mutex
	token    string
	port     int
	library  string
	state    daemonState
	err      error
	restarts int
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stopped  bool

	// onChange runs on the daemon's own goroutine when the state changes; the
	// app hands it a Window.Update so the view rebuilds on the main thread.
	onChange func()
}

const readyPrefix = "COURSIVE_READY "

func newDaemon() *daemon {
	return &daemon{token: newToken()}
}

// newToken is 24 random bytes as hex, the shape the Flutter client mints.
func newToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("daemon token: %v", err))
	}
	return hex.EncodeToString(b)
}

func (d *daemon) changed() {
	d.mu.Lock()
	fn := d.onChange
	d.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (d *daemon) snapshot() (state daemonState, port int, library string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state, d.port, d.library, d.err
}

func (d *daemon) fail(err error) {
	d.mu.Lock()
	d.state, d.err = stateFailed, err
	d.mu.Unlock()
	d.changed()
}

// start launches the daemon and waits for it to report its port.
func (d *daemon) start() {
	d.mu.Lock()
	d.state, d.err = stateStarting, nil
	d.mu.Unlock()
	d.changed()
	go d.run()
}

func (d *daemon) run() {
	for {
		d.mu.Lock()
		stopped := d.stopped
		d.mu.Unlock()
		if stopped {
			return
		}

		exe, err := findDaemon()
		if err != nil {
			d.fail(err)
			return
		}
		args := []string{"-exit-on-stdin-eof"}
		// A prototype must never open the real library, so both directories can
		// be pointed elsewhere: tests use a temporary pair, and a developer can
		// aim at a copy of a sanitised library.
		if data := os.Getenv("COURSIVE_DATA"); data != "" {
			args = append(args, "-data", data)
		}
		if library := os.Getenv("COURSIVE_LIBRARY"); library != "" {
			args = append(args, "-library", library)
		}
		if agent, ok := findAgent(); ok {
			args = append(args, "-agent", agent)
		}
		if node, ok := findNode(); ok {
			args = append(args, "-node", node)
		}

		cmd := exec.Command(exe, args...)
		cmd.Env = append(os.Environ(), "COURSIVE_TOKEN="+d.token)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			d.fail(err)
			return
		}
		stdin, err := cmd.StdinPipe()
		if err != nil {
			d.fail(err)
			return
		}
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			d.fail(fmt.Errorf("start %s: %w", filepath.Base(exe), err))
			return
		}

		d.mu.Lock()
		d.cmd, d.stdin = cmd, stdin
		d.mu.Unlock()

		ready := make(chan struct{})
		go d.readReady(stdout, ready)

		err = cmd.Wait()

		d.mu.Lock()
		stopped, restarts := d.stopped, d.restarts
		d.mu.Unlock()
		if stopped {
			return
		}

		select {
		case <-ready:
			// It ran and then exited: restart it, backing off.
			restarts++
			d.mu.Lock()
			d.restarts = restarts
			d.state = stateStarting
			d.mu.Unlock()
			d.changed()
			if restarts > 5 {
				d.fail(fmt.Errorf("daemon keeps exiting (last: %v)", err))
				return
			}
			time.Sleep(time.Duration(400*restarts*restarts) * time.Millisecond)
		default:
			d.fail(fmt.Errorf("daemon exited before it was ready: %v", err))
			return
		}
	}
}

// readReady scans the daemon's stdout for the handshake line and closes ready
// once the port is known.
func (d *daemon) readReady(r io.Reader, ready chan<- struct{}) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	once := sync.Once{}
	for sc.Scan() {
		line := sc.Text()
		if rest, ok := cut(line, readyPrefix); ok {
			var info struct {
				Port    int    `json:"port"`
				Library string `json:"library"`
			}
			if err := json.Unmarshal([]byte(rest), &info); err != nil {
				d.fail(fmt.Errorf("daemon handshake: %w", err))
				return
			}
			d.mu.Lock()
			d.port, d.library = info.Port, info.Library
			d.state, d.err, d.restarts = stateReady, nil, 0
			d.mu.Unlock()
			once.Do(func() { close(ready) })
			d.changed()
			continue
		}
		fmt.Fprintln(os.Stderr, "daemon:", line)
	}
}

func cut(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || s[:len(prefix)] != prefix {
		return "", false
	}
	return s[len(prefix):], true
}

// stop closes the daemon's stdin, which -exit-on-stdin-eof turns into a clean
// exit, and waits for it.
func (d *daemon) stop() {
	d.mu.Lock()
	d.stopped = true
	stdin, cmd := d.stdin, d.cmd
	d.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
		}
	}
}

// findDaemon looks where the Flutter client looks: an explicit override, the
// directory beside the executable, then the development tree.
func findDaemon() (string, error) {
	name := "coursive-daemon"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var candidates []string
	if env := os.Getenv("COURSIVE_DAEMON"); env != "" {
		candidates = append(candidates, env)
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(dir, name), filepath.Join(dir, "lib", name))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "daemon", "bin", name),
			filepath.Join(wd, "..", "daemon", "bin", name))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("coursive-daemon not found; set COURSIVE_DAEMON or build daemon/bin/%s", name)
}

func findAgent() (string, bool) {
	if env := os.Getenv("COURSIVE_AGENT"); env != "" {
		if st, err := os.Stat(env); err == nil && !st.IsDir() {
			return env, true
		}
	}
	rel := filepath.Join("agent", "src", "host.mjs")
	if wd, err := os.Getwd(); err == nil {
		for _, c := range []string{filepath.Join(wd, rel), filepath.Join(wd, "..", rel)} {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c, true
			}
		}
	}
	return "", false
}

// findNode follows the Flutter client's search: PATH, then mise's installs,
// then the usual system locations. A window opened from a launcher gets a
// minimal PATH, so the list matters.
func findNode() (string, bool) {
	win := runtime.GOOS == "windows"
	exe := "node"
	if win {
		exe = "node.exe"
	}
	if p, err := exec.LookPath(exe); err == nil {
		return p, true
	}
	home, _ := os.UserHomeDir()
	sep := ":"
	if win {
		sep = ";"
	}
	_ = sep
	var candidates []string
	versions, _ := filepath.Glob(filepath.Join(home, ".local", "share", "mise", "installs", "node", "*", "bin", exe))
	for i := len(versions) - 1; i >= 0; i-- {
		candidates = append(candidates, versions[i])
	}
	candidates = append(candidates, "/usr/bin/"+exe, "/usr/local/bin/"+exe, "/opt/homebrew/bin/"+exe)
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, true
		}
	}
	return "", false
}
