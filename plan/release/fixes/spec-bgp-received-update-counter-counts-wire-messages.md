# Spec: bgp-received-update-counter-counts-wire-messages

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | A peer's received UPDATE counter, shown by `show` and exported as a metric, reports fewer UPDATEs than the peer sent whenever the read path coalesces them, so an operator comparing it with the peer's sent counter sees a loss that did not happen |
| Scope | protocol |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Found by the RFC 8212 spec (`spec-bgp-rfc8212-ebgp-default-reject.md`) and tracked in `plan/journal/counter-counts-the-wrong-packets.md` (2026-10-29). With `ze.bgp.reactor.coalesce` enabled (the default), `Session.readAndProcessCoalesced` (`internal/component/bgp/reactor/session_coalesce.go`) merges consecutive IPv4 unicast announcements whose path attributes match the held batch, and `flushCoalesce` dispatches the combined NLRIs as one synthetic UPDATE through `processValidatedMessage` and `deliverReceivedMessage`. `Reactor.notifyOneMessage` (`internal/component/bgp/reactor/reactor_notify.go`) then calls `peer.incrUpdatesReceived()` once per dispatched message, so N consecutive wire UPDATEs count as 1 in both `PeerStats.UpdatesReceived` (`show bgp peer` / `show bgp summary` field `updates-received` and derived `rate-updates-received`) and the Prometheus metric `ze_peer_messages_received_total{type="update"}`.

The same misplaced producer also miscounts on the standard path whenever semantic dispatch diverges from wire frames: RFC 7606 treat-as-withdraw extra bodies (`validation.typedWithdrawBodies` and `SynthesizeWithdrawFamilies` `bodies[1:]` in `internal/component/bgp/reactor/session_read.go`) invoke `s.onMessageReceived` multiple times for one wire UPDATE and over-count `updates-received`, whereas a wire UPDATE dropped after header/length validation by `ignore-families`, `receiveCeiling`, `checkPrefixLimits`, or empty treat-as-withdraw synthesis (`len(bodies) == 0`) is never counted even though the peer sent it and `wireBytesRecv`, `teeCapture`, and MRT observed it.

Meanwhile, `Session.observeReceivedFrame` (`internal/component/bgp/reactor/session_read.go`) already runs once per complete wire frame before coalescing or semantic rewriting, recording the capture tee, MRT raw wire observation, and `ze_wire_bytes_received_total`.

**Goal:** Count every received BGP UPDATE message once per complete wire frame inside `Session.observeReceivedFrame` on both the standard (`readFrame`) and coalescing (`readAndProcessCoalesced`) read paths, via a lock-free `Session.onUpdateRecv` stats callback installed by `Peer.attachStatsCallbacks`, while removing `peer.incrUpdatesReceived()` from `Reactor.notifyOneMessage` and leaving EOR counting (`incrEORReceived`), coalesced dispatch, and RIB delivery unchanged. Isolate unchosen incoming collision candidates so stray pre-OPEN UPDATEs on a losing TCP connection never mutate the incumbent peer's counters.

## Required Reading

### Doors
- [ ] `ai/doors/system.md` - platform contracts, registries, and shared machinery
  → Constraint: no external API or wire contract changes; reuse existing stats hook pattern on `Session` and `Peer`.
- [ ] `ai/doors/bgp-engine.md` - BGP engine boundaries, zero-copy invariants, and `-tags ze_bgp` test gate
  → Constraint: received wire buffers remain borrowed and zero-copy; unit tests build with `-tags ze_bgp`.
- [ ] `ai/doors/bgp-plugins.md` - BGP command plugins (`cmd/peer`) consume `PeerStats` via `ReactorIntrospector`
  → Constraint: `show bgp peer` and `show bgp summary` read `PeerStats.UpdatesReceived` unchanged; no plugin edit required.
- [ ] `ai/doors/le-tooling.md` - functional `.ci` tests and native fixtures under `internal/test/fixture/` and `test/plugin/`
  → Constraint: add a deterministic `.ci` test and native fixture proving operator-visible `updates-received` and `ze_peer_messages_received_total{type="update"}`.

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - "Receive UPDATE coalescing" and `## Design decisions`
  → Decision: coalescing combines consecutive legacy IPv4 announcements after per-message NLRI validation and dispatches one synthetic batch; wire-level observation (capture tee, MRT, byte metrics, and now received UPDATE count) runs per complete wire frame before coalescing.
- [ ] `docs/architecture/buffer-architecture.md` - complete-frame reader (`readFrame`), pool ownership, and zero-alloc hot path
  → Constraint: no per-UPDATE heap allocation; sharing `observeReceivedFrame` in `readAndProcessCoalesced` also removes the per-message `s.settings.Address.String()` allocation by reusing `s.addrLabel`.
- [ ] `docs/architecture/behavior/peer-lifecycle.md` - incoming collision candidates (`privateNegotiationEpoch`) and session isolation
  → Constraint: an unchosen incoming candidate must not alter the incumbent session's counters before selection (`selectMessages`).
- [ ] `docs/architecture/api/architecture.md` - engine vs plugin responsibilities and `## Design decisions`
  → Constraint: the engine owns wire I/O and per-peer message counters; NLRI-level counters stay in the RIB plugin.
- [ ] `docs/architecture/bgp/egress-attribute-rules.md` - "ATOMIC_AGGREGATE policy admission" (derived import continuations)
  → Constraint: derived policy continuation bodies are not physical receipts and MUST NOT increment peer receive or EOR counters.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Section 4.1 (Message Header Format), Section 4.3 (UPDATE Message Format), Section 6.1 (Message Header Error Handling)
  → Constraint: a complete BGP UPDATE wire message has a validated 16-octet marker, length 23..4096 (or 23..65535 with RFC 8654 Extended Message), and type 2 (`TypeUPDATE`).
- [ ] `rfc/short/rfc4724.md` - Section 2 (End-of-RIB marker is an empty UPDATE message)
  → Constraint: an End-of-RIB marker is an UPDATE message on the wire, so it increments both `updates-received` (wire layer) and `eor-received` (semantic dispatch layer).

**Key insights:**
- `Session.observeReceivedFrame` (`session_read.go:152`) is the single complete-frame wire observation boundary on the standard read path, while `readAndProcessCoalesced` (`session_coalesce.go:127-133`) duplicated its three calls inline with an allocating `s.settings.Address.String()`. Unifying both paths on `observeReceivedFrame` guarantees identical wire counting.
- `TestAtomicRetirementImportPublication` (`filter_atomic_retirement_pipeline_test.go:196`) invokes `Reactor.notifyMessageReceiver` directly without a `Session` read path and currently asserts `before.UpdatesReceived+1 == after.UpdatesReceived` to prove continuations do not double-count; moving the producer to `Session.observeReceivedFrame` means direct `notifyMessageReceiver` calls increment `UpdatesReceived` by 0 (requiring a `ze-test-judge`-approved test update plus a session-level proof).

## Current Behavior

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_coalesce.go` - `readAndProcessCoalesced` reads complete wire messages, calls `teeCapture`, `observeReceivedWire`, and `wireBytesRecv` inline per wire frame, then coalesces consecutive IPv4 unicast announcements with identical attributes into `s.coalesce.body` and dispatches one synthetic UPDATE via `flushCoalesce` -> `processValidatedMessage`.
- [ ] `internal/component/bgp/reactor/session_read.go` - `readFrame` validates header marker/length, reads body, and calls `observeReceivedFrame(hdr.Type, buf.Buf[:hdr.Length])`; `processValidatedMessage` dispatches primary UPDATEs via `deliverReceivedMessage` and also dispatches synthetic RFC 7606 extra withdrawal bodies (`typedWithdrawBodies` at line 431 and `SynthesizeWithdrawFamilies` `bodies[1:]` at line 527) directly to `s.onMessageReceived`, while dropping non-deliverable UPDATEs (empty treat-as-withdraw, `famDrop`, `pathsLimitRecv`, `prefixDrop`) before `deliverReceivedMessage`.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyOneMessage` (line 504) calls `peer.incrUpdatesReceived()` on every received `TypeUPDATE` with `policyDisposition != ingressPolicyAlreadyApplied`, then checks `wireUpdate.IsEOR()` to call `peer.incrEORReceived()` and EOR hooks.
- [ ] `internal/component/bgp/reactor/peer_stats.go` - `Peer.incrUpdatesReceived()` atomically increments `p.counters.updatesReceived` and `ze_peer_messages_received_total{peer, type="update"}`; `Peer.attachStatsCallbacks` installs per-message callbacks (`onOpenSent`, `onOpenRecv`, `onNotifSent`, `onNotifRecv`, `onRefreshRecv`, `onRead`, `onWrite`) on every `Session`.
- [ ] `internal/component/bgp/reactor/session.go` - declares `Session` stats callback fields (`onOpenSent`, `onOpenRecv`, `onRefreshRecv`, `onRead`, `onWrite`) and `addrLabel`.
- [ ] `internal/component/bgp/reactor/session_open_publication.go` - `privateNegotiationEpoch` isolates unchosen incoming candidates until `selectMessages()` runs on adoption.
- [ ] `internal/component/bgp/reactor/peer_session_factory.go` - constructs every `Session` (both ordinary and incoming candidate) and invokes `p.attachStatsCallbacks(session)`.
- [ ] `internal/component/bgp/reactor/filter_atomic_retirement_pipeline_test.go` - `TestAtomicRetirementImportPublication` calls `notifyMessageReceiver` directly and checks `UpdatesReceived` and `EORReceived`.

**Behavior to preserve:**
- Coalescing itself (`ze.bgp.reactor.coalesce`, default `true`), coalesced batch layout, RFC 7606 per-message NLRI validation before append, and single synthetic dispatch to plugins/RIB.
- EOR counting (`PeerStats.EORReceived`, `ze_peer_messages_received_total{type="eor"}`) and all EOR hooks (`updateDelayEndOfRIB`, `typedNLRIHeld.endOfRIB`, `health.onEORReceived`, `fwdWeights.PeerEORReceived`) staying in `Reactor.notifyOneMessage` gated on `policyDisposition != ingressPolicyAlreadyApplied`.
- `PeerStats.KeepalivesReceived`, `OpensReceived`, `NotificationsReceived`, `RefreshReceived`, and all sent counters unchanged.
- `MessageCallback` and `Reactor.notifyMessageReceiver` / `notifyMessageReceiverFrom` signatures unchanged (preserving all 30+ RFC-tagged test callers).
- Lock-free hot path on received UPDATEs (atomic counter increment + Prometheus counter increment, no `Peer.mu` or `Reactor.mu` lock acquisition per wire UPDATE).
- Unchosen incoming collision candidates never mutate the incumbent peer's `UpdatesReceived` counter.

**Behavior to change:**
- `PeerStats.UpdatesReceived` and `ze_peer_messages_received_total{type="update"}` increment once per complete received BGP UPDATE frame on the wire (in `Session.observeReceivedFrame`, called by both `readFrame` and `readAndProcessCoalesced`) instead of once per `Reactor.notifyOneMessage` semantic dispatch.
- `Session.readAndProcessCoalesced` calls `s.observeReceivedFrame(hdr.Type, buf.Buf[:hdr.Length])` instead of duplicating `teeCapture`, `observeReceivedWire`, and `wireBytesRecv` with an allocating `s.settings.Address.String()`.
- `Reactor.notifyOneMessage` no longer calls `peer.incrUpdatesReceived()`.

## Existing Mechanisms

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Complete-frame wire observation | `internal/component/bgp/reactor/session_read.go` `Session.observeReceivedFrame` | Runs once per complete validated wire frame on `readFrame` (and pre-read `acceptWithOpen`): tees protocol capture, invokes MRT `observeReceivedWire`, and increments `wireBytesRecv` using cached `s.addrLabel` | Extends: invokes `s.onUpdateRecv()` when `kind == msgtype.TypeUPDATE`, and `readAndProcessCoalesced` calls `observeReceivedFrame` instead of duplicating its body |
| Per-session stats callbacks | `internal/component/bgp/reactor/peer_stats.go` `Peer.attachStatsCallbacks` and `internal/component/bgp/reactor/session.go` `Session` callback fields | Installs `onOpenSent`, `onOpenRecv`, `onNotifSent`, `onNotifRecv`, `onRefreshRecv`, `onRead`, `onWrite` before TCP starts, immutable for the session's lifetime | Extends: adds `onUpdateRecv func()` on `Session` and wires it in `Peer.attachStatsCallbacks` to `p.incrUpdatesReceived` |
| Received UPDATE atomic + Prometheus counter | `internal/component/bgp/reactor/peer_stats.go` `Peer.incrUpdatesReceived`, `PeerStats.UpdatesReceived`, `ClearStats` | Atomically increments `p.counters.updatesReceived` and `ze_peer_messages_received_total{peer, type="update"}`; `ClearStats` resets it on session teardown | Extends: keeps the exact producer method, atomic counter, Prometheus series, and reset semantics; changes only its single call site from `notifyOneMessage` to `onUpdateRecv` |
| Incoming candidate publication isolation | `internal/component/bgp/reactor/session_open_publication.go` `privateNegotiationEpoch.selectMessages` | Isolates an unchosen incoming TCP candidate's OPEN observations from the incumbent peer until selection promotes the candidate | Extends: adds a `selected atomic.Bool` set in `selectMessages()` so a candidate's `onUpdateRecv` stays lock-free while ignoring stray pre-OPEN UPDATEs on a losing incoming connection |
| Semantic EOR counting and dispatch | `internal/component/bgp/reactor/reactor_notify.go` `Reactor.notifyOneMessage` | Counts `updatesReceived`, detects EOR (`wireUpdate.IsEOR()`), counts `eorReceived`, triggers EOR hooks, runs ingress filters, and publishes to cache/plugins while skipping counters on `ingressPolicyAlreadyApplied` | Modifies in place: removes `peer.incrUpdatesReceived()` from `case msgtype.TypeUPDATE:` while keeping `incrEORReceived()` and all EOR hooks untouched |
| Peer stats CLI / API rendering | `internal/component/bgp/plugins/cmd/peer/peer.go`, `summary.go` (`fieldUpdatesReceived`, `fieldRateUpdatesReceived`) | Reads `PeerStats.UpdatesReceived` from `ReactorIntrospector` and renders `updates-received`, `messages.received.updates`, and `rate-updates-received` | Does not apply (unchanged consumer): automatically reflects the corrected `PeerStats.UpdatesReceived` across CLI, JSON, REST, gRPC, and MCP |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Before you write code in a subsystem | always | "Grep for the pattern that already exists before you design one" | agrees | Extends existing `Session.observeReceivedFrame`, `Peer.attachStatsCallbacks`, and `Peer.incrUpdatesReceived` rather than inventing a second counter or callback path |
| `docs/architecture/core-design.md` | Receive UPDATE coalescing | `// Design:` and anchors on `session.go`, `session_read.go`, `session_coalesce.go`, `peer_stats.go`, `reactor_notify.go`; modified by this spec | "The session combines consecutive legacy IPv4 announcements only after validating each original NLRI field, including negotiated ADD-PATH framing." | agrees | Preserves coalescing, per-message NLRI validation, and single synthetic dispatch unchanged; moves received UPDATE counting ahead of coalescing into `observeReceivedFrame` and documents that placement in this section |
| `docs/architecture/buffer-architecture.md` | Who owns the buffer at each stage | `// Design:` on `session.go`; anchored to `session.go`, `session_read.go`, `session_coalesce.go`, `session_open_publication.go` | "The complete-frame reader transfers its parsed header and existing pooled buffer handle by value to the selected or incoming reader. It creates no separate heap wrapper for the frame." | agrees | Leaves `receivedFrame` value transfer and pooled buffer ownership untouched, and removes a per-message `s.settings.Address.String()` allocation in `readAndProcessCoalesced` by sharing `observeReceivedFrame` (`s.addrLabel`) |
| `docs/architecture/api/architecture.md` | RIB Ownership | `// Design:` on `peer_stats.go`; anchored to `session.go`, `reactor_notify.go` | "The engine is a minimal BGP speaker." | agrees | Keeps per-peer wire UPDATE counting inside the BGP engine (`Session`/`Peer`) and leaves NLRI-level counters and RIB state in plugins |
| `docs/architecture/behavior/fsm-active.md` | Events handled in Active | anchored to `session.go` | - | silent | Covers BGP FSM Active state transitions and ConnectRetry/DelayOpen handling; silent on per-peer received UPDATE statistics |
| `docs/architecture/behavior/fsm-connect.md` | Events handled in Connect | anchored to `session.go`, `session_read.go` | - | silent | Covers BGP FSM Connect state transitions and TCP dial failure handling; silent on per-peer received UPDATE statistics |
| `docs/architecture/behavior/fsm-established.md` | Events handled in Established | anchored to `session.go`, `session_read.go` | - | silent | Covers BGP FSM Established state transitions, HoldTimer restart, and teardown events; silent on per-peer `updates-received` counter placement |
| `docs/architecture/behavior/fsm-idle.md` | Events handled in Idle | anchored to `session.go` | - | silent | Covers BGP FSM Idle state start/stop events; silent on per-peer received UPDATE statistics |
| `docs/architecture/behavior/fsm-open-confirm.md` | Events handled in OpenConfirm | anchored to `session.go`, `session_read.go` | - | silent | Covers BGP FSM OpenConfirm KEEPALIVE wait and collision resolution; silent on per-peer received UPDATE statistics |
| `docs/architecture/behavior/fsm-open-sent.md` | Events handled in OpenSent | anchored to `session.go`, `session_read.go`, `peer_stats.go` | - | silent | Covers BGP FSM OpenSent OPEN validation and negotiation; silent on per-peer received UPDATE statistics |
| `docs/architecture/behavior/fsm.md` | State Transitions | anchored to `session.go`, `session_read.go`, `peer_stats.go`, `reactor_notify.go` | - | silent | Covers the conceptual BGP FSM states, events, and timer callbacks; silent on per-peer `updates-received` counter placement |
| `docs/architecture/behavior/peer-lifecycle.md` | Tests exercising this layer | `// Design:` on `session_open_publication.go`; anchored to `session.go`, `session_read.go`, `session_coalesce.go`, `peer_stats.go`, `reactor_notify.go` | "Incoming candidates use the ordinary Session frame reader, including header errors, NOTIFICATION events and the original-wire capture tee." | agrees | Keeps incoming candidates on `Session.readFrame` and `observeReceivedFrame`, and gates candidate `onUpdateRecv` on `privateNegotiationEpoch.isSelected()` so unchosen candidates never mutate incumbent peer counters |
| `docs/architecture/congestion-industry.md` | Universal Rule | anchored to `session.go` | - | silent | Covers industry BGP congestion control comparisons and read-throttle bounds; silent on per-peer received UPDATE counting |
| `docs/architecture/edge-cases/extended-message.md` | Ze implementation | anchored to `session.go`, `session_read.go`, `session_coalesce.go` | "Both the ordinary and coalesced readers validate the header before attempting to read its declared body." | agrees | Preserves `ValidateLengthWithMax` before body read and calls `observeReceivedFrame` only after the complete validated frame is read on both paths |
| `docs/architecture/encoding-context.md` | Overview | anchored to `session.go`, `session_read.go` | - | silent | Covers per-direction `EncodingContext` and `ContextID` zero-copy equality; silent on per-peer received UPDATE counting |
| `docs/architecture/forward-congestion-pool.md` | Two-Tier Pool Model | anchored to `session.go`, `session_read.go` | - | silent | Covers outbound forwarding pool sizing, overflow tokens, and backpressure; silent on inbound UPDATE counter placement |
| `docs/architecture/memory/lifetime-contracts.md` | A. WireUpdate — eager copy | anchored to `session.go`, `session_read.go`, `session_coalesce.go`, `reactor_notify.go` | "The session's owned receipt gates semantic processing, not socket reading." | agrees | Places `onUpdateRecv` in `observeReceivedFrame` before `beginAdmission` without retaining any slice of the pooled read buffer |
| `docs/architecture/message-buffer-design.md` | Overview | anchored to `session.go` | - | silent | Covers message buffer layout and `WriteTo` serialization; silent on per-peer received UPDATE counting |
| `docs/architecture/rib-transition.md` | Current: Engine Has No RIB | anchored to `session.go` | - | silent | Covers the transition of RIB storage from engine to plugins; silent on per-peer received UPDATE counting |
| `docs/architecture/update-building.md` | Path 0: Receive Path (Zero-Copy Ingest) | anchored to `session.go`, `reactor_notify.go` | - | silent | Covers outbound UPDATE construction and zero-copy ingest buffers; silent on inbound received UPDATE counting |
| `ai/doors/bgp-engine.md` | Boundaries | door covering `internal/component/bgp/reactor/*` | "A received UPDATE points into a reusable read buffer, so `RawMessage.IsAsyncSafe()` is false and anything that hands it to another goroutine copies first" | agrees | `onUpdateRecv` takes no byte slice and performs only an atomic increment and Prometheus counter increment synchronously on the read goroutine |
| `docs/architecture/edge-cases/as4.md` | Overview | anchored to `session_read.go` | - | silent | Covers RFC 6793 AS_PATH/AS4_PATH collapse and two-octet interoperability; silent on per-peer received UPDATE counting |
| `docs/architecture/wire/attributes.md` | Attribute Header Format | anchored to `session_read.go` | - | silent | Covers BGP path attribute wire formats and flags; silent on per-peer received UPDATE counting |
| `docs/architecture/wire/messages.md` | Header Parsing | anchored to `session_read.go` | "Both session read paths (`readAndProcessMessage` and the coalescing reader) answer its error with `Session.notifyHeaderErr`" | agrees | Keeps header marker and length error handling ahead of `observeReceivedFrame` on both read paths so header-invalid frames are never counted as UPDATEs |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `session_open_publication.go`, `reactor_notify.go` | "Receive observation precedes message-body validation and coalescing; send observation follows accepted transport bytes, not buffer staging. Synthetic withdrawals are not wire packets." | agrees | Places wire UPDATE counting at the exact same pre-validation, pre-coalescing receive observation boundary (`observeReceivedFrame`), so coalesced batches count N wire messages and synthetic withdrawals count 0 |
| `docs/architecture/bgp/egress-attribute-rules.md` | ATOMIC_AGGREGATE policy admission | `// Design:` on `filter_atomic_retirement_pipeline_test.go`; anchored to `reactor_notify.go` | "Derived bodies are not physical receipts and do not increment the peer's receive or EOR counters." | agrees | Moving `incrUpdatesReceived` out of `notifyOneMessage` guarantees by construction that derived continuation bodies never increment `updatesReceived`, while `notifyOneMessage` retains the `ingressPolicyAlreadyApplied` guard on `incrEORReceived` |
| `docs/architecture/bgp/structural-forwarding.md` | Receive validation and cache changes | anchored to `reactor_notify.go` | - | silent | Covers destination forwarding rails and structural work moved off the critical path; silent on inbound received UPDATE counting |
| `docs/architecture/exabgp-bridge.md` | The bridge translates in two directions | anchored to `reactor_notify.go` | - | silent | Covers ExaBGP wrapper process protocol and base64 wire event format; silent on per-peer `updates-received` counter placement |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `reactor_notify.go` | - | silent | Covers RFC 9234 BGP Role and OTC route metadata; silent on per-peer received UPDATE counting |
| `docs/architecture/mrt.md` | Decoding (read side) | anchored to `reactor_notify.go` | - | silent | Covers MRT wire encoding/decoding and BGP4MP capture records; `observeReceivedWire` continues to run at the same point inside `observeReceivedFrame` with no change to MRT behavior |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `reactor_notify.go` | - | silent | Covers RIB plugin storage, deduplication pools, and best-path selection; silent on reactor-level per-peer received UPDATE counting |
| `ai/doors/le-tooling.md` | Boundaries | door covering `internal/test/fixture/plugin_fixture_09_bgp.go` and `test/plugin/bgp-coalesced-updates-received-count.ci` | "Run one package through `./le test unit pkg path <dir>/ [run <regex>] [vet]`, which takes the tags from the toolchain." | agrees | Names `./le test unit pkg path ./internal/component/bgp/reactor/` and `./le test functional plugin bgp-coalesced-updates-received-count` for main-thread verification, and returns an error from the Go fixture on assertion failure |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Touches no VPP, kernel data-plane, `fib`, or `sysctl` configuration or commit validation; decision remains fully preserved |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Touches no config validation or startup/reload mismatch verdicts; decision remains fully preserved |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Touches no firewall verifier or YANG backend annotations; decision remains fully preserved |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Touches no data-plane backend selectors or YANG schemas; decision remains fully preserved |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Touches only inbound UPDATE counter placement; egress next-hop rewriting and export chains are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Touches no `policychain` or export next-hop authoring forms; decision remains fully preserved |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Touches no compiled export policy chain steps or config compilation; decision remains fully preserved |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Touches no export next-hop folding or raw override provenance; decision remains fully preserved |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Touches no policy next-hop validation or config loading; decision remains fully preserved |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Touches no forwarding or origination next-hop rewriting; decision remains fully preserved |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Touches no filter-chain insertions or `\| implied` output; decision remains fully preserved |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Adds no startup refusal or service configuration gate; `ze start` behavior is unchanged |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Touches no VPP/kernel startup or commit mismatch handling; decision remains fully preserved |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Touches no `\| implied` config rendering; decision remains fully preserved |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Touches no bus participant registrations or routing; decision remains fully preserved |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Touches no bus `send > to` configuration or delivery filtering; decision remains fully preserved |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Touches no bus `receive > from` configuration or delivery filtering; decision remains fully preserved |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Touches no bus allow-lists; decision remains fully preserved |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Touches no bus filter compilation or commit conflict checks; decision remains fully preserved |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Touches no bus participant arbitration; decision remains fully preserved |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Touches no `\| implied` display; decision remains fully preserved |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Touches no outbound FlowSpec MP_REACH next-hop encoding or `ze exabgp migrate`; decision remains fully preserved |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no engine RPC operation or plugin authorization profile; decision remains fully preserved |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Touches no engine-to-plugin event queue or delivery behavior; decision remains fully preserved |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | Touches no ExaBGP wrapper or bus endpoint mode; decision remains fully preserved |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-26 | "ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`, zero key names, `0:` when absent) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases" | agrees | Touches no ExaBGP wrapper event encoding, `MuxConn` batching, or `FormatBase64` delivery; decision remains fully preserved |

## Data Flow

### Entry Point
- Complete BGP wire frames read from a peer's TCP connection via `bufio.Reader` in either `Session.readFrame` (`session_read.go`, standard path when `ze.bgp.reactor.coalesce=false` and incoming candidate handshake) or `Session.readAndProcessCoalesced` (`session_coalesce.go`, default path when `ze.bgp.reactor.coalesce=true`).
- Format at entry: validated BGP-4 message frame of length 19..4096 (or 19..65535 with RFC 8654 Extended Message), with verified 16-octet `0xFF` marker, validated per-type minimum length (`>= 23` octets for `TypeUPDATE`), and complete body read from `bufReader`.

### Transformation Path
1. TCP read in `Session.readFrame` or `Session.readAndProcessCoalesced` reads the 19-octet header, validates marker and length (`ParseHeader`, `ValidateLengthWithMax`), and reads the complete body into the pooled read buffer.
2. Both paths call `Session.observeReceivedFrame(hdr.Type, buf.Buf[:hdr.Length])` once per complete wire frame, before admission gating or coalescing.
3. `Session.observeReceivedFrame` runs `s.teeCapture`, `s.observeReceivedWire`, `wireBytesRecv.Add`, and — when `kind == msgtype.TypeUPDATE && s.onUpdateRecv != nil` — invokes `s.onUpdateRecv()`.
4. `s.onUpdateRecv()` calls `Peer.incrUpdatesReceived()` directly for selected sessions (or after checking `private.selected.Load()` for a `privateNegotiationEpoch` candidate), atomically incrementing `p.counters.updatesReceived` and Prometheus `ze_peer_messages_received_total{peer, type="update"}`.
5. Coalescing (`readAndProcessCoalesced`), RFC 7606 validation (`processValidatedMessage`), synthetic withdrawal splitting (`typedWithdrawBodies` / `bodies[1:]`), and semantic delivery (`deliverReceivedMessage` -> `Reactor.notifyOneMessage`) continue unchanged; `notifyOneMessage` no longer increments `updatesReceived`, while still decoding `wireUpdate.IsEOR()` to increment `eorReceived` and fire EOR hooks once per genuine EOR (never on `ingressPolicyAlreadyApplied`).
6. Operational readers (`show bgp peer`, `show bgp summary`, `show metrics`) read `Peer.Stats().UpdatesReceived` and the Prometheus counter, reporting exact wire UPDATE counts matching the peer's sent UPDATE counter.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| TCP socket (`bufReader`) ↔ `Session` wire observer | `Session.observeReceivedFrame(hdr.Type, buf.Buf[:hdr.Length])` called synchronously on the session read goroutine | No |
| `Session` ↔ `Peer` statistics | Immutable `Session.onUpdateRecv` callback installed by `Peer.attachStatsCallbacks(session)` before TCP starts; lock-free atomic increment | No |
| `Session` ↔ `Reactor` semantic delivery | Existing `Session.onMessageReceived` -> `Reactor.notifyMessageReceiverFrom` -> `notifyOneMessage` (signature unchanged; no longer increments `updatesReceived`, still increments `eorReceived`) | No |
| `Reactor` ↔ CLI / API / Telemetry | Existing `Peer.Stats().UpdatesReceived` via `ReactorIntrospector` and Prometheus `ze_peer_messages_received_total{peer, type="update"}` | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/component/bgp/reactor/`, `internal/component/bgp/plugins/cmd/peer/`, `internal/test/`, and `docs/` for `incrUpdatesReceived`, `UpdatesReceived`, `updates-received`, and `ze_peer_messages_received_total` (positive controls: `Peer.incrUpdatesReceived` in `internal/component/bgp/reactor/peer_stats.go`, `Reactor.notifyOneMessage` in `internal/component/bgp/reactor/reactor_notify.go`, `fieldUpdatesReceived` in `internal/component/bgp/plugins/cmd/peer/fields.go`, and `peerMsgRecv` in `internal/component/bgp/reactor/reactor_metrics.go`).

No wire format, RPC payload, JSON event schema, CLI/API response schema, or Prometheus metric name/label set changes: only the in-process call site that increments the existing `Peer.counters.updatesReceived` atomic and `ze_peer_messages_received_total{peer, type="update"}` series moves from semantic dispatch (`notifyOneMessage`) to complete-frame wire observation (`observeReceivedFrame`).

### Integration Points
- `Peer.attachStatsCallbacks` (`internal/component/bgp/reactor/peer_stats.go`) - installs `session.onUpdateRecv` beside `onOpenRecv`, `onRefreshRecv`, and `onNotifRecv` for every `Session` built by `Peer.newSessionWithEpoch`.
- `Session.observeReceivedFrame` (`internal/component/bgp/reactor/session_read.go`) - shared wire-observation entry point now called by both `Session.readFrame` and `Session.readAndProcessCoalesced`.
- `privateNegotiationEpoch.selectMessages` (`internal/component/bgp/reactor/session_open_publication.go`) - flips `selected` to `true` once when an incoming candidate wins arbitration so its `onUpdateRecv` activates with zero lock overhead.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Both `readFrame` and `readAndProcessCoalesced` funnel complete validated wire frames through `Session.observeReceivedFrame`, which triggers `onUpdateRecv` once per wire UPDATE; semantic EOR detection stays in `Reactor.notifyOneMessage` |
| No unintended coupling (components stay isolated) | Yes | `Session` calls an opaque `func()` callback (`onUpdateRecv`) installed by `Peer.attachStatsCallbacks`, matching `onOpenRecv`/`onRefreshRecv`; no new package imports |
| No duplicated functionality (extends existing, does not recreate) | Yes | Eliminates the duplicated wire-observation block in `readAndProcessCoalesced` by calling `Session.observeReceivedFrame` directly, and reuses `Peer.incrUpdatesReceived` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No buffer copies added; switching `readAndProcessCoalesced` to `observeReceivedFrame` eliminates a per-message `s.settings.Address.String()` allocation by reusing `s.addrLabel` |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, view, family, plugin, or feature switch added; reuses existing `PeerStats` and `ze_peer_messages_received_total` registrations |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Introduces no new operator-visible name, enum value, metric name, or command word; searched `msgTypeNames` in `peer_stats.go` and `fieldUpdatesReceived` in `plugins/cmd/peer/fields.go`, both unchanged |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every received BGP UPDATE from TCP enters through either `Session.readFrame` or `Session.readAndProcessCoalesced` | `session.go:1566-1570`, `session_read.go:65`, `incoming_candidate.go:359` (`git grep readFrame\|readAndProcessCoalesced`) | An alternate TCP read path would miss UPDATE counting | Code inspection of `Session.Run`, `ReadAndProcess`, `incomingCandidate.readFrame`, and unit tests covering both paths | unvalidated |
| A-2 | `TestAtomicRetirementImportPublication` (`filter_atomic_retirement_pipeline_test.go:196`) is the only unit test calling `Reactor.notifyMessageReceiver` directly and asserting `UpdatesReceived` increments | `git grep -n "UpdatesReceived" internal/component/bgp/reactor/` checked all 12 test files referencing `UpdatesReceived` | Another direct-call unit test would fail after removing `incrUpdatesReceived` from `notifyOneMessage` | Scoped unit test run `./le test unit pkg path ./internal/component/bgp/reactor/` | unvalidated |
| A-3 | Existing functional `.ci` fixtures that poll `updates-received >= 1` (`plugin_fixture_12_prefix.go`, `plugin_fixture_13.go`, `rfc8212_policy.go`, `policy_final.go`) remain green when `updates-received` increments in `observeReceivedFrame` right before `beginAdmission` | `plugin_fixture_12_prefix.go`, `rfc8212_policy.go`, and `policy_final.go` all follow the poll with `request quiesce`; `plugin_fixture_13.go` (`rfc7606-withdraw.ci`) runs in-memory `applyRFC7606` synchronously in nanoseconds before the plugin RPC round-trip | A fixture polling `updates-received` without quiesce could observe the counter before semantic dispatch finishes | Running the affected functional `.ci` tests (`rfc7606-withdraw`, `metrics-session-lifecycle`, `bgp-coalesced-updates-received-count`) | unvalidated |
| A-4 | An incoming candidate (`privateNegotiationEpoch`) never legitimately receives an UPDATE before `selectMessages()` runs | `incoming_candidate.go:336-350` (`readOpen` rejects non-OPEN before OPEN as unexpected) and `peer_run.go` calls `selectMessages()` on promotion before entering the established read loop | A legitimate UPDATE on an adopted candidate would be under-counted | Unit test covering adopted incoming candidate receiving UPDATEs after promotion and unchosen candidate receiving pre-OPEN UPDATE | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Updating `TestAtomicRetirementImportPublication` (`filter_atomic_retirement_pipeline_test.go:196`) could look like test weakening | `ze-test-judge` or commit hook flags assertion change in existing test | Explain that `notifyMessageReceiver` is no longer the producer of `UpdatesReceived`, retain the non-increment proof (`after.UpdatesReceived == before.UpdatesReceived`, `after.EORReceived == before.EORReceived`), add a session-level test proving 1 wire UPDATE with synthetic continuations counts as 1, and obtain independent `ze-test-judge` approval |
| R-2 | `acceptWithOpen` calls `observeReceivedFrame(msgtype.TypeOPEN, wire)` on pre-read OPENs | Accidental counting of non-UPDATE messages in `observeReceivedFrame` | Gate `s.onUpdateRecv()` strictly on `kind == msgtype.TypeUPDATE`; unit test verifies OPEN, KEEPALIVE, NOTIFICATION, and ROUTE-REFRESH leave `UpdatesReceived` at 0 |
| R-3 | Coalescing functional test might not coalesce if TCP splits the two UPDATEs into separate kernel reads | Functional test passes even with fix reverted (vacuous test) | Write both identical-attribute IPv4 unicast UPDATEs plus EOR in a single `action=send` hex frame (single TCP `Write` call so `bufReader.Buffered() > 0` between messages), and verify vacuity by reverting the fix and confirming the test goes red (`updates-received` reports 2 instead of 3) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Only per-peer received UPDATE statistics (`PeerStats.UpdatesReceived`, `show bgp peer`/`summary`, `ze_peer_messages_received_total{type="update"}`); no BGP wire bytes, FSM transitions, RIB state, or forwarding behavior changes |
| How is it reverted? | Single commit revert of the reactor counter placement and test updates; no config, schema, or state-store migration |
| Who else touches this path? | `spec-bgp-rfc8212-ebgp-default-reject` (which discovered the defect); `session_read.go`, `session_coalesce.go`, `peer_stats.go`, `reactor_notify.go`, and `session_open_publication.go` in `internal/component/bgp/reactor/` |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer TCP burst of two identical-attribute IPv4 unicast UPDATEs (`ze.bgp.reactor.coalesce=true`) | → | `Session.readAndProcessCoalesced` → `Session.observeReceivedFrame` → `Session.onUpdateRecv` → `Peer.incrUpdatesReceived` | `TestCoalescedUpdatesReceivedCountsWireMessages` (`internal/component/bgp/reactor/session_coalesce_test.go`) and `test/plugin/bgp-coalesced-updates-received-count.ci` |
| Peer TCP UPDATE on standard non-coalescing read path (`ze.bgp.reactor.coalesce=false`) | → | `Session.readFrame` → `Session.observeReceivedFrame` → `Session.onUpdateRecv` → `Peer.incrUpdatesReceived` | `TestStandardPathUpdatesReceivedCountsWireMessages` (`internal/component/bgp/reactor/session_coalesce_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Coalescing enabled (default `ze.bgp.reactor.coalesce=true`); peer sends 2 (and 3) consecutive IPv4 unicast UPDATEs with identical path attributes in one TCP burst | Coalescer merges them into 1 dispatched synthetic UPDATE containing all NLRIs, while `Peer.Stats().UpdatesReceived` and `ze_peer_messages_received_total{peer, type="update"}` each increase by the exact wire message count (2 or 3) | Single synthetic dispatch to `onMessageReceived`/RIB remains permitted and required: coalescing itself is preserved by design |
| AC-2 | Coalescing disabled (`ze.bgp.reactor.coalesce=false`); peer sends N wire UPDATEs (announcements, withdrawals, and an EOR) | `Peer.Stats().UpdatesReceived` and `ze_peer_messages_received_total{peer, type="update"}` each increase by N, identical to the coalescing path, and `EORReceived` / `type="eor"` increases by 1 for the EOR | Non-UPDATE frames (OPEN, KEEPALIVE, NOTIFICATION, ROUTE-REFRESH) do not increment `UpdatesReceived` because they have their own type counters |
| AC-3 | Peer sends 1 wire UPDATE that produces multiple semantic deliveries via RFC 7606 treat-as-withdraw multi-family synthesis (`bodies[1:]` or `typedWithdrawBodies`) or import policy atomic-retirement continuations | All synthesized/continuation bodies still reach downstream consumers in order, while `Peer.Stats().UpdatesReceived` and `ze_peer_messages_received_total{peer, type="update"}` increase by exactly 1 (never 2+) and `EORReceived` remains 0 | Multiple internal `onMessageReceived` dispatches for the synthesized withdrawals/continuations remain permitted because they are required by RFC 7606 §3(g) and RFC 4271 §5.1.6 |
| AC-4 | Peer sends 1 well-formed IPv4 unicast EOR (empty UPDATE) after 2 coalesced announcements | `Peer.Stats().UpdatesReceived` is 3, `ze_peer_messages_received_total{peer, type="update"}` is 3, `Peer.Stats().EORReceived` is 1, and `ze_peer_messages_received_total{peer, type="eor"}` is 1 | Counting an EOR in both `updates-received` and `eor-received` remains permitted because RFC 4724 §2 defines EOR as an UPDATE message on the wire and `eor-received` tracks the EOR subset |
| AC-5 | Unchosen incoming collision candidate (`privateNegotiationEpoch`) receives a stray pre-OPEN UPDATE while an incumbent session is Established; and a winning incoming candidate is adopted and receives UPDATEs | Stray pre-OPEN UPDATE on the unchosen candidate leaves the incumbent peer's `UpdatesReceived` unchanged; after `selectMessages()` adopts a winning candidate, each wire UPDATE it receives increments `UpdatesReceived` by 1 | Wire capture and MRT observation on the candidate remain permitted as documented in `peer-lifecycle.md`; only incumbent peer counter mutation is prevented |
| AC-6 | End-to-end daemon with default coalescing enabled receives a single-burst pair of identical-attribute IPv4 unicast UPDATEs plus an EOR from `le test peer` | `show bgp peer peer1 detail \| json` reports `updates-received` = 3 and `eor-received` = 1, `show metrics` reports `ze_peer_messages_received_total{peer="127.0.0.1",type="update"} 3` and `type="eor"} 1`, and both announced prefixes exist in `show bgp rib` | None; all three operator surfaces (CLI JSON, Prometheus metrics, and RIB) must agree on the exact wire count and both prefixes |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `show bgp peer <peer> detail` and scrapes Prometheus `ze_peer_messages_received_total` after a peer sent N consecutive identical-attribute IPv4 unicast UPDATEs plus an EOR | `le test peer` TCP burst -> `Session.readAndProcessCoalesced` -> `Session.observeReceivedFrame` -> `Session.onUpdateRecv` -> `Peer.incrUpdatesReceived` -> `Peer.Stats()` / Prometheus -> `show bgp peer detail` / `show metrics` | `test/plugin/bgp-coalesced-updates-received-count.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCoalescedUpdatesReceivedCountsWireMessages` | `internal/component/bgp/reactor/session_coalesce_test.go` | AC-1, AC-4: Peer + Session with coalescing on receives 2 and 3 identical-attribute IPv4 UPDATEs + EOR in one TCP burst; asserts 1 coalesced announcement dispatch, `Peer.Stats().UpdatesReceived` == wire count (3 / 4), `EORReceived` == 1, and spy Prometheus `ze_peer_messages_received_total{type="update"}` matches | |
| `TestStandardPathUpdatesReceivedCountsWireMessages` | `internal/component/bgp/reactor/session_coalesce_test.go` | AC-2: Same input burst with `coalesceEnabled=false` (`readAndProcessMessage` / `readFrame`) produces identical `UpdatesReceived`, `EORReceived`, and `ze_peer_messages_received_total{type="update"}` counts, while KEEPALIVE does not increment `UpdatesReceived` | |
| `TestUpdatesReceivedCountsOnePerWireMessageOnSyntheticSplits` | `internal/component/bgp/reactor/session_coalesce_test.go` | AC-3: 1 malformed multi-MP-family UPDATE triggering RFC 7606 treat-as-withdraw synthesis (`bodies[1:]`) and 1 wire UPDATE triggering import atomic-retirement continuations each increment `UpdatesReceived` by exactly 1 (not 2+) | |
| `TestIncomingCandidatePreOpenUpdateDoesNotMutateIncumbentStats` | `internal/component/bgp/reactor/session_coalesce_test.go` | AC-5: Unchosen `privateNegotiationEpoch` candidate receiving a stray pre-OPEN UPDATE leaves incumbent `Peer.Stats().UpdatesReceived` unchanged; after `selectMessages()` promotion, received wire UPDATEs increment `UpdatesReceived` by 1 each | |
| `TestAtomicRetirementImportPublication` (updated, `ze-test-judge` approved) | `internal/component/bgp/reactor/filter_atomic_retirement_pipeline_test.go` | AC-3: Direct `notifyMessageReceiver` call with synthetic continuations leaves both `UpdatesReceived` and `EORReceived` unchanged (`before == after`), confirming semantic delivery never manufactures wire UPDATE counts | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A (no new numeric config or CLI input; BGP UPDATE wire length bounds 23..4096 / 23..65535 are already enforced by `ValidateLengthWithMax` before `observeReceivedFrame`) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-coalesced-updates-received-count` | `test/plugin/bgp-coalesced-updates-received-count.ci` (observer driver `plugin/bgp-coalesced-updates-received-count` in `internal/test/fixture/plugin_fixture_09_bgp.go`) | AC-1, AC-4, AC-6: Default coalescing on; `le test peer` sends two identical-attribute IPv4 unicast UPDATEs and an EOR in one concatenated TCP write; observer asserts `show bgp peer peer1 detail` reports `updates-received == 3` and `eor-received == 1`, `show metrics` reports `ze_peer_messages_received_total{peer="127.0.0.1",type="update"} 3` and `type="eor"} 1`, and `show bgp rib` holds both prefixes | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A (no wire-visible protocol behavior changes; omitted per `ai/rules/interop-and-goal-validation.md`: "pure internal refactor with no wire-visible change") | - | - | Counter placement is strictly local operational telemetry on Ze's receive path; zero outbound or negotiated wire bytes change, and `test/plugin/bgp-coalesced-updates-received-count.ci` deterministically exercises the coalesced TCP burst that an external daemon's TCP segmentation cannot guarantee | |

## Files to Modify
- `internal/component/bgp/reactor/session.go` - add `onUpdateRecv func()` callback field on `Session` beside `onOpenRecv` / `onRefreshRecv`
- `internal/component/bgp/reactor/session_read.go` - invoke `s.onUpdateRecv()` inside `Session.observeReceivedFrame` when `kind == msgtype.TypeUPDATE`
- `internal/component/bgp/reactor/session_coalesce.go` - replace the duplicated inline `teeCapture` / `observeReceivedWire` / `wireBytesRecv` block in `Session.readAndProcessCoalesced` with `s.observeReceivedFrame(hdr.Type, buf.Buf[:hdr.Length])`
- `internal/component/bgp/reactor/session_open_publication.go` - add `selected atomic.Bool` on `privateNegotiationEpoch`, set in `selectMessages()`, with a lock-free `isSelected() bool` accessor
- `internal/component/bgp/reactor/peer_stats.go` - install `session.onUpdateRecv` in `Peer.attachStatsCallbacks` (direct `p.incrUpdatesReceived` for selected sessions; `if private.isSelected() { p.incrUpdatesReceived() }` for `privateNegotiationEpoch` candidates)
- `internal/component/bgp/reactor/reactor_notify.go` - remove `peer.incrUpdatesReceived()` from `Reactor.notifyOneMessage` (`case msgtype.TypeUPDATE:`), keeping `incrEORReceived()` and all EOR hooks unchanged
- `internal/component/bgp/reactor/filter_atomic_retirement_pipeline_test.go` - update `TestAtomicRetirementImportPublication` expectation from `before.UpdatesReceived+1` to `before.UpdatesReceived` (with independent `ze-test-judge` approval) since `notifyMessageReceiver` is no longer the wire UPDATE counter producer
- `internal/component/bgp/reactor/session_coalesce_test.go` - add unit tests for AC-1 through AC-5
- `internal/test/fixture/plugin_fixture_09_bgp.go` - register and implement `plugin/bgp-coalesced-updates-received-count` observer driver
- `docs/architecture/core-design.md` - document in `### Receive UPDATE coalescing` that per-peer received UPDATE counting runs per complete wire frame in `Session.observeReceivedFrame` before coalescing while EOR counting stays on semantic dispatch

## Files to Create
- `test/plugin/bgp-coalesced-updates-received-count.ci` - functional test sending a single-burst coalesced pair of IPv4 unicast UPDATEs plus EOR and verifying `show bgp peer`, Prometheus metrics, and RIB state

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No new config leaf or RPC; existing `ze.bgp.reactor.coalesce` and `show bgp peer` schema unchanged |
| YANG validation constraints | N-A | No YANG schema change |
| YANG custom validators | N-A | No YANG schema change |
| CLI commands/flags | N-A | Existing `show bgp peer` and `show bgp summary` already read `PeerStats.UpdatesReceived` |
| CLI grammar (keyword before value) | N-A | No CLI grammar change |
| Editor autocomplete | N-A | No config or CLI addition |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-coalesced-updates-received-count.ci` proves `show bgp peer detail` and `show metrics` end-to-end |
| Pipe completeness | N-A | Existing `show bgp peer` pipes unchanged |
| Env var registration | N-A | Existing `ze.bgp.reactor.coalesce` registration unchanged |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Doctor code per problem class | N-A | No new refusal, warning, or error class introduced |
| Prometheus counters/metrics | Yes | Existing `ze_peer_messages_received_total{peer, type="update"}` (`reactor_metrics.go`, `peer_stats.go`) now increments per wire UPDATE frame instead of per coalesced dispatch |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new SAFI, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Defect fix to existing received UPDATE counter; no feature maturity change |
| 2 | Config syntax changed? | No | No config syntax change |
| 3 | CLI command added/changed? | No | Existing `show bgp peer` / `show bgp summary` output fields unchanged |
| 4 | API/RPC added/changed? | No | No API/RPC schema change |
| 5 | Plugin added/changed? | No | No plugin code changes |
| 6 | Has a user guide page? | No | `docs/guide/monitoring.md:517` already describes `ze_peer_messages_received_total` as "Messages received (type: update, keepalive, open, notification, refresh, eor)", which the fix now fulfills accurately |
| 7 | Wire format changed? | No | No wire format change |
| 8 | Plugin SDK/protocol changed? | No | No `pkg/plugin` or process protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No new RFC requirement row or wire behavior; RFC 4271/4724 message counting semantics unchanged |
| 10 | Test infrastructure changed? | No | Uses existing `le test peer` and `internal/test/fixture/` runner |
| 11 | Affects daemon comparison? | No | No comparison table claim affected |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (`### Receive UPDATE coalescing`) updated to record wire-frame counting in `observeReceivedFrame` before coalescing |
| 13 | Route metadata keys added/changed? | No | No route metadata change |
| 14 | Prometheus counters added/changed? | No | Metric name and labels unchanged; `docs/guide/monitoring.md` already matches wire-message semantics |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration inventory change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Verified via `./le spec citation anchors`: `docs/architecture/core-design.md` is updated (`### Receive UPDATE coalescing`). Declared `// Design:` pages `docs/architecture/buffer-architecture.md`, `docs/architecture/api/architecture.md`, `docs/architecture/behavior/peer-lifecycle.md`, and `docs/architecture/bgp/egress-attribute-rules.md` are verified unaffected (existing contracts already hold). Advisory `<!-- source: -->` mentions in `ai/patterns/functional-test.md`, `docs/DESIGN.md`, `docs/architecture.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/behavior/fsm-active.md`, `docs/architecture/behavior/fsm-connect.md`, `docs/architecture/behavior/fsm-established.md`, `docs/architecture/behavior/fsm-idle.md`, `docs/architecture/behavior/fsm-open-confirm.md`, `docs/architecture/behavior/fsm-open-sent.md`, `docs/architecture/behavior/fsm.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/congestion-industry.md`, `docs/architecture/edge-cases/as4.md`, `docs/architecture/edge-cases/extended-message.md`, `docs/architecture/encoding-context.md`, `docs/architecture/exabgp-bridge.md`, `docs/architecture/forward-congestion-pool.md`, `docs/architecture/memory/lifetime-contracts.md`, `docs/architecture/message-buffer-design.md`, `docs/architecture/meta/role.md`, `docs/architecture/mrt.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/rib-transition.md`, `docs/architecture/update-building.md`, `docs/architecture/wire/attributes.md`, `docs/architecture/wire/messages.md`, `docs/bgp-fsm.md`, `docs/comparison.md`, `docs/features/configuration.md`, `docs/functional-tests.md`, `docs/guide/add-path.md`, `docs/guide/command-reference.md`, `docs/guide/configuration.md`, `docs/guide/debugging-tools.md`, `docs/guide/operational-reports.md`, and `docs/guide/route-reflection.md` are unaffected because no FSM, wire, buffer, config, CLI, or forwarding contract changes |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No config/CLI/API syntax changed |

## Design presentation

### What changes

Move the single production increment of a peer's received UPDATE counter (`PeerStats.UpdatesReceived` and Prometheus `ze_peer_messages_received_total{type="update"}`) from semantic message dispatch (`Reactor.notifyOneMessage`) to complete-frame wire observation (`Session.observeReceivedFrame`, shared by both `Session.readFrame` and `Session.readAndProcessCoalesced`), wired through a lock-free `Session.onUpdateRecv` callback installed by `Peer.attachStatsCallbacks` and isolated from unchosen incoming collision candidates via `privateNegotiationEpoch`. Coalescing, EOR counting (`incrEORReceived`), and downstream RIB/plugin dispatch remain unchanged.

### Before and after

1. **Coalesced IPv4 unicast burst (default `ze.bgp.reactor.coalesce=true`):**
   - **Before:** Peer sends 2 consecutive IPv4 unicast UPDATEs with identical path attributes plus 1 EOR in one burst. Ze coalesces the 2 announcements into 1 synthetic UPDATE and dispatches 2 messages to `notifyOneMessage`. `show bgp peer peer1 detail` reports `updates-received: 2` and `eor-received: 1`, and `ze_peer_messages_received_total{peer="127.0.0.1",type="update"}` reads `2` where the peer sent `3`.
   - **After:** `Session.observeReceivedFrame` runs once per complete wire frame before coalescing. `show bgp peer peer1 detail` reports `updates-received: 3` and `eor-received: 1`, `ze_peer_messages_received_total{peer="127.0.0.1",type="update"}` reads `3`, and both prefixes are still delivered in 1 coalesced batch to the RIB.

2. **RFC 7606 multi-family treat-as-withdraw (1 wire UPDATE synthesized into 2 withdrawal bodies):**
   - **Before:** `processValidatedMessage` invokes `onMessageReceived` twice (primary + extra family body), so `show bgp peer` reports `updates-received` incremented by `2` for 1 wire UPDATE.
   - **After:** `Session.observeReceivedFrame` runs once for the single wire frame, so `updates-received` increments by `1`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep `peer.incrUpdatesReceived()` in `Reactor.notifyOneMessage` and thread a `wireCount` integer through `coalesceState`, `processValidatedMessage`, `MessageCallback`, and `notifyMessageReceiverFrom` | Widens an 11-parameter `MessageCallback` signature referenced in 30+ files and `notifyMessageReceiverFrom` (explicitly kept stable for RFC-tagged tests), still requires passing `wireCount=0` for RFC 7606 synthesized `typedWithdrawBodies` / `bodies[1:]`, and still misses wire UPDATEs dropped before `deliverReceivedMessage` (`ignore-families`, `receiveCeiling`, `prefixDrop`, empty treat-as-withdraw) |
| Guard `session.onUpdateRecv` with `p.sessionOwnsStats(session)` (which takes `Peer.mu.RLock()`) on every UPDATE | Adds a mutex `RLock`/`RUnlock` per wire UPDATE on the hot path before coalescing, undermining the lock-reduction benefit of coalescing; using `privateNegotiationEpoch.selected` (`atomic.Bool`) only on incoming candidates keeps both selected sessions and adopted candidates 100% lock-free |
| Keep `peer.incrUpdatesReceived()` in `notifyOneMessage` AND add a separate `onCoalescedUpdate` delta callback for merged messages only | Splits one counter across two producers (one semantic, one coalescer-internal), leaves RFC 7606 multi-body over-counting and pre-delivery under-counting broken, and duplicates accounting logic |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase 1: Wiring and failing tests (MANDATORY FIRST)**
   - Add `TestCoalescedUpdatesReceivedCountsWireMessages`, `TestStandardPathUpdatesReceivedCountsWireMessages`, `TestUpdatesReceivedCountsOnePerWireMessageOnSyntheticSplits`, and `TestIncomingCandidatePreOpenUpdateDoesNotMutateIncumbentStats` in `internal/component/bgp/reactor/session_coalesce_test.go`.
   - Verify `TestCoalescedUpdatesReceivedCountsWireMessages` and `TestUpdatesReceivedCountsOnePerWireMessageOnSyntheticSplits` FAIL against HEAD (proving coalesced 2-UPDATE burst reports 1 instead of 2 and multi-body treat-as-withdraw over-counts).
2. **Phase 2: Wire-boundary counter producer**
   - Add `onUpdateRecv func()` on `Session` (`session.go`) and invoke it in `Session.observeReceivedFrame` (`session_read.go`) when `kind == msgtype.TypeUPDATE`.
   - Replace the duplicated inline wire-observation block in `Session.readAndProcessCoalesced` (`session_coalesce.go`) with `s.observeReceivedFrame(hdr.Type, buf.Buf[:hdr.Length])`.
   - Add `selected atomic.Bool` and `isSelected()` on `privateNegotiationEpoch` (`session_open_publication.go`), set in `selectMessages()`.
   - Wire `session.onUpdateRecv` in `Peer.attachStatsCallbacks` (`peer_stats.go`), direct for selected sessions and gated on `private.isSelected()` for `privateNegotiationEpoch` candidates.
   - Remove `peer.incrUpdatesReceived()` from `Reactor.notifyOneMessage` (`reactor_notify.go`), keeping EOR counting and hooks untouched.
   - Update `TestAtomicRetirementImportPublication` (`filter_atomic_retirement_pipeline_test.go:196`) with independent `ze-test-judge` approval.
   - Verify all unit tests in `internal/component/bgp/reactor/` pass under `-tags ze_bgp`.
3. **Phase 3: Functional test and documentation**
   - Add `plugin/bgp-coalesced-updates-received-count` driver in `internal/test/fixture/plugin_fixture_09_bgp.go` and `test/plugin/bgp-coalesced-updates-received-count.ci`.
   - Update `docs/architecture/core-design.md` (`### Receive UPDATE coalescing`).
   - Run the functional test (plus vacuity check by temporarily reverting the producer change to confirm red, then restoring to confirm green) and scoped lint/doc checks in the main thread.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has producing code and a passing test at file:line |
| Feature completeness | User story 1 works end-to-end across `show bgp peer detail`, `show metrics`, and `show bgp rib` |
| Correctness | `observeReceivedFrame` counts `TypeUPDATE` only (never `TypeOPEN`, `TypeKEEPALIVE`, `TypeNOTIFICATION`, `TypeROUTEREFRESH`), runs once per wire frame on both standard and coalescing paths, and never double-counts on `flushCoalesce`, `typedWithdrawBodies`, `bodies[1:]`, or `ingressPolicyContinuation` |
| Lock-free hot path | `session.onUpdateRecv` takes neither `Peer.mu` nor `Reactor.mu` on selected or adopted sessions, and `readAndProcessCoalesced` uses `s.addrLabel` via `observeReceivedFrame` with zero per-message string allocations |
| EOR preservation | `Peer.incrEORReceived()` and all four EOR hooks stay in `Reactor.notifyOneMessage` gated on `policyDisposition != ingressPolicyAlreadyApplied` |
| Candidate isolation | Unchosen `privateNegotiationEpoch` candidates do not mutate incumbent `Peer.Stats().UpdatesReceived`; adopted candidates do after `selectMessages()` |
| Rule: `ai/rules/testing.md` | `filter_atomic_retirement_pipeline_test.go:196` change has independent `ze-test-judge` approval recorded before commit |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Wire-level `onUpdateRecv` wired in `observeReceivedFrame` and `attachStatsCallbacks`, removed from `notifyOneMessage` | `git grep -n "incrUpdatesReceived\|onUpdateRecv" internal/component/bgp/reactor/` |
| Unit tests passing with race detector | `./le test unit pkg path ./internal/component/bgp/reactor/ run "TestCoalesce\|TestStandardPathUpdatesReceived\|TestUpdatesReceivedCountsOne\|TestIncomingCandidatePreOpenUpdate\|TestAtomicRetirementImportPublication\|TestPeerStats\|TestPeerEventsIncrementChurnCounters"` |
| Functional test passing (and red-on-revert proven) | `./le test functional plugin bgp-coalesced-updates-received-count` and `./le test functional plugin rfc7606-withdraw` and `./le test functional plugin metrics-session-lifecycle` |
| Lint and doc anchor checks clean | `./le go lint run scope "internal/component/bgp/reactor"` and `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Unauthenticated/unadopted incoming connections | An incoming collision loser (`privateNegotiationEpoch` before `selectMessages()`) sending stray pre-OPEN UPDATEs must not inflate or corrupt the incumbent Established peer's `UpdatesReceived` counter |
| Header validation precedes counting | `ParseHeader` (marker check) and `ValidateLengthWithMax` (min 23 octets, max 4096/65535) and complete `io.ReadFull` of the body must all succeed before `observeReceivedFrame` increments `UpdatesReceived`, so truncated or invalid-header frames never increment the counter |

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

## Review Gate

To be completed by an independent review context during `/ze-close` (appending `plan/templates/TEMPLATE-CLOSURE.md`).

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `counter-counts-the-wrong-packets.md` | 2026-10-29 | Coalesced UPDATEs counted once in `updates-received` |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Count received UPDATEs inside `Session.observeReceivedFrame` via a lock-free `Session.onUpdateRecv` callback and remove `peer.incrUpdatesReceived()` from `Reactor.notifyOneMessage` | Threading a `wireCount` through `coalesceState`, `MessageCallback`, and `notifyMessageReceiverFrom`; or adding a second coalescer-only delta callback | `observeReceivedFrame` is already the single complete-frame wire boundary shared by capture tee, MRT, and `wireBytesRecv`; keeps `MessageCallback` and `notifyMessageReceiverFrom` signatures untouched, fixes RFC 7606 multi-body over-counting and pre-delivery under-counting at once, and eliminates duplicated observation code in `session_coalesce.go` |
| Gate `privateNegotiationEpoch` candidates with an `atomic.Bool` set in `selectMessages()` instead of calling `Peer.sessionOwnsStats` (which acquires `Peer.mu.RLock()`) | Calling `p.sessionOwnsStats(session)` inside `onUpdateRecv`, or ignoring candidate isolation | Prevents unchosen incoming TCP connections from polluting an incumbent peer's counters while keeping both normal and adopted sessions 100% lock-free on the hot path |

## Known Limitations

- `Session.readAndProcessCoalesced` does not call `s.onRead()` (`touchLastRead`), whereas `Session.readFrame` does (`session_read.go:104`). Fixing `LastReadTime` on the coalescing path is outside this spec's scope and left unchanged here.

## RFC Documentation (Scope: protocol)

No new RFC MUST/MUST NOT wire enforcement is added or altered by this fix; existing RFC 4271 Section 6.1 header/length validation and RFC 8654 Section 6 extended-length validation continue to run before `observeReceivedFrame`, and RFC 4724 Section 2 End-of-RIB detection remains in `notifyOneMessage`.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 4271 Sections 4.1, 4.3, 6.1; RFC 8654 Section 6 | Default (`ze.bgp.reactor.coalesce=true`): every validated complete BGP UPDATE wire frame (length 23..4096, or 23..65535 with Extended Message) increments `updates-received` by 1 regardless of internal coalescing | `ze.bgp.reactor.coalesce=false` (standard non-coalescing read path) produces identical `updates-received` counts | `TestCoalescedUpdatesReceivedCountsWireMessages`, `test/plugin/bgp-coalesced-updates-received-count.ci` | `TestStandardPathUpdatesReceivedCountsWireMessages` |
| RFC 4724 Section 2 | Default: an End-of-RIB empty UPDATE increments `updates-received` by 1 (as a wire UPDATE) and `eor-received` by 1 (at semantic dispatch) | Synthetic policy continuations (`ingressPolicyAlreadyApplied`) increment neither `updates-received` nor `eor-received` | `TestCoalescedUpdatesReceivedCountsWireMessages`, `test/plugin/bgp-coalesced-updates-received-count.ci` | `TestAtomicRetirementImportPublication`, `TestUpdatesReceivedCountsOnePerWireMessageOnSyntheticSplits` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions with positive Expected Behavior and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped verification run in the main checkout by the main thread: `./le test unit pkg path ./internal/component/bgp/reactor/`, `./le test functional plugin bgp-coalesced-updates-received-count`, `./le test functional plugin rfc7606-withdraw`, `./le test functional plugin metrics-session-lifecycle`, `./le go lint run scope "internal/component/bgp/reactor"`, and `./le doc check all`
- [ ] Feature code integrated (`internal/component/bgp/reactor/`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes and `ze-test-judge` approval recorded for `filter_atomic_retirement_pipeline_test.go`
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` test passes and vacuity (red-on-revert) proven

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; update the owned row in `plan/journal/counter-counts-the-wrong-packets.md` to `fixed:`
- [ ] **Commit A:** code + tests + docs + edited spec + updated journal row
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-received-update-counter-counts-wire-messages.md` only
