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
| Updated | 2026-11-26 |

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
dump, gets only the dump it asked for, never retains a route withdrawn concurrently
during the dump, and is not left waiting forever if the RIB dies.

`docs/guide/bmp.md` was corrected to say what happens today, in the same piece of
work that wrote this spec. It is corrected back when this spec lands.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` "Redistribute Late-Join Replay" - the shared replay vocabulary and replay-vs-incremental emit ordering
  → Decision: a targeted hop uses an opaque token that the emitter maps back to one consumer, and the producer orders replay emits against incremental emits with an RWMutex (`redistEmitMu` on the redistribute hop). The `bgp-rib` hop is still described as broadcast-only there, though BMP already puts a targeted token on it; this spec brings the wire codec and the page into line and mirrors `redistEmitMu` as `bestChangeEmitMu` on `(bgp-rib, best-change)`.
  → Constraint: `internal/core/replay` stays a leaf that spells no protocol concept.
- [ ] `docs/architecture/plugin/plugin-system.md` "The process boundary", "Communication patterns"
  → Constraint: the fix must hold when `bgp-rib` runs forked, and its wire contract must carry no in-process-only assumption for `bgp-bmp` (per the 2026-11-24 DQ-3 ruling in `plan/release/fixes/spec-bmp-late-attach-and-forked.md`). The EventBus is the one-to-many route, and nothing may rely on delivery running inside the emit.
- [ ] `docs/plugin-development/protocol.md`, `docs/architecture/api/process-protocol.md`
  → Constraint: a new event type (`bgp-rib/replay-complete`) and a new JSON member (`replay-id` on `best-change`) are a published contract, changed on purpose, declared in `bgp-rib`'s `Sends` and namespace registration, and documented with the page.

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
- `replayBestPaths` currently holds no emit-ordering lock against concurrent `publishBestChanges` callers (`rib_aigp.go`, `rib_commands.go`, `source_policy.go`, `selection_deferral.go`, `rib_structured.go`), unlike `replayRedistribute` which holds `redistEmitMu`. Mirroring `redistEmitMu` as `bestChangeEmitMu` prevents a concurrent withdrawal from landing before a replay Add.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/bmp/bmp_locrib.go` - `emitReplayRequest` publishes `dumpScope`, emits, retracts, then runs `closeDumpFamilies`. `handleBestChange` claims a batch only while the scope with its token is published, and sends End-of-RIB per claimed batch. `closeDumpFamilies` sends the markers no batch sent. Its own comment says the sweep "can run before the batches arrive"
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - `BMPPlugin` holds a single `dumpScope atomic.Pointer[dumpScope]` guarded by `dumpMu`, a single `locRIBUnsub` handle, and `stopCh`
- [ ] `internal/component/bgp/plugins/bmp/locrib_runtime.go` - `validateLocRIBRuntimeConfig` currently refuses `loc-rib true` when `getEventBus() == nil` until `plan/release/fixes/spec-bmp-late-attach-and-forked.md` wires the RPC/event rail for forked `bgp-bmp`
- [ ] `internal/component/bgp/plugins/bmp/sender.go`, `sender_drain.go`, `bmp_replay.go` - `clearConnAndResetIf` atomically clears `ss.conn` and resets the transmit queue only if the connection has not been replaced; `abortAdjReplay` resets the session without sending End-of-RIB or Termination
- [ ] `internal/component/bgp/plugins/rib/rib.go`, `register.go` - `AddStartupSubscriptionsIn(ribevents.Namespace, ...)` takes `EventReplayRequest` as a plugin subscription, in-process and forked alike. `ribBusSends()` and `events.RegisterNamespace(ribevents.Namespace, ...)` list every event `bgp-rib` emits
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `replayBestPaths` runs `replayFlowSpecs`, then emits one batch per non-empty family and stamps `req.ReplayID`. It emits nothing else, returns silently when the bus is nil, and holds no lock ordering its `BestChange.Emit` calls against concurrent `publishBestChanges` callers (unlike `replayRedistribute` which takes `redistEmitMu.Lock()` while `publishBestChanges` takes `redistEmitMu.RLock()` around `bgpredist.EmitBestChange`)
- [ ] `internal/component/bgp/plugins/rib/rib_event_sdk.go` - `sdkEventBus.Emit` cuts an oversized batch into ordered chunks under one `emitMu`, preserving all header fields verbatim with a `30 * time.Second` per-frame deadline (`sdkEventEmitTimeout`). `dispatchReplayRequest` decodes the request and calls `replayBestPaths`
- [ ] `internal/core/bgp/ribevents/ribevents.go` - `BestChangeBatch.ReplayID` is `json:"-"`. `MarshalJSON` writes only the `replay` boolean, and `UnmarshalJSON` turns a true one into `replay.Broadcast`, so the token does not survive JSON marshaling
- [ ] `internal/core/replay/replay.go` - `Request`, `Broadcast`, `IsReplay`
- [ ] `internal/component/plugin/process/delivery.go` - `Process.Deliver` queues and returns
- [ ] `internal/core/events/typed.go`, `internal/component/plugin/server/engine_event.go` - `Event.Emit` returns the number of plugin-process subscribers delivered; engine subscribers run synchronously
- [ ] `internal/plugins/redistribute/replay.go` - the targeted hop's coordinator: token to target, `replayTTL` (`30 * time.Second`), hard cap, and `removed` map
- [ ] `internal/component/bgp/plugins/bmp/bmp_reconnect_test.go`, `rfc9069_bmp_locrib_test.go`, `rfc9069_verdict_test.go` - each stand-in RIB subscribes `ribevents.ReplayRequest` with `Subscribe` and answers inside the emit; `locRIBTestBus.Emit` always returns `0, nil`
- [ ] `test/plugin/bmp-locrib.ci` - the real-RIB path; asserts a valid Route Monitoring and exit status, and no order against End-of-RIB

**Behavior to preserve:**
- Every family in `dumpFamilies` gets a marker, also when the table holds no route for it.
- A replay another subsystem requested (`flowspec-firewall` or any `replay.Broadcast` emitter) still reaches every collector as Route Monitoring and closes no family.
- The `replay` boolean stays on the JSON of `BestChangeBatch`, for any external reader.
- `handleBestChange` never blocks on I/O: it writes into each session's bounded queue.
- Peer Up precedes every Route Monitoring, on each session.

**Behavior to change:**
- A family closes when the RIB emits `bgp-rib/replay-complete` for that dump token, never when `ReplayRequest.Emit` returns (except when zero plugin-process RIB subscribers received the request).
- A claimed batch reaches only the scope's collector, both in-process and when `bgp-rib` runs forked and chunks batches over JSON.
- `replayBestPaths` and `publishBestChanges` serialize their `(bgp-rib, best-change)` emits with `bestChangeEmitMu` so a concurrent withdrawal cannot overtake a replay Add.
- A dump that stays silent past `locRIBDumpIdleTimeout` (`30 * time.Second`) ends the collector session without sending End-of-RIB or Termination.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Replay correlation token | `internal/core/replay` `Request`; `ribevents.BestChangeBatch.ReplayID` | One opaque token on every replay hop, echoed on the batch | Extends: `BestChangeBatch` JSON carries `replay-id` beside `replay`, and the completion event carries the same token in the same `replay.Request` shape |
| Typed events and namespace sends | `internal/core/events` `Register`; `ribevents` handles; `rib/register.go` `ribBusSends` | Declares a namespace, type, payload, and plugin send authorization reachable in-process and forked | Extends: adds `ribevents.ReplayComplete`, registers `EventReplayComplete` in the `bgp-rib` namespace, and lists `bgp-rib:replay-complete` in `ribBusSends()`. No new channel between plugins |
| Replay-vs-incremental emit lock | `internal/component/bgp/plugins/rib/rib_bestchange.go` `redistEmitMu` | Orders `replayRedistribute` (exclusive lock from `collectBestPaths` through emits) against `publishBestChanges`'s `bgpredist.EmitBestChange` (shared lock) | Extends the same pattern: adds `bestChangeEmitMu` ordering `replayBestPaths` (from `collectBestPaths` through `ReplayComplete.Emit`) against `publishBestChanges`'s `ribevents.BestChange.Emit` |
| Forked RIB bus | `rib_event_sdk.go` `sdkEventBus` | Hands events to the engine in order, under `emitMu`, cutting oversized batches into chunks while keeping header metadata verbatim | Extends unchanged: each chunk keeps `replay-id` from `BestChangeBatch.MarshalJSON`, and `ReplayComplete` follows the last chunk on the same bus under `emitMu` |
| Plugin startup subscriptions | `rib.go` `AddStartupSubscriptionsIn` | What the RIB hears, on both transports | Does not apply as a change: the RIB already hears `replay-request` on both transports. `bgp-bmp`'s forked `OnEvent`/subscription transport is owned by `plan/release/fixes/spec-bmp-late-attach-and-forked.md` |
| Dump scope and sweep | `bmp.go` `dumpScope`; `bmp_locrib.go` `emitReplayRequest`, `closeDumpFamilies` | Publishes one scope for the length of the emit, then sweeps | Replaces the window: scopes live in a map keyed by token until completion or `locRIBDumpIdleTimeout` (`30 * time.Second`). `closeDumpFamilies` stays and moves to the completion handler. The per-batch marker in `handleBestChange` goes: chunks made it unsound |
| Token table with a bound | `internal/plugins/redistribute/replay.go` `replayCoordinator` (`replayTTL = 30s`, hard cap) | Token to target, evicted by time and cap, for a late answer | Extends the design, not the code: BMP's scope table uses the same 30s idle window (`locRIBDumpIdleTimeout`), bounded by active collector sessions, with one long-lived worker rather than per-dump goroutines |
| Session reset on failure | `sender.go` `clearConnAndResetIf`; `sender_drain.go` overflow reset; `bmp_replay.go` `abortAdjReplay` | Closes the active connection and drops its queue without sending End-of-RIB or Termination, leaving a newer connection untouched | Extends: adds a `senderSession.resetSession` helper using `clearConnAndResetIf` so dump-deadline expiry takes the same safe exit and the collector reconnects cleanly |
| Test stand-in RIB | `bmp_reconnect_test.go`, `rfc9069_bmp_locrib_test.go`, `rfc9069_verdict_test.go` | Answers synchronously inside the emit, on a bus returning 0 subscribers | Replaces: a queued stand-in that delivers batches and `ReplayComplete` after the emit returns, on a test bus reporting subscriber count accurately |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | `bgp-rib` emits `best-change` (with JSON `replay-id`) and `replay-complete` over the EventBus/RPC rail identically internal and forked, declares `bgp-rib:replay-complete` in `ribBusSends()`, and BMP's scope/completion logic depends only on self-contained event payloads so `spec-bmp-late-attach-and-forked.md` can drive the same handlers over forked RPC without changing the RIB or wire contract |
| `rfc/short/rfc9069.md` | Compliance Checklist | the `// Design:` page of `internal/component/bgp/plugins/bmp/bmp_locrib.go` | "[RFC9069-x-4] [MUST] Route Monitoring messages are used for initial synchronization of the Loc-RIB. (§5.4, Route Monitoring)" | agrees | Initial Loc-RIB synchronization remains Peer Type 3 Route Monitoring preceded by the Loc-RIB Peer Up, and End-of-RIB markers now follow every dumped route after `replay-complete` |
| `ai/doors/bgp-plugins.md` | Where it sits | the door covering `internal/component/bgp/plugins/bmp/*` and `internal/component/bgp/plugins/rib/*` | "A plugin runs internal (a goroutine) or external (a forked subprocess over TLS RPC), and the SDK hides which" | agrees | `bgp-rib` runs the same `replayBestPaths` path internal and forked via `sdkEventBus`, declares `bgp-rib:replay-complete` in `ribBusSends()`, and neither plugin imports or calls the other directly |
| `docs/architecture/plugin/rib-storage-design.md` | Validation eligibility | the `// Design:` page of `internal/component/bgp/plugins/rib/rib_bestchange.go`; anchored to `rib_bestchange.go`; modified by this spec | "What `bgp-rib` publishes leaves it the same way in either process." | agrees | `bgp-rib/replay-complete` leaves `bgp-rib` through the same daemon bus or `sdkEventBus` path as `bgp-rib/best-change`, and `rib-storage-design.md` is updated to name `bgp-rib/replay-complete` and `bestChangeEmitMu` |
| `docs/architecture/buffer-architecture.md` | When a copy is deliberate | anchored to `internal/component/bgp/plugins/rib/rib_bestchange.go` | "Oversized best-change and redistribution arrays reuse their encoded entries inside independently framed payloads" | agrees | `sdkEventBus.Emit` keeps the same single-marshal chunk splitting and copies the header verbatim onto every chunk, preserving `replay-id` without extra per-route copies |
| `docs/architecture/core-design.md` | Redistribute Late-Join Replay | anchored to `internal/component/bgp/plugins/rib/rib_bestchange.go`; `// Design:` of `bmp.go` and `sender.go`; modified by this spec | "The `bgp-rib` and `system-rib` hops are the BROADCAST case (the token is `replay.Broadcast`, the handler ignores it and walks its whole table for every subscriber); the `redistribute` hop below is the TARGETED case, where the token addresses one new peer." | departs | `bmp_locrib.go` already puts a per-dump token on `bgp-rib/replay-request` in-process, and this spec makes `replay-id` survive `best-change` JSON marshaling and adds `bgp-rib/replay-complete` carrying `replay-id` so targeted per-collector dumps work across both transports (while `replay.Broadcast` stays intact for broadcast requesters like `flowspec-firewall`). `core-design.md` is updated to document `bgp-rib` as supporting both `replay.Broadcast` and targeted correlation tokens, plus `bestChangeEmitMu` mirroring `redistEmitMu` |
| `docs/architecture/edge-cases/extended-message.md` | Invalid lengths and cleanup | anchored to `internal/component/bgp/plugins/rib/rib_bestchange.go` | | silent | Anchored to `rib_bestchange.go` via `emitPurgedWithdraws` for RFC 8654 fatal-length cleanup; silent on Loc-RIB replay correlation, `best-change` JSON serialization, `replay-complete`, and BMP End-of-RIB ordering |
| `docs/architecture/rib-transition.md` | Stable (API Contract) | anchored to `internal/component/bgp/plugins/rib/rib_bestchange.go` | "Stable - additive changes only" | agrees | Adding optional `replay-id` beside the preserved `replay` boolean on `best-change` JSON and registering the new `bgp-rib/replay-complete` event are purely additive JSON contract changes |
| `docs/architecture/rib/forward-handle.md` | The decisions | anchored to `internal/component/bgp/plugins/rib/rib_bestchange.go` | | silent | Covers `locrib.ForwardHandle` zero-copy wire access and the fast-path tracker; silent on `replayBestPaths`, `best-change` JSON replay tokens, `replay-complete`, and BMP Loc-RIB dumps |
| `docs/architecture/route-selection.md` | Candidate Extraction | anchored to `internal/component/bgp/plugins/rib/rib_bestchange.go` | | silent | Covers BGP Phase 1 validation, Phase 2 best-path ranking, and AIGP session policy; silent on Loc-RIB dump correlation, `replay-complete`, and BMP End-of-RIB ordering |
| `docs/architecture/api/process-protocol.md` | Batched Delivery | the `// Design:` page of `internal/core/bgp/ribevents/ribevents.go`; modified by this spec | "Each chunk keeps the original event shape and replay metadata, and each chunk carries a deadline of its own: a dead engine connection fails the hand-off within one chunk's deadline, while a live engine acknowledging a whole family chunk by chunk is never cut off by the family's size." | agrees | Every chunk of a forked `best-change` batch keeps both `replay` and `replay-id` in its header, BMP's 30s idle deadline restarts on every claimed chunk to match `sdkEventEmitTimeout`, and `process-protocol.md` is updated to document `replay-id` and `bgp-rib/replay-complete` |
| `ai/doors/bgp-engine.md` | Who depends on it from outside | the door covering `internal/core/bgp/ribevents/ribevents.go` | "A change to an event field, a keyword of `update text`, a command word or an RPC field breaks code that is not in this repository. Change it on purpose, with its page, and check the ExaBGP bridge in the same change." | agrees | `replay` stays unchanged on `best-change` JSON, `replay-id` and `bgp-rib/replay-complete` are additive and documented in `docs/architecture/api/process-protocol.md`, and the ExaBGP bridge does not decode `bgp-rib/best-change` |
| `docs/guide/bmp.md` | Every Connection Is a Fresh BMP Session | the `// Design:` page of `internal/component/bgp/plugins/bmp/rfc9069_verdict_test.go` | "so this order does not meet the RFC. The fix is `plan/release/fixes/spec-bmp-locrib-dump-end-of-rib-before-routes.md`." | agrees | This spec implements the exact RFC 7854 Section 5 ordering fix that `docs/guide/bmp.md` points to, and updates those paragraphs to describe the corrected order and idle-deadline reset |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `test/plugin/bmp-locrib.ci` and `test/plugin/bmp-locrib-forked-rib.ci` | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>` on the functional runner" | agrees | The updated `test/plugin/bmp-locrib.ci` and new `test/plugin/bmp-locrib-forked-rib.ci` follow the `.ci` harness layout under `test/plugin/` and run through `./le test bgp plugin`, while unit runs pass `-tags 'ze_bgp ze_bmp'` |
| `docs/architecture/api/birdwatcher-compat.md` | 5. Protocol object | anchored to `internal/component/bgp/plugins/bmp/bmp.go` | | silent | Anchored to `bmp.go` via `bmpCompositeKey` for Looking Glass `protocols/bmp` peer keys; silent on BMP sender Loc-RIB dump completion, `replay-complete`, and `best-change` JSON tokens |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to `internal/component/bgp/plugins/rib/register.go` | | silent | Anchored to `rib/register.go` via `init` for `bgp-rib` doctor check code registration; silent on `replayBestPaths`, `bgp-rib/replay-complete`, and BMP Loc-RIB dump correlation |
| `docs/architecture/plugin/plugin-relationships.md` | Cross-Plugin Bus Dependencies | anchored to `internal/component/bgp/plugins/rib/register.go` | "These are runtime dependencies expressed through bus subscriptions, distinct from startup dependencies in the `Dependencies` field." | agrees | `bgp-bmp` and `bgp-rib` coordinate Loc-RIB dumps solely through EventBus events (`replay-request`, `best-change`, `replay-complete`) declared in `ribBusSends()`, adding no startup `Dependencies` coupling |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec touches only BGP RIB and BMP event delivery and neither configures nor accepts any kernel-only data-plane feature under VPP |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec adds no configuration validation path or start-only bypass for kernel/VPP mismatches (and agrees with the 2026-11-26 supersession at start below) |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec touches neither VPP firewall verification nor YANG `ze:backend` annotations |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec adds or renames no data-plane selector, backend registration, or YANG annotation |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec touches only BMP Loc-RIB dump correlation and completion ordering, leaving BGP egress next-hop rewriting and export chains unchanged |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec does not modify `policy { modify }`, `policychain.Sets`, or `next-hop:*` chain references |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec adds no filter insertion or policy chain step and leaves `next-hop:*` compiled into `policychain.Chain` |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec does not alter export-chain folding or `wireOverride` next-hop provenance |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec does not alter import or export next-hop policy evaluation or validation |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec does not alter next-hop handling for forwarded or locally originated routes |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec introduces no automatic filter-chain member and leaves the implied-filter seam untouched |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec adds no startup exit or refusal path; a silent or dead RIB during a Loc-RIB dump resets only the affected BMP collector session after the 30s idle deadline, leaving `ze` and all other services running |
| `docs/architecture/core-design.md` | Design decisions | `docs/architecture/core-design.md`, decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec touches no kernel/VPP mismatch handling at start, commit, reload, or validate |
| `docs/architecture/api/process-protocol.md` | Design decisions | `docs/architecture/api/process-protocol.md`, decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | `bgp-rib` declares `bgp-rib:replay-complete` in `ribBusSends()` and registers `EventReplayComplete` in `events.RegisterNamespace(ribevents.Namespace, ...)` so its compiled Stage 1 send profile authorizes emitting `replay-complete` over `emit-event` when forked |
| `docs/architecture/api/process-protocol.md` | Design decisions | `docs/architecture/api/process-protocol.md`, decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec introduces no lossy or drop-on-full policy on engine-to-plugin event delivery; BMP's collector-facing TCP transmit queue remains separate from the plugin event bus and resets the collector session rather than dropping messages |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A collector session comes up, or `loc-rib` turns on: `requestLocRIBDump`, `startLocRIB`.
- Format: a `*replay.Request` on `(bgp-rib, replay-request)`, with a monotonic token BMP allocated (`1` to `MaxUint64 - 1`).

### Transformation Path
1. BMP records a scope in its token-keyed scope table (`token -> {session, conn, deadline = now + locRIBDumpIdleTimeout}`) where `locRIBDumpIdleTimeout = 30 * time.Second` (overridable in unit tests via unexported `BMPPlugin.dumpIdleTimeout`), wakes its single long-lived dump-deadline worker, and emits `(bgp-rib, replay-request)`.
2. If `ReplayRequest.Emit` returns an error, BMP removes the scope and returns the error. If `ReplayRequest.Emit` returns `0` plugin-process subscribers (and no completion arrived synchronously), BMP removes the scope and runs `closeDumpFamilies` at once because no RIB subscriber exists to answer.
3. Otherwise, the request queues for `bgp-rib` through `Process.Deliver`.
4. `replayBestPaths` runs `replayFlowSpecs()`, checks `eb != nil && req != nil`, acquires `bestChangeEmitMu.Lock()` (ordering the whole replay ahead of any incremental `publishBestChanges` emit on `(bgp-rib, best-change)`), walks `collectBestPaths()`, emits each family's `BestChangeBatch` carrying `req.ReplayID` (serialized as both `replay: true` and `replay-id: N` in JSON), emits `(bgp-rib, replay-complete)` carrying `&replay.Request{ReplayID: req.ReplayID}` even when the table is empty, and releases `bestChangeEmitMu.Unlock()`.
5. `handleBestChange` looks up `batch.ReplayID` in BMP's scope table:
   - Known token: writes Route Monitoring only to that scope's collector session (or all current sessions if `scope.session == nil`), and restarts that scope's deadline to `now + locRIBDumpIdleTimeout`. Never sends End-of-RIB per batch.
   - `replay.Broadcast`: fans out Route Monitoring to every connected collector and closes no family.
   - Unknown non-broadcast token (e.g. from an expired dump): drops with a warning so stale dump routes never leak to other collectors.
6. `handleReplayComplete` removes the matching scope from BMP's scope table and runs `closeDumpFamilies` for every family in `dumpFamilies`, after the last Route Monitoring. An unknown token (e.g. a broadcast replay requested by another subsystem) is ignored.
7. If a scope reaches its idle deadline with no claimed batch and no `replay-complete`, the single dump-deadline worker removes the scope, logs a warning naming the collector and idle timeout, and resets the scope's collector session via `senderSession.resetSession` (`clearConnAndResetIf`) with no End-of-RIB and no Termination.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BMP to RIB | `(bgp-rib, replay-request)` event, `replay.Request` JSON `{"replay-id":N}` | Yes: `dispatchReplayRequest` reads the token on both transports |
| RIB to BMP, batches | `(bgp-rib, best-change)`; typed in-process, JSON when forked (split into ordered chunks under `sdkEventBus.emitMu`) | Yes: `BestChangeBatch.MarshalJSON` currently drops the token, which this spec fixes by adding `replay-id` beside `replay` |
| RIB to BMP, completion | `(bgp-rib, replay-complete)`, payload `replay.Request` JSON `{"replay-id":N}`, declared in `ribBusSends()` | No: proven by the unit, forked, functional, and interop tests below |
| BMP to collector | BMP Route Monitoring, in session transmit-queue order | Yes: the session's bounded queue keeps order under `writeMu` |

### Changed-Wire Census

**Search evidence:** scope `internal`, `pkg`, `test`, `cmd`, `docs/architecture/api`. Terms: `"replay"`, `replay-id`, `BestChange`, `best-change`, `EventReplayRequest`, `ribBusSends`. Control: `BestChangeBatch.MarshalJSON` is found as the writer, `rib/register.go` `ribBusSends` as the RIB send declaration, and the `ribevents.BestChange.Subscribe` sites in sysrib, fib, flowexport and BMP as readers. No Python or shell reader decodes `best-change`. The `meta["replay"]` hits in `rib_replay.go` and `update_text.go` are route metadata, another wire.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `best-change` JSON | writer | `ribevents.go` `BestChangeBatch.MarshalJSON` | writes `replay` from the token | Change: adds `replay-id` (`omitempty`) beside `replay` |
| `best-change` JSON | reader | `ribevents.go` `BestChangeBatch.UnmarshalJSON` | a true `replay` becomes `replay.Broadcast` | Change: reads `replay-id`; without it, a true `replay` stays `replay.Broadcast` for an external writer |
| `best-change` typed | writer | `rib_bestchange.go` `replayBestPaths`, `publishBestChanges` | emits `*ribevents.BestChangeBatch` carrying `req.ReplayID` on replay and `0` on incremental | Change: adds `bestChangeEmitMu` ordering `replayBestPaths` against concurrent incremental `publishBestChanges` emits |
| `best-change` typed | reader | `bmp_locrib.go` `handleBestChange`; sysrib, `fibkernel`, `fibp4`, `fibvpp`, `flowexport`, `redistribute/producer.go` | BMP matches `ReplayID` against its dump scope; other consumers read `IsReplay` or the batch and never compare the token | Change in BMP (scope-table lookup, no per-batch End-of-RIB); preserve in all other readers (`IsReplay` answers the same for any nonzero token) |
| `replay-complete` | writer | `rib_bestchange.go` `replayBestPaths`; `rib/register.go` `ribBusSends` | new | New: `{"replay-id":N}` emitted under `bestChangeEmitMu` after the last batch, declared in `ribBusSends()` |
| `replay-complete` | reader | `bmp_locrib.go` `handleReplayComplete` | new | New: closes `dumpFamilies` for a matching token; an unknown token is ignored because broadcast replays also complete |
| BMP messages to a collector | writer | `bmp_locrib.go` `handleBestChange`, `closeDumpFamilies` | order changes, bytes do not | Change of order: all dumped Route Monitoring precede End-of-RIB, proven against pmacct |
| BMP messages to a collector | reader | External BMP collectors (e.g. pmacct `pmbmpd`), Ze BMP receiver (`bmp.go` `handleSession` / `DecodeMsg`), and functional test `BMP-COLLECTOR` (`test/plugin/bmp-locrib*.ci`) | decode BMP stream and treat End-of-RIB as dump completion per RFC 7854 Section 5 | Preserve wire decoding; assert corrected message order (Peer Up -> dumped Route Monitoring -> End-of-RIB) in `bmp-locrib.ci`, `bmp-locrib-forked-rib.ci`, and `bmp-locrib-pmacct` |

### Integration Points
- `rib_bestchange.go` `replayBestPaths` and `publishBestChanges` - `bestChangeEmitMu` orders replay emits against concurrent incremental emits; `replayBestPaths` emits `ribevents.ReplayComplete` on every exit that answered a request.
- `rib/register.go` `ribBusSends` and `events.RegisterNamespace` - registers and authorizes `bgp-rib:replay-complete` for forked `bgp-rib`.
- `bmp.go` `BMPPlugin` and `bmp_locrib.go` `startLocRIB`, `stopLocRIB`, `emitReplayRequest`, `handleBestChange`, `handleReplayComplete`, `closeDumpFamilies` - token-keyed scope table, completion subscription, single idle-deadline worker.
- `sender.go` `resetSession` (`clearConnAndResetIf`) - safe collector session reset on dump idle-deadline expiry without sending End-of-RIB or Termination.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Request, batches, and completion all cross as registered EventBus events |
| No unintended coupling (components stay isolated) | Yes | `bgp-rib` does not know BMP; it echoes an opaque token for any requester |
| No duplicated functionality (extends existing, does not recreate) | Yes | See Existing Mechanisms; the emit window is deleted and replaced by the scope table, and `bestChangeEmitMu` mirrors `redistEmitMu` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Cold path; a replay already builds its batches and `sdkEventBus` keeps single-marshal framing |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `events.Register` in `ribevents` and `ribBusSends()` in `rib/register.go`, which already own this namespace |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `EventReplayRequest` and `replay-request` across `internal/`, `pkg/`, `cmd/`: the RIB startup subscription list names only what the RIB hears, `ribBusSends()` and `events.RegisterNamespace` register what the RIB emits, and the event type registry fills from `events.Register` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The forked RIB's `replay-complete` reaches an engine subscriber after its last `best-change` chunk | `sdkEventBus.Emit` holds `emitMu` and hands each frame to the engine in order before returning | A marker overtakes a chunk | `TestDumpForkedRIBChunksPrecedeCompletion` | unvalidated |
| A-2 | While `bgp-bmp` currently gates `loc-rib true` on an available EventBus (`locrib_runtime.go`), forked `bgp-bmp` over the RPC/event rail is a supported placement per the 2026-11-24 DQ-3 ruling in `plan/release/fixes/spec-bmp-late-attach-and-forked.md`; this spec's wire contract (`best-change` `replay-id` JSON member, `bgp-rib:replay-complete`, and `ribBusSends()`) carries no in-process-only assumption so forked `bgp-bmp` can consume the same events once `spec-bmp-late-attach-and-forked.md` wires `OnEvent` and external subscriptions | `plan/release/fixes/spec-bmp-late-attach-and-forked.md` DQ-3 (2026-11-24); `internal/component/bgp/plugins/bmp/locrib_runtime.go`; `ai/doors/bgp-plugins.md` | An in-process-only correlation hook or undeclared event would break when `spec-bmp-late-attach-and-forked.md` enables forked `bgp-bmp` | `TestBestChangeBatchTokenSurvivesJSON`, `TestDumpForkedRIBChunksPrecedeCompletion`, and `ribBusSends()` / `events.RegisterNamespace` coverage for `EventReplayComplete` | unvalidated |
| A-3 | When `ReplayRequest.Emit` returns 0 plugin-process subscribers, no RIB subscriber is present to answer, so closing the dump immediately is safe (including during Stage 2 startup before `bgp-rib` finishes Stage 5, because `StartPeers` runs in `OnPostStartup` after Stage 5 so no BGP routes exist yet and BMP's live `BestChange` subscription is already installed) | `internal/component/plugin/server/engine_event.go` `EmitEngineEvent`; `docs/architecture/api/process-protocol.md` "5-Stage Startup Protocol" (`StartPeers`) | A dump closes early when a RIB subscriber is attached and holding routes | `TestDumpWithNoRIBClosesAtOnce` and `test/plugin/bmp-locrib.ci` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An idle deadline shorter than a pause between slow chunks resets a healthy collector, which reconnects and loops | A collector reset log during a large table | `locRIBDumpIdleTimeout = 30 * time.Second` matches `sdkEventEmitTimeout` and `replayTTL`, restarts on every claimed batch, and `reconnectMin = 30 * time.Second` bounds reconnect retries |
| R-2 | Two collectors each ask, so the RIB walks the table twice | Load on reconnect storms | Accepted: dumps stay per-token. The RIB already walks once per request |
| R-3 | A batch from an expired token fans out to other collectors | A warning on an unknown non-broadcast token | Dropped with a warning: the session it was for is already reset |
| R-4 | A replay batch arrives after a concurrent incremental withdraw of a route it snapshotted, reviving the removed route on the collector | Verified in `rib_bestchange.go`: `replayBestPaths` holds no lock against concurrent `publishBestChanges` callers (`rib_aigp.go`, `rib_commands.go`, `source_policy.go`, `selection_deferral.go`, `rib_structured.go`) | Add `bestChangeEmitMu` (`sync.RWMutex`) in `rib_bestchange.go` mirroring `redistEmitMu`: `replayBestPaths` holds `Lock()` after `replayFlowSpecs()` across `collectBestPaths()`, `BestChange.Emit`, and `ReplayComplete.Emit`; `publishBestChanges` holds `RLock()` around `ribevents.BestChange.Emit`. Proven by `TestReplayBestPathsOrdersAgainstIncrementalWithdraw`. No `BestChange` engine subscriber calls back into `RIBManager` synchronously |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A collector sees a missing or late End-of-RIB, retains a withdrawn route, or is reset. BGP sessions and forwarding routes are not touched |
| How is it reverted? | Single commit revert; the new event and JSON member have no persistent state |
| Who else touches this path? | sysrib, flowexport, and the FIB backends read `best-change`. `plan/release/fixes/spec-bmp-late-attach-and-forked.md` (forked `bgp-bmp` and late-attach Peer Up) and `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` edit other BMP paths |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Collector connects with `loc-rib true` | → | `requestLocRIBDump`, `replayBestPaths`, `handleReplayComplete`, `closeDumpFamilies` | `test/plugin/bmp-locrib.ci`, extended to assert order through `BMP-COLLECTOR` |
| Same, against a real queued RIB | → | `emitReplayRequest` returns before any batch | `TestDumpMarkersFollowRoutesWithQueuedRIB` |
| Forked `bgp-rib` answers dump request | → | `sdkEventBus.Emit`, `BestChangeBatch.MarshalJSON`/`UnmarshalJSON`, `ReplayComplete`, `handleReplayComplete` | `test/plugin/bmp-locrib-forked-rib.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A collector connects to a router holding IPv4 and IPv6 best paths, through the real `bgp-rib` | It receives Peer Up, then every Route Monitoring of the dump, then End-of-RIB for IPv4 and IPv6 unicast | Incremental Route Monitoring may interleave: RFC 7854 Section 5 says "There is no requirement on the ordering of messages in the peer dumps" |
| AC-2 | The Loc-RIB holds IPv6 routes and no IPv4 route | Both markers arrive, each after the last IPv6 Route Monitoring | None: the RFC owes a marker for the empty family, and only completion can place it |
| AC-3 | The Loc-RIB is empty | Peer Up, then both markers, after the RIB's completion | None |
| AC-4 | Two collectors connect together | Each receives its own complete dump and its own markers, and neither receives the other's dump | Both still receive incremental changes |
| AC-5 | Another subsystem requests a `replay.Broadcast` replay during a BMP dump | Every collector receives those routes as Route Monitoring. No family closes because of it, and the BMP dump still closes on its own completion | Those routes are real, so they stay visible to every collector |
| AC-6 | `bgp-rib` runs forked and a family is cut into chunks | All chunks reach the scope's collector only, and the marker follows the last chunk after `replay-complete` | None |
| AC-7 | The RIB dies, or never answers, during a dump | After `locRIBDumpIdleTimeout` (`30 * time.Second`, shorter in unit tests via `dumpIdleTimeout`), the collector session resets with a logged reason, and no End-of-RIB was sent for that dump | The collector's reconnect then requests a fresh dump, as for any reset |
| AC-8 | A slow dump keeps producing batches past one idle-deadline length | Each claimed batch restarts the scope's idle deadline; the dump finishes and closes normally on `replay-complete` | None: the deadline measures silence, not total length |
| AC-9 | A route is withdrawn by an incremental election concurrently while `replayBestPaths` is running | The incremental withdrawal is emitted on `(bgp-rib, best-change)` after the replay's Add (or absent from the replay if removed before `collectBestPaths`), so the collector never ends up holding the withdrawn route | None: `bestChangeEmitMu` orders replay emits against incremental emits |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator points pmacct at a router with `loc-rib true` | collector connect -> request -> RIB queue -> batches -> completion -> markers | Interop `bmp-locrib-pmacct`, order asserted |
| 2 | A collector restarts while routes exist | session up -> dump for this session only | `test/plugin/bmp-locrib.ci`, reconnect step |
| 3 | An operator runs `bgp-rib` forked with `loc-rib true` | collector connect -> request -> forked RIB chunks (`replay-id`) -> `replay-complete` -> markers | `test/plugin/bmp-locrib-forked-rib.ci` |

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
| `TestReplayBestPathsOrdersAgainstIncrementalWithdraw` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | AC-9 / R-4: `bestChangeEmitMu` prevents a concurrent withdrawal from overtaking a replay Add | |
| `TestDumpForkedRIBChunksPrecedeCompletion` | `internal/component/bgp/plugins/rib/rib_event_sdk_test.go` | AC-6: chunks carry `replay-id` and precede `replay-complete` | |
| `TestBestChangeBatchTokenSurvivesJSON` | `internal/core/bgp/ribevents/ribevents_test.go` | Round trip of `replay-id`; a true `replay` without it stays `replay.Broadcast` | |

The stand-ins in `bmp_reconnect_test.go`, `rfc9069_bmp_locrib_test.go` and
`rfc9069_verdict_test.go` become queued and emit `ReplayComplete` (with
`locRIBTestBus.Emit` returning subscriber count). An RFC-tagged test among them
whose claim or proof changes owes a fresh `./le rfc record`, and any change that
could lower what it proves goes to `ze-test-judge`.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Dump token | 1 to MaxUint64-1 | MaxUint64-1 | 0 (incremental) | MaxUint64 (`replay.Broadcast`) |
| `dumpIdleTimeout` (internal field) | > 0 (defaults to `locRIBDumpIdleTimeout = 30s` when `<= 0`) | `locRIBDumpIdleTimeout` | `<= 0` falls back to `30s` | N/A (Go `time.Duration`) |

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
- `internal/component/bgp/plugins/bmp/bmp_locrib.go` - `locRIBDumpIdleTimeout` (`30 * time.Second`), scope table operations, `handleReplayComplete`, single idle-deadline worker, narrowing and expired-token drop; delete the emit window and per-batch marker
- `internal/component/bgp/plugins/bmp/bmp.go` - replace `dumpScope` atomic pointer and `dumpMu` on `BMPPlugin` with token-keyed `dumpScopes`, `locRIBCompleteUnsub`, `dumpIdleTimeout`, and deadline worker lifecycle hooks
- `internal/component/bgp/plugins/bmp/sender.go` - `resetSession` helper using `clearConnAndResetIf` to close a timed-out collector session without End-of-RIB or Termination
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - `bestChangeEmitMu` ordering `replayBestPaths` against `publishBestChanges`, plus `ribevents.ReplayComplete.Emit` at the end of `replayBestPaths` on every exit that answered a request
- `internal/component/bgp/plugins/rib/register.go` - add `ribevents.EventReplayComplete` to `ribBusSends()` and `events.RegisterNamespace(ribevents.Namespace, ...)`
- `internal/core/bgp/ribevents/ribevents.go` - `EventReplayComplete` constant, `ReplayComplete` typed handle, and `replay-id` JSON member in `BestChangeBatch.MarshalJSON`/`UnmarshalJSON`
- `internal/component/bgp/plugins/bmp/bmp_reconnect_test.go`, `rfc9069_bmp_locrib_test.go`, `rfc9069_verdict_test.go` - queued stand-in RIB emitting `ReplayComplete`
- `test/plugin/bmp-locrib.ci` - order assertion
- `docs/guide/bmp.md` - the Loc-RIB paragraphs state the corrected order and idle-deadline reset
- `docs/architecture/api/process-protocol.md` - document `bgp-rib/replay-complete` and `replay-id` on `bgp-rib/best-change`
- `docs/architecture/core-design.md` - update `Redistribute Late-Join Replay` so `bgp-rib` reflects both `replay.Broadcast` and targeted correlation tokens with `replay-complete` and `bestChangeEmitMu`
- `docs/architecture/plugin/rib-storage-design.md` - add `bgp-rib/replay-complete` and `bestChangeEmitMu` to the RIB replay text

## Files to Create
- `internal/component/bgp/plugins/bmp/bmp_locrib_dump_test.go` - the BMP dump unit tests above
- `test/plugin/bmp-locrib-forked-rib.ci` - the forked-RIB functional test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config or RPC |
| YANG validation constraints | No | No leaf |
| YANG custom validators | No | No leaf |
| CLI commands/flags | No | None |
| CLI grammar (keyword before value) | No | None |
| Editor autocomplete | No | None |
| Functional test for new RPC/API | Yes | `test/plugin/bmp-locrib.ci`, `test/plugin/bmp-locrib-forked-rib.ci` |
| Pipe completeness | No | None |
| Env var registration | No | `locRIBDumpIdleTimeout = 30 * time.Second` is a constant, as `replayTTL` and `sdkEventEmitTimeout` are |
| Doctor check for runtime dependencies | No | No new dependency |
| Doctor code per problem class | No | Session reset on dump idle-deadline expiry logs a Warn naming collector and idle timeout (matching `sender_drain.go` queue-overflow reset) and triggers normal collector reconnect; no new doctor problem class |
| Prometheus counters/metrics | No | The reset logs its reason |
| BGP family surface (new SAFI / capability / attribute) | No | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Fix of an existing feature |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md`: `bgp-rib/replay-complete` and `replay-id` on `best-change` |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md`: replace the limit written on 2026-11-06 with the corrected order and idle-deadline reset |
| 7 | Wire format changed? | No | BMP bytes do not change; only message order changes |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7854.md`, `rfc/short/rfc9069.md` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (`Redistribute Late-Join Replay`), `docs/architecture/plugin/rib-storage-design.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md` if it lists `bgp-rib` event types |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Updated: `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/core-design.md`, `docs/architecture/api/process-protocol.md`, `docs/guide/bmp.md`. Checked via `./le spec citation anchors` and unaffected (all anchor to unrelated symbols in `rib_bestchange.go`, `rib/register.go`, `bmp.go`, or `sender.go`): `docs/DESIGN.md`, `docs/architecture.md`, `docs/architecture/api/birdwatcher-compat.md`, `docs/architecture/buffer-architecture.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/edge-cases/extended-message.md`, `docs/architecture/plugin/plugin-relationships.md`, `docs/architecture/rib-transition.md`, `docs/architecture/rib/forward-handle.md`, `docs/architecture/route-selection.md`, `docs/features/bgp-protocol.md`, `docs/features/plugins.md`, `docs/features/srv6.md`, `docs/guide/bgp-resilience.md`, `docs/guide/configuration.md`, `docs/guide/plugins.md`, `docs/guide/route-injection.md`, `docs/plugin-development/metrics.md`, `docs/plugin-development/schema.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Design presentation

### What changes

When `loc-rib true` is configured, a BMP collector now receives its Loc-RIB dump in RFC 7854 Section 5 order: Peer Up, then every dumped Route Monitoring message for that collector alone, and only then the IPv4 and IPv6 End-of-RIB markers. `bgp-rib` echoes the dump's correlation token (`replay-id`) on the JSON wire of `bgp-rib/best-change` (beside the preserved `replay` boolean), serializes replay emits against concurrent incremental withdrawals with `bestChangeEmitMu` (mirroring `redistEmitMu`), and emits a new `bgp-rib/replay-complete` event (`{"replay-id":N}`, declared in `ribBusSends()`) after its last batch even when the table is empty. BMP tracks active dumps in a token-keyed scope table with a 30-second idle deadline (`locRIBDumpIdleTimeout`) watched by one long-lived worker; each claimed batch restarts the deadline, completion closes every family in `dumpFamilies`, and silence past 30 seconds resets the collector session cleanly without sending a misleading End-of-RIB.

### Before and after

1. **Collector connects to a router holding IPv4 and IPv6 best paths (`loc-rib true`):**
   - Before: `emitReplayRequest` returns while `bgp-rib` still has the request queued, closes both families immediately, and sends IPv4 and IPv6 End-of-RIB markers BEFORE any dumped Route Monitoring arrives; the later batches match no scope and fan out to every connected collector.
   - After: `emitReplayRequest` leaves the token in BMP's scope table; `bgp-rib` emits all batches tagged with `replay-id` followed by `bgp-rib/replay-complete`; BMP delivers those batches only to the requesting collector and emits IPv4 and IPv6 End-of-RIB only on `replay-complete`.
2. **Forked `bgp-rib` dumps a large table cut into multiple JSON chunks:**
   - Before: `BestChangeBatch.MarshalJSON` omits `ReplayID` and `UnmarshalJSON` decodes every replay chunk as `replay.Broadcast`, so chunks are never scoped to the requesting collector and a per-batch End-of-RIB would fire after chunk 1.
   - After: `BestChangeBatch` JSON carries `"replay":true,"replay-id":N` on every chunk, BMP routes every chunk only to the matching scope's collector and restarts its 30s idle deadline, and End-of-RIB is emitted once when `bgp-rib/replay-complete` arrives after the final chunk.
3. **Concurrent route withdrawal during a Loc-RIB dump:**
   - Before: `replayBestPaths` holds no lock against concurrent `publishBestChanges` callers, so an incremental withdrawal can reach BMP before the replay batch carrying the snapshotted route, reviving a withdrawn route on the collector.
   - After: `bestChangeEmitMu` serializes `replayBestPaths` against `publishBestChanges` on `(bgp-rib, best-change)`, so the withdrawal either precedes `collectBestPaths` (absent from the replay) or is emitted after the replay's Add.
4. **RIB dies or stops answering mid-dump:**
   - Before: BMP already sent End-of-RIB before waiting, claiming an empty or complete dump that never happened.
   - After: After 30 seconds (`locRIBDumpIdleTimeout`) with no claimed batch and no `replay-complete`, BMP logs a warning and resets the collector session without End-of-RIB so the collector reconnects and requests a fresh dump.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Count batches per family to decide when a dump is done | `replayBestPaths` emits no batch for an empty family, so a count can never close an empty or mixed table, and a forked RIB splits one family into many chunks |
| Close families after a quiet period following the last batch | A quiet period is a timing guess that closes a slow or loaded RIB early with a false End-of-RIB |
| Replace the `replay` boolean on `best-change` JSON with `replay-id` only | Breaks external plugins already decoding the `replay` boolean; keeping `replay` alongside `replay-id` is purely additive |
| Fixed wall-clock dump timeout instead of an idle deadline | A million-route dump that is steadily producing chunks is healthy at any total duration; measuring silence (`locRIBDumpIdleTimeout = 30s`) aligns with `sdkEventEmitTimeout` and never aborts a progressing dump |
| Maintain a `removed` tombstone map in BMP instead of `bestChangeEmitMu` in `bgp-rib` | `rib_bestchange.go` already solves the identical race on the redistribution hop with `redistEmitMu`; mirroring it as `bestChangeEmitMu` fixes ordering at the single producer for all `(bgp-rib, best-change)` consumers (BMP, sysrib, FIB) without per-consumer state |

### Documents departed from

- `docs/architecture/core-design.md`, section `Redistribute Late-Join Replay`:
  - Current statement: "The `bgp-rib` and `system-rib` hops are the BROADCAST case (the token is `replay.Broadcast`, the handler ignores it and walks its whole table for every subscriber); the `redistribute` hop below is the TARGETED case, where the token addresses one new peer."
  - Why: `bmp_locrib.go` already allocates per-dump tokens on `bgp-rib/replay-request` in-process; this spec makes `replay-id` a JSON wire field on `bgp-rib/best-change` and adds `bgp-rib/replay-complete` carrying `replay-id` so targeted dumps work across both in-process and forked transports while keeping `replay.Broadcast` unchanged for broadcast requesters (`flowspec-firewall`).
  - After this work: `docs/architecture/core-design.md` describes `bgp-rib` as carrying either `replay.Broadcast` (broadcast late-join) or a per-request correlation token (targeted BMP Loc-RIB dump, closed by `bgp-rib/replay-complete` and ordered against incremental emits by `bestChangeEmitMu`).

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the handle, send declaration, emit lock, and JSON codec
   - Tests: `TestReplayBestPathsEmitsCompletionAfterBatches`, `TestReplayBestPathsOrdersAgainstIncrementalWithdraw`, `TestBestChangeBatchTokenSurvivesJSON`
   - Files: `ribevents.go`, `rib_bestchange.go`, `rib/register.go`
   - Verify: `replay-complete` is registered, declared in `ribBusSends()`, observable on the bus after `replayBestPaths` under `bestChangeEmitMu`, and BMP does not read it yet
2. **Phase: Scope table and close** -- BMP waits for completion
   - Tests: `TestDumpMarkersFollowRoutesWithQueuedRIB`, `TestDumpEmptyFamilyClosedOnCompletion`, `TestDumpScopeNarrowsToItsCollector`, `TestForeignReplayClosesNoFamilyDuringDump`
   - Files: `bmp.go`, `bmp_locrib.go`, the test stand-ins (`bmp_reconnect_test.go`, `rfc9069_bmp_locrib_test.go`, `rfc9069_verdict_test.go`). Delete the emit window and per-batch marker first (`ai/rules/no-layering.md`)
   - Verify: tests fail, implement, tests pass
3. **Phase: Deadline** -- a RIB that never answers
   - Tests: `TestDumpDeadlineResetsSessionWithoutMarker`, `TestDumpDeadlineRestartsOnBatch`, `TestDumpWithNoRIBClosesAtOnce`
   - Files: `bmp.go`, `bmp_locrib.go`, `sender.go`. `locRIBDumpIdleTimeout = 30 * time.Second` (injectable via `BMPPlugin.dumpIdleTimeout`), watched by one long-lived worker per `BMPPlugin`, never a goroutine per dump (`ai/rules/goroutine-lifecycle.md`)
   - Verify: tests fail, implement, tests pass
4. **Phase: Forked and end to end** -- the real paths
   - Tests: `TestDumpForkedRIBChunksPrecedeCompletion`, `test/plugin/bmp-locrib.ci`, `test/plugin/bmp-locrib-forked-rib.ci`, `bmp-locrib-pmacct`
   - Files: the `.ci` files, the lab assertion
   - Verify: revert the fix, rebuild, see red, restore, see green, and record it
5. **Phase: Pages** -- `docs/guide/bmp.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/core-design.md`, `docs/architecture/plugin/rib-storage-design.md`, the RFC summaries

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-9 has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No End-of-RIB is sent on any path that did not see the matching completion (except `n == 0` plugin-process subscribers) |
| Correctness | `replayBestPaths` emits completion on every path that received a request (nil-bus/nil-req excepted, covered by the deadline) and holds `bestChangeEmitMu.Lock()` across `collectBestPaths()` and all emits |
| Correctness | `ribBusSends()` and `events.RegisterNamespace` include `EventReplayComplete` so forked `bgp-rib` is authorized to emit it |
| Naming | `replay-id` kebab-case, the same member as `replay.Request`; `locRIBDumpIdleTimeout` = `30 * time.Second` |
| Data flow | BMP compares tokens only; `bgp-rib` holds no knowledge of who asked; no in-process-only assumption blocks forked `bgp-bmp` |
| Rule: stale comments | The comments in `emitReplayRequest`, `closeDumpFamilies`, `handleBestChange`, and `ribevents.go` that describe synchronous delivery or in-process-only tokens are rewritten |
| Rule: no layering | The emit window and the per-batch marker are gone, not kept as a fallback |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Order proven on the real RIB (in-process and forked) | `./le test bgp plugin bmp-locrib` and `./le test bgp plugin bmp-locrib-forked-rib` |
| Order proven at a peer | `./le test integration interop` with scenario `bmp-locrib-pmacct` |
| Scoped unit tests | `./le job run label bmp-unit command go test -race -tags 'ze_bgp ze_bmp' ./internal/component/bgp/plugins/bmp/ ./internal/component/bgp/plugins/rib/ ./internal/core/bgp/ribevents/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `replay-id` from a plugin is an opaque number with no authority: an unknown non-broadcast `best-change` token is dropped with a warning, and an unknown `replay-complete` token is ignored |
| Resource exhaustion | The scope table is bounded by the number of collector sessions, and each entry expires after `locRIBDumpIdleTimeout` (`30 * time.Second`) of silence |

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

- The commissioning brief named "a replay-complete event, a token surviving the JSON codec, a timeout". The code agrees on all three and adds three concrete findings from source inspection:
  1. A surviving token means forked chunks are claimed, so the per-batch marker in `handleBestChange` must go, because the comment that defends it relied on the token not surviving JSON.
  2. `replayBestPaths` held no emit-ordering lock against concurrent `publishBestChanges` callers (unlike `replayRedistribute` with `redistEmitMu`), so mirroring `redistEmitMu` as `bestChangeEmitMu` closes the concurrent-withdraw revival race at the producer.
  3. `bgp-rib` must declare `bgp-rib:replay-complete` in `ribBusSends()` and `events.RegisterNamespace` so forked `bgp-rib` is authorized to emit it, and keeping the whole dump protocol on JSON events ensures compatibility with forked `bgp-bmp` per the 2026-11-24 DQ-3 ruling in `plan/release/fixes/spec-bmp-late-attach-and-forked.md`.
- The comment "BMP is in-process, so the token survives delivery" was both obsolete under DQ-3 and false about timing, which is how the bug stayed hidden.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `false-synchronization-claim.md` | 2026-11-06 | BMP Loc-RIB dump, `emitReplayRequest` sends End-of-RIB before the routes |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An explicit completion event (`bgp-rib/replay-complete`) | Count batches; a quiet period after the last batch | An empty family emits no batch, so a count cannot close it, and a quiet period is a guess that a slow RIB breaks |
| Add `replay-id` and keep `replay` on `BestChangeBatch` JSON | Replace `replay` | External readers decode `replay`; a second member costs one derived field and keeps the wire strictly additive |
| `locRIBDumpIdleTimeout = 30 * time.Second` idle deadline (overridable via `BMPPlugin.dumpIdleTimeout` in tests) | Fixed wall-clock dump length; env var knob | Matches `replayTTL` (`redistribute/replay.go`), `sdkEventEmitTimeout` (`rib_event_sdk.go`), and `reconnectMin` (`sender.go`); measuring silence lets a million-route dump progress indefinitely while bounding a dead RIB |
| Reset the session at the deadline via `senderSession.resetSession` (`clearConnAndResetIf`) | Send the markers anyway; keep waiting | A marker would assert a dump that did not complete. Waiting forever violates AC-7. `clearConnAndResetIf` matches the overflow/storage-failure exit without touching a successor connection |
| Mirror `redistEmitMu` as `bestChangeEmitMu` in `rib_bestchange.go` | Consumer-side `removed` map in BMP | Fixes the replay-vs-incremental-withdraw race once at the producer for all `(bgp-rib, best-change)` consumers without per-consumer tombstone state |

## Known Limitations
- Each collector's dump walks the table once. Sharing one walk between collectors is an optimization nobody asked for.
- Forked `bgp-bmp` (`OnEvent` / RPC event rail and removing the `locrib_runtime.go` bus gate) is tracked separately in `plan/release/fixes/spec-bmp-late-attach-and-forked.md` per the 2026-11-24 DQ-3 ruling; this spec keeps its wire contract fully transport-agnostic so that spec can plug directly into `handleBestChange` and `handleReplayComplete`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. The completion handler carries RFC 7854 Section 5 and RFC 4724 Section 4; the caller in `requestLocRIBDump` carries the section reference.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 7854 Section 5 | With `bgp { bmp { sender { loc-rib true; } } }`, initial Loc-RIB dump sends Peer Up, all dumped Route Monitoring messages, and only then End-of-RIB after `bgp-rib/replay-complete`; failure to complete resets the session without End-of-RIB | None (no option relaxes End-of-RIB ordering; broadcast replays from other subsystems continue to fan out without closing families) | `TestDumpMarkersFollowRoutesWithQueuedRIB`, `TestDumpDeadlineResetsSessionWithoutMarker`, `test/plugin/bmp-locrib.ci`, `test/plugin/bmp-locrib-forked-rib.ci`, `bmp-locrib-pmacct` | `TestForeignReplayClosesNoFamilyDuringDump` |
| RFC 4724 Section 4 | Every address family in `dumpFamilies` (IPv4 unicast, IPv6 unicast) receives its End-of-RIB marker on `replay-complete`, including families with zero best paths | None | `TestDumpEmptyFamilyClosedOnCompletion` | `TestDumpWithNoRIBClosesAtOnce` |
| RFC 9069 Section 5.4 | Initial Loc-RIB synchronization uses Peer Type 3 Route Monitoring preceded by the Loc-RIB Peer Up | None | `test/plugin/bmp-locrib.ci`, `rfc9069_bmp_locrib_test.go` | `TestDumpScopeNarrowsToItsCollector` |

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass, run in the main checkout: `./le test bgp plugin bmp-locrib`, `./le test bgp plugin bmp-locrib-forked-rib`, `./le test integration interop` for `bmp-locrib-pmacct`, and the scoped package tests above. GitHub CI verifies the pushed commit
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
