# Alert preferences and snoozing

[Back to the README](../../README.md)

Bambu TUI separates **what the printer reports** from **when automatic modals or opt-in desktop notifications interrupt you**. Preferences and snoozes change only interruptions. Printer state, faults, notice counts, and event history remain visible. No printer controls are sent.

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

On a local macOS desktop, first run `bambu-tui doctor --notify` to allow notifications and verify the helper. Then set `desktop_notifications = true` under `[alerts]` (or a printer override) and restart. Set `notification_sound = true` to request sound, and `completed_prints = false` to disable completion alerts. Defaults preserve existing behavior: desktop delivery and sound are off. No notification permissions are requested automatically while monitoring.

The live TUI sends intervention-required and telemetry-confirmed completion notifications through the branded helper while its session runs. Acknowledged/dismissed incidents, disabled categories, and snoozed printers are suppressed. A pending unreviewed incident or still-finished job may become eligible after snooze expiry. Fresh, connected telemetry is required; missing or stale data cannot establish resolution or completion. Job names, ETA, and 100% progress are not completion evidence. Missing/zero or reused job identities can conservatively suppress completion. Eligible intervention notices co-reported for the same printer are included in one desktop notification, with a notice count when several are included. Their review and deduplication identities remain separate. Other printers and completion notifications stay separate; timed grouping across separate reports is not implemented yet.

Notification identity and delivery reservations persist per account/region under `bambu-tui/notifications/` in the OS configuration directory, independently of notice reviews and snoozes. Files are private and contain hashed identities, random occurrence IDs, and attempt states, not raw job IDs, printer names, or credentials. Accepted/failed/ambiguous attempts are not automatically resent after reconnect or restart. Reserving before the OS call favors avoiding duplicates: a crash in that gap can lose a notification. Corrupt/unsupported/full/unwritable storage, lock contention, or queue overflow pauses delivery with a persistent warning; monitoring continues. Fix the problem and restart to retry the worker. No automatic retention purge is implemented.

The footer reports recent acceptance/suppression, and `s` shows effective desktop/sound/completion settings plus session-wide accepted/failed/suppressed counts and the latest delivery activity. Failures remain visible. Acceptance means macOS accepted the request; Focus, notification permissions/settings, and audio settings can suppress actual presentation. Notification titles show the triggering printer name (prefixed Demo in demo mode); bodies also include that name and a generic intervention/completion message, without job names or raw telemetry. OS notification settings control lock-screen visibility.

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

This test does not enable automatic notifications or leave monitoring running. Configure live delivery separately as described above. Quiet hours, timed grouping across reports, and interactive preference editing remain planned under [notification issue #11](https://github.com/cliffom/bambu-tui/issues/11).
