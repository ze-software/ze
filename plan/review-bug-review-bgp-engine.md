# BGP Engine Core Bug Review Report

Generated: 2026-06-19
Child spec: `plan/spec-bug-review-3-bgp-engine-core.md`
Inventory: `plan/review-bug-review-inventory.md`

## Summary

Reviewed BGP core engine surfaces named by child 3, excluding BGP plugin implementation internals except as call-site seams. Four findings are confirmed and one remains plausible. The highest-risk confirmed bug is oversized forwarding taking the raw split path before context conversion, which can send source-context UPDATE bytes to a destination with different negotiated capabilities.

| Result | Count | IDs |
|---|---:|---|
| Confirmed findings | 4 | BENG-001, BENG-002, BENG-003, BENG-004 |
| Plausible findings | 1 | BENG-005 |
| Rejected candidates | 5 | R1..R5 |

## Scope and files read

| Area | Files and ranges read | Evidence used |
|---|---|---|
| Spec and inventory | `plan/spec-bug-review-3-bgp-engine-core.md:1-526`, `plan/spec-bug-review-0-umbrella.md:1-163`, `plan/spec-bug-review-1-inventory-and-self-containment.md:1-183`, `plan/review-bug-review-inventory.md:1-153` | Child 3 owns BGP core engine files, BGP plugin internals excluded, and child 5 consumes complete evidence-backed findings. |
| Architecture | `docs/architecture/core-design.md:23-180`, `181-223`, `496-717`, `972-1058` | Receive, build, forwarding, cache, RIB/FIB, zero-copy and DirectBridge path expectations. |
| Rules and guidance | `skill://ze-review`, `skill://ze-hunt`, `skill://ze-find-alloc`, `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md`, `ai/rules/no-sprintf-alloc.md`, `ai/rules/rfc-compliance.md`, `plan/learned/RECURRING-PATTERNS.md:1-170` | Evidence rules, RFC checks, hot-path allocation constraints, wiring checks, silent parser fall-through traps. |
| RFC summaries | `rfc/short/rfc4271.md`, `rfc/short/rfc4760.md`, `rfc/short/rfc5492.md`, `rfc/short/rfc2918.md`, `rfc/short/rfc7313.md`, `rfc/short/rfc7606.md`, `rfc/short/rfc8654.md`, `rfc/short/rfc6793.md`, `rfc/short/rfc7911.md:1-223` | Protocol status for OPEN, UPDATE, capabilities, route refresh, error handling, extended messages, ASN4, and ADD-PATH. |
| Wire and lazy update | `internal/component/bgp/wireu/wire_update.go:1-320`, `internal/component/bgp/wire/update_sections.go:1-157`, `internal/component/bgp/wireu/split.go` via searches | Lazy section parsing, snapshot, EOR, split path. |
| Message build/parse | `internal/component/bgp/message/header.go:117-210`, `open.go:1-220`, `update.go:52-150`, `update_build.go:1-130,213-384` | Header size gates, OPEN optional params, UPDATE builder lifetime, raw/parsed encoding. |
| Attribute, filter, and context seams | `internal/component/bgp/attribute/builder_parse.go:1-333`, `attribute/wire.go:1-440`, `internal/component/bgp/filterapi/filterapi.go:1-335`, `internal/component/bgp/context/context.go:1-223` | Command/API attribute parsing, lazy attribute lifetime, filter ordering, mod accumulator contracts, ContextID and ADD-PATH/ASN4 derivation. |
| Session/FSM and route refresh | `internal/component/bgp/reactor/session_read.go:120-260`, `session_handlers.go:1-342`, `session_negotiate.go:1-260` | Callback ordering, OPEN validation, negotiated timers, route-refresh receive. |
| Reactor startup/reload/API | `internal/component/bgp/reactor/reactor.go:91-307,547-691,914-1288,1301-1368`, `reactor_api.go` search hits, `reactor_api_forward_batch.go:1-170`, `internal/component/bgp/reactor/config.go:1-260,360-620` | API server startup, cleanup, DirectBridge forwarding, destination cap, native family validation, config parser input bounds. |
| Forwarding/cache | `internal/component/bgp/reactor/received_update.go:1-160`, `recent_cache.go:1-260,360-540`, `reactor_api_forward.go:78-159,161-913`, `reactor_api_forward_batch.go:1-170`, `forward_body.go:1-170`, `forward_pool.go:94-180,820-920`, `forward_rs.go:99-393` | Retain/release, slow path, DirectBridge, RS inline fast path, batching, split and write semantics. |
| Config/reload seams | `internal/component/bgp/config/peers.go:28-120,240-370`, `internal/component/bgp/reactor/config.go:1-260,360-620` | Peer defaults, timer/port/family bounds, prefix limit validation, route config migration overlap. |

### Review lens coverage

| Lens | Coverage result |
|---|---|
| Receive path | Header validation, UPDATE validation ordering, lazy payload lifetime, cache ownership, and route-refresh receive ordering traced. |
| Build path | OPEN optional parameters, UPDATE builder scratch lifetime, message size bounds, attribute parse/build, and command attribute parsing traced. |
| Forwarding parity | Slow path, DirectBridge, and RS inline fast path compared for source exclusion, egress filters, next-hop policy, retain/release, and context/split handling. |
| Capability/context correctness | Capability TLV parsing, negotiated context derivation, ADD-PATH/ASN4/extended-message effects, and ContextID zero-copy decisions checked. |
| Malformed input/RFC behavior | Protocol findings cite RFC 2918, 4271, 5492, 7313, 7606, 8654, 6793, or 7911, or state non-RFC lifecycle scope. |
| Cache, pool, zero-copy safety | Recent cache retain/release, EBGP variants, `WireUpdate` snapshots, `AttributesWire` no-copy ownership, and forwarding pool release paths checked. |
| Config/reload/API limits | Parser bounds for timer, ports, family keys, prefix limits, forwarding destinations, and startup failure cleanup checked. |
| Concurrency | `sync.Once` wire parsing, `AttributesWire` locks, filter registry locks, synchronous observer contract, cache ack modes, and startup goroutine cleanup checked. |
| Hot-path allocations | Forwarding mods, next-hop-self, message builder scratch, filter accumulator, and attribute re-pack paths checked for avoidable allocations. |

## Wiring/coverage audit table

| File group | Entry point checked | Coverage result |
|---|---|---|
| Receive path | TCP read -> `Session.processMessage` -> `notifyMessageReceiver` -> cache/dispatcher | Covered. `session_read.go:147-195` validates UPDATE before callback. Non-UPDATE validation ordering has finding BENG-003. |
| Build path | API/config route -> `UpdateBuilder`/batch builder -> `Peer.SendUpdate` | Covered. Builder scratch aliasing contract read at `update_build.go:38-41`, alloc bounds at `110-119`, unicast builder at `220-384`. |
| Forward slow path | `bgp cache forward` -> `ForwardUpdate` -> `forwardUpdateCore` -> `fwdPool` | Covered. Shared forwarding loop read at `reactor_api_forward.go:178-692`. |
| DirectBridge path | SDK `ForwardCached` -> `ForwardUpdatesDirect` -> `forwardUpdateCore` | Covered. Destination cap and ack path read at `reactor_api_forward_batch.go:53-150`. |
| RS inline fast path | `notifyMessageReceiver` -> `reactorForwardRS` before delivery enqueue | Covered. Inline path read at `reactor_notify.go:553-568`, `forward_rs.go:99-393`. |
| Cache retain/release | `RecentUpdateCache.Add/Activate/Ack/RetainN/Release/evictLocked` | Covered. FIFO/unordered and EBGP buffer release read at `recent_cache.go:39-59,368-471`. |
| Route refresh | API send and session receive | Covered. Send gates read at `reactor_api_forward.go:113-159`; receive ordering and validation read at `session_handlers.go:263-329`. |
| Config/reload lifecycle | startup, listener/API server, peer reconcile, dynamic groups | Covered. Startup cleanup finding BENG-004. Dynamic group validation remains in remaining risks. |
| Metrics/API limits | destination cap and per-peer stats | Covered. Forward destinations capped at `reactor_api_forward_batch.go:57-60`; refresh counters read in `peer_stats.go` search evidence. |

### Receive matrix

| Lens | Files | Result |
|---|---|---|
| Header length/type | `message/header.go:117-210`, `session_read.go:100-107` | UPDATE, OPEN, KEEPALIVE, NOTIFICATION length gates present. ROUTE-REFRESH allows extra payload until handler, see BENG-003. |
| UPDATE RFC 7606 | `session_read.go:153-195`, `message/rfc7606.go` searched | UPDATE validation runs before plugin delivery. |
| Lazy payload lifetime | `reactor_notify.go:512-529`, `received_update.go:40-60`, `recent_cache.go:455-471` | Cache owns pool buffer and evicts original plus EBGP variants. |
| Malformed MP/NLRI | `wire_update.go:122-168`, `rfc4760`, `rfc7606` | Minimum MP_REACH/UNREACH checks present, deeper per-family behavior needs fuzz coverage. |

### Build matrix

| Lens | Files | Result |
|---|---|---|
| OPEN capabilities | `session_negotiate.go:80-239`, `capability.go:814-845` | Optional parameter splitting present. Malformed capability TLVs are ignored, see BENG-001. |
| UPDATE builder | `update_build.go:220-384` | Attribute ordering and IPv4 vs MP_REACH placement covered. Hot-path config allocations noted under BENG-005. |
| Max size | `update_build.go:100-119`, `peer_send.go` search hits | Builder caps scratch at RFC 8654 extended max, split wrappers exist. |

### Forward matrix

| Lens | Slow path | DirectBridge | RS inline fast path | Result |
|---|---|---|---|---|
| Shared core loop | `ForwardUpdate` -> `forwardUpdateCore` | `ForwardUpdatesDirect` -> `forwardUpdateCore` | Separate `reactorForwardRS` mirrors logic | Mostly covered. Oversized split before context conversion affects slow and DirectBridge via shared `buildFwdBody`, and RS via same helper. |
| Source exclusion | `reactor_api_forward.go:197-219` | `reactor_api_forward_batch.go:121-128` | `forward_rs.go:99-116` | Covered. |
| Egress filters and mods | `reactor_api_forward.go:469-624` | same | `forward_rs.go:314-410` | RS skips export filters by design and records `FastPathSkipped`. |
| Retain/release | `reactor_api_forward.go:672-684` | same plus ack at `reactor_api_forward_batch.go:138-143` | `forward_rs.go:450-459` search/read | Covered, with stopped-pool release tests present in search evidence. |
| Context/size split | `forward_body.go:40-96` | same | same helper | Finding BENG-002. |

### Refresh/reload matrices

| Path | Files | Result |
|---|---|---|
| Normal route refresh send | `reactor_api_forward.go:113-159` | Gates on negotiated RouteRefresh. |
| BoRR/EoRR send | `reactor_api_forward.go:144-149` | Gates on EnhancedRouteRefresh. No finding because local config currently advertises base and enhanced together. |
| Route refresh receive | `session_read.go:221-240`, `session_handlers.go:263-329` | Callback runs before route-refresh body length/subtype validation, see BENG-003. |
| Startup failure | `reactor.go:923-969`, `1111-1162`, `1200-1208` | Listener/cache cleanup incomplete on `NewServer` failure, see BENG-004. |
| Reload peer diff | `reactor.go:589-600`, `reactor_api.go` search hits | Journal seam exists. Full rollback proof for every dynamic group edge remains in remaining risks. |

## Confirmed findings

### BENG-001: Malformed known capabilities are silently ignored or accepted during OPEN negotiation

- Severity: BLOCKER
- Owner: BGP capability and session OPEN negotiation, `internal/component/bgp/capability` and `internal/component/bgp/reactor`
- Source: `internal/component/bgp/capability/capability.go:197-222`, `capability.go:203-208`, `capability.go:814-845`, `internal/component/bgp/reactor/session_handlers.go:118-127`, `rfc/short/rfc5492.md:86-109`, `rfc/short/rfc8654.md:231-240`, `rfc/short/rfc2918.md:187-190`
- Reachable trigger: A peer sends an OPEN with a Type 2 capability optional parameter containing a malformed known capability TLV, for example Extended Message code 6 with non-zero length, Route Refresh code 2 with non-zero length, or a TLV whose declared length overruns the parameter value.
- Expected behavior: Known capabilities with fixed lengths must validate their length, and malformed capability TLV boundaries must reject the OPEN with an OPEN Message Error. Unknown capability codes are ignored, but malformed known capabilities must not be negotiated as if valid.
- Actual behavior: `parseCapability` returns `&RouteRefresh{}`, `&ExtendedMessage{}`, and `&EnhancedRouteRefresh{}` without checking that `data` length is zero. `ParseFromOptionalParams` calls `Parse` and appends parsed caps only when `err == nil`; parse errors such as `ErrShortRead` are otherwise dropped and negotiation continues with the malformed parameter ignored.
- Impact: A malicious or broken peer can enable capabilities it did not encode correctly, or can downgrade negotiated capabilities by sending truncated TLVs that are silently ignored. This affects max message size, route refresh behavior, and any downstream context ID derived from capabilities.
- RFC status: RFC 5492 requires TLVs to fit within the optional parameter. RFC 8654 says Extended Message capability length is 0 and a speaker not advertising it must not accept extended messages. RFC 2918 says Route Refresh capability length is 0.
- Regression test plan: Add capability parser tests for code 2, 6, and 70 with non-zero lengths and for a Type 2 optional parameter with a truncated inner TLV. Add a session OPEN test that asserts malformed known capability data sends OPEN Message Error and does not establish or negotiate the capability.

### BENG-002: Oversized forwarding bypasses destination context conversion and can send source-encoded UPDATE chunks

- Severity: BLOCKER
- Owner: BGP engine forwarding, `internal/component/bgp/reactor`
- Source: `internal/component/bgp/reactor/forward_body.go:40-54`, `reactor_api_forward.go:632-646`, `forward_rs.go:416-438`, `rfc/short/rfc8654.md:235-240`, `rfc/short/rfc7911.md:111-123`, `rfc/short/rfc6793.md:261-267`
- Reachable trigger: A cached UPDATE larger than a destination peer's max message size is forwarded to a peer whose `destCtxID` differs from `peerWire.SourceCtxID`, for example ADD-PATH enabled on source but not destination, ASN4/ASN2 mismatch, or another negotiated-context mismatch. The path is reachable through both `ForwardUpdate` and DirectBridge because both call `forwardUpdateCore`, and the RS inline path calls `buildFwdBody` too.
- Expected behavior: When source and destination encoding contexts differ, the reactor must parse and re-encode for the destination capabilities before splitting, then ensure each outbound UPDATE fits RFC 8654 peer size. Same `ContextID` is the only zero-copy condition per `core-design.md:116-129`.
- Actual behavior: `buildFwdBody` checks `updateSize > maxMsgSize` first. In that branch it calls `wireu.SplitWireUpdate(peerWire, maxBodySize, srcCtx)` and appends `split.Payload()` to `rawBodies`, without checking `destCtxID` and without re-encoding for the destination. The later zero-copy/context branch is skipped entirely.
- Impact: A peer can receive UPDATE chunks encoded with the source peer's ADD-PATH, ASN width, or other negotiated wire context. This can make NLRI boundaries wrong, AS_PATH width wrong, or attributes inconsistent. It is peer-visible protocol corruption and can drop or misroute routes.
- RFC status: Violates RFC 8654 size handling because splitting must account for the recipient's negotiated max message size and encoding. Can violate RFC 7911 ADD-PATH negotiation and RFC 6793 ASN4 encoding if the context mismatch is those capabilities.
- Regression test plan: Add a reactor forwarding unit test that creates an oversized cached UPDATE with source context ADD-PATH true and destination context ADD-PATH false, calls `ForwardUpdatesDirect`, and asserts emitted raw/update bodies decode with the destination context and contain no path IDs. Add a second case for ASN4 source to ASN2 destination with a large AS_PATH forcing split.

### BENG-003: Malformed ROUTE-REFRESH is delivered to plugins before body validation

- Severity: ISSUE
- Owner: BGP session receive path, `internal/component/bgp/reactor`
- Source: `internal/component/bgp/reactor/session_read.go:221-240`, `internal/component/bgp/reactor/session_handlers.go:263-329`, `internal/component/bgp/message/header.go:142-143`, `rfc/short/rfc2918.md:66-76`, `rfc/short/rfc7313.md:94-103`
- Reachable trigger: A peer sends a ROUTE-REFRESH message whose BGP header length is at least 23 but whose body is not exactly 4 bytes, for example body length 5.
- Expected behavior: ROUTE-REFRESH payload length must be exactly 4 bytes. For BoRR/EoRR invalid length, RFC 7313 requires a ROUTE-REFRESH Message Error, Invalid Message Length, and the malformed message must not be presented as a valid event to consumers.
- Actual behavior: `processMessage` calls `s.onMessageReceived(...)` for all message types before switching to `handleRouteRefresh`. The route-refresh handler validates `len(body) != 4` only after the callback. Header validation only enforces the minimum route-refresh length, not exact length.
- Impact: Internal or external consumers subscribed to refresh events can observe and act on malformed refresh/BoRR/EoRR before the session sends the required NOTIFICATION and closes. This breaks the receive-path invariant already enforced for UPDATEs, where RFC validation runs before plugin delivery.
- RFC status: RFC 2918 defines fixed 4-byte route-refresh payload; RFC 7313 requires subtype processing and invalid-length notification for BoRR/EoRR.
- Regression test plan: Add a session unit test with an `onMessageReceived` spy and a 5-byte route-refresh body. Assert the spy is not called, the notification code/subcode is 7/1 for BoRR/EoRR or header bad length for normal refresh per chosen policy, and the connection closes.

### BENG-004: Startup failure after listener/cache start can leak reactor resources

- Severity: ISSUE
- Owner: BGP reactor lifecycle, `internal/component/bgp/reactor`
- Source: `internal/component/bgp/reactor/reactor.go:923-969`, `internal/component/bgp/reactor/reactor.go:937-943`, `internal/component/bgp/reactor/reactor.go:1111-1162`, `internal/component/bgp/reactor/reactor.go:1200-1208`, `internal/component/plugin/server/server.go:173-177`
- Reachable trigger: `StartWithContext` starts the recent update cache scanner and listeners, then `startAPIServer` fails while creating the plugin server, for example because `yang.DefaultLoader()` returns an error in `pluginserver.NewServer`.
- Expected behavior: Any startup failure after resources are started should call a single abort path that stops listeners, cancels the reactor context, and stops background loops. `startMultiListeners` and API server start failures already call `abortStartup` in some paths.
- Actual behavior: `StartWithContext` returns immediately when `startAPIServer()` returns an error at `reactor.go:967-969`. The `pluginserver.NewServer` error path in `startAPIServer` returns `fmt.Errorf("create plugin server: %w", serverErr)` without calling `abortStartup`. The recent cache goroutine started at `reactor.go:937-938` and any listeners started before that point remain active until an external caller notices and calls `Stop` on a reactor that never reached `running=true`.
- Impact: Failed startup can leave bound sockets, event bus subscriptions, and cache scanner goroutines behind. A subsequent reload/start can fail due to ports still in use or duplicate event handling.
- RFC status: Not protocol RFC. Lifecycle/resource correctness.
- Regression test plan: Add a reactor startup unit test using a listener factory that records `Stop`, plus a plugin server creation failure injected through a YANG loader/server factory seam. Assert `StartWithContext` returns an error, listeners are stopped, `r.ctx` is canceled, and `recentUpdates.Stop` is called or the scan loop exits.

## Plausible findings

### BENG-005: IPv6 link-local next-hop-self allocates on the per-forward hot path

- Severity: ISSUE
- Owner: BGP forwarding/filterapi mods, `internal/component/bgp/reactor`
- Source: `internal/component/bgp/reactor/reactor_api_forward.go:773-856`, especially `819`, `ai/rules/no-sprintf-alloc.md:150-162`, `ai/rules/memory-architecture.md:172-323`
- Reachable trigger: A peer has `next-hop self`, an IPv6 local address, and a valid IPv6 link-local address. Every forwarded UPDATE for that peer calls `applyNextHopMod`, allocates `make([]byte, 32)`, then stores it in the mod accumulator.
- Expected behavior: Per-UPDATE forwarding paths should use caller-owned or stack buffers and copy into the eventual pooled modification buffer, avoiding heap allocation on every route.
- Actual behavior: The 32-byte next-hop value is created with `make([]byte, 32)` in the per-destination loop before `mods.Op`.
- Impact: Hot-path heap allocation and GC pressure for IPv6 route-server or transit peers using link-local next-hop-self. The behavior is correct on the wire, but violates the no-allocation forwarding rule.
- RFC status: RFC 2545 next-hop encoding relevant, but this is a performance/lifetime finding, not a protocol violation.
- Regression test plan: Add an allocation benchmark or `testing.AllocsPerRun` unit around `applyNextHopMod` for IPv6 local plus link-local. Expected after fix: zero heap allocations for constructing the mod value.
- Evidence status: Plausible because the source shows a heap slice, but escape behavior should be confirmed with `go test -run TestName -bench/allocs` or `go test -gcflags=-m` in the fix spec.

## Rejected candidates with proof

| ID | Candidate | Proof and disposition |
|---|---|---|
| R1 | UPDATEs delivered before RFC 7606 validation | Rejected. `session_read.go:153-195` runs `enforceRFC7606` and family validation before `onMessageReceived` at `221-227`. |
| R2 | Cache original and EBGP variant buffers leak on normal eviction | Rejected. `recent_cache.go:459-466` returns `poolBuf`, `ebgpSlotASN4`, and `ebgpSlotASN2`. `Delete` duplicates the same returns at `521-533`. |
| R3 | DirectBridge allows unlimited destination lists | Rejected. `reactor_api_forward_batch.go:57-60` rejects lists larger than `maxForwardDestinations`, default 4096. |
| R4 | Unknown capability code causes session reset | Rejected. `capability.go:193-222` returns `Unknown` for unrecognized codes and RFC 5492 requires ignore. The problem is malformed known capability lengths, not unknown codes. |
| R5 | Route-refresh send ignores base RouteRefresh for normal refresh | Rejected. `reactor_api_forward.go:137-142` gates normal refresh on `neg.RouteRefresh`. |

## Cleared classes

| Class | Evidence |
|---|---|
| Receive UPDATE buffer lifetime | `ReceivedUpdate` stores the pool handle at `received_update.go:58-60`; cache eviction returns it at `recent_cache.go:459-466`. |
| FIFO vs unordered cache ack | FIFO cumulative ack and unordered per-entry ack are explicitly separated at `recent_cache.go:368-386`. |
| Forward destination resource cap | `reactor_api_forward_batch.go:57-60` enforces cap. |
| Unknown capability handling | RFC-compliant ignore path at `capability.go:193-222`. |
| Route-refresh send capability gates | Normal and enhanced gates present at `reactor_api_forward.go:137-149`. |
| Observer blocking contract documented | Synchronous observer warnings are explicit at `reactor.go:189-203`; no blocking observer implementation was reviewed in core. |

## Assumptions resolved

| Assumption | Status | Evidence |
|---|---|---|
| Child 3 owns core engine, not BGP plugin internals | Confirmed | Inventory `plan/review-bug-review-inventory.md:131-135`. |
| Slow path, DirectBridge, and RS inline should preserve forwarding semantics | Confirmed | `core-design.md:536-578`; shared helper evidence in `forward_body.go`. |
| RFC summaries exist for protocol findings | Confirmed for reported findings | RFCs 2918, 4271, 5492, 7313, 7606, 8654, 6793, and 7911 read. |
| Active spec overlap | Recorded | No confirmed finding directly targets `spec-exabgp-compat-sync.md` or `spec-route-config-plugin-migration.md`. BENG-004 may affect startup/reload lifecycle, but it is core reactor, not route config plugin migration. |

## Remaining risks

| Risk | Why still open | Suggested owner/action |
|---|---|---|
| Dynamic peer group validation gaps | Read `buildDynamicGroupSettings` but did not fully trace every validation inherited from `parsePeerFromTree`. | Child 5 or a follow-up BGP config/reload review should compare dynamic group accepted fields against static peer parser. |
| Full RFC 7606 per-attribute parity | UPDATE validation path was checked for ordering before dispatch, but not every attribute action was re-audited. | Add a targeted RFC 7606 matrix review over `message/rfc7606.go` and attribute parsers. |
| Pool lifetime for temporary EBGP wires generated inside forwarding calls | Cache-owned EBGP variants are released. Per-call transient wires in `getEBGPWire` rely on downstream fwd item resource paths and need a focused lifetime proof. | Include in BENG-002 fix review, because context/split changes will touch the same path. |
| API command input length caps beyond forwarding destinations | Destination cap was verified. Text command parser and selector token sizes were not exhaustively bounded in this pass. | Child 5 may add a small command/API input-limit audit if needed. |
