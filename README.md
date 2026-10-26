# Bambu TUI

![Bambu TUI — your print farm, one terminal.](assets/branding/hero.png)

[![CI](https://github.com/cliffom/bambu-tui/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/cliffom/bambu-tui/actions/workflows/ci.yml)
[![Formatting and lint](https://github.com/cliffom/bambu-tui/actions/workflows/lint.yml/badge.svg?branch=main)](https://github.com/cliffom/bambu-tui/actions/workflows/lint.yml)
[![Latest release](https://img.shields.io/github/v/release/cliffom/bambu-tui)](https://github.com/cliffom/bambu-tui/releases/latest)

Monitor your Bambu Lab print farm from the terminal: printer status, print progress, time remaining, temperatures, filament, and faults in one dashboard.

Bambu TUI connects through **Bambu Cloud**, with displayed telemetry validated on **H2D and X2D**. It is a read-only monitor: printer control and LAN connections are not supported. Try a simulated farm without an account or printer.

![Bambu TUI farm dashboard showing three printers, selected printer details, filament inventory, notices, and recent events.](assets/screenshots/farm.png)

## Get started

You need macOS or Linux and a terminal of at least **60 columns × 18 rows**. To monitor real printers, use a Bambu account with your printers already bound to it. Saved login sessions use macOS Keychain or Linux Secret Service; Linux needs `secret-tool` (usually in `libsecret-tools`) and a running Secret Service session.

### 1. Install

Download your archive and the checksums file from the [latest release](https://github.com/cliffom/bambu-tui/releases/latest):

| Machine | Archive suffix |
| --- | --- |
| Mac with Apple silicon | `darwin_arm64.tar.gz` |
| Mac with Intel processor | `darwin_amd64.tar.gz` |
| Linux with x86-64 processor | `linux_amd64.tar.gz` |
| Linux with ARM64 processor | `linux_arm64.tar.gz` |

From your download directory, set the release version below to the one you downloaded. This example uses the Apple silicon archive; change the suffix for your machine.

```sh
release_version=vX.Y.Z # Replace with the downloaded release's version.
archive="bambu-tui_${release_version}_darwin_arm64.tar.gz"
shasum -a 256 "$archive"
# Compare with the matching line in bambu-tui_${release_version}_checksums.txt.
tar -xzf "$archive"
./bambu-tui --version
```

On Linux, you can use `sha256sum` instead of `shasum -a 256`. Keep the executable here or move it to a directory on your `PATH`. For macOS notification tests, keep the bundled `Bambu TUI Notify.app` beside it; see [notification setup and limitations](docs/user/alerts.md#desktop-notification-test-and-sound).

### 2. Try the demo

```sh
./bambu-tui --demo
```

Browse a simulated farm with AMS units and an external spool. Press `m` to inspect filament and `?` for help. A simulated runout demonstrates an action-needed alert about 15 seconds after launch. Close dialogs with `Esc`, then press `q` to quit.

### 3. Connect your printers

```sh
./bambu-tui login --email-code
./bambu-tui
```

Enter your Bambu account email and the emailed code when prompted. This also works for **Google-linked accounts** using the email bound to Bambu. For password login, use `./bambu-tui login`.

All printers on the account appear automatically. If saving the login session is unavailable, login opens a dashboard for the current session instead; see [login troubleshooting](docs/user/usage.md#login-and-saved-sessions). For Bambu's China region, [set `region = "china"`](docs/user/usage.md#configuration) before logging in.

## Using the dashboard

Use arrows or `j` / `k` to select a printer, `Tab` to switch panels, and `?` for all controls. These shortcuts cover the main tasks:

| Key | Action |
| --- | --- |
| `F` / `S` | Cycle printer filters / sorting |
| `m` | Browse AMS units and filament |
| `Enter` / `f` | Inspect a selected notice / the printer's first active fault |
| `a` / `c` | Acknowledge / dismiss a notice locally |
| `n` | Manage saved notice decisions |
| `s` | View alert preferences and snooze/resume controls |
| `D` | Preview and confirm a local diagnostic export |
| `Esc` | Close a dialog |
| `q` / `Ctrl+C` | Quit after closing dialogs |

Action-needed alerts highlight faults requiring intervention. Acknowledging or dismissing a notice changes only the TUI; it does not clear a printer fault or resume a print. Filament quantities are estimates, and firmware information reflects what the printer reports. See the [user guide](docs/user/usage.md) for controls, configuration, saved sessions, and telemetry details.

## Build from source

Requires **Go 1.27.3 or newer**, Git, and Make.

```sh
git clone https://github.com/cliffom/bambu-tui.git
cd bambu-tui
make demo   # build and try the simulated farm
make auth   # build and log in with an email code
make run    # build and open your farm dashboard
```

The executable is built at `bin/bambu-tui`. See [contributing](docs/development/contributing.md) for development setup, tests, formatting, and linting.

## More information

- [Documentation index](docs/README.md): user guides, development, and references
- [Troubleshooting and diagnostic exports](docs/user/diagnostics.md)
- [Hardware validation and known gaps](docs/development/validation.md)
- [Roadmap](docs/ROADMAP.md) and [changelog](docs/CHANGELOG.md)
- [Report a bug or request a feature](https://github.com/cliffom/bambu-tui/issues)

Licensed under [MIT](LICENSE). Release archives include dependency licenses. This project is not affiliated with Bambu Lab.
