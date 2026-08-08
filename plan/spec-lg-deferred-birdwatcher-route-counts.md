# Spec: lg-deferred-birdwatcher-route-counts

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/lg/handler_api.go` - the birdwatcher transform and its `getNum` / `getStr` helpers
4. `internal/component/bgp/plugins/cmd/peer/summary.go` - the producer of the peer row
5. `plan/learned/1158-cross-plugin-count-aggregation-via-dispatch.md` - what Phase 4 shipped and why

## Task

Holds work deferred from `plan/spec-lg-birdwatcher-peer-fields.md` (Phase 4). That spec
is **closed and removed** from `plan/`: Phase 4 shipped in commit `79037a3d4` and the
spec file was deleted by the two-commit closure `4723c326c`. Its filled text survives in
git history; its learned summary is `plan/learned/1158-cross-plugin-count-aggregation-via-dispatch.md`.

**The originating deferral row (dated 2026-07-15) is largely obsolete.** It claimed only
`routes_received` had a correct source and that `routes_imported` / `routes_exported` had
none. Phase 4 landed the day after and wired all three. Re-verified at the producers:

| Row claim | Verdict | Producer checked |
|-----------|---------|------------------|
| `state_changed` implemented | HOLDS | `stateChangedString` sets the row key, `cmd/peer/summary.go`, emitted at `summary.go` |
| `last_error` implemented | HOLDS | `lastErrorString`, `cmd/peer/summary.go`, emitted at `summary.go` |
| Only `routes_received` has a source | **STALE** | `mergeRibRouteCounts` (`cmd/peer/summary.go`) emits `routes-received`, `routes-accepted` and `routes-sent` |
| Source is `adj_rib_in/rib_commands.go` | **WRONG** | Counts come from `RIBManager.status` (`bgp/plugins/rib/rib_commands.go`), the `bgp-rib` plugin. `bgp-adj-rib-in` is a different plugin (learned 1158, "Traps") |
| `routes_filtered` untracked repo-wide | HOLDS | No producer emits `routes-filtered`; deliberate, `cmd/peer/summary.go`. Reject gate drops the route at `bgp/reactor/reactor_notify.go` |
| Counts are config-conditional | HOLDS, and it is the live bug | `fetchRibRouteCounts` returns nil when `bgp-rib` is absent (`summary.go`) |

**The four fields are present-but-always-zero, never absent.** `transformProtocols`
(`lg/handler_api.go`) always writes all four keys from `getNum`
(`:537-540`), and `getNum` returns 0 for a missing key (`lg/handler_api.go`).
So the remaining work is three specific gaps, not the "route counts are missing" the row describes:

| ID | Gap | Evidence |
|----|-----|----------|
| G-1 | The producer deliberately OMITS the count keys when `bgp-rib` is not loaded, "never faked to 0" (`summary.go, 139`). The LG then converts absent to `0` at `getNum` (`handler_api.go`) and publishes it. The consumer destroys the producer's honesty: Alice-LG cannot distinguish "this peer sent no routes" from "Ze cannot tell you" |
| G-2 | `routes_imported` equals `routes_received` by construction: both are the Adj-RIB-In size (`summary.go`). Ze drops rejects before storage, so there is no separate pre-policy count. Alice-LG shows two identical numbers where BIRD shows received >= imported |
| G-3 | `transformBMPProtocols` hardcodes all four fields to literal `0` for BMP-monitored peers (`handler_api.go`), with no source consulted at all |

`routes_filtered` is **out of scope**: `plan/spec-bgp-filtered-route-storage.md` (status
`design`) already owns it end to end (AC-1..AC-8), so this spec must not duplicate it
(`ai/rules/planning.md`, "Choosing the Destination Spec"). Per "Verify Before
Deferring", the missing infrastructure was grepped for and is genuinely absent: no
filtered-route store exists (`ai/rules/repo-maintenance.md` line 15), and the specific
thing that would need adding is retention at the reject gate, which is that spec's design.

Fix G-1 first. It is the `ai/rules/evidence.md` failure mode exactly: a zero
that reads as a valid answer.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/json-format.md` - birdwatcher snake_case is the documented exception
- [ ] `ai/rules/evidence.md` - why an absent count must not surface as 0
- [ ] `ai/rules/plugins.md` - `cmd/peer` must not import the RIB plugin

### Decision Records
- [ ] `plan/learned/1158-cross-plugin-count-aggregation-via-dispatch.md` - the dispatch pattern, the "honest zero" decision, the race that killed the pre-policy source
- [ ] `plan/learned/488-lg-looking-glass.md` - LG component boundaries
- [ ] `plan/spec-bgp-filtered-route-storage.md` - owns `routes_filtered`; read before touching it

**Key insights:**
- `handler_api.go`: `getNum` cannot express "absent". Any fix to G-1 lives here or at the call site.
- Phase 4 chose one owner (the RIB) for all counts, accepting `received == accepted`, because the pre-policy counter races the session write loop (learned 1158, "Trap").
- `bgp-rib` only fills its per-peer Adj-RIB-In when wired `receive [ update ]`. A functional test that forgets this gets empty counts.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/lg/handler_api.go` - `transformProtocols` (:521-568) writes all four fields; `transformBMPProtocols` (:210-249) hardcodes them to 0; `getNum` (:823-834) maps absent to 0
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `handleBgpSummary` builds the peer row (:239-240); `fetchRibRouteCounts` (:87-105) and `mergeRibRouteCounts` (:140-148) add the counts or omit them
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `RIBManager.status` (:668-705) owns the per-peer `route-counts` map
- [ ] `internal/component/lg/handler_api_test.go` - existing assertions on the four fields (:106-137)

**Behavior to preserve:**
- `state_changed` and `last_error` keep their current values and empty-string semantics for a peer that never transitioned or never errored (`summary.go, 63-65`).
- The producer keeps omitting count keys when the RIB plugin is absent. The fix belongs at the consumer; do not make the producer fake a 0.
- `cmd/peer` gains no compile-time edge to `bgp/plugins/rib` (`make ze-plugin-boundary-check` stays green).
- Family-scoped counts under a family-filtered summary (`rib_commands.go`).
- `/routes/filtered/{name}` and `/routes/noexport/{name}` keep returning empty lists until the filtered-storage spec lands.

**Behavior to change:**
- An unavailable count must not be published as 0 (G-1).
- `routes_imported` must stop being an alias of `routes_received`, or be documented as one at the API surface (G-2).
- BMP peer rows must source their counts or omit them (G-3).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- HTTP GET `/api/looking-glass/protocols/bgp` on the LG port, served by the birdwatcher API handler in `internal/component/lg/handler_api.go`.
- Alice-LG is the primary consumer of the response.

### Transformation Path
1. The LG handler queries the engine with `show bgp summary` through the command dispatcher (`lg/server.go` query path).
2. `handleBgpSummary` (`cmd/peer/summary.go`) builds one row per peer and calls `fetchRibRouteCounts`.
3. `fetchRibRouteCounts` forwards `show bgp rib status` to the `bgp-rib` plugin over RPC and parses the per-peer `route-counts` map, or returns nil.
4. `mergeRibRouteCounts` adds `routes-received` / `routes-accepted` / `routes-sent` to the row, or adds nothing.
5. `transformProtocols` reads those keys with `getNum` and writes the four snake_case birdwatcher fields, defaulting a missing key to 0.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LG component -> BGP engine | CommandDispatcher `show bgp summary` | [ ] |
| `cmd/peer` -> `bgp-rib` plugin | `ForwardToPlugin`, string-keyed, JSON over pipe | [ ] |
| Ze kebab-case -> birdwatcher snake_case | `transformProtocols` | [ ] |

### Integration Points
- `lg/handler_api.go` `transformProtocols`, `transformBMPProtocols`, `getNum`
- `cmd/peer/summary.go` `mergeRibRouteCounts`
- `rib/rib_commands.go` `RIBManager.status` per-peer `route-counts`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Alice-LG tolerates an omitted or null count field | birdwatcher clients treat protocols entries as loosely typed | Must emit a sentinel instead of omitting | Read Alice-LG's own source, not a binding stub (`ai/rules/evidence.md`) | unvalidated |
| A-2 | `received == accepted` is acceptable to operators if documented | learned 1158 accepted it deliberately | G-2 needs the racy pre-policy plumbing after all | Ask the user; re-read the race in `session_prefix.go` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Overlap with `spec-bgp-filtered-route-storage` | Both specs edit `transformProtocols` | That spec owns `routes_filtered`; this one must not touch it |
| R-2 | Chasing the pre-policy count reintroduces the data race learned 1158 avoided | `-race` failures around `prefixCounts` | Prefer documenting G-2 over new hot-path atomics |

## Wiring Test (MANDATORY)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| GET `/api/looking-glass/protocols/bgp`, RIB plugin absent | -> | count fields omitted or explicitly unknown, never 0 | (fill during design) |
| GET `/api/looking-glass/protocols/bgp`, RIB loaded with `receive [ update state ]` | -> | real per-peer counts | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `bgp-rib` plugin not loaded | The four count fields are not published as 0; the response distinguishes unknown from zero (G-1) |
| AC-2 | `bgp-rib` loaded, peer holds N routes in and M out | `routes_received` is N, `routes_exported` is M |
| AC-3 | Peer established, holds zero routes | Counts are a real 0, not an omission |
| AC-4 | `routes_imported` requested | Either a distinct post-policy count, or documented as equal to `routes_received` at the API surface (G-2) |
| AC-5 | BMP-monitored peer queried | Counts are sourced or omitted, never hardcoded 0 (G-3) |
| AC-6 | Any condition | `routes_filtered` behavior is unchanged; that field belongs to `spec-bgp-filtered-route-storage` |

## End-to-End User Stories (MANDATORY for new features)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Views Ze peers in Alice-LG on a build without the RIB plugin | LG -> summary -> no counts -> UI shows unknown, not "0 routes" | (fill during design) |
| 2 | Views per-peer route counts in Alice-LG on a normal build | LG -> summary -> RIB dispatch -> real counts | (fill during design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestProtocolsOmitsCountsWhenRibAbsent` | `internal/component/lg/handler_api_test.go` | AC-1: absent counts never surface as 0 | |
| `TestProtocolsRealZeroDistinctFromUnknown` | `internal/component/lg/handler_api_test.go` | AC-3: a real 0 still renders | |
| `TestBmpProtocolsCountsNotFaked` | `internal/component/lg/handler_api_test.go` | AC-5 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `lg-birdwatcher-route-counts` | `test/plugin/lg-birdwatcher-route-counts.ci` | Alice-LG reads per-peer counts with the RIB wired `receive [ update state ]` | |

### Interop Tests (MANDATORY for protocol features)
Not a wire protocol change: this spec only affects the birdwatcher HTTP surface. Alice-LG
compatibility is covered by the functional test above.

### Future (if deferring any tests)
- (fill during design)

## Files to Modify

- [ ] `internal/component/lg/handler_api.go` - `transformProtocols`, `transformBMPProtocols`, `getNum`
- [ ] `internal/component/lg/handler_api_test.go` - unit tests above
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - only if G-2 needs a new key

## Implementation Steps

1. (fill during design) Decide the wire representation for an unknown count (omit the key, or null) after reading Alice-LG's own source (A-1).
2. (fill during design) Fix G-1 at the LG consumer, keeping the producer's omit semantics.
3. (fill during design) Resolve G-2: document the alias, or justify the pre-policy plumbing against learned 1158's race.
4. (fill during design) Resolve G-3 for BMP rows.

## Known Limitations
- `routes_filtered` stays 0 until `plan/spec-bgp-filtered-route-storage.md` lands. Not this spec's work.
- The pre-policy received count exists only on the `ze_bgp_prefix_count` gauge, by decision (learned 1158).
  **CORRECTED 2026-08-08: that gauge is not a received count under either mode.** The per-family `count` leaf makes it two different numbers. Under `offered` it tallies ANNOUNCEMENTS, so a peer re-announcing one prefix raises it without holding a second route. Under `installed` it is the SIZE OF THE SET this family delivered to plugins, so a prefix an over-limit UPDATE refused is absent from it (`applyInstalledPrefixSections`, `internal/component/bgp/reactor/session_prefix.go`). Neither is "what the peer advertised", and neither carries a label saying which one the reader got.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING, before ANY commit)
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE. Paste the final clean run. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")
