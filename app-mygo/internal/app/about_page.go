package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/assets"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// The repository Coursive is built from; its releases carry the app.
const (
	repoSlug = "WALKERKILLER/Coursive"
	repoURL  = "https://github.com/" + repoSlug
)

// Release is a Coursive release on GitHub (tag v<version>).
type Release struct {
	Version, Page string
	PublishedAt   time.Time
	// Assets maps a file name to its download address.
	Assets map[string]string
}

// Download is this platform's installer, or "" where the user picks a
// package format (Linux) on the release page.
func (r *Release) Download() string {
	var suffix string
	switch runtime.GOOS {
	case "android":
		suffix = "android-arm64-v8a.apk"
		if runtime.GOARCH == "amd64" {
			suffix = "android-x86_64.apk"
		}
	case "windows":
		suffix = "windows-x64-setup.exe"
	case "darwin":
		suffix = "macos-universal.dmg"
	default:
		return ""
	}
	for name, u := range r.Assets {
		if strings.HasSuffix(name, suffix) {
			return u
		}
	}
	return ""
}

// CompareVersions orders versions such as 0.2.0 and 0.2.0-beta.1; a
// pre-release comes before its release (semantic versioning).
func CompareVersions(a, b string) int {
	split := func(v string) ([]string, []string) {
		if i := strings.IndexByte(v, '-'); i >= 0 {
			return strings.Split(v[:i], "."), strings.Split(v[i+1:], ".")
		}
		return strings.Split(v, "."), nil
	}
	cmp := func(x, y int) int {
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
	num := func(p []string, i int) int {
		if i >= len(p) {
			return 0
		}
		n, _ := strconv.Atoi(p[i])
		return n
	}
	ca, pa := split(a)
	cb, pb := split(b)
	for i := 0; i < 3; i++ {
		if c := cmp(num(ca, i), num(cb, i)); c != 0 {
			return c
		}
	}
	if (len(pa) == 0) != (len(pb) == 0) {
		if len(pa) == 0 {
			return 1
		}
		return -1
	}
	for i := 0; i < len(pa) && i < len(pb); i++ {
		nx, ex := strconv.Atoi(pa[i])
		ny, ey := strconv.Atoi(pb[i])
		var c int
		switch {
		case ex == nil && ey == nil:
			c = cmp(nx, ny)
		case ex == nil: // numeric identifiers sort first
			c = -1
		case ey == nil:
			c = 1
		default:
			c = strings.Compare(pa[i], pb[i])
		}
		if c != 0 {
			return c
		}
	}
	return cmp(len(pa), len(pb))
}

var releaseTag = regexp.MustCompile(`^v\d`)

// NewerRelease is the newest published release after current in GitHub's
// release list. Pre-releases count only for someone already on one.
func NewerRelease(releases []core.J, current string) *Release {
	pre := strings.Contains(current, "-")
	var best *Release
	for _, r := range releases {
		tag := r.S("tag_name")
		if r.IsTrue("draft") || !releaseTag.MatchString(tag) || (r.IsTrue("prerelease") && !pre) {
			continue
		}
		v := tag[1:]
		if CompareVersions(v, current) <= 0 || (best != nil && CompareVersions(v, best.Version) <= 0) {
			continue
		}
		at, _ := time.Parse(time.RFC3339, r.S("published_at"))
		rel := &Release{Version: v, Page: r.S("html_url"), PublishedAt: at.Local(), Assets: map[string]string{}}
		for _, a := range r.Os("assets") {
			rel.Assets[a.S("name")] = a.S("browser_download_url")
		}
		best = rel
	}
	return best
}

func fetchReleases() ([]core.J, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+repoSlug+"/releases?per_page=30", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	var list []any
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		return nil, err
	}
	return core.Objs(list), nil
}

type updateCheck int

const (
	checkIdle updateCheck = iota
	checkRunning
	checkLatest
	checkNewer
	checkFailed
)

// AboutSection is Settings → 关于: version, update check, repository and
// feedback.
type AboutSection struct {
	state   updateCheck
	release *Release
	err     error
}

// NewAboutSection builds the pane.
func NewAboutSection() *AboutSection { return &AboutSection{} }

func appVersion() string { return State.Health.S("version") }

func (a *AboutSection) check() {
	a.state = checkRunning
	version := appVersion()
	core.Async(fetchReleases, func(list []core.J, err error) {
		if err != nil {
			a.err, a.state = err, checkFailed
			return
		}
		a.release = NewerRelease(list, version)
		a.state = checkLatest
		if a.release != nil {
			a.state = checkNewer
		}
	})
}

func feedbackURL() string {
	body := l10n.AboutFeedbackBody(appVersion(), runtime.GOOS+" "+runtime.GOARCH)
	return "https://github.com/" + repoSlug + "/issues/new?body=" + url.QueryEscape(body)
}

// Show builds the pane.
func (a *AboutSection) Show(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	version := appVersion()
	rel := a.release
	var status string
	ic, col := icons.InfoOutlineRounded, cs.OnSurfaceVariant
	switch a.state {
	case checkIdle:
		status = l10n.AboutCheckHint
	case checkRunning:
		status = l10n.AboutChecking
	case checkLatest:
		status, ic = l10n.AboutUpToDate, icons.CheckCircleOutlineRounded
	case checkNewer:
		at := rel.PublishedAt
		if at.IsZero() {
			at = time.Now()
		}
		status, ic, col = l10n.AboutNewVersion(rel.Version, core.FmtRelative(at)), icons.NewReleasesOutlined, cs.Primary
	case checkFailed:
		status, ic, col = l10n.AboutCheckFailed(apiMessage(a.err)), icons.ErrorOutlineRounded, cs.Error
	}
	ui.Column(c).FillWidth().Children(func() {
		m3.VGap(c, design.SpaceXXL)
		aboutHero(c, version)
		SettingsGroup(c, l10n.AboutUpdates, icons.SystemUpdateAltRounded, nil, func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				m3.Icon(c, ic, 20, col)
				m3.HGap(c, design.SpaceMD)
				st := tt.BodyMedium
				if a.state == checkFailed {
					st = st.C(cs.Error)
				}
				m3.Text(c, status, st).Grow(1).Basis(0)
			})
			m3.VGap(c, design.SpaceMD)
			ui.Row(c).FillWidth().Wrap().Gap(design.SpaceSM).Children(func() {
				if rel != nil && a.state == checkNewer {
					if (m3.Button{Kind: m3.Filled, Icon: icons.DownloadRounded, Label: l10n.AboutDownload(rel.Version)}).Show(c).Clicked() {
						u := rel.Download()
						if u == "" {
							u = rel.Page
						}
						widgets.OpenPlainLink(c, u)
					}
					if m3.TextButton(c, l10n.AboutReleaseNotes).Clicked() {
						widgets.OpenPlainLink(c, rel.Page)
					}
					return
				}
				busy := a.state == checkRunning
				if (m3.Button{Kind: m3.Tonal, Icon: icons.RefreshRounded, Label: l10n.AboutCheckUpdates, Disabled: busy, Busy: busy}).Show(c).Clicked() {
					a.check()
				}
			})
		})
		settingsGroupPad(c, l10n.AboutLinks, icons.LinkRounded, nil, [4]float32{design.SpaceSM, 0, design.SpaceSM, 0}, func() {
			link := func(key string, t m3.Tile) bool {
				t.Tap, t.Pad, t.PadX, t.NoRadius = true, true, design.SpaceXL, true
				return t.Show(c.Key(key)).Clicked()
			}
			open := func() { m3.Icon(c, icons.OpenInNewRounded, 18, cs.OnSurfaceVariant) }
			if link("repo", m3.Tile{LeadingIcon: icons.CodeRounded, Title: l10n.AboutRepo, Subtitle: "github.com/" + repoSlug, SubtitleLines: 1, Trailing: open}) {
				widgets.OpenPlainLink(c, repoURL)
			}
			if link("feedback", m3.Tile{LeadingIcon: icons.FeedbackOutlined, Title: l10n.AboutFeedback, Subtitle: l10n.AboutFeedbackHint, Trailing: open}) {
				widgets.OpenPlainLink(c, feedbackURL())
			}
			if link("licenses", m3.Tile{LeadingIcon: icons.DescriptionOutlined, Title: l10n.AboutLicenses, Subtitle: l10n.AboutLicensesHint, TrailingIcon: icons.ChevronRightRounded}) {
				m3.Nav().Push(&licensesPage{version: version})
			}
		})
		m3.VGap(c, design.SpaceXL)
		// Quiet on purpose: a text button in the footer, the code only on
		// request.
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			if (m3.Button{Kind: m3.TextKind, Icon: icons.VolunteerActivismOutlined, Label: l10n.AboutReward, FG: cs.OnSurfaceVariant}).Show(c).Clicked() {
				showReward()
			}
		})
		m3.VGap(c, design.SpaceXL)
	})
}

// aboutHero is the logo, name, a line on what the app does, and the
// version (selectable, marked 预览版 on a pre-release).
func aboutHero(c *ui.Context, version string) {
	cs := design.S()
	tt := design.TT()
	ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
		widgets.Logo(c, 88)
		m3.VGap(c, design.SpaceLG)
		m3.Text(c, "Coursive", tt.HeadlineSmall).TextAlign(ui.Center)
		m3.VGap(c, design.SpaceXS)
		m3.Text(c, l10n.AboutTagline, tt.BodyMedium.C(cs.OnSurfaceVariant)).TextAlign(ui.Center)
		m3.VGap(c, design.SpaceMD)
		ui.Row(c).AlignItems(ui.Center).Padding(design.SpaceXS, design.SpaceMD).Radius(design.CornerFull).Background(cs.SurfaceContainerHigh).Children(func() {
			v := version
			if v == "" {
				v = "—"
			}
			m3.Text(c, l10n.AboutVersion(v), tt.LabelLarge.C(cs.OnSurfaceVariant)).Selectable().Shrink(1)
			if strings.Contains(version, "-") {
				m3.HGap(c, design.SpaceSM)
				m3.Text(c, l10n.AboutPreview, tt.LabelLarge.C(cs.Tertiary))
			}
		})
	})
}

// showReward is the author's WeChat tip code: a bottom sheet on phones, a
// dialog elsewhere.
func showReward() {
	if Compact() {
		m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true}, func(c *ui.Context, r *m3.Route) {
			ui.Scroll(c).FillWidth().Children(func() {
				ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() { reward(c, r, true) })
			})
		})
		return
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) { m3.Dialog(c, func() { reward(c, r, false) }) }, nil)
}

func reward(c *ui.Context, r *m3.Route, sheet bool) {
	cs := design.S()
	tt := design.TT()
	top := float32(design.SpaceXL)
	if sheet {
		top = 0
	}
	ui.Column(c).MaxWidth(360).Padding(top, design.SpaceXL, design.SpaceXL, design.SpaceXL).AlignItems(ui.Center).Children(func() {
		m3.Text(c, l10n.AboutRewardTitle, tt.TitleLarge).TextAlign(ui.Center)
		m3.VGap(c, design.SpaceSM)
		m3.Text(c, l10n.AboutRewardBody, tt.BodyMedium.C(cs.OnSurfaceVariant)).TextAlign(ui.Center)
		m3.VGap(c, design.SpaceXL)
		// The code keeps its own dark card; a hairline separates it from
		// dark surfaces.
		line := ui.RGBA(0, 0, 0, 26)
		if design.T().Dark {
			line = ui.RGBA(255, 255, 255, 26)
		}
		ui.Box(c).Size(240, 240).Radius(design.CornerLG).Clip().Border(1, line).Children(func() {
			if img := assets.Image("reward.png"); img != nil {
				ui.Image(c, img).Size(240, 240).Fit(ui.Contain).Label(l10n.AboutRewardImage)
			}
		})
		m3.VGap(c, design.SpaceLG)
		if sheet {
			m3.Text(c, l10n.AboutRewardPhoneHint, tt.BodySmall.C(cs.OnSurfaceVariant)).TextAlign(ui.Center)
			return
		}
		ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
			if m3.TextButton(c, l10n.CommonClose).Clicked() {
				r.Close()
			}
		})
	})
}

// licensesPage lists the modules built into the app (showLicensePage);
// each module's licence ships with its source.
type licensesPage struct {
	version string
	scroll  ui.ScrollState
}

func (p *licensesPage) MainScroll() *ui.ScrollState { return &p.scroll }

func (p *licensesPage) View(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	SettingsSubpage(c, l10n.AboutLicenses, &p.scroll, func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceLG).Children(func() {
			widgets.Logo(c, 64)
			m3.VGap(c, design.SpaceSM)
			m3.Text(c, "Coursive", tt.HeadlineSmall)
			m3.Text(c, p.version, tt.BodyMedium.C(cs.OnSurfaceVariant))
		})
		info, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}
		for _, m := range info.Deps {
			m3.Tile{Title: m.Path, Subtitle: m.Version, Pad: true, PadX: 0, Dense: true}.Show(c.Key(m.Path))
		}
		for _, name := range []string{"brands/NOTICE", "providers/NOTICE"} {
			if b, err := assets.Bytes(name); err == nil {
				m3.VGap(c, design.SpaceLG)
				m3.Text(c, string(b), tt.BodySmall.C(cs.OnSurfaceVariant)).Selectable()
			}
		}
	})
}
