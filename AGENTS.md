# Repository guidance

Bambu TUI is a Go terminal application for read-only Bambu Cloud monitoring. Keep printer control outside the project scope. Preserve distinctions between unknown and confirmed telemetry, local notice decisions, and physical fault resolution.

## Development

Use Go 1.26 or newer, Git, and Make. Run commands from the repository root. Start with [contributing](docs/development/contributing.md) for tool versions and setup.

- `make demo` builds and runs a synthetic farm without credentials or hardware.
- `make fmt-check lint test check build` checks Go formatting, lint, race tests, vet, and the native build. Use `make fmt` to format Go changes.
- `make lint-actions` checks workflow changes; `make lint-make` checks Makefile changes.
- `make release` verifies cross-platform builds and packaging when release tooling or bundled documentation paths change.
- `make audit-faults` performs the explicit network audit when changing the fault catalogue. Ordinary tests use synthetic fixtures and fake services.

Use the pinned tools documented in the contributing guide; Make supports overrides for tools installed outside `PATH`. Record checks actually run and any remaining limitations.

## Code and behavior

- `cmd/bambu-tui`: CLI, login, terminal lifecycle, and PTY smoke tests.
- `internal/cloud`, `internal/mqtt`, `internal/monitor`: authentication/discovery, transport, and monitor workers.
- `internal/farm`: telemetry decoding, normalization, condition lifetime, and farm state.
- `internal/ui`: Bubble Tea reducer, dialogs, panels, and asynchronous persistence commands.
- `internal/config`, `internal/credentials`, `internal/noticestore`, `internal/alertstore`, `internal/atomicfile`: configuration and storage.
- `internal/diagnostics`: allowlisted local exports.

Read [architecture and conventions](docs/development/architecture.md) before changing ownership, concurrency, persistence, or protocol handling. Keep regression fixtures synthetic or sanitized; never commit credentials, private telemetry, or unredacted account/printer identifiers. Automated coverage does not establish live hardware validation.

## Documentation map

Only root `README.md` and `AGENTS.md` belong outside `docs/`. Keep documentation current, consolidate repeated guidance, and update relative links and release tooling when moving documents. Put ongoing validation in the validation record; do not add version-specific readiness documents.

- **User behavior:** [usage and configuration](docs/user/usage.md), [farm view](docs/user/farm-view.md), [alert preferences](docs/user/alerts.md), [action-needed alerts](docs/user/action-needed.md), [diagnostic exports](docs/user/diagnostics.md).
- **Development and operations:** [contributing](docs/development/contributing.md), [architecture](docs/development/architecture.md), [terminal tests](docs/development/terminal-tests.md), [live-validation record](docs/development/validation.md), [publishing](docs/development/publishing.md).
- **References:** [protocol sources](docs/reference/protocol-sources.md), [branding artwork](docs/reference/branding.md).
- **Planning and history:** [roadmap](docs/ROADMAP.md), [changelog](docs/CHANGELOG.md).

The [documentation index](docs/README.md) is the human-facing navigation page. Release archives bundle `docs/`; update `tools/package-release.sh` and `.github/workflows/release.yml` if their document paths change.
