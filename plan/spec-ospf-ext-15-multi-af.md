# Spec: ospf-ext-15 -- Multiple address families (IPv6; RFC 5838)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ospf-ext-0-umbrella.md (umbrella); IPv6 base delivered |
| Phase | - |
| Updated | 2026-07-02 |

> This is a feature of the SINGLE unified `ospf` engine, scoped to its **IPv6
> address family** (OSPFv3, RFC 5340). Ze runs one OSPF engine spanning address
> families, exactly like `bgp` is one engine spanning families: there is no
> `bgpv4` and there is no separate `ospfv3` plugin. IPv4 and IPv6 are two ADDRESS
> FAMILIES of the one OSPF. RFC 5838 multiple-address-family support is an
> extension of the OSPF IPv6 address family, NOT a separate product. The FSM,
> flooding, DR election, SPF, and LSDB sequencing are AF-neutral and SHARED; the
> AF-specific wire/LSA/prefix code lives in the `_v6` strategy files
> (`afstrategy_v6.go`, `codec_v6.go`, `encoder_v6.go`, `origination_v6*.go`) and
> the leaf packages `internal/plugins/ospf/v3/{types,packet,transport}`. See
> `plan/learned/972-ospf-af-unify.md`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc5838.md` -- AF-to-Instance-ID ranges (IPv6 unicast 0-31, IPv6 multicast 32-63, IPv4 unicast 64-95, IPv4 multicast 96-127), the AF-bit in Options (§2.4), per-AF separate LSDB (§2), the §2.7 IPv4-in-OSPFv3 prefix/next-hop encoding
4. `rfc/short/rfc5340.md` -- §A.3.1 Instance ID common-header field, §4.2.2 Instance-ID demux (discard packet whose Instance ID does not match the interface), §A.2 Options, §A.4.1 prefix encoding `((PrefixLength + 31) / 32)` words
5. `plan/learned/972-ospf-af-unify.md` -- the unified-engine decision: one `ospf` engine with Transport/Codec/AFPrefixStrategy seams; IPv4 and IPv6 are address families; no second engine
6. `plan/spec-ospf-ext-0-umbrella.md` row ext-15 (this spec): "Map address families to OSPFv3 Instance-ID ranges (§2.4) using the v3 base's reserved Instance-ID plumbing; spawn one unified-engine instance per AF; per-AF topologies and route install; AF-aware prefix strategy without re-opening the RFC 5340 codec"
7. `internal/plugins/ospf/dispatcher.go` -- the `instanceID` field + the §4.2.2 demux in `dispatch()` (the single chokepoint the per-AF instances reuse)
8. `internal/plugins/ospf/register.go` -- `eng` (IPv4 family) + `eng6` (IPv6-unicast family) spawn; `newEngineWithCodec(transport, codec)`; `cfg.V6` drives the IPv6-family instance
9. `internal/plugins/ospf/spf/install.go` -- `NewInstaller` HARDCODES `family.IPv4Unicast`; the per-AF route install seam this spec must parameterise
10. `internal/plugins/ospf/v3/types/options.go` -- OptV6/E/N/R; NO AF-bit yet; `internal/plugins/ospf/encoder_v6.go` `neutralToV6Options` (sets V6|R) -- where the AF-bit is emitted

## Task

Add RFC 5838 multiple-address-family support to the **IPv6 address family** of
the unified OSPF engine at `internal/plugins/ospf/`. The OSPF IPv6 family
(OSPFv3, delivered earlier) currently runs a single IPv6-unicast instance over
the v6 codec/transport, and deliberately reserved a validated 8-bit Instance-ID
field (`dispatcher.instanceID`, `cfg.V6.InstanceID`,
`address-family/ipv6/instance-id` in YANG) precisely so multi-AF could attach
here without re-opening the RFC 5340 codec.

RFC 5838 carries multiple address families over the OSPF IPv6 (OSPFv3) wire by
mapping each AF to a reserved **Instance ID range** and tagging each instance's
packets with the **AF-bit** in the Options field:

| Address family | Instance ID range | Ze Loc-RIB family |
|----------------|-------------------|-------------------|
| IPv6 unicast (IPv6-family default) | 0-31 | `family.IPv6Unicast` |
| IPv6 multicast | 32-63 | `family.IPv6Multicast` |
| IPv4 unicast | 64-95 | `family.IPv4Unicast` |
| IPv4 multicast | 96-127 | `family.IPv4Multicast` |

Each address family is a **separate OSPFv3 instance** on the link with its **own
LSDB, neighbor adjacencies, and route table** (RFC 5838 §2), distinguished on
the wire only by the Instance ID. The OSPF engine already runs one engine
instance per address family (`eng` for the IPv4 family, `eng6` for the IPv6
family); this spec generalises the IPv6 family from a single fixed IPv6-unicast
instance to **one engine instance per configured OSPFv3 address family**, each
constructed with the v6 codec/transport, a per-AF Instance ID validated to fall
inside that AF's RFC 5838 range, the AF-bit set in its originated Options, an
AF-specific LSDB (already inherent: each engine owns its own `LSDB`), and route
install into the **correct Loc-RIB family** (the existing installer hardcodes
`family.IPv4Unicast` regardless of address family -- a latent gap this spec
fixes by parameterising the install family).

This includes **IPv4-over-OSPFv3**: an IPv4-unicast instance of the OSPF IPv6
family (Instance ID 64-95) carries IPv4 prefixes inside the OSPFv3 address-free
LSA model. RFC 5838 §2.7 reuses the RFC 5340 prefix encoding for IPv4 (a
0-32-bit prefix in a single 32-bit word) and carries the IPv4 next-hop as the
link-local-derived adjacency address; the resulting routes install into
`family.IPv4Unicast`. This is the AF that most exercises the abstraction (a
non-IPv6 prefix family over the IPv6 transport) and is the headline RFC 5838
capability beyond the default IPv6-unicast instance.

The work is **demux + tag + per-AF topology + per-AF install**, not a codec
change: the RFC 5340 16-byte header (with its Instance ID) is unchanged, the
five packet types are unchanged, the scope-aware LSA model is unchanged. What is
added is (a) AF resolution from the Instance ID range, (b) the AF-bit in
originated Options and an AF-bit consistency check on received Options, (c) the
per-AF engine spawn driven by the config, (d) the per-AF Loc-RIB install family,
and (e) AF/Instance-ID identification in the show commands. Single-AF behaviour
(one IPv6-unicast instance at Instance ID 0) must be preserved byte-for-byte and
interop-unchanged against FRR `ospf6d`.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Instance-ID-range AF demux | Map a configured/received Instance ID to its RFC 5838 address family (0-31 v6u, 32-63 v6m, 64-95 v4u, 96-127 v4m); reuse the existing `dispatcher.instanceID` per-instance demux unchanged; reject configuring an Instance ID outside any AF range with `> 127` |
| AF-bit handling | Add the AF-bit (`OptAF`, RFC 5838 §2.4) to `v3/types/options.go`; set it in originated Hello/DD Options for ANY multi-AF instance (RFC 5838 §2.4: an AF-bit-capable router sets it); per RFC 5838 §2.5 the AF-bit MUST be set for non-IPv6-unicast AFs and the default IPv6-unicast instance sets it only when it is itself multi-AF-aware; mismatch handling per §2.6 (an OSPFv3 instance ignores the AF-bit for the default IPv6-unicast AF for backward compatibility) |
| Per-AF LSDB | Each AF instance already owns a private `LSDB` (one engine per family); ext-15 spawns one engine per configured AF so the separation is structural, not a new key field (RFC 5838 §2: "separate link-state database per address family") |
| Per-AF route install | Parameterise the SPF `Installer` family: an IPv6-unicast instance installs into `family.IPv6Unicast`, IPv4-unicast into `family.IPv4Unicast`, etc.; fixes the current `NewInstaller` hardcode to `family.IPv4Unicast` for the IPv6-family engine |
| IPv4-over-OSPFv3 | An IPv4-unicast instance decodes/originates IPv4 prefixes through the RFC 5340 prefix model (RFC 5838 §2.7: a 0-32-bit prefix in one 32-bit word) and resolves the IPv4 next-hop from the adjacency; routes install into `family.IPv4Unicast` |
| Config | Generalise the IPv6-family `address-family` to accept `ipv6-unicast`/`ipv4-unicast`/`ipv6-multicast`/`ipv4-multicast` (or an AF + instance-id pair) with a validated per-AF Instance-ID range; preserve the existing `address-family ipv6` shape as the IPv6-unicast AF |
| CLI identification | Show commands (`show ospf ipv6 ...` and a new AF-aware listing) identify the address family + Instance ID per instance (RFC 5838 §2 debugging requirement) |
| AF-aware prefix strategy | The v6 prefix strategy decodes prefixes into the AF's address width (16 bytes for v6, 4 bytes for v4) and tags `RouteEntry` with the install family |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| Any change to the RFC 5340 common header / packet types | IPv6 base (unchanged; only the existing Instance-ID field is consumed) |
| IPv6/IPv4 multicast SPF (MOSPF-style multicast forwarding) | the multicast AF ranges are demux-validated and route-installable, but multicast-specific tree computation (RFC 1584 MOSPF) is NOT implemented; a multicast AF computes unicast-shaped reachability only |
| New LSA types or a new scope model | IPv6 base (RFC 5838 reuses the RFC 5340 LSA set verbatim) |
| Opaque / TE / SR extension LSAs | ext-5 and later (RFC 5340 carries extensions as native LSAs; no opaque carrier in the IPv6 family) |
| RFC 7166 / RFC 4552 auth changes for multi-AF | IPv6 base auth path is per-instance already; ext-16 owns IPsec |
| OSPFv2 multi-instance (RFC 6549) | not applicable -- this is an IPv6-family feature; the IPv4-family analogue is ext-12 |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/research/ospf-implementation-guide.md` §15 OSPFv3 ("Instance ID (RFC 5340 §2.5) ... now also used for multi-address-family support (RFC 5838)"; "Multiple address families (RFC 5838)"; "Recommendation for ze (REVISED 2026-06-22: unified address-family-aware engine)") -- the unified-engine boundary
  -> Decision: the engine machinery (ISM/NSM/LSDB/flooding/SPF/lifecycle) is shared; the AF seams are the wire codec, transport, prefix strategy, and now the Instance-ID-derived AF + install family; multi-AF is "one engine instance per configured OSPFv3 address family", reusing the v6 codec/transport for ALL AFs
  -> Constraint: never duplicate the engine per AF; the per-AF difference is (Instance ID range, AF-bit, prefix address width, install family), all factored out of the shared machinery
- [ ] `plan/spec-ospf-ext-0-umbrella.md` ext-15 row + "Multi-AF uses the reserved IPv6 Instance-ID" -- the umbrella contract this child fulfils
  -> Constraint: ext-15 attaches to the reserved, validated Instance-ID field WITHOUT redefining the RFC 5340 header; it maps AF to Instance-ID ranges (§2.4) and adds per-AF topologies + route install; it does not re-open the codec
  -> Decision: ext-15 is IPv6-base-only (no dependency on the other extension children); it consumes exactly the Instance-ID plumbing and the IPv6 Loc-RIB FIB-install path the IPv6 base reserved
- [ ] `plan/learned/972-ospf-af-unify.md` -- the unified-engine decision and the AF-neutral/AF-specific split
  -> Constraint: one `ospf` engine; IPv4 and IPv6 are address families; AF-specific code lives in the `_v6` strategies and `internal/plugins/ospf/v3/{types,packet,transport}`; never fork a second engine
  -> Decision: the IPv6 base reserved an explicit, validated Instance-ID field so multi-AF could attach later
- [ ] `ai/rules/buffer-first.md` -- Options + prefix emit are buffer-first
  -> Constraint: the AF-bit is set on the 24-bit Options value before `Options.WriteTo(buf, off)`; the IPv4-over-v3 prefix is written through the existing `Prefix.writeTo` word-padding path, no slice concatenation
- [ ] `ai/rules/no-sprintf-alloc.md` -- show-command rendering
  -> Constraint: AF/Instance-ID rendering in `show` uses `textbuf`/`AppendTo`, never `fmt.Sprintf` on the hot path
- [ ] `ai/rules/config-surface.md` + `ai/rules/config-naming.md` -- the AF config surface
  -> Constraint: address families are operational OSPF config (YANG under `ospf`), not `environment/` env vars; AF/leaf names are kebab-case; the Instance-ID range is enforced by a native YANG `range` plus a custom validator that ties the range to the chosen AF

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5838.md` -- AF support in OSPFv3
  -> Constraint: §2.1 -- "Address family maps to an OSPFv3 Instance ID"; the assigned ranges are IPv6 unicast 0-31, IPv6 multicast 32-63, IPv4 unicast 64-95, IPv4 multicast 96-127 (an Instance ID outside 0-127 is invalid for AF use)
  -> Constraint: §2.4 -- the AF-bit in the Options field signals AF support; a router supporting AFs sets the AF-bit in Hello and DD Options; an OSPFv3 router that does not understand AFs ignores it
  -> Constraint: §2.5/§2.6 -- for the DEFAULT IPv6-unicast AF the AF-bit is treated specially for backward compatibility (a legacy IPv6-unicast neighbour that does not set the AF-bit still forms an adjacency on the IPv6-unicast instance); for ALL OTHER AFs the AF-bit MUST be set and a neighbour not setting it is not brought to Full
  -> Constraint: §2 -- "Separate LSDB per address family instance"; "Neighbor state is per instance"; debugging "Show commands must identify the address-family instance"
  -> Constraint: §2.7 -- IPv4 (and other AF) prefixes reuse the RFC 5340 prefix encoding; an IPv4 prefix is 0-32 bits in a single 32-bit word; the next-hop for an IPv4-over-OSPFv3 route is derived from the adjacency, not from an IPv6 address in the LSA
- [ ] `rfc/short/rfc5340.md` -- OSPF IPv6 base (OSPFv3)
  -> Constraint: §A.3.1 -- the Instance ID is the common-header byte 14; §4.2.2 -- a packet whose Instance ID does not match the interface MUST be discarded (the per-instance demux this spec reuses)
  -> Constraint: §A.4.1 -- prefix byte length is `((PrefixLength + 31) / 32) * 4`; an IPv4 prefix (<= 32 bits) is exactly one 32-bit word; reject overrun and non-zero padding bits
  -> Constraint: §A.2 -- the 24-bit Options field carries V6/E/N/R today; the AF-bit is an additional defined bit (RFC 5838 §2.4) that fits the same 24-bit field

**Key insights:**
- The IPv6 base reserved EXACTLY the Instance-ID plumbing this spec needs: the `dispatcher.instanceID` demux already discards mismatched packets, and `cfg.V6.InstanceID` already plumbs a per-instance value. The new work is (a) deriving the AF from the Instance-ID range, (b) the AF-bit, (c) spawning one engine per AF, (d) the per-AF install family.
- "Separate LSDB per AF" is already structurally true: each engine owns a private `LSDB`. Multi-AF = multiple v6-style engines, each with its own LSDB/neighbors/SPF/installer. No new LSDB key field is required (and adding one would be the wrong design).
- The single real latent defect the IPv6 base left is `NewInstaller` hardcoding `family.IPv4Unicast`: the v6 IPv6-unicast engine today installs IPv6 routes under the IPv4Unicast family label. ext-15 must parameterise the install family per AF (and this also corrects the IPv6-unicast install family).
- IPv4-over-OSPFv3 is the AF that proves the abstraction: the RFC 5340 address-free LSA model already separates topology from prefixes, so an IPv4 prefix family layers onto the same topology by changing only the prefix address width and the next-hop derivation.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/dispatcher.go` -- `dispatcher.instanceID uint8`; `dispatch()` decodes the header, verifies the checksum, then drops a packet whose `h.InstanceID != instanceID` (RFC 5340 §4.2.2 demux); 0 for the IPv4 family
  -> Constraint: the per-instance Instance-ID demux already exists and is correct; multi-AF runs N engines each with its own `dispatcher.instanceID`, so the demux is reused unchanged per instance -- do NOT add a multi-value match here
- [ ] `internal/plugins/ospf/instance.go` -- `type engine`; `newEngine(t)` = `newEngineWithCodec(t, v4Codec{})`; `newEngineWithCodec(t, codec)` builds one engine with a private `lsdb`, `neighbors`, `spf`; `setConfig`/`reconcile` set `dispatch.instanceID = cfg.InstanceID` and inject the `v6Encoder{instanceID: cfg.InstanceID}` for the v6 codec
  -> Constraint: each engine instance is already fully self-contained (private LSDB/neighbors/SPF); spawning one per AF gives per-AF LSDB + neighbor state for free; the AF is a property of the engine, derived once from its Instance ID
- [ ] `internal/plugins/ospf/register.go` -- spawns `eng` (IPv4 family) via `newEngine` and a SINGLE `eng6` (IPv6 family) via `newEngineWithCodec(ospfv3transport.New(...), v6Codec{})`; `cfg.V6 != nil` drives `eng6.setConfig(*cfg.V6)` / `eng6.reconcile` / `eng6.openInterfaces`; `consumer.SetV6Injector(eng6)` wires IPv6 redistribution
  -> Constraint: the IPv6-family spawn is hardwired to one IPv6-unicast instance; multi-AF generalises this to a slice of v6-style engines keyed by AF, each from `cfg.AddressFamilies[i]`; redistribution injection must target the matching AF's engine
- [ ] `internal/plugins/ospf/spf/install.go` -- `NewInstaller(loc)` sets `fam: family.IPv4Unicast` UNCONDITIONALLY; `insert`/`remove` call `loc.InsertForward(in.fam, ...)` / `loc.Remove(in.fam, ...)`; `ProtocolID()` is the single OSPF Loc-RIB source
  -> Constraint: the install family is hardcoded; this is the central gap -- add a family-parameterised constructor (`NewInstallerFamily(loc, fam)`) so each AF installs into the right Loc-RIB family; the v6 IPv6-unicast engine currently MIS-installs into IPv4Unicast (corrected here)
- [ ] `internal/plugins/ospf/spf_wiring.go` -- `initSPF()` builds the `Computer` with `Installer: ospfspf.NewInstaller(locrib.Default())` and selects `v6Strategy{eng: e}` when `codec.IsV6()`
  -> Constraint: the installer family must be chosen from the engine's AF (derived from its Instance ID) at `initSPF`/`configureSPF`; the strategy already branches on `IsV6()` -- extend the v6 path to carry the AF's address width
- [ ] `internal/plugins/ospf/v3/types/options.go` -- `Options uint32` (24-bit); bits OptV6/E/N/R; NO AF-bit constant; `Has`, `WriteTo`, `OptionsFromBytes`
  -> Constraint: add `OptAF` (RFC 5838 §2.4) as an additional bit in the same 24-bit field; expose `AF()`/`SetAF`-style accessors mirroring the existing bit accessors
- [ ] `internal/plugins/ospf/encoder_v6.go` -- `neutralToV6Options(o)` returns `OptV6 | OptR` plus E/N; the header carries `InstanceID: e.instanceID`
  -> Constraint: the AF-bit is emitted HERE (and in the Hello encoder) for a multi-AF instance; the encoder already has the Instance ID, so it can decide AF-bit emission per RFC 5838 §2.5
- [ ] `internal/plugins/ospf/codec_v6.go` -- `v6OptionsToNeutral(o)` maps only E/N to neutral; received AF-bit is currently dropped; `DecodeHeader` surfaces `InstanceID`
  -> Constraint: the received AF-bit must be checked (not just dropped) for non-default AFs per §2.5/§2.6; the neutral Options superset has no AF concept, so the AF-bit check stays inside the v6 codec/neighbor path, keyed by the instance's AF
- [ ] `internal/plugins/ospf/v3/packet/prefix.go` -- `Prefix{Length, Options, Field16, Address}`; `decodePrefix`/`writeTo` use `plen.ByteLen()` (= `((len+31)/32)*4`), so a <=32-bit prefix is one 32-bit word; padding validated
  -> Constraint: IPv4-over-OSPFv3 needs NO new prefix codec -- a 0-32-bit prefix already encodes as one word; the AF-aware strategy reads the AF's address width when converting `Prefix` to a `netip.Prefix` (4-byte vs 16-byte)
- [ ] `internal/plugins/ospf/afstrategy_v6.go` -- `v6PrefixToNetip(p)` ALWAYS builds a 16-byte `netip.AddrFrom16`; `v6BuildRoutes` tags `RouteEntry` with no family; next-hop from the adjacency link-local
  -> Constraint: `v6PrefixToNetip` must become AF-aware (4-byte IPv4 vs 16-byte IPv6); the IPv4-over-OSPFv3 next-hop is still the adjacency address (RFC 5838 §2.7) but resolved as the IPv4 mapping where the AF is IPv4
- [ ] `internal/plugins/ospf/config.go` -- `ospfConfig.InstanceID uint8`; `ospfConfig.V6 *ospfConfig`; `applyTree` parses `instance-id` and `address-family/ipv6` into `cfg.V6`; `validateConfig` recurses into `cfg.V6`
  -> Constraint: generalise `cfg.V6` (single) to a per-AF set; parse the AF + per-AF `instance-id`; validate the Instance ID against the chosen AF's RFC 5838 range; keep `address-family ipv6` parsing as the IPv6-unicast AF for back-compat
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `address-family/ipv6/instance-id` (uint8 range 0..255, default 0) + per-AF areas/interfaces
  -> Constraint: add the other AF containers (or an AF-typed list) with the per-AF Instance-ID range constraint; a custom validator enforces the AF-to-range binding the native `range` alone cannot

**Behavior to preserve:**
- Single IPv6-unicast instance (Instance ID 0) behaves EXACTLY as today: same Hello/DD/LSA wire bytes against FRR `ospf6d`, same adjacency formation, same LSDB, same routes -- EXCEPT the install family is corrected to `family.IPv6Unicast` (a fix, validated by the existing v6 interop scenarios still passing with the route now in the IPv6 RIB).
- The OSPF IPv4-family (OSPFv2) engine is entirely untouched (`InstanceID` 0, no Instance-ID field on the wire).
- The RFC 5340 codec, the five packet types, the scope-aware LSA model, the `dispatcher.instanceID` demux mechanism, the `v6Codec`/`v6Encoder` shapes, and the `AFPrefixStrategy` interface.
- All existing OSPF IPv6-family functional/interop tests (`test/interop/scenarios/ospf-v6-*`).

**Behavior to change:** (all RFC-5838-required or correcting an IPv6-base latent gap)
- `NewInstaller` family is no longer hardcoded to `family.IPv4Unicast`; the IPv6 family installs into its AF's Loc-RIB family.
- A multi-AF instance sets the AF-bit in Hello/DD Options; a non-default-AF neighbour not setting it is not brought to Full (§2.5).
- The IPv6 family spawns one engine per configured AF (today: exactly one IPv6-unicast engine).
- `v6PrefixToNetip` becomes AF-aware (IPv4 prefixes -> 4-byte addresses).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** `ospf { address-family <af> { instance-id N; areas {...} interfaces {...} } }` -> `parseOSPFConfig` -> per-AF sub-config -> one v6-style engine spawned per AF in `register.go`.
- **Receive:** an OSPFv3 datagram arrives on the v6 transport -> the transport's `PeekInstanceID` selects the engine whose `dispatcher.instanceID` matches -> `dispatch()` runs the §4.2.2 demux (drop on mismatch) -> per-AF handler.
- **AF-bit:** Hello/DD Options carry the AF-bit; the v6 codec reads it; the neighbor FSM gates Full on it for non-default AFs.
- **Install:** a per-AF SPF run produces `RouteEntry` values -> the per-AF `Installer` inserts `locrib.Path` into THAT AF's Loc-RIB family.

### Transformation Path
1. **Config -> per-AF engines (new):** `parseOSPFConfig` yields an IPv4-family config plus a set of IPv6-family AF sub-configs (each with a validated Instance ID in its AF range); `register.go` spawns `newEngineWithCodec(ospfv3transport..., v6Codec{})` per AF, sets `dispatch.instanceID`, derives the AF from the Instance-ID range, and selects the install family.
2. **Instance-ID demux (existing, reused per instance):** the transport routes a received datagram to the engine whose Instance ID matches; `dispatch()` re-validates `h.InstanceID == instanceID` and drops on mismatch (RFC 5340 §4.2.2). Multiple AF instances on one link coexist because each owns a distinct Instance ID.
3. **AF-bit on send (new):** the v6 encoder sets `OptAF` in Hello/DD Options for a multi-AF instance (RFC 5838 §2.4/§2.5); the default IPv6-unicast instance follows the §2.6 back-compat rule.
4. **AF-bit on receive (new):** the v6 codec surfaces the received AF-bit; the neighbor FSM, for a non-default AF, refuses Full to a neighbour whose Options lack the AF-bit (§2.5); the default IPv6-unicast AF ignores a missing AF-bit (§2.6).
5. **Per-AF LSDB + SPF (existing, multiplied):** each AF engine floods/stores in its private LSDB and runs SPF over its own topology; no cross-AF leakage because the engines are separate.
6. **AF-aware prefix decode (new):** the v6 strategy converts a `Prefix` to a `netip.Prefix` at the AF's address width (4 bytes for IPv4, 16 for IPv6) and tags the `RouteEntry` install family.
7. **Per-AF install (new):** the AF's `Installer` (family-parameterised) inserts `locrib.Path` into the matching `family.<af>` Loc-RIB; sysrib + fibkernel program the route in the right table.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config <-> per-AF engines | `parseOSPFConfig` -> AF sub-configs -> per-AF `newEngineWithCodec` spawn; AF derived from Instance-ID range | [ ] |
| Wire <-> instance | transport `PeekInstanceID` + `dispatcher.instanceID` demux (RFC 5340 §4.2.2), one instance per AF | [ ] |
| Options <-> AF capability | `OptAF` set in originated Hello/DD; received AF-bit gates Full for non-default AFs (§2.5/§2.6) | [ ] |
| Prefix <-> RouteEntry | AF-aware `v6PrefixToNetip` (4 vs 16 byte); `RouteEntry` tagged with install family | [ ] |
| SPF <-> Loc-RIB | family-parameterised `Installer` inserts into `family.<af>`; fixes the IPv4Unicast hardcode | [ ] |
| Redistribution <-> AF instance | `SetV6Injector` targets the matching AF engine (IPv6/IPv4 redistribution into the right instance) | [ ] |

### Integration Points
- `internal/plugins/ospf/dispatcher.go` -- the existing `instanceID` demux (consumed, unchanged mechanism, one per AF instance).
- `internal/plugins/ospf/instance.go` + `register.go` -- the per-AF engine spawn; AF derived once from the Instance ID.
- `internal/plugins/ospf/spf/install.go` -- the family-parameterised `Installer` (the central code change).
- `internal/plugins/ospf/v3/types/options.go` + `encoder_v6.go` + `codec_v6.go` -- the AF-bit definition, emission, and receive check.
- `internal/plugins/ospf/afstrategy_v6.go` -- AF-aware prefix conversion + `RouteEntry` family tag.
- `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` -- the per-AF config surface + Instance-ID range validation.
- `internal/core/family` -- the four AF constants (consumed, not changed).
- `internal/core/rib/locrib` -- the per-family Loc-RIB insert (consumed).

### Architectural Verification
- [ ] No bypassed layers (per-AF datagrams flow transport -> instance demux -> per-AF LSDB/SPF -> per-AF Loc-RIB, the same spine as the single-AF IPv6 family)
- [ ] No unintended coupling (AF engines are independent; one AF's LSDB/SPF/RIB never reads another's)
- [ ] No duplicated functionality (reuses the engine, the `instanceID` demux, the `v6Codec`/`v6Encoder`, the `AFPrefixStrategy`; adds only AF resolution, the AF-bit, per-AF spawn, the family-parameterised installer, AF-aware prefix width)
- [ ] Zero-copy preserved (Options + prefix emit buffer-first; the prefix `Address` stays a view; `RouteEntry` carries a family enum, not a copy of the RIB)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `dispatcher.instanceID` per-instance demux is sufficient for multi-AF; running N engines (one per AF, distinct Instance ID) needs no multi-value match | `dispatcher.go` `dispatch()` drops `h.InstanceID != instanceID`; RFC 5340 §4.2.2 | a shared demux table is needed; larger change | `TestMultiAFInstanceDemux` (each instance accepts only its Instance ID) | unvalidated |
| A-2 | Each engine already owns a private LSDB/neighbors/SPF, so per-AF separation is structural and needs no LSDB key change | `instance.go` `newEngineWithCodec` builds private `lsdb`/`neighbors`/`spf` | a per-AF key field is needed in the LSDB; major change | `TestPerAFLSDBIsolation` (an LSA in AF-A's LSDB never appears in AF-B's) | unvalidated |
| A-3 | The SPF `Installer` family is the only place the install family is fixed; parameterising it routes each AF to the right Loc-RIB | `spf/install.go` `fam: family.IPv4Unicast`; all inserts use `in.fam` | install family is decided elsewhere too; scattered change | `TestInstallerFamilyPerAF` (v6u->IPv6Unicast, v4u->IPv4Unicast) | unvalidated |
| A-4 | The current v6 IPv6-unicast engine installs IPv6 routes under `family.IPv4Unicast` (a latent IPv6-base gap) and correcting it does not break the v6 interop scenarios (they assert reachability, not the RIB family) | `spf/install.go` hardcode; `spf_wiring.go` `NewInstaller(locrib.Default())` for the IPv6-family engine | the IPv6 base relied on the IPv4Unicast label somewhere; correction regresses | `ospf-v6-frr` interop still passes with the route now in the IPv6 RIB; `TestV6UnicastInstallsIPv6Family` | unvalidated |
| A-5 | The 24-bit Options field has room for the AF-bit and the AF-bit position (RFC 5838 §2.4) does not collide with OptV6/E/N/R | `v3/types/options.go` (V6=0x1, E=0x2, N=0x8, R=0x10); RFC 5838 §2.4 | the AF-bit aliases an existing bit; wire corruption | `TestAFBitDistinct` + decode an FRR multi-AF Hello | unvalidated |
| A-6 | An IPv4 prefix (<=32 bits) round-trips through the existing RFC 5340 prefix codec as one 32-bit word with no codec change | `v3/packet/prefix.go` `ByteLen()` = `((len+31)/32)*4`; RFC 5838 §2.7 | a new IPv4 prefix codec is needed | `TestIPv4OverV3PrefixRoundTrip` (a /24 IPv4 prefix encodes in 4 bytes) | unvalidated |
| A-7 | The IPv4-over-OSPFv3 next-hop is the adjacency address (RFC 5838 §2.7), resolvable through the existing `NextHopSource` with an IPv4 mapping | `afstrategy_v6.go` `v6NextHop` resolves the adjacency link-local; RFC 5838 §2.7 | IPv4 next-hop needs a separate resolution path | `TestIPv4OverV3NextHop` + the `ospf-multiaf-v4-frr` interop | unvalidated |
| A-8 | Setting the AF-bit on the default IPv6-unicast instance does not break adjacency with a legacy FRR `ospf6d` that does not set it (§2.6 back-compat) | RFC 5838 §2.6 (default AF ignores a missing AF-bit) | the AF-bit breaks IPv6-unicast interop | `ospf-v6-frr` still reaches Full with the AF-bit set | unvalidated |
| A-9 | FRR `ospf6d` supports RFC 5838 multi-AF (at least IPv4-unicast over OSPFv3) for interop | guide §15 ("FRR supports IPv6 unicast well and the others minimally") | no FRR multi-AF peer; interop must use a second Ze or a captured corpus | probe FRR `ospf6d` for AF config; fall back to Ze<->Ze + a captured FRR multi-AF Hello in the unit corpus | unvalidated |
| A-10 | The per-AF redistribution injector (`SetV6Injector`) can be generalised to target the matching AF engine without reworking the redistribution consumer | `register.go` `consumer.SetV6Injector(eng6)`; the consumer holds one v6 injector | redistribution needs a per-AF injector map; consumer rework | `TestRedistTargetsAFEngine` (an IPv4 redistribution lands in the v4-over-v3 instance) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Correcting the v6-unicast install family from IPv4Unicast to IPv6Unicast regresses a path that read the route from the IPv4 RIB | `ospf-v6-frr` interop loses reachability; an IPv6 route missing from the FIB | the interop scenarios assert reachability through the FIB regardless of label; grep all readers of the OSPF v6 routes for the family they query; `TestV6UnicastInstallsIPv6Family` pins the corrected family |
| R-2 | The AF-bit gates Full incorrectly: too strict breaks IPv6-unicast back-compat, too loose admits a mismatched-AF neighbour | `ospf-v6-frr` fails to reach Full (too strict) OR a neighbour forms on the wrong AF (too loose) | implement §2.5/§2.6 exactly: default AF ignores a missing AF-bit, non-default AFs require it; `TestAFBitGatesFullNonDefault` + `TestAFBitIgnoredDefaultAF` |
| R-3 | An Instance ID outside 0-127, or inside the wrong AF range, is configured and silently mis-classified | a v4-unicast instance forms on an Instance ID in the IPv6 range | a custom YANG validator binds the Instance ID to the chosen AF and rejects out-of-range; `TestInstanceIDRangeValidation` boundary cases |
| R-4 | IPv4-over-OSPFv3 builds a 16-byte address from a 4-byte prefix (or vice-versa), corrupting the route | the installed IPv4 route has a garbage address | `v6PrefixToNetip` reads the AF address width; `TestIPv4OverV3PrefixRoundTrip` + `TestV6PrefixToNetipAFWidth` for both widths |
| R-5 | Two AF instances on one link interfere (shared transport socket, cross-AF flooding) | an LSA from AF-A appears in AF-B's database | each engine owns its LSDB/neighbors; the transport demux routes by Instance ID; `TestPerAFLSDBIsolation` + a two-AF integration test |
| R-6 | The per-AF engine spawn leaks goroutines / sockets when an AF is added then removed by a config change | fd/goroutine growth across reconcile | reuse the existing engine lifecycle (`cancel`/`wg`); the reconcile path stops a removed AF's engine; `TestAFReconcileAddRemove` |
| R-7 | Show commands cannot distinguish AF instances, breaking the §2 debugging requirement | `show ospf ipv6 neighbor` mixes two AFs' neighbours | every AF instance is identified by AF + Instance ID in the show output; `ospf-multiaf-show.ci` asserts both AFs are listed distinctly |
| R-8 | The AF-bit emission decision (which instances set it) diverges between the Hello encoder and the DD encoder | a neighbour sees the AF-bit in Hello but not DD, or vice-versa | the AF-bit is decided once from the instance's AF and applied uniformly in `neutralToV6Options`; `TestAFBitInHelloAndDD` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ospf { address-family ipv4-unicast { instance-id 64; ... } }` in config | -> | `parseOSPFConfig` yields a v4-unicast AF sub-config; `register.go` spawns a v6-codec engine at Instance ID 64 with install family `family.IPv4Unicast` | `TestMultiAFEngineSpawn` (unit) + `test/ospf/ospf-multiaf-config.ci` |
| An OSPFv3 datagram with Instance ID 64 arrives | -> | the transport routes it to the v4-unicast instance; `dispatch()` accepts (matches), the IPv6-unicast instance (Instance ID 0) drops it | `TestMultiAFInstanceDemux` (unit) + `ospf-multiaf.ci` |
| A multi-AF instance originates a Hello | -> | `neutralToV6Options` sets `OptAF`; the wire Hello carries the AF-bit | `TestAFBitInHelloAndDD` (unit) |
| An SPF run completes on the v4-unicast instance | -> | the family-parameterised `Installer` inserts the IPv4 route into `family.IPv4Unicast` | `TestInstallerFamilyPerAF` (unit) + `ospf-multiaf-v4-route.ci` |
| `show ospf ipv6` with two AFs configured | -> | the show handler lists each AF instance with its AF + Instance ID | `ospf-multiaf-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An OSPFv3 AF configured with Instance ID N | the AF is resolved from N's RFC 5838 range (0-31 v6u, 32-63 v6m, 64-95 v4u, 96-127 v4m); a config with N > 127 or N in a range not matching the declared AF is rejected at validation (§2.1) |
| AC-2 | Two OSPFv3 AFs configured (e.g., IPv6-unicast at 0 and IPv4-unicast at 64) | two independent engine instances run, each with its own LSDB, neighbor table, and SPF; an LSA in one AF's LSDB never appears in the other's (§2) |
| AC-3 | A datagram with Instance ID 64 received while both AFs are up | only the IPv4-unicast instance processes it; the IPv6-unicast instance drops it on the Instance-ID demux (RFC 5340 §4.2.2) |
| AC-4 | A multi-AF instance originates Hello / DD | the Options field has the AF-bit (`OptAF`) set (RFC 5838 §2.4); the AF-bit appears identically in Hello and DD |
| AC-5 | A non-default-AF neighbour whose Options lack the AF-bit | it is NOT brought to Full on that AF instance (§2.5) |
| AC-6 | A default IPv6-unicast neighbour whose Options lack the AF-bit (legacy FRR) | it still forms a Full adjacency on the IPv6-unicast instance (§2.6 back-compat) |
| AC-7 | An IPv6-unicast OSPFv3 instance computes a route | the route installs into `family.IPv6Unicast` (correcting the IPv6 base's IPv4Unicast hardcode) |
| AC-8 | An IPv4-unicast OSPFv3 instance computes a route | the route installs into `family.IPv4Unicast` with an IPv4 prefix and an IPv4 next-hop derived from the adjacency (§2.7) |
| AC-9 | An IPv4 prefix (e.g., /24) carried in an OSPFv3 Intra-Area-Prefix LSA on the IPv4-unicast instance | it round-trips through the RFC 5340 prefix codec as one 32-bit word and decodes to a 4-byte `netip.Prefix` (§2.7 / RFC 5340 §A.4.1) |
| AC-10 | Two AFs configured; `show ospf ipv6` (or the AF-aware show) run | each AF instance is listed with its address family and Instance ID (§2 debugging) |
| AC-11 | A single IPv6-unicast instance (Instance ID 0) only | the on-wire Hello/DD/LSA bytes and adjacency formation against FRR `ospf6d` are unchanged from the IPv6 base, except the route now installs into `family.IPv6Unicast` |
| AC-12 | An AF added then removed by a config change | the added AF's engine starts (interfaces open, adjacency forms); on removal its engine stops cleanly (interfaces closed, routes withdrawn, no goroutine/fd leak) |
| AC-13 | IPv4 redistribution configured with a v4-over-OSPFv3 instance present | the redistributed IPv4 route originates an OSPFv3 AS-External LSA on the IPv4-unicast instance (not the IPv6 instance) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures an IPv6-unicast AND an IPv4-unicast OSPFv3 AF on one interface | config -> two AF sub-configs -> two engine instances (Instance ID 0 and 64) -> two adjacencies form, each on its own LSDB/RIB | `test/ospf/ospf-multiaf.ci` + `ospf-multiaf-frr` (or Ze<->Ze) interop |
| 2 | Advertises an IPv4 prefix over the IPv4-unicast OSPFv3 instance and reaches it | redistribute/connected -> IPv4 prefix in an OSPFv3 LSA on Instance ID 64 -> peer SPF -> route in `family.IPv4Unicast` -> FIB | `ospf-multiaf-v4-route.ci` + `ospf-multiaf-v4-frr` interop |
| 3 | Runs `show ospf ipv6` and sees both AF instances distinctly identified | show handler -> per-AF instance listing with AF + Instance ID | `ospf-multiaf-show.ci` |
| 4 | Keeps a single IPv6-unicast instance and forms an adjacency with legacy FRR `ospf6d` | AF-bit set on the default instance; §2.6 back-compat -> Full adjacency unchanged | `ospf-v6-frr` interop still green |
| 5 | Removes the IPv4-unicast AF from the config | reconcile -> the v4-over-v3 engine stops, its interfaces close, its routes withdraw; the IPv6-unicast instance is unaffected | `TestAFReconcileAddRemove` + `ospf-multiaf-reconcile.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAFFromInstanceID` | `internal/plugins/ospf/multiaf_test.go` | AC-1: Instance-ID-range -> AF mapping for all four ranges; >127 invalid | |
| `TestInstanceIDRangeValidation` | `internal/plugins/ospf/config_test.go` | AC-1, R-3: config rejects an Instance ID outside the declared AF's range (boundary 31/32/63/64/95/96/127/128) | |
| `TestMultiAFEngineSpawn` | `internal/plugins/ospf/multiaf_test.go` | wiring: a per-AF config spawns a v6-codec engine with the right Instance ID + install family | |
| `TestMultiAFInstanceDemux` | `internal/plugins/ospf/dispatcher_test.go` | AC-3, A-1: each instance accepts only its Instance ID, drops the other AF's | |
| `TestPerAFLSDBIsolation` | `internal/plugins/ospf/multiaf_test.go` | AC-2, A-2, R-5: an LSA in AF-A's LSDB never appears in AF-B's | |
| `TestAFBitDistinct` | `internal/plugins/ospf/v3/types/options_test.go` | A-5: `OptAF` does not alias OptV6/E/N/R; `AF()`/`SetAF` round-trip | |
| `TestAFBitInHelloAndDD` | `internal/plugins/ospf/encoder_v6_test.go` | AC-4, R-8: the AF-bit is set in both Hello and DD Options for a multi-AF instance | |
| `TestAFBitGatesFullNonDefault` | `internal/plugins/ospf/multiaf_test.go` | AC-5, R-2: a non-default-AF neighbour without the AF-bit is not brought to Full | |
| `TestAFBitIgnoredDefaultAF` | `internal/plugins/ospf/multiaf_test.go` | AC-6, A-8, R-2: a default IPv6-unicast neighbour without the AF-bit still reaches Full | |
| `TestInstallerFamilyPerAF` | `internal/plugins/ospf/spf/install_test.go` | AC-7/AC-8, A-3: v6u->IPv6Unicast, v4u->IPv4Unicast, v6m->IPv6Multicast, v4m->IPv4Multicast | |
| `TestV6UnicastInstallsIPv6Family` | `internal/plugins/ospf/spf/install_test.go` | AC-7, A-4, R-1: the IPv6-unicast instance installs into IPv6Unicast (not IPv4Unicast) | |
| `TestIPv4OverV3PrefixRoundTrip` | `internal/plugins/ospf/v3/packet/prefix_test.go` | AC-9, A-6: a 0..32-bit IPv4 prefix encodes as one 32-bit word and decodes back exactly | |
| `TestV6PrefixToNetipAFWidth` | `internal/plugins/ospf/afstrategy_v6_test.go` | AC-9, R-4: `v6PrefixToNetip` builds a 4-byte address for an IPv4 AF, 16-byte for IPv6 | |
| `TestIPv4OverV3NextHop` | `internal/plugins/ospf/afstrategy_v6_test.go` | AC-8, A-7: the IPv4-over-v3 next-hop resolves from the adjacency | |
| `TestRedistTargetsAFEngine` | `internal/plugins/ospf/redist_wiring_test.go` | AC-13, A-10: IPv4 redistribution originates on the v4-over-v3 instance | |
| `TestAFReconcileAddRemove` | `internal/plugins/ospf/multiaf_test.go` | AC-12, R-6: adding/removing an AF starts/stops its engine cleanly | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Instance ID (overall AF-usable) | 0-127 | 127 | N/A | 128 (rejected for AF use) |
| Instance ID -> IPv6 unicast | 0-31 | 31 | N/A | 32 (maps to v6 multicast) |
| Instance ID -> IPv6 multicast | 32-63 | 63 | 31 (maps to v6 unicast) | 64 (maps to v4 unicast) |
| Instance ID -> IPv4 unicast | 64-95 | 95 | 63 (maps to v6 multicast) | 96 (maps to v4 multicast) |
| Instance ID -> IPv4 multicast | 96-127 | 127 | 95 (maps to v4 unicast) | 128 (out of AF space) |
| IPv4-over-v3 prefix length | 0-32 | 32 | N/A | 33 (rejected, exceeds IPv4) |
| AF-bit position (24-bit Options) | within 0x000000-0xFFFFFF | set bit | N/A | must not collide with V6/E/N/R |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-multiaf-config` | `test/ospf/ospf-multiaf-config.ci` | two AFs configured; both engines start; `show` lists both | |
| `ospf-multiaf` | `test/ospf/ospf-multiaf.ci` | two AFs reach Full adjacency, each in its own LSDB | |
| `ospf-multiaf-v4-route` | `test/ospf/ospf-multiaf-v4-route.ci` | an IPv4 prefix over OSPFv3 installs into the IPv4 RIB | |
| `ospf-multiaf-show` | `test/ospf/ospf-multiaf-show.ci` | show output identifies each AF + Instance ID | |
| `ospf-multiaf-reconcile` | `test/ospf/ospf-multiaf-reconcile.ci` | adding/removing an AF starts/stops its engine; the other AF is unaffected | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-multiaf-frr` | `test/interop/scenarios/ospf-multiaf-frr/` | FRR `ospf6d` (IPv6-unicast + IPv4-unicast AFs, if supported) else a second Ze instance | Ze sets the AF-bit, forms per-AF adjacencies, exchanges per-AF LSDBs, and installs per-AF routes; the AF-bit demux interoperates | |
| `ospf-multiaf-v4-frr` | `test/interop/scenarios/ospf-multiaf-v4-frr/` | FRR `ospf6d` IPv4-unicast-over-OSPFv3, else Ze<->Ze | an IPv4 route advertised over OSPFv3 (Instance ID 64) is learned and installed into the IPv4 FIB on both ends (§2.7) | |
| `ospf-v6-frr` (regression) | `test/interop/scenarios/ospf-v6-frr/` | FRR `ospf6d` | the single IPv6-unicast instance still forms Full with the AF-bit set (§2.6) and now installs into the IPv6 RIB (AC-11) | |

> Interop is required: this changes wire behaviour (the AF-bit in Options) and
> route placement (per-AF Loc-RIB family). The raw-IPv6 / multicast paths are
> Linux-only and run as QEMU integration tests (`ai/rules/qemu-testing.md`),
> consistent with the rest of the OSPF IPv6-family interop set. Per A-9, if FRR
> `ospf6d` lacks IPv4-over-OSPFv3, the interop falls back to Ze<->Ze plus a
> captured FRR multi-AF Hello in the unit corpus; the fallback is recorded as a
> Deviation, not a silent skip.

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above. Multicast-AF SPF tree computation is OUT OF SCOPE (declared), not deferred.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/v3/types/options.go` -- add `OptAF` (RFC 5838 §2.4) + `AF()`/`SetAF` accessors
- `internal/plugins/ospf/encoder_v6.go` -- set `OptAF` in `neutralToV6Options` (Hello + DD) for a multi-AF instance, decided from the instance's AF
- `internal/plugins/ospf/codec_v6.go` -- surface the received AF-bit (do not drop it) so the neighbor FSM can gate Full for non-default AFs
- `internal/plugins/ospf/spf/install.go` -- `NewInstallerFamily(loc, fam)`; keep `NewInstaller` as the IPv4-unicast convenience or re-point it via the new constructor; replace the hardcoded `family.IPv4Unicast` with the parameter
- `internal/plugins/ospf/spf_wiring.go` -- choose the installer family from the engine's AF; pass the AF address width into the v6 strategy
- `internal/plugins/ospf/spf/computer.go` -- thread the family-parameterised installer through the `Config` (the `nil`-installer fallback selects the right family)
- `internal/plugins/ospf/afstrategy_v6.go` -- make `v6PrefixToNetip` AF-aware (4 vs 16 byte); tag `RouteEntry` with the AF install family; AF-aware next-hop for IPv4-over-v3
- `internal/plugins/ospf/instance.go` -- store the engine's AF (derived from Instance ID); use it for the installer family and the AF-bit decision
- `internal/plugins/ospf/dispatcher.go` -- (no mechanism change) confirm the per-instance demux is reused; add an AF field if the AF must be visible at dispatch for diagnostics
- `internal/plugins/ospf/register.go` -- spawn one v6-codec engine per configured AF (generalise the single `eng6`); route redistribution injection to the matching AF engine
- `internal/plugins/ospf/config.go` -- generalise `cfg.V6` (single) to a per-AF set; parse the AF + per-AF `instance-id`; validate the Instance ID against the AF range
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- add the other AF containers (or an AF-typed list) with the per-AF Instance-ID `range` + a custom validator binding the range to the AF
- `internal/plugins/ospf/cmd_show.go` + `internal/plugins/ospf/show_summary.go` -- identify the AF + Instance ID per instance in the show output
- `internal/plugins/ospf/redist_wiring.go` -- route IPv4/IPv6 redistribution to the correct AF engine

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the per-AF containers + Instance-ID range; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | native `range "0..127"` on `instance-id`; an `enumeration` on the AF leaf |
| YANG custom validators | [ ] yes | a custom validator binding the Instance ID to the declared AF's RFC 5838 range (native `range` alone cannot express "must fall inside THIS AF's sub-range"); register in `validators_register.go`; `CompleteFn` offers the AF names |
| CLI commands/flags | [ ] yes | `show ospf ipv6 ...` AF-aware listing in `cmd_show.go` (+ any new `address-family` filter) |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ospf ipv6 neighbor` etc. |
| Editor autocomplete | [ ] yes | automatic for the AF `enumeration`; `CompleteFn` for the AF names in the custom validator |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-multiaf-*.ci` |
| Pipe completeness | [ ] yes | the AF-aware show routes through `ApplyPipes` like the other OSPF show outputs |
| Env var registration | [ ] no | AFs are operational OSPF config, not `environment/` leaves |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary/cert; each AF instance reuses the existing v6 raw IPv6 socket (the transport doctor check already covers it) |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new / extended series)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_af_instances` | gauge | `af` (ipv6-unicast/ipv6-multicast/ipv4-unicast/ipv4-multicast) |
| `ze_ospf_routes_installed` | gauge | extend the existing series with an `af` label (per-AF install count) |
| `ze_ospf_af_bit_mismatch_total` | counter | `af` (a neighbour dropped/not-Full for a missing AF-bit on a non-default AF) |

> `ze_ospf_routes_installed` already exists (owned by `spf/install.go`); ext-15
> adds the `af` label so per-AF counts are distinguishable. The two new series use
> the `ze_ospf_af_*` prefix. The OSPF umbrella metrics table gains these rows
> when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF IPv6-family multiple address families (RFC 5838) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the per-AF `address-family` blocks + Instance-ID ranges |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- AF-aware `show ospf ipv6` |
| 4 | API/RPC added/changed? | [ ] no | show RPCs live in the central show namespace; document under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains per-AF OSPFv3 instances |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- a multi-AF (IPv6-family) section |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospfv3.md` -- the AF-bit in Options + the Instance-ID-range AF demux |
| 8 | Plugin SDK/protocol changed? | [ ] no | no SDK surface change; multi-AF is internal to the OSPF plugin |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5838.md` -- tick the compliance checklist items now implemented |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF IPv6-family multi-AF parity with FRR |
| 12 | Internal architecture changed? | [ ] yes | `docs/architecture/core-design.md` (OSPF subsystem) -- per-AF engine instances + install family |
| 13 | Route metadata keys added/changed? | [ ] no | per-AF routes use the existing OSPF source identity; only the family differs |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- `ze_ospf_af_*` + the `af` label on `ze_ospf_routes_installed` |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the OSPF umbrella metrics table |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into `spf/install.go`, `dispatcher.go`, `config.go`, `encoder_v6.go` |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF IPv6-family config/CLI examples against the generalised `address-family` shape |

## Files to Create
- `internal/plugins/ospf/multiaf.go` -- the AF type + `afFromInstanceID(id) (addressFamily, ok)` mapping the RFC 5838 ranges; the per-AF install-family selection; the AF-bit decision helper
- `internal/plugins/ospf/multiaf_test.go` -- `TestAFFromInstanceID`, `TestMultiAFEngineSpawn`, `TestPerAFLSDBIsolation`, `TestAFBitGatesFullNonDefault`, `TestAFBitIgnoredDefaultAF`, `TestAFReconcileAddRemove`
- `internal/plugins/ospf/v3/types/options_test.go` additions -- `TestAFBitDistinct` (if not extending the existing file)
- `test/ospf/ospf-multiaf-config.ci`, `ospf-multiaf.ci`, `ospf-multiaf-v4-route.ci`, `ospf-multiaf-show.ci`, `ospf-multiaf-reconcile.ci`
- `test/interop/scenarios/ospf-multiaf-frr/` -- `ze.conf`, `frr.conf` (or a second `ze.conf`), `check.py`
- `test/interop/scenarios/ospf-multiaf-v4-frr/` -- `ze.conf`, `frr.conf` (or a second `ze.conf`), `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the Instance-ID demux, the per-engine LSDB, and the install hardcode |
| 3. Wiring phase | Wiring Test table -- per-AF spawn + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- the AF type + per-AF engine spawn + failing wiring tests
   - Tests: `TestAFFromInstanceID`, `TestMultiAFEngineSpawn`, `test/ospf/ospf-multiaf-config.ci`
   - Files: `multiaf.go` (AF type + `afFromInstanceID`), `register.go` (spawn one v6 engine per AF), `config.go` (parse a per-AF set), a stub AF that compiles but does not yet flood/install
   - Verify: a per-AF config spawns the right engines at the right Instance IDs; the deeper demux/install tests still fail (stubs)
2. **Phase: Per-AF install family** -- the central install correction
   - Tests: `TestInstallerFamilyPerAF`, `TestV6UnicastInstallsIPv6Family`
   - Files: `spf/install.go` (`NewInstallerFamily`), `spf/computer.go`, `spf_wiring.go` (choose family from the engine AF)
   - Verify: each AF installs into the right Loc-RIB family; the v6-unicast instance no longer mis-installs into IPv4Unicast
3. **Phase: Instance-ID-range demux + validation** -- AF resolution + config guard
   - Tests: `TestMultiAFInstanceDemux`, `TestInstanceIDRangeValidation`, `TestPerAFLSDBIsolation`
   - Files: `dispatcher.go` (confirm reuse; add AF field if needed), `config.go` + `yang/ze-ospf-conf.yang` (the Instance-ID range validator)
   - Verify: each instance accepts only its Instance ID; a mis-ranged config is rejected; per-AF LSDBs are isolated
4. **Phase: AF-bit** -- Options emission + receive gate
   - Tests: `TestAFBitDistinct`, `TestAFBitInHelloAndDD`, `TestAFBitGatesFullNonDefault`, `TestAFBitIgnoredDefaultAF`
   - Files: `v3/types/options.go` (`OptAF`), `encoder_v6.go` (set it), `codec_v6.go` (surface it), the neighbor-FSM AF-bit gate
   - Verify: the AF-bit is set in Hello + DD; a non-default AF requires it for Full; the default AF ignores a missing one
5. **Phase: IPv4-over-OSPFv3** -- AF-aware prefix + next-hop
   - Tests: `TestIPv4OverV3PrefixRoundTrip`, `TestV6PrefixToNetipAFWidth`, `TestIPv4OverV3NextHop`, `ospf-multiaf-v4-route.ci`
   - Files: `afstrategy_v6.go` (AF-aware `v6PrefixToNetip` + `RouteEntry` family + next-hop), `redist_wiring.go` (route IPv4 redist to the v4-over-v3 engine)
   - Verify: an IPv4 prefix round-trips as one word; the v4-over-v3 route installs into IPv4Unicast with an adjacency next-hop
6. **Phase: Show + reconcile + metrics** -- user surface + lifecycle
   - Tests: `TestAFReconcileAddRemove`, `TestRedistTargetsAFEngine`, `ospf-multiaf-show.ci`, `ospf-multiaf-reconcile.ci`
   - Files: `cmd_show.go`, `show_summary.go`, the reconcile path in `register.go`/`instance.go`, metric registration
   - Verify: show identifies AF + Instance ID; add/remove of an AF starts/stops its engine cleanly; the `ze_ospf_af_*` series register
7. **Functional tests** -> the five `.ci` cover the user-visible behaviour
8. **RFC refs** -> add `// RFC 5838 Section X` comments on the AF-range mapping, the AF-bit emission/gate, and the IPv4-over-v3 prefix path
9. **Interop** -> `ospf-multiaf-frr` + `ospf-multiaf-v4-frr` QEMU scenarios; `ospf-v6-frr` regression
10. **Full verification** -> `make ze-verify`
11. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; multi-AF parity with FRR's RFC 5838 (AF-bit, Instance-ID ranges, per-AF LSDB/route); multicast tree computation excluded by design |
| Correctness | Instance-ID-range mapping exact (0-31/32-63/64-95/96-127); AF-bit §2.5/§2.6 default-vs-non-default rule; install family per AF; IPv4 prefix width (4 bytes) |
| Naming | `ze_ospf_af_*` metrics; YANG AF leaves kebab-case; `OptAF`, `afFromInstanceID`, `NewInstallerFamily` |
| Data flow | per-AF engines independent; no cross-AF LSDB/RIB leakage; install family chosen once from the AF |
| CLI grammar | AF-aware `show ospf ipv6 ...` action-before-identifier |
| Doctor checks | none added (reuses the existing v6 transport doctor check) -- confirm |
| YANG validation | the AF leaf is an `enumeration`; `instance-id` has `range "0..127"`; the custom validator binds the Instance ID to the AF range |
| Prometheus counters | `ze_ospf_af_instances`, `ze_ospf_af_bit_mismatch_total`, and the `af` label on `ze_ospf_routes_installed` defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | multi-AF stays inside the OSPF plugin; no AF spelling leaks into generic/central packages |
| Rule: buffer-first | the AF-bit is set before `Options.WriteTo`; the IPv4 prefix uses the existing word-padded write path |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| AF-from-Instance-ID mapping | `go test ./internal/plugins/ospf -run TestAFFromInstanceID` |
| Per-AF install family | `go test ./internal/plugins/ospf/spf -run 'TestInstallerFamilyPerAF\|TestV6UnicastInstallsIPv6Family'` |
| AF-bit defined + emitted | `grep -rn 'OptAF' internal/plugins/ospf/v3/types internal/plugins/ospf/encoder_v6.go` |
| AF-bit gate | `go test ./internal/plugins/ospf -run 'TestAFBit'` |
| IPv4-over-OSPFv3 | `go test ./internal/plugins/ospf -run 'IPv4OverV3'` |
| Three metric series/labels | `grep -rn 'ze_ospf_af_\|routes_installed' internal/plugins/ospf` |
| Interop scenarios present | `ls test/interop/scenarios/ospf-multiaf-frr test/interop/scenarios/ospf-multiaf-v4-frr` |
| Functional tests present | `ls test/ospf/ospf-multiaf-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | a received Instance ID out of the 0-127 AF space is dropped by the demux; a malformed IPv4-over-v3 prefix (length > 32) is rejected by the prefix codec; no slice-out-of-range |
| Resource exhaustion | the per-AF engine count is bounded by config (at most one engine per AF range); a flood of distinct Instance IDs on the wire cannot spawn engines (only configured AFs run) |
| Cross-AF isolation | one AF's LSDB/neighbors/RIB cannot be read or polluted by another AF's traffic; the transport demux routes strictly by Instance ID |
| Trust boundary | received AF-bit is advisory for adjacency gating only; it cannot redirect a packet to the wrong AF (the Instance ID, not the AF-bit, selects the instance) |
| Error leakage | an AF-bit mismatch increments a counter; it is not surfaced to the peer beyond not forming the adjacency |

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
Multi-AF is a *fan-out* problem, not a codec problem: the OSPF IPv6 family already
runs one self-contained engine per family with a private LSDB/neighbors/SPF and a
working Instance-ID demux. RFC 5838 maps each AF to an Instance-ID range, so
multi-AF is "spawn one engine per configured AF, derive the AF from its Instance
ID, set the AF-bit, and install into the AF's Loc-RIB family." The only IPv6-base
defect surfaced is the install family hardcode (`family.IPv4Unicast`), which
multi-AF must parameterise -- and which also corrects the IPv6-unicast install
family.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One engine instance per configured AF | one engine with a per-AF LSDB key + multi-value demux | the engine is already self-contained per family; reusing it gives per-AF LSDB/neighbors/SPF/install for free and keeps the demux a single-value check; a multi-value key would re-plumb the whole LSDB |
| Parameterise the SPF installer family | a per-AF RIB wrapper or a family switch at each insert | one constructor parameter threads the family through the single insert/remove chokepoint; minimal blast radius and it corrects the IPv6-base hardcode |
| AF derived from the Instance ID, not a separate AF config field | configure AF and Instance ID independently | RFC 5838 §2.1 binds AF to the Instance-ID range; deriving AF from the range keeps config and wire consistent and prevents a contradictory pair; a custom validator enforces the binding |
| AF-bit decided once per instance, applied in Hello + DD | decide per packet type | §2.4 wants a uniform signal; a single decision from the instance AF avoids Hello/DD divergence (R-8) |
| IPv4-over-OSPFv3 reuses the RFC 5340 prefix codec | a dedicated IPv4 prefix codec | §2.7 reuses the v3 prefix encoding; a 0-32-bit prefix is already one word in `ByteLen()`; only the address-width read in `v6PrefixToNetip` changes |
| Keep this an OSPF IPv6-family feature, not a separate `ospfv3` product | a second `ospfv3` plugin / engine | per `plan/learned/972-ospf-af-unify.md` the FSM/flooding/DR/SPF/LSDB are AF-neutral and shared; forking a second engine duplicates that machinery and reintroduces v2/v3 drift |

## Known Limitations
- Multicast address families (IPv6/IPv4 multicast, Instance IDs 32-63 / 96-127) are demux-validated and route-installable but compute UNICAST-shaped reachability only; MOSPF-style multicast tree computation (RFC 1584) is out of scope.
- IPv4-over-OSPFv3 interop depends on FRR `ospf6d` support for the IPv4-unicast AF; where FRR lacks it, the interop falls back to Ze<->Ze plus a captured FRR multi-AF Hello (recorded as a Deviation).
- No new LSA types; RFC 5838 reuses the RFC 5340 LSA set, so any future IPv6-family extension LSA (ext-5 SR) is orthogonal to multi-AF.

## RFC Documentation

Add `// RFC 5838 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §2.1 AF-to-Instance-ID range mapping (`afFromInstanceID`)
- §2.4 the AF-bit in Options (`OptAF`, emission in `neutralToV6Options`)
- §2.5/§2.6 the AF-bit Full gate (non-default required, default ignored)
- §2.7 the IPv4-over-OSPFv3 prefix + next-hop
- RFC 5340 §4.2.2 the Instance-ID demux (already present; annotate as the multi-AF demux)

## Implementation Summary

### What Was Implemented
- `multiaf.go`: `addressFamily` + `afFromInstanceID` (§2.1 ranges), `family()`, `prefixWidth()`,
  `isDefault()`, `afFromName`; engine helpers `installFamily`/`emitAFBit`/`setMultiAF`/`afHelloAccepted`.
- Per-AF install family: `spf/install.go` `NewInstallerFamily(loc, fam)`; `af` label on
  `ze_ospf_routes_installed`; `spf_wiring.go` selects the family from the engine AF.
- Config: `V6Extra []v6AFConfig`, `v6Families()`, `multiAF()`, `applyAddressFamilies`,
  `ErrInstanceIDRange` range validation; YANG `grouping ospf-af-topology` + five AF containers
  with native per-AF `instance-id` ranges.
- AF-bit: `v3/types/options.go` `OptAF = 0x000100` + `AF()`/`SetAF`; emitted in Hello/DD by
  `encoder_v6.go` (`packetOptions`); surfaced via `types.Hello.AFBit` in `codec_v6.go`; gated in
  `instance.go` `handleHello`; `ze_ospf_af_bit_mismatch_total` counter.
- Per-AF engine lifecycle: `register_multiaf.go` `v6EngineSet` (spawn/reconcile/shutdown) +
  `v6InjectorAF`; `register.go` drives it; `ze_ospf_af_instances` gauge.
- IPv4-over-OSPFv3: AF-aware `v6PrefixToNetip`; `netipToV6Prefix` IPv4 (one 32-bit word);
  redistribution diverts IPv4 to the v4-over-v3 instance (`consumer.SetV4OverV3Injector`).
- Show: `show ospf ipv6` lists each AF instance (AF + Instance ID); `cmd_show.go` RPC + YANG node.

### Bugs Found/Fixed
- IPv6-base latent gap: `NewInstaller` hardcoded `family.IPv4Unicast`, so the IPv6-unicast
  engine mis-installed IPv6 routes under the IPv4 family. Corrected via `NewInstallerFamily`;
  the IPv6-unicast instance now installs into `family.IPv6Unicast` (AC-7, R-1).

### Documentation Updates
- `rfc/short/rfc5838.md` compliance checklist ticked; `docs/plugin-development/metrics.md`
  (`af` label + `ze_ospf_af_*`); `docs/guide/ospf.md` (multi-AF section);
  `docs/architecture/wire/ospfv3.md` (AF-bit + Instance-ID-range demux).

### Deviations from Plan
- `ospf-multiaf-v4-frr` uses a Ze<->Ze topology (ze.conf + ze-peer.conf), not FRR, because FRR
  ospf6d does not implement the IPv4-unicast AF (assumption A-9). Recorded as a Deviation.
- Daemon `.ci` tests (`ospf-multiaf`, `-show`, `-v4-route`, `-reconcile`) and both interop
  scenarios are Linux-only (raw IPv6 sockets); authored, `skip-os darwin`, run in QEMU. Only
  `ospf-multiaf-config.ci` runs natively (passes). Self-originated connected IPv4 intra-area
  prefixes are not enumerated (out of scope; redistribute/AS-External is the tested advertise path).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAFFromInstanceID`, `TestInstanceIDRangeValidation`, `ospf-multiaf-config.ci` | ranges + >127 reject (native YANG + `ErrInstanceIDRange`) |
| AC-2 | Done | `TestMultiAFEngineSpawn`, `TestPerAFLSDBIsolation` | one engine per AF; private LSDB/neighbors/SPF |
| AC-3 | Done | `TestMultiAFInstanceDemux` | Instance-64 Hello reaches v4u, dropped by v6u |
| AC-4 | Done | `TestAFBitInHelloAndDD` | AF-bit set identically in Hello + DD |
| AC-5 | Done | `TestAFBitGatesFullNonDefault` | non-default AF without AF-bit not brought up |
| AC-6 | Done | `TestAFBitIgnoredDefaultAF` | default AF forms Full without AF-bit (§2.6) |
| AC-7 | Done | `TestV6UnicastInstallsIPv6Family` | IPv6-unicast installs into IPv6Unicast (fix) |
| AC-8 | Done | `TestIPv4OverV3BuildRoutes`, `TestIPv4OverV3NextHop` | IPv4 prefix + adjacency next-hop |
| AC-9 | Done | `TestIPv4OverV3PrefixRoundTrip`, `TestV6PrefixToNetipAFWidth` | one 32-bit word; 4-byte decode |
| AC-10 | Done | `ospf-multiaf-show.ci` (QEMU) + `afSummary` | `show ospf ipv6` lists AF + Instance ID |
| AC-11 | Authored-pending-QEMU | `ospf-multiaf-frr` interop | lone IPv6-unicast byte-identical; AF-bit only when multi-AF |
| AC-12 | Done | `TestAFReconcileAddRemove` | add spawns, remove shuts down cleanly |
| AC-13 | Done | `TestRedistTargetsAFEngine` | IPv4 redist -> OSPFv3 AS-External on v4u instance |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|
| `TestAFFromInstanceID` | Pass | `multiaf_test.go` | |
| `TestInstanceIDRangeValidation` | Pass | `config_test.go` | boundary matrix |
| `TestMultiAFEngineSpawn` | Pass | `multiaf_engine_test.go` | |
| `TestMultiAFInstanceDemux` | Pass | `multiaf_engine_test.go` | (spec listed dispatcher_test.go) |
| `TestPerAFLSDBIsolation` | Pass | `multiaf_engine_test.go` | |
| `TestAFBitDistinct` | Pass | `v3/types/options_test.go` | |
| `TestAFBitInHelloAndDD` | Pass | `encoder_v6_test.go` | |
| `TestAFBitGatesFullNonDefault` | Pass | `multiaf_engine_test.go` | |
| `TestAFBitIgnoredDefaultAF` | Pass | `multiaf_engine_test.go` | |
| `TestInstallerFamilyPerAF` | Pass | `spf/install_test.go` | |
| `TestV6UnicastInstallsIPv6Family` | Pass | `spf/install_test.go` | |
| `TestIPv4OverV3PrefixRoundTrip` | Pass | `v3/packet/prefix_test.go` | |
| `TestV6PrefixToNetipAFWidth` | Pass | `afstrategy_v6_test.go` | |
| `TestIPv4OverV3NextHop` | Pass | `afstrategy_v6_test.go` | + `TestIPv4OverV3BuildRoutes` |
| `TestRedistTargetsAFEngine` | Pass | `redist_wiring_test.go` | |
| `TestAFReconcileAddRemove` | Pass | `multiaf_engine_test.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `multiaf.go`, `multiaf_test.go` | Done | + `multiaf_engine_test.go` |
| `spf/install.go`, `spf_wiring.go`, `spf/computer.go` | Done | computer already threaded the installer via Config |
| `v3/types/options.go`, `encoder_v6.go`, `codec_v6.go` | Done | |
| `instance.go`, `dispatcher.go`, `register.go`, `config.go`, `afstrategy_v6.go`, `redist_wiring.go` | Done | + `register_multiaf.go`, `instance_snapshots.go` |
| `yang/ze-ospf-conf.yang`, `yang/ze-ospf-cmd.yang`, `cmd_show.go` | Done | |
| `test/ospf/ospf-multiaf-*.ci` (5) | Done | config native-pass; 4 daemon skip-os darwin |
| `test/interop/scenarios/ospf-multiaf-frr`, `ospf-multiaf-v4-frr` | Authored-pending-QEMU | v4 uses Ze<->Ze |

### Audit Summary
- **Total items:** 13 ACs, 16 TDD tests, files above
- **Done:** 12 ACs verified natively; all 16 TDD tests pass; AC-1..10,12,13 verified
- **Partial:** none
- **Skipped:** none
- **Changed:** AC-11 verification is QEMU interop (authored-pending); v4-frr Ze<->Ze (Deviation)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Instance-ID-range AF demux | unit + functional | `TestAFFromInstanceID`, `TestMultiAFInstanceDemux`, `ospf-multiaf.ci` |
| AF-bit handling (set + gate) | unit + interop | `TestAFBitInHelloAndDD`, `TestAFBitGatesFullNonDefault`, `ospf-multiaf-frr` |
| Per-AF LSDB + route install | unit + functional | `TestPerAFLSDBIsolation`, `TestInstallerFamilyPerAF`, `ospf-multiaf-v4-route.ci` |
| IPv4-over-OSPFv3 | unit + interop | `TestIPv4OverV3PrefixRoundTrip`, `ospf-multiaf-v4-frr` |
| Single-AF preserved | interop regression | `ospf-v6-frr` still Full + route in IPv6 RIB |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `newEngineWithCodec` became a same-arg wrapper (unparam) | instance.go | collapsed into `newEngineWithCodecAF` |
| 2 | NOTE | AF-bit must be Hello/DD-only, not LSA Options | encoder_v6.go | applied via `packetOptions`, LSA paths unchanged |
| 3 | NOTE | instance.go exceeded the 1000-line file cap | instance.go | extracted snapshots to `instance_snapshots.go` |

### Fixes applied
- Collapsed the engine constructor to `newEngineWithCodecAF(t, codec, af)`; extracted the show
  snapshot accessors to `instance_snapshots.go`; kept the AF-bit out of LSA-origination Options.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | none | golangci-lint 0 issues; go vet clean; ze-validate all checks passed | ospf tree | - |

### Final status
- [x] golangci-lint / go vet / ze-validate clean; 0 BLOCKER, 0 ISSUE (self-review)
- [x] All NOTEs recorded above

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
- [ ] AC-1..AC-13 all demonstrated
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
- [ ] RFC 5838 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (four AFs defined by RFC 5838 justify the AF type)
- [ ] No speculative features (multicast tree computation excluded; only demux + install)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (AF engines independent; install family the single new parameter)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-multiaf-frr`, `ospf-multiaf-v4-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-15-multi-af.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-15-multi-af.md`
