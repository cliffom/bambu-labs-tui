# Terminal smoke tests

[Development guide](contributing.md) · [Live-validation record](validation.md)

The PTY suite in `cmd/bambustand/pty_test.go`, `webserver_pty_test.go`, `serve_pty_test.go`, and `startup_login_pty_test.go` runs on Linux and macOS under ordinary `make test`, including the existing Ubuntu/macOS CI matrix. Its build constraint excludes other operating systems; PTY setup failures on supported hosts fail the tests rather than silently skipping them.

Run just this suite, or repeat it to exercise scheduling races:

```sh
go test -race ./cmd/bambustand -run '^TestPTY' -count=1 -v
go test -race ./cmd/bambustand -run '^TestPTY' -count=10
```

The parent starts the current Go test executable in a new session with a controlling pseudo-terminal, attaching stdin/stdout/stderr to its slave. The child invokes the real CLI with `--demo` and a generated configuration, which disables automatic modals so results do not depend on the demo's timed runout. The startup-login fixture separately exercises the shared startup flow, real prompts, email-code authentication, and account validation against synthetic local HTTP responses, then opens the demo dashboard with the validated account checked at the handoff. It uses a fake credential store. No fixture reads saved credentials, contacts real printers/cloud services, or uses the user's configuration. The helper and its environment guard exist only in test builds.

## Covered behavior

- Startup enters raw mode and the alternate screen and shows the connected synthetic farm.
- Minimum-size Help scrolls through all controls, captures quit/Ctrl+C, suspends for Status / Feedback on raw Ctrl+E, and restores its position through status and below-minimum resizing.
- Raw `j` input changes the selected printer's details, and `m` opens its filament dialog.
- `q` and raw Ctrl+C stay inside the dialog: a subsequent arrow sequence visibly advances the unit selection, proving the dialog and process remained active. Ctrl+E opens status and returns to the same selected unit.
- An actual PTY window-size ioctl plus SIGWINCH produces the undersized resize screen. Restoring the size returns to the dialog with its unit selection intact; Escape returns to the dashboard.
- Config accepts raw Tab/arrow/Enter input across General, Alerts, and Printers, rejects fractional percentage input, preserves its state across minimum-size resizing, loads a custom JSON theme relative to its configuration directory under `NO_COLOR`, and persists general/global/per-printer edits while preserving `theme_file` in the launch configuration file before quitting.
- Web Server starts a real localhost HTTP listener on its configured port from Settings, reports occupied-port failures with explicit same-port retry, serves allowlisted synthetic snapshots, preserves its running URL through fallback/restoration, stops on disable, saves enabled state on quit, and starts from saved preferences. Normal quit and SIGTERM both close its listener. Hidden toggle/retry behavior is covered synchronously in UI reducer tests.
- Launch-only `--theme default`, `--theme gruvbox-dark`, `--theme hotdog`, `--theme c64`, and `--theme ibm3270` bypass a missing configured theme, and a subsequent farm-view save retains the original `theme_file`.
- Uppercase `N` opens notification activity with its empty-state guidance. The dialog survives resize, captures quit/Ctrl+C, and closes back to the dashboard.
- Dashboard `q` returns normally. OS SIGINT and SIGTERM also return within a bound, separately from raw Ctrl+C keyboard behavior.
- A test-only wrapper around the real UI raises a panic on raw Ctrl+G. Bubble Tea recovers, the dashboard cleanup helper preserves the error, and the child exits nonzero. There is no production fault-injection flag.
- For each exit path, terminal settings match the original state, and output includes alternate-screen exit, cursor restoration, and bracketed-paste disable sequences.

The CLI's signal context owns SIGINT/SIGTERM; Bubble Tea's duplicate signal handler is disabled for the dashboard and login prompts. The shared terminal runner translates context cancellation into a graceful quit, letting Bubble Tea join its input reader before closing cancellation descriptors. It preserves a cancellation error for callers and leaves unexpected errors intact. This avoids both competing quit-message sends and the v1 hard-cancel input-reader close race, while preserving raw key dispatch and SIGWINCH handling.

## First-launch login coverage

`startup_login_pty_test.go` checks email/code prompts, masked code input, validated account handoff into the demo dashboard, verified-save feedback, session-only fallback, existing-session prompt bypass, unavailable-store guidance, rejected codes, request/account-validation failures, and Escape/raw Ctrl+C cancellation at both prompts. Prompts begin at 120 × 40; successful login resizes to 60 × 18 before entering email/code and opening the dashboard. Every path checks terminal restoration; only dashboard paths require alternate-screen entry/exit. Unit tests cover noninteractive missing/saved sessions, invalid stores, cancelled lookup, and minimum-width secret masking. These seams exercise actual cloud authentication with fake HTTP and credential persistence results, not native storage or live monitoring.

## Foreground serve coverage

`serve_pty_test.go` runs the real `serve --demo` CLI at 120 × 40, then resizes to 80 × 24, 60 × 18, 40 × 12, 20 × 10, and back. It checks the URL, synthetic mode, connection, shutdown guidance, canonical Ctrl+C, listener closure, cursor/screen restoration, and unchanged terminal input settings. A synthetic cloud-mode runner exercises disconnect/reconnect and typed authentication rejection with private diagnostic sentinels; an occupied-port startup checks ordinary error output without entering an alternate screen. A TrueColor serve PTY checks the default accent/healthy color, a real keepalive request with IP/User-Agent in the ledger, grouped disconnect after client close/grace, active counts, and SIGTERM cleanup. Serve never enables raw mode or bracketed paste, so its restoration assertions cover the modes it actually uses.

`serve_status_test.go` checks bounded/coalesced redirected diagnostics, exclusion of private event/storage strings, persistent warnings, width/height budgets through 1 × 1, TrueColor/ANSI256/no-color layouts, once-only plain client events, and cleanup after presentation errors. Webserver ledger regressions cover keepalive reuse, sanitized/missing user agents, first-request capture, parallel socket grouping, no-request socket exclusion, replacement grace, listener-stop retirement, socket IPs rather than forwarded addresses, immutable snapshots, bounded/concurrent tracking, browser DTO exclusion, and actual HTTP disable/closure. Existing terminal-free child-process, persistence, and rejection tests continue to exercise non-TTY behavior. These fixtures use no saved credentials or live cloud connection.

## Synchronization and cleanup

Assertions wait for newly emitted semantic text or explicit completion signals, with five-second step bounds and a thirty-second child lifetime limit. They use no timing sleeps or full ANSI-frame golden files. The transcript is capped at 1 MiB. A minimal cursor-position reply lets startup color probes fall back from unsupported OSC queries; the harness does not implement a screen emulator.

Wait for the dashboard to render after Escape before sending `q`; adjacent Escape and `q` bytes can decode as Alt+Q instead of two keys.

Do not send keys with no observable acknowledgement immediately before resizing: PTY writes and SIGWINCH do not guarantee reducer processing order, so queued keys can arrive after the supported size is restored. Keep hidden-input guard assertions in the synchronous UI reducer tests; PTY tests verify the fallback screen and restored dialog state.

After `Program.Run` and monitor cleanup return, the child signals a private control pipe and waits for the parent's acknowledgement through another pipe. The parent inspects terminal settings at that point, then releases process exit and checks the status and cleanup sequences. This handshake is necessary because macOS revokes a controlling slave terminal when its session leader exits. It also avoids competing with the app's terminal input reader. The child must still exit within five seconds of acknowledgement.

Failure cleanup kills the isolated child process group, waits for the child, closes PTY/control descriptors, and joins readers. The master uses a nonblocking descriptor registered with Go's poller so closing it can unblock pending reads/writes.

These tests establish synthetic terminal integration behavior. They do not establish visual correctness in every terminal emulator, color/theme accessibility, real credential-store behavior, or hardware/network validation. Keep those observations separate in the live-validation record.
