// Package pluginfabric owns the Plugin Fabric service catalog: how a capability
// is described, which providers offer it, and which one a contribution gets.
//
// Contract: docs/specs/0031-plugin-fabric-core.md (capabilities and service
// dependencies). The package is pure data and decisions: it starts no process,
// reads no file and holds no credential. Providers report their own
// availability; the host records permissions and asks for consent.
package pluginfabric

import (
	"fmt"
	"regexp"
	"strings"
)

// Scope is where a contribution may use a service.
type Scope string

// Service scopes from spec 0031.
const (
	ScopeGlobal       Scope = "global"
	ScopeCourse       Scope = "course"
	ScopeLecture      Scope = "lecture"
	ScopeCanvas       Scope = "canvas"
	ScopeConversation Scope = "conversation"
	ScopeSelection    Scope = "selection"
)

// Boundary says where a provider moves the data it handles. A change of
// boundary needs the user's consent again (spec 0031).
type Boundary string

const (
	// BoundaryLocal keeps the user's data on this device.
	BoundaryLocal Boundary = "local"
	// BoundaryCloud sends it to a remote service.
	BoundaryCloud Boundary = "cloud"
)

// Availability states a provider reports about itself.
const (
	StateReady        = "ready"
	StateNeedsConsent = "needs_consent"
	StateNeedsAuth    = "needs_auth"
	StateDisabled     = "disabled"
	StateFailed       = "failed"
)

// Cost tiers, from cheapest to dearest.
const (
	CostFree      = "free"
	CostCheap     = "cheap"
	CostMetered   = "metered"
	CostExpensive = "expensive"
)

// Availability is a provider's readiness and, when it is not ready, the reason
// in words the user can act on.
type Availability struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

// Ready reports whether the provider may be selected.
func (a Availability) Ready() bool { return a.State == StateReady }

// Cost tells the app and the agent what a call costs before it runs.
type Cost struct {
	Tier   string `json:"tier"`
	Detail string `json:"detail,omitempty"`
}

// Field is one member of a payload shape.
type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	Note     string `json:"note,omitempty"`
}

// Schema names a versioned payload shape. The JSON Schema itself belongs to the
// manifest layer; the catalog keeps the reference and its fields so the agent
// shows a shape only when asked for it.
type Schema struct {
	Ref    string  `json:"ref"`
	Fields []Field `json:"fields,omitempty"`
}

// Service is one provider's published description of a capability.
type Service struct {
	// ID is "domain.verb", such as "transcript.read".
	ID string `json:"id"`
	// Version is the offered version: "1", "1.2" or "1.2.3".
	Version string `json:"version"`
	// Owner is the plugin id, or BuiltinOwner for the daemon's own services.
	Owner  string `json:"owner"`
	Title  string `json:"title"`
	Scope  Scope  `json:"scope"`
	Input  Schema `json:"input"`
	Output Schema `json:"output"`
	// Permissions are the permissions a call needs, such as "library.read".
	Permissions []string `json:"permissions,omitempty"`
	// Device lists device capabilities the provider needs, such as "asr.local".
	Device       []string     `json:"device,omitempty"`
	Cost         Cost         `json:"cost"`
	Availability Availability `json:"availability"`
	Boundary     Boundary     `json:"boundary"`
	// Priority breaks ties between otherwise equal providers; higher wins.
	Priority int `json:"priority,omitempty"`
}

// Key is the service key used in diagnostics: "transcript.read@1".
func (s Service) Key() string { return s.ID + "@" + s.Version }

var (
	serviceIDRE = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$`)
	ownerRE     = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	fieldNameRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
)

// Validate reports the first declaration the catalog cannot accept.
func (s Service) Validate() error {
	switch {
	case !serviceIDRE.MatchString(s.ID):
		return fmt.Errorf("服务 id %q 应为 domain.verb，例如 transcript.read", s.ID)
	case !ownerRE.MatchString(s.Owner):
		return fmt.Errorf("服务 %s 的 owner %q 无效", s.ID, s.Owner)
	case !scopeValid(s.Scope):
		return fmt.Errorf("服务 %s 的 scope %q 无效", s.ID, s.Scope)
	case s.Boundary != BoundaryLocal && s.Boundary != BoundaryCloud:
		return fmt.Errorf("服务 %s 的 boundary %q 应为 local 或 cloud", s.ID, s.Boundary)
	case !availabilityValid(s.Availability.State):
		return fmt.Errorf("服务 %s 的 availability %q 无效", s.ID, s.Availability.State)
	}
	if _, err := ParseVersion(s.Version); err != nil {
		return fmt.Errorf("服务 %s：%w", s.ID, err)
	}
	if s.Cost.Tier != "" && !costValid(s.Cost.Tier) {
		return fmt.Errorf("服务 %s 的 cost %q 无效", s.ID, s.Cost.Tier)
	}
	for _, list := range [][]string{s.Permissions, s.Device} {
		seen := map[string]bool{}
		for _, name := range list {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("服务 %s 有空白的权限或设备能力", s.ID)
			}
			if seen[name] {
				return fmt.Errorf("服务 %s 重复声明 %q", s.ID, name)
			}
			seen[name] = true
		}
	}
	for _, schema := range []Schema{s.Input, s.Output} {
		for _, f := range schema.Fields {
			if !fieldNameRE.MatchString(f.Name) || f.Type == "" {
				return fmt.Errorf("服务 %s 的字段 %q 无效", s.ID, f.Name)
			}
		}
	}
	return nil
}

// normalize fills the two fields that have an obvious default.
func (s *Service) normalize() {
	if s.Title == "" {
		s.Title = s.Key()
	}
	if s.Cost.Tier == "" {
		s.Cost.Tier = CostFree
	}
}

func scopeValid(s Scope) bool {
	switch s {
	case ScopeGlobal, ScopeCourse, ScopeLecture, ScopeCanvas, ScopeConversation, ScopeSelection:
		return true
	}
	return false
}

func costValid(t string) bool {
	switch t {
	case CostFree, CostCheap, CostMetered, CostExpensive:
		return true
	}
	return false
}

func availabilityValid(state string) bool {
	switch state {
	case StateReady, StateNeedsConsent, StateNeedsAuth, StateDisabled, StateFailed:
		return true
	}
	return false
}
