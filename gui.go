package main

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ---- 品牌色（深浅模式通用）----

var (
	accent = color.NRGBA{0x0a, 0x84, 0xff, 0xff} // 蓝
	warn   = color.NRGBA{0xff, 0x45, 0x3a, 0xff} // 橙红（倒计时 ≤3s）
)

// tcol 取当前主题/变体下的语义色（深浅模式自适应）。
func tcol(n fyne.ThemeColorName) color.Color {
	s := fyne.CurrentApp().Settings()
	return s.Theme().Color(n, s.ThemeVariant())
}

func withAlpha(c color.Color, a uint8) color.NRGBA {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), a}
}

// ---- 浏览器卡片（可点击 / 可悬停 / 可选中 / 自带浅投影）----

type cardPalette struct{ idleFill, idleStroke, hoverFill, selFill, selStroke color.Color }

func paletteFromTheme() cardPalette {
	fg := tcol(theme.ColorNameForeground)
	return cardPalette{
		idleFill:   tcol(theme.ColorNameInputBackground),
		idleStroke: withAlpha(fg, 0x12),
		hoverFill:  withAlpha(accent, 0x1f),
		selFill:    withAlpha(accent, 0x2b),
		selStroke:  accent,
	}
}

type card struct {
	widget.BaseWidget
	bg          *canvas.Rectangle
	obj         fyne.CanvasObject
	onTap       func()
	onSecondary func()
	pal         cardPalette
	hovered     bool
	sel         bool
}

func newCard(content fyne.CanvasObject, bg *canvas.Rectangle, pal cardPalette, onTap func()) *card {
	c := &card{bg: bg, obj: content, pal: pal, onTap: onTap}
	c.ExtendBaseWidget(c)
	c.refreshBG()
	return c
}
func (c *card) Tapped(*fyne.PointEvent) { c.onTap() }
func (c *card) TappedSecondary(*fyne.PointEvent) {
	if c.onSecondary != nil {
		c.onSecondary()
	}
}
func (c *card) MouseIn(*desktop.MouseEvent)         { c.hovered = true; c.refreshBG() }
func (c *card) MouseOut()                           { c.hovered = false; c.refreshBG() }
func (c *card) MouseMoved(*desktop.MouseEvent)      {}
func (c *card) Cursor() desktop.Cursor              { return desktop.PointerCursor }
func (c *card) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(c.obj) }
func (c *card) setSelected(s bool)                  { c.sel = s; c.refreshBG() }
func (c *card) refreshBG() {
	switch {
	case c.sel:
		c.bg.FillColor = c.pal.selFill
		c.bg.StrokeColor = c.pal.selStroke
		c.bg.StrokeWidth = 2
	case c.hovered:
		c.bg.FillColor = c.pal.hoverFill
		c.bg.StrokeColor = color.Transparent
		c.bg.StrokeWidth = 0
	default:
		c.bg.FillColor = c.pal.idleFill
		c.bg.StrokeColor = c.pal.idleStroke
		c.bg.StrokeWidth = 1
	}
	c.bg.Refresh()
}

// shadowed 在卡片底部叠一层轻微投影增加层次。
func shadowed(cardObj fyne.CanvasObject) fyne.CanvasObject {
	sh := canvas.NewRectangle(color.NRGBA{0, 0, 0, 0x1e})
	sh.CornerRadius = 14
	return container.New(&shadowLayout{}, sh, cardObj)
}

type shadowLayout struct{}

func (l *shadowLayout) MinSize(objs []fyne.CanvasObject) fyne.Size { return objs[1].MinSize() }
func (l *shadowLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	objs[0].Resize(fyne.NewSize(s.Width-2, s.Height))
	objs[0].Move(fyne.NewPos(1, 3))
	objs[1].Resize(s)
	objs[1].Move(fyne.NewPos(0, 0))
}

// ---- 细进度线（颜色随剩余时间变化）----

type progressLine struct {
	widget.BaseWidget
	track *canvas.Rectangle
	fill  *canvas.Rectangle
	frac  float64
}

func newProgressLine(trackCol color.Color) *progressLine {
	p := &progressLine{
		track: &canvas.Rectangle{FillColor: trackCol, CornerRadius: 2.5},
		fill:  &canvas.Rectangle{FillColor: accent, CornerRadius: 2.5},
		frac:  1,
	}
	p.ExtendBaseWidget(p)
	return p
}
func (p *progressLine) set(frac float64, c color.Color) {
	if frac < 0 {
		frac = 0
	}
	p.frac = frac
	p.fill.FillColor = c
	p.Refresh()
}
func (p *progressLine) CreateRenderer() fyne.WidgetRenderer { return &progressLineRenderer{p: p} }

type progressLineRenderer struct{ p *progressLine }

func (r *progressLineRenderer) Layout(s fyne.Size) {
	const h = 5
	y := (s.Height - h) / 2
	r.p.track.Resize(fyne.NewSize(s.Width, h))
	r.p.track.Move(fyne.NewPos(0, y))
	r.p.fill.Resize(fyne.NewSize(s.Width*float32(r.p.frac), h))
	r.p.fill.Move(fyne.NewPos(0, y))
}
func (r *progressLineRenderer) MinSize() fyne.Size { return fyne.NewSize(40, 5) }
func (r *progressLineRenderer) Refresh()           { r.Layout(r.p.Size()); canvas.Refresh(r.p) }
func (r *progressLineRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.p.track, r.p.fill}
}
func (r *progressLineRenderer) Destroy() {}

// ---- 图标 / 文本工具 ----

// browserIcon 返回某浏览器的图标对象（取不到则用字母头像兜底）。
func browserIcon(b Browser, size float32) fyne.CanvasObject {
	if p := browserIconPath(b); p != "" {
		img := canvas.NewImageFromFile(p)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(size, size))
		return img
	}
	return letterAvatar(b.Name, size)
}

func letterAvatar(name string, size float32) fyne.CanvasObject {
	bg := &canvas.Rectangle{FillColor: accent, CornerRadius: size / 2}
	bg.SetMinSize(fyne.NewSize(size, size))
	ch := "?"
	if name != "" {
		ch = strings.ToUpper(name[:1])
	}
	letter := canvas.NewText(ch, color.White)
	letter.TextSize = size * 0.46
	letter.Alignment = fyne.TextAlignCenter
	letter.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewStack(bg, container.NewCenter(letter))
}

func moreGlyph(fg color.Color, size float32) fyne.CanvasObject {
	bg := &canvas.Rectangle{FillColor: withAlpha(fg, 0x14), CornerRadius: 14}
	bg.SetMinSize(fyne.NewSize(size, size))
	dots := canvas.NewText("⋯", fg)
	dots.TextSize = size * 0.5
	dots.Alignment = fyne.TextAlignCenter
	dots.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewStack(bg, container.NewCenter(dots))
}

func boldText(s string, size float32, c color.Color) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}
func plainText(s string, size float32, c color.Color) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	return t
}
func hSpace(w float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(w, 0))
	return r
}
func vSpace(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, h))
	return r
}

// shortName 去掉常见厂商前缀，弹窗里更紧凑。
func shortName(n string) string {
	n = strings.TrimPrefix(n, "Google ")
	n = strings.TrimPrefix(n, "Microsoft ")
	return n
}

func hostOf(rawURL string) string {
	s := strings.TrimSpace(rawURL)
	if i := strings.Index(s, "://"); i != -1 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i != -1 {
		s = s[:i]
	}
	return strings.TrimPrefix(s, "www.")
}
