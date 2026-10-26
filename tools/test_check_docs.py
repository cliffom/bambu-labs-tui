"""Regression tests for documentation failures that should block a PR/release."""

import importlib.util
import io
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest


spec = importlib.util.spec_from_file_location("check_docs", Path(__file__).with_name("check-docs.py"))
docs = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = docs
spec.loader.exec_module(docs)


class DocumentationChecks(unittest.TestCase):
    def setUp(self):
        names = docs.ROOT_MARKDOWN | docs.DOCS_ROOT | docs.DIRECTION | {docs.RELEASE_README}
        self.documents = {n: "# Guide\n" for n in names}
        self.documents["docs/README.md"] += "".join(
            f"\n[Guide]({n.removeprefix('docs/')})" for n in sorted(names) if n.startswith("docs/") and n != "docs/README.md"
        )
        self.documents["AGENTS.md"] += "".join(f"\n[Direction]({n})" for n in sorted(docs.DIRECTION))

    def tree(self):
        return docs.Tree(set(self.documents) | {"assets/hero.png"}, self.documents)

    def test_valid_tree_and_navigation(self):
        self.documents["README.md"] += "\n![Hero](assets/hero.png)\n[Guide](docs/README.md#guide)\n"
        self.assertEqual(docs.validate(self.tree()), [])

    def test_missing_targets_and_anchors(self):
        self.documents["README.md"] += "\n[Broken](docs/missing.md)\n[Wrong heading](docs/README.md#missing)\n"
        errors = docs.validate(self.tree())
        self.assertTrue(any("missing local target" in e for e in errors))
        self.assertTrue(any("missing heading anchor" in e for e in errors))

    def test_placement_index_and_readiness(self):
        for name, message in [
            (".github/notes.MD", "Markdown belongs under docs/"),
            ("docs/extra.md", "topic directory"),
            ("docs/user/orphan.md", "add a direct link"),
            ("docs/development/v0.2.0-readiness.md", "readiness documents are forbidden"),
            ("docs/development/v0.2.0-checklist.md", "readiness documents are forbidden"),
        ]:
            with self.subTest(name=name):
                tree = self.tree()
                tree = docs.Tree(tree.files | {name}, tree.documents | {name: "# New\n"})
                self.assertTrue(any(message in e for e in docs.validate(tree)))

    def test_missing_entry_point_or_agent_direction(self):
        del self.documents["docs/ROADMAP.md"]
        self.documents["AGENTS.md"] = "# Agent\n"
        errors = docs.validate(self.tree())
        self.assertIn("missing required document: docs/ROADMAP.md", errors)
        self.assertEqual(sum("link required direction guide" in e for e in errors), 2)

    def test_fences_inline_code_and_external_urls_are_not_local_links(self):
        self.documents["README.md"] += '''
````md
[Example](missing.md)
```
[Still an example](missing-again.md)
````
~~~sh
[Example](missing.md)
~~~
`[inline](missing.md)`
[External](https://example.invalid/missing#heading)
'''
        self.assertEqual(docs.validate(self.tree()), [])

    def test_heading_slugs_duplicates_and_encoded_paths(self):
        self.documents["docs/user/with spaces.md"] = "# **Hello** `world`!\n# Hello world\n# Hello world-1\n# Café\n"
        self.documents["docs/README.md"] += "\n[Extra](<user/with spaces.md>)\n"
        self.documents["README.md"] += "\n[First](docs/user/with%20spaces.md#hello-world)\n[Second](docs/user/with%20spaces.md#hello-world-1)\n[Collision](docs/user/with%20spaces.md#hello-world-1-1)\n[Unicode](docs/user/with%20spaces.md#caf%C3%A9)\n"
        self.assertEqual(docs.validate(self.tree()), [])

    def test_reference_definitions_and_html_navigation_require_inline_links(self):
        self.documents["README.md"] += '\n[ref]: missing.md\n<a href="missing.md">Link</a>\n'
        self.assertTrue(any("use inline Markdown links/images" in e for e in docs.validate(self.tree())))

    def test_local_link_cannot_escape_repository(self):
        self.documents["README.md"] += "\n[Outside](../outside.md)\n[Absolute](/etc/hosts)\n"
        self.assertEqual(sum("relative link within" in e for e in docs.validate(self.tree())), 2)

    def test_git_inventory_includes_new_hidden_markdown_but_ignores_builds(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            subprocess.run(["git", "init", "-q", tmp], check=True)
            (root / ".gitignore").write_text("dist/\n")
            (root / "README.md").write_text("# App\n")
            subprocess.run(["git", "add", "."], cwd=root, check=True)
            (root / ".github").mkdir()
            (root / ".github/extra.markdown").write_text("# Stray\n")
            (root / "dist").mkdir()
            (root / "dist/README.md").write_text("# Bundled\n")
            inventory = docs.repository(root)
            self.assertEqual(set(inventory.documents), {"README.md", ".github/extra.markdown"})
            (root / "README.md").unlink()
            self.assertNotIn("README.md", docs.repository(root).documents)

    def write_archive(self, path, documents, files=("assets/hero.png",)):
        with tarfile.open(path, "w:gz") as archive:
            for name, text in documents.items():
                data = text.encode()
                member = tarfile.TarInfo(name)
                member.size = len(data)
                archive.addfile(member, io.BytesIO(data))
            for name in files:
                archive.addfile(tarfile.TarInfo(name), io.BytesIO())

    def test_archive_checks_install_guide_instead_of_full_repository(self):
        guide = "# Install\n[Config](config.example.toml)\n[License](LICENSE)\n[Help](https://example.invalid/docs)\n"
        self.documents['docs/user/release-installation.md'] = guide
        source = self.tree()
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "release.tar.gz"
            documents = {'README.md': guide}
            files = ('LICENSE', 'config.example.toml')
            self.write_archive(path, documents, files=files)
            self.assertEqual(docs.archive_errors(path, source), [])
            self.write_archive(path, {}, files=files)
            self.assertIn('archive missing document: README.md', docs.archive_errors(path, source))
            self.write_archive(path, documents | {'README.md': '# Stale\n', 'AGENTS.md': '# Agent\n', 'docs/README.md': '# Docs\n'}, files=files)
            errors = docs.archive_errors(path, source)
            self.assertIn('archive has stale document: README.md', errors)
            self.assertIn('archive has unexpected document: AGENTS.md', errors)
            self.assertIn('archive has unexpected document: docs/README.md', errors)
            self.write_archive(path, documents, files=())
            self.assertTrue(any('missing local target: config.example.toml' in e for e in docs.archive_errors(path, source)))


if __name__ == "__main__":
    unittest.main()
