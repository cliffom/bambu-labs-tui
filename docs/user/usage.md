# User guide

[Back to the README](../../README.md)

## Login and saved sessions

Use a Bambu account that already has your printers bound to it:

```sh
./bambu-tui login
./bambu-tui
```

Login supports password authentication, email verification, and authenticator challenges. You can also request an email login code or paste an existing access token through a masked prompt:

```sh
./bambu-tui login --email-code
./bambu-tui login --token
```

For accounts normally accessed with Google, use `login --email-code` with the email bound to the Bambu account. This is Bambu email verification; direct Google OAuth is not implemented.

The commands above use the executable from a release archive. From a source checkout, `make auth` builds the app and starts email-code login; `make run` builds and opens the dashboard. Use `make demo` to try the simulated farm without login. Override login options with `make auth AUTH_ARGS='--token'` (or `AUTH_ARGS=''` for password login). Pass dashboard options with `make run RUN_ARGS='--config /path/to/config.toml'`.

Account tokens are saved in macOS Keychain or the Linux Secret Service through `secret-tool` (usually provided by `libsecret-tools`). Passwords are never saved. Secret values are passed to credential-store commands through standard input, not command arguments. When the credential store is unavailable, login opens a session-only dashboard. To choose that behavior explicitly:

```sh
./bambu-tui login --session
./bambu-tui logout
```

If saving fails, run `./bambu-tui doctor`. It tests save, reload, and deletion using a disposable credential, without reading or sending Bambu credentials. On macOS, unlock your login keychain and try from a normal terminal if a sandboxed terminal cannot authorize access. Linux needs `secret-tool` and a running Secret Service session. The app verifies writes before reporting success and keeps unavailable stores distinct from missing credentials. Session-only login remains available when the host cannot provide a credential store.

For an independent synthetic desktop notification test on macOS, run `./bambu-tui doctor --notify`, optionally adding `--sound`. This skips the credential check and requires no login. See [notification testing](alerts.md#desktop-notification-test-and-sound) for permissions, focused/unfocused terminal checks, and delivery limitations.

`logout` removes this application's saved session for the configured region; it does not revoke the token with Bambu. Authentication failures stop reconnect attempts and show a notice directing you to log in again. Cloudflare blocks and account API errors are reported without printing response bodies or tokens. If direct login is blocked, token import may help, but account discovery must still be accessible.

## Configuration

Print the default configuration path:

```sh
./bambu-tui --config-path
```

Create a `config.toml` there, or pass `--config /path/to/config.toml`. Configuration is optional; defaults are:

```toml
region = "global" # or "china"
low_filament_percent = 10
```

The configuration contains no credentials. Each region uses a separate saved session. All printers discovered on the account appear in the dashboard.

Press `,` in the dashboard to edit general settings and the nine boolean alert defaults/overrides. Quiet-hour schedules and burst windows use [TOML configuration](alerts.md#quiet-hours-and-burst-grouping). `Tab` / `Shift+Tab` switches General, Alerts, Printers, and Web Server; arrows select rows. In General, use `Enter` or `←` / `→` to switch Cloud region. Region edits are saved for the next launch: **restart and log in for the selected region**. The current cloud connection, credential lookup, notice store, snoozes, and notification ledger keep their original region until exit.

For Low filament threshold, use `←` / `→` to adjust by one percentage point, or `Enter` to type a whole number from 0 to 100. Typing replaces the initial value; `Ctrl+U` clears input, `Backspace` edits it, `Enter` validates/applies, and `Esc` cancels the input before closing Config. Invalid input stays visible with a red error and does not change settings. Threshold edits immediately re-evaluate reported remaining estimates; they do not establish physical fault resolution or create intervention alerts from unknown estimates.

See [Config controls](alerts.md#change-notifications-in-the-tui) for all nine alert defaults, per-printer On/Off/Inherit overrides, reset confirmation, Test Notification, and save failures.

### Local web dashboard

The browser status bar stays at the bottom of the window, separated by a border. Scroll the farm workspace above it to reach all printer cards and the result count; the connection/demo status, monitoring reminder, and GitHub repository link remain visible. The bar wraps to accommodate narrow windows and larger browser text. When a short window cannot fit the header, workspace, and wrapped bar together, the page also scrolls to keep the farm and footer accessible.

Press `,`, then use `Tab` / `Shift+Tab` to select **Web Server**. Select **Enable web server** and use `Enter`, Space, or `←` / `→` to toggle it. It is off by default. The default port is **8080**; a successful start shows `http://127.0.0.1:8080` (or your configured port). Open that exact URL on the same computer.

Select **Port** to configure it: `←` / `→` adjusts by one, or `Enter` lets you type a whole number from **1 to 65535**. Typing replaces the current value; `Ctrl+U` clears, `Enter` applies, and `Esc` cancels the edit. Invalid values stay in the editor with an error. Applying a new port while enabled stops the previous listener and starts on the new port immediately. While disabled, it only saves the next startup port. Port 0 and automatic port selection are not supported.

If the port is already in use, the server **does not start** and never falls back to a different port. Web Server shows Failed and the occupied-port error; Status / Feedback (`Ctrl+E`) retains full details. The enabled preference records the requested state; only Running and a URL confirm a successful bind. Choose another port, or free the occupied port and press `r` to retry explicitly. Unrelated input and ticks do not retry a failed start. The same failure behavior applies when changing a running server to an occupied port: the old listener closes and the server remains stopped.

Closing Settings leaves a successfully started server running. Disabling it closes the listener and active browser connections; exiting the TUI also stops it, including on signal cancellation.

The enabled preference and port save asynchronously in the active configuration, with the same save-failure/retry/shutdown behavior as other settings:

```toml
[web_server]
enabled = false
port = 8080
```

The dashboard shows the whole-farm summary, searchable/filterable printer cards, print state/progress, remaining-time estimates, temperatures, AMS/external filament, and reported conditions with intervention guidance where available. It polls every three seconds while visible, and best-effort while hidden when browser notifications are enabled. Multiple browser tabs share the TUI's monitoring connection and one encoded snapshot; they do not create cloud connections. Browser filters/sorting are session-local and do not change TUI preferences. Every browser sort groups online non-idle printers first, reported idle/finished printers next, and offline devices last, using the same [grouping and unknown/last-known distinctions](farm-view.md#sorting) as the TUI. Demo uses the same dashboard with clearly labeled synthetic data (`make demo`).

Use the single printer toolbar to choose **All printers**, **Printing**, **Idle / finished**, **Attention**, or **Unavailable**. Every filter tab shows its matching printer count before search is applied; attention and availability can overlap reported print states. Summary totals describe the whole farm and are not filter buttons. **Idle / finished** includes only currently available online printers explicitly reporting `IDLE` or `FINISH`, with a count before search is applied. Search narrows the selected filter; sorting applies within it. The count below the cards reads `3 printers` when all are visible or `2 of 3 printers` for a subset; Help explains how whole-farm state, attention, and availability counts overlap. Missing/stale telemetry, offline/cloud-disconnected devices, and unknown/active/paused/failed states are excluded from this view. A delayed or disconnected browser feed also removes these current-availability results until recovery. Existing Printing totals and filters still describe the last reported state, independently of availability; the groups can overlap Attention.

Idle and Finished stay distinct on the cards. Neither confirms that a build plate is clear, a fault is resolved, or a printer is ready for another job. Offline cards show the printer name, model, and image with an explicit **Offline** label, a dashed border, and dimmed grayscale artwork. Job, progress, time, temperature, and material readings are omitted from these cards; click or keyboard-open the card to inspect last-known details and notices. Attention warnings remain visible. Only an explicit device-offline report selects this layout; stale data or a disconnected cloud/browser feed does not. Other cards with unavailable telemetry show one warning that their readings are last-known and missing values remain unknown. Material text wraps; full job names and telemetry remain available through inspection. Polling preserves focus and cards that remain visible; if the focused card leaves the active filter, focus returns to that filter button. Connection/demo mode stays on the left of the footer and the repository link on the right, without a duplicate sidebar.

Open **Settings** using the gear button beside Help in the dashboard header. Its **Browser notifications** section offers explicit Enable/Test/Disable controls and capability, permission, and submission status. Close with the close button or `Esc`; closing Settings leaves automatic notifications running. Browser permission and automatic opt-in are separate from native macOS delivery. New actionable incidents and confirmed print completions can notify while an enabled page runs; tabs/browsers share durable claims and reloads establish a baseline without backlog. See [browser notification policy, settings, and lifecycle limits](alerts.md#browser-system-notifications). Closing a notification never hides a notice.

In **Settings > Appearance**, **Follow host (configured theme)** uses the host's effective TUI palette, including custom `theme_file` colors and launch-only `--theme` overrides. The selector also offers every bundled TUI theme. A selection applies immediately and is remembered in this browser at the current dashboard origin; other tabs at that origin follow it. Choose Follow host to reset. It leaves the host configuration and TUI appearance unchanged. If browser storage is unavailable, selection still works for the current page with a warning. Unknown saved choices fall back to Follow host.

The browser uses the same semantic RGB colors for text, backgrounds, borders, selection/focus, and status feedback. A `null` terminal background has no browser equivalent, so it uses a dark `#101312` canvas; panels share that canvas. Light themes also switch native form controls to a light color scheme. Custom colors are used as supplied without contrast correction, so inspect their readability. Reported filament swatches and printer artwork retain their original colors. Theme files load at host startup; restart to reload file edits. Host palette updates reach following browsers within 30 seconds while visible, or on opening Settings/returning to the page. Theme-fetch failure preserves the applied appearance and does not interrupt telemetry.

![Browser Appearance settings using the bundled Hotdog theme](../../assets/screenshots/browser-themes.png)

Unknown fields remain unknown. Stale, offline, and disconnected readings are marked last-known; completion is never inferred from progress or an estimate. A stale remaining-time estimate remains last-known even if temperature reports are fresh. If the dashboard server becomes unreachable or its snapshot is delayed, the browser preserves last-known data with a warning; this does not establish printer connectivity or fault resolution. The summary retains the last server-reported counts during an interruption.

Printer cards use locally bundled pixel-style illustrations for H2D, X2D, X1 Carbon, P1S, and A1 mini, with a generic fallback for other or unknown models. These illustrations identify the reported model visually; they do not show installed accessories or live printer state. They load from localhost and need no external image service.

Open a printer and choose **Hide notice locally** beside a reported condition to dismiss that exact occurrence in this session's TUI and all connected browser tabs. Dismissed conditions are hidden from the browser inspector by default. Choose **View hidden messages** to inspect them, labeled hidden locally, and **Hide hidden messages** to collapse them again. The view keeps this choice as reports arrive and resets it when you open a printer. This display option does not restore or change saved decisions. A dismissed condition leaves TUI notices, action-needed lists, attention counts, and future alert eligibility. Acknowledgement, snooze, restoration, closing an inspector/system notification, and physical fault resolution are separate actions. Use the TUI's saved decisions view to restore a dismissal, and the printer or Bambu's applications for physical intervention.

The browser shows a pending request before the reducer applies it, then an applied local decision. **Saving** is not durable success; **saved** means the shared notice snapshot reached disk. **Session-only** applies in demo or when persistence is unavailable; unreadable saved data is preserved. **Failed** means the local change applied but may be lost on restart: fix storage, then use **View hidden messages** and choose **Retry saving local decisions** on a hidden condition. Retry saves the latest complete notice snapshot. A failed or timed-out request has an unknown outcome: refresh before trying again. Stale/replaced targets are rejected, and repeated hides of the same current occurrence are safe. Newly arriving conditions and confirmed clear-and-recurrence remain independent.

TUI changes reach visible browser tabs on their next three-second poll; hidden tabs pause polling and refresh on return. Browser decisions reach the TUI through its event loop without waiting for disk I/O. This synchronization is limited to one running session, with no independent process-to-process synchronization. Shutdown rejects further browser decisions and uses the existing pending-save/failure handling. The browser receives normalized, allowlisted display data, including printer/job names; credentials, raw payloads, protocol job IDs, raw device serials, account information, storage paths, and backend error text are excluded.

Access is **localhost only, without authentication**. Other users and programs on this computer can access the dashboard. Other local clients can also hide notices and retry saves. Exact Host/Origin checks, strict JSON POST requests, and a random session CSRF token protect browser writes from foreign sites; they do not authenticate local programs/users who can fetch the dashboard and token. LAN binding, reverse-proxy/remote access, and TLS remain outside scope. Real-printer browsing requires either the TUI or the foreground `serve` process to stay running. The web server is not a background service.

### TUI Server Status

Press **`W`** from the dashboard, or select **Server Status** in **Config (,) > Web Server** and press `Enter` or Space. `W` also opens it from that Config section when no edit or confirmation owns input. This read-only modal stays live while monitoring, HTTP clients, listener commands, and saves continue. Enablement, port changes, and retry remain in Config.

The modal shows cloud/demo mode; Stopped (disabled), Starting, Running, Stopping, or Failed HTTP state; monitoring connection/reconnect/authentication state; and the complete bound localhost URL only after a successful bind. **Session uptime** measures this TUI session; **Listener uptime** measures the current successful binding and resets after a listener restart. Connected monitoring does not establish fresh printer telemetry. Persistent Config, notice, and snooze storage warnings include recovery guidance. Recent diagnostics retain up to eight fixed, sanitized messages; they exclude raw backend errors and telemetry.

HTTP activity uses the same session-only ledger as the foreground view: up to 32 connect/disconnect events and 128 tracked sockets/groups, grouped best-effort by socket IP and sanitized first-request User-Agent. Sockets without requests are ignored. The last socket's closure starts a five-second disconnect grace; matching replacement sockets keep the group alive. Server shutdown retires groups immediately. Active counts cover tracked groups, and tracking omissions and expired history are labeled. A group is neither an authenticated user nor an exact browser-tab count. Client metadata never enters browser snapshots or persistent storage.

Use arrows, `j`/`k`, page keys, or Home/End (`g`/`G`) to scroll; long client metadata wraps. Updates preserve the scroll offset where content permits. `Esc` or `W` returns to the dashboard or Config, preserving its selection and input. `Ctrl+E` opens Status / Feedback and returns to this modal, preserving the suspended Config as well. Closing inspection leaves the server running. Quit remains dashboard-only; below 60 × 18, navigation is blocked and resizing back restores inspection.

### Foreground server status

On an interactive terminal, `serve` and `serve --demo` show a compact, continuously updated status screen. The localhost dashboard URL stays at the top, followed by cloud/demo mode, HTTP server state, monitoring connection/reconnect state, session uptime, up to three recent sanitized diagnostics, and HTTP client activity. Open the URL on the same computer. **Connected** describes the monitoring transport; use the browser dashboard for individual printer connectivity and telemetry freshness.

The screen uses the default palette: green for the URL and healthy states, amber for connecting/reconnecting and warnings, and red for authentication, server, and storage failures. Labels retain their meaning with `NO_COLOR=1` or without color support. It updates at most once per second and adapts to terminal resizing. Narrow terminals wrap the URL and keep `Ctrl+C to stop` visible, dropping lower-priority rows when needed. At extremely small sizes even the URL/guidance may not fit; enlarge the terminal to inspect the full status. The status screen retains its default palette; the browser uses the configured theme or `serve --theme` override. `Ctrl+C` or `SIGTERM` closes HTTP and performs the existing bounded shutdown; the prior terminal screen and cursor return on exit, followed by a stopped/failed summary.

**HTTP clients** groups connections by socket IP and the first request's sanitized user agent. A browser opening several sockets produces one Connected entry and one active client group; sockets that never send a request produce no entries. Entries include local time, Connected/Disconnected, a session client number, IP, and user agent. Groups are a best-effort display identity, not authenticated people or tabs: identical IP/user-agent pairs share one group, and changing a user agent can create another. The IP comes from the socket, ignoring forwarded headers; with localhost binding it will normally be `127.0.0.1`.

A group disconnects after all its sockets close and a five-second grace period passes. New matching activity during that period keeps the same client lifetime. Closing a tab may leave a connection idle until the server's 30-second idle timeout; the ledger therefore cannot report the exact time someone closes a browser tab. Listener shutdown disconnects all groups immediately.

The session keeps the latest 32 client events, with up to six recent entries visible when space permits. It tracks at most 128 sockets and 128 client groups, showing a count when details are omitted at these limits; the active count covers tracked groups. User agents have terminal escapes/control/non-ASCII characters removed and are limited to 160 characters; missing user agents are labeled not supplied, and narrow ledger rows truncate with an ellipsis. This is session-only display history, not a durable audit log, and is excluded from browser telemetry. Request paths, cookies, authorization headers, and request bodies are not logged.

Redirected output, missing terminal input, and `TERM=dumb` use plain output: the listening URL, startup status and exit guidance, then changed status/diagnostics and shutdown states. New client events print once in plain output. If events outpace the retained history, a count reports skipped entries. Uptime ticks do not create log lines. Monitor diagnostics use fixed application text and omit payloads, credentials, account/printer identifiers, paths, and backend errors. Malformed-report warnings remain limited to one per session on stderr; storage failures also remain visible in browser status. Binding/startup failures exit with ordinary error output before entering the status screen.

### Terminal-free cloud server

After saving a login, run the cloud monitor and dashboard in the foreground without a TTY:

```sh
./bambu-tui login --email-code --config ./config.toml
./bambu-tui serve --config ./config.toml
```

Use the same configuration/region for both commands. `serve` reads the existing saved session from macOS Keychain or Linux Secret Service; it never opens login dialogs or accepts `--session`, `--token`, or `--email-code`. A session-only login cannot be reused by `serve`. Missing or invalid credentials fail with login guidance. An unavailable/locked store fails with unlock/Secret Service and `doctor` guidance. Native-store lookup is cancellable and bounded to 15 seconds. A cloud-rejected or expired session stops the server with a nonzero exit: log in again and restart. Transient cloud failures keep reconnecting and the dashboard labels retained data last-known.

Malformed reports and reports exceeding telemetry limits are ignored while monitoring continues. The dashboard retains the last accepted readings, timestamps, and local decisions; rejection does not refresh data or confirm resolution/completion. The first rejection prints a fixed warning to stderr; further rejection warnings are suppressed for that server session to bound diagnostics. Payloads, identifiers, and backend errors are never included in that warning.

Open the localhost URL in the [foreground status display](#foreground-server-status) on the same computer. The command starts HTTP regardless of saved web enablement and preserves configuration, using the configured port without fallback. Both `serve` modes resolve `theme_file` beside the active configuration and accept `--theme` with the same launch-only precedence as the TUI; invalid themes fail before monitoring or HTTP startup. The URL confirms a listener, not successful cloud authentication or fresh telemetry. One monitor serves all browser tabs. Closing tabs leaves monitoring running; `Ctrl+C` or `SIGTERM` stops it. Unexpected monitor/listener termination also exits nonzero.

Browser notice decisions reuse the TUI's account/region-scoped files and survive restart. Snoozes are loaded and expired deadlines are saved; browser hiding remains separate from snoozing and physical fault resolution. Unreadable stores are preserved with writes disabled and session-only guidance; repair them and restart. Failed writes remain visible with explicit browser retry for notice decisions. Shutdown closes HTTP and drains accepted writes/coalesced revisions for up to five seconds, returning a failure for failed or unconfirmed saves; monitor cancellation/join is bounded separately to five seconds. One cloud host per account/region and storage directory is enforced before loading decisions; another host exits with stop-and-retry guidance. See [saved notice decisions](#saved-notice-decisions) for upgrade and external-edit limits.

Native desktop delivery stays disabled even if saved preferences enable it. Browser system notifications are independently opt-in; see [browser notification policy](alerts.md#browser-system-notifications). This is a localhost-only foreground process, without dashboard authentication, remote access, or an installed background service. Other local users/programs can read the dashboard and submit local notice decisions as described above.

### Terminal-free demo server

Run the synthetic farm in the foreground without a terminal UI, credentials, or hardware:

```sh
./bambu-tui serve --demo
./bambu-tui serve --demo --config ./config.toml
# From a source checkout:
make build
./bin/bambu-tui serve --demo --config ./config.toml
```

Open the printed listening URL on the same computer. This command starts HTTP even when `[web_server].enabled = false`, uses the configured port (default 8080), and never rewrites that preference or the configuration file. An occupied port exits with an error and no fallback; a listening URL is printed only after binding succeeds. Invalid configuration also stops startup. Terminal themes are unused; `--theme` is rejected.

Browser inspection and local notice hiding use the same dashboard and shared session. Demo decisions are **session-only** and reset on restart; real-account notice, snooze, credential, and notification stores are not opened. Native desktop delivery is disabled even if saved preferences enable it, and those preferences are preserved. Browser system notifications use temporary demo-only storage and explicit browser opt-in; see [browser notification policy](alerts.md#browser-system-notifications).

Stop with `Ctrl+C` or `SIGTERM`. Closing browser tabs leaves the foreground process running. Shutdown closes the listener and rejects new decisions, drains accepted persistence jobs for up to five seconds, and reports unconfirmed saves as a failure; demo has no durable decision writes. Unexpected HTTP termination also exits with an error. For real printers, use the [cloud server](#terminal-free-cloud-server) after saving a login. This mode does not install a service or enable remote access.

### Custom themes

Your own themes are ordinary JSON files loaded from disk at startup. **No Go changes, registration, or rebuild are needed.** Create a file using the partial example below, or copy one of the complete theme examples and edit its colors. Launch an installed or already-built binary with the file path:

```sh
bambu-tui --theme ./my-theme.json
# From a source checkout, reuse the binary you already built:
./bin/bambu-tui --theme ./my-theme.json
```

Edit the JSON and restart that same binary to apply changes. `make run THEME=./my-theme.json` also works, though `make run` builds the application before launching. The persistent `theme_file` setting below loads your file on each launch. Bundled names are convenient shipped examples; adding a custom file does not require adding it to the application bundle.

Select a bundled theme for one launch, with no setup files:

```sh
bambu-tui --theme gruvbox-dark
bambu-tui --demo --theme gruvbox-dark
```

From a source checkout:

```sh
make run THEME=gruvbox-dark
make demo THEME=gruvbox-dark
```

`--theme` accepts `default`, `gruvbox-dark`, `hotdog`, `c64`, `ibm3270`, `deskmate`, or a custom JSON file path. For example, `bambu-tui --theme ./my-theme.json` or `make run THEME=./my-theme.json`. CLI paths resolve from the working directory. The launch override takes precedence over `theme_file`, including when that configured file is missing, and is never saved by interactive edits. Use `--theme default` to temporarily bypass a configured theme. `make run` and `make demo` also accept `RUN_ARGS='--theme gruvbox-dark'`; when both are set, `THEME` wins. The option is supported for monitoring, demo, and login, and is rejected for doctor/logout.

For an unmistakable light-theme demonstration, try **Hotdog**, inspired by the red/yellow Hotdog Stand look:

```sh
bambu-tui --theme hotdog
make run THEME=hotdog
make demo THEME=hotdog
```

Hotdog uses a very light cream background, dark body text, vivid red headings/borders, a yellow selection, and a red status bar with bright yellow/white text. Its complete [hotdog.json](../../internal/ui/themes/hotdog.json) is also available as a starting point for customization.

For Commodore-inspired blues, use **C64**:

```sh
bambu-tui --theme c64
make run THEME=c64
make demo THEME=c64
```

C64 uses a deep blue body, light blue/lavender text and borders, blue selections, and a darker blue status bar. Its base/selection blues and green/yellow accents draw from [VICE's Pepto PAL palette](https://github.com/VICE-Team/svn-mirror/blob/main/vice/data/C64/pepto-pal.vpl); text and feedback colors are brightened for monitoring readability. The complete [c64.json](../../internal/ui/themes/c64.json) is available for customization.

For the green-screen terminal look, use **IBM 3270**:

```sh
bambu-tui --theme ibm3270
make run THEME=ibm3270
make demo THEME=ibm3270
```

IBM 3270 uses a near-black green body, phosphor-green text and borders, a reverse green selection, and a dark green status bar. App-owned colors stay in shades of green; text labels and emphasis distinguish information, success, warning, and errors. Reported filament swatches retain their actual colors. The name references [IBM's 3270 green-screen interfaces](https://www.ibm.com/docs/en/om-shared?topic=deployment-decision-10-which-user-interfaces-use). The complete [ibm3270.json](../../internal/ui/themes/ibm3270.json) is available for customization.

For the classic **DeskMate II** blue/gray text-mode look:

```sh
bambu-tui --theme deskmate
make run THEME=deskmate
make demo THEME=deskmate
```

DeskMate uses the four colors in the preserved [blue/gray text-mode screen capture](https://nerdlypleasures.blogspot.com/2024/07/tandy-deskmate-tandys-ace-in-hole.html): blue `#0000AA`, light gray `#AAAAAA`, dark gray `#555555`, and black `#000000`. It maps those colors to a blue body with gray text, gray selections with black text, and a gray status bar. Labels and emphasis retain message meaning in this limited-palette theme. Reported filament swatches retain their actual colors. The [original Tandy manual](https://www.manualslib.com/manual/1336444/Tandy-1000.html?page=34) describes user-selectable foreground/background and highlighted colors, so period color schemes varied. Its complete [deskmate.json](../../internal/ui/themes/deskmate.json) can be customized without rebuilding.

For a persistent custom file, set `theme_file` **before any TOML table** in the configuration file:

```toml
theme_file = "gruvbox-dark.json"
```

Relative paths resolve beside the active configuration file (`--config` selects it); absolute paths work too. Theme files load once at startup. Restart after editing or selecting a theme. Config and farm-view saves preserve this setting and unrelated external edits. Removing the setting restores the built-in default. Theme loading does not change your terminal settings.

Download [gruvbox-dark.json](../../internal/ui/themes/gruvbox-dark.json) beside your config to try the complete sample, or copy [default.json](../../internal/ui/themes/default.json) to customize the current palette. All bundled themes are embedded in the binary; selecting one by name needs no external theme file. The Gruvbox sample uses a dark body background, warm neutrals, aqua focus, and distinct feedback colors, drawing from the [requested palette](https://www.color-hex.com/color-palette/1026676) and [official Gruvbox colors](https://github.com/morhetz/gruvbox/blob/master/colors/gruvbox.vim). It uses brighter variants for prominent status text.

A small override file is sufficient; omitted colors retain the current defaults:

```json
{
  "version": 1,
  "name": "My theme",
  "colors": {
    "background": "#282828",
    "text": "#ebdbb2",
    "footer_info": "#fbf1c7",
    "footer_background": "#3c3836"
  }
}
```

`version` and the `colors` object are required; `name` is optional descriptive metadata. Keys use exact snake_case names; the complete examples list every role. Colors accept `#RRGGBB` strings. Only `background` also accepts `null`, which inherits the terminal background as the default theme does. Unknown/duplicate keys, invalid colors, unsupported versions, unreadable files, and files larger than 64 KiB stop startup with an error; there is no silent fallback. JSON is the supported format. Host themes are selected through `--theme` or `theme_file`; there is no interactive TUI picker or theme-file live reload. The browser has an independent selector in Settings > Appearance, as described above.

Use contrasting text/background pairs, particularly `text`/`background`, `selected_text`/`selected_background`, and all `footer_*` text roles against `footer_background`. Supplied colors are used as written; the loader validates syntax rather than measuring or correcting contrast. Limited-color terminals approximate RGB values, and `NO_COLOR=1` keeps readable labels and selection markers without colors. Reported filament swatches retain the printer's color instead of using theme accents. See [palette roles and contrast assumptions](../development/ui-direction.md#palette-and-message-emphasis).

## Keyboard and navigation

| Key | Action |
| --- | --- |
| `↑` / `↓`, `j` / `k` | Select a printer/notice, or scroll printer details |
| `Tab` / `Shift+Tab` | Switch between printer list, details, and notices |
| `PgUp` / `PgDown` | Scroll details or page through notices |
| `g` / `G` | Select the first/last printer or notice |
| `Enter` | View the selected notice; `Esc` or `Enter` returns |
| `f` | Open the selected printer's first active fault |
| `D` | Preview and confirm a local allowlisted diagnostic export |
| `F` / `S` | Cycle and save the printer filter / sort without changing panel focus |
| `0` | Reset and save All printers / Name sorting from the dashboard |
| `m` | Open the selected printer's AMS/filament browser |
| `s` | View effective printer alert settings and snooze/resume controls |
| `N` | Inspect notification activity, ownership, and paused-worker recovery |
| `,` | Edit general settings, global alerts, and printer overrides |
| `n` | Manage local notice decisions, including dismissals |
| `!` | Reopen unreviewed action-needed alerts |
| `a` | Toggle local acknowledgement of the selected active notice |
| `c` | Dismiss an active notice from the TUI, or delete a historical event |
| `C` | Clear recent event history after confirmation |
| `X` | Clear all notices across the farm after confirmation (Notices panel) |
| `o` | Open the HMS troubleshooting page from notice details |
| `r` | Request a full status snapshot |
| `?` | Open/close scrollable help; arrows/page keys scroll, Home/End jumps |
| `A` | Open read-only About: version, platform, active theme, and hue-sorted color grid |
| `W` | Open live Server Status from dashboard or Config > Web Server; `Esc` / `W` returns |
| `Ctrl+E` | Inspect full status/errors; `Esc` or `Ctrl+E` returns to the same screen |
| `Esc` | Close help and focus the printer list |
| `q` / `Ctrl+C` | Quit from the dashboard; close any modal first |

Use a terminal of at least 60 columns × 18 rows. At 105 columns the details and notices panels sit side by side; narrower terminals stack them. Focus each panel and scroll to see its full content. Status labels remain readable without color.

The footer follows the focused panel. Fresh informational messages, including delivery ownership and refresh requests, use bright bold text. In the default theme, confirmed local actions use green; warnings use amber and errors use red, with amber dashboard warnings for persistent storage/delivery problems. Custom themes retain these semantic roles with their chosen colors. Labels remain meaningful without color, and ownership does not confirm notification delivery or printer fault resolution.

Press `A` from the dashboard for **About Bambu TUI**. It shows the app name/version, OS/architecture, project URL, branding, and attribution without field labels, followed by the effective bundled theme or custom file path (including launch overrides). Theme field labels use the same secondary text styling as dashboard labels, while the theme source uses primary text. **Theme Palette** shows unique explicit RGB colors as an unlabeled swatch grid, sorted by hue; gray colors come first from dark to light. Colors shared by multiple roles appear once, and an inherited terminal background is omitted because its RGB is unknown. Without color, solid blocks preserve the grid shape but cannot identify the colors; see the [custom theme format](#custom-themes) for named roles and RGB values. About displays no account information. Use arrows/page keys or Home/End to scroll long theme paths; `Enter`, `Esc`, or `A` returns to the prior dashboard focus and selection. Telemetry continues while About is open.

Long feedback shortens before essential controls; multiple failures show an error count. `^E status` means `Ctrl+E`: open **Status / Feedback** for complete messages and recovery guidance, using arrows/page keys or Home/End to scroll. It preserves the current dialog, input, selection, confirmation, and scroll while telemetry and saves continue. Ordinary transient feedback lasts eight seconds; storage, export, browser, and delivery failures remain until their source reports recovery. Notice and snooze saves each show **saving**, **saved**, **failed**, or **session-only**. When an idle writable store has failed, `r retry saves` retries all eligible failed notice/snooze stores. This view does not acknowledge notices or confirm physical fault resolution.

When the terminal is smaller than 60 columns × 18 rows, the resize screen blocks hidden navigation, review/snooze actions, exports, and confirmations. `Esc` still closes the current dialog or returns to its parent; `q` / `Ctrl+C` still quit from the dashboard. Telemetry and asynchronous saves/results continue. Resize back to resume the same dialog and any pending confirmation. The compact persistence shutdown screen retains its exit/cancel controls.

### Diagnostic export

Press `D` on the dashboard to preview a local JSON export for the selected printer. Enter creates the file; Esc cancels. The export includes classified telemetry and structure, sanitized partial reports, app/platform/model/firmware metadata, connection state, and fault codes. Unknown/private fields and free-form values are omitted. No upload occurs. Inspect the file before attaching it voluntarily; see [diagnostic exports](diagnostics.md) for scope, limitations, and fixture creation.

### Farm view

The summary counts the whole farm. `F` cycles All, Attention, Printing, and Unavailable; `S` cycles Name, Attention, State, and Finish. Active settings and visible/total counts stay above the printer list. Selection follows printer identity through live sorting, and filtered-out/removed printers fall back to another visible row. Missing/stale ETAs sort after live estimates. These view settings are saved in the active configuration and restored on restart. Press `0` to reset All / Name, including from an empty filtered view. They do not alter notices, review decisions, or alert eligibility. See the [farm view guide](farm-view.md) for exact definitions and navigation behavior.

### AMS and filament

The dashboard keeps printer details compact: the filament summary shows inventory counts, low/unknown AMS estimates, and the active nozzle path. Press `m` for the full **AMS / Filament** browser. `Tab` / `Shift+Tab` or left/right switches standard AMS units, AMS HT units, and external spools; up/down or `j/k` selects a slot, `Home/End` selects the first/last slot, `Enter` brings its details into view, and `PgUp/PgDn` scrolls. `m` or `Esc` closes the browser and preserves dashboard focus/scroll. As with other modals, `q` and `Ctrl+C` do not quit until you close it.

The browser stays on the same printer, unit, and slot as reports arrive, and falls back safely if an inventory item disappears. Live telemetry continues; action-needed alerts queue until the browser closes. A reported 0% remains distinct from an empty slot, missing estimates stay unknown, and external spool quantity remains unavailable. The compact low-estimate count reflects reported AMS quantities even if the corresponding notice was locally dismissed. Try `make demo` and press `m` to browse two standard AMS units, an AMS HT, and an external spool without logging in.

### Notices

To review notices, press `Tab` until the notices panel is focused, use `j/k` or the arrow keys to select an entry, and press `Enter`. The detail view shows the full message, status, severity, first-seen/event time, and fault code. HMS entries include a Bambu troubleshooting link; `o` opens it in your browser. While viewing a printer, `f` jumps straight to its first active fault.

`a` marks a notice as reviewed with `[ACK]` while keeping it visible. `c` dismisses an active notice from the TUI, removes its matching history entry, and excludes it from notice counts and the printer's fault list. Routine telemetry updates do not bring it back. If the condition resolves and recurs, it appears again; a changed connection problem or print failure also needs review again.

### Saved notice decisions

Cloud monitoring supports **one TUI or foreground `serve` process per account and region** in the same OS configuration directory. Starting another exits before monitoring or opening a dashboard, with “another cloud TUI or serve process owns saved notices and snoozes” and instructions to stop it and retry. Different accounts/regions and synthetic demos remain independent; a different `--config` file does not bypass account ownership.

Ownership covers both notice decisions and snoozes, including automatic resolution and cleanup, from before loading until accepted saves finish. Exit or crash releases the OS lock; restart loads the latest committed snapshots. A crash can still lose pending saves. Private empty lock files remain under `bambu-tui/sessions/`; do not delete or replace them while monitoring is running. Lock access failures also stop startup, with configuration-directory access guidance.

Configuration, notice, snooze, and notification-ledger saves sync the replacement file and then its containing directory and ancestors before reporting success. This includes newly created paths. The guarantee depends on the filesystem and storage honoring these sync operations; unsupported directory syncs fail the save. A failure after replacement can leave the new data visible while durability remains unconfirmed. Notice/snooze saves stay failed until a successful retry, and notification reservation failures pause delivery without sending or returning the affected notification. A visible reservation remains consumed on recovery. These checks do not establish physical power-loss recovery on every filesystem or device.

Stop older instances before upgrading: all cloud processes sharing this storage must use v0.3.6 or newer. Older binaries do not honor this lock. Running sessions do not reload external file edits or synchronize decisions, preferences, prompt eligibility, or delivery eligibility across processes. Use browser tabs attached to the same running host for a shared view; manually editing/restoring stored decisions requires stopping the host first. Local review decisions still never establish physical resolution.

Press `n` from any dashboard panel to open **Notice decisions**. It includes dismissed and acknowledged active notices, showing the printer, description, code, status, visibility, and review state. Use up/down or `j/k` to select an entry, `Home/End` (or `g/G`) to jump to the first/last entry, and `PgUp/PgDn` to scroll its details. Press `n` or `Esc` to close. Close the view before quitting.

- `r` restores a dismissal. The notice becomes visible, and its independent acknowledgement stays unchanged. Deleted history is not recreated.
- `u` explicitly marks an acknowledged notice unreviewed. This leaves its visibility or dismissal unchanged. To make a dismissed, acknowledged notice both visible and unreviewed, use both actions.
- `R` opens a confirmation to reset all acknowledgements and dismissals for the **current account and region**, including decisions awaiting telemetry. Press `y` to reset or `n` / `Esc` to cancel. Reset preserves printer telemetry and session event history. Demo mode changes only demo decisions.

Saved decisions awaiting startup reports are labelled **Awaiting telemetry**; saved low-filament decisions show their AMS unit and slot, while other descriptions may be unavailable until the printer reports. Unsupported notice types retain their saved identity. Editing them does not create active notices. When telemetry confirms resolution, the saved decision disappears; a later recurrence is a new, unreviewed occurrence.

An individual restore or mark-unreviewed action clears session-only prompt suppression for that occurrence. A restored notice that remains acknowledged does not prompt. A visible, unreviewed actionable notice can prompt after you close this view and the normal batching delay passes. Reset clears session suppression for all occurrences. Alerts continue to queue while this view or its reset confirmation is open.

Changes save immediately through the normal notice store. Storage failures remain visible in both screens. If the saved file could not be loaded, writes remain disabled to preserve it, and reset affects only this session; the warning remains visible.

Acknowledgements and dismissals are queued for saving as soon as they change and restored when the TUI reopens. Disk writes run in the background so slow storage does not block input or telemetry. Files are separate for each account and region, under the OS configuration directory's `bambu-tui/notices/` folder (`~/Library/Application Support/bambu-tui/notices/` on macOS; `$XDG_CONFIG_HOME/bambu-tui/notices/` or `~/.config/bambu-tui/notices/` on Linux). They contain local notice decisions and printer identifiers, with private file permissions; account tokens stay in the OS credential store. A custom `--config` path does not change this account-scoped storage location. Press `n` to manage these decisions in the TUI; manual file deletion is not needed.

Saved decisions survive discovery and partial startup telemetry until the relevant status arrives. Confirmed resolution removes the saved decision. If a condition resolves and recurs entirely while the TUI is closed, the app cannot identify that as a new occurrence without a printer-provided identifier; the matching decision is retained until resolution is observed. Recent event history remains session-only, and `--demo` neither reads nor writes saved notice decisions. If storage fails, a persistent warning reports that decisions may apply only to the current session; an unreadable or unsupported file is preserved without overwriting it.

For historical events, `c` deletes the selected event; `C` clears the recent events captured when its confirmation opens. Events arriving while the confirmation is open are kept. Clearing history leaves active conditions intact. In the Notices panel, `X` opens **Clear all notices**: confirm with `y` to dismiss visible active notices locally and remove recent session events across the entire account farm, regardless of printer selection or filter. The confirmation shows counts captured when it opens; new events and new or recurring notice occurrences are preserved while telemetry continues. `n` or `Esc` cancels. When clearing leaves the panel empty, focus returns to Printers so arrow keys and `j/k` immediately navigate the printer list. Pressing `X` on an already empty panel shows a message and returns focus to Printers as well. If newer notices remain, focus stays in Notices. Bulk dismissals follow the same persistence, restore, and resolution/recurrence rules as individual dismissals. Local dismissal preserves printer telemetry and does not send printer-side fault-clear commands; resolve or dismiss those through the printer or Bambu's own applications.

When quitting from the dashboard, the app waits up to five seconds for pending notice and snooze saves. If a save fails or takes longer, it shows a warning: press `q` / `Ctrl+C` to exit anyway, or `Esc` to continue using the app. `Esc` while waiting cancels quitting; background saves continue. Failed saves stay visible. After correcting a transient storage problem, cancel shutdown with `Esc`, open Status / Feedback (`Ctrl+E`), and press `r` to retry eligible failed notice/snooze saves without changing your decisions. A new local change also queues another save. External cancellation can interrupt saving; if changes were not confirmed saved, a warning is printed after the terminal is restored. Abrupt process termination can lose pending changes.

A retry saves the latest complete snapshot. Repeated retry keys while queued or saving are ignored; edits made during a write coalesce into a following snapshot. Retrying does not change acknowledgements, dismissals, snooze deadlines, occurrence identities, or prompt suppression, and does not resend desktop notifications. Failed loads keep unreadable/unsupported stores write-protected for the session; fix storage and restart to load them again. Demo remains session-only. Config saves retain their own `r` retry in Config.

### Alert preferences and snoozing

Press `s` on the dashboard or an action-needed prompt to snooze its printer for 15 minutes, 1 hour, or 4 hours; `u` in those controls ends a snooze early. Deadlines save across restarts, independently of acknowledgements and dismissals. All underlying notices stay visible. Global and per-printer category/master switches are editable in Config (`,`), or under `[alerts]` in TOML; defaults preserve automatic alerts. `!` manually inspects unreviewed actionable notices even while muted or snoozed. See the [alert guide](alerts.md) for configuration, expiry, removed-printer behavior, and storage errors. Opt-in macOS desktop delivery and sound share category preferences and snoozes; see the alert guide for setup.

### Action-needed alerts

An **Action needed** modal automatically opens for verified runout, feed/retract failures, clogs, and other supported intervention-required faults, or a failed print. Unknown faults accompanying a stopped printer get a generic review prompt. An ambiguous pause is reviewed once both HMS and print-error snapshots have arrived; its cause is not guessed. Maintenance, low estimates, explicit user pauses, and reported automatic recovery do not independently interrupt you.

Reports are collected for two seconds and grouped by printer. `Tab` / `Shift+Tab` switches printers; arrows or page keys scroll. `Enter` opens a notice, `p` selects printer details, and `o` opens a displayed HMS troubleshooting link. `a` acknowledges only that printer's displayed notices; `c` dismisses them locally. `Esc` closes the entire queued prompt without acknowledging anything, restoring your previous dashboard focus. Those occurrences do not automatically prompt again during this session; `!` reopens eligible unreviewed alerts.

Each independent notice emphasizes its fault and next intervention, with its first app-observed timestamp (local timezone/UTC offset) and a live relative age. This is observation time, not a printer-side start time or desktop delivery time. Codes and review status remain visible; full troubleshooting URLs are available in the notice inspector.

Existing inspectors, confirmations, help, and the AMS/filament browser finish before an alert opens. Telemetry continues, and a resolved condition is marked in the open modal. Saved acknowledgements/dismissals suppress corresponding startup alerts. A different fault or an observed resolved-and-recurring occurrence remains eligible. Distinct codes arriving after a dialog was closed can prompt again because telemetry does not always identify which reports belong to one incident. Underlying notices remain independent. See [alert behavior](action-needed.md) for details.

To preview the modal without a printer, run `make demo`: the X2D simulates runout about 15 seconds after launch and recovers about nine seconds later. Demo decisions are never persisted.

## Interpreting the data

- **Unknown filament** means no valid estimate was reported. **Empty** means the slot reports no spool. **0%** is a reported estimate, not an inference that a slot is empty. External spool quantities are unavailable; placeholder values are deliberately ignored.
- Remaining filament is the printer's estimate, not a measured inventory. Low-filament notices use the configured threshold for any AMS slot with a valid estimate. The application cannot determine whether that quantity will finish the current job.
- The **Nozzle** column shows a slot's reported right/left nozzle assignment. The temperature panel separately marks the active nozzle.
- **Stale** means no print status has arrived for more than ten minutes while the last reported state is idle or finished, or two minutes for any other state. A new printing report immediately restores the two-minute threshold. Connection labels, unavailable filtering, and stale notices use this same policy; missing initial telemetry still shows Waiting for data. Device availability is refreshed every 60 seconds, and offline/cloud-disconnected status takes effect independently of the grace period. `~` beside time estimates marks data that is stale, offline, or disconnected.
- Reports that exceed telemetry size or complexity limits are ignored in full. Last accepted data and local notice decisions remain in place; rejected reports do not refresh timestamps or confirm that a fault resolved.
- ETA uses the timestamp of the last remaining-time update. Temperature-only updates do not move ETA. Paused prints have no finish ETA, and elapsed time never implies completion.
- Known printer errors and maintenance notices have readable descriptions, including verified standard AMS/AMS HT and external-spool runout, empty-slot reports, feed/retract failures, possible tangles, and nozzle clogs. Runout recovery stages remain distinct from faults requiring replacement filament. HMS severity follows the reported code, and printer-supplied messages take precedence. A known empty-message internal code is suppressed; other unmapped HMS notices retain their codes and troubleshooting links. [Protocol sources](../reference/protocol-sources.md#filament-fault-catalogue-expansion) document exact coverage and remaining gaps.
- A paused-state notice remains separate from the fault that may have caused it. HMS and print-error reports also retain their individual codes; acknowledgement or dismissal applies to each notice independently, preserving visibility of distinct faults.
- Standard AMS and AMS HT units have separate readable labels; internal IDs are preserved for filament-path assignments.
- Notices disappear when their condition resolves or you dismiss them locally; the event panel retains up to 500 events until you quit.

Monitoring requests only: the MQTT client can subscribe to reports and request full status. It exposes no pause, resume, cancel, heating, motion, or job-submission operations. This version has no database, background service, camera feed, LAN connection. Opt-in desktop alerts require a running local macOS TUI session and the bundled helper.

### Firmware

Printer details also show the installed printer **Firmware** and **FW Status**. Status is the printer's reported assessment: **Current (printer reported)**, **Update available** with the offered OTA version, or **Unknown (not confirmed)**. Component-only updates do not imply a printer firmware update. Old assessments or disconnected/stale printers show **Last known**; press `r` to request fresh status and firmware metadata. This is not a live comparison against a public release catalogue, and no firmware installation is performed.

## Notification history maintenance

Use `compact-notifications --consume-unfinished` only after stopping account hosts. It preserves deduplication history and frees live job capacity, while consuming unfinished completion tracking in both native and browser channels. See [retention, command examples, consequences, and backup policy](alerts.md#notification-history-and-capacity) before running it. Ordinary activity recheck does not perform maintenance or retry consumed delivery attempts.
