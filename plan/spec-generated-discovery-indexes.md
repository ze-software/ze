# Spec: generated-discovery-indexes

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-07-05 |

> Session note (2026-07-05): implemented and verified from the gh-pages worktree.
> The main working tree currently holds another session's uncommitted work
> (as112-bgp-redistribute plus ~30 skeleton specs and a not-yet-committed
> `plan/learned/1066`). Therefore the learned counter was NOT bumped and no
> commit was prepared. Remaining closure (full `make ze-verify`, `/ze-review`,
> learned summary at the next free number, two-commit closure) must run in a
> clean main session on this spec's files only. My files are listed under
> Implementation Summary.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `AI-NAVIGATION-AUDIT.md` (repo root) - the audit that motivated this spec
4. `scripts/dev/code_to_docs.py`, `scripts/dev/rules_index.py` - the two existing generators this pattern copies
5. `scripts/inventory/inventory.go`, `scripts/dev/arch_map.py` - candidate hosts for the package map
6. `mk/inventory.mk` (targets `ze-doc-index`, `ze-doc-test`, lines ~72-105) - the gate wiring

## Task

Make the "what does what" and "which code implements this doc" information a **generated
build artifact** sourced from the code itself, so a local edit (a package doc comment, a
plugin `Description`, a `// Design:` header) regenerates the central index, and a stale
index fails `make ze-verify` instead of rotting silently.

Three generated indexes, one shared gate:

1. `ai/PACKAGE-MAP.md` — one row per package/plugin: path, one-line responsibility, key entrypoint. Sourced from `// Package` doc comments + plugin registry `Description`.
2. `ai/DOCS-TO-CODE.md` — the inverse of the ~5717 `// Design:` headers: given a design doc, the `.go` files that implement it. Sourced from the same `// Design:` edges `code_to_docs.py` already parses.
3. `ai/LEARNED-FULL-INDEX.md` — the complete list of all `plan/learned/NNN-*.md` (id, slug, first line), so the ~840 summaries absent from the curated `ai/LEARNED-INDEX.md` stop being an `ls`-and-guess lookup.

All three follow the existing `code_to_docs.py` / `rules_index.py` pattern: a generator with a `--check` mode wired into `ze-doc-test`. This spec adds tooling and documentation surfaces only. It changes no runtime, wire, or protocol behavior.

Motivation and evidence: `AI-NAVIGATION-AUDIT.md` (repo root).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `ai/rules/discovery-updates.md` — the rule this spec operationalizes (generated + gated discovery surfaces)
  → Constraint: a new discovery surface must be registered as a "Current Discovery Surface" and kept fresh by a gate, not hand-maintained.
- [ ] `ai/rules/design-doc-references.md` — defines the `// Design:` header this spec inverts
  → Constraint: exempt files (`*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`, `// Code generated`) carry no `// Design:` and must be excluded from DOCS-TO-CODE.
- [ ] `ai/rules/canonical-sources.md` — never edit generated files by hand
  → Constraint: each generated index needs a `<!-- GENERATED ... do not edit -->` banner and a regen command line.

### RFC Summaries (MUST for protocol work)
- N/A — this is build tooling, not a protocol feature.

**Key insights:**
- The pattern already exists twice (`code_to_docs.py`, `rules_index.py`), both gated in `ze-doc-test`. This spec is a third and fourth instance of a proven pattern, not a new mechanism.

## Current Behavior (MANDATORY)

**Source files read:** (from the AI-NAVIGATION-AUDIT sub-audits)
- [ ] `scripts/dev/arch_map.py` — generates the `arch-components` / `arch-system-plugins` / `arch-bgp-plugins` blocks in `ai/INSTRUCTIONS.md` and `CLAUDE.md` as comma-separated **directory-name lists only** (`block()` joins names, no descriptions).
  → Constraint: it walks the same trees a package map needs; extending it is one option.
- [ ] `scripts/inventory/inventory.go` — imports the plugin registry and renders a Description column (`inventory.go:426-437` reads `reg.Description`). Enumerates **plugins only**, not plain components or `internal/core/*`. Output is generated on demand, not committed.
- [ ] `internal/component/plugin/registry/registry.go` — `Description string` field (`registry.go:42`); 106 `register.go` files set a real one-liner (e.g. `internal/plugins/ospf/register.go:99`). Surfaced only at runtime (help text, TUI).
- [ ] `scripts/dev/code_to_docs.py` — parses `// Design:` and emits the **forward** `ai/CODE-TO-DOCS.md` (code → docs). Has a `--check` mode. No inverse index exists.
- [ ] `scripts/dev/rules_index.py` — generates `ai/rules/INDEX.md` with a `--check` gate. The closest precedent for a generated `ai/*.md` index.
- [ ] `mk/inventory.mk` — `ze-doc-index` runs the generators; `ze-doc-test` runs each with `--check` and sets `FAIL=1` on drift (lines ~72-105).
- [ ] `plan/learned/` — 1082 summaries; `ai/LEARNED-INDEX.md` curates ~237 (~22%); the rest are discoverable only by `ls` + slug guessing.

**Behavior to preserve:**
- `ai/CODE-TO-DOCS.md`, `ai/rules/INDEX.md`, and the `arch-*` generated blocks keep their current format and generators. This spec adds indexes; it does not reformat existing ones.
- The `ze-doc-index` / `ze-doc-test` contract: `ze-doc-index` writes, `ze-doc-test` checks and never writes.
- `// Design:` exemptions stay authoritative — the same exclusion list the enforcement hook uses.

**Behavior to change:**
- None to existing artifacts. Net-new generated files + gate rows only.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Build-time only. Inputs are source files in the working tree: `.go` files (doc comments, `// Design:` headers), `register.go` declarations (`Description`), and `plan/learned/NNN-*.md` filenames + first lines.

### Transformation Path
1. Enumerate packages under `internal/`, `pkg/`, `cmd/` (exclude `vendor/`, generated, `tmp/`).
2. For each package: extract first sentence of the `// Package` comment (AST/doc parse, no import). Where the package is a registered plugin/component, join the registry `Description`. Missing both → `TODO` row.
3. For DOCS-TO-CODE: reuse the `// Design:` edge set parsed by `code_to_docs.py`; invert to doc → [files], respecting the same exemptions.
4. For LEARNED-FULL-INDEX: list `plan/learned/NNN-*.md`, read the title/first line, group by number range.
5. Emit deterministic, sorted Markdown with a GENERATED banner + regen command.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | none — build tool, no runtime path | [ ] |
| Source tree ↔ generated index | read-only parse → Markdown write | [ ] |
| Registry import ↔ AST parse | registry `Description` needs a Go import; package-doc first line needs only AST (no import) — keep them separable to avoid import cycles | [ ] |

### Integration Points
- `mk/inventory.mk`: new recipes fold into `ze-doc-index` (write) and `ze-doc-test` (check). `make ze-verify` already calls `ze-doc-test`.
- `ai/rules/discovery-updates.md` "Current Discovery Surfaces" table: add the three new indexes.
- `ai/INDEX.md`: add a short "understand existing code, not change it" front-door pointing at the three indexes.

### Architectural Verification
- [ ] No bypassed layers (generators read source, write Markdown, nothing else)
- [ ] No unintended coupling (package-doc extraction does not import the packages it documents)
- [ ] No duplicated functionality (extend/mirror `code_to_docs.py` + `rules_index.py`; do not add a third tree-walker where `arch_map.py` or `inventory.go` already walks)
- [ ] Zero-copy preserved where applicable — N/A (build tool)
- [ ] Registration over hardcoding — the package map derives rows from registry metadata and doc comments; it hardcodes no plugin list.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Plugin `Description` is populated widely enough to be useful | 106 `register.go` set it (audit); `registry.go:42` | Package map is thin for plugins; fall back to package doc | `grep -rl "Description:" --include=register.go internal/` count | unvalidated |
| A-2 | `// Package` first sentence is a usable one-liner where present | 356/610 dirs carry it (audit); sampled real descriptions | Rows read poorly; may need manual curation column | sample 20 extracted rows for readability | unvalidated |
| A-3 | Packages can be enumerated + doc-parsed without importing them (no cycle risk) | Go `go/parser`+`go/doc` are AST-only | Must shell registry data separately from doc parse | prototype AST walk over `internal/core/` | unvalidated |
| A-4 | Registry `Description` is reachable only by importing registries (as `inventory.go` does) | `inventory.go:123-137` imports `registry.All()` | Package-map generator must be Go or must consume `ze-inventory-json` | read `inventory.go`; run `make ze-inventory-json` | unvalidated |
| A-5 | A `--check` mode composes into `ze-doc-test` exactly like the two existing generators | `mk/inventory.mk:83,86` show the `|| FAIL=1` pattern | Gate wiring differs; adjust recipe | mirror the existing recipe lines | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | ~254 undocumented packages produce noisy `TODO` rows | PACKAGE-MAP is mostly TODO on first generation | Frame TODO rows as a backfill worklist (feature); optionally sort TODO rows into a trailing section so the documented map reads cleanly |
| R-2 | Generated files churn diffs on every run | large unrelated diffs in `ze-doc-index` output | Deterministic sort + stable formatting; snapshot the output once and diff |
| R-3 | Import cycle if the package-map generator imports every package for docs | `go build` of the generator fails | Use AST/`go/doc` for doc comments (no import); import only the registry for `Description`, exactly as `inventory.go` already does |
| R-4 | Two hosts (Python vs Go) for one artifact split the maintenance | reviewers unsure which script owns what | Decide host in DESIGN (see Key Design Decisions); prefer one generator per artifact |
| R-5 | LEARNED-FULL-INDEX duplicates the curated LEARNED-INDEX and confuses which to read | agents open the wrong one | Header of the full index states "complete, uncurated; for topic-grouped curation see LEARNED-INDEX.md" and vice-versa |

## Wiring Test (MANDATORY — NOT deferrable)

The gate IS the wiring: the value proposition ("a local edit forces regeneration") is only real if `--check` fails on drift and `ze-doc-index` repairs it.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Edit a package doc / `Description` / `// Design:` then run `make ze-doc-test` | → | the new `--check` recipes | a test that mutates a source signal in a fixture and asserts `--check` exits non-zero |
| Run `make ze-doc-index` | → | the new generators | a test that asserts a second immediate `--check` exits zero (idempotent, deterministic) |
| `make ze-verify` | → | `ze-doc-test` including the new `--check` rows | assert the new checks run inside `ze-verify` (grep the recipe / run the target) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Run the package-map generator | `ai/PACKAGE-MAP.md` lists every non-vendor package/plugin under `internal/`, `pkg/`, `cmd/` with path + one-line responsibility (or explicit `TODO`) + key entrypoint where derivable |
| AC-2 | A plugin has a registry `Description` | its PACKAGE-MAP row shows that `Description` verbatim (source of truth = the code) |
| AC-3 | Run the docs-to-code generator | `ai/DOCS-TO-CODE.md` maps each design doc to the `.go` files whose `// Design:` header cites it, excluding the standard exempt files |
| AC-4 | Run the learned-index generator | `ai/LEARNED-FULL-INDEX.md` lists all `plan/learned/NNN-*.md` with id + slug + first line, grouped by number range |
| AC-5 | A source signal changes and the index is not regenerated | `make ze-doc-test` (hence `make ze-verify`) fails, naming the drifted file |
| AC-6 | `make ze-doc-index` is run then `--check` immediately | `--check` passes; output is byte-stable across repeated runs (deterministic) |
| AC-7 | A reader opens `ai/INDEX.md` cold | a short "understand existing code" section points at PACKAGE-MAP, DOCS-TO-CODE, LEARNED-FULL-INDEX |
| AC-8 | The three indexes exist | `ai/rules/discovery-updates.md` "Current Discovery Surfaces" lists them |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Agent asks "what does `internal/component/managed` do?" | opens `ai/PACKAGE-MAP.md`, one row answers it | AC-1 fixture row present |
| 2 | Agent told "subsystem X is governed by `docs/architecture/Y.md`, change the code" | opens `ai/DOCS-TO-CODE.md`, reads the file list under Y.md | AC-3 fixture doc→files present |
| 3 | Developer edits a plugin `Description`, forgets to regen, runs `make ze-verify` | `ze-doc-test` fails with the drift | AC-5 gate test |
| 4 | Agent needs a learned summary not in the curated index | opens `ai/LEARNED-FULL-INDEX.md`, finds it by slug | AC-4 completeness (count == `ls plan/learned/NNN-*.md`) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| package-map: doc first-sentence extraction | generator test (Python `_test.py` or Go `_test.go` per host) | `// Package Foo bars.` → "bars" one-liner | |
| package-map: registry Description join | generator test | plugin with `Description` renders it verbatim | |
| package-map: undocumented package | generator test | package with neither doc nor Description → `TODO` row | |
| docs-to-code: inversion + exemptions | generator test | `// Design: a.md` in `x.go` → row under a.md; `register.go` excluded | |
| learned-index: completeness | generator test | row count == number of `plan/learned/NNN-*.md` | |
| determinism | generator test | two runs produce byte-identical output | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — no numeric inputs | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `--check` fails on drift | test harness invoking the generator on a fixture tree | edit a source signal → `--check` non-zero | |
| `ze-doc-index` then `--check` passes | same | regen repairs drift | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A — not a protocol feature | - | - | - | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `mk/inventory.mk` — add generator recipes into `ze-doc-index`; add `--check` rows into `ze-doc-test`.
- `ai/INDEX.md` — add the "understand existing code" front-door section (AC-7).
- `ai/rules/discovery-updates.md` — list the three indexes under Current Discovery Surfaces (AC-8).
- One of `scripts/dev/arch_map.py` or `scripts/inventory/inventory.go` — extend to emit PACKAGE-MAP (host decided in DESIGN; see Key Design Decisions).
- `scripts/dev/code_to_docs.py` — add DOCS-TO-CODE emission (it already parses the edges), or a sibling `scripts/dev/docs_to_code.py`.

### BGP Family Checklist (if new SAFI / capability / attribute)
N/A — not a BGP protocol extension.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No | - |
| CLI commands/flags | [ ] No | - |
| Functional test for new RPC/API | [ ] No (build tool; covered by generator/gate tests) | - |
| Doctor check for runtime dependencies | [ ] No | - |
| Prometheus counters/metrics | [ ] No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No (agent/dev-facing) | - |
| 10 | Test infrastructure changed? | [ ] Yes | `docs/functional-tests.md` (new gate rows) if it documents `ze-doc-test` |
| 12 | Internal architecture changed? | [ ] No | - |
| 15 | Runtime inventory changed? | [ ] No | - |
| 16 | Changed source referenced by doc source anchors? | [ ] Check | grep `docs/` for anchors on `arch_map.py` / `code_to_docs.py` / `inventory.go` |

## Files to Create
- `ai/PACKAGE-MAP.md` — generated (banner + regen line).
- `ai/DOCS-TO-CODE.md` — generated.
- `ai/LEARNED-FULL-INDEX.md` — generated.
- Generator source file(s) as decided in DESIGN (extend existing where possible).
- Generator unit tests + the `--check` gate test.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, existing `code_to_docs.py`/`rules_index.py`/`inventory.go` |
| 3. Wiring phase | Wiring Test table — add `--check` recipes + a failing gate test |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7-14 | Critical review, deliverables, security, summary per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — add empty `--check` recipes + a gate test that fails
   - Tests: `--check` fails on a drift fixture
   - Files: `mk/inventory.mk`
   - Verify: `ze-doc-test` invokes the new checks and fails because generators are stubs
2. **Phase: DOCS-TO-CODE** — cheapest, 100% mechanical (invert existing edges)
   - Tests: inversion + exemptions, determinism
   - Files: `code_to_docs.py` (or sibling), `mk/inventory.mk`
   - Verify: fixture doc→files; `--check` green after `ze-doc-index`
3. **Phase: LEARNED-FULL-INDEX** — list + first-line + grouping
   - Tests: completeness (count match), determinism
   - Files: new generator, `mk/inventory.mk`
   - Verify: count == `ls plan/learned/NNN-*.md`
4. **Phase: PACKAGE-MAP** — doc-comment + registry `Description` join (host decided here)
   - Tests: doc extraction, Description join, TODO row, determinism
   - Files: `arch_map.py` or `inventory.go`, `mk/inventory.mk`
   - Verify: every package appears once; AC-1/AC-2
5. **Phase: Front door + discovery surface** — `ai/INDEX.md` section, `discovery-updates.md` row
   - Tests: AC-7/AC-8 presence checks
6. **Full verification** → `make ze-verify`
7. **Complete spec** → audit, learned summary `plan/learned/NNN-<name>.md`, two-commit closure per `ai/rules/planning.md`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has file:line implementation |
| Correctness | `--check` truly fails on drift and passes after regen (not a no-op green) |
| Determinism | Output byte-stable; no timestamps, no map-iteration nondeterminism |
| No duplication | Extended an existing walker; did not add a third |
| No import cycle | Package-doc extraction imports nothing it documents |
| Canonical sources | Each generated file has GENERATED banner + regen command |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `ai/PACKAGE-MAP.md` | `ls`; row count vs package count |
| `ai/DOCS-TO-CODE.md` | `ls`; spot-check a doc's file list vs `grep "// Design: <doc>"` |
| `ai/LEARNED-FULL-INDEX.md` | row count == `ls plan/learned/[0-9]*.md \| wc -l` |
| Gate active | `grep -c -- "--check" mk/inventory.mk` increased; `make ze-doc-test` runs them |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input handling | Generators parse repo-local files only; no network, no exec of untrusted input |
| Path safety | Confine walks to repo tree; skip `vendor/`, `tmp/`, symlinks |

### Failure Routing
| Failure | Route To |
|---------|----------|
| `--check` green when it should be red | fix the check (it is a no-op); this is the mechanism-not-behavior trap |
| Generator import cycle | switch package-doc path to AST-only |
| Nondeterministic diff | add stable sort |
| 3 fix attempts fail | STOP, present approaches, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Follow the `code_to_docs.py` / `rules_index.py` generator + `--check` pattern | hand-maintained indexes; a PostToolUse auto-regen hook | The gate makes drift a red build (trustworthy); a hook is convenience and can be added later. Hand-maintained indexes are exactly the staleness this spec removes. |
| PACKAGE-MAP host: (decide in DESIGN) `inventory.go` vs `arch_map.py` | either | `inventory.go` already imports the registry for `Description` but enumerates plugins only; `arch_map.py` already walks all trees but has no registry access. Leaning: `inventory.go` extended to walk all packages for doc comments (Go `go/doc`) and keep its registry join, since it already owns `Description`. Confirm at the design gate. |
| TODO rows for undocumented packages | silently omit them | Omitting hides the 42% doc gap; a TODO row turns the map into a backfill worklist. |
| Keep LEARNED-FULL-INDEX separate from the curated LEARNED-INDEX | replace the curated one | Curation and completeness serve different needs; cross-link both. |

## Known Limitations
- PACKAGE-MAP responsibility text is only as good as the `// Package` comment / `Description`. The 264 packages that showed TODO were backfilled with `// Package` docs (one `doc.go` each), so the map now has zero TODO rows. Per-subsystem flow digests were also added under `ai/digests/`.
- DOCS-TO-CODE is only as complete as `// Design:` coverage (already hook-enforced, so high).
- The optional PostToolUse auto-regen hook is out of scope; the `--check` gate is the mechanism this spec commits to.
- ADR revival/removal (`docs/architecture/decisions/`, one entry) is a separate process decision, out of scope here.

## RFC Documentation
N/A — no protocol behavior.

## Implementation Summary
### What Was Implemented

Three generators (Python, mirroring `code_to_docs.py` / `rules_index.py`), each with
a `--check` mode, plus their outputs, tests, gate wiring, and discovery surfaces.

My files (this spec only):

- `scripts/dev/package_map.py` + `scripts/dev/package_map_test.py` -> `ai/PACKAGE-MAP.md` (593 packages, 329 described, 264 TODO)
- `scripts/dev/docs_to_code.py` + `scripts/dev/docs_to_code_test.py` -> `ai/DOCS-TO-CODE.md` (267 design docs, 2834 files; inverse of `// Design:`)
- `scripts/dev/learned_index.py` + `scripts/dev/learned_index_test.py` -> `ai/LEARNED-FULL-INDEX.md` (1078 summaries, by number range)
- `mk/inventory.mk` -> new `ze-discovery-index` / `ze-discovery-index-check`; the three `--check`s folded into `ze-doc-test`
- `Makefile` -> `ze-regen` runs the writers; `ze-regen-check` diffs + `--check`s the three outputs
- `scripts/dev/verify_wiring_docs.py` -> `is_discovery_source()` selects `ze-discovery-index-check` on the right edits (register.go, `// Package`/`// Design:` .go headers, `plan/learned/*.md`, the generators/outputs, Makefile/mk). This is the load-bearing wire: without it the gate never runs in `ze-verify`.
- `ai/INDEX.md` -> "Understand Existing Code (not change it)" front-door section
- `ai/rules/discovery-updates.md` -> three rows in Current Discovery Surfaces
- `AI-NAVIGATION-AUDIT.md` (repo root) -> the motivating audit

Commit gate (added on request, because the project has no CI, so the commit
script is the only enforcement point):

- `scripts/dev/commit_helper.py` -> a discovery-index gate in `create()`, alongside the existing verify-status gate. It blocks a commit when (a) a generated index is confirmed stale versus the tree (`make ze-regen` fix), or (b) the commit changes an index-feeding source (register.go, a `.go` with a `// Package`/`// Design:` header, a `plan/learned/*.md`, the generators, Makefile/mk) but omits the regenerated index. Overridable with `--stale-index-ok "<reason>"`; never blocks on missing tooling (mirrors the verify-status gate).
- `scripts/dev/commit_helper_test.py` -> unit tests for `feeds_discovery_index` and `index_pending`.
- `ai/rules/git-safety.md` -> documents the new gate in Commit Rules item 3.
- Verified end-to-end (dry-run): fresh + non-feeding = pass; stale = block ("run make ze-regen"); feeding source with omitted index = block ("Add them: --file ..."); `--stale-index-ok` = pass.

### Bugs Found/Fixed
- `package_map.head()` first used `next(fh)` in a list comprehension; PEP 479 turns an exhausted-file `StopIteration` into `RuntimeError`, which the handler would not catch. Rewrote as a bounded `for` loop before first run.

### Documentation Updates
- `ai/INDEX.md`, `ai/rules/discovery-updates.md` (see above). No `docs/` change: `ze-doc-test` documents doc drift, and these are agent-facing `ai/` surfaces, not user docs.

### Deviations from Plan
- PACKAGE-MAP host: chose a new standalone `scripts/dev/package_map.py` over extending `inventory.go`, so the check is pure-text and build-independent (a doc gate must not fail because the Go build is temporarily broken). It parses register.go `Description:` directly rather than importing the registry (assumption A-4 resolved: registry import avoided).
- Added a pure-`embed.go` package skip so the TODO column is a real doc-coverage worklist, not YANG-schema noise (dropped ~130 empty rows).
- Closure (learned summary, counter bump, commit) deferred to a clean main session (see Session note).

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| "what does what" answerable from one artifact | generated output | `ai/PACKAGE-MAP.md`: 593 packages, one line each (329 described, 264 TODO) |
| local edit forces central regen | end-to-end gate test | Added a throwaway `internal/zztestdiscovery/register.go`: `package_map.py --check` exited RED ("stale"); removed it: GREEN. `make ze-discovery-index-check` passes all three |
| doc -> implementing code answerable | generated output | `ai/DOCS-TO-CODE.md`: 267 docs -> 2834 files; e.g. `behavior/fsm.md` -> fsm.go/state.go/timer.go |
| gate runs in real verify path | selection test | `verify_wiring_docs.selected_targets(...)` returns `ze-discovery-index-check` for a register.go / plan-learned / index edit |
| generators are deterministic + lint-clean | unit tests + ruff | 9 unittests pass (incl. determinism per generator); `ruff check` clean on all 6 files |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
### Fixes applied
### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — the `--check` gate proven to fail on drift and pass after regen
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify` passes (includes `ze-doc-test`)
- [ ] Generators integrated into `ze-doc-index` + `ze-doc-test`
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3 concrete artifacts justify the shared pattern)
- [ ] No speculative features (auto-regen hook explicitly deferred)
- [ ] Single responsibility per generator
- [ ] Explicit > implicit (generated banners, regen commands)
- [ ] Minimal coupling (no package imported to document it)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Determinism test present
- [ ] Gate test proves drift → failure
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** generators + gate + generated indexes + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-generated-discovery-indexes.md`
