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

Help, notice inspectors, and history confirmations defer opening. Alerts also wait until the terminal reaches 60 columns × 18 rows. Transport events, reconnects, and status refresh processing continue while the dialog is open. Resolved or replaced occurrences lose action guidance and remain marked in the dialog until the operator leaves it. If later telemetry supplies an explicit pause/recovery explanation, an earlier generic pause can be marked as no longer requiring an alert.

| Key | Effect |
| --- | --- |
| `Tab` / `Shift+Tab` | Cycle queued printers |
| Arrows / `j` / `k` / page keys | Scroll the selected printer's alert |
| `Enter` | Leave the dialog and open a current notice for the selected printer |
| `p` | Leave the dialog and select that printer's details |
| `o` | Open the first displayed HMS troubleshooting URL |
| `a` | Acknowledge the current printer's displayed active occurrences, save decisions, and advance to the next printer |
| `c` | Dismiss those occurrences locally, save decisions, and advance; printer telemetry is unchanged |
| `Esc` | Close all queued prompts without acknowledging/dismissing notices; preserve dashboard focus |
| `q` / `Ctrl+C` | Quit |

Closing or navigating away suppresses automatic re-prompting for the dialog's occurrences during the current session. It does not persist an acknowledgement. `!` from the dashboard explicitly reopens eligible unreviewed alerts. Acknowledgement and dismissal use the existing immediate account/region-scoped storage; storage failures remain visible.

## Recurrence and startup

Saved reviews are restored before telemetry arrives. Reviewed faults do not re-prompt through a separately reported paused state. Independent failed-state reviews remain meaningful. Each actual notice uses its key and first-seen timestamp as its occurrence identity; confirmed resolution and recurrence, another printer, or a different fault remain eligible for attention.

Initial eligible conditions are batched into the same queue rather than stacked dialogs. Partial reports arriving while a dialog is open join that dialog. A different code arriving after it closes can prompt again: the app cannot reliably prove it belongs to the same incident. Likewise, resolution/recurrence entirely while the app is closed cannot be distinguished without a printer-provided occurrence identifier.

The modal never sends resume, heat, motion, fault-clear, or other printer-control commands. Physical intervention and printer-side confirmation remain actions on the printer or in Bambu's own applications.

## Validation

Run `make demo` to build and launch the simulated farm without logging in.

Synthetic H2D/X2D fixtures cover AMS/AMS HT/external runout, feed failures, clogs, and empty slots. Tests also cover unknown stopped-printer faults, failed/ambiguous/explicit user pauses, recovery, wording independence, persisted reviews, incremental reports, multiple printers, dialog deferral, live resolution, recurrence, focus restoration, keyboard navigation, and terminal bounds. The existing demo introduces a runout incident at approximately 15 seconds and clears it at approximately 24 seconds; no live printer is needed and no demo reviews are saved.

Real printer validation is still a hardware smoke check; synthetic and terminal-demo results do not establish live behavior for every model/firmware fault.
