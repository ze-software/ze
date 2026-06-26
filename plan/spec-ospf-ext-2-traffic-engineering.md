# Spec: ospf-ext-2 -- OSPFv2 Traffic Engineering LSAs (RFC 3630, RFC 5392)

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
3. `rfc/short/rfc3630.md` -- TE LSA = Type 10 Opaque, Opaque type 1; Router Address TLV (1) + Link TLV (2); sub-TLVs 1-9 (§2.4, §2.5); 4-octet TLV alignment (§2.3.2); IEEE-float bandwidth in bytes/sec (§2.4.2)
4. `rfc/short/rfc5392.md` -- Inter-AS-TE-v2 = Opaque type 6 (Type 10 or 11 by policy); new Link sub-TLVs Remote AS Number (21), IPv4 Remote ASBR ID (22), IPv6 Remote ASBR ID (24); Link ID sub-TLV MUST NOT be used (§3.2.1)
5. `rfc/short/rfc5250.md` -- the Opaque carrier this consumes: §3.1 scope rules, §3 / App A.2 Opaque Type / Opaque ID split, §5 Type-11 reachability gate
6. `plan/spec-ospf-ext-1-opaque-framework.md` -- the ext-1 carrier: `RegisterOpaqueConsumer(opaqueType, scope, OnOriginate, OnReceive)`, the generic 4-byte-aligned TLV iterator + builder (`packet/opaque_tlv.go`), the `OpaqueType()`/`OpaqueID()` LS-ID split, scope-correct storage/flooding, the O-bit gate
7. `docs/research/ospf-implementation-guide.md` ~1522-1525 ("Traffic Engineering (RFC 3630, RFC 5392)") -- a TE LSA is an opaque-LSA consumer feeding a TED; first-pass implementations may skip CSPF / TE-aware path computation
8. `internal/plugins/ospf/config.go` -- `interfaceConfig` struct (where TE per-link attributes attach) and `ospfConfig` (where a TE Router Address attaches)
9. `internal/plugins/ospf/iface/iface.go` -- `InterfaceInfo` / `Neighbor` (the live Link ID, local/remote address, neighbour Router ID that origination reads)
10. `internal/plugins/rsvpte/admission.go` -- the existing `interfaceBandwidth` (`MaxBandwidth`/`MaxReservable`/`ReservedBandwidth` as `float64`) and `Available()`, the precedent the TED mirrors and a future TED consumer

## Task

Add OSPFv2 Traffic Engineering LSA support to the native OSPFv2 plugin at
`internal/plugins/ospf/` as a **consumer of the ext-1 Opaque-LSA framework**.
ext-1 delivers the generic carrier (scope-correct flooding, the Opaque Type /
Opaque ID split of the Link State ID, the O-bit gate, the 4-byte-aligned TLV
iterator/builder, and `RegisterOpaqueConsumer`). This spec registers Opaque
type 1 (RFC 3630) and Opaque type 6 (RFC 5392 Inter-AS-TE-v2) as consumers and
implements the TE LSA *body*: the Router Address TLV, the Link TLV, and the
link sub-TLVs (link type/id, local + remote interface address, TE metric,
max / max-reservable / unreserved bandwidth, administrative group), plus the
RFC 5392 inter-AS sub-TLVs (Remote AS Number, IPv4/IPv6 Remote ASBR ID).

The spec covers three movements: (a) **origination** -- from configured TE link
attributes (a per-interface `traffic-engineering` block) and a TE Router Address,
the consumer's `OnOriginate` builds one Router-Address TE LSA plus one Link TE
LSA per TE-enabled link, with distinct Opaque IDs (Instance), and hands them to
the carrier which sequences, installs, and floods them; (b) **reception/parse**
-- the consumer's `OnReceive` parses TE LSA bodies arriving from peers into a
**Traffic Engineering Database (TED)** keyed by link (advertising router + Link
ID + local address), so the local node holds the area's TE topology; (c)
**query** -- a `show ospf database opaque-area` TE decode plus a dedicated
`show ospf te-database` view and TED metrics expose the stored TE topology.

Flooding is **not** re-implemented: TE LSAs are area-scope (Type 10) or, for
RFC 5392 inter-AS-TE by policy, AS-scope (Type 11) opaque LSAs, flooded entirely
by the ext-1 carrier. The TED is a passive store; it never triggers SPF (RFC 3630
§1, "No SPF or other route calculations are necessary").

### In scope (this spec)

| Item | Detail |
|------|--------|
| Opaque type 1 consumer registration (RFC 3630) | `RegisterOpaqueConsumer(1, scopeArea, OnOriginate, OnReceive)` from the OSPF plugin; the carrier owns flooding/sequencing |
| Opaque type 6 consumer registration (RFC 5392) | Inter-AS-TE-v2; scope Type 10 or Type 11 chosen by per-link config policy (§3.1.1) |
| TE LSA body codec | Router Address TLV (top-level type 1), Link TLV (top-level type 2), sub-TLVs 1-9 (RFC 3630 §2.5) + sub-TLVs 21/22/24 (RFC 5392 §3.3); built on the ext-1 TLV builder/iterator |
| IEEE-float bandwidth | encode/decode 32-bit IEEE-754 single-precision bytes/sec for sub-TLVs 6/7/8 (RFC 3630 §2.4.2); store as `float64` in the TED (mirrors `rsvpte` admission) |
| Origination from config | per-interface `traffic-engineering` block (enable, te-metric, max-bandwidth, max-reservable-bandwidth, admin-group) + a TE `router-address`; one Router-Address LSA + one Link LSA per TE link, distinct Instance |
| Inter-AS origination | a `traffic-engineering inter-as` link block (remote-as, remote-asbr-id, scope area\|as); proxies one direction per RFC 5392 §4 |
| Reception into TED | `OnReceive` parses the body and upserts a TED entry keyed by (advertising-router, Link ID, local-addr); on withdraw/MaxAge the entry is removed |
| §5 reachability gate (Type 11 only) | the carrier passes `reachable`; an inter-AS-TE-v2 Type-11 LSA from an unreachable originator is held "present but unusable" in the TED |
| CLI + metrics | `show ospf te-database` (router addresses + links + attributes), TE decode under `show ospf database opaque-area`, `ze_ospf_te_*` metric series |
| TED query API | a read-only `Snapshot`/lookup the future `rsvpte` TED consumer can call (value-typed, no cross-boundary pointers) |

### Out of scope (noted so it is not silently assumed done)

| Item | Where / why |
|------|-------------|
| CSPF / constraint-based path computation | future work; the TED is a passive store (RFC 3630 §1; guide ~1524 "no TE-aware path computation") |
| RSVP-TE signalling integration | `internal/plugins/rsvpte` is a **future TED consumer**, not wired here; this spec exposes the read-only TED query it will call |
| OSPFv3 TE (RFC 5329) + Inter-AS-TE-v3 (RFC 5392 §3.1.2) | OSPFv3 carries TE as native LSAs, not opaque; there is no separate `ospfv3` plugin (v3 is a sub-config of `ospf`); v3 TE is a separate future spec |
| SRLG sub-TLV (RFC 4203 / 5392 GMPLS) | not in the RFC 3630 base set; future GMPLS spec |
| Unnumbered links, multi-access reservation state | explicitly out of scope of RFC 3630 (§1.2) |
| Re-implementing flooding / sequencing / the LS-ID split / the TLV iterator | owned by ext-1; this spec only registers consumers and codes bodies |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/research/ospf-implementation-guide.md` ~1518-1525 ("Opaque LSA Framework", "Traffic Engineering (RFC 3630, RFC 5392)") -- the TE LSA is an opaque-LSA *consumer* feeding a TED; first-pass implementations may skip CSPF/TE path computation
  → Decision: build TE strictly as an ext-1 opaque consumer; do NOT add a new flooding path, a new LSDB store, or any SPF change; the TED is a separate passive structure fed by `OnReceive`
  → Constraint: bandwidth/admin-group/TE-metric are the carried attributes; no MPLS-TE and no TE-aware route computation in this spec
- [ ] `plan/spec-ospf-ext-1-opaque-framework.md` (Task, "In scope", Data Flow, Wiring Test) -- the carrier contract this spec depends on
  → Constraint: the carrier owns scope-correct flooding, sequencing, the Opaque Type/ID split, the O-bit gate, and the 4-byte-aligned TLV iterator/builder; this spec registers a consumer and supplies only `OnOriginate`/`OnReceive` and the body codec
  → Decision: the consumer registers Opaque type 1 (RFC 3630) and type 6 (RFC 5392) with `scope = area` (type 1, always Type 10) and `scope = area|as` (type 6, by policy); the carrier rejects duplicate Opaque-Type registration, so the two registrations are distinct
  → Constraint: `OnOriginate` returns `(opaqueID, scope, body, withdraw)`; `OnReceive(opaqueID, body, scope, advRouter, reachable)`; payloads are value-typed (no cross-boundary pointers, `ai/rules/plugin-design.md`)
- [ ] `ai/rules/plugin-self-containment.md` -- TE is a self-contained opaque consumer
  → Constraint: all TE spelling (TLV types, TED, `traffic-engineering` YANG, `te-database` show, `ze_ospf_te_*` metrics) lives in TE-owned files; removing the TE consumer removes the registration and all TE behaviour, leaving the carrier and base OSPF intact; no TE type appears in the ext-1 carrier
- [ ] `ai/rules/buffer-first.md` -- TE body encode + TED render are buffer-first
  → Constraint: the TE body is built with the ext-1 TLV builder (`WriteTo(buf, off) int`); the IEEE-float bandwidth encode writes the 4 bytes directly; the TLV iterator returns views over caller bytes (zero-copy); no slice concatenation for the body
- [ ] `ai/rules/no-sprintf-alloc.md` -- no `fmt`/`+` on wire or render hot path
  → Constraint: TE LSA rendering (`show ospf te-database`, opaque-area TE decode) uses `textbuf`/`AppendTo`; bandwidth-to-string formatting uses the existing numeric append helpers
- [ ] `ai/rules/data-flow-tracing.md` -- trace config → origination and wire → TED
  → Constraint: every TED entry must be traceable to a received TE LSA; every originated TE LSA must be traceable to a config `traffic-engineering` block; no silent synthesis

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc3630.md` -- the TE LSA body
  → Constraint: §2.2 the TE LSA uses Opaque type 1; the LS Type byte is 10 (area-local); the 24-bit Instance has no topological significance (up to 16777216 TE LSAs)
  → Constraint: §2.3.2 every TLV is padded to 4-octet alignment; padding is NOT counted in Length; a 3-octet value has Length=3 but occupies 8 octets total -- walking by raw Length without re-aligning corrupts the parse
  → Constraint: §2.4 an LSA contains exactly one top-level TLV (Router Address OR Link); §2.4.1 the Router Address TLV (type 1, length 4) appears in exactly one TE LSA per router; §2.4.2 only one Link TLV per LSA
  → Constraint: §2.4.2 / §2.5 within the Link TLV, Link Type (sub-TLV 1) and Link ID (sub-TLV 2) are mandatory exactly once; all other defined sub-TLVs at most once; sub-TLVs have no ordering requirement; unrecognized sub-TLVs ignored
  → Constraint: §2.4.2 / §2.5.6-8 bandwidth sub-TLVs (6 Max, 7 Max-Reservable, 8 Unreserved×8) are 32-bit IEEE-754 single-precision **bytes/sec**, not bits/sec and not integers; §2.5.8 the eight Unreserved values run priority 0 first → priority 7 last, each ≤ Max-Reservable
  → Constraint: §2.5.9 Administrative Group is a 32-bit mask, LSB = group 0, MSB = group 31
  → Constraint: §3 originate on TE content change and on refresh; rate-limit to at most one origination per `MinLSInterval` [RFC 2328]; no SPF on receipt
- [ ] `rfc/short/rfc5392.md` -- inter-AS TE sub-TLVs and carrier
  → Constraint: §3.1.1 Inter-AS-TE-v2 uses Opaque type 6; carried as Type 10 (area) or Type 11 (AS) per AS-wide policy; configuration control of the Type-10-vs-11 choice SHOULD be provided
  → Constraint: §3.2.1 the only top-level TLV is the existing Link TLV (type 2); the Link ID sub-TLV (RFC 3630 sub-TLV 2) MUST NOT be used in an Inter-AS-TE-v2 Link TLV; the remote end is identified by the new sub-TLVs instead
  → Constraint: §3.3.1 Remote AS Number sub-TLV = type 21, length 4, REQUIRED in any inter-AS Link TLV; a 2-byte ASN is left-padded with two zero octets into the 4-octet field
  → Constraint: §3.3.2 IPv4 Remote ASBR ID = type 22, length 4 (MUST be present in OSPFv2 if the remote ASBR has an IPv4 address); §3.3.3 IPv6 Remote ASBR ID = type 24 (NOT 23 — §3.2.1's "23" is an editorial slip; §6.2 assigns 24), length 16
  → Constraint: §4 no Hello and no OSPF adjacency on the inter-AS link; the ASBR proxies the link into its own AS; re-advertise on TE change with the same RFC 3630 rate-limit precautions
- [ ] `rfc/short/rfc5250.md` -- the carrier semantics this consumer relies on (delivered by ext-1)
  → Constraint: §3.1 Type 10 TE LSAs never leave their area; Type 11 inter-AS-TE LSAs are not flooded into stub/NSSA; §5 a Type-11 LSA from an unreachable originator is unusable — the carrier passes `reachable=false` and the TED holds it as "present but unusable"

**Key insights:**
- TE is a *body codec + a TED*, not a protocol change: ext-1 already floods Type 10/11 opaque LSAs by scope, sets the O-bit, splits the LS-ID, and offers a 4-byte-aligned TLV iterator/builder. This spec registers Opaque type 1 and type 6 and codes the Router-Address / Link / sub-TLV bodies.
- The two load-bearing wire details are the **4-octet TLV alignment with padding excluded from Length** (RFC 3630 §2.3.2) and the **IEEE-float bandwidth in bytes/sec** (§2.4.2); both have existing precedents in tree (the ext-1 TLV builder for alignment, `rsvpte` `interfaceBandwidth` `float64` for bandwidth).
- One Link TLV per LSA: multiple TE links → multiple Type-10 LSAs distinguished by Instance (§2.4.2). The Router Address TLV lives in its own dedicated LSA (Instance 0 by convention).
- RFC 5392 adds NO top-level TLV: it is Opaque type 6 + three sub-TLVs under the same Link TLV, with the Link ID sub-TLV prohibited. The IPv6 sub-TLV is type 24, not 23.
- The TED is keyed by link and queryable; `rsvpte` is the intended future consumer but is NOT wired here. The TED never feeds SPF.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/plugins/ospf/types/lstype.go` -- `LSTypeOpaqueArea` (10), `LSTypeOpaqueAS` (11), `IsOpaque()`; ext-1 adds the active scope routing and the `OpaqueType()`/`OpaqueID()` split
  → Constraint: TE reads `LSTypeOpaqueArea`/`LSTypeOpaqueAS` and the ext-1 LS-ID split; it does NOT add or redefine LS types or the split — Opaque type 1/6 select TE bodies inside the existing 8-bit Opaque-Type space
- [ ] `internal/plugins/ospf/packet/lsa_opaque.go` + `lsa.go` -- `LSA.Opaque *OpaqueLSA{Type, Data}`; opaque bodies retained verbatim; ext-1 adds `OpaqueType()`/`OpaqueID()` accessors and `packet/opaque_tlv.go` (the generic TLV iterator + builder)
  → Constraint: the TE body codec is built ON the ext-1 `opaque_tlv.go` iterator/builder; do NOT re-implement TLV framing or 4-byte alignment — only the TE-specific Router-Address / Link / sub-TLV layer is new
- [ ] `internal/plugins/ospf/config.go` -- `interfaceConfig{Name, AreaID, Cost, NetworkType, ...}` (no TE fields yet); `ospfConfig{RouterID, ReferenceBandwidth, Interfaces, ...}` (no TE router-address yet); `parseOSPFConfig` unmarshals the YANG sections; `DefaultReferenceBandwidth = 100000`
  → Constraint: TE per-link attributes attach as a new `traffic-engineering` sub-block on `interfaceConfig`; the TE Router Address attaches as a new `ospfConfig` field; reuse `parseOSPFConfig` (extend the wrapper struct), do not add a parallel parser
  → Constraint: `interfaceConfig.Cost` (uint16) is the standard OSPF metric; the TE metric (sub-TLV 5) is a SEPARATE uint32 and MUST NOT alias `Cost` (RFC 3630 §2.5.5 "may differ from the standard OSPF link metric")
- [ ] `internal/plugins/ospf/iface/iface.go` -- `InterfaceInfo{RouterID, InterfaceAddress [4]byte, Cost, ...}`; `Neighbor{RouterID, Address netip.Addr, ...}`; `Snapshot` exposes interface state to the engine
  → Constraint: origination reads the live Link ID (neighbour Router ID for p2p, DR interface address for multi-access — RFC 3630 §2.5.2), the local interface address, and the remote interface address from `InterfaceInfo`/`Neighbor`; it does not invent topology
  → Constraint: a TE LSA is originated only for an interface whose `traffic-engineering` block is enabled AND that has a usable Link ID (adjacency up / DR known); origination defers until the Link TLV is well-formed (Link Type + Link ID present, §2.5)
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `container interfaces { list interface { leaf cost; leaf network-type; ... } }` (line ~164); a parallel v6 `interfaces` container at line ~290
  → Constraint: the `traffic-engineering` container is added under the v4 `list interface` only (OSPFv2 TE); the v6 interface container is NOT touched (OSPFv3 TE out of scope); a top-level TE `router-address` leaf is added near `router-id`
- [ ] `internal/plugins/ospf/cmd_show.go` + `show_database.go` -- `cmdShowDatabase = "show ospf database"`; `dbSubviewForwarder("show ospf database <view>")`; RPCs registered as `ze-show:ospf-database-*` central-namespace methods bound by `ze-ospf-cmd.yang`
  → Constraint: the TE views follow this exact pattern — a new `show ospf te-database` RPC + the `opaque-area` database subview decoding TE bodies; register through `pluginserver.RegisterRPCs` and bind in `ze-ospf-cmd.yang`, do not invent a new dispatch mechanism
- [ ] `internal/plugins/ospf/register.go` -- `registerOSPF()`, `runOSPFEngine(conn)`; ext-1 creates the opaque consumer registry and discovers consumers at startup
  → Constraint: the TE consumer registers from its own `init()`/`registerOSPF` sibling via `RegisterOpaqueConsumer`; the engine discovers it through the ext-1 registry; no new top-level component (module-tiers: TE has the future `rsvpte` consumer but lives inside the OSPF plugin as an opaque extension)
- [ ] `internal/plugins/rsvpte/admission.go` -- `interfaceBandwidth{MaxBandwidth, MaxReservable, ReservedBandwidth float64}`, `Available()`; bandwidth in `float64` because RSVP carries IEEE-32-bit floats but float32 rounds small reservations away
  → Constraint: the TED stores bandwidth as `float64` for the same reason; the on-wire IEEE-32-bit encode/decode is the boundary, the in-memory representation is `float64`; this is the precedent the future `rsvpte` TED consumer aligns to

**Behavior to preserve:**
- The ext-1 carrier contract (scope flooding, sequencing, LS-ID split, O-bit gate, TLV iterator/builder) is consumed unchanged.
- The base OSPFv2 LSDB key triple, the standard OSPF interface `Cost`/metric, and all existing OSPF functional/interop tests — a router with no `traffic-engineering` config behaves exactly as today (the TE consumer registers but originates nothing).
- `parseOSPFConfig` signature and the existing `interfaceConfig`/`ospfConfig` fields; TE adds fields, it renames none.
- The `show ospf database` view dispatch and the `ze-show:ospf-*` RPC namespace.

**Behavior to change:** (all RFC-3630/5392-required, gated on TE being configured)
- A TE-enabled interface originates a Router-Address TE LSA (once per router) and a Link TE LSA (once per TE link) via the ext-1 carrier.
- Received Type-10 (Opaque type 1) and Type-10/11 (Opaque type 6) TE LSAs are parsed into the TED (in addition to ext-1's verbatim store/re-flood).
- New `show ospf te-database`, opaque-area TE decode, and `ze_ospf_te_*` metrics.
- A new `traffic-engineering` interface block + a TE `router-address` leaf in the OSPFv2 config.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Origination:** OSPF config carries a per-interface `traffic-engineering` block + a TE `router-address` → the TE consumer registers via `RegisterOpaqueConsumer` → on topology/config change the carrier invokes the consumer's `OnOriginate`.
- **Reception:** an LS Update carrying a Type-10/11 opaque LSA with Opaque type 1 or 6 arrives → ext-1 stores/floods verbatim and (because a consumer is registered) calls `OnReceive(opaqueID, body, scope, advRouter, reachable)`.
- **Query:** a CLI `show ospf te-database` / `show ospf database opaque-area` request → the TE show handler reads the TED / decodes the stored bodies.

### Transformation Path
1. **Config resolve (new):** `parseOSPFConfig` unmarshals the `traffic-engineering` block per interface and the TE `router-address` into `interfaceConfig.TE` / `ospfConfig.TERouterAddress`.
2. **Origination build (new):** `OnOriginate` reads enabled TE interfaces from config + live Link ID / local addr / remote addr from `iface.Snapshot`; it emits (a) one Router-Address LSA body (top-level TLV 1) at a fixed Instance, and (b) one Link LSA body per TE link (top-level TLV 2 with sub-TLVs 1,2,3,4,5,6,7,8,9 as configured; for inter-AS: Opaque type 6, no sub-TLV 2 (Link ID prohibited), plus sub-TLVs 21/22/24). Bodies are built with the ext-1 TLV builder; bandwidth fields encode IEEE-float bytes/sec.
3. **Carrier flood (existing, ext-1):** the carrier assigns the Instance/sequence, installs into the area (Type 10) or AS (Type 11) opaque store, and floods by scope. Re-origination of an unchanged body floods nothing (idempotent); withdraw MaxAge-flushes.
4. **Reception parse (new):** `OnReceive` parses the body with the ext-1 TLV iterator: a Router-Address TLV upserts the originator's TE Router Address; a Link TLV upserts a TED link entry keyed by (advertising router, Link ID, local addr) with all decoded attributes. Bandwidth decodes IEEE-float → `float64`. Unrecognized sub-TLVs are ignored (§2.4.2). For Opaque type 6, a missing/forbidden Link ID is expected; the remote end is taken from sub-TLVs 21/22/24.
5. **Reachability gate (new, Type 11 only):** the carrier-supplied `reachable` flag marks a Type-11 inter-AS-TE entry usable/unusable (§5); Type-10 entries are always usable.
6. **Withdraw (new):** a MaxAge/withdraw delivery removes the corresponding TED entry (the carrier signals the removal through `OnReceive` with a withdraw indication or a separate purge hook from ext-1).
7. **Query (new):** `show ospf te-database` renders router addresses + links + attributes from the TED; `show ospf database opaque-area` decodes TE bodies inline; `ze_ospf_te_*` metrics expose counts/gauges; the read-only TED `Snapshot` is available to the future `rsvpte` consumer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ opaque LSA | ext-1 verbatim passthrough + `OpaqueType()`/`OpaqueID()`; TE selects bodies by Opaque type 1/6 | [ ] |
| Opaque body ↔ TLVs | ext-1 TLV iterator/builder (4-byte aligned, zero-copy); TE-specific Router-Address/Link/sub-TLV layer | [ ] |
| Config ↔ origination | `interfaceConfig.TE` + `ospfConfig.TERouterAddress` → `OnOriginate` → carrier | [ ] |
| Carrier ↔ TE consumer | `RegisterOpaqueConsumer(1,…)` and `(6,…)`; `OnOriginate`/`OnReceive` value-typed payloads | [ ] |
| Reception ↔ TED | `OnReceive` parse → TED upsert/withdraw keyed by link | [ ] |
| Type-11 ↔ reachability | carrier-supplied `reachable` marks inter-AS-TE entries usable/unusable (§5) | [ ] |
| TED ↔ future consumer | read-only `Snapshot`/lookup for `rsvpte` (value-typed; not wired here) | [ ] |
| IEEE-float ↔ float64 | bandwidth encode/decode at the wire boundary; `float64` in the TED | [ ] |

### Integration Points
- `internal/plugins/ospf` (engine) -- registers the TE opaque consumer; owns the TED; routes `OnOriginate`/`OnReceive`.
- ext-1 carrier (`internal/plugins/ospf/opaque_registry.go`, `packet/opaque_tlv.go`) -- `RegisterOpaqueConsumer`, the TLV iterator/builder, the LS-ID split, scope flooding (consumed, not modified).
- `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` -- the `traffic-engineering` block and TE `router-address`.
- `internal/plugins/ospf/iface` -- READ ONLY: live Link ID / local addr / remote addr for origination.
- `internal/plugins/ospf/cmd_show.go` + `yang/ze-ospf-cmd.yang` -- `show ospf te-database` + opaque-area TE decode.
- `internal/plugins/rsvpte` -- READ ONLY future consumer of the TED `Snapshot`; NOT wired in this spec.

### Architectural Verification
- [ ] No bypassed layers (TE LSAs flow wire → ext-1 carrier → TE `OnReceive` → TED; origination flows config → `OnOriginate` → ext-1 carrier → flood)
- [ ] No unintended coupling (the carrier names no TE type; TE depends on the carrier; `rsvpte` is not imported)
- [ ] No duplicated functionality (reuses the ext-1 TLV iterator/builder, LS-ID split, scope flooding; reuses `float64` bandwidth from the `rsvpte` precedent; adds only TE bodies + the TED)
- [ ] Zero-copy preserved (TLV iterator returns views; body builder is buffer-first; TED stores decoded values, not wire slices, after parse)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ext-1 delivers `RegisterOpaqueConsumer(opaqueType, scope, OnOriginate, OnReceive)` with value-typed `OnOriginate`→`(opaqueID, scope, body, withdraw)` and `OnReceive(opaqueID, body, scope, advRouter, reachable)` | `plan/spec-ospf-ext-1-opaque-framework.md` Task + Data Flow | TE needs its own origination/reception path; large scope creep | `TestTEConsumerRegistered`, build against the ext-1 registry signature | unvalidated |
| A-2 | ext-1 provides a generic 4-byte-aligned TLV iterator + builder in `packet/opaque_tlv.go` usable for nested sub-TLVs | `plan/spec-ospf-ext-1` "Generic TLV carriage"; `packet/opaque_tlv.go` (created by ext-1) | TE must add its own TLV framing; duplicated alignment code | `TestTELinkTLVRoundTrip`, decode an FRR TE LSA | unvalidated |
| A-3 | The 24-bit Opaque ID / Instance is owned per Opaque Type by the carrier; TE may assign distinct Instances (0 for Router-Address, 1..N per link) without colliding | RFC 3630 §2.2 (no topological significance); ext-1 LS-ID split | TE LSAs overwrite each other in the LSDB key | `TestTEMultipleLinksDistinctInstance` | unvalidated |
| A-4 | Live Link ID (neighbour Router ID for p2p, DR address for multi-access), local addr, and remote addr are available from `iface.Snapshot`/`Neighbor` at origination time | `iface/iface.go` `InterfaceInfo`/`Neighbor` | origination cannot fill the mandatory Link ID sub-TLV; LSA is malformed | `TestTEOriginateLinkTLVFromSnapshot` | unvalidated |
| A-5 | Bandwidth as `float64` in the TED with an IEEE-32-bit wire boundary is the right representation (matches `rsvpte`) | `rsvpte/admission.go` `interfaceBandwidth` rationale (float32 rounds small reservations) | bandwidth precision loss; TED ≠ admission view | `TestTEBandwidthIEEERoundTrip` (encode→decode→float64) | unvalidated |
| A-6 | A Type-10 TE LSA never needs the §5 reachability gate; only Type-11 inter-AS-TE LSAs do | RFC 5250 §5; RFC 3630 area-scope only | the gate is applied where it must not be, hiding usable TE links | `TestTEType10AlwaysUsable`, `TestInterAsTEType11UnreachableUnusable` | unvalidated |
| A-7 | The standard OSPF interface `Cost` (uint16) and the TE metric (uint32, sub-TLV 5) are independent; TE metric defaults to the OSPF cost only if not configured | RFC 3630 §2.5.5 ("may differ"); `config.go` `interfaceConfig.Cost` | TE metric wrongly aliases cost; CSPF later mis-weights | `TestTEMetricIndependentOfCost` | unvalidated |
| A-8 | RFC 5392 IPv6 Remote ASBR ID is sub-TLV type 24 (not 23) | `rfc/short/rfc5392.md` pitfall (§3.2.1 "23" is editorial; §6.2 assigns 24) | wrong sub-TLV type; FRR interop fails | `TestInterAsTEIPv6AsbrIdType24`, decode an FRR inter-AS-TE LSA | unvalidated |
| A-9 | The TED can be a passive store with no SPF trigger; OSPF route computation is unaffected | RFC 3630 §1 ("No SPF or other route calculations are necessary") | TE reception perturbs SPF/route table | `TestTEReceptionDoesNotTriggerSPF` | unvalidated |
| A-10 | A `show ospf te-database` RPC can register through `pluginserver.RegisterRPCs` + `ze-ospf-cmd.yang` exactly like the existing database subviews | `cmd_show.go` `RPCRegistration` rows; `ze-ospf-cmd.yang` | a new dispatch mechanism is needed | `test/ospf/ospf-te-show.ci` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | TLV padding mis-handled (Length excludes pad, §2.3.2) → all following sub-TLVs misparse | self round-trip passes but FRR rejects / Ze misreads FRR | use the ext-1 builder/iterator exclusively (it owns 4-byte alignment); `TestTELinkTLVAlignment` for sub-TLV value lengths 1/3/4N; decode an FRR TE LSA |
| R-2 | Bandwidth read as uint32 or assumed bits/sec → values off by 8× or wildly wrong | TED shows implausible bandwidth vs FRR | IEEE-float bytes/sec encode/decode at the boundary, `float64` in the TED; `TestTEBandwidthIEEERoundTrip`; cross-check against FRR `show ip ospf database opaque-area` |
| R-3 | Unreserved Bandwidth ordering reversed (priority 0 first, §2.5.8) → reservation levels misassigned | priority-7 value where priority-0 expected | encode/decode priority 0→7 in order; `TestTEUnreservedBandwidthOrder` (8 distinct values) |
| R-4 | Mandatory Link Type / Link ID sub-TLV omitted on origination → malformed Link TLV (§2.4.2) | FRR drops the LSA; TED entry never appears on peer | origination refuses to emit a Link TLV without sub-TLV 1 and (for type 1) sub-TLV 2; `TestTEOriginateRefusesIncompleteLink` |
| R-5 | Link ID sub-TLV wrongly emitted in an Inter-AS-TE-v2 Link TLV (prohibited, §3.2.1) | FRR flags a spec violation | the inter-AS origination path omits sub-TLV 2 and emits 21/22/24 instead; `TestInterAsTEOmitsLinkID` |
| R-6 | Two TE links share the same Instance → one LSA overwrites the other in the LSDB | only one TE link appears on peers | distinct Instance per link (0 = Router-Address); `TestTEMultipleLinksDistinctInstance` |
| R-7 | A withdrawn / MaxAged TE LSA leaves a stale TED entry | TED shows a link that peers no longer advertise | withdraw delivery removes the TED entry; `TestTEWithdrawRemovesTEDEntry` |
| R-8 | Decoder panic on a malformed/truncated TE body or sub-TLV (untrusted input) | fuzz crash | parse via the bound-checked ext-1 iterator; the TE sub-TLV decode is bound-checked and never panics; extend the `packet` fuzz target with TE bodies; `TestTEBodyMalformedNoPanic` |
| R-9 | Originating TE LSAs faster than `MinLSInterval` on flapping links (§3 rate-limit) | LSA storm; FRR logs rate complaints | reuse the carrier's `MinLSInterval` rate-limit (ext-1 `OriginateSelf`); origination batches on the periodic tick; `TestTEOriginationRateLimited` |
| R-10 | TED grows unbounded under a flood of distinct TE LSAs | memory growth under attack | bound the TED to the area/AS opaque store cap (shared with ext-1); a TE LSA evicted from the opaque store removes its TED entry; `TestTEDBoundedByOpaqueStore` |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| TE consumer `init()`/`registerOSPF` calls `RegisterOpaqueConsumer(1, area, onOrig, onRecv)` and `(6, area\|as, …)` | → | the ext-1 registry stores both registrations; the engine discovers them at startup | `TestTEConsumerRegistered` (unit) + `test/ospf/ospf-te-register.ci` |
| A `traffic-engineering` interface block + TE `router-address` in config, link up | → | `OnOriginate` builds Router-Address + Link LSAs → carrier installs + floods | `test/ospf/ospf-te-originate.ci` |
| An LS Update carrying a Type-10 Opaque-type-1 TE LSA arrives | → | ext-1 `OnReceive` → TE parse → TED upsert | `test/ospf/ospf-te-receive.ci` |
| An LS Update carrying a Type-10/11 Opaque-type-6 inter-AS-TE LSA arrives | → | TE parse (sub-TLVs 21/22/24, no Link ID) → TED upsert with remote-AS/ASBR | `TestInterAsTEReceiveIntoTED` (unit) + `test/ospf/ospf-te-interas.ci` |
| `show ospf te-database` issued | → | TE show handler reads the TED → rendered router addresses + links | `test/ospf/ospf-te-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | OSPF starts with TE compiled in | the TE consumer is registered for Opaque type 1 (area) and Opaque type 6 (area or AS by policy); duplicate registration is rejected by the carrier |
| AC-2 | A `traffic-engineering` block enabled on an interface with an up adjacency / known DR + a TE `router-address` | one Router-Address TE LSA (top-level TLV 1, length-4 IPv4 = router-address, Instance 0) and one Link TE LSA (top-level TLV 2) per TE link are originated via the carrier and flood as Type 10 |
| AC-3 | A configured TE link | the Link TLV carries Link Type (sub-TLV 1) and Link ID (sub-TLV 2) exactly once, plus Local/Remote Interface IP (3/4), TE Metric (5), Max/Max-Reservable Bandwidth (6/7), Unreserved Bandwidth (8, eight floats priority 0→7), and Admin Group (9) as configured; each TLV is 4-octet aligned with padding excluded from Length (§2.3.2) |
| AC-4 | Bandwidth values to encode (e.g. 1.25e9 bytes/sec) | sub-TLVs 6/7/8 carry 32-bit IEEE-754 single-precision bytes/sec; decode reproduces the value within float32 precision and stores `float64` in the TED |
| AC-5 | Two TE-enabled links on one router | two Link TE LSAs with distinct Instances are originated; both appear in a peer's `show ospf database opaque-area`; neither overwrites the other |
| AC-6 | A received Type-10 Opaque-type-1 TE LSA | the body is parsed and a TED entry keyed by (advertising router, Link ID, local addr) is upserted with all decoded attributes; a Router-Address TLV upserts the originator's TE Router Address |
| AC-7 | A received inter-AS-TE LSA (Opaque type 6) with Remote AS Number (21), IPv4 Remote ASBR ID (22), and no Link ID sub-TLV | parsed into a TED entry whose remote end is the Remote ASBR ID + Remote AS; the absent Link ID is accepted (§3.2.1); a 2-byte ASN zero-extended into 4 octets decodes correctly |
| AC-8 | An inter-AS-TE LSA carrying an IPv6 Remote ASBR ID sub-TLV | decoded as sub-TLV type 24 (16 octets), not 23 (§3.3.3 / §6.2) |
| AC-9 | An inter-AS-TE-v2 LSA originated by Ze | it omits the Link ID sub-TLV (prohibited §3.2.1) and includes Remote AS Number (REQUIRED) plus at least one Remote ASBR ID; scope is Type 10 or Type 11 per the link's configured policy |
| AC-10 | A Type-11 inter-AS-TE LSA whose originating router is unreachable in the route table | the TED holds the entry as "present but unusable" (`reachable=false`); when the originator becomes reachable it is marked usable (§5) |
| AC-11 | A Type-10 TE LSA | always marked usable in the TED (the §5 reachability gate is Type-11-only) |
| AC-12 | A self-originated TE LSA whose config attributes do not change | re-origination floods nothing (idempotent); a changed attribute re-originates at most once per `MinLSInterval` (§3) |
| AC-13 | A TE link removed from config / interface down / TE disabled | the corresponding TE LSA is MaxAge-withdrawn via the carrier and its TED entry is removed |
| AC-14 | A withdrawn / MaxAged TE LSA received from a peer | the corresponding TED entry is removed |
| AC-15 | `show ospf te-database` issued | router addresses, links (Link ID, local/remote addr, link type), and attributes (TE metric, bandwidths, admin group, and for inter-AS the remote AS/ASBR) are rendered from the TED |
| AC-16 | `show ospf database opaque-area` on a TE LSA | the TE body is decoded inline (Router-Address or Link TLV with sub-TLVs), not shown as raw hex |
| AC-17 | A TE LSA received | the route table and SPF result are unchanged (RFC 3630 §1); no TE LSA becomes an SPF vertex (inherited from ext-1) |
| AC-18 | A malformed / truncated TE body or sub-TLV received | parsing never panics; the LSA is still stored and re-flooded verbatim by the carrier; the bad TED entry is skipped and an error metric increments |
| AC-19 | TE metric not configured on a TE link | the TE metric defaults to the standard OSPF interface cost; an explicitly configured TE metric is independent of and does not change the OSPF cost (§2.5.5) |
| AC-20 | The TED in any state | a read-only value-typed `Snapshot`/lookup is available for the future `rsvpte` consumer; `rsvpte` is not imported by the TE code |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `traffic-engineering` (bandwidth, admin-group, te-metric) on an OSPF interface + a TE router-address; a peer sees the TE LSA | config → `OnOriginate` → ext-1 carrier → flood; peer `show ip ospf database opaque-area` decodes the Link TLV | `test/ospf/ospf-te-originate.ci` + `ospf-te-frr` interop |
| 2 | Receives FRR's TE LSAs; runs `show ospf te-database` | wire → ext-1 → TE `OnReceive` → TED; `show ospf te-database` lists FRR's links + bandwidths | `test/ospf/ospf-te-receive.ci` + `ospf-te-frr` interop |
| 3 | Configures an inter-AS TE link (remote-as, remote-asbr-id, scope as); a PCE-style peer sees it AS-wide | config inter-as → `OnOriginate` (Opaque type 6, sub-TLVs 21/22/24, no Link ID, Type 11) → carrier floods AS-wide | `test/ospf/ospf-te-interas.ci` |
| 4 | Runs `ze` decode on a TE LSA hex | CLI → `packet.DecodeLSA` → ext-1 `OpaqueType()`==1/6 → TE body decode → rendered TLVs | `test/ospf/ospf-te-decode.ci` |
| 5 | Disables TE on a link / link goes down | `OnOriginate(withdraw)` → carrier MaxAge-flush; peer's TED entry and database entry clear | `test/ospf/ospf-te-withdraw.ci` |
| 6 | Removes the TE consumer (build without it) | `RegisterOpaqueConsumer(1/6)` gone; TE LSAs still flood verbatim (ext-1) but are not parsed into a TED; base OSPF unchanged | `TestUnregisteredTEStillReflooded` (ext-1 behaviour) + existing OSPF suite green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTEConsumerRegistered` | `internal/plugins/ospf/te_register_test.go` | AC-1: type 1 + type 6 registered; duplicate rejected | |
| `TestTERouterAddressTLVRoundTrip` | `internal/plugins/ospf/packet/te_lsa_test.go` | AC-2: Router-Address TLV (type 1, len 4) encode/decode | |
| `TestTELinkTLVRoundTrip` | `internal/plugins/ospf/packet/te_lsa_test.go` | AC-3, A-2: Link TLV with sub-TLVs 1-9 round-trips | |
| `TestTELinkTLVAlignment` | `internal/plugins/ospf/packet/te_lsa_test.go` | AC-3, R-1: 4-octet alignment for value lengths 1/3/4N; pad excluded from Length | |
| `TestTEBandwidthIEEERoundTrip` | `internal/plugins/ospf/packet/te_lsa_test.go` | AC-4, A-5, R-2: IEEE-float bytes/sec ↔ float64 | |
| `TestTEUnreservedBandwidthOrder` | `internal/plugins/ospf/packet/te_lsa_test.go` | AC-3, R-3: 8 floats priority 0→7 in order | |
| `TestTEAdminGroupBitNumbering` | `internal/plugins/ospf/packet/te_lsa_test.go` | AC-3: admin group LSB=group 0, MSB=group 31 (§2.5.9) | |
| `TestInterAsTERemoteAsTLV` | `internal/plugins/ospf/packet/te_interas_test.go` | AC-7: Remote AS (21) incl. 2-byte ASN zero-extension | |
| `TestInterAsTEIPv4AsbrId` / `TestInterAsTEIPv6AsbrIdType24` | `internal/plugins/ospf/packet/te_interas_test.go` | AC-7/AC-8, A-8: ASBR ID sub-TLVs 22 and 24 (not 23) | |
| `TestInterAsTEOmitsLinkID` | `internal/plugins/ospf/packet/te_interas_test.go` | AC-9, R-5: Link ID sub-TLV omitted in inter-AS Link TLV | |
| `TestTEBodyMalformedNoPanic` | `internal/plugins/ospf/packet/te_lsa_test.go` | AC-18, R-8: truncated body/sub-TLV never panics, reports error | |
| `TestTEOriginateLinkTLVFromSnapshot` | `internal/plugins/ospf/te_originate_test.go` | AC-2/AC-3, A-4: Link ID/local/remote filled from `iface.Snapshot` | |
| `TestTEOriginateRefusesIncompleteLink` | `internal/plugins/ospf/te_originate_test.go` | R-4: no Link TLV without sub-TLV 1 and (type 1) sub-TLV 2 | |
| `TestTEMultipleLinksDistinctInstance` | `internal/plugins/ospf/te_originate_test.go` | AC-5, A-3, R-6: distinct Instance per link, Router-Address at Instance 0 | |
| `TestTEMetricIndependentOfCost` | `internal/plugins/ospf/te_originate_test.go` | AC-19, A-7: TE metric ≠ OSPF cost; defaults to cost when unset | |
| `TestTEOriginationRateLimited` | `internal/plugins/ospf/te_originate_test.go` | AC-12, R-9: idempotent re-origination; ≤1 per MinLSInterval | |
| `TestInterAsTEOriginateScopePolicy` | `internal/plugins/ospf/te_originate_test.go` | AC-9: Type 10 vs Type 11 by configured policy (§3.1.1) | |
| `TestTEReceiveIntoTED` | `internal/plugins/ospf/te_ted_test.go` | AC-6: Link/Router-Address parsed into TED keyed by link | |
| `TestInterAsTEReceiveIntoTED` | `internal/plugins/ospf/te_ted_test.go` | AC-7: inter-AS entry with remote AS/ASBR | |
| `TestTEWithdrawRemovesTEDEntry` | `internal/plugins/ospf/te_ted_test.go` | AC-14, R-7: withdraw/MaxAge removes TED entry | |
| `TestTEType10AlwaysUsable` / `TestInterAsTEType11UnreachableUnusable` | `internal/plugins/ospf/te_ted_test.go` | AC-10/AC-11, A-6: §5 gate Type-11-only | |
| `TestTEDBoundedByOpaqueStore` | `internal/plugins/ospf/te_ted_test.go` | R-10: TED bounded; eviction removes entry | |
| `TestTEDSnapshotReadOnly` | `internal/plugins/ospf/te_ted_test.go` | AC-20: value-typed Snapshot; no `rsvpte` import | |
| `TestTEReceptionDoesNotTriggerSPF` | `internal/plugins/ospf/te_ted_test.go` | AC-17, A-9: no SPF/route-table change on TE receipt | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Opaque type (TE) | {1, 6} | 6 | N/A | a non-TE opaque type is not claimed by this consumer |
| Instance (Opaque ID) | 0-16777215 | 16777215 | N/A | masked to 24 bits by ext-1 |
| TE metric (sub-TLV 5) | 0-4294967295 | 4294967295 | N/A | N/A (uint32) |
| Admin group mask (sub-TLV 9) | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A | N/A (32-bit mask) |
| Unreserved bandwidth count (sub-TLV 8) | exactly 8 floats (32 octets) | 8 | <8 → malformed | >8 → malformed |
| Local/Remote IP count (sub-TLV 3/4) | 4N octets | any N≥1 | length not multiple of 4 → error | length past LSA Length → iterator error |
| Remote AS Number (sub-TLV 21) | 0-4294967295 | 4294967295 | N/A | 2-byte ASN zero-extended into high 16 bits |
| IPv6 Remote ASBR ID length (sub-TLV 24) | exactly 16 octets | 16 | <16 → malformed | >16 → malformed |
| TLV value length (alignment) | 0-65531 | any | N/A | length past LSA Length → iterator error |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-te-register` | `test/ospf/ospf-te-register.ci` | TE consumer registered; `show ospf` reports TE enabled | |
| `ospf-te-originate` | `test/ospf/ospf-te-originate.ci` | configured TE link originates Router-Address + Link LSAs; visible in `show ospf database opaque-area` | |
| `ospf-te-receive` | `test/ospf/ospf-te-receive.ci` | a received TE LSA appears in `show ospf te-database` | |
| `ospf-te-interas` | `test/ospf/ospf-te-interas.ci` | an inter-AS TE link (Opaque type 6) originates with remote-AS/ASBR; Type 10 or 11 per policy | |
| `ospf-te-decode` | `test/ospf/ospf-te-decode.ci` | `ze` decode of TE hex shows the Router-Address/Link TLV + sub-TLVs | |
| `ospf-te-withdraw` | `test/ospf/ospf-te-withdraw.ci` | disabling TE / link-down clears the TE LSA and the TED entry | |
| `ospf-te-show` | `test/ospf/ospf-te-show.ci` | `show ospf te-database` renders router addresses + links + attributes | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-te-frr` | `test/interop/scenarios/ospf-te-frr/` | FRR `ospfd` with `mpls-te on` (originates RFC 3630 TE LSAs) | Ze parses FRR's TE LSAs into the TED with correct bandwidth (IEEE-float), admin group, and TE metric; FRR accepts and decodes Ze's originated TE LSAs (Router-Address + Link TLV); 4-octet alignment and bandwidth encoding validated across implementations | |
| `ospf-te-interas-frr` | `test/interop/scenarios/ospf-te-interas-frr/` | FRR `ospfd` inter-AS TE (RFC 5392, Opaque type 6) | Ze decodes FRR's Inter-AS-TE-v2 LSA (Remote AS 21, IPv4 ASBR ID 22, no Link ID); Ze's originated inter-AS-TE LSA is accepted by FRR; Type 10 vs Type 11 scope honoured | |

> Interop is required: this adds wire body codecs (TE LSA + inter-AS sub-TLVs)
> consumed/produced across implementations. The raw-IP / multicast OSPF paths are
> Linux-only and run as QEMU integration tests (`ai/rules/qemu-testing.md`),
> consistent with the existing OSPF interop set (`ospf-p2p-frr`, etc.).

### Future (if deferring any tests)
- None. All ACs are covered by a unit, functional, or interop test above. OSPFv3 TE and CSPF/`rsvpte` integration are out of scope (separate future specs), not deferred tests of this spec.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/plugins/ospf/config.go` -- add a `teConfig` sub-block to `interfaceConfig` (enable, te-metric, max-bandwidth, max-reservable-bandwidth, admin-group, inter-as remote-as/remote-asbr-id/scope) and a `TERouterAddress` field to `ospfConfig`; extend the `parseOSPFConfig` wrapper struct (no signature change); validation (TE metric range, admin-group mask, inter-as requires remote-as)
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- a `traffic-engineering` container under the v4 `list interface` (line ~164) + a TE `router-address` leaf near `router-id`; native constraints (`range`, `pattern` for IPv4/IPv6, boolean)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- bind `show ospf te-database`; ensure `show ospf database opaque-area` exists (from ext-1) and decodes TE
- `internal/plugins/ospf/register.go` -- register the TE opaque consumer (Opaque type 1 + 6) via the ext-1 `RegisterOpaqueConsumer`; create/own the TED; register TE metrics
- `internal/plugins/ospf/cmd_show.go` -- a `show ospf te-database` RPC (`ze-show:ospf-te-database`) following the existing `RPCRegistration`/`dbSubviewForwarder` pattern
- `internal/plugins/ospf/instance.go` -- invoke the TE `OnOriginate` from the self-LSA origination trigger; route the TE `OnReceive` from the ext-1 reception hook; pass the §5 `reachable` flag through for Type 11
- `internal/plugins/ospf/doctor.go` -- (only if a runtime dependency is added; none expected — no new socket/port/binary)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `traffic-engineering` container + `router-address`; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `te-metric` `range "0..4294967295"`; `admin-group` `range`/hex pattern; bandwidth `decimal64`/`uint64` with units bytes/sec; `remote-asbr-id` IPv4/IPv6 `pattern`; `scope` `enumeration { area; as; }`; `router-address` IPv4 `pattern` |
| YANG custom validators | [ ] yes | inter-as block requires `remote-as` + at least one `remote-asbr-id` (§3.2.1/§3.3.1) — `ze:validate` + `ValidateFn`; `CompleteFn` for `scope` enum |
| CLI commands/flags | [ ] yes | `show ospf te-database` in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ospf te-database` |
| Editor autocomplete | [ ] yes | automatic for the YANG enum/typed leaves; `CompleteFn` for `scope` |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-te-show.ci`, `ospf-te-*.ci` |
| Pipe completeness | [ ] yes | `show ospf te-database` routes through `ApplyPipes`/`ProcessPipes` like the other show outputs |
| Env var registration | [ ] no | TE is operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary/cert; reuses the existing OSPF raw socket and the ext-1 carrier |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_te_lsas` | gauge | `scope` (area/as), `kind` (router-address/link/inter-as) |
| `ze_ospf_te_database_links` | gauge | `area` |
| `ze_ospf_te_originations_total` | counter | `kind` |
| `ze_ospf_te_received_total` | counter | `kind`, `usable` |
| `ze_ospf_te_parse_errors_total` | counter | `opaque_type` |
| `ze_ospf_te_unreachable_originators` | gauge | (inter-AS Type-11 §5 holds) |

> These extend the `ze_ospf_*` metric set with a `ze_ospf_te_*` prefix, registered
> by this spec's owner code. They are distinct from the ext-1 `ze_ospf_opaque_*`
> carrier series (which count opaque LSAs generically); the TE series count the
> *parsed* TE topology.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPFv2 Traffic Engineering LSAs + TED |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `traffic-engineering` block + `router-address` |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ospf te-database` |
| 4 | API/RPC added/changed? | [ ] no | the show RPC lives in the central `ze-show` namespace; documented under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains a TE opaque consumer + TED |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- a Traffic Engineering section |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` (or equivalent) -- TE LSA body + sub-TLVs |
| 8 | Plugin SDK/protocol changed? | [ ] no | TE consumes the ext-1 `RegisterOpaqueConsumer` API; no SDK change |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc3630.md` + `rfc/short/rfc5392.md` -- flip the compliance checklist items to implemented |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF TE parity with FRR |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- the TE consumer + TED |
| 13 | Route metadata keys added/changed? | [ ] no | TE LSAs install no routes (the TED is passive) |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the `ze_ospf_te_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` -- the `show ospf te-database` command + TE consumer |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF files (`config.go`, `cmd_show.go`, `register.go`) |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPF config/CLI examples against the new `traffic-engineering` block |

## Files to Create
- `internal/plugins/ospf/te.go` -- the TE opaque consumer: `OnOriginate` (build Router-Address + Link LSAs from config + `iface.Snapshot`), `OnReceive` (parse → TED upsert/withdraw), the §5 reachability gate
- `internal/plugins/ospf/te_ted.go` -- the Traffic Engineering Database: link-keyed store (advertising router, Link ID, local addr), router-address map, `Snapshot`/lookup (value-typed), bound + eviction
- `internal/plugins/ospf/packet/te_lsa.go` -- the RFC 3630 TE body codec: Router-Address TLV (1), Link TLV (2), sub-TLVs 1-9, IEEE-float bandwidth encode/decode, built on the ext-1 TLV iterator/builder
- `internal/plugins/ospf/packet/te_interas.go` -- the RFC 5392 inter-AS sub-TLVs (Remote AS 21, IPv4 ASBR ID 22, IPv6 ASBR ID 24) + the Opaque-type-6 body rules (no Link ID)
- `internal/plugins/ospf/te_show.go` -- the `show ospf te-database` render (textbuf) + the opaque-area TE decode hook
- `internal/plugins/ospf/te_register_test.go`, `te_originate_test.go`, `te_ted_test.go`
- `internal/plugins/ospf/packet/te_lsa_test.go`, `internal/plugins/ospf/packet/te_interas_test.go`
- `test/ospf/ospf-te-register.ci`, `ospf-te-originate.ci`, `ospf-te-receive.ci`, `ospf-te-interas.ci`, `ospf-te-decode.ci`, `ospf-te-withdraw.ci`, `ospf-te-show.ci`
- `test/interop/scenarios/ospf-te-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospf-te-interas-frr/` -- `ze.conf`, `frr.conf`, `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm ext-1 carrier (`RegisterOpaqueConsumer`, `opaque_tlv.go`, LS-ID split) is delivered |
| 3. Wiring phase | Wiring Test table -- register the TE consumer + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- register the TE consumer + failing wiring tests
   - Tests: `TestTEConsumerRegistered`, `test/ospf/ospf-te-register.ci`
   - Files: `te.go` (register Opaque type 1 + 6 via `RegisterOpaqueConsumer`, stub `OnOriginate`/`OnReceive`), `register.go` (create the TED, discover the consumer), `te_ted.go` (empty store skeleton)
   - Verify: both Opaque types register and the engine discovers them; origination/reception are stubs so deeper tests still fail
2. **Phase: TE body codec (RFC 3630)** -- Router-Address + Link TLV + sub-TLVs 1-9
   - Tests: `TestTERouterAddressTLVRoundTrip`, `TestTELinkTLVRoundTrip`, `TestTELinkTLVAlignment`, `TestTEBandwidthIEEERoundTrip`, `TestTEUnreservedBandwidthOrder`, `TestTEAdminGroupBitNumbering`, `TestTEBodyMalformedNoPanic`
   - Files: `packet/te_lsa.go` (built on the ext-1 TLV iterator/builder), IEEE-float helpers
   - Verify: bodies round-trip; 4-octet alignment with pad excluded from Length; bandwidth IEEE-float bytes/sec ↔ float64; malformed input never panics
3. **Phase: Inter-AS sub-TLVs (RFC 5392)** -- Opaque type 6 body
   - Tests: `TestInterAsTERemoteAsTLV`, `TestInterAsTEIPv4AsbrId`, `TestInterAsTEIPv6AsbrIdType24`, `TestInterAsTEOmitsLinkID`
   - Files: `packet/te_interas.go`
   - Verify: sub-TLVs 21/22/24; 2-byte ASN zero-extension; Link ID omitted; type 24 (not 23)
4. **Phase: Origination from config** -- `OnOriginate`
   - Tests: `TestTEOriginateLinkTLVFromSnapshot`, `TestTEOriginateRefusesIncompleteLink`, `TestTEMultipleLinksDistinctInstance`, `TestTEMetricIndependentOfCost`, `TestTEOriginationRateLimited`, `TestInterAsTEOriginateScopePolicy`, `test/ospf/ospf-te-originate.ci`, `ospf-te-interas.ci`
   - Files: `te.go` (`OnOriginate`), `config.go` + `yang/ze-ospf-conf.yang` (the `traffic-engineering` block + `router-address`)
   - Verify: Router-Address + Link LSAs built from config + live snapshot; distinct Instances; rate-limited; inter-AS scope policy honoured
5. **Phase: Reception into the TED + §5 gate** -- `OnReceive` + TED
   - Tests: `TestTEReceiveIntoTED`, `TestInterAsTEReceiveIntoTED`, `TestTEWithdrawRemovesTEDEntry`, `TestTEType10AlwaysUsable`, `TestInterAsTEType11UnreachableUnusable`, `TestTEDBoundedByOpaqueStore`, `TestTEReceptionDoesNotTriggerSPF`, `test/ospf/ospf-te-receive.ci`, `ospf-te-withdraw.ci`
   - Files: `te.go` (`OnReceive`), `te_ted.go` (upsert/withdraw, bound, §5 usable flag), `instance.go` (pass `reachable` through)
   - Verify: TED upsert/withdraw keyed by link; Type-11 reachability gate; bounded; no SPF trigger
6. **Phase: CLI + metrics + TED Snapshot** -- user surface + future consumer hook
   - Tests: `TestTEDSnapshotReadOnly`, `test/ospf/ospf-te-show.ci`, `ospf-te-decode.ci`
   - Files: `te_show.go`, `cmd_show.go`, `yang/ze-ospf-cmd.yang`, metric registration in `register.go`
   - Verify: `show ospf te-database`; opaque-area TE decode; `ze_ospf_te_*` series; value-typed `Snapshot` for `rsvpte`
7. **Functional tests** → the seven `.ci` cover the user-visible behaviour
8. **RFC refs** → add `// RFC 3630 Section X` / `// RFC 5392 Section X` comments on the TLV layout, alignment, bandwidth, §5 gate, and inter-AS sub-TLV-prohibition code
9. **Interop** → `ospf-te-frr` + `ospf-te-interas-frr` QEMU scenarios
10. **Full verification** → `make ze-verify`
11. **Complete spec** → audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; TE parity with FRR's `mpls-te` (RFC 3630 sub-TLVs 1-9 + RFC 5392 inter-AS), CSPF/SPF excluded by design |
| Correctness | 4-octet TLV alignment (pad excluded from Length); IEEE-float bytes/sec bandwidth; Unreserved order priority 0→7; admin-group bit numbering; mandatory Link Type/Link ID; inter-AS Link ID prohibited; IPv6 ASBR sub-TLV = 24; distinct Instance per link; §5 Type-11-only gate |
| Naming | `ze_ospf_te_*` metrics; YANG `traffic-engineering`/`te-metric`/`admin-group` kebab-case; TED key (adv-router, Link ID, local-addr) |
| Data flow | TE touches the ext-1 carrier + the TED only; no SPF change; no `rsvpte` import; every TED entry traces to a received LSA, every LSA to config |
| CLI grammar | `show ospf te-database` action-before-identifier |
| Doctor checks | none added (no new runtime dependency) -- confirm |
| YANG validation | every TE leaf has max native constraints; inter-as custom validator (`remote-as` + a `remote-asbr-id` required); `scope` enum |
| Prometheus counters | the six `ze_ospf_te_*` series defined, registered, listed |
| Rule: plugin-self-containment | removing the TE consumer removes all TE behaviour cleanly; no TE type in the ext-1 carrier |
| Rule: buffer-first | TE body built with the ext-1 builder; render via textbuf; IEEE-float written directly |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| TE consumer registered (type 1 + 6) | `grep -rn 'RegisterOpaqueConsumer' internal/plugins/ospf/te.go` |
| TE body codec | `go test ./internal/plugins/ospf/packet -run 'TE'` |
| Inter-AS sub-TLVs (21/22/24, no Link ID) | `go test ./internal/plugins/ospf/packet -run 'InterAsTE'` |
| Origination from config | `go test ./internal/plugins/ospf -run 'TestTEOriginate'` |
| TED reception + §5 gate | `go test ./internal/plugins/ospf -run 'TestTE.*TED|UnreachableUnusable'` |
| Six metric series registered | `grep -rn 'ze_ospf_te_' internal/plugins/ospf` |
| `show ospf te-database` | `ls test/ospf/ospf-te-show.ci` |
| Interop scenarios present | `ls test/interop/scenarios/ospf-te-frr/ test/interop/scenarios/ospf-te-interas-frr/` |
| Functional tests present | `ls test/ospf/ospf-te-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | TE body + sub-TLV parse bound-checked via the ext-1 iterator; the `packet` fuzz target extended with TE bodies; no slice-out-of-range on malformed input; IEEE-float decode handles NaN/Inf without crashing |
| Resource exhaustion | the TED is bounded by the area/AS opaque store cap (shared with ext-1); a flood of distinct TE LSAs cannot grow the TED unbounded; eviction removes entries |
| Trust boundary | TE LSAs flood only to opaque-capable neighbours (ext-1 O-bit gate) and rely on existing OSPF authentication; inter-AS remote-AS/ASBR are operator-entered config, validated before origination (§5) |
| Error leakage | TE parse errors are counted (`ze_ospf_te_parse_errors_total`), not surfaced to peers; a bad TED entry is skipped, the LSA still re-floods verbatim |
| Spec-violation handling | a received Link TLV missing mandatory sub-TLVs, or an inter-AS Link TLV carrying a prohibited Link ID, is skipped (entry not stored) without crashing |

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
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Core Insight
TE is a *body codec plus a passive database*, not a protocol change. ext-1
already floods Type 10/11 opaque LSAs by scope, sets the O-bit, splits the
LS-ID, and provides a 4-byte-aligned TLV iterator/builder. This spec only
registers Opaque type 1 (RFC 3630) and type 6 (RFC 5392), codes the
Router-Address / Link / sub-TLV bodies, and feeds a link-keyed TED that the
future `rsvpte` consumer will query. The two load-bearing wire details — 4-octet
TLV alignment with padding excluded from Length, and IEEE-float bandwidth in
bytes/sec — each have an existing precedent (the ext-1 builder; `rsvpte`
`float64` bandwidth).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build TE as an ext-1 opaque consumer | a standalone TE module with its own flooding | RFC 3630 is explicitly an opaque-LSA application (§1); ext-1 owns flooding/sequencing/scope; duplicating it violates plugin-self-containment and module-tiers |
| TED is a passive store, no SPF trigger | feed TE into SPF / route table | RFC 3630 §1 "No SPF or other route calculations are necessary"; CSPF is out of scope (guide ~1524) |
| Bandwidth `float64` in the TED, IEEE-32-bit at the wire | uint32 / float32 storage | RFC 3630 §2.4.2 is IEEE-float bytes/sec; `rsvpte` already uses `float64` because float32 rounds small reservations away — TED aligns to the future admission consumer |
| One Link TLV per LSA, distinct Instance per link | one LSA with all links | RFC 3630 §2.4.2 mandates one Link TLV per LSA for fine-grained change flooding |
| TE metric independent of OSPF cost, defaulting to cost when unset | alias TE metric to `Cost` | RFC 3630 §2.5.5 "may differ from the standard OSPF link metric" |
| Inter-AS-TE (RFC 5392) as a second Opaque type (6) registration | fold into the type-1 consumer | Opaque type is the dispatch key; type 6 has different scope policy (Type 10/11) and prohibits the Link ID sub-TLV; a distinct registration keeps the rules separate |
| `rsvpte` is a future TED consumer, not wired here | wire CSPF/admission now | scope boundary: the spec delivers the TED + query API; signalling integration is a separate effort, and `rsvpte` must not be imported by the TE code (coupling) |

## Known Limitations
- No CSPF / constraint-based path computation; the TED is passive (RFC 3630 §1).
- No RSVP-TE signalling integration; `rsvpte` is a documented future TED consumer.
- OSPFv3 TE (RFC 5329) and Inter-AS-TE-v3 (RFC 5392 §3.1.2) are out of scope (no separate `ospfv3` plugin; OSPFv3 carries TE natively, not as opaque).
- No SRLG / GMPLS sub-TLVs (RFC 4203); not in the RFC 3630 base set.
- Unnumbered links and multi-access reservation state are out of scope per RFC 3630 §1.2.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code:
- RFC 3630 §2.2 -- Opaque type 1, Instance has no topological significance
- RFC 3630 §2.3.2 -- 4-octet TLV alignment, padding excluded from Length
- RFC 3630 §2.4 / §2.4.1 / §2.4.2 -- one top-level TLV; Router Address once per router; one Link TLV per LSA; mandatory Link Type + Link ID sub-TLVs
- RFC 3630 §2.4.2 / §2.5.6-8 -- IEEE-float bandwidth bytes/sec; Unreserved order priority 0→7
- RFC 3630 §2.5.9 -- admin group bit numbering
- RFC 3630 §3 -- originate on change/refresh, rate-limit to MinLSInterval, no SPF on receipt
- RFC 5392 §3.1.1 -- Inter-AS-TE-v2 Opaque type 6, Type 10/11 by policy
- RFC 5392 §3.2.1 -- Link ID sub-TLV prohibited in inter-AS Link TLV
- RFC 5392 §3.3.1/§3.3.2/§3.3.3 -- Remote AS (21), IPv4 ASBR ID (22), IPv6 ASBR ID (24)
- RFC 5250 §5 -- Type-11 originator-reachability gate

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
|------|--------|----------|-------|

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
| Originate TE LSAs from configured TE link attributes | functional + interop | `ospf-te-originate.ci`, `ospf-te-frr` |
| Parse received TE LSAs into a queryable TED | functional + interop | `ospf-te-receive.ci`, `ospf-te-show.ci`, `ospf-te-frr` |
| RFC 5392 inter-AS sub-TLVs (21/22/24, no Link ID) | unit + functional + interop | `TestInterAsTE*`, `ospf-te-interas.ci`, `ospf-te-interas-frr` |
| Flooding reuses ext-1 (no re-implementation) | unit | `TestTEConsumerRegistered` + no new flooding code in the diff |
| §5 Type-11 reachability gate | unit | `TestInterAsTEType11UnreachableUnusable` |
| TED is passive (no SPF) | unit | `TestTEReceptionDoesNotTriggerSPF` |

## Review Gate

<!-- BLOCKING: run /ze-review BEFORE final verify; loop until 0 BLOCKER, 0 ISSUE. -->

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

<!-- BLOCKING: re-verify everything independently; do NOT trust the audit. -->

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
- [ ] AC-1..AC-20 all demonstrated
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
- [ ] RFC 3630 / RFC 5392 / RFC 5250 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (TED has a concrete future consumer in `rsvpte`)
- [ ] No speculative features (no CSPF, no signalling -- explicitly out of scope)
- [ ] Single responsibility per component (codec / TED / origination separated)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (no `rsvpte` import; carrier names no TE type)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-te-frr`, `ospf-te-interas-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-2-traffic-engineering.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-2-traffic-engineering.md`
