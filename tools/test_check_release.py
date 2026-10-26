import importlib.util
import io
from pathlib import Path
import plistlib
import struct
import tarfile
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('check_release', Path(__file__).with_name('check-release.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseInventoryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        # Synthetic architecture headers; native builds separately verify with lipo/codesign.
        binary = struct.pack('>II', 0xcafebabe, 2)
        binary += b''.join(struct.pack('>IIIII', cpu, 0, 0, 0, 0) for cpu in sorted(release.UNIVERSAL_CPUS))
        metadata = {'CFBundleIdentifier': release.BUNDLE_ID, 'CFBundleExecutable': 'bambu-notify', 'CFBundleIconFile': 'BambuTUI.icns', 'CFBundleShortVersionString': '0.2.0', 'CFBundleVersion': '0.2.0', 'BambuTUIReleaseVersion': '0.2.0', 'LSUIElement': True}
        self.files = {
            'bambu-tui': (b'synthetic-cli', 0o755),
            release.EXECUTABLE: (binary, 0o755),
            release.CONTENTS + 'Info.plist': (plistlib.dumps(metadata), 0o644),
            release.CONTENTS + 'Resources/BambuTUI.icns': (b'synthetic-icon', 0o644),
            release.CONTENTS + '_CodeSignature/CodeResources': (b'synthetic-signature', 0o644),
        }

    def check(self, target='darwin_arm64', version='0.2.0'):
        path = Path(self.temp.name) / ('bambu-tui_v0.2.0_' + target + '.tar.gz')
        with tarfile.open(path, 'w:gz') as archive:
            for name, (data, mode) in self.files.items():
                entry = tarfile.TarInfo(name)
                entry.size = len(data)
                entry.mode = mode
                archive.addfile(entry, io.BytesIO(data))
        return release.archive_errors(path, version)

    def test_macos_archives_require_the_complete_helper(self):
        for target in ('darwin_arm64', 'darwin_amd64'):
            self.assertEqual(self.check(target), [])
        del self.files[release.CONTENTS + 'Resources/BambuTUI.icns']
        self.assertTrue(any('missing helper file' in error for error in self.check()))

    def test_linux_archives_exclude_the_helper(self):
        self.assertIn('Linux archive contains macOS notification helper', self.check('linux_amd64'))
        self.files = {'bambu-tui': self.files['bambu-tui']}
        self.assertEqual(self.check('linux_arm64'), [])

    def test_executable_mode_architectures_and_version_are_checked(self):
        self.files[release.EXECUTABLE] = (b'thin-or-invalid-binary', 0o644)
        errors = self.check(version='0.2.1')
        self.assertIn('notification helper is not executable', errors)
        self.assertIn('notification helper must contain arm64 and x86_64 slices', errors)
        self.assertIn('notification helper identity/version/icon metadata mismatch', errors)

    def test_prerelease_keeps_numeric_app_version_and_full_release_identity(self):
        metadata = plistlib.loads(self.files[release.CONTENTS + 'Info.plist'][0])
        metadata['BambuTUIReleaseVersion'] = '0.2.0-rc1'
        self.files[release.CONTENTS + 'Info.plist'] = (plistlib.dumps(metadata), 0o644)
        self.assertEqual(self.check(version='0.2.0-rc1'), [])

    def test_missing_signature_and_cli_fail(self):
        del self.files[release.CONTENTS + '_CodeSignature/CodeResources']
        del self.files['bambu-tui']
        errors = self.check()
        self.assertIn('missing or non-executable CLI', errors)
        self.assertTrue(any('_CodeSignature' in error for error in errors))

    def test_malformed_plist_and_truncated_binary_fail(self):
        self.files[release.CONTENTS + 'Info.plist'] = (b'invalid plist', 0o644)
        self.files[release.EXECUTABLE] = (struct.pack('>II', 0xcafebabe, 2), 0o755)
        errors = self.check()
        self.assertIn('invalid notification helper plist', errors)
        self.assertIn('notification helper must contain arm64 and x86_64 slices', errors)


if __name__ == '__main__':
    unittest.main()
