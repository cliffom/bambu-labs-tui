#!/usr/bin/env bash
set -euo pipefail

# Compatibility entry point for the original local PoC instructions.
notify_repo=$(cd "$(dirname "$0")/.." && pwd)
bash "$notify_repo/tools/build-notify-helper.sh"
/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f "$notify_repo/bin/Bambu TUI Notify.app"
