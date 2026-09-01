package main

import (
	"errors"
	"fmt"

	"github.com/kklinan/browser-switch/i18n"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// ---- 通用面板 ----

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

	// 自动打开倒计时：滑块比数字输入框更符合系统设置里「时长」的表达，
	// 也让取值范围一目了然（0 = 关闭）。
	delayText := func(n int) string {
		if n <= 0 {
			return i18n.T("settings.general.auto_close_off")
		}
		return i18n.Tf("settings.general.auto_close_value", n)
	}
	autoSlider := widget.NewSlider(0, 15)
	autoSlider.Step = 1
	autoSlider.Value = float64(cfg.AutoCloseDelay) // 先赋值再挂回调，避免初始化触发 OnChanged
	autoLabel := plainText(delayText(cfg.AutoCloseDelay), fsBody, secCol())
	autoSlider.OnChanged = func(v float64) {
		n := int(v)
		if n < 0 {
			n = 0
		}
		cfg.AutoCloseDelay = n
		_ = SaveConfig(cfg)
		autoLabel.Text = delayText(n)
		autoLabel.Refresh()
	}

	// 无匹配规则的网址（默认规则）：可弹选择器让用户选，或指定一个已有浏览器直接打开。
	// 选项 0 固定为"弹出选择器"，其后每个浏览器一项"使用 XX 打开"，与 names/idByIdx 顺序对齐。
	missOpts := make([]string, 0, len(names)+1)
	missOpts = append(missOpts, i18n.T("settings.general.on_miss.show_picker"))
	for _, n := range names {
		missOpts = append(missOpts, fmt.Sprintf(i18n.T("settings.general.on_miss.use_browser"), n))
	}
	missSel := widget.NewSelect(missOpts, nil)
	// 先设初始值，再挂 OnChanged：同 defSel/langSel。
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
		rebuild() // 切换语言后立即重建设置窗口所有文本
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
	setOtherDefaultBtn := widget.NewButton(i18n.T("settings.general.set_other_default"), func() {
		showSetOtherDefaultDialog(cfg, w)
	})
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
	uninstallBtn.Importance = widget.DangerImportance // 破坏性操作，用系统的红色按钮

	// ---- 组装：圆角分组 ----
	// 相关条目成组放进圆角卡片（macOS「系统设置」的做法），破坏性操作留在组外。
	// 下拉框统一钉成固定宽度（见 selectFixedSize）：既留出余量，
	// 也避免切换选项时同行的标签被推来推去。
	settings := groupBox(
		formRow(i18n.T("settings.general.language"), fixedSelect(langSel, selectExtraChars)),
		formRow(i18n.T("settings.general.default_browser"), fixedSelect(defSel, selectExtraChars)),
		formRow(i18n.T("settings.general.on_miss"), fixedSelect(missSel, selectExtraChars)),
	)

	// 倒计时单独成组：它是唯一一个「连续值」设置，与下拉选择语义不同。
	delayRow := container.NewBorder(nil, nil,
		widget.NewLabel(i18n.T("settings.general.auto_close")),
		container.NewHBox(
			container.New(layout.NewGridWrapLayout(fyne.NewSize(180, autoSlider.MinSize().Height)), autoSlider),
			hSpace(sp8),
			autoLabel,
		),
	)
	delayGroup := container.NewVBox(
		groupBox(delayRow),
		vSpace(sp2),
		container.NewBorder(nil, nil, hSpace(sp12), hSpace(sp12),
			plainText(i18n.T("settings.general.auto_close_hint"), fsCaption, secCol())),
	)

	actions := container.NewVBox(
		container.NewHBox(installBtn, setOtherDefaultBtn),
		vSpace(sp8),
		uninstallBtn,
	)

	body := container.NewVBox(
		settings,
		vSpace(sp20),
		delayGroup,
		vSpace(sp20),
		actions,
	)
	return container.NewVScroll(inset(body, sp20))
}

// showSetOtherDefaultDialog 将其他浏览器设为系统默认：列出已检测浏览器，
// 选中后直接写入系统默认 handler。
func showSetOtherDefaultDialog(cfg *Config, w fyne.Window) {
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
}
