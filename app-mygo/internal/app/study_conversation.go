package app

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
)

// study_conversation_ui.dart's _ConversationBodyState and
// study_visual.dart: one conversation of the canvas, its bound material,
// screenshots and turns. Its async methods run on their own goroutine and
// change state only on the UI thread (onUI); see study_async.go.

// studyConversation is a conversation the canvas mounted this session; a
// hidden one keeps its run going.
type studyConversation struct {
	s      *StudyCanvas
	cardID string
	// id is the conversation's own id, the card's conversationId.
	id            string
	input         string
	data          core.J
	preview, live core.J
	pending       []*studySnapshot
	visuals       map[string]core.J
	fingerprints  map[string]string
	err, runID    string
	notice        string
	busy, loading bool
	stopRequested bool
	disposed      bool
	saveTimer     *studyTimer
	saveMu        sync.Mutex
	view          stageViewState
}

func (s *StudyCanvas) mountConversation(card *study.Card) *studyConversation {
	if cv := s.conversations[card.ID]; cv != nil {
		return cv
	}
	cv := &studyConversation{s: s, cardID: card.ID, id: card.ConversationID, visuals: map[string]core.J{},
		fingerprints: map[string]string{}, view: newStageViewState()}
	s.conversations[card.ID] = cv
	go cv.load()
	return cv
}

func (cv *studyConversation) lecture() string { return cv.s.lectureID() }

func (cv *studyConversation) turns() []core.J { return cv.data.Os("turns") }

func (cv *studyConversation) attachments() []any { return core.List(cv.data["draft_attachments"]) }

// query is a deep copy of the bound query.
func (cv *studyConversation) query() core.J {
	q := core.Obj(study.Clone(plainJSON(cv.data["query"])))
	if q == nil {
		q = core.J{}
	}
	return q
}

func (cv *studyConversation) gone() bool {
	return onUIValue(func() bool { return cv.disposed || cv.s.disposed })
}

func (cv *studyConversation) load() {
	out, err := State.Api().StudyConversation(cv.lecture(), cv.id)
	var initial *conversationStart
	ready := onUIValue(func() bool {
		if cv.disposed {
			return false
		}
		if err != nil {
			cv.err = err.Error()
			return false
		}
		data := core.Obj(plainJSON(out))
		if data == nil {
			data = core.J{}
		}
		if data["draft_attachments"] == nil {
			data["draft_attachments"] = []any{}
		}
		if data["context_scope"] == nil {
			data["context_scope"] = "selected"
		}
		if data["turns"] == nil {
			data["turns"] = []any{}
		}
		cv.input = data.S("draft")
		for _, t := range data.Os("turns") {
			studyUpgradeCitations(t)
			if st := t.S("state"); st == "running" || st == "preparing" {
				t["state"], t["error"] = "interrupted", l10n.StudyTurnReopened
			}
		}
		cv.data = data
		if in := cv.s.initialConversations[cv.cardID]; in != nil && data.I("revision") == 0 {
			initial = in
			cv.input = in.draft
		}
		return true
	})
	if !ready {
		return
	}
	if initial == nil {
		cv.refreshContext()
		return
	}
	cv.bind(initial.query, nil)
	if initial.snapshot != nil {
		cv.attachSnapshot(initial.snapshot)
	}
	// The first question typed on the idle stage, or 「选择参考资料」.
	onUI(func() { delete(cv.s.initialConversations, cv.cardID) })
	if cv.gone() {
		return
	}
	switch {
	case initial.send != "":
		onUI(func() {
			cv.data["allow_images"] = initial.allowImages
			cv.input = initial.send
		})
		cv.start(nil, "", initial.task)
	case initial.pick:
		onUI(cv.pickSources)
	}
}

// save writes the conversation; saves run one after another.
func (cv *studyConversation) save() bool {
	var snapshot core.J
	onUI(func() {
		cv.saveTimer.stop()
		if cv.data == nil {
			return
		}
		snapshot = core.Obj(study.Clone(plainJSON(cv.data)))
		snapshot["draft"] = cv.input
		snapshot["version"] = 2
	})
	if snapshot == nil {
		return false
	}
	cv.saveMu.Lock()
	defer cv.saveMu.Unlock()
	onUI(func() { snapshot["revision"] = cv.data["revision"] })
	out, err := State.Api().SaveStudyConversation(cv.lecture(), cv.id, snapshot)
	onUI(func() {
		if err != nil {
			if !cv.disposed {
				cv.err = l10n.StudyConversationSaveFailed(err.Error())
			}
			return
		}
		cv.data["revision"] = core.Obj(out)["revision"]
		cv.data["version"] = 2
	})
	return err == nil
}

// saveSoon saves half a second after the last edit of the draft.
func (cv *studyConversation) saveSoon() {
	cv.saveTimer.stop()
	cv.saveTimer = studyAfter(500*time.Millisecond, func() { go cv.save() })
}

func (cv *studyConversation) attachSnapshot(snap *studySnapshot) {
	ok := onUIValue(func() bool {
		if cv.data == nil {
			return false
		}
		if !slices.Contains(cv.pending, snap) {
			// The limit is said where the shots are, not thrown at the
			// canvas.
			if len(cv.attachments())+len(cv.pending) >= 2 {
				cv.err = l10n.StudySelectionImageLimit
				return false
			}
			cv.pending = append(cv.pending, snap)
		}
		return true
	})
	if !ok {
		return
	}
	attachment, err := snap.upload(cv.lecture())
	saved := onUIValue(func() bool {
		if cv.disposed {
			return false
		}
		if err != nil {
			cv.err = err.Error()
			return false
		}
		cv.pending = slices.DeleteFunc(cv.pending, func(p *studySnapshot) bool { return p == snap })
		for _, a := range cv.attachments() {
			if core.Obj(a).S("id") == attachment.S("id") {
				return true
			}
		}
		cv.data["draft_attachments"] = append(cv.attachments(), map[string]any(attachment))
		return true
	})
	if saved {
		cv.save()
	}
}

// bind adds a selection's material to what the conversation reads.
func (cv *studyConversation) bind(added core.J, images []core.J) {
	ok := onUIValue(func() bool {
		if cv.data == nil || cv.busy || cv.loading {
			return false
		}
		previous, q := cv.query(), cv.query()
		seen := map[string]bool{}
		var objects []any
		for _, id := range append(q.Ss("objects"), added.Ss("objects")...) {
			if !seen[id] && len(objects) < 12 {
				seen[id] = true
				objects = append(objects, id)
			}
		}
		q["objects"] = nonNilList(objects)
		replaced := map[string]bool{}
		for _, id := range added.Ss("objects") {
			replaced[id] = true
		}
		if sel := q.O("selection"); sel != nil && replaced[sel.S("object_id")] {
			q["selection"] = map[string]any{}
		}
		for _, key := range []string{"selections", "object_pages", "object_video_ms", "video_ranges"} {
			if m := q.O(key); m != nil {
				for id := range m {
					if replaced[id] {
						delete(m, id)
					}
				}
			}
		}
		if len(added.O("selection")) > 0 {
			q["selection"] = added["selection"]
		}
		if added.B("include_video") {
			q["include_video"] = true
			q["video_ms"] = added["video_ms"]
			delete(q, "from_ms")
			delete(q, "to_ms")
		}
		for _, key := range []string{"selections", "object_pages", "object_video_ms"} {
			merged := map[string]any{}
			for k, v := range q.O(key) {
				merged[k] = v
			}
			for k, v := range added.O(key) {
				merged[k] = v
			}
			q[key] = merged
		}
		for id := range cv.visuals {
			if studyVisualBindingChanged(id, previous, q) {
				delete(cv.visuals, id)
			}
		}
		for id := range cv.fingerprints {
			if cv.visuals[id] == nil {
				delete(cv.fingerprints, id)
			}
		}
		cv.data["query"] = map[string]any(q)
		for _, img := range images {
			cv.visuals[img.S("source_id")] = img
			delete(cv.fingerprints, img.S("source_id"))
		}
		return true
	})
	if !ok {
		return
	}
	cv.save()
	cv.refreshContext()
}

func nonNilList(l []any) []any {
	if l == nil {
		return []any{}
	}
	return l
}

// refreshContext reconciles the query with the canvas and previews what
// the conversation reads; it reports a repaired query.
func (cv *studyConversation) refreshContext() bool {
	if !onUIValue(func() bool {
		if cv.data == nil || cv.disposed {
			return false
		}
		cv.loading = true
		return true
	}) {
		return false
	}
	defer onUI(func() { cv.loading = false })
	fail := func(err error) bool {
		onUI(func() { cv.err, cv.preview = err.Error(), nil })
		return false
	}
	if !cv.s.flushWait() {
		return fail(formatError(l10n.StudyConversationUnsaved))
	}
	var q core.J
	repaired := false
	if !onUIValue(func() bool {
		d := cv.s.doc
		if cv.disposed || d == nil {
			return false
		}
		previous := cv.query()
		q = core.Obj(study.ReconcileQuery(plainMap(previous), d))
		repaired = study.Encode(plainJSON(previous)) != study.Encode(plainJSON(q))
		q["revision"] = d.Revision
		cv.data["query"] = map[string]any(q)
		if repaired {
			cv.notice = l10n.StudyReferencesRemoved
			// Removing unrelated video timing fields must not discard a
			// fresh PDF crop: only attachments whose binding changed go.
			for id := range cv.visuals {
				if studyVisualBindingChanged(id, previous, q) {
					delete(cv.visuals, id)
				}
			}
			for id := range cv.fingerprints {
				if cv.visuals[id] == nil {
					delete(cv.fingerprints, id)
				}
			}
		}
		return true
	}) {
		return false
	}
	if repaired && (!cv.save() || cv.gone()) {
		return true
	}
	out, err := State.Api().StudyContext(cv.lecture(), q)
	if err != nil {
		fail(err)
		return repaired
	}
	onUI(func() {
		if cv.disposed {
			return
		}
		cv.preview = core.Obj(plainJSON(out))
		ids := map[string]bool{}
		for _, src := range cv.preview.Os("sources") {
			ids[src.S("id")] = true
		}
		for id := range cv.visuals {
			if !ids[id] {
				delete(cv.visuals, id)
			}
		}
		for id := range cv.fingerprints {
			if cv.visuals[id] == nil {
				delete(cv.fingerprints, id)
			}
		}
		for _, src := range cv.preview.Os("sources") {
			id := src.S("id")
			if cv.visuals[id] != nil {
				if _, ok := cv.fingerprints[id]; !ok {
					cv.fingerprints[id] = cv.fingerprint(src)
				}
			}
		}
		cv.err = ""
	})
	return repaired
}

func (cv *studyConversation) turnsOf(source core.J) int {
	if card := cv.s.doc.Card(source.O("anchor").S("object_id")); card != nil {
		return card.QuarterTurns
	}
	return 0
}

func (cv *studyConversation) fingerprint(source core.J) string {
	return studyVisualFingerprint(source, cv.turnsOf(source))
}

func (cv *studyConversation) rotatedOnly(id string, source core.J) bool {
	for t := range 4 {
		if studyVisualFingerprint(source, t) == cv.fingerprints[id] {
			return true
		}
	}
	return false
}

// captureBound takes the screenshot now that 「看图」 is on, while the
// canvas still shows exactly the bound selection.
func (cv *studyConversation) captureBound() {
	if !onUIValue(func() bool {
		s := cv.s
		bound := cv.query().O("selection")
		return s.regionSelection != nil && len(bound) > 0 && study.Encode(plainJSON(bound)) == study.Encode(s.detail.JSON())
	}) {
		return
	}
	snap, err := cv.s.prepareSelectionSnapshotWait()
	if err != nil {
		onUI(func() { cv.err = err.Error() })
		return
	}
	cv.attachSnapshot(snap)
}

func (cv *studyConversation) removeSource(source core.J) {
	onUI(func() {
		q := cv.query()
		id := source.O("anchor").S("object_id")
		if source.S("id") == "lecture-video" {
			q["include_video"] = false
		}
		var objects []any
		for _, v := range q.L("objects") {
			if core.Str(v) != id {
				objects = append(objects, v)
			}
		}
		q["objects"] = nonNilList(objects)
		if source.S("id") == "selected-ink" || q.O("selection").S("object_id") == id {
			q["selection"] = map[string]any{}
		}
		delete(q.O("selections"), id)
		delete(q.O("object_pages"), id)
		delete(cv.visuals, source.S("id"))
		delete(cv.fingerprints, source.S("id"))
		cv.data["query"] = map[string]any(q)
	})
	cv.save()
	cv.refreshContext()
}

// start asks the composer's question, question, or again a failed retry
// turn; task propose_outline drafts a review outline instead.
func (cv *studyConversation) start(retry core.J, question, task string) {
	var sentAttachments []any
	var sentScope any
	if !onUIValue(func() bool {
		if task == "" && retry != nil {
			task = retry.S("task")
		}
		switch {
		case retry != nil && retry.S("question") != "":
			question = retry.S("question")
		case question == "":
			question = cv.input
		}
		question = strings.TrimSpace(question)
		if question == "" || cv.busy || cv.loading || cv.data == nil || len(cv.pending) > 0 {
			return false
		}
		atts := cv.data["draft_attachments"]
		if retry != nil && retry["attachments"] != nil {
			atts = retry["attachments"]
		}
		sentAttachments = nonNilList(core.List(study.Clone(plainJSON(atts))))
		sentScope = cv.data["context_scope"]
		if retry != nil && retry["context_scope"] != nil {
			sentScope = retry["context_scope"]
		}
		cv.stopRequested, cv.runID, cv.busy = false, "", true
		return true
	}) {
		return
	}
	idle := func(msg string) {
		onUI(func() {
			cv.busy = false
			if msg != "" {
				cv.err = msg
			}
		})
	}
	repaired := cv.refreshContext()
	if onUIValue(func() bool { return repaired || cv.preview == nil || cv.disposed || cv.stopRequested }) {
		idle("")
		return
	}
	allowImages := onUIValue(func() bool { return cv.data.B("allow_images") })
	if len(sentAttachments) == 0 && allowImages {
		if err := cv.restoreVisuals(); err != nil {
			if !errors.Is(err, errGone) {
				idle(err.Error())
			} else {
				idle("")
			}
			return
		}
	}
	var turn core.J
	if !onUIValue(func() bool {
		if len(cv.turns()) >= 300 {
			cv.busy, cv.err = false, l10n.StudyConversationFull
			return false
		}
		turn = retry
		if turn == nil {
			turn = core.J{"id": study.NewID(), "question": question, "answer": "", "context_key": "", "state": "preparing"}
			if task != "" {
				turn["task"] = task
			}
		}
		turn["state"] = "preparing"
		turn["attachments"] = sentAttachments
		turn["context_scope"] = sentScope
		if retry == nil {
			cv.data["turns"] = append(core.List(cv.data["turns"]), map[string]any(turn))
			sent := map[string]bool{}
			for _, a := range sentAttachments {
				sent[core.Obj(a).S("id")] = true
			}
			cv.data["draft_attachments"] = nonNilList(slices.DeleteFunc(slices.Clone(cv.attachments()), func(a any) bool { return sent[core.Obj(a).S("id")] }))
		}
		// Recovery can take time; keep a newer draft, including when
		// retrying history.
		if retry == nil && strings.TrimSpace(cv.input) == question {
			cv.input = ""
		}
		cv.busy, cv.err, cv.notice, cv.live = true, "", "", nil
		cv.view.toBottom = true
		return true
	}) {
		return
	}
	if !cv.save() {
		onUI(func() {
			if retry == nil && cv.input == "" {
				cv.data["turns"] = slices.DeleteFunc(slices.Clone(core.List(cv.data["turns"])), func(t any) bool { return core.Obj(t).S("id") == turn.S("id") })
				cv.input = question
				for _, a := range sentAttachments {
					if !slices.ContainsFunc(cv.attachments(), func(v any) bool { return core.Obj(v).S("id") == core.Obj(a).S("id") }) {
						cv.data["draft_attachments"] = append(cv.attachments(), a)
					}
				}
			} else {
				turn["state"], turn["error"] = "error", cv.err
			}
			cv.busy = false
		})
		cv.save()
		return
	}
	if onUIValue(func() bool { return cv.stopRequested }) {
		onUI(func() { turn["state"], cv.busy = "cancelled", false })
		cv.save()
		return
	}
	request := onUIValue(func() core.J {
		r := core.J{"lecture_id": cv.lecture(), "conversation_id": cv.id, "turn_id": turn["id"], "question": question,
			"selection": map[string]any(cv.query()), "attachments": sentAttachments, "context_scope": sentScope}
		if task != "" {
			r["task"] = task
		}
		images := len(sentAttachments) == 0 && cv.data.B("allow_images")
		r["allow_images"] = images
		visuals := []any{}
		if images {
			for _, v := range cv.visuals {
				visuals = append(visuals, map[string]any(v))
			}
		}
		r["visuals"] = visuals
		return r
	})
	out, err := State.Api().StartCanvasRun(request)
	if err != nil {
		onUI(func() {
			turn["state"], turn["error"] = "error", err.Error()
			cv.busy, cv.err = false, err.Error()
		})
		cv.save()
		return
	}
	runID := core.Obj(out).S("id")
	// The message id is stable; the job has its own run id.
	stop := onUIValue(func() bool {
		cv.runID = runID
		turn["state"] = "running"
		if cv.disposed || cv.stopRequested {
			if cv.disposed {
				turn["state"] = "interrupted"
			} else {
				turn["state"] = "cancelled"
			}
			return true
		}
		return false
	})
	if stop {
		_, _ = State.Api().CancelCanvasRun(runID)
		cv.save()
		onUI(func() { cv.busy = false })
		return
	}
	cv.save()
	cv.poll(turn)
}

// restoreVisuals rebuilds the approved crops that went missing, before a
// question that may look at them.
func (cv *studyConversation) restoreVisuals() error {
	type need struct {
		source core.J
		turns  int
		size   *study.Pt
	}
	var needs []need
	var problem error
	onUI(func() {
		if len(cv.visuals) > 2 {
			problem = formatError(l10n.StudySelectionImageLimit)
			return
		}
		count := len(cv.visuals)
		for _, source := range cv.preview.Os("sources") {
			id := source.S("id")
			if cv.visuals[id] != nil && cv.fingerprints[id] != cv.fingerprint(source) {
				// Only the card turned: the approved region is unchanged,
				// so the crop is rebuilt with the new rotation.
				if cv.rotatedOnly(id, source) && len(source.O("anchor").L("region")) > 0 {
					delete(cv.visuals, id)
					delete(cv.fingerprints, id)
					count--
				} else {
					problem = formatError(l10n.StudyVisualChanged)
					return
				}
			}
			if source.S("kind") == "ink" && cv.visuals[id] == nil {
				problem = formatError(l10n.StudyVisualExpired)
				return
			}
			if cv.visuals[id] == nil && len(source.O("anchor").L("region")) > 0 {
				if count >= 2 {
					problem = formatError(l10n.StudySelectionImageLimit)
					return
				}
				count++
				n := need{source: source}
				if card := cv.s.doc.Card(source.O("anchor").S("object_id")); card != nil {
					n.turns = card.QuarterTurns
					if card.Type == "image" || card.Type == "pdf" {
						n.size = &study.Pt{X: card.Rect.W(), Y: card.Rect.H()}
					}
				}
				needs = append(needs, n)
			}
		}
	})
	if problem != nil {
		return problem
	}
	for _, n := range needs {
		restored, err := restoreStudyVisual(n.source, n.turns, n.size)
		if err != nil {
			return err
		}
		if onUIValue(func() bool { return cv.disposed || cv.stopRequested }) {
			return errGone
		}
		onUI(func() {
			id := n.source.S("id")
			cv.visuals[id] = restored
			cv.fingerprints[id] = cv.fingerprint(n.source)
		})
	}
	return nil
}

func (cv *studyConversation) stop() {
	cv.stopRequested = true
	id := cv.runID
	if id == "" {
		return
	}
	go func() {
		if _, err := State.Api().CancelCanvasRun(id); err != nil {
			onUI(func() { cv.err = err.Error() })
		}
	}()
}

func (cv *studyConversation) poll(turn core.J) {
	api := State.Api()
	for {
		id := onUIValue(func() string { return cv.runID })
		out, err := api.CanvasRun(id)
		if err != nil {
			onUI(func() {
				turn["state"], turn["error"] = "error", err.Error()
				cv.busy = false
			})
			cv.save()
			return
		}
		run := core.Obj(plainJSON(out))
		if cv.gone() {
			return
		}
		switch run.S("state") {
		case "done":
			outline := false
			onUI(func() {
				cv.live = run
				turn["answer"], turn["context_key"], turn["state"] = run["answer"], run["context_key"], "done"
				// Keep only answer provenance. Older hosts' operations never
				// run here.
				sources := run["sources"]
				if sources == nil {
					sources = []any{}
				}
				result := core.J{"sources": sources}
				if run.I("schema_version") == 2 {
					result["schema_version"] = 2
					result["evidence"] = nonNilList(run.L("evidence"))
					result["suggestions"] = nonNilList(run.L("suggestions"))
					if core.Obj(run["outline_proposal"]) != nil {
						result["outline_proposal"] = run["outline_proposal"]
					}
				}
				turn["result"] = map[string]any(result)
				outline = study.OutlineProposal(plainMap(turn)) != nil
			})
			cv.save()
			onUI(func() {
				cv.busy = false
				cv.view.pulse()
				// A finished draft opens for review; nothing is written until
				// then.
				if outline && !cv.disposed {
					cv.s.openReview(cv.cardID, turn.S("id"))
				}
			})
			return
		case "error", "cancelled":
			onUI(func() {
				cv.live = run
				turn["state"], turn["error"] = run["state"], run["error"]
				cv.busy = false
			})
			cv.save()
			return
		}
		onUI(func() { cv.live = run })
		time.Sleep(700 * time.Millisecond)
	}
}

// dispose leaves a run that is under way interrupted, as closing the
// canvas does.
func (cv *studyConversation) dispose() {
	if cv.disposed {
		return
	}
	cv.saveTimer.stop()
	cv.stopRequested = true
	if cv.data != nil {
		for _, t := range cv.turns() {
			if st := t.S("state"); st == "preparing" || st == "running" {
				t["state"] = "interrupted"
			}
		}
	}
	if cv.busy && cv.runID != "" {
		id := cv.runID
		go func() { _, _ = State.Api().CancelCanvasRun(id) }()
	}
	if cv.data != nil {
		snapshot := core.Obj(study.Clone(plainJSON(cv.data)))
		snapshot["draft"], snapshot["version"] = cv.input, 2
		lecture, id := cv.lecture(), cv.id
		go func() { _, _ = State.Api().SaveStudyConversation(lecture, id, snapshot) }()
	}
	cv.disposed = true
}

// outline is 「整理为复习提纲」: the learner confirms which bound material is
// read and whether images may be, then it runs like a question.
func (cv *studyConversation) outline() {
	if cv.busy || cv.loading || cv.data == nil {
		return
	}
	sources := cv.preview.Os("sources")
	images := cv.data.B("allow_images")
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		cs, tt := design.S(), design.TT()
		m3.AlertDialog(c, m3.Alert{Title: l10n.OutlineConfirmTitle, Width: 420, Content: func() {
			ui.Column(c).FillWidth().Children(func() {
				m3.Text(c, l10n.OutlineConfirmBody, tt.BodyMedium.C(cs.OnSurfaceVariant))
				ui.Box(c).Height(design.SpaceMD)
				if len(sources) == 0 {
					m3.Text(c, l10n.OutlineNoSources, tt.BodySmall.C(cs.Error))
				} else {
					ui.Scroll(c).FillWidth().MaxHeight(220).Children(func() {
						ui.Row(c).Wrap().Gap(design.SpaceXS).Children(func() {
							for _, src := range sources {
								label := src.S("label")
								if label == "" {
									label = src.S("id")
								}
								m3.Chip{Label: truncRunes(label, 40), Avatar: sourceIcon(src), Compact: true}.Show(c)
							}
						})
					})
				}
				ui.Box(c).Height(design.SpaceSM)
				m3.SwitchTile(c, m3.Tile{Title: l10n.StudyAllowImages, Subtitle: l10n.OutlineImagesHint, Pad: true}, &images)
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Pop(false)
			}
			if (m3.Button{Kind: m3.Filled, Label: l10n.OutlineConfirmStart, Disabled: len(sources) == 0}).Show(c.Key("outline-confirm-start")).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v != true || cv.disposed {
			return
		}
		go func() {
			if onUIValue(func() bool {
				changed := images != cv.data.B("allow_images")
				cv.data["allow_images"] = images
				return changed && images
			}) {
				cv.captureBound()
			}
			cv.start(nil, l10n.OutlineAsk, "propose_outline")
		}()
	})
}

func (cv *studyConversation) editVideo(source core.J) {
	a := source.O("anchor")
	end := 60000
	if a.Has("time_end_ms") {
		end = a.I("time_end_ms")
	}
	from := strconv.FormatFloat(float64(a.I("time_start_ms"))/1000, 'f', 1, 64)
	to := strconv.FormatFloat(float64(end)/1000, 'f', 1, 64)
	invalid := ""
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: "指定视频区间 · 最多 180 秒", Width: 360, Content: func() {
			ui.Column(c).FillWidth().Gap(design.SpaceSM).Children(func() {
				m3.Field{Label: "开始时间（秒）"}.Show(c, &from)
				m3.Field{Label: "结束时间（秒）"}.Show(c, &to)
				if invalid != "" {
					m3.Text(c, invalid, design.TT().BodySmall.C(design.S().Error))
				}
			})
		}, Actions: func() {
			if m3.TextButton(c, "取消").Clicked() {
				r.Pop(nil)
			}
			if m3.FilledButton(c, "使用此区间").Clicked() {
				x, e1 := strconv.ParseFloat(strings.TrimSpace(from), 64)
				y, e2 := strconv.ParseFloat(strings.TrimSpace(to), 64)
				if e1 != nil || e2 != nil || math.IsInf(x, 0) || math.IsInf(y, 0) || x < 0 || y <= x || y-x > 180 {
					invalid = "请输入有效的 0–180 秒区间"
					return
				}
				r.Pop([2]int{int(math.Round(x * 1000)), int(math.Round(y * 1000))})
			}
		}})
	}, func(v any) {
		rng, ok := v.([2]int)
		if !ok || cv.disposed {
			return
		}
		q := cv.query()
		if source.S("id") == "lecture-video" {
			q["from_ms"], q["to_ms"] = rng[0], rng[1]
		} else {
			ranges := map[string]any{}
			for k, v := range q.O("video_ranges") {
				ranges[k] = v
			}
			ranges[a.S("object_id")] = []any{rng[0], rng[1]}
			q["video_ranges"] = ranges
		}
		cv.data["query"] = map[string]any(q)
		go func() {
			cv.save()
			cv.refreshContext()
		}()
	})
}

func (cv *studyConversation) inspectSources() {
	sources := cv.preview.Os("sources")
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: "本次实际可用的资料", Width: 650, Content: func() {
			ui.Scroll(c).FillWidth().Height(400).Selectable().Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					for _, src := range sources {
						m3.Text(c, src.S("label")+" · "+src.S("availability"), design.TT().BodyMedium.W(700).C(design.S().OnSurface))
						m3.Text(c, src.S("text"), design.TT().BodyMedium.C(design.S().OnSurface))
						m3.DividerIndent(c, 1, 0, 0)
					}
				})
			})
		}, Actions: func() {
			if m3.TextButton(c, "关闭").Clicked() {
				r.Pop(nil)
			}
		}})
	}, nil)
}

// pickSources is 「仅引用勾选的资料」.
func (cv *studyConversation) pickSources() {
	s := cv.s
	current := map[string]bool{}
	for _, id := range cv.query().Ss("objects") {
		current[id] = true
	}
	var cards []*study.Card
	for _, c := range s.doc.Cards {
		if c.Type != "conversation" && s.doc.Visible(c.Layer) {
			cards = append(cards, c)
		}
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: "仅引用勾选的资料", Width: 420, Content: func() {
			ui.Scroll(c).FillWidth().Height(330).Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					for _, card := range cards {
						on := current[card.ID]
						if m3.CheckTile(c, m3.Tile{Title: card.Title, Subtitle: card.Type}, &on) {
							current[card.ID] = on
						}
					}
				})
			})
		}, Actions: func() {
			if m3.TextButton(c, "取消").Clicked() {
				r.Pop(nil)
			}
			if m3.FilledButton(c, "使用这些资料").Clicked() {
				r.Pop(current)
			}
		}})
	}, func(v any) {
		chosen, ok := v.(map[string]bool)
		if !ok || cv.disposed {
			return
		}
		var objects []any
		times := map[string]any{}
		for _, card := range cards {
			if chosen[card.ID] && len(objects) < 12 {
				objects = append(objects, card.ID)
				if t, ok := s.videoTimes[card.ID]; ok {
					times[card.ID] = t
				}
			}
		}
		cv.data["query"] = map[string]any{"objects": nonNilList(objects), "selection": map[string]any{},
			"include_video": chosen["lecture"], "video_ms": s.ctl.PositionMs(), "object_video_ms": times}
		clear(cv.visuals)
		go func() {
			cv.save()
			cv.refreshContext()
		}()
	})
}

// studyVisualFingerprint is a source's visual identity: a scanned page's
// text hash can be empty on every page, so a crop cannot silently follow
// another page, selection or rotation.
func studyVisualFingerprint(source core.J, quarterTurns int) string {
	return study.Encode([]any{plainJSON(source["image_path"]), plainJSON(source["anchor"]), ((quarterTurns % 4) + 4) % 4})
}

func studyVisualBindingChanged(id string, before, after core.J) bool {
	if id == "selected-ink" {
		return study.Encode(plainJSON(before["selection"])) != study.Encode(plainJSON(after["selection"]))
	}
	object := strings.TrimPrefix(id, "object-")
	if id == "lecture-video" {
		object = "lecture"
	}
	if !slices.Contains(after.Ss("objects"), object) {
		return true
	}
	scope := func(q core.J) string {
		sel := q.O("selections")[object]
		if sel == nil {
			sel = q["selection"]
		}
		var own any
		if m := core.Obj(sel); m != nil && m.S("object_id") == object {
			own = plainJSON(sel)
		}
		return study.Encode([]any{own, plainJSON(q.O("object_pages")[object]), plainJSON(q.O("video_ranges")[object])})
	}
	return scope(before) != scope(after)
}

// restoreStudyVisual rebuilds a lost attachment from the approved source,
// independent of the viewport and the conversation's lifetime. It never
// substitutes the whole page.
func restoreStudyVisual(source core.J, quarterTurns int, cardSize *study.Pt) (core.J, error) {
	path := source.S("image_path")
	region := source.O("anchor").L("region")
	if path == "" || len(region) != 4 {
		return nil, formatError(l10n.StudyVisualExpired)
	}
	r := make([]float64, 4)
	for i, v := range region {
		f, ok := v.(float64)
		if !ok {
			return nil, formatError(l10n.StudyVisualRegionInvalid)
		}
		r[i] = f
	}
	for _, v := range r {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, formatError(l10n.StudyVisualRegionInvalid)
		}
	}
	if r[0] < 0 || r[1] < 0 || r[2] <= 0 || r[3] <= 0 || r[0]+r[2] > 1.000001 || r[1]+r[3] > 1.000001 {
		return nil, formatError(l10n.StudyVisualRegionInvalid)
	}
	data, err := State.Api().StudyImageBytes(path)
	if err != nil {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width*cfg.Height > 24000000 {
		return nil, formatError(l10n.StudyVisualDimensionsTooLarge)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	turns := ((quarterTurns % 4) + 4) % 4
	// Legacy PDF and image regions are normalized against the whole card,
	// whose title and contain-fit padding are no pixels of the asset.
	orig := study.Pt{X: iw, Y: ih}
	if cardSize != nil {
		orig = *cardSize
		if turns%2 == 1 {
			orig = study.Pt{X: cardSize.Y, Y: cardSize.X}
		}
	}
	area := study.RectLTWH(0, 0, orig.X, orig.Y)
	if cardSize != nil {
		area = study.RectLTWH(0, 36, orig.X, math.Max(1, orig.Y-36))
	}
	fit := math.Min(area.W()/iw, area.H()/ih)
	dest := study.RectCenter(area.Center(), iw*fit, ih*fit)
	width, height := orig.X, orig.Y
	if turns%2 == 1 {
		width, height = orig.Y, orig.X
	}
	crop := study.RectLTWH(r[0]*width, r[1]*height, r[2]*width, r[3]*height)
	for _, edge := range []float64{1200, 900, 600, 320} {
		scale := math.Min(1, edge/math.Max(crop.W(), crop.H()))
		w, h := max(1, int(math.Round(crop.W()*scale))), max(1, int(math.Round(crop.H()*scale)))
		out := image.NewRGBA(image.Rect(0, 0, w, h))
		for oy := range h {
			for ox := range w {
				X := crop.L + (float64(ox)+.5)*crop.W()/float64(w)
				Y := crop.T + (float64(oy)+.5)*crop.H()/float64(h)
				// Back from the card's clockwise turn to the upright asset.
				qx, qy := X, Y
				switch turns {
				case 1:
					qx, qy = Y, width-X
				case 2:
					qx, qy = width-X, height-Y
				case 3:
					qx, qy = height-Y, X
				}
				col := color.RGBA{255, 255, 255, 255}
				if dest.Contains(study.Pt{X: qx, Y: qy}) {
					col = sampleBilinear(img, (qx-dest.L)/dest.W()*iw, (qy-dest.T)/dest.H()*ih)
				}
				out.SetRGBA(ox, oy, col)
			}
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, out); err == nil && buf.Len() <= 450000 {
			return core.J{"source_id": source["id"], "mime_type": "image/png", "data": base64.StdEncoding.EncodeToString(buf.Bytes())}, nil
		}
	}
	return nil, formatError(l10n.StudyVisualPayloadTooLarge)
}

// sampleBilinear reads img at the continuous pixel position x, y, over
// white.
func sampleBilinear(img image.Image, x, y float64) color.RGBA {
	b := img.Bounds()
	x, y = x-.5, y-.5
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	at := func(px, py int) [4]float64 {
		px = min(max(px, 0), b.Dx()-1)
		py = min(max(py, 0), b.Dy()-1)
		r, g, bb, a := img.At(b.Min.X+px, b.Min.Y+py).RGBA()
		return [4]float64{float64(r), float64(g), float64(bb), float64(a)}
	}
	p00, p10, p01, p11 := at(x0, y0), at(x0+1, y0), at(x0, y0+1), at(x0+1, y0+1)
	var c [4]float64
	for i := range 4 {
		top := p00[i]*(1-fx) + p10[i]*fx
		bottom := p01[i]*(1-fx) + p11[i]*fx
		c[i] = top*(1-fy) + bottom*fy
	}
	// Premultiplied over white.
	a := c[3] / 65535
	ch := func(v float64) uint8 { return uint8(math.Round(math.Min(255, (v/65535+(1-a))*255))) }
	return color.RGBA{ch(c[0]), ch(c[1]), ch(c[2]), 255}
}

// sourceIcon is the icon of a kind of material a conversation reads.
func sourceIcon(source core.J) icons.Icon {
	switch source.S("kind") {
	case "video":
		return icons.SmartDisplayOutlined
	case "ink":
		return icons.GestureRounded
	case "image", "region", "visual":
		return icons.CropRounded
	case "text", "note":
		return icons.NotesRounded
	}
	return icons.DescriptionOutlined
}

// sourceDetail is the page or time range after a material's name.
func sourceDetail(source core.J) string {
	a := source.O("anchor")
	if a.Has("page") && a["page"] != nil && a.I("page") != 0 {
		return l10n.StudySourcePage(a.I("page"))
	}
	if source.S("kind") == "video" {
		return l10n.StudyVideoRange(int(math.Round(float64(a.I("time_start_ms"))/1000)), int(math.Round(float64(a.I("time_end_ms"))/1000)))
	}
	return ""
}

var _ = fmt.Sprint
