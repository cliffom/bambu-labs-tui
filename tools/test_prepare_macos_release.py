import importlib.util
import io
from pathlib import Path
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('prepare_macos_release', Path(__file__).with_name('prepare-macos-release.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)

METADATA = 'Authority=Developer ID Application: Synthetic\nTeamIdentifier=TESTTEAM\nTimestamp=Oct 26 2026\nflags=0x10000(runtime)\n'


class MacOSSigningTests(unittest.TestCase):
    def setUp(self):
        output = patch('sys.stdout', new_callable=io.StringIO)
        output.start()
        self.addCleanup(output.stop)
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.dist = Path(self.temp.name)
        helper = self.dist / 'notify-helper' / release.BUNDLE
        helper.mkdir(parents=True)
        (helper / 'synthetic-resource').write_text('synthetic')
        for arch in ('arm64', 'amd64'):
            (self.dist / ('bambustand-darwin-' + arch)).write_text('synthetic')
        self.calls = []
        self.status = 'Accepted'
        self.notary_exit = 0

    def command(self, args, capture=False, check=True):
        self.calls.append(args)
        output = ''
        if args[:2] == ['codesign', '--display']:
            output = METADATA
        elif args[:3] == ['xcrun', 'notarytool', 'submit']:
            output = '{"id":"synthetic-submission","status":"' + self.status + '"}'
            return SimpleNamespace(stdout=output, stderr='', returncode=self.notary_exit)
        return SimpleNamespace(stdout='', stderr=output, returncode=0)

    def test_default_candidate_does_not_use_signing_or_apple_services(self):
        with patch.object(release, 'command', side_effect=self.command):
            release.prepare(self.dist, '', '')
        self.assertEqual(self.calls, [])
        with self.assertRaises(ValueError):
            release.prepare(self.dist, '', 'synthetic-profile')

    def test_signed_candidate_signs_all_macos_code_without_notarization(self):
        with patch.object(release, 'command', side_effect=self.command):
            release.prepare(self.dist, 'synthetic-identity', '')
        signing = [args for args in self.calls if args[:2] == ['codesign', '--force']]
        self.assertEqual(len(signing), 3)
        identities = [args[args.index('--identifier') + 1] for args in signing]
        self.assertEqual(identities, ['com.suitepotato.bambustand.notify',
                                     'com.suitepotato.bambustand', 'com.suitepotato.bambustand'])
        for args in signing:
            self.assertIn('--timestamp', args)
            self.assertEqual(args[args.index('--options') + 1], 'runtime')
        self.assertFalse(any(args[0] == 'xcrun' for args in self.calls))

    def test_rejected_or_interrupted_notarization_never_staples(self):
        for status, code in [('Invalid', 65), ('In Progress', 75), ('Accepted', 1)]:
            self.calls = []
            self.status, self.notary_exit = status, code
            with patch.object(release, 'command', side_effect=self.command), self.assertRaises(ValueError):
                release.prepare(self.dist, 'synthetic-identity', 'synthetic-profile')
            self.assertFalse(any(args[:2] == ['xcrun', 'stapler'] for args in self.calls))
            self.assertIn(status, (self.dist / 'notarization' / 'submission.json').read_text())

    def test_accepted_submission_is_stapled_then_validated(self):
        with patch.object(release, 'command', side_effect=self.command):
            release.prepare(self.dist, 'synthetic-identity', 'synthetic-profile')
        staple = next(i for i, args in enumerate(self.calls) if args[:3] == ['xcrun', 'stapler', 'staple'])
        validate = next(i for i, args in enumerate(self.calls) if args[:3] == ['xcrun', 'stapler', 'validate'])
        self.assertLess(staple, validate)
        self.assertTrue((self.dist / 'notarization' / 'submission.json').is_file())

    def test_ci_keychain_is_used_for_submission_and_failure_log(self):
        self.status, self.notary_exit = 'Invalid', 65
        with patch.object(release, 'command', side_effect=self.command), self.assertRaises(ValueError):
            release.prepare(self.dist, 'synthetic-identity', 'synthetic-profile', '/synthetic/ci.keychain-db')
        notary = [args for args in self.calls if args[:2] == ['xcrun', 'notarytool']]
        self.assertEqual(len(notary), 2)
        for args in notary:
            self.assertEqual(args[args.index('--keychain') + 1], '/synthetic/ci.keychain-db')

    def test_incomplete_or_ad_hoc_signatures_are_rejected(self):
        for metadata in [METADATA.replace('Developer ID Application:', 'Apple Development:'),
                         METADATA.replace('Timestamp=', 'Signed Time='),
                         METADATA.replace('(runtime)', ''), METADATA.replace('TESTTEAM', 'not set')]:
            def fake(args, capture=False, check=True):
                return SimpleNamespace(stdout='', stderr=metadata, returncode=0)
            with patch.object(release, 'command', side_effect=fake), self.assertRaises(ValueError):
                release.verify(self.dist / 'synthetic')

    def test_mixed_teams_and_failed_native_verification_are_rejected(self):
        with patch.object(release, 'verify', side_effect=['TESTONE', 'TESTTWO']), self.assertRaises(ValueError):
            release.verify_all([Path('first'), Path('second')])
        with patch.object(release, 'command', side_effect=subprocess.CalledProcessError(1, 'codesign')), self.assertRaises(subprocess.CalledProcessError):
            release.verify(self.dist / 'synthetic')

    def test_signing_or_stapling_failure_stops_preparation(self):
        for failing in [('codesign', '--force'), ('xcrun', 'stapler', 'staple')]:
            self.calls = []
            def fake(args, capture=False, check=True):
                if tuple(args[:len(failing)]) == failing:
                    raise subprocess.CalledProcessError(1, args)
                return self.command(args, capture, check)
            with patch.object(release, 'command', side_effect=fake), self.assertRaises(subprocess.CalledProcessError):
                release.prepare(self.dist, 'synthetic-identity', 'synthetic-profile')
            self.assertFalse(any(args[:3] == ['xcrun', 'stapler', 'validate'] for args in self.calls))

    def test_malformed_reports_do_not_staple(self):
        for output in ['', '[]', '{broken']:
            self.calls = []
            def fake(args, capture=False, check=True):
                if args[:3] == ['xcrun', 'notarytool', 'submit']:
                    return SimpleNamespace(stdout=output, stderr='', returncode=0)
                return self.command(args, capture, check)
            with patch.object(release, 'command', side_effect=fake), self.assertRaises(ValueError):
                release.prepare(self.dist, 'synthetic-identity', 'synthetic-profile')
            self.assertFalse(any(args[:2] == ['xcrun', 'stapler'] for args in self.calls))


if __name__ == '__main__':
    unittest.main()
