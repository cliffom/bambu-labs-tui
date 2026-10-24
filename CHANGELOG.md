# Changelog

## 0.0.1 — Initial release candidate

- Read-only Bambu Cloud monitoring with password, email-code, authenticator, and token-import login.
- Farm overview, per-printer progress/ETA, dual-nozzle temperatures, AMS/AMS HT filament paths, and estimated remaining filament.
- Selectable notices, detailed HMS views, local acknowledgement and dismissal, and session event history.
- HMS severity classification, maintenance descriptions, and suppression of a known internal empty-message code.
- Credential-store diagnostics through `bambu-tui doctor`, checked save/readback, and explicit session-only fallback.
- Offline demo mode, configurable region/filament threshold, and responsive keyboard navigation.
- macOS/Linux binaries for arm64/amd64, checksums, third-party licenses, and build version/commit/date metadata.

### Validation and limitations

Live email-code login and printer telemetry have been observed in a user-provided screenshot. Local race tests and build checks use synthetic telemetry and fake services. A complete live smoke test is recorded separately in [docs/validation.md](docs/validation.md).

Direct Google OAuth, LAN connections, printer control, and persistent event history are outside this release. HMS descriptions cover a curated subset; unknown codes remain visible with their code and troubleshooting link. Dismissal only hides conditions in the current TUI session.
