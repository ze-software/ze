# Handover: RFC "Linux cannot do this" requirement state

| Status | Next action |
|--------|-------------|
| Closed through the native two-commit script, with one independent CLEAN Review Gate | Parent runs `./le verify changed` over the committed change. No full verification or push is authorized |

## Goal

Thomas asked (2026-10-28): "Add a 'Linux can't do this' for requirements".
At design time the RFC ledger could not distinguish a requirement Linux cannot
meet from work Ze still owes. The completed change adds that
state, checked register evidence and per-instance owner approval, plus computed
per-RFC and per-profile attainable ceilings without increasing proof figures.

## Where it stands

Branch: `main`.

- Closed spec: `spec-rfc-linux-limit-annotation`. The implementation and closure
  record are retained in commit A before commit B removes the spec.
  Thomas approved the design in this session:
  D-1(a), D-2(A), D-3(B), D-4(A), D-5(A), **D-6(B)** and D-7 neutral.
  Do not ask those decisions again. The approval covers the design, not any
  requirement instance.
- Implementation code, tests and accompanying docs are carried by commit A.
  Independent core and site reviews drove fixes for summary/ancestor symlink
  approval bypasses, Supported-with-Remaining admission, missing register IDs
  and draft tie order. The independent closure Review Gate is CLEAN, one round,
  with no outstanding BLOCKER or ISSUE.
- The corrected guards, in-memory corpus invariant and affected publication
  regressions passed. The real-corpus test passed alone under its unchanged
  default timeout. A final direct-snapshot fixture needed the `tmp/` setup
  production `Create` performs; that repair preserved its assertions and passed.
- Actual CLI evidence and Supported-claim refusals passed. Browser checks
  covered filters, profiles, neutral Linux-limit standing, register IDs, empty
  populations and draft ties. Final canonical `site build` and `site check`
  passed, and its actual index was visually inspected.
- The earlier shared BGP compilation blocker was repaired by its owner.
  No foreign source was changed or included in closure.
- Nothing in `rfc/short/` is annotated. The closed spec's historical candidates
  list records rows that cite `docs/architecture/linux-limitations.md`. None
  qualifies today (each cited entry is `constrains design`, a library entry, or
  unverified).

## Approved decisions

| # | Decision | Approved choice |
|---|----------|----------------|
| D-1 | Syntax and evidence | `{linux-limit: LL-<ID>; <why no Ze code substitutes>}`; the cited `LL-` entry in `docs/architecture/linux-limitations.md` must be `Kind: kernel`, `Effect: blocks`, `Status: verified`, name the requirement id in `Blocks`, and carry Evidence |
| D-2 | Per-instance owner approval | `./le commit create` writes `Linux-limit-owed: <RID>: <LL-id>`; the commit lands, the push waits for Thomas's `Owner-attested:` |
| D-3 | What the ceiling subtracts | every row kind the figure can never prove: linux-limit, lower-layer, feature-declined, not-applicable, each as its own share |
| D-4 | Status row | counted as unmet: a `Supported` status is refused, Remaining counts it |
| D-5 | Register parser home | `internal/le/linuxlimit` |
| D-6 | Per-kind switches | one shared annotation-traits registry; existing readers migrate without changing their prior behavior |
| D-7 | Card tone | neutral, with its own note |

## Context the spec does not carry

- The public legend sentence `rfcIndexCeiling` (`internal/le/site/rfcfilter.go`)
  used to claim "every RFC here can reach 100%". That was false: `figuresByStem`
  (`internal/le/rfc/profilecoverage.go`) keeps `{not-applicable}`,
  `{lower-layer}` and `{feature-declined}` rows in the denominator unproven
  (RFC 4302 has sixteen lower-layer rows). It was corrected in `b4a36b70be`
  before publication. D-3 replaces that fixed sentence with a computed
  ceiling. The fixed declaration is removed. The similarly named
  `rfcIndexCeilingRows` is only the measured-row selector.
- The RFC compliance index was rewritten for readers on 2026-10-28
  (`d09a3cc477`, `2b02bd4125`): a glossary in `internal/le/site/rfcreader.go`
  and plain check-group lines from `internal/le/rfc/checkgroups.go`. The new
  kind has a glossary entry and its evidence check joins Requirement
  annotations. No wording-only check-group test was retained.
- The per-RFC bars and the profile-link overview (`rfcStemRows`,
  `rfcFigureRowCells`, `rfcFilterScript` in `rfcfilter.go`) sum `data-mp`,
  `data-mt`, `data-op`, `data-ot` per row exactly as the profile rollup does;
  `data-ma` and `data-oa` now carry attainable counts.
- Existing site shard-write races are recorded in
  `plan/journal/check-mode-mutates-the-tree.md`. Do not confuse a shared-tree
  race with a failed annotation assertion.

## Evidence and ownership

- Session: `644a4e78-0122-4eeb-8642-eb50638a1641`.
- State: `tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/state/session-state-rfc-linux-limit-annotation-644a4e78-0122-4eeb-8642-eb50638a1641.md`.
- Scratch: the same session's `scratch/`. `linux-limit-closure-files.txt`
  names commit A's whole-file population. Its `linux-limit-index.patch`
  selects only the RFC discovery row from shared `ai/INDEX.md`.
- The closed spec's implementation and closure tables identify the producing
  functions and concrete runtime evidence for all 19 acceptance criteria.
- Independent test approvals are in `tmp/commit-weakening-approved-0cf1e0e8.md`.
  They preserve all original parser cases and corpus inputs. The final parser
  extension adds a malformed Checked pin and requires zero usable entries on
  every invalid form; its named regression passed.
- No actual corpus annotation, owner attestation, push or deployment was made.
  `../wiki` is unchanged: this is internal tooling, not a new NOS capability.
- Own preview servers and browser tabs were stopped. The disposable Go
  emitter, six generated smoke-fixture directories and two emitter recipes
  were removed after the completed proof. Retain logs, browser JSON and
  `linux-limit-final-gap.webp` / `linux-limit-final-canonical.webp` as evidence.
  The canonical `../gh-pages` output remains a deliverable, not scratch.
- The foreign terminal-demo index row and health fixture edit were excluded.
  Four parser-move structural audit reports received independent native
  approvals after checking the preserved original assertions.

## Verified evidence

All log names below are relative to the session's scratch directory.

| Surface | Evidence |
|---------|----------|
| Initial commit/parser and appliance pin | `job-linux-limit-core-final-8ab2b653.log`, `job-linux-limit-appliance-final-c68ac1b6.log`; the initial RFC failure/timeout is superseded by the named follow-up results below |
| Remaining RFC tests and preserved annotation behavior | `job-linux-limit-rfc-remaining-b4de86f2.log`; its new-kind corpus assumption was repaired and passed in the final corrections run |
| Final RFC, approval and site corrections | `job-linux-limit-final-corrections-19f68aed.log`; the one fixture-setup failure then passed in `job-linux-limit-final-smoke-cfc3f8b2.log` |
| Real corpus tree/priority path | `job-linux-limit-corpus-only-0baead0f.log` |
| Parser malformed-pin boundary | `job-linux-limit-parser-boundary-a4406c82.log`; together with the prior parser profile, all 87 statements were exercised |
| Site/health and draft ordering | `job-linux-limit-site-be12bc74.log`, `job-linux-limit-site-corrections-2cc0e25c.log` |
| Real CLI | `linux-limit-cli-valid.log`, `linux-limit-cli-empty-evidence.log`, `linux-limit-cli-stale-checked.log`, `linux-limit-cli-supported.log` |
| Actual browser surfaces | `linux-limit-browser-evidence.json` and the two retained final screenshots |
| Canonical publication | `./le --name linux-limit site build` then `site check` passed: 23 producers and 1,091 published/written paths; source digest recorded in the spec and browser evidence |

Two legacy migration tests skipped themselves because their generated files
already exist at HEAD. The site build reported existing stale proof warnings
for seven unrelated features and two unstamped talks; none was refreshed to
manufacture a green result. No full verification or performance run was made.

## Closure and remaining verification

The machine review artifact is
`tmp/review/rfc-linux-limit-annotation-644a4e78-0122-4eeb-8642-eb50638a1641.md`.
LinuxLimitClosure independently checked logic/wiring, security/edge cases and
publication/proof preservation. No product edit was needed during closure.
The spec records the scoped review-check result and the closure prose correction.

Global repository and documentation compositions were not run under the standing
host ban. Scoped citation anchors passed. The existing actual CLI, browser,
targeted tests and canonical site evidence were inspected, not repeated.
The parent owes `./le verify changed` once over the committed change.
The full release check remains Thomas's, not an agent task.

No requirement instance was approved or annotated. No owner attestation,
push or deployment was made. Pushing either repository still requires Thomas's
explicit instruction.
