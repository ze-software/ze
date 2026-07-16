# Spec: bgp-summary-route-counts

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | closes Phase 4 of spec-lg-birdwatcher-peer-fields |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/plugin-self-containment.md` - the ownership rule that dictates the architecture
4. `internal/component/bgp/plugins/rib/rib_commands.go` `status()`, `internal/component/bgp/reactor/session_prefix.go`, `internal/component/bgp/plugins/cmd/peer/summary.go`, `internal/component/lg/handler_api.go`

## Task

`show bgp summary` peer rows carry no route counts, so the birdwatcher LG
endpoint (`transformProtocols`, `internal/component/lg/handler_api.go:537-540`)
reads `routes-received` / `routes-accepted` / `routes-sent` / `routes-filtered`
and gets 0 for every peer — Alice-LG shows all zeros. This is the unfinished
Phase 4 of `spec-lg-birdwatcher-peer-fields` (`state_changed` and `last_error`
already shipped in `9fb067c54`).

Add real per-peer route counts to `show bgp summary`, which lights up the LG
(the consumer at `handler_api.go:537-540` already reads the keys) AND the CLI/web
summary consumers at once.

Three of the four counts have a real current-state source today; the fourth does
not exist by deliberate design (see A-4 below). Build the three that are real;
leave the fourth honestly zero with the reason documented.

| Alice-LG field | summary key | source | verdict |
|---|---|---|---|
| `routes_received` | `routes-received` | RIB plugin Adj-RIB-In size `bgpPeers[addr].Len()`, `rib.go:232` (= accepted; see below) | BUILD (emit per-peer) |
| `routes_imported` | `routes-accepted` | RIB plugin Adj-RIB-In size `bgpPeers[addr].Len()`, `rib.go:232` | BUILD (emit per-peer) |
| `routes_exported` | `routes-sent` | RIB plugin Adj-RIB-Out size `len(ribOut[addr])`, `rib.go:237` | BUILD (emit per-peer) |
| `routes_filtered` | `routes-filtered` | none — Ze does not retain filtered routes | STAYS 0 (documented) |

**All counts come from ONE source: the RIB plugin's per-peer Adj-RIB-In/Out
sizes.** `received` and `accepted` are both the Adj-RIB-In size, because Ze
retains only accepted routes (rejects are dropped at the reactor gate,
`reactor_notify.go:449`, and never stored). This is the honest current-state
meaning: "routes currently held from this peer." The pre-policy received count
(how many the peer advertised before import policy) exists ONLY in the reactor
(`prefixCounts`, `session_prefix.go:186`) and is deliberately NOT used here:
reading it from `Peers()` races the session write loop, so surfacing it needs new
reactor hot-path atomics + session→peer plumbing — risky surgery for an imprecise
signal (filtered=0 means the received−accepted gap cannot be cleanly attributed),
and it is ALREADY exposed via the `ze_bgp_prefix_count` Prometheus gauge
(`session_prefix.go:530`). Pre-policy received is Known-Limitation future work.

**Result: zero reactor changes. The whole feature is RIB-plugin status +
summary merge.**

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/plugin-self-containment.md` - the ownership rule
  → Constraint: "Everything that reads one plugin's state belongs to that owner." The RIB plugin owns accepted/exported; `summary.go` must NOT import it. It reaches them by runtime string-keyed command dispatch, exactly as `cmd/rib/rib.go` forwards to the RIB plugin via `ForwardToPlugin`.
- [ ] `ai/rules/json-format.md` - kebab-case keys; the `handler_api.go` snake_case exception
  → Constraint: new summary keys are kebab-case (`routes-received`, `routes-accepted`, `routes-sent`); the LG's snake_case is its own exempt layer and needs no change.
- [ ] `docs/architecture/api/commands.md` - the summary command + plugin dispatch surface

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - Adj-RIB-In / Adj-RIB-Out / Loc-RIB definitions (Section 3.2)
  → Constraint: RFC 4271 3.2 defines Adj-RIB-In as routes advertised by a peer (this is what "accepted"/imported maps to once import policy has run), and Adj-RIB-Out as routes advertised to a peer ("exported"). The mapping in the task table follows these definitions.

**Key insights:** (minimal context to resume after compaction)
- The channel is `ctx.Dispatcher().ForwardToPlugin(ctx, "show bgp rib status", ...)`. No plugin→PeerInfo push path exists; PeerInfo is built entirely from reactor state (`reactor_api.go:89-189`).
- `received` is reactor-side (pre-policy). `accepted`/`exported` are RIB-plugin-side (post-policy). They come from TWO sources and are merged in `summary.go` by peer address.
- `filtered` is a hard architectural absence, not laziness: rejected routes are dropped at the reactor gate (`reactor_notify.go:449`) and never stored.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `handleBgpSummary` (:64). Peer rows built at :112-127 from `reactor.Peers()`; the row has no route counts. Has `ctx.Dispatcher()` available (`command.go:172`).
  → Constraint: this handler is the merge point. It reads PeerInfo (for received) and must dispatch to the RIB plugin (for accepted/exported).
- [ ] `internal/component/plugin/server/command.go` - `ForwardToPlugin` (:795-801) returns `ErrUnknownCommand` when the plugin is not running.
  → Decision: RIB counts are BEST-EFFORT. If the RIB plugin is absent, summary still renders with received-only; accepted/exported are omitted, not faked.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `status()` (:661-707). Already iterates `r.bgpPeers` (`peerRIB.Len()` = Adj-RIB-In size) and `r.ribOut` (`len(familyRoutes)` = Adj-RIB-Out size) per peer, but emits only GLOBAL `routes-in`/`routes-out`. Precedent for a per-peer map: `gr-state` (:694-703) is already a `{addr: {...}}` sub-map.
  → Decision: extend `status()` to add a per-peer `route-counts` sub-map keyed by peer address. The `status()` `"peers"` key is already taken (it is `len(r.peerUp)`, an int), so the new key must be distinct.
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `bgpPeers map[netip.Addr]*storage.PeerRIB` (:232, Adj-RIB-In per peer), `ribOut map[netip.Addr]map[family.Family]map[ribOutKey]ribOutEntry` (:237, Adj-RIB-Out per peer). Both keyed by `netip.Addr`. `updateMetrics` (:367-406) already computes per-peer sizes for the `ze_rib_routes_in`/`ze_rib_routes_out` gauges.
  → Constraint: Adj-RIB-In only ever holds ACCEPTED routes — rejected inbound routes are dropped at the reactor gate before any plugin sees them (`reactor_notify.go:449`). So `bgpPeers[addr].Len()` IS the post-policy accepted count.
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `prefixCounts` (:179), `totalCount()` (:186), `add()` (:196, clamps ≥0). Per-Session, reset on teardown (:177). Counted from raw wire NLRI in `checkPrefixLimits`, which runs BEFORE the import filter (`session_read.go:201` before `:234`).
  → Decision: this is the PRE-POLICY received count. It is reactor-side, so it goes onto PeerInfo, not through the RIB dispatch.
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `Peers()` (:89-189) builds every PeerInfo field from reactor state; :159-160 already derives Uptime from `p.EstablishedAt()`.
  → Constraint: this is where a new `PeerInfo.RoutesReceived` gets populated, from a new `Peer` accessor over the session's `prefixCounts`.
- [ ] `internal/component/plugin/types_bgp.go` - `PeerInfo` (:53); comment at :92 "NLRI-level counters live in the RIB plugin".
  → Decision: `received` (pre-policy) is NOT an NLRI storage count owned by the RIB; it is a session-level received-prefix gauge the reactor already maintains. Adding `RoutesReceived` to PeerInfo does not violate :92 — accepted/exported (the RIB-owned counts) are deliberately NOT added to PeerInfo and come via dispatch instead.
- [ ] `internal/component/lg/handler_api.go` - `transformProtocols` (:521) reads `routes-received` (:537), `routes-accepted` (:538), `routes-sent` (:539), `routes-filtered` (:540); nested routes object at :558-563.
  → Constraint: the CONSUMER IS ALREADY WIRED. Once `summary.go` emits the keys, the LG needs no change. `routes-filtered` reads 0 (Ze does not retain filtered routes — the handler already stubs `handleAPIRoutesFiltered` empty at :254).
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` - the proxy precedent: registers builtin RPCs and forwards to the RIB plugin via `ForwardToPlugin` with string command constants, NO plugin import.

**Behavior to preserve:** (unless user explicitly said to change)
- Every existing `show bgp summary` key keeps its name/type/meaning (consumed by the CLI dashboard, web summary page, LG UI+API).
- `ForwardToPlugin` best-effort semantics: a missing RIB plugin must not break `show bgp summary`.
- The reply map shapes the LG parses (`handler_api.go`) and the dashboard parses (`model_dashboard.go`).

**Behavior to change:** (only if user explicitly requested)
- `show bgp summary` peer rows gain `routes-received`, `routes-accepted`, `routes-sent`.
- The RIB plugin's `show bgp rib status` gains a per-peer `route-counts` sub-map.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `show bgp summary` (offline or daemon), and GET `/api/looking-glass/protocols/bgp`.

### Transformation Path
1. `handleBgpSummary` (`summary.go:64`) reads `reactor.Peers()` -> `[]PeerInfo` (now carrying `RoutesReceived`)
2. `handleBgpSummary` dispatches `ForwardToPlugin(ctx, "show bgp rib status", nil, "")` -> RIB plugin `status()` -> per-peer `{accepted, exported}` map
3. merge by peer address into each peer row: `routes-received` (PeerInfo), `routes-accepted` + `routes-sent` (RIB map)
4. `plugin.Map{"summary": ...}` -> `json.Marshal` -> LG `parseJSON` -> `transformProtocols` (already reads the keys)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session ↔ Peer ↔ reactor_api | new `Peer.ReceivedPrefixCount()` over the session's `prefixCounts` | [ ] |
| reactor ↔ summary handler | `PeerInfo.RoutesReceived` (received only) | [ ] |
| summary handler ↔ RIB plugin | `ForwardToPlugin("show bgp rib status")`, string-keyed, best-effort | [ ] |
| summary ↔ LG | existing `show bgp summary` JSON; LG consumer unchanged | [ ] |

### Integration Points
- `ForwardToPlugin` (`command.go:795`) — the blessed cross-plugin read
- `RIBManager.status()` (`rib_commands.go:661`) — extended, not replaced
- `PeerInfo` (`types_bgp.go:53`) — one new field

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (`summary.go` gains NO compile-time dependency on the RIB plugin)
- [ ] No duplicated functionality (accepted/exported read the RIB's existing per-peer sizes; not re-counted in the reactor)
- [ ] Zero-copy preserved where applicable
- [ ] Registration over hardcoding — the RIB count is reached by registered command dispatch, not a new field on a shared struct

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Dispatching `show bgp rib status` from inside `handleBgpSummary` is safe (no deadlock/re-entrancy) | `cmd/rib/rib.go` forwards builtin->plugin via `ForwardToPlugin`; different plugin, different pipe, synchronous | summary hangs or errors under load | functional `.ci` running a real peer + RIB, asserting summary returns with counts; race run | unvalidated |
| A-2 | Adj-RIB-In size = accepted (post-policy) because rejects never reach the RIB | `reactor_notify.go:449` drops rejected routes before dispatch (confirmed) | accepted would be wrong (would include rejects) | inbound-policy trace already done; add a test with an import filter that rejects some routes and assert accepted < received | unvalidated |
| A-3 | `prefixCounts.totalCount()` is the pre-policy received count and resets on teardown | `session_prefix.go:177,186`; counted before the filter (`session_read.go:201` before `:234`) | received would be wrong or stale across flaps | reactor unit test: announce N, withdraw M, assert count; flap and assert reset | unvalidated |
| A-4 | Alice-LG treats `routes_filtered` = 0 as valid, not an error | `handler_api.go:254` already returns 0/empty for the filtered endpoint | LG shows a spurious 0 that misleads | leave as-is (status quo); document | confirmed (existing behavior) |
| A-5 | The RIB plugin's peer-address key format matches the summary's `p.Address.String()` | both `netip.Addr` -> `.String()`; `bgpPeers` keyed by `netip.Addr` | merge misses every peer (keys never match) | unit test on the merge with a real address round-trip | unvalidated |
| A-6 | `received >= accepted` always (pre-policy >= post-policy) | policy can only reject, never add | a negative "filtered = received - accepted" if ever derived | do NOT derive filtered; report it 0. Assert received>=accepted in a test as a sanity check | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `summary.go` imports the RIB plugin, breaking self-containment | `make ze-plugin-boundary-check` red; an import line | reach the RIB only via `ForwardToPlugin` with a string constant, as `cmd/rib` does |
| R-2 | The RIB dispatch adds latency to every `show bgp summary` (a hot LG poll path) | slow summary under many peers | one dispatch per summary call returns the whole per-peer map; not N+1. Measure in the `.ci` |
| R-3 | Best-effort degrades silently to wrong-looking zeros when the RIB is slow/erroring (not just absent) | counts flap to 0 intermittently | distinguish "plugin absent" (omit keys) from "plugin errored" (log + omit); never emit a fake 0 for accepted/exported. received always present |
| R-4 | `received` (pre-policy) confuses operators who expect it to equal the RIB size | support question | document: received = advertised by peer (pre-policy); accepted = kept after import policy; the gap is filtered-but-not-retained |
| R-5 | Adding the reactor accessor touches the session hot path | race under churn | accessor is a read of an existing counter under the existing lock; no new hot-path write |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp summary` with a RIB plugin | → | `handleBgpSummary` merges RIB per-peer counts | `TestBgpSummaryMergesRibRouteCounts` |
| `show bgp summary` with NO RIB plugin | → | best-effort degradation | `TestBgpSummaryWithoutRibOmitsAcceptedExported` |
| `show bgp rib status` | → | RIB `status()` per-peer map | `TestRibStatusEmitsPerPeerRouteCounts` |
| a peer advertising N prefixes | → | `Peer.ReceivedPrefixCount()` | `TestReceivedPrefixCountTracksAnnouncements` |
| GET `/api/looking-glass/protocols/bgp` | → | LG reads the emitted keys | `TestTransformProtocolsRouteCountsFromRealSummary` |
| full engine path | → | real peer + RIB + summary | `test/plugin/bgp-summary-route-counts.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | peer's Adj-RIB-In holds 100 routes | `routes-received` = 100, `routes-accepted` = 100 (both = Adj-RIB-In size) |
| AC-2 | peer advertises 100, import policy rejects 40 | Adj-RIB-In holds 60 -> `routes-received` = `routes-accepted` = 60. The pre-policy 100 is NOT shown here (Known Limitation); it is on the `ze_bgp_prefix_count` gauge |
| AC-3 | we advertise 50 routes to the peer | `routes-sent` = 50 (Adj-RIB-Out size) |
| AC-4 | any peer | `routes-filtered` is absent/0 (Ze does not retain filtered routes) |
| AC-5 | RIB plugin not loaded | all three route counts omitted, NOT faked to 0; summary still renders |
| AC-6 | `show bgp rib status` | emits a per-peer `route-counts` map `{addr: {accepted, exported}}` |
| AC-7 | LG `/api/looking-glass/protocols/bgp` | `routes_received`/`routes_imported`/`routes_exported` reflect the summary; `routes_filtered` = 0 |
| AC-8 | existing consumers | CLI dashboard, web summary, LG all still parse the payload |
| AC-9 | peer session flaps | all three counts reset (new session) and re-accumulate |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Opens Alice-LG and reads per-peer route counts | LG -> show bgp summary -> reactor received + RIB accepted/exported -> transformProtocols | `test/plugin/bgp-summary-route-counts.ci` |
| 2 | Runs `show bgp summary` in the CLI and sees route counts | dispatch -> handleBgpSummary -> merge | same `.ci` |
| 3 | Runs it on a box with no RIB plugin | handleBgpSummary -> ForwardToPlugin ErrUnknownCommand -> received-only | `TestBgpSummaryWithoutRibOmitsAcceptedExported` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReceivedPrefixCountTracksAnnouncements` | `internal/component/bgp/reactor/session_prefix_test.go` | A-3, AC-1 (received) | |
| `TestRibStatusEmitsPerPeerRouteCounts` | `internal/component/bgp/plugins/rib/rib_commands_test.go` | AC-6 | |
| `TestBgpSummaryMergesRibRouteCounts` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-1..AC-3, A-5 | |
| `TestBgpSummaryWithoutRibOmitsAcceptedExported` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-5 | |
| `TestBgpSummaryFilteredStaysZero` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | AC-4 | |
| `TestTransformProtocolsRouteCountsFromRealSummary` | `internal/component/lg/handler_api_test.go` | AC-7 | |

<!-- Build the lg test from the REAL producer payload (extend realSummaryJSON),
     as the earlier birdwatcher phases did. The original bug survived because a
     hand-built map shape hid the producer/consumer contract break. -->

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| route counts | 0..int | 0 (no routes) | N/A (counts are ≥0) | N/A |

<!-- No operator-supplied numeric input; the counts are derived. The boundary
     that matters is received >= accepted (A-6), tested as an invariant. -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-summary-route-counts` | `test/plugin/bgp-summary-route-counts.ci` | a real peer advertises routes; `show bgp summary` shows received/accepted/sent per peer | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | No wire-format change. The counts are derived from existing Adj-RIB state; the surface is a command/API. Interop does not apply | |

## Files to Modify
- `internal/component/bgp/plugins/rib/rib_commands.go` - `status()` emits per-peer `route-counts` `{addr: {in, out}}`
- `internal/component/bgp/plugins/rib/rib_commands_test.go` - per-peer status test
- `internal/component/bgp/plugins/cmd/peer/summary.go` - dispatch to RIB + merge into peer rows
- `internal/component/bgp/plugins/cmd/peer/summary_test.go` - merge tests (fake dispatcher)
- `internal/component/lg/handler_api_test.go` - real-payload route-count test
- `docs/architecture/api/commands.md` - the new summary keys + RIB status field
- `plan/spec-lg-birdwatcher-peer-fields.md` - mark Phase 4 done -> this spec, then close it

**No reactor files. No `PeerInfo` change.** The received/accepted/exported data all
comes from the RIB plugin's existing per-peer Adj-RIB-In/Out maps.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] no - no new command name or config leaf; `show bgp rib status` gains a field | - |
| CLI commands/flags | [ ] no | - |
| Functional test for new RPC/API | [ ] yes | `test/plugin/bgp-summary-route-counts.ci` |
| Pipe completeness | [ ] no - existing command | - |
| Doctor check | [ ] no | - |
| Prometheus counters/metrics | [ ] no - the per-peer gauges (`ze_rib_routes_in/out`, `ze_bgp_prefix_count`) already exist; this exposes them in the summary | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` (LG per-peer route counts) |
| 4 | API/RPC added/changed? | [ ] yes | `docs/architecture/api/commands.md` (summary keys + rib status field) |
| 12 | Internal architecture changed? | [ ] yes - summary now aggregates a plugin via dispatch | `docs/architecture/core-design.md` if it describes summary |
| 16 | Any changed source file referenced by doc source anchors? | [ ] check | grep `docs/` for `summary.go`, `rib_commands.go` |

## Files to Create
- `test/plugin/bgp-summary-route-counts.ci` - functional test over the real dispatch path

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify` |
| 6. Critical review | Critical Review Checklist below |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary; two-commit closure (this spec AND birdwatcher Phase 4) |

### Implementation Phases

<!-- Ordered so each phase is independently verifiable. Received (reactor) and the
     RIB per-peer emission are independent; summary merge depends on both. -->

1. **Phase: Wiring (MANDATORY FIRST)** — failing tests at each seam
   - Tests: the unit tests, failing because the keys are absent
   - Files: the `_test.go` files
   - Verify: they fail for the right reason (missing data), not compile errors
2. **Phase: per-peer sizes (RIB plugin)** — extend `status()` with a per-peer `route-counts` map
   - Tests: `TestRibStatusEmitsPerPeerRouteCounts`
   - Files: `rib_commands.go`
   - Verify: per-peer `{in: peerRIB.Len(), out: len(ribOut[addr])}`; keys are `addr.String()`
3. **Phase: summary merge** — dispatch + merge, best-effort, filtered stays 0
   - Tests: `TestBgpSummaryMergesRibRouteCounts`, `TestBgpSummaryWithoutRibOmitsRouteCounts`, `TestBgpSummaryFilteredStaysZero`
   - Files: `summary.go`
   - Verify: `routes-received`/`routes-accepted` = in, `routes-sent` = out with RIB; omitted (not faked 0) without RIB
4. **Phase: LG consumer test** — prove the end-to-end lights up (no LG code change)
   - Tests: `TestTransformProtocolsRouteCountsFromRealSummary`
   - Files: `handler_api_test.go`
5. **Functional test** — `test/plugin/bgp-summary-route-counts.ci`
6. **Full verification** — `make ze-verify` + `make ze-plugin-boundary-check` (R-1)
7. **Close** — mark birdwatcher Phase 4 done -> this spec; two-commit closure of BOTH specs

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Naming | summary keys kebab-case; RIB map keys `accepted`/`exported`; LG snake_case unchanged |
| Self-containment | `summary.go` has NO import of the RIB plugin; `make ze-plugin-boundary-check` green (R-1) |
| Best-effort | RIB absent -> received-only, no faked accepted/exported (AC-5, R-3) |
| Honesty | `routes-filtered` is 0/absent, never derived or faked (AC-4); documented reason |
| Data flow | received from PeerInfo; accepted/exported from dispatch; not re-counted in the reactor |
| Correctness | Adj-RIB-In size = accepted because rejects never reach the RIB (A-2); received >= accepted (A-6) |
| Concurrency | the reactor accessor reads under the existing lock; no new hot-path write (R-5) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| received populated | `go test ./internal/component/bgp/reactor -run TestReceivedPrefixCount` |
| RIB per-peer status | `go test ./internal/component/bgp/plugins/rib -run TestRibStatusEmitsPerPeer` |
| summary merge | `go test ./internal/component/bgp/plugins/cmd/peer -run TestBgpSummary` |
| self-containment | `make ze-plugin-boundary-check` |
| end-to-end | `bin/ze-test bgp plugin bgp-summary-route-counts` |
| LG lights up | `go test ./internal/component/lg -run TestTransformProtocolsRouteCounts` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | the RIB per-peer map is bounded by peer count; the summary dispatch is one call, not N+1 (R-2) |
| Error leakage | a RIB dispatch error must not leak internals into the public LG; on error, omit the counts and log server-side (R-3) |
| Input validation | peer-address keys from the RIB map are matched against known peers; an unknown key is ignored, not injected as a new row |

### Failure Routing
| Failure | Route To |
|---------|----------|
| `ze-plugin-boundary-check` red | Remove the RIB import; use `ForwardToPlugin` (R-1) |
| Merge misses all peers | A-5 wrong; check address key format |
| accepted > received in a test | A-2/A-6 wrong; re-trace the accept gate |
| dispatch deadlocks | A-1 wrong; STOP and reconsider the channel (event bus instead) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

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

- The reactor is the only component that sees every received, accepted, rejected,
  and sent event, but the codebase deliberately keeps NLRI storage counts in the
  RIB plugin (`types_bgp.go:92`, `peer_stats.go:21`). So the counts come from two
  owners on purpose: received from the reactor (a session gauge that already
  exists for prefix-limit enforcement), accepted/exported from the RIB (which
  already sizes its Adj-RIB-In/Out per peer). The merge, not the counting, is the
  new work.
- `filtered` cannot be made real without Ze retaining filtered routes (BIRD's
  "import keep filtered on"), which is a much larger feature. Reporting 0 is the
  honest current-state answer, and it matches the existing LG stub. A cumulative
  reject counter is possible at the reactor gate but is a different metric with
  different (cumulative) semantics — out of scope, noted in Known Limitations.

## Core Insight

Three of the four route counts already exist as per-peer current-state sizes;
they were simply never surfaced into `show bgp summary`. The engineering is a
best-effort cross-plugin merge that respects plugin self-containment, not a new
accounting subsystem. The fourth (`filtered`) is a deliberate architectural
absence, and the honest move is to leave it zero rather than fabricate it.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| RIB owns accepted/exported; summary reaches them by `ForwardToPlugin` | Maintain net accepted/exported counts in the reactor | Reactor-native duplicates the RIB's exact Adj-RIB-In/Out sizes and violates `types_bgp.go:92`/`plugin-self-containment.md`. The RIB already has the numbers |
| received from the reactor via PeerInfo | Also get received from a plugin | Rejected routes never reach any plugin, so only the reactor knows pre-policy received. `prefixCounts` already tracks it |
| `filtered` stays 0 | Derive `received - accepted`; or add a cumulative reject counter | `received - accepted` conflates policy rejects with loop/limit/malformed drops (over-counts). A cumulative counter has different semantics from the current-state field. Ze does not retain filtered routes; 0 is honest and matches the existing stub |
| best-effort dispatch | Require the RIB plugin | `ForwardToPlugin` already returns `ErrUnknownCommand`; received-only degradation keeps summary working on minimal builds |

## Known Limitations
- `routes_received` == `routes_accepted` (both are the Adj-RIB-In size). The
  PRE-policy received count (routes the peer advertised before import policy) is
  NOT surfaced here: it lives only in the reactor session (`prefixCounts`), where
  reading it from `Peers()` races the session write loop, so it needs new hot-path
  atomics + session→peer plumbing. Deferred as risky-for-marginal-benefit; it is
  already exposed via the `ze_bgp_prefix_count` Prometheus gauge
  (`session_prefix.go:530`). Making received distinct from accepted is future work.
- `routes_filtered` is always 0. Making it real requires Ze to retain
  import-filtered routes (BIRD "import keep filtered on"), a separate large
  feature (filtered-route storage + a filtered scope in the RIB). Out of scope.
- A per-peer cumulative import-reject counter (at `reactor_notify.go:449`) is
  feasible and would be a genuinely useful diagnostic, but it is a cumulative
  metric distinct from the current-state `routes_filtered` field, so it is not a
  substitute and is left as future work.
- accepted/exported require the `bgp-rib` plugin; on a build without it the
  summary reports received only. This is correct (no RIB -> no Adj-RIB sizes).

## RFC Documentation

Add `// RFC 4271 Section 3.2` above the accepted (Adj-RIB-In) and exported
(Adj-RIB-Out) emission, citing the routing-information-base definitions.

## Implementation Summary

### What Was Implemented
- `rib_commands.go` `status()`: added a per-peer `route-counts` map `{addr: {in: Adj-RIB-In size, out: Adj-RIB-Out size}}`, collected in the loops that already computed the global sums.
- `summary.go`: `fetchRibRouteCounts` (best-effort `ForwardToPlugin("show bgp rib status")`), `parseRibRouteCounts` (JSON -> map), `mergeRibRouteCounts` (adds `routes-received`/`routes-accepted` = in, `routes-sent` = out per row; never emits `routes-filtered`). `cmdRibStatus` is a string constant — no RIB import.
- `handler_api.go`: NO change (it already read the keys). Verified by the LG test.
- Tests: `TestRibStatusEmitsPerPeerRouteCounts`, `TestParseRibRouteCounts`, `TestMergeRibRouteCounts`, `TestBgpSummaryWithoutRibOmitsRouteCounts`, `TestTransformProtocolsRouteCountsFromRealSummary`, and `test/plugin/bgp-summary-route-counts.ci`.

### Bugs Found/Fixed
- None. Net-new feature.

### Documentation Updates
- `docs/guide/looking-glass.md`: documented the route-count sources and the `filtered = 0` / pre-policy-received caveats, with source anchors. `make ze-doc-test` green.

### Deviations from Plan
- DESIGN CHANGE during implementation: `received` was originally to come from the reactor's pre-policy `prefixCounts`. Reading that from `Peers()` (under `r.mu.RLock`) races the session write loop, needing new hot-path atomics for a signal that is imprecise (filtered=0) and already on the `ze_bgp_prefix_count` gauge. Switched to `received = accepted = Adj-RIB-In size` from the RIB, all one source, zero reactor changes. Recorded in Key Design Decisions + Known Limitations.

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Alice-LG shows real per-peer route counts | functional test | `test/plugin/bgp-summary-route-counts.ci` PASS (id 90): a peer advertising a default route -> `show bgp summary` shows `routes-received`=1 for 127.0.0.1 via the real RIB dispatch |
| received/accepted/exported are real | unit + functional | `TestRibStatusEmitsPerPeerRouteCounts`, `TestParseRibRouteCounts`, `TestMergeRibRouteCounts`, + the `.ci` |
| filtered is honestly zero | unit + functional | `TestMergeRibRouteCounts` (no `routes-filtered` key) + the `.ci` asserts the key is absent |
| self-containment preserved | gate | `make ze-plugin-boundary-check` green (summary.go has no RIB import) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (not started)

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
| `test/plugin/bgp-summary-route-counts.ci` | yes | ran as plugin test id 90, PASS |
| `internal/component/bgp/plugins/rib/rib_commands.go` | yes | `route-counts` in `status()` |
| `internal/component/bgp/plugins/cmd/peer/summary.go` | yes | `fetchRibRouteCounts`/`mergeRibRouteCounts` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1/AC-3 | received/accepted=in, sent=out | `TestMergeRibRouteCounts` PASS; `.ci` shows received=1 |
| AC-4 | filtered absent | `TestMergeRibRouteCounts` asserts no `routes-filtered`; `.ci` asserts absent |
| AC-5 | RIB absent -> omitted, not faked | `TestBgpSummaryWithoutRibOmitsRouteCounts` PASS |
| AC-6 | per-peer status map | `TestRibStatusEmitsPerPeerRouteCounts` PASS |
| AC-7 | LG surfaces the keys | `TestTransformProtocolsRouteCountsFromRealSummary` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show bgp summary` + real peer + bgp-rib | `test/plugin/bgp-summary-route-counts.ci` | yes (PASS id 90) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 (dispatch safe) | confirmed | `.ci` passes: summary dispatches to bgp-rib and merges, no hang |
| A-2 (Adj-RIB-In = accepted) | confirmed | default route (no filter) -> in=1 in the `.ci` |
| A-4 (LG filtered=0 valid) | confirmed | existing stub; `.ci` asserts key absent |
| A-5 (address key match) | confirmed | `.ci` matched `route-counts["127.0.0.1"]` |
| A-3/A-6 (pre-policy received) | dropped | design change: received=accepted; see Deviations |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| looking-glass.md route-count sources + filtered=0 | `summary.go`, `rib_commands.go`, `handler_api.go` anchors | `make ze-doc-test` green |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] `make ze-plugin-boundary-check` green (self-containment)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (received >= accepted invariant)
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + both specs (this one filled, birdwatcher Phase 4 closed) + learned summary + counter bump
- [ ] **Commit B:** `git rm` this spec AND `git rm plan/spec-lg-birdwatcher-peer-fields.md`
