# Spec: bgp-epe-peer-segments-and-srv6-peernode -- EPE PeerAdj, PeerSet, per-peer control and SRv6 PeerNode

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The BGP child of the rfc-verdict pass (spec-rfc-verdict-fix-bgp, closed 2026-10-21; its
"Gap accounting" section survives in git history) closed with requirement rows that no spec owned.
One spec took them (`plan/immediate/spec-bgp-unowned-rfc-rows.md`). On 2026-10-11 the
owner put EPE PeerAdj/PeerSet IN, ruled that the RFC 9514 §7.2 rows are owned
"Only if EPE is in" (it is), and split the work: "Split into three". This spec owns
the EPE rows. The defects stay in the immediate spec; DelayOpen and Event 3 are
`plan/spec-bgp-fsm-delayopen-and-automatic-start.md`.

No existing spec could absorb these rows: the BGP-LS specs in the tree
(`plan/pre-release/spec-bgp-ls-origination-and-the-scheduled-marker.md`,
`plan/immediate/spec-bgp-ls-receiver-accept-and-limit-controls.md`,
`plan/immediate/spec-bgp-ls-receiver-fault-management.md`) own RFC 9552 node keys
and receiver fault handling, and none names RFC 9086, RFC 9514 §7.2 or the EPE
plugin (grep, 2026-10-11).

Row numbers are the numbers the rows had in the parent spec.

| # | Row id | Document, section | Class | What Ze does today |
|---|--------|-------------------|-------|--------------------|
| 8 | `RFC9086-7-2` (and `RFC9086-7-1`'s gap half) | RFC 9086 §7 | gap: feature absent, owner IN | Only PeerNode SID configuration exists; no PeerAdj or PeerSet SID, no per-recipient control |
| 11 | `RFC9514-7.2-5` | RFC 9514 §7.2 | gap half (`partial`): feature absent, owned because EPE is in | Decodes TLV 1251 flags and ignores reserved bits; originates no SRv6 BGP PeerNode SID TLV, so "set to 0 when originated" is reached by no layer |
| 12 | `RFC9514-7.2-6` | RFC 9514 §7.2 | gap half (`partial`): feature absent, owned because EPE is in | Same TLV, the 2-octet Reserved field: ignored on receipt, never originated |

The class column follows `ai/rules/rfc-compliance.md`: "a capability Ze does not
provide is an implementation gap". The owner scoped them in, so they are
implemented here.

### Bucket

`plan/`. PeerAdj, PeerSet, per-recipient control and SRv6 PeerNode are features
the release goes out without: the PeerNode SID Ze has today keeps working, and no
operator meets the absence as a bug. `plan/README.md` puts that at this level. The
owner's split put them here. Rows 11-12 bring a dataplane feature with them:
SRv6 End.X SID installation, which Ze has no writer for today (R-7).

### The RFC text, quoted from the source

| # | Quote | Read at |
|---|-------|---------|
| 8 | "The operator MUST be provided with the options of configuring, enabling, and disabling the advertisement of each of the PeerNode SID, PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer." | `rfc/full/rfc9086.txt` §7 |
| 11, 12 | "Other bits are reserved for future use and MUST be set to 0 when originated and ignored on receipt." and "Reserved: 2-octet field. The value MUST be set to 0 when originated and ignored on receipt." The section opens: "Similar Peer Node and Peer Set functionality can be realized with SRv6 using SIDs with END.X behavior." | `rfc/full/rfc9514.txt` §7.2 |

### How FRR and BIRD handle it

| # | FRR (`/Users/thomas/Code/github.com/FRRouting/frr`) | BIRD (`/Users/thomas/Code/gitlab.nic.cz/labs/bird`) |
|---|------|------|
| 8 | No PeerAdj or PeerSet SID in `bgpd/`, and no BGP-LS in `bgpd/` at all (no TLV 1102 anywhere), so FRR cannot receive them | None |
| 11, 12 | No BGP-LS origination | None |

Neither can be the interop receiver; GoBGP decodes the TLVs (R-8).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/nlri-bgpls.md` - the BGP-LS NLRI and attribute TLVs Ze encodes
  → Constraint: rows 8, 11 and 12 add PeerAdj (TLV 1102), PeerSet (TLV 1103) and SRv6 PeerNode (TLV 1251) origination; the page edit lands with the EPE phase
- [ ] `docs/architecture/testing/interop.md` - scenario naming and the revert-and-red walk
  → Constraint: every new scenario is named (no numeric prefix) and owes a recorded red with the fix reverted and the artifact rebuilt

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9086.md` - rows 7-1 (partial, line 372) and 7-2 (line 373)
  → Constraint: 7-2 is the R3 split of 7-1's sentence; implementing 7-2 (owner: IN) also closes 7-1's gap half, so both rows change in the EPE phase
- [ ] `rfc/short/rfc9514.md` - rows 7.2-5 (line 491) and 7.2-6 (line 493)
  → Constraint: both are `partial`, receipt tested; the origination half needs a live SRv6 EPE path that emits TLV 1251, which only the SRv6 phase here provides (owner: "Only if EPE is in"; EPE is in)

**Key insights:**
- Per-recipient control is an egress filter on the BGP-LS export, not a change to the SIDs themselves.
- An SRv6 EPE SID is only honest while it forwards, so it is advertised only while installed as End.X, the rule the MPLS PeerNode label already follows.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/epe/epe_config.go::ParseConfig`, `internal/component/bgp/plugins/ls_export/export_config.go::parseExportConfig` - PeerNode only, no per-recipient control (per the rfc/short annotation; re-read at implementation)
- [ ] `internal/component/bgp/plugins/epe/yang/ze-bgp-epe-conf.yang` - container `bgp-epe` (`srgb`, `list peer { address, sid-index, weight }`); PeerNode SID over MPLS only
- [ ] `internal/component/bgp/plugins/nlri/ls/attr_srv6.go` - `lsSRv6BGPPeerNodeSID` encodes and decodes TLV 1251; no producer originates it. Package-private since route D2 of spec-plugin-ownership-imports (owner, 2026-10-16): an originator outside the plugin exports it again or reaches it through the registered decoder
- [ ] `internal/plugins/fib/` - no `seg6local` writer: Ze installs no SRv6 End.X SID today (R-7). `fib/kernel/nexthop_linux.go` builds only a `seg6` encap (headend, `buildSEG6Encap`), never a `seg6local` route; `fib/vpp/srv6.go` programs SRv6 encap, no localsid
- [ ] `internal/component/bgp/plugins/epe/epe_source.go` - the EPE source publishes MPLS label actions through `internal/core/mplsfib` as label source 5 (`epeMPLSSource`, line 26; `RemoveLabelSource` and `emitLabel`, lines 150-162). That contract is MPLS-only: `mplsfib.Entry` carries an in-label, out-labels and a next hop, no IPv6 SID and no behaviour
- [ ] `internal/core/mplsfib/doc.go`, `events.go` - the leaf event package that keeps fib-kernel "the single programmer of kernel forwarding state"; the shape the End.X contract follows
- [ ] `internal/plugins/fib/kernel/kernelcap_linux.go` - fib-kernel registers its kernel requirement with `kernelcap.MustRegister` (Subsystem, Kernel config symbol, ConfigLeaf, InUse predicate, Probe); `internal/component/kernelcap/predicate.go` `MPLSInUse` gates on `fib { kernel { } }` so a VPP or P4 config is never refused for a kernel table it does not use
- [ ] `internal/component/bgp/plugins/ls_export/export_epe.go` - already admits TLVs 1101, 1102 and 1103 in native link attributes (line 59), but refuses any native EPE link without a PeerNode SID (lines 80-82, "native EPE link requires a PeerNode SID"), which rejects a PeerAdj Link NLRI
- [ ] `internal/component/bgp/plugins/ls_export/export_encode.go` lines 166-185 - already originates SRv6 SID NLRIs from a link-state snapshot and refuses one without Endpoint Behavior TLV 1250
- [ ] `internal/component/bgp/plugins/ls_export/export_state.go` lines 295-359 - the exporter already sends per peer (`topologyExporter.send(ctx, peer, ...)`), so a recipient filter fits its peer loop
- [ ] Linux `net/ipv6/seg6_local.c` (torvalds/linux master, read 2026-10-11): the End.X action requires `SEG6_LOCAL_NH6` and takes `SEG6_LOCAL_OIF` as optional; the state holds one `struct in6_addr nh6`, and `input_action_end_x_finish` calls `seg6_lookup_any_nexthop(skb, &slwt->nh6, 0, false, slwt->oif)`. One End.X carries exactly one next hop: no multipath inside the action

**Behavior to preserve:**
- PeerNode SID configuration and its tests (RFC9086-7-1 tested half).
- TLV 1251 receipt and its reserved-bit tests (RFC9514-7.2-5/-6 receipt halves).

**Behavior to change:** rows 8, 11 and 12, as the ACs state.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `bgp-epe` adjacency, peer-set, advertise, advertise-to and SRv6 SID leaves (rows 8, 11, 12).
- Session state: a peer session (PeerNode, PeerSet member) or its interface (PeerAdj) going up or down.

### Transformation Path
1. Config parse in the EPE plugin into PeerNode, PeerAdj and PeerSet state.
2. Label install (MPLS) through `mplsfib` as today, while the session or interface is up. A PeerSet label needs several next hops (round-2 I2): `mplsfib.Entry` holds one `NextHop` (`internal/core/mplsfib/events.go`) and fib-kernel keys swaps by in-label and replaces on re-add (`mplsSwaps`, `internal/plugins/fib/kernel/mplsentry.go`), so today the label could point at one member only. The contract changes, replacing rather than adding beside (`ai/rules/no-layering.md`): `Entry.NextHop` becomes `Entry.NextHops`, one or more next hops sharing the entry's `OutLabels`. The producers are RSVP-TE (`internal/plugins/rsvpte/fib.go`, `busFIB`), LDP (`internal/plugins/ldp/fib.go`, `ldpFIB`: push, pop and their removes), OSPF SR (`internal/plugins/ospf/sr_fib.go`, `srFIB`: push, swap, pop and their removes) and EPE (`epe_source.go`) (`gopls references` on `Entry.NextHop`, 2026-10-11, round-4 I1); RSVP-TE, LDP and OSPF SR set one next hop or none. The rule per `Op` applies to `ActionAdd` only (round-4 I1): a remove is keyed by FEC (push) or in-label (swap, pop) and carries no next hop, as every producer's remove does today. An add with `OpPush` takes exactly one next hop and is refused at apply otherwise; `OpSwap` takes one or more; `OpPop` takes zero or more, zero being pop then IP lookup, which LDP's egress `ProgramPop` emits ("forwarded by normal IP lookup", `ldp/fib.go`) and fib-kernel installs today (`addMPLSSwap` adds a via only `if nextHop.IsValid()`, `mplsentry_linux.go`). The one consumer, fib-kernel (`mplsentry.go` and `mplsentry_linux.go` for swap and pop, `mplscontext.go` for push), writes one `RTA_VIA` route for one next hop and an `RTA_MULTIPATH` route for several, each nexthop with its own `RTA_VIA` and `RTA_NEWDST` (Linux `net/mpls/af_mpls.c` `mpls_nh_build_multi` reads `RTA_NEWDST` per nexthop and `mpls_select_multipath` hashes across them; `vishvananda/netlink` `NexthopInfo` carries `NewDst` and `Via`). The EPE source re-publishes the PeerSet entry with the current up members on every member up or down. The PeerAdj label's next hop is the adjacency's `neighbor-address` on its `interface` (RFC 9086 §5.2 "Next-Hop: the interface peer address").
3. End.X install (SRv6) through a NEW leaf event package `internal/core/srv6localsid`, the SRv6 sibling of `mplsfib`: an `Entry` carries Action (add, remove, remove-source), Source (the EPE source id), the SID (`netip.Addr`, /128), the Behavior (End.X, RFC 8986 endpoint behaviour 5) and a list of next hops, each an IPv6 address plus an optional output interface. The EPE source is its first producer; fib-kernel is its only consumer, so fib-kernel stays the single programmer of kernel forwarding state.
4. fib-kernel's writer (`internal/plugins/fib/kernel/srv6localsid_linux.go`) installs the SID as an IPv6 /128 route with a `seg6local` lightweight-tunnel encap, action End.X, `nh6` the peer address and `oif` the session's interface (required for a link-local next hop). A PeerNode SID has one next hop. A PeerSet SID has one per member: since Linux End.X takes exactly one `nh6`, the PeerSet SID is installed as one multipath IPv6 route whose every nexthop carries its own `seg6local` End.X encap toward one member, which is A-2. Linux refuses a device-only nexthop on the IPv6 multipath API (`net/ipv6/route.c`, `ip6_route_multipath_add` path: "Device only routes can not be added for IPv6 using the multipath API.", raised when a nexthop has no `RTA_GATEWAY` or uses a nexthop object), so each PeerSet nexthop is `via <member address> dev <member interface> encap seg6local action End.X nh6 <member address> oif <member interface>` (round-2 I1). The gateway is validated on-link through the nexthop's device, so a member that is not on-link (multihop EBGP) cannot be a PeerSet nexthop: it is left out of the route and one Warn names the member, the set and the reason; when no on-link member is up the SRv6 PeerSet SID is neither installed nor advertised, and the Warn says so.
5. The writer acknowledges each batch as `mplsfib` does; the EPE source advertises a SRv6 SID only after the install is acknowledged and withdraws it before removing the route.
6. BGP-LS export: TLV 1101 on a PeerNode Link NLRI; TLV 1102 on a PeerAdj Link NLRI; TLV 1103 inside the attribute of every PeerNode or PeerAdj Link NLRI of each member (RFC 9086 §5.3, PeerSet has no NLRI of its own); an SRv6 SID NLRI carrying TLV 1250 (End.X) and TLV 1251.
7. Per-recipient egress filter on the BGP-LS export from `advertise-to`, applied inside the exporter's existing per-peer send loop (`export_state.go`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| EPE plugin to FIB (MPLS) | `mplsfib` label actions, source 5, as today | Yes (`epe_source.go`) |
| EPE plugin to FIB (SRv6) | `srv6localsid` End.X entries on the event bus, acknowledged by fib-kernel | No (new contract) |
| fib-kernel to Linux | netlink `RTM_NEWROUTE` with `LWTUNNEL_ENCAP_SEG6_LOCAL`, End.X, `nh6`, `oif`; a PeerSet adds `RTA_MULTIPATH` with `RTA_GATEWAY` per nexthop (A-2); the MPLS PeerSet label an `AF_MPLS` `RTA_MULTIPATH` route | No |
| fib-vpp | consumes no `srv6localsid` entry; a `srv6-sid` with `fib { vpp { } }` is refused at commit | No |
| EPE plugin to BGP-LS export | SID state per peer, adjacency, set | No |
| BGP-LS export to peers | egress filtered per recipient | No |

### Integration Points
- `ls_export` - the exporter already admits TLVs 1102 and 1103 and already originates SRv6 SID NLRIs with TLV 1250; what changes is `export_epe.go`'s PeerNode-required check (a PeerAdj Link NLRI carries TLV 1102 and no PeerNode SID), and the recipient filter in the per-peer send loop. The gap is in the EPE source and its config, which produce none of these today.
- `internal/core/srv6localsid` (new) - the End.X contract, owned by core as `mplsfib` is.
- `internal/plugins/fib/kernel/` - the End.X writer and a `kernelcap` registration for `CONFIG_IPV6_SEG6_LWTUNNEL` with an `SRv6EndXInUse` predicate (EPE `srv6-sid` configured on the kernel FIB backend), probed by installing an End.X route in the probe namespace; the doctor check comes from that registration.
- `internal/component/kernelcap/predicate.go` - `SRv6EndXInUse`, gated on `fib { kernel { } }` as `MPLSInUse` is.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be shown at implementation |
| No unintended coupling (components stay isolated) | No | the EPE plugin reaches the FIB through event contracts (`mplsfib` for labels, the new `srv6localsid` for End.X), never by importing fib-kernel |
| No duplicated functionality (extends existing, does not recreate) | No | PeerAdj and PeerSet extend the PeerNode path |
| Zero-copy preserved where applicable (refs, not copies) | No | TLVs written with the existing `WriteTo` encoders |
| Registration over hardcoding, outbound | No | to be shown at implementation |
| Registration over hardcoding, inbound | No | to be shown at implementation |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | GoBGP v3.31.0 decodes TLVs 1101-1103 and GoBGP v4 decodes TLV 1251 | GoBGP source read in the parent spec (R-8) | the interop receiver changes, and the scenario README says why | `gobgp global rib -a ls` against Ze in each scenario | holds by source reading (2026-10-11) |
| A-2 | Linux load-balances a PeerSet End.X SID when the SID is one multipath IPv6 route whose every nexthop carries a gateway (`via <member>`) and its own `seg6local` End.X encap | Sources fetched 2026-10-11 (torvalds/linux master, iproute2 main), read by the round-2 review and rechecked here: `seg6_local.c` End.X requires `SEG6_LOCAL_NH6`, takes `SEG6_LOCAL_OIF` as optional, and holds one `nh6`; `route.c` `ip6_route_multipath_add` reads `RTA_ENCAP` and `RTA_ENCAP_TYPE` from each `rtnexthop` into that path's config, and `ip6_rt_init_dst` takes the lwtstate of the nexthop `fib6_select_path` chose (`res->nh->fib_nh_lws`), so the selected path's End.X is the one applied; the same file refuses a nexthop with no gateway ("Device only routes can not be added for IPv6 using the multipath API."), so per-nexthop encap holds only with `RTA_GATEWAY` on every path; iproute2 `ip/iproute.c` `parse_one_nh` accepts `encap` inside each `nexthop` | PeerSet SRv6 forwarding cannot load-balance on Linux. Then the SRv6 PeerSet half stops and the owner is asked which way to fix it (install toward one member and set weight accordingly, or refuse `srv6-sid` on a `peer-set` at commit naming the kernel limit); MPLS PeerSet and SRv6 PeerNode are unaffected | first step of the SRv6 phase: a `needs-linux:caps=net-admin` test installs the route on two dummy links with two neighbours, sends flows with distinct hashes, and reads each link's counters | holds by source reading (2026-10-11) with `RTA_GATEWAY` on every path; the counter test stays the behavioural proof (AC-5) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-7 | Rows 11-12 originate SRv6 PeerNode/PeerSet SIDs, which RFC 9514 §7.2 realises "using SIDs with END.X behavior"; Ze installs no SRv6 End.X SID today (no `seg6local` writer in `internal/plugins/fib/`) | an advertised SRv6 EPE SID that forwards nothing | The MPLS writer cannot carry it (`mplsfib` is label-keyed), so the SRv6 phase adds the `srv6localsid` contract and the fib-kernel End.X writer (Data Flow steps 3-5) and advertises a SID only once its install is acknowledged, the rule the MPLS PeerNode label follows. On `fib { vpp { } }` (and P4) a `srv6-sid` is refused at commit: "bgp-epe peer <address> srv6-sid: the VPP FIB backend installs no SRv6 End.X SID, so the SID would be advertised with nothing forwarding it; use fib kernel or remove srv6-sid". On the kernel backend without `CONFIG_IPV6_SEG6_LWTUNNEL` the `kernelcap` registration refuses it at commit and `ze doctor` names the missing kernel option |
| R-8 | GoBGP decodes TLV 1251 only from v4.0.0, and the interop image pins v3.31.0 | the SRv6 PeerNode scenario cannot read the TLV | The scenario for rows 11-12 builds a GoBGP v4 image beside the v3 one; TLVs 1101-1103 are decoded since v2.1.0, so the MPLS PeerAdj/PeerSet half runs on the existing image |

R-7 and R-8 keep the ids they had in the parent spec.

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Rows 8, 11, 12: EPE labels and SIDs installed or advertised wrongly, or advertised to a peer `advertise-to` excludes; an SRv6 SID advertised with no End.X behind it |
| How is it reverted? | Each phase lands as its own commit, revertible alone |
| Who else touches this path? | `plan/immediate/spec-bgp-unowned-rfc-rows.md` row 10 changes the byte order of every MP UPDATE, BGP-LS included: whichever lands second writes its expectations against the other. `plan/pre-release/spec-bgp-ls-origination-and-the-scheduled-marker.md` (BGP-LS node keys), `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` (ledger rows; its BGP child spec-rfc-verdict-fix-bgp closed 2026-10-21) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `bgp-epe peer <address> adjacency <interface>` and `bgp-epe peer-set <name>` | → | EPE plugin state, BGP-LS export of TLVs 1102 and 1103 | `test/plugin/bgp-epe-peer-adj-set-export.ci` |
| Config `bgp-epe ... advertise false` and `advertise-to <peer>` | → | per-SID enable/disable and per-recipient egress control | `test/plugin/bgp-epe-advertise-control.ci` |
| Config `bgp-epe peer <address> srv6-sid <sid>` | → | `srv6localsid` entry, fib-kernel `seg6local` End.X route, TLV 1251 originated with Flags reserved bits and Reserved zero | `test/plugin/bgp-epe-srv6-peernode-origination.ci` |
| Config `srv6-sid` with `fib { vpp { } }` | → | commit refusal | `test/plugin/bgp-epe-srv6-vpp-refused.ci` |

## Acceptance Criteria

AC numbers changed in the split: AC-1 was AC-8 in the parent spec, AC-2 and AC-3
were AC-12 and AC-13, and AC-4 is the parent's ledger AC-11 for the `rfc9086` and
`rfc9514` stems.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 (row 8) | `bgp-epe` configures a PeerAdj SID (`peer <address> adjacency <interface>`) and a PeerSet SID (`peer-set <name>`), each with `advertise` and `advertise-to` (owner: IN) | Each of PeerNode, PeerAdj and PeerSet can be configured, enabled and disabled independently; the BGP-LS export carries TLV 1101, 1102 and 1103 for the enabled ones only, and the forwarding label of each is installed while its session (PeerAdj: its interface; PeerSet: any member) is up. `advertise-to` limits a SID's BGP-LS route to the named BGP peers, internal or external, and an empty list sends it to every BGP-LS peer; a disabled or filtered SID is withdrawn from a peer that had it. RFC9086-7-1's gap half closes with 7-2 |
| AC-1a (row 8, PeerAdj wire shape) | `bgp-epe peer <address> adjacency <interface> neighbor-address <ip>` with the session up | One BGP-LS Link NLRI per adjacency: local node descriptors Local BGP Router-ID (TLV 516) and Local ASN (TLV 512), remote node descriptors the peer's BGP ID and ASN, link descriptor TLV 258 with the interface's ifindex as Link Local Identifier and 0 as Link Remote Identifier (RFC 9086 §5.2 "The value 0 is used by default when the link remote identifier is unknown"), plus TLV 259/261 (the adjacency interface's own address on that link, which for a multihop session is not the session's source address) and 260/262 (`neighbor-address`); the attribute carries TLV 1102 and no TLV 1101, and `export_epe.go` no longer refuses it. The PeerAdj label is installed with next hop `neighbor-address` via `interface` |
| AC-1b (row 8, PeerSet placement) | `bgp-epe peer-set <name> member <a> member <b>` with both sessions up | No NLRI of its own: TLV 1103 with the same SID value is carried in the BGP-LS attribute of every PeerNode and PeerAdj Link NLRI of each up member (RFC 9086 §5.3); when a member goes down its NLRIs go, and the set's label stays installed while any member is up. Forwarding (round-2 I2): the PeerSet label is ONE kernel MPLS route whose multipath nexthops are every up member, so traffic load-balances across them as RFC 9086 §5.3 intends; a member going down removes its nexthop, the last one removes the label. Read back through netlink in a `needs-linux:caps=net-admin` test: one `RTA_MULTIPATH` entry per up member, each with its `via` |
| AC-1c (config, SRv6-only, owner 2026-10-11) | A `bgp-epe` block whose peers carry only `srv6-sid` | Commit accepts it: `srgb` and `sid-index` are optional. Each `peer`, `adjacency` and `peer-set` entry MUST carry at least one of `sid-index` (MPLS) or `srv6-sid` (SRv6), and an entry with neither is refused naming the entry and both leaves ("bgp-epe peer 192.0.2.1: no SID configured; set sid-index (MPLS, needs srgb) or srv6-sid (SRv6)"). A `sid-index` with no `srgb` is refused naming `srgb`. A `srv6-sid` on a peer whose address is IPv4, or on a `peer-set` with an IPv4 member, is refused, because End.X forwards to an IPv6 `nh6` and Ze has no IPv6 address of that peer it could know to be right (round-2 I3): "bgp-epe peer 192.0.2.1 srv6-sid: End.X needs an IPv6 next hop and this peer is IPv4; remove srv6-sid, or configure the EPE peer by its IPv6 session address" |
| AC-2 (row 11, `RFC9514-7.2-5`) | `bgp-epe peer <address> srv6-sid <sid>` (or the same leaf on a `peer-set`) is configured and the session is up | Ze installs the SID as an End.X SID (AC-5), and once the install is acknowledged the BGP-LS export originates one SRv6 SID NLRI whose attribute carries TLV 1250 with the End.X endpoint behaviour and TLV 1251. A PeerNode SID carries one TLV 1251 (Peer AS, Peer BGP Identifier of that peer, S clear); a PeerSet SID carries one TLV 1251 per up member, each with that member's Peer AS and Peer BGP Identifier and the S flag set (RFC 9514 §7.2 "multiple instances of this TLV (one for each peer in the "peer set") are associated with the SRv6 SID and the S-Flag is set"). B and P as configured, every other Flags bit zero; the reserved-bit receipt half stays tested; with the session (PeerSet: every member) down the SID is withdrawn and then uninstalled |
| AC-3 (row 12, `RFC9514-7.2-6`) | The same origination | The 2-octet Reserved field of TLV 1251 is zero on the wire, asserted from the originated bytes, not from an offline encoder; the receipt half stays tested |
| AC-5 (End.X installed) | AC-2's config on the kernel FIB | The kernel holds an IPv6 route for the SID /128 with `encap seg6local action End.X nh6 <peer address> oif <session interface>`, installed on the session interface (`dev`); for a multihop PeerNode, whose `nh6` is a global address reached by lookup, `oif` is omitted (`seg6_lookup_any_nexthop` resolves it) and the route is installed on the device the kernel route to the peer address resolves to; a PeerSet SID holds one nexthop per up on-link member, each read back with its gateway, device and encap: `via <member address> dev <member interface> encap seg6local action End.X nh6 <member address> oif <member interface>` (A-2). A member that is not on-link is absent from the route and one Warn names it; with no on-link member up, no route and no advertised SRv6 PeerSet SID. Asserted by reading the kernel route back through netlink in a `option=needs-linux:caps=net-admin` test, never from the request Ze built; with the session down the route is gone |
| AC-6 (VPP refusal) | `srv6-sid` configured with `fib { vpp { } }` | Commit refused with R-7's message naming the leaf, the reason and the fix; no SID is advertised |
| AC-7 (capability) | `srv6-sid` on the kernel FIB, kernel without `CONFIG_IPV6_SEG6_LWTUNNEL` (probe fails) | Commit refused through the `kernelcap` registration, and `ze doctor` reports the missing kernel option with the fix; with the option present the probe passes and no diagnostic is shown |
| AC-4 (ledger) | Each row once its AC is met | `RFC9086-7-2` loses `{gap}`, `RFC9086-7-1` and `RFC9514-7.2-5/-6` leave `partial`, each carries tags in both polarities and a discrimination record (`./le rfc discriminate-record`); `./le rfc check` exits clean for `rfc9086` and `rfc9514` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures PeerAdj and PeerSet SIDs and limits who receives them | config -> EPE plugin -> label install -> BGP-LS export -> per-recipient egress | `bgp-epe-peer-segments-gobgp` |
| 2 | Configures an SRv6 PeerNode SID | config -> End.X install -> TLV 1251 in BGP-LS export | `bgp-epe-srv6-peernode-gobgp` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEPEPeerAdjPeerSetConfig` | `internal/component/bgp/plugins/epe/` | AC-1 config parse, enable/disable, refusals | |
| `TestEPEAdvertiseToFiltersRecipients` | `internal/component/bgp/plugins/epe/` or `ls_export/` | AC-1 per-recipient control both polarities | |
| `TestEPESRv6PeerNodeTLVReservedZero` | `internal/component/bgp/plugins/epe/` | AC-2, AC-3 from the originated bytes, PeerNode and PeerSet (one TLV 1251 per member, TLV 1250 present) | |
| `TestEPEPeerAdjLinkNLRIShape`, `TestEPEPeerSetTLVOnMemberLinks` | `internal/component/bgp/plugins/epe/`, `ls_export/` | AC-1a, AC-1b from the exported bytes; `export_epe.go` accepts a PeerAdj-only link | |
| `TestEPEConfigSIDRequired` | `internal/component/bgp/plugins/epe/` | AC-1c: SRv6-only accepted, neither refused, `sid-index` without `srgb` refused | |
| `TestMPLSEntryNextHopsRulePerOp` | `internal/plugins/fib/kernel/mplsentry_test.go` (new file) | round-3 I1: an `ActionAdd` `OpPush` entry with zero or two next hops is refused at apply with the FEC, the count and the reason, and installs nothing; with one it installs as today; `OpSwap` accepts one and several, and an `ActionAdd` `OpSwap` with zero next hops is refused at apply and installs nothing, where today `addMPLSSwap` installs it with no via (round-5 N2, the rule's negative polarity); `OpPop` accepts one and several, and with none installs as pop then IP lookup (no via, LDP's egress); an `ActionRemove` with no next hop, for push by FEC and for swap and pop by in-label, is accepted and removes the entry (round-4 I1) | |
| `TestMPLSPeerSetMultipathKernelState` | `internal/plugins/fib/kernel/` (`needs-linux:caps=net-admin`) | AC-1b forwarding: a two-member PeerSet label reads back as one route with two `RTA_MULTIPATH` nexthops, each with its `via`; one member down leaves one nexthop; the last down removes the label; an RSVP-TE single-next-hop entry still reads back as a plain `RTA_VIA` route | |
| `TestEPESRv6SIDRefusedOnIPv4Peer` | `internal/component/bgp/plugins/epe/` | AC-1c, round-2 I3: `srv6-sid` on an IPv4 peer, and on a `peer-set` with an IPv4 member, refused naming the leaf, the reason and the fix; on an IPv6 peer accepted | |
| `TestSRv6PeerSetOffLinkMemberOmitted` | `internal/plugins/fib/kernel/` (`needs-linux:caps=net-admin`) | AC-5, round-2 I1: a member whose address is not on-link is absent from the multipath route with one Warn; with no on-link member, no route and no advertised SID | |
| `TestSRv6LocalSIDEndXKernelState` | `internal/plugins/fib/kernel/` (`needs-linux:caps=net-admin`) | AC-5: route read back by netlink, single and multipath (A-2), removed on withdraw | |
| `TestSRv6EndXCapabilityRefusal`, `TestEPESRv6RefusedOnVPP` | `internal/component/kernelcap/`, `internal/component/bgp/plugins/epe/` | AC-6, AC-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `bgp-epe ... adjacency ... sid-index`, `peer-set ... sid-index` | 0..1048559, as `peer sid-index` | 1048559 | N/A | 1048560 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-epe-peer-adj-set-export` | `test/plugin/` | AC-1 | |
| `bgp-epe-advertise-control` | `test/plugin/` | AC-1 per-SID and per-recipient | |
| `bgp-epe-srv6-peernode-origination` | `test/plugin/` | AC-2, AC-3, AC-5 (`option=needs-linux:caps=net-admin`, dummy link and a seeded neighbour, the shape `test/plugin/fib-mpls-kernel.ci` uses; reads the installed `seg6local` route) | |
| `bgp-epe-srv6-vpp-refused` | `test/plugin/` | AC-6 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-epe-peer-segments-gobgp` | `test/interop/scenarios/` | GoBGP v3.31.0 (decodes TLVs 1101, 1102, 1103 since v2.1.0) | GoBGP, a BGP-LS receiver, shows Ze's PeerNode, PeerAdj and PeerSet SIDs in `gobgp global rib -a ls`; a SID set to `advertise false`, or whose `advertise-to` excludes GoBGP, is absent or withdrawn. FRR and BIRD cannot be the receiver: neither has BGP-LS | |
| `bgp-epe-srv6-peernode-gobgp` | same | GoBGP v4 (decodes TLV 1251 since v4.0.0; R-8) | GoBGP shows the SRv6 PeerNode SID TLV with reserved Flags bits and Reserved zero | |

Every scenario owes the revert, rebuild, red, restore, green walk with the red recorded.

## Files to Modify
- `internal/component/bgp/plugins/epe/yang/ze-bgp-epe-conf.yang` - rows 8, 11, 12, new revision: under `bgp-epe/peer`, leaf `advertise` (boolean, default true: the PeerNode SID), leaf-list `advertise-to` (BGP peer address), leaf `srv6-sid` (IPv6 address, End.X); new `bgp-epe/peer/adjacency` list keyed by `interface` with `sid-index` (mandatory, `0..1048559`), `weight`, `advertise`, `advertise-to`; new `bgp-epe/peer-set` list keyed by `name` with `sid-index`, `weight`, leaf-list `member` (leafref to `../../peer/address`, min 1), `advertise`, `advertise-to`, `srv6-sid`
- `internal/component/bgp/plugins/epe/epe_config.go`, `epe_plugin.go`, `epe_source.go` - rows 8, 11, 12
- `internal/component/bgp/plugins/ls_export/export_config.go`, `export_state.go` (per-recipient filter in the per-peer send loop), `export_epe.go` (lines 80-82: a PeerAdj-only Link NLRI no longer refused) - row 8
- `internal/component/bgp/plugins/epe/yang/ze-bgp-epe-conf.yang` - also: `srgb` bounds and every `sid-index` become optional, `adjacency` gains mandatory `neighbor-address` (IP address of the peer interface on that link, RFC 9086 §5.2 Next-Hop), and the "at least one of `sid-index` or `srv6-sid`" rule is a commit validator (AC-1c)
- `internal/plugins/fib/kernel/` - rows 11, 12: `register.go` subscribes to `srv6localsid` entries; `kernelcap_linux.go` gains the End.X capability registration
- `internal/component/kernelcap/predicate.go`, `internal/component/kernelcap/probe_linux.go` - `SRv6EndXInUse` and the End.X probe (AC-7)
- `internal/le/interoplab/bgp/check_special.go` - map entries for `bgp-epe-peer-segments-gobgp` and `bgp-epe-srv6-peernode-gobgp`
- `internal/le/interoplab/bgp/run.go` (line 114, the image list beside `Dockerfile.gobgp`) - a `gobgp4-interop` image entry for the v4 Dockerfile (R-8), keyed `Name: peerGoBGP4` as the `gobgp-interop` entry is keyed `peerGoBGP`; `names.go` gains `peerGoBGP4 = "gobgp4"` beside `peerGoBGP`, and `bgp-epe-srv6-peernode-gobgp` names the `gobgp4` peer, so its checks query that container (round-3 N1)
- `docs/architecture/doctor-and-health-checks.md` - the new kernel capability row
- `docs/architecture/fib/fib-depth-4-srv6.md` - fib-kernel now also installs `seg6local` End.X SIDs from `srv6localsid`
- `docs/architecture/wire/nlri-bgpls.md` - rows 8, 11, 12
- `rfc/short/rfc9086.md`, `rfc/short/rfc9514.md` - AC-4
- `test/interop/Dockerfile.gobgp` or a sibling GoBGP v4 Dockerfile - R-8
- `internal/core/mplsfib/events.go` (and `doc.go`) - row 8, round-2 I2: `Entry.NextHop` replaced by `Entry.NextHops` (sharing `OutLabels`), with its rule per `Op` for `ActionAdd` only (round-3 I1, round-4 I1): `OpPush` takes exactly one next hop, because a multipath IP route with per-path MPLS encap is not designed here, and an add with none or several is refused at apply; `OpSwap` takes one or more; `OpPop` takes zero (pop then IP lookup) or more; an `ActionRemove` is keyed by FEC or in-label and carries none
- `internal/plugins/ldp/fib.go` (`ldpFIB`: the push entry moved to a one-element `NextHops`; the egress pop keeps an empty `NextHops`; the removes carry none) and `internal/plugins/ospf/sr_fib.go` (`srFIB`: push, swap and pop entries moved to a one-element `NextHops`; the removes carry none) - the LDP and OSPF SR `mplsfib.Entry` producers (round-4 I1)
- `internal/plugins/rsvpte/fib.go` (`busFIB`: the push, swap, swap-stack and pop entries and their removes) - an `mplsfib.Entry` producer, moved to a one-element `NextHops`; `build.go`, `reservation.go`, `engine.go` and `mtu.go` use `mplsfib` only for `PathMTUMinimum`, and their `NextHop` fields are RSVP objects and `PathRoute`, so they do not change
- `internal/plugins/fib/kernel/mplsentry.go` (swap and pop: `addMPLSSwap(e.InLabel, e.OutLabels, e.NextHop, ...)` takes the set; the `OpPush` arm refuses an entry whose `NextHops` is not exactly one, naming the FEC, the count and the reason) and `mplsentry_linux.go` - one `RTA_VIA` route for one next hop, an `RTA_MULTIPATH` route for several; replace-on-re-add keeps the whole nexthop set
- `internal/plugins/fib/kernel/mplscontext.go` (the push route built as `RichRoute{... NextHop: e.NextHop ...}`) - read `NextHops[0]` of a push add, the one element the rule allows; `mplscontext_linux.go` reads the `RichRoute`, not the entry, and does not change for this reason (round-4 I1)
- test readers and writers of `Entry.NextHop`, moved to `NextHops`: `internal/component/bgp/plugins/ls_export/rfc9086_epe_test.go`, `internal/plugins/rsvpte/fib_test.go`, `internal/plugins/ldp/fib_test.go`, the ten OSPF tests `internal/plugins/ospf/sr_fib_test.go`, `sr_install_test.go`, `sr_adjsid_test.go`, `rfc8665_test.go`, `rfc8665_mapped_sid_spf_test.go`, `rfc8665_mapped_sid_intra_spf_test.go`, `rfc8665_mapped_sid_php_test.go`, `rfc8665_nexthop_flags_test.go`, `rfc8666_test.go`, `rfc8666_sr_reception_v6_test.go`, the fib-kernel unit tests `mplsapply_test.go` and `rfc3032_mplsentry_test.go` (round-4 I1; the `rfc*` files are RFC-tagged, so the field rename in them is a mechanical change that keeps every assertion and goes through `rfc-change-ok` at commit), and the fib-kernel integration tests `mplscontext_integration_linux_test.go`, `mplsowner_integration_linux_test.go`, `mplsmtu_integration_linux_test.go`, `mplsentry_integration_linux_test.go` and `mplstransitmtu_integration_linux_test.go` (round-5 N1: `gopls` on darwin does not see `//go:build integration && linux` files, and grep finds `mplsfib.Entry` literals with `NextHop:` in the last two; `ownership_integration_linux_test.go` was dropped from this list, because its `NextHop` fields are on `incomingChange` and `sysribevents.ECMPPath`, and the file names no `mplsfib` symbol. Re-derive the list with `GOOS=linux` and the `integration` tag at implementation; the compiler catches a miss)
- `internal/plugins/fib/kernel/srv6localsid_linux.go` (created, Files to Create) carries the gateway per PeerSet nexthop and leaves non-on-link members out with a Warn (round-2 I1)

## Files to Create
- the four `test/plugin/bgp-epe-*.ci` files and the two interop scenarios named above
- `internal/core/srv6localsid/` (`doc.go`, `events.go`, tests) - the End.X contract: Entry, EntryBatch with acknowledgement, RemoveSource, mirroring `mplsfib`
- `internal/plugins/fib/kernel/srv6localsid_linux.go` (and `_other.go` refusing with "SRv6 End.X needs Linux") - the `seg6local` End.X writer
- `internal/le/interoplab/bgp/check_epe.go` (and `_test.go`) - checkers for the two EPE scenarios: parse GoBGP's `global rib -a ls` JSON and assert TLVs 1101/1102/1103 per NLRI, absence after `advertise false`, and TLV 1251 Flags and Reserved bits
- `test/interop/Dockerfile.gobgp4` - GoBGP v4 image (R-8)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-bgp-epe-conf.yang` `advertise`, `advertise-to`, `srv6-sid`, list `adjacency`, list `peer-set` (Files to Modify) |
| YANG validation constraints | Yes | `sid-index` range; `peer-set member` leafref and min 1; a `srv6-sid` refused at commit on a FIB backend that cannot install End.X, with the leaf, the reason and the fix |
| YANG custom validators | Yes | the End.X backend refusal |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | Yes | the new leaves come from YANG; `peer-set member` completes from configured `bgp-epe peer` addresses, `adjacency` from interface names, `advertise-to` from BGP peers |
| Functional test for new RPC/API | N-A | no new RPC; behavior `.ci` tests are listed above |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | rows 11-12 need kernel SRv6 End.X (`seg6local`) support when `srv6-sid` is configured. A `ze doctor` check names a missing kernel capability with the fix, beside the commit refusal |
| Prometheus counters/metrics | No | confirm at implementation whether EPE exposes SID counts |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family, capability or attribute; TLVs 1102, 1103 and 1251 are new BGP-LS attribute content, not a new attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the EPE feature entry under `features/` (find it with `./le feature report`) |
| 2 | Config syntax changed? | Yes | `docs/config-reference.md` and `docs/guide/configuration.md`, which carry the `bgp-epe` / `sid-index` config |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` where it describes the EPE plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` (`bgp-epe` section) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/nlri-bgpls.md` (TLVs 1102, 1103, 1251 originated) |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9086.md`, `rfc/short/rfc9514.md` and `docs/features/rfc-status.md` (generated by `./le rfc index-update`) |
| 10 | Test infrastructure changed? | Yes | the GoBGP v4 interop image (R-8), in `docs/architecture/testing/interop.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists EPE or BGP-LS |
| 12 | Internal architecture changed? | Yes | `docs/architecture/wire/nlri-bgpls.md` (rows 8, 11, 12), `docs/architecture/fib/fib-depth-4-srv6.md` (the End.X writer and the `srv6localsid` contract), `docs/architecture/doctor-and-health-checks.md` (the End.X kernel capability) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the new `srv6localsid` event namespace and the `mplsfib.Entry.NextHops` contract change: `docs/architecture/mpls/mpls-kernel.md` (the MPLS kernel programming page; `ai/CODE-TO-DOCS.md` holds no row for `internal/core/mplsfib`, so the implementer adds one with the page), extended for `srv6localsid` and the multipath label route; the `kernelcap.MustRegister` registration for `CONFIG_IPV6_SEG6_LWTUNNEL` and its doctor row: `docs/architecture/doctor-and-health-checks.md` (the page `ai/CODE-TO-DOCS.md` names for `kernelcap.go`) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/wire/nlri-bgpls.md` (rows 8, 11, 12: affected). `docs/architecture/rsvpte/mpls-rsvp-te.md` (declared by the RSVP-TE files the `mplsfib.Entry.NextHops` change touches: affected where it names the entry's next hop, round-2 I2). `docs/architecture/ldp/mpls-ldp.md` and `docs/architecture/wire/ospf.md` (the `// Design:` anchors of `ldp/fib.go` and `ospf/sr_fib.go`: affected where they name the entry's next hop, round-4 I1). Re-derive at implementation with `./le spec citation anchors spec plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `bgp-epe` examples in the pages of item 2 |

## Implementation Steps

1. **Phase: Wiring** -- the three `bgp-epe-*.ci` tests written and red.
2. **Phase: YANG and config (row 8)** -- `ze-bgp-epe-conf.yang` new revision, parse, refusals. Own commit.
3. **Phase: PeerAdj and PeerSet (row 8)** -- label install while up, BGP-LS export of TLVs 1102 and 1103. Own commit.
4. **Phase: per-peer control (row 8)** -- `advertise` and `advertise-to` egress filter, withdraw on disable or filter. Own commit.
5. **Phase: SRv6 PeerNode (rows 11, 12)** -- first validate A-2 (stop and ask the owner if it breaks); then `srv6localsid`, the fib-kernel End.X writer, the `kernelcap` registration and doctor row, the VPP refusal, TLVs 1250 and 1251 originated. Own commit.
6. **Phase: interop** -- `bgp-epe-peer-segments-gobgp` on v3.31.0, `bgp-epe-srv6-peernode-gobgp` on a v4 image, each with its recorded red.
7. **Phase: ledger** -- AC-4, discrimination records, `./le rfc check` clean for `rfc9086` and `rfc9514`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No SID is advertised while its label or End.X is not installed |
| Correctness | A disabled or filtered SID is withdrawn from a peer that had it |
| Rule: rfc-compliance | Each new implementing function quotes its RFC sentence; each caller names the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `{gap}` or `partial` left on rows 8, 11, 12 and 7-1 | `grep -n` each id in its `rfc/short/` file |
| Both polarities tagged | `grep -rn "RFC requirement: <id>" internal/ test/` |
| Ledger clean | `./le rfc check` exit status for `rfc9086` and `rfc9514` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `sid-index` and `srv6-sid` validated at commit |
| Resource exhaustion | one label or End.X per configured SID, bounded by config |
| Error leakage | `advertise-to` never leaks a SID to an excluded peer, including during a reload |

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

## Owner decisions (2026-10-11)

| Question | Owner's words | What it means here |
|----------|---------------|--------------------|
| Row 8: scope PeerAdj and PeerSet SIDs and per-recipient EPE control in, or out? | IN | AC-1, with `RFC9086-7-1`'s gap half |
| RFC9514-7.2-5/-6: should this work own them? | "Only if EPE is in" | EPE is in, so they are owned here (AC-2, AC-3), with the End.X installation they need (R-7) |
| SRv6-only EPE: may a peer carry only `srv6-sid`, with no `srgb` and no `sid-index`? (raised by the independent design review, I5) | "Yes, SRv6-only allowed" | AC-1c: `srgb` and `sid-index` optional when an SRv6 SID is configured; each entry needs at least one of `sid-index` or `srv6-sid`, refused at commit otherwise |
| Approve the design after the round-5 design review (0 BLOCKER, 0 ISSUE, 2 NOTE, both folded)? (Thomas, 2026-10-11) | "Approve" | Status moves from `design` to `ready`, so `/ze-implement` may claim it |
| One spec or several? | "Split into three" | This spec owns rows 8, 11, 12; the defects stay in `plan/immediate/spec-bgp-unowned-rfc-rows.md`, DelayOpen and Event 3 are `plan/spec-bgp-fsm-delayopen-and-automatic-start.md` |

## Open Questions (owner)

None open.

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A new spec, not an existing BGP-LS spec | Fold into `spec-bgp-ls-origination-and-the-scheduled-marker` | That spec owns RFC 9552 node keys and a ledger marker, not the EPE plugin or RFC 9086/9514 |
| SRv6 EPE SIDs installed as End.X and advertised only while installed | Advertise without installing | RFC 9514 §7.2 realises the SIDs "using SIDs with END.X behavior"; an uninstalled SID forwards nothing (R-7) |
| A new `srv6localsid` leaf event contract, consumed by fib-kernel | Extend `mplsfib.Entry` with an IPv6 SID; let the EPE plugin call netlink itself | `mplsfib` is label-keyed and its consumers (LDP, RSVP-TE, OSPF SR) would all see a field they never use; a plugin calling netlink would make two programmers of kernel forwarding state, which `mplsfib/doc.go` exists to prevent |
| PeerSet End.X as one multipath route with a `seg6local` End.X per nexthop | One End.X toward one member | Linux End.X holds one `nh6` (`seg6_local.c`), and RFC 9086 §5.3 says PeerSet "load-balance across any connected interface to any peer in the associated peer set"; A-2 validates it first |
| PeerAdj next hop is a configured `neighbor-address` | Derive it from ND or ARP on the interface | RFC 9086 §5.2 "Next-Hop: the interface peer address"; a multihop session over several links carries no per-link peer address, and a neighbour-table guess can change under the operator; Link Remote Identifier defaults to 0 as §5.2 permits |
| `advertise-to` is a recipient filter on the BGP-LS export | A per-SID policy language | RFC 9086 §7 continues: "This is not different from what is required by a BGP speaker in terms of information origination and advertisement." The exporter already sends per peer (`export_state.go`), so the filter sits in that loop as an egress filter, and RFC 9086 §7 asks for control of "which information is advertised to which internal or external peer" while the SID itself is the same for every recipient, so no per-recipient SID state is kept (merged with the former duplicate row, round-2 N2) |
| SRv6 PeerAdj (End.X SID TLV in a Link NLRI, RFC 9514 §4.1) is out of scope | Include it | The owner scoped SRv6 PeerNode and PeerSet (rows 11, 12); SRv6 PeerAdj is not a ledger row here. Recorded so the scope is explicit |
| PeerSet MPLS label as one multipath kernel route, `mplsfib.Entry.NextHops` replacing `NextHop` (round-2 I2) | One member per label; a second field beside `NextHop` | RFC 9086 §5.3 PeerSet exists to "load-balance across any connected interface to any peer in the associated peer set"; one member defeats that. A second field beside `NextHop` is the layering `ai/rules/no-layering.md` forbids; RSVP-TE, LDP and OSPF SR, the other producers, set one element or, for LDP's egress pop, none (round-4 I1) |
| `srv6-sid` on an IPv4 EPE peer refused at commit (round-2 I3) | Derive an IPv6 next hop for the peer | End.X takes an IPv6 `nh6` only, and nothing in config names an IPv6 address of an IPv4 BGP peer that Ze could know to be the right one |
| GoBGP as the interop receiver, v4 image for TLV 1251 | FRR or BIRD | Neither has BGP-LS (R-8) |

## Known Limitations
- None recorded at design; the End.X backend refusal covers FIB backends that cannot install SRv6 End.X.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code, using
the quotes in the Task section.

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
- [ ] AC-1, AC-1a..AC-1c, AC-2..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md` only, in the same `./le commit create` script
