package pluginfabric

import (
	"reflect"
	"strings"
	"testing"
)

func readyFacts() BuiltinFacts {
	ready := Availability{State: StateReady}
	return BuiltinFacts{LocalASR: ready, CloudASR: ready, TranscriptSearch: ready, CanvasPreview: ready}
}

func builtinRegistry(t *testing.T, f BuiltinFacts) *Registry {
	t.Helper()
	r := New()
	r.Grant("library.read", "network")
	r.SetDevice("asr.local", true)
	mustRegister(t, r, BuiltinProviders(f)...)
	return r
}

func TestBuiltinProvidersAreValidAndStable(t *testing.T) {
	first := BuiltinProviders(readyFacts())
	if !reflect.DeepEqual(first, BuiltinProviders(readyFacts())) {
		t.Fatal("BuiltinProviders is not deterministic")
	}
	seen := map[string]bool{}
	for _, p := range first {
		if err := p.Service.Validate(); err != nil {
			t.Fatalf("%s: %v", p.ID, err)
		}
		if seen[p.ID] {
			t.Fatalf("duplicate provider id %s", p.ID)
		}
		seen[p.ID] = true
	}
	if len(first) != 4 {
		t.Fatalf("got %d providers, want the four built-in capabilities", len(first))
	}
}

func TestBuiltinTranscriptReadPrefersLocalThenAsksForCloud(t *testing.T) {
	r := builtinRegistry(t, readyFacts())
	res := mustResolve(t, r, readTranscript())
	if res.Provider != "core/local-asr" {
		t.Fatalf("selected %q, want the on-device provider", res.Provider)
	}
	if res.Consent != nil || !res.Usable() {
		t.Fatalf("local selection needs no consent, got %+v", res.Consent)
	}
	if res.Service.Cost.Tier != CostFree || res.Service.Boundary != BoundaryLocal {
		t.Fatalf("descriptor = %+v, want a free local provider", res.Service)
	}

	// The user picks the cloud channel while the local one is in use.
	r.Use("transcript.read", "core/local-asr")
	r.Prefer("transcript.read", "core/cloud-asr")
	res = mustResolve(t, r, readTranscript())
	if res.Provider != "core/cloud-asr" || res.Consent == nil {
		t.Fatalf("selection = %q consent = %+v, want the cloud provider asking for consent", res.Provider, res.Consent)
	}
	if res.Consent.From != BoundaryLocal || res.Consent.To != BoundaryCloud {
		t.Fatalf("consent = %+v, want local -> cloud", res.Consent)
	}

	r.Consent("transcript.read", BoundaryCloud)
	res = mustResolve(t, r, readTranscript())
	if res.Consent != nil || res.Provider != "core/cloud-asr" {
		t.Fatalf("after consent: provider %q consent %+v, want the cloud provider without a prompt", res.Provider, res.Consent)
	}
	if res.Service.Cost.Tier != CostMetered {
		t.Fatalf("cloud cost = %q, want the metered hint", res.Service.Cost.Tier)
	}
}

func TestBuiltinWithoutLocalModelFallsBackToCloudWithConsent(t *testing.T) {
	facts := readyFacts()
	facts.LocalASR = Availability{State: StateDisabled, Reason: "还没有安装本机模型"}
	r := builtinRegistry(t, facts)
	r.SetDevice("asr.local", false)

	res := mustResolve(t, r, readTranscript())
	if res.Provider != "core/cloud-asr" {
		t.Fatalf("selected %q, want the cloud provider", res.Provider)
	}
	if res.Consent == nil {
		t.Fatal("falling back to a cloud provider must ask for consent")
	}
	if len(res.Rejected) != 1 || res.Rejected[0].Step != StepDevice && res.Rejected[0].Step != StepAvailability {
		t.Fatalf("rejections = %+v, want the local provider rejected", res.Rejected)
	}
}

func TestBuiltinSearchAndCanvasPreviewResolve(t *testing.T) {
	r := builtinRegistry(t, readyFacts())
	for _, service := range []string{"transcript.search", "canvas.preview"} {
		res := mustResolve(t, r, Requirement{Service: service, Version: "^1"})
		if !res.Usable() {
			t.Fatalf("%s: %+v, want a usable provider", service, res)
		}
	}

	// A disabled path is reported with its own reason, not silently ignored.
	facts := readyFacts()
	facts.CanvasPreview = Availability{State: StateFailed, Reason: "渲染器不可用"}
	r = builtinRegistry(t, facts)
	res := mustResolve(t, r, Requirement{Service: "canvas.preview", Version: "^1"})
	if res.Resolved() {
		t.Fatalf("selected %q while the renderer failed", res.Provider)
	}
	if !strings.Contains(res.Recovery, "渲染器不可用") {
		t.Fatalf("recovery = %q, want the renderer's reason", res.Recovery)
	}
}
