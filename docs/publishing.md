# Publishing the initial repository

The implementation workspace cannot create its protected `.git` directory. The connected GitHub integration can read `cliffom/bambu-labs-tui` but rejected a write with `403 Resource not accessible by integration`. No remote commit, branch, tag, or release was created.

A self-contained Git bundle is available in `dist/bambu-labs-tui-v0.0.1.bundle`. It contains the reviewed source and validation record on `main`. To import it from a normal terminal, choose a new directory and run:

```sh
git clone /absolute/path/to/bambu-labs-tui-v0.0.1.bundle bambu-labs-tui
cd bambu-labs-tui
git remote set-url origin https://github.com/cliffom/bambu-labs-tui.git
git push -u origin main
```

Use your normal GitHub authentication. If the remote has changed since it was inspected, reconcile that history before pushing; do not force-push over it. The bundle can also be inspected without importing it:

```sh
git bundle list-heads /absolute/path/to/bambu-labs-tui-v0.0.1.bundle
# Run verify inside the imported Git repository:
git bundle verify /absolute/path/to/bambu-labs-tui-v0.0.1.bundle
```

After CI passes and the remaining checks in [validation.md](validation.md) are completed, the release can be prepared:

```sh
git tag -a v0.0.1 -m 'Bambu TUI v0.0.1'
git push origin v0.0.1
```

The tag workflow builds archives and checksums and creates a **draft** GitHub release. Review its artifacts and notes before publishing it. Local candidates are stamped with the bundled source commit when packaged. Running Make without Git metadata defaults to an `unknown` commit; rebuilding from the imported repository reads the commit directly from Git.
