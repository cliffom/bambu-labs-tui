# Alert preferences and snoozing

[Back to the README](../README.md)

Bambu TUI separates **what the printer reports** from **when an automatic modal interrupts you**. Preferences and snoozes change only interruptions. Printer state, faults, notice counts, and event history remain visible. No printer controls are sent.

## Configure automatic alerts

Add these settings to the file printed by `bambu-tui --config-path`, or your custom `--config` file. Restart the TUI after editing. Existing configurations retain these defaults:

```toml
[alerts]
automatic_modals = true
filament = true
extrusion = true
printer_faults = true
failed_prints = true
unknown_pauses = true

# Example: disable automatic interruptions for a maintenance printer.
# Replace the example ID with the Printer ID shown by s in the dashboard.
[alerts.printers."YOUR_PRINTER_ID"]
automatic_modals = false

# Another printer can override just one category, inheriting all other settings.
[alerts.printers."ANOTHER_PRINTER_ID"]
filament = false
```

Each per-printer field overrides the corresponding global field independently. An explicit `true` can override a global `false`. Printer IDs are stable identifiers, so renaming a printer does not change its preferences. Overrides for absent printers remain in your configuration and apply if they return. The configuration contains no credentials. Booleans are required; unknown keys, unsupported options such as `sound`, and empty printer IDs are rejected.

| Setting | Automatic interruptions it permits |
| --- | --- |
| `automatic_modals` | Master switch for that printer's automatic action-needed modals |
| `filament` | Verified runout and filament feed/retract/path failures |
| `extrusion` | Verified nozzle/extrusion faults requiring intervention |
| `printer_faults` | Other verified intervention faults and unknown non-info faults on a stopped printer |
| `failed_prints` | The independently reported failed-print state |
| `unknown_pauses` | An ambiguous pause after both HMS and print-error snapshots arrive, without a verified intervention, recovery, or manual-pause reason |

Preferences filter the existing verified actionable catalogue; enabling a category does not turn maintenance, low-filament estimates, explicit user pauses, or automatic recovery into automatic alerts. Classification uses verified fault metadata, never printer message wording. Related paused-state notices follow the enabled categories of their unreviewed faults, so muting a runout does not produce a duplicate generic pause prompt. Multiple fault categories can contribute to that state; any enabled contributing category permits it. A separately reported failed print has its own switch.

The firmware/telemetry view and notice panels are never filtered by these settings. The printer details show alert status; press `s` to see the effective switches and stable printer ID. The overview counts snoozed printers and printers with all automatic interruptions disabled.

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

Snoozes use absolute host-clock deadlines. Until the deadline, automatic prompts are suppressed for all occurrences on that printer, including new or recurring faults. At expiry, or after `u`, session-only prompt suppression for that printer is cleared. Visible, unacknowledged actionable notices can prompt again after the normal batching delay, subject to configured category/master switches. Resolved conditions do not generate synthetic alerts. No notifications are delivered while the TUI is closed.

## Persistence and other review actions

| Action | Effect on later automatic alerts |
| --- | --- |
| Configure a category/master switch off | Suppress the corresponding interruptions until configuration changes |
| Snooze | Suppress all that printer's automatic interruptions until its deadline |
| Acknowledge (`a`) | Save that occurrence as reviewed; snooze expiry does not undo it |
| Dismiss (`c`) | Hide that occurrence locally; snooze expiry does not restore it |
| Close a prompt (`Esc`) | Suppress its occurrences for this TUI session; snooze expiry/resume re-arms the printer |
| Reset notice decisions (`n`, then `R`) | Reset acknowledgements/dismissals, without changing preferences or snoozes |
| Manual inspection (`!`) | Inspect eligible unreviewed actionable notices even if muted or snoozed; saved reviews still apply |

Manual inspection does not end the snooze or edit configuration. Closing that manual prompt during a snooze does not prevent the deadline from re-arming its occurrences.

Snooze deadlines save immediately, separately for each Bambu account and region, in the OS configuration directory under `bambu-tui/alerts/`. Files have private permissions and contain only printer identifiers and deadlines. Tokens stay in the OS credential store; review decisions stay in their existing notice store. A custom configuration path does not change account-scoped storage.

On restart, unexpired deadlines remain effective. Expired deadlines are removed on the next update. Successful device discovery removes snoozes for printers no longer bound to the account; snoozes are retained while discovery has not completed. Renaming a printer preserves its deadline. A returning removed printer starts without its previous snooze; configured overrides still apply.

Write failures produce a persistent warning and the current session continues with its local deadline. A failed load preserves the unreadable/unsupported file and disables snooze writes for that session. Another explicit snooze/resume action retries a previous write failure; the app does not continuously retry on every tick. Demo mode uses only in-memory deadlines and never loads or saves account snoozes.

## Sound

Sound is off: this version has no sound delivery backend and exposes no sound preference. Optional sound/desktop notification delivery belongs to [notification issue #11](https://github.com/cliffom/bambu-tui/issues/11). The later backend must honor these preferences, snoozes, and saved reviews.
