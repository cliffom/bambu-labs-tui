# UI direction

[Documentation index](../README.md) · [Architecture](architecture.md) · [Documentation direction](documentation-direction.md)

## Product and visual direction

Keep the interface a compact, keyboard-operated farm monitor. Operators should be able to find unavailable or troubled printers, understand the reported condition, and inspect details without losing their place as telemetry arrives. Preserve read-only behavior unless the user explicitly changes that product scope.

Extend the existing dashboard hierarchy: version/connection header, farm summary, printer list, printer details, and notices. Put detailed inspection or local configuration in the existing dialog system. Reuse the shared styles in `internal/ui/model.go`, colors and layout policies in `internal/ui/values.go`, table widths in `internal/ui/layout.go`, and version helpers in `internal/ui/version.go`. Keep the green accent, muted secondary text, amber warnings/offline rows, red errors, and visible selected-row background consistent. Introduce a new style or layout policy only for a clear operator need, and explain it in the PR.

Color supplements text. Preserve readable state labels, fault codes, focus/selection markers, and unknown/last-known qualifiers when color is unavailable. Treat printer names and other external strings as untrusted terminal text; use existing sanitization and ANSI-aware width/wrapping helpers. Avoid decorative content that consumes space needed for monitoring or navigation.

Use a full-width slate background for the bottom dashboard and full-screen notice footer, with a one-column horizontal inset. Use bold, bright dashboard hotkeys with secondary labels and keep dashboard warnings amber; preserve the footer's single-row height and readable controls without color.

## Interaction and layout contracts

- Reuse key meanings from `internal/ui/keys.go`. Keep dashboard help, dialog hints, and the [keyboard reference](../user/usage.md#keyboard-and-navigation) consistent with actual routing. Context-specific reuse is allowed when the active dialog makes the action clear.
- Give one dialog explicit ownership through `dialogState` and `routeFor`. Keep input, rendering, synchronization, cleanup, and any suspended parent consistent. Telemetry and asynchronous results continue during dialogs; automatic attention waits for the current owner to finish.
- Keep Escape's close/return behavior and the existing dashboard-only quit behavior. Confirm destructive local bulk actions, preserve newly arriving occurrences during confirmations, and keep local notice decisions distinct from printer-side actions.
- Support the minimum 60 × 18 terminal. Below that size, hidden navigation, mutations, exports, and confirmations stay blocked; Escape, dashboard quit, and compact shutdown controls retain their documented behavior. Resizing back restores dialog state. Keep details/notices stacked below 105 columns and side by side at wider sizes unless a requested change deliberately revises this contract.
- Preserve selection by printer/notice identity through live updates, sorting, and filtering. Preserve focus and scroll where the selected identity remains; provide a defined fallback when it disappears. Show useful empty, disconnected, stale, unknown, saving, and failed-save states.
- The separate Config dialog (`,`) edits global notification defaults immediately and saves asynchronously to the launch configuration path. Preserve per-printer overrides, silent background saves, visible red failure/retry states, and shutdown tracking. Its explicit Test Notification action remains responsive, uses the global sound preference, shows pending/accepted/failed states, and cancels on close without consuming live notification identities. Config uses its single-row footer for selected-option helper text plus test feedback separated by a dot. Accepted feedback is prominent green, dims after four seconds, and expires after seven; test failures stay red. Shorten helper text before clipping result text at narrow widths. Show storage failures and retry guidance in its body; emphasize navigation/activation/close keys below its header.
- Desktop delivery is independently opt-in and must honor current categories, reviews, and snoozes while telemetry remains responsive in all dialogs. Show acceptance separately from visual delivery, preserve failures in the footer, and expose session activity in the existing alert dialog. Shutdown pauses/cancels delivery; canceling shutdown can resume it.
- Do not infer completion, resolution, runout, or connectivity from missing fields, elapsed time, color, or severity alone. Reuse the farm's interpretation and the distinctions documented in the user guides.

## Verification for a UI change

Extend the relevant behavioral tests when changing an interaction contract. Prefer semantic assertions, ANSI-aware bounds, synthetic telemetry, and explicit synchronization over full-frame snapshots or sleeps. Existing examples include `internal/ui/dialogs_test.go`, `modal_keys_test.go`, `resize_input_test.go`, `overview_test.go`, `offline_test.go`, and the [PTY suite](terminal-tests.md). `make test` runs these with the race detector in Linux/macOS CI.

For layout or styling changes, inspect the affected states in `make demo` at 60 × 18, below minimum size, and a wide size such as 120 × 32. Check selected/unselected rows, empty/unavailable data, dialogs, text without color, clipping, and focus/scroll after updates as applicable. Automated tests cannot establish visual quality in every terminal. Report which states/sizes were actually inspected and which remain unverified; do not claim a demo or synthetic fixture establishes real-printer behavior.

Existing automated guards make the interaction contracts verifiable:

| Contract | Regression coverage |
| --- | --- |
| Dialog ownership, return paths, and deferred attention | `internal/ui/dialogs_test.go`, `modal_keys_test.go` |
| Block hidden actions below minimum size; continue telemetry/saves | `internal/ui/resize_input_test.go` |
| Stable selection, filters/sorts, empty states, and bounds | `internal/ui/overview_test.go` |
| Offline text/color and selection across table widths | `internal/ui/offline_test.go` |
| Raw keyboard input, resizing, quit/signals, terminal restoration | `cmd/bambu-tui/pty_test.go` |

Keep these guards passing and extend them for new behavior rather than weakening assertions to fit a regression. Run `go test -race ./internal/ui ./cmd/bambu-tui` for the UI/terminal suites, or `make test` for all packages. Visual review and documentation impact remain required even when automated tests pass.

In the PR, describe the operator-visible before/after behavior, identify the contracts affected, link updated user guidance, and report regression tests and visual inspection results. An intentional departure from these defaults should be backed by the user's requested behavior, reflected in the direction/user guides, and covered by the relevant checks in the same change. Routine fixes within this direction do not need a separate approval step.
