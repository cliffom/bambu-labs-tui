# Repository guidance

Bambu TUI is a Go terminal application for read-only Bambu Cloud monitoring. Keep printer control outside the project scope. Preserve distinctions between unknown and confirmed telemetry, local notice decisions, and physical fault resolution.

## Required direction and verification

Before making changes, read [UI direction](docs/development/ui-direction.md) for UI work and [documentation direction](docs/development/documentation-direction.md) for every change. Preserve their contracts unless the user requests a change to them; update affected guidance and verification in the same PR. Assess documentation impact and describe it in the PR, including why no documentation update is needed when applicable.

Run `make docs-check` for every PR. For UI changes, extend relevant interaction tests, perform the applicable demo/layout inspection, and report the sizes/states actually checked and any gaps. See the direction guides for the review criteria.

## Development

Use Go 1.27.3 or newer, Git, and Make. Run commands from the repository root. Start with [contributing](docs/development/contributing.md) for tool versions and setup.

- `make demo` builds and runs a synthetic farm without credentials or hardware.
- `make fmt-check lint test check build` checks Go formatting, lint, race tests, vet, and the native build. Use `make fmt` to format Go changes.
- `make vuln-check` scans known vulnerabilities with the pinned scanner and current build toolchain (network required).
- `make lint-actions` checks workflow changes; `make lint-make` checks Makefile changes.
- `make release` verifies cross-platform builds and packaging when release tooling or bundled documentation paths change; it requires a macOS host with Xcode Command Line Tools (Swift) for the universal notification helper.
- `make audit-faults` performs the explicit network audit when changing the fault catalogue. Ordinary tests use synthetic fixtures and fake services.

Use the pinned tools documented in the contributing guide; Make supports overrides for tools installed outside `PATH`. Record checks actually run and any remaining limitations.

## Code and behavior

- `cmd/bambu-tui`: CLI, login, terminal lifecycle, and PTY smoke tests.
- `internal/cloud`, `internal/mqtt`, `internal/monitor`: authentication/discovery, transport, and monitor workers.
- `internal/farm`: telemetry decoding, normalization, condition lifetime, and farm state.
- `internal/session`: shared serial owner for farm mutations/reconciliation, local browser decisions, immutable publishing, and persistence jobs.
- `internal/ui`: Bubble Tea adapter, dialogs, panels, notification integration, and interactive shutdown.
- `internal/webserver`: session-owned localhost HTTP lifecycle, embedded dashboard, and allowlisted immutable display snapshots.
- `internal/config`, `internal/credentials`, `internal/noticestore`, `internal/alertstore`, `internal/notification_state`, `internal/atomicfile`: configuration and storage.
- `internal/diagnostics`: allowlisted local exports.

Read [architecture and conventions](docs/development/architecture.md) before changing ownership, concurrency, persistence, or protocol handling. Keep regression fixtures synthetic or sanitized; never commit credentials, private telemetry, or unredacted account/printer identifiers. Automated coverage does not establish live hardware validation.

## Documentation map

Only root `README.md` and `AGENTS.md` belong outside `docs/`. Keep documentation current, consolidate repeated guidance, and update relative links and release tooling when moving documents. Put ongoing validation in the validation record; do not add version-specific readiness documents.

- **User behavior:** [usage and configuration](docs/user/usage.md), [farm view](docs/user/farm-view.md), [alert preferences](docs/user/alerts.md), [action-needed alerts](docs/user/action-needed.md), [diagnostic exports](docs/user/diagnostics.md).
- **Development and operations:** [contributing](docs/development/contributing.md), [architecture](docs/development/architecture.md), [UI direction](docs/development/ui-direction.md), [documentation direction](docs/development/documentation-direction.md), [terminal tests](docs/development/terminal-tests.md), [live-validation record](docs/development/validation.md), [publishing](docs/development/publishing.md).
- **References:** [protocol sources](docs/reference/protocol-sources.md), [branding artwork](docs/reference/branding.md).
- **Planning and history:** [roadmap](docs/ROADMAP.md), [changelog](docs/CHANGELOG.md).

The [documentation index](docs/README.md) is the human-facing navigation page. Binary release archives include `docs/user/release-installation.md` as `README.md`, the configuration example, and `THIRD_PARTY_NOTICES.txt`; full documentation stays in the repository. Keep `tools/package-release.sh`, archive checkers, and `.github/workflows/release.yml` aligned with this layout. The workflow reads release notes from `docs/CHANGELOG.md`.
