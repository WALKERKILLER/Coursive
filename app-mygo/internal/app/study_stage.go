package app

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// study_agent_stage.dart and the rest of study_conversation_ui.dart: the
// conversations live in a collapsible sidebar; only answers pinned
// explicitly become canvas cards. Contract: docs/multimodal-canvas.md,
// spec 0021.

const (
	sidebarRadius = design.CornerXL
	sidebarInset  = design.SpaceMD
	innerRadius   = sidebarRadius - sidebarInset
)

// conversationStart is how a conversation starts: bound to the selection
// or not, with its first question, a screenshot, or the source picker.
type conversationStart struct {
	send, task        string
	bind, allowImages bool
	pick              bool
	snapshot          *studySnapshot
	at                *study.Pt
	query             core.J
	draft             string
}

type reviewTarget struct{ card, turn string }

// stageParts is the conversation state of the canvas.
type stageParts struct {
	conversations        map[string]*studyConversation
	initialConversations map[string]*conversationStart
	activeConversation   string
	idleInput            string
	idleView             stageViewState
	focusComposer        bool
	review               *reviewTarget
	reviewPeek           bool
	snapshotFrames       map[string]*ui.Bitmap
}

// stageViewState is _StageViewState: what the sidebar of a conversation
// keeps between frames.
type stageViewState struct {
	list        ui.ListState
	hover       bool
	focusSource string
	trayOpen    bool
	// flowStart starts a pass of the light along the cited threads; the
	// flow loops while the agent works.
	flowStart time.Time
	toBottom  bool
	composer  ui.Rect
}

func newStageViewState() stageViewState {
	return stageViewState{list: ui.ListState{FollowEnd: true}}
}

// pulse sends one pass of light along the cited threads, after an answer.
func (v *stageViewState) pulse() { v.flowStart = time.Now() }

func (s *StudyCanvas) threads() []*study.Card {
	var out []*study.Card
	if s.doc == nil {
		return out
	}
	for _, c := range s.doc.Cards {
		if c.Type == "conversation" {
			out = append(out, c)
		}
	}
	return out
}

// adoptThreads keeps conversation identities and histories without moving
// saved objects.
func (s *StudyCanvas) adoptThreads(d *study.Document) {
	if d.Card(s.activeConversation) == nil {
		s.activeConversation = ""
		for _, c := range d.Cards {
			if c.Type == "conversation" {
				s.activeConversation = c.ID
			}
		}
	}
}

// selectedQuery is what the selection binds a conversation to.
func (s *StudyCanvas) selectedQuery() core.J {
	d := s.doc
	var ids []any
	idSet := map[string]bool{}
	consider := func(id string) {
		if c := d.Card(id); c != nil && c.Type != "conversation" && d.Visible(c.Layer) && !idSet[id] && len(ids) < 12 {
			idSet[id] = true
			ids = append(ids, id)
		}
	}
	// The selection's order follows the cards'.
	for _, c := range d.Cards {
		if s.selection[c.ID] {
			consider(c.ID)
		}
	}
	if s.detail.ObjectID != "" {
		consider(s.detail.ObjectID)
	}
	selections := map[string]any{}
	if s.detail.ObjectID != "" && len(s.detail.StrokeIDs) == 0 {
		selections[s.detail.ObjectID] = s.detail.JSON()
	}
	pages := map[string]any{}
	for _, c := range d.Cards {
		if idSet[c.ID] && c.Type == "pdfdoc" {
			pages[c.ID] = c.Page
		}
	}
	times := map[string]any{}
	for id := range idSet {
		if t, ok := s.videoTimes[id]; ok {
			times[id] = t
		}
	}
	return core.J{"revision": d.Revision, "objects": nonNilList(ids), "selection": s.detail.JSON(), "selections": selections,
		"object_pages": pages, "include_video": idSet["lecture"], "video_ms": s.ctl.PositionMs(), "object_video_ms": times}
}

func (s *StudyCanvas) activeStage() *studyConversation {
	if s.activeConversation == "" {
		return nil
	}
	return s.conversations[s.activeConversation]
}

// openConversation brings the agent forward. With newConversation a new
// conversation starts; otherwise the selection joins the one on stage, or
// starts one when none is there yet.
func (s *StudyCanvas) openConversation(newConversation, bind bool) {
	d := s.doc
	if d == nil || !d.Editable(d.ActiveLayer) {
		return
	}
	picked := bind && (len(s.selection) > 0 || s.detail.ObjectID != "" || s.regionSelection != nil)
	cv := s.activeStage()
	// With 「看图」 on, the region joins as one immutable screenshot; a new
	// conversation starts with it off, so nothing is captured.
	shot := picked && s.regionSelection != nil && !newConversation && cv != nil && cv.data.B("allow_images")
	go func() {
		var snap *studySnapshot
		if shot {
			var err error
			if snap, err = s.prepareSelectionSnapshotWait(); err != nil {
				onUI(func() { s.toast(err.Error()) })
				return
			}
		}
		var bound *studyConversation
		var query core.J
		onUI(func() {
			if s.disposed {
				return
			}
			cv := s.activeStage()
			switch {
			case newConversation || (cv == nil && picked):
				s.startConversation(conversationStart{bind: bind, snapshot: snap}, nil)
			case cv != nil && picked:
				bound, query = cv, s.selectedQuery()
			}
			s.tool = toolSelect
			s.stageOpen = true
			s.focusComposer = true
		})
		if bound != nil {
			bound.bind(query, nil)
			if snap != nil {
				bound.attachSnapshot(snap)
			}
		}
	}()
}

// chooseConversation is a card's 「加入对话」: it joins the conversation on
// stage.
func (s *StudyCanvas) chooseConversation(card *study.Card) {
	clear(s.selection)
	s.selection[card.ID] = true
	s.detail.Clear()
	s.regionSelection = nil
	s.selectionPolygon = nil
	s.openConversation(false, true)
}

// startConversation starts a conversation node bound to the selection; its
// first question goes once the node has loaded. done hears whether it
// started.
func (s *StudyCanvas) startConversation(o conversationStart, done func(error)) {
	d := s.doc
	if d == nil || !d.Editable(d.ActiveLayer) {
		if done != nil {
			done(formatError(l10n.StudyLocked))
		}
		return
	}
	query := core.J{"objects": []any{}, "selection": map[string]any{}, "include_video": false}
	if o.bind {
		query = s.selectedQuery()
	}
	finish := func(snap *studySnapshot) {
		if s.disposed || s.doc != d {
			return
		}
		rect := s.freeSpot(study.Pt{X: 300, Y: 200})
		if o.at != nil {
			rect = study.RectCenter(*o.at, 300, 200)
		}
		card := &study.Card{ID: study.NewID(), Type: "conversation", Layer: d.ActiveLayer, Rect: rect,
			Title: l10n.StudyThreadTitle(len(s.threads()) + 1), ConversationID: study.NewID(), Responses: map[string]string{}}
		d.Checkpoint()
		d.Cards = append(d.Cards, card)
		d.Changed()
		o.query, o.snapshot, o.draft = query, snap, s.idleInput
		o.send = strings.TrimSpace(o.send)
		s.initialConversations[card.ID] = &o
		s.idleInput = ""
		s.activeConversation = card.ID
		s.stageOpen = true
		s.mountConversation(card)
		if done != nil {
			done(nil)
		}
	}
	// With 「看图」 off no screenshot is taken at all; with it on the region
	// is one immutable attachment, exactly as shown.
	if o.snapshot != nil || !(o.bind && o.allowImages && s.regionSelection != nil) {
		finish(o.snapshot)
		return
	}
	core.Async(s.prepareSelectionSnapshotWait, func(snap *studySnapshot, err error) {
		if err != nil {
			if done != nil {
				done(err)
			} else {
				s.toast(err.Error())
			}
			return
		}
		finish(snap)
	})
}

// freeSpot is a place for a new node at the right of the view, clear of
// cards.
func (s *StudyCanvas) freeSpot(size study.Pt) study.Rect {
	d := s.doc
	view := s.visibleWorld()
	rect := study.RectLTWH(view.R-size.X-48/d.Zoom, view.T+96/d.Zoom, size.X, size.Y)
	for i := 0; i < 60; i++ {
		taken := false
		for _, c := range d.Cards {
			if c.Type != "conversation" && c.Rect.Inflate(16).Overlaps(rect) {
				taken = true
				break
			}
		}
		if !taken {
			break
		}
		rect = rect.Shift(study.Pt{Y: size.Y + 24})
	}
	return rect
}

// placeNear is a free place beside source (right, below, then left),
// clear of cards and ink; otherwise the next free spot in view.
func (s *StudyCanvas) placeNear(source *study.Rect, size study.Pt) study.Rect {
	d := s.doc
	if source == nil {
		return s.freeSpot(size)
	}
	free := func(r study.Rect) bool {
		for _, c := range d.Cards {
			if c.Type != "conversation" && d.Visible(c.Layer) && c.Rect.Inflate(16).Overlaps(r) {
				return false
			}
		}
		for _, st := range d.Strokes {
			if d.Visible(st.Layer) && st.Bounds().Overlaps(r) {
				return false
			}
		}
		return true
	}
	const gap = 24.0
	src := *source
	for _, start := range []study.Rect{
		study.RectLTWH(src.R+gap, src.T, size.X, size.Y),
		study.RectLTWH(src.L, src.B+gap, size.X, size.Y),
		study.RectLTWH(src.L-gap-size.X, src.T, size.X, size.Y),
	} {
		r := start
		for i := 0; i < 8; i++ {
			if free(r) {
				return r
			}
			r = r.Shift(study.Pt{Y: size.Y/2 + gap})
		}
	}
	return s.freeSpot(size)
}

// reveal brings rect into view at the current zoom when it is not.
func (s *StudyCanvas) reveal(rect study.Rect) {
	d := s.doc
	view := s.visibleWorld().Inflate(-64 / d.Zoom)
	if view.Contains(rect.TopLeft()) && view.Contains(rect.BottomRight()) {
		return
	}
	s.flyTo(s.viewport.Div(2).Sub(rect.Center().Mul(d.Zoom)), d.Zoom)
}

// pinnedCard is the card a turn was pinned as, found by its generation.
// Cards pinned before generations match by text only when that answer
// occurs once in turns.
func (s *StudyCanvas) pinnedCard(turn core.J, conversation string, turns []core.J) *study.Card {
	d := s.doc
	if d == nil {
		return nil
	}
	gen := studyPinGeneration(conversation, turn.S("id"))
	var linked *study.Card
	for _, c := range d.Cards {
		if c.Type == "text" && c.Generation == gen {
			linked = c
		}
	}
	if linked != nil || len(turns) == 0 {
		return linked
	}
	answer := strings.TrimSpace(turn.S("answer"))
	same := 0
	for _, t := range turns {
		if strings.TrimSpace(t.S("answer")) == answer {
			same++
		}
	}
	if answer == "" || same != 1 {
		return nil
	}
	plain := strings.TrimSpace(studyPlainAnswer(answer))
	var found *study.Card
	for _, c := range d.Cards {
		if c.Type == "text" && c.Generation == "" && (c.Text == answer || c.Text == plain) {
			found = c
		}
	}
	return found
}

// revealCard selects a pinned card and carries the view to it.
func (s *StudyCanvas) revealCard(card *study.Card) {
	clear(s.selection)
	s.selection[card.ID] = true
	s.stageOpen = false
	s.reveal(card.Rect)
}

// pinAnswer keeps an answer, or a reviewed outline's text, as an editable
// card linked to its turn and its sources.
func (s *StudyCanvas) pinAnswer(turn core.J, conversation, text, title string) {
	d := s.doc
	answer := text
	if answer == "" {
		answer = studyPlainAnswer(turn.S("answer"))
	}
	answer = strings.TrimSpace(answer)
	if d == nil || answer == "" || !d.Editable(d.ActiveLayer) {
		return
	}
	if pinned := s.pinnedCard(turn, conversation, nil); pinned != nil {
		s.revealCard(pinned)
		return
	}
	if len(d.Cards) >= 2000 {
		s.toast("画布卡片已达上限")
		return
	}
	// Course passages have no card here until followed; the pin keeps the
	// canvas sources it can lead back to.
	var raw []any
	if evidence := studyTurnEvidence(turn); evidence != nil {
		for _, e := range evidence {
			if e["course_source"] == nil {
				raw = append(raw, e["anchor"])
			}
		}
	} else {
		raw = turn.O("result").L("sources")
	}
	seen := map[string]bool{}
	var anchors []study.Anchor
	for _, a := range raw {
		if m := core.Obj(a); m != nil && len(anchors) < 24 {
			key := study.Encode(plainJSON(m))
			if !seen[key] {
				seen[key] = true
				anchors = append(anchors, anchorOf(m))
			}
		}
	}
	var source *study.Rect
	for _, c := range d.Cards {
		if slices.ContainsFunc(anchors, func(a study.Anchor) bool { return a.ObjectID == c.ID }) {
			source = rectPtr(c.Rect)
			break
		}
	}
	rect := s.placeNear(source, study.Pt{X: 400, Y: 300})
	if title == "" {
		title = turn.S("question")
		if title == "" {
			title = "AI 解答"
		}
	}
	d.Checkpoint()
	d.Cards = append(d.Cards, &study.Card{ID: study.NewID(), Type: "text", Layer: d.ActiveLayer, Rect: rect,
		Title: truncRunes(title, 60), Text: answer, Generation: studyPinGeneration(conversation, turn.S("id")),
		Sources: anchors, Responses: map[string]string{}})
	d.Changed()
	// Pinning is an explicit placement: the view goes to the card.
	s.reveal(rect)
	s.stageOpen = false
	s.toast("已留在画布，可移动、编辑或撤销")
}

func (s *StudyCanvas) activateThread(id string) {
	s.activeConversation = id
	s.stageOpen = true
	if id == "" {
		s.focusComposer = true
	}
}

// courseMaterials puts one of the course's library files on the canvas.
func (s *StudyCanvas) courseMaterials() {
	course := ""
	if s.ctl.Lecture != nil {
		course = s.ctl.Lecture.CourseID()
	}
	if course == "" {
		s.toast("课程信息尚未加载，请稍后重试")
		return
	}
	var items []core.J
	errText, filter := "", ""
	loading := true
	load := func() {
		loading, errText = true, ""
		core.Async(func() (any, error) { return State.Api().LibraryCatalog(course) }, func(out any, err error) {
			loading = false
			if err != nil {
				errText = err.Error()
				return
			}
			items = core.Objs(out)
		})
	}
	load()
	supported := map[string]bool{"pdf": true, "png": true, "jpg": true, "jpeg": true, "webp": true, "txt": true, "md": true,
		"mp4": true, "mov": true, "mkv": true, "webm": true}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		cs, tt := design.S(), design.TT()
		m3.AlertDialog(c, m3.Alert{Title: "本课程全部资料", Width: 680, Content: func() {
			ui.Column(c).FillWidth().Height(460).Children(func() {
				m3.Field{Prefix: icons.Search, Hint: "搜索文件名 · 按课次分组"}.Show(c, &filter)
				body := ui.Box(c).FillWidth().Grow(1).Basis(0)
				body.Children(func() {
					switch {
					case errText != "":
						ui.Column(c).Fill().Justify(ui.Center).AlignItems(ui.Center).Children(func() {
							m3.Text(c, errText, tt.BodyMedium.C(cs.OnSurface))
							if m3.TextButton(c, "重试").Clicked() {
								load()
							}
						})
					case loading:
						ui.Box(c).Fill().Center().Children(func() { m3.Circular(c, m3.Progress{}) })
					case len(items) == 0:
						ui.Box(c).Fill().Center().Children(func() {
							m3.Text(c, "这门课程还没有本地资料。可先在资料库下载，或通过“文件”导入。", tt.BodyMedium.C(cs.OnSurfaceVariant))
						})
					default:
						var order []string
						groups := map[string][]core.J{}
						for _, it := range items {
							if !strings.Contains(strings.ToLower(it.S("name")), strings.ToLower(filter)) {
								continue
							}
							g := it.Or("lecture", "课程公共资料")
							if _, ok := groups[g]; !ok {
								order = append(order, g)
							}
							groups[g] = append(groups[g], it)
						}
						ui.Scroll(c).Fill().Children(func() {
							ui.Column(c).FillWidth().Children(func() {
								for _, g := range order {
									m3.Text(c, g, tt.BodyMedium.W(700).C(cs.OnSurface)).Margin(18, 0, 4, 0)
									for _, it := range groups[g] {
										sub := it.S("file_type")
										if it.S("lecture_id") == s.lectureID() {
											sub += "  · 当前课次"
										}
										path := it.S("path")
										m3.Tile{Title: it.S("name"), Subtitle: sub, Dense: true, Trailing: func() {
											if supported[it.S("file_type")] {
												if m3.TextButton(c, "放入画布").Clicked() {
													r.Pop(path)
												}
											} else {
												m3.Text(c, "暂不支持此格式", design.TextStyle{Size: 11, Color: cs.OnSurfaceVariant})
											}
										}}.Show(c)
									}
								}
							})
						})
					}
				})
			})
		}, Actions: func() {
			if m3.TextButton(c, "关闭").Clicked() {
				r.Pop(nil)
			}
		}})
	}, func(v any) {
		if path, ok := v.(string); ok && !s.disposed {
			s.importMaterial(path, true, nil)
		}
	})
}

// videoModules is 「视频模块」.
func (s *StudyCanvas) videoModules() {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.Dialog(c, func() {
			m3.Text(c, "视频模块", design.TT().HeadlineSmall.C(design.S().OnSurface)).Margin(24, 24, 12, 24)
			for _, o := range []struct{ value, label string }{{"lecture", "定位当前课程视频"}, {"import", "导入本地视频"}, {"course", "从课程资料选择视频"}} {
				e := ui.ButtonBase(c).FillWidth().Padding(8, 24).Cursor(ui.CursorPointer)
				m3.Ink(e, design.S().OnSurface, 0)
				e.Children(func() { m3.Text(c, o.label, design.TT().BodyLarge.C(design.S().OnSurface)) })
				if e.Clicked() {
					r.Pop(o.value)
				}
			}
			ui.Box(c).Height(16)
		})
	}, func(v any) {
		if s.disposed {
			return
		}
		switch v {
		case "import":
			s.importMaterial("", false, nil)
		case "course":
			s.courseMaterials()
		case "lecture":
			for _, card := range s.doc.Cards {
				if card.Type == "lecture" {
					s.doc.Camera = s.viewport.Div(2).Sub(card.Rect.Center().Mul(s.doc.Zoom))
					s.cameraChanged()
					break
				}
			}
		}
	})
}

// agentStage is the sidebar of the conversation on stage, or the idle
// stage that starts one.
func (s *StudyCanvas) agentStage(c *ui.Context) {
	threads := map[string]*study.Card{}
	for _, t := range s.threads() {
		threads[t.ID] = t
	}
	for id, cv := range s.conversations {
		if threads[id] == nil {
			cv.dispose()
			delete(s.conversations, id)
		}
	}
	if threads[s.activeConversation] == nil {
		s.activeConversation = ""
	}
	var cv *studyConversation
	if s.activeConversation != "" {
		cv = s.mountConversation(threads[s.activeConversation])
	}
	if !s.stageOpen {
		return
	}
	s.stageView(c, cv, threads[s.activeConversation])
}

// stageProps are _StageView's arguments, for the conversation on stage or
// the idle stage.
type stageProps struct {
	cv       *studyConversation
	thread   *study.Card
	input    *string
	view     *stageViewState
	turns    []core.J
	sources  []core.J
	busy     bool
	loading  bool
	live     core.J
	err      string
	ready    bool
	allowImg *bool
}

func (s *StudyCanvas) stageView(c *ui.Context, cv *studyConversation, thread *study.Card) {
	p := stageProps{cv: cv, thread: thread, input: &s.idleInput, view: &s.idleView}
	if cv != nil {
		p.input, p.view = &cv.input, &cv.view
		p.ready = cv.data != nil
		if p.ready {
			p.turns = cv.turns()
		}
		p.sources = cv.preview.Os("sources")
		p.busy, p.loading, p.live = cv.busy, cv.loading || (!p.ready && cv.err == ""), cv.live
		p.err = cv.err
		if p.err == "" {
			p.err = cv.notice
		}
		if p.ready && len(cv.attachments()) == 0 && len(cv.pending) == 0 {
			show := len(cv.visuals) > 0 || cv.data.B("allow_images")
			for _, src := range p.sources {
				switch src.S("kind") {
				case "pdf", "pdfdoc", "image", "ink", "video":
					show = true
				}
				if len(src.O("anchor").L("region")) > 0 {
					show = true
				}
			}
			if show {
				v := cv.data.B("allow_images")
				p.allowImg = &v
			}
		}
	}
	vp := s.viewport
	width := math.Min(420, math.Max(240, vp.X-16))
	top := math.Max(76, s.topBarsBottom()+8)
	left := s.aiPlacement.edge == dockLeft
	if p.busy {
		c.AnimationFrame()
	}
	cited, bound := map[string]bool{}, map[string]bool{}
	if n := len(p.turns); n > 0 {
		for _, src := range p.turns[n-1].O("result").Os("sources") {
			if id := src.S("object_id"); id != "" {
				cited[id] = true
			}
		}
	}
	for _, src := range p.sources {
		if id := src.O("anchor").S("object_id"); id != "" {
			bound[id] = true
		}
	}
	reading := readingNow(p.live, p.sources)
	view := p.view
	ui.Box(c.Key("study-stage-flow")).Absolute().Top(0).Left(0).Right(0).Bottom(0).IgnoreInput(true).Draw(func(pt *ui.Painter, r ui.Rect) {
		s.paintFlow(pt, r, view, bound, reading, cited, p.busy)
	})
	from := float32(6)
	if left {
		from = -6
	}
	panel := ui.Column(c.Key("study-stage-panel")).Absolute().Top(f32(top)).Bottom(80).Width(f32(width)).
		Padding(sidebarInset).AlignItems(ui.Stretch).
		Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.EmphasizedDecelerate, ExitDuration: design.Short4,
			ExitEase: design.EmphasizedAccelerate, Enter: &ui.Motion{Opacity: 0, XPercent: from}, Exit: &ui.Motion{Opacity: 0, XPercent: from}, Shadows: true})
	if left {
		panel = panel.Left(8)
	} else {
		panel = panel.Right(8)
	}
	widgets.Glass{Radius: sidebarRadius, Depth: design.GlassPanel}.Apply(panel)
	s.recordControl(panel)
	panel.Children(func() {
		s.stageHeader(c, p)
		ui.Box(c).FillWidth().Grow(1).Basis(0).Children(func() {
			if len(p.turns) == 0 {
				ui.Scroll(c).Fill().Children(func() { s.stageWelcome(c, p) })
			} else {
				s.stageConversation(c, p)
			}
		})
		ui.Box(c).Height(design.SpaceSM)
		ui.Scroll(c).FillWidth().MaxHeight(f32(math.Max(100, vp.Y*.4))).Children(func() { s.composerPane(c, p) })
	})
}

func (s *StudyCanvas) stageHeader(c *ui.Context, p stageProps) {
	cs, tt := design.S(), design.TT()
	threads := s.threads()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, 0, design.SpaceSM, 0).Children(func() {
		m3.Menu(c, "study-thread-menu", m3.Below, func(t ui.Element, open bool) {
			t.Radius(design.CornerFull).Background(cs.OnSurface.Alpha(.06)).Padding(design.SpaceSM, design.SpaceSM, design.SpaceSM, design.SpaceMD).
				AlignItems(ui.Center).Cursor(ui.CursorPointer)
			m3.Ink(t, cs.OnSurface, design.CornerFull)
			t.Children(func() {
				title := l10n.StudyThreadNew
				if p.thread != nil {
					title = p.thread.Title
				}
				m3.Text(c, title, tt.LabelLarge.C(cs.OnSurface)).MaxLines(1).MaxWidth(220)
				ui.Box(c).Width(design.SpaceXXS)
				m3.Icon(c, icons.UnfoldMoreRounded, 16, cs.OnSurfaceVariant)
			})
		}, func() []m3.MenuItem {
			var items []m3.MenuItem
			for _, t := range threads {
				ic := icons.ForumOutlined
				if p.thread != nil && t.ID == p.thread.ID {
					ic = icons.CheckRounded
				}
				id := t.ID
				items = append(items, m3.MenuItem{Icon: ic, IconSize: 18, Label: t.Title, OnTap: func() { s.activateThread(id) }})
			}
			if len(threads) > 0 {
				items = append(items, m3.MenuItem{Divider: true})
			}
			items = append(items, m3.MenuItem{Icon: icons.AddCommentOutlined, IconSize: 18, Label: l10n.StudyThreadNew, OnTap: func() { s.activateThread("") }})
			return items
		})
		ui.Spacer(c)
		// Points the way the panel leaves.
		ic := icons.ChevronRightRounded
		if s.aiPlacement.edge == dockLeft {
			ic = icons.ChevronLeftRounded
		}
		if (m3.IconButton{Icon: ic, Tooltip: "收起 AI 对话"}).Show(c.Key("study-ai-close")).Clicked() {
			s.stageOpen = false
		}
	})
}

// well is _Well: a tonal surface inside the glass pane, which it does not
// blur again.
func well(c *ui.Context, key string, radius float32, color ui.Color, outline *ui.Color) ui.Element {
	e := ui.Column(c.Key(key)).Radius(radius).Background(color).Clip()
	if outline != nil {
		e = e.Border(1.5, *outline)
	}
	return e
}

// wellOf is a reading surface inside the sidebar: lighter than the frost in
// light mode, raised from it in dark mode; strong is the composer's.
func wellOf(strong bool) ui.Color {
	cs := design.S()
	if design.T().Dark {
		if strong {
			return cs.SurfaceBright.Alpha(.55)
		}
		return cs.SurfaceBright.Alpha(.38)
	}
	if strong {
		return cs.SurfaceContainerLowest.Alpha(.94)
	}
	return cs.SurfaceContainerLowest.Alpha(.78)
}

func (s *StudyCanvas) stageWelcome(c *ui.Context, p stageProps) {
	cs, tt := design.S(), design.TT()
	well(c, "study-stage-welcome", innerRadius, wellOf(false), nil).FillWidth().Padding(design.SpaceLG).Children(func() {
		m3.Text(c, l10n.StudyStageTitle, tt.TitleMedium.C(cs.OnSurface))
		ui.Box(c).Height(design.SpaceXS)
		m3.Text(c, l10n.StudyStageBody, tt.BodyMedium.C(cs.OnSurfaceVariant))
		ui.Box(c).Height(design.SpaceMD)
		ui.Row(c).Wrap().Gap(design.SpaceSM).Children(func() {
			for _, idea := range []string{l10n.StudyIdeaSummary, l10n.StudyIdeaExplain, l10n.StudyIdeaQuiz} {
				if (m3.Chip{Kind: m3.ActionChip, Label: idea}).Show(c).Clicked() {
					*p.input = idea
					s.focusComposer = true
				}
			}
		})
	})
}

func (s *StudyCanvas) stageConversation(c *ui.Context, p stageProps) {
	view := p.view
	if view.toBottom {
		view.toBottom = false
		view.list.ScrollToEnd()
	}
	turns := p.turns
	view.list.Key = func(i int) any { return turns[i].S("id") }
	ui.List(c, &view.list, len(turns), func(i int) {
		s.stageTurn(c, p, turns[i], i == len(turns)-1)
	}).Fill().Padding(design.SpaceXL, 0, 0, 0)
}

func (s *StudyCanvas) stageTurn(c *ui.Context, p stageProps, raw core.J, newest bool) {
	cs, tt := design.S(), design.TT()
	answer := raw.S("answer")
	st := raw.S("state")
	failed := raw["error"] != nil || st == "interrupted" || st == "error" || st == "cancelled"
	working := newest && p.busy && answer == "" && !failed
	sources := raw.O("result").Os("sources")
	evidence := studyTurnEvidence(raw)
	conversation := ""
	if p.thread != nil {
		conversation = p.thread.ConversationID
	}
	pin := func() {
		if st != "done" {
			return
		}
		pinned := s.pinnedCard(raw, conversation, p.turns)
		ui.Box(c.Key("study-pin-answer-" + raw.S("id"))).Children(func() {
			ui.Box(c.Key(pinned == nil)).
				Transition(design.Switcher(design.Short4, design.EmphasizedDecelerate, design.EmphasizedAccelerate, ui.Motion{Opacity: 0, Scale: .92})).
				Children(func() {
					if pinned == nil {
						if (m3.Button{Kind: m3.TextKind, Icon: icons.PushPinOutlined, Label: "留在画布"}).Show(c.Key("pin")).Clicked() {
							s.pinAnswer(raw, conversation, "", "")
						}
					} else if (m3.Button{Kind: m3.TextKind, Icon: icons.CheckRounded, Label: "已在画布 · 查看"}).Show(c.Key("pinned")).Clicked() {
						s.revealCard(pinned)
					}
				})
		})
	}
	maxW := float32(460)
	if s.viewport.X < 600 {
		maxW = 280
	}
	ui.Column(c).FillWidth().Padding(0, 0, design.SpaceMD, 0).AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
			well(c, "q", innerRadius, cs.PrimaryContainer, nil).MaxWidth(maxW).Padding(design.SpaceMD, design.SpaceLG).Children(func() {
				ui.Scroll(c).Selectable().Children(func() {
					m3.Text(c, raw.S("question"), tt.BodyMedium.C(cs.OnPrimaryContainer))
				})
			})
		})
		if atts := raw.Os("attachments"); len(atts) > 0 {
			ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(design.SpaceXS).Padding(design.SpaceXS, 0, 0, 0).Children(func() {
				for _, a := range atts {
					s.shot(c, "study-turn-shot-"+a.S("id"), State.Api().StudyAttachmentURL(s.lectureID(), a.S("id")), nil, false, false, nil, nil)
				}
			})
		}
		ui.Box(c).Height(design.SpaceSM)
		switch {
		case working:
			ui.Row(c).Children(func() {
				well(c, "working", design.CornerFull, wellOf(false), nil).Padding(design.SpaceSM, design.SpaceMD).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						readingOrb(c, 10)
						ui.Box(c).Width(design.SpaceSM)
						m3.Text(c, stepLabel(p.live, p.sources), tt.LabelLarge.C(readingShimmerColor(true, cs.OnSurfaceVariant))).MaxLines(1).Shrink(1)
					})
				})
			})
		case study.OutlineProposal(plainMap(raw)) != nil && st == "done":
			s.outlineCard(c, raw, p.thread)
		case answer != "":
			well(c, "a", innerRadius, wellOf(false), nil).FillWidth().Padding(design.SpaceMD, design.SpaceLG, design.SpaceSM, design.SpaceLG).Children(func() {
				md := CoursiveMarkdown{Data: answer, LectureID: s.lectureID()}
				if evidence != nil {
					byID := map[string]core.J{}
					for _, e := range evidence {
						byID[e.S("id")] = e
					}
					md.Data = studyEvidenceMarkdown(answer, evidence)
					md.OnEvidence = func(id string) {
						if e := byID[id]; e != nil {
							s.followEvidence(e)
						}
					}
					md.Stale = func(id string) bool {
						e := byID[id]
						return e != nil && s.doc != nil && s.evidenceOf(e).problem != ""
					}
				}
				ui.Scroll(c).FillWidth().Selectable().Children(func() { md.Show(c) })
				if evidence != nil && st == "done" {
					ui.Box(c).Height(design.SpaceSM)
					s.provenance(c, p, evidence, raw.O("result").L("suggestions"), pin)
				} else if st == "done" {
					ui.Row(c).FillWidth().Justify(ui.End).Children(pin)
				}
				if evidence == nil && len(sources) > 0 {
					ui.Box(c).Height(design.SpaceSM)
					ui.Row(c).Wrap().Gap(design.SpaceXS).Children(func() {
						for _, src := range sources {
							label := src.S("label")
							if label == "" {
								label = l10n.StudySourceShow
							}
							chip := m3.Chip{Kind: m3.ActionChip, Label: truncRunes(label, 30), Avatar: icons.MyLocationRounded, Compact: true}.Show(c)
							if chip.Hovered() {
								p.view.focusSource = src.S("object_id")
							}
							if chip.Clicked() {
								s.focusSource(anchorOf(src))
							}
						}
					})
				}
			})
		}
		if st == "pending" {
			m3.Text(c, l10n.StudyLegacyOperations, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(design.SpaceXS, 0, 0, 0)
		}
		if failed {
			well(c, "failed", innerRadius, cs.ErrorContainer, nil).FillWidth().Padding(design.SpaceXS, design.SpaceXS, design.SpaceXS, design.SpaceLG).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					m3.Icon(c, icons.ErrorOutlineRounded, 18, cs.OnErrorContainer)
					ui.Box(c).Width(design.SpaceSM)
					msg := raw.S("error")
					if raw["error"] == nil {
						msg = l10n.StudyTurnInterrupted
					}
					m3.Text(c, msg, tt.BodySmall.C(cs.OnErrorContainer)).Grow(1).Basis(0)
					if (m3.Button{Kind: m3.TextKind, Label: l10n.StudyTurnRetry, Disabled: p.busy, FG: cs.OnErrorContainer}).Show(c).Clicked() && p.cv != nil {
						if raw.S("task") == "propose_outline" {
							p.cv.outline()
						} else {
							go p.cv.start(raw, "", "")
						}
					}
				})
			})
		}
	})
}

// provenance is _Provenance: how much of an answer rests on material, and
// the next steps it suggests.
func (s *StudyCanvas) provenance(c *ui.Context, p stageProps, evidence []core.J, suggestions []any, pin func()) {
	cs, tt := design.S(), design.TT()
	count := func(kind string) int {
		n := 0
		for _, e := range evidence {
			if e.S("kind") == kind {
				n++
			}
		}
		return n
	}
	stale := len(s.staleEvidence(evidence))
	var parts []string
	if n := count("exact_text"); n > 0 {
		parts = append(parts, l10n.StudyCoveragePassages(n))
	}
	if n := count("timestamped_transcript"); n > 0 {
		parts = append(parts, l10n.StudyCoverageTranscript(n))
	}
	if n := count("visual_observation"); n > 0 {
		parts = append(parts, l10n.StudyCoverageVisual(n))
	}
	line := l10n.StudyCoverageNone
	if len(parts) > 0 {
		line = l10n.StudyCoverageLead(strings.Join(parts, " · "))
		if stale > 0 {
			line += " · " + l10n.StudyCoverageStale(stale)
		}
	}
	col := cs.OnSurfaceVariant
	if stale > 0 {
		col = cs.Error
	}
	byID := map[string]core.J{}
	for _, e := range evidence {
		byID[e.S("id")] = e
	}
	ui.Column(c).FillWidth().Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			m3.Text(c, line, tt.LabelSmall.C(col)).Grow(1).Basis(0).Key("study-coverage")
			pin()
		})
		var next []func()
		for _, raw := range suggestions {
			sg := core.Obj(raw)
			switch {
			case sg.S("kind") == "open_source" && byID[sg.S("evidence_id")] != nil:
				e := byID[sg.S("evidence_id")]
				next = append(next, func() {
					if (m3.Chip{Kind: m3.ActionChip, Label: l10n.StudySuggestOpen(s.evidenceLocator(e)), Avatar: icons.NorthEastRounded, Compact: true}).Show(c).Clicked() {
						s.followEvidence(e)
					}
				})
			case sg.S("kind") == "ask_followup" && sg.S("text") != "":
				text := sg.S("text")
				next = append(next, func() {
					if (m3.Chip{Kind: m3.ActionChip, Label: truncRunes(text, 40), Avatar: icons.SubdirectoryArrowRightRounded,
						Tooltip: l10n.StudySuggestFollowupHint, Compact: true}).Show(c).Clicked() {
						*p.input = text
						s.focusComposer = true
					}
				})
			case sg.S("kind") == "propose_outline" && p.cv != nil && !p.busy:
				next = append(next, func() {
					if (m3.Chip{Kind: m3.ActionChip, Label: l10n.OutlineAsk, Avatar: icons.FormatListBulletedRounded,
						Tooltip: l10n.OutlineAskHint, Compact: true}).Show(c.Key("study-suggest-outline")).Clicked() {
						p.cv.outline()
					}
				})
			}
		}
		if len(next) > 0 {
			ui.Box(c).Height(design.SpaceSM)
			ui.Row(c).FillWidth().Wrap().Gap(design.SpaceXS).Children(func() {
				for _, n := range next {
					n()
				}
			})
		}
	})
}

// stepLabel is what the agent is doing now, from the last step of its
// trace.
func stepLabel(live core.J, sources []core.J) string {
	trace := live.Os("trace")
	if len(trace) == 0 {
		return l10n.StudyAgentPreparing
	}
	step := trace[len(trace)-1]
	id := step.S("source_id")
	label := ""
	for _, src := range sources {
		if src.S("id") == id {
			label = src.S("label")
		}
	}
	switch step.S("tool") {
	case "search_course_materials":
		return l10n.StudyAgentSearchingCourse
	case "read_course_source":
		return l10n.StudyAgentReadingCourse
	case "inspect_snapshot":
		return l10n.StudyAgentLookingShot
	case "validate_citations":
		return l10n.StudyAgentCheckingQuotes
	}
	if label == "" {
		if step.S("tool") == "get_canvas_context" {
			return l10n.StudyAgentLookingAround
		}
		return l10n.StudyAgentThinking
	}
	switch step.S("tool") {
	case "get_video_context":
		return l10n.StudyAgentListening(label)
	case "prepare_video_clip":
		return l10n.StudyAgentTranscribing(label)
	case "inspect_canvas_region":
		return l10n.StudyAgentLooking(label)
	}
	return l10n.StudyAgentReading(label)
}

// readingNow are the cards the agent is reading right now.
func readingNow(live core.J, sources []core.J) map[string]bool {
	out := map[string]bool{}
	trace := live.Os("trace")
	if len(trace) == 0 || trace[len(trace)-1].S("status") != "running" {
		return out
	}
	id := trace[len(trace)-1].S("source_id")
	for _, src := range sources {
		if src.S("id") == id {
			if obj := src.O("anchor").S("object_id"); obj != "" {
				out[obj] = true
			}
		}
	}
	return out
}

// studyCardDrag is a card dragged by its 「加入对话」 button.
type studyCardDrag struct{ id string }

// studySelectionDrop is a screenshot of the selection dragged to a
// conversation; snapshot is prepared as the drag starts.
type studySelectionDrop struct {
	done    chan struct{}
	snap    *studySnapshot
	err     error
	started bool
}

func (s *StudyCanvas) cardDragSource(c *ui.Context, e ui.Element, card *study.Card) {
	e.Drag(studyCardDrag{card.ID})
}

func (s *StudyCanvas) composerPane(c *ui.Context, p stageProps) {
	cs, tt := design.S(), design.TT()
	cv := p.cv
	pane := well(c, "study-composer", innerRadius, wellOf(true), nil).FillWidth().Padding(design.SpaceSM)
	if _, over := ui.DragOver[studyCardDrag](pane); over && !p.loading {
		p.view.hover = true
	} else if _, over := ui.DragOver[*studySelectionDrop](pane); over && !p.loading {
		p.view.hover = true
	} else {
		p.view.hover = false
	}
	if p.view.hover {
		pane = pane.Border(1.5, cs.Primary)
	}
	if b := pane.Bounds(); b.W > 0 {
		p.view.composer = ui.Rect{X: b.X - f32(s.origin.X), Y: b.Y - f32(s.origin.Y), W: b.W, H: b.H}
	}
	if drop, ok := ui.Drop[studyCardDrag](pane); ok && !p.loading {
		s.dropOnComposer(cv, drop.id)
	}
	if drop, ok := ui.Drop[*studySelectionDrop](pane); ok && !p.loading {
		s.acceptSnapshot(drop)
	}
	pane.Children(func() {
		if cv != nil && (len(cv.attachments()) > 0 || len(cv.pending) > 0) {
			ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Padding(design.SpaceXS, design.SpaceXS, design.SpaceSM, design.SpaceXS).Children(func() {
				for _, raw := range cv.attachments() {
					a := core.Obj(raw)
					var remove func()
					if !p.busy {
						remove = func() {
							cv.data["draft_attachments"] = slices.DeleteFunc(slices.Clone(cv.attachments()), func(v any) bool { return core.Obj(v).S("id") == a.S("id") })
							go cv.save()
						}
					}
					s.shot(c, "study-shot-"+a.S("id"), State.Api().StudyAttachmentURL(s.lectureID(), a.S("id")), nil, false, false, remove, nil)
				}
				for _, snap := range cv.pending {
					var remove func()
					if !snap.uploading {
						remove = func() { cv.pending = slices.DeleteFunc(cv.pending, func(x *studySnapshot) bool { return x == snap }) }
					}
					s.shot(c, "", "", snap, snap.uploading, snap.err != nil, remove, func() { go cv.attachSnapshot(snap) })
				}
			})
		}
		scope := "selected"
		if cv != nil && cv.data != nil {
			scope = cv.data.S("context_scope")
		}
		canScope := cv != nil && cv.data != nil && !p.busy
		if len(p.sources) > 0 || p.loading || canScope {
			s.sourceTray(c, p, scope, canScope)
		}
		ui.Row(c).FillWidth().AlignItems(ui.End).Children(func() {
			if (m3.IconButton{Icon: icons.AddCircleOutlineRounded, Tooltip: l10n.StudyPickSources, Color: cs.OnSurfaceVariant,
				Disabled: p.busy || p.loading}).Show(c).Clicked() {
				if cv == nil {
					s.startConversation(conversationStart{bind: true, pick: true}, nil)
				} else if cv.data != nil {
					cv.pickSources()
				} else {
					go cv.load()
				}
			}
			var in ui.Element
			ui.Box(c).Grow(1).Basis(0).Children(func() {
				hint := l10n.StudyComposerHint
				if p.view.hover {
					hint = l10n.StudyComposerDrop
				}
				st := tt.BodyLarge
				in = m3.Field{Hint: hint, Borderless: true, MinLines: 1, MaxLines: 5, MaxLength: 4000, Pad: true, PadX: 0, PadY: design.SpaceMD,
					Style: &st}.Show(c.Key("study-composer-input"), p.input)
			})
			if s.focusComposer {
				s.focusComposer = false
				in.Focus()
			}
			if in.Changed() && cv != nil {
				cv.saveSoon()
			}
			// Enter sends, Shift+Enter breaks the line; Esc goes back from a
			// citation.
			in.HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind != ui.InputKeyDown {
					return false
				}
				switch {
				case ev.Key == ui.KeyEscape:
					return s.goBack()
				case ev.Key == ui.KeyEnter && ev.Mods == 0:
					core.Post(func() { s.sendStage(p) })
					return true
				}
				return false
			})
			ui.Box(c).Width(design.SpaceXS)
			enter := design.Switcher(design.Short4, nil, nil, ui.Motion{Scale: .25, Opacity: 0})
			if p.busy {
				ui.Box(c.Key("stop")).Transition(enter).Children(func() {
					if (m3.IconButton{Kind: m3.TonalIcon, Icon: icons.StopRounded, Tooltip: l10n.StudyStop}).Show(c).Clicked() && cv != nil {
						cv.stop()
					}
				})
			} else {
				ui.Box(c.Key("send")).Transition(enter).Children(func() {
					if (m3.IconButton{Kind: m3.FilledIcon, Icon: icons.ArrowUpwardRounded, Tooltip: l10n.StudySend, Disabled: p.loading}).Show(c).Clicked() {
						s.sendStage(p)
					}
				})
			}
		})
		if p.err != "" {
			m3.Text(c, p.err, tt.BodySmall.C(cs.Error)).MaxLines(3).Margin(0, design.SpaceMD, design.SpaceXS, design.SpaceMD)
		}
	})
}

func (s *StudyCanvas) sendStage(p stageProps) {
	text := strings.TrimSpace(*p.input)
	if text == "" || p.busy || p.loading {
		return
	}
	if p.cv == nil {
		s.startConversation(conversationStart{send: text, bind: true}, nil)
		return
	}
	go p.cv.start(nil, "", "")
}

// dropOnComposer binds a card dropped on the composer.
func (s *StudyCanvas) dropOnComposer(cv *studyConversation, id string) {
	if s.doc.Card(id) == nil {
		return
	}
	clear(s.selection)
	s.selection[id] = true
	s.detail.Clear()
	s.regionSelection = nil
	if cv == nil {
		s.startConversation(conversationStart{bind: true}, nil)
		return
	}
	if cv.data == nil {
		return
	}
	q := s.selectedQuery()
	go cv.bind(q, nil)
}

// sourceTray is _SourceTray: the bound materials in one line beside
// 「看图」 while they fit; past that the line folds into a count that opens
// a list.
func (s *StudyCanvas) sourceTray(c *ui.Context, p stageProps, scope string, canScope bool) {
	cs, tt := design.S(), design.TT()
	cv := p.cv
	reading := readingNow(p.live, p.sources)
	// Pills need about 120 DIPs each beside the chips; one never folds.
	folded := len(p.sources) > 1 && float64(len(p.sources))*130 > math.Min(420, math.Max(240, s.viewport.X-16))-200
	if !folded {
		p.view.trayOpen = false
	}
	ui.Column(c).FillWidth().Children(func() {
		ui.Row(c).FillWidth().Height(36).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).Basis(0).AlignItems(ui.Center).Padding(0, design.SpaceXS).Clip().Children(func() {
				if folded {
					s.traySummary(c, p, reading)
				} else {
					for _, src := range p.sources {
						s.sourcePill(c, p, src, reading, false)
					}
				}
				if p.loading {
					ui.Box(c).Size(16, 16).Margin(0, design.SpaceXS).Children(func() { m3.Circular(c, m3.Progress{Size: 16, Stroke: 2}) })
				}
			})
			if canScope || scope == "course" {
				if (m3.Chip{Kind: m3.FilterChip, Label: l10n.StudyCourseScopeChip, Avatar: icons.TravelExploreRounded, Selected: scope == "course",
					NoCheck: true, Compact: true, Tooltip: l10n.StudyCourseScopeHint, Disabled: !canScope}).Show(c.Key("study-course-scope")).Clicked() && canScope {
					if scope == "course" {
						cv.data["context_scope"] = "selected"
					} else {
						cv.data["context_scope"] = "course"
					}
					go cv.save()
				}
				ui.Box(c).Width(design.SpaceXS)
			}
			if p.allowImg != nil {
				if (m3.Chip{Kind: m3.FilterChip, Label: l10n.StudyAllowImages, Avatar: icons.VisibilityOutlined, Selected: *p.allowImg,
					NoCheck: true, Compact: true, Tooltip: l10n.StudyAllowImagesHint, Disabled: p.busy}).Show(c).Clicked() && !p.busy {
					v := !*p.allowImg
					cv.data["allow_images"] = v
					go func() {
						if v {
							cv.captureBound()
						}
						cv.save()
					}()
				}
			}
			if cv != nil && cv.preview != nil {
				if (m3.IconButton{Icon: icons.ManageSearchRounded, Size: 20, Box: 36, Tooltip: l10n.StudyInspectSources}).Show(c).Clicked() {
					cv.inspectSources()
				}
			}
		})
		if p.view.trayOpen && folded {
			ui.Scroll(c.Key("study-source-list")).FillWidth().MaxHeight(4.5*40).Margin(design.SpaceXS, 0, 0, 0).
				Background(cs.OnSurface.Alpha(.04)).Radius(design.CornerMD).Padding(design.SpaceXS, 0).
				Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Enter: &ui.Motion{Opacity: 0}, Shadows: true}).
				Children(func() {
					ui.Column(c).FillWidth().Children(func() {
						for _, src := range p.sources {
							s.sourcePill(c, p, src, reading, true)
						}
					})
				})
		}
		_ = tt
	})
}

func (s *StudyCanvas) traySummary(c *ui.Context, p stageProps, reading map[string]bool) {
	cs, tt := design.S(), design.TT()
	var readingSrc core.J
	for _, src := range p.sources {
		if reading[src.O("anchor").S("object_id")] {
			readingSrc = src
			break
		}
	}
	label := l10n.StudySourcesCount(len(p.sources))
	if readingSrc != nil {
		label = l10n.StudySourcesReading(readingSrc.S("label"))
		c.AnimationFrame()
	}
	tip := l10n.StudySourcesShow
	bg := cs.OnSurface.Alpha(.06)
	if p.view.trayOpen {
		tip, bg = l10n.StudySourcesHide, cs.SecondaryContainer
	}
	readingGlow(c, readingSrc != nil, design.CornerFull, func() {
		e := ui.ButtonBase(c.Key("study-source-summary")).Radius(design.CornerFull).Background(bg).AlignItems(ui.Center).
			Padding(design.SpaceXS, design.SpaceSM, design.SpaceXS, design.SpaceXS).Cursor(ui.CursorPointer)
		m3.Tip(c, e, tip)
		m3.Ink(e, cs.OnSurface, design.CornerFull)
		e.Children(func() {
			shown := p.sources[:min(3, len(p.sources))]
			// Overlapping discs, like a stack of cards seen edge on.
			ui.Box(c).Size(f32(float64(20+12*(len(shown)-1))), 22).Children(func() {
				for i, src := range shown {
					ui.Box(c).Absolute().Left(float32(12*i)).Top(0).Size(22, 22).Radius(11).Background(cs.SurfaceContainerHighest).
						Border(1.5, cs.Surface).Center().Children(func() { m3.Icon(c, sourceIcon(src), 12, cs.OnSurfaceVariant) })
				}
			})
			ui.Box(c).Width(design.SpaceSM)
			m3.Text(c, label, tt.LabelMedium.C(readingShimmerColor(readingSrc != nil, cs.OnSurface))).MaxLines(1).Shrink(1)
			ui.Box(c).Width(design.SpaceXXS)
			ic := icons.ExpandMoreRounded
			if p.view.trayOpen {
				ic = icons.KeyboardArrowUpRounded
			}
			m3.Icon(c, ic, 18, cs.OnSurfaceVariant)
		})
		if e.Clicked() {
			p.view.trayOpen = !p.view.trayOpen
		}
	})
}

// sourcePill is _SourcePill, or in the opened list _SourceRow: a bound
// material, tap to find it on the canvas.
func (s *StudyCanvas) sourcePill(c *ui.Context, p stageProps, src core.J, reading map[string]bool, row bool) {
	cs, tt := design.S(), design.TT()
	cv := p.cv
	id := src.O("anchor").S("object_id")
	isReading := reading[id]
	detail := sourceDetail(src)
	editable := cv != nil && !p.busy && !p.loading
	radius := float32(design.CornerFull)
	if row {
		radius = design.CornerSM
	}
	if isReading {
		c.AnimationFrame()
	}
	readingGlowHalo(c, isReading, radius, 4, func() {
		e := ui.ButtonBase(c.Key("study-source-" + src.S("id"))).Radius(radius).AlignItems(ui.Center).Height(36).Cursor(ui.CursorPointer)
		if row {
			e = e.FillWidth().Margin(design.SpaceXXS, design.SpaceXS)
		} else {
			bg := cs.OnSurface.Alpha(.06)
			if isReading {
				bg = cs.SurfaceContainerHighest
			}
			e = e.Background(bg).Margin(0, design.SpaceXS, 0, 0).Shrink(1)
			if !editable {
				e = e.PaddingEnd(design.SpaceMD)
			}
		}
		e = e.PaddingStart(design.SpaceSM + 2)
		m3.Ink(e, cs.OnSurface, radius)
		if e.Hovered() {
			p.view.focusSource = id
		} else if p.view.focusSource == id {
			p.view.focusSource = ""
		}
		e.Children(func() {
			fg := cs.OnSurfaceVariant
			if isReading {
				fg = cs.OnSurface
			}
			m3.Icon(c, sourceIcon(src), 16, fg)
			ui.Box(c).Width(design.SpaceXS + 2)
			t := m3.Text(c, src.S("label"), tt.LabelMedium.C(cs.OnSurface)).MaxLines(1)
			if row {
				t.Grow(1).Basis(0)
			} else {
				t.MaxWidth(160).Shrink(1)
			}
			m3.Tip(c, t, src.S("label"))
			if detail != "" {
				ui.Box(c).Width(design.SpaceXS)
				d := m3.Text(c, detail, tt.LabelSmall.C(cs.OnSurfaceVariant)).FontFeatures("tnum")
				if src.S("kind") == "video" && editable {
					d = d.Underline().Cursor(ui.CursorPointer)
					if d.Clicked() {
						cv.editVideo(src)
					}
				}
			}
			if isReading && row {
				ui.Box(c).Width(design.SpaceSM)
				readingOrb(c, 8)
			}
			if editable {
				if (m3.IconButton{Icon: icons.CloseRounded, Size: 14, Box: 28, Tooltip: l10n.StudyRemoveSource}).Show(c).Clicked() {
					go cv.removeSource(src)
				}
			} else if row {
				ui.Box(c).Width(design.SpaceSM)
			}
		})
		if e.Clicked() {
			s.focusSource(anchorOf(src["anchor"]))
		}
	})
}

// shot is _Shot: a screenshot in a message or the composer, opened full
// size on tap; in the composer it can be removed, an upload under way
// glows and a failed one retries on tap.
func (s *StudyCanvas) shot(c *ui.Context, key, url string, snap *studySnapshot, uploading, failed bool, remove, retry func()) {
	cs := design.S()
	k := c.Key(key)
	if key == "" {
		k = c.Key(snap)
	}
	ui.Box(k).Size(68, 50).Children(func() {
		readingGlow(c, uploading, design.CornerSM, func() {
			e := ui.ButtonBase(c).Size(68, 50).Radius(design.CornerSM).Background(cs.SurfaceContainerHighest).Clip().
				Border(1, cs.OutlineVariant).Cursor(ui.CursorPointer).Label(l10n.StudyShot)
			if failed {
				e = e.Border(1, cs.Error)
			}
			if uploading {
				c.AnimationFrame()
			}
			e.Children(func() {
				var bmp *ui.Bitmap
				if snap != nil {
					bmp = snap.bitmap()
				} else if b, bad := widgets.NetBitmap(url); bad {
					ui.Box(c).Fill().Center().Children(func() { m3.Icon(c, icons.BrokenImageOutlined, 18, cs.Outline) })
				} else {
					bmp = b
				}
				if bmp != nil {
					img := ui.Image(c, bmp).Fit(ui.Cover).Absolute().Top(0).Left(0).Right(0).Bottom(0)
					if uploading || failed {
						img.Opacity(.5)
					}
				}
				if failed {
					ic := ui.Box(c).Fill().Center()
					m3.Tip(c, ic, l10n.StudySnapshotRetry)
					ic.Children(func() { m3.Icon(c, icons.RefreshRounded, 20, cs.Error) })
				}
			})
			if e.Clicked() {
				switch {
				case failed && retry != nil:
					retry()
				case !uploading && snap != nil:
					viewShot(snap.bitmap())
				case !uploading && url != "":
					ShowImageViewer(url)
				}
			}
		})
		if remove != nil {
			x := ui.ButtonBase(c).Absolute().Top(-design.SpaceXS-2).Right(-design.SpaceXS-2).Size(18, 18).Radius(9).
				Background(cs.SurfaceContainerHighest).Border(1, cs.OutlineVariant).Center().Cursor(ui.CursorPointer)
			m3.Tip(c, x, l10n.StudySnapshotRemove)
			x.Children(func() { m3.Icon(c, icons.CloseRounded, 12, cs.OnSurface) })
			if x.Clicked() {
				remove()
			}
		}
	})
}

// viewShot opens a screenshot's exact pixels full size.
func viewShot(bmp *ui.Bitmap) {
	if bmp == nil {
		return
	}
	m3.ShowGeneral(true, func(c *ui.Context, r *m3.Route) {
		ui.Box(c).Fill().Background(ui.RGB(0, 0, 0).Alpha(.87)).Children(func() {
			ui.Image(c, bmp).Fit(ui.Contain).Absolute().Top(0).Left(0).Right(0).Bottom(0).Margin(design.SpaceXL)
			ui.Box(c).Absolute().Top(design.SpaceLG).Right(design.SpaceLG).Children(func() {
				if (m3.IconButton{Kind: m3.TonalIcon, Icon: icons.CloseRounded, Tooltip: l10n.CommonClose}).Show(c).Clicked() {
					r.Close()
				}
			})
		})
	}, nil)
}

// outlineCard is _OutlineCard: an outline draft in the conversation, how
// far its review went and the way into 「审核提纲」.
func (s *StudyCanvas) outlineCard(c *ui.Context, turn core.J, thread *study.Card) {
	cs, tt := design.S(), design.TT()
	prop := study.OutlineProposal(plainMap(turn))
	stats := study.CountOutline(prop, s.staleEvidence(studyTurnEvidence(turn)))
	sections := len(core.List(prop["sections"]))
	saved := core.Str(prop["state"]) == "accepted"
	open := s.review != nil && s.review.turn == turn.S("id")
	var outline *ui.Color
	if open {
		o := cs.Primary.Alpha(.5)
		outline = &o
	}
	well(c, "outline-"+turn.S("id"), innerRadius, wellOf(false), outline).FillWidth().Padding(design.SpaceMD, design.SpaceMD, design.SpaceMD, design.SpaceLG).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Start).Children(func() {
			m3.Icon(c, icons.FormatListBulletedRounded, 18, cs.Primary).Margin(2, 0, 0, 0)
			ui.Box(c).Width(design.SpaceSM)
			m3.Text(c, core.Str(prop["title"]), tt.TitleSmall.W(600).C(cs.OnSurface)).MaxLines(2).Grow(1).Basis(0)
			ui.Box(c).Width(design.SpaceSM)
			label, col := l10n.OutlineCardDraft, cs.OnSurfaceVariant
			if saved {
				label, col = l10n.OutlineCardSaved, cs.Primary
			}
			m3.Text(c, label, tt.LabelSmall.W(600).C(col))
		})
		ui.Box(c).Height(design.SpaceXS)
		parts := []string{l10n.OutlineCardSummary(sections, stats.Total), l10n.OutlineReviewed(stats.Reviewed, stats.Total)}
		if stats.Marks > 0 {
			parts = append(parts, l10n.OutlineMarksCount(stats.Marks))
		}
		m3.Text(c, strings.Join(parts, " · "), tt.LabelSmall.C(cs.OnSurfaceVariant))
		ui.Box(c).Height(design.SpaceSM)
		ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
			if (m3.Button{Kind: m3.Tonal, Icon: icons.RateReviewOutlined, Label: l10n.OutlineOpenReview, Disabled: thread == nil}).Show(c).
				Key("study-outline-review-"+turn.S("id")).Clicked() && thread != nil {
				s.openReview(thread.ID, turn.S("id"))
			}
		})
	})
}

// paintFlow is _FlowPainter: threads from bound cards to the composer. A
// thread at rest is a faint dotted line; the one being read flows toward
// the composer and its card wears the reading light; after an answer, a
// light travels the cited threads once.
func (s *StudyCanvas) paintFlow(p *ui.Painter, size ui.Rect, view *stageViewState, bound, reading, cited map[string]bool, busy bool) {
	d := s.doc
	target := view.composer
	if d == nil || target.W == 0 {
		return
	}
	ids := map[string]bool{}
	for id := range bound {
		ids[id] = true
	}
	if view.focusSource != "" {
		ids[view.focusSource] = true
	}
	var cards []*study.Card
	for _, c := range d.Cards {
		if ids[c.ID] && d.Visible(c.Layer) {
			cards = append(cards, c)
		}
	}
	if len(cards) == 0 {
		return
	}
	cs := design.S()
	flow := 0.0
	if busy && !design.T().ReduceMotion {
		flow = readingPhase(1400 * time.Millisecond)
		p.AnimationFrame()
	}
	pass := -1.0
	if !view.flowStart.IsZero() && !design.T().ReduceMotion {
		t := float64(time.Since(view.flowStart)) / float64(1400*time.Millisecond)
		if t < 1 {
			pass = t
			p.AnimationFrame()
		} else {
			view.flowStart = time.Time{}
		}
	}
	vw := ui.Rect{X: size.X - 80, Y: size.Y - 80, W: size.W + 160, H: size.H + 160}
	for i, c := range cards {
		q := s.screenRect(c.Rect)
		r := ui.Rect{X: size.X + f32(q.L), Y: size.Y + f32(q.T), W: f32(q.W()), H: f32(q.H())}
		if !uiOverlaps(r, vw) {
			continue
		}
		ex := size.X + target.X + target.W*float32(i+1)/float32(len(cards)+1)
		ey := size.Y + target.Y + 1
		var sx, sy float32
		if r.Y+r.H < ey-24 {
			sx, sy = min(max(ex, r.X+20), r.X+r.W-20), r.Y+r.H
		} else {
			sx = r.X
			if r.X+r.W < ex {
				sx = r.X + r.W
			}
			sy = min(max(r.Y+r.H/2, r.Y+20), r.Y+r.H-20)
		}
		lift := float32(math.Abs(float64(ey-sy)))*.5 + 40
		path := (&ui.Path{}).MoveTo(sx, sy).CubeTo(sx, sy+lift, ex, ey-lift, ex, ey).Cap(ui.RoundCap)
		lit := busy && reading[c.ID]
		active := lit || reading[c.ID] || c.ID == view.focusSource
		width, alpha := float32(1.25), float32(.32)
		dash, gap := float32(2), float32(5)
		if active {
			width, alpha, dash, gap = 2, .85, 8, 6
		}
		phase := float32(0)
		if active && busy {
			phase = float32(flow * 12)
		}
		path.Dash(-phase, dash, gap)
		if lit {
			p.StrokePathGradient(path, width, ui.LinearGradient{From: readingHues[0].Alpha(.9), To: readingHues[3].Alpha(.9)})
		} else {
			p.StrokePath(path, width, cs.Primary.Alpha(alpha))
		}
		dot := float32(3)
		dotA := float32(.5)
		if active {
			dot, dotA = 4, .9
		}
		p.FillPath((&ui.Path{}).Circle(sx, sy, dot), cs.Primary.Alpha(dotA))
		radius := f32(float64(design.CornerMD)*d.Zoom) + 3
		box := ui.Rect{X: r.X - 3, Y: r.Y - 3, W: r.W + 6, H: r.H + 6}
		if lit {
			// What is being read wears the turning light, breathing a
			// little as it turns.
			paintReadingLight(p, box, radius, flow, 14, .85+.15*math.Sin(flow*math.Pi*2))
		} else if active {
			glow := .5
			if busy {
				glow = .35 + .3*math.Abs(math.Sin(flow*math.Pi*2))
			}
			p.Shadow(ui.Rect{X: r.X - 4, Y: r.Y - 4, W: r.W + 8, H: r.H + 8}, radius+1, 0, 0, 8, 0, cs.Primary.Alpha(f32(glow)))
		}
		// After an answer: a light runs once from each cited card to the
		// chat.
		if pass >= 0 && cited[c.ID] {
			t := float32(easeInOut(pass))
			x, y := cubicAt(sx, sy, sx, sy+lift, ex, ey-lift, ex, ey, t)
			p.Shadow(ui.Rect{X: x - 5, Y: y - 5, W: 10, H: 10}, 5, 0, 0, 6, 0, cs.Primary)
			p.FillPath((&ui.Path{}).Circle(x, y, 5), cs.Primary)
		}
	}
}

func easeInOut(t float64) float64 {
	if t < .5 {
		return 2 * t * t
	}
	return 1 - math.Pow(-2*t+2, 2)/2
}

func cubicAt(x0, y0, x1, y1, x2, y2, x3, y3, t float32) (float32, float32) {
	u := 1 - t
	a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return a*x0 + b*x1 + c*x2 + d*x3, a*y0 + b*y1 + c*y2 + d*y3
}
