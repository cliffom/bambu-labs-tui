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
python3 tools/check-docs.py

release_stage=$(mktemp -d)
trap 'rm -rf "$release_stage"' EXIT

cp README.md AGENTS.md LICENSE config.example.toml "$release_stage/"
mkdir -p "$release_stage/third_party" "$release_stage/assets/branding"
cp -R docs "$release_stage/docs"
cp assets/branding/hero.png "$release_stage/assets/branding/"
cp -R assets/screenshots "$release_stage/assets/screenshots"
go_root=$(go env GOROOT)
if [[ -f "$go_root/LICENSE" ]]; then
  cp "$go_root/LICENSE" "$release_stage/third_party/Go-LICENSE"
else
  # Homebrew keeps the license one directory above libexec/GOROOT.
  cp "$go_root/../LICENSE" "$release_stage/third_party/Go-LICENSE"
fi

# Include notices from dependencies actually linked into the application.
go list -deps -f '{{with .Module}}{{.Path}} {{.Dir}}{{end}}' ./cmd/bambu-tui | sort -u > "$release_stage/modules.txt"
while read -r dependency dependency_dir; do
  [[ -z "$dependency" || "$dependency" == 'github.com/cliffom/bambu-tui' ]] && continue
  destination="$release_stage/third_party/${dependency//\//_}"
  mkdir -p "$destination"
  shopt -s nullglob
  notices=("$dependency_dir"/LICENSE* "$dependency_dir"/COPYING* "$dependency_dir"/NOTICE*)
  if [[ ${#notices[@]} -eq 0 ]]; then
    echo "No license found for $dependency" >&2
    exit 1
  fi
  for notice in "${notices[@]}"; do
    [[ -f "$notice" ]] && cp "$notice" "$destination/"
  done
done < "$release_stage/modules.txt"
rm "$release_stage/modules.txt"

notify_bundle='Bambu TUI Notify.app'
cp -R "dist/notify-helper/$notify_bundle" "$release_stage/$notify_bundle"
/usr/bin/codesign --verify --strict --all-architectures "$release_stage/$notify_bundle"
for notify_arch in arm64 x86_64; do
  xcrun lipo "$release_stage/$notify_bundle/Contents/MacOS/bambu-notify" -verify_arch "$notify_arch"
done

archives=()
for target in darwin-arm64 darwin-amd64 linux-arm64 linux-amd64; do
  cp "dist/bambu-tui-$target" "$release_stage/bambu-tui"
  archive="bambu-tui_v${release_version}_${target//-/_}.tar.gz"
  archive_members=(bambu-tui README.md AGENTS.md LICENSE config.example.toml docs assets third_party)
  if [[ "$target" == darwin-* ]]; then
    archive_members+=("$notify_bundle")
  fi
  COPYFILE_DISABLE=1 tar -czf "dist/$archive" -C "$release_stage" "${archive_members[@]}"
  python3 tools/check-release.py "dist/$archive" --version "$release_version"
  if [[ "$target" == darwin-* ]]; then
    # Verify signatures from the actual archive, not only the pre-tar bundle.
    notify_extracted="$release_stage/verify-$target"
    mkdir -p "$notify_extracted"
    tar -xzf "dist/$archive" -C "$notify_extracted" "$notify_bundle"
    /usr/bin/codesign --verify --strict --all-architectures "$notify_extracted/$notify_bundle"
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
) > "dist/bambu-tui_v${release_version}_checksums.txt"
echo "Release archives and checksums prepared in dist/"
