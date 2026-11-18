package app

import (
	"os"
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

// maxVideo is the largest video the daemon imports.
const maxVideo = 4 << 30

// OpenLocalCourseDialog creates a course on this device, or with courseID
// adds videos to one. done gets the course id once the course exists, even
// if an import failed or the window was closed afterwards.
func OpenLocalCourseDialog(courseID string, done func(string)) {
	d := &localCourse{course: courseID, creating: courseID == ""}
	r := m3.AdaptiveSheet(nil, 0, true, d.view, func(v any) {
		id, _ := v.(string)
		if done != nil && id != "" {
			done(id)
		}
	})
	// Esc, back and the barrier close the window unless videos are
	// uploading, and still report a course that already exists.
	r.OnDismiss = func() {
		if !d.busy {
			r.Pop(d.course)
		}
	}
}

// StartLocalCourse creates a course on this device and opens it. onCreated
// runs once the course exists, before it opens.
func StartLocalCourse(onCreated ...func(string)) {
	OpenLocalCourseDialog("", func(id string) {
		for _, f := range onCreated {
			f(id)
		}
		State.LoadCoursesThen(false, false, func() { m3.Nav().Push(NewCoursePage(id)) })
	})
}

type localFile struct {
	path string
	size int64
}

type localCourse struct {
	title, teacher string
	files          []localFile
	course         string
	creating       bool
	busy           bool
	done           int
	err            string
	titleErr       string
}

func (d *localCourse) tooLarge() bool {
	return slices.ContainsFunc(d.files, func(f localFile) bool { return f.size > maxVideo })
}

func (d *localCourse) pick() {
	core.PickFiles(l10n.LocalVideosChoose, true, false, []core.FileFilter{{Name: "Video", Extensions: []string{"mp4", "mov", "m4v", "mkv", "webm"}}}, func(paths []string) {
		if len(paths) == 0 {
			return
		}
		var files []localFile
		for _, p := range paths {
			st, err := os.Stat(p)
			if err != nil {
				d.err = l10n.LocalVideosPickFailed(err.Error())
				return
			}
			files = append(files, localFile{p, st.Size()})
		}
		slices.SortFunc(files, func(a, b localFile) int { return strings.Compare(filepath.Base(a.path), filepath.Base(b.path)) })
		d.files, d.done, d.err = files, 0, ""
	})
}

func (d *localCourse) save(r *m3.Route) {
	if d.busy || d.tooLarge() {
		return
	}
	if d.course == "" && strings.TrimSpace(d.title) == "" {
		d.titleErr = l10n.LocalCourseNameRequired
		return
	}
	d.titleErr = ""
	d.busy, d.err = true, ""
	api := State.Api()
	if d.course == "" {
		title, teacher := strings.TrimSpace(d.title), strings.TrimSpace(d.teacher)
		core.Async(func() (any, error) { return api.CreateLocalCourse(title, teacher) }, func(v any, err error) {
			if err != nil {
				d.busy, d.err = false, l10n.LocalCourseCreateFailed(err)
				return
			}
			d.course = core.Obj(v).S("id")
			d.next(r)
		})
		return
	}
	d.next(r)
}

// next imports the videos from the first one not yet done.
func (d *localCourse) next(r *m3.Route) {
	if !r.Open() {
		return
	}
	if d.done >= len(d.files) {
		d.busy = false
		r.Pop(d.course)
		return
	}
	f, course, api := d.files[d.done], d.course, State.Api()
	core.Async(func() (any, error) {
		fh, err := os.Open(f.path)
		if err != nil {
			return nil, err
		}
		defer fh.Close()
		return api.ImportLocalVideo(course, filepath.Base(f.path), fh, f.size)
	}, func(_ any, err error) {
		if err != nil {
			d.busy, d.err = false, l10n.LocalVideoImportFailed(filepath.Base(f.path), err)
			return
		}
		d.done++
		d.next(r)
	})
}

func (d *localCourse) submitLabel() string {
	switch {
	case d.err != "":
		if d.course != "" && d.done < len(d.files) {
			return l10n.LocalVideosResume
		}
		return l10n.CommonRetry
	case d.course == "" && len(d.files) == 0:
		return l10n.LocalCourseSubmit
	case d.course == "":
		return l10n.LocalCourseSubmitWithVideos(len(d.files))
	}
	return l10n.LocalVideosSubmit(len(d.files))
}

func (d *localCourse) view(c *ui.Context, r *m3.Route) {
	cs := design.S()
	tt := design.TT()
	locked := d.busy || d.course != ""
	muted := tt.BodyMedium.C(cs.OnSurfaceVariant)
	ui.Scroll(c).FillWidth().Shrink(1).Children(func() {
		ui.Column(c).FillWidth().Padding(m3.SheetPadding(c, false)).Children(func() {
			head, body := l10n.LocalVideosAdd, l10n.LocalVideosSectionHint
			if d.creating {
				head, body = l10n.LocalCourseCreate, l10n.LocalCourseCreateBody
			}
			m3.Text(c, head, tt.HeadlineSmall)
			m3.VGap(c, design.SpaceSM)
			m3.Text(c, body, muted)
			m3.VGap(c, design.SpaceXL)
			if d.creating {
				m3.Field{Label: l10n.LocalCourseName, Hint: l10n.LocalCourseNameHint, AutoFocus: true, Disabled: locked,
					MaxLength: 200, Counter: true, Error: d.titleErr}.Show(c, &d.title)
				m3.VGap(c, design.SpaceSM)
				m3.Field{Label: l10n.LocalCourseTeacher, Disabled: locked, MaxLength: 100, Counter: true}.Show(c, &d.teacher)
				m3.VGap(c, design.SpaceLG)
				m3.Text(c, l10n.LocalVideosSection, tt.TitleSmall)
				m3.VGap(c, design.SpaceXS)
				m3.Text(c, l10n.LocalVideosSectionHint, tt.BodySmall.C(cs.OnSurfaceVariant))
				m3.VGap(c, design.SpaceMD)
			}
			if len(d.files) == 0 {
				ui.Row(c).Children(func() {
					if (m3.Button{Kind: m3.Outlined, Icon: icons.VideoLibraryOutlined, Label: l10n.LocalVideosChoose, Disabled: d.busy}).Show(c).Clicked() {
						d.pick()
					}
				})
			} else {
				d.fileList(c)
				m3.VGap(c, design.SpaceSM)
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					var total int64
					for _, f := range d.files {
						total += f.size
					}
					m3.Text(c, l10n.LocalVideosSummary(len(d.files), core.FmtBytes(float64(total))), tt.BodySmall.C(cs.OnSurfaceVariant)).Grow(1).Basis(0)
					if d.done == 0 && (m3.Button{Kind: m3.TextKind, Label: l10n.LocalVideosRechoose, Disabled: d.busy}).Show(c).Clicked() {
						d.pick()
					}
				})
			}
			if d.busy && len(d.files) > 0 {
				m3.VGap(c, design.SpaceMD)
				m3.Linear(c, m3.Progress{Value: m3.Of(float64(d.done) / float64(len(d.files)))})
				m3.VGap(c, design.SpaceSM)
				m3.Text(c, l10n.LocalVideosProgress(d.done, len(d.files)), muted)
			}
			if d.err != "" {
				m3.VGap(c, design.SpaceLG)
				widgets.InlineError(c, d.err)
				if d.course != "" && d.done < len(d.files) {
					m3.VGap(c, design.SpaceSM)
					m3.Text(c, l10n.LocalVideosPartialSaved, tt.BodySmall.C(cs.OnSurfaceVariant))
				}
			}
			m3.VGap(c, design.SpaceXL)
			ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(design.SpaceSM).Children(func() {
				cancel := l10n.CommonCancel
				if d.creating && d.course != "" {
					cancel = l10n.LocalVideosLater
				}
				if (m3.Button{Kind: m3.TextKind, Label: cancel, Disabled: d.busy}).Show(c).Clicked() {
					r.Pop(d.course)
				}
				off := d.busy || d.tooLarge() || (d.course != "" && len(d.files) == 0)
				if (m3.Button{Kind: m3.Filled, Label: d.submitLabel(), Disabled: off}).Show(c).Clicked() {
					d.save(r)
				}
			})
		})
	})
}

// fileList is the chosen videos in import order, each marked with its state.
func (d *localCourse) fileList(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	removable := !d.busy && d.done == 0
	ui.Scroll(c).FillWidth().MaxHeight(248).Border(1, cs.OutlineVariant).Radius(design.CornerMD).Children(func() {
		ui.Column(c).FillWidth().Padding(design.SpaceXS, 0).Children(func() {
			for i, f := range d.files {
				big := f.size > maxVideo
				sub := core.FmtBytes(float64(f.size))
				t := m3.Tile{Dense: true, Title: filepath.Base(f.path), TitleLines: 1, Subtitle: sub, Leading: func() {
					ui.Box(c).Size(24, 24).Center().Children(func() {
						switch {
						case i < d.done:
							m3.Icon(c, icons.CheckCircleRounded, 24, cs.Primary)
						case d.busy && i == d.done:
							m3.Spinner(c, 20, 2.5, cs.Primary)
						case big:
							m3.Icon(c, icons.ErrorOutlineRounded, 24, cs.Error)
						default:
							m3.Text(c, itoa(i+1), tt.LabelLarge.C(cs.OnSurfaceVariant))
						}
					})
				}}
				if big {
					t.SubtitleFn = func() { m3.Text(c, l10n.LocalVideoLimit, tt.BodySmall.C(cs.Error)) }
				}
				if removable {
					t.Trailing = func() {
						if m3.IconBtn(c, icons.CloseRounded, l10n.CommonRemove).Clicked() {
							d.files = slices.Delete(slices.Clone(d.files), i, i+1)
							d.err = ""
						}
					}
				}
				ui.Box(c.Key(f.path)).FillWidth().Children(func() { t.Show(c) })
			}
		})
	})
}
