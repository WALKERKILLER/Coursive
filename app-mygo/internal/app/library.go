package app

import (
	"regexp"
	"slices"
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

// library_page.dart and catalog_browser.dart: the library of every
// course's files, the full-text search of transcripts and notes, and the
// practice sets.

// CatalogCategoryLabel is catalogCategoryLabel.
func CatalogCategoryLabel(category string) string {
	switch category {
	case "courseware":
		return l10n.CatalogCourseware
	case "homework":
		return l10n.CatalogHomework
	case "lab":
		return l10n.CatalogLab
	case "transcript":
		return l10n.CommonTranscript
	case "video":
		return l10n.CatalogVideo
	case "slides":
		return l10n.CatalogSlides
	case "notes":
		return l10n.CommonNotes
	case "reference":
		return l10n.CatalogReference
	}
	return l10n.CatalogOther
}

var catalogCategories = []string{"courseware", "homework", "lab", "transcript", "video", "slides", "notes", "reference", "other"}

// LibraryPage is the library destination.
type LibraryPage struct {
	query     string
	items     []core.J
	library   core.J
	courseID  string
	err       error
	searching bool
	hits      []core.Hit
	version   int
	debounce  *time.Timer
	category  string
	fileType  string
	quizzes   *QuizList
	cancel    func()
	scroll    ui.ScrollState
}

// NewLibraryPage loads the catalog and follows note and lecture changes.
func NewLibraryPage() m3.Page {
	p := &LibraryPage{quizzes: NewQuizList("")}
	p.load()
	p.cancel = State.Events.Listen(func(e core.J) {
		if t := e.S("type"); t == "notes_changed" || t == "lecture_changed" {
			p.load()
		}
	})
	return p
}

// Dispose stops listening.
func (p *LibraryPage) Dispose() {
	p.cancel()
	p.quizzes.Dispose()
	if p.debounce != nil {
		p.debounce.Stop()
	}
}

// MainScroll is the page's scroll, which shrinks the dock.
func (p *LibraryPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *LibraryPage) load() {
	type data struct {
		items   []core.J
		library core.J
	}
	core.Async(func() (data, error) {
		api := State.Api()
		items, err := api.LibraryCatalog("")
		if err != nil {
			return data{}, err
		}
		lib, err := api.Get("/api/library")
		if err != nil {
			return data{}, err
		}
		return data{core.Objs(items), core.Obj(lib)}, nil
	}, func(d data, err error) {
		if err != nil {
			p.err = err
			return
		}
		p.items, p.library, p.err = d.items, d.library, nil
	})
}

func (p *LibraryPage) search() {
	p.version++
	version := p.version
	q := strings.TrimSpace(p.query)
	if q == "" {
		p.hits, p.searching = nil, false
		return
	}
	p.searching = true
	params := map[string]string{"q": q, "limit": "120"}
	if p.courseID != "" {
		params["course_id"] = p.courseID
	}
	core.Async(func() (any, error) { return State.Api().Get("/api/search", params) }, func(r any, err error) {
		if version != p.version {
			return
		}
		p.searching = false
		if err != nil {
			widgets.ShowSnack(l10n.SearchFailed(err), "", nil)
			return
		}
		p.hits = nil
		for _, j := range core.Objs(r) {
			p.hits = append(p.hits, core.Hit{J: j})
		}
	})
}

func (p *LibraryPage) queried() {
	p.version++
	if p.debounce != nil {
		p.debounce.Stop()
	}
	p.debounce = time.AfterFunc(220*time.Millisecond, func() { core.Post(p.search) })
}

func (p *LibraryPage) open(item core.J) {
	if id := item.S("note_id"); id != "" {
		m3.Nav().Push(NewNotePage(id))
		return
	}
	OpenLibraryFile(item.S("path"))
}

func (p *LibraryPage) reindex() {
	core.Async(func() (any, error) { return State.Api().Post("/api/library/reindex", nil) }, func(r any, err error) {
		if err != nil {
			widgets.ShowSnack(l10n.LibraryReindexFailed(err), "", nil)
			return
		}
		widgets.ShowSnack(l10n.LibraryReindexed(core.Obj(r)["indexed"]), "", nil)
		p.load()
	})
}

// View builds the page.
func (p *LibraryPage) View(c *ui.Context) {
	cs, tt := design.S(), design.TT()
	ReportScreen(map[string]string{"screen": l10n.ScreenLibrary})
	pad := design.SpaceXL
	if Compact() {
		pad = design.SpaceLG
	}
	padded := func(key any, child func()) {
		widgets.ContentWidth(c.Key(key), 0, func() {
			ui.Column(c).FillWidth().Padding(design.SpaceMD, pad).Children(child)
		})
	}
	q := strings.TrimSpace(p.query)
	m3.Scaffold{BG: ui.Transparent, FABInset: BottomInset,
		FAB: func() {
			if (m3.FAB{Icon: icons.EditNoteRounded, Label: l10n.LibraryWriteNote}).Show(c).Clicked() {
				m3.Nav().Push(NewNotePage(""))
			}
		},
		Body: func() {
			ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
				padded("head", func() {
					m3.VGap(c, design.SpaceLG)
					m3.Text(c, l10n.ScreenLibrary, tt.HeadlineLarge)
					m3.VGap(c, design.SpaceLG)
					in := m3.SearchBar{Hint: l10n.CatalogSearchHint, MaxWidth: -1,
						Leading: func() {
							if p.searching {
								m3.Spinner(c, 20, 2, cs.Primary)
							} else {
								m3.Icon(c, icons.ManageSearchRounded, 24, cs.OnSurface)
							}
						},
						Trailing: func() {
							if p.query != "" && m3.IconBtn(c, icons.CloseRounded, "").Clicked() {
								p.query = ""
								p.queried()
								p.search()
							}
						}}.Show(c, &p.query)
					if in.Changed() {
						p.queried()
					}
					if in.Submitted() {
						p.search()
					}
				})
				if p.err != nil {
					padded("err", func() {
						ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
							m3.Text(c, l10n.CatalogLoadFailed(p.err.Error()), tt.BodyMedium)
							if m3.TextButton(c, l10n.CommonRetry).Clicked() {
								p.load()
							}
						})
					})
				}
				if p.items == nil && p.err == nil {
					m3.Linear(c, m3.Progress{})
				}
				if p.items != nil {
					padded("catalog", func() { p.catalog(c) })
				}
				if q != "" && p.hits != nil {
					padded("hits", func() { m3.Text(c, l10n.CatalogFulltextTitle(len(p.hits)), tt.TitleLarge) })
					for i, h := range p.hits {
						padded(i, func() {
							if hitTile(c, h) {
								if h.Kind() == "note" {
									m3.Nav().Push(NewNotePage(h.NoteID()))
								} else {
									m3.RootNav.Push(NewPlayerPage(h.LectureID(), h.StartMs()))
								}
							}
						})
					}
				}
				if q == "" {
					padded("quizzes", func() {
						cols := 3
						if Compact() {
							cols = 1
						}
						p.quizzes.Show(c, cols, func() { m3.Text(c, l10n.CommonPractice, tt.TitleLarge) }, nil)
					})
				}
				if p.library != nil {
					padded("stats", func() { p.stats(c) })
				}
				ui.Box(c).Height(96 + BottomInset)
			})
		}}.Show(c)
}

func (p *LibraryPage) stats(c *ui.Context) {
	tt := design.TT()
	st := p.library.O("stats")
	m3.Card(c, m3.CardElevated).FillWidth().Padding(design.SpaceLG).Children(func() {
		m3.Text(c, l10n.LibraryStatLocal, tt.TitleSmall)
		m3.Text(c, core.Str(st["local_lectures"])+" · "+core.FmtBytes(st.F("bytes")), tt.BodyMedium)
		ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
			if core.CanOpenFolders() && (m3.Button{Kind: m3.TextKind, Icon: icons.FolderOpenOutlined, Label: l10n.CommonOpenFolder}).Show(c).Clicked() {
				core.OpenPath(p.library.S("root"))
			}
			if (m3.Button{Kind: m3.TextKind, Icon: icons.ManageHistoryRounded, Label: l10n.LibraryReindex}).Show(c).Clicked() {
				p.reindex()
			}
		})
	})
}

// catalog is CatalogBrowser: the files by course and use, filtered by
// course, category and file type.
func (p *LibraryPage) catalog(c *ui.Context) {
	cs, tt := design.S(), design.TT()
	type course struct{ id, title string }
	var courses []course
	for _, i := range p.items {
		if !slices.ContainsFunc(courses, func(x course) bool { return x.id == i.S("course_id") }) {
			courses = append(courses, course{i.S("course_id"), i.S("course")})
		}
	}
	var scoped []core.J
	for _, i := range p.items {
		if p.courseID == "" || i.S("course_id") == p.courseID {
			scoped = append(scoped, i)
		}
	}
	var types []string
	for _, i := range scoped {
		if t := i.S("file_type"); !slices.Contains(types, t) {
			types = append(types, t)
		}
	}
	slices.Sort(types)
	q := strings.ToLower(strings.TrimSpace(p.query))
	var visible []core.J
	for _, i := range scoped {
		if (p.category == "" || i.S("category") == p.category) && (p.fileType == "" || i.S("file_type") == p.fileType) &&
			(q == "" || strings.Contains(strings.ToLower(i.S("name")+" "+i.S("lecture")+" "+i.S("course")+" "+i.S("path")), q)) {
			visible = append(visible, i)
		}
	}
	type group struct {
		category string
		items    []core.J
	}
	type byCourse struct {
		title  string
		groups []*group
	}
	var grouped []*byCourse
	for _, i := range visible {
		idx := slices.IndexFunc(grouped, func(g *byCourse) bool { return g.title == i.S("course") })
		if idx < 0 {
			grouped = append(grouped, &byCourse{title: i.S("course")})
			idx = len(grouped) - 1
		}
		gc := grouped[idx]
		gi := slices.IndexFunc(gc.groups, func(g *group) bool { return g.category == i.S("category") })
		if gi < 0 {
			gc.groups = append(gc.groups, &group{category: i.S("category")})
			gi = len(gc.groups) - 1
		}
		gc.groups[gi].items = append(gc.groups[gi].items, i)
	}
	width := float32(220)
	if Compact() {
		width = 0
	}
	m3.Text(c, l10n.CatalogTitle(len(visible)), tt.TitleLarge)
	m3.VGap(c, design.SpaceMD)
	ui.Row(c).FillWidth().Wrap().Gap(design.SpaceMD).Children(func() {
		box := func(key string, child func()) {
			b := ui.Box(c.Key(key))
			if width > 0 {
				b = b.Width(width)
			} else {
				b = b.FillWidth()
			}
			b.Children(child)
		}
		box("course", func() {
			entries := []m3.DropdownEntry[string]{{Value: "", Label: l10n.LibraryAllCourses}}
			for _, co := range courses {
				entries = append(entries, m3.DropdownEntry[string]{Value: co.id, Label: co.title})
			}
			if m3.Dropdown(c, "course", m3.DropdownOpts{Label: l10n.CommonCourses}, entries, &p.courseID) {
				p.fileType = ""
				p.search()
			}
		})
		box("category", func() {
			entries := []m3.DropdownEntry[string]{{Value: "", Label: l10n.CatalogAllCategories}}
			for _, k := range catalogCategories {
				n := 0
				for _, i := range scoped {
					if i.S("category") == k {
						n++
					}
				}
				entries = append(entries, m3.DropdownEntry[string]{Value: k, Label: CatalogCategoryLabel(k) + " (" + itoa(n) + ")"})
			}
			m3.Dropdown(c, "category", m3.DropdownOpts{Label: l10n.CatalogCategory}, entries, &p.category)
		})
		box("type", func() {
			entries := []m3.DropdownEntry[string]{{Value: "", Label: l10n.CatalogAllTypes}}
			for _, t := range types {
				entries = append(entries, m3.DropdownEntry[string]{Value: t, Label: strings.ToUpper(t)})
			}
			if !slices.Contains(types, p.fileType) {
				p.fileType = ""
			}
			m3.Dropdown(c, "type", m3.DropdownOpts{Label: l10n.CatalogFileType}, entries, &p.fileType)
		})
	})
	m3.VGap(c, design.SpaceSM)
	m3.Text(c, l10n.CatalogClassificationHint, tt.BodySmall.C(cs.OnSurfaceVariant))
	m3.VGap(c, design.SpaceLG)
	if len(visible) == 0 {
		m3.Text(c, l10n.CatalogEmpty, tt.BodyMedium).Padding(design.SpaceXL)
	}
	filter := p.courseID + ":" + p.category + ":" + p.fileType + ":" + q
	for _, gc := range grouped {
		ui.Column(c.Key(gc.title)).FillWidth().Children(func() {
			m3.Text(c, gc.title, tt.TitleMedium)
			m3.VGap(c, design.SpaceSM)
			for _, g := range gc.groups {
				ic := icons.FolderOutlined
				switch g.category {
				case "lab":
					ic = icons.ScienceOutlined
				case "homework":
					ic = icons.AssignmentOutlined
				}
				items := g.items
				m3.Card(c.Key(g.category), m3.CardElevated).FillWidth().Margin(0, 0, design.SpaceSM, 0).Clip().Children(func() {
					m3.Expansion{Tile: m3.Tile{LeadingIcon: ic, Title: CatalogCategoryLabel(g.category) + " · " + itoa(len(items))},
						Initially: g.category != "slides" && g.category != "video",
						Children: func() {
							for k, item := range items {
								sub := []string{}
								if l := item.S("lecture"); l != "" {
									sub = append(sub, l)
								}
								sub = append(sub, core.FmtBytes(item.F("bytes")))
								t := m3.Tile{Dense: true, Tap: true, Title: item.S("name"), Subtitle: strings.Join(sub, " · "),
									Leading: func() {
										m3.Text(c, strings.ToUpper(item.S("file_type")), tt.LabelSmall.C(cs.Primary)).Width(50)
									},
									Trailing: func() { m3.Icon(c, icons.OpenInNewRounded, 18, cs.OnSurfaceVariant) }}
								if t.Show(c.Key(k)).Clicked() {
									p.open(item)
								}
							}
						}}.Show(c, filter+":"+gc.title+":"+g.category)
				})
			}
			m3.VGap(c, design.SpaceLG)
		})
	}
}

var hitMark = regexp.MustCompile(`«(.*?)»`)

// hitTile is _HitTile: a search hit with its matches marked. It reports a
// click.
func hitTile(c *ui.Context, h core.Hit) bool {
	cs, tt := design.S(), design.TT()
	base := tt.BodyMedium.H(1.55)
	var spans []ui.Span
	at := 0
	snippet := h.Snippet()
	for _, m := range hitMark.FindAllStringSubmatchIndex(snippet, -1) {
		if m[0] > at {
			spans = append(spans, base.Span(snippet[at:m[0]]))
		}
		mark := base.C(cs.OnTertiaryContainer).W(600).Span(snippet[m[2]:m[3]])
		mark.Background = cs.TertiaryContainer
		spans = append(spans, mark)
		at = m[1]
	}
	spans = append(spans, base.Span(snippet[at:]))
	note := h.Kind() == "note"
	card := m3.Tappable(c, design.CornerMD).FillWidth().Background(cs.SurfaceContainer)
	clicked := card.Clicked()
	card.Padding(design.SpaceLG).Children(func() {
		ui.Row(c).FillWidth().Children(func() {
			ic := icons.SubjectRounded
			if note {
				ic = icons.DescriptionOutlined
			}
			m3.Icon(c, ic, 20, cs.Primary)
			m3.HGap(c, design.SpaceMD)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				ui.RichText(c, spans...)
				m3.VGap(c, design.SpaceXS)
				var meta []string
				for _, s := range []string{h.CourseTitle(), h.Lecture()} {
					if s != "" {
						meta = append(meta, s)
					}
				}
				if !note {
					meta = append(meta, core.FmtClock(h.StartMs()))
				}
				m3.Text(c, strings.Join(meta, " · "), tt.LabelSmall.C(cs.OnSurfaceVariant))
			})
			if !note {
				m3.Icon(c, icons.PlayCircleOutlineRounded, 24, cs.OnSurfaceVariant)
			}
		})
	})
	return clicked
}
