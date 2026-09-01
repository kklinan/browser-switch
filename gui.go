package main

import (
	"image/color"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 本文件是界面的设计系统层：间距、字号、圆角、颜色与基础控件。
// 目标是让整个应用看起来像 macOS 原生程序，而不是「用跨平台工具包画的界面」，
// 因此所有度量都锚定 Apple HIG：8pt 网格、11/12/13/15/20 的字号阶梯、
// 6/10/14 的圆角、系统语义色，以及跟随系统强调色。

// ---- 设计令牌 ----

// 间距（Apple HIG 8pt 网格）
const (
	sp2  = float32(2)
	sp4  = float32(4)
	sp8  = float32(8)
	sp12 = float32(12)
	sp16 = float32(16)
	sp20 = float32(20)
	sp24 = float32(24)
)

// 字号（macOS 排版阶梯）
const (
	fsCaption = float32(11) // 说明、提示、快捷键
	fsSubhead = float32(12) // 次级文字
	fsBody    = float32(13) // 正文
	fsTitle   = float32(15) // 分组 / 面板标题
	fsDisplay = float32(20) // 对话框主标题
)

// 圆角（macOS 控件 6 / 卡片 10 / 大磁贴 14）
const (
	radControl = float32(6)
	radCard    = float32(10)
	radTile    = float32(14)
)

// selectExtraChars 下拉框在「最长选项」之外额外预留的字符数。
// 不预留的话文字会顶到下拉箭头上，看起来像被截断。
const selectExtraChars = 5

// ---- Apple 系统色板 ----

// sysColor 保存一个 Apple 系统色在浅色 / 深色模式下的两个取值。
// Apple 的系统色（System Colors）深浅两套并不相同，直接用一套会在深色模式下
// 显得过亮或发灰，因此这里成对存放并在取用时按当前变体解析。
type sysColor struct{ light, dark color.NRGBA }

// get 按当前外观（浅色 / 深色）返回该颜色的实际取值。
func (c sysColor) get() color.Color {
	if isDark() {
		return c.dark
	}
	return c.light
}

// accentColor 色板中与系统强调色一致的颜色（由 syncAccentColor 同步）。
var (
	accentMu  sync.RWMutex
	sysAccent = sysColor{light: color.NRGBA{0x00, 0x7a, 0xff, 0xff}, dark: color.NRGBA{0x0a, 0x84, 0xff, 0xff}}
)

// Apple 系统色（浅色 / 深色）。
var (
	sysBlue  = sysColor{light: color.NRGBA{0x00, 0x7a, 0xff, 0xff}, dark: color.NRGBA{0x0a, 0x84, 0xff, 0xff}}
	sysGreen = sysColor{light: color.NRGBA{0x34, 0xc7, 0x59, 0xff}, dark: color.NRGBA{0x30, 0xd1, 0x58, 0xff}}
	sysRed   = sysColor{light: color.NRGBA{0xff, 0x3b, 0x30, 0xff}, dark: color.NRGBA{0xff, 0x45, 0x3a, 0xff}}
	sysGray  = sysColor{light: color.NRGBA{0x8e, 0x8e, 0x93, 0xff}, dark: color.NRGBA{0x8e, 0x8e, 0x93, 0xff}}

	// accentByName：macOS「强调色」设置的索引 → 对应系统色。
	accentByName = map[int]sysColor{
		-1: sysGray,
		0:  {light: color.NRGBA{0xff, 0x3b, 0x30, 0xff}, dark: color.NRGBA{0xff, 0x45, 0x3a, 0xff}}, // 红
		1:  {light: color.NRGBA{0xff, 0x95, 0x00, 0xff}, dark: color.NRGBA{0xff, 0x9f, 0x0a, 0xff}}, // 橙
		2:  {light: color.NRGBA{0xff, 0xcc, 0x00, 0xff}, dark: color.NRGBA{0xff, 0xd6, 0x0a, 0xff}}, // 黄
		3:  {light: color.NRGBA{0x34, 0xc7, 0x59, 0xff}, dark: color.NRGBA{0x30, 0xd1, 0x58, 0xff}}, // 绿
		4:  sysBlue,                                                                                 // 蓝
		5:  {light: color.NRGBA{0xaf, 0x52, 0xde, 0xff}, dark: color.NRGBA{0xbf, 0x5a, 0xf2, 0xff}}, // 紫
		6:  {light: color.NRGBA{0xff, 0x2d, 0x55, 0xff}, dark: color.NRGBA{0xff, 0x37, 0x5f, 0xff}}, // 粉
	}
)

// accent 返回当前强调色（跟随系统强调色设置，深浅模式各自取值）。
func accent() sysColor {
	accentMu.RLock()
	defer accentMu.RUnlock()
	return sysAccent
}

// syncAccentColor 读取 macOS「系统设置 › 外观 › 强调色」并同步强调色。
// 原生应用应当跟随系统强调色，而不是硬编码一个蓝。读取失败（未设置过强调色
// 时键不存在，此时系统就是蓝色）保持默认蓝，因此永远不会因读取问题改变观感。
func syncAccentColor() {
	out, err := exec.Command("defaults", "read", "-g", "AppleAccentColor").Output()
	if err != nil {
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return
	}
	c, ok := accentByName[n]
	if !ok {
		return
	}
	accentMu.Lock()
	sysAccent = c
	accentMu.Unlock()
}

// ---- 主题语义色 ----

// isDark 判断当前是否处于深色模式。
func isDark() bool { return themeVariant() == theme.VariantDark }

// themeVariant 返回当前外观变体；无 app（如单元测试）时按浅色处理。
func themeVariant() fyne.ThemeVariant {
	a := fyne.CurrentApp()
	if a == nil {
		return theme.VariantLight
	}
	return a.Settings().ThemeVariant()
}

// tcol 取当前主题 / 变体下的语义色（深浅模式自适应）。
func tcol(n fyne.ThemeColorName) color.Color {
	s := fyne.CurrentApp().Settings()
	return s.Theme().Color(n, s.ThemeVariant())
}

// fgCol 主文字色（对应 NSColor.labelColor）。
func fgCol() color.Color { return tcol(theme.ColorNameForeground) }

// secCol 次级文字色（对应 NSColor.secondaryLabelColor）。
func secCol() color.Color { return tcol(theme.ColorNamePlaceHolder) }

// withAlpha 保持 RGB 不变、替换透明度，用于从语义色派生填充色。
func withAlpha(c color.Color, a uint8) color.NRGBA {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), a}
}

// ---- 文本与间距工具 ----

// selectFixedSize 下拉选择器的固定尺寸：宽度覆盖所有选项（而非只有当前选中项），
// 再追加 extraChars 个字符的余量。
//
// 必须固定宽度：Select 的 MinSize 只按 PlaceHolder 与当前选中项计算（见 Fyne 的
// selectRenderer.MinSize），不设 PlaceHolder 时宽度就只等于当前文字宽度 ——
// 切换选项会把同一行的其他控件推来推去。macOS 的 NSPopUpButton 同样是固定宽度。
func selectFixedSize(sel *widget.Select, extraChars int) fyne.Size {
	th := fyne.CurrentApp().Settings().Theme()
	textSize := th.Size(theme.SizeNameText)
	measure := func(s string) float32 {
		return fyne.MeasureText(s, textSize, fyne.TextStyle{}).Width
	}

	// 覆盖最长的选项，选中它时也不会把行挤动
	widest := float32(0)
	for _, o := range sel.Options {
		widest = max(widest, measure(o))
	}
	// 控件的 MinSize 已含内边距与下拉箭头，这里只需补上「最宽选项 − 当前选项」的差值
	cur := measure(sel.Selected)
	w := sel.MinSize().Width + max(float32(0), widest-cur) + measure(strings.Repeat("0", extraChars))
	// 上限：极长的选项（如「使用 XXXX 打开」）不能把标签挤出可视区
	const selectMaxW = float32(340)
	return fyne.NewSize(min(w, selectMaxW), sel.MinSize().Height)
}

// fixedSelect 把下拉框钉在固定宽度上（理由见 selectFixedSize）。
func fixedSelect(sel *widget.Select, extraChars int) fyne.CanvasObject {
	return container.New(layout.NewGridWrapLayout(selectFixedSize(sel, extraChars)), sel)
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

// inset 在四周加等宽留白。container.NewPadded 固定只有 4px，
// 无法表达 macOS 窗口 20px 的内容边距，故自建一个。
func inset(o fyne.CanvasObject, pad float32) fyne.CanvasObject {
	return container.NewBorder(vSpace(pad), vSpace(pad), hSpace(pad), hSpace(pad), o)
}

// fixedSize 让内容以给定尺寸参与布局。Fyne 的部分布局（如 GridWrap）的
// MinSize 会随上一次 Layout 的结果变化，放进滚动容器时会得到一个过小的高度、
// 导致内容被裁掉。这里用一个透明的占位矩形把尺寸钉死。
func fixedSize(o fyne.CanvasObject, w, h float32) fyne.CanvasObject {
	sp := canvas.NewRectangle(color.Transparent)
	sp.SetMinSize(fyne.NewSize(w, h))
	return container.NewStack(sp, o)
}

// hairline 1px 分隔线（对应 macOS 的分隔线；颜色取自主题，深浅模式自适应）。
func hairline() fyne.CanvasObject {
	r := canvas.NewRectangle(tcol(theme.ColorNameSeparator))
	r.SetMinSize(fyne.NewSize(0, 1))
	return r
}

// vLine 竖直分隔线（侧边栏与内容区之间）。
func vLine() fyne.CanvasObject {
	r := canvas.NewRectangle(tcol(theme.ColorNameSeparator))
	r.SetMinSize(fyne.NewSize(1, 0))
	return r
}

// sectionHeader 分组标题：15pt 粗体标题 + 11pt 次级说明，
// 与 macOS「系统设置」里的分组标题一致。
func sectionHeader(title, subtitle string) fyne.CanvasObject {
	box := container.NewVBox(boldText(title, fsTitle, fgCol()))
	if subtitle != "" {
		box.Add(vSpace(sp2))
		box.Add(plainText(subtitle, fsCaption, secCol()))
	}
	return box
}

// truncateTail 按 rune 截断并加省略号。canvas.Text 不会自动截断，
// 而磁贴宽度固定，长浏览器名会换行撑破布局，故在渲染前手动收敛。
func truncateTail(s string, max int) string {
	if max < 2 {
		max = 2
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// ---- 卡片（磁贴 / 列表行）----

// cardPalette 卡片在 静止 / 悬停 / 选中 三态下的填充与描边色。
type cardPalette struct{ idleFill, idleStroke, hoverFill, selFill, selStroke color.Color }

// paletteFromTheme 从当前主题与强调色派生卡片配色；深浅模式自动适配。
func paletteFromTheme() cardPalette {
	fg := fgCol()
	acc := accent().get()
	return cardPalette{
		idleFill:   tcol(theme.ColorNameInputBackground),
		idleStroke: withAlpha(fg, 0x10),
		hoverFill:  withAlpha(acc, 0x14),
		selFill:    withAlpha(acc, 0x24),
		selStroke:  acc,
	}
}

// card 是可点击的容器：静止 / 悬停 / 选中三态，选中时描边强调色，
// 键盘聚焦时描边加粗（对应 macOS 的 focus ring）。
type card struct {
	widget.BaseWidget
	bg          *canvas.Rectangle
	obj         fyne.CanvasObject
	onTap       func()
	onSecondary func()
	pal         cardPalette
	onHover     func(bool) // 悬停变化回调（用于抑制键盘聚焦环）
	hovered     bool
	sel         bool
	focused     bool
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

func (c *card) MouseIn(*desktop.MouseEvent) {
	c.hovered = true
	c.refreshBG()
	if c.onHover != nil {
		c.onHover(true)
	}
}
func (c *card) MouseOut() {
	c.hovered = false
	c.refreshBG()
	if c.onHover != nil {
		c.onHover(false)
	}
}
func (c *card) MouseMoved(*desktop.MouseEvent) {}
func (c *card) Cursor() desktop.Cursor         { return desktop.PointerCursor }
func (c *card) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.obj)
}

func (c *card) setSelected(s bool) { c.sel = s; c.refreshBG() }
func (c *card) setFocused(f bool)  { c.focused = f; c.refreshBG() }

func (c *card) refreshBG() {
	switch {
	case c.sel:
		c.bg.FillColor = c.pal.selFill
		c.bg.StrokeColor = c.pal.selStroke
		c.bg.StrokeWidth = 2
		if c.focused {
			c.bg.StrokeWidth = 3 // 键盘聚焦：加粗描边，等同系统的 focus ring
		}
	case c.hovered:
		c.bg.FillColor = c.pal.hoverFill
		c.bg.StrokeColor = withAlpha(c.pal.selStroke, 0x40)
		c.bg.StrokeWidth = 1
	default:
		c.bg.FillColor = c.pal.idleFill
		c.bg.StrokeColor = c.pal.idleStroke
		c.bg.StrokeWidth = 1
	}
	c.bg.Refresh()
}

// tileShadow 在磁贴下方叠两层递减阴影，模拟 macOS 的柔和投影。
// 用两层而非一层硬边矩形，是为了避免 Fyne 无法做模糊时出现的「描边感」。
func tileShadow(radius float32, content fyne.CanvasObject) fyne.CanvasObject {
	outer := &canvas.Rectangle{FillColor: color.NRGBA{0, 0, 0, 0x10}, CornerRadius: radius}
	inner := &canvas.Rectangle{FillColor: color.NRGBA{0, 0, 0, 0x0e}, CornerRadius: radius}
	return container.New(&shadowLayout{}, outer, inner, content)
}

type shadowLayout struct{}

func (l *shadowLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return objs[len(objs)-1].MinSize().Add(fyne.NewSize(sp4, sp4))
}
func (l *shadowLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	content := objs[len(objs)-1]
	objs[0].Resize(fyne.NewSize(s.Width-sp2, s.Height-sp2))
	objs[0].Move(fyne.NewPos(1, sp2))
	objs[1].Resize(fyne.NewSize(s.Width-sp4, s.Height-sp2))
	objs[1].Move(fyne.NewPos(sp2, 1))
	content.Resize(fyne.NewSize(s.Width-sp4, s.Height-sp4))
	content.Move(fyne.NewPos(0, 0))
}

// ---- macOS 风格开关 ----

// appleSwitch 是 macOS 的开关控件（38×22 轨道 + 白色圆钮），替代 Fyne 的复选框。
// 系统设置里「记住此选择」这类布尔项用的都是开关而非勾选框。
type appleSwitch struct {
	widget.BaseWidget
	on      bool
	changed func(bool)
	track   *canvas.Rectangle
	knob    *canvas.Circle
}

// switchKnobPad 圆钮与轨道边缘的间距（Apple 的开关为 2pt）。
const switchKnobPad = float32(2)

// 开关尺寸：macOS 的开关固定 38×22，不随行高伸缩。
const (
	switchW = float32(38)
	switchH = float32(22)
)

func newAppleSwitch(on bool, changed func(bool)) *appleSwitch {
	s := &appleSwitch{
		on:      on,
		changed: changed,
		track:   &canvas.Rectangle{CornerRadius: 11},
		knob:    &canvas.Circle{FillColor: color.NRGBA{0xff, 0xff, 0xff, 0xff}},
	}
	s.ExtendBaseWidget(s)
	s.refresh()
	return s
}

func (s *appleSwitch) IsOn() bool { return s.on }

func (s *appleSwitch) toggle() {
	s.on = !s.on
	s.refresh()
	if s.changed != nil {
		s.changed(s.on)
	}
}

func (s *appleSwitch) Tapped(*fyne.PointEvent) { s.toggle() }
func (s *appleSwitch) Cursor() desktop.Cursor  { return desktop.PointerCursor }

func (s *appleSwitch) refresh() {
	if s.on {
		s.track.FillColor = accent().get()
	} else {
		s.track.FillColor = tcol(theme.ColorNameInputBorder)
	}
	s.track.StrokeColor = color.Transparent
	s.track.StrokeWidth = 0
	s.track.Refresh()
}

func (s *appleSwitch) CreateRenderer() fyne.WidgetRenderer { return &switchRenderer{s: s} }

type switchRenderer struct{ s *appleSwitch }

func (r *switchRenderer) Layout(size fyne.Size) {
	d := size.Height - 2*switchKnobPad
	x := switchKnobPad
	if r.s.on {
		x = size.Width - switchKnobPad - d
	}
	r.s.track.Resize(size)
	r.s.track.Move(fyne.NewPos(0, 0))
	r.s.knob.Resize(fyne.NewSize(d, d))
	r.s.knob.Move(fyne.NewPos(x, switchKnobPad))
}
func (r *switchRenderer) MinSize() fyne.Size { return fyne.NewSize(switchW, switchH) }
func (r *switchRenderer) Refresh()           { r.s.refresh(); r.Layout(r.s.Size()); canvas.Refresh(r.s) }
func (r *switchRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.s.track, r.s.knob}
}
func (r *switchRenderer) Destroy() {}

// tapLabel 可点击的文字。用于开关右侧的标签 —— 系统里点击文字同样能拨动开关。
type tapLabel struct {
	widget.BaseWidget
	text  *canvas.Text
	onTap func()
}

func newTapLabel(s string, size float32, c color.Color, fn func()) *tapLabel {
	t := &tapLabel{text: plainText(s, size, c), onTap: fn}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapLabel) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.text) }
func (t *tapLabel) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}
func (t *tapLabel) Cursor() desktop.Cursor { return desktop.PointerCursor }
func (t *tapLabel) SetText(s string)       { t.text.Text = s; t.text.Refresh() }

// switchRow 组装「开关 + 文字标签」，点击文字与拨动开关等效。
func switchRow(text string, on bool, changed func(bool)) (*appleSwitch, fyne.CanvasObject) {
	sw := newAppleSwitch(on, changed)
	lab := newTapLabel(text, fsSubhead, fgCol(), func() { sw.toggle() })
	// 用 GridWrap 把开关钉成 38×22：HBox 会把子控件纵向拉伸到整行高度，
	// 开关会被撑成 38×36，轨道变成一颗臃肿的胶囊。
	swBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(switchW, switchH)), sw)
	return sw, container.NewHBox(swBox, hSpace(sp8), lab)
}

// ---- 小控件 ----

// pill 胶囊标签（匹配模式等元信息）：强调色低透明度底 + 同色加粗小字，
// 对应 macOS 里邮件标签、文件标记的观感。
func pill(text string, c color.Color) fyne.CanvasObject {
	t := plainText(text, 10, c)
	t.TextStyle = fyne.TextStyle{Bold: true}
	bg := &canvas.Rectangle{FillColor: withAlpha(c, 0x1c), CornerRadius: 8}
	stack := container.NewStack(bg, container.NewCenter(t))
	w := t.MinSize().Width + sp16
	return container.New(layout.NewGridWrapLayout(fyne.NewSize(w, 16)), stack)
}

// toolbarButton 无边框的图标按钮，仅 hover 时显现圆角底色 —— macOS 工具栏观感。
func toolbarButton(res fyne.Resource, fn func()) *widget.Button {
	b := widget.NewButtonWithIcon("", res, fn)
	b.Importance = widget.LowImportance
	return b
}

// iconButton 列表行右侧的小图标按钮（低强调度，不抢主体内容）。
func iconButton(res fyne.Resource, fn func()) *widget.Button {
	b := widget.NewButtonWithIcon("", res, fn)
	b.Importance = widget.LowImportance
	return b
}

// badgeNum 序号徽章（收藏顺序 = ⌘ 编号）。
func badgeNum(n int) fyne.CanvasObject {
	bg := &canvas.Rectangle{FillColor: accent().get(), CornerRadius: 9}
	bg.SetMinSize(fyne.NewSize(18, 18))
	t := plainText(strconv.Itoa(n), fsCaption, color.NRGBA{0xff, 0xff, 0xff, 0xff})
	t.Alignment = fyne.TextAlignCenter
	t.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewStack(bg, container.NewCenter(t))
}

// ---- 倒计时进度条 ----

// progressLine 一条细进度线（颜色随剩余时间变化），用于选择器底部。
type progressLine struct {
	widget.BaseWidget
	track  *canvas.Rectangle
	fill   *canvas.Rectangle
	height float32
	frac   float64
}

func newProgressLine(trackCol color.Color, height float32) *progressLine {
	p := &progressLine{
		track:  &canvas.Rectangle{FillColor: trackCol},
		fill:   &canvas.Rectangle{FillColor: accent().get()},
		height: height,
		frac:   1,
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
	h := r.p.height
	y := (s.Height - h) / 2
	r.p.track.Resize(fyne.NewSize(s.Width, h))
	r.p.track.Move(fyne.NewPos(0, y))
	r.p.fill.Resize(fyne.NewSize(s.Width*float32(r.p.frac), h))
	r.p.fill.Move(fyne.NewPos(0, y))
}
func (r *progressLineRenderer) MinSize() fyne.Size { return fyne.NewSize(40, r.p.height) }
func (r *progressLineRenderer) Refresh()           { r.Layout(r.p.Size()); canvas.Refresh(r.p) }
func (r *progressLineRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.p.track, r.p.fill}
}
func (r *progressLineRenderer) Destroy() {}

// ---- 侧边栏项（设置窗口）----

// sidebarItem 设置窗口侧边栏的一项：图标 + 文字，选中时强调色圆角底 + 白色文字，
// 对齐 macOS「系统设置 / Finder」侧栏的观感。
type sidebarItem struct {
	widget.BaseWidget
	bg    *canvas.Rectangle
	icon  *widget.Icon
	label *canvas.Text
	res   fyne.Resource
	sel   bool
	onTap func()
}

func newSidebarItem(res fyne.Resource, text string, onTap func()) *sidebarItem {
	it := &sidebarItem{
		bg:    &canvas.Rectangle{CornerRadius: radControl},
		icon:  widget.NewIcon(res),
		label: plainText(text, fsBody, fgCol()),
		res:   res,
		onTap: onTap,
	}
	it.ExtendBaseWidget(it)
	it.refresh()
	return it
}

func (it *sidebarItem) Tapped(*fyne.PointEvent) { it.onTap() }
func (it *sidebarItem) Cursor() desktop.Cursor  { return desktop.PointerCursor }

func (it *sidebarItem) setSelected(s bool) { it.sel = s; it.refresh() }

func (it *sidebarItem) refresh() {
	if it.sel {
		it.bg.FillColor = accent().get()
		it.label.Color = color.NRGBA{0xff, 0xff, 0xff, 0xff}
		it.icon.SetResource(theme.NewColoredResource(it.res, theme.ColorNameForegroundOnPrimary))
	} else {
		it.bg.FillColor = color.Transparent
		it.label.Color = fgCol()
		it.icon.SetResource(theme.NewColoredResource(it.res, theme.ColorNameForeground))
	}
	it.bg.Refresh()
	it.label.Refresh()
}

func (it *sidebarItem) CreateRenderer() fyne.WidgetRenderer {
	row := container.NewHBox(hSpace(sp8), it.icon, hSpace(sp8), it.label, hSpace(sp8))
	return widget.NewSimpleRenderer(container.NewStack(it.bg, row))
}

// ---- 图标 ----

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

// circle 生成直径为 d 的实心圆。canvas.Circle 没有 SetMinSize，
// 只有对角两个顶点（Position1/Position2），故用一个透明的占位矩形把尺寸撑出来。
func circle(c color.Color, d float32) fyne.CanvasObject {
	sp := canvas.NewRectangle(color.Transparent)
	sp.SetMinSize(fyne.NewSize(d, d))
	return container.NewStack(sp, canvas.NewCircle(c))
}

// letterAvatar 用名称首字母生成柔和的圆形头像（取不到真实图标时兜底）。
// 底色取强调色的低透明度而非实色：深色模式下实色底 + 白字对比过强、显廉价。
func letterAvatar(name string, size float32) fyne.CanvasObject {
	acc := accent().get()
	bg := circle(withAlpha(acc, 0x24), size)
	ch := "?"
	if rs := []rune(name); len(rs) > 0 {
		ch = strings.ToUpper(string(rs[0]))
	}
	letter := canvas.NewText(ch, acc)
	letter.TextSize = size * 0.46
	letter.Alignment = fyne.TextAlignCenter
	letter.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewStack(bg, container.NewCenter(letter))
}

// ---- 文本工具 ----

// shortName 去掉常见厂商前缀，界面里更紧凑。
func shortName(n string) string {
	n = strings.TrimPrefix(n, "Google ")
	n = strings.TrimPrefix(n, "Microsoft ")
	return n
}

// hostOf 提取 URL 的主机名（去掉协议、路径与 www. 前缀）。
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

// ---- 窗口级工具 ----

// trackedWindows 是「主窗口 → 它开过的子窗口」映射。
//
// 关掉主窗口时手动遍历它开过的子窗口一并关掉 —— Fyne 的 fyne.Window 接口
// 不暴露稳定身份（test 驱动下 a.NewWindow 返回的类型与 AllWindows() 的元素
// 类型不一致，无法用 `==` 判等），所以只能由开窗者自己记账。
//
// 只在 UI 线程访问（Fyne 的所有回调均在主线程），故无需加锁。
var trackedWindows = map[fyne.Window][]fyne.Window{}

// trackChild 登记：child 是 parent 开出来的子窗口。
// parent 关闭时会一并关掉所有登记的 child —— 避免「关闭主窗口后子窗口被孤立」。
func trackChild(parent, child fyne.Window) {
	trackedWindows[parent] = append(trackedWindows[parent], child)
	// parent 已关闭后调用 child.SetOnClosed 把它从表里摘掉 —— 否则反复开/关
	// 后表会无限膨胀。
	parent.SetOnClosed(func() {
		for _, w := range trackedWindows[parent] {
			w.Close()
		}
		delete(trackedWindows, parent)
	})
	// child 关闭时也摘掉自己。
	child.SetOnClosed(func() {
		children := trackedWindows[parent]
		out := children[:0]
		for _, c := range children {
			if c != child {
				out = append(out, c)
			}
		}
		trackedWindows[parent] = out
	})
}

// closeTrackedWindows 关掉 parent 开过的所有子窗口。给 picker's closeIntercept 用。
func closeTrackedWindows(parent fyne.Window) {
	for _, w := range trackedWindows[parent] {
		w.Close()
	}
	delete(trackedWindows, parent)
}
