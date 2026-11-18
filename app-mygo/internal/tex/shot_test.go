package tex

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShot(t *testing.T) {
	out := os.Getenv("TEX_SHOT")
	if out == "" {
		t.Skip()
	}
	formulas := []string{os.Getenv("TEX_ONE")}
	if formulas[0] == "" {
		formulas = []string{
			`\sum_{i=1}^n i^2 = \frac{n(n+1)(2n+1)}{6}`, `\int_0^\infty e^{-x^2}\,dx = \frac{\sqrt\pi}2 \quad \sqrt[3]{x^2+y}`,
			`\begin{aligned} dp &= \rho\left(f_x dx + f_y dy + f_z dz\right) \\ dW &= f_x dx + f_y dy \end{aligned}`,
			`f(x)=\begin{cases} 1 & x>0 \\ 0 & \text{其他} \end{cases} \qquad \begin{pmatrix} a & b \\ c & d \end{pmatrix}`,
			`\hat{x}\ \widehat{xyz}\ \overline{AB}\ \overrightarrow{AB}\ \underbrace{a+b+c}_{n}\ \vec v\ \tilde a \dot x`,
			`\mathbb{R}^n\ \mathcal{L}\ \mathbf{v}\ \boldsymbol\theta\ \lim_{x\to 0}\frac{\sin x}{x}\ \left\{ x \middle| x>0 \right\}`,
			`\xrightarrow[a]{f}\ \not\in\ \neq\ \color{red}{x}\ \boxed{E=mc^2}\ \big( \Big[ \bigg\{ \Bigg\langle\ x_{i}^{2}`,
			`\rho f_x - \frac{\partial p}{\partial x} = 0,\qquad \left[\frac{a}{b}\right]^2 \alpha\beta\gamma\Gamma\Delta`,
		}
	}
	size := float32(20)
	if os.Getenv("TEX_ONE") != "" {
		size = 48
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(ui.RGB(255, 255, 255)).Padding(12).Gap(14).Children(func() {
			for i, s := range formulas {
				disp := i%2 == 0
				f := Typeset(s, size, disp || len(formulas) == 1)
				ui.Box(c.Key(i)).Width(f.Width).Height(f.Height + f.Depth).Background(ui.RGB(245, 245, 230)).Draw(func(p *ui.Painter, r ui.Rect) {
					f.Draw(p, r.X, r.Y+f.Height, ui.RGB(20, 20, 20))
				})
			}
			ui.Text(c, "inline: x").FontSize(20)
		})
	}, 900, 900)
	w, _ := os.Create(out)
	defer w.Close()
	png.Encode(w, tt.Image())
}
