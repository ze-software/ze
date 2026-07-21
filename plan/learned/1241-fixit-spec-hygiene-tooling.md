# 1241 -- fixit-spec-hygiene-tooling

## Context

The spec/plan system had no freshness gate, so it rotted silently: specs cited sibling
`plan/spec-*.md` files and `path:line` locations that drift or vanish, the `ze-spec-status`
view did not separate committed backlog from idea-capture skeletons, and the `.ci`-sleep
ratchet was an absolute floor that merge-conflicted whenever two sessions each removed a
sleep. This adds a citation-freshness gate, a bucketed status view with a skeleton TTL, a
composable delta ratchet, and a non-blocking closure advisory.

## Decisions

- **Citation gate FAILs on spec->spec dangling, WARNs on line-token drift, with a BASELINE**
  (`scripts/dev/spec-citation-check.py` + `plan/.citation-baseline`). A `plan/spec-*.md` that
  cites an absent sibling `plan/spec-*.md` exits 1; a `` `path:line` `` whose adjacent token
  left that line is a non-fatal WARN. The baseline (46 current spec->spec dangling) makes the
  ALREADY-ROTTED tree pass exit-0, so the gate ratchets rot DOWN without a flag-day cleanup.
- **learned->spec references are EXCLUDED from FAIL.** The two-commit closure model leaves a
  `plan/learned/NNN-<stem>.md` referencing its now-removed spec; treating that as fatal would
  make every closure fail. Only spec->spec is fatal; learned->spec is legitimate history.
- **The gate runs ONLY when a `plan/` file changes** (via `verify_wiring_docs.py` `is_plan_source`
  in the verify router), so a clean checkout / a code-only change never runs it -- it cannot red
  an unrelated commit.
- **Delta ratchet: signed-int lines summed** (`parse_sleep_baseline`), backward-compatible with a
  plain `125`; `.gitattributes merge=union` on BOTH `test/.ci-sleep-baseline` AND
  `plan/.citation-baseline` so two independent appends auto-union instead of conflicting; a NET
  RISE still fails (monotonic guarantee kept).
- **`spec_status` logic extracted to a testable `scripts/status/specbucket` package** because
  `spec_status.go` is `//go:build ignore` (not unit-testable in place); Category + SkeletonStale
  (6-week TTL, `ageDays > TTL` so not-stale AT the TTL) live there with boundary tests.

## Consequences

- **CLOSURE FOOTGUN (intended, not a bug):** closing spec X while spec Y still cites X makes
  Y->X dangle; that closure commit's verify FAILs with an actionable message unless Y's citation
  is fixed or Y->X is appended to `plan/.citation-baseline` (`--write-baseline`). This converts
  silent citation rot into a loud failure at the moment of closure. Closing an UNCITED spec (the
  common case) passes cleanly.
- Two other baseline readers had to migrate off bare-int parsing: `testing_health.py` (kept its
  fail-closed guard by mapping unparseable -> CollectError) and `verify_wiring_docs_test.go`.
- `ze-spec-status` now shows committed backlog vs idea capture as distinct buckets, flags
  skeletons past the TTL, and surfaces `spec-closure-check.py --list` as a non-blocking advisory.

## Gotchas

- The AC-2 WARN heuristic is the token IMMEDIATELY ADJACENT to each citation (down from "every
  backtick token on the line"), giving ~255 current-tree WARNs (many nearest-token false
  positives); WARN-forever + swallowed in router runs (shown only on a direct
  `make ze-spec-citation-check`). Do NOT FAIL-harden it without first cleaning the WARN set.
- Editing a shared baseline's REPRESENTATION (absolute -> delta) silently breaks every consumer
  that parsed the old format. Grep for readers before changing a baseline's shape.
- Dangling detection is keyed by TARGET only, so a NEW spec citing an already-baselined-gone
  target passes silently (minor false-negative surface) -- acceptable for a ratchet-down gate.

## Files

- scripts/dev/spec-citation-check.py, spec_citation_check_test.py, plan/.citation-baseline (AC-1/AC-2 gate + baseline)
- scripts/dev/verify_wiring_docs.py, verify_wiring_docs_test.py/.go, test/.ci-sleep-baseline, .gitattributes (AC-4 delta ratchet + union both baselines)
- scripts/status/spec_status.go, specbucket/specbucket.go(+test), spec_status_test.go (AC-3 bucket split + TTL)
- scripts/dev/testing_health.py (consumer migration), mk/inventory.mk + ai/INDEX.md (registration + AC-5 advisory)
