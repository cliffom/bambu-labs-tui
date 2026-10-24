# v0.0.1 validation record

## Observed live

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

- [x] Run `./bin/bambu-tui doctor` from a normal terminal. User confirmed save, reload, and deletion succeeded on 2026-10-23.
- [ ] Run `login --email-code`, confirm the saved-session message, quit, and start `bambu-tui` without logging in again.
- [ ] Compare H2D and X2D progress, remaining time, temperatures, and active filament paths against Bambu Studio during a real print.
- [ ] Observe pause/resume and completion; verify completion is reported from telemetry rather than inferred from elapsed time.
- [ ] Interrupt the monitor's network connection, restore it, and verify reconnection and fresh telemetry. Do not interrupt printer power or a running job.
- [ ] Dismiss a notice and verify routine reports do not restore it; resolve/recur a condition through normal printer use and verify it returns.
- [ ] Run `logout`; verify the next startup requests login.

Leave incomplete items unchecked. Record model/firmware and results here after performing them; never attach credentials, unredacted serials, or account tokens.
