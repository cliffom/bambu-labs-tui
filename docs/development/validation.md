# Validation record

[Contributing](contributing.md) · [Documentation index](../README.md)

This is the ongoing record of observed hardware behavior and remaining operational checks. Follow-up work is tracked in [#38](https://github.com/cliffom/bambu-tui/issues/38). Record application version, host OS/terminal, printer model/firmware, date, and actual results for each new check.

## v0.1.8 — Live job transitions and connection recovery

The user's 2026-10-25 screenshot and report show real session events during a job start and network disconnects on v0.1.8. The user subsequently confirmed that they restarted their routers, providing the network-interruption context for this recovery evidence:

- Printzilla (H2D, firmware `01.03.00.00`) transitioned from finished → prepare at 09:56:18, then prepare → printing at 09:56:29. At capture time (11:11:06), it showed Printing, 11% progress, layer 74 / 896, and a last report at 11:11:05.
- A cloud-disconnected/reconnecting event at 09:51:40 resolved at 09:51:41.
- Another cloud-disconnected/reconnecting event at 10:46:18 resolved at 10:48:17. Printer-offline events for Printy and Printzilla appeared at 10:48:17 and resolved at 10:48:28 and 10:48:31 respectively.
- At capture time, the dashboard showed Cloud Connected, all three printer links Live, zero unavailable printers, and zero active notices. Resolved conditions remained visible in session history.

This confirms observed live job-state transitions and reconnection with fresh H2D telemetry following a user-reported router restart. The report does not map individual router restarts to the two disconnect intervals or isolate monitor connectivity from printer connectivity; the offline events alone do not establish that either printer physically lost connectivity. Automatic action-needed modals, desktop/sound delivery, saved review behavior, and pause/completion remain unverified by this screenshot.

## v0.1.8 — Idle telemetry grace period

The user's 2026-10-25 screenshot of an overnight v0.1.6 cloud session showed all three printers currently idle or finished and repeated stale/resolved events. For Printzilla, the displayed last-report and resolution times were 03:09:02 → 03:18:33 (9m31s), 02:57:12 → 03:01:51 (4m39s), and 03:18:35 → 03:21:27 (2m52s). These are apparent gaps inferred from event text, not a captured report-cadence trace: resolution can also reflect connection/availability changes, and the printer's state during each gap was not recorded.

The ten-minute idle/finished threshold is a provisional policy choice covering the longest apparent gap in that evidence, rather than a verified protocol guarantee. Printing and all other states retain two minutes. Synthetic regressions cover threshold boundaries, fresh partial-report recovery, state transitions, missing telemetry, offline/cloud handling, and unavailable filtering.

- [ ] Observe idle/finished report intervals on H2D/X2D, record model/firmware and actual timestamps, and confirm or adjust the ten-minute threshold. Verify stale/recovery behavior and transition to printing during normal use. The screenshot alone does not complete this check.

## v0.0.5 — User-reported live validation

After the v0.0.5 release, the user confirmed that the **displayed telemetry** was validated on both their **H2D** and **X2D**. This extends the earlier H2D screenshot evidence to user-reported telemetry testing on both models.

The user clarified that this validation covered displayed telemetry, rather than login/restart, network reconnection, persisted notice decisions, or action-needed modal behavior. Firmware versions were not provided. Keep those operational checks open, and do not infer a field-by-field comparison against Bambu Studio from this confirmation.

## v0.0.1 — Observed live

From the user's 2026-10-23 screenshot:

- Email-code login reached Bambu Cloud and discovered three bound printers.
- An H2D reported an active print, job name, progress, remaining time, layers, both nozzle temperatures, bed/chamber temperatures, AMS/AMS HT inventory, material colors, and filament estimates.
- HMS notices and a low-filament notice arrived over the live connection.
- The user confirmed login fell back to a session-only dashboard because credential storage failed.
- After the release-readiness changes, the user ran `bambu-tui doctor` in a normal terminal and confirmed that credential save, reload, and deletion succeeded. This used a disposable test entry, not a Bambu account token.

This is evidence of live connectivity, not a complete verification of every field or supported model. No raw account telemetry or tokens were retained.

## Automated checks

`make test check build release` covers race detection, static checks, synthetic H2D/X2D telemetry, partial-report merging, notice lifetimes, dismissal/recurrence, fake account login challenges, in-memory MQTT exchanges, reconnection, and platform packaging. Credential tests exercise save/reload/delete, readback validation, command failures, redaction, and safe command inputs using fake native processes.

The native credential probe in the implementation workspace failed with `security` exit 152: macOS could not obtain authorization to write the disposable entry. A follow-up lookup confirmed no diagnostic entry remained. The normal-terminal probe subsequently passed, confirming the failure is specific to the restricted execution environment. A full real-account login/restart check remains open.

## Remaining live checks

Unchecked items below remain unverified. Closing a tracking issue or publishing a release does not count as passing these checks.

- [ ] Validate global/per-printer alert preferences, snooze/resume, restart persistence, and expiry on H2D/X2D. v0.0.9 has synthetic/store/demo coverage; live interruption behavior remains unconfirmed.

- [ ] Compare installed OTA firmware and reported update status against the printer/Bambu app on H2D and X2D; record versions. The v0.0.6 firmware parser and read-only query have synthetic/broker/demo coverage, not confirmed live firmware results.
- [x] Validate displayed telemetry on H2D and X2D. User confirmed both models after v0.0.5; operational checks remain separate.
- [x] Run `./bin/bambu-tui doctor` from a normal terminal. User confirmed save, reload, and deletion succeeded on 2026-10-23.
- [ ] Run `login --email-code`, confirm the saved-session message, quit, and start `bambu-tui` without logging in again.
- [ ] Compare H2D and X2D progress, remaining time, temperatures, and active filament paths against Bambu Studio during a real print.
- [ ] Observe pause/resume and completion; verify completion is reported from telemetry rather than inferred from elapsed time.
- [x] Verify reconnection and fresh telemetry after a network interruption. On 2026-10-25, the user reported restarting their routers; the v0.1.8 screenshot shows resolved disconnect/offline notices and subsequent fresh H2D printing telemetry. This was a router restart rather than an isolated monitor-only interruption; see the evidence above.
- [ ] Acknowledge and dismiss notices, restart, and verify saved decisions persist and routine reports do not restore dismissed notices. Resolve/recur a condition through normal printer use and verify it returns unreviewed.
- [ ] Verify action-needed prompts during naturally occurring intervention conditions; confirm recovery/maintenance do not interrupt and telemetry continues while dialogs are open.
- [ ] Exercise farm filters/sorting, keyboard/modal navigation, and resizing in a normal terminal while telemetry continues.
- [ ] Preview and inspect a local diagnostic export on the supported live setup before voluntarily sharing any data.
- [ ] Run `logout`; verify the next startup requests login.

Do not deliberately fault a printer to satisfy a checklist. Record unavailable scenarios as untested with a clear limitation. Leave incomplete items unchecked. Record model/firmware and results here after performing them; never attach credentials, unredacted serials, or account tokens.
