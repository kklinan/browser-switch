# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.1] - 2026-09-04

**主题：规则终于能记住「用哪个账户打开」。** 1.0.0 的「记住选择」只存浏览器，选择了子账号或无痕的用户下次打开时仍会落到默认账户；1.0.1 把账户纳入了规则模型。

### 升级说明

- **配置文件向后兼容，无需迁移。** 老规则没有 `profile` 字段，反序列化为空串（= 不指定账户），行为与 1.0.0 完全一致。
- **升级需要重新安装 `.app`**：`./browser-switch --install`，或 `make app` 后手动替换 `~/Applications/Browser Switch.app`。bundle 版本随之更新为 1.0.1。
- 已有的「记住选择」规则不会自动获得账户信息。想让某条规则走特定账户，在设置 → 规则里编辑它并选定账户即可。

### Added

- **规则可以指定浏览器的具体账户**：新增 `Rule.Profile`（JSON `profile`）。添加／编辑规则时，目标浏览器下拉会逐项列出该浏览器的每个账户，例如 `Google Chrome`、`Google Chrome · 工作`、`Google Chrome · 无痕`。
- **「记住选择」现在连账户一起记**：从账户菜单里选定的子账号／无痕会写入规则，下次命中时用同一个账户打开。去重粒度相应变为「浏览器 + 账户」。
- 规则列表在浏览器名后显示账户名，「记住了无痕」不再与「记住了 Chrome」长得一样；账户已被删除时显示「账户已不存在」，而不是静默退化成默认账户。
- **Chromium / Firefox 现在总能选到无痕**：此前只有检测到 ≥2 个配置档才展开账户菜单，导致只有默认账户的用户（占绝大多数）根本看不到无痕选项。
- 新匹配模式 `urlequal`（完整网址相等）：比较包含 scheme / 路径 / 查询串的完整 URL，忽略大小写，区分度最高。
- 规则支持**强制新窗口打开**（`open_in_new_window`）。因为 Chrome / Edge / Safari 是单实例应用、`open -n` 会被忽略，改为 Chromium 与 Firefox 直接传 `--new-window`、Safari 走 AppleScript。
- 规则可以**原地编辑**（每行规则右侧的铅笔按钮），复用添加规则的对话框。预填表单时会关掉模式自动推测，避免把已保存的模式覆盖掉（例如 `.pdf` 后缀规则被改回「域名相等」）。
- 添加／编辑规则时，匹配模式下方实时显示该模式的说明；三个快捷方式额外展示等价的通配写法（如「包含 x」⇔ `*x*`）。
- 选择器中 `⌘R` 展开完整浏览器列表，与点击「更多」卡片等价。
- 点击选择器顶部的 URL 即可复制完整网址到剪贴板。
- DMG 打包支持背景图与 Finder 窗口布局：新增 `assets/dmg-background.png`，窗口尺寸与图标位置按背景图重排（见 `scripts/build-dmg.sh`）。

### Changed

- `wildcard` / `prefix` / `suffix` 现在**同时对完整 URL 匹配**，不再只看域名（`contains` 原本就匹配两者）。此前 `*/settings`、`https://`、`.pdf` 这类规则永远不可能命中，因为那些字符串不出现在主机名里。三者仍是 `wildcard` 的一等快捷方式（`*p*` / `p*` / `*p`），UI 中归为一组并标注为快捷方式。
- 手动选择的匹配模式不再被自动推测覆盖：对话框只在用户未干预时自动推荐，之后每敲一个字符都不会重置你选的模式。
- 从选择器的齿轮按钮打开设置时，**暂停自动关闭倒计时**，设置窗口不会再被倒计时顶掉。
- 「更多」卡片标注了 `⌘R` 快捷键。
- 文档同步到「单 App + macOS only + Fyne v2.7」的现状：四份 README 译文、`CLAUDE.md`、`PRD.md` 均已校对。

### Fixed

- **「记住选择」不再丢弃你选的账户。** 此前只把浏览器写进规则，选了子账号或无痕并勾选记住后，之后每次命中都静默回落到默认账户。现在账户会写入规则，且两条 URL 入口（命令行参数、以及作为默认浏览器收到的 Apple Event）统一经 `launchForRule()` 启动。
- 选择器卡片点击与 `⌘N` 语义统一：浏览器有多个真实账户时两者都弹账户菜单，只有一个账户时两者都直接打开，无痕通过右键菜单选择（卡片副标题会提示）。
- 通配规则现在也对完整 URL 匹配，含路径或 scheme 的写法（`*/settings`、`https://*`）终于能命中；编译后的通配正则加入缓存。
- 保存规则前做语义校验并**就地显示错误**，不再默默接受永远不可能命中的规则：`urlequal` 缺 `://`、`exact` 含 scheme 或路径、快捷方式里出现 `*`/`?`（那里它们是字面量）、非法正则、完全重复的规则，以及配置写盘失败。
- 反复点击「添加规则」或多条规则的编辑按钮不再叠出一堆对话框：规则窗口全局单例，始终显示最近一次请求。
- 重开设置窗口会先从磁盘重载配置，不再展示（并回写覆盖）别处已修改的旧数据。

## [1.0.0] - 2026-08-26

- Initial public release: macOS default-browser picker with a per-site rule
  engine (6 modes + priority), per-account/profile routing, a keyboard-first
  card picker with countdown fallback, and a 3-tab settings window.
- 补记：本版本还包含开源前的准备工作，此前未写入 changelog —— 清理死代码与从未
  被读取的配置字段（`remember_choice`、`skip_installed`、`tray_icon`、
  `window_width`、`window_height`）、移除与 `en.json` 逐字节相同的 `en-US.json`
  （语言集精简为 7 种）、基于 `AppleLocale` 的系统语言检测、`make app` / `make dmg`
  多架构打包（amd64 / arm64 / universal）、`CONTRIBUTING.md` / `SECURITY.md` /
  `NOTICE` 等社区文件，以及 `.gitignore` 补齐。

[Unreleased]: https://github.com/kklinan/browser-switch/compare/v1.0.1...HEAD
[1.0.1]: https://github.com/kklinan/browser-switch/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/kklinan/browser-switch/releases/tag/v1.0.0
