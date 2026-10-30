#!/usr/bin/env python3
"""Sign macOS release binaries and optionally notarize/staple the helper."""

import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

BUNDLE = 'BambuStand Notify.app'
CLI_ID = 'com.github.cliffom.bambustand'
# Keep notification authorization attached to the original helper identity.
HELPER_ID = 'com.github.cliffom.bambu-tui.notify-poc'
# Apple's Developer ID Application certificate extension, anchored to Apple.
DEVELOPER_ID_REQUIREMENT = 'anchor apple generic and certificate leaf[field.1.2.840.113635.100.6.1.13] exists'


def command(args, capture=False, check=True):
    return subprocess.run(args, check=check, text=True, capture_output=capture)


def verify(path):
    command(['codesign', '--verify', '--strict', '--all-architectures', '-R=' + DEVELOPER_ID_REQUIREMENT, str(path)])
    result = command(['codesign', '--display', '--verbose=4', str(path)], capture=True)
    metadata = result.stdout + result.stderr
    team = re.search(r'^TeamIdentifier=(\S+)$', metadata, re.MULTILINE)
    if ('Authority=Developer ID Application:' not in metadata or '(runtime)' not in metadata
            or 'Timestamp=' not in metadata or not team or team[1] == 'not set'):
        raise ValueError(f'{path}: requires Developer ID Application, hardened runtime, and secure timestamp')
    return team[1]


def verify_all(paths):
    teams = {verify(path) for path in paths}
    if len(teams) != 1:
        raise ValueError('macOS release binaries must use the same Developer ID team')
    return teams.pop()


def sign(path, identity, identifier):
    command(['codesign', '--force', '--sign', identity, '--options', 'runtime',
             '--timestamp', '--identifier', identifier, str(path)])


def prepare(dist, identity, profile, keychain=''):
    if profile and not identity:
        raise ValueError('MACOS_NOTARY_PROFILE requires MACOS_SIGN_IDENTITY; refusing unsigned notarization')
    if not identity:
        print('Preparing ad-hoc signed candidate archives; distribution signing/notarization not enabled.')
        return
    helper = dist / 'notify-helper' / BUNDLE
    binaries = [dist / ('bambustand-darwin-' + arch) for arch in ('arm64', 'amd64')]
    if not helper.is_dir() or any(not path.is_file() for path in binaries):
        raise ValueError('build both macOS CLIs and the notification helper before signing')
    sign(helper, identity, HELPER_ID)
    for binary in binaries:
        sign(binary, identity, CLI_ID)
    team = verify_all([helper, *binaries])
    print(f'Developer ID signatures verified for helper and both CLIs (team {team}).', flush=True)
    if not profile:
        print('Signed candidate only: MACOS_NOTARY_PROFILE is unset; notarization has not been performed.')
        return
    reports = dist / 'notarization'
    reports.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='bambustand-notarize-') as temporary:
        staging = Path(temporary) / 'payload'
        staging.mkdir()
        shutil.copytree(helper, staging / BUNDLE)
        for binary in binaries:
            shutil.copy2(binary, staging / binary.name)
        upload = Path(temporary) / 'submission.zip'
        command(['ditto', '-c', '-k', '--keepParent', str(staging), str(upload)])
        print('Submitting helper and both macOS CLIs to Apple; waiting up to 20 minutes.', flush=True)
        credentials = ['--keychain-profile', profile]
        if keychain:
            credentials += ['--keychain', keychain]
        result = command(['xcrun', 'notarytool', 'submit', str(upload), *credentials,
                          '--wait', '--timeout', '20m', '--output-format', 'json'], capture=True, check=False)
        (reports / 'submission.json').write_text(result.stdout)
        (reports / 'submission-stderr.txt').write_text(result.stderr)
        try:
            report = json.loads(result.stdout)
        except json.JSONDecodeError as error:
            raise ValueError('notarytool returned no valid JSON; see dist/notarization/ reports') from error
        if not isinstance(report, dict):
            raise ValueError('notarytool returned an invalid report; see dist/notarization/ reports')
        if result.returncode != 0 or report.get('status') != 'Accepted':
            submission = report.get('id')
            if submission:
                log = command(['xcrun', 'notarytool', 'log', submission, *credentials], capture=True, check=False)
                (reports / 'log.json').write_text(log.stdout)
            raise ValueError('Apple did not accept notarization; see dist/notarization/ for the report/log')
    command(['xcrun', 'stapler', 'staple', str(helper)])
    command(['xcrun', 'stapler', 'validate', str(helper)])
    verify_all([helper, *binaries])
    print('Apple accepted both CLIs and the helper; helper ticket stapled and validated.', flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['prepare', 'verify'])
    parser.add_argument('paths', nargs='+', type=Path)
    args = parser.parse_args()
    try:
        if args.action == 'verify':
            verify_all(args.paths)
        elif len(args.paths) == 1:
            prepare(args.paths[0], os.environ.get('MACOS_SIGN_IDENTITY', ''),
                    os.environ.get('MACOS_NOTARY_PROFILE', ''), os.environ.get('MACOS_NOTARY_KEYCHAIN', ''))
        else:
            parser.error('prepare requires one dist directory')
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        parser.exit(1, f'macOS release preparation failed: {error}\n')


if __name__ == '__main__':
    main()
