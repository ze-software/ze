# 1067 -- Generated Discovery Indexes

## Context

Every session re-discovered the codebase: "what does package X do", "which files
implement design doc Y", "where is learned summary N". The repo had a deep rule and
history layer (80 rules, 1082 learned summaries, 100 architecture docs) but no
generated map of the code itself: the generated arch list named 134 directories with
zero descriptions, `ai/LEARNED-INDEX.md` curated only ~22% of summaries, and the
per-file `// Design:` edges were never inverted. Goal: make "what does what" a
generated build artifact sourced from the code and kept fresh by a gate, so a local
edit regenerates it and a stale index never ships. Motivating audit:
`AI-NAVIGATION-AUDIT.md`.

## Decisions

- Three generators mirroring `code_to_docs.py` / `rules_index.py`, each with a
  `--check` mode: `package_map.py` (`ai/PACKAGE-MAP.md`), `docs_to_code.py`
  (`ai/DOCS-TO-CODE.md`, the inverse of `// Design:`), `learned_index.py`
  (`ai/LEARNED-FULL-INDEX.md`, all summaries by number).
- Package responsibility from the `// Package` doc comment, else the register.go
  `Description`, else `TODO`. Chose pure-text parsing over importing the registry so a
  doc gate cannot fail because the Go build is temporarily broken.
- Skip pure-`embed.go` (yang/) packages so the TODO column is a real doc-coverage
  worklist, not schema noise.
- Wired the gate where it actually runs: `verify_wiring_docs.py` selects
  `ze-discovery-index-check` on register.go / `// Package`|`// Design:` .go /
  `plan/learned` edits. Folding into `ze-doc-test` alone would have been a no-op gate
  (wiring-completeness).
- No CI here, so the enforcement lives in `commit_helper.py`: the commit gate blocks a
  stale index (run `make ze-regen`) or a commit that changes a feeding source but
  omits the regenerated index; overridable with `--stale-index-ok`. A HEAD-consistency
  preflight additionally warns (non-blocking) when HEAD's committed index does not
  match HEAD's committed sources (a prior bypass), by re-running the generators against
  a materialized `git archive HEAD`, so it works even when the working tree is dirty.

## Consequences

- `ai/INDEX.md` gained an "understand existing code" front door; `discovery-updates.md`
  lists the three surfaces; `git-safety.md` documents the commit gate.
- Freshness is structural (index matches sources), enforced pre-commit via
  `commit_helper` and by `ze-regen-check`. Semantic truth of the one-liners is
  inherited from the comments: `// Design:` presence and target-validity are already
  enforced; prose truthfulness of `// Package` / `Description` is not mechanically
  checkable.
- The commit gate applies to every session, so no one can commit a feeding source
  without the regenerated index riding along.
- The 264 undocumented packages were backfilled with `// Package` doc comments (one new
  `doc.go` each, fanned out over parallel agents that each `go build`-verified their
  batch), so `ai/PACKAGE-MAP.md` now has zero TODO rows and describes every package.
- Per-subsystem flow digests were added under `ai/digests/` (reactor, wire/pools, rib,
  config pipeline, plugin transport, CLI): a living, hand-maintained orientation layer
  between the per-package map and the canonical `docs/architecture/`. The digest trace
  even surfaced two doc-vs-code drifts (the attribute package is `internal/core/bgp/
  attribute`, not `component`; there is no `PackContext` type).

## Gotchas

- `verify_wiring_docs` computes "changed" from the working tree only (no base ref), so
  on a clean CI checkout the doc/discovery gates are no-ops. Enforcement is the
  pre-commit developer loop plus the `commit_helper` gate.
- A file-head reader must not use `next(fh)` in a comprehension: PEP 479 turns the
  exhausted-file `StopIteration` into `RuntimeError`.
- The maps are computed from the whole working tree, so committing them while a
  parallel session holds uncommitted packages/summaries bakes references to that
  uncommitted work into history (accepted for this commit by owner decision).

## Files

- `scripts/dev/package_map.py`, `docs_to_code.py`, `learned_index.py` (+ a `_test.py` each)
- `ai/PACKAGE-MAP.md`, `ai/DOCS-TO-CODE.md`, `ai/LEARNED-FULL-INDEX.md` (generated)
- `scripts/dev/commit_helper.py` (+ `commit_helper_test.py`), `scripts/dev/verify_wiring_docs.py`
- `mk/inventory.mk`, `Makefile` (`ze-discovery-index`, `ze-regen`, gate in `ze-doc-test`)
- `ai/INDEX.md`, `ai/rules/discovery-updates.md`, `ai/rules/git-safety.md`
- `AI-NAVIGATION-AUDIT.md` (audit), `plan/learned/1067-generated-discovery-indexes.md`
