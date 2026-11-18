package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// courseware.dart: courseware uploaded to a lecture, which MinerU parses.

// CoursewareExts are kCoursewareExts.
var CoursewareExts = []string{"pdf", "ppt", "pptx", "doc", "docx", "xls", "xlsx", "png", "jpg", "jpeg", "webp"}

// PickAndUploadCourseware lets the user pick documents and uploads them to
// a lecture; the daemon queues MinerU parsing when automatic parsing is
// on. done gets how many it uploaded.
func PickAndUploadCourseware(lectureID string, done func(int)) {
	core.PickFiles(l10n.AssetCourseware, true, false, []core.FileFilter{{Name: l10n.AssetCourseware, Extensions: CoursewareExts}}, func(paths []string) {
		if len(paths) == 0 {
			return
		}
		type result struct {
			ok     int
			queued bool
		}
		core.Async(func() (result, error) {
			var res result
			for _, p := range paths {
				name := filepath.Base(p)
				err := func() error {
					f, err := os.Open(p)
					if err != nil {
						return err
					}
					defer f.Close()
					st, err := f.Stat()
					if err != nil {
						return err
					}
					r, err := State.Api().Upload("/api/lectures/"+core.Esc(lectureID)+"/courseware", map[string]string{"name": name}, f, st.Size())
					if err != nil {
						return err
					}
					res.ok++
					res.queued = res.queued || core.Obj(r).IsTrue("parse_queued")
					return nil
				}()
				if err != nil {
					core.Post(func() { widgets.ShowSnack(l10n.CoursewareUploadFailed(name, err), "", nil) })
				}
			}
			return res, nil
		}, func(res result, _ error) {
			if res.ok > 0 {
				msg := l10n.CoursewareUploaded(res.ok)
				if res.queued {
					msg = l10n.CoursewareUploadedParsing(res.ok)
				}
				widgets.ShowSnack(msg, "", nil)
			}
			if done != nil {
				done(res.ok)
			}
		})
	})
}

// FileBadge is the file-type badge: the extension on a tonal square.
func FileBadge(c *ui.Context, ext string) ui.Element {
	cs := design.S()
	bg, fg := cs.SurfaceContainerHighest, cs.OnSurfaceVariant
	switch ext {
	case "pdf":
		bg, fg = cs.ErrorContainer, cs.OnErrorContainer
	case "ppt", "pptx":
		bg, fg = cs.TertiaryContainer, cs.OnTertiaryContainer
	case "doc", "docx":
		bg, fg = cs.PrimaryContainer, cs.OnPrimaryContainer
	case "xls", "xlsx":
		bg, fg = cs.SecondaryContainer, cs.OnSecondaryContainer
	}
	b := ui.Column(c).Size(40, 46).Shrink(0).Justify(ui.End).AlignItems(ui.Center).Padding(0, 0, 6, 0).Background(bg).
		Radius(design.CornerXS, design.CornerMD, design.CornerXS, design.CornerXS)
	b.Children(func() {
		m3.Text(c, strings.ToUpper(ext), design.TT().LabelSmall.C(fg).W(700).S(10))
	})
	return b
}

// CoursewareList is the uploaded courseware of a lecture with its parse
// state and actions.
func CoursewareList(c *ui.Context, lectureID string, files []core.CoursewareFile, onChanged func()) {
	job, ok := State.JobFor(lectureID, "parse")
	parsing := ok && job.Active()
	upload := func() {
		PickAndUploadCourseware(lectureID, func(n int) {
			if n > 0 {
				onChanged()
			}
		})
	}
	if len(files) == 0 {
		widgets.EmptyState{Icon: icons.UploadFileRounded, Title: l10n.CoursewareUploadTitle, Body: l10n.CoursewareUploadBody, Action: func() {
			if (m3.Button{Kind: m3.Tonal, Icon: icons.AddRounded, Label: l10n.CommonChooseFiles}).Show(c).Clicked() {
				upload()
			}
		}}.Show(c)
		return
	}
	cs, tt := design.S(), design.TT()
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(design.SpaceSM, design.SpaceMD, design.SpaceXXL, design.SpaceMD).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				label := l10n.CommonFilesCount(len(files))
				if parsing {
					label = job.Stage()
					if label == "" {
						label = l10n.CoursewareParsing
					}
				}
				m3.Text(c, label, tt.LabelLarge.C(cs.OnSurfaceVariant)).Grow(1).Basis(0)
				if (m3.Button{Kind: m3.TextKind, Icon: icons.UploadRounded, Label: l10n.CommonUpload}).Show(c).Clicked() {
					upload()
				}
			})
			if parsing {
				p := m3.Progress{Radius: 2}
				if job.Progress() > 0 {
					p.Value = m3.Of(job.Progress())
				}
				ui.Box(c).FillWidth().Padding(0, 0, design.SpaceSM, 0).Children(func() { m3.Linear(c, p) })
			}
			for _, f := range files {
				ui.Box(c.Key(f.Path())).FillWidth().Padding(0, 0, design.SpaceSM, 0).Children(func() {
					coursewareCard(c, lectureID, f, parsing, onChanged)
				})
			}
		})
	})
}

func coursewareCard(c *ui.Context, lectureID string, f core.CoursewareFile, parsing bool, onChanged func()) {
	cs, tt := design.S(), design.TT()
	var status string
	color := cs.OnSurfaceVariant
	switch f.ParseState() {
	case "done":
		pages := ""
		if f.Pages() > 0 {
			pages = l10n.CoursewarePagesSuffix(f.Pages())
		}
		status, color = l10n.CoursewareParsed(pages), cs.Primary
	case "failed":
		status, color = l10n.CoursewareParseFailed, cs.Error
	default:
		status = l10n.CoursewareNotParsedState
		if parsing {
			status = l10n.CoursewareParseQueued
		}
	}
	original := func() { core.OpenPath(State.Daemon.LibraryRoot() + "/" + f.Path()) }
	parse := func() {
		State.Enqueue([]string{lectureID}, []string{"parse"}, func(_ []core.Job, err error) {
			if err == nil {
				widgets.ShowSnack(l10n.CoursewareParseQueuedToast, "", nil)
			}
		})
	}
	card := m3.Tappable(c, design.CornerLG).FillWidth().Background(cs.SurfaceContainerHigh)
	if card.Clicked() {
		if f.Parsed() {
			OpenCoursewareReader(f, lectureID, "")
		} else {
			original()
		}
	}
	card.Padding(design.SpaceMD, design.SpaceXS, design.SpaceMD, design.SpaceMD).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			FileBadge(c, f.Ext())
			m3.HGap(c, design.SpaceMD)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				m3.Text(c, f.Name(), tt.TitleSmall).MaxLines(2)
				m3.VGap(c, 2)
				ui.RichText(c, tt.BodySmall.C(cs.OnSurfaceVariant).Span(core.FmtBytes(float64(f.Bytes()))+" · "), tt.BodySmall.C(color).Span(status))
				if f.ParseState() == "failed" && f.ParseError() != "" {
					m3.Text(c, f.ParseError(), tt.BodySmall.C(cs.Error)).MaxLines(2)
				}
			})
			if !f.Parsed() && !parsing && m3.IconBtn(c, icons.DocumentScannerOutlined, l10n.CoursewareParseTooltip).Clicked() {
				parse()
			}
			m3.MenuButton(c, "more", icons.MoreVertRounded, l10n.CommonMore, m3.Over, func() []m3.MenuItem {
				var items []m3.MenuItem
				if f.Parsed() {
					items = append(items, m3.MenuItem{Label: l10n.CoursewareReadParsed, OnTap: func() { OpenCoursewareReader(f, lectureID, "") }})
				}
				items = append(items, m3.MenuItem{Label: l10n.CoursewareOpenOriginal, OnTap: original})
				if !parsing {
					label := l10n.CoursewareParse
					if f.Parsed() {
						label = l10n.CoursewareReparse
					}
					items = append(items, m3.MenuItem{Label: label, OnTap: parse})
				}
				return append(items, m3.MenuItem{Label: l10n.CommonDelete, OnTap: func() {
					m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
						m3.AlertDialog(c, m3.Alert{Title: l10n.CoursewareDeleteTitle, Message: l10n.CoursewareDeleteBody(f.Name()), Actions: func() {
							if m3.TextButton(c, l10n.CommonCancel).Clicked() {
								r.Close()
							}
							if m3.FilledButton(c, l10n.CommonDelete).Clicked() {
								r.Pop(true)
							}
						}})
					}, func(v any) {
						if v != true {
							return
						}
						core.Async(func() (any, error) {
							return State.Api().Delete("/api/lectures/" + core.Esc(lectureID) + "/courseware?path=" + core.Esc(f.Path()))
						}, func(_ any, err error) {
							if err != nil {
								widgets.ShowSnack(err.Error(), "", nil)
							}
							onChanged()
						})
					})
				}})
			})
		})
	})
}
