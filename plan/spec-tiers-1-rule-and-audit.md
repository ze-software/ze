# Spec: tiers-1-rule-and-audit

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-tiers-0-umbrella.md |
| Phase | 1/3 |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-tiers-0-umbrella.md` - parent umbrella (taxonomy, Path C decision, Phase 5 Hardening Analysis)
3. `.claude/rules/planning.md` - workflow rules
4. `ai/rules/rule-placement.md` + `ai/rules/canonical-sources.md` - where the rule doc must live, never edit generated files
5. `scripts/dev/dep_audit.py` - the tool being extended into the gate
6. `scripts/codegen/plugin_imports.go` - source of `pluginDirs` (the gate parses this)

## Task

Phase 0/1 of the tiers umbrella: make the module-tier boundary documented and
machine-auditable, with NO directory moves. Specifically:

1. Document the three-tier taxonomy (core / component / plugin, by dependency
   direction) as a canonical rule in `ai/rules/module-tiers.md`, so new code lands
   in the correct tier.
2. Extend `scripts/dev/dep_audit.py` with a Path C `--check` mode that enforces ONLY
   the engine-placement rule: a config-driven engine (`sdk.NewWithConn`) at a
   top-level subsystem must live in `internal/component/` if a feature depends on it,
   else in `internal/plugins/`. Nested sub-plugin namespaces (from the generator's
   `pluginDirs`) are excluded. Non-engine core/composition classification is printed
   as ADVISORY only, never enforced. No permanent allowlist.
3. Hold the 8 currently-misplaced engines in a transitional migration baseline
   (`scripts/dev/tier_migration_baseline.txt`) so `make ze-verify` stays green now;
   the gate FAILS on any NEW misplaced engine and on STALE baseline entries.
4. Wire the gate into `make ze-verify` and add a Go test that exercises it.
5. Link the rule from `ai/rules/INDEX.md`, the `CLAUDE.md` Before-You table, and
   `ai/INDEX.md` (discovery-updates).

**Goal:** A documented rule + a green, blocking, allowlist-free engine-placement gate
that prevents new engines landing in the wrong tier, with the known 8 recorded as a
shrinking migration baseline.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-tiers-0-umbrella.md` - taxonomy, Path C, blockers B-1/B-2/B-3
  -> Decision: enforce only the engine rule (Path C); core/composition advisory; no allowlist
  -> Constraint: the gate's plugin/engine scoping MUST reuse the generator's `pluginDirs`, not a bare grep (blocker B-1)
- [ ] `ai/rules/plugin-self-containment.md` - the delete-the-folder invariant this generalizes
  -> Constraint: removal test = a tier is honest only if placement matches dependency direction
- [ ] `ai/rules/rule-placement.md` (global) - shared rules go in `ai/rules/`
  -> Constraint: rule doc belongs in `ai/rules/module-tiers.md`, linked from INDEX + CLAUDE, NOT in ~/.claude
- [ ] `ai/rules/discovery-updates.md` - adding a gate means registering it for discovery
  -> Constraint: add keyword/path in `ai/INDEX.md` and document the verification target

**Key insights:**
- The component/plugin split is dependency direction, made auditable. Path C enforces only the crisp, mechanical engine subset.
- The generator `plugin_imports.go` is the authority on what is a plugin and where nested plugins may live; the gate parses its `pluginDirs`.

## Current Behavior (MANDATORY)

**Source files read:**
- `scripts/dev/dep_audit.py` - reverse-dependency reporter; classifies dirs by external importers. No `--check`/gate today.
  -> Constraint: extend, do not rewrite; keep the existing report modes (`--json`, `--candidates-only`).
- `scripts/codegen/plugin_imports.go` (lines 122-133) - `var pluginDirs []string` lists scan dirs incl. nested `bgp/plugins`, `firewall/plugins`.
  -> Constraint: nested namespaces = `pluginDirs` entries under `internal/component/` with >=4 path segments. Engine pkgs inside them are sub-plugins, excluded from the tier check.
- `Makefile` - `_ze-verify-impl: ze-lint ze-verify-wiring-docs ze-vet-evidence ze-unit-test-cached ze-unit-test-race-changed ze-functional-test ze-exabgp-test`.
  -> Constraint: add the gate as a prerequisite of `_ze-verify-impl` (or a target it calls) so it runs in the pre-commit gate.
- Engine-package audit (this session): 8 top-level subsystems are misplaced engines.
  -> Constraint: baseline = isis, ldp, rsvpte, flowexport, mrt (component->plugins) + bfd, sysctl, sysrib (plugins->component). `mpls` is NOT an engine (no `sdk.NewWithConn`) -> advisory only, NOT in the engine baseline. `plugins/iface` is a grouping dir (engine is nested `iface/dhcp`); confirm it is excluded as nested or recorded with disposition TBD.

**Behavior to preserve:**
- `dep_audit.py` existing report output and flags.
- `make ze-verify` stays green on the current tree (baseline absorbs the 8).
- No directory moves; no change to any plugin's runtime behavior.

**Behavior to change:**
- Add `--check` (gate) and `--write-baseline` to `dep_audit.py`; add the rule doc; wire the gate; add pointers and the test.

## Data Flow (MANDATORY)

### Entry Point
`make ze-verify` -> `python3 scripts/dev/dep_audit.py --check`; and `go test ./scripts/dev/` -> `TestEnginePlacement`.

### Transformation Path
1. Parse `pluginDirs` from `scripts/codegen/plugin_imports.go`.
2. Walk `internal/component/*` and `internal/plugins/*` top-level dirs; collect import edges.
3. Determine `is_engine(X)` = `sdk.NewWithConn` in X outside any nested namespace.
4. Determine `depended(X)` = a feature (component/plugins file, excluding own subtree, cmd/ze, core, chaos, test, all.go, dispatch, tests) imports X.
5. `expected = component if depended else plugins`; misplaced if `area(X) != expected`.
6. Compare misplaced set to the baseline; classify new vs stale; exit 0/2.
7. Print core/composition advisory lines (informational only).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| gate <-> generator | parse `pluginDirs` literal | [ ] |
| gate <-> ze-verify | Makefile prerequisite | [ ] |
| gate <-> baseline | read/compare `tier_migration_baseline.txt` | [ ] |

### Integration Points
- Reuses `dep_audit.py` graph walk; reads the generator's `pluginDirs`.

### Architectural Verification
- [ ] No bypassed layers (gate reads the same import graph as the report)
- [ ] No unintended coupling (gate only reads source, mutates nothing)
- [ ] No duplicated functionality (one graph walker; pluginDirs parsed, not re-listed)
- [ ] Zero-copy preserved (N/A)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Path C engine set is exactly the 8 listed | engine-package audit this session | baseline wrong; gate red or misses a case | run the new `--check` on current tree; compare | unvalidated |
| A-2 | Nested namespaces are `pluginDirs` entries with >=4 segments under component | `pluginDirs` literal inspected | bgp sub-plugins falsely flagged | run `--check`; assert no `bgp/plugins/*` in output | unvalidated |
| A-3 | `make ze-verify` can take a new prerequisite cheaply | Makefile `_ze-verify-impl` chain | verify slowed or breaks | time the gate (<1s expected); run `make ze-verify` | unvalidated |
| A-4 | Parsing `pluginDirs` from the Go literal is stable | simple slice of string literals | parser breaks if format changes | `plugin_imports_test.go`-style guard; the gate errors loudly if zero parsed | unvalidated |
| A-5 | `plugins/iface` is excluded as nested OR recorded with disposition | grouping dir, engine in `iface/dhcp` | spurious baseline entry | run `--check`; inspect whether iface appears | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Baseline rots (entries never removed) | stale-entry check fires | gate FAILS on stale entries, forcing removal as moves land |
| R-2 | `pluginDirs` parser silently returns empty -> everything flagged | huge mismatch count | gate errors if zero pluginDirs parsed (fail-loud) |
| R-3 | New engine added during another session lands mid-baseline | gate fails their verify | rule doc + INDEX pointer tells them the correct tier; baseline only holds the known 8 |
| R-4 | Concurrent edits to Makefile/CLAUDE.md by other sessions | merge conflict | edits are additive, localized; user commits |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-verify` | -> | `dep_audit.py --check` runs and gates | `TestEnginePlacement` (asserts `--check` exit 0 on real tree) |
| `python3 dep_audit.py --check` with injected misplaced engine fixture | -> | gate detects new violation | `TestEnginePlacementDetectsNew` (exit 2) |
| baseline entry that is no longer misplaced | -> | stale detection | `TestEnginePlacementStaleBaseline` (exit 2) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ai/rules/module-tiers.md` exists | Defines core/component/plugin by dependency direction, the engine rule, the baseline mechanism, and the authoring guidance; linked from `ai/rules/INDEX.md` and `CLAUDE.md` |
| AC-2 | `dep_audit.py --check` on current tree | Exit 0 (8 known engines are baselined); prints them as "baselined (pending migration)"; prints core/composition advisory |
| AC-3 | `dep_audit.py --check` with a misplaced engine NOT in baseline | Exit 2; names the dir, its expected tier, and points to `ai/rules/module-tiers.md` |
| AC-4 | `dep_audit.py --check` with a baseline entry that is no longer misplaced | Exit 2; reports the stale entry to remove |
| AC-5 | No permanent allowlist | The only exception list is the migration baseline; each entry is annotated with its resolving child spec; an empty baseline = full enforcement |
| AC-6 | `make ze-verify` | Runs the gate; passes on current tree |
| AC-7 | `pluginDirs` parse yields the nested namespaces | `bgp/plugins/*`, `firewall/plugins/*` engine sub-plugins are NEVER flagged |
| AC-8 | `pluginDirs` parser returns empty (corruption) | Gate errors loudly (non-zero), does not silently pass or flag everything |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | adds a new edge engine under `internal/component/foo` | `ze-verify` -> gate -> not in baseline -> fail with "move to internal/plugins" | `TestEnginePlacementDetectsNew` |
| 2 | runs `ze-verify` on the current repo | gate -> 8 baselined -> exit 0 | `TestEnginePlacement` |
| 3 | reads the rule before creating a package | `ai/rules/module-tiers.md` via INDEX/CLAUDE | doc-link check (`make ze-rules-index` / INDEX presence) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEnginePlacement` | `scripts/dev/dep_audit_gate_test.go` | `--check` exits 0 on the real tree (baseline matches reality) | |
| `TestEnginePlacementDetectsNew` | same | injected misplaced engine fixture -> exit 2 | |
| `TestEnginePlacementStaleBaseline` | same | baseline entry not currently misplaced -> exit 2 | |
| `TestPluginDirsParsed` | same | parser returns the nested namespaces from plugin_imports.go (non-empty; includes bgp/plugins) | |

### Boundary Tests
N/A (no numeric inputs).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| gate-in-verify | `make ze-verify` run | the gate actually runs in the pre-commit gate and passes | |

### Interop Tests
N/A - no protocol/wire behavior changes.

### Future
- None. Engine-placement enforcement extension (core/composition) is Path B, tracked in the umbrella, not here.

## Files to Modify
- `scripts/dev/dep_audit.py` - add `--check`, `--write-baseline`, `pluginDirs` parse, engine classification
- `Makefile` - add `ze-tier-check` target; add it to `_ze-verify-impl`
- `ai/rules/INDEX.md` - one-line pointer to `module-tiers.md`
- `CLAUDE.md` - Before-You table row ("Add a new component/plugin/package")
- `ai/INDEX.md` - discovery entry (keyword -> rule + gate)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Rule doc | [ ] yes | `ai/rules/module-tiers.md` |
| Verification gate | [ ] yes | `Makefile` `ze-tier-check` -> `_ze-verify-impl` |
| Discovery-updates | [ ] yes | `ai/INDEX.md`, `ai/rules/INDEX.md`, `CLAUDE.md` |
| Doctor check | [ ] no | gate is a dev/CI check, not a runtime dependency |
| Prometheus counters | [ ] no | no runtime state |
| YANG validation | [ ] no | no config leaves |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/core-design.md` or `docs/plugin-overview.md` - note the tier rule + gate |
| 15 | Runtime inventory changed? | [ ] no | no plugins moved this phase (grep docs for moved paths: none) |
| others | - | [ ] no | grep `docs/` for `dep_audit`/tier references: none expected |

## Files to Create
- `ai/rules/module-tiers.md` - the canonical rule
- `scripts/dev/tier_migration_baseline.txt` - the 8 baselined engines, each annotated with resolving child spec
- `scripts/dev/dep_audit_gate_test.go` - the gate tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 3. Audit | Current Behavior; confirm 8-engine set with the new `--check` |
| 4. Wiring | Wiring Test - add `--check` skeleton + failing Go test first |
| 5. Implement | Implementation Phases below |
| 6. Verify | `make ze-lint && make ze-unit-test` + `make ze-tier-check` |
| 7-10. Review | Critical Review Checklist |
| 11. Deliverables | Deliverables Checklist |
| 12. Security | Security Review Checklist |
| 14. Docs | Documentation Update Checklist |
| 15. Close | learned summary + two-commit script |

### Implementation Phases
1. **Phase: Wiring (FIRST)** - add `--check` to `dep_audit.py` returning a stub non-zero + write `TestEnginePlacement` (fails because logic is a stub). Add `ze-tier-check` Makefile target.
2. **Phase: Classification** - implement `pluginDirs` parse + engine detection + `depended` + expected-tier; `--check` reports the real misplaced set.
3. **Phase: Baseline** - implement `--write-baseline` + baseline read/compare (new vs stale); generate `tier_migration_baseline.txt`; `--check` exits 0 on current tree.
4. **Phase: Rule doc + pointers** - write `ai/rules/module-tiers.md`; add INDEX/CLAUDE/ai-INDEX pointers.
5. **Phase: Wire into verify** - add `ze-tier-check` to `_ze-verify-impl`; confirm `make ze-verify` (or the targeted gate) passes.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | The 8-engine set matches the audit; no `bgp/plugins/*` flagged; mpls not in engine baseline |
| Fail-loud | empty `pluginDirs` parse -> non-zero, not silent pass |
| No allowlist | only the migration baseline exists; staleness enforced |
| Reuse | gate reuses dep_audit graph walk + generator pluginDirs, no duplicate logic |
| Rule placement | rule in `ai/rules/`, not `~/.claude`; linked from INDEX + CLAUDE (`ai/rules/canonical-sources.md`) |
| Idempotence | `--check` mutates nothing; `--write-baseline` deterministic (sorted) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| rule doc | `ls ai/rules/module-tiers.md`; `grep module-tiers ai/rules/INDEX.md CLAUDE.md` |
| gate | `python3 scripts/dev/dep_audit.py --check; echo $?` -> 0 |
| baseline | `cat scripts/dev/tier_migration_baseline.txt` -> 8 entries |
| new-violation detection | run gate against fixture -> exit 2 (the Go test) |
| verify wiring | `grep ze-tier-check Makefile` and it is a prereq of `_ze-verify-impl` |
| tests | `go test ./scripts/dev/ -run EnginePlacement` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input handling | gate reads source files read-only; no exec of repo content; path handling stays within repo root |
| Resource use | single bounded tree walk; no unbounded recursion (skip vendor/.git/tmp) |
| Information leakage | gate output lists package paths only (no secrets) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| `--check` red on current tree | baseline content or classification logic; fix, re-run |
| `bgp/plugins/*` flagged | nested-namespace exclusion via pluginDirs; fix parse |
| ze-verify slow | move gate earlier/lighter; it should be <1s |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Path C gate = grep `sdk.NewWithConn` at top-level dirs | At engine-package granularity, nested sub-plugins (`bgp/plugins/*`, `firewall/plugins/irr`) and grouping dirs (`plugins/iface`, engine in `iface/dhcp`) produce ~33 false positives | implementation audit (engine-package probe) | gate built at engine-package granularity + nested-namespace exclusion via the generator's `pluginDirs`; clean set = exactly 8 |
| `mpls` is an edge engine to move | `mpls` has NO `sdk.NewWithConn` (forwarding helper) | engine probe | mpls is NOT in the engine baseline; Path C treats it as advisory, not enforced |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Top-level-dir granularity for the gate | flagged `plugins/iface` (grouping dir) as misplaced because a sibling backend is depended-on | engine-package granularity: classify the package that calls `sdk.NewWithConn`, depended-on computed on that package's own subtree |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Path C engine-only gate, no allowlist | full trichotomy (needs Path B preconditions) | only the engine subset is fully mechanical now |
| Migration baseline (shrinking, staleness-enforced) | hard-fail on current 8 (verify red) / permanent allowlist | keeps verify green now without a permanent exception list |
| Gate parses generator `pluginDirs` | grep heuristic; hardcoded nested list | authoritative; avoids 33 sub-plugin false positives (blocker B-1) |
| Gate in Python (extends dep_audit.py) | Go gate reusing generator | user directive; keeps the audit in one tool/language |

## Known Limitations
- Only engine placement is enforced; core/composition is advisory until Path B.
- The baseline holds known misplacements until child specs 2-4 move them.

## Implementation Summary

### What Was Implemented
- `ai/rules/module-tiers.md` — canonical 3-tier rule (core/component/plugin by dependency direction), engine rule, gate + baseline mechanism, authoring guidance. `**When:**` line for the rules index.
- `scripts/dev/dep_audit.py` — added `--check` (Path C engine gate), `--write-baseline`, `--selftest`; parses generator `pluginDirs`; engine-package granularity + nested-namespace exclusion. Existing report modes preserved.
- `scripts/dev/tier_migration_baseline.txt` — 8 baselined engines, each tagged with its resolving child spec (tiers-2/-3).
- `scripts/dev/dep_audit_gate_test.go` — `TestEnginePlacement` (`--check` exit 0 on real tree) + `TestEnginePlacementSelftest` (`--selftest` fixtures).
- `Makefile` — `ze-tier-check` target wired into `_ze-verify-impl` and `_ze-verify-changed-impl`.
- Pointers: `ai/rules/INDEX.md` (regenerated), `ai/INDEX.md`, `ai/INSTRUCTIONS.md` Before-You row (→ regenerated CLAUDE.md/AGENTS.md), `docs/plugin-overview.md` (with source anchors).

### Bugs Found/Fixed
- `write_baseline` did not create its parent dir — found by `--selftest`, fixed with `os.makedirs(exist_ok=True)`.

### Documentation Updates
- `docs/plugin-overview.md` "Module tiers" subsection added with source anchors. `check_doc_links.py --md-only`: my link valid; one pre-existing unrelated break at `plan/TEMPLATE.md:198` (not touched).

### Deviations from Plan
- Spec assumed top-level-dir granularity might suffice; implementation required engine-package granularity + generator `pluginDirs` exclusion (Mistake Log). Clean set is 8 as planned. `plugins/iface` correctly NOT flagged (engine is nested `iface/dhcp`).

## Goal Validation (BLOCKING)
| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| Rule documented | file + links | `ai/rules/module-tiers.md` exists; linked in `ai/rules/INDEX.md:58`, `ai/INDEX.md`, generated `CLAUDE.md:154` |
| Placement auditable by code | gate run | `dep_audit.py --check` exit 0 (8 baselined); exit 2 on injected fake engine (verified manually + `--selftest`) |
| No new misplaced engines possible | regression test | `--selftest` asserts empty-baseline → exit 2; `TestEnginePlacement` green |
| Green in pre-commit gate | make run | `make ze-tier-check` exit 0; `go test ./scripts/dev/` ok |

## Review Gate
### Run 1 (self-review against Critical Review Checklist)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `errorsastype` editor hint suggests `errors.AsType` | dep_audit_gate_test.go:28 | acknowledged; golangci-lint (project gate) passes with `errors.As`; kept portable form |
| 2 | (none) | Critical Review Checklist items all verified (correctness=8 set, no bgp/plugins flagged, fail-loud on empty pluginDirs, no allowlist, reuse, rule placement, idempotence) | — | — |

### Final status
- [ ] `/ze-review` not run (targeted dev-tooling change; self-review done while user is editing the tree). Recommended before final commit if desired.
- All findings NOTE-only.

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `ai/rules/module-tiers.md` | yes | `ls` 4.8K |
| `scripts/dev/tier_migration_baseline.txt` | yes | `ls` 860B, 8 non-comment lines |
| `scripts/dev/dep_audit_gate_test.go` | yes | `ls` 1.9K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | rule doc + links | `grep -l module-tiers ai/rules/INDEX.md ai/INDEX.md CLAUDE.md AGENTS.md` → all 4 |
| AC-2 | check exit 0, baselined | `dep_audit.py --check` exit 0, prints 8 baselined |
| AC-3 | new violation → exit 2 | injected `internal/component/faketier` → exit 2 |
| AC-4 | stale entry → exit 2 | appended `doesnotexist` → "stale baseline entry" exit 2 |
| AC-5 | no permanent allowlist | only baseline; staleness enforced; comment header states transitional |
| AC-6 | gate in ze-verify | `grep ze-tier-check Makefile` → prereq of `_ze-verify-impl` |
| AC-7 | nested excluded | no `bgp/plugins/*` in `--check` output; `--selftest` asserts nested excluded |
| AC-8 | empty pluginDirs → fail-loud | `engine_misplacements` raises SystemExit if parse empty |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed (8) | `--check` lists exactly the 8 (mpls excluded as non-engine, plugins/iface excluded) |
| A-2 | confirmed | nested namespaces from pluginDirs; no bgp sub-plugins flagged |
| A-3 | confirmed | `make ze-tier-check` runs in <2s |
| A-4 | confirmed | parser returns pluginDirs; gate raises if empty (fail-loud) |
| A-5 | confirmed | `plugins/iface` NOT flagged — engine is nested `iface/dhcp`, not depended |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| plugin-overview "Module tiers" | source anchors to module-tiers.md + dep_audit.py | yes; my link passes check_doc_links |

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 demonstrated
- [ ] Gate wired into `make ze-verify`
- [ ] Rule linked from INDEX + CLAUDE
- [ ] `make ze-lint && make ze-unit-test` pass
- [ ] Documentation Update Checklist answered with evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken
