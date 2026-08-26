# Security Policy

## Supported versions

Browser Switch is a small macOS utility released from a single line of
development. Security fixes are applied to the latest release only.

| Version | Supported |
| ------- | --------- |
| 1.0.x   | ✅        |
| < 1.0   | ❌        |

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security problems.

Instead, report privately through GitHub's
[private vulnerability reporting](https://github.com/kklinan/browser-switch/security/advisories/new)
("Security" tab → "Report a vulnerability"). Include:

- affected version (`browser-switch --version`) and macOS version,
- a description of the issue and its impact,
- steps to reproduce, if available.

We aim to acknowledge reports within a few days and to coordinate a fix and
disclosure timeline with you.

## Scope notes

Browser Switch runs entirely on the local machine. It registers as the system
`http`/`https` handler and launches other browsers; it makes no network requests
of its own. Points worth keeping in mind when assessing impact:

- The app is **ad-hoc signed and not Apple-notarized** (see ISSUES I-26).
  Gatekeeper may prompt on first launch; this is expected, not a vulnerability.
- Config is plain JSON at `~/.config/browser-picker/config.json`. Rules can
  contain user-authored regular expressions that are compiled at match time.
- The app shells out to built-in macOS tools (`open`, `plutil`, `sips`,
  `codesign`, `xattr`, `lsregister`). Reports about argument handling around
  these are in scope.
