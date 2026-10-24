# Changelog

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

Live email-code login and printer telemetry have been observed in a user-provided screenshot. Local race tests and build checks use synthetic telemetry and fake services. A complete live smoke test is recorded separately in [docs/validation.md](docs/validation.md).

Direct Google OAuth, LAN connections, printer control, and persistent event history are outside this release. HMS descriptions cover a curated subset; unknown codes remain visible with their code and troubleshooting link. Dismissal only hides conditions in the current TUI session.
