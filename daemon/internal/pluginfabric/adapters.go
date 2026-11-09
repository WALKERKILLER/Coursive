package pluginfabric

// This file maps capabilities the daemon already implements to service
// descriptors so the fabric can resolve them. It is a mapping only: nothing
// here calls the implementations, changes their code paths or replaces their
// routes. Wiring a caller passes the facts it reads from the owning subsystem;
// until then the descriptors are inert data.
//
// Where each fact comes from:
//   - LocalASR: internal/localasr (engine and model installs) and the asr
//     pool's local channel; GET /api/asr/local.
//   - CloudASR: internal/asr channels; GET /api/asr.
//   - TranscriptSearch: the library's full-text search; GET /api/search.
//   - CanvasPreview: canvas and PDF previews; POST /api/export/pdf-preview and
//     the study-canvas material route (internal/library).
//
// The task that changes a data boundary, such as on-device recognition to a
// cloud channel, is the resolver's consent case (spec 0031), not this file's.

// BuiltinOwner owns the services the daemon implements itself.
const BuiltinOwner = "core"

// BuiltinFacts is what the daemon already knows about its own capabilities.
// Report StateReady only when the path really works: a missing model, engine,
// index or renderer is StateDisabled or StateFailed with a readable reason.
type BuiltinFacts struct {
	LocalASR         Availability
	CloudASR         Availability
	TranscriptSearch Availability
	CanvasPreview    Availability
}

// BuiltinProviders maps those capabilities to service descriptors, in a fixed
// order so two catalogs built from the same facts are identical.
func BuiltinProviders(f BuiltinFacts) []Provider {
	return []Provider{
		{
			ID: "core/local-asr",
			Service: Service{
				ID: "transcript.read", Version: "1", Owner: BuiltinOwner, Title: "本机语音识别",
				Scope: ScopeLecture,
				Input: Schema{Ref: "coursive/transcript.read.input@1", Fields: []Field{
					{Name: "lecture_id", Type: "string", Required: true},
					{Name: "from_ms", Type: "number"},
					{Name: "to_ms", Type: "number"},
				}},
				Output: Schema{Ref: "coursive/transcript.read.output@1", Fields: []Field{
					{Name: "segments", Type: "array", Required: true, Note: "{start_ms, end_ms, text}"},
				}},
				Permissions:  []string{"library.read"},
				Device:       []string{"asr.local"},
				Cost:         Cost{Tier: CostFree, Detail: "在本机运行，不联网"},
				Availability: f.LocalASR,
				Boundary:     BoundaryLocal,
				Priority:     30,
			},
		},
		{
			ID: "core/cloud-asr",
			Service: Service{
				ID: "transcript.read", Version: "1", Owner: BuiltinOwner, Title: "云端语音识别",
				Scope: ScopeLecture,
				Input: Schema{Ref: "coursive/transcript.read.input@1", Fields: []Field{
					{Name: "lecture_id", Type: "string", Required: true},
					{Name: "from_ms", Type: "number"},
					{Name: "to_ms", Type: "number"},
				}},
				Output: Schema{Ref: "coursive/transcript.read.output@1", Fields: []Field{
					{Name: "segments", Type: "array", Required: true, Note: "{start_ms, end_ms, text}"},
				}},
				Permissions:  []string{"library.read", "network"},
				Cost:         Cost{Tier: CostMetered, Detail: "按音频时长计费"},
				Availability: f.CloudASR,
				Boundary:     BoundaryCloud,
				Priority:     10,
			},
		},
		{
			ID: "core/transcript-search",
			Service: Service{
				ID: "transcript.search", Version: "1", Owner: BuiltinOwner, Title: "转写与笔记全文搜索",
				Scope: ScopeGlobal,
				Input: Schema{Ref: "coursive/transcript.search.input@1", Fields: []Field{
					{Name: "query", Type: "string", Required: true},
					{Name: "course_id", Type: "string"},
					{Name: "limit", Type: "number"},
				}},
				Output: Schema{Ref: "coursive/transcript.search.output@1", Fields: []Field{
					{Name: "hits", Type: "array", Required: true, Note: "{lecture_id, start_ms, snippet}"},
				}},
				Permissions:  []string{"library.read"},
				Cost:         Cost{Tier: CostFree, Detail: "本机索引"},
				Availability: f.TranscriptSearch,
				Boundary:     BoundaryLocal,
				Priority:     10,
			},
		},
		{
			ID: "core/canvas-preview",
			Service: Service{
				ID: "canvas.preview", Version: "1", Owner: BuiltinOwner, Title: "画布与 PDF 预览",
				Scope: ScopeCanvas,
				Input: Schema{Ref: "coursive/canvas.preview.input@1", Fields: []Field{
					{Name: "source", Type: "string", Required: true, Note: "画布文档或 PDF 引用"},
					{Name: "page", Type: "number"},
				}},
				Output: Schema{Ref: "coursive/canvas.preview.output@1", Fields: []Field{
					{Name: "image", Type: "string", Required: true},
					{Name: "width", Type: "number"},
					{Name: "height", Type: "number"},
				}},
				Permissions:  []string{"library.read"},
				Cost:         Cost{Tier: CostFree, Detail: "本机渲染"},
				Availability: f.CanvasPreview,
				Boundary:     BoundaryLocal,
				Priority:     10,
			},
		},
	}
}
