package app

import (
	"regexp"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// bot_widgets.dart: chat pieces of the bot operating the app: a chip for
// an operation done at once, and a card for a change that waits for the
// user (docs/specs/0007-resident-bot-and-course-agents.md).

var proposalKindIcons = map[string]icons.Icon{
	"settings": icons.TuneRounded, "download": icons.DownloadRounded, "delete_files": icons.DeleteSweepOutlined,
	"downloads": icons.DownloadingRounded, "course": icons.SchoolOutlined, "task": icons.ScheduleRounded,
	"plugin": icons.ExtensionOutlined,
}

// AppOpChip is an operation the bot already did, with 返回 (navigation)
// or 撤销. OnAction runs once: after it succeeds the chip shows
// DoneLabel; Done says the action already ran.
type AppOpChip struct {
	Label                  string
	Icon                   icons.Icon
	ActionLabel, DoneLabel string
	OnAction               func(done func(error))
	Done                   bool
}

type appOpChipState struct{ busy, ran bool }

// Show builds the chip.
func (a AppOpChip) Show(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	row := ui.Row(c).AlignItems(ui.Center).MinHeight(40).Padding(0, design.SpaceXS, 0, design.SpaceMD).Radius(design.CornerFull).Shrink(1)
	st := ui.Local(row, "app-op", func() *appOpChipState { return &appOpChipState{} })
	done := a.Done || (*st).ran
	fg, bg := cs.OnTertiaryContainer, cs.TertiaryContainer
	if done {
		fg, bg = cs.OnSurfaceVariant, cs.SurfaceContainerHigh
	}
	row.Background(bg).Children(func() {
		ic := a.Icon
		label := a.Label
		if done {
			ic = icons.UndoRounded
			dl := a.DoneLabel
			if dl == "" {
				dl = l10n.CommonUndone
			}
			label = dl + " · " + a.Label
		}
		m3.Icon(c, ic, 16, fg)
		m3.HGap(c, design.SpaceSM)
		m3.Clamp(m3.Text(c, label, tt.LabelLarge.C(fg)), 1).Shrink(1)
		if a.OnAction != nil && !done {
			m3.HGap(c, design.SpaceXS)
			al := a.ActionLabel
			if al == "" {
				al = l10n.CommonUndo
			}
			if (m3.Button{Kind: m3.TextKind, Label: al, FG: cs.OnTertiaryContainer, Disabled: (*st).busy, Height: 32}).Show(c).Clicked() {
				(*st).busy = true
				a.OnAction(func(err error) {
					(*st).busy = false
					if err == nil {
						(*st).ran = true
					}
				})
			}
		} else {
			m3.HGap(c, design.SpaceSM)
		}
	})
}

var apiErrorPrefix = regexp.MustCompile(`^ApiError\(\d+\): `)

type proposalState struct{ busy, err string }

// ProposalCard is a change the bot proposed: its lines and 应用 / 忽略,
// then the outcome.
func ProposalCard(c *ui.Context, p core.J, onApply, onDismiss, onUndo func(done func(error))) {
	cs := design.S()
	tt := design.TT()
	state := p.Or("state", "pending")
	if state == "" {
		state = "pending"
	}
	pending := state == "pending"
	var lines []string
	for _, l := range p.L("lines") {
		if s := core.Str(l); s != "" {
			lines = append(lines, s)
		}
	}
	outcome, outcomeIcon := l10n.ProposalPending, icons.PendingOutlined
	switch state {
	case "applied":
		outcome, outcomeIcon = l10n.ProposalApplied, icons.CheckCircleRounded
	case "dismissed":
		outcome, outcomeIcon = l10n.ProposalIgnored, icons.DoNotDisturbOnOutlined
	case "undone":
		outcome, outcomeIcon = l10n.CommonUndone, icons.UndoRounded
	case "expired":
		outcome, outcomeIcon = l10n.ProposalExpired, icons.ScheduleRounded
	case "failed":
		outcome, outcomeIcon = l10n.ProposalFailed, icons.ErrorOutlineRounded
	}
	bg, fg, muted := cs.SurfaceContainerHigh, cs.OnSurface, cs.OnSurfaceVariant
	if pending {
		bg, fg, muted = cs.SecondaryContainer, cs.OnSecondaryContainer, cs.OnSecondaryContainer
	}
	card := ui.Column(c.Key("proposal-"+p.S("id"))).FillWidth().MaxWidth(520).Radius(design.CornerLG).
		Padding(design.SpaceMD, design.SpaceSM, design.SpaceSM, design.SpaceLG).
		Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Colors: true})
	st := ui.Local(card, "proposal", func() *proposalState { return &proposalState{} })
	run := func(which string, f func(done func(error))) {
		(*st).busy, (*st).err = which, ""
		f(func(err error) {
			(*st).busy = ""
			if err != nil {
				(*st).err = apiErrorPrefix.ReplaceAllString(err.Error(), "")
			}
		})
	}
	card.Background(bg).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ic, ok := proposalKindIcons[p.S("kind")]
			if !ok {
				ic = icons.TouchAppOutlined
			}
			icC := cs.OnSurfaceVariant
			if pending {
				icC = cs.OnSecondaryContainer
			}
			m3.Icon(c, ic, 20, icC)
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, p.S("title"), tt.TitleSmall.C(fg)).Grow(1).Basis(0)
		})
		if len(lines) > 0 {
			m3.VGap(c, design.SpaceSM)
			for i, l := range lines {
				if i >= 14 {
					break
				}
				m3.Text(c, l, tt.BodySmall.C(muted)).Padding(0, design.SpaceSM, 2, 28)
			}
		}
		if (*st).err != "" {
			m3.Text(c, (*st).err, tt.BodySmall.C(cs.Error)).Padding(design.SpaceXS, design.SpaceSM, 0, 28)
		}
		m3.VGap(c, design.SpaceXS)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			m3.HGap(c, 28)
			busy := (*st).busy != ""
			if !pending {
				oc := cs.OnSurfaceVariant
				if state == "failed" {
					oc = cs.Error
				}
				m3.Icon(c, outcomeIcon, 16, oc)
				m3.HGap(c, design.SpaceXS)
				m3.Text(c, outcome, tt.LabelMedium.C(cs.OnSurfaceVariant)).Grow(1).Basis(0)
				if state == "applied" && p.IsTrue("undoable") {
					if (m3.Button{Kind: m3.TextKind, Label: l10n.CommonUndo, Disabled: busy}).Show(c).Clicked() {
						run("undo", onUndo)
					}
				}
				return
			}
			ui.Box(c).Grow(1)
			if (m3.Button{Kind: m3.TextKind, Label: l10n.CommonIgnore, Disabled: busy}).Show(c).Clicked() {
				run("dismiss", onDismiss)
			}
			m3.HGap(c, design.SpaceXS)
			b := m3.Button{Kind: m3.Filled, Label: l10n.CommonApply, Disabled: busy}
			if (*st).busy == "apply" {
				b.Child = func(ui.Color) { m3.Spinner(c, 18, 2, cs.OnPrimary) }
			}
			if b.Show(c).Clicked() {
				run("apply", onApply)
			}
		})
	})
}
