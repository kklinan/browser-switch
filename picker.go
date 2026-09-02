package main

import (
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kklinan/browser-switch/i18n"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ShowPicker 一次性弹出选择器：自建 app、进入主循环，选择/超时后启动浏览器并退出进程。
func ShowPicker(url string, cfg *Config) {
	a := app.New()
	w := a.NewWindow(i18n.T("picker.window_title"))
	setupPickerWindow(a, w, url, cfg, func() { a.Quit() })
	w.ShowAndRun()
}

// setupPickerWindow 在给定窗口上装配选择器交互。done 为“关闭此选择器”动作：
// 一次性模式传 a.Quit（退出进程），常驻 agent 模式传 w.Close（仅关窗，进程留存）。
func setupPickerWindow(a fyne.App, w fyne.Window, url string, cfg *Config, done func()) {
	var picked atomic.Bool
	open := func(b Browser, remember bool) {
		if !picked.CompareAndSwap(false, true) {
			return
		}
		if remember {
			addRuleForURL(url, b, cfg)
		}
		if err := LaunchBrowser(b, url, false); err != nil {
			fmt.Fprintf(os.Stderr, i18n.T("browser.open_failed"), err)
		}
		done()
	}
	openProfile := func(b Browser, p Profile, remember bool) {
		if !picked.CompareAndSwap(false, true) {
			return
		}
		if remember {
			addRuleForURL(url, b, cfg) // 仅记浏览器；配置档不进规则
		}
		if err := LaunchBrowserProfile(b, p, url, false); err != nil {
			fmt.Fprintf(os.Stderr, i18n.T("browser.open_profile_failed"), p.Name, b.Name, err)
		}
		done()
	}
	// cancel 取消选择器（Esc / 点击窗口关闭按钮）：先抢占 picked 阻断倒计时自动打开，
	// 再执行 done 关闭。CAS 保证与 open/openProfile 及倒计时到点互斥——已选择或已打开则不重复触发。
	cancel := func() {
		if !picked.CompareAndSwap(false, true) {
			return
		}
		done()
	}

	pu := buildPickerUI(a, w, url, cfg, open, openProfile)
	w.SetContent(pu.root)

	// 点击窗口关闭按钮（左上红点）等同取消：仅关窗，不再自动用默认浏览器打开。
	w.SetCloseIntercept(cancel)

	// Esc 取消 / Enter 打开默认 / 纯数字键触发对应浏览器
	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		switch {
		case k.Name == fyne.KeyEscape:
			cancel()
		case k.Name == fyne.KeyReturn || k.Name == fyne.KeyEnter:
			if pu.def != nil {
				open(*pu.def, pu.remember.Checked)
			}
		default:
			if n, err := strconv.Atoi(string(k.Name)); err == nil && n >= 1 && n <= len(pu.shortcutActions) {
				pu.shortcutActions[n-1]()
			}
		}
	})

	// ⌘1..⌘9 打开第 N 个浏览器 —— 即便它被收进“更多”未直接显示，也能直接打开。
	for i := 0; i < 9; i++ {
		n := i
		w.Canvas().AddShortcut(&desktop.CustomShortcut{
			KeyName:  fyne.KeyName(strconv.Itoa(n + 1)),
			Modifier: fyne.KeyModifierSuper,
		}, func(fyne.Shortcut) {
			if n < len(pu.shortcutActions) {
				pu.shortcutActions[n]()
			}
		})
	}

	// ⌘R：与点击"更多"卡片完全等价 —— 展开完整浏览器列表。
	w.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyR,
		Modifier: fyne.KeyModifierSuper,
	}, func(fyne.Shortcut) {
		fyne.Do(func() { pu.expand() })
	})

	// 倒计时：用 stopCh 而非固定 Sleep，保证"暂停"（打开设置）能立即中断等待，
	// 不会出现刚点设置、1 秒后又被倒计时自动打开浏览器关掉设置窗口的竞态。
	total := cfg.AutoCloseDelay
	stopCh := make(chan struct{})
	var stopOnce sync.Once // 齿轮可能被反复点击，close 只能执行一次
	stopped := func() bool {
		select {
		case <-stopCh:
			return true
		default:
			return picked.Load()
		}
	}
	if total > 0 && pu.def != nil {
		go func() {
			for i := total; i > 0; i-- {
				if stopped() {
					return
				}
				rem := i
				fyne.Do(func() { pu.setCountdown(rem) })
				select {
				case <-time.After(time.Second):
				case <-stopCh:
					return
				}
			}
			if stopped() {
				return
			}
			fyne.Do(func() { open(*pu.def, false) })
		}()
	}

	// 当用户点击设置（齿轮）时调用：暂停倒计时并隐藏倒计时显示，让设置窗口能正常停留。
	pu.onSettings = func() {
		stopOnce.Do(func() { close(stopCh) })
		pu.hideCountdown()
		OpenSettingsWindow(a, cfg)
	}

	resizePicker(w, 1) // 默认 1 排
	w.CenterOnScreen()
}

// resizePicker 按浏览器数量（行数）调整窗口高度，最多 2 行。
func resizePicker(w fyne.Window, n int) {
	if n > 8 {
		n = 8
	}
	rows := (n + 3) / 4
	if rows < 1 {
		rows = 1
	}
	w.Resize(fyne.NewSize(560, float32(210+rows*118)))
}

// pickerUI 聚合选择器界面与其交互所需的句柄（供窗口与离屏渲染共用）。
type pickerUI struct {
	root            fyne.CanvasObject
	def             *Browser
	remember        *widget.Check
	setCountdown    func(remaining int)
	hideCountdown   func()
	expand          func()   // 展开为完整列表（多排），等价于点击"更多"卡片
	shortcutActions []func() // ⌘(i+1) 打开第 i 个浏览器（完整列表，含“更多”里隐藏的）
	// onSettings 由 setupPickerWindow 赋值：点齿轮时同时暂停倒计时，避免弹出设置后窗口被自动关闭。
	onSettings func()
}

// buildPickerUI 构建选择器内容（不驱动主循环）。open 处理卡片点击/快捷键，
// openProfile 处理右键菜单里选定的多账户配置。
func buildPickerUI(a fyne.App, w fyne.Window, url string, cfg *Config, open func(Browser, bool), openProfile func(Browser, Profile, bool)) *pickerUI {
	fg := tcol(theme.ColorNameForeground)
	subtle := tcol(theme.ColorNamePlaceHolder)
	surface := tcol(theme.ColorNameInputBackground)
	pal := paletteFromTheme()

	// 提前声明 pu，以便齿轮按钮 / 卡片回调等内部闭包可以在 pu 填充后访问 onSettings。
	var pu *pickerUI

	// 完整有序列表：有收藏则取收藏项（浏览器或具体账户），否则全部（已去隐藏）。⌘N 始终映射到它。
	items := cfg.FavoriteItems()
	def := findBrowserByID(cfg.DefaultBrowser, cfg)
	if def == nil && len(items) > 0 {
		def = &items[0].Browser
	}

	rememberChk := widget.NewCheck(i18n.Tf("picker.remember", hostOf(url)), nil)

	// 顶部满宽地址栏：点击整段即可复制完整 URL 到剪贴板（Hyperlink 自带 hover 视觉提示）
	globe := plainText("🌐", 15, accent)
	urlLabel := newClickableURLLabel(url)
	addrBG := canvas.NewRectangle(surface)
	addrBG.CornerRadius = 9
	addrBG.StrokeColor = withAlpha(fg, 0x10)
	addrBG.StrokeWidth = 1
	header := container.NewStack(addrBG, container.NewBorder(nil, nil, container.NewPadded(globe), nil, urlLabel))

	// 倒计时
	total := cfg.AutoCloseDelay
	countText := plainText("", 12, subtle)
	countText.Alignment = fyne.TextAlignCenter
	prog := newProgressLine(withAlpha(fg, 0x14))
	countArea := container.NewVBox(container.NewCenter(countText), prog)
	if total <= 0 || def == nil {
		countArea.Hide()
	}
	setCountdown := func(remaining int) {
		if def == nil {
			return
		}
		c := accent
		countText.Color = subtle
		if remaining <= 3 {
			c = warn
			countText.Color = warn
		}
		countText.Text = i18n.Tf("picker.countdown", remaining, shortName(def.Name))
		countText.Refresh()
		if total > 0 {
			prog.set(float64(remaining)/float64(total), c)
		}
	}

	// ⌘N 动作覆盖完整列表（即便未直接显示）。收藏了具体账户的项直接用该账户打开，
	// 收藏整浏览器的项若有多账户则弹账户菜单、否则直接打开。
	profilesByID := map[string][]Profile{}
	shortcutActions := make([]func(), len(items))
	for i := range items {
		it := items[i]
		if it.Profile != nil {
			// 收藏的是具体账户：⌘N 直接用该账户打开。
			b, p := it.Browser, *it.Profile
			shortcutActions[i] = func() { openProfile(b, p, rememberChk.Checked) }
			continue
		}
		// 收藏的是整浏览器：预取其多账户配置，多账号则弹菜单，单账号直接开。
		b := it.Browser
		profs := DetectProfiles(b)
		if len(profs) > 0 {
			profilesByID[b.ID] = profs
			shortcutActions[i] = func() { showProfileMenu(w, nil, b, profs, rememberChk.Checked, openProfile) }
		} else {
			shortcutActions[i] = func() { open(b, rememberChk.Checked) }
		}
	}

	// 卡片网格：默认 1 排（前 3 个 + “更多”）；点“更多”展开成多排。
	const oneRow = 4
	gridHolder := container.NewStack()
	var renderGrid func(expanded bool)
	renderGrid = func(expanded bool) {
		visN := len(items)
		compact := !expanded && visN > oneRow
		if compact {
			visN = oneRow - 1 // 留一格给“更多”
		}
		var objs []fyne.CanvasObject
		for i := 0; i < visN; i++ {
			it := items[i]
			b := it.Browser
			tapAction := shortcutActions[i]

			if it.Profile != nil {
				// 收藏的具体账户：标题「浏览器 · 账户」，左键直接用该账户打开，无二级菜单。
				title := shortName(b.Name) + " · " + shortName(it.Profile.Name)
				c := makePickerCard(browserIcon(b, 52), title, i18n.Tf("picker.shortcut", i+1), fg, subtle, pal, tapAction)
				objs = append(objs, shadowed(c))
				continue
			}

			profs := profilesByID[b.ID]
			sub := i18n.Tf("picker.shortcut", i+1)
			if len(profs) > 0 {
				// 多账号整浏览器：副标题提示可选账户，左键点击直接展开账户菜单（右键亦可）。
				sub = i18n.Tf("picker.shortcut_profiles", i+1)
			}
			c := makePickerCard(browserIcon(b, 52), shortName(b.Name), sub, fg, subtle, pal, tapAction)
			if def != nil && b.ID == def.ID {
				c.setSelected(true)
			}
			if len(profs) > 0 {
				cc, bb := c, b
				c.onSecondary = func() { showProfileMenu(w, cc, bb, profs, rememberChk.Checked, openProfile) }
			}
			objs = append(objs, shadowed(c))
		}
		if compact {
			hidden := len(items) - visN
			more := makePickerCard(moreGlyph(fg, 52), i18n.T("picker.more"), i18n.Tf("picker.more_detail", hidden, visN+1), fg, subtle, pal, func() {
				renderGrid(true)
				resizePicker(w, len(items))
			})
			objs = append(objs, shadowed(more))
		}
		cols := len(objs)
		if cols > 4 {
			cols = 4
		}
		gridHolder.Objects = []fyne.CanvasObject{container.NewGridWithColumns(cols, objs...)}
		gridHolder.Refresh()
	}
	renderGrid(false)

	// hideCountdown / expand 都通过 pu 来对外暴露 —— ⌘R 与点击设置都需要用到它们。
	hideCountdown := func() { countArea.Hide() }
	expand := func() {
		renderGrid(true)
		resizePicker(w, len(items))
	}

	// 底部动作
	copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		a.Clipboard().SetContent(url)
	})
	copyBtn.Importance = widget.LowImportance
	gearBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		// 优先走 setupPickerWindow 注入的回调（它会同步暂停倒计时并打开设置窗口），
		// 避免回调缺失时点击齿轮等于无反应。
		if pu != nil && pu.onSettings != nil {
			pu.onSettings()
		} else {
			OpenSettingsWindow(a, cfg)
		}
	})
	gearBtn.Importance = widget.LowImportance
	footer := container.NewBorder(nil, nil, rememberChk, container.NewHBox(copyBtn, gearBtn))

	title := boldText(i18n.T("picker.title"), 15, fg)

	root := container.NewPadded(container.NewVBox(
		container.NewCenter(title),
		header,
		widget.NewSeparator(),
		container.NewPadded(gridHolder),
		container.NewPadded(countArea),
		widget.NewSeparator(),
		footer,
	))

	pu = &pickerUI{
		root:            root,
		def:             def,
		remember:        rememberChk,
		setCountdown:    setCountdown,
		hideCountdown:   hideCountdown,
		expand:          expand,
		shortcutActions: shortcutActions,
	}
	return pu
}

// showProfileMenu 在卡片下方弹出该浏览器的多账户配置菜单，选定后用对应配置打开。
func showProfileMenu(w fyne.Window, c *card, b Browser, profiles []Profile, remember bool, openProfile func(Browser, Profile, bool)) {
	var items []*fyne.MenuItem
	for i := range profiles {
		p := profiles[i]
		label := p.Name
		switch p.Kind {
		case "default":
			label = i18n.Tf("picker.default_profile", p.Name)
		case "incognito":
			label = i18n.Tf("picker.incognito_profile", p.Name)
		default:
			label = i18n.Tf("picker.other_profile", p.Name)
		}
		items = append(items, fyne.NewMenuItem(label, func() { openProfile(b, p, remember) }))
	}
	menu := fyne.NewMenu(shortName(b.Name), items...)
	// c 为卡片时把菜单弹在卡片正下方；c 为 nil（左键触发，无锚点）时弹在窗口中心偏上。
	var pos fyne.Position
	if c != nil {
		pos = fyne.CurrentApp().Driver().AbsolutePositionForObject(c)
		pos.Y += c.Size().Height
	} else {
		sz := w.Canvas().Size()
		pos = fyne.NewPos(sz.Width/2-90, sz.Height/2-60)
	}
	widget.ShowPopUpMenuAtPosition(menu, w.Canvas(), pos)
}

func makePickerCard(icon fyne.CanvasObject, title, shortcut string, fg, subtle color.Color, pal cardPalette, onTap func()) *card {
	name := plainText(title, 12.5, fg)
	name.Alignment = fyne.TextAlignCenter
	key := plainText(shortcut, 11, subtle)
	key.Alignment = fyne.TextAlignCenter
	bg := &canvas.Rectangle{CornerRadius: 14}
	inner := container.NewVBox(
		container.NewPadded(container.NewCenter(icon)),
		container.NewCenter(name),
		container.NewCenter(key),
	)
	stack := container.NewStack(bg, container.NewPadded(inner))
	return newCard(stack, bg, pal, onTap)
}

// ---- 可点击复制的 URL 标签 ----

// clickableURLLabel 是 picker 头部地址栏的"点击复制"组件 ——
// 底层是 widget.Hyperlink（自带 hover/cursor/focus 视觉），但劫持 OnTapped
// 让点击时仅把整段 URL 写入剪贴板，而不是让浏览器打开 URL 本身。
type clickableURLLabel struct {
	widget.Hyperlink
}

func newClickableURLLabel(text string) *clickableURLLabel {
	l := &clickableURLLabel{Hyperlink: *widget.NewHyperlink(text, nil)}
	l.Hyperlink.Truncation = fyne.TextTruncateEllipsis
	l.Hyperlink.OnTapped = func() { fyne.CurrentApp().Clipboard().SetContent(l.Hyperlink.Text) }
	l.ExtendBaseWidget(l)
	return l
}

// ---- 记住选择 → 生成规则 ----

func addRuleForURL(rawURL string, browser Browser, cfg *Config) {
	host := extractDomain(rawURL)
	if host == "" {
		return
	}
	for _, r := range cfg.Rules {
		if r.Pattern == host && r.Browser == browser.ID {
			return
		}
	}
	rule := Rule{
		ID:       fmt.Sprintf("auto_%s_%d", host, time.Now().UnixNano()),
		Pattern:  host,
		Mode:     MatchExact,
		Browser:  browser.ID,
		Priority: 100,
		Enabled:  true,
		Comment:  fmt.Sprintf("Auto-created for %s", rawURL),
	}
	cfg.Rules = append(cfg.Rules, rule)
	_ = SaveConfig(cfg)
}

func extractDomain(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if idx := strings.Index(rawURL, "://"); idx != -1 {
		rawURL = rawURL[idx+len("://"):]
	}
	rawURL = strings.TrimPrefix(rawURL, "www.")
	if idx := strings.Index(rawURL, "/"); idx != -1 {
		rawURL = rawURL[:idx]
	}
	if idx := strings.Index(rawURL, ":"); idx != -1 {
		rawURL = rawURL[:idx]
	}
	if idx := strings.Index(rawURL, "?"); idx != -1 {
		rawURL = rawURL[:idx]
	}
	return strings.ToLower(rawURL)
}
