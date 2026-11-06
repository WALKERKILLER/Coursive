package sources

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Source plugins listed in a plugin index, installed by downloading the
// artifact for this device, checking its size and SHA-256, and unpacking it
// into staging before the user consents.
// Contract: docs/specs/draft-plugin-marketplace.md

// PluginAPI is the plugin API version manifests' engine ranges match.
const PluginAPI = "2.0.0"

const (
	maxArtifact = 256 << 20
	stagedTTL   = 15 * time.Minute
)

// Index is the plugin index (index.json, schema version 1).
type Index struct {
	SchemaVersion int          `json:"schemaVersion"`
	Plugins       []IndexEntry `json:"plugins"`
}

// IndexEntry is one plugin of the index.
type IndexEntry struct {
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	Summary    string         `json:"summary"`
	Icon       string         `json:"icon"`
	Author     string         `json:"author"`
	Homepage   string         `json:"homepage"`
	Categories []string       `json:"categories"`
	Kinds      []string       `json:"kinds"`
	Official   bool           `json:"official"`
	Versions   []IndexVersion `json:"versions"`
}

// IndexVersion is one released version and its artifacts.
type IndexVersion struct {
	Version   string     `json:"version"`
	Engine    string     `json:"engine"`
	Published string     `json:"published"`
	Artifacts []Artifact `json:"artifacts"`
}

// Artifact is a downloadable package for one platform.
type Artifact struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
}

// Listing is a source plugin as the market shows it.
type Listing struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	Icon      string `json:"icon"`
	Author    string `json:"author"`
	Homepage  string `json:"homepage"`
	Official  bool   `json:"official"`
	Version   string `json:"version"`
	Available bool   `json:"available"`
	// Installed is the installed version, empty when not installed.
	Installed string `json:"installed"`
	Update    bool   `json:"update"`
}

// Staged is a verified, unpacked artifact awaiting consent.
type Staged struct {
	Token  string     `json:"token"`
	Plugin PluginInfo `json:"plugin"`
}

type staged struct {
	dir, root, id string
	at            time.Time
}

// Market reads one plugin index. Index is the path of a local index.json,
// whose artifact URLs are paths relative to it or GitHub addresses; an
// empty Index lists nothing.
type Market struct {
	Index    string
	Staging  string
	Registry *Registry
	HTTP     *http.Client

	mu     sync.Mutex
	staged map[string]staged
}

func (m *Market) read() (*Index, error) {
	if m.Index == "" {
		return &Index{SchemaVersion: 1}, nil
	}
	b, err := os.ReadFile(m.Index)
	if err != nil {
		return nil, errorf(KindUnavailable, "无法读取插件目录：%v", err)
	}
	var ix Index
	if err := json.Unmarshal(b, &ix); err != nil || ix.SchemaVersion != 1 {
		return nil, errorf(KindInvalid, "无法读取插件目录：格式不受支持")
	}
	return &ix, nil
}

// pick returns the newest version whose engine matches and that has an
// artifact for this device.
func pick(e IndexEntry) (IndexVersion, Artifact, bool) {
	vs := append([]IndexVersion(nil), e.Versions...)
	sort.SliceStable(vs, func(i, j int) bool { return compareVersions(vs[i].Version, vs[j].Version) > 0 })
	for _, v := range vs {
		if !engineMatches(v.Engine, PluginAPI) {
			continue
		}
		for _, a := range v.Artifacts {
			if a.Platform == Platform() || a.Platform == "any" {
				return v, a, true
			}
		}
	}
	return IndexVersion{}, Artifact{}, false
}

func hasKind(e IndexEntry, k string) bool {
	for _, x := range e.Kinds {
		if x == k {
			return true
		}
	}
	return false
}

// List returns the index's source plugins with their install state.
func (m *Market) List() ([]Listing, error) {
	ix, err := m.read()
	if err != nil {
		return nil, err
	}
	installed := map[string]string{}
	for _, p := range m.Registry.Plugins() {
		installed[p.ID] = p.Version
	}
	out := []Listing{}
	for _, e := range ix.Plugins {
		if !idRE.MatchString(e.ID) || !hasKind(e, "sources") {
			continue
		}
		l := Listing{ID: e.ID, Title: e.Title, Summary: e.Summary, Icon: listedIcon(e.Icon), Author: e.Author, Homepage: e.Homepage,
			Official: e.Official, Installed: installed[e.ID]}
		if v, _, ok := pick(e); ok {
			l.Version, l.Available = v.Version, true
		} else if len(e.Versions) > 0 {
			l.Version = e.Versions[0].Version
		}
		l.Update = l.Installed != "" && l.Available && compareVersions(l.Version, l.Installed) > 0
		out = append(out, l)
	}
	return out, nil
}

// Stage downloads and verifies id's artifact and unpacks it for consent.
func (m *Market) Stage(ctx context.Context, id string) (Staged, error) {
	ix, err := m.read()
	if err != nil {
		return Staged{}, err
	}
	var entry *IndexEntry
	for i := range ix.Plugins {
		if ix.Plugins[i].ID == id && hasKind(ix.Plugins[i], "sources") {
			entry = &ix.Plugins[i]
		}
	}
	if entry == nil {
		return Staged{}, errorf(KindInvalid, "插件目录中没有 %s", id)
	}
	v, a, ok := pick(*entry)
	if !ok {
		return Staged{}, errorf(KindInvalid, "「%s」没有适用于本机（%s）的版本", entry.Title, Platform())
	}
	body, err := m.fetch(ctx, a)
	if err != nil {
		return Staged{}, err
	}
	m.sweep()
	if err := os.MkdirAll(m.Staging, 0o700); err != nil {
		return Staged{}, err
	}
	dir, err := os.MkdirTemp(m.Staging, "stage-")
	if err != nil {
		return Staged{}, err
	}
	fail := func(err error) (Staged, error) { os.RemoveAll(dir); return Staged{}, err }
	if err := untar(body, dir); err != nil {
		return fail(errorf(KindInvalid, "无法解包「%s」：%v", entry.Title, err))
	}
	root := dir
	if _, err := os.Stat(filepath.Join(root, "package.json")); err != nil {
		root = filepath.Join(dir, "package") // npm pack layout
	}
	man, err := ReadManifest(root)
	if err != nil {
		return fail(err)
	}
	if man.Coursive.ID != entry.ID || man.Version != v.Version {
		return fail(errorf(KindInvalid, "无法安装「%s」：插件包是 %s@%s，目录登记的是 %s@%s",
			entry.Title, man.Coursive.ID, man.Version, entry.ID, v.Version))
	}
	info, err := m.Registry.Preview(root)
	if err != nil {
		return fail(err)
	}
	var t [12]byte
	rand.Read(t[:])
	token := hex.EncodeToString(t[:])
	m.mu.Lock()
	if m.staged == nil {
		m.staged = map[string]staged{}
	}
	m.staged[token] = staged{dir: dir, root: root, id: entry.ID, at: time.Now()}
	m.mu.Unlock()
	return Staged{Token: token, Plugin: info}, nil
}

// Install moves a staged plugin into place.
func (m *Market) Install(token string) (PluginInfo, error) {
	m.mu.Lock()
	s, ok := m.staged[token]
	delete(m.staged, token)
	m.mu.Unlock()
	if !ok {
		return PluginInfo{}, errorf(KindInvalid, "无法安装：下载已过期，请重新安装")
	}
	defer os.RemoveAll(s.dir)
	return m.Registry.Install(s.root)
}

// sweep removes staged artifacts nobody installed.
func (m *Market) sweep() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, s := range m.staged {
		if time.Since(s.at) > stagedTTL {
			os.RemoveAll(s.dir)
			delete(m.staged, k)
		}
	}
}

// fetch reads an artifact and checks its size and SHA-256.
func (m *Market) fetch(ctx context.Context, a Artifact) ([]byte, error) {
	if a.Size <= 0 || a.Size > maxArtifact {
		return nil, errorf(KindInvalid, "插件包大小无效")
	}
	var r io.ReadCloser
	u, err := url.Parse(a.URL)
	switch {
	case err != nil:
		return nil, errorf(KindInvalid, "插件包地址无效")
	case u.Scheme == "":
		p := filepath.FromSlash(path.Clean("/" + u.Path))[1:] // stays below the index
		f, err := os.Open(filepath.Join(filepath.Dir(m.Index), p))
		if err != nil {
			return nil, errorf(KindUnavailable, "无法读取插件包：%v", err)
		}
		r = f
	case u.Scheme == "https" && artifactHost(u):
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
		if err != nil {
			return nil, err
		}
		hc := m.HTTP
		if hc == nil {
			hc = http.DefaultClient
		}
		resp, err := hc.Do(req)
		if err != nil {
			return nil, errorf(KindUnavailable, "无法下载插件包：%v", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, errorf(KindUnavailable, "无法下载插件包：HTTP %d", resp.StatusCode)
		}
		r = resp.Body
	default:
		return nil, errorf(KindInvalid, "插件包地址不是 GitHub 发布文件：%s", u.Host)
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, a.Size+1))
	if err != nil {
		return nil, errorf(KindUnavailable, "无法下载插件包：%v", err)
	}
	sum := sha256.Sum256(b)
	if int64(len(b)) != a.Size || hex.EncodeToString(sum[:]) != strings.ToLower(a.SHA256) {
		return nil, errorf(KindInvalid, "插件包校验失败：内容与插件目录登记的不一致")
	}
	return b, nil
}

func artifactHost(u *url.URL) bool {
	if u.Host == "raw.githubusercontent.com" {
		return true
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	return u.Host == "github.com" && len(parts) > 4 && parts[2] == "releases" && parts[3] == "download"
}

// untar unpacks a gzipped tar into dir, refusing absolute paths, "..",
// links and devices.
func untar(b []byte, dir string) error {
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	var total int64
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(h.Name, "./")
		if name == "" || name == "." {
			continue
		}
		clean := path.Clean(name)
		if path.IsAbs(name) || strings.Contains(name, "\\") || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("路径 %q 越出插件目录", h.Name)
		}
		target := filepath.Join(dir, filepath.FromSlash(clean))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			total += h.Size
			if total > maxArtifact {
				return errors.New("解包后超过 256 MiB")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			mode := os.FileMode(0o600)
			if h.Mode&0o111 != 0 {
				mode = 0o700
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, io.LimitReader(tr, h.Size)); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%q 是链接或特殊文件，插件不能包含", h.Name)
		}
	}
}

// compareVersions orders semver strings by their numeric core; a
// prerelease sorts before its release.
func compareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	ra, rb := strings.Contains(a, "-"), strings.Contains(b, "-")
	switch {
	case ra && !rb:
		return -1
	case rb && !ra:
		return 1
	}
	return 0
}

func versionParts(v string) [3]int {
	v, _, _ = strings.Cut(strings.TrimPrefix(v, "v"), "-")
	var out [3]int
	for i, p := range strings.SplitN(v, ".", 3) {
		out[i], _ = strconv.Atoi(p)
	}
	return out
}

// engineMatches supports "*", "^x.y.z", ">=x.y.z" and an exact version.
func engineMatches(rng, v string) bool {
	rng = strings.TrimSpace(rng)
	switch {
	case rng == "" || rng == "*":
		return true
	case strings.HasPrefix(rng, "^"):
		want := versionParts(rng[1:])
		return versionParts(v)[0] == want[0] && compareVersions(v, rng[1:]) >= 0
	case strings.HasPrefix(rng, ">="):
		return compareVersions(v, strings.TrimSpace(rng[2:])) >= 0
	}
	return compareVersions(v, rng) == 0
}
