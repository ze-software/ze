# Spec: Typed config decode and BGP diff consumption (DESIGN-REVIEW finding 3)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-unify-config-diff (map-diff dedup, adjacent; closed, learned 1079) |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. ~~`DESIGN-REVIEW.md` finding 3 ("The stringly-typed middle") and its verification notes~~ (2026-07-22: ephemeral session artifact, never committed; the finding is restated inline in this spec)
4. `internal/component/bgp/reactor/reactor_api.go`, `internal/component/bgp/reactor/config.go`, `internal/component/config/yang_schema.go`, `internal/component/config/tree.go`

## Task

Close the config-pipeline half of DESIGN-REVIEW finding 3: type information created at the
YANG edge is destroyed in the middle (coarsened schema, all-string `Tree`, `map[string]any`
of strings) and re-derived by hand at each consumer, and BGP (the largest consumer) throws
the resolved tree away and re-reads the whole file from disk.

Two verified defects in scope:

1. **Type coarsening then string round-trip.** YANG (fully typed) is coarsened into a Go
   schema where `uint64` collapses to `uint32` and every enum becomes a string
   (`internal/component/config/yang_schema.go`), then everything becomes strings in
   `Tree` (`tree.go`), then `map[string]any` still holding strings (`tree.go`,
   `ToMap`), and only `PeersFromTree` hand-parses back into typed fields via `mapUint32` /
   `mapString` / `netip.ParseAddr` (`config.go`). Each consumer re-derives the types
   the YANG schema already knew.

2. **BGP discards the computed diff and re-reads the file (the worst single instance).**
   In production `VerifyConfig` and `ApplyConfigDiff` ignore the resolved `bgpTree` they are
   handed and call `reloadFunc(configPath)`, re-reading the whole config file from disk
   through the full pipeline (`reactor_api.go`, `loadPeersFullOrTree`). The
   transaction machinery computes precise per-root deltas (`transaction/types.go`,
   `DiffSection` with string `Added`/`Removed`/`Changed`) that then serve only as a
   participation gate, not as the data BGP applies.

Goal: (1) make BGP parse peers from the resolved tree/diff it is handed, eliminating the
disk re-read on the production apply/verify path; (2) decode config values through a single
schema-driven typed path so `PeersFromTree`'s hand parsers and the `uint64` coarsening are
removed for the BGP subtree, establishing the pattern for other consumers.

**Explicitly out of scope (owned elsewhere, referenced not duplicated):**
- Command/response envelope and client `json.Valid` guessing: owned by
  `spec-unify-response-envelope.md` (finding 3 "Commands" bullet).
- BGP hot-path filter text serialization ordering: owned by `spec-unify-filters.md`;
  eliminating in-process-capable filter text is its declared follow-on.
- Redistribution text `update ...` command injection (`redistribute/consumer.go`,
  `formatAnnounce`): a sibling follow-on, tracked separately, not addressed here.
- The generic reload map-diff duplication (`plugin/server` vs `config.DiffMaps`): owned by
  `spec-unify-config-diff.md`.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/core-design.md` (config pipeline section) - how File -> Tree ->
  ResolveBGPTree -> map[string]any -> PeersFromTree flows
  → Decision: config is YANG-modeled; the canonical typed source is the YANG schema, not the Tree.
  → Constraint: plugin config delivery format (`{"bgp":{...}}` JSON) is a wire contract; do not change it here.
- [ ] `ai/rules/config.md` - YANG vs env var, typed leaf expectations
  → Constraint: every leaf should carry maximal native YANG typing; string is a fallback, not a default.
- [ ] `ai/rules/architecture.md` - required for the Data Flow section below
  → Constraint: no bypassed layers; the resolved tree must reach BGP through the transaction path, not a side disk read.

**Key insights:** the YANG schema already knows every leaf's type; the string coarsening and
per-consumer re-parse are redundant work, and the BGP disk re-read exists only because the
resolved tree handed to BGP was historically not fully resolved (templates/defaults).

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/config/yang_schema.go` - maps YANG kinds to a coarse Go type set;
  `Yuint32, Yuint64 -> TypeUint32` (line 888), `Yenum -> TypeString` (line 893).
- [ ] `internal/component/config/tree.go` - `Tree` stores `values map[string]string`,
  `multiValues map[string][]string` (lines 27-37); `ToMap` copies strings into
  `map[string]any` (lines 800-838).
- [ ] `internal/component/bgp/reactor/config.go` - `PeersFromTree(map[string]any)` hand-parses
  via `mapMap`/`mapUint32`/`mapString`/`netip.ParseAddr` (lines 472-537).
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `VerifyConfig`/`ApplyConfigDiff` call
  `loadPeersFullOrTree`, which uses `reloadFunc(configPath)` in production (re-reads disk) and
  the passed `bgpTree` only in the test fallback (lines 404-445).
- [ ] `internal/component/config/transaction/types.go` - `DiffSection` carries
  `Added`/`Removed`/`Changed` as strings (lines 158-166).

**Behavior to preserve:** (unless user explicitly said to change)
- Config file syntax and semantics unchanged; existing `.conf` files parse identically.
- Plugin config delivery format unchanged: plugins still receive the `{"bgp":{...}}` JSON
  subtree wrapper; the transaction verify/apply/rollback protocol is untouched.
- Parsed `PeerSettings` are byte-for-byte equivalent to today's full-pipeline result: no new
  false diffs in `peerSettingsEqual`, no dropped fields (capabilities, static routes, families).
- `ze config diff` output and `rpc.ConfigDiffSection` JSON unchanged.
- Test-mode fallback (`reloadFunc == nil`) still parses from the passed tree.

**Behavior to change:** (only if user explicitly requested)
- Production `VerifyConfig`/`ApplyConfigDiff` stop re-reading the file from disk; they parse
  from the resolved tree/diff they are handed.
- BGP config values are obtained via typed accessors backed by the YANG schema, not by
  ad-hoc string parsing; `uint64` leaves consumed by BGP are no longer silently truncated.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config change arrives via the CLI commit / config transaction: the resolved config Tree
  for the `bgp` root is produced by `ResolveBGPTree` and flattened via `Tree.ToMap()`.
- Format at entry today: `map[string]any` whose scalar leaves are Go strings (never typed).

### Transformation Path
1. YANG modules load into a coarse Go `Schema`; `yangKindToType` maps `Yuint64 -> TypeUint32`
   and `Yenum -> TypeString` (`yang_schema.go`).
2. Parsed config populates a `Tree` whose `values` are strings (`tree.go`).
3. `Tree.ToMap()` produces `map[string]any` still holding those strings (`tree.go`).
4. The transaction layer computes per-root `DiffSection`s with string bodies
   (`transaction/types.go`) and drives verify/apply.
5. BGP `VerifyConfig`/`ApplyConfigDiff` receive the resolved `bgpTree` but, in production,
   discard it and call `reloadFunc(configPath)` to re-read the file (`reactor_api.go`).
6. `PeersFromTree`/`parsePeerFromTree` hand-parse strings back into typed `PeerSettings`
   (`config.go`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema ↔ Go schema | type coarsening in `yangKindToType` (uint64→uint32, enum→string) | [ ] |
| Tree ↔ consumers | `ToMap` to `map[string]any` of strings; hand-parsed by `PeersFromTree` | [ ] |
| Transaction ↔ BGP | resolved tree passed but re-read from disk via `reloadFunc(configPath)` | [ ] |
| BGP ↔ plugins | `{"bgp":{...}}` JSON subtree (delivery format, preserved) | [ ] |

### Integration Points
- `ResolveBGPTree` and the transaction verify/apply coordinator - source of the resolved tree.
- `reactorAPIAdapter.loadPeersFullOrTree` - the seam where the disk re-read is chosen.
- `PeersFromTree` / `parsePeerFromTree` and the `mapUint32`/`mapString` helpers - the hand parsers.
- The YANG `Schema` type table - the single source of typed leaf information to key decode from.

### Architectural Verification
- [ ] No bypassed layers (BGP consumes the transaction-resolved tree, not a side disk read)
- [ ] No unintended coupling (typed decode keyed off the existing schema, no new global)
- [ ] No duplicated functionality (extends the schema/decode path, does not add a parallel one)
- [ ] Zero-copy preserved where applicable (typed subtree references, avoid re-stringify)
- [ ] Registration over hardcoding — typed decode is schema-driven; no per-leaf switch case
  or per-consumer factory is added to a core/shared package (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The resolved `bgpTree` handed to `ApplyConfigDiff` can be made as complete as the disk re-read (templates + defaults applied) | `reactor_api.go` comment says the re-read exists to avoid false diffs from incomplete settings | Diff consumption yields false diffs; must resolve the tree fully before parse | Compare `PeerSettings` parsed from tree vs from disk for a fixture config | unvalidated |
| A-2 | No BGP-consumed leaf that is semantically uint64 currently relies on the uint32 coarsening (e.g. AIGP metric) | `yang_schema.go` collapses uint64; AIGP is 64-bit | A real truncation bug exists and the fix changes behavior (must add boundary test + note) | grep YANG for uint64 leaves consumed by BGP; boundary test | unvalidated |
| A-3 | Plugin config delivery JSON can stay string-valued while internal BGP decode is typed | `transaction/types.go`, plugin protocol docs | Delivery format change leaks to plugins (contract break) | Diff emitted plugin JSON before/after; `.ci` byte compare | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Removing the disk re-read changes apply semantics under concurrent edits | reload `.ci` tests differ | Keep the resolved tree the single source; add a concurrent-edit `.ci` test |
| R-2 | Typed decode scope creeps into every consumer | diff touches unrelated packages | Scope to the BGP subtree first; land the pattern, defer broad rollout in Known Limitations |
| R-3 | uint64 fix silently changes an existing (accidentally relied-upon) truncation | interop/config test flips | Boundary test + explicit note; treat as bug fix per `ai/rules/completion.md` |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config commit changes a peer's ASN in memory (file unchanged on disk) | → | `ApplyConfigDiff` parses from resolved tree, not `reloadFunc(configPath)` | `test/plugin/config-apply-consumes-diff.ci` |
| a uint64-valued BGP leaf set to a value > 2^32-1 | → | typed decode preserves the full value | `test/plugin/config-uint64-no-truncation.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Production config apply (reloadFunc present) | `ApplyConfigDiff`/`VerifyConfig` parse peers from the resolved tree they are handed; no `reloadFunc(configPath)` disk read occurs on the apply/verify path (assert via seam counter/log) |
| AC-2 | Same config resolved via tree vs via disk re-read | Parsed `PeerSettings` are equivalent (no false diffs in `peerSettingsEqual`; all fields present) |
| AC-3 | BGP config values (ASN, router-id, MED, local-pref, families) | Obtained via schema-typed accessors; `PeersFromTree` no longer hand-parses raw strings for these leaves |
| AC-4 | A BGP-consumed uint64 leaf with value > 2^32-1 (or a guard if none exists) | Value is preserved end to end; a boundary test proves no silent uint32 truncation |
| AC-5 | Plugin config delivery + `ze config diff` | Output byte-identical to pre-change (delivery format and diff JSON preserved) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | commits a peer ASN change | CLI commit -> transaction resolve -> `ApplyConfigDiff(tree)` -> typed parse -> reconcile | `test/plugin/config-apply-consumes-diff.ci` |
| 2 | sets a 64-bit config value | config -> schema typed decode -> BGP field | `test/plugin/config-uint64-no-truncation.ci` |
| 3 | runs `ze config diff` after edit | Tree diff -> `rpc.ConfigDiffSection` JSON | existing config-diff `.ci` unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyConfigDiffParsesFromTree` | `internal/component/bgp/reactor/reactor_api_test.go` | production path parses from tree, no disk read | |
| `TestPeersFromTreeTypedDecode` | `internal/component/bgp/reactor/config_test.go` | typed accessors replace hand parse; equivalent PeerSettings | |
| `TestYangUint64NoTruncation` | `internal/component/config/yang_schema_test.go` | uint64 leaves preserved (boundary) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| uint64 config leaf | 0 .. 2^64-1 | 18446744073709551615 | N/A | N/A |
| uint32 (ASN/MED/local-pref) | 0 .. 2^32-1 | 4294967295 | N/A | overflow rejected |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-apply-consumes-diff` | `test/plugin/config-apply-consumes-diff.ci` | edit applied from resolved tree, not disk re-read | |
| `config-uint64-no-truncation` | `test/plugin/config-uint64-no-truncation.ci` | 64-bit value survives apply | |

### Interop Tests (MANDATORY for protocol features)
Not applicable: this is an internal config-representation change with no wire-format effect.
Peer-facing behavior is unchanged; existing BGP interop scenarios must remain green as a regression gate.

### Future (if deferring any tests)
- Broad typed-decode rollout to non-BGP consumers is deferred (see Known Limitations); its
  tests land with that follow-on.

## Files to Modify
- `internal/component/bgp/reactor/reactor_api.go` - `loadPeersFullOrTree`: parse from resolved
  tree on the production path; keep test fallback.
- `internal/component/bgp/reactor/config.go` - `PeersFromTree`/`parsePeerFromTree`: use
  schema-typed accessors instead of `mapUint32`/`mapString` hand parsing.
- `internal/component/config/yang_schema.go` - stop collapsing `Yuint64` into `TypeUint32`
  (add `TypeUint64` or preserve width); keep enum handling documented.
- `internal/component/config/tree.go` - typed accessor surface for schema-known leaves (read
  side only; storage may remain string-backed initially).
- `internal/component/config/transaction/` - ensure the resolved tree passed to consumers is
  fully resolved (templates + defaults) so parse-from-tree matches the disk re-read.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (type width) | [ ] | `internal/component/config/yang_schema.go` (uint64 width) |
| Functional test for changed apply path | [ ] | `test/plugin/config-apply-consumes-diff.ci` |
| Prometheus counter (optional: disk-reread eliminated) | [ ] | reactor telemetry, if a regression guard is wanted |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` config pipeline section |
| 16 | Changed source referenced by doc anchors? | [ ] | grep `docs/` for `source: .../reactor_api.go` and `yang_schema.go` |

## Files to Create
- `test/plugin/config-apply-consumes-diff.ci` - proves apply consumes the resolved tree.
- `test/plugin/config-uint64-no-truncation.ci` - proves 64-bit config values survive.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate |
| 6. Full verification | `./le verify current mode full` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — add the seam counter/log that proves whether a disk
   re-read happened; write `config-apply-consumes-diff.ci` expecting no disk read (fails now).
   - Files: `reactor_api.go`, `test/plugin/config-apply-consumes-diff.ci`
   - Verify: test fails because production still re-reads disk.
2. **Phase: BGP consumes the resolved tree** — parse peers from the passed tree; guarantee the
   tree is fully resolved first so `PeerSettings` match the disk-read result.
   - Tests: `TestApplyConfigDiffParsesFromTree`, then `config-apply-consumes-diff.ci` passes.
3. **Phase: Typed decode for the BGP subtree** — replace `mapUint32`/`mapString` hand parsing
   with schema-typed accessors; remove `Yuint64 -> TypeUint32` coarsening; add boundary test.
   - Tests: `TestPeersFromTreeTypedDecode`, `TestYangUint64NoTruncation`, `config-uint64-no-truncation.ci`.
4. **Functional tests** → the two `.ci` files above.
5. **Full verification** → `./le verify current mode full`.
6. **Complete spec** → learned summary `plan/learned/NNN-typed-config-decode.md`; two commits.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Parsed PeerSettings equivalent to disk-read result; no false diffs |
| Data flow | Resolved tree is the single source; no side disk read on apply |
| Registration over hardcoding | Typed decode is schema-driven; no per-leaf switch added to a core package |
| Rule: no-layering | Disk re-read path fully removed from production, not left dormant |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| No production disk re-read | grep `reloadFunc(configPath)` reachable only from test fallback; seam counter zero in `.ci` |
| Typed decode | grep shows `mapUint32`/`mapString` removed from `PeersFromTree` scalar leaves |
| uint64 preserved | `config-uint64-no-truncation.ci` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Typed decode still rejects out-of-range/malformed leaves (no panic on bad config) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| False diff after tree consumption | Ensure full resolution before parse (Phase 2) |
| uint64 test flips interop | Treat as real bug fix; document in Mistake Log |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Consume the resolved tree instead of re-reading disk | Keep disk re-read, only add typed decode | The re-read is the "worst single instance"; the transaction already computes the exact data |
| Scope typed decode to the BGP subtree first | Repo-wide typed decode in one spec | Keeps the spec closeable; lands the pattern without a mega-diff |

## Known Limitations
- Repo-wide typed decode (non-BGP consumers) is deferred; this spec establishes the pattern
  on the BGP path and the schema-width fix.
- The command/response envelope re-parse is not touched here (owned by
  `spec-unify-response-envelope.md`).
- BGP hot-path text serialization (filters, redistribution `update ...`) is not touched here
  (filter ordering: `spec-unify-filters.md`; redistribution text inject: a sibling follow-on).

## Implementation Summary

### What Was Implemented
- [filled during /implement]

### Bugs Found/Fixed
- [uint64 truncation, if confirmed real]

### Documentation Updates
- [core-design.md config pipeline section, or "None" with grep evidence]

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `./le verify current mode full` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (typed decode keyed off existing schema)
- [ ] No speculative features (BGP subtree only)
- [ ] Explicit > implicit behavior (single resolved-tree source)
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (uint64/uint32)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests N/A (internal change; existing BGP interop is the regression gate)

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Write learned summary to `plan/learned/NNN-typed-config-decode.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-review-typed-config-decode.md`
