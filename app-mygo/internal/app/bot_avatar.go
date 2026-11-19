package app

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// bot_avatar.dart: the resident bot's mascot, built like a desk companion
// robot: a soft matte shell, a recessed dark face screen and two glowing
// eyes that carry every mood. Each body wears one rounded topper that lags
// behind the body on a spring.
// Source: fonzu/RobotEye (MIT) — https://github.com/fonzu/RobotEye
// Derived: an expression is a set of eye parameters (rounded rectangle, a
// top lid cut with a slope, a bottom ellipse mask) eased between moods; a
// blink hides each expression change; idle glances that return, squints and
// drift.
// Source: FluxGarage RoboEyes (GPL-3.0) — ideas only, no code: the outer eye
// grows when looking aside and tired lids for sleep.
// Source: bloub by Jérémy Perret (MIT) — https://github.com/jeremy-prt/bloub
// Derived: bodies as polar radius profiles that morph by interpolating
// radii, ease-out-quint morphs.

// BotMood is what the bot's face shows.
type BotMood int

const (
	MoodIdle BotMood = iota
	MoodListening
	MoodThinking
	MoodWorking
	MoodTalking
	MoodHappy
	MoodConfused
	MoodDreaming
	MoodNotify
)

// Label is kBotMoodLabels.
func (m BotMood) Label() string {
	return [...]string{l10n.BotMoodIdle, l10n.BotMoodListening, l10n.BotMoodThinking, l10n.BotMoodWorking,
		l10n.BotMoodTalking, l10n.BotMoodHappy, l10n.BotMoodConfused, l10n.BotMoodDreaming, l10n.BotMoodNotify}[m]
}

// BotShape is a body of bot.json's shape key, and the character it makes.
type BotShape struct {
	Key   string
	Shape widgets.OrganicShape
	Label string
}

// BotShapes are the bodies in picker order (agent/src/bot.mjs SHAPES).
var BotShapes = []BotShape{
	{"pebble", widgets.ShapePebble, l10n.BotAccessoryAntenna},
	{"mochi", widgets.ShapeMochi, l10n.BotAccessoryCatEars},
	{"bean", widgets.ShapeBean, l10n.BotAccessorySprout},
	{"circle", widgets.ShapeCircle, l10n.BotAccessoryBunnyEars},
	{"squircle", widgets.ShapeSquircle, l10n.BotAccessoryFeelers},
	{"capsule", widgets.ShapeCapsule, l10n.BotAccessoryHeadphones},
	{"droplet", widgets.ShapeDroplet, l10n.BotAccessoryDrop},
	{"ghost", widgets.ShapeGhost, l10n.BotAccessoryGhost},
}

// DefaultBotShape is the body a new bot wears: cat ears.
const DefaultBotShape = "mochi"

// BotColorKeys are bot.json's colours: theme follows the app, the rest are
// the seed palette in order.
var BotColorKeys = []string{"theme", "blue", "teal", "green", "amber", "red", "rose", "violet", "graphite"}

func botColorIndex(key string) int {
	for i, k := range BotColorKeys {
		if k == key {
			return i
		}
	}
	return -1
}

// BotColorLabel names a colour key.
func BotColorLabel(key string) string {
	i := botColorIndex(key)
	if i <= 0 {
		return l10n.CommonFollowTheme
	}
	return design.SeedPalette[i-1].Label
}

func botScheme(key string) *design.Scheme {
	i := botColorIndex(key)
	if i <= 0 {
		return design.S()
	}
	return design.FromSeed(design.ARGB(design.SeedPalette[i-1].Seed), design.TonalSpot, design.T().Dark, 0)
}

// botPal is the bot's colours, from M3 fixed roles so it looks the same in
// light and dark themes.
type botPal struct{ body, ink, eye, blush, trim, leaf ui.Color }

func botPalette(key string) botPal {
	s := botScheme(key)
	white := ui.RGB(255, 255, 255)
	return botPal{
		body:  s.PrimaryFixedDim,
		ink:   s.OnPrimaryFixed,
		eye:   design.Lerp(s.PrimaryFixed, white, .4),
		blush: s.TertiaryFixedDim,
		trim:  design.Lerp(s.PrimaryFixedDim, s.OnPrimaryFixed, .32),
		leaf:  design.Lerp(design.ARGB(0xFF86B872), s.PrimaryFixedDim, .2),
	}
}

// BotColors is the bot's body and accent colour (chips, swatches).
func BotColors(key string) (ui.Color, ui.Color) {
	s := botScheme(key)
	return s.PrimaryFixedDim, s.Tertiary
}

func shapeOf(key string) widgets.OrganicShape {
	for _, s := range BotShapes {
		if s.Key == key {
			return s.Shape
		}
	}
	return widgets.ShapeMochi
}

// botFace is where the face screen sits on a body, in body radii.
type botFace struct{ y, w, h, top float64 }

var botFaces = map[string]botFace{
	"pebble":   {-.06, 1.22, .82, -1},
	"mochi":    {-.04, 1.4, .88, -.86},
	"bean":     {.04, 1.3, .7, -.65},
	"circle":   {-.02, 1.4, .94, -1},
	"squircle": {-.02, 1.46, .98, -1},
	"capsule":  {0, 1.4, .76, -.74},
	"droplet":  {.2, 1.1, .7, -1},
	"ghost":    {-.1, 1.3, .82, -1},
}

func faceOf(key string) botFace {
	if f, ok := botFaces[key]; ok {
		return f
	}
	return botFaces[DefaultBotShape]
}

func lerpFace(a, b botFace, k float64) botFace {
	return botFace{a.y + (b.y-a.y)*k, a.w + (b.w-a.w)*k, a.h + (b.h-a.h)*k, a.top + (b.top-a.top)*k}
}

// botPose is what the painter needs; eye sizes are in body radii, lids in
// fractions of the eye's height.
type botPose struct {
	eyeW, eyeH, open, lidT, lidS, lidB, asym float64
	gazeX, gazeY, glow, tip, blush, droop    float64
	sx, sy, tilt, lift, badge, dream, time   float64
	sway, swayV, bob, bobV, flick            float64
	morph                                    float64
	radii                                    []float64
	shape, from                              string
	fromRadii                                []float64
	morphAt                                  float64
	nextBlink, blinkAt, nextGaze             float64
	nextSquint, squintAt, nextFlick, flickAt float64
	flickSide                                float64
	gazeTX, gazeTY                           float64
	pointer                                  bool
	pointerX, pointerY                       float64
	moodAt, lastLift                         float64
	mood                                     BotMood
	started, last                            time.Time
	init                                     bool
}

func newPose(shape string, mood BotMood) botPose {
	p := botPose{eyeW: .24, eyeH: .32, open: 1, glow: 1, blush: .25, sx: 1, sy: 1, morph: 1,
		nextBlink: 1.5, blinkAt: -1, nextGaze: .8, nextSquint: 7, squintAt: -9, nextFlick: 3, flickAt: -1, flickSide: 1,
		morphAt: -1, shape: shape, from: shape, mood: mood, init: true}
	p.radii = widgets.ShapeRadii(shapeOf(shape))
	p.fromRadii = p.radii
	p.settle(mood, 0, true)
	return p
}

const botMorphSecs = .6

var quint = ui.CubicBezier(.22, 1, .36, 1)

// BotAvatar is the animated mascot. animate off draws a still pose; reduced
// motion keeps only blinking.
type BotAvatar struct {
	Size          float32
	Shape, Color  string
	Mood          BotMood
	Still         bool
	FollowPointer bool
}

// Show builds the avatar.
func (b BotAvatar) Show(c *ui.Context) ui.Element {
	if b.Size == 0 {
		b.Size = 40
	}
	if b.Shape == "" {
		b.Shape = DefaultBotShape
	}
	if b.Color == "" {
		b.Color = "theme"
	}
	e := ui.Box(c).Size(b.Size, b.Size).Shrink(0)
	p := ui.Local(e, "bot", func() botPose { return botPose{} })
	if !p.init {
		*p = newPose(b.Shape, b.Mood)
		p.started = time.Now()
		p.last = p.started
	}
	animate := !b.Still
	if p.shape != b.Shape {
		p.fromRadii = append([]float64{}, p.radii...)
		if p.morph >= .5 {
			p.from = p.shape
		}
		p.shape = b.Shape
		if !animate {
			p.radii, p.from, p.morph = widgets.ShapeRadii(shapeOf(b.Shape)), b.Shape, 1
		} else {
			p.morph, p.morphAt = 0, p.time
		}
	}
	if p.mood != b.Mood {
		p.mood, p.moodAt = b.Mood, p.time
		if animate {
			p.blinkAt = p.time
		}
	}
	if b.FollowPointer && animate {
		if x, y, ok := e.PointerPosition(); ok && e.Hovered() {
			r := float64(b.Size) / 2
			p.pointer = true
			p.pointerX = math.Max(-1, math.Min(1, (float64(x)-r)/(r*2.2)))
			p.pointerY = math.Max(-1, math.Min(1, (float64(y)-r)/(r*2.2)))
		} else {
			p.pointer = false
		}
	}
	pal := botPalette(b.Color)
	s := design.S()
	mood := b.Mood
	// The pose moves in the drawing, frame by frame, as the ticker moves it:
	// the view is built again only when something else changes.
	return e.Draw(func(pt *ui.Painter, r ui.Rect) {
		if animate {
			now := pt.Now()
			t := now.Sub(p.started).Seconds()
			dt := math.Min(math.Max(now.Sub(p.last).Seconds(), 0), .1)
			p.last, p.time = now, t
			still := design.T().ReduceMotion
			if p.morphAt >= 0 {
				k := 1.0
				if !still {
					k = float64(quint(float32(math.Min(math.Max((t-p.morphAt)/botMorphSecs, 0), 1))))
				}
				to := widgets.ShapeRadii(shapeOf(p.shape))
				out := make([]float64, len(to))
				for i := range to {
					out[i] = p.fromRadii[i] + (to[i]-p.fromRadii[i])*k
				}
				p.radii, p.morph = out, k
				if k >= 1 {
					p.morphAt, p.from = -1, p.shape
				}
			}
			p.settle(mood, dt, still)
			// Every frame, as the ticker runs: the springs of its toppers
			// and its gaze after the pointer stutter at a lower rate.
			pt.AnimationFrame()
		} else {
			p.settle(mood, 0, true)
		}
		paintBot(pt, r, p, pal, s.Tertiary, s.Shadow)
	})
}

// settle moves the pose toward the mood's target; dt 0 with still jumps
// there.
func (p *botPose) settle(mood BotMood, dt float64, still bool) {
	t := p.time
	since := t - p.moodAt
	eyeW, eyeH, lidT, lidS, lidB, asym, gx, gy := .24, .32, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0
	glow, tip, blush, droop := 1.0, 0.0, .25, 0.0
	sx, sy, tilt, lift, badge, dream := 1.0, 1.0, 0.0, 0.0, 0.0, 0.0
	if !still && t >= p.nextGaze {
		p.gazeTX, p.gazeTY = rand.Float64()*1.2-.6, rand.Float64()*.7-.35
		if rand.Float64() < .35 {
			p.gazeTX, p.gazeTY = 0, 0
		}
		p.nextGaze = t + 1.4 + rand.Float64()*2.6
	}
	if !still && t >= p.nextSquint {
		p.squintAt = t
		p.nextSquint = t + 9 + rand.Float64()*9
	}
	squint := !still && t-p.squintAt < 1.1
	lookX, lookY := p.gazeTX, p.gazeTY
	if p.pointer {
		lookX, lookY = p.pointerX, p.pointerY
	}
	breath := 0.0
	if !still {
		breath = math.Sin(2 * math.Pi * t / 4.2)
	}
	hop := func(period float64) float64 {
		if still || math.Mod(t, period) >= .5 {
			return 0
		}
		return math.Sin(math.Pi * math.Mod(t, period) / .5)
	}
	switch mood {
	case MoodIdle:
		gx, gy = lookX, lookY
		tilt = lookX * .03
		sy = 1 + .018*breath
		sx = 1 - .008*breath
		if squint {
			lidT, lidB, blush = .26, .22, .4
		}
	case MoodListening:
		eyeW, eyeH, blush = .26, .37, .4
		gx, gy = 0, .3
		if p.pointer {
			gx, gy = p.pointerX, p.pointerY
		}
		if !still {
			tilt = .07
		}
		tip, droop = .8, -.25
		sy = 1 + .012*breath
	case MoodThinking:
		eyeW, eyeH, lidT, lidB = .22, .28, .22, .12
		gx, gy = .6, -.65
		tilt, tip = .04, 1
		if !still {
			tilt = .05 * math.Sin(t*1.6)
			tip = .55 + .45*math.Sin(t*5)
		}
		sy = 1 + .015*breath
	case MoodWorking:
		eyeH, lidT, lidS = .26, .3, .35
		gy, tip = .15, 1
		if !still {
			gx = math.Sin(t*3.4) * .7
			tip = .75 + .25*math.Sin(t*13)
			sy = 1 + .012*math.Sin(t*9)
		}
	case MoodTalking:
		beat := 0.0
		if !still {
			beat = math.Abs(math.Sin(2 * math.Pi * t * 3.1))
		}
		gx = lookX * .4
		gy = lookY*.4 - .15*beat
		lidB = .2 * beat
		sy = 1 + .035*beat
		sx = 1 - .015*beat
		blush, tip = .45, beat
	case MoodHappy:
		eyeH, lidB, blush, droop = .34, .62, .85, -.2
		if !still && since <= 1.4 {
			lift = -.1 * math.Abs(math.Sin(math.Pi*math.Mod(since*2.2, 1)))
		}
		sy = 1 + .018*breath
		if lift < 0 {
			sy = 1.03
		}
	case MoodConfused:
		asym, lidT, lidS, tilt = .42, .24, -.5, -.13
		gx = -.25
		if !still {
			gx = -.25 + .12*math.Sin(t*7)
		}
		gy, glow, droop = -.2, .85, .35
	case MoodDreaming:
		eyeW, lidT, lidS, gy, glow = .26, .8, -.3, .35, .45
		slow := 0.0
		if !still {
			slow = math.Sin(2 * math.Pi * t / 6)
		}
		sy = 1 + .04*slow
		sx = 1 - .015*slow
		dream, blush, droop = 1, .3, 1
	case MoodNotify:
		eyeW, eyeH, badge, tip = .26, .38, 1, 1
		h := hop(2.6)
		lift = -.08 * h
		sy = 1 + .03*h
	}
	// The ghost floats.
	if p.shape == "ghost" {
		if still {
			lift -= .05
		} else {
			lift += -.05 + .03*math.Sin(2*math.Pi*t/2.8)
		}
	}
	// Blinks every 3–6 s and on every mood change; close fast, open slower.
	closeAmt := 0.0
	if dt > 0 || !still {
		if t >= p.nextBlink {
			p.blinkAt = t
			p.nextBlink = t + 2.8 + rand.Float64()*3.4
			if rand.Float64() < .18 {
				p.nextBlink = t + .34
			}
		}
		u := t - p.blinkAt
		if p.blinkAt >= 0 && u < .2 {
			if u < .08 {
				closeAmt = math.Sin(math.Pi / 2 * u / .08)
			} else {
				closeAmt = math.Cos(math.Pi / 2 * (u - .08) / .12)
			}
		}
	}
	if !still && t >= p.nextFlick {
		p.flickAt = t
		p.flickSide = 1
		if rand.IntN(2) == 0 {
			p.flickSide = -1
		}
		p.nextFlick = t + 4 + rand.Float64()*6
	}
	fu := t - p.flickAt
	p.flick = 0
	if !still && p.flickAt >= 0 && fu < .3 {
		p.flick = p.flickSide * math.Sin(math.Pi*fu/.3)
	}
	k := 1.0
	if dt != 0 {
		rate := 11.0
		if still {
			rate = 30
		}
		k = 1 - math.Exp(-dt*rate)
	}
	to := func(cur *float64, target float64) { *cur += (target - *cur) * k }
	to(&p.eyeW, eyeW)
	to(&p.eyeH, eyeH)
	to(&p.lidT, lidT)
	to(&p.lidS, lidS)
	to(&p.lidB, lidB)
	to(&p.asym, asym)
	to(&p.gazeX, gx)
	to(&p.gazeY, gy)
	to(&p.glow, glow)
	to(&p.tip, tip)
	to(&p.blush, blush)
	to(&p.droop, droop)
	to(&p.sx, sx)
	to(&p.sy, sy)
	to(&p.tilt, tilt)
	to(&p.lift, lift)
	to(&p.badge, badge)
	to(&p.dream, dream)
	p.open = 1 - .9*closeAmt
	// Toppers lag behind the head: a spring toward the lean, a bob against lift.
	swayTo := -p.tilt*1.6 - p.gazeX*.1
	if !still {
		swayTo += .035 * math.Sin(t*1.3)
	}
	if dt == 0 || still {
		p.sway, p.swayV, p.bob, p.bobV = swayTo, 0, 0, 0
	} else {
		lv := (p.lift - p.lastLift) / dt
		p.swayV += (-80*(p.sway-swayTo) - 7*p.swayV) * dt
		p.sway += p.swayV * dt
		p.bobV += (-110*(p.bob-math.Max(-.15, math.Min(.15, lv*.12))) - 9*p.bobV) * dt
		p.bob = math.Max(-.2, math.Min(.2, p.bob+p.bobV*dt))
	}
	p.lastLift = p.lift
}

// xf is a 2D affine transform: x' = a·x + c·y + e, y' = b·x + d·y + f.
type xf struct{ a, b, c, d, e, f float64 }

var identity = xf{1, 0, 0, 1, 0, 0}

func (m xf) mul(n xf) xf {
	return xf{m.a*n.a + m.c*n.b, m.b*n.a + m.d*n.b, m.a*n.c + m.c*n.d, m.b*n.c + m.d*n.d,
		m.a*n.e + m.c*n.f + m.e, m.b*n.e + m.d*n.f + m.f}
}
func (m xf) translate(x, y float64) xf { return m.mul(xf{1, 0, 0, 1, x, y}) }
func (m xf) scale(x, y float64) xf     { return m.mul(xf{x, 0, 0, y, 0, 0}) }
func (m xf) rotate(a float64) xf {
	s, c := math.Sin(a), math.Cos(a)
	return m.mul(xf{c, s, -s, c, 0, 0})
}
func (m xf) pt(x, y float64) (float32, float32) {
	return float32(m.a*x + m.c*y + m.e), float32(m.b*x + m.d*y + m.f)
}
func (m xf) scaleFactor() float64 { return math.Sqrt(math.Abs(m.a*m.d - m.b*m.c)) }

// tpath builds a path through a transform.
type tpath struct {
	m xf
	p ui.Path
}

func (t *tpath) move(x, y float64) { t.p.MoveTo(t.m.pt(x, y)) }
func (t *tpath) line(x, y float64) { t.p.LineTo(t.m.pt(x, y)) }
func (t *tpath) quad(cx, cy, x, y float64) {
	ax, ay := t.m.pt(cx, cy)
	bx, by := t.m.pt(x, y)
	t.p.QuadTo(ax, ay, bx, by)
}
func (t *tpath) cube(ax, ay, bx, by, x, y float64) {
	a1, a2 := t.m.pt(ax, ay)
	b1, b2 := t.m.pt(bx, by)
	c1, c2 := t.m.pt(x, y)
	t.p.CubeTo(a1, a2, b1, b2, c1, c2)
}

const kappa = .5522847498

// oval adds an ellipse centred at cx, cy.
func (t *tpath) oval(cx, cy, w, h float64) {
	rx, ry := w/2, h/2
	kx, ky := rx*kappa, ry*kappa
	t.move(cx+rx, cy)
	t.cube(cx+rx, cy+ky, cx+kx, cy+ry, cx, cy+ry)
	t.cube(cx-kx, cy+ry, cx-rx, cy+ky, cx-rx, cy)
	t.cube(cx-rx, cy-ky, cx-kx, cy-ry, cx, cy-ry)
	t.cube(cx+kx, cy-ry, cx+rx, cy-ky, cx+rx, cy)
	t.p.Close()
}

// rrect adds a rounded rectangle from x, y.
func (t *tpath) rrect(x, y, w, h, r float64) {
	r = math.Min(r, math.Min(w, h)/2)
	k := r * (1 - kappa)
	t.move(x+r, y)
	t.line(x+w-r, y)
	t.cube(x+w-k, y, x+w, y+k, x+w, y+r)
	t.line(x+w, y+h-r)
	t.cube(x+w, y+h-k, x+w-k, y+h, x+w-r, y+h)
	t.line(x+r, y+h)
	t.cube(x+k, y+h, x, y+h-k, x, y+h-r)
	t.line(x, y+r)
	t.cube(x, y+k, x+k, y, x+r, y)
	t.p.Close()
}

func (t *tpath) circle(cx, cy, r float64) { t.oval(cx, cy, 2*r, 2*r) }

func newPath(m xf) *tpath { return &tpath{m: m} }

// blob paints a blurred shape of r, as a mask-blurred oval: the shadow of
// a box moved out of the way, which MyGo paints only outside the box.
func blob(p *ui.Painter, m xf, cx, cy, w, h, sigma float64, c ui.Color) {
	x, y := m.pt(cx, cy)
	s := m.scaleFactor()
	W, H := float32(w*s), float32(h*s)
	blur := float32(sigma*s) * 2
	shift := W + 3*blur + 8
	r := ui.Rect{X: x - W/2 - shift, Y: y - H/2, W: W, H: H}
	p.Shadow(r, min(W, H)/2, shift, 0, blur, 0, c)
}

func white(a float32) ui.Color { return ui.RGBA(255, 255, 255, a) }

var black = ui.RGB(0, 0, 0)
var whiteC = ui.RGB(255, 255, 255)

func paintBot(pt *ui.Painter, box ui.Rect, p *botPose, pal botPal, accent, shadow ui.Color) {
	size := math.Min(float64(box.W), float64(box.H))
	r := size / 2 * .7
	cx, cy := float64(box.X+box.W/2), float64(box.Y+box.H/2)+r*.12
	fine := size >= 44
	f := lerpFace(faceOf(p.from), faceOf(p.shape), p.morph)
	bodyPath := widgets.ShapePath(p.radii, ui.Rect{X: float32(-r), Y: float32(-r), W: float32(2 * r), H: float32(2 * r)}, 0)
	b := bodyPath.Bounds()
	bBottom := float64(b.Y + b.H)

	// Contact shadow: tighter and darker when down, fainter as it lifts.
	up := math.Min(math.Max(-p.lift, 0), .2) / .2
	blob(pt, identity, cx, cy+bBottom+r*.05, float64(b.W)*.66*(1-.25*up)*p.sx, r*.15*(1-.3*up), math.Max(1, r*.06),
		design.WithOpacity(shadow, float32(.16*(1-.5*up))))

	m := identity.translate(cx, cy+p.lift*r).rotate(p.tilt).translate(0, bBottom).scale(p.sx, p.sy).translate(0, -bBottom)
	// The body path through the transform: ShapePath's points, moved.
	body := newPath(m)
	n := len(p.radii)
	{
		xs := make([]float64, n)
		ys := make([]float64, n)
		// Recompute the samples ShapePath fitted, to transform them.
		minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for i := 0; i < n; i++ {
			a := -math.Pi/2 + 2*math.Pi*float64(i)/float64(n)
			xs[i], ys[i] = math.Cos(a)*p.radii[i], math.Sin(a)*p.radii[i]
			minX, maxX = math.Min(minX, xs[i]), math.Max(maxX, xs[i])
			minY, maxY = math.Min(minY, ys[i]), math.Max(maxY, ys[i])
		}
		s := math.Min(2*r/(maxX-minX), 2*r/(maxY-minY))
		ccx, ccy := (minX+maxX)/2, (minY+maxY)/2
		for i := range xs {
			xs[i], ys[i] = (xs[i]-ccx)*s, (ys[i]-ccy)*s
		}
		body.move(xs[0], ys[0])
		for i := 0; i < n; i++ {
			p0, p1, p2, p3 := (i-1+n)%n, i, (i+1)%n, (i+2)%n
			body.cube(xs[p1]+(xs[p2]-xs[p0])/6, ys[p1]+(ys[p2]-ys[p0])/6,
				xs[p2]-(xs[p3]-xs[p1])/6, ys[p2]-(ys[p3]-ys[p1])/6, xs[p2], ys[p2])
		}
		body.p.Close()
	}
	bp := &body.p
	bb := bp.Bounds()

	toppers(pt, m, p, pal, r, f, false)

	// Matte shell: a soft light from the upper left, depth toward the base.
	pt.ClipPath(bp, func() {
		pt.FillRadialGradient(bb, ui.RadialGradient{
			Center: ui.Point{X: .5 - .35/2, Y: .5 - .6/2}, Radius: float32(1.2 * float64(max(bb.W, bb.H)) / 2),
			Stops: []ui.GradientStop{{Color: design.Lerp(pal.body, whiteC, .26), At: 0}, {Color: pal.body, At: .5}, {Color: design.Lerp(pal.body, black, .1), At: 1}},
		}, 0)
		if fine {
			hy := (f.top + f.y - f.h/2) / 2 * r
			blob(pt, m, -.3*r, hy, r*.5, r*.22, r*.07, white(.2))
		}
	})

	// The face screen turns a touch toward the gaze, set into a bezel shaded
	// from above.
	vcx, vcy := p.gazeX*.03*r, f.y*r
	vw, vh := f.w*r, f.h*r
	rad := f.h * r * .42
	bz := newPath(m)
	bz.rrect(vcx-vw/2-r*.05, vcy-vh/2-r*.05, vw+r*.1, vh+r*.1, rad+r*.05)
	bzb := bz.p.Bounds()
	pt.FillPathGradient(&bz.p, ui.LinearGradient{From: design.Lerp(pal.body, black, .22), To: design.Lerp(pal.body, whiteC, .18), Angle: 180 + float32(p.tilt*180/math.Pi)})
	_ = bzb
	visor := newPath(m)
	visor.rrect(vcx-vw/2, vcy-vh/2, vw, vh, rad)
	pt.FillPathGradient(&visor.p, ui.LinearGradient{From: design.Lerp(pal.ink, whiteC, .08), To: pal.ink, Angle: 180 + float32(p.tilt*180/math.Pi)})
	pt.ClipPath(&visor.p, func() {
		botFaceEyes(pt, m, p, pal, vcx, vcy, f, r)
		if fine {
			g := newPath(m)
			g.oval(vcx-.28*vw, vcy-.52*vh, vw*.9, vh*.5)
			pt.FillPath(&g.p, white(.06))
		}
	})
	if fine {
		pt.StrokePathGradient(&visor.p, float32(math.Max(.6, r*.014)*m.scaleFactor()), ui.LinearGradient{
			Angle: 180, Stops: []ui.GradientStop{{Color: white(.18), At: 0}, {Color: white(0), At: .5}, {Color: white(0), At: 1}},
		})
	}

	toppers(pt, m, p, pal, r, f, true)

	if p.badge > .02 {
		// A notification light on the screen's corner.
		ax, ay := vcx+vw/2-r*.04, vcy-vh/2+r*.04
		pulse := 1 + .08*math.Sin(p.time*4)
		rr := r * .15 * p.badge * pulse
		c1 := newPath(m)
		c1.circle(ax, ay, rr+math.Max(1.5, r*.055))
		pt.FillPath(&c1.p, pal.body)
		c2 := newPath(m)
		c2.circle(ax, ay, rr)
		pt.FillPath(&c2.p, accent)
	}

	if p.dream > .02 {
		// Three small dots drift up and away from the sleeping head.
		for i := 0; i < 3; i++ {
			ph := math.Mod(p.time/2.8+float64(i)/3, 1)
			d := newPath(identity)
			d.circle(cx+r*(.62+ph*.45), cy+r*(f.top*.7-ph*.6), r*(.045+.04*ph))
			pt.FillPath(&d.p, design.WithOpacity(pal.trim, float32(p.dream*(1-ph)*.7)))
		}
	}
}

// botFaceEyes draws the blush and the eyes on the screen: each eye a rounded
// rectangle cut by a sloped top lid and a bottom ellipse, with a soft bloom.
func botFaceEyes(pt *ui.Painter, m xf, p *botPose, pal botPal, vcx, vcy float64, f botFace, r float64) {
	spread := f.w / 1.32
	dx := .25 * spread * r
	y := vcy + p.gazeY*.08*r
	if p.blush > .02 {
		col := design.WithOpacity(pal.blush, float32(.6*math.Min(math.Max(p.blush, 0), 1)))
		for _, side := range []float64{-1, 1} {
			blob(pt, m, vcx+side*(dx+.07*r), y+r*.21, r*.2, r*.08, math.Max(.8, r*.04), col)
		}
	}
	fill := design.Lerp(pal.ink, pal.eye, float32(.35+.65*p.glow))
	for _, side := range []float64{-1, 1} {
		// Looking aside, the outer eye grows (curiosity).
		cur := 1 + .22*math.Max(0, side*p.gazeX) - .06*math.Max(0, -side*p.gazeX)
		w := p.eyeW * r * (1 + side*p.asym*.12)
		h := math.Max(.035*r, p.eyeH*r*cur*(1+side*p.asym*.4)*p.open)
		ex := vcx + side*dx + p.gazeX*.13*spread*r
		ey := y - side*p.asym*.04*r
		top, bottom := ey-h/2, ey+h/2
		eye := newPath(m)
		eye.rrect(ex-w/2, top, w, h, math.Min(w, h)*.45)
		draw := func() {
			if p.glow > .05 {
				blob(pt, m, ex, ey, w, h, math.Max(.8, r*.07), design.WithOpacity(pal.eye, float32(.5*p.glow)))
			}
			pt.FillPath(&eye.p, fill)
		}
		// The bottom ellipse (the smile) cuts the eye: clip to what is
		// outside it, by the even-odd rule.
		if p.lidB > .01 {
			inner := draw
			smileTop := bottom - h*p.lidB*.8
			cut := newPath(m)
			cut.rrect(ex-w*2, top-h*2, w*4, h*5, 0)
			cut.oval(ex, smileTop+h*.75, w*1.8, h*1.5)
			cut.p.FillRule(ui.EvenOdd)
			draw = func() { pt.ClipPath(&cut.p, inner) }
		}
		if p.lidT > .01 {
			// Positive slope lowers the inner end (determined), negative the
			// outer (sleepy, worried): keep what is below the lid's curve.
			inner := draw
			lid := top + h*p.lidT
			in, out := ex-side*w, ex+side*w
			yi, yo := lid+p.lidS*h*.4, lid-p.lidS*h*.4
			keep := newPath(m)
			keep.move(in, yi)
			keep.quad(ex, (yi+yo)/2+h*.1, out, yo)
			keep.line(out, bottom+h*2)
			keep.line(in, bottom+h*2)
			keep.p.Close()
			draw = func() { pt.ClipPath(&keep.p, inner) }
		}
		draw()
	}
}

func toppers(pt *ui.Painter, m xf, p *botPose, pal botPal, r float64, f botFace, front bool) {
	if p.from != p.shape && p.morph < 1 {
		topper(pt, m, p, pal, p.from, 1-p.morph, r, f, front)
	}
	k := p.morph
	if p.from == p.shape {
		k = 1
	}
	topper(pt, m, p, pal, p.shape, k, r, f, front)
}

// topper is one body's topper, scaled by k about where it meets the head.
func topper(pt *ui.Painter, m xf, p *botPose, pal botPal, shape string, k, r float64, f botFace, front bool) {
	if k < .02 {
		return
	}
	top := f.top * r
	ay := top
	if shape == "capsule" {
		ay = 0
	}
	m = m.translate(0, ay).scale(k, k).translate(0, -ay)
	switch {
	case shape == "pebble" && !front:
		antenna(pt, m, p, pal, 0, top+.12*r, 0, .4*r, .1*r)
	case shape == "squircle" && !front:
		for _, side := range []float64{-1, 1} {
			antenna(pt, m, p, pal, side*.3*r, top+.12*r, side*.32, .36*r, .075*r)
		}
	case shape == "mochi" && !front:
		for _, side := range []float64{-1, 1} {
			ear(pt, m, p, pal, side, side*.5*r, top+.16*r, r, true)
		}
	case shape == "circle" && !front:
		for _, side := range []float64{-1, 1} {
			ear(pt, m, p, pal, side, side*.3*r, top+.12*r, r, false)
		}
	case shape == "bean" && !front:
		sprout(pt, m, p, pal, .02*r, top+.1*r, r)
	case shape == "capsule" && !front:
		// Headphone band over the head.
		band := newPath(m)
		cyy, rx, ry := -.04*r, 1.03*r, .87*r
		start, sweep := math.Pi+.28, math.Pi-.56
		const steps = 24
		for i := 0; i <= steps; i++ {
			a := start + sweep*float64(i)/steps
			x, y := math.Cos(a)*rx, cyy+math.Sin(a)*ry
			if i == 0 {
				band.move(x, y)
			} else {
				band.line(x, y)
			}
		}
		band.p.Cap(ui.RoundCap).Join(ui.RoundJoin)
		pt.StrokePath(&band.p, float32(math.Max(1.2, r*.09)*m.scaleFactor()), pal.trim)
	case shape == "capsule" && front:
		for _, side := range []float64{-1, 1} {
			cx, cy, w, h := side*.99*r, .02*r, .22*r, .52*r
			cup := newPath(m)
			cup.rrect(cx-w/2, cy-h/2, w, h, .11*r)
			pt.FillPathGradient(&cup.p, ui.LinearGradient{From: design.Lerp(pal.trim, whiteC, .2), To: pal.trim, Angle: 135})
			light(pt, m, p, pal, cx, cy, .035*r, design.Lerp(pal.trim, whiteC, .35))
		}
	}
}

// light is a small light that turns to the eye colour and glows while the
// bot works.
func light(pt *ui.Painter, m xf, p *botPose, pal botPal, x, y, rad float64, rest ui.Color) {
	tip := math.Min(math.Max(p.tip, 0), 1)
	lit := design.Lerp(rest, pal.eye, float32(tip))
	if p.tip > .02 {
		blob(pt, m, x, y, rad*3.2, rad*3.2, math.Max(.8, rad*.8), design.WithOpacity(pal.eye, float32(.5*tip)))
	}
	c := newPath(m)
	c.circle(x, y, rad)
	bb := c.p.Bounds()
	pt.ClipPath(&c.p, func() {
		pt.FillRadialGradient(bb, ui.RadialGradient{Center: ui.Point{X: .3, Y: .275}, Radius: bb.W / 2,
			Stops: []ui.GradientStop{{Color: design.Lerp(lit, whiteC, .45), At: 0}, {Color: lit, At: 1}}}, 0)
	})
}

// antenna is a stem from base leaning angle plus the sway, ending in a ball.
func antenna(pt *ui.Painter, m xf, p *botPose, pal botPal, bx, by, angle, length, ball float64) {
	a := angle + p.sway
	tx, ty := bx+math.Sin(a)*length*(1+p.bob), by-math.Cos(a)*length*(1+p.bob)
	cx, cy := bx+math.Sin(angle)*length*.55, by-math.Cos(angle)*length*.55
	stem := newPath(m)
	stem.move(bx, by)
	stem.quad(cx, cy, tx, ty)
	stem.p.Cap(ui.RoundCap)
	pt.StrokePath(&stem.p, float32(math.Max(1.2, ball*.65)*m.scaleFactor()), pal.trim)
	light(pt, m, p, pal, tx, ty, ball, pal.blush)
}

// ear is a cat ear (rounded triangle) or a bunny ear (long oval), behind
// the head; ears perk with interest, droop when sleepy and flick now and then.
func ear(pt *ui.Painter, m xf, p *botPose, pal botPal, side, bx, by, r float64, cat bool) {
	droop := p.droop
	if side > 0 {
		droop += p.asym * 1.2
	}
	flick := math.Max(0, side*p.flick)
	base, dr, sw := .14, .9, .5
	if cat {
		base, dr, sw = .26, .35, .35
	}
	rot := side*(base+.22*flick+droop*dr) + p.sway*sw
	em := m.translate(bx, by).rotate(rot)
	outer := design.Lerp(pal.body, black, .04)
	inner := design.WithOpacity(pal.blush, .85)
	if cat {
		o := newPath(em)
		roundedPoly(o, [][2]float64{{-.24 * r, .06 * r}, {0, -.46 * r}, {.24 * r, .06 * r}}, .12*r)
		pt.FillPath(&o.p, outer)
		i := newPath(em)
		roundedPoly(i, [][2]float64{{-.12 * r, .02 * r}, {0, -.3 * r}, {.12 * r, .02 * r}}, .06*r)
		pt.FillPath(&i.p, inner)
		return
	}
	o := newPath(em)
	o.oval(0, -.27*r, .26*r, .64*r)
	pt.FillPath(&o.p, outer)
	i := newPath(em)
	i.oval(0, -.3*r, .12*r, .42*r)
	pt.FillPath(&i.p, inner)
}

// roundedPoly adds a closed polygon with every corner rounded by radius.
func roundedPoly(t *tpath, pts [][2]float64, radius float64) {
	n := len(pts)
	for i := 0; i < n; i++ {
		prev, cur, next := pts[(i-1+n)%n], pts[i], pts[(i+1)%n]
		d1x, d1y := prev[0]-cur[0], prev[1]-cur[1]
		d2x, d2y := next[0]-cur[0], next[1]-cur[1]
		l1, l2 := math.Hypot(d1x, d1y), math.Hypot(d2x, d2y)
		k1, k2 := math.Min(radius, l1/2)/l1, math.Min(radius, l2/2)/l2
		ax, ay := cur[0]+d1x*k1, cur[1]+d1y*k1
		bx, by := cur[0]+d2x*k2, cur[1]+d2y*k2
		if i == 0 {
			t.move(ax, ay)
		} else {
			t.line(ax, ay)
		}
		t.quad(cur[0], cur[1], bx, by)
	}
	t.p.Close()
}

// sprout is a seedling: a short stem and two leaves that sway with the head.
func sprout(pt *ui.Painter, m xf, p *botPose, pal botPal, bx, by, r float64) {
	a := p.sway * 1.2
	tx, ty := bx+math.Sin(a)*.3*r*(1+p.bob), by-math.Cos(a)*.3*r*(1+p.bob)
	stem := newPath(m)
	stem.move(bx, by)
	stem.quad(bx, by-.18*r, tx, ty)
	stem.p.Cap(ui.RoundCap)
	pt.StrokePath(&stem.p, float32(math.Max(1.2, r*.055)*m.scaleFactor()), design.Lerp(pal.leaf, black, .15))
	for _, l := range [][2]float64{{-1, .26}, {.9, .21}} {
		lm := m.translate(tx, ty).rotate(a + l[0])
		leaf := newPath(lm)
		leaf.oval(0, -l[1]/2*r, l[1]*.52*r, l[1]*r)
		pt.FillPathGradient(&leaf.p, ui.LinearGradient{From: design.Lerp(pal.leaf, whiteC, .2), To: pal.leaf, Angle: 90})
	}
}
