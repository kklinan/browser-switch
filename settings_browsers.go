package main

import (
	"github.com/kklinan/browser-switch/i18n"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// browserExpanded 记录右侧列表中哪些多账号浏览器处于展开状态（跨 refresh 保持）。
var browserExpanded = map[string]bool{}

// ---- 浏览器面板：左收藏 / 右全部 ----

func buildBrowsersTab(cfg *Config, refresh func()) fyne.CanvasObject {
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

	// ---- 左：收藏（顺序 = 弹窗显示顺序与 ⌘ 编号）----
	favRows := container.NewVBox()
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
		up := iconButton(theme.MoveUpIcon(), func() { moveFav(keyc, -1) })
		down := iconButton(theme.MoveDownIcon(), func() { moveFav(keyc, +1) })
		rm := iconButton(theme.ContentRemoveIcon(), func() { toggleFav(keyc) })
		row := container.NewBorder(nil, nil,
			container.NewHBox(badgeNum(pos), hSpace(sp8), browserIcon(*b, browserIconSm), hSpace(sp8),
				plainText(truncateTail(label, 20), fsBody, fgCol())),
			container.NewHBox(up, down, rm),
		)
		favRows.Add(listRow(row, sp12))
	}
	if pos == 0 {
		favRows.Add(emptyState(i18n.T("settings.browsers.empty_fav_hint")))
	}
	leftPanel := container.NewBorder(
		inset(sectionHeader(i18n.T("settings.browsers.favorites"), i18n.T("settings.browsers.favorites_hint")), sp16),
		nil, nil, nil,
		container.NewVScroll(favRows),
	)

	// ---- 右：全部浏览器（展开账户 / 显示隐藏 / 收藏）----
	profilesByID := map[string][]Profile{}
	for i := range cfg.Browsers {
		if ps := DetectProfiles(cfg.Browsers[i]); len(ps) > 0 {
			profilesByID[cfg.Browsers[i].ID] = ps
		}
	}

	rightRows := container.NewVBox()
	for i := range cfg.Browsers {
		b := cfg.Browsers[i]
		idc := b.ID
		profs := profilesByID[b.ID]

		// 收藏：未收藏显示「+」，已收藏显示「✓」——与左侧列表的增删语义一致。
		favIcon := theme.ContentAddIcon()
		if isFav(b.ID) {
			favIcon = theme.ConfirmIcon()
		}
		favBtn := iconButton(favIcon, func() { toggleFav(idc) })

		hideIcon := theme.VisibilityOffIcon() // 眼睛 = 当前可见，点击隐藏
		if isHidden(b.ID) {
			hideIcon = theme.VisibilityIcon() // 划线眼 = 已隐藏，点击恢复
		}
		hideBtn := iconButton(hideIcon, func() { toggleHidden(idc) })

		var controls *fyne.Container
		if len(profs) > 0 {
			expIcon := theme.NavigateNextIcon() // › 收起态
			if browserExpanded[b.ID] {
				expIcon = theme.MenuDropDownIcon() // ▾ 展开态
			}
			expBtn := iconButton(expIcon, func() {
				browserExpanded[idc] = !browserExpanded[idc]
				refresh()
			})
			controls = container.NewHBox(expBtn, hideBtn, favBtn)
		} else {
			controls = container.NewHBox(hideBtn, favBtn)
		}

		name := plainText(shortName(b.Name), fsBody, fgCol())
		if isHidden(b.ID) {
			name = plainText(shortName(b.Name)+i18n.T("settings.browsers.hidden_suffix"), fsBody, secCol())
		}
		row := container.NewBorder(nil, nil,
			container.NewHBox(browserIcon(b, browserIconMd), hSpace(sp12), name),
			controls,
		)
		rightRows.Add(listRow(row, sp12))

		// 展开的多账号：在该浏览器行下方缩进列出各账户，每个账户可独立收藏。
		if len(profs) > 0 && browserExpanded[b.ID] {
			for _, p := range profs {
				label := p.Name
				if p.Kind == "incognito" {
					label = i18n.T("picker.incognito")
				}
				fkey := encodeFavKey(b.ID, p.ID)
				pFavIcon := theme.ContentAddIcon()
				if isFav(fkey) {
					pFavIcon = theme.ConfirmIcon()
				}
				pFav := iconButton(pFavIcon, func() { toggleFav(fkey) })
				sub := container.NewBorder(nil, nil,
					container.NewHBox(
						hSpace(sp24),
						widget.NewIcon(theme.AccountIcon()),
						hSpace(sp8),
						plainText(truncateTail(label, 20), fsSubhead, secCol()),
					),
					pFav,
				)
				rightRows.Add(listRow(sub, sp24))
			}
		}
	}

	// 重新扫描：点击后转圈（Activity），异步扫描完成后停止并刷新，给用户明确反馈。
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
				refresh() // 重建整个面板，activity 随之丢弃，无需手动恢复按钮
			})
		}()
	})
	rightHeader := container.NewBorder(nil, nil,
		boldText(i18n.T("settings.browsers.all_browsers"), fsTitle, fgCol()),
		container.NewHBox(scanActivity, rescanBtn))
	rightPanel := container.NewBorder(
		inset(rightHeader, sp16),
		nil, nil, nil,
		container.NewVScroll(rightRows),
	)

	split := container.NewHSplit(leftPanel, rightPanel)
	split.SetOffset(0.42)
	return split
}
