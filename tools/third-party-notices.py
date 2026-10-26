#!/usr/bin/env python3
"""Collect Go and linked-module notices into one attributed release file."""

import argparse
from pathlib import Path

PROJECT_MODULE = 'github.com/cliffom/bambu-tui'


def collect_notices(go_root, modules):
    go_license = go_root / 'LICENSE'
    if not go_license.is_file():
        # Homebrew keeps the license above libexec/GOROOT.
        go_license = go_root.parent / 'LICENSE'
    sections = [('Go', go_license)]
    dependencies = {}
    for line in modules.splitlines():
        if not line.strip():
            continue
        module, directory = line.split(maxsplit=1)
        if module == PROJECT_MODULE:
            continue
        path = Path(directory)
        if module in dependencies and dependencies[module] != path:
            raise ValueError('multiple source directories for ' + module)
        dependencies[module] = path
    for module, directory in sorted(dependencies.items()):
        notices = sorted({path for pattern in ('LICENSE*', 'COPYING*', 'NOTICE*')
                          for path in directory.glob(pattern) if path.is_file()})
        if not notices:
            raise ValueError('no license found for ' + module)
        sections.extend((module, path) for path in notices)
    data = bytearray(b'Third-party notices for Bambu TUI\n\n')
    for module, path in sections:
        content = path.read_bytes()
        if not content.strip():
            raise ValueError('empty notice: ' + module + '/' + path.name)
        data.extend(('===== ' + module + ' / ' + path.name + ' =====\n\n').encode())
        data.extend(content)
        if not content.endswith(b'\n'):
            data.extend(b'\n')
        data.extend(b'\n')
    return bytes(data)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go-root', required=True, type=Path)
    parser.add_argument('--modules', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    try:
        data = collect_notices(args.go_root, args.modules.read_text())
        args.output.write_bytes(data)
    except (OSError, ValueError) as error:
        parser.exit(1, f'Cannot assemble third-party notices: {error}\n')


if __name__ == '__main__':
    main()
