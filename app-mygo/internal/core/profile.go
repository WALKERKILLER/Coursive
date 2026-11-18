package core

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// AvatarKind is where the avatar comes from.
type AvatarKind string

const (
	AvatarInitial AvatarKind = "initial"
	AvatarEmoji   AvatarKind = "emoji"
	AvatarArt     AvatarKind = "art"
	AvatarFile    AvatarKind = "file"
	AvatarURL     AvatarKind = "url"
)

// AvatarFrame is the outline of the avatar.
type AvatarFrame string

const (
	FrameCircle   AvatarFrame = "circle"
	FrameSquircle AvatarFrame = "squircle"
	FrameCookie   AvatarFrame = "cookie"
	FrameFlower   AvatarFrame = "flower"
)

// AvatarFrames lists the frames in AvatarFrame.values order.
var AvatarFrames = []AvatarFrame{FrameCircle, FrameSquircle, FrameCookie, FrameFlower}

func (f AvatarFrame) Label() string {
	switch f {
	case FrameSquircle:
		return l10n.AvatarShapeSquircle
	case FrameCookie:
		return l10n.AvatarShapeCookie
	case FrameFlower:
		return l10n.AvatarShapePetal
	}
	return l10n.AvatarShapeCircle
}

// UserProfile is the local display profile: nickname, avatar and the home
// greeting's options, kept on this device under the key "profile".
type UserProfile struct {
	Nickname string
	Kind     AvatarKind
	// Value is the emoji, art seed, absolute file path or image URL.
	Value string
	// Tone is the background tone index for emoji and initial avatars.
	Tone      int
	Frame     AvatarFrame
	ShowQuote bool
	// QuoteTypes are hitokoto categories: d 文学, i 诗词, k 哲学, h 影视, a 动画, j 网易云.
	QuoteTypes []string
}

// LoadProfile reads the profile from the store.
func LoadProfile() *UserProfile {
	p := &UserProfile{Kind: AvatarInitial, Frame: FrameCircle, ShowQuote: true, QuoteTypes: []string{"d", "i", "k"}}
	raw, ok := Prefs.String("profile")
	if !ok {
		return p
	}
	v, err := Decode([]byte(raw))
	m := Obj(v)
	if err != nil || m == nil {
		return p
	}
	q := *p
	defer func() {
		if recover() != nil {
			*p = q
		}
	}()
	p.Nickname = m.S("nickname")
	p.Kind = AvatarKind(m.Or("kind", "initial"))
	p.Value = m.S("value")
	p.Tone = m.I("tone")
	p.Frame = AvatarFrame(m.Or("frame", "circle"))
	if m["showQuote"] != nil {
		p.ShowQuote = m["showQuote"].(bool)
	}
	if l, ok := m["quoteTypes"].([]any); ok {
		p.QuoteTypes = Strs(l)
	}
	return p
}

// DisplayName is the nickname, else the signed-in account's name.
func (p *UserProfile) DisplayName(fallback string) string {
	if p.Nickname != "" {
		return p.Nickname
	}
	return fallback
}

func (p *UserProfile) save() {
	Prefs.SetString("profile", Encode(J{
		"nickname": p.Nickname, "kind": string(p.Kind), "value": p.Value, "tone": p.Tone,
		"frame": string(p.Frame), "showQuote": p.ShowQuote, "quoteTypes": p.QuoteTypes,
	}))
}

// SetNickname keeps at most 24 characters.
func (p *UserProfile) SetNickname(v string) {
	t := strings.TrimSpace(v)
	if t == p.Nickname {
		return
	}
	if r := []rune(t); len(r) > 24 {
		t = string(r[:24])
	}
	p.Nickname = t
	p.save()
}

// Update applies f and saves.
func (p *UserProfile) Update(f func(p *UserProfile)) {
	f(p)
	p.save()
}

func (p *UserProfile) setAvatar(k AvatarKind, v string) {
	old := ""
	if p.Kind == AvatarFile {
		old = p.Value
	}
	p.Kind, p.Value = k, v
	p.save()
	if old != "" && old != v {
		_ = os.Remove(old)
	}
}

func (p *UserProfile) SetEmoji(emoji string, tone int) {
	if tone >= 0 {
		p.Tone = tone
	}
	p.setAvatar(AvatarEmoji, emoji)
}
func (p *UserProfile) SetArt(seed string) { p.setAvatar(AvatarArt, seed) }
func (p *UserProfile) SetURL(url string)  { p.setAvatar(AvatarURL, url) }
func (p *UserProfile) SetQQ(qq string)    { p.setAvatar(AvatarURL, QQAvatarURL(qq)) }
func (p *UserProfile) Reset()             { p.setAvatar(AvatarInitial, "") }

// SetFile copies source into the support directory, so the avatar survives
// the original being moved.
func (p *UserProfile) SetFile(source string) error {
	dir := filepath.Join(SupportDir(), "avatar")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(dir, fmt.Sprintf("avatar-%d%s", time.Now().UnixMilli(), strings.ToLower(filepath.Ext(source))))
	b, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dest, b, 0o644); err != nil {
		return err
	}
	p.setAvatar(AvatarFile, dest)
	return nil
}

func QQAvatarURL(qq string) string {
	return "https://q2.qlogo.cn/headimg_dl?dst_uin=" + qq + "&spec=640"
}

var qqRe = regexp.MustCompile(`^[1-9]\d{4,11}$`)

func ValidQQ(s string) bool { return qqRe.MatchString(strings.TrimSpace(s)) }

// RandomSeed is a random art seed.
func RandomSeed() string { return strconv.FormatInt(int64(rand.IntN(1<<31)), 36) }
