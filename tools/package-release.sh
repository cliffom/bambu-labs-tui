#!/usr/bin/env bash
set -euo pipefail

release_version=${1:?Usage: package-release.sh VERSION}
if [[ ! "$release_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]]; then
  echo 'Invalid release version' >&2
  exit 1
fi
if [[ $(uname -s) != Darwin ]]; then
  echo 'Packaging macOS notification helpers requires a macOS build host.' >&2
  exit 1
fi
python3 -m unittest discover -s tools -p 'test_check_release.py'
python3 -m unittest discover -s tools -p 'test_third_party_notices.py'
python3 -m unittest discover -s tools -p 'test_prepare_macos_release.py'
python3 -m unittest discover -s tools -p 'test_setup_macos_ci_signing.py'
python3 tools/check-docs.py

release_stage=$(mktemp -d)
trap 'rm -rf "$release_stage"' EXIT

cp LICENSE config.example.toml "$release_stage/"
cp docs/user/release-installation.md "$release_stage/README.md"
go_root=$(go env GOROOT)
# Include notices from dependencies actually linked into the application.
for target in darwin-arm64 darwin-amd64 linux-arm64 linux-amd64; do
  CGO_ENABLED=0 GOOS=${target%-*} GOARCH=${target#*-} \
    go list -deps -f '{{with .Module}}{{.Path}} {{.Dir}}{{end}}' ./cmd/bambustand || exit 1
done | sort -u > "$release_stage/modules.txt"
python3 tools/third-party-notices.py --go-root "$go_root" \
  --modules "$release_stage/modules.txt" --output "$release_stage/THIRD_PARTY_NOTICES.txt"
rm "$release_stage/modules.txt"

notify_bundle='BambuStand Notify.app'
cp -R "dist/notify-helper/$notify_bundle" "$release_stage/$notify_bundle"
/usr/bin/codesign --verify --strict --all-architectures "$release_stage/$notify_bundle"
for notify_arch in arm64 x86_64; do
  xcrun lipo "$release_stage/$notify_bundle/Contents/MacOS/bambu-notify" -verify_arch "$notify_arch"
done

archives=()
for target in darwin-arm64 darwin-amd64 linux-arm64 linux-amd64; do
  cp "dist/bambustand-$target" "$release_stage/bambustand"
  download_target=${target/darwin/macos}
  archive="bambustand_v${release_version}_${download_target//-/_}.tar.gz"
  archive_members=(bambustand README.md LICENSE config.example.toml THIRD_PARTY_NOTICES.txt)
  if [[ "$target" == darwin-* ]]; then
    archive_members+=("$notify_bundle")
  fi
  COPYFILE_DISABLE=1 tar -czf "dist/$archive" -C "$release_stage" "${archive_members[@]}"
  python3 tools/check-release.py "dist/$archive" --version "$release_version"
  if [[ "$target" == darwin-* ]]; then
    # Verify signatures from the actual archive, not only the pre-tar bundle.
    notify_extracted="$release_stage/verify-$target"
    mkdir -p "$notify_extracted"
    tar -xzf "dist/$archive" -C "$notify_extracted" "$notify_bundle" bambustand
    /usr/bin/codesign --verify --strict --all-architectures "$notify_extracted/$notify_bundle"
    if [[ -n "${MACOS_SIGN_IDENTITY:-}" ]]; then
      python3 tools/prepare-macos-release.py verify "$notify_extracted/$notify_bundle" "$notify_extracted/bambustand"
    fi
    if [[ -n "${MACOS_NOTARY_PROFILE:-}" ]]; then
      xcrun stapler validate "$notify_extracted/$notify_bundle"
      /usr/bin/codesign --verify --strict --all-architectures --check-notarization -R=notarized "$notify_extracted/bambustand"
    fi
    "$notify_extracted/$notify_bundle/Contents/MacOS/bambu-notify" --help
  fi
  python3 tools/check-docs.py --archive "dist/$archive"
  archives+=("$archive")
done
(
  cd dist
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${archives[@]}"
  else
    shasum -a 256 "${archives[@]}"
  fi
) > "dist/bambustand_v${release_version}_checksums.txt"
echo "Release archives and checksums prepared in dist/"
