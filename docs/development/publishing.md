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

## macOS signing and notarization

Local development helper builds and plain `make release` use an ad-hoc signature. This verifies bundle integrity but does not establish Developer ID trust, notarization, or downloaded-app Gatekeeper acceptance. The draft-release workflow also remains in candidate mode until signing credentials are provisioned on its build runner.

For a signed candidate, install a Developer ID Application certificate and its private key in your build Mac's Keychain, then select it explicitly by its identity name or fingerprint:

```sh
security find-identity -v -p codesigning
make release MACOS_SIGN_IDENTITY='Developer ID Application: Your Name (TEAMID)'
```

Release preparation signs the universal helper and both macOS CLI binaries with hardened runtime and a secure timestamp. It verifies Developer ID identity, matching teams, runtime, timestamps, and signature integrity. A failed signing operation stops packaging; it never falls back to ad-hoc signing when an identity was requested. Linux binaries are unchanged. A signed candidate alone is still not notarized.

For notarization, first create an app-specific password at your Apple Account's Sign-In and Security page. Store it interactively in Keychain; unspecified account/password options are prompted for by `notarytool`:

```sh
xcrun notarytool store-credentials 'bambu-tui-notary' --team-id YOUR_TEAM_ID
make release \
  MACOS_SIGN_IDENTITY='Developer ID Application: Your Name (TEAMID)' \
  MACOS_NOTARY_PROFILE='bambu-tui-notary'
```

The profile name is a reference to Keychain credentials, not the password. Credentials and signing private keys stay outside the repository. See [Apple's credential workflow](https://developer.apple.com/documentation/technotes/tn3147-migrating-to-the-latest-notarization-tool) and [notarization requirements](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution).

The release tool submits a temporary ZIP containing the signed helper and both macOS CLIs, waits up to 20 minutes, and requires an Accepted result. Rejected, interrupted, malformed, or still-processing responses stop packaging. Reports are stored under ignored `dist/notarization/`; Apple may continue processing after a client timeout. Use the submission ID from the report with `xcrun notarytool info SUBMISSION_ID --keychain-profile bambu-tui-notary` to inspect it before deciding to submit a new build.

After acceptance, the helper's ticket is stapled and validated before archives and checksums are generated. Packaging verifies Developer ID signatures/teams from the extracted helper and CLI in both macOS archives, and validates each extracted helper's stapled ticket. A raw CLI cannot carry a stapled ticket; its notarization record is checked by macOS online. Stapling the helper does not establish offline installation of the standalone CLI. The ZIP submission is temporary; the public archive format remains `.tar.gz`.

Before public distribution, download the actual release archive onto a clean Mac with Gatekeeper enabled, keep the helper beside the CLI, and test CLI startup, the notification permission workflow, banner/sound delivery, and quit. Verify the helper with `xcrun stapler validate` and `spctl --assess --type execute --verbose=2`. Signing verification, Apple's acceptance, and local execution do not establish the clean-machine experience. CI signing/notarization needs a certificate/private-key import and notarization credentials on the runner; creating a local Keychain profile does not configure GitHub Actions.

## Verify and publish

Wait for CI, lint, and release packaging to pass. Download the draft assets from [Releases](https://github.com/cliffom/bambu-tui/releases), verify all four archives against the checksum manifest, and check the native binary's `--version` output. Replace the draft's full-changelog notes with focused release notes describing the changes, validation, and limitations.

Publish the verified draft as the latest stable release. The tag workflow does not publish automatically. Existing archived releases retain the source/module identity used when they were built.
