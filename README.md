<!--
Browser Switch — a free, open-source default browser picker and per-site browser router for macOS.
Keywords: macOS default browser, browser picker, browser chooser, per-site browser rules,
URL router, open links in different browsers, multi-profile browser launcher, Chrome profile switcher,
Finicky alternative, Velja alternative, Browserosaurus alternative, Choosy alternative.
-->

# Browser Switch — macOS Default Browser Picker & Per-Site URL Router 🌐

**Browser Switch** is a free, open-source **default browser picker for macOS**. Set it as your default browser and every link you click is routed by your own rules — open work links in Edge, personal links in Chrome, and dev links in Firefox, automatically. When no rule matches, a fast keyboard-driven **browser chooser** pops up so you decide on the spot.

<p>
<a href="README.md"><b>English</b></a> ·
<a href="README.zh-CN.md">简体中文</a> ·
<a href="README.ja.md">日本語</a> ·
<a href="README.ko.md">한국어</a>
</p>

![platform](https://img.shields.io/badge/platform-macOS%2010.14%2B-black)
![Go](https://img.shields.io/badge/go-1.23%2B-00ADD8)
![GUI](https://img.shields.io/badge/GUI-Fyne%20v2.7-blue)
![license](https://img.shields.io/badge/license-Apache_2.0-green)

> **macOS only.** Default-browser registration uses CoreServices (cgo) and URL delivery uses the Carbon Apple Event API — both macOS-specific. There are no Linux or Windows platform files in this repository.

---

## Table of Contents

- [Why Browser Switch?](#why-browser-switch)
- [Features](#features)
- [How It Works](#how-it-works)
- [Installation](#installation)
- [Usage](#usage)
- [Rule Matching](#rule-matching)
- [Multi-Profile & Per-Account Favorites](#multi-profile--per-account-favorites)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Building from Source](#building-from-source)
- [Uninstall](#uninstall)
- [Comparison with Alternatives](#comparison-with-alternatives)
- [FAQ](#faq)
- [Known Limitations](#known-limitations)
- [Documentation](#documentation)
- [License](#license)

---

## Why Browser Switch?

If you juggle multiple browsers or multiple browser accounts every day, macOS forces you into one default browser and one workflow. Browser Switch fixes that:

- **Route links per site.** A rule engine sends each URL to the right browser based on its domain — no more copy-pasting links between browsers.
- **Route links per account.** Uniquely among macOS link routers, Browser Switch can send a link to a **specific Chrome/Edge/Firefox profile** (your work Google account vs. your personal one).
- **Never lose a click.** When no rule matches, a native picker appears with a countdown fallback, so a link is never dropped.
- **Native and tiny.** A single Go binary using the system's own AppKit — not a ~150 MB Electron app.

---

## Features

| Feature | Description |
| ------- | ----------- |
| 🎯 **URL interception** | Registers as the macOS `http`/`https` handler and receives URLs directly via Carbon Apple Events |
| 📋 **Six rule modes** | Exact / wildcard / regex / contains / prefix / suffix, evaluated by descending priority |
| 🖱️ **Card-based picker** | A grid of browser icons; more than 4 collapse behind a "More" card |
| ⌨️ **Keyboard-first** | `⌘1`–`⌘9` or number keys open the Nth browser; `Enter` opens the default; `Esc` cancels |
| ⏱️ **Countdown fallback** | After a configurable timeout (default 5s) it auto-opens the **default browser**, so links never hang |
| 💾 **Remember choice** | Tick a box to auto-create an exact-match rule for that domain (priority 100) |
| 👥 **Multi-profile support** | Auto-detects Chromium (Chrome/Edge/Brave/Vivaldi/Opera) and Firefox profiles, plus incognito |
| ⭐ **Favorites & ordering** | Choose which browsers — and which accounts — appear in the picker and in what order (this sets the ⌘N numbering) |
| 🌍 **7 languages** | Simplified/Traditional Chinese, English, Japanese, Korean, Portuguese, Hindi — embedded at build time |
| ♻️ **Clean uninstall** | Restores the default browser that was active before installation |

---

## How It Works

```
You click a link
    ↓
macOS LaunchServices delivers a GetURL Apple Event to Browser Switch.app
    ↓
The app matches the URL against your rules (by priority)
    ├── Match  → opens the mapped browser directly (no UI), then exits
    └── No match
        ├── show_picker_on_miss = false → opens your default browser, exits
        └── show_picker_on_miss = true  → shows the picker
            ├── click a card / ⌘N / Enter → opens the chosen browser (or profile)
            ├── Esc                       → cancels
            └── countdown reaches 0       → opens the default browser
```

Browser Switch is a **single app**: it registers itself as the system's `http`/`https` handler and installs a Carbon Apple Event handler (`kInternetEventClass` / `kAEGetURL`) to receive URLs directly — no AppleScript forwarder needed.

---

## Installation

### Requirements

Only the Xcode Command Line Tools are required to build (they provide the CoreServices / Carbon headers for cgo):

```bash
xcode-select --install
```

All runtime dependencies are built-in macOS commands: `plutil`, `sips`, `open`, `codesign`, `xattr`, `lsregister`.

### Build & install

```bash
# 1. Build (CGO is mandatory)
make build
# equivalent to:
CGO_ENABLED=1 go build -ldflags="-s -w" -o browser-switch .

# 2. Install as the default browser
./browser-switch --install       # creates ~/Applications/Browser Switch.app and registers it
./browser-switch --check-default # verify
```

`--install` will:

1. Copy the current executable into `~/Applications/Browser Switch.app/Contents/MacOS/browser-switch`
2. Write an `Info.plist` declaring the `http`/`https` URL schemes
3. Ad-hoc code-sign and register with LaunchServices (`lsregister`)
4. Record the current default browser (so uninstall can restore it)
5. Call `LSSetDefaultHandlerForURLScheme`; if that doesn't stick, open **System Settings → General**

> macOS security policy may ask you to confirm the default-browser change once in System Settings. This is expected.

---

## Usage

### Command line

```bash
browser-switch https://example.com   # route via rules / show the picker
browser-switch --settings            # open the settings window
browser-switch --installer           # open the install wizard UI
browser-switch --list-browsers       # list detected browsers (⭐ marks the default)
browser-switch --list-profiles       # list each browser's profiles
browser-switch --test https://github.com  # test rule matching without opening a browser
browser-switch --check-default       # check whether it is the system default
browser-switch --install             # install and register as default
browser-switch --uninstall           # uninstall and restore the previous default
browser-switch --version             # version info
```

### Picker interaction

| Input | Action |
| ----- | ------ |
| Left-click a card | Open that browser; **if it has multiple profiles, an account menu appears** |
| Right-click a card | Open the account menu (multi-profile browsers only) |
| `⌘1`–`⌘9` / `1`–`9` | Open the Nth browser directly (uses the default profile) |
| `Enter` | Open the default browser |
| `Esc` | Cancel without opening anything |
| "Remember this domain" | Writes an `exact` rule for the current choice |
| Gear / copy buttons | Open settings / copy the URL to the clipboard |

When the countdown reaches zero it uses the **default browser** (`default_browser` in the config), not whichever card is highlighted.

### Settings window

Three tabs:

- **Browsers** — left: favorites list (reorder / remove; order = ⌘N numbering); right: all browsers (favorite ♥ / hide 👁 / expand accounts / rescan)
- **Rules** — all rules listed by descending priority; add and delete
- **General** — language, default browser, auto-open delay, on-no-match action (show the picker, or open a specific browser directly), install/uninstall, "set another browser as system default"

---

## Rule Matching

| Mode | Matches against | Example |
| ---- | --------------- | ------- |
| `exact` | host equals pattern | `github.com` → only github.com, not sub.github.com |
| `wildcard` | host, with `*` and `?` | `*.google.com` → mail.google.com |
| `regex` | host **or** full URL | `.*\.(test\|staging)\..*` |
| `contains` | host **or** full URL substring | `login` → example.com/login |
| `prefix` | host prefix | `dev.` → dev.example.com |
| `suffix` | host suffix | `.cn` → example.cn |

- Rules are evaluated in **descending `priority`**; the first match wins.
- Matching tries both the raw host and the host with `www.` stripped, so a rule for `example.com` also matches `www.example.com`.
- "Remember choice" rules are fixed at priority `100`; manually added rules default to `50`.

Test any URL without opening a browser:

```bash
browser-switch --test https://mail.google.com/u/1/inbox
```

---

## Multi-Profile & Per-Account Favorites

Browser Switch reads the profiles you have configured in each browser:

- **Chromium family** (Chrome, Edge, Brave, Vivaldi, Opera): from `~/Library/Application Support/<app>/Local State`
- **Firefox**: from `~/Library/Application Support/Firefox/profiles.ini`
- Every multi-profile browser also gets a synthetic **Incognito / Private** entry.

**Per-account favorites.** In the Browsers tab, expand a multi-account browser and click ♥ on any individual account. Favorited accounts appear in the picker as **standalone cards** (titled "Browser · Account") with their own ⌘N number, and clicking one launches that exact profile immediately — no submenu. This is stored as a composite favorite key: `bundleID` for a whole browser, `bundleID#profileID` for a specific account. Deleting a profile silently drops its dangling favorite.

Profiles are launched by executing the browser binary directly with `--profile-directory=` (Chromium) or `-P` (Firefox), because `open -b` ignores those flags when the browser is already running.

---

## Configuration

Config file: `~/.config/browser-picker/config.json` (auto-created on first run with detected browsers).

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
      "comment": "Work sites open in Edge"
    }
  ],
  "auto_close_delay": 5,
  "show_picker_on_miss": true,
  "language": "",
  "prev_default_browser": "com.apple.safari"
}
```

| Field | Description |
| ----- | ----------- |
| `default_browser` | Browser ID (the bundle ID on macOS). Used when no rule matches and the picker is off; also the countdown fallback |
| `favorites` | Picker order. A plain bundle ID = whole browser; `bundleID#profileID` = a specific account. Empty falls back to all (minus `hidden`) |
| `hidden` | Browser IDs hidden from the picker and lists (to suppress falsely-detected non-browser apps) |
| `auto_close_delay` | Countdown seconds; `0` disables auto-open |
| `show_picker_on_miss` | `false` opens the default browser directly on a miss, without the picker |
| `language` | Empty follows the system; otherwise `zh-CN` / `zh-TW` / `en` / `ja` / `ko` / `pt` / `hi` |
| `prev_default_browser` | Recorded at install, restored at uninstall; falls back to Safari if unset |

---

## Architecture

```
main.go            → CLI dispatch, command-line URL path (handleURL), installer UI
config.go          → Config / Browser / Rule / Profile types + JSON persistence
rules.go           → MatchURL rule engine, ValidatePattern, SuggestMatchMode
picker.go          → picker window, countdown, shortcuts, "remember choice"
settings.go        → settings window (three tabs)
gui.go             → shared Fyne components (card, progressLine, icon/text helpers)
constants.go       → app name and bundle ID
browsers_darwin.go → detect browsers via .app + CFBundleURLTypes; launch via open -b
install_darwin.go  → build the .app, ad-hoc sign, LaunchServices default handler (cgo)
urlhandler_darwin.go → receive URLs via Carbon Apple Event (cgo), single-app main loop
profiles_darwin.go → Chromium Local State / Firefox profiles.ini detection & launch
icons_darwin.go    → extract and cache .icns → PNG
i18n/              → 7 embedded locale packs + T/Tf translation helpers
```

**Non-resident by design.** Every click is a fresh cold start that exits when done. Fyne's glfw driver does not forward the Dock "Reopen" event to the Carbon handler, so a resident process would never learn about "click again." Cold starts keep the behavior predictable: clicking the Dock icon always shows settings, clicking a link always shows the picker.

See [PRD.md](PRD.md) for the full product spec and [CLAUDE.md](CLAUDE.md) for architecture constraints.

---

## Building from Source

```bash
git clone https://github.com/kklinan/browser-switch.git
cd browser-switch
make build        # CGO_ENABLED=1 go build -ldflags="-s -w" -o browser-switch .

go test ./...     # run the pure-function test suite
go vet ./...
```

> The `Makefile` provides `build` / `test` / `vet` / `app` / `dmg` / `clean`. `make app` and `make dmg` build the distributable `.app` and DMG (macOS only).

---

## Uninstall

```bash
./browser-switch --uninstall     # restore the previous default browser + delete ~/Applications/Browser Switch.app
rm -rf ~/.config/browser-picker  # remove config
rm -rf /tmp/browser-picker-icons # remove the icon cache
```

---

## Comparison with Alternatives

| | **Browser Switch** | Velja | Browserosaurus | Finicky | Choosy |
| --- | --- | --- | --- | --- | --- |
| Price | **Free, open source** | Free / IAP | Free, open source | Free, open source | Paid |
| Rule engine | **6 modes + priority** | Yes | No (picker only) | Yes (JS config) | Yes |
| GUI rule editor | **Yes** | Yes | — | No (edit JS) | Yes |
| **Per-account / profile routing** | **✅ Yes** | No | No | No | No |
| Picker window | **Yes** | Yes | Yes | No | Yes |
| Native binary | **Yes (Go/AppKit)** | Yes | No (~150 MB Electron) | Yes | Yes |
| Countdown fallback | **Yes** | No | No | No | No |

Browser Switch's niche: **Finicky's rule power + Browserosaurus's GUI simplicity + per-account routing nobody else has.**

---

## FAQ

**How do I set a different default browser on macOS per website?**
Install Browser Switch as your default browser, then add rules mapping domains to browsers (Settings → Rules, or edit the JSON config). Each click is routed automatically.

**Can I open links in a specific Chrome or Firefox profile automatically?**
Yes. Browser Switch detects your browser profiles and lets you favorite individual accounts as standalone picker cards. Automatic per-profile *rules* are on the roadmap ([ROADMAP.md](docs/ROADMAP.md) §3.1).

**Is Browser Switch a good Velja / Finicky / Choosy alternative?**
It's a free, open-source, native alternative. Its distinguishing feature is per-account (profile) routing, which the others don't offer. Finicky requires a JS config file; Browser Switch has a GUI.

**Does it work on Linux or Windows?**
No. It is macOS-only by design — default-browser registration and URL delivery rely on macOS-specific APIs.

**Why does a terminal or WebView app show up in my browser list?**
Any app that declares an `http`/`https` handler in its `Info.plist` is detected as a candidate (the same list macOS uses for "default browser"). Hide false positives in Settings → Browsers.

**Will my links ever get lost?**
No. If no rule matches and you walk away, the countdown opens your default browser automatically.

**Where is the config stored?**
`~/.config/browser-picker/config.json`. It's plain JSON you can edit or sync across machines.

---

## Known Limitations

1. Some non-browser apps with a WebView declare an `http` handler and get detected — hide them in Settings → Browsers.
2. macOS may require a manual confirmation in System Settings when changing the default browser.
3. Newly installed browsers do not appear automatically — click "Rescan" in Settings.
4. The rules UI supports add and delete only; editing, enabling/disabling, and priority changes require editing the JSON directly.
5. The app is ad-hoc signed and not Apple-notarized, so Gatekeeper may prompt on first launch.
6. See [docs/ISSUES.md](docs/ISSUES.md) for the full issue list.

---

## Documentation

| Doc | Purpose |
| --- | ------- |
| [README.md](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) · [한국어](README.ko.md) | User guide (this file) |
| [PRD.md](PRD.md) | Product requirements, verified against the implementation |
| [CLAUDE.md](CLAUDE.md) | Architecture constraints for AI coding assistants (single source of truth) |
| [docs/ISSUES.md](docs/ISSUES.md) | Known issues with code locations |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Product roadmap |

---

## License

[Apache-2.0](LICENSE)
