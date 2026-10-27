# Alert preferences and snoozing

[Back to the README](../../README.md)

Bambu TUI separates **what the printer reports** from **when automatic modals or opt-in desktop notifications interrupt you**. Preferences and snoozes change only interruptions. Printer state, faults, notice counts, and event history remain visible. No printer controls are sent.

## Change notifications in the TUI

Press `,` from any dashboard panel to open **Config**. `Tab` / `Shift+Tab` changes between General, Alerts, and Printers. Use `↑` / `↓` (or `j` / `k`) to select a row; `Home` / `End` and `PgUp` / `PgDown` jump within the section. The selected row scrolls into view. `Enter` / `Space` activates a control; `←` / `→` (or `h` / `l`) changes values. `Esc` / `,` closes Config; close it before quitting. The footer explains the selected control and appends test feedback after a dot. Navigation and close keys appear below the header.

| Section | Controls |
| --- | --- |
| General | Cloud region and low-filament warning percentage |
| Alerts | All nine global alert defaults, followed by Test Notification |
| Printers | Printer selector, all nine overrides with effective values, and Reset to global defaults |

In Printers, select the printer row and use `←` / `→` to choose a discovered printer or a saved absent printer. Each preference cycles **Inherit → On → Off → Inherit**; the effective value is shown beside it. An explicit On overrides a global Off. **Reset this printer to global defaults** asks for `Enter` confirmation; `Esc` cancels. Reset removes only that printer's overrides and preserves its notice reviews and snooze. The target stays selected through discovery and name changes. Use `s` from the dashboard for effective settings, stable printer ID, and snooze controls.

Global and per-printer alert changes apply immediately. Enabling delivery or a category allows currently eligible, unreviewed, unsnoozed incidents; already attempted occurrences remain deduplicated. Co-reported interventions still group per printer. Eligible automatic modals wait until Config closes. Editing preferences never acknowledges or resolves a fault, resets reviews, or ends a snooze. See [General settings](usage.md#configuration) for the percentage editor and region restart requirement.

Config shows **Config file location:** with the active `--config` path or default OS path. A missing file is created, including in demo mode. Background saves reload valid TOML and merge fields edited since the last successful snapshot; untouched external settings and overrides survive. If both editors change the same field, the Config edit wins. Writes use a private atomic replacement, normalize formatting, and remove comments. Avoid simultaneous file editing when possible. Malformed or unknown-key files cause a failure without replacement.

Successful saves are silent. Failed saves remain red with `r` retry guidance in the body; alert/threshold changes remain active in the session, while region changes still require restart. Navigation and ticks do not retry failed writes. Normal quit waits for pending saves and requires an explicit exit choice after failure or timeout. Changing settings does not request macOS permission. In **Alerts**, select **Test Notification** to request permission and test delivery, or run `bambu-tui doctor --notify`.

Select **Test Notification** and press `Space` / `Enter` to send one synthetic notification through the same branded helper as doctor. It uses the current global Notification sound setting and works even when Desktop notifications is off. This action does not change preferences, consume incident/job identities, or affect live delivery counts. Only one Config test runs at a time; the status bar shows bright informational Sending text, green acceptance, or red failure after the selected-option helper text. Accepted results appear in bright green, dim after four seconds, and disappear after seven seconds. On narrow screens, ordinary helper text shortens to keep the result readable. While typing a percentage or confirming a reset, the footer keeps apply/reset/cancel instructions; `Ctrl+E` shows the retained test feedback without canceling the input or test. Test failures remain red until another test. Acceptance does not confirm a visible banner or audible sound: check macOS Focus and notification settings if needed. On first use, allow macOS permission when prompted. Closing Config cancels a pending test; already accepted notifications may still appear. A failed test can be repeated explicitly; doctor provides detailed failure guidance.

## Inspect notification activity

Press `N` from the dashboard to open **Notification activity** for the entire TUI session. Use `↑` / `↓` or `j` / `k` to select a decision, `PgUp` / `PgDn` to page, `Home` / `g` to follow newest decisions, and `End` / `G` for the oldest retained decision. `r` appears only for an idle paused or standby worker: it rechecks paused delivery after its cause is fixed, or checks standby ownership again. It is unavailable while work or recovery is pending. `N` / `Esc` closes the view. New decisions keep the selected row stable while browsing; if it expires from the retained history, selection moves to the oldest remaining row. Printer names are captured when decisions occur, so entries remain understandable after renaming or removal.

The view retains the last **100 decision records in memory**, including decision times, printer, fault code or completion subject, macOS desktop channel, outcome, and reason. It starts empty on each launch. Repeated matching decisions update the retained row's last-observed time and count rather than adding rows; a change of reason creates a new record. Co-reported incidents retain separate rows with the same request number and group size. History eviction does not alter saved deduplication identities or cause another notification.

| Outcome | Meaning |
| --- | --- |
| Accepted | macOS accepted the request; banner visibility and sound remain unconfirmed |
| Failed | The sender returned a failure, such as denied permission or an unavailable helper |
| Unconfirmed | Delivery was interrupted or timed out; whether macOS accepted it is unknown |
| Suppressed | Delivery was withheld because of reviews, preferences, snoozing, deduplication, completion baselines/missing identity, or changed/unavailable telemetry |
| Paused | Identity storage, capacity, or queue failure paused delivery; monitoring continues |
| Status | Notification ownership changed or recovery was requested |

The selected activity row uses the theme's selection text/background colors for its outcome as well as its other fields. Unselected outcomes retain their semantic colors; outcome labels always remain explicit. Each selected row explains the applicable reason and recovery guidance. The ownership line identifies this TUI as the delivery owner, a standby behind another TUI, or a muted observer. Fresh ownership/standby/observer/recovery messages use prominent informational text; they describe coordination, not a printer fault or a delivered notification. An already-reserved attempt may come from an earlier session; its outcome remains uncertain. Failed and unconfirmed attempts have **no automatic retry**. Corrupt/busy storage and capacity exhaustion are distinguished; restarting does not free exhausted capacity. Activity cannot diagnose whether Focus hid an accepted banner. Quiet-hours suppression and optional grouping across reports are also shown; see [Quiet hours and burst grouping](#quiet-hours-and-burst-grouping).

Accepted/Failed counters count OS submission outcomes, while Suppressed counts newly recorded per-incident decision transitions, not telemetry reports. Counters cover the entire session and may exceed the retained 100 rows. History contains no credentials, raw telemetry, backend stderr, or raw protocol job IDs, and is never written to disk. Config tests and doctor are separate explicit probes, outside this live activity history. Before desktop opt-in the worker remains idle; the empty view explains disabled or unsupported delivery rather than creating identity files to populate a log.

## Notification ownership and recovery

When several TUIs monitor the same Bambu account and region, the first opted-in session to acquire the private ownership lock delivers automatic notifications. Other sessions show **Another TUI owns desktop delivery** and continue monitoring, local notice decisions, configuration, and snoozing normally. Each running TUI keeps its own live preferences; the owner's preferences govern delivery until ownership changes. Config does not broadcast preference changes to other running TUIs.

A standby checks for ownership again about once per second through new UI snapshots. It takes over after the owner exits or crashes, without replaying handled incidents or startup completions. Cached standby frames never initiate takeover delivery. Turning off all desktop preferences in the owner releases its delivery ownership; once-started muted sessions can still observe identity lifetimes when no active owner holds the lock, releasing it after each observation. A new owner rechecks current telemetry, preferences, reviews, and snoozes. A canceled native request must finish before its ownership lock is released. All concurrently running TUIs should use v0.2.5 or newer for this ownership policy; older versions do not participate in the session lock.

Press `r` in Notification activity after fixing a paused worker's cause. Recovery discards old queued frames, waits for fresh UI state, validates the existing ledger, and attempts ownership again. It preserves the session activity history and counters. This resumes delivery for eligible unattempted occurrences; it **does not resend accepted, failed, or reserved/unconfirmed attempts**, request permission, clear reviews/snoozes, or reset identities. Config's Test Notification and doctor remain separate repeatable probes.

Unreadable/unsupported state remains untouched on every recheck. Fix file access or repair/restore valid state before retrying; losing prior attempts through an older backup or deleting the ledger can permit repeat alerts. Capacity remains bounded at 4,096 identities / 2 MiB and fails closed. A retry may apply newly confirmed condition clears, but it never forgets retained job identities to make space. A full job history still requires a future retention/recovery policy; repeatedly pressing `r` does not bypass that protection. Ownership uses an empty private `.owner.lock` file beside the account/region ledger; the OS releases its lock after process exit. Do not delete or replace lock files while sessions are running.

## Configure automatic alerts

Add these settings to the file printed by `bambu-tui --config-path`, or your custom `--config` file. Restart the TUI after editing. Existing configurations retain these defaults:

```toml
[alerts]
automatic_modals = true
desktop_notifications = false # opt-in, local macOS desktop only
notification_sound = false
completed_prints = true
filament = true
extrusion = true
printer_faults = true
failed_prints = true
unknown_pauses = true

# Example: disable automatic interruptions for a maintenance printer.
# Replace the example ID with the Printer ID shown by s in the dashboard.
[alerts.printers."YOUR_PRINTER_ID"]
automatic_modals = false
desktop_notifications = false

# Another printer can override just one category, inheriting all other settings.
[alerts.printers."ANOTHER_PRINTER_ID"]
filament = false
```

Each per-printer field overrides the corresponding global field independently. An explicit `true` can override a global `false`. Printer IDs are stable identifiers, so renaming a printer does not change its preferences. Overrides for absent printers remain in your configuration and apply if they return. The configuration contains no credentials. Booleans are required; unknown keys, unsupported options such as `sound`, and empty printer IDs are rejected.

| Setting | Automatic interruptions it permits |
| --- | --- |
| `automatic_modals` | Master switch for that printer's automatic action-needed modals |
| `desktop_notifications` | Independent opt-in master switch for macOS desktop delivery |
| `notification_sound` | Request sound on desktop notifications; defaults off |
| `completed_prints` | Telemetry-confirmed completion notifications when desktop delivery is on |
| `filament` | Verified runout and filament feed/retract/path failures |
| `extrusion` | Verified nozzle/extrusion faults requiring intervention |
| `printer_faults` | Other verified intervention faults and unknown non-info faults on a stopped printer |
| `failed_prints` | The independently reported failed-print state |
| `unknown_pauses` | An ambiguous pause after both HMS and print-error snapshots arrive, without a verified intervention, recovery, or manual-pause reason |

Preferences filter the existing verified actionable catalogue; enabling a category does not turn maintenance, low-filament estimates, explicit user pauses, or automatic recovery into automatic alerts. Classification uses verified fault metadata, never printer message wording. Related paused-state notices follow the enabled categories of their unreviewed faults, so muting a runout does not produce a duplicate generic pause prompt. Multiple fault categories can contribute to that state; any enabled contributing category permits it. A separately reported failed print has its own switch.

The firmware/telemetry view and notice panels are never filtered by these settings. The printer details show alert status; press `s` to see the effective switches and stable printer ID. The overview counts snoozed printers and printers with all automatic modal interruptions disabled. Desktop delivery has its own master switch and does not require automatic modals to be enabled; category switches are shared.

## Quiet hours and burst grouping

These global policies are configured in TOML and require restart after file edits. Config continues to edit the nine boolean alert defaults and printer overrides; its background saves preserve externally edited schedules/windows.

```toml
[alerts]
burst_window_seconds = 3 # 0 (default) disables; otherwise 1..60 seconds

[alerts.quiet_hours]
enabled = true # false by default
timezone = "America/Los_Angeles"
start = "22:00"
end = "07:00"
days = "mon,tue,wed,thu,fri"
```

Use an explicit IANA timezone (`UTC` is also supported), `HH:MM` times, and comma-separated lowercase weekday abbreviations. Start is inclusive and end is exclusive; equal times are rejected. An overnight period belongs to its **starting weekday**: Friday 22:00 through Saturday 07:00 is controlled by `fri`. For a same-day period, only the selected day applies. Daylight-saving changes follow local wall-clock minutes: skipped minutes never occur, and repeated minutes have the same quiet policy on both passes. The bundled timezone database supports hosts without zoneinfo.

Quiet hours suppress only automatic macOS desktop banners and their sound. Printer state, notices, review decisions, and automatic modal preferences remain independent. There is **no urgent exception**, regardless of severity; every otherwise eligible automatic desktop event is quieted. Config Test Notification and doctor remain explicit tests and bypass this policy.

An otherwise eligible occurrence observed during quiet hours is durably consumed, without marking its notice acknowledged, dismissed, or physically resolved. There is **no summary after quiet hours and no backlog replay**, including after restart. A confirmed clear followed by a new recurrence has a new identity and can notify normally. Events already excluded by categories, reviews, or snoozes retain those policies rather than being consumed by quiet hours. Observation requires the running TUI and fresh telemetry; events while monitoring is stopped cannot be reconstructed.

With a nonzero burst window, the first eligible event opens a fixed window; subsequent reports do not extend it. Still-current interventions and completions across printers share one generic count notification. Each member retains its own printer, job/incident identity, reservation, result, and activity row with a shared request number. Sound is requested if any included member permits it. Grouping never reviews or resolves notices. Policy, review, connection, and identity changes are rechecked before reservation and submission; obsolete members are dropped. A change affecting an in-flight member can cancel the whole submission conservatively.

All automatic delivery, including when the burst window is zero, is limited to **12 submission attempts per rolling minute per running owner**. Failed and uncertain submissions count and have no automatic retry. Excess unattempted events wait for a rate slot and are rechecked against fresh UI snapshots; resolved/changed events are not replayed. The session rate window resets on restart or owner takeover; persistent identity deduplication remains. Waiting windows/rate slots are checked about once per second through the reducer and never send from a timer's cached telemetry. Shutdown cancels waiting delivery without installing a background trigger.

Quiet suppression adds a saved `quiet_suppressed` attempt state. Older binaries that cannot read it fail closed; use this version or newer after enabling quiet hours. Do not delete the ledger to downgrade, because that loses deduplication history.

## Snooze one printer

Press `s` from a dashboard panel to open the selected printer's **Printer alerts / Snooze** view. From an **Action needed** prompt, `s` opens controls for the printer currently shown there.

| Key | Action |
| --- | --- |
| `1` | Snooze all automatic categories for 15 minutes |
| `2` | Snooze for 1 hour |
| `3` | Snooze for 4 hours |
| `u` | End the snooze early and re-arm eligible alerts |
| `↑` / `↓`, `PgUp` / `PgDn` | Scroll the settings and explanation |
| `s` / `Esc` | Close without changing the snooze |

A preset replaces the previous deadline with a new interval starting now and closes the controls. It does not acknowledge or dismiss anything. Starting a snooze from an action-needed prompt removes only that printer from the queue; other printers remain available for review. Cancelling the controls returns to the existing prompt. Close dialogs before using `q` or `Ctrl+C` to quit.

Snoozes use absolute host-clock deadlines. Until the deadline, automatic modals and desktop notifications are suppressed for all occurrences on that printer, including new or recurring faults. At expiry, or after `u`, session-only prompt suppression for that printer is cleared. Visible, unacknowledged actionable notices can prompt again after the normal batching delay, subject to configured category/master switches. Resolved conditions do not generate synthetic alerts. Monitoring and automatic notification delivery end when the TUI closes.

## Persistence and other review actions

| Action | Effect on later automatic alerts |
| --- | --- |
| Configure a category/master switch off | Suppress the corresponding interruptions until configuration changes |
| Snooze | Suppress all that printer's automatic interruptions until its deadline |
| Acknowledge (`a`) | Save that occurrence as reviewed; snooze expiry does not undo it |
| Dismiss (`c`) | Hide that occurrence locally; snooze expiry does not restore it |
| Close a modal prompt (`Esc`) | Suppress its modal occurrences for this TUI session; snooze expiry/resume re-arms the printer. Desktop delivery has separate occurrence tracking |
| Reset notice decisions (`n`, then `R`) | Reset acknowledgements/dismissals, without changing preferences or snoozes |
| Manual inspection (`!`) | Inspect eligible unreviewed actionable notices even if muted or snoozed; saved reviews still apply |

Manual inspection does not end the snooze or edit configuration. Closing that manual prompt during a snooze does not prevent the deadline from re-arming its occurrences.

Snooze deadlines are queued for background saving as soon as they change, separately for each Bambu account and region, in the OS configuration directory under `bambu-tui/alerts/`. Files have private permissions and contain only printer identifiers and deadlines. Tokens stay in the OS credential store; review decisions stay in their existing notice store. A custom configuration path does not change account-scoped storage.

On restart, unexpired deadlines remain effective. Expired deadlines are removed on the next update. Successful device discovery removes snoozes for printers no longer bound to the account; snoozes are retained while discovery has not completed. Renaming a printer preserves its deadline. A returning removed printer starts without its previous snooze; configured overrides still apply.

Write failures produce a persistent warning and the current session continues with its local deadline. A failed load preserves the unreadable/unsupported file and disables snooze writes for that session. Another explicit snooze/resume action retries a previous write failure; the app does not continuously retry on every tick. Demo mode uses only in-memory deadlines and never loads or saves account snoozes. Normal quit waits up to five seconds for pending notice and snooze saves; failures or timeouts require an explicit exit or cancellation. See [saved notice decisions](usage.md#saved-notice-decisions) for shutdown controls.

## Live desktop notifications

On a local macOS desktop, first run `bambu-tui doctor --notify` to allow notifications and verify the helper. Then open Config with `,` and enable Desktop notifications. Adjust other defaults in Alerts or use Printers for overrides; external TOML edits require restart. Defaults preserve existing behavior: desktop delivery and sound are off. No notification permissions are requested automatically while monitoring.

The live TUI sends intervention-required and telemetry-confirmed completion notifications through the branded helper while its session runs. Acknowledged/dismissed incidents, disabled categories, and snoozed printers are suppressed. A pending unreviewed incident or still-finished job may become eligible after snooze expiry. Fresh, connected telemetry is required; missing or stale data cannot establish resolution or completion. Job names, ETA, and 100% progress are not completion evidence. Missing/zero or reused job identities can conservatively suppress completion. Eligible intervention notices co-reported for the same printer are included in one desktop notification, with a notice count when several are included. Their review and deduplication identities remain separate. By default, other printers and completion notifications stay separate. An optional burst window can group them across reports; see [Quiet hours and burst grouping](#quiet-hours-and-burst-grouping).

Notification identity and delivery reservations persist per account/region under `bambu-tui/notifications/` in the OS configuration directory, independently of notice reviews and snoozes. Files are private and contain hashed identities, random occurrence IDs, and attempt states, not raw job IDs, printer names, or credentials. Accepted/failed/ambiguous attempts are not automatically resent after reconnect or restart. Reserving before the OS call favors avoiding duplicates: a crash in that gap can lose a notification. Corrupt/unsupported/full/unwritable storage, lock contention, or queue overflow pauses delivery with a persistent warning; monitoring continues. Fix the problem, then press `r` in Notification activity to recheck the existing ledger. No automatic retention purge is implemented.

The footer reports recent acceptance/suppression, and `s` shows effective desktop/sound/completion settings plus session-wide accepted/failed/suppressed counts and the latest delivery activity. Failures remain indicated in the footer; `Ctrl+E` shows complete details and recovery guidance when messages overlap. Acceptance means macOS accepted the request; Focus, notification permissions/settings, and audio settings can suppress actual presentation. Notification titles show the triggering printer name (prefixed Demo in demo mode); bodies also include that name and a generic intervention/completion message, without job names or raw telemetry. OS notification settings control lock-screen visibility.

Delivery runs off the reducer. Policy changes cancel an in-flight helper when possible, but already accepted OS notifications cannot be withdrawn. Shutdown pauses submissions and cancels delivery while persistence finishes; Esc can resume the session. Normal shutdown waits up to five seconds, then offers the existing exit/cancel choice. A final worker join is bounded; unconfirmed shutdown/outcomes are reported after terminal restoration. No new submission starts after shutdown cancellation, and no background service or future delivery trigger is installed. An accepted request may still be presented later by macOS.

To test automatic delivery without credentials or hardware, use the same opt-in configuration with `bambu-tui --demo --config /path/to/demo.toml`. From a source checkout with a repository-local config, use `make demo RUN_ARGS='--config ./config.toml'`. Demo notifications are labelled Demo and use a temporary ledger deleted on clean exit; real account state is not read or written. The synthetic runout occurs after about 15 seconds and completion after about 45 seconds. Restarting the demo starts fresh notification identities.

## Desktop notification test and sound

Doctor remains an explicit, repeatable test independent of the ledger. On macOS, you can explicitly test desktop delivery from a local terminal, without logging in or connecting to a printer:

```sh
./bambu-tui doctor --notify
./bambu-tui doctor --notify --sound
```

The first command requests a notification without sound; the second requests the default macOS notification sound. Both invoke the branded Bambu TUI helper to submit a fixed synthetic message. They skip the credential-store probe and configuration loading, and do not read account or printer data or change alert preferences. `--sound` requires `doctor --notify`.

macOS archives include `Bambu TUI Notify.app` beside the `bambu-tui` executable. Keep them together when extracting or moving the installation. From a source checkout on macOS, build both:

```sh
make build
make notify-helper
./bin/bambu-tui doctor --notify
```

The app must be named `Bambu TUI Notify.app` and contain `Contents/MacOS/bambu-notify`. Doctor locates it beside the resolved CLI executable, including when the CLI is launched through a symlink; the current directory is not searched. A missing or invalid helper reports build guidance. There is no AppleScript fallback. Doctor registers the adjacent bundle with macOS before requesting the test notification. See [helper build, test, and cleanup instructions](../development/contributing.md#macos-notification-helper) for prerequisites and local registration details.

The helper uses the Bambu TUI logo and requests its own notification permission. Allow Bambu TUI notifications if macOS prompts. Success means the native service accepted the request; it does not confirm that a banner appeared or sound played. Check the desktop and Notification Center. If nothing appears, check Bambu TUI in System Settings → Notifications and the active Focus. Focus can delay or suppress a banner even when app notifications are enabled.

If a delivered notification shows only “Notification,” open System Settings → Notifications → Bambu TUI → Show previews and choose When Unlocked or Always. Default inherits the system setting and can effectively hide all preview text, including the printer title. The helper’s `--status` command reports the effective preview setting.

Banner lifetime is controlled by macOS. For notifications that stay until dismissed, open System Settings → Notifications → Bambu TUI and set Alert Style to Persistent instead of Temporary. The helper does not remove the notification when it exits. See [Apple’s notification settings guide](https://support.apple.com/guide/mac-help/mh40583/mac).

Test both commands with the terminal focused, then unfocused. To allow time to switch applications, use `sleep 3; ./bin/bambu-tui doctor --notify` (append `--sound` for audio). Record whether the banner appeared and whether audio played separately from command acceptance. The helper allows up to 60 seconds for permission/service access, and doctor bounds its execution to 65 seconds and cancels it on interruption. SSH sessions are explicitly unsupported, Linux delivery is not implemented, and headless delivery is unverified.

This test does not enable automatic notifications or leave monitoring running. Configure live delivery separately as described above. Explicit tests bypass quiet hours, grouping, and automatic delivery rate limits.
