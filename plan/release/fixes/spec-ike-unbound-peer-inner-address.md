# Spec: ike-unbound-peer-inner-address

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P0 |
| Release reason | Shipped IKEv2 tunnel-mode responder violates an RFC 3948 Section 5.1 MUST for every peer bound to no virtual-IP pool: two peers can hold Child SAs and kernel policies for one inner address |
| RFC gaps | RFC3948-5.1-1 |
| Depends | - |
| Phase | 1/5 |
| Handoff | - |
| Updated | 2026-10-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 3948 Section 5.1: "Because SGW will now see two possible SAs that lead to
10.1.2.3, it can become confused about where to send packets coming from Suzy's
server.  Implementors MUST devise ways of preventing this from occurring."

`spec-ike-virtual-ip-assignment` met this for a responder peer bound to a
virtual-IP pool (lease registry, TSi substitution, Ze-initiated rekey proposing
the lease only). An independent ze-rfc-audit judge (2026-10-26, verdict `weak`,
`rfc/audit/rfc3948/RFC3948-5.1-1.json`) found the MUST unmet for a tunnel-mode
peer bound to no pool, and the row is now `{partial}` in `rfc/short/rfc3948.md`.

The defect, at the producers:

| Producer | What it does |
|----------|--------------|
| `internal/component/ike/engine/ts_narrow.go::narrowSelectors` | reads an empty configured selector list as allow-everything ("An unconfigured peer allows everything"), so an unbound responder answers whatever inner TSi the peer proposes |
| `internal/component/ike/dataplane/policy_owner.go::claim` | refuses a second owner only for a byte-identical policy key; overlap is compared for transport mode only (RFC 3948 Section 5.2), and tunnel mode is skipped as "a deliberate configuration" |

Probe evidence (the judge's): two unbound peers both proposing TSi 10.1.2.3/32,
one with TSr 0.0.0.0/0 and one with TSr 192.0.2.10/32, were both answered, the
two policy keys differ, and both tunnel policies installed. Configured remote
selectors that overlap another peer's are not refused either.

Two fix options were put to design:

| Option | Behavior | Verdict |
|--------|----------|---------|
| A | Refuse a tunnel Child SA whose peer-chosen selectors overlap another peer's live Child SA selectors (TS_UNACCEPTABLE) | CHOSEN, engine-level, see Key Design Decisions |
| B | Narrow an unconfigured responder's TSi to the peer's observed address, as strongSwan's `%dynamic` default does | REJECTED: insufficient on its own, and it breaks the unconfigured site-to-site and route-based use while refusing every NATed road warrior; see Key Design Decisions |

Goal: no two peers can hold a tunnel Child SA (or a kernel policy) whose
selectors overlap unless the operator configured both, and `RFC3948-5.1-1`
loses its `{partial}` marker with both polarities proven over the unbound path.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` and `ai/doors/access-security.md` (the IKE door)
  → Constraint: `ike/dataplane` is ungated and shared with OSPF (RFC 4552); the guard must not live in a dataplane backend, because the VPP and noop backends do not run `policyOwners` and the obligation is the IKE responder's, not one backend's.
  → Constraint: readers of IKE state outside the engine use `ipsecinventory` / `ikeprobe`; the new registry is engine-internal and needs no core leaf.
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - virtual-IP narrowing and the responder selector path
  → Constraint: `narrowChildSelectors` is the single responder narrowing entry: IKE_AUTH direct (`responder.go` `selectAuthChildSA`), EAP (`responder_eap.go`), CREATE_CHILD_SA new (`create_child.go` `respondNewChild`) and Child rekey (`rekey.go` `respondChildRekey`). A filter placed there reaches every responder path.
- [ ] `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` - "selectors of a peer bound to no pool pass through unchanged"
  → Constraint: that sentence becomes false; the page gains the cross-peer inner-selector guard in the same change.
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - "Policy ownership" (the `// Design:` doc of `policy_owner.go`)
  → Constraint: its last sentence ("Tunnel-mode selectors are compared for identity only, because an overlap between operator prefixes that priority orders is a deliberate configuration") stays true for operator-fixed selectors only; the page must name the engine guard for peer-chosen tunnel selectors.
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the lease registry (`virtual_ip.go` `// Design:`)
  → Decision: the new registry follows the lease registry's shape: one instance on `SATable`, carried to each `PeerSession` the way `SATable.virtualIP` reaches `PeerSession.virtualIP` (`reconcile.go`), mutex-guarded, no package-level global.
- [ ] `docs/guide/ipsec.md` - virtual IP pool section, which discloses this gap (lines 284-289)
  → Constraint: the disclosure paragraph is replaced by the operator-visible rule and the refusal's fix.
- [ ] `plan/learned/035-ike-virtual-ip-assignment.md`
  → Constraint: lines 12-16 say this spec owns the unbound-peer remainder; the learned note's pointer is updated at closure.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3948.md` - RFC3948-5.1-1 (`{partial}`, line 225) and the `Support remaining` row (line 21)
  → Constraint: both texts describe the gap and are rewritten when the row loses `{partial}`; new tagged tests need `./le rfc discriminate-record` records.
- [ ] `rfc/full/rfc3948.txt` Sections 5.1, 5.2
  → Constraint: 5.1 says "it is possible for remote peers to negotiate entries that overlap in an SGW (security gateway), an issue affecting tunnel mode" and "Implementors MUST devise ways of preventing this from occurring." The hazard is a PACKET that matches two peers' SAs ("confused about where to send packets coming from Suzy's server"), so the guard compares whole selector pairs, not the remote half alone.
  → Decision: 5.2 already names the remedy Ze uses for transport mode: "either by disallowing conflicting connections, or by other means." Ze applies the same remedy to tunnel mode.
- [ ] `rfc/full/rfc7296.txt` Section 2.9 (lines 2424-2428)
  → Constraint: "If the responder's policy does not allow it to accept any part of the proposed Traffic Selectors, it responds with a TS_UNACCEPTABLE Notify message." A conflict with another peer is the responder's policy refusing that part, so the notify is TS_UNACCEPTABLE (`errTSUnacceptable`, `notifyForRefusal`).
  → Constraint: Section 2.9 MUST "If the responder's policy allows it to accept the first selector of TSi and TSr, then the responder MUST narrow the Traffic Selectors to a subset that includes the initiator's first choices" (quoted in `narrowSelectors`). Refusing a whole multi-pair proposal because a LATER pair conflicts would break it, so conflicting pairs are DROPPED at narrowing and only an empty remainder is refused.
- [ ] `rfc/full/rfc4301.txt` Section 4.4.1 (line 1087)
  → Constraint: "Thus, a user or administrator MUST be able to order the entries to express a desired access control policy." Overlap between operator-configured selectors is the operator's ordering (`policy-priority`), so it stays legal.

### Peer implementation (interop, owner rule: read the deciding function)
strongSwan upstream master, commit `b8e275ca6230d42fc24edb2d9881e8eb4a1b0c8f`.

| File | Function | What it decides |
|------|----------|-----------------|
| `src/libcharon/plugins/vici/vici_config.c` | child config load | an unset `remote_ts` becomes `traffic_selector_create_dynamic(0, 0, 65535)`: strongSwan's default is `dynamic`, never allow-everything |
| `src/libcharon/sa/ike_sa.c` | `ike_sa_get_dynamic_hosts` | with no virtual IP: `else if (!have_pool(this))` / `{	/* use remote host only if we don't have a pool configured */` / `list->insert_last(list, this->other_host);` -- `dynamic` resolves to the peer's observed (post-NAT) IKE address |
| `src/libstrongswan/selectors/traffic_selector_list.c` | `resolve_dynamic_ts`, `select_` | a dynamic TS takes the host address (`ts2->set_address(ts2, host);`), then each received TS is intersected: `selected = ts1->get_subset(ts1, ts2);` -- an empty result is TS_UNACCEPTABLE |
| `src/libcharon/sa/ikev2/tasks/child_create.c` | `narrow_ts` | the responder narrows the received TSi with `hosts = ike_sa_get_dynamic_hosts(this->ike_sa, local);`; NAT substitution (`get_transport_nat_ts`) is applied for `MODE_TRANSPORT` only |
| `src/libcharon/sa/ikev2/tasks/child_create.c` | initiator notify handling | on TS_UNACCEPTABLE: `"received %N notify, no CHILD_SA built"` and `/* an error in CHILD_SA creation is not critical */` -- the IKE SA survives |
| `src/libcharon/plugins/kernel_netlink/kernel_netlink_ipsec.c` | `policy_entry_t`, `get_priority` | identical policies are shared (`/** List of SAs this policy is used by, ordered by priority */`); overlaps are ordered by an automatic priority from prefix lengths (`priority -= (policy->sel.prefixlen_s + policy->sel.prefixlen_d) * 512;`). strongSwan never refuses an overlap: its 5.1 protection is the `dynamic` default |

  → Decision: strongSwan as a gateway with no pool and default `remote_ts` answers a NATed tunnel client proposing its private 10.1.2.3/32 with TS_UNACCEPTABLE (dynamic = public address, intersection empty). Two such clients never conflict because NEITHER gets service. Copying that (option B) would turn a prevented conflict into no service for every NATed road warrior without a pool.
  → Decision: a strongSwan initiator handles TS_UNACCEPTABLE in IKE_AUTH gracefully (IKE SA kept, no CHILD_SA), so option A's refusal interoperates.
  → libreswan was not consulted: strongSwan answers both interop questions (what default protects 5.1, and how the initiator reads the refusal).

**Key insights:** (minimal context to resume after compaction)
- Chosen: option A only, enforced in the ENGINE by a new cross-peer inner-selector registry on `SATable`; filter at `narrowChildSelectors` (drop conflicting pairs, TS_UNACCEPTABLE when none left), atomic check-and-record at `installChildSA` (race backstop, both roles), release on every Child SA removal.
- A selector pair is "operator-fixed" when the whole pair as the dataplane installs it (`childPolicyPair`: both prefixes, the protocol both halves admit, both ports) equals a whole configured pair through `samePolicyPair`: a configured traffic selector as its installed pair (`configuredPairs`), or, for a transport-mode pair, the host pair between the configured `local-address` and `remote-address` (`configuredHostPair`) (`classifyInnerPair`, b99c6503a4; Key Design Decisions). A pair narrowed on either half is not operator-fixed. Two overlapping pairs conflict unless they belong to the same peer, or both are operator-fixed AND sit at different `policy-priority` values (`operatorOrdered`, f87c1883ac); equal priority is refused, because the kernel would then order them by install time, not by the operator.
- Overlap = one packet can match both pairs: same if_id, both address sides overlap, protocols meet, ports meet. The dataplane already owns that predicate (`selectorsOverlap`); it is exported over `SPParams`, not rewritten.
- Responder installs BEFORE it answers (`responder.go` line 987 to `buildChildlessAuthResponse`, `create_child.go` `respondNewChild` line 441), so an install-time refusal wrapping `errTSUnacceptable` still reaches the wire as TS_UNACCEPTABLE.
- The new registry records transport-mode pairs too, and compares a pair against another peer's only when at least one of the two is tunnel mode, so a tunnel a peer chose around a transport-mode peer's address is refused (RFC 3948 Section 5.1). Two transport-mode pairs stay with `policyOwners.claim` (Section 5.2), so no overlap is refused twice (Key Design Decisions, "Record transport too").

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/engine/ts_narrow.go` - `narrowSelectors` (line 86), `narrowChildSelectors` (line 600), `policyPairs` (line 416)
  → Constraint: `narrowSelectors` with an empty `policy` returns every proposed combination ("An unconfigured peer allows everything ... This is Section 2.9's second bullet"). `narrowChildSelectors` order: NAT substitution (transport only), `substituteVirtualIP`, `narrowSelectors` against `policyPairs(sa.PeerCfg, false)` (TSi = configured REMOTE), transport single-address filter, `closedSubset`, rekey floor check, then records `sa.NegotiatedPairs`. The new filter goes after the transport filter and BEFORE `closedSubset`, so the closed subset is cut from conflict-free pairs and the first choice still leads.
- [ ] `internal/component/ike/dataplane/policy_owner.go` - `claim` (line 106), `selectorsOverlap` (line 156), `policyKey` (line 37)
  → Constraint: `claim` refuses a different owner on an identical key always, and on an overlapping key for transport mode only. `selectorsOverlap` compares dir, if_id, ifIndex, upper protocol, both port matches and both prefixes. This behavior is preserved; only an exported wrapper over `SPParams` is added for the engine.
- [ ] `internal/component/ike/engine/cp.go` - `substituteVirtualIP` (line 286)
  → Constraint: a pool-bound peer's TSi is already narrowed to its unique lease. Its Child SAs are recorded in the new registry like any other (so an unbound peer cannot take a leased address, and vice versa, first come first served); leases among pool peers never overlap, so pool peers are never refused against each other.
- [ ] `internal/component/ike/engine/ts_nat_substitute.go` - `substituteResponderSelectors`, `transportSelectorsInPlay`, `observedRemoteAddress`
  → Constraint: NAT substitution is transport-mode only (RFC 7296 Section 2.23.1). Tunnel-mode inner selectors are never substituted, which is why tunnel 5.1 is not met by the existing NAT code.
- [ ] `internal/component/ike/engine/register.go` - `matchResponderPeer` (line 738)
  → Constraint: an unsolicited IKE_SA_INIT is matched to the one `respond` peer whose configured `remote-address` equals the source. Two road warriors therefore are two configured peers with two distinct (post-NAT) remote addresses, and the owner identity of a Child SA is the peer name (`ChildSA.Owner`).
- [ ] `internal/component/ike/engine/responder.go` - `selectAuthChildSA` (line 1026), the `createFirstChildSA` call (line 987)
  → Constraint: narrowing precedes install, and an install error is answered with `buildChildlessAuthResponse`, which maps the error with `notifyForRefusal`. An error wrapping `errTSUnacceptable` is therefore sent as TS_UNACCEPTABLE and the IKE SA is kept (RFC 7296 Section 2.21.2).
- [ ] `internal/component/ike/engine/create_child.go` - `respondNewChild` (line 352: narrow at 379, `installChildTolerant` at 441); `applyChildCreateResponse` (line 964, initiator install at 1070)
  → Constraint: the CREATE_CHILD_SA responder also installs before it answers; the initiator installs after adopting the answer.
- [ ] `internal/component/ike/engine/child.go` - `installChildSA` (line 379), `installChildTolerant` (line 613), `removeChildSA` / `removeChildSAExcept` (lines 835, 859), `childPairPolicy` (line 706)
  → Constraint: `installChildTolerant` returns early on a nil dataplane and otherwise calls `installChildSA`; an XFRM-unsupported error degrades without failing. `removeChildSAExcept` returns early on a nil dataplane or child. The record must be taken before any kernel call and released before those early returns. `childPairPolicy` already builds the `SPParams` (Src, Dst, ports, proto, IfID, Owner) of one pair, which is the shape the overlap predicate compares.
- [ ] `internal/component/ike/engine/virtual_ip.go`, `reconcile.go` (line 42-47, 762), `table.go` (line 12-20)
  → Decision: the registry rides `SATable` and is copied into `PeerSession` at session start exactly like `virtualIP`.
- [ ] `internal/component/ike/ipsec/config.go` (line 535-548), `types.go` (line 666), YANG `ze-ipsec-conf.yang` (`virtual-ip-pool` line 817, `policy-priority` line 938, `traffic-selector` line 970)
  → Constraint: the operator surfaces named in the refusal's fix are `vpn ipsec site-to-site peer <name> traffic-selector <n> remote prefix <prefix>` and `vpn ipsec site-to-site peer <name> virtual-ip-pool <pool>`. No new leaf is added.
- [ ] `test/interop-ipsec/scenarios/virtual-ip-uniqueness/` and `internal/le/interoplab/ipsec/checkers.go` (line 96), `virtualip.go` (`checkVirtualIPUniqueness`, line 335), `nat.go`
  → Decision: the new interop scenario copies this two-clients-behind-NAT lab (strongSwan holds 172.28.0.3 and adds 172.28.0.8, NAT shows them as 172.28.0.7 and 172.28.0.9, Ze is 172.28.0.2 seen as 172.28.0.6) with the pool removed.
- [ ] `test/ipsec/ipsec-sa-show.ci`, `ipsec-child-rekey-xfrm-narrowing.ci`, `ipsec-traffic-selector-config.ci`
  → Constraint: multi-daemon `.ci` tests use loopback addresses (127.0.0.1, 127.0.0.2) sharing `ze_test_ike_port`, and `ze_test_ike_dataplane=noop` runs without CAP_NET_ADMIN. An engine-level guard is provable with the noop dataplane.

**Behavior to preserve:** (unless the user explicitly said to change it)
- The pool-bound path and its tests (`TestTwoPeersSameRequestedAddressDistinctLeases`, `TestNoChildSAWithoutLeaseOnBoundPeer`, `TestRFC3948ZeInitiatedRekeyNamesOnlyTheLease`)
- Transport-mode overlap refusal (RFC 3948 Section 5.2) in `policyOwners.claim`, and the identical-key refusal for every mode
- The unconfigured-peer default of `narrowSelectors` (an empty policy accepts the proposal): a single unbound peer, or several whose selectors are disjoint, are answered exactly as today
- Operator-configured overlapping selectors that each peer negotiates whole (the RFC 4301 Section 4.4.1 ordering by `policy-priority`)
- Route-based peers with distinct if_id all negotiating 0.0.0.0/0 (no packet can match two of them)
- A Child SA rekey or IKE SA re-authentication of the SAME peer, which overlaps its own live selectors by construction
- Every existing interop scenario (`responder-psk`, `responder-ike-rekey`, the `real-nat-*` and EAP responder scenarios each run one peer)

**Behavior to change:** (only what the user asked for)
- A tunnel-mode Child SA whose negotiated selector pair overlaps a live Child SA of ANOTHER peer, where at least one of the two pairs is not operator-fixed, is no longer created: the responder drops the conflicting pairs from its answer and answers TS_UNACCEPTABLE when none remain; an install that loses a race is refused before any kernel call.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire: the peer's TSi/TSr payloads in IKE_AUTH (direct or EAP final) or CREATE_CHILD_SA (new Child SA or Child rekey), decrypted and parsed by the owner `PeerSession` goroutine into `wire.PayloadTS`.
- Format at entry: RFC 7296 Section 3.13.1 selector entries, converted by `wireToSelectors` to `tsSelector` prefixes.

### Transformation Path
1. `narrowChildSelectors` (ts_narrow.go): NAT substitution (transport), `substituteVirtualIP` (pool-bound), `narrowSelectors` against configured policy, transport single-address filter.
2. NEW: cross-peer filter. For a tunnel-mode exchange, each narrowed pair is classified operator-fixed or peer-chosen against `sa.PeerCfg.TrafficSelectors`, and dropped when the registry holds an overlapping pair of another owner that is not operator-fixed on both sides. No pair left: `errTSUnacceptable` with the conflict named. The filter is read-only.
3. `closedSubset`, rekey floor check, record `sa.NegotiatedPairs` (unchanged).
4. Answer payloads built; responder calls `createFirstChildSA` / `installChildTolerant` before sending.
5. NEW: `installChildSA` first performs the registry's atomic check-and-record for a tunnel-mode child (owner, outbound pairs, fixed flags, if_id). A conflict returns an error wrapping `errTSUnacceptable` before any dataplane call; a later kernel failure in the same call releases the record.
6. Answer sent (TS_UNACCEPTABLE via `notifyForRefusal` when step 2 or 5 refused).
7. NEW: every Child SA removal (`removeChildSAExcept`, and any direct path that ends a Child SA) releases its record.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer session goroutine ↔ other peer sessions | the registry, one mutex, owned by `SATable` and shared by pointer like `virtualIPState` | No |
| Engine ↔ dataplane | the engine calls an exported overlap predicate over `SPParams`; no new Dataplane interface method, no backend change | No |
| Engine ↔ wire | TS_UNACCEPTABLE notify through the existing `notifyForRefusal` | No |

### Integration Points
- `narrowChildSelectors` - gains the filter step; signature gains access to the registry (through the SA or PeerSession, implementer's choice, one pointer from `SATable`).
- `installChildSA` / `removeChildSAExcept` - record and release.
- `dataplane.selectorsOverlap` / `policyKey` - wrapped by one exported function comparing two `SPParams`.
- `childPairPolicy` - builds the `SPParams` the registry stores and compares (outbound direction).
- `notifyForRefusal` - unchanged; maps the wrapped `errTSUnacceptable`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the filter sits in `narrowChildSelectors`, the single responder narrowing entry; the backstop sits in `installChildSA`, the single install funnel |
| No unintended coupling (components stay isolated) | Yes | engine-internal registry; the only new dataplane surface is a pure function in the ungated `ike/dataplane` package the engine already imports |
| No duplicated functionality (extends existing, does not recreate) | Yes | overlap is the dataplane's existing `selectorsOverlap`, exported; registry pattern copies `virtualIPState`. `policyOwners` keeps the kernel-level transport check; the new registry holds tunnel-mode negotiation state the backends do not all track (VPP and noop run no `policyOwners`) |
| Zero-copy preserved where applicable (refs, not copies) | Yes | the registry stores the `SPParams` already built per pair; no wire buffer is retained (`tsSelector` nets are detached copies, per `firstSelectorAddress`'s rule) |
| Registration over hardcoding, outbound | Yes | no command, family or handler added |
| Registration over hardcoding, inbound | Yes | searched: `notifyForRefusal` (maps by `errors.Is`, no list to extend), `checkers.go` scenario map (the interop runner's existing registry; one row is how every scenario registers), no switch learns a peer name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every responder path installs the Child SA before sending its answer, so an install-time refusal still answers TS_UNACCEPTABLE | `responder.go` line 987-991 (IKE_AUTH, EAP final shares it), `create_child.go` line 441 (`respondNewChild`), `rekey.go` line 831 (`respondChildRekey` installs, then builds the response) | a responder path answers first and then fails to install, leaving the peer with a Child SA Ze lacks | read at design (2026-10-26) on all three; `TestInnerSelectorInstallBackstopRefusesSecond` re-proves it on IKE_AUTH and CREATE_CHILD_SA | validated (source read) |
| A-2 | `ChildSA.Owner` is the configured peer name and is equal for the old and new SA of a rekey and of an IKE re-authentication | `childPairPolicy` sets `Owner: child.Owner`; `policyOwners.claim` relies on the same equality for rekey upserts | same-peer rekey refused | `TestInnerSelectorSamePeerRekeyAllowed` | validated (2026-10-26): `TestInnerSelectorSamePeerRekeyAllowed` green in the engine package run before 286bd6e63f |
| A-3 | Removal of every Child SA funnels through `removeChildSAExcept` (or a path the implementer finds with `gopls references`) | `removeChildSA` delegates to it; the outgoing/incoming split is called from rekey | a leaked record locks other peers out of an address until restart | `gopls references` on `removeChildSAOutgoing`/`removeChildSAIncoming`; `TestInnerSelectorRegistryEmptyAfterTeardown` covering delete, rekey, IKE SA death, peer reload | validated (2026-10-26): `gopls references` shows `removeChildSAOutgoing` and `removeChildSAIncoming` called only from `removeChildSAExcept`, whose callers are `removeChildSA` (`child.go`), two in `delete.go`, three in `established.go`, one in `inbound.go` (the child rekey response) and one in `rekey.go`; every one reaches the release; `TestInnerSelectorRegistryEmptyAfterTeardown` green |
| A-4 | A strongSwan initiator keeps its IKE SA and reports the child failure on TS_UNACCEPTABLE in IKE_AUTH | strongSwan `child_create.c`: "received %N notify, no CHILD_SA built", "an error in CHILD_SA creation is not critical" | interop checker cannot assert IKE SA ESTABLISHED for the refused client | interop scenario `unbound-inner-address-uniqueness` | validated (2026-10-26): the green interop run shows strongSwan logging TS_UNACCEPTABLE for `ze-b` and keeping IKE SA `ze-b` ESTABLISHED with no Child SA (`tmp/session/2026-10-25-f3ae7c00-09af-4420-914f-39f2f7169980/scratch/interop-green.log`) |
| A-5 | No existing interop or functional test runs two peers whose live tunnel selectors overlap with a peer-chosen side | survey of `test/interop-ipsec/scenarios/*/ze.conf` (each responder scenario has one peer) | an existing test goes red | the scoped ipsec functional suite and the IKE engine package tests | partly validated (2026-10-26): the IKE engine and dataplane package tests are green with the guard, and the two new `.ci` tests pass; the whole `./le test ipsec -a` suite and the other interop scenarios were not run here (the Linux host runs scoped tests only), so they are owed by the main thread or another host |
| A-6 | Comparing outbound-direction pairs suffices: the inbound policy is the mirror, so two pairs overlap outbound exactly when they overlap inbound | `childPolicies` builds the inbound policy by swapping Src/Dst of the same pair | a conflict seen only inbound is missed | unit test asserting symmetric verdicts | validated (2026-10-26): `TestInnerSelectorOverlapSymmetric` green |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Record leaked on some removal path: a peer that went away keeps an address, and another peer is refused until restart | `TestInnerSelectorRegistryEmptyAfterTeardown` fails; interop re-initiate of client b after a's delete is refused | release in the removal funnel before its early returns; registry size exposed in the test hook; the refusal log names the holder so an operator sees the stale owner |
| R-2 | Race: two peers pass the narrowing filter at once | both answered with Child SAs | atomic check-and-record in `installChildSA` under the registry mutex; `TestInnerSelectorInstallBackstopRefusesSecond` |
| R-3 | Same-peer make-before-break (Child rekey, IKE re-auth, re-initiation) refused | rekey tests red | same owner always exempt; explicit tests |
| R-4 | First-come lockout: a hostile but authenticated unbound peer proposing 0.0.0.0/0 first blocks every overlapping peer | refusal logs name that peer | documented in the guide with the fix (configure the peer's remote prefixes or bind a pool); the unconfigured default is the operator's choice and two such peers already collide today on the identical key |
| R-5 | Operator-fixed classification wrong for a configured selector with ports or a protocol | a whole-negotiated configured pair refused | classification compares the whole installed pair (both prefixes, protocol, both ports) exactly against a whole configured pair through `samePolicyPair`; `TestInnerSelectorConfiguredOverlapNegotiatedWholeAllowed` with a protocol/port selector |
| R-6 | Initiator-role install refused by the backstop leaves the remote responder holding a Child SA | initiator install error | the existing initiator install-failure path in `applyChildCreateResponse` / `initiatorFirstChildSA` handles it as any install failure; `TestInnerSelectorInitiatorInstallRefusedInstallsNothing` asserts no dataplane call and the logged refusal |
| R-7 | Registry lock held across a dataplane call | slow installs serialize every peer | check-and-record and release are pure map operations; no dataplane call under the registry mutex |
| R-8 | A pool-bound client refused because an unbound peer already holds its leased address | pool client gets TS_UNACCEPTABLE | that is the 5.1 prevention working (first come); the Warn line names both peers and the fix (keep pool ranges disjoint from other peers' remote prefixes) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a legitimate second tunnel is refused (TS_UNACCEPTABLE, IKE SA kept, Warn line), or the conflict is still admitted |
| How is it reverted? | single commit revert; no config migration |
| Who else touches this path? | `ts_narrow.go` and `child.go` are hot IKE files; check `./le spec status` for in-progress IKE specs before implementing |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH from a second unbound peer proposing an inner TSi a first peer holds | → | `narrowChildSelectors` cross-peer filter | `test/ipsec/ipsec-unbound-inner-address-unique.ci` |
| IKE_AUTH from two strongSwan clients behind two NATs, no pool, both 10.1.2.3 | → | filter + `installChildSA` record | interop `unbound-inner-address-uniqueness` |
| Two configured peers with overlapping remote prefixes negotiated whole | → | operator-fixed exemption | `test/ipsec/ipsec-overlapping-configured-selectors-kept.ci` |
| CREATE_CHILD_SA from a second peer | → | `respondNewChild` → filter | `TestRFC3948CreateChildSecondPeerSameInnerRefused` |
| Two exchanges racing past narrowing | → | `installChildSA` check-and-record | `TestInnerSelectorInstallBackstopRefusesSecond` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Unbound, unconfigured responder peers a and b; a holds a tunnel Child SA TSi 10.1.2.3/32 TSr 0.0.0.0/0; b proposes TSi 10.1.2.3/32 TSr 192.0.2.10/32 in IKE_AUTH | b is answered TS_UNACCEPTABLE, b's IKE SA is established with no Child SA, no policy naming 10.1.2.3 is installed for b, a's Child SA and policies are untouched |
| AC-2 | As AC-1 but b proposes TSi 10.1.2.4/32 | b is answered with its Child SA (positive polarity: distinct inner addresses both served) |
| AC-3 | As AC-1 but b proposes TSi 10.1.2.3/32 with TSr 198.51.100.0/24 while a holds TSr 192.0.2.10/32 | both served: no packet matches both pairs |
| AC-4 | Peers a and b each configured with remote prefix 10.0.0.0/8; a holds narrowed 10.1.2.3/32; b proposes 10.1.2.3/32 | b refused TS_UNACCEPTABLE (peer-chosen inside an operator overlap) |
| AC-5 | Peer a configured remote 10.1.0.0/16, peer b configured remote 10.1.2.0/24 (different `policy-priority`), both peers propose exactly their configured selectors | both served (operator-fixed on both sides); policies installed with each peer's priority |
| AC-6 | Peer a holds operator-fixed 10.1.0.0/16; unconfigured peer b proposes 10.1.5.0/24 | b refused (peer-chosen overlapping a fixed holder) |
| AC-7 | b proposes TSi {10.1.2.4/32, 10.1.2.3/32} (first entry free, second held by a) | b is answered with the first pair only (RFC 7296 Section 2.9 first-choice MUST) |
| AC-8 | Peer a rekeys its Child SA, or re-authenticates its IKE SA, with the same selectors | accepted; no conflict with itself |
| AC-9 | Route-based peers a and b with distinct if_id both negotiate 0.0.0.0/0 ↔ 0.0.0.0/0 | both served |
| AC-10 | Two peers pass narrowing for the same inner address before either installs | the second install is refused before any dataplane call, with an error wrapping `errTSUnacceptable`; on the responder the answer is TS_UNACCEPTABLE |
| AC-11 | a's Child SA is deleted (peer DELETE, IKE SA death, `clear`, peer reload, rekey retire); then b proposes 10.1.2.3/32 | b is served; the registry holds no record of the removed child |
| AC-12 | Second peer refused for a conflict | Ze logs one Warn line naming: the refused peer and its selector pair, the holding peer and its pair, that one packet could be sent into either tunnel (RFC 3948 Section 5.1), and the fix: configure disjoint remote prefixes (`set vpn ipsec site-to-site peer <name> traffic-selector <n> remote prefix <prefix>`) or bind the peer to a virtual-IP pool (`set vpn ipsec site-to-site peer <name> virtual-ip-pool <pool>`) |
| AC-13 | Transport-mode Child SAs | recorded in the registry (independent audit of 7d7658dae0: a tunnel around a transport-mode peer's address went unrefused); a tunnel overlapping another peer's transport-mode pair, and the reverse, is refused at narrowing and at install in either arrival order; two transport-mode pairs stay with `policyOwners.claim` (RFC 3948 Section 5.2) |
| AC-14 | Pool-bound peers | existing pool tests green; a lease equal to an unbound peer's live inner address is refused for the newcomer, whichever came second |
| AC-15 | `rfc/short/rfc3948.md` | RFC3948-5.1-1 loses `{partial}`, the `Support remaining` row no longer describes the unbound gap, and every new tagged test carries a discrimination record (`./le rfc check` green) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator configures two respond peers with no pool and no selectors; both clients behind NATs use 10.1.2.3 | strongSwan IKE_AUTH → Ze narrowing filter → TS_UNACCEPTABLE to the second; first keeps traffic | interop `unbound-inner-address-uniqueness` |
| 2 | Operator configures a hub with deliberately overlapping, priority-ordered remote prefixes for two spokes | both IKE_AUTH answered, both tunnels installed | `ipsec-overlapping-configured-selectors-kept.ci` |
| 3 | Operator reads why the second client has no tunnel | Warn line with both peers and the fix | `ipsec-unbound-inner-address-unique.ci` (stderr assertion) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC3948TwoUnboundPeersSameInnerAddressSecondRefused` | `internal/component/ike/engine/rfc3948_inner_selector_test.go` | AC-1, the judge's probe (TSr 0.0.0.0/0 vs 192.0.2.10/32); tagged `RFC requirement: RFC3948-5.1-1 negative` | |
| `TestRFC3948TwoUnboundPeersDistinctInnerAddressesServed` | same | AC-2; tagged `RFC3948-5.1-1 positive` | |
| `TestInnerSelectorDisjointLocalSidesNoConflict` | same | AC-3 | |
| `TestRFC3948ConfiguredOverlapNarrowedByPeerRefused` | same | AC-4; tagged `RFC3948-5.1-1 negative` | |
| `TestInnerSelectorConfiguredOverlapNegotiatedWholeAllowed` | same | AC-5, includes a selector with protocol 6 and a port (R-5) | |
| `TestInnerSelectorPeerChosenOverlapsFixedRefused` | same | AC-6 | |
| `TestInnerSelectorFirstChoiceKeptWhenLaterPairConflicts` | same | AC-7 | |
| `TestInnerSelectorSamePeerRekeyAllowed` | same | AC-8 (Child rekey through `respondChildRekey` with floor, and a second SA of the same owner) | |
| `TestInnerSelectorDistinctIfIDNoConflict` | same | AC-9 | |
| `TestInnerSelectorInstallBackstopRefusesSecond` | same | AC-10, recording dataplane receives no call for the refused child | |
| `TestRFC3948CreateChildSecondPeerSameInnerRefused` | same | AC-1 over CREATE_CHILD_SA (`respondNewChild`), notify TS_UNACCEPTABLE | |
| `TestInnerSelectorRegistryEmptyAfterTeardown` | same | AC-11 over delete, rekey retire, IKE SA death, failed kernel install | |
| `TestInnerSelectorRefusalNamesBothPeersAndFix` | same | AC-12 error text | |
| `TestInnerSelectorTransportModeRecordedAndLeftToDataplane` (`rfc3948_inner_selector_test.go`; `TestInnerSelectorTransportModeNotRecorded` renamed in place, body unchanged, because its old name stated the defect), `TestRFC3948TunnelAroundTransportPeerRefused`, `TestRFC3948TunnelAroundTransportPeerInstallBackstop` (`rfc3948_inner_selector_transport_test.go`) | same | AC-13 | records: `innerSelectorGuard.claim` transport skip restored, `bothTransport` answering true, both red |
| `TestRFC3948LeaseVersusUnboundPeerFirstComeWins` | same | AC-14 | |
| `TestInnerSelectorInitiatorInstallRefusedInstallsNothing` | same | R-6 | |
| `TestInnerSelectorOverlapSymmetric` | same | A-6 | |
| `TestInnerSelectorPairsOfNoSelectorsMatchesPolicy` | same | NOTE 1: a Child SA with no negotiated pairs is recorded in `childPolicies`' orientation | red with `innerSelectorPairsOf`'s fallback pair swapped (overlay `scratch/r2-mut/pairs.json`): "recorded policy 10.1.2.3/32 -> 192.0.2.0/24, installed 192.0.2.0/24 -> 10.1.2.3/32" (`scratch/r2-red-TestInnerSelectorPairsOfNoSelectorsMatchesPolicy.log`); green `scratch/r2-green-four.log` |
| `TestInnerSelectorDeclinedTransportRequestFiltered` | same | NOTE 2: a declined transport request installs a tunnel, so the narrowing filter runs | red with `narrowChildSelectors`' gate back on `transportSelectorsInPlay` (`scratch/r2-mut/transport.json`): "peer b's declined transport request at narrowing: served, want TS_UNACCEPTABLE" (`scratch/r2-red-TestInnerSelectorDeclinedTransportRequestFiltered.log`); green `scratch/r2-green-four.log` |
| `TestInnerSelectorNilDataplaneRecorded` | same | NOTE 3: with no dataplane the record is still taken | red with the claim dropped in `createFirstChildSA` and `installChildTolerant` nil-dp paths (`scratch/r2-mut/nildp.json`): "registry holds 0 Child SAs with no dataplane, want 1" (`scratch/r2-red-TestInnerSelectorNilDataplaneRecorded.log`); green `scratch/r2-green-four.log` |
| `TestInnerSelectorInitiatorRefusalKeepsIKESA` | same | NOTE 4: an initiator-side refusal keeps the IKE SA childless | red with `runEstablished`'s `errInnerSelectorConflict` case disabled (`scratch/r2-mut/initiator.json`): `runEstablished` returned the conflict, tearing the IKE SA down (`scratch/r2-red-TestInnerSelectorInitiatorRefusalKeepsIKESA.log`); green `scratch/r2-green-four.log` |
| `TestPolicySelectorsOverlapExported` | `internal/component/ike/dataplane/policy_owner_test.go` | the exported wrapper agrees with `selectorsOverlap` on identical, overlapping, disjoint, different-if_id and different-protocol inputs | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefix length in overlap (IPv4) | 0-32 | /32 vs /0 overlap; /32 vs adjacent /32 do not | N/A | N/A |
| prefix length in overlap (IPv6) | 0-128 | /128 vs ::/0 overlap | N/A | N/A |
| port match | 0-65535 | port 65535 vs any overlaps; 179 vs 180 do not | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-unbound-inner-address-unique` | `test/ipsec/ipsec-unbound-inner-address-unique.ci` | hub (127.0.0.1, two unconfigured `respond` peers, noop dataplane) and two Ze initiator spokes (127.0.0.2, 127.0.0.3) each configured local 10.1.2.3/32 remote 0.0.0.0/0; spoke-a first, then spoke-b. Assert: `show vpn ipsec sa` on the hub lists spoke-a's Child SA with 10.1.2.3/32 and spoke-b's IKE SA with no Child SA; hub stderr carries the AC-12 Warn line; spoke-b stderr names TS_UNACCEPTABLE | green in `./le test functional ipsec` after f87c1883ac and 3c8b9e42de (27/27 pass, 7 net-admin skips, `scratch/r2-func-ipsec.log`); red `scratch/ci-unique-red.log` |
| `ipsec-overlapping-configured-selectors-kept` | `test/ipsec/ipsec-overlapping-configured-selectors-kept.ci` | hub with spoke-a configured remote 10.1.0.0/16 (`policy-priority 2000`) and spoke-b remote 10.1.2.0/24 (`policy-priority 1500`), spokes propose exactly those; both Child SAs listed | green under the equal-priority refusal (distinct priorities 2000/1500 keep the exemption), `scratch/r2-func-ipsec.log`; red `scratch/ci-overlap-red.log`. No existing ipsec `.ci` config relied on an equal-priority overlap: the suite had no new red |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `unbound-inner-address-uniqueness` | `test/interop-ipsec/scenarios/unbound-inner-address-uniqueness/` (nat.conf, swanctl.conf, ze.conf) and checker `checkUnboundInnerAddressUniqueness` in `internal/le/interoplab/ipsec/unbound_inner.go`, registered in `checkers.go` | strongSwan (two connections ze-a from 172.28.0.3 and ze-b from 172.28.0.8, both `local_ts = 10.1.2.3/32`; ze-a `remote_ts = 10.20.0.1/32`, ze-b `remote_ts = 10.20.0.0/24` (aa18d486d6: overlapping but not identical, so the dataplane's exact-key `policyOwners.claim` cannot refuse it and only the guard can), no `vips`), NAT as `virtual-ip-uniqueness` | after ze-a: child-a INSTALLED and Ze holds one out policy to 10.1.2.3/32 with a's public tunnel endpoint; straight after ze-b initiates, Ze still holds that one out policy (judged before the notify); then swanctl reports TS_UNACCEPTABLE / no CHILD_SA, IKE SA ze-b ESTABLISHED, Ze still holds exactly one policy pair naming 10.1.2.3/32 and it is a's; a ping from 10.1.2.3 via child-a to 10.20.0.1 still succeeds. Red proof: revert the guard, rebuild the Ze image, rerun: two policies naming 10.1.2.3 and the checker fails; record the red | green `scratch/r1-interop-green.log`; red observed with the guard's conflict return replaced by `continue` (`scratch/r1-overlay.json`): "ze's kernel holds 2 outbound ESP policies to 10.1.2.3/32 (tunnels [172.28.0.9 172.28.0.7]), want exactly one" (`scratch/r1-interop-red.log`) |

## Files to Modify
- `internal/component/ike/engine/ts_narrow.go` - cross-peer filter step in `narrowChildSelectors` (tunnel only), with the RFC 3948 Section 5.1 and RFC 7296 Section 2.9 quotes; the `narrowSelectors` doc comment on the empty policy gains a pointer to the filter (its own behavior is unchanged)
- `internal/component/ike/engine/child.go` - check-and-record in `installChildSA`, release in `removeChildSAExcept` (before its early returns) and on a failed install
- `internal/component/ike/engine/table.go` - `SATable` carries the registry
- `internal/component/ike/engine/reconcile.go` - `PeerSession` receives it at session start (beside `virtualIP`)
- the four `narrowChildSelectors` callers (`responder.go`, `responder_eap.go`, `create_child.go`, `rekey.go`) only if the registry is passed as an argument rather than reached through the SA
- `internal/component/ike/dataplane/policy_owner.go` - export one predicate over two `SPParams` wrapping `policyKey` + `selectorsOverlap`; update the `claim` comment ("Tunnel mode is not compared ...") to name the engine guard for peer-chosen tunnel selectors
- `internal/le/interoplab/ipsec/checkers.go` - register the scenario
- `rfc/short/rfc3948.md` - drop `{partial}` on RFC3948-5.1-1; rewrite the `Support remaining` row
- `docs/guide/ipsec.md`, `docs/architecture/ike/ipsec-14-responder.md`, `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/ike/ipsec-7-ikev2-engine.md` - see Documentation checklist
- `plan/learned/035-ike-virtual-ip-assignment.md` - pointer to the closure

## Files to Create
- `internal/component/ike/engine/inner_selectors.go` - the registry (`// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md`, `// RFC: rfc/short/rfc3948.md`)
- `internal/component/ike/engine/rfc3948_inner_selector_test.go`
- `test/ipsec/ipsec-unbound-inner-address-unique.ci`, `test/ipsec/ipsec-overlapping-configured-selectors-kept.ci`
- `test/interop-ipsec/scenarios/unbound-inner-address-uniqueness/{nat.conf,swanctl.conf,ze.conf}`
- `internal/le/interoplab/ipsec/unbound_inner.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf; the fix names existing `traffic-selector`, `virtual-ip-pool`, `policy-priority` |
| YANG validation constraints | No | refusal is runtime (it depends on which peer is live) |
| YANG custom validators | No | as above |
| CLI commands/flags | No | none added |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no config change |
| Functional test for new RPC/API | N-A | no RPC; behavior proven by the two `.ci` tests |
| Pipe completeness | N-A | no command output changed |
| Env var registration | No | none added |
| Doctor check for runtime dependencies | N-A | no runtime dependency added |
| Prometheus counters/metrics | No | the refusal is visible as the existing TS_UNACCEPTABLE notify and a Warn line; no counter is required by any AC |
| BGP family surface (new SAFI / capability / attribute) | N-A | IKE |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | RFC 3948 support level via `rfc/short/rfc3948.md` (`docs/features/rfc-status.md` regenerates from it) |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` lines 284-289: replace the disclosure with the rule (one inner selector per live tunnel across peers, operator-configured overlaps kept), the refusal and its fix |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc3948.md`, `plan/learned/035-ike-virtual-ip-assignment.md` |
| 10 | Test infrastructure changed? | Yes | new interop scenario: `docs/architecture/testing/interop.md` scenario list if it enumerates ipsec scenarios |
| 11 | Affects daemon comparison? | Yes | strongSwan comparison: strongSwan prevents 5.1 by its `dynamic` default, Ze by refusing the conflict; add to `docs/guide/ipsec.md` beside the pool section |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-14-responder.md`, `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` (line 157 sentence), `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` ("Policy ownership"), `docs/architecture/ike/ipsec-7-ikev2-engine.md` (new registry beside the lease registry) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/release/fixes/spec-ike-unbound-peer-inner-address.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` remote-access and site-to-site examples: check none shows two peers that would now be refused |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- registry type and `SATable` / `PeerSession` plumbing; exported dataplane overlap predicate with `TestPolicySelectorsOverlapExported`. Write `ipsec-unbound-inner-address-unique.ci` and see it red (both spokes get Child SAs).
2. **Phase: Install backstop** -- check-and-record in `installChildSA`, release in the removal funnel; tests AC-10, AC-11, R-6 red then green. Resolve A-1 and A-3 with `gopls references` first.
3. **Phase: Narrowing filter** -- classification (operator-fixed) and the drop step in `narrowChildSelectors`, error text (AC-12); tests AC-1 to AC-9, AC-13, AC-14 red then green.
4. **Phase: Functional and interop** -- both `.ci` green; interop scenario green, then the red proof by revert and image rebuild, recorded.
5. **Phase: RFC ledger and docs** -- discrimination records (`./le rfc discriminate-record`) for each tagged test, `rfc/short/rfc3948.md`, every page in the Documentation checklist in the same phase as the code it describes.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the filter runs before `closedSubset` and after the transport filter; the first choice still leads; same-owner always exempt |
| Correctness | the record is taken before any dataplane call and released on every removal and failed install; no dataplane call under the registry mutex |
| Correctness | operator-fixed compares the whole installed pair (`childPolicyPair`: both prefixes, protocol, both ports) exactly against the whole configured pairs of `sa.PeerCfg.TrafficSelectors` (`configuredPairs`) and, for transport mode, the configured host pair (`configuredHostPair`), through `samePolicyPair` |
| Preservation | transport mode untouched; `claim` behavior byte-for-byte unchanged |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| registry file | `ls internal/component/ike/engine/inner_selectors.go` |
| tagged tests with records | `./le rfc check` green; `rfc/discrimination/rfc3948.json` lists the new units |
| interop scenario | `ls test/interop-ipsec/scenarios/unbound-inner-address-uniqueness`; checker row in `checkers.go` |
| `{partial}` gone | `grep -n 'RFC3948-5.1-1' rfc/short/rfc3948.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Peer-proposed traffic selectors are untrusted input that can capture another peer's inner address: every tunnel pair a peer chose is compared against every other owner's live pairs |
| Fail closed | an unparsable or unexpected selector in the comparison counts as overlapping (as `prefixesOverlap` already does) |
| Denial of service | first-come lockout is documented (R-4); the Warn line names the holder |

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

## Planned Goal Validation

Carried into the closure template's Goal Validation table by `/ze-close`.

| Goal (from Task) | Evidence that will prove it |
|------------------|-----------------------------|
| No two peers hold a tunnel Child SA or kernel policy whose selectors overlap unless the operator configured both at different `policy-priority` values | interop `unbound-inner-address-uniqueness`: green, Ze's `ip xfrm policy` holds exactly one policy pair naming 10.1.2.3/32 after both strongSwan clients initiate and strongSwan logs TS_UNACCEPTABLE for `ze-b` (`scratch/r1-interop-green.log`, after aa18d486d6). Red, with the guard's conflict return replaced by `continue` through `scratch/r1-overlay.json` and the Ze image rebuilt: "ze's kernel holds 2 outbound ESP policies to 10.1.2.3/32 (tunnels [172.28.0.9 172.28.0.7]), want exactly one" (`scratch/r1-interop-red.log`), the RFC 3948 Section 5.1 confusion itself. (The earlier run, `scratch/interop-red.log`, had both clients propose the identical pair, so `policyOwners.claim` refused the second by exact key and the red was only the wrong notify, NO_PROPOSAL_CHOSEN; aa18d486d6 gave `ze-b` `remote_ts = 10.20.0.0/24` so only the guard can refuse it.) `.ci`: reds `scratch/ci-unique-red.log`, `scratch/ci-overlap-red.log`; greens `scratch/ci-*-promoted.log`, and the whole `./le test functional ipsec` suite green after the equal-priority refusal (27/27, 7 net-admin skips, `scratch/r2-func-ipsec.log`). Pool-bound peers sharing one configured remote prefix at default priority: interop `virtual-ip-uniqueness` green after the refusal (`scratch/r2-vip-uniqueness.log`) (scratch = `tmp/session/2026-10-25-f3ae7c00-09af-4420-914f-39f2f7169980/scratch`). Equal-priority configured overlap: `TestRFC3948ConfiguredOverlapEqualPriorityRefused`, mutant red recorded |
| Operator-configured overlaps keep working | `ipsec-overlapping-configured-selectors-kept.ci`: both Child SAs listed; `TestInnerSelectorConfiguredOverlapNegotiatedWholeAllowed` |
| The first client keeps its traffic (the SGW is not confused) | interop checker: ping from 10.1.2.3 via child-a to 10.20.0.1 succeeds after client b was refused |
| `RFC3948-5.1-1` loses `{partial}` with both polarities proven over the unbound path | `./le rfc check` green with discrimination records for the positive and negative tagged units; `grep` of the row shows no `{partial}` |

## Design Insights

- RFC 3948 Section 5.1's hazard is per packet: two SAs a packet could be sent into. Comparing whole selector pairs (both address sides, protocol, ports, if_id) refuses exactly that and no more; comparing the remote half alone would refuse disjoint-server cases no packet can confuse (AC-3).
- strongSwan meets 5.1 by never accepting a peer-chosen address it did not observe or lease (`dynamic` default), and does not refuse overlaps at all (kernel policies shared or ordered by prefix length). Ze keeps its documented unconfigured default and refuses only the conflict, which serves strictly more clients while still preventing the confusion.
- The guard must be engine-level: the obligation is the responder's answer (TS_UNACCEPTABLE), and only the xfrm backend runs `policyOwners`.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-added-to-one-half-of-a-pair.md` | 2026-10-26 | IKE tunnel inner address: unbound peer keeps any proposed TSi, `claim` checks tunnel overlap never |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Option A (refuse the conflict), not B | B (`%dynamic` narrowing of an unconfigured responder's TSi to the observed address); A+B | B alone is insufficient: it does not cover a peer narrowing inside overlapping CONFIGURED remote prefixes (AC-4), nor an unbound peer's outer /32 inside another peer's configured prefix. B also breaks Ze's documented load-bearing default (`narrowSelectors`: an empty policy accepts the proposal) for unconfigured site-to-site and route-based responders, and behind NAT the observed address is not the tunnel client's inner source, so B answers every NATed road warrior without a pool TS_UNACCEPTABLE (strongSwan `ike_sa_get_dynamic_hosts` + `select_` do exactly that). A prevents 5.1 in every case and follows the remedy RFC 3948 Section 5.2 names ("disallowing conflicting connections"), already used by `claim` for transport mode. A+B adds B's breakage for no extra prevention |
| Engine registry, not `policyOwners.claim` | extend `claim` to tunnel overlap | `claim` runs in the xfrm backend only (VPP, noop do not track owners), knows nothing of operator-fixed vs peer-chosen, and cannot drop single pairs at narrowing (RFC 7296 Section 2.9 first-choice MUST) |
| Filter at narrowing AND check-and-record at install | install check only; narrowing check only | install-only refuses a whole proposal when a later pair conflicts (breaks the first-choice MUST); narrowing-only races between peer goroutines. One predicate, two call sites |
| Operator-fixed = the whole installed pair equals a whole configured pair (b99c6503a4, `classifyInnerPair`) | "peer has any configured selector"; remote half equals a configured remote selector (the rule before b99c6503a4) | the pair is judged as the policy the dataplane installs, whose protocol is the intersection of both halves' and whose local port comes from the local half (`childPolicyPair`), so a peer that narrows only Ze's local half still narrows the policy the priorities would order. Two ways to be operator-fixed, both through `samePolicyPair` (both prefixes, protocol, both ports): equal to a configured traffic selector as its installed pair (`configuredPairs`), or, for a transport-mode pair, equal to the host pair between the configured `local-address` and `remote-address` (each a /32 or /128 host prefix), protocol 0 and both ports any (`configuredHostPair`, `isConfiguredHost`; nil, so never fixed, when either address is a DNS hostname rather than a literal). Configured-but-narrowed, on either half, is the 5.1 case (AC-4) |
| Both-fixed overlaps allowed only at distinct `policy-priority` values (`operatorOrdered`, f87c1883ac) | allow every both-fixed overlap regardless of priority; refuse at commit time | the goal says "unless the operator configured both at different `policy-priority` values": RFC 4301 Section 4.4.1 gives the ordering to the operator, and only distinct priorities express it. At equal priority the kernel orders by install time, so a packet's tunnel depends on which peer came up first, the RFC 3948 Section 5.1 confusion; the newcomer is refused with TS_UNACCEPTABLE and a fix line naming `policy-priority` (`TestRFC3948ConfiguredOverlapEqualPriorityRefused`). A commit-time refusal was rejected: two peers sharing a wide configured prefix that narrow to disjoint pairs are legitimate, and only the install-time pair comparison tells the two cases apart. See Known Limitations |
| Same owner exempt | none | rekey and IKE re-authentication overlap themselves by construction |
| Record transport too, compare only tunnel-versus-any (superseded "tunnel only in the registry" after the audit of 7d7658dae0) | tunnel only | `policyOwners.claim` compares transport only against transport, so a tunnel a peer chose around a transport-mode peer's address went unrefused (RFC 3948 Section 5.1). Two transport-mode pairs still stay with `claim` (Section 5.2), so no overlap is refused twice |
| Pool-bound children recorded | exempt them | an unbound peer must not take a leased address, nor a lease an unbound peer's live address |
| TS_UNACCEPTABLE | NO_PROPOSAL_CHOSEN, TEMPORARY_FAILURE | RFC 7296 Section 2.9: the responder's policy does not accept that part of the proposal |

## Known Limitations
- Two operator-fixed overlapping selectors at EQUAL `policy-priority` are not both installed: the second peer to install is refused with TS_UNACCEPTABLE, and its Warn line tells the operator to give the peers distinct `policy-priority` values. The refusal happens at negotiation, not at commit, so a config that can produce it commits cleanly and shows the conflict only once both peers come up.
- An edit of `policy-priority` or of a peer's selectors cannot leave an unordered overlap live (review NOTE 6, refuted with no code change). Both are fields of `ipsec.SiteToSitePeer`, so `peerConfigChanged` (`internal/component/ike/engine/reconcile.go`, `peerCfg.Equal`) reports the peer changed and the reconcile restarts its session: its Child SA is removed, its inner-selector record is released once that policy removal succeeds (a failed removal keeps the record, `releaseInnerRecord`), and its new Child SA is claimed against the registry under the new priority and selectors. Whichever peer re-establishes second at an equal priority is refused then, exactly as at first start.
- First-come wins: an authenticated unbound peer proposing a wide TSi first excludes later overlapping peers until it goes away (R-4). The fix for an operator is configuring each peer's remote prefixes or a pool.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

| Site | Quote |
|------|-------|
| filter in `narrowChildSelectors`, check-and-record in the registry | RFC 3948 Section 5.1: "Implementors MUST devise ways of preventing this from occurring." |
| the refusal | RFC 7296 Section 2.9: "If the responder's policy does not allow it to accept any part of the proposed Traffic Selectors, it responds with a TS_UNACCEPTABLE Notify message." |
| dropping a later pair, keeping the first | RFC 7296 Section 2.9: "If the responder's policy allows it to accept the first selector of TSi and TSr, then the responder MUST narrow the Traffic Selectors to a subset that includes the initiator's first choices." |
| operator-fixed exemption | RFC 4301 Section 4.4.1: "Thus, a user or administrator MUST be able to order the entries to express a desired access control policy." |
| callers (`installChildSA` call sites added) | `// RFC 3948 Section 5.1` reference only |

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
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
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
- [ ] **Commit B:** `remove plan/release/fixes/spec-ike-unbound-peer-inner-address.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- Engine-wide inner-selector registry (`innerSelectorRegistry`, `innerSelectorGuard`, `internal/component/ike/engine/inner_selectors.go`, 286bd6e63f): every Child SA, tunnel and transport mode, pool-bound or not, is recorded by owner with its installed policy pairs.
- Cross-peer filter at narrowing (`dropForeignInnerOverlaps`, called from `narrowChildSelectors` in `ts_narrow.go`): a peer-chosen pair overlapping another owner's live pair is dropped, the first choice kept (RFC 7296 Section 2.9), and an empty answer is TS_UNACCEPTABLE.
- Check-and-record at install (`installChildSA`, `child.go`, through `innerGuard.claim`, release through `innerGuard.release`): closes the race between peers that pass narrowing together; release on every removal and failed install, kept while a policy removal is stranded (`retryStranded`, 3c8b9e42de, dd20e0aa1a, 000e6fdfea, dc7de8e76f).
- Operator-ordered exemption: an overlap stands only when both pairs are exactly operator-configured (`classifyInnerPair`, `configuredPairs`, `configuredHostPair`, `samePolicyPair`) AND their `policy-priority` values differ (`operatorOrdered`, f87c1883ac, b99c6503a4, fcf2e39dc8, 4287e22441).
- Tunnel against transport-mode peers refused in either order (bb90bdec23); two transport-mode pairs stay with `policyOwners.claim` (RFC 3948 Section 5.2).
- Exported dataplane predicate `PolicySelectorsOverlap` (`internal/component/ike/dataplane/policy_owner.go`).
- Interop scenario `unbound-inner-address-uniqueness` with checker `checkUnboundInnerAddressUniqueness` (91ff2f88f0, aa18d486d6); two `.ci` tests (9ea5de8246).
- `rfc/short/rfc3948.md`: `RFC3948-5.1-1` carries no `{partial}`; discrimination records for every tagged unit (b80a2eda36, 635aa08b78, 0274a6774d, 349d6d6f23); audit verdict enforced (1a5f53cd30, rejudged 58a09337f8).

### Bugs Found/Fixed
- A failed policy removal released the record while the kernel still held the policy: `TestRFC3948FailedPolicyRemovalKeepsRecord` and the stranded-retry tests (3c8b9e42de, dd20e0aa1a).
- VPP stranded retry lost the SAD id once the SA was removed (dc7de8e76f, `TestVPPRemovalWhileBothSAsLive` 30bf686318).
- Equal-priority configured overlap was admitted although the kernel then orders by install time: `TestRFC3948ConfiguredOverlapEqualPriorityRefused` (f87c1883ac).
- A tunnel around a transport-mode peer's address went unrefused: `TestRFC3948TunnelAroundTransportPeerRefused`, `TestRFC3948TunnelAroundTransportPeerInstallBackstop` (bb90bdec23).
- Operator-fixed was judged on the remote half, so a peer narrowing Ze's local half or one port passed as configured: `TestRFC3948OrderedTransportLocalHalfNarrowedRefused`, `TestRFC3948ConfiguredSelectorNarrowedOnLocalPortRefused`, `TestRFC3948ConfiguredSelectorNarrowedOnLocalPrefixRefused` (b99c6503a4, fcf2e39dc8, 690d38f274).

### Documentation Updates
- `docs/guide/ipsec.md`, `docs/architecture/ike/ipsec-14-responder.md`, `ipsec-7-ikev2-engine.md`, `ipsec-8-ikev2-child-xfrm.md`, `rfcgate-1b-rfc7296-pilot.md` each carry an anchor on `inner_selectors.go` / `dropForeignInnerOverlaps` / `innerSelectorRegistry` (`git grep -c` 1, 2, 3, 2, 1); `docs/architecture/testing/interop.md:1767` describes the scenario.
- `./le doc check all` (2026-10-27, `scratch/close-doc-check.log`): exit 1 on three source anchors this spec did not write: `core-design.md:156` (`commandWordsMax`, file modified by another session), `ipsec-9-ikev2-eap-nat.md:292` (`Available`, removed by c8d3e41036 of `spec-ike-virtual-ip-assignment`), `ci-workflows.md:126` (`workflowNestedLe`). Every other step green.

### Deviations from Plan
- Transport-mode Child SAs are recorded (planned: tunnel only), after the audit of 7d7658dae0 (Key Design Decisions).
- Both-fixed overlaps need distinct `policy-priority` values (planned: any both-fixed overlap kept), f87c1883ac.
- Operator-fixed compares the whole installed pair (planned: remote half), b99c6503a4.
- The install backstop gained the stranded-policy retry (`innerGuard.retryStranded`, `strand`), which the plan did not foresee: a record is kept while a failed removal leaves the policy installed.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | the plan recorded tunnel-mode Child SAs only | a tunnel around a transport-mode peer's address is the same Section 5.1 confusion, and `claim` compares transport only with transport | independent audit 7d7658dae0 | fixed in bb90bdec23 |
| approach | operator-fixed judged on the remote half | the kernel installs the intersection of both halves, so narrowing the local half or a port narrows the policy | review rounds 6 and 7 | fixed in fcf2e39dc8, b99c6503a4 |
| approach | first interop red proof relied on identical selectors | `policyOwners.claim` refused the second by exact key, so the red proved the wrong guard | reading `scratch/interop-red.log` (NO_PROPOSAL_CHOSEN) | aa18d486d6 made ze-b overlap without equality |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| No two peers hold overlapping tunnel Child SAs unless operator-ordered | Done | `inner_selectors.go` `dropForeignInnerOverlaps`, `conflictLocked`; `child.go` `installChildSA` | |
| `RFC3948-5.1-1` loses `{partial}`, both polarities proven | Done | `rfc/short/rfc3948.md:227`; `rfc/audit/rfc3948/RFC3948-5.1-1.json` enforced | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC3948TwoUnboundPeersSameInnerAddressSecondRefused`, `ipsec-unbound-inner-address-unique.ci`, interop `unbound-inner-address-uniqueness` | |
| AC-2 | Done | `TestRFC3948TwoUnboundPeersDistinctInnerAddressesServed` | |
| AC-3 | Done | `TestInnerSelectorDisjointLocalSidesNoConflict` | |
| AC-4 | Done | `TestRFC3948ConfiguredOverlapNarrowedByPeerRefused` | |
| AC-5 | Done | `TestInnerSelectorConfiguredOverlapNegotiatedWholeAllowed`, `ipsec-overlapping-configured-selectors-kept.ci` | distinct priorities |
| AC-6 | Done | `TestInnerSelectorPeerChosenOverlapsFixedRefused` | |
| AC-7 | Done | `TestInnerSelectorFirstChoiceKeptWhenLaterPairConflicts` | |
| AC-8 | Done | `TestInnerSelectorSamePeerRekeyAllowed`, `rfc3948_inner_selector_rekey_test.go` | |
| AC-9 | Done | `TestInnerSelectorDistinctIfIDNoConflict` | |
| AC-10 | Done | `TestInnerSelectorInstallBackstopRefusesSecond`, `TestRFC3948ConcurrentInstallsHoldOneInnerAddress` | |
| AC-11 | Done | `TestInnerSelectorRegistryEmptyAfterTeardown` | |
| AC-12 | Done | `TestInnerSelectorRefusalNamesBothPeersAndFix`; `.ci` stderr assertion | |
| AC-13 | Done | `TestInnerSelectorTransportModeRecordedAndLeftToDataplane`, `TestRFC3948TunnelAroundTransportPeerRefused`, `TestRFC3948TunnelAroundTransportPeerInstallBackstop` | |
| AC-14 | Done | `TestRFC3948LeaseVersusUnboundPeerFirstComeWins`; interop `virtual-ip-uniqueness` green (`scratch/r2-vip-uniqueness.log`) | |
| AC-15 | Done | `rfc/short/rfc3948.md:227` without `{partial}`; Support remaining row rewritten (line 21); records in `rfc/discrimination/rfc3948.json` | `./le rfc check` not run here, see Pre-Commit Verification |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| all 24 unit tests of the TDD plan plus `TestRFC3948ConfiguredOverlapEqualPriorityRefused` | Done | `internal/component/ike/engine/rfc3948_inner_selector*_test.go`, `internal/component/ike/dataplane/policy_owner_test.go` | each `func` found by `git grep` at HEAD 58a09337f8 |
| `ipsec-unbound-inner-address-unique`, `ipsec-overlapping-configured-selectors-kept` | Done | `test/ipsec/` | |
| interop `unbound-inner-address-uniqueness` | Done | `test/interop-ipsec/scenarios/unbound-inner-address-uniqueness/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `ts_narrow.go`, `child.go`, `table.go`, `reconcile.go`, callers | Done | |
| `dataplane/policy_owner.go` | Done | `PolicySelectorsOverlap` |
| `interoplab/ipsec/checkers.go`, `unbound_inner.go` | Done | `checkers.go:95` |
| `rfc/short/rfc3948.md` | Done | |
| five doc pages | Done | see Documentation Updates |
| `plan/learned/035-ike-virtual-ip-assignment.md` | Done in commit A | pointer to the closure |
| `inner_selectors.go`, test files, `.ci`, scenario files | Done | Files Exist |

### Audit Summary
- **Total items:** 15 AC, 2 task requirements, 27 planned tests and files
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| No two peers hold a tunnel Child SA or kernel policy whose selectors overlap unless the operator configured both at different `policy-priority` values | interop + functional | interop `unbound-inner-address-uniqueness` green (`scratch/r1-interop-green.log`): one policy pair naming 10.1.2.3/32, strongSwan logs TS_UNACCEPTABLE for `ze-b`. Red with the guard's conflict return replaced by `continue` and the image rebuilt: "ze's kernel holds 2 outbound ESP policies to 10.1.2.3/32 (tunnels [172.28.0.9 172.28.0.7]), want exactly one" (`scratch/r1-interop-red.log`). `.ci` red `scratch/ci-unique-red.log`, green `scratch/ci-ipsec-unbound-inner-address-unique-promoted.log` |
| Operator-configured overlaps keep working | functional | `ipsec-overlapping-configured-selectors-kept.ci` red `scratch/ci-overlap-red.log`, green `scratch/ci-ipsec-overlapping-configured-selectors-kept-promoted.log`; `./le test functional ipsec` 27/27, 7 net-admin skips (`scratch/r2-func-ipsec.log`) |
| The first client keeps its traffic | interop | checker pings from 10.1.2.3 via child-a to 10.20.0.1 after ze-b's refusal, green in `scratch/r1-interop-green.log` |
| `RFC3948-5.1-1` loses `{partial}` with both polarities proven | RFC ledger | row at `rfc/short/rfc3948.md:227` has no `{partial}`; audit verdict `enforced` over 34 units (1a5f53cd30, rejudged at HEAD 58ee58c176 in 58a09337f8); discrimination records for the positive and negative units |

(scratch = `tmp/session/2026-10-25-f3ae7c00-09af-4420-914f-39f2f7169980/scratch`)

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Transport Child SA rekey after an IKE SA rekey loses RFC 7296 Section 2.23.1 handling (journal `one-state-held-in-two-fields.md`, 2026-10-26) | outside RFC 3948 Section 5.1 | `plan/release/fixes/spec-ike-transport-rekey-nat-state.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ike-unbound-peer-inner-address-f3ae7c00-09af-4420-914f-39f2f7169980.md` (3 files: the closure's spec, learned 038 and the learned 035 repoint; the code landed in the fix commits below, each reviewed in its round). Rounds 6 to 9 authorised by Thomas on 2026-10-27 ("approve") |
| `./le spec review check` | run at closure over the same three files, before `./le commit create` |
| Rounds | 9; round 9 clean (0 BLOCKER, 0 ISSUE). Product defects that earned rounds 6 to 8: fcf2e39dc8 (round 6), b99c6503a4 (round 7), 0308ef4d70 (round 8: `childPolicyPair` returned a pair no caller installs; a dead return, no behaviour change) |
| Reviewer lenses used | `/ze-review` full pass incl. step 18 style, independent ze-rfc-audit judges, ze-test-judge for every changed tagged or weakened unit; the three hand-written breaks of 690d38f274 judged genuine |

### Findings fixed

Rows 1 (round 1) and 4 (round 5) name their round in the fix commit; the round and severity of rows 2, 3, 5, 6 and 7 are reconstructed from the fix commits, and the main thread holds the round reports.

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | record released while a failed removal left the policy installed | `child.go` removal funnel | 3c8b9e42de |
| 2 | ISSUE | stranded record never retried | `inner_selectors.go` | dd20e0aa1a |
| 3 | ISSUE | stranded retry removed live policies; VPP lost the SAD id | `inner_selectors.go`, `dataplane/vpp_policy.go` | 000e6fdfea, dc7de8e76f |
| 4 | ISSUE | transport peer not ordered by its configured address | `classifyInnerPair` | 4287e22441 |
| 5 | ISSUE | transport peer narrowed to one port marked operator-fixed | `isConfiguredHost` | fcf2e39dc8 |
| 6 | ISSUE | operator-fixed judged on the remote half, not the installed pair | `classifyInnerPair` | b99c6503a4 |
| 7 | ISSUE | `childPolicyPair` returned a pair no caller installs | `childPolicyPair` | 0308ef4d70 |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/engine/inner_selectors.go` | yes | `ls -l`: 42225 bytes |
| `internal/component/ike/engine/rfc3948_inner_selector_test.go` (+ audit_gaps, local_half, ordered_transport, rekey, transport, wiring) | yes | `ls -l`: 38055, 12101, 5843, 10569, 3936, 14121, 17023 bytes |
| `test/ipsec/ipsec-unbound-inner-address-unique.ci`, `test/ipsec/ipsec-overlapping-configured-selectors-kept.ci` | yes | 6759, 6000 bytes |
| `test/interop-ipsec/scenarios/unbound-inner-address-uniqueness/{nat,swanctl,ze}.conf` | yes | 584, 1823, 1588 bytes |
| `internal/le/interoplab/ipsec/unbound_inner.go` | yes | 7859 bytes; registered `checkers.go:95` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-14 | each test exists | `git grep -l "func <Test>("` found all 25 at HEAD 58a09337f8 |
| AC-1 to AC-14 | tests green | full engine package green `scratch/job-unit-pkg-91a1d19e.log` (2026-10-27 00:30, 217s); a fresh run on 2026-10-27 could not build: the working tree's `inner_selectors.go` carries another session's in-flight edit (mtime 01:34, registry `claim` replaced by `beginInstall`/`finishInstall`), so `rfc3948_inner_selector_test.go:717` `reg.claim` is undefined there; HEAD's engine tests do not compile either (journal row 3f1a944493). Dataplane package green (`scratch/job-close-inner-820606d2.log`) |
| AC-15 | no `{partial}` | `grep -n 'RFC3948-5.1-1' rfc/short/rfc3948.md`: line 227 `- [ ] [RFC3948-5.1-1] [MUST] ...` with no marker |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| IKE_AUTH from a second unbound peer | `test/ipsec/ipsec-unbound-inner-address-unique.ci` | header names AC-1, AC-12; hub runs two `respond` peers with no pool, red/green logs above |
| Two configured peers, overlapping prefixes, distinct priorities | `test/ipsec/ipsec-overlapping-configured-selectors-kept.ci` | header names AC-4 and user story 2; red/green logs above |
| Two strongSwan clients behind NAT | interop `unbound-inner-address-uniqueness` | checker registered `checkers.go:95`; red proven |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | source read; `TestInnerSelectorInstallBackstopRefusesSecond` |
| A-2 | confirmed | `TestInnerSelectorSamePeerRekeyAllowed` |
| A-3 | confirmed | `gopls references`; `TestInnerSelectorRegistryEmptyAfterTeardown` |
| A-4 | confirmed | `scratch/r1-interop-green.log`: strongSwan keeps IKE SA `ze-b` ESTABLISHED, no Child SA |
| A-5 | confirmed | `./le test functional ipsec` 27/27 (7 net-admin skips, `scratch/r2-func-ipsec.log`); interop `virtual-ip-uniqueness` green (`scratch/r2-vip-uniqueness.log`) |
| A-6 | confirmed | `TestInnerSelectorOverlapSymmetric` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| guide and four architecture pages describe the guard | anchors on `inner_selectors.go` / `dropForeignInnerOverlaps` / `innerSelectorRegistry` resolve (`./le doc check all` source-anchor step lists no claim on them) | yes |
| RFC 3948 support row | `rfc/short/rfc3948.md` Support remaining (line 21) states 5.1 met among IKE peers' Child SAs | yes |
| test infrastructure | `docs/architecture/testing/interop.md:1767` describes `unbound-inner-address-uniqueness` | yes |
| `./le rfc check` | not run in this closure: HEAD's `ike/engine` and `ike/crypto` tests do not compile (journal row 3f1a944493), so only a clean clone with the test moved aside can run it; the two stale verdicts were rejudged (58a09337f8) | owed |
| `./le verify changed` | not run: owed to Thomas; it widens to the full verification after a deleted package (journal row 04dbfa9038) | owed |
| `./le test functional ipsec` | last 27/27 (7 net-admin skips) in an earlier round; a later attempt was blocked by another session's config build break | owed |
