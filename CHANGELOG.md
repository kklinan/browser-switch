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

### Changed
- Rewrote the `Makefile`: removed the non-compiling `build-gtk4` / `build-windows`
  targets and added `test` / `vet` / `clean` / `app` / `dmg`.
- Documentation synchronized to the single-app, macOS-only, Fyne v2.7 reality
  across all four README translations and `CLAUDE.md`.

### Removed
- Duplicate `en-US.json` locale (byte-identical to `en.json`); the locale set is
  now 7 languages. `en_US` still resolves to `en` via locale normalization.
- Dead code and never-read config fields (`remember_choice`, `skip_installed`,
  `tray_icon`, `window_width`, `window_height`, and several unused helpers).

### Fixed
- `.gitignore` now ignores the current binary name `browser-switch` (the stale
  pre-rename binary name) plus `dist/` and `.DS_Store`.

## [1.0.0]

- Initial public release: macOS default-browser picker with a per-site rule
  engine (6 modes + priority), per-account/profile routing, a keyboard-first
  card picker with countdown fallback, and a 3-tab settings window.
