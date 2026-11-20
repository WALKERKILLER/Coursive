package core

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// DaemonState is the stage the daemon reached. DaemonSetup is Android's
// uninstalled workspace, which the desktop never reaches.
type DaemonState int

const (
	DaemonSetup DaemonState = iota
	DaemonStarting
	DaemonReady
	DaemonFailed
)

const readyPrefix = "COURSIVE_READY "

// Daemon starts and supervises the Go daemon beside the app, as
// daemon_service.dart does: the token goes in COURSIVE_TOKEN, the child
// answers on stdout with "COURSIVE_READY {port, library}", and
// -exit-on-stdin-eof makes it exit when the app closes its stdin. It
// restarts the daemon when it exits, backing off, and gives up after five
// restarts in a row.
type Daemon struct {
	mu          sync.Mutex
	state       DaemonState
	err         string
	token       string
	port        int
	libraryRoot string
	log         []string
	restarts    int
	disposed    bool
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	exited      chan struct{}

	// OnChange runs on a background goroutine whenever the state changes.
	OnChange func()
}

// NewDaemon is a daemon supervisor with a fresh token.
func NewDaemon() *Daemon { return &Daemon{state: DaemonStarting, token: newToken()} }

func newToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (d *Daemon) changed() {
	if f := d.OnChange; f != nil {
		f()
	}
}

func (d *Daemon) State() DaemonState { d.mu.Lock(); defer d.mu.Unlock(); return d.state }
func (d *Daemon) Error() string      { d.mu.Lock(); defer d.mu.Unlock(); return d.err }
func (d *Daemon) Token() string      { return d.token }
func (d *Daemon) Port() int          { d.mu.Lock(); defer d.mu.Unlock(); return d.port }
func (d *Daemon) BaseURL() string    { return fmt.Sprintf("http://127.0.0.1:%d", d.Port()) }

// LibraryRoot is the library directory the daemon reported.
func (d *Daemon) LibraryRoot() string { d.mu.Lock(); defer d.mu.Unlock(); return d.libraryRoot }

// Log is the daemon's last 400 lines of output.
func (d *Daemon) Log() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.log...)
}

func (d *Daemon) logLine(line string) {
	d.mu.Lock()
	d.log = append(d.log, line)
	if n := len(d.log); n > 400 {
		d.log = append([]string(nil), d.log[n-400:]...)
	}
	d.mu.Unlock()
}

func (d *Daemon) fail(msg string) {
	d.mu.Lock()
	d.state, d.err = DaemonFailed, msg
	d.mu.Unlock()
	d.changed()
}

// Start launches the daemon.
func (d *Daemon) Start() { go d.spawn() }

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return filepath.Dir(exe)
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// find is _find: an environment override, a file bundled beside the
// executable, then the development tree, walking up from the executable
// (and, for go run, from the working directory).
func find(envKey string, bundled, devRel []string) string {
	if env := os.Getenv(envKey); env != "" && isFile(env) {
		return env
	}
	dir := exeDir()
	for _, b := range bundled {
		if f := filepath.Join(dir, b); isFile(f) {
			return f
		}
	}
	starts := []string{dir}
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	for _, start := range starts {
		d := start
		for i := 0; i < 10; i++ {
			for _, rel := range devRel {
				if f := filepath.Join(d, rel); isFile(f) {
					return f
				}
			}
			parent := filepath.Dir(d)
			if parent == d {
				break
			}
			d = parent
		}
	}
	return ""
}

// FindNode is _findNode: COURSIVE_NODE, PATH, mise's installs, then the
// usual places, as apps started from a launcher get a minimal PATH. Off
// Windows it skips a node that cannot start, such as one a partial system
// upgrade left without its shared libraries.
func FindNode() string {
	if env := os.Getenv("COURSIVE_NODE"); env != "" && isFile(env) {
		return env
	}
	win := runtime.GOOS == "windows"
	exe, sep, home := "node", ":", os.Getenv("HOME")
	if win {
		exe, sep, home = "node.exe", ";", os.Getenv("USERPROFILE")
	}
	var candidates []string
	for _, dir := range strings.Split(os.Getenv("PATH"), sep) {
		if dir != "" {
			candidates = append(candidates, filepath.Join(dir, exe))
		}
	}
	mise := filepath.Join(home, ".local/share/mise/installs/node")
	if entries, err := os.ReadDir(mise); err == nil {
		var versions []string
		for _, e := range entries {
			if e.IsDir() {
				versions = append(versions, filepath.Join(mise, e.Name()))
			}
		}
		sort.Strings(versions)
		for i := len(versions) - 1; i >= 0; i-- {
			if win {
				candidates = append(candidates, filepath.Join(versions[i], exe))
			} else {
				candidates = append(candidates, filepath.Join(versions[i], "bin", exe))
			}
		}
	}
	if win {
		pf := os.Getenv("ProgramFiles")
		if pf == "" {
			pf = `C:\Program Files`
		}
		candidates = append(candidates, filepath.Join(pf, "nodejs", exe))
	} else {
		candidates = append(candidates, "/usr/bin/node", "/usr/local/bin/node", "/opt/homebrew/bin/node")
	}
	for _, c := range candidates {
		if isFile(c) && (win || nodeRuns(c)) {
			return c
		}
	}
	return ""
}

func nodeRuns(node string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, node, "--version").Run() == nil
}

// MacOSDaemonPath keeps the inherited PATH first and adds the package
// managers' locations, as Finder loads no shell profile.
func MacOSDaemonPath(inherited string) string {
	seen := map[string]bool{}
	var out []string
	for _, p := range append(strings.Split(inherited, ":"), "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin") {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return strings.Join(out, ":")
}

func (d *Daemon) spawn() {
	exe := "coursive-daemon"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	bin := find("COURSIVE_DAEMON", []string{exe, "lib/" + exe}, []string{"coursive/daemon/bin/" + exe, "daemon/bin/" + exe})
	if bin == "" {
		d.fail(l10n.DaemonNotFound)
		return
	}
	agent := find("COURSIVE_AGENT", []string{"agent/src/host.mjs", "../Resources/agent/src/host.mjs"}, []string{"coursive/agent/src/host.mjs", "agent/src/host.mjs"})
	node := FindNode()
	args := []string{"-exit-on-stdin-eof"}
	// Development overrides, so a run can point at a throwaway library.
	if v := os.Getenv("COURSIVE_DATA"); v != "" {
		args = append(args, "-data", v)
	}
	if v := os.Getenv("COURSIVE_LIBRARY"); v != "" {
		args = append(args, "-library", v)
	}
	if agent != "" {
		args = append(args, "-agent", agent)
	}
	if node != "" {
		args = append(args, "-node", node)
	}
	env := append(os.Environ(), "COURSIVE_TOKEN="+d.token)
	if runtime.GOOS == "darwin" {
		env = append(env, "PATH="+MacOSDaemonPath(os.Getenv("PATH")))
	}
	d.launch(bin, args, env)
}

func (d *Daemon) launch(exe string, args, env []string) {
	d.mu.Lock()
	d.state = DaemonStarting
	d.mu.Unlock()
	d.changed()
	cmd := exec.Command(exe, args...)
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		d.fail(l10n.DaemonCannotStart(err))
		return
	}
	stderr, _ := cmd.StderrPipe()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		d.fail(l10n.DaemonCannotStart(err))
		return
	}
	if err := cmd.Start(); err != nil {
		d.fail(l10n.DaemonCannotStart(err))
		return
	}
	exited := make(chan struct{})
	d.mu.Lock()
	d.cmd, d.stdin, d.exited = cmd, stdin, exited
	d.mu.Unlock()
	go d.lines(stderr, nil)
	done := make(chan struct{})
	go d.lines(stdout, done)
	go func() {
		<-done
		_ = cmd.Wait()
		close(exited)
		code := cmd.ProcessState.ExitCode()
		d.mu.Lock()
		if d.disposed {
			d.mu.Unlock()
			return
		}
		d.state = DaemonStarting
		d.restarts++
		restarts := d.restarts
		d.mu.Unlock()
		d.logLine(fmt.Sprintf("daemon exited with code %d", code))
		d.changed()
		if restarts > 5 {
			d.fail(l10n.DaemonKeepsExiting(code))
			return
		}
		time.Sleep(time.Duration(400*restarts*restarts) * time.Millisecond)
		d.mu.Lock()
		disposed := d.disposed
		d.mu.Unlock()
		if !disposed {
			d.spawn()
		}
	}()
}

// lines reads the daemon's output; on stdout it watches for the handshake.
func (d *Daemon) lines(r io.Reader, done chan struct{}) {
	if r == nil {
		return
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if done != nil && strings.HasPrefix(line, readyPrefix) {
			var info struct {
				Port    int    `json:"port"`
				Library string `json:"library"`
			}
			if json.Unmarshal([]byte(line[len(readyPrefix):]), &info) == nil {
				d.mu.Lock()
				d.port, d.libraryRoot = info.Port, info.Library
				d.state, d.err, d.restarts = DaemonReady, "", 0
				d.mu.Unlock()
				d.changed()
				continue
			}
		}
		d.logLine(line)
	}
	if done != nil {
		close(done)
	}
}

// Restart stops the daemon; the exit handler starts it again.
func (d *Daemon) Restart() {
	d.mu.Lock()
	d.restarts = 0
	cmd := d.cmd
	d.cmd = nil
	failed := d.state == DaemonFailed
	d.mu.Unlock()
	if cmd != nil && cmd.Process != nil && !failed {
		terminate(cmd)
		return
	}
	go d.spawn()
}

// Dispose closes the daemon's stdin, which it exits on, and waits for it a
// little before stopping it.
func (d *Daemon) Dispose() {
	d.mu.Lock()
	d.disposed = true
	stdin, cmd, exited := d.stdin, d.cmd, d.exited
	d.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd == nil || cmd.Process == nil {
		return
	}
	select {
	case <-exited:
	case <-time.After(3 * time.Second):
		terminate(cmd)
	}
}

// terminate sends SIGTERM, or kills the process where signals don't exist.
func terminate(cmd *exec.Cmd) {
	if cmd.Process.Signal(syscall.SIGTERM) != nil {
		_ = cmd.Process.Kill()
	}
}
