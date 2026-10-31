#!/usr/bin/env python3
"""Offline checks for the repository's ATX headings and inline Markdown links."""

import argparse
from dataclasses import dataclass
import html
from pathlib import Path
import posixpath
import re
import subprocess
import sys
import tarfile
import unicodedata
from urllib.parse import unquote, urlsplit


RELEASE_README = "docs/user/release-installation.md"
ROOT_MARKDOWN = {"README.md", "AGENTS.md"}
DOCS_ROOT = {"docs/README.md", "docs/ROADMAP.md", "docs/CHANGELOG.md"}
VALIDATION = "docs/development/validation.md"
EVIDENCE_FIELDS = ("Scope", "Method", "Result", "Limits")
CHECKBOX = re.compile(r"^\s*(?:[-*+]|\d+[.)])\s+\[[ xX]\]", re.M)
LEVEL_TWO = re.compile(r"^ {0,3}##[ \t]+(.+?)[ \t]*#*[ \t]*$", re.M)
DIRECTION = {
    "docs/development/ui-direction.md",
    "docs/development/documentation-direction.md",
}
LINK = re.compile(
    r'!?\[([^\]\n]*)\]\(\s*(?:<([^>\n]+)>|((?:\\.|[^()\s]|\([^()\n]*\))+))'
    r'(?:\s+"[^"\n]*")?\s*\)'
)


@dataclass
class Tree:
    files: set[str]
    documents: dict[str, str]

    def exists(self, path):
        return path == "." or path in self.files or any(n.startswith(path.rstrip("/") + "/") for n in self.files)


def is_markdown(name):
    return Path(name).suffix.lower() in {".md", ".markdown"}


def prose(text):
    """Omit fenced code examples; retain inline code for heading text."""
    fence = None
    lines = []
    for line in text.splitlines():
        marker = re.match(r"^ {0,3}(`{3,}|~{3,})(.*)$", line)
        if fence:
            if marker and marker[1][0] == fence[0] and len(marker[1]) >= len(fence) and not marker[2].strip():
                fence = None
        elif marker:
            fence = marker[1]
        else:
            lines.append(line)
    return "\n".join(lines)


def anchors(text):
    result = set()
    for heading in re.findall(r"^ {0,3}#{1,6}\s+(.+?)\s*#*\s*$", prose(text), re.M):
        heading = LINK.sub(lambda m: m[1], heading)
        heading = re.sub(r"<[^>]+>", "", heading)
        heading = html.unescape(heading).lower()
        slug = "".join(c for c in heading if c in " -_" or unicodedata.category(c)[0] in "LN").replace(" ", "-")
        candidate = slug
        duplicate = 0
        while candidate in result:
            duplicate += 1
            candidate = f"{slug}-{duplicate}"
        result.add(candidate)
    return result


def local_links(name, text):
    # Inline code contains examples, not navigation. Backtick spans can span lines.
    text = re.sub(r"(`+).*?\1", "", prose(text), flags=re.S)
    for match in LINK.finditer(text):
        target = html.unescape(match[2] or match[3])
        url = urlsplit(target)
        if url.scheme or url.netloc:
            continue
        path = unquote(url.path)
        if path.startswith("/"):
            yield target, path, unquote(url.fragment)
        else:
            dest = posixpath.normpath(posixpath.join(posixpath.dirname(name), path)) if path else name
            yield target, dest, unquote(url.fragment)


def sections(text):
    """Return level-two sections outside fenced examples, including subheadings."""
    parts = LEVEL_TWO.split(prose(text))
    return [(parts[i], parts[i + 1]) for i in range(1, len(parts), 2)]


def ledger_errors(documents):
    errors = []
    validation = documents.get(VALIDATION, "")
    evidence = sections(validation)
    pending = [body for title, body in evidence if title == "Remaining live checks"]
    if len(pending) != 1:
        errors.append(f"{VALIDATION}: require one Remaining live checks section")
    if not any(title != "Remaining live checks" for title, _ in evidence):
        errors.append(f"{VALIDATION}: require a scoped evidence entry")
    # Include the preamble so tasks cannot silently escape the maintained list.
    preamble = LEVEL_TWO.split(prose(validation), maxsplit=1)[0]
    for title, body in [(None, preamble)] + evidence:
        if title == "Remaining live checks":
            continue
        if CHECKBOX.search(body):
            errors.append(f"{VALIDATION}: {title or 'preamble'}: checkboxes belong in Remaining live checks")
        if title is None:
            continue
        fields = re.findall(r"^- \*\*(Scope|Method|Result|Limits):\*\*([^\n]*)", body, re.M)
        if tuple(label for label, _ in fields) != EVIDENCE_FIELDS or any(not value.strip() for _, value in fields):
            errors.append(f"{VALIDATION}: {title}: require nonempty Scope, Method, Result, Limits fields in order")

    changelog = "docs/CHANGELOG.md"
    seen = set()
    releases = sections(documents.get(changelog, ""))
    if not releases:
        errors.append(f"{changelog}: require a release heading")
    for title, _ in releases:
        version = re.fullmatch(r"(\d+\.\d+\.\d+(?:-[\w.-]+)?(?:\+[\w.-]+)?)(?: — .+)?", title)
        key = version[1] if version else title
        if not version and title != "Unreleased":
            errors.append(f"{changelog}: {title}: use a version or Unreleased release heading")
        elif key in seen:
            errors.append(f"{changelog}: duplicate release heading: {key}")
        seen.add(key)

    roadmap = "docs/ROADMAP.md"
    for title, _ in sections(documents.get(roadmap, "")):
        if re.match(r"v?\d+\.\d+(?:\.|\b)", title, re.I):
            errors.append(f"{roadmap}: {title}: release history belongs in the changelog")
    for name in (changelog, roadmap):
        if CHECKBOX.search(prose(documents.get(name, ""))):
            errors.append(f"{name}: keep validation checkboxes in {VALIDATION}")
    return errors


def validate(tree):
    errors = []
    names = set(tree.documents)
    for missing in sorted((ROOT_MARKDOWN | DOCS_ROOT | DIRECTION | {RELEASE_README, VALIDATION}) - names):
        errors.append(f"missing required document: {missing}")
    all_links = {}
    heading_ids = {name: anchors(text) for name, text in tree.documents.items()}
    for name, text in sorted(tree.documents.items()):
        navigation = re.sub(r"(`+).*?\1", "", prose(text), flags=re.S)
        if re.search(r"^ {0,3}\[[^]\n]+\]:|<(?:a|img)\s", navigation, re.M | re.I):
            errors.append(f"{name}: use inline Markdown links/images so navigation can be verified")
        if name not in ROOT_MARKDOWN and not name.startswith("docs/"):
            errors.append(f"{name}: Markdown belongs under docs/")
        if name.startswith("docs/") and name.count("/") == 1 and name not in DOCS_ROOT:
            errors.append(f"{name}: detailed documents belong in a topic directory")
        if re.search(r"readiness|checklist|release[-_]checks", name, re.I) and re.search(r"v?\d+\.\d+", name, re.I):
            errors.append(f"{name}: version-specific readiness documents are forbidden")
        links = list(local_links(name, text))
        all_links[name] = {dest for _, dest, _ in links}
        for target, dest, anchor in links:
            if dest.startswith(("/", "../")):
                errors.append(f"{name}: use a relative link within the repository: {target}")
            elif not tree.exists(dest):
                errors.append(f"{name}: missing local target: {target}")
            elif anchor and dest in heading_ids and anchor not in heading_ids[dest]:
                errors.append(f"{name}: missing heading anchor: {target}")
    index_links = all_links.get("docs/README.md", set())
    for name in sorted(names - {"docs/README.md"}):
        if name.startswith("docs/") and name not in index_links:
            errors.append(f"{name}: add a direct link from docs/README.md")
    for name in sorted(DIRECTION - all_links.get("AGENTS.md", set())):
        errors.append(f"AGENTS.md: link required direction guide: {name}")
    errors.extend(ledger_errors(tree.documents))
    return errors


def repository(root):
    output = subprocess.check_output(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"], cwd=root
    )
    files = {n for n in output.decode().split("\0") if n and (root / n).is_file()}
    return Tree(files, {n: (root / n).read_bytes().decode("utf-8") for n in files if is_markdown(n)})


def archive_errors(path, source):
    with tarfile.open(path) as archive:
        members = archive.getmembers()
        files = {m.name.rstrip("/") for m in members if m.isfile()}
        documents = {
            m.name: archive.extractfile(m).read().decode("utf-8")
            for m in members if m.isfile() and is_markdown(m.name)
        }
    expected = {'README.md': source.documents[RELEASE_README]}
    errors = []
    for name in sorted(expected.keys() | documents.keys()):
        if name not in documents:
            errors.append(f"archive missing document: {name}")
        elif name not in expected:
            errors.append(f"archive has unexpected document: {name}")
        elif documents[name] != expected[name]:
            errors.append(f"archive has stale document: {name}")
    tree = Tree(files, documents)
    for name, text in documents.items():
        for target, dest, anchor in local_links(name, text):
            if dest.startswith(("/", "../")) or not tree.exists(dest):
                errors.append(f"{name}: missing local target: {target}")
            elif anchor and dest in documents and anchor not in anchors(documents[dest]):
                errors.append(f"{name}: missing heading anchor: {target}")
    return errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--archive", action="append", default=[], help="also check a release archive against source")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    source = repository(root)
    errors = validate(source)
    for path in args.archive:
        errors.extend(f"{path}: {error}" for error in archive_errors(path, source))
    if errors:
        print("Documentation checks failed:\n" + "\n".join(f"- {error}" for error in errors), file=sys.stderr)
        return 1
    print(f"Documentation checks passed ({len(source.documents)} documents, {len(args.archive)} archives)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
