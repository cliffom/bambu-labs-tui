# Validation record

## v0.1.7 — Current run: live validation pending

Tracking: [#38](https://github.com/cliffom/bambu-tui/issues/38), following closed #6. Use the [live-validation worksheet](live-validation.md) to perform and report this run. v0.1.7 is a local candidate, not a published release. No new hardware or normal-terminal results have been established for this run; earlier observations below retain their original version and scope.

### Environment and evidence

| Field | Current evidence |
| --- | --- |
| Candidate version | 0.1.7 |
| Source baseline | `2ff884cdb19cdcd622fdcf1e67fe18b75ca9b680` (v0.1.6), with pending version/documentation changes |
| Automated check host | macOS 27.0.1, arm64; Go 1.27.3 |
| Automated check date | 2026-10-25 UTC |
| Live host OS and terminal/version | Not supplied for this run |
| H2D installed/offered OTA firmware | Not supplied for this run |
| X2D installed/offered OTA firmware | Not supplied for this run |
| Live test date / observer | Not supplied for this run |

### Current automated results

Executed in the development workspace on the host/date above:

| Check | Result |
| --- | --- |
| `make test` | Pass (race-enabled repository tests; cached results where applicable) |
| `make check` | Pass (`go vet ./...`) |
| `make fmt-check` | Pass |
| `make build` | Pass |
| Pinned golangci-lint v2.15.0 | Pass, zero issues |
| `./bin/bambu-tui --version` | `bambu-tui 0.1.7 (commit 2ff884cdb19c, built 2026-10-25T04:36:07Z)` |

These establish a candidate build and automated regression results. They do not establish credential-store, terminal, cloud reconnect, or printer behavior on the live setup.

### Current live results

| Check | Status | Observation |
| --- | --- | --- |
| Saved email-code login and restart without reauthentication | Not run | — |
| Logout removes saved session | Not run | — |
| Firmware/update assessment compared with Bambu software | Not run | — |
| Telemetry/filament paths compared during a real print | Not run | — |
| Reported pause, recovery, and completion | Not run | — |
| Monitor connection interruption/recovery and fresh telemetry | Not run | — |
| Acknowledgement/dismissal survives restart and routine reports | Not run | — |
| Observed resolution and recurrence returns unreviewed | Not run | — |
| Snooze deadline survives restart; early resume works | Not run | — |
| Real deadline expiry and eligible alert re-arming | Not run | — |
| Global/per-printer interruption preferences | Not run | — |
| Keyboard, modal return/quit, resize, and continued telemetry | Not run | — |

Update each row with the observed result, date, candidate build identifier, and printer alias/model where applicable. Use **Not observed** for a scenario unavailable during a real session, **Fail** for a reproduced mismatch, and **Pass** only for an executed check with an observation. Record models separately if results differ. Keep #38 open while required evidence is missing.

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

Unchecked items below remain unverified. Closing a tracking issue or publishing any release does not count as passing these checks.

- [ ] Validate global/per-printer alert preferences, snooze/resume, restart persistence, and expiry on H2D/X2D. v0.0.9 has synthetic/store/demo coverage; live interruption behavior remains unconfirmed.

- [ ] Compare installed OTA firmware and reported update status against the printer/Bambu app on H2D and X2D; record versions. The v0.0.6 firmware parser and read-only query have synthetic/broker/demo coverage, not confirmed live firmware results.
- [x] Validate displayed telemetry on H2D and X2D. User confirmed both models after v0.0.5; operational checks remain separate.
- [x] Run `./bin/bambu-tui doctor` from a normal terminal. User confirmed save, reload, and deletion succeeded on 2026-10-23.
- [ ] Run `login --email-code`, confirm the saved-session message, quit, and start `bambu-tui` without logging in again.
- [ ] Compare H2D and X2D progress, remaining time, temperatures, and active filament paths against Bambu Studio during a real print.
- [ ] Observe pause/resume and completion; verify completion is reported from telemetry rather than inferred from elapsed time.
- [ ] Interrupt the monitor's network connection, restore it, and verify reconnection and fresh telemetry. Do not interrupt printer power or a running job.
- [ ] Dismiss a notice and verify routine reports do not restore it; resolve/recur a condition through normal printer use and verify it returns.
- [ ] Run `logout`; verify the next startup requests login.

Leave incomplete items unchecked. Record model/firmware and results here after performing them; never attach credentials, unredacted serials, or account tokens.
