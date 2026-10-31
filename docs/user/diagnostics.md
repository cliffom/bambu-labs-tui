# Local diagnostic export

[Back to the README](../../README.md)

Select a printer and press uppercase `D` from the dashboard. The preview shows the local destination and export scope. Press Enter to create that file, or Esc to cancel. There is no automatic export, upload, issue attachment, or support message. Close this view before quitting.

The export stays attached to the requested printer if telemetry changes the visible filter selection. If discovery removes that printer, confirmation fails without creating a synthetic printer. Telemetry continues while the preview is open, and new attention prompts wait until it closes. A completed export is a snapshot from confirmation time.

## Included data

The versioned JSON schema includes:

- App release version/commit and operating system/architecture. Hostname, environment variables, username, paths, and configuration are excluded.
- One printer aliased as `printer-1`. Only the exact recognized H2D/X2D model labels and numeric four-component firmware versions are retained; other model/version strings are omitted or labelled unknown.
- Cloud connection status, the printer's fixed connection-state label, relative print-report age, and protocol fault codes. Free-form connection errors and printer fault messages are excluded.
- A merged snapshot of classified print fields and this printer's samples from the last **16 sanitized reports across the farm**, in receipt order. Older samples may have been displaced by other printers. Malformed or oversized reports retain only a fixed status, without their body or parse-error text.
- Fixed normalized temperature, progress, remaining-time, layer, nozzle/path, AMS, and external-slot fields, recomputed from the sanitized merged snapshot. These describe only the retained data; they are not a complete copy of live internal state.

`values` contains a fixture-ready `print` / `info` envelope. `structure` records the types and acceptance of **known** fields, preserving evidence such as a numeric string, a null, or an unexpected object. Unclassified field names are omitted; only an omitted-field count remains. Rejected values are omitted. Invalid array entries use null placeholders to preserve positions. Arrays and processing are bounded; truncation is visible through omitted counts. Bodies larger than 1 MiB are not captured for diagnostics.

Classified values cover state/command enums, bounded numeric telemetry, packed temperatures/path flags, HMS attributes and error codes, standard material enums and hex color values, AMS unit/slot indices, and OTA metadata. See the explicit [schema](https://github.com/cliffom/bambustand/blob/v0.1.0/internal/diagnostics/schema.go) and [document builder](https://github.com/cliffom/bambustand/blob/v0.1.0/internal/diagnostics/diagnostics.go). Internal unit/slot indices are retained; hardware serials and spool identifiers are not.

Unknown fields, arbitrary strings, URLs, authentication headers, tokens, email/account identifiers, printer/AMS serials, printer names, job/file names, job identifiers, spool UUIDs, custom material labels, raw errors, and review/snooze files are excluded. A private value nested inside a known field with an unexpected type is rejected rather than recursively dumped. The exporter uses an allowlist, not a general promise to recognize every kind of secret in arbitrary text. Recognized operational measurements and colors remain data about your printer, so inspect the artifact before sharing it voluntarily.

## File location and errors

Normal non-demo launches migrate previous cache exports without replacing conflicting destination files; see [upgrade recovery](usage.md#upgrading-from-bambu-tui).

Exports go to `com.suitepotato.bambustand/diagnostics/` under the OS user cache directory. The preview shows the exact path, including a generic timestamp filename. The artifact contains no local destination path and no original printer identifier. New directories use mode `0700`; files use `0600`. Existing files and output symlinks are never overwritten. A failed write removes its incomplete file; a failure before file creation leaves the destination untouched.

After success, inspect the JSON with an editor or a local viewer, for example:

```sh
python3 -m json.tool /path/shown/in/the/preview.json
```

If export fails, the UI shows a fixed error message without echoing raw error text. Check the displayed directory's accessibility and permissions. Close with Esc and press `D` again to choose a new timestamp filename. If you close the preview while writing, the confirmed local write can still finish and report its result in the dashboard.

Demo mode supports export too, using only simulated printer data. It still requires `D` and Enter. Exports have no automatic retention/cleanup policy; remove files when you no longer need them. The repository ignores the generic diagnostic filenames to avoid accidental commits of copied artifacts.

## Voluntary bug reports and synthetic fixtures

1. Reproduce the display/normalization issue while the TUI is open, then select that printer and export.
2. Inspect the file. Check the retained operational data and omitted/truncated/type-rejected fields before sharing.
3. Attach it yourself to a bug report if you choose. Describe the expected behavior separately; do not paste credentials or original raw reports.
4. To build a regression fixture, copy `merged_snapshot.values` or a relevant `recent_reports[].values` envelope into a synthetic fixture. Replay partial reports in their listed order against a synthetic printer ID such as `TEST`. Keep the `structure` evidence when investigating omitted or malformed input types.

Sanitized snapshots cannot reproduce omitted custom strings or arbitrary unknown telemetry. Create representative synthetic values for such cases rather than adding original private data to the repository.
