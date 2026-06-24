# Spec: ospf-ext-1 -- OSPFv2 Opaque-LSA Framework (RFC 5250)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-0-umbrella.md (delivered) |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc5250.md` -- Opaque-LSA framework: Type 9/10/11 scope rules (§3.1), Link State ID split (§3 / App A.2), O-bit negotiation (§3.1 / App A.1), Type-11 reachability (§5)
4. `plan/spec-ospf-0-umbrella.md` -- delivered OSPFv2 umbrella; "Shared Contracts" (LSA inventory row 9/10/11, LSA header layout, flooding eligibility), and the note that opaque is a future framework on the stable base
5. `internal/plugins/ospf/lsdb/flooding.go` -- the §13 flooding procedure, `floodExcept`, `eligibleInterface`, `ReceiveUpdate`, `shouldDropByArea`
6. `internal/plugins/ospf/lsdb/link_scope.go` -- the per-interface link-scope store (`d.links`), `OriginateLinkSelf`, `floodLink`, `isLinkLSAType` (the existing precedent for scope #9)
7. `internal/plugins/ospf/lsdb/lsdb.go` -- `dbForLocked` store routing, `asExternal` store, `install`/`installLocked`, `Install`
8. `internal/plugins/ospf/lsdb/origination.go` -- `OriginateSelf`, `SelfLSAEncoder`, `OriginateExternal`, `FlushStaleSelfLSAs`
9. `internal/plugins/ospf/packet/lsa.go`, `internal/plugins/ospf/packet/lsa_opaque.go` -- the `LSA` struct's `Opaque *OpaqueLSA` field and verbatim passthrough
10. `internal/plugins/ospf/types/lstype.go` -- `LSTypeOpaqueLink/Area/AS`, `IsOpaque()`; `internal/plugins/ospf/types/options.go` -- `OptionO` (the O-bit)

## Task

Add the OSPFv2 Opaque-LSA carrier framework (RFC 5250, which obsoletes RFC 2370)
to the existing native OSPFv2 plugin at `internal/plugins/ospf/`. The umbrella
(`plan/spec-ospf-0-umbrella.md`) delivered OSPFv2 with opaque LSAs out of scope:
the codec already recognises LS types 9/10/11 and retains their bodies verbatim
(`packet.OpaqueLSA`), the types leaf already classifies them (`IsOpaque()`), and
the O-bit constant (`types.OptionO`) already exists. What is missing is the
*active* framework: scope-correct flooding for the three opaque scopes, the
Opaque Type / Opaque ID split of the Link State ID, the O-bit capability
negotiation that gates opaque flooding to opaque-capable neighbours, and a
registration API that lets consumer modules (TE / ext-2, Router-Information /
ext-3, Extended-Link/Prefix / ext-4, Grace-LSA / ext-9) claim an Opaque Type and
hook origination plus reception/parse.

This is the foundation the four dependent specs build on. It deliberately
implements NO opaque consumer semantics: no TLV interpretation, no TE / RI / SR
bodies, no SPF participation (opaque LSAs are never vertices). It provides only
the generic carrier: scope-aware storage and flooding, the LS-ID split, the O-bit
gate, generic 4-byte-aligned TLV iteration/emission helpers, and the consumer
registry. A consumer registers an Opaque Type with an origination callback (the
framework assigns sequence numbers, installs, and floods) and a reception
callback (the framework decodes the header, hands the consumer the opaque body
and Opaque ID, and re-floods verbatim regardless of whether a consumer is
registered). An unregistered opaque LSA is still flooded per its scope and never
crashes the engine.

The framework runs entirely inside the existing OSPF edge plugin. It registers
through the OSPF plugin's own in-process registry (not a new top-level
component), and consumer modules remain self-contained: removing a consumer
removes its Opaque Type registration and all its behaviour, leaving the carrier
intact.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Scope-correct flooding | Type 9 link-local (the existing link store), Type 10 area (the existing per-area store), Type 11 AS-wide (a new AS-wide opaque store, parallel to `asExternal`); reuse `floodExcept`/`floodLink` restricted by the new scope rules (§3.1) |
| LS-ID split | Encode/decode the 32-bit Link State ID as Opaque Type (high 8 bits) + Opaque ID (low 24 bits); a typed `OpaqueID` accessor on the codec layer, no per-type reinterpretation in the LSDB key |
| O-bit negotiation | Set `OptionO` in originated DD packets when opaque is enabled; record per-neighbour opaque capability from received DD Options; flood opaque LSAs only to opaque-capable neighbours; ignore the O-bit outside DD packets (§3.1) |
| Generic TLV carriage | Read-only TLV iterator and a TLV builder with 4-byte alignment and pad accounting, used by consumers; the framework does NOT interpret any TLV type |
| Consumer registry | `RegisterOpaqueConsumer(opaqueType, scope, OnOriginate, OnReceive)`; the OSPF plugin discovers consumers at startup; origination + reception hooks |
| Type-11 reachability | When a received Type-11 opaque LSA is delivered to a consumer, gate "usable" on the originating router's reachability (reuse the SPF/route-table ASBR-reachability already computed for Type 5); unreachable -> deliver as "present but unusable" (§5) |
| Verbatim re-flood | Unknown / unregistered opaque LSAs are stored and re-flooded byte-for-byte per scope, never dropped, never parsed |

### Out of scope (dependent specs; noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| TE LSA body + sub-TLVs (RFC 3630) | spec-ospf-ext-2 |
| Router-Information LSA body (RFC 7770) | spec-ospf-ext-3 |
| Extended-Link / Extended-Prefix bodies (RFC 7684) | spec-ospf-ext-4 |
| Grace-LSA body + GR helper (RFC 3623) | spec-ospf-ext-9 |
| Any SPF change | none -- opaque LSAs are never SPF vertices (RFC 5250 §3) |
| OSPFv3 opaque (RFC 5340 carries extensions as native LSAs, not opaque) | not applicable |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -->
- [ ] `docs/research/ospf-implementation-guide.md` §14 (FRR feature catalogue, "Opaque LSA Framework (RFC 5250)") -- the consumer landscape the framework must support
  -> Decision: model the carrier as a registration API exactly as the guide describes FRR ("a registration API lets extension modules hook into origination and reception"); consumers (TE/RI/SR/Grace) are separate specs that plug in, not part of this carrier
  -> Constraint: opaque LSAs do NOT feed SPF; the carrier touches the LSDB and flooding only, never `spf/`
- [ ] `plan/spec-ospf-0-umbrella.md` "Shared Contracts" (LSA inventory, LSA header layout, flooding eligibility) -- the contracts this framework extends
  -> Constraint: the LSDB key stays `(LS Type, Link State ID, Advertising Router)`; the Opaque Type/ID split lives ENTIRELY inside the 4-byte Link State ID, so opaque LSAs share the existing key type with no schema change
  -> Decision: opaque storage reuses the THREE existing stores -- link (`d.links`, scope 9), per-area (`d.areas`, scope 10), and a NEW AS-wide opaque store parallel to `asExternal` (scope 11); do not invent a fourth storage mechanism
- [ ] `ai/rules/plugin-self-containment.md` -- consumer modules must be self-contained
  -> Constraint: the carrier exposes the registry; each consumer (ext-2/3/4/9) registers its own Opaque Type, command, schema, and doctor checks; no opaque-type spelling (TE/RI/SR) appears in the carrier
- [ ] `ai/rules/buffer-first.md` -- TLV emit and opaque-body encode are buffer-first
  -> Constraint: the TLV builder writes into a caller-owned buffer via `WriteTo(buf, off) int`; the 4-byte alignment pad is written, never produced via slice concatenation; the TLV iterator returns views over the caller's bytes (zero-copy), no per-TLV allocation
- [ ] `ai/rules/no-sprintf-alloc.md` -- no `fmt`/`+` on the wire or hot path
  -> Constraint: any opaque-LSA rendering (CLI `show ip ospf database opaque`) uses `textbuf`/`AppendTo`

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5250.md` -- the framework spec
  -> Constraint: §3.1 scope enforcement -- Type 9 received on an interface other than the target interface MUST be discarded and not acknowledged; Type 10 whose area differs from the target interface's area MUST be discarded; Type 11 MUST NOT be flooded into stub/NSSA areas and a Type 11 received on a stub/NSSA interface MUST be discarded
  -> Constraint: §3.1 -- opaque LSAs are flooded ONLY to opaque-capable neighbours (O-bit set in their DD); the O-bit MUST be ignored when received in non-DD packets
  -> Constraint: §3 / App A.2 -- the Link State ID splits into Opaque Type (first 8 bits) + Opaque ID (remaining 24 bits); each Opaque Type owns its 24-bit ID namespace
  -> Constraint: §5 -- a received Type-11 opaque LSA is usable only if the originating ASBR/router is reachable; if unreachable, do nothing with it; discontinue using all opaque LSAs from an originator once it becomes unreachable
  -> Constraint: §9 -- Opaque Type 0-127 is standards-track (IETF review), 128-255 private use; the registry the carrier exposes must accept any 8-bit value but the four in-tree consumers use the assigned types (1 TE, 3 grace, 4 RI; 7/8 ext-prefix/link per RFC 7684)

**Key insights:**
- The carrier is a *registration framework*: new applications claim an Opaque Type and define a body, with NO change to core flooding except the scope gate and the O-bit gate.
- The three opaque scopes map exactly onto stores Ze already has: link (Type 9 -> `d.links`, like the OSPFv3 Link-LSA precedent), area (Type 10 -> `d.areas`), AS (Type 11 -> a new store mirroring `asExternal`). No new key type.
- The codec already retains opaque bodies verbatim (`packet.OpaqueLSA`), so verbatim re-flood is mostly already true; the gap is scope-correct *store routing* and *flood eligibility*, plus the O-bit neighbour gate.
- The O-bit (`types.OptionO`) and the LS-type classifiers (`IsOpaque()`) already exist; this spec wires them into DD negotiation and flooding, it does not redefine them.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/types/lstype.go` -- defines `LSTypeOpaqueLink` (9), `LSTypeOpaqueArea` (10), `LSTypeOpaqueAS` (11); `IsOpaque()` returns true for them; `Known()` = `InScope() || IsOpaque()`; `InScope()` is false for opaque types
  -> Constraint: `ASExternal()` returns false for Type 11 (opaque-AS has no scope bits), so today a Type-11 opaque LSA would route to the per-area store in `dbForLocked` -- WRONG for AS-wide flooding; this spec must add explicit opaque-scope routing
- [ ] `internal/plugins/ospf/types/options.go` -- `OptionO Options = 0x40` already defined; `Has`/`Set`/`Clear` available
  -> Constraint: the O-bit constant exists; it is NOT yet set in originated DDs nor checked in flooding. `expectedOptionsLocked` (iface.go) sets only E and N/P
- [ ] `internal/plugins/ospf/types/linkstateid.go` -- `LinkStateID [4]byte`, treated as an opaque 4-octet value; no Opaque Type / Opaque ID split helpers
  -> Constraint: the split must be added at the codec/framework layer (not the types leaf's identity semantics), so the LSDB key stays a plain 4-byte LinkStateID
- [ ] `internal/plugins/ospf/packet/lsa.go` + `lsa_opaque.go` -- `LSA.Opaque *OpaqueLSA{Type, Data}`; `DecodeLSA` retains raw bytes; `WriteTo` re-emits `RawBytes` verbatim when no typed body is set (opaque passthrough already works); `DecodeLSAHeader` rejects only `!lt.Known()`, so opaque types decode
  -> Constraint: the codec already round-trips opaque LSAs verbatim; do NOT rebuild it -- add only the Opaque Type/ID accessor and the generic TLV helpers
- [ ] `internal/plugins/ospf/lsdb/lsdb.go` -- `dbForLocked`/`dbForReadLocked` route by `key.Type.ASExternal()` to `asExternal`, else to the per-area store; `Install` rejects `isLinkLSAType` (Type 8 link-scope) so it cannot land in an area store; `LSDB` struct has `areas`, `asExternal`, `links`, `own`, `linkOwn`
  -> Constraint: store routing is the single chokepoint; add opaque-scope routing here (Type 9 -> link store, Type 10 -> area store, Type 11 -> new opaque-AS store) so all install paths inherit it
- [ ] `internal/plugins/ospf/lsdb/flooding.go` -- `ReceiveUpdate` runs §13 receive; branches on `isLinkLSAType` for the link store path; `floodExcept` floods area/AS LSAs out eligible interfaces gated by `eligibleInterface`; `shouldDropByArea` is the stub/NSSA receive filter (does not yet handle Type 11 vs stub/NSSA); flooding has NO opaque-capability neighbour gate
  -> Constraint: `eligibleInterface`/`shouldDropByArea`/`floodExcept` are the flood-scope chokepoints; add Type 9/10/11 scope rules and the O-bit neighbour gate here
- [ ] `internal/plugins/ospf/lsdb/link_scope.go` -- the per-interface link store (`d.links`), `OriginateLinkSelf`, `floodLink`, `installLink`, `ReleaseLink`; `isLinkLSAType` currently matches only Type 8 (OSPFv3 Link-LSA)
  -> Constraint: Type-9 opaque (link-local) reuses this entire link-store machinery; broaden the link-scope predicate so Type 9 routes through `installLink`/`floodLink`, mirroring the Type-8 precedent
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `OriginateSelf(area, key, body, enc)` + `SelfLSAEncoder` is the address-family-neutral self-LSA origination seam (sequencing, MinLSInterval, install, flood); `OriginateExternal` is the AS-wide example; `FlushStaleSelfLSAs(router, manage, keep)` withdraws stale self-LSAs
  -> Constraint: opaque origination reuses `OriginateSelf` (area/AS scope) and `OriginateLinkSelf` (link scope); the framework builds the opaque header + body via the consumer callback and passes a `SelfLSAEncoder`; no new sequencing/flooding code
- [ ] `internal/plugins/ospf/dispatcher.go` + `instance.go` -- the engine registers per-`PacketType` handlers; `ReceiveUpdate`/`ReceiveAck` route LS Update/Ack into the LSDB; `originateSelfLSAs` regenerates self-LSAs on topology change and on the periodic tick
  -> Constraint: opaque origination triggers hang off `originateSelfLSAs` (consumer callbacks invoked there); opaque reception delivery hangs off `ReceiveUpdate` (after install, before/with `notifyChange`); no new packet type, no new dispatcher handler
- [ ] `internal/plugins/ospf/neighbor/dd.go` + `neighbor.go` -- DD exchange records `n.Options = dd.Options`; outgoing DD sets `Options: cfg.Options`; `Snapshot`/neighbor struct carry `Options`
  -> Constraint: per-neighbour opaque capability = received DD Options has `OptionO`; the outgoing DD must set `OptionO` when opaque is enabled; this is the O-bit negotiation seam
- [ ] `internal/plugins/ospf/register.go` -- `registerOSPF()` registers the namespace, redistribution source, validators, and the `registry.Registration`; consumers do not exist yet
  -> Constraint: the consumer registry is a new in-process registry inside the OSPF plugin; `registerOSPF()` (or a sibling init) creates it; consumer specs call `RegisterOpaqueConsumer` from their own `init()`

**Behavior to preserve:**
- The OSPFv2 codec's verbatim opaque passthrough (`packet.LSA` re-emits `RawBytes`); the LSDB key triple; the link-store (Type 8) and AS-external (Type 5) routing; `OriginateSelf`/`OriginateLinkSelf`/`OriginateExternal` signatures and the `SelfLSAEncoder` contract.
- All existing OSPFv2 functional/interop tests (the framework must be additive; a router with no opaque consumer behaves exactly as today, except it now sets the O-bit and floods received opaque LSAs by scope instead of misrouting Type 11).
- `shouldDropByArea`/`eligibleInterface` behaviour for Types 1-7.

**Behavior to change:** (all RFC-5250-required, not discretionary)
- `dbForLocked`/`dbForReadLocked`: route Type 9 -> link store, Type 10 -> area store, Type 11 -> new opaque-AS store (today Type 11 wrongly routes to the per-area store).
- Flooding eligibility: add Type 9/10/11 scope rules and an opaque-capable-neighbour gate.
- Outgoing DD Options: set `OptionO` when opaque is enabled.
- Self-originated Hello/DD/Router-LSA unchanged except the DD O-bit.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Reception:** an LS Update arrives -> `dispatcher` -> instance LS Update handler -> `lsdb.ReceiveUpdate`. Each LSA whose `Header.Type.IsOpaque()` enters the opaque path.
- **Origination:** a registered consumer's state changes -> `originateSelfLSAs` invokes each consumer's `OnOriginate` -> the framework assigns a sequence and floods.
- **Negotiation:** DD packets carry the Options byte; the O-bit is read on receive (per-neighbour capability) and set on send (when opaque is enabled).

### Transformation Path
1. **Decode (existing):** `packet.DecodeLSA` returns an `LSA` with `Header.Type` in {9,10,11} and the body retained as `RawBytes`/`Body`; `VerifyChecksum` (Fletcher) validates it.
2. **Scope route (new):** the install path selects the store by opaque scope -- Type 9 via `installLink` (link store), Type 10 via `install` (per-area store), Type 11 via a new `installOpaqueAS` (the new AS-wide opaque store). `dbForLocked` gains an opaque-scope branch so every install path inherits the routing.
3. **Scope-correct receive filter (new):** before install, the §3.1 discard rules apply -- Type 9 received on a non-target interface discarded (no ack), Type 11 on a stub/NSSA interface discarded; Type 10 area-mismatch is already handled by the area binding on the receiving interface.
4. **Install + flood (mostly existing):** install runs the §13.1 freshness compare; on Newer, `floodExcept` (area/AS) or `floodLink` (link) re-floods verbatim, now gated by `eligibleInterface` opaque-scope rules AND the per-neighbour O-bit (only opaque-capable neighbours are queued).
5. **Consumer delivery (new):** after a Newer install, if a consumer is registered for the Opaque Type, the framework calls `OnReceive(opaqueID, body, scope, advertisingRouter, reachable)` where `reachable` is the §5 Type-11 originator reachability (true for Types 9/10). Unregistered opaque LSAs are still stored and flooded; they are simply not delivered.
6. **Origination (new, reuses existing seams):** `OnOriginate` returns `(opaqueID, scope, body, withdraw)`; the framework builds the opaque LSA header (LS type from scope, LS ID = OpaqueType<<24 | opaqueID), and calls `OriginateSelf` (area/AS) or `OriginateLinkSelf` (link) with a `SelfLSAEncoder` that emits the opaque body. The LSDB owns sequence/age/flood. Withdraw issues a MaxAge flush via the existing purge path.
7. **TLV helpers (new, used by consumers, not the carrier):** a TLV iterator yields `(type, value-slice)` over a body with 4-byte alignment; a TLV builder appends `(type, value)` with padding. The carrier never calls these; they are library helpers for ext-2/3/4/9.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire <-> opaque LSA | existing `packet.DecodeLSA`/`WriteTo` verbatim passthrough; new `OpaqueType()`/`OpaqueID()` accessor over the LS-ID | [ ] |
| LSA <-> store | new opaque-scope routing in `dbForLocked` + `installOpaqueAS`; link store reused for Type 9 | [ ] |
| LSDB <-> flooding | `floodExcept`/`floodLink` + opaque-scope `eligibleInterface` + per-neighbour O-bit gate | [ ] |
| Neighbor DD <-> capability | outgoing DD sets `OptionO`; received DD Options recorded as per-neighbour opaque capability | [ ] |
| Framework <-> consumer | `RegisterOpaqueConsumer`; `OnOriginate`/`OnReceive` callbacks; value-typed payloads (no cross-boundary pointers) | [ ] |
| Type-11 <-> reachability | `OnReceive.reachable` derived from the SPF route table's originator reachability (read-only) | [ ] |

### Integration Points
- `internal/plugins/ospf/types` -- `LSTypeOpaque*`, `IsOpaque()`, `OptionO` (consumed, not redefined).
- `internal/plugins/ospf/packet` -- `LSA`/`OpaqueLSA` verbatim passthrough; new `OpaqueType`/`OpaqueID` accessors + TLV helpers added here (codec layer).
- `internal/plugins/ospf/lsdb` -- store routing, flooding eligibility, the new opaque-AS store, the consumer-delivery hook; reuses `OriginateSelf`/`OriginateLinkSelf`/`installLink`/`floodExcept`.
- `internal/plugins/ospf/neighbor` -- DD O-bit set/read; per-neighbour opaque capability surfaced to flooding via the topology snapshot (`NeighborInfo`).
- `internal/plugins/ospf/iface` -- `expectedOptionsLocked` unchanged for Hello (the O-bit is a DD-only signal per §3.1); opaque enablement read from config.
- `internal/plugins/ospf` (engine) -- the consumer registry; origination trigger in `originateSelfLSAs`; reception delivery from `ReceiveUpdate`.
- `internal/plugins/ospf/spf` -- READ ONLY: originator reachability for the §5 Type-11 gate; opaque LSAs never become vertices.

### Architectural Verification
- [ ] No bypassed layers (opaque LSAs flow wire -> codec -> store-routing -> §13 install/flood -> consumer delivery, the same spine as Types 1-7)
- [ ] No unintended coupling (the carrier names no consumer; consumers depend on the carrier, not vice-versa)
- [ ] No duplicated functionality (reuses the three existing stores, `OriginateSelf`/`OriginateLinkSelf`, `floodExcept`/`floodLink`; adds only opaque-scope routing, the O-bit gate, the AS-opaque store, the registry, and the LS-ID/TLV helpers)
- [ ] Zero-copy preserved (opaque bodies retained as views; verbatim re-flood; TLV iterator returns slices; TLV builder + opaque encode are buffer-first)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The codec already round-trips opaque LSAs (9/10/11) verbatim and `VerifyChecksum` validates the Fletcher checksum unchanged | `packet/lsa.go` `WriteTo` re-emits `RawBytes`; `lsa_opaque.go`; `DecodeLSAHeader` accepts `Known()` types | the carrier must add codec work; scope creep | `TestOpaqueLSARoundTrip`, decode an FRR opaque-LSA capture | unvalidated |
| A-2 | The three opaque scopes map onto the existing link / per-area / (new) AS-wide stores with no new LSDB key type | `lsdb.go` `dbForLocked`, `link_scope.go` Type-8 precedent | a new key/store abstraction is needed; larger change | `TestOpaqueScopeRouting` (9->link, 10->area, 11->as-opaque) | unvalidated |
| A-3 | `OriginateSelf` (area/AS) and `OriginateLinkSelf` (link) can originate an opaque LSA via a `SelfLSAEncoder` without new sequencing/flooding code | `origination.go` `OriginateSelf`/`SelfLSAEncoder`; `OriginateExternal` precedent | a new origination path is needed | `TestOpaqueOriginateArea`, `TestOpaqueOriginateAS`, `TestOpaqueOriginateLink` | unvalidated |
| A-4 | Per-neighbour DD Options already carry through to a structure flooding can read (so the O-bit gate is a read, not new plumbing) | `neighbor/dd.go` `n.Options = dd.Options`; `NeighborInfo` in `flooding.go` | new neighbour->flooding plumbing for capability | `TestOpaqueFloodOnlyToOpaqueNeighbor` | unvalidated |
| A-5 | Type-11 originator reachability is available from the SPF route table without recomputation (reused from Type-5 handling) | umbrella "Route preference"; `spf/` external route computation | the §5 gate needs new reachability tracking | `TestOpaqueType11UnreachableOriginatorNotUsable` | unvalidated |
| A-6 | Setting `OptionO` in DD does not break adjacency with non-opaque peers (the O-bit is not part of the Hello E/N match) | `iface.go` `expectedOptionsLocked` checks only E and N/P; §3.1 "ignore O outside DD" | adjacencies fail against legacy peers; interop regression | `ospf-p2p-frr` still forms full adjacency; new `ospf-opaque-frr` interop | unvalidated |
| A-7 | `IsOpaque()` and `OptionO` as defined are sufficient; no types-leaf change needed | `lstype.go`, `options.go` | a types-leaf edit widens the blast radius | package builds; `IsOpaque()` covers 9/10/11 | unvalidated |
| A-8 | Opaque LSAs must never enter SPF; the existing SPF graph build ignores types outside 1-5/7 | umbrella ("opaque LSAs are not SPF vertices"); §3 | opaque bodies corrupt the topology graph | `TestOpaqueLSANotInSPFGraph` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Type 11 routed to the per-area store (today's behaviour) leaks an AS-wide LSA per-area or fails AS-wide flooding | a Type-11 opaque LSA appears in only one area's database | explicit opaque-scope routing in `dbForLocked` + `installOpaqueAS`; `TestOpaqueScopeRouting` asserts the store for each scope |
| R-2 | Opaque LSA flooded to a non-opaque neighbour (violates §3.1) breaks interop / wastes the neighbour's LSDB | FRR logs "unknown LSA from non-opaque peer"; ack storms | per-neighbour O-bit gate in `floodExcept`/`floodLink`; `TestOpaqueFloodOnlyToOpaqueNeighbor` |
| R-3 | Type 9 leaks beyond its link / Type 10 beyond its area / Type 11 into stub/NSSA | the LSA reaches a router it must not | scope rules in `eligibleInterface`/`shouldDropByArea`; per-scope flood-boundary tests |
| R-4 | LS-ID split byte order wrong (Opaque Type vs ID) -> consumers collide in the key namespace or claim the wrong type | a TE LSA delivered to the RI consumer | `OpaqueType()` = high byte, `OpaqueID()` = low 24 bits; `TestOpaqueLinkStateIDSplit` pins both directions against an RFC-shaped value |
| R-5 | TLV builder mis-pads to 4-byte alignment -> consumer bodies are off by the pad and cross-interop fails | self-round-trip passes, FRR rejects | the builder writes the pad explicitly; `TestOpaqueTLVAlignment` for value lengths 0..7; decode an FRR TLV body |
| R-6 | An unregistered opaque LSA is dropped instead of re-flooded (breaks transit for downstream consumers) | downstream router never sees the LSA when an intermediate has no consumer | store-and-flood is unconditional; consumer delivery is separate; `TestUnregisteredOpaqueReflooded` |
| R-7 | Withdraw (MaxAge flush) of a self opaque LSA does not reach the existing purge path -> stale opaque LSA lingers | a withdrawn opaque LSA stays in peers' databases until it ages out | route withdraw through `OnOriginate(withdraw=true)` -> the existing MaxAge purge in `OriginateSelf`/`OriginateLinkSelf`; `TestOpaqueWithdrawFlushes` |
| R-8 | Decoder panic on a malformed opaque body or TLV (untrusted input) | fuzz crash | reuse the bound-checked codec; the TLV iterator is bound-checked and never panics; extend the existing `packet` fuzz target with opaque bodies |
| R-9 | A consumer's `OnReceive`/`OnOriginate` panics and takes down the engine | a single bad consumer crashes OSPF | the framework recovers around consumer callbacks and counts a metric; `TestOpaqueConsumerPanicIsolated` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `RegisterOpaqueConsumer(type, scope, onOrig, onRecv)` from a consumer `init()` | -> | the OSPF opaque consumer registry stores the registration; the engine discovers it at startup | `TestOpaqueConsumerRegistered` (unit) + `test/ospf/ospf-opaque-register.ci` |
| A test consumer originates an opaque LSA (scope 10) | -> | `originateSelfLSAs` -> consumer `OnOriginate` -> framework builds header -> `OriginateSelf` -> install + flood | `test/ospf/ospf-opaque-originate.ci` |
| An LS Update carrying an opaque LSA for a registered type arrives | -> | `ReceiveUpdate` -> opaque path -> scope route + install -> `OnReceive` delivery | `test/ospf/ospf-opaque-receive.ci` |
| An LS Update carrying an opaque LSA for an UNregistered type arrives | -> | `ReceiveUpdate` -> store + verbatim re-flood, no delivery | `TestUnregisteredOpaqueReflooded` (unit) |
| A DD packet is sent while opaque is enabled | -> | outgoing DD sets `OptionO` | `TestDDSetsOpaqueBit` (unit) + observed in `ospf-opaque-frr` interop |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A consumer calls `RegisterOpaqueConsumer(opaqueType, scope, onOrig, onRecv)` | the registration is stored; duplicate registration of the same Opaque Type is rejected with an error; the engine invokes the callbacks at the correct times |
| AC-2 | A received Type-9 opaque LSA | stored in the link store for the receiving interface; a Type 9 received on an interface other than its target interface is discarded and NOT acknowledged (§3.1) |
| AC-3 | A received Type-10 opaque LSA | stored in the per-area store for the receiving interface's area; a Type 10 whose area differs from the receiving interface's area is discarded (§3.1) |
| AC-4 | A received Type-11 opaque LSA | stored in the AS-wide opaque store; flooded AS-wide; NOT flooded into stub/NSSA areas; a Type 11 received on a stub/NSSA interface is discarded (§3.1) |
| AC-5 | An opaque LSA to flood and a mix of opaque-capable and non-opaque neighbours | flooded only to neighbours whose last DD set `OptionO`; non-opaque neighbours are not queued (§3.1) |
| AC-6 | A DD packet originated while opaque is enabled | the Options field has `OptionO` set; a received `OptionO` in a non-DD packet is ignored (§3.1) |
| AC-7 | A Link State ID `0xAABBCCDD` on an opaque LSA | `OpaqueType()` == `0xAA`, `OpaqueID()` == `0x00BBCCDD`; encoding `(type, id)` reproduces the LS ID exactly (App A.2) |
| AC-8 | A consumer's `OnOriginate` returns `(opaqueID, scope, body, withdraw=false)` | the framework builds an opaque LSA (LS type from scope, LS ID from type+id), assigns a sequence, installs, and floods; identical re-origination floods nothing (idempotent) |
| AC-9 | A consumer's `OnOriginate` returns `withdraw=true` for a previously originated opaque LSA | the framework MaxAge-flushes the instance through the existing purge path so peers withdraw it |
| AC-10 | An opaque LSA arrives for a registered Opaque Type | after a Newer install the framework calls `OnReceive(opaqueID, body, scope, advRouter, reachable)`; an Equal/Older instance does not re-deliver |
| AC-11 | A Type-11 opaque LSA whose originating router is unreachable in the route table | delivered to the consumer with `reachable=false` (or not delivered as usable); when the originator later becomes unreachable, the carrier stops treating its opaque LSAs as usable (§5) |
| AC-12 | An opaque LSA for an UNregistered Opaque Type arrives | stored and re-flooded verbatim per scope; never delivered; never dropped; never crashes |
| AC-13 | A consumer builds a body with the TLV builder (values of length 0..7) | each TLV is 4-byte aligned with correct padding; the TLV iterator reads them back as `(type, value)` exactly; total length accounts for pad bytes |
| AC-14 | Any opaque LSA in any store | it never appears as a vertex in the SPF graph and never changes the route table directly (§3) |
| AC-15 | A consumer callback panics | the engine recovers, increments `ze_ospf_opaque_consumer_errors_total`, and continues processing other LSAs |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables OSPF opaque capability; a registered consumer (test stub) originates an area-scope opaque LSA | config -> engine -> `originateSelfLSAs` -> `OnOriginate` -> `OriginateSelf` -> install + flood; peer's `show ip ospf database opaque-area` shows it | `test/ospf/ospf-opaque-originate.ci` |
| 2 | Receives an opaque LSA from FRR for a registered type | wire -> dispatcher -> `ReceiveUpdate` -> scope route + install -> `OnReceive`; `show ip ospf database opaque-*` lists it; the consumer's hook fired | `test/ospf/ospf-opaque-receive.ci` + `ospf-opaque-frr` interop |
| 3 | Runs `ze` decode on opaque-LSA hex | CLI -> `packet.DecodeLSA` -> `OpaqueType()`/`OpaqueID()` rendered, body shown as TLVs/hex | `test/ospf/ospf-opaque-decode.ci` |
| 4 | Forms an adjacency with FRR where both set the O-bit, then both flood opaque LSAs | DD O-bit negotiation -> per-neighbour capability -> opaque flooding both ways | `ospf-opaque-frr` interop (full adjacency + opaque exchange) |
| 5 | Removes the test consumer (build without it) | `RegisterOpaqueConsumer` is gone; opaque LSAs still flood verbatim but are not delivered; OSPF otherwise unchanged | `TestUnregisteredOpaqueReflooded` + existing OSPF suite still green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOpaqueLinkStateIDSplit` | `internal/plugins/ospf/packet/lsa_opaque_test.go` | AC-7: Opaque Type = high byte, Opaque ID = low 24 bits, both directions | |
| `TestOpaqueLSARoundTrip` | `internal/plugins/ospf/packet/lsa_opaque_test.go` | A-1/AC-12: decode then re-encode a 9/10/11 opaque LSA byte-for-byte | |
| `TestOpaqueTLVAlignment` | `internal/plugins/ospf/packet/opaque_tlv_test.go` | AC-13: TLV builder pads to 4 bytes for value lengths 0..7; iterator reads back exactly | |
| `TestOpaqueTLVIteratorMalformed` | `internal/plugins/ospf/packet/opaque_tlv_test.go` | R-8: truncated/over-length TLV never panics, reports an error | |
| `TestOpaqueScopeRouting` | `internal/plugins/ospf/lsdb/opaque_scope_test.go` | AC-2/3/4, R-1: 9->link store, 10->area store, 11->AS-opaque store | |
| `TestOpaqueType9WrongInterfaceDiscarded` | `internal/plugins/ospf/lsdb/opaque_scope_test.go` | AC-2: Type 9 on a non-target interface discarded, not acked | |
| `TestOpaqueType11StubDiscarded` | `internal/plugins/ospf/lsdb/opaque_scope_test.go` | AC-4: Type 11 not flooded into stub/NSSA; discarded if received there | |
| `TestOpaqueFloodOnlyToOpaqueNeighbor` | `internal/plugins/ospf/lsdb/opaque_flood_test.go` | AC-5, R-2: flood gated by per-neighbour O-bit | |
| `TestOpaqueOriginateArea` / `TestOpaqueOriginateAS` / `TestOpaqueOriginateLink` | `internal/plugins/ospf/lsdb/opaque_originate_test.go` | AC-8, A-3: origination via `OriginateSelf`/`OriginateLinkSelf` per scope | |
| `TestOpaqueOriginateIdempotent` | `internal/plugins/ospf/lsdb/opaque_originate_test.go` | AC-8: identical body floods nothing | |
| `TestOpaqueWithdrawFlushes` | `internal/plugins/ospf/lsdb/opaque_originate_test.go` | AC-9, R-7: withdraw MaxAge-flushes via the purge path | |
| `TestOpaqueDeliveryOnNewerOnly` | `internal/plugins/ospf/lsdb/opaque_receive_test.go` | AC-10: `OnReceive` fires on Newer, not on Equal/Older | |
| `TestUnregisteredOpaqueReflooded` | `internal/plugins/ospf/lsdb/opaque_receive_test.go` | AC-12, R-6: unregistered type stored + reflooded, not delivered | |
| `TestOpaqueType11UnreachableOriginatorNotUsable` | `internal/plugins/ospf/opaque_reachability_test.go` | AC-11, A-5: §5 reachability gate | |
| `TestOpaqueConsumerRegistered` / `TestOpaqueConsumerDuplicateRejected` | `internal/plugins/ospf/opaque_registry_test.go` | AC-1: registry stores; duplicate Opaque Type rejected | |
| `TestOpaqueConsumerPanicIsolated` | `internal/plugins/ospf/opaque_registry_test.go` | AC-15, R-9: callback panic recovered, metric incremented | |
| `TestDDSetsOpaqueBit` / `TestOpaqueBitIgnoredOutsideDD` | `internal/plugins/ospf/neighbor/dd_opaque_test.go` | AC-6, A-6: O-bit set in DD, ignored elsewhere | |
| `TestOpaqueLSANotInSPFGraph` | `internal/plugins/ospf/spf/opaque_exclusion_test.go` | AC-14, A-8: opaque LSAs never become SPF vertices | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Opaque Type (high 8 bits of LS ID) | 0-255 | 255 | N/A | N/A (1 byte) |
| Opaque ID (low 24 bits of LS ID) | 0-16777215 | 16777215 | N/A | N/A (masked to 24 bits) |
| LS type (opaque scope) | {9,10,11} | 11 | N/A | a non-opaque type is rejected by the opaque path |
| TLV value length (alignment) | 0-65531 | any | N/A | a length pushing past the LSA Length is an iterator error |
| TLV padding | 0-3 bytes | 3 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-opaque-register` | `test/ospf/ospf-opaque-register.ci` | a registered consumer appears; opaque enabled shows in `show ip ospf` | |
| `ospf-opaque-originate` | `test/ospf/ospf-opaque-originate.ci` | a stub consumer originates an opaque LSA; it appears in `show ip ospf database opaque-area` | |
| `ospf-opaque-receive` | `test/ospf/ospf-opaque-receive.ci` | a received opaque LSA is stored, listed, and delivered to the consumer | |
| `ospf-opaque-decode` | `test/ospf/ospf-opaque-decode.ci` | `ze` decode of opaque hex shows Opaque Type/ID + TLVs | |
| `ospf-opaque-scope` | `test/ospf/ospf-opaque-scope.ci` | Type 9/10/11 honour their flood boundaries (link/area/AS, not into stub) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-opaque-frr` | `test/interop/scenarios/ospf-opaque-frr/` | FRR `ospfd` (opaque on; a TE or RI opaque LSA originated by FRR) | Ze sets the O-bit, forms full adjacency, stores and re-floods FRR's opaque LSA per scope, and FRR accepts Ze's originated opaque LSA; non-opaque adjacency unaffected | |

> Interop is required: this changes wire behaviour (DD O-bit, opaque flooding). The
> raw-IP / multicast paths are Linux-only and run as QEMU integration tests
> (`ai/rules/qemu-testing.md`), consistent with the rest of the OSPF interop set.

### Future (if deferring any tests)
- None. All ACs are covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/lsdb/lsdb.go` -- opaque-scope routing in `dbForLocked`/`dbForReadLocked`; the new AS-wide opaque store field + init/snapshot/aging wiring
- `internal/plugins/ospf/lsdb/link_scope.go` -- broaden the link-scope predicate so Type 9 routes through `installLink`/`floodLink` (mirroring the Type-8 precedent)
- `internal/plugins/ospf/lsdb/flooding.go` -- opaque-scope rules in `eligibleInterface`/`shouldDropByArea`; the per-neighbour O-bit gate in `floodExcept`; opaque-scope branch in `ReceiveUpdate`; the consumer-delivery hook after a Newer install
- `internal/plugins/ospf/lsdb/origination.go` -- the opaque self-origination entry point (`OriginateOpaque`) reusing `OriginateSelf`/`OriginateLinkSelf`; withdraw via the existing purge path
- `internal/plugins/ospf/lsdb/aging.go` -- include the AS-opaque store in MaxAge aging/refresh
- `internal/plugins/ospf/packet/lsa.go` -- `OpaqueType()`/`OpaqueID()` accessors on the LSA / LS-ID
- `internal/plugins/ospf/neighbor/dd.go` -- set `OptionO` in outgoing DD when opaque is enabled; record per-neighbour opaque capability from received DD
- `internal/plugins/ospf/neighbor/neighbor.go` -- carry the per-neighbour opaque-capable flag into `Snapshot`
- `internal/plugins/ospf/instance.go` -- surface neighbour opaque capability into `NeighborInfo`; invoke consumer `OnOriginate` from `originateSelfLSAs`; deliver `OnReceive` from the LSDB hook; the Type-11 reachability lookup
- `internal/plugins/ospf/lsdb/flooding.go` `NeighborInfo` -- add `OpaqueCapable bool`
- `internal/plugins/ospf/register.go` -- create the opaque consumer registry; wire discovery into the engine
- `internal/plugins/ospf/cmd_show.go` + `internal/plugins/ospf/show_database.go` -- `show ip ospf database opaque-link|opaque-area|opaque-as`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- a top-level `opaque` leaf (enable/disable opaque capability); default off until a consumer is present
- `internal/plugins/ospf/config.go` -- resolve the `opaque` leaf into the engine config
- `internal/plugins/ospf/doctor.go` -- (only if a runtime dependency is added; none expected -- no new socket/port)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `opaque` enable leaf; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `opaque` is `type boolean` (native); no custom validator needed |
| YANG custom validators | [ ] no | native boolean suffices |
| CLI commands/flags | [ ] yes | `show ip ospf database opaque-link|opaque-area|opaque-as` in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ip ospf database opaque-*` |
| Editor autocomplete | [ ] yes | automatic for the YANG boolean + the new show subcommands |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-opaque-*.ci` |
| Pipe completeness | [ ] yes | `show ip ospf database opaque-*` routes through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | `opaque` is operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary/cert; reuses the existing OSPF raw socket |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_opaque_lsas` | gauge | `scope` (link/area/as), `opaque_type` |
| `ze_ospf_opaque_originations_total` | counter | `opaque_type` |
| `ze_ospf_opaque_received_total` | counter | `opaque_type`, `registered` |
| `ze_ospf_opaque_consumer_errors_total` | counter | `opaque_type` |
| `ze_ospf_opaque_capable_neighbors` | gauge | `interface` |

> These extend the umbrella's canonical OSPF metric set; they use the
> `ze_ospf_opaque_*` prefix and are registered by this spec's owner code, not by
> ospf-13. The umbrella "Metrics" table must gain these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF opaque-LSA framework |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `opaque` leaf |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ip ospf database opaque-*` |
| 4 | API/RPC added/changed? | [ ] no | show RPCs live in the central `ze-show` namespace; document under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains an opaque consumer registry |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- opaque section (carrier only) |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` (or equivalent) -- opaque LSA + DD O-bit |
| 8 | Plugin SDK/protocol changed? | [ ] yes | document `RegisterOpaqueConsumer` for ext-2/3/4/9 authors |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5250.md` -- flip the framework checklist items to implemented |
| 10 | Test infrastructure changed? | [ ] yes (interop scenario added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF opaque framework parity with FRR |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- opaque store routing + registry |
| 13 | Route metadata keys added/changed? | [ ] no | opaque LSAs do not install routes |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the five `ze_ospf_opaque_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF files |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF config/CLI examples against the new `opaque` leaf |

## Files to Create
- `internal/plugins/ospf/opaque_registry.go` -- the `RegisterOpaqueConsumer` API, the registry, the consumer-callback recover wrapper
- `internal/plugins/ospf/opaque.go` -- the engine-side glue: origination trigger, reception delivery, Type-11 reachability lookup
- `internal/plugins/ospf/packet/opaque_tlv.go` -- the generic 4-byte-aligned TLV iterator + builder (no type interpretation)
- `internal/plugins/ospf/lsdb/opaque_as.go` -- the AS-wide opaque store + `installOpaqueAS` (mirrors `link_scope.go`)
- `internal/plugins/ospf/packet/opaque_tlv_test.go`, `internal/plugins/ospf/packet/lsa_opaque_test.go`
- `internal/plugins/ospf/lsdb/opaque_scope_test.go`, `opaque_flood_test.go`, `opaque_originate_test.go`, `opaque_receive_test.go`
- `internal/plugins/ospf/opaque_registry_test.go`, `internal/plugins/ospf/opaque_reachability_test.go`
- `internal/plugins/ospf/neighbor/dd_opaque_test.go`, `internal/plugins/ospf/spf/opaque_exclusion_test.go`
- `test/ospf/ospf-opaque-register.ci`, `ospf-opaque-originate.ci`, `ospf-opaque-receive.ci`, `ospf-opaque-decode.ci`, `ospf-opaque-scope.ci`
- `test/interop/scenarios/ospf-opaque-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the codec opaque passthrough + the three stores exist |
| 3. Wiring phase | Wiring Test table -- registry + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- the consumer registry + failing wiring tests
   - Tests: `TestOpaqueConsumerRegistered`, `TestOpaqueConsumerDuplicateRejected`, `test/ospf/ospf-opaque-register.ci`
   - Files: `opaque_registry.go` (registry + `RegisterOpaqueConsumer`), `register.go` (create the registry, discover at startup), a test-only stub consumer
   - Verify: a consumer registers and the engine discovers it; origination/reception are stubs so the deeper tests still fail
2. **Phase: LS-ID split + TLV helpers** -- the codec-layer carrier primitives
   - Tests: `TestOpaqueLinkStateIDSplit`, `TestOpaqueLSARoundTrip`, `TestOpaqueTLVAlignment`, `TestOpaqueTLVIteratorMalformed`
   - Files: `packet/lsa.go` (`OpaqueType`/`OpaqueID`), `packet/opaque_tlv.go`
   - Verify: split round-trips; TLV builder/iterator align to 4 bytes and never panic
3. **Phase: Scope-correct storage + receive filter** -- route by scope, discard per §3.1
   - Tests: `TestOpaqueScopeRouting`, `TestOpaqueType9WrongInterfaceDiscarded`, `TestOpaqueType11StubDiscarded`, `TestUnregisteredOpaqueReflooded`
   - Files: `lsdb/lsdb.go` (dbForLocked branch + AS-opaque store), `lsdb/opaque_as.go`, `lsdb/link_scope.go` (Type-9 predicate), `lsdb/flooding.go` (`ReceiveUpdate` opaque branch + `shouldDropByArea`)
   - Verify: each scope lands in the right store; §3.1 discards hold; unregistered LSAs still store + reflood
4. **Phase: Scope-correct flooding + O-bit gate** -- flood boundaries and opaque-capable neighbours
   - Tests: `TestOpaqueFloodOnlyToOpaqueNeighbor`, `ospf-opaque-scope.ci`, `TestDDSetsOpaqueBit`, `TestOpaqueBitIgnoredOutsideDD`
   - Files: `lsdb/flooding.go` (`eligibleInterface` + O-bit gate, `NeighborInfo.OpaqueCapable`), `neighbor/dd.go`, `neighbor/neighbor.go`, `instance.go` (surface capability into `NeighborInfo`)
   - Verify: opaque LSAs honour scope and only reach opaque-capable neighbours; DD sets the O-bit
5. **Phase: Origination + reception delivery** -- the consumer callbacks
   - Tests: `TestOpaqueOriginate*`, `TestOpaqueOriginateIdempotent`, `TestOpaqueWithdrawFlushes`, `TestOpaqueDeliveryOnNewerOnly`, `TestOpaqueConsumerPanicIsolated`, `ospf-opaque-originate.ci`, `ospf-opaque-receive.ci`
   - Files: `opaque.go` (origination trigger from `originateSelfLSAs`, reception delivery hook), `lsdb/origination.go` (`OriginateOpaque`), the recover wrapper in `opaque_registry.go`
   - Verify: origination floods, idempotent, withdraw flushes; delivery on Newer only; a panicking consumer is isolated
6. **Phase: Type-11 reachability gate** -- §5
   - Tests: `TestOpaqueType11UnreachableOriginatorNotUsable`, `TestOpaqueLSANotInSPFGraph`
   - Files: `opaque_reachability.go` (or `opaque.go`), `spf/` read-only reachability lookup
   - Verify: Type-11 from an unreachable originator is delivered unusable; opaque LSAs never enter SPF
7. **Phase: CLI + config + metrics** -- user surface
   - Tests: `ospf-opaque-decode.ci`, the register/originate/receive `.ci`
   - Files: `cmd_show.go`, `show_database.go`, `yang/ze-ospf-conf.yang`, `yang/ze-ospf-cmd.yang`, `config.go`, metric registration
   - Verify: `show ip ospf database opaque-*`, the `opaque` leaf, the five metric series
8. **Functional tests** -> the five `.ci` cover the user-visible behaviour
9. **RFC refs** -> add `// RFC 5250 Section X` comments on the scope, O-bit, LS-ID-split, and §5 enforcement code
10. **Interop** -> `ospf-opaque-frr` QEMU scenario
11. **Full verification** -> `make ze-verify`
12. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; carrier parity with FRR's RFC 5250 carrier (scope flooding + registration API), consumers excluded by design |
| Correctness | scope routing exact (9 link / 10 area / 11 AS); §3.1 discards; O-bit gate; LS-ID split byte order; TLV 4-byte alignment; §5 reachability |
| Naming | `ze_ospf_opaque_*` metrics; YANG `opaque` kebab-case; `OpaqueType`/`OpaqueID` |
| Data flow | opaque touches LSDB + flooding only; SPF read-only for §5; no consumer name in the carrier |
| CLI grammar | `show ip ospf database opaque-*` action-before-identifier |
| Doctor checks | none added (no new runtime dependency) -- confirm |
| YANG validation | the `opaque` leaf is a native boolean |
| Prometheus counters | the five series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | the carrier names no consumer; removing a consumer removes its registration cleanly |
| Rule: buffer-first | TLV builder + opaque encode write into caller buffers; iterator zero-copy |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `RegisterOpaqueConsumer` exists and is called by a (test stub) consumer | `grep -rn RegisterOpaqueConsumer internal/plugins/ospf` |
| Opaque-scope routing | `go test ./internal/plugins/ospf/lsdb -run TestOpaqueScopeRouting` |
| O-bit DD negotiation | `go test ./internal/plugins/ospf/neighbor -run TestDDSetsOpaqueBit` |
| LS-ID split + TLV helpers | `go test ./internal/plugins/ospf/packet -run 'Opaque'` |
| Five metric series registered | `grep -rn 'ze_ospf_opaque_' internal/plugins/ospf` |
| Interop scenario present | `ls test/interop/scenarios/ospf-opaque-frr/` |
| Functional tests present | `ls test/ospf/ospf-opaque-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | opaque body + TLV iteration bound-checked; the existing fuzz target extended with opaque bodies; no slice-out-of-range on malformed input |
| Resource exhaustion | opaque stores share the existing `MaxLSAsPerArea` / a cap on the AS-opaque store; a flood of opaque LSAs cannot grow memory unbounded |
| Consumer isolation | `OnReceive`/`OnOriginate` panics recovered; a bad consumer cannot crash OSPF or wedge the LSDB lock |
| Trust boundary | opaque LSAs are flooded only to opaque-capable neighbours; received opaque LSAs rely on the existing OSPF authentication (no new auth surface) |
| Error leakage | consumer-callback errors are counted, not surfaced to peers |

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
The opaque framework is a *routing* problem, not a *codec* problem: the OSPFv2
codec already carries opaque bodies verbatim, and the three opaque scopes map
onto stores Ze already has. The work is wiring the scope gate, the O-bit gate,
and a consumer registry into existing chokepoints (`dbForLocked`,
`eligibleInterface`, `floodExcept`, DD Options, `originateSelfLSAs`,
`ReceiveUpdate`), keeping the carrier ignorant of any consumer.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reuse the link / per-area / new AS-opaque stores | a single dedicated opaque store keyed by scope | The three scopes already match three existing stores (Type-8 link precedent, Type-5 AS precedent); reuse keeps flooding/aging/snapshot uniform |
| Registry inside the OSPF plugin, not a new component | a top-level opaque component | module-tiers: opaque has no feature depending on it; it is an OSPF-internal extension point; consumers stay self-contained |
| Carrier interprets NO TLV | bake TE/RI awareness into the carrier | plugin-self-containment + RFC 5250's framework intent: consumers own their bodies; the carrier only floods |
| O-bit is a DD-only signal (not in Hello E/N match) | gate adjacency on the O-bit | §3.1 says ignore the O-bit outside DD; gating Hello would break adjacency with legacy peers |
| Type-11 reachability read from the existing route table | a separate opaque reachability tracker | §5 explicitly reuses the Type-5 ASBR reachability; no new tracking |

## Known Limitations
- No opaque consumer ships with this spec; without a consumer the engine floods opaque LSAs but interprets none (by design -- consumers are ext-2/3/4/9).
- Opaque LSAs never participate in SPF (RFC 5250 §3); any future use of opaque data for path computation is a consumer concern.
- OSPFv3 opaque is not applicable (RFC 5340 carries extensions as native LSAs).

## RFC Documentation
Add `// RFC 5250 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §3.1 scope discards (Type 9 wrong interface, Type 10 area mismatch, Type 11 stub/NSSA)
- §3.1 flood only to opaque-capable (O-bit) neighbours; ignore O outside DD
- §3 / App A.2 the Opaque Type / Opaque ID split of the Link State ID
- §5 Type-11 originator-reachability gate

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
| Scope-correct flooding for Type 9/10/11 | functional + interop | `ospf-opaque-scope.ci`, `ospf-opaque-frr` |
| Registration API for consumers | unit + functional | `TestOpaqueConsumerRegistered`, `ospf-opaque-register.ci` |
| O-bit negotiation gates opaque flooding | unit + interop | `TestOpaqueFloodOnlyToOpaqueNeighbor`, `ospf-opaque-frr` |
| Verbatim re-flood of unregistered opaque LSAs | unit | `TestUnregisteredOpaqueReflooded` |
| §5 Type-11 reachability gate | unit | `TestOpaqueType11UnreachableOriginatorNotUsable` |

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
- [ ] AC-1..AC-15 all demonstrated
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
- [ ] RFC 5250 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (4 consumers planned -- ext-2/3/4/9 -- justify the registry)
- [ ] No speculative features (only the carrier; no consumer bodies)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (carrier names no consumer)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-opaque-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-1-opaque-framework.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-1-opaque-framework.md`
