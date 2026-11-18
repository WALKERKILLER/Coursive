package study

// ReconcileQuery is reconcileStudyQuery: it drops stale live references
// from a conversation's query without rewriting historical answers or
// citations.
func ReconcileQuery(query M, d *Document) M {
	out := obj(Clone(query))
	if out == nil {
		out = M{}
	}
	visible := map[string]bool{}
	for _, l := range d.Layers {
		if l.Visible {
			visible[l.ID] = true
		}
	}
	cards := map[string]bool{}
	for _, c := range d.Cards {
		if visible[c.Layer] {
			cards[c.ID] = true
		}
	}
	chosen := map[string]bool{}
	var order []any
	for _, o := range list(out["objects"]) {
		if id, ok := o.(string); ok && cards[id] && !chosen[id] {
			chosen[id] = true
			order = append(order, id)
		}
	}
	if _, ok := out["objects"]; ok {
		out["objects"] = nonNil(order)
	}
	// The daemon can append lecture subtitles independently of objects.
	// Removing that binding also revokes its context flag and time scope.
	if !chosen["lecture"] {
		if _, ok := out["include_video"]; ok {
			out["include_video"] = false
		}
		// The daemon restores zero defaults on read; they carry no video
		// scope. Keeping them means reopening a PDF-only conversation is
		// not a repair.
		for _, f := range []string{"video_ms", "from_ms", "to_ms"} {
			if v, ok := out[f]; ok && num(v, -1) != 0 {
				delete(out, f)
			}
		}
	}
	for _, key := range []string{"selections", "object_pages", "object_video_ms", "video_ranges"} {
		if values, ok := out[key].(M); ok {
			for id := range values {
				if !chosen[id] {
					delete(values, id)
				}
			}
		}
	}
	if sel, ok := out["selection"].(M); ok {
		object := sel["object_id"]
		if object != nil && object != "" && !chosen[str(object)] {
			out["selection"] = M{}
		} else if ids, ok := sel["stroke_ids"].([]any); ok {
			strokes := map[string]bool{}
			for _, s := range d.Strokes {
				if visible[s.Layer] {
					strokes[s.ID] = true
				}
			}
			kept := []any{}
			for _, id := range ids {
				if s, ok := id.(string); ok && strokes[s] {
					kept = append(kept, s)
				}
			}
			sel["stroke_ids"] = kept
		}
	}
	return out
}
