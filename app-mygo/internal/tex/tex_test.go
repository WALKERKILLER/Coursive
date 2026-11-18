package tex

import "testing"

func TestTypeset(t *testing.T) {
	for _, s := range []string{
		`ABC`, `S_x,\ S_y,\ S_z`, `S_x = S\cos\alpha,\quad S_y = S\cos\beta`, `V=\tfrac13 S_x\,dx`, `dx\to 0`,
		`\rho f_x - \frac{\partial p}{\partial x} = 0`, `\begin{aligned} dp &= \rho\left(f_x dx + f_y dy\right) \\ dW &= f_x dx \end{aligned}`,
		`\vec f = \dfrac{1}{\rho}\nabla p`, `\sum_{i=1}^n i^2 = \frac{n(n+1)(2n+1)}{6}`, `\int_0^\infty e^{-x^2}\,dx = \frac{\sqrt\pi}2`,
		`\sqrt[3]{x}`, `\begin{pmatrix} a & b \\ c & d \end{pmatrix}`, `f(x)=\begin{cases} 1 & x>0 \\ 0 & \text{其他} \end{cases}`,
		`\hat{x} \widehat{xyz} \overline{AB} \overrightarrow{AB} \underbrace{a+b}_{n}`, `\mathbb{R}^n \mathcal{L} \mathbf{v} \boldsymbol\theta`,
		`\lim_{x\to 0}\frac{\sin x}{x}`, `\left\{ x \middle| x>0 \right\}`, `\xrightarrow[a]{f} \not= \neq \color{red}{x} \boxed{E=mc^2}`,
		`a \over b`, `x^{2^{3}}`, `\big( \Big[ \bigg\{ \Bigg\langle`, `\operatorname{tr} A \pmod p`, "速度 v",
	} {
		f := Typeset(s, 16, true)
		if f.Width <= 0 || f.Height <= 0 {
			t.Errorf("%s: %+v", s, f)
		}
		if len(f.Unknown) > 0 {
			t.Errorf("%s: unknown %v", s, f.Unknown)
		}
	}
	if f := Typeset(`\nosuch x`, 16, false); len(f.Unknown) != 1 {
		t.Errorf("unknown: %v", f.Unknown)
	}
	// A fraction stands taller in display style.
	if d, i := Typeset(`\frac12`, 16, true), Typeset(`\frac12`, 16, false); d.Height <= i.Height {
		t.Errorf("display %v inline %v", d.Height, i.Height)
	}
}

func TestFontTables(t *testing.T) {
	f := font()
	if f.c.axisHeight <= 0 || f.c.fracRuleThickness <= 0 || f.c.scriptScale < .5 {
		t.Fatalf("constants: %+v", f.c)
	}
	g, _ := f.glyph('(')
	if c := f.vert[g]; c == nil || len(c.variants) < 3 || len(c.parts) == 0 {
		t.Errorf("( has no variants: %+v", c)
	}
	if len(f.italics) == 0 || len(f.accents) == 0 {
		t.Errorf("italics %d accents %d", len(f.italics), len(f.accents))
	}
}
