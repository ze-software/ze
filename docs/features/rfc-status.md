# RFC Implementation Status

This page is the public standards ledger for Ze. It lists RFCs that Ze implements, partially implements, intentionally does not implement, or has deferred. It is a product-support view, not a formal IETF compliance certificate: `Supported` means the behavior is implemented and tied to current source anchors; `Experimental` means implemented but still needs deployment evidence or hardening before production claims; `Partial` means a named subset is missing, intentionally skipped, or not proven; `Unsupported` means Ze explicitly does not implement it; `Future` means tracked but not shipped.

Reference summaries under `rfc/short/` are not implementation claims by themselves. A row belongs here only when current docs, source code, tests, or learned closure notes tie the RFC to a Ze feature.

## BGP base protocol, capabilities, and session safety

| RFC | Area | Status | Implemented coverage | Remaining if not complete |
|-----|------|--------|----------------------|---------------------------|
| RFC 4271 | BGP-4 base protocol | Supported | FSM, OPEN, UPDATE, NOTIFICATION, KEEPALIVE, core path attributes, session events. | No tracked gap in current source anchors. |
| RFC 5492 | BGP capability advertisement | Supported | Capability TLV parser, encoder, unknown-capability ignore behavior, negotiated session view. | No tracked gap in current source anchors. |
| RFC 4760 | Multiprotocol BGP | Supported | AFI/SAFI capability negotiation, MP_REACH_NLRI, MP_UNREACH_NLRI, family-specific UPDATE handling. | RFC 7606 MP attribute ordering tradeoff is tracked under RFC 7606. |
| RFC 6793 | 4-byte AS numbers | Supported | ASN4 capability, AS4_PATH, AS4_AGGREGATOR, AS_TRANS handling. | No tracked gap in current source anchors. |
| RFC 2918 | Route Refresh | Supported | Route Refresh capability and ROUTE-REFRESH message handling. | No tracked gap in current source anchors. |
| RFC 7313 | Enhanced Route Refresh | Supported | BoRR and EoRR support, capability checks, bounded route resend. | No tracked gap in current source anchors. |
| RFC 7911 | ADD-PATH | Supported | Per-family send and receive modes, Path ID packing, NLRI path IDs where negotiated. | No tracked gap in current source anchors. |
| RFC 8654 | BGP Extended Message | Supported | Extended message capability and 65535-byte message limit when negotiated. | No tracked gap in current source anchors. |
| RFC 8950 | Extended Next Hop | Supported | IPv6 next-hop for IPv4 NLRI and negotiated extended next-hop lookup. | No tracked gap in current source anchors. |
| RFC 5549 | Legacy extended next-hop encoding | Supported | Backward-compatible parser for the older format superseded by RFC 8950. | Main public claim uses RFC 8950. |
| RFC 4724 | Graceful Restart | Supported | GR capability, End-of-RIB markers, receiving-speaker stale handling, restart marker integration. | No tracked gap in current source anchors. |
| RFC 9494 | Long-Lived Graceful Restart | Supported | LLGR capability state, LLGR_STALE community, route depreference during long-lived stale retention. | No tracked gap in current source anchors. |
| RFC 9234 | BGP Role and OTC | Supported | Role capability negotiation, role mismatch NOTIFICATION, OTC egress stamping, OTC ingress leak detection and treat-as-withdraw, unicast-only (AFI 1/2, SAFI 1) OTC scoping. | No tracked gap in current source anchors. |
| RFC 8516 | FQDN capability | Supported | Hostname capability code 73, decode support, per-peer hostname and domain advertisement. | Some older plugin comments still reference the draft name. |
| RFC 2545 | IPv6 next-hop handling | Supported | IPv6 MP_REACH next-hop parsing, including global plus link-local next-hop form. | Capability code 77 is draft-based; RFC 2545 covers the next-hop wire behavior. |
| RFC 4486 | BGP Cease subcodes and prefix maximum | Supported | Cease subcode catalog, max-prefix teardown, backoff, and operator reset paths. | No tracked gap in current source anchors. |
| RFC 6608 | BGP FSM Error subcodes | Partial | The RFC 6608 FSM Error subcodes (1 OpenSent, 2 OpenConfirm, 3 Established) are defined and decoded for display of a received NOTIFICATION. | Three MUST gaps, gated in `rfc/short/rfc6608.md`: ze does not originate a code-5 FSM Error NOTIFICATION with these subcodes on an unexpected message; the reactor dispatches received messages by type without an FSM-state guard. |
| RFC 7607 | Prefix-limit enforcement context | Supported | Prefix-limit enforcement is called before route admission and documented with RFC 4486. | No separate gap tracked beyond RFC 4486 behavior. |
| RFC 9687 | Send Hold Timer | Supported | Send Hold Timer, auto duration `max(8min, 2x hold-time)`, NOTIFICATION code 8 on expiry. | No tracked gap in current source anchors. |
| RFC 7606 | Revised UPDATE error handling | Partial | Structural UPDATE validation, treat-as-withdraw (routes are synthesized into withdrawals and removed from the Adj-RIB-In), attribute-discard decisions, session-reset with NOTIFICATION UPDATE Message Error, per-attribute validation for 15 attribute codes, inner MP_REACH/MP_UNREACH NLRI overrun and RFC 4760 flag-consistency validation (§5.3, session reset via §3.j) for IPv4/IPv6 unicast and multicast (ADD-PATH aware, RFC 7911 path-ids skipped when negotiated), tests bound per requirement in `ai/RFC-REQUIREMENTS.md`. | Seven MUST-level gaps, each annotated in `rfc/short/rfc7606.md` and gated by `make ze-rfc-check`: (1) §5.1 — Ze intentionally emits MP_UNREACH first and MP_REACH last; (2) §5.4 — unrecognized typed NLRI (EVPN/MVPN) is retained as an opaque route and re-advertised rather than discarded; (3) §7.13, (4) §7.15, (5) §7.16 — attribute codes 24, 25 and 128 have no RFC 7606 validator, so malformed lengths are accepted; (6) §7.15's unrecognized-type tolerance is met only by that same omission; (7) §6 — error logs carry a description only, without the NLRI or the malformed UPDATE bytes. |
| RFC 8203 | Administrative Shutdown Communication | Supported | UTF-8 shutdown message for Cease/Admin Shutdown and Cease/Admin Reset. | Sender keeps the conservative 128-byte RFC 8203 limit. |
| RFC 7947 | BGP Route Server | Supported | Transparent RS forwarding for RS clients: no AS_PATH prepend, no NEXT_HOP rewrite, MED preserved (verbatim wire forwarding); per-client import/export policy applied on redistribution. | Optional per-peer next-hop override and path-hiding mitigation (add-path) are operator-configured. |
| RFC 9003 | Administrative Shutdown Communication update | Supported | Receiver accepts the updated UTF-8 format and 255-byte length field; invalid UTF-8 is stripped on send. | Sender remains conservative at 128 bytes for interoperability. |
| RFC 8538 | Notification GR | Unsupported | Cease Hard Reset subcode (9) name is known. | The GR N-bit is decoded but not acted upon; routes are dropped on any NOTIFICATION, so RFC 8538 route retention is not implemented. |
| RFC 5065 | BGP confederations | Unsupported | None claimed. | Explicitly unsupported. |
| RFC 2385 | TCP MD5 | Supported on supported kernels | Per-peer MD5 password plumbing and OS-specific socket hooks. | Platform availability depends on kernel support. |
| RFC 5082 | GTSM and TTL security | Supported on Linux | TTL security settings and BFD/BGP TTL gates where supported. | Platform availability depends on kernel support. |
| RFC 5925 | TCP-AO | Unsupported | None claimed. | Not implemented. |

<!-- source: docs/features/bgp-protocol.md -- BGP family, capability, and attribute inventory -->
<!-- source: docs/architecture/wire/messages.md -- BGP message wire formats and shutdown communication -->
<!-- source: docs/architecture/wire/attributes.md -- BGP path attribute status table -->
<!-- source: docs/architecture/wire/mp-nlri-ordering.md -- RFC 7606 ordering decision -->
<!-- source: docs/architecture/behavior/fsm-established.md -- prefix-limit, RFC 7606, and timer processing order -->
<!-- source: internal/core/bgp/capability/capability.go -- capability codes, parsing, FQDN support -->
<!-- source: internal/component/bgp/message/notification.go -- NOTIFICATION codes, Cease subcodes, shutdown data -->
<!-- source: internal/component/bgp/reactor/session_write.go -- RFC 9687 Send Hold Timer -->

## BGP attributes, families, policy, and NLRI

| RFC | Area | Status | Implemented coverage | Remaining if not complete |
|-----|------|--------|----------------------|---------------------------|
| RFC 1997 | Standard communities | Partial | COMMUNITY attribute parsing (multiple-of-4 length enforced), encoding, JSON, well-known names, and operator community-match policy. | The well-known egress semantics are not auto-enforced: a received NO_EXPORT / NO_ADVERTISE / NO_EXPORT_SUBCONFED is honored only through an operator-configured community-match reject list, never automatically. Gated per requirement in `rfc/short/rfc1997.md` via `make ze-rfc-check`. |
| RFC 3765 | NOPEER community | Supported | NOPEER well-known community parsing and text output. | No tracked gap in current source anchors. |
| RFC 4360 | Extended communities | Supported | Extended community attribute parsing, encoding, JSON, and policy use. | No tracked gap in current source anchors. |
| RFC 5701 | IPv6 extended communities | Partial | IPv6 Address Specific Extended Community (code 25) codec: optional-transitive flags, 20-octet per-community encoding, and length-multiple-of-20 parse validation. Tests bound per requirement in `ai/RFC-REQUIREMENTS.md`. | One MUST gap, gated in `rfc/short/rfc5701.md`: a malformed code-25 attribute is not treat-as-withdrawn per RFC 7606 (the structural validation pass has no validator for code 25), the same code-25 omission already tracked under RFC 7606 §7.15. The lazy parser still rejects a non-multiple-of-20 length on access. |
| RFC 8092 | Large communities | Supported | LARGE_COMMUNITY parsing, validation, duplicate removal, JSON, and RFC 7606 length checks. | No tracked gap in current source anchors. |
| RFC 4456 | Route Reflection | Supported | ORIGINATOR_ID, CLUSTER_LIST, route-reflector plugin behavior, cluster checks. | No tracked gap in current source anchors. |
| RFC 7311 | AIGP attribute | Partial | AIGP wire encoding, decoding, JSON, and set/increment/decrement filters; TLV validation (type-1 length 11, total-length consistency, unknown-TLV preservation) gated per requirement in `ai/RFC-REQUIREMENTS.md`. | AIGP is not consumed by best-path selection, and Ze does not strip AIGP at the eBGP administrative-domain boundary (§3.2 MUST NOT): received AIGP is forwarded verbatim as an optional-transitive attribute, removable only by explicit operator policy. Gated as a gap in `rfc/short/rfc7311.md`. |
| RFC 4364 | BGP/MPLS IP VPNs | Supported | VPNv4 NLRI, RD, labels, route config, encode, and decode. | No tracked gap in current source anchors. |
| RFC 4659 | VPNv6 | Supported | VPNv6 NLRI and next-hop behavior through VPN family support. | No tracked gap in current source anchors. |
| RFC 8277 | BGP Labeled Unicast | Supported | IPv4 and IPv6 labeled unicast NLRI, label stack handling, route config, and dataplane handoff. | No tracked gap in current source anchors. |
| RFC 3032 | MPLS label stack encoding | Supported as dependency | 20-bit label stack encoding and validation used by labeled unicast and VPN NLRI. | No tracked gap in current source anchors. |
| RFC 4761 | VPLS NLRI | Supported | L2VPN VPLS family registration, encode, decode, and route config. | No tracked gap in current source anchors. |
| RFC 4762 | VPLS architecture | Supported at feature scope | VPLS feature inventory and NLRI support tie the RFC to current behavior. | No separate protocol-compliance section is maintained. |
| RFC 7432 | EVPN | Supported | EVPN NLRI family, common header, route-type parsing, encode, and decode. | Routes are announced via the `update text` API command; config-file route declaration is not implemented for EVPN. |
| RFC 9136 | EVPN IP Prefix extensions | Supported in EVPN scope | EVPN implementation tracks the extension alongside RFC 7432. | No separate gap tracked. |
| RFC 8955 | IPv4 FlowSpec | Supported | IPv4 FlowSpec and FlowSpec VPN NLRI encoding, decoding, route config, filters, and action communities. | No tracked gap in current source anchors. |
| RFC 8956 | IPv6 FlowSpec | Supported | IPv6 FlowSpec and FlowSpec VPN family support. | No tracked gap in current source anchors. |
| RFC 5575 | Earlier FlowSpec reference | Supported as legacy reference | Legacy FlowSpec action community behavior remains documented. | Main public claim uses RFC 8955 and RFC 8956. |
| RFC 7752 | BGP-LS | Partial | BGP-LS and BGP-LS VPN NLRI decode, including many TLVs. | Encode and route config are not implemented; BGP_LS path attribute remains marked not implemented. |
| RFC 9085 | BGP-LS Segment Routing extensions | Partial | Decoded as part of BGP-LS TLV coverage. | Same BGP-LS encode and route-config gaps. |
| RFC 9514 | BGP-LS SRv6 extensions | Partial | Decoded as part of BGP-LS SRv6 TLV coverage. | Same BGP-LS encode and route-config gaps. |
| RFC 4684 | Route Target Constraint | Partial | RTC NLRI decode for display/analysis (`ze bgp decode`). Tests bound per requirement in `ai/RFC-REQUIREMENTS.md`. | Decode-only: no encode/origination path and no RT-membership distribution. Four MUST gaps gated in `rfc/short/rfc4684.md`: Ze does not advertise RT membership NLRI (so no §3.2 Originator/Next-hop or best-path/client-path selection), builds no outbound route filter from RT membership (§3.2), and gates no VPN route advertisement on RT-membership End-of-RIB (§6). |
| RFC 6514 | MVPN and PMSI Tunnel | Partial | MVPN NLRI decode, MVPN NLRI encode primitives, and config route parser for source-active, shared-tree join, and source-tree join routes. | PMSI_TUNNEL path attribute remains marked not implemented; not all MVPN route types have config builders. |
| RFC 9012 | Tunnel Encapsulation attribute | Partial | On-demand parsing of selected Tunnel Encapsulation sub-TLVs. | Not exposed as a full feature claim. |
| RFC 8669 | BGP Prefix-SID attribute | Partial | SRv6 Prefix-SID validation and filtering are recorded in learned notes. | Architecture attribute table still says BGP_PREFIX_SID is not implemented; docs need reconciliation before a Supported claim. |
| RFC 9252 | BGP SRv6 Service TLV | Partial | SRv6 Prefix-SID service TLV validation and RFC 7606 error handling are implemented in the learned closure. | Shares the RFC 8669 documentation conflict. |
| RFC 9830 | BGP SR Policy | Supported | IPv4 and IPv6 SR Policy NLRI encode, decode, and route config. | No tracked gap in current source anchors. |

<!-- source: docs/architecture/wire/nlri.md -- NLRI family overview and wire formats -->
<!-- source: docs/architecture/wire/nlri-flowspec.md -- FlowSpec NLRI wire format -->
<!-- source: docs/architecture/wire/nlri-evpn.md -- EVPN NLRI wire format -->
<!-- source: docs/architecture/wire/nlri-bgpls.md -- BGP-LS NLRI wire format -->
<!-- source: internal/component/bgp/plugins/nlri/ -- per-family register.go files hold family registrations and RFC metadata -->
<!-- source: internal/component/bgp/plugins/nlri/mvpn/config.go -- MVPN config route parser -->
<!-- source: plan/learned/776-srv6-prefix-sid.md -- SRv6 Prefix-SID implementation closure and doc conflict -->

## BGP operations, telemetry, RPKI, and BFD-adjacent standards

| RFC | Area | Status | Implemented coverage | Remaining if not complete |
|-----|------|--------|----------------------|---------------------------|
| RFC 7854 | BMP v3 | Partial | BMP receiver and sender, peer lifecycle, route monitoring messages, CLI and config. | Loc-RIB route monitoring is deferred under RFC 9069. |
| RFC 8671 | BMP Adj-RIB-Out | Supported within BMP sender scope | Adj-RIB-Out direction flag handling and sent-route monitoring. | No tracked gap in current source anchors. |
| RFC 9069 | BMP Loc-RIB monitoring | Future | Tracked by BMP learned notes. | Requires reconstructing wire UPDATEs from structured best-change batches. |
| RFC 6811 | RPKI origin validation | Supported | Origin validation states (Valid/Invalid/NotFound) from ROA/VRP lookup with four-octet ASN support, RTR transport, an operator-configurable per-state action (invalid-action reject/log-only/accept, default reject) so exclusion of Invalid routes is an explicit policy choice, automatic re-validation of installed routes when the VRP set changes, and a fail-open guard. | No tracked gap in current source anchors. |
| RFC 8210 | RPKI RTR | Supported | RTR cache sessions, VRP download, multi-cache merge, v1 fallback for ASPA path. | No tracked gap in current source anchors. |
| RFC 6810 | Earlier RPKI RTR | Supported as compatibility reference | Comparison docs group RFC 6810 and RFC 8210 for RTR support. | Main guide and registration use RFC 8210. |
| RFC 9582 | RPKI RTR v2 ASPA records | Supported | ASPA PDU cache, full replacement semantics, RTR v2 negotiation. | No tracked gap in current source anchors. |
| RFC 5880 | BFD base | Partial | BFD FSM, timers, jitter, echo advertisement, authentication, metrics, show commands. | Feature remains Partial; demand mode is intentionally skipped and IPv6 transport coverage is not complete. |
| RFC 5881 | BFD single-hop | Partial | Single-hop UDP 3784 sessions, TTL/GTSM gate, interface binding. | IPv6 dual-bind and wider deployment proof remain tracked with BFD. |
| RFC 5882 | BFD generic application | Partial | BFD integration model used by BGP and static next-hop tracking. | Same BFD partial status. |
| RFC 5883 | BFD multi-hop | Partial | Multi-hop UDP 4784 sessions and min-TTL floor. | Same BFD partial status. |
| RFC 9384 | BFD-triggered BGP Cease | Supported within BFD | BGP peer opt-in sends Cease subcode 10 when BFD reports forwarding path loss. | No tracked gap beyond BFD feature status. |

<!-- source: docs/guide/bmp.md -- BMP receiver and sender behavior -->
<!-- source: internal/component/bgp/plugins/bmp/register.go -- BMP RFC registration -->
<!-- source: docs/guide/rpki.md -- RPKI origin validation and ASPA behavior -->
<!-- source: internal/component/bgp/plugins/rpki/register.go -- RPKI RFC registration -->
<!-- source: docs/architecture/bfd.md -- BFD architecture and reference RFC list -->
<!-- source: internal/component/bfd/auth/doc.go -- BFD authentication implementation -->

## IS-IS, OSPF, MPLS, and traffic engineering

| RFC | Area | Status | Implemented coverage | Remaining if not complete |
|-----|------|--------|----------------------|---------------------------|
| RFC 1195 | Integrated IS-IS for IPv4 | Experimental | Native IS-IS over Layer 2, L1/L2, broadcast and point-to-point circuits, TLV 132. | Feature remains Experimental pending production hardening and deployment evidence. |
| RFC 5301 | IS-IS dynamic hostname | Experimental | Dynamic hostname TLV support. | Same IS-IS experimental status. |
| RFC 5303 | IS-IS point-to-point three-way | Experimental | Point-to-point adjacency behavior. | Same IS-IS experimental status. |
| RFC 5304 | IS-IS HMAC-MD5 authentication | Experimental | Interface and level key-chain authentication. | Same IS-IS experimental status. |
| RFC 5310 | IS-IS generic crypto authentication | Experimental | HMAC-SHA authentication path. | Same IS-IS experimental status. |
| RFC 5305 | IS-IS wide metrics and TE TLVs | Experimental | Extended IS reachability and IPv4 reachability TLVs. | Same IS-IS experimental status. |
| RFC 3787 | IS-IS interoperability guidelines | Partial | Obsolete TLV 131/133 are ignored on receipt (no codec decoder, opaque passthrough, TLV 133 never treated as auth); the originated IIH carries the Protocols Supported TLV (129, IPv4 NLPID) and IP Interface Address TLV (132) for mixed-environment interoperability. Tests bound per requirement in `ai/RFC-REQUIREMENTS.md`. | One MUST gap, gated in `rfc/short/rfc3787.md`: Ze originates ONLY wide metrics (TLV 22/135) and never the narrow TLV 2, so it does not fall back to narrow-metric origination for a mixed narrow/wide domain -- it requires every device to be wide-capable. Ze still DECODES a legacy neighbor's narrow TLV 2. |
| RFC 2966 | IS-IS up/down bit | Experimental | Up/down bit retention and redistribution behavior. | Same IS-IS experimental status. |
| RFC 5308 | IS-IS IPv6 | Experimental | IPv6 reachability over the same IS-IS instance. | Same IS-IS experimental status. |
| RFC 5120 | IS-IS multi-topology | Unsupported | None; IS-IS runs single-topology dual-stack only. | Non-congruent IPv4/IPv6 topologies are out of scope; multi-topology TLVs are not implemented. |
| RFC 2328 | OSPFv2 | Experimental | Native OSPFv2 engine, raw protocol 89, LSDB, SPF, flooding, virtual links. | Feature remains Experimental pending production hardening and deployment evidence. |
| RFC 3101 | OSPF NSSA | Experimental | NSSA Type 7 origination, translator election, Type 7 to Type 5 translation, preference rules. | Same OSPF experimental status. |
| RFC 5709 | OSPFv2 HMAC-SHA authentication | Experimental | OSPFv2 cryptographic authentication path. | Same OSPF experimental status. |
| RFC 7474 | OSPFv2 manual-key security extension | Experimental | Cryptographic authentication with per-packet-type replay protection (RFC 7474 defines no OSPFv2 authentication trailer; that construct is OSPFv3/RFC 7166). | Same OSPF experimental status. |
| RFC 5340 | OSPFv3 | Experimental | OSPFv3 IPv6 family, packet format, instance IDs, IPv6 LSAs. | Same OSPF experimental status. |
| RFC 5838 | OSPFv3 multiple address families | Experimental | Multi-AF OSPFv3 behavior. | Same OSPF experimental status. |
| RFC 7166 | OSPFv3 authentication trailer | Experimental | OSPFv3 Authentication Trailer support. | Same OSPF experimental status. |
| RFC 4552 | OSPFv3 IPsec authentication | Experimental | Manual AH/ESP IPsec config for OSPFv3 IPv6 interfaces, XFRM readiness check, SA and policy lifecycle. | Manual keying only; feature remains under OSPF experimental status. |
| RFC 5250 | OSPFv2 opaque LSAs | Experimental | Opaque LSA framework and retention. | Same OSPF experimental status. |
| RFC 3630 | OSPFv2 Traffic Engineering LSA | Experimental | TE LSA body and sub-TLV support. | Same OSPF experimental status. |
| RFC 5392 | OSPF inter-AS TE extensions | Experimental | Inter-AS TE sub-TLV support. | Same OSPF experimental status. |
| RFC 7770 | OSPF Router Information LSA | Experimental | Router Information LSA body and multi-instance ordering. | Same OSPF experimental status. |
| RFC 7684 | OSPF Extended Prefix and Link LSAs | Experimental | Extended Prefix and Extended Link LSA bodies and malformed TLV handling. | Same OSPF experimental status. |
| RFC 3623 | OSPFv2 Graceful Restart | Experimental | Restarter and helper behavior. | Same OSPF experimental status. |
| RFC 5187 | OSPFv3 Graceful Restart | Experimental | Restarter and helper behavior for OSPFv3. | Same OSPF experimental status. |
| RFC 8665 | OSPF Segment Routing extensions | Experimental | Segment Routing TLV encoding shared by OSPFv2 and OSPFv3 paths. | Same OSPF experimental status. |
| RFC 8666 | OSPFv3 Segment Routing | Experimental | OSPFv3 Segment Routing (SR-MPLS) over RFC 8362 Extended LSAs: SRGB/SRLB, index-to-label arithmetic, Prefix-SID and Adjacency-SID. | Same OSPF experimental status. |
| RFC 8362 | OSPFv3 Extended LSAs | Experimental | Extended LSA framework used by OSPFv3 extensions. | Same OSPF experimental status. |
| RFC 5286 | Loop-Free Alternate FRR | Experimental | LFA and TI-LFA fast reroute: per-neighbor SPFs, loop-free / node-protecting / downstream backup selection, SR repair lists, multi-area suppression. | Two MUST gaps gated in `rfc/short/rfc5286.md`: the LFA backup is attached to every SPF route regardless of address family, so an OSPFv3 multicast AF (RFC 5838) route inherits it (RFC5286-x-5); and no explicit Section 4.1 hold-down timer bounds how long an alternate stays active, only SPF reconvergence (RFC5286-x-6). IS-IS has no LFA. |
| RFC 5036 | LDP | Experimental | UDP discovery, TCP session FSM, label information base, kernel MPLS integration. | Feature remains Experimental, despite learned notes showing interop progress. |
| RFC 3209 | RSVP-TE | Experimental | PATH and RESV signaling, ERO routing, bandwidth admission, soft-state refresh, teardown. | Cross-vendor RSVP-TE interop remains constrained by available open daemons. |
| RFC 2205 | RSVP base protocol | Experimental | RSVP base common-header and object codec used by RSVP-TE: Version-1 header enforced on decode, reserved octet zeroed on send, every emitted object length a multiple of 4; tests bound per requirement in `ai/RFC-REQUIREMENTS.md`. | Three MUST gaps gated in `rfc/short/rfc2205.md`: the receive path does not verify the RSVP checksum or drop bad-checksum messages (3.1); PATH is sent without the IP Router Alert option; and DecodeMessage does not inspect an unknown Class-Num's high-order bits, so an unknown object with high-order bits 00 is not rejected (3.10). |
| RFC 4090 | RSVP-TE Fast Reroute | Experimental | Facility backup behavior. | One-to-one detour backup was explicitly split to later work. |

<!-- source: docs/features.md -- IS-IS, OSPF, MPLS, LDP, and RSVP-TE feature statuses -->
<!-- source: docs/architecture/wire/isis.md -- IS-IS wire and TLV behavior -->
<!-- source: docs/architecture/wire/ospf.md -- OSPFv2 wire and extension behavior -->
<!-- source: docs/architecture/wire/ospfv3.md -- OSPFv3 wire and extension behavior -->
<!-- source: internal/plugins/ospf/config_ipsec.go -- RFC 4552 OSPFv3 IPsec config validation -->
<!-- source: internal/plugins/ospf/ipsec_install.go -- RFC 4552 OSPFv3 IPsec XFRM installer -->
<!-- source: plan/learned/920-mpls-ldp.md -- LDP learned closure -->
<!-- source: plan/learned/921-mpls-rsvp-te.md -- RSVP-TE learned closure -->
<!-- source: plan/learned/925-mpls-rsvp-te-fast-reroute.md -- RSVP-TE FRR learned closure -->

## First-hop redundancy

| RFC | Area | Status | Implemented coverage | Remaining if not complete |
|-----|------|--------|----------------------|---------------------------|
| RFC 9568 | VRRPv3 (IPv4 and IPv6) | Experimental | Default version. Advert encode/decode, the RFC 9568 Section 6.4 state machine (Backup/Master, Master_Down_Timer, skew time), priority and preemption (including preempt-delay), the address-owner priority 255 rule, centisecond Max_Advert_Int, virtual MAC 00:00:5e:00:01:{vrid} (IPv4) / 00:00:5e:00:02:{vrid} (IPv6) on a per-group macvlan, 224.0.0.18 / ff02::12 multicast, IP protocol 112, GTSM TTL/hop-limit 255 on TX and RX, gratuitous ARP and unsolicited NA on Master transition. | One gap: Accept_Mode (Section 6.4.3) is not enforced on the dataplane -- the leaf is parsed, validated and reported, but no filtering is installed, so an Active router answers traffic addressed to the virtual address whichever way it is set. Interoperability IS proven: ze exchanges adverts with keepalived 2.3.1 under QEMU and passes election, node-death failover, and graceful-stop scenarios, including virtual-MAC ownership of the virtual IP (a foreign host resolves the VIP to 00:00:5e:00:01:{vrid}). Experimental pending deployment hardening. |
| RFC 3768 | VRRPv2 (IPv4 only) | Experimental | Opt-in via `version 2`. Whole-second Advertisement_Interval encoding, v2 advert format, and the v2 rejection rules (no accept-mode, no IPv6). | Same VRRP experimental status. RFC 3768 authentication types are deliberately not implemented: RFC 9568 Section 9 removed them as providing no real security. |
| RFC 5798 | VRRPv3 (obsoleted by RFC 9568) | Supported | For IPv4, ze transmits the RFC 5798 pseudo-header checksum form, because that is what keepalived (proven on the wire: its own adverts use it) and the rest of the deployed base compute and require; a message-only advert is rejected by them as "Invalid VRRPv3 checksum". On receive, ze dual-accepts both this form and the RFC 9568 message-only form. | RFC 9568 Section 5.2.8 clarifies the IPv4 checksum as message-only (no pseudo-header); ze diverges from that clarification on transmit for interoperability, and counts message-only senders (`checksum-rfc9568-message-only`) so the strict-RFC-9568 population is visible. When that population dominates, the transmit form can be revisited. |

<!-- source: internal/plugins/vrrp/packet -- advert encode/decode, checksum forms -->
<!-- source: internal/plugins/vrrp/fsm -- RFC 9568 Section 6.4 state machine -->
<!-- source: internal/plugins/vrrp/transport -- proto 112 sockets, multicast joins, GTSM, GARP/NA -->
<!-- source: internal/component/iface/macvlan.go -- per-group macvlan carrying the virtual MAC -->
<!-- source: rfc/short/rfc9568.md -- VRRPv3 summary -->
<!-- source: rfc/short/rfc3768.md -- VRRPv2 summary -->

## Access, AAA, PPP, and subscriber services

| RFC | Area | Status | Implemented coverage | Remaining if not complete |
|-----|------|--------|----------------------|---------------------------|
| RFC 2661 | L2TPv2 | Partial | LNS/LAC tunnel lifecycle (answerer and **initiator**: ze dials SCCRQ, verifies SCCRP, sends SCCCN), AVP codec, hidden AVPs, challenge/response, reliable control channel, HELLO, StopCCN, data sessions, **LNS-side outgoing call (OCRQ/OCRP/OCCN) via `request l2tp outgoing-call`**, dial-target config, LAC PPPoE→L2TP relay (control plane). <!-- source: internal/component/l2tp/tunnel_initiator.go -- initiate/handleSCCRP; internal/component/l2tp/session_initiator.go -- placeOutgoingCall/handleOCRP --> | Feature remains Partial; see L2TP guide for operational limits. Initiator tunnel interop proven vs xl2tpd (test/l2tp-interop/scenarios/03). LAC data-plane bridge (A-4) is QEMU/CAP_NET_ADMIN-gated. |
| RFC 1661 | PPP LCP | Partial | PPP LCP FSM, negotiation, echo keepalive, common NCP structure used by L2TP and PPPoE. | Carries the L2TP and PPPoE Partial status. |
| RFC 1334 | PAP | Partial | PAP authentication option through PPP auth handling. | Carries the L2TP and PPPoE Partial status. |
| RFC 1994 | CHAP | Partial | CHAP-MD5 authentication for PPP and tunnel auth contexts. | Carries the L2TP and PPPoE Partial status. |
| RFC 2759 | MS-CHAPv2 | Supported within PPP and IPsec EAP | Mutual authentication in both the PPP and IPsec EAP paths; MPPE/MSK key derivation in the IPsec EAP path only. | No tracked gap in current source anchors. |
| RFC 1332 | IPCP | Partial | IPv4 address negotiation and pool integration, IPCP option codec (IP-Address type 3, RFC 1877 DNS 129/131), FSM negotiation to Opened, pppN address/route programming. | One MUST gap gated in `rfc/short/rfc1332.md`: codes 8-11 received on IPCP are not Code-Rejected (mapped to LCP echo/protocol-reject handling); codes 12 and above are Code-Rejected. |
| RFC 1877 | IPCP DNS options | Partial | Primary and secondary DNS option parsing and negotiation; the Configure-Ack echoes acceptable options verbatim, the Configure-Reject echoes unsupported ones, and the IPv4 link stays usable with or without DNS. Tests bound per requirement in `ai/RFC-REQUIREMENTS.md`. | Carries the L2TP and PPPoE Partial status. One MUST gap gated in `rfc/short/rfc1877.md`: a DNS option with a Length other than 6 is answered with a Configure-Nak (correcting the value) rather than a Configure-Reject, because Ze's reject path is keyed on unknown option TYPE, not option length. |
| RFC 5072 | IPv6CP | Partial | Interface identifier negotiation and independent IPv6 NCP handling. | IPv6 address assignment is outside IPv6CP and handled separately. |
| RFC 2516 | PPPoE | Partial | Access concentrator discovery state machine, AC-Cookie, session tables, AF_PPPOX kernel sessions, shared PPP driver. | Feature remains Partial. |
| RFC 2865 | RADIUS authentication | Supported for subscriber access | Access-Accept profile extraction, Filter-Id, Session-Timeout, Idle-Timeout, VSAs, pool selection. | Operator/admin login RADIUS (PAP) is a separate backend under `system/authentication/radius`; the profile attributes above are subscriber-access only. |
| RFC 2866 | RADIUS accounting | Supported for subscriber access | Start, Stop, and Interim-Update accounting records. | Admin/operator RADIUS accounting is not wired; the admin backend is authentication-only. |
| RFC 2869 | RADIUS extensions | Supported for subscriber access | Gigaword counters and selected accounting extensions. | Scoped to subscriber access. |
| RFC 5176 | RADIUS CoA and Disconnect Message | Supported for subscriber access | CoA/DM listener for RADIUS-initiated changes and disconnects. | Scoped to subscriber access. |
| RFC 8907 | TACACS+ | Partial | SSH login PAP auth, ordered failover, MD5 pseudo-pad encryption, command accounting, optional authorization, single-connect mode. | Feature inventory still marks TACACS+ Partial while learned notes record several closed gaps. |

<!-- source: docs/guide/l2tp.md -- L2TP, PPP, IPCP, IPv6CP, RADIUS scope -->
<!-- source: docs/guide/pppoe.md -- PPPoE access concentrator behavior -->
<!-- source: docs/guide/tacacs.md -- TACACS+ status and limits -->
<!-- source: docs/comparison.md -- RADIUS subscriber attributes and TACACS+ comparison rows -->
<!-- source: internal/component/l2tp/ppp/ipcp.go -- RFC 1877 IPCP DNS options -->

## IPsec, IKE, EAP, and kernel security associations

| RFC | Area | Status | Implemented coverage | Remaining if not complete |
|-----|------|--------|----------------------|---------------------------|
| RFC 7296 | IKEv2 | Supported | Wire codec, cryptographic primitives, initiator and responder FSM, IKE_SA_INIT, IKE_AUTH, CREATE_CHILD_SA, INFORMATIONAL, rekey, DPD. Section 1.4 authenticated Delete on operator `clear` (graceful bounce); Section 2.4 state synchronization: the responder accepts a fresh IKE_SA_INIT in parallel with an established SA and supersedes it only once the new SA authenticates (never on the unauthenticated init); INITIAL_CONTACT emitted on the first IKE_AUTH request and honored on receipt. | No tracked gap in current source anchors. |
| RFC 4301 | IPsec architecture | Supported | SAD/SPD model projected through XFRM policies and route-based IPsec interfaces. | No tracked gap in current source anchors. |
| RFC 4302 | Authentication Header | Supported in OSPFv3 manual IPsec path | AH algorithm planning and RFC 4552 OSPFv3 use. | Scoped to configured manual IPsec support. |
| RFC 4303 | ESP | Supported | ESP SA parameter model, protocol 50, tunnel and transport modes, XFRM installation, OSPFv3 manual ESP. | No tracked gap in current source anchors. |
| RFC 3948 | UDP encapsulation of ESP | Supported | NAT-T non-ESP marker, UDP 4500 encapsulation, NAT keepalive, XFRM UDP encap attributes. | No tracked gap in current source anchors. |
| RFC 4555 | MOBIKE | Unsupported | None; the IKE engine does not define the MOBIKE notify types (16396/16400) and does not handle UPDATE_SA_ADDRESSES. | Responder role (announce MOBIKE_SUPPORTED, accept UPDATE_SA_ADDRESSES, migrate XFRM endpoints) is not implemented. |
| RFC 3748 | EAP | Supported in IPsec | EAP framework inside IKEv2 IKE_AUTH, Success and Failure handling. | No tracked gap in current source anchors. |
| RFC 5216 | EAP-TLS | Supported in IPsec | TLS handshake in EAP, fragmentation, MSK derivation feeding IKEv2 AUTH. | No tracked gap in current source anchors. |
| RFC 5282 | IKEv2 AEAD algorithms | Supported | AES-GCM IKEv2 AEAD encryption and decryption framing. | No tracked gap in current source anchors. |

<!-- source: docs/features.md -- IPsec feature rows -->
<!-- source: internal/component/ike/wire/ -- IKEv2 wire format codec -->
<!-- source: internal/component/ike/crypto/ -- IKEv2 cryptographic primitives -->
<!-- source: internal/component/ike/dataplane/dataplane.go -- RFC 4301, RFC 4302, RFC 4303, and RFC 3948 dataplane model -->
<!-- source: internal/component/ike/eap/ -- EAP framework, MS-CHAPv2, and EAP-TLS -->

## DNS, provisioning, MRT, and flow telemetry

| RFC | Area | Status | Implemented coverage | Remaining if not complete |
|-----|------|--------|----------------------|---------------------------|
| RFC 1035 | DNS message behavior | Supported | DNS cache TTL handling, authoritative GeoDNS and AS112 response shaping. | No tracked gap in current source anchors. |
| RFC 7871 | EDNS0 Client Subnet | Supported | GeoDNS client IP selection from EDNS0 client-subnet or packet source. | No tracked gap in current source anchors. |
| RFC 7534 | AS112 operations | Supported | Authoritative sink for misdirected RFC 1918 and link-local reverse DNS. | No tracked gap in current source anchors. |
| RFC 7535 | EMPTY.AS112.ARPA | Supported | EMPTY.AS112.ARPA DNAME-redirection zone. | No tracked gap in current source anchors. |
| RFC 2131 | DHCPv4 | Supported | DHCP server with DORA, leases, static mappings, PXE support. | No tracked gap in current source anchors. |
| RFC 2132 | DHCP options | Supported | DHCP options used by DHCP server and PXE support. | No tracked gap in current source anchors. |
| RFC 4578 | DHCP PXE options | Supported | PXE boot option injection for BIOS and UEFI bootfile selection. | No tracked gap in current source anchors. |
| RFC 1350 | TFTP | Supported | Read-only TFTP server for PXE bootloader delivery. | No tracked gap in current source anchors. |
| RFC 2347 | TFTP option negotiation | Supported | Option negotiation for TFTP provisioning paths. | No tracked gap in current source anchors. |
| RFC 6396 | MRT | Supported | Daemon-side MRT recording, TABLE_DUMP_V2 snapshots, BGP4MP messages, analysis tools. | No tracked gap in current source anchors. |
| RFC 8050 | MRT ADD-PATH | Partial | Add-path MRT subtypes: TABLE_DUMP_V2 RIB_*_ADDPATH (8-11) carry a 4-byte big-endian Path Identifier between Originated Time and Attribute Length; RIB_GENERIC_ADDPATH (12) keeps the Path Identifier in the raw NLRI blob and does not redefine the RIB Entry; BGP4MP add-path subtypes (8-11) preserve the Path Identifier inside the encapsulated message's NLRI; add-path is distinguished purely by MRT subtype. Tests bound per requirement in `ai/RFC-REQUIREMENTS.md`. | One MUST gap, gated in `rfc/short/rfc8050.md`: ze selects the MRT add-path subtype from a static operator config toggle (`internal/plugins/mrt/dump.go` ribSubtype/bgp4mpTypeSubtype, `config.go` AddPath) rather than from each peer's negotiated RFC 7911 Add-Path capability (`internal/plugins/mrt/component.go` OnBGPMessage does not consult it), so a single dump cannot represent a mix of add-path and non-add-path peers. |
| RFC 3954 | NetFlow v9 | Experimental | Flow export templates and records over UDP. | Flow export feature remains Experimental. |
| RFC 7011 | IPFIX | Experimental | IPFIX export templates and records over UDP. | Flow export feature remains Experimental. |

<!-- source: docs/guide/plugins.md -- DHCP, TFTP, GeoDNS, and AS112 plugin rows -->
<!-- source: docs/guide/as112.md -- AS112 behavior -->
<!-- source: docs/guide/ze-install.md -- DHCP, TFTP, and PXE provisioning flow -->
<!-- source: docs/guide/mrt-analysis.md -- MRT recording and analysis -->
<!-- source: docs/guide/flow-export.md -- NetFlow v9 and IPFIX flow export -->
<!-- source: internal/core/dnsserver/client.go -- RFC 7871 client-subnet source selection -->
<!-- source: internal/core/dnsserver/handler.go -- RFC 1035 authoritative response shaping -->

## Drafts and non-RFC standards tracked near RFC work

These are not RFCs, but they sit next to RFC implementation status and are useful when reading the tables above.

| Standard | Area | Status | Note |
|----------|------|--------|------|
| draft-abraitis-idr-addpath-paths-limit | BGP PATHS-LIMIT | Supported | Per-family path-count limit capability for ADD-PATH. |
| draft-ietf-sidrops-aspa-verification | ASPA path verification | Supported | ASPA validation algorithm, policy actions, and RPKI event output. |
| draft-ietf-idr-linklocal-capability | Link-local next-hop capability code 77 | Supported | Capability declaration around RFC 2545 next-hop behavior. |
| draft-ietf-idr-software-version | BGP Software Version capability code 75 | Supported | Software version advertisement plugin. |
| ISO/IEC 10589 | IS-IS base protocol | Experimental | Base IS-IS protocol reference, paired with the IS-IS RFC rows above. |
| sFlow v5 | sFlow export | Experimental | Flow export protocol alongside NetFlow v9 and IPFIX. |

<!-- source: docs/features.md -- feature status vocabulary and draft-backed feature rows -->
<!-- source: docs/features/bgp-protocol.md -- draft-backed BGP capability rows -->
<!-- source: docs/guide/rpki.md -- ASPA draft verification behavior -->
