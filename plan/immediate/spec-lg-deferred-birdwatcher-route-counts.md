# Spec: lg-deferred-birdwatcher-route-counts

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-08-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/lg/handler_api.go` - the birdwatcher transform and its `getNum` / `getStr` helpers
4. `internal/component/bgp/plugins/cmd/peer/summary.go` - the producer of the peer row
5. The cross-plugin count-aggregation record (retired with the learned corpus) - what Phase 4 shipped and why

## Task

Holds work deferred from `plan/spec-lg-birdwatcher-peer-fields.md` (Phase 4). That spec
is **closed and removed** from `plan/`: Phase 4 shipped in commit `79037a3d4` and the
spec file was deleted by the two-commit closure `4723c326c`. Its filled text survives in
git history, and its learned summary was retired with the learned corpus.

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

`routes_filtered` is **out of scope**: `plan/immediate/spec-bgp-filtered-route-storage.md` (status
`design`) already owns it end to end (AC-1..AC-8), so this spec must not duplicate it
(`ai/rules/planning.md`, "Choosing the Destination Spec"). Per "Verify Before
Deferring", the missing infrastructure was grepped for and is genuinely absent: no
filtered-route store exists (`ai/rules/repo-maintenance.md` line 15), and the specific
thing that would need adding is retention at the reject gate, which is that spec's design.

Fix G-1 first. It is the `ai/rules/evidence.md` failure mode exactly: a zero
that reads as a valid answer.

**STATE OF THE TREE, 2026-08-29. G-1, G-2 and G-3 are closed, and the three
paragraphs above are the pre-fix picture.** The owner ruled on 2026-08-05 that
this compatibility surface keeps all four counts unconditionally, so the answer
was never omission: the truth travels beside the counts in a fifth member,
`routes_counts_available`. The contract is
`docs/architecture/api/birdwatcher-compat.md` Sections 7.2 and 7.3, which state
the divergence from upstream birdwatcher and why it was taken.

| Gap | Verdict | Producer checked |
|-----|---------|------------------|
| G-1 | CLOSED | `routeCountsAvailable` (`lg/handler_api.go`) is false when the producer omitted the count keys, and `transformProtocols` emits it beside the four counts |
| G-2 | CLOSED as documented | `mergeRibRouteCounts` (`cmd/peer/summary.go`) assigns the Adj-RIB-In size to both keys. Section 7.3 of the contract states the equality and divergence row 2 publishes it |
| G-3 | CLOSED | `transformBMPProtocols` (`lg/handler_api.go`) reports `routes_counts_available` false, which is what Section 7.2 requires of a peer whose counts no source produced |

**Remaining defect found and fixed in the same pass: the availability guard read
key PRESENCE, not the count's value.** A key holding a value `getNum` cannot
read as a number publishes the same fabricated zero as an absent key, so the
guard answered "available" over four zeros. That was live in the captured
response of the public endpoint: `testdata/handler/api-protocols-bgp.txt`
recorded `routes_counts_available: true` above four zeros, because
`mockDispatch` sends its counts as strings. `routeCountsAvailable` now derives
the answer from `numValue`, which reports whether the value read as a number
(`ai/rules/evidence.md`: `ok` proves the key exists, never that its value is
usable). The same mismatch already cost this transform once, which is why
`uptimeSeconds` exists.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/json-format.md` - birdwatcher snake_case is the documented exception
- [ ] `ai/rules/evidence.md` - why an absent count must not surface as 0
- [ ] `ai/rules/plugins.md` - `cmd/peer` must not import the RIB plugin

### Decision Records
- [ ] The cross-plugin count-aggregation record - the dispatch pattern, the "honest zero" decision, the race that killed the pre-policy source
- [ ] `plan/immediate/spec-bgp-filtered-route-storage.md` - owns `routes_filtered`; read before touching it

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
- `cmd/peer` gains no compile-time edge to `bgp/plugins/rib` (`./le plugin boundary check` stays green).
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
1. The LG handler queries the engine with `show bgp` through the command dispatcher (`lg/server.go` query path).
2. `handleBgpSummary` (`cmd/peer/summary.go`) builds one row per peer and calls `fetchRibRouteCounts`.
3. `fetchRibRouteCounts` forwards `show bgp rib status` to the `bgp-rib` plugin over RPC and parses the per-peer `route-counts` map, or returns nil.
4. `mergeRibRouteCounts` adds `routes-received` / `routes-accepted` / `routes-sent` to the row, or adds nothing.
5. `transformProtocols` reads those keys with `getNum` and writes the four snake_case birdwatcher fields, defaulting a missing key to 0.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LG component -> BGP engine | CommandDispatcher `show bgp` | [ ] |
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
| A-1 | Alice-LG tolerates an omitted or null count field | birdwatcher clients treat protocols entries as loosely typed | Must emit a sentinel instead of omitting | Read Alice-LG's own source, not a binding stub (`ai/rules/evidence.md`) | MOOT. The owner decided on 2026-08-05 that the counts stay, so the assumption is never relied on. Section 7.1 of the contract records what upstream does, read at `alice-lg/birdwatcher`, `bird/parser.go`, `setChangeCount` |
| A-2 | `received == accepted` is acceptable to operators if documented | learned 1158 accepted it deliberately | G-2 needs the racy pre-policy plumbing after all | Ask the user; re-read the race in `session_prefix.go` | HELD. Documented at the API surface instead of plumbed, Section 7.3 and divergence row 2 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Overlap with `spec-bgp-filtered-route-storage` | Both specs edit `transformProtocols` | That spec owns `routes_filtered`; this one must not touch it |
| R-2 | Chasing the pre-policy count reintroduces the data race learned 1158 avoided | `-race` failures around `prefixCounts` | Prefer documenting G-2 over new hot-path atomics |

## Wiring Test (MANDATORY)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| GET `/api/looking-glass/protocols/bgp`, RIB plugin absent | -> | counts stay 0 for compatibility, `routes_counts_available` false | `TestAPIProtocolsCountAvailabilityOverHTTP/no-counts` |
| GET `/api/looking-glass/protocols/bgp`, RIB loaded with `receive [ update state ]` | -> | real per-peer counts, `routes_counts_available` true | `TestAPIProtocolsCountAvailabilityOverHTTP/with-counts` |
| GET `/api/looking-glass/protocols/bgp`, a count key present but unreadable | -> | `routes_counts_available` false | `TestAPIProtocolsCountAvailabilityOverHTTP/text-counts` |
| GET `/api/looking-glass/protocols/bmp` | -> | `routes_counts_available` false on every entry | `TestAPIBMPProtocolsCountsUnavailableOverHTTP` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `bgp-rib` plugin not loaded | The four count fields are not published as 0; the response distinguishes unknown from zero (G-1) |
| AC-2 | `bgp-rib` loaded, peer holds N routes in and M out | `routes_received` is N, `routes_exported` is M |
| AC-3 | Peer established, holds zero routes | Counts are a real 0, not an omission |
| AC-4 | `routes_imported` requested | Either a distinct post-policy count, or documented as equal to `routes_received` at the API surface (G-2) |
| AC-5 | BMP-monitored peer queried | Counts are sourced or omitted, never hardcoded 0 (G-3) |
| AC-6 | Any condition | `routes_filtered` behavior is unchanged; that field belongs to `spec-bgp-filtered-route-storage` |

Evidence, 2026-08-29. Every row is driven from the HTTP endpoint, not from the
transform, because the endpoint is what an operator and Alice-LG read.

| AC | Met by | Evidence |
|----|--------|----------|
| AC-1 | `routes_counts_available` false, the four counts still emitted | `TestAPIProtocolsCountAvailabilityOverHTTP/no-counts` |
| AC-2 | `transformProtocols` reads the producer's keys | `TestAPIProtocolsCountAvailabilityOverHTTP/with-counts` asserts 60 in and 50 out |
| AC-3 | a real 0 keeps `routes_counts_available` true | `TestAPIProtocolsCountAvailabilityOverHTTP/zero-counts` |
| AC-4 | documented as an alias, not made distinct | `docs/architecture/api/birdwatcher-compat.md` Section 7.3 and divergence row 2. The pre-policy count would need the plumbing R-2 refuses |
| AC-5 | BMP rows declare their counts unavailable | `TestAPIBMPProtocolsCountsUnavailableOverHTTP`. The literal wording ("sourced or omitted") is superseded by the 2026-08-05 ruling: the counts stay, the truth travels beside them |
| AC-6 | untouched | `mergeRibRouteCounts` still never emits `routes-filtered`; Section 7.3 unchanged |

## End-to-End User Stories (MANDATORY for new features)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Views Ze peers in Alice-LG on a build without the RIB plugin | LG -> summary -> no counts -> UI shows unknown, not "0 routes" | (fill during design) |
| 2 | Views per-peer route counts in Alice-LG on a normal build | LG -> summary -> RIB dispatch -> real counts | (fill during design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRouteCountsAvailableFlagsFabricatedZeros` | `internal/component/lg/handler_api_test.go` | AC-1, AC-3 at the transform | done |
| `TestAPIProtocolsCountAvailabilityOverHTTP` | `internal/component/lg/handler_api_test.go` | AC-1, AC-2, AC-3 driven from the endpoint, plus the unreadable-value case | done |
| `TestBMPProtocolsDeclareCountsUnavailable` | `internal/component/lg/handler_api_test.go` | AC-5 at the transform | done |
| `TestAPIBMPProtocolsCountsUnavailableOverHTTP` | `internal/component/lg/handler_api_test.go` | AC-5 driven from the endpoint | done |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `lg-birdwatcher-counts-unavailable` | `test/ui/lg-birdwatcher-counts-unavailable.ci` | A build with no source for the counts answers `routes_counts_available` false over the compatibility zeros, with the real daemon behind the endpoint | done |

### Interop Tests (MANDATORY for protocol features)
Not a wire protocol change: this spec only affects the birdwatcher HTTP surface. Alice-LG
compatibility is covered by the functional test above.

### Future (if deferring any tests)
- (fill during design)

## Files to Modify

- [ ] `internal/component/lg/handler_api.go` - `routeCountsAvailable`, `getNum`, `numValue`, and the two transforms that emit the field
- [ ] `internal/component/lg/handler_api_test.go` - the transform tests and the two endpoint tests
- [ ] `internal/component/lg/port_check_test.go` - the divergence reason for the one captured response that changed
- [ ] `internal/component/lg/testdata/handler/api-protocols-bgp.txt` - regenerated capture
- [ ] `test/ui/lg-birdwatcher-counts-unavailable.ci` - the functional test
- [ ] `docs/architecture/api/birdwatcher-compat.md` - Section 7.2 states that availability is derived from the count's value
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - unchanged: G-2 needed no new key

## Implementation Steps

1. Done. The wire representation is a fifth member beside the counts, not an omission: the owner ruled on 2026-08-05 that this compatibility surface keeps all four counts.
2. Done. `routeCountsAvailable` at the LG consumer carries the producer's omit semantics through as `routes_counts_available`.
3. Done. The alias is documented at the API surface, Section 7.3. The pre-policy plumbing is refused by R-2.
4. Done. BMP rows report the counts unavailable.
5. Done. The guard answers on the count's VALUE rather than on its key, so a count the transform cannot read is a placeholder too.

## Known Limitations
- `routes_filtered` stays 0 until `plan/immediate/spec-bgp-filtered-route-storage.md` lands. Not this spec's work.
- The pre-policy received count exists only on the `ze_bgp_prefix_count` gauge, by decision (learned 1158).
  **CORRECTED 2026-08-08: that gauge is not a received count under either mode.** The per-family `count` leaf makes it two different numbers. Under `offered` it tallies ANNOUNCEMENTS, so a peer re-announcing one prefix raises it without holding a second route. Under `installed` it is the SIZE OF THE SET this family delivered to plugins, so a prefix an over-limit UPDATE refused is absent from it (`applyInstalledPrefixSections`, `internal/component/bgp/reactor/session_prefix.go`). Neither is "what the peer advertised", and neither carries a label saying which one the reader got.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes (lint + all ze tests)
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

## Implementation Summary

### What Was Implemented
- `routeCountsAvailable` (`internal/component/lg/handler_api.go`) and the
  `routes_counts_available` member it feeds into `transformProtocols`. The four
  counts stay 0 for compatibility; the new member says whether they mean
  anything.
- `numValue` (`internal/component/lg/handler_api.go`) answers the number AND
  whether one was readable. `getNum` keeps its signature and delegates, so no
  caller that legitimately wants a zero changed.
- `transformBMPProtocols` (`internal/component/lg/handler_api.go`) reports
  `routes_counts_available` false: it consults no source for those four members.
- `docs/architecture/api/birdwatcher-compat.md`, the normative contract for this
  surface, Sections 7.2 and 7.3 and divergence rows 1 and 2.
- `test/ui/lg-birdwatcher-counts-unavailable.ci` drives the real daemon over
  HTTP on a build with no `bgp-rib` plugin.

### Bugs Found/Fixed
- The availability guard read key PRESENCE, not the count's value, so a count
  the transform could not read as a number published a fabricated zero under
  `routes_counts_available: true`. It was live in the committed capture
  `internal/component/lg/testdata/handler/api-protocols-bgp.txt`. Fixed at
  `routeCountsAvailable`, covered by
  `TestAPIProtocolsCountAvailabilityOverHTTP/text-counts`.

### Documentation Updates
- `docs/architecture/api/birdwatcher-compat.md` -- created by the implementation.
  Anchors `internal/component/lg/handler_api.go -- routeCountsAvailable`,
  `-- transformBMPProtocols`, and
  `internal/component/bgp/plugins/cmd/peer/summary.go -- fetchRibRouteCounts`,
  `-- mergeRibRouteCounts`.
- `docs/guide/looking-glass-howto.md` -- repaired at closure. It said "Route
  counts are present only when the `bgp-rib` plugin is loaded", which
  `transformProtocols` contradicts: all four counts are emitted on every peer.
  Its JSON sample gained `routes_counts_available`. Its anchor now names
  `routeCountsAvailable`.
- `docs/guide/looking-glass.md` -- repaired at closure. Same false sentence, and
  the page never named `routes_counts_available`. New anchor
  `internal/component/lg/handler_api.go -- routeCountsAvailable`.
- `./le doc check verify`: source-anchor pass clean over these edits. The run
  exits 1 on three findings in files this spec does not touch, named in
  Pre-Commit Verification.

### Deviations from Plan
- AC-1's literal wording ("the four count fields are not published as 0") and
  AC-5's ("sourced or omitted, never hardcoded 0") are superseded by the owner
  ruling of 2026-08-05: this is a compatibility surface, the counts stay, and
  the truth travels beside them. Recorded in
  `docs/architecture/api/birdwatcher-compat.md` Section 7.2.
- AC-4 (G-2) is specified rather than plumbed. A distinct pre-policy count needs
  the counter R-2 refuses. Section 7.3 and divergence row 2 publish the equality.
- `internal/component/bgp/plugins/cmd/peer/summary.go` is unchanged, as the
  Files to Modify table says: the consumer carries the producer's omit semantics.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first guard tested key PRESENCE, on the reading that a present key holds a usable value | `ok` proves the key exists, never that its value reads as a number. `mockDispatch` sends its counts as strings, and the committed capture recorded `routes_counts_available: true` above four zeros | The captured response of the public endpoint disagreed with the flag above it | `routeCountsAvailable` derives from `numValue`; `TestAPIProtocolsCountAvailabilityOverHTTP/text-counts` pins it |
| escalation | Two user guides kept a sentence the transform contradicts, and neither named the new member | `transformProtocols` emits all four counts on every peer, always | Closure step 4, grepping `docs/` for the count claims | Both pages repaired in commit A; the class is `doc-page-outlives-its-code` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| G-1: an unavailable count must not read as a measurement | Done | `routeCountsAvailable`, `internal/component/lg/handler_api.go` | The zero stays for compatibility; the truth is a fifth member |
| G-2: `routes_imported` must stop aliasing `routes_received`, or be documented as one | Done (documented) | `docs/architecture/api/birdwatcher-compat.md` Section 7.3, divergence row 2 | Producer `mergeRibRouteCounts` assigns the Adj-RIB-In size to both; R-2 refuses the racy pre-policy counter |
| G-3: BMP rows must source their counts or declare them | Done | `transformBMPProtocols`, `internal/component/lg/handler_api.go` | Emits `routes_counts_available: false` |
| `routes_filtered` out of scope | Done | unchanged | `mergeRibRouteCounts` still never emits `routes-filtered` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Changed | `TestAPIProtocolsCountAvailabilityOverHTTP/no-counts`, `test/ui/lg-birdwatcher-counts-unavailable.ci` | The response distinguishes unknown from zero through `routes_counts_available`, not through omission (2026-08-05 ruling) |
| AC-2 | Done | `TestAPIProtocolsCountAvailabilityOverHTTP/with-counts` | Asserts `routes_received` 60 and `routes_exported` 50 |
| AC-3 | Done | `TestAPIProtocolsCountAvailabilityOverHTTP/zero-counts` | A real 0 keeps the flag true |
| AC-4 | Changed | `docs/architecture/api/birdwatcher-compat.md` Section 7.3 | Documented as an alias rather than made distinct |
| AC-5 | Changed | `TestAPIBMPProtocolsCountsUnavailableOverHTTP` | The counts stay and declare themselves unavailable |
| AC-6 | Done | `mergeRibRouteCounts` unchanged | `routes-filtered` has no producer; owned by `spec-bgp-filtered-route-storage` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRouteCountsAvailableFlagsFabricatedZeros` | Done | `internal/component/lg/handler_api_test.go` | PASS |
| `TestAPIProtocolsCountAvailabilityOverHTTP` | Done | `internal/component/lg/handler_api_test.go` | PASS, four subtests |
| `TestBMPProtocolsDeclareCountsUnavailable` | Done | `internal/component/lg/handler_api_test.go` | PASS |
| `TestAPIBMPProtocolsCountsUnavailableOverHTTP` | Done | `internal/component/lg/handler_api_test.go` | PASS |
| `lg-birdwatcher-counts-unavailable` | Done | `test/ui/lg-birdwatcher-counts-unavailable.ci` | Passed under `./le functional ui` at commit `47cfece1c` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/lg/handler_api.go` | Done | `routeCountsAvailable`, `numValue`, `getNum`, both transforms |
| `internal/component/lg/handler_api_test.go` | Done | Four tests, 261 added lines across the two commits |
| `internal/component/lg/port_check_test.go` | Done | `lgPortCountsReadable` divergence reason |
| `internal/component/lg/testdata/handler/api-protocols-bgp.txt` | Done | `routes_counts_available` goes to false |
| `test/ui/lg-birdwatcher-counts-unavailable.ci` | Done | 77 lines |
| `docs/architecture/api/birdwatcher-compat.md` | Done | Sections 7.2, 7.3, divergence table |
| `internal/component/bgp/plugins/cmd/peer/summary.go` | Unchanged | As planned |

### Audit Summary
- **Total items:** 20
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (AC-1, AC-4, AC-5, each under the 2026-08-05 owner ruling, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator or Alice-LG can tell "this peer sent no routes" from "Ze cannot tell you" | functional, over the real daemon | `test/ui/lg-birdwatcher-counts-unavailable.ci`: a build with no `bgp-rib` answers `"routes_counts_available": false` beside `"routes_received": 0` on the live HTTP endpoint |
| The distinction survives a count the transform cannot read, not only an absent key | functional, over the handler chain | `TestAPIProtocolsCountAvailabilityOverHTTP/text-counts` PASS. Reverting `routeCountsAvailable` to the key-presence test reddens this subtest; returning true unconditionally reddens `text-counts` and `no-counts` (recorded in `47cfece1c`) |
| A BMP-monitored peer stops answering a confident zero | functional, over the handler chain | `TestAPIBMPProtocolsCountsUnavailableOverHTTP` PASS |
| The Alice-LG divergence is written down where a client author finds it | documentation | `docs/architecture/api/birdwatcher-compat.md` Sections 7.2, 7.3 and the five-row divergence table, each statement about upstream read at `alice-lg/birdwatcher`, `bird/parser.go`, `setChangeCount` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| A distinct pre-policy `routes_imported` | Ze drops rejects at the reactor gate before storage, so no pre-policy count exists. Building one reintroduces the race R-2 names | None. Documented as a permanent divergence, `docs/architecture/api/birdwatcher-compat.md` Section 7.3 |
| `routes_filtered` backed by a real store | Out of scope by AC-6 | `plan/immediate/spec-bgp-filtered-route-storage.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/lg-deferred-birdwatcher-route-counts-zeclose-lgcounts.md` |
| `./le spec session review check` | `OK (1 code files, clean, hashes match)`. One NOTE: the harness did not report the running model, so the review-model boundary is recorded as `unknown`. The review ran on Opus 5 |
| Rounds | 2 |
| Reviewer lenses used | Round 1: wiring + guard audit + logic + security + allocation + style pass, over the whole landed diff (`442d25073`, `47cfece1c`). Round 2: the two documentation repairs round 1 produced |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | "Route counts are present only when the `bgp-rib` plugin is loaded" is false: `transformProtocols` emits all four on every peer. The JSON sample omitted `routes_counts_available`. The page's own anchor names `transformProtocols`, the symbol the diff changed | `docs/guide/looking-glass-howto.md` | Sample and paragraph rewritten; anchor extended to `routeCountsAvailable`; link to the normative Sections 7.2 and 7.3 |
| 2 | ISSUE | The same false sentence, and the page never named `routes_counts_available` | `docs/guide/looking-glass.md` | Paragraph rewritten; anchor `internal/component/lg/handler_api.go -- routeCountsAvailable` added |

### Notes recorded, not blocking
- `handler_api.go`'s `// Design:` header names `docs/architecture/web-interface.md` and not the new normative contract, so `ai/DOCS-TO-CODE.md` carries no reverse row for it. Not raised to ISSUE: `ai/CODE-TO-DOCS.md` line 1863 already routes `handler_api.go` to `birdwatcher-compat.md`, which is the direction `ai/rules/documentation.md` names first.
- `routes_filtered` stays a fabricated 0 while `routes_counts_available` can be true. Bounded by AC-6 and stated in Section 7.3 and divergence row 3.
- `test/ui/lg-birdwatcher-counts-unavailable.ci` covers the unavailable path only. The available path runs through the real handler chain over HTTP in `TestAPIProtocolsCountAvailabilityOverHTTP/with-counts`; a `.ci` for it needs an established session with a loaded RIB.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/lg/handler_api.go` | Yes | `gopls symbols` lists `routeCountsAvailable` at 860:6 and `numValue` at 876:6 |
| `internal/component/lg/handler_api_test.go` | Yes | 993 lines, `wc -l` |
| `test/ui/lg-birdwatcher-counts-unavailable.ci` | Yes | 77 lines, `wc -l` |
| `docs/architecture/api/birdwatcher-compat.md` | Yes | 259 lines, `wc -l` |
| `internal/component/lg/testdata/handler/api-protocols-bgp.txt` | Yes | line 27 holds `"routes_counts_available": false` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Unknown is distinguishable from zero | `--- PASS: TestAPIProtocolsCountAvailabilityOverHTTP` (subtest `no-counts`), `go test ./internal/component/lg/` this session |
| AC-2 | Real counts reach the endpoint | same run, subtest `with-counts`, asserts 60 in and 50 out |
| AC-3 | A real 0 stays available | same run, subtest `zero-counts` |
| AC-4 | The alias is stated at the API surface | `sed -n '216,224p' docs/architecture/api/birdwatcher-compat.md` holds the MUST and the `mergeRibRouteCounts` anchor |
| AC-5 | BMP declares its counts unavailable | `--- PASS: TestAPIBMPProtocolsCountsUnavailableOverHTTP` |
| AC-6 | `routes_filtered` unchanged | `mergeRibRouteCounts` (`internal/component/bgp/plugins/cmd/peer/summary.go`) sets only `routes-received`, `routes-accepted`, `routes-sent`; read this session |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| GET `/api/looking-glass/protocols/bgp`, RIB plugin absent | `test/ui/lg-birdwatcher-counts-unavailable.ci` | Yes. Read in full: it starts `ze` with the looking glass on `$PORT2` and no rib plugin, waits on the endpoint, then asserts `"routes_counts_available": false`, `"routes_received": 0` and `"routes_exported": 0` |
| GET `/api/looking-glass/protocols/bgp`, counts present | `TestAPIProtocolsCountAvailabilityOverHTTP/with-counts` | Yes. `lgServeAPI` builds a real `LGServer` and calls `srv.server.Handler.ServeHTTP` |
| GET `/api/looking-glass/protocols/bgp`, count present but unreadable | `TestAPIProtocolsCountAvailabilityOverHTTP/text-counts` | Yes, same handler chain |
| GET `/api/looking-glass/protocols/bmp` | `TestAPIBMPProtocolsCountsUnavailableOverHTTP` | Yes, same handler chain |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | Moot, never relied on | The owner ruled on 2026-08-05 that the counts stay, so no omission reaches a client. Section 7.1 records upstream behavior, read at `alice-lg/birdwatcher`, `bird/parser.go`, `setChangeCount` |
| A-2 | Confirmed | Documented instead of plumbed. `mergeRibRouteCounts` assigns `c.in` to both `routes-received` and `routes-accepted`; Section 7.3 and divergence row 2 state it |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| API/RPC docs: the birdwatcher contract | `docs/architecture/api/birdwatcher-compat.md` Section 7.2 requires all four counts plus `routes_counts_available` derived from the VALUE. Matches `routeCountsAvailable` and `transformProtocols` | Yes |
| User guide: `docs/guide/looking-glass.md` | Repaired this session; the new paragraph matches `transformProtocols` and `transformBMPProtocols` | Yes |
| User guide: `docs/guide/looking-glass-howto.md` | Repaired this session; sample and paragraph match `transformProtocols` | Yes |
| CLI reference | No. `grep -rn 'routes_counts_available\|routes_received' docs/guide/command-reference.md` returns nothing: this member is HTTP-only and no CLI command prints it | Yes |
| Feature list, comparison table | No. `grep -rn 'routes_received\|route counts' docs/features/looking-glass.md docs/comparison.md` names only the peer dashboard's counts, which did not change | Yes |
| Wire format, RFC compliance, plugin SDK, config syntax | No. The diff adds one JSON member to an HTTP response and touches no wire path, no RFC-governed behavior, no SDK type and no YANG leaf | Yes |
| Doctor checks | No new runtime dependency: no file path, socket, port, module, binary or cert is added. `routeCountsAvailable` reads a map already in memory | Yes |
| `./le repository check` | `all checks passed`, run after the doc edits | Yes |
| `./le doc check verify` | Source-anchor pass clean over the edited pages. The run exits 1 on three findings outside this spec: a `bgp/defaults/attribute` YANG summary over the char and word caps, and anchors naming `getHelpExtension` in `internal/component/config/yang/command.go` and `Node.Help` in `internal/component/command/node.go`. Another session owns all three | Yes |

## Core Insight

A guard that asks "is the key there" answers a different question from "can I
use this value", and the two agree until a producer sends the right key with the
wrong type. The API had already paid for this once, in `uptimeSeconds`, where
the engine sends a duration as a string and `getNum` answered 0 for every real
response. The second occurrence landed inside the fix for the first class of the
same defect. When a transform reads an untyped map, availability is a property
of the VALUE, and any guard that reads the key instead cannot fail.
