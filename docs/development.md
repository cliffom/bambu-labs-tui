# Development

[Back to the README](../README.md)

Requires Go 1.26 or newer. Bubble Tea v1, Bubbles, and Lip Gloss are pinned in `go.mod`.

```sh
make test    # tests with the race detector
make check   # go vet
make fmt     # format Go source
make fmt-check # check formatting without changing files
make lint    # golangci-lint
make lint-actions # actionlint for GitHub Actions workflows
make lint-make # checkmake for the Makefile
make audit-faults # verify fault codes against pinned upstream catalogues (network required)
make build   # bin/bambu-tui
make auth    # build and log in using an email code
make run     # build and open the dashboard
make demo    # build and open the simulated farm without login
make cross   # macOS/Linux, arm64/amd64 binaries in dist/
make release # versioned archives, SHA-256 checksums, and dependency licenses
```

`VERSION` is the release version. Make injects version, commit, and UTC build date into `--version`; a source build without Git metadata reports `unknown` for its commit. CI runs race tests and vet on Linux/macOS, checks native credential persistence on macOS, and packages release candidates on Linux. A pushed version tag creates a **draft** release for review; it does not publish automatically.

The separate **Formatting and lint** workflow checks Go formatting, error handling, vet diagnostics, ineffective assignments, static analysis, unused code, GitHub Actions syntax, and Makefile rules on pull requests and pushes to `main`. It reports formatting problems without rewriting source. Run `make fmt` locally to fix Go formatting. Checkmake validates the Makefile; it does not format or rewrite it.

Install the versions in `.golangci-lint-version` and `.actionlint-version` to reproduce the lint checks locally. Get golangci-lint from its [official releases](https://github.com/golangci/golangci-lint/releases), and install actionlint with `go install "github.com/rhysd/actionlint/cmd/actionlint@$(cat .actionlint-version)"`. `make lint` and `make lint-actions` use tools on `PATH`; override `GOLANGCI_LINT` or `ACTIONLINT` when installed elsewhere. Workflow lint also uses ShellCheck when it is available.

Install the pinned [checkmake](https://github.com/checkmake/checkmake) version with `go install "github.com/checkmake/checkmake/cmd/checkmake@$(cat .checkmake-version)"`. Run `make lint-make`, or override `CHECKMAKE` if the binary is installed elsewhere. `checkmake.ini` requires the project's build, auth, run, test, and check targets to exist and be declared phony; other lint rules use their defaults.

Packages separate cloud authentication/discovery, the MQTT transport, the monitor worker, normalized farm state, credential storage, configuration, and the terminal UI. The UI reducer alone mutates farm state. Transports communicate through messages and stop on context cancellation. Notice and snooze persistence commands receive immutable snapshots and report completion through messages. Each store permits one write in flight; changes during that write coalesce into a complete latest snapshot after completion. Attempted, successfully saved, and failed revisions stay distinct, with no tick-driven retry of failed revisions. The existing store implementations and on-disk formats remain unchanged. Normal quit waits up to five seconds, then requires an explicit exit or cancellation on failure/timeout; external cancellation reports unconfirmed saves after the terminal is restored.

Tests use fake HTTP/native credential processes, an in-memory MQTT broker connection, and **synthetic** H2D/X2D telemetry fixtures. Displayed telemetry has been validated on H2D and X2D, and the native credential-store probe has passed in a normal terminal. Remaining hardware and real-account restart checks are tracked in [the validation record](validation.md). Captured regression fixtures must have tokens, account identifiers, serials, job names, and spool identifiers removed.

Bambu's cloud protocol is community documented and may change. Implementation references:

- [OpenBambuAPI: cloud HTTP](https://github.com/Doridian/OpenBambuAPI/blob/main/cloud-http.md) and [MQTT](https://github.com/Doridian/OpenBambuAPI/blob/main/mqtt.md)
- [Home Assistant Bambu integration](https://github.com/greghesp/ha-bambulab/tree/main/custom_components/bambu_lab/pybambu), including dual-nozzle telemetry and login challenge handling
- [Bubble Tea](https://github.com/charmbracelet/bubbletea/tree/v1.3.10)


Dialog focus is owned by `dialogState` in `internal/ui/dialogs.go`. Its route catalog supplies keyboard handling, rendering, synchronization, and payload cleanup for each dialog; open/close transitions use that owner rather than pointer priority. Snooze controls opened from attention explicitly suspend and return to that prompt. Reset confirmation stays inside the decisions dialog, preserving selection and scroll state. Automatic attention waits until the owner releases focus, including during persistence shutdown.

Within `internal/farm`, `telemetry.go` owns report decoding, job-field invalidation, and flexible delta merging; `normalize.go` interprets the merged snapshot. `telemetry_presence.go` records condition-family and per-slot knowledge, distinguishing omitted data from explicit empty collections or reported unknown estimates. Review lifetime decisions consume that knowledge in `reviews.go`, while `state.go` and `conditions.go` handle lifecycle, history, and reconciliation. `Printer.Raw` remains the merged input for telemetry interpretation and allowlisted diagnostic snapshots; diagnostics still sanitize and re-normalize their own independent snapshot rather than serializing the live printer or its internal knowledge.
