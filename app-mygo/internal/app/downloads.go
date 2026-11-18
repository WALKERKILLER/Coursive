package app

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// DownloadsPage is downloads_page.dart's: the jobs, with a selection mode
// acting on picked jobs, and the files on disk.
type DownloadsPage struct {
	view int // 0 tasks, 1 local files
	// Selection mode: long-press a job or use 选择任务; the toolbar then
	// acts on the picked jobs only.
	selecting bool
	picked    map[string]bool
	scroll    ui.ScrollState
	local     localFiles
}

// NewDownloadsPage shows the jobs.
func NewDownloadsPage() m3.Page { return &DownloadsPage{picked: map[string]bool{}} }

// MainScroll is the page's scroll.
func (p *DownloadsPage) MainScroll() *ui.ScrollState { return &p.scroll }

// Dispose stops the local view's listener.
func (p *DownloadsPage) Dispose() { p.local.stop() }

func pickable(j core.Job) bool { return j.Active() || j.State() == "paused" || j.State() == "failed" }

func kindLabel(kind string) string {
	if l := core.KindLabel(kind); l != "" {
		return l
	}
	return kind
}

func (p *DownloadsPage) toggle(j core.Job) {
	p.selecting = true
	if p.picked[j.ID()] {
		delete(p.picked, j.ID())
	} else {
		p.picked[j.ID()] = true
	}
}

func (p *DownloadsPage) endSelection() {
	p.selecting = false
	clear(p.picked)
}

func (p *DownloadsPage) clearFinished() {
	files := false
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		cs := design.S()
		m3.AlertDialog(c, m3.Alert{Icon: icons.CleaningServicesOutlined, Title: l10n.DownloadsClearTitle, Content: func() {
			ui.Column(c).FillWidth().Children(func() {
				m3.Body(c, l10n.DownloadsClearBody)
				m3.VGap(c, design.SpaceMD)
				m3.CheckTile(c, m3.Tile{Title: l10n.DownloadsClearAlsoFiles, Subtitle: l10n.DownloadsClearAlsoFilesHint, Pad: true}, &files)
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Pop(false)
			}
			b := m3.Button{Kind: m3.Filled, Label: l10n.DownloadsClearRecords}
			if files {
				b.Label, b.BG, b.FG = l10n.DownloadsClearAndDelete, cs.Error, cs.OnError
			}
			if b.Show(c).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v != true {
			return
		}
		State.ClearFinished(files, func(freed int, err error) {
			switch {
			case err != nil:
				widgets.ShowSnack(l10n.DownloadsClearFailed(err), "", nil)
			case files:
				widgets.ShowSnack(l10n.DownloadsClearedFreed(core.FmtBytes(float64(freed))), "", nil)
			default:
				widgets.ShowSnack(l10n.DownloadsCleared, "", nil)
			}
		})
	})
}

func (p *DownloadsPage) bulk(action string, ids []string, then func()) {
	State.BulkJobs(action, ids, func(n int, err error) {
		if err != nil {
			widgets.ShowSnack(l10n.CommonActionFailed(err), "", nil)
		} else if n > 0 {
			verb := map[string]string{"pause": l10n.CommonPaused, "resume": l10n.DownloadsResumed, "retry": l10n.DownloadsRetried, "cancel": l10n.CommonCancelled}[action]
			widgets.ShowSnack(l10n.DownloadsBulkDone(verb, n), "", nil)
		}
		if then != nil {
			then()
		}
	})
}

// bulkPicked runs a toolbar action on the picked jobs it applies to.
func (p *DownloadsPage) bulkPicked(action string, picked []core.Job) {
	var ids []string
	for _, j := range picked {
		var ok bool
		switch action {
		case "pause":
			ok = j.Active()
		case "resume":
			ok = j.State() == "paused"
		case "retry":
			ok = j.State() == "failed"
		default:
			ok = j.Active() || j.State() == "paused"
		}
		if ok {
			ids = append(ids, j.ID())
		}
	}
	if len(ids) == 0 {
		return
	}
	run := func() { p.bulk(action, ids, p.endSelection) }
	if action != "cancel" {
		run()
		return
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Icon: icons.StopCircleOutlined, Title: l10n.DownloadsCancelTitle(len(ids)), Message: l10n.DownloadsCancelBody, Actions: func() {
			if m3.TextButton(c, l10n.DownloadsKeep).Clicked() {
				r.Pop(false)
			}
			if m3.FilledButton(c, l10n.DownloadsCancelConfirm).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v == true {
			run()
		}
	})
}

// View builds the page.
func (p *DownloadsPage) View(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	compact := Compact()
	pad := design.SpaceXL
	if compact {
		pad = design.SpaceLG
	}
	jobs := State.SortedJobs()
	var active, failed, done, pickableJobs []core.Job
	speed, running, pausable, paused := 0, 0, 0, 0
	for _, j := range jobs {
		switch j.State() {
		case "failed":
			failed = append(failed, j)
		case "done", "canceled":
			done = append(done, j)
		case "paused":
			paused++
		}
		if j.Active() || j.State() == "paused" {
			active = append(active, j)
		}
		if j.State() == "running" {
			running++
			speed += j.Speed()
		}
		if j.Active() {
			pausable++
		}
		if pickable(j) {
			pickableJobs = append(pickableJobs, j)
		}
	}
	for id := range p.picked {
		if !slices.ContainsFunc(pickableJobs, func(j core.Job) bool { return j.ID() == id }) {
			delete(p.picked, id)
		}
	}
	selecting := p.selecting && p.view == 0 && len(pickableJobs) > 0
	var picked []core.Job
	for _, j := range pickableJobs {
		if p.picked[j.ID()] {
			picked = append(picked, j)
		}
	}
	view := l10n.CommonTasks
	if p.view == 1 {
		view = l10n.CommonDownloaded
	}
	ReportScreen(map[string]string{"screen": l10n.ScreenDownloads, "view": view})

	header := func() {
		widgets.ContentWidth(c, 0, func() {
			top := design.SpaceXXL
			if compact {
				top = design.SpaceXL
			}
			ui.Column(c).FillWidth().Padding(top, pad, design.SpaceMD, pad).Children(func() {
				st := tt.DisplaySmall
				if compact {
					st = tt.HeadlineMedium
				}
				m3.Text(c, l10n.CommonDownload, st)
				m3.VGap(c, design.SpaceXS)
				summary, tone := l10n.DownloadsNoneRunning, cs.OnSurfaceVariant
				switch {
				case running > 0:
					summary, tone = l10n.DownloadsRunningSummary(running, core.FmtSpeed(float64(speed))), cs.Primary
				case pausable > 0:
					summary = l10n.DownloadsWaiting(pausable)
				case paused > 0:
					summary = l10n.DownloadsPausedSummary(paused)
				}
				key := "idle"
				if running > 0 {
					key = "running"
				}
				ui.Box(c.Key(key)).Transition(ui.ElementTransition{Duration: design.Short4, Enter: &ui.Motion{Opacity: 0}}).Children(func() {
					m3.Text(c, summary, tt.BodyLarge.C(tone))
				})
				m3.VGap(c, design.SpaceLG)
				ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
					tasks := l10n.CommonTasks
					if len(jobs) > 0 {
						tasks = l10n.CommonTasksCount(len(jobs))
					}
					m3.Segmented(c, []m3.Segment[int]{
						{Value: 0, Icon: icons.DownloadingRounded, Label: tasks},
						{Value: 1, Icon: icons.Inventory2Outlined, Label: l10n.CommonDownloaded},
					}, &p.view, m3.SegOpts{NoCheck: true})
					if p.view == 0 && len(pickableJobs) > 0 && !selecting &&
						(m3.Button{Kind: m3.TextKind, Icon: icons.ChecklistRounded, Label: l10n.DownloadsSelectJobs}).Show(c).Clicked() {
						p.selecting = true
					}
				})
			})
		})
	}
	if p.view == 1 {
		p.local.view(c, header, pad, &p.scroll)
		return
	}
	p.local.stop()

	sc := ui.Scroll(c).Fill().TrackScroll(&p.scroll)
	sc.Children(func() {
		ui.Column(c).FillWidth().MinHeight(WindowHeight - 64).Children(func() {
			header()
			if selecting {
				allPicked := len(picked) == len(pickableJobs)
				can := map[string]bool{}
				for _, j := range picked {
					can["pause"] = can["pause"] || j.Active()
					can["resume"] = can["resume"] || j.State() == "paused"
					can["retry"] = can["retry"] || j.State() == "failed"
					can["cancel"] = can["cancel"] || j.Active() || j.State() == "paused"
				}
				widgets.ContentWidth(c, 0, func() {
					ui.Box(c).FillWidth().Padding(design.SpaceXS, pad-design.SpaceXS, design.SpaceSM, pad-design.SpaceXS).Children(func() {
						selectionBar(c, len(picked), allPicked, len(failed) > 0, can, p.endSelection, func() {
							if allPicked {
								clear(p.picked)
							} else {
								for _, j := range pickableJobs {
									p.picked[j.ID()] = true
								}
							}
						}, func(a string) { p.bulkPicked(a, picked) })
					})
				}).Sticky().Top(0).ZIndex(1)
			}
			if len(jobs) == 0 {
				ui.Column(c).FillWidth().Grow(1).MinHeight(320).Children(func() {
					widgets.EmptyState{Icon: icons.DownloadRounded, Title: l10n.DownloadsEmpty, Body: l10n.DownloadsEmptyBody}.Show(c)
				})
			}
			p.section(c, l10n.DownloadsSectionActive, active, pad, selecting, func() {
				if (m3.Button{Kind: m3.TextKind, Icon: icons.PauseRounded, Label: l10n.DownloadsPauseAll, Disabled: pausable == 0}).Show(c).Clicked() {
					p.bulk("pause", nil, nil)
				}
				if (m3.Button{Kind: m3.TextKind, Icon: icons.PlayArrowRounded, Label: l10n.DownloadsResumeAll, Disabled: paused == 0}).Show(c).Clicked() {
					p.bulk("resume", nil, nil)
				}
			})
			p.section(c, l10n.CommonFailed, failed, pad, selecting, func() {
				if (m3.Button{Kind: m3.TextKind, Icon: icons.RefreshRounded, Label: l10n.DownloadsRetryAll}).Show(c).Clicked() {
					p.bulk("retry", nil, nil)
				}
			})
			p.section(c, l10n.CommonDone, done, pad, selecting, func() {
				if (m3.Button{Kind: m3.TextKind, Icon: icons.CleaningServicesOutlined, Label: l10n.DownloadsClearDone}).Show(c).Clicked() {
					p.clearFinished()
				}
			})
			m3.VGap(c, design.SpaceHuge+BottomInset)
		})
	})
}

func (p *DownloadsPage) section(c *ui.Context, title string, list []core.Job, pad float32, selecting bool, actions func()) {
	if len(list) == 0 {
		return
	}
	cs := design.S()
	widgets.ContentWidth(c, 0, func() {
		ui.Row(c).FillWidth().Wrap().Justify(ui.SpaceBetween).AlignItems(ui.Center).Padding(design.SpaceLG, pad-design.SpaceSM, design.SpaceXS, pad).Children(func() {
			ui.Box(c).PaddingY(design.SpaceSM).Children(func() {
				m3.Text(c, title+" · "+itoa(len(list)), design.TT().TitleSmall.C(cs.OnSurfaceVariant))
			})
			// The toolbar takes over while picking jobs.
			if !selecting {
				ui.Row(c).Wrap().Children(actions)
			}
		})
	})
	for _, j := range list {
		widgets.ContentWidth(c, 0, func() {
			ui.Box(c.Key(j.ID())).FillWidth().Padding(2, pad).Children(func() {
				var pick func()
				if pickable(j) {
					pick = func() { p.toggle(j) }
				}
				jobTile(c, j, selecting, p.picked[j.ID()], pick)
			})
		})
	}
}

// ConfirmDeleteFiles is confirmDeleteFiles: confirms, deletes local files
// and shows the freed space.
func ConfirmDeleteFiles(what string, run func(done func(int, error))) {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		cs := design.S()
		m3.AlertDialog(c, m3.Alert{Icon: icons.DeleteForeverOutlined, Title: l10n.DownloadsDeleteFilesTitle, Message: l10n.DownloadsDeleteFilesBody(what), Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Pop(false)
			}
			if (m3.Button{Kind: m3.Filled, Label: l10n.CommonDelete, BG: cs.Error, FG: cs.OnError}).Show(c).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v != true {
			return
		}
		run(func(freed int, err error) {
			if err != nil {
				widgets.ShowSnack(l10n.CommonDeleteFailed(err), "", nil)
				return
			}
			widgets.ShowSnack(l10n.DownloadsDeletedFreed(core.FmtBytes(float64(freed))), "", nil)
		})
	})
}

// RevealLecture opens a lecture's folder in the system file manager.
func RevealLecture(lectureID string) {
	api := State.Api()
	core.Async(func() (string, error) {
		v, err := api.Get("/api/lectures/" + core.Esc(lectureID))
		if err != nil {
			return "", err
		}
		r := core.Obj(v)
		dir := core.Obj(r["lecture"]).S("dir")
		if dir == "" {
			return "", nil
		}
		return filepath.Join(r.S("library_root"), dir), nil
	}, func(path string, err error) {
		switch {
		case err != nil:
			widgets.ShowSnack(err.Error(), "", nil)
		case path == "" || !core.OpenPath(path):
			widgets.ShowSnack(l10n.CommonCannotOpenFolder, "", nil)
		}
	})
}

func jobIcon(kind string) icons.Icon {
	switch kind {
	case "video":
		return icons.MovieOutlined
	case "slides":
		return icons.SlideshowOutlined
	case "courseware":
		return icons.AttachFileRounded
	case "parse":
		return icons.DocumentScannerOutlined
	case "transcript":
		return icons.SubjectRounded
	}
	return icons.DownloadOutlined
}

// jobTile is _JobTile; pick toggles the job in the selection, nil for jobs
// that can't be picked.
func jobTile(c *ui.Context, job core.Job, selecting, picked bool, pick func()) {
	cs := design.S()
	tt := design.TT()
	failed := job.State() == "failed"
	done := job.State() == "done"
	pickMode := selecting && pick != nil
	bg := cs.SurfaceContainer
	switch {
	case picked:
		bg = cs.SecondaryContainer
	case failed:
		bg = design.WithOpacity(cs.ErrorContainer, .4)
	}
	card := m3.Tappable(c, design.CornerMD).FillWidth().Background(bg)
	if pickMode {
		card = card.Selected(picked)
	}
	// While picking, a tap only picks; finished jobs ignore it.
	if card.Clicked() {
		switch {
		case selecting && pick != nil:
			pick()
		case !selecting && done && job.Kind() == "video":
			m3.RootNav.Push(NewPlayerPage(job.LectureID(), -1))
		}
	}
	if !selecting && pick != nil && card.LongPressed() {
		pick()
	}
	card.Children(func() {
		right := design.SpaceSM
		if pickMode {
			right = design.SpaceLG
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD, right, design.SpaceMD, design.SpaceLG).Children(func() {
			key := "kind"
			if pickMode {
				key = "pick"
			}
			ui.Box(c.Key(key)).Transition(ui.ElementTransition{Duration: design.Short4, Enter: &ui.Motion{Opacity: 0, Scale: .8}}).Children(func() {
				if pickMode {
					b := ui.Box(c).Size(44, 44).Center()
					anim := design.T().Motion(design.Short4, nil)
					rad := design.CornerFull
					if picked {
						rad = design.CornerMD
						b = b.Background(cs.Primary)
					} else {
						b = b.Border(2, cs.Outline)
					}
					b.Radius(min(b.AnimateTo("r", rad, anim), 22)).Transition(ui.ElementTransition{Duration: anim.Duration, Colors: true}).Children(func() {
						if picked {
							m3.Icon(c, icons.CheckRounded, 24, cs.OnPrimary)
						}
					})
					return
				}
				bg, fg, ic := cs.SecondaryContainer, cs.OnSecondaryContainer, jobIcon(job.Kind())
				switch {
				case done:
					bg, fg, ic = cs.TertiaryContainer, cs.OnTertiaryContainer, icons.DownloadDoneRounded
				case failed:
					bg, fg = cs.ErrorContainer, cs.OnErrorContainer
				}
				ui.Box(c).Size(44, 44).Center().Radius(design.CornerMD).Background(bg).Children(func() { m3.Icon(c, ic, 24, fg) })
			})
			m3.HGap(c, design.SpaceLG)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				m3.Clamp(m3.Text(c, job.Title(), tt.TitleSmall), 1)
				line := kindLabel(job.Kind())
				if co, ok := State.Course(job.CourseID()); ok {
					line += " · " + co.Title()
				}
				m3.Clamp(m3.Text(c, line, tt.BodySmall.C(cs.OnSurfaceVariant)), 1)
				switch {
				case job.Active() || job.State() == "paused":
					m3.VGap(c, design.SpaceSM)
					jobProgress(c, job)
				case failed:
					m3.VGap(c, design.SpaceXS)
					m3.Clamp(m3.Text(c, job.Error(), tt.BodySmall.C(cs.Error)), 2)
				case done && job.TotalBytes() > 0:
					m3.Text(c, core.FmtBytes(float64(job.TotalBytes())), tt.LabelSmall.C(cs.OnSurfaceVariant))
				}
			})
			if pickMode {
				return
			}
			m3.HGap(c, design.SpaceSM)
			act := func(ic icons.Icon, tip, action string) {
				if m3.IconBtn(c, ic, tip).Clicked() {
					State.JobAction(job.ID(), action, nil)
				}
			}
			switch job.State() {
			case "running", "queued":
				act(icons.PauseRounded, l10n.CommonPause, "pause")
			case "paused":
				act(icons.PlayArrowRounded, l10n.CommonResume, "resume")
			}
			if failed {
				act(icons.RefreshRounded, l10n.CommonRetry, "retry")
			}
			if done && core.CanOpenFolders() && m3.IconBtn(c, icons.FolderOpenOutlined, l10n.CommonShowInFolder).Clicked() {
				RevealLecture(job.LectureID())
			}
			if job.Active() || job.State() == "paused" {
				act(icons.CloseRounded, l10n.CommonCancel, "cancel")
				return
			}
			m3.MenuButton(c, "delete", icons.DeleteOutlineRounded, l10n.CommonDelete, m3.Below, func() []m3.MenuItem {
				items := []m3.MenuItem{{Icon: icons.PlaylistRemoveRounded, Label: l10n.DownloadsRemoveRecordOnly, OnTap: func() { State.JobAction(job.ID(), "remove", nil) }}}
				if done {
					size := ""
					if job.TotalBytes() > 0 {
						size = l10n.DownloadsSizeSuffix(core.FmtBytes(float64(job.TotalBytes())))
					}
					items = append(items, m3.MenuItem{Icon: icons.DeleteForeverOutlined, Label: l10n.DownloadsDeleteRecordAndFiles, Danger: true, OnTap: func() {
						ConfirmDeleteFiles(l10n.DownloadsJobFiles(job.Title(), kindLabel(job.Kind()), size), func(done func(int, error)) {
							State.RemoveJobFiles(job.ID(), done)
						})
					}})
				}
				return items
			})
		})
	})
}

// jobProgress is _JobProgress: only a running job is drawn in the accent
// colour; waiting and paused jobs keep their progress on a muted bar, and
// the bar only sweeps while the daemon works without knowing the size yet.
func jobProgress(c *ui.Context, job core.Job) {
	cs := design.S()
	tt := design.TT()
	running := job.State() == "running"
	retrying := job.State() == "queued" && !job.RetryAt().IsZero()
	known := job.TotalBytes() > 0 || job.Progress() > 0
	percent := int(job.Progress() * 100)
	var ic icons.Icon
	var label string
	switch {
	case running:
		ic, label = icons.ArrowDownwardRounded, job.Stage()
		if label == "" {
			label = l10n.DownloadsStateRunning
		}
	case retrying:
		ic, label = icons.WifiOffRounded, l10n.DownloadsStateRetrying(core.FmtClockTime(job.RetryAt()))
	case job.State() == "paused":
		ic, label = icons.PauseRounded, l10n.CommonPaused
	default:
		ic, label = icons.ScheduleRounded, l10n.CommonQueued
	}
	tone := cs.OnSurfaceVariant
	switch {
	case running:
		tone = cs.Primary
	case retrying:
		tone = cs.Error
	}
	details := []string{label}
	if running && job.Speed() > 0 {
		details = append(details, core.FmtSpeed(float64(job.Speed())))
	}
	if job.TotalBytes() > 0 {
		details = append(details, l10n.DownloadsSizeProgress(core.FmtBytes(float64(job.DoneBytes())), core.FmtBytes(float64(job.TotalBytes()))))
	}
	if running && job.Conns() > 1 {
		details = append(details, l10n.DownloadsConnections(job.Conns()))
	}
	ui.Column(c).FillWidth().Children(func() {
		bar := ui.Box(c).FillWidth()
		v := float64(bar.AnimateTo("v", float32(job.Progress()), design.T().Motion(design.Medium2, nil)))
		bar.Children(func() {
			pr := m3.Progress{Value: m3.Of(v), Radius: design.CornerFull}
			switch {
			case running && !known:
				pr.Value = nil
			case running:
			case retrying:
				pr.Color, pr.Track = cs.Error, cs.SurfaceContainerHighest
			default:
				pr.Color, pr.Track = cs.Outline, cs.SurfaceContainerHighest
			}
			m3.Linear(c, pr).Label(label)
		})
		m3.VGap(c, design.SpaceXS)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			m3.Icon(c, ic, 14, tone)
			m3.HGap(c, design.SpaceXS)
			dc := cs.OnSurfaceVariant
			if retrying {
				dc = cs.Error
			}
			m3.Clamp(m3.Text(c, strings.Join(details, " · "), tt.LabelSmall.C(dc)), 1).FontFeatures("tnum").Grow(1).Basis(0)
			if known {
				m3.HGap(c, design.SpaceSM)
				m3.Text(c, l10n.DownloadsPercent(percent), tt.LabelMedium.C(tone)).FontFeatures("tnum")
			}
		})
	})
}

// selectionBar is _SelectionBar, the floating toolbar for the picked jobs
// (M3 Expressive): actions that don't apply to any picked job stay visible
// but disabled.
func selectionBar(c *ui.Context, count int, allPicked, showRetry bool, can map[string]bool, onClose, onAll func(), onAction func(string)) {
	cs := design.S()
	tt := design.TT()
	bar := ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceXS).Radius(design.CornerFull).Background(cs.SecondaryContainer)
	bar = m3.Elevation(bar, 3)
	labels := widgets.Width(bar, 600) >= 640
	fg := cs.OnSecondaryContainer
	bar.Children(func() {
		if (m3.IconButton{Icon: icons.CloseRounded, Tooltip: l10n.DownloadsExitSelection, Color: fg}).Show(c).Clicked() {
			onClose()
		}
		m3.HGap(c, design.SpaceXS)
		m3.Clamp(m3.Text(c, l10n.DownloadsSelectedCount(count), tt.TitleSmall.C(fg)), 1).Grow(1).Basis(0)
		tip, ic := l10n.CommonSelectAll, icons.SelectAllRounded
		if allPicked {
			tip, ic = l10n.DownloadsSelectNone, icons.DeselectRounded
		}
		if (m3.IconButton{Icon: ic, Tooltip: tip, Color: fg}).Show(c).Clicked() {
			onAll()
		}
		act := func(a string, ic icons.Icon, label string) {
			off := !can[a]
			var e ui.Element
			if labels {
				e = (m3.Button{Kind: m3.TextKind, Icon: ic, Label: label, Disabled: off, FG: fg}).Show(c)
			} else {
				e = (m3.IconButton{Icon: ic, Tooltip: label, Disabled: off, Color: fg}).Show(c)
			}
			if e.Clicked() && !off {
				onAction(a)
			}
		}
		act("pause", icons.PauseRounded, l10n.CommonPause)
		act("resume", icons.PlayArrowRounded, l10n.CommonResume)
		if showRetry {
			act("retry", icons.RefreshRounded, l10n.CommonRetry)
		}
		act("cancel", icons.StopCircleOutlined, l10n.DownloadsCancelConfirm)
	})
}

type localItem struct {
	lectureID, courseID, courseTitle, lecture, date, kind string
	bytes, files                                          int
}

var kindOrder = []string{"video", "slides", "courseware", "transcript"}

// localFiles is _LocalView: everything on disk, grouped by course then
// lecture, with per-kind delete.
type localFiles struct {
	items   []localItem
	loaded  bool
	err     error
	filter  string
	cancel  func()
	refresh ui.RefreshState
}

func (v *localFiles) stop() {
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
}

func (v *localFiles) start() {
	if v.cancel != nil {
		return
	}
	v.load(nil)
	v.cancel = State.Events.Listen(func(e core.J) {
		if e.S("type") == "lecture_changed" {
			v.load(nil)
		}
	})
}

func (v *localFiles) load(then func()) {
	api := State.Api()
	core.Async(func() (any, error) { return api.Get("/api/local") }, func(r any, err error) {
		if err != nil {
			v.err = err
		} else {
			v.items, v.loaded, v.err = nil, true, nil
			for _, e := range core.List(r) {
				m := core.Obj(e)
				v.items = append(v.items, localItem{m.S("lecture_id"), m.S("course_id"), m.S("course_title"), m.S("lecture"), m.S("date"), m.S("kind"), m.I("bytes"), m.I("files")})
			}
		}
		if then != nil {
			then()
		}
	})
}

func (v *localFiles) delete(lectureID string, kinds []string, what string) {
	ConfirmDeleteFiles(what, func(done func(int, error)) {
		freed := 0
		var next func(i int)
		next = func(i int) {
			if i == len(kinds) {
				v.load(func() { done(freed, nil) })
				return
			}
			State.RemoveAsset(lectureID, kinds[i], func(n int, err error) {
				if err != nil {
					done(0, err)
					return
				}
				freed += n
				next(i + 1)
			})
		}
		next(0)
	})
}

func (v *localFiles) view(c *ui.Context, header func(), pad float32, scroll *ui.ScrollState) {
	v.start()
	cs := design.S()
	tt := design.TT()
	kindColor := map[string]ui.Color{"video": cs.Primary, "slides": cs.Tertiary, "courseware": cs.Secondary, "transcript": cs.Outline}
	sc := ui.Scroll(c).Fill().TrackScroll(scroll)
	if v.loaded && sc.PullToRefresh(&v.refresh) {
		v.load(func() { v.refresh.Refreshing = false })
	}
	sc.Children(func() {
		ui.Column(c).FillWidth().MinHeight(WindowHeight - 64).Children(func() {
			header()
			fill := func(child func()) { ui.Column(c).FillWidth().Grow(1).MinHeight(320).Children(child) }
			switch {
			case !v.loaded && v.err != nil:
				fill(func() { widgets.ErrorState(c, v.err, func() { v.load(nil) }) })
				return
			case !v.loaded:
				fill(func() { widgets.CenteredLoader(c, "") })
				return
			case len(v.items) == 0:
				fill(func() {
					widgets.EmptyState{Icon: icons.Inventory2Outlined, Title: l10n.DownloadsLocalEmpty, Body: l10n.DownloadsLocalEmptyBody}.Show(c)
				})
				m3.VGap(c, design.SpaceHuge+BottomInset)
				return
			}
			total := 0
			byKind := map[string]int{}
			for _, it := range v.items {
				total += it.bytes
				byKind[it.kind] += it.bytes
			}
			widgets.ContentWidth(c, 0, func() {
				ui.Box(c).FillWidth().Padding(design.SpaceSM, pad, design.SpaceMD, pad).Children(func() {
					m3.CardColor(c, cs.SurfaceContainer).FillWidth().Padding(design.SpaceXL).Children(func() {
						ui.Row(c).AlignItems(ui.End).Children(func() {
							m3.Text(c, core.FmtBytes(float64(total)), tt.HeadlineMedium.C(cs.OnSurface))
							m3.HGap(c, design.SpaceSM)
							m3.Text(c, l10n.HomeStatStorage, tt.BodyMedium.C(cs.OnSurfaceVariant)).Padding(0, 0, 4, 0)
						})
						m3.VGap(c, design.SpaceMD)
						// Stacked usage bar by kind.
						ui.Row(c).FillWidth().Height(10).Radius(design.CornerFull).Clip().Children(func() {
							for _, k := range kindOrder {
								if byKind[k] > 0 {
									flex := max(1, int(float64(byKind[k])*1000/float64(max(total, 1))+.5))
									ui.Box(c).Grow(float32(flex)).Basis(0).FillHeight().Padding(0, 2, 0, 0).Children(func() {
										ui.Box(c).Fill().Background(kindColor[k])
									})
								}
							}
						})
						m3.VGap(c, design.SpaceMD)
						ui.Row(c).Wrap().Gap(design.SpaceSM).Children(func() {
							for _, k := range kindOrder {
								if byKind[k] == 0 {
									continue
								}
								col := kindColor[k]
								ch := m3.Chip{Kind: m3.FilterChip, Label: kindLabel(k) + " " + core.FmtBytes(float64(byKind[k])), Selected: v.filter == k, NoCheck: true,
									AvatarFn: func(ui.Color) { ui.Box(c).Size(10, 10).Radius(5).Background(col) }}
								if ch.Show(c).Clicked() {
									if v.filter == k {
										v.filter = ""
									} else {
										v.filter = k
									}
								}
							}
						})
					})
				})
			})
			// course -> lecture -> items, in first-seen order.
			type lectureGroup struct {
				id    string
				items []localItem
			}
			type courseGroup struct {
				id       string
				lectures []*lectureGroup
			}
			var courses []*courseGroup
			for _, it := range v.items {
				if v.filter != "" && it.kind != v.filter {
					continue
				}
				i := slices.IndexFunc(courses, func(g *courseGroup) bool { return g.id == it.courseID })
				if i < 0 {
					courses = append(courses, &courseGroup{id: it.courseID})
					i = len(courses) - 1
				}
				cg := courses[i]
				j := slices.IndexFunc(cg.lectures, func(g *lectureGroup) bool { return g.id == it.lectureID })
				if j < 0 {
					cg.lectures = append(cg.lectures, &lectureGroup{id: it.lectureID})
					j = len(cg.lectures) - 1
				}
				cg.lectures[j].items = append(cg.lectures[j].items, it)
			}
			for _, cg := range courses {
				bytes := 0
				for _, lg := range cg.lectures {
					for _, it := range lg.items {
						bytes += it.bytes
					}
				}
				widgets.ContentWidth(c, 0, func() {
					ui.Row(c.Key("c-"+cg.id)).FillWidth().AlignItems(ui.Center).Padding(design.SpaceLG, pad, design.SpaceSM, pad).Children(func() {
						title := cg.lectures[0].items[0].courseTitle
						if co, ok := State.Course(cg.id); ok {
							widgets.CourseGlyph(c, co, 28, false)
							m3.HGap(c, design.SpaceMD)
							title = co.Title()
						}
						m3.Clamp(m3.Text(c, title, tt.TitleMedium), 1).Grow(1).Basis(0)
						m3.Text(c, core.FmtBytes(float64(bytes)), tt.LabelMedium.C(cs.OnSurfaceVariant))
					})
				})
				for _, lg := range cg.lectures {
					list := lg.items
					slices.SortStableFunc(list, func(a, b localItem) int { return slices.Index(kindOrder, a.kind) - slices.Index(kindOrder, b.kind) })
					widgets.ContentWidth(c, 0, func() {
						ui.Box(c.Key("l-"+lg.id)).FillWidth().Padding(2, pad).Children(func() {
							v.lectureCard(c, lg.id, list, kindColor)
						})
					})
				}
			}
			m3.VGap(c, design.SpaceHuge+BottomInset)
		})
	})
}

func (v *localFiles) lectureCard(c *ui.Context, lectureID string, list []localItem, kindColor map[string]ui.Color) {
	cs := design.S()
	tt := design.TT()
	hasVideo := slices.ContainsFunc(list, func(x localItem) bool { return x.kind == "video" })
	// Without a video the card takes no taps, but its chips still do.
	card := ui.Column(c).FillWidth().Radius(design.CornerMD)
	if hasVideo {
		card = m3.Tappable(c, design.CornerMD).FillWidth()
	}
	card = card.Background(cs.SurfaceContainer)
	if hasVideo && card.Clicked() {
		m3.RootNav.Push(NewPlayerPage(lectureID, -1))
	}
	first := list[0]
	card.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceMD, design.SpaceSM, design.SpaceMD, design.SpaceLG).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				var head []string
				for _, s := range []string{first.date, first.lecture} {
					if s != "" {
						head = append(head, s)
					}
				}
				m3.Clamp(m3.Text(c, strings.Join(head, " · "), tt.TitleSmall), 1)
				m3.VGap(c, design.SpaceSM)
				ui.Row(c).Wrap().Gap(design.SpaceXS).Children(func() {
					for _, it := range list {
						col := kindColor[it.kind]
						ch := m3.Chip{Kind: m3.InputChip, Compact: true, Deletable: true, Label: kindLabel(it.kind) + " · " + core.FmtBytes(float64(it.bytes)),
							DeleteIcon: icons.CloseRounded, DeleteSize: 16, DeleteTooltip: l10n.DownloadsDeleteKindFiles(core.KindLabel(it.kind)),
							AvatarFn: func(ui.Color) { ui.Box(c).Size(8, 8).Radius(4).Background(col) }}
						ui.Box(c.Key(it.kind)).Children(func() {
							if ch.Show(c).Deleted {
								v.delete(it.lectureID, []string{it.kind}, l10n.DownloadsLectureKind(kindLabel(it.kind), core.FmtBytes(float64(it.bytes))))
							}
						})
					}
				})
			})
			m3.MenuButton(c, "more", icons.MoreVertRounded, l10n.CommonMore, m3.Below, func() []m3.MenuItem {
				var items []m3.MenuItem
				if core.CanOpenFolders() {
					items = append(items, m3.MenuItem{Icon: icons.FolderOpenOutlined, Label: l10n.CommonShowInFolder, OnTap: func() { RevealLecture(lectureID) }})
				}
				total := 0
				var kinds []string
				for _, x := range list {
					total += x.bytes
					kinds = append(kinds, x.kind)
				}
				items = append(items, m3.MenuItem{Icon: icons.DeleteForeverOutlined, Label: l10n.DownloadsDeleteAllFiles, Danger: true, OnTap: func() {
					v.delete(lectureID, kinds, l10n.DownloadsLectureAllFiles(core.FmtBytes(float64(total))))
				}})
				return items
			})
		})
	})
}
