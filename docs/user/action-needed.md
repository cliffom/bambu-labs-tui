# Action-needed alerts

The automatic dialog surfaces operator-attention conditions while preserving the existing read-only monitor and per-notice decisions.

## Trigger rules

- Verified filament runout, empty-slot faults, filament breakage, feeder/tangle/buffer problems, feed/retract failures, failed runout recovery, and supported extrusion/nozzle faults carry explicit intervention classifications. Supported heatbed sensor, camera disconnection, chamber-control malfunction, and abnormal firmware faults also warrant review. Classification uses catalogue codes; printer-supplied wording never changes it.
- A reported failed print warrants review even without a known fault code. Unmapped non-informational faults accompanying `PAUSE` or `FAILED` retain their reported message/code and receive generic guidance, without an inferred diagnosis.
- A pause without a known intervention, explicit user-pause reason, or automatic recovery gets a generic review prompt after both `hms` and `print_error` snapshots have arrived. Omitted fields in partial startup reports do not establish a reason or resolution.
- Known maintenance notices, low estimated filament, cloud/offline/stale notices, explicit user pauses, and automatic purge/backup-spool switch messages do not independently open the dialog. A separate intervention-required fault or failed state remains eligible when reported alongside recovery.

The catalogue is deliberately partial. Unknown errors while the printer reports running stay visible in Notices without an automatic modal. Connection loss does not prove a printer condition resolved; the dialog labels the last reported state and current connection status.

## Batching and navigation

The first eligible occurrence starts a two-second collection window. Available incidents are presented in one dialog, grouped by printer. Additional reports update the current group or append another printer without replacing the dialog being read. Grouping is presentation only: HMS, print-error, and state notice identities remain separate.

Help, notice inspectors, the AMS/filament browser, the notice decisions view, printer alert controls, and reset/history confirmations defer opening. Alerts also wait until the terminal reaches 60 columns × 18 rows. Transport events, reconnects, and status refresh processing continue while the dialog is open. Resolved or replaced occurrences lose action guidance and remain marked in the dialog until the operator leaves it. If later telemetry supplies an explicit pause/recovery explanation, an earlier generic pause can be marked as no longer requiring an alert.

| Key | Effect |
| --- | --- |
| `Tab` / `Shift+Tab` | Cycle queued printers |
| Arrows / `j` / `k` / page keys | Scroll the selected printer's alert |
| `Enter` | Leave the dialog and open a current notice for the selected printer |
| `p` | Leave the dialog and select that printer's details |
| `o` | Open the first displayed HMS troubleshooting URL |
| `a` | Acknowledge the current printer's displayed active occurrences, save decisions, and advance to the next printer |
| `s` | Open snooze controls for the current printer without reviewing its notices |
| `c` | Dismiss those occurrences locally, save decisions, and advance; printer telemetry is unchanged |
| `Esc` | Close all queued prompts without acknowledging/dismissing notices; preserve dashboard focus |
| `q` / `Ctrl+C` | Ignored while the modal is open; close it with `Esc` before quitting from the dashboard |

Closing or navigating away suppresses automatic re-prompting for the dialog's occurrences during the current session. It does not persist an acknowledgement. `!` from the dashboard explicitly reopens eligible unreviewed alerts. Acknowledgement and dismissal queue background saves to account/region-scoped storage; storage failures remain visible.

## Reading an alert

The dialog emphasizes the printer, reported state/current connection, and each independent fault summary. Suggested physical intervention appears directly below its fault as **Next**. Review status and the single code line are secondary text. `o` opens the first displayed HMS troubleshooting link; `Enter` opens a current notice with its full, copyable URL. Printer details and Notices provide access to other notices individually. The single-row footer keeps acknowledgement, hide, snooze, and close visible; notice/printer/browser navigation stays below the heading.

Each occurrence shows **First observed** with an absolute local timestamp, timezone and UTC offset, and a relative age (just now, minutes, hours, or days). Ages update while the dialog remains open without moving focus or scroll. This is when the app first observed that occurrence, not a confirmed printer-side fault start or desktop delivery time. Unavailable timestamps say unavailable; a future timestamp explicitly reports clock skew rather than a negative age. Elapsed time never establishes resolution.

Routine reports, reopening, and reconnects preserve an ongoing occurrence's time. Saved reviews restore their existing observation time when matched by telemetry; unreviewed conditions first discovered after startup use that session's observation. Confirmed recurrence receives a new time. Every notice keeps its own timestamp even when grouped; an older occurrence remains labelled as replaced if its recurrence arrives while the dialog is open.

## Recurrence and startup

Saved reviews are restored before telemetry arrives. Reviewed faults do not re-prompt through a separately reported paused state. Independent failed-state reviews remain meaningful. Each actual notice uses its key and first-seen timestamp as its occurrence identity; confirmed resolution and recurrence, another printer, or a different fault remain eligible for attention.

Initial eligible conditions are batched into the same queue rather than stacked dialogs. Partial reports arriving while a dialog is open join that dialog. A different code arriving after it closes can prompt again: the app cannot reliably prove it belongs to the same incident. Likewise, resolution/recurrence entirely while the app is closed cannot be distinguished without a printer-provided occurrence identifier.

The modal never sends resume, heat, motion, fault-clear, or other printer-control commands. Physical intervention and printer-side confirmation remain actions on the printer or in Bambu's own applications.

## Validation

Run `make demo` to build and launch the simulated farm without logging in.

Synthetic H2D/X2D fixtures cover AMS/AMS HT/external runout, feed failures, clogs, and empty slots. Tests also cover unknown stopped-printer faults, failed/ambiguous/explicit user pauses, recovery, wording independence, persisted reviews, incremental reports, multiple printers, dialog deferral, live resolution, recurrence, focus restoration, keyboard navigation, and terminal bounds. The existing demo introduces a runout incident at approximately 15 seconds and clears it at approximately 24 seconds; no live printer is needed and no demo reviews are saved.

Real printer validation is still a hardware smoke check; synthetic and terminal-demo results do not establish live behavior for every model/firmware fault.

## Reversing local decisions

Press `n` on the dashboard to manage saved notice decisions. Restoring a dismissal preserves acknowledgement. Marking a notice unreviewed preserves dismissal. Either explicit reversal clears session prompt suppression for that occurrence; it becomes eligible only when active, visible, unacknowledged, and actionable. The alert waits until the decisions view closes. A confirmed account/region reset clears all review flags and session prompt suppression, without creating faults or restoring deleted event history. See [saved notice decisions](usage.md#saved-notice-decisions) for controls and startup/storage behavior.

## Interruption policy

Automatic prompts respect configured global/per-printer category switches and saved snooze deadlines. Related pause-state alerts follow their contributing verified fault categories, so a muted fault does not bypass policy through a duplicate state notice. `!` is an explicit manual inspection and bypasses interruption policy while still respecting saved reviews. Snooze expiry/resume re-arms only that printer, preserving other printers' session suppression. See [alert preferences and snoozing](alerts.md).
