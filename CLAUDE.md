# CLAUDE.md

本文件为 AI 编码助手（Claude Code / CodeBuddy / Cursor 等）提供本仓库的架构约束与工作约定。**这是 Agent 文档的唯一事实来源**；其他 Agent 配置文件应指向本文件而非复制内容。

---

## 0. 硬性约束（先读这一节）

1. **这是 macOS-only 项目。** 所有平台代码都带 `//go:build darwin`。仓库中**不存在** `*_linux.go` / `*_windows.go`，`Makefile` 也只提供 macOS 目标（`build` / `test` / `vet` / `app` / `dmg` / `clean`）。任何声称可跨平台构建的说法都是过时的——本项目只能在 macOS 上编译运行。
2. **CGO 必须开启。** `CGO_ENABLED=0` 编译会失败。依赖三个 framework：CoreServices（默认浏览器读写）、CoreFoundation、Carbon（Apple Event）。
3. **GUI 是 Fyne v2.7，不是 IUP。** 早期提交用过 IUP，已在 `c892172` 迁移。任何提到 IUP、nib 资源、`IupPostMessage` 的描述都是过时的。
4. **架构是单 App，不是双 App。** `1c7298a` 已从「AppleScript 转发器 + Helper」迁移到「单 App + Carbon Apple Event 直收」。任何提到 `osacompile`、`on open location`、`Browser Switch Helper.app` 的描述都是过时的（除 `cleanupLegacyApps()` 的升级清理逻辑外）。
5. **不要主动执行 git 提交、分支、push 操作**，除非用户明确要求。
6. **注释一律用中文**，与现有代码库保持一致。新增代码的注释密度、命名风格须匹配周边代码。

---

## 1. 常用命令

```bash
# 构建（必须 CGO；产物 browser-switch）
make build
# 等价：CGO_ENABLED=1 go build -ldflags="-s -w" -o browser-switch .

# 测试与静态检查
go test ./...
go test -v -run TestMatchPatternModes ./...
go vet ./...

# 功能验证（不改动系统状态）
./browser-switch --list-browsers
./browser-switch --list-profiles
./browser-switch --test https://github.com
./browser-switch --check-default
./browser-switch --version

# 会改动系统状态（⚠️ 修改默认浏览器 / 写入 ~/Applications）
./browser-switch --install
./browser-switch --uninstall

# GUI
./browser-switch --settings
./browser-switch --installer
```

`make build` / `test` / `vet` / `app` / `dmg` / `clean` 均已定义；`make deps` **未定义**，别照抄。

配置文件：`~/.config/browser-switch/config.json`。

---

## 2. 架构总览

### 2.1 两条 URL 入口路径（重要）

代码里存在**两条几乎相同但独立实现**的 URL 决策路径，改动规则逻辑时**必须同时改两处**：

| 路径 | 触发场景 | 函数 |
|------|----------|------|
| 命令行参数 | `./browser-switch https://x.com` | [main.go](main.go) `handleURL()` |
| Apple Event | 已安装为默认浏览器，用户点击链接 | [urlhandler_darwin.go](urlhandler_darwin.go) `openPicker()` |

此外 [picker_decision_test.go](picker_decision_test.go) 里的 `decideAction()` 是**第三份复制**——测试测的是复制品，不是生产代码。这是已知的 DRY 违规（见 [docs/ISSUES.md](docs/ISSUES.md) I-1），修复时应抽取 `decideURLAction(url, cfg) (action, *Browser)` 供三处共用。

### 2.2 生命周期

```
进程启动
  → init(): runtime.LockOSThread()      // Fyne on macOS 要求主 goroutine 锁 OS 线程
  → i18n.Init()                          // 必须先于任何 i18n.T() 调用
  → flag.Parse()
  → InitConfig()                         // 首次运行检测浏览器并落盘
  → cfg.Language 覆盖系统语言检测
  → 分派：
      CLI 只读命令  → 打印后 return
      安装/卸载命令 → Install()/Uninstall() 后 return
      --settings   → ShowSettings()（自建 app，阻塞）
      argv 里有 URL → handleURL()
      其余（.app 被唤起）→ runAppModeGUI()
```

`runAppModeGUI()`（单 App 主循环，**非常驻**）：
1. `installURLHandler()` 注册 Carbon `kAEGetURL` 处理器
2. 建一个 1×1 的隐藏 `master` 窗口维持事件循环
3. 起 goroutine 消费 `urlEventCh`，收到 URL → `fyne.Do(openPicker)`
4. 起 goroutine 等 500ms；仍无 URL → 判定用户主动点了 App → 把 `master` 变成设置窗口
5. `decided int32` 用 CAS 保证 3 与 4 只有一个胜出

**为什么非常驻**：Fyne 的 glfw 驱动不把 Dock 点击的 Reopen 事件转发给 Carbon 处理器。常驻进程收不到「再次点击」，行为不可预测。非常驻下每次都是冷启动，语义明确。

### 2.3 模块职责

| 文件 | 职责 | 关键符号 |
|------|------|----------|
| [main.go](main.go) | CLI 分派、`handleURL` 命令行路径、安装向导 UI | `main`、`handleURL`、`BuildInstallerUI` |
| [config.go](config.go) | 数据模型 + JSON 持久化 | `Config`、`Browser`、`Rule`、`Profile`、`InitConfig`、`SaveConfig`、`FavoriteBrowsers` |
| [rules.go](rules.go) | 纯函数匹配引擎（无副作用，易测） | `MatchURL`、`matchPattern`、`matchWildcard`、`ValidatePattern`、`SuggestMatchMode`、`findBrowserByID` |
| [picker.go](picker.go) | 选择器 UI + 倒计时 + 快捷键 + 写规则 | `ShowPicker`、`setupPickerWindow`、`buildPickerUI`、`addRuleForURL`、`extractDomain` |
| [settings.go](settings.go) | 设置窗口外壳（侧边栏 + 面板切换）+ 共用行/空态 | `buildSettingsContent`、`listRow`、`formRow`、`emptyState`、`mergeDetected`、`indexOf` |
| [settings_browsers.go](settings_browsers.go) | 浏览器面板（左收藏 / 右全部） | `buildBrowsersTab` |
| [settings_rules.go](settings_rules.go) | 规则面板 + 添加规则对话框 | `buildRulesTab`、`buildRuleList`、`showAddRuleDialog`、`removeRule`、`modeDisplayName` |
| [settings_general.go](settings_general.go) | 通用面板（语言 / 默认浏览器 / 倒计时 / 安装卸载） | `buildGeneralTab`、`showSetOtherDefaultDialog` |
| [gui.go](gui.go) | 设计系统 + 共享控件与主题工具 | `sysColor`、`accent`、`syncAccentColor`、`appleSwitch`、`sidebarItem`、`tileShadow`、`pill`、`progressLine`、`browserIcon`、`truncateTail`、`tcol`、`shortName` |
| [constants.go](constants.go) | 应用名与 bundle ID | `AppName`、`AppBundleID`、`HelperExecName` |
| [browsers_darwin.go](browsers_darwin.go) | 检测 + 启动 | `DetectBrowsers`、`LaunchBrowser`、`looksLikeBundleID` |
| [install_darwin.go](install_darwin.go) | `.app` 打包、签名、LaunchServices（cgo） | `Install`、`Uninstall`、`GetInstallInfo`、`CheckDefaultBrowser`、`SetSystemDefaultBrowser` |
| [urlhandler_darwin.go](urlhandler_darwin.go) | Apple Event 接收（cgo）+ 单 App 主循环 | `installURLHandler`、`runAppModeGUI`、`goHandleAppleEventURL` |
| [profiles_darwin.go](profiles_darwin.go) | 多账户检测与带参启动 | `DetectProfiles`、`LaunchBrowserProfile`、`appExecPath`、`Browser.BundleID` |
| [icons_darwin.go](icons_darwin.go) | `.icns` → PNG 缓存 | `browserIconPath` |
| [i18n/i18n.go](i18n/i18n.go) | 内嵌 7 语言包 | `Init`、`T`、`Tf`、`SetLanguage`、`SupportedLanguages` |

---

## 3. 关键实现细节（改代码前必读）

### 3.1 规则匹配

- 排序在**副本**上做（`sortedRules := make(...)`），不改动 `cfg.Rules` 的原始顺序。
- **www 归一化**：先用 `host` 匹配，未中且 `host != hostNoWWW` 时再用 `hostNoWWW` 匹配一次。这是「记住选择」写入 `example.com` 后能命中 `www.example.com` 的原因。写入侧 `extractDomain()` 也剥离了 `www.`，两侧必须保持一致。
- `regex` 与 `contains` 同时对 host 和完整 URL 做匹配；其余模式只看 host。
- 命中规则但 `findBrowserByID` 返回 nil 时**继续下一条规则**，不中断。
- 正则每次匹配都重新 `regexp.Compile`，无缓存。

### 3.2 Fyne 使用约定

- **后台 goroutine 更新 UI 必须包在 `fyne.Do()` 里**（倒计时、异步重新扫描均如此）。
- **`SetSelected` / `SetSelectedIndex` / `SetChecked` 会触发 `OnChanged`。** 必须先设初值再挂回调，否则初始化就会触发副作用。`buildGeneralTab` 里的语言选择器曾因此无限递归重建设置页导致内存暴涨——注释里有记录，别改回去。
- **Fyne 2.7 中替换 `TabItem.Content` 对当前显示的 tab 不生效。** `buildSettingsContent` 用稳定的 `container.NewStack()` 包裹每个 tab 内容，刷新时替换 `Stack.Objects` 再 `Refresh()`。
- 新建对话框时**复用已有的 `fyne.App` 实例**（`OpenSettingsWindow(a, cfg)`、`showAddRuleDialog(a, ...)`），不要 `app.New()`——会嵌套事件循环。
- 颜色一律走 `tcol(theme.ColorNameXxx)` 取语义色，不要硬编码，否则深色模式下不可读。品牌色只有 `accent`（蓝）和 `warn`（红）两个。

### 3.3 浏览器启动

- 普通启动：`open -b <bundleID> <url>`（不加 `-n`，复用已开窗口）。
- **带 profile 启动必须直接执行包内二进制**，因为 `open -b` 在浏览器已运行时会丢弃 `--profile-directory` 等参数。路径由 `appExecPath()` 解析 `CFBundleExecutable` 得到。
- 用 `cmd.Start()` 而非 `Run()`，选择器不等浏览器退出。
- Edge 的无痕参数是 `--inprivate`，不是 `--incognito`。

### 3.4 安装流程的顺序敏感点

`buildMainApp()` 中**必须先 `os.Open(srcExec)` 拿到文件句柄，再 `os.RemoveAll(appPath)`**。当进程本身运行在 `appPath` 内（从已安装的 `.app` 里点「重新安装」）时，先删目录会让源文件句柄失效。这是 `copyReader()` 存在的唯一原因，别把它合并回 `copyFile()`。

### 3.5 配置持久化的坑

- `configPath` 是包级全局变量，由 `InitConfig()` 设置。测试里通过直接赋值来隔离（见 [remember_flow_test.go](remember_flow_test.go)），因此这些测试**不能并行**。
- `Install()` / `Uninstall()` 内部各自调用 `InitConfig()` 重新从磁盘读一份 `Config` 并 `SaveConfig`，与 UI 持有的 `cfg` 是**不同对象**。这会导致 `prev_default_browser` 被 UI 侧的后续 `SaveConfig(cfg)` 覆盖丢失（见 [docs/ISSUES.md](docs/ISSUES.md) I-2）。改动安装逻辑时务必注意。
- `SaveConfig` 用 `os.WriteFile` 直接截断写，**非原子**。写入中途崩溃会留下空文件（`InitConfig` 里的空文件处理分支就是为此打的补丁）。

### 3.6 i18n

- `i18n.Init()` 必须在**任何** `i18n.T()` 之前调用。`main()` 第一行就是它。
- `flag.Usage` 里的 flag 描述文本是在 `Usage` 回调内延迟设置的，因为 `flag.BoolVar` 注册时 i18n 尚未初始化。
- 新增 UI 文案需**同时更新 7 个语言包**（`i18n/locales/*.json`），每个文件当前 147 个 key。可用下面的脚本自检漏翻：

```bash
rg -o 'i18n\.Tf?\("([^"]+)"' -r '$1' --no-filename . | sort -u > /tmp/used.txt
for f in i18n/locales/*.json; do
  python3 -c "import json;print('\n'.join(sorted(json.load(open('$f')))))" > /tmp/have.txt
  echo "== $f 缺失："; comm -23 /tmp/used.txt /tmp/have.txt
done
```

- `app.version` 是一个**翻译 key**（值为 `"Version 1.0.0"`），版本号硬编码在 7 个语言包里。版本升级需要改 7 个文件——这是设计缺陷（ISSUES I-18），不是可以照抄的模式。

### 3.7 界面设计系统（改 UI 前必读）

界面按 macOS HIG 实现，度量与颜色集中在 [gui.go](gui.go)，**新增界面请复用那里的令牌与控件，不要另起一套数字**：

- **间距**用 `sp2/4/8/12/16/20/24`（8pt 网格），**字号**用 `fsCaption/Subhead/Body/Title/Display`（11/12/13/15/20），**圆角**用 `radControl/radCard/radTile`（6/10/14）。直接写 `float32(12)` 会让界面在多语言下错位。
- **颜色**用主题语义色（`tcol` / `fgCol` / `secCol`）或 Apple 系统色（`accent()` / `sysRed` 等）。强调色在 `main()` 里经 `syncAccentColor()` 从 `AppleAccentColor` 读取一次，因此**必须在构建任何界面之前调用它**。
- **不要用 `container.NewPadded`**：它固定 4px，放不下 macOS 的 20pt 内容边距。用 `inset()` / `padH()`。
- **不要硬编码窗口高度**：头尾高度随字号与语言变化。`resize` 用「整窗 MinSize − 网格 MinSize」实测出 chrome 高度，再加上网格高度。
- **网格用 `container.NewGridWrap` 而非 `NewGridWithColumns`**：后者按内容最小宽度撑出上千像素的 MinSize，会把窗口撑大、磁贴裁切。
- **放进滚动容器的内容要用 `fixedSize()` 钉住尺寸**：`GridWrap` 的 MinSize 依赖上一次 Layout 的结果，直接塞进 `VScroll` 会只显示一个单元格高。

#### 三个 Fyne 布局陷阱（都已踩过，改布局前务必确认）

1. **`GridWrap` 的列数是「算出来的」，不可靠**。它的公式是
   `floor((可用宽 + padding) / (格宽 + padding))`；当我们按「格宽 = 可用宽 / 期望列数」
   取值时，该式正好等于整数 N，float32 舍入误差会让它变成 N−0.0000001，floor 后少一整列
   ——最后一个磁贴被挤到下一行，而我们的行数仍按原列数算，于是它溢出网格、盖在 footer 上。
   **改用 `layout.NewGridLayout(cols)`**：列数显式给定、宽度自动均分，没有临界问题
   （它过大的 MinSize 由外层 `fixedSize` 覆盖）。

2. **`GridWrap.MinSize` 只读 Layout 缓存的 `rowCount`**（永远 ≤ 已布局过的行数），
   所以「renderGrid 换完内容立刻读 `root.MinSize()`」会拿到旧值，窗口比内容矮一截。
   `GridLayout.MinSize` 则是由子元素数量与列数直接算出，不依赖布局时机 —— 这是选它的第二个理由。

3. **`Border` 即使没放 left/right，也会给中间区预留 `2×theme.Padding()`**（8px）。
   按「窗口宽 − 左右边距」算可用宽度会多算这 8px，内容比窗口宽、整行被横向压扁。
   计算可用宽度时要再减 `2*theme.Padding()`。

另外：**`HBox` 会把子控件纵向拉伸到行高**。定高的控件（如 38×22 的开关）必须用
`layout.NewGridWrapLayout` 包一层钉住尺寸，否则会被撑变形。
- 布尔项用 `appleSwitch`（开关）而不是 `widget.NewCheck`（勾选框），这是系统设置的表达。
- **下拉框必须用 `fixedSelect()` 钉住宽度**：`widget.Select` 的 `MinSize()` 只按 `PlaceHolder` 与**当前选中项**计算（`selectRenderer.MinSize`），不设 PlaceHolder 时宽度就等于当前文字宽度 —— 切换选项会把同一行的标签推来推去。`fixedSelect` 按「最长选项 + 5 个字符余量」固定宽度。
- **弹窗类窗口要做单例**：`a.NewWindow()` 每次调用都会开一个新窗口，重复点击按钮会叠出多个相同窗口（后开的盖住前一个，用户以为内容丢了）。`showAddRuleDialog` 用包级 `addRuleWin` + `SetOnClosed` 清理作范本 —— `SetOnClosed` 能覆盖提交/取消/点关闭按钮全部路径，避免留下悬空引用。
- **窗口之间要登记父-子关系**：Fyne 的 `fyne.Window` 接口没有稳定身份（`test` 驱动与真驱动下 `a.NewWindow` 返回的具体类型不同，`==` 比较会失败），所以「关主窗口时关子窗口」不能依赖 `AllWindows` + `==`。用 `trackChild(parent, child)` 显式登记（见 `gui.go`）：parent 关闭时遍历自己开过的 child 一起关，child 关闭时把自己从 parent 的列表里摘掉，避免反复开关后表无限膨胀。
- 长文本（浏览器名、URL、域名）渲染前用 `truncateTail()` 收敛：`canvas.Text` 不会自动截断，会撑破固定宽度的磁贴。

---

## 4. 测试

现有测试全部是纯函数测试，不触碰 GUI：

| 文件 | 覆盖 |
|------|------|
| [addrule_test.go](addrule_test.go) | `ValidatePattern`、`SuggestMatchMode`、`matchPattern` 六模式、优先级排序、禁用规则跳过、`removeRule`、去重逻辑 |
| [picker_decision_test.go](picker_decision_test.go) | `ShowPickerOnMiss` 分支决策（**测的是复制的 `decideAction`，非生产代码**） |
| [remember_flow_test.go](remember_flow_test.go) | `addRuleForURL` 落盘 + 闭环匹配（会写临时目录，改 `configPath` 全局） |
| [rules_flow_test.go](rules_flow_test.go) | 「记住选择」→ 下次匹配的 www 一致性 |
| [ui_smoke_test.go](ui_smoke_test.go) | 离屏冒烟：用 `test.NewApp()` 渲染选择器 / 设置三个面板 / 安装器，断言不 panic 且窗口尺寸在合理区间（含 24 个浏览器的滚动分支） |

**无 cgo 层测试、无集成测试。** 新增测试时优先把逻辑抽成纯函数，而不是去测 Fyne 控件；`ui_smoke_test.go` 只保证界面「能建起来、尺寸不失控」，不校验视觉细节。

---

## 5. 编码原则（本仓库的执行标准）

- **KISS**：优先最直观的实现。这个项目的复杂度全部集中在 macOS 平台交互，业务逻辑应保持简单。
- **YAGNI**：不为未来预留字段。历史上 `Config` 曾长期保留 `remember_choice` / `skip_installed` / `tray_icon` / `window_width` / `window_height` 等从未被读取的字段，以及 `visibleBrowsers()`、`truncMiddle()`、`reopenCh` / `goHandleReopen()` 等死代码，`i18n.appleLocale()` 也曾是空实现——均已在开源前清理（`appleLocale` 现已真正实现）。当前仅存 `Browser.IsCustom`（`mergeDetected` 会保留但无 UI 能创建，见 ISSUES I-15）尚待处理，不要再新增此类预留。
- **DRY**：新增逻辑前先搜是否已存在。当前已知重复：`handleURL` vs `openPicker` vs `decideAction`（三份 URL 决策，见 ISSUES I-1）、`buildInfoPlist` vs `buildInfoPlistWithIcon`（两份 plist 模板，见 ISSUES I-16）。
- **SRP**：设置窗口已按面板拆成 [settings.go](settings.go)（外壳 + 共用行/空态）、[settings_browsers.go](settings_browsers.go)、[settings_rules.go](settings_rules.go)、[settings_general.go](settings_general.go) 四个文件。新增设置项请落到对应面板的文件里，不要往外壳文件追加。
- **先读后写**：修改任何 `_darwin.go` 前先通读该文件，其中大量注释记录了为何不能用「更显然」的写法。

---

## 6. 危险操作清单

以下操作会改动用户系统状态或不可逆，执行前必须向用户明确确认：

| 操作 | 影响 |
|------|------|
| `./browser-switch --install` | 修改系统默认浏览器；在 `~/Applications` 写入 `.app` |
| `./browser-switch --uninstall` | 修改系统默认浏览器；删除 `~/Applications/Browser Switch.app` |
| 设置界面「将其他浏览器设为系统默认」 | 直接调用 `LSSetDefaultHandlerForURLScheme` |
| 修改 `~/.config/browser-switch/config.json` | 覆盖用户规则；`SaveConfig` 非原子写，中断会损坏配置 |
| `rm -rf /tmp/browser-switch-icons` | 清除图标缓存（可重建，风险低） |
| 任何 `git commit` / `git push` / `git reset --hard` | 用户未主动要求时**不得执行** |

---

## 7. 相关文档

- [README.md](README.md) —— 用户向使用说明
- [PRD.md](PRD.md) —— 产品需求（已按真实实现校对）
- [docs/ISSUES.md](docs/ISSUES.md) —— 现存问题清单（含代码定位与修复建议）
- [docs/ROADMAP.md](docs/ROADMAP.md) —— 产品规划建议
