# BambuStand

Read-only Bambu Cloud monitoring for your terminal. This guide is included as `README.md` in binary release archives.

## Install and run

Download the archive for your machine and its checksum manifest from [GitHub Releases](https://github.com/cliffom/bambustand/releases). From v0.2.1, Mac downloads use `macos_arm64` (Apple silicon) or `macos_amd64` (Intel); Linux downloads use `linux_arm64` or `linux_amd64`. Earlier Mac downloads use `darwin` in place of `macos`.

Compare `shasum -a 256 <archive.tar.gz>` (or `sha256sum` on Linux) with the matching manifest entry, then extract with `tar -xzf <archive.tar.gz>`. From the extracted directory:

```sh
./bambustand --version
./bambustand --demo
./bambustand
```

The demo needs no account or printer. On the first interactive launch without a saved session, enter the email bound to your Bambu account and the emailed code. The app saves the validated session and opens the dashboard; later launches reuse it. If saving fails, it explains the failure and continues for this session. Use `./bambustand login --email-code` to save a login separately. Press `?` for dashboard controls and `q` to quit.

For a browser demo without a terminal UI, run `./bambustand serve --demo` (optionally `--config ./config.toml`) and open the printed localhost URL. Stop with `Ctrl+C`. It uses synthetic data and session-only notice decisions; native notifications are disabled. For real printers, save a login and run `./bambustand serve` or keep the TUI running. See [demo server guidance](https://github.com/cliffom/bambustand/blob/main/docs/user/usage.md#terminal-free-demo-server).

On macOS, keep `BambuStand Notify.app` beside `bambustand`, including when moving the executable to a directory on your `PATH`. Signed releases include a Developer ID-signed, notarized CLI and helper; no Go, Swift, or Apple Developer account is needed to run them. Linux archives contain the CLI without the helper.

## Configure notifications

Notifications are off by default and require a live TUI session on a local Mac. Press `,` to open Config, switch to **Alerts** with `Tab`, and select **Test Notification**, and allow notification permission. Enable **Desktop notifications** and optionally sound. Check macOS Focus and notification preview settings if a banner is hidden or generic. Quitting the TUI ends monitoring and delivery.

The included `config.example.toml` documents available settings. It is not loaded automatically from the current directory. To use an edited copy explicitly:

```sh
cp config.example.toml config.toml
# Edit config.toml before running.
./bambustand --config ./config.toml
```

For Bambu's China region, set `region = "china"` before logging in. See the [user guide](https://github.com/cliffom/bambustand/blob/main/docs/user/usage.md) for default config locations and login troubleshooting, [alert documentation](https://github.com/cliffom/bambustand/blob/main/docs/user/alerts.md) for notification settings, and the [documentation index](https://github.com/cliffom/bambustand/blob/main/docs/README.md) for full help.

Licensed under MIT (see the included `LICENSE`); Go and dependency notices are in `THIRD_PARTY_NOTICES.txt`. This project is not affiliated with Bambu Lab.

Upgrading from Bambu TUI: replace CLI invocations with `bambustand` and keep the new helper beside it. Existing configuration, saved logins, local decisions, and notification history remain in their original locations. See [upgrade compatibility](https://github.com/cliffom/bambustand/blob/main/docs/user/usage.md#upgrading-from-bambu-tui).
