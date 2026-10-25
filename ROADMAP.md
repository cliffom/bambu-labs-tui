# Roadmap

Bambu TUI aims to be a dependable, read-only monitor for a Bambu Lab print farm. Prioritize trustworthy state, useful alerts, and quick navigation before adding more integrations.

The released foundation through v0.1.0 includes cloud monitoring, readable fault descriptions, persisted review management, action-needed modals, firmware reporting, an AMS/filament browser, per-printer interruption preferences/snoozes, the farm overview/filter/sort controls, and local allowlisted diagnostic exports. The user has validated displayed telemetry on H2D and X2D; operational restart/reconnect/notice checks remain open in the [validation record](docs/validation.md). See the [v0.1.0 readiness assessment](docs/v0.1.0-readiness.md) for feature coverage and the remaining release evidence.

These milestones define intended scope and release criteria, not delivery dates. Linked issues track implementation details and acceptance criteria; adjust the roadmap as hardware results and user feedback establish priorities.

## Milestones

| Milestone | Focus | Release criteria |
| --- | --- | --- |
| **Next — Reliability** | Real-printer validation and fixes | Documented H2D/X2D smoke tests pass for the supported setup; reconnects and restarts preserve correct state and notice behavior. |
| **v0.1.0 — Daily farm usability** | Find troubled printers and manage alerts in the TUI | Operators can quickly locate printers needing attention, inspect and reverse saved notice decisions, and configure interruptions without editing internal state files. |
| **v0.2.0 — Alerts outside the terminal** | Optional desktop, completion, and sound notifications | Notifications are opt-in, configurable, deduplicated, and consistent with saved reviews and snoozing; process-lifecycle behavior is explicit. |
| **v0.3.0 — LAN monitoring** | Investigate and validate a read-only local transport | Only documented, hardware-tested model/firmware combinations ship, with secure credential storage and consistent cloud/LAN monitoring state. |

### Next — Reliability

The [validation record](docs/validation.md) retains the unperformed live checks; [#6](https://github.com/cliffom/bambu-tui/issues/6) records their original scope. Verify login persistence, H2D/X2D telemetry and filament paths, reported pause/recovery/completion, network reconnection, saved decisions, and action-needed modals during normal printer use. Record models, firmware, host environment, and actual results in the validation documentation. Fix findings and add sanitized regressions where useful.

Record evidence for the supported setup as checks are performed. Leave unperformed checks and unsupported combinations explicit.

- [Longer telemetry stale threshold for idle/finished printers (#45)](https://github.com/cliffom/bambu-tui/issues/45): **target v0.1.8** — allow ten minutes for reported `IDLE`/`FINISH` states while retaining two minutes for all other states. Connection labels, unavailable filtering, and stale notices share the policy; offline/cloud-disconnected warnings remain independent. The [validation record](docs/validation.md) documents the screenshot evidence, provisional threshold rationale, and remaining live cadence checks.

### v0.1.0 — Daily farm usability

- [View and restore persisted dismissed notices (#7)](https://github.com/cliffom/bambu-tui/issues/7): **released v0.0.8** — inspect hidden active conditions, reverse individual decisions, and explicitly reset saved reviews from the TUI.
- [Attention-focused farm summary, filtering, and sorting (#8)](https://github.com/cliffom/bambu-tui/issues/8): **released v0.0.10** — find troubled printers and upcoming completions while keeping selection stable during live updates.
- [Per-printer alert preferences and snoozing (#9)](https://github.com/cliffom/bambu-tui/issues/9): **released v0.0.9** — define global defaults, printer overrides, and temporary suppression distinct from acknowledgement or dismissal.
- [Privacy-safe diagnostic export (#10)](https://github.com/cliffom/bambu-tui/issues/10): **released v0.1.0** — explicitly export useful, sanitized troubleshooting artifacts for unknown codes and partial telemetry. Never upload automatically.

Sound preferences should be coordinated with the notification milestone; do not expose settings that have no working delivery backend. Underlying faults and connection state must remain visible when notifications are muted or snoozed.

### v0.2.0 — Alerts outside the terminal

Implement [optional desktop, completion, and sound notifications (#11)](https://github.com/cliffom/bambu-tui/issues/11). Reuse intervention classifications, saved review decisions, and per-printer preferences. Completion must come from reported job state, with occurrence/job identity preventing duplicates after reconnects or restarts.

Define whether notifications require the TUI to remain open. If delivery must continue after it closes, design monitor ownership, persistence, installation, and lifecycle before introducing an optional background service. An always-running service is not an implicit requirement for the first notification implementation.

### v0.3.0 — LAN monitoring

[Investigate and prototype read-only LAN monitoring (#12)](https://github.com/cliffom/bambu-tui/issues/12). Verify current protocol and authentication requirements, then establish feasibility on available hardware before promising model/firmware support. Plan secure credential storage, mixed cloud/LAN farms, source indicators, identity matching, partial reports, and reconnect handling.

LAN delivery depends on hardware-backed feasibility. If a required combination is unsupported, document the limitation and defer it rather than claiming compatibility.

## Recommended implementation order

1. Record remaining live validation and fix findings (original scope: [#6](https://github.com/cliffom/bambu-tui/issues/6)).
2. Make persisted notice decisions inspectable and reversible: [#7](https://github.com/cliffom/bambu-tui/issues/7).
3. Improve attention-focused farm navigation: [#8](https://github.com/cliffom/bambu-tui/issues/8).
4. Add per-printer interruption preferences and snoozing: [#9](https://github.com/cliffom/bambu-tui/issues/9).
5. Add a safe diagnostic export to support model/firmware troubleshooting: [#10](https://github.com/cliffom/bambu-tui/issues/10).
6. Extend notification delivery using those preferences: [#11](https://github.com/cliffom/bambu-tui/issues/11).
7. Deliver a LAN pilot only after feasibility and compatibility checks: [#12](https://github.com/cliffom/bambu-tui/issues/12).

## Scope boundaries

Printer control, job submission, camera feeds, and detailed filament inventory are outside the near-term milestones. Preserve the monitoring-only scope, distinguish unknown data from confirmed state, and keep acknowledgements, dismissals, snoozing, and physical fault resolution separate.

Each milestone should include meaningful regression coverage, keyboard and terminal-size validation where relevant, updated documentation, and an accurate record of remaining hardware limitations.
