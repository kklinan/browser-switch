# Browser Switch v1.0.1

macOS default browser picker & per-site URL router (Go + Fyne, macOS 10.14+, Apache 2.0)

## Core Features

- **Default browser interception**: Set as default browser and every link you click is routed by your own rules — work links open in Edge, personal links in Chrome, dev links in Firefox, automatically
- **Rule engine**: 7 matching modes + priority ordering, with site/domain-level rules; can target a specific browser account
- **Multi-account & multi-profile routing**: Distribute links by browser account/profile, including sub-accounts and incognito (e.g. `*.company.com` → Chrome · Work, `*.personal.com` → Chrome · Personal, all `*.pdf` → Firefox · Incognito)
- **Keyboard-first card picker**: A fast chooser pops up when no rule matches, with keyboard support and countdown fallback
- **3-tab settings window**: Rules management, browser management, and general settings
- **7 languages**: en / zh-CN / zh-TW / ja / ko / pt / hi, auto-detected from system locale (`AppleLocale`), with manual override in settings

## Changes in This Release

### Rule engine

- **Account-aware rules**: Rules can target a specific browser account (e.g. all `*.company.com` links open in Chrome · Work); the rule list and add/edit dialog list each account as its own entry
- **"Remember choice" remembers the account**: previously only the browser was stored, so sub-account / incognito selections silently defaulted back to the default account on every later visit
- **New matching mode `urlequal`**: full URL equal (scheme + path + query + fragment), case-insensitive — highest specificity
- **Wildcard & quick modes now match the full URL**: `*.pdf`, `https://`, `*/settings` patterns finally fire, because those tokens don't show up in a hostname
- **Rules can be edited in place** (pencil button per rule), reusing the add-rule dialog
- **Rules can force "open in a new window"**: Chromium/Firefox use `--new-window`; Safari uses AppleScript (because `open -n` is silently ignored by all three)
- **Inline validation** when saving a rule: catches patterns that can never match (urlequal without scheme, exact with a path, `*`/`?` inside the quick modes where they're literal), duplicate rules, and write failures

### Picker & settings

- **`⌘R` expands the full browser list** in the picker, same as clicking the "More" card
- **Click the URL in the picker header** to copy the full URL to the clipboard
- **Incognito is always reachable**: Chromium / Firefox browsers always expose an Incognito entry, so single-account users can right-click the card to open one
- **Picker card click and `⌘N` keyboard shortcut now agree**: both pop the account menu when the browser has several real accounts; both open directly when it has only one
- **Settings pauses the picker countdown**: opening settings from the gear button no longer gets immediately closed by the auto-open countdown
- **Settings is now a single reusable window**: re-opening it reloads the config from disk first; the rule dialog is likewise a single window (no stacking)

### Packaging

- **DMG installer ships with a branded background** and Finder-style icon layout (`assets/dmg-background.png`)
- **Bundle version bumped to 1.0.1** in both `Info.plist` templates and the seven locale `app.version` entries

### Cleanup & fixes

- Wildcard regexes are compiled once and cached, so each match no longer triggers a fresh `regexp.Compile`
- Loading the config silently drops rules with empty patterns and falls back to `exact` for unknown mode strings (previously they stayed in the list and never matched)

## Tech Stack

| Item     | Value                                 |
| -------- | ------------------------------------- |
| Language | Go 1.23+ (CGO: CoreServices / Carbon) |
| GUI      | Fyne v2.7                             |
| Platform | macOS 10.14+                          |
| License  | Apache 2.0                            |

## Upgrade Notes

- **No config migration required.** Existing rules have no `profile` field; it deserializes as an empty string (= no account specified), which behaves identically to v1.0.0.
- **Upgrade by re-installing from the DMG** (fresh installs follow the same steps). The bundle version is bumped to **1.0.1**:

  1. Download the DMG for your Mac — see **Release Artifacts** below.
  2. Open the DMG and drag **Browser Switch** onto the **Applications** shortcut.
  3. Choose *Replace* if macOS asks — your rules live in `~/.config/browser-switch/config.json`, so nothing is lost.
  4. Open **Browser Switch** from Applications. The build is ad-hoc signed, so Gatekeeper may block the first launch: right-click and choose **Open**, or allow it under *System Settings → Privacy & Security → Open Anyway*.
  5. In the settings window that opens, go to **General → Set as Default Browser**.
  6. If macOS asks for confirmation, pick *Browser Switch* under *System Settings → Desktop & Dock → Default web browser*.

  > Building from source instead? `./browser-switch --install` performs steps 2–6 in one shot.

- **Existing "remember choice" rules don't auto-gain an account.** To make an existing rule use a specific account or incognito, edit it and pick the account from the browser dropdown.

## Release Artifacts

- `dist/BrowserSwitch-1.0.1-amd64.dmg` — Intel Mac (x64)
- `dist/BrowserSwitch-1.0.1-arm64.dmg` — Apple Silicon
- `dist/BrowserSwitch-1.0.1-universal.dmg` — Universal (both architectures)
