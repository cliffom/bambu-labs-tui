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

Press `,` in the dashboard to edit general settings and the nine boolean alert defaults/overrides. Quiet-hour schedules and burst windows use [TOML configuration](alerts.md#quiet-hours-and-burst-grouping). `Tab` / `Shift+Tab` switches General, Alerts, and Printers; arrows select rows. In General, use `Enter` or `←` / `→` to switch Cloud region. Region edits are saved for the next launch: **restart and log in for the selected region**. The current cloud connection, credential lookup, notice store, snoozes, and notification ledger keep their original region until exit.

For Low filament threshold, use `←` / `→` to adjust by one percentage point, or `Enter` to type a whole number from 0 to 100. Typing replaces the initial value; `Ctrl+U` clears input, `Backspace` edits it, `Enter` validates/applies, and `Esc` cancels the input before closing Config. Invalid input stays visible with a red error and does not change settings. Threshold edits immediately re-evaluate reported remaining estimates; they do not establish physical fault resolution or create intervention alerts from unknown estimates.

See [Config controls](alerts.md#change-notifications-in-the-tui) for all nine alert defaults, per-printer On/Off/Inherit overrides, reset confirmation, Test Notification, and save failures.

### Custom themes

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

`--theme` accepts `default`, `gruvbox-dark`, `hotdog`, `c64`, `ibm3270`, or a custom JSON file path. For example, `bambu-tui --theme ./my-theme.json` or `make run THEME=./my-theme.json`. CLI paths resolve from the working directory. The launch override takes precedence over `theme_file`, including when that configured file is missing, and is never saved by interactive edits. Use `--theme default` to temporarily bypass a configured theme. `make run` and `make demo` also accept `RUN_ARGS='--theme gruvbox-dark'`; when both are set, `THEME` wins. The option is supported for monitoring, demo, and login, and is rejected for doctor/logout.

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

`version` and the `colors` object are required; `name` is optional descriptive metadata. Keys use exact snake_case names; the complete examples list every role. Colors accept `#RRGGBB` strings. Only `background` also accepts `null`, which inherits the terminal background as the default theme does. Unknown/duplicate keys, invalid colors, unsupported versions, unreadable files, and files larger than 64 KiB stop startup with an error; there is no silent fallback. JSON is the supported format. Themes are selected through `--theme` or `theme_file`; there is no interactive picker or live reload.

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
| `Ctrl+E` | Inspect full status/errors; `Esc` or `Ctrl+E` returns to the same screen |
| `Esc` | Close help and focus the printer list |
| `q` / `Ctrl+C` | Quit from the dashboard; close any modal first |

Use a terminal of at least 60 columns × 18 rows. At 105 columns the details and notices panels sit side by side; narrower terminals stack them. Focus each panel and scroll to see its full content. Status labels remain readable without color.

The footer follows the focused panel. Fresh informational messages, including delivery ownership and refresh requests, use bright bold text. In the default theme, confirmed local actions use green; warnings use amber and errors use red, with amber dashboard warnings for persistent storage/delivery problems. Custom themes retain these semantic roles with their chosen colors. Labels remain meaningful without color, and ownership does not confirm notification delivery or printer fault resolution.

Long feedback shortens before essential controls; multiple failures show an error count. `^E status` means `Ctrl+E`: open **Status / Feedback** for complete messages and recovery guidance, using arrows/page keys or Home/End to scroll. It preserves the current dialog, input, selection, confirmation, and scroll while telemetry and saves continue. Ordinary transient feedback lasts eight seconds; storage, export, browser, and delivery failures remain until their source reports recovery. This view does not acknowledge notices or confirm physical fault resolution.

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

When quitting from the dashboard, the app waits up to five seconds for pending notice and snooze saves. If a save fails or takes longer, it shows a warning: press `q` / `Ctrl+C` to exit anyway, or `Esc` to continue using the app. `Esc` while waiting cancels quitting; background saves continue. Failed saves stay visible and retry only after a new change. External cancellation can interrupt saving; if changes were not confirmed saved, a warning is printed after the terminal is restored. Abrupt process termination can lose pending changes.

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
