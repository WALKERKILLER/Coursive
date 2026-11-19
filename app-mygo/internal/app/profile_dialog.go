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

// UserDisplayName is the nickname, else the signed-in account's name.
func UserDisplayName() string { return Profile.DisplayName(State.AccountName()) }

// UserAvatar is the configured avatar.
func UserAvatar(c *ui.Context, size float32) ui.Element {
	return widgets.UserAvatar(c, Profile, UserDisplayName(), size)
}

func randomIdeas() []string {
	out := make([]string, 6)
	for i := range out {
		out[i] = core.RandomSeed()
	}
	return out
}

// ShowProfileDialog edits the nickname and the avatar; changes apply at once.
func ShowProfileDialog() {
	p := Profile
	name := p.Nickname
	ideas := randomIdeas()
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		s := design.S()
		tt := design.TT()
		fallback := State.AccountName()
		toned := p.Kind == core.AvatarEmoji || p.Kind == core.AvatarInitial
		w, _ := c.Size()
		inset := float32(0)
		if design.SizeClassOf(w) == design.SizeCompact {
			inset = design.SpaceLG - 40
		}
		m3.Dialog(c, func() {
			ui.Scroll(c).MaxWidth(460).Margin(0, inset).Shrink(1).Children(func() {
				ui.Column(c).FillWidth().Padding(design.SpaceXL, design.SpaceXL, design.SpaceLG, design.SpaceXL).Children(func() {
					m3.Text(c, l10n.ProfileTitle, tt.HeadlineSmall.C(s.OnSurface))
					m3.VGap(c, design.SpaceXS)
					m3.Text(c, l10n.ProfileLocalOnly, tt.BodySmall.C(s.OnSurfaceVariant))
					m3.VGap(c, design.SpaceXL)
					ui.Row(c).Justify(ui.Center).Children(func() {
						m3.Menu(c, "avatar", m3.Below, func(t ui.Element, open bool) { editableAvatar(c, t) }, func() []m3.MenuItem {
							return []m3.MenuItem{
								{Icon: icons.EmojiEmotionsOutlined, Label: l10n.ProfilePickEmoji, OnTap: func() {
									initial := ""
									if p.Kind == core.AvatarEmoji {
										initial = p.Value
									}
									ShowEmojiPicker(initial, func(e string) { p.SetEmoji(e, -1) })
								}},
								{Icon: icons.ImageOutlined, Label: l10n.ProfileFromFile, OnTap: func() {
									core.PickFile(l10n.CommonImage, []core.FileFilter{core.ImageFilter}, func(path string) {
										if path == "" {
											return
										}
										if err := p.SetFile(path); err != nil {
											widgets.ShowSnack(l10n.ProfileImageFailed(err), "", nil)
										}
									})
								}},
								{Icon: icons.LinkRounded, Label: l10n.ProfileFromUrl, OnTap: func() { showLinkDialog(false, p.SetURL) }},
								{Icon: icons.ChatBubbleOutlineRounded, Label: l10n.ProfileFromQq, OnTap: func() { showLinkDialog(true, p.SetQQ) }},
								{Icon: icons.AutoAwesomeOutlined, Label: l10n.ProfileRandomShape, OnTap: func() { p.SetArt(core.RandomSeed()) }},
								{Divider: true},
								{Icon: icons.RestartAltRounded, Label: l10n.ProfileResetInitial, OnTap: p.Reset},
							}
						})
					})
					m3.VGap(c, design.SpaceXL)
					f := m3.Field{Label: l10n.ProfileNickname, Hint: fallback, MaxLength: 24, Counter: true, Prefix: icons.BadgeOutlined}
					if fallback != "" {
						f.Helper = l10n.ProfileNicknameHelper(fallback)
					}
					in := f.Show(c, &name)
					if in.Changed() {
						p.SetNickname(name)
					}
					if in.Submitted() {
						r.Close()
					}
					m3.VGap(c, design.SpaceLG)
					ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
						m3.Text(c, l10n.ProfileInspiration, tt.TitleSmall.C(s.OnSurface))
						m3.HGap(c, design.SpaceSM)
						m3.Clamp(m3.Text(c, l10n.ProfileTapToUse, tt.BodySmall.C(s.OnSurfaceVariant)), 1).Grow(1).Basis(0)
						if m3.IconBtn(c, icons.ShuffleRounded, l10n.ProfileShuffle).Clicked() {
							ideas = randomIdeas()
						}
					})
					m3.VGap(c, design.SpaceXS)
					row := ui.Row(c.Key(ideas[0])).FillWidth().Justify(ui.SpaceBetween).
						Transition(design.Switcher(design.Medium2, nil, nil, ui.Motion{Opacity: 0}))
					bw := row.Bounds().W
					size := float32(52)
					if bw > 0 {
						size = min(52, (bw-design.SpaceXS*5)/6-10)
					}
					row.Children(func() {
						for _, seed := range ideas {
							sel := p.Kind == core.AvatarArt && p.Value == seed
							pick := ui.Box(c.Key(seed)).Padding(3).Radius(design.CornerFull).Cursor(ui.CursorPointer).
								Border(2, map[bool]ui.Color{true: s.Primary, false: ui.Transparent}[sel]).
								Transition(ui.ElementTransition{Duration: design.Medium1, Colors: true})
							m3.InkWell(pick, design.CornerFull)
							pick.Children(func() {
								widgets.Avatar{Kind: core.AvatarArt, Value: seed, Frame: p.Frame, Size: size}.Show(c)
							})
							if pick.Clicked() {
								p.SetArt(seed)
							}
						}
					})
					m3.VGap(c, design.SpaceLG)
					m3.Text(c, l10n.ProfileShape, tt.TitleSmall.C(s.OnSurface))
					m3.VGap(c, design.SpaceSM)
					var segs []m3.Segment[core.AvatarFrame]
					for _, f := range core.AvatarFrames {
						segs = append(segs, m3.Segment[core.AvatarFrame]{Value: f, Label: f.Label()})
					}
					frame := p.Frame
					if m3.Segmented(c, segs, &frame, m3.SegOpts{NoCheck: true, Fill: true}) {
						p.Update(func(x *core.UserProfile) { x.Frame = frame })
					}
					if toned {
						ui.Column(c.Key("tones")).FillWidth().Padding(design.SpaceLG, 0, 0, 0).
							Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Enter: &ui.Motion{Collapse: true, Opacity: 0}, Exit: &ui.Motion{Collapse: true, Opacity: 0}}).
							Children(func() {
								m3.Text(c, l10n.ProfileBackground, tt.TitleSmall.C(s.OnSurface))
								m3.VGap(c, design.SpaceSM)
								ui.Row(c).Wrap().Gap(design.SpaceSM).Children(func() {
									for i := range widgets.AvatarTones {
										if toneDot(c, i, p.Tone == i) {
											i := i
											p.Update(func(x *core.UserProfile) { x.Tone = i })
										}
									}
								})
							})
					}
					m3.VGap(c, design.SpaceMD)
					show := p.ShowQuote
					if m3.SwitchTile(c, m3.Tile{Title: l10n.ProfileShowQuote, Subtitle: l10n.ProfileShowQuoteHint, Pad: true}, &show) {
						p.Update(func(x *core.UserProfile) { x.ShowQuote = show })
					}
					m3.VGap(c, design.SpaceSM)
					ui.Row(c).Justify(ui.End).Children(func() {
						if m3.FilledButton(c, l10n.CommonFinished).Clicked() {
							r.Close()
						}
					})
				})
			})
		})
	}, nil)
}

// editableAvatar is the large avatar with an edit badge that opens the menu.
func editableAvatar(c *ui.Context, t ui.Element) {
	s := design.S()
	t.Cursor(ui.CursorPointer).Label(l10n.ProfileChangeAvatar)
	m3.Tip(c, t, l10n.ProfileChangeAvatar)
	sc := t.AnimateTo("hover", map[bool]float32{true: 1.04, false: 1}[t.Hovered()], design.T().Motion(design.Medium2, design.Emphasized))
	t.Scale(sc, sc).Children(func() {
		key := string(Profile.Kind) + "|" + Profile.Value + "|" + string(Profile.Frame)
		ui.Box(c.Key(key)).
			Transition(design.Switcher(design.Medium2, design.EmphasizedDecelerate, nil, ui.Motion{Opacity: 0, Scale: .85})).Children(func() { UserAvatar(c, 104) })
		ui.Box(c).Absolute().Right(-2).Bottom(-2).Size(32, 32).Radius(16).Background(s.Primary).Border(3, s.SurfaceContainerHigh).
			Center().Children(func() { m3.Icon(c, icons.EditRounded, 16, s.OnPrimary) })
	})
}

func toneDot(c *ui.Context, i int, selected bool) bool {
	s := design.S()
	bg, fg := widgets.AvatarToneColors(i)
	side, w := s.OutlineVariant, float32(1)
	if selected {
		side, w = s.Primary, 2.5
	}
	d := ui.Box(c.Key(i)).Size(32, 32).Radius(16).Background(bg).Border(w, side).Center().Cursor(ui.CursorPointer).
		Transition(ui.ElementTransition{Duration: design.Medium1, Colors: true})
	tip := l10n.ProfileBackgroundIndex(i)
	if i == 0 {
		tip = l10n.CommonFollowTheme
	}
	m3.Tip(c, d, tip)
	d.Children(func() {
		if selected {
			m3.Icon(c, icons.CheckRounded, 16, fg)
		} else if i == 0 {
			m3.Icon(c, icons.PaletteOutlined, 14, fg)
		}
	})
	return d.Clicked()
}

var urlRe = regexp.MustCompile(`^https?://\S+$`)

// showLinkDialog asks for an image link or a QQ number, with a debounced
// preview.
func showLinkDialog(qq bool, use func(string)) {
	text, preview := "", ""
	var debounce *time.Timer
	valid := func(s string) bool {
		if qq {
			return core.ValidQQ(s)
		}
		return urlRe.MatchString(strings.TrimSpace(s))
	}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		s := design.S()
		ok := valid(text)
		ic, title := icons.LinkRounded, l10n.ProfileImageUrl
		label, hint, invalid := l10n.ProfileImageAddress, "https://example.com/avatar.png", l10n.ProfileUrlInvalid
		if qq {
			ic, title = icons.ChatBubbleOutlineRounded, l10n.ProfileUseQq
			label, hint, invalid = l10n.ProfileQqNumber, l10n.ProfileQqHint, l10n.ProfileQqInvalid
		}
		m3.AlertDialog(c, m3.Alert{Icon: ic, Title: title, Width: 360, Content: func() {
			ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Box(c.Key(preview)).Transition(design.Switcher(design.Medium2, nil, nil, ui.Motion{Opacity: 0})).Children(func() {
					if preview == "" {
						widgets.CircleAvatar(c, 88, s.SurfaceContainerHighest, func() { m3.Icon(c, icons.ImageSearchRounded, 24, s.OnSurfaceVariant) })
					} else {
						widgets.Avatar{Kind: core.AvatarURL, Value: preview, Size: 88, Name: "?"}.Show(c)
					}
				})
				m3.VGap(c, design.SpaceLG)
				f := m3.Field{Label: label, Hint: hint, AutoFocus: true}
				if text != "" && !ok {
					f.Error = invalid
				}
				in := f.Show(c, &text)
				if in.Changed() {
					if debounce != nil {
						debounce.Stop()
					}
					v := text
					debounce = time.AfterFunc(450*time.Millisecond, func() {
						core.Post(func() {
							preview = ""
							if valid(v) {
								preview = strings.TrimSpace(v)
								if qq {
									preview = core.QQAvatarURL(strings.TrimSpace(v))
								}
							}
						})
					})
				}
				if in.Submitted() && valid(text) {
					r.Pop(strings.TrimSpace(text))
				}
				if qq {
					m3.VGap(c, design.SpaceSM)
					m3.Text(c, l10n.ProfileQqNote, design.TT().BodySmall.C(s.OnSurfaceVariant)).FillWidth()
				}
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Close()
			}
			if (m3.Button{Kind: m3.Filled, Label: l10n.CommonUse, Disabled: !ok}).Show(c).Clicked() {
				r.Pop(strings.TrimSpace(text))
			}
		}})
	}, func(v any) {
		if debounce != nil {
			debounce.Stop()
		}
		if s, ok := v.(string); ok && s != "" {
			use(s)
		}
	})
}

var emojiGroups = []struct{ label, list string }{
	{l10n.EmojiStudy, "📚 📖 ✏️ 📝 🎓 🧠 💡 🔬 🔭 🧪 🧮 📐 📏 🖊️ 🗂️ 📎 🎒 🏫 💻 ⌨️ 📊 🧭 🌏 🗺️"},
	{l10n.EmojiFaces, "😀 😄 😁 😆 😊 🙂 😉 😍 🥰 😎 🤓 🧐 🤔 😴 🥱 😌 😇 🤗 🤩 🥳 😺 😸 🫠 🫡"},
	{l10n.EmojiAnimals, "🐱 🐶 🐼 🐨 🦊 🐻 🐰 🐹 🐯 🦁 🐮 🐷 🐸 🐵 🐧 🐦 🦉 🦄 🐝 🦋 🐢 🐬 🐳 🦕"},
	{l10n.EmojiNature, "🌸 🌼 🌻 🌷 🌹 🍀 🌿 🌵 🌲 🍁 🍂 🌙 ⭐ 🌈 ☀️ ⛅ ❄️ 🌊 🔥 ⚡ 🪐 🌍 🍄 🌾"},
	{l10n.EmojiFood, "🍎 🍊 🍋 🍉 🍓 🍑 🍒 🥝 🍞 🥐 🍩 🍪 🍰 🧁 🍫 🍬 🍙 🍜 🍣 🥟 ☕ 🧋 🍵 🥤"},
	{l10n.EmojiHobbies, "⚽ 🏀 🏸 🏓 🎾 🏐 🎮 🎧 🎸 🎹 🎨 📷 🎬 🚀 ✈️ 🚲 ⛵ 🏝️ 🎈 🎁 🔑 🧩 🪴 ⏰"},
}

// firstGrapheme is characters.first, close enough for emoji: the first
// rune with the joiners, selectors and modifiers that follow it.
func firstGrapheme(s string) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) == 0 {
		return ""
	}
	end := 1
	for end < len(rs) {
		r := rs[end]
		switch {
		case r == 0x200D && end+1 < len(rs):
			end += 2
		case r == 0xFE0F || r == 0xFE0E || (r >= 0x1F3FB && r <= 0x1F3FF) || r == 0x20E3 || (r >= 0xE0020 && r <= 0xE007F):
			end++
		case r >= 0x1F1E6 && r <= 0x1F1FF && rs[0] >= 0x1F1E6 && rs[0] <= 0x1F1FF && end == 1:
			end++
		default:
			return string(rs[:end])
		}
	}
	return string(rs[:end])
}

// ShowEmojiPicker is a grid of curated emoji with free input for any.
func ShowEmojiPicker(initial string, pick func(string)) {
	text := initial
	group := 0
	scroll := &ui.ScrollState{}
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		s := design.S()
		typed := firstGrapheme(text)
		list := strings.Split(emojiGroups[group].list, " ")
		m3.AlertDialog(c, m3.Alert{Title: l10n.ProfilePickEmoji, Width: 380, Content: func() {
			ui.Column(c).FillWidth().Children(func() {
				shown, col := typed, ui.Color{}
				if shown == "" {
					shown, col = "🙂", s.Outline
				}
				in := m3.Field{Hint: l10n.ProfileEmojiHint, PrefixFn: func() {
					ui.Box(c).Padding(design.SpaceSM).Children(func() {
						st := design.TextStyle{Size: 22, Color: col}
						m3.Text(c, shown, st)
					})
				}}.Show(c, &text)
				if in.Submitted() && typed != "" {
					r.Pop(typed)
				}
				m3.VGap(c, design.SpaceMD)
				widgets.HScroller{State: scroll, Fade: true, Buttons: true}.Show(c, func() {
					for i, g := range emojiGroups {
						if m3.ChoiceChipOf(c.Key(i), g.label, group == i) {
							group = i
						}
						m3.HGap(c, design.SpaceXS)
					}
				})
				m3.VGap(c, design.SpaceMD)
				ui.Scroll(c.Key(group)).FillWidth().Height(200).Transition(design.Switcher(design.Medium1, nil, nil, ui.Motion{Opacity: 0})).Children(func() {
					ui.Row(c).Wrap().Gap(4).FillWidth().Children(func() {
						for i, e := range list {
							cell := ui.Box(c.Key(i)).Size(44, 44).Radius(design.CornerMD).Center().Cursor(ui.CursorPointer)
							m3.InkWell(cell, design.CornerMD)
							cell.Children(func() { m3.Text(c, e, design.TextStyle{Size: 26}) })
							if cell.Clicked() {
								r.Pop(e)
							}
						}
					})
				})
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Close()
			}
			if (m3.Button{Kind: m3.Filled, Label: l10n.CommonUse, Disabled: typed == ""}).Show(c).Clicked() {
				r.Pop(typed)
			}
		}})
	}, func(v any) {
		if e, ok := v.(string); ok && e != "" {
			pick(e)
		}
	})
}
