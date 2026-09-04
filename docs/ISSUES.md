# 现存问题清单

> 生成日期：2026-07-08 · 基线：`master` @ `52d6a8b` · 方法：逐文件通读 + `go vet` + `go test ./...`（均通过）
>
> 分级：**P0** = 用户可感知的功能性缺陷 · **P1** = 正确性/体验问题 · **P2** = 工程质量与可维护性

统计：P0 × 5，P1 × 9，P2 × 13。

---

## P0 —— 功能性缺陷

### I-1 · URL 决策逻辑存在三份副本，且测试测的是副本而非生产代码

**定位**：[main.go:227](main.go#L227) `handleURL()` · [urlhandler_darwin.go:126](urlhandler_darwin.go#L126) `openPicker()` · [picker_decision_test.go:7](picker_decision_test.go#L7) `decideAction()`

「匹配规则 → 命中则启动 / 未命中且关闭弹窗则用默认浏览器 / 否则弹选择器」这段逻辑被独立实现了三遍：命令行路径一份、Apple Event 路径一份、测试里又抄了一份。

**后果**：
- 测试对 `ShowPickerOnMiss` 分支的覆盖是**假覆盖**——改坏生产代码测试也不会红。
- 任何规则决策的修改必须记得改两处生产代码，漏一处就会出现「命令行行为与真实点击行为不一致」。

**建议**：抽取纯函数

```go
type URLAction int
const (ActionLaunch URLAction = iota; ActionPicker)
// decideURLAction 是 URL 处理的唯一决策入口，无副作用。
func decideURLAction(rawURL string, cfg *Config) (URLAction, *Browser)
```

`handleURL`、`openPicker`、测试三处共用。**这是本清单中优先级最高的一项**，它同时消除 DRY 违规和测试失真。

---

### I-2 · 卸载时无法还原原默认浏览器（`prev_default_browser` 被静默清空）

**定位**：[install_darwin.go:121](install_darwin.go#L121) `rememberPreviousDefault()` · [config.go:171](config.go#L171) `SaveConfig()` · [settings.go:554](settings.go#L554) `installBtn`

`Install()` 内部调用 `rememberPreviousDefault()`，后者用 `InitConfig()` **重新从磁盘读一个新的 `Config` 对象**，写入 `PrevDefaultBrowser` 后落盘。而设置窗口持有的是启动时加载的另一个 `cfg` 实例，其 `PrevDefaultBrowser` 仍为空字符串。

用户在设置界面点「设为默认浏览器」后，只要再动任何一个设置项（语言、默认浏览器、倒计时秒数、弹窗开关⋯⋯每一项的 `OnChanged` 都会 `SaveConfig(cfg)`），磁盘上的 `prev_default_browser` 就被空值覆盖。

**后果**：`--uninstall` 时 `restore` 回退到硬编码的 `com.apple.safari`，用户原本的默认浏览器（如 Arc）不会被还原。

**建议**：`Install()` / `Uninstall()` 接受调用方传入的 `*Config`，不再内部 `InitConfig()`。CLI 路径把 `main()` 里的 `cfg` 传进去，GUI 路径把设置窗口的 `cfg` 传进去。全进程只维护一个 `Config` 实例。

---

### I-3 · 从已安装的 `.app` 内重新安装会丢失应用图标

**定位**：[install_darwin.go:338](install_darwin.go#L338) `getIconPath()`

```go
assetDir := filepath.Join(filepath.Dir(exePath), "assets")
iconPaths := []string{
    filepath.Join(assetDir, "BrowserSwitch.icns"),
    filepath.Join(assetDir, "BrowserSwitch.icns"),  // ← 两项完全相同
}
```

图标从「可执行文件同级的 `assets/` 目录」查找。安装后可执行文件位于 `~/Applications/Browser Switch.app/Contents/MacOS/browser-switch`，该目录下没有 `assets/`，`getIconPath()` 返回空 → 走 `buildInfoPlist()`（无图标版本）→ 生成的 `.app` 没有图标。

用户在设置界面点「设为默认浏览器」（内部即 `Install()`）就会触发这条路径。

**次要问题**：`iconPaths` 两个元素字符串完全相同，是复制粘贴残留。

**建议**：用 `//go:embed assets/BrowserSwitch.icns` 把图标编译进二进制，安装时直接写出。彻底摆脱运行时路径依赖。

---

### I-4 · 安装后 GUI 始终以英文启动（系统语言检测在 `.app` 场景下必然失效）（已解决）

> **✅ 已修复**：`appleLocale()` 已实现为 `exec.Command("defaults", "read", "-g", "AppleLocale")`，返回值经 `mapLocale` 归一（`zh_CN`→`zh-CN`、`en_US`→`en`）。已安装的 `.app` 现能正确跟随系统区域语言。此处采用 `defaults` 而非建议中的 cgo `CFLocaleCopyPreferredLanguages()`——避免为一次语言检测扩大 cgo 面（KISS）。

**定位**：[i18n/i18n.go:69](i18n/i18n.go#L69) `detectLanguage()` · [i18n/i18n.go:122](i18n/i18n.go#L122) `appleLocale()`

```go
func detectLanguage() string {
    for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} { ... }
    if stdLang := appleLocale(); stdLang != "" { return stdLang }  // 永远返回 ""
    return "en"
}

func appleLocale() string {
    // 仅在 darwin 上此调用有意义；出于简洁不走 cgo，仅读 LANG 兜底
    return ""   // ← 函数体是空实现
}
```

macOS 通过 LaunchServices 启动的 `.app` **不继承 shell 环境变量**，`LANG` / `LC_ALL` 通常均未设置。因此已安装的应用（也就是绝大多数用户实际使用的形态）永远命中 `return "en"`。

只有从终端运行 `./browser-switch` 时才会正确检测到中文——即开发者自测时看不到这个 bug。

**后果**：8 个语言包中的 7 个在真实使用场景下形同虚设，中文用户必须手动去设置里选语言。

**建议**：`appleLocale()` 真正实现——两个方案：
1. cgo 调用 `CFLocaleCopyPreferredLanguages()`（最准确，项目本就依赖 CoreFoundation）
2. `exec.Command("defaults", "read", "-g", "AppleLanguages")`（无 cgo，但要多一次子进程）

推荐方案 1。

---

### I-5 · 配置写入非原子，进程中断会损坏配置文件

**定位**：[config.go:171](config.go#L171)

```go
func SaveConfig(cfg *Config) error {
    ...
    return os.WriteFile(configPath, data, 0644)   // 先 truncate 再写
}
```

`os.WriteFile` 以 `O_TRUNC` 打开，先清空再写。写入过程中崩溃/断电会留下**空文件或半截 JSON**。

[config.go:152](config.go#L152) 的「处理空文件或仅包含空白字符的文件」分支正是为此打的补丁——它掩盖了症状（重建默认配置），但代价是**用户的全部规则与收藏静默丢失**。半截 JSON 的情况连这个补丁都救不了，`json.Unmarshal` 报错 → `main()` 直接 `os.Exit(1)`，应用完全无法启动。

考虑到 `SaveConfig` 在设置界面里被极其频繁地调用（每次勾选、每次移动收藏项都写一次全量 JSON），命中概率不算低。

**建议**：写临时文件 + `os.Rename` 原子替换：

```go
tmp := configPath + ".tmp"
if err := os.WriteFile(tmp, data, 0644); err != nil { return err }
return os.Rename(tmp, configPath)
```

同时把 `json.Unmarshal` 失败降级为「备份损坏文件 + 重建默认配置」，而非退出进程。

---

## P1 —— 正确性与体验问题

### I-6 · 冷启动 500ms 竞态：慢机器上设置窗口会先闪现，随后才弹出选择器

**定位**：[urlhandler_darwin.go:176](urlhandler_darwin.go#L176)

```go
go func() {
    time.Sleep(500 * time.Millisecond)
    fyne.Do(showSettingsInMaster)   // 500ms 内没 URL → 认定用户主动打开 App
}()
```

500ms 是经验值。系统负载高、冷缓存、首次 Gatekeeper 校验时，Apple Event 完全可能晚于 500ms 送达。此时 `decided` 已被设置窗口 CAS 抢占 → URL 走 `openPicker` 的「已在处理其它 URL」分支 → 另开一个独立选择器窗口。

**用户看到的**：点了个链接，先跳出设置窗口，再跳出选择器，关掉选择器后设置窗口还赖着不走。

**建议**：不用定时器猜。macOS 在 Apple Event 到达前会先发 `kAEOpenApplication`（`aevt/oapp`）；注册该事件处理器即可确定性区分「用户点 App」与「系统投递 URL」。或者退一步：把 500ms 提到 1.5s，并在收到 URL 时若设置窗口已显示则关闭它。

---

### I-7 · 浏览器超过 8 个时，展开「更多」后卡片被窗口裁剪

**定位**：[picker.go:113](picker.go#L113) `resizePicker()`

```go
func resizePicker(w fyne.Window, n int) {
    if n > 8 { n = 8 }          // 高度按最多 8 个算 → 最多 2 行
    rows := (n + 3) / 4
    w.Resize(fyne.NewSize(560, float32(210+rows*118)))
}
```

而 [picker.go:246](picker.go#L246) 的网格是 `container.NewGridWithColumns(4, objs...)`，`objs` 是**全部**浏览器。9 个浏览器 → 网格 3 行，窗口只按 2 行高。第三行不可见也不可滚动。

macOS 上装 9 个以上能处理 http 的 `.app` 并不罕见（误检测的 WebView 应用会推高这个数）。

**建议**：`rows := (n + 3) / 4` 不设上限，改为给网格套 `container.NewVScroll` 并限制窗口最大高度。

---

### I-8 · Firefox 默认 profile 识别失效（现代 `profiles.ini` 格式）

**定位**：[profiles_darwin.go:119](profiles_darwin.go#L119) `detectFirefoxProfiles()`

解析器只在 `[ProfileN]` 段内识别 `Default=1`。而现代 Firefox 把默认 profile 记在独立的 `[Install<HASH>]` 段里：

```ini
[Profile0]
Name=default-release
Path=Profiles/xxxx.default-release

[Install4F96D1932A9F858E]
Default=Profiles/xxxx.default-release
```

代码遇到 `[Install...]` 时命中 `case strings.HasPrefix(line, "[")` → `flush()` 清空 `cur`；随后的 `Default=Profiles/...` 设置了 `cur.isDef=true` 但 `cur.hasName` 为 `false`，最终 `flush()` 直接丢弃。

**后果**：所有 profile 的 `Kind` 都是 `"profile"`，没有一个被标记为 `"default"`。排序退化为纯字母序，菜单里也不会显示「默认」标签。

**建议**：先收集 `[Install*]` 段的 `Default=<path>`，再与各 `[ProfileN]` 的 `Path=` 比对来确定默认项；同时兼容旧格式的 `Default=1`。

---

### I-9 · 多 profile 浏览器的左键点击与 ⌘N 语义不一致 — ✅ 已解决（2026-09-03）

**定位**：[picker.go:217](picker.go#L217) vs [picker.go:188](picker.go#L188)

```go
// 卡片点击：有 profile → 弹账户菜单
if len(profs) > 0 { tapAction = func() { showProfileMenu(...) } }

// ⌘N / 数字键：始终直接打开，跳过账户菜单
shortcutActions[i] = func() { open(b, rememberChk.Checked) }
```

同一张卡片，鼠标点击弹出账户选择菜单，按 `⌘1` 却直接用默认 profile 打开。两条路径对「选择这个浏览器」的语义定义不同。

**建议**：统一。要么 ⌘N 也弹菜单（键盘用户可继续用方向键选择），要么给卡片加一个显式的「▾」区域触发菜单、卡片主体直接打开。后者体验更好。

**现状**：已统一到第一条 —— 卡片点击与 ⌘N 现在共用同一个 `shortcutActions[i]`。是否弹菜单由新增的 `profilesNeedChoice(profiles)` 决定：只有存在多个**真实** profile 时才必须选账户（无痕是合成项，不算），此时点击与 ⌘N 都弹菜单；只有「默认 + 无痕」时两者都直接打开，无痕走右键菜单，卡片副标题给出「⌘N · 右键选无痕」提示。

---

### I-10 · 「记住选择」生成的规则优先级高于手动规则，导致手动规则无法覆盖

**定位**：[picker.go:352](picker.go#L352) `priority: 100` vs [settings.go:395](settings.go#L395) `priorityEntry.SetText("50")`

自动规则 `priority=100`，手动新增规则默认 `priority=50`。规则按优先级降序匹配，首个命中即返回。

**后果**：用户某次误勾了「记住此域名的选择」，之后在设置里为同一域名新建规则想改用别的浏览器——不生效，且界面上没有任何提示告诉他为什么。规则列表虽按优先级排序，但**不显示优先级数值**，用户无从判断。

**建议**：三选一（可叠加）
1. 手动规则默认优先级改为 `100`，自动规则降为 `50`（更符合直觉：显式配置 > 隐式记忆）
2. 规则列表显示优先级数值与来源徽章（`auto` / `user`）
3. 新增规则时若检测到同 pattern 的已有规则，提示并提供「替换」选项

---

### I-11 · 规则只能新增和删除，无法编辑、禁用或调整优先级

**定位**：[settings.go:283](settings.go#L283) `buildRulesTab()`

`Rule` 结构体有 `Enabled`、`Priority`、`Comment` 三个字段，规则引擎也完整支持它们（[rules.go:43](rules.go#L43) 跳过 `!Enabled` 的规则），但设置界面：
- 不渲染 `Priority`、不渲染 `Comment`
- 没有 `Enabled` 开关
- 没有编辑入口，改一个字符也得删了重建

用户唯一的选择是手工编辑 `~/.config/browser-switch/config.json`。

**建议**：规则行加 `Enabled` 开关 + 点击行进入编辑对话框（复用 `showAddRuleDialog` 的表单，传入已有 `Rule`）。

---

### I-12 · 非 ASCII 浏览器名会让首字母头像渲染出非法 UTF-8

**定位**：[gui.go:176](gui.go#L176)

```go
ch := "?"
if name != "" {
    ch = strings.ToUpper(name[:1])   // 按字节切片，不是按 rune
}
```

`name[:1]` 取的是**第一个字节**。浏览器名为「夸克浏览器」「花瓣浏览器」等中文名时，取到的是 UTF-8 三字节序列的首字节，构成非法 UTF-8。

只在 `browserIconPath()` 取不到图标时才走这条路径，所以平时不易触发——但正是那些非主流的国产浏览器最可能取不到图标。

**修复**：

```go
if r := []rune(name); len(r) > 0 {
    ch = strings.ToUpper(string(r[0]))
}
```

---

### I-13 · 浏览器图标缓存永不失效

**定位**：[icons_darwin.go:21](icons_darwin.go#L21)

```go
if fi, err := os.Stat(out); err == nil && fi.Size() > 0 {
    return out   // 只判断存在且非空
}
```

缓存 key 是 bundle ID，浏览器版本更新换了图标后不会刷新。缓存在 `/tmp` 下，靠系统重启清理——现代 macOS 用户几个月不重启很常见。

**建议**：比较 `.icns` 源文件的 `ModTime` 与缓存 PNG 的 `ModTime`，源较新则重新生成。

---

### I-14 · Apple Event 队列满时静默丢弃 URL

**定位**：[urlhandler_darwin.go:78](urlhandler_darwin.go#L78)

```go
select {
case urlEventCh <- u:
default:            // ← 通道满 → 直接丢弃，无日志无提示
}
```

`urlEventCh` 缓冲为 8。cgo 回调不能阻塞（否则卡死 Apple Event 分发线程），所以 `default` 分支的存在是合理的，但**静默丢弃**不合理——用户点了链接，什么都没发生，也没有任何提示。

同一问题也出现在 `reopenCh`（缓冲 4）。附带一提：`reopenCh` 与 `goHandleReopen()` **从未被任何代码消费**，是为常驻模式预埋的死代码。

**建议**：丢弃时至少 `fmt.Fprintf(os.Stderr, ...)` 记录，或把缓冲提到 64（URL 是短字符串，内存代价可忽略）。

---

## P2 —— 工程质量与可维护性

### I-15 · 死代码与从未被读取的配置字段（YAGNI 违规）（部分已解决）

> **✅ 已清理**：`Config.RememberChoice` / `SkipInstalled` / `TrayIcon` / `WindowWidth` / `WindowHeight`、`visibleBrowsers()`、`truncMiddle()`、`var _ = fmt.Sprintf`、`reopenCh` / `goHandleReopen()` 均已删除；`i18n.appleLocale()` 已实现（见 I-4）。**仅剩 `Browser.IsCustom` 待处理**——`mergeDetected()` 会保留它，但没有任何 UI 能创建自定义浏览器。

| 符号 | 位置 | 状态 |
|------|------|------|
| `Config.RememberChoice` | [config.go:58](config.go#L58) | 声明后从未被读取 |
| `Config.SkipInstalled` | [config.go:61](config.go#L61) | 同上 |
| `Config.TrayIcon` | [config.go:62](config.go#L62) | 同上（`fyne.io/systray` 已在依赖中，但无托盘实现） |
| `Config.WindowWidth` / `WindowHeight` | [config.go:63](config.go#L63) | 同上（窗口尺寸硬编码在 `resizePicker`） |
| `visibleBrowsers()` | [picker.go:322](picker.go#L322) | 定义后无调用点 |
| `truncMiddle()` | [gui.go:235](gui.go#L235) | 定义后无调用点 |
| `var _ = fmt.Sprintf` | [gui.go:242](gui.go#L242) | 为保住未使用的 `fmt` import 而存在的 hack |
| `i18n.appleLocale()` | [i18n/i18n.go:122](i18n/i18n.go#L122) | 函数体 `return ""`（同时是 I-4 的成因） |
| `reopenCh` / `goHandleReopen()` | [urlhandler_darwin.go:73](urlhandler_darwin.go#L73) | 注册了处理器但无消费者 |
| `Browser.IsCustom` | [config.go:18](config.go#L18) | `mergeDetected()` 会保留它，但没有任何 UI 能创建自定义浏览器 |

这些字段一旦出现在用户的 `config.json` 里就成了事实上的公开契约，删除时需考虑向后兼容（JSON 反序列化会忽略未知字段，所以删掉是安全的）。

---

### I-16 · `buildInfoPlist` 与 `buildInfoPlistWithIcon` 是两份复制粘贴（DRY）

**定位**：[install_darwin.go:231](install_darwin.go#L231) 与 [install_darwin.go:277](install_darwin.go#L277)

两个函数除了后者多 4 行 `CFBundleIconFile` 键之外**完全相同**（含 43 行内嵌模板）。`urlTypes` 块在两处逐字重复。

**建议**：单函数 + `iconFile string` 参数（空串则不输出该键）。或干脆用 `text/template`。

---

### I-17 · `en-US.json` 与 `en.json` 字节级完全相同（已解决）

> **✅ 已修复**：已删除 `en-US.json`，并从 `SupportedLanguages()` / `LanguageNativeName()` 移除 `en-US`。`mapLocale` 的 `strings.Cut(locale, "_")` 通用逻辑天然把 `en_US` 归一到 `en`，无需额外映射。语言选择器不再出现重复的「English (US)」。

两个文件曾均为 8678 字节、141 个 key、内容一致。`SupportedLanguages()` 把它们都列进语言选择器，用户会看到「English」和「English (US)」两个完全等价的选项。

**建议**：删除 `en-US.json`，从 `SupportedLanguages()` 移除；在 `mapLocale()` 中把 `en_US` 映射到 `en`。

---

### I-18 · 版本号硬编码在 7 个语言包里

**定位**：`i18n/locales/*.json` 中的 `"app.version": "Version 1.0.0"`

版本号本不是需要翻译的内容，却被塞进了翻译 key。发一个 v1.0.1 需要改 7 个 JSON 文件，且必然有人漏改。

**建议**：`constants.go` 加 `const AppVersion = "1.0.0"`（或构建时 `-ldflags "-X main.AppVersion=..."` 注入 git tag），语言包只保留 `"app.version_label": "Version %s"` 这样的模板。

---

### I-19 · `constants.go` 命名在单 App 迁移后已失真；`install_darwin.go` 存在影子常量

**定位**：[constants.go:1](constants.go#L1) · [install_darwin.go:46](install_darwin.go#L46)

- `HelperExecName = "browser-switch"` —— 迁移到单 App 后，这是**主 App** 的可执行文件名，早已与 "Helper" 无关，却仍在 `buildMainApp()` 中被引用。
- `HelperName` / `HelperBundleID` 现在只被 `cleanupLegacyApps()` 使用（清理旧架构残留），语义应改名为 `LegacyHelperName` / `LegacyHelperBundleID`。
- `install_darwin.go:46-52` 定义了 `appName = AppName`、`appBundleID = AppBundleID` 等一组小写影子常量，纯粹是间接层，没有任何收益。

---

### I-20 · Makefile 目标失效且描述与项目不符（已解决）

> **✅ 已修复**：`Makefile` 重写为 `.PHONY: build test vet clean app dmg`，删除了必然编译失败的 `build-gtk4` / `build-windows`，补齐 `test` / `vet` / `clean`，并新增 `app` / `dmg`（分别调用 `scripts/build-app.sh` / `scripts/build-dmg.sh`）。`build` 目标注释已更正为 macOS。

**定位**：[Makefile:1](Makefile#L1)

```makefile
.PHONY: build clean install uninstall run test deps   # 声明了 7 个
# 实际只定义了 build / build-gtk4 / build-windows
# Linux GTK3 (default)   ← 注释说这是 Linux 构建
build:
	CGO_ENABLED=1 go build $(GOFLAGS) -o $(BINARY) .
```

- `make test` / `make clean` / `make deps` / `make install` 全部报 "No rule to make target"，而旧版 README 曾指导用户执行 `make test`。
- `build-gtk4`（`-tags gtk4`）与 `build-windows`（MinGW 交叉编译）在没有 `*_linux.go` / `*_windows.go` 的情况下必然编译失败。
- 默认 `build` 目标的注释写着 "Linux GTK3 (default)"，实际产出的是 macOS 二进制。

**建议**：删掉 `build-gtk4` / `build-windows`，补齐 `test` / `vet` / `clean` 目标，修正注释。

---

### I-21 · 25MB 编译产物被 git 跟踪（已解决）

> **✅ 已修复**：核实 `browser-switch` 二进制实际**未被 git 跟踪**（`git ls-files` 无记录），无需 `git rm --cached`。`.gitignore` 已把重命名前的旧二进制名更新为 `browser-switch`，并补充 `dist/`、`.DS_Store`。

`browser-switch`（约 25 MB）曾被担心进入版本控制；`.gitignore` 里当时写的是重命名前的旧二进制名，重命名后没同步更新。

每次编译都会产生一个巨大的二进制 diff，仓库会迅速膨胀。

**建议**：`.gitignore` 加 `browser-switch`，并 `git rm --cached browser-switch`。⚠️ 该操作涉及 git 索引变更，需用户确认后执行。

---

### I-22 · README 标注 MIT License 但仓库无 LICENSE 文件（已解决）

已补齐仓库根目录 `LICENSE`（Apache-2.0 全文），并将 4 个 README 的协议 badge 与 License 段统一改为 Apache-2.0。

---

### I-23 · `settings.go` 1039 行，职责过载（SRP 违规）

**定位**：[settings.go](settings.go)

单文件承担了：三个标签页构建 + 添加/编辑规则对话框 + 通用小工具（`badgeNum` / `iconButton` / `ternary` / `indexOf` / `thinSepObj`）+ i18n 匹配模式名的正反查表 + `mergeDetected` 业务逻辑 + **窗口单例管理**（`settingsWin` / `ruleDialogWin` / `reloadConfigInto`）。

它是全项目最大的文件（**1039 行**，第二名 `main.go` 415 行）。`modeDisplayName` / `modeFromDisplayName` 这对互逆函数尤其危险——两处 `switch` 必须同步维护，漏一个 case 就静默退化为 `MatchExact`。新增 `urlequal` 模式时新增了第三个函数 `modeHintText`，三处 `switch` 现在都要同步，风险进一步上升。

**建议**：拆为 `settings_browsers.go` / `settings_rules.go` / `settings_general.go`；小工具移入 `gui.go`；模式名映射改为单一 `var modeMeta = []struct{ mode MatchMode; key string }{...}` 表驱动。

---

### I-24 · 无 GUI 测试、无 cgo 层测试、无集成测试

现有 4 个测试文件全部只覆盖纯函数（规则匹配、pattern 校验、模式推测、`removeRule`）。完全未覆盖：

- `DetectBrowsers()` / `DetectProfiles()`（可通过注入 fake 文件系统根路径来测）
- `Install()` / `Uninstall()`（可用 `t.TempDir()` 替换 `~/Applications` 来测 `.app` 结构生成）
- `matchWildcard()` 的正则转义（`[`、`{` 等元字符）
- `extractDomain()` 的边界（IPv6、端口、无 scheme）

另：[remember_flow_test.go:14](remember_flow_test.go#L14) 直接给包级全局 `configPath` 赋值来隔离，这让该测试**不能与其他写配置的测试并行**。

---

### I-25 · 冷启动性能：每次点击链接都要跑大量子进程

每次链接点击都是一次冷启动（见 PRD「关键设计决策」），而以下操作都在启动路径上：

| 操作 | 位置 | 代价 |
|------|------|------|
| `DetectBrowsers()`（仅首次运行） | [browsers_darwin.go:62](browsers_darwin.go#L62) | 每个 `.app` 一次 `plutil` 子进程，4 个目录可能 100+ 次 |
| `DetectProfiles()` × 全部收藏浏览器 | [picker.go:195](picker.go#L195) | 每个浏览器读一次 `Local State` JSON / `profiles.ini` |
| `browserIconPath()` 缓存未命中 | [icons_darwin.go:26](icons_darwin.go#L26) | 每个浏览器 1 次 `plutil` + 1 次 `sips` 子进程 |
| 正则编译 | [rules.go](rules.go) `matchPattern` | 每条 **regex** 规则每次匹配仍 `regexp.Compile`；**wildcard 已加 `sync.Map` 缓存**（`wildcardRegexp`） |

首次启动或图标缓存被清空后，弹出选择器前要串行跑十几个子进程。

**建议**：
- 图标提取改并发（`errgroup`），或用 `//go:embed` 预置常见浏览器图标
- **regex 模式补上同样的缓存**（照 `wildcardRegexp` 的写法即可，`wildcardCache` 已是现成模板）
- `DetectProfiles` 结果在单次进程内缓存（当前 `buildBrowsersTab` 每次 refresh 都重新读盘）

---

### I-26 · 签名方式已弃用，且未经 Apple 公证

**定位**：[install_darwin.go:325](install_darwin.go#L325)

```go
exec.Command("/usr/bin/codesign", "--force", "--deep", "--sign", "-", appPath)
```

- `--deep` 已被 Apple 标记为 deprecated，不应用于签名（只应用于验证）。
- Ad-hoc 签名 (`--sign -`) + 手动 `xattr -dr com.apple.quarantine` 可以绕过本机 Gatekeeper，但分发给其他用户时仍会被拦截。

**建议**：分发前接入 Developer ID 签名 + `notarytool` 公证流程。

---

### I-27 · 匹配日志未国际化

**定位**：[rules.go:59](rules.go#L59)

```go
MatchLog: fmt.Sprintf("Rule '%s' (%s: %s) → %s", ...)
MatchLog: fmt.Sprintf("no rule matched for host: %s", host)
MatchLog: fmt.Sprintf("failed to parse URL: %v", err)
```

`MatchLog` 是硬编码英文，却被 `--test` 命令通过 `i18n.T("cli.output.match_detail")` 这个已翻译的模板打印出来。中文用户会看到「匹配详情：no rule matched for host: example.com」这样的半中半英输出。

---

## 附：修复优先级建议

| 批次 | 内容 | 理由 |
|------|------|------|
| **第一批** | I-1、I-2、I-5 | 数据正确性与测试有效性，是后续所有改动的地基 |
| **第二批** | I-4、I-3 | 用户可感知度最高的两个缺陷（永远英文界面 / 图标消失） |
| **第三批** | I-10、I-11、I-9 | 规则系统的可用性短板，直接决定产品是否"好用" |
| **第四批** | I-6、I-7、I-8、I-12、I-13、I-14 | 边界与体验打磨 |
| **持续** | I-15 ~ I-27 | 随手清理；I-21、I-22 在开源前必须完成（均已完成）。本轮另已解决 I-4、I-17、I-20，并部分解决 I-15 |
