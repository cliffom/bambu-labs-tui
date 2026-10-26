import base64
import importlib.util
import io
import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('setup_macos_ci', Path(__file__).with_name('setup-macos-ci-signing.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class CISigningTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.env = {
            'GITHUB_ACTIONS': 'true', 'RUNNER_TEMP': str(self.root), 'GITHUB_ENV': str(self.root / 'env'),
            'MACOS_CERTIFICATE_P12_BASE64': base64.b64encode(b'synthetic-private-identity').decode(),
            'MACOS_CERTIFICATE_PASSWORD': 'synthetic-export-password', 'MACOS_SIGN_IDENTITY': 'SYNTHETICFINGERPRINT',
            'APPLE_ID': 'synthetic@example.invalid', 'APPLE_TEAM_ID': 'TESTTEAM',
            'APPLE_APP_SPECIFIC_PASSWORD': 'synthetic-notary-password',
        }
        self.calls = []
        self.platform = patch.object(release.sys, 'platform', 'darwin')
        self.platform.start()
        self.addCleanup(self.platform.stop)
        output = patch('sys.stdout', new_callable=io.StringIO)
        output.start()
        self.addCleanup(output.stop)

    def command(self, args, description):
        self.calls.append(args)
        if args[:2] == ['security', 'create-keychain']:
            Path(args[-1]).touch(mode=0o600)
        if args[:2] == ['security', 'delete-keychain']:
            Path(args[-1]).unlink()
        if args[:2] == ['security', 'import']:
            identity = Path(args[2])
            self.assertEqual(identity.stat().st_mode & 0o777, 0o600)
            self.assertEqual(identity.read_bytes(), b'synthetic-private-identity')
        if args[:2] == ['security', 'list-keychains'] and '-s' not in args:
            return '"/synthetic/login.keychain-db"\n'
        if args[:2] == ['security', 'find-identity']:
            return '1) SYNTHETICFINGERPRINT "Synthetic Developer ID"'
        return ''

    def test_setup_uses_private_files_and_exports_only_profile_paths(self):
        with patch.object(release, 'command', side_effect=self.command):
            release.setup(self.env)
        keychain, certificate, state = release.paths(self.env)
        self.assertFalse(certificate.exists())
        self.assertEqual(state.stat().st_mode & 0o777, 0o600)
        self.assertEqual(json.loads(state.read_text()), ['/synthetic/login.keychain-db'])
        exported = Path(self.env['GITHUB_ENV']).read_text()
        self.assertIn('MACOS_NOTARY_KEYCHAIN=' + str(keychain), exported)
        self.assertIn('MACOS_NOTARY_PROFILE=bambu-tui-notary', exported)
        self.assertNotIn(self.env['MACOS_CERTIFICATE_PASSWORD'], exported)
        self.assertNotIn(self.env['APPLE_APP_SPECIFIC_PASSWORD'], exported)
        credential = next(args for args in self.calls if args[:3] == ['xcrun', 'notarytool', 'store-credentials'])
        self.assertEqual(credential[credential.index('--keychain') + 1], str(keychain))
        imported = next(args for args in self.calls if args[:2] == ['security', 'import'])
        self.assertNotIn('-A', imported)
        self.assertIn('/usr/bin/codesign', imported)

    def test_cleanup_restores_search_list_and_deletes_credentials(self):
        with patch.object(release, 'command', side_effect=self.command):
            release.setup(self.env)
            release.cleanup(self.env)
            release.cleanup(self.env)  # Always-run cleanup is safe if already removed.
        keychain, certificate, state = release.paths(self.env)
        self.assertFalse(any(path.exists() for path in (keychain, certificate, state)))
        self.assertIn(['security', 'list-keychains', '-d', 'user', '-s', '/synthetic/login.keychain-db'], self.calls)

    def test_missing_or_malformed_secrets_do_not_modify_keychains(self):
        for changed in [{'APPLE_ID': ''}, {'MACOS_CERTIFICATE_P12_BASE64': 'not-base64!'}]:
            with patch.object(release, 'command', side_effect=self.command), self.assertRaises(ValueError):
                release.setup(self.env | changed)
        self.assertEqual(self.calls, [])

    def test_import_failure_removes_p12_and_allows_cleanup(self):
        def failed(args, description):
            if args[:2] == ['security', 'import']:
                raise ValueError('Import signing identity failed')
            return self.command(args, description)
        with patch.object(release, 'command', side_effect=failed), self.assertRaises(ValueError):
            release.setup(self.env)
        keychain, certificate, state = release.paths(self.env)
        self.assertFalse(certificate.exists())
        with patch.object(release, 'command', side_effect=self.command):
            release.cleanup(self.env)
        self.assertFalse(keychain.exists())

    def test_mismatched_identity_is_rejected_before_notary_credentials(self):
        with patch.object(release, 'command', side_effect=self.command), self.assertRaises(ValueError):
            release.setup(self.env | {'MACOS_SIGN_IDENTITY': 'DIFFERENTIDENTITY'})
        self.assertFalse(any(args[0] == 'xcrun' for args in self.calls))

    def test_notary_failure_does_not_export_profile_and_cleanup_still_works(self):
        def failed(args, description):
            if args[:3] == ['xcrun', 'notarytool', 'store-credentials']:
                raise ValueError('Store notarization credentials failed')
            return self.command(args, description)
        with patch.object(release, 'command', side_effect=failed), self.assertRaises(ValueError):
            release.setup(self.env)
        self.assertFalse(Path(self.env['GITHUB_ENV']).exists())
        with patch.object(release, 'command', side_effect=self.command):
            release.cleanup(self.env)
        self.assertFalse(any(path.exists() for path in release.paths(self.env)))

    def test_tool_errors_do_not_include_secret_argv_or_output(self):
        result = SimpleNamespace(returncode=1, stdout='secret-output', stderr='secret-output')
        with patch.object(release.subprocess, 'run', return_value=result), self.assertRaises(ValueError) as raised:
            release.command(['tool', 'secret-argument'], 'Synthetic operation')
        self.assertNotIn('secret-', str(raised.exception))

    def test_local_non_ci_execution_is_blocked(self):
        with patch.object(release, 'command', side_effect=self.command), self.assertRaises(ValueError):
            release.setup(self.env | {'GITHUB_ACTIONS': 'false'})
        self.assertEqual(self.calls, [])


if __name__ == '__main__':
    unittest.main()
