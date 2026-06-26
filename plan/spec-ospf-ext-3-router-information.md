# Spec: ospf-ext-3 -- OSPFv2/OSPFv3 Router Information LSA (RFC 7770)

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
3. `rfc/short/rfc7770.md` -- the RI LSA: OSPFv2 Opaque type 4 (§2.1), OSPFv3 function code 12 (§2.2), RI TLV format (§2.3), Informational Capabilities TLV type 1 (§2.4), capability bits (§2.5), Functional Capabilities TLV type 2 (§2.6), multi-instance / smallest-Instance-ID rule (§3), flooding scope (§2.7)
4. `rfc/short/rfc5250.md` -- the OSPFv2 Opaque carrier: LS-ID split Opaque Type (8) + Opaque ID (24) (§3, App A.2), scope flooding (§3.1), O-bit (§3.1), §5 Type-11 reachability -- RI rides this carrier (ext-1)
5. `plan/spec-ospf-ext-1-opaque-framework.md` -- the dependency: `RegisterOpaqueConsumer(opaqueType, scope, OnOriginate, OnReceive)`, the generic 4-byte-aligned TLV iterator/builder (`packet/opaque_tlv.go`), the consumer-callback recover wrapper, the §5 reachability gate -- RI is the FIRST consumer of this carrier
6. `internal/plugins/ospf/lsdb/origination.go` -- `OriginateSelf(area, key, body, enc)` + `SelfLSAEncoder`; the address-family-neutral self-LSA origination seam RI uses for OSPFv3 (and that ext-1's `OriginateOpaque` wraps for OSPFv2)
7. `internal/plugins/ospf/instance.go` (881 `originateSelfLSAs`) -- the per-topology-change + periodic origination trigger; branches `v6OriginateSelf` (OSPFv3) vs `lsdb.OriginateFromTopology` (OSPFv2); RI origination hangs off here
8. `internal/plugins/ospf/afstrategy_v6.go` / `origination_v6.go` (`v6OriginateSelf`, `v6OriginateRouter`, `v6OriginHeader`) -- the OSPFv3 self-LSA builder RI extends with function code 12
9. `internal/plugins/ospf/v3/types/lsa.go` -- the OSPFv3 16-bit LS Type (U | S2 | S1 | 13-bit function code), `Scope()`, base function-code constants; RI adds `LSTypeRouterInformation` (function code 12) here
10. `internal/plugins/ospf/show_database.go` / `cmd_show.go` -- the `show ospf database <type>` subview map RI extends with `router-information`

## Task

Originate, flood, and store the **Router Information (RI) LSA** (RFC 7770, which
obsoletes RFC 4970) in both OSPFv2 and OSPFv3, advertising this router's optional
capabilities to the OSPF domain. The RI LSA is the standards-track mechanism by
which an OSPF router announces informational and functional capabilities (and,
later, Segment Routing parameters) without changing the base topology calculation.

For **OSPFv2** the RI LSA is an **Opaque LSA with Opaque type 4** (RFC 7770 §2.1):
this spec is a **consumer of the ext-1 opaque framework**. It registers Opaque
type 4 via `RegisterOpaqueConsumer`, supplies an `OnOriginate` callback that
builds the RI TLV body, and lets ext-1 own the LS-ID split (Opaque type 4 in the
high byte, the RI LSA Instance ID in the low 24 bits), the scope-correct flooding
(LS type 9/10/11), the O-bit gate, sequencing, install, flood, withdraw, and the
§5 Type-11 reachability gate. RI adds no flooding, no LSDB, no codec-scope work
for OSPFv2: it only supplies the body.

For **OSPFv3** the RI LSA is a **native LSA with function code 12** (RFC 7770 §2.2),
NOT an opaque LSA (RFC 5340 carries extensions as native LSAs). This spec adds the
`LSTypeRouterInformation` function-code constant to the OSPFv3 type layer (with the
U-bit set and the S2/S1 scope bits selecting link/area/AS), builds the RI LSA body,
and originates it through the existing `v6OriginateSelf` -> `OriginateSelf` self-LSA
seam exactly as the OSPFv3 Router-LSA and Intra-Area-Prefix-LSA already do.

The shared deliverable across both address families is a single **RI TLV codec**
plus a **TLV-registration hook**: this spec carries the **Router Informational
Capabilities TLV (type 1)** (and the Functional Capabilities TLV type 2 as an
empty-by-default carrier), advertises the capability bits this router actually
implements (per §2.5: graceful-restart-capable, GR-helper, stub-router, TE), and
exposes a `RegisterRITLV(tlvType, scope, BuildFn)` registration point so a
downstream consumer (Segment Routing / ext-5) can inject SR-Algorithm / SRGB /
SRLB TLVs into the SAME RI LSA without this spec naming Segment Routing.

The RI LSA Instance ID 0 is the canonical instance and MUST carry the
Informational Capabilities TLV as the first TLV (§2.4); overflow TLVs that no
longer fit go into subsequent Instance IDs (this spec emits a single instance 0
unless a registered TLV builder overflows it). Flooding scope is operator-selectable
per RFC 7770 §2.7 (link / area / AS); the default is **area + AS** (an area-scoped
RI LSA into every attached area plus one AS-scoped RI LSA), matching the common
FRR/SR deployment where SR consumers need both.

### In scope (this spec)

| Item | Detail |
|------|--------|
| OSPFv2 RI carriage | Register Opaque type 4 with ext-1's `RegisterOpaqueConsumer`; `OnOriginate` returns the RI TLV body + Instance ID + scope; ext-1 owns LS-ID split, scope flooding, O-bit, sequencing, install, flood, withdraw, §5 gate |
| OSPFv3 RI carriage | New `LSTypeRouterInformation` (function code 12, U=1, S2/S1 = scope) in `v3/types/lsa.go`; build + originate via `v6OriginateSelf` -> `OriginateSelf` per scope |
| RI TLV codec | Shared encode/decode of the RI TLV stream (RFC 7770 §2.3 format = RFC 3630 TLV: 2-byte type, 2-byte length-excluding-pad, value, 4-byte-aligned pad); reuses ext-1's generic `packet/opaque_tlv.go` iterator/builder for OSPFv2 and a parallel call for OSPFv3 |
| Informational Capabilities TLV (type 1) | Emit TLV type 1 (length multiple of 4, initially 4 octets / 32 bits); set the bits this router implements (§2.5 bit 0 GR-capable, bit 1 GR-helper, bit 2 stub-router, bit 3 TE); MUST be the FIRST TLV in Instance 0 (§2.4) |
| Functional Capabilities TLV (type 2) | Carrier present but empty by default (RFC 7770 §5.5 registry is initially empty); absence-means-not-supported semantics documented (§2.6); a registered TLV builder may populate it |
| TLV-registration hook | `RegisterRITLV(tlvType, scope, BuildFn)` so SR/ext-5 (and other future consumers) append TLVs into the RI LSA; this spec invokes registered builders in TLV-type order after the type-1 TLV; the hook names no consumer |
| Configurable advertisement scope | A YANG `router-information` container under `ospf` with a `scope` leaf (enumeration `link`/`area`/`as`, multiple allowed; default `area` + `as`) and an `enabled` leaf; resolved into the engine config and applied to both address families |
| Multi-instance rule | Single Instance 0 by default; when registered TLV builders overflow the maximum LSA length, overflow goes into Instance 1, 2, ...; on receipt, for an unspecified-multi-instance TLV use the numerically smallest Instance ID and ignore the rest (§3) |
| CLI visibility | `show ospf database router-information` (OSPFv2 opaque + OSPFv3 native) decodes and renders the capability bits and TLV list |
| Capability-bit accuracy | The advertised informational bits MUST reflect this router's actual capabilities in the advertised scope (§2.4); the bits are derived from live engine state (GR helper enabled, stub-router/max-metric configured, etc.), not hard-coded |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| Opaque carrier (LS-ID split, scope flooding, O-bit, §5 reachability, generic TLV helpers) | spec-ospf-ext-1 (the dependency) -- this spec consumes it, does not reimplement it |
| SR-Algorithm / SRGB / SRLB TLVs and any SR semantics | spec-ospf-ext-5 (Segment Routing) -- this spec provides only the `RegisterRITLV` hook; it defines NO SR TLV |
| Acting on a RECEIVED peer's capability bits to change protocol behavior | none in this spec -- informational bits are advertised and rendered only; only Functional bits may drive behavior (§2.6) and the functional registry is empty |
| TE LSA body / TE capability semantics beyond setting bit 3 when TE is configured | spec-ospf-ext-2 (TE) -- this spec sets the informational TE bit but originates no TE LSA |
| Grace-LSA / GR helper machinery beyond setting bits 0/1 when GR is configured | spec-ospf-ext-9 (Grace-LSA) -- this spec only reflects GR capability into the bits |
| Defining new RI TLV types or new capability bits | none -- this spec emits only the assigned type-1/type-2 TLVs and §2.5 bits 0-3; the hook accepts other types but defines none |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/research/ospf-implementation-guide.md` §14 "Router Information (RFC 7770)" (~1526-1529) -- "Advertises router capabilities via an opaque LSA. Feeds into Segment Routing and PCE discovery."
  -> Decision: model RI as a thin consumer that fills a TLV body and registers it; the carriage (opaque for v2, native for v3) and the SR consumption are separate layers -- RI owns only the Informational Capabilities TLV plus a hook for SR
  -> Constraint: RI does NOT feed SPF or the route table; it is informational; only the (empty) Functional Capabilities TLV may ever drive protocol operation, and this spec drives none
- [ ] `plan/spec-ospf-ext-1-opaque-framework.md` "In scope" + "Data Flow" -- the `RegisterOpaqueConsumer` API, the `OnOriginate (opaqueID, scope, body, withdraw)` contract, the generic TLV iterator/builder in `packet/opaque_tlv.go`, and the §5 Type-11 gate
  -> Constraint: for OSPFv2 the RI LSA Instance ID maps to ext-1's 24-bit Opaque ID; this spec passes Instance ID as the Opaque ID and the RI body as the opaque body; it never touches LS-ID encoding, the O-bit, or flooding directly
  -> Decision: reuse ext-1's `packet/opaque_tlv.go` builder/iterator for the RI TLV stream for OSPFv2, and call the SAME helpers (or a thin shared `packet/ri_tlv.go` over them) for OSPFv3 so the TLV format is encoded once
- [ ] `ai/rules/plugin-self-containment.md` -- consumers must be self-contained; no consumer name leaks into shared code
  -> Constraint: `RegisterRITLV` names no consumer; the SR-Algorithm/SRGB/SRLB TLV types live entirely in ext-5; removing ext-5 removes its TLVs and leaves the RI LSA with only the type-1 TLV; RI itself names no Segment Routing symbol
- [ ] `ai/rules/buffer-first.md` -- TLV emit and RI body build are buffer-first
  -> Constraint: the RI body is built into a caller-owned buffer via `WriteTo(buf, off) int` (the ext-1 TLV builder); the 4-byte pad is written, never produced by slice concatenation; the capability bitfield is written in place; no `append`-grown body on the origination path
- [ ] `ai/rules/no-sprintf-alloc.md` -- no `fmt`/`+` on the wire or render path
  -> Constraint: `show ospf database router-information` renders capability bits and TLVs through `textbuf`/`AppendTo`, not `fmt.Sprintf`
- [ ] `ai/rules/config-surface.md` + `ai/rules/config-naming.md` -- YANG vs env var, kebab-case
  -> Constraint: the advertisement scope and enable flag are operational config (YANG `router-information` container, kebab-case leaves), not environment vars

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7770.md` -- the RI LSA definition
  -> Constraint: §2.1 OSPFv2 RI LSA = Opaque type 4; LS type 9/10/11 selects flooding scope; Opaque ID = the RI LSA Instance ID (first instance = 0)
  -> Constraint: §2.2 OSPFv3 RI LSA = function code 12 with U-bit set; wire LS Type differs per scope (0x000C link, 0x200C area, 0x400C AS), NOT a single constant; Link State ID = the Instance ID
  -> Constraint: §2.3 TLV format = 2-byte Type, 2-byte Length (value octets only, padding NOT counted), value padded to a 4-octet boundary with undefined bits; `total = 4 + roundup4(Length)`; unrecognized TLV types are ignored
  -> Constraint: §2.4 the Informational Capabilities TLV is type 1, length a multiple of 4 (initially 4 octets), bits numbered left-to-right with the MSB = bit 0; if included it MUST be the FIRST TLV in Instance 0 and MUST accurately reflect this router's capabilities in the advertised scope
  -> Constraint: §2.5 informational bits: 0 GR-capable, 1 GR-helper, 2 stub-router (RFC 6987), 3 TE (RFC 3630), 4 P2P-over-LAN (RFC 5309), 5 experimental-TE; this spec sets 0-3 from live engine state, leaves 4-5 clear
  -> Constraint: §2.6 the Functional Capabilities TLV is type 2; absence, or a Length too short to include a bit, implicitly means "not supported" (do NOT treat absence as unknown); the functional registry (§5.5) is initially empty so this TLV is carried empty
  -> Constraint: §3 for a TLV with unspecified multi-instance handling, use the RI LSA with the numerically smallest Instance ID and ignore subsequent instances; previously advertised TLVs SHOULD stay in Instance 0
  -> Constraint: §2.7 RI TLV flooding-scope rules are per-TLV; the operator selects link/area/AS; if AS-wide scope is chosen the router SHOULD also advertise area-scoped RI LSA(s) into any attached NSSA area(s)
- [ ] `rfc/short/rfc5250.md` -- the OSPFv2 Opaque carrier RI rides on (via ext-1)
  -> Constraint: §3 / App A.2 the 32-bit LS ID splits into Opaque Type (8 bits, =4 for RI) + Opaque ID (24 bits, = RI Instance ID); ext-1 owns this split
  -> Constraint: §3.1 opaque LSAs flood only to O-bit neighbours and honour Type-9/10/11 scope; §5 a Type-11 RI LSA from an unreachable originator is not used -- all owned by ext-1, inherited by RI for free

**Key insights:** (minimal context to resume after compaction)
- RI has TWO carriages: OSPFv2 = ext-1 opaque consumer (Opaque type 4); OSPFv3 = native function-code-12 LSA. The TLV body is identical; only the LSA header/carriage differs.
- This spec writes NO flooding, NO LSDB scope, NO O-bit, NO LS-ID-split code -- ext-1 owns all of that for OSPFv2, and the existing `OriginateSelf` self-LSA seam owns it for OSPFv3.
- The deliverable is: (1) a shared RI TLV body codec, (2) the type-1 Informational Capabilities TLV filled from live capability state, (3) a `RegisterRITLV` hook so SR (ext-5) can add TLVs, (4) per-scope origination wiring, (5) config (scope/enable), (6) a `show ... database router-information` view.
- Default scope is area + AS. Instance 0 carries the type-1 TLV first (§2.4). Functional TLV (type 2) is an empty carrier.
- Capability bits are derived from real engine state (GR helper on, stub-router/max-metric configured, TE configured), never hard-coded, so the advertisement is accurate (§2.4 MUST).

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/plugins/ospf/instance.go` (881 `originateSelfLSAs`) -- the origination trigger fired on topology change, neighbour change, and the periodic tick; it branches `e.v6OriginateSelf(...)` for OSPFv3 (line 892) and `e.lsdb.OriginateFromTopology(...)` for OSPFv2 (line 895)
  -> Constraint: RI origination hangs off `originateSelfLSAs` in BOTH branches: after the v2/v3 self-LSAs are regenerated, invoke the RI originator so an RI LSA is (re-)originated on the same triggers; no new timer, no new trigger
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `OriginateSelf(area, key, body, enc SelfLSAEncoder) (LSAHeader, bool)` (245) runs sequencing + MinLSInterval + install + flood for an address-family-neutral self-LSA; `SelfLSAEncoder` (235) is `func(seq, purge) packet.LSA`; `FlushStaleSelfLSAs` (273) withdraws stale self-LSAs by managed type
  -> Constraint: OSPFv3 RI uses `OriginateSelf` directly (area/AS scope), exactly as `v6OriginateRouter`/`v6OriginateIntraAreaPrefix` do; OSPFv2 RI uses ext-1's `OriginateOpaque` (which itself wraps `OriginateSelf`/`OriginateLinkSelf`); RI adds NO sequencing/flood code
  -> Constraint: withdraw (RI disabled, or scope removed) reuses `FlushStaleSelfLSAs` (OSPFv3) / ext-1 `OnOriginate(withdraw=true)` (OSPFv2); both MaxAge-flush through the existing purge path
- [ ] `internal/plugins/ospf/afstrategy_v6.go` / `origination_v6.go` -- `v6OriginateSelf(router, maxMetric)` (70) regenerates every OSPFv3 self-LSA per active area; `v6OriginateRouter` (158) and `v6OriginateIntraAreaPrefix` (179) each build a body, compute a key, and call `e.lsdb.OriginateSelf(area, key, bodyBytes, enc)`; `v6OriginHeader(t, lsid, router, seq, purge)` (428) builds the OSPFv3 LSA header; `v6ManagedSelfTypes` (36) lists the managed self-LSA types for stale flush
  -> Constraint: the OSPFv3 RI originator is a sibling of `v6OriginateRouter` -- build the RI body, compute the key `(LSTypeRouterInformation, lsid=InstanceID, router)`, call `OriginateSelf` per scope, and add `LSTypeRouterInformation` to `v6ManagedSelfTypes` so withdrawal is handled by the existing stale flush
- [ ] `internal/plugins/ospf/v3/types/lsa.go` -- OSPFv3 `LSType uint16` = `U(1) | S2 | S1 | function(13)`; `Scope()` (46) reads S2/S1; base constants (24-32) like `LSTypeRouter = 0x2001`, `LSTypeASExternal = 0x4005`; `Known()` (49) switches the base types
  -> Constraint: add `LSTypeRouterInformation` here. It is NOT a single constant: per RFC 7770 the wire type is `0x000C` link, `0x200C` area, `0x400C` AS (U=1, function=12). Provide a scope->LSType helper (and add the three values to `Known()`), mirroring how the scope bits already work for the base types
- [ ] `internal/plugins/ospf/lsdb/lsdb.go` -- `LSAKey` triple `(Type, LinkStateID, AdvertisingRouter)`; `dbForLocked`/`dbForReadLocked` route AS-external to `asExternal`, else the per-area store; `OriginateSelf` installs through this
  -> Constraint: for OSPFv3 AS-scoped RI, the store routing must send function-code-12 AS-scope LSAs to the AS-wide store. ext-1 already adds AS-wide opaque routing for OSPFv2; the OSPFv3 AS-scope routing follows the existing `LSTypeASExternal` (0x4005) AS-store precedent via the S2/S1 scope bits -- confirm the OSPFv3 LSDB routes AS-scope by `Scope()`, not by the opaque-type-11 path
- [ ] `internal/plugins/ospf/show_database.go` -- `dbSubviewType` (10) maps `show ospf database <type>` strings to snapshot types (router/network/summary/asbr-summary/external/nssa); `databaseSnapshotByType` (23) filters the LSDB snapshot to one type
  -> Constraint: add a `router-information` subview. For OSPFv2 it filters the opaque store to Opaque type 4; for OSPFv3 it filters to function code 12. The renderer decodes the RI TLV stream and lists the capability bits
- [ ] `internal/plugins/ospf/cmd_show.go` -- `init()` (39) registers the `ze-show:ospf-database-*` RPCs via `dbSubviewForwarder("show ospf database <type>")`; the database subviews are registered there
  -> Constraint: register `ze-show:ospf-database-router-information` -> `dbSubviewForwarder("show ospf database router-information")`, mirroring the existing subview registrations; the command YANG binding lives in `yang/ze-ospf-cmd.yang`
- [ ] `internal/plugins/ospf/config.go` -- `parseOSPFConfig` (268) / `applyTree` (300) resolve the `ospf` YANG tree into `ospfConfig`; existing containers like `max-metric` (`parseMaxMetric` 505) and `default-information` (`parseDefaultInformation` 492) are the pattern for a new container
  -> Constraint: add a `router-information` parse (enable + scope list) following the `parseMaxMetric` pattern; the resolved scope/enable flags feed the RI originator; the capability-bit derivation reads existing config (`MaxMetric`, GR helper, etc.), not new leaves
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- top-level `container ospf` (12) holds `default-information`, `timers`, `max-metric`, `redistribute`, `areas`, `interfaces`; native-typed leaves with `range`/`enumeration` constraints throughout
  -> Constraint: add `container router-information` as a sibling of `max-metric`: `leaf enabled` (boolean), `leaf-list scope` (enumeration link/area/as, default area+as); fully native-typed, no custom validator needed
- [ ] `internal/plugins/ospf/register.go` -- `registerOSPF()` (76) registers the namespace + the plugin `registry.Registration`; `init()` (130) calls it; this is where an in-process RI-TLV registry would be created and discovered
  -> Constraint: the `RegisterRITLV` registry is a new in-process registry inside the OSPF plugin (like ext-1's opaque registry); consumers (ext-5) call `RegisterRITLV` from their own `init()`; RI discovers registered builders at origination time

**Behavior to preserve:**
- ext-1's `RegisterOpaqueConsumer` / `OnOriginate` / `OnReceive` contract, the LS-ID split, scope flooding, the O-bit gate, the §5 Type-11 reachability gate, and the generic `packet/opaque_tlv.go` helpers -- RI consumes them unchanged.
- The OSPFv3 self-LSA origination seam: `v6OriginateSelf` -> per-area body build -> `OriginateSelf`; `v6ManagedSelfTypes` stale flush; `v6OriginHeader`. RI is one more managed self-LSA type flowing through it.
- The `LSAKey` triple and the OSPFv2/OSPFv3 LSDB store routing; RI adds no key type and no store.
- All existing OSPFv2/OSPFv3 functional and interop tests: a router with RI disabled behaves exactly as today.

**Behavior to change:** (all RFC-7770-required, not discretionary)
- `originateSelfLSAs` additionally (re-)originates the RI LSA in both address families when RI is enabled.
- `v3/types/lsa.go` gains `LSTypeRouterInformation` (function code 12, scope-dependent wire value) and includes it in `Known()`.
- `v6ManagedSelfTypes` includes the RI type(s) for stale flush.
- `dbSubviewType` + `cmd_show.go` gain the `router-information` subview/RPC.
- `config.go` + `ze-ospf-conf.yang` gain the `router-information` container.
- A new `RegisterRITLV` in-process registry is created and discovered at origination time.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Origination:** `originateSelfLSAs` fires (topology/neighbour change or periodic tick) -> after the base self-LSAs are regenerated, the RI originator runs for each configured scope.
- **Config:** the `router-information` container (`enabled`, `scope`) resolves into `ospfConfig`; a change re-evaluates which RI LSAs to originate or withdraw.
- **Reception (rendering only):** an LS Update carrying an RI LSA enters the existing path -- OSPFv2 via ext-1's opaque receive (Opaque type 4 delivered to RI's `OnReceive`), OSPFv3 via the native LSDB install; the body is stored and rendered by `show ... database router-information`; no protocol behavior is driven by received informational bits.
- **TLV registration:** a consumer (ext-5) calls `RegisterRITLV(tlvType, scope, BuildFn)` from its `init()`; the builder is invoked during RI origination.

### Transformation Path
1. **Capability snapshot (new):** the RI originator reads live engine state and builds the §2.5 informational bitfield -- bit 0 (GR-capable) and bit 1 (GR-helper) from the GR config/helper state, bit 2 (stub-router) when max-metric/stub-router is configured, bit 3 (TE) when TE is configured; bits 4-5 clear. The functional bitfield (§2.6) is empty.
2. **RI body build (new, shared):** the RI TLV builder emits, in order: the type-1 Informational Capabilities TLV (first, §2.4) with the bitfield (length 4, padded to multiple of 4); optionally a type-2 Functional Capabilities TLV (empty carrier); then each `RegisterRITLV` builder's TLVs in ascending TLV-type order. Each TLV is 4-byte aligned via the ext-1 builder. The body is built buffer-first into a caller-owned buffer.
3a. **OSPFv2 carriage (consumer of ext-1):** the RI `OnOriginate` callback returns `(opaqueID = InstanceID, scope, body, withdraw)`; ext-1 builds the Opaque LSA (LS type 9/10/11 from scope, LS ID = `4<<24 | InstanceID`), assigns a sequence, installs, floods to O-bit neighbours, and (for Type 11) gates on §5 reachability. RI writes none of this.
3b. **OSPFv3 carriage (native self-LSA):** the OSPFv3 RI originator computes the key `(LSTypeRouterInformation-for-scope, lsid=InstanceID, router)`, builds the LSA via `v6OriginHeader` + the RI body, and calls `e.lsdb.OriginateSelf(area, key, body, enc)` (area/AS scope) -- the same seam `v6OriginateRouter` uses. The U-bit is set so non-supporting routers still flood it.
4. **Store + flood (existing):** OSPFv2 via ext-1's opaque stores/flooding; OSPFv3 via the existing per-area / AS-wide LSDB and §13 flooding selected by the `Scope()` bits. No new store, no new flooding.
5. **Withdraw (existing):** RI disabled or a scope removed -> OSPFv2 `OnOriginate(withdraw=true)` (ext-1 MaxAge purge); OSPFv3 `FlushStaleSelfLSAs` over `v6ManagedSelfTypes` (existing MaxAge purge). Peers withdraw the RI LSA.
6. **Multi-instance (new, rare):** if the registered builders overflow the maximum LSA length for Instance 0, the originator emits Instance 1, 2, ... carrying the overflow; Instance 0 always carries the type-1 TLV first (§2.4).
7. **Render (new):** `show ospf database router-information` reads the RI LSA bytes (OSPFv2 opaque store filtered to Opaque type 4; OSPFv3 LSDB filtered to function code 12), iterates the TLV stream via the ext-1 iterator, and renders the capability bits + TLV list via `textbuf`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RI body <-> OSPFv2 opaque carriage | ext-1 `RegisterOpaqueConsumer` + `OnOriginate (opaqueID, scope, body, withdraw)`; value-typed payload (no cross-boundary pointers) | [ ] |
| RI body <-> OSPFv3 native carriage | `OriginateSelf(area, key, body, enc)` with `LSTypeRouterInformation`; same seam as `v6OriginateRouter` | [ ] |
| RI originator <-> capability state | read-only snapshot of GR/stub-router/TE config + helper state into the §2.5 bitfield | [ ] |
| RI originator <-> TLV consumers | `RegisterRITLV(tlvType, scope, BuildFn)`; builders invoked in TLV-type order; recover-wrapped | [ ] |
| RI TLV stream <-> generic TLV codec | ext-1 `packet/opaque_tlv.go` builder/iterator (shared); a thin `packet/ri_tlv.go` for the type-1 bitfield helpers | [ ] |
| RI LSA bytes <-> CLI render | `show ospf database router-information` decodes via the iterator; `textbuf` render | [ ] |
| config <-> RI scope/enable | `router-information` YANG container -> `ospfConfig` -> originator | [ ] |

### Integration Points
- `internal/plugins/ospf` (engine) -- the RI originator (`ri.go`), the `RegisterRITLV` registry, the origination hook in `originateSelfLSAs`, the capability-bit derivation.
- `internal/plugins/ospf` (ext-1, the dependency) -- `RegisterOpaqueConsumer`, `OnOriginate`/`OnReceive`, the generic `packet/opaque_tlv.go` helpers, the §5 gate (all consumed).
- `internal/plugins/ospf/v3/types/lsa.go` -- `LSTypeRouterInformation` function-code constant + scope helper.
- `internal/plugins/ospf/afstrategy_v6.go` / `origination_v6.go` -- the OSPFv3 RI originator sibling + `v6ManagedSelfTypes` entry.
- `internal/plugins/ospf/lsdb/origination.go` -- `OriginateSelf` (consumed unchanged) for OSPFv3.
- `internal/plugins/ospf/show_database.go` / `cmd_show.go` / `yang/ze-ospf-cmd.yang` -- the `router-information` subview/RPC/command binding.
- `internal/plugins/ospf/config.go` / `yang/ze-ospf-conf.yang` -- the `router-information` container.
- `internal/plugins/ospf/register.go` -- create + discover the `RegisterRITLV` registry.
- `internal/plugins/ospf/spf` -- NOT touched: RI never feeds SPF (informational only).

### Architectural Verification
- [ ] No bypassed layers (RI fills a body; carriage flows through ext-1 opaque (v2) or `OriginateSelf` (v3); reception/flooding/§5 inherited, not re-implemented)
- [ ] No unintended coupling (RI names no Segment Routing symbol; SR depends on the `RegisterRITLV` hook, not vice-versa; RI depends on ext-1, not vice-versa)
- [ ] No duplicated functionality (reuses ext-1's opaque consumer + TLV helpers, the OSPFv3 `OriginateSelf` seam, the existing LSDB stores/flooding/§13; adds only the RI body codec, the capability bitfield, the TLV registry, the per-scope wiring, config, and a show view)
- [ ] Zero-copy preserved (RI body built buffer-first; TLV iterator returns views; render reads stored bytes without re-decode-then-copy)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ext-1's `RegisterOpaqueConsumer` + `OnOriginate(opaqueID, scope, body, withdraw)` is sufficient to carry the OSPFv2 RI LSA with no RI-side flooding/LS-ID/O-bit work | `plan/spec-ospf-ext-1-opaque-framework.md` "In scope" (consumer registry, `OnOriginate` returns `(opaqueID, scope, body, withdraw)`) | RI must do opaque carriage itself; large scope creep; ext-1/ext-3 boundary wrong | `TestRIOpaqueConsumerRegistered`, `ospf-ri-originate.ci` shows Opaque type 4 with `OnOriginate`-supplied body | unvalidated |
| A-2 | The 24-bit Opaque ID can carry the RI LSA Instance ID directly (Instance 0 = Opaque ID 0) | RFC 7770 §2.1 "Opaque ID: the RI LSA Instance ID"; RFC 5250 App A.2 24-bit Opaque ID | Instance numbering needs a separate field; multi-instance breaks | `TestRIv2InstanceIDIsOpaqueID` (Instance 0 -> LS ID `0x04000000`) | unvalidated |
| A-3 | OSPFv3 RI can be originated through `OriginateSelf` exactly like `v6OriginateRouter`, with a new function-code-12 LSType and an entry in `v6ManagedSelfTypes` for stale flush | `origination_v6.go` `v6OriginateRouter` calls `OriginateSelf`; `v6ManagedSelfTypes`; `v3/types/lsa.go` scope bits | a new OSPFv3 origination path is needed | `TestRIv3OriginateArea`/`TestRIv3OriginateAS`, withdrawal via `FlushStaleSelfLSAs` | unvalidated |
| A-4 | The OSPFv3 LSDB already routes AS-scope LSAs (S2/S1 = AS) to the AS-wide store via `Scope()`, so the AS-scoped RI LSA needs no new store routing | `lsdb.go` store routing; `v3/types/lsa.go` `Scope()`; `LSTypeASExternal = 0x4005` AS precedent | AS-scoped RI mis-stored per-area; AS flooding broken | `TestRIv3ASScopeRouting` (function-code-12 AS LSA in AS store) | unvalidated |
| A-5 | ext-1's generic `packet/opaque_tlv.go` 4-byte-aligned TLV builder/iterator is reusable verbatim for the RI TLV stream (RFC 7770 §2.3 = RFC 3630 TLV) | RFC 7770 §2.3 "same TLV format as RFC 3630"; ext-1 "Generic TLV carriage" | RI needs its own TLV codec; duplication | `TestRITLVRoundTrip` over the ext-1 builder; `TestRITLVAlignment` value lengths 0..7 | unvalidated |
| A-6 | The §2.5 informational bits this router can truthfully set (0 GR-capable, 1 GR-helper, 2 stub-router, 3 TE) are all derivable from existing engine/config state with no new tracking | `config.go` `parseMaxMetric` (stub-router), GR helper config (ext-9 future), TE config (ext-2 future) | the bitfield cannot be accurate; §2.4 MUST violated; or RI must wait on ext-2/9 | `TestRICapabilityBitsFromState` (stub-router set -> bit 2; TE absent -> bit 3 clear) | unvalidated |
| A-7 | Setting the OSPFv3 U-bit on the RI LSA makes non-supporting routers flood it without breaking adjacency | RFC 7770 §2.2 + RFC 5340 U-bit semantics; `v3/types/lsa.go` U-bit is the top type bit | OSPFv3 peers drop or mishandle the RI LSA | `ospf6-ri-frr` interop forms full adjacency and floods the RI LSA | unvalidated |
| A-8 | Default scope area+AS is acceptable and matches the common SR deployment; the operator can narrow it | RFC 7770 §2.7 operator-selectable scope; guide §14 "feeds into Segment Routing" | default floods too widely or too narrowly; SR consumers miss the LSA | user confirmation of the default; `TestRIDefaultScopeAreaAndAS` | unvalidated |
| A-9 | RI never feeds SPF or the route table; it is informational and the empty functional TLV drives nothing | RFC 7770 §2.4 informational-only; §2.6 functional registry empty | RI corrupts the route table or SPF graph | `TestRINotInSPFGraph`; route table unchanged with RI on/off | unvalidated |
| A-10 | A single Instance 0 suffices unless a registered TLV builder overflows; overflow into Instance 1+ is rare and only triggered by SR-scale data | RFC 7770 §3 multi-instance; only the small type-1 TLV by default | overflow logic untested until ext-5; an over-long body silently truncates | `TestRIInstanceOverflow` with a synthetic large registered TLV | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | OSPFv2 LS ID built with the wrong Opaque type or Instance-ID byte order -> peers file the RI LSA under the wrong Opaque type | FRR shows the LSA as an unknown opaque type, not RI | RI passes Opaque type 4 + Instance ID to ext-1, which owns the split; `TestRIv2InstanceIDIsOpaqueID` pins LS ID `0x04000000` for Instance 0; `ospf-ri-frr` interop confirms FRR decodes it as RI |
| R-2 | OSPFv3 wire LS Type hard-coded to one value instead of per-scope (0x000C/0x200C/0x400C) -> area RI flooded AS-wide or dropped | the RI LSA appears in the wrong scope's database; FRR rejects the type | scope->LSType helper in `v3/types/lsa.go`; `TestRIv3LSTypePerScope` asserts all three; `Known()` accepts all three |
| R-3 | The type-1 Informational Capabilities TLV is not first in Instance 0 (e.g. a registered builder emitted before it) -> §2.4 MUST violation; FRR may reject | FRR logs a malformed RI LSA; interop fails | the originator ALWAYS emits the type-1 TLV first, then registered builders in type order; `TestRITLVType1First` asserts ordering even with a registered builder |
| R-4 | TLV Length includes padding (or the walker fails to skip padding) -> the TLV stream desyncs and the body is garbage | self round-trip passes but FRR rejects; or a received RI LSA renders wrong | reuse ext-1's tested 4-byte builder/iterator (Length = value octets only); `TestRITLVAlignment` for value lengths 0..7; decode an FRR RI capture |
| R-5 | Capability bits stale or wrong (e.g. TE bit set when TE is not configured) -> §2.4 MUST violation; peers believe a false capability | a peer attempts to use an advertised capability the router lacks | bits derived from live state each origination; `TestRICapabilityBitsFromState`; re-origination on config change |
| R-6 | A registered TLV builder (SR/ext-5) panics during RI origination and takes down OSPF | a single bad consumer crashes the engine | the originator recover-wraps each `BuildFn` (mirroring ext-1's consumer recover), counts a metric, and emits the RI LSA without the failing TLV; `TestRITLVBuilderPanicIsolated` |
| R-7 | RI LSA re-originated on every tick even when nothing changed -> LSDB churn and MinLSInterval pressure | excessive RI sequence-number increments; peers see constant RI updates | the body is compared to the last originated body; identical body re-floods nothing (idempotent, via `OriginateSelf`'s unchanged-body short-circuit); `TestRIOriginateIdempotent` |
| R-8 | OSPFv2 RI and OSPFv3 RI diverge in TLV encoding because the body is built twice | a capture shows different TLV bytes for the same capabilities across address families | a SINGLE shared RI body builder is used by both carriages; `TestRIBodyIdenticalAcrossAF` compares the two bodies byte-for-byte |
| R-9 | AS-scope chosen but no area-scoped RI into attached NSSAs (SHOULD §2.7) -> NSSA-internal routers miss the RI LSA | an NSSA-internal router has no RI LSA from this router | when scope includes AS and an NSSA is attached, also originate area-scoped RI into the NSSA; documented as a SHOULD; `TestRIASScopeAlsoAreaIntoNSSA` |
| R-10 | Received RI LSA with a malformed/truncated TLV crashes the renderer (untrusted input) | fuzz crash on `show ... database router-information` | the ext-1 iterator is bound-checked and never panics; the renderer tolerates a truncated stream; extend the packet fuzz target with RI bodies |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| RI enabled in config; `originateSelfLSAs` fires (OSPFv2) | -> | RI originator -> `OnOriginate` (Opaque type 4) -> ext-1 builds Opaque LSA -> install + flood | `TestRIv2OriginateArea` (unit) + `test/ospf/ospf-ri-originate.ci` |
| RI enabled in config; `originateSelfLSAs` fires (OSPFv3) | -> | OSPFv3 RI originator -> `OriginateSelf` (function code 12) -> install + flood | `TestRIv3OriginateArea` (unit) + `test/ospf/ospf6-ri-originate.ci` |
| A consumer calls `RegisterRITLV(tlvType, scope, BuildFn)` from `init()` | -> | the RI TLV registry stores it; the originator invokes it after the type-1 TLV | `TestRITLVRegistered` (unit) + `test/ospf/ospf-ri-register-tlv.ci` |
| An LS Update carrying an RI LSA (Opaque type 4) arrives | -> | ext-1 opaque receive -> RI `OnReceive` -> stored; `show ... database router-information` lists it | `test/ospf/ospf-ri-receive.ci` |
| `show ospf database router-information` is run | -> | `cmd_show.go` RPC -> `databaseSnapshotByType("router-information")` -> TLV decode + render | `test/ospf/ospf-ri-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RI enabled, OSPFv2, scope=area | an Opaque type-4 LSA (LS type 10) is originated into each attached area via ext-1; LS ID = `0x0A...`? -> Opaque type 4 high byte, Instance ID 0 low 24 bits (LS ID `0x04000000`); it carries the type-1 Informational Capabilities TLV first (§2.1, §2.4) |
| AC-2 | RI enabled, OSPFv2, scope=as | one Opaque type-4 LSA (LS type 11) is originated AS-wide; flooded only to opaque-capable neighbours; NOT into stub/NSSA (ext-1 §3.1/§5 inherited) |
| AC-3 | RI enabled, OSPFv3, scope=area | a native RI LSA (function code 12, wire type `0x200C`, U=1) is originated into each attached area via `OriginateSelf`; non-supporting routers still flood it (U-bit) (§2.2) |
| AC-4 | RI enabled, OSPFv3, scope=as | a native RI LSA (function code 12, wire type `0x400C`) is originated AS-wide and routed to the AS-wide store (§2.2) |
| AC-5 | Default config (no explicit scope) | RI is originated with scope area + AS in both address families (the documented default) |
| AC-6 | Router has GR-helper enabled and stub-router (max-metric) configured, TE not configured | the type-1 TLV sets bit 1 (GR-helper) and bit 2 (stub-router), clears bit 3 (TE); the bitfield accurately reflects live state (§2.4, §2.5) |
| AC-7 | A consumer registers a TLV builder via `RegisterRITLV(type, scope, BuildFn)` | the RI LSA contains the type-1 TLV FIRST, then the registered TLV; total length accounts for 4-byte padding of each TLV (§2.3, §2.4) |
| AC-8 | A registered TLV builder panics | the originator recovers, increments `ze_ospf_ri_tlv_builder_errors_total`, and emits the RI LSA without the failing TLV; OSPF continues |
| AC-9 | RI disabled (or a scope removed) after being advertised | the previously originated RI LSA(s) are MaxAge-flushed through the existing purge path (OSPFv2 via ext-1 withdraw, OSPFv3 via `FlushStaleSelfLSAs`); peers withdraw them |
| AC-10 | RI enabled, no config change, periodic tick fires | the RI body is unchanged so re-origination floods nothing (idempotent); the sequence number does not advance |
| AC-11 | The same capabilities advertised in OSPFv2 and OSPFv3 | the RI TLV body bytes are identical across the two address families (single shared builder) |
| AC-12 | `show ospf database router-information` run with an RI LSA present | the output lists the Instance ID, the decoded capability bits by name, and each TLV (type, length, value summary), for both OSPFv2 opaque and OSPFv3 native |
| AC-13 | A received RI LSA from FRR (Opaque type 4 / function code 12) | it is stored and rendered; the informational bits are decoded; no protocol behavior changes from the received informational bits |
| AC-14 | A received RI LSA with a truncated/malformed TLV stream | the renderer reports what it can and does not crash (bound-checked iterator) |
| AC-15 | Multiple RI LSA instances received for the same scope/router (unspecified multi-instance TLV) | the renderer/consumer uses the numerically smallest Instance ID and ignores the rest (§3) |
| AC-16 | A registered TLV builder overflows the maximum LSA length for Instance 0 | overflow TLVs are emitted in Instance 1+, with Instance 0 retaining the type-1 TLV first (§2.4, §3) |
| AC-17 | Any RI LSA in any store | it never appears as an SPF vertex and never changes the route table (informational only, §2.4) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables `router-information` (default scope) on an OSPFv2 router | config -> `ospfConfig` -> `originateSelfLSAs` -> RI `OnOriginate` (Opaque type 4) -> ext-1 install + flood; FRR's `show ip ospf database opaque-area` shows the RI LSA with capability bits | `test/ospf/ospf-ri-originate.ci` + `ospf-ri-frr` interop |
| 2 | Enables `router-information` on an OSPFv3 router | config -> `originateSelfLSAs` -> OSPFv3 RI originator -> `OriginateSelf` (function code 12) -> install + flood; FRR's `show ipv6 ospf6 database router-information` shows it | `test/ospf/ospf6-ri-originate.ci` + `ospf6-ri-frr` interop |
| 3 | A Segment Routing module (ext-5, future) registers SR TLVs | `RegisterRITLV(sr-algorithm, ...)` from ext-5 `init()` -> RI originator appends the SR TLVs after the type-1 TLV in the SAME RI LSA | `TestRITLVRegistered` + `test/ospf/ospf-ri-register-tlv.ci` (with a test-stub TLV builder) |
| 4 | Inspects advertised capabilities | `show ospf database router-information` -> decode TLV stream -> render bits + TLVs | `test/ospf/ospf-ri-show.ci` |
| 5 | Receives an RI LSA from FRR (both AFs) | wire -> ext-1 opaque receive (v2) / native install (v3) -> stored -> rendered; informational bits decoded, no behavior change | `test/ospf/ospf-ri-receive.ci` + both interop scenarios |
| 6 | Disables `router-information` | config change -> RI withdraw (MaxAge flush) -> peers purge the RI LSA; OSPF otherwise unchanged | `test/ospf/ospf-ri-withdraw.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRITLVRoundTrip` | `internal/plugins/ospf/packet/ri_tlv_test.go` | A-5/AC-7: encode then decode the RI TLV stream byte-for-byte over the ext-1 builder | |
| `TestRITLVAlignment` | `internal/plugins/ospf/packet/ri_tlv_test.go` | AC-7, R-4: each TLV 4-byte aligned for value lengths 0..7; Length excludes padding | |
| `TestRITLVType1First` | `internal/plugins/ospf/packet/ri_tlv_test.go` | AC-7, R-3: the type-1 Informational TLV is emitted first even with a registered builder | |
| `TestRICapabilityBitsFromState` | `internal/plugins/ospf/ri_test.go` | AC-6, A-6, R-5: bits 0-3 derived from live GR/stub-router/TE state; absent capability -> bit clear | |
| `TestRIBodyIdenticalAcrossAF` | `internal/plugins/ospf/ri_test.go` | AC-11, R-8: the RI body bytes are identical for OSPFv2 and OSPFv3 given the same capabilities | |
| `TestRIv2InstanceIDIsOpaqueID` | `internal/plugins/ospf/ri_v2_test.go` | AC-1, A-2, R-1: Instance 0 -> LS ID `0x04000000`; Opaque type 4 high byte, Instance ID low 24 bits | |
| `TestRIv2OriginateArea` / `TestRIv2OriginateAS` | `internal/plugins/ospf/ri_v2_test.go` | AC-1/AC-2, A-1: OSPFv2 RI originated via ext-1 `RegisterOpaqueConsumer`/`OnOriginate` per scope | |
| `TestRIOpaqueConsumerRegistered` | `internal/plugins/ospf/ri_v2_test.go` | A-1: RI registers Opaque type 4 with ext-1 at startup | |
| `TestRIv3LSTypePerScope` | `internal/plugins/ospf/v3/types/lsa_ri_test.go` | AC-3/AC-4, R-2: function code 12 -> 0x000C link, 0x200C area, 0x400C AS; all three `Known()` | |
| `TestRIv3OriginateArea` / `TestRIv3OriginateAS` | `internal/plugins/ospf/origination_v6_ri_test.go` | AC-3/AC-4, A-3: OSPFv3 RI originated via `OriginateSelf` per scope; in `v6ManagedSelfTypes` | |
| `TestRIv3ASScopeRouting` | `internal/plugins/ospf/origination_v6_ri_test.go` | AC-4, A-4: function-code-12 AS LSA routed to the AS-wide store via `Scope()` | |
| `TestRIv3UBitSet` | `internal/plugins/ospf/origination_v6_ri_test.go` | AC-3, A-7: the OSPFv3 RI LSA has U=1 (flood-if-not-understood) | |
| `TestRIDefaultScopeAreaAndAS` | `internal/plugins/ospf/config_test.go` | AC-5, A-8: with no explicit scope, RI defaults to area + AS | |
| `TestRITLVRegistered` / `TestRITLVRegisteredOrder` | `internal/plugins/ospf/ri_registry_test.go` | AC-7: `RegisterRITLV` stored; builders invoked in ascending TLV-type order after type-1 | |
| `TestRITLVBuilderPanicIsolated` | `internal/plugins/ospf/ri_registry_test.go` | AC-8, R-6: a panicking builder is recovered, counted, omitted; RI still emitted | |
| `TestRIOriginateIdempotent` | `internal/plugins/ospf/ri_test.go` | AC-10, R-7: unchanged body re-floods nothing; sequence does not advance | |
| `TestRIWithdrawFlushes` | `internal/plugins/ospf/ri_test.go` | AC-9: disable -> MaxAge flush (v2 ext-1 withdraw, v3 `FlushStaleSelfLSAs`) | |
| `TestRIInstanceOverflow` | `internal/plugins/ospf/ri_test.go` | AC-16, A-10: a large registered TLV overflows into Instance 1+; Instance 0 keeps type-1 first | |
| `TestRIMultiInstanceSmallestID` | `internal/plugins/ospf/ri_test.go` | AC-15: received multiple instances -> smallest Instance ID used, rest ignored (§3) | |
| `TestRINotInSPFGraph` | `internal/plugins/ospf/spf/ri_exclusion_test.go` | AC-17, A-9: RI LSAs never become SPF vertices; route table unchanged | |
| `TestRIShowRender` | `internal/plugins/ospf/show_database_test.go` | AC-12: `router-information` subview decodes bits + TLVs for v2 and v3 | |
| `TestRIShowMalformedTLV` | `internal/plugins/ospf/show_database_test.go` | AC-14, R-10: a truncated TLV stream renders partially, does not crash | |
| `TestRIASScopeAlsoAreaIntoNSSA` | `internal/plugins/ospf/ri_test.go` | R-9 (§2.7 SHOULD): AS scope + attached NSSA also originates area-scoped RI into the NSSA | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| OSPFv2 Opaque type (RI) | 4 (fixed) | 4 | N/A | N/A (constant) |
| RI Instance ID (OSPFv2 Opaque ID) | 0-16777215 | 16777215 | N/A | masked to 24 bits |
| RI Instance ID (OSPFv3 Link State ID) | 0-4294967295 | 4294967295 | N/A | N/A (32-bit) |
| OSPFv3 RI wire LS Type | {0x000C, 0x200C, 0x400C} | 0x400C | N/A | other function codes rejected by the RI path |
| Informational Capabilities TLV Length | multiple of 4, initially 4 | any multiple of 4 | a non-multiple-of-4 is malformed | a Length past the LSA length is an iterator error |
| Capability bit index used by this spec | 0-5 (sets 0-3) | 3 | N/A | bits 6-31 left clear (unassigned) |
| RI TLV value length | 0-65531 | any | N/A | a length pushing past the LSA Length is an iterator error |
| RI TLV padding | 0-3 bytes | 3 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-ri-originate` | `test/ospf/ospf-ri-originate.ci` | OSPFv2: enable RI; an Opaque type-4 RI LSA appears with the type-1 TLV in `show ospf database router-information` | |
| `ospf6-ri-originate` | `test/ospf/ospf6-ri-originate.ci` | OSPFv3: enable RI; a function-code-12 RI LSA appears | |
| `ospf-ri-register-tlv` | `test/ospf/ospf-ri-register-tlv.ci` | a test-stub `RegisterRITLV` builder's TLV appears after the type-1 TLV in the RI LSA | |
| `ospf-ri-receive` | `test/ospf/ospf-ri-receive.ci` | a received RI LSA is stored and rendered; informational bits decoded | |
| `ospf-ri-show` | `test/ospf/ospf-ri-show.ci` | `show ospf database router-information` renders bits + TLVs (both AFs) | |
| `ospf-ri-withdraw` | `test/ospf/ospf-ri-withdraw.ci` | disabling RI MaxAge-flushes the RI LSA; peers purge it | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-ri-frr` | `test/interop/scenarios/ospf-ri-frr/` | FRR `ospfd` (opaque + RI on) | FRR decodes Ze's OSPFv2 RI LSA as Opaque type 4 / Router Information with the type-1 capability TLV; Ze stores and renders FRR's RI LSA; adjacency unaffected | |
| `ospf6-ri-frr` | `test/interop/scenarios/ospf6-ri-frr/` | FRR `ospf6d` (RI on) | FRR decodes Ze's OSPFv3 RI LSA (function code 12, U-bit) and floods it; Ze stores and renders FRR's RI LSA; adjacency unaffected | |

> Interop is required: this changes wire behaviour (a new Opaque type-4 LSA and a
> new OSPFv3 function-code-12 LSA on the wire). The raw-IP / multicast paths are
> Linux-only and run as QEMU integration tests (`ai/rules/qemu-testing.md`),
> consistent with the rest of the OSPF interop set.

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above. SR-specific TLV content is ext-5's responsibility (the `RegisterRITLV` hook is tested with a stub builder here).

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/instance.go` -- invoke the RI originator from `originateSelfLSAs` (after base self-LSAs) in both the OSPFv2 and OSPFv3 branches
- `internal/plugins/ospf/afstrategy_v6.go` / `internal/plugins/ospf/origination_v6.go` -- add the OSPFv3 RI originator sibling of `v6OriginateRouter`; add `LSTypeRouterInformation` to `v6ManagedSelfTypes`
- `internal/plugins/ospf/v3/types/lsa.go` -- add `LSTypeRouterInformation` (function code 12) + a scope->LSType helper; include the three scoped values in `Known()`
- `internal/plugins/ospf/config.go` -- `parseRouterInformation` (enable + scope list) following the `parseMaxMetric` pattern; resolve into `ospfConfig`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- a `router-information` container (`enabled` boolean; `scope` leaf-list enumeration link/area/as, default area+as)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `show ospf database router-information` command binding
- `internal/plugins/ospf/show_database.go` -- add `router-information` to `dbSubviewType`; render the RI TLV stream (bits + TLVs)
- `internal/plugins/ospf/cmd_show.go` -- register `ze-show:ospf-database-router-information` -> `dbSubviewForwarder("show ospf database router-information")`
- `internal/plugins/ospf/register.go` -- create + discover the `RegisterRITLV` in-process registry; register the OSPFv2 RI opaque consumer (Opaque type 4) with ext-1
- `internal/plugins/ospf/doctor.go` -- (only if a runtime dependency is added; none expected -- no new socket/port)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `router-information` container; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `enabled` boolean (native); `scope` `enumeration` link/area/as (native); no custom validator |
| YANG custom validators | [ ] no | native enumeration + boolean suffice |
| CLI commands/flags | [ ] yes | `show ospf database router-information` in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ospf database router-information` |
| Editor autocomplete | [ ] yes | automatic for the YANG enumeration/boolean + the new show subcommand |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-ri-*.ci`, `ospf6-ri-*.ci` |
| Pipe completeness | [ ] yes | `show ospf database router-information` routes through `ApplyPipes` like the other show subviews |
| Env var registration | [ ] no | RI scope/enable is operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary/cert; reuses the existing OSPF raw socket and ext-1 carriage |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_ri_lsas` | gauge | `af` (v2/v3), `scope` (link/area/as) |
| `ze_ospf_ri_originations_total` | counter | `af`, `scope` |
| `ze_ospf_ri_received_total` | counter | `af` |
| `ze_ospf_ri_tlv_builder_errors_total` | counter | (none) |
| `ze_ospf_ri_capability_bits` | gauge | `bit` (gr-capable/gr-helper/stub-router/te) |

> These extend the OSPF metric set with the `ze_ospf_ri_*` prefix and are
> registered by this spec's owner code. ext-1's `ze_ospf_opaque_*` series count the
> OSPFv2 RI LSA as one opaque type; these RI series are address-family-neutral and
> additionally cover OSPFv3.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF Router Information LSA (RFC 7770) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `router-information` container |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ospf database router-information` |
| 4 | API/RPC added/changed? | [ ] no | the show RPC lives in the central `ze-show` namespace; document under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains an RI originator + RI-TLV registry |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- Router Information section |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` (or equivalent) -- RI LSA (Opaque type 4 / function code 12) + TLV format |
| 8 | Plugin SDK/protocol changed? | [ ] yes | document `RegisterRITLV` for ext-5 (SR) authors |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc7770.md` -- flip the RI/Informational-TLV compliance items to implemented |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF RI LSA parity with FRR |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- RI body codec, the dual carriage (opaque v2 / native v3), the TLV registry |
| 13 | Route metadata keys added/changed? | [ ] no | RI LSAs install no routes |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the five `ze_ospf_ri_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` -- the `router-information` show RPC + the RI-TLV registry |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF files |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF config/CLI examples against the new `router-information` leaf |

## Files to Create
- `internal/plugins/ospf/ri.go` -- the address-family-neutral RI originator: capability-bit derivation, the shared RI body builder, per-scope origination dispatch (calls ext-1 for v2, the OSPFv3 sibling for v3), idempotency, withdraw, instance overflow
- `internal/plugins/ospf/ri_registry.go` -- `RegisterRITLV(tlvType, scope, BuildFn)`, the registry, the builder recover wrapper
- `internal/plugins/ospf/packet/ri_tlv.go` -- the RI TLV helpers over the ext-1 generic TLV builder/iterator (the type-1 Informational Capabilities bitfield encode/decode, the type-2 Functional carrier, the smallest-Instance-ID selection)
- `internal/plugins/ospf/ri_test.go`, `internal/plugins/ospf/ri_v2_test.go`, `internal/plugins/ospf/ri_registry_test.go`
- `internal/plugins/ospf/origination_v6_ri_test.go`
- `internal/plugins/ospf/packet/ri_tlv_test.go`
- `internal/plugins/ospf/v3/types/lsa_ri_test.go`
- `internal/plugins/ospf/spf/ri_exclusion_test.go`
- `test/ospf/ospf-ri-originate.ci`, `ospf6-ri-originate.ci`, `ospf-ri-register-tlv.ci`, `ospf-ri-receive.ci`, `ospf-ri-show.ci`, `ospf-ri-withdraw.ci`
- `test/interop/scenarios/ospf-ri-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospf6-ri-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm ext-1's `RegisterOpaqueConsumer` + TLV helpers exist and the OSPFv3 `OriginateSelf` seam is reachable |
| 3. Wiring phase | Wiring Test table -- RI registers with ext-1 (v2), the OSPFv3 RI type + originator stub (v3), the RI-TLV registry; failing wiring tests |
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

<!-- Phase 1 is ALWAYS wiring. -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register the RI carriers + the RI-TLV registry + failing wiring tests
   - Tests: `TestRIOpaqueConsumerRegistered`, `TestRITLVRegistered`, `test/ospf/ospf-ri-originate.ci` (failing), `test/ospf/ospf6-ri-originate.ci` (failing)
   - Files: `register.go` (register Opaque type 4 with ext-1; create + discover the `RegisterRITLV` registry), `ri.go` (originator skeleton -> stub body), `v3/types/lsa.go` (`LSTypeRouterInformation` + scope helper), `ri_registry.go` (`RegisterRITLV` + recover wrapper), the `originateSelfLSAs` hook (both branches), a test-stub TLV builder
   - Verify: RI registers as an opaque consumer and as a native OSPFv3 type; the origination hook is reachable; deeper tests still fail because the body/scope/bits are stubs
2. **Phase: RI TLV codec + capability bits** -- the shared body
   - Tests: `TestRITLVRoundTrip`, `TestRITLVAlignment`, `TestRITLVType1First`, `TestRICapabilityBitsFromState`, `TestRIBodyIdenticalAcrossAF`
   - Files: `packet/ri_tlv.go` (type-1 bitfield encode/decode over the ext-1 builder; type-2 carrier; smallest-Instance-ID), `ri.go` (capability-bit derivation from live state, the shared body builder)
   - Verify: the type-1 TLV is first; bits reflect state; the body is identical across AFs; TLVs are 4-byte aligned
3. **Phase: OSPFv2 carriage (ext-1 consumer)** -- originate + withdraw via opaque
   - Tests: `TestRIv2InstanceIDIsOpaqueID`, `TestRIv2OriginateArea`, `TestRIv2OriginateAS`, `TestRIWithdrawFlushes` (v2 path)
   - Files: `ri.go` (the `OnOriginate` returning `(InstanceID, scope, body, withdraw)`), `register.go`
   - Verify: Instance 0 -> LS ID `0x04000000`; per-scope LSAs originated/withdrawn through ext-1
4. **Phase: OSPFv3 carriage (native self-LSA)** -- function code 12 per scope
   - Tests: `TestRIv3LSTypePerScope`, `TestRIv3OriginateArea`, `TestRIv3OriginateAS`, `TestRIv3ASScopeRouting`, `TestRIv3UBitSet`, `TestRIWithdrawFlushes` (v3 path)
   - Files: `origination_v6.go` / `afstrategy_v6.go` (the RI originator sibling; `v6ManagedSelfTypes` entry), `v3/types/lsa.go`
   - Verify: per-scope wire types; U-bit set; AS scope to the AS store; withdraw via `FlushStaleSelfLSAs`
5. **Phase: TLV registry + scope/idempotency/overflow** -- the hook and the originator policy
   - Tests: `TestRITLVRegisteredOrder`, `TestRITLVBuilderPanicIsolated`, `TestRIOriginateIdempotent`, `TestRIDefaultScopeAreaAndAS`, `TestRIInstanceOverflow`, `TestRIMultiInstanceSmallestID`, `TestRIASScopeAlsoAreaIntoNSSA`, `ospf-ri-register-tlv.ci`
   - Files: `ri_registry.go`, `ri.go`, `config.go` (`parseRouterInformation`), `yang/ze-ospf-conf.yang`
   - Verify: registered builders ordered after type-1; panics isolated; default scope area+AS; overflow into Instance 1+; smallest-instance selection; §2.7 NSSA SHOULD
6. **Phase: CLI render + SPF exclusion + metrics** -- user surface
   - Tests: `TestRIShowRender`, `TestRIShowMalformedTLV`, `TestRINotInSPFGraph`, `ospf-ri-show.ci`, `ospf-ri-receive.ci`, `ospf-ri-withdraw.ci`
   - Files: `show_database.go`, `cmd_show.go`, `yang/ze-ospf-cmd.yang`, `spf/` (confirm RI excluded), metric registration
   - Verify: `show ospf database router-information` renders both AFs; malformed TLV does not crash; RI never an SPF vertex; the five metric series
7. **Functional tests** -> the six `.ci` cover the user-visible behaviour
8. **RFC refs** -> add `// RFC 7770 Section X` comments on the type-1-first rule, the per-scope wire type, the Instance-ID/Opaque-ID mapping, the §3 smallest-instance rule, and the U-bit
9. **Interop** -> `ospf-ri-frr` and `ospf6-ri-frr` QEMU scenarios
10. **Full verification** -> `make ze-verify`
11. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; RI parity with FRR's RFC 7770 RI LSA (type-1 capability TLV, both AFs); SR TLVs excluded by design (hook only) |
| Correctness | OSPFv2 LS ID = `4<<24 | InstanceID`; OSPFv3 wire type per scope (0x000C/0x200C/0x400C, U=1); type-1 TLV first in Instance 0; TLV Length excludes padding; bits reflect live state; smallest-Instance-ID on receive |
| Naming | `ze_ospf_ri_*` metrics; YANG `router-information`/`scope`/`enabled` kebab-case; `RegisterRITLV`; `LSTypeRouterInformation` |
| Data flow | RI fills a body; carriage via ext-1 (v2) / `OriginateSelf` (v3); SPF untouched; no SR symbol in RI |
| CLI grammar | `show ospf database router-information` action-before-identifier |
| Doctor checks | none added (no new runtime dependency) -- confirm |
| YANG validation | `enabled` boolean, `scope` enumeration native-constrained |
| Prometheus counters | the five `ze_ospf_ri_*` series defined, registered, listed |
| Rule: plugin-self-containment | RI names no Segment Routing; `RegisterRITLV` is consumer-neutral; removing ext-5 removes its TLVs cleanly |
| Rule: buffer-first | the RI body + TLVs built into caller buffers; the iterator is zero-copy |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| OSPFv2 RI registered as Opaque type 4 with ext-1 | `grep -rn 'RegisterOpaqueConsumer' internal/plugins/ospf/ri*.go register.go` |
| OSPFv3 `LSTypeRouterInformation` (function code 12) | `grep -rn 'LSTypeRouterInformation' internal/plugins/ospf/v3/types` |
| Shared RI body builder + type-1 TLV | `go test ./internal/plugins/ospf/packet -run 'RITLV'` |
| `RegisterRITLV` hook exists and is exercised by a stub | `grep -rn 'RegisterRITLV' internal/plugins/ospf` |
| Capability bits from live state | `go test ./internal/plugins/ospf -run TestRICapabilityBitsFromState` |
| `router-information` config container | `grep -n 'router-information' internal/plugins/ospf/yang/ze-ospf-conf.yang` |
| `show ospf database router-information` | `go test ./internal/plugins/ospf -run TestRIShowRender` |
| Five metric series registered | `grep -rn 'ze_ospf_ri_' internal/plugins/ospf` |
| Interop scenarios present | `ls test/interop/scenarios/ospf-ri-frr/ test/interop/scenarios/ospf6-ri-frr/` |
| Functional tests present | `ls test/ospf/ospf-ri-*.ci test/ospf/ospf6-ri-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the RI TLV iteration is bound-checked (ext-1 iterator); a truncated/over-length received RI LSA never panics the renderer; the packet fuzz target is extended with RI bodies |
| Resource exhaustion | RI origination is rate-limited by the existing MinLSInterval; instance overflow is bounded by the maximum LSA length and a cap on instances; a registered builder cannot emit unbounded TLVs (length-checked) |
| Consumer isolation | `RegisterRITLV` builders are recover-wrapped; a panicking or slow builder cannot crash OSPF or wedge origination |
| Capability accuracy | the advertised bits cannot claim a capability the router lacks (derived from live state); no operator override that would let the advertisement lie |
| Trust boundary | received RI LSAs ride the existing OSPF authentication; informational bits drive NO protocol behavior, so a forged RI LSA cannot alter routing |
| Error leakage | builder errors are counted, not surfaced to peers; the RI LSA is emitted without the failing TLV |

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
RI has two carriages but one body. OSPFv2 RI is an ext-1 opaque consumer (Opaque
type 4); OSPFv3 RI is a native function-code-12 self-LSA. The protocol-meaningful
work -- LS-ID split, scope flooding, the O-bit, §5 reachability (v2), and the
self-LSA sequencing/install/flood (both) -- already exists in ext-1 and the
`OriginateSelf` seam. This spec adds only the RI TLV body, the capability bitfield
filled from live state, a consumer-neutral `RegisterRITLV` hook so Segment Routing
plugs into the same LSA, and the per-scope/config/show wiring. RI never feeds SPF.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| OSPFv2 RI as an ext-1 opaque consumer; OSPFv3 RI as a native self-LSA | a single unified RI carriage | the two address families carry RI differently per RFC 7770 (opaque type 4 vs function code 12); reusing each AF's existing carriage avoids reimplementing flooding/scope twice |
| One shared RI body builder for both AFs | build the body separately per AF | the TLV body is identical (RFC 7770 §2.3); a single builder guarantees byte-identical advertisements and one fuzz surface |
| `RegisterRITLV` hook with no consumer name | bake SR TLVs into RI | plugin-self-containment + RFC 7770's TLV-extensible design; SR (ext-5) owns its TLVs; RI carries the type-1 TLV and a hook |
| Capability bits derived from live state | a config flag per capability bit | §2.4 MUST: the advertisement must be accurate; deriving from real state cannot lie and stays correct as features toggle |
| Default scope area + AS | area-only, or AS-only, or link | matches the common SR deployment (guide §14) where consumers need both; operator-narrowable per §2.7 |
| RI excluded from SPF | allow RI to influence path computation | RFC 7770 §2.4 informational-only; only the (empty) functional TLV may drive behavior, and this spec drives none |

## Known Limitations
- No Functional Capabilities content ships: the §5.5 functional registry is empty in RFC 7770, so the type-2 TLV is carried empty (absence = not supported, §2.6).
- No SR / Extended-Link/Prefix TLVs: those are ext-4/ext-5, which plug in via `RegisterRITLV`; without them the RI LSA carries only the type-1 informational TLV.
- Received informational bits are rendered, never acted upon: this spec changes no protocol behavior based on a peer's advertised capabilities (only the functional TLV could, and it is empty).
- TE / GR capability bits reflect configuration only; the actual TE LSA (ext-2) and Grace-LSA/helper (ext-9) machinery are separate specs -- setting the bit does not imply those features are implemented here.

## RFC Documentation

Add `// RFC 7770 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §2.1 OSPFv2 RI = Opaque type 4; Opaque ID = Instance ID
- §2.2 OSPFv3 RI = function code 12, U-bit set; per-scope wire LS Type
- §2.3 TLV format: Length excludes padding; 4-octet alignment; unrecognized types ignored
- §2.4 the Informational Capabilities TLV MUST be the first TLV in Instance 0 and MUST accurately reflect capabilities
- §2.5 informational capability bit assignments (0 GR-capable, 1 GR-helper, 2 stub-router, 3 TE)
- §2.6 the Functional Capabilities TLV; absence means not supported
- §3 multi-instance: smallest Instance ID wins for unspecified-multi-instance TLVs
- §2.7 per-TLV flooding scope; AS scope SHOULD also advertise area-scoped into attached NSSAs

Add `// RFC 5250 Section X` where RI relies on the ext-1 carrier (the LS-ID split and the §5 gate it inherits).

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
| Originate/flood/store the OSPFv2 RI Opaque LSA (Opaque type 4) | functional + interop | `ospf-ri-originate.ci`, `ospf-ri-frr` |
| Originate/flood/store the OSPFv3 RI LSA (function code 12) | functional + interop | `ospf6-ri-originate.ci`, `ospf6-ri-frr` |
| Carry the Router Informational Capabilities TLV (type 1) | unit + functional | `TestRICapabilityBitsFromState`, `ospf-ri-show.ci` |
| TLV-registration hook for downstream consumers (SR/ext-5) | unit + functional | `TestRITLVRegistered`, `ospf-ri-register-tlv.ci` |
| Configurable advertisement scope (default area+AS) | unit + functional | `TestRIDefaultScopeAreaAndAS`, `ospf-ri-originate.ci` |
| RI is informational only (no SPF/route effect) | unit | `TestRINotInSPFGraph` |

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
- [ ] RFC 7770 + RFC 5250 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the `RegisterRITLV` hook has a concrete consumer planned -- ext-5 SR)
- [ ] No speculative features (only the type-1 TLV + the hook; no SR TLV bodies)
- [ ] Single responsibility per component (RI fills a body; carriage owned elsewhere)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (RI names no Segment Routing; depends on ext-1, not vice-versa)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-ri-frr`, `ospf6-ri-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-3-router-information.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-3-router-information.md`
