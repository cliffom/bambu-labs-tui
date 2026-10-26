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
3. **Build optional macOS notifications for v0.2.0.** Delivery requires a running TUI session in a local macOS desktop environment. Connect the packaged helper and persisted identity foundation to live-session delivery before notification controls, grouping, and final macOS validation. Linux notification delivery is deferred beyond v0.2.x.
4. **Investigate LAN monitoring for v0.3.0.** Establish hardware-backed feasibility before promising compatibility or delivery.

## v0.2.0 — macOS alerts outside the terminal

The v0.2.x notification scope is macOS desktop delivery only. Linux notification delivery is deferred and is not a v0.2.x release blocker. Cloud monitoring remains available on Linux. SSH and headless notification delivery are unsupported; validate clear unsupported-environment behavior rather than promising remote delivery.

[Notification tracking issue #11](https://github.com/cliffom/bambu-tui/issues/11) covers opt-in desktop notifications for intervention-required incidents and telemetry-confirmed completion, with configurable sound. Reuse verified classifications, saved reviews, and per-printer preferences/snoozes. Completion must come from reported job state, with occurrence/job identity preventing duplicates after reconnects or restarts.

An initial macOS feasibility probe is available through [`doctor --notify`](user/alerts.md#desktop-notification-test-and-sound), with optional `--sound`. It invokes the adjacent [native helper](development/contributing.md#macos-notification-helper) using the Bambu TUI logo and app-specific notification permissions. The universal helper is bundled beside the CLI in macOS archives. Tag/manual release builds sign and notarize it; local and ordinary CI candidates remain ad-hoc signed. Native request acceptance does not establish visible/audible delivery or automatic TUI integration. Developer ID signing/notarization tooling is available as an opt-in release path; local and GitHub-hosted Apple acceptance/helper stapling have passed, while clean-install validation, hardware-backed lifecycle validation, and broader macOS validation remain open; the delivery/doctor work below remains broader than this probe.

Local development and notification testing continue with ad-hoc signed builds. Complete Developer ID signing, notarization, and clean-Mac Gatekeeper validation at the final release stage, before public distribution; they do not block notification implementation or local testing. The persisted identity foundation and opt-in serialized delivery worker now connect live macOS sessions to the branded helper, with category/review/snooze checks, sound/completion preferences, cancellation, co-reported per-printer intervention grouping, visible session outcomes, and a separate Config modal for immediately applied/saved global notification defaults. Synthetic regression coverage does not establish live hardware behavior. Next, extend per-printer notification editing/activity, quiet hours and timed grouping across reports, and validate remaining real-session behavior.

| Work | Issue |
| --- | --- |
| Persist notification identity and prevent duplicates | [#18](https://github.com/cliffom/bambu-tui/issues/18) |
| Test delivery and provide a doctor workflow | [#19](https://github.com/cliffom/bambu-tui/issues/19) |
| Show delivery activity and suppression reasons | [#20](https://github.com/cliffom/bambu-tui/issues/20) |
| Configure notification preferences in the TUI | [#21](https://github.com/cliffom/bambu-tui/issues/21) |
| Add quiet hours and burst grouping | [#22](https://github.com/cliffom/bambu-tui/issues/22) |
| Validate macOS desktop delivery and unsupported-environment behavior | [#23](https://github.com/cliffom/bambu-tui/issues/23) |

Notifications apply only while the TUI session is running. Quitting ends monitoring and notification delivery; v0.2.x does not introduce a background service. Persisted occurrence/job identity still prevents duplicate delivery when the TUI restarts.

The release gate is configurable, testable, deduplicated macOS desktop notifications with visible failure/suppression outcomes and verified shutdown of monitoring and delivery when the TUI exits. Saved reviews and snoozing must remain consistent with delivery. [Remembering farm filters/sorting (#24)](https://github.com/cliffom/bambu-tui/issues/24) is optional milestone polish, not a notification release blocker.

## v0.3.0 — LAN monitoring

[Investigate and prototype read-only LAN monitoring (#12)](https://github.com/cliffom/bambu-tui/issues/12). Verify protocol/authentication requirements and feasibility on available hardware. Plan secure credential storage, mixed cloud/LAN farms, source indicators, identity matching, partial reports, and reconnect handling.

Ship only documented, hardware-tested model/firmware combinations with consistent cloud/LAN state. Document unsupported combinations and defer them rather than claiming compatibility.

## Scope and validation

Printer control, job submission, camera feeds, and detailed filament inventory remain outside near-term scope. Keep unknown data distinct from confirmed state, and keep local acknowledgements, dismissals, snoozing, and physical fault resolution separate.

Each milestone needs meaningful regression coverage, keyboard/terminal-size validation where relevant, current documentation, and an accurate record of hardware limitations. Keep ongoing checks in the validation record rather than creating version-specific readiness documents.
