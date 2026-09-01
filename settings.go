package main

import (
	"github.com/kklinan/browser-switch/i18n"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 设置窗口沿用 macOS「系统设置」的信息架构：左侧边栏切换面板，右侧是内容。
// 相比顶部标签栏，侧边栏能承载更长的面板名（多语言下尤其明显），
// 也更符合用户在 macOS 上对「设置」的预期。

// settingsSelected 记录当前显示的面板序号，切换语言重建窗口后保持不变。
var settingsSelected = 0

// ShowSettings 打开设置窗口（独立进程/独立 app）。
func ShowSettings(cfg *Config) {
	a := app.New()
	w := a.NewWindow(i18n.T("settings.window_title"))
	w.SetContent(buildSettingsContent(a, cfg, w))
	w.Resize(fyne.NewSize(840, 600))
	w.CenterOnScreen()
	w.ShowAndRun()
}

// OpenSettingsWindow 在已有 app 内开设置窗口（供选择器齿轮按钮复用，不另起 app）。
// parent 是触发它的窗口（通常是 picker 主窗口），picker 关闭时会一并关掉设置窗口。
func OpenSettingsWindow(a fyne.App, cfg *Config, parent fyne.Window) {
	w := a.NewWindow(i18n.T("settings.window_title"))
	if parent != nil {
		trackChild(parent, w)
	}
	w.SetContent(buildSettingsContent(a, cfg, w))
	w.Resize(fyne.NewSize(840, 600))
	w.CenterOnScreen()
	w.Show()
}

// buildSettingsContent 构建设置窗口（侧边栏 + 内容区），不驱动主循环。
func buildSettingsContent(a fyne.App, cfg *Config, w fyne.Window) fyne.CanvasObject {
	// 三个面板各自用稳定的 Stack 包裹：刷新时替换 Stack 内部对象再 Refresh，
	// 而非替换外层容器 —— 后者在 Fyne 2.7 对当前显示内容不生效，会导致操作看似无反应。
	holders := []*fyne.Container{container.NewStack(), container.NewStack(), container.NewStack()}

	var rebuildBrowsers func()
	rebuildBrowsers = func() {
		holders[0].Objects = []fyne.CanvasObject{buildBrowsersTab(cfg, rebuildBrowsers)}
		holders[0].Refresh()
	}
	rebuildBrowsers()
	// 规则面板自带局部刷新（搜索/增删只重建列表），无需整体重建。
	// 透传 w：添加规则对话框在 w 关闭时也要关。
	holders[1].Objects = []fyne.CanvasObject{buildRulesTab(a, cfg, w)}
	holders[1].Refresh()

	// 切换语言：整窗重建（各面板文案都要换）。
	// 当前面板由包级变量 settingsSelected 记录，重建时会落在同一个面板上，
	// 无需在此处保存/恢复 —— 早先版本在这里赋值反而造成「重建后回到首个面板」的错觉。
	rebuildAll := func() { w.SetContent(buildSettingsContent(a, cfg, w)) }
	holders[2].Objects = []fyne.CanvasObject{buildGeneralTab(cfg, w, rebuildAll)}
	holders[2].Refresh()

	// ---- 侧边栏 ----
	meta := []struct {
		title string
		icon  fyne.Resource
	}{
		{i18n.T("settings.tab.browsers"), theme.ComputerIcon()},
		{i18n.T("settings.tab.rules"), theme.ListIcon()},
		{i18n.T("settings.tab.general"), theme.SettingsIcon()},
	}

	content := container.NewStack()
	items := make([]*sidebarItem, len(meta))
	showPane := func(idx int) {
		if idx < 0 || idx >= len(meta) {
			idx = 0
		}
		settingsSelected = idx
		content.Objects = []fyne.CanvasObject{holders[idx]}
		content.Refresh()
		for i, it := range items {
			it.setSelected(i == idx)
		}
	}

	sideRows := container.NewVBox(vSpace(sp8))
	for i, m := range meta {
		idx := i
		items[i] = newSidebarItem(m.icon, m.title, func() { showPane(idx) })
		sideRows.Add(container.New(layout.NewGridWrapLayout(fyne.NewSize(sidebarW, sidebarItemH)), items[i]))
		sideRows.Add(vSpace(sp2))
	}
	sidebar := container.NewBorder(nil, nil, hSpace(sp8), hSpace(sp8), sideRows)

	showPane(settingsSelected)

	body := container.NewBorder(nil, nil, container.NewHBox(sidebar, vLine()), nil, content)
	return body
}

// 侧边栏尺寸：宽度取语言中最长面板名的安全值，高度固定一行的行高。
const (
	sidebarW      = float32(184)
	sidebarItemH  = float32(30)
	browserIconSm = float32(22)
	browserIconMd = float32(26)
)

// ---- 跨面板共用的行、空态与工具 ----

// listRow 列表中的一行：内容左右留白 + 底部发丝线（分隔线自左侧缩进，与文字对齐）。
func listRow(content fyne.CanvasObject, indent float32) fyne.CanvasObject {
	return container.NewVBox(
		container.NewBorder(nil, nil, hSpace(indent), hSpace(sp12), content),
		container.NewBorder(nil, nil, hSpace(indent), nil, hairline()),
	)
}

// formRow 一行「左侧标签 + 右侧控件」，不带分隔线 —— 分隔由 groupBox 负责。
func formRow(label string, control fyne.CanvasObject) fyne.CanvasObject {
	return container.NewBorder(nil, nil, widget.NewLabel(label), control)
}

// groupBox 把若干行装进一张圆角卡片，行间用左侧缩进的发丝线分隔。
// macOS「系统设置」把相关条目成组放进圆角容器，而不是散落在窗口里 ——
// 这样分组关系一眼可见，也省掉了每行一条贯通全宽的分隔线。
func groupBox(rows ...fyne.CanvasObject) fyne.CanvasObject {
	inner := container.NewVBox()
	for i, r := range rows {
		if i > 0 {
			inner.Add(container.NewBorder(nil, nil, hSpace(sp12), nil, hairline()))
		}
		inner.Add(container.NewBorder(nil, nil, hSpace(sp12), hSpace(sp12), r))
	}
	bg := &canvas.Rectangle{FillColor: tcol(theme.ColorNameInputBackground), CornerRadius: radCard}
	bg.StrokeColor = withAlpha(fgCol(), 0x0c)
	bg.StrokeWidth = 1
	return container.NewStack(bg, inner)
}

// emptyState 空列表占位：居中、次级色、带一枚弱化图标，比一行文字更像原生空态。
func emptyState(text string) fyne.CanvasObject {
	t := plainText(text, fsSubhead, secCol())
	t.Alignment = fyne.TextAlignCenter
	return container.NewCenter(container.NewVBox(
		vSpace(sp24),
		widget.NewIcon(theme.NewDisabledResource(theme.ListIcon())),
		vSpace(sp8),
		t,
		vSpace(sp24),
	))
}

func mergeDetected(cfg *Config) []Browser {
	detected := DetectBrowsers()
	seen := map[string]bool{}
	var out []Browser
	for _, b := range detected {
		out = append(out, b)
		seen[b.ID] = true
	}
	// 保留用户自定义的浏览器
	for _, b := range cfg.Browsers {
		if b.IsCustom && !seen[b.ID] {
			out = append(out, b)
		}
	}
	return out
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}
