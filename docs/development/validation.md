# Validation record

[Contributing](contributing.md) · [Documentation index](../README.md) · [Entry requirements](documentation-direction.md#validation-record)

This record identifies checks actually performed, their source revisions, observations, and limits. It includes automated checks, synthetic browser/terminal inspection, native OS checks, and user-reported hardware observations; those evidence types are distinct. Outstanding work is maintained in [Remaining live checks](#remaining-live-checks).

Historical entries were consolidated from the [record at `2e9f74a`](https://github.com/cliffom/bambustand/blob/2e9f74a/docs/development/validation.md). Consolidation adds no new product validation. A local pass reported below is a historical report, not a retained test log or a rerun against the current release. Linked CI runs and repository captures are inspectable evidence; temporary capture paths and user-supplied screenshots without repository links were not archived here. Missing environment or revision details are stated rather than reconstructed. Historical names and compatibility results apply to the revision checked.

## Canonical application namespace (#162)

- **Scope:** 2026-10-30 PDT; v0.6.0 candidate on `issue-162-canonical-namespace`, based on `3e5743e`; direct canonical storage, credential, browser, and signing identifiers.
- **Method:** Go 1.27.3, pinned golangci-lint v2.15.0; synthetic account/region, persistence, credential, configuration/relative-theme and browser fixtures. Actual demo Config at 120 × 32 and 60 × 18, fallback at 59 × 17. Chromium 156.0.8078.4 against isolated `serve --demo`: 28 theme/layout combinations at 1280 × 800 and 390 × 844, plus 320 × 568 and 568 × 320 at 200% text.
- **Result:** `make docs-check fmt-check lint test check build release` and all 41 dependency-free browser tests passed. Four macOS/Linux arm64/amd64 archives passed inventory, bundled-document, helper architecture/ad-hoc signature and checksum checks. Config paths fit at 120 columns and truncated at 60; [Config storage capture](../../assets/screenshots/config-storage.png) remained representative. Follow host Settings captures were inspected at desktop and 320 × 568/200% for focus, scrolling and readable controls. Other gallery screens expose no host namespace and were unchanged.
- **Limits:** Initial namespace rollout, with no import of earlier development storage. Host OS version was not recorded for this entry. No new live permission/banner, clean-machine, physical-terminal, Linux runtime or printer checks. Local packaging was ad-hoc; no publication was performed.

## First-launch email login (#160)

- **Scope:** 2026-10-30 PDT; v0.5.4 candidate on `feat/160-startup-login`, based on `08b8b88`.
- **Method:** Synthetic local HTTP and fake credential storage with real CLI prompts/dashboard in race-enabled PTYs. Successful flows started at 120 × 40 and resized to 60 × 18; saved-session bypass used 120 × 40. No-color transcripts were inspected for masking, input bounds, save/fallback feedback and terminal restoration.
- **Result:** Full race/PTY tests, vet, docs, formatting, pinned lint, native build, vulnerability scan and four-target `make release` passed. Cases covered login/account validation, bypass, save failure, unavailable stores, rejected codes/requests, Escape and raw Ctrl+C. CLI reported 0.5.4. Linux CI exposed a fixture guard interrupting the existing injected-panic restoration handshake; restricting that guard to login fixtures restored the handshake. Dashboard/browser captures contain no login prompts and remained accurate.
- **Limits:** Host OS/toolchain versions were not recorded in this entry. The fixture fix does not establish repair of upstream Bubble Tea/cancelreader panic cleanup. No new real email/account/store, physical-terminal/color, hardware, clean-Mac or Linux interactive-runtime evidence. Packages were local ad-hoc candidates.

## BambuStand capitalization and artwork

- **Scope:** 2026-10-30 PDT; working tree based on `7e162db2b110`, initially reporting 0.5.2, then prepared as 0.5.3.
- **Method:** macOS, headless Chrome 156.0.8078.79; actual TrueColor demo PTYs at 150 × 48 and 60 × 18, fallback at 59 × 17. Documented browser capture command against isolated demo: all six themes at 1280 × 1200, compact desktop at 1280 × 640, mobile expanded/compact/offline at 390 × 844; additional default 320 × 568 and C64 568 × 320 at 200% text.
- **Result:** Inspected dashboard, About, Help, hero and ten browser wordmark captures. Hero's 400 × 140 layered source exported at 3× matched its PNG byte-for-byte. Terminal overview was rendered from actual PTY cells with Menlo, with resize repaint preventing stale startup rows. All 33 dashboard cases, 28 theme/layout combinations, 41 browser tests and 20 release-tool tests passed. Final 0.5.3 checks passed docs/formatting, pinned Go/workflow/Make lint, fault audit, vulnerability scan, race tests, vet and four-target release packaging; no known vulnerabilities. Settings/inspector crops contained no changed name and remained accurate; historical captures retained their recorded names.
- **Limits:** Newly inspected terminal color was TrueColor only; ANSI256/no-color behavior had automated coverage. Exact host OS/toolchain versions were not recorded here. Local helper/packages were ad-hoc. No new native authorization/banner continuity, notarization, live account/printer, Safari/Firefox, physical-mobile or physical-terminal accessibility checks.

## Source notification helper after rename

- **Scope:** 2026-10-30; local 0.5.1 installation, commit `8e066ca580c9`, built `2026-10-30T15:51:28Z`; subsequent 0.5.2 release preparation.
- **Method:** macOS source install contained `Bambu TUI Notify.app` while the CLI required `Bambu Stand Notify.app`. Ran `make notify-helper`, then `./bin/bambustand doctor --notify` outside the sandbox for Launch Services access.
- **Result:** Universal helper metadata/ad-hoc signatures passed; macOS accepted the silent request and the user confirmed a visible notification. Source-upgrade instructions now require rebuilding the helper. Final 0.5.2 `make docs-check fmt-check lint lint-actions lint-make audit-faults vuln-check test check release` passed on macOS 27.0.1 with pinned linters, no vulnerability findings and four-target archive/helper checks. No screen layout changed, so captures were retained.
- **Limits:** Sound, automatic delivery, Focus suppression, clean-Mac installation and live printers were not tested. This result concerns the locally rebuilt helper.

## Dependency maintenance (0.5.1)

- **Scope:** 2026-10-30; v0.5.1 candidate on `release/0.5.1`, dependency updates #123–#127.
- **Method:** macOS; pinned tools; full race/UI/PTY suite, including minimum-size Help, resize restoration and Server Status across TrueColor, ANSI256 and no color.
- **Result:** `make docs-check fmt-check lint lint-actions lint-make audit-faults vuln-check test check release` passed; no known reachable vulnerabilities. Four-platform archives passed inventory/docs and ad-hoc helper/signature checks. README, usage/farm guidance, Help and capture descriptions were compared with the dependency-only diff; no documented layout/control changed.
- **Limits:** OS version and candidate base commit were not recorded. No new visual demo, physical-terminal, clean-Mac, live account/printer or native-delivery inspection.

## Bambu Stand rename (#153)

- **Scope:** 2026-10-29 PDT; v0.5.0 candidate on `issue-153-bambu-stand`, based on `e4e4e10`.
- **Method:** macOS, Chrome 156.0.8078.79; synthetic upgrade fixtures for original storage, credential services, notice/snooze decisions, locks, delivery reservations and browser preferences. Actual TrueColor demo PTYs at 150 × 48 and 60 × 18, fallback at 59 × 17. Browser: 33 empty/single/24-printer cases across 1920 × 1080, 1280 × 800, 390 × 844, 320 × 568, 568 × 320 and 1280 × 240; 200% cases used 320 × 568, 390 × 844, 780 × 844, 568 × 320 and 1280 × 240.
- **Result:** Upgrade fixtures retained the identifiers used by that release and suppressed reserved-event replay. All 33 dashboard, 28 theme/layout and 41 browser tests passed, as did docs/formatting, pinned lint, race/PTY tests, vet, vulnerability scanning and four-target ad-hoc packaging. CLI reported 0.5.0. Dashboard/About/Help and ten refreshed browser wordmark captures were inspected at 1280 × 1200, 1280 × 640, 390 × 844 and 320 × 568/200%. Hero export matched its source; unaffected Settings/inspector crops and explicitly historical images were retained.
- **Limits:** No real credentials/accounts/printers or automatic OS notifications. Authorization continuity, clean-Mac trust, distribution signing/notarization, other browser engines, physical mobile and screen readers were not newly checked. Compatibility evidence is historical; #162 later changed the namespace. See current [identity/storage guidance](../user/usage.md#application-identity-and-storage).

## Tabbed browser printer inspection (#151)

- **Scope:** 2026-10-29 PDT; v0.4.8 candidate on `issue-151-inspector-tabs`, based on `3d63a27`.
- **Method:** Chrome 156.0.8078.79 on macOS with isolated demo at port 8149. Eleven inspector cases: 1920 × 1080, 1280 × 800, 390 × 844, 320 × 568, 568 × 320 and 1280 × 240 at default text; 320 × 568, 390 × 844, 780 × 844, 568 × 320 and 1280 × 240 at 200%. Captures used actual UI controls through `tools/capture-browser-screenshots.cjs`.
- **Result:** All 41 browser tests, eleven inspector cases, 33 dashboard cases and 28 theme/layout combinations passed. Cases exercised tab keys, independent scroll, fixed controls, polling/focus, local hiding/save errors, unavailable/unknown data, modal reset and Escape return. An old panel offset restored by native modal reopening was reproduced and covered after `showModal()`. Refreshed Status/Filament/Notices captures at 1280 × 1200 and narrow/short enlarged panel captures were inspected. Docs/formatting, pinned lint, race/PTY tests, vet, vulnerability scan and four-target ad-hoc packaging passed; CLI reported 0.4.8.
- **Limits:** At 1280 × 240/200%, only part of a panel fits; scrolling is necessary. No new TUI layout, Safari/Firefox, physical mobile, screen-reader, real account/printer or clean-Mac evidence.

## Compact browser header and pinned toolbar (#149)

- **Scope:** 2026-10-29 PDT; v0.4.7 candidate on `issue-149-compact-header`, based on `14be0f2`; review fixes included bases `426f8e9`, `4bf3dad` and `f6f278e`.
- **Method:** macOS 27.0.1 arm64, Go 1.27.3, Chrome 156.0.8078.79; isolated demo and empty/single/24-printer fixtures. Final 33-case matrix used the eleven size/text combinations recorded for [#151](#tabbed-browser-printer-inspection-151). Documented capture command generated thirteen gallery views across six themes at 1280 × 1200, compact 1280 × 640 and mobile 390 × 844, plus enlarged C64/IBM narrow/short views.
- **Result:** Final 33 dashboard cases, 28 theme/layout combinations and 39 browser tests passed, along with docs/formatting, pinned lint, race/PTY tests, vet, build and local packaging. Vulnerability scan found no findings. Captures were inspected for artwork, labels, focus, scroll and toolbar separation; Settings/inspector crops and TUI images remained unchanged. Review regressions reproduced and corrected Search focus moving 1800px → 1455px; background focus restoration moving 2200px → 370px after State sorting; a first-bottom-scroll clamp of 3972px → 3948px; and outer-page collapse shifting combined offset 69px → 48px at 1280 × 240. Final checks preserved content/scroll positions, including 0/1/12px bottom distances and 1280 × 240/200% outer-page cases. Corresponding desktop/mobile/short-window captures were inspected, including the [scrolled desktop view](../../assets/screenshots/browser-scrolled.png); temporary regression PNGs were not archived.
- **Limits:** Small workspaces still require page/workspace scrolling. Historical gallery refreshes are superseded by later naming captures. No new Safari/Firefox, physical-mobile, live-printer or physical-terminal validation. Packages were local ad-hoc candidates.

## Browser documentation screenshots

- **Scope:** 2026-10-29 PDT; v0.4.6 UI, source `017397b`; documentation-only refresh.
- **Method:** macOS 27.0.1 arm64, headless Chrome 156.0.8078.73; `tools/capture-browser-screenshots.cjs` against synthetic H2D/X2D/P1S demo fixtures, desktop 1280 × 1200 and mobile 390 × 844, default text.
- **Result:** Twelve captures covered six themes, Settings, telemetry/notices, scrolled filament, finished filtering and mobile/offline views. Each was visually inspected. Capture assertions checked image decode, horizontal bounds, offline omission, filters and modal contents without page errors; syntax and docs checks passed. Historical #134 before/after images retained distinct filenames. The [current gallery](../user/usage.md#browser-screenshots) has since been refreshed for later changes.
- **Limits:** No cloud connection, notification request or notice write. Full layout/theme/200% matrices, Go suite, hardware, other browser engines and physical-mobile checks were not rerun for this capture-only change.

## Offline browser cards (#145)

- **Scope:** 2026-10-29 PDT; v0.4.6 candidate; simplified explicitly offline cards.
- **Method:** macOS 27.0.1 arm64, Go 1.27.3, Chrome 156.0.8078.73; 30 empty/single/24-printer cases at six default viewports (1920 × 1080, 1280 × 800, 1280 × 240, 390 × 844, 320 × 568, 568 × 320) and four 200% viewports (320 × 568, 568 × 320, 780 × 844, 390 × 844); 28 host/bundled-theme combinations.
- **Result:** All 39 browser tests, layout/theme cases, full race/PTY suite, docs/formatting, pinned lint, vet, build, vulnerability scan, fault audit and four-target ad-hoc packaging passed. Cases retained attention, image/focus identity and last-known inspection while excluding full card telemetry only for explicit offline status. Default/Hotdog desktop, DeskMate mobile, C64 narrow enlarged and IBM short enlarged captures were inspected. Retained historical evidence: [desktop](../../assets/screenshots/browser-offline.png), [mobile](../../assets/screenshots/browser-offline-mobile.png).
- **Limits:** Small/short views require scrolling. Candidate base commit was not recorded. No new Safari/Firefox, physical-mobile, live transitions, clean-Mac or manual TUI inspection.

## Browser theme parity (#141)

- **Scope:** 2026-10-28 PDT; v0.4.5 candidate; browser host/bundled-theme parity and filter counts.
- **Method:** macOS 27.0.1 arm64, Go 1.27.3, Chrome 156.0.8078.73; synthetic demo, 28 theme/layout combinations and 30 dashboard cases. Desktop/mobile Settings and enlarged narrow/short captures were inspected.
- **Result:** All 37 browser interaction tests and thirty dashboard cases passed after the filter-count update; the preceding theme check passed 36 browser tests and 28 theme/layout combinations, including follow-host/custom colors, browser-only persistence, palette/contrast, filter counts, offline/last-known states, focus and overflow. Documentation/formatting, pinned Go/workflow/Make lint, full race/PTY tests, vet, build, fault audit, vulnerability scan (no findings) and four-target ad-hoc packaging passed. Appearance capture was refreshed after removal of the redundant MONITORING badge; the sort-chevron assertion was scoped to the toolbar control.
- **Limits:** Candidate base commit was not recorded; Chromium evidence does not establish Safari/Firefox, physical-mobile or hardware results. No new terminal presentation or native notification validation.

## Browser dashboard cleanup (#134)

- **Scope:** 2026-10-28 PDT; v0.4.4 candidate; single toolbar, current Idle / finished filter, counts and footer status. Base commit was not recorded.
- **Method:** macOS 27.0.1 (26A434) arm64, Go 1.27.3, Chrome 156.0.8078.73; synthetic demo snapshots, 30 empty/single/24-printer cases across the six default/four enlarged viewports listed for [#145](#offline-browser-cards-145). Historical captures use the same three-printer fixture: [desktop before](../../assets/screenshots/browser-before-134.png), [after](../../assets/screenshots/browser-after-134.png), [mobile before](../../assets/screenshots/browser-mobile-before-134.png), [after](../../assets/screenshots/browser-mobile-after-134.png), at 1280 × 1200 and 390 × 844.
- **Result:** All 31 browser interaction tests, 30 layout cases, docs/formatting, pinned lint, race/PTY tests, vet, build, fault audit, vulnerability scan (no findings) and four-target ad-hoc packaging passed after the result-count, status and chevron refinements. Cases exercised current versus last-reported state, unavailable/feed-loss membership, attention overlap, disappearing focused cards, filter keyboard focus, sort icon bounds and footer accessibility. Desktop/mobile captures were inspected and refreshed.
- **Limits:** These are synthetic browser observations; no new live hardware, physical-mobile or other-engine evidence. Historical before/after captures depict that revision rather than the current gallery.

## Pinned browser status bar (#132)

- **Scope:** 2026-10-28 PDT; v0.4.3 candidate.
- **Method:** macOS arm64, Go 1.27.3, Chrome 156.0.8078.73; `tools/test-browser-layout.cjs` with empty/single/24-printer demo farms. Initial eighteen combinations expanded to thirty after narrow/short-window failure reproduction.
- **Result:** Reproduced a zero-height workspace at 320 × 568/200%. Final thirty cases included that size, 568 × 320 at default/200% and 1280 × 240 at default; they verified usable workspace, bounds, final-content reachability, focus, footer keyboard access and restoration at 1280 × 800. Desktop/narrow/enlarged and full-page 320 × 568/568 × 320 enlarged captures were inspected. All 29 browser tests, docs/formatting, pinned lint, race/PTY tests, vet, vulnerability scan, fault audit and four-target ad-hoc packaging passed; CLI reported 0.4.3.
- **Limits:** Page scrolling is necessary when header/footer cannot fit with the workspace. No new live account/printer, physical-terminal/mobile, Linux runtime, Safari/Firefox, native-delivery or clean-Mac evidence.

## Live print job names (#129)

- **Scope:** 2026-10-28 PDT; v0.4.2 fix. Original H2D screenshot showed the name-loss symptom on firmware `01.04.00.00`.
- **Method:** macOS 27.0.1 (26A434) arm64, Go 1.27.3; synthetic partial-start, delayed identity, zero/null/whitespace placeholder and known-boundary fixtures. Render tests at 60 × 18, 120 × 32 and 160 × 50; actual demo PTY text inspected at 120 × 32, 60 × 18 and 40 × 10 fallback.
- **Result:** Names/estimates/stages survived delayed identities while confirmed new jobs still cleared old metadata; selection/focus and table/details agreed. Docs/formatting, pinned lint, fault audit, vulnerability scan, full race/PTY tests, vet, build and four-target ad-hoc packaging passed; CLI reported 0.4.2.
- **Limits:** Original screenshot establishes the symptom, not live validation of the fix. Firmware identity ordering, live H2D/X2D reproduction, physical-terminal quality, Linux runtime and clean-Mac/native-delivery checks remain unverified.

## Notification history and session boundaries (#105, #106)

- **Scope:** 2026-10-27 PDT; v0.4.1 candidate.
- **Method:** macOS 27.0.1 (26A434) arm64, Go 1.27.3; temporary ledgers/archives, including a full 4,096-job v1 ledger, replay, overflow, checksum/I/O/compaction failures, competing claims and transport-isolation fixtures. Rendered capacity states at 60 × 18, 120 × 32 and 59 × 17 across three color profiles; actual no-color demo PTY at those sizes/restoration. Chrome 156.0.8078.73 Settings at 1280 × 800, 390 × 844 and 780 × 844/200%, with permission/submission stubs.
- **Result:** History migration/replay, bounded live state, conservative identities, protected maintenance and shared failed-load policy passed. Near-capacity fixtures and browser warnings were inspected; no real notification storage was modified. All 29 browser tests, docs/formatting, pinned lint, full race/PTY tests, vet, vulnerability scan and four-target ad-hoc packaging passed; CLI reported 0.4.1.
- **Limits:** No live firmware/job-identity, native/browser banner, physical power-loss, physical-terminal, Linux runtime, other browser engine or clean-Mac evidence. Exact archives grow on disk; this check did not establish a disk-space purge.

## Terminal safety and persistence durability (#119, #120)

- **Scope:** 2026-10-27 PDT; v0.4.0 candidate.
- **Method:** macOS 27.0.1 (26A434) arm64, Go 1.27.3; synthetic future/OSC 52/CSI/C0/C1 state labels, directory-open/sync/close failures and visible-but-unconfirmed reservations. Actual demo PTY/pyte text and rendered fixtures at 120 × 32, 60 × 18, 59 × 17 and restoration in TrueColor, ANSI256 and no color. Reproduction: `go test ./internal/ui -run '^TestTelemetryStateCannotInjectTerminalControls$' -v`.
- **Result:** Printable labels survived sanitization; raw telemetry was unchanged. Modeled commit failures paused delivery without sending; recovery consumed visible reservations. Layout text, guards and terminal restoration were inspected. Full race/PTY tests, 28 browser tests, docs/formatting, pinned lint, vet, vulnerability scan, fault audit and four-target ad-hoc packaging passed; CLI reported 0.4.0.
- **Limits:** At 60 × 18 the full details require scrolling. Clipboard effects in physical terminals and physical power-loss recovery were not observed. Durability depends on OS/filesystem/device sync support; no new live account/printer, Linux runtime, native banner or clean-Mac evidence.

## Explicit persistence retry (#107)

- **Scope:** 2026-10-27 PDT; v0.3.11 candidate.
- **Method:** macOS 27.0.1 (26A434) arm64, Go 1.27.3; injected independent store failures, delayed completions and coalesced edits. Rendered failed/saving/saved/protected/demo states at 60 × 18 and 120 × 32 in three color profiles, fallback at 59 × 17. Actual demo PTY/pyte at those sizes confirmed session-only behavior. Reproduction: `go test ./internal/ui -run '^TestStatusRetry' -v`.
- **Result:** Retry preserved decisions/identities and ownership, rejected obsolete completions and protected unreadable stores. Failure details remained during retry; demo offered no durable-save retry and blocked hidden `r`. Rendered fixtures and demo returns/quit were inspected. Docs/formatting, pinned lint, full/focused race tests, vet, build, vulnerability scan and four-target ad-hoc packaging passed; CLI reported 0.3.11.
- **Limits:** Failed/pending/saved durability states came from injected fixtures, since demo has no account stores. No real storage-failure recovery, account/printer, physical-terminal, OS banner, Linux runtime or clean-Mac evidence.

## Active printer sorting (#116)

- **Scope:** 2026-10-27 PDT; v0.3.10 candidate and browser grouping/card-width follow-ups.
- **Method:** macOS 27.0.1 (26A434) arm64, Go 1.27.3; mixed activity/state/freshness/offline fixtures for all sort modes. Actual TrueColor/no-color demo PTY text at 120 × 32, 60 × 18 and 59 × 17/restoration; mixed-group/empty-filter rendered fixtures. Chrome 156.0.8078.73 with intercepted demo snapshots at 1440 × 1000, 1000 × 900, 390 × 844 and 320 × 700.
- **Result:** TUI/browser grouping, deterministic ties and identity-based selection/focus/scroll passed. One/two-result cards retained full-list widths: 372px at 1440, 364px at 1000, 354px at 390, 284px at 320. Wide single-card screenshot was inspected. All 28 browser tests and docs/formatting, pinned lint, full race/PTY tests, vet, vulnerability scan, fault audit and ad-hoc release packaging passed.
- **Limits:** At 60 × 18 only three rows fit; trailing groups require navigation. Historical 320px/200% browser check measured 377px content width, an unresolved gap at this revision; later [#132](#pinned-browser-status-bar-132) checked narrow enlarged layouts after correction. No new hardware, physical-terminal, Linux runtime, direct Safari or clean-Mac evidence.

## TUI Server Status (#113)

- **Scope:** 2026-10-27 PDT; v0.3.9 candidate.
- **Method:** macOS 27.0.1 (26A434) arm64, Go 1.27.3; actual demo xterm-256color PTYs at 120 × 32, 60 × 18 and 59 × 17/restoration in TrueColor, ANSI256 and no color. Synthetic long-history, failed/empty/stopped and simultaneous storage-warning fixtures; real localhost client connect/disconnect with five-second grace.
- **Result:** Inspected URL, separate uptimes, client activity, scrolling, nested Config/feedback returns, hidden-input guards and cleanup. Diagnostic privacy/bounds, listener restart timestamps, independent snapshots and omission counts passed. Docs/formatting, pinned lint, full/focused race/PTY tests, vet, build, vulnerability scan and four-target ad-hoc packaging passed; CLI reported 0.3.9.
- **Limits:** PTY/rendered text does not establish physical-terminal/font appearance. No new real cloud/reconnect, printer firmware, real-browser socket timing, Linux runtime, OS notification or clean-Mac checks.

## Browser printer image continuity

- **Scope:** 2026-10-27 PDT; v0.3.8 plus continuity fix.
- **Method:** macOS 27.0.1 arm64, Go 1.27.3, Node.js 24.22.1; Playwright 1.64.1 with WebKit 27.2 and Chrome 156.0.8078.73 against demo. Five immediate renders and two real polls; bounds at 1440 × 1000, 768 × 1024, 390 × 844, 320 × 700 and 390 × 844/200%.
- **Result:** Before/after probes reproduced old image replacement and measured zero unchanged-image removals or source writes after the fix. Sorting/filtering/fallback transitions, focused cards, keyboard inspection and a WebKit local hide passed. WebKit desktop/narrow screenshots were inspected. All 22 browser tests, webserver race tests, build, docs and whitespace checks passed.
- **Limits:** WebKit automation is not direct Safari.app/iOS confirmation. No new physical-terminal, live-printer or other browser-version evidence.

## Foreground serve status (#110)

- **Scope:** 2026-10-27 PDT; v0.3.7 source plus #110, followed by v0.3.8 preparation.
- **Method:** macOS arm64, Go 1.27.3; actual demo xterm-256color PTYs at 120 × 40, 80 × 24, 60 × 18, 40 × 12 and 20 × 10, first monochrome then TrueColor. Fake-cloud disconnect/auth rejection, storage warnings, signals and redirected logs had separate fixtures. Two same-IP/User-Agent synthetic HTTP sockets and one unused socket exercised grouping/grace.
- **Result:** Inspected URL wrapping, connection/uptime, warning/authentication text, resized output and restored screen/cursor after Ctrl+C. Client tracking produced one group/connect and one disconnect after five seconds; unused sockets were excluded. Ledger rows fit through 40 × 12 and dropped at 20 × 10. Three-profile fixtures covered bounds/color roles, diagnostics/privacy, concurrency and cleanup; 10 × 6 and 1 × 1 cases recorded expected guidance loss. Final 0.3.8 docs/formatting, pinned lint, fault audit, vulnerability scan, full race tests, vet and four-target ad-hoc packaging passed.
- **Limits:** TrueColor connected/disconnected captures were inspected; ANSI256 and attention/error colors had automated fixtures. Extremely small terminals cannot display all guidance. Grouping is sanitized IP/User-Agent, not authenticated identity/tab count. No new physical-terminal, Linux runtime, live cloud/printer, real-browser socket or notification-delivery evidence.

## Browser system notifications (#91)

- **Scope:** 2026-10-27 PDT; v0.3.7 development/release checks, Settings relocation and repeat-test correction.
- **Method:** macOS 27.0.1 arm64, Go 1.27.3, headless Chrome 156.0.8078.48; actual demo with temporary stores. Browser-created API requests, programmatic permission grants/denials/clicks, two tabs, eleven-second freeze/resume, reload and storage events. Bounds at 1440 × 1000, 768 × 1024, 390 × 844, 320 × 700 and 390 × 1000/200%; desktop/mobile/Settings/inspector captures were inspected.
- **Result:** Independent native/browser reservation, restart/concurrent-claim deduplication, baseline/suppression/overflow and shutdown cases passed. Enable/Test separation, Settings focus return, polling while closed and cross-tab Disable passed without page errors. Repeat Test initially replaced earlier synthetic notifications; removing their shared tag passed script regressions and the user confirmed repeat tests worked. Final Settings suite reported nineteen passing browser fixtures; the repeat-test fix reported eighteen at its stage. Docs/formatting, pinned lint, full race/PTY tests, vet, vulnerability scan and four-target ad-hoc packaging passed.
- **Limits:** User repeat-test report omitted OS/browser versions and settings. Automation establishes API/interaction outcomes, not human OS permission/banner clicks or audio. No new Safari/Firefox, Linux/Windows browser, physical-mobile, hardware completion, Focus or clean-Mac results. Closed/suspended-page delivery is not guaranteed.

## Account persistence ownership (#104)

- **Scope:** 2026-10-27 PDT; v0.3.6.
- **Method:** macOS arm64, Go 1.27.3; synthetic accounts/telemetry and private temporary storage. Independent handles and pipe-synchronized child processes exercised contention, normal exit, crash release, handoff and a blocked active save during Close; host tests checked rejection before terminal startup.
- **Result:** Ownership protected all notice/snooze mutations, rejected stale snapshots and retained the lock for entered writes. New lifecycle cases passed ten race-enabled repetitions. Full race/PTY suite, docs/formatting, pinned lint, vet, build, vulnerability scan and four-target ad-hoc packaging passed. Contention is pre-UI; no layout inspection was needed.
- **Limits:** No live cloud/hardware, Linux contention, network-filesystem locking or mixed-version validation. Older sharing hosts must be stopped; external edits are not live synchronized.

## Foreground telemetry rejection (#103)

- **Scope:** 2026-10-27 PDT; v0.3.5.
- **Method:** macOS arm64, Go 1.27.3; malformed/oversized/deep/cumulatively over-limit synthetic report bursts followed by valid partial reports on the same owner/listener; temporary configuration.
- **Result:** Readings, freshness, occurrence targets, hiding and snoozes survived rejection; valid telemetry resumed with one fixed private diagnostic. Recovery/monitor-outcome cases passed twenty race-enabled repetitions. Docs/formatting, pinned lint, full race/PTY tests, vet, build, vulnerability scan and four-target ad-hoc packaging passed.
- **Limits:** No new UI layout inspection because presentation was unchanged. No real cloud/H2D/X2D recovery, Linux runtime or distribution-trust evidence.

## Cloud-backed terminal-free server (#98)

- **Scope:** 2026-10-27 PDT; v0.3.4.
- **Method:** macOS arm64, Go 1.27.3; fake cloud/native services, temporary stores, real subprocess hosts and headless Chrome. Browser bounds at 1440 × 1000, 390 × 844, 320 × 700 and 390px/200%; desktop fake-cloud, narrow demo and enlarged inspector captures were inspected.
- **Result:** Missing/invalid/cancelled lookup, typed auth rejection, transient reconnect, monitor/listener failure, signals, store isolation, failed-load protection, coalescing/retry/drain, browser hiding/reload and cloud-decision restart restoration passed. Config bytes and saved notification opt-in remained unchanged. Docs/formatting, pinned lint, race/PTY tests, browser scripts, vet, build, vulnerability scan and four-target ad-hoc packaging passed.
- **Limits:** Browser version was not recorded. No real saved-account startup/expiry, live recurrence, H2D/X2D, Linux Secret Service/runtime, locked-store UI, native delivery or clean-Mac evidence. Earlier #90 phase descriptions are historical, not the current backlog.

## Terminal-free demo server (#97)

- **Scope:** 2026-10-27 PDT; v0.3.3.
- **Method:** macOS arm64, Go 1.27.3; synthetic telemetry, temporary stores and real terminal-free binary in headless Chrome at 1440 × 1000, 390 × 844, 320 × 700 and 390px/200%.
- **Result:** Browser focus/hiding, second-tab polling, hidden inspection, reload, tabs closing without monitor exit, SIGTERM, unchanged config and restart-reset demo decisions passed. Desktop/phone hidden-message captures were inspected. Store/session fixtures covered coalescing, drain and shutdown rejection. Docs/formatting, pinned lint, race/PTY tests, browser scripts, vet, build, vulnerability scan, fault audit and four-target ad-hoc packaging passed.
- **Limits:** Browser version was not recorded. No live cloud/hardware, native notifications, Linux runtime, physical-terminal, Safari/Firefox or clean-Mac validation. Cloud serving was not implemented at this revision; #98 supplied later evidence.

## Shared monitoring session (#96)

- **Scope:** 2026-10-27 PDT; v0.3.2 session extraction.
- **Method:** macOS arm64, Go 1.27.3; synthetic reports/stores, actual demo PTY/emulator at 120 × 32, 60 × 18 and 59 × 17/restoration; headless Chrome at 1440 × 1000, 390 × 844, 320 × 700 and 390px/200%.
- **Result:** Reconciliation/identical-timestamp recurrence, decisions, immutable/coalesced saves, delayed completions, protected loads, config cloning, snooze cleanup and suspended status passed. Actual browser hide updated the suspended TUI inspector; duplicate/unknown/foreign-Origin requests behaved as expected, with second-tab polling and retained focus. Phone, Settings and updated inspector captures were inspected. Docs/formatting, pinned lint, race/PTY tests, browser scripts, vet, build, vulnerability scan and four-target ad-hoc packaging passed.
- **Limits:** Browser version was not recorded. No new hardware/account, physical-terminal, other browser engine, Linux rendering, theme/no-color or clean-Mac evidence. Terminal-free modes were later phases, not validated by this extraction.

## Browser hidden-message visibility

- **Scope:** 2026-10-27 PDT; browser filtering/toggle, spacing and Close-focus correction; exact revision not recorded.
- **Method:** Headless Chrome on macOS arm64 with actual assets and intercepted synthetic same-origin responses, 1440 × 1000, 390 × 844, 320 × 700 and 390 × 1000/200%.
- **Result:** Default hidden filtering, counted reveal without decision changes, all-hidden versus empty, polling/focus retention, reset and failed-save retry passed. Measured toggle gaps were 16px at default text and 32px at 200%. Hiding/removing a focused condition retained scroll and moved focus to Close; removing `preventScroll` in memory made both regressions fail. Phone/expanded captures were inspected. Node regressions, syntax/docs/build and UI/webserver race checks passed at their recorded stages.
- **Limits:** Chrome version was not recorded. Intercepted data does not establish live persistence, hardware or other browsers. Terminal presentation was unchanged.

## Browser notice dismissal (#92)

- **Scope:** 2026-10-27 PDT; v0.3.1.
- **Method:** macOS arm64, Go 1.27.3; synthetic telemetry/stores; real demo listener in headless Chrome at 1440 × 1000, 390 × 844, 320 × 700 and 390px/200%. Actual PTY/emulator at 120 × 32, 60 × 18 and 59 × 17/restoration; separate pending/failed/saved response fixtures.
- **Result:** Host/Origin/CSRF/body/queue restrictions, stale and identical-timestamp occurrence handling, saving/retry/coalescing and shutdown rejection passed. Browser keyboard hide propagated to a second tab and TUI decisions; restore and suspended-inspector updates were inspected. Desktop/phone and TUI return captures retained controls/focus. Docs/formatting, pinned lint, full/focused race tests, syntax, vet, build, vulnerability scan and four-target ad-hoc packaging passed.
- **Limits:** Browser version was not recorded. No new Safari/Firefox/touch, Linux rendering, physical-terminal/color-profile, live recurrence, cross-process synchronization or signed-distribution evidence. Local request protections do not authenticate other users/programs on the host.

## Local web dashboard (#88) — synthetic evidence

- **Scope:** 2026-10-26 PDT; branch based on v0.2.12, followed by v0.3.0 preparation and dashboard/artwork/port/typography corrections.
- **Method:** macOS arm64, Go 1.27.3; actual demo PTY/pyte in TrueColor/no color at 120 × 32, 60 × 18 and 59 × 17/restoration. Chrome 156.0.8078.48 against real demo listener and synthetic responses at 1440 × 1100, 1024 × 900, 768 × 1024, 390 × 844 and 320 × 700; typography also used 1440 × 1500, 1024 × 1000 and 1280 × 1000/200%, 390 × 1000/200%.
- **Result:** Inspected Web Server state/URL/returns, browser telemetry/conditions/unknown/last-known views, outage retention, search/inspection and same-origin assets without page errors. Route/privacy/concurrency, lifecycle, persistence and hidden-control regressions passed. v0.3.0 docs/formatting, pinned lint, fault audit, vulnerability scan, race/PTY tests, vet, build and four-target ad-hoc packaging passed. Two macOS-CI PTY quit failures were reproduced as Escape+q decoding as Alt+Q; waiting for the dashboard passed ten race-enabled repetitions of `TestPTYWeb`.
- **Limits:** Full-page headless captures showed an artifact; visual inspection used viewport captures and scrolling. ANSI256/other terminal themes were not newly inspected. No live cloud/printer, Safari/Firefox, Linux rendering, physical-terminal or clean-Mac evidence. The original dashboard had no headless mode; later releases added it.

### Printer sprite follow-up

Built-in image generation on the same date/setup used official Bambu model renders for shape and repository logo/hero for style. All five PNGs were inspected for transparency/padding. [Branding sources](../reference/branding.md#printer-sprites) and [exact prompts](../reference/printer-sprite-prompts.md) retain provenance; reference renders were not bundled. Real demo served H2D/X2D; Chrome fixtures decoded all five models plus generic fallback at 1440 × 1500, 1024 × 900, 390 × 844 and 320 × 700. Phone/desktop inspection confirmed aspect ratio and local assets; waiting for lazy decode avoided blank early captures. Model alias/digest/route/PNG/privacy tests and full checks passed. Decorative art does not validate hardware state/accessories.

### Condition heading and close-button spacing

Chrome synthetic fixtures at 1440 × 1000 and 390 × 844 confirmed a 12px condition-heading gap. Initial text-box centering did not resolve the user-reported close-glyph alignment; CSS strokes retained centered 12 × 2px geometry at 2× scale with serif/monospace 40px overrides. Phone/enlarged icon captures were inspected; docs/build passed. No hardware or other-engine inspection.

### Configurable port and occupied-port failure

On the same date/setup, TrueColor/no-color demo PTYs at 60 × 18, 120 × 32 and 59 × 17/restoration exercised occupied-port failure, invalid input, feedback return, explicit retry, port change/old socket closure and saved reload. Regression tests covered 1–65535 validation, default 8080, no fallback bind, independent merge/coalescing and hidden guards. Docs/formatting, pinned lint, race tests, vet and build passed. This superseded the earlier ephemeral-port policy; browser presentation was unchanged.

### Browser typography and console font

Computed Chrome checks found no visible text below 14px at default root and one inherited monospace stack; desktop/phone/dialog captures were inspected. Narrow attention-button overflow at 200% was corrected to wrap; horizontal bounds, search/reset, Escape and page-error checks passed. This is focused readability evidence, not accessibility certification or validation of other browsers/assistive technologies.

## v0.2.12 — About label styling

- **Scope:** 2026-10-26 PDT; v0.2.12 styling fix.
- **Method:** macOS arm64, Go 1.27.3; sixteen actual demo PTY/emulator frames in default TrueColor/no color at 120 × 32, 60 × 18 and 59 × 17/restoration.
- **Result:** Contact sheets showed secondary Theme/Theme Palette labels, primary values and restored status/dashboard navigation. Formatting/docs, pinned v2.15.0 lint, vet and UI/CLI race tests passed; palette-replacement regression covered TrueColor/ANSI256/no color.
- **Limits:** Other themes and ANSI256 had automated coverage only; no new physical-terminal/font or live hardware evidence.

## v0.2.11 — About Bambu TUI (#85)

- **Scope:** 2026-10-26 PDT; v0.2.11, with the final revised presentation superseding the initial email/labelled-palette layout.
- **Method:** macOS arm64, Go 1.27.3; actual synthetic demo PTYs across all six bundled themes in TrueColor/no color at 120 × 32, scrollable 60 × 18 and 59 × 17/restoration. Initial and final presentations each had 96 captured frames; revised contact sheets were inspected for metadata order, unique swatches, spacing, URL and restored navigation.
- **Result:** Final metadata/branding/attribution, unique hue-sorted swatches, effective theme source, status returns and dashboard focus were inspected. Account information and optional-email changes were removed before final presentation. Palette/ownership/cache/resize regressions, full checks and four-target ad-hoc packaging passed. Earlier labelled grids and email-fixture results apply only to the superseded implementation.
- **Limits:** ANSI256 and custom/long-value cases had automated coverage, not separate demo inspection. No real-account migration/login, hardware, physical-terminal, native banner, clean-Mac or Intel runtime validation.

## v0.2.10 — customizable themes

- **Scope:** 2026-10-26 PDT; v0.2.10 preparation; detailed theme inspection follows under [#82/PR #83](#status-prominence-and-palette-foundation-82).
- **Method:** Local macOS arm64, Go 1.27.3; pinned tools and default `make release`.
- **Result:** Docs/formatting, Go/workflow/Make lint, fault audit, vulnerability scan, race/PTY tests, vet and four-target ad-hoc archive/helper/checksum checks passed. CI exposed unsynchronized hidden input arriving after SIGWINCH; removing that input from the PTY handshake restored deterministic resize testing while synchronous reducer guards retained hidden-input coverage.
- **Limits:** This test synchronization correction adds no new visual or live evidence. No new physical-terminal, hardware/account, native banner, clean-Mac or Intel runtime checks.

## Status prominence and palette foundation (#82)

- **Scope:** 2026-10-26 PDT; v0.2.9-based checkout for [#82](https://github.com/cliffom/bambustand/issues/82) and PR #83 theme additions.
- **Method:** Local macOS arm64, Go 1.27.3; actual demo PTYs at 60 × 18, 120 × 32 and 59 × 17/restoration, plus rendered fixtures in TrueColor, ANSI256 and no color. Temporary absent-printer overrides exposed ownership while both demo printers' desktop delivery stayed off.
- **Result:** Palette isolation/cache round trips, unchanged feedback lifetimes, every footer cell, semantic outcomes and inherited/reported-color exceptions passed; [UI direction](ui-direction.md#palette-and-message-emphasis) records contrast criteria and inherited-background/quantization limits. Foundation inspection covered 36 demo and 120 synthetic frames; subsequent theme inspections are scoped below. Docs/formatting, pinned lint, full race/PTY tests, vet, build and vulnerability scanning passed; theme additions also passed four-target ad-hoc packaging. No native probes were requested in these inspections.
- **Limits:** Captured terminal cells/contact sheets are emulator evidence. Custom colors are syntax-validated without automatic contrast correction; ANSI256 quantizes RGBs. No new physical-terminal/font, live account/printer, native banner/audio, clean-Mac or Intel runtime checks.

### JSON themes and body surfaces (PR #83)

Default/Gruvbox inspection covered 104 demo frames (26 per theme/color combination), including ownership, refresh, local actions, notices/status, save failure/retry, activity, Help, inventory, Config and percentage editing. Painted-cell fixtures checked nine routes across supported/fallback sizes and three profiles. JSON cases covered partial/null/duplicate/unknown/version/oversize/path failures, pre-auth rejection and config merging. Default inherits terminal background; Gruvbox paints its own surface.

### Launch theme selection (PR #83)

Eighteen demo captures checked CLI, Make and custom paths containing spaces, conflicting launch flags and missing configured files. Dashboard/filament/Config, fallback/restoration and saved filter edits retained the configured theme choice while applying the launch override. Make lint (checkmake 0.3.2), full checks and packaging passed.

### Hotdog light-theme demonstration (PR #83)

Hotdog explicitly takes inspiration from the red/yellow Hotdog Stand look. Fifty-two TrueColor/no-color demo captures covered the shared routes/states and cream/yellow/red surfaces. Tests guarded body/selection contrast at 4.5:1 and fresh footer information at 7:1; inherited terminal surfaces remain outside those guarantees.

### C64 blue theme (PR #83)

Base/selection blues reference the [VICE Pepto PAL palette](https://github.com/VICE-Team/svn-mirror/blob/main/vice/data/C64/pepto-pal.vpl); brighter text/feedback variants retain readability. Fifty-two demo captures and shared canvas/contrast/CLI tests passed. This is a palette reference, with filament colors still supplied by telemetry.

### IBM 3270 green-screen theme (PR #83)

Fifty-two demo captures checked green-intensity body, text, selection and footer across shared routes/states. Embedded loading, green-dominant role values, contrast, three-profile canvas and CLI saved-choice tests passed; reported filament swatches were unchanged.

### JSON discovery and custom themes without rebuilding (PR #83)

Wildcard inventory and validator regressions passed without per-theme registration. Eighteen captures used one compiled executable and a custom file with spaces, first cream, then edited light blue, then no color. Binary SHA-256 stayed unchanged; invalid color failed before alternate-screen entry. Dashboard/filament/Config and fallback/restoration were inspected.

### DeskMate II blue/gray theme (PR #83)

The palette was taken from the [preserved blue/gray capture](https://nerdlypleasures.blogspot.com/2024/07/tandy-deskmate-tandys-ace-in-hole.html), with its four PNG colors inspected directly; the [Tandy manual](https://www.manualslib.com/manual/1336444/Tandy-1000.html?page=34) documents selectable schemes. Fifty-two demo captures and shared tests passed. Measured minimum body contrast was 5.72:1, selection/fresh-footer 9.04:1. Addition required JSON/docs only.

### Selection foreground precedence fixes (PR #83 review)

Rendered-cell tests reproduced unreadable selected offline rows/activity outcomes before correction. Tests then passed for all six themes, widths 60/80/120 and three color profiles, preserving semantic colors when unselected and guarding selected contrast at 4.5:1. Inspected 96 Default/DeskMate synthetic frames and eighteen actual demo PTY frames for populated activity, selection and fallback/restoration at both supported sizes with color on/off. Confirmed-offline and delivery outcomes used synthetic models; actual demo printers were online with native delivery off. Full checks and targeted uncached race tests passed; no new live or physical-terminal evidence.

## v0.2.9 — consistent status feedback

- **Scope:** 2026-10-26 PDT; v0.2.9 release preparation.
- **Method:** Local macOS arm64, Go 1.27.3; pinned tools, full race/UI/PTY suites and default release packaging.
- **Result:** Docs/formatting, Go/workflow/Make lint, fault audit, vulnerability scan, vet and all four archive/install-guide/helper/checksum checks passed. Actual UI inspection is scoped in [Footer and status audit](#footer-and-status-audit-79).
- **Limits:** Local packages were ad-hoc. Preparation adds no new hardware/account, physical-terminal, native banner, clean-Mac or Intel runtime evidence.

## Footer and status audit (#79)

- **Scope:** 2026-10-26 PDT; v0.2.8-based checkout for [#79](https://github.com/cliffom/bambustand/issues/79); route/status audit and footer paint correction.
- **Method:** Local macOS arm64, Go 1.27.3; 342 rendered frames (nineteen screens × three states × three sizes × two color modes), plus 48 footer-cell cases in TrueColor/ANSI256/no color. Four actual demo PTY/pyte sessions captured 148 frames at 60 × 18 and 120 × 32 with color on/off, 59 × 17 fallback/restoration and injected Config save failure.
- **Result:** Help scrolling, focus-specific hints, overlapping errors, nested status return, Config editing/test completion, local feedback, late failures and confirmation-scoped history passed. Inspected all dashboard panels, notice/attention/snooze, Help/Status, filament, alerts, decisions/activity, Config and exports/confirmations. An ANSI reset exposing terminal background at the footer separator was reproduced; every-cell regression failed before correction and passed afterward. Formatting, pinned lint, full race/PTY tests, vet, build, docs and vulnerability checks passed.
- **Limits:** Demo desktop delivery stayed off; no native probe. Pending/native-test outcomes, overlapping failures, long/empty/unknown variants came from synthetic models. No new physical-terminal/font, live account/printer, native banner/audio, clean-Mac or Intel runtime evidence. Route contracts are maintained in [UI direction](ui-direction.md), rather than duplicated here.

## v0.2.8 — action-needed hierarchy and observation time (#77)

- **Scope:** 2026-10-26 PDT; v0.2.8.
- **Method:** Local macOS arm64, Go 1.27.3; timestamp/lifetime/recurrence/skew and interaction fixtures; actual TrueColor/no-color demo PTYs at 120 × 32, 60 × 18 and 40 × 10/restoration with synthetic three-notice X2D runout.
- **Result:** Wrapped intervention/time/code text, fixed printer header, footer actions, scrolling and observed resolution were inspected; resolution removed guidance without resetting observation times. Full checks, pinned lint, vulnerability scan, fault audit and four-target ad-hoc packaging passed.
- **Limits:** Multi-printer queues, long/skewed/missing times and stale/disconnected states had synthetic coverage only. No new physical-terminal, real account/hardware, native banner/audio, clean-Mac or Intel runtime checks.

## v0.2.7 — saved farm views (#24)

- **Scope:** 2026-10-26 PDT; v0.2.7 release preparation.
- **Method:** macOS arm64, Go 1.27.3; v0.2.6-based implementation fixtures for filter/sort round trips, per-field invalid fallbacks, merges, coalescing, failures/retry and restart. Actual demo PTY with color on/off at 120 × 32 (populated/empty and selection), 60 × 18 (empty/reset) and 59 × 17 (blocked hidden reset).
- **Result:** The real CLI PTY saved Attention/Attention, restarted with the same config, and reset/saved All/Name at 60 × 18. Both demo inspections retained selection and exited after saving; no telemetry/selected identity was persisted. Docs/formatting, pinned Go/workflow/Make lint, fault audit, vulnerability scan, race tests, vet and four-target local archive/install-guide/helper/checksum checks passed.
- **Limits:** Local ad-hoc packaging does not establish signed distribution or real-account restart behavior. No new live hardware or native-delivery evidence.

## v0.2.6 — quiet hours and burst grouping (#22)

- **Scope:** 2026-10-26 PDT; v0.2.6.
- **Method:** Local macOS arm64, Go 1.27.3; controllable-clock tests for weekday/overnight/DST, exit/restart/recurrence, quiet completion/queued frames, grouping across reports/printers/completions, changed exclusions and rolling rate bounds. ANSI-stripped quiet activity views were inspected at 60 × 18 and 120 × 32; 59 × 17 guards had automated coverage.
- **Result:** Docs/formatting, pinned Go/workflow/Make lint, fault audit, vulnerability scan, full race/PTY tests, vet and four-target ad-hoc packaging passed. Quiet explanations fit both sizes; independent snooze/category/desktop/completion exclusions retained pending identities/reasons. Configuration merge and modal independence passed.
- **Limits:** No new interactive color/demo, real-account quiet-time/burst, live timezone, banner/audio, clean-Mac or Intel runtime evidence. Older ledger readers reject the added attempt value; deleting history is not a downgrade path. Session rate windows reset on takeover/restart while durable deduplication persists.

## v0.2.5 — notification ownership and recovery (#18)

- **Scope:** 2026-10-26 PDT; v0.2.5 candidate and ownership/recurrence review corrections.
- **Method:** Local macOS arm64, Go 1.27.3; synthetic worker/store/subprocess tests, blocked senders, corrupt/full ledgers and intervening owners. Actual demo PTY at 60 × 18/no color, 120 × 32/color and 59 × 17/restoration; rendered owner/standby/observer/paused/capacity/recovery states at both supported sizes. Demo printer delivery stayed off.
- **Result:** Inspected ownership, corrupt-ledger pause/recovery and stable activity selection. Deterministic regressions reproduced idle-overflow lease retention; recurrence suppression behind cancelled senders or muted-observer queues; duplicate recurrence after an unseen intervening owner; and stale Observer role on retained-lease delivery. Corrected cases passed thirty race-enabled repetitions each with related ownership tests. Synced random acquisition tokens distinguished continuity from unseen handoff without account/PID metadata. Docs/formatting, pinned lint, full race/PTY tests, vet, build, vulnerability scan and four-target ad-hoc packaging passed.
- **Limits:** Concurrency/standby evidence uses synthetic workers, not two live sessions. Recovery retained consumed identities and did not prune capacity. No new native banner/audio, real-account concurrency, hardware, clean-Mac or distribution signing evidence.

## v0.2.4 release preparation

- **Scope:** 2026-10-26 PDT; v0.2.4, consolidated license notices.
- **Method:** Local macOS arm64, Go 1.27.3; pinned tools and default `make release`.
- **Result:** Docs/formatting, Go/workflow/Make lint, fault audit, vulnerability scan, race tests, vet and all four archive/install-guide/helper/checksum checks passed. Notice-preservation evidence follows.
- **Limits:** Local ad-hoc candidates; no new native/hardware or signed-distribution validation.

## Unreleased — consolidated release notices

- **Scope:** 2026-10-26 PDT; VERSION 0.2.3, local candidates for the subsequent 0.2.4 change.
- **Method:** macOS arm64, Go 1.27.3; 42 Python tooling tests and comparison against the downloaded published v0.2.3 macOS archive; default four-target release packaging.
- **Result:** All 22 earlier notice files appeared verbatim inside `THIRD_PARTY_NOTICES.txt`; consolidated bytes matched across targets and excluded `third_party/`. Tests covered complete LICENSE/COPYING/NOTICE text, target-module union, ordering, duplicate/conflicting modules, CRLF/non-ASCII, paths with spaces, Homebrew fallback and absent/empty notices. Docs, shell syntax, inventory/install-guide/helper/checksum checks passed.
- **Limits:** Candidates did not replace public assets. Application/UI and signing tooling were unchanged; no native delivery or hardware revalidation.

## v0.2.3 release preparation

- **Scope:** 2026-10-26 PDT; v0.2.3 Notification activity release preparation.
- **Method:** Local macOS arm64, Go 1.27.3; pinned tools and default four-target packaging.
- **Result:** Docs/formatting, Go/workflow/Make lint, fault audit, vulnerability scan, race tests, vet and archive/install-guide/helper/checksum checks passed.
- **Limits:** Ad-hoc candidate checks; activity inspection is separately scoped below. No new hardware, real-account or native-presentation evidence.

## Unreleased — notification activity (#20)

- **Scope:** 2026-10-26 PDT; VERSION 0.2.2, activity implementation before 0.2.3 preparation.
- **Method:** macOS 27.0 arm64, Go 1.27.3; synthetic decision/storage/backend fixtures and actual demo PTYs at 60 × 18/no color, 120 × 32/color and 59 × 17/restoration.
- **Result:** Empty/muted/grouped detail and corrupt-ledger pause were inspected; separate rendered fixtures covered Accepted/Failed/Unconfirmed/capacity. Outcome reasons, bounded history, stable selection, paging, backend privacy and real CLI ownership/quit tests passed. Docs/formatting, pinned lint, race tests, vet, build and vulnerability scan passed. Demo printer native delivery remained off; no native requests.
- **Limits:** No hardware, real-account persistence/region changes, clean-Mac, signing/notarization or banner/audio revalidation.

## v0.2.2 release preparation

- **Scope:** 2026-10-26 PDT; v0.2.2 complete Config controls.
- **Method:** Local macOS arm64, Go 1.27.3; pinned tools, full race suite including Config CLI PTY and default four-target packaging.
- **Result:** Docs/formatting, Go/workflow/Make lint, fault audit, vulnerability scan, vet and archive/install-guide/helper/checksum checks passed.
- **Limits:** Local ad-hoc candidates; feature inspection below adds no hardware/real-account confirmation.

## Unreleased — complete Config controls (v0.2.2 scope)

- **Scope:** 2026-10-26 PDT; VERSION 0.2.1, implementation before 0.2.2 preparation.
- **Method:** macOS 27.0 arm64, Go 1.27.3; synthetic config/store/printer fixtures; real CLI PTY editing/reload; demo at 60 × 18/no color, 120 × 32/color and 59 × 17/restoration.
- **Result:** General, pending region restart, percentage validation, all global/default/per-printer settings, absent printers, reset, body save failure and helper footer were inspected. PTY persisted edits across all three sections and rejected `0.5`; tests covered immutable/coalesced merging, protected files, live reconciliation, retry and shutdown. Docs/formatting, pinned lint, race tests, vet, build, vulnerability scan and four-target packaging passed.
- **Limits:** No real-account region change, clean-Mac, native banner/audio or hardware checks; signing/notarization was not repeated for this UI change.

## v0.2.1 release preparation

- **Scope:** 2026-10-26 PDT; v0.2.1 lean archives and `macos` download names.
- **Method:** Local macOS arm64, Go 1.27.3; pinned tools and default four-target packaging.
- **Result:** Docs/formatting, Go/workflow/Make lint, fault audit, vulnerability scan, race tests, vet and archive/install-guide/helper/checksum checks passed.
- **Limits:** Final candidate was ad-hoc. The preceding signed packaging experiment below is separate; hardware and fresh-Mac interactive validation were not repeated.

## Unreleased — lean runtime archives (#66)

- **Scope:** 2026-10-26 PDT; VERSION 0.2.0, local packaging candidates for 0.2.1.
- **Method:** macOS 27.0 arm64, Go 1.27.3; 37 Python tooling tests, default packaging and a second `make release` using an existing Developer ID identity/notary profile.
- **Result:** All four archives passed inventory/install-guide/checksum checks; both macOS CLIs and universal helper passed actual Apple notarization, extracted identity/team/runtime/timestamp checks, helper stapling and online CLI notarization. Tests rejected unsafe/duplicate/unexpected/missing members and stale guidance. Full docs/screenshots were absent from runtime archives. Docs, workflow lint, shell syntax and whitespace checks passed.
- **Limits:** No UI/telemetry changes or new banner/audio, browser-download/clean-Mac, Intel/Linux runtime or hardware checks. Signing experiment was a candidate, not publication.

## v0.2.0 release preparation

- **Scope:** 2026-10-26 PDT; v0.2.0, signed workflow commit `380fa3cdedf2f9b973604cae6f80091de70d3a80`.
- **Method:** Local macOS arm64, Go 1.27.3; default packaging plus [signed run 40358474367](https://github.com/cliffom/bambustand/actions/runs/40358474367), downloaded artifact digest/checksums and extracted native binaries.
- **Result:** Local checks passed formatting, pinned lint, vulnerability scan, race tests, vet, build and docs. Hosted signing/notarization, archive verification, cleanup and uploads passed; publication was skipped. Download digest and all four checksums matched. CLI/helper metadata reported 0.2.0; Developer ID/runtime/timestamp, online CLI notarization, helper staple and local Gatekeeper checks passed. Extracted arm64 `doctor --notify` submitted a silent request accepted by macOS without Bambu access.
- **Limits:** Acceptance is not banner/audio confirmation. No browser download on a fresh Mac, interactive Intel delivery or real-printer check; successful manual workflow does not establish tag publication.

## Unreleased — GitHub Actions release signing

- **Scope:** 2026-10-26 PDT; manual workflow commit `da79de142122c624f59b335f595836b47c699d01`.
- **Method:** Seventeen local synthetic setup/cleanup tests; [hosted macOS run 40351551784](https://github.com/cliffom/bambustand/actions/runs/40351551784) after provisioning six repository secrets; downloaded artifact and extracted arm64 checks.
- **Result:** Tests covered malformed/missing secrets, permissions, wrong identities, failure cleanup, secret-free errors and explicit Keychain use. Workflow/Make lint, formatting/docs passed. Hosted scan/race/vet, identity import/profile validation, signing/notarization, extracted checks, cleanup and uploads passed. Download digest/four checksums matched; extracted CLI reported `da79de1`, passed online notarization; helper staple/Gatekeeper returned Notarized Developer ID. Publication was skipped. Diagnostic artifact contained allowlisted notarization JSON, not credential files.
- **Limits:** Local OS/toolchain versions were not recorded in this entry. No fresh-Mac first run, Intel runtime or version-tag publication-path validation.

## Unreleased — Developer ID signing and notarization

- **Scope:** 2026-10-26 PDT; development 0.1.15 on base `1c660b0f1c62`, then notarization on base `e63cbd7`.
- **Method:** User's existing Developer ID Keychain identity and validated notary profile; both macOS CLIs and universal helper, ad-hoc and signed four-target packaging, eight synthetic failure-path regressions.
- **Result:** Matching team/certificate/runtime/timestamp/integrity checks passed. Apple returned Accepted; helper stapling and extracted CLI online notarization passed. Extracted arm64 CLI ran `--version`; helper Gatekeeper assessment returned Notarized Developer ID. Standalone CLI app-style `spctl` assessment rejected it as not an app; the appropriate codesign notarized requirement passed. Cold Intel ticket lookup was corrected by forcing online lookup before the requirement check. Formatting, Make lint, race tests, vet, docs and packaging passed.
- **Limits:** Reports were in ignored `dist/notarization/`, not a retained repository artifact. No downloaded/quarantined clean-Mac or Intel runtime test. CI credentials were not yet provisioned at this stage; later hosted-run evidence establishes that separate result. No release was published.

## Unreleased — Synthetic macOS notification probe

- **Scope:** 2026-10-25 PDT; development 0.1.15 plus notification changes, with revision-specific observations below.
- **Method:** macOS 27.0.1 (26A434) arm64; actual helper/OS submissions, synthetic demo telemetry, temporary notification state and PTY/emulator inspection. User reports/screenshots are identified separately from command acceptance and automated fixtures.
- **Result:** Helper delivery, permission/Focus handling, Config persistence/feedback, worker decisions and identity regressions are scoped below. Formatting, pinned lint, full race/UI/CLI tests, vet, build and docs passed at the recorded stages.
- **Limits:** These synthetic/local OS observations do not establish real-printer incidents/completion, real-account persistence, clean-Mac distribution or Intel runtime. Terminal cell inspection is not physical-terminal/font review.

### Config modal and immediate notification preferences

Base `0031e6f78db7`, Go 1.27.3: demo PTYs checked immediate desktop/sound/completion changes and saved config, preserving printer overrides/unrelated fields. Save errors were induced with temporary invalid/restored TOML. Final silent-save behavior was checked on base `153f66b59a87` at 60 × 18, 120 × 32 and 59 × 17/restoration: success/pending were silent, failure text remained in the body with retry, and decoded failure RGB `ff8087` matched the danger style. Earlier static Saved/Saving presentations were superseded. Regression tests retained immutable/coalesced saves and shutdown tracking; slow-save appearance had reducer coverage, not timed visual capture.

### Config notification test action

Base `52cbb7925bbd`, Go 1.27.3: actual demo PTYs at 120 × 32, 60 × 18 and 59 × 17/restoration with sound/automatic delivery off. Both explicit requests were accepted, config bytes stayed unchanged and processes exited normally. Tests covered pending/result states, duplicate blocking, blocked-probe telemetry, cancellation/stale results and hidden guards. Full checks passed. Banner/audio, first permission prompt, color and real long-running/failure layout were not inspected; fake processes covered failure paths.

### Config selected-option helper text

PTY inspections on bases `da6f4fc5a308`, `e392734c7ef2` and `461f1065aefd` at 60 × 18, 120 × 32 and 59 × 17/restoration checked option helpers, header close controls, injected-save-failure body text and retained selection. The final silent-save inspection above supersedes intermediate successful-save labels. No notification was submitted by these helper-text inspections.

### Transient Config test feedback

Base `bcc5bcc80ff7`: two silent tests were accepted with automatic delivery off. PTY captures at 120 × 32 and 60 × 18 showed pending/accepted, dimmed and expired footer feedback; 59 × 17 blocked hidden activation. Clock-driven tests verified bold green acceptance after helper/dot, dimming at four seconds, expiry at seven, and persistent red failure. Decoded colors changed `a3e3bb` → `929ba8`; actual banner/audio/color appearance was not independently inspected.

### Printer-name titles and preview diagnosis

A silent `Demo: X2D · Studio` request was accepted, but the user reported generic “Notification” text. Helper status showed authorization/alerts/sound enabled and effective previews Never; screenshot showed Default/Temporary settings. After the user changed preview settings, a screenshot confirmed the branded printer title and test body. Worker title bounds and helper empty/oversized/wrong-type rejection passed. This establishes one visible synthetic title/body, not automatic hardware delivery.

### Co-reported desktop notification grouping

NotificationCenter logs showed three requests at 22:40:34.568/22:40:35.570 and a separate completion at 22:41:04.689. Grouping regressions then verified one request for three same-report intervention notices, per-member reservations/results, muted exclusions and printer/completion separation. Actual demo activity at 120 × 32 reported Accepted 1 / Failed 0; the process was interrupted with its alert open. Grouped-banner presentation/duration was not confirmed. The burst evidence did not trace banner preemption/expiry; later #22 covered timed grouping separately.

### Live-session delivery worker (synthetic and local demo)

Actual demo PTYs showed accepted intervention/completion requests at 120 × 32, preferences/missing-helper failure at 60 × 18 and 59 × 17/restoration. An initial harness omitted cursor-probe responses and produced incomplete captures; the corrected harness reached the expected states. A separate user-run demo confirmed a visible first banner and automatic modal; completion/audio/banner duration were not confirmed. Race tests covered deduplication, snooze/shutdown cancellation, policy changes, overflow, storage/submission failures and per-printer inheritance. No real printer/account was used.

### Notification identity foundation (synthetic)

Synthetic reports covered restart/reconnect replay, partial/null telemetry, family-specific recurrence, completion baselines, matching/ambiguous job identities and progress/ETA non-completion. Child processes verified lock contention and reservation persistence. Store failures, privacy, unsupported/corrupt state and capacity were checked. Persist-before-send permits lost notifications after reservation crashes and suppresses ambiguous attempts; it does not establish exact-once banners or hardware job-ID uniqueness.

### Packaged macOS helper

On 2026-10-25, all four candidate archives passed inventory/metadata, extracted helper architecture/signature and help checks. The ad-hoc arm64 extraction's `bambu-tui doctor --notify` registered and submitted a silent accepted request. This is local extraction evidence; at this stage no Developer ID identity was configured and signing/notarization/clean-Mac/Intel runtime remained unverified. Later signing entries provide separate evidence.

### Doctor integration with the branded helper

Base `3c25826bae52` plus uncommitted changes: integrated silent request returned zero. User confirmed silent and sound-requested doctor commands worked, including unfocused runs after a five-second delay. A separately identified focused-terminal test was not reported. Fake-process tests covered adjacent/symlink/path-with-spaces lookup, missing helper, cancellation and permission/timeout guidance; copied CLI correctly refused to find a helper in the source cwd.

### Branded native helper

Swift 6.4 PoC in the Codex command environment on the same host passed plist, ad-hoc signatures, registration, ICNS decoding and status/help. User screenshot confirmed name/logo in permission UI; initial request was denied and status reported denied. After permission was allowed, requests were accepted, but no banner was initially observed. Native logs showed Focus suppression with delayed delivery. User later confirmed branded banner delivery and attributed the earlier absence to Focus. Exact Focus change, focus state and audible behavior were not recorded. Help/invalid/SSH cases and byte-preserved icon payloads passed; the test process exited after submission.

### Original AppleScript probe (superseded)

Base `3c25826bae52` plus uncommitted changes: `./bin/bambu-tui doctor --notify` returned zero in the Codex command environment. User screenshot/report confirmed a Bambu TUI banner with generic script-host icon. Focused/unfocused and sound behavior were not reported for this backend. Synthetic command/error/cancellation/platform tests passed. The branded helper later replaced this path; its results do not retroactively validate the AppleScript backend's unperformed checks.

## v0.1.8 — Live job transitions and connection recovery

- **Scope:** User screenshot/report dated 2026-10-25; v0.1.8; user confirmed router restarts. H2D firmware `01.03.00.00`; host OS/terminal not recorded.
- **Method:** Read screenshot session events and displayed report timestamps; no raw telemetry trace retained.
- **Result:** H2D finished → prepare at 09:56:18, prepare → printing at 09:56:29. At 11:11:06 it showed 11%, layer 74/896, last report 11:11:05. Cloud disconnects resolved over 09:51:40–09:51:41 and 10:46:18–10:48:17. Two offline notices at 10:48:17 resolved at 10:48:28/10:48:31. Capture showed Cloud Connected, three Live links, zero unavailable/active notices and retained resolved history.
- **Limits:** Report did not map individual router restarts to intervals or isolate monitor from printer connectivity. Screenshot does not establish physical printer disconnection, automatic prompts, saved decisions, pause/completion or notification/sound delivery.

## v0.1.8 — Idle telemetry grace period

- **Scope:** Screenshot/report dated 2026-10-25 of overnight v0.1.6 session; informed v0.1.8's ten-minute idle/finished threshold.
- **Method:** Compared displayed last-report/resolution times for one printer: 03:09:02 → 03:18:33, 02:57:12 → 03:01:51, 03:18:35 → 03:21:27.
- **Result:** Apparent gaps were 9m31s, 4m39s and 2m52s. Synthetic regressions checked provisional ten-minute idle/finished versus two-minute other-state boundaries, partial-report recovery, transitions and unavailable filtering.
- **Limits:** Not a captured cadence trace; resolution can reflect availability/connection changes, and state during each gap was unknown. Host/firmware were not recorded for this observation. Actual interval validation remains in [Remaining live checks](#remaining-live-checks).

## v0.0.5 — User-reported live validation

- **Scope:** User confirmation after v0.0.5; H2D and X2D displayed telemetry. Exact date, firmware, host OS and terminal were not supplied.
- **Method:** User-reported observation; no retained field-by-field comparison or raw trace.
- **Result:** User confirmed displayed telemetry on both models, extending earlier H2D screenshot evidence.
- **Limits:** Confirmation explicitly excluded login/restart, reconnection, persisted decisions and action-needed modals. It does not establish comparison of every field against Bambu Studio or live firmware-query correctness.

## v0.0.1 — Observed live

- **Scope:** User screenshot dated 2026-10-23, initial release period; H2D active print, three bound printers. Firmware/host OS/terminal not recorded.
- **Method:** Screenshot/report plus subsequent normal-terminal disposable credential probe; no raw account data retained.
- **Result:** Email-code login/discovery and displayed job/progress/time/layers, both nozzles, bed/chamber, AMS/AMS HT/material estimates, HMS and low-filament notices were observed. Credential saving initially failed and dashboard used session-only login; later `bambu-tui doctor` save/read/delete succeeded in a normal terminal with a disposable entry.
- **Limits:** Connectivity/display observation is not complete field/model validation or a saved real-account login/restart check. No credentials or tokens were retained.

## Automated checks

- **Scope:** Historical automated coverage and native credential-probe outcome; see [contributing](contributing.md#test-evidence) for current procedures.
- **Method:** Synthetic H2D/X2D reports, fake cloud/native services, in-memory MQTT, temporary private stores, race/PTY tests and platform packaging.
- **Result:** Coverage includes report merging, condition/decision lifetime, auth challenges, reconnect and error handling. A native credential probe in the restricted implementation environment failed with `security` exit 152; follow-up lookup found no leftover diagnostic entry. The normal-terminal probe on 2026-10-23 later passed, as recorded above.
- **Limits:** Coverage description is not a fresh pass report. Restricted-environment failure versus normal-terminal success does not establish saved real-account persistence. New checks must identify the actual revision and environment.

## Remaining live checks

This is the maintained list of outstanding validation, not a release gate automatically satisfied by issue closure or publication. Prior observations remain in their scoped entries above. Current tracking: [#38](https://github.com/cliffom/bambustand/issues/38) (operational), [#23](https://github.com/cliffom/bambustand/issues/23) (native notifications/distribution), [#139](https://github.com/cliffom/bambustand/issues/139) (browser/device coverage), [#39](https://github.com/cliffom/bambustand/issues/39) (performance). Issue state was checked on 2026-10-31; #23/#38 retain their recorded deferral behind usability work.

### Accounts, telemetry and interaction (#38)

- [ ] Save an email-code login, quit, restart without login, then logout and verify next startup requests login; record native-store and account/region behavior.
- [ ] Verify current-release interruption/reconnection and fresh telemetry after recovery; distinguish monitor-only from router/printer outages. Historical v0.1.8 recovery remains scoped above.
- [ ] Compare H2D/X2D progress, time, temperatures, active filament paths, installed OTA firmware and offered update status with printer/Bambu Studio readings; record firmware and actual fields compared.
- [ ] Observe pause/resume and telemetry-confirmed completion, including job identity/reprints; elapsed time/progress alone must not establish completion.
- [ ] Validate global/per-printer preferences, snooze/resume/expiry and restart persistence; preserve distinctions between local decisions and physical fault resolution.
- [ ] Acknowledge/hide notices, restart and verify persistence; observe natural clear/recurrence returning unreviewed without routine-report replay.
- [ ] Observe natural intervention prompts and noninterrupting recovery/maintenance while telemetry continues through dialogs.
- [ ] Exercise filters/sorting, keyboard/modal routing and resizing in a physical terminal during telemetry updates; record terminal/font/color profile and sizes.
- [ ] Inspect a live local diagnostic export before voluntary sharing; retain only sanitized evidence.
- [ ] Measure actual idle/finished report intervals and stale/recovery/printing transitions to assess the provisional ten-minute threshold.

### Native notifications and distribution (#23)

- [ ] Verify current native identity/permission continuity, focused/unfocused automatic delivery, sound-off/sound-requested audio, denial and Focus suppression on a local desktop.
- [ ] Verify real-printer intervention/completion, category/review/snooze/quiet behavior, grouping and restart deduplication; distinguish accepted requests from visible/audible delivery.
- [ ] Download a signed/notarized archive through a browser on a fresh Mac, inspect quarantine/Gatekeeper and first run; separately exercise Intel runtime. Earlier local extraction/hosted-signing results do not complete these checks.
- [ ] Record SSH/headless/exceptional-shutdown behavior and Linux runtime/Secret Service results; do not infer support for native delivery in unsupported environments.

### Browser/device coverage and performance (#139, #39)

- [ ] Check current Firefox/WebKit/Safari and physical-mobile layout, keyboard/touch, focus/scroll, notification permission/presentation and lifecycle; record exact versions/device sizes and separate automation from human OS observations.
- [ ] Measure representative large-farm updates/rendering/navigation and notice/history loads, including timing and allocations before proposing optimization.

Do not deliberately fault a printer for validation. Record unavailable scenarios explicitly and leave them unchecked. Each completed check needs a scoped evidence entry with version/revision, date, environment, method, results and limitations; never attach credentials or private identifiers.
