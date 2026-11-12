# Spec: route-policy-1-gate-proof-and-forked-sysrib -- correct internal and external system RIB operation, with daemon proof of every import chain

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | The external system RIB lacks the authoritative routing state and transport wiring its internal mode uses. Its BGP-only fallback bypasses the Loc-RIB import gate when an EventBus is injected. Repair both execution modes, prove the import chains through the daemon, and declare the shipped capability. |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-route-policy-0-umbrella.md` (the committed gate, chains, and fixture drivers; its closure review produced B-1, B-2, and B-3) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Repair the system RIB so an operator gets correct routing with either an
internal plugin or an external subprocess. The owner requirement is recorded
verbatim in `docs/architecture/rib/unified-locrib.md`, “Owner requirement:
internal and external system RIB.” That requirement rejects the previous
internal-only proposal. The transport design below remains proposed until reviewed.

This spec owns the umbrella's three findings. The umbrella's ownership pointers
still need its separate closure edits. Creating this spec did not change them.

| Finding | Required result |
|---------|-----------------|
| B-2: the forked system RIB has a different, ungated input | Both modes consume the daemon's post-policy, cross-protocol selection. Both preserve distances, recursive next-hop resolution, ECMP, FIB policy, operational commands, and lifecycle recovery. |
| B-1: missing daemon proof | Promote the central and static import tests, add connected-chain proof, and retain the OSPF central-chain leg on the BIRD scenario. Exercise both system RIB placements. |
| B-3: undeclared and uncompared route policy | Add `features/route-policy.md` and the source-backed comparison row. Claims distinguish implementation from recorded proof. |

| Goal | Evidence required at implementation |
|------|-------------------------------------|
| Internal and external system RIBs have equivalent useful behavior | Paired daemon and kernel cases, with an actual TLS subprocess in the external case. Every case installs a permitted control and checks the changed behavior. |
| Every route reaches one import gate | Hidden candidate evidence and FIB outcomes for BGP, static, connected, OSPF, and IS-IS. Producer placement and system RIB placement are independent test dimensions. |
| Existing routes follow configuration and topology changes | Distance and filter reload, rollback, next-hop loss/recovery, ECMP promotion, and restart/replay tests without producer reannouncement. |
| Route policy is discoverable and honestly described | Feature report, comparison row, operator pages, process contract, and named proof records. |

### Revision history

| Date | Decision | Consequence |
|------|----------|-------------|
| 2026-11-12, initial design | Proposed declaring the system RIB internal-only, refusing external configuration, and adding VRRP/AS112 refusals. | Rejected by the owner. These proposed refusals are withdrawn, including the proposed exception to the system door. They were a consequence of the rejected repair, not one of B-1 to B-3. |
| 2026-11-12, this revision | Preserve and repair both system RIB modes. | Keep one authoritative Loc-RIB and gate in the daemon. Supply the missing process interfaces and prove parity. Existing VRRP/AS112 behavior is outside this system RIB repair and is not represented as repaired. |

## Required Reading

### Architecture Docs

- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/forwarding.md`
  → Constraint: internal and external transports provide the same feature. A feature owns its handlers and reaches shared infrastructure through registration.
- [ ] `docs/architecture/rib/unified-locrib.md`
  → Constraint: the daemon's Loc-RIB owns admission, hidden candidates, administrative distance, and cross-source selection. A selected-route projection must not run another gate or selector.
- [ ] `docs/architecture/forked-route-install.md`
  → Constraint: keep the existing producer sink and `locrib.Default()` contract. The refusal of a remote Loc-RIB proxy does not prohibit a narrow value protocol for consumers.
- [ ] `docs/architecture/plugin/plugin-system.md`, `docs/architecture/api/process-protocol.md`
  → Constraint: one evaluator serves both transports. Network operations and encoding never run under a Loc-RIB shard lock. Stage-2 configure cannot wait for runtime dispatch that starts after the handshake.
- [ ] `pkg/plugin/rpc/bridge.go`, `ai/patterns/registration.md`
  → Decision: extend the existing engine-op registration mechanism to admit owner-registered, process-bound handlers. The system RIB registers its operations from its own package. The shared dispatcher discovers them.
- [ ] `docs/architecture/route-selection.md`
  → Constraint: the existing engine `route-metrics` RPC resolves against the authoritative Loc-RIB directly. Do not introduce a daemon-to-external-system-RIB-to-daemon lookup cycle.
- [ ] `docs/guide/route-policy.md`
  → Constraint: filter `accept` continues the chain. Test configurations need the intended `final` action. Hidden paths retain the stored rejection reason.
- [ ] `docs/architecture/testing/interop.md`, `docs/contributing/feature-maturity.md`, `.agents/skills/ze-test/SKILL.md`
  → Constraint: prove discrimination at the real entry point, promote drafts into tracked tests, and claim only the feature level the retained evidence supports.

### RFC Summaries (Scope: protocol)

- [ ] No new protocol feature or RFC enrollment.
  → Constraint: preserve existing RFC-tagged forwarding and metric tests. A changed tagged unit still owes its existing review and discrimination procedure.

**Key insights:**

- Supplying an external consumer with authoritative values is different from replacing the core Loc-RIB with a remote proxy.
- A route feed alone cannot repair distance publication, hidden-candidate inspection, or external FIB output.
- The old design's claimed real-fork bypass was not demonstrated. The external launcher also omits EventBus injection, so the actual subprocess can stop routing before reaching that fallback.
- The selected-view revision does not track every hidden-candidate change and is not an ordered cursor across concurrent shards.

## Current Behavior (MANDATORY)

**Source files read for this revision:**

- [ ] `internal/component/sysrib/sysrib.go` `run` — Returns when no EventBus is configured. With a Loc-RIB, subscribes before replay and processes through a worker. Without one, subscribes to BGP best-change events.
  → Constraint: Replace the BGP fallback with authoritative external input. Wire input, output, and replay in a real subprocess.
- [ ] `internal/component/plugin/cli/cli.go` `BaseConfig`, `RunPlugin`; `internal/component/plugin/inprocess.go` `GetInternalPluginRunner` — Only the internal runner injects `ConfigureEventBus`. The external CLI connects and invokes the engine without that hook.
  → Constraint: Do not describe the unsafe fallback as proven real-fork behavior. Prove the launched external process, not an injected-bus substitute.
- [ ] `internal/component/sysrib/register.go` `runSysRIBPlugin`, `declareDistances`, `publishDistances`, `reselectLocRIB` — Wires the process-local Loc-RIB. Distances publish locally. Reselection does nothing when that Loc-RIB is nil. Configure/apply/rollback share this path.
  → Constraint: Publish and acknowledge distance changes at the daemon's authority in both modes, including startup defaults.
- [ ] `internal/component/sysrib/sysrib.go` `SetLocRIB`, `fibEntry`, `processLocRIBChange`, `changeToBatch` — A nil Loc-RIB removes the resolver. The no-resolver branch treats a raw target as proved. The local change carries the selected distance, complete primary metadata, backup, and ECMP siblings.
  → Constraint: Missing transport must never mean proven reachability. Preserve the existing local forwarding semantics and every input field.
- [ ] `internal/component/sysrib/nhresolver.go` `Resolve`, `CoveredNHs`, `chainCovered` — Reads Loc-RIB LPM and tracks recursive dependencies. Covering-route changes trigger dependent recomputation.
  → Constraint: Use the same resolver and cascade over local and external read backends.
- [ ] `internal/component/sysrib/locrib_feed.go` `offer`, `takeOverflow`, `resyncOverflow` — Defers processing outside shard locks. Overflow records prefixes and rereads them after older queued changes.
  → Constraint: Extend this nonblocking handoff. A slow subprocess cannot block route insertion or silently lose a withdrawal.
- [ ] `internal/component/sysrib/candidates.go` `showRIBCandidates`, `candidateEntryOf`, `candidatePathOf` — Reads all daemon candidates, including hidden ones, and refuses without a Loc-RIB.
  → Constraint: External commands query the authority and use the same renderer, including stored chain attribution and optional tag presence.
- [ ] `internal/core/rib/locrib/default.go` `Default`; `internal/core/rib/locrib/manager.go` `OnChange`, `Revision` — The singleton is nil in subprocesses. Callbacks run under shard locks. Revision describes selected-route changes.
  → Constraint: Keep the singleton process-local. Use a separate ordered feed cursor and a separate candidate snapshot identity.
- [ ] `internal/core/rib/locrib/policy.go` `admit`, `PublishPolicy`; `internal/core/rib/locrib/distance.go` `Reselect` — Own admission and rerank stored paths, including hidden candidates, without another producer event.
  → Constraint: Keep these as the only policy and cross-source selection authority.
- [ ] `internal/component/plugin/server/dispatch_route_metrics.go` `opRouteMetrics`; `internal/core/rib/igpcost/igpcost.go` `Lookup`, `Resolve` — Engine metric RPC resolves the daemon's Loc-RIB directly. The local seam has an authoritative fallback.
  → Constraint: Preserve this route and received-AIGP semantics.
- [ ] `internal/component/plugin/server/dispatch_registry.go` `engineOps`; `internal/component/plugin/server/dispatch.go` `dispatchPluginRPC`, `dispatchPluginRPCDirect` — Engine ops drive JSON and Direct dispatch. Raw extension handlers have no authenticated process context.
  → Constraint: Extend owner registration with process context. Do not add a list of system RIB method names in shared dispatch.
- [ ] `internal/component/plugin/server/rpc_register.go` `RegisterRPCs`, `RegisterProcessCleanup` — Builtin command registration and process cleanup exist. Builtins are a different dispatch path from raw engine RPCs.
  → Constraint: Reuse cleanup. Do not assume builtin command registration wires a raw SDK call.
- [ ] `internal/component/plugin/server/startup_driver.go` `runStartupHandshake`; `internal/component/plugin/server/startup.go` `deliverConfigRPC` — The handshake waits for configure before runtime handlers start.
  → Constraint: Explicitly serve only registered startup-safe operations during configure. An external distance RPC cannot wait for the runtime reader.
- [ ] `internal/component/sysrib/sysrib.go` `publishChanges`, `replayBest`; `internal/component/sysrib/events/events.go` `BestChangeBatch` — Publish live and replay output through the injected bus. Replay includes only routes Ze programs and recomputes promoted forwarding paths.
  → Constraint: Carry output through an acknowledged, generation-bound daemon sink. Preserve the downstream JSON and replay contract.
- [ ] `internal/test/fixture/rib_import_policy_fixture.go` `ribImportCentralBogons`, `staticImportFilterDistance`; `internal/test/fixture/register_rib_import_policy.go` — Drivers are committed. They require selected controls and hidden candidates. Their candidate decoder lacks distance/tag and they do not reload.
  → Constraint: Extend the drivers and promote the missing carriers.
- [ ] `test/plugin/forked-route-install-kernel.ci` — Forks a route producer and explicitly keeps the system RIB internal.
  → Constraint: Producer proof does not prove external system RIB operation.

**Behavior to preserve:** operator configuration syntax, accepted internal and
external plugin forms, local fast-path behavior, producer route-install/remove,
BGP observation events, FIB events and replay markers, all four system RIB
operational commands, FIB withholding, OS-owned routes, and existing test strength.

**Behavior to change:** replace the unsafe BGP input and disconnected external
state with complete process interfaces. Remove the candidate-command instruction
to move the system RIB inside the daemon. Do not add an internal-only declaration,
an external-run refusal, or an exception to the system door.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| One Loc-RIB and admission gate | `internal/core/rib/locrib/` `insert`, `admit`, `Reselect` | Stores candidates, applies policy, and selects across sources. | Preserves it as the daemon authority. |
| Forked producer sink | `internal/core/rib/routeinstall/`; `internal/component/plugin/server/dispatch_route.go` `applyRouteInstall` | Sends producer paths through that authority. | Preserves it and tests producer placement independently. |
| Worker and overflow resync | `internal/component/sysrib/locrib_feed.go` | Moves processing outside shard locks without dropping overflowed prefixes. | Extends it for the ordered external feed. |
| Local recursive resolver and FIB logic | `internal/component/sysrib/nhresolver.go`; `internal/component/sysrib/sysrib.go` `fibEntry` | Resolves and tracks next hops, builds forwarding changes and replay. | Uses the same algorithms with two read backends. |
| Engine-op declaration and DirectBridge | `internal/component/plugin/server/dispatch_registry.go`; `pkg/plugin/rpc/bridge.go` | One shared handler answers the two transports. | Extends registration to owner-supplied, process-bound operations with explicit startup admission. |
| Process cleanup | `internal/component/plugin/server/rpc_register.go` `RegisterProcessCleanup` | Runs owner cleanup on process exit. | Registers routing-session cleanup and output withdrawal. |
| Candidate renderer | `internal/component/sysrib/candidates.go` | Formats selected, standby, and hidden candidates. | Reuses it for authoritative local and external queries. |
| Internal-only declaration | `internal/component/plugin/registry/registry.go` `Registration.RunsInProcessOnly` | Describes plugins that require daemon state. | Does not apply to this repair. Its existence does not authorize disabling external system RIB support. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Existing internal configuration and `plugin external` configurations launch
the same system RIB implementation. Protocols offer paths locally or through
route-install. Operators use the same configuration and command paths in both modes.

### Transformation Path

1. The daemon Loc-RIB admits each offered path through the existing source and central chains, stores rejected candidates hidden, resolves distance, and selects.
2. Internal system RIB input uses the local feed. External input opens a process-bound routing-view session with the daemon. An owner-registered adapter supplies authoritative selected entries and tombstones.
3. A bounded handoff records changes under the shard lock. A worker copies owned values, orders the feed, and performs snapshot/delta transport outside every shard lock.
4. The external consumer builds a read-only prefix projection of selected paths. It runs no admission or cross-source selection and never inserts the projection into a second Loc-RIB.
5. Both modes run the existing next-hop resolver, dependency tracking, cascade, ECMP, FIB permission, and replay algorithms. The resolver uses a narrow LPM reader: authoritative local reads or the completed external projection.
6. Distance configuration uses the same parsed declaration. An acknowledged daemon-side operation publishes it through the existing distance seam and calls authoritative `Reselect`. Apply and rollback report success only after that operation succeeds.
7. Both modes publish forwarding changes through a session-bound sink. The daemon adapter emits the unchanged system-RIB event and records the outstanding output for scoped cleanup.
8. Candidate commands query the daemon's full candidate state on demand. The existing renderer supplies identical output. Selected-view cursors are not candidate-cache validity tokens.

### Process contract

These are new operations, not claims that the present SDK already implements them.
The system RIB owns their handlers in `internal/component/sysrib/` and their
YANG in `internal/component/sysrib/yang/ze-sysrib-api.yang`. Thin SDK methods and
public value types carry them. Shared dispatch learns no system RIB config path,
plugin name, or policy. Owner registration extends the existing engine-op registry.

| Operation | Request | Result and obligations |
|-----------|---------|------------------------|
| Open selected view | Authenticated caller and execution generation, established by the server rather than trusted from JSON. Allowed during Stage 2 after registration and during runtime. | One session per active system RIB owner, server epoch, initial cursor, and snapshot identity. This bootstrap operation establishes the token distance application needs. A replacement waits for old cleanup. |
| Read selected view | Session token, cursor, record limit, bounded wait. | Ordered snapshot pages or deltas, completion marker, next cursor, epoch, and explicit reset-required result. Records contain selected entries or tombstones. |
| Read candidates | Session token, optional family/prefix, bounded page cursor. | A separately pinned candidate snapshot with hidden and standby paths, selected flags, stored rejection attribution, and all existing display fields. Prefix queries use an atomic prefix read. |
| Apply distances | Session token, complete validated protocol-name/distance table, ordered operation identity. | Publish at the daemon and reselect. Acknowledge the applied declaration and routing-view barrier. Retry is idempotent. Reject a stale operation or another process's session. |
| Publish forwarding | Session token, consumed view cursor, ordered publication identity, existing system best-change batch including replay identity. | Admit only the active generation's ordered output. Record attempted output ownership before emission, emit on the existing bus, and retain the acknowledgement. Duplicate retry does not publish twice. An emission failure fences the session and withdraws every possibly emitted entry. |
| Close selected view | Session token. | Fence further publication, cancel workers and reads, release snapshots, and withdraw outstanding output. Process exit takes the same cleanup path. |

| Selected entry field group | Required values |
|----------------------------|-----------------|
| Identity | Family AFI/SAFI, masked prefix, action, protocol name, source instance. Numeric process-local protocol IDs never cross the wire. |
| Ranking and policy results | Resolved administrative distance, metric, tag with presence, source level, OSPF route type, BGP/eBGP class, AIGP with presence, recursive-metric flag. These are values already decided by the authority. |
| Primary forwarding | Gateway, interface, on-link, weight, forwarding route type, labels, SRv6 SID. Keep absent values distinct from zero-valued values. |
| Multipath and repair | Complete equal-cost sibling targets and the primary's repair gateway, interface, on-link flag, and repair labels. Preserve every input `changeToBatch` and the resolver consume. |
| Lifetime | Owned slices and values. No borrowed path slice, Loc-RIB pointer, forward handle, or pooled wire-buffer pointer travels. |

| Ordering and lifecycle rule | Required behavior |
|-----------------------------|-------------------|
| Snapshot and live changes | Subscribe before snapshot. Reconcile mutations during the scan before completing the initial view. Apply a completed snapshot as a unit before forwarding or replay. Buffered older state must never overwrite a later withdrawal. |
| Feed cursor | The session worker assigns an epoch and ordered cursor. `locrib.Revision()` remains the metric invalidation signal and is not repurposed as that cursor. Reject impossible, future, stale, and cross-session cursors explicitly. |
| Overflow and resync | Record dirty prefixes without waiting for the consumer. Reread them after older work. If retained deltas cannot satisfy a cursor, fence publication, withdraw the session's output, and return reset-required. Rebuild and reconcile before publication resumes. |
| Next-hop ordering | Apply an entire selected-view batch before resolving its dependent routes. A full snapshot builds the complete LPM projection before computing FIB entries. Cover loss and recovery trigger recomputation without a producer event. |
| Candidate consistency | Hidden-only changes can leave the selected revision unchanged. Candidate paging uses an immutable request snapshot and its own identity, not that revision. Close, timeout, or disconnect releases it. |
| Startup | Extend the shared handshake to serve only owner-registered startup-safe engine operations while configure awaits its reply. Authenticate and bind the owner first. Stage 2 opens its session and applies distances, even when its sections are empty and it must use schema defaults. Only open and distance-apply are startup-safe. Feed reads, candidate reads, and forwarding publication wait for runtime admission. No second connection reader or wait for a later plugin tier is introduced. |
| Startup readiness | Synchronize the already-open selected view once runtime dispatch exists. Until complete, the plugin reports initializing and publishes no forwarding or replay. Retain early FIB replay requests and answer after synchronization, so a consumer need not request again. The daemon ready file alone does not prove view readiness. A synchronization failure closes the connection and follows the existing failed-plugin policy. |
| Feed failure | Mark the session unhealthy, fence output, and withdraw only its outstanding routes through existing FIB events. Close the failed SDK connection and use the existing restart policy. Never fall back to BGP or treat unavailable state as an empty table or proved reachability. |
| Output ownership | Generation fencing happens before emission. Cleanup and replacement serialize so an old cleanup cannot withdraw a new generation's output. OS-owned and withheld routes never enter the outstanding output ledger. |
| Limits | Read limits are 1 through 4096 records. Wait is 0 through 1000 ms and cannot exceed the caller deadline. Bound retained deltas and snapshot lifetime. Expiration produces an explicit reset, not partial success. Malformed pages or field values fail atomically. |

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Producer to daemon | Existing local inserts or route-install/remove | Existing mechanism, new placement matrix still owed |
| Daemon to external system RIB | Ordered value snapshot/deltas and candidate pages | Not implemented |
| System RIB configuration to daemon | Acknowledged distance publication and reselection | Not implemented |
| System RIB to FIB consumers | Session sink, then unchanged best-change/replay events | External sink and cleanup proof owed |

### Changed-Wire Census

**Search evidence:** source searches over `internal/`, `pkg/`, and `test/`
covered Go and the `.ci`, YANG, shell, Python, JSON, and config carriers.
Terms included `BestChange.Subscribe`, `BestChange.Emit`,
`ReplayRequest.Emit`, `FromLocRIB`, `route-install`, `route-metrics`,
and `system-rib`. Positive controls were `sysrib.run`'s BGP subscription,
both system output emitters, `forked-route-install-kernel.ci`, and the engine
YANG `route-metrics` declaration. New method names must be searched again
after registration, SDK, schema, and tests are written.

| Wire | Role | Carrier | Source evidence | Migration or preservation disposition |
|------|------|---------|-----------------|--------------------------------------|
| New routing-view operations | writer | Planned SDK routing-view methods and local adapter | The existing SDK engine-call pattern selects DirectBridge or MuxConn. | Add the public DTOs, owner YANG, registered handler, and two-transport tests together. |
| New routing-view operations | reader | Planned system-RIB-owned daemon adapter and external projection consumer | Current authority and renderer are `locrib` and `candidates.go`. | Read authority in the daemon. Decode owned values externally. Include non-Go JSON fixtures. |
| Startup engine calls | writer | `pkg/plugin/sdk/sdk.go` `callEngineRaw`, configure callback | Current raw calls wait on engine dispatch. | Add bounded startup-safe admission, preserving existing stage declarations and barriers. |
| Startup engine calls | reader | `internal/component/plugin/server/startup_driver.go` `runStartupHandshake`, `startup.go` `deliverConfigRPC` | Configure precedes runtime handlers. | Both transports use one registered operation definition. Unknown and runtime-only operations remain refused. |
| BGP best-change | writer | `internal/component/bgp/plugins/rib/rib_bestchange.go` `publishBestChanges` and replay emitter | Both emit `ribevents.BestChange`. | Preserve payload and publication. |
| BGP best-change | reader | `internal/component/sysrib/sysrib.go` `run`; `internal/plugins/flowexport/enrichbgp.go` `applyBatch`; `internal/component/bgp/plugins/bmp/bmp_locrib.go` `handleBestChange`; outside subscribers | Source search finds all three in-tree subscriptions. | Replace only the system RIB subscription. Preserve BMP, flow export, and outside observers. |
| BGP replay-request | writer | System RIB `run`, BMP `bmp_locrib.go`, FlowSpec firewall `engine.go` | Each emits `ribevents.ReplayRequest`. | Remove only the system RIB request. The other producers remain. |
| BGP replay-request | reader | BGP RIB replay handler and `rib_bestchange.go` emitter | Existing replay response stays BGP-scoped. | Preserve. |
| Route-install/remove and route-metrics | writer | SDK engine methods, route-install sink, external producer fixtures, engine YANG | Existing RPC methods and `forked-route-install-kernel.ci` are controls. | Preserve method names and payloads. Add the producer/system-RIB placement cross-product proof. |
| Route-install/remove and route-metrics | reader | `dispatch_route.go` `applyRouteInstall`; `dispatch_route_metrics.go` `opRouteMetrics` | Both read the daemon's Loc-RIB. | Preserve gate and metric authority. |
| System best-change | writer | `sysrib.go` `publishChanges`, `replayBest`; test helper `internal/test/plugins/fakefib/` | Two production emit sites and fake FIB fixture population. | Production output uses the generation-bound sink. Preserve the downstream DTO and fixture contract. |
| System best-change | reader | `internal/plugins/fib/kernel/fibkernel.go` `run`; `internal/plugins/fib/vpp/register.go` `subscribeFibVPP`; `internal/plugins/fib/p4/fibp4.go` `run`; external consumers | The VPP lifecycle registration holds its subscription. Kernel and P4 subscribe in `run`. All three read `sysribevents.BestChange`. | Preserve the JSON, ownership, multipath, and replay semantics. |
| System replay-request | writer | Kernel, VPP, and P4 FIB startup paths | Each requests system replay. | Preserve the request shape and replay identity. |
| System replay-request | reader | `sysrib.go` `run`, `replayBest`; new external SDK subscription | Current subscription assumes an injected bus. | Wire external delivery through the SDK and answer only from a synchronized current view. |

### Integration Points

- `runSysRIBPlugin` connects the selected execution process to the registered daemon view, shared forwarding algorithms, and acknowledged output sink.
- `declareDistances` reaches the authoritative distance operation at configure, apply, and rollback, including empty-section startup defaults.
- The engine-op registry supplies both transport wrappers and phase admission. The system RIB registers its handlers and cleanup without a shared-code plugin-name list.
- Candidate commands read the daemon authority. SDK event subscription carries FIB replay requests to the external process.

### Architectural Verification

| Check | Design requirement |
|-------|--------------------|
| No bypassed layers | The single daemon gate and selector produce every selected entry. No BGP-only consumer fallback survives. |
| No unintended coupling | The system RIB owns handlers. Generic registration supplies authenticated process context and startup admission. Shared code names no system RIB config path. |
| No duplicated functionality | A read-only projection supplies LPM. It has no producer insertion, policy evaluator, or best-path selector. |
| Local performance preserved | Keep direct local reads and nonblocking feed handoff. Encode bounded batches only on external transport. Measure allocations for affected paths before claiming parity. |
| Registration, both directions | Register new operations from the owner. JSON and Direct wrappers discover them. Do not add them to central dispatch cases, method lists, or plugin-name validators. |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | A narrow LPM reader covers the resolver's store dependency. | `nhResolver.Resolve` and `chainCovered` use LPM. | Reader design needs another operation. | Enumerate all resolver/cascade reads and run paired backend units. | Source-supported, runtime proof owed |
| A-2 | Existing engine-op wrappers can serve an owner registry with process-bound context. | `engineOps` serves both transports, but current raw extension handlers lack that context. | Generic registration or lifecycle adaptation needs redesign. | Wiring units and actual fork startup with a named external block. | unvalidated |
| A-3 | Startup-safe nested calls can preserve the existing handshake barriers. | `runStartupHandshake` currently waits synchronously for configure. | Distance bootstrap deadlocks. | Bounded configure test on socket and Direct transports, with a second plugin in the tier. | unvalidated |
| A-4 | A complete selected projection contains all inputs to the existing FIB algorithms. | `changeToBatch`, `fibEntry`, resolver, and replay reads. | Add missing values to the contract before implementing consumers. | Field-by-field round trips and recursive/ECMP parity tests. | unvalidated |
| A-5 | Existing cleanup order can serialize old session teardown with replacement. | `RegisterProcessCleanup` exists. Existing producer cleanup is not system-output cleanup. | Add explicit session-generation coordination. | Delayed old-publish/cleanup versus replacement interleavings and daemon restart test. | unvalidated |
| A-6 | A real interface can provide the connected/static controls without an extra routing peer. | Existing connected-distance and static-distance functional tests. | Adjust fixture setup without reducing kernel proof. | Draft runs on Ze's kernel with net-admin. | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Old snapshot data reinstalls a newly hidden route. | Controlled snapshot/withdraw test leaves a ghost. | Reconcile scan-time mutations and fence stale epochs/cursors before publication. |
| R-2 | Network I/O under a shard lock stalls inserts or deadlocks resolution. | Blocked consumer blocks a concurrent insert. | Callback only records bounded work. Worker owns copying, encoding, and sending. |
| R-3 | A selected-only revision hides candidate changes. | Hidden tag/reason changes do not reach candidates output. | Query authority and pin candidate snapshots separately. |
| R-4 | External startup acknowledges a distance table only in its own process. | Candidates retain the old distance or startup stalls. | Acknowledged daemon operation and explicit startup-safe request servicing. |
| R-5 | Failure leaves stale FIB output, or old cleanup deletes replacement output. | Kill/restart case leaves a ghost or loses the control. | Process-generation fencing, outstanding-output ledger, serialized cleanup and fresh snapshot. |
| R-6 | Tests pass because nothing arrived. | Dropping every path leaves a green run. | Require candidate existence, selected controls, and kernel-installed permitted routes. Record discriminating red runs. |
| R-7 | Refactoring batch processing weakens existing tests. | Units are deleted, assertions disappear, or a no-op backend passes. | Preserve assertions and use independent `ze-test-judge` for changes subject to that rule. |
| R-8 | Slow consumers or abandoned snapshots retain unbounded memory. | Retention grows after timeout/disconnect. | Bound journals, one active owner session, bounded reads and snapshot lifetimes, explicit reset and cleanup. |
| R-9 | Foreign callers change distances or forge a feed generation. | Another process can publish or acquire the owner lease. | Bind authority to the authenticated configured registration and process generation, never JSON names. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every FIB consumer can receive wrong or stale routes. Startup or configuration can deadlock. The paired daemon/kernel and controlled lifecycle tests are required before closure. |
| Recovery | Keep the configuration syntax stable. Correct defects through scoped follow-up commits and preserve the last committed working sources. No compatibility fallback to the unsafe BGP input. |
| Related work | The umbrella supplies the existing gate. `plan/release/fixes/spec-single-owner-reload-reconcile.md` owns broader reload ordering. `plan/release/validation/spec-fib-depth.md` owns forwarding proof beyond this transport repair. Preserve other sessions' changes in shared files. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Internal and actual external system RIB configs, validate and boot | → | `runSysRIBPlugin`, owner operation registration and SDK transport | `test/plugin/sysrib-execution-parity.ci`, `sysrib-execution-parity-forked.ci` |
| Stage-2 configure with explicit distances and with no RIB block | → | Startup-safe registered distance operation | `TestRouteViewStartupDistancesBothTransports` |
| Source and central import chains | → | Daemon `admit`, selected feed, system RIB, candidates | Paired central/static/connected tests below |
| Kernel installation through an external producer and external system RIB | → | Route-install, authoritative view, forwarding sink, fib-kernel | `test/plugin/sysrib-forked-producer-kernel.ci` |
| FIB consumer replay request | → | SDK subscription, synchronized `replayBest`, output sink | `test/plugin/sysrib-ecmp-replay-forked.ci` |
| System RIB process exits and restarts | → | Process cleanup, generation fence, output withdrawal and resnapshot | `test/plugin/sysrib-feed-restart-forked.ci` |

## Acceptance Criteria

AC-6 through AC-15 retain the original findings' identities. AC-1 through AC-5
replace the rejected refusal design. Every paired test runs once with an internal
system RIB and once with an actual external subprocess.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Equivalent internal and external system RIB configurations. | Validate and boot accept both. A permitted route reaches system output and the kernel in both. External placement is proved by a live child and TLS transport. | Internal-by-`use` and `ze.<plugin>` forms remain accepted because execution placement is an operator choice. |
| AC-2 | Selected, standby, and hidden candidates in either mode. | Full and prefix-filtered candidate commands show equivalent source, instance, gateway, interface, distance, metric, tag presence, level, route type, selected, hidden, hidden-by, and chain values. Hidden-only edits become visible. | Hidden and standby paths remain inspectable because the authority stores them for re-admission and diagnosis. |
| AC-3 | BGP, static, connected, OSPF, and IS-IS sources reach the daemon, with each system RIB placement. | Each source has an admitted control in the selected view. Policy-rejected paths stay visible as hidden candidates and cannot enter system forwarding output. | BGP observation events and protocol databases retain their own routes because import policy governs local installation. |
| AC-4 | Emit a BGP best-change event for a rejected prefix beside an admitted control. | Both system RIB modes retain the admitted control and follow only authoritative post-policy selection. | BMP, flow export, and third-party BGP observers still receive the unchanged BGP event for their own purposes. |
| AC-5 | External input/output transport absent, malformed, or lost after a permitted route was installed. | The plugin reports the actionable routing-view failure, fences publication, and withdraws its outstanding output. A fresh healthy session installs the permitted control again. | OS-owned routes, withheld selections, and other daemons' routes survive because cleanup is scoped to this session's emitted output. |
| AC-6 | Central import rejects 192.0.2.0/24 offered by BGP and static, with admitted controls. | Both placements select the controls. Each judged path is stored with hidden true, hidden-by central, and chain rib import. FIB output agrees. | Controls are installed because the chain permits them and they prove route arrival. |
| AC-7 | BIRD advertises the existing OSPF scenario prefixes plus one central-chain reject. | Both system RIB placements keep the original distance/winner assertions and show the new OSPF candidate hidden by rib import. | The OSPF database retains the rejected prefix because the import gate acts after SPF. |
| AC-8 | Static import rejects a prefix also offered by BGP. | In both placements BGP wins and static is hidden by the source chain. | BGP remains permitted because the source chain is specific to static. |
| AC-9 | Remove AC-8's rejection and reload without changing the static route or BGP session. | In both placements the stored static path becomes selected and reaches the FIB. | BGP stays as an unselected candidate because re-admission does not erase competitors. |
| AC-10 | Static import sets a distance that changes the winner against BGP. | Candidate distance and FIB winner match in both placements. Reload and rollback restore the corresponding winners. | Unmatched routes keep their declared distance because the modify filter is selective. |
| AC-11 | Static offers tag 4242, tag zero, and an untagged control; central policy rejects the tagged match. | With static internal and external, and system RIB internal and external, tag presence survives each boundary. The matching candidate is hidden and the control is selected. | Untagged and zero-tag paths remain distinct because zero is a real tag. |
| AC-12 | Connected import rejects one of two prefixes on an interface. | In both placements the admitted connected prefix is selected and the other candidate is hidden by connected import. | Both kernel-owned connected routes survive because Ze does not own their installation. |
| AC-13 | Break each admission, feed, distance, candidate, resolver, and output behavior its new test claims. | The relevant test fails at its semantic assertion and passes after restoration. Logs identify actual test execution and kernel proof where required. | Unrelated checks remain unchanged because each break targets the behavior its test proves. |
| AC-14 | Feature report after implementation. | Route policy is declared with source, test, and documentation evidence. Both system RIB modes are included only after their proof runs. | Experimental maturity remains permitted until supported-level evidence exists. |
| AC-15 | Comparison and redistribution examples. | The comparison has a source-backed per-protocol import/export row and current filters/final syntax. | Unverified competitor cells say Unclear because unsupported claims are not evidence. |
| AC-16 | Startup defaults, configured distances, per-route overrides, runtime change, rejected reload, rollback, and root removal. | Both placements publish the same daemon declaration and select the same existing routes without new announcements. A failed apply reports failure and restores the previous declaration. | Per-route overrides and schema defaults remain effective because they are part of existing ranking semantics. |
| AC-17 | Snapshot concurrent with insert/update/withdraw, delta overflow, expired cursor, and restart. | A completed projection matches the authority. A newer withdrawal remains withdrawn. Reset reconstructs a complete current view and reinstalls the admitted control. | Other prefixes continue once a synchronized view permits publication. Partial snapshots never count as synchronized. |
| AC-18 | Covering next-hop routes change across recursive IPv4, IPv6, and cross-family cases. | Both modes produce the same resolved gateway, device, on-link state, metric, and dependency updates. Loss and recovery of previously proved reachability update FIB state without producer reannouncement. | Preserve the existing internal distinction between never-proved and previously-proved targets. Missing routing-view state is always an error, never proof. |
| AC-19 | Weighted ECMP, member loss/recovery, primary promotion, device-only targets, labels, SRv6 SID, and repair backup. | Both placements preserve the complete forwarding targets and emit equivalent live and replay entries, including promoted metadata. | Remaining reachable members continue forwarding because one member's loss does not withdraw the whole group. |
| AC-20 | FIB withholding and OS-installed selections, then reload and replay. | Both modes show the selected routes correctly and program the permitted Ze-owned control. Withheld/OS-owned selections create no Ze FIB write or stale replay entry. | OS and other-daemon routes survive because system selection does not transfer their ownership. |
| AC-21 | Internal and external BGP RIB query route-metrics while the system RIB runs externally. | Metrics match authoritative daemon resolution, including resolved zero, recursive cost, and missing-AIGP distinction. Topology changes invalidate the metric result. | Existing received-AIGP handling survives because the metric RPC remains independent of the consumer process. |
| AC-22 | Old-generation publication races replacement and old cleanup. | The new generation's admitted control stays installed. Old publish/replay is refused and old cleanup cannot withdraw replacement output. | Only the active generation publishes because the authenticated session owns the output lease. |
| AC-23 | Foreign process/token, invalid protocol/family/prefix, malformed page, duplicate operation, or exceeded limit. | A healthy owner's control remains installed. Invalid requests return an error before partial mutation; valid retries return the same acknowledgement without duplicate output. | Other registered producer protocols and ordinary SDK callers retain their existing contracts. |
| AC-24 | Internal fast path and external full-table burst with a slow reader. | Internal routing uses direct reads without serialization. External memory stays within declared retention bounds, and insertion can progress while transport is blocked. Overflow produces explicit recovery and current routes. | Existing BGP and FIB event consumers retain their payloads and replay semantics. |
| AC-25 | Runtime doctor during initialization, healthy operation, reset, failure, and recovery. | It reports the actual view/session state and the exact restart command when recovery requires it. Healthy external placement is reported as healthy. | Existing internal-only diagnostics for other plugins remain unchanged because this spec adds no such restriction. |

## End-to-End User Stories

| User does | Path through system | Test proving it works |
|-----------|--------------------|-----------------------|
| Chooses internal or external system RIB placement | Config, startup, authoritative view, common forwarding logic, FIB | Paired `sysrib-execution-parity` tests |
| Blocks a bogon and later permits it | Source/central gate, hidden candidate, reload, selected delta, FIB | Paired import-chain tests and OSPF/IS-IS scenarios |
| Changes distance or loses a next hop | Daemon reselection or view update, common resolver/cascade, FIB | Paired `sysrib-distance-reload` and `sysrib-recursive-nexthop` tests |
| Restarts the external system RIB or a FIB subscriber | Fenced old session, scoped withdrawal, new snapshot, current replay | `sysrib-feed-restart-forked` and paired `sysrib-ecmp-replay` tests |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRouteViewRegisteredBothTransports` | `internal/component/sysrib/route_view_test.go` | Owner registration reaches the same handler on JSON and Direct paths. | |
| `TestRouteViewStartupDistancesBothTransports` | `internal/component/plugin/server/startup_route_view_test.go` | Safe nested configure calls, no-RIB-block defaults, deadline and tier barrier. | |
| `TestRouteViewSnapshotWithdrawOrdering`, `TestRouteViewOverflowReset` | `internal/component/sysrib/route_view_test.go` | Controlled ordering and complete resync, including concurrent shards. | |
| `TestRouteViewFieldsRoundTrip`, `TestRouteViewRejectsForeignSession` | `internal/component/sysrib/route_view_test.go` | Every field, presence bit, boundary, and caller binding. | |
| `TestRouteViewDistanceApplyRollback` | `internal/component/sysrib/route_view_test.go` | Authoritative declaration and reselection, ordered idempotent apply/rollback. | |
| `TestRouteViewHiddenCandidatesStayFresh` | `internal/component/sysrib/route_view_test.go` | Hidden-only edits and separately pinned candidate paging. | |
| `TestRouteViewResolverParity`, `TestRouteViewReplayParity` | `internal/component/sysrib/route_view_test.go` | Local and projection backends use the same forwarding algorithm. | |
| `TestRouteViewOutputGenerationCleanup` | `internal/component/sysrib/route_view_test.go` | Acknowledged output, failure withdrawal, replacement fencing. | |
| Existing system RIB, resolver, permission, and forwarding units | `internal/component/sysrib/` | Preserve all prior assertions. Repoint obsolete BGP-arbitration subjects through independent test judgment where required. | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Read limit | 1..4096 records | 4096 | 0 | 4097 |
| Requested wait | 0..1000 ms and within caller deadline | 1000 | Negative wire value | 1001 |
| Declared distance and member weight | Existing uint8 contracts, with zero presence semantics preserved | 255 | Negative wire value | 256 |
| Epoch/cursor and operation identity | Session-bound ordered values | Current valid identity | Stale/foreign identity | Future identity and wrap |
| Snapshot/journal retention | Declared bounded capacity/lifetime | Exact capacity or deadline | N-A | Overflow/expiration requires explicit reset |

### Functional Tests

Each paired row creates a base `.ci` and a `-forked.ci` carrier in
`test/plugin/`, sharing an assertion driver. The external carrier uses
`run "ze plugin rib"` and proves child-process placement. Kernel cases carry
Linux and net-admin requirements and run on Ze's kernel. Draft, discriminate,
prove under load, then promote through `ze-test`.

| Carrier stem | Scenario / criteria |
|--------------|---------------------|
| `sysrib-execution-parity` | Validate/boot, real external process, admitted kernel route, equivalent commands: AC-1..AC-3. |
| `rib-import-central-bogons` | Central chain and candidate/FIB control: AC-4, AC-6. |
| `static-import-filter-distance` | Source rejection, re-admission, distance and tag changes: AC-8..AC-11. |
| `connected-import-filter-hides` | Selected control and hidden connected path, preserved OS routes: AC-12. |
| `sysrib-distance-reload` | Defaults, override, apply/reject/rollback/root removal: AC-16. |
| `sysrib-recursive-nexthop` | Recursive reachability and BGP metric checks: AC-18, AC-21. |
| `sysrib-ecmp-replay` | Weighted paths, promotion, metadata, replay, FIB permissions: AC-19, AC-20. |
| `sysrib-feed-restart` | Overflow/reset, process failure/restart, scoped withdrawal, doctor recovery: AC-5, AC-17, AC-22, AC-25. |
| `sysrib-forked-producer-kernel` (external system RIB) | Actual forked static producer and route-install fixture, authoritative gate, admitted kernel route, producer disconnect withdrawal: AC-3, AC-11, AC-23. |

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer | What it proves |
|----------|-----------|------|----------------|
| `ospf-import-policy-bird` | `test/interop/scenarios/ospf-import-policy-bird/` | BIRD 3 | Retain existing checks and add central rejection/control. Run each system RIB placement. |
| `isis-import-policy-frr` | `test/interop/scenarios/isis-import-policy-frr/` | FRR | Retain import checks and prove selected/control/hidden outcomes with each system RIB placement. |

Read the peer source at the lab version before changing expectations. Keep
existing export-policy scenarios green within the affected scope. Do not use
a routing-protocol stand-in as the only proof that the actual OSPF/IS-IS
attachment and external system RIB are connected.

## Files to Modify

- `internal/component/sysrib/register.go` - Bootstrap and configure both transports, including daemon distance apply and rollback.
- `internal/component/sysrib/sysrib.go` - Replace the BGP fallback with selected-view input and the acknowledged output sink.
- `internal/component/sysrib/nhresolver.go` - Read local authority or completed projection through the same narrow LPM dependency.
- `internal/component/sysrib/locrib_feed.go` - Nonblocking handoff, ordered delivery and authoritative overflow recovery.
- `internal/component/sysrib/candidates.go` - Render full authoritative candidates in either placement.
- `internal/component/sysrib/fibimport.go` - Preserve permission and output ownership through the common sink.
- `internal/component/plugin/server/dispatch_registry.go` - Generic owner operation registration, authenticated generation context, bounded startup-safe dispatch and cleanup integration. No system RIB-specific branches.
- `internal/component/plugin/server/dispatch.go` - Companion change for the preceding source.
- `internal/component/plugin/server/startup_driver.go` - Companion change for the preceding source.
- `internal/component/plugin/server/startup.go` - Companion change for the preceding source.
- `internal/component/plugin/server/rpc_register.go` - Companion change for the preceding source.
- `pkg/plugin/rpc/bridge.go` - Public value contract, thin transport methods and direct path. Preserve existing generic SDK event loops and declaration purity.
- `pkg/plugin/rpc/types.go` - Companion change for the preceding source.
- `pkg/plugin/sdk/sdk.go` - Companion change for the preceding source.
- `pkg/plugin/sdk/sdk_engine.go` - Companion change for the preceding source.
- `internal/core/rib/igpcost/igpcost.go` - Generalize only the lookup dependency needed to reuse the existing resolver against both readers. Preserve the default authoritative route and metric semantics.
- `internal/core/bgp/ribevents/ribevents.go` - Remove the obsolete consumer-only marker if it has no remaining use after all batch callers are migrated. Preserve BGP JSON.
- `internal/test/fixture/rib_import_policy_fixture.go` - Distance/tag observations, reload and connected-chain drivers.
- `internal/test/fixture/register_rib_import_policy.go` - Companion change for the preceding source.
- `internal/le/interoplab/bgp/check_import_policy.go` - Add required parity observations and placement runs without weakening existing checks.
- `internal/core/diagnostic/codes.go` - Register the routing-view failure code and retain an exact recovery command.
- `internal/component/sysrib/` - Preserve semantic coverage under the new input adapters and get independent judgment when required.

## Files to Create

- `internal/component/sysrib/route_view.go` - Owner daemon adapter, ordered session, snapshots and output ledger.
- `internal/component/sysrib/route_view_client.go` - External read-only projection and local reader adapter.
- `internal/component/sysrib/register_route_view.go` - Register process operations and cleanup from the owner.
- `internal/component/sysrib/route_view_test.go` - Controlled ordering, fields, authority, resolver and lifecycle tests.
- `internal/component/sysrib/doctor_route_view.go` - Runtime view health declared through the system RIB's SDK doctor handler.
- `internal/component/sysrib/yang/ze-sysrib-api.yang` - Every new process operation with bounded input/output schema. Regenerate YANG glue.
- `internal/component/plugin/server/engine_op_register.go` - Generic registrar and startup/declaration tests. The registrar has no routing policy.
- `internal/component/plugin/server/startup_route_view_test.go` - Companion change for the preceding new file.
- `pkg/plugin/rpc/route_view.go` - Public value types and thin SDK operations.
- `pkg/plugin/sdk/sdk_route_view.go` - Companion change for the preceding new file.
- `internal/test/fixture/sysrib_parity_fixture.go` - Shared paired-mode and lifecycle assertions.
- `internal/test/fixture/register_sysrib_parity.go` - Companion change for the preceding new file.
- `test/plugin/` - Tracked daemon proof, including base and `-forked.ci` forms.
- `features/route-policy.md` - Feature scope, components, documentation, and retained evidence.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | Owner `ze-sysrib-api.yang` for every process operation. |
| YANG validation constraints | Yes | Session/cursor shape, bounds, protocol names, address/family values, tables and arrays. Validate again in the shared handler. |
| Custom validators | No | No new operator config leaf or external-run refusal. |
| CLI commands/flags and grammar | No | Existing operator paths and answer shapes remain. Transport operations carry YANG without adding operator placement restrictions. |
| Editor autocomplete | No | Existing configuration syntax remains. |
| Functional proof for RPC/API | Yes | Actual TLS subprocess and paired daemon/kernel carriers. |
| Pipe completeness | Yes | Existing candidates/RIB/nexthop/ECMP answer shapes remain data, with existing generic pipes. |
| Env var registration | No | Use existing SDK connection and identity. No new listener, port, or environment option. |
| Doctor dependency and failure checks | Yes | Runtime SDK check `rib-routing-view`, code `doctor-rib-routing-view-unavailable`, registered in `internal/core/diagnostic/codes.go`. Reports initialization, reset, lost authority and output failure. Recovery names `ze request plugin restart <configured-name>`. |
| Metrics | Yes | `ze_rib_view_resync_total`, `ze_rib_view_pending_prefixes`, `ze_rib_view_ready`, declared by the owner and published through existing SDK metrics. Labels never contain route prefixes or session tokens. |
| BGP family surface | N-A | No family, capability, or BGP attribute is added. |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File / section and change |
|---|----------|----------|---------------------------|
| 1 | Feature scope/evidence | Yes | `features/route-policy.md`, both execution modes and actual evidence, without claiming unrun proof. |
| 2 | Config syntax | No | Existing external and internal configurations remain accepted. |
| 3 | CLI changes | Yes | `docs/guide/command-reference.md`, candidate/nexthop commands answer with an external system RIB too. No syntax change. |
| 4 | API/RPC | Yes | `docs/architecture/api/process-protocol.md`, new methods, schemas, startup admission, cursor and acknowledgement rules. |
| 5 | Plugin | Yes | `docs/guide/plugins.md`, `docs/plugin-overview.md`, external system RIB example and lifecycle. |
| 6 | User guide | Yes | `docs/guide/route-policy.md`, equivalent placement and new proof. `docs/guide/static-routes.md`, named-table exclusion from the import attachment. |
| 7 | Wire | Yes | Process protocol page for the new value contract. Existing BGP and FIB payloads remain unchanged. |
| 8 | SDK | Yes | `docs/plugin-development/protocol.md`, `docs/plugin-development/handlers.md`, public methods and failure behavior. Existing plugin rules already require parity. |
| 9 | RFC | No new enrollment | Local policy and transport repair. Preserve existing RFC proofs, and refresh any record a changed tagged unit invalidates. |
| 10 | Tests | Yes | `docs/functional-tests.md`, `docs/architecture/testing/interop.md`, paired placement and kernel evidence. |
| 11 | Comparison | Yes | `docs/comparison.md`, per-protocol policy row and corrected redistribution example. |
| 12 | Architecture | Yes | `docs/architecture/rib/unified-locrib.md`, `docs/architecture/forked-route-install.md`, `docs/architecture/core-design.md`, `docs/architecture/system-architecture.md`. Explain consumer protocol, single authority, startup and output lifecycle. Preserve both-mode requirement in the doors. |
| 13 | Route metadata | No new key | Existing routing values and presence bits cross the new contract. |
| 14 | Metrics | Yes | `docs/plugin-development/metrics.md`, owner view health and resync metrics. |
| 15 | Inventory and discovery | Yes | `ai/INDEX.md`, `docs/guide/status.md`, owner-registered operations and routing-view health. Runtime inventories derive from registration. |
| 16 | Source anchors | Yes | Derived with `./le spec citation anchors spec plan/release/fixes/spec-route-policy-1-gate-proof-and-forked-sysrib.md`. Update `docs/architecture/api/ipc_protocol.md` for new public values while retaining its framing contract, and `docs/architecture/doctor-and-health-checks.md` for runtime diagnosis. Preserve `docs/architecture/route-selection.md`'s authoritative metric fallback. `docs/features/ai-first.md` is unaffected: diagnostic registration remains the source of codes and fixes. No `RunsInProcessOnly` or validator behavior changes. |
| 17 | Examples | Yes | Validate internal/external examples against loader and schema. Both must boot and install the permitted control. |

Documentation changes accompany their implementation phase. This design revision
records the owner requirement on the architecture page without claiming that
the new transport is already implemented.

## Implementation Steps

| Phase | Work and files | Required proof |
|-------|----------------|----------------|
| 1: wiring first | Owner YANG, public DTOs, generic engine-op registration, authenticated context, explicit startup-safe admission, shared SDK transport. Update the process/SDK pages in this phase. | Both-transport handler test and real external boot test. Prove distance configure does not deadlock or acknowledge a local-only write. |
| 2: selected view | Nonblocking handoff, ordered snapshot/deltas, bounded retention/reset, external read-only prefix projection, local reader. | Concurrent scan/withdraw and overflow interleavings, field round trips, local/direct allocation evidence. |
| 3: full routing behavior | Distance apply/rollback, shared resolver/cascade/ECMP, authoritative candidates, generation-bound output sink and replay. Remove the unsafe BGP path and obsolete arbitration. | Paired distance, candidates, recursive, metadata, permission and metric tests, preserving old assertions. |
| 4: lifecycle | Cleanup ledger, generation fence, failed view/output recovery, restart snapshot, runtime doctor and owner metrics. | Kill/replace/slow-consumer tests with kernel withdrawal/reinstallation and unaffected-route controls. |
| 5: import-chain daemon proof | Extend committed drivers, add connected proof and forked producer cases, draft/discriminate/load-test/promote all paired carriers. | AC-6 and AC-8..AC-13 through a real daemon and FIB. Verify every promoted path is tracked. |
| 6: routing peers and declaration | OSPF/BIRD and IS-IS/FRR placement runs, feature declaration, comparison and remaining source-backed page updates. | Retained original scenario assertions plus new parity checks, recorded red/green, feature report and scoped documentation validation. |

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Useful external behavior | An actual subprocess installs permitted routes. A rejection, empty table, local test object, or internal replacement cannot satisfy parity. |
| Single authority | Projection contains selected values only. No second gate, selector, Loc-RIB singleton, or copied chain runs externally. |
| Startup | Nested distance operations have an admitted request consumer before configure waits. All other handshake barriers and refused calls keep their behavior. |
| Concurrency | No transport under shard locks. Snapshot reconciliation and output admission exclude stale generation/cursor writes. |
| Complete process path | Input, distance publication, candidate reads, FIB output, replay subscription, health, and cleanup all cross the real transport. |
| Metadata | Every field consumed by current resolver/FIB/replay is carried, including presence, scoped devices, ECMP and repair. |
| Test strength | Paired semantic assertions and independent judgment for covered test changes. No golden refresh hides a placement difference. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Correct two-mode system RIB | Recorded paired daemon and kernel runs for AC-1..AC-25. |
| Registered public process contract | JSON and Direct handler tests, schema/registration check, non-Go JSON round-trip fixture. |
| Lifecycle safety | Controlled stale-generation interleavings plus real kill/restart and FIB replay runs. |
| Import proof | Tracked paired central/static/connected tests and retained OSPF/IS-IS placement records. |
| Declaration and comparison | Feature report, source-backed comparison row, scoped documentation checks. |
| No scope shortcut | Source review confirms no internal-only declaration, external refusal, secondary gate, or BGP fallback for the system RIB. |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Authority | Caller generation and configured registration decide the lease. Payload names cannot grant permission to change distance or publish FIB output. |
| Fail-open | Missing/reset view fences output. No missing-resolver success or unsafe input fallback. |
| Atomic validation | Entire distance tables and pages validate before mutation. Unknown protocol names are looked up, never registered from wire input. |
| Resource lifetime | Bounded pages/journals, cancellation/deadlines, released snapshots, and joined workers on every stop path. |
| Ownership | Withdrawal targets only the outgoing session ledger and preserves OS, withheld, and unrelated routes. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Missing field or reader in the process contract | Repair this design and the affected tests. Do not reduce parity. |
| Startup-safe admission cannot preserve barriers | Rework that integration before implementing consumers. Do not make external configuration invalid. |
| Functional test proves only a producer or injected bus | Repair placement and real transport setup. Keep semantic assertions. |
| Runtime or test defect related to this capability | Fix at its producer within this spec. |
| Unrelated defect | Follow the existing journal ownership and completion rules. |
| Three failed fixes | Report evidence and ask which technical repair to take. |

## Design Insights

- A broken execution mode needs the missing interfaces and proof. An internal-only declaration is not a substitute for that repair.
- A read-only selected projection preserves one admission and selection authority.
- A feed cursor, a selected-routing revision, and a candidate snapshot identity describe different state and cannot substitute for one another.
- External output and failure cleanup need explicit ownership just as external producer input does.
- Draft files are deliverables only after promotion and inclusion in git.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

The owner chose two-mode correctness. The following implementation design remains
at `design` for review. No implementation or completed proof is claimed.

| Decision | Alternatives considered | Rationale |
|----------|-------------------------|-----------|
| Preserve internal and external system RIB support | Refuse external mode and declare an architectural exception. | The owner rejected withdrawal of the execution mode. Repair all missing behavior. |
| One daemon authority, external selected projection | A second full Loc-RIB/policy engine externally; a proxy returned by `locrib.Default()`. | A projection has no admission or selector. The core singleton and existing producer sink keep their contracts. |
| Local reader and external projection share forwarding algorithms | A remote LPM RPC for every recursive step; moving all system RIB work into a daemon helper behind an inert child. | Batched values allow local recursive resolution and dependency tracking in the selected execution process. A dummy subprocess does not prove external execution. |
| Owner-registered process-bound engine operations | Proc-less raw codec handlers; a list of system RIB methods in shared dispatch; assuming builtin command registration wires raw calls. | Existing dispatch paths show why those shortcuts lose identity or reachability. Extend registration with one handler definition for both transports. |
| Explicit startup-safe admission | Calling runtime RPC from configure and hoping the reader exists; publishing distances only after readiness without acknowledgement. | The current handshake establishes the deadlock risk. Distance application must reach the authority and report its real outcome. |
| Fenced, acknowledged output with scoped cleanup | Generic best-change emission without generation ownership; trusting producer-disconnect cleanup to remove consumer output. | The old cleanup tracks route-install producers. New system output requires its own ledger and replacement ordering. |
| Candidate reads query full authority | Cache candidates under selected revision; inspect only the selected projection. | Hidden-only edits do not advance selected revision and a selected projection cannot show hidden candidates. |
| Keep B-1 and B-3 with B-2 | Drop daemon proofs or split the owner's three findings again. | The owner requested one spec. The tests and declarations prove the capability the repair restores. |

Independent design admission review on 2026-11-12 reported CLEAN after the
bootstrap-session ordering and VPP subscriber census corrections. This is a
review of the proposed design, not implementation or test evidence.

## Known Limitations

No execution-mode exception or deferred system RIB parity is permitted.
Existing forked-producer graceful-restart retention remains a separate
producer-lifecycle limitation described in `docs/architecture/forked-route-install.md`.
This consumer transport repair does not claim to add that feature.

## RFC Documentation (Scope: protocol)

No new RFC requirement is introduced by the local policy/process transport.
Existing tagged tests retain their proof obligations when touched.

## Checklist

### Pre-Spec Verification

- [ ] Valid metadata, owner requirement link, revision history, and unchanged umbrella finding ownership.
- [ ] Existing mechanisms, source-derived census, positive AC outcomes and survivor reasons.
- [ ] Two-mode wiring, complete DTO/lifecycle contract, risks and assumption validation methods.
- [ ] Every implementation phase names its tests and accompanying documentation.
- [ ] No code snippets or completed-test claims.

### Goal Gates (MUST pass)

- [ ] AC-1..AC-25 demonstrated through the stated entry points.
- [ ] All paired daemon/kernel and named peer proofs retained, with discriminating red runs.
- [ ] Both internal and external modes remain accepted and useful.
- [ ] No unsafe BGP fallback, duplicate gate, inert external facade, or external-run refusal.
- [ ] Every assumption validated and every relevant test change independently judged.
- [ ] Scoped package evidence recorded with `./le test unit pkg path ./internal/component/sysrib/`, plus the affected plugin-server, SDK, and core reader package tests.
- [ ] Named paired cases in Functional Tests run through `./le test functional plugin`; schema and documentation evidence recorded. No agent runs any `./le verify` command.

### TDD

- [ ] Tests written.
- [ ] Tests FAIL for the claimed behavior, with the observed output retained.
- [ ] Tests PASS after the repair, with the observed output retained.
- [ ] Boundaries, concurrency, overflow, disconnect/restart and rollback covered.
- [ ] Every functional draft promoted and tracked.

### Closure

- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and finish the independent Review Gate.
- [ ] Commit A preserves implementation, proof, pages and edited spec.
- [ ] Commit B removes the spec through the same generated `./le commit create` script.
