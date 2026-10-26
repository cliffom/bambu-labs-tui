# Validation record

[Contributing](contributing.md) · [Documentation index](../README.md)

This is the ongoing record of observed hardware behavior and remaining operational checks. Follow-up work is tracked in [#38](https://github.com/cliffom/bambu-tui/issues/38). Record application version, host OS/terminal, printer model/firmware, date, and actual results for each new check.

## Unreleased — Synthetic macOS notification probe

### Config modal and immediate notification preferences

On 2026-10-25 (PDT), development version 0.1.15 plus Config changes on base commit `0031e6f78db7` was checked on the same macOS 27.0.1 arm64 host, using Go 1.27.3 and actual synthetic demo PTYs. No account, printer, or firmware was involved. Plain terminal captures were inspected at 120×32 (initial Off, toggled On/Saved, and session activity), 60×18 (selected/unselected toggles, Off/On, Saved, and an injected save failure), and 59×17 (resize fallback). Returning to 60×18 preserved selection and settings; Enter while undersized did not toggle. Both processes exited normally, including the explicit exit choice after save failure. Color appearance and empty/disconnected farm layouts were not separately inspected in this run.

The demo began with desktop delivery off, enabled it through Config, disabled/re-enabled it, and showed one accepted intervention submission with zero failures after the synthetic runout. This establishes startup-off/live-enable integration and macOS acceptance, not visible banner/audio presentation or hardware behavior. The active synthetic config file contained the saved global values and retained automatic_modals=false. A separate 60×18 process replaced its config path with a directory after startup; toggling then showed Save failed / r retry and a persistent footer warning while retaining the session value.

Regression tests cover immediate delivery policies, initial opt-out without ledger observation, sound/completion navigation, override precedence, dialog ownership/deferred attention, size bounds/hidden toggles, asynchronous immutable snapshots, serialization/coalescing, failed-save retry, and pending/failed-save quit handling. Config store tests cover missing files/private permissions, preservation of unrelated values and override inheritance, and refusal to overwrite malformed/unknown-key files. `make fmt-check lint test check build docs-check` passed; lint used the pinned v2.15.0 through go run. Slow-save display duration, automatic completion/audio after an interactive toggle, real-printer behavior, and separate color review remain unverified. Saving normalizes TOML formatting and removes comments as disclosed in the modal/user guide.

A follow-up user screenshot showed that Config's footer contained only the TOML save note. The footer now shows close/select/toggle keys, and retains close/retry keys during save failure; the TOML note moved into the body. On the same host/date, actual demo PTY captures were inspected at 120×32 (normal footer), 60×18 (normal, save failure, and restored layout), and 59×17 (resize fallback). Both processes exited normally. Regression tests cover footer close keys and failure/retry text at 60 and 120 columns. Formatting, pinned lint, UI/CLI race tests, vet, build, and docs checks passed. Color appearance was not separately reviewed.

### Config notification test action

On 2026-10-25 (PDT), development version 0.1.15 plus the Test Notification action on base `52cbb7925bbd` was checked on the same macOS 27.0.1 arm64 host with Go 1.27.3. Actual synthetic demo PTYs were inspected at 120×32 (selected action and accepted result), 60×18 (accepted result and resize restoration), and 59×17 (fallback and blocked hidden activation). Each test used sound Off and automatic desktop delivery Off; both requests were accepted by macOS, and both processes exited normally. Config files remained unchanged. No account/printer/firmware was involved. Banner/audio presentation, first-run permission prompting, color appearance, and actual long-running/failure layout were not separately inspected.

Race tests cover global sound snapshots, asynchronous invocation while opted out, no settings changes, duplicate activation blocking, pending/failure/accepted text, telemetry updates during a blocked probe, close cancellation, stale-result protection, size bounds, and blocked hidden activation. `make fmt-check lint test check build docs-check` passed with the pinned lint tool. The explicit action reuses doctor's bounded native probe; it does not consume persisted live notification identities or increment live delivery counts.

### Config selected-option helper text

After the user confirmed the Config test worked, they requested selected-option explanations in the status bar and noted that header key hints were easy to miss. Config now emphasizes header navigation/activation keys and uses its single-row footer for the selected option's purpose, retaining close keys. Save failures still take priority with retry/close controls. Pending test helper text explains cancellation on close.

On 2026-10-25 (PDT), development version 0.1.15 plus these changes on base `da6f4fc5a308` was inspected on the same macOS host using actual demo PTYs. Plain captures at 60×18 and 120×32 showed all four selected/unselected rows and their complete helper text; 59×17 showed resize fallback, and returning to 60×18 retained Test Notification selection and its helper. Both processes exited normally. No tests were sent in this layout run. UI/CLI race tests, formatting, pinned lint, vet, build, and docs checks passed. Regression coverage verifies each option's helper at both widths, pending-test help, retained header/close keys, and failure priority. Color appearance remains unverified separately.

The user then requested that Config's status bar contain helper text only. Close keys now remain solely in the emphasized header; storage warnings and retry guidance moved into the body. On the same host/date, actual demo PTYs on base `e392734c7ef2` were inspected at 60×18 and 120×32 for all four helper texts, 59×17 for fallback, and 60×18 after restoration and during an injected config-save failure. The failed-save body retained warning/retry text while the footer continued to explain the selected option. UI/CLI race tests, formatting, pinned lint, vet, build, and docs checks passed. No notifications were sent; separate color review remains open.

### Printer-name titles and preview diagnosis

The user requested the triggering printer name in notification text. Live requests now carry the printer name as a bounded plain-text title, prefixed Demo in synthetic sessions; the name remains in the body. Worker regressions cover intervention/completion/demo titles and text bounds, and the universal helper built successfully. A silent synthetic `Demo: X2D · Studio` request was accepted; native validation rejected empty, oversized, and wrong-type titles.

The user reported that the visible text still read "Notification." The helper's read-only `--status` query then reported authorization granted, alerts/sound enabled, and an effective preview setting of Never. Their screenshot shows Show previews set to Default, consistent with an inherited effective setting, plus Temporary alert style. The user was directed to select When Unlocked or Always explicitly. No system preferences were changed by the agent. After changing the preview setting, the user requested another silent synthetic test. The helper accepted it, and the user supplied a screenshot showing the Bambu TUI logo, title "Demo: X2D · Studio," and body "Printer-name preview test. No real printer event occurred." This confirms visible printer-title/body delivery after the preview change. Native status now includes the effective preview setting.

### Co-reported desktop notification grouping

After the user observed different apparent banner lifetimes, they confirmed that multiple notifications arrived close together. macOS NotificationCenter logs for the helper showed three banners at 22:40:34.568 (two requests) and 22:40:35.570, followed by a separate completion banner at 22:41:04.689. The demo emits independent HMS, print-error, and paused-state occurrences in one report. This confirms a burst; the exact banner-preemption/expiry behavior was not traced, so Temporary alert style alone does not explain the observation.

The worker now groups eligible intervention occurrences co-reported for one printer into one OS submission, with atomic member reservations and individual result/review identities. Regression tests cover one submission for three notices, unchanged deduplication after delivery, muted members remaining pending, separation of printers/completion, and rollback of a group reservation with an unknown member. A follow-up actual demo PTY on the same macOS host showed one accepted intervention submission (Accepted: 1, Failed: 0) for the three co-reported notices. The dashboard acceptance footer and session activity were inspected at 120×32. The inspector then interrupted that process while its alert dialog was open; native grouped-banner presentation/duration was not separately confirmed. Timed grouping across separate reports remains open.

### Live-session delivery worker (synthetic and local demo)

On 2026-10-25 (PDT), the development build on the same macOS 27.0.1 arm64 host was inspected in an actual demo PTY with opt-in desktop delivery and automatic modals disabled. The branded helper accepted synthetic intervention requests and the telemetry-confirmed completion request; the footer reported native acceptance, and the alert dialog showed session activity (three accepted intervention occurrences: HMS, print-error, and paused-state notices). Actual desktop banner/audio presentation for the agent’s captured run was not confirmed. In a subsequent user-run demo using the repository-local opt-in config, the user confirmed that the first desktop notification appeared and the automatic action-needed modal opened. They reported that the banner disappeared quickly; no duration or Alert Style setting was provided. This confirms user-observed demo banner/modal behavior, not automatic audio, completion presentation, or hardware validation. No account or printer was contacted; demo used temporary notification state.

Plain terminal captures were inspected at 120×32 (dashboard, alert preferences/activity, accepted intervention/completion), 60×18 (alert preferences and persistent missing-helper failure), and 59×17 (resize fallback with an open alert dialog). Both PTY processes exited normally. The first harness run did not answer terminal cursor probes and its early/completion captures were incomplete; the corrected run answered probes and reached the expected states. Layout regression tests cover these sizes without depending on color; color appearance was not separately inspected.

Race regressions cover worker restart deduplication, completion, snooze/shutdown cancellation, policy changes during reservation, bounded-queue overflow, storage/submission failure, independent modal/desktop preferences, per-printer inheritance, and effective review/category/snooze snapshots. UI regressions cover shutdown pause/resume and dimensions/failure visibility. Native `--deliver` input validation is checked without sending notifications. These tests do not establish hardware incident/completion behavior. Real-session preference/snooze timing, job identity/reprints, focused/unfocused automatic delivery, audio, and exceptional shutdown cases remain open.

### Notification identity foundation (synthetic)

The account/region ledger and immutable farm notification observations now have synthetic regression coverage for repeated reports, reconnect/restart replay, null/malformed/partial telemetry, family-specific clear/recurrence, pause-to-failure changes, startup completion baselines, matching job/copy identities, progress/ETA non-completion, and ambiguous reused/placeholder job IDs. Tests confirm that reviews change eligibility without resetting occurrence identity, that reserved/failed outcomes do not become accepted outcomes, and that failed reservation persistence does not permit a delivery attempt.

Separate-process tests confirm OS lock contention and that a reservation written by a child process remains consumed in the parent. Temporary-store tests cover account/region isolation, private permissions, omission of raw printer/job IDs and names, corrupt/unsupported files, and bounded-capacity failure without history eviction. These are synthetic/local filesystem checks; no live account or printer was used, and they do not establish hardware job-ID uniqueness or live duplicate prevention.

At this foundation-only stage, TUI delivery was not yet connected; the live-worker evidence above supersedes that implementation limitation. Hardware completion/reprint identity validation remains open. Persist-before-send intentionally chooses at-most-once attempts: a crash after reservation can lose a notification, and ambiguous attempts remain suppressed. Exact-once banner delivery is not claimed. Existing explicit doctor tests remain repeatable and do not consume ledger identities.

### Packaged macOS helper

On 2026-10-25 (PDT), `make release` produced all four candidate archives on the same macOS 27.0.1 arm64 host. Both macOS archives include the universal arm64/x86_64 helper beside the CLI; Linux archives omit it. Archive inventory/metadata checks passed, and signatures verified for both helper architectures after extraction. The extracted helper help commands also passed. From a separate `bin/release-smoke/` extraction of the Apple silicon archive, `bambu-tui doctor --notify` successfully registered the adjacent app and submitted a silent synthetic request (exit 0). This establishes registration and acceptance after local extraction, not a fresh visual/audio confirmation or clean-machine installation.

The helper is ad-hoc signed. A read-only check found no valid Developer ID Application signing identity configured locally. Developer ID signing, notarization, Gatekeeper acceptance of a downloaded/quarantined archive, and execution on Intel hardware remain unverified; a compiled x86_64 slice and verified signature do not establish Intel runtime testing. No signing credentials were created/imported, no notarization submission was made, and no release was published.

Formatting, pinned Go lint, race tests, vet, native build, workflow lint, Makefile lint, shell syntax checks, archive inventory regression tests, documentation checks, and complete release packaging passed. Synthetic tests also cover registration failure without attempting delivery.

### Doctor integration with the branded helper

On 2026-10-25 (PDT), `doctor --notify` was changed to invoke `Bambu TUI Notify.app/Contents/MacOS/bambu-notify` beside the resolved CLI executable. The AppleScript path below is superseded. On the same macOS host, the rebuilt CLI (version 0.1.15, base commit `3c25826bae52`, with uncommitted notification changes) submitted a silent test through the branded helper and returned exit 0. The user subsequently confirmed that both `./bin/bambu-tui doctor --notify` and `./bin/bambu-tui doctor --notify --sound` worked. This records user-observed integrated notification delivery in silent and sound-requested modes, in addition to their earlier confirmation of the helper's correct logo. After receiving instructions to delay each command with `sleep 5 &&` and switch to another application, the user confirmed that both unfocused tests worked. A separately identified focused-terminal check has not been reported.

- [x] Confirm user-observed delivery through `doctor --notify`.
- [x] Confirm user-reported success of `doctor --notify --sound`.
- [x] Confirm integrated delivery with the terminal unfocused, in both silent and sound-requested modes.
- [ ] Confirm integrated delivery with the terminal explicitly focused.

Formatting, pinned lint, race tests (including PTY coverage), vet, build, and documentation checks passed. Synthetic tests cover adjacent helper discovery, paths with spaces, symlinked CLI discovery, missing/non-executable helper rejection, sound arguments, cancellation, and permission/timeout error guidance. A manual copied-binary check also confirmed that doctor reports a missing adjacent helper even when invoked from the source checkout; it does not search that current directory or fall back to AppleScript. At that integration-only stage, release archives did not yet include the helper; the packaging evidence above supersedes that limitation.

### Branded native helper

On 2026-10-25 (PDT), the local helper PoC built with Apple Swift 6.4 on macOS 27.0.1 (26A434), arm64, from the same development checkout described below. Plist validation, ad-hoc signature verification, Launch Services registration, ICNS decoding through `iconutil`, and the help/status commands succeeded in the Codex command environment. Initial authorization status was not determined. An explicit silent `--test` triggered the macOS permission UI; the user's screenshot confirms the Bambu TUI name and approved logo in that UI. The request returned "Notifications are not allowed for this application" (exit 4 in the initial build), and a subsequent status check reported denied. The helper now classifies that framework permission error as exit 3 with retry guidance.

The user's subsequent screenshot shows a separate Bambu TUI entry with the approved logo and "Sounds, Desktop" in notification settings. The helper then reported authorization granted with alerts and sound enabled; retrying the silent test returned exit 0 and native request acceptance. The user requested another test because they may have missed the banner. At that point, this confirmed independent app settings and accepted submission, while normal banner delivery remained unconfirmed.

The user subsequently ran the helper from zsh in a local terminal, saw successful command acceptance, and reported that nothing appeared. Inspection of macOS unified logs restricted to the PoC's bundle ID showed the native notification pipeline accepted the requests but Focus resolved them as suppressed with delayed delivery. NotificationCenter logged "muted by DND suppression: delay," stored the notification, and reported that it could not display while Notification Center was closed. This established observed Focus suppression for the helper. No Focus or permission settings were changed by the agent.

The user then confirmed that the helper notification did work and displayed the correct Bambu TUI logo, attributing the earlier missing banner to Focus mode. They also confirmed that `doctor --notify` showed the generic script-host logo, consistent with the separate AppleScript backend. This establishes user-observed branded helper delivery. The exact Focus change, terminal focus state, and audible behavior were not reported; the remaining checks retain those limits.

Local smoke checks also passed for no-argument/help launches, invalid arguments, SSH rejection without delivery, and byte-for-byte preservation of the approved PNG payloads in the ICNS container. The helper's test process exited after submission.

- [x] Confirm the Bambu TUI logo and app name in the permission UI through the user's screenshot.
- [x] Observe a real permission-denial error and status reporting, separately from command acceptance.
- [x] Confirm independent app notification settings and an accepted silent request after permission was allowed.
- [x] Observe Focus suppression of accepted helper requests in macOS logs after the user reported no visible banner.
- [x] Confirm the branded synthetic notification banner and correct logo through the user's report.
- [x] Confirm user-reported success of the helper's silent and sound-requested modes through the integrated doctor commands above.

The user's final report establishes helper delivery separately from the earlier permission UI. Distribution signing/notarization, installation, other macOS versions/architectures, headless behavior, and TUI integration remain untested. No printer or account was used.

### Original AppleScript probe (superseded)

On 2026-10-25 (PDT), `./bin/bambu-tui doctor --notify` returned exit status 0 from the Codex command environment on macOS 27.0.1 (26A434), arm64. The development build reported version 0.1.15 and base commit `3c25826bae52`, with the uncommitted notification probe changes. This establishes native AppleScript command acceptance only. No printer, firmware, account, or telemetry was involved.

The user subsequently confirmed that the notification worked and supplied a screenshot showing a banner titled Bambu TUI with the script host's icon. This confirms visible banner delivery on the user's setup. Terminal focus state and audible behavior were not reported, so the focused/unfocused and sound checks below remain open.

- [x] Submit a silent synthetic notification through the actual macOS command.
- [x] Confirm visible banner delivery through the user's screenshot and report.
- [ ] Confirm banner/Notification Center visibility with the terminal focused and unfocused in a normal local desktop terminal.
- [ ] Confirm sound-off and sound-requested behavior audibly in that environment.
- [ ] Observe disabled-permission and Focus suppression behavior. The command cannot confirm visual/audio delivery.

Synthetic tests cover sound request construction, command failure, cancellation, unsupported platforms/SSH, CLI flag validation, and independence from configuration and credential storage. Linux delivery is not implemented; headless delivery and automatic live-TUI notification integration remain unverified. This probe does not satisfy the complete notification environment validation milestone.

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
