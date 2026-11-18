package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// ChatBlock is one rendered piece of an assistant turn: text, thinking or a
// tool call.
type ChatBlock struct {
	Kind    string
	Text    string
	ID      string
	Name    string
	Args    any
	Running bool
	Error   bool
	Result  string
	Details core.J
	Ms      int
}

func textBlock(s string) *ChatBlock     { return &ChatBlock{Kind: "text", Text: s} }
func thinkingBlock(s string) *ChatBlock { return &ChatBlock{Kind: "thinking", Text: s} }
func toolBlock(id, name string, args any) *ChatBlock {
	return &ChatBlock{Kind: "tool", ID: id, Name: name, Args: args}
}

// Actions are the tool's offered follow-ups (details.actions).
func (b *ChatBlock) Actions() []core.J { return b.Details.Os("actions") }

// AppOp is an app operation the bot ran (details.app).
func (b *ChatBlock) AppOp() core.J { return b.Details.O("app") }

// Proposal is a change waiting on a card (details.proposal).
func (b *ChatBlock) Proposal() core.J { return b.Details.O("proposal") }

// ChatMessage is a user message or an assistant turn.
type ChatMessage struct {
	Role      string
	Text      string
	Blocks    []*ChatBlock
	Streaming bool
	Error     string
	Status    string
	Model     string
	Usage     core.J
}

func (m *ChatMessage) LastBlock() *ChatBlock {
	if len(m.Blocks) == 0 {
		return nil
	}
	return m.Blocks[len(m.Blocks)-1]
}

// ChatSession is an entry of /agent/sessions.
type ChatSession struct{ core.J }

func (s ChatSession) ID() string { return s.S("id") }
func (s ChatSession) Title() string {
	if t := s.S("title"); t != "" {
		return t
	}
	return l10n.ChatNew
}
func (s ChatSession) Kind() string {
	if k := s.S("kind"); k != "" {
		return k
	}
	return "chat"
}
func (s ChatSession) CourseID() string { return s.S("course_id") }
func (s ChatSession) Course() string   { return s.S("course") }
func (s ChatSession) Modified() time.Time {
	t, err := time.Parse(time.RFC3339Nano, s.S("modified"))
	if err != nil {
		return time.UnixMilli(0)
	}
	return t.Local()
}
func (s ChatSession) Messages() int { return s.I("messages") }

// CourseRef is a course agent's course.
type CourseRef struct{ ID, Title string }

// AssistantController talks to the embedded Pi agent host, proxied by the
// daemon under /agent.
type AssistantController struct {
	Sessions       []ChatSession
	SessionID      string
	Messages       []*ChatMessage
	LoadingSession bool
	Stats          core.J
	Compacting     bool
	Models         core.J
	ModelsError    error

	// Bot is GET /agent/bot: name, shape, color, persona, session_id.
	Bot           core.J
	Consolidating bool
	// View is "bot" for the bot's thread, "sessions" for course agents and
	// chats.
	View string
	// PendingCourse is a course agent not yet started; Course the course of
	// the open conversation.
	PendingCourse *CourseRef
	Course        *CourseRef

	ComposerFocused bool
	happyUntil      time.Time
	confusedUntil   time.Time

	// MemoryChanged counts changes to the memory files; the bot page reloads.
	MemoryChanged int
	// AppOps hears the app operations of tool results as they happen.
	AppOps core.Events

	wasReady  bool
	listening bool
	cancel    context.CancelFunc
	appState  map[string]any
	catalog   []core.J
}

// NewAssistantController starts on the bot's thread.
func NewAssistantController() *AssistantController {
	return &AssistantController{View: "bot", Bot: core.J{}}
}

func api() *core.Api { return State.Api() }

func (a *AssistantController) watch() {
	if !a.listening && State.Connected() {
		a.listening = true
		State.Events.Listen(a.onAppEvent)
	}
	ready := a.Ready()
	if ready && !a.wasReady {
		a.wasReady = true
		a.RefreshSessions()
		a.LoadModels(nil)
		a.LoadBot(a.View == "bot" && a.SessionID == "" && len(a.Messages) == 0)
		if a.appState != nil {
			a.PushAppState(a.appState)
		}
	} else if !ready {
		a.wasReady = false
	}
}

func (a *AssistantController) Ready() bool { return State.AgentReady() }
func (a *AssistantController) Busy() bool {
	return len(a.Messages) > 0 && a.Messages[len(a.Messages)-1].Streaming
}
func (a *AssistantController) AgentState() string {
	if s := State.Agent.S("state"); s != "" {
		return s
	}
	return "starting"
}
func (a *AssistantController) AgentError() string { return State.Agent.S("error") }

// Selected is the selected model.
func (a *AssistantController) Selected() core.J { return a.Models.O("selected") }

func (a *AssistantController) ModelLabel() string {
	s := a.Selected()
	if s == nil {
		return l10n.ModelNone
	}
	return s.S("model")
}

func (a *AssistantController) Thinking() string {
	if t := a.Models.O("prefs").S("thinking"); t != "" {
		return t
	}
	return "medium"
}

// Approval is how the assistant's changes run: "auto" at once with 撤销,
// "ask" as cards the user applies.
func (a *AssistantController) Approval() string {
	if a.Models.O("prefs").S("approval") == "ask" {
		return "ask"
	}
	return "auto"
}

// SetApproval switches the mode, showing it before the agent answers.
func (a *AssistantController) SetApproval(mode string, done func(error)) {
	prefs := a.Models.O("prefs")
	was := a.Approval()
	if prefs != nil {
		prefs["approval"] = mode
	}
	a.SetPrefs(core.J{"approval": mode}, func(err error) {
		if err != nil && prefs != nil {
			prefs["approval"] = was
		}
		if done != nil {
			done(err)
		}
	})
}

// ---- models ----

func (a *AssistantController) LoadModels(done func()) {
	core.Async(func() (core.J, error) { return api().GetJ("/agent/models") }, func(m core.J, err error) {
		if err != nil {
			a.ModelsError = err
		} else {
			a.Models, a.ModelsError = m, nil
		}
		if done != nil {
			done()
		}
	})
}

func (a *AssistantController) setModels(call func() (any, error), done func(error)) {
	core.Async(call, func(v any, err error) {
		if err == nil {
			a.Models = core.Obj(v)
		}
		if done != nil {
			done(err)
		}
	})
}

func (a *AssistantController) SetPrefs(patch core.J, done func(error)) {
	a.setModels(func() (any, error) { return api().Put("/agent/prefs", patch) }, done)
}

func (a *AssistantController) SetKey(provider, key string, done func(error)) {
	a.setModels(func() (any, error) {
		return api().Put("/agent/providers/"+core.Esc(provider)+"/key", core.J{"api_key": key})
	}, done)
}

func (a *AssistantController) UpsertProvider(id string, body core.J, done func(error)) {
	a.setModels(func() (any, error) { return api().Put("/agent/providers/"+core.Esc(id), body) }, done)
}

func (a *AssistantController) RemoveProvider(id string, done func(error)) {
	a.setModels(func() (any, error) { return api().Delete("/agent/providers/" + core.Esc(id)) }, done)
}

// Discover lists the model IDs an endpoint offers, with models.dev metadata.
func (a *AssistantController) Discover(provider, baseURL, apiKind, apiKey, catalogID string, done func([]core.J, error)) {
	body := core.J{}
	for k, v := range map[string]string{"provider": provider, "base_url": baseURL, "api": apiKind, "api_key": apiKey, "catalog_id": catalogID} {
		if v != "" {
			body[k] = v
		}
	}
	core.Async(func() ([]core.J, error) {
		r, err := api().Post("/agent/discover", body)
		if err != nil {
			return nil, err
		}
		return core.Obj(r).Os("models"), nil
	}, done)
}

// CatalogProviders is the models.dev provider directory, cached.
func (a *AssistantController) CatalogProviders(refresh bool, done func([]core.J, error)) {
	if a.catalog != nil && !refresh {
		done(a.catalog, nil)
		return
	}
	core.Async(func() ([]core.J, error) {
		var q map[string]string
		if refresh {
			q = map[string]string{"refresh": "1"}
		}
		r, err := api().Get("/agent/catalog/providers", q)
		if err != nil {
			return nil, err
		}
		return core.Obj(r).Os("providers"), nil
	}, func(l []core.J, err error) {
		if err == nil {
			a.catalog = l
		}
		done(l, err)
	})
}

func (a *AssistantController) CatalogModels(id string, done func([]core.J, error)) {
	core.Async(func() ([]core.J, error) {
		r, err := api().Get("/agent/catalog/providers/" + core.Esc(id) + "/models")
		if err != nil {
			return nil, err
		}
		return core.Obj(r).Os("models"), nil
	}, done)
}

// SelectedModelInfo is the selected model's metadata from the registry.
func (a *AssistantController) SelectedModelInfo() core.J {
	s := a.Selected()
	if s == nil {
		return nil
	}
	for _, p := range a.Models.Os("providers") {
		if p.S("id") != s.S("provider") {
			continue
		}
		for _, m := range p.Os("models") {
			if m.S("id") == s.S("model") {
				return m
			}
		}
	}
	return nil
}

func (a *AssistantController) TestProvider(id, model string, done func(core.J, error)) {
	core.Async(func() (core.J, error) {
		r, err := api().Post("/agent/providers/"+core.Esc(id)+"/test", core.J{"model": model})
		return core.Obj(r), err
	}, done)
}

// ImportPi copies custom providers and keys from the Pi CLI.
func (a *AssistantController) ImportPi(done func(string, error)) {
	core.Async(func() (core.J, error) {
		r, err := api().Post("/agent/import/pi", nil)
		return core.Obj(r), err
	}, func(r core.J, err error) {
		if err != nil {
			done("", err)
			return
		}
		a.Models = r.O("models")
		imp := r.O("imported")
		done(l10n.ModelsImported(imp.I("providers"), imp.I("keys")), nil)
	})
}

// ---- the resident bot ----

func (a *AssistantController) BotName() string {
	if n := a.Bot.S("name"); n != "" {
		return n
	}
	return l10n.BotDefaultName
}
func (a *AssistantController) BotShape() string {
	if s := a.Bot.S("shape"); s != "" {
		return s
	}
	return DefaultBotShape
}
func (a *AssistantController) BotColor() string {
	if s := a.Bot.S("color"); s != "" {
		return s
	}
	return "theme"
}

// Mood is the bot's mood from what it is doing.
func (a *AssistantController) Mood() BotMood {
	now := time.Now()
	if a.Busy() {
		r := a.Messages[len(a.Messages)-1]
		for _, b := range r.Blocks {
			if b.Kind == "tool" && b.Running {
				return MoodWorking
			}
		}
		if b := r.LastBlock(); b != nil && b.Kind == "text" && b.Text != "" {
			return MoodTalking
		}
		return MoodThinking
	}
	if now.Before(a.confusedUntil) {
		return MoodConfused
	}
	if now.Before(a.happyUntil) {
		return MoodHappy
	}
	if a.Consolidating {
		return MoodDreaming
	}
	if a.PendingProposals() > 0 {
		return MoodNotify
	}
	if a.ComposerFocused {
		return MoodListening
	}
	return MoodIdle
}

// PendingProposals counts the cards still waiting for the user.
func (a *AssistantController) PendingProposals() int {
	n := 0
	for _, m := range a.Messages {
		for _, b := range m.Blocks {
			if b.Proposal().S("state") == "pending" {
				n++
			}
		}
	}
	return n
}

func (a *AssistantController) flash(happy, confused bool) {
	until := time.Now().Add(2400 * time.Millisecond)
	if happy {
		a.happyUntil = until
	}
	if confused {
		a.confusedUntil = until
	}
	go func() { time.Sleep(2500 * time.Millisecond); core.Post(func() {}) }()
}

func (a *AssistantController) SetComposerFocused(v bool) { a.ComposerFocused = v }

func (a *AssistantController) onAppEvent(e core.J) {
	if e.S("type") != "bot_event" {
		return
	}
	switch e.O("data").S("kind") {
	case "consolidating":
		a.Consolidating = true
	case "consolidated", "failed":
		a.Consolidating = false
		a.MemoryChanged++
		a.LoadBot(false)
	case "memory_changed":
		a.MemoryChanged++
	}
}

func (a *AssistantController) LoadBot(openThread bool) {
	core.Async(func() (core.J, error) { return api().GetJ("/agent/bot") }, func(b core.J, err error) {
		if err != nil {
			return
		}
		a.Bot = b
		a.Consolidating = b.IsTrue("consolidating")
		if id := b.S("session_id"); openThread && id != "" && a.View == "bot" && a.SessionID == "" {
			a.OpenSession(id)
		}
	})
}

func (a *AssistantController) setBot(call func() (any, error), done func(error)) {
	core.Async(call, func(v any, err error) {
		if err == nil {
			a.Bot = core.Obj(v)
		}
		if done != nil {
			done(err)
		}
	})
}

func (a *AssistantController) UpdateBot(patch core.J, done func(error)) {
	a.setBot(func() (any, error) { return api().Put("/agent/bot", patch) }, done)
}

// ResetBot starts a new bot thread; memory stays.
func (a *AssistantController) ResetBot(done func(error)) {
	a.setBot(func() (any, error) { return api().Post("/agent/bot/reset", nil) }, func(err error) {
		if err == nil && a.View == "bot" {
			a.clear()
		}
		if done != nil {
			done(err)
		}
	})
}

func (a *AssistantController) ConsolidateNow(done func(error)) {
	a.setBot(func() (any, error) { return api().Post("/agent/bot/memory/consolidate", nil) }, func(err error) {
		if err == nil {
			a.Consolidating = a.Bot.IsTrue("consolidating")
		}
		if done != nil {
			done(err)
		}
	})
}

func (a *AssistantController) MemoryFiles(done func([]core.J, error)) {
	core.Async(func() ([]core.J, error) {
		r, err := api().Get("/agent/bot/memory")
		return core.Objs(r), err
	}, done)
}

func (a *AssistantController) ReadMemory(path string, done func(core.J, error)) {
	core.Async(func() (core.J, error) {
		r, err := api().Get("/agent/bot/memory/file", map[string]string{"path": path})
		return core.Obj(r), err
	}, done)
}

func (a *AssistantController) WriteMemory(path, content, baseHash string, done func(error)) {
	body := core.J{"path": path, "content": content}
	if baseHash != "" {
		body["base_hash"] = baseHash
	}
	core.Go(func() error { _, err := api().Put("/agent/bot/memory/file", body); return err }, func(err error) {
		if err == nil {
			a.MemoryChanged++
		}
		done(err)
	})
}

func (a *AssistantController) DeleteMemory(path string, done func(error)) {
	core.Go(func() error {
		_, err := api().Call("DELETE", "/agent/bot/memory/file?path="+core.Esc(path), nil)
		return err
	}, func(err error) {
		if err == nil {
			a.MemoryChanged++
		}
		done(err)
	})
}

func (a *AssistantController) MemoryRuns(done func([]core.J, error)) {
	core.Async(func() ([]core.J, error) {
		r, err := api().Get("/agent/bot/memory/runs")
		return core.Objs(r), err
	}, done)
}

// PushAppState sends the preferences the bot may read.
func (a *AssistantController) PushAppState(prefs map[string]any) {
	a.appState = prefs
	if !a.Ready() {
		return
	}
	go api().Put("/agent/app-state", prefs)
}

// ShowBot shows the bot's thread.
func (a *AssistantController) ShowBot() {
	if a.Busy() {
		return
	}
	a.View, a.PendingCourse = "bot", nil
	a.clear()
	if id := a.Bot.S("session_id"); id != "" {
		a.OpenSession(id)
	}
}

// ShowSessions shows course agents and chats, from a new conversation.
func (a *AssistantController) ShowSessions() {
	if a.Busy() {
		return
	}
	a.View = "sessions"
	a.clear()
	a.RefreshSessions()
}

// OpenCourseAgent opens the latest conversation of a course's agent, or
// prepares a new one.
func (a *AssistantController) OpenCourseAgent(courseID, title string, fresh bool) {
	if a.Busy() {
		return
	}
	a.View = "sessions"
	a.refreshSessionsThen(func() {
		if !fresh {
			for _, s := range a.Sessions {
				if s.Kind() == "course" && s.CourseID() == courseID {
					a.PendingCourse = nil
					a.OpenSession(s.ID())
					return
				}
			}
		}
		a.clear()
		a.PendingCourse = &CourseRef{courseID, title}
		a.Course = a.PendingCourse
	})
}

func (a *AssistantController) clear() {
	a.SessionID, a.Messages, a.Stats, a.Course = "", nil, nil, nil
}

func (a *AssistantController) proposal(b *ChatBlock, action string, done func(core.J, error)) {
	id := b.Proposal().S("id")
	core.Async(func() (core.J, error) {
		r, err := api().Post("/agent/proposals/"+id+"/"+action, nil)
		return core.Obj(r), err
	}, func(r core.J, err error) {
		if err == nil {
			p := b.Proposal().Copy()
			p["state"] = r["state"]
			if b.Details == nil {
				b.Details = core.J{}
			}
			b.Details["proposal"] = p
			if res := r.O("result"); res.O("app") != nil {
				a.AppOps.Emit(core.J{"op": "prefs", "set": res.O("app"), "silent": true})
			}
		}
		if done != nil {
			done(r, err)
		}
	})
}

func (a *AssistantController) ApplyProposal(b *ChatBlock, done func(error)) {
	a.proposal(b, "apply", func(_ core.J, err error) {
		if err == nil {
			a.flash(true, false)
		}
		done(err)
	})
}

func (a *AssistantController) DismissProposal(b *ChatBlock, done func(error)) {
	a.proposal(b, "dismiss", func(_ core.J, err error) { done(err) })
}

func (a *AssistantController) UndoProposal(b *ChatBlock, done func(error)) {
	a.proposal(b, "undo", func(_ core.J, err error) { done(err) })
}

// UndoApplied undoes an operation applied at once (its chip's 撤销).
func (a *AssistantController) UndoApplied(b *ChatBlock, done func(error)) {
	id := b.AppOp().S("proposal_id")
	if id == "" {
		return
	}
	core.Async(func() (core.J, error) {
		r, err := api().Post("/agent/proposals/"+id+"/undo", nil)
		return core.Obj(r), err
	}, func(r core.J, err error) {
		if err == nil {
			op := b.AppOp().Copy()
			op["undone"] = true
			b.Details["app"] = op
			if res := r.O("result"); res.O("app") != nil {
				a.AppOps.Emit(core.J{"op": "prefs", "set": res.O("app"), "silent": true})
			}
		}
		done(err)
	})
}

// ---- sessions ----

func (a *AssistantController) RefreshSessions() { a.refreshSessionsThen(nil) }

func (a *AssistantController) refreshSessionsThen(then func()) {
	core.Async(func() ([]core.J, error) {
		r, err := api().Get("/agent/sessions")
		return core.Objs(r), err
	}, func(l []core.J, err error) {
		if err == nil {
			a.Sessions = a.Sessions[:0]
			for _, s := range l {
				a.Sessions = append(a.Sessions, ChatSession{s})
			}
		}
		if then != nil {
			then()
		}
	})
}

// NewChat starts a conversation; on a course or lecture page it is that
// course's agent.
func (a *AssistantController) NewChat() {
	if a.Busy() {
		return
	}
	if a.View == "bot" {
		a.View = "sessions"
	}
	a.clear()
	c := Screen.Value
	a.PendingCourse = nil
	if c["course_id"] != "" {
		a.PendingCourse = &CourseRef{c["course_id"], c["course"]}
	}
	a.Course = a.PendingCourse
}

// NewPlainChat starts a conversation not bound to a course.
func (a *AssistantController) NewPlainChat() {
	if a.Busy() {
		return
	}
	a.View = "sessions"
	a.clear()
	a.PendingCourse = nil
}

func (a *AssistantController) OpenSession(id string) {
	if a.Busy() || id == a.SessionID {
		return
	}
	a.SessionID, a.LoadingSession, a.Messages = id, true, nil
	core.Async(func() (core.J, error) { return api().GetJ("/agent/sessions/" + id) }, func(r core.J, err error) {
		if err != nil {
			a.Messages = []*ChatMessage{{Role: "assistant", Error: l10n.ChatOpenFailed(err)}}
		} else {
			a.Messages = fromHistory(r.L("messages"))
			a.Stats = r.O("stats")
			a.View = "sessions"
			if r.S("kind") == "bot" && id == a.Bot.S("session_id") {
				a.View = "bot"
			}
			a.Course = nil
			if cid := r.S("course_id"); cid != "" {
				a.Course = &CourseRef{cid, r.S("course")}
			}
			a.PendingCourse = nil
		}
		a.LoadingSession = false
	})
}

func (a *AssistantController) DeleteSession(id string, done func(error)) {
	core.Go(func() error { _, err := api().Delete("/agent/sessions/" + id); return err }, func(err error) {
		if err == nil {
			out := a.Sessions[:0]
			for _, s := range a.Sessions {
				if s.ID() != id {
					out = append(out, s)
				}
			}
			a.Sessions = out
			if a.SessionID == id {
				a.NewPlainChat()
			}
		}
		if done != nil {
			done(err)
		}
	})
}

func fromHistory(raw []any) []*ChatMessage {
	var out []*ChatMessage
	tools := map[string]*ChatBlock{}
	for _, v := range raw {
		m := core.Obj(v)
		if m == nil {
			continue
		}
		switch m.S("role") {
		case "user":
			out = append(out, &ChatMessage{Role: "user", Text: m.S("text")})
		case "assistant":
			var turn *ChatMessage
			if len(out) > 0 && out[len(out)-1].Role == "assistant" {
				turn = out[len(out)-1]
			} else {
				turn = &ChatMessage{Role: "assistant"}
				out = append(out, turn)
			}
			for _, b := range m.Os("blocks") {
				switch b.S("type") {
				case "text":
					if strings.TrimSpace(b.S("text")) != "" {
						turn.Blocks = append(turn.Blocks, textBlock(b.S("text")))
					}
				case "thinking":
					if strings.TrimSpace(b.S("text")) != "" {
						turn.Blocks = append(turn.Blocks, thinkingBlock(b.S("text")))
					}
				case "tool":
					tb := toolBlock(b.S("id"), b.S("name"), b["args"])
					tools[tb.ID] = tb
					turn.Blocks = append(turn.Blocks, tb)
				}
			}
			if m["error"] != nil {
				turn.Error = m.S("error")
			}
			if u := m.O("usage"); u != nil {
				turn.Usage = u
			}
			if m["model"] != nil {
				turn.Model = m.S("model")
			}
		case "tool":
			if tb := tools[m.S("id")]; tb != nil {
				tb.Result = m.S("text")
				tb.Error = m.IsTrue("error")
				if d := m.O("details"); d != nil {
					tb.Details = d
				}
			}
		}
	}
	return out
}

// ---- chat ----

func (a *AssistantController) context() map[string]string {
	c := map[string]string{}
	for k, v := range Screen.Value {
		c[k] = v
	}
	if p := Players.Value; p != nil {
		c["player_lecture_id"] = p.LectureID()
		c["player_position"] = fmt.Sprintf("%s (%ds)", core.FmtClock(p.PositionMs()), p.PositionMs()/1000)
	}
	c["now"] = time.Now().Format("2006-01-02T15:04")
	return c
}

// Send asks the agent and streams its reply into a new turn.
func (a *AssistantController) Send(text string) {
	msg := strings.TrimSpace(text)
	if msg == "" || a.Busy() || !a.Ready() {
		return
	}
	lecture := ""
	if Players.Value != nil {
		lecture = Players.Value.LectureID()
	}
	core.Tracker.Count("ask", lecture)
	reply := &ChatMessage{Role: "assistant", Streaming: true}
	a.Messages = append(append([]*ChatMessage{}, a.Messages...), &ChatMessage{Role: "user", Text: msg}, reply)
	body := core.J{"message": msg, "context": a.context()}
	if a.SessionID != "" {
		body["session_id"] = a.SessionID
	} else if a.View == "bot" {
		body["agent"] = "bot"
	} else if a.PendingCourse != nil {
		body["course_id"] = a.PendingCourse.ID
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	sse := api()
	go func() {
		err := sse.SSE(ctx, "POST", "/agent/chat", body, func(e core.J) { core.Post(func() { a.onEvent(reply, e) }) })
		core.Post(func() {
			if err != nil && !errors.Is(err, context.Canceled) {
				var ae *core.ApiError
				if errors.As(err, &ae) {
					reply.Error = ae.Message
				} else {
					reply.Error = err.Error()
				}
			}
			a.finish(reply)
			a.RefreshSessions()
		})
	}()
}

func (a *AssistantController) onEvent(r *ChatMessage, e core.J) {
	switch e.S("type") {
	case "start":
		a.SessionID = e.S("session_id")
		if e["model"] != nil {
			r.Model = e.S("model")
		}
		if e.S("kind") == "bot" {
			a.Bot = a.Bot.Copy()
			a.Bot["session_id"] = a.SessionID
		}
		if a.PendingCourse != nil {
			a.Course, a.PendingCourse = a.PendingCourse, nil
		}
	case "block":
		if e.S("kind") == "thinking" {
			r.Blocks = append(r.Blocks, thinkingBlock(""))
		} else {
			r.Blocks = append(r.Blocks, textBlock(""))
		}
		r.Status = ""
	case "text", "thinking":
		kind := e.S("type")
		if b := r.LastBlock(); b == nil || b.Kind != kind {
			r.Blocks = append(r.Blocks, &ChatBlock{Kind: kind})
		}
		r.LastBlock().Text += e.S("delta")
	case "tool_call":
		r.Blocks = append(r.Blocks, toolBlock(e.S("id"), e.S("name"), e["args"]))
	case "tool_start":
		b := toolOf(r, e.S("id"))
		if b == nil {
			b = toolBlock(e.S("id"), e.S("name"), e["args"])
			r.Blocks = append(r.Blocks, b)
		}
		b.Running = true
	case "tool_end":
		if b := toolOf(r, e.S("id")); b != nil {
			b.Running = false
			b.Error = e.IsTrue("error")
			b.Result = e.S("text")
			b.Ms = e.I("ms")
			if d := e.O("details"); d != nil {
				b.Details = d
			}
			if op := b.AppOp(); op != nil && !b.Error {
				a.AppOps.Emit(op)
			}
		}
	case "message_end":
		if u := e.O("usage"); u != nil {
			if p := r.Usage; p == nil {
				r.Usage = u
			} else {
				r.Usage = core.J{"input": p.F("input") + u.F("input"), "output": p.F("output") + u.F("output"), "cost": p.F("cost") + u.F("cost")}
			}
		}
		if e["error"] != nil {
			r.Error = e.S("error")
		}
	case "status":
		r.Status = e.S("text")
	case "error":
		r.Error = e.S("message")
	case "done":
		if s := e.O("stats"); s != nil {
			a.Stats = s
		}
	}
}

func toolOf(r *ChatMessage, id string) *ChatBlock {
	for i := len(r.Blocks) - 1; i >= 0; i-- {
		if b := r.Blocks[i]; b.Kind == "tool" && b.ID == id {
			return b
		}
	}
	return nil
}

func (a *AssistantController) finish(r *ChatMessage) {
	if r.Streaming {
		a.flash(r.Error == "", r.Error != "")
	}
	r.Streaming, r.Status = false, ""
	for _, b := range r.Blocks {
		b.Running = false
	}
	a.cancel = nil
}

// Stop aborts the running turn.
func (a *AssistantController) Stop() {
	if id := a.SessionID; id != "" {
		go api().Post("/agent/sessions/"+id+"/abort", nil)
	}
	if a.cancel != nil {
		a.cancel()
	}
	if len(a.Messages) > 0 {
		a.finish(a.Messages[len(a.Messages)-1])
	}
}

// Compact summarises older turns so the conversation fits again.
func (a *AssistantController) Compact(done func(error)) {
	id := a.SessionID
	if id == "" || a.Busy() || a.Compacting {
		return
	}
	a.Compacting = true
	core.Async(func() (core.J, error) {
		r, err := api().Post("/agent/sessions/"+id+"/compact", nil)
		return core.Obj(r), err
	}, func(r core.J, err error) {
		if err == nil {
			a.Stats = r.O("stats")
		}
		a.Compacting = false
		if done != nil {
			done(err)
		}
	})
}

// RetryLast resends the last user message in a fresh turn.
func (a *AssistantController) RetryLast() {
	for i := len(a.Messages) - 1; i >= 0; i-- {
		if m := a.Messages[i]; m.Role == "user" {
			if m.Text != "" {
				a.Send(m.Text)
			}
			return
		}
	}
}
