# Browser Switch 产品需求文档（PRD）

> 版本：v1.0.0 · 最后校对：2026-07-08 · 校对方式：逐行比对 `master` 分支实际代码实现
>
> 本文档描述**已实现**的产品行为。未实现的构想统一收敛到 [docs/ROADMAP.md](docs/ROADMAP.md)，已知缺陷收敛到 [docs/ISSUES.md](docs/ISSUES.md)。

---

## 1. 概述

Browser Switch 是一款 macOS 桌面工具。注册为系统默认浏览器后，它拦截所有 HTTP/HTTPS 链接，根据用户规则自动分发到不同浏览器；无规则匹配时弹出选择器供临时决定。

**核心价值**：解决「私用 Chrome、工作用 Edge、开发用 Firefox」的多浏览器并行场景，消除手动复制粘贴 URL 的摩擦。

**目标用户**：同时维护多个浏览器 / 多个工作账户的开发者、顾问、多租户运维人员。

**非目标**：不做浏览器本身、不做书签同步、不做跨平台（当前）。

---

## 2. 功能需求

### F1. 默认浏览器注册与 URL 拦截

**用户故事**：作为用户，我希望把 Browser Switch 设为系统默认浏览器，使所有链接先经过它。

**实现方式**（单 App 架构，见 [install_darwin.go](install_darwin.go)、[urlhandler_darwin.go](urlhandler_darwin.go)）：

- 入口：`--install` CLI 命令 / 设置界面「设为默认浏览器」按钮 / `--installer` 安装向导 UI
- 在 `~/Applications/` 下生成**单个** `Browser Switch.app`（bundle ID `com.browserswitch.app`）：
  - 将当前可执行文件复制到 `Contents/MacOS/browser-switch`
  - 写入 `Info.plist`，声明 `CFBundleURLTypes` = `http` + `https`
  - 若可执行文件同级目录存在 `assets/BrowserSwitch.icns`，一并写入 `Contents/Resources/AppIcon.icns`
- Ad-hoc 代码签名（`codesign --force --deep --sign -`）+ 清除 quarantine 属性 + `lsregister -f` 注册
- 通过 CoreServices 的 `LSSetDefaultHandlerForURLScheme`（cgo）设置 http/https 默认处理器
- 若设置后校验仍非默认，自动打开「系统设置 > 通用」提示用户手动确认
- 升级路径：`cleanupLegacyApps()` 删除旧双 App 架构残留的 `Browser Switch Helper.app`

**URL 接收**：应用通过 `AEInstallEventHandler(kInternetEventClass, kAEGetURL, ...)` 注册 Carbon Apple Event 处理器，cgo 回调把 URL 送入 Go channel，由 Fyne 主循环消费。**不依赖命令行参数，也不依赖 AppleScript 转发器。**

**卸载**（`--uninstall`）：
- 将 http/https 默认处理器还原为 `prev_default_browser`（安装时记录），无记录时回退 `com.apple.safari`
- 清空 `prev_default_browser` 记录
- 删除 `Browser Switch.app` 及旧 Helper 残留

**查询**：`--check-default` 输出当前默认处理器 bundle ID 与是否为本应用。

---

### F2. 浏览器检测

**用户故事**：作为用户，我希望工具自动发现电脑上安装的所有浏览器。

**实现方式**（[browsers_darwin.go](browsers_darwin.go)）：

1. **主路径**：扫描 `~/Applications`、`/Applications`、`/Applications/Utilities`、`/System/Applications` 下的 `.app`，用 `plutil -convert json` 解析 `Info.plist`，凡在 `CFBundleURLTypes` 中声明 `http` 或 `https` scheme 者判定为浏览器。跳过本应用自身（`com.browserswitch.app`）。
2. **兜底路径**：`knownMacBrowsers` 硬编码列表（Safari、Chrome、Firefox、Edge、Brave、Arc、Opera、Vivaldi、Chromium、Edge Dev、Tor、Zen），按 bundle ID + 固定路径补全。Safari 不在 plist 中声明 http scheme，必须靠此兜底。
3. 结果按 Name 升序排序。

**数据模型**：`Browser{ID: bundleID, Name: 包目录名, Exec: bundleID, Desktop: .app 路径, Icon: bundleID}`

**触发时机**：
- 首次运行（`InitConfig()` 检测到无配置文件或空文件）自动检测并写入配置，第一个浏览器设为默认
- 设置界面「重新扫描」按钮（异步执行，UI 显示 Activity 指示器）
- `--list-browsers` CLI 命令

**重新扫描语义**：`mergeDetected()` 用新检测结果整体替换，仅保留 `IsCustom=true` 的用户自定义条目。

---

### F2.5 浏览器启动与新窗口打开

**用户故事**：作为用户，我希望某些链接（如需要登录的工作后台）强制在新窗口打开，而不是复用已有窗口的标签页。

**实现方式**（[browsers_darwin.go](browsers_darwin.go)、[profiles_darwin.go](profiles_darwin.go)）：

两个入口都带 `newWindow bool` 参数：

```go
LaunchBrowser(b Browser, url string, newWindow bool) error
LaunchBrowserProfile(b Browser, p Profile, url string, newWindow bool) error
```

- **普通启动**：`open -b <bundleID> <url>`，不加 `-n` 以复用已开窗口
- **带 profile 启动**：必须直接执行包内二进制，因为 `open -b` 在浏览器已运行时会丢弃 `--profile-directory` 等参数。路径由 `appExecPath()` 解析 `CFBundleExecutable` 得到
- 用 `cmd.Start()` 而非 `Run()`，选择器不等浏览器退出

**`newWindow` 不能只用 `open -n`**：Chrome / Edge / Safari 都是**单实例应用**，进程内用锁保证唯一，`open -n` 会被直接忽略、URL 照旧进已有窗口的新标签页。`launchNewWindow()` 因此按类型分派：

| 浏览器 | 方式 |
|--------|------|
| Chromium 系（Chrome/Edge/Brave/Vivaldi/Opera） | 执行二进制 + `--new-window` |
| Firefox | 执行二进制 + `--new-window` |
| Safari | AppleScript `make new document`（**它没有任何 CLI 开关**） |
| 其它 | 退回 `open -n`（单实例应用可能无效） |

全部失败时**降级为普通打开**——宁可开在已有窗口，也不能打不开。AppleScript 拼串走 `appleScriptString()` 转义反斜杠与引号。

**规则绑定**：`Rule.OpenInNewWindow`（JSON `open_in_new_window`）为 true 时，规则命中路径（`handleURL`、`openPicker`）会把该标志透传给 `LaunchBrowser`。选择器手动点击与倒计时回退一律传 `false`。

---

### F3. URL 规则匹配引擎

**用户故事**：作为用户，我希望为不同域名设置不同浏览器，点击时自动路由。

**实现方式**（[rules.go](rules.go)）：

| 模式 | 匹配对象 | 示例 |
|------|----------|------|
| `exact` | host 全等 | `github.com` |
| `urlequal` | 完整网址全等（含 scheme / 路径 / 查询串），忽略大小写 | `https://github.com/a/b` |
| `wildcard` | host **或**完整 URL（`*` `?` 转译为正则，结果带缓存） | `*.google.com`、`*/settings` |
| `regex` | host **或**完整 URL | `.*\.(test\|staging)\..*` |
| `contains` | host **或**完整 URL 子串（≈ `*p*`） | `login` |
| `prefix` | host **或**完整 URL 前缀（≈ `p*`） | `dev.`、`https://` |
| `suffix` | host **或**完整 URL 后缀（≈ `*p`） | `.cn`、`.pdf` |

- 匹配前将规则按 `priority` **降序**排序（副本排序，不改动原配置），首个命中的启用规则即返回
- `enabled=false` 的规则跳过
- 命中规则但其 `browser` ID 在配置中找不到对应浏览器时，继续尝试下一条规则
- **www 归一化**：先用原始 host 匹配一次，未中且 host 带 `www.` 前缀时用剥离后的 host 再匹配一次。因此「记住选择」写入的 `example.com` 也能命中 `www.example.com`
- pattern 与 host 在比较前均转为小写
- **`exact` / `urlequal` 是精确比较，只看各自的目标串；其余模式对 host 与完整 URL 各试一次（`||`）。** 这是必须的：`.pdf`、`https://`、`*/settings` 根本不出现在 host 里
- `contains` / `prefix` / `suffix` 是 `wildcard` 的**快捷方式**，面向不熟悉 glob/正则的用户，**不解释** `*` 与 `?`（按字面量处理）

**辅助能力**：
- `ValidatePattern(pattern, mode)`：空 pattern 报错；`regex` 模式额外校验语法可编译；`wildcard` 转译后校验正则可用
- `ValidateRuleInput(pattern, mode, browserID, profileID, rules, excludeID)`：保存前的**语义**校验，捕捉"语法合法但保存后一定不生效"或"毫无意义"的写法——`urlequal` 缺 `://`、`exact` 含 scheme 或路径、快捷方式里出现 `*`/`?`、完全重复的规则（**同 pattern + 同 mode + 同浏览器 + 同账户**才算重复）。`excludeID` 用于编辑场景排除自身
- `SuggestMatchMode(pattern)`：含 `\ [ ] ( ) |` → `regex`；含 `://` → `urlequal`；含 `* ?` → `wildcard`；否则 `exact`。用于添加规则对话框的实时模式推测
- `--test <url>`：只输出匹配结果与 MatchLog，不启动浏览器

---

### F4. 浏览器选择器弹窗

**用户故事**：对于未设规则的链接，我希望弹出选择器临时决定。

**实现方式**（[picker.go](picker.go)）：

**布局**：
- 顶部满宽地址栏（🌐 图标 + 可截断的 URL 文本）
- 中部浏览器卡片网格，每行最多 4 列
  - 浏览器总数 ≤ 4：全部直接展示
  - 浏览器总数 > 4：只展示前 3 个 + 一张「更多」卡片；点击「更多」展开完整列表并按行数放大窗口（最多 2 行 / 8 个的高度上限）
  - 默认浏览器的卡片高亮选中（蓝色描边）
- 倒计时区域（文本 + 细进度条）
- 底部：「记住此域名的选择」复选框 + 复制 URL 按钮 + 齿轮（打开设置，复用当前 app 实例）

**卡片来源**：`cfg.FavoriteBrowsers()` —— 有收藏取收藏顺序，无收藏取全部（均已剔除 `hidden`）。

**交互**：

| 输入 | 行为 |
|------|------|
| 左键点击普通卡片 | 用该浏览器打开 URL |
| 左键点击**多 profile** 卡片 | 弹出账户菜单（不直接打开） |
| 右键点击多 profile 卡片 | 弹出账户菜单（锚定在卡片下方） |
| `⌘1`~`⌘9` | 打开完整列表中第 N 个浏览器（含被「更多」折叠的），使用默认 profile |
| 数字键 `1`~`9` | 同 ⌘N |
| `⌘R` | 展开完整浏览器列表，与点击「更多」卡片完全等价（复用同一个 `expand()`） |
| 点击顶部地址栏 | 把完整 URL 写入剪贴板（`clickableURLLabel`，基于 `widget.Hyperlink` 劫持 `OnTapped`） |
| `Enter` / `Return` | 用默认浏览器打开 |
| `Esc` | 关闭选择器，不打开任何浏览器 |

**并发保护**：`atomic.Bool` 保证「打开浏览器」动作只执行一次，避免快捷键与倒计时同时触发导致重复开窗。

**倒计时暂停**：点击齿轮打开设置时，通过 `close(stopCh)` 立即中断倒计时的 `select` 等待并隐藏倒计时区。倒计时的等待用 `select { time.After / stopCh }` 而非固定 `time.Sleep`，否则会出现「刚点开设置、1 秒后又被自动打开浏览器关掉」的竞态。`sync.Once` 保证 `stopCh` 只关闭一次。

**记住选择**：勾选后调用 `addRuleForURL()`，用 `extractDomain()`（剥离 scheme / www / 路径 / 端口 / query）得到的 host 写入一条 `exact` 规则，`priority=100`，`comment="Auto-created for <url>"`。同 host + 同浏览器 + 同账户已存在时跳过（去重）。

**账户一并记住**：从账户菜单里选定的子账号 / 无痕会写入 `Rule.Profile`（账户 ID，无痕为 `__incognito__`），命中规则时由 `launchForRule()` 走 `LaunchBrowserProfile()`；只选整浏览器时该字段为空，行为与过去一致。只记浏览器的话，用户选的"无痕/某账号"下次打开会被静默忽略。

---

### F5. 倒计时自动回退

**用户故事**：弹窗后我临时离开，工具不应卡住链接。

**实现方式**：

- 选择器弹出即启动 goroutine 倒计时，时长 = `auto_close_delay`（默认 300 秒）
- 通过 `fyne.Do()` 每秒回主线程刷新文本与进度条
- 剩余 ≤ 3 秒时文本与进度条转为警告红（`warn` 色）
- 用户已作出选择（`picked` 原子标志置位）时倒计时静默退出
- 倒计时归零 → 用 **`default_browser`** 打开（不是卡片上高亮的选中项，二者仅在默认浏览器存在时恰好一致）
- `auto_close_delay = 0` 或不存在默认浏览器时，整个倒计时区域隐藏

---

### F6. 默认 / 回退浏览器

- 设置界面「设置」标签选择默认浏览器
- `show_picker_on_miss = false`：无规则匹配 → 直接用默认浏览器打开
- `show_picker_on_miss = true`：无规则匹配 → 弹选择器，默认浏览器高亮并作为倒计时回退目标
- **多级兜底**：默认浏览器 ID 无效（未设置或已卸载）时回退到 `FavoriteBrowsers()` 的第一项；若一个可用浏览器都没有，则强制弹出选择器（即便设置关闭），避免 URL 被静默丢弃

---

### F7. 浏览器收藏、排序与隐藏

**用户故事**：我装了 10 个浏览器但常用只有 3 个。

- 设置「浏览器」标签左侧为收藏列表，条目带序号徽章（即 ⌘N 编号）、上移 / 下移 / 移除按钮
- 右侧为全部浏览器列表，每行提供：
  - ♥ 收藏切换
  - 👁 隐藏切换（隐藏后名称变灰并加后缀，不在选择器中出现）
  - ›/▼ 展开切换（仅多 profile 浏览器，展开后缩进列出各账户）
  - 「重新扫描」按钮（异步 + Activity 指示）
- 所有切换即时 `SaveConfig` 并重建面板
- **账户级收藏**：多账号浏览器展开后，每个账户（含无痕）行尾提供 ♥ 收藏按钮。收藏的账户作为选择器独立卡片出现（标题「浏览器 · 账户」），拥有独立 ⌘N 编号，点击/快捷键直接用该账户打开，不弹二级菜单。收藏项在 `Favorites` 中以复合 key 存储：`bundleID` = 整浏览器，`bundleID#<profileID>` = 指定账户。账户被删除后对应收藏项自动跳过（悬空清理）。

---

### F8. 多账户配置档（Profile）支持

**用户故事**：Chrome 有个人和工作两个账户，选择器应能区分。

**实现方式**（[profiles_darwin.go](profiles_darwin.go)）：

- **Chromium 家族**：读取 `~/Library/Application Support/<映射目录>/Local State` 的 `profile.info_cache`。已映射：Chrome、Chrome Canary、Edge、Brave、Vivaldi、Opera
- **Firefox**：解析 `~/Library/Application Support/Firefox/profiles.ini` 的 `[ProfileN]` 段
- 末尾始终追加一个合成的「无痕模式」条目（`__incognito__`）——**即便只检测到默认这一个 profile**。多数用户就只有默认账户，此前"≤1 个就返回 nil"让他们在选择器里根本选不到无痕
- 不支持多账户的浏览器（如 Safari）仍返回 `nil`
- 默认 profile 排首位，其余按名称稳定排序
- `profilesNeedChoice(profiles)` 判断打开前是否**必须**挑账户：只有存在多个真实 profile 时才必须（无痕是合成项，不算）。单账户浏览器点卡片直接打开，无痕走右键菜单

**启动方式**（关键约束）：必须**直接执行 `.app` 包内的二进制**并传参，因为浏览器已在运行时 `open -b` 会忽略 `--profile-directory` 等参数。
- Chromium：`<exe> --profile-directory=<ID> <url>`
- Firefox：`<exe> -P <name> <url>`
- 无痕：Chromium `--incognito`（Edge 为 `--inprivate`）、Firefox `--private-window`
- 取不到包内二进制路径时回退到 `LaunchBrowser()`
- 使用 `cmd.Start()` 而非 `Run()`，选择器无需等待浏览器退出

---

### F9. 规则管理

- 设置「规则」标签按 `priority` 降序列出：`模式徽章 | pattern | → | 浏览器名[ · 账户名] |「新窗口」角标 | 编辑按钮 | 删除按钮`。规则指定了账户时**必须显示账户名**，否则"记住了无痕"和"记住了 Chrome"在列表里长得一样；账户已删除时显示「账户已不存在」
- 「添加规则」/「编辑规则」打开独立窗口表单（复用当前 app 实例，避免嵌套事件循环）：
  - 匹配内容（输入时实时调用 `SuggestMatchMode` 自动切换模式下拉；**用户一旦手动选过模式就不再自动覆盖**）
  - 匹配模式（7 选 1，本地化显示名；选中项下方实时显示该模式的说明，三个快捷方式额外展示等价的通配写法）
  - 目标浏览器（下拉逐项列出「浏览器」与「浏览器 · 账户」，含无痕；新建时预选 `default_browser`，即不指定账户）
  - 优先级（默认 `50`，须为非负整数）
  - 备注（可选）
  - 新窗口打开（`open_in_new_window`）
- 编辑时先设模式、再设 pattern，并在两者之间关闭自动推测——顺序反了会被自动推测覆盖掉原模式
- 提交时校验（`ValidateRuleInput`）：pattern 非空、所选模式的语法合法、`urlequal` 必须带 `://`、`exact` 不得含 scheme 或路径、快捷方式不得含 `*`/`?`、不与既有规则完全重复
- 校验失败与写盘失败都在表单顶部**就地显示红字**，不弹模态框，用户输入内容保持可见
- 规则 ID 生成规则：手动 `user_<pattern>_<unixnano>`；自动 `auto_<host>_<unixnano>`。编辑时 ID 保持不变

**窗口单例**：`settingsWin` 与 `ruleDialogWin` 两个包级变量各保证最多一个窗口。重复点击「添加规则」或连点不同规则的编辑按钮，只会保留最新那一个。设置窗口关闭或重载时一并关掉规则窗口，避免用旧数据误保存。

**重新打开设置窗口时先重载磁盘数据**（`reloadConfigInto`）：就地赋值 `*cfg = *fresh`——选择器与设置窗口共享同一个 `*Config`，换指针只有一方看得到新数据。读盘失败或配置损坏时保留内存中现有值。

**当前不支持**：切换 `enabled`、拖拽调整优先级、在列表里展示 `comment`、按 profile/无痕匹配。需直接编辑 JSON。

---

### F10. 浏览器图标

- 读取浏览器 `.app` 的 `CFBundleIconFile`，定位 `Contents/Resources/*.icns`
- `sips -s format png -Z 128` 转 PNG，缓存到 `/tmp/browser-switch-icons/<bundleID>.png`
- 已缓存（文件存在且非空）时直接复用，**不做失效判断**
- 取不到图标时使用首字母头像兜底（蓝色圆底 + 白色大写首字母）
- 使用尺寸：选择器卡片 52px、收藏列表 22px、全部列表 26px

---

### F11. 设置界面与国际化

**设置界面**（[settings.go](settings.go)）三标签页：浏览器 / 规则 / 设置。

「设置」标签包含：语言选择、默认浏览器、自动打开秒数、无匹配弹窗开关、安装 / 卸载按钮、「将其他浏览器设为系统默认」（直接调用 `LSSetDefaultHandlerForURLScheme`，绕过本应用）。

**国际化**（[i18n/i18n.go](i18n/i18n.go)）：
- 7 个语言包（`zh-CN`/`zh-TW`/`en`/`ja`/`ko`/`pt`/`hi`），各 141 个 key，通过 `go:embed` 编译进二进制
- 检测优先级：`LC_ALL` > `LC_MESSAGES` > `LANG` > 系统区域（`defaults read -g AppleLocale`）> 默认 `en`
- 用户在设置中选择的语言存入 `config.language`，启动时覆盖系统检测
- 翻译回退链：当前语言 → `en` → 返回 key 本身
- 切换语言后立即重建设置窗口全部文本，并保留当前选中的标签页索引

---

### F12. 安装向导 UI

`--installer` 打开独立窗口（[main.go](main.go) `BuildInstallerUI`）：应用标题 + 版本 + 四项核心特性说明 + 当前状态面板（是否已安装 / 当前默认浏览器）+ 「安装」与「设为默认」两个按钮。

---

## 3. 非功能需求

| 维度 | 约束 |
|------|------|
| **平台** | 仅 macOS 10.14+（`LSMinimumSystemVersion`）；平台代码由 `//go:build darwin` 隔离 |
| **语言 / 运行时** | Go 1.23+ |
| **GUI 框架** | Fyne v2.7.4，颜色取自主题语义色，自动适配深浅色模式 |
| **CGO** | **必须开启**。CoreServices（默认浏览器读写）+ Carbon（Apple Event 接收）+ CoreFoundation |
| **外部命令依赖** | `plutil`、`sips`、`open`、`codesign`、`xattr`、`lsregister`（均为 macOS 内置） |
| **线程模型** | `init()` 中 `runtime.LockOSThread()`；后台 goroutine 通过 `fyne.Do()` 回主线程更新 UI；`atomic.Bool`/`atomic.Int32` 保护一次性动作 |
| **配置持久化** | `~/.config/browser-switch/config.json`，`json.MarshalIndent` + `os.WriteFile`（非原子写），`sync.RWMutex` 保护写入 |
| **进程模型** | 非常驻。每次链接点击 = 一次冷启动，处理完退出 |
| **签名** | 仅 ad-hoc 签名，未经 Apple 公证 |
| **权限** | 设置默认浏览器可能需用户在「系统设置」中手动确认（macOS 安全策略） |

---

## 4. 关键设计决策与取舍

| 决策 | 理由 | 代价 |
|------|------|------|
| **单 App 取代双 App** | 直接用 cgo 注册 Carbon `kAEGetURL` 处理器接收 URL，无需 AppleScript 转发器，安装物少一半，维护面收窄 | 强依赖 cgo 与 Carbon（已弃用 API，需 `-Wno-deprecated-declarations`） |
| **非常驻进程** | Fyne 的 glfw 驱动不转发 Dock Reopen 事件给 Carbon 处理器，常驻进程收不到「再次点击」通知。冷启动让「点 Dock 必显示设置、点链接必弹选择器」行为可预测 | 每次点击链接都要付一次进程启动 + Fyne 初始化开销 |
| **冷启动 500ms 判定** | 500ms 内收到 URL → 弹选择器；未收到 → 判定为用户主动打开 App → 显示设置窗口 | 系统负载高时 Apple Event 可能晚于 500ms 到达，导致设置窗口先闪现（见 [docs/ISSUES.md](docs/ISSUES.md) I-4） |
| **`open -b <bundleID>` 启动浏览器** | 遵循 LaunchServices，最可靠；不加 `-n` 以复用已开窗口 | 无法传递 `--profile-directory` 等参数，故 profile 启动必须直接执行包内二进制 |
| **profile 启动直执二进制** | `open -b` 在浏览器已运行时丢弃参数 | 绕过 LaunchServices，需自行解析 `CFBundleExecutable` |
| **「记住选择」连 profile 一起记** | 规则模型增加 `Rule.Profile`，命中时走 `LaunchBrowserProfile`。只记浏览器会让"选了无痕/子账号"的选择被静默丢弃 | 规则多一个字段；账户被删除后规则退化为默认账户打开 |
| **图标缓存到 `/tmp`** | 免去缓存目录管理，系统重启自动清理 | 浏览器更新图标后不刷新（无失效判断） |
| **配置字段先声明后使用** | 早期为托盘、窗口尺寸预留字段 | 现存 5 个死字段（违反 YAGNI，见 ISSUES I-9） |

---

## 5. 用户界面流程

```
点击链接
  → macOS LaunchServices 投递 GetURL Apple Event 给 Browser Switch.app
    → cgo 回调 → urlEventCh → Fyne 主循环 openPicker(url)
      → MatchURL(url, cfg)
        ├─ 规则命中 → LaunchBrowser() → a.Quit()（无 GUI 闪现）
        └─ 无匹配
            ├─ show_picker_on_miss = false → LaunchBrowser(default) → a.Quit()
            └─ show_picker_on_miss = true → 选择器窗口
                ├─ 点击普通卡片            → LaunchBrowser()
                ├─ 点击/右键多 profile 卡片 → 账户菜单 → LaunchBrowserProfile()
                ├─ ⌘N / 数字键             → LaunchBrowser(list[N-1])
                ├─ ⌘R                      → 展开完整列表（同点击「更多」）
                ├─ 点击地址栏               → 复制完整 URL
                ├─ 点击齿轮                 → 暂停倒计时 + 打开设置
                ├─ Enter                   → LaunchBrowser(default)
                ├─ Esc                     → 关闭，不打开
                └─ 倒计时归零              → LaunchBrowser(default)

双击 Dock 图标（无 URL）
  → 500ms 内无 Apple Event → 显示设置窗口 → 关闭即退出进程
```

---

## 6. 配置文件结构

见 [README.md](README.md#️-配置)。

---

## 7. 已知限制

1. **误检测**：带 WebView 的非浏览器应用（部分终端、开发工具）会声明 http handler 而被列出，需手动隐藏。这与系统「默认浏览器」候选列表的口径一致。
2. **默认浏览器变更需确认**：macOS 安全策略可能弹出系统确认框，无法完全静默。
3. **浏览器列表非自动刷新**：新装浏览器需手动「重新扫描」。
4. **规则只能增删**：编辑、启用/禁用、调整优先级需直接改 JSON。
5. **无系统托盘**：`tray_icon` 配置字段存在但无对应实现。
6. **无公证签名**：ad-hoc 签名，首次运行可能触发 Gatekeeper 提示。
7. 完整缺陷清单见 [docs/ISSUES.md](docs/ISSUES.md)。
