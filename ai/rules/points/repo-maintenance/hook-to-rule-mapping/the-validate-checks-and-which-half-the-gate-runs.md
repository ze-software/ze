---
kind: table
level:
stage:
---
| Check | Enforces | Triggers on | What it does |
|---|---|---|---|
| `check_source_anchor_line_numbers` | `writing.md` | every `docs/**/*.md` | Rejects a `<!-- source: -->` anchor that carries a line number, because line numbers rot. Gated: `ze-validate-tree`. BLOCKING. |
| `check_source_anchor_stale_paths` | `evidence.md` | every `docs/**/*.md` | Rejects a repo-relative anchor path that no file or directory answers. It resolves ANY root, which makes it the only gated check over the 74 anchors that point outside the nine `PATH_PREFIX` roots `scripts/dev/code_to_docs.py` walks (`docs/` 38, `tools/` 9, `gokrazy/` 9, `ai/` 8, `../` 7, `.github/` 2, `demos/` 1; all 74 resolve on 2026-08-09). Gated: `ze-validate-tree`. BLOCKING. |
| `check_spec_ac_completeness` | `completion.md`, `planning.md` | every `plan/spec-*.md` whose Status is `in-progress` | Rejects an acceptance-criterion row whose `Demonstrated By` cell is empty, so an in-flight spec cannot claim an AC with no named evidence. Gated: `ze-validate-tree`. BLOCKING. |
| `check_cross_package_wiring` | `completion.md` | `git diff HEAD` plus untracked `.go` files under `internal/` or `cmd/` | Reports an exported symbol with no cross-package non-test caller. UNENFORCED: no gate runs it (owner decision, 2026-08-09). Run `make ze-validate` by hand. |
| `check_cli_handler_coverage` | `testing.md` | the same changed-file list, `.go` files under the CLI paths only | Reports a newly registered command that no `.ci` test names. UNENFORCED: no gate runs it (owner decision, 2026-08-09). Run `make ze-validate` by hand. |
