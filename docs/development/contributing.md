# Contributing

[Back to the README](../../README.md)

Read [UI direction](ui-direction.md) before UI changes and [documentation direction](documentation-direction.md) for documentation ownership and required PR evidence.

Requires Go 1.27.3 or newer and Python 3.10+ for documentation checks. Bubble Tea v1, Bubbles, and Lip Gloss are pinned in `go.mod`.

```sh
make docs-check # documentation policy, links, and checker regression tests
make test    # tests with the race detector
make check   # go vet
make vuln-check # known vulnerabilities for the current target/toolchain (network required)
make fmt     # format Go source
make fmt-check # check formatting without changing files
make lint    # golangci-lint
make lint-actions # actionlint for GitHub Actions workflows
make lint-make # checkmake for the Makefile
make audit-faults # verify fault codes against pinned upstream catalogues (network required)
make build   # bin/bambu-tui
make auth    # build and log in using an email code
make run     # build and open the dashboard
make demo    # build and open the simulated farm without login
make cross   # macOS/Linux, arm64/amd64 binaries in dist/
make notify-helper # universal branded helper beside bin/bambu-tui (macOS + Swift)
make release # all archives/checksums/licenses; requires macOS + Swift
```

Both `make run` and `make demo` accept `RUN_ARGS` and a launch-only `THEME` choice: `make run THEME=gruvbox-dark`, `make demo THEME=gruvbox-dark`, `make demo THEME=hotdog` for a light red/yellow theme, `make demo THEME=c64` for Commodore-inspired blues, `make demo THEME=ibm3270` for monochrome greens, `make demo THEME=deskmate` for the DeskMate II blue/gray palette, or `make run THEME=./my-theme.json`. Custom theme paths resolve from the working directory; `THEME` overrides a `--theme` in `RUN_ARGS` without changing saved preferences. To use a repository-local configuration for synthetic notification testing, run `make demo RUN_ARGS='--config ./config.toml'`. The default configuration location still applies when no `--config` argument is supplied.

`VERSION` is the release version. Make injects version, commit, and UTC build date into `--version`; a source build without Git metadata reports `unknown` for its commit. CI runs pinned vulnerability scans, race tests, and vet on Linux/macOS using the Go version in `go.mod`, checks native credential persistence on macOS, and packages release candidates on macOS so the notification helper can be compiled and signed. A pushed version tag creates a **draft** release for review; it does not publish automatically.

`make vuln-check` runs govulncheck pinned in `.govulncheck-version` through `go run`; override `GOVULNCHECK` for an installed binary. It queries the Go vulnerability database and fails for reachable known advisories. Run it with the same patched Go toolchain used to build the application; an advisory-free local scan with a newer toolchain does not validate an older release toolchain. CI scans native macOS and Linux targets, and the tag workflow scans before packaging. Keep the Go version in `go.mod` and these setup requirements current when security patches are released.

The separate **Formatting and lint** workflow checks Go formatting, type declaration grouping, error handling, vet diagnostics, ineffective assignments, static analysis, unused code, GitHub Actions syntax, and Makefile rules on pull requests and pushes to `main`. It reports formatting problems without rewriting source. Run `make fmt` locally to fix Go formatting. Checkmake validates the Makefile; it does not format or rewrite it.

`make fmt` and `make fmt-check` use gofumpt pinned in `.gofumpt-version`, running it through `go run` without requiring a global installation. The first run may download the formatter and its dependencies. Override `GOFUMPT` to use an installed binary. Gofumpt adds spacing between multiline top-level declarations and groups adjacent declarations. Keep a blank line between functions, including short helpers, and collect each file's package-level types in one `type (...)` block when it declares multiple types. The `decorder` linter enforces the single type declaration; type grouping itself requires a manual edit. Separate constant blocks remain allowed so independent `iota` sequences retain their meaning.

For VS Code, enable `"gopls": { "gofumpt": true }` in your settings to use gofumpt formatting on save. The version bundled with gopls may differ from the project pin; `make fmt-check` is the authoritative formatting check.

Install the versions in `.golangci-lint-version` and `.actionlint-version` to reproduce the lint checks locally. Get golangci-lint from its [official releases](https://github.com/golangci/golangci-lint/releases), and install actionlint with `go install "github.com/rhysd/actionlint/cmd/actionlint@$(cat .actionlint-version)"`. `make lint` and `make lint-actions` use tools on `PATH`; override `GOLANGCI_LINT` or `ACTIONLINT` when installed elsewhere. Workflow lint also uses ShellCheck when it is available.

Install the pinned [checkmake](https://github.com/checkmake/checkmake) version with `go install "github.com/checkmake/checkmake/cmd/checkmake@$(cat .checkmake-version)"`. Run `make lint-make`, or override `CHECKMAKE` if the binary is installed elsewhere. `checkmake.ini` requires the project's build, auth, run, test, and check targets to exist and be declared phony; other lint rules use their defaults.

## Theme files

For personal themes, edit a JSON file and run the existing binary with `./bin/bambu-tui --theme ./my-theme.json`. Restart to reload it; there is no registration step or build requirement. `make run` and `make demo` remain source-build shortcuts. See the [custom theme format](../user/usage.md#custom-themes).

To ship another bundled example, add a complete `internal/ui/themes/<name>.json` and build. A wildcard `embed.FS` discovers each JSON filename as its theme name; CLI help and generic JSON/canvas/contrast/PTY checks use that inventory. No per-theme Go variable, switch branch, or help-list entry is needed. Keep examples within the palette/contrast contracts in [UI direction](ui-direction.md#palette-and-message-emphasis), and document the design and actual inspection evidence. Explicit custom file paths use the same validator while remaining independent of the bundled examples.

## macOS notification helper

From a source checkout on macOS with Xcode Command Line Tools (Swift compiler) and Python 3.10+, run:

```sh
make build
make notify-helper
./bin/bambu-tui doctor --notify
"./bin/Bambu TUI Notify.app/Contents/MacOS/bambu-notify" --status
"./bin/Bambu TUI Notify.app/Contents/MacOS/bambu-notify" --test
"./bin/Bambu TUI Notify.app/Contents/MacOS/bambu-notify" --test --sound
```

The helper build requires macOS, Xcode Command Line Tools (Swift), and Python 3.10+. It creates a universal arm64/x86_64 app under ignored `bin/`, wraps the existing 32/64/512 PNG logos in an ICNS file without resampling, sets the app version from `VERSION`, ad-hoc signs the bundle, and verifies both architectures and signatures. It does not install into Applications. Doctor registers the adjacent bundle with Launch Services before invoking it; registration failure stops the test. The helper declares macOS 12 as its minimum; actual host results belong in the [validation record](validation.md).

`make release` runs on macOS, cross-builds all four Go targets, and builds a separate universal helper under `dist/notify-helper/`. Only macOS archives include that bundle, beside `bambu-tui`. Downloads use `macos_arm64`/`macos_amd64` filenames from v0.2.1; intermediate Go binaries keep `darwin` targets. Archives contain only runtime files, a short installation README, the configuration example, and `THIRD_PARTY_NOTICES.txt`. Packaging runs synthetic inventory regression tests, validates archive inventory/metadata/architectures, and verifies the signatures and help command from each extracted macOS bundle. CI and tag-release packaging run on macOS; Linux still runs Go checks and produces a Linux-compatible monitor binary. Developer ID signing and notarization are opt-in release modes using MACOS_SIGN_IDENTITY and MACOS_NOTARY_PROFILE; plain local builds and ordinary PR/main CI remain ad-hoc candidates; tag/manual release builds require signing secrets. Signed mode covers both macOS CLIs and the helper, verifies extracted signatures, and validates stapled tickets when notarized. Clean-Mac Gatekeeper validation remains required; see [publishing](publishing.md).

The helper uses bundle ID `com.github.cliffom.bambu-tui.notify-poc` and display name Bambu TUI. It requests its own notification permission through Apple's UserNotifications framework. Allow notifications in the system prompt; if denied or the request returns "not allowed," enable Bambu TUI in System Settings → Notifications and retry `--test`. If the request is accepted but no banner appears, check Notification Center and the active Focus: temporarily disable Focus or allow Bambu TUI through that Focus, then retry. Enabled app notification settings do not establish that Focus permits interruption. The permission prompt itself is not evidence of normal banner delivery. The helper reports permission denial as exit 3, other framework failures as exit 4, and a 60-second permission/service timeout as exit 5. Bad arguments and SSH use exit 2. `--status` inspects authorization, alert/sound settings, and the effective preview setting without requesting permission or sending a notification; it does not diagnose Focus suppression. An effective preview setting of Never can hide both the supplied title and body behind generic notification text.

Each explicit `--test` requests one immediate synthetic notification; `--sound` requests the default system notification sound. Command acceptance does not establish banner/icon/audio delivery. Inspect the logo and notification text with the terminal focused and unfocused, check sound separately, and record suppression/denial outcomes. Starting the app without arguments, including a notification click that relaunches it, prints help and exits without sending another notification. The helper itself never connects to Bambu, schedules future notifications, starts a monitor, or persists a background service. The opted-in TUI worker uses `--deliver` with bounded JSON stdin and requires existing permission. `doctor --notify` invokes this helper beside the resolved CLI executable, passing `--test` and optional `--sound`. Doctor reports missing-helper build guidance, permission denial, and service timeout; it has a 65-second command deadline and terminates the helper on cancellation. There is no AppleScript fallback. See the alert guide for opt-in automatic delivery.

To remove a locally built helper, unregister it before deleting its generated app bundle:

```sh
/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -u "$PWD/bin/Bambu TUI Notify.app"
rm -r "./bin/Bambu TUI Notify.app"
```

macOS may retain its notification settings/history after removal. The helper does not reset permissions or remove existing notifications automatically.

## Test evidence

For browser notice visibility changes, run `node --test tools/test-browser-notices.cjs` with Node.js 18 or newer. These dependency-free tests exercise the dashboard script with a minimal DOM stand-in; use a real browser separately to inspect layout and keyboard behavior.

On Linux/macOS, `make test` also runs the [PTY smoke suite](terminal-tests.md), which exercises the real demo CLI and terminal lifecycle without account access. Tests use fake HTTP/native credential processes, an in-memory MQTT broker connection, and **synthetic** H2D/X2D telemetry fixtures. Displayed telemetry has been validated on H2D and X2D, and the native credential-store probe has passed in a normal terminal. Remaining hardware and real-account restart checks are tracked in [the validation record](validation.md). Captured regression fixtures must have tokens, account identifiers, serials, job names, and spool identifiers removed.

Account ownership regressions in `internal/accountstore` use independent handles and pipe-synchronized child processes to cover contention, normal/crash release, stale jobs, and active-write shutdown. Cloud-host tests exercise decision/snooze lifecycle handoffs and pre-terminal contention rejection. Run `go test -race ./internal/accountstore ./cmd/bambu-tui` for these checks. They use synthetic accounts and temporary private storage, without live account/hardware validation.

Notification identity regressions use synthetic farm reports and temporary private stores. They cover reconnect/restart replay, family-specific clears and recurrence, job baselines/identities, failed and unknown attempts, cross-process locking semantics, corruption/capacity failures, and reservation write failures. These do not establish live automatic notification delivery.

`internal/session/session_test.go` exercises telemetry, occurrence identity, local browser decisions, immutable publishing, save coalescing/retry, failed-load protection, and snooze expiry without Bubble Tea. Run `go test -race ./internal/session ./internal/ui ./internal/webserver` for the foundation and its existing adapters; the PTY suite still verifies real CLI lifecycle. `cmd/bambu-tui/serve_test.go` and `serve_cloud_test.go` exercise terminal-free demo/fake-cloud startup, snapshots/local decisions and restart persistence, occupied ports, signals, typed authentication rejection, transient reconnects, malformed/over-limit report bursts followed by valid partial telemetry on the same HTTP session (including preserved freshness, condition targets, local decisions, and bounded private diagnostics), failed-load protection, snooze cleanup, unexpected termination, and bounded persistence drain with injected stores. Credential tests cover cancellable native lookup. Run `make build` then `./bin/bambu-tui serve --demo --config ./config.toml` for a foreground browser demo. For real accounts, save a login first and use `./bin/bambu-tui serve --config ./config.toml`; synthetic tests do not establish live account access. Keep adapters on one serial loop and return immutable-job completions to that loop.

See [architecture and conventions](architecture.md) for state ownership, concurrency, persistence, and code conventions. Protocol references are maintained in [protocol sources](../reference/protocol-sources.md).
