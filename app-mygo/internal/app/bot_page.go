package app

import (
	"regexp"
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

// bot_page.dart: the resident bot's page: who it is, what it remembers
// and how it learns (docs/specs/0007-resident-bot-and-course-agents.md).

type memoryKind struct {
	prefix, label string
	icon          icons.Icon
}

var memoryKinds = []memoryKind{
	{"profile.md", l10n.MemoryAboutYou, icons.PersonOutlineRounded},
	{"preferences.md", l10n.MemoryPreferences, icons.TuneRounded},
	{"routines/", l10n.MemoryRoutines, icons.RouteOutlined},
	{"courses/", l10n.CommonCourses, icons.SchoolOutlined},
	{"notes/", l10n.CommonNotes, icons.StickyNote2Outlined},
}

func memoryKindOf(path string) (string, icons.Icon) {
	for _, k := range memoryKinds {
		if path == k.prefix || (strings.HasSuffix(k.prefix, "/") && strings.HasPrefix(path, k.prefix)) {
			return k.label, k.icon
		}
	}
	return l10n.CommonOther, icons.DescriptionOutlined
}

// BotPage is the bot's page.
type BotPage struct {
	name, persona string
	nameFocused   bool
	synced        bool
	files, runs   []core.J
	loading       bool
	err           error
	preview       *BotMood
	previewAt     time.Time
	memoryVer     int
	scroll        ui.ScrollState
}

// NewBotPage opens the bot's page.
func NewBotPage() m3.Page {
	p := &BotPage{loading: true, memoryVer: Assistant.MemoryChanged}
	p.load()
	if len(Assistant.Bot) == 0 {
		Assistant.LoadBot(false)
	}
	return p
}

func (p *BotPage) syncFields() {
	p.name = Assistant.BotName()
	p.persona = Assistant.Bot.S("persona")
}

func (p *BotPage) load() {
	a := Assistant
	pending := 2
	var firstErr error
	finish := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if pending--; pending == 0 {
			p.loading, p.err = false, firstErr
		}
	}
	a.MemoryFiles(func(f []core.J, err error) {
		if err == nil {
			p.files = f
		}
		finish(err)
	})
	a.MemoryRuns(func(r []core.J, err error) {
		if err == nil {
			p.runs = r
		}
		finish(err)
	})
}

func (p *BotPage) patch(body core.J) {
	Assistant.UpdateBot(body, func(err error) {
		if err != nil {
			widgets.ShowSnack(apiMessage(err), "", nil)
			p.syncFields()
			return
		}
		p.flash(MoodHappy)
	})
}

func (p *BotPage) flash(m BotMood) {
	p.preview = &m
	p.previewAt = time.Now()
}

// View builds the page.
func (p *BotPage) View(c *ui.Context) {
	a := Assistant
	if !p.synced && len(a.Bot) > 0 {
		p.synced = true
		p.syncFields()
	}
	if a.MemoryChanged != p.memoryVer {
		p.memoryVer = a.MemoryChanged
		p.load()
	}
	if p.preview != nil {
		if d := time.Since(p.previewAt); d >= 1800*time.Millisecond {
			p.preview = nil
		} else {
			c.After(1800*time.Millisecond - d)
		}
	}
	w, _ := c.Size()
	pad := design.PagePadding(w)
	m3.Scaffold{
		AppBar: &m3.AppBar{Title: a.BotName(), Scrolled: p.scroll.Y > 0},
		Body: func() {
			ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
				ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
					ui.Column(c).FillWidth().MaxWidth(760).Padding(0, pad, design.SpaceXXL, pad).Children(func() { p.body(c) })
				})
			})
		},
	}.Show(c)
}

func botSection(c *ui.Context, title string, trailing func()) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceXL, 0, design.SpaceSM, 0).Children(func() {
		m3.Text(c, title, design.TT().TitleMedium).Grow(1).Basis(0)
		if trailing != nil {
			trailing()
		}
	})
}

func (p *BotPage) body(c *ui.Context) {
	a := Assistant
	cs := design.S()
	tt := design.TT()
	memory := a.Bot.O("memory")
	p.hero(c)
	botSection(c, l10n.BotSectionLook, nil)
	in := m3.Field{Label: l10n.BotName, MaxLength: 12, Counter: true}.Show(c, &p.name)
	focused := in.Focused()
	name := strings.TrimSpace(p.name)
	if in.Submitted() || (p.nameFocused && !focused && name != a.BotName() && name != "") {
		p.patch(core.J{"name": p.name})
	}
	p.nameFocused = focused
	m3.VGap(c, design.SpaceSM)
	ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
		for _, s := range BotShapes {
			s := s
			if shapeOption(c, s, a.BotColor(), a.BotShape() == s.Key) {
				p.patch(core.J{"shape": s.Key})
			}
		}
	})
	m3.VGap(c, design.SpaceLG)
	m3.Text(c, l10n.CommonColor, tt.LabelLarge.C(cs.OnSurfaceVariant))
	m3.VGap(c, design.SpaceSM)
	ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
		for _, k := range BotColorKeys {
			k := k
			body, _ := BotColors(k)
			if (m3.Chip{Kind: m3.ChoiceChip, Label: BotColorLabel(k), Selected: a.BotColor() == k, AvatarFn: func(ui.Color) {
				ui.Box(c).Size(16, 16).Radius(8).Background(body)
			}}).Show(c.Key("color-" + k)).Clicked() {
				p.patch(core.J{"color": k})
			}
		}
	})
	botSection(c, l10n.BotSectionPersona, nil)
	m3.Field{Hint: l10n.BotPersonaHint, MaxLength: 500, Counter: true, MinLines: 2, MaxLines: 5}.Show(c, &p.persona)
	ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
		if (m3.Button{Kind: m3.Tonal, Label: l10n.CommonSave, Disabled: strings.TrimSpace(p.persona) == a.Bot.S("persona")}).Show(c).Clicked() {
			p.patch(core.J{"persona": p.persona})
		}
	})
	botSection(c, l10n.BotSectionMemory, func() {
		if (m3.IconButton{Icon: icons.AddRounded, Tooltip: l10n.MemoryAdd}).Show(c).Clicked() {
			p.edit("")
		}
		m3.HGap(c, design.SpaceXS)
		b := m3.Button{Kind: m3.Tonal, Icon: icons.BedtimeOutlined, Label: l10n.MemoryConsolidateNow, Disabled: a.Consolidating}
		if a.Consolidating {
			b.Icon, b.Label = 0, l10n.MemoryConsolidating
		}
		if b.Show(c).Clicked() {
			a.ConsolidateNow(func(err error) {
				if err != nil {
					widgets.ShowSnack(apiMessage(err), "", nil)
				}
			})
		}
	})
	m3.Text(c, l10n.MemoryIntro(a.BotName()), tt.BodySmall.C(cs.OnSurfaceVariant).H(1.6))
	m3.VGap(c, design.SpaceMD)
	switch {
	case p.loading:
		ui.Box(c).FillWidth().Padding(design.SpaceXL).Center().Children(func() { m3.Circular(c, m3.Progress{}) })
	case p.err != nil:
		m3.Text(c, l10n.MemoryReadFailed(apiMessage(p.err)), tt.BodySmall.C(cs.Error))
	case len(p.files) == 0:
		ui.Box(c).FillWidth().Padding(design.SpaceLG).Background(cs.SurfaceContainer).Radius(design.CornerLG).Children(func() {
			m3.Text(c, l10n.MemoryEmpty(a.BotName()), tt.BodyMedium.C(cs.OnSurfaceVariant))
		})
	default:
		p.memoryList(c)
	}
	botSection(c, l10n.MemoryRuns, nil)
	if len(p.runs) == 0 {
		m3.Text(c, l10n.MemoryRunsEmpty, tt.BodySmall.C(cs.OnSurfaceVariant))
	}
	for i, r := range p.runs {
		if i >= 20 {
			break
		}
		runTile(c.Key(i), r)
	}
	botSection(c, l10n.PluginCategoryTutor, nil)
	auto := !memory.Has("auto") || memory["auto"] != false
	if m3.SwitchTile(c, m3.Tile{Title: l10n.MemoryAuto, Subtitle: l10n.MemoryAutoHint, Pad: true}, &auto) {
		p.patch(core.J{"memory": core.J{"auto": auto}})
	}
	learn := memory.IsTrue("learn_from_courses")
	if m3.SwitchTile(c, m3.Tile{Title: l10n.MemoryFromCourses, Subtitle: l10n.MemoryFromCoursesHint, Pad: true}, &learn) {
		p.patch(core.J{"memory": core.J{"learn_from_courses": learn}})
	}
	m3.VGap(c, design.SpaceLG)
	ui.Row(c).Children(func() {
		if (m3.Button{Kind: m3.Outlined, Icon: icons.RestartAltRounded, Label: l10n.BotRestartChat}).Show(c).Clicked() {
			m3.Confirm(l10n.BotRestartTitle, l10n.BotRestartBody(a.BotName()), l10n.CommonCancel, l10n.BotRestart, false, func() { a.ResetBot(nil) })
		}
	})
}

func (p *BotPage) hero(c *ui.Context) {
	a := Assistant
	cs := design.S()
	tt := design.TT()
	mood := a.Mood()
	if p.preview != nil {
		mood = *p.preview
	}
	ui.Column(c).FillWidth().Margin(design.SpaceLG, 0, 0, 0).Padding(design.SpaceXL, design.SpaceLG).Background(cs.SurfaceContainerHigh).
		Radius(design.CornerXLIncreased).AlignItems(ui.Center).Children(func() {
		b := ui.ButtonBase(c).FocusRing(false).Cursor(ui.CursorPointer).Label(l10n.BotMoodLabel(a.BotName(), mood.Label()))
		b.Children(func() {
			BotAvatar{Size: 132, Shape: a.BotShape(), Color: a.BotColor(), Mood: mood, FollowPointer: true}.Show(c)
		})
		if b.LongPressed() || b.RightClicked() {
			p.flash(MoodConfused)
		} else if b.Clicked() {
			p.flash(MoodHappy)
		}
		m3.VGap(c, design.SpaceMD)
		m3.Text(c, a.BotName(), tt.HeadlineSmall)
		m3.VGap(c, design.SpaceXS)
		parts := []string{l10n.MemoryEmptyShort}
		if len(p.files) > 0 {
			parts[0] = l10n.MemoryCount(len(p.files))
		}
		if a.Consolidating {
			parts = append(parts, l10n.MemoryWorking)
		} else if last := a.Bot.O("last_run"); last != nil && last["at"] != nil {
			parts = append(parts, l10n.MemoryLastRun(core.FmtRelative(time.UnixMilli(int64(last.F("at"))))))
		}
		m3.Text(c, strings.Join(parts, " · "), tt.BodyMedium.C(cs.OnSurfaceVariant))
	})
}

// shapeOption is one body in the picker; it reports a tap.
func shapeOption(c *ui.Context, s BotShape, color string, selected bool) bool {
	cs := design.S()
	tt := design.TT()
	bg, fg, r := cs.SurfaceContainer, cs.OnSurfaceVariant, design.CornerLG
	if selected {
		bg, fg, r = cs.SecondaryContainer, cs.OnSecondaryContainer, design.CornerXL
	}
	b := m3.Tappable(c.Key("shape-"+s.Key), design.CornerLG).Width(80).PaddingY(design.SpaceSM).Background(bg).
		Label(s.Label).Selected(selected).Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Colors: true})
	b = b.Radius(b.AnimateTo("shape.radius", r, design.T().Motion(design.Medium2, design.Emphasized)))
	b.Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
			BotAvatar{Size: 48, Shape: s.Key, Color: color, Still: !selected}.Show(c)
			m3.VGap(c, design.SpaceXS)
			m3.Text(c, s.Label, tt.LabelMedium.C(fg))
		})
	})
	return b.Clicked()
}

func (p *BotPage) memoryList(c *ui.Context) {
	cs := design.S()
	var order []string
	groups := map[string][]core.J{}
	for _, f := range p.files {
		k, _ := memoryKindOf(f.S("path"))
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], f)
	}
	for _, g := range order {
		m3.Text(c.Key("mg-"+g), g, design.TT().LabelLarge.C(cs.Primary)).Padding(design.SpaceSM, 0, design.SpaceXS, 0)
		for _, f := range groups[g] {
			p.memoryTile(c, f)
		}
	}
}

func (p *BotPage) memoryTile(c *ui.Context, f core.J) {
	cs := design.S()
	path := f.S("path")
	title := f.S("title")
	if title == "" && strings.HasPrefix(path, "courses/") {
		if co, ok := State.Course(strings.TrimSuffix(strings.TrimPrefix(path, "courses/"), ".md")); ok {
			title = co.Title()
		}
	}
	if title == "" {
		title = path
	}
	var sub []string
	if d := f.S("description"); d != "" {
		sub = append(sub, d)
	}
	if w := f.S("when"); w != "" {
		sub = append(sub, l10n.MemoryWhen(w))
	}
	sub = append(sub, core.FmtRelative(time.UnixMilli(int64(f.F("updated")))))
	_, ic := memoryKindOf(path)
	ui.Column(c.Key("m-"+path)).FillWidth().Margin(0, 0, design.SpaceSM, 0).Background(cs.SurfaceContainer).Radius(design.CornerMD).Clip().Children(func() {
		if (m3.Tile{LeadingIcon: ic, IconColor: cs.OnSurfaceVariant, Title: title, TitleLines: 1, Subtitle: strings.Join(sub, " · "), SubtitleLines: 2, Tap: true,
			Trailing: func() {
				if (m3.IconButton{Icon: icons.DeleteOutlineRounded, Tooltip: l10n.CommonDelete}).Show(c).Clicked() {
					p.delete(path, title)
				}
			}}).Show(c).Clicked() {
			p.edit(path)
		}
	})
}

func (p *BotPage) delete(path, title string) {
	m3.Confirm(l10n.MemoryDeleteTitle, l10n.MemoryDeleteBody(title), l10n.CommonCancel, l10n.CommonDelete, false, func() {
		Assistant.DeleteMemory(path, func(err error) {
			if err != nil {
				widgets.ShowSnack(apiMessage(err), "", nil)
			}
		})
	})
}

func (p *BotPage) edit(path string) {
	if path == "" {
		showMemoryEditor("", "", "")
		return
	}
	Assistant.ReadMemory(path, func(f core.J, err error) {
		if err != nil {
			widgets.ShowSnack(apiMessage(err), "", nil)
			return
		}
		showMemoryEditor(path, f.S("content"), f.S("hash"))
	})
}

func runTile(c *ui.Context, run core.J) {
	cs := design.S()
	tt := design.TT()
	changes := run.Os("changes")
	at := time.UnixMilli(int64(run.F("at")))
	title, ic, col := l10n.MemoryRunNothing, icons.CheckRounded, cs.OnSurfaceVariant
	switch run.S("status") {
	case "ok":
		title, ic, col = l10n.MemoryRunChanged(len(changes)), icons.TaskAltRounded, cs.Primary
	case "error":
		title, ic, col = l10n.MemoryRunUnfinished, icons.ErrorOutlineRounded, cs.Error
	}
	meta := []string{core.FmtRelative(at), l10n.MemoryRunScope(run.I("sessions"), run.I("turns"))}
	if cost := run.F("cost"); cost > 0 {
		meta = append(meta, core.FmtUsd(cost))
	}
	ui.Row(c).FillWidth().AlignItems(ui.Start).PaddingY(design.SpaceSM).Gap(design.SpaceLG).Children(func() {
		m3.Icon(c, ic, 24, col)
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			m3.Text(c, title, tt.BodyLarge)
			m3.Text(c, strings.Join(meta, " · "), tt.BodyMedium.C(cs.OnSurfaceVariant))
			if s := run.S("summary"); s != "" {
				m3.Clamp(m3.Text(c, s, tt.BodySmall.C(cs.OnSurfaceVariant)), 3)
			}
			if len(changes) > 0 {
				ui.Row(c).FillWidth().Wrap().Gap(design.SpaceXS).Padding(design.SpaceXS, 0, 0, 0).Children(func() {
					for i, ch := range changes {
						act := map[string]string{"created": l10n.CommonNew, "updated": l10n.MemoryUpdated, "appended": l10n.MemoryAppended}[ch.S("action")]
						if act == "" {
							act = ch.S("action")
						}
						m3.Chip{Kind: m3.PlainChip, Label: act + " " + ch.S("path"), Compact: true}.Show(c.Key(i))
					}
				})
			}
		})
	})
}

var memorySlug = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// showMemoryEditor edits one memory file; a new file (empty path) picks
// its kind and name first.
func showMemoryEditor(path, content, hash string) {
	text, slug, kind := content, "", "notes/"
	saving, errText := false, ""
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true, MaxWidth: 720}, func(c *ui.Context, r *m3.Route) {
		cs := design.S()
		tt := design.TT()
		target := func() string {
			if path != "" {
				return path
			}
			if !strings.HasSuffix(kind, "/") {
				return kind
			}
			if s := strings.TrimSpace(slug); memorySlug.MatchString(s) {
				return kind + s + ".md"
			}
			return ""
		}
		save := func() {
			pth := target()
			if pth == "" {
				errText = l10n.MemoryFileNameInvalid
				return
			}
			saving, errText = true, ""
			Assistant.WriteMemory(pth, text, hash, func(err error) {
				if err != nil {
					saving = false
					errText = strings.SplitN(apiMessage(err), "\n", 2)[0]
					return
				}
				r.Close()
			})
		}
		ui.Column(c).FillWidth().Padding(0, design.SpaceXL, design.SpaceLG, design.SpaceXL).Children(func() {
			title := path
			if title == "" {
				title = l10n.MemoryAdd
			}
			m3.Text(c, title, tt.TitleMedium)
			m3.VGap(c, design.SpaceMD)
			if path == "" {
				var segs []m3.Segment[string]
				for _, k := range memoryKinds {
					segs = append(segs, m3.Segment[string]{Value: k.prefix, Label: k.label, Icon: k.icon})
				}
				m3.Segmented(c, segs, &kind, m3.SegOpts{NoCheck: true})
				if strings.HasSuffix(kind, "/") {
					m3.VGap(c, design.SpaceMD)
					m3.Field{Label: l10n.MemoryFileName, PrefixText: kind, SuffixText: ".md"}.Show(c, &slug)
				}
				m3.VGap(c, design.SpaceMD)
			}
			mono := tt.BodyMedium.F("monospace").H(1.5)
			m3.Field{Hint: l10n.MemoryFileHint, MinLines: 8, MaxLines: 18, Style: &mono}.Show(c, &text)
			if errText != "" {
				m3.Text(c, errText, tt.BodySmall.C(cs.Error)).Margin(design.SpaceSM, 0, 0, 0)
			}
			m3.VGap(c, design.SpaceMD)
			ui.Row(c).FillWidth().Justify(ui.End).Gap(design.SpaceSM).Children(func() {
				if m3.TextButton(c, l10n.CommonCancel).Clicked() {
					r.Close()
				}
				label := l10n.CommonSave
				if saving {
					label = l10n.CommonSaving
				}
				if (m3.Button{Kind: m3.Filled, Label: label, Disabled: saving}).Show(c).Clicked() {
					save()
				}
			})
		})
	})
}
