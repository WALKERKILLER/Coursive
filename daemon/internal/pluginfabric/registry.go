package pluginfabric

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Provider is one implementation of a service, plus the services it needs
// itself.
type Provider struct {
	// ID is the provider's stable key, unique in the registry, such as
	// "core/local-asr" or "<plugin>/<service>".
	ID      string
	Service Service
	// Requires are the services this provider needs to serve calls.
	Requires []Requirement
}

var providerIDRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}(/[a-z][a-z0-9.-]{0,31})?$`)

// Requirement is one entry of a contribution's `requires`.
type Requirement struct {
	// Service is "domain.verb", such as "transcript.read".
	Service string `json:"service"`
	// Version is the accepted versions: "1", "^1" or ">=1.2.0".
	Version string `json:"version"`
	// Optional keeps the contribution usable when nothing provides it.
	Optional bool `json:"optional,omitempty"`
	// Fallback documents the degraded mode used when an optional dependency is
	// missing, such as "只提供规则式思维导图".
	Fallback string `json:"fallback,omitempty"`
}

// Environment is what the host knows besides the catalog. The zero value grants
// nothing, prefers nothing and has consented to nothing.
type Environment struct {
	Permissions map[string]bool
	Devices     map[string]bool
	// Preference maps a service id to the provider the user prefers: a
	// provider id or a provider owner.
	Preference map[string]string
	// Current maps a service id to the provider id in use, so a change of data
	// boundary can be reported as such.
	Current map[string]string
	// Consented maps a service id to the data boundary the user accepted.
	Consented map[string]Boundary
}

// Registry is the service catalog and the environment it resolves against.
type Registry struct {
	mu        sync.RWMutex
	providers map[string][]Provider // service id -> providers, sorted by provider id
	ids       map[string]bool
	env       Environment
}

// New returns an empty catalog with nothing granted.
func New() *Registry {
	return &Registry{
		providers: map[string][]Provider{},
		ids:       map[string]bool{},
		env: Environment{
			Permissions: map[string]bool{},
			Devices:     map[string]bool{},
			Preference:  map[string]string{},
			Current:     map[string]string{},
			Consented:   map[string]Boundary{},
		},
	}
}

// Grant records permissions the user has granted.
func (r *Registry) Grant(permissions ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range permissions {
		if p = strings.TrimSpace(p); p != "" {
			r.env.Permissions[p] = true
		}
	}
}

// SetDevice records whether this device has a capability such as "asr.local".
func (r *Registry) SetDevice(name string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ok {
		r.env.Devices[name] = true
		return
	}
	delete(r.env.Devices, name)
}

// Prefer sets the user's provider preference for a service; "" clears it.
func (r *Registry) Prefer(service, provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if provider == "" {
		delete(r.env.Preference, service)
		return
	}
	r.env.Preference[service] = provider
}

// Use records the provider now serving a service, so a later selection can
// report a change of data boundary.
func (r *Registry) Use(service, provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if provider == "" {
		delete(r.env.Current, service)
		return
	}
	r.env.Current[service] = provider
}

// Consent records the data boundary the user accepted for a service.
func (r *Registry) Consent(service string, b Boundary) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.env.Consented[service] = b
}

// Environment returns a copy of what the registry resolves against.
func (r *Registry) Environment() Environment {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := Environment{
		Permissions: map[string]bool{},
		Devices:     map[string]bool{},
		Preference:  map[string]string{},
		Current:     map[string]string{},
		Consented:   map[string]Boundary{},
	}
	for k, v := range r.env.Permissions {
		out.Permissions[k] = v
	}
	for k, v := range r.env.Devices {
		out.Devices[k] = v
	}
	for k, v := range r.env.Preference {
		out.Preference[k] = v
	}
	for k, v := range r.env.Current {
		out.Current[k] = v
	}
	for k, v := range r.env.Consented {
		out.Consented[k] = v
	}
	return out
}

// Register adds a provider. It rejects an invalid descriptor, a duplicate
// provider id and a dependency cycle, whose error names the path.
func (r *Registry) Register(p Provider) error {
	if !providerIDRE.MatchString(p.ID) {
		return fmt.Errorf("服务提供者 id %q 无效", p.ID)
	}
	p.Service.normalize()
	if err := p.Service.Validate(); err != nil {
		return err
	}
	for _, req := range p.Requires {
		if !serviceIDRE.MatchString(req.Service) {
			return fmt.Errorf("提供者 %s 的依赖 %q 应为 domain.verb", p.ID, req.Service)
		}
		if _, err := ParseRange(req.Version); err != nil {
			return fmt.Errorf("提供者 %s 对 %s 的版本要求无效：%w", p.ID, req.Service, err)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ids[p.ID] {
		return fmt.Errorf("服务提供者 %s 已注册", p.ID)
	}
	r.addLocked(p)
	if cycle := r.findCycleLocked(p.Service.ID); cycle != nil {
		r.removeLocked(p.ID)
		return fmt.Errorf("依赖循环：%s", strings.Join(cycle, " -> "))
	}
	return nil
}

func (r *Registry) addLocked(p Provider) {
	list := append(r.providers[p.Service.ID], p)
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	r.providers[p.Service.ID] = list
	r.ids[p.ID] = true
}

func (r *Registry) removeLocked(id string) {
	delete(r.ids, id)
	for service, list := range r.providers {
		kept := list[:0]
		for _, p := range list {
			if p.ID != id {
				kept = append(kept, p)
			}
		}
		if len(kept) == 0 {
			delete(r.providers, service)
			continue
		}
		r.providers[service] = kept
	}
}

// Catalog returns every registered provider, sorted by service and provider id.
func (r *Registry) Catalog() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	services := make([]string, 0, len(r.providers))
	for service := range r.providers {
		services = append(services, service)
	}
	sort.Strings(services)
	var out []Provider
	for _, service := range services {
		out = append(out, r.providers[service]...)
	}
	return out
}

// Services returns every registered descriptor, sorted by service key.
func (r *Registry) Services() []Service {
	out := make([]Service, 0)
	for _, p := range r.Catalog() {
		out = append(out, p.Service)
	}
	return out
}

// findCycleLocked walks the requires graph from start and returns the provider
// keys that close a cycle, or nil. Providers are sorted, so the reported path
// is deterministic.
func (r *Registry) findCycleLocked(start string) []string {
	var path []string          // service keys on the current path
	onPath := map[string]int{} // service id -> index of its first provider in path
	var walk func(service string) []string
	walk = func(service string) []string {
		if i, ok := onPath[service]; ok {
			return rotateCycle(append(append([]string{}, path[i:]...), path[i]))
		}
		onPath[service] = len(path)
		defer delete(onPath, service)
		for _, p := range r.providers[service] {
			path = append(path, p.Service.Key())
			for _, req := range p.Requires {
				if cycle := walk(req.Service); cycle != nil {
					return cycle
				}
			}
			path = path[:len(path)-1]
		}
		return nil
	}
	return walk(start)
}

// rotateCycle starts the reported path at its smallest service key, so one
// cycle reads the same whatever was registered last.
func rotateCycle(cycle []string) []string {
	body := cycle[:len(cycle)-1]
	first := 0
	for i, key := range body {
		if key < body[first] {
			first = i
		}
	}
	out := append(append([]string{}, body[first:]...), body[:first]...)
	return append(out, out[0])
}
