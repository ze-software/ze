# Spec: ike-virtual-ip-assignment

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | `plan/spec-ipsec-remote-access.md` |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 3948 Section 5.1 states an obligation Ze does not meet. Two remote peers behind
separate NATs, each having chosen the inner address 10.1.2.3 for itself, both reach one
security gateway. The RFC's words, read at Section 5.1 in `rfc/full/rfc3948.txt`:

> Because SGW will now see two possible SAs that lead to 10.1.2.3, it can become confused
> about where to send packets coming from Suzy's server. Implementors MUST devise ways of
> preventing this from occurring.

The same section names the remedy it prefers:

> It is RECOMMENDED that SGW either assign locally unique IP addresses to Ari's and Bob's
> laptop (by using a protocol such as DHCP over IPsec) or use NAT to change Ari's and Bob's
> laptop source IP addresses to these locally unique addresses before sending packets
> forward to Suzy's server.

Ze is that security gateway. It terminates tunnel-mode Child SAs, and it carries a
`remote-access` configuration surface whose `pool` list exists to hand out exactly the
locally unique addresses the section recommends. It devises no way of preventing the
conflict, because it assigns no address at all.

**The goals of this work:**

1. **G-1 Conformance.** Close `RFC3948-5.1-1`: every remote peer that asks for an inner
   address receives one that no other live peer holds, so the gateway never sees two SAs
   leading to the same inner address.
2. **G-2 Reachability.** Make `Pool.Allocate` reachable from the wire. The allocator is
   correct and has no non-test caller; a correct allocator nobody calls emits nothing, so
   no layer performs the requirement (`ai/rules/rfc-compliance.md`).
3. **G-3 Interoperability.** A strongSwan client configured for a virtual IP receives one
   from Ze's pool and passes traffic over the resulting Child SA.
4. **G-4 Honest configuration.** An operator who configures `remote-access pool` gets the
   behavior the YANG description promises, instead of a daemon that accepts the config and
   assigns nothing.

**This reverses a recorded design decision, on Thomas's ruling of 2026-08-31.** The
RFC 7296 row of `docs/features/rfc-status.md` records the decision being reversed:

> Sections 2.19, 2.20 and 3.15.1: Ze builds no Configuration payload. It takes no IRAC
> role, which Section 4 permits. It ignores CFG_SET, which Section 3.15.1 permits. It sends
> no CFG_REQUEST, no CFG_REPLY and no CFG_ACK, and it gives out no version string.

That sentence is not a gap disclosure. It is a positive claim that Ze deliberately declines
the role, resting on RFC 7296 Section 4, which lists the Configuration payload outside the
conformance set an IKEv2 implementation owes. This spec makes it false. Ze becomes an IRAS
(the responder half of the address-assignment exchange) while still declining the IRAC half.
The blast radius of the reversal is set out below; it is not a wiring job.

**Surfaces the reversed decision touches, and what each owes after this work:**

| Surface | What it says today | What it owes |
|---------|--------------------|--------------|
| `docs/features/rfc-status.md`, RFC 7296 row | "Ze builds no Configuration payload. It takes no IRAC role" | Rewritten: Ze builds a CFG_REPLY as responder, still sends no CFG_REQUEST and takes no IRAC role |
| `docs/features/rfc-status.md`, RFC 3948 row | Gap on Section 5.1, recorded by this work before the spec ran | Gap removed once the tagged tests and the interop scenario are green |
| `rfc/short/rfc3948.md`, `RFC3948-5.1-1` | `{gap}` naming the unreached producer | Gap annotation removed, requirement proven |
| `rfc/short/rfc7296.md`, the Section 2.19 / 2.20 / 3.15.1 rows | Not implemented on the responder | Implemented rows, each with a tagged test and a discrimination record |
| `rfc/extraction/rfc3948.json`, site `5.1:1` | `mapped` to `RFC3948-5.1-1`, reason names the unreached producer | Reason rewritten to name the producing function that now answers |
| `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`, `remote-access` | The `x509` leaves say "It has no effect today" | Unchanged by this spec; those leaves belong to `plan/spec-ipsec-remote-access.md` |
| `docs/guide/ipsec.md` | Says the pool is not wired (corrected 2026-08-16 during the wiki sync) | Says what an operator gets |
| `plan/journal/unwired-feature.md`, the 2026-08-16 row | Records the discard as not fixed | Stays as the historical record; closure adds no second row |

**Relationship to `plan/spec-ipsec-remote-access.md` (OPEN OWNER DECISION, OI-1).** That
spec exists at Status `design` and already carries the Configuration-payload work as its
phases A to G, with acceptance criteria AC-7 to AC-10 and AC-16 to AC-28 covering the same
behavior this spec covers. Two specs declaring the same work is a future disagreement with
nothing to arbitrate it (`ai/rules/principles.md`). This spec is written as the
address-assignment slice cut out of that one, narrowed to the RFC 3948 Section 5.1
obligation and to what makes `Pool.Allocate` reachable. **Thomas must decide which of the
two owns the CP work before implementation starts.** The two viable answers are: this spec
supersedes phases A, B, D and E of `plan/spec-ipsec-remote-access.md` and those rows are
repointed here, or this spec is folded into that one and deleted. Implementing both is not
an answer. Neither is implementing this one while the rows in the other stay live.

**A false conformance claim is on the ledger today (OPEN OWNER DECISION, OI-2).** Two tests
in `internal/component/ike/eap/pool_test.go` carry an `RFC requirement: RFC3948-5.1-1` tag.
`TestVirtualIPPoolNeverHandsOneAddressTwice` is tagged positive and its claim reads "ze
prevents the RFC 3948 Section 5.1 conflict the way the section recommends: the gateway
assigns each client a locally unique address instead of carrying the address the client
brought with it". `TestVirtualIPPoolExhausted` is tagged negative and claims "the security
gateway refuses rather than hand a second client an inner address that is already in use".
Neither is true of the gateway. Both test bodies drive `Pool.Allocate` directly, and that
function has no non-test caller, so the gateway assigns nothing and refuses nothing. The
claims are wider than the assertions, which `ai/rules/rfc-compliance.md` names as the
violation with a green bar on top.

The two tags and the `{gap}` on `RFC3948-5.1-1` cannot both stand: `./le rfc check` refuses a
`{gap}` on a requirement that carries any tagged unit (`checkStaleAnnotations`,
`internal/le/rfc/check_core.go`). Correcting the claims means editing a tagged test, which
the write hook refuses without an owner-approved row in `test/rfc-changed.md`. **Thomas must
approve narrowing those two claims to what the test bodies check.** Until he does,
`./le rfc check` reports the stale-annotation error against `rfc/short/rfc3948.md`, and that
red is the honest state: removing the gap instead would restore the false claim. When this
spec lands, the tags become true and are restored with prose matching the assertions.

**What this spec does NOT cover.** Admission of an IKE_SA_INIT from an unconfigured source,
per-user EAP credentials, and the `remote-access authentication` surface stay in
`plan/spec-ipsec-remote-access.md`. Without admission, the address assignment specified here
is reachable only by a client that connects as a configured responder peer, which is how the
existing `responder-eap-mschapv2` and `responder-eap-tls` interop scenarios drive Ze today.
That is enough to prove and to ship the assignment behavior, and it is not enough to make
road-warrior access usable.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - the design page `eap/pool.go` and the
  EAP responder path declare in their `// Design:` headers
  → Decision: the virtual IP pool is documented here as the road-warrior address source, so
  this page states the intent the code never carried out and is the page a behavior change
  makes wrong first
  → Constraint: a page this change makes wrong is repaired in the same work, before the next
  code edit (`ai/rules/documentation.md`)
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - the responder exchange the CFG_REPLY
  must be inserted into
  → Constraint: the responder builds one IKE_AUTH response through
  `(*PeerSession).buildAuthResponse` (`internal/component/ike/engine/responder.go`) on the
  direct path and reaches the same builder through `(*PeerSession).sendResponderEAP`
  (`internal/component/ike/engine/responder_eap.go`) on the EAP path, so a reply inserted at
  one site only serves one of the two flows
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - Child SA install and the traffic
  selectors the leased address must reach
  → Constraint: the responder's TSr is decided before the Child SA is installed, so the
  address must be allocated before the traffic selectors are built or the lease never
  appears in the negotiated selectors and the client cannot route
- [ ] `docs/architecture/testing/interop.md` - the IPsec interop suite
  → Constraint: the suite is `test/interop-ipsec/`, driven by `./le integration
  interop-ipsec`, and a scenario directory is NAMED with no numeric prefix
- [ ] `docs/architecture/wire/buffer-writer.md` - the encoding contract `payload_cp.go`
  declares
  → Constraint: `WriteTo(buf, off) int` writes into a caller buffer; the CFG_REPLY builder
  allocates no intermediate encoding buffer

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3948.md` - `RFC3948-5.1-1`, the requirement this spec closes
  → Constraint: the obligation is to PREVENT the overlap, and the RECOMMENDED remedy is a
  locally unique address per peer; a design that assigns addresses but can hand the same one
  to two live peers has not met it
- [ ] `rfc/short/rfc7296.md` - Sections 2.19, 2.20, 3.15, 3.15.1 and 3.15.4, the Configuration
  payload
  → Constraint: the CFG_REPLY is sent in the message that carries the SA payload, which under
  EAP is the FINAL IKE_AUTH response and not the first
  → Constraint: an unrecognized configuration attribute is ignored, never rejected, and a
  message is never refused over payload order
- [ ] `rfc/short/rfc4301.md` - the SPD and SAD model the leased address is programmed into
  → Constraint: the leased address reaches the kernel only through the negotiated traffic
  selectors, so the lease is not effective until it is inside TSr

**Key insights:** (minimal context to resume after compaction)
- The pool is correct and unreached. The gap is a missing consumer, not a broken allocator.
- The Configuration payload codec exists and is registered on the decode path. What is
  missing is every engine-side use of it.
- Allocation must happen before traffic selectors are built, and the reply must be emitted in
  the response that carries the SA payload. Those two orderings are the whole difficulty.
- This work reverses a positive claim in `docs/features/rfc-status.md`, not a silence.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/eap/pool.go` - `NewPool` parses an IPv4 CIDR and an optional
  IPv6 CIDR, excludes network and broadcast for IPv4, and bounds an IPv6 prefix itself.
  `Pool.Allocate` returns an `AllocateResult` carrying IPv4, IPv6, split DNS lists and a
  search domain, refuses with `ErrPoolExhausted`, and rolls back an IPv4 lease when the IPv6
  half fails. `Pool.Release` and `Pool.Available` complete the surface. Every caller is a
  test: `pool_test.go` and `pool_release_test.go`.
- [ ] `internal/component/ike/engine/register.go` - `registerIKE` declares `var ipPool
  *eap.Pool`, fills it from `cfg.RemoteAccess` through `eap.NewPool` on each config apply,
  logs `ike: virtual IP pool created`, and then discards it at a bare `_ = ipPool` on the
  teardown path. Nothing between those points reads it.
- [ ] `internal/component/ike/wire/payload_cp.go` - the Configuration payload codec.
  **Present:** the four CFG types (`CFGTypeRequest`, `CFGTypeReply`, `CFGTypeSet`,
  `CFGTypeACK`), nine attribute-type constants including `CPAttrInternalIP4Address`,
  `CPAttrInternalIP4Netmask`, `CPAttrInternalIP4DNS`, `CPAttrInternalIP6Address` and
  `CPAttrInternalIP6DNS`, the `ConfigAttr` and `PayloadCP` types, `WriteTo`, `Len` and
  `ReadFrom`. `cpAttrTypeMask` masks the RFC 7296 Section 3.15.1 Reserved bit on BOTH the
  read path and the write path, with the RFC quoted above the constant.
  **Missing:** every use. No engine file names `PayloadCP`, `CFGTypeRequest`,
  `CFGTypeReply` or `CPAttrInternalIP4Address`. `ReadFrom` also accepts a trailing remnant
  shorter than four octets without error, and imposes no cap on the attribute count.
- [ ] `internal/component/ike/wire/payload.go` - `decodePayload` routes payload type 47
  (`PayloadTypeCP`) to `PayloadCP`, so an inbound CFG_REQUEST is already parsed into a typed
  payload and then reaches no handler.
- [ ] `internal/component/ike/engine/responder.go` - `(*PeerSession).handleAuthRequest`
  walks the decrypted IKE_AUTH payloads, and `(*PeerSession).buildAuthResponse` assembles
  the response payload list. Neither has a case for payload type 47.
- [ ] `internal/component/ike/engine/responder_eap.go` -
  `(*PeerSession).startResponderEAP`, `handleResponderEAP` and `sendResponderEAP` carry the
  EAP exchange. The final response, the one carrying SAr2, is produced through this path.
- [ ] `internal/component/ike/ipsec/config.go` - `parseRemoteAccess` reads the
  `remote-access` container and `parseVirtualIPPool` reads ONE pool: it takes `pools[0]`
  even though the YANG models `list pool` keyed by name. A second configured pool is
  silently dropped.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `container remote-access`
  carries `list pool` with `range`, `range6`, `dns` (a single leaf, not a leaf-list) and
  `domain`. No leaf ties a pool to a peer or to a profile, so nothing selects which pool
  serves a given client.
- [ ] `internal/component/ike/engine/child.go` - `ChildSA` and the install path. The
  negotiated `Selectors`, `TSLocal` and `TSRemote` decide what the kernel policy admits.

**Behavior to preserve:** (unless the user explicitly said to change it)
- Site-to-site peers negotiate exactly as they do today. A peer with no pool configured, and
  a peer that sends no CFG_REQUEST, sees no change in the bytes on the wire.
- The IKE_AUTH payload walk accepts payloads in any order and refuses no message over payload
  order (`RFC7296-2.5-13`).
- An unrecognized configuration attribute is ignored, never rejected.
- Every existing interop scenario under `test/interop-ipsec/scenarios/` stays green,
  `responder-eap-mschapv2`, `responder-eap-tls13` and `psk-site-to-site` in particular.
- Ze still sends no CFG_REQUEST and takes no IRAC role.

**Behavior to change:** (only what the user asked for)
- A responder that holds a pool and receives a CFG_REQUEST leases an address and answers with
  a CFG_REPLY.
- The responder's TSr for that Child SA names the leased address rather than what the client
  proposed.
- An IKE SA teardown returns the lease to the pool.
- The RFC 7296 row of `docs/features/rfc-status.md` stops claiming Ze builds no Configuration
  payload.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An IKE_AUTH request datagram on UDP 500 or 4500, from a peer Ze is answering as responder.
- Format at entry: an encrypted IKE message whose decrypted payload list carries a
  Configuration payload of type 47 with `CFGType` equal to `CFGTypeRequest`, holding at
  least an `INTERNAL_IP4_ADDRESS` attribute, usually with a zero-length value.
- The second entry point is the operator's config: `vpn ipsec remote-access pool <name>`,
  applied through the IKE component's config apply path in
  `internal/component/ike/engine/register.go`.

### Transformation Path
1. `UDPTransport` receives the datagram and the established-SA handler decrypts it
   (`internal/component/ike/engine/register.go`, `internal/component/ike/engine/responder.go`).
2. `decodePayload` (`internal/component/ike/wire/payload.go`) turns payload type 47 into a
   `*wire.PayloadCP` through `PayloadCP.ReadFrom`.
3. `(*PeerSession).handleAuthRequest` (`internal/component/ike/engine/responder.go`) picks
   the CP payload out of the walk and hands it to the new consumer.
4. The new consumer reads the requested attribute types, asks the pool for a lease, and
   records the lease on the peer session so teardown can release it.
5. The leased address is fed into the responder's traffic-selector narrowing before the
   Child SA response payloads are built.
6. `(*PeerSession).buildAuthResponse` places a CFG_REPLY payload into the response payload
   list ahead of the SA payload.
7. On the EAP path the same reply is placed by the final `sendResponderEAP`
   (`internal/component/ike/engine/responder_eap.go`) response, which is the one carrying
   SAr2.
8. `PayloadCP.WriteTo` encodes the reply into the message buffer.
9. Child SA install programs the narrowed selectors into XFRM
   (`internal/component/ike/engine/child.go`).
10. IKE SA teardown returns the lease through `Pool.Release`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ engine | `wire.PayloadCP` typed payload in and out of the message payload list | No |
| Engine ↔ pool | `eap.Pool.Allocate` and `eap.Pool.Release` calls, guarded by the pool's own mutex | No |
| Config ↔ engine | `ipsec.RemoteAccessConfig.Pool` read at apply time, pool rebuilt on change | No |
| Engine ↔ kernel | narrowed traffic selectors installed as XFRM policy and state | No |

### Integration Points
- `eap.Pool` (`internal/component/ike/eap/pool.go`) - the allocator, called for the first
  time from production code.
- `wire.PayloadCP` (`internal/component/ike/wire/payload_cp.go`) - the codec, used for the
  first time from production code.
- `(*PeerSession).buildAuthResponse` (`internal/component/ike/engine/responder.go`) - the
  payload list the reply joins.
- The responder traffic-selector narrowing (`internal/component/ike/engine/ts_narrow.go`) -
  the leased address must be the source selector Ze answers with.
- `ChildSA` (`internal/component/ike/engine/child.go`) - carries the selectors the lease
  produced.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `Pool.Allocate` is correct and needs no repair before it is called | read of `internal/component/ike/eap/pool.go`: `allocateV4` scans for a free offset, `allocateV6` rebuilds the address through `addressForHost`, `releaseV6Locked` rejects an address the pool would not have leased | the first real caller hands out a duplicate or an out-of-range address | a concurrency test under `-race` driving `Allocate` from many goroutines and asserting no duplicate | unvalidated |
| A-2 | `PayloadCP.ReadFrom` is safe against a hostile CFG_REQUEST | read of `internal/component/ike/wire/payload_cp.go`: it bounds each attribute against the buffer and returns `ErrTruncated` | an unauthenticated peer drives unbounded allocation through the attribute list | a fuzz or table test driving oversized, truncated and remnant-carrying payloads | unvalidated |
| A-3 | The reply can be inserted in `buildAuthResponse` without disturbing payload ordering rules | `docs/architecture/ike/ipsec-14-responder.md` and the RFC 7296 Section 2.19 requirement that CP precede SA | the reply lands after SA and a conforming peer rejects it | a test asserting the CP payload index is strictly less than the SA payload index in both flows | unvalidated |
| A-4 | One pool per daemon is enough for the first landing | `parseVirtualIPPool` already takes `pools[0]` only, so multi-pool is not a regression | an operator with two pools gets silent selection of one | a config test asserting a second pool is refused at commit rather than dropped | unvalidated |
| A-5 | The existing responder interop scenarios can drive a virtual-IP client without the admission work of `plan/spec-ipsec-remote-access.md` | `responder-eap-mschapv2` and `responder-eap-tls13` connect a strongSwan client to Ze as responder with a fixed remote address | the interop scenario cannot be written and G-3 has no evidence | write the scenario first and watch it fail for the right reason | unvalidated |
| A-6 | Thomas resolves OI-1 before implementation, so only one spec owns the CP work | the overlap read in `plan/spec-ipsec-remote-access.md` phases A to G and AC-7, AC-16 to AC-28 | the work is implemented twice, or each spec waits for the other | owner decision recorded in this spec and in the other one | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A lease leaks when an IKE SA dies on a path that does not run the release | `Pool.Available` falls across connect and disconnect cycles in a soak test | release from the single SA-close point rather than from each caller, and assert `Available` recovers over N cycles |
| R-2 | Allocation runs after the traffic selectors are built, so the lease never reaches TSr | the client establishes, holds an address, and cannot pass traffic | the ordering is an acceptance criterion of its own (AC-5), asserted on the payload list rather than inferred |
| R-3 | The reply is inserted on the direct path only, so EAP clients get no address | `responder-eap-mschapv2` still passes while a virtual-IP EAP client gets nothing | one insertion point serving both flows, and the interop scenario runs over EAP |
| R-4 | A hostile peer drains the pool by opening many half-authenticated sessions | `Available` falls with no established SA to account for it | allocate only after the peer authenticates, never on the unauthenticated init |
| R-5 | Reversing the no-IRAC-role claim leaves `docs/features/rfc-status.md` self-contradictory | the RFC 7296 row still says "builds no Configuration payload" while a CFG_REPLY is on the wire | the row edit is in the same work as the code, listed in the Documentation checklist |
| R-6 | Both specs implement the CP work | two branches touching `internal/component/ike/engine` for the same reason | OI-1 is answered before implementation starts |
| R-7 | Narrowing TSr to a single leased address breaks a site-to-site peer that shares the code path | an existing scenario such as `psk-site-to-site` or `child-rekey-narrowing` goes red | the narrowing change is conditional on a lease existing, and the negative test drives a peer with no pool |
| R-8 | OI-2 is answered by removing the `{gap}` rather than by narrowing the two tagged claims | `./le rfc check` goes green while `Pool.Allocate` still has no non-test caller | the gap is the honest record; the red stays until the claims are corrected or the feature lands |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Two live remote peers hold the same inner address and the gateway sends return traffic down the wrong SA, which is the exact hazard Section 5.1 names. A narrowing defect drops traffic for site-to-site peers that share the responder path. A lease leak exhausts the pool and refuses new clients. |
| How is it reverted? | Single commit revert while no operator has a pool configured, because nothing today depends on the behavior. Once a deployment leases addresses, a revert strands clients that expect one, so the revert window closes at first use. |
| Who else touches this path? | `plan/spec-ipsec-remote-access.md` (the same files, unresolved, OI-1), `plan/spec-ipsec-transport-nat-selector-substitution.md` and `plan/spec-ipsec-opaque-selector-port-mask.md` (traffic selectors), `plan/spec-ike-reauth.md` (IKE SA lifecycle and therefore lease release). |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH request carrying CP(CFG_REQUEST) | → | the CP consumer reached from `(*PeerSession).handleAuthRequest` | `TestResponderCFGRequestReachesPool` |
| the CP consumer | → | `eap.Pool.Allocate` | `TestResponderLeaseComesFromConfiguredPool` |
| operator config `remote-access pool` | → | the pool the responder actually consults | `TestConfiguredPoolIsTheOneServingClients` |
| IKE SA teardown | → | `eap.Pool.Release` | `TestIKESATeardownReturnsLease` |
| strongSwan client asking for a virtual IP | → | the whole chain | interop scenario `virtual-ip-assignment` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IKE_AUTH request carrying CP(CFG_REQUEST) with `INTERNAL_IP4_ADDRESS`, a pool configured | the response carries CP(CFG_REPLY) with an `INTERNAL_IP4_ADDRESS` attribute holding an address inside the configured range |
| AC-2 | the same request, `INTERNAL_IP4_NETMASK` also requested | the reply carries at most one netmask, and one only beside an `INTERNAL_IP4_ADDRESS` |
| AC-3 | the same request, `INTERNAL_IP4_DNS` requested and DNS configured on the pool | the reply carries the configured DNS servers, IPv4 servers under the IPv4 attribute and IPv6 servers under the IPv6 attribute |
| AC-4 | any IKE_AUTH response carrying both CP and SA | the CP payload index is strictly less than the SA payload index |
| AC-5 | a lease was granted for this Child SA | the responder's TSr names the leased address, and the installed XFRM policy admits it |
| AC-6 | an EAP session requesting a virtual IP | the CFG_REPLY appears in the response carrying SAr2, which is the final IKE_AUTH response, not the first |
| AC-7 | two clients connecting in sequence against the same pool | each receives a different address, and neither address equals the other's |
| AC-8 | two clients connected at the same time, both having proposed the same inner address | each holds a distinct leased address, and the gateway's installed policies name two distinct inner sources |
| AC-9 | the pool is exhausted | the client is refused with `INTERNAL_ADDRESS_FAILURE`, no address is handed out twice, and no empty CFG_REPLY is sent |
| AC-10 | IKE SA teardown for a client that held a lease | `Pool.Available` returns to the value it had before that client connected |
| AC-11 | N connect and disconnect cycles by one client | `Pool.Available` returns to base, with no lease left held |
| AC-12 | an IKE_AUTH request with no CP payload, or a CP whose type is Reply, Set or ACK | the response carries no CP payload, and the session establishes exactly as it does today |
| AC-13 | a CFG_REQUEST carrying unrecognized attribute types beside `INTERNAL_IP4_ADDRESS` | a correct CFG_REPLY; the unknown types appear in neither the reply nor any state, and the message is not refused |
| AC-14 | `INTERNAL_IP4_ADDRESS` carrying the RFC 7296 Section 3.15.1 Reserved bit set | recognized as attribute type 1 and answered |
| AC-15 | a client requesting only `INTERNAL_IP6_ADDRESS` against an IPv4-only pool | no address of the wrong family is sent, and the session still completes |
| AC-16 | a site-to-site peer with no pool configured | byte-identical behavior to today: no CP payload, no narrowing change, no lease |
| AC-17 | concurrent `Allocate` from many goroutines under `-race` | no duplicate address and no race report |
| AC-18 | an IPv6 pool with a prefix longer than `/64` | every leased address falls inside the configured prefix |
| AC-19 | two `pool` entries configured | commit is refused with an error naming the unsupported second pool, rather than the second being silently dropped |
| AC-20 | a CFG_REQUEST arriving before the peer has authenticated | no address is allocated, so an unauthenticated peer cannot drain the pool |

## Goal Validation (planned evidence)

<!-- Design-time table required by ai/rules/interop-and-goal-validation.md: one row per goal
     from the Task section, each Evidence cell naming a concrete artifact. /ze-close replaces
     it with the OBSERVED table from plan/TEMPLATE-CLOSURE.md. Nothing here is a result. -->

| Goal | Evidence that will prove it |
|------|------------------------------|
| G-1 Conformance: no two live peers hold the same inner address | interop scenario `virtual-ip-uniqueness` under `test/interop-ipsec/scenarios/`: two strongSwan clients connect at once, both configured to propose 10.1.2.3, and the assertion reads the two installed XFRM policies on the Ze side and requires two distinct inner sources. Plus the tagged positive and negative pair for `RFC3948-5.1-1` and its `./le rfc discriminate-record` record |
| G-2 Reachability: `Pool.Allocate` is reached from the wire | `TestResponderLeaseComesFromConfiguredPool` drives a decrypted IKE_AUTH carrying CP(CFG_REQUEST) through the responder and asserts the address in the reply is one the configured pool leased, plus the absence of `_ = ipPool` in `internal/component/ike/engine/register.go` |
| G-3 Interoperability: strongSwan receives an address from Ze's pool | interop scenario `virtual-ip-assignment` under `test/interop-ipsec/scenarios/`: a strongSwan client with a virtual-IP request establishes against Ze, the scenario asserts the client's assigned address is inside Ze's configured range, and traffic passes over the Child SA in both directions |
| G-4 Honest configuration: the YANG promise is kept | `test/ipsec/ipsec-remote-access-pool.ci` configures a pool, connects a client and shows the lease through the operational CLI, and `docs/guide/ipsec.md` no longer says the pool is unwired |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | operator configures a pool and a client dials in and receives an address | config apply → pool build → IKE_AUTH CP(CFG_REQUEST) → `Pool.Allocate` → TSr narrowing → CP(CFG_REPLY) → Child SA install | interop scenario `virtual-ip-assignment` |
| 2 | two clients that both chose 10.1.2.3 connect at once and both work | two sessions, two leases, two distinct inner sources in the installed policies | interop scenario `virtual-ip-uniqueness` |
| 3 | a client disconnects and reconnects, and the pool does not drain | IKE SA teardown → `Pool.Release` → next `Allocate` reuses the address | `TestIKESATeardownReturnsLease`, `test/ipsec/ipsec-remote-access-pool.ci` |
| 4 | operator asks which addresses are leased | `show vpn ipsec sa` reports the leased inner address per SA | `TestShowSAReportsLeasedAddress` |
| 5 | operator configures a pool too small for the clients that connect | `Pool.Allocate` refuses → `INTERNAL_ADDRESS_FAILURE` → the client sees a named failure rather than silence | `TestPoolExhaustionRefusesWithInternalAddressFailure` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResponderCFGRequestReachesPool` | `internal/component/ike/engine/cp_test.go` | AC-1, the wiring row | |
| `TestResponderLeaseComesFromConfiguredPool` | `internal/component/ike/engine/cp_test.go` | AC-1, G-2 | |
| `TestCFGReplyPrecedesSAPayload` | `internal/component/ike/engine/cp_test.go` | AC-4 | |
| `TestCFGReplyInEAPFinalResponse` | `internal/component/ike/engine/cp_eap_test.go` | AC-6 | |
| `TestNetmaskOnlyBesideIPv4Address` | `internal/component/ike/engine/cp_test.go` | AC-2 | |
| `TestDNSAttributesSplitByFamily` | `internal/component/ike/engine/cp_test.go` | AC-3 | |
| `TestTSrNamesLeasedAddress` | `internal/component/ike/engine/cp_test.go` | AC-5 | |
| `TestNoCPPayloadWhenRequestCarriesNone` | `internal/component/ike/engine/cp_test.go` | AC-12 | |
| `TestUnknownAttributesIgnoredNotRejected` | `internal/component/ike/engine/cp_test.go` | AC-13 | |
| `TestReservedBitAttributeRecognized` | `internal/component/ike/engine/cp_test.go` | AC-14 | |
| `TestIPv6OnlyRequestAgainstIPv4Pool` | `internal/component/ike/engine/cp_test.go` | AC-15 | |
| `TestPoolExhaustionRefusesWithInternalAddressFailure` | `internal/component/ike/engine/cp_test.go` | AC-9 | |
| `TestIKESATeardownReturnsLease` | `internal/component/ike/engine/cp_lease_test.go` | AC-10, AC-11 | |
| `TestNoLeaseBeforeAuthentication` | `internal/component/ike/engine/cp_test.go` | AC-20, R-4 | |
| `TestSiteToSitePeerUnaffected` | `internal/component/ike/engine/cp_test.go` | AC-16, R-7 | |
| `TestAllocateConcurrentNoDuplicate` | `internal/component/ike/eap/pool_test.go` | AC-17, A-1 | |
| `TestSecondPoolRefusedAtCommit` | `internal/component/ike/ipsec/config_test.go` | AC-19, A-4 | |
| `TestShowSAReportsLeasedAddress` | `internal/component/ike/engine/show_test.go` | story 4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IPv4 pool prefix length | /8 to /30 | /30 | /31 (names no host pair) | /7 |
| IPv6 pool prefix length | /48 to /126 | /126 | /127 | /47 |
| addresses available in a /30 pool | 0 to 2 | 2 | N/A | the third request draws `ErrPoolExhausted` |
| configuration attribute count in one CFG_REQUEST | 0 to the cap | the cap | N/A | one past the cap is refused |
| configuration attribute value length | 0 to the payload remainder | the remainder | N/A | one past it draws `ErrTruncated` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-remote-access-pool` | `test/ipsec/ipsec-remote-access-pool.ci` | operator configures a pool, a client connects, the CLI shows the leased address, the client disconnects and the address returns | |
| `ipsec-remote-access-pool-exhausted` | `test/ipsec/ipsec-remote-access-pool-exhausted.ci` | operator configures a pool with two addresses and a third client is refused with a named error | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `virtual-ip-assignment` | `test/interop-ipsec/scenarios/` | strongSwan | a strongSwan client requesting a virtual IP receives an address from Ze's configured pool and passes traffic over the Child SA in both directions (G-3) | |
| `virtual-ip-uniqueness` | `test/interop-ipsec/scenarios/` | strongSwan | two strongSwan clients that both propose the inner address 10.1.2.3 receive two distinct leased addresses, and Ze installs two policies with distinct inner sources (G-1, `RFC3948-5.1-1`) | |

Both scenarios owe the discrimination walk of `ai/rules/interop-and-goal-validation.md`:
revert the consumer, rebuild the Ze image the scenario drives, confirm RED, restore, confirm
GREEN. The tagged units carrying `RFC requirement:` run it through `./le rfc
discriminate-record`.

## Files to Modify
- `internal/component/ike/engine/register.go` - the pool stops being discarded; it is handed
  to the responder path instead of `_ = ipPool`
- `internal/component/ike/engine/responder.go` - `handleAuthRequest` picks up the CP payload,
  `buildAuthResponse` places the CFG_REPLY ahead of the SA payload
- `internal/component/ike/engine/responder_eap.go` - the final EAP response carries the same
  reply
- `internal/component/ike/engine/ts_narrow.go` - the leased address becomes the responder's
  source selector when a lease exists
- `internal/component/ike/engine/sa.go` - the SA close path releases the lease
- `internal/component/ike/wire/payload_cp.go` - the trailing-remnant error and the attribute
  count cap (A-2)
- `internal/component/ike/ipsec/config.go` - refuse a second pool rather than dropping it
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the leaf that names which pool
  serves a peer or a profile, and `dns` widened from a leaf to a leaf-list
- `docs/features/rfc-status.md` - the RFC 7296 row and the RFC 3948 row
- `rfc/short/rfc3948.md` - the `RFC3948-5.1-1` gap annotation
- `rfc/short/rfc7296.md` - the Configuration payload rows
- `rfc/extraction/rfc3948.json` - the site `5.1:1` reason
- `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - the pool section stops describing an
  intent and describes the path
- `docs/guide/ipsec.md` - the operator-facing description of the pool
- `plan/spec-ipsec-remote-access.md` - the rows OI-1 repoints, whichever way it is answered

## Files to Create
- `internal/component/ike/engine/cp.go` - the Configuration payload consumer: read the
  request, lease, build the reply, record the lease on the session
- `internal/component/ike/engine/cp_test.go` - the unit tests above
- `internal/component/ike/engine/cp_eap_test.go` - the EAP-path reply placement
- `internal/component/ike/engine/cp_lease_test.go` - the lease lifecycle
- `test/ipsec/ipsec-remote-access-pool.ci` - the operator path
- `test/ipsec/ipsec-remote-access-pool-exhausted.ci` - the exhaustion path
- `test/interop-ipsec/scenarios/virtual-ip-assignment/` - the strongSwan assignment scenario
- `test/interop-ipsec/scenarios/virtual-ip-uniqueness/` - the two-client uniqueness scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`: the leaf selecting a pool per peer or profile, and `dns` as a leaf-list |
| YANG validation constraints | Yes | `range` and `range6` take a `pattern` for CIDR form and the prefix-length bounds of the Boundary table; the pool-selecting leaf takes a `leafref` to the pool name |
| YANG custom validators | Yes | `ze:validate` on `range` and `range6` to reject a prefix that names no usable host, since a `pattern` cannot express it |
| CLI commands/flags | No | no new verb; the existing `show vpn ipsec sa` gains a field |
| CLI grammar (keyword before value) | N-A | no new command is added |
| Editor autocomplete | Yes | automatic from the `leafref` on the pool-selecting leaf |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-remote-access-pool.ci` |
| Pipe completeness | Yes | the leased address is a field of the existing `show vpn ipsec sa` payload, so it renders under `\| json`, `\| yaml` and `\| table` with no new plumbing |
| Env var registration | N-A | no leaf under `environment/` |
| Doctor check for runtime dependencies | No | no new file path, socket, listen port, kernel module, binary or certificate. The pool is in-process state, and the XFRM policy the lease produces is already covered by the existing IPsec dataplane checks |
| Prometheus counters/metrics | Yes | leases held and pool exhaustion refusals, registered beside the existing IPsec metrics in `internal/component/ike/engine/` |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, and the IPsec config section of `docs/guide/ipsec.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, for the leased-address field of `show vpn ipsec sa` |
| 4 | API/RPC added/changed? | No | no new RPC; the existing SA report gains a field |
| 5 | Plugin added/changed? | N-A | IKE is a component, not a plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | Yes | the Configuration payload joins the messages Ze emits; `docs/architecture/wire/` IKE page |
| 8 | Plugin SDK/protocol changed? | No | no SDK surface is touched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc3948.md`, `rfc/short/rfc7296.md`, `rfc/extraction/rfc3948.json`, and both rows of `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/interop.md` gain the two scenario names |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: remote-access address assignment is a feature other daemons list |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` and `docs/architecture/ike/ipsec-14-responder.md` |
| 13 | Route metadata keys added/changed? | N-A | no route metadata |
| 14 | Prometheus counters added/changed? | Yes | the subsystem telemetry page for IPsec |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/features/plugins.md` and the feature inventory row that records the pool as unwired |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED from `./le spec citation anchors spec plan/spec-ike-virtual-ip-assignment.md`, run 2026-08-31. Three pages are DECLARED by this spec's own code and therefore BLOCK: `docs/architecture/ike/ipsec-3-data-model.md` (declared by `internal/component/ike/ipsec/config.go`, and the pool-selecting leaf changes the data model it describes), `docs/architecture/ike/ipsec-7-ikev2-engine.md` (declared by `register.go` and `sa.go`, and the pool stops being discarded there), `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` (declared by `ts_narrow.go`, and the leased address becomes a source selector). `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` is declared by `eap/pool.go` and is already named in row 12. Five pages MENTION this code and are advisory: `docs/DESIGN.md`, `docs/architecture/ike/ipsec-10-cli-diag.md`, `docs/architecture/ike/ipsec-11-interop-eap.md`, `docs/architecture/ike/ipsec-13-rekey-wire.md`, `docs/config-reference.md`. Of those, `docs/config-reference.md` and `docs/architecture/ike/ipsec-10-cli-diag.md` are affected in fact (new leaf, new SA field) and are updated; the other three describe rekey, interop-EAP and the design overview, which this change does not alter |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `remote-access` examples in `docs/guide/ipsec.md` are verified against the YANG after the pool-selecting leaf lands |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the pool reachable and the CP payload visible
   - Tests: `TestResponderCFGRequestReachesPool`, `TestConfiguredPoolIsTheOneServingClients`
   - Files: `internal/component/ike/engine/register.go` (the pool stops being discarded),
     `internal/component/ike/engine/cp.go` (a stub consumer),
     `internal/component/ike/engine/responder.go` (the CP case in the payload walk)
   - Verify: the entry point exists and is reachable; the wiring test fails because the
     consumer is a stub
2. **Phase: Lease and reply** -- allocate and answer
   - Tests: `TestResponderLeaseComesFromConfiguredPool`, `TestCFGReplyPrecedesSAPayload`,
     `TestNetmaskOnlyBesideIPv4Address`, `TestDNSAttributesSplitByFamily`,
     `TestIPv6OnlyRequestAgainstIPv4Pool`
   - Files: `internal/component/ike/engine/cp.go`,
     `internal/component/ike/engine/responder.go`
   - Verify: tests fail → implement → tests pass → the wiring test progresses
3. **Phase: Ordering** -- allocation before the traffic selectors, reply in the SA-carrying
   response
   - Tests: `TestTSrNamesLeasedAddress`, `TestCFGReplyInEAPFinalResponse`
   - Files: `internal/component/ike/engine/ts_narrow.go`,
     `internal/component/ike/engine/responder_eap.go`
   - Verify: the EAP flow and the direct flow both place the reply, and TSr carries the lease
4. **Phase: Lifecycle** -- release, refusal and the drain guard
   - Tests: `TestIKESATeardownReturnsLease`, `TestNoLeaseBeforeAuthentication`,
     `TestPoolExhaustionRefusesWithInternalAddressFailure`
   - Files: `internal/component/ike/engine/sa.go`,
     `internal/component/ike/engine/cp.go`
   - Verify: `Pool.Available` recovers over cycles, and an unauthenticated peer leases nothing
5. **Phase: Config surface** -- the pool-selecting leaf, the leaf-list DNS, the second-pool
   refusal
   - Tests: `TestSecondPoolRefusedAtCommit`, the YANG boundary tests
   - Files: `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`,
     `internal/component/ike/ipsec/config.go`
   - Verify: a config naming a pool selects that pool, and a second pool is refused
6. **Phase: Codec hardening** -- the trailing remnant and the attribute cap
   - Tests: the boundary tests over `PayloadCP.ReadFrom`
   - Files: `internal/component/ike/wire/payload_cp.go`
   - Verify: a hostile payload is refused rather than partly accepted
7. **Phase: Interop and proof** -- the two scenarios and the discrimination walk
   - Tests: `virtual-ip-assignment`, `virtual-ip-uniqueness`, the tagged pairs for
     `RFC3948-5.1-1` and the RFC 7296 Configuration payload rows
   - Files: `test/interop-ipsec/scenarios/virtual-ip-assignment/`,
     `test/interop-ipsec/scenarios/virtual-ip-uniqueness/`
   - Verify: each scenario is reverted to RED and restored to GREEN, and
     `./le rfc discriminate-record` writes the record
8. **Phase: Documentation and ledger** -- the reversal is disclosed everywhere it was claimed
   - Tests: `./le rfc index-update`, `./le rfc check`
   - Files: every row of the Documentation checklist
   - Verify: no page still says Ze builds no Configuration payload

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation, and AC-8 has an assertion over TWO live sessions rather than two sequential ones |
| Feature completeness | The reply reaches the wire on BOTH the direct and the EAP path, not one of them |
| Correctness | Allocation precedes traffic-selector construction; the reply precedes the SA payload; the lease is released exactly once |
| Fail-closed | An exhausted pool refuses with `INTERNAL_ADDRESS_FAILURE`; it never sends an empty CFG_REPLY and never reuses a live address |
| Naming | The interop scenario directories are NAMED and carry no numeric prefix |
| Data flow | The pool is consulted in one place; no second allocator, no per-session copy of the pool |
| Rule: `ai/rules/rfc-compliance.md` | Every MUST enforced in the new code carries the quoted RFC section above it, and no claim is wider than what its test body checks |
| Rule: `ai/rules/interop-and-goal-validation.md` | Both scenarios have an observed RED, recorded, not reasoned |
| Rule: `ai/rules/principles.md` | The reversed decision is corrected at every surface that declared it, so no page disagrees with the code |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `_ = ipPool` is gone | `grep -n "_ = ipPool" internal/component/ike/engine/register.go` returns nothing |
| `Pool.Allocate` has a non-test caller | `grep -rn "Allocate()" --include=*.go internal/component/ike \| grep -v _test.go` |
| The CFG_REPLY reaches the wire | `./le integration interop-ipsec` with scenario `virtual-ip-assignment` |
| Two peers never share an inner address | `./le integration interop-ipsec` with scenario `virtual-ip-uniqueness` |
| The gap is closed in the ledger | `./le rfc index-update` then `./le rfc check` shows no `RFC3948-5.1-1` gap |
| The reversal is disclosed | `grep -n "builds no Configuration payload" docs/features/rfc-status.md` returns nothing |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A CFG_REQUEST is attacker-controlled: the attribute count, each attribute length and the total payload length are all bounded before any allocation |
| Resource exhaustion | The pool is a finite resource an attacker can drain. No lease is granted before the peer authenticates, and a lease is released on every SA-close path |
| Authorization fails closed | A peer with no pool assigned to it receives no address rather than an address from a default pool |
| Information leakage | The refusal names `INTERNAL_ADDRESS_FAILURE` and nothing about pool size, remaining capacity or other clients' addresses |
| Cross-client isolation | A leased address is bound to one session; a second session never inherits a lease, and release never frees another session's lease |

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

- The gap is a missing CONSUMER, not a missing capability. The allocator and the codec are
  both written, both tested and both unreached, which is why every artifact that read the
  code from one end concluded the feature existed.
- `binds-another-role` was the wrong exclusion kind because the role question is about the
  packet, not the code. Ze terminates tunnel-mode Child SAs, so it is the security gateway of
  the Section 5.1 diagram whether or not it assigns addresses.
- The two orderings are the real design content: allocate before the selectors are built, and
  reply in the message that carries the SA payload. Everything else is plumbing.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Assign a locally unique address per peer | NAT the inner source addresses at the gateway, which Section 5.1 also permits | The RFC states the address assignment first and calls it RECOMMENDED, the pool and the codec for it already exist, and NAT of decrypted inner traffic would add a translation layer Ze does not have |
| Become an IRAS while still declining the IRAC role | Implement both halves of the Configuration payload exchange | The obligation is on the security gateway. Ze as initiator asks nobody for an address, so a CFG_REQUEST sender would be code with no user (`ai/rules/completion.md`) |
| One insertion point serving both the direct and the EAP response | Insert the reply separately on each path | Two insertion points is two chances for the EAP path to be forgotten, which is exactly R-3 |
| Refuse a second configured pool rather than silently dropping it | Support multiple pools now | `parseVirtualIPPool` already drops the second pool today, so refusing is the smaller change and removes a silent wrong answer (`ai/rules/principles.md`). Multi-pool is a separate feature |

## Known Limitations

- Road-warrior ADMISSION is not in this spec. Until
  `plan/spec-ipsec-remote-access.md` lands, a client must connect as a configured responder
  peer for the assignment to be reachable. The feature is complete for that shape and
  incomplete for a client whose address is not known in advance.
- One pool per daemon. A second configured pool is refused rather than served.
- No lease persistence across a daemon restart. Every lease is lost and re-leased, which is
  correct for a stateless gateway and would be wrong for a deployment expecting a stable
  address per user.
- No lease lifetime independent of the IKE SA. A lease lives exactly as long as its SA.
  A per-lease expiry timer is a separate feature and is named in
  `plan/spec-ipsec-remote-access.md` AC-26.

## RFC Documentation (Scope: protocol)

Add above enforcing code:
- `// RFC 3948 Section 5.1: "Implementors MUST devise ways of preventing this from occurring."`
- `// RFC 3948 Section 5.1: "It is RECOMMENDED that SGW either assign locally unique IP addresses to Ari's and Bob's laptop"`
- `// RFC 7296 Section 2.19: the Configuration payload is sent in the message that carries the SA payload, and precedes it`
- `// RFC 7296 Section 3.15.1: the Configuration Attribute Type is 15 bits; the Reserved bit MUST be set to zero and MUST be ignored on receipt`
- `// RFC 7296 Section 3.15.1: an unrecognized configuration attribute is ignored, never rejected`
- `// RFC 7296 Section 3.15.4: a request for an address family the responder cannot serve does not fail the exchange`

Requirement ids this work closes: `RFC3948-5.1-1`, and the RFC 7296 Configuration payload
rows homed in `rfc/short/rfc7296.md` for Sections 2.19, 2.20, 3.15.1 and 3.15.4. The RFC 7296
rows are homed in `plan/spec-ipsec-remote-access.md` today; OI-1 decides which spec closes
them.

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
- [ ] OI-1 answered by Thomas: this spec or `plan/spec-ipsec-remote-access.md` owns the Configuration payload work
- [ ] OI-2 answered by Thomas: the two `RFC3948-5.1-1` tags in `internal/component/ike/eap/pool_test.go` are narrowed to what their bodies check, with the approval row in `test/rfc-changed.md`

### Goal Gates (MUST pass)
- [ ] AC-1..AC-20 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
