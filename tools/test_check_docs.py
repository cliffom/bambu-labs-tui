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
        names = docs.ROOT_MARKDOWN | docs.DOCS_ROOT | docs.DIRECTION | {docs.RELEASE_README, docs.VALIDATION}
        self.documents = {n: "# Guide\n" for n in names}
        self.documents["docs/CHANGELOG.md"] += "\n## 1.0.0 — Initial release\n\n- Add monitoring.\n"
        self.documents[docs.VALIDATION] += """
## Monitor startup (#1)

- **Scope:** 2026-10-31; v1.0.0 working tree based on abc123.
- **Method:** Synthetic reports in a local PTY at 60 × 18.
- **Result:** Startup and resize assertions passed; capture not archived.
- **Limits:** No hardware or physical-terminal inspection.

## Remaining live checks

- [ ] Observe a real printer (#1).
"""
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

    def test_evidence_requires_complete_ordered_nonempty_fields(self):
        original = self.documents[docs.VALIDATION]
        for replacement in (
            "",  # Omitted result.
            "- **Result:**\n",  # Empty result.
            "- **Limits:** No hardware.\n- **Result:** Passed.\n",  # Out of order/duplicate limit.
            "```md\n- **Result:** Passed.\n```\n",  # Example is not evidence.
        ):
            with self.subTest(replacement=replacement):
                self.documents[docs.VALIDATION] = original.replace(
                    "- **Result:** Startup and resize assertions passed; capture not archived.\n", replacement
                )
                self.assertTrue(any("fields in order" in e for e in docs.validate(self.tree())))

    def test_new_unstructured_entry_cannot_reuse_previous_evidence_fields(self):
        original = self.documents[docs.VALIDATION]
        for heading in ("## Another check", "  ## Another check ##"):
            with self.subTest(heading=heading):
                self.documents[docs.VALIDATION] = original + f"\n{heading}\n\nEverything passed.\n"
                errors = docs.validate(self.tree())
                self.assertTrue(any("Another check: require nonempty" in e for e in errors))

    def test_subheadings_keep_evidence_scope_and_examples_do_not_create_entries(self):
        self.documents[docs.VALIDATION] += """
```md
## Example
- [ ] Example only.
```
"""
        self.documents[docs.VALIDATION] = self.documents[docs.VALIDATION].replace(
            "## Remaining live checks", "### Retained capture\n\nNo raw telemetry retained.\n\n## Remaining live checks"
        )
        self.assertEqual(docs.validate(self.tree()), [])

    def test_outstanding_checks_have_one_home(self):
        original = self.documents[docs.VALIDATION]
        for text in (
            original.replace("## Remaining live checks", "## Other checks"),
            original + "\n## Remaining live checks\n\n- [ ] Duplicate task.\n",
            original.replace("## Monitor startup", "- [ ] Preamble task.\n\n## Monitor startup"),
            original.replace("- **Limits:**", "- [x] Historical task.\n- **Limits:**"),
        ):
            with self.subTest(text=text):
                self.documents[docs.VALIDATION] = text
                self.assertTrue(any("Remaining live checks" in e for e in docs.validate(self.tree())))

    def test_changelog_release_headings_are_unique_and_versioned(self):
        for heading in ("## 1.0.0 — Repeated", "## Test diary", "## Unreleased\n\n## Unreleased"):
            with self.subTest(heading=heading):
                self.documents["docs/CHANGELOG.md"] = "# Changelog\n\n## 1.0.0 — First\n\n" + heading + "\n"
                self.assertTrue(any("release heading" in e for e in docs.validate(self.tree())))
        self.documents["docs/CHANGELOG.md"] = "# Changelog\n\n## Unreleased\n\n## 1.1.0-rc.1 — Candidate\n\n## 1.0.0\n"
        self.assertEqual(docs.validate(self.tree()), [])

    def test_roadmap_rejects_release_history_but_allows_current_version_in_prose(self):
        self.documents["docs/ROADMAP.md"] += "\nCurrent baseline: v1.0.0. See the changelog.\n"
        self.assertEqual(docs.validate(self.tree()), [])
        self.documents["docs/ROADMAP.md"] += "\n## v1.0.0 — Delivered work\n"
        self.assertTrue(any("release history belongs" in e for e in docs.validate(self.tree())))

    def test_changelog_and_roadmap_cannot_duplicate_validation_tasks(self):
        for name in ("docs/CHANGELOG.md", "docs/ROADMAP.md"):
            for marker in ("- [ ]", "* [x]", "1. [ ]", "2) [X]"):
                with self.subTest(name=name, marker=marker):
                    original = self.documents[name]
                    self.documents[name] += f"\n{marker} Test notifications.\n"
                    self.assertTrue(any("keep validation checkboxes" in e for e in docs.validate(self.tree())))
                    self.documents[name] = original

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
