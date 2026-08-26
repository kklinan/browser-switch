<!--
Browser Switch —— 免费开源的 macOS 默认浏览器选择器与按站点分流的 URL 路由工具。
关键词：macOS 默认浏览器、浏览器选择器、浏览器路由、按域名规则打开浏览器、
用不同浏览器打开链接、多账户浏览器启动器、Chrome 多账号切换、
Finicky 替代、Velja 替代、Browserosaurus 替代、Choosy 替代。
-->

# Browser Switch —— macOS 默认浏览器选择器与按站点 URL 路由 🌐

**Browser Switch** 是一款免费开源的 **macOS 默认浏览器选择器**。把它设为默认浏览器后，你点击的每个链接都会按你自己的规则分流——工作链接用 Edge、私人链接用 Chrome、开发链接用 Firefox，全自动完成。没有规则命中时，弹出一个键盘友好的**浏览器选择器**让你当场决定。

<p>
<a href="README.md">English</a> ·
<a href="README.zh-CN.md"><b>简体中文</b></a> ·
<a href="README.ja.md">日本語</a> ·
<a href="README.ko.md">한국어</a>
</p>

![platform](https://img.shields.io/badge/platform-macOS%2010.14%2B-black)
![Go](https://img.shields.io/badge/go-1.23%2B-00ADD8)
![GUI](https://img.shields.io/badge/GUI-Fyne%20v2.7-blue)
![license](https://img.shields.io/badge/license-Apache_2.0-green)

> **仅支持 macOS。** 默认浏览器注册依赖 CoreServices（cgo），URL 接收依赖 Carbon Apple Event API，均为 macOS 专有实现。仓库中不存在 Linux / Windows 平台文件。

---

## 目录

- [为什么用 Browser Switch？](#为什么用-browser-switch)
- [功能特性](#功能特性)
- [工作原理](#工作原理)
- [安装](#安装)
- [使用方法](#使用方法)
- [规则匹配](#规则匹配)
- [多账户与账户级收藏](#多账户与账户级收藏)
- [配置](#配置)
- [架构](#架构)
- [从源码构建](#从源码构建)
- [卸载](#卸载)
- [与同类工具对比](#与同类工具对比)
- [常见问题](#常见问题)
- [已知限制](#已知限制)
- [文档](#文档)
- [许可证](#许可证)

---

## 为什么用 Browser Switch？

如果你每天要在多个浏览器或多个浏览器账户之间切换，macOS 只允许一个默认浏览器、一套工作流。Browser Switch 解决这个问题：

- **按站点分流链接。** 规则引擎根据域名把每个 URL 送到对应浏览器——不必再在浏览器之间复制粘贴链接。
- **按账户分流链接。** 在 macOS 链接路由工具中独一无二：Browser Switch 能把链接送到**指定的 Chrome/Edge/Firefox 账户**（工作 Google 账号 vs 个人账号）。
- **链接绝不丢失。** 无规则命中时弹出原生选择器并带倒计时兜底，链接永远不会被丢弃。
- **原生、小巧。** 单个 Go 二进制，使用系统自带的 AppKit——不是 ~150 MB 的 Electron 应用。

---

## 功能特性

| 功能 | 说明 |
| ---- | ---- |
| 🎯 **URL 拦截** | 注册为 macOS `http`/`https` 处理器，通过 Carbon Apple Event 直接接收 URL |
| 📋 **六种规则模式** | 精确 / 通配符 / 正则 / 包含 / 前缀 / 后缀，按优先级降序命中 |
| 🖱️ **卡片式选择器** | 浏览器图标网格，超过 4 个折叠进「更多」卡片 |
| ⌨️ **键盘优先** | `⌘1`~`⌘9` 或数字键打开第 N 个浏览器；`Enter` 打开默认；`Esc` 取消 |
| ⏱️ **倒计时回退** | 超过可配置时长（默认 5 秒）后自动用**默认浏览器**打开，链接绝不卡住 |
| 💾 **记住选择** | 勾选后自动为该域名生成精确匹配规则（优先级 100） |
| 👥 **多账户支持** | 自动检测 Chromium（Chrome/Edge/Brave/Vivaldi/Opera）与 Firefox 账户，附带无痕模式 |
| ⭐ **收藏与排序** | 自定义选择器中显示哪些浏览器——以及哪些账户——及其顺序（决定 ⌘N 编号） |
| 🌍 **7 种语言** | 简中 / 繁中 / 英 / 日 / 韩 / 葡 / 印地，编译期内嵌 |
| ♻️ **干净卸载** | 还原安装前处于活动状态的默认浏览器 |

---

## 工作原理

```
你点击链接
    ↓
macOS LaunchServices 向 Browser Switch.app 投递 GetURL Apple Event
    ↓
应用按优先级把 URL 与你的规则匹配
    ├── 命中  → 直接用映射的浏览器打开（无界面），随后退出
    └── 未命中
        ├── show_picker_on_miss = false → 用默认浏览器打开，退出
        └── show_picker_on_miss = true  → 弹出选择器
            ├── 点击卡片 / ⌘N / Enter → 用所选浏览器（或账户）打开
            ├── Esc                    → 取消
            └── 倒计时归零             → 用默认浏览器打开
```

Browser Switch 是**单 App**：它把自己注册为系统 `http`/`https` 处理器，并安装 Carbon Apple Event 处理器（`kInternetEventClass` / `kAEGetURL`）直接接收 URL——无需 AppleScript 转发器。

---

## 安装

### 依赖

构建只需 Xcode 命令行工具（提供 cgo 所需的 CoreServices / Carbon 头文件）：

```bash
xcode-select --install
```

所有运行期依赖均为 macOS 内置命令：`plutil`、`sips`、`open`、`codesign`、`xattr`、`lsregister`。

### 构建与安装

```bash
# 1. 构建（必须开启 CGO）
make build
# 等价于：
CGO_ENABLED=1 go build -ldflags="-s -w" -o browser-switch .

# 2. 安装为默认浏览器
./browser-switch --install       # 生成 ~/Applications/Browser Switch.app 并注册
./browser-switch --check-default # 验证
```

`--install` 会：

1. 把当前可执行文件拷贝进 `~/Applications/Browser Switch.app/Contents/MacOS/browser-switch`
2. 写入声明 `http`/`https` URL scheme 的 `Info.plist`
3. Ad-hoc 代码签名并用 LaunchServices（`lsregister`）注册
4. 记录当前默认浏览器（供卸载还原）
5. 调用 `LSSetDefaultHandlerForURLScheme`；若未生效则打开**系统设置 → 通用**

> macOS 安全策略可能要求你在系统设置中确认一次默认浏览器变更，属正常现象。

---

## 使用方法

### 命令行

```bash
browser-switch https://example.com   # 走规则匹配 / 弹选择器
browser-switch --settings            # 打开设置窗口
browser-switch --installer           # 打开安装向导 UI
browser-switch --list-browsers       # 列出检测到的浏览器（⭐ 标注默认）
browser-switch --list-profiles       # 列出各浏览器的账户
browser-switch --test https://github.com  # 只测试规则匹配，不打开浏览器
browser-switch --check-default       # 查询是否为系统默认
browser-switch --install             # 安装并注册为默认
browser-switch --uninstall           # 卸载并还原原默认浏览器
browser-switch --version             # 版本信息
```

### 选择器交互

| 输入 | 行为 |
| ---- | ---- |
| 左键点击卡片 | 打开该浏览器；**若有多个账户，则弹出账户菜单** |
| 右键点击卡片 | 弹出账户菜单（仅多账户浏览器） |
| `⌘1`~`⌘9` / `1`~`9` | 直接打开第 N 个浏览器（使用默认账户） |
| `Enter` | 打开默认浏览器 |
| `Esc` | 取消，不打开任何浏览器 |
| 「记住此域名」 | 为本次选择写入一条 `exact` 规则 |
| 齿轮 / 复制按钮 | 打开设置 / 复制 URL 到剪贴板 |

倒计时归零时使用的是**默认浏览器**（配置中的 `default_browser`），而非当前高亮的卡片。

### 设置窗口

三个标签页：

- **浏览器**——左侧：收藏列表（上移/下移/移除；顺序即 ⌘N 编号）；右侧：全部浏览器（收藏 ♥ / 隐藏 👁 / 展开账户 / 重新扫描）
- **规则**——按优先级降序列出全部规则；支持新增与删除
- **通用**——语言、默认浏览器、自动打开秒数、无匹配规则时的处理（弹出选择器，或直接用指定浏览器打开）、安装/卸载、「将其他浏览器设为系统默认」

---

## 规则匹配

| 模式 | 匹配对象 | 示例 |
| ---- | -------- | ---- |
| `exact` | host 全等 | `github.com` → 仅 github.com，不含 sub.github.com |
| `wildcard` | host，支持 `*` `?` | `*.google.com` → mail.google.com |
| `regex` | host **或**完整 URL | `.*\.(test\|staging)\..*` |
| `contains` | host **或**完整 URL 子串 | `login` → example.com/login |
| `prefix` | host 前缀 | `dev.` → dev.example.com |
| `suffix` | host 后缀 | `.cn` → example.cn |

- 规则按 `priority` **降序**匹配，首个命中即返回。
- 匹配时会同时用原始 host 和剥离 `www.` 后的 host 各试一次，因此 `example.com` 规则也能命中 `www.example.com`。
- 「记住选择」生成的规则优先级固定为 `100`，手动新增默认为 `50`。

不打开浏览器测试任意 URL：

```bash
browser-switch --test https://mail.google.com/u/1/inbox
```

---

## 多账户与账户级收藏

Browser Switch 读取你在各浏览器中配置好的账户：

- **Chromium 家族**（Chrome、Edge、Brave、Vivaldi、Opera）：读取 `~/Library/Application Support/<app>/Local State`
- **Firefox**：读取 `~/Library/Application Support/Firefox/profiles.ini`
- 每个多账户浏览器还附带一个合成的**无痕 / 隐私**条目。

**账户级收藏。** 在「浏览器」标签展开任一多账户浏览器，点击任意账户行尾的 ♥。被收藏的账户在选择器中作为**独立卡片**出现（标题「浏览器 · 账户」），拥有自己的 ⌘N 编号，点击即用该账户直接启动——不弹二级菜单。它以复合收藏 key 存储：`bundleID` 表示整浏览器，`bundleID#profileID` 表示指定账户。账户被删除后，对应的悬空收藏项会被自动跳过。

启动指定账户时会直接执行浏览器二进制并传 `--profile-directory=`（Chromium）或 `-P`（Firefox），因为 `open -b` 在浏览器已运行时会忽略这些参数。

---

## 配置

配置文件：`~/.config/browser-switch/config.json`（首次运行自动创建并检测浏览器）。

```json
{
  "default_browser": "com.google.Chrome",
  "browsers": [
    {
      "id": "com.google.Chrome",
      "name": "Google Chrome",
      "exec": "com.google.Chrome",
      "desktop": "/Applications/Google Chrome.app",
      "icon": "com.google.Chrome"
    }
  ],
  "favorites": ["com.google.Chrome", "com.google.Chrome#Profile 1", "com.apple.Safari"],
  "hidden": [],
  "rules": [
    {
      "id": "work",
      "pattern": "*.company.com",
      "mode": "wildcard",
      "browser": "com.microsoft.edgemac",
      "priority": 100,
      "enabled": true,
      "comment": "工作站点用 Edge"
    }
  ],
  "auto_close_delay": 5,
  "show_picker_on_miss": true,
  "language": "",
  "prev_default_browser": "com.apple.safari"
}
```

| 字段 | 说明 |
| ---- | ---- |
| `default_browser` | 浏览器 ID（macOS 上即 bundle ID）。规则未命中且不弹窗时使用，也是倒计时回退目标 |
| `favorites` | 选择器顺序。纯 bundle ID = 整浏览器；`bundleID#profileID` = 指定账户。为空时回退到全部（去除 `hidden`） |
| `hidden` | 从选择器与列表中隐藏的浏览器 ID（用于屏蔽误检测的非浏览器应用） |
| `auto_close_delay` | 倒计时秒数；`0` 表示不自动打开 |
| `show_picker_on_miss` | `false` 时无匹配直接用默认浏览器打开，不弹窗 |
| `language` | 空字符串跟随系统；否则取值 `zh-CN` / `zh-TW` / `en` / `ja` / `ko` / `pt` / `hi` |
| `prev_default_browser` | 安装时自动记录，卸载时还原；无记录则回退 Safari |

---

## 架构

```
main.go            → CLI 分派、命令行 URL 路径（handleURL）、安装器 UI
config.go          → Config / Browser / Rule / Profile 类型 + JSON 持久化
rules.go           → MatchURL 规则引擎、ValidatePattern、SuggestMatchMode
picker.go          → 选择器窗口、倒计时、快捷键、「记住选择」
settings.go        → 设置窗口（三标签页）
gui.go             → 共享 Fyne 组件（card、progressLine、图标/文本工具）
constants.go       → 应用名与 bundle ID
browsers_darwin.go → 通过 .app + CFBundleURLTypes 检测浏览器；open -b 启动
install_darwin.go  → 生成 .app、ad-hoc 签名、LaunchServices 默认处理器（cgo）
urlhandler_darwin.go → 通过 Carbon Apple Event 接收 URL（cgo）、单 App 主循环
profiles_darwin.go → Chromium Local State / Firefox profiles.ini 检测与启动
icons_darwin.go    → 提取并缓存 .icns → PNG
i18n/              → 7 个内嵌语言包 + T/Tf 翻译函数
```

**非常驻设计。** 每次点击都是一次冷启动，处理完即退出。Fyne 的 glfw 驱动不会把 Dock 的「Reopen」事件转发给 Carbon 处理器，常驻进程收不到「再次点击」。冷启动让行为可预测：点 Dock 图标必显示设置，点链接必弹选择器。

产品完整规格见 [PRD.md](PRD.md)，架构约束见 [CLAUDE.md](CLAUDE.md)。

---

## 从源码构建

```bash
git clone https://github.com/kklinan/browser-switch.git
cd browser-switch
make build        # CGO_ENABLED=1 go build -ldflags="-s -w" -o browser-switch .

go test ./...     # 运行纯函数测试套件
go vet ./...
```

> `Makefile` 提供 `build` / `test` / `vet` / `app` / `dmg` / `clean` 目标；`make app` 与 `make dmg` 生成可分发的 `.app` 与 DMG（仅 macOS）。

---

## 卸载

```bash
./browser-switch --uninstall     # 还原原默认浏览器 + 删除 ~/Applications/Browser Switch.app
rm -rf ~/.config/browser-switch  # 删除配置
rm -rf /tmp/browser-switch-icons # 删除图标缓存
```

---

## 与同类工具对比

| | **Browser Switch** | Velja | Browserosaurus | Finicky | Choosy |
| --- | --- | --- | --- | --- | --- |
| 价格 | **免费开源** | 免费 / 内购 | 免费开源 | 免费开源 | 付费 |
| 规则引擎 | **6 种模式 + 优先级** | 有 | 无（仅选择器） | 有（JS 配置） | 有 |
| 图形化规则编辑 | **有** | 有 | — | 无（改 JS） | 有 |
| **按账户 / 账户分流** | **✅ 有** | 无 | 无 | 无 | 无 |
| 选择器窗口 | **有** | 有 | 有 | 无 | 有 |
| 原生二进制 | **有（Go/AppKit）** | 有 | 无（~150 MB Electron） | 有 | 有 |
| 倒计时回退 | **有** | 无 | 无 | 无 | 无 |

Browser Switch 的生态位：**Finicky 的规则能力 + Browserosaurus 的 GUI 易用性 + 谁都没有的按账户分流。**

---

## 常见问题

**如何在 macOS 上为不同网站设置不同默认浏览器？**
把 Browser Switch 安装为默认浏览器，然后添加把域名映射到浏览器的规则（设置 → 规则，或编辑 JSON 配置）。每次点击都会自动分流。

**能否自动用指定的 Chrome 或 Firefox 账户打开链接？**
可以。Browser Switch 会检测你的浏览器账户，并允许把单个账户收藏为选择器独立卡片。按账户自动匹配的**规则**在规划中（[ROADMAP.md](docs/ROADMAP.md) §3.1）。

**Browser Switch 是不是好的 Velja / Finicky / Choosy 替代？**
它是免费、开源、原生的替代品。其独特之处是按账户（profile）分流，这是其他工具都没有的。Finicky 需要写 JS 配置文件；Browser Switch 有 GUI。

**支持 Linux 或 Windows 吗？**
不支持。设计上仅限 macOS——默认浏览器注册和 URL 接收依赖 macOS 专有 API。

**为什么终端或带 WebView 的应用出现在浏览器列表里？**
任何在 `Info.plist` 中声明了 `http`/`https` 处理器的应用都会被检测为候选（与 macOS「默认浏览器」候选列表口径一致）。可在设置 → 浏览器中隐藏误报项。

**链接会丢失吗？**
不会。若无规则命中而你恰好走开，倒计时会自动用默认浏览器打开。

**配置存在哪里？**
`~/.config/browser-switch/config.json`。是可编辑、可跨机同步的纯 JSON。

---

## 已知限制

1. 部分带 WebView 的非浏览器应用会声明 `http` 处理器而被检测——在设置 → 浏览器中隐藏它们。
2. macOS 变更默认浏览器时可能要求在系统设置中手动确认。
3. 新装的浏览器不会自动出现——在设置中点击「重新扫描」。
4. 规则界面仅支持新增与删除；编辑、启用/禁用、调整优先级需直接改 JSON。
5. 应用仅做 ad-hoc 签名，未经 Apple 公证，首次运行可能触发 Gatekeeper 提示。
6. 完整问题清单见 [docs/ISSUES.md](docs/ISSUES.md)。

---

## 文档

| 文档 | 用途 |
| --- | --- |
| [English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) · [한국어](README.ko.md) | 用户指南（本文件） |
| [PRD.md](PRD.md) | 产品需求，已按实现校对 |
| [CLAUDE.md](CLAUDE.md) | 面向 AI 编码助手的架构约束（唯一事实来源） |
| [docs/ISSUES.md](docs/ISSUES.md) | 现存问题清单（含代码定位） |
| [docs/ROADMAP.md](docs/ROADMAP.md) | 产品规划 |

---

## 许可证

[Apache-2.0](LICENSE)
