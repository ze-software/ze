# Spec: rpki-cache-preference-and-source-marking

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P0 |
| Release reason | Cache preference and source isolation must prevent one cache withdrawal corrupting another cache's validation data. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.** `bgp/rpki/cache-server/preference`
(`internal/component/bgp/plugins/rpki/yang/ze-rpki.yang`) is a `uint8` with
default 100, and its description says "Preference of this cache server". The
list it belongs to says Ze reads the validated ROA and ASPA records from those
servers, and that "Each entry carries the port, the preference and the source
address". RFC 8210 Section 10 says what a preference on a cache list means:
"The client router attempts to establish a session with each potential serving
cache in preference order and then starts to load data from the most preferred
cache to which it can connect and authenticate." The same section says "the
lower the value, the more preferred". A reader takes the leaf for cache
selection and failover order. This spec covers that one leaf and the multi-cache
behavior behind it.

**What Ze did when this spec was written (2026-09-06), and what landed since.**
At the spec's baseline, `(*rPKIPlugin).startSessions` started one RTR session
per cache server at once into the same ROA and ASPA stores, and the preference
was read by nothing. Commit `c594e072b7` (2026-09-20, "fix(rpki): the status ze
reports is the state ze is in") replaced that: `startSessions` now sorts the
servers stably by preference and builds ONE `cacheGroup` over the whole list,
and `(*cacheGroup).poll` (`internal/component/bgp/plugins/rpki/rtr_session.go`)
walks the list most preferred first and stops at the first cache that completes
a sync. Only that cache, `cacheGroup.holder`, feeds the stores. A cache that is
not the holder is forced into a Reset Query by `startFullSync`, and its payload
is staged in the session's pending lists until End of Data, where
`(*ROACache).Replace` and `(*aSPACache).Replace` swap the whole set under the
store lock. Commit `17d5ba4753` (2026-10-08) added the Cache Reset fail-back
test. So the store never holds two caches' records at once, and `vrpEntry`
needs no per-record source field: the source mark is the data lease generation
each session carries (`rtrDataLease`, `rtr_expire.go`), and a delta from a
session whose generation is not the live one is converted to a full sync
before it can touch the store.

**What is still wrong.** Three behaviors and the proof around them.

| # | Gap at HEAD | Producer | RFC text |
|---|-------------|----------|----------|
| 1 | A Cache Reset from the holder, when no more-preferred cache answered in this round, makes `poll` move on to the NEXT, less-preferred cache and load its whole set, instead of sending the holder a Reset Query | `(*cacheGroup).poll` treats `errRtrCacheResetReceivedWillDo` like any failure | RFC 8210 Section 8.3: "If there are no more-preferred caches, it MUST issue a Reset Query and get an entire new load from the cache." Section 4: "The router MUST choose the most preferred, by configuration, cache or set of caches so that the operator may control load on their caches and the Global RPKI." |
| 2 | After a failover, the more-preferred cache is tried again only when the standby's Refresh Interval runs out (3600 seconds by default), so fail-back can take an hour | `(*cacheGroup).Run` waits `poll`'s delay, which is the holder's refresh | RFC 8210 Section 10: "When a more-preferred cache becomes available, if resources allow, it would be prudent for the client to start fetching from that cache." |
| 3 | No interop scenario runs Ze against two real caches, so neither failover nor fail-back is proven against another implementation | `test/interop/scenarios/` has `rtr-stayrtr` and `rpki-frr`, each with one cache | `ai/rules/interop-and-goal-validation.md` |

The proof debt beside them: the tags `RFC8210-4-1` and `RFC8210-10-2` in
`rfc8210_rtr_preference_test.go` carry no discrimination record under
`rfc/discrimination/rfc8210/`, and the SHOULDs `RFC8210-8.4-2`,
`RFC8210-10-3` and `RFC8210-10-4`, which the code already performs, carry no
test tag.

**What closing it means.** Gaps 1 and 2 are fixed in `cacheGroup`, gap 3 gets a
named interop scenario with two StayRTR caches, and the five rows get their
tags and records. The multi-cache model is not reopened: the owner fixed it for
this spec (Key Design Decisions, "Owner decision, 2026-10-27").

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/guide/rpki.md` - the operator page for RTR caches, preference, expiry and No Data Available
  → Decision: "Ze tries them in preference order and uses the most preferred server that answers. A completed full synchronization atomically replaces the previous server's data; partial transfers do not replace the working set."
  → Constraint: every behavior change here (Cache Reset, fail-back timing) edits this page in the same phase (`ai/rules/documentation.md`)
- [ ] `docs/architecture/testing/interop.md` - the StayRTR worked example, lab addresses, `opStop`/`opStart`
  → Constraint: StayRTR runs at 172.30.0.12 with `-refresh 10`; a second cache needs its own lab address (host octet 14 is free in `internal/le/interoplab/bgp/prepare.go`) and a row in the address table
- [ ] `docs/contributing/rfc-conformance-gates.md` - tag, discrimination record and audit verdict rules
  → Constraint: a tag added, or a tagged unit whose behavior changes, carries a record made by `./le rfc record` in the same commit

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8210.md` - rows `RFC8210-4-1`, `RFC8210-8.3-1`, `RFC8210-8.3-2`, `RFC8210-8.4-1`, `RFC8210-8.4-2`, `RFC8210-10-1`, `RFC8210-10-2`, `RFC8210-10-3`, `RFC8210-10-4`, `RFC8210-6-3`, `RFC8210-6-4`
  → Constraint: Section 8.3 "If there are no more-preferred caches, it MUST issue a Reset Query and get an entire new load from the cache."
  → Constraint: Section 6 "The router SHOULD NOT poll the cache sooner than indicated by this parameter." A fail-back probe MUST NOT re-query the holder early
- [ ] `rfc/short/draft-ietf-sidrops-8210bis.md` - ASPA records travel on the same session and the same End of Data
  → Constraint: ASPA follows the holder exactly as VRPs do; no ASPA-specific selection exists or is added

**Key insights:** (minimal context to resume after compaction)
- One holder at a time: the stores never hold two caches' data, so source marking is the lease generation, not a per-record field.
- The open defects are Cache Reset from the holder (moves to a less-preferred cache) and fail-back latency (holder's refresh interval).
- Owner fixed the model on 2026-10-27: FRR/rtrlib, most preferred reachable cache, never a union.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - `startSessions` sorts `CacheServers` stably by preference and starts one `cacheGroup`; `statusCommand` renders per-session `preference`, `state`, `synced`
- [ ] `internal/component/bgp/plugins/rpki/rtr_session.go` - `cacheGroup`, `Run`, `poll`, `syncOnce`, `startFullSync`, `handlePDU` (Cache Reset sets `fullSync` and returns `errRtrCacheResetReceivedWillDo`; No Data Available returns `errRtrNoDataAvailable`), End of Data applies `Replace` on full sync and `ApplyDelta` otherwise, `Snapshot.Synced` is `synced` and the lease generation current
- [ ] `internal/component/bgp/plugins/rpki/rtr_expire.go` - `rtrDataLease` shared by every session of the group, generation and Expire deadline
- [ ] `internal/component/bgp/plugins/rpki/roa_cache.go` - `vrpEntry` (MaxLength, ASN), `Replace`, `ApplyDelta`
- [ ] `internal/component/bgp/plugins/rpki/aspa_cache.go` - `aSPACache.Replace`, `ApplyDelta`
- [ ] `internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go` - `TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers`, `TestRTRCacheSwitchKeepsSerialBasesSeparate`, `TestRTRCacheResetReturnsToPreferredCache`
- [ ] `internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` - `leaf preference`, uint8, default 100, description "Lower values are tried first."
- [ ] `internal/le/interoplab/bgp/prepare.go` - StayRTR peer started when the scenario holds `vrps.json`, host 12
- [ ] `internal/le/interoplab/bgp/check_rpki_reload.go` - the `rtr-stayrtr` checker, the model for the new one
- [ ] `test/interop/Dockerfile.stayrtr` - StayRTR v0.6.4, `-refresh 10`, `-protocol 1`

**Behavior to preserve:** (unless the user explicitly said to change it)
- One holder at a time; a switch replaces the set at End of Data and never merges (`TestRTRCacheSwitchKeepsSerialBasesSeparate`).
- Equal preferences keep the operator's config order (stable sort); RFC 8210 Section 10 allows "a non-unique preference value".
- No Data Available moves to the next cache in the same round; with no other cache, periodic Reset Queries (`TestNoDataAvailableKeepsResetQueryMode`).
- `show bgp rpki status` JSON keys and session order (most preferred first).

**Behavior to change:** (only what the user asked for)
- Cache Reset from the holder with no more-preferred cache answering: Reset Query to the same cache in the same round.
- While the holder is not the most preferred cache: probe the more-preferred caches at their Retry Interval, without polling the holder before its Refresh Interval.
- `ze:help` of the preference leaf states the order ("lower is preferred").

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator configuration arrives as a YANG-modeled config tree, at load and at commit.
- Each RTR cache server sends ROA and ASPA PDUs over its own TCP session.

### Transformation Path
1. Config: `bgp/rpki/cache-server` entries parse into `cacheServerConfig` (`rpki_config.go`), preference included.
2. Selection: `startSessions` sorts stably by preference and hands the ordered sessions to `newCacheGroup`; `cacheGroup.Run` loops `poll` rounds.
3. Round: `poll` tries sessions most preferred first; a non-holder gets `startFullSync`; `syncOnce` dials, queries, reads to End of Data, closes.
4. Staging: payload PDUs collect in the session's pending lists; nothing reaches a store before End of Data.
5. Apply: End of Data runs `Replace` (full sync) or `ApplyDelta` (delta from the holder whose lease generation is current) on `ROACache` and `aSPACache`, renews the lease, and fires re-validation.
6. Read side: validation reads the stores without knowing the source (RFC 8210 Section 10, "MUST NOT distinguish between data sources"); `statusCommand` reads `Snapshot` per session.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network ↔ Plugin | RTR PDUs over TCP or TLS from each cache, one connection per round | Yes, `serveRTR` unit fakes and the `rtr-stayrtr` lab |
| Config ↔ Plugin | `bgp rpki` subtree delivered on configure and reload, `startSessions` restarts the group | Yes, `TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers` drives it from parsed config |
| Plugin ↔ Operator | `show bgp rpki status` JSON, per-session `preference` and `synced` | Yes, `rpki_status_test.go` |

### Integration Points
- `(*cacheGroup).poll` - the only place the Cache Reset and fail-back changes land; no new type.
- `(*RTRSession).pollDelay` and the session's `retryInterval` - the fail-back probe uses the more-preferred cache's own Retry Interval.
- `rtrDataLease.generation` - the source mark; unchanged, now named and tested as such.
- `internal/le/interoplab/bgp/prepare.go` and `check_special.go` - the second StayRTR peer and the new checker register beside the existing ones.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every store write stays in the End of Data handler under the store lock |
| No unintended coupling (components stay isolated) | Yes | Changes stay inside the `bgp-rpki` plugin package and the interop lab |
| No duplicated functionality (extends existing, does not recreate) | Yes | `poll` is edited; no second selector, no per-record source field beside the lease (`ai/rules/no-layering.md`) |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Not on a wire-encoding path; pending lists and `Replace` are unchanged |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The checker registers in the lab's existing scenario map; no core package changes |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `startSessions` builds one `cacheGroup` over all servers, sorted by preference | `rpki.go` `startSessions` at HEAD, commit `c594e072b7` | the whole spec | read at HEAD 2026-10-27 | validated |
| A-2 | A Cache Reset from the holder makes `poll` try the next, less-preferred cache | `poll` has no branch for `errRtrCacheResetReceivedWillDo`; `syncOnce` special-cases only the version downgrade | gap 1 would not exist and AC-1 would pass on HEAD | read at HEAD; Phase 1's first test must go red on HEAD before the fix | validated by reading, red owed |
| A-3 | The stores never hold two caches' data: non-holders always full-sync, End of Data replaces | `poll` calls `startFullSync` for every non-holder; End of Data line `if fullSync { Replace }`; lease generation check before a delta | source marking would need a per-record field | `TestRTRCacheSwitchKeepsSerialBasesSeparate` audit verdict `enforced` | validated |
| A-4 | Fail-back waits for the holder's Refresh Interval | `poll` returns `s.pollDelay(true)` of the holder | AC-2 would pass on HEAD | read at HEAD; Phase 1 test red owed | validated by reading, red owed |
| A-5 | The lab can run two StayRTR containers | `prepare.go` starts StayRTR from `vrps.json` at host 12; host 14 is unused; `opStop`/`opStart` exist in `check_engine.go` | Phase 3 needs a different second cache | grep of `Host:` values in `prepare.go` | validated |
| A-6 | StayRTR's 10-second refresh makes failover observable inside a checker timeout | `test/interop/Dockerfile.stayrtr` CMD `-refresh 10` | checker waits would need minutes | Dockerfile read | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Retrying the holder after Cache Reset loops forever against a cache that answers every query with Cache Reset | test with a fake that always resets | one immediate Reset Query per round; a second Cache Reset in the same round counts as a failure and the round moves on |
| R-2 | The fail-back probe polls the holder sooner than its Refresh Interval (RFC 8210 Section 6 SHOULD NOT) | a probe round that reaches the holder before its due time | a probe round only tries caches more preferred than the holder; the holder is queried only when its refresh is due |
| R-3 | A probe that succeeds against the preferred cache replaces the standby set with a partial load | the preferred cache drops mid-transfer | already covered: staging plus `Replace` at End of Data; a failed transfer leaves the holder's set live |
| R-4 | ROA and ASPA stores are replaced one after the other, so for an instant ROAs come from the new cache and ASPAs from the old | none observable by a withdraw | accepted: Section 10 forbids distinguishing sources in validation, and no update is applied to the other cache's data; recorded in Design Insights |
| R-5 | Interop timing flakes under host load | intermittent red in `rtr-stayrtr-preference` | the checker waits on state (BIRD route present or absent, ze status) with bounded polls, never on elapsed time |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Origin and ASPA validation runs on the wrong cache's set, or on none after a Cache Reset, so routes flip Valid, Invalid or NotFound |
| How is it reverted? | Single commit revert per phase; no config or wire format change |
| Who else touches this path? | Only the `bgp-rpki` plugin; the interop lab files are shared with other scenarios but the change is additive |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `bgp rpki cache-server` list with two preferences, applied through `startSessions` | → | `(*cacheGroup).poll`, Cache Reset branch | `TestCacheGroupCacheResetFromPreferredReloadsIt` |
| same config, preferred cache down then back | → | `(*cacheGroup).Run` delay and `poll` probe round | `TestCacheGroupFailsBackAtPreferredRetryInterval` |
| `show bgp rpki status` | → | `statusCommand` over `Snapshot` | `TestStatusNamesTheCacheWhoseDataIsInUse` |
| interop lab, two StayRTR caches, Ze, FRR source, BIRD sink | → | whole RTR client path to BGP export | `rtr-stayrtr-preference` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two caches, preference 10 holds the data; it answers a Serial Query with Cache Reset | In the same round Ze sends that cache a Reset Query and validation then uses its new full set; the preference 20 cache receives no connection (RFC 8210 Sections 4 and 8.3) |
| AC-2 | The preference 10 cache answers the Reset Query that follows a Cache Reset with another Cache Reset | The attempt counts as failed, the round moves to the preference 20 cache, and Ze does not loop on the first cache |
| AC-3 | Preference 10 is down, preference 20 holds the data with Refresh Interval 3600 and the preference 10 session's Retry Interval is short; preference 10 comes back | Ze sends preference 10 a Reset Query within its Retry Interval and switches to its set at End of Data; preference 20 receives no query before its own Refresh Interval (RFC 8210 Sections 6 and 10) |
| AC-4 | A switch is in progress: the new cache has sent Cache Response and payload, including a withdraw of a VRP the old cache announced, but not End of Data | Validation still answers from the old cache's set, unchanged; at End of Data the stores equal the new cache's set exactly, ROAs and ASPAs alike (RFC 8210 Section 10, `RFC8210-10-2`, `RFC8210-10-4`) |
| AC-5 | The holder answers No Data Available and a less-preferred cache answers | That cache supplies the data in the same round (`RFC8210-8.4-2`) |
| AC-6 | `./le rfc check` over `rfc8210` | Every tag in `rfc8210_rtr_preference_test.go` (`RFC8210-4-1`, `RFC8210-10-2`, `RFC8210-8.3-1`, `RFC8210-8.3-2`, `RFC8210-8.4-2`, `RFC8210-10-3`, `RFC8210-10-4`) has a discrimination record that observed red |
| AC-7 | `show bgp rpki status` after failover and after fail-back | Sessions listed most preferred first; exactly one session reports `synced` true while a set is live, and it is the cache whose set validation uses |
| AC-8 | `ze:help` of `bgp rpki cache-server preference`, `docs/guide/rpki.md` | Help says a lower value is preferred; the guide states the Cache Reset and fail-back behavior of AC-1 and AC-3 |
| AC-9 | Interop: StayRTR A at preference 10 authorises prefix P for FRR's AS, StayRTR B at preference 20 authorises P for another AS; `action invalid reject` | BIRD receives P from Ze while A runs; after A stops, BIRD loses P (B's set makes it Invalid); after A restarts, BIRD receives P again; `show bgp rpki status` names the live cache at each step |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures two caches with preferences; the preferred one restarts and loses state | StayRTR A Cache Reset or outage → `poll` → Reset Query to A or switch to B → End of Data `Replace` → re-validation → BIRD export | `rtr-stayrtr-preference`, `TestCacheGroupCacheResetFromPreferredReloadsIt` |
| 2 | Brings the preferred cache back after maintenance | A reachable → probe round at A's Retry Interval → A full load → `Replace` → re-validation | `rtr-stayrtr-preference`, `TestCacheGroupFailsBackAtPreferredRetryInterval` |
| 3 | Asks which cache is in use | `show bgp rpki status` → `statusCommand` → `Snapshot.Synced` | `TestStatusNamesTheCacheWhoseDataIsInUse`, `rtr-stayrtr-preference` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCacheGroupCacheResetFromPreferredReloadsIt` | `internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go` | AC-1: Reset Query to the same cache, standby never accepts; tags `RFC8210-8.3-1` (multi-cache) and `RFC8210-4-1` | |
| `TestCacheGroupRepeatedCacheResetMovesOn` | same file | AC-2: a second Cache Reset in one round fails the attempt and the standby loads | |
| `TestCacheGroupFailsBackAtPreferredRetryInterval` | same file | AC-3: preferred queried within its Retry Interval, standby not queried before its Refresh Interval | |
| `TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData` | same file | AC-4: staged load and staged withdraw invisible until End of Data, then exact replacement for ROA and ASPA; tags `RFC8210-10-2`, `RFC8210-10-3`, `RFC8210-10-4` | |
| `TestNoDataAvailableMovesToNextCacheInPreferenceOrder` | same file | AC-5; tag `RFC8210-8.4-2` | |
| `TestStatusNamesTheCacheWhoseDataIsInUse` | `internal/component/bgp/plugins/rpki/rpki_status_test.go` | AC-7 | |
| existing `TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers`, `TestRTRCacheSwitchKeepsSerialBasesSeparate` | `rfc8210_rtr_preference_test.go` | AC-6: discrimination records for their `RFC8210-4-1` and `RFC8210-10-2` tags | |
| `TestPreferenceHelpStatesOrder` | `internal/component/bgp/plugins/rpki/rpki_config_test.go` | AC-8: the YANG `ze:help` text of the leaf names the order | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `cache-server preference` | 0-255 (uint8) | 0 and 255 sort first and last | N/A (YANG type) | 256 refused by YANG |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rpki-cache-preference-failover` | `test/plugin/rpki-cache-preference-failover.ci` | Config lists the standby `le mock rtr` (preference 20, VRP authorising the route) first and the preferred one (preference 10) on a port with no listener; `show bgp rpki status` reports the standby as synced and the preferred one not, and the route validates against the standby's VRP | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `rtr-stayrtr-preference` | `test/interop/scenarios/rtr-stayrtr-preference/` | two StayRTR v0.6.4 caches, FRR source, BIRD sink | AC-9: preferred cache supplies the set, failover on stop, fail-back on restart, observed at BIRD's export; red when `poll` reads config order or never fails back | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/bgp/plugins/rpki/rtr_session.go` - `poll`: Cache Reset branch and probe round; `Run`: delay while the holder is not the most preferred
- `internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` - `ze:help` of `preference`
- `internal/component/bgp/plugins/rpki/rfc8210_rtr_preference_test.go` - new tests and tags
- `internal/component/bgp/plugins/rpki/rpki_status_test.go` - AC-7
- `internal/component/bgp/plugins/rpki/rpki_config_test.go` - AC-8
- `internal/le/interoplab/bgp/prepare.go` - second StayRTR peer from `vrps-standby.json`, host 14
- `internal/le/interoplab/bgp/names.go` - peer name `stayrtr-standby`
- `internal/le/interoplab/bgp/check_special.go` - register the new checker
- `rfc/short/rfc8210.md`, `rfc/discrimination/rfc8210/`, `rfc/audit/rfc8210/` - rows, records and verdicts for AC-6
- `docs/guide/rpki.md` - Cache Reset and fail-back
- `docs/architecture/testing/interop.md` - address table row and the scenario's section

## Files to Create
- `internal/le/interoplab/bgp/check_rpki_preference.go` - the `rtr-stayrtr-preference` checker
- `test/interop/scenarios/rtr-stayrtr-preference/ze.conf`, `frr.conf`, `bird.conf`, `vrps.json`, `vrps-standby.json` - the scenario
- `test/plugin/rpki-cache-preference-failover.ci` - functional test

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-rpki.yang`: `ze:help` text of `preference` only; no new node |
| YANG validation constraints | N-A | `preference` keeps `uint8`; the RFC sets no narrower range |
| YANG custom validators | N-A | no new constraint |
| CLI commands/flags | N-A | no command added; `show bgp rpki status` output keys unchanged |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/rpki-cache-preference-failover.ci` covers the status output under failover |
| Pipe completeness | N-A | output path unchanged |
| Env var registration | N-A | no `environment/` leaf |
| Doctor check for runtime dependencies | N-A | no new socket, port, file or binary; the caches are already configured dependencies |
| Prometheus counters/metrics | N-A | `setSessionsActive` keeps its meaning; no new metric |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP wire change |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | behavior correction of an existing leaf |
| 2 | Config syntax changed? | No | syntax unchanged; `docs/config-reference.md` mentions the YANG and is regenerated if it carries `ze:help` text |
| 3 | CLI command added/changed? | No | `show bgp rpki status` keys unchanged |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | behavior inside `bgp-rpki`, no registration change |
| 6 | Has a user guide page? | Yes | `docs/guide/rpki.md`: Cache Reset and fail-back paragraphs |
| 7 | Wire format changed? | No | RTR PDUs unchanged; only which cache is queried and when |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8210.md` rows `RFC8210-4-1`, `RFC8210-8.3-1`, `RFC8210-8.4-2`, `RFC8210-10-2`, `RFC8210-10-3`, `RFC8210-10-4`, and the `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`: second StayRTR address row and the scenario section |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` has no RTR failover claim to change |
| 12 | Internal architecture changed? | No | `docs/architecture/plugin/rib-storage-design.md` is declared by `rtr_session.go` but describes RIB storage, not RTR cache selection (grep finds no cache preference text); unaffected |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors` lists `docs/config-reference.md` (via `ze-rpki.yang`, checked in phase 2) and `docs/guide/vrrp.md` (via `prepare.go`, unaffected: it cites the keepalived peer, not StayRTR) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/rpki.md` preference table row and multi-server paragraph, checked against the YANG |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

Each phase is sized at about 60 tool calls.

1. **Phase 1: Wiring and selection fixes (AC-1, AC-2, AC-3, AC-7)** -- the entry point already exists (`startSessions` from parsed config), so wiring is the failing tests that drive it
   - Tests: `TestCacheGroupCacheResetFromPreferredReloadsIt`, `TestCacheGroupRepeatedCacheResetMovesOn`, `TestCacheGroupFailsBackAtPreferredRetryInterval`, `TestStatusNamesTheCacheWhoseDataIsInUse`; each red on HEAD first (A-2, A-4)
   - Files: `rtr_session.go` (`poll`, `Run`), `rfc8210_rtr_preference_test.go`, `rpki_status_test.go`, `docs/guide/rpki.md`
   - Steps: (a) in `poll`, a Cache Reset from a session ends with one immediate Reset Query to that same session, since every more-preferred session already failed in this round; a second Cache Reset counts as failure and the round moves on. (b) when the round ends on a holder that is not the head, the next wake is the earlier of the holder's Refresh Interval and the shortest Retry Interval among the failed more-preferred sessions; a wake before the holder is due tries only the more-preferred sessions, and the holder is queried only when its refresh is due. (c) RFC 8210 Section 8.3 and Section 10 quotes above the changed statements, per `ai/rules/rfc-compliance.md`
   - Verify: `./le test unit` on `./internal/component/bgp/plugins/rpki/...` red then green; `./le rfc record` for each new tag
2. **Phase 2: Source window proof, tags, help and docs (AC-4, AC-5, AC-6, AC-8)**
   - Tests: `TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData`, `TestNoDataAvailableMovesToNextCacheInPreferenceOrder`, `TestPreferenceHelpStatesOrder`, `rpki-cache-preference-failover.ci`; discrimination records for the existing `RFC8210-4-1` and `RFC8210-10-2` tags
   - Files: `rfc8210_rtr_preference_test.go`, `rpki_config_test.go`, `ze-rpki.yang`, `test/plugin/rpki-cache-preference-failover.ci`, `rfc/short/rfc8210.md`, `rfc/discrimination/rfc8210/`, `rfc/audit/rfc8210/`, `docs/features/rfc-status.md`
   - Verify: `./le rfc check` clean for rfc8210; `./le test functional` on the one `.ci`
3. **Phase 3: Interop scenario `rtr-stayrtr-preference` (AC-9)**
   - Tests: `INTEROP_SCENARIO=rtr-stayrtr-preference ./le test integration interop`
   - Files: `prepare.go`, `names.go`, `check_special.go`, new `check_rpki_preference.go`, scenario directory, `docs/architecture/testing/interop.md`
   - Steps: second StayRTR from `vrps-standby.json` at host 14 as `stayrtr-standby`; checker waits for P at BIRD, `opStop` the preferred StayRTR, waits for P to leave BIRD and for ze status to name the standby, `opStart` it, waits for P to return and for status to name the preferred cache
   - Verify: green run, then the vacuity walk: revert the Phase 1 `poll` change and the stable sort, rebuild the ze image, observe red, restore, observe green; record the red run in the spec

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No round ever loads a less-preferred cache while a more-preferred one answered in that round, Cache Reset included; the holder is never queried before its Refresh Interval |
| Naming | Test names state the behavior; JSON keys unchanged; scenario named without a numeric prefix |
| Data flow | Every store write stays in the End of Data handler; no per-record source field added beside the lease generation (`ai/rules/no-layering.md`) |
| Rule: `ai/rules/rfc-compliance.md` | RFC 8210 Section 8.3 and Section 10 quotes byte-exact above the statements that implement them; every new tag has a record |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| Cache Reset retries the same cache | `./le test unit` running `TestCacheGroupCacheResetFromPreferredReloadsIt` |
| Fail-back at the preferred Retry Interval | `./le test unit` running `TestCacheGroupFailsBackAtPreferredRetryInterval` |
| Records for every tag | `./le rfc check` and `ls rfc/discrimination/rfc8210/` |
| Interop scenario | `ls test/interop/scenarios/rtr-stayrtr-preference/` and its green run |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | A hostile cache answering every query with Cache Reset cannot pin Ze in a tight loop (AC-2) |
| Resource exhaustion | Fail-back probes run at the cache's Retry Interval, never faster, so a dead preferred cache costs one dial per Retry Interval |
| Fail-open | A failed or partial load never replaces the live set; Expire still clears data no cache refreshed |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

- The source mark is per store, not per record: only the holder writes, and the lease generation a session carries says whether its serial base is the live one. RFC 8210 Section 10 asks that "later updates MUST affect the correct data"; a delta from a session whose generation is stale is turned into a full sync before End of Data applies it.
- ROA and ASPA stores are replaced one after the other at End of Data, so for an instant ROAs come from the new cache and ASPAs from the old. No update is applied to the other cache's data, and Section 10 forbids distinguishing sources during validation ("implementations MUST NOT distinguish between data sources when performing validation of BGP announcements"), so this is not a source-marking breach (R-4).
- `syncOnce` closes the connection after each round, so Ze notices a dead holder only at the next round. Failover latency is therefore the holder's Refresh Interval (10 seconds in the StayRTR lab). That is the poll model Ze already ships and is not changed here.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| **Owner decision, 2026-10-27, for this spec:** load from the most preferred reachable cache, lower value preferred, and fail over by preference when it goes down or stale (the FRR/rtrlib model) | BIRD's model: each cache is its own `protocol rpki` and imports into the ROA table under its own route source, so the table holds the union | Thomas decided it. Evidence: FRR `get_groups` (`bgpd/bgp_rpki.c`) builds one `rtr_mgr_group` per cache with `rtr_mgr_groups[i].sockets_len = 1;` and `rtr_mgr_groups[i].preference = cache->preference;`, and `get_connected_group` returns `rtr_mgr_get_first_group(rpki_vrf->rtr_config)`, the single group in use. RFC 8210 Section 10: "The client router attempts to establish a session with each potential serving cache in preference order and then starts to load data from the most preferred cache to which it can connect and authenticate." Section 10 also: "the lower the value, the more preferred." Already the shape at HEAD (`c594e072b7`) |
| Failover rule: when the holder fails, try the next cache in preference order | Start every cache at once and keep standbys connected | rtrlib `_rtr_mgr_cb_state_error`: on a group error, "if (!is_some_rtr_mgr_group_established(config))" it takes `get_best_inactive_rtr_mgr_group` and calls `rtr_mgr_start_sockets(next_group, config)`, so a standby starts only when nothing is established. Ze's `poll` walks the sorted list each round, which is the same order |
| Fail-back rule: a recovered more-preferred cache takes over once it completes a full load, and the less-preferred one is then dropped | Stay on the standby until it fails | rtrlib `_rtr_mgr_cb_state_established`, from ERROR: "only change state to ESTABLISHED if all other more preferable socket groups are also in ERROR or SHUTDOWN state", then `rtr_mgr_close_less_preferable_groups` stops every group with `current_group->preference > group->preference`. RFC 8210 Section 10: "When a more-preferred cache becomes available, if resources allow, it would be prudent for the client to start fetching from that cache." |
| Fail-back cadence: probe the more-preferred caches at their own Retry Interval, never the holder before its Refresh Interval | Wait for the holder's Refresh Interval (HEAD, up to 3600 seconds) | RFC 8210 Section 6: Retry Interval "tells the router how long to wait before retrying a failed Serial Query or Reset Query", and Refresh Interval: "The router SHOULD NOT poll the cache sooner than indicated by this parameter." rtrlib's `_rtr_mgr_cb_state_error` sets the failing group to ERROR and does not stop its sockets, so they keep reconnecting while the standby serves (their reconnect timer lives in rtrlib `rtr.c`, not read here) |
| Cache Reset from the holder with no more-preferred cache answering: Reset Query to the same cache at once | Fail over to the next, less-preferred cache (HEAD) | RFC 8210 Section 8.3: "When a router receives this, the router SHOULD attempt to connect to any more-preferred caches in its cache list. If there are no more-preferred caches, it MUST issue a Reset Query and get an entire new load from the cache." BIRD `rpki.c`: `case CACHE_RESET:` moves the cache to `RPKI_CS_NO_INCR_UPDATE_AVAIL`, and from ESTABLISHED that goes to `RPKI_CS_RESET`, which sets `cache->serial_num = 0;` and sends a Reset Query to the same cache. rtrlib `rtr_mgr_cb` fails a group over only on `RTR_ERROR_FATAL`, `RTR_ERROR_TRANSPORT` and `RTR_ERROR_NO_DATA_AVAIL`, so Cache Reset does not switch groups |
| No Data Available: next cache in the same round | Wait a Retry Interval on the same cache | RFC 8210 Section 8.4: "When a router receives this kind of error, the router SHOULD attempt to connect to any other caches in its cache list, in preference order." rtrlib lists `RTR_ERROR_NO_DATA_AVAIL` among the group-error states. HEAD already does this |
| Merge for validation: only the active cache's set, never a union | Union across caches with per-record source (BIRD) | Owner decision above. Section 10 permits holding several caches' data but does not require it: "A client may hold data from multiple caches but MUST keep the data marked as to source, as later updates MUST affect the correct data." |
| Withdraw from one cache when another holds the same VRP | Per-record reference counts per source | Cannot occur: only the holder applies deltas, a new cache's load (withdraws included) is staged until its End of Data and then replaces the whole set (AC-4) |
| Equal preferences keep config order | Refuse duplicates as FRR's `find_cache` lookup by preference implies | RFC 8210 Section 10: "in a router, this list has a non-unique preference value for each server." HEAD's stable sort already honors it |
| ASPA follows the holder with the VRPs | Separate selection for ASPA | One session carries both; `draft-ietf-sidrops-8210bis` adds the ASPA PDU to the same session and End of Data |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- Equal-preference caches are not loaded as a set: Ze uses the first in config order. RFC 8210 Section 4 allows "cache or set of caches", so either is compliant, and FRR also uses one cache per preference.
- Failover detection waits for the next poll round because Ze closes the RTR connection after each sync. Holding a persistent connection is a different design and not owed by any RFC 8210 MUST.

## Goal Validation

| Goal (from Task) | Evidence owed |
|------------------|---------------|
| The preferred cache supplies the data, and a less-preferred one only while it cannot | `rtr-stayrtr-preference` against two StayRTR caches, with BIRD's export as the observed effect, plus its recorded red run |
| A Cache Reset never hands the load to a less-preferred cache | `TestCacheGroupCacheResetFromPreferredReloadsIt` with its `./le rfc record` red |
| Fail-back happens within the preferred cache's Retry Interval | `TestCacheGroupFailsBackAtPreferredRetryInterval` and the restart step of `rtr-stayrtr-preference` |
| A withdraw only ever hits its own cache's data | `TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData` and the `RFC8210-10-2` record |
| The operator can see which cache is in use | `rpki-cache-preference-failover.ci` and the status steps of `rtr-stayrtr-preference` |

## Disposition

| Field | Value |
|-------|-------|
| Date | 2026-10-27 |
| Checked at | `4831b585d9` (last commit touching the spec) on HEAD `2f36373081` |
| Verdict | REWORK |
| Evidence | `c594e072b7` (2026-09-20): `startSessions` sorts by preference and runs one `cacheGroup` |
| Evidence | `17d5ba4753` (2026-10-08): `TestRTRCacheResetReturnsToPreferredCache` |
| Evidence | `vrpEntry` still has no source field, and needs none: `poll` gives every non-holder `startFullSync`, End of Data `Replace`s |
| Evidence | `RTRSession.preference` is read by the sort and the status snapshot |
| Evidence | Still open: `poll` treats a Cache Reset like a failure and loads the next, less-preferred cache |
| Evidence | Still open: fail-back waits for the holder's Refresh Interval |
| Evidence | Still open: no two-cache interop scenario; no discrimination record for the `RFC8210-4-1` and `RFC8210-10-2` tags |

The goal holds and most of the original design landed in September under a
different spec's commit, so the spec was reworked to the remainder rather than
closed. Partly done is REWORK, never SUPERSEDED. ASPA shares the session and
the End of Data, so it needs no separate work.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
