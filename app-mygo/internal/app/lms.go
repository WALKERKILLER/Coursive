package app

import (
	"fmt"
	"net/url"
	"path"
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

func lmsTime(v any) time.Time {
	s := core.Str(v)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Local()
		}
	}
	return time.Time{}
}

func lmsDue(d time.Time) string {
	return l10n.DateMonthDayTime(int(d.Month()), d.Day(), fmt.Sprintf("%02d", d.Hour()), fmt.Sprintf("%02d", d.Minute()))
}

var (
	htmlBreak  = regexp.MustCompile(`(?i)<br\s*/?>|</p>`)
	htmlTag    = regexp.MustCompile(`<[^>]*>`)
	blankLines = regexp.MustCompile(`\n\s*\n+`)
)

func plainHTML(html string) string {
	s := htmlBreak.ReplaceAllString(html, "\n")
	s = htmlTag.ReplaceAllString(s, "")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`).Replace(s)
	return strings.TrimSpace(blankLines.ReplaceAllString(s, "\n"))
}

func lmsQuery(source, key, id string) string {
	return "source=" + url.QueryEscape(source) + "&" + key + "=" + url.QueryEscape(id)
}

// assignmentState is an assignment's state, worded for a student.
func assignmentState(a core.J) (label string, ic icons.Icon, alert bool) {
	due := lmsTime(a["due_at"])
	switch {
	case a["done"] == true:
		return l10n.CommonDone, icons.CheckCircleOutlineRounded, false
	case !due.IsZero() && due.Before(time.Now()):
		return l10n.AssignmentMissing, icons.ErrorOutlineRounded, true
	case !due.IsZero():
		return l10n.AssignmentTimeLeft(core.FmtUntil(due)), icons.RadioButtonUncheckedRounded, time.Until(due) < 24*time.Hour
	}
	return l10n.AssignmentNoDue, icons.RadioButtonUncheckedRounded, false
}

// LmsSection is LmsCourseSection: the learning-platform side of a library
// course, its open assignments and shortcuts to all assignments, files and
// announcements. It shows nothing when no platform course is linked.
type LmsSection struct {
	courseID string
	details  []core.J
	cancel   func()
}

// NewLmsSection loads the platform courses linked to a library course.
func NewLmsSection(courseID string) *LmsSection {
	s := &LmsSection{courseID: courseID}
	s.load()
	s.cancel = State.Events.Listen(func(e core.J) {
		if e.S("type") == "lms_changed" && e.O("data").S("what") != "syncing" {
			s.load()
		}
	})
	return s
}

// Dispose stops listening for platform changes.
func (s *LmsSection) Dispose() { s.cancel() }

func (s *LmsSection) load() {
	api, id := State.Api(), s.courseID
	core.Async(func() ([]core.J, error) {
		r, err := api.GetJ("/api/lms")
		if err != nil {
			return nil, err
		}
		var out []core.J
		for _, src := range r.Os("sources") {
			for _, c := range src.Os("courses") {
				if c.S("library_course") != id {
					continue
				}
				d, err := api.GetJ("/api/lms/course?" + lmsQuery(src.S("source"), "id", c.S("id")))
				if err != nil {
					return nil, err
				}
				out = append(out, d)
			}
		}
		return out, nil
	}, func(out []core.J, err error) {
		// Learning platforms are optional; the course page works without
		// them.
		if err == nil {
			s.details = out
		}
	})
}

// Show builds the section.
func (s *LmsSection) Show(c *ui.Context) {
	ui.Column(c).FillWidth().Clip().Transition(ui.ElementTransition{Duration: design.T().Motion(design.Medium2, nil).Duration, Ease: design.Emphasized, Height: true}).Children(func() {
		for _, d := range s.details {
			ui.Box(c.Key(d.S("source") + "/" + d.O("course").S("id"))).FillWidth().Children(func() { lmsCard(c, d) })
		}
	})
}

func lmsCard(c *ui.Context, d core.J) {
	cs := design.S()
	tt := design.TT()
	co := d.O("course")
	source, id, name := d.S("source"), co.S("id"), co.S("title")
	now := time.Now()
	as := d.Os("assignments")
	var open []core.J
	for _, a := range as {
		if a["done"] != true && lmsTime(a["due_at"]).After(now) {
			open = append(open, a)
		}
	}
	slices.SortFunc(open, func(a, b core.J) int { return lmsTime(a["due_at"]).Compare(lmsTime(b["due_at"])) })
	anns := d.Os("announcements")
	sheet := func(tab int) { ShowLmsCourseSheet(source, id, name, tab) }
	ui.Column(c).FillWidth().Margin(design.SpaceMD, 0, 0, 0).Radius(design.CornerXL).Clip().Background(cs.SurfaceContainerLow).
		Padding(design.SpaceMD, design.SpaceSM, design.SpaceMD, design.SpaceLG).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			m3.Icon(c, icons.CloudSyncOutlined, 20, cs.Primary)
			m3.HGap(c, design.SpaceSM)
			m3.Clamp(m3.Text(c, l10n.LmsCourseHeading(name), tt.TitleSmall), 1).Grow(1).Basis(0)
			count := l10n.LmsNoOpenAssignments
			if len(open) > 0 {
				count = l10n.AssignmentOpenCount(len(open))
			}
			m3.Text(c, count, tt.LabelMedium.C(cs.OnSurfaceVariant))
			m3.HGap(c, design.SpaceSM)
		})
		for _, a := range open[:min(len(open), 3)] {
			label, ic, alert := assignmentState(a)
			col := cs.OnSurfaceVariant
			if alert {
				col = cs.Error
			}
			sub := l10n.AssignmentDue(lmsDue(lmsTime(a["due_at"])), label)
			t := m3.Tile{Dense: true, Compact: true, Tap: true, Pad: true, PadX: design.SpaceXXS, Title: a.S("title"), TitleLines: 1, Subtitle: sub,
				Leading: func() { m3.Icon(c, ic, 20, col) }}
			if alert {
				t.SubtitleFn = func() { m3.Text(c, sub, tt.BodySmall.C(cs.Error)) }
			}
			if t.Show(c).Clicked() {
				widgets.OpenPlainLink(c, a.S("url"))
			}
		}
		m3.VGap(c, design.SpaceXS)
		ui.Row(c).Wrap().GapX(design.SpaceSM).GapY(design.SpaceXS).Children(func() {
			if m3.ActionChipOf(c, icons.AssignmentOutlined, l10n.LmsAssignmentsTab(len(as))) {
				sheet(0)
			}
			if m3.ActionChipOf(c, icons.FolderOutlined, l10n.LmsFiles) {
				sheet(1)
			}
			if m3.ActionChipOf(c, icons.CampaignOutlined, l10n.LmsAnnouncementsTab(len(anns))) {
				sheet(2)
			}
		})
	})
}

// ShowLmsCourseSheet shows all of a platform course: assignments, files
// (download into the course folder) and recent announcements. A dialog on
// wide windows, a bottom sheet on compact ones.
func ShowLmsCourseSheet(source, courseID, name string, tab int) {
	v := &lmsCourseView{source: source, courseID: courseID, name: name, tab: tab, downloading: map[string]bool{}, saved: map[string]string{}}
	v.load()
	v.loadFiles()
	if !Compact() {
		m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
			ui.Column(c).Width(640).MaxWidthPercent(100).Height(min(720, WindowHeight-80)).Radius(design.CornerXL).Clip().
				Background(design.S().SurfaceContainerHigh).Children(func() { v.view(c, r) })
		}, nil)
		return
	}
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true}, func(c *ui.Context, r *m3.Route) {
		ui.Column(c).FillWidth().Height(WindowHeight * .85).Children(func() { v.view(c, r) })
	})
}

type lmsCourseView struct {
	source, courseID, name string
	tab                    int
	detail                 core.J
	files                  []core.J
	filesLoaded            bool
	err, filesErr          error
	downloading            map[string]bool
	// saved maps a file id to its library path.
	saved   map[string]string
	scrolls [3]ui.ScrollState
}

func (v *lmsCourseView) load() {
	api := State.Api()
	core.Async(func() (core.J, error) {
		return api.GetJ("/api/lms/course?" + lmsQuery(v.source, "id", v.courseID))
	}, func(d core.J, err error) {
		if err != nil {
			v.err = err
			return
		}
		v.detail, v.err = d, nil
	})
}

func (v *lmsCourseView) loadFiles() {
	v.filesErr = nil
	api := State.Api()
	core.Async(func() (any, error) {
		return api.Get("/api/lms/files?" + lmsQuery(v.source, "course", v.courseID))
	}, func(r any, err error) {
		if err != nil {
			v.filesErr = err
			return
		}
		v.files, v.filesLoaded = core.Objs(r), true
	})
}

func (v *lmsCourseView) download(f core.J) {
	id := f.S("id")
	v.downloading[id] = true
	api := State.Api()
	core.Async(func() (any, error) {
		return api.PostWait("/api/lms/download", core.J{"source": v.source, "course": v.courseID, "file": id}, 10*time.Minute)
	}, func(r any, err error) {
		delete(v.downloading, id)
		if err != nil {
			widgets.ShowSnack(l10n.CommonDownloadFailed(err), "", nil)
			return
		}
		p := core.Obj(r).S("path")
		v.saved[id] = p
		root := State.Daemon.LibraryRoot()
		widgets.ShowSnack(l10n.LmsSavedToCourse, l10n.CommonShowInFolder, func() { core.RevealPath(path.Join(root, p)) })
	})
}

func (v *lmsCourseView) view(c *ui.Context, r *m3.Route) {
	tt := design.TT()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceLG, design.SpaceLG, 0, design.SpaceXL).Children(func() {
		m3.Clamp(m3.Text(c, v.name, tt.TitleLarge), 2).Grow(1).Basis(0)
		if !Compact() && m3.IconBtn(c, icons.CloseRounded, l10n.CommonClose).Clicked() {
			r.Close()
		}
	})
	m3.TabBar(c, []m3.Tab{{Label: l10n.CommonAssignments}, {Label: l10n.LmsFiles}, {Label: l10n.LmsAnnouncements}}, &v.tab, m3.TabOpts{})
	ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() {
		d := v.detail
		switch {
		case d == nil && v.err != nil:
			widgets.ErrorState(c, v.err, v.load)
			return
		case d == nil:
			widgets.CenteredLoader(c, "")
			return
		}
		m3.TabView(c, ui.Box(c).Fill(), v.tab, func(tab int) {
			switch tab {
			case 0:
				v.assignments(c, d)
			case 1:
				v.filesTab(c)
			default:
				v.announcements(c, d)
			}
		})
	})
}

func (v *lmsCourseView) assignments(c *ui.Context, d core.J) {
	cs := design.S()
	as := d.Os("assignments")
	if len(as) == 0 {
		widgets.EmptyState{Icon: icons.AssignmentOutlined, Title: l10n.LmsNoAssignments, Body: l10n.LmsNoAssignmentsBody}.Show(c)
		return
	}
	ui.Scroll(c).Fill().TrackScroll(&v.scrolls[0]).Children(func() {
		ui.Column(c).FillWidth().PaddingY(design.SpaceSM).Children(func() {
			for i, a := range as {
				label, ic, alert := assignmentState(a)
				due := lmsTime(a["due_at"])
				var parts []string
				if !due.IsZero() {
					parts = append(parts, l10n.AssignmentDueShort(lmsDue(due)))
				}
				if pts := a.F("points"); pts > 0 {
					parts = append(parts, l10n.AssignmentPoints(core.Str(a["points"])))
				}
				parts = append(parts, label)
				col := cs.OnSurfaceVariant
				switch {
				case alert:
					col = cs.Error
				case a["done"] == true:
					col = cs.Primary
				}
				sub := strings.Join(parts, " · ")
				t := m3.Tile{Tap: true, Title: a.S("title"), Subtitle: sub, Leading: func() { m3.Icon(c, ic, 24, col) },
					Trailing: func() { m3.Icon(c, icons.OpenInNewRounded, 18, cs.OnSurfaceVariant) }}
				if alert {
					t.SubtitleFn = func() { m3.Text(c, sub, design.TT().BodyMedium.C(cs.Error)) }
				}
				ui.Box(c.Key(i)).FillWidth().Children(func() {
					if t.Show(c).Clicked() {
						widgets.OpenPlainLink(c, a.S("url"))
					}
				})
			}
		})
	})
}

func (v *lmsCourseView) filesTab(c *ui.Context) {
	cs := design.S()
	switch {
	case !v.filesLoaded && v.filesErr != nil:
		widgets.ErrorState(c, v.filesErr, v.loadFiles)
		return
	case !v.filesLoaded:
		widgets.CenteredLoader(c, l10n.LmsLoadingFiles)
		return
	case len(v.files) == 0:
		widgets.EmptyState{Icon: icons.FolderOffOutlined, Title: l10n.LmsNoFiles, Body: l10n.LmsNoFilesBody}.Show(c)
		return
	}
	root := State.Daemon.LibraryRoot()
	ui.Scroll(c).Fill().TrackScroll(&v.scrolls[1]).Children(func() {
		ui.Column(c).FillWidth().PaddingY(design.SpaceSM).Children(func() {
			for _, f := range v.files {
				id := f.S("id")
				var parts []string
				if size := f.F("size"); size > 0 {
					parts = append(parts, core.FmtBytes(size))
				}
				if u := lmsTime(f["updated_at"]); !u.IsZero() {
					parts = append(parts, l10n.LmsFileUpdated(int(u.Month()), u.Day()))
				}
				if f.S("folder") != "" {
					parts = append(parts, f.S("folder"))
				}
				t := m3.Tile{Title: f.S("name"), TitleLines: 2, Subtitle: strings.Join(parts, " · "),
					Leading: func() { m3.Icon(c, lmsFileIcon(f.S("name")), 24, cs.OnSurfaceVariant) },
					Trailing: func() {
						saved, ok := v.saved[id]
						switch {
						case v.downloading[id]:
							ui.Box(c).Padding(design.SpaceMD).Children(func() { m3.Spinner(c, 20, 2.5, cs.Primary) })
						case ok:
							if m3.IconBtn(c, icons.FileOpenOutlined, l10n.CommonOpen).Clicked() {
								core.OpenPath(path.Join(root, saved))
							}
						default:
							if m3.IconBtn(c, icons.DownloadRounded, l10n.LmsDownloadToCourse).Clicked() {
								v.download(f)
							}
						}
					}}
				ui.Box(c.Key(id)).FillWidth().Children(func() { t.Show(c) })
			}
		})
	})
}

func (v *lmsCourseView) announcements(c *ui.Context, d core.J) {
	cs := design.S()
	tt := design.TT()
	ns := d.Os("announcements")
	if len(ns) == 0 {
		widgets.EmptyState{Icon: icons.CampaignOutlined, Title: l10n.LmsNoAnnouncements}.Show(c)
		return
	}
	ui.Scroll(c).Fill().TrackScroll(&v.scrolls[2]).Children(func() {
		ui.Column(c).FillWidth().Padding(design.SpaceLG).GapY(design.SpaceMD).Children(func() {
			for i, n := range ns {
				card := m3.Tappable(c.Key(i), design.CornerLG).FillWidth().Radius(design.CornerLG).Clip().Background(cs.SurfaceContainerLow)
				if card.Clicked() {
					widgets.OpenPlainLink(c, n.S("url"))
				}
				card.Children(func() {
					ui.Column(c).FillWidth().Padding(design.SpaceLG).Children(func() {
						m3.Text(c, n.S("title"), tt.TitleSmall)
						m3.VGap(c, design.SpaceXXS)
						var meta []string
						if a := n.S("author"); a != "" {
							meta = append(meta, a)
						}
						if at := lmsTime(n["posted_at"]); !at.IsZero() {
							meta = append(meta, core.FmtRelative(at))
						}
						m3.Text(c, strings.Join(meta, " · "), tt.LabelSmall.C(cs.OnSurfaceVariant))
						m3.VGap(c, design.SpaceSM)
						m3.Clamp(m3.Text(c, plainHTML(n.S("html")), tt.BodyMedium), 4)
					})
				})
			}
		})
	})
}

func lmsFileIcon(name string) icons.Icon {
	ext := ""
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		ext = strings.ToLower(name[i+1:])
	}
	switch ext {
	case "pdf":
		return icons.PictureAsPdfOutlined
	case "ppt", "pptx", "key":
		return icons.SlideshowOutlined
	case "doc", "docx", "txt", "md":
		return icons.DescriptionOutlined
	case "xls", "xlsx", "csv":
		return icons.TableChartOutlined
	case "zip", "rar", "7z":
		return icons.FolderZipOutlined
	case "mp4", "mov", "mkv":
		return icons.MovieOutlined
	case "png", "jpg", "jpeg", "gif":
		return icons.ImageOutlined
	}
	return icons.InsertDriveFileOutlined
}

// LmsPage is lms_page.dart's 作业与通知: each signed-in learning platform,
// its sync state, and which library course each platform course belongs
// to.
// Contract: docs/specs/0024-source-provider-plugins.md
type LmsPage struct {
	sources []core.J
	loaded  bool
	err     error
	cancel  func()
	scroll  ui.ScrollState
}

// NewLmsPage loads the platforms.
func NewLmsPage() m3.Page {
	p := &LmsPage{}
	p.load()
	p.cancel = State.Events.Listen(func(e core.J) {
		if e.S("type") == "lms_changed" {
			p.load()
		}
	})
	return p
}

// Dispose stops listening for platform changes.
func (p *LmsPage) Dispose() { p.cancel() }

// MainScroll is the page's scroll.
func (p *LmsPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *LmsPage) load() {
	api := State.Api()
	core.Async(func() (core.J, error) { return api.GetJ("/api/lms") }, func(r core.J, err error) {
		if err != nil {
			p.err = err
			return
		}
		p.sources, p.loaded, p.err = r.Os("sources"), true, nil
	})
}

func (p *LmsPage) sync() {
	api := State.Api()
	core.Async(func() (any, error) { return api.Post("/api/lms/sync", nil) }, func(_ any, err error) {
		if err != nil {
			widgets.ShowSnack(err.Error(), "", nil)
		}
	})
}

func (p *LmsPage) link(source, course, library string, auto bool) {
	api := State.Api()
	core.Async(func() (core.J, error) {
		v, err := api.Put("/api/lms/link", core.J{"source": source, "course": course, "library_course": library, "auto": auto})
		return core.Obj(v), err
	}, func(r core.J, err error) {
		if err != nil {
			widgets.ShowSnack(err.Error(), "", nil)
			return
		}
		p.sources = r.Os("sources")
	})
}

// View builds the page.
func (p *LmsPage) View(c *ui.Context) {
	pad := design.SpaceXL
	if Compact() {
		pad = design.SpaceLG
	}
	ss := p.sources
	signedIn, syncing := false, false
	for _, s := range ss {
		signedIn = signedIn || s["signed_in"] == true
		syncing = syncing || s["syncing"] == true
	}
	m3.Scaffold{
		AppBar: &m3.AppBar{Title: l10n.LmsTitle, Scrolled: p.scroll.Y > 0, Actions: func() {
			if p.loaded && signedIn && (m3.IconButton{Icon: icons.SyncRounded, Tooltip: l10n.CommonSyncNow, Disabled: syncing}).Show(c).Clicked() {
				p.sync()
			}
		}},
		Body: func() {
			switch {
			case !p.loaded && p.err != nil:
				widgets.ErrorState(c, p.err, p.load)
				return
			case !p.loaded:
				widgets.CenteredLoader(c, "")
				return
			case len(ss) == 0:
				widgets.EmptyState{Icon: icons.AssignmentOutlined, Title: l10n.LmsEmptyTitle, Body: l10n.LmsEmptyBody}.Show(c)
				return
			}
			ui.Scroll(c).Fill().TrackScroll(&p.scroll).Children(func() {
				ui.Column(c).FillWidth().Padding(design.SpaceSM, pad, design.SpaceHuge, pad).Children(func() {
					for _, s := range ss {
						widgets.ContentWidth(c, 840, func() {
							ui.Column(c.Key(s.S("source"))).FillWidth().Padding(0, 0, design.SpaceXXL, 0).Children(func() { p.sourceSection(c, s) })
						})
					}
				})
			})
		},
	}.Show(c)
}

func (p *LmsPage) sourceSection(c *ui.Context, source core.J) {
	cs := design.S()
	tt := design.TT()
	syncing := source["syncing"] == true
	synced := source.I("synced_at")
	errText := source.S("error")
	courses := source.Os("courses")
	open := 0
	for _, co := range courses {
		open += co.I("open")
	}
	var status []string
	if source["signed_in"] != true {
		status = append(status, l10n.LmsSignedOut)
	}
	switch {
	case syncing:
		status = append(status, l10n.CommonSyncing)
	case synced > 0:
		status = append(status, l10n.CommonLastSynced(core.FmtRelative(time.UnixMilli(int64(synced)))))
	default:
		status = append(status, l10n.LmsNotSynced)
	}
	status = append(status, l10n.LmsSummary(len(courses), open))
	m3.Text(c, source.S("title"), tt.TitleLarge)
	m3.VGap(c, design.SpaceXS)
	m3.Text(c, strings.Join(status, " · "), tt.BodyMedium.C(cs.OnSurfaceVariant))
	ui.Column(c).FillWidth().Clip().Transition(ui.ElementTransition{Duration: design.T().Motion(design.Medium2, nil).Duration, Ease: design.Emphasized, Height: true}).Children(func() {
		if syncing {
			ui.Box(c).FillWidth().Padding(design.SpaceSM, 0, 0, 0).Children(func() { m3.Linear(c, m3.Progress{}) })
		}
	})
	if errText != "" {
		m3.VGap(c, design.SpaceMD)
		m3.Text(c, l10n.LmsSyncFailed(errText), tt.BodyMedium.C(cs.Error))
	}
	m3.VGap(c, design.SpaceLG)
	m3.Text(c, l10n.LmsCourseLinks, tt.TitleSmall)
	m3.VGap(c, design.SpaceXS)
	m3.Text(c, l10n.LmsCourseLinksBody, tt.BodyMedium.C(cs.OnSurfaceVariant))
	m3.VGap(c, design.SpaceMD)
	if len(courses) == 0 {
		msg := l10n.LmsNoCourses
		if syncing {
			msg = l10n.LmsLoadingCourses
		}
		m3.Text(c, msg, tt.BodyMedium.C(cs.OnSurfaceVariant)).FillWidth().TextAlign(ui.Center).PaddingY(design.SpaceXL)
		return
	}
	ui.Column(c).FillWidth().Radius(design.CornerLG).Clip().Background(cs.SurfaceContainerLow).Children(func() {
		for _, co := range courses {
			ui.Box(c.Key(co.S("id"))).FillWidth().Children(func() { p.courseRow(c, source.S("source"), co) })
		}
	})
}

func (p *LmsPage) courseRow(c *ui.Context, source string, course core.J) {
	cs := design.S()
	id, name := course.S("id"), course.S("title")
	linked, manual := course.S("library_course"), course["manual"] == true
	title := ""
	for _, co := range State.Courses {
		if co.ID() == linked {
			title = co.Title()
			break
		}
	}
	var label string
	switch {
	case linked == "" && manual:
		label = l10n.LmsNoLink
	case linked == "":
		label = l10n.LmsAutoNoMatch
	case manual:
		label = title
	default:
		label = l10n.LmsAutoMatch(title)
	}
	open := course.I("open")
	var meta []string
	if t := course.S("term"); t != "" {
		meta = append(meta, t)
	}
	if open > 0 {
		meta = append(meta, l10n.AssignmentOpenCount(open))
	} else {
		meta = append(meta, l10n.LmsNoOpenAssignments)
	}
	// On a phone the link sits under the name, which keeps the row's width.
	compact := Compact()
	link := func() {
		ic := icons.LinkRounded
		if linked == "" {
			ic = icons.LinkOffRounded
		}
		maxW := float32(180)
		if compact {
			maxW = 220
		}
		m3.Menu(c, "link", m3.Below, func(t ui.Element, _ bool) {
			cs := design.S()
			t.Cursor(ui.CursorPointer).Radius(design.CornerSM).Border(1, cs.OutlineVariant).Height(32).PaddingX(8)
			m3.Tip(c, t, l10n.LmsLinkCourse)
			m3.Ink(t, cs.OnSurface, design.CornerSM)
			t.Children(func() {
				ui.Row(c).AlignItems(ui.Center).FillHeight().Children(func() {
					m3.Icon(c, ic, 18, cs.Primary)
					m3.HGap(c, 8)
					m3.Clamp(m3.Text(c, label, design.TT().LabelLarge.C(cs.OnSurface)), 1).MaxWidth(maxW)
				})
			})
		}, func() []m3.MenuItem {
			items := []m3.MenuItem{
				{Icon: icons.AutoAwesomeOutlined, Label: l10n.LmsAutoLink, OnTap: func() { p.link(source, id, "", true) }},
				{Icon: icons.LinkOffRounded, Label: l10n.LmsNoLink, OnTap: func() { p.link(source, id, "", false) }},
			}
			if len(State.Courses) > 0 {
				items = append(items, m3.MenuItem{Divider: true})
			}
			for _, co := range State.Courses {
				ic := icons.ClassOutlined
				if co.ID() == linked && manual {
					ic = icons.CheckRounded
				}
				lib := co.ID()
				items = append(items, m3.MenuItem{Icon: ic, Label: co.Title(), OnTap: func() { p.link(source, id, lib, false) }})
			}
			return items
		})
	}
	right := design.SpaceSM
	if compact {
		right = design.SpaceLG
	}
	initial := "?"
	if r := []rune(name); len(r) > 0 {
		initial = string(r[0])
	}
	t := m3.Tile{Tap: true, Pad: true, PadX: design.SpaceLG, PadY: design.SpaceXS, Title: name, TitleLines: 1,
		Leading: func() {
			widgets.CircleAvatar(c, 40, cs.TertiaryContainer, func() {
				m3.Text(c, initial, design.TT().TitleMedium.C(cs.OnTertiaryContainer))
			})
		},
		SubtitleFn: func() {
			ui.Column(c).Children(func() {
				m3.Clamp(m3.Text(c, strings.Join(meta, " · "), design.TT().BodyMedium.C(cs.OnSurfaceVariant)), 1)
				if compact {
					m3.VGap(c, design.SpaceXS)
					link()
				}
			})
		}}
	if compact {
		t.TitleLines = 2
	} else {
		t.Trailing = link
	}
	ui.Box(c).FillWidth().Padding(0, right-design.SpaceLG, 0, 0).Children(func() {
		if t.Show(c).Clicked() {
			ShowLmsCourseSheet(source, id, name, 0)
		}
	})
}
