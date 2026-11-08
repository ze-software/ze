# Spec: ipsec-spd-selection

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P3 |
| Release reason | Ze already runs one SPD per XFRM interface if_id beside the node-wide SPD at if_id 0, so RFC 4301 Section 4.4.1 owes the selection function and each SPD its own final DISCARD entry; the shipped partial selection leaves one IPv6 cleartext path. RFC4301-4.4.1-8 (application override) is Phase 8, in implementation: Thomas answered D8-1 to D8-4 (cilium/ebpf approved for `ike/dataplane`, default `deny`, socket policies only, one switch). |
| Scope | protocol |
| RFC gaps | RFC4301-4.4.1-5, RFC4301-4.4.1-8 |
| Depends | - |
| Phase | 8/8 |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze holds more than one SPD. The Linux kernel keeps one policy database for each
XFRM if_id: if_id 0 is the node-wide SPD that carries Ze's catch-all, the IKE
bypass and the operator's `policy` entries, and every XFRM interface a peer binds
with `vti bind` is a further SPD at that interface's if_id, which carries the
peer's Child SA policies (`engine/established.go` `resolveIfID`). RFC 4301 Section
4.4.1 therefore owes an explicit SPD selection function (RFC4301-4.4.1-5), and
Sections 4.4.1 and 5.2 ask every SPD for a final DISCARD entry.

Commit d0c2738a48 ("ike: traffic into an xfrm interface passes the discard
catch-all") shipped a partial selection function: if_id 0 BYPASS entries one rank
above the DISCARD catch-all, on the XFRM interface's ifindex for out and fwd, and
on each IPv6 address of the interface for out
(`engine/unmatched_xfrm_interface.go` `xfrmInterfaceExemptions`). Two things are
still missing:

| Missing | Effect today |
|---|---|
| A final entry in each interface SPD | An interface SPD has no entry beneath its Child SA policies. The kernel drops what reaches it unmatched for its own reasons (below), with no entry the operator can see |
| A sound source-address selection | An IPv6 packet sourced from the tunnel address but routed out of another interface meets the source BYPASS at if_id 0 and leaves in the clear (`docs/guide/ipsec.md` names this residual) |

Thomas approved the complete design on 2026-11-07, in his words: "The complete
work is the right solution. The extra spec is accepted." The design has five parts:

1. Each XFRM interface SPD carries its own final DISCARD entry.
2. Selection by output interface: the shipped if_id 0 BYPASS on the interface
   ifindex, out and fwd, is kept and becomes tested evidence.
3. Selection by tunnel source address, made sound: the IPv6 source BYPASS is kept,
   and a guard makes sure a packet sourced from a tunnel address leaves only
   through its XFRM interface.
4. Inbound selection (RFC 4301 Section 5.2): a kernel test proves a packet that
   arrives through an XFRM interface is judged in that interface's SPD. Code is
   added only if the test shows a gap.
5. Visibility and conformance: `show vpn ipsec dataplane policy` names each
   entry's SPD and role, and RFC4301-4.4.1-5 moves from gap to covered with a
   recorded discrimination.

Two points of the approved brief did not survive the source check. Each is
recorded under Key Design Decisions with its evidence, and the main thread
reports both to Thomas:

| Brief said | Source shows (Linux master, `net/xfrm/`, `net/ipv6/`) | This spec does |
|---|---|---|
| The interface catch-all honours `unmatched` (bypass or discard) | An XFRM interface has no cleartext path. `xfrmi_xmit` drops a packet whose lookup returns no state (`x = dst->xfrm; if (!x) goto tx_err_link_failure;`), and `__xfrm_policy_check` rejects a tunnel-mode packet matched by an ALLOW entry with no template (`secpath_has_nontransport(sp, k, &xerr_idx)` then `goto reject`). A BYPASS entry in an interface SPD would be listed as bypass while the kernel drops | The interface catch-all is DISCARD under both values. `unmatched` keeps deciding the if_id 0 catch-all, and the exemptions and the guard still follow it |
| The `ip rule from <tunnel address>` guard is consulted before the SPD 0 decision | True only when the socket already chose the source. With no source chosen, `ip6_dst_lookup_tail` looks the route up with the source unset, picks the source from that route, and keeps that route: no second lookup consults a source rule. The packet then meets the source BYPASS at if_id 0 | The approved rule and its per-interface table stay, and a postrouting drop in a Ze-owned nft table closes the path the rule cannot see |

### RFC 4301 text this work implements

| Section | Verbatim text (`rfc/full/rfc4301.txt`) |
|---|---|
| 4.4.1 | "An IPsec implementation MUST have at least one SPD, and it MAY support multiple SPDs, if appropriate for the context in which the IPsec implementation operates." |
| 4.4.1 | "However, if an implementation supports multiple SPDs, then it MUST include an explicit SPD selection function that is invoked to select the appropriate SPD for outbound traffic processing." |
| 4.4.1 | "The inputs to this function are the outbound packet and any local metadata (e.g., the interface via which the packet arrived) required to effect the SPD selection function." |
| 4.4.1 | "Every SPD SHOULD have a nominal, final entry that matches anything that is otherwise unmatched, and discards it." |
| 5.1 step 1 | "When a packet arrives from the subscriber (protected) interface, invoke the SPD selection function to obtain the SPD-ID needed to choose the appropriate SPD." |
| 5.2 step 1 | "When a packet arrives, it may be tagged with the ID of the interface (physical or virtual) via which it arrived, if necessary, to support multiple SPDs and associated SPD-I caches." and "(The interface ID is mapped to a corresponding SPD-ID.)" |
| 5.2 step 2 | "If multiple SPDs are employed, the tag assigned to the packet in step 1 is used to select the appropriate SPD-I (and cache) to search." |
| 5.2 | "Every SPD SHOULD have a nominal, final entry that catches anything that is otherwise unmatched, and discards it." |
| 5.2 step 5 | "If an IPsec system receives an inbound packet on an SA and the packet's header fields are not consistent with the selectors for the SA, it MUST discard the packet." |

### RFC4301-4.4.1-8, designed in Phase 8

RFC 4301 Section 4.4.1: "In host systems, applications MAY be allowed to create SPD
entries. (The means of signaling such requests to the IPsec implementation are
outside the scope of this standard.) However, the system administrator MUST be able
to specify whether or not a user or application can override (default) system
policies." Ze has no user or application principal in its SPD model
(`ipsec/spd_policy.go` `SPDPolicy` carries no override leaf). Thomas decided that
its design is a new phase of this spec: see "Phase 8: application override control"
below. Thomas answered its decisions D8-1 to D8-4 (table 8.10).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - SPD dispositions, the catch-all and the XFRM interface exemption
  → Decision: the catch-all is the first SPD entry in and the last out (`releaseSPD`); the exemptions go in before a re-asserted catch-all and come out after it
  → Constraint: a new entry kind follows the same install, prune and release path, and the SPD ledger (`spd_left.go`) hears every removal
- [ ] `docs/architecture/ike/ipsec-3-data-model.md` - the SPD data model, which describes one SPD
  → Constraint: the page is wrong once interface SPDs are named; it is edited in the same phase
- [ ] `docs/architecture/policyroute/policy-routing.md` - the `ip rule` writer
  → Decision: the rule encoder must start from `netlink.NewRule()` (`newIPRule`), and the table conversion is per architecture (`netlinkTableInt`)
  → Constraint: Ze reserves three table ranges, each declared once in `internal/core/iprule` (`ZeTableRanges`): 1000 to 1999 for VRFs, 2000 to 2999 for policy-routing auto tables, and 3000 to 3999 for the IPsec source guard; an operator table may use none of them, and a new owner of tables needs a range disjoint from all three
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - nft table ownership
  → Constraint: a producer's table name carries the `ze_` prefix (Rule 2, enforced by `RegisterTables`); teardown belongs to the firewall engine (Rule 3); the registry is process-local
- [ ] `docs/architecture/core-design.md`, section 19 - allowed imports
  → Constraint: `ike` may import `iface` today, not `firewall`; the guard adds `firewall` and the new core leaf to the `ike` row
- [ ] `docs/architecture/iface/logical-name-resolution.md` - how a logical name becomes a device
  → Constraint: the guard and the role column resolve the interface through `iface`, never the kernel directly

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - rows RFC4301-4.4.1-5 and RFC4301-4.4.1-8
  → Constraint: both rows once cited this spec at the root of `plan/`, where it never lived; the row edit names `plan/post-release/features/spec-ipsec-spd-selection.md`
- [ ] `rfc/requirements/rfc4301.md` - the same rows, same stale path

**Key insights:**
- if_id is the kernel's SPD-ID. Policy match requires equal if_id (`xfrm_policy_match`, `net/xfrm/xfrm_policy.c`)
- Outbound host and forwarded traffic is looked up at if_id 0 first, then again at the interface if_id inside `xfrmi_xmit`
- Inbound: `__xfrm_policy_check` takes the if_id from `xfrmi_decode_session`, which answers only for a packet with a secpath that arrived through an XFRM interface; a cleartext packet is always judged at if_id 0
- With no policy at all the kernel accepts a cleartext packet and rejects a tunnel-mode one (the `if (!pol)` branch of `__xfrm_policy_check`)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/unmatched.go` - `unmatchedPolicies`, `installUnmatched`, `removeUnmatched`: the if_id 0 catch-all in, out and fwd for both families at `PriorityUnmatched`, disposition from `unmatched`
- [ ] `internal/component/ike/engine/unmatched_xfrm_interface.go` - `xfrmInterfaceExemptions`, `boundXFRMInterfaces`, `readXFRMInterface`, `planXFRMExemptions`, `installXFRMExemptions`, `pruneXFRMExemptions`: if_id 0 BYPASS entries at `PriorityUnmatchedXFRMInterface`, only while the catch-all discards; a failed interface read keeps the held entries
- [ ] `internal/component/ike/engine/apply.go` - `applyConfig` installs exemptions before a re-asserted catch-all and prunes after; `releaseSPD` prunes exemptions before `removeUnmatched`, and keeps the catch-all while the ledger is not empty
- [ ] `internal/component/ike/engine/dynamic_wan.go` - the address worker re-plans, installs and prunes the exemptions on an address or link event
- [ ] `internal/component/ike/engine/spd_left.go` - `spdSelector`, `selectorOf`, `removeZePolicy`, `spdLeftover`
- [ ] `internal/component/ike/engine/established.go` - `emitRouteAdd(bus, child.TSRemote, log)` when a Child SA comes up
- [ ] `internal/component/ike/engine/child.go` - Child SA receive policies in and fwd (`appendChildReceivePolicies`) carry `child.IfID`
- [ ] `internal/component/ike/ipsec/spd_policy.go` - the operator's SPD entries, no override leaf
- [ ] `internal/component/ike/dataplane/dataplane.go` - priorities; `SPParams.Owner`; `PolicyInfo` with `Owner`, `OwnerKnown`, `IfIndex`, `IfID`
- [ ] `internal/component/ike/dataplane/policy_owner.go` - `policyOwners.claim` records only template-carrying policies (`isTemplateFree` returns early), keyed by the full selector including if_id and ifindex
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `policyInfoFromKernel` joins a kernel row back to its owner through `ownerOf`
- [ ] `internal/component/ike/dataplane/vpp_policy.go` - `ListPolicies` answers `ErrNotSupported` on VPP
- [ ] `internal/component/ike/cmd/show_dataplane.go` - `handleShowVPNIPsecDataplanePolicy`, `policyInfoToMap`: no ifindex, SPD or role key
- [ ] `internal/component/ike/yang/ze-ipsec-cmd.yang` - `container policy` description lists the keys
- [ ] `internal/plugins/policyroute/rules_linux.go` - `newIPRule`: fwmark rules, `AF_INET` only, in the gated `ze_policyroute` plugin
- [ ] `internal/core/rtproto/rtproto.go` - values 249 to 253 taken; `IsZe` and `names` list them
- [ ] `internal/component/firewall/registry.go` - `RegisterTables(owner, tables)` refuses a name without the `ze_` prefix

**Behavior to preserve:**
- The if_id 0 catch-all, its disposition from `unmatched`, its order (first in, last out) and its failure semantics
- The shipped exemptions, their priority, their lifecycle, and `TestXFRMInterfaceExemptionPassesTheCatchAllInKernel`
- Every key `show vpn ipsec dataplane policy` emits today, with its meaning
- policyroute's rules, tables, marks and output

**Behavior to change:**
- Each bound XFRM interface SPD gains a final DISCARD entry
- An IPv6 packet sourced from a tunnel address can no longer leave in the clear through another interface
- The policy listing names each entry's SPD and role

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The if_id 0 catch-all | `engine/unmatched.go` `unmatchedPolicies`, `installUnmatched`, `removeUnmatched` | Final entry of the node-wide SPD, in, out and fwd, both families | Extends: the builder takes the if_id, so the interface catch-all is the same entry in another SPD, riding the same install and release ordering |
| The XFRM interface exemptions | `engine/unmatched_xfrm_interface.go` `planXFRMExemptions`, `installXFRMExemptions`, `pruneXFRMExemptions` | The partial selection function: if_id 0 BYPASS on the interface ifindex and IPv6 addresses | Extends: the per-interface plan gains the interface catch-all and the source guard as further entry kinds, held beside the exemptions, read by the same `readXFRMInterface` |
| The address worker | `engine/dynamic_wan.go` | Re-plans the exemptions on an address or link event | Extends: the same call re-plans the guard, which follows the interface's IPv6 addresses |
| The SPD ledger | `engine/spd_left.go` `removeZePolicy`, `spdLeftover` | Records an entry whose removal failed and keeps the catch-all beneath it at exit | Extends: interface catch-all removals go through `removeZePolicy`. The ledger holds SPD entries only, so the guard's rules, routes and nft term report their own removal failures |
| The policy owner ledger | `dataplane/policy_owner.go` `policyOwners` | Records who installed each template-carrying selector, joined back on read | Extends: a record of role and SPD name for every Ze entry, template-free ones included, joined on read the same way; ownership refusal stays limited to template-carrying policies |
| `show vpn ipsec dataplane policy` | `ike/cmd/show_dataplane.go` `policyInfoToMap`; `yang/ze-ipsec-cmd.yang` `container policy` | Lists kernel SPD rows with owner | Extends: adds `spd`, `role` and `if-index` keys; no key removed |
| policyroute's `ip rule` encoder | `internal/plugins/policyroute/rules_linux.go` `newIPRule`, `netlinkTableInt` and its per-architecture files | Encodes a fwmark rule safely (NewRule seeding, table bound) | Replaces its location: the encoder moves to a core leaf `internal/core/iprule`, generalised to a source-prefix rule in either family; policyroute calls the leaf and its copy is deleted in the same commit (no layering). IKE cannot import a gated plugin, and policyroute's rules are IPv4 fwmark rules a host-originated IPv6 packet never meets |
| Route protocol ownership | `internal/core/rtproto` `Proto`, `IsZe`, `names` | Marks which routes Ze owns | Extends: one new value for the guard table routes, a free value outside the kernel's assigned ones, added to `IsZe` and `names` |
| Firewall table registry | `internal/component/firewall/registry.go` `RegisterTables`; `vrrp/ownerfilter.go` as precedent | A component or plugin contributes a `ze_` nft table | Extends: the IKE engine registers one `ze_ipsec_source_guard` table holding the postrouting drop |
| Interface resolution | `iface.ResolveIndex`, `iface.Addresses` | Logical name to device and addresses | Extends use: the guard reads the same facts the exemptions read |
| Child SA route events | `engine/established.go` `emitRouteAdd`, `emitRouteRemove` | Publish the remote traffic selector as a route | Extends: the same points add and remove the guard table's tunnel routes |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config commit of `vpn ipsec` with `unmatched` and a peer `vti bind <interface>`; interface address and link events from `iface`; Child SA establishment and removal
- `show vpn ipsec dataplane policy` from any command surface

### Transformation Path
1. `applyConfig` calls `planXFRMExemptions`, which for each bound interface reads its facts and returns three entry kinds: the if_id 0 exemptions (as today, only under DISCARD), the interface catch-all (the interface if_id, DISCARD, under both values of `unmatched`), and the IPv6 source guard (only under DISCARD, one per IPv6 address)
2. The interface if_id comes from the same resolution the Child SA policies use (`resolveIfID`), so the catch-all and the Child SAs sit in one SPD
3. Install order on every apply and address event: interface catch-all, then guard (nft drop, then table routes and rule), then the if_id 0 exemptions, then the if_id 0 catch-all re-assert as today; prune runs in reverse. A source BYPASS is never installed before its guard holds
4. The guard: the nft table `ze_ipsec_source_guard` holds, per bound interface, one postrouting rule dropping IPv6 whose source is one of the interface's addresses and whose output interface is not that interface. The routing part: one IPv6 rule `from <address>/128 lookup <table>` per address; the per-interface table holds a route per Child SA remote traffic selector bound to that interface (`dev <interface>`) and an `unreachable ::/0`
5. Child SA establishment and removal add and remove the table's tunnel routes where `emitRouteAdd` and `emitRouteRemove` already run
6. `releaseSPD` removes the guard after the exemptions, and each interface catch-all after its Child SA policies, before the if_id 0 catch-all
7. The dataplane records role and SPD name per selector at install; `ListPolicies` joins them; `policyInfoToMap` renders `spd`, `role`, `if-index`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IKE engine to kernel XFRM | `dataplane.Dataplane.InstallPolicy` / `RemovePolicyParams` (netlink) | No |
| IKE engine to kernel fib rules and routes | `internal/core/iprule` (netlink), routes marked with the new rtproto value | No |
| IKE engine to firewall component | `firewall.RegisterTables` then `ApplyAll`, in process (the IKE engine is a component and never forks) | No |
| IKE engine to iface | `iface.ResolveIndex`, `iface.Addresses`, the address and link events already consumed | No |
| Command dispatcher to IKE cmd | existing `ze-ike:show-vpn-ipsec-dataplane-policy` RPC | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `test/`, `docs/`; terms `show-vpn-ipsec-dataplane-policy`, `dataplane policy`, `policyInfoToMap`, `owner-known`; positive control: `internal/component/ike/cmd/show_dataplane_test.go`, `internal/component/ike/yang/cmd_schema_test.go` and `internal/le/site/terminaldemo/validate_topic_ipsec.go` were found. No protocol wire changes: IKE and ESP bytes are untouched. The only changed format is one show command's JSON, extended by new keys.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `show vpn ipsec dataplane policy` JSON | writer | `ike/cmd/show_dataplane.go` `policyInfoToMap` | builds the row map | Extend: add `spd`, `role`, `if-index`; every existing key kept |
| same | reader | `ike/cmd/show_dataplane_test.go` | asserts row keys | Extend with the new keys |
| same | reader | `internal/le/site/terminaldemo/validate_topic_ipsec.go` | validates the demo topic output | Re-validate; extend only if it asserts the full key set |
| same | reader | any `internal/le/interoplab/ipsec/` checker reading the listing | the implementing agent greps `dataplane policy` there | Re-validate |

### Integration Points
- `planXFRMExemptions`, `installXFRMExemptions`, `pruneXFRMExemptions`: the per-interface plan gains entry kinds
- `releaseSPD`: release order
- `emitRouteAdd`/`emitRouteRemove` call sites in `established.go`: guard table routes
- `policyOwners`: role and SPD name recorded
- `firewall.RegisterTables`: the guard table
- policyroute `applyIPRules`/`removeIPRules`: moved onto `internal/core/iprule`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes, by design | XFRM through `dataplane`, nft through `firewall.RegisterTables`, interfaces through `iface` |
| No unintended coupling (components stay isolated) | To verify | `ike` gains `firewall` and the core leaf in section 19; `firewall` is always on and `ike` is gated, so the direction is allowed |
| No duplicated functionality (extends existing, does not recreate) | Yes, by design | The rule encoder moves, it is not copied |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control-plane state only |
| Registration over hardcoding, outbound | Yes, by design | The guard table is a registered nft table; nothing in firewall names IPsec |
| Registration over hardcoding, inbound | To verify | `internal/core/rtproto` `IsZe` and `names` are central lists that must learn the new value; the implementing agent greps `rtproto.` for every other list naming Ze's route protocols |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An interface SPD's DISCARD catch-all at `PriorityUnmatched` sits below every Child SA policy of that if_id | `PriorityChildSA` 2000; lower value wins in XFRM | Child SA traffic dropped | `TestRFC4301InterfaceSPDDiscardsUnmatchedInKernel` positive leg | unvalidated |
| A-2 | On Linux 7.3 the `from <address>` rule is consulted before the if_id 0 SPD lookup when the socket chose the source | `ip6_dst_lookup_tail` looks up with the source set, and `xfrm_lookup_route` runs after it (`net/ipv6/ip6_output.c`, master) | Bound-source traffic is not steered; the nft drop still holds | `TestTunnelSourceGuardInKernel` bound-source leg on Ze's 7.3 kernel in QEMU | unvalidated |
| A-3 | With no source chosen the rule is not consulted, and the postrouting drop stops the leak | `ip6_dst_lookup_tail` comment: "the route-specific preferred source forces the ip6_route_output call _before_ ip6_route_get_saddr"; the first dst is kept when it has no error | If 7.3 re-looked up, the drop is redundant, not wrong | `TestTunnelSourceGuardInKernel` auto-source leg goes red with the nft table absent | unvalidated |
| A-4 | A packet arriving through an XFRM interface is judged in that interface's SPD, in and fwd | `__xfrm_policy_check` takes `if_id` from `xfrmi_decode_session`; `xfrm_policy_match` requires equal if_id | Inbound selection is not met; code is owed in phase 3 | `TestRFC4301InboundSelectsTheInterfaceSPDInKernel` | unvalidated |
| A-5 | A BYPASS entry at an interface if_id cannot pass cleartext | `xfrmi_xmit` `if (!x) goto tx_err_link_failure;`; template-less ALLOW with a tunnel secpath rejects in `__xfrm_policy_check` | The interface catch-all should then honour `unmatched` | `TestInterfaceSPDBypassCarriesNoCleartextInKernel` | unvalidated |
| A-6 | No other Ze writer uses the routing table range chosen for guard tables | `policy-routing.md` ranges 1000 to 2999 | Table collision | grep of table constants across `internal/`, and a core-leaf unit test asserting disjoint ranges | unvalidated |
| A-7 | No configuration uses a tunnel address as the IKE local address | `vti bind` interfaces carry inner addresses | The guard would capture IKE | verify refuses it, `TestGuardRefusesTunnelAddressAsIKEEndpoint` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The guard drops router traffic an operator expected to leave elsewhere with a tunnel source | `vti-ipv6-tunnel-source-no-cleartext-leak`, `vti-ipv6-inner-ipv4-outer` | That traffic left in the clear before; the guide states the new behaviour |
| R-2 | A reload window where a source BYPASS exists without its guard | ordering unit test | Install order in step 3; `TestXFRMInterfacePlanInstallOrder` |
| R-3 | Moving the encoder changes policyroute's rules | `rules_integration_linux_test.go` | Move only; policyroute tests unchanged and green |
| R-4 | A metadata-mode (`collect_md`) XFRM interface takes its if_id per packet | `xfrmi_xmit` reads `md_info->if_id` | The implementing agent confirms `iface` creates non-metadata interfaces only, and refuses `vti bind` to a metadata one if it can be configured |
| R-5 | A failed guard install leaves the source BYPASS unprotected | install error | The source BYPASS is not installed until its guard is; failure logs at error and retries on the next apply or address event, as `installXFRMExemptions` does |
| R-6 | The VPP backend has no if_id SPD | `vppPolicyInterface` refuses ifindex 0; `ListPolicies` unsupported | The interface catch-all follows the backend's own declaration, as `installUnmatched` does with `CatchAllInstaller`; the implementing agent reads `vpp.go` and records the VPP answer |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Route-based tunnels carry nothing (interface catch-all ranked above Child SAs), or IPv6 host traffic is dropped or leaks |
| How is it reverted? | Single-commit revert per phase; no config migration, no new config leaf |
| Who else touches this path? | `spec-home-cpe-1-logical-interface-name` (shipped the exemptions), `spec-ipsec-spd-name-selection` (operator entries in the if_id 0 SPD), policyroute |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config commit `vti bind` under `unmatched discard` | → | `planXFRMExemptions` interface catch-all and guard | `TestApplyInstallsInterfaceSPDCatchAllAndGuard` |
| config commit `vti bind` under `unmatched bypass` | → | `planXFRMExemptions` interface catch-all only | `TestApplyInstallsInterfaceSPDCatchAllUnderBypass` |
| interface IPv6 address added or removed | → | address worker re-plan | `TestAddressEventFollowsTheGuard` |
| Child SA up and down on a bound peer | → | guard table route add and remove | `TestChildSAMaintainsTheGuardTable` |
| `show vpn ipsec dataplane policy` | → | `policyInfoToMap` with `spd`, `role` | `test/ipsec/dataplane-policy-spd-role.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Peer with `vti bind xfrm0`, `unmatched discard` | The kernel SPD holds, at xfrm0's if_id, a DISCARD entry in, out and fwd for IPv4 and IPv6 at `PriorityUnmatched`, and traffic the Child SA covers still crosses xfrm0 | Child SA policies above it, because they rank first |
| AC-2 | Same, `unmatched bypass` | The same interface catch-all is installed as DISCARD; the if_id 0 catch-all is BYPASS; no exemption and no guard exist | None: a bypass at an interface if_id carries no cleartext (A-5) |
| AC-3 | Packet routed into xfrm0 that no Child SA covers | Dropped by the interface catch-all, and the kernel's policy block counter rises | None |
| AC-4 | ESP packet arriving through xfrm0 whose inner header matches no xfrm0 in or fwd entry, while an if_id 0 BYPASS would match it | Dropped: judged in xfrm0's SPD, not in SPD 0 | None |
| AC-5 | IPv6 packet with a socket-chosen source that is a tunnel address of xfrm0, destination routed out eth0 in main | Steered into xfrm0 by the rule when a tunnel route covers it, otherwise refused as unreachable; never leaves eth0 | Traffic a Child SA covers reaches the peer encrypted |
| AC-6 | IPv6 packet whose kernel-chosen source is a tunnel address of xfrm0, route out eth0 | Dropped at postrouting; never leaves eth0 in the clear | None |
| AC-7 | Packet into xfrm0, forwarded or IPv4 host | Passes the if_id 0 catch-all through the ifindex exemption and is protected (shipped behaviour kept) | All, as today |
| AC-8 | Interface loses an IPv6 address, peer unbinds, or Ze exits | Its guard rule, table routes and nft term are removed; the interface catch-all leaves after its Child SA policies; a failed removal is reported by the SPD ledger or the guard's own warning | The if_id 0 catch-all until release, as today |
| AC-9 | `show vpn ipsec dataplane policy` on every surface | Each row carries `spd` (`node` for if_id 0, the interface name for a bound if_id, `if-id <n>` when no bound interface has it), `role` (`catch-all`, `interface-catch-all`, `interface-exemption`, `ike-bypass`, `child-sa`, `operator`, or `unknown` for an entry Ze did not install) and `if-index`; existing keys unchanged | Foreign entries show `unknown`, never blank |
| AC-10 | `./le rfc check` after this work | RFC4301-4.4.1-5 carries a tagged positive and negative unit, each with a discrimination record from `./le rfc record`, and no `{gap}`; RFC4301-4.4.1-8 keeps its gap and cites this spec's real path | 4.4.1-8 until Phase 8 lands (AC-22) |
| AC-11 | policyroute config with mark rules | Rules, tables and routes identical to before the encoder move | All |
| AC-12 | Config whose IKE local address is also an address on a bound XFRM interface | Refused at verify, naming the address, the interface and the fix | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a route-based tunnel to strongSwan under `unmatched discard` and pings across it, both families | config → apply → interface catch-all, exemptions, guard → kernel → strongSwan | `vti-unmatched-discard-interface-spd` |
| 2 | Sends from a tunnel IPv6 address to a host only reachable out the WAN | host stack → rule or postrouting drop | `vti-ipv6-tunnel-source-no-cleartext-leak` |
| 3 | Reads which SPD holds each entry | CLI → RPC → `ListPolicies` → `policyInfoToMap` | `test/ipsec/dataplane-policy-spd-role.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInterfaceSPDCatchAllEntries` | `internal/component/ike/engine/unmatched_xfrm_interface_test.go` | six DISCARD entries at the interface if_id under both `unmatched` values (AC-1, AC-2) | |
| `TestXFRMInterfacePlanInstallOrder` | same | catch-all, guard, exemptions, if_id 0 catch-all; prune in reverse (R-2) | |
| `TestReleaseRemovesInterfaceSPDAndGuard` | same | AC-8; the ledger hears a failed removal | |
| `TestGuardRefusesTunnelAddressAsIKEEndpoint` | same | AC-12 | |
| `TestApplyInstallsInterfaceSPDCatchAllAndGuard` | the engine's apply test file | wiring row 1 through the recording dataplane | |
| `TestApplyInstallsInterfaceSPDCatchAllUnderBypass` | same | wiring row 2 | |
| `TestAddressEventFollowsTheGuard` | the engine's dynamic WAN test file | wiring row 3 | |
| `TestChildSAMaintainsTheGuardTable` | the engine's established test file | wiring row 4 | |
| `TestPolicyOwnersRecordRoleAndSPD` | `internal/component/ike/dataplane/policy_owner_test.go` | role and SPD name join on read, template-free entries included | |
| `TestShowDataplanePolicyNamesSPDAndRole` | `internal/component/ike/cmd/show_dataplane_test.go` | AC-9, a foreign row reads `unknown` | |
| `TestSourceRuleEncodesBothFamilies` | `internal/core/iprule/iprule_test.go` | NewRule seeding kept, source prefix, family, table bound, ranges disjoint (A-6) | |
| `TestRFC4301SPDSelectionByOutputInterface` (positive, tagged RFC4301-4.4.1-5) | `internal/component/ike/engine/rfc4301_spd_selection_test.go` | the selection entries hand traffic routed into the interface to its SPD | |
| `TestRFC4301SPDSelectionLeavesOtherTrafficInTheNodeSPD` (negative, tagged RFC4301-4.4.1-5) | same | traffic not routed into a bound interface is not exempted: no exemption entry without the interface ifindex, and no IPv6 source entry without its guard | |

### Kernel integration tests (QEMU, Ze's 7.3 kernel)
| Test | File | Validates |
|------|------|-----------|
| `TestXFRMInterfaceExemptionPassesTheCatchAllInKernel` | `internal/component/ike/dataplane/xfrm_xfrmi_catchall_integration_linux_test.go` | existing, kept (AC-7) |
| `TestRFC4301InterfaceSPDDiscardsUnmatchedInKernel` | same file | AC-1, AC-3, A-1 |
| `TestInterfaceSPDBypassCarriesNoCleartextInKernel` | same file | A-5 |
| `TestRFC4301InboundSelectsTheInterfaceSPDInKernel` | same file | AC-4, A-4 |
| `TestTunnelSourceGuardInKernel` | `internal/component/ike/engine/source_guard_integration_linux_test.go` | AC-5, AC-6, A-2, A-3, with legs showing each half alone is red on its own path |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| guard routing table id | the reserved range the core leaf declares | its last value | range start minus 1 | range end plus 1 |
| interface if_id from `resolveIfID` | 1 to 2^32-1 | 2^32-1 | 0 (that is SPD 0, never an interface SPD) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dataplane-policy-spd-role` | `test/ipsec/dataplane-policy-spd-role.ci` | the operator lists policies and sees SPD and role per entry | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `vti-unmatched-discard-interface-spd` | `test/interop-ipsec/scenarios/vti-unmatched-discard-interface-spd/` | strongSwan | Under `unmatched discard`, IPv4 and IPv6 cross the route-based tunnel both ways; a packet routed into the interface outside the traffic selectors is dropped and counted | |
| `vti-ipv6-tunnel-source-no-cleartext-leak` | `test/interop-ipsec/scenarios/vti-ipv6-tunnel-source-no-cleartext-leak/` | strongSwan | A packet from the tunnel address towards a WAN-only destination never appears in the clear on the WAN capture | |
| `vti-ipv6-inner-ipv4-outer` | existing | strongSwan | still green (AC-7) | |

strongSwan is only the far end: the SPD entries are Ze's own and nothing changes
on the wire. Before a scenario expects strongSwan to send, answer or print
anything new, the implementing agent reads strongSwan's source for it and names
the function (`ai/rules/interop-and-goal-validation.md`).

### Red walks (discrimination)
| Test | Break of the producing code | Expected |
|------|-----------------------------|----------|
| `TestRFC4301SPDSelectionByOutputInterface` | `xfrmInterfaceExemptions` returns no ifindex entries | red |
| `TestRFC4301SPDSelectionLeavesOtherTrafficInTheNodeSPD` | the exemption's `IfIndex` set to 0 (any device), or the source entry planned without its guard | red |
| `TestRFC4301InterfaceSPDDiscardsUnmatchedInKernel` | the interface catch-all not installed | red on the counter leg |
| `TestRFC4301InboundSelectsTheInterfaceSPDInKernel` | the probe's in entry installed at if_id 0 only | red |
| `TestTunnelSourceGuardInKernel` | omit the nft table; separately omit the rule | red on the auto-source leg; red on the bound-source steering leg |
| `vti-unmatched-discard-interface-spd` | Ze image rebuilt without the exemptions | red |
| `vti-ipv6-tunnel-source-no-cleartext-leak` | Ze image rebuilt without the guard | red |

The two tagged units are recorded with `./le rfc record`, which performs the break,
refuses a red it did not observe, and writes `rfc/discrimination/rfc4301.json`.

## Files to Modify

- `internal/component/ike/engine/unmatched_xfrm_interface.go` - interface catch-all and guard as entry kinds of the per-interface plan; install order
- `internal/component/ike/engine/unmatched.go` - the catch-all builder takes an if_id
- `internal/component/ike/engine/apply.go` - order in `applyConfig`, `releaseSPD`
- `internal/component/ike/engine/dynamic_wan.go` - re-plan includes the guard
- `internal/component/ike/engine/established.go` - guard table routes at the Child SA route events
- `internal/component/ike/engine/spd_left.go` - interface catch-all removals reported
- `internal/component/ike/dataplane/dataplane.go` - `SPParams` and `PolicyInfo` gain role and SPD name; the role type
- `internal/component/ike/dataplane/policy_owner.go` - role and SPD name recorded for every Ze entry
- `internal/component/ike/dataplane/xfrm_linux.go` - join role and SPD on read
- `internal/component/ike/cmd/show_dataplane.go` - `spd`, `role`, `if-index`
- `internal/component/ike/yang/ze-ipsec-cmd.yang` - `container policy` description names the new keys
- `internal/plugins/policyroute/rules_linux.go` - uses `internal/core/iprule`; the local encoder and its per-architecture table conversion files are deleted
- `internal/core/rtproto/rtproto.go` - new value and name for the guard routes
- `docs/architecture/core-design.md` - section 19, `ike` row: `firewall`, `internal/core/iprule`
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - SPD dispositions per SPD, the guard
- `docs/architecture/ike/ipsec-3-data-model.md` - multiple SPDs and the selection function
- `docs/architecture/ike/ipsec-dataplane-inspection.md` - the `spd`, `role` and `if-index` keys of the policy read surface (declared by `show_dataplane.go`)
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the guard table routes at Child SA establishment, if the page describes the route events; otherwise named unaffected at closure with the reason (declared by the engine files this spec changes)
- `docs/architecture/policyroute/policy-routing.md` - the encoder's new home
- `docs/guide/ipsec.md` - "The catch-all entry", "XFRM interfaces", "Reading the kernel dataplane"; the residual paragraph replaced by the guard
- `rfc/short/rfc4301.md` - RFC4301-4.4.1-5 covered; the 4.4.1-8 path corrected
- `rfc/requirements/rfc4301.md` - same rows
- `docs/features/rfc-status.md` - the RFC 4301 row

## Files to Create

- `internal/core/iprule/iprule_linux.go`, `internal/core/iprule/iprule_other.go`, `internal/core/iprule/iprule_test.go` - the moved encoder, source-prefix rules for both families, table ranges
- `internal/component/ike/engine/source_guard.go` - the guard's nft table, rules and table routes
- `internal/component/ike/engine/source_guard_integration_linux_test.go`
- `internal/component/ike/engine/rfc4301_spd_selection_test.go`
- `test/ipsec/dataplane-policy-spd-role.ci`
- `test/interop-ipsec/scenarios/vti-unmatched-discard-interface-spd/` (`ze.conf`, `swanctl.conf`)
- `test/interop-ipsec/scenarios/vti-ipv6-tunnel-source-no-cleartext-leak/` (`ze.conf`, `swanctl.conf`)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new leaf or RPC; the existing command's description changes |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | Yes | AC-12 refusal in the IKE config verify path |
| CLI commands/flags | No | Existing command |
| CLI grammar (keyword before value) | No | No grammar change |
| Editor autocomplete | No | No new value |
| Functional test for new RPC/API | Yes | `test/ipsec/dataplane-policy-spd-role.ci` |
| Pipe completeness | No | The handler still returns a map rendered by `ApplyPipes` |
| Env var registration | No | None |
| Doctor check for runtime dependencies | No | XFRM, nft and fib rules are dependencies the IKE and firewall checks already cover; the implementing agent confirms in the diagnostic code registry |
| Prometheus counters/metrics | No | Kernel counters are read by existing commands |
| BGP family surface | No | N-A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the IPsec feature file under `features/` |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` if it lists the policy keys |
| 4 | API/RPC added/changed? | No | same RPC |
| 5 | Plugin added/changed? | No | policyroute behaviour unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4301.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `core-design.md` section 19, `ipsec-8-ikev2-child-xfrm.md`, `ipsec-3-data-model.md`, `policy-routing.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/post-release/features/spec-ipsec-spd-selection.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` policy listing examples |

## Phase 8: application override control (RFC4301-4.4.1-8)

Designed 2026-11-08. Thomas answered D8-1 to D8-4 the same day (table 8.10,
Answer column), and the phase is in implementation.

### 8.1 Facts the design rests on

Linux sources read at tag v7.3 (the shipped kernel), fetched from
`torvalds/linux` into the session scratch (`linux-xfrm/v7.3/`); strongSwan 6.1.0
from the lab tarball.

| # | Fact | Source and quote |
|---|------|------------------|
| F1 | A socket policy needs CAP_NET_ADMIN in the user namespace that owns the socket's network namespace, and nothing else | `net/ipv4/ip_sockglue.c` `do_ip_setsockopt`: "case IP_IPSEC_POLICY: case IP_XFRM_POLICY: err = -EPERM; if (!sockopt_ns_capable(sock_net(sk)->user_ns, CAP_NET_ADMIN)) break; err = xfrm_user_policy(sk, optname, optval, optlen);". `net/ipv6/ipv6_sockglue.c` holds the same for `IPV6_IPSEC_POLICY` and `IPV6_XFRM_POLICY`. `xfrm_user_policy` (`net/xfrm/xfrm_state.c`) adds no further check: it compiles the policy through `km->compile_policy` and calls `xfrm_sk_policy_insert` |
| F2 | A socket policy outranks the SPD | `xfrm_lookup_with_ifid` (`net/xfrm/xfrm_policy.c`): "if (sk && sk->sk_policy[XFRM_POLICY_OUT]) { num_pols = 1; pols[0] = xfrm_sk_policy_lookup(...)", and the SPD is searched only under "if (xdst == NULL)". `__xfrm_policy_check`: "if (sk && sk->sk_policy[dir]) { pol = xfrm_sk_policy_lookup(sk, dir, &fl, family, if_id);" then "if (!pol) pol = xfrm_policy_lookup(net, &fl, family, dir, if_id);". So one ALLOW socket policy takes that socket out of every SPD entry, the DISCARD catch-all included |
| F3 | No sysctl or netns knob restricts socket policies | `net/xfrm/xfrm_sysctl.c` registers four entries only: `xfrm_aevent_etime`, `xfrm_aevent_rseqth`, `xfrm_larval_drop`, `xfrm_acq_expires`. `XFRM_MSG_SETDEFAULT` (`policy_default`) decides only a packet that meets no policy at all. The only LSM call on this path is `security_xfrm_policy_lookup`, made from `xfrm_sk_policy_lookup` against a labelled policy; Ze's kernel builds no SELinux and no BPF LSM (`gokrazy/kernel/kernel.config` carries `CONFIG_SECURITY_LANDLOCK=y` only, and Landlock has no setsockopt rule) |
| F4 | The kernel's own enforcement point exists: cgroup BPF on setsockopt | `net/socket.c` `do_sock_setsockopt` runs "err = security_socket_setsockopt(sock, level, optname);" and then "BPF_CGROUP_RUN_PROG_SETSOCKOPT(sock->sk, &level, &optname, ...)" before the protocol handler. `kernel/bpf/cgroup.c` `bpf_prog_run_array_cg`: "if (!func_ret && cgroup_bpf_hook_returns_errno(atype) && !IS_ERR_VALUE((long)run_ctx.retval)) run_ctx.retval = -EPERM;", so any attached program that answers 0 refuses the call, whatever other programs answer. `cg_sockopt_func_proto` offers `BPF_FUNC_get_netns_cookie`; `bpf_base_func_proto` (`kernel/bpf/helpers.c`) offers `BPF_FUNC_get_current_pid_tgid`. Ze's kernel carries `CONFIG_CGROUP_BPF=y`, `CONFIG_BPF_SYSCALL=y` and `CONFIG_BPF_JIT=y` (`gokrazy/kernel/runtime.config`). gokrazy mounts cgroup2 ("syscall.Mount(\"cgroup2\", \"/sys/fs/cgroup\", \"cgroup2\", 0, \"\")", `mount.go` in the local gokrazy checkout; Ze's pinned version is assumption A8-2) |
| F5 | Socket policies are visible in the XFRM dump, and cannot be deleted through it | `xfrm_sk_policy_link` calls "__xfrm_policy_link(pol, XFRM_POLICY_MAX + dir)", which does "list_add(&pol->walk.all, &net->xfrm.policy_all)", and `xfrm_policy_walk` walks `policy_all` with no socket filter. iproute2 `ipxfrm.c` prints "socket " for "xpinfo->dir >= XFRM_POLICY_MAX", and `ip xfrm policy` offers `nosock` to hide them. `verify_policy_dir` (`net/xfrm/xfrm_user.c`) accepts only `XFRM_POLICY_IN`, `OUT` and `FWD`, so no netlink request can remove one: it lives as long as its socket. The dump names no owning process |
| F6 | A direct SPD write needs the same capability | `xfrm_user_rcv_msg` (`net/xfrm/xfrm_user.c`): "/* All operations require privileges, even GET */ if (!netlink_net_capable(skb, CAP_NET_ADMIN)) return -EPERM;". cgroup BPF has no hook on a netlink send, so on Ze's kernel nothing separates "may write the SPD" from "holds CAP_NET_ADMIN" |
| F7 | Ze sets exactly one socket policy | `dataplane/espform_linux.go` `espFormBypassInboundPolicy`: an IPv4 `XFRM_POLICY_IN` ALLOW with an empty selector on the ESP form reader's raw protocol-50 socket, set through `unix.IP_XFRM_POLICY`. The IKE bypass is not a socket policy: `engine/bypass.go` installs SPD entries on the local IKE ports, "the shape of its port_bypass option" (strongSwan's). A grep of `internal/`, `cmd/` and `pkg/` finds no other `IP_XFRM_POLICY` or `IPV6_XFRM_POLICY` |
| F8 | Ze lists a socket policy with direction `unknown` today | `dataplane/xfrm_linux.go` `policyInfoFromKernel` maps "Dir: SADir(p.Dir) + 1", so the dump's socket directions 3 to 5 become 4 to 6, and `cmd/show_dataplane.go` `policyDirName` answers `unknownValue` for them. The vendored netlink names them (`XFRM_SOCKET_IN`, `XFRM_SOCKET_OUT`, `XFRM_SOCKET_FWD`). Journal row below |
| F9 | Ze's external plugins inherit the daemon's credentials | `plugin/sysproc_linux.go` `NewSysProcAttr` sets only `Setpgid` and `Pdeathsig`; `docs/architecture/system-architecture.md`: each external plugin "inherits the daemon's (already-dropped) uid/gid". The appliance sets no `ze.user`, so a forked plugin runs as root with CAP_NET_ADMIN |
| F10 | strongSwan has no control over other applications | `kernel_netlink_ipsec.c` `add_socket_bypass` sets `IP_XFRM_POLICY` or `IPV6_XFRM_POLICY` ALLOW, out then in, on charon's own IKE sockets. `conf/plugins/kernel-netlink.opt`: "charon.plugins.kernel-netlink.port_bypass = no ... Whether to use port or socket based IKE XFRM bypass policies." It chooses charon's own bypass shape only |
| F11 | FRR and BIRD expose nothing | A grep for `XFRM_POLICY` and `IPSEC_POLICY` over the local FRR and BIRD checkouts finds no use |

What "user or application" means for Ze, from F1, F6 and F9:

| Host | Who can override the SPD | What Linux already decides |
|------|--------------------------|----------------------------|
| Ze appliance | Ze itself (F7), and every external plugin the operator configures (F9). A CLI user runs no process and reaches the SPD only through a config commit, which `system authorization` already governs | A process without CAP_NET_ADMIN in Ze's netns is refused (F1). An unprivileged user who makes its own user and network namespace gains the capability only over that namespace's SPD, never Ze's |
| Distribution host | Every process holding CAP_NET_ADMIN in Ze's network namespace: root shells, other daemons | The same refusal for every other process |

### 8.2 Existing Mechanisms

| Mechanism | Where | What it does today | Phase 8 |
|-----------|-------|--------------------|---------|
| The kernel's CAP_NET_ADMIN gate | `ip_sockglue.c`, `xfrm_user.c` (F1, F6) | Refuses a socket policy and an SPD write to a non-holder | Kept as the floor. Phase 8 adds the administrator's choice above it for holders |
| `vpn ipsec unmatched` | `ipsec/yang/ze-ipsec-conf.yang`, `ipsec/config.go`, `engine/unmatched.go` | An administrator-chosen disposition read at apply | Extends: the new leaf is its sibling and rides the same parse, verify and apply path |
| The ESP form socket policy | `dataplane/espform_linux.go` `espFormBypassInboundPolicy` | Ze's only socket override | Extends: it carries a reserved priority as Ze's mark, so the listing names it `esp-form-socket`, and the guard exempts the process that sets it |
| The IKE bypass | `engine/bypass.go` `ikeBypassPolicies` | SPD entries on the local IKE ports | Does not apply: it is system policy in the SPD, listed as `ike-bypass`, not an application override |
| The traffic-usage eBPF loader | `plugins/trafficusage/attach_linux.go` (`cilium/ebpf`, `asm`, `link`) | Assembles a program in Go, attaches it by link, Ze owns the lifetime, no bpffs pin | Extends the pattern to a cgroup setsockopt program, subject to D8-1 |
| The policy role vocabulary | `dataplane/policy_role.go` `PolicyRole`, `policyRoleNames` | Names why Ze installed each entry | Extends: `esp-form-socket` and `socket-override` |
| The policy read-back | `dataplane/xfrm_linux.go` `policyInfoFromKernel`; `cmd/show_dataplane.go` `policyInfoToMap` | Maps the dump; socket rows read `unknown` (F8) | Replaces the socket direction mapping; adds the socket SPD name |
| Apply refusal | `engine/apply.go`, commit bf2e2d753c ("a refused interface SPD catch-all fails the apply") | A dataplane step the kernel refuses fails the apply | Extends: a guard that cannot attach fails the apply the same way |
| IKE doctor checks | `engine/register.go` `DoctorChecks`; `core/diagnostic/codes.go` | Registered readiness checks with codes | Extends: two checks, three codes |
| Startup capability warnings | `core/privilege/check_linux.go` `requiredCaps` | Warns at start for a missing capability, unconditionally | Does not apply: CAP_BPF is owed only while the leaf is `deny`, so the doctor check reports it, not the unconditional list |
| External plugin spawn | `plugin/sysproc_linux.go` `NewSysProcAttr`, `StartChild` | Plugins inherit the daemon's credentials | Not changed: a forked plugin is an application, and the guard covers it because it is not the process that loaded the guard |

### 8.3 Design

| Part | Decision |
|------|----------|
| The control | `vpn ipsec application-override`, an enumeration `deny` or `allow`, default per D8-2. It answers the RFC's question for every user and application in Ze's network namespace at once (granularity per D8-4) |
| What `deny` enforces | A cgroup setsockopt program, attached by link to the cgroup2 root with multi-attach. It refuses with EPERM `IP_XFRM_POLICY`, `IP_IPSEC_POLICY` (level `SOL_IP`) and `IPV6_XFRM_POLICY`, `IPV6_IPSEC_POLICY` (level `SOL_IPV6`), values taken from `golang.org/x/sys/unix`, when the socket's netns cookie is Ze's and the caller is not the process that loaded the guard: the one that runs the IKE dataplane and sets the ESP form policy. The program reads the caller's tgid in Ze's own PID namespace with `bpf_get_ns_current_pid_tgid`, given the device and inode of Ze's `/proc/self/ns/pid` (the device converted to the kernel's MKDEV encoding, which the helper compares), and compares it with Ze's `os.Getpid`: `bpf_get_current_pid_tgid` answers in the initial namespace, which a container's `os.Getpid` never equals. A caller in another PID namespace makes the helper fail, and is refused even when its initial-namespace tgid equals Ze's. Every other setsockopt passes untouched. A per-CPU counter map counts refusals |
| What `allow` does | No program. Linux's CAP_NET_ADMIN gate alone applies |
| Ze's own override | The ESP form socket policy is the IPsec implementation's own receive path (RFC 7296 Section 2.23, both ESP forms), not a user or application, so the tgid exemption lets it through under `deny`. It is marked and listed as `esp-form-socket`, and the doctor names it, so the administrator sees it. The IKE bypass is SPD content, not an override |
| Direct SPD writes | Out of the guard's reach on Ze's kernel (F6). Treated per D8-3 |
| Lifetime and order | Attached in `applyConfig` right after the if_id 0 catch-all is first installed, before any IKE socket opens; detached in `releaseSPD` after the catch-all leaves; a reload moves between `deny` and `allow` without a restart. The link closes with the process, so the guard lives exactly as long as Ze's SPD does |
| Failure | Under `deny`, a guard that cannot load or attach (no CAP_BPF or CAP_NET_ADMIN, no cgroup2 mount, no `CONFIG_CGROUP_BPF`) fails the apply and installs nothing further: exact or refuse |
| Visibility | The listing maps socket directions to `in`, `out` and `fwd`, gives socket rows the SPD name `socket`, and the role `esp-form-socket` (Ze's mark) or `socket-override` (anything else) |
| What Ze cannot do | Remove a socket policy set before the guard attached (F5), name the process that owns one (F5), or stop a CAP_NET_ADMIN holder writing the SPD (F6). Each is reported by doctor |
| VPP | The VPP SPD has no socket policies; the guard concerns the kernel XFRM backend only. The implementing agent reads `dataplane/vpp.go` and records how the VPP API socket is protected, as R-6 did for the catch-all |

User-facing text (`ai/rules/user-facing-errors.md`):

| Surface | Text |
|---------|------|
| Apply refusal | "vpn ipsec application-override deny: Ze could not attach its setsockopt guard to the cgroup2 root (<kernel error>). Without it any process holding CAP_NET_ADMIN can exempt its sockets from the IPsec policy. Run ze with CAP_BPF and CAP_NET_ADMIN, mount cgroup2 at /sys/fs/cgroup, or set `vpn ipsec application-override allow` to accept that risk." |
| Doctor, guard detached under `deny` | "The IPsec application-override guard is not attached, so applications can override the SPD. Something detached it or Ze's load failed. Restart ze; if this persists, run `ze doctor --json` and read the attach error." |
| Doctor, foreign socket policy | "A process holds a per-socket IPsec policy (<direction>, <selector>) that overrides the SPD for its socket. The kernel cannot remove it and does not say which process owns it. Restart the applications started before Ze, or set `vpn ipsec application-override deny` and restart them." (warning, under `deny` only: under `allow` the administrator accepted overrides) |
| Doctor, other SPD writers | "Processes <pid (comm), ...> hold CAP_NET_ADMIN in Ze's network namespace, so Linux lets them write the IPsec SPD directly. Remove the capability from them or stop them." |

### 8.4 Data Flow

| Step | What happens |
|------|--------------|
| 1 | `vpn ipsec application-override` is parsed beside `unmatched` (`ipsec/config.go`) into the typed config; YANG refuses another value |
| 2 | `applyConfig` asks the dataplane for the guard under `deny` after the catch-all; the Linux backend loads and attaches the program; a failure fails the apply with the text above |
| 3 | A process in Ze's netns calls one of the four setsockopt options: the kernel runs the program before `do_ip_setsockopt`, which answers EPERM unless the caller is the guard's own process |
| 4 | Reload: `deny` to `allow` closes the link; `allow` to `deny` attaches it; leaving IPsec detaches at `releaseSPD` |
| 5 | `ListPolicies` reads socket rows, maps their direction, names SPD `socket` and the role from the ESP form mark |
| 6 | Doctor reads the guard's link state, the refusal counter, the socket rows, and the CAP_NET_ADMIN holders in Ze's netns (`/proc/<pid>/status` CapEff bit 12, `/proc/<pid>/ns/net` equal to Ze's) |

| Boundary | How | Verified |
|----------|-----|----------|
| IKE engine to kernel BPF and cgroup2 | `bpf(2)` program load and link create through the dataplane seam | No |
| IKE engine to kernel XFRM | the existing policy dump | No |
| Doctor to `/proc` | read only, inside the daemon and in `ze doctor` | No |

### 8.5 Assumptions and risks

| ID | Statement | Basis | Validated by |
|----|-----------|-------|--------------|
| A8-1 | The program sees the caller's tgid and the socket's netns cookie in a setsockopt hook on 7.3 | F4 helper lists | `TestRFC4301ApplicationOverrideDeniedInKernel` exemption and netns legs |
| A8-2 | Ze's pinned gokrazy mounts cgroup2 at `/sys/fs/cgroup` | the local gokrazy `mount.go` | CONFIRMED: Ze pins gokrazy v0.0.0-20260703061218-a4a45a20149d (`gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod`), whose `mount.go` line 174 reads "syscall.Mount("cgroup2", "/sys/fs/cgroup", "cgroup2", 0, "")" (`gokrazy/modcache`); the kernel test boots Ze's image |
| A8-3 | A distribution host with a cgroup v1 hybrid layout mounts cgroup2 elsewhere | systemd hybrid layout | the guard finds the cgroup2 mount from `/proc/self/mountinfo`, never a fixed path |
| A8-4 | The kernel copies the priority of a socket policy and ignores it for matching | `copy_from_user_policy` "xp->priority = p->priority;", called from `xfrm_compile_policy`; `xfrm_sk_policy_lookup` reads no priority | CONFIRMED at v7.3 source (`xfrm_user.c` line 2200 in `copy_from_user_policy`, reached from `xfrm_compile_policy`); `TestESPFormSocketPolicyCarriesZeMarkInKernel` proves it on the running kernel |
| R8-1 | Default `deny` on a host whose ze lacks CAP_BPF fails every IPsec apply | F4 | the refusal names the fix; the guide says so |
| R8-2 | A process with CAP_SYS_ADMIN or CAP_BPF detaches the link | link semantics | doctor reports a detached guard as an error |
| R8-3 | The program runs on every setsockopt on the host | F4 | it returns on the level and option compare first; the counter is per-CPU |

### 8.6 Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-13 | `vpn ipsec` configured without the leaf | `show config` reports the default from D8-2, and the apply behaves as that value | None |
| AC-14 | `deny`; a process other than the guard's own, with CAP_NET_ADMIN, in Ze's netns, calls each of the four options in both families | Each call fails with EPERM; its traffic still meets the SPD (an unmatched probe from it is dropped by the DISCARD catch-all, `XfrmOutPolBlock` rises); the refusal counter rises | None |
| AC-15 | `deny`; the same call from a process in another network namespace | Succeeds: the guard governs Ze's SPD only | Other namespaces have their own SPD |
| AC-16 | `deny`; an SA that asks for both ESP forms | The ESP form reader sets its socket policy and receives both forms, as today; the listing shows it as `esp-form-socket` | Ze's own receive path |
| AC-17 | `allow` | No guard attached; the call in AC-14 succeeds; the listing shows the policy as `socket-override`; doctor reports nothing about socket policies, because the administrator chose to allow them (doctor has only the error and warning levels) | All socket policies of CAP_NET_ADMIN holders |
| AC-18 | `deny` where the guard cannot attach | The apply fails with the refusal text, naming the kernel error; doctor reports an error | None |
| AC-19 | Reload `deny` to `allow`, `allow` to `deny`, IPsec removed | The guard detaches, attaches, detaches, with no restart | Socket policies set while `allow` held stay until their sockets close (F5), and doctor names them |
| AC-20 | `show vpn ipsec dataplane policy` with socket policies present | Each socket row reads direction `in`, `out` or `fwd`, `spd` `socket`, role `esp-form-socket` or `socket-override`; no row reads direction `unknown` | None |
| AC-21 | Doctor under each state | Guard detached under `deny`: error. Socket policy not attributable to Ze: warning under `deny`, nothing under `allow`. Another CAP_NET_ADMIN holder in Ze's netns: warning naming pid and command | None |
| AC-22 | `./le rfc check` after Phase 8 | RFC4301-4.4.1-8 carries a tagged positive and negative test, each with a discrimination record, and the annotation D8-3 settles; no `{gap}` | None |

### 8.7 TDD

| Test | File | Validates |
|------|------|-----------|
| `TestApplicationOverrideLeafParses` | `internal/component/ike/ipsec/config_test.go` | AC-13, both values and the default |
| `TestApplyAttachesOverrideGuardUnderDeny` | the engine's apply test file | wiring: commit to guard attach through the recording dataplane, after the catch-all |
| `TestReloadMovesOverrideGuard` | same | AC-19 |
| `TestApplyFailsWhenOverrideGuardCannotAttach` | same | AC-18, the text names the error and the fix |
| `TestPolicyInfoMapsSocketDirections` | `internal/component/ike/dataplane/xfrm_linux_test.go` | F8 fixed: directions 3 to 5 map to in, out, fwd with SPD `socket` |
| `TestShowDataplanePolicyNamesSocketRows` | `internal/component/ike/cmd/show_dataplane_test.go` | AC-20 |
| `TestDoctorApplicationOverride` | `internal/component/ike/engine/doctor_application_override_test.go` | AC-21, each state through seams |
| `TestDoctorSPDWritersReadsProc` | same | AC-21: another CAP_NET_ADMIN holder in Ze's network namespace is named by pid and command; Ze, a kernel thread, a non-holder and another namespace are not |
| `TestDoctorSPDWritersReportsUnreadableProc` | same | a missing `/proc`, and a process whose namespace Linux hides, raise `doctor-ipsec-spd-writer-unknown` with the cause and the fix, never "no writer"; a process that exited during the scan does not |
| `TestOverrideGuardProgramExemptsZeInItsPIDNamespace` | `internal/component/ike/dataplane/override_guard_program_linux_test.go` | the exemption over the program's instructions: Ze passes on the host and in a container (its initial-namespace tgid differs from `os.Getpid`); another process, and a caller in another PID namespace whose initial-namespace tgid equals Ze's, are refused and counted; other options and other network namespaces pass uncounted |
| `TestKernelDevEncodesMKDEV` | same | the device of `/proc/self/ns/pid` is converted to the kernel's MKDEV encoding the helper compares |
| `TestRFC4301OverrideLeafSelectsTheGuard` (tagged RFC4301-4.4.1-8) | `internal/component/ike/engine/rfc4301_application_override_test.go` | `deny` attaches and `allow` does not, through the apply path |
| `ipsec-application-override` | `test/parse/ipsec-application-override.ci` | the leaf through the real config path; a bad value is refused with the allowed values named |

| Kernel test (QEMU, Ze's 7.3 kernel) | File | Validates |
|-------------------------------------|------|-----------|
| `TestRFC4301ApplicationOverrideDeniedInKernel` (tagged RFC4301-4.4.1-8, positive) | `internal/component/ike/dataplane/override_guard_integration_linux_test.go` | AC-14 with a forked helper as the application, AC-15 and AC-16 legs; the PID namespace leg: the loader forked with CLONE_NEWPID runs as pid 1 there, its own ESP form policy passes, and this process, outside that namespace, is refused with EPERM |
| `TestRFC4301ApplicationOverrideAllowedInKernel` (tagged, negative) | same | AC-17: the same helper succeeds with no guard |
| `TestESPFormSocketPolicyCarriesZeMarkInKernel` | same | A8-4 |
| `TestOverrideGuardHelperProcess` | same | not a test of its own: the forked helper the kernel tests run as the application and as the loader |

Each kernel test is root-only (it skips otherwise) and runs in its own network namespace on a locked thread.

| Red walk | Break of the producing code | Expected |
|----------|-----------------------------|----------|
| `TestRFC4301ApplicationOverrideDeniedInKernel` | the program answers 1 for every call | red on the EPERM leg |
| same | the tgid exemption removed | red on the ESP form leg |
| same | the netns compare removed | red on the other-namespace leg |
| `TestRFC4301OverrideLeafSelectsTheGuard` | the apply ignores the leaf | red |
| `TestPolicyInfoMapsSocketDirections` | the old offset mapping restored | red |

The two tagged carriers are recorded with `./le rfc record`. If the recorder cannot
drive a QEMU test, the implementing agent reports it to the main thread rather than
moving the tag onto a unit that proves less. No interop scenario is owed: the
control changes no IKE or ESP byte and has no protocol peer
(`ai/rules/interop-and-goal-validation.md`, the config-only exemption).

### 8.8 Files

| Action | File | Change |
|--------|------|--------|
| Modify | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` | `application-override` leaf beside `unmatched`, quoting Section 4.4.1 |
| Modify | `internal/component/ike/ipsec/config.go`, `internal/component/ike/ipsec/types.go` | parse and carry the leaf |
| Modify | `internal/component/ike/engine/apply.go` | attach after the catch-all, detach in `releaseSPD`, reload transitions |
| Modify | `internal/component/ike/dataplane/dataplane.go` | the guard seam on the backend interface and the recording backend |
| Modify | `internal/component/ike/dataplane/espform_linux.go` | Ze's mark priority on the socket policy |
| Modify | `internal/component/ike/dataplane/policy_role.go` | `esp-form-socket`, `socket-override`, SPD name `socket` |
| Modify | `internal/component/ike/dataplane/xfrm_linux.go` | socket direction mapping, role from the mark |
| Modify | `internal/component/ike/cmd/show_dataplane.go`, `internal/component/ike/yang/ze-ipsec-cmd.yang` | socket rows; the `container policy` description |
| Modify | `internal/component/ike/engine/register.go` | two doctor checks |
| Modify | `internal/core/diagnostic/codes.go` | four codes with their remedies: the guard, the socket policy, the SPD writer, and the SPD writer Ze cannot see |
| Create | `internal/component/ike/dataplane/override_guard_linux.go`, `internal/component/ike/dataplane/override_guard_other.go` | program, load, attach, detach, counter; the PID namespace the exemption reads |
| Create | `internal/component/ike/engine/doctor_application_override.go` and its test `internal/component/ike/engine/doctor_application_override_test.go` | the checks, and the unreadable `/proc` answer |
| Create | `internal/component/ike/dataplane/override_guard_program_linux_test.go`, `internal/component/ike/dataplane/override_guard_integration_linux_test.go`, `internal/component/ike/engine/rfc4301_application_override_test.go`, `test/parse/ipsec-application-override.ci` | the tests above |
| Docs | `docs/guide/ipsec.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/ike/ipsec-dataplane-inspection.md`, `docs/features/rfc-status.md` | the control, the guard, the socket rows |
| RFC | `rfc/short/rfc4301.md`, `rfc/requirements/rfc4301.md` | RFC4301-4.4.1-8 per D8-3 |

### 8.9 Annotation outcome

Not a `{linux-limit}`: Linux provides the primitive (F4), and Ze's kernel builds it.
Under D8-3 (a) the row ends covered: the administrator decides whether applications
may override, Ze enforces it at the only per-application override Linux offers, and
a direct SPD write belongs to a CAP_NET_ADMIN holder, which Linux treats as an
administrator and doctor names. Under D8-3 (b) it ends covered on Ze's kernel once
the BPF LSM leg lands.

### 8.10 Decisions only Thomas can make

Thomas's answers, 2026-11-08:

| ID | Answer |
|----|--------|
| D8-1 | (a): `cilium/ebpf` (`asm`, `link`) is approved for `ike/dataplane` |
| D8-2 | (a) `deny`. No config migration (no release shipped a config); every repository user of `vpn ipsec` (configs, `.ci`, fixtures, interop scenarios, engine tests) is converted or re-validated in this work |
| D8-3 | (a): socket policies only; doctor names the other CAP_NET_ADMIN holders |
| D8-4 | (a): one switch |

| ID | Question | Options | Recommendation |
|----|----------|---------|----------------|
| D8-1 | `cilium/ebpf` is approved for traffic-usage only; the guard needs a BPF program in `ike` | (a) reuse `cilium/ebpf` (`asm`, `link`) in `ike/dataplane`, as traffic-usage does; (b) hand-encode the program and call `bpf(2)` through `golang.org/x/sys/unix` | (a): the program is a dozen instructions, and (b) writes a second, unreviewed BPF loader |
| D8-2 | Default of `application-override` | (a) `deny`: no application overrides Ze's SPD unless the administrator allows it, and every IPsec apply then needs CAP_BPF; (b) `allow`: today's behaviour | (a): on the appliance nothing legitimate needs an override, and a default that lets any root plugin bypass the catch-all defeats `unmatched discard` |
| D8-3 | Does "override" cover a direct SPD write by another CAP_NET_ADMIN holder | (a) no: the guard covers socket policies, Linux makes every SPD writer an administrator (F6), doctor names them; (b) yes: also refuse XFRM netlink sends from other processes through a BPF LSM program on `netlink_send`, which needs `CONFIG_BPF_LSM=y` and `bpf` in the boot `lsm=` list on Ze's kernel, and works on a distribution host only where its kernel has both | (a), with (b) as a spec of its own if wanted: (b) changes Ze's kernel and boot line, and an external plugin that may write the SPD is one the administrator configured |
| D8-4 | Granularity | (a) one switch for every user and application; (b) an allow list of users (uid) or programs under `deny` | (a): the RFC asks whether, not for whom, and no caller needs a list today |

## Implementation Steps

1. **Phase: Wiring** - recording-dataplane tests for wiring rows 1 to 4 fail; the plan returns the new entry kinds as stubs
   - Tests: `TestApplyInstallsInterfaceSPDCatchAllAndGuard`, `TestApplyInstallsInterfaceSPDCatchAllUnderBypass`, `TestAddressEventFollowsTheGuard`, `TestChildSAMaintainsTheGuardTable`
2. **Phase: Interface SPD catch-all** - builder at an if_id, plan, install, prune, release, ledger
   - Tests: `TestInterfaceSPDCatchAllEntries`, `TestReleaseRemovesInterfaceSPDAndGuard`, `TestRFC4301InterfaceSPDDiscardsUnmatchedInKernel`, `TestInterfaceSPDBypassCarriesNoCleartextInKernel`
3. **Phase: Inbound proof** - `TestRFC4301InboundSelectsTheInterfaceSPDInKernel`; code only if it is red for a reason other than the missing catch-all
4. **Phase: Core rule leaf** - move the encoder, generalise it, policyroute onto it, old copy deleted
   - Tests: `TestSourceRuleEncodesBothFamilies`; policyroute's existing unit and integration tests unchanged
5. **Phase: Source guard** - nft table, rules, table routes, order, AC-12 refusal
   - Tests: `TestXFRMInterfacePlanInstallOrder`, `TestGuardRefusesTunnelAddressAsIKEEndpoint`, `TestTunnelSourceGuardInKernel`
6. **Phase: Visibility** - role and SPD in the owner ledger, read join, show keys, `.ci`
   - Tests: `TestPolicyOwnersRecordRoleAndSPD`, `TestShowDataplanePolicyNamesSPDAndRole`, `dataplane-policy-spd-role`
7. **Phase: Conformance** - the tagged pair, `./le rfc record` for each, rows edited, both interop scenarios with their red walks, docs
8. **Phase: Application override control** - D8-1 to D8-4 answered (table 8.10): the leaf, the guard, the socket rows, the doctor checks, the tagged pair (the Phase 8 section)
   - Tests: the Phase 8 TDD tables

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No instant where a source BYPASS exists without its guard; the interface catch-all never ranks above a Child SA policy |
| Correctness | The interface catch-all uses the same if_id as the peer's Child SA policies (`resolveIfID`) |
| Naming | JSON keys kebab-case; role values declared once |
| Data flow | Interface facts read only through `iface`; nft only through `firewall.RegisterTables` |
| Rule: no-layering | policyroute's local encoder deleted in the commit that adds the leaf |
| Rule: rfc-compliance | Quote above each new implementing function and a section reference at each call |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Interface catch-all in the kernel | `TestRFC4301InterfaceSPDDiscardsUnmatchedInKernel` under `./le test qemu` |
| Guard | `TestTunnelSourceGuardInKernel` |
| Role and SPD in the listing | `./le test ipsec` running `dataplane-policy-spd-role` |
| RFC row covered | `./le rfc check` prints nothing for RFC4301-4.4.1-5 |
| Interop | `./le test integration interop-ipsec` for the two named scenarios |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail closed | A guard install failure leaves the source BYPASS uninstalled |
| Cleartext leak | Both source-selection paths covered by kernel tests with a vacuity leg |
| Foreign state | Guard tables, rules and the nft table carry Ze ownership marks; removal touches only Ze's own |
| Error leakage | Refusal messages name the address, interface and fix, nothing secret |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-2 or A-4 broken on 7.3 | Record the kernel behaviour; A-2 leaves the nft drop as the sole guard (spec edit), A-4 adds inbound code in phase 3 |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The kernel's if_id is the SPD-ID; the exemptions shipped in d0c2738a48 are the outbound selection function, and the kernel already selects inbound by interface.
- A source rule cannot steer IPv6 traffic whose source the kernel chooses: the route is chosen first and kept.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/kernel-value-outside-the-decoded-set.md` | 2026-11-08 | `policyInfoFromKernel` socket policy direction |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Interface catch-all is DISCARD under both `unmatched` values | Honour `unmatched` (approved brief) | A BYPASS at an interface if_id carries no cleartext (`xfrmi_xmit`, `__xfrm_policy_check`), so it would be listed as bypass while the kernel drops: a configured value reported as behaviour. Reported to Thomas |
| Guard = approved `ip rule` plus per-interface table, and a postrouting nft drop | Rule alone (approved brief) | The rule is not consulted when the kernel picks the source (`ip6_dst_lookup_tail`); the drop covers that path. The rule keeps the steering Thomas approved for socket-chosen sources. Reported to Thomas |
| Move policyroute's encoder to `internal/core/iprule` | Call policyroute; copy the encoder | IKE cannot import a gated plugin or reach its in-process state; a copy is a second declaration |
| Role and SPD recorded at install in the owner ledger | Infer from priority in the renderer | An operator `order` can equal any priority; the ledger already joins owner the same way |
| Guard for IPv6 only | Both families | IPv4 has no source exemption; IPv4 host flows already carry the output device (`ip_route_output_key_hash_rcu`) |

## Known Limitations

- RFC4301-4.4.1-8 stays a gap until Phase 8 lands (AC-22).
- A process that sets a socket policy carrying Ze's reserved priority (`PriorityESPFormSocket`) is listed as `esp-form-socket`: the kernel copies the priority as given (A8-4) and names no owner (F5). Under `deny` the guard refuses that process anyway, so only a policy set under `allow` or before the guard attached can copy the mark. Doctor still counts every socket policy, and reports the rows beyond the ones Ze holds as not attributable to Ze.
- A socket policy set before the guard attached, or while `allow` held, stays until its socket closes: no netlink request removes one (F5). Doctor names it under `deny`.
- A direct SPD write by another CAP_NET_ADMIN holder is outside the guard (D8-3 (a), F6): Linux treats every such holder as an administrator. Doctor names the holders.
- The VPP backend has no socket policies, so the guard concerns the kernel XFRM backend only. How VPP protects its API socket is unverified: Ze writes `socksvr { socket-name }` with no mode or group (`internal/component/vpp/startupconf.go`), and no VPP source checkout was available locally to read its default socket mode.
- RFC4301-4.4.1.1-5 is owned by `plan/release/fixes/spec-ipsec-spd-name-selection.md`.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| RFC4301-4.4.1-5 multi-SPD selection, the interface SPD catch-all, the source guard and the visibility (this design); RFC4301-4.4.1-8 application override control (Phase 8) | `plan/post-release/features/spec-ipsec-spd-selection.md` |
| RFC4301-4.4.1.1-5, mandatory named selection only | `plan/release/fixes/spec-ipsec-spd-name-selection.md` |

## RFC Documentation (Scope: protocol)

`rfc/short/rfc4301.md` rows RFC4301-4.4.1-5 and RFC4301-4.4.1-8. Every new
implementing function quotes the Section 4.4.1, 5.1 or 5.2 sentence it performs
(the table in Task), and each caller names the section.

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
- [ ] AC-1..AC-22 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped evidence for the packages and suites this change reaches (no agent runs `./le verify`)
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script
