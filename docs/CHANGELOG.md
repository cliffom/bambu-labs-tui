# Changelog

## 0.2.12 — Palette label correction

- Correct “Theme Pallet” to “Theme Palette” in About and the accompanying documentation.

## 0.2.11 — About Bambu TUI

- Add read-only About on `A`, with app name/version, platform, project URL, branding, and “Lovingly crafted by Michael Clifford” attribution without field labels, followed by the effective bundled/custom theme source. About displays no account information.
- Show a responsive, unlabeled grid of unique explicit theme colors sorted by hue. Omit unknown inherited terminal colors, and preserve swatch shapes without color. Preserve dashboard focus/selection, ongoing telemetry, status return, and below-minimum restoration.

## 0.2.10 — Customizable themes

- Make fresh status notices bright and bold, with distinct informational, local-success, warning, and error styling. Preserve feedback priority and lifetime, and keep local success and native notification acceptance separate from physical fault resolution and confirmed presentation.
- Add six embedded themes: Default, Gruvbox Dark, Hotdog, C64, IBM 3270, and DeskMate II. Select a bundled theme or custom JSON file with `--theme`, `make run THEME=…`, or `make demo THEME=…`; save a default with `theme_file` in the configuration. Launch overrides do not change saved preferences.
- Load custom theme files at startup without rebuilding. Support partial version 1 JSON color overrides, validate malformed files before authentication, and paint the complete body, dialogs, selections, and footer. Preserve readable offline labels and activity outcomes on selected rows, no-color meaning, and printer-reported filament swatches.

## 0.2.9 — Consistent status feedback

- Add scrollable Help and a read-only Status / Feedback view on `Ctrl+E`, with complete error messages and recovery guidance. Return to the same dialog, input, confirmation, selection, and scroll while telemetry and asynchronous work continue.
- Keep footer controls specific to the focused panel and enabled actions. Summarize overlapping failures without hiding essential controls, retain late export/browser failures until recovery, and preserve local action feedback through background notification activity. Keep the slate background continuous across messages, dot separators, hotkeys, and padding.
- Protect Config apply/reset/cancel instructions from retained notification-test feedback, while keeping pending tests responsive during status inspection. Show notification-worker recheck controls only when available; native acceptance remains distinct from visible delivery.
- Capture event IDs when opening Clear history so confirmation preserves events arriving afterward. Local notice decisions and read-only monitoring remain independent of physical fault resolution.

## 0.2.8 — Clearer action-needed alerts

- Emphasize printer/state and independent fault summaries with intervention guidance directly below each fault. Move review status, codes, preferences, and version into secondary text; expose full troubleshooting URLs through the notice inspector and keep browser access with `o`. Use the shared single-row footer with explicit acknowledgement, hide, snooze, and close controls.
- Show each occurrence’s first app-observed time with timezone/UTC offset and a live relative age. Preserve occurrence times, focus, and scroll through updates; label unavailable times, future observations, and replaced occurrences explicitly. Local reviews and elapsed time never establish physical resolution.

## 0.2.7 — Saved farm views

- Save and restore the last farm filter and sort order in the active TOML configuration, across accounts/regions and demo sessions using that path. Use stable named values and fall back independently to All / Name for missing, unknown, or non-string choices. Startup restoration does not write the configuration.
- Add dashboard `0` to reset and save All printers / Name sorting, with guidance in the list status, empty view, and help. Preserve live identity-based selection, dialog ownership, and below-minimum input guards.
- Reuse asynchronous Config saves, independent field merging, coalescing, visible failures, explicit retry, and bounded shutdown. Persist only view choices without cached telemetry, selected identities, or changes to notice reviews and snoozes.

## 0.2.6 — Quiet hours and burst grouping

- Add opt-in quiet hours configured in TOML with an explicit timezone, weekday selection, overnight periods, and documented daylight-saving behavior. Suppress automatic desktop banners and sound without changing modal policy, telemetry, or local notice reviews. There is no urgent bypass, exit summary, or backlog replay; otherwise eligible quiet occurrences are durably consumed. Independent snooze/category/desktop exclusions remain pending and retain their activity reasons.
- Add an optional fixed burst window across reports, printers, and completions while preserving every member's identity, reservation, result, and activity row. Limit automatic submissions to twelve attempts per rolling minute per running owner, recheck waiting events against fresh snapshots, and retain the no-automatic-retry policy. Schedule/window edits require restart; Config saves preserve them.
- Add a saved `quiet_suppressed` attempt state. Older readers fail closed after this state is written; use this version or newer rather than deleting delivery history to downgrade.

## 0.2.5 — Notification ownership and recovery

- Elect one automatic notification owner per account/region using a private OS file lock. Show ownership and standby status without interrupting secondary monitoring; take over from a fresh UI snapshot after owner exit/crash or desktop opt-out. Muted sessions release delivery ownership while preserving identity observation when available.
- Add `r` in Notification activity to recheck paused delivery after fixing its cause. Drain obsolete frames and require fresh telemetry/preferences before resuming. Preserve accepted, failed, and uncertain attempts, leave unreadable state untouched, and retain capacity limits without resetting history. Hold ownership until any canceled sender has returned.

## 0.2.4 — Consolidated license notices

- Consolidate Go and dependency license notices into one attributed `THIRD_PARTY_NOTICES.txt` in binary archives, replacing the nested `third_party/` tree while preserving all notice text.

## 0.2.3 — Notification activity

- Add a keyboard-accessible Notification activity view (`N`) with the last 100 session decisions, stable selection, incident/channel/outcome/reason details, repeat coalescing, and grouped-request membership. Explain reviews, preferences, snoozes, prior attempts, completion baselines, native failures, and storage pauses. Keep macOS acceptance separate from confirmed presentation and preserve accepted outcomes when saving their result fails. Activity is memory-only and does not change delivery identities or retry policy.

## 0.2.2 — Complete configuration controls

- Expand Config into General, Alerts, and Printers sections with all TOML settings, a validated percentage editor, all nine global alert defaults, and per-printer On/Off/Inherit/effective-value controls plus confirmed reset. Apply alert/threshold edits immediately and save region edits for restart/login. Merge edited fields into valid TOML asynchronously while preserving untouched settings, private atomic writes, silent success, and red failures. Keep Test Notification and footer feedback.

## 0.2.1 — Leaner release downloads

- Trim binary release archives to the CLI, macOS notification helper, concise installation README, configuration example, and license notices. Keep full documentation online. Rename macOS download filenames from `darwin` to `macos`, retaining Go build targets and signed/notarized packaging verification.

## 0.2.0 — macOS desktop notifications

- Add opt-in macOS desktop notifications for intervention-required incidents and telemetry-confirmed print completion while the TUI session is running. Show the triggering printer name and Bambu TUI logo, and group co-reported intervention notices into one notification per printer.
- Add Config (`,`) with immediate desktop notification, sound, and completion toggles saved to the active configuration file. Preserve printer overrides and unrelated settings; save silently and show persistent red failures with retry controls.
- Add Test Notification in Config and `doctor --notify [--sound]` for explicit, repeatable tests without an account or printer. Request macOS permission explicitly and show delivery acceptance separately from visible banners or sound. Config test feedback appears after helper text, dims, and expires.
- Honor global/per-printer categories, saved notice reviews, and snoozes. Persist account-scoped incident/job identities and delivery reservations to avoid repeat attempts after reconnect or restart; unknown telemetry preserves identity and ambiguous job IDs suppress completion.
- Keep monitoring responsive with delivery outside the UI reducer, current-policy checks, cancellation of stale requests, and bounded shutdown. Show delivery failures and session activity; no background service or future notification schedule is installed.
- Include a universal notification helper in macOS release archives. Developer ID-sign and notarize both macOS CLIs and the helper, staple the helper ticket, and verify extracted archives. Source checkouts build the helper separately with `make notify-helper`; local development and ordinary PR/main CI use ad-hoc candidates.
- Sign and notarize tag/manual release builds using repository secrets and a temporary runner Keychain, with credential cleanup and no unsigned fallback. Manual runs produce verified artifacts without creating a release; version-tag runs prepare a draft for review. Forward `RUN_ARGS` through `make demo` for synthetic testing with a custom config.

Desktop notifications require a local macOS desktop and an active TUI session. Linux cloud monitoring remains supported; Linux, SSH, and headless desktop notification delivery are unsupported. See the [validation record](development/validation.md) for observed results and remaining clean-Mac, Intel-runtime, and real-printer checks.

## 0.1.15 — Clearer footer navigation

- Give dashboard and full-screen notice footers a full-width slate background with a one-column inset to separate them from monitoring content.
- Emphasize dashboard hotkeys in bold with brighter text and show up/down arrows in navigation hints. Keep both arrow and `j` / `k` navigation available, along with amber dashboard warnings and readable controls without color.

## 0.1.14 — Cleaner section headers

- Remove numeric prefixes from the Printers, Printer Details, and Notices section headers in compact and wide layouts, retaining the existing focus marker and keyboard navigation.

## 0.1.13 — Security hardening

- Bound cumulative telemetry retention and report complexity. Reject over-limit reports atomically, preserving last accepted telemetry, firmware, freshness, faults, and local notice decisions; retain partial-report and explicit-empty semantics within the limits.
- Require Go 1.27.3 or newer for source builds, update x/sys to v0.44.0, and add pinned vulnerability scans to macOS/Linux CI and tag release builds.
- Pin workflow actions to commit SHAs, disable persisted checkout credentials, and separate read-only release builds from draft publishing with checksum verification.
- Consolidate documentation under `docs/` and enforce documentation placement, navigation, and release-archive integrity checks; document UI and documentation direction for contributors.

## 0.1.12 — Offline printer visibility

- Group online printers before offline printers in every farm view sort, keeping the chosen sort within each group and selection attached to the printer during status changes.
- Color offline printer rows amber, including the selected row, and show only the printer name and Offline link status with blank telemetry columns. Keep an explicit Offline label in compact layouts.

## 0.1.11 — Named domain and protocol values

- Replace embedded printer states, severities, alert categories, condition identities, telemetry keys, protocol fields, and timing/size policies with named definitions in their owning packages.
- Use named notice spans, shared table-column records, cloud endpoint/request records, and fault-action catalogue records to clarify field meanings and keep consumers consistent.
- Preserve wire and saved formats, partial/unknown telemetry handling, keyboard controls, timing thresholds, and displayed output.

## 0.1.10 — Restore navigation after clearing notices

- Return focus to Printers after `X` clears the Notices panel or finds it already empty, so arrow keys and `j/k` immediately navigate printers. Keep focus in Notices when newer entries remain.

- Replace numeric dashboard panel indexes with typed constants throughout focus handling, Tab cycling, rendering, and tests.

## 0.1.9 — Clear all notices

- Add `X` in the Notices panel to clear all notices across the farm after confirmation, saving local dismissals and clearing captured history while preserving newly arriving occurrences and events.

## 0.1.8 — Quieter idle telemetry notices

- Allow ten minutes between print telemetry reports for idle and finished printers before marking them stale; retain two minutes for printing and all other states. Connection labels, unavailable filtering, and stale notices share the same policy; offline and cloud-disconnected warnings remain immediate.

## 0.1.7 — Accurate idle stages and terminal shutdown

- Show Idle, Finished, or Failed stages from the overall job state instead of stale printing stages or negative stage codes; clear the previous stage when a new job starts.
- Capitalize Idle consistently in printer status displays.
- Add PTY smoke tests on Linux/macOS for real keyboard input, modal quit ownership, resizing, alternate-screen/cursor cleanup, terminal-mode restoration, and bounded normal/interrupted/panic exits.
- Use the CLI signal context as the sole owner of SIGINT/SIGTERM and request graceful terminal shutdown on cancellation, preventing competing signal handlers and input-reader closure races.

## 0.1.6 — Exit handling and resize input

- Preserve unexpected Bubble Tea failures during dashboard cleanup, including recovered panics, while retaining intentional cancellation and monitor shutdown behavior.
- Block hidden actions and confirmations below the minimum terminal size; preserve Escape, dashboard quit, compact shutdown controls, and asynchronous updates.

## 0.1.5 — Internal API cleanup

- Use a named monitor event-kind type and constants consistently across live/demo producers, UI dispatch, and tests.
- Share private atomic snapshot replacement between the notice and snooze stores while preserving schemas, validation, account scoping, and asynchronous persistence behavior.
- Return a concrete model from the private UI reducer while keeping the public Bubble Tea adapter unchanged.

## 0.1.4 — Responsive monitoring and reconnects

- Reset reconnect backoff after two minutes of monitoring following successful subscription and initial refresh; retain exponential backoff and jitter for short failures.
- Move periodic discovery and serialized outbound requests off the report loop. Coalesce pending refreshes, prioritize due keepalives, bound write batches to ten seconds, and cancel/join session workers on exit.

## 0.1.3 — Telemetry interpretation boundaries

- Separate report decoding, partial merging, and normalization from farm lifecycle, condition reconciliation, and history within the farm package.
- Reconcile saved reviews against explicit condition-family and slot knowledge rather than raw protocol fields. Preserve omitted/empty/unknown semantics, job and replacement-spool estimate invalidation, and allowlisted diagnostic snapshots.

## 0.1.2 — Explicit dialog ownership

- Centralize dialog focus, keyboard routing, rendering, synchronization, and open/close transitions around one active owner. Preserve existing dialog controls and confirmations.
- Make the attention-to-snooze return path explicit, keep suspended attention current during telemetry and resizing, and defer automatic attention while another dialog or shutdown screen owns focus.

## 0.1.1 — Responsive persistence

- Move notice and snooze saves into background commands with immutable snapshots, serialized writes per store, and coalesced pending changes. Keep failures visible without continuous retries.
- Wait up to five seconds for saves on normal quit; show explicit exit/cancel controls on failure or timeout and report unconfirmed saves after interrupted exits.

## 0.1.0 — Daily farm usability

- Show `BAMBU TUI vX.Y.Z / CLOUD STATUS / FARM` in the dashboard header and the running version in dialog headings; shorten the clock on compact terminals to preserve version and connection visibility.
- Add explicit local diagnostic export (`D`, then confirmation) for one aliased printer. Include classified telemetry/types, bounded sanitized partial reports, version/platform/model/firmware metadata, link state, and fault codes; omit unknown fields and private/free-form data. Write private exclusive files without uploading.
- Add adversarial/fuzz, synthetic-fixture replay, file/error, confirmation, selection, and modal tests. Document export scope and a v0.1.0 readiness assessment: the planned feature scope is complete; unperformed live operational checks remain explicitly unverified in the validation record.

## 0.0.10 — Farm overview and navigation

- Add whole-farm printing, paused/failed, idle/finished, actionable-attention, and unavailable counts, independent of the visible printer filter.
- Add `F` filters for All, Attention, Printing, and Unavailable, plus `S` sorting by Name, Attention, State, and Finish. Preserve selected printer identity and details scroll during live reordering, and handle empty lists and explicit alert navigation.
- Sort only live, recent running-print ETAs as comparable; mark old estimates as last known, retain unknown/paused cases, and make duplicate-name ordering/notice counts deterministic by printer ID. Keep view controls visible at minimum terminal size.

## 0.0.9 — Alert preferences and snoozing

- Add validated global alert defaults and independent per-printer overrides for automatic modals, filament, extrusion, printer faults, failed prints, and unknown pauses. Preserve existing defaults and verified recovery/intervention classification.
- Add printer alert controls (`s`) with 15-minute, 1-hour, and 4-hour snoozes plus early resume. Persist deadlines per account/region, re-arm eligible alerts on expiry, and keep underlying notices, saved reviews, and other printers' queued alerts intact.
- Show effective policy and stable printer IDs; allow explicit `!` inspection while muted/snoozed. Keep demo snoozes in memory and defer sound delivery/preferences to #11.

## 0.0.8 — Manage notice decisions

- Add a notice decisions view (`n`) for inspecting dismissed and acknowledged notices, restoring individual dismissals, explicitly marking notices unreviewed, and resetting decisions for the current account and region with confirmation.
- Persist reversals immediately, preserve independent acknowledgement/dismissal state, and expose saved startup decisions as awaiting telemetry without creating active faults. Re-arm eligible alerts after the decisions view closes.
- Identify pending low-filament decisions by AMS unit and slot, including AMS HT, before telemetry arrives.
- Simplify getting started in the README, move detailed references into user/development guides, and include linked documentation and branding in release archives.

## 0.0.7 — AMS/filament browser

- Replace expanded filament inventory in printer details with a compact summary and an `m` AMS/filament browser. Navigate units and slots, inspect selected-slot details, and retain focus/selection through live updates.
- Defer action-needed alerts while the inventory browser is open; preserve close-before-quit behavior, empty/unknown/zero distinctions, external-spool limitations, and read-only monitoring.
- Show filament color chips beside material types in the inventory table, with an outline for unknown colors and aligned columns.
- Extend demo mode with multiple standard AMS units and an AMS HT for browser navigation.

## 0.0.6 — Firmware reporting

- Display the installed printer OTA firmware and reported update status in aligned printer details. Distinguish current, offered update, unknown, and last-known assessments without confusing AMS/controller versions with printer firmware.
- Query firmware through read-only `info.get_version` requests on connection, device discovery, and manual refresh. Preserve print telemetry freshness separately and add synthetic firmware examples to demo mode.
- Record user-confirmed displayed telemetry validation on H2D and X2D. Live firmware reporting and operational restart/reconnect/notice checks remain open.

## 0.0.5 — Clearer printer details

- Label printer details explicitly and align values, including status, connection, stage, finish time, temperatures, filament summaries, and faults. Wrap long values beneath the value column.
- Update repository links, badges, Go module/imports, and packaging references for the rename to `cliffom/bambu-tui`.
- Correct the logo shadow's lower-right pixel stepping in layered sources, icon exports, and the README hero.
- Add the linked project roadmap. Live hardware/restart validation remains open under the v0.0.6 reliability target.

## 0.0.4 — Action-needed alerts

- Require closing modal dialogs before quitting the TUI; `q` and `Ctrl+C` apply only from the dashboard.
- Add `make demo` to build and launch the simulated farm without login.
- Show an action-needed modal for verified intervention-required faults, failed prints, and reported pauses whose reason remains unknown after fault snapshots arrive.
- Group related notices by printer, collect incremental reports, defer alerts during other dialogs, and retain independent notice identities and saved reviews. Closing a prompt leaves notices unreviewed and suppresses repeat prompts for those occurrences during the session; `!` reopens eligible alerts.
- Add explicit local acknowledge/dismiss actions, notice/printer navigation, troubleshooting links, live resolution updates, and a simulated runout scenario in demo mode. Maintenance, low estimates, explicit user pauses, and automatic recovery messages do not independently trigger alerts.

## 0.0.3 — Expanded filament fault descriptions

- Add `make audit-faults` and a CI check to verify all bundled fault codes and generated HMS slot labels against pinned upstream catalogues.
- Expand verified filament fault descriptions for standard AMS, AMS HT, and external spools: distinguish runout, empty slots, recovery, feed/retract failures, possible tangles, and nozzle clogs.
- Preserve printer-provided messages, wire severity, troubleshooting codes, unknown-code fallbacks, and independent review lifetimes for paused state and individual faults.
- Add synthetic H2D/X2D fault fixtures and regression coverage for runout persistence, new fault identities, observed recurrence, and automatic recovery stages. Document pinned upstream sources and remaining catalogue gaps.

## 0.0.2 — Persistent notice decisions

- Persist local notice acknowledgements and dismissals across TUI restarts, isolated by Bambu account and region.
- Save decisions immediately using private files and atomic replacement. Keep restored decisions through discovery and partial telemetry until the relevant condition is reported.
- Remove saved decisions after confirmed resolution; changed cloud/print-state conditions and observed recurrences require review again. Dismissed faults stay out of startup history and printer details.
- Keep demo decisions and recent event history in memory. Show a persistent warning when saved decisions cannot be loaded or written, preserving unreadable files.
- Add the pixel-art logo and README hero, `make auth`/`make run` shortcuts, and checkmake validation locally and in CI.

Resolution and recurrence while the application is closed cannot be distinguished from the same ongoing condition when the printer supplies no occurrence identifier. Such conditions keep their saved decision until resolution is observed.

## 0.0.1 — Initial release candidate

- Read-only Bambu Cloud monitoring with password, email-code, authenticator, and token-import login.
- Farm overview, per-printer progress/ETA, dual-nozzle temperatures, AMS/AMS HT filament paths, and estimated remaining filament.
- Selectable notices, detailed HMS views, local acknowledgement and dismissal, and session event history.
- HMS severity classification, maintenance descriptions, and suppression of a known internal empty-message code.
- Credential-store diagnostics through `bambu-tui doctor`, checked save/readback, and explicit session-only fallback.
- Offline demo mode, configurable region/filament threshold, and responsive keyboard navigation.
- macOS/Linux binaries for arm64/amd64, checksums, third-party licenses, and build version/commit/date metadata.

### Validation and limitations

Live email-code login and printer telemetry have been observed in a user-provided screenshot. Local race tests and build checks use synthetic telemetry and fake services. A complete live smoke test is recorded separately in [validation record](development/validation.md).

Direct Google OAuth, LAN connections, printer control, and persistent event history are outside this release. HMS descriptions cover a curated subset; unknown codes remain visible with their code and troubleshooting link. Dismissal only hides conditions in the current TUI session.
