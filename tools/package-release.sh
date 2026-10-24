#!/usr/bin/env bash
set -euo pipefail

release_version=${1:?Usage: package-release.sh VERSION}
if [[ ! "$release_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]]; then
  echo 'Invalid release version' >&2
  exit 1
fi
release_stage=$(mktemp -d)
trap 'rm -rf "$release_stage"' EXIT

cp README.md LICENSE CHANGELOG.md config.example.toml "$release_stage/"
mkdir -p "$release_stage/docs" "$release_stage/third_party"
cp docs/protocol-sources.md docs/validation.md "$release_stage/docs/"
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
  [[ -z "$dependency" || "$dependency" == 'github.com/cliffom/bambu-labs-tui' ]] && continue
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

archives=()
for target in darwin-arm64 darwin-amd64 linux-arm64 linux-amd64; do
  cp "dist/bambu-tui-$target" "$release_stage/bambu-tui"
  archive="bambu-tui_v${release_version}_${target//-/_}.tar.gz"
  COPYFILE_DISABLE=1 tar -czf "dist/$archive" -C "$release_stage" bambu-tui README.md LICENSE CHANGELOG.md config.example.toml docs third_party
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
