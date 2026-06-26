# Spec: ospf-ext-12 -- OSPFv2 Multi-Instance (RFC 6549)

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
3. `rfc/short/rfc6549.md` -- the Instance ID is the high octet of the former 16-bit AuType (header offset 14); AuType keeps the low octet (offset 15); a received packet whose Instance ID matches none of the receiving interface's configured Instance IDs MUST be discarded (§2, §3.1); local subnet significance only, never in LSAs; default 0 (§3); legacy routers read the two octets as a 16-bit AuType and drop non-zero instances at auth (§5, §6)
4. `rfc/short/rfc2328.md` -- App A.3.1 OSPFv2 common header (the AuType field this RFC splits); §9 conceptual interface data structure (the OSPFv2 Interface Instance ID is a new member of it, default 0)
5. `plan/spec-ospf-0-umbrella.md` -- delivered OSPFv2 umbrella; the common-header layout, the Codec seam (AF-neutral `Header`), and the note that the engine already runs a second (OSPFv3) family as a separate engine instance
6. `internal/plugins/ospf/packet/header.go` -- the OSPFv2 common-header codec: `AuType uint16` at `offAuType = 14`, `DecodeHeader`/`Header.WriteTo`, the offset table this spec splits
7. `internal/plugins/ospf/codec.go` -- the AF-neutral `Header{... InstanceID uint8}` and `v4Codec.DecodeHeader` (today returns `InstanceID: 0`)
8. `internal/plugins/ospf/dispatcher.go` -- `dispatcher.instanceID uint8` + the `h.InstanceID != instanceID` discard (today wired for OSPFv3 only via `instance.go`)
9. `internal/plugins/ospf/instance.go` -- `newEngine`/`newEngineWithCodec`, `setConfig` (sets `e.dispatch.instanceID` for the v6 engine only), the per-engine model
10. `internal/plugins/ospf/register.go` -- `runOSPFEngine`, `newEngine` (v4), `newEngineWithCodec` (v6); the place a per-instance v4 engine set is created
11. `internal/plugins/ospf/iface/iface.go` -- `Config` (per-interface), the v4 Hello encoder, `EncodeHello`; the seam where the per-interface Instance ID must reach transmit
12. `internal/plugins/ospf/neighbor/neighbor.go` -- `encodeV4`, the DD/LSReq/LSUpdate `packet.Header{}` build on transmit
13. `internal/plugins/ospf/packet/auth_verify.go` -- the auth path reads/uses the AuType octet(s); splitting AuType to 8 bits touches it

## Task

Add OSPFv2 Multi-Instance support (RFC 6549, which updates RFC 2328) to the
native OSPFv2 plugin at `internal/plugins/ospf/`. RFC 6549 carves an 8-bit
Instance ID out of the OSPFv2 common header by splitting the previously 16-bit
Authentication Type (AuType) field into an 8-bit Instance ID (the high octet, at
header byte offset 14) and an 8-bit AuType (the low octet, offset 15). The
Instance ID lets several independent OSPFv2 protocol instances coexist on one
physical interface / subnet: a router transmits each instance's packets with that
instance's configured Interface Instance ID in the header, and **discards any
received packet whose Instance ID does not match one of the Instance IDs
configured for the receiving interface** (§2, §3.1). The Instance ID has local
subnet significance only: it lives only in the packet header, is never carried in
an LSA, and is never compared across links.

The umbrella (`plan/spec-ospf-0-umbrella.md`) delivered single-instance OSPFv2
plus a second OSPFv3 family that already runs as a *separate engine instance*
selected by a codec, and the AF-neutral `Header` already carries an `InstanceID`
field consumed by a dispatcher discard rule -- but that machinery is wired for the
OSPFv3 Instance ID (RFC 5340 §2.5, a different header location) and the OSPFv2
codec hard-codes `InstanceID: 0`. What is missing is the OSPFv2 wire change (the
AuType/Instance-ID split in `packet/header.go`), per-interface Instance ID
configuration, instance-aware demux on receive, instance-tagged transmit, and the
ability to stand up more than one OSPFv2 engine bound to the same interface set,
each owning one Instance ID.

This is a wire-format change to a field every OSPFv2 router parses, so interop
care is mandatory: Instance ID 0 (the default) must be bit-for-bit identical to
today's behavior so a Ze router with no multi-instance config interoperates
unchanged with legacy OSPFv2 routers (FRR `ospfd`), and a non-zero Instance ID
must be transmitted and demultiplexed correctly against another RFC 6549 speaker.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Header Instance ID encode/decode | Split the OSPFv2 common-header AuType field: read offset 14 as an 8-bit Instance ID and offset 15 as an 8-bit AuType in `packet.DecodeHeader`; write both octets in `Header.WriteTo`. Add `InstanceID uint8` to `packet.Header`; narrow `AuType` to its 8-bit role on the wire while keeping the existing `AuType` constants |
| AF-neutral surfacing | `v4Codec.DecodeHeader` projects the OSPFv2 header Instance ID into the existing `Header.InstanceID` field (today hard-coded 0), so the shared dispatcher demux sees it the same way it sees the OSPFv3 one |
| Per-interface Instance ID config | A per-interface OSPFv2 `instance-id` YANG leaf (default 0), parsed into the per-interface config and the engine's `InstanceID`; the existing OSPFv3 `instance-id` leaf under `address-family ipv6` is the precedent |
| Instance-aware packet demux + neighbor matching | The dispatcher discard rule (`h.InstanceID != instanceID`) becomes active for the OSPFv2 engine; a received packet whose Instance ID does not match the engine's configured Instance ID is dropped before any handler / neighbor FSM runs, so neighbors only form within a matching instance |
| Instance-tagged transmit | The per-interface Instance ID reaches every OSPFv2 transmit path (Hello via the v4 Hello encoder, DD/LSReq/LSUpdate/LSAck via `encodeV4`), so every outgoing packet carries the engine's Instance ID |
| Multiple-instance lifecycle | `register.go` stands up one OSPFv2 engine per configured Instance ID (mirroring the existing v4/v6 two-engine pattern), each with its own LSDB/SPF/neighbor tables, each bound to the same transport but demuxed by Instance ID; add/remove an instance at config reload |
| Auth-path adjustment | The auth verification path (`packet/auth_verify.go`) reads AuType from offset 15 (8-bit) instead of treating offsets 14-15 as a 16-bit AuType, so cryptographic/simple auth is unaffected for Instance ID 0 and correct for non-zero instances |
| Backward compatibility | Instance ID 0 is bit-for-bit identical to today on the wire; a single-instance config produces exactly today's packets and parses today's packets unchanged |

### Out of scope (noted so it is not silently assumed done)

| Item | Why |
|------|-----|
| Multi-Topology Routing (MTR / RFC 4915) | Explicitly excluded by the task; the three reserved Instance IDs (0/1/2) are analogous to MTR topology IDs (§3.2) but this spec only demultiplexes instances, it does NOT compute per-topology routes |
| OSPFv3 Instance ID changes (RFC 5340 §2.5) | Already delivered by the umbrella; this spec reuses the same AF-neutral demux but does not alter the v6 codec or its header location |
| SNMP notification filtering (RFC 3413 §6) | RFC 6549 §6 recommends it to damp legacy-router auth-failure notifications; Ze has no OSPF SNMP MIB surface, so there is nothing to filter (recorded as a Known Limitation, not implemented) |
| New Instance ID IANA semantics | Ze accepts any 8-bit value; it does not enforce the 0/1/2 "Base" meanings or the 3-127 Private / 128-255 Standards-Action policy (operational config, not protocol) |
| Sharing one engine across instances | Rejected design (see Key Design Decisions); each instance is a full, isolated OSPF database, mirroring the per-family engine model |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -->
- [ ] `docs/research/ospf-implementation-guide.md` lines 1570-1572 ("OSPFv2 Multi-Instance (RFC 6549)") -- the feature description and the explicit warning that the wire change is one byte but the hard part is plumbing the Instance ID through interface binding, neighbor matching, and configuration
  -> Decision: treat this as an edge feature on the codec + interface enrollment + a per-instance engine, NOT a new LSA, packet type, or SPF change -- the guide and RFC both frame it as a header/interface-binding feature
  -> Constraint: "the hard part is plumbing the Instance ID through the interface binding, neighbor matching, and configuration surfaces" -- the spec's risk surface is the plumbing (demux, transmit tagging, multi-engine lifecycle), not the wire split
- [ ] `plan/spec-ospf-0-umbrella.md` "Shared Contracts" + the address-family engine model -- the contracts this feature extends
  -> Constraint: the OSPFv2 common-header layout and the Codec seam are umbrella contracts; the AuType field already exists at offset 14; this spec splits that field and surfaces the high octet through the existing AF-neutral `Header.InstanceID`, with no new key type and no new packet type
  -> Decision: reuse the per-family "separate engine instance, demuxed by a codec/Instance ID" pattern the umbrella already established for OSPFv3 -- a second OSPFv2 instance is another engine, not a re-entrant single engine
- [ ] `ai/rules/buffer-first.md` -- header encode is buffer-first
  -> Constraint: `Header.WriteTo` already writes the AuType field via `writeUint16(buf, off+offAuType, ...)`; the split writes the Instance ID and AuType as two single-octet writes into the caller-owned buffer, never via slice concatenation, preserving the buffer-first contract
- [ ] `ai/rules/plugin-self-containment.md` -- multi-instance lives entirely inside the OSPF plugin
  -> Constraint: the Instance ID config leaf, the engine-per-instance lifecycle, and the demux all live under `internal/plugins/ospf/`; no Instance-ID spelling appears in any generic/central package; removing OSPF removes the feature
- [ ] `ai/rules/config-surface.md` + `ai/rules/config-naming.md` -- the new config leaf
  -> Constraint: `instance-id` is operational routing config (per-interface), so it is a YANG leaf, not an env var; it mirrors the existing OSPFv3 `instance-id` leaf name and `type uint8 { range "0..255"; }` exactly

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6549.md` -- the multi-instance spec
  -> Constraint: §2 / §3.1 -- the Instance ID is the high (first) octet of the former 16-bit AuType field (header byte offset 14); AuType is reduced to 8 bits "without any change in meaning" and occupies the low octet (offset 15); total header size is unchanged
  -> Constraint: §2 / §3.1 (MUST) -- "Received packets with an Instance ID not equal to one of the Instance IDs corresponding to one of the configured OSPFv2 Instances for the receiving interface MUST be discarded"; this is the core demux guard, applied before any other protocol processing
  -> Constraint: §3 -- the OSPFv2 Interface Instance ID is a new member of the RFC 2328 §9 conceptual interface data structure, default 0; on transmit the interface's configured Instance ID is written into the header
  -> Constraint: §2, §3.2 -- the Instance ID has local subnet significance only; it is never carried in an LSA and never compared across links; it is NOT used to place an interface in multiple areas (the OSPFv3 overload), per §1
  -> Constraint: §5, §6 -- a legacy router reads the two octets as a 16-bit AuType, sees a mismatched (large) auth type for any non-zero Instance ID, and drops the packet at authentication; this is the intended isolation and means Instance ID 0 MUST stay bit-for-bit compatible
  -> Constraint: §8 -- AuType is now 8 bits; values 256-65535 of the former 16-bit space are deprecated; Ze's `AuType` constants (0-3) are unaffected
- [ ] `rfc/short/rfc2328.md` -- the base OSPFv2 spec this RFC updates
  -> Constraint: App A.3.1 -- the OSPFv2 common header is 24 octets: Version, Type, Length, Router ID, Area ID, Checksum, AuType (the field split here), Authentication (8 octets); the split does not change any offset except reinterpreting offset 14
  -> Constraint: §9 -- the per-interface data structure (area, Hello/Dead intervals, cost, etc.) gains the Interface Instance ID; per §13 flooding and §10 the neighbor FSM are per-interface/per-instance, so instances do not share neighbor state

**Key insights:**
- The wire change is genuinely one byte: reinterpret header offset 14 as Instance ID and offset 15 as AuType. Ze's codec currently reads both as a single `uint16` AuType; the split is local to `packet/header.go` and the auth path.
- The AF-neutral plumbing already exists: `Header.InstanceID`, the dispatcher discard, and the per-family engine model were all built for OSPFv3. This spec activates them for OSPFv2 by (a) making the v4 codec surface the Instance ID instead of 0, and (b) running one engine per configured Instance ID.
- Default 0 is the compatibility anchor: a config with no `instance-id` (or `instance-id 0`) must emit and accept today's exact bytes. The interop test must prove a legacy FRR adjacency is unaffected at Instance ID 0 and isolated at non-zero.
- "Multiple instances on one interface" maps onto Ze's model as "multiple engines bound to the same transport, each demuxing its own Instance ID" -- not a re-entrant single engine. This keeps each instance's LSDB/SPF/neighbor tables fully isolated, exactly as RFC 2328 §9 requires.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/packet/header.go` -- the OSPFv2 common-header codec. `AuType uint16` (24-31); `Header` struct carries `AuType AuType` and `Auth AuthField` (79-88); offset table has `offAuType = 14` (40-49); `DecodeHeader` reads `AuType: AuType(readUint16(buf, offAuType))` (156); `Header.WriteTo` writes `writeUint16(buf, off+offAuType, uint16(h.AuType))` (170). `CommonHeaderLen = 24` is unchanged
  -> Constraint: today offsets 14-15 are one 16-bit AuType; this spec reads/writes offset 14 as Instance ID and offset 15 as AuType. The `AuType` *constants* (0-3) are unchanged; only the wire width and the `Header` struct gain an `InstanceID` field
  -> Constraint: `DecodeHeader` returns `(Header, int, error)` and `Header.WriteTo(buf, off) int` -- both signatures must be preserved (callers in `codec.go`, `auth_verify.go`, encode paths depend on them)
- [ ] `internal/plugins/ospf/codec.go` -- the AF-neutral `Header` already has `InstanceID uint8` (49-56); `v4Codec.DecodeHeader` projects the OSPFv2 header but sets no `InstanceID` (so it defaults 0) (94-106); the doc comment already says "OSPFv3's Instance ID ... surfaced here as InstanceID (zero for v2)"
  -> Constraint: the single change here is `v4Codec.DecodeHeader` must copy the decoded `packet.Header.InstanceID` into the neutral `Header.InstanceID`; the `Codec` interface and every other adapter method are untouched
- [ ] `internal/plugins/ospf/dispatcher.go` -- `dispatcher.instanceID uint8` (32); `dispatch` reads `instanceID := d.instanceID` under the RLock and drops on `h.InstanceID != instanceID` (73-76) with an existing RFC 5340 comment; the discard happens after header decode + checksum + handler lookup, before `areaOK`/`authOK`/handler
  -> Constraint: the demux guard already exists and is correct for OSPFv2; the only gap is that for the v4 engine `instanceID` stays 0 and `h.InstanceID` always decodes 0, so the guard is a no-op today. Wiring `e.dispatch.instanceID = cfg.InstanceID` for the v4 engine (see `instance.go`) and surfacing the real `h.InstanceID` (see `codec.go`) activates it. The RFC comment must be widened to cite RFC 6549 §2/§3.1 for OSPFv2
- [ ] `internal/plugins/ospf/instance.go` -- `newEngine(t)` = `newEngineWithCodec(t, v4Codec{})` (89); `setConfig` sets `e.dispatch.instanceID = cfg.InstanceID` ONLY inside `if e.dispatch.codec.IsV6()` (323-326); the engine owns one dispatcher, one LSDB, one neighbor table, one SPF -- i.e. one instance
  -> Constraint: the `IsV6()` guard around `e.dispatch.instanceID = cfg.InstanceID` must be lifted so the v4 engine also adopts its configured Instance ID; the encoder branches stay v6-only (OSPFv2 uses the default v4 encoders, which this spec teaches to tag the Instance ID via the per-interface config, not via a v6-style encoder swap)
  -> Constraint: the engine is single-instance by construction; multi-instance = multiple engines, so the lifecycle work is in `register.go`/config, not a re-entrant engine
- [ ] `internal/plugins/ospf/register.go` -- `runOSPFEngine` builds the v4 engine via `newEngine(transport.New(...))` (213) and the v6 engine via `newEngineWithCodec(ospfv3transport.New(...), v6Codec{})` (227); both share the process and the config feed
  -> Constraint: this is the multi-instance lifecycle seam: instead of exactly one v4 engine, build one v4 engine per configured OSPFv2 Instance ID, each bound to the OSPFv2 transport, each demuxing its own Instance ID; the v6 engine is unchanged
- [ ] `internal/plugins/ospf/iface/iface.go` -- per-interface `Config` (70+) with `AreaID`, `Cost`, `HelloInterval`, `Priority`, etc., no Instance ID; the v4 Hello encoder builds `packet.Packet{Header: packet.Header{RouterID, AreaID}, Hello: &h}` (50-51) and `EncodeHello` is called with `i.cfg.RouterID, i.cfg.AreaID` (491); the interface owns Hello transmission and the neighbor table per interface
  -> Constraint: the per-interface `Config` gains an `InstanceID uint8`; the v4 Hello encoder must set `packet.Header.InstanceID` so every Hello carries the instance's ID; the encoder signature (`EncodeHello(routerID, areaID, h)`) either gains the Instance ID or the encoder is constructed with it (mirroring `v6Encoder{instanceID: ...}`)
- [ ] `internal/plugins/ospf/neighbor/neighbor.go` -- `encodeV4(p packet.Packet)` (126) is the OSPFv2 DD/LSReq/LSUpdate encode helper; callers build `packet.Header{RouterID, AreaID}` with no Instance ID (115-123); the neighbor table is per interface and carries the FSM
  -> Constraint: the DD/LSReq/LSUpdate/LSAck transmit paths must stamp the engine's Instance ID into `packet.Header.InstanceID`; like the Hello encoder, the cleanest seam is to carry the Instance ID on the encoder/sender the neighbor table already holds (the v6 path already threads `instanceID` through `v6Encoder`)
- [ ] `internal/plugins/ospf/packet/auth_verify.go` -- the cryptographic/simple auth verifier reads the AuType and the 8-octet Auth field from the raw payload; it operates on offsets within the common header and the trailer
  -> Constraint: auth reads the AuType as the value at offset 15 (8-bit) once split; for AuType 0/1/2/3 (Ze's supported set) the low octet already holds the meaningful value, so Instance ID 0 is unchanged; the verifier must not treat offsets 14-15 as one 16-bit value once the split lands
- [ ] `internal/plugins/ospf/config.go` -- `ospfConfig.InstanceID uint8` (191-192, documented as the OSPFv3 Instance ID, 0 for v4) parsed from `tree["instance-id"]` (348-349) within the `address-family ipv6` subtree; the v4 config has no Instance ID today
  -> Constraint: the v4 config gains a per-interface Instance ID source (the new `instance-id` interface leaf); because each OSPFv2 instance is a separate engine, the parser must produce one v4 `ospfConfig` per distinct Instance ID found across the interfaces (or per explicit instance block, depending on the chosen config shape -- see Data Flow)
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- per-interface OSPFv2 config is `container interfaces { list interface { ... } }` (164-191) with `network-type`, `cost`, `hello-interval`, `priority`, `passive`; the OSPFv3 `instance-id` leaf is at line 262 under `address-family ipv6` as `type uint8 { range "0..255"; } default 0`
  -> Constraint: add an `instance-id` leaf to the OSPFv2 per-interface `list interface` (lines 164-191) with the identical type/range/default as the v6 one; this is the per-interface Interface Instance ID of RFC 6549 §3

**Behavior to preserve:**
- Instance ID 0: the OSPFv2 common header is bit-for-bit identical to today (the high octet of AuType is already 0 in practice for Ze's `AuType` 0-3), so `DecodeHeader`/`Header.WriteTo`/auth/checksum produce and accept the exact same bytes; all existing OSPFv2 functional and interop tests pass unchanged.
- `DecodeHeader(buf) (Header, int, error)` and `Header.WriteTo(buf, off) int` signatures; the `Codec` interface; `encodeV4`; `EncodeHello`'s caller contract.
- The OSPFv3 engine, its codec, its Instance ID handling, and the `v6Encoder` Instance-ID threading -- all unchanged.
- The dispatcher discard ordering (decode -> checksum -> handler lookup -> Instance ID -> area -> auth -> handler).
- Per-interface neighbor isolation and the §13 flooding / §10 FSM (instances get full, separate copies, not shared state).

**Behavior to change:** (all RFC-6549-required, not discretionary)
- `packet/header.go`: split offset 14 into Instance ID (offset 14) + AuType (offset 15); add `InstanceID uint8` to `packet.Header`; read/write both octets.
- `codec.go`: `v4Codec.DecodeHeader` surfaces the decoded Instance ID into `Header.InstanceID` (was hard-coded 0).
- `instance.go`: lift the `IsV6()` guard so the v4 engine adopts `cfg.InstanceID` for its dispatcher demux.
- `register.go`/`config.go`: build one OSPFv2 engine per configured Instance ID; reconcile the set on config reload.
- `iface/iface.go` + `neighbor/neighbor.go`: stamp the engine's Instance ID into every transmitted OSPFv2 packet header (Hello + DD/LSReq/LSUpdate/LSAck).
- `auth_verify.go`: read AuType from the low octet (offset 15) instead of a 16-bit value.
- YANG: add the per-interface OSPFv2 `instance-id` leaf.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Receive:** an OSPFv2 datagram arrives on the OSPFv2 transport -> `transport` -> `dispatcher.dispatch(rp)`. `codec.DecodeHeader` now fills `Header.InstanceID` from header offset 14. The dispatcher of each per-instance engine drops the packet unless `h.InstanceID == engine.dispatch.instanceID`.
- **Transmit:** an interface's Hello timer fires, or the neighbor FSM emits DD/LSReq/LSUpdate/LSAck -> the v4 encoder builds `packet.Packet` with `Header.InstanceID = engine's Instance ID` -> `WriteTo` stamps offset 14 -> the transport sends to AllSPFRouters/AllDRouters or the neighbor.
- **Config:** `ospf { interfaces { interface <name> { instance-id N } } }` (and/or an instance grouping, see below) -> `config.go` -> one `ospfConfig` per distinct Instance ID -> `register.go` stands up / tears down engines.

### Transformation Path
1. **Header decode (changed):** `packet.DecodeHeader` reads offset 14 as `InstanceID uint8` and offset 15 as `AuType` (8-bit), returns a `packet.Header` carrying both. The total header length is unchanged (24 octets).
2. **AF-neutral projection (changed):** `v4Codec.DecodeHeader` copies `packet.Header.InstanceID` into the neutral `Header.InstanceID` (was 0).
3. **Demux (activated):** `dispatcher.dispatch` drops the packet if `h.InstanceID` differs from the engine's configured Instance ID. Because each OSPFv2 Instance ID runs in its own engine, a packet reaches exactly the engine that owns its Instance ID; all other engines drop it. This satisfies the §2/§3.1 MUST-discard.
4. **Per-instance processing (unchanged spine):** the matching engine runs the existing area check, auth, and per-type handler; the neighbor FSM, LSDB install, flooding, and SPF are exactly today's code, scoped to that engine's instance.
5. **Transmit tagging (changed):** the per-interface `Config.InstanceID` (sourced from the engine's Instance ID) is carried on the v4 Hello encoder and the neighbor-table sender; `packet.Header.InstanceID` is set on every outgoing packet, so `Header.WriteTo` stamps offset 14.
6. **Auth (adjusted):** `auth_verify.go` reads/uses the 8-bit AuType at offset 15; for Instance ID 0 and Ze's AuType set (0-3) the bytes and the digest inputs are identical to today.
7. **Lifecycle (changed):** `register.go` computes the set of configured OSPFv2 Instance IDs from config; for each it builds an engine bound to the shared OSPFv2 transport with that Instance ID; on reload it adds engines for new Instance IDs and tears down engines for removed ones, each releasing its own neighbor/LSDB state.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire <-> OSPFv2 header | `packet.DecodeHeader`/`Header.WriteTo` split offset 14 (Instance ID) / offset 15 (AuType) | [ ] |
| OSPFv2 header <-> AF-neutral Header | `v4Codec.DecodeHeader` surfaces `InstanceID` (was 0) | [ ] |
| AF-neutral Header <-> demux | `dispatcher.dispatch` `h.InstanceID != instanceID` discard, now active for v4 | [ ] |
| Config <-> engine set | one `ospfConfig` per Instance ID; `register.go` engine-per-instance lifecycle | [ ] |
| Engine Instance ID <-> transmit | per-interface `Config.InstanceID` -> v4 Hello encoder + neighbor sender -> `packet.Header.InstanceID` | [ ] |
| AuType field <-> auth path | `auth_verify.go` reads the 8-bit AuType at offset 15 | [ ] |

### Integration Points
- `internal/plugins/ospf/packet` -- `Header`/`DecodeHeader`/`Header.WriteTo` (the wire split); `auth_verify.go` (8-bit AuType read).
- `internal/plugins/ospf/codec.go` -- `v4Codec.DecodeHeader` surfaces the Instance ID; the neutral `Header.InstanceID` is the shared carrier.
- `internal/plugins/ospf/dispatcher.go` -- the demux discard, activated for v4.
- `internal/plugins/ospf/instance.go` -- `setConfig` adopts `cfg.InstanceID` for the v4 engine; otherwise unchanged.
- `internal/plugins/ospf/register.go` + `config.go` -- the engine-per-instance lifecycle and the config-to-instance-set derivation.
- `internal/plugins/ospf/iface/iface.go` + `internal/plugins/ospf/neighbor/neighbor.go` -- transmit tagging.
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the per-interface `instance-id` leaf.

### Architectural Verification
- [ ] No bypassed layers (Instance ID flows wire -> codec -> AF-neutral Header -> demux, the same spine the OSPFv3 Instance ID already uses)
- [ ] No unintended coupling (no Instance-ID spelling outside `internal/plugins/ospf/`; instances are isolated engines, no shared LSDB/neighbor state)
- [ ] No duplicated functionality (reuses the existing `Header.InstanceID`, the dispatcher discard, and the per-family engine model; adds only the wire split, the config leaf, transmit tagging, and the multi-engine lifecycle)
- [ ] Zero-copy / buffer-first preserved (the header split is two single-octet reads/writes into the existing buffer; no new allocation on the wire path)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The OSPFv2 Instance ID is the HIGH octet of the former 16-bit AuType (header offset 14); AuType keeps the low octet (offset 15) | `rfc/short/rfc6549.md` §2 wire diagram + offset table ("Instance ID" at offset 14, AuType at offset 15) | the byte split is wrong and every multi-instance packet mismatches | `TestHeaderInstanceIDSplit` pins offset 14 = Instance ID, offset 15 = AuType against the RFC byte layout; interop with FRR/BIRD at non-zero | unvalidated |
| A-2 | Instance ID 0 is bit-for-bit identical to today's OSPFv2 header (Ze's AuType 0-3 already leave the high octet zero) | `packet/header.go` writes `uint16(h.AuType)` with `AuType` in 0-3, so offset 14 is already 0 | a single-instance Ze regresses against legacy peers | `TestHeaderInstanceZeroUnchanged` (round-trips a golden today-shaped header byte-for-byte); existing OSPFv2 interop suite stays green | unvalidated |
| A-3 | The AF-neutral `Header.InstanceID` + the dispatcher discard rule (built for OSPFv3) work unchanged for OSPFv2 once the v4 codec surfaces the Instance ID and the engine adopts it | `codec.go` `Header.InstanceID`; `dispatcher.go` `h.InstanceID != instanceID`; `instance.go` v6-only wiring | the demux needs new plumbing rather than activation | `TestDispatchDropsMismatchedInstance` (v4 engine drops non-matching Instance ID) | unvalidated |
| A-4 | Each OSPFv2 instance is a full, isolated engine (separate LSDB/SPF/neighbor), mirroring the per-family engine model, NOT a re-entrant single engine | `instance.go` engine = one dispatcher/LSDB/neighbor/SPF; `register.go` already builds two engines (v4 + v6) | a re-entrant engine is needed; far larger change and shared-state hazards | `TestTwoInstancesIsolatedLSDB` (two engines on one transport keep separate databases); `register.go` builds N engines | unvalidated |
| A-5 | The per-interface Instance ID can be stamped on transmit by carrying it on the v4 Hello encoder + the neighbor sender, exactly as `v6Encoder{instanceID}` already does for OSPFv3 | `iface/iface.go` v4 Hello encoder; `neighbor/neighbor.go` `encodeV4`; `encoder_v6.go` `v6Encoder{instanceID}` precedent | transmit tagging needs a deeper rework of the encode seam | `TestHelloCarriesInstanceID`, `TestDBDescCarriesInstanceID` | unvalidated |
| A-6 | Narrowing AuType to 8 bits on the wire does not break Ze's auth (AuType 0-3); the verifier reads the meaningful value from offset 15 | `packet/auth_verify.go` operates on AuType 0-3; the low octet already holds 0-3 | crypto/simple auth breaks for non-zero instances or even Instance ID 0 | `TestAuthUnaffectedByInstanceSplit` (AuType 2 digest identical pre/post split at Instance ID 0); existing auth tests stay green | unvalidated |
| A-7 | The transport can be shared by multiple v4 engines (each receives every datagram and demuxes by Instance ID), so no per-instance socket is needed | `register.go` shares one transport per family; multicast OSPFv2 packets reach every instance on the subnet (§6) | a per-instance transport/socket is needed; lifecycle complexity | `TestSharedTransportFanOut` (one datagram fans to all v4 engines; only the matching one processes it) | unvalidated |
| A-8 | A legacy OSPFv2 router (FRR ospfd) drops a non-zero-Instance-ID packet at authentication (reads it as a 16-bit AuType mismatch), so non-zero instances are isolated from legacy peers without extra mechanism | `rfc/short/rfc6549.md` §5, §6 | non-zero packets leak into a legacy peer's processing or break its adjacency unexpectedly | interop: FRR at Instance ID 0 stays adjacent; FRR sees a Ze non-zero-instance packet and drops it (no adjacency, no crash) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The AuType/Instance-ID byte order is swapped (Instance ID written to offset 15) -> every multi-instance packet mismatches and silently no adjacency forms | non-zero instances never form adjacency even with another Ze; `tcpdump` shows the wrong octet set | `TestHeaderInstanceIDSplit` pins both octets against the RFC diagram; interop decode of an FRR/BIRD non-zero capture | 
| R-2 | Instance ID 0 regresses (the split changes today's bytes) and breaks legacy OSPFv2 interop | the existing `ospf-*-frr` interop suite fails; golden header bytes change | `TestHeaderInstanceZeroUnchanged` golden round-trip; run the full existing OSPFv2 interop suite as a gate before claiming done |
| R-3 | Auth breaks because the verifier still treats offsets 14-15 as a 16-bit AuType | AuType 2/3 interop or unit auth tests fail after the split | adjust `auth_verify.go` in the same phase as the header split; `TestAuthUnaffectedByInstanceSplit` |
| R-4 | Two v4 engines on one transport race on shared transport state or double-process a datagram | duplicate Hellos / duplicate LSAs in logs; data race under `-race` | each engine demuxes independently; the transport fan-out is read-only per engine; `TestSharedTransportFanOut` run under `-race`; if the transport cannot be shared, fall back to per-instance transport binding |
| R-5 | Engine lifecycle leak: removing an Instance ID from config leaves its engine (and neighbors/LSDB) running | a removed instance keeps sending Hellos; stale neighbors persist | `register.go` reconciles the engine set on reload (add new, tear down removed); `TestInstanceRemovedTearsDown` |
| R-6 | A non-zero Instance ID Hello reaches the wrong engine's neighbor FSM (demux gap) and forms a cross-instance adjacency | a neighbor appears under the wrong Instance ID; `show ospf neighbor` lists it in the wrong instance | the demux drops before the handler runs; `TestNeighborOnlyFormsWithinInstance` proves no cross-instance adjacency |
| R-7 | Config shape ambiguity: per-interface `instance-id` cannot express two instances sharing one interface (a single interface leaf holds one value) | a user wants two instances on `eth0` and the config cannot express it | the config models instances as the unit (an interface can be enrolled under multiple instances); decide the YANG shape in Phase 0 design and pin it in `TestConfigTwoInstancesOneInterface` |
| R-8 | Decoder panic on a truncated header after the split (off-by-one on the new field) | fuzz crash in `packet` | the split reuses the existing bound-checked `CommonHeaderLen` guard; extend the existing `packet` fuzz target; `TestDecodeHeaderTruncated` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ospf { interfaces { interface eth0 { instance-id 5 } } }` is committed | -> | `config.go` derives an OSPFv2 instance with Instance ID 5; `register.go` stands up an engine for it | `TestConfigSpawnsInstanceEngine` (unit) + `test/ospf/ospf-instance-config.ci` |
| A Hello timer fires on an Instance-ID-5 interface | -> | the v4 Hello encoder stamps `packet.Header.InstanceID = 5`; `Header.WriteTo` writes offset 14 | `TestHelloCarriesInstanceID` (unit) + observed in `ospf-multiinstance-frr` interop |
| An OSPFv2 datagram with Instance ID 5 arrives | -> | `codec.DecodeHeader` surfaces `InstanceID = 5`; the Instance-ID-5 engine's `dispatch` accepts it; all other engines drop it | `TestDispatchDropsMismatchedInstance` (unit) + `test/ospf/ospf-instance-demux.ci` |
| An OSPFv2 datagram with Instance ID 5 arrives at an interface configured only for Instance ID 0 | -> | every engine's `dispatch` drops it (no handler runs, no neighbor forms) | `TestNeighborOnlyFormsWithinInstance` (unit) |
| An Instance ID is removed from config and recommitted | -> | `register.go` tears down that instance's engine and its neighbor/LSDB state | `TestInstanceRemovedTearsDown` (unit) + `test/ospf/ospf-instance-teardown.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An OSPFv2 header with Instance ID `0xAB` and AuType `0x02` | `DecodeHeader` returns `InstanceID == 0xAB` (offset 14) and `AuType == 2` (offset 15); `Header.WriteTo` reproduces both octets exactly (RFC 6549 §2) |
| AC-2 | A header with Instance ID 0 (single-instance / default config) | the encoded 24 octets are bit-for-bit identical to today's OSPFv2 header for the same fields; decoding today's golden header yields `InstanceID == 0` and the same `AuType` as before (compatibility) |
| AC-3 | `v4Codec.DecodeHeader` on a packet whose header Instance ID is N | the neutral `Header.InstanceID == N` (no longer hard-coded 0) |
| AC-4 | A received OSPFv2 packet whose Instance ID does not match the receiving engine's configured Instance ID | the packet is discarded before any handler / neighbor FSM runs; no adjacency forms; the drop counter increments (RFC 6549 §2, §3.1 MUST) |
| AC-5 | An interface configured with `instance-id 5` transmits a Hello/DD/LSReq/LSUpdate/LSAck | every outgoing OSPFv2 packet header carries Instance ID 5 at offset 14 |
| AC-6 | `ospf { interfaces { interface eth0 { instance-id 5 } } }` committed | exactly one OSPFv2 engine bound to Instance ID 5 is created, with its own LSDB/SPF/neighbor table; `show ospf` reflects the instance |
| AC-7 | Two OSPFv2 instances (e.g. IDs 0 and 5) configured on overlapping interfaces | both engines run on the shared transport; each demuxes its own Instance ID; their LSDBs and neighbor tables are fully isolated (no LSA or neighbor crosses instances) |
| AC-8 | An Instance ID present in the running config is removed and the config recommitted | the corresponding engine is torn down, stops transmitting, and releases its neighbors/LSDB; remaining instances are unaffected |
| AC-9 | A Ze interface at Instance ID 0 peers with a legacy OSPFv2 router (FRR) | full adjacency forms exactly as today; the legacy peer sees today's bytes |
| AC-10 | A Ze interface at a non-zero Instance ID shares a subnet with a legacy OSPFv2 router | the legacy router drops Ze's non-zero-instance packets at authentication (16-bit AuType mismatch) and does not form an adjacency or crash; Ze does not form an adjacency with it either (RFC 6549 §5, §6) |
| AC-11 | An AuType-2 (cryptographic) packet at Instance ID 0 | authentication verifies exactly as before the split (the 8-bit AuType at offset 15 is read, the digest inputs are unchanged) |
| AC-12 | A truncated datagram shorter than the 24-octet common header | `DecodeHeader` returns `ErrShortBuffer`; no panic; the split adds no new out-of-bounds read |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a single OSPFv2 instance (no `instance-id`, default 0) and peers with FRR | config -> one v4 engine (Instance ID 0) -> Hello/DD carry Instance ID 0 (today's bytes) -> full adjacency with FRR | `ospf-multiinstance-frr` interop (Instance ID 0 path) + existing OSPFv2 interop suite |
| 2 | Configures `instance-id 5` on a set of interfaces and peers with a second Ze (also Instance ID 5) | config -> v4 engine (Instance ID 5) -> packets tagged 5 -> peer's Instance-ID-5 engine accepts, demuxes, forms adjacency | `test/ospf/ospf-instance-demux.ci` + `ospf-multiinstance-frr` (Ze<->BIRD non-zero) |
| 3 | Runs two instances (0 and 5) on the same interface set | config -> two v4 engines on one transport -> each demuxes its own ID -> `show ospf instance` lists both with separate databases | `test/ospf/ospf-instance-config.ci` (two instances, isolated `show` output) |
| 4 | Removes the Instance-ID-5 instance and recommits | config diff -> `register.go` tears down the Instance-ID-5 engine -> it stops sending Hellos; instance 0 unaffected | `test/ospf/ospf-instance-teardown.ci` |
| 5 | Decodes an OSPFv2 packet with a non-zero Instance ID via the CLI | CLI -> `packet.DecodeHeader` -> Instance ID and AuType rendered as separate fields | `test/ospf/ospf-instance-decode.ci` |
| 6 | Brings a non-zero-instance Ze interface onto a subnet with a legacy FRR router | Ze tags Instance ID N; FRR reads a 16-bit AuType mismatch and drops; no adjacency, no crash on either side | `ospf-multiinstance-frr` interop (legacy-isolation path) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHeaderInstanceIDSplit` | `internal/plugins/ospf/packet/header_test.go` | AC-1, A-1, R-1: offset 14 = Instance ID, offset 15 = AuType, both directions | |
| `TestHeaderInstanceZeroUnchanged` | `internal/plugins/ospf/packet/header_test.go` | AC-2, A-2, R-2: Instance ID 0 round-trips today's golden header byte-for-byte | |
| `TestDecodeHeaderTruncated` | `internal/plugins/ospf/packet/header_test.go` | AC-12, R-8: short buffer returns `ErrShortBuffer`, no panic | |
| `TestAuthUnaffectedByInstanceSplit` | `internal/plugins/ospf/packet/auth_verify_test.go` | AC-11, A-6, R-3: AuType-2 digest identical pre/post split at Instance ID 0 | |
| `TestV4CodecSurfacesInstanceID` | `internal/plugins/ospf/codec_test.go` | AC-3, A-3: `v4Codec.DecodeHeader` returns the real Instance ID, not 0 | |
| `TestDispatchDropsMismatchedInstance` | `internal/plugins/ospf/dispatcher.go` test (`dispatcher_test.go`) | AC-4, A-3, R-6: v4 engine drops a non-matching Instance ID before any handler | |
| `TestNeighborOnlyFormsWithinInstance` | `internal/plugins/ospf/hello_dispatch_test.go` | AC-4, R-6: a mismatched-instance Hello forms no neighbor | |
| `TestHelloCarriesInstanceID` | `internal/plugins/ospf/iface/iface_test.go` | AC-5, A-5: Hello header carries the interface's Instance ID | |
| `TestDBDescCarriesInstanceID` | `internal/plugins/ospf/neighbor/neighbor.go` test (`neighbor_test.go`) | AC-5, A-5: DD/LSReq/LSUpdate/LSAck carry the engine's Instance ID via `encodeV4` | |
| `TestConfigSpawnsInstanceEngine` | `internal/plugins/ospf/config_test.go` | AC-6, A-4: a per-interface `instance-id` yields one v4 engine for that Instance ID | |
| `TestConfigTwoInstancesOneInterface` | `internal/plugins/ospf/config_test.go` | AC-7, R-7: the config shape expresses two instances on one interface | |
| `TestTwoInstancesIsolatedLSDB` | `internal/plugins/ospf/instance_test.go` | AC-7, A-4: two engines on one transport keep separate LSDBs/neighbors | |
| `TestSharedTransportFanOut` | `internal/plugins/ospf/instance_test.go` | A-7, R-4: one datagram fans to all v4 engines; only the matching one processes it (run under `-race`) | |
| `TestInstanceRemovedTearsDown` | `internal/plugins/ospf/register.go` test (`register_test.go`) | AC-8, R-5: removing an Instance ID tears down its engine | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Instance ID (header offset 14) | 0-255 | 255 | N/A | N/A (1 byte) |
| AuType (header offset 15, post-split) | 0-255 on the wire (Ze uses 0-3) | 3 (cryptographic-ESN) | N/A | a value outside 0-3 is an unsupported AuType, handled as today |
| `instance-id` YANG leaf | 0-255 | 255 | N/A (uint8) | rejected by YANG `range "0..255"` |
| Common header length | 24 octets | 24 | a buffer < 24 returns `ErrShortBuffer` | N/A (split adds no length) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-instance-config` | `test/ospf/ospf-instance-config.ci` | `instance-id 5` on interfaces spawns an instance; `show ospf instance` lists it with its own database | |
| `ospf-instance-demux` | `test/ospf/ospf-instance-demux.ci` | two Ze nodes at Instance ID 5 form an adjacency; a node at Instance ID 0 on the same subnet does not | |
| `ospf-instance-teardown` | `test/ospf/ospf-instance-teardown.ci` | removing an Instance ID stops its Hellos and clears its neighbors; other instances unaffected | |
| `ospf-instance-decode` | `test/ospf/ospf-instance-decode.ci` | `ze` decode of a non-zero-instance OSPFv2 packet shows Instance ID and AuType as separate fields | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-multiinstance-frr` | `test/interop/scenarios/ospf-multiinstance-frr/` | FRR `ospfd` (legacy, Instance ID 0 only) + BIRD `ospf` (RFC 6549 multi-instance, non-zero Instance ID) | At Instance ID 0 Ze forms a full adjacency with legacy FRR (unchanged bytes); at a non-zero Instance ID Ze forms an adjacency with BIRD and FRR drops Ze's non-zero packets at auth without crashing (RFC 6549 §5/§6 isolation) | |

> Interop is required: this changes the OSPFv2 common-header wire format (the
> AuType/Instance-ID split) that every OSPFv2 router parses. The raw-IP /
> multicast paths are Linux-only and run as QEMU integration tests
> (`ai/rules/qemu-testing.md`), consistent with the rest of the OSPF interop set.
> BIRD is the multi-instance reference (the guide notes RFC 6549 is implemented in
> BIRD, only partially in FRR), so the non-zero-instance adjacency is validated
> against BIRD and the legacy-isolation path against FRR.

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/packet/header.go` -- add `InstanceID uint8` to `Header`; split offset 14 (Instance ID) / offset 15 (AuType) in `DecodeHeader` and `Header.WriteTo`; keep `CommonHeaderLen = 24` and the offset table (rename/annotate `offAuType` -> the two octets)
- `internal/plugins/ospf/packet/auth_verify.go` -- read AuType as the 8-bit value at offset 15; do not treat offsets 14-15 as a 16-bit AuType
- `internal/plugins/ospf/packet/json.go` -- render Instance ID and AuType as separate fields in the decoded-header JSON (decode CLI output)
- `internal/plugins/ospf/codec.go` -- `v4Codec.DecodeHeader` copies the decoded `packet.Header.InstanceID` into the neutral `Header.InstanceID`
- `internal/plugins/ospf/dispatcher.go` -- widen the Instance-ID discard comment to cite RFC 6549 §2/§3.1 for OSPFv2 (the code is unchanged; it activates once the codec surfaces the ID and the engine adopts it)
- `internal/plugins/ospf/instance.go` -- in `setConfig`, lift the `IsV6()` guard so the v4 engine sets `e.dispatch.instanceID = cfg.InstanceID`; keep the v6 encoder swap v6-only
- `internal/plugins/ospf/register.go` -- build one OSPFv2 engine per configured Instance ID bound to the shared OSPFv2 transport; reconcile the engine set on config reload (add new, tear down removed)
- `internal/plugins/ospf/config.go` -- parse the per-interface `instance-id`; derive the set of OSPFv2 instances and one `ospfConfig` per Instance ID; document the v4 `InstanceID` source (was v6-only)
- `internal/plugins/ospf/iface/iface.go` -- add `InstanceID uint8` to the per-interface `Config`; carry it on the v4 Hello encoder so every Hello header is stamped
- `internal/plugins/ospf/neighbor/neighbor.go` -- carry the engine's Instance ID through `encodeV4` so DD/LSReq/LSUpdate/LSAck headers are stamped
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- add an `instance-id` leaf to the OSPFv2 per-interface `list interface` (lines 164-191), `type uint8 { range "0..255"; } default 0`, mirroring the v6 leaf at line 262
- `internal/plugins/ospf/cmd_show.go` -- surface the per-instance view (the engine's Instance ID) in `show ospf` / a `show ospf instance` summary
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `show ospf instance` command leaf (if a dedicated subcommand is added)
- `internal/plugins/ospf/doctor.go` -- (only if a runtime dependency is added; none expected -- no new socket/port/binary)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- per-interface `instance-id` leaf; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `instance-id` is `type uint8 { range "0..255"; } default 0` (native, mirrors the v6 leaf) |
| YANG custom validators | [ ] no | native uint8 range suffices |
| CLI commands/flags | [ ] yes | `show ospf instance` (or instance column in `show ospf neighbor`/`database`) in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ospf instance` |
| Editor autocomplete | [ ] yes | automatic for the YANG uint8 leaf + the new show subcommand |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-instance-*.ci` |
| Pipe completeness | [ ] yes | `show ospf instance` routes through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | `instance-id` is operational routing config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary/cert; instances share the existing OSPFv2 raw socket/transport |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_instances` | gauge | (number of configured OSPFv2 instances) |
| `ze_ospf_instance_mismatch_drops_total` | counter | `interface` (packets dropped by the Instance-ID demux) |

> These extend the umbrella's canonical OSPF metric set; they use the `ze_ospf_*`
> prefix and are registered by this spec's owner code. The umbrella "Metrics"
> table must gain these rows when this spec lands. The existing per-engine drop
> counter (`dispatcher.droppedCnt`) already increments on a mismatch; the new
> counter labels the Instance-ID-mismatch subset for observability.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPFv2 multi-instance (RFC 6549) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the per-interface `instance-id` leaf |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ospf instance` |
| 4 | API/RPC added/changed? | [ ] no | show RPCs live in the central `ze-show` namespace; document under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains multi-instance |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- a multi-instance section |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` (or equivalent) -- the AuType/Instance-ID header split |
| 8 | Plugin SDK/protocol changed? | [ ] no | no plugin-SDK surface change |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc6549.md` -- flip the Compliance Checklist items to implemented |
| 10 | Test infrastructure changed? | [ ] yes (interop scenario added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPFv2 multi-instance parity with BIRD/FRR |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- the engine-per-instance lifecycle + the demux |
| 13 | Route metadata keys added/changed? | [ ] no | the Instance ID is not a route metadata key |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the two `ze_ospf_instance*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into `packet/header.go`, `codec.go`, `dispatcher.go` |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF interface config/CLI examples against the new `instance-id` leaf |

## Files to Create
- `internal/plugins/ospf/dispatcher_test.go` -- `TestDispatchDropsMismatchedInstance` (if a dispatcher test file does not already exist; otherwise add to the nearest existing test)
- `internal/plugins/ospf/register_test.go` -- `TestInstanceRemovedTearsDown` (if no register test file exists)
- `internal/plugins/ospf/neighbor/neighbor_test.go` -- `TestDBDescCarriesInstanceID` (if no neighbor test file exists)
- `test/ospf/ospf-instance-config.ci`, `test/ospf/ospf-instance-demux.ci`, `test/ospf/ospf-instance-teardown.ci`, `test/ospf/ospf-instance-decode.ci`
- `test/interop/scenarios/ospf-multiinstance-frr/` -- `ze.conf`, `frr.conf`, `bird.conf`, `check.py`

> New unit tests for existing files (`header_test.go`, `auth_verify_test.go`,
> `codec_test.go`, `iface_test.go`, `config_test.go`, `instance_test.go`,
> `hello_dispatch_test.go`) are added to those files, not created fresh; only
> the test files above are net-new.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the AF-neutral `Header.InstanceID`, the dispatcher discard, and the two-engine pattern exist |
| 3. Wiring phase | Wiring Test table -- config -> engine + failing wiring tests |
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

0. **Phase 0: Config-shape design (no code)** -- resolve R-7 before any implementation
   - Decide how the YANG expresses "multiple instances, possibly sharing an interface": a per-interface `instance-id` leaf (one instance per interface) vs an instance grouping that enrolls interfaces. Pin the decision in Key Design Decisions and `TestConfigTwoInstancesOneInterface`.
   - Verify: the chosen shape can express AC-7 (two instances on one interface); the parser-to-engine-set mapping is unambiguous
1. **Phase: Wiring (MANDATORY FIRST)** -- config spawns a demuxing engine; failing wiring tests
   - Tests: `TestConfigSpawnsInstanceEngine`, `TestDispatchDropsMismatchedInstance`, `test/ospf/ospf-instance-config.ci`
   - Files: `config.go` (parse `instance-id`, derive the instance set), `register.go` (engine-per-instance lifecycle skeleton), `instance.go` (lift the `IsV6()` guard so the v4 engine adopts `cfg.InstanceID`)
   - Verify: a non-zero `instance-id` spawns an engine and its dispatcher carries the Instance ID; demux drops mismatches; the deeper transmit/header tests still fail (the wire split is a stub)
2. **Phase: Header split + AF-neutral surfacing** -- the wire change
   - Tests: `TestHeaderInstanceIDSplit`, `TestHeaderInstanceZeroUnchanged`, `TestDecodeHeaderTruncated`, `TestV4CodecSurfacesInstanceID`
   - Files: `packet/header.go` (the offset-14/15 split + `Header.InstanceID`), `codec.go` (`v4Codec.DecodeHeader` surfaces it)
   - Verify: the split pins both octets; Instance ID 0 round-trips today's golden bytes; the v4 codec surfaces the real Instance ID
3. **Phase: Auth-path adjustment** -- keep auth correct under the split
   - Tests: `TestAuthUnaffectedByInstanceSplit`, existing auth tests stay green
   - Files: `packet/auth_verify.go`
   - Verify: AuType-2 digest identical pre/post split at Instance ID 0; non-zero instances read AuType from offset 15
4. **Phase: Transmit tagging** -- every outgoing packet carries the Instance ID
   - Tests: `TestHelloCarriesInstanceID`, `TestDBDescCarriesInstanceID`
   - Files: `iface/iface.go` (per-interface `Config.InstanceID` + v4 Hello encoder), `neighbor/neighbor.go` (`encodeV4` Instance ID)
   - Verify: Hello and DD/LSReq/LSUpdate/LSAck headers carry the engine's Instance ID
5. **Phase: Multi-instance lifecycle + isolation** -- multiple engines, reload reconcile, demux isolation
   - Tests: `TestTwoInstancesIsolatedLSDB`, `TestSharedTransportFanOut` (under `-race`), `TestInstanceRemovedTearsDown`, `TestNeighborOnlyFormsWithinInstance`, `TestConfigTwoInstancesOneInterface`, `ospf-instance-demux.ci`, `ospf-instance-teardown.ci`
   - Files: `register.go` (build N engines, reconcile on reload), `config.go` (instance-set derivation), `instance.go` (teardown path)
   - Verify: two instances stay isolated on one transport; removing an instance tears it down; neighbors only form within an instance
6. **Phase: CLI + metrics** -- user surface
   - Tests: `ospf-instance-config.ci`, `ospf-instance-decode.ci`
   - Files: `cmd_show.go`, `yang/ze-ospf-cmd.yang`, `packet/json.go` (decode shows Instance ID + AuType), metric registration
   - Verify: `show ospf instance` lists instances; decode shows the split fields; the two metric series register
7. **Functional tests** -> the four `.ci` cover the user-visible behaviour
8. **RFC refs** -> add `// RFC 6549 Section 2 / 3.1` comments on the split, the demux discard, and the transmit tagging
9. **Interop** -> `ospf-multiinstance-frr` QEMU scenario (FRR legacy at ID 0, BIRD multi-instance at non-zero)
10. **Full verification** -> `make ze-verify`; run the FULL existing OSPFv2 interop suite as a regression gate for R-2
11. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; Instance ID 0 is bit-for-bit compatible; non-zero demux + isolation works; parity with BIRD's RFC 6549 (the multi-instance reference) |
| Correctness | offset-14 = Instance ID, offset-15 = AuType (byte order); Instance ID 0 unchanged; demux discard before any handler; transmit stamps every packet type; engines isolated; reload reconciles |
| Naming | `instance-id` YANG leaf (kebab-case, mirrors v6); `ze_ospf_instance*` metrics; `InstanceID` field |
| Data flow | Instance ID flows wire -> codec -> AF-neutral Header -> demux; no shared LSDB/neighbor across instances; no Instance-ID spelling outside the OSPF plugin |
| CLI grammar | `show ospf instance` action-before-identifier |
| Doctor checks | none added (no new runtime dependency) -- confirm |
| YANG validation | the `instance-id` leaf is a native `uint8 range "0..255"` |
| Prometheus counters | the two series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | no Instance-ID spelling in any central/generic package; removing OSPF removes the feature |
| Rule: buffer-first | the header split is two single-octet writes into the caller buffer; no new allocation |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Header Instance-ID/AuType split | `go test ./internal/plugins/ospf/packet -run 'Instance'` |
| Instance ID 0 byte compatibility | `go test ./internal/plugins/ospf/packet -run TestHeaderInstanceZeroUnchanged` |
| Demux discard active for v4 | `go test ./internal/plugins/ospf -run TestDispatchDropsMismatchedInstance` |
| Transmit tagging | `go test ./internal/plugins/ospf/iface -run TestHelloCarriesInstanceID` |
| Engine-per-instance lifecycle | `go test ./internal/plugins/ospf -run 'TestTwoInstancesIsolatedLSDB|TestInstanceRemovedTearsDown'` |
| YANG leaf present | `grep -n 'instance-id' internal/plugins/ospf/yang/ze-ospf-conf.yang` (two hits: v4 interface + v6) |
| Two metric series registered | `grep -rn 'ze_ospf_instance' internal/plugins/ospf` |
| Interop scenario present | `ls test/interop/scenarios/ospf-multiinstance-frr/` |
| Functional tests present | `ls test/ospf/ospf-instance-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the header split adds no out-of-bounds read (reuses the `CommonHeaderLen` guard); the existing `packet` fuzz target is extended; a truncated header returns `ErrShortBuffer`, never panics |
| Trust boundary | the Instance-ID demux is a drop filter, not an auth bypass; auth still runs after demux on accepted packets; a non-zero Instance ID does not weaken or skip authentication |
| Resource exhaustion | each instance is a full engine; the number of instances is bounded by config (uint8 range, operator-controlled); a flood of mismatched-Instance-ID packets is dropped cheaply at the demux before any handler allocates |
| Cross-instance isolation | one instance's LSDB/neighbor state cannot be reached or corrupted by another instance's packets (demux + separate engines); a malformed packet for instance A cannot affect instance B |
| Legacy interaction | non-zero-instance packets are intentionally dropped by legacy peers at auth (§5/§6); Ze must not emit a non-zero Instance ID for an interface configured Instance ID 0 (would break legacy interop) |

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
OSPFv2 multi-instance is mostly an *activation* of machinery the OSPFv3 work
already built: the AF-neutral `Header.InstanceID`, the dispatcher discard rule,
and the per-family "separate engine per instance, demuxed by a codec/Instance ID"
model all exist. The genuinely new work is a one-byte wire split in
`packet/header.go` (reinterpret the 16-bit AuType as Instance ID + 8-bit AuType),
the per-interface config leaf, transmit tagging, and a lifecycle that stands up
one OSPFv2 engine per configured Instance ID. The hardest constraint is
non-functional: Instance ID 0 must be bit-for-bit identical to today so legacy
OSPFv2 interop is untouched.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One full engine per OSPFv2 Instance ID, demuxed on a shared transport | A re-entrant single engine keyed by Instance ID internally | RFC 2328 §9 makes each instance a full conceptual interface/LSDB/neighbor set; the umbrella already runs OSPFv3 as a separate engine; per-instance engines reuse all existing per-engine code and guarantee isolation with no shared-state hazards |
| Surface the Instance ID through the existing AF-neutral `Header.InstanceID` + the existing dispatcher discard | A new OSPFv2-specific demux path | The demux is identical to the OSPFv3 one; reusing it avoids duplicate logic and keeps the discard ordering uniform |
| Split offset 14 = Instance ID (high octet), offset 15 = AuType (low octet) | Treat the Instance ID as a separate appended field | RFC 6549 §2 carves the Instance ID out of the existing 16-bit AuType; the header size is unchanged; the split is the literal RFC byte layout |
| Per-interface `instance-id` leaf mirroring the OSPFv3 leaf (subject to Phase 0 confirmation for the multi-instance-on-one-interface case) | A top-level instance list | Keeps naming/typing consistent with the existing v6 leaf; Phase 0 resolves whether one interface can host two instances within the chosen shape |
| Instance ID 0 stays bit-for-bit compatible | Always emit the split unconditionally | §5/§6 require legacy interop at Instance ID 0; the high octet of AuType is already 0 for Ze's AuType 0-3, so 0 is naturally compatible -- but it must be tested, not assumed |

## Known Limitations
- No Multi-Topology Routing: instances are demultiplexed but Ze does not compute per-topology routes (RFC 4915 is out of scope); the three reserved Instance IDs (0/1/2) carry no special semantics in Ze.
- No SNMP notification filtering (RFC 6549 §6): Ze has no OSPF SNMP MIB surface, so there is nothing to filter; legacy-peer auth-failure damping is a non-issue here.
- The Instance ID has local subnet significance only; it is never carried in an LSA and never compared across links (by design, per §2).
- Ze does not enforce the IANA Instance ID semantics (Base 0/1/2, Private 3-127, Standards-Action 128-255); any 8-bit value is accepted as operational config.

## RFC Documentation

Add `// RFC 6549 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §2 / §3.1 the AuType/Instance-ID header split (offset 14 Instance ID, offset 15 AuType) in `packet/header.go`
- §2 / §3.1 (MUST) the demux discard of a non-matching Instance ID in `dispatcher.go`
- §3 the per-interface Interface Instance ID on transmit in `iface/iface.go` + `neighbor/neighbor.go`
- §5 / §6 the Instance-ID-0 backward-compatibility note in `packet/header.go`

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
| Header Instance ID encode/decode (offset 14/15 split) | unit | `TestHeaderInstanceIDSplit`, `TestHeaderInstanceZeroUnchanged` |
| Per-interface Instance ID config spawns an instance | unit + functional | `TestConfigSpawnsInstanceEngine`, `ospf-instance-config.ci` |
| Instance-aware demux + neighbor matching | unit + functional + interop | `TestDispatchDropsMismatchedInstance`, `TestNeighborOnlyFormsWithinInstance`, `ospf-instance-demux.ci`, `ospf-multiinstance-frr` |
| Multiple-instance lifecycle (isolation + teardown) | unit + functional | `TestTwoInstancesIsolatedLSDB`, `TestInstanceRemovedTearsDown`, `ospf-instance-teardown.ci` |
| Backward compatibility at Instance ID 0 | interop | `ospf-multiinstance-frr` (FRR legacy adjacency) + existing OSPFv2 interop suite green |

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
- [ ] AC-1..AC-12 all demonstrated
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
- [ ] RFC 6549 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (multi-instance reuses the per-family engine model; no new abstraction invented)
- [ ] No speculative features (no MTR, no SNMP filtering; only the demux + lifecycle)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (no Instance-ID spelling outside the OSPF plugin)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-multiinstance-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-12-multi-instance.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-12-multi-instance.md`
