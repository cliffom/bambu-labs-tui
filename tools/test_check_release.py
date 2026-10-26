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
            'README.md': (b'# Install', 0o644),
            'LICENSE': (b'MIT', 0o644),
            'config.example.toml': (b'region = "global"', 0o644),
            'THIRD_PARTY_NOTICES.txt': (b'Go and dependency license notices', 0o644),
            release.EXECUTABLE: (binary, 0o755),
            release.CONTENTS + 'Info.plist': (plistlib.dumps(metadata), 0o644),
            release.CONTENTS + 'Resources/BambuTUI.icns': (b'synthetic-icon', 0o644),
            release.CONTENTS + '_CodeSignature/CodeResources': (b'synthetic-signature', 0o644),
        }

    def check(self, target='macos_arm64', version='0.2.0', extra_members=()):
        path = Path(self.temp.name) / ('bambu-tui_v' + version + '_' + target + '.tar.gz')
        with tarfile.open(path, 'w:gz') as archive:
            for name, (data, mode) in self.files.items():
                entry = tarfile.TarInfo(name)
                entry.size = len(data)
                entry.mode = mode
                archive.addfile(entry, io.BytesIO(data))
            for entry in extra_members:
                archive.addfile(entry)
        return release.archive_errors(path, version)

    def test_rejects_legacy_and_unknown_download_names(self):
        for target in ('darwin_arm64', 'windows_amd64', 'macos_x86_64'):
            self.assertTrue(any('invalid release archive filename' in e for e in self.check(target)))

    def test_rejects_unexpected_files_even_inside_helper_or_notices(self):
        for name in ('cmd/main.go', 'docs/README.md', 'AGENTS.md', 'assets/hero.png', 'third_party/dependency/main.go', 'third_party/dependency/LICENSE', 'third_party/Go-LICENSE', release.CONTENTS + 'Resources/extra.png', '../outside'):
            with self.subTest(name=name):
                self.files[name] = (b'unexpected', 0o644)
                self.assertTrue(any('unexpected archive entry' in e for e in self.check()))
                del self.files[name]

    def test_rejects_old_notice_directories_and_empty_consolidated_file(self):
        directory = tarfile.TarInfo('third_party/')
        directory.type = tarfile.DIRTYPE
        self.assertTrue(any('unexpected archive entry' in e for e in self.check(extra_members=(directory,))))
        self.files['THIRD_PARTY_NOTICES.txt'] = (b'', 0o644)
        self.assertIn('missing runtime file: THIRD_PARTY_NOTICES.txt', self.check())

    def test_duplicate_members_and_unsafe_links_fail(self):
        duplicate = tarfile.TarInfo('README.md')
        self.assertIn('duplicate archive entries', self.check(extra_members=(duplicate,)))
        for name, target in (('LICENSE', '/outside'), (release.CONTENTS + 'CodeResources', '../../outside')):
            entry = tarfile.TarInfo(name)
            entry.type = tarfile.SYMTYPE
            entry.linkname = target
            self.assertTrue(any('unexpected archive entry' in e for e in self.check(extra_members=(entry,))))
        # Notarization can add this safe, relative CodeResources link.
        entry = tarfile.TarInfo(release.CONTENTS + 'CodeResources')
        entry.type = tarfile.SYMTYPE
        entry.linkname = '_CodeSignature/CodeResources'
        self.assertEqual(self.check(extra_members=(entry,)), [])

    def test_required_runtime_guidance_and_licenses(self):
        for name in ('README.md', 'LICENSE', 'config.example.toml', 'THIRD_PARTY_NOTICES.txt'):
            with self.subTest(name=name):
                original = self.files.pop(name)
                self.assertIn('missing runtime file: ' + name, self.check())
                self.files[name] = original

    def test_macos_archives_require_the_complete_helper(self):
        for target in ('macos_arm64', 'macos_amd64'):
            self.assertEqual(self.check(target), [])
        del self.files[release.CONTENTS + 'Resources/BambuTUI.icns']
        self.assertTrue(any('missing helper file' in error for error in self.check()))

    def test_linux_archives_exclude_the_helper(self):
        self.assertIn('Linux archive contains macOS notification helper', self.check('linux_amd64'))
        self.files = {name: value for name, value in self.files.items() if not name.startswith(release.BUNDLE)}
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
