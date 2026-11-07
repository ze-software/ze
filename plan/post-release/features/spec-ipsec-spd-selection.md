# Spec: ipsec-spd-selection

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P3 |
| Release reason | Ze already runs one SPD per XFRM interface if_id beside the node-wide SPD at if_id 0, so RFC 4301 Section 4.4.1 owes the selection function and each SPD its own final DISCARD entry; the shipped partial selection leaves one IPv6 cleartext path. RFC4301-4.4.1-8 (application override) stays owned here without a design. |
| Scope | protocol |
| RFC gaps | RFC4301-4.4.1-5, RFC4301-4.4.1-8 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-07 |

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

### RFC4301-4.4.1-8, retained without a design

RFC 4301 Section 4.4.1: "However, the system administrator MUST be able to specify
whether or not a user or application can override (default) system policies." Ze
has no user or application principal in its SPD model (`ipsec/spd_policy.go`
`SPDPolicy` carries no override leaf). This spec keeps owning the row, as the
migration assigned it, and the approved design does not cover it. The row stays a
gap after this work. Its design is owed in a phase of this spec, or in a spec of
its own, before this spec closes: the main thread asks Thomas which.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - SPD dispositions, the catch-all and the XFRM interface exemption
  → Decision: the catch-all is the first SPD entry in and the last out (`releaseSPD`); the exemptions go in before a re-asserted catch-all and come out after it
  → Constraint: a new entry kind follows the same install, prune and release path, and the SPD ledger (`spd_left.go`) hears every removal
- [ ] `docs/architecture/ike/ipsec-3-data-model.md` - the SPD data model, which describes one SPD
  → Constraint: the page is wrong once interface SPDs are named; it is edited in the same phase
- [ ] `docs/architecture/policyroute/policy-routing.md` - the `ip rule` writer
  → Decision: the rule encoder must start from `netlink.NewRule()` (`newIPRule`), and the table conversion is per architecture (`netlinkTableInt`)
  → Constraint: the auto table range is 2000 to 2999 and an operator table may use 1000 to 2999; a new owner of tables needs a range disjoint from both
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - nft table ownership
  → Constraint: a producer's table name carries the `ze_` prefix (Rule 2, enforced by `RegisterTables`); teardown belongs to the firewall engine (Rule 3); the registry is process-local
- [ ] `docs/architecture/core-design.md`, section 19 - allowed imports
  → Constraint: `ike` may import `iface` today, not `firewall`; the guard adds `firewall` and the new core leaf to the `ike` row
- [ ] `docs/architecture/iface/logical-name-resolution.md` - how a logical name becomes a device
  → Constraint: the guard and the role column resolve the interface through `iface`, never the kernel directly

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - rows RFC4301-4.4.1-5 and RFC4301-4.4.1-8
  → Constraint: both rows cite `plan/spec-ipsec-spd-selection.md`, a path that does not exist; the row edit names `plan/post-release/features/spec-ipsec-spd-selection.md`
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
| AC-10 | `./le rfc check` after this work | RFC4301-4.4.1-5 carries a tagged positive and negative unit, each with a discrimination record from `./le rfc record`, and no `{gap}`; RFC4301-4.4.1-8 keeps its gap and cites this spec's real path | 4.4.1-8 until its own design lands |
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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Interface catch-all is DISCARD under both `unmatched` values | Honour `unmatched` (approved brief) | A BYPASS at an interface if_id carries no cleartext (`xfrmi_xmit`, `__xfrm_policy_check`), so it would be listed as bypass while the kernel drops: a configured value reported as behaviour. Reported to Thomas |
| Guard = approved `ip rule` plus per-interface table, and a postrouting nft drop | Rule alone (approved brief) | The rule is not consulted when the kernel picks the source (`ip6_dst_lookup_tail`); the drop covers that path. The rule keeps the steering Thomas approved for socket-chosen sources. Reported to Thomas |
| Move policyroute's encoder to `internal/core/iprule` | Call policyroute; copy the encoder | IKE cannot import a gated plugin or reach its in-process state; a copy is a second declaration |
| Role and SPD recorded at install in the owner ledger | Infer from priority in the renderer | An operator `order` can equal any priority; the ledger already joins owner the same way |
| Guard for IPv6 only | Both families | IPv4 has no source exemption; IPv4 host flows already carry the output device (`ip_route_output_key_hash_rcu`) |

## Known Limitations

- RFC4301-4.4.1-8 stays a gap: no design yet (see Task).
- RFC4301-4.4.1.1-5 is owned by `plan/release/fixes/spec-ipsec-spd-name-selection.md`.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| RFC4301-4.4.1-5 multi-SPD selection, the interface SPD catch-all, the source guard and the visibility (this design); RFC4301-4.4.1-8 absent application-principal surface | `plan/post-release/features/spec-ipsec-spd-selection.md` |
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
- [ ] AC-1..AC-12 all demonstrated
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
