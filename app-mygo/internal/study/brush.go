package study

import "math"

// Brush is StudyBrush (study_brush.dart): stored with each new stroke;
// strokes without one replay as version-1 ink.
type Brush struct {
	Family                       string
	Opacity, Pressure, Smoothing float64
	SquareTip                    bool
}

// DefaultBrush is const StudyBrush().
var DefaultBrush = Brush{Family: "fountain", Opacity: 1, Pressure: .65, Smoothing: .2}

// RadiusScale is how much of the width a segment at raw pressure takes.
func (b Brush) RadiusScale(raw float64) float64 {
	if b.Family == "ballpoint" || b.Family == "highlighter" {
		return 1
	}
	exp := .85
	if b.Family == "brush" {
		exp = 1.6
	}
	shaped := math.Pow(clamp(raw, 0, 1), exp)
	return (1 - b.Pressure) + b.Pressure*(.15+.85*shaped)
}

func (b Brush) JSON() M {
	return M{"family": b.Family, "opacity": b.Opacity, "pressure": b.Pressure, "smoothing": b.Smoothing, "square_tip": b.SquareTip}
}

// BrushFrom is StudyBrush.fromJson.
func BrushFrom(m M) Brush {
	family := "fountain"
	switch f, _ := m["family"].(string); f {
	case "ballpoint", "fountain", "pencil", "brush", "highlighter":
		family = f
	}
	return Brush{
		Family:    family,
		Opacity:   clamp(num(m["opacity"], 1), .05, 1),
		Pressure:  clamp(num(m["pressure"], .65), 0, 1),
		Smoothing: clamp(num(m["smoothing"], .2), 0, 1),
		SquareTip: m["square_tip"] == true,
	}
}

// PenPreset is StudyPenPreset: a saved pen.
type PenPreset struct {
	Name        string
	Color       uint32
	Width       float64
	Brush       Brush
	ShapeAssist bool
}

func (p PenPreset) JSON() M {
	return M{"name": p.Name, "color": p.Color, "width": p.Width, "brush": p.Brush.JSON(), "shape_assist": p.ShapeAssist}
}

// PenPresetFrom is StudyPenPreset.fromJson.
func PenPresetFrom(m M) PenPreset {
	name := "收藏笔"
	if v, ok := m["name"]; ok && v != nil {
		name = str(v)
	}
	color := uint32(0xff25344a)
	if v, ok := m["color"].(float64); ok {
		color = uint32(int64(v))
	}
	return PenPreset{Name: name, Color: color, Width: clamp(num(m["width"], 3), .2, 80),
		Brush: BrushFrom(obj(m["brush"])), ShapeAssist: m["shape_assist"] != false}
}

// DefaultPresets are the five pens a new canvas offers.
func DefaultPresets() []PenPreset {
	b := DefaultBrush
	ball, pencil, soft, hl := b, b, b, b
	ball.Family, ball.Pressure = "ballpoint", 0
	pencil.Family, pencil.Opacity = "pencil", .72
	soft.Family, soft.Pressure, soft.Smoothing = "brush", .9, .35
	hl.Family, hl.Opacity, hl.Pressure, hl.SquareTip = "highlighter", .35, 0, true
	return []PenPreset{
		{"钢笔", 0xff26354a, 3, b, true},
		{"圆珠笔", 0xff6254d7, 2, ball, true},
		{"铅笔", 0xff4d5969, 4, pencil, true},
		{"软笔", 0xffd65360, 7, soft, true},
		{"荧光笔", 0xffffd34f, 22, hl, true},
	}
}
