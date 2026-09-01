package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kklinan/browser-switch/i18n"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 本文件是离屏冒烟测试：用 Fyne 的 test 驱动把界面真正渲染并测量一遍，
// 覆盖自定义控件（磁贴 / 开关 / 侧边栏）的布局与刷新路径，
// 确保改动不会在构建期或刷新时 panic，且窗口尺寸始终落在合理区间。
// 不打开真实窗口、不改动系统状态。

const (
	smokeMaxW = 620.0 // pickerMaxW + 放宽量
	smokeMaxH = 700.0 // 三行磁贴 + 头尾的合理上限；超出说明布局失控
)

func smokePicker(t *testing.T, cfg *Config, favorites int) fyne.Size {
	t.Helper()

	sub := *cfg
	sub.Favorites = nil
	for i := 0; i < favorites && i < len(cfg.Browsers); i++ {
		sub.Favorites = append(sub.Favorites, cfg.Browsers[i].ID)
	}

	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("smoke")
	pu := buildPickerUI(a, w, "https://github.com/some/very/long/path/that/keeps/going", &sub,
		func(Browser, bool) {}, func(Browser, Profile, bool) {})
	w.SetContent(pu.root)

	// 倒计时、键盘导航、展开/收起、重排都要能反复调用而不出错
	pu.setCountdown(3)
	pu.setCountdown(1)
	pu.moveSelection(1)
	pu.moveSelection(-1)
	pu.resize()
	if w.Canvas().Capture() == nil {
		t.Fatal("选择器渲染失败")
	}

	size := w.Canvas().Size()
	if size.Width > smokeMaxW || size.Height > smokeMaxH || size.Width < 240 || size.Height < 200 {
		t.Fatalf("选择器尺寸异常: %v（收藏 %d 项）", size, favorites)
	}

	// 展开后再收起：窗口应当变高（或不变），且始终不越界
	pu.expand()
	pu.resize()
	expanded := w.Canvas().Size()
	if expanded.Width > smokeMaxW || expanded.Height > smokeMaxH {
		t.Fatalf("展开后尺寸异常: %v（收藏 %d 项）", expanded, favorites)
	}
	pu.expand()
	pu.resize()

	// 关键回归：gridArea 的 MinSize 必须与 GridWrap 实排后的高度一致，
	// 否则多出的那行会盖在 footer 之上。
	got := pu.root.MinSize().Height
	if got < pickerRowH*2 {
		t.Errorf("网格 MinSize %v 太小，应至少容纳 2 行（%.0f），"+
			"否则第 2 行会画到 footer 之上（截图里的「Quark 盖住倒计时」）", got, pickerRowH*2)
	}
	return size
}

// synthBrowsers 生成 n 个合成浏览器，用于覆盖「磁贴多到需要滚动」的极端布局。
// 不依赖真实环境里恰好装了多少浏览器。
func synthBrowsers(n int) []Browser {
	out := make([]Browser, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("com.test.browser%d", i)
		out = append(out, Browser{ID: id, Name: fmt.Sprintf("Browser %d", i), Exec: "/usr/bin/true"})
	}
	return out
}

func TestSmokePickerLayout(t *testing.T) {
	i18n.Init()
	cfg := DefaultConfig()
	cfg.Browsers = DetectBrowsers()

	// 真实环境的收藏数量：0（全部）/ 1 / 3 / 5
	for _, n := range []int{0, 1, 3, 5} {
		t.Logf("收藏 %d 项 → 窗口 %v", n, smokePicker(t, cfg, n))
	}

	// 合成大量浏览器：验证网格会转为滚动且窗口不会失控
	many := DefaultConfig()
	many.Browsers = synthBrowsers(24)
	many.Favorites = nil
	for _, n := range []int{0, 5, 24} {
		t.Logf("合成 %d 项（收藏 %d）→ 窗口 %v", len(many.Browsers), n, smokePicker(t, many, n))
	}
}

func TestSmokeSettingsPanes(t *testing.T) {
	i18n.Init()
	a := test.NewApp()
	defer a.Quit()

	cfg := DefaultConfig()
	cfg.Browsers = DetectBrowsers()
	w := a.NewWindow("smoke")

	// 三个面板各渲染一次（侧边栏切换 = 重建后选中对应面板）
	for idx := 0; idx < 3; idx++ {
		settingsSelected = idx
		w.SetContent(buildSettingsContent(a, cfg, w))
		w.Resize(fyne.NewSize(840, 600))
		if w.Canvas().Capture() == nil {
			t.Fatalf("设置面板 %d 渲染失败", idx)
		}
	}
	settingsSelected = 0
}

func TestSmokeInstaller(t *testing.T) {
	i18n.Init()
	a := test.NewApp()
	defer a.Quit()

	w := a.NewWindow("smoke")
	w.SetContent(BuildInstallerUI(a, w))
	w.Resize(fyne.NewSize(560, 700))
	if w.Canvas().Capture() == nil {
		t.Fatal("安装器渲染失败")
	}
}

// TestAddRuleDialogIsSingleton 「添加规则」必须同时只有一个：重复点击按钮时
// 应把已开的窗口提到前台，而不是叠出第二个（后开的会盖住前一个，用户以为内容丢了）。
func TestAddRuleDialogIsSingleton(t *testing.T) {
	i18n.Init()
	a := test.NewApp()
	defer a.Quit()

	cfg := DefaultConfig()
	cfg.Browsers = DetectBrowsers()

	parent := a.NewWindow("settings")
	base := len(a.Driver().AllWindows())

	showAddRuleDialog(a, cfg, nil, nil)
	after1 := len(a.Driver().AllWindows())
	if after1 != base+1 {
		t.Fatalf("首次点击应新开 1 个窗口：%d → %d", base, after1)
	}

	// 连点 3 次：不应再产生新窗口
	for i := 0; i < 3; i++ {
		showAddRuleDialog(a, cfg, nil, nil)
	}
	if got := len(a.Driver().AllWindows()); got != after1 {
		t.Errorf("重复点击又开了窗口：%d → %d（应保持不变）", after1, got)
	}

	// 关闭后应能再次打开（引用必须被清理，不能留下悬空窗口）
	if addRuleWin == nil {
		t.Fatal("addRuleWin 未记录窗口")
	}
	addRuleWin.Close()
	if addRuleWin != nil {
		t.Error("窗口关闭后 addRuleWin 未清空，此后按钮将失效")
	}
	showAddRuleDialog(a, cfg, nil, nil)
	if got := len(a.Driver().AllWindows()); got != after1 {
		t.Errorf("关闭后重新打开失败：%d → %d", after1, got)
	}
	addRuleWin.Close()
	_ = parent
}

// TestClosePickerDropsOtherWindows 关掉 picker 时，app 下其他窗口（设置 / 添加规则）
// 必须一并关掉 —— 否则用户关 picker 后设置窗口被孤立在屏幕上。
func TestClosePickerDropsOtherWindows(t *testing.T) {
	i18n.Init()
	a := test.NewApp()
	defer a.Quit()

	cfg := DefaultConfig()
	cfg.Browsers = DetectBrowsers()

	picker := a.NewWindow("picker")
	pu := buildPickerUI(a, picker, "https://example.com", cfg,
		func(Browser, bool) {}, func(Browser, Profile, bool) {})
	picker.SetContent(pu.root)
	pu.resize()

	// 模拟用户从齿轮打开设置窗口、又从设置打开添加规则窗口
	OpenSettingsWindow(a, cfg, picker)
	settingsWin := trackedWindows[picker][0]

	showAddRuleDialog(a, cfg, nil, settingsWin)
	if got := len(trackedWindows[settingsWin]); got != 1 {
		t.Fatalf("添加规则窗口应登记到 settingsWin，实际 %d 个", got)
	}

	// 模拟点 picker 关闭按钮：应清空 picker 自己登记的所有子窗口。
	// 断言用 trackedWindows 而非 AllWindows：测试驱动下 Close() 不一定真移除窗口，
	// 但 SetOnClosed 一定会被调用，trackedWindows 的清理路径是确定的。
	closeTrackedWindows(picker)
	if _, ok := trackedWindows[picker]; ok {
		t.Error("picker 关闭后 trackedWindows 应不再包含 picker")
	}
}
func TestSelectWidthIsFixed(t *testing.T) {
	i18n.Init()
	a := test.NewApp()
	defer a.Quit()

	sel := widget.NewSelect([]string{"en", "Français", "日本語", "Português"}, nil)
	sel.SetSelectedIndex(0)

	selMin := sel.MinSize().Width
	box := fixedSelect(sel, selectExtraChars)
	w := box.MinSize().Width

	if w <= selMin {
		t.Errorf("未加宽：%.0f 应大于原始宽度 %.0f", w, selMin)
	}

	// 换选中最长的选项：宽度必须纹丝不动
	sel.SetSelected("Português")
	if got := box.MinSize().Width; got != w {
		t.Errorf("切换选项后宽度变了：%.0f → %.0f（应固定为 %.0f）", w, got, w)
	}

	// 余量应当至少够放 selectExtraChars 个字符
	th := a.Settings().Theme()
	extra := fyne.MeasureText(strings.Repeat("0", selectExtraChars), th.Size(theme.SizeNameText), fyne.TextStyle{}).Width
	if got := w - selMin; got < extra {
		t.Errorf("余量不足：加了 %.0f，应至少 %.0f（%d 个字符）", got, extra, selectExtraChars)
	}
}

// ---- 网格与窗口尺寸的回归测试 ----
//
// 下面的断言全部读离屏布局后的真实坐标，不是估算值。曾经出过的三类问题
// ——磁贴被挤到第二行并盖住 footer、窗口比内容矮/窄导致裁切、开关被 HBox
// 纵向拉伸——都能被它们拦住。

func collectObjs(o fyne.CanvasObject, out *[]fyne.CanvasObject) {
	*out = append(*out, o)
	if c, ok := o.(*fyne.Container); ok {
		for _, ch := range c.Objects {
			collectObjs(ch, out)
		}
	}
}

// pickerGeometry 离屏搭一个含 n 个浏览器的选择器，返回：
// 每个磁贴的绝对纵坐标、开关尺寸、窗口尺寸、内容最小尺寸。
func pickerGeometry(t *testing.T, n int) (tileYs []float32, sw, win, rootMin fyne.Size) {
	t.Helper()
	a := test.NewApp()
	defer a.Quit()

	cfg := DefaultConfig()
	cfg.Browsers = synthBrowsers(n)
	cfg.Favorites = nil
	for i := 0; i < n; i++ {
		cfg.Favorites = append(cfg.Favorites, cfg.Browsers[i].ID)
	}
	cfg.AutoCloseDelay = 5

	w := a.NewWindow("smoke")
	pu := buildPickerUI(a, w, "https://example.com/x", cfg,
		func(Browser, bool) {}, func(Browser, Profile, bool) {})
	w.SetContent(pu.root)
	// 顺序必须和 setupPickerWindow 一致：先填倒计时文字再 resize，
	// 否则底部文案是空的，窗口会按更窄的宽度算好、随后被撑破。
	pu.setCountdown(4)
	pu.resize()

	var objs []fyne.CanvasObject
	collectObjs(pu.root, &objs)
	for _, o := range objs {
		switch v := o.(type) {
		case *card:
			tileYs = append(tileYs, fyne.CurrentApp().Driver().AbsolutePositionForObject(o).Y)
		case *appleSwitch:
			sw = v.Size()
		}
	}
	return tileYs, sw, w.Canvas().Size(), pu.root.MinSize()
}

// TestPickerTilesFitOneRow 磁贴必须整齐排成一行。列数由我们显式给定，
// 不该出现「最后一个被挤到第二行、还盖在 footer 上」的情况。
func TestPickerTilesFitOneRow(t *testing.T) {
	i18n.Init()
	for _, n := range []int{3, 4, 5} {
		ys, _, _, _ := pickerGeometry(t, n)
		rows := map[float32]int{}
		for _, y := range ys {
			rows[y]++
		}
		if len(rows) != 1 {
			t.Errorf("%d 个浏览器应排成 1 行，实际 %d 行（各行磁贴数 %v）", n, len(rows), rows)
		}
		if len(ys) != n {
			t.Errorf("应有 %d 个磁贴，实际渲染了 %d 个", n, len(ys))
		}
	}
}

// TestPickerWindowFitsContent 窗口尺寸必须正好容纳内容。
// 窗口小了会裁掉底部；窄了会把整行磁贴横向压扁（曾因漏算 Border 的 padding 窄 8px）。
func TestPickerWindowFitsContent(t *testing.T) {
	i18n.Init()
	for _, n := range []int{3, 5, 8} {
		_, _, win, rootMin := pickerGeometry(t, n)
		if rootMin.Width != win.Width || rootMin.Height != win.Height {
			t.Errorf("%d 个浏览器：窗口 %.0fx%.0f 与内容最小尺寸 %.0fx%.0f 不符，"+
				"内容会被裁切或压缩", n, win.Width, win.Height, rootMin.Width, rootMin.Height)
		}
	}
}

// TestSwitchNotStretched 开关必须保持 38×22：直接放进 HBox 会被纵向拉伸到行高，
// 轨道变成一颗臃肿的胶囊。
func TestSwitchNotStretched(t *testing.T) {
	i18n.Init()
	_, sw, _, _ := pickerGeometry(t, 3)
	want := fyne.NewSize(switchW, switchH)
	if sw != want {
		t.Errorf("开关尺寸应为 %.0fx%.0f，实际 %.0fx%.0f", want.Width, want.Height, sw.Width, sw.Height)
	}
}
