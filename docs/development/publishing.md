# Publishing releases

[Contributing](contributing.md) · [Documentation index](../README.md)

Run these commands from the repository root on macOS with Xcode Command Line Tools (Swift) and Python 3.10+. Release packaging requires macOS to build and verify the universal notification helper.

## Prepare a release

Update `VERSION` and the default version in `internal/buildinfo/buildinfo.go` to a new, unused release version. Update [docs/CHANGELOG.md](../CHANGELOG.md) and any affected user guides, then run the appropriate checks:

```sh
make docs-check fmt-check lint lint-actions lint-make
make audit-faults
make vuln-check test check release
```

The lint commands require the pinned tools documented in the [development guide](contributing.md). The catalogue audit requires network access; ordinary tests remain offline. Review the hardware-validation limitations in [validation.md](validation.md) and any feature-specific validation notes.

Commit and push the release preparation to `main`. After CI passes, create and push an annotated tag matching `VERSION`:

```sh
release_tag="v$(cat VERSION)"
git tag -a "$release_tag" -m "Bambu TUI $release_tag"
git push origin "$release_tag"
```

Release archives include the README, repository guidance, configuration example, the complete `docs/` tree, branding banner, and dependency licenses. macOS archives also include the universal `Bambu TUI Notify.app` beside the CLI; Linux archives omit it. Keep the CLI and helper together after extraction. Doctor registers that adjacent bundle on first use. Roadmap and changelog paths inside each archive match the repository: `docs/ROADMAP.md` and `docs/CHANGELOG.md`. The draft release reads its initial notes from `docs/CHANGELOG.md`.

The tag workflow uses the patched Go version in `go.mod`, verifies the version, scans known vulnerabilities with the pinned scanner, runs race tests and vet, builds macOS/Linux archives for arm64/amd64, and includes dependency licenses. Actions are pinned to commit SHAs and checkout does not persist credentials. The build job runs on macOS with read-only repository permissions and uploads the archives, checksums, and changelog as a one-day artifact. A separate publishing job downloads that artifact and uses repository write permission only to create a **draft** GitHub release with SHA-256 checksums. Build metadata includes the source commit and UTC build date; builds without Git metadata report `unknown` for the commit.

## macOS signing status

The helper currently has an ad-hoc signature for local validation. This verifies bundle integrity but does not establish Developer ID trust or notarization. Before distributing the helper as a supported downloaded app, configure Developer ID Application signing and notarization, then test the downloaded, quarantined archive on a clean Mac with Gatekeeper enabled. See [Apple's notarization guidance](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution). No Developer ID credentials or notarization submission are configured by these build changes.

Local `make release` and the draft-release workflow can prepare and verify ad-hoc signed candidate archives; neither check establishes Gatekeeper acceptance. Do not report that requirement as validated merely because packaging or `codesign --verify` passes.

Developer ID signing and notarization are deferred to the final v0.2.x release stage. Continue implementation and local testing with ad-hoc signed candidates; complete distribution signing, notarization, and clean-Mac Gatekeeper validation before public distribution.

## Verify and publish

Wait for CI, lint, and release packaging to pass. Download the draft assets from [Releases](https://github.com/cliffom/bambu-tui/releases), verify all four archives against the checksum manifest, and check the native binary's `--version` output. Replace the draft's full-changelog notes with focused release notes describing the changes, validation, and limitations.

Publish the verified draft as the latest stable release. The tag workflow does not publish automatically. Existing archived releases retain the source/module identity used when they were built.
