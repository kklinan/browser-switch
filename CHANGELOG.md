# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- New match mode `urlequal` (完整网址相等): compares the full URL including
  scheme / path / query / fragment, case-insensitively.
- The add/edit rule dialog now shows a live hint under the match-mode field,
  including the equivalent wildcard form for the quick modes.
- Rules can now force **open in new window** (`open_in_new_window`), supported
  by both `LaunchBrowser` and `LaunchBrowserProfile`.
- Existing rules can be **edited** in place (pencil button on each rule row)
  — the add-rule dialog is reused for editing.
- `⌘R` in the picker expands the full browser list, exactly like clicking the
  "更多" card.
- Clicking the URL in the picker header copies the full URL to the clipboard.
- Community/governance files: `CONTRIBUTING.md`, `SECURITY.md`, `NOTICE`,
  this changelog, and GitHub issue/PR templates.
- macOS packaging: `make app` and `make dmg` build a distributable `.app`
  bundle and DMG (see `scripts/build-app.sh`, `scripts/build-dmg.sh`).
- System-language detection now implemented via `AppleLocale`, so an installed
  `.app` follows the system region even without shell environment variables.

### Changed
- Rewrote the `Makefile`: removed the non-compiling `build-gtk4` / `build-windows`
  targets and added `test` / `vet` / `clean` / `app` / `dmg`.
- Documentation synchronized to the single-app, macOS-only, Fyne v2.7 reality
  across all four README translations and `CLAUDE.md`.
- `contains` / `prefix` / `suffix` stay first-class match modes — they are the
  plain-language shortcuts to `wildcard` (`*p*` / `p*` / `*p`), kept for users
  who don't want to hand-write globs or regexes. They are now grouped together
  and marked as quick modes in the UI.
- The three quick modes now also match against the **full URL**, not just the
  host. Previously `.pdf` or `https://` rules could never fire because those
  strings never appear in a hostname.
- Manually picking a match mode is now respected: the dialog only auto-suggests
  a mode until the user overrides it, so typing another character no longer
  resets a mode you chose by hand.
- Opening the settings window from the picker's gear button now **pauses the
  auto-close countdown** instead of being interrupted by it.
- The "更多" card in the picker now advertises the `⌘R` shortcut.

### Removed
- Duplicate `en-US.json` locale (byte-identical to `en.json`); the locale set is
  now 7 languages. `en_US` still resolves to `en` via locale normalization.
- Dead code and never-read config fields (`remember_choice`, `skip_installed`,
  `tray_icon`, `window_width`, `window_height`, and several unused helpers).

### Fixed
- `.gitignore` now ignores the current binary name `browser-switch` (the stale
  pre-rename binary name) plus `dist/` and `.DS_Store`.
- Editing a rule no longer loses its match mode. The dialog used to re-run the
  mode auto-suggestion while pre-filling the pattern, silently overwriting the
  saved mode (e.g. a `.pdf` suffix rule came back as "domain equal").
- Wildcard rules now match against the full URL in addition to the host, so
  patterns containing a path or scheme (`*/settings`, `https://*`) work instead
  of never firing. Compiled wildcard regexps are cached.
- "Open in new window" now actually opens a new window. `open -n` alone is
  ignored by single-instance apps like Chrome, Edge and Safari, so Chromium and
  Firefox are now invoked with `--new-window` and Safari via AppleScript.
- Saving a rule validates it and reports the problem inline instead of silently
  accepting rules that can never match: `urlequal` without a scheme, `exact`
  containing a scheme or path, `*`/`?` inside the quick modes (where they are
  literal), invalid regexes, duplicate rules, and config write failures.
- Repeatedly clicking "add rule" or the edit button on several rules no longer
  stacks up multiple dialogs — a single rule dialog is reused and always shows
  the most recent request.
- Re-opening the settings window reloads the configuration from disk first, so
  changes made elsewhere are no longer shown as stale (and no longer written
  back over them).

## [1.0.0]

- Initial public release: macOS default-browser picker with a per-site rule
  engine (6 modes + priority), per-account/profile routing, a keyboard-first
  card picker with countdown fallback, and a 3-tab settings window.
