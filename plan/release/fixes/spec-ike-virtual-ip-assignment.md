# Spec: ike-virtual-ip-assignment

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P0 |
| Release reason | Prevent conflicting peer inner addresses (RFC 3948 Section 5.1 MUST) and make the accepted remote-access pool effective. |
| RFC gaps | RFC3948-5.1-1 |
| Depends | - |
| Phase | 1/9 |
| Approved | 2026-10-26, supervising session f3ae7c00 under the owner's standing rule for away goals (`ai/META-DECISIONS.md`, "How agents work"): every design question is answered by the RFC 7296 / RFC 3948 text and strongSwan source quoted here. Q-1 settled as written: the pool `domain` leaf is refused at commit (RFC 8598 enrolment is frozen) |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Revision 2026-10-26 (design pass)

The 2026-08-31 skeleton was re-verified against the working tree and completed. What
changed, so nobody reads the old text as current:

| Item | 2026-08-31 skeleton said | Now |
|------|--------------------------|-----|
| OI-1, who owns the Configuration-payload work | open owner question | **Answered: this spec is the single owner.** Basis and the full AC mapping are in "Ownership" below |
| OI-2, the two false `RFC3948-5.1-1` tags in `internal/core/eap/pool_test.go` | open, needs owner approval to narrow | **Resolved by observation.** `TestVirtualIPPoolNeverHandsOneAddressTwice` carries no tag today and its comment calls the requirement a gap (owner ruling 2026-08-31). No tagged unit names `RFC3948-5.1-1`, and `rfc/short/rfc3948.md` holds the `{gap}` honestly |
| Where the pool is discarded | `_ = ipPool` in `register.go` | `(*ikeEngineState).reloadPool` (`internal/component/ike/engine/apply.go`) stores the pool in `ikeEngineState.pool`, which no code reads |
| Which selector carries the lease | "the responder's TSr names the leased address" | **Wrong, corrected.** The lease is the INITIATOR's inner address, so it replaces the remote side: TSi on the wire, `TSRemote` on Ze's Child SA. RFC 7296 Section 2.19's own example answers TSi = 192.0.2.202-192.0.2.202 |
| Pools | one pool; refuse a second at commit | every configured pool is parsed and served; each peer names the pool it draws from; overlapping pools are refused |
| Pool rebuild on edit | not addressed | a rebuild never drops a live lease (D-2); the old code's rebuild would have handed a live address out again the moment a consumer existed |

## Task

RFC 3948 Section 5.1 states an obligation Ze does not meet. Two remote peers behind
separate NATs, each having chosen the inner address 10.1.2.3 for itself, both reach one
security gateway. Read at Section 5.1 in `rfc/full/rfc3948.txt`:

> Because SGW will now see two possible SAs that lead to 10.1.2.3, it can become confused
> about where to send packets coming from Suzy's server. Implementors MUST devise ways of
> preventing this from occurring.

The same section names the remedy it prefers:

> It is RECOMMENDED that SGW either assign locally unique IP addresses to Ari's and Bob's
> laptop (by using a protocol such as DHCP over IPsec) or use NAT to change Ari's and Bob's
> laptop source IP addresses to these locally unique addresses before sending packets
> forward to Suzy's server.

Ze is that security gateway: it terminates tunnel-mode Child SAs. It carries a
`remote-access pool` configuration surface built to hand out exactly the locally unique
addresses the section recommends, and it assigns no address at all.

**The goals of this work:**

1. **G-1 Conformance.** Close `RFC3948-5.1-1`: a peer bound to a pool holds an inner address
   no other live peer holds, and every Child SA of that peer names that address as its
   inner source, so the gateway never holds two SAs leading to one inner address.
2. **G-2 Reachability.** `eap.Pool` allocation is reached from the wire through the IKE_AUTH
   responder, on both the direct (PSK, X.509) and the EAP path.
3. **G-3 Interoperability.** A strongSwan client configured for a virtual IP receives one
   from Ze's pool and passes traffic over the resulting Child SA in both directions.
4. **G-4 Honest configuration.** An operator who configures a pool gets the behavior the YANG
   describes, and every leaf of the pool either acts or is refused at commit.

**This reverses a recorded design claim, on Thomas's ruling of 2026-08-31.** The RFC 7296
row of `docs/features/rfc-status.md` says Ze builds no Configuration payload and takes no
IRAC role. After this work Ze is an IRAS (it answers CFG_REQUEST as responder) and still
takes no IRAC role (it never sends CFG_REQUEST). Every surface that carries the old claim is
rewritten in the same work (Documentation checklist).

### Ownership (OI-1, answered 2026-10-26)

`plan/release/fixes/spec-ipsec-remote-access.md` held the Configuration-payload work as its
phases A to G and paused it until this question was answered. **This spec is now the single
owner of the IRAS role, the virtual-IP lease lifecycle, the codec hardening, and every RFC
7296 Configuration-payload row.** `spec-ipsec-remote-access.md` keeps admission of
unconfigured sources, per-user EAP credentials, the `remote-access authentication` checks and
the road-warrior scenario, and depends on this spec.

Basis, in the order it decides:

| Source | What it settles |
|--------|-----------------|
| `ai/META-DECISIONS.md`, "Split a spec that mixes fixes with new features when release planning needs them apart, keeping every fix, feature, acceptance criterion and dependency in its owning spec" | The RFC 3948 Section 5.1 MUST is a conformance fix in a capability Ze ships (tunnel-mode Child SAs). Road-warrior admission is a feature. They are separated, with nothing dropped |
| `spec-ipsec-remote-access.md`, "Configuration-payload ownership pause" | Names "the sibling becomes the single owner and the overlapping phases here become prerequisites" as a valid outcome, provided the complete union is mapped. The mapping table below is that union |
| `ai/rules/principles.md`, "Every fact MUST be declared once" | Two live owners for one behavior is the defect; one owner removes it |
| Owner ruling 2026-08-31 (recorded in this spec's Task) | Ze becomes IRAS and declines IRAC. This answers `spec-ipsec-remote-access.md` OI-1 as well |

**Mapping of every shared criterion of `spec-ipsec-remote-access.md`:**

| remote-access AC | Here | Note |
|------------------|------|------|
| AC-7 (CFG_REPLY with pool address, netmask, DNS) | AC-1, AC-2, AC-3 | domain: see Q-1 |
| AC-8 (selector narrowed to the assigned address) | AC-5 | corrected to the remote (TSi) side |
| AC-9 (exhaustion refused, no duplicate, no empty CFG_REPLY) | AC-9 | |
| AC-10 (teardown returns the address) | AC-10 | the "session is reaped" half stays there with AC-11 |
| AC-15 (concurrent allocate under `-race`) | AC-17 | |
| AC-16 (`2.19-2`, CP before SA) | AC-4 | |
| AC-17 (`2.19-3`, CP in SA-bearing message, both flows) | AC-6 | |
| AC-18 (`2.19-5`, no CFG_REPLY without CFG_REQUEST) | AC-12 | |
| AC-19 (`3.15.1-1`, one netmask beside an IPv4 address) | AC-2 | |
| AC-20 (`2.19-6`, FAILED_CP_REQUIRED) | AC-21 | |
| AC-21 (`3.15.1-4`, ignore unknown, R bit) | AC-13, AC-14 | |
| AC-22 (`4-2`, parse in first IKE_AUTH, both flows) | AC-22 | |
| AC-23 (`4-3`, family fidelity) | AC-15 | |
| AC-24 (`1.7-1`, ignore type 5) | AC-23 | lands as a renumbered id, see "RFC Documentation" |
| AC-25 (`3.15.1-3`, SUPPORTED_ATTRIBUTES, conditional on its OI-3) | AC-24 | its OI-3 answered by its own text: "Answering is strictly more compliance and needs no permission". Answered |
| AC-26 (lease expiry) | AC-10, AC-25 | the lease is returned at IKE SA end, so `Available` recovers at teardown with no timer; the identity's last address is preferred on reconnect. See Key Design Decisions |
| AC-27 (per-identity quota) | AC-26 | structural: one identity holds at most one address per family per pool, however many sessions it opens. See Key Design Decisions |
| AC-28 (IPv6 prefix longer than /64) | AC-18 | |
| Security list: attribute-count cap, `NewPool` self-bounding, rotating IPv4 cursor, doctor check | AC-19, AC-20, AC-27, AC-28 | |
| `3.15.4` INTERNAL_ADDRESS_FAILURE (its OI-4, answered there) | AC-9, AC-15 | |
| `2.20-1` APPLICATION_VERSION | AC-29 | becomes a live obligation once Ze supports CP |

Stays in `spec-ipsec-remote-access.md`: AC-1 to AC-6, AC-11, AC-12, AC-13, AC-14. Its OI-2
(ignore CFG_SET) stays recorded as answered by RFC 7296 Section 3.15.1, "An implementation of
this specification MAY ignore CFG_SET payloads." This spec keeps that behavior (AC-12).

### What this spec does NOT cover

Admission of an IKE_SA_INIT from an unconfigured source, per-user EAP credentials and the
`remote-access authentication` surface stay in `spec-ipsec-remote-access.md`. Without
admission the assignment here is reachable by a client that connects as a configured
responder peer, which is how `responder-eap-mschapv2` and `responder-psk` drive Ze today. That
proves and ships the assignment behavior; it does not make road-warrior access usable.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - the page `eap/pool.go` declares
  → Decision: its pool section ("allocates sequentially from the CIDR base") is the page this
  change makes wrong first: allocation becomes owner-aware with a rotating cursor
  → Constraint: repaired in the same phase as the code (`ai/rules/documentation.md`)
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - the responder exchange
  → Constraint: both responder flows reach `selectAuthChildSA` (`responder.go`): the direct
  flow from `buildAuthResponse` after `verifyRemoteAuth`, the EAP flow from the final
  `handleResponderEAP` after the EAP AUTH check. It is the one insertion point for CP and
  for allocation, and it runs before `createFirstChildSA`
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - Child SA install
  → Constraint: `narrowChildSelectors` records the one selector result both the echoed TS
  payloads and the installed Child SA read, so the lease must enter there or the wire and the
  kernel disagree
- [ ] `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` - selector narrowing, NAT substitution
  → Constraint: `substituteResponderSelectors` runs before the policy match inside
  `narrowChildSelectors`; the lease substitution sits beside it, before the match
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - engine state and reload
  → Constraint: `ikeEngineState` has one writer (`applyConfig`); peer sessions run on their
  own goroutines, so anything they read from engine state crosses goroutines and needs its
  own lock or an atomic pointer
- [ ] `docs/architecture/testing/interop.md` - the IPsec interop suite and NAT box
  → Constraint: scenarios live in `test/interop-ipsec/scenarios/<name>/`, named with no
  numeric prefix; a `nat.conf` starts the NAT box at 172.28.0.5, one `<real> <public>`
  translation per line; `test/interop-ipsec/` is refused as an RFC tag carrier
- [ ] `docs/architecture/wire/buffer-writer.md`
  → Constraint: `PayloadCP.WriteTo(buf, off) int` writes into the message buffer; the reply
  builder allocates no intermediate encoding buffer
- [ ] `ai/doors/access-security.md` - the IKE door

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3948.md` - `RFC3948-5.1-1` `{gap}` (re-read 2026-10-26)
  → Constraint: the gap text names `reloadPool` and a stale path `plan/immediate/...`; both
  are rewritten when the gap closes
- [ ] `rfc/short/rfc7296.md` - Sections 1.7, 2.19, 2.20, 3.15.1, 4
  → Constraint: `RFC7296-2.19-1`, `-2.19-4`, `-2.20-1`, `-3.15.1-2`, `-5`, `-6`, `-7`,
  `-4-1`, `-4-4`, `-4-5` exist today, so the high-water marks are 2.19:4, 3.15.1:7, 4:5,
  1.7:2. New rows below a mark are refused by `check_id_allocation` and take a higher
  ordinal. Recompute at landing, never copy these numbers
- [ ] `rfc/short/rfc4301.md` - SPD/SAD
  → Constraint: the lease reaches the kernel only through the negotiated selectors

### Peer implementation read (strongSwan, master `b8e275ca62`, 2026-10-23)
- [ ] `src/libcharon/sa/ikev2/tasks/ike_config.c`, `process_r` and `build_r`
  → Decision: CP is parsed in the first IKE_AUTH only ("process attributes in first IKE_AUTH
  only") and answered once the SA is authenticated ("in last IKE_AUTH exchange"), so the EAP
  flow parses early and replies late
  → Decision: pools belong to the peer's connection (`config->create_pool_enumerator`); a
  connection with pools and no address request is refused with FAILED_CP_REQUIRED ("expected
  a virtual IP request, sending %N"); requested addresses with none found draw
  INTERNAL_ADDRESS_FAILURE and no CFG_REPLY ("no virtual IP found, sending %N")
  → Decision: configured attributes (DNS) are added whether or not they were requested
  (`create_responder_enumerator`)
- [ ] `src/libcharon/attributes/mem_pool.c`, `acquire_address`, `get_existing`, `get_new`,
  `release_address`
  → Decision: leases are bound to the peer identity; an identity with an online lease from the
  same peer host gets the SAME address again during make-before-break reauthentication
  ("reassigning online lease"), and a released lease goes offline and is reassigned to that
  identity on reconnect ("reassigning offline lease"); a requested address of the wrong
  family returns nothing
- [ ] `src/libcharon/sa/ike_sa.c`, `ike_sa_get_dynamic_hosts`
  → Decision: the remote side's dynamic selector is the assigned virtual IP; with a pool
  configured and no virtual IP assigned, the remote host is NOT used ("use remote host only
  if we don't have a pool configured"), so a pool-bound peer without a lease gets no Child SA
- [ ] `src/libcharon/sa/ikev2/tasks/ike_config.c`, `build_vip`
  → Decision: an IPv6 address is sent as 16 octets plus a prefix-length octet; strongSwan
  sends 64 and sends no INTERNAL_IP4_NETMASK unrequested

**Key insights:**
- The allocator and the codec exist; the gap is a consumer, a lease registry that survives
  pool edits, and the per-peer binding nothing declares today.
- One insertion point serves both flows: `selectAuthChildSA`, after authentication, before
  `narrowChildSelectors` and before `createFirstChildSA`.
- The lease replaces the REMOTE selector (TSi on the wire), for every Child SA of that IKE SA.
- A lease lives with its IKE SA and its rekeyed successors, and is released at the single SA
  close point, `(*SA).forgetKeys`.

## Current Behavior (MANDATORY)

**Source files read (2026-10-26):**
- [ ] `internal/core/eap/pool.go` (247L) - `NewPool(ipv4CIDR, ipv6CIDR, dns, domain)` builds
  an IPv4 range (network and broadcast excluded, /31 and /32 refused) and an optional IPv6
  range bounded by `host6`/`max6`. `(*Pool).Allocate()` takes no argument and ALWAYS leases
  one address of EVERY configured family, rolling the IPv4 lease back if IPv6 fails; it
  copies DNS split by family and the domain into `AllocateResult`. `allocateV4` scans from
  offset 1 every time (no cursor); `allocateV6` uses the rotating `next6`. `Release(ip)`
  frees by family; `releaseV6Locked` rejects an address the pool would not have produced.
  `Available()` counts IPv4 only. No identity, no requested-address input, no lease record.
  → Constraint: callers are `pool_test.go` and `pool_release_test.go` only
- [ ] `internal/component/ike/engine/apply.go` (522L) - `ikeEngineState` carries `pool
  *eap.Pool` and `poolCfg`. `reloadPool(ra)` rebuilds the pool from `ra.Pool` whenever the
  whole `ipsec.VirtualIPPool` differs (`reflect.DeepEqual`), drops it on error with a
  warning, and logs `ike: virtual IP pool created`. Its comment says "NOTHING READS s.pool
  YET". A rebuild replaces the object, so any lease held in the old pool would be forgotten
  → Constraint: once a consumer exists, a rebuild of this shape hands a live address to a
  second peer; D-2 removes that
- [ ] `internal/component/ike/ipsec/config.go` - `parseRemoteAccess` reads `pools[0]` only
  (`t.GetListOrdered("pool")`), so a second configured pool is silently dropped.
  `parseVirtualIPPool` reads `range`, `range6`, `dns`, `domain`
- [ ] `internal/component/ike/ipsec/types.go` - `VirtualIPPool{Name, Range, Range6, DNS
  []string, Domain}`; `RemoteAccessConfig.Pool` is a single value
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `container remote-access` with
  `list pool` (name, range, range6, `dns` a single leaf, domain). Its descriptions say the
  subtree is read by no session and point at the stale path `plan/immediate/...`. `list
  peer` has `connection-type` (`initiate` default, `respond`) and
  `traffic-selector <number> { local prefix, remote prefix, ... }`. No leaf binds a peer to a
  pool
- [ ] `internal/component/ike/wire/payload_cp.go` (96L) - `PayloadCP{CFGType, Attrs}`,
  `ConfigAttr{Type, Value}`, `WriteTo`, `Len`, `ReadFrom`; `cpAttrTypeMask` (0x7fff) clears
  the Reserved bit on read and write. Constants for types 1, 2, 3, 4, 6, 7, 8, 10, 12. Missing:
  5, 13, 14, 15. `ReadFrom` silently drops a trailing remnant shorter than 4 octets and
  places no cap on the attribute count
- [ ] `internal/component/ike/wire/payload.go` - payload 47 decodes to `*wire.PayloadCP`
- [ ] `internal/component/ike/wire/payload_notify.go` - `NotifyInternalAddressFailure` (36)
  and `NotifyFailedCPRequired` (37) are declared and have no engine referent
- [ ] `internal/component/ike/engine/responder.go` (1216L) -
  `(*PeerSession).handleAuthRequest` decrypts, collects payloads in one order-free walk (ID,
  AUTH, CERT, Notify, SA, TS; no CP case), calls `decideResponderTransportMode`, routes to
  `startResponderEAP` when AUTH is absent and the mode is EAP, else verifies AUTH, adopts
  the endpoint, and calls `buildAuthResponse`. `buildAuthResponse` concatenates
  `buildAuthIdentityPayloads` (IDr, CERT, AUTH) with `selectAuthChildSA` (SAr2,
  USE_TRANSPORT_MODE, TSi, TSr), encrypts, then installs via `createFirstChildSA`; a
  `selectAuthChildSA` error yields `buildChildlessAuthResponse` (auth half plus one error
  notify from `notifyForRefusal`, IKE SA kept). `finishResponderEstablish` adopts the child
  (or the pending child for a parallel re-initiation, then `signalSupersede`)
- [ ] `internal/component/ike/engine/responder_eap.go` (417L) - `startResponderEAP` stores
  the ESP SPI, narrows selectors on the FIRST IKE_AUTH (pre-authentication) and keeps a
  refusal in `sa.eapChildRefusal`; it keeps the narrowing RESULT, not the received TSi/TSr.
  The final `handleResponderEAP` reaches `buildAuthResponse(..., nil, nil, nil, true, ...)`,
  where `selectAuthChildSA` skips narrowing because tsi and tsr are nil
- [ ] `internal/component/ike/engine/ts_narrow.go` - `narrowChildSelectors(sa, tsi, tsr,
  floor)` is the single responder narrowing entry: `substituteResponderSelectors` first, then
  the policy match, recording the result on the SA
- [ ] `internal/component/ike/engine/sa.go` - `(*SA).forgetKeys` is "Called once per SA, on
  the path that closes it" and already calls `closeEAPSession`; it is invoked from `fsm.go`,
  `established.go` (owner-loop exit, IKE rekey retirement at the `out.newSA` swap, pending
  cleanup) and `reconcile.go` (`pendingIKESwap`)
- [ ] `internal/component/ike/engine/established.go` - the IKE rekey swap stores the new SA,
  re-keys the table and calls `oldSA.forgetKeys()`
- [ ] `internal/component/ike/engine/inbound.go` - `handleNewChildRequest` (CREATE_CHILD_SA)
  and `handleInformationalOwned` (INFORMATIONAL) carry no CP case
- [ ] `internal/component/ike/engine/transport_mode.go` - `decideResponderTransportMode` sets
  `UseTransportMode` from the peer's request and `wantsTransportMode`

**Behavior to preserve:**
- A peer with no pool bound, and any peer that sends no CP, sees byte-identical IKE_AUTH
  responses and selectors (`responder-psk`, `psk-site-to-site`, `responder-eap-*`,
  `child-rekey-narrowing` stay green).
- The IKE_AUTH walk refuses no message over payload order (`RFC7296-2.5-13`); no
  receive-side order check is added.
- CFG_SET is ignored (RFC 7296 Section 3.15.1 MAY). Ze sends no CFG_REQUEST.
- EAP pre-authentication narrowing and its early refusal stay as they are.

**Behavior to change:** listed as the design below.

## Design

### D-1 Configuration surface

| Element | Behavior |
|---------|----------|
| `vpn ipsec remote-access pool <name>` | Every entry is parsed and served (the `pools[0]` drop is removed). `RemoteAccessConfig.Pools` becomes a name-keyed set |
| `pool <name> dns` | Widened from leaf to leaf-list; IPv4 and IPv6 servers mixed, split by family on the wire |
| `pool <name> domain` | Q-1. Proceeding reading: commit refuses a set `domain` with an error naming why (RFC 7296 defines no search-domain attribute) and the fix (remove the leaf) |
| New leaf `vpn ipsec site-to-site peer <name> virtual-ip-pool <pool>` | Binds the peer to one pool. Same reference pattern as `ike-group`: a string the commit check resolves, refusing a name no pool carries |
| Binding implies CP required | strongSwan `build_r`: a connection with pools and no request draws FAILED_CP_REQUIRED. No separate "required" leaf |
| Commit refusals (each names the peer or pool, why, and the fix) | two pools whose IPv4 or IPv6 ranges overlap; a peer naming an undeclared pool; `virtual-ip-pool` on a peer whose `connection-type` is not `respond` (Ze answers CFG_REQUEST only as IKE_AUTH responder and never requests one); a peer whose configured `traffic-selector` remote prefixes do not cover its pool's ranges (every lease would draw TS_UNACCEPTABLE) |
| Transport mode | A pool-bound peer declines USE_TRANSPORT_MODE (RFC 7296 Section 1.3.1 lets the responder decline; a virtual inner address is a tunnel-mode concept). `decideResponderTransportMode` reads the binding |

### D-2 Lease registry

One registry per engine owns every live lease across every pool. It is the only uniqueness
authority (RFC 3948 Section 5.1).

| Property | Rule |
|----------|------|
| Home | `internal/core/eap/pool.go`, beside `Pool`; the registry is created once with `ikeEngineState` and never replaced |
| Lease record | address, family, pool name, owner key, holder count |
| Owner key | the configured peer name today (Ze runs one session per configured peer). The API takes an opaque string so `spec-ipsec-remote-access.md` passes the EAP identity later |
| Pools | range and attribute descriptors that allocate THROUGH the registry. `reloadPool` rebuilds descriptors per pool on edit; the registry and its live leases are untouched, so an edit can never re-issue a live address. A live lease that falls outside every range after an edit stays held until its IKE SA ends |
| Concurrency | registry guarded by its own mutex; peer sessions read the current pool descriptors through an atomic pointer that `applyConfig` swaps |
| Allocation, per requested family | (1) the owner already holds a live lease of that family in that pool: return it, holder count +1 (strongSwan `get_existing`, online reassignment during reauthentication and Ze's parallel re-initiation); (2) a non-zero requested address inside the range, a usable host address, not held: lease it (RFC 7296 Section 3.15.1 and 3.15.3); for IPv6 outside the prefix, try its interface identifier inside the prefix (Section 3.15.3); (3) the owner's last released address of that family, if free (strongSwan offline lease); (4) the next free address from a rotating cursor, IPv4 and IPv6 alike; (5) none: exhausted |
| Families | only the families requested AND configured on the pool are leased (fixes `Allocate` leasing every family) |
| Release | holder count -1; at zero the address is free and remembered as the owner's last address of that family |
| Visibility | `Available` reports both families; per-pool held count feeds metrics, `show` and the doctor check |

### D-3 Wire consumer (`internal/component/ike/engine/cp.go`)

| Step | Where | Rule |
|------|-------|------|
| Collect | `handleAuthRequest` walk gains a `*wire.PayloadCP` case that only collects (count and keep) | no order check |
| Decide | one function returning the request and an explicit ok, denying unless: exactly one CP; CFG type `CFGTypeRequest`; at least one INTERNAL_IP4_ADDRESS or INTERNAL_IP6_ADDRESS attribute; the peer is pool-bound and the pool resolves (an unresolvable binding is a bug state and DENIES) | `spec-ipsec-remote-access.md` "Security requirements" |
| EAP keep | `startResponderEAP` stores the decided request and the received TSi/TSr on the SA. A request that arrives only in the final IKE_AUTH is also accepted | RFC 7296 Section 4 parse in the first message; AC-22 |
| Allocate | inside `selectAuthChildSA`, reached only after authentication, BEFORE `narrowChildSelectors` and `createFirstChildSA` | strongSwan `build_r` checks `COND_AUTHENTICATED`; an unauthenticated peer drains nothing |
| Refuse, no request | pool-bound peer, decision denied: error mapped by `notifyForRefusal` to FAILED_CP_REQUIRED; `buildChildlessAuthResponse` sends the auth half plus the notify, IKE SA kept, no Child SA installed | RFC 7296 Section 2.19 |
| Refuse, none assignable | every requested family failed (exhausted or not configured): INTERNAL_ADDRESS_FAILURE through the same childless path; no CFG_REPLY | RFC 7296 Section 3.15.4; strongSwan `build_r` |
| Substitute | `narrowChildSelectors` gains the IKE SA's lease: the proposed remote selectors are replaced by the leased host addresses (keeping the proposal's protocol and ports) before the policy match, beside `substituteResponderSelectors`. On the EAP final, narrowing is re-run from the stored TSi/TSr with the lease | `ike_sa_get_dynamic_hosts` |
| Every later child | `handleNewChildRequest` and both Child rekey paths apply the same substitution while the IKE SA holds a lease; CP in CREATE_CHILD_SA is ignored, as strongSwan's `process_r` does | no Child SA of a leased peer names a non-leased inner source |
| Reply | CFG_REPLY placed FIRST in the Child SA half, so it precedes SAr2 in both flows: INTERNAL_IP4_ADDRESS (4 octets); INTERNAL_IP4_NETMASK (pool prefix mask) only if requested and only beside an IPv4 address; INTERNAL_IP6_ADDRESS (16 octets plus the pool prefix length); configured DNS of each assigned family whether or not requested; APPLICATION_VERSION with an empty value when requested; SUPPORTED_ATTRIBUTES (the handled types, 2 octets each, derived from the same handler table) when a zero-length one is requested | RFC 7296 Sections 2.19, 2.20, 3.15.1, 3.15.3 |
| Ignore | types 5 (RFC 7296 Section 1.7, the attribute, not the proposal, per erratum 5056) and every unrecognized type; the Reserved bit already masked | Section 3.15.1 |
| INFORMATIONAL | a CFG_REQUEST in `handleInformationalOwned` gets a CFG_REPLY only when it asks APPLICATION_VERSION (empty value); address requests there are not served | RFC 7296 Section 2.20 decline; strongSwan serves addresses in IKE_AUTH only |

### D-4 Lease lifetime

| Event | Behavior | Site |
|-------|----------|------|
| Grant | the lease is held by the IKE SA | `selectAuthChildSA` |
| IKE SA rekey | the successor takes the lease before `oldSA.forgetKeys()`; holder count unchanged | `established.go` `out.newSA` swap, and the responder-side rekey adoption |
| Parallel re-initiation (same peer) | the new SA re-acquires through step (1), count 2; the superseded SA releases at its close, count 1 | `finishResponderEstablish` |
| IKE SA close (delete, DPD, peer stop, reload restart, shutdown) | released exactly once | `(*SA).forgetKeys`, beside `closeEAPSession` |
| Childless outcome after a grant (TS_UNACCEPTABLE, dataplane refusal) | lease kept with the IKE SA; a later CREATE_CHILD_SA uses it | RFC 7296 Section 2.19, "That address persists at least until the deletion of the IKE SA." |

### D-5 Codec, visibility, operations

- `PayloadCP.ReadFrom`: a trailing remnant shorter than 4 octets is `ErrTruncated`; more than
  64 attributes is refused as malformed (the payload is attacker-shaped; the largest real
  request seen from strongSwan, Windows and macOS clients is under 20); constants 5, 13, 14,
  15 declared.
- `show vpn ipsec sa` gains the leased address(es) per IKE SA (`virtual-ip`, kebab-case).
- Metrics: leases held per pool (gauge), INTERNAL_ADDRESS_FAILURE and FAILED_CP_REQUIRED sent
  (counters, beside the existing IPsec metrics).
- Doctor check `doctor-ipsec-virtual-ip-pool`: warns when a pool is at or above 90% held,
  names the pool, and gives the fix (widen `range` or `range6`).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- IKE_AUTH request on UDP 500/4500 from a configured `respond` peer bound to a pool; the
  decrypted chain carries CP(CFG_REQUEST) (Section 2.19 figure).
- Operator config `vpn ipsec remote-access pool` and `site-to-site peer <name>
  virtual-ip-pool`, applied through `applyConfig` (`apply.go`).

### Transformation Path
1. `handleAuthRequest` decrypts and collects the CP payload (`responder.go`).
2. Direct flow: `verifyRemoteAuth`, then `buildAuthResponse`. EAP flow: `startResponderEAP`
   stores the request and TSi/TSr; the final `handleResponderEAP` reaches `buildAuthResponse`.
3. `selectAuthChildSA` decides the request, allocates through the registry, or refuses with
   FAILED_CP_REQUIRED / INTERNAL_ADDRESS_FAILURE.
4. `narrowChildSelectors` substitutes the lease for the remote selectors, then matches policy.
5. The CFG_REPLY is the first payload of the Child SA half; `PayloadCP.WriteTo` encodes it.
6. `createFirstChildSA` installs XFRM state and policy naming the lease as the remote inner.
7. Rekeys carry the lease; `forgetKeys` releases it when the IKE SA closes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire to engine | `*wire.PayloadCP` in the decrypted chain, out in the response chain | read 2026-10-26 |
| Engine to registry | registry calls under its mutex; pool descriptors through an atomic pointer | design |
| Config to engine | `RemoteAccessConfig.Pools` and the peer binding, applied by `applyConfig` | design |
| Engine to kernel | narrowed selectors installed by `createFirstChildSA` | read 2026-10-26 |

### Integration Points
- `eap` lease registry and `Pool` (`internal/core/eap/pool.go`)
- `wire.PayloadCP` (`internal/component/ike/wire/payload_cp.go`)
- `selectAuthChildSA`, `buildChildlessAuthResponse`, `notifyForRefusal`
- `narrowChildSelectors` (`ts_narrow.go`)
- `(*SA).forgetKeys` (`sa.go`), the rekey swap (`established.go`)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | allocation and reply sit on the existing responder path; no second producer |
| No unintended coupling | Yes (design) | `engine` already imports `eap`; no new component edge |
| No duplicated functionality | Yes (design) | extends `Pool` and `PayloadCP`; one registry |
| Zero-copy preserved | Yes (design) | reply encoded by `WriteTo` into the message buffer |
| Registration over hardcoding | Yes (design) | the reply attributes and SUPPORTED_ATTRIBUTES derive from one handler table in `cp.go`; no central switch elsewhere |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every IKE SA close path reaches `forgetKeys` exactly once | its doc comment and the call sites listed in Current Behavior | a lease leaks or double-releases | `TestLeaseReleasedOnEveryClosePath` drives delete, DPD death, peer stop, reload restart, superseded SA and rekey retirement; a double release returns `ErrNotAllocated` and fails the test | validated: `TestLeaseReleasedOnEveryClosePath` (cp_test.go), plus `TestLeaseReleasedWhenRunEstablishedExits` (the owner loop's exit) and `TestLeaseReleasedOnResponderDeadBranch` (`runResponder`'s StateDead branch) in cp_close_test.go, the two close paths review round 1 found unexercised; each asserts no lease held and no `virtual-ip` left on the closed SA |
| A-2 | The responder-side IKE rekey adopts its successor through a site that can move the lease | the `out.newSA` swap in `established.go`; `responder-ike-rekey` scenario exists | the lease is released at a responder rekey and the address re-issued | `TestIKERekeyKeepsLease` in both rekey directions | unvalidated |
| A-3 | The interop lab can give strongSwan two source addresses translated by the NAT box | `nat.conf` takes one translation per line; secondary addresses are how the box itself works | `virtual-ip-uniqueness` cannot be written as designed | write the scenario first; if the strongSwan container cannot hold two addresses, extend `internal/le/interoplab/ipsec` to add declared secondary addresses (named in Files) | validated 2026-10-26: the container holds a second address (the checker adds 172.28.0.8/32 to strongSwan's eth0, as `mobikeMove` adds one); the lab limit was `natTranslationsMax = 2` in `nat.go`, raised to 3 so `nat.conf` maps Ze and both clients (commit `5fcf169117`) |
| A-4 | strongSwan accepts the CFG_REPLY ahead of SAr2 with Ze's attribute set | strongSwan `process_i` reads CP from the last IKE_AUTH | G-3 fails | `virtual-ip-assignment` | unvalidated |
| A-5 | A `respond` peer is how configured clients reach Ze | `responder-psk`, `responder-eap-mschapv2` | the `connection-type` refusal blocks a real use | the two scenarios use `respond` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | lease leak on a close path | held count does not return to base over N cycles | release only in `forgetKeys`; AC-11 cycles |
| R-2 | lease added after narrowing, so the selectors name the self-chosen address | client gets an address and the installed policy names 10.1.2.3 | substitution is inside `narrowChildSelectors`; AC-5 asserts the installed selector |
| R-3 | EAP final skips narrowing and installs the pre-auth result | EAP client passes no traffic | AC-6 and `virtual-ip-assignment` run over EAP |
| R-4 | a pool rebuild re-issues a live address | duplicate inner address after a commit | registry outlives pools; AC-30 |
| R-5 | site-to-site peers change bytes | `psk-site-to-site` or `child-rekey-narrowing` red | every new branch is conditional on a pool binding; AC-16 |
| R-6 | `rfc-status.md` still claims no CP | grep hit | Documentation checklist row 9 |
| R-7 | new RFC ids collide with marks | `./le rfc check` refusal | recompute marks at landing |
| R-8 | a later Child SA of a leased peer names a non-leased source | second child carries the proposed address | AC-7b over CREATE_CHILD_SA |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Two peers hold one inner address and return traffic goes down the wrong SA, the exact Section 5.1 hazard; or a narrowing defect drops traffic for site-to-site peers; or a leak exhausts the pool |
| How is it reverted? | One commit while no operator binds a pool; nothing depends on the behavior before that |
| Who else touches this path? | `spec-ipsec-remote-access.md` (depends on this), `plan/post-release/features/spec-ike-reauth.md` (IKE SA lifecycle), `plan/post-release/features/spec-ipsec-opaque-selector-port-mask.md` (selectors) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH carrying CP(CFG_REQUEST), direct flow | → | `selectAuthChildSA` allocation through the registry | `TestResponderCFGRequestLeasesFromBoundPool` |
| IKE_AUTH carrying CP(CFG_REQUEST), EAP flow | → | stored request, final-response allocation | `TestCFGReplyInEAPFinalResponse` |
| operator config `virtual-ip-pool` | → | the pool the responder consults | `TestPeerBindingSelectsNamedPool` |
| IKE SA close | → | registry release | `TestLeaseReleasedOnEveryClosePath` |
| strongSwan client asking for a virtual IP | → | the whole chain | interop `virtual-ip-assignment` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | pool-bound peer, IKE_AUTH with CP(CFG_REQUEST) holding an empty INTERNAL_IP4_ADDRESS | response carries CFG_REPLY with INTERNAL_IP4_ADDRESS inside the bound pool's range |
| AC-2 | the same, INTERNAL_IP4_NETMASK also requested; and an IPv6-only lease | exactly one netmask, only beside INTERNAL_IP4_ADDRESS; the IPv6-only reply carries none |
| AC-3 | pool with IPv4 and IPv6 DNS servers, address assigned | DNS servers of each assigned family under INTERNAL_IP4_DNS / INTERNAL_IP6_DNS, requested or not |
| AC-4 | any response carrying CP and SA | CP index strictly less than SA index, both flows |
| AC-5 | a lease granted to a peer requesting 10.1.2.3: (a) its TSi proposal contains the lease (0.0.0.0/0, as strongSwan proposes before it knows its address); (b) its TSi proposal is 10.1.2.3/32 only | (a) response TSi and the installed Child SA's remote selector are the leased host address, and 10.1.2.3 appears in neither; (b) TS_UNACCEPTABLE, no TS payload, IKE SA kept |
| AC-6 | EAP session requesting an address | CFG_REPLY in the final IKE_AUTH (carrying SAr2), absent from the first, and the installed selector names the lease |
| AC-7 | two pool-bound peers, both requesting 10.1.2.3, connected at the same time | the first holds 10.1.2.3, the second a different address; the two installed Child SAs name two distinct remote inner sources |
| AC-7b | a leased peer opens a second Child SA by CREATE_CHILD_SA, or rekeys one: (a) proposing TSi selectors of which one contains the lease; (b) proposing only TSi selectors that do not contain it (10.1.2.3/32) | (a) the new Child SA's remote selector is the lease; (b) TS_UNACCEPTABLE, and no answer recorded |
| AC-8 | a peer requesting a specific free in-pool address | that address is returned |
| AC-9 | pool exhausted | INTERNAL_ADDRESS_FAILURE, auth half sent, IKE SA established, no Child SA installed, no CFG_REPLY, no address handed out twice |
| AC-10 | IKE SA closes for a leased peer | held count returns to its pre-connect value at teardown |
| AC-11 | N connect and disconnect cycles of one peer | held count returns to base; the peer gets its previous address back each time |
| AC-12 | no CP, or CP of type Reply, Set or ACK, or CP with zero attributes, or two CP payloads, on a peer NOT bound to a pool | no CP in the response, session as today; a well-formed request on a bound peer DOES get a reply |
| AC-13 | CFG_REQUEST with unknown types beside INTERNAL_IP4_ADDRESS | correct CFG_REPLY; unknown types in neither reply nor state; message not refused |
| AC-14 | INTERNAL_IP4_ADDRESS with the Reserved bit set | recognized as type 1 and answered |
| AC-15 | IPv6-only request against an IPv4-only pool | no wrong-family address; INTERNAL_ADDRESS_FAILURE; IKE SA established; dual request against a v4-only pool gets the IPv4 address only |
| AC-16 | site-to-site peer with no pool, with and without CP in its request | byte-identical response and selectors to HEAD; CP ignored |
| AC-17 | concurrent allocate and release from many goroutines under `-race` | no duplicate, no race report |
| AC-18 | IPv6 pool longer than /64 (a /96) | every lease inside the prefix |
| AC-19 | CP with 65 attributes; CP with a 1 to 3 octet trailing remnant | refused as malformed; 64 attributes accepted |
| AC-20 | `NewPool` with an unrepresentable range (/31, /32, IPv6 /127, /128) | error, no pool |
| AC-21 | pool-bound peer sends no valid CFG_REQUEST | FAILED_CP_REQUIRED, no SA or TS payload, IKE SA established, no Child SA installed (refusal precedes `createFirstChildSA`) |
| AC-22 | CFG_REQUEST in the first IKE_AUTH, direct and EAP; and only in the final EAP IKE_AUTH | recognized and answered in each case |
| AC-23 | attribute type 5 in a CFG_REQUEST | ignored; the rest of the payload and the SA proposal processed as usual |
| AC-24 | zero-length SUPPORTED_ATTRIBUTES requested | reply attribute of even length listing exactly the handled types, derived from the handler table |
| AC-25 | a peer whose address was released reconnects while it is free | the same address |
| AC-26 | one peer with an established SA re-initiates in parallel | the new SA holds the same address; the pool held count rises by 0, and after the old SA closes the address is still held |
| AC-27 | a pool cycled through every address with releases between | the rotating cursor does not hand the most recently released address to a different owner while another address is free |
| AC-28 | pool at 90% held | `ze doctor` names the pool and the fix |
| AC-29 | CFG_REQUEST asking APPLICATION_VERSION, in IKE_AUTH on a pool-bound peer; and in INFORMATIONAL | IKE_AUTH: APPLICATION_VERSION with an empty value in the CFG_REPLY; INFORMATIONAL: no CP payload, as at HEAD |

-> Decision (AC-29, 2026-10-26, supervisor): RFC 7296 Section 2.20 (`rfc/full/rfc7296.txt`, line 3209) reads "In that case, it MUST either return an empty string or no CP payload if CP is not supported." Ze takes both permitted forms: the IKE_AUTH CFG_REPLY to a granted request carries the empty string (`buildCFGReply`, `TestSupportedAttributesDerived`), and an INFORMATIONAL CFG_REQUEST keeps HEAD's no-CP answer, which two committed RFC7296-2.20-1 tagged tests pin: `TestRFC7296InformationalVersionRequestDrawsNoCP` and `TestRFC7296InformationalVersionRequestReflectsNoVersionString` (`internal/component/ike/engine/rfc7296_cp_version_informational_test.go`). `inbound.go` is unchanged.
| AC-30 | pool range edited while a lease is live | the live address is never handed to another peer; the old lease stays held until its IKE SA closes |

-> Decision (AC-5, AC-7b, 2026-10-26): the lease is answered only inside the initiator's proposal. RFC 7296 Section 2.9: "When the responder chooses a subset of the traffic proposed by the initiator, it narrows the Traffic Selectors to some subset of the initiator's proposal (provided the set does not become the null set)." `substituteVirtualIP` (`cp.go`) keeps the lease host for a proposed TSi only when that selector contains it (same family, read from the mask length), drops the others, and refuses with TS_UNACCEPTABLE when none does, so the rekey floor check judges a proposal that genuinely covers the floor. strongSwan as a gateway does the same: `traffic_selector_list.c` `select_` intersects the selector resolved to the virtual IP with each received one, `selected = ts1->get_subset(ts1, ts2);`. The earlier wording replaced any proposal with the lease, which answered traffic the peer never proposed (audit 469838479d, RFC7296-2.9-1 weak). Tests: `TestRFC7296VirtualIPAnsweredOnlyInsideTheProposal` (`RFC7296-2.9-1` positive and negative), `TestRemoteSelectorIsLease`, `TestCreateChildUsesLease`.
| AC-31 | commit with overlapping pools / unknown pool name / `virtual-ip-pool` on an `initiate` peer / selectors not covering the pool / `domain` set | each refused with an error naming the item, why, and the fix |
| AC-32 | IKE SA rekey, Ze initiating and Ze responding | lease kept by the successor; held count unchanged; no release at retirement |
| AC-33 | `show vpn ipsec sa` for a leased peer | the `virtual-ip` field shows the lease, in `\| json` too |

## Goal Validation (planned evidence)

| Goal | Evidence that will prove it |
|------|------------------------------|
| G-1 Conformance | interop `virtual-ip-uniqueness`: two strongSwan connections from two NAT-translated sources both request 10.1.2.3; the checker reads Ze's installed XFRM policies and requires two distinct remote inner sources, one of them 10.1.2.3, and traffic from each passes. Plus the tagged pair for `RFC3948-5.1-1` (`TestTwoPeersSameRequestedAddressDistinctLeases` positive, `TestNoChildSAWithoutLeaseOnBoundPeer` negative) with a `./le rfc discriminate-record` record. **Interop run 2026-10-26:** GREEN (`virtual-ip-uniqueness: pass`, logs `tmp/session/2026-10-25-f3ae7c00-09af-4420-914f-39f2f7169980/scratch/vip-unique-green1.log` and, after the red, `virtual-ip-uniqueness-green2.log`). RED with `substituteVirtualIP` made a no-op through a build overlay (`GOFLAGS=-overlay=`, `scratch/vip-red/overlay.json`, no shared-tree edit; the lab rebuilt the Ze binary and image): `strongSwan could not bring up ze-b ... establishing CHILD_SA 'child-b' failed`, Ze logging `refused the IKE_AUTH Child SA ... install inbound policy 10.1.2.0/24 -> 10.20.0.1/32: ... is installed for peer "swan-a"`, which is the Section 5.1 hazard: without the lease substitution both clients name one inner selector and the second cannot get an SA (`scratch/vip-unique-red.log`) |
| G-2 Reachability | `TestResponderCFGRequestLeasesFromBoundPool` and `TestCFGReplyInEAPFinalResponse` drive encrypted IKE_AUTH requests through `handleAuthRequest`, as `rfc3948_spi_test.go` does, and assert the lease is held in the registry |
| G-3 Interoperability | interop `virtual-ip-assignment` (EAP-MSCHAPv2): strongSwan `vips = 0.0.0.0` gets an address inside Ze's range (read from `swanctl --list-sas` and Ze `show vpn ipsec sa \| json`), DNS installed, ping both directions over the Child SA. **Interop run 2026-10-26:** GREEN (`virtual-ip-assignment: pass`, logs `tmp/session/2026-10-25-f3ae7c00-09af-4420-914f-39f2f7169980/scratch/vip-assign-green1.log` and, after the red, `virtual-ip-assignment-green2.log`). RED with `substituteVirtualIP` made a no-op through the same build overlay: `ze's Child SA of peer swan has ts-remote 10.10.0.0/24, want the lease 10.10.0.1/32` (`scratch/vip-assign-red.log`). strongSwan still established, held 10.10.0.1 and narrowed its own side to it, so the client-side reads alone would have passed: the Ze selector and policy assertions are the discriminating ones |
| G-4 Honest configuration | `test/ipsec/ipsec-virtual-ip-pool-config.ci` commits a valid binding and is refused for each AC-31 case; the YANG descriptions and `docs/guide/ipsec.md` no longer say the pool is unread |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | operator binds a pool and a strongSwan client dials in | config, IKE_AUTH CP, allocation, substitution, CFG_REPLY, Child SA | `virtual-ip-assignment` |
| 2 | two clients that chose 10.1.2.3 connect at once | two leases, two distinct inner sources | `virtual-ip-uniqueness` |
| 3 | a client reconnects | release at teardown, same address back | `TestLeaseReleasedOnEveryClosePath`, AC-25 unit |
| 4 | operator checks leases | `show vpn ipsec sa` `virtual-ip` | `TestShowSAReportsVirtualIP`, `virtual-ip-assignment` checker |
| 5 | pool too small | INTERNAL_ADDRESS_FAILURE, doctor warning | `TestPoolExhaustionRefusesWithInternalAddressFailure`, `TestDoctorVirtualIPPoolUtilization` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResponderCFGRequestLeasesFromBoundPool` | `internal/component/ike/engine/cp_test.go` | AC-1, G-2 | |
| `TestNetmaskOnlyBesideIPv4Address` | `cp_test.go` | AC-2 | |
| `TestDNSAttributesSplitByFamily` | `cp_test.go` | AC-3 | |
| `TestCFGReplyPrecedesSAPayload` | `cp_test.go` | AC-4 (both flows) | |
| `TestRemoteSelectorIsLease` | `cp_test.go` | AC-5 | |
| `TestCFGReplyInEAPFinalResponse` | `cp_eap_test.go` | AC-6, AC-22 | |
| `TestTwoPeersSameRequestedAddressDistinctLeases` | `cp_test.go` | AC-7, `RFC3948-5.1-1` positive | |
| `TestNoChildSAWithoutLeaseOnBoundPeer` | `cp_test.go` | AC-9, AC-21, `RFC3948-5.1-1` negative | |
| `TestCreateChildUsesLease` | `cp_test.go` | AC-7b | |
| `TestRequestedAddressHonoured` | `internal/core/eap/pool_test.go` | AC-8 | |
| `TestPoolExhaustionRefusesWithInternalAddressFailure` | `cp_test.go` | AC-9 | |
| `TestLeaseReleasedOnEveryClosePath` | `cp_lease_test.go` | AC-10, AC-11, A-1 | |
| `TestNoCPReplyWithoutRequest` | `cp_test.go` | AC-12 | |
| `TestUnknownAttributesIgnored` | `cp_test.go` | AC-13, AC-23 | |
| `TestReservedBitAttributeRecognized` | `cp_test.go` | AC-14 | |
| `TestFamilyFidelity` | `cp_test.go` | AC-15 | |
| `TestUnboundPeerUnchanged` | `cp_test.go` | AC-16 | |
| `TestRegistryConcurrentNoDuplicate` | `internal/core/eap/pool_test.go` | AC-17 | |
| `TestIPv6LongPrefixInRange` | `pool_test.go` | AC-18 | |
| `TestCPReadFromBounds` | `internal/component/ike/wire/payload_cp_test.go` | AC-19 | |
| `TestNewPoolRefusesUnrepresentable` | `pool_test.go` | AC-20 | |
| `TestSupportedAttributesDerived` | `cp_test.go` | AC-24 | |
| `TestOwnerGetsLastAddress` | `pool_test.go` | AC-25 | |
| `TestParallelReinitSharesLease` | `cp_lease_test.go` | AC-26 | |
| `TestRotatingCursor` | `pool_test.go` | AC-27 | |
| `TestDoctorVirtualIPPoolUtilization` | `internal/component/ike/engine/doctor_test.go` | AC-28 | |
| `TestSupportedAttributesDerived` (IKE_AUTH); `TestRFC7296InformationalVersionRequestDrawsNoCP`, `TestRFC7296InformationalVersionRequestReflectsNoVersionString` (INFORMATIONAL, committed) | `cp_test.go`, `rfc7296_cp_version_informational_test.go` | AC-29 | |
| `TestPoolEditKeepsLiveLease` | `internal/component/ike/engine/apply_test.go` | AC-30 | |
| `TestVirtualIPPoolCommitRefusals` | `internal/component/ike/ipsec/config_test.go` | AC-31 | |
| `TestIKERekeyKeepsLease` | `cp_lease_test.go` | AC-32, A-2 | |
| `TestShowSAReportsVirtualIP` | `internal/component/ike/engine/show_test.go` or the existing SA show test file | AC-33 | |
| `TestPeerBindingSelectsNamedPool` | `apply_test.go` | wiring | |

Tagged units (`RFC requirement:`) carry the new RFC 7296 rows and `RFC3948-5.1-1`, each with
a positive and a negative unit and a `./le rfc discriminate-record` record
(`docs/contributing/rfc-conformance-gates.md`).

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IPv4 pool prefix | /8 to /30 | /30 (2 hosts) | /7 | /31 |
| IPv6 pool prefix | /48 to /126 | /126 | /47 | /127 |
| CP attribute count | 0 to 64 | 64 | N/A | 65 |
| CP trailing remnant | 0 octets | 0 | N/A | 1 to 3 octets |
| hosts in a /30 pool | 2 | the second lease | N/A | the third request draws INTERNAL_ADDRESS_FAILURE |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-virtual-ip-pool-config` | `test/ipsec/ipsec-virtual-ip-pool-config.ci` | operator binds a pool (accepted) and each AC-31 mistake is refused with its named error | |

A `.ci` cannot reach the lease path: functional IPsec tests run ze against ze, and Ze never
sends CFG_REQUEST. The real client entry point is covered by the strongSwan scenarios below.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `virtual-ip-assignment` | `test/interop-ipsec/scenarios/virtual-ip-assignment/` | strongSwan | EAP-MSCHAPv2 client with `vips = 0.0.0.0` gets an in-range address and DNS, traffic both ways, Ze `show` reports it (G-3, multi-IKE_AUTH path) | green, red recorded (Goal Validation G-3) |
| `virtual-ip-uniqueness` | `test/interop-ipsec/scenarios/virtual-ip-uniqueness/` | strongSwan + NAT box | two PSK connections from two NAT-translated sources, both `vips = 10.1.2.3`, get distinct leases; Ze installs two distinct remote inner sources (G-1, direct path) | green, red recorded (Goal Validation G-1) |

Both owe the discrimination walk (`ai/rules/interop-and-goal-validation.md`): revert the
consumer, rebuild the Ze image, confirm RED, restore, confirm GREEN, record the red.

## Files to Modify
- `internal/core/eap/pool.go` - lease registry, owner-aware per-family allocation, requested address, rotating IPv4 cursor, two-family `Available`
- `internal/core/eap/pool_test.go` - stale comment (gap claim, `plan/spec-...` path) rewritten when the gap closes
- `internal/component/ike/engine/apply.go` - registry on `ikeEngineState`, per-pool descriptors swapped atomically, `reloadPool` comment rewritten
- `internal/component/ike/engine/responder.go` - CP collect case; allocation, refusals and CP placement in `selectAuthChildSA`
- `internal/component/ike/engine/responder_eap.go` - store the request and TSi/TSr
- `internal/component/ike/engine/ts_narrow.go` - lease substitution in `narrowChildSelectors`
- `internal/component/ike/engine/inbound.go` - CREATE_CHILD_SA substitution; INFORMATIONAL APPLICATION_VERSION
- `internal/component/ike/engine/transport_mode.go` - pool-bound peers decline transport mode
- `internal/component/ike/engine/sa.go` - lease field; release in `forgetKeys`
- `internal/component/ike/engine/established.go` - lease moves at the rekey swap
- `internal/component/ike/engine/notify_error.go` - `notifyForRefusal` maps the two new refusals
- `internal/component/ike/engine/metrics.go`, `doctor.go` - gauge, counters, doctor check
- the SA show producer in `internal/component/ike/engine/` - `virtual-ip` field
- `internal/component/ike/wire/payload_cp.go` - remnant error, cap, constants 5, 13, 14, 15
- `internal/component/ike/ipsec/config.go`, `types.go`, `validate.go` - all pools, peer binding, refusals
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `virtual-ip-pool`, `dns` leaf-list, descriptions made true (stale `plan/immediate/` paths removed)
- `internal/le/interoplab/ipsec/` - only if A-3 shows the strongSwan container cannot hold two source addresses
- `rfc/short/rfc3948.md`, `rfc/short/rfc7296.md`, `rfc/extraction/rfc3948.json`, `ai/RFC-REQUIREMENTS.md` (via `./le rfc index-update`), `docs/features/rfc-status.md` (generated)
- `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - owner-aware allocation, rotating cursor, registry
- `docs/architecture/ike/ipsec-14-responder.md` - CP in the responder exchange
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - registry on engine state, pool reload
- `docs/architecture/ike/ipsec-3-data-model.md` - `virtual-ip-pool`, all pools, `dns` leaf-list
- `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` - lease substitution in narrowing
- `docs/architecture/ike/ipsec-10-cli-diag.md` - `virtual-ip` SA field, doctor check
- `docs/guide/ipsec.md`, `docs/config-reference.md`, `docs/features.md`, `docs/comparison.md`, `docs/architecture/testing/interop.md`
- `plan/release/fixes/spec-ipsec-remote-access.md` - ownership transfer recorded (done in this design pass)

## Files to Create
- `internal/component/ike/engine/cp.go`, `cp_test.go`, `cp_eap_test.go`, `cp_lease_test.go`
- `test/ipsec/ipsec-virtual-ip-pool-config.ci`
- `test/interop-ipsec/scenarios/virtual-ip-assignment/` and `virtual-ip-uniqueness/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-ipsec-conf.yang`: `virtual-ip-pool` peer leaf, `dns` leaf-list |
| YANG validation constraints | Yes | commit checks in `ipsec/validate.go` (AC-31) |
| YANG custom validators | Yes | the existing prefix validators for `range`/`range6`, plus overlap and coverage checks |
| CLI commands/flags | No | `show vpn ipsec sa` gains a field, no new verb |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | Yes | pool names offered for `virtual-ip-pool` through the existing completion of referenced names |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-virtual-ip-pool-config.ci` |
| Pipe completeness | Yes | `virtual-ip` is a field of the existing SA payload, so `\| json` renders it |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | Yes | `doctor-ipsec-virtual-ip-pool` (pool utilization), `doctor.go` |
| Prometheus counters/metrics | Yes | `metrics.go` |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`; `features/<id>.md` level per `./le feature report`. Done: `features/ipsec-eap-authentication.md`, the declaration that carries the pool, states the binding, the unique lease, the lease-only Child SAs and the `virtual-ip` field; level unchanged |
| 2 | Config syntax changed? | Yes | `docs/config-reference.md`, `docs/guide/ipsec.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`virtual-ip` field). Done: the `show vpn ipsec` section names the field; `docs/guide/ipsec.md` and `ipsec-10-cli-diag.md` already did |
| 4 | API/RPC added/changed? | No | no RPC |
| 5 | Plugin added/changed? | N-A | IKE is a component |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | Yes | CP joins emitted messages: `ipsec-14-responder.md` |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented/changed/proven? | Yes | `rfc/short/rfc3948.md`, `rfc/short/rfc7296.md`, `rfc/extraction/rfc3948.json`, generated `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (two scenarios; secondary addresses if A-3 needs them) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `ipsec-9-ikev2-eap-nat.md`, `ipsec-7-ikev2-engine.md`, `rfcgate-1b-rfc7296-pilot.md` |
| 13 | Route metadata keys? | N-A | |
| 14 | Prometheus counters added/changed? | Yes | the IPsec telemetry page |
| 15 | Registered inventory changed? | Yes | the feature inventory row that says the pool is not wired |
| 16 | Changed source referenced by doc anchors? | Yes | rerun `./le spec citation anchors spec plan/release/fixes/spec-ike-virtual-ip-assignment.md` at each phase; at 2026-08-31 it named `ipsec-3-data-model.md`, `ipsec-7-ikev2-engine.md`, `rfcgate-1b-rfc7296-pilot.md`, `ipsec-9-ikev2-eap-nat.md` as declared |
| 17 | Existing examples for this area? | Yes | `remote-access` examples in `docs/guide/ipsec.md` |

## Implementation Steps

1. **Codec** - `payload_cp.go` bounds and constants. Tests: `TestCPReadFromBounds`.
2. **Registry and pool** - D-2. Tests: the `pool_test.go` rows. Page: `ipsec-9-ikev2-eap-nat.md`.
3. **Config** - D-1, YANG, parse, refusals, `.ci`. Page: `ipsec-3-data-model.md`, guide, config reference.
4. **Engine wiring** - registry on engine state, atomic descriptors, `reloadPool`. Tests: `TestPeerBindingSelectsNamedPool`, `TestPoolEditKeepsLiveLease`. Page: `ipsec-7-ikev2-engine.md`.
5. **Consumer** - `cp.go`, collect, decide, allocate, refuse, reply placement, both flows. Tests: AC-1..AC-4, AC-6, AC-9, AC-12..AC-16, AC-21..AC-24, AC-29. Page: `ipsec-14-responder.md`.
6. **Selectors** - substitution in `narrowChildSelectors`, EAP re-narrow, CREATE_CHILD_SA, transport decline. Tests: AC-5, AC-7, AC-7b. Page: `rfcgate-1b-rfc7296-pilot.md`.
7. **Lifetime** - forgetKeys release, rekey move, parallel re-init. Tests: AC-10, AC-11, AC-25, AC-26, AC-32.
8. **Visibility** - show field, metrics, doctor. Tests: AC-28, AC-33. Pages: CLI diag, telemetry.
9. **Interop and proof** - both scenarios, discrimination walks, tagged pairs, RFC rows (marks recomputed), `./le rfc index-update`, `./le rfc check`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N implemented; AC-7 over two LIVE sessions |
| Correctness | allocation before narrowing; CP before SA; release exactly once; refusals before `createFirstChildSA` |
| Fail-closed | unresolvable binding denies; exhausted pool never sends an empty CFG_REPLY |
| Naming | scenario directories named, no numeric prefix; JSON key `virtual-ip` |
| Data flow | one registry, one insertion point, no second allocator |
| Rule: `ai/rules/rfc-compliance.md` | every MUST enforced carries its quoted section; claims no wider than test bodies |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| registry has a non-test caller | grep `internal/component/ike/engine` non-test files for the registry allocate call |
| CFG_REPLY on the wire | `./le test integration interop-ipsec` scenario `virtual-ip-assignment` |
| uniqueness | scenario `virtual-ip-uniqueness` |
| gap closed | `./le rfc check` shows no `RFC3948-5.1-1` gap |
| reversal disclosed | `docs/features/rfc-status.md` no longer says Ze builds no Configuration payload |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | attribute count, each length and the remnant bounded before any allocation |
| Resource exhaustion | no lease before authentication; one address per identity per family; release on every close path |
| Authorization fails closed | an unbound peer never gets an address; an unresolvable binding denies |
| Information leakage | refusals name the notify only, nothing about pool size or other clients |
| Cross-client isolation | release never frees another owner's lease; holder counts per owner |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | fix in the phase that introduced it |
| Test fails for the wrong reason | fix the test setup |
| Behavior mismatch | re-read Current Behavior; misunderstood means back to RESEARCH |
| Lint failure | fix inline; architectural means DESIGN |
| Interop red | check the AC; wrong AC goes to DESIGN |
| 3 fix attempts failed | stop and report all three |

## Design Insights

- The lease is the initiator's inner address, so it lands on the REMOTE selector. The
  skeleton's "TSr" would have narrowed Ze's own protected side to the client's address.
- Uniqueness must live in an object that outlives configuration. A pool rebuilt on edit is a
  configuration object; the leases are runtime state.
- strongSwan's identity-bound leases answer reauthentication, parallel re-initiation and the
  per-identity drain in one rule: an owner never holds two addresses of one family.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The open `unwired-feature.md` row of 2026-08-16 (the pool built and never used) is owned by
`plan/release/fixes/spec-wire-or-delete-unreached-code.md`; this spec's implementation is
what discharges it, and its closure marks that row fixed.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Assign a locally unique address per peer | NAT the inner sources (also permitted by Section 5.1) | the RECOMMENDED remedy, the pool and codec exist, Ze has no inner-NAT layer |
| This spec owns all CP work | fold into `spec-ipsec-remote-access.md` | fix/feature split rule in `ai/META-DECISIONS.md`; the fix is reachable without admission |
| Per-peer `virtual-ip-pool` leaf; binding implies CP required | a `configuration-payload` container with a `required` leaf; a global default pool | strongSwan binds pools to the connection and requires CP when pools exist; one leaf, no option nobody asked for |
| Serve every pool, refuse overlaps | refuse a second pool | per-peer selection makes multiple pools free; overlap refusal keeps uniqueness checkable at commit |
| Engine-wide lease registry outliving pools | leases inside each `Pool` object | a rebuild would forget live leases and re-issue them |
| Release at IKE SA end, owner remembers its last address (AC-26 mapping) | a configured offline-lease expiry timer | RFC 7296 Section 2.19 ties the address to the IKE SA; immediate return cannot drain the pool; reuse follows strongSwan's offline lease without a timer or a new leaf |
| One address per identity per family (AC-27 mapping) | a configured per-identity session maximum | strongSwan `get_existing`; a single credential cannot hold two addresses, so it cannot drain the pool, and no leaf is added |
| Pool-bound peer without a lease gets no Child SA | install the self-chosen address | strongSwan `ike_sa_get_dynamic_hosts`; installing it recreates the Section 5.1 conflict |
| IPv6 prefix length in the reply is the pool prefix | strongSwan's fixed 64 | RFC 7296 Section 3.15.3 makes it the analogue of the netmask; strongSwan accepts any value |
| DNS sent whenever configured | only when requested | strongSwan `create_responder_enumerator`; Section 3.15.1 "It MAY also add new attributes" |

## Known Limitations

- Road-warrior admission is `spec-ipsec-remote-access.md`; until it lands, a client connects as
  a configured `respond` peer.
- No lease persistence across a daemon restart.
- Address requests in INFORMATIONAL or CREATE_CHILD_SA are not served (strongSwan serves them
  in IKE_AUTH only); the Section 3.15.4 retry by INFORMATIONAL therefore gets no address.
- No search-domain attribute (Q-1).
- A childless IKE_AUTH (no SAi2) from a pool-bound peer is answered without a lease, because
  `selectAuthChildSA` refuses the missing SAi2 before `assignVirtualIP` runs. It fails closed:
  every later CREATE_CHILD_SA on that IKE SA draws TS_UNACCEPTABLE, since no Child SA may name
  an inner address the peer was not leased. The peer must reconnect with a Child SA in IKE_AUTH.

## Open questions

| Id | Question | Evidence | What each answer changes |
|----|----------|----------|--------------------------|
| Q-1 | The pool's `domain` leaf: refuse it at commit, or implement RFC 8598 INTERNAL_DNS_DOMAIN (attribute 25)? | `ze-ipsec-conf.yang` `list pool` `leaf domain`; `VirtualIPPool.Domain`; RFC 7296 Section 3.15.1 lists no domain attribute; RFC 8598 is not under `rfc/short/` and RFC enrolment is frozen; strongSwan sends attribute 25 only through its `attr` plugin when configured | refuse: AC-31 covers it and the leaf's description says so; implement: needs RFC 8598 enrolment (an owner scope call), an attribute constant, one reply attribute and a tagged pair. Proceeding reading: refuse |

## RFC Documentation (Scope: protocol)

Quoted above the enforcing code (byte-exact, read 2026-10-26 from `rfc/full/`):
- RFC 3948 Section 5.1: "Implementors MUST devise ways of preventing this from occurring."
- RFC 7296 Section 2.19: "In all cases, the CP payload MUST be inserted before the SA payload."
- RFC 7296 Section 2.19: "In variations of the protocol where there are multiple IKE_AUTH exchanges, the CP payloads MUST be inserted in the messages containing the SA payloads."
- RFC 7296 Section 2.19: "The responder MUST NOT send a CFG_REPLY without having first received a CP(CFG_REQUEST) from the initiator, because we do not want the IRAS to perform an unnecessary configuration lookup if the IRAC cannot process the REPLY."
- RFC 7296 Section 2.19: "IRAS MUST fail the request, and terminate the Child SA creation with a FAILED_CP_REQUIRED error."
- RFC 7296 Section 2.19: "That address persists at least until the deletion of the IKE SA."
- RFC 7296 Section 2.20: "In that case, it MUST either return an empty string or no CP payload if CP is not supported."
- RFC 7296 Section 3.15.1: "The requested address is valid as long as this IKE SA (or its rekeyed successors) requesting the address is valid."
- RFC 7296 Section 3.15.1: "Only one netmask is allowed in the request and response messages (e.g., 255.255.255.0), and it MUST be used only with an INTERNAL_IP4_ADDRESS attribute."
- RFC 7296 Section 3.15.1: "Unrecognized or unsupported attributes MUST be ignored in both requests and responses."
- RFC 7296 Section 3.15.4: "If the responder encounters an error while attempting to assign an IP address to the initiator during the processing of a Configuration payload, it responds with an INTERNAL_ADDRESS_FAILURE notification."
- RFC 7296 Section 3.15.4: "In this case, the responder simply ignores the type of address it does not support and processes the rest of the request as usual."

Requirement ids: `RFC3948-5.1-1` closes. New RFC 7296 rows (verbatim spans) for Sections 2.19
(CP before SA, CP in SA-bearing message, no CFG_REPLY without request, FAILED_CP_REQUIRED),
3.15.1 (netmask, ignore unknown, SUPPORTED_ATTRIBUTES), 1.7 (type 5) and 4 (parse
CFG_REQUEST, reply with the requested type), each at an ordinal above its section's mark at
landing, plus a 3.15.4 row for INTERNAL_ADDRESS_FAILURE. `RFC7296-2.20-1`'s verdict moves from
"no CP payload" to "empty string". `RFC7296-2.19-1` and `-2.19-4` stay conformant by
non-participation (Ze takes no IRAC role).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "Doctor check" for the pool utilization check

### Goal Gates (MUST pass)
- [ ] AC-1..AC-33 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] Scoped evidence for the packages and suites this change reaches (`ai/META-DECISIONS.md`); the full verification is Thomas's to run
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table re-checked against the code
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` test for the config surface
- [ ] Interop scenarios with recorded RED

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** remove the spec only
