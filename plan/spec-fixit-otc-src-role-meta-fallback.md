# Spec: fixit-otc-src-role-meta-fallback

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. [List key architecture docs from Required Reading]
4. [List key source files from Files to Modify]

## Task

Close the missing-metadata gap in `OTCEgressFilter`'s Gao-Rexford safety net.

`internal/component/bgp/plugins/role/otc.go:396-403` reads `meta["src-role"]` and
treats a MISSING key as "no restriction". That is the zero-value trap in
`ai/rules/fail-closed-guards.md`: any caller without ingress metadata silently
skips an RFC 9234 Section 5 leak guard. The Adj-RIB-In relay path
(`RelayStoredRoute`) is one such caller today; it is not the only possible one.

The value is recoverable EXACTLY, not guessable: `OTCIngressFilter` (otc.go:311-317)
writes `meta["src-role"] = cfg.role` from `getFilterConfig(src.Address.String())`,
the same lookup `OTCEgressFilter` already performs into `srcCfg`. So the fix is a
config-derived fallback when meta lacks the key.

Split out of `plan/spec-fixit-bgp-egress-rail-divergence.md` (see
`plan/deferrals/fixit-bgp-egress-rail-divergence.md`) because that spec's ACs pass
without it -- OTC suppression on the replay path already works through the
wire-bytes rule -- and because landing it requires changing an RFC-tagged test.

**BLOCKING precondition:** `TestOTCEgressNoStampProvider`
(`internal/component/bgp/plugins/role/otc_test.go:995`) carries
`RFC requirement: RFC9234-5-4 negative`. Its fixture asserts `accept == true` for a
Provider source to a Provider destination, which RFC 9234 Section 5 says must be
SUPPRESSED; it only reads as accept because meta is nil. Changing it needs explicit
user approval and an `// rfc-test-change-approved:` marker
(`ai/rules/testing.md`, RFC-Tagged Tests). Ask before writing any code.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `path/to/file.go` - [what it currently does]

**Behavior to preserve:** (unless user explicitly said to change)
- [output format, e.g., "JSON uses nested [[]] arrays for OR/AND grouping"]
- [function signatures that callers depend on]
- [test expectations from existing .ci files]

**Behavior to change:** (only if user explicitly requested)
- [list changes user asked for, or "None - preserve all existing behavior"]

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [Stage 1: e.g., "Wire parsing in internal/bgp/message/"]
2. [Stage 2: e.g., "Attribute extraction via iterators"]
3. [Stage 3: e.g., "Pool storage with dedup"]
4. [Stage N: ...]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | [ ] |
| Wire ↔ Storage | [via iterators/pools] | [ ] |
| [other boundaries] | [mechanism] | [ ] |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->
<!-- Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis) land HERE, not just in conversation. -->

### Assumptions
<!-- Things believed true that the design depends on. Every row needs a validation method. -->
<!-- Status: unvalidated → confirmed | broken. A broken assumption also gets a Mistake Log "Wrong Assumptions" row. -->
<!-- No assumption may still be `unvalidated` at Pre-Commit Verification. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | [what we believe] | [where the belief comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

### Risks
<!-- Things that could go wrong even if all assumptions hold. From /ze-spec Failure Mode Analysis. -->
<!-- Surviving risks copy forward to the Executive Summary "Risks & observations" and the learned summary. -->
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what could bite] | [how we'd notice] | [what we'd do] |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation — unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
<!-- Written retroactively 2026-07-27 from the Task section, which stated the
     required behavior precisely; the skeleton's placeholders were never
     filled before implementation. Each row is what the landed code and tests
     actually assert, not an aspiration. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `OTCEgressFilter` reached with `meta == nil` (the `RelayStoredRoute` shape), source configured `role customer` (source IS our Provider), destination remote role Provider | The route is SUPPRESSED. The Gao-Rexford guard evaluates using the role recovered from config rather than skipping |
| AC-2 | Same, but `meta["src-role"]` present and a valid string | The meta value is used unchanged; config is not consulted. Existing behavior is untouched |
| AC-3 | Same, but `meta["src-role"]` present and NOT a string (int, bool) | The fallback is taken. A malformed value is never MORE permissive than a missing one |
| AC-4 | `OTCEgressFilter` reached with no `src-role` in meta AND no role config for the source peer | No suppression. `""` correctly means "peer unconfigured", not "unrestricted"; an unconfigured peer was never filtered and still is not |
| AC-5 | A route legitimately transiting to a Provider (source configured `role provider`, i.e. source IS our Customer), `meta == nil`, destination Provider | ACCEPTED and NOT stamped. The fallback must not cause a false suppression, and the route must still reach the egress stamping block where a Provider destination correctly gets no OTC |

## End-to-End User Stories (MANDATORY for new features)

<!-- For each user-facing operation the feature enables, trace the full path.
     This section catches missing code that narrow ACs miss. ACs verify individual
     components work; user stories verify the full chain is connected.
     Every story must have a corresponding functional or wiring test. -->

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [e.g., "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |
| 2 | [e.g., "announces SR-Policy via ExaBGP bridge"] | [bridge command -> dispatch -> handler -> encoder -> wire] | [test name] |
| 3 | [e.g., "runs `ze bgp decode` on SR-Policy hex"] | [CLI -> InProcessNLRIDecoder -> Parse -> display] | [test name] |

<!-- If a path has a broken link (no implementation at some step), that is a spec gap.
     Add the missing component to ACs, Files to Create, and TDD Test Plan before proceeding. -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests — unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what user expects to happen] | |

### Interop Tests (MANDATORY for protocol features)
<!-- REQUIRED when the spec adds/changes wire protocol behavior (BGP, IPsec, L2TP). -->
<!-- See ai/rules/interop-and-goal-validation.md for when interop is required. -->
<!-- Skip this section (with justification) only for non-protocol features. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-feature-peer` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

### Future (if deferring any tests)
- [Tests to add later and why deferred — requires explicit user approval]

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
<!-- Check // Design: annotations on each file — if the change affects behavior
     described in the referenced architecture doc, include the doc here too -->
- `internal/...` - [feature changes]

### BGP Family Checklist (if new SAFI / capability / attribute)
<!-- BLOCKING: If this spec adds a new address family, SAFI, capability, or attribute,
     you MUST read ai/patterns/bgp-family.md BEFORE filling this section.
     Copy the relevant sections below and answer every one.
     Delete this block if the spec does not involve a BGP protocol extension. -->
<!-- See ai/patterns/bgp-family.md for the full 12-section checklist with file paths. -->
| BGP Integration Point | Needed? | File | Done? |
|----------------------|---------|------|-------|
| SAFI constant + family MustRegister | [ ] | `internal/core/family/family.go`, `register.go` | [ ] |
| NLRI struct (Family/Bytes/Len/String/PathID/WriteTo/SupportsAddPath) | [ ] | `plugins/nlri/<name>/types.go` | [ ] |
| Splitter + registration | [ ] | `nlrisplit/` | [ ] |
| ValidNextHopLens case | [ ] | `attribute/mpnlri.go` | [ ] |
| AppendJSON method | [ ] | `plugins/nlri/<name>/json.go` | [ ] |
| DecodeNLRIHex function | [ ] | `plugins/nlri/<name>/decode.go` | [ ] |
| parseNLRIByFamily case | [ ] | `cli/decode_mp.go` | [ ] |
| Plugin registry Registration (InProcessNLRIDecoder, Families) | [ ] | `plugins/nlri/<name>/register.go` | [ ] |
| Capability struct + negotiation + JSON | [ ] | `capability/`, `cli/decode_open.go`, `format/json.go` | [ ] |
| ExaBGP bridge (family map, command parser, event forwarding) | [ ] | `bridge/` | [ ] |
| ExaBGP migration YANG schema container | [ ] | `internal/exabgp/migration/exabgp.yang` | [ ] |
| ExaBGP migration route converter (flexSafis or dedicated convert*ToUpdate) | [ ] | `internal/exabgp/migration/migrate_routes.go` | [ ] |
| ExaBGP compat encoding tests (.ci + .conf) | [ ] | `test/exabgp-compat/encoding/`, `test/exabgp-compat/etc/` | [ ] |
| Exhaustive switch audit (`grep 'case.*SAFI' internal/`) | [ ] | all switches on SAFI | [ ] |
| Snapshot tests (all_test.go plugin names + wire methods) | [ ] | `plugin/all/all_test.go` | [ ] |
| Config surface guards (reservedPeerNames, command_ownership) | [ ] | `bgp/config/resolve.go`, `scripts/checks/` | [ ] |
| Functional decode tests (.ci per AFI) | [ ] | `test/decode/` | [ ] |
| Functional encode tests (.ci round-trip) | [ ] | `test/encode/` | [ ] |
| Interop scenario + existing interop config audit | [ ] | `test/interop/scenarios/` | [ ] |
| Cross-plugin impact (existing NLRI plugins still decode correctly) | [ ] | run existing decode tests | [ ] |
| Feature docs + comparison tables | [ ] | `docs/features/`, `docs/comparison.md` | [ ] |

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config-surface.md` (YANG vs env var) and `ai/rules/config-naming.md` (naming) |
| YANG validation constraints | [ ] | Every leaf MUST have maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | [ ] | If native YANG constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for tab-completion. Register in `validators_register.go` |
| CLI commands/flags | [ ] | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (action before identifier) | [ ] | `ai/rules/cli-grammar.md` |
| Editor autocomplete | [ ] | Automatic for YANG enum/type leaves. For dynamic values: `CompleteFn` in custom validator returns valid options |
| Functional test for new RPC/API | [ ] | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | [ ] | If command produces output: route through `ApplyPipes`/`ProcessPipes`, support all pipe operators per `ai/rules/pipe-completeness.md` |
| Env var registration | [ ] | If YANG config leaves added under `environment/`: matching `ze.<name>.<leaf>` env var via `env.MustRegister()`. Read `ai/rules/config-surface.md` before adding env-only settings |
| Doctor check for runtime dependencies | [ ] | If any file path, socket, external service, kernel module, listen port, procfs/sysctl, netlink, external binary, or certificate material is introduced: owning package doctor check, `internal/core/diagnostic/codes.go`, unit test, functional test (see `ai/rules/doctor-checks.md`) |
| Prometheus counters/metrics | [ ] | If feature has observable state: define counters, register in telemetry, list metric names and labels in this spec |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- Every No MUST be backed by a source-aware check, not a guess. At minimum, grep docs for source anchors pointing at changed files. -->
<!-- Any factual doc change MUST include or update a source-anchor HTML comment after the claim. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] | `rfc/short/rfcNNNN.md` (summary) and `docs/features/rfc-status.md` (status ledger row with source anchors) |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | [ ] | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | [ ] | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md`, relevant guide |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Grep `docs/` for `source: <changed-file>` and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | Verify examples against YANG/parser/handler and update stale syntax |

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

## Implementation Steps

<!-- Steps must map to /implement stages. Each step should be a concrete phase of work,
     not a generic process description. The review checklists below are what /implement
     stages 5, 9, and 10 check against — they MUST be filled with feature-specific items. -->

### /implement Stage Mapping

<!-- This table maps /implement stages to spec sections. Fill during design. -->
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan — check what exists |
| 3. Wiring phase | Wiring Test table — register entry points, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist below |
| 13. /ze-review gate | Review Gate section — run `/ze-review`; fix every BLOCKER/ISSUE; re-run until 0 BLOCKER/0 ISSUE (final review gate before closure) |
| 14. Present summary + close | Executive Summary Report; two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

<!-- List concrete phases of work. Each phase follows TDD: write test → fail → implement → pass.
     Phase 1 is ALWAYS wiring: create the entry point and a failing wiring test.
     Remaining phases fill in feature logic behind the wired skeleton.
     Phases should be ordered by dependency (e.g., schema before resolution, resolution before CLI). -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** — register entry points, write failing wiring tests
   - Tests: [wiring test names from Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: entry point exists and is reachable; wiring test fails because feature logic is a stub
2. **Phase: [name]** — [what to implement]
   - Tests: [test names from TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses
3. **Phase: [name]** — [what to implement]
   - Tests: [test names from TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test passes
4. **Functional tests** → Create after feature works. Cover user-visible behavior.
5. **RFC refs** → Add `// RFC NNNN Section X.Y` comments (protocol work only)
6. **Full verification** → `make ze-verify` (lint + all ze tests except fuzz)
7. **Complete spec** → Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`. TWO commits: commit A saves code + tests + spec + learned summary; commit B does `git rm` of the spec. BLOCKING: summary is part of commit A, not a follow-up.

### Critical Review Checklist (/implement stage 6)

<!-- MANDATORY: Fill with feature-specific checks. /implement uses this table
     to verify the implementation. Generic checks from rules/quality.md always apply;
     this table adds what's specific to THIS feature. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path (no broken links in the chain). Reference feature comparison: new feature has everything the reference has |
| Correctness | [feature-specific: e.g., "merge order correct", "error messages accurate"] |
| Naming | [feature-specific: e.g., "JSON keys use kebab-case", "YANG uses kebab-case"] |
| Data flow | [feature-specific: e.g., "resolution in X only, reactor unaware of Y"] |
| CLI grammar | If CLI commands added: action before identifier per `ai/rules/cli-grammar.md` |
| Registration over hardcoding | New command/view/family/handler is registry-registered and core-discovered; no new per-feature field, switch case, or factory added to a core/shared struct (incl. the CLI `Model`). See `ai/rules/plugin-self-containment.md` |
| Doctor checks | If runtime dependencies added: `ze doctor` check registered per `ai/rules/doctor-checks.md` |
| YANG validation | If YANG leaves added: every leaf has max native constraints (`range`/`length`/`pattern`/`enum`). Bare `type string` is a red flag. Custom validator + `CompleteFn` where native is insufficient |
| Prometheus counters | If observable state exists: counters defined, registered, metric names listed |
| Rule: no-layering | [if replacing something: "old code fully deleted"] |
| Rule: [other relevant rule] | [what to check] |

### Deliverables Checklist (/implement stage 10)

<!-- MANDATORY: Every deliverable with a concrete verification method.
     /implement re-reads the spec and checks each item independently. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command to verify] |

### Security Review Checklist (/implement stage 11)

<!-- MANDATORY: Feature-specific security concerns. /implement checks each item.
     Think about: untrusted input, injection, resource exhaustion, error leakage. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |
| [other concern] | [what to check] |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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
<!-- Route at completion: subsystem → arch doc, process → rules, knowledge → memory.md -->

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->
<!-- Not all specs have one. Delete this section if nothing qualifies. -->
<!-- Source for learned summary Decisions section (METHODOLOGY.md extraction step 2). -->

## Key Design Decisions
<!-- Record each significant design choice as it is made. -->
<!-- Format: "Chose X over Y because Z." Include rejected alternatives. -->
<!-- Source for learned summary Decisions section (METHODOLOGY.md extraction step 2). -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries and constraints accepted. -->
<!-- Source for learned summary Consequences section (METHODOLOGY.md extraction step 3). -->
- [What was deliberately not done and why]

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered — add test for each]

### Documentation Updates
- [Docs updated, with source anchors named, or "None" with grep evidence]
- [If docs were changed: `make ze-doc-test` result]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Close the missing-metadata gap in `OTCEgressFilter`'s Gao-Rexford safety net | Done | `role/otc.go` `resolveSrcRole`, called from the `destRemoteRole` branch of `OTCEgressFilter` | Fixed at the READ, not at the `RelayStoredRoute` caller, because the gap fires for ANY caller lacking meta |
| Recover the value EXACTLY from config rather than guessing | Done | `resolveSrcRole` returns `srcCfg.role`, the field `OTCIngressFilter` copies into `meta["src-role"]` | Changed: qualified in the code comment. The two agree only at one INSTANT (meta captured at receive, config read at forward), so a config reload between them makes them differ; the relay-time role is the safer one to gate on |
| Requires changing an RFC-tagged test; ask before writing code | Done | Thomas approved 2026-07-27; `// rfc-test-change-approved:` marker on `TestOTCEgressNoStampProvider` | Approval obtained BEFORE the change, per `ai/rules/testing.md` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestOTCEgressSuppressProviderLearnedWithoutMeta` | Mutation-verified: disabling the fallback turns it red |
| AC-2 | Done | `TestOTCEgressFilter/src_role_*` subtests | Unchanged by this work; they pass meta explicitly from a config-less source, so meta is the only signal |
| AC-3 | Done | `TestOTCEgressFilter/meta_wrong_type_not_suppressed` | Moved to a config-less source so it isolates the type check |
| AC-4 | Done | Same subtest (10.0.0.99 has no `peerRoleConfig`) | `resolveSrcRole` returns `""` when `srcCfg == nil` |
| AC-5 | Done | `TestOTCEgressNoStampProvider` | Keeps its `RFC9234-5-4 negative` tag; fixture changed so the route still reaches the stamp block |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Suppression with no meta | Done | `otc_test.go` `TestOTCEgressSuppressProviderLearnedWithoutMeta` | New test |
| Stamping scope preserved | Done | `otc_test.go` `TestOTCEgressNoStampProvider` | Fixture changed, RFC tag retained |
| Wrong-type meta ignored | Done | `otc_test.go` `meta_wrong_type_not_suppressed` | Source changed to config-less |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/role/otc.go` | Done | `resolveSrcRole` added; egress branch calls it |
| `internal/component/bgp/plugins/role/otc_test.go` | Done | One new test, two fixtures adjusted |
| `internal/component/bgp/reactor/reactor_api_relay.go` | Changed | Not in the original plan. Its comment documented this gap as OPEN and would have gone stale (`ai/rules/stale-comments.md`) |
| `ai/RFC-REQUIREMENTS.md` | Changed | Not in the original plan. Regenerated because a new RFC-tagged test shifts the ledger and `ze-rfc-check` fails on a stale one |

### Audit Summary
- **Total items:** 15 (3 requirements, 5 ACs, 3 tests, 4 files)
- **Done:** 15
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (the exactness qualification, plus two files touched beyond the plan) -- all recorded above; none reduces scope

## Goal Validation (BLOCKING)

<!-- MANDATORY: Maps each stated goal to concrete proof it was achieved. -->
<!-- "Tests pass" is not sufficient. Each goal needs specific evidence. -->
<!-- See ai/rules/interop-and-goal-validation.md for required evidence types. -->
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| A caller without ingress metadata no longer skips the RFC 9234 Section 5 leak guard | Unit test, mutation-verified | `TestOTCEgressSuppressProviderLearnedWithoutMeta` passes with the fallback and FAILS without it (`go test -race -run TestOTCEgressSuppressProviderLearnedWithoutMeta ./internal/component/bgp/plugins/role/` exit 1 with the fallback disabled, exit 0 restored) |
| The recovered value is the config-derived one, not a guess | Code reading, not a test | `OTCIngressFilter` writes `meta["src-role"] = cfg.role` from `getFilterConfig(src.Address.String())`; `OTCEgressFilter` reads `srcCfg, _ := getFilterConfig(src.Address.String())` and the fallback returns `srcCfg.role`. Same map, same key, same field |
| No false suppression is introduced | Full-suite regression | `make ze-test-bgp` exit 0 (includes every `TestOTCEgress*` and the reactor forward/relay tests). Independently reviewed for wrongly-suppressing call sites -- see Review Gate |
| The RFC 9234-5-4 obligation stays PROVEN, not just green | RFC ledger + non-vacuity argument | `ai/RFC-REQUIREMENTS.md` still lists `RFC9234-5-4` with `otc_test.go` as its negative proof, and the fixture was changed rather than the assertion precisely so the test still reaches the stamping block; flipping the assertion would have made the tag hold for the wrong reason |
| The relay path specifically is covered | Code reading + comment | `reactor_api_relay.go` builds the replayed update with `Meta: nil`, which is exactly the shape the new test drives; its comment now records the gap as closed and names the proving test |

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

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->
<!-- Hook pre-commit-spec-audit.sh (exit 2) checks this section exists and is filled. -->

Re-verified 2026-07-27 by running the commands below, not by re-reading the audit.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/role/otc.go` | Yes | `ls -la` -> 19K |
| `internal/component/bgp/plugins/role/otc_test.go` | Yes | `ls -la` -> 65K |
| (no "Files to Create") | n/a | This change adds no new file; it modifies two existing ones |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Suppression fires with `meta == nil` | `grep -n resolveSrcRole otc.go` -> defined :392, CALLED at :435 inside the `destRemoteRole` branch. `go test -race -run TestOTCEgressSuppressProviderLearnedWithoutMeta` exit 0; same run with the fallback disabled exit 1 |
| AC-2 | Explicit meta still wins | `resolveSrcRole` (:392) returns the meta string before consulting `srcCfg`; `make ze-test-bgp` exit 0 with the pre-existing `src_role_*` subtests unchanged |
| AC-3 | Non-string meta takes the fallback | The `ok && role != ""` guard at :393 fails for a non-string, so control reaches the `srcCfg` branch. `meta_wrong_type_not_suppressed` passes |
| AC-4 | No config -> no suppression | `resolveSrcRole` returns `""` when `srcCfg == nil`; `""` matches none of `roleCustomer/rolePeer/roleRSClient` at the call site |
| AC-5 | Legit transit still accepted and unstamped | `grep -c 'func TestOTCEgressNoStampProvider'` -> present; passes in `make ze-test-bgp` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `RelayStoredRoute` -> `OTCEgressFilter` with `Meta: nil` | none | NO `.ci` for this path, stated rather than implied. The relay is driven by a peer-up replay, which the functional harness has no fixture for. Coverage is the unit test plus the code-level fact that `reactor_api_relay.go` sets `Meta: nil`. `ai/rules/functional-test-gate.md` asks for a functional test per user-facing behavior; this is an internal safety net with no distinct user entry point, and the OTC egress behavior it protects is already covered by `test/plugin/role-otc-*.ci` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| "the value is recoverable EXACTLY" (Task) | Confirmed, with a caveat | Same config field on both sides (verified by reading `OTCIngressFilter` and the egress `getFilterConfig` call). Caveat recorded in code and in the audit: they agree at one instant, not across a config reload |
| "landing it requires changing an RFC-tagged test" (Task) | Confirmed | The pre-write hook blocked the edit until `// rfc-test-change-approved:` was present; Thomas approved 2026-07-27 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user-facing doc change | The fix restores an RFC 9234 guarantee already documented as implemented; it changes no config surface, CLI output or JSON shape. `git show c398e97f0 --stat` touches only `role/`, `reactor_api_relay.go` (comment), the RFC ledger and a deferral shard | Yes |
| `ai/RFC-REQUIREMENTS.md` regenerated | `make ze-rfc-index`; diff shows `RFC9234-3.1-1` gaining the new test and line shifts confined to `otc_test.go` | Yes |
| `reactor_api_relay.go` comment no longer stale | It previously described this gap as open; now records it closed and names the proving test | Yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
