# Terminal smoke tests

[Development guide](development.md) · [Live-validation record](validation.md)

The PTY suite in `cmd/bambu-tui/pty_test.go` runs on Linux and macOS under ordinary `make test`, including the existing Ubuntu/macOS CI matrix. Its build constraint excludes other operating systems; PTY setup failures on supported hosts fail the tests rather than silently skipping them.

Run just this suite, or repeat it to exercise scheduling races:

```sh
go test -race ./cmd/bambu-tui -run '^TestPTY' -count=1 -v
go test -race ./cmd/bambu-tui -run '^TestPTY' -count=10
```

The parent starts the current Go test executable in a new session with a controlling pseudo-terminal, attaching stdin/stdout/stderr to its slave. The child invokes the real CLI with `--demo` and a generated configuration, which disables automatic modals so results do not depend on the demo's timed runout. It does not authenticate, read saved credentials, contact printers/cloud services, or use the user's configuration. The helper and its environment guard exist only in test builds.

## Covered behavior

- Startup enters raw mode and the alternate screen and shows the connected synthetic farm.
- Raw `j` input changes the selected printer's details, and `m` opens its filament dialog.
- `q` and raw Ctrl+C stay inside the dialog: a subsequent arrow sequence visibly advances the unit selection, proving the dialog and process remained active.
- An actual PTY window-size ioctl plus SIGWINCH produces the undersized resize screen. Restoring the size returns to the dialog with its unit selection intact; Escape returns to the dashboard.
- Dashboard `q` returns normally. OS SIGINT and SIGTERM also return within a bound, separately from raw Ctrl+C keyboard behavior.
- A test-only wrapper around the real UI raises a panic on raw Ctrl+G. Bubble Tea recovers, the dashboard cleanup helper preserves the error, and the child exits nonzero. There is no production fault-injection flag.
- For each exit path, terminal settings match the original state, and output includes alternate-screen exit, cursor restoration, and bracketed-paste disable sequences.

The CLI's signal context owns SIGINT/SIGTERM; Bubble Tea's duplicate signal handler is disabled for the dashboard and login prompts. This avoids its unbuffered quit-message send racing with context cancellation, while preserving raw key dispatch and SIGWINCH handling.

## Synchronization and cleanup

Assertions wait for newly emitted semantic text or explicit completion signals, with five-second step bounds and a thirty-second child lifetime limit. They use no timing sleeps or full ANSI-frame golden files. The transcript is capped at 1 MiB. A minimal cursor-position reply lets startup color probes fall back from unsupported OSC queries; the harness does not implement a screen emulator.

After `Program.Run` and monitor cleanup return, the child signals a private control pipe and waits for the parent's acknowledgement through another pipe. The parent inspects terminal settings at that point, then releases process exit and checks the status and cleanup sequences. This handshake is necessary because macOS revokes a controlling slave terminal when its session leader exits. It also avoids competing with the app's terminal input reader. The child must still exit within five seconds of acknowledgement.

Failure cleanup kills the isolated child process group, waits for the child, closes PTY/control descriptors, and joins readers. The master uses a nonblocking descriptor registered with Go's poller so closing it can unblock pending reads/writes.

These tests establish synthetic terminal integration behavior. They do not establish visual correctness in every terminal emulator, color/theme accessibility, real credential-store behavior, or hardware/network validation. Keep those observations separate in the live-validation record.
