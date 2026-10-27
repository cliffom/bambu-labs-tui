# Roadmap

Bambu TUI aims to be a dependable, read-only monitor for a Bambu Lab print farm. Prioritize trustworthy state, useful alerts, and quick navigation before adding integrations. Milestones define scope and release criteria rather than delivery dates; linked issues track implementation details.

[Documentation index](README.md) · [Changelog](CHANGELOG.md) · [Open issues](https://github.com/cliffom/bambu-tui/issues)

## Delivered

The cloud-monitoring foundation includes readable faults and action-needed dialogs, firmware reporting, an AMS/filament browser, persisted notice decisions, per-printer interruption preferences and snoozes, farm summaries/filtering/sorting, and local allowlisted diagnostic exports. The daily-farm-usability work in [#7](https://github.com/cliffom/bambu-tui/issues/7), [#8](https://github.com/cliffom/bambu-tui/issues/8), [#9](https://github.com/cliffom/bambu-tui/issues/9), and [#10](https://github.com/cliffom/bambu-tui/issues/10) is complete.

Releases through **v0.1.13** also include asynchronous persistence, centralized dialog routing, separated telemetry interpretation, reconnect/worker fixes, PTY smoke coverage, a ten-minute idle/finished telemetry grace period, clear-all-notices controls, named domain/protocol values, offline printers sorted last with distinct labels, bounded telemetry retention, and hardened dependency checks and release workflows. See the changelog for individual releases; the completed v0.1.1–v0.1.5 cleanup work is no longer an implementation queue.

Displayed telemetry has been validated on H2D and X2D, and live router-restart recovery has been observed. These results have specific limits: saved real-account sessions, notice/snooze persistence, pause/completion, firmware comparison, and other operational checks remain open in the [validation record](development/validation.md). A shipped feature or closed issue does not establish that those checks passed.

## Next priorities

1. **Record current operational evidence ([#38](https://github.com/cliffom/bambu-tui/issues/38)).** Run the remaining applicable live checks on the current release: saved login/restart/logout, notice and snooze persistence/recurrence, pause/completion, firmware/telemetry comparisons, and keyboard/modal behavior during updates. Record host OS/terminal, application version, model/firmware, and actual results in the validation record. Preserve the existing reconnect evidence and explicitly record unavailable scenarios. This follows up the closed original validation issue #6.
2. **Measure large-farm behavior ([#39](https://github.com/cliffom/bambu-tui/issues/39)).** Benchmark report/tick updates, navigation, dialogs, and rendering with representative synthetic farm sizes and notice/history loads. Record time and allocations, then profile material costs before proposing optimization. No performance defect is assumed, and fragile wall-clock CI thresholds are outside scope.
3. **Validate v0.2.0 macOS distribution and live delivery ([#23](https://github.com/cliffom/bambu-tui/issues/23)).** The live notification worker, identity persistence, Config controls, grouped intervention delivery, and signed/notarized release workflow are implemented. Complete clean-Mac download/first-run and remaining hardware checks, then extend preferences/activity as tracked below. Linux notification delivery is deferred beyond v0.2.x.
4. **Investigate LAN monitoring for v0.3.0.** Establish hardware-backed feasibility before promising compatibility or delivery.

## Optional local web dashboard

[#88](https://github.com/cliffom/bambu-tui/issues/88) adds a default-off Web Server section in Config. Enabling starts a read-only dashboard at a configurable `http://127.0.0.1:<port>` (default 8080, with a visible error and no fallback if occupied), sharing the running TUI's monitoring state. The browser includes farm summaries, filtering/search, printer inspection, AMS/temperatures, and reported conditions with explicit unknown/last-known qualifiers. Disabling or exiting stops the listener. This initial scope is localhost-only without authentication; remote access and standalone/headless mode remain outside scope. See [usage](user/usage.md#local-web-dashboard) and the [validation record](development/validation.md) for evidence and limits.

## v0.2.11 — About Bambu TUI

[#85](https://github.com/cliffom/bambu-tui/issues/85) adds read-only About on `A`: app name/version, platform, project URL, branding, and attribution without field labels, followed by the effective theme source and an unlabeled grid of unique colors sorted by hue. About displays no account information. The dialog preserves focus and telemetry updates. Synthetic layout evidence and remaining terminal gaps are recorded in the [validation record](development/validation.md).

## v0.2.10 — status message prominence and customizable themes

[#82](https://github.com/cliffom/bambu-tui/issues/82) gives fresh status messages explicit informational/success/warning/error roles and defines a semantic default palette with instance-owned styles. Versioned JSON files can now customize any palette role through the startup `theme_file` setting or a launch-only `--theme` override (also available with `make run THEME=…`), with complete default/Gruvbox Dark/Hotdog/C64/IBM 3270/DeskMate II examples and body foreground/background support. Bundled JSON files are discovered without per-theme Go registration; personal JSON themes load at runtime with no rebuild. Feedback lifetimes, dialog state, and reported filament colors remain independent of presentation. Actual layout evidence and limits belong in the [validation record](development/validation.md).

## v0.2.9 — consistent status feedback

[#79](https://github.com/cliffom/bambu-tui/issues/79) adds contextual compact controls, scrollable Help and Status / Feedback, persistent error details, protected Config editing instructions, and confirmation-scoped history clearing. Footer exceptions and actual validation evidence are recorded in the [validation record](development/validation.md).

## v0.2.8 — clearer action-needed alerts

[#77](https://github.com/cliffom/bambu-tui/issues/77) improves fault/intervention hierarchy, compact local-action guidance, and the shared footer. Each independent occurrence shows its first app-observed timestamp and a live relative age, preserving review lifetimes and read-only behavior. Synthetic coverage does not establish live hardware fault timing.

## v0.2.5 — notification ownership and recovery

[#18](https://github.com/cliffom/bambu-tui/issues/18) adds one delivery owner per account/region, visible standby sessions, crash/exit takeover from fresh UI state, and explicit paused-worker rechecks with `r`. Accepted, failed, and uncertain attempts remain deduplicated; corrupt/full state is protected from automatic reset or eviction. Long-term capacity retention remains a separate policy decision.

## Notification activity

[#20](https://github.com/cliffom/bambu-tui/issues/20) adds a read-only `N` activity view with the last 100 session decisions, per-printer incident subjects, outcome/reason details, repeat coalescing, and grouped-request membership. It distinguishes macOS acceptance from confirmed presentation and preserves diagnostics when storage fails after submission. Ownership and worker recovery are covered by v0.2.5 above; quiet scheduling and timed grouping are implemented below.

## v0.2.2 — complete configuration controls

Config now exposes General, Alerts, and Printers sections: region/restart guidance, a validated live percentage threshold, all nine alert defaults, per-printer On/Off/Inherit/effective values, and confirmed reset. Edits merge into valid TOML asynchronously with private atomic writes and visible failures. This implements [#65](https://github.com/cliffom/bambu-tui/issues/65) and the notification controls in [#21](https://github.com/cliffom/bambu-tui/issues/21); notification activity and quiet scheduling are described in their own sections.

## v0.2.1 — distribution polish

[#66](https://github.com/cliffom/bambu-tui/issues/66) trims binary archives to runtime files, concise installation guidance, configuration examples, and licenses, and renames consumer-facing macOS downloads from `darwin` to `macos`.

## v0.2.0 — macOS alerts outside the terminal

The v0.2.x notification scope is macOS desktop delivery only. Linux notification delivery is deferred and is not a v0.2.x release blocker. Cloud monitoring remains available on Linux. SSH and headless notification delivery are unsupported; validate clear unsupported-environment behavior rather than promising remote delivery.

[Notification tracking issue #11](https://github.com/cliffom/bambu-tui/issues/11) covers opt-in intervention-required and telemetry-confirmed completion notifications with configurable sound. v0.2.0 connects the live-session worker to the branded helper, reuses verified classifications/reviews/snoozes, and persists incident/job identities to avoid duplicate attempts after reconnect or restart. Missing or ambiguous telemetry cannot establish resolution or completion. The identity foundation and explicit test workflow are tracked in [#18](https://github.com/cliffom/bambu-tui/issues/18) and [#19](https://github.com/cliffom/bambu-tui/issues/19).

Config exposes the nine boolean alert defaults and general settings through General, Alerts, and Printers, including per-printer inheritance/reset and Test Notification. Quiet schedules and burst windows are configured in TOML. The alert dialog shows effective settings and session activity. Co-reported intervention notices group per printer by default. [#22](https://github.com/cliffom/bambu-tui/issues/22) adds opt-in timezone/weekday quiet hours, fixed burst windows across reports/printers/completions, and bounded automatic submission rates. Quiet occurrences are consumed without an exit summary or replay; notice reviews and modal policy stay independent.

macOS release archives include the universal helper beside the CLI. Tag/manual release builds sign and notarize both programs; source builds and ordinary PR/main CI use ad-hoc candidates. Local and GitHub-hosted signing, Apple acceptance, and helper stapling have passed. Clean-Mac browser-download/first-run, Intel runtime, real-printer notification/job lifecycle, and broader environment validation remain open in the [validation record](development/validation.md). Successful submission or synthetic tests do not establish those outcomes.

| Follow-up work | Issue |
| --- | --- |
| Complete clean-install, hardware, and unsupported-environment validation | [#23](https://github.com/cliffom/bambu-tui/issues/23) |

Notifications apply only while the TUI session is running. Quitting ends monitoring and notification delivery; v0.2.x does not introduce a background service. Persisted occurrence/job identity still prevents duplicate delivery when the TUI restarts.

The release gate is configurable, testable, deduplicated macOS desktop notifications with visible failure/suppression outcomes and verified shutdown of monitoring and delivery when the TUI exits. Saved reviews and snoozing must remain consistent with delivery. [Saved farm views (#24)](https://github.com/cliffom/bambu-tui/issues/24) restore the last filter/sort from the active configuration, with safe per-field fallbacks and a dashboard `0` reset to All / Name. View changes reuse asynchronous configuration saves without persisting telemetry or changing reviews/snoozes.

## v0.3.0 — LAN monitoring

[Investigate and prototype read-only LAN monitoring (#12)](https://github.com/cliffom/bambu-tui/issues/12). Verify protocol/authentication requirements and feasibility on available hardware. Plan secure credential storage, mixed cloud/LAN farms, source indicators, identity matching, partial reports, and reconnect handling.

Ship only documented, hardware-tested model/firmware combinations with consistent cloud/LAN state. Document unsupported combinations and defer them rather than claiming compatibility.

## Scope and validation

Printer control, job submission, camera feeds, and detailed filament inventory remain outside near-term scope. Keep unknown data distinct from confirmed state, and keep local acknowledgements, dismissals, snoozing, and physical fault resolution separate.

Each milestone needs meaningful regression coverage, keyboard/terminal-size validation where relevant, current documentation, and an accurate record of hardware limitations. Keep ongoing checks in the validation record rather than creating version-specific readiness documents.
