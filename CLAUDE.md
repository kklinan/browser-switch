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
| [config.go](config.go) | 数据模型 + JSON 持久化 + 加载时清理 | `Config`、`Browser`、`Rule`、`Profile`、`InitConfig`、`SaveConfig`、`NormalizeRules`、`isKnownMatchMode`、`FavoriteBrowsers` |
| [rules.go](rules.go) | 纯函数匹配引擎（无副作用，易测） | `MatchURL`、`matchPattern`、`matchWildcard`、`wildcardRegexp`、`ValidatePattern`、`ValidateRuleInput`、`SuggestMatchMode`、`findBrowserByID` |
| [picker.go](picker.go) | 选择器 UI + 倒计时 + 快捷键 + 写规则 | `ShowPicker`、`setupPickerWindow`、`buildPickerUI`、`addRuleForURL`、`extractDomain`、`clickableURLLabel` |
| [settings.go](settings.go) | 设置窗口三标签页 + 规则对话框 | `OpenSettingsWindow`、`reloadConfigInto`、`buildSettingsContent`、`buildBrowsersTab`、`buildRulesTab`、`buildGeneralTab`、`showAddRuleDialog`、`modeDisplayNames`、`modeFromDisplayName`、`modeHintText` |
| [gui.go](gui.go) | 共享控件与主题工具 | `card`、`progressLine`、`shadowed`、`browserIcon`、`tcol`、`shortName` |
| [constants.go](constants.go) | 应用名与 bundle ID | `AppName`、`AppBundleID`、`HelperExecName` |
| [browsers_darwin.go](browsers_darwin.go) | 检测 + 启动（含新窗口） | `DetectBrowsers`、`LaunchBrowser`、`launchNewWindow`、`appleScriptString`、`looksLikeBundleID` |
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
- `exact`（域名相等）与 `urlequal`（完整网址相等）是精确比较，只看各自的目标串。
- `contains` / `prefix` / `suffix` 是 `wildcard` 的**快捷方式**（≈ `*p*` / `p*` / `*p`），面向不熟悉 glob/正则的用户，属一等公民，**不要**在加载时把它们迁移成 `wildcard`。三者**不解释** `*` 与 `?`（按字面量处理），写了必然永不命中——这由 `ValidateRuleInput` 在保存时拦截。
- **除 `exact` / `urlequal` 外，所有模式都对 host 和完整 URL 各试一次**（`||` 关系）。这是必须的：`.pdf`、`https://`、`*/settings` 这类内容根本不出现在 host 里，只看 host 的话规则永远不生效（曾经的 bug）。
- 命中规则但 `findBrowserByID` 返回 nil 时**继续下一条规则**，不中断。
- `wildcard` 编译结果有缓存（`wildcardCache` / `sync.Map`）；**`regex` 仍每次重新 `regexp.Compile`，无缓存**（见 ISSUES 性能章节）。

**保存前的语义校验**：`ValidateRuleInput(pattern, mode, browserID, rules, excludeID)` 独立于 `ValidatePattern`。后者只保证语法可编译，前者捕捉"语法没问题但保存后一定不生效"或"毫无意义"的写法：`urlequal` 缺 `://`、`exact` 含 scheme 或路径、快捷方式里出现 `*`/`?`、完全重复的规则。新增匹配模式时，两个函数**都要**同步。`excludeID` 用于编辑场景排除规则自身。

### 3.2 Fyne 使用约定

- **后台 goroutine 更新 UI 必须包在 `fyne.Do()` 里**（倒计时、异步重新扫描均如此）。
- **`SetSelected` / `SetSelectedIndex` / `SetChecked` 会触发 `OnChanged`。** 必须先设初值再挂回调，否则初始化就会触发副作用。`buildGeneralTab` 里的语言选择器曾因此无限递归重建设置页导致内存暴涨——注释里有记录，别改回去。
- **由此而来的第二个坑：`SetSelected` 无法区分"代码设置"与"用户操作"。** 规则对话框靠 `setProgrammatically` 标志位区分，仅当回调来自用户时才关闭模式自动推测（`modeAuto = false`）。没有这层保护的话，`SetSelected` 自己触发的 `OnChanged` 会被误判成用户改过模式。
- **预填表单的顺序敏感**：编辑规则时必须**先设模式、再设 pattern**，并在两者之间关掉自动推测。反过来的话 `SetText` 触发的 `OnChanged` 会用自动推测结果覆盖掉刚设好的原模式（用户看到"修改规则时匹配模式不对"）。
- **窗口单例**：`settingsWin` 与 `ruleDialogWin` 两个包级变量保证各窗口最多一个。创建后必须 `SetOnClosed` 清引用，否则会去 `Close()` 一个已销毁的窗口。规则窗口是设置窗口的子窗口，设置窗口关闭/重载时也要一并关掉它，避免用旧数据误保存。
- **重开设置窗口必须先重载磁盘数据**：见 3.5。
- **Fyne 2.7 中替换 `TabItem.Content` 对当前显示的 tab 不生效。** `buildSettingsContent` 用稳定的 `container.NewStack()` 包裹每个 tab 内容，刷新时替换 `Stack.Objects` 再 `Refresh()`。
- 新建对话框时**复用已有的 `fyne.App` 实例**（`OpenSettingsWindow(a, cfg)`、`showAddRuleDialog(a, ...)`），不要 `app.New()`——会嵌套事件循环。
- 颜色一律走 `tcol(theme.ColorNameXxx)` 取语义色，不要硬编码，否则深色模式下不可读。品牌色只有 `accent`（蓝）和 `warn`（红）两个。

### 3.3 浏览器启动

两个入口都带 `newWindow bool` 参数：

```go
LaunchBrowser(b Browser, url string, newWindow bool) error
LaunchBrowserProfile(b Browser, p Profile, url string, newWindow bool) error
```

- 普通启动：`open -b <bundleID> <url>`（不加 `-n`，复用已开窗口）。
- **带 profile 启动必须直接执行包内二进制**，因为 `open -b` 在浏览器已运行时会丢弃 `--profile-directory` 等参数。路径由 `appExecPath()` 解析 `CFBundleExecutable` 得到。
- 用 `cmd.Start()` 而非 `Run()`，选择器不等浏览器退出。
- Edge 的无痕参数是 `--inprivate`，不是 `--incognito`。
- **`newWindow` 不能只用 `open -n`。** Chrome / Edge / Safari 都是单实例应用，进程内用锁保证唯一，`open -n` 会被直接忽略、URL 照旧进已有窗口的新标签页。`launchNewWindow()` 因此按类型分派：Chromium 系与 Firefox 直接执行二进制传 `--new-window`；**Safari 没有任何 CLI 开关**，只能用 AppleScript `make new document`；未知浏览器才退回 `open -n`。全部失败时降级为普通打开（宁可开在已有窗口，也不能打不开）。
- AppleScript 拼串**必须**走 `appleScriptString()` 转义反斜杠与引号，否则 URL 里的特殊字符会破坏脚本。

### 3.4 安装流程的顺序敏感点

`buildMainApp()` 中**必须先 `os.Open(srcExec)` 拿到文件句柄，再 `os.RemoveAll(appPath)`**。当进程本身运行在 `appPath` 内（从已安装的 `.app` 里点「重新安装」）时，先删目录会让源文件句柄失效。这是 `copyReader()` 存在的唯一原因，别把它合并回 `copyFile()`。

### 3.5 配置持久化的坑

- `configPath` 是包级全局变量，由 `InitConfig()` 设置。测试里通过直接赋值来隔离（见 [remember_flow_test.go](remember_flow_test.go)），因此这些测试**不能并行**。
- `Install()` / `Uninstall()` 内部各自调用 `InitConfig()` 重新从磁盘读一份 `Config` 并 `SaveConfig`，与 UI 持有的 `cfg` 是**不同对象**。这会导致 `prev_default_browser` 被 UI 侧的后续 `SaveConfig(cfg)` 覆盖丢失（见 [docs/ISSUES.md](docs/ISSUES.md) I-2）。改动安装逻辑时务必注意。
- `SaveConfig` 用 `os.WriteFile` 直接截断写，**非原子**。写入中途崩溃会留下空文件（`InitConfig` 里的空文件处理分支就是为此打的补丁）。原子化改造见 ISSUES I-5。
- **`reloadConfigInto(cfg)` 是就地赋值 `*cfg = *fresh`，不是返回新指针。** 选择器与设置窗口共享同一个 `*Config`，换指针的话只有一方看得到新数据。读盘失败或配置损坏时**保留内存中现有值**，不把界面清空。
- `NormalizeRules()` 在 `LoadConfig` 里自动调用，只做两件事：丢弃空 pattern 规则、未知 mode 兜底为 `exact`。**不要把 `contains`/`prefix`/`suffix` 改成 `wildcard`**——它们是面向小白的快捷方式，改写会让用户再次编辑时看到模式莫名变了。

### 3.6 i18n

- `i18n.Init()` 必须在**任何** `i18n.T()` 之前调用。`main()` 第一行就是它。
- `flag.Usage` 里的 flag 描述文本是在 `Usage` 回调内延迟设置的，因为 `flag.BoolVar` 注册时 i18n 尚未初始化。
- 新增 UI 文案需**同时更新 7 个语言包**（`i18n/locales/*.json`），每个文件当前 **163 个 key**。可用下面的脚本自检漏翻：

```bash
rg -o 'i18n\.Tf?\("([^"]+)"' -r '$1' --no-filename . | sort -u > /tmp/used.txt
for f in i18n/locales/*.json; do
  python3 -c "import json;print('\n'.join(sorted(json.load(open('$f')))))" > /tmp/have.txt
  echo "== $f 缺失："; comm -23 /tmp/used.txt /tmp/have.txt
done
```

- `app.version` 是一个**翻译 key**（值为 `"Version 1.0.0"`），版本号硬编码在 7 个语言包里。版本升级需要改 7 个文件——这是设计缺陷（ISSUES I-18），不是可以照抄的模式。

---

## 4. 测试

现有测试全部是纯函数测试，不触碰 GUI：

| 文件 | 覆盖 |
|------|------|
| [addrule_test.go](addrule_test.go) | `ValidatePattern`、`ValidateRuleInput`、`SuggestMatchMode`、`matchPattern` 七模式、快捷方式匹配完整 URL、`NormalizeRules` 保留快捷方式、`modeEquivalentWildcard`、优先级排序、禁用规则跳过、`removeRule`、去重逻辑 |
| [picker_decision_test.go](picker_decision_test.go) | `ShowPickerOnMiss` 分支决策（**测的是复制的 `decideAction`，非生产代码**） |
| [remember_flow_test.go](remember_flow_test.go) | `addRuleForURL` 落盘 + 闭环匹配（会写临时目录，改 `configPath` 全局） |
| [rules_flow_test.go](rules_flow_test.go) | 「记住选择」→ 下次匹配的 www 一致性 |

**无 GUI 测试、无 cgo 层测试、无集成测试。** 新增测试时优先把逻辑抽成纯函数，而不是去测 Fyne 控件。

---

## 5. 编码原则（本仓库的执行标准）

- **KISS**：优先最直观的实现。这个项目的复杂度全部集中在 macOS 平台交互，业务逻辑应保持简单。
- **YAGNI**：不为未来预留字段。历史上 `Config` 曾长期保留 `remember_choice` / `skip_installed` / `tray_icon` / `window_width` / `window_height` 等从未被读取的字段，以及 `visibleBrowsers()`、`truncMiddle()`、`reopenCh` / `goHandleReopen()` 等死代码，`i18n.appleLocale()` 也曾是空实现——均已在开源前清理（`appleLocale` 现已真正实现）。当前仅存 `Browser.IsCustom`（`mergeDetected` 会保留但无 UI 能创建，见 ISSUES I-15）尚待处理，不要再新增此类预留。
- **DRY**：新增逻辑前先搜是否已存在。当前已知重复：`handleURL` vs `openPicker` vs `decideAction`（三份 URL 决策，见 ISSUES I-1）、`buildInfoPlist` vs `buildInfoPlistWithIcon`（两份 plist 模板，见 ISSUES I-16）。
- **SRP**：[settings.go](settings.go) 已 **1039 行**，承担了三个标签页 + 对话框 + 工具函数 + i18n 反查 + 窗口单例管理。新增设置项时优先考虑拆文件（见 ISSUES I-23）而非继续追加。
- **用户可见的失败必须提示，不能静默吞掉。** 规则保存失败（校验不过或写盘失败）要就地显示错误，别 `_ = SaveConfig(cfg)` 就当没事——用户会以为保存成功，下次打开规则就没了。
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
