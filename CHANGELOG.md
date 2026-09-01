# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Community/governance files: `CONTRIBUTING.md`, `SECURITY.md`, `NOTICE`,
  this changelog, and GitHub issue/PR templates.
- macOS packaging: `make app` and `make dmg` build a distributable `.app`
  bundle and DMG (see `scripts/build-app.sh`, `scripts/build-dmg.sh`).
- System-language detection now implemented via `AppleLocale`, so an installed
  `.app` follows the system region even without shell environment variables.
- System accent color is read from `AppleAccentColor` at startup, so the UI
  follows the user's Appearance setting instead of a hard-coded blue.
- `gui.go` now holds a design system (8pt spacing grid, 11/12/13/15/20 type
  scale, 6/10/14 corner radii, Apple system colors) that all panes share.
- macOS-style switch (`appleSwitch`) replaces checkboxes; `sidebarItem`,
  `groupBox`, `pill`, `tileShadow` and `emptyState` are new shared controls.
- Rule pane gained a search box and a "showing N of M rules" counter.
- `ui_smoke_test.go`: offscreen smoke tests that render every pane and assert
  window sizes stay within bounds (including the 24-tile scrolling case).
- `ui_static_test.go`: static checks that every i18n key used in code exists in
  all 7 locale files (and no dead keys remain), that no emoji is used for UI
  icons, and that changed files are gofmt-clean.

### Fixed
- "Add rule" now opens at most one window: repeated clicks raise the existing
  window instead of stacking duplicates on top of each other (which made the
  previously entered text look lost).
- Drop-down selects in the general pane no longer resize as you change the
  selected item, which used to shove the row's label sideways. Their width now
  covers the longest option plus a 5-character margin.
- Picker layout no longer lets a tile overflow onto the footer. Fyne's
  `GridWrap` derives its column count as `floor((w+pad)/(cell+pad))`, which
  sits exactly on an integer and drops a whole column on float32 rounding —
  the last tile wrapped to a second row that our row count didn't know about.
  The grid now uses `GridLayout` (explicit column count, width divided evenly).
- The picker window is now sized to fit its content exactly. It used to be
  short (clipping the footer) because `GridWrap.MinSize` only reports the row
  count cached from the previous layout, and narrow by 8px because `Border`
  reserves `2×theme.Padding()` for the middle region even with no left/right
  children.
- The "remember this choice" switch keeps its 38×22 shape instead of being
  stretched to the row height by `HBox`.
- Closing the main picker now closes every popup window it spawned (Settings,
  Add Rule, …). Implemented as a `trackedWindows` registry since Fyne's
  `fyne.Window` interface has no stable identity for `==` comparison across
  drivers.
- Countdown text stays in the secondary text color for the whole duration
  instead of flashing red in the last 3 seconds; the progress bar alone is
  the urgency cue.

### Changed
- Rewrote the `Makefile`: removed the non-compiling `build-gtk4` / `build-windows`
  targets and added `test` / `vet` / `clean` / `app` / `dmg`.
- Documentation synchronized to the single-app, macOS-only, Fyne v2.7 reality
  across all four README translations and `CLAUDE.md`.
- **Picker**: replaced the address bar + title banner with a link header
  (host in bold, full URL beneath), moved the countdown to a progress line
  pinned to the window's bottom edge, and switched the grid to fixed-size
  tiles. Keyboard navigation now covers ←/→/↑/↓ plus Return/Space to open;
  more than 5 browsers collapse behind a "Show N More" disclosure.
- **Settings**: replaced the top tab bar with a macOS System Settings style
  sidebar, and grouped the general pane's rows into rounded cards.
  Picker auto-open delay changed from a numeric entry to a slider (0–15s).
- **Installer**: emoji icons (`🚀 ✅ 👥 ⚡ ⏱️`) and status dots replaced with
  themed vector icons; uninstall is now a destructive (red) button.
- Settings window split by pane into `settings.go` (shell + shared rows),
  `settings_browsers.go`, `settings_rules.go`, `settings_general.go`.
- Window sizing is measured at runtime rather than hard-coded, so panes no
  longer clip or leave gaps in other languages.

### Removed
- Duplicate `en-US.json` locale (byte-identical to `en.json`); the locale set is
  now 7 languages. `en_US` still resolves to `en` via locale normalization.
- Dead code and never-read config fields (`remember_choice`, `skip_installed`,
  `tray_icon`, `window_width`, `window_height`, and several unused helpers).
- Unused i18n keys `picker.title`, `picker.more`, `picker.more_detail`,
  `settings.browsers.favorited`, `settings.browsers.unfavorited`,
  `settings.general.title` (the sidebar now carries the pane name),
  plus pre-existing dead keys `app.settings_title`, `common.ok`,
  `settings.browsers.hide`, `settings.browsers.show`. Locale files are now
  143 keys each; `ui_static_test.go` enforces this going forward.
- Emoji from the UI: installer icons now use themed vector resources, and
  `--list-browsers` marks the default browser with `*` instead of `⭐`
  (stable column alignment when piping output).

### Fixed
- `.gitignore` now ignores the current binary name `browser-switch` (the stale
  pre-rename binary name) plus `dist/` and `.DS_Store`.

## [1.0.0]

- Initial public release: macOS default-browser picker with a per-site rule
  engine (6 modes + priority), per-account/profile routing, a keyboard-first
  card picker with countdown fallback, and a 3-tab settings window.
