package pluginfabric

import (
	"strings"
	"testing"
)

// transcriptProvider is a transcript.read provider with the fields the
// resolution tests vary.
func transcriptProvider(id, owner string, priority int, boundary Boundary, state string) Provider {
	return Provider{ID: id, Service: Service{
		ID: "transcript.read", Version: "1", Owner: owner, Title: id, Scope: ScopeLecture,
		Permissions: []string{"library.read"}, Cost: Cost{Tier: CostFree},
		Availability: Availability{State: state}, Boundary: boundary, Priority: priority,
	}}
}

func mustRegister(t *testing.T, r *Registry, providers ...Provider) {
	t.Helper()
	for _, p := range providers {
		if err := r.Register(p); err != nil {
			t.Fatalf("Register(%s): %v", p.ID, err)
		}
	}
}

func mustResolve(t *testing.T, r *Registry, req Requirement) Result {
	t.Helper()
	res, err := r.Resolve(req)
	if err != nil {
		t.Fatalf("Resolve(%s): %v", req.Service, err)
	}
	return res
}

func readTranscript() Requirement {
	return Requirement{Service: "transcript.read", Version: "^1"}
}

func TestResolveIsDeterministic(t *testing.T) {
	providers := []Provider{
		transcriptProvider("core/beta", "beta", 10, BoundaryLocal, StateReady),
		transcriptProvider("core/alpha", "alpha", 10, BoundaryLocal, StateReady),
		transcriptProvider("core/gamma", "gamma", 10, BoundaryLocal, StateReady),
	}
	// Every registration order must pick the same provider: priority ties fall
	// back to owner, then provider id.
	for _, order := range [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		r := New()
		r.Grant("library.read")
		for _, i := range order {
			mustRegister(t, r, providers[i])
		}
		res := mustResolve(t, r, readTranscript())
		if res.Provider != "core/alpha" {
			t.Fatalf("order %v selected %q, want core/alpha", order, res.Provider)
		}
		if len(res.Rejected) != 0 {
			t.Fatalf("order %v rejected %v, want none", order, res.Rejected)
		}
	}
}

func TestVersionMismatchIsUnavailable(t *testing.T) {
	r := New()
	r.Grant("library.read")
	mustRegister(t, r, transcriptProvider("core/local-asr", "core", 10, BoundaryLocal, StateReady))

	res := mustResolve(t, r, Requirement{Service: "transcript.read", Version: "^2"})
	if res.Resolved() {
		t.Fatalf("selected %q for a version it does not offer", res.Provider)
	}
	if len(res.Rejected) != 1 || res.Rejected[0].Step != StepVersion {
		t.Fatalf("rejections = %+v, want one version rejection", res.Rejected)
	}
	if !strings.Contains(res.Recovery, "请安装或升级") {
		t.Fatalf("recovery = %q, want an upgrade hint", res.Recovery)
	}
}

func TestPermissionMismatchIsUnavailableUntilGranted(t *testing.T) {
	r := New()
	mustRegister(t, r, transcriptProvider("core/cloud-asr", "core", 10, BoundaryLocal, StateReady))

	res := mustResolve(t, r, readTranscript())
	if res.Resolved() {
		t.Fatalf("selected %q without the required permission", res.Provider)
	}
	if len(res.Rejected) != 1 || res.Rejected[0].Step != StepPermission || !strings.Contains(res.Rejected[0].Reason, "library.read") {
		t.Fatalf("rejections = %+v, want a permission rejection naming library.read", res.Rejected)
	}
	if !strings.Contains(res.Recovery, "授权") {
		t.Fatalf("recovery = %q, want an authorization hint", res.Recovery)
	}

	r.Grant("library.read")
	if res := mustResolve(t, r, readTranscript()); !res.Usable() {
		t.Fatalf("still unresolved after granting the permission: %+v", res)
	}
}

func TestMissingDeviceFallsBackAndAsksForConsent(t *testing.T) {
	r := New()
	r.Grant("library.read", "network")
	local := transcriptProvider("core/local-asr", "core", 30, BoundaryLocal, StateReady)
	local.Service.Device = []string{"asr.local"}
	cloud := transcriptProvider("core/cloud-asr", "core", 10, BoundaryCloud, StateReady)
	mustRegister(t, r, local, cloud)

	res := mustResolve(t, r, readTranscript())
	if res.Provider != "core/cloud-asr" {
		t.Fatalf("selected %q, want the cloud provider as the only usable one", res.Provider)
	}
	if len(res.Rejected) != 1 || res.Rejected[0].Step != StepDevice {
		t.Fatalf("rejections = %+v, want the local provider rejected for a device capability", res.Rejected)
	}
	if res.Consent == nil || res.Consent.To != BoundaryCloud {
		t.Fatalf("consent = %+v, want a cloud consent request", res.Consent)
	}
	if res.Usable() {
		t.Fatal("a selection that needs consent must not report usable")
	}

	r.Consent("transcript.read", BoundaryCloud)
	res = mustResolve(t, r, readTranscript())
	if res.Consent != nil || !res.Usable() {
		t.Fatalf("consent = %+v after the user agreed, want none", res.Consent)
	}
}

func TestPreferenceSelectsProvider(t *testing.T) {
	r := New()
	r.Grant("library.read", "network")
	r.Consent("transcript.read", BoundaryCloud)
	mustRegister(t, r,
		transcriptProvider("core/local-asr", "core", 30, BoundaryLocal, StateReady),
		transcriptProvider("core/cloud-asr", "core", 10, BoundaryCloud, StateReady),
	)
	if res := mustResolve(t, r, readTranscript()); res.Provider != "core/local-asr" {
		t.Fatalf("default selection = %q, want the higher-priority local provider", res.Provider)
	}

	r.Prefer("transcript.read", "core/cloud-asr")
	if res := mustResolve(t, r, readTranscript()); res.Provider != "core/cloud-asr" {
		t.Fatalf("selection with a provider preference = %q, want core/cloud-asr", res.Provider)
	}

	// A preference may name an owner instead of a provider.
	mustRegister(t, r, transcriptProvider("plugin-x/reader", "plugin-x", 1, BoundaryLocal, StateReady))
	r.Prefer("transcript.read", "plugin-x")
	if res := mustResolve(t, r, readTranscript()); res.Provider != "plugin-x/reader" {
		t.Fatalf("selection with an owner preference = %q, want plugin-x/reader", res.Provider)
	}
}

func TestBoundarySwitchNeedsConsentAgain(t *testing.T) {
	r := New()
	r.Grant("library.read", "network")
	mustRegister(t, r,
		transcriptProvider("core/local-asr", "core", 30, BoundaryLocal, StateReady),
		transcriptProvider("core/cloud-asr", "core", 10, BoundaryCloud, StateReady),
	)
	r.Use("transcript.read", "core/local-asr")
	r.Consent("transcript.read", BoundaryLocal)
	r.Prefer("transcript.read", "core/cloud-asr")

	res := mustResolve(t, r, readTranscript())
	if res.Consent == nil {
		t.Fatal("switching from the local to the cloud provider must ask for consent")
	}
	if res.Consent.From != BoundaryLocal || res.Consent.To != BoundaryCloud {
		t.Fatalf("consent = %+v, want local -> cloud", res.Consent)
	}
	if !strings.Contains(res.Consent.Message, "本机处理改为") {
		t.Fatalf("consent message = %q, want it to name the boundary change", res.Consent.Message)
	}

	r.Consent("transcript.read", BoundaryCloud)
	if res := mustResolve(t, r, readTranscript()); res.Consent != nil {
		t.Fatalf("consent = %+v after agreeing, want none", res.Consent)
	}
}

func TestMissingRequiredDependencyIsUnavailable(t *testing.T) {
	r := New()
	c, err := r.ResolveRequires("review-lab", []Requirement{readTranscript()})
	if err != nil {
		t.Fatalf("ResolveRequires: %v", err)
	}
	if c.State != ContributionUnavailable {
		t.Fatalf("state = %q, want %q", c.State, ContributionUnavailable)
	}
	if len(c.Missing) != 1 || c.Missing[0] != "transcript.read" {
		t.Fatalf("missing = %v, want transcript.read", c.Missing)
	}
	if !strings.Contains(c.Recovery, "请安装或启用") {
		t.Fatalf("recovery = %q, want an install hint", c.Recovery)
	}
}

func TestOptionalDependencySelectsDegradedMode(t *testing.T) {
	r := New()
	r.Grant("library.read")
	mustRegister(t, r, transcriptProvider("core/local-asr", "core", 10, BoundaryLocal, StateReady))

	c, err := r.ResolveRequires("mind-map", []Requirement{
		readTranscript(),
		{Service: "llm.generate", Version: "^1", Optional: true, Fallback: "只提供规则式思维导图"},
	})
	if err != nil {
		t.Fatalf("ResolveRequires: %v", err)
	}
	if c.State != ContributionDegraded {
		t.Fatalf("state = %q, want %q", c.State, ContributionDegraded)
	}
	if len(c.Missing) != 0 {
		t.Fatalf("missing = %v, want none for an optional dependency", c.Missing)
	}
	if len(c.Degraded) != 1 || c.Degraded[0] != "llm.generate：只提供规则式思维导图" {
		t.Fatalf("degraded = %v, want the documented fallback", c.Degraded)
	}
}

func TestContributionWaitsForConsent(t *testing.T) {
	r := New()
	r.Grant("library.read", "network")
	mustRegister(t, r, transcriptProvider("core/cloud-asr", "core", 10, BoundaryCloud, StateReady))

	c, err := r.ResolveRequires("review-lab", []Requirement{readTranscript()})
	if err != nil {
		t.Fatalf("ResolveRequires: %v", err)
	}
	if c.State != ContributionNeedsConsent {
		t.Fatalf("state = %q, want %q", c.State, ContributionNeedsConsent)
	}
	if len(c.Consent) != 1 || c.Consent[0] != "transcript.read" {
		t.Fatalf("consent = %v, want transcript.read", c.Consent)
	}

	r.Consent("transcript.read", BoundaryCloud)
	if c, _ = r.ResolveRequires("review-lab", []Requirement{readTranscript()}); c.State != ContributionReady {
		t.Fatalf("state = %q after consent, want %q", c.State, ContributionReady)
	}
}

func TestCyclesAreRejectedWithThePath(t *testing.T) {
	r := New()
	mustRegister(t, r, Provider{ID: "a/one", Service: Service{
		ID: "a.one", Version: "1", Owner: "a", Scope: ScopeGlobal, Boundary: BoundaryLocal,
		Availability: Availability{State: StateReady},
	}, Requires: []Requirement{{Service: "b.two", Version: "^1"}}})

	err := r.Register(Provider{ID: "b/two", Service: Service{
		ID: "b.two", Version: "1", Owner: "b", Scope: ScopeGlobal, Boundary: BoundaryLocal,
		Availability: Availability{State: StateReady},
	}, Requires: []Requirement{{Service: "a.one", Version: "^1"}}})
	if err == nil {
		t.Fatal("Register accepted a dependency cycle")
	}
	if !strings.Contains(err.Error(), "a.one@1 -> b.two@1 -> a.one@1") {
		t.Fatalf("error = %q, want the cycle path", err)
	}
	if got := len(r.Services()); got != 1 {
		t.Fatalf("catalog has %d services after a rejected registration, want 1", got)
	}

	self := Provider{ID: "c/self", Service: Service{
		ID: "c.self", Version: "1", Owner: "c", Scope: ScopeGlobal, Boundary: BoundaryLocal,
		Availability: Availability{State: StateReady},
	}, Requires: []Requirement{{Service: "c.self", Version: "^1"}}}
	err = r.Register(self)
	if err == nil || !strings.Contains(err.Error(), "c.self@1 -> c.self@1") {
		t.Fatalf("self dependency error = %v, want the cycle path", err)
	}
}

func TestRegisterRejectsInvalidDescriptors(t *testing.T) {
	base := func() Provider {
		return transcriptProvider("core/local-asr", "core", 10, BoundaryLocal, StateReady)
	}
	cases := []struct {
		name   string
		mutate func(*Provider)
		want   string
	}{
		{"service id", func(p *Provider) { p.Service.ID = "transcriptread" }, "domain.verb"},
		{"version", func(p *Provider) { p.Service.Version = "one" }, "版本"},
		{"owner", func(p *Provider) { p.Service.Owner = "Core!" }, "owner"},
		{"scope", func(p *Provider) { p.Service.Scope = "world" }, "scope"},
		{"boundary", func(p *Provider) { p.Service.Boundary = "cloudy" }, "boundary"},
		{"availability", func(p *Provider) { p.Service.Availability.State = "ok" }, "availability"},
		{"duplicate permission", func(p *Provider) { p.Service.Permissions = []string{"library.read", "library.read"} }, "重复"},
		{"provider id", func(p *Provider) { p.ID = "Core/Local" }, "提供者 id"},
		{"requires", func(p *Provider) { p.Requires = []Requirement{{Service: "nope", Version: "^1"}} }, "domain.verb"},
		{"requires version", func(p *Provider) { p.Requires = []Requirement{{Service: "a.one", Version: "latest"}} }, "版本要求"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := New()
			p := base()
			c.mutate(&p)
			err := r.Register(p)
			if err == nil {
				t.Fatalf("Register accepted %+v", p)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error = %q, want it to mention %q", err, c.want)
			}
			if got := len(r.Services()); got != 0 {
				t.Fatalf("catalog has %d services after a rejected registration, want 0", got)
			}
		})
	}

	r := New()
	mustRegister(t, r, base())
	if err := r.Register(base()); err == nil || !strings.Contains(err.Error(), "已注册") {
		t.Fatalf("duplicate provider error = %v, want a duplicate rejection", err)
	}
}

func TestResolveRejectsMalformedRequirement(t *testing.T) {
	r := New()
	if _, err := r.Resolve(Requirement{Service: "transcript.read", Version: "latest"}); err == nil {
		t.Fatal("Resolve accepted a malformed version range")
	}
}

func TestUnavailableProviderReportsItsReason(t *testing.T) {
	r := New()
	r.Grant("library.read")
	p := transcriptProvider("core/local-asr", "core", 10, BoundaryLocal, StateFailed)
	p.Service.Availability.Reason = "模型未安装"
	mustRegister(t, r, p)

	res := mustResolve(t, r, readTranscript())
	if res.Resolved() {
		t.Fatalf("selected %q while it reported a failure", res.Provider)
	}
	if len(res.Rejected) != 1 || res.Rejected[0].State != StateFailed {
		t.Fatalf("rejections = %+v, want the failed state", res.Rejected)
	}
	if !strings.Contains(res.Recovery, "模型未安装") {
		t.Fatalf("recovery = %q, want the provider's own reason", res.Recovery)
	}
}

func TestVersionRangeMatching(t *testing.T) {
	cases := []struct {
		spec    string
		version string
		want    bool
	}{
		{"1", "1", true},
		{"1", "2", false},
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "1.2.4", false},
		{"^1", "1", true},
		{"^1", "1.9.9", true},
		{"^1", "2", false},
		{"^1.2", "1.1.9", false},
		{">=1.2", "1.2.0", true},
		{">=1.2", "1.1.9", false},
	}
	for _, c := range cases {
		rng, err := ParseRange(c.spec)
		if err != nil {
			t.Fatalf("ParseRange(%q): %v", c.spec, err)
		}
		v, err := ParseVersion(c.version)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", c.version, err)
		}
		if got := rng.Matches(v); got != c.want {
			t.Fatalf("%s matches %s = %v, want %v", c.spec, c.version, got, c.want)
		}
	}
}
