#!/usr/bin/env bash
set -euo pipefail

if [[ $(uname -s) != Darwin ]]; then
  echo 'The notification helper requires macOS and Xcode Command Line Tools.' >&2
  exit 1
fi
notify_repo=$(cd "$(dirname "$0")/.." && pwd)
if [[ $# -gt 1 ]]; then
  echo 'Usage: build-notify-helper.sh [OUTPUT_DIRECTORY]' >&2
  exit 2
fi
notify_output=${1:-"$notify_repo/bin"}
notify_version=${VERSION:-$(cat "$notify_repo/VERSION")}
if [[ ! "$notify_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]]; then
  echo 'Invalid helper version' >&2
  exit 2
fi
notify_bundle="$notify_output/Bambu TUI Notify.app"
notify_slices=$(mktemp -d)
trap 'rm -rf "$notify_slices"' EXIT
mkdir -p "$notify_bundle/Contents/MacOS" "$notify_bundle/Contents/Resources"
cp "$notify_repo/tools/macos-notify/Info.plist" "$notify_bundle/Contents/Info.plist"

# Wrap the approved PNGs in an ICNS container without resampling the artwork.
python3 - "$notify_repo" "$notify_bundle/Contents/Resources/BambuTUI.icns" <<'PY'
import pathlib
import struct
import sys

root = pathlib.Path(sys.argv[1])
chunks = []
for kind, size in [(b'icp5', 32), (b'icp6', 64), (b'ic09', 512)]:
    png = (root / 'assets' / 'branding' / f'icon-{size}.png').read_bytes()
    chunks.append(kind + struct.pack('>I', len(png) + 8) + png)
body = b''.join(chunks)
pathlib.Path(sys.argv[2]).write_bytes(b'icns' + struct.pack('>I', len(body) + 8) + body)
PY

# Compile both slices on macOS, then sign the combined executable/bundle.
for notify_arch in arm64 x86_64; do
  xcrun swiftc -swift-version 5 -O -target "$notify_arch-apple-macos12.0" \
    -framework AppKit -framework UserNotifications \
    "$notify_repo/tools/macos-notify/main.swift" \
    -o "$notify_slices/$notify_arch"
done
xcrun lipo -create "$notify_slices/arm64" "$notify_slices/x86_64" \
  -output "$notify_bundle/Contents/MacOS/bambu-notify"
for notify_arch in arm64 x86_64; do
  xcrun lipo "$notify_bundle/Contents/MacOS/bambu-notify" -verify_arch "$notify_arch"
done
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString ${notify_version%%-*}" "$notify_bundle/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion ${notify_version%%-*}" "$notify_bundle/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :BambuTUIReleaseVersion $notify_version" "$notify_bundle/Contents/Info.plist"
/usr/bin/plutil -lint "$notify_bundle/Contents/Info.plist"
/usr/bin/codesign --force --sign - --identifier com.github.cliffom.bambu-tui.notify-poc "$notify_bundle"
/usr/bin/codesign --verify --strict --all-architectures "$notify_bundle"
printf 'Built universal, ad-hoc signed helper: %s\n' "$notify_bundle"
