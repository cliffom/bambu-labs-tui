import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('third_party_notices', Path(__file__).with_name('third-party-notices.py'))
notices = importlib.util.module_from_spec(spec)
spec.loader.exec_module(notices)


class ThirdPartyNoticesTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.go_root = self.root / 'go'
        self.go_root.mkdir()
        (self.go_root / 'LICENSE').write_bytes(b'Go copyright\r\nGo terms\r\n')

    def dependency(self, module, files):
        directory = self.root / ('dependency ' + module.replace('/', '_'))
        directory.mkdir()
        for name, content in files.items():
            (directory / name).write_bytes(content)
        return f'{module} {directory}\n'

    def test_all_notice_bytes_and_attribution_are_preserved_in_stable_order(self):
        copyright = 'Copyright example contributor ©'.encode()
        a = self.dependency('example.org/a', {'LICENSE.txt': copyright, 'NOTICE': b'Notice text\r\n', 'COPYING': b'Additional terms', 'README.md': b'Not a notice'})
        b = self.dependency('example.org/b', {'LICENSE': b'Another dependency license\n'})
        own = f'{notices.PROJECT_MODULE} {self.root}\n'
        result = notices.collect_notices(self.go_root, b + a + a + own)
        self.assertEqual(result, notices.collect_notices(self.go_root, own + a + b))
        for content in (b'Go copyright\r\nGo terms\r\n', copyright, b'Notice text\r\n', b'Additional terms', b'Another dependency license\n'):
            self.assertEqual(result.count(content), 1)
        for heading in ('Go / LICENSE', 'example.org/a / COPYING', 'example.org/a / LICENSE.txt', 'example.org/a / NOTICE', 'example.org/b / LICENSE'):
            self.assertIn(('===== ' + heading + ' =====').encode(), result)
        self.assertNotIn(b'Not a notice', result)
        self.assertNotIn(str(self.root).encode(), result)
        self.assertLess(result.index(b'example.org/a'), result.index(b'example.org/b'))

    def test_homebrew_go_license_fallback(self):
        (self.go_root / 'LICENSE').unlink()
        (self.go_root.parent / 'LICENSE').write_bytes(b'Homebrew Go license')
        self.assertIn(b'Homebrew Go license', notices.collect_notices(self.go_root, ''))

    def test_missing_empty_or_directory_notices_fail(self):
        manifest = self.dependency('example.org/missing', {})
        directory = Path(manifest.split(maxsplit=1)[1].strip())
        (directory / 'LICENSE-directory').mkdir()
        with self.assertRaisesRegex(ValueError, 'no license found'):
            notices.collect_notices(self.go_root, manifest)
        (directory / 'LICENSE').write_bytes(b' \n')
        with self.assertRaisesRegex(ValueError, 'empty notice'):
            notices.collect_notices(self.go_root, manifest)
        (self.go_root / 'LICENSE').unlink()
        with self.assertRaises(OSError):
            notices.collect_notices(self.go_root, '')

    def test_conflicting_or_malformed_module_manifest_fails(self):
        with self.assertRaisesRegex(ValueError, 'multiple source directories'):
            notices.collect_notices(self.go_root, 'example.org/a /one\nexample.org/a /two\n')
        with self.assertRaises(ValueError):
            notices.collect_notices(self.go_root, 'no-directory\n')


if __name__ == '__main__':
    unittest.main()
