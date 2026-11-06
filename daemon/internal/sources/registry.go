package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/walkerkiller/coursive/daemon/internal/vault"
)

var idRE = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)

// Platform is this device's key in a manifest's executable map.
func Platform() string {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	osName := map[string]string{"linux": "linux", "windows": "windows", "darwin": "macos", "android": "linux"}[runtime.GOOS]
	if arch == "" || osName == "" {
		return runtime.GOOS + "-" + runtime.GOARCH
	}
	return osName + "-" + arch
}

// Manifest is the part of a plugin's package.json the daemon reads.
type Manifest struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Coursive struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Summary string `json:"summary"`
		// Icon is a Material Symbols name or an image in the package.
		Icon        string   `json:"icon"`
		Engine      string   `json:"engine"`
		Permissions []string `json:"permissions"`
		Contributes struct {
			Sources []SourceDecl `json:"sources"`
		} `json:"contributes"`
	} `json:"coursive"`
}

// SourceDecl is one entry of contributes.sources.
type SourceDecl struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Protocol   string            `json:"protocol"`
	Executable map[string]string `json:"executable"`
	Hosts      []string          `json:"hosts"`
}

// ReadManifest reads and validates dir/package.json as a source plugin.
func ReadManifest(dir string) (*Manifest, error) {
	b, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, errorf(KindInvalid, "无法读取插件清单：%v", err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, errorf(KindInvalid, "无法读取插件清单：%v", err)
	}
	c := &m.Coursive
	switch {
	case !idRE.MatchString(c.ID):
		return nil, errorf(KindInvalid, "插件 id 无效：%q", c.ID)
	case c.Title == "":
		return nil, errorf(KindInvalid, "插件缺少 title")
	case len(c.Contributes.Sources) == 0:
		return nil, errorf(KindInvalid, "插件「%s」没有提供来源", c.Title)
	}
	if _, err := packageIcon(dir, c.Icon); err != nil {
		return nil, err
	}
	granted := false
	for _, p := range c.Permissions {
		granted = granted || p == "source"
	}
	if !granted {
		return nil, errorf(KindInvalid, "插件「%s」没有声明 source 权限", c.Title)
	}
	seen := map[string]bool{}
	for _, s := range c.Contributes.Sources {
		switch {
		case !idRE.MatchString(s.ID) || seen[s.ID]:
			return nil, errorf(KindInvalid, "来源 id 无效或重复：%q", s.ID)
		case s.Protocol != ProtocolVersion:
			return nil, errorf(KindInvalid, "来源「%s」使用协议 %q，本机支持 %s", s.Title, s.Protocol, ProtocolVersion)
		}
		seen[s.ID] = true
		for platform, rel := range s.Executable {
			clean := filepath.Clean(filepath.FromSlash(rel))
			if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return nil, errorf(KindInvalid, "来源「%s」的 %s 程序路径越出插件目录", s.Title, platform)
			}
		}
	}
	return &m, nil
}

type stateFile struct {
	Plugins map[string]struct {
		Enabled bool `json:"enabled"`
	} `json:"plugins"`
}

// Registry is the set of installed source plugins and their providers.
type Registry struct {
	// Dir holds one directory per installed plugin.
	Dir string
	// DataDir holds each source's working directory and vault.
	DataDir string
	// Publish announces "sources" changes on the event bus.
	Publish func(kind string, data any)

	mu        sync.Mutex
	manifests map[string]*Manifest
	icons     map[string]string
	enabled   map[string]bool
	providers map[string]*Provider
}

func (r *Registry) statePath() string { return filepath.Join(filepath.Dir(r.Dir), "sources.json") }

// Load reads the installed plugins. A broken plugin is skipped, not fatal.
func (r *Registry) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manifests, r.icons, r.enabled = map[string]*Manifest{}, map[string]string{}, map[string]bool{}
	if r.providers == nil {
		r.providers = map[string]*Provider{}
	}
	var st stateFile
	if b, err := os.ReadFile(r.statePath()); err == nil {
		json.Unmarshal(b, &st)
	}
	entries, err := os.ReadDir(r.Dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := ReadManifest(filepath.Join(r.Dir, e.Name()))
		if err != nil || m.Coursive.ID != e.Name() {
			continue
		}
		r.manifests[m.Coursive.ID] = m
		r.icons[m.Coursive.ID], _ = packageIcon(filepath.Join(r.Dir, e.Name()), m.Coursive.Icon)
		s, ok := st.Plugins[m.Coursive.ID]
		r.enabled[m.Coursive.ID] = !ok || s.Enabled
	}
	r.syncProvidersLocked()
	return nil
}

func (r *Registry) saveStateLocked() error {
	st := stateFile{Plugins: map[string]struct {
		Enabled bool `json:"enabled"`
	}{}}
	for id, on := range r.enabled {
		st.Plugins[id] = struct {
			Enabled bool `json:"enabled"`
		}{on}
	}
	b, _ := json.MarshalIndent(st, "", "  ")
	os.MkdirAll(filepath.Dir(r.statePath()), 0o700)
	return os.WriteFile(r.statePath(), b, 0o600)
}

// syncProvidersLocked creates providers for enabled sources available on this
// platform and stops the rest.
func (r *Registry) syncProvidersLocked() {
	want := map[string]bool{}
	for id, m := range r.manifests {
		if !r.enabled[id] {
			continue
		}
		for _, s := range m.Coursive.Contributes.Sources {
			rel, ok := s.Executable[Platform()]
			if !ok {
				continue
			}
			key := id + "/" + s.ID
			want[key] = true
			if _, ok := r.providers[key]; ok {
				continue
			}
			data := filepath.Join(r.DataDir, id, s.ID)
			p := &Provider{ID: key, Plugin: id, Title: s.Title, Exe: filepath.Join(r.Dir, id, filepath.FromSlash(rel)),
				DataDir: filepath.Join(data, "work"), Vault: &vault.Namespace{Path: filepath.Join(data, "vault.json")}}
			p.Changed = r.changed
			r.providers[key] = p
		}
	}
	for key, p := range r.providers {
		if !want[key] {
			p.Stop()
			delete(r.providers, key)
		}
	}
}

func (r *Registry) changed() {
	if r.Publish != nil {
		r.Publish("sources", map[string]any{})
	}
}

// PluginInfo describes an installed source plugin.
type PluginInfo struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	// Icon is a Material Symbols name or a data: URL of the package's image.
	Icon      string     `json:"icon"`
	Version   string     `json:"version"`
	Enabled   bool       `json:"enabled"`
	Hosts     []string   `json:"hosts"`
	Available bool       `json:"available"`
	Sources   []Snapshot `json:"sources"`
}

func (r *Registry) infoLocked(id string) PluginInfo {
	m := r.manifests[id]
	in := PluginInfo{ID: id, Title: m.Coursive.Title, Summary: m.Coursive.Summary, Icon: r.icons[id], Version: m.Version,
		Enabled: r.enabled[id], Hosts: []string{}, Sources: []Snapshot{}}
	for _, s := range m.Coursive.Contributes.Sources {
		in.Hosts = append(in.Hosts, s.Hosts...)
		if _, ok := s.Executable[Platform()]; ok {
			in.Available = true
		}
		if p, ok := r.providers[id+"/"+s.ID]; ok {
			in.Sources = append(in.Sources, p.Snapshot())
		}
	}
	return in
}

// Plugins lists the installed source plugins, sorted by id.
func (r *Registry) Plugins() []PluginInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]PluginInfo, 0, len(r.manifests))
	for id := range r.manifests {
		out = append(out, r.infoLocked(id))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Preview validates a plugin folder without installing it, for the consent dialog.
func (r *Registry) Preview(from string) (PluginInfo, error) {
	m, err := ReadManifest(from)
	if err != nil {
		return PluginInfo{}, err
	}
	icon, _ := packageIcon(from, m.Coursive.Icon)
	in := PluginInfo{ID: m.Coursive.ID, Title: m.Coursive.Title, Summary: m.Coursive.Summary, Icon: icon, Version: m.Version,
		Hosts: []string{}, Sources: []Snapshot{}}
	for _, s := range m.Coursive.Contributes.Sources {
		in.Hosts = append(in.Hosts, s.Hosts...)
		if rel, ok := s.Executable[Platform()]; ok {
			if _, err := os.Stat(filepath.Join(from, filepath.FromSlash(rel))); err == nil {
				in.Available = true
			}
		}
		in.Sources = append(in.Sources, Snapshot{ID: m.Coursive.ID + "/" + s.ID, Plugin: m.Coursive.ID, Title: s.Title,
			Process: Stopped, Auth: Auth{State: SignedOut}, Capabilities: []string{}, Fields: []Field{}})
	}
	return in, nil
}

// Install copies a plugin folder into place, replacing an installed copy.
func (r *Registry) Install(from string) (PluginInfo, error) {
	m, err := ReadManifest(from)
	if err != nil {
		return PluginInfo{}, err
	}
	id := m.Coursive.ID
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return PluginInfo{}, err
	}
	staging, err := os.MkdirTemp(r.Dir, ".install-")
	if err != nil {
		return PluginInfo{}, err
	}
	defer os.RemoveAll(staging)
	if err := copyTree(from, staging); err != nil {
		return PluginInfo{}, errorf(KindInvalid, "无法复制插件：%v", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, p := range r.providers {
		if strings.HasPrefix(key, id+"/") {
			p.Stop()
			delete(r.providers, key)
		}
	}
	dest := filepath.Join(r.Dir, id)
	os.RemoveAll(dest)
	if err := os.Rename(staging, dest); err != nil {
		return PluginInfo{}, err
	}
	r.manifests[id] = m
	r.icons[id], _ = packageIcon(dest, m.Coursive.Icon)
	if _, ok := r.enabled[id]; !ok {
		r.enabled[id] = true
	}
	r.saveStateLocked()
	r.syncProvidersLocked()
	go r.changed()
	return r.infoLocked(id), nil
}

// copyTree copies regular files and directories, keeping execute bits and
// refusing links, so an installed plugin cannot point outside its folder.
func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			return fmt.Errorf("%s 是链接，插件不能包含链接", rel)
		case d.IsDir():
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o700)
		case !d.Type().IsRegular():
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm()&0o755|0o600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			return err
		}
		return dst.Close()
	})
}

// Uninstall removes a plugin, its working data and its vault. Library
// entries it produced stay.
func (r *Registry) Uninstall(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.manifests[id]; !ok {
		return errorf(KindInvalid, "没有安装插件 %s", id)
	}
	delete(r.manifests, id)
	delete(r.icons, id)
	delete(r.enabled, id)
	r.syncProvidersLocked()
	r.saveStateLocked()
	os.RemoveAll(filepath.Join(r.Dir, id))
	os.RemoveAll(filepath.Join(r.DataDir, id))
	go r.changed()
	return nil
}

// SetEnabled turns a plugin's sources on or off.
func (r *Registry) SetEnabled(id string, on bool) (PluginInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.manifests[id]; !ok {
		return PluginInfo{}, errorf(KindInvalid, "没有安装插件 %s", id)
	}
	r.enabled[id] = on
	r.syncProvidersLocked()
	r.saveStateLocked()
	go r.changed()
	return r.infoLocked(id), nil
}

// Provider returns one source's provider.
func (r *Registry) Provider(id string) (*Provider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.providers[id]; ok {
		return p, nil
	}
	return nil, errorf(KindInvalid, "来源 %s 未安装或未启用", id)
}

// Providers returns every enabled provider, sorted by id.
func (r *Registry) Providers() []*Provider {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Provider, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// SignedIn returns the providers whose last known state is signed in.
func (r *Registry) SignedIn() []*Provider {
	var out []*Provider
	for _, p := range r.Providers() {
		if p.SignedIn() {
			out = append(out, p)
		}
	}
	return out
}

// ForCourse returns the provider that owns a course's source. A course from
// before sources were recorded ("") belongs to the only signed-in source.
func (r *Registry) ForCourse(source string) (*Provider, error) {
	if source != "" {
		p, err := r.Provider(source)
		if err != nil {
			return nil, errorf(KindAuthRequired, "这门课程的来源插件未安装或未启用，无法刷新")
		}
		return p, nil
	}
	signed := r.SignedIn()
	if len(signed) == 1 {
		return signed[0], nil
	}
	if len(signed) == 0 {
		return nil, errorf(KindAuthRequired, "无法刷新这门课程：请先在「来源」中登录")
	}
	return nil, errorf(KindInvalid, "无法确定这门课程属于哪个来源")
}

// Stop ends every provider process.
func (r *Registry) Stop() {
	for _, p := range r.Providers() {
		p.Stop()
	}
}

// SignIn submits a sign-in form; values whose field is not remembered are
// redacted from diagnostics.
func (r *Registry) SignIn(ctx context.Context, id string, fields map[string]string) (Auth, error) {
	p, err := r.Provider(id)
	if err != nil {
		return Auth{}, err
	}
	for _, v := range fields {
		p.Secret(v)
	}
	var a Auth
	if err := p.Call(ctx, "auth.signIn", map[string]any{"fields": fields}, &a); err != nil {
		return Auth{}, err
	}
	p.setAuth(a)
	return a, nil
}

// Answer submits a challenge answer.
func (r *Registry) Answer(ctx context.Context, id, challenge, answer string) (Auth, error) {
	p, err := r.Provider(id)
	if err != nil {
		return Auth{}, err
	}
	var a Auth
	if err := p.Call(ctx, "auth.answer", map[string]any{"challenge": challenge, "answer": answer}, &a); err != nil {
		return Auth{}, err
	}
	p.setAuth(a)
	return a, nil
}

// SignOut signs a source out.
func (r *Registry) SignOut(ctx context.Context, id string) (Auth, error) {
	p, err := r.Provider(id)
	if err != nil {
		return Auth{}, err
	}
	if err := p.Call(ctx, "auth.signOut", struct{}{}, nil); err != nil {
		return Auth{}, err
	}
	a := Auth{State: SignedOut}
	p.setAuth(a)
	return a, nil
}

// Refresh asks every enabled provider for its sign-in state (starting it),
// so remembered sessions are known after the daemon starts.
func (r *Registry) Refresh(ctx context.Context) { r.RefreshPlugin(ctx, "") }

// RefreshPlugin is Refresh for one plugin's providers; "" means all.
func (r *Registry) RefreshPlugin(ctx context.Context, plugin string) {
	for _, p := range r.Providers() {
		if plugin != "" && p.Plugin != plugin {
			continue
		}
		var a Auth
		if err := p.Call(ctx, "auth.status", struct{}{}, &a); err == nil {
			p.setAuth(a)
		}
	}
}
