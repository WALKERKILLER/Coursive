// Package pluginfabric parses and validates the canonical Coursive plugin
// manifest.
//
// The accepted shape is docs/schemas/plugin-manifest.v3.schema.json and the
// contract it implements is docs/specs/0031-plugin-fabric-core.md: one
// manifest declares the plugin identity, the API range it needs, its runtime
// components, the services it requires and provides, its permissions and its
// contributions. Parse rejects a manifest before anything is installed or
// activated, and reports every defect it finds rather than the first one.
//
// Version-range evaluation against the running host belongs to the host; this
// package checks declared syntax and cross-field rules only.
package pluginfabric

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// SchemaVersion identifies the manifest shape this package accepts.
const SchemaVersion = "coursive/plugin-manifest@3"

// RuntimeKinds are the executors a manifest may name.
var RuntimeKinds = []string{"pi", "moejs", "process", "web-ui", "go"}

// Permissions are the permission names a manifest may declare.
var Permissions = []string{"library.read", "library.write", "views", "localhost", "exec", "workspace", "network", "folder", "source"}

// ContributionGroups are the keys contributes may hold.
var ContributionGroups = []string{"navigation", "pages", "slots", "tools", "views", "actions", "settings", "sources"}

// Categories are the marketplace categories a manifest may name.
var Categories = []string{"notes", "visualize", "review", "programming", "connector", "tutor", "assignment"}

var (
	idRE              = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)
	manifestVersionRE = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)
	apiRE             = regexp.MustCompile(`^(?:\^|>=)?\d+\.\d+\.\d+$`)
	serviceRE         = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-z0-9]*)+$`)
	serviceRangeRE    = regexp.MustCompile(`^(?:\^|>=)?\d+(?:\.\d+){0,2}$`)
	contributionRE    = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

	runtimeKindSet = setOf(RuntimeKinds)
	permissionSet  = setOf(Permissions)
	categorySet    = setOf(Categories)

	// pluginFields are the keys coursive may hold; anything else means the
	// manifest was written for another schema version.
	pluginFields = []string{"id", "title", "summary", "description", "icon", "categories", "api", "runtime",
		"requires", "provides", "permissions", "exec", "hosts", "settings", "examples", "contributes"}

	// v1Homes maps a v1 manifest field to its v3 home, so a v1 package gets a
	// migration instruction instead of "unknown field".
	v1Homes = map[string]string{
		"engine": "api",
		"agent":  `runtime[{kind: "pi", entry}]`,
		"skills": `runtime[{kind: "pi", skills}]`,
		"views":  "contributes.views",
	}
)

// Problem is one actionable defect in a manifest.
type Problem struct {
	// Code is a stable machine-readable rule name, for example "api-missing".
	Code string
	// Path is where the defect is, for example "coursive.runtime[1].entry".
	Path string
	// Message says what to change, for a plugin author.
	Message string
}

// Error reports every problem found in one manifest or one canvas object.
type Error struct {
	// Subject names what failed and the version it was checked against, such as
	// "画布对象不符合 coursive/canvas-object@1". Empty reads as a manifest
	// error, so the original callers keep their message.
	Subject  string
	Problems []Problem
}

func (e *Error) Error() string {
	subject := e.Subject
	if subject == "" {
		subject = "插件清单不符合 " + SchemaVersion
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s：%d 处问题", subject, len(e.Problems))
	for _, p := range e.Problems {
		b.WriteString("\n")
		if p.Path != "" {
			b.WriteString(p.Path + ": ")
		}
		b.WriteString(p.Message)
	}
	return b.String()
}

// Code returns the first problem's code, or "" when err is not a manifest
// error.
func Code(err error) string {
	var e *Error
	if errors.As(err, &e) && len(e.Problems) > 0 {
		return e.Problems[0].Code
	}
	return ""
}

// Manifest is a plugin package's manifest: the package fields this repository
// reads, and the coursive block that declares the plugin.
type Manifest struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Coursive Coursive `json:"coursive"`

	unknownFields []string
}

// Coursive is the plugin declaration under a package.json "coursive" key.
type Coursive struct {
	ID          string                `json:"id"`
	Title       string                `json:"title"`
	Summary     string                `json:"summary"`
	Description string                `json:"description"`
	Icon        string                `json:"icon"`
	Categories  []string              `json:"categories"`
	API         string                `json:"api"`
	Runtime     []Runtime             `json:"runtime"`
	Requires    []ManifestRequirement `json:"requires"`
	Provides    []Provision           `json:"provides"`
	Permissions []string              `json:"permissions"`
	// Exec and Hosts carry the detail the exec and network permissions allow.
	Exec        []string    `json:"exec"`
	Hosts       []string    `json:"hosts"`
	Examples    []string    `json:"examples"`
	Contributes Contributes `json:"contributes"`
}

// Runtime is one runtime component: the engine that executes part of the
// plugin and the entry it loads.
type Runtime struct {
	Kind     string   `json:"kind"`
	Entry    Entry    `json:"entry"`
	Skills   []string `json:"skills"`
	Optional bool     `json:"optional"`
}

// Entry is a package-relative path, or one path per platform for a process
// runtime. A leading ./ is removed, as v1 manifests write ./agent.mjs.
type Entry struct {
	Path      string
	Platforms map[string]string

	invalid bool
}

// UnmarshalJSON accepts a string or an object of platform keys to paths.
func (e *Entry) UnmarshalJSON(b []byte) error {
	var path string
	if json.Unmarshal(b, &path) == nil {
		e.Path = trimDotSlash(path)
		return nil
	}
	var platforms map[string]string
	if json.Unmarshal(b, &platforms) == nil {
		e.Platforms = make(map[string]string, len(platforms))
		for platform, p := range platforms {
			e.Platforms[platform] = trimDotSlash(p)
		}
		return nil
	}
	e.invalid = true
	return nil
}

// Paths returns every entry path, sorted.
func (e Entry) Paths() []string {
	if len(e.Platforms) == 0 {
		if e.Path == "" {
			return nil
		}
		return []string{e.Path}
	}
	out := make([]string, 0, len(e.Platforms))
	for _, p := range e.Platforms {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// ManifestRequirement is a service the plugin needs before its contributions activate.
type ManifestRequirement struct {
	Service  string `json:"service"`
	Version  string `json:"version"`
	Optional bool   `json:"optional"`
}

// Provision is a service the plugin offers to other plugins and to the agent.
type Provision struct {
	Service string `json:"service"`
	Version string `json:"version"`
}

// ManifestContribution is one entry of a contribution group. Only the identity is
// decoded: the group-specific fields belong to docs/specs/0032-ui-extension-protocol.md.
type ManifestContribution struct {
	ID string `json:"id"`
}

// Contributes holds the contribution groups a manifest declares.
type Contributes struct {
	Navigation []ManifestContribution
	Pages      []ManifestContribution
	Slots      []ManifestContribution
	Tools      []string
	Views      []ManifestContribution
	Actions    []ManifestContribution
	Settings   []ManifestContribution
	Sources    []Source

	unknown []string
}

// Source declares a content source: the sign-in, course and media surface a
// source-provider executable offers (docs/specs/0024-source-provider-plugins.md).
type Source struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Protocol   string   `json:"protocol"`
	Executable Entry    `json:"executable"`
	Hosts      []string `json:"hosts"`
}

// UnmarshalJSON reads every known group and records the unknown ones.
func (c *Contributes) UnmarshalJSON(b []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	*c = Contributes{}
	for name, raw := range fields {
		var err error
		switch name {
		case "navigation":
			err = json.Unmarshal(raw, &c.Navigation)
		case "pages":
			err = json.Unmarshal(raw, &c.Pages)
		case "slots":
			err = json.Unmarshal(raw, &c.Slots)
		case "tools":
			err = json.Unmarshal(raw, &c.Tools)
		case "views":
			err = json.Unmarshal(raw, &c.Views)
		case "actions":
			err = json.Unmarshal(raw, &c.Actions)
		case "settings":
			err = json.Unmarshal(raw, &c.Settings)
		case "sources":
			err = json.Unmarshal(raw, &c.Sources)
		default:
			c.unknown = append(c.unknown, name)
			continue
		}
		if err != nil {
			return fmt.Errorf("contributes.%s: %w", name, err)
		}
	}
	sort.Strings(c.unknown)
	return nil
}

// UnknownGroups are the contributes keys this schema does not define.
func (c *Contributes) UnknownGroups() []string { return append([]string(nil), c.unknown...) }

// Parse decodes a manifest and validates it.
func Parse(data []byte) (*Manifest, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fail(Problem{Code: "manifest-invalid", Message: "无法解析插件清单：" + err.Error()})
	}
	if raw, ok := top["coursive"]; !ok || string(raw) == "null" {
		return nil, fail(Problem{Code: "manifest-missing", Path: "coursive", Message: "插件包缺少 coursive 清单"})
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fail(Problem{Code: "manifest-invalid", Path: "coursive", Message: "清单字段的类型不对：" + err.Error()})
	}
	m.unknownFields = unknownKeys(top["coursive"], pluginFields)
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate reports every problem in m; it returns nil when the manifest is
// acceptable.
func (m *Manifest) Validate() error {
	p := &problems{}
	if m.Version == "" {
		p.add("version-missing", "version", "缺少 version")
	} else if !manifestVersionRE.MatchString(m.Version) {
		p.add("version-invalid", "version", "version 应为 semver，例如 1.0.0")
	}

	c := &m.Coursive
	switch {
	case c.ID == "":
		p.add("id-missing", "coursive.id", "缺少 id")
	case !idRE.MatchString(c.ID):
		p.add("id-invalid", "coursive.id", "id 应为 2–32 位小写字母、数字或 -，以字母开头")
	}
	if n := utf8.RuneCountInString(c.Title); n == 0 || n > 24 {
		p.add("title-invalid", "coursive.title", "title 应为 1–24 字")
	}
	if n := utf8.RuneCountInString(c.Summary); n == 0 || n > 80 {
		p.add("summary-invalid", "coursive.summary", "summary 应为 1–80 字")
	}
	switch {
	case c.API == "":
		p.add("api-missing", "coursive.api", "缺少 api：插件需要的接口范围，例如 ^3.0.0")
	case !apiRE.MatchString(c.API):
		p.add("api-invalid", "coursive.api", "api 应为 ^x.y.z、>=x.y.z 或精确版本")
	}

	for i, category := range c.Categories {
		if !categorySet[category] {
			p.add("category-unknown", fmt.Sprintf("coursive.categories[%d]", i), "未知分类 %q", category)
		}
	}

	for i, r := range c.Runtime {
		at := fmt.Sprintf("coursive.runtime[%d]", i)
		if !runtimeKindSet[r.Kind] {
			p.add("runtime-kind", at+".kind", "未知的运行时 %q：应为 %s", r.Kind, strings.Join(RuntimeKinds, "、"))
		}
		if r.Entry.invalid || (r.Entry.Path == "" && len(r.Entry.Platforms) == 0) {
			p.add("runtime-entry", at+".entry", "entry 应为插件目录内的相对路径，或按平台给出的路径表")
		}
		for _, rel := range r.Entry.Paths() {
			if !insidePackage(rel) {
				p.add("runtime-path", at+".entry", "路径 %q 越出插件目录：应为插件目录内的相对路径", rel)
			}
		}
		if len(r.Skills) > 0 && r.Kind != "pi" {
			p.add("runtime-skills", at+".skills", "只有 pi 运行时可以声明 skills")
		}
		for _, rel := range r.Skills {
			if !insidePackage(trimDotSlash(rel)) {
				p.add("runtime-path", at+".skills", "路径 %q 越出插件目录：应为插件目录内的相对路径", rel)
			}
		}
	}

	for i, r := range c.Requires {
		at := fmt.Sprintf("coursive.requires[%d]", i)
		if !serviceRE.MatchString(r.Service) {
			p.add("requires-invalid", at+".service", "服务 id 应为 domain.verb，例如 transcript.read")
		}
		if !serviceRangeRE.MatchString(r.Version) {
			p.add("requires-invalid", at+".version", "服务版本应为 ^1、^1.2.3、>=1.0.0 或精确版本")
		}
	}
	for i, r := range c.Provides {
		at := fmt.Sprintf("coursive.provides[%d]", i)
		if !serviceRE.MatchString(r.Service) {
			p.add("provides-invalid", at+".service", "服务 id 应为 domain.verb，例如 transcript.read")
		}
		if !manifestVersionRE.MatchString(r.Version) {
			p.add("provides-invalid", at+".version", "提供的服务版本应为精确的 semver，例如 1.0.0")
		}
	}

	declared := map[string]bool{}
	for i, permission := range c.Permissions {
		at := fmt.Sprintf("coursive.permissions[%d]", i)
		if !permissionSet[permission] {
			p.add("permission-unknown", at, "未知权限 %q", permission)
		}
		if declared[permission] {
			p.add("permission-duplicate", at, "权限 %q 重复声明", permission)
		}
		declared[permission] = true
	}
	if declared["exec"] && len(c.Exec) == 0 {
		p.add("exec-undeclared", "coursive.exec", "exec 权限需要在 exec 中列出允许的命令")
	}
	if declared["network"] && len(c.Hosts) == 0 {
		p.add("hosts-undeclared", "coursive.hosts", "network 权限需要在 hosts 中列出允许的域名")
	}

	for _, name := range c.Contributes.UnknownGroups() {
		p.add("contribution-unknown", "coursive.contributes."+name, "未知的贡献组 %q：应为 %s", name, strings.Join(ContributionGroups, "、"))
	}
	for _, group := range c.Contributes.entryGroups() {
		seen := map[string]bool{}
		for i, e := range group.entries {
			at := fmt.Sprintf("coursive.contributes.%s[%d].id", group.name, i)
			if !contributionRE.MatchString(e.ID) {
				p.add("contribution-invalid", at, "贡献 id %q 无效：应以小写字母开头，只含小写字母、数字和 . _ -", e.ID)
				continue
			}
			if seen[e.ID] {
				p.add("contribution-duplicate", at, "贡献 id %q 在同一组内重复", e.ID)
			}
			seen[e.ID] = true
		}
	}
	seenSources := map[string]bool{}
	for i, source := range c.Contributes.Sources {
		at := fmt.Sprintf("coursive.contributes.sources[%d]", i)
		if !contributionRE.MatchString(source.ID) {
			p.add("contribution-invalid", at+".id", "来源 id %q 无效", source.ID)
		} else if seenSources[source.ID] {
			p.add("contribution-duplicate", at+".id", "来源 id %q 重复声明", source.ID)
		}
		seenSources[source.ID] = true
		if strings.TrimSpace(source.Title) == "" {
			p.add("source-invalid", at+".title", "来源需要 title")
		}
		if strings.TrimSpace(source.Protocol) == "" {
			p.add("source-invalid", at+".protocol", "来源需要它实现的协议版本")
		}
		if source.Executable.invalid || source.Executable.Path != "" || len(source.Executable.Platforms) == 0 {
			p.add("source-invalid", at+".executable", "executable 应为按平台给出的程序路径表")
		}
		for _, rel := range source.Executable.Paths() {
			if !insidePackage(rel) {
				p.add("source-path", at+".executable", "路径 %q 越出插件目录：应为插件目录内的相对路径", rel)
			}
		}
	}
	seenTools := map[string]bool{}
	for i, tool := range c.Contributes.Tools {
		at := fmt.Sprintf("coursive.contributes.tools[%d]", i)
		if !serviceRE.MatchString(tool) {
			p.add("contribution-invalid", at, "工具应由它提供的服务标识，例如 review.generate")
			continue
		}
		if seenTools[tool] {
			p.add("contribution-duplicate", at, "工具 %q 重复声明", tool)
		}
		seenTools[tool] = true
	}

	for _, field := range m.unknownFields {
		if home, ok := v1Homes[field]; ok {
			p.add("field-unknown", "coursive."+field, "v1 字段 %q 在 v3 中改为 %s", field, home)
			continue
		}
		p.add("field-unknown", "coursive."+field, "v3 清单没有 %q 字段", field)
	}

	if len(p.list) > 0 {
		return &Error{Problems: p.list}
	}
	return nil
}

type problems struct {
	list []Problem
}

func (p *problems) add(code, path, format string, args ...any) {
	p.list = append(p.list, Problem{Code: code, Path: path, Message: fmt.Sprintf(format, args...)})
}

func fail(p Problem) error { return &Error{Problems: []Problem{p}} }

type entryGroup struct {
	name    string
	entries []ManifestContribution
}

func (c *Contributes) entryGroups() []entryGroup {
	return []entryGroup{
		{"navigation", c.Navigation},
		{"pages", c.Pages},
		{"slots", c.Slots},
		{"views", c.Views},
		{"actions", c.Actions},
		{"settings", c.Settings},
	}
}

// insidePackage reports whether p is a path inside the plugin package.
func insidePackage(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) {
		return false
	}
	if len(p) >= 2 && p[1] == ':' {
		return false
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// trimDotSlash removes a leading ./ that v1 manifests write.
func trimDotSlash(p string) string {
	for strings.HasPrefix(p, "./") {
		p = p[2:]
	}
	return p
}

func unknownKeys(raw json.RawMessage, known []string) []string {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil
	}
	allowed := setOf(known)
	var out []string
	for field := range fields {
		if !allowed[field] {
			out = append(out, field)
		}
	}
	sort.Strings(out)
	return out
}

func setOf(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}
