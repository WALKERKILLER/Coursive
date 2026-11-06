package sources

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/vault"
)

// Supervision limits from the source-provider contract (variables so tests
// can shorten them).
var (
	initTimeout = 10 * time.Second
	callTimeout = 60 * time.Second
	backoff     = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
)

const (
	failureWindow  = 10 * time.Minute
	maxFailures    = 5
	stderrPerMin   = 64 << 10
	stderrLineCap  = 4 << 10
	redactedMarker = "‹已隐去›"
)

// Process states.
const (
	Stopped  = "stopped"
	Starting = "starting"
	Ready    = "ready"
	Failed   = "error"
)

// Provider supervises one source's executable.
type Provider struct {
	// ID is "<plugin>/<source>".
	ID      string
	Plugin  string
	Title   string
	Exe     string
	DataDir string
	Vault   *vault.Namespace
	// Changed is called after the process or sign-in state changes.
	Changed func()

	mu        sync.Mutex
	cmd       *exec.Cmd
	conn      *conn
	init      *InitResult
	state     string
	lastErr   string
	failures  []time.Time
	notBefore time.Time
	auth      Auth
	secrets   map[string]bool
	stopping  bool
}

// Snapshot is a provider's public state.
type Snapshot struct {
	ID           string   `json:"id"`
	Plugin       string   `json:"plugin"`
	Title        string   `json:"title"`
	Process      string   `json:"process"`
	Error        string   `json:"error,omitempty"`
	Auth         Auth     `json:"auth"`
	Capabilities []string `json:"capabilities"`
	Fields       []Field  `json:"fields"`
}

func (p *Provider) Snapshot() Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := Snapshot{ID: p.ID, Plugin: p.Plugin, Title: p.Title, Process: p.state, Error: p.lastErr, Auth: p.auth,
		Capabilities: []string{}, Fields: []Field{}}
	if s.Process == "" {
		s.Process = Stopped
	}
	if s.Auth.State == "" {
		s.Auth.State = SignedOut
	}
	if p.init != nil {
		s.Capabilities, s.Fields = p.init.Capabilities, p.init.SignIn.Fields
	}
	return s
}

func (p *Provider) changed() {
	if p.Changed != nil {
		p.Changed()
	}
}

// Has reports whether the running provider announced a capability group.
func (p *Provider) Has(capability string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.init != nil && p.init.has(capability)
}

// SignedIn reports whether the last known sign-in state is signed_in.
func (p *Provider) SignedIn() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.auth.State == SignedIn
}

// Call starts the provider if needed and performs one request.
func (p *Provider) Call(ctx context.Context, method string, params, out any) error {
	c, err := p.ensure(ctx)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	err = c.call(ctx, method, params, out)
	var pe *Error
	if errors.As(err, &pe) && pe.Kind == KindAuthRequired {
		p.setAuth(Auth{State: Expired})
	}
	return err
}

// Restart clears a failed provider so the next call starts it again.
func (p *Provider) Restart() {
	p.Stop()
	p.mu.Lock()
	p.failures, p.notBefore, p.lastErr, p.state = nil, time.Time{}, "", Stopped
	p.mu.Unlock()
	p.changed()
}

// Stop ends the process, if running.
func (p *Provider) Stop() {
	p.mu.Lock()
	cmd := p.cmd
	p.stopping = true
	p.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}

func (p *Provider) ensure(ctx context.Context) (*conn, error) {
	p.mu.Lock()
	if p.conn != nil {
		select {
		case <-p.conn.closed:
		default:
			c := p.conn
			p.mu.Unlock()
			return c, nil
		}
	}
	if p.state == Failed {
		msg := p.lastErr
		p.mu.Unlock()
		return nil, errorf(KindUnavailable, "来源「%s」已停止：%s。请在插件页重新启动", p.Title, msg)
	}
	wait := time.Until(p.notBefore)
	p.mu.Unlock()
	if wait > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return p.start(ctx)
}

func providerEnv() []string {
	keep := []string{"HOME", "TMPDIR", "LANG", "LC_ALL", "http_proxy", "https_proxy", "no_proxy", "all_proxy",
		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY", "SSL_CERT_DIR", "SSL_CERT_FILE"}
	if runtime.GOOS == "windows" {
		keep = append(keep, "SystemRoot", "TEMP", "TMP", "USERPROFILE", "LOCALAPPDATA")
	}
	env := []string{"COURSIVE_PROTOCOL=" + ProtocolVersion}
	for _, k := range keep {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func (p *Provider) start(ctx context.Context) (*conn, error) {
	p.mu.Lock()
	if p.conn != nil {
		select {
		case <-p.conn.closed:
		default:
			c := p.conn
			p.mu.Unlock()
			return c, nil
		}
	}
	p.state, p.stopping = Starting, false
	p.mu.Unlock()
	p.changed()

	if err := os.MkdirAll(p.DataDir, 0o700); err != nil {
		return nil, p.fail(err.Error())
	}
	cmd := exec.Command(p.Exe)
	cmd.Dir, cmd.Env = p.DataDir, providerEnv()
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return nil, p.fail("无法启动：" + err.Error())
	}
	c := newConn(stdout, stdin, p.serve)
	p.mu.Lock()
	p.cmd, p.conn = cmd, c
	p.mu.Unlock()
	go p.logStderr(stderr)
	go func() {
		err := cmd.Wait()
		c.close(errorf(KindUnavailable, "来源「%s」已退出", p.Title))
		p.mu.Lock()
		intentional := p.stopping || p.cmd != cmd
		if p.cmd == cmd {
			p.cmd, p.conn = nil, nil
			if p.state == Ready || p.state == Starting {
				p.state = Stopped
			}
		}
		p.mu.Unlock()
		if !intentional {
			msg := "进程意外退出"
			if err != nil {
				msg += "：" + err.Error()
			}
			p.fail(msg)
		}
		p.changed()
	}()

	ictx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	var in InitResult
	err := c.call(ictx, "initialize", map[string]any{"protocol": ProtocolVersion, "daemon_version": daemonVersion, "locale": "zh-CN"}, &in)
	if err == nil && in.Protocol != ProtocolVersion {
		err = errorf(KindInvalid, "协议版本 %q 不受支持（需要 %s）", in.Protocol, ProtocolVersion)
	}
	if err != nil {
		p.mu.Lock()
		p.stopping = true
		p.mu.Unlock()
		cmd.Process.Kill()
		return nil, p.fail("初始化失败：" + err.Error())
	}
	p.mu.Lock()
	p.init, p.state, p.lastErr = &in, Ready, ""
	p.mu.Unlock()

	var a Auth
	if err := c.call(ictx, "auth.status", struct{}{}, &a); err == nil {
		p.setAuth(a)
	} else {
		p.changed()
	}
	return c, nil
}

// fail records a failure, arms the backoff and, past the limit, stops the source.
func (p *Provider) fail(msg string) error {
	now := time.Now()
	p.mu.Lock()
	recent := p.failures[:0]
	for _, t := range p.failures {
		if now.Sub(t) < failureWindow {
			recent = append(recent, t)
		}
	}
	p.failures = append(recent, now)
	p.lastErr = msg
	n := len(p.failures)
	if n >= maxFailures {
		p.state = Failed
	} else {
		p.state = Stopped
		p.notBefore = now.Add(backoff[n-1])
	}
	p.mu.Unlock()
	slog.Warn("source provider failed", "source", p.ID, "failures", n, "err", p.redact(msg))
	p.changed()
	return errorf(KindUnavailable, "无法启动来源「%s」：%s", p.Title, msg)
}

func (p *Provider) setAuth(a Auth) {
	if a.State == "" {
		a.State = SignedOut
	}
	p.mu.Lock()
	p.auth = a
	p.mu.Unlock()
	p.changed()
}

// Secret marks a value (such as a password passed to sign-in) for redaction
// from provider diagnostics.
func (p *Provider) Secret(v string) {
	if len(v) < 4 {
		return
	}
	p.mu.Lock()
	if p.secrets == nil {
		p.secrets = map[string]bool{}
	}
	p.secrets[v] = true
	p.mu.Unlock()
}

func (p *Provider) redact(s string) string {
	p.mu.Lock()
	vals := make([]string, 0, len(p.secrets))
	for v := range p.secrets {
		vals = append(vals, v)
	}
	p.mu.Unlock()
	if p.Vault != nil {
		vals = append(vals, p.Vault.Values()...)
	}
	for _, v := range vals {
		if len(v) >= 4 {
			s = strings.ReplaceAll(s, v, redactedMarker)
		}
	}
	return redactPatterns(s)
}

var secretWords = []string{"password", "passwd", "token", "cookie", "authorization", "secret", "ticket", "session"}

// redactPatterns hides the value after a secret-looking key, such as
// "token=abc" or `"cookie": "abc"`.
func redactPatterns(s string) string {
	lower := strings.ToLower(s)
	for _, w := range secretWords {
		for from := 0; ; {
			i := strings.Index(lower[from:], w)
			if i < 0 {
				break
			}
			j := from + i + len(w)
			k := j
			for k < len(s) && strings.ContainsRune(`"' :=`, rune(s[k])) {
				k++
			}
			end := k
			for end < len(s) && !strings.ContainsRune("\"' ,;&\t", rune(s[end])) {
				end++
			}
			if k > j && end > k {
				s = s[:k] + redactedMarker + s[end:]
				lower = strings.ToLower(s)
				from = k + len(redactedMarker)
			} else {
				from = j
			}
		}
	}
	return s
}

func (p *Provider) logStderr(r io.Reader) {
	br := bufio.NewReaderSize(r, stderrLineCap)
	windowStart, used, dropped := time.Now(), 0, 0
	for {
		line, err := br.ReadSlice('\n')
		if len(line) > 0 {
			if time.Since(windowStart) >= time.Minute {
				if dropped > 0 {
					slog.Warn("source provider diagnostics dropped", "source", p.ID, "bytes", dropped)
				}
				windowStart, used, dropped = time.Now(), 0, 0
			}
			if used+len(line) > stderrPerMin {
				dropped += len(line)
			} else {
				used += len(line)
				slog.Info("source provider", "source", p.ID, "msg", p.redact(strings.TrimRight(string(line), "\r\n")))
			}
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			// Discard the rest of an overlong line.
			for errors.Is(err, bufio.ErrBufferFull) {
				_, err = br.ReadSlice('\n')
			}
		}
		if err != nil {
			return
		}
	}
}

// serve answers the provider's own requests.
func (p *Provider) serve(method string, params json.RawMessage) (any, error) {
	var in struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		State string `json:"state"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &in); err != nil {
			return nil, errors.New("invalid params")
		}
	}
	switch method {
	case "vault.get":
		v, ok, err := p.Vault.Get(in.Key)
		if err != nil {
			return nil, err
		}
		if !ok {
			return map[string]any{}, nil
		}
		return map[string]any{"value": v}, nil
	case "vault.put":
		return struct{}{}, p.Vault.Put(in.Key, in.Value)
	case "vault.delete":
		return struct{}{}, p.Vault.Delete(in.Key)
	case "notify":
		go p.refreshAuth()
		return nil, nil
	}
	return nil, errors.New("method not found: " + method)
}

func (p *Provider) refreshAuth() {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	var a Auth
	if err := p.Call(ctx, "auth.status", struct{}{}, &a); err == nil {
		p.setAuth(a)
	}
}

// daemonVersion is reported to providers in initialize; set by the server.
var daemonVersion = "dev"

// SetDaemonVersion records the daemon's version for initialize.
func SetDaemonVersion(v string) { daemonVersion = v }
