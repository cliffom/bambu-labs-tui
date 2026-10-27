# Farm summary, filters, and sorting

[Back to the README](../../README.md)

The dashboard summary always describes the **whole account farm**. Filtering changes only the printer list and its selected details. Notices, event history, saved reviews, snoozes, and automatic alert eligibility are unchanged.

The bottom dashboard row uses a contrasting background and bold hotkeys to separate navigation hints from printer data. Hints show `↑/↓` for selection or scrolling; `j` / `k` remain available. Toasts and storage warnings can temporarily replace the hints.

## Summary definitions

| Count | Meaning |
| --- | --- |
| Printing | Printers whose last reported state is `RUNNING` |
| Paused/failed | Printers whose last reported state is `PAUSE` or `FAILED` |
| Idle/done | Printers whose last reported state is `IDLE` or `FINISH`; completion is reported, never inferred from progress or time |
| Attention | Printers with at least one visible, unacknowledged actionable notice under the existing verified intervention/failed/ambiguous-pause classifier |
| Unavailable | Printers whose cloud connection is disconnected, whose device is offline, or whose print telemetry is missing or stale (older than ten minutes for reported `IDLE`/`FINISH`, two minutes for all other states) |
| Notices / unreviewed | The existing visible active-notice and unacknowledged-notice counts; these count notices rather than printers |

State counts use the last reported state even when the link is unavailable. Availability and attention can overlap the state groups, so summary counts are not intended to add up. Printers with unknown/other states remain in the total but are not called idle or finished. Very large counts use compact labels (`stopped` for paused/failed, `attn`, `unavail`, and `unrev`) to keep the summary readable. Snoozed/muted printer counts appear when space permits; each printer's alert status is always available in details or via `s`.

Attention is independent of the automatic-interruption preferences. A snoozed printer, or one with automatic modals disabled, still needs review when its actionable notice is unacknowledged. Low-filament estimates, maintenance, severity labels alone, and offline/stale notices do not independently imply an intervention alert. Acknowledgement/dismissal removes the corresponding occurrence from actionable review; remaining independent actionable notices can keep its printer in Attention. See [alert behavior](action-needed.md) and [interruption preferences](alerts.md).

## Filters

Press uppercase `F` from any dashboard panel to cycle:

1. **All** — every discovered printer (the default).
2. **Attention** — printers counted as needing attention above.
3. **Printing** — printers last reported as running, including those currently unavailable.
4. **Unavailable** — disconnected, offline, waiting-for-data, or stale printers.

The printer list always shows its active filter, visible/total count, sort, and `F` / `S` shortcuts plus `0 reset`, including at 60×18. The summary stays unfiltered. Empty results explicitly say that no printers match and show no selected printer details. Press `0` to reset to All printers / Name sorting, or continue pressing `F` to change the filter; no reset of notices or reviews is needed.

## Sorting

Press uppercase `S` to cycle:

| Sort | Ordering |
| --- | --- |
| Name | Case-insensitive printer name (the default) |
| Attention | Unreviewed actionable printers first, then other paused/failed printers, unavailable printers, running printers, and other states |
| State | Alphabetical by displayed state label, ignoring case |
| Finish | Earliest comparable estimated finish first; missing, paused, finished, and stale/unavailable estimates follow |

Every sort first groups printers in this order:

1. **Active / non-idle:** online printers whose reported state is running, preparing, slicing, paused, or failed. Unknown, missing, or unrecognized states also stay in this group for ordering only; they do not establish activity.
2. **Inactive / idle:** online printers with reported `IDLE` or `FINISH` state.
3. **Offline:** devices reported offline, regardless of their last-known state.

The selected sort applies within each group. The local web dashboard uses the same grouping with its Name, Attention, and State sorts; browser filters/sorts remain session-local. Grouping uses the device online flag and the existing reported state, independently of telemetry freshness or cloud connectivity. Stale states remain last-known, missing states remain unknown, and a disconnected cloud link does not infer that a printer is offline or idle. Telemetry and device updates refresh the ordering while preserving the selected printer's identity, panel focus, and details scroll.

Unselected offline rows use the warning color (amber in the default theme). Selected offline rows use the theme's contrasting selection text/background colors. Offline rows show only the printer name and `Offline` link status; all telemetry columns are blank. Compact layouts show `Offline` in the state column because the link column is hidden.

Use `g` / Home after choosing a sort to jump to its first printer. All ties use case-insensitive name and then stable printer ID. Duplicate names do not combine printers or their notice counts.

A comparable finish estimate requires a running printer, remaining-time data, live cloud/device telemetry, and a remaining-time report no older than 120 seconds. Temperature-only updates do not refresh the age of the time estimate. Last-known estimates remain visible with `~` but are grouped with unknown estimates in Finish sorting. Paused and non-running printers show no finish ETA. Estimates on another date include the weekday in the printer list and a fuller date in details. A past-due estimate does not imply completion.

## Selection and navigation

The dashboard sections are labeled **Printers**, **Printer Details**, and **Notices**, without numeric prefixes. A `›` marker identifies the focused panel, including when color is unavailable. Use `Tab` / `Shift+Tab` to move focus between panels.

Updates may reorder rows, but selection stays attached to the same printer ID while it still matches the filter. Printer details retain their scroll position when that identity stays selected, subject to normal viewport limits after resizing. Renaming a printer does not change its identity.

If the selected printer is removed or stops matching, selection falls back to the nearest remaining row and details start at the top. An empty list has no selected printer. Panel focus is preserved, and regular `j/k`, arrows, and `g/G` or Home/End navigation operates on the visible list. Filter and sort choices are saved and restored on restart as described below.

An action-needed modal still opens for an eligible printer hidden by a filter. Its explicit `p` / Enter navigation resets an excluding filter to All so the requested printer can be selected; the chosen sort remains. That All filter is saved too. Other dialogs keep their existing printer/notice identity and key handling.

## Saved view preferences

`F` and `S` save the selected filter and sort in the active TOML configuration (`--config` or the path printed by `--config-path`). Dashboard `0` resets both to **All / Name** and saves that choice; it works from any dashboard panel, including an empty filtered list. Dialogs keep their own key handling. Below 60×18, hidden view changes are blocked until the terminal is resized.

```toml
[view]
filter = "all" # all, attention, printing, unavailable
sort = "name" # name, attention, state, finish
```

Scope follows the configuration path, across accounts, regions, and demo sessions. Use a separate `--config` file for a separate view. Only the two view choices are saved: no selected printer, cached telemetry, notices, reviews, or snoozes. Missing, unsupported, and non-string choices fall back independently to All or Name. A fallback does not write the file on startup. Other malformed TOML or unknown configuration keys still report configuration errors.

Saves run asynchronously through the same private atomic configuration writer as Config edits. Rapid changes coalesce into the latest complete snapshot; unrelated external settings survive, and an edited field wins a conflict. Successful writes are silent and normalize TOML/remove comments. Failures leave the current view active and show a persistent Config save warning; open `,` and press `r` to retry, or make another preference change. Ticks and navigation do not retry failed writes. Normal quit waits for pending writes and offers the existing exit/cancel choice after failure or timeout. Concurrent TUIs keep independent live views; the last successful save of an edited field governs the next launch.
