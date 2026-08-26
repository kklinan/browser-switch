# Contributing to Browser Switch

Thanks for your interest in improving Browser Switch. This document covers the
practical basics; the architecture rules that every change must respect live in
[CLAUDE.md](CLAUDE.md) — read it before writing code.

## Ground rules

- **macOS only.** All platform code is guarded by `//go:build darwin`. There are
  no `*_linux.go` / `*_windows.go` files and none should be added.
- **CGO is mandatory.** The project links CoreServices, CoreFoundation, and
  Carbon. `CGO_ENABLED=0` will not compile.
- **GUI is Fyne v2.7**, single-app architecture. See CLAUDE.md §2 for the
  lifecycle and the Fyne conventions you must follow (background UI updates via
  `fyne.Do`, set values before wiring `OnChanged`, etc.).
- **Comments are written in Chinese**, matching the existing codebase. Match the
  comment density and naming style of the surrounding code.

## Development setup

```bash
# Xcode Command Line Tools provide the cgo headers
xcode-select --install

make build        # CGO_ENABLED=1 go build -ldflags="-s -w" -o browser-switch .
make test         # go test ./...
make vet          # go vet ./...
```

Config lives at `~/.config/browser-switch/config.json`.

## Making changes

1. **Read the file before editing it.** The `_darwin.go` files carry extensive
   comments explaining why the non-obvious approach is required.
2. **Prefer pure functions.** New logic should be testable without the GUI. The
   test suite (`*_test.go`) covers pure functions only — extend it there.
3. **Follow KISS / YAGNI / DRY / SRP.** See CLAUDE.md §5 for how these are
   applied in this repo. In particular, do not add config fields that nothing
   reads.
4. **Watch the two URL-decision paths.** Rule-decision logic exists in both
   `handleURL()` (CLI) and `openPicker()` (Apple Event). Changing one usually
   means changing the other — see CLAUDE.md §2.1 and ISSUES I-1.
5. **i18n:** new UI strings must be added to all 7 locale packs in
   `i18n/locales/*.json`. CLAUDE.md §3.6 has a script to detect missing keys.

## Before you open a PR

- `make build` succeeds (CGO on).
- `make vet` is clean.
- `make test` passes.
- New user-facing strings exist in all 7 locales.
- Comments are in Chinese and match the surrounding style.

## Dangerous operations

Some commands modify system state (the default browser, `~/Applications`) — they
are listed in CLAUDE.md §6. Do not wire tests or CI to run `--install` /
`--uninstall` against a real machine.

## Reporting bugs & requesting features

Use the GitHub issue templates. For security-sensitive reports, follow
[SECURITY.md](SECURITY.md) instead of filing a public issue.
