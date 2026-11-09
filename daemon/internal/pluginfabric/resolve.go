package pluginfabric

import (
	"fmt"
	"sort"
	"strings"
)

// Resolution steps, in the order the resolver applies them (spec 0031):
// version, permission and device capability, user preference, availability,
// priority, then a deterministic tie-break.
const (
	StepVersion      = "version"
	StepPermission   = "permission"
	StepDevice       = "device"
	StepAvailability = "availability"
)

// Rejection is one provider that could not serve a requirement, the step that
// removed it and a readable reason.
type Rejection struct {
	Provider string `json:"provider"`
	Service  string `json:"service"`
	Step     string `json:"step"`
	// State is the provider's availability state for StepAvailability.
	State  string `json:"state,omitempty"`
	Reason string `json:"reason"`
}

// Consent is a selection that changes where the user's data goes. The caller
// must not run it before the user agrees (spec 0031).
type Consent struct {
	Service  string   `json:"service"`
	Provider string   `json:"provider"`
	From     Boundary `json:"from,omitempty"`
	To       Boundary `json:"to"`
	Message  string   `json:"message"`
}

// Result is one requirement resolved.
type Result struct {
	Requirement Requirement `json:"requirement"`
	// Provider is the selected provider id; empty when nothing was selected.
	Provider string      `json:"provider,omitempty"`
	Service  *Service    `json:"service,omitempty"`
	Consent  *Consent    `json:"consent,omitempty"`
	Rejected []Rejection `json:"rejected,omitempty"`
	// Recovery names the step that would make the requirement resolvable.
	Recovery string `json:"recovery,omitempty"`
}

// Resolved reports whether a provider was selected.
func (res Result) Resolved() bool { return res.Service != nil }

// Usable reports whether a provider was selected and needs no consent first.
func (res Result) Usable() bool { return res.Service != nil && res.Consent == nil }

// Resolve selects one provider for a requirement. An error means the
// requirement itself is malformed, not that nothing was available.
//
// The order is the one in spec 0031: version match, permission and device
// capability, the user's preference, availability, priority and a
// deterministic tie-break. An unavailable provider is never selected, so
// availability also filters: a preferred provider that cannot run falls back
// to the next one, and a fallback that changes the data boundary still asks
// for consent.
func (r *Registry) Resolve(req Requirement) (Result, error) {
	want, err := ParseRange(req.Version)
	if err != nil {
		return Result{Requirement: req}, fmt.Errorf("%s 的版本要求无效：%w", req.Service, err)
	}
	res := Result{Requirement: req}
	r.mu.RLock()
	defer r.mu.RUnlock()
	providers := r.providers[req.Service]
	if len(providers) == 0 {
		res.Recovery = fmt.Sprintf("没有可用的 %s 提供者：请安装或启用提供该能力的插件", req.Service)
		return res, nil
	}

	eligible := make([]Provider, 0, len(providers))
	for _, p := range providers {
		if v, err := ParseVersion(p.Service.Version); err != nil || !want.Matches(v) {
			res.Rejected = append(res.Rejected, Rejection{Provider: p.ID, Service: p.Service.Key(), Step: StepVersion,
				Reason: fmt.Sprintf("需要 %s，提供 %s", want, p.Service.Version)})
			continue
		}
		if missing := missingFrom(r.env.Permissions, p.Service.Permissions); len(missing) > 0 {
			res.Rejected = append(res.Rejected, Rejection{Provider: p.ID, Service: p.Service.Key(), Step: StepPermission,
				Reason: "权限 " + strings.Join(missing, "、")})
			continue
		}
		if missing := missingFrom(r.env.Devices, p.Service.Device); len(missing) > 0 {
			res.Rejected = append(res.Rejected, Rejection{Provider: p.ID, Service: p.Service.Key(), Step: StepDevice,
				Reason: "设备能力 " + strings.Join(missing, "、")})
			continue
		}
		if !p.Service.Availability.Ready() {
			res.Rejected = append(res.Rejected, Rejection{Provider: p.ID, Service: p.Service.Key(), Step: StepAvailability,
				State: p.Service.Availability.State, Reason: availabilityReason(p.Service.Availability)})
			continue
		}
		eligible = append(eligible, p)
	}
	if len(eligible) == 0 {
		res.Recovery = recoveryHint(req.Service, res.Rejected)
		return res, nil
	}

	preference := r.env.Preference[req.Service]
	sort.SliceStable(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		if pa, pb := preferred(a, preference), preferred(b, preference); pa != pb {
			return pa
		}
		if a.Service.Priority != b.Service.Priority {
			return a.Service.Priority > b.Service.Priority
		}
		if a.Service.Owner != b.Service.Owner {
			return a.Service.Owner < b.Service.Owner
		}
		return a.ID < b.ID
	})
	chosen := eligible[0].Service
	res.Provider = eligible[0].ID
	res.Service = &chosen
	if chosen.Boundary == BoundaryCloud && r.env.Consented[req.Service] != BoundaryCloud {
		from := r.boundaryLocked(req.Service)
		res.Consent = &Consent{Service: req.Service, Provider: res.Provider, From: from, To: BoundaryCloud,
			Message: consentMessage(req.Service, from)}
	}
	return res, nil
}

// preferred reports whether a provider is the one the user asked for; a
// preference names either a provider id or its owner.
func preferred(p Provider, preference string) bool {
	return preference != "" && (preference == p.ID || preference == p.Service.Owner)
}

func missingFrom(have map[string]bool, names []string) []string {
	var missing []string
	for _, name := range names {
		if !have[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

// boundaryLocked is the data boundary of the provider now serving a service.
func (r *Registry) boundaryLocked(service string) Boundary {
	key := r.env.Current[service]
	for _, p := range r.providers[service] {
		if p.ID == key {
			return p.Service.Boundary
		}
	}
	return ""
}

func availabilityReason(a Availability) string {
	if a.Reason != "" {
		return a.Reason
	}
	switch a.State {
	case StateNeedsConsent:
		return "需要用户同意"
	case StateNeedsAuth:
		return "需要登录"
	case StateDisabled:
		return "已停用"
	case StateFailed:
		return "启动失败"
	}
	return a.State
}

func consentMessage(service string, from Boundary) string {
	if from == BoundaryLocal {
		return service + " 会从本机处理改为把数据发送到云端，需要重新确认"
	}
	return service + " 需要把数据发送到云端，需要确认"
}

// recoveryHint names the first step that would make the requirement
// resolvable, in resolution order.
func recoveryHint(service string, rejected []Rejection) string {
	for _, step := range []string{StepPermission, StepAvailability, StepDevice, StepVersion} {
		for _, rj := range rejected {
			if rj.Step != step {
				continue
			}
			switch step {
			case StepPermission:
				return fmt.Sprintf("请先在「设置 → 插件」中授权 %s 需要的%s", service, rj.Reason)
			case StepAvailability:
				switch rj.State {
				case StateNeedsConsent:
					return "请先在「设置 → 插件」中同意 " + service + " 的数据使用方式"
				case StateNeedsAuth:
					return "请先登录提供 " + service + " 的来源"
				case StateDisabled:
					return "请在「设置 → 插件」中启用提供 " + service + " 的插件"
				}
				return "提供 " + service + " 的插件暂不可用：" + rj.Reason
			case StepDevice:
				return "本机缺少" + rj.Reason + "，无法使用 " + service
			case StepVersion:
				return "请安装或升级提供 " + service + " 的插件（" + rj.Reason + "）"
			}
		}
	}
	return "没有可用的 " + service + " 提供者：请安装或启用提供该能力的插件"
}

// Contribution states: an unmet required dependency makes a contribution
// unavailable, an unmet optional one selects a documented degraded mode, and a
// selection that crosses the data boundary waits for consent (spec 0031).
const (
	ContributionReady        = "ready"
	ContributionDegraded     = "degraded"
	ContributionNeedsConsent = "needs_consent"
	ContributionUnavailable  = "unavailable"
)

// Contribution is one plugin's `requires` resolved against the catalog.
type Contribution struct {
	Plugin  string   `json:"plugin"`
	State   string   `json:"state"`
	Results []Result `json:"results"`
	// Missing lists required services with no usable provider.
	Missing []string `json:"missing,omitempty"`
	// Degraded lists optional services with no provider and the fallback used.
	Degraded []string `json:"degraded,omitempty"`
	// Consent lists services waiting for the user's agreement.
	Consent []string `json:"consent,omitempty"`
	// Recovery names the first step that would make the contribution usable.
	Recovery string `json:"recovery,omitempty"`
}

// ResolveRequires resolves every requirement of one contribution.
func (r *Registry) ResolveRequires(plugin string, reqs []Requirement) (Contribution, error) {
	c := Contribution{Plugin: plugin, State: ContributionReady, Results: make([]Result, 0, len(reqs))}
	for _, req := range reqs {
		res, err := r.Resolve(req)
		if err != nil {
			return Contribution{Plugin: plugin}, err
		}
		c.Results = append(c.Results, res)
		switch {
		case res.Service == nil && req.Optional:
			c.Degraded = append(c.Degraded, req.Service+"："+fallback(req))
		case res.Service == nil:
			c.Missing = append(c.Missing, req.Service)
			if c.Recovery == "" {
				c.Recovery = res.Recovery
			}
		case res.Consent != nil:
			c.Consent = append(c.Consent, req.Service)
		}
	}
	switch {
	case len(c.Missing) > 0:
		c.State = ContributionUnavailable
	case len(c.Consent) > 0:
		c.State = ContributionNeedsConsent
	case len(c.Degraded) > 0:
		c.State = ContributionDegraded
	}
	return c, nil
}

func fallback(req Requirement) string {
	if req.Fallback != "" {
		return req.Fallback
	}
	return "该能力不可用，相关功能降级"
}
