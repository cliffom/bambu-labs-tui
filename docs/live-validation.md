# Current-release live-validation worksheet

[Validation record](validation.md) · [User guide](usage.md) · [Alert guide](alerts.md)

This is the manual procedure for [#38](https://github.com/cliffom/bambu-tui/issues/38). Its target is the v0.1.7 candidate. Automated tests, a demo run, or a published release do not establish a live pass. Record results in `validation.md`; this worksheet alone does not complete the issue.

## Identify the run

From the source checkout, build once and keep using that executable throughout the run:

```sh
make build
./bin/bambu-tui --version
```

Record the full version/build identifier, source changes if the checkout is dirty, test date, host OS, terminal application/version, and H2D/X2D installed firmware. Record offered OTA firmware if available. Use local aliases such as **printer A (H2D)** and **printer B (X2D)** in shared observations. Do not paste credentials, serials, account identities, job names, spool identifiers, configuration contents, or raw telemetry. Use the app's masked prompts for authentication.

Choose **Pass**, **Fail**, **Not observed**, or **Not run** separately for each check and model. A pass needs a short actual observation, not just a checked box. If a required situation does not arise, record the limitation and leave it unverified. Use normal printer operation; do not manufacture faults, interrupt printer power, or change the system clock to accelerate expiry.

## Saved session: login and restart

1. In a normal terminal, run `./bin/bambu-tui login --email-code`. Complete authentication locally and note whether the app confirms a saved session or falls back to session-only monitoring.
2. Start `./bin/bambu-tui`. Check that the expected account printers appear and reports arrive.
3. Close dialogs with `Esc`, then quit from the dashboard with `q`. Start the same executable again. A saved-session pass requires reaching live monitoring without reentering credentials. Session-only fallback is a separate observation, not a saved-session pass.
4. Record storage warnings and terminal restoration if any. Do the logout check last so other checks can use the saved session.

## Firmware and telemetry comparison

1. For each model, compare **Firmware** and **FW Status** with the printer or Bambu software. Record installed and offered OTA versions, the displayed assessment, and any unknown/last-known status. An unavailable update assessment stays unverified.
2. During a real print, compare reported state, progress, remaining time, layers, temperatures, and active filament paths with Bambu Studio/the printer. Note the observation time and sampling delay when values move. Cover both nozzle paths and standard AMS/AMS HT where available; mark unavailable paths explicitly.
3. Observe normal pause/recovery/completion when available. Verify completion follows a reported transition, rather than an elapsed ETA. Do not infer failure/runout coverage from a successful print.

## Local reviews across restart and recurrence

1. Choose an existing active notice; note its model, code/type, and first-seen time locally. Press `a` to acknowledge it. Allow routine reports to arrive and verify the same occurrence remains reviewed.
2. Quit normally and restart. After startup reports arrive, verify that acknowledgement remains. While awaiting relevant reports, the decisions view (`n`) may identify the saved decision as awaiting telemetry.
3. On an existing active notice, use `c` to dismiss locally. Verify routine reports and a normal restart do not restore the same dismissed occurrence; inspect it through `n` as needed.
4. If the condition resolves and later recurs during normal use, keep the TUI running to observe both transitions. Verify the saved decision is removed on confirmed resolution and recurrence returns unreviewed. Resolution/reappearance entirely while the app is closed cannot establish this check.
5. Restore visibility (`n`, then `r`) and mark unreviewed (`u`) as appropriate to restore your desired review state afterward. Avoid resetting all account decisions merely to run this check.

## Snooze, preferences, and actual expiry

1. Select a printer, press `s`, then `1` for a 15-minute snooze. Note the displayed absolute deadline. Underlying notices must stay visible and review flags must remain unchanged.
2. Quit normally and restart before that deadline. Verify the same deadline remains effective; restarting must not extend it.
3. Test early resume (`s`, then `u`) separately. With a visible, unacknowledged, eligible active condition and enabled preferences, verify automatic attention can re-arm after the batching delay. With no eligible condition, record only deadline/resume state, leaving interruption behavior unobserved.
4. Set another 15-minute snooze and let its real deadline pass while monitoring. Verify expiry clears the snooze and re-arms an eligible unreviewed condition. Do not edit stored files or the host clock as a substitute for live expiry.
5. Where an eligible condition is available, check global and per-printer interruption preferences using your local configuration and the [alert guide](alerts.md). Restart after configuration changes. Confirm muting keeps notices/telemetry visible and manual `!` inspection remains available. Keep configuration identifiers local and restore desired preferences after testing.

## Monitor reconnect

1. Start from live monitoring and note the displayed report freshness.
2. Temporarily interrupt only the monitor's network connection using an appropriate method for your host. Leave printer power, printer connectivity, and the running print untouched. If no suitable method is available, record **Not run** rather than substituting a fake transport or process restart.
3. Restore the monitor's connection and observe automatic reconnection without restarting the TUI. Confirm new telemetry, not merely retained values: watch freshness reset on incoming reports or state/progress values change where available and optionally request a fresh snapshot with `r`.
4. Record the interruption method, approximate outage/recovery duration, connection/freshness labels, and whether reviews/snoozes remained intact. Preserve failures as observations and file a separate defect with sanitized reproduction steps.

## Terminal behavior while live reports arrive

1. Navigate printer/details/notices panels; open and close help, inspector, AMS browser, decisions, and snooze controls where available. Verify selection and scroll remain usable as reports arrive, and `q` / `Ctrl+C` stay local to open dialogs.
2. If an action-needed prompt naturally appears, open snooze controls with `s`, cancel with `Esc`, and verify return to the same prompt. Confirm pending attention waits while another dialog owns focus.
3. In the diagnostic export preview (`D`), resize below 60×18. The resize screen should appear. `Enter` must not create the hidden export; `Esc` must still close the preview. Resize back and confirm normal input resumes. No diagnostic upload is needed for this check.
4. Try supported narrow and wide layouts. Confirm continued telemetry while a dialog is open and after resizing back. Quit normally and verify the terminal is restored. Record terminal application/version alongside the result.

## Logout and finish the record

1. After the other checks, quit normally and run `./bin/bambu-tui logout`.
2. Start `./bin/bambu-tui` and verify it reports the absence of a saved session and directs you to login. A credential-store access failure is a different outcome and should be recorded as such.
3. Log in again if you want to restore monitoring after the test. Restore any temporary review, snooze, or preference changes to the state you want.
4. Update the current table in `validation.md` with observations and limitations. A useful text report is: **build; date; host/terminal; printer alias/model/firmware; check; expected; observed; result**. Do not attach raw private files. Keep #38 open for missing required evidence; synthetic regressions can accompany a separately tracked defect.
