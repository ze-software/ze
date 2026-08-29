---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `checkSourceAnchorLineNumbers` | `writing.md` | every `docs/**/*.md` | Rejects a `<!-- source: -->` anchor that carries a line number, because line numbers rot. Gated: `./le repository tree-check`. BLOCKING. |
| `checkSourceAnchorStalePaths` | `evidence.md` | every `docs/**/*.md` | Rejects a repo-relative anchor path that no file or directory answers. It resolves ANY root, including anchors outside the `PATH_PREFIX` roots that `internal/le/docstocode/codetodocs.go` walks. Gated: `./le repository tree-check`. BLOCKING. |
| `checkSpecACCompleteness` | `completion.md`, `planning.md` | every `plan/spec-*.md` whose Status is `in-progress` | Rejects an acceptance-criterion row whose `Demonstrated By` cell is empty, so an in-flight spec cannot claim an AC with no named evidence. Gated: `./le repository tree-check`. BLOCKING. |
| `checkCrossPackageWiring` | `completion.md` | `git diff HEAD` plus untracked `.go` files under `internal/` or `cmd/` | Reports an exported symbol with no cross-package non-test caller. UNENFORCED: no gate runs it (owner decision, 2026-08-09). Run `./le repository check` by hand. |
| `checkCLIHandlerCoverage` | `testing.md` | the same changed-file list, `.go` files under the CLI paths only | Reports a newly registered command that no `.ci` test names. UNENFORCED: no gate runs it (owner decision, 2026-08-09). Run `./le repository check` by hand. |
