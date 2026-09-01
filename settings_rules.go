package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kklinan/browser-switch/i18n"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ruleFilterText 规则面板的搜索词，重建列表时保留，避免输入被清空。
var ruleFilterText = ""

// ---- 规则面板 ----

func buildRulesTab(a fyne.App, cfg *Config, parent fyne.Window) fyne.CanvasObject {
	// 搜索框与按钮构成稳定的外壳，列表单独放一个 Stack：
	// 每次输入都重建整个面板会让输入框失去焦点，故只重建列表部分。
	listHolder := container.NewStack()
	footer := plainText("", fsCaption, secCol())

	// 先声明再赋值：rebuildList 需要自引用（删除规则后就地重建列表）。
	var rebuildList func()
	rebuildList = func() {
		rows, matched, total := buildRuleList(cfg, func() { rebuildList() })
		listHolder.Objects = []fyne.CanvasObject{container.NewVScroll(rows)}
		listHolder.Refresh()
		footer.Text = i18n.Tf("settings.rules.count", matched, total)
		footer.Refresh()
	}

	// 搜索框：规则靠「记住选择」自动累积，量上来后没有过滤几乎不可用。
	search := widget.NewEntry()
	search.SetPlaceHolder(i18n.T("settings.rules.search"))
	search.SetText(ruleFilterText) // 先赋值再挂 OnChanged，避免初始化触发重建
	search.OnChanged = func(s string) {
		ruleFilterText = s
		rebuildList()
	}
	searchWrap := container.NewBorder(nil, nil, widget.NewIcon(theme.SearchIcon()), nil, search)

	addBtn := widget.NewButtonWithIcon(i18n.T("settings.rules.add_rule"), theme.ContentAddIcon(), func() {
		showAddRuleDialog(a, cfg, func() { rebuildList() }, parent)
	})

	rebuildList()

	return container.NewBorder(
		inset(container.NewBorder(nil, nil, nil, addBtn,
			container.New(layout.NewGridWrapLayout(fyne.NewSize(240, search.MinSize().Height)), searchWrap)), sp16),
		inset(footer, sp16),
		nil, nil,
		listHolder,
	)
}

// buildRuleList 构建规则列表，返回内容对象与「命中条数 / 总条数」。
// onChanged 由删除操作触发，用于就地重建列表。
func buildRuleList(cfg *Config, onChanged func()) (fyne.CanvasObject, int, int) {
	// 图标提取会读磁盘（.icns → PNG 缓存），规则可能上百条，按浏览器缓存避免重复开销。
	iconCache := map[string]fyne.CanvasObject{}
	browserIconFor := func(id string) fyne.CanvasObject {
		if o, ok := iconCache[id]; ok {
			return o
		}
		var o fyne.CanvasObject
		if b := findBrowserByID(id, cfg); b != nil {
			o = browserIcon(*b, 18)
		} else {
			o = widget.NewIcon(theme.QuestionIcon())
		}
		iconCache[id] = o
		return o
	}

	// 排序在副本上做，不改动 cfg.Rules 的原始顺序。
	sorted := append([]Rule(nil), cfg.Rules...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Priority > sorted[j].Priority })

	q := strings.ToLower(strings.TrimSpace(ruleFilterText))
	matched := sorted
	if q != "" {
		matched = nil
		for _, r := range sorted {
			name := ""
			if b := findBrowserByID(r.Browser, cfg); b != nil {
				name = strings.ToLower(shortName(b.Name))
			}
			if strings.Contains(strings.ToLower(r.Pattern), q) || strings.Contains(name, q) {
				matched = append(matched, r)
			}
		}
	}

	rows := container.NewVBox()
	if len(matched) == 0 {
		if len(cfg.Rules) == 0 {
			rows.Add(emptyState(i18n.T("settings.rules.empty_hint")))
		} else {
			rows.Add(emptyState(i18n.T("settings.rules.no_match")))
		}
		return rows, 0, len(cfg.Rules)
	}

	for _, r := range matched {
		rule := r
		name := rule.Browser
		if b := findBrowserByID(rule.Browser, cfg); b != nil {
			name = shortName(b.Name)
		}
		rowContent := container.NewHBox(
			pill(modeDisplayName(rule.Mode), accent().get()),
			hSpace(sp8),
			plainText(truncateTail(rule.Pattern, 26), fsBody, fgCol()),
			hSpace(sp8),
			plainText("→", fsBody, secCol()),
			hSpace(sp8),
			browserIconFor(rule.Browser),
			hSpace(sp4),
			plainText(truncateTail(name, 16), fsBody, fgCol()),
		)
		row := container.NewBorder(nil, nil, rowContent,
			iconButton(theme.DeleteIcon(), func() {
				cfg.Rules = removeRule(cfg.Rules, rule.ID)
				_ = SaveConfig(cfg)
				onChanged()
			}),
		)
		rows.Add(listRow(row, sp16))
	}
	return rows, len(matched), len(cfg.Rules)
}

// addRuleWin 当前打开的「添加规则」窗口（nil 表示未打开）。
// 只在 UI 线程访问（Fyne 的回调均在主线程），故无需加锁。
var addRuleWin fyne.Window

// showAddRuleDialog 显示添加规则对话框（复用已有 app，避免嵌套事件循环）。
// refresh 在成功新增规则后调用，用于重建规则列表使新规则立即可见。
// parent 是触发它的设置窗口，关闭时一并关掉（避免「关掉设置后对话框残留」）。
func showAddRuleDialog(a fyne.App, cfg *Config, refresh func(), parent fyne.Window) {
	// 同一时刻只允许一个「添加规则」窗口：重复点击按钮时把已打开的提到前台，
	// 而不是再叠一个相同的窗口 —— 后开的那个会盖住前一个，用户会以为前面填的内容丢了。
	if addRuleWin != nil {
		addRuleWin.RequestFocus()
		return
	}

	w := a.NewWindow(i18n.T("settings.add_rule.title"))
	if parent != nil {
		trackChild(parent, w)
	}
	addRuleWin = w
	// 所有关闭路径（提交成功 / 取消 / 点窗口关闭按钮）都会走到这里，
	// 统一在此清理，避免留下悬空引用导致按钮此后失效。
	w.SetOnClosed(func() { addRuleWin = nil })

	patternEntry := widget.NewEntry()
	patternEntry.SetPlaceHolder(i18n.T("settings.add_rule.pattern_placeholder"))

	modeNames := modeDisplayNames()
	modeSelect := widget.NewSelect(modeNames, nil)
	modeSelect.SetSelectedIndex(0)

	// 根据输入内容自动推测匹配模式
	patternEntry.OnChanged = func(s string) {
		if s == "" {
			return
		}
		modeSelect.SetSelected(modeDisplayName(SuggestMatchMode(s)))
	}

	browserSelect := widget.NewSelect(nil, nil)
	var browserID string
	for _, b := range cfg.Browsers {
		browserSelect.Options = append(browserSelect.Options, shortName(b.Name))
	}
	// 先设初始值再挂 OnChanged：SetSelected 会触发 OnChanged（见 CLAUDE.md 3.2）。
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

	priorityEntry := widget.NewEntry()
	priorityEntry.SetText("50")
	commentEntry := widget.NewEntry()
	commentEntry.SetPlaceHolder(i18n.T("settings.add_rule.comment_placeholder"))

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
				n, err := strconv.Atoi(ps)
				if err != nil || n < 0 {
					dialog.ShowError(fmt.Errorf(i18n.T("settings.add_rule.error.invalid_priority")), w)
					return
				}
				priority = n
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
		OnCancel:   func() { w.Close() },
		SubmitText: i18n.T("settings.add_rule.submit"),
		CancelText: i18n.T("common.cancel"),
	}

	w.SetContent(container.NewBorder(
		inset(boldText(i18n.T("settings.add_rule.title"), fsDisplay, fgCol()), sp20),
		nil, nil, nil,
		inset(form, sp20),
	))
	w.Resize(fyne.NewSize(520, 460))
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
