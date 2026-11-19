package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// GitHub mirrors: routes the index and the artifacts may take, ranked by a
// speed test. A mirror can make a download faster but never change it: the
// index's signature and each artifact's size and SHA-256 are checked on
// whatever route brought them, and a route that fails a check is passed
// over for the next. Contract: docs/specs/0025-plugin-marketplace.md.

// Mirror is a route for GitHub files. Template, empty for direct, is an
// HTTPS URL holding {url}, the full GitHub URL, or {path}, the URL without
// its scheme and host; a mirror of raw files only may use {owner}, {repo},
// {ref} and {file} instead. Kinds are the files it serves: raw, release
// and archive.
type Mirror struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Template string   `json:"template"`
	Kinds    []string `json:"kinds"`
	Custom   bool     `json:"custom,omitempty"`
}

const (
	KindRaw     = "raw"
	KindRelease = "release"
	KindArchive = "archive"

	// DirectMirror is GitHub itself.
	DirectMirror = "direct"
	// MirrorAuto ranks the mirrors by the speed test.
	MirrorAuto = "auto"
)

var allKinds = []string{KindRaw, KindRelease, KindArchive}

// BuiltinMirrors are direct and the public mirrors that served raw files,
// release assets and clones in the tests cited by the spec (2026-11).
// coursive-plugins does not publish the signed mirror list yet.
var BuiltinMirrors = []Mirror{
	{ID: DirectMirror, Name: "GitHub 直连", Kinds: allKinds},
	{ID: "ghfast", Name: "ghfast.top", Template: "https://ghfast.top/{url}", Kinds: allKinds},
	{ID: "gh-proxy", Name: "gh-proxy.com", Template: "https://gh-proxy.com/{url}", Kinds: allKinds},
	{ID: "ghproxy-net", Name: "ghproxy.net", Template: "https://ghproxy.net/{url}", Kinds: allKinds},
}

// MirrorConfig is the user's choice, the sources.mirrors setting: Mode is
// auto or a mirror's id, Custom the mirrors the user added.
type MirrorConfig struct {
	Mode   string   `json:"mode"`
	Custom []Mirror `json:"custom"`
}

// MirrorSetting is the key MirrorConfig is kept under.
const MirrorSetting = "sources.mirrors"

// ParseMirrorConfig reads the setting; anything unreadable is auto.
func ParseMirrorConfig(raw json.RawMessage) MirrorConfig {
	var c MirrorConfig
	json.Unmarshal(raw, &c)
	if c.Mode == "" {
		c.Mode = MirrorAuto
	}
	return c
}

// ProbeResult is one route's speed test: Latency is the time to the
// index's first byte, Speed its bytes per second.
type ProbeResult struct {
	ID      string  `json:"id"`
	OK      bool    `json:"ok"`
	Latency int64   `json:"latency_ms"`
	Speed   float64 `json:"speed"`
	Error   string  `json:"error,omitempty"`
	took    time.Duration
}

// SpeedTest is the last test of every route.
type SpeedTest struct {
	At      time.Time     `json:"at"`
	Results []ProbeResult `json:"results"`
}

// MirrorState is what the 下载线路 page shows.
type MirrorState struct {
	Mode    string     `json:"mode"`
	Mirrors []Mirror   `json:"mirrors"`
	Test    *SpeedTest `json:"test"`
	// Using is the route the index takes first.
	Using string `json:"using"`
}

const (
	probeDeadline = 6 * time.Second
	probeFresh    = 24 * time.Hour
	probeParallel = 4
)

var mirrorID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// githubKind is the kind of a GitHub file URL, or "" for any other.
func githubKind(u *url.URL) string {
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	switch {
	case u.Scheme != "https":
		return ""
	case u.Host == "raw.githubusercontent.com" && len(parts) >= 4:
		return KindRaw
	case u.Host == "github.com" && len(parts) > 4 && parts[2] == "releases" && parts[3] == "download":
		return KindRelease
	case u.Host == "github.com" && len(parts) > 3 && parts[2] == "archive":
		return KindArchive
	}
	return ""
}

// rewrite is u through the mirror, false when it does not serve u's kind.
func (m Mirror) rewrite(u *url.URL) (string, bool) {
	kind := githubKind(u)
	if kind == "" || !slices.Contains(m.Kinds, kind) {
		return "", false
	}
	if m.Template == "" {
		return u.String(), true
	}
	t := m.Template
	if strings.Contains(t, "{owner}") {
		if kind != KindRaw {
			return "", false
		}
		p := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 4)
		return strings.NewReplacer("{owner}", p[0], "{repo}", p[1], "{ref}", p[2], "{file}", p[3]).Replace(t), true
	}
	path := strings.TrimPrefix(u.EscapedPath(), "/")
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return strings.NewReplacer("{url}", u.Scheme+"://"+u.Host+"/"+path, "{path}", path).Replace(t), true
}

// ValidateMirror checks a mirror the user adds.
func ValidateMirror(m Mirror) error {
	if !mirrorID.MatchString(m.ID) || slices.ContainsFunc(BuiltinMirrors, func(b Mirror) bool { return b.ID == m.ID }) || m.ID == MirrorAuto {
		return errorf(KindInvalid, "线路标识应为小写字母、数字和连字符，且不与内置线路重名")
	}
	if strings.TrimSpace(m.Name) == "" || len(m.Name) > 40 {
		return errorf(KindInvalid, "请填写线路名称")
	}
	u, err := url.Parse(strings.NewReplacer("{url}", "x", "{path}", "x", "{owner}", "x", "{repo}", "x", "{ref}", "x", "{file}", "x").Replace(m.Template))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errorf(KindInvalid, "线路地址需要 https")
	}
	raw := strings.Contains(m.Template, "{owner}")
	if !raw && !strings.Contains(m.Template, "{url}") && !strings.Contains(m.Template, "{path}") {
		return errorf(KindInvalid, "线路地址需要包含 {url} 或 {path}")
	}
	if raw && (!strings.Contains(m.Template, "{file}") || slices.ContainsFunc(m.Kinds, func(k string) bool { return k != KindRaw })) {
		return errorf(KindInvalid, "按 {owner}/{repo} 改写的线路只能用于 raw 文件，且需要 {file}")
	}
	if len(m.Kinds) == 0 || slices.ContainsFunc(m.Kinds, func(k string) bool { return !slices.Contains(allKinds, k) }) {
		return errorf(KindInvalid, "线路类型应为 raw、release 或 archive")
	}
	return nil
}

// CheckMirrorConfig checks a mirror choice before it is saved: the mode is
// auto or a mirror that exists, and each mirror the user adds is valid and
// named once.
func (m *Market) CheckMirrorConfig(c MirrorConfig) error {
	seen := map[string]bool{}
	for _, x := range c.Custom {
		if err := ValidateMirror(x); err != nil {
			return err
		}
		if seen[x.ID] {
			return errorf(KindInvalid, "线路标识重复: %s", x.ID)
		}
		seen[x.ID] = true
	}
	if c.Mode != MirrorAuto && !seen[c.Mode] && !slices.ContainsFunc(m.builtin(), func(b Mirror) bool { return b.ID == c.Mode }) {
		return errorf(KindInvalid, "没有这条线路: %s", c.Mode)
	}
	return nil
}

// probeError is why a route failed, short and without its URL.
func probeError(err error) string {
	var dns *net.DNSError
	var op *net.OpError
	var ne net.Error
	var ue *url.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "超时"
	case errors.As(err, &dns):
		return "找不到服务器"
	case errors.As(err, &op):
		return "无法连接"
	case errors.As(err, &ue):
		return ue.Err.Error()
	}
	return err.Error()
}

// mirrorTests is the market's speed test state.
type mirrorTests struct {
	mu   sync.Mutex
	last *SpeedTest
	read bool
}

func (m *Market) config(ctx context.Context) MirrorConfig {
	if m.Config == nil {
		return MirrorConfig{Mode: MirrorAuto}
	}
	return m.Config(ctx)
}

// mirrors are the built-in mirrors and the user's.
func (m *Market) mirrors(c MirrorConfig) []Mirror {
	out := append([]Mirror(nil), m.builtin()...)
	for _, x := range c.Custom {
		if ValidateMirror(x) == nil {
			x.Custom = true
			out = append(out, x)
		}
	}
	return out
}

func (m *Market) builtin() []Mirror {
	if m.Builtin != nil {
		return m.Builtin
	}
	return BuiltinMirrors
}

// routes are the mirrors serving kind in the order a download tries them:
// the chosen one first; then, by the speed test, those that passed,
// fastest first, those not tested, and those that failed.
func (m *Market) routes(ctx context.Context, kind string) []Mirror {
	c := m.config(ctx)
	all := m.mirrors(c)
	test := m.lastTest()
	if c.Mode == MirrorAuto && (test == nil || time.Since(test.At) > probeFresh) {
		test = m.SpeedTest(ctx)
	}
	rank := func(id string) (int, time.Duration) {
		if c.Mode == id {
			return 0, 0
		}
		if test != nil {
			for _, r := range test.Results {
				if r.ID == id {
					if r.OK {
						return 1, r.took
					}
					return 3, 0
				}
			}
		}
		return 2, 0
	}
	out := []Mirror{}
	for _, x := range all {
		if slices.Contains(x.Kinds, kind) {
			out = append(out, x)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, ti := rank(out[i].ID)
		rj, tj := rank(out[j].ID)
		if ri != rj {
			return ri < rj
		}
		return ti < tj
	})
	return out
}

// SpeedTest fetches the signed index through every route that serves raw
// files, four at a time, each within 6 s, and checks its signature: a
// mirror that changed it fails. The result is kept for 24 h.
func (m *Market) SpeedTest(ctx context.Context) *SpeedTest {
	u, err := url.Parse(m.IndexURL)
	if err != nil || m.IndexURL == "" {
		return nil
	}
	var probes []Mirror
	for _, x := range m.mirrors(m.config(ctx)) {
		if slices.Contains(x.Kinds, KindRaw) {
			probes = append(probes, x)
		}
	}
	results := make([]ProbeResult, len(probes))
	sem := make(chan struct{}, probeParallel)
	var wg sync.WaitGroup
	for i, x := range probes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = m.probe(ctx, x, u)
		}()
	}
	wg.Wait()
	t := &SpeedTest{At: time.Now(), Results: results}
	m.tests.mu.Lock()
	m.tests.last, m.tests.read = t, true
	m.tests.mu.Unlock()
	if m.Cache != "" {
		if b, err := json.Marshal(t); err == nil && os.MkdirAll(m.Cache, 0o700) == nil {
			os.WriteFile(filepath.Join(m.Cache, "mirrors.json"), b, 0o600)
		}
	}
	return t
}

func (m *Market) probe(ctx context.Context, x Mirror, index *url.URL) ProbeResult {
	r := ProbeResult{ID: x.ID}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), probeDeadline)
	defer cancel()
	via, _ := x.rewrite(index)
	start := time.Now()
	var first time.Duration
	body, err := m.getTimed(ctx, via, func() { first = time.Since(start) })
	took := time.Since(start)
	var sig []byte
	if err == nil {
		sigURL, _ := url.Parse(m.IndexURL + ".sig")
		via, _ := x.rewrite(sigURL)
		sig, err = m.get(ctx, via)
	}
	if err == nil {
		_, err = m.verify(body, sig)
		if err != nil {
			err = errors.New("内容与签名不符")
		}
	}
	if err != nil {
		r.Error = probeError(err)
		return r
	}
	r.OK, r.took = true, took
	r.Latency = first.Milliseconds()
	if s := took.Seconds(); s > 0 {
		r.Speed = float64(len(body)) / s
	}
	return r
}

// lastTest is the last speed test, read from the cache once.
func (m *Market) lastTest() *SpeedTest {
	m.tests.mu.Lock()
	defer m.tests.mu.Unlock()
	if !m.tests.read && m.Cache != "" {
		m.tests.read = true
		if b, err := os.ReadFile(filepath.Join(m.Cache, "mirrors.json")); err == nil {
			var t SpeedTest
			if json.Unmarshal(b, &t) == nil {
				for i := range t.Results {
					if t.Results[i].Speed > 0 {
						t.Results[i].took = time.Duration(float64(time.Second) / t.Results[i].Speed)
					}
				}
				m.tests.last = &t
			}
		}
	}
	return m.tests.last
}

// Mirrors is the routes' state; it does not start a speed test.
func (m *Market) Mirrors(ctx context.Context) MirrorState {
	c := m.config(ctx)
	st := MirrorState{Mode: c.Mode, Mirrors: m.mirrors(c), Test: m.lastTest()}
	if rs := m.routesNoTest(ctx); len(rs) > 0 {
		st.Using = rs[0].ID
	}
	return st
}

// routesNoTest is routes for raw files as they stand.
func (m *Market) routesNoTest(ctx context.Context) []Mirror {
	c := m.config(ctx)
	if c.Mode != MirrorAuto {
		return m.routes(ctx, KindRaw)
	}
	if t := m.lastTest(); t == nil || time.Since(t.At) > probeFresh {
		return nil
	}
	return m.routes(ctx, KindRaw)
}

// Forget drops the fetched index, so that the next listing fetches it
// along the routes as they now stand.
func (m *Market) Forget() {
	m.ixMu.Lock()
	m.ixAt = time.Time{}
	m.ixMu.Unlock()
}

// getTimed is get, calling first once the response's headers are in.
func (m *Market) getTimed(ctx context.Context, u string, first func()) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	first()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxIndex+1))
	if err == nil && len(b) > maxIndex {
		err = errors.New("插件目录超过 8 MiB")
	}
	return b, err
}
