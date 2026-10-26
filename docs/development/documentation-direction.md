# Documentation direction

[Documentation index](../README.md) · [UI direction](ui-direction.md) · [Contributing](contributing.md)

## Readers and sources of truth

Keep documentation useful for people discovering, using, and maintaining the current application. Describe implemented behavior in user guides and planned behavior in the roadmap. Use plain language, concrete commands, and links to detailed references. Keep the README short enough to scan while browsing the repository: purpose, supported setup, installation, demo, first connection, essential controls, and where to learn more.

| Location | Owns |
| --- | --- |
| Root `README.md` | Human-facing introduction and getting started |
| Root `AGENTS.md` | Required agent workflow, repository map, and links to detailed policy |
| `docs/README.md` | Index linking every maintained Markdown document under `docs/` |
| `docs/user/` | Current controls, configuration, behavior, and troubleshooting |
| `docs/development/` | Contributor setup, architecture, UI/documentation direction, testing, validation, and publishing |
| `docs/reference/` | Protocol evidence and branding asset guidance |
| `docs/ROADMAP.md` | Delivered baseline, open priorities, milestone intent, and issue links |
| `docs/CHANGELOG.md` | Release history |

Only root `README.md` and `AGENTS.md` may be Markdown files outside `docs/`, including in hidden directories. Keep the root of `docs/` limited to its index, roadmap, and changelog; put detailed material in a topic directory. Add each document to the index and link new policy from `AGENTS.md` when agents need it. Prefer updating an existing authoritative document to adding a parallel explanation. Other guides should summarize and link rather than duplicate full procedures or behavior tables.

## Changes that require documentation review

| Change | Review and update when affected |
| --- | --- |
| Keys, dialogs, labels, sorting, alerts, or notice lifetime | Relevant user guide, dashboard help/dialog hints, and [UI direction](ui-direction.md) |
| CLI, login, installation, platform requirements, configuration | User guide; README if getting started changes; configuration example if defaults/options change |
| State ownership, concurrency, persistence, package boundaries | Architecture and relevant contributor/testing guidance |
| Build/check commands, tool pins, workflows, release layout | Contributing, publishing, `AGENTS.md` commands, and affected tooling |
| Shipped/planned scope or issue/milestone status | Roadmap; changelog when preparing a release |
| New hardware evidence or limitations | Ongoing validation record, with application version, host/terminal, model/firmware, date, and observed results |
| File moves, new topics, or deletions | Index, agent map, inbound links/anchors, source comments, and release references |

Every change must assess documentation impact. Update affected guidance in the same PR. If no documentation changes are needed, explain why in the PR rather than making a cosmetic edit to satisfy a check. Automated checks do not determine whether prose matches behavior, so this assessment is a review requirement.

Preserve historical release facts in the changelog. Keep completed work out of the roadmap's next-work queue and verify issue status when refreshing priorities. Do not create version-specific readiness/checklist documents; put ongoing operational checks and actual evidence in the validation record. Keep unperformed checks unchecked. Synthetic tests, demo runs, issue closure, and release publication do not count as hardware validation. Never add private telemetry or credentials to documentation.

## Verifiable rules

Run `make docs-check` for every PR. It uses Python 3.10+ and the standard library, with no network access, and runs in the Formatting and lint workflow. It checks the Git inventory (including untracked, non-ignored Markdown), placement, required entry points/direction guides, version-specific readiness filenames, local link/image targets and heading anchors, index coverage, and agent links to both direction guides. Use ATX headings (`# Heading`) and standard inline Markdown links/images for local navigation. Reference-link definitions and raw HTML navigation are rejected so links cannot silently bypass verification. External URLs are not fetched; their accuracy and the meaning of the prose require review.

The checker tests also exercise broken links/anchors, misplaced/orphaned documents, and incomplete or stale archive contents. `make release` runs the same checks before packaging and validates the documentation bytes and local navigation in every generated archive. Binary archives contain `docs/user/release-installation.md` renamed to `README.md`, the configuration example, and license notices, rather than the repository documentation tree or artwork. Archive checks verify that this guide matches its source and its local links resolve; release inventory checks reject unexpected bundled files. Full documentation is linked online. The draft-release workflow reads `docs/CHANGELOG.md`; publishing instructions and tooling must agree on that path.

In the PR description, report documentation impact, the checks actually run, and any unverified visual/hardware behavior. For UI changes, follow the UI direction's evidence requirements. When intentionally changing a documentation convention, update this guide, `AGENTS.md`, the checker/tests, and tooling as applicable in the same PR; do not bypass the checker to accommodate drift. Passing checks validates structure and navigation, not editorial or visual correctness.
