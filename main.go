package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

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

func init() {
	// macOS 上 GUI 工具包要求运行在主线程；锁定主 goroutine 到其 OS 线程。
	runtime.LockOSThread()
}

func main() {
	// 初始化国际化语言包（必须在任何 i18n.T() 调用之前）
	i18n.Init()

	var (
		installFlag   bool
		uninstallFlag bool
		settingsFlag  bool
		testURL       string
		listBrowsers  bool
		listProfiles  bool
		checkDefault  bool
		versionFlag   bool
		installerFlag bool
		buildBundle   string
	)

	flag.BoolVar(&installFlag, "install", false, "")
	flag.BoolVar(&uninstallFlag, "uninstall", false, "")
	flag.BoolVar(&settingsFlag, "settings", false, "")
	flag.StringVar(&testURL, "test", "", "")
	flag.BoolVar(&listBrowsers, "list-browsers", false, "")
	flag.BoolVar(&listProfiles, "list-profiles", false, "")
	flag.BoolVar(&checkDefault, "check-default", false, "")
	flag.BoolVar(&versionFlag, "version", false, "")
	flag.BoolVar(&installerFlag, "installer", false, "")
	flag.StringVar(&buildBundle, "build-bundle", "", "")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, i18n.T("cli.usage.title"))
		fmt.Fprint(os.Stderr, i18n.T("cli.usage.usage"))
		fmt.Fprint(os.Stderr, i18n.T("cli.usage.desc"))
		// 设置标志描述文本（延迟到 i18n 初始化后）
		installSet := flag.Lookup("install")
		if installSet != nil {
			installSet.Usage = i18n.T("cli.flag.install")
		}
		if f := flag.Lookup("uninstall"); f != nil {
			f.Usage = i18n.T("cli.flag.uninstall")
		}
		if f := flag.Lookup("settings"); f != nil {
			f.Usage = i18n.T("cli.flag.settings")
		}
		if f := flag.Lookup("test"); f != nil {
			f.Usage = i18n.T("cli.flag.test")
		}
		if f := flag.Lookup("list-browsers"); f != nil {
			f.Usage = i18n.T("cli.flag.list_browsers")
		}
		if f := flag.Lookup("list-profiles"); f != nil {
			f.Usage = i18n.T("cli.flag.list_profiles")
		}
		if f := flag.Lookup("check-default"); f != nil {
			f.Usage = i18n.T("cli.flag.check_default")
		}
		if f := flag.Lookup("version"); f != nil {
			f.Usage = i18n.T("cli.flag.version")
		}
		if f := flag.Lookup("installer"); f != nil {
			f.Usage = i18n.T("cli.flag.installer")
		}
		flag.PrintDefaults()
		fmt.Fprint(os.Stderr, i18n.T("cli.usage.examples"))
		fmt.Fprint(os.Stderr, i18n.T("cli.usage.example_install"))
		fmt.Fprint(os.Stderr, i18n.T("cli.usage.example_settings"))
		fmt.Fprint(os.Stderr, i18n.T("cli.usage.example_installer"))
		fmt.Fprint(os.Stderr, i18n.T("cli.usage.example_url"))
	}
	flag.Parse()

	if versionFlag {
		fmt.Println(i18n.T("app.full_name") + " " + i18n.T("app.version"))
		fmt.Println(i18n.T("app.built_with"))
		return
	}

	// --build-bundle：无头构建期打包，产出可分发的 .app 后退出（不启动 GUI、不改系统状态）。
	if buildBundle != "" {
		if err := BuildAppBundle(buildBundle); err != nil {
			fmt.Fprintf(os.Stderr, "build bundle failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("bundle built: %s\n", buildBundle)
		return
	}

	cfg, err := InitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, i18n.T("error.load_config"), err)
		os.Exit(1)
	}

	// 应用用户保存的语言偏好（覆盖系统自动检测）
	if cfg.Language != "" {
		i18n.SetLanguage(cfg.Language)
	}

	// ---- 非 GUI 命令 ----
	if listBrowsers {
		browsers := DetectBrowsers()
		if len(browsers) == 0 {
			fmt.Println(i18n.T("cli.output.no_browsers"))
			return
		}
		fmt.Printf(i18n.T("cli.output.detected_browsers"), len(browsers))
		for _, b := range browsers {
			marker := "  "
			if b.ID == cfg.DefaultBrowser {
				marker = "⭐"
			}
			fmt.Printf("%s %-22s %s\n", marker, b.Name, b.Exec)
		}
		return
	}

	if listProfiles {
		browsers := DetectBrowsers()
		any := false
		for _, b := range browsers {
			profiles := DetectProfiles(b)
			if len(profiles) == 0 {
				continue
			}
			any = true
			fmt.Printf("%s:\n", b.Name)
			for _, p := range profiles {
				fmt.Printf("   [%s] %s (id=%s)\n", p.Kind, p.Name, p.ID)
			}
		}
		if !any {
			fmt.Println(i18n.T("cli.output.no_profiles"))
		}
		return
	}

	if checkDefault {
		isDefault, current := CheckDefaultBrowser()
		if isDefault {
			fmt.Println(i18n.T("cli.output.is_default"))
		} else {
			fmt.Printf(i18n.T("cli.output.not_default"), current)
		}
		return
	}

	if testURL != "" {
		result := MatchURL(testURL, cfg)
		if result.Matched {
			fmt.Printf(i18n.T("cli.output.match_success"),
				result.Rule.Pattern, result.Rule.Mode, result.Browser.Name, result.Browser.Exec)
		} else {
			fmt.Print(i18n.T("cli.output.match_none"))
		}
		fmt.Printf(i18n.T("cli.output.match_detail"), result.MatchLog)
		return
	}

	if installFlag {
		if err := Install(); err != nil {
			fmt.Fprintf(os.Stderr, i18n.T("cli.output.install_failed"), err)
			os.Exit(1)
		}
		if ok, _ := CheckDefaultBrowser(); ok {
			fmt.Println(i18n.T("cli.output.install_success"))
		} else {
			fmt.Println(i18n.T("cli.output.install_partial"))
			fmt.Println(i18n.T("cli.output.install_manual"))
		}
		return
	}

	if uninstallFlag {
		if err := Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, i18n.T("cli.output.uninstall_failed"), err)
			os.Exit(1)
		}
		fmt.Println(i18n.T("cli.output.uninstall_success"))
		return
	}

	if installerFlag {
		RunInstallerUI()
		return
	}

	// ---- GUI / URL 处理 ----
	url := getURLFromArgs(flag.Args())

	if settingsFlag {
		ShowSettings(cfg)
		return
	}
	if url != "" {
		handleURL(url, cfg)
		return
	}
	// 无命令行 URL：作为 .app 被唤起。单 App 模式下进入常驻主循环，
	// 注册 URL 处理器等待 Apple Event，或在用户主动打开时显示设置。
	runAppModeGUI(cfg)
}

// getURLFromArgs extracts a URL from remaining command line args.
func getURLFromArgs(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") || strings.HasPrefix(arg, "ftp://") {
			return arg
		}
	}
	return ""
}

// handleURL：直接处理 URL，弹出选择器或根据规则打开浏览器。
func handleURL(url string, cfg *Config) {
	// 规则命中：无需界面，直接打开。
	result := MatchURL(url, cfg)
	if result.Matched && result.Browser != nil {
		fmt.Printf(i18n.T("cli.output.rule_matched"), result.Rule.Pattern, result.Browser.Name)
		if err := launchForRule(url, result); err == nil {
			return
		}
		fmt.Fprint(os.Stderr, i18n.T("cli.output.rule_fallback"))
	}

	if !cfg.ShowPickerOnMiss {
		// 关闭"无匹配弹选择器"：直接用默认浏览器打开。
		def := findBrowserByID(cfg.DefaultBrowser, cfg)
		if def == nil {
			// 默认浏览器无效（未设置或已卸载）：回退到第一个可用浏览器，
			// 避免 URL 被静默丢弃、什么也打不开。
			if list := cfg.FavoriteBrowsers(); len(list) > 0 {
				def = &list[0]
			}
		}
		if def != nil {
			if err := LaunchBrowser(*def, url, false); err != nil {
				fmt.Fprintf(os.Stderr, i18n.T("cli.output.default_failed"), err)
			}
			return
		}
		// 连一个可用浏览器都没有：只能退回选择器（即便设置关闭），总比什么都不做好。
		fmt.Fprint(os.Stderr, i18n.T("cli.output.rule_fallback"))
	}

	// 弹出选择器窗口
	ShowPicker(url, cfg)
}

// RunInstallerUI 运行安装器 UI（独立应用）
func RunInstallerUI() {
	a := app.New()
	w := a.NewWindow(i18n.T("installer.window_title"))

	content := BuildInstallerUI(a, w)
	w.SetContent(content)
	w.Resize(fyne.NewSize(600, 520))
	w.CenterOnScreen()
	w.ShowAndRun()
}

// BuildInstallerUI 构建安装器 UI
func BuildInstallerUI(a fyne.App, w fyne.Window) fyne.CanvasObject {
	fg := tcol(theme.ColorNameForeground)
	subtle := tcol(theme.ColorNamePlaceHolder)

	// 标题区域
	titleIcon := canvas.NewText("🚀", accent)
	titleIcon.TextSize = 48
	titleIcon.Alignment = fyne.TextAlignCenter
	appName := boldText(i18n.T("app.full_name"), 28, accent)
	versionText := plainText(i18n.T("app.version"), 14, subtle)
	versionText.Alignment = fyne.TextAlignCenter

	titleArea := container.NewVBox(
		container.NewCenter(titleIcon),
		vSpace(10),
		container.NewCenter(appName),
		container.NewCenter(versionText),
		vSpace(8),
	)

	// 描述
	descLabel := plainText(i18n.T("installer.tagline"), 14, fg)
	descLabel.Alignment = fyne.TextAlignLeading

	descArea := container.NewPadded(container.NewBorder(nil, nil, descLabel, nil))

	// 功能列表
	features := []struct {
		name        string
		description string
		icon        string
	}{
		{i18n.T("installer.features.smart_rules.name"), i18n.T("installer.features.smart_rules.desc"), "✅"},
		{i18n.T("installer.features.multi_profile.name"), i18n.T("installer.features.multi_profile.desc"), "👥"},
		{i18n.T("installer.features.custom_rules.name"), i18n.T("installer.features.custom_rules.desc"), "⚡"},
		{i18n.T("installer.features.countdown.name"), i18n.T("installer.features.countdown.desc"), "⏱️"},
	}

	featuresBox := container.NewVBox(boldText(i18n.T("installer.core_features"), 14, fg), vSpace(6))
	for _, feat := range features {
		row := container.NewHBox(
			plainText(feat.icon, 16, accent),
			hSpace(8),
			container.NewVBox(
				plainText(feat.name, 13, fg),
				plainText(feat.description, 11, subtle),
			),
		)
		featuresBox.Add(container.NewPadded(row))
	}
	featuresArea := container.NewPadded(container.NewBorder(nil, nil, featuresBox, nil))

	// 当前状态
	statusBox := buildStatusPanel(w)

	// 操作按钮
	actionBtns := container.NewGridWithColumns(2,
		widget.NewButtonWithIcon(i18n.T("installer.btn.install"), theme.ContentAddIcon(), func() {
			handleInstall(w)
		}),
		widget.NewButtonWithIcon(i18n.T("installer.btn.set_default"), theme.SettingsIcon(), func() {
			handleSetDefault(w)
		}),
	)

	// 主布局
	return container.NewVBox(
		titleArea,
		container.NewBorder(nil, nil, descArea, nil),
		vSpace(12),
		featuresArea,
		vSpace(12),
		statusBox,
		vSpace(12),
		container.NewCenter(actionBtns),
		layout.NewSpacer(),
	)
}

func buildStatusPanel(w fyne.Window) fyne.CanvasObject {
	fg := tcol(theme.ColorNameForeground)

	info, err := GetInstallInfo()
	if err != nil {
		return plainText(fmt.Sprintf(i18n.T("installer.status.fetch_failed"), err), 13, fg)
	}

	statusIcon := "🟡"
	statusText := i18n.T("common.not_installed")
	if info.Installed {
		statusIcon = "✅"
		statusText = i18n.T("common.installed")
	}

	currentDefault := i18n.T("common.unknown")
	if isDef, cur := CheckDefaultBrowser(); isDef {
		currentDefault = "BrowserSwitch"
	} else if cur != "" && cur != "com.apple.safari" {
		currentDefault = cur
	} else {
		currentDefault = i18n.T("installer.status.safari_default")
	}

	return container.NewBorder(
		nil, nil,
		container.NewVBox(
			boldText(i18n.T("installer.current_status"), 14, fg),
			vSpace(6),
			container.NewHBox(
				plainText(statusIcon+" ", 14, accent),
				plainText(fmt.Sprintf(i18n.T("installer.status.install"), statusText), 13, fg),
			),
			container.NewHBox(
				plainText("🔗 ", 14, accent),
				plainText(fmt.Sprintf(i18n.T("installer.status.default_browser"), currentDefault), 13, fg),
			),
		), nil,
	)
}

func handleInstall(w fyne.Window) {
	if err := Install(); err != nil {
		dialog.ShowError(err, w)
		return
	}

	if ok, _ := CheckDefaultBrowser(); ok {
		dialog.ShowInformation(i18n.T("common.done"), i18n.T("installer.dialog.done_default"), w)
	} else {
		dialog.ShowInformation(i18n.T("common.installed"), i18n.T("installer.dialog.installed_manual"), w)
	}
}

func handleSetDefault(w fyne.Window) {
	if ok, _ := CheckDefaultBrowser(); ok {
		dialog.ShowInformation(i18n.T("common.tip"), i18n.T("installer.dialog.already_default"), w)
		return
	}

	handleInstall(w)
}
