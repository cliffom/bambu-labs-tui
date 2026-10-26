#!/usr/bin/env python3
"""Validate CLI/helper inventory and metadata in generated release archives."""

import argparse
import plistlib
import struct
import tarfile

BUNDLE = 'Bambu TUI Notify.app'
CONTENTS = BUNDLE + '/Contents/'
EXECUTABLE = CONTENTS + 'MacOS/bambu-notify'
BUNDLE_ID = 'com.github.cliffom.bambu-tui.notify-poc'
UNIVERSAL_CPUS = {0x01000007, 0x0100000C}  # x86_64, arm64


def universal_cpus(data):
    if len(data) < 8 or data[:4] not in (b'\xca\xfe\xba\xbe', b'\xca\xfe\xba\xbf'):
        return set()
    count = struct.unpack_from('>I', data, 4)[0]
    stride = 32 if data[:4] == b'\xca\xfe\xba\xbf' else 20
    if count > 16 or len(data) < 8 + count * stride:
        return set()
    return {struct.unpack_from('>I', data, 8 + i * stride)[0] for i in range(count)}


def archive_errors(path, version):
    errors = []
    macos = '_darwin_' in str(path)
    with tarfile.open(path) as archive:
        entries = {m.name.removeprefix('./').rstrip('/'): m for m in archive.getmembers()}
        cli = entries.get('bambu-tui')
        if not cli or not cli.isfile() or not cli.size or not cli.mode & 0o111:
            errors.append('missing or non-executable CLI')
        helper_entries = [name for name in entries if name == BUNDLE or name.startswith(BUNDLE + '/')]
        if not macos:
            if helper_entries:
                errors.append('Linux archive contains macOS notification helper')
            return errors
        required = [EXECUTABLE, CONTENTS + 'Info.plist', CONTENTS + 'Resources/BambuTUI.icns', CONTENTS + '_CodeSignature/CodeResources']
        for name in required:
            entry = entries.get(name)
            if not entry or not entry.isfile() or not entry.size:
                errors.append('missing helper file: ' + name)
        binary = entries.get(EXECUTABLE)
        if binary and binary.isfile():
            if not binary.mode & 0o111:
                errors.append('notification helper is not executable')
            if universal_cpus(archive.extractfile(binary).read()) != UNIVERSAL_CPUS:
                errors.append('notification helper must contain arm64 and x86_64 slices')
        plist = entries.get(CONTENTS + 'Info.plist')
        if plist and plist.isfile():
            try:
                metadata = plistlib.loads(archive.extractfile(plist).read())
                expected = {'CFBundleIdentifier': BUNDLE_ID, 'CFBundleExecutable': 'bambu-notify', 'CFBundleIconFile': 'BambuTUI.icns', 'CFBundleShortVersionString': version.split('-')[0], 'CFBundleVersion': version.split('-')[0], 'BambuTUIReleaseVersion': version}
                if not isinstance(metadata, dict) or any(metadata.get(k) != v for k, v in expected.items()) or metadata.get('LSUIElement') is not True:
                    errors.append('notification helper identity/version/icon metadata mismatch')
            except (plistlib.InvalidFileException, ValueError, TypeError):
                errors.append('invalid notification helper plist')
    return errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('archive')
    parser.add_argument('--version', required=True)
    args = parser.parse_args()
    errors = archive_errors(args.archive, args.version)
    if errors:
        parser.exit(1, '\n'.join(errors) + '\n')
    print('Release inventory checks passed: ' + args.archive)


if __name__ == '__main__':
    main()
