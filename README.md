# Bambu TUI

A terminal dashboard for a Bambu Lab print farm. Built for monitoring an **H2D and X2D through Bambu Cloud**, with a simulated farm you can try immediately. Source: [cliffom/bambu-labs-tui](https://github.com/cliffom/bambu-labs-tui).

## Install v0.0.1

Download the archive for your OS and CPU from [Releases](https://github.com/cliffom/bambu-labs-tui/releases), along with its checksums file. Verify the archive before extracting it:

```sh
# macOS example; use the matching Linux archive on Linux.
shasum -a 256 -c bambu-tui_v0.0.1_checksums.txt --ignore-missing
tar -xzf bambu-tui_v0.0.1_darwin_arm64.tar.gz
./bambu-tui --version
./bambu-tui --demo
```

If your checksum utility does not support `--ignore-missing`, compare `shasum -a 256 <archive>` with the matching line in the checksums file. Archives contain the executable, configuration example, documentation, and licenses. The executable remains named `bambu-tui`.

To build from source, install Go 1.26 or newer and run:

```sh
make build
./bin/bambu-tui --demo
```

The dashboard shows printer state, progress, remaining print time, estimated finish time, layers, temperatures for both nozzles, AMS slots, filament estimates, and active notices. Recent events stay in memory for the current session.

## Connect your printers

Use a Bambu account that already has your printers bound to it:

```sh
./bin/bambu-tui login
./bin/bambu-tui
```

Login supports password authentication, email verification, and authenticator challenges. You can also request an email login code or paste an existing access token through a masked prompt:

```sh
./bin/bambu-tui login --email-code
./bin/bambu-tui login --token
```

For accounts normally accessed with Google, use `login --email-code` with the email bound to the Bambu account. This is Bambu email verification; direct Google OAuth is not implemented.

Account tokens are saved in macOS Keychain or the Linux Secret Service through `secret-tool` (usually provided by `libsecret-tools`). Passwords are never saved. Secret values are passed to credential-store commands through standard input, not command arguments. When the credential store is unavailable, login opens a session-only dashboard. To choose that behavior explicitly:

```sh
./bin/bambu-tui login --session
./bin/bambu-tui logout
```

If saving fails, run `./bin/bambu-tui doctor`. It tests save, reload, and deletion using a disposable credential, without reading or sending Bambu credentials. On macOS, unlock your login keychain and try from a normal terminal if a sandboxed terminal cannot authorize access. Linux needs `secret-tool` and a running Secret Service session. The app verifies writes before reporting success and keeps unavailable stores distinct from missing credentials. Session-only login remains available when the host cannot provide a credential store.

`logout` removes this application's saved session for the configured region; it does not revoke the token with Bambu. Authentication failures stop reconnect attempts and show a notice directing you to log in again. Cloudflare blocks and account API errors are reported without printing response bodies or tokens. If direct login is blocked, token import may help, but account discovery must still be accessible.

## Configuration

Print the default configuration path:

```sh
./bin/bambu-tui --config-path
```

Create a `config.toml` there, or pass `--config /path/to/config.toml`. Configuration is optional; defaults are:

```toml
region = "global" # or "china"
low_filament_percent = 10
```

The configuration contains no credentials. Each region uses a separate saved session. All printers discovered on the account appear in the dashboard.

## Keyboard

| Key | Action |
| --- | --- |
| `↑` / `↓`, `j` / `k` | Select a printer/notice, or scroll printer details |
| `Tab` / `Shift+Tab` | Switch between printer list, details, and notices |
| `PgUp` / `PgDown` | Scroll details or page through notices |
| `g` / `G` | Select the first/last printer or notice |
| `Enter` | View the selected notice; `Esc` or `Enter` returns |
| `f` | Open the selected printer's first active fault |
| `a` | Toggle local acknowledgement of the selected active notice |
| `c` | Dismiss an active notice from the TUI, or delete a historical event |
| `C` | Clear recent event history after confirmation |
| `o` | Open the HMS troubleshooting page from notice details |
| `r` | Request a full status snapshot |
| `?` | Toggle help |
| `Esc` | Close help and focus the printer list |
| `q` / `Ctrl+C` | Quit |

Use a terminal of at least 60 columns × 18 rows. At 105 columns the details and notices panels sit side by side; narrower terminals stack them. Focus each panel and scroll to see its full content. Status labels remain readable without color.

To review notices, press `Tab` until the notices panel is focused, use `j/k` or the arrow keys to select an entry, and press `Enter`. The detail view shows the full message, status, severity, first-seen/event time, and fault code. HMS entries include a Bambu troubleshooting link; `o` opens it in your browser. While viewing a printer, `f` jumps straight to its first active fault.

`a` marks a notice as reviewed with `[ACK]` while keeping it visible. `c` dismisses an active notice from the TUI, removes its matching history entry, and excludes it from notice counts and the printer's fault list. Routine telemetry updates do not bring it back. If the condition resolves and recurs, it appears again; a changed connection problem or print failure also needs review again. Dismissal and acknowledgement last for this session.

For historical events, `c` deletes the selected event; `C` clears all recent events after confirmation. Clearing history leaves active conditions intact. Local dismissal preserves printer telemetry and does not send printer-side fault-clear commands; resolve or dismiss those through the printer or Bambu's own applications.

## Interpreting the data

- **Unknown filament** means no valid estimate was reported. **Empty** means the slot reports no spool. **0%** is a reported estimate, not an inference that a slot is empty. External spool quantities are unavailable; placeholder values are deliberately ignored.
- Remaining filament is the printer's estimate, not a measured inventory. Low-filament notices use the configured threshold for any AMS slot with a valid estimate. The application cannot determine whether that quantity will finish the current job.
- **R / L** beside a filament slot indicates its association with the right/left nozzle. The temperature panel separately marks the active nozzle.
- **Stale** means no printer status has arrived for more than 120 seconds. Device availability is refreshed every 60 seconds. Cloud disconnection is separate from a printer's reported offline status. `~` beside time estimates marks data that is stale, offline, or disconnected.
- ETA uses the timestamp of the last remaining-time update. Temperature-only updates do not move ETA. Paused prints have no finish ETA, and elapsed time never implies completion.
- Known printer errors and maintenance notices have readable descriptions. HMS severity distinguishes errors, warnings, and informational notices. A known empty-message internal code is suppressed; other unmapped HMS notices retain their codes and troubleshooting links. [Protocol sources](docs/protocol-sources.md) document the curated coverage and fallbacks.
- Standard AMS and AMS HT units have separate readable labels; internal IDs are preserved for filament-path assignments.
- Notices disappear when their condition resolves or you dismiss them locally; the event panel retains up to 500 events until you quit.

Monitoring requests only: the MQTT client can subscribe to reports and request full status. It exposes no pause, resume, cancel, heating, motion, or job-submission operations. This version has no database, background service, camera feed, desktop alerts, or LAN connection.

## Development

Requires Go 1.26 or newer. Bubble Tea v1, Bubbles, and Lip Gloss are pinned in `go.mod`.

```sh
make test    # tests with the race detector
make check   # go vet
make fmt     # format Go source
make fmt-check # check formatting without changing files
make lint    # golangci-lint
make lint-actions # actionlint for GitHub Actions workflows
make build   # bin/bambu-tui
make cross   # macOS/Linux, arm64/amd64 binaries in dist/
make release # versioned archives, SHA-256 checksums, and dependency licenses
```

`VERSION` is the release version. Make injects version, commit, and UTC build date into `--version`; a source build without Git metadata reports `unknown` for its commit. CI runs race tests and vet on Linux/macOS, checks native credential persistence on macOS, and packages release candidates on Linux. A pushed version tag creates a **draft** release for review; it does not publish automatically.

The separate **Formatting and lint** workflow checks Go formatting, error handling, vet diagnostics, ineffective assignments, static analysis, unused code, and GitHub Actions syntax on pull requests and pushes to `main`. It reports formatting problems without rewriting source. Run `make fmt` locally to fix formatting.

Install the versions in `.golangci-lint-version` and `.actionlint-version` to reproduce the lint checks locally. Get golangci-lint from its [official releases](https://github.com/golangci/golangci-lint/releases), and install actionlint with `go install "github.com/rhysd/actionlint/cmd/actionlint@$(cat .actionlint-version)"`. `make lint` and `make lint-actions` use tools on `PATH`; override `GOLANGCI_LINT` or `ACTIONLINT` when installed elsewhere. Workflow lint also uses ShellCheck when it is available.

Packages separate cloud authentication/discovery, the MQTT transport, the monitor worker, normalized farm state, credential storage, configuration, and the terminal UI. The UI reducer alone mutates farm state. Transports communicate through messages and stop on context cancellation.

Tests use fake HTTP/native credential processes, an in-memory MQTT broker connection, and **synthetic** H2D/X2D telemetry fixtures. A user-provided screenshot confirms live email-code login and H2D telemetry, and the native credential-store probe has passed in a normal terminal. Remaining hardware and real-account restart checks are tracked in [the validation record](docs/validation.md). Captured regression fixtures must have tokens, account identifiers, serials, job names, and spool identifiers removed.

Bambu's cloud protocol is community documented and may change. Implementation references:

- [OpenBambuAPI: cloud HTTP](https://github.com/Doridian/OpenBambuAPI/blob/main/cloud-http.md) and [MQTT](https://github.com/Doridian/OpenBambuAPI/blob/main/mqtt.md)
- [Home Assistant Bambu integration](https://github.com/greghesp/ha-bambulab/tree/main/custom_components/bambu_lab/pybambu), including dual-nozzle telemetry and login challenge handling
- [Bubble Tea](https://github.com/charmbracelet/bubbletea/tree/v1.3.10)

Licensed under [MIT](LICENSE). Packaged dependencies retain their own licenses under `third_party/` in release archives.
