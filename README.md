# Bambu TUI

![Bambu TUI — your print farm, one terminal.](assets/branding/hero.png)

[![CI](https://github.com/cliffom/bambu-tui/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/cliffom/bambu-tui/actions/workflows/ci.yml)
[![Formatting and lint](https://github.com/cliffom/bambu-tui/actions/workflows/lint.yml/badge.svg?branch=main)](https://github.com/cliffom/bambu-tui/actions/workflows/lint.yml)
[![Latest release](https://img.shields.io/github/v/release/cliffom/bambu-tui)](https://github.com/cliffom/bambu-tui/releases/latest)

Monitor your Bambu Lab print farm from the terminal: printer status, print progress, time remaining, temperatures, filament, and faults in one dashboard.

Built for H2D and X2D printers connected through **Bambu Cloud**, with displayed telemetry validated on both models. Bambu TUI is read-only: it monitors your printers and does not control them. You can try a simulated farm without an account or printer.

## Get started

### 1. Install

Download an archive and the checksums file from the [latest release](https://github.com/cliffom/bambu-tui/releases/latest). Choose the archive for your machine:

| Machine | Archive suffix |
| --- | --- |
| Mac with Apple silicon | `darwin_arm64.tar.gz` |
| Mac with Intel processor | `darwin_amd64.tar.gz` |
| Linux with x86-64 processor | `linux_amd64.tar.gz` |
| Linux with ARM64 processor | `linux_arm64.tar.gz` |

For example, on an Apple silicon Mac with v0.0.10 downloaded:

```sh
shasum -a 256 bambu-tui_v0.0.10_darwin_arm64.tar.gz
# Compare the output with the matching line in bambu-tui_v0.0.10_checksums.txt.
tar -xzf bambu-tui_v0.0.10_darwin_arm64.tar.gz
./bambu-tui --version
```

Run these commands from the download directory, using your archive's filename. You can keep the executable there or move it to a directory on your `PATH`.

### 2. Try the demo

```sh
./bambu-tui --demo
```

The demo includes two printers, multiple AMS units, and an external spool. Press `m` to browse filament. An action-needed alert demonstrates a simulated runout about 15 seconds after launch. Close dialogs with `Esc`, then press `q` to quit.

### 3. Connect your printers

Use the email for the Bambu account that already has your printers bound to it:

```sh
./bambu-tui login --email-code
./bambu-tui
```

Enter the emailed code when prompted. This also works for **Google-linked accounts** using the email bound to the Bambu account; it uses Bambu's email verification rather than opening Google sign-in. For password login, use `./bambu-tui login`.

Login saves your session in macOS Keychain or Linux Secret Service. Linux requires `secret-tool` (usually in `libsecret-tools`) and a running Secret Service session. If saving is unavailable, login opens a dashboard for the current session instead. See [login troubleshooting](docs/usage.md#login-and-saved-sessions).

All printers on the account appear automatically. If you use Bambu's China region, [set `region = "china"`](docs/usage.md#configuration) before logging in.

## Using the dashboard

The running version appears in the dashboard and dialog headings. Use a terminal of at least **60 columns × 18 rows**; a wider window shows printer details and notices side by side. Press `?` for help.

| Key | Action |
| --- | --- |
| `↑` / `↓` or `j` / `k` | Select a printer or notice; scroll details |
| `Tab` / `Shift+Tab` | Switch panels |
| `D` | Preview and confirm a local diagnostic export (source builds) |
| `F` / `S` | Cycle printer filters / sorting |
| `m` | Browse the selected printer's AMS units and filament |
| `Enter` | Inspect a selected notice |
| `f` | Inspect the selected printer's first active fault |
| `a` / `c` | Acknowledge / dismiss a notice locally |
| `s` | View alert preferences; snooze or resume the selected printer |
| `n` | Manage local notice decisions: restore, mark unreviewed, or reset |
| `!` | Reopen unreviewed action-needed alerts |
| `r` | Refresh printer status and firmware information |
| `Esc` | Close a dialog |
| `q` / `Ctrl+C` | Quit from the dashboard after closing dialogs |

The farm summary counts printing, paused/failed, idle/finished, attention-needed, and unavailable printers. Use `F` to find actionable, printing, or unavailable printers; `S` sorts by name, attention, state, or finish time. Selection follows the printer during updates. See the [farm view guide](docs/farm-view.md) for definitions and ETA handling.

In the **AMS/filament browser**, use `Tab` or left/right to switch units and up/down to select a slot. Each material has a color chip; selected-slot details show its color, remaining estimate, and nozzle assignment. Press `m` or `Esc` to return to the dashboard.

**Action-needed alerts** open for supported faults requiring intervention, failed prints, and some stopped-printer conditions. Acknowledging or dismissing a notice affects only the TUI; it does not clear the printer's fault or resume a print. These decisions survive restarts. Press `n` to inspect and reverse them, or reset all decisions for the current account and region after confirmation. Recent event history lasts only for the current session.

Press `s` to snooze a printer for 15 minutes, 1 hour, or 4 hours, or resume alerts early. Global and per-printer [alert preferences](docs/alerts.md) control automatic interruptions while keeping printer state and notices visible. Snoozes survive restarts until their deadline.

Filament quantities are printer estimates; **Unknown**, **Empty**, and **0%** mean different things. External spool quantities are unavailable. Firmware status reflects the printer's reported assessment and may be unknown or out of date. The [user guide](docs/usage.md) explains these fields, all keyboard controls, and notice behavior.

For troubleshooting, source builds also provide a local diagnostic export via `D`. The preview shows what is included and where the file will go; Enter writes it locally. Unknown/private fields and free-form messages are omitted, and nothing is uploaded. See [diagnostic exports](docs/diagnostics.md).

## Configuration

Defaults work for global accounts. To change the region or low-filament threshold, create a file at the path printed by:

```sh
./bambu-tui --config-path
```

```toml
region = "global" # or "china"
low_filament_percent = 10
```

You can also use `--config /path/to/config.toml`. Configuration contains no credentials. See the [configuration reference](docs/usage.md#configuration) for details.

## Build from source

Requires **Go 1.26 or newer**, Git, and Make.

```sh
git clone https://github.com/cliffom/bambu-tui.git
cd bambu-tui
make demo   # build and try the simulated farm
make auth   # build and log in with an email code
make run    # build and open your farm dashboard
```

The executable is built at `bin/bambu-tui`. See the [development guide](docs/development.md) for tests, formatting, linting, and release tooling.

## More information

- [User guide](docs/usage.md): login, keyboard controls, saved notices, and telemetry details
- [Diagnostic export](docs/diagnostics.md): local export scope, inspection, and synthetic fixtures
- [v0.1.0 readiness](docs/v0.1.0-readiness.md): feature coverage and remaining operational checks
- [Farm view](docs/farm-view.md): summary definitions, filters, sorting, and selection
- [Alert preferences and snoozing](docs/alerts.md): per-printer interruption controls
- [Roadmap](ROADMAP.md): upcoming work and tracked issues
- [Changelog](CHANGELOG.md): release history
- [Validation record](docs/validation.md): tested hardware and outstanding checks
- [Protocol sources](docs/protocol-sources.md): Bambu protocol references and fault coverage
- [Report a bug or request a feature](https://github.com/cliffom/bambu-tui/issues)

Licensed under [MIT](LICENSE). Release archives include dependency licenses.
