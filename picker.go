package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kklinan/browser-switch/i18n"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 选择器的尺寸模型：磁贴尺寸固定，窗口随磁贴列数与行数伸缩。
// 头部与底部的高度不写死 —— 它们随字号与语言变化，运行时实测最小尺寸更可靠。
const (
	pickerMaxW   = float32(540) // 窗口最大宽度
	pickerMaxCol = 5            // 一行最多几个磁贴
	pickerMaxRow = 3            // 最多几行（超出改为滚动）
	pickerCellW  = float32(112) // 单个磁贴的最大宽度
	pickerRowH   = float32(112) // 单个磁贴的高度（含投影；磁贴内容最小高度约 111）
	pickerGap    = float32(4)   // 网格间距（Fyne 的 GridWrap 用 theme.Padding，取值一致）
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
		if err := LaunchBrowser(b, url); err != nil {
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
		if err := LaunchBrowserProfile(b, p, url); err != nil {
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
	// 窗口尺寸由内容决定，不允许用户拉伸——原生弹窗同样不可缩放。
	w.SetFixedSize(true)

	// 点击窗口关闭按钮（左上红点）等同取消：先关掉子窗口（设置 / 添加规则），
	// 再走 cancel（抢 picked 阻断倒计时 + done）。
	// 顺序：先关子窗口再 cancel —— 若反过来，cancel 触发的 w.Close 会让
	// trackedWindows 跟着摘表，子窗口就成孤儿了。
	w.SetCloseIntercept(func() {
		closeTrackedWindows(w)
		cancel()
	})

	// Esc 取消 / 回车或空格打开选中项 / 方向键移动选中 / 纯数字键直接触发
	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		switch k.Name {
		case fyne.KeyEscape:
			cancel()
		case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
			pu.activate()
		case fyne.KeyLeft:
			pu.moveSelection(-1)
		case fyne.KeyRight:
			pu.moveSelection(1)
		case fyne.KeyUp:
			pu.moveSelection(-pu.colCount())
		case fyne.KeyDown:
			pu.moveSelection(pu.colCount())
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

	// 倒计时。先立即画一次初值：否则首秒内进度条是空的、文字也还没出现，
	// 看起来像「功能没生效」，一秒后才突然跳动。
	total := cfg.AutoCloseDelay
	if total > 0 && pu.def != nil {
		pu.setCountdown(total)
		go func() {
			for i := total; i > 0; i-- {
				fyne.Do(func() { pu.setCountdown(i) })
				time.Sleep(time.Second)
				if picked.Load() {
					return
				}
			}
			fyne.Do(func() { open(*pu.def, false) })
		}()
	}

	pu.resize()
	w.CenterOnScreen()
}

// pickerUI 聚合选择器界面与其交互所需的句柄（供窗口与离屏渲染共用）。
type pickerUI struct {
	root            fyne.CanvasObject
	def             *Browser
	remember        *appleSwitch
	setCountdown    func(remaining int)
	shortcutActions []func() // ⌘(i+1) 打开第 i 个浏览器（完整列表，含收起时隐藏的）
	activate        func()   // 打开当前选中项
	moveSelection   func(delta int)
	resize          func()     // 按当前展开状态重算窗口尺寸
	colCount        func() int // 当前网格列数（供上下键按行移动）
	expand          func()     // 展开 / 收起「显示其余 N 个」
}

// buildPickerUI 构建选择器内容（不驱动主循环）。open 处理卡片点击/快捷键，
// openProfile 处理右键菜单里选定的多账户配置。
func buildPickerUI(a fyne.App, w fyne.Window, url string, cfg *Config, open func(Browser, bool), openProfile func(Browser, Profile, bool)) *pickerUI {
	pal := paletteFromTheme()
	fg := fgCol()

	// 完整有序列表：有收藏则取收藏项（浏览器或具体账户），否则全部（已去隐藏）。⌘N 始终映射到它。
	items := cfg.FavoriteItems()
	def := findBrowserByID(cfg.DefaultBrowser, cfg)
	if def == nil && len(items) > 0 {
		def = &items[0].Browser
	}

	// ---- 头部：链接信息 ----
	// 不再放「选择浏览器打开」这类标题条：主机名用粗体、完整链接用次级色，
	// 用户一眼就知道要为哪个链接做决定，这与系统的共享表单一致。
	host := hostOf(url)
	if host == "" {
		host = url
	}
	header := container.NewVBox(
		plainText(i18n.T("picker.open_link"), fsCaption, secCol()),
		vSpace(sp2),
		boldText(truncateTail(host, 44), fsTitle, fg),
		vSpace(sp2),
		plainText(truncateTail(url, 64), fsCaption, secCol()),
	)

	// ---- 底部：记住选择 / 工具按钮 / 倒计时 ----
	rememberSw, rememberRow := switchRow(i18n.Tf("picker.remember", truncateTail(host, 24)), false, nil)

	countText := plainText("", fsCaption, secCol())
	prog := newProgressLine(withAlpha(fg, 0x12), 3)
	counting := cfg.AutoCloseDelay > 0 && def != nil
	setCountdown := func(remaining int) {
		if def == nil {
			return
		}
		// 文字始终保持次级色：情绪化变红（"快到了"）会打断用户阅读。
		// 进度条本身就是倒计时提示 —— 这点 macOS「自动锁屏」也是同色文案 + 进度条。
		countText.Text = i18n.Tf("picker.countdown", remaining, shortName(def.Name))
		countText.Refresh()
		prog.set(float64(remaining)/float64(cfg.AutoCloseDelay), accent().get())
	}
	if !counting {
		countText.Hide()
		prog.Hide()
	}

	// ---- ⌘N 动作（覆盖完整列表，即便未直接显示）----
	// 收藏了具体账户的项直接用该账户打开；收藏整浏览器的项若有多账户则弹账户菜单、否则直接打开。
	profilesByID := map[string][]Profile{}
	shortcutActions := make([]func(), len(items))
	for i := range items {
		it := items[i]
		if it.Profile != nil {
			// 收藏的是具体账户：⌘N 直接用该账户打开。
			b, p := it.Browser, *it.Profile
			shortcutActions[i] = func() { openProfile(b, p, rememberSw.IsOn()) }
			continue
		}
		// 收藏的是整浏览器：预取其多账户配置，多账号则弹菜单，单账号直接开。
		b := it.Browser
		profs := DetectProfiles(b)
		if len(profs) > 0 {
			profilesByID[b.ID] = profs
			shortcutActions[i] = func() { showProfileMenu(w, nil, b, profs, rememberSw.IsOn(), openProfile) }
		} else {
			shortcutActions[i] = func() { open(b, rememberSw.IsOn()) }
		}
	}

	// ---- 磁贴网格 ----
	// gridBox 是网格的落点：renderGrid 每次重建其中的网格（固定尺寸 + 超出则滚动）。
	gridBox := container.NewStack()
	var (
		cards       []*card
		cardActions []func()
		expanded    bool
		cols        = pickerMaxCol
		gridH       float32 // 网格区高度：renderGrid 算好，resize 直接用
		discShown   bool
		selIdx      int
		usingKeys   bool // 是否处于键盘导航态：决定要不要画聚焦环
	)
	// topBox / bottomBox / progWrap / root 在组装阶段赋值；resize 要读 root 的最小高度。
	var topBox, bottomBox, progWrap, root fyne.CanvasObject

	// 初始选中项 = 默认浏览器所在的位置；找不到则选中第一项。
	if def != nil {
		for i := range items {
			if items[i].Browser.ID == def.ID {
				selIdx = i
				break
			}
		}
	}

	// renderGrid / resize 互相引用，先声明再赋值。
	var renderGrid, resize func()

	discBtn := &widget.Button{}
	discBtn.OnTapped = func() {
		expanded = !expanded
		renderGrid()
		resize()
	}
	discBtn.Importance = widget.LowImportance
	discBox := container.NewCenter(discBtn)

	applySelection := func() {
		for i, c := range cards {
			c.setSelected(i == selIdx)
			c.setFocused(i == selIdx && usingKeys)
		}
	}

	renderGrid = func() {
		n := len(items)
		cols = n
		if cols > pickerMaxCol {
			cols = pickerMaxCol
		}
		if cols < 1 {
			cols = 1
		}
		visN := n
		discShown = n > pickerMaxCol
		if discShown && !expanded {
			visN = pickerMaxCol // 收起态只显示一行
		}

		// 网格宽度 = 窗口宽度 − 左右边距 − Border 中间区自带的左右 padding，
		// 再按 cols 均分出格宽（GridLayout 会自行均分，无需再算 cellW）。
		//
		// Border 即使没有 left/right 子对象，也会给中间区预留 2×theme.Padding()，
		// 漏掉这 8px 的话网格就会比窗口宽，整行被横向压缩几个像素。
		availW := winWidth(cols, bottomBox) - 2*sp20 - 2*theme.Padding()
		gridW := pickerCellW*float32(cols) + pickerGap*float32(cols-1)
		if gridW > availW {
			gridW = availW
		}

		// 行数：展开态按 visN/cols 自动算；收起态固定 1 行。
		// 高度由我们自己算并用 fixedSize 钉住，不依赖布局的 MinSize。
		rows := (visN + cols - 1) / cols
		if discShown && !expanded {
			rows = 1
		}
		if rows < 1 {
			rows = 1
		}
		if rows > pickerMaxRow {
			rows = pickerMaxRow
		}
		gridH = pickerRowH*float32(rows) + pickerGap*float32(rows-1)

		cards = cards[:0]
		cardActions = cardActions[:0]
		objs := make([]fyne.CanvasObject, 0, visN)
		for i := 0; i < visN; i++ {
			it := items[i]
			b := it.Browser
			action := shortcutActions[i]

			title, sub := shortName(b.Name), i18n.Tf("picker.shortcut", i+1)
			if it.Profile != nil {
				// 收藏的具体账户：标题「浏览器 · 账户」，左键直接用该账户打开，无二级菜单。
				title = shortName(b.Name) + " · " + shortName(it.Profile.Name)
			} else if len(profilesByID[b.ID]) > 0 {
				// 多账号整浏览器：副标题提示可选账户，左键点击直接展开账户菜单（右键亦可）。
				sub = i18n.Tf("picker.shortcut_profiles", i+1)
			}

			c := makePickerCard(browserIcon(b, 44), title, sub, pal, action)
			c.onHover = func(h bool) {
				if h {
					usingKeys = false // 鼠标介入后收起聚焦环，避免同时出现两套焦点提示
				}
				applySelection()
			}
			if it.Profile == nil {
				if profs := profilesByID[b.ID]; len(profs) > 0 {
					cc, bb := c, b
					c.onSecondary = func() { showProfileMenu(w, cc, bb, profs, rememberSw.IsOn(), openProfile) }
				}
			}
			cards = append(cards, c)
			cardActions = append(cardActions, action)
			objs = append(objs, tileShadow(radTile, c))
		}
		// 用 GridLayout（固定 cols 列、宽度均分）而不是 GridWrap：
		// GridWrap 的列数是 floor((宽+间距)/(格宽+间距))，边界上只要少 1px
		// 就会少放一整列 —— 最后一个磁贴被挤到下一行，而 rows 仍按原列数算作
		// 1 行，于是它溢出网格、盖在 footer 上。GridLayout 的列数是显式给定的，
		// 不存在这个临界问题（它过大的 MinSize 由外层 fixedSize 覆盖）。
		grid := container.New(layout.NewGridLayout(cols), objs...)

		// 磁贴数超过「列数 × 最大行数」时改为滚动。
		var gridObj fyne.CanvasObject = grid
		if visN > cols*pickerMaxRow {
			gridObj = container.NewVScroll(grid)
		}
		// fixedSize 钉住整体尺寸：外层 Border 才能给网格预留出正确高度。
		gridBox.Objects = []fyne.CanvasObject{fixedSize(gridObj, gridW, gridH)}
		gridBox.Refresh()

		if discShown {
			if expanded {
				discBtn.SetText(i18n.T("picker.show_less"))
				discBtn.SetIcon(theme.MenuDropUpIcon())
			} else {
				discBtn.SetText(i18n.Tf("picker.show_more", n-visN))
				discBtn.SetIcon(theme.MenuDropDownIcon())
			}
			discBox.Show()
		} else {
			discBox.Hide()
		}

		if selIdx >= len(cards) {
			selIdx = 0
		}
		applySelection()
	}

	// 高度直接取 root.MinSize()，不要自己逐段累加：VBox 会在子元素之间插入
	// theme.Padding()，手算必然漏掉这几点，窗口就比内容矮一截。
	//
	// 这个值是可靠的，前提是网格用 GridLayout —— 它的 MinSize 由子元素数量与列数
	// 直接算出，不依赖上一次布局。早先用 GridWrap 时不行：它的 MinSize 只读
	// Layout 缓存下来的 rowCount，renderGrid 刚换过内容就读到旧值。
	resize = func() {
		w.Resize(fyne.NewSize(winWidth(cols, bottomBox), root.MinSize().Height))
	}

	// ---- 组装窗口 ----
	copyBtn := toolbarButton(theme.ContentCopyIcon(), func() {
		a.Clipboard().SetContent(url)
	})
	gearBtn := toolbarButton(theme.SettingsIcon(), func() {
		OpenSettingsWindow(a, cfg, w)
	})
	footer := container.NewBorder(nil, nil, rememberRow, container.NewHBox(copyBtn, gearBtn))

	// padH 统一左右 20pt 内容边距（container.NewPadded 只有 4px，放不下 macOS 的边距）。
	padH := func(o fyne.CanvasObject) fyne.CanvasObject {
		return container.NewBorder(nil, nil, hSpace(sp20), hSpace(sp20), o)
	}

	bottomBox = padH(container.NewVBox(vSpace(sp8), footer, vSpace(sp4), countText, vSpace(sp8)))
	bottom := container.NewVBox(hairline(), bottomBox)
	progWrap = container.NewVBox(bottom, prog) // 进度条贴在窗口最底边、通栏

	topBox = padH(container.NewVBox(vSpace(sp16), header, vSpace(sp12)))
	gridArea := container.NewVBox(gridBox, vSpace(sp12), discBox)
	root = container.NewBorder(topBox, progWrap, nil, nil, padH(gridArea))

	renderGrid()

	return &pickerUI{
		root:            root,
		def:             def,
		remember:        rememberSw,
		setCountdown:    setCountdown,
		shortcutActions: shortcutActions,
		colCount:        func() int { return cols },
		activate: func() {
			if selIdx < len(cardActions) {
				cardActions[selIdx]()
			}
		},
		moveSelection: func(delta int) {
			if len(cards) == 0 {
				return
			}
			usingKeys = true
			selIdx = (selIdx + delta + len(cards)) % len(cards)
			applySelection()
		},
		resize: func() {
			renderGrid()
			resize()
		},
		expand: discBtn.OnTapped,
	}
}

// winWidth 计算窗口宽度。抽成函数是因为 renderGrid（算 cellW）与 resize（设窗口）
// 都要它：cellW 必须由最终窗口宽度反推，两者必须一致，否则网格会少放一列。
//
// 基准宽度由列数决定；底部文案（多语言下可能很长）更宽时以它为准；
// 上限放宽 80pt，宁可窗口略宽也不要把按钮挤出可视区。
func winWidth(cols int, bottomBox fyne.CanvasObject) float32 {
	w := pickerCellW*float32(cols) + pickerGap*float32(cols-1) + 2*sp20
	if w > pickerMaxW {
		w = pickerMaxW
	}
	if bottomBox != nil {
		if need := bottomBox.MinSize().Width; need > w {
			w = need
		}
	}
	if w > pickerMaxW+80 {
		w = pickerMaxW + 80
	}
	return w
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

// makePickerCard 生成一个浏览器磁贴：图标 + 名称 + 快捷键提示。
func makePickerCard(icon fyne.CanvasObject, title, shortcut string, pal cardPalette, onTap func()) *card {
	name := plainText(truncateTail(title, 11), fsBody, fgCol())
	name.Alignment = fyne.TextAlignCenter
	key := plainText(shortcut, fsCaption, secCol())
	key.Alignment = fyne.TextAlignCenter
	bg := &canvas.Rectangle{CornerRadius: radTile}
	inner := container.NewVBox(
		container.NewCenter(icon),
		vSpace(sp4),
		container.NewCenter(name),
		vSpace(sp2),
		container.NewCenter(key),
	)
	stack := container.NewStack(bg, container.NewPadded(inner))
	return newCard(stack, bg, pal, onTap)
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
