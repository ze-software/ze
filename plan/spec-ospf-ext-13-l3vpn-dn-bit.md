# Spec: ospf-ext-13 -- OSPFv2 L3VPN PE-CE DN bit (RFC 4576, RFC 4577)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-0-umbrella.md (delivered; closed, learned 1114) |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc4576.md` -- the DN bit: §4 set on PE->CE Type 3/5/7, honour on receive, ignore on other types, no other effect on LSA handling; §5 security (use crypto auth)
4. `rfc/short/rfc4577.md` -- OSPF-as-PE-CE: §4.2.5/§4.2.6 DN + VPN Route Tag; §4.2.4/§4.2.8.1 Domain Identifier; §4.2.6 OSPF Route Type Ext Community; §4.2.7 sham link
5. `plan/spec-ospf-0-umbrella.md` -- "Shared Contracts" (LSA inventory Type 3/5/7, LSA header Options byte, route preference resolved inside OSPF SPF), "Out of scope" (no MPLS L3VPN / VRF / VPNv4)
6. `internal/plugins/ospf/types/options.go` -- `OptionDN Options = 0x80` ALREADY exists, with `Has`/`Set`/`Clear` and the `optionNames` "DN" entry
7. `internal/plugins/ospf/lsdb/origination.go` -- `OriginateSummary` (Type 3) and `OriginateExternal` (Type 5) both take `opts types.Options` and write it verbatim into the LSA header; `OriginateNSSA` (Type 7) sibling
8. `internal/plugins/ospf/spf/external.go` -- `v4ExternalReader` already reads `lsa.Header.Options.Has(types.OptionNP)`; the DN honour gate lands here (Type 5/7)
9. `internal/plugins/ospf/spf/interarea.go` -- `summaryBody`/`ComputeInterAreaWith`/`v4SummaryReader`; the DN honour gate for Type 3 lands in the summary reader
10. `internal/plugins/ospf/redist_wiring.go` + `internal/plugins/ospf/redistribute/consumer.go` -- the redistribute->OSPF origination glue (`InjectExternal`, `OriginateExternal(... OptionE ...)`)

## Task

Specify the OSPF-side protocol mechanics that let a Ze OSPFv2 instance act as the
**PE-CE routing protocol for a BGP/MPLS IP L3VPN** (RFC 4577), with the **DN (Down)
bit** loop-prevention of RFC 4576 as the central mechanism. When a PE originates a
Type 3 (Summary), Type 5 (AS-External), or Type 7 (NSSA-External) LSA from a route
that was learned across the VPN backbone (via BGP VPNv4), it sets the DN bit (the
high-order bit of the OSPFv2 LSA Options octet, mask `0x80`); a receiving PE that
sees the DN bit set on a Type 3/5/7 LSA MUST exclude that LSA from its OSPF route
calculation, which breaks the PE -> CE -> backbone -> PE loop. RFC 4577 layers four
further mechanisms on top of the DN bit: the **OSPF Domain Identifier** (which OSPF
domain a route came from, deciding Type 3 vs Type 5 on re-origination), the **VPN
Route Tag** (a route tag in PE-originated Type 5 LSAs that other PEs ignore, the
backward-compatible fallback for PEs that predate the DN bit), the **OSPF Route Type
Extended Community** (Area Number / Route Type / Options carried across the backbone),
and the **sham link** (an unnumbered intra-area point-to-point link between two VRFs
so the backbone path appears intra-area).

`OptionDN` (the bit) already exists in the OSPFv2 Options type, and the three
origination methods already accept an `opts types.Options` argument and write it
verbatim into the LSA header, and the SPF readers already inspect
`lsa.Header.Options`. The gap is therefore not codec or storage: it is the
**set-on-originate / honour-on-receive policy plumbing** plus the RFC 4577 metadata
(Domain ID, VPN Route Tag, Route Type community, sham link) and the configuration
surface that marks an OSPF instance as a PE-CE instance bound to a VRF.

### CRITICAL gating assumption (blocking)

Ze currently has **NO MPLS L3VPN / VRF / VPNv4 infrastructure**: there is no VRF
abstraction, no per-VRF OSPF instance binding, no VPN-IPv4 (AFI 1 / SAFI 128) NLRI,
no Route Target import/export, no MPLS L3VPN data path. This spec specifies the OSPF
protocol mechanics (DN set/honour, Domain ID, Route Type community, VPN Route Tag,
sham link OSPF behaviour) and **explicitly gates its full implementation on that
VRF / VPNv4 infrastructure landing first** (recorded as blocking assumption A-1).
The OSPF-only mechanics that do NOT need VRF/VPNv4 (the DN-bit honour on receive, the
DN-bit set on PE-originated LSAs given a "this is a PE-CE instance" flag, and the
codec/SPF wiring) are specified to be implementable now behind that flag; the
mechanics that inherently require the backbone (Domain ID / Route Type community
encode/decode into BGP VPNv4 attributes, VRF binding, VPNv4 route eligibility,
sham-link endpoint distribution as a /32 VPNv4 route) are specified here but their
acceptance criteria are marked dependent on the infrastructure spec and MUST NOT be
claimed done until it exists.

### In scope (this spec)

| Item | Detail |
|------|--------|
| DN bit honour on receive | A received Type 3/5/7 LSA with `OptionDN` set is EXCLUDED from the OSPF route calculation (not from the LSDB; it is still stored, aged, flooded). The bit is IGNORED for all other LSA types (RFC 4576 §4) |
| DN bit set on originate | When a PE-CE OSPF instance originates a Type 3/5/7 LSA from a backbone (BGP VPNv4) route toward a CE, `OptionDN` is set in the LSA Options; it is CLEAR for all other LSA types and for non-PE-CE instances (RFC 4576 §4; RFC 4577 §4.2.5.1, §4.2.8.1) |
| VPN Route Tag | PE-originated Type 5 LSAs from extra-domain routes carry an OSPF external route tag equal to the configured/auto-computed VPN Route Tag; a received Type 5 whose route tag equals the VPN Route Tag is excluded from route calculation (RFC 4577 §4.2.5.2, §4.2.6, §4.2.8.1) |
| OSPF Domain Identifier | A configurable per-instance Domain Identifier (8-byte Ext Community value, NULL by default), used to decide Type 3 (same domain, inter/intra-area) vs Type 5/7 (different domain or external) when re-originating a backbone route into the CE-facing OSPF (RFC 4577 §4.2.4, §4.2.8) |
| OSPF Route Type Ext Community | The Area Number / OSPF Route Type / Options encode and decode that travels with the VPN-IPv4 route across the backbone and drives the LSA-type selection on re-origination (RFC 4577 §4.2.6, §4.2.8) |
| OSPF Router ID Ext Community | OPTIONAL: the PE's OSPF Router ID in the originating instance, carried with the VPN-IPv4 route (RFC 4577 §4.2.6) |
| LSA-type selection on re-origination | Domain match + route type -> Type 3 / Type 5 / Type 7, per the §4.2.8 algorithm; external => Area Number 0; Type 4-sourced routes never redistributed to BGP (RFC 4577 §4.2.6, §4.2.8) |
| Sham link (OSPF side) | The OSPF behaviour of a sham link: an unnumbered intra-area point-to-point link advertised as a Type 1 link in the Type 1 Router-LSA; treated as a Demand Circuit; configurable metric + default; the sham link is up iff a VRF route to the remote /32 endpoint exists (RFC 4577 §4.2.7) |
| PE-CE instance config surface | A YANG surface marking an OSPF instance as a PE-CE (L3VPN) instance bound to a VRF, with Domain ID, VPN Route Tag, sham-link, and area-0 attachment settings; the gate behind which the DN/tag behaviour activates |
| Security posture | Honour DN/tag even when also using tags; OSPF cryptographic authentication is the recommended PE-CE protection (RFC 4576 §5; RFC 4577 §6) -- reuse the existing OSPF auth keystore, no new auth surface |

### Out of scope (the infrastructure this spec depends on; noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| VRF abstraction (per-VRF RIB/FIB, route import/export) | a future MPLS L3VPN / VRF infrastructure spec (does not yet exist) |
| BGP VPN-IPv4 (AFI 1 / SAFI 128) NLRI + Route Target | the same future infrastructure spec; this spec consumes its Ext-Community plumbing, it does NOT build VPNv4 NLRI |
| MPLS L3VPN data path / label stack / sham-link MPLS encapsulation | the same future infrastructure spec; this spec specifies only the OSPF control-plane behaviour of a sham link |
| BGP Extended Communities attribute encode/decode machinery | reuse BGP's existing Ext-Community attribute support; this spec defines the THREE OSPF-specific Ext-Community type codes and their value layout, not the attribute container |
| Sham link forwarding (forward per the BGP route, TTL for multi-hop) | the infrastructure spec's data path; this spec covers only LSA origination/flooding over the sham link |
| OSPFv3 PE-CE (RFC 6565) | not applicable to RFC 4576/4577 (OSPFv2-only DN bit); a separate RFC, separate spec |
| Any change to the OSPFv2 LSA codec or Options type | none -- `OptionDN` and the Options codec already exist (RFC 4576 adds no wire structure) |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ospf/ospf-10-as-external-asbr.md` - ASBR Type 5 origination, redistribution and default-information
- [ ] `docs/architecture/ospf/ospf-11-stub-nssa.md` - stub and totally-stubby areas (RFC 2328 Section 3.6) and NSSA (RFC 3101)
- [ ] `docs/architecture/ospf/ospf-13-cli-diag-interop.md` - the presentation and observability layer over the OSPF engine
- [ ] `docs/architecture/ospf/ospf-4-component-config.md` - the OSPF config-to-engine backbone: the plugin root, the YANG tree, the SDK
- [ ] `docs/architecture/ospf/ospf-7-lsdb-flooding.md` - raw-byte LSA storage, freshness comparison, origination and flooding
- [ ] `docs/architecture/ospf/ospf-9-inter-area-abr.md` - ABR detection, and Type 3 network and Type 4 ASBR Summary-LSA origination
- [ ] `docs/features/ai-first.md` - register once, expose everywhere: one command and discovery surface
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/research/ospf-implementation-guide.md` lines 1574-1576 ("OSPF for MPLS/BGP L3VPN: the DN Bit (RFC 4576)") -- the feature's deployment context and the explicit "DEFER in ze; only relevant for L3VPN deployments, which depend on MPLS and VRF support that are separate undertakings"
  -> Decision: the guide itself gates this on MPLS/VRF support; the spec follows that by specifying OSPF mechanics now and gating the VRF-dependent ACs on the infrastructure spec (A-1)
  -> Constraint: the DN bit is "bit 0x8000 of the LSA header Options field" in the guide's prose, which is the high-order bit of the one-octet OSPFv2 Options field, i.e. mask `0x80` per RFC 4576 §4 -- use `OptionDN` (0x80), NOT a 16-bit value
- [ ] `plan/spec-ospf-0-umbrella.md` "Shared Contracts" (LSA inventory rows Type 3/5/7, LSA header Options byte, "Route preference / path types" resolved INSIDE OSPF SPF) and "Out of scope" (MPLS L3VPN / VRF / VPNv4 absent)
  -> Constraint: the LSDB key and the LSA header layout are unchanged; DN lives entirely in the existing Options octet that every Type 3/5/7 LSA already carries
  -> Constraint: route exclusion happens INSIDE OSPF SPF (the umbrella publishes one winning `locrib.Path` per prefix); the DN/tag gate must drop the candidate at the SPF reader, before it becomes a route, so the exclusion is invisible to the rest of the system
- [ ] `ai/rules/plugins.md` -- the PE-CE behaviour and its config/metrics/doctor must be self-contained in the OSPF plugin; no "l3vpn"/"dn-bit" spelling leaks into generic config/redistribute packages
  -> Constraint: the OSPF Route Type / Domain ID / Router ID Ext-Community type codes are OSPF-owned; if the future VRF/VPNv4 infrastructure exposes a generic Ext-Community attach point, OSPF registers ITS communities there, the generic package does not name them
- [ ] `ai/rules/config.md` + `ai/rules/config.md` -- YANG vs env var, kebab-case naming for the PE-CE config surface
  -> Constraint: PE-CE settings (domain-id, vpn-route-tag, sham-link, vrf binding) are operational OSPF config -> YANG under the `ospf` container, NOT environment vars; every leaf gets maximum native validation (Domain ID pattern, route-tag range, metric range)
- [ ] `ai/rules/performance.md` + `ai/rules/performance.md` -- any Ext-Community value encode and any `show` rendering are buffer-first / no `fmt` on the wire
  -> Constraint: the 8-byte Domain ID / 8-byte Route Type Ext-Community values are written with `WriteTo(buf, off) int`; `show ospf` PE-CE rows render via `textbuf`/`AppendTo`

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4576.md` -- the DN bit
  -> Constraint: §4 -- "When a type 3, 5, or 7 LSA is sent from a PE to a CE, the DN bit MUST be set"; "The DN bit MUST be clear in all other LSA types"
  -> Constraint: §4 -- "When the PE receives, from a CE router, a type 3, 5, or 7 LSA with the DN bit set, the information from that LSA MUST NOT be used during the OSPF route calculation"; "The DN bit MUST be ignored in all other LSA types"
  -> Constraint: §4 -- DN "has no other effect on LSA handling": a DN LSA is still stored in the LSDB, aged, and flooded exactly as if DN were clear. Do NOT drop or refuse to flood a DN LSA; the suppression is at SPF / route calculation ONLY
  -> Constraint: §4 -- DN is the HIGH-ORDER bit (bit 7, mask `0x80`) of the one-octet Options field; `OptionDN = 0x80` is correct
  -> Constraint: §5 -- there is NO capability negotiation; correct behaviour relies on every PE honouring DN; mitigation against forged DN is OSPF cryptographic authentication (RFC 2328 §D) -- reuse the existing OSPF auth
- [ ] `rfc/short/rfc4577.md` -- OSPF as the PE-CE protocol
  -> Constraint: §4.2.6 -- on receive from a CE: "If the received LSA has the DN bit set, its information MUST NOT be used by the route calculation"; "If a received Type 5 LSA has an OSPF route tag equal to the VPN Route Tag, its information MUST NOT be used by the route calculation"; routes in Type 4 LSAs MUST NOT be redistributed to BGP
  -> Constraint: §4.2.5.2 -- the VPN Route Tag value MUST be configurable; for a 2-byte backbone AS the default is the auto-computed tag (Automatic=1, Complete=1, PathLength=01, AS in the low 16 bits); for a 4-byte AS a tag MUST be configured and MUST be distinct from any tag used within the VPN
  -> Constraint: §4.2.4 / §4.2.8.1 -- Domain Identifier default is NULL; equality rules (identical 8 bytes, or matching low-6 bytes with 0005/8005 type fields, or both all-zero = NULL); if an instance has >1 Domain ID, NULL MUST NOT be one of them
  -> Constraint: §4.2.6 / §4.2.8 -- the OSPF Route Type Ext Community (type `0306`, accept `8000`) MUST be present on every PE-originated VPN-IPv4 OSPF route; route type 1/2 = intra-area, 3 = inter-area, 5 = external (Area Number MUST be 0), 7 = NSSA; same-domain inter/intra -> Type 3 LSA, different-domain/external -> Type 5 (or Type 7 if PE/CE link is NSSA); external Type 5 Forwarding Address = 0
  -> Constraint: §4.2.7 -- a sham link is an unnumbered intra-area point-to-point link advertised as a Type 1 link in a Type 1 LSA, SHOULD be a Demand Circuit, has a configurable metric + default, is up iff a VRF route to the remote /32 endpoint exists; the endpoint /32 MUST NOT be advertised by OSPF and MUST NOT be a Virtual Link endpoint
  -> Constraint: §6 -- OSPF cryptographic authentication MUST be implemented on each PE and SHOULD be used PE<->CE

**Key insights:**
- This is a **policy** change, not a codec change: `OptionDN` exists, the three origination methods already carry `opts` into the header, and the SPF readers already read `lsa.Header.Options`. The work is (a) deciding when to set DN on originate, (b) dropping DN/tag-marked Type 3/5/7 candidates at the SPF reader, and (c) the RFC 4577 metadata + config.
- The DN gate is at the SPF reader, NOT at the LSDB: a DN LSA stays in the database and floods normally (RFC 4576 §4 "no other effect"). The single highest-risk mistake is dropping or not flooding a DN LSA.
- Everything that touches the **backbone** (Domain ID / Route Type Ext-Community encode into BGP VPNv4, VRF binding, sham-link /32 distribution) inherently needs the VRF/VPNv4 infrastructure Ze does not have yet; those ACs are gated on A-1.
- The OSPF-only honour-on-receive behaviour CAN be implemented and tested now (feed an LSA with DN set into SPF and assert exclusion), independent of VRF -- so the spec is not entirely blocked.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/plugins/ospf/types/options.go` -- defines `OptionDN Options = 0x80` with the doc comment "the down-bit used by VPN extensions"; `Has`/`Set`/`Clear`; `optionNames` already renders "DN"; `OptionsFromBytes`/`WriteTo` handle the one-octet OSPFv2 Options
  -> Constraint: the bit, its name, its codec, and its String() rendering ALL already exist; this spec uses `OptionDN`, it does not add or redefine the constant
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `OriginateSummary(area, router, opts, typ, lsid, mask, metric)` (Type 3/Type 4) writes `Options: opts` into the header verbatim; `OriginateExternal(router, network, mask, opts, type2, metric, fwd, tag)` (Type 5) writes `Options: opts` and carries `ExternalRouteTag: tag`; `existingSelfBodyUnchanged` makes re-origination idempotent
  -> Constraint: setting DN on a self-originated Type 3/5 is purely a matter of passing `opts | OptionDN`; the header already round-trips Options; do NOT add a new origination path
  -> Constraint: `OriginateExternal` already takes a `tag uint32` -- the VPN Route Tag is carried through this existing argument; the External-LSA body already has `ExternalRouteTag`
- [ ] `internal/plugins/ospf/lsdb/nssa.go` -- `OriginateNSSA(area, router, network, mask, type2, metric, fa, tag, propagate)` originates the Type 7 NSSA-External-LSA; the Options it writes set the N/P bit, not DN
  -> Constraint: the Type 7 DN path mirrors Type 5: the Options passed to the Type 7 origination must OR in `OptionDN` for a PE-CE backbone route; `OriginateNSSA` currently takes no `opts` argument, so a DN-carrying variant or an extra parameter is required (the one signature change in scope)
- [ ] `internal/plugins/ospf/spf/external.go` -- `v4ExternalReader` decodes Type 5 / Type 7 into `ExternalRecord`; it ALREADY reads `lsa.Header.Options.Has(types.OptionNP)` to pick the Type-7 preference; `ComputeExternalWith` turns records into route candidates
  -> Constraint: the DN honour for Type 5/7 lands HERE: when `lsa.Header.Options.Has(types.OptionDN)` (or the route tag equals the VPN Route Tag for Type 5) the reader returns `false` so the candidate never becomes a route -- but only when honouring is enabled (PE-CE instance); a non-PE-CE instance ignores DN entirely (RFC 4576: DN has no effect outside the PE-CE context)
- [ ] `internal/plugins/ospf/spf/interarea.go` -- `v4SummaryReader`/`summaryBody`/`ComputeInterAreaWith` decode Type 3 Summary-LSAs into `InterAreaSummary`; `summaryBody` returns the decoded body; the LSA header (carrying Options) is available at the reader
  -> Constraint: the DN honour for Type 3 lands HERE: a Type 3 with `OptionDN` set is skipped in `ComputeInterAreaWith`/`v4SummaryReader` so it produces no inter-area route, gated on the PE-CE honour flag
- [ ] `internal/plugins/ospf/redist_wiring.go` -- `InjectExternal`-driven glue calls `OriginateExternal(cfg.RouterID, network, mask, types.OptionE, type2, metric, [4]byte{}, tag)` and `OriginateNSSA(...)`; `externalParams(cfg, source)` resolves per-source metric/type/tag from the `redistribute` config; `source` is "connected"/"static"/"bgp"
  -> Constraint: this is where "the route came from BGP" is known (`source == "bgp"`); for a PE-CE instance a BGP-VPNv4-sourced route is where DN is set and the VPN Route Tag is applied -- but distinguishing a *VPNv4* route from a plain BGP route needs the VRF/VPNv4 infrastructure (A-1)
- [ ] `internal/plugins/ospf/redistribute/source.go` -- the producer side (OSPF SPF route changes -> BGP export); `OnSPFChange`/`emitDelta`/`addEntry` build the redistribute batch
  -> Constraint: §4.2.6 "do not export a route learned with DN set back to BGP" is enforced upstream by the SPF gate (a DN route never becomes a `RouteEntry`), so the producer needs no DN logic; the Type-4 "MUST NOT redistribute to BGP" rule is a producer-side filter
- [ ] `internal/plugins/ospf/spf/summary.go` -- `Sink.OriginateSummary(...)` interface + `OriginateInterAreaSummaries`; the ABR origination loop that re-originates inter-area Type 3 LSAs, using a per-area `Options` map (`in.Options[dst]`)
  -> Constraint: the per-area `Options` already plumbs through to `OriginateSummary`; a PE-CE instance ORs `OptionDN` into the Options it passes for backbone-sourced summaries
- [ ] `internal/plugins/ospf/config.go` + `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `ospf` container config resolution; areas, interfaces, redistribute, NSSA, auth all resolved here into the engine config
  -> Constraint: the PE-CE config surface (vrf binding, domain-id, vpn-route-tag, sham-link) is added under the `ospf` container and resolved in `config.go` into new engine-config fields; gated off by default
- [ ] `internal/plugins/ospf/auth_keystore.go` -- the existing OSPFv2 cryptographic auth (MD5/keyed) keystore
  -> Constraint: RFC 4576 §5 / RFC 4577 §6 "use crypto auth PE<->CE" is satisfied by the EXISTING auth; no new auth surface; the PE-CE doctor check SHOULD warn when a PE-CE instance has no auth configured

**Behavior to preserve:** (unless user explicitly said to change)
- The OSPFv2 LSA codec, the `OptionDN` constant and Options codec, the LSDB key triple, and verbatim flooding of every LSA including DN-marked ones (RFC 4576 §4: DN has no effect on flooding).
- `OriginateSummary` / `OriginateExternal` signatures and idempotent re-origination; the existing redistribute `source`/`metric`/`tag` plumbing.
- All existing OSPF SPF behaviour for non-PE-CE instances: when PE-CE is NOT enabled, the DN bit is IGNORED in route calculation (a stray DN bit in a non-VPN deployment must not silently drop routes). The honour gate is conditional on the PE-CE config.
- Existing OSPFv2 functional / interop tests stay green; a node with no PE-CE config behaves exactly as today.

**Behavior to change:** (only if user explicitly requested)
- SPF readers (`v4ExternalReader` for Type 5/7, the Type 3 summary reader): when the instance is a PE-CE instance, exclude DN-marked (and VPN-Route-Tag-marked Type 5) candidates from the route calculation (RFC 4576 §4, RFC 4577 §4.2.6).
- Origination of Type 3/5/7 from backbone (VPNv4) routes in a PE-CE instance: set `OptionDN`; apply the VPN Route Tag to Type 5; choose Type 3 vs Type 5/7 by Domain ID match (RFC 4577 §4.2.5, §4.2.8). (Gated on A-1 for the VPNv4-route-eligibility part.)
- `OriginateNSSA`: accept the DN bit so a PE-CE Type 7 can carry it (the single origination-signature change).
- New PE-CE config surface + new metrics + a PE-CE doctor check.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- **Honour on receive:** an LS Update carrying a Type 3/5/7 LSA arrives -> existing `lsdb.ReceiveUpdate` stores + floods it unchanged -> on the next SPF run the inter-area / external readers inspect the LSA header Options. Format at entry: the one-octet LSA Options field with the high-order DN bit (mask `0x80`) and, for Type 5, the 4-byte External Route Tag.
- **Set on originate:** a backbone (BGP VPNv4) route eligible for re-origination into a PE-CE OSPF instance -> the redistribute / inter-area origination path -> `OriginateExternal` / `OriginateSummary` / `OriginateNSSA` with `OptionDN` set and (for Type 5) the VPN Route Tag. Format at entry: a VPN-IPv4 BGP route carrying the OSPF Route Type / Domain Identifier Ext Communities (from the future infrastructure).
- **Config:** the `ospf` YANG container's PE-CE leaves -> `config.go` resolution -> engine config flags (per-instance: PE-CE on/off, VRF name, Domain ID, VPN Route Tag, sham links).

### Transformation Path
1. **Decode (existing):** `packet.DecodeLSA` returns the Type 3/5/7 LSA with `Header.Options` (DN visible) and, for Type 5, `ExternalRouteTag`; `VerifyChecksum` validates; the LSA installs and floods unchanged.
2. **Honour gate (new, SPF readers):** during SPF, for a PE-CE instance, the inter-area reader (Type 3) and the external reader (Type 5/7) test `Header.Options.Has(OptionDN)`; the external reader additionally tests `ExternalRouteTag == vpnRouteTag` for Type 5. A match -> the reader yields no candidate -> no route is installed and nothing is exported back to BGP (the producer never sees it).
3. **Domain decision (new, gated on A-1):** for a VPNv4 route eligible for re-origination, the Domain Identifier Ext Community is compared (RFC 4577 §4.2.8.1 equality) against the instance's Domain ID; same domain + inter/intra -> Type 3; different domain or external/NSSA -> Type 5 (or Type 7 on an NSSA PE/CE link). The OSPF Route Type Ext Community supplies Area Number / route type.
4. **Set on originate (new):** the chosen origination call passes `opts | OptionDN`; Type 5 also passes the VPN Route Tag through the existing `tag` argument and Forwarding Address 0; Type 7 passes DN through the new NSSA-origination parameter.
5. **Ext-Community encode/decode (new, gated on A-1):** when a PE exports a VRF prefix to BGP VPNv4, it attaches the OSPF Domain Identifier (type 0005/0105/0205), OSPF Route Type (0306), and optional OSPF Router ID (0107) Ext Communities, with the value layouts from RFC 4577 §4.2.6; on import it reads them back. This rides on the future infrastructure's BGP Ext-Community attach point.
6. **Sham link (new, OSPF side):** a configured sham link contributes a Type 1 point-to-point link to the Router-LSA when up (a VRF route to the remote /32 endpoint exists), is treated as a Demand Circuit, and uses its configurable metric; its OSPF packets ride the backbone (data path = infrastructure spec).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire <-> LSA Options | existing Options codec; DN read at the SPF reader, set at origination | [ ] |
| LSA <-> SPF route calc | new DN/tag exclusion in `v4ExternalReader` (Type 5/7) and the Type 3 summary reader | [ ] |
| Backbone (BGP VPNv4) <-> OSPF instance | Domain ID / Route Type / Router ID Ext Communities; eligibility = VRF route (gated on A-1) | [ ] |
| OSPF instance <-> VRF | per-instance VRF binding from config; one instance per OSPF domain (gated on A-1) | [ ] |
| Sham link <-> Router-LSA | a Type 1 link added when the sham link is up; Demand Circuit treatment | [ ] |
| PE-CE config <-> engine | YANG leaves -> `config.go` -> engine-config flags gating all of the above | [ ] |

### Integration Points
- `internal/plugins/ospf/types` -- `OptionDN` consumed (not redefined).
- `internal/plugins/ospf/spf/external.go` + `interarea.go` -- the DN/tag honour gate in the readers (the route-calc exclusion).
- `internal/plugins/ospf/lsdb/origination.go` + `nssa.go` -- set DN on Type 3/5/7 origination; the VPN Route Tag on Type 5; the one NSSA-origination signature change.
- `internal/plugins/ospf/redist_wiring.go` + `redistribute/` -- where a backbone (VPNv4) route is re-originated; the Domain decision; the Type-4-not-exported and DN-route-not-exported filters.
- `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` -- the PE-CE config surface.
- `internal/plugins/ospf/auth_keystore.go` -- reused for the RFC 4576 §5 / RFC 4577 §6 crypto-auth recommendation.
- The future MPLS L3VPN / VRF / VPNv4 infrastructure (does not exist) -- the VRF binding, VPNv4 NLRI, Route Target, and BGP Ext-Community attach point (A-1).

### Architectural Verification
- [ ] No bypassed layers (DN LSAs flow wire -> codec -> LSDB store/flood -> SPF reader gate, the same spine as every other LSA; the gate is at route-calc, not at the LSDB)
- [ ] No unintended coupling (no "l3vpn" spelling in generic config/redistribute; OSPF owns its Ext-Community type codes; the carrier never short-circuits flooding for DN LSAs)
- [ ] No duplicated functionality (reuses `OptionDN`, `OriginateSummary`/`OriginateExternal`/`OriginateNSSA`, the SPF readers, the redistribute glue, the auth keystore; adds only the honour gate, the set-on-originate policy, the RFC 4577 metadata, and the config)
- [ ] Zero-copy preserved (DN LSAs flooded verbatim; Ext-Community values written buffer-first; `show` rendering via `textbuf`)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze has NO MPLS L3VPN / VRF / VPNv4 infrastructure; the VRF binding, VPN-IPv4 NLRI, Route Target, and BGP Ext-Community attach point this spec needs for its backbone-facing ACs do not exist and must land in a separate infrastructure spec first | task statement ("CRITICAL ASSUMPTION ... Ze currently has NO MPLS L3VPN / VRF / VPNv4 infrastructure"); no `vrf`/`vpnv4`/`vpn-ipv4` dirs under `internal/`; guide lines 1574-1576 "depend on MPLS and VRF support that are separate undertakings" | the VRF-dependent ACs (Domain ID / Route Type Ext-Community encode-decode, VPNv4 route eligibility, sham-link /32 distribution) cannot be implemented or claimed done; only the OSPF-only honour/set-behind-a-flag ACs are deliverable now | `grep -rn "VRF\|VPNv4\|VPN-IPv4\|SAFI 128\|vpnv4" internal/ pkg/` returns no VRF/L3VPN abstraction; the infrastructure spec does not exist | unvalidated |
| A-2 | `OptionDN = 0x80` is the correct bit and is already wired through the OSPFv2 Options codec and `String()` | `types/options.go` (`OptionDN Options = 0x80`, `optionNames` "DN", `WriteTo`/`OptionsFromBytes`) | the bit or its codec is wrong; DN corrupts another option | `TestDNBitValueAndCodec` round-trips Options with DN through `WriteTo`/`OptionsFromBytes` | unvalidated |
| A-3 | Setting DN on a Type 3/5 self-LSA is purely passing `opts | OptionDN`; the header round-trips Options with no other change | `lsdb/origination.go` `OriginateSummary`/`OriginateExternal` write `Options: opts` verbatim | a new origination path is needed | `TestOriginateSummaryWithDN` / `TestOriginateExternalWithDN` show the bit in the originated header | unvalidated |
| A-4 | The DN honour gate fits in the existing SPF readers, which already inspect `lsa.Header.Options` | `spf/external.go` `v4ExternalReader` already calls `lsa.Header.Options.Has(types.OptionNP)`; the Type 3 reader has the header | the gate needs new SPF plumbing | `TestExternalReaderSkipsDN` / `TestInterAreaReaderSkipsDN` | unvalidated |
| A-5 | A DN-marked LSA must still be stored, aged, and flooded (RFC 4576 §4 "no other effect"); the gate is at route-calc ONLY | `rfc/short/rfc4576.md` §4; `lsdb/flooding.go` floods by LS type, unaware of Options | dropping/refusing to flood DN LSAs breaks transit and interop | `TestDNLSAStillFloodedAndStored` (the LSA is in the LSDB and re-flooded; only excluded from SPF) | unvalidated |
| A-6 | The "route came from BGP" signal exists in the redistribute glue (`source == "bgp"`), but distinguishing a *VPNv4* route from a plain BGP route requires the VRF/VPNv4 infrastructure | `redist_wiring.go`/`redistribute` use a `source` string; no VRF/VPNv4 marker exists | the set-on-originate trigger cannot distinguish VPN routes; would wrongly DN-mark plain BGP redistribution | `grep` confirms no VPNv4 source marker; the trigger is gated behind the PE-CE/VRF binding (A-1) | unvalidated |
| A-7 | The VPN Route Tag rides the existing `tag uint32` argument of `OriginateExternal` and the External-LSA `ExternalRouteTag` body field | `lsdb/origination.go` `OriginateExternal(... tag uint32 ...)`, `packet.ExternalLSA.ExternalRouteTag` | a new tag-carrying path is needed | `TestOriginateExternalCarriesVPNRouteTag` | unvalidated |
| A-8 | OSPF cryptographic authentication (RFC 4576 §5 / RFC 4577 §6) is satisfied by the EXISTING OSPFv2 auth keystore; no new auth surface | `auth_keystore.go` (MD5/keyed OSPFv2 auth already present) | a new auth mechanism is needed; larger blast radius | the PE-CE doctor check references the existing auth config; `TestPECEDoctorWarnsNoAuth` | unvalidated |
| A-9 | The OSPFv2 Options field is the right place for DN even though the guide prose says "0x8000" -- it is the high-order bit of the one-octet field, mask `0x80` | `rfc/short/rfc4576.md` §4 ("high-order bit ... bit 7, mask 0x80"); guide lines 1574-1576 wording | the wrong bit width corrupts an option or fails interop | RFC summary §4 + `TestDNBitValueAndCodec`; FRR/BIRD interop shows the bit at `0x80` | unvalidated |
| A-10 | RFC 4576/4577 are IPv4-family-only; the IPv6 (`*_v6`) code path needs NO DN bit from this spec (OSPFv3 PE-CE is RFC 6565, out of scope) | RFC 4576 §4 (OSPFv2 Options octet); the v6 code lives inside `ospf` (the `*_v6` seams + the `internal/plugins/ospf/v3/` leaves), there is no separate ospfv3 plugin | the spec wrongly touches the IPv6 readers/origination | the v6 readers/origination are untouched; the spec's ACs reference only the v4 (`v4...`) readers | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A DN-marked LSA is dropped or not flooded instead of merely excluded from route-calc (violates RFC 4576 §4) | a downstream router never receives the DN LSA; interop transit broken | the gate is implemented ONLY in the SPF readers; a dedicated `TestDNLSAStillFloodedAndStored` asserts storage + re-flood; flooding code never inspects DN |
| R-2 | The honour gate fires for a NON-PE-CE instance, silently dropping a legitimate route whose DN bit happened to be set (e.g. a misbehaving peer) | routes disappear from a plain OSPF deployment after upgrade | the gate is conditional on the PE-CE config flag; `TestDNIgnoredWhenNotPECE` proves a non-PE-CE instance ignores DN |
| R-3 | Wrong bit (16-bit `0x8000` from the guide prose vs the correct one-octet `0x80`) corrupts an unrelated option (DC/EA/N-P/MC/E) | adjacency/option mismatches against FRR/BIRD | use `OptionDN` (0x80) only; `TestDNBitValueAndCodec`; interop against FRR ospfd confirms the bit position |
| R-4 | The VPN Route Tag default for a 2-byte AS is mis-encoded (Automatic/Complete/PathLength bits) so other PEs do not recognise it | a PE re-imports its own tagged Type 5 (loop) | encode per RFC 4577 §4.2.5.2 bit layout; `TestVPNRouteTagDefault2ByteAS`; require an explicit tag for 4-byte AS |
| R-5 | Domain Identifier equality implemented naively (byte-equal only) misclassifies same-domain routes as external, advertising Type 5 instead of Type 3 | inter-area VPN routes appear as externals; metrics/preference wrong | implement the full §4.2.8.1 equality (8-byte, 0005/8005 low-6, both-NULL); `TestDomainIDEquality` covers all three cases |
| R-6 | A Type 4-sourced route is exported back to BGP (violates RFC 4577 §4.2.6) | a loop or unexpected VPNv4 route for an ASBR summary | the producer filters Type-4-derived routes; `TestType4NotRedistributed` |
| R-7 | The spec is claimed "done" while the VRF/VPNv4-dependent ACs are unimplemented because the infrastructure is absent (A-1) | a review marks AC-6..AC-12 done without VRF code | those ACs are explicitly marked "gated on A-1"; the Implementation Audit MUST show them blocked, not done; the spec stays open until the infrastructure lands |
| R-8 | A sham link advertised as a Type 1 link confuses intra-area SPF when the remote endpoint /32 is unreachable | spurious intra-area path over a down sham link | the sham link is up iff a VRF route to the remote /32 exists; `TestShamLinkDownNoType1Link` (gated on A-1 for the VRF route) |
| R-9 | Forged DN/tag from an unauthenticated CE makes prefixes unreachable or creates loops (RFC 4576 §5) | a VPN site loses reachability after a spoofed LSA | the PE-CE doctor check warns when crypto auth is absent; reuse the existing OSPF auth; documented as the required mitigation |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A PE-CE OSPF instance is configured (YANG `pe-ce` leaf + vrf binding) | → | `config.go` resolves the PE-CE flag into engine config; the honour gate and set-on-originate read it | `TestPECEConfigResolves` (unit) + `test/ospf/ospf-l3vpn-config.ci` |
| A received Type 5/7 LSA with `OptionDN` set, instance is PE-CE | → | `v4ExternalReader` returns no candidate -> no route installed | `TestExternalReaderSkipsDN` (unit) + `test/ospf/ospf-l3vpn-dn-honour.ci` |
| A received Type 3 LSA with `OptionDN` set, instance is PE-CE | → | the inter-area summary reader skips it -> no inter-area route | `TestInterAreaReaderSkipsDN` (unit) |
| A backbone route re-originated as a Type 5 in a PE-CE instance | → | `OriginateExternal(... opts|OptionDN ..., tag=vpnRouteTag ...)` sets DN + VPN Route Tag | `TestOriginateExternalWithDN` (unit) + `test/ospf/ospf-l3vpn-dn-set.ci` |
| A received Type 5 whose route tag == VPN Route Tag, instance is PE-CE | → | `v4ExternalReader` returns no candidate | `TestExternalReaderSkipsVPNRouteTag` (unit) |
| A DN-marked LSA is received | → | `lsdb.ReceiveUpdate` stores + floods it unchanged (gate is route-calc only) | `TestDNLSAStillFloodedAndStored` (unit) |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A received Type 3/5/7 LSA with `OptionDN` set, instance is PE-CE | the LSA is EXCLUDED from the OSPF route calculation (no route installed, not exported to BGP); the LSA is still stored in the LSDB, aged, and flooded (RFC 4576 §4) |
| AC-2 | A received LSA of any type OTHER than 3/5/7 with the DN bit set | the DN bit is IGNORED; the LSA is used normally (RFC 4576 §4) |
| AC-3 | A received Type 3/5/7 LSA with `OptionDN` set, instance is NOT PE-CE | the DN bit is IGNORED; the route is used normally (DN has effect only in the PE-CE context) -- guards a plain OSPF deployment |
| AC-4 | A PE-CE instance originates a Type 3/5/7 LSA from a backbone (VPNv4) route toward a CE | the originated LSA has `OptionDN` set; for any non-3/5/7 LSA type the bit is CLEAR (RFC 4576 §4; RFC 4577 §4.2.5.1) -- (set-on-originate; the VPNv4 eligibility part gated on A-1) |
| AC-5 | A PE-CE instance originates a Type 5 LSA from an extra-domain route | the Type 5 carries an OSPF external route tag equal to the configured/auto VPN Route Tag; Forwarding Address = 0 (RFC 4577 §4.2.5.2, §4.2.8) |
| AC-6 | A PE-CE instance receives a Type 5 LSA whose route tag equals the VPN Route Tag | the LSA is EXCLUDED from the route calculation (RFC 4577 §4.2.6) |
| AC-7 | A VPN-IPv4 route eligible for re-origination, same domain (Domain ID match per §4.2.8.1), inter/intra-area origin | re-originated as a Type 3 inter-area LSA with DN set (RFC 4577 §4.2.8.2) -- **gated on A-1** |
| AC-8 | A VPN-IPv4 route, different domain or external/NSSA origin | re-originated as a Type 5 (or Type 7 on an NSSA PE/CE link) with DN set, VPN Route Tag, Area Number 0 for Type 5 (RFC 4577 §4.2.8.1, §4.2.8.3) -- **gated on A-1** |
| AC-9 | The OSPF Domain Identifier configured / received | encoded/decoded as an 8-byte Ext Community (type 0005/0105/0205, accept 8005); equality per §4.2.8.1 (8-byte, low-6 + 0005/8005, both-NULL); default NULL -- **gated on A-1** for the BGP attach |
| AC-10 | A PE-originated VPN-IPv4 OSPF route | carries the OSPF Route Type Ext Community (type 0306, accept 8000) with Area Number / Route Type (1/2 intra, 3 inter, 5 external area 0, 7 NSSA) / Options (LSB = type-2 metric); optionally the OSPF Router ID Ext Community (0107, accept 8001) -- **gated on A-1** |
| AC-11 | A route received in a Type 4 LSA | NOT redistributed to BGP (RFC 4577 §4.2.6) -- **gated on A-1** for the BGP export |
| AC-12 | A sham link configured with a remote /32 endpoint reachable via a VRF route | the sham link is UP and contributes an unnumbered Type 1 point-to-point link to the Router-LSA, treated as a Demand Circuit, using its configurable metric; the endpoint /32 is NOT advertised by OSPF (RFC 4577 §4.2.7) -- **gated on A-1** for the VRF route |
| AC-13 | A VPN Route Tag for a 2-byte backbone AS with no explicit tag configured | defaults to the auto-computed tag (Automatic=1, Complete=1, PathLength=01, AS in the low 16 bits); a 4-byte AS REQUIRES an explicit tag distinct from any VPN-internal tag (RFC 4577 §4.2.5.2) |
| AC-14 | A PE-CE instance with no OSPF cryptographic authentication configured | `ze doctor` raises a warning recommending crypto auth PE<->CE (RFC 4576 §5; RFC 4577 §6) |
| AC-15 | The `ospf` config with the PE-CE leaf set | resolves into engine config; the honour gate and set-on-originate activate; `show ospf` reflects the PE-CE / VRF / Domain ID / VPN Route Tag state |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures an OSPF instance as a PE-CE (L3VPN) instance with a VPN Route Tag | YANG `pe-ce` -> `config.go` -> engine flags; `show ospf` shows PE-CE on | `test/ospf/ospf-l3vpn-config.ci` |
| 2 | Receives, from a CE, a Type 5 LSA with the DN bit set | wire -> `ReceiveUpdate` (stored + flooded) -> SPF `v4ExternalReader` excludes it -> no route, no BGP export | `test/ospf/ospf-l3vpn-dn-honour.ci` + `ospf-l3vpn-frr` interop |
| 3 | Has the PE re-originate a backbone route to a CE as a Type 5 | redistribute glue -> `OriginateExternal(opts|OptionDN, tag=vpnRouteTag)` -> flooded; the CE/FRR sees DN + tag | `test/ospf/ospf-l3vpn-dn-set.ci` + `ospf-l3vpn-frr` interop |
| 4 | Receives a Type 5 whose route tag equals the VPN Route Tag (legacy non-DN PE) | wire -> SPF reader excludes by tag -> no route | `TestExternalReaderSkipsVPNRouteTag` + `ospf-l3vpn-frr` interop |
| 5 | Runs `ze` to decode an LSA with DN set | CLI -> `packet.DecodeLSA` -> `Options.String()` renders "DN" | `test/ospf/ospf-l3vpn-dn-decode.ci` |
| 6 | Configures a sham link between two VRFs (gated on A-1) | YANG `sham-link` -> Router-LSA Type 1 link when up -> intra-area SPF | `test/ospf/ospf-l3vpn-sham-link.ci` (gated on A-1) |
| 7 | Runs `ze doctor` on a PE-CE instance with no auth | doctor -> PE-CE check -> warning | `test/ospf/ospf-l3vpn-doctor.ci` |

<!-- If a path has a broken link (no implementation at some step), that is a spec gap. -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDNBitValueAndCodec` | `internal/plugins/ospf/types/options_dn_test.go` | A-2/A-9, AC-2: `OptionDN == 0x80`; round-trips through `WriteTo`/`OptionsFromBytes`; `String()` renders "DN" | |
| `TestExternalReaderSkipsDN` | `internal/plugins/ospf/spf/external_dn_test.go` | AC-1, A-4: Type 5/7 with DN set yields no candidate when PE-CE | |
| `TestExternalReaderSkipsVPNRouteTag` | `internal/plugins/ospf/spf/external_dn_test.go` | AC-6, R-4: Type 5 with route tag == VPN Route Tag yields no candidate | |
| `TestInterAreaReaderSkipsDN` | `internal/plugins/ospf/spf/interarea_dn_test.go` | AC-1: Type 3 with DN set yields no inter-area route when PE-CE | |
| `TestDNIgnoredWhenNotPECE` | `internal/plugins/ospf/spf/external_dn_test.go` | AC-3, R-2: a non-PE-CE instance uses a DN-marked route normally | |
| `TestDNBitIgnoredOnOtherLSATypes` | `internal/plugins/ospf/spf/external_dn_test.go` | AC-2: DN on a non-3/5/7 LSA is ignored | |
| `TestDNLSAStillFloodedAndStored` | `internal/plugins/ospf/lsdb/dn_flood_test.go` | AC-1, A-5, R-1: a DN LSA is stored + re-flooded; only route-calc excludes it | |
| `TestOriginateSummaryWithDN` | `internal/plugins/ospf/lsdb/origination_dn_test.go` | AC-4, A-3: Type 3 originated with `opts|OptionDN` has DN in the header | |
| `TestOriginateExternalWithDN` | `internal/plugins/ospf/lsdb/origination_dn_test.go` | AC-4, A-3: Type 5 originated with `opts|OptionDN` has DN in the header | |
| `TestOriginateExternalCarriesVPNRouteTag` | `internal/plugins/ospf/lsdb/origination_dn_test.go` | AC-5, A-7: Type 5 carries the VPN Route Tag, FA = 0 | |
| `TestOriginateNSSAWithDN` | `internal/plugins/ospf/lsdb/origination_dn_test.go` | AC-4: Type 7 NSSA-External originated with DN (the one signature change) | |
| `TestVPNRouteTagDefault2ByteAS` | `internal/plugins/ospf/l3vpn_tag_test.go` | AC-13, R-4: 2-byte-AS default tag bit layout; 4-byte AS requires explicit tag | |
| `TestDomainIDEquality` | `internal/plugins/ospf/l3vpn_domain_test.go` | AC-9, R-5: §4.2.8.1 equality (8-byte, low-6 + 0005/8005, both-NULL) -- gated on A-1 | |
| `TestRouteTypeExtCommunityCodec` | `internal/plugins/ospf/l3vpn_extcomm_test.go` | AC-10: Route Type / Domain ID / Router ID Ext-Community value layout + compat type codes -- gated on A-1 | |
| `TestLSATypeSelectionByDomain` | `internal/plugins/ospf/l3vpn_reorigin_test.go` | AC-7/AC-8: same-domain inter/intra -> Type 3; different/external -> Type 5/7 -- gated on A-1 | |
| `TestType4NotRedistributed` | `internal/plugins/ospf/redistribute/dn_export_test.go` | AC-11, R-6: Type-4-sourced route not exported to BGP -- gated on A-1 | |
| `TestShamLinkUpAddsType1Link` / `TestShamLinkDownNoType1Link` | `internal/plugins/ospf/l3vpn_sham_test.go` | AC-12, R-8: sham link contributes a Type 1 link iff up -- gated on A-1 for the VRF route | |
| `TestPECEConfigResolves` | `internal/plugins/ospf/config_l3vpn_test.go` | AC-15: PE-CE leaves resolve into engine config | |
| `TestPECEDoctorWarnsNoAuth` | `internal/plugins/ospf/doctor_l3vpn_test.go` | AC-14, A-8, R-9: doctor warns when a PE-CE instance has no crypto auth | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| DN bit (Options octet) | bit 7, mask `0x80` | `0x80` | N/A | N/A (single bit) |
| Applicable LSA types for DN | {3, 5, 7} | 7 | N/A | a non-3/5/7 type is not gated by DN |
| VPN Route Tag | 0 .. 4294967295 (32-bit OSPF route tag) | 4294967295 | N/A | N/A (masked to 32 bits) |
| Backbone AS for auto tag | 2-byte (0..65535) auto / 4-byte requires explicit | 65535 (2-byte) | N/A | 4-byte AS => explicit tag REQUIRED (RFC 4577 §4.2.5.2) |
| Domain Identifier value | 8-byte Ext Community (type 2 + value 6) | all-zero = NULL | N/A | N/A |
| Sham link metric | 1 .. 65535 (OSPF interface metric) | 65535 | 0 (invalid) | N/A |
| Sham link endpoint prefix | /32 | /32 | non-/32 invalid (RFC 4577 §4.2.7.1) | non-/32 invalid |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-l3vpn-config` | `test/ospf/ospf-l3vpn-config.ci` | PE-CE instance configures; `show ospf` shows PE-CE / VRF / VPN Route Tag | |
| `ospf-l3vpn-dn-honour` | `test/ospf/ospf-l3vpn-dn-honour.ci` | a received DN-marked Type 3/5/7 is excluded from the route table but present in the LSDB | |
| `ospf-l3vpn-dn-set` | `test/ospf/ospf-l3vpn-dn-set.ci` | a PE-originated Type 5 from a backbone route carries DN + VPN Route Tag | |
| `ospf-l3vpn-dn-decode` | `test/ospf/ospf-l3vpn-dn-decode.ci` | `ze` decode of a DN LSA shows "DN" in the Options | |
| `ospf-l3vpn-doctor` | `test/ospf/ospf-l3vpn-doctor.ci` | `ze doctor` warns when a PE-CE instance has no crypto auth | |
| `ospf-l3vpn-sham-link` | `test/ospf/ospf-l3vpn-sham-link.ci` | a sham link appears as a Type 1 link when up (gated on A-1) | |

### Interop Tests (MANDATORY for protocol features)
<!-- REQUIRED when the spec adds/changes wire protocol behavior. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-l3vpn-frr` | `test/interop/scenarios/ospf-l3vpn-frr/` | FRR `ospfd` (PE-CE: originates a Type 3/5 with the DN bit and a VPN route tag) | Ze, as a PE, honours FRR's DN bit (excludes the route from SPF while keeping the LSA in the LSDB), excludes a Type 5 whose tag == VPN Route Tag, and FRR honours Ze's DN-marked / tagged LSAs; the bit is at `0x80`; non-PE-CE adjacency unaffected | |

> Interop is required: this changes wire-visible behaviour (the DN bit set in
> PE-originated Type 3/5/7 LSAs and the VPN route tag in Type 5). The raw-IP /
> multicast paths are Linux-only and run as QEMU integration tests
> (`ai/rules/platform-linux.md`), consistent with the rest of the OSPF interop set.
> The full PE-CE-over-MPLS interop (VRF + VPNv4) is gated on A-1 and deferred to
> the infrastructure spec; this scenario exercises the OSPF-only DN/tag mechanics
> over a direct PE-CE adjacency.

### Future (if deferring any tests)
- The VRF/VPNv4-dependent ACs (AC-7..AC-12) and their tests are blocked on A-1 (the MPLS L3VPN / VRF / VPNv4 infrastructure spec). They are specified here but MUST NOT be claimed done until that infrastructure lands. Deferring them requires explicit user approval and the spec stays open (R-7).

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/plugins/ospf/spf/external.go` -- DN honour gate + VPN-Route-Tag gate in `v4ExternalReader` (Type 5/7), conditional on the PE-CE honour flag
- `internal/plugins/ospf/spf/interarea.go` -- DN honour gate in the Type 3 summary reader / `ComputeInterAreaWith`, conditional on the PE-CE honour flag
- `internal/plugins/ospf/spf/summary.go` -- pass `OptionDN` into the per-area Options for backbone-sourced inter-area Type 3 re-origination
- `internal/plugins/ospf/lsdb/nssa.go` -- accept the DN bit in the Type 7 NSSA-External origination (the one origination-signature change)
- `internal/plugins/ospf/redist_wiring.go` -- set DN + the VPN Route Tag when re-originating a backbone (VPNv4) route in a PE-CE instance (gated on A-1 for VPNv4 eligibility)
- `internal/plugins/ospf/redistribute/source.go` -- filter Type-4-derived routes and DN-excluded routes out of the BGP export (gated on A-1 for the export)
- `internal/plugins/ospf/config.go` -- resolve the PE-CE / VRF / Domain ID / VPN Route Tag / sham-link leaves into engine config
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the PE-CE config surface (leaves below)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- `show ospf` PE-CE / sham-link rows
- `internal/plugins/ospf/cmd_show.go` -- render the PE-CE / VRF / Domain ID / VPN Route Tag / sham-link state
- `internal/plugins/ospf/doctor.go` -- the PE-CE crypto-auth warning check
- `internal/core/diagnostic/codes.go` -- the new `doctor-ospf-l3vpn-auth` diagnostic code (PE-CE without crypto auth)
- `internal/plugins/ospf/lsdb/origination.go` -- (no signature change for Type 3/5; confirm `opts|OptionDN` flows through `OriginateSummary`/`OriginateExternal`)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `pe-ce` (boolean), `vrf` (string, name ref), `domain-id` (8-byte hex/pattern), `vpn-route-tag` (uint32), `sham-link` list (local-endpoint /32, remote-endpoint /32, area, metric); read `ai/rules/config.md` + `ai/rules/config.md` |
| YANG validation constraints | [ ] yes | `domain-id` pattern (8-byte Ext-Community), `vpn-route-tag` range 0..4294967295, `sham-link/metric` range 1..65535, endpoint `pattern` for an IPv4 /32; every leaf maximally constrained |
| YANG custom validators | [ ] yes | a 4-byte-AS-requires-explicit-tag cross-field check; an endpoint-must-be-/32 check via `ze:validate` + `ValidateFn`; `CompleteFn` for the `vrf` name where a VRF registry exists (gated on A-1) |
| CLI commands/flags | [ ] yes | `show ospf` PE-CE / sham-link rows in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli.md` -- `show ospf` already action-first; new rows reuse it |
| Editor autocomplete | [ ] yes | automatic for the YANG enum/boolean/range leaves; `CompleteFn` for `vrf` (gated on A-1) |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-l3vpn-*.ci` |
| Pipe completeness | [ ] yes | the `show ospf` PE-CE output routes through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | PE-CE settings are operational OSPF config, not `environment/` leaves |
| Doctor check for runtime dependencies | [ ] yes | `doctor-ospf-l3vpn-auth` (PE-CE instance without crypto auth) in `doctor.go` + `internal/core/diagnostic/codes.go` + unit + functional test, per `ai/rules/repo-maintenance.md`; no new socket/port (reuses the OSPF raw socket) |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_l3vpn_dn_excluded_total` | counter | `lsa_type` (3/5/7) |
| `ze_ospf_l3vpn_vpn_route_tag_excluded_total` | counter | (none) |
| `ze_ospf_l3vpn_dn_originated_total` | counter | `lsa_type` (3/5/7) |
| `ze_ospf_l3vpn_pece_instances` | gauge | `vrf` |
| `ze_ospf_l3vpn_sham_links` | gauge | `area`, `state` (up/down) |

> These extend the umbrella's canonical OSPF metric set; they use the
> `ze_ospf_l3vpn_*` prefix and are registered by this spec's owner code. The
> umbrella "Metrics" table gains these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF L3VPN PE-CE (DN bit) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` -- the PE-CE leaves |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ospf` PE-CE / sham-link rows |
| 4 | API/RPC added/changed? | [ ] no | show RPCs live in the central `ze-show` namespace; document under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains PE-CE behaviour |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- a PE-CE / L3VPN section (with the A-1 gating note) |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` -- DN bit usage + VPN route tag in Type 5 |
| 8 | Plugin SDK/protocol changed? | [ ] check | only if the future VRF/VPNv4 infrastructure exposes an Ext-Community attach point this spec registers into |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc4576.md`, `rfc/short/rfc4577.md` -- flip the implemented compliance items |
| 10 | Test infrastructure changed? | [ ] yes (interop scenario added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF L3VPN DN-bit parity with FRR/BIRD |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- the DN honour gate + set-on-originate policy |
| 13 | Route metadata keys added/changed? | [ ] check | if a VPNv4 route metadata key is introduced when A-1 lands |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the five `ze_ospf_l3vpn_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table + `docs/guide/status.md` |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into `spf/external.go`, `spf/interarea.go`, `lsdb/origination.go`, `config.go` |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF config/CLI examples against the new PE-CE leaves |

## Files to Create
- `internal/plugins/ospf/l3vpn.go` -- the PE-CE policy seam: the honour flag, the VPN Route Tag default computation (RFC 4577 §4.2.5.2), the Domain ID equality (§4.2.8.1), the LSA-type-by-domain decision (§4.2.8), the set-on-originate triggers (gated on A-1 for VPNv4 eligibility)
- `internal/plugins/ospf/l3vpn_extcomm.go` -- the OSPF Domain Identifier / Route Type / Router ID Ext-Community value layouts + compat type codes (RFC 4577 §4.2.6), buffer-first encode/decode (gated on A-1 for the BGP attach)
- `internal/plugins/ospf/l3vpn_sham.go` -- the sham-link OSPF state: up/down by VRF route to the remote /32, the Type 1 link contribution, the Demand Circuit treatment (gated on A-1 for the VRF route)
- `internal/plugins/ospf/types/options_dn_test.go`
- `internal/plugins/ospf/spf/external_dn_test.go`, `internal/plugins/ospf/spf/interarea_dn_test.go`
- `internal/plugins/ospf/lsdb/dn_flood_test.go`, `internal/plugins/ospf/lsdb/origination_dn_test.go`
- `internal/plugins/ospf/l3vpn_tag_test.go`, `internal/plugins/ospf/l3vpn_domain_test.go`, `internal/plugins/ospf/l3vpn_extcomm_test.go`, `internal/plugins/ospf/l3vpn_reorigin_test.go`, `internal/plugins/ospf/l3vpn_sham_test.go`
- `internal/plugins/ospf/redistribute/dn_export_test.go`
- `internal/plugins/ospf/config_l3vpn_test.go`, `internal/plugins/ospf/doctor_l3vpn_test.go`
- `test/ospf/ospf-l3vpn-config.ci`, `ospf-l3vpn-dn-honour.ci`, `ospf-l3vpn-dn-set.ci`, `ospf-l3vpn-dn-decode.ci`, `ospf-l3vpn-doctor.ci`, `ospf-l3vpn-sham-link.ci`
- `test/interop/scenarios/ospf-l3vpn-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm `OptionDN`, the origination `opts`, and the SPF readers exist; confirm A-1 (no VRF/VPNv4) |
| 3. Wiring phase | Wiring Test table -- config flag + failing wiring tests |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `./le verify lint run && ./le test-unit  && ./le functional` |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | from critical review |
| 9. Re-verify | re-run stage 6 |
| 10. Repeat 7-9 | until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | re-run stage 6 |
| 14. Present summary | Executive Summary per `ai/rules/planning.md` |

### Implementation Phases

<!-- Phase 1 is ALWAYS wiring. -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- the PE-CE config flag + failing wiring tests
   - Tests: `TestPECEConfigResolves`, `test/ospf/ospf-l3vpn-config.ci`
   - Files: `yang/ze-ospf-conf.yang` (`pe-ce` boolean + the minimal leaves), `config.go` (resolve the flag into engine config), `l3vpn.go` (the honour-flag accessor, stubs for the gate)
   - Verify: the flag resolves and the honour/set-on-originate seams read it; the deeper DN tests still fail because the gate is a stub
2. **Phase: DN honour on receive** -- exclude DN-marked Type 3/5/7 from route-calc
   - Tests: `TestExternalReaderSkipsDN`, `TestInterAreaReaderSkipsDN`, `TestDNIgnoredWhenNotPECE`, `TestDNBitIgnoredOnOtherLSATypes`, `TestDNLSAStillFloodedAndStored`, `ospf-l3vpn-dn-honour.ci`
   - Files: `spf/external.go`, `spf/interarea.go`, `types/options_dn_test.go`, `lsdb/dn_flood_test.go`
   - Verify: a DN Type 3/5/7 yields no route when PE-CE; ignored when not PE-CE and on other types; the LSA still floods + stores (R-1)
3. **Phase: DN + VPN Route Tag on originate** -- set the bit and the tag
   - Tests: `TestOriginateSummaryWithDN`, `TestOriginateExternalWithDN`, `TestOriginateExternalCarriesVPNRouteTag`, `TestOriginateNSSAWithDN`, `TestExternalReaderSkipsVPNRouteTag`, `TestVPNRouteTagDefault2ByteAS`, `ospf-l3vpn-dn-set.ci`
   - Files: `lsdb/nssa.go` (the one NSSA DN signature change), `redist_wiring.go`, `spf/summary.go`, `l3vpn.go` (VPN Route Tag default), `l3vpn_tag_test.go`, `lsdb/origination_dn_test.go`
   - Verify: PE-originated Type 3/5/7 carry DN; Type 5 carries the VPN Route Tag with FA 0; the tag default for a 2-byte AS is correct; a tag-matched Type 5 is excluded on receive
4. **Phase: RFC 4577 metadata + LSA-type-by-domain (gated on A-1)** -- Domain ID, Route Type / Router ID Ext Communities, re-origination decision
   - Tests: `TestDomainIDEquality`, `TestRouteTypeExtCommunityCodec`, `TestLSATypeSelectionByDomain`, `TestType4NotRedistributed`
   - Files: `l3vpn.go`, `l3vpn_extcomm.go`, `redistribute/source.go`, `l3vpn_domain_test.go`, `l3vpn_extcomm_test.go`, `l3vpn_reorigin_test.go`, `redistribute/dn_export_test.go`
   - Verify: §4.2.8.1 equality holds; the Ext-Community layouts + compat codes round-trip; same-domain inter/intra -> Type 3, else Type 5/7; Type-4-sourced not exported. BLOCKED until A-1 lands (R-7)
5. **Phase: Sham link (OSPF side, gated on A-1)** -- Type 1 link contribution + Demand Circuit
   - Tests: `TestShamLinkUpAddsType1Link`, `TestShamLinkDownNoType1Link`, `ospf-l3vpn-sham-link.ci`
   - Files: `l3vpn_sham.go`, the Router-LSA origination link list, `l3vpn_sham_test.go`
   - Verify: a sham link contributes a Type 1 link iff a VRF route to the remote /32 exists. BLOCKED until A-1 lands
6. **Phase: doctor + metrics + CLI** -- user surface
   - Tests: `TestPECEDoctorWarnsNoAuth`, `ospf-l3vpn-doctor.ci`, `ospf-l3vpn-dn-decode.ci`, `ospf-l3vpn-config.ci`
   - Files: `doctor.go`, `internal/core/diagnostic/codes.go`, `cmd_show.go`, `yang/ze-ospf-cmd.yang`, metric registration, `doctor_l3vpn_test.go`, `config_l3vpn_test.go`
   - Verify: doctor warns on missing PE-CE auth; `show ospf` shows PE-CE state; the five metric series register
7. **Functional tests** -> the six `.ci` cover the user-visible behaviour
8. **RFC refs** -> add `// RFC 4576 Section 4` / `// RFC 4577 Section 4.2.x` comments on the honour gate, the set-on-originate, the tag, the Domain ID, the Route Type community, and the sham-link code
9. **Interop** -> `ospf-l3vpn-frr` QEMU scenario (OSPF-only DN/tag mechanics)
10. **Full verification** -> `./le verify current mode full`
11. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec). BLOCKING: if the A-1-gated phases (4, 5, and the gated ACs) are unimplemented because the infrastructure is absent, the spec MUST stay OPEN and the audit MUST show those items blocked, not done (R-7)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every DELIVERABLE AC-N (non-A-1-gated) has file:line; every A-1-gated AC is explicitly marked blocked, not silently dropped |
| Feature completeness | each non-gated user story has a working path; DN honour + set + VPN Route Tag parity with FRR's RFC 4576 behaviour |
| Correctness | DN at `0x80` only; gate at route-calc not the LSDB; DN ignored when not PE-CE and on non-3/5/7 types; VPN Route Tag default bit layout; Domain ID §4.2.8.1 equality |
| Naming | `ze_ospf_l3vpn_*` metrics; YANG `pe-ce`/`domain-id`/`vpn-route-tag`/`sham-link` kebab-case; `doctor-ospf-l3vpn-auth` |
| Data flow | DN LSAs flood + store unchanged; exclusion only at the SPF reader; no DN route reaches the BGP producer; no "l3vpn" spelling in generic config/redistribute |
| CLI grammar | `show ospf` PE-CE rows action-before-identifier |
| Doctor checks | `doctor-ospf-l3vpn-auth` registered per `ai/rules/repo-maintenance.md`, code in `codes.go`, unit + functional test |
| YANG validation | every PE-CE leaf maximally constrained; `domain-id` pattern, `vpn-route-tag` range, `sham-link/metric` range, endpoint /32; cross-field 4-byte-AS-tag validator |
| Prometheus counters | the five `ze_ospf_l3vpn_*` series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | OSPF owns its Ext-Community type codes; removing the PE-CE config removes all PE-CE behaviour |
| Rule: A-1 gating honoured | no A-1-gated AC claimed done without VRF/VPNv4 infrastructure; spec stays open |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| DN honour gate in the SPF readers | `go test ./internal/plugins/ospf/spf -run 'DN'` |
| DN set on origination + VPN Route Tag | `go test ./internal/plugins/ospf/lsdb -run 'DN'` |
| DN LSA still flooded + stored | `go test ./internal/plugins/ospf/lsdb -run TestDNLSAStillFloodedAndStored` |
| PE-CE config resolves | `go test ./internal/plugins/ospf -run TestPECEConfigResolves` |
| Five metric series registered | `grep -rn 'ze_ospf_l3vpn_' internal/plugins/ospf` |
| PE-CE doctor check registered | `grep -rn 'doctor-ospf-l3vpn-auth' internal/plugins/ospf internal/core/diagnostic` |
| Interop scenario present | `ls test/interop/scenarios/ospf-l3vpn-frr/` |
| Functional tests present | `ls test/ospf/ospf-l3vpn-*.ci` |
| A-1-gated items marked blocked, not done | the Implementation Audit shows AC-7..AC-12 blocked with the reason "VRF/VPNv4 infrastructure absent (A-1)" |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the DN bit and the route tag come from untrusted CE LSAs; the gate is a bounded bit test + a 32-bit compare; no slice math; Ext-Community decode (gated on A-1) is bound-checked |
| Forged DN / tag | a forged DN (RFC 4576 §5) can make prefixes unreachable; a cleared DN can loop; the PE-CE doctor check warns when crypto auth is absent; reuse the existing OSPF auth -- no new auth surface |
| Route leakage | the honour gate must NOT silently drop routes in a non-PE-CE deployment (gated on the config flag); a DN-excluded route must not be re-exported to BGP (the producer never sees it) |
| Resource exhaustion | DN LSAs share the existing per-area LSDB caps; the gate adds no storage; the metrics counters are bounded |
| Error leakage | doctor warnings name the instance, not key material; no auth secret rendered in `show ospf` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase; if the AC is A-1-gated, mark blocked (do NOT fake it) |
| A-1-gated work cannot proceed (no VRF/VPNv4) | STOP that phase; keep the spec open; report the blocker to the user |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Core Insight
The DN bit is a **policy** layered on an existing carrier, not a wire change:
`OptionDN` already exists, the Type 3/5/7 origination methods already carry the
Options octet, and the SPF readers already inspect `lsa.Header.Options`. The
implementable-now core is two gates -- exclude DN/tag-marked candidates at the SPF
reader (honour) and OR `OptionDN` into the Options at origination (set) -- both
conditional on a PE-CE config flag, with the LSA still flooded and stored verbatim
(RFC 4576 §4 "no other effect"). Everything that touches the VPN backbone (Domain
ID / Route Type Ext-Community, VRF binding, sham-link /32 distribution) inherently
requires the MPLS L3VPN / VRF / VPNv4 infrastructure Ze does not yet have (A-1), and
those parts are specified but gated.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Gate the honour at the SPF reader, not the LSDB | drop DN LSAs at install | RFC 4576 §4 says DN has no effect on LSA handling; dropping breaks flooding/transit (R-1) |
| Make the honour conditional on a PE-CE config flag | always honour DN | a stray DN bit in a plain OSPF deployment must not silently drop routes (R-2); DN is meaningful only in the PE-CE context |
| Set DN by OR-ing `OptionDN` into the existing `opts` argument | a new DN-aware origination path | the origination methods already write Options verbatim; minimal change (A-3) |
| Carry the VPN Route Tag through the existing `tag` argument | a new tag field | `OriginateExternal` and the External-LSA body already carry an external route tag (A-7) |
| Specify, but explicitly gate, the VRF/VPNv4-dependent mechanics on A-1 | drop them from scope, or pretend they are doable now | the task requires the full RFC 4576/4577 OSPF mechanics specified; A-1 records that the infrastructure must land first; gating prevents false "done" (R-7) |
| Reuse the existing OSPF auth keystore for the §5/§6 crypto-auth recommendation | a new PE-CE auth surface | RFC 4576 §5 / RFC 4577 §6 point at standard OSPF crypto auth, already present (A-8) |

## Known Limitations
- The VRF/VPNv4-dependent ACs (AC-7..AC-12) cannot be implemented until the MPLS L3VPN / VRF / VPNv4 infrastructure spec lands (A-1); they are specified and gated, not delivered, by this spec.
- OSPFv3 PE-CE (RFC 6565) is out of scope; RFC 4576/4577 are OSPFv2-only.
- Sham-link forwarding (per the BGP route, multi-hop TTL) is the infrastructure spec's data path; this spec covers only the sham link's OSPF control-plane behaviour.
- There is no DN capability negotiation (RFC 4576 §5); correct loop prevention relies on every PE honouring DN, which Ze does only for PE-CE instances.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document on:
- `// RFC 4576 Section 4` -- DN set on PE->CE Type 3/5/7; clear on other types; ignored on other types; honoured (route-calc exclusion) on received 3/5/7; "no other effect on LSA handling"
- `// RFC 4577 Section 4.2.5.1 / 4.2.5.2` -- DN set on PE-originated LSAs; the VPN Route Tag default + configurability + the 4-byte-AS requirement
- `// RFC 4577 Section 4.2.6` -- honour DN / VPN Route Tag on receive; OSPF Route Type Ext Community present on PE-originated routes; Type 4 not redistributed
- `// RFC 4577 Section 4.2.4 / 4.2.8.1` -- Domain Identifier default NULL + equality rules
- `// RFC 4577 Section 4.2.8` -- LSA-type selection by domain match + route type; external => Area Number 0, FA 0
- `// RFC 4577 Section 4.2.7` -- sham link as a Type 1 link, Demand Circuit, up iff VRF route to the /32 endpoint, endpoint not advertised by OSPF

## Implementation Summary

### What Was Implemented
- [filled at implementation time]

### Bugs Found/Fixed
- [filled at implementation time]

### Documentation Updates
- [filled at implementation time]

### Deviations from Plan
- [filled at implementation time]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Blocked on A-1 (VRF/VPNv4 absent):** (must NOT be counted as done -- R-7)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| DN bit honoured on receive (Type 3/5/7 excluded from route-calc) | unit + functional + interop | `TestExternalReaderSkipsDN`, `ospf-l3vpn-dn-honour.ci`, `ospf-l3vpn-frr` |
| DN bit set on PE-originated Type 3/5/7 | unit + functional + interop | `TestOriginateExternalWithDN`, `ospf-l3vpn-dn-set.ci`, `ospf-l3vpn-frr` |
| DN LSA still flooded + stored (route-calc-only exclusion) | unit | `TestDNLSAStillFloodedAndStored` |
| VPN Route Tag set + honoured | unit + interop | `TestOriginateExternalCarriesVPNRouteTag`, `TestExternalReaderSkipsVPNRouteTag`, `ospf-l3vpn-frr` |
| Domain ID / Route Type community / sham link | unit | `TestDomainIDEquality`, `TestRouteTypeExtCommunityCodec`, `TestShamLinkUpAddsType1Link` -- **blocked on A-1** |
| PE-CE auth posture | functional | `ospf-l3vpn-doctor.ci` |

## Review Gate

<!-- BLOCKING: Run /ze-review BEFORE the final testing/verify step. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated (A-1-gated AC-7..AC-12 marked blocked, not done)
- [ ] End-to-End User Stories: every non-gated story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/*`, `internal/core/diagnostic/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary
- [ ] A-1-gated items shown blocked (not done); spec stays open if the infrastructure is absent

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC 4576 / RFC 4577 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (DN policy reuses the existing carrier)
- [ ] No speculative features (A-1-gated parts specified but not implemented ahead of the infrastructure)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior (DN honour conditional on the PE-CE flag)
- [ ] Minimal coupling (no "l3vpn" spelling in generic config/redistribute)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-l3vpn-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped/Blocked items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-13-l3vpn-dn-bit.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-13-l3vpn-dn-bit.md`
