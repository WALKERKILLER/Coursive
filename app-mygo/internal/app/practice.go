package app

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// quiz.dart and practice_page.dart: practice quizzes
// (docs/specs/0002-tutor-quiz-cards.md).

// QuizOption is one choice of a question.
type QuizOption struct{ Text, Rationale string }

// QuizQuestion is one question.
type QuizQuestion struct {
	Stem, Hint, Topic, Ref string
	Options                []QuizOption
	Answer                 int
}

// QuizScore is an attempt's score.
type QuizScore struct {
	Correct, Total int
}

// Label is "correct/total".
func (s QuizScore) Label() string { return strconv.Itoa(s.Correct) + "/" + strconv.Itoa(s.Total) }

// Quiz is a practice set.
type Quiz struct {
	ID, Title, LectureID, LectureTitle, Difficulty, Source string
	Count, Attempts                                        int
	Questions                                              []QuizQuestion
	Best, Last                                             *QuizScore
}

func score(j core.J) *QuizScore {
	if j == nil {
		return nil
	}
	return &QuizScore{Correct: j.I("correct"), Total: j.I("total")}
}

// ParseQuiz reads a quiz from its JSON.
func ParseQuiz(j core.J) Quiz {
	q := Quiz{ID: j.S("id"), Title: j.S("title"), LectureID: j.S("lecture_id"), LectureTitle: j.S("lecture_title"),
		Difficulty: j.Or("difficulty", "medium"), Source: j.S("source"), Count: j.I("count"), Attempts: j.I("attempts"),
		Best: score(j.O("best")), Last: score(j.O("last"))}
	for _, x := range j.Os("questions") {
		qq := QuizQuestion{Stem: x.S("stem"), Hint: x.S("hint"), Topic: x.S("topic"), Ref: x.S("ref"), Answer: x.I("answer")}
		for _, o := range x.Os("options") {
			qq.Options = append(qq.Options, QuizOption{Text: o.S("text"), Rationale: o.S("rationale")})
		}
		q.Questions = append(q.Questions, qq)
	}
	return q
}

// Meta is the lecture, the size and the difficulty.
func (q Quiz) Meta() string {
	var parts []string
	if q.LectureTitle != "" {
		parts = append(parts, q.LectureTitle)
	}
	return strings.Join(append(parts, l10n.QuizCount(q.Count), DifficultyLabel(q.Difficulty)), " · ")
}

// DifficultyLabel is difficultyLabel.
func DifficultyLabel(d string) string {
	switch d {
	case "easy":
		return l10n.QuizEasy
	case "hard":
		return l10n.QuizHard
	}
	return l10n.QuizMedium
}

// AskInSheet sends prompt to the assistant and shows it in a sheet over
// the current screen, so the student keeps their place.
func AskInSheet(prompt string) {
	if !Assistant.Ready() || Assistant.Busy() {
		msg := l10n.QuizAssistantNotReady
		if Assistant.Busy() {
			msg = l10n.QuizAssistantBusy
		}
		widgets.ShowSnack(msg, "", nil)
		return
	}
	Assistant.Send(prompt)
	ShowAssistantSheet()
}

// ShowAssistantSheet is showAssistantSheet: the assistant docked in a
// sheet.
func ShowAssistantSheet() {
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true, MaxWidth: 720, HeightFactor: .85}, func(c *ui.Context, r *m3.Route) {
		ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() { AssistantPanelView(c, true) })
	})
}

// OpenPractice opens a practice set.
func OpenPractice(quizID string) { m3.RootNav.Push(NewPracticePage(quizID)) }

// QuizCard is a practice set: in chat after the assistant writes one,
// under a lecture and in the library. Live loads the latest scores and
// follows changes; Dense makes it one row with a compact action, for
// lists.
type QuizCard struct {
	quiz   Quiz
	gone   bool
	live   bool
	cancel func()
}

// NewQuizCard keeps a card of q; live follows its scores.
func NewQuizCard(q Quiz, live bool) *QuizCard {
	k := &QuizCard{quiz: q, live: live}
	if live {
		k.refresh()
		k.cancel = State.Events.Listen(func(e core.J) {
			if e.S("type") == "quizzes_changed" && e.O("data").S("id") == k.quiz.ID {
				k.refresh()
			}
		})
	}
	return k
}

// Dispose stops following the quiz.
func (k *QuizCard) Dispose() {
	if k.cancel != nil {
		k.cancel()
	}
}

// Set replaces the quiz of a card that does not follow it.
func (k *QuizCard) Set(q Quiz) {
	if !k.live {
		k.quiz = q
	}
}

func (k *QuizCard) refresh() {
	id := k.quiz.ID
	core.Async(func() (any, error) { return State.Api().Get("/api/quizzes/" + core.Esc(id)) }, func(r any, err error) {
		var ae *core.ApiError
		switch {
		case errors.As(err, &ae) && ae.Status == 404:
			k.gone = true
		case err == nil:
			k.quiz = ParseQuiz(core.Obj(r))
		}
	})
}

// Show builds the card.
func (k *QuizCard) Show(c *ui.Context, dense bool) {
	cs, tt := design.S(), design.TT()
	q := k.quiz
	sc := l10n.QuizNone
	switch {
	case k.gone:
		sc = l10n.CommonDeleted
	case q.Best != nil:
		sc = l10n.QuizBestAttempts(q.Best.Label(), q.Attempts)
	}
	card := m3.Tappable(c, design.CornerLG).FillWidth().Background(cs.TertiaryContainer).Disabled(k.gone)
	if card.Clicked() {
		OpenPractice(q.ID)
	}
	icon := func() {
		ui.Box(c).Size(40, 40).Shrink(0).Radius(20).Background(design.WithOpacity(cs.SurfaceContainerLowest, .7)).Center().Children(func() {
			m3.Icon(c, icons.QuizOutlined, 20, cs.OnTertiaryContainer)
		})
	}
	texts := func() {
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			m3.Text(c, q.Title, tt.TitleSmall.C(cs.OnTertiaryContainer)).MaxLines(2)
			m3.VGap(c, 2)
			m3.Text(c, q.Meta(), tt.BodySmall.C(design.WithOpacity(cs.OnTertiaryContainer, .8))).MaxLines(1)
			m3.Text(c, sc, tt.LabelMedium.C(cs.OnTertiaryContainer)).FontFeatures("tnum")
		})
	}
	card.Padding(design.SpaceMD, design.SpaceMD, design.SpaceMD, design.SpaceLG).Children(func() {
		if dense {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(design.SpaceMD).Children(func() {
				icon()
				texts()
				m3.Icon(c, icons.ChevronRightRounded, 24, cs.OnTertiaryContainer)
			})
			return
		}
		ui.Column(c).FillWidth().Children(func() {
			ui.Row(c).FillWidth().Gap(design.SpaceMD).Children(func() {
				icon()
				texts()
			})
			m3.VGap(c, design.SpaceMD)
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				label := l10n.QuizAgain
				if q.Attempts == 0 {
					label = l10n.QuizStart
				}
				if (m3.Button{Kind: m3.Filled, Label: label, BG: cs.Tertiary, FG: cs.OnTertiary, Disabled: k.gone}).Show(c).Clicked() {
					OpenPractice(q.ID)
				}
			})
		})
	})
}

// QuizList is the practice sets of one lecture (or all, with an empty
// id), newest first.
type QuizList struct {
	lectureID string
	quizzes   []Quiz
	loaded    bool
	cards     map[string]*QuizCard
	cancel    func()
}

// NewQuizList loads the sets and follows changes.
func NewQuizList(lectureID string) *QuizList {
	l := &QuizList{lectureID: lectureID, cards: map[string]*QuizCard{}}
	l.load()
	l.cancel = State.Events.Listen(func(e core.J) {
		if e.S("type") == "quizzes_changed" {
			l.load()
		}
	})
	return l
}

// Dispose stops following changes.
func (l *QuizList) Dispose() { l.cancel() }

func (l *QuizList) load() {
	q := map[string]string{}
	if l.lectureID != "" {
		q["lecture_id"] = l.lectureID
	}
	core.Async(func() (any, error) { return State.Api().Get("/api/quizzes", q) }, func(r any, err error) {
		if err != nil {
			return
		}
		l.quizzes, l.loaded = nil, true
		for _, j := range core.Objs(r) {
			l.quizzes = append(l.quizzes, ParseQuiz(j))
		}
	})
}

// Len is the number of sets, -1 before they load.
func (l *QuizList) Len() int {
	if !l.loaded {
		return -1
	}
	return len(l.quizzes)
}

// Show builds the sets in columns, header above them when there are any,
// empty when there are none.
func (l *QuizList) Show(c *ui.Context, columns int, header, empty func()) {
	if !l.loaded {
		return
	}
	if len(l.quizzes) == 0 {
		if empty != nil {
			empty()
		}
		return
	}
	ui.Column(c).FillWidth().Children(func() {
		if header != nil {
			header()
		}
		ui.Grid(c).FillWidth().Columns(max(columns, 1)).GapY(design.SpaceSM).GapX(design.SpaceMD).Children(func() {
			for _, q := range l.quizzes {
				k := l.cards[q.ID]
				if k == nil {
					k = NewQuizCard(q, false)
					l.cards[q.ID] = k
				}
				k.Set(q)
				ui.Box(c.Key(q.ID)).FillWidth().Children(func() { k.Show(c, true) })
			}
		})
	})
}

// ShowQuizSheet asks the assistant for a practice set on a lecture: count,
// difficulty and an optional focus. onAsk receives the prompt.
func ShowQuizSheet(lectureID string, onAsk func(string)) {
	count, difficulty, focus := 10, "medium", ""
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true, MaxWidth: 560}, func(c *ui.Context, r *m3.Route) {
		cs, tt := design.S(), design.TT()
		label := func(t string) {
			m3.Text(c, t, tt.LabelLarge.C(cs.OnSurfaceVariant)).Padding(0, 0, design.SpaceSM, 0)
		}
		ui.Column(c).FillWidth().Padding(0, design.SpaceXL, design.SpaceXL, design.SpaceXL).Children(func() {
			m3.Text(c, l10n.QuizCreateTitle, tt.TitleLarge)
			m3.VGap(c, design.SpaceXS)
			m3.Text(c, l10n.QuizCreateHint, tt.BodyMedium.C(cs.OnSurfaceVariant))
			m3.VGap(c, design.SpaceXL)
			label(l10n.QuizSize)
			var sizes []m3.Segment[int]
			for _, n := range []int{5, 10, 15} {
				sizes = append(sizes, m3.Segment[int]{Value: n, Label: l10n.QuizQuestions(n)})
			}
			m3.Segmented(c, sizes, &count, m3.SegOpts{NoCheck: true, Fill: true})
			m3.VGap(c, design.SpaceLG)
			label(l10n.QuizDifficulty)
			var levels []m3.Segment[string]
			for _, d := range []string{"easy", "medium", "hard"} {
				levels = append(levels, m3.Segment[string]{Value: d, Label: DifficultyLabel(d)})
			}
			m3.Segmented(c, levels, &difficulty, m3.SegOpts{NoCheck: true, Fill: true})
			m3.VGap(c, design.SpaceLG)
			m3.Field{Label: l10n.QuizFocus, Hint: l10n.QuizFocusHint}.Show(c, &focus)
			m3.VGap(c, design.SpaceXL)
			if (m3.Button{Kind: m3.Filled, Icon: icons.AutoAwesomeOutlined, Label: l10n.QuizCreate}).Show(c).FillWidth().Clicked() {
				f := strings.TrimSpace(focus)
				on := l10n.QuizFocusAll
				if f != "" {
					on = l10n.QuizFocusOn(f)
				}
				r.Close()
				onAsk(l10n.PromptCreateQuiz(lectureID, count, DifficultyLabel(difficulty), on))
			}
		})
	})
}

// PracticePage is the full-screen practice: one question at a time,
// feedback after each answer, and a summary at the end. With only, it
// retakes just those questions and records no attempt.
type PracticePage struct {
	quizID    string
	only      []int
	quiz      *Quiz
	err       error
	order     []int
	pos       int
	answers   map[int]int
	hints     map[int]bool
	started   time.Time
	finished  time.Time
	reviewing bool
	best      *QuizScore
	saving    bool
	activity  ActivityRegion
}

// NewPracticePage opens the set quizID.
func NewPracticePage(quizID string) m3.Page { return newPractice(quizID, nil) }

func newPractice(quizID string, only []int) *PracticePage {
	p := &PracticePage{quizID: quizID, only: only, answers: map[int]int{}, hints: map[int]bool{}, started: time.Now()}
	p.load()
	return p
}

// Dispose stops counting practice time.
func (p *PracticePage) Dispose() { p.activity.Stop() }

func (p *PracticePage) load() {
	p.err = nil
	core.Async(func() (any, error) { return State.Api().Get("/api/quizzes/" + core.Esc(p.quizID)) }, func(r any, err error) {
		if err != nil {
			p.err = err
			return
		}
		q := ParseQuiz(core.Obj(r))
		p.quiz, p.best, p.order = &q, q.Best, nil
		if p.only != nil {
			for _, i := range p.only {
				if i >= 0 && i < len(q.Questions) {
					p.order = append(p.order, i)
				}
			}
		} else {
			for i := range q.Questions {
				p.order = append(p.order, i)
			}
		}
	})
}

func (p *PracticePage) current() int { return p.order[p.pos] }
func (p *PracticePage) done() bool   { return !p.finished.IsZero() && !p.reviewing }

func (p *PracticePage) choose(option int) {
	q := p.quiz.Questions[p.current()]
	if _, ok := p.answers[p.current()]; ok || option >= len(q.Options) {
		return
	}
	p.answers[p.current()] = option
}

func (p *PracticePage) step(delta int) {
	next := p.pos + delta
	if next < 0 {
		return
	}
	if _, ok := p.answers[p.current()]; delta > 0 && !ok {
		return
	}
	if next >= len(p.order) {
		if p.finished.IsZero() {
			p.finish()
		} else {
			p.reviewing = false
		}
		return
	}
	p.pos = next
}

func (p *PracticePage) finish() {
	p.finished = time.Now()
	if p.only != nil {
		return
	}
	q := p.quiz
	answers := make([]int, len(q.Questions))
	for i := range answers {
		answers[i] = -1
		if a, ok := p.answers[i]; ok {
			answers[i] = a
		}
	}
	p.saving = true
	body := core.J{"started_at": p.started.Unix(), "answers": answers}
	core.Async(func() (any, error) { return State.Api().Post("/api/quizzes/"+core.Esc(q.ID)+"/attempts", body) }, func(r any, err error) {
		p.saving = false
		if err != nil {
			widgets.ShowSnack(l10n.PracticeScoreNotSaved(err), "", nil)
			return
		}
		if b := core.Obj(r).O("best"); b != nil {
			p.best = score(b)
		}
	})
}

func (p *PracticePage) explain() {
	q := p.quiz
	x := q.Questions[p.current()]
	a := p.answers[p.current()]
	opt := func(i int) string { return l10n.PracticeOption(string(rune('A'+i)), x.Options[i].Text) }
	var all []string
	for i := range x.Options {
		all = append(all, opt(i))
	}
	why := l10n.PracticeAskWhyWrong
	if a == x.Answer {
		why = l10n.PracticeAskWhyRight
	}
	ref := ""
	if x.Ref != "" {
		ref = l10n.PracticeSourceRef(x.Ref)
	}
	AskInSheet(l10n.PromptExplainQuestion(q.Title, p.pos+1, x.Stem, strings.Join(all, l10n.PracticeOptionSeparator), opt(a), opt(x.Answer), why, ref))
}

func (p *PracticePage) keys(c *ui.Context) {
	if p.quiz == nil || p.done() || m3.AnyRoute() {
		return
	}
	for i := 0; i < 6; i++ {
		if c.Shortcut(0, ui.Key1+ui.Key(i)) {
			p.choose(i)
		}
	}
	if c.Shortcut(0, ui.KeyEnter) || c.Shortcut(0, ui.KeyRight) {
		p.step(1)
	}
	if c.Shortcut(0, ui.KeyLeft) {
		p.step(-1)
	}
}

// View builds the page.
func (p *PracticePage) View(c *ui.Context) {
	q := p.quiz
	p.keys(c)
	title := l10n.CommonPractice
	if q != nil {
		title = q.Title
		if p.only != nil {
			title = l10n.PracticeRetryTitle(q.Title)
		}
	}
	answered := 0
	for _, i := range p.order {
		if _, ok := p.answers[i]; ok {
			answered++
		}
	}
	lecture := ""
	if q != nil {
		lecture = q.LectureID
	}
	bar := &m3.AppBar{Title: title,
		Leading: func() {
			if m3.IconBtn(c, icons.CloseRounded, l10n.CommonClose).Clicked() {
				m3.RootNav.Pop()
			}
		},
		Actions: func() {
			if q != nil && !p.done() {
				m3.Text(c, strconv.Itoa(p.pos+1)+" / "+strconv.Itoa(len(p.order)), design.TT().TitleSmall).FontFeatures("tnum").Padding(0, design.SpaceLG, 0, 0)
			}
		}}
	if q != nil && !p.done() {
		v := float64(answered) / float64(max(1, len(p.order)))
		bar.Bottom = func() {
			e := ui.Box(c).FillWidth().Height(4)
			at := e.AnimateTo("progress", float32(v), design.T().Motion(design.Medium2, design.Emphasized))
			e.Children(func() { m3.Linear(c, m3.Progress{Value: m3.Of(float64(at))}) })
		}
	}
	m3.Scaffold{AppBar: bar, Body: func() {
		key := "load"
		switch {
		case q == nil:
		case p.done():
			key = "done"
		default:
			key = "q" + strconv.Itoa(p.current())
		}
		body := ui.Column(c.Key(key)).Fill().Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.EmphasizedDecelerate,
			Enter: &ui.Motion{Opacity: 0, X: 16}})
		p.activity.Show(c, body, "practice", lecture, p.pos)
		body.Children(func() {
			switch {
			case q == nil && p.err != nil:
				widgets.ErrorState(c, p.err, p.load)
			case q == nil:
				widgets.CenteredLoader(c, "")
			case p.done():
				p.summary(c)
			default:
				p.question(c)
			}
		})
	}}.Show(c)
}

func (p *PracticePage) question(c *ui.Context) {
	cs, tt := design.S(), design.TT()
	q := p.quiz
	idx := p.current()
	x := q.Questions[idx]
	chosen, answered := p.answers[idx]
	ref, hasRef := ParseRef(x.Ref)
	onRef := func(r CoursiveRef) { OpenRef(r, q.LectureID, nil) }
	last := p.pos == len(p.order)-1
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(design.SpaceXL, 0, design.SpaceXXXL, 0).Children(func() {
			widgets.ContentWidth(c, 720, func() {
				ui.Column(c).FillWidth().Padding(0, design.PagePadding(WindowWidth)).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(design.SpaceSM).Children(func() {
						m3.Text(c, l10n.PracticeQuestionNumber(p.pos+1), tt.LabelLarge.C(cs.Primary))
						if x.Topic != "" {
							m3.Text(c, x.Topic, tt.LabelSmall.C(cs.OnSecondaryContainer)).Padding(2, design.SpaceSM).
								Background(cs.SecondaryContainer).Radius(design.CornerFull)
						}
					})
					m3.VGap(c, design.SpaceMD)
					st := tt.TitleMedium.H(1.6)
					CoursiveMarkdown{Data: x.Stem, Style: &st, LectureID: q.LectureID, OnRef: onRef}.Show(c)
					m3.VGap(c, design.SpaceXL)
					for i, o := range x.Options {
						state := optionOpen
						switch {
						case !answered:
						case i == x.Answer:
							state = optionCorrect
						case i == chosen:
							state = optionWrong
						default:
							state = optionOther
						}
						ui.Box(c.Key(i)).FillWidth().Padding(0, 0, design.SpaceSM, 0).Children(func() {
							if OptionRow(c, string(rune('A'+i)), o, state, q.LectureID) && !answered {
								p.choose(i)
							}
						})
					}
					if x.Hint != "" && !answered {
						if p.hints[idx] {
							ui.Row(c).FillWidth().Margin(design.SpaceXS, 0, 0, 0).Padding(design.SpaceMD).Gap(design.SpaceSM).
								Background(cs.SurfaceContainerHigh).Radius(design.CornerMD).Children(func() {
								m3.Icon(c, icons.LightbulbOutlineRounded, 18, cs.OnSurfaceVariant)
								ui.Box(c).Grow(1).Basis(0).Children(func() {
									CoursiveMarkdown{Data: x.Hint, Style: &tt.BodyMedium, LectureID: q.LectureID, OnRef: onRef}.Show(c)
								})
							})
						} else if (m3.Button{Kind: m3.TextKind, Icon: icons.LightbulbOutlineRounded, Label: l10n.PracticeHint}).Show(c).Clicked() {
							p.hints[idx] = true
						}
					}
					m3.VGap(c, design.SpaceLG)
					ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
						if answered {
							if (m3.Button{Kind: m3.TextKind, Icon: icons.ForumOutlined, Label: l10n.PracticeExplain}).Show(c).Clicked() {
								p.explain()
							}
							if hasRef && (m3.Button{Kind: m3.TextKind, Icon: icons.TravelExploreRounded, Label: l10n.PracticeSource}).Show(c).Clicked() {
								OpenRef(ref, q.LectureID, nil)
							}
						}
						ui.Box(c).Grow(1)
						if p.pos > 0 && m3.IconBtn(c, icons.ArrowBackRounded, l10n.PracticePrevious).Clicked() {
							p.step(-1)
						}
						m3.HGap(c, design.SpaceXS)
						if p.reviewing && !last {
							if (m3.Button{Kind: m3.Outlined, Label: l10n.PracticeBackToSummary}).Show(c).Clicked() {
								p.reviewing = false
							}
							m3.HGap(c, design.SpaceSM)
						}
						label := l10n.PracticeNext
						if last {
							label = l10n.CommonFinished
							if p.reviewing {
								label = l10n.PracticeBackToSummary
							}
						}
						if (m3.Button{Kind: m3.Filled, Label: label, Disabled: !answered}).Show(c).Clicked() {
							p.step(1)
						}
					})
				})
			})
		})
	})
}

type optionState int

const (
	optionOpen optionState = iota
	optionCorrect
	optionWrong
	optionOther
)

// OptionRow is one answer choice. After answering, the right answer turns
// primary, a wrong choice turns error (both with an icon and a label), and
// every option shows why it is right or wrong. It reports a click while
// open.
func OptionRow(c *ui.Context, letter string, o QuizOption, state optionState, lectureID string) bool {
	cs, tt := design.S(), design.TT()
	bg, fg, border := ui.Color{}, cs.OnSurface, cs.OutlineVariant
	note := ""
	switch state {
	case optionCorrect:
		bg, fg, border, note = cs.PrimaryContainer, cs.OnPrimaryContainer, ui.Color{}, l10n.PracticeCorrectAnswer
	case optionWrong:
		bg, fg, border, note = cs.ErrorContainer, cs.OnErrorContainer, ui.Color{}, l10n.PracticeYourChoice
	case optionOther:
		fg = cs.OnSurfaceVariant
	}
	noteLabel := ""
	if note != "" {
		noteLabel = l10n.PracticeOptionNote(note)
	}
	row := m3.Tappable(c, design.CornerLG).FillWidth().Background(bg).Label(l10n.PracticeOptionLabel(letter, noteLabel)).Disabled(state != optionOpen)
	if border.A > 0 {
		row = row.Border(1, border)
	}
	clicked := row.Clicked()
	onRef := func(r CoursiveRef) { OpenRef(r, lectureID, nil) }
	row.Padding(design.SpaceMD, design.SpaceLG, design.SpaceMD, design.SpaceMD).Children(func() {
		ui.Row(c).FillWidth().Gap(design.SpaceMD).Children(func() {
			badge := func(bg ui.Color, child func()) {
				ui.Box(c).Size(28, 28).Shrink(0).Radius(14).Background(bg).Center().Children(child)
			}
			switch state {
			case optionCorrect:
				badge(cs.Primary, func() { m3.Icon(c, icons.CheckRounded, 18, cs.OnPrimary) })
			case optionWrong:
				badge(cs.Error, func() { m3.Icon(c, icons.CloseRounded, 18, cs.OnError) })
			default:
				badge(cs.SurfaceContainerHighest, func() { m3.Text(c, letter, tt.LabelLarge.C(cs.OnSurfaceVariant)) })
			}
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				st := tt.BodyLarge.C(fg)
				ui.Box(c).FillWidth().Padding(3, 0, 0, 0).Children(func() {
					CoursiveMarkdown{Data: o.Text, Style: &st, LectureID: lectureID, OnRef: onRef}.Show(c)
				})
				if note != "" {
					m3.Text(c, note, tt.LabelMedium.C(fg))
				}
				if state != optionOpen && o.Rationale != "" {
					m3.VGap(c, design.SpaceXS)
					rs := tt.BodySmall.C(design.WithOpacity(fg, .85)).H(1.5)
					CoursiveMarkdown{Data: o.Rationale, Style: &rs, LectureID: lectureID, OnRef: onRef}.Show(c)
				}
			})
		})
	})
	return clicked
}

var plainStem = regexp.MustCompile("[*_`#>$]")
var spaces = regexp.MustCompile(`\s+`)

func (p *PracticePage) summary(c *ui.Context) {
	cs, tt := design.S(), design.TT()
	q := p.quiz
	qs := q.Questions
	right := func(i int) bool { a, ok := p.answers[i]; return ok && a == qs[i].Answer }
	var wrong []int
	for _, i := range p.order {
		if !right(i) {
			wrong = append(wrong, i)
		}
	}
	correct := len(p.order) - len(wrong)
	type tally struct{ r, n int }
	var topics []string
	byTopic := map[string]*tally{}
	for _, i := range p.order {
		t := qs[i].Topic
		if t == "" {
			t = l10n.CommonOther
		}
		if byTopic[t] == nil {
			byTopic[t] = &tally{}
			topics = append(topics, t)
		}
		byTopic[t].n++
		if right(i) {
			byTopic[t].r++
		}
	}
	secs := int(p.finished.Sub(p.started).Seconds())
	elapsed := l10n.DurationSeconds(secs)
	if secs >= 60 {
		elapsed = widgets.FmtMinutes(float64(secs / 60))
		if secs%60 != 0 {
			elapsed += l10n.PracticeSecondsSuffix(secs % 60)
		}
	}
	retake := p.only != nil
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(design.SpaceXXL, 0, design.SpaceXXXL, 0).Children(func() {
			widgets.ContentWidth(c, 720, func() {
				ui.Column(c).FillWidth().Padding(0, design.PagePadding(WindowWidth)).Children(func() {
					head := l10n.PracticeDone
					if retake {
						head = l10n.PracticeRetryDone
					}
					m3.Text(c, head, tt.TitleMedium.C(cs.OnSurfaceVariant))
					m3.VGap(c, design.SpaceSM)
					ui.RichText(c, tt.DisplayMedium.C(cs.OnSurface).Span(strconv.Itoa(correct)),
						tt.HeadlineSmall.C(cs.OnSurfaceVariant).Span(" / "+strconv.Itoa(len(p.order)))).FontFeatures("tnum").
						Label(l10n.PracticeScoreSemantics(correct, len(p.order)))
					m3.VGap(c, design.SpaceXS)
					parts := []string{l10n.PracticeAccuracy(int(math.Round(float64(correct) * 100 / float64(max(1, len(p.order)))))), l10n.PracticeTime(elapsed)}
					switch {
					case p.saving:
						parts = append(parts, l10n.CommonSavingShort)
					case !retake && p.best != nil:
						parts = append(parts, l10n.PracticeBest(p.best.Label()))
					}
					m3.Text(c, strings.Join(parts, " · "), tt.BodyMedium.C(cs.OnSurfaceVariant)).FontFeatures("tnum")
					m3.VGap(c, design.SpaceXL)
					ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
						if len(wrong) > 0 {
							if (m3.Button{Kind: m3.Outlined, Icon: icons.ReplayRounded, Label: l10n.PracticeRetryWrong(len(wrong))}).Show(c).Clicked() {
								m3.RootNav.Replace(newPractice(q.ID, wrong))
							}
							if !retake && (m3.Button{Kind: m3.Outlined, Icon: icons.AutoAwesomeOutlined, Label: l10n.PracticeMoreOnWrong}).Show(c).Clicked() {
								AskInSheet(l10n.PromptRetryWrong(q.Title, q.ID))
							}
						}
						if m3.FilledButton(c, l10n.CommonFinished).Clicked() {
							m3.RootNav.Pop()
						}
					})
					if len(topics) > 1 {
						m3.VGap(c, design.SpaceXXL)
						m3.Text(c, l10n.PracticeByTopic, tt.TitleSmall)
						m3.VGap(c, design.SpaceSM)
						for _, t := range topics {
							n := byTopic[t]
							ui.Row(c.Key(t)).FillWidth().AlignItems(ui.Center).Padding(design.SpaceXS, 0).Children(func() {
								if n.r == n.n {
									m3.Icon(c, icons.CheckCircleOutlineRounded, 18, cs.Primary)
								} else {
									m3.Icon(c, icons.ErrorOutlineRounded, 18, cs.Error)
								}
								m3.HGap(c, design.SpaceSM)
								m3.Text(c, t, tt.BodyMedium).Grow(1).Basis(0)
								m3.Text(c, strconv.Itoa(n.r)+"/"+strconv.Itoa(n.n), tt.BodyMedium.C(cs.OnSurfaceVariant)).FontFeatures("tnum")
							})
						}
					}
					m3.VGap(c, design.SpaceXL)
					m3.Text(c, l10n.PracticeReview, tt.TitleSmall)
					m3.VGap(c, design.SpaceXS)
					for n, i := range p.order {
						stem := strings.TrimSpace(spaces.ReplaceAllString(plainStem.ReplaceAllString(qs[i].Stem, ""), " "))
						t := m3.Tile{Title: strconv.Itoa(n+1) + ". " + stem, TitleLines: 2, Tap: true, Pad: true, PadX: 0,
							TrailingIcon: icons.ChevronRightRounded, Leading: func() {
								_, has := p.answers[i]
								switch {
								case right(i):
									m3.Icon(c, icons.CheckCircleOutlineRounded, 24, cs.Primary).Label(l10n.PracticeCorrect)
								case !has:
									m3.Icon(c, icons.CancelOutlined, 24, cs.Error).Label(l10n.PracticeUnanswered)
								default:
									m3.Icon(c, icons.CancelOutlined, 24, cs.Error).Label(l10n.PracticeWrong)
								}
							}}
						if t.Show(c.Key(n)).Clicked() {
							p.reviewing = true
							for k, o := range p.order {
								if o == i {
									p.pos = k
								}
							}
						}
					}
				})
			})
		})
	})
}
