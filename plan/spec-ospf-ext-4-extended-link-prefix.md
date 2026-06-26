# Spec: ospf-ext-4 -- OSPFv2 Extended Link/Prefix Opaque LSAs (RFC 7684)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-ext-1-opaque-framework.md |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc7684.md` -- the two LSAs (Opaque Type 7 / Type 8), the three top-level TLVs (Extended Prefix §2.1, Extended Link §3.1), the generic TLV/sub-TLV format (§2), malformed-LSA rules (§5), N-Flag/A-Flag semantics (§2.1), one-Extended-Link-TLV-per-LSA SHALL (§3.1), lowest-Opaque-ID tie-break (§2/§3)
4. `rfc/short/rfc5250.md` -- the carrier this rides on: LS Type 10 (area) / 11 (AS) scope (§3.1), the Opaque Type/ID split of the Link State ID (§3, App A.2), O-bit gating, §5 Type-11 reachability
5. `plan/spec-ospf-ext-1-opaque-framework.md` -- the dependency: `RegisterOpaqueConsumer(opaqueType, scope, onOrig, onRecv)`, `OriginateOpaque`, the generic 4-byte-aligned TLV iterator/builder in `internal/plugins/ospf/packet/opaque_tlv.go`, scope-correct flooding, verbatim re-flood of unregistered types. This spec is a CONSUMER of that carrier and adds NO carrier behaviour.
6. `internal/plugins/ospf/lsdb/origination.go` -- `OriginateRouter`/`OriginateNetwork`/`routerLinks`/`advertiseInterfaceLinks` (the Router/Network LSA origination that defines which prefixes/links this router advertises, to be mirrored by ext-prefix/link); `OriginateSelf`/`SelfLSAEncoder` (the self-origination seam ext-1's `OriginateOpaque` reuses)
7. `internal/plugins/ospf/spf/route.go` -- `RouteEntry{Prefix, Type RouteType}`, `RouteType` (`RouteIntraArea`/`RouteInterArea`/external), `stubPrefix`, `BuildRoutes` (the source of advertised intra/inter-area prefixes and their route types -> the Extended Prefix TLV Route Type field)
8. `internal/plugins/ospf/packet/lsa_router.go` -- `RouterLink{LinkID, LinkData, LinkType, Metric}`, `RouterLinkTypeP2P/Transit/Stub/Virtual` (the Link Type / Link ID / Link Data the Extended Link TLV mirrors from the Router-LSA, RFC 2328 §A.4.2)
9. `internal/plugins/ospf/instance.go` -- `originateSelfLSAs` (the topology-change origination trigger ext-prefix/link hangs off via ext-1's consumer `OnOriginate`), the route/prefix snapshots
10. `internal/plugins/ospf/show_database.go` -- `dbSubviewType` (the `show ospf database <type>` map to extend with `opaque-area`/`opaque-as` extended views)

## Task

Implement the OSPFv2 **Extended Prefix Opaque LSA** (Opaque Type 7) and
**Extended Link Opaque LSA** (Opaque Type 8) defined by RFC 7684, as **consumers
of the ext-1 opaque-LSA framework**. These two LSAs are TLV containers: the
Extended Prefix Opaque LSA carries one or more **Extended Prefix TLVs** (top-level
type 1) that associate attributes with prefixes; the Extended Link Opaque LSA
carries exactly one **Extended Link TLV** (top-level type 1) that associates
attributes with a single link. Both top-level TLVs nest **sub-TLVs** in the
RFC-3630-identical generic TLV format. This spec delivers the **carriage**: the
two LSA bodies, the three top-level TLV codecs (Extended Prefix TLV §2.1,
Extended Prefix Range TLV, Extended Link TLV §3.1), the sub-TLV walk, a
**sub-TLV registration hook** so later applications (SR / ext-5) attach their own
sub-TLV codecs, and the **association** of each advertised prefix/link with its
originating Router-LSA / Network-LSA so the right Route Type / Link Type / Link
ID / Link Data are emitted.

This is the carrier foundation that Segment Routing (ext-5) attaches Prefix-SID
and Adj-SID sub-TLVs to. This spec defines NO SID, NO label, NO SRGB: it builds
the empty containers (top-level TLVs with their fixed fields and a sub-TLV slot)
and the hook ext-5 plugs into. A built-without-ext-5 router originates Extended
Prefix/Link LSAs whose top-level TLVs carry their fixed fields and zero sub-TLVs,
floods them per scope (via ext-1), receives and decodes peers' Extended
Prefix/Link LSAs, walks their sub-TLVs (delivering unknown ones to nobody but
never crashing), and shows them under `show ospf database opaque-area`.

The two LSAs register with ext-1 as opaque consumers: Opaque Type 7 at scope
area (LS Type 10) or AS (LS Type 11), Opaque Type 8 at scope area (LS Type 10)
only. Origination is driven by `originateSelfLSAs` (the same topology-change
trigger that drives Router/Network LSAs); reception delivery comes from ext-1's
`OnReceive`. Removing this spec's code removes Opaque Type 7/8 registration and
all Extended Prefix/Link behaviour, leaving the ext-1 carrier intact (it would
then re-flood Type 7/8 verbatim as unregistered opaque LSAs).

### In scope (this spec)

| Item | Detail |
|------|--------|
| Extended Prefix Opaque LSA (Opaque Type 7) | body = one or more Extended Prefix TLVs; registers with ext-1 at scope area (LS Type 10) and AS (LS Type 11); origination + reception decode |
| Extended Link Opaque LSA (Opaque Type 8) | body = exactly one Extended Link TLV (§3.1 SHALL); registers with ext-1 at scope area (LS Type 10) only; origination + reception decode |
| Extended Prefix TLV (top-level type 1, §2.1) | fixed fields Route Type / Prefix Length / AF / Flags (A=0x80, N=0x40) / Address Prefix (32-bit for AF=0), then sub-TLV region |
| Extended Prefix Range TLV (RFC 8665 §4, present here as an empty carrier only) | the second Extended Prefix Opaque LSA top-level TLV; its fixed fields + sub-TLV slot are decoded/encoded; the RFC 7684 summary notes this TLV is defined by RFC 8665, not 7684, so it is carried as a container with NO range semantics (no SID, no SRGB) -- see Decision in Required Reading |
| Extended Link TLV (top-level type 1, §3.1) | fixed fields Link Type / Reserved / Link ID / Link Data (mirrored from the Router-LSA link, RFC 2328 §A.4.2), then sub-TLV region |
| Sub-TLV registration hook | `RegisterPrefixSubTLV(type, codec)` / `RegisterLinkSubTLV(type, codec)`: later applications (ext-5 SR) register a sub-TLV type + encode/decode; the carrier walks all sub-TLVs and dispatches known types to their codec, skips unknown via Length |
| Prefix/link <-> origin association | each Extended Prefix TLV's Route Type / Prefix derived from the router's advertised prefixes (the `spf/route.go` `RouteEntry` set + connected/stub prefixes from `lsdb/origination.go`); each Extended Link TLV's Link Type / Link ID / Link Data derived from the matching `packet.RouterLink` (`OriginateRouter`/`routerLinks`) so the Extended LSA references a real Router-LSA link |
| Malformed-LSA guard (§5) | a TLV/sub-TLV overrunning the subsuming LSA/TLV/sub-TLV, or trailing data smaller than a TLV header, makes the whole LSA malformed: NOT stored, NOT acked, NOT reflooded; counted/logged |
| N-Flag / A-Flag handling (§2.1) | N-Flag ignored if the prefix is not a host prefix; N-Flag preserved on inter-area propagation; A-Flag set by an ABR for an inter-area prefix locally connected in another connected area |
| Tie-break + dedup (§2/§3) | duplicate Extended Prefix TLV for the same prefix in one LSA -> use first, log; same prefix across LSAs from the same router -> use lowest Opaque ID; duplicate Extended Link TLV in one LSA -> use first, log |
| CLI / show | `show ospf database opaque-area` / `opaque-as` render Extended Prefix/Link LSAs decoded (Route Type, prefix, flags, Link Type/ID/Data, sub-TLVs as type/len/hex) |
| Config | a `extended-prefix` / `extended-link` enable leaf gating origination (off until SR or another sub-TLV producer needs it); decode/store of received LSAs is always on once the plugin is built |

### Out of scope (dependent spec; noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| Prefix-SID sub-TLV, Adj-SID / LAN-Adj-SID sub-TLV (the SR sub-TLV values) | spec-ospf-ext-5 (Segment Routing) |
| SID/label allocation, SRGB/SRLB, Extended Prefix Range *semantics* (range advertisement of prefix SIDs) | spec-ospf-ext-5 |
| The ext-1 opaque carrier itself (scope flooding, O-bit, LS-ID split, verbatim re-flood, §5 Type-11 reachability, the generic TLV iterator/builder) | spec-ospf-ext-1 (this spec consumes it) |
| OSPFv3 extended LSAs (RFC 8362 replaces fixed LSAs with TLV LSAs, a different mechanism, not opaque) | not applicable to RFC 7684; `internal/plugins/ospf/v3` is untouched |
| Any route-table change from Extended Prefix/Link attributes | none -- like all opaque LSAs these never become SPF vertices and install no route (RFC 5250 §3) |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/research/ospf-implementation-guide.md` lines 1530-1533 ("Extended Link and Extended Prefix Opaque LSAs (RFC 7684)") -- positions these LSAs as the SR foundation
  -> Decision: build the carrier (containers + sub-TLV hook) now and defer all SID semantics to ext-5, exactly as the guide says ("the foundation for Segment Routing")
  -> Constraint: Extended Link/Prefix LSAs are opaque LSAs -- they ride ext-1's RFC-5250 flooding and never feed SPF or the route table
- [ ] `plan/spec-ospf-ext-1-opaque-framework.md` "In scope" / "Files to Create" -- the carrier this spec plugs into
  -> Decision: register Opaque Type 7 and 8 via `RegisterOpaqueConsumer`; originate via `OnOriginate` returning `(opaqueID, scope, body, withdraw)`; receive via `OnReceive(opaqueID, body, scope, advRouter, reachable)`; reuse the generic TLV iterator/builder from `internal/plugins/ospf/packet/opaque_tlv.go` -- this spec adds NO new carrier primitive
  -> Constraint: scope is a property of the LS Type owned by the carrier; Type 7 uses scope area (10) or AS (11), Type 8 uses scope area (10) only; the carrier rejects a scope the consumer does not declare
- [ ] `ai/rules/plugin-self-containment.md` -- removing this spec removes Opaque Type 7/8 and all Extended Prefix/Link behaviour
  -> Constraint: no SR / SID / ext-5 spelling appears in this spec's code; the sub-TLV registry is generic (`RegisterPrefixSubTLV`/`RegisterLinkSubTLV`), and ext-5 registers from its own `init()`
- [ ] `ai/rules/buffer-first.md` -- the two LSA bodies and the three top-level TLVs are buffer-first
  -> Constraint: every body/TLV is emitted via `WriteTo(buf, off) int` over a caller-owned buffer using ext-1's TLV builder; the 4-byte alignment pad is written, never produced by slice concatenation; decode returns views over the received bytes (zero-copy), no per-TLV allocation
- [ ] `ai/rules/no-sprintf-alloc.md` -- rendering uses `textbuf`/`AppendTo`
  -> Constraint: `show ospf database opaque-*` extended rendering (Route Type, prefix, flags, Link Type/ID/Data, sub-TLV type/len/hex) uses `textbuf`, never `fmt.Sprintf` or `+`

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7684.md` -- the two LSAs and three top-level TLVs
  -> Constraint: §2 generic TLV format -- Length counts only value octets; total TLV size = `4 + Length + pad` where `pad = (4 - (Length mod 4)) mod 4`; padding is zeros and excluded from Length; a 3-octet value has Length 3 but occupies 8 octets
  -> Constraint: §2.1 Extended Prefix TLV -- Route Type (0 Unspecified / 1 Intra-Area / 3 Inter-Area / 5 AS-External / 7 NSSA-External), Prefix Length, AF (0 = IPv4 unicast only), Flags (A=0x80, N=0x40), then a 32-bit Address Prefix for AF=0 regardless of Prefix Length, then sub-TLVs
  -> Constraint: §2.1 N-Flag -- if set but the prefix length is not a host prefix the flag MUST be ignored (not malformed); the N-Flag is preserved when the LSA is propagated between areas
  -> Constraint: §3.1 Extended Link TLV -- exactly one SHALL be advertised per Extended Link Opaque LSA; fixed fields Link Type / Reserved (3 bytes) / Link ID / Link Data take their meaning from the Router-LSA link (RFC 2328 §A.4.2), then sub-TLVs
  -> Constraint: §5 malformed -- a TLV/sub-TLV that overruns the subsuming LSA/TLV/sub-TLV, or trailing data smaller than a TLV header, makes the whole LSA malformed: it MUST NOT be stored in the LSDB, acknowledged, or reflooded; reception SHOULD be counted/logged
  -> Constraint: §2/§3 dedup -- same prefix in multiple Extended Prefix Opaque LSAs from the same router -> use the lowest Opaque ID; duplicate Extended Prefix/Link TLV in one LSA -> use the first instance and log
  -> Decision: RFC 7684 does NOT define an "Extended Prefix Range TLV" (that is RFC 8665 §4); carry it here ONLY as a fixed-field + sub-TLV container with no range semantics, so ext-5 can fill it; do not invent range fields absent from RFC 8665
- [ ] `rfc/short/rfc5250.md` -- the carrier semantics the LSAs inherit (read via ext-1, summarized here)
  -> Constraint: §3, App A.2 -- the Link State ID is `(OpaqueType << 24) | OpaqueID`; Opaque Type 7/8 occupies the high octet; the Opaque ID is an arbitrary instance differentiator with no semantics (the carrier owns this split; this spec only chooses Opaque IDs)
  -> Constraint: §3.1 -- Type 10 (area) Extended Prefix/Link LSAs never leave their area; Type 11 (AS) Extended Prefix LSAs are not flooded into stub/NSSA; the carrier enforces this, this spec must pick LS Type 10 vs 11 per the prefix scope (§2.1: scope MUST satisfy every prefix in the LSA)
  -> Constraint: §5 -- for a received Type-11 Extended Prefix LSA, the carrier supplies `reachable`; this spec treats an unreachable-originator LSA as present-but-unusable (no attribute application)

**Key insights:** (minimal context to resume after compaction)
- This is a pure **consumer** spec: two LSA bodies + three top-level TLV codecs + a sub-TLV registry hook + prefix/link-to-origin association. The flooding, scope, O-bit, LS-ID split, and verbatim re-flood are all ext-1's; this spec never touches them.
- The Extended Prefix TLV's Route Type comes from the router's advertised route set (`spf/route.go` `RouteType`); the Extended Link TLV's Link Type/ID/Data come from the matching Router-LSA link (`packet/lsa_router.go` `RouterLink`). The "association" requirement is exactly this mapping.
- This spec ships **empty containers**: the top-level TLVs carry their fixed fields and zero sub-TLVs until ext-5 registers Prefix-SID/Adj-SID codecs. A built-without-ext-5 router is fully RFC-7684-conformant (RFC 7684 defines only the containers; sub-TLV *values* live in RFC 8665).
- The sub-TLV registry is the SR attachment point: `RegisterPrefixSubTLV`/`RegisterLinkSubTLV` mirror ext-1's `RegisterOpaqueConsumer` self-containment pattern one level down.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/plugins/ospf/packet/lsa_opaque.go` -- `OpaqueLSA{Type, Data}` is the raw opaque body; opaque types 9/10/11 are retained verbatim in v1; the codec does not interpret bodies
  -> Constraint: Type 7/8 are Opaque *Types* (the high octet of the LS ID), carried inside LS Type 10/11 opaque LSAs; the existing `OpaqueLSA` raw body is the bytes this spec's Extended Prefix/Link codec parses -- the new codec lives beside it, it does not replace passthrough
- [ ] `internal/plugins/ospf/packet/lsa_router.go` -- `RouterLink{LinkID types.LinkStateID, LinkData [4]byte, LinkType, Metric}`; `RouterLinkTypeP2P=1`, `Transit=2`, `Stub=3`, `Virtual=4`; `LinkData` is a raw 4-octet field whose meaning depends on `LinkType`
  -> Constraint: the Extended Link TLV's Link Type / Link ID / Link Data fields are exactly these three `RouterLink` fields (RFC 2328 §A.4.2); the association code reads the originated Router-LSA's links and emits one Extended Link Opaque LSA per advertised link that needs attributes
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `OriginateRouter`/`routerLinks`/`advertiseInterfaceLinks` build the Router-LSA link set; `OriginateNetwork` builds the Network-LSA; `OriginateSelf(area, key, body, enc)` + `SelfLSAEncoder` is the self-LSA origination seam (sequence, MinLSInterval, install, flood); `FlushStaleSelfLSAs` withdraws stale self-LSAs
  -> Constraint: `routerLinks` is the authoritative list of this router's advertised links; the Extended Link association mirrors it. `OriginateSelf` is what ext-1's `OriginateOpaque` calls; this spec does not re-implement sequencing/flooding
  -> Constraint: `FlushStaleSelfLSAs` is the withdrawal pattern; when an advertised prefix/link disappears, the corresponding Extended Prefix/Link Opaque LSA must be withdrawn through ext-1's `OnOriginate(withdraw=true)`, which routes to the carrier's MaxAge purge
- [ ] `internal/plugins/ospf/spf/route.go` -- `RouteEntry{Prefix netip.Prefix, Type RouteType, Metric, NextHops}`; `RouteType` is `RouteIntraArea`(1)/`RouteInterArea`(2)/external; `BuildRoutes` selects routes; `stubPrefix(id, mask)` reconstructs a prefix from a stub link
  -> Constraint: the Extended Prefix TLV Route Type maps from `RouteType` (Intra-Area->1, Inter-Area->3, external->5/7); the prefix set this router advertises Extended Prefix TLVs for is its connected/stub prefixes (intra-area) plus, for an ABR, inter-area prefixes -- derived from the same data `BuildRoutes`/`routerLinks` use, NOT recomputed
- [ ] `internal/plugins/ospf/instance.go` -- `originateSelfLSAs()` regenerates self-LSAs on topology change (`neighborEventSink{onChange: e.originateSelfLSAs}`) and on the periodic retransmit tick; `routeSnapshot`/`databaseSnapshot` expose state to CLI
  -> Constraint: ext-1 invokes each opaque consumer's `OnOriginate` from inside (or alongside) `originateSelfLSAs`; this spec's consumer recomputes its Extended Prefix/Link LSA set there, so prefix/link changes drive Extended LSA re-origination on the same cadence as Router/Network LSAs
- [ ] `internal/plugins/ospf/show_database.go` -- `dbSubviewType` maps `show ospf database <type>` to a snapshot type for Types 1-5/7; there is no opaque sub-view yet
  -> Constraint: ext-1 adds `opaque-link`/`opaque-area`/`opaque-as` raw views; this spec decorates the area/as views so Extended Prefix/Link bodies render decoded instead of hex (a registered-Opaque-Type renderer the carrier calls), keeping the carrier ignorant of Type 7/8
- [ ] `internal/plugins/ospf/types/lstype.go` -- `LSTypeOpaqueArea`(10), `LSTypeOpaqueAS`(11), `IsOpaque()`; `internal/plugins/ospf/packet/opaque_tlv.go` (ext-1) -- the generic 4-byte-aligned TLV iterator + builder
  -> Constraint: this spec consumes `opaque_tlv.go`; its three top-level TLV codecs and the sub-TLV walk are thin layers over that iterator/builder; if ext-1 is not yet delivered, this spec's first audit step confirms those primitives exist

**Behavior to preserve:**
- ext-1's carrier contract: scope flooding, the O-bit gate, the LS-ID split, verbatim re-flood of unregistered opaque LSAs, §5 Type-11 reachability, the `RegisterOpaqueConsumer`/`OnOriginate`/`OnReceive` signatures, the `OriginateOpaque` seam, and `packet/opaque_tlv.go`.
- The OSPFv2 Router/Network/Summary/External origination (`OriginateRouter`/`OriginateNetwork`/`OriginateSummary`/`OriginateExternal`) and the `RouterLink`/`RouteEntry` shapes -- this spec reads them, it does not change them.
- All existing OSPFv2 and OSPFv3 functional/interop tests: a router without an Extended Prefix/Link producer behaves as today plus, once enabled, originates empty-container Extended Prefix/Link LSAs and decodes peers'.

**Behavior to change:** (all RFC-7684 carriage, gated by the enable leaf for origination; decode is always on)
- Register Opaque Type 7 (scope area + AS) and Opaque Type 8 (scope area) with ext-1.
- Originate Extended Prefix/Link Opaque LSAs from `originateSelfLSAs` when enabled, associated with advertised prefixes/links; withdraw on disappearance.
- Decode received Extended Prefix/Link Opaque LSAs (always on): walk top-level TLVs and sub-TLVs, dispatch known sub-TLVs to registered codecs, enforce §5 malformed rules, apply N-Flag ignore / dedup / lowest-Opaque-ID tie-break.
- Render Extended Prefix/Link bodies under `show ospf database opaque-area`/`opaque-as`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Origination:** a topology/prefix/link change -> `originateSelfLSAs` -> ext-1 invokes this consumer's `OnOriginate` -> this spec computes the Extended Prefix/Link LSA set (one Extended Prefix Opaque LSA per scope grouping; one Extended Link Opaque LSA per advertised link with attributes) -> returns `(opaqueID, scope, body, withdraw)` to ext-1, which sequences, installs, and floods via `OriginateOpaque`.
- **Reception:** an LS Update carrying an opaque LSA whose Opaque Type is 7 or 8 -> ext-1 carrier decodes the opaque header, applies scope + §5 reachability -> calls this consumer's `OnReceive(opaqueID, body, scope, advRouter, reachable)` -> this spec parses the body's top-level TLVs and sub-TLVs.
- **Sub-TLV registration:** ext-5 (or any future application) calls `RegisterPrefixSubTLV(type, codec)` / `RegisterLinkSubTLV(type, codec)` from its own `init()`; this spec stores the codec and dispatches matching sub-TLVs to it during decode and lets it contribute bytes during origination.
- **CLI:** `show ospf database opaque-area`/`opaque-as` -> the carrier snapshot -> this spec's registered renderer for Opaque Type 7/8.

### Transformation Path
1. **Origination input (new):** read the router's advertised prefixes (`spf/route.go` `RouteEntry` set + connected/stub prefixes from `lsdb/origination.go`) and advertised links (`routerLinks`/the originated Router-LSA links); for each, derive Route Type / Link Type / Link ID / Link Data.
2. **Body build (new, buffer-first):** for Extended Prefix, append each Extended Prefix TLV (fixed fields + 32-bit Address Prefix) then ask each registered prefix sub-TLV codec to append its bytes (none until ext-5); for Extended Link, append the single Extended Link TLV (fixed fields) then the registered link sub-TLV codecs; use ext-1's TLV builder for 4-byte alignment.
3. **Carrier origination (ext-1):** `OnOriginate` returns the body, scope (10 or 11), and chosen Opaque ID; ext-1 builds the opaque LSA header (LS ID = `7<<24 | id` or `8<<24 | id`), assigns sequence, installs, floods. Withdraw -> ext-1 MaxAge purge.
4. **Carrier reception (ext-1):** the carrier hands `(opaqueID, body, scope, advRouter, reachable)` to `OnReceive`.
5. **Body decode (new):** walk top-level TLVs with ext-1's iterator; for type 1 in a Type-7 LSA parse the Extended Prefix TLV fixed fields then recurse into sub-TLVs; for the Extended Prefix Range TLV parse its fixed fields then sub-TLVs; for type 1 in a Type-8 LSA parse the single Extended Link TLV then sub-TLVs. Each sub-TLV: dispatch known type to its registered codec, skip unknown via Length.
6. **Validation (new):** before storing the decoded result, enforce §5 (overrun/short-trailing -> malformed -> reject, do not store/ack/reflood -- the carrier already refused to store; this spec signals malformed so the carrier drops it), N-Flag ignore on non-host prefix, dedup (first instance in one LSA; lowest Opaque ID across LSAs).
7. **Render (new):** the registered renderer formats Route Type / prefix / flags / Link Type-ID-Data / sub-TLV (type, len, hex, or a registered sub-TLV's own string) via `textbuf`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Advertised prefixes/links <-> Extended TLV fields | read `spf/route.go` `RouteEntry`/`RouteType` + `lsdb` `routerLinks` (read-only) | [ ] |
| Extended LSA body <-> wire | this spec's top-level TLV codecs over ext-1's `opaque_tlv.go` iterator/builder (buffer-first, zero-copy decode) | [ ] |
| Consumer <-> ext-1 carrier | `RegisterOpaqueConsumer`(7,8); `OnOriginate`/`OnReceive`; value-typed payloads, no cross-boundary pointers | [ ] |
| Sub-TLV codec <-> this spec | `RegisterPrefixSubTLV`/`RegisterLinkSubTLV`; encode/decode callbacks; ext-5 registers from its own `init()` | [ ] |
| Type-11 Extended Prefix <-> reachability | `reachable` from ext-1 (§5); unreachable -> present-but-unusable | [ ] |
| Extended LSA <-> CLI snapshot | registered Opaque-Type renderer the carrier calls for `opaque-area`/`opaque-as` | [ ] |

### Integration Points
- `internal/plugins/ospf/packet` -- new Extended Prefix/Link TLV codecs alongside `lsa_opaque.go`, built on ext-1's `opaque_tlv.go`.
- `internal/plugins/ospf` (engine) -- the two opaque consumers' `init()` registrations; the origination computation in `originateSelfLSAs`; the reception decode in `OnReceive`; the sub-TLV registries.
- `internal/plugins/ospf/spf` and `internal/plugins/ospf/lsdb` -- READ ONLY: the advertised prefix set (`RouteEntry`/`RouteType`/`stubPrefix`) and link set (`routerLinks`); no change to SPF or route install.
- `internal/plugins/ospf/show_database.go` + `cmd_show.go` -- the extended renderer registered with the carrier's opaque snapshot.
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` + `config.go` -- the `extended-prefix`/`extended-link` enable leaves.
- ext-1 (`internal/plugins/ospf/opaque_registry.go`, `opaque.go`, `packet/opaque_tlv.go`) -- consumed, never modified.

### Architectural Verification
- [ ] No bypassed layers (Extended LSAs flow through ext-1's carrier: origination via `OnOriginate`/`OriginateOpaque`, reception via `OnReceive`; no direct LSDB or flooding access)
- [ ] No unintended coupling (this spec names no SR/SID; the sub-TLV registry is generic; ext-1 names no Type 7/8)
- [ ] No duplicated functionality (reuses ext-1's TLV iterator/builder, carrier flooding, LS-ID split, §5 reachability; reuses `routerLinks`/`RouteEntry` for association; adds only the Type 7/8 bodies, top-level TLV codecs, sub-TLV registry, and renderer)
- [ ] Zero-copy preserved (decode returns views over received bytes; origination is buffer-first via the TLV builder)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ext-1 delivers `RegisterOpaqueConsumer`, `OnOriginate`/`OnReceive`, `OriginateOpaque`, and the generic TLV iterator/builder in `packet/opaque_tlv.go` before this spec implements | `plan/spec-ospf-ext-1-opaque-framework.md` "Files to Create" lists `opaque_registry.go`, `opaque.go`, `packet/opaque_tlv.go`; Depends row | this spec must build carrier primitives -> scope creep / duplication | first audit step greps for those symbols; `TestExtPrefixRegistersAsOpaqueConsumer` | unvalidated |
| A-2 | The Extended Link TLV Link Type/ID/Data are exactly the `packet.RouterLink` fields (RFC 2328 §A.4.2) and `routerLinks` is the authoritative advertised-link list | `packet/lsa_router.go` `RouterLink`; `lsdb/origination.go` `routerLinks`; RFC 7684 §3.1 | the association emits wrong link identity -> FRR cannot correlate the Extended Link LSA with the Router-LSA link | `TestExtLinkMirrorsRouterLSALink`; `ospf-ext-prefix-link-frr` interop | unvalidated |
| A-3 | The Extended Prefix TLV Route Type maps directly from `spf.RouteType` / OSPFv2 LS type (Intra=1, Inter=3, Ext=5, NSSA=7) and the advertised-prefix set is derivable from `RouteEntry` + connected/stub prefixes | `spf/route.go` `RouteType`/`RouteEntry`/`stubPrefix`; RFC 7684 §2.1 | wrong Route Type or missing prefixes -> attributes fail to correlate | `TestExtPrefixRouteTypeMapping`; `ospf-ext-prefix-link-frr` | unvalidated |
| A-4 | RFC 7684 defines only the containers; no sub-TLV *value* is required for conformance, so an empty-sub-TLV Extended Prefix/Link LSA is valid and FRR accepts/floods it | RFC 7684 "Foundation Layering" table (registries seeded with only Reserved/type-1); §4 backward compatibility | FRR rejects an empty container -> the carrier-only deliverable is unprovable without ext-5 | `ospf-ext-prefix-link-frr` (FRR floods Ze's empty-container LSA); `TestExtPrefixEmptyContainerRoundTrip` | unvalidated |
| A-5 | The Extended Prefix Range TLV can be carried as a fixed-field + sub-TLV container with no range semantics, since its semantics belong to RFC 8665, not RFC 7684 | RFC 7684 summary "RFC 7684 does NOT define an Extended Prefix Range TLV ... RFC 8665 §4" | inventing range fields not in any read RFC | spec review: no field absent from RFC 8665 §4 is added; ext-5 owns range semantics | unvalidated |
| A-6 | ext-1 supplies `reachable` for received Type-11 opaque LSAs (§5), so this spec needs no separate reachability tracking | ext-1 `OnReceive(... reachable)`; RFC 5250 §5 | this spec must compute ASBR reachability -> duplicates ext-1 | `TestExtPrefixType11UnreachableUnusable` reuses ext-1's reachable flag | unvalidated |
| A-7 | `originateSelfLSAs` is the correct trigger and runs whenever advertised prefixes/links change (interface up/down, neighbor up, ABR summary change) | `instance.go` `neighborEventSink{onChange: e.originateSelfLSAs}` and the retransmit-tick call | Extended LSAs go stale on topology change | `TestExtLinkReoriginatesOnTopologyChange`; `ospf-ext-prefix-link-frr` after a link flap | unvalidated |
| A-8 | No YANG range/enum is needed beyond a boolean enable; Opaque ID assignment is internal (ascending per §2.1/§3.1 recommendation), not configured | RFC 7684 §2/§3 (Opaque ID has no semantics); `config.go` boolean leaves precedent | unnecessary config surface | `extended-prefix`/`extended-link` are native booleans; `TestExtConfigEnableLeaf` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | TLV 4-byte mis-padding (Length excludes pad; a 3-octet value occupies 8) -> FRR rejects the body | self round-trip passes, FRR drops the LSA | reuse ext-1's pad-correct TLV builder/iterator; `TestExtTLVAlignment` for value lengths 0..7; decode an FRR Extended Prefix/Link capture |
| R-2 | Malformed-LSA handling weaker than §5 (a sub-TLV overrunning its parent stored/reflooded) -> a crafted LSA propagates corruption | fuzz finds a stored over-length sub-TLV | bound-checked walk that fails the whole LSA on any overrun; signal malformed to ext-1 so it is not stored/acked/reflooded; `TestExtMalformedNotStored`; extend the packet fuzz target |
| R-3 | Extended Link LSA carrying more than one Extended Link TLV (violates §3.1 SHALL) | interop log "multiple Extended Link TLVs" | origination emits exactly one Extended Link TLV per LSA (one LSA per link); decode uses the first and logs extras; `TestExtLinkSingleTLVEnforced` |
| R-4 | Wrong LS Type scope (Type 7 at AS scope for an area-local prefix, or Type 8 at AS scope) -> §2.1 scope violation / FRR rejects | FRR floods it into the wrong scope or drops it | Type 8 always area (10); Type 7 area (10) for intra/inter-area prefixes, AS (11) only when the prefix scope demands; `TestExtPrefixScopeSelection` |
| R-5 | N-Flag mishandled: set on a non-host prefix treated as malformed (must be ignored), or dropped on inter-area propagation | a host-route attribute lost across an ABR | ignore-not-reject on non-host; preserve N-Flag through ABR re-origination; `TestExtPrefixNFlagIgnoredNonHost`, `TestExtPrefixNFlagPreservedInterArea` |
| R-6 | Stale Extended LSA after a prefix/link disappears (no withdraw) | a peer keeps a withdrawn prefix's attributes until MaxAge | withdraw via `OnOriginate(withdraw=true)` -> ext-1 MaxAge purge; `TestExtPrefixWithdrawOnPrefixGone` |
| R-7 | Sub-TLV registry leaks an SR/ext-5 name into this carrier spec (self-containment violation) | grep finds "sid"/"sr" in this spec's files | the registry is generic (type+codec); `TestSubTLVRegistryGenericNoSRSpelling` greps the package for forbidden spellings |
| R-8 | Dedup wrong: same prefix in two LSAs uses the higher Opaque ID, or duplicate in one LSA uses the last | attributes from the wrong LSA applied | lowest-Opaque-ID across LSAs, first-instance in one LSA, both logged; `TestExtPrefixLowestOpaqueIDWins`, `TestExtPrefixDuplicateInLSAFirstWins` |
| R-9 | A registered sub-TLV codec panics during decode/origination and takes down OSPF | one bad sub-TLV codec crashes the engine | recover around sub-TLV codec callbacks (mirror ext-1's consumer recover), count a metric; `TestSubTLVCodecPanicIsolated` |
| R-10 | Address Prefix decoded as variable-length (OSPFv3 style) instead of fixed 32-bit for AF=0 -> off-by-N parse | sub-TLVs parse at the wrong offset | AF=0 Address Prefix is always 32 bits regardless of Prefix Length (§2.1); `TestExtPrefixAddressPrefixFixed32` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| this spec's `init()` registers Opaque Type 7 + 8 with ext-1 | -> | `RegisterOpaqueConsumer(7, ...)` and `RegisterOpaqueConsumer(8, ...)`; the engine discovers both at startup | `TestExtPrefixRegistersAsOpaqueConsumer`, `TestExtLinkRegistersAsOpaqueConsumer` (unit) + `test/ospf/ospf-ext-register.ci` |
| `extended-prefix` enabled + an advertised prefix changes | -> | `originateSelfLSAs` -> consumer `OnOriginate` -> Extended Prefix TLV body -> `OriginateOpaque` -> install + flood | `test/ospf/ospf-ext-prefix-originate.ci` |
| `extended-link` enabled + an advertised link changes | -> | `originateSelfLSAs` -> consumer `OnOriginate` -> single Extended Link TLV body -> `OriginateOpaque` -> install + flood | `test/ospf/ospf-ext-link-originate.ci` |
| an LS Update carrying an Opaque Type 7 LSA arrives | -> | ext-1 carrier -> `OnReceive` -> Extended Prefix TLV + sub-TLV walk -> stored decoded | `test/ospf/ospf-ext-prefix-receive.ci` |
| ext-5 (test stub) calls `RegisterPrefixSubTLV(type, codec)` | -> | the sub-TLV registry stores the codec; decode dispatches a matching sub-TLV to it; origination lets it append bytes | `TestRegisterPrefixSubTLVDispatched` (unit) + `test/ospf/ospf-ext-subtlv-hook.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The OSPF plugin starts with this spec built | Opaque Type 7 is registered with ext-1 at scope area + AS, Opaque Type 8 at scope area; both appear in the carrier's consumer set |
| AC-2 | `extended-prefix` enabled and the router advertises an intra-area connected prefix | an Extended Prefix Opaque LSA (Opaque Type 7, LS Type 10) is originated containing an Extended Prefix TLV with Route Type 1 (Intra-Area), correct Prefix Length, AF 0, a 32-bit Address Prefix, and zero sub-TLVs |
| AC-3 | `extended-link` enabled and the router advertises a point-to-point link | an Extended Link Opaque LSA (Opaque Type 8, LS Type 10) is originated containing exactly one Extended Link TLV whose Link Type/Link ID/Link Data equal the matching Router-LSA link's fields |
| AC-4 | An ABR advertises an inter-area prefix locally connected in another connected area | the Extended Prefix TLV uses Route Type 3 (Inter-Area) and sets the A-Flag (0x80) (§2.1) |
| AC-5 | An Extended Prefix TLV sets the N-Flag (0x40) but the prefix length is not a host prefix | the N-Flag is ignored on receive (not treated as malformed) (§2.1) |
| AC-6 | An Extended Prefix Opaque LSA is propagated between areas by an ABR with the N-Flag set on a host prefix | the N-Flag is preserved in the re-originated LSA (§2.1) |
| AC-7 | A received Extended Prefix/Link LSA contains a TLV or sub-TLV that overruns the subsuming LSA/TLV, or trailing data smaller than a TLV header | the whole LSA is treated as malformed: not stored, not acknowledged, not reflooded; a malformed-LSA metric is incremented (§5) |
| AC-8 | A received Extended Link Opaque LSA carries more than one Extended Link TLV | only the first Extended Link TLV is used; the extra is logged (§3.1 SHALL: one per LSA) |
| AC-9 | The same prefix appears in two Extended Prefix Opaque LSAs from the same router with different Opaque IDs | the attributes from the LSA with the lowest Opaque ID are used (§2) |
| AC-10 | The same prefix appears twice in one Extended Prefix Opaque LSA | the first Extended Prefix TLV instance is used; the duplicate is logged (§2.1) |
| AC-11 | ext-5 (or a test stub) calls `RegisterPrefixSubTLV(t, codec)` / `RegisterLinkSubTLV(t, codec)` and a received LSA carries a sub-TLV of type `t` | the sub-TLV is dispatched to the registered codec; a sub-TLV of an unregistered type is skipped via its Length without error |
| AC-12 | This spec is built WITHOUT any sub-TLV producer (no ext-5) | Extended Prefix/Link LSAs are originated with zero sub-TLVs, decoded on receipt, and flooded per scope; nothing crashes; the empty-container LSA round-trips byte-for-byte |
| AC-13 | An advertised prefix or link disappears (interface down, summary withdrawn) | the corresponding Extended Prefix/Link Opaque LSA is withdrawn via `OnOriginate(withdraw=true)` -> ext-1 MaxAge purge |
| AC-14 | A received Type-11 Extended Prefix LSA whose originating router is unreachable (ext-1 `reachable=false`) | its attributes are treated as present-but-unusable (not applied) (RFC 5250 §5) |
| AC-15 | `show ospf database opaque-area` / `opaque-as` with a stored Extended Prefix/Link LSA | the output decodes Route Type, prefix, flags, Link Type/ID/Data, and each sub-TLV as (type, length, hex or registered-codec string), not raw opaque hex |
| AC-16 | A received sub-TLV's registered codec panics during decode | the engine recovers, increments a sub-TLV-error metric, and continues processing the rest of the LSA / other LSAs |
| AC-17 | An Extended Prefix TLV for AF=0 (IPv4 unicast) with any Prefix Length 0..32 | the Address Prefix is parsed as a fixed 32-bit field and sub-TLV parsing resumes at the correct offset (not variable-length) (§2.1) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables `extended-prefix`; the router advertises a loopback prefix | config -> engine -> `originateSelfLSAs` -> `OnOriginate` -> Extended Prefix TLV body -> ext-1 `OriginateOpaque` -> flood; FRR's `show ip ospf database opaque-area` lists the Extended Prefix LSA | `test/ospf/ospf-ext-prefix-originate.ci` + `ospf-ext-prefix-link-frr` interop |
| 2 | Enables `extended-link`; a p2p adjacency comes up | engine -> `OnOriginate` -> single Extended Link TLV mirroring the Router-LSA link -> ext-1 flood; FRR correlates it to the Router-LSA link | `test/ospf/ospf-ext-link-originate.ci` + `ospf-ext-prefix-link-frr` |
| 3 | Receives FRR's Extended Prefix/Link Opaque LSAs | wire -> ext-1 carrier -> `OnReceive` -> top-level TLV + sub-TLV walk -> stored; `show ospf database opaque-area` decodes them | `test/ospf/ospf-ext-prefix-receive.ci` + `ospf-ext-prefix-link-frr` |
| 4 | Later builds with ext-5 SR which registers a Prefix-SID sub-TLV | ext-5 `init()` -> `RegisterPrefixSubTLV` -> the existing Extended Prefix LSA now carries the SR sub-TLV; decode dispatches it | `TestRegisterPrefixSubTLVDispatched` + `test/ospf/ospf-ext-subtlv-hook.ci` |
| 5 | Runs `ze` decode on an Extended Prefix/Link LSA hex | CLI -> `packet.DecodeLSA` -> Opaque Type 7/8 -> Extended TLV codec -> rendered Route Type/prefix/flags or Link Type/ID/Data + sub-TLVs | `test/ospf/ospf-ext-decode.ci` |
| 6 | Builds without ext-5; peers exchange empty-container Extended LSAs | `OnOriginate` zero sub-TLVs -> flood -> peer decodes empty container; full adjacency unaffected | `TestExtPrefixEmptyContainerRoundTrip` + `ospf-ext-prefix-link-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtPrefixTLVRoundTrip` | `internal/plugins/ospf/packet/ext_prefix_test.go` | AC-2/AC-12: Extended Prefix TLV fixed fields + 32-bit prefix + empty sub-TLVs encode/decode byte-for-byte | |
| `TestExtPrefixAddressPrefixFixed32` | `internal/plugins/ospf/packet/ext_prefix_test.go` | AC-17, R-10: AF=0 Address Prefix is 32 bits for any Prefix Length; sub-TLV offset correct | |
| `TestExtPrefixRangeTLVContainerRoundTrip` | `internal/plugins/ospf/packet/ext_prefix_test.go` | A-5: Extended Prefix Range TLV carried as fixed-field + sub-TLV container, no range semantics | |
| `TestExtLinkTLVRoundTrip` | `internal/plugins/ospf/packet/ext_link_test.go` | AC-3/AC-12: Extended Link TLV Link Type/Reserved/Link ID/Link Data + empty sub-TLVs round-trip | |
| `TestExtTLVAlignment` | `internal/plugins/ospf/packet/ext_tlv_test.go` | R-1: top-level + sub-TLV 4-byte padding for value lengths 0..7 (via ext-1 builder/iterator) | |
| `TestExtMalformedNotStored` | `internal/plugins/ospf/packet/ext_tlv_test.go` | AC-7, R-2: overrun / short-trailing -> malformed signal, never partial-stored, never panics | |
| `TestExtLinkSingleTLVEnforced` | `internal/plugins/ospf/packet/ext_link_test.go` | AC-8, R-3: decode uses first Extended Link TLV, logs extras; origination emits exactly one | |
| `TestExtPrefixRouteTypeMapping` | `internal/plugins/ospf/ext_prefix_origin_test.go` | AC-2/AC-4, A-3: `spf.RouteType` -> Route Type 1/3/5/7; A-Flag on inter-area-attached | |
| `TestExtLinkMirrorsRouterLSALink` | `internal/plugins/ospf/ext_link_origin_test.go` | AC-3, A-2: Link Type/ID/Data equal the matching `RouterLink` fields | |
| `TestExtPrefixScopeSelection` | `internal/plugins/ospf/ext_prefix_origin_test.go` | R-4: Type 7 LS Type 10 for area-local prefixes, 11 only when scope demands; Type 8 always 10 | |
| `TestExtPrefixNFlagIgnoredNonHost` | `internal/plugins/ospf/ext_prefix_recv_test.go` | AC-5, R-5: N-Flag set on non-host prefix is ignored, not malformed | |
| `TestExtPrefixNFlagPreservedInterArea` | `internal/plugins/ospf/ext_prefix_recv_test.go` | AC-6, R-5: N-Flag preserved across ABR re-origination | |
| `TestExtPrefixLowestOpaqueIDWins` | `internal/plugins/ospf/ext_prefix_recv_test.go` | AC-9, R-8: same prefix across LSAs -> lowest Opaque ID used | |
| `TestExtPrefixDuplicateInLSAFirstWins` | `internal/plugins/ospf/ext_prefix_recv_test.go` | AC-10, R-8: duplicate in one LSA -> first instance, logged | |
| `TestExtPrefixType11UnreachableUnusable` | `internal/plugins/ospf/ext_prefix_recv_test.go` | AC-14, A-6: Type-11 with `reachable=false` -> present-but-unusable | |
| `TestRegisterPrefixSubTLVDispatched` / `TestRegisterLinkSubTLVDispatched` | `internal/plugins/ospf/ext_subtlv_test.go` | AC-11: registered sub-TLV dispatched; unknown skipped via Length | |
| `TestSubTLVRegistryGenericNoSRSpelling` | `internal/plugins/ospf/ext_subtlv_test.go` | R-7: this spec's package contains no SR/SID spelling | |
| `TestSubTLVCodecPanicIsolated` | `internal/plugins/ospf/ext_subtlv_test.go` | AC-16, R-9: a panicking sub-TLV codec is recovered, metric incremented | |
| `TestExtPrefixRegistersAsOpaqueConsumer` / `TestExtLinkRegistersAsOpaqueConsumer` | `internal/plugins/ospf/ext_register_test.go` | AC-1, A-1: registration with ext-1 at the correct scopes | |
| `TestExtPrefixWithdrawOnPrefixGone` | `internal/plugins/ospf/ext_prefix_origin_test.go` | AC-13, R-6: prefix disappears -> withdraw via `OnOriginate(withdraw=true)` | |
| `TestExtLinkReoriginatesOnTopologyChange` | `internal/plugins/ospf/ext_link_origin_test.go` | A-7: link change drives re-origination through `originateSelfLSAs` | |
| `TestExtPrefixEmptyContainerRoundTrip` | `internal/plugins/ospf/packet/ext_prefix_test.go` | AC-12, A-4: zero-sub-TLV container round-trips byte-for-byte | |
| `TestExtConfigEnableLeaf` | `internal/plugins/ospf/config_test.go` | A-8: `extended-prefix`/`extended-link` boolean leaves gate origination | |
| `TestExtDatabaseRenderDecoded` | `internal/plugins/ospf/show_database_test.go` | AC-15: `opaque-area`/`opaque-as` render decoded fields, not hex | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Opaque Type (this spec) | {7, 8} | 8 | N/A | a non-7/8 type is not this spec's |
| Extended Prefix TLV Route Type | 0,1,3,5,7 | 7 | N/A | unlisted route type: no correlation (skip per §2.1) |
| Prefix Length (AF=0) | 0-32 | 32 | N/A | >32 invalid for IPv4; reject |
| AF | 0 | 0 | N/A | non-0: prefix encoding out of scope (skip) |
| Flags (A/N bits) | 0x80, 0x40 | both | N/A | other bits unassigned, ignored |
| TLV / sub-TLV value length | 0-65531 | bounded by LSA Length | N/A | length past the LSA/TLV bound -> malformed (§5) |
| TLV padding | 0-3 bytes | 3 | N/A | N/A |
| Extended Link TLVs per LSA | exactly 1 (§3.1 SHALL) | 1 | 0 -> malformed | >1 -> use first, log |
| Opaque ID (per §2.1 tie-break) | 0-16777215 | 16777215 | N/A | masked to 24 bits by the carrier |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-ext-register` | `test/ospf/ospf-ext-register.ci` | Opaque Type 7/8 registered; `show ospf` reflects extended capability when enabled | |
| `ospf-ext-prefix-originate` | `test/ospf/ospf-ext-prefix-originate.ci` | enabling `extended-prefix` originates an Extended Prefix LSA for a connected prefix; visible in `opaque-area` | |
| `ospf-ext-link-originate` | `test/ospf/ospf-ext-link-originate.ci` | enabling `extended-link` originates exactly one Extended Link TLV per link; visible in `opaque-area` | |
| `ospf-ext-prefix-receive` | `test/ospf/ospf-ext-prefix-receive.ci` | a received Extended Prefix LSA is decoded, stored, and listed with Route Type/prefix/flags | |
| `ospf-ext-subtlv-hook` | `test/ospf/ospf-ext-subtlv-hook.ci` | a test sub-TLV codec registers, and a received sub-TLV of that type is dispatched; unknown skipped | |
| `ospf-ext-decode` | `test/ospf/ospf-ext-decode.ci` | `ze` decode of Extended Prefix/Link hex shows decoded fields + sub-TLVs | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-ext-prefix-link-frr` | `test/interop/scenarios/ospf-ext-prefix-link-frr/` | FRR `ospfd` with `segment-routing` / extended LSAs enabled | Ze originates Extended Prefix/Link Opaque LSAs (Opaque Type 7/8) that FRR accepts, floods, and correlates with the Router/Network LSAs; Ze decodes FRR's Extended Prefix/Link LSAs (including FRR's sub-TLVs, skipped as unknown without ext-5); empty-container LSAs interop; full adjacency unaffected | |

> Interop is required: this adds wire-visible LSAs (Opaque Type 7/8) and TLV
> bodies that FRR parses. The raw-IP / multicast paths are Linux-only and run as
> QEMU integration tests (`ai/rules/qemu-testing.md`), consistent with the rest of
> the OSPF interop set (`test/interop/scenarios/ospf-*-frr/`). FRR's `ospfd`
> originates Extended Prefix/Link LSAs when its Segment Routing / extended-LSA
> support is enabled; this scenario validates the carriage, not the SR sub-TLV
> semantics (those are validated in ext-5).

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above. SR sub-TLV *value* tests belong to ext-5.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/instance.go` -- invoke the Extended Prefix/Link `OnOriginate` computation from `originateSelfLSAs`; surface the decoded extended LSAs into the database snapshot
- `internal/plugins/ospf/show_database.go` -- register the Opaque Type 7/8 decoded renderer for the carrier's `opaque-area`/`opaque-as` views
- `internal/plugins/ospf/cmd_show.go` -- route the extended renderer output through `ApplyPipes` like the other show outputs
- `internal/plugins/ospf/register.go` -- ensure the two consumer `init()` registrations are discovered (the consumers live in their own files; no consumer name leaks into generic code)
- `internal/plugins/ospf/config.go` -- resolve the `extended-prefix` / `extended-link` enable leaves into the engine config
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `extended-prefix` / `extended-link` boolean enable leaves
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- `show ospf database opaque-area` / `opaque-as` already added by ext-1; this spec relies on them (no new command needed beyond ext-1's)
- `internal/plugins/ospf/packet/json.go` -- decode Opaque Type 7/8 bodies into the JSON opaque view (extended fields, not just hex)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `extended-prefix`/`extended-link` enable leaves; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | both are `type boolean` (native); no custom validator |
| YANG custom validators | [ ] no | native boolean suffices; Opaque ID is internal, not configured |
| CLI commands/flags | [ ] no new | reuses ext-1's `show ospf database opaque-area`/`opaque-as`; this spec adds only a decoded renderer |
| CLI grammar (action before identifier) | [ ] n/a | no new command |
| Editor autocomplete | [ ] yes | automatic for the two YANG booleans |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-ext-*.ci` |
| Pipe completeness | [ ] yes | the decoded `opaque-area`/`opaque-as` output routes through `ApplyPipes` |
| Env var registration | [ ] no | operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary/cert; reuses the existing OSPF raw socket and ext-1's carrier |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_ext_prefix_lsas` | gauge | `scope` (area/as) |
| `ze_ospf_ext_link_lsas` | gauge | (none; always area) |
| `ze_ospf_ext_originations_total` | counter | `opaque_type` (7/8) |
| `ze_ospf_ext_malformed_total` | counter | `opaque_type` (7/8) |
| `ze_ospf_ext_subtlv_errors_total` | counter | `registry` (prefix/link) |

> These extend the umbrella's canonical OSPF metric set and ext-1's
> `ze_ospf_opaque_*` series; they use the `ze_ospf_ext_*` prefix and are registered
> by this spec's owner code. The umbrella "Metrics" table gains these rows when
> this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPFv2 Extended Prefix/Link Opaque LSAs (RFC 7684) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `extended-prefix`/`extended-link` leaves |
| 3 | CLI command added/changed? | [ ] no new | `show ospf database opaque-*` documented by ext-1; note the decoded extended view |
| 4 | API/RPC added/changed? | [ ] no | show RPCs live in the central `ze-show` namespace |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains Opaque Type 7/8 consumers + a sub-TLV registry |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- Extended Prefix/Link section (carrier only) |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` (or equivalent) -- Extended Prefix/Link TLV layout |
| 8 | Plugin SDK/protocol changed? | [ ] yes | document `RegisterPrefixSubTLV`/`RegisterLinkSubTLV` for ext-5 authors |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc7684.md` -- flip the carriage compliance items to implemented (sub-TLV value items remain for ext-5) |
| 10 | Test infrastructure changed? | [ ] yes (interop scenario added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF RFC 7684 parity with FRR (containers) |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- the Extended Prefix/Link consumers + sub-TLV registry |
| 13 | Route metadata keys added/changed? | [ ] no | Extended LSAs install no route |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the five `ze_ospf_ext_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF files |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPF config/CLI examples against the new enable leaves |

## Files to Create
- `internal/plugins/ospf/packet/ext_prefix.go` -- Extended Prefix TLV + Extended Prefix Range TLV (container) codec over ext-1's `opaque_tlv.go`
- `internal/plugins/ospf/packet/ext_link.go` -- Extended Link TLV codec over ext-1's `opaque_tlv.go`
- `internal/plugins/ospf/ext_prefix.go` -- the Opaque Type 7 consumer: `init()` registration, `OnOriginate`/`OnReceive`, prefix-to-Route-Type association, dedup/N-Flag/A-Flag, withdraw
- `internal/plugins/ospf/ext_link.go` -- the Opaque Type 8 consumer: `init()` registration, `OnOriginate`/`OnReceive`, link-to-Router-LSA-link association, single-TLV enforcement
- `internal/plugins/ospf/ext_subtlv.go` -- the generic sub-TLV registry (`RegisterPrefixSubTLV`/`RegisterLinkSubTLV`), dispatch, and the panic-recover wrapper
- `internal/plugins/ospf/ext_render.go` -- the decoded renderer registered with the carrier for `opaque-area`/`opaque-as`
- `internal/plugins/ospf/packet/ext_prefix_test.go`, `internal/plugins/ospf/packet/ext_link_test.go`, `internal/plugins/ospf/packet/ext_tlv_test.go`
- `internal/plugins/ospf/ext_prefix_origin_test.go`, `internal/plugins/ospf/ext_link_origin_test.go`, `internal/plugins/ospf/ext_prefix_recv_test.go`, `internal/plugins/ospf/ext_subtlv_test.go`, `internal/plugins/ospf/ext_register_test.go`
- `test/ospf/ospf-ext-register.ci`, `ospf-ext-prefix-originate.ci`, `ospf-ext-link-originate.ci`, `ospf-ext-prefix-receive.ci`, `ospf-ext-subtlv-hook.ci`, `ospf-ext-decode.ci`
- `test/interop/scenarios/ospf-ext-prefix-link-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm ext-1's `RegisterOpaqueConsumer`/`OnOriginate`/`OnReceive`/`OriginateOpaque`/`opaque_tlv.go` exist |
| 3. Wiring phase | Wiring Test table -- the two consumer registrations + sub-TLV registry + failing wiring tests |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | from critical review |
| 9. Re-verify | re-run stage 6 |
| 10. Repeat 7-9 | until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | re-run stage 6 |
| 14. Present summary | Executive Summary per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register Opaque Type 7 + 8 and the sub-TLV registry; failing wiring tests
   - Tests: `TestExtPrefixRegistersAsOpaqueConsumer`, `TestExtLinkRegistersAsOpaqueConsumer`, `TestRegisterPrefixSubTLVDispatched`, `test/ospf/ospf-ext-register.ci`
   - Files: `ext_prefix.go` + `ext_link.go` (`init()` -> `RegisterOpaqueConsumer`, stub `OnOriginate`/`OnReceive`), `ext_subtlv.go` (registry), `register.go` discovery
   - Verify: both Opaque Types register with ext-1 at the right scopes; the sub-TLV registry stores a codec; origination/reception are stubs so deeper tests still fail
2. **Phase: Top-level TLV codecs** -- the three containers over ext-1's `opaque_tlv.go`
   - Tests: `TestExtPrefixTLVRoundTrip`, `TestExtPrefixAddressPrefixFixed32`, `TestExtPrefixRangeTLVContainerRoundTrip`, `TestExtLinkTLVRoundTrip`, `TestExtTLVAlignment`, `TestExtMalformedNotStored`, `TestExtLinkSingleTLVEnforced`, `TestExtPrefixEmptyContainerRoundTrip`
   - Files: `packet/ext_prefix.go`, `packet/ext_link.go`
   - Verify: fixed fields + 32-bit prefix + sub-TLV region round-trip; 4-byte alignment; §5 malformed rejected; one Extended Link TLV enforced
3. **Phase: Sub-TLV registry + dispatch** -- the SR attachment hook
   - Tests: `TestRegisterLinkSubTLVDispatched`, `TestSubTLVRegistryGenericNoSRSpelling`, `TestSubTLVCodecPanicIsolated`
   - Files: `ext_subtlv.go` (dispatch + recover wrapper)
   - Verify: known sub-TLV dispatched, unknown skipped via Length, a panicking codec isolated, no SR spelling
4. **Phase: Origination + association** -- prefix/link -> Extended TLV fields, driven by `originateSelfLSAs`
   - Tests: `TestExtPrefixRouteTypeMapping`, `TestExtLinkMirrorsRouterLSALink`, `TestExtPrefixScopeSelection`, `TestExtPrefixWithdrawOnPrefixGone`, `TestExtLinkReoriginatesOnTopologyChange`, `ospf-ext-prefix-originate.ci`, `ospf-ext-link-originate.ci`
   - Files: `ext_prefix.go`, `ext_link.go` (`OnOriginate` reading `RouteEntry`/`routerLinks`), `instance.go` (trigger)
   - Verify: correct Route Type / Link Type-ID-Data / scope; withdraw on disappearance; re-origination on change
5. **Phase: Reception decode + RFC rules** -- N-Flag, dedup, §5, Type-11 reachability
   - Tests: `TestExtPrefixNFlagIgnoredNonHost`, `TestExtPrefixNFlagPreservedInterArea`, `TestExtPrefixLowestOpaqueIDWins`, `TestExtPrefixDuplicateInLSAFirstWins`, `TestExtPrefixType11UnreachableUnusable`, `ospf-ext-prefix-receive.ci`
   - Files: `ext_prefix.go`, `ext_link.go` (`OnReceive`)
   - Verify: N-Flag ignore/preserve, lowest-Opaque-ID, first-in-LSA, present-but-unusable on unreachable Type-11
6. **Phase: CLI + config + metrics + JSON** -- user surface
   - Tests: `TestExtConfigEnableLeaf`, `TestExtDatabaseRenderDecoded`, `ospf-ext-decode.ci`, `ospf-ext-subtlv-hook.ci`
   - Files: `ext_render.go`, `show_database.go`, `cmd_show.go`, `packet/json.go`, `yang/ze-ospf-conf.yang`, `config.go`, metric registration
   - Verify: decoded `opaque-area`/`opaque-as` rendering, the two enable leaves, the five metric series, JSON decode
7. **Functional tests** -> the six `.ci` cover the user-visible behaviour
8. **RFC refs** -> add `// RFC 7684 Section X.Y` comments on the TLV fixed fields, §5 malformed guard, N-Flag rules, §3.1 single-TLV, dedup; `// RFC 5250 Section 5` on the Type-11 reachability use
9. **Interop** -> `ospf-ext-prefix-link-frr` QEMU scenario
10. **Full verification** -> `make ze-verify`
11. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; carriage parity with FRR's RFC 7684 containers (Opaque Type 7/8, Extended Prefix/Link TLVs, sub-TLV walk), SR sub-TLV values excluded by design (ext-5) |
| Correctness | TLV 4-byte alignment; §5 malformed rejection; 32-bit Address Prefix for AF=0; Route Type / Link Type-ID-Data association exact; one Extended Link TLV per LSA; N-Flag ignore/preserve; lowest-Opaque-ID dedup; Type 7 scope 10/11, Type 8 scope 10 |
| Naming | `ze_ospf_ext_*` metrics; YANG `extended-prefix`/`extended-link` kebab-case; `RegisterPrefixSubTLV`/`RegisterLinkSubTLV` |
| Data flow | Extended LSAs flow only through ext-1's carrier (`OnOriginate`/`OnReceive`/`OriginateOpaque`); SPF/route read-only for association; no carrier or SPF change |
| CLI grammar | no new command (reuses ext-1's `show ospf database opaque-*`) |
| Doctor checks | none added (no new runtime dependency) -- confirm |
| YANG validation | the two enable leaves are native booleans |
| Prometheus counters | the five `ze_ospf_ext_*` series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | no SR/SID spelling in this spec; removing this spec removes Opaque Type 7/8 cleanly; ext-1 still re-floods Type 7/8 verbatim |
| Rule: buffer-first | TLV codecs write into caller buffers via ext-1's builder; decode zero-copy |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Opaque Type 7 + 8 registered with ext-1 | `grep -rn 'RegisterOpaqueConsumer' internal/plugins/ospf/ext_prefix.go internal/plugins/ospf/ext_link.go` |
| Sub-TLV registry hook present and generic | `grep -rn 'RegisterPrefixSubTLV\|RegisterLinkSubTLV' internal/plugins/ospf` and grep shows no SR/SID spelling in this spec's files |
| Top-level TLV codecs | `go test ./internal/plugins/ospf/packet -run 'Ext'` |
| Association (Route Type / Link) | `go test ./internal/plugins/ospf -run 'TestExtPrefixRouteTypeMapping|TestExtLinkMirrorsRouterLSALink'` |
| §5 malformed + alignment | `go test ./internal/plugins/ospf/packet -run 'TestExtMalformedNotStored|TestExtTLVAlignment'` |
| Five metric series registered | `grep -rn 'ze_ospf_ext_' internal/plugins/ospf` |
| Interop scenario present | `ls test/interop/scenarios/ospf-ext-prefix-link-frr/` |
| Functional tests present | `ls test/ospf/ospf-ext-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | every TLV/sub-TLV walk is bound-checked against the subsuming LSA/TLV; overrun/short-trailing -> malformed (§5), never slice-out-of-range; extend the `packet` fuzz target with Extended Prefix/Link bodies |
| Resource exhaustion | a flood of Extended Prefix/Link LSAs is bounded by ext-1's opaque store caps; sub-TLV nesting depth bounded by the enclosing TLV extent (no unbounded recursion) |
| Codec isolation | a registered sub-TLV codec's panic is recovered, counted, and cannot crash OSPF or wedge the LSDB lock |
| Trust boundary | Extended LSAs ride ext-1's flooding (opaque-capable neighbours, existing OSPF auth); no new auth surface |
| Error leakage | malformed-LSA and sub-TLV-codec errors are counted/logged, not surfaced to peers; a malformed LSA is dropped, not partially applied |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
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
<!-- LIVE -->

## Core Insight
RFC 7684 is a pure *carriage* layer: two opaque LSAs whose bodies are three
top-level TLVs and a sub-TLV slot. Ze already has every piece below it (ext-1's
opaque carrier, the generic TLV iterator/builder, the Router-LSA link encoding,
the route table's Route Type). The work is association (mapping advertised
prefixes/links onto Extended TLV fixed fields) and a sub-TLV registry that lets
SR (ext-5) plug in one level down, exactly mirroring how ext-1's
`RegisterOpaqueConsumer` lets this spec plug in. The containers ship empty and
are fully RFC-7684-conformant on their own, because RFC 7684 defines no sub-TLV
values (those are RFC 8665).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build empty containers + a sub-TLV registry now, defer all SID semantics to ext-5 | implement SR sub-TLVs here | RFC 7684 defines only the containers; the registries are seeded with Reserved/type-1 only; SID values are RFC 8665 (ext-5); self-containment keeps SR out of the carrier |
| Generic sub-TLV registry (`RegisterPrefixSubTLV`/`RegisterLinkSubTLV`) | hard-code SR sub-TLV awareness | mirrors ext-1's `RegisterOpaqueConsumer`; plugin-self-containment; ext-5 registers from its own `init()` |
| Associate via the existing `RouteEntry`/`RouteType` + `routerLinks`, read-only | recompute prefix/link sets independently | the Route Type and Link Type/ID/Data must match the Router/Network LSAs FRR sees; reusing the authoritative source guarantees correlation |
| Type 8 always area scope (10); Type 7 area (10) or AS (11) by prefix scope | always area, or a config knob | RFC 7684 §3 fixes Extended Link to area; §2.1 requires the Extended Prefix LSA scope to satisfy every prefix it carries |
| Carry the Extended Prefix Range TLV as a fixed-field + sub-TLV container with no range semantics | omit it, or invent range fields | RFC 7684 explicitly does NOT define this TLV (RFC 8665 §4 does); a container-only carrier lets ext-5 add range semantics without a re-decode |

## Known Limitations
- Ships empty containers: without ext-5 the Extended Prefix/Link LSAs carry no sub-TLVs (by design -- RFC 7684 defines no sub-TLV values).
- Extended Prefix/Link LSAs install no route and never enter SPF (RFC 5250 §3); attributes are stored for consumer use only.
- The Extended Prefix Range TLV has fixed-field + sub-TLV carriage only; its range semantics are ext-5 (RFC 8665 §4).
- OSPFv3 is unaffected: RFC 8362 uses TLV-based extended LSAs, not opaque LSAs; `internal/plugins/ospf/v3` is untouched.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code:
- RFC 7684 §2 generic TLV format (Length excludes pad; 4-byte alignment)
- RFC 7684 §2.1 Extended Prefix TLV fixed fields, AF=0 32-bit prefix, A/N flags
- RFC 7684 §2.1 N-Flag ignored on non-host prefix; preserved across areas
- RFC 7684 §3.1 exactly one Extended Link TLV per LSA (SHALL)
- RFC 7684 §5 malformed LSA MUST NOT be stored/acked/reflooded
- RFC 7684 §2/§3 lowest-Opaque-ID and first-instance dedup
- RFC 5250 §5 Type-11 originator reachability (consumed from ext-1)

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
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Originate/flood/store Extended Prefix (Type 7) + Extended Link (Type 8) Opaque LSAs | functional + interop | `ospf-ext-prefix-originate.ci`, `ospf-ext-link-originate.ci`, `ospf-ext-prefix-link-frr` |
| Three top-level TLV codecs with sub-TLV carriage | unit | `TestExtPrefixTLVRoundTrip`, `TestExtLinkTLVRoundTrip`, `TestExtPrefixRangeTLVContainerRoundTrip` |
| Sub-TLV registration hook (SR attachment point) | unit + functional | `TestRegisterPrefixSubTLVDispatched`, `ospf-ext-subtlv-hook.ci` |
| Prefix/link <-> originating Router/Network LSA association | unit + interop | `TestExtPrefixRouteTypeMapping`, `TestExtLinkMirrorsRouterLSALink`, `ospf-ext-prefix-link-frr` |
| §5 malformed safety + alignment | unit + fuzz | `TestExtMalformedNotStored`, `TestExtTLVAlignment`, extended `packet` fuzz target |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE |  | file:line |  |

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

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
- [ ] AC-1..AC-17 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC 7684 + RFC 5250 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the sub-TLV registry justified by ext-5 + future prefix/link attribute apps)
- [ ] No speculative features (only the containers + hook; no SID values)
- [ ] Single responsibility per component (codecs in packet, consumers in plugin, registry generic)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (no SR spelling; ext-1 names no Type 7/8)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-ext-prefix-link-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-4-extended-link-prefix.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-4-extended-link-prefix.md`
