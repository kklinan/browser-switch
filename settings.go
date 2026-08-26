package main

import (
	"errors"
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"time"

	"github.com/kklinan/browser-switch/i18n"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ShowSettings 打开设置窗口（独立进程/独立 app）。
func ShowSettings(cfg *Config) {
	a := app.New()
	w := a.NewWindow(i18n.T("settings.window_title"))
	w.SetContent(buildSettingsContent(a, cfg, w))
	w.Resize(fyne.NewSize(760, 560))
	w.CenterOnScreen()
	w.ShowAndRun()
}

// OpenSettingsWindow 在已有 app 内开设置窗口（供选择器齿轮按钮复用，不另起 app）。
func OpenSettingsWindow(a fyne.App, cfg *Config) {
	w := a.NewWindow(i18n.T("settings.window_title"))
	w.SetContent(buildSettingsContent(a, cfg, w))
	w.Resize(fyne.NewSize(760, 560))
	w.CenterOnScreen()
	w.Show()
}

// buildSettingsContent 构建设置窗口的标签页内容（不驱动主循环）。
func buildSettingsContent(a fyne.App, cfg *Config, w fyne.Window) fyne.CanvasObject {
	var browsersTab *container.TabItem
	var rulesTab *container.TabItem
	var rebuildBrowsers func()
	var rebuildRules func()
	var tabs *container.AppTabs

	// rebuildAll 切换语言后重建全部标签页内容，保留当前选中的标签页索引，
	// 避免语言切换后跳回第一个标签页（浏览器页）。
	rebuildAll := func() {
		idx := 0
		if tabs != nil {
			idx = tabs.SelectedIndex()
		}
		newContent := buildSettingsContent(a, cfg, w)
		if nt, ok := newContent.(*container.AppTabs); ok && idx >= 0 && idx < len(nt.Items) {
			nt.SelectIndex(idx)
		}
		w.SetContent(newContent)
	}

	tabs = container.NewAppTabs()
	// 用稳定的外层容器包裹各 tab 内容：刷新时替换容器内部对象并 Refresh，
	// 而非替换 TabItem.Content —— 后者在 Fyne 2.7 对当前正在显示的 tab 不生效，
	// 导致"重新扫描"等操作看似无反应。
	browsersHolder := container.NewStack()
	rulesHolder := container.NewStack()
	browsersTab = container.NewTabItem(i18n.T("settings.tab.browsers"), browsersHolder)
	rulesTab = container.NewTabItem(i18n.T("settings.tab.rules"), rulesHolder)
	tabs.Append(browsersTab)
	tabs.Append(rulesTab)
	tabs.Append(container.NewTabItem(i18n.T("settings.tab.general"), buildGeneralTab(cfg, w, rebuildAll)))

	rebuildBrowsers = func() {
		browsersHolder.Objects = []fyne.CanvasObject{buildBrowsersTab(cfg, func() { rebuildBrowsers() })}
		browsersHolder.Refresh()
	}
	rebuildRules = func() {
		rulesHolder.Objects = []fyne.CanvasObject{buildRulesTab(a, cfg, func() { rebuildRules() })}
		rulesHolder.Refresh()
	}
	rebuildBrowsers()
	rebuildRules()
	return tabs
}

// ---- 浏览器标签：左收藏 / 右全部 ----

// browserExpanded 记录右侧列表中哪些多账号浏览器处于展开状态（跨 refresh 保持）。
var browserExpanded = map[string]bool{}

func buildBrowsersTab(cfg *Config, refresh func()) fyne.CanvasObject {
	fg := tcol(theme.ColorNameForeground)
	subtle := tcol(theme.ColorNamePlaceHolder)

	favPos := map[string]int{}
	for i, id := range cfg.Favorites {
		favPos[id] = i
	}
	isFav := func(id string) bool { _, ok := favPos[id]; return ok }
	isHidden := func(id string) bool {
		for _, h := range cfg.Hidden {
			if h == id {
				return true
			}
		}
		return false
	}

	toggleFav := func(id string) {
		if isFav(id) {
			out := cfg.Favorites[:0]
			for _, f := range cfg.Favorites {
				if f != id {
					out = append(out, f)
				}
			}
			cfg.Favorites = out
		} else {
			cfg.Favorites = append(cfg.Favorites, id)
		}
		_ = SaveConfig(cfg)
		refresh()
	}
	moveFav := func(id string, delta int) {
		i := favPos[id]
		j := i + delta
		if j < 0 || j >= len(cfg.Favorites) {
			return
		}
		cfg.Favorites[i], cfg.Favorites[j] = cfg.Favorites[j], cfg.Favorites[i]
		_ = SaveConfig(cfg)
		refresh()
	}
	toggleHidden := func(id string) {
		if isHidden(id) {
			out := cfg.Hidden[:0]
			for _, h := range cfg.Hidden {
				if h != id {
					out = append(out, h)
				}
			}
			cfg.Hidden = out
		} else {
			cfg.Hidden = append(cfg.Hidden, id)
		}
		_ = SaveConfig(cfg)
		refresh()
	}

	// 左：收藏（带上下移/移除，决定弹窗顺序与 ⌘ 编号）
	favBox := container.NewVBox(
		boldText(i18n.T("settings.browsers.favorites"), 14, fg),
		plainText(i18n.T("settings.browsers.favorites_hint"), 12, subtle),
		vSpace(4),
	)
	// 收藏项可能是整浏览器（key=bundleID）或具体账户（key=bundleID#profileID）。
	// 编号按"有效显示位"递增，与选择器 ⌘N 对齐（两侧都跳过悬空项）。
	pos := 0
	for _, key := range cfg.Favorites {
		bid, pid := decodeFavKey(key)
		b := findBrowserByID(bid, cfg)
		if b == nil {
			continue // 悬空：浏览器已卸载
		}
		label := shortName(b.Name)
		if pid != "" {
			p := findProfileByID(*b, pid)
			if p == nil {
				continue // 悬空：账户已删除
			}
			pname := p.Name
			if p.Kind == "incognito" {
				pname = i18n.T("picker.incognito")
			}
			label = shortName(b.Name) + " · " + pname
		}
		pos++
		keyc := key
		badge := badgeNum(pos)
		up := iconButton(theme.MoveUpIcon(), func() { moveFav(keyc, -1) })
		down := iconButton(theme.MoveDownIcon(), func() { moveFav(keyc, +1) })
		rm := iconButton(theme.CancelIcon(), func() { toggleFav(keyc) })
		row := container.NewBorder(nil, nil,
			container.NewHBox(badge, hSpace(4), browserIcon(*b, 22), hSpace(6), plainText(label, 13, fg)),
			container.NewHBox(up, down, rm),
		)
		favBox.Add(rowCard(row))
	}
	if len(cfg.Favorites) == 0 {
		favBox.Add(dropHint(i18n.T("settings.browsers.empty_fav_hint"), subtle))
	}

	// 预取各浏览器的多账号配置，供展开显示。
	profilesByID := map[string][]Profile{}
	for i := range cfg.Browsers {
		if ps := DetectProfiles(cfg.Browsers[i]); len(ps) > 0 {
			profilesByID[cfg.Browsers[i].ID] = ps
		}
	}

	// 右：全部浏览器（展开账户 / 👁 隐藏 / ♥ 收藏）
	rightRows := container.NewVBox()
	for i := range cfg.Browsers {
		b := cfg.Browsers[i]
		idc := b.ID
		profs := profilesByID[b.ID]

		heart := widget.NewButton(ternary(isFav(b.ID), i18n.T("settings.browsers.favorited"), i18n.T("settings.browsers.unfavorited")), func() { toggleFav(idc) })
		heart.Importance = widget.LowImportance

		// 隐藏：文字改为图标（眼睛=可见可点隐藏 / 划线眼睛=已隐藏可点显示）。
		hideIcon := theme.VisibilityOffIcon()
		if isHidden(b.ID) {
			hideIcon = theme.VisibilityIcon()
		}
		hideBtn := iconButton(hideIcon, func() { toggleHidden(idc) })

		// 右侧控件组：多账号时在隐藏左侧加展开/收起图标。
		var controls *fyne.Container
		if len(profs) > 0 {
			expIcon := theme.NavigateNextIcon() // › 收起态
			if browserExpanded[b.ID] {
				expIcon = theme.MenuDropDownIcon() // ▼ 展开态
			}
			expBtn := iconButton(expIcon, func() {
				browserExpanded[idc] = !browserExpanded[idc]
				refresh()
			})
			controls = container.NewHBox(expBtn, hideBtn, heart)
		} else {
			controls = container.NewHBox(hideBtn, heart)
		}

		name := plainText(shortName(b.Name), 13, fg)
		if isHidden(b.ID) {
			name = plainText(shortName(b.Name)+i18n.T("settings.browsers.hidden_suffix"), 13, subtle)
		}
		row := container.NewBorder(nil, nil,
			container.NewHBox(hSpace(2), browserIcon(b, 26), hSpace(10), name),
			controls,
		)
		rightRows.Add(container.NewPadded(row))

		// 展开的多账号：在该浏览器行下方缩进列出各账户，每个账户可独立收藏。
		if len(profs) > 0 && browserExpanded[b.ID] {
			for _, p := range profs {
				label := p.Name
				if p.Kind == "incognito" {
					label = i18n.T("picker.incognito")
				}
				fkey := encodeFavKey(b.ID, p.ID)
				pHeart := widget.NewButton(ternary(isFav(fkey), i18n.T("settings.browsers.favorited"), i18n.T("settings.browsers.unfavorited")), func() { toggleFav(fkey) })
				pHeart.Importance = widget.LowImportance
				sub := container.NewBorder(nil, nil,
					container.NewHBox(
						hSpace(38),
						widget.NewIcon(theme.AccountIcon()),
						hSpace(6),
						plainText(label, 12, subtle),
					),
					pHeart,
				)
				rightRows.Add(container.NewPadded(sub))
			}
		}
		rightRows.Add(thinSepObj(fg))
	}

	// 重新扫描：点击后左侧转圈（Activity），异步扫描完成后停止并刷新，给用户明确反馈。
	scanActivity := widget.NewActivity()
	scanActivity.Hide()
	var rescanBtn *widget.Button
	scanning := false
	rescanBtn = widget.NewButtonWithIcon(i18n.T("settings.browsers.rescan"), theme.ViewRefreshIcon(), func() {
		if scanning {
			return
		}
		scanning = true
		rescanBtn.Disable()
		scanActivity.Show()
		scanActivity.Start()
		go func() {
			merged := mergeDetected(cfg) // 慢操作（plutil 扫描）放后台，避免卡 UI
			fyne.Do(func() {
				cfg.Browsers = merged
				_ = SaveConfig(cfg)
				scanActivity.Stop()
				refresh() // 重建整个 tab，activity 随之丢弃，无需手动恢复按钮
			})
		}()
	})
	rightHeader := container.NewBorder(nil, nil, boldText(i18n.T("settings.browsers.all_browsers"), 14, fg),
		container.NewHBox(scanActivity, rescanBtn))
	rightPanel := container.NewBorder(container.NewPadded(rightHeader), nil, nil, nil,
		container.NewVScroll(rightRows))

	split := container.NewHSplit(container.NewPadded(favBox), rightPanel)
	split.SetOffset(0.42)
	return split
}

// ---- 规则标签（完整规则编辑器）----

func buildRulesTab(a fyne.App, cfg *Config, refresh func()) fyne.CanvasObject {
	fg := tcol(theme.ColorNameForeground)
	subtle := tcol(theme.ColorNamePlaceHolder)

	// 新建按钮
	addBtn := widget.NewButtonWithIcon(i18n.T("settings.rules.add_rule"), theme.ContentAddIcon(), func() {
		showAddRuleDialog(a, cfg, refresh)
	})

	// 规则列表
	rows := container.NewVBox()
	if len(cfg.Rules) == 0 {
		rows.Add(container.NewPadded(dropHint(i18n.T("settings.rules.empty_hint"), subtle)))
	}

	sorted := append([]Rule(nil), cfg.Rules...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Priority > sorted[j].Priority })
	for i := range sorted {
		r := sorted[i]
		name := r.Browser
		if b := findBrowserByID(r.Browser, cfg); b != nil {
			name = shortName(b.Name)
		}

		modeText := modeDisplayName(r.Mode)

		// 规则行：图标 | 模式 | 模式值 | → | 浏览器 | 操作
		rowContent := container.NewHBox(
			boldText(modeText, 12, accent),
			hSpace(4),
			plainText(r.Pattern, 13, fg),
			hSpace(10),
			plainText("→", 16, subtle),
			hSpace(4),
			plainText(name, 13, fg),
		)

		row := container.NewBorder(nil, nil,
			container.NewPadded(rowContent),
			container.NewHBox(
				iconButton(theme.CancelIcon(), func() {
					cfg.Rules = removeRule(cfg.Rules, r.ID)
					_ = SaveConfig(cfg)
					refresh()
				}),
			),
		)

		rows.Add(container.NewPadded(row))
		if i < len(sorted)-1 {
			rows.Add(thinSepObj(fg))
		}
	}

	scroll := container.NewVScroll(rows)

	return container.NewBorder(
		container.NewBorder(nil, nil, addBtn, nil),
		nil, nil, nil,
		container.NewPadded(scroll),
	)
}

// showAddRuleDialog 显示添加规则对话框（复用已有 app，避免嵌套事件循环）。
// refresh 在成功新增规则后调用，用于重建规则列表使新规则立即可见。
func showAddRuleDialog(a fyne.App, cfg *Config, refresh func()) {
	w := a.NewWindow(i18n.T("settings.add_rule.title"))

	// 匹配内容输入
	patternEntry := widget.NewEntry()
	patternEntry.SetPlaceHolder(i18n.T("settings.add_rule.pattern_placeholder"))

	// 匹配模式选择
	modeNames := modeDisplayNames()
	modeSelect := widget.NewSelect(modeNames, nil)
	modeSelect.SetSelectedIndex(0)

	// 根据输入内容自动推测匹配模式
	patternEntry.OnChanged = func(s string) {
		if s == "" {
			return
		}
		suggested := SuggestMatchMode(s)
		modeSelect.SetSelected(modeDisplayName(suggested))
	}

	// 浏览器选择
	browserSelect := widget.NewSelect(nil, nil)
	var browserID string
	for _, b := range cfg.Browsers {
		browserSelect.Options = append(browserSelect.Options, shortName(b.Name))
	}
	if cfg.DefaultBrowser != "" {
		for _, b := range cfg.Browsers {
			if b.ID == cfg.DefaultBrowser {
				browserSelect.SetSelected(shortName(b.Name))
				browserID = b.ID
				break
			}
		}
	}
	browserSelect.OnChanged = func(s string) {
		for _, b := range cfg.Browsers {
			if shortName(b.Name) == s {
				browserID = b.ID
				return
			}
		}
	}

	// 优先级
	priorityEntry := widget.NewEntry()
	priorityEntry.SetText("50")

	// 备注
	commentEntry := widget.NewEntry()
	commentEntry.SetPlaceHolder(i18n.T("settings.add_rule.comment_placeholder"))

	// 构建表单
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: i18n.T("settings.add_rule.field.pattern"), Widget: patternEntry, HintText: i18n.T("settings.add_rule.hint.pattern")},
			{Text: i18n.T("settings.add_rule.field.mode"), Widget: modeSelect, HintText: i18n.T("settings.add_rule.hint.mode")},
			{Text: i18n.T("settings.add_rule.field.browser"), Widget: browserSelect, HintText: i18n.T("settings.add_rule.hint.browser")},
			{Text: i18n.T("settings.add_rule.field.priority"), Widget: priorityEntry, HintText: i18n.T("settings.add_rule.hint.priority")},
			{Text: i18n.T("settings.add_rule.field.comment"), Widget: commentEntry, HintText: i18n.T("settings.add_rule.hint.comment")},
		},
		OnSubmit: func() {
			pattern := patternEntry.Text
			if pattern == "" {
				dialog.ShowError(fmt.Errorf(i18n.T("settings.add_rule.error.empty_pattern")), w)
				return
			}

			mode := modeFromDisplayName(modeSelect.Selected)

			// 正则模式额外校验语法
			if mode == MatchRegex {
				if err := ValidatePattern(pattern, mode); err != nil {
					dialog.ShowError(fmt.Errorf(i18n.T("settings.add_rule.error.invalid_regex"), err), w)
					return
				}
			}

			if browserID == "" {
				dialog.ShowError(fmt.Errorf(i18n.T("settings.add_rule.error.no_browser")), w)
				return
			}

			priority := 50
			if ps := priorityEntry.Text; ps != "" {
				if n, err := strconv.Atoi(ps); err == nil && n >= 0 {
					priority = n
				} else {
					dialog.ShowError(fmt.Errorf(i18n.T("settings.add_rule.error.invalid_priority")), w)
					return
				}
			}

			rule := Rule{
				ID:       fmt.Sprintf("user_%s_%d", pattern, time.Now().UnixNano()),
				Pattern:  pattern,
				Mode:     mode,
				Browser:  browserID,
				Priority: priority,
				Enabled:  true,
				Comment:  commentEntry.Text,
			}

			cfg.Rules = append(cfg.Rules, rule)
			_ = SaveConfig(cfg)
			if refresh != nil {
				refresh()
			}
			w.Close()
		},
		OnCancel: func() {
			w.Close()
		},
		SubmitText: i18n.T("settings.add_rule.submit"),
		CancelText: i18n.T("common.cancel"),
	}

	w.SetContent(container.NewPadded(form))
	w.Resize(fyne.NewSize(520, 380))
	w.CenterOnScreen()
	w.Show()
}

// removeRule 从规则列表中移除指定 ID 的规则
func removeRule(rules []Rule, id string) []Rule {
	result := make([]Rule, 0, len(rules)-1)
	for _, r := range rules {
		if r.ID != id {
			result = append(result, r)
		}
	}
	return result
}

// ---- 设置标签 ----

func buildGeneralTab(cfg *Config, w fyne.Window, rebuild func()) fyne.CanvasObject {
	names := []string{}
	idByIdx := []string{}
	for _, b := range cfg.Browsers {
		names = append(names, shortName(b.Name))
		idByIdx = append(idByIdx, b.ID)
	}
	defSel := widget.NewSelect(names, nil)
	// 先设初始值，再挂 OnChanged：SetSelectedIndex 会触发 OnChanged，同 langSel/missSel。
	if i := indexOf(idByIdx, cfg.DefaultBrowser); i >= 0 {
		defSel.SetSelectedIndex(i)
	}
	defSel.OnChanged = func(s string) {
		if i := indexOf(names, s); i >= 0 {
			cfg.DefaultBrowser = idByIdx[i]
			_ = SaveConfig(cfg)
		}
	}

	autoEntry := widget.NewEntry()
	autoEntry.SetText(strconv.Itoa(cfg.AutoCloseDelay))
	autoEntry.OnChanged = func(s string) {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			cfg.AutoCloseDelay = n
			_ = SaveConfig(cfg)
		} else if s != "" {
			// 非法输入（非数字或负数），回退到已有配置值，保持 UI 与配置一致
			autoEntry.SetText(strconv.Itoa(cfg.AutoCloseDelay))
		}
	}

	// 无匹配规则的网址（默认规则）：可弹选择器让用户选，或指定一个已有浏览器直接打开。
	// 选项 0 固定为"弹出选择器"，其后每个浏览器一项"使用 XX 打开"，与 names/idByIdx 顺序对齐。
	missOpts := make([]string, 0, len(names)+1)
	missOpts = append(missOpts, i18n.T("settings.general.on_miss.show_picker"))
	for _, n := range names {
		missOpts = append(missOpts, fmt.Sprintf(i18n.T("settings.general.on_miss.use_browser"), n))
	}
	missSel := widget.NewSelect(missOpts, nil)
	// 先设初始值，再挂 OnChanged：SetSelectedIndex 会触发 OnChanged，同 defSel/langSel。
	missInit := 0 // 默认"弹出选择器"
	if !cfg.ShowPickerOnMiss {
		if j := indexOf(idByIdx, cfg.DefaultBrowser); j >= 0 {
			missInit = j + 1 // +1 跳过第 0 项"弹出选择器"
		}
	}
	missSel.SetSelectedIndex(missInit)
	missSel.OnChanged = func(string) {
		idx := missSel.SelectedIndex()
		if idx <= 0 {
			cfg.ShowPickerOnMiss = true
		} else {
			// 指定具体浏览器：无匹配时直接用它打开，并同步"默认/回退浏览器"下拉保持一致。
			cfg.ShowPickerOnMiss = false
			cfg.DefaultBrowser = idByIdx[idx-1]
			defSel.SetSelectedIndex(idx - 1)
		}
		_ = SaveConfig(cfg)
	}

	// 语言选择器
	langOpts := []string{i18n.T("settings.general.language_auto")}
	for _, l := range i18n.SupportedLanguages() {
		langOpts = append(langOpts, i18n.LanguageNativeName(l))
	}
	langSel := widget.NewSelect(langOpts, nil)
	// 先设初始值，再挂 OnChanged：Fyne 的 SetSelected/SetSelectedIndex 会触发 OnChanged，
	// 若构造时就绑定 rebuild()，初始化选中即会递归重建设置页 → 无限递归、内存暴涨、Dock 卡死。
	if cfg.Language != "" {
		langSel.SetSelected(i18n.LanguageNativeName(cfg.Language))
	} else {
		langSel.SetSelectedIndex(0)
	}
	langSel.OnChanged = func(s string) {
		langCode := ""
		if s != i18n.T("settings.general.language_auto") {
			for _, l := range i18n.SupportedLanguages() {
				if i18n.LanguageNativeName(l) == s {
					langCode = l
					break
				}
			}
		}
		cfg.Language = langCode
		_ = SaveConfig(cfg)
		i18n.SetLanguage(langCode)
		// 切换语言后立即重建设置窗口所有文本
		rebuild()
	}

	installBtn := widget.NewButton(i18n.T("settings.general.set_default"), func() {
		if err := Install(); err != nil {
			dialog.ShowError(err, w)
			return
		}
		if ok, cur := CheckDefaultBrowser(); ok {
			dialog.ShowInformation(i18n.T("common.done"), i18n.T("settings.general.set_default_success"), w)
		} else {
			dialog.ShowInformation(i18n.T("common.installed"), i18n.T("settings.general.install_confirm_msg")+cur, w)
		}
	})
	installBtn.Importance = widget.HighImportance
	uninstallBtn := widget.NewButton(i18n.T("settings.general.uninstall"), func() {
		dialog.ShowConfirm(i18n.T("settings.general.uninstall_confirm_title"), i18n.T("settings.general.uninstall_confirm_msg"), func(ok bool) {
			if !ok {
				return
			}
			// 先卸载（还原默认浏览器 + 删除 .app，其内部会 SaveConfig 写回配置），
			// 再清除配置数据 —— 顺序不可颠倒，否则删掉的配置会被 Uninstall 重新写回。
			_ = Uninstall()
			_ = RemoveConfig()
			// 数据已清除：提示后关闭设置窗口结束会话，避免 UI 侧后续 SaveConfig 重建残留配置。
			info := dialog.NewInformation(i18n.T("common.done"), i18n.T("settings.general.uninstalled"), w)
			info.SetOnClosed(func() { w.Close() })
			info.Show()
		}, w)
	})

	// 将其他浏览器设为系统默认：列出所有已检测浏览器，选中后直接写入系统默认 handler。
	setOtherDefaultBtn := widget.NewButton(i18n.T("settings.general.set_other_default"), func() {
		if len(cfg.Browsers) == 0 {
			dialog.ShowInformation(i18n.T("common.tip"), i18n.T("settings.general.no_browser_to_set"), w)
			return
		}
		names := make([]string, len(cfg.Browsers))
		for i, b := range cfg.Browsers {
			names[i] = shortName(b.Name)
		}
		sel := widget.NewSelect(names, nil)
		sel.SetSelectedIndex(0)
		dialog.ShowCustomConfirm(
			i18n.T("settings.general.set_other_default"),
			i18n.T("common.confirm"), i18n.T("common.cancel"),
			container.NewVBox(widget.NewLabel(i18n.T("settings.general.pick_default_browser")), sel),
			func(ok bool) {
				if !ok {
					return
				}
				idx := indexOf(names, sel.Selected)
				if idx < 0 {
					return
				}
				bundleID := cfg.Browsers[idx].BundleID()
				if err := SetSystemDefaultBrowser(bundleID); err != nil {
					if errors.Is(err, ErrDefaultChangeNeedsApproval) {
						// macOS 已弹出系统确认窗口：引导用户在其中批准，而非抛出技术错误码。
						dialog.ShowInformation(i18n.T("common.tip"),
							fmt.Sprintf(i18n.T("settings.general.set_other_default_needs_approval"), sel.Selected), w)
						return
					}
					dialog.ShowError(err, w)
					return
				}
				dialog.ShowInformation(i18n.T("common.done"),
					fmt.Sprintf(i18n.T("settings.general.set_other_default_success"), sel.Selected), w)
			}, w)
	})

	// missSel 行右缘对齐下方按钮行：按钮行宽度随按钮文案（多语言）动态变化，
	// 故按其实际 MinSize 反推 missSel 宽度，避免硬编码在不同语言下右缘错位。
	btnRow := container.NewHBox(installBtn, setOtherDefaultBtn, uninstallBtn)
	pad := theme.Padding()
	btnRowW := installBtn.MinSize().Width + setOtherDefaultBtn.MinSize().Width + uninstallBtn.MinSize().Width + 2*pad
	missLabel := widget.NewLabel(i18n.T("settings.general.on_miss"))
	// HBox(label, missSel) 右缘 = labelW + pad + missSelW，令其等于 btnRowW 反解宽度。
	missSelW := btnRowW - missLabel.MinSize().Width - pad
	if mw := missSel.MinSize().Width; missSelW < mw {
		missSelW = mw // 按钮行过窄放不下最长选项时，至少保证完整显示
	}
	missRow := container.NewHBox(missLabel,
		container.New(layout.NewGridWrapLayout(fyne.NewSize(missSelW, missSel.MinSize().Height)), missSel))

	form := container.NewVBox(
		boldText(i18n.T("settings.general.title"), 14, tcol(theme.ColorNameForeground)),
		vSpace(6),
		container.NewHBox(widget.NewLabel(i18n.T("settings.general.language")), langSel),
		container.NewHBox(widget.NewLabel(i18n.T("settings.general.default_browser")), defSel),
		container.NewHBox(widget.NewLabel(i18n.T("settings.general.auto_close")), container.New(layout.NewGridWrapLayout(fyne.NewSize(60, 36)), autoEntry)),
		missRow,
		vSpace(10),
		// 三个操作按钮同一行，顺序：设为默认浏览器 · 设置其他浏览器为默认 · 卸载并清除数据。
		btnRow,
	)
	return container.NewPadded(form)
}

// ---- 小工具 ----

func rowCard(content fyne.CanvasObject) fyne.CanvasObject { return content }

func badgeNum(n int) fyne.CanvasObject {
	bg := &canvas.Rectangle{FillColor: accent, CornerRadius: 9}
	bg.SetMinSize(fyne.NewSize(18, 18))
	t := plainText(strconv.Itoa(n), 11, color.White)
	t.Alignment = fyne.TextAlignCenter
	t.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewStack(bg, container.NewCenter(t))
}

func dropHint(text string, subtle color.Color) fyne.CanvasObject {
	return plainText("＋ "+text, 12, subtle)
}

func iconButton(res fyne.Resource, fn func()) *widget.Button {
	b := widget.NewButtonWithIcon("", res, fn)
	b.Importance = widget.LowImportance
	return b
}

func thinSepObj(fg color.Color) fyne.CanvasObject {
	r := canvas.NewRectangle(withAlpha(fg, 0x0c))
	r.SetMinSize(fyne.NewSize(0, 1))
	return r
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

func ternary(b bool, a, c string) string {
	if b {
		return a
	}
	return c
}
func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

// ---- i18n 辅助：匹配模式名称的翻译与反向查找 ----

// modeDisplayNames 返回当前语言下所有匹配模式的展示名列表。
func modeDisplayNames() []string {
	return []string{
		i18n.T("settings.rules.mode.exact"),
		i18n.T("settings.rules.mode.wildcard"),
		i18n.T("settings.rules.mode.regex"),
		i18n.T("settings.rules.mode.contains"),
		i18n.T("settings.rules.mode.prefix"),
		i18n.T("settings.rules.mode.suffix"),
	}
}

// modeDisplayName 返回给定 MatchMode 的本地化展示名。
func modeDisplayName(m MatchMode) string {
	switch m {
	case MatchWildcard:
		return i18n.T("settings.rules.mode.wildcard")
	case MatchRegex:
		return i18n.T("settings.rules.mode.regex")
	case MatchContains:
		return i18n.T("settings.rules.mode.contains")
	case MatchPrefix:
		return i18n.T("settings.rules.mode.prefix")
	case MatchSuffix:
		return i18n.T("settings.rules.mode.suffix")
	default:
		return i18n.T("settings.rules.mode.exact")
	}
}

// modeFromDisplayName 根据展示名反查 MatchMode 常量。
func modeFromDisplayName(display string) MatchMode {
	switch display {
	case i18n.T("settings.rules.mode.wildcard"):
		return MatchWildcard
	case i18n.T("settings.rules.mode.regex"):
		return MatchRegex
	case i18n.T("settings.rules.mode.contains"):
		return MatchContains
	case i18n.T("settings.rules.mode.prefix"):
		return MatchPrefix
	case i18n.T("settings.rules.mode.suffix"):
		return MatchSuffix
	default:
		return MatchExact
	}
}
