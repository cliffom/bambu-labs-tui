#!/usr/bin/env python3
"""Install release credentials in an isolated GitHub Actions runner keychain."""

import argparse
import base64
import binascii
import json
import os
from pathlib import Path
import secrets
import shlex
import subprocess
import sys

REQUIRED = ('MACOS_CERTIFICATE_P12_BASE64', 'MACOS_CERTIFICATE_PASSWORD', 'MACOS_SIGN_IDENTITY',
            'APPLE_ID', 'APPLE_TEAM_ID', 'APPLE_APP_SPECIFIC_PASSWORD')
PROFILE = 'bambustand-notary'


def command(args, description):
    # Never include argv or raw tool output in errors: they can contain secrets.
    result = subprocess.run(args, capture_output=True, text=True)
    if result.returncode:
        raise ValueError(f'{description} failed (exit {result.returncode})')
    return result.stdout


def paths(env):
    if env.get('GITHUB_ACTIONS') != 'true' or not env.get('RUNNER_TEMP') or sys.platform != 'darwin':
        raise ValueError('This helper runs only on a macOS GitHub Actions runner')
    root = Path(env['RUNNER_TEMP'])
    return (root / 'bambustand-signing.keychain-db', root / 'bambustand-signing.p12',
            root / 'bambustand-keychain-search-list.json')


def write_private(path, data):
    with os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'wb') as file:
        file.write(data)


def setup(env):
    keychain, certificate, state = paths(env)
    missing = [name for name in REQUIRED if not env.get(name)]
    if missing:
        raise ValueError('Missing GitHub Actions secrets: ' + ', '.join(missing))
    try:
        data = base64.b64decode(''.join(env['MACOS_CERTIFICATE_P12_BASE64'].split()), validate=True)
    except (binascii.Error, ValueError) as error:
        raise ValueError('MACOS_CERTIFICATE_P12_BASE64 is not valid Base64') from error
    if not data:
        raise ValueError('The certificate secret is empty')
    if keychain.exists() or state.exists() or certificate.exists():
        raise ValueError('CI signing files already exist; clean them before setup')
    original = shlex.split(command(['security', 'list-keychains', '-d', 'user'], 'Read keychain search list'))
    write_private(state, json.dumps(original).encode())
    password = secrets.token_urlsafe(32)
    try:
        write_private(certificate, data)
        command(['security', 'create-keychain', '-p', password, str(keychain)], 'Create CI keychain')
        command(['security', 'set-keychain-settings', '-lut', '21600', str(keychain)], 'Configure CI keychain')
        command(['security', 'unlock-keychain', '-p', password, str(keychain)], 'Unlock CI keychain')
        command(['security', 'import', str(certificate), '-P', env['MACOS_CERTIFICATE_PASSWORD'],
                 '-f', 'pkcs12', '-k', str(keychain), '-T', '/usr/bin/codesign'], 'Import signing identity')
        command(['security', 'set-key-partition-list', '-S', 'apple-tool:,apple:,codesign:',
                 '-s', '-k', password, str(keychain)], 'Allow noninteractive signing')
        command(['security', 'list-keychains', '-d', 'user', '-s', str(keychain), *original], 'Add CI keychain')
        identities = command(['security', 'find-identity', '-v', '-p', 'codesigning', str(keychain)], 'Verify imported identity')
        if env['MACOS_SIGN_IDENTITY'] not in identities:
            raise ValueError('MACOS_SIGN_IDENTITY does not match a valid imported signing identity')
        command(['xcrun', 'notarytool', 'store-credentials', PROFILE, '--keychain', str(keychain),
                 '--apple-id', env['APPLE_ID'], '--team-id', env['APPLE_TEAM_ID'],
                 '--password', env['APPLE_APP_SPECIFIC_PASSWORD']], 'Store notarization credentials')
        with open(env['GITHUB_ENV'], 'a') as output:
            output.write(f'MACOS_NOTARY_PROFILE={PROFILE}\nMACOS_NOTARY_KEYCHAIN={keychain}\n')
        print('Release signing identity and notarization profile installed in the temporary keychain.')
    finally:
        certificate.unlink(missing_ok=True)


def cleanup(env):
    keychain, certificate, state = paths(env)
    failures = []
    if state.exists():
        try:
            original = json.loads(state.read_text())
            command(['security', 'list-keychains', '-d', 'user', '-s', *original], 'Restore keychain search list')
        except (OSError, ValueError) as error:
            failures.append(str(error))
    if keychain.exists():
        try:
            command(['security', 'delete-keychain', str(keychain)], 'Delete CI signing keychain')
        except ValueError as error:
            failures.append(str(error))
    certificate.unlink(missing_ok=True)
    if not failures:
        state.unlink(missing_ok=True)
    if failures:
        raise ValueError('; '.join(failures))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['setup', 'cleanup'])
    args = parser.parse_args()
    try:
        if args.action == 'setup':
            setup(os.environ)
        else:
            cleanup(os.environ)
    except (OSError, ValueError) as error:
        parser.exit(1, f'CI signing {args.action}: {error}\n')


if __name__ == '__main__':
    main()
