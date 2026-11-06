# Spec: bmp-locrib-dump-end-of-rib-before-routes

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | A BMP collector reads End-of-RIB as "the dump is complete", and Ze sends it before the routes |
| RFC gaps | rfc7854 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-06 |

Scope is protocol: the wire change is the order of BMP messages to a collector.
The End-to-End section applies; no config or CLI surface changes.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

With `loc-rib true`, a collector receives the End-of-RIB markers of both families
before the Route Monitoring of the dump, and the dump's routes reach every
connected collector rather than the one that asked.

The cause is one wrong assumption. `emitReplayRequest` treats the replay request
as synchronous: it publishes a scope, emits, retracts the scope, and sweeps the
families no batch closed. `bgp-rib` takes the request as a plugin subscription,
so `Server.deliverEvent` calls `Process.Deliver`, which only queues. The emit
returns before the RIB has read the request. The sweep then closes both families,
and the batches arrive after the scope is gone. They match no scope, so they fan
out to every collector and close nothing.

The BMP unit tests do not see this, because their stand-in RIB subscribes on the
engine side and answers inside the emit. RFC 7854 Section 5: "When the initial
dump is completed for a given peer, this MUST be indicated by sending an
End-of-RIB marker for that peer".

Goal: a collector gets its End-of-RIB markers only after the RIB has finished its
dump, gets only the dump it asked for, and is not left waiting forever if the RIB
dies.

`docs/guide/bmp.md` was corrected to say what happens today, in the same piece of
work that wrote this spec. It is corrected back when this spec lands.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` "Redistribute Late-Join Replay" - the shared replay vocabulary
  → Decision: a targeted hop uses an opaque token that the emitter maps back to one consumer. The bgp-rib hop is named broadcast there, and BMP already puts a targeted token on it.
  → Constraint: `internal/core/replay` stays a leaf that spells no protocol concept.
- [ ] `docs/architecture/plugin/plugin-system.md` "The process boundary", "Communication patterns"
  → Constraint: the fix must hold when `bgp-rib` runs forked. The EventBus is the one-to-many route, and nothing may rely on delivery running inside the emit.
- [ ] `docs/plugin-development/protocol.md`, `docs/architecture/api/process-protocol.md`
  → Constraint: a new event type and a new JSON member are a published contract, changed on purpose, with the page.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7854.md` - the initial dump and its End-of-RIB
  → Constraint: RFC 7854 Section 5: "When the initial dump is completed for a given peer, this MUST be indicated by sending an End-of-RIB marker for that peer".
- [ ] `rfc/short/rfc9069.md` - Loc-RIB Route Monitoring and Peer Up
  → Constraint: the Peer Up precedes the first Route Monitoring, an End-of-RIB marker included.
- [ ] `rfc/short/rfc4724.md` - the marker's meaning
  → Constraint: RFC 4724 Section 4: the marker "MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family". An empty family still gets its marker, after the dump.

**Key insights:**
- The RIB emits no batch for an empty family, so only an explicit completion signal can close it. A count of batches cannot.
- A forked RIB cuts a family into chunks, so "one batch is the whole family" holds only in-process.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/bmp/bmp_locrib.go` - `emitReplayRequest` publishes `dumpScope`, emits, retracts, then runs `closeDumpFamilies`. `handleBestChange` claims a batch only while the scope with its token is published, and sends End-of-RIB per claimed batch. `closeDumpFamilies` sends the markers no batch sent. Its own comment says the sweep "can run before the batches arrive"
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `AddStartupSubscriptionsIn(ribevents.Namespace, ...)` takes `EventReplayRequest` as a plugin subscription, in-process and forked alike
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `replayBestPaths` runs `replayFlowSpecs`, then emits one batch per non-empty family and stamps `req.ReplayID`. It emits nothing else, and returns silently when the bus is nil
- [ ] `internal/component/bgp/plugins/rib/rib_event_sdk.go` - `sdkEventBus.Emit` cuts an oversized batch into ordered chunks under one `emitMu`. `dispatchReplayRequest` decodes the request and calls `replayBestPaths`
- [ ] `internal/core/bgp/ribevents/ribevents.go` - `BestChangeBatch.ReplayID` is `json:"-"`. `MarshalJSON` writes only the `replay` boolean, and `UnmarshalJSON` turns a true one into `replay.Broadcast`, so the token does not survive
- [ ] `internal/core/replay/replay.go` - `Request`, `Broadcast`, `IsReplay`
- [ ] `internal/component/plugin/process/delivery.go` - `Process.Deliver` queues and returns
- [ ] `internal/core/events/typed.go` - `Event.Emit` returns the number of plugin-process subscribers delivered; engine subscribers run synchronously
- [ ] `internal/plugins/redistribute/replay.go` - the targeted hop's coordinator: token to target, `replayTTL`, hard cap, and the `removed` map that stops a replay reviving a route removed meanwhile
- [ ] `internal/component/bgp/plugins/bmp/bmp_reconnect_test.go`, `rfc9069_bmp_locrib_test.go`, `rfc9069_verdict_test.go` - each stand-in RIB subscribes `ribevents.ReplayRequest` with `Subscribe` and answers inside the emit
- [ ] `test/plugin/bmp-locrib.ci` - the real-RIB path; asserts a valid Route Monitoring and exit status, and no order against End-of-RIB

**Behavior to preserve:**
- Every family in `dumpFamilies` gets a marker, also when the table holds no route for it.
- A replay another subsystem requested (sysrib, flowspec-firewall, both `replay.Broadcast`) still reaches every collector as Route Monitoring and closes no family.
- The `replay` boolean stays on the JSON of `BestChangeBatch`, for an external reader.
- `handleBestChange` never blocks on I/O: it writes into each session's bounded queue.
- Peer Up precedes every Route Monitoring, on each session.

**Behavior to change:**
- A family closes when the RIB says the dump finished, never when the emit returns.
- A claimed batch reaches only the scope's collector.
- A dump the RIB never finishes ends the collector session, with no End-of-RIB.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Replay correlation token | `internal/core/replay` `Request`; `ribevents.BestChangeBatch.ReplayID` | One opaque token on every replay hop, echoed on the batch | Extends: the completion event carries the same token, in the same `Request` shape |
| Typed events | `internal/core/events` `Register`; `ribevents` handles | Declares a namespace, type and payload, reachable in-process and across the boundary | Extends: `ribevents.ReplayComplete` is one more handle. No new channel between plugins |
| Forked RIB bus | `rib_event_sdk.go` `sdkEventBus` | Hands events to the engine in order, under `emitMu` | Extends unchanged: the completion follows the last chunk on the same bus, so order holds |
| Plugin startup subscriptions | `rib.go` `AddStartupSubscriptionsIn` | What the RIB hears, on both transports | Does not apply as a change: the RIB already hears the request. The reader of completion is BMP, on the daemon bus |
| Dump scope and sweep | `bmp_locrib.go` `dumpScope`, `emitReplayRequest`, `closeDumpFamilies` | Publishes one scope for the length of the emit, then sweeps | Replaces the window: scopes live in a table keyed by token until completion or deadline. `closeDumpFamilies` stays and moves to the completion handler. The per-batch marker in `handleBestChange` goes: chunks made it unsound |
| Token table with a bound | `internal/plugins/redistribute/replay.go` `replayCoordinator` (`replayTTL`, hard cap) | Token to target, evicted by time and cap, for a late answer | Extends the design, not the code: BMP's table has the same shape. The code stays separate because that coordinator holds peer and consumer targets, and BMP needs a session |
| Session reset on failure | `docs/guide/bmp.md` (storage failure "closes collector sessions without sending a misleading End-of-RIB"); `sender.go` queue overflow reset | A failure closes the session rather than lying to the collector | Extends: the dump deadline takes the same exit, so the collector reconnects and asks again |
| Test stand-in RIB | `bmp_reconnect_test.go` and two others | Answers inside the emit | Replaces: a queued stand-in, which is what `bgp-rib` is on both transports |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A collector session comes up, or `loc-rib` turns on: `requestLocRIBDump`, `startLocRIB`.
- Format: a `*replay.Request` on (bgp-rib, replay-request), with a token BMP allocated.

### Transformation Path
1. BMP records a scope under the token (session, deadline), then emits the request.
2. The request queues for `bgp-rib` through `Process.Deliver`.
3. `replayBestPaths` emits its batches with the token, then emits (bgp-rib, replay-complete) with the same token, also when it emitted no batch.
4. `handleBestChange` finds the scope by token and writes Route Monitoring only to that scope's sessions. It restarts the scope's deadline.
5. The completion handler removes the scope and runs `closeDumpFamilies` for every family in `dumpFamilies`, after the last Route Monitoring.
6. A deadline with no batch and no completion removes the scope and resets its sessions, with no End-of-RIB.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BMP to RIB | (bgp-rib, replay-request) event, `replay.Request` JSON | Yes: `dispatchReplayRequest` reads the token on both transports |
| RIB to BMP, batches | (bgp-rib, best-change); typed in-process, JSON when forked | Yes: `BestChangeBatch.MarshalJSON` drops the token, which this spec fixes |
| RIB to BMP, completion | (bgp-rib, replay-complete), new | No: proven by the tests below |
| BMP to collector | BMP Route Monitoring, in session queue order | Yes: the session's bounded queue keeps order |

### Changed-Wire Census

**Search evidence:** scope `internal`, `pkg`, `test`, `cmd`, `docs/architecture/api`. Terms: `"replay"`, `replay-id`, `BestChange`, `best-change`, `EventReplayRequest`. Control: `BestChangeBatch.MarshalJSON` is found as the writer, and the `ribevents.BestChange.Subscribe` sites in sysrib, fib, flowexport and BMP as readers. No Python or shell reader decodes `best-change`. The `meta["replay"]` hits in `rib_replay.go` and `update_text.go` are route metadata, another wire.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `best-change` JSON | writer | `ribevents.go` `BestChangeBatch.MarshalJSON` | writes `replay` from the token | Change: adds `replay-id` beside `replay` |
| `best-change` JSON | reader | `ribevents.go` `BestChangeBatch.UnmarshalJSON` | a true `replay` becomes `replay.Broadcast` | Change: reads `replay-id`; without it, a true `replay` stays `replay.Broadcast` for an external writer |
| `best-change` typed | reader | sysrib, `fibkernel`, `fibp4`, `fibvpp`, `flowexport`, `redistribute/producer.go` | read `IsReplay` or the batch, none compares the token | Preserve: `IsReplay` answers the same |
| `replay-complete` | writer | `rib_bestchange.go` `replayBestPaths` | new | New: `{"replay-id":N}` after the last batch |
| `replay-complete` | reader | `bmp_locrib.go`, new handler | new | New: an unknown token is ignored, because sysrib's own replay also completes |
| BMP messages to a collector | writer | `handleBestChange`, `closeDumpFamilies` | order changes, bytes do not | Change of order, proven against pmacct |

### Integration Points
- `replayBestPaths` - the one place that knows the walk ended; emits the completion.
- `closeDumpFamilies` - moves from the emit's return to the completion handler, unchanged in what it sends.
- `sender.go` session reset - the deadline's exit.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Request and completion both cross as events |
| No unintended coupling (components stay isolated) | Yes | `bgp-rib` does not know BMP; it answers a token for any requester |
| No duplicated functionality (extends existing, does not recreate) | Yes | See Existing Mechanisms; the scope window is replaced, not kept beside the table |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Cold path; a replay already builds its batches |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `events.Register` in `ribevents`, which already owns this namespace |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `EventReplayRequest` and `replay-request`: the startup subscription list names only what the RIB hears, and BMP's reader is an engine `Subscribe`. The event type registry fills from `Register` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The forked RIB's completion reaches an engine subscriber after its last chunk | `sdkEventBus.Emit` holds `emitMu` and hands each frame in order | A marker overtakes a chunk | `TestDumpForkedRIBChunksPrecedeCompletion` | unvalidated |
| A-2 | BMP always runs in the daemon process, with the daemon bus | `handleBestChange` comment says "BMP is in-process"; `bmp_locrib.go` calls `Subscribe` | A forked BMP hears no batch at all, which is a wider defect | `./le mock plugin-external` for `bgp-bmp`, read at implementation | unvalidated |
| A-3 | With no RIB process subscribed, a dump has nothing to wait for | `Event.Emit` returns 0 plugin-process subscribers | A dump closes early when a subscriber is only late | `TestDumpWithNoRIBClosesAtOnce`, and the startup order read at `startLocRIB` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A deadline shorter than a slow dump resets a healthy collector, which reconnects and loops | A collector reset log during a large table | The deadline restarts on every claimed batch, and the reconnect interval bounds the loop |
| R-2 | Two collectors each ask, so the RIB walks the table twice | Load on reconnect storms | Accepted: dumps stay per-token. The RIB already walks once per request |
| R-3 | A batch from an expired token fans out | A warning on an unknown non-broadcast token | Dropped with a warning: the session it was for is already reset |
| R-4 | A replay batch arrives after an incremental withdraw of a route it snapshotted, so the collector keeps a removed route | A route on the collector that `ze show bgp rib best` does not hold | Read the RIB's emit order at implementation. If the two can cross, take the `removed` shape from `replayCoordinator` and add a test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A collector sees a missing or late End-of-RIB, or is reset. BGP sessions and routes are not touched |
| How is it reverted? | Single commit revert; the new event and JSON member have no persistent state |
| Who else touches this path? | sysrib and the FIB backends read `best-change`. `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` edits other BMP paths |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Collector connects with `loc-rib true` | → | `requestLocRIBDump`, `replayBestPaths`, completion handler, `closeDumpFamilies` | `test/plugin/bmp-locrib.ci`, extended to assert order through `BMP-COLLECTOR` |
| Same, against a real queued RIB | → | `emitReplayRequest` returns before any batch | `TestDumpMarkersFollowRoutesWithQueuedRIB` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A collector connects to a router holding IPv4 and IPv6 best paths, through the real `bgp-rib` | It receives Peer Up, then every Route Monitoring of the dump, then End-of-RIB for IPv4 and IPv6 unicast | Incremental Route Monitoring may interleave: RFC 7854 Section 5 says "There is no requirement on the ordering of messages in the peer dumps" |
| AC-2 | The Loc-RIB holds IPv6 routes and no IPv4 route | Both markers arrive, each after the last IPv6 Route Monitoring | None: the RFC owes a marker for the empty family, and only completion can place it |
| AC-3 | The Loc-RIB is empty | Peer Up, then both markers, after the RIB's completion | None |
| AC-4 | Two collectors connect together | Each receives its own complete dump and its own markers, and neither receives the other's dump | Both still receive incremental changes |
| AC-5 | sysrib requests a replay during a BMP dump | Every collector receives those routes as Route Monitoring. No family closes because of it, and the BMP dump still closes on its own completion | Those routes are real, so they stay visible to every collector |
| AC-6 | `bgp-rib` runs forked and a family is cut into chunks | All chunks reach the scope's collector only, and the marker follows the last chunk | None |
| AC-7 | The RIB dies, or never answers, during a dump | After the deadline the collector session resets with a logged reason, and no End-of-RIB was sent for that dump | The collector's reconnect then requests a fresh dump, as for any reset |
| AC-8 | A slow dump keeps producing batches past one deadline length | The dump finishes and closes normally | None: the deadline measures silence, not length |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator points pmacct at a router with `loc-rib true` | collector connect -> request -> RIB queue -> batches -> completion -> markers | Interop `bmp-locrib-pmacct`, order asserted |
| 2 | A collector restarts while routes exist | session up -> dump for this session only | `test/plugin/bmp-locrib.ci`, reconnect step |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDumpMarkersFollowRoutesWithQueuedRIB` | `internal/component/bgp/plugins/bmp/bmp_locrib_dump_test.go` | AC-1, with a stand-in that answers after the emit returned | |
| `TestDumpEmptyFamilyClosedOnCompletion` | same | AC-2, AC-3 | |
| `TestDumpScopeNarrowsToItsCollector` | same | AC-4 | |
| `TestForeignReplayClosesNoFamilyDuringDump` | same | AC-5 | |
| `TestDumpDeadlineResetsSessionWithoutMarker` | same | AC-7 | |
| `TestDumpDeadlineRestartsOnBatch` | same | AC-8 | |
| `TestDumpWithNoRIBClosesAtOnce` | same | A-3 | |
| `TestReplayBestPathsEmitsCompletionAfterBatches` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | Completion follows the last batch, also with an empty table | |
| `TestDumpForkedRIBChunksPrecedeCompletion` | `internal/component/bgp/plugins/rib/rib_event_sdk_test.go` | AC-6 | |
| `TestBestChangeBatchTokenSurvivesJSON` | `internal/core/bgp/ribevents/ribevents_test.go` | Round trip of `replay-id`; a true `replay` without it stays `replay.Broadcast` | |

The stand-ins in `bmp_reconnect_test.go`, `rfc9069_bmp_locrib_test.go` and
`rfc9069_verdict_test.go` become queued. An RFC-tagged test among them whose
claim or proof changes owes a fresh `./le rfc record`, and any change that could
lower what it proves goes to `ze-test-judge`.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Dump token | 1 to MaxUint64-1 | MaxUint64-1 | 0 (incremental) | MaxUint64 (`replay.Broadcast`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bmp-locrib` | `test/plugin/bmp-locrib.ci` | A collector sees every route before the markers, through the real RIB | |
| `bmp-locrib-forked-rib` | `test/plugin/bmp-locrib-forked-rib.ci` | The same with `bgp-rib` forked | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bmp-locrib-pmacct` | `test/interop/scenarios/` | pmacct | The collector's decoded output holds the routes before its End-of-RIB record. Read pmacct's source at the lab's version before asserting what it prints for End-of-RIB | |

## Files to Modify
- `internal/component/bgp/plugins/bmp/bmp_locrib.go` - scope table, completion handler, deadline, narrowing; the window in `emitReplayRequest` and the per-batch marker go
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - completion in `replayBestPaths`, on every exit that answered a request
- `internal/core/bgp/ribevents/ribevents.go` - `ReplayComplete` handle, the event name, the `replay-id` JSON member
- `internal/component/bgp/plugins/bmp/bmp_reconnect_test.go`, `rfc9069_bmp_locrib_test.go`, `rfc9069_verdict_test.go` - queued stand-in
- `test/plugin/bmp-locrib.ci` - order assertion
- `docs/guide/bmp.md` - the Loc-RIB paragraphs say the corrected order
- `docs/architecture/api/process-protocol.md` - the new event and member
- `docs/architecture/core-design.md` - the bgp-rib hop is targeted for BMP

## Files to Create
- `internal/component/bgp/plugins/bmp/bmp_locrib_dump_test.go` - the tests above
- `test/plugin/bmp-locrib-forked-rib.ci` - the forked path

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config or RPC |
| YANG validation constraints | No | No leaf |
| YANG custom validators | No | No leaf |
| CLI commands/flags | No | None |
| CLI grammar (keyword before value) | No | None |
| Editor autocomplete | No | None |
| Functional test for new RPC/API | Yes | `test/plugin/bmp-locrib.ci` |
| Pipe completeness | No | None |
| Env var registration | No | The deadline is a constant, as `replayTTL` is |
| Doctor check for runtime dependencies | No | No new dependency |
| Prometheus counters/metrics | No | The reset logs its reason |
| BGP family surface (new SAFI / capability / attribute) | No | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Fix of an existing feature |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md`: the event |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md`: replace the limit written on 2026-11-06 with the corrected order |
| 7 | Wire format changed? | No | BMP bytes do not change |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7854.md`, `rfc/short/rfc9069.md` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, the replay section |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md` if it lists event types |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/plugin/rib-storage-design.md`: its replay text gains the completion event. Others: `./le spec citation anchors spec plan/release/fixes/spec-bmp-locrib-dump-end-of-rib-before-routes.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the handle and the emit
   - Tests: `TestReplayBestPathsEmitsCompletionAfterBatches`, `TestBestChangeBatchTokenSurvivesJSON`
   - Files: `ribevents.go`, `rib_bestchange.go`
   - Verify: the completion is observable on the bus, and BMP does not read it yet
2. **Phase: Scope table and close** -- BMP waits for completion
   - Tests: `TestDumpMarkersFollowRoutesWithQueuedRIB`, `TestDumpEmptyFamilyClosedOnCompletion`, `TestDumpScopeNarrowsToItsCollector`, `TestForeignReplayClosesNoFamilyDuringDump`
   - Files: `bmp_locrib.go`, the test stand-ins. Delete the emit window first (`ai/rules/no-layering.md`)
   - Verify: tests fail, implement, tests pass
3. **Phase: Deadline** -- a RIB that never answers
   - Tests: `TestDumpDeadlineResetsSessionWithoutMarker`, `TestDumpDeadlineRestartsOnBatch`, `TestDumpWithNoRIBClosesAtOnce`
   - Files: `bmp_locrib.go`. The deadline is read by one long-lived worker, never a goroutine per dump (`ai/rules/goroutine-lifecycle.md`)
   - Verify: tests fail, implement, tests pass
4. **Phase: Forked and end to end** -- the real paths
   - Tests: `TestDumpForkedRIBChunksPrecedeCompletion`, `test/plugin/bmp-locrib.ci`, `test/plugin/bmp-locrib-forked-rib.ci`, `bmp-locrib-pmacct`
   - Files: the `.ci` files, the lab assertion
   - Verify: revert the fix, rebuild, see red, restore, see green, and record it
5. **Phase: Pages** -- `docs/guide/bmp.md`, `process-protocol.md`, `core-design.md`, the RFC summaries

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No End-of-RIB is sent on any path that did not see the matching completion |
| Correctness | `replayBestPaths` emits completion on every path that received a request, the nil-bus return excepted, which the deadline covers |
| Naming | `replay-id` kebab-case, the same member as `replay.Request` |
| Data flow | BMP compares tokens only; `bgp-rib` holds no knowledge of who asked |
| Rule: stale comments | The comments in `emitReplayRequest`, `closeDumpFamilies` and `handleBestChange` that describe synchronous delivery are rewritten |
| Rule: no layering | The emit window and the per-batch marker are gone, not kept as a fallback |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Order proven on the real RIB | `./le test bgp plugin bmp-locrib` |
| Order proven at a peer | `./le test integration interop` with scenario `bmp-locrib-pmacct` |
| Scoped unit tests | `./le job run label bmp-unit command go test -tags 'ze_bgp ze_bmp' ./internal/component/bgp/plugins/bmp/ ./internal/component/bgp/plugins/rib/ ./internal/core/bgp/ribevents/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `replay-id` from a plugin is a number with no authority: an unknown token is ignored |
| Resource exhaustion | The scope table is bounded by the number of collector sessions, and each entry has a deadline |

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

- The commissioning brief named "a replay-complete event, a token surviving the JSON codec, a timeout". The code agrees on all three. It adds one consequence: a surviving token means forked chunks are claimed, so the per-batch marker in `handleBestChange` must go, because the comment that defends it relies on the token not surviving.
- The comment "BMP is in-process, so the token survives delivery" is true of the batch and false of the timing, which is how this stayed hidden.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `false-synchronization-claim.md` | 2026-11-06 | BMP Loc-RIB dump, `emitReplayRequest` sends End-of-RIB before the routes |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An explicit completion event | Count batches; a quiet period after the last batch | An empty family emits no batch, so a count cannot close it, and a quiet period is a guess that a slow RIB breaks |
| Add `replay-id` and keep `replay` | Replace `replay` | External readers decode `replay`; a second member costs one derived field |
| Reset the session at the deadline | Send the markers anyway; keep waiting | A marker would assert a dump that did not complete. Waiting forever is the case AC-7 forbids. The reset matches the storage-failure exit |
| Deadline measures silence | A fixed length | A million-route dump is healthy at any length while it progresses |

## Known Limitations
- Each collector's dump walks the table once. Sharing one walk between collectors is an optimization nobody asked for.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. The completion handler carries RFC 7854 Section 5 and RFC 4724 Section 4; the caller in `requestLocRIBDump` carries the section reference.

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
- [ ] The targeted tests pass, run in the main checkout: `./le test bgp plugin bmp-locrib`, `./le test integration interop` for `bmp-locrib-pmacct`, and the scoped package tests above. GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script
