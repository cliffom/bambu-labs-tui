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

## v0.4.6 — Simplified offline cards

[#145](https://github.com/cliffom/bambu-tui/issues/145) simplifies explicitly offline browser cards to printer identity, artwork, Offline status, and any attention warning. Dashed borders and dimmed grayscale images convey inactivity while last-known telemetry remains accessible in inspection. See [browser behavior](user/usage.md#local-web-dashboard) and [validation](development/validation.md#offline-browser-cards-145).

## v0.4.5 — Browser theme parity

[#141](https://github.com/cliffom/bambu-tui/issues/141) adds the host's effective TUI theme to both browser hosts, plus an independent Settings selector using the same bundled themes. Custom-file colors and launch overrides are available through Follow host, while browser choices leave TUI configuration unchanged. See [browser appearance](user/usage.md#local-web-dashboard) and [verification](development/validation.md#browser-theme-parity-141).

## v0.4.4 — Clearer browser dashboard

[#134](https://github.com/cliffom/bambu-tui/issues/134) consolidates browser filtering into one toolbar, removes duplicate sidebar/status/heading content, and adds a current online Idle / finished view. Static whole-farm totals retain last-reported state semantics; cards preserve attention and unknown/last-known distinctions. See [controls](user/usage.md#local-web-dashboard) and [validation evidence](development/validation.md#browser-dashboard-cleanup-134).

## v0.4.3 — Pinned browser status bar

[#132](https://github.com/cliffom/bambu-tui/issues/132) keeps the browser status line at the viewport bottom with a full-width divider matching the sidebar. The farm workspace scrolls above it without covering the result count or cards, and the bar reflows with narrow screens and enlarged text. See [dashboard behavior](user/usage.md#local-web-dashboard) and [validation evidence and limits](development/validation.md#pinned-browser-status-bar-132).

## v0.4.2 — Live print job names

[#129](https://github.com/cliffom/bambu-tui/issues/129) preserves newly reported job names and estimates through partial startup reports, delayed task/subtask identities, and unknown zero-valued IDs. Previous-job metadata is cleared at the observed boundary, while the table and details update without moving selection or focus. See [display behavior](user/farm-view.md) and [validation evidence and limits](development/validation.md#live-print-job-names-129).

## v0.4.1 — Notification history and shared session boundaries

[#105](https://github.com/cliffom/bambu-tui/issues/105) moves retired job identities into exact bounded-read archives without expiry or replay, exposes live capacity, and provides explicit offline job compaction with documented unfinished-completion consequences. [#106](https://github.com/cliffom/bambu-tui/issues/106) separates session display/decision contracts from HTTP/JSON, centralizes local occurrence comparisons, and shares account persistence initialization between hosts. See [retention and recovery](user/alerts.md#notification-history-and-capacity) and [validation evidence and gaps](development/validation.md#notification-history-and-session-boundaries-105-106).

## v0.4.0 — Terminal safety and persistence durability

[#119](https://github.com/cliffom/bambu-tui/issues/119) sanitizes unrecognized printer-state labels before terminal rendering. [#120](https://github.com/cliffom/bambu-tui/issues/120) commits replacement-directory ancestry before acknowledging saves and notification reservations, preserving conservative behavior after ambiguous storage errors. See [validation evidence and limits](development/validation.md#terminal-safety-and-persistence-durability-119-120). Notification retention (#105) and shared session boundaries (#106) follow in v0.4.1.

## v0.3.11 — Explicit save recovery

[#107](https://github.com/cliffom/bambu-tui/issues/107) adds `r retry saves` in Status / Feedback for eligible failed notice/snooze writes, shared session retry policy for browser notices, and separate durability status. Retries preserve local decisions and occurrence/delivery identities, protect failed-load stores, and participate in bounded shutdown. This recovery follow-up targets v0.3.x; notification retention (#105) and session-boundary refactoring (#106) follow in v0.4.1. See [saved decisions](user/usage.md#saved-notice-decisions) and [validation](development/validation.md#explicit-persistence-retry-107).

## v0.3.10 — Active printers first

[#116](https://github.com/cliffom/bambu-tui/issues/116) groups online non-idle printers before reported idle/finished printers, with offline devices last, before applying the saved sort. Unknown and last-known telemetry retain their distinctions, and selection/focus/scroll follow printer identity during updates. See [sorting](user/farm-view.md#sorting) and [validation](development/validation.md#active-printer-sorting-116).

## v0.3.9 — TUI server inspection

[#113](https://github.com/cliffom/bambu-tui/issues/113) adds live read-only Server Status on `W` and in Config > Web Server, with listener/monitor states, separate session/listener uptime, bounded diagnostics, storage warnings, and the existing grouped client ledger. It uses shared dialog routing and immutable observations while preserving Config, feedback return paths, ongoing monitoring/persistence, and localhost boundaries. See [usage](user/usage.md#tui-server-status) and [validation evidence and gaps](development/validation.md#tui-server-status-113). This completes the next v0.3.x dashboard follow-up before v0.4.x feature work.

## v0.3.8 — Foreground server status

[#110](https://github.com/cliffom/bambu-tui/issues/110) adds a compact colored foreground status display, bounded diagnostics, and a session-only grouped HTTP client ledger to both serve modes. Noninteractive output remains plain, and localhost/read-only monitoring, persistence, and notification behavior are preserved. See [usage](user/usage.md#foreground-server-status) and [validation evidence and limits](development/validation.md#foreground-serve-status-110).

## v0.3.7 — Browser system notifications

[#91](https://github.com/cliffom/bambu-tui/issues/91) delivers browser notifications in the [v0.3.x dashboard follow-ups](https://github.com/cliffom/bambu-tui/milestone/12): explicit browser enable/test/disable controls, Go-generated incident/completion events, separate durable browser claims across tabs/reloads/host restarts, and documented permission/lifecycle limits. Both server modes retain localhost binding and the display-data allowlist. Browser delivery is independent from native macOS opt-in, and closing a system notification never dismisses a notice or establishes physical resolution. See [browser notification policy](user/alerts.md#browser-system-notifications), [architecture](development/architecture.md#browser-notification-worker), and [validation evidence and gaps](development/validation.md#browser-system-notifications-91).

## v0.3.6 — Saved-decision integrity

[#104](https://github.com/cliffom/bambu-tui/issues/104) introduces exclusive account/region ownership before cloud TUI/foreground server startup, protecting notice decisions and snoozes from concurrent stale snapshots. A second host exits with stop-and-retry guidance; all sharing binaries must use v0.3.6 or newer. Browser tabs still share their host, and external file edits are not synchronized. See [usage](user/usage.md#saved-notice-decisions) and [validation](development/validation.md#account-persistence-ownership-104).

## v0.3.5 — Foreground telemetry recovery

[#103](https://github.com/cliffom/bambu-tui/issues/103) keeps foreground serving alive after individual malformed/over-limit reports, preserving accepted state and local decisions with bounded private diagnostics. Authentication rejection and monitor/listener failure remain terminal. See [validation](development/validation.md#foreground-telemetry-rejection-103) for synthetic evidence and hardware gaps.

## v0.3.4 — Cloud-backed terminal-free server

[#98](https://github.com/cliffom/bambu-tui/issues/98) adds foreground `serve` with existing saved credentials, shared cloud discovery/monitoring, account/region-scoped local decisions and snoozes, typed authentication rejection, transient reconnection, and bounded shutdown without prompts. Native notifications remain disabled and localhost/read-only boundaries remain intact. This delivers phase 3 of #90; live account/hardware and environment checks remain open. See [usage](user/usage.md#terminal-free-cloud-server) and [validation](development/validation.md#cloud-backed-terminal-free-server-98).

## v0.3.3 — Terminal-free demo server

[#97](https://github.com/cliffom/bambu-tui/issues/97) adds foreground `serve --demo` with optional `--config`, localhost HTTP startup independent of saved enablement, shared synthetic monitoring and session-only browser notice decisions, and bounded signal shutdown. Native desktop delivery is disabled and real-account stores remain isolated. This completes phase 2 of #90; v0.3.4 delivers cloud-backed `serve` under #98. See [usage](user/usage.md#terminal-free-demo-server) and [validation](development/validation.md#terminal-free-demo-server-97).

## v0.3.2 — Shared monitoring session

[#96](https://github.com/cliffom/bambu-tui/issues/96) establishes `internal/session` as the shared owner for telemetry/reconciliation, local browser decisions, immutable snapshots, and persistence, used by the existing TUI. It is phase 1 of [#90](https://github.com/cliffom/bambu-tui/issues/90). Synthetic terminal-free demo serving (#97) is delivered in v0.3.3 and cloud-backed serving ([#98](https://github.com/cliffom/bambu-tui/issues/98)) in v0.3.4. #90 stays open until all phases and the overall acceptance criteria are complete. See [architecture](development/architecture.md) and the [validation record](development/validation.md#shared-monitoring-session-96).

## v0.3.1 — Browser notice dismissal

[#92](https://github.com/cliffom/bambu-tui/issues/92) adds protected per-occurrence browser hiding through the running reducer, shared saved decisions and save retry, and synchronization with the TUI and other visible tabs. It preserves physical-resolution and telemetry distinctions. See [usage](user/usage.md#local-web-dashboard) and the [validation record](development/validation.md#browser-notice-dismissal-92).

## v0.3.0 — Local web dashboard

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

## LAN monitoring

Read-only LAN monitoring is not planned; [#12](https://github.com/cliffom/bambu-tui/issues/12) was closed as not planned.

## Scope and validation

Printer control, job submission, camera feeds, and detailed filament inventory remain outside near-term scope. Keep unknown data distinct from confirmed state, and keep local acknowledgements, dismissals, snoozing, and physical fault resolution separate.

Each milestone needs meaningful regression coverage, keyboard/terminal-size validation where relevant, current documentation, and an accurate record of hardware limitations. Keep ongoing checks in the validation record rather than creating version-specific readiness documents.
