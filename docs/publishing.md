# Publishing releases

The repository is [cliffom/bambu-tui](https://github.com/cliffom/bambu-tui). Clone it with:

```sh
git clone https://github.com/cliffom/bambu-tui.git
cd bambu-tui
```

For an existing checkout, update its remote:

```sh
git remote set-url origin https://github.com/cliffom/bambu-tui.git
```

## Prepare a release

Update `VERSION` and the default version in `internal/buildinfo/buildinfo.go` to a new, unused release version. Update the changelog and README installation examples, then run the appropriate checks:

```sh
make fmt-check lint lint-actions lint-make
make audit-faults
make test check release
```

The lint commands require the pinned tools documented in the [development guide](development.md). The catalogue audit requires network access; ordinary tests remain offline. Review the hardware-validation limitations in [validation.md](validation.md) and any feature-specific validation notes.

Commit and push the release preparation to `main`. After CI passes, create and push an annotated tag matching `VERSION`:

```sh
release_tag="v$(cat VERSION)"
git tag -a "$release_tag" -m "Bambu TUI $release_tag"
git push origin "$release_tag"
```

The tag workflow verifies the version, runs race tests and vet, builds macOS/Linux archives for arm64/amd64, includes dependency licenses, and creates a **draft** GitHub release with SHA-256 checksums. Build metadata includes the source commit and UTC build date; builds without Git metadata report `unknown` for the commit.

## Verify and publish

Wait for CI, lint, and release packaging to pass. Download the draft assets from [Releases](https://github.com/cliffom/bambu-tui/releases), verify all four archives against the checksum manifest, and check the native binary's `--version` output. Replace the draft's full-changelog notes with focused release notes describing the changes, validation, and limitations.

Publish the verified draft as the latest stable release. The tag workflow does not publish automatically. Existing archived releases retain the source/module identity used when they were built.
