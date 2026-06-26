# Spec: OSPF Graceful Restart restarter + helper (both address families)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-ext-0-umbrella.md; spec-ospf-ext-1-opaque-framework.md (IPv4 Grace-LSA carrier only) |
| Phase | - |
| Updated | 2026-06-24 |

> One feature, both address families. Ze implements OSPF as a single unified
> engine (`internal/plugins/ospf/`), exactly as `bgp` is one engine spanning
> families. There is no separate "OSPFv3" plugin and no separate product. The
> IPv4 family is OSPFv2 (RFC 2328); the IPv6 family is OSPFv3 (RFC 5340), run as a
> second instance of the same engine over the v6 codec. Graceful Restart is one
> feature with one config, one CLI surface, and ONE shared control plane (the two
> state machines, the FIB-retention coupling, the NVS restart-fact). Only the wire
> object differs per family: IPv4 carries the Grace-LSA as a link-local Type 9
> opaque LSA (RFC 3623, via the ext-1 opaque framework), and IPv6 carries it as a
> native link-scope OSPFv3 LSA (RFC 5187, function code 11, LS Type 0x000B, added
> in the v3/packet leaf -- there is no opaque carrier in v3). Tables below use an
> "Address family" column or explicit IPv4 / IPv6 sub-rows wherever the two
> families diverge; everything else is stated once because it is shared.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc3623.md` -- Graceful OSPF Restart (the IPv4 feature spec): grace-LSA wire format (§A), restarter Tx/state/exit (§2, §2.1, §2.2, §2.3, §5), helper Rx/state/exit (§3, §3.1, §3.2), timers/constants, the two state-machine tables, the unplanned-outage rules (§5). This is the SHARED control plane both families inherit.
4. `rfc/short/rfc5187.md` -- OSPFv3 Graceful Restart (the IPv6 wire delta): the native Grace-LSA (§2.1 LS Type 0x000B / function code 11, link-local U=0/S2=0/S1=0; §2.2 Link State ID = Interface ID, two TLVs, the dropped router-address TLV), the two OSPFv3 preservation rules (§3.1 LSA-ID->prefix, §3.2 Interface ID), and the "identical to RFC 3623 except the differences" framing (Abstract)
5. `rfc/short/rfc5250.md` -- Opaque-LSA framework (IPv4 carrier reference): Type 9 link-local scope rule (§3.1), the Opaque Type / Opaque ID split (§3 / App A.2), the O-bit DD gate (§3.1)
6. `rfc/short/rfc5340.md` -- OSPFv3 base (IPv6 wire substrate): the 16-bit scope-bearing LS Type (§A.4.2.1 U/S2/S1 + 13-bit function code), the 20-byte LSA header, link-local flooding scope, the Interface ID model (§A.3.2)
7. `plan/spec-ospf-ext-1-opaque-framework.md` -- the IPv4 Grace-LSA carrier: `RegisterOpaqueConsumer(opaqueType, scope, OnOriginate, OnReceive)`, the link-store (Type 9) origination/flood path, the 4-byte-aligned TLV iterator + builder, the O-bit DD negotiation, the verbatim re-flood guarantee (IPv6 does NOT use this)
8. `plan/learned/972-ospf-af-unify.md` -- why OSPF is one engine with Transport/Codec/AFPrefixStrategy seams; the FSM, flooding, DR election, SPF, and LSDB sequencing are AF-neutral and shared; AF-specific wire lives in `_v6` strategy files and the `internal/plugins/ospf/v3/{types,packet,transport}` leaves
9. `internal/plugins/ospf/instance.go` -- the shared engine: `originateSelfLSAs` (the self-LSA chokepoint; dispatches to `lsdb.OriginateFromTopology` for IPv4 and `v6OriginateSelf` for IPv6 via `codec.IsV6()`), `shutdown()`, the boot-count NVS seam (`loadOSPFBootCount`/`openBootCountStore`), `runOSPFEngine` lifecycle
10. `internal/plugins/ospf/spf/install.go` -- `Installer.Apply`/`RemoveAll`: the SHARED route-install seam; the restarter must NOT install/remove routes during restart (FIB retention), both families
11. `internal/plugins/fib/kernel/backend.go` + `backend_linux.go` -- `RTPROT_ZE` (250) marking + `sweepDelay` startup stale-mark-then-sweep: the SHARED mechanism keeping kernel routes (v4 and v6) alive across a control-plane restart
12. `internal/plugins/ospf/neighbor/neighbor.go` + `nsm.go` + `table.go` -- the neighbor record, the shared NSM, and the table the helper freezes at Full; `FloodNeighbor`/`Snapshot` (OSPFv3 keys neighbours by Router ID)
13. `internal/plugins/ospf/lsdb/origination.go` -- `OriginateLinkSelf`/`OriginateSelf`/`SelfLSAEncoder` (link-scope self-origination used by both families); `flushReceivedSelfLSA`/`handleSelfReceived`; `FlushStaleSelfLSAs`
14. `internal/plugins/ospf/lsdb/link_scope.go` -- the shared link-local store (`d.links`, `OriginateLinkSelf`, `installLink`, `LinkLSAs`, `isLinkLSAType`); IPv6 broadens the link-scope predicate so function code 11 routes through the link store
15. `internal/plugins/ospf/v3/types/lsa.go` + `internal/plugins/ospf/v3/packet/lsa.go` + `lsa_link.go` -- the IPv6 wire half: the OSPFv3 `LSType`, `Known()`, `LSAKey`; the typed-body `LSA` with `RawBytes` verbatim passthrough; the link-local Link-LSA precedent the Grace-LSA body mirrors
16. `internal/plugins/ospf/origination_v6_link.go` + `origination_v6.go` -- `v6OriginateLinkLSA` (IPv6 link-scope self-origination), `v6OriginateSelf` (the IPv6 self-LSA chokepoint), `v6OriginateRouter` (the IPv6 Router-LSA topology builder)
17. `internal/plugins/ospf/auth_keystore.go` -- the ZeFS (`pkg/zefs`) NVS persistence pattern (`loadOSPFBootCount`/`openBootCountStore`) the restart-fact NVS storage mirrors (shared)
18. `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` + `yang/ze-ospf-cmd.yang` -- the single OSPF schema; `max-metric` is the precedent for the family-neutral `graceful-restart` container

## Task

Add **OSPF Graceful Restart** to the unified OSPF engine at
`internal/plugins/ospf/`, covering **both address families** (IPv4 = OSPFv2,
RFC 3623; IPv6 = OSPFv3, RFC 5187) and implementing **both roles** for each: the
**restarting router** (restarter) and the **helper neighbor**. Graceful Restart
lets a Ze router restart or reload its OSPF control software while staying on the
forwarding path ("non-stop forwarding"): the restarting router floods link-scope
**Grace-LSAs** asking neighbours to keep advertising it as fully adjacent for a
bounded grace period, preserves its FIB across the control-plane restart, and
re-acquires adjacencies and re-syncs the LSDB without flapping routes; neighbours
that receive a Grace-LSA enter **helper mode**, hold the adjacency at Full and
suppress LSDB churn until the grace period ends or a topology change forces an
early exit.

This is **one feature**, not two. The control-plane behaviour -- the restarter
and helper state machines, the FIB-retention coordination with the fib-kernel
sweep, the in-restart self-LSA suppression, the helper's adjacency freeze, the
NVS restart-fact, the grace timers, the config / CLI / metrics surface -- is
**shared across both families** because the OSPF engine's FSM, flooding, DR
election, SPF, and LSDB sequencing are address-family-neutral (`plan/learned/972`).
RFC 5187 is explicit that "the OSPFv3 graceful restart is identical to that of
OSPFv2 except for the differences described in this document" (Abstract): the
restarter and helper *procedures* are inherited unchanged from RFC 3623. The
**only** material per-family difference is the Grace-LSA wire carriage:

| Address family | Grace-LSA carriage | RFC | TLVs | Link State ID | Carrier |
|----------------|--------------------|-----|------|---------------|---------|
| IPv4 (OSPFv2) | link-local Type 9 **opaque** LSA, Opaque Type 3, Opaque ID 0 | RFC 3623 §A | three (Grace Period type 1, Restart Reason type 2, IP Interface Address type 3 on shared media) | the opaque Type/ID split (3 / 0) | the ext-1 opaque framework (`RegisterOpaqueConsumer(3, link, ...)`) |
| IPv6 (OSPFv3) | **native** link-scope LSA, function code 11, LS Type 0x000B, U=0/S2=0/S1=0 | RFC 5187 §2.1 | two (Grace Period type 1, Restart Reason type 2; **no** IP Interface Address TLV) | the originating interface's OSPFv3 Interface ID | a new native LSA type in `v3/types` + a typed body in `v3/packet` + the link-store predicate broadened for function code 11 |

The IPv4 family is therefore a **consumer of the ext-1 opaque carrier** (it
claims Opaque Type 3, originates through the carrier's link-store
self-origination, receives through the carrier's `OnReceive`, and does NOT
re-implement opaque flooding, the LS-ID split, the O-bit DD gate, or the TLV
iterator/builder). The IPv6 family adds the Grace-LSA as a **first-class v3 LSA**
(RFC 5340 carries extensions as native, scope-aware LSAs; there is **no opaque
carrier in v3**), reusing the delivered v3 Link-LSA store/origination and the
verbatim `RawBytes` passthrough. The two families share **no wire / LSA code**
(the v3 Grace-LSA codec lives entirely under `internal/plugins/ospf/v3/`).

In both families the grace period is measured by the Grace-LSA's LS age (which
MUST start at 0 and MUST NOT be reset on retransmit, and DoNotAge MUST NOT be
set), so the helper's expiry timer reads LS age against the Grace Period TLV, not
a separate clock.

IPv6 adds two **OSPFv3-only preservation rules** with no IPv4 counterpart: the
restarter MUST preserve the LSA-ID->prefix correspondence for Inter-Area-Prefix
and External LSAs (RFC 5187 §3.1, arbitrary 32-bit LSA IDs) and MUST preserve the
OSPFv3 Interface ID across the restart (§3.2), so pre-restart Link / Network /
Router LSAs still match neighbour adjacency state on resume. Both are persisted
in the (shared) restart-fact NVS blob alongside {restarting, grace-end, reason}.

### In scope (this spec)

**Shared control plane (stated once; applies to both families):**

| Item | Detail |
|------|--------|
| Restarter: pre-restart | Ensure FIB current/persistent; originate one Grace-LSA per OSPF interface (LS age 0); reliably flood; persist {restarting, grace-end, reason} (plus, IPv6-only, the §3.1/§3.2 preservation maps) to NVS (ZeFS) (RFC 3623 §2.1) |
| Restarter: in-restart suppression | Suppress self-LSA origination and route install while the grace window is open; run SPF without installing; keep pre-restart received self-LSAs (RFC 3623 §2) |
| Restarter: DR re-election | Re-elect self DR on a segment if a Hello in Waiting state lists self as DR (was DR before restart) (RFC 3623 §2) |
| Restarter: exit | Exit on all-adjacencies-up / inconsistent-LSA / grace-expiry; re-originate self-LSAs, install routes, flush stale self-LSAs, flush own Grace-LSAs (RFC 3623 §2.2, §2.3) |
| Restarter: unplanned outage | Config-gated: on cold/unplanned start, send Grace-LSAs before any Hello, reason restricted to 0 or 3, operator can disable (RFC 3623 §5) |
| Helper: entry checks | Full adjacency, LSDB unchanged since restart, grace period not expired, policy permits, helper not restarting (RFC 3623 §3.1) |
| Helper: while-helping | Continue advertising adjacency to X (Router-LSA / Network-LSA), keep X as DR, suppress LSDB churn for the grace window (RFC 3623 §3) |
| Helper: strict LSA checking | Default-on: a changed LSA that would flood to X terminates helping; config-gated relaxation; stub-area exception for an external change that would not flood to X (RFC 3623 §3.2) |
| Helper: exit | Grace-LSA flushed / grace expiry / topology change -> DR recalc + Router/Network-LSA re-origination (RFC 3623 §3.2) |
| FIB retention coordination | The restarter relies on the existing fib-kernel `RTPROT_ZE` stale-mark-then-sweep; this spec ensures the grace window closes (routes re-installed) before the sweep deadline, and `RemoveAll` on engine stop is NOT invoked on a graceful restart (RFC 3623 §2.1) |
| Grace timers | Grace Period (1-1800 s, suggested default 120 s) measured by Grace-LSA LS age; helper expiry timer; restarter exit timer (RFC 3623 §2.1, §B.1; RFC 5187 §2.2) |
| Config + CLI + metrics | a single family-neutral `graceful-restart` config (restarter support/interval/unplanned, helper support/strict-checking); `show ospf graceful-restart` (IPv4) and `show ospf ipv6 graceful-restart` (IPv6); Prometheus series |

**Per-address-family wire half (labelled):**

| Item | Address family | Detail |
|------|----------------|--------|
| Grace-LSA carriage | IPv4 | Opaque-consumer registration `RegisterOpaqueConsumer(opaqueType=3, scope=link, OnOriginate, OnReceive)` from the plugin's `init()`; origination + reception driven by ext-1; Opaque Type 3, Opaque ID 0, scope link (Type 9) |
| Grace-LSA carriage | IPv6 | a new `LSTypeGrace LSType = 0x000B` (function code 11, link-local) in `v3/types`; a typed `GraceLSA` body + `DecodeGrace()` in `v3/packet`; the LSDB link-scope predicate broadened so function code 11 routes through `installLink`/`OriginateLinkSelf`/`LinkLSAs`; verbatim re-flood via `LSA.RawBytes` |
| Grace-LSA body | IPv4 | three TLVs (Grace Period / Reason / IP Interface Address) built/parsed via the ext-1 4-byte-aligned TLV builder/iterator; type-3 IP Interface Address required on broadcast/NBMA/P2MP (RFC 3623 §A) |
| Grace-LSA body | IPv6 | two TLVs (Grace Period / Reason) built/parsed via a v3 4-octet-aligned TLV codec; NO IP Interface Address TLV (helper keys X by Advertising Router) (RFC 5187 §2, §2.2) |
| Grace-LSA origination | IPv4 | through the ext-1 link-store self-origination (carrier assigns sequence/age, floods to opaque-capable neighbours only) |
| Grace-LSA origination | IPv6 | through `e.lsdb.OriginateLinkSelf` + a `v6OriginHeader(LSTypeGrace, InterfaceID-as-LSID, ...)` + the typed `GraceLSA` body, reusing `v6OriginateLinkLSA`'s pattern |
| Preservation rules | IPv6 only | preserve §3.1 LSA-ID->prefix (Inter-Area-Prefix / External) and §3.2 Interface ID across restart; persist both in the restart-fact NVS (no IPv4 counterpart) |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| The ext-1 opaque carrier itself (IPv4 link-store flooding, LS-ID split, O-bit DD gate, TLV iterator/builder) | spec-ospf-ext-1-opaque-framework.md (the IPv4 dependency) |
| Any shared OSPFv2/OSPFv3 wire or LSA package | forbidden; the IPv6 Grace-LSA codec lives entirely under `internal/plugins/ospf/v3/`; the IPv4 Grace-LSA rides ext-1 -- no shared Grace-LSA codec |
| TE / RI / SR opaque consumers (IPv4) | spec-ospf-ext-2 / ext-3 / ext-5 |
| OSPFv3 SR / RI / extended-LSA preservation specifics | the owning OSPFv3 SR spec; any arbitrary-32-bit-LSA-ID LSA it adds follows the same §3.1 preservation rule, validated when it lands |
| New FIB backend or kernel-route mechanism | fib-kernel already provides `RTPROT_ZE` + `sweepDelay`; this spec consumes it, does not extend it |
| BFD-coordinated GR (either family) | OSPF BFD is a separate spec (ext-10); a BFD-down during restart degrades to a normal restart (no special handling) |
| Virtual-link GR specifics | the restarter runs SPF for virtual-link restore, but the virtual-links feature is its own spec; GR does not add virtual links |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/research/ospf-implementation-guide.md` §"Graceful Restart and Helper (RFC 3623)" (~1538-1541) and the RFC 5187 references (~1654 `0x000B` Grace-LSA, ~1873-1874) -- the FRR landscape: grace-LSAs announce the restart window; helpers suppress LSDB churn and adjacency tear-downs; helper mode is strictly receive-side; FRR splits GR into `ospf_gr.c` / `ospf_gr_helper.c` (v4) and `ospf6_gr.c` (v6)
  -> Decision: implement BOTH roles in one spec, but build the wire + helper first (receive-side, lower risk) then the restarter on top, matching the guide's "support helper mode first, defer the restarter" ordering as the phase order, not as a scope cut
  -> Constraint: the IPv4 Grace-LSA is Opaque Type 3 carried by ext-1; the IPv6 Grace-LSA is a NATIVE link-scope LSA (function code 11); the control plane is shared; only the wire half forks per family
- [ ] `plan/learned/972-ospf-af-unify.md` -- OSPF is one engine; FSM, flooding, DR election, SPF, LSDB sequencing are AF-neutral and shared
  -> Constraint: the GR control plane (the two state machines, the in-restart gate, the helper map, the NVS, the FIB-retention coupling) is written ONCE on the shared engine; the AF-specific Grace-LSA wire lives in the IPv4 ext-1 consumer glue and the IPv6 `v3/` codec
  -> Decision: do not create a second OSPFv3 engine or any separate top-level OSPFv3 plugin directory; the IPv6 wire half lives under `internal/plugins/ospf/v3/`
- [ ] `plan/spec-ospf-ext-1-opaque-framework.md` "In scope" + "Data Flow" -- the IPv4 carrier this consumer uses
  -> Constraint: IPv4 registers `RegisterOpaqueConsumer(opaqueType=3, scope=link, OnOriginate, OnReceive)`; origination returns `(opaqueID=0, scope=link, body, withdraw)` and the carrier assigns sequence/age, installs into the link store, and floods only to opaque-capable neighbours; reception delivers `OnReceive(opaqueID, body, scope, advRouter, reachable)` after a Newer install; the IPv4 body is built/parsed with the ext-1 TLV builder/iterator
- [ ] `plan/spec-ospf-ext-0-umbrella.md` "Child Decomposition" + "Dependency / Build Order" -- this spec is a child of the OSPF extension umbrella
  -> Constraint: GR depends on the umbrella and (IPv4 only) on ext-1 for the opaque carrier; it shares the family-neutral config / CLI / metrics shape with the rest of the OSPF feature set
- [ ] `ai/rules/plugin-self-containment.md` -- the GR feature must be self-contained
  -> Constraint: removing GR removes the IPv4 Opaque-Type-3 registration, the IPv6 `LSTypeGrace` registration in the v3 codec dispatch, the `graceful-restart` config, both show commands, the doctor check, and all GR metrics; no GR spelling appears in the ext-1 carrier or in generic OSPF packages
- [ ] `ai/rules/buffer-first.md` -- the Grace-LSA body encode is buffer-first (both families)
  -> Constraint: the IPv4 TLVs are emitted through the ext-1 TLV builder into a caller-owned buffer; the IPv6 TLVs are emitted through a v3 TLV builder with an explicit 4-octet pad; no `+`/`fmt` string building of either body; both parse paths return zero-copy iterator views
- [ ] `ai/rules/qemu-testing.md` -- GR is Linux-only (raw IP/IPv6 multicast flood, real FIB retention)
  -> Constraint: the FIB-retention and FRR interop validation (FRR `ospfd` for IPv4, FRR `ospf6d` for IPv6) run as QEMU integration tests; "needs hardware / needs a real restart" is not a reason to skip

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc3623.md` -- Graceful OSPF Restart (the shared control plane + the IPv4 wire)
  -> Constraint: §A -- the IPv4 Grace-LSA is LS type 9, Opaque Type 3, Opaque ID 0; body is TLV-encoded (RFC 3630 format); LS age MUST be 0 at first origination, MUST NOT be reset on retransmit, DoNotAge MUST NOT be set (LS age is the grace clock); type 1 Grace Period (4 bytes) + type 2 Restart Reason (1 byte) MUST always be present; type 3 IP Interface Address (4 bytes) is required on broadcast/NBMA/P2MP; unrecognised TLV types are ignored
  -> Constraint: §2 -- during restart the restarter MUST NOT originate self-LSA types, MUST NOT modify/flush received self-originated LSAs, runs SPF without installing routes, and re-elects self DR if it was DR; §2.1 -- FIB current before reload, grace period SHOULD NOT exceed LSRefreshTime (1800 s); §2.2 -- the three exit triggers; §2.3 -- the exit actions
  -> Constraint: §3.1 -- helper entry only if ALL checks pass; if already helping X, accept the new Grace-LSA and update the grace period; §3.2 -- helper exit on Grace-LSA flushed / grace expiry / strict-checking change; a changed AS-external-LSA must NOT terminate helping for a neighbour in a stub area
  -> Constraint: §5 -- unplanned outage is config-gated, Grace-LSAs before any Hello, reason restricted to 0/3; §B.1 -- RestartInterval 1-1800 s, default 120 s; RestartSupport = none / planned / planned+unplanned; RestartHelperSupport and RestartHelperStrictLSAChecking (default enabled)
- [ ] `rfc/short/rfc5187.md` -- OSPFv3 Graceful Restart (the IPv6 wire delta + the two preservation rules)
  -> Constraint: §2.1 -- the IPv6 Grace-LSA is LS Type 0x000B (function code 11), flooding-scope bits U=0/S2=0/S1=0 (link-local); never flood it beyond the originating link
  -> Constraint: §2.2 -- the Link State ID is the Interface ID of the originating interface; the body is TLV-encoded per RFC 3630 §2.3.2 (4-octet aligned, Length is the unpadded value length); both the Grace Period (type 1, Length 4) and Restart Reason (type 2, Length 1) TLVs MUST always be present
  -> Constraint: §2 -- the RFC 3623 router-address (IP Interface Address, type 3) TLV is NOT required and not emitted; OSPFv3 keys neighbours by Router ID; §3.1 -- preserve the LSA-ID->prefix correspondence for Inter-Area-Prefix and External LSAs (arbitrary 32-bit LSA IDs); §3.2 -- preserve the OSPFv3 Interface ID across restarts; §State Machine -- RFC 5187 defines no new FSM (restarter and helper inherited from RFC 3623); §Pitfalls -- LS age is the grace clock; the Restart Reason TLV declares Length 1 but occupies 4 padded octets
- [ ] `rfc/short/rfc5250.md` -- Opaque-LSA framework (IPv4 carrier reference)
  -> Constraint: §3.1 -- a Type 9 LSA received on an interface other than the target interface MUST be discarded and not acknowledged; the IPv4 Grace-LSA is bound to the single link it arrived on; §3 / App A.2 -- the Opaque Type / Opaque ID split (3 / 0) is owned by the ext-1 carrier; this consumer passes opaqueType=3, opaqueID=0
- [ ] `rfc/short/rfc5340.md` -- OSPFv3 base (IPv6 wire substrate)
  -> Constraint: §A.4.2.1 -- the 16-bit LS Type carries scope in the top 3 bits (U/S2/S1) + a 13-bit function code; the LSDB keys by (LS Type, Link State ID, Advertising Router); a link-local LSA MUST never be flooded beyond its originating link; §A.3.2 / §A.4.3 -- the Interface ID is a router-local 32-bit identifier (the value the Grace-LSA's Link State ID carries); the 20-byte LSA header and Fletcher LS checksum are shared by every v3 LSA including the Grace-LSA; the codec retains unknown LSAs verbatim (`LSA.RawBytes`)

**Key insights:** (minimal context to resume after compaction)
- ONE feature, ONE control plane, TWO wire encodings. The hard part (the two state machines, the in-restart suppression, the FIB-retention coupling, the NVS restart-fact) is shared and written once on the AF-neutral engine. The wire half forks: IPv4 = Opaque Type 3 via ext-1 (consumer); IPv6 = native LS Type 0x000B in `v3/packet` (producer). The two wire halves share no code.
- FIB retention is NOT new code (either family): the fib-kernel `RTPROT_ZE` + `sweepDelay` stale-mark-then-sweep already keeps kernel routes across a process restart. The restarter's job is to (a) NOT call `Installer.RemoveAll` on a graceful stop, and (b) close the grace window (re-install routes) before the fib sweep deadline.
- "Suppress self-LSA origination during restart" = gate `originateSelfLSAs` (instance.go), which dispatches to the v4 topology origination or `v6OriginateSelf`, plus the SPF route install (`spf.Installer.Apply`), behind one per-engine "in graceful restart" flag.
- "Helper freezes the adjacency at Full" = the helper keeps advertising the link to X in its Router-LSA even though the NSM may regress; it does NOT freeze the NSM. The v4 topology builder and the v6 `v6OriginateRouter` both keep X's link while helping.
- LS age is the grace clock (both families): the helper's expiry timer reads the Grace-LSA's LS age vs the Grace Period TLV; never a separate wall clock.
- IPv6 adds two preservation rules (§3.1 arbitrary 32-bit LSA-IDs, §3.2 Interface ID) with NO IPv4 counterpart; both are persistence requirements (restart-fact NVS), not wire fields. A renumbered Interface ID or re-assigned LSA-ID silently terminates the restart early.
- Backward compatibility is automatic for both families (no capability negotiation beyond the IPv4 O-bit). A non-helper neighbour reverts the restart to a normal restart with no loops.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/instance.go` -- `originateSelfLSAs()` is the single self-LSA origination chokepoint (dispatches to `lsdb.OriginateFromTopology` for IPv4 and `v6OriginateSelf` when `codec.IsV6()`); `shutdown()` cancels the context, closes transport, waits the WG; the boot-count NVS seam is `loadOSPFBootCount(openBootCountStore())` seeded once at construction; `runOSPFEngine` is the forked-subprocess lifecycle; `handleLSUpdate` -> `lsdb.ReceiveUpdate`
  -> Constraint: the restarter's "suppress origination" gate wraps `originateSelfLSAs` (return early while the grace window is open) -- this covers BOTH families because it is the shared chokepoint; the restart-fact NVS reuses the `openBootCountStore()` ZeFS blob-store seam, not a new store; the IPv6 helper reception hangs off `ReceiveUpdate` after a link-store install, the IPv4 helper reception hangs off ext-1 `OnReceive`
- [ ] `internal/plugins/ospf/spf/install.go` -- `Installer.Apply` diffs computed routes and inserts/removes `locrib.Path`; `RemoveAll` withdraws every OSPF path; `loc` may be nil in a forked subprocess; this is the SHARED route-install seam for both families
  -> Constraint: during restart the restarter must NOT call `Apply` (no route churn) and must NOT call `RemoveAll` on the graceful stop; on exit it resumes `Apply` so the Loc-RIB is reconciled and the fib-kernel sweep refreshes routes (v4 or v6) instead of deleting them
- [ ] `internal/plugins/fib/kernel/backend.go` + `backend_linux.go` -- `sweepDelay = 30s`; routes marked `RTPROT_ZE` (250); startup `startupSweep` marks existing ZE routes stale, refreshes the ones re-installed within `sweepDelay`, and sweeps the rest
  -> Constraint: this is the FIB-retention substrate for both families; kernel routes survive the OSPF subprocess restart and are refreshed when SPF re-installs them on GR exit; the grace period and the restarter exit MUST complete within `sweepDelay` (or be reconciled with it) so non-stop forwarding holds; this coupling is a design constraint, not new code
- [ ] `internal/plugins/ospf/neighbor/neighbor.go` + `nsm.go` + `table.go` -- the `Neighbor` record (State, Options, RouterID, Address), the shared NSM (Down..Full), `FloodNeighbor`/`Snapshot`, the neighbor table; `EventSink.NeighborUp/Down`; OSPFv3 keys neighbours by Router ID
  -> Constraint: the helper does NOT add a neighbor state; it adds a per-neighbour "helping (restart-in-progress)" flag consulted by the Router-LSA topology builder (v4 and v6) and the LSDB-churn suppressor; the NSM stays unchanged; IPv4 identifies the restarting neighbour by the type-3 IP Interface Address on shared media (else the Advertising Router), IPv6 always by the Grace-LSA's Advertising Router (RFC 5187 §2)
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `OriginateLinkSelf`/`OriginateSelf` + `SelfLSAEncoder` (link-scope self-origination used by both families); `flushReceivedSelfLSA`/`handleSelfReceived` (the §13.4 "neighbour restarted -> flush my received self-LSA" path); `FlushStaleSelfLSAs` (MaxAge flush of stale self-LSAs on exit)
  -> Constraint: during restart the restarter must NOT run `handleSelfReceived`'s flush of its own pre-restart self-LSAs (RFC 3623 §2); on exit it uses `FlushStaleSelfLSAs` to purge the now-stale ones (§2.3); IPv4 originates the Grace-LSA through ext-1, IPv6 through `OriginateLinkSelf`
- [ ] `internal/plugins/ospf/lsdb/link_scope.go` -- the shared link-local store (`d.links`, `linkForLocked`), `OriginateLinkSelf` (gated on `isLinkLSAType(key.Type)`), `installLink`/`installLinkLocked`, `LinkLSAs(iface)`, `FlushStaleLinkSelfLSAs`; `isLinkLSAType` currently matches only `types.LSTypeLink` (0x0008)
  -> Constraint: IPv6 broadens `isLinkLSAType` (or adds a sibling link-scope predicate) so function code 11 (`LSTypeGrace`) also routes through `installLink`/`OriginateLinkSelf`/`LinkLSAs`, mirroring the Type-8 Link-LSA precedent; this is the single IPv6 store-routing chokepoint (the IPv4 path is the ext-1 carrier's link store, not this predicate)
- [ ] `internal/plugins/ospf/lsdb/flooding.go` -- `ReceiveUpdate` runs §13 receive and `notifyChange` on a content change (both families); the helper's "LSDB unchanged since restart" check and the strict-checking "changed LSA that would flood to X" exit trigger read this path
  -> Constraint: the helper hooks the post-install content-change signal (the same signal `notifyChange` raises) to evaluate the §3.2 strict-checking exit; it does NOT change §13 receive semantics
- [ ] `internal/plugins/ospf/v3/types/lsa.go` -- the OSPFv3 `LSType` (16-bit, scope embedded in the top 3 bits, 13-bit function code), the base types' `Known()`, `Scope()`, and `LSAKey` = (LSType, LinkStateID, AdvertisingRouter); there is NO Grace function code yet
  -> Constraint: IPv6 adds `LSTypeGrace LSType = 0x000B` (U=0/S2=0/S1=0 link-local, function code 11) and includes it in `Known()`; the LS-ID stays a plain 4-byte `LinkStateID` (the Interface ID), no opaque split
- [ ] `internal/plugins/ospf/v3/packet/lsa.go` + `lsa_link.go` -- `LSAHeader` (no Options byte), the typed-body `LSA` with `RawBytes` verbatim passthrough (`WriteTo` re-emits `RawBytes` when no typed body is set), `DecodeLSA`/`EncodedLen`/`VerifyChecksum` (Fletcher), `LSAIterator`; the link-local Link-LSA body codec is the precedent the Grace-LSA body mirrors
  -> Constraint: IPv6 adds a typed `Grace *GraceLSA` body field + a `DecodeGrace()` method (mirroring the `Link *LinkLSA` precedent) so a self-originated Grace-LSA encodes through `WriteTo` (Length + Fletcher recomputed) and a received one re-floods via `RawBytes`; the `GraceLSA` body + the v3 TLV iterator/builder live alongside `lsa_link.go` under `v3/packet`, following the same bound-checked-decode / buffer-first-encode / consume-exactly discipline
- [ ] `internal/plugins/ospf/origination_v6_link.go` + `origination_v6.go` -- `v6OriginateLinkLSA(router, iface)` originates a Link-LSA via `e.lsdb.OriginateLinkSelf(...)` with a `v6OriginHeader(...)` + a typed body; `v6OriginateSelf(router, maxMetric)` is the IPv6 self-LSA chokepoint (per-link Link-LSAs, Router-LSA, Intra-Area-Prefix, DR Network-LSA, stale-flush); `v6OriginateRouter` builds the IPv6 Router-LSA topology; `v6ManagedSelfTypes`; `v6SummaryLSID(uint32)` maps a uint32 to a `LinkStateID`
  -> Constraint: IPv6 Grace-LSA origination reuses `v6OriginateLinkLSA`'s exact path -- `OriginateLinkSelf` + `v6OriginHeader(LSTypeGrace, InterfaceID-as-LSID, ...)` + the typed `GraceLSA` body; the restarter's "suppress origination" gate wraps `v6OriginateSelf` (reached via `originateSelfLSAs`); §3.1/§3.2 require the LSA-IDs and Interface IDs `v6OriginateSelf` assigns be STABLE across restart
- [ ] `internal/plugins/ospf/auth_keystore.go` -- `loadOSPFBootCount(store)` reads a ZeFS blob, increments, writes back; `openBootCountStore()` opens the `pkg/zefs` blob store under `internal/core/paths`; the `bootCountStore` interface
  -> Constraint: the restart-fact NVS ({restarting, grace-end, reason}, plus IPv6-only the §3.2 Interface-ID map and §3.1 prefix->LSA-ID map) reuses this ZeFS blob-store pattern (a sibling blob), so a planned restart survives the process restart without a new persistence subsystem
- [ ] `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` -- `max-metric` (RFC 6987 stub-router) is the precedent for a restart/shutdown timer config (`max-metric/router-lsa/{always,on-startup,on-shutdown}`); `ospfConfig` carries `MaxMetric maxMetricConfig`
  -> Constraint: the `graceful-restart` config follows the `max-metric` shape (a sibling container with restarter + helper sub-containers); the SAME YANG file serves both families (the config is family-neutral; the codec path selects the wire behaviour); GR is a distinct mechanism (Grace-LSA + FIB retention), not stub-router

**Behavior to preserve:**
- The shared OSPF engine: the AF-neutral NSM, the `originateSelfLSAs` chokepoint and its v4/v6 dispatch, the SPF route table + `Installer.Apply` insert/remove shape, the fib-kernel `RTPROT_ZE` + `sweepDelay` reconciliation, the shared LSDB link store.
- IPv4: the ext-1 opaque carrier (link-store flooding, LS-ID split, O-bit DD gate, TLV iterator/builder, verbatim re-flood) -- GR adds only a consumer; the carrier is unchanged.
- IPv6: the delivered OSPFv3 base (the RFC 5340 v3 codec under `v3/packet` / `v3/types`, the link-local Link-LSA origination `v6OriginateLinkLSA`, the v3 self-LSA chokepoint `v6OriginateSelf`); the `LSA.RawBytes` verbatim passthrough and the `(LSType, LinkStateID, AdvertisingRouter)` LSDB key.
- All existing OSPF functional/interop tests (both families): a router with GR disabled (the default) behaves exactly as today -- it originates no Grace-LSA, never enters helper mode, and restarts normally.
- The §13.4 "neighbour restarted -> flush my received self-LSA" path for the non-GR case (only the GR restarter suppresses its OWN self-LSA flush during restart).

**Behavior to change:** (all RFC-required, gated behind GR config)
- `originateSelfLSAs`: return early (suppress) while this engine is in graceful restart (covers both families via the shared chokepoint) (RFC 3623 §2).
- The SPF route install: skip `Installer.Apply` while in graceful restart; do NOT `RemoveAll` on a graceful stop (RFC 3623 §2, §2.1).
- The Router-LSA topology builders (v4 topology builder and v6 `v6OriginateRouter`): while helping X, keep X's link advertised even if the NSM regressed; keep X as DR if X was DR (RFC 3623 §3).
- Engine stop: distinguish a graceful restart (preserve FIB, persist restart-fact, originate Grace-LSAs) from a normal shutdown.
- LSDB receive: surface the post-install content-change signal to the helper's strict-checking exit (§3.2) and the restarter's inconsistent-LSA exit (§2.2).
- IPv4 only: register the Opaque-Type-3 consumer via ext-1.
- IPv6 only: add `LSTypeGrace` (0x000B) + `Known()`; add the typed `GraceLSA` body + `DecodeGrace()` + the v3 TLV codec in `v3/packet`; broaden the LSDB link-scope predicate for function code 11; preserve the §3.1 LSA-IDs and §3.2 Interface IDs across restart.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Restarter trigger (shared):** an operator command (`ospf graceful-restart prepare`, or a managed reload) -> the engine enters the pre-restart phase -> originate Grace-LSAs (per family wire) -> persist NVS -> stop without `RemoveAll`. After the subprocess restarts, the persisted restart-fact puts the engine into the in-restart phase.
- **Helper trigger (per family):**
  - IPv4: an LS Update carrying a Type 9 / Opaque Type 3 Grace-LSA arrives -> the ext-1 carrier installs it in the link store and calls this consumer's `OnReceive` -> the helper evaluates the §3.1 entry checks.
  - IPv6: an LS Update carrying a link-scope Grace-LSA (LS Type 0x000B) arrives -> the LSDB link-store install -> the engine inspects the installed LSA, decodes the Grace-LSA, and dispatches it to the helper -> the helper evaluates the §3.1 entry checks (neighbour identified by Advertising Router).
- **Exit triggers (shared):** restarter -- all-adjacencies-up / inconsistent-LSA / grace-timer; helper -- Grace-LSA flushed / grace-timer / strict-checking topology change.

### Transformation Path
1. **Pre-restart (restarter):** ensure the FIB is current (the last `Installer.Apply` has settled); for each interface build a Grace-LSA body (IPv4: three TLVs via the ext-1 builder; IPv6: two TLVs via the v3 builder); originate (IPv4: the ext-1 origination path, opaqueType 3 / opaqueID 0 / scope link, LS age 0; IPv6: `OriginateLinkSelf` + `v6OriginHeader(LSTypeGrace, InterfaceID, ...)` + the typed body, LS age 0); persist {restarting, grace-end, reason} (plus, IPv6, the §3.1 prefix->LSA-ID map and the §3.2 Interface-ID assignments) to the ZeFS NVS blob.
2. **Graceful stop (restarter, shared):** the engine stops WITHOUT `Installer.RemoveAll`; kernel routes (RTPROT_ZE, v4 or v6) remain; the subprocess exits.
3. **In-restart (restarter, after resume, shared):** the persisted restart-fact (grace-end not yet passed) sets the in-restart flag; (IPv6) the §3.2 Interface IDs and §3.1 LSA-IDs are restored from NVS so re-originated LSAs match pre-restart values; `originateSelfLSAs` is suppressed; `Installer.Apply` is suppressed; SPF runs (virtual-link restore) without installing; received self-LSAs are NOT flushed; DR re-election runs if a Hello in Waiting state lists self as DR.
4. **Helper entry (helper):** parse the Grace-LSA TLVs (IPv4: ext-1 iterator, identify X by the type-3 IP address on shared media else the Advertising Router; IPv6: v3 iterator, identify X by the Advertising Router); run the §3.1 checks; on pass, set the per-neighbour helping flag, record grace-end = now + min(Grace Period TLV - LS age, remaining), keep X as DR if X was DR; if already helping X, just update the grace period.
5. **While helping (helper, shared):** the Router-LSA topology builder (v4 builder / v6 `v6OriginateRouter`) keeps X's link advertised (and the Network-LSA if DR) regardless of NSM state; the LSDB-churn suppressor holds; the strict-checking watcher evaluates each installed LSA's content-change-that-would-flood-to-X signal (with the stub-area exception).
6. **Restarter exit (shared):** on the earliest of the three triggers, clear the in-restart flag, re-run `originateSelfLSAs` (Router-LSAs all areas, Network-LSAs where DR; IPv6 also Link-LSAs / Intra-Area-Prefix, with preserved Interface-IDs / LSA-IDs), re-run SPF + `Installer.Apply` (routes re-installed -> fib-kernel sweep refreshes them), `FlushStaleSelfLSAs` (IPv6 also `FlushStaleLinkSelfLSAs`) for now-stale self-LSAs, and originate the Grace-LSAs at MaxAge to flush them; clear the NVS restart-fact.
7. **Helper exit (shared):** on the earliest trigger, clear the per-neighbour helping flag, recalc DR for the segment, re-originate the Router-LSA (and Network-LSA if DR) so the frozen adjacency view is corrected.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator / managed reload <-> restarter (shared) | `ospf graceful-restart prepare` RPC (or a managed-reload hook) sets the pre-restart phase | [ ] |
| IPv4 Grace-LSA body <-> ext-1 carrier | `RegisterOpaqueConsumer(3, link, OnOriginate, OnReceive)`; body built/parsed with the ext-1 TLV builder/iterator | [ ] |
| IPv6 Grace-LSA body <-> v3 codec | a typed `GraceLSA` body in `v3/packet`; built/parsed with the v3 4-octet TLV builder/iterator; LS Type 0x000B, LS ID = Interface ID | [ ] |
| IPv6 Grace-LSA <-> LSDB link store | function code 11 routed through `installLink`/`OriginateLinkSelf`/`LinkLSAs` via the broadened link-scope predicate | [ ] |
| Restarter <-> self-LSA origination (shared) | the in-restart flag gates `originateSelfLSAs` (instance.go) -> v4 topology origination / `v6OriginateSelf` | [ ] |
| Restarter <-> route install (shared) | the in-restart flag gates `spf.Installer.Apply`; graceful stop skips `RemoveAll` | [ ] |
| Restarter <-> FIB retention (shared) | fib-kernel `RTPROT_ZE` routes persist; SPF re-install on exit refreshes them within `sweepDelay` | [ ] |
| Restarter <-> NVS (shared) | `{restarting, grace-end, reason}` (+ IPv6 §3.1/§3.2 maps) persisted via the `pkg/zefs` blob store (sibling of the boot-count blob) | [ ] |
| Helper <-> Router-LSA builder (shared) | the per-neighbour helping flag keeps X's link advertised in the v4 topology builder / `v6OriginateRouter` | [ ] |
| Helper <-> LSDB churn (shared) | the helping flag + the post-install content-change signal drive the §3.2 strict-checking exit | [ ] |

### Integration Points
- `internal/plugins/ospf` (engine, shared) -- the in-restart flag, the helper map, the pre-restart/exit orchestration; gates `originateSelfLSAs` and the route install; the restart-fact NVS; the IPv6 Grace-LSA reception dispatch from `handleLSUpdate`/`ReceiveUpdate`.
- `internal/plugins/ospf` opaque consumer registration (IPv4, ext-1) -- `RegisterOpaqueConsumer(3, link, ...)`.
- `internal/plugins/ospf/v3/types` (IPv6) -- the new `LSTypeGrace` (0x000B) + `Known()` inclusion.
- `internal/plugins/ospf/v3/packet` (IPv6) -- the typed `GraceLSA` body, `DecodeGrace()`, the v3 TLV iterator/builder.
- `internal/plugins/ospf/lsdb` -- `OriginateLinkSelf` (IPv6 Grace-LSA origination), `FlushStaleSelfLSAs`/`FlushStaleLinkSelfLSAs` (exit cleanup), the post-install content-change signal (strict checking); the broadened link-scope predicate (IPv6).
- `internal/plugins/ospf/origination_v6.go` (IPv6) -- the Grace-LSA origination glue (`v6OriginateGraceLSAs` reusing `v6OriginateLinkLSA`); gate `v6OriginateSelf`; `v6OriginateRouter` keeps X's link while helping.
- `internal/plugins/ospf/neighbor` -- the per-neighbour helping flag surfaced into the topology snapshot; DR-was-X preservation.
- `internal/plugins/ospf/spf` -- READ/gate: `Installer.Apply` suppressed during restart, resumed on exit; SPF still computed (virtual-link restore) but not installed.
- `internal/plugins/fib/kernel` -- READ ONLY: the `RTPROT_ZE` + `sweepDelay` retention substrate (no code change; the coupling is verified by the FIB-retention QEMU tests).
- `internal/plugins/ospf/auth_keystore.go` -- the ZeFS NVS blob-store pattern reused for the restart-fact (+ IPv6 preserved maps).
- `internal/plugins/ospf/config.go` + `yang/` -- the family-neutral `graceful-restart` config and the two show commands.

### Architectural Verification
- [ ] No bypassed layers (IPv4 Grace-LSA flows wire -> ext-1 carrier -> `OnReceive`; IPv6 flows wire -> v3 codec -> link-store install -> engine helper dispatch; origination flows the GR glue -> ext-1 link-store (v4) / `OriginateLinkSelf` (v6); no direct LSDB poke)
- [ ] No unintended coupling (the ext-1 carrier names no GR; the v3 GR code shares no packet/LSA code with the IPv4 GR consumer; fib-kernel is read-only; the GR control plane sits on the AF-neutral engine)
- [ ] No duplicated functionality (reuses `originateSelfLSAs`, `FlushStaleSelfLSAs`, `Installer.Apply`/`RemoveAll`, the fib-kernel sweep, the ZeFS blob store, the link store; IPv4 reuses ext-1, IPv6 reuses `v6OriginateLinkLSA`; adds only the two state machines, the wire bodies, the gates, and the config/CLI/metrics)
- [ ] Zero-copy preserved (both Grace-LSA bodies built buffer-first; TLV parse is a zero-copy iterator view; IPv6 received Grace-LSAs re-flood from `RawBytes`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The GR control plane (the two state machines, the in-restart gate, the helper map, the NVS, the FIB-retention coupling) is genuinely AF-neutral and is written once on the shared engine, with only the Grace-LSA wire forking per family | `plan/learned/972-ospf-af-unify.md`; `instance.go` `originateSelfLSAs` v4/v6 dispatch | the control plane must fork per family; duplicated state machines | `TestGRControlPlaneSharedAcrossFamilies` (the same restarter/helper code paths drive both `codec.IsV6()` true and false) | unvalidated |
| A-2 | ext-1 exposes `RegisterOpaqueConsumer(opaqueType, scope, OnOriginate, OnReceive)` with link scope, and originating with opaqueID 0 / scope link produces a Type 9 IPv4 Grace-LSA flooded only to opaque-capable neighbours | `plan/spec-ospf-ext-1-opaque-framework.md` "Consumer registry" + "Data Flow" | the IPv4 GR consumer must add its own opaque flooding; scope creep | `TestGraceLSAv4OriginatedViaCarrier` observes a Type 9/Opaque-3 LSA in the link store | unvalidated |
| A-3 | The v3 codec round-trips a new link-scope LSA verbatim via `LSA.RawBytes`, so a received IPv6 Grace-LSA re-floods byte-for-byte and a self one encodes through `WriteTo` (Length + Fletcher recomputed) | `internal/plugins/ospf/v3/packet/lsa.go` `WriteTo`/`DecodeLSA`/`VerifyChecksum`; the `Link *LinkLSA` precedent | the v3 codec needs new passthrough work; scope creep | `TestGraceLSAv6RoundTrip` (decode then re-encode a 0x000B LSA byte-for-byte) + decode an FRR `ospf6d` Grace-LSA capture | unvalidated |
| A-4 | Broadening `isLinkLSAType` (or a sibling predicate) to include function code 11 routes the IPv6 Grace-LSA through `installLink`/`OriginateLinkSelf`/`LinkLSAs` with no new store | `internal/plugins/ospf/lsdb/link_scope.go` `isLinkLSAType` (matches only `LSTypeLink`); the Type-8 precedent | a new IPv6 link-scope store/key is needed | `TestGraceLSALinkScopeRouting` (a 0x000B LSA lands in the link store, never an area/AS store) | unvalidated |
| A-5 | Both TLV codecs pad to 4-octet alignment exactly (RFC 3630 format); the IPv4 three TLVs and the IPv6 two TLVs (Reason Length 1 -> 4 padded octets) round-trip against FRR | ext-1 "Generic TLV carriage" + `rfc/short/rfc3623.md` §A; `rfc/short/rfc5187.md` §2.2 | a body is mis-padded; FRR rejects the Grace-LSA | `TestGraceLSAv4TLVRoundTrip`, `TestGraceLSAv6TLVRoundTrip`; the two interop scenarios | unvalidated |
| A-6 | The fib-kernel `RTPROT_ZE` routes (v4 and v6) survive the OSPF subprocess restart and `sweepDelay` (30 s) is long enough for a default 120 s grace period IF the restarter re-installs routes early; if not, the sweep deletes them before GR exit | `internal/plugins/fib/kernel/backend.go` `sweepDelay`; `backend_linux.go` stale-mark-then-sweep | the FIB is swept mid-restart -> black hole | `ospf-gr-fib-retention` (v4) + `ospf-v6-gr-fib-retention` (v6) QEMU tests; design reconciles grace period vs `sweepDelay` | unvalidated |
| A-7 | `originateSelfLSAs` is the single self-LSA origination chokepoint for BOTH families, so gating it suppresses all self-LSA types (v4 types 1-5/7; v6 Router/Network/Link/Intra-Area-Prefix/Inter-Area-Prefix/Inter-Area-Router/External/NSSA) during restart | `instance.go` `originateSelfLSAs` -> `OriginateFromTopology` / `v6OriginateSelf` | a self-LSA leaks during restart (violates §2) | `TestRestarterSuppressesSelfLSAsV4`, `TestRestarterSuppressesSelfLSAsV6` assert no self-LSA re-origination while in restart | unvalidated |
| A-8 | The restarter can run SPF for virtual-link restore without installing routes by suppressing `Installer.Apply` while leaving the SPF computation running (shared seam) | `internal/plugins/ospf/spf/install.go` `Apply` is the only install seam; the SPF compute is separate | SPF compute and install are entangled | `TestRestarterRunsSPFNoInstall` (SPF table populated, `loc` unchanged, RTPROT_ZE routes retained) | unvalidated |
| A-9 | A helper can keep X's link advertised across a transient NSM regression by consulting a per-neighbour helping flag in the topology builder (v4 builder and v6 `v6OriginateRouter`), without freezing the NSM | `instance.go` `lsdbTopology`; `origination_v6.go` `v6OriginateRouter`; `neighbor.go` `FloodNeighbor` | the helper drops X's link, prematurely terminating X's restart | `TestHelperKeepsAdjacencyAdvertisedV4`, `TestHelperKeepsAdjacencyAdvertisedV6` | unvalidated |
| A-10 | The post-install content-change signal (`notifyChange` in `lsdb/flooding.go`) is sufficient to drive the §3.2 strict-checking exit without a new flooding hook (shared) | `internal/plugins/ospf/lsdb/flooding.go` `ReceiveUpdate` -> `notifyChange` | strict checking needs new per-LSA "would-flood-to-X" plumbing | `TestHelperStrictExitOnTopologyChange` + the stub-area exception test | unvalidated |
| A-11 | The ZeFS boot-count blob-store seam can persist a second small blob (restart-fact, plus IPv6 §3.1/§3.2 maps) for planned-restart survival | `internal/plugins/ospf/auth_keystore.go` `bootCountStore` + `pkg/zefs` | a new NVS subsystem is needed | `TestRestartFactPersistsAcrossRestart` (write, re-open, read grace-end + preserved maps) | unvalidated |
| A-12 | The OSPFv3 Interface ID (`iface.InterfaceID`) and the arbitrary 32-bit LSA-IDs (`v6SummaryLSID` / external allocator) can be preserved across restart by persisting and restoring them, satisfying RFC 5187 §3.2 / §3.1 | `origination_v6_link.go` uses `iface.InterfaceID`; `origination_v6_summary.go` `v6SummaryLSID`; RFC 5187 §3.1/§3.2 | a renumbered Interface ID or re-assigned LSA-ID silently terminates the restart / churns the network | `TestInterfaceIDPreservedAcrossRestart`, `TestLSAIDPrefixCorrespondencePreserved` | unvalidated |
| A-13 | GR with default config OFF is fully backward compatible for both families: no Grace-LSA originated, no helper entry, normal restart; existing OSPF tests stay green | RFC 3623 §4 / RFC 5187 inheritance (automatic backward compat); the default-off config | enabling the consumer changes default behaviour; interop regression | existing OSPF suite green with GR disabled; `TestGRDisabledNoGraceLSA` (both families) | unvalidated |
| A-14 | The DR-was-X state ("keep X as DR while helping" / "re-elect self DR") is recoverable from the pre-restart Hello/election state (shared election machinery; OSPFv3 keys by Router ID) | `internal/plugins/ospf/iface` election; `neighbor.go` `DeclaredDR` | DR continuity is lost across restart -> election churn defeats GR | `TestRestarterReElectsSelfDR`, `TestHelperKeepsXAsDR` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The fib-kernel sweep deletes OSPF routes (v4 or v6) mid-restart (grace period > `sweepDelay`) -> the exact black hole GR must prevent | a FIB-retention scenario shows a forwarding gap; kernel loses RTPROT_ZE routes | the restarter re-installs routes on the FIRST SPF after resume (not only on full GR exit); the design pins the relationship between the grace window, the restarter exit, and `sweepDelay`; the FIB-retention QEMU tests are the gate |
| R-2 | A self-LSA leaks during restart (a code path other than `originateSelfLSAs` originates, in either family) -> violates §2, peers see a changed Router-LSA, helpers exit early | a peer logs a Router-LSA change for the restarting router during the window | audit ALL origination call sites (v4 and v6); the in-restart flag gates the shared chokepoint AND the stale-flush; `TestRestarterSuppressesSelfLSAsV4`/`V6` |
| R-3 | The helper drops X's link on a transient NSM regression -> premature termination of X's restart | X exits GR early; the adjacency flaps | the per-neighbour helping flag keeps X's link advertised in the topology builder for the whole grace window (both families); the keep-advertised tests |
| R-4 | Strict LSA checking fires on a benign change (an external-LSA change that would NOT flood to X in a stub area) -> the helper exits early for no reason | helping ends right after an unrelated external change | implement the §3.2 stub-area exception: only LSAs that WOULD flood to X count; `TestHelperStubAreaExternalDoesNotExit` |
| R-5 | LS age reset on Grace-LSA retransmit extends the grace period indefinitely (the §A / RFC 5187 §Pitfalls trap, either family) | the helper's window never closes; a stuck restarter holds the adjacency forever | originate the Grace-LSA once at LS age 0 and retransmit the SAME instance (reliable flooding); never re-stamp LS age; `TestGraceLSAAgeNotResetOnRetransmit` (both families) |
| R-6 | (IPv6) The Interface ID is renumbered across restart (§3.2 pitfall) -> pre-restart Link/Network/Router LSAs mismatch neighbour state -> helpers terminate early, silently | the restart fails over to normal reconvergence even though helpers were present | persist and restore the Interface-ID assignments in the restart-fact NVS; `TestInterfaceIDPreservedAcrossRestart`; document the IfIndex-stability dependency (RFC 5187 §3.2 / Errata 1453, RFC 2863 §3.1.5) |
| R-7 | (IPv6) A prefix's arbitrary 32-bit LSA-ID is re-assigned across restart (§3.1 pitfall) -> network churn for Inter-Area-Prefix / External routes | peers withdraw + re-learn the same prefix under a different LSA-ID during the restart | persist and restore the prefix->LSA-ID map in the restart-fact NVS; `TestLSAIDPrefixCorrespondencePreserved` |
| R-8 | (IPv6) The Restart Reason TLV padding is mishandled (Length 1, 4 padded octets) -> a decoder advancing by Length misparses; FRR rejects it | the type-2 TLV parse misaligns; FRR logs a malformed Grace-LSA | the v3 TLV iterator advances by the 4-octet-padded length, not the raw Length; `TestGraceLSAv6TLVRoundTrip` covers the 1-byte reason value |
| R-9 | A forged Grace-LSA spoofs a restart for a router that was actually withdrawn (the §Security spoofing risk, either family) | a router that is really down is held adjacent by helpers | Grace-LSAs ride the existing OSPF cryptographic auth (IPv4: ospf-12 auth; IPv6: RFC 7166 trailer in the delivered base); the helper still requires a prior Full adjacency with X; document the residual risk |
| R-10 | A planned restart's NVS restart-fact is stale (the process restarted for an unrelated reason after the grace window) -> the engine wrongly suppresses origination on a normal boot | a cold boot starts in in-restart mode with no real GR in flight | the restart-fact records grace-end; on resume, if grace-end has passed, ignore it and boot normally; `TestStaleRestartFactIgnored` |
| R-11 | The unplanned-outage path (§5) sends Grace-LSAs after a crash without a guaranteed-sane FIB -> forwarding on stale entries | a crashed router resumes forwarding on routes that no longer match the topology | unplanned support is config-gated (default off), reason restricted to 0/3, Grace-LSAs before any Hello; the operator opt-in is mandatory; `TestUnplannedDisabledByDefault` |
| R-12 | The grace period exceeds LSRefreshTime (1800 s) -> the restarting router's own LSAs age out mid-restart, defeating GR | the restarter's pre-restart LSAs disappear during the window | YANG `range "1..1800"` on the interval; a doctor/validation warning if the configured interval approaches 1800; `TestGracePeriodRangeRejectsAbove1800` |
| R-13 | Family divergence creeps into the shared control plane (e.g. a v6-only branch in the restarter) -> the "one feature" goal erodes into two code paths | review finds `IsV6()` branches inside the state machines rather than only at the wire seam | keep `codec.IsV6()` checks at the Grace-LSA origination/decode seam only; the restarter/helper state machines stay family-neutral; `TestGRControlPlaneSharedAcrossFamilies` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| (IPv4) the GR consumer's `init()` calls `RegisterOpaqueConsumer(3, link, OnOriginate, OnReceive)` | -> | the ext-1 carrier stores the Opaque-Type-3 registration; the engine discovers it | `TestGraceLSAConsumerRegistered` (unit) + `test/ospf/ospf-gr-register.ci` |
| (IPv6) GR enabled; the engine registers the Grace-LSA type on the v6 path | -> | `LSTypeGrace` (0x000B) recognised by the v3 codec/LSDB; routed to the link store | `TestGraceLSATypeRegistered` (unit) + `test/ospf/ospf-v6-gr-register.ci` |
| `ospf graceful-restart prepare` (operator/managed, shared) | -> | the engine enters pre-restart, originates one Grace-LSA per interface (per-family wire), persists the NVS restart-fact | `test/ospf/ospf-gr-prepare.ci` (v4) + `test/ospf/ospf-v6-gr-prepare.ci` (v6) |
| (IPv4) an LS Update carrying a Type 9 / Opaque-3 Grace-LSA arrives from X | -> | ext-1 `OnReceive` -> helper entry checks -> helping flag set; X's link stays advertised | `test/ospf/ospf-gr-helper.ci` |
| (IPv6) an LS Update carrying a link-scope Grace-LSA (0x000B) arrives from X | -> | link-store install -> engine Grace-LSA dispatch -> helper entry checks -> helping flag set | `test/ospf/ospf-v6-gr-helper.ci` |
| The OSPF subprocess restarts with a valid NVS restart-fact (shared) | -> | the in-restart flag is set; `originateSelfLSAs` and `Installer.Apply` suppressed; FIB retained | `ospf-gr-fib-retention` + `ospf-v6-gr-fib-retention` (QEMU) |
| The grace period expires (helper, shared) | -> | the helper exits, recalcs DR, re-originates its Router-LSA | `TestHelperExitOnGraceExpiry` (unit) |

## Acceptance Criteria

<!-- AC rows tagged "(both)" apply to both families and share one shared-engine implementation; rows tagged "(IPv4)" / "(IPv6)" are family-specific wire/preservation. -->
| AC ID | Address family | Input / Condition | Expected Behavior |
|-------|----------------|-------------------|-------------------|
| AC-1 | IPv4 | GR enabled; the GR consumer registers | `RegisterOpaqueConsumer(3, link, ...)` is stored; the engine invokes `OnOriginate`/`OnReceive` for Opaque Type 3 (RFC 3623 §A) |
| AC-2 | IPv6 | GR enabled; the v3 codec/LSDB sees an LS Type 0x000B LSA | it is recognised (`Known()` true), routed to the link store, and re-flooded link-local only; it never appears in an area or AS store (RFC 5187 §2.1, RFC 5340) |
| AC-3 | IPv4 | a planned restart; one or more OSPF interfaces up | one Grace-LSA per interface, LS age 0, with type-1 Grace Period and type-2 Reason TLVs always present, and a type-3 IP Interface Address TLV on broadcast/NBMA/P2MP segments (RFC 3623 §A) |
| AC-4 | IPv6 | a planned restart; one or more OSPFv3 interfaces up | one Grace-LSA per interface, LS age 0, Link State ID = that interface's Interface ID, with type-1 Grace Period and type-2 Restart Reason TLVs always present, and NO type-3 IP Interface Address TLV (RFC 5187 §2, §2.2) |
| AC-5 | both | a Grace-LSA is retransmitted (reliable flooding) before it is acked | the SAME instance is retransmitted; LS age is NOT reset and DoNotAge is NOT set (RFC 3623 §A / RFC 5187 §Pitfalls, R-5) |
| AC-6 | both | the restart-fact is persisted, then the OSPF subprocess restarts within the grace window | on resume the engine enters in-restart mode (grace-end not passed); a restart-fact whose grace-end has passed is ignored and the engine boots normally (RFC 3623 §2.1, R-10) |
| AC-7 | both | the router is in graceful restart | it does NOT originate any self-LSA type (IPv4 types 1-5/7; IPv6 Router/Network/Link/Intra-Area-Prefix/Inter-Area-Prefix/Inter-Area-Router/External/NSSA) and does NOT modify/flush its received self-originated LSAs (RFC 3623 §2) |
| AC-8 | both | the router is in graceful restart and SPF runs | SPF is computed (virtual-link restore) but NO OSPF routes are installed/removed; the pre-restart FIB (RTPROT_ZE kernel routes) remains programmed (RFC 3623 §2, A-6, A-8) |
| AC-9 | IPv6 | the router restarts | the OSPFv3 Interface ID of every interface is preserved, so re-originated Link/Network/Router LSAs carry the same Interface IDs as before (RFC 5187 §3.2, R-6) |
| AC-10 | IPv6 | the router restarts as an ABR/ASBR with Inter-Area-Prefix / External LSAs | the LSA-ID->prefix correspondence is preserved; each prefix is re-originated under the same arbitrary 32-bit LSA ID (RFC 5187 §3.1, R-7) |
| AC-11 | both | the router was DR on a segment before restart and a Hello in Waiting state lists it as DR | it re-elects itself DR on that segment (RFC 3623 §2, A-14) |
| AC-12 | both | all adjacencies are re-established (pre-restart Router/Network-LSA reflected by helpers) | the restarter exits graceful restart and runs the exit actions (RFC 3623 §2.2) |
| AC-13 | both | an LSA inconsistent with the pre-restart Router-LSA is received during restart | the restarter exits graceful restart immediately and runs the exit actions (RFC 3623 §2.2) |
| AC-14 | both | the grace period expires before all adjacencies are up | the restarter exits graceful restart and runs the exit actions (RFC 3623 §2.2) |
| AC-15 | both | the restarter exits graceful restart (any trigger) | it re-originates Router-LSAs (all areas) and Network-LSAs (segments where DR) (IPv6 also Link-LSAs / Intra-Area-Prefix with preserved Interface-IDs / LSA-IDs), re-runs SPF and installs routes (refreshing the RTPROT_ZE routes within `sweepDelay`), flushes stale received self-LSAs, and flushes its own Grace-LSAs at MaxAge (RFC 3623 §2.3) |
| AC-16 | both | a Grace-LSA is received from X and ALL §3.1 checks pass (Full adjacency, LSDB unchanged since X restarted, grace not expired, policy permits, helper not restarting) | the router enters helper mode for X (IPv4 identifies X by the type-3 IP address on shared media else Advertising Router; IPv6 always by Advertising Router), advertises the adjacency to X (Router-LSA, and Network-LSA if DR) regardless of synchronisation state, and keeps X as DR if X was DR (RFC 3623 §3, §3.1) |
| AC-17 | both | a Grace-LSA is received from X but at least one §3.1 check fails | the router does NOT enter helper mode (RFC 3623 §3.1) |
| AC-18 | both | a new Grace-LSA arrives from X while already helping X on the segment | the existing helper relationship is kept and the grace period is updated; no re-entry churn (RFC 3623 §3.1) |
| AC-19 | both | while helping X: the Grace-LSA is flushed, OR the grace period expires (LS age >= Grace Period), OR (strict checking on) an LSA is installed with changed content that would have flooded to X | the router exits helper mode for X, recalcs the DR, and re-originates its Router-LSA (and Network-LSA if DR) (RFC 3623 §3.2) |
| AC-20 | both | while helping X in a stub area: a changed AS-external-LSA is installed that would NOT flood to X | helping for X does NOT terminate (the §3.2 stub-area exception, R-4) |
| AC-21 | IPv6 | a received Grace-LSA missing the mandatory Grace Period or Restart Reason TLV | the Grace-LSA is treated as malformed and ignored (no helper entry, no crash); the engine continues (RFC 5187 §2.2 Validation) |
| AC-22 | both | unplanned-outage support is disabled (the default) and the router cold-boots without a planned restart-fact | no Grace-LSA is originated; the router boots normally (RFC 3623 §5, R-11) |
| AC-23 | both | unplanned-outage support is enabled by the operator | on a cold/unplanned start Grace-LSAs are sent BEFORE any Hello, with reason restricted to 0 (unknown) or 3 (switch to redundant CP) (RFC 3623 §5) |
| AC-24 | both | the configured grace period (RestartInterval) | accepts 1-1800 s, default 120 s; a value above 1800 is rejected by YANG validation (RFC 3623 §2.1, §B.1, R-12) |
| AC-25 | both | GR is disabled (the default) | no Grace-LSA is originated, helper mode is never entered, and a restart behaves exactly as today (backward compatibility, A-13) |
| AC-26 | both | `show ospf graceful-restart` (IPv4) / `show ospf ipv6 graceful-restart` (IPv6) | reports the restarter state (in-restart / not, grace-end, reason) and the per-neighbour helper state (helping which neighbours, remaining grace) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables GR (IPv4) and triggers a planned restart; IPv4 routes keep forwarding across the restart | `graceful-restart` config -> `prepare` -> Opaque-3 Grace-LSA per interface + NVS persist -> graceful stop (no `RemoveAll`) -> subprocess restart -> in-restart suppression -> RTPROT_ZE routes retained -> GR exit re-installs -> sweep refreshes | `ospf-gr-fib-retention` (QEMU) |
| 2 | Enables GR (IPv6) and triggers a planned restart; IPv6 routes keep forwarding across the restart | as story 1 but native 0x000B Grace-LSA (LS ID = Interface ID) + persisted Interface-ID/LSA-ID maps | `ospf-v6-gr-fib-retention` (QEMU) |
| 3 | A Ze router is a helper for a restarting FRR neighbour (both families) | FRR floods a Grace-LSA -> wire -> carrier (ext-1 v4 / v3 codec v6) -> helper dispatch -> §3.1 checks -> helping; Ze keeps the adjacency advertised; FRR completes GR without flapping | `ospf-gr-frr` (FRR `ospfd`) + `ospf-v6-gr-frr` (FRR `ospf6d`) |
| 4 | An FRR router is a helper for a restarting Ze neighbour (both families) | Ze originates Grace-LSAs, restarts, re-acquires adjacencies; FRR holds the adjacency; Ze exits GR cleanly with no route flap | `ospf-gr-frr` (Ze restarter) + `ospf-v6-gr-frr` (Ze restarter) |
| 5 | Runs `show ospf graceful-restart` / `show ospf ipv6 graceful-restart` during a restart | CLI -> the GR state reporter -> restarter/helper state rendered | `test/ospf/ospf-gr-show.ci` + `test/ospf/ospf-v6-gr-show.ci` |
| 6 | Leaves GR disabled (default) and restarts the router (both families) | no Grace-LSA, normal restart, routes reconverge normally | `TestGRDisabledNoGraceLSA` + existing OSPF suite green |
| 7 | Decodes a Grace-LSA hex capture (both families) | CLI decode -> IPv4: ext-1 opaque decode (Opaque Type 3 + three TLVs); IPv6: v3 LSA decode (LS Type 0x000B + LS ID = Interface ID + two TLVs) | `test/ospf/ospf-gr-decode.ci` + `test/ospf/ospf-v6-gr-decode.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGRControlPlaneSharedAcrossFamilies` | `internal/plugins/ospf/gr_test.go` | A-1, R-13: the restarter/helper state machines drive both families through the same code paths | |
| `TestGraceLSAConsumerRegistered` | `internal/plugins/ospf/gr_register_test.go` | AC-1: (IPv4) `RegisterOpaqueConsumer(3, link, ...)` stored; engine invokes the callbacks | |
| `TestGraceLSATypeRegistered` | `internal/plugins/ospf/v3/types/lsa_test.go` | AC-2: (IPv6) `LSTypeGrace` (0x000B) recognised by `Known()`, scope link-local | |
| `TestGraceLSAv6RoundTrip` | `internal/plugins/ospf/v3/packet/lsa_grace_test.go` | A-3, AC-2: (IPv6) decode then re-encode a 0x000B LSA byte-for-byte; Fletcher checksum valid | |
| `TestGraceLSALinkScopeRouting` | `internal/plugins/ospf/lsdb/lsdb_linkscope_test.go` | A-4, AC-2: (IPv6) a 0x000B LSA lands in the link store, never an area/AS store | |
| `TestGraceLSAv4BodyBuild` | `internal/plugins/ospf/gr_lsa_test.go` | AC-3: (IPv4) body has type-1 Grace Period + type-2 Reason always; type-3 IP addr on shared media | |
| `TestGraceLSAv6BodyBuild` | `internal/plugins/ospf/v3/packet/lsa_grace_test.go` | AC-4: (IPv6) body has type-1 + type-2 always; NO type-3 IP-address TLV | |
| `TestGraceLSAv4TLVRoundTrip` | `internal/plugins/ospf/gr_lsa_test.go` | A-5: (IPv4) the three TLVs round-trip via the ext-1 builder/iterator with 4-octet padding | |
| `TestGraceLSAv6TLVRoundTrip` | `internal/plugins/ospf/v3/packet/tlv_test.go` | A-5, R-8: (IPv6) the two TLVs round-trip with 4-octet padding (Reason Length 1 -> 4 octets) | |
| `TestGraceLSAv6TLVIteratorMalformed` | `internal/plugins/ospf/v3/packet/tlv_test.go` | AC-21, R-8: (IPv6) truncated/over-length TLV never panics, reports an error | |
| `TestGraceLSAv4OriginatedViaCarrier` | `internal/plugins/ospf/gr_restarter_test.go` | A-2: (IPv4) origination produces a Type 9 / Opaque-3 LSA in the link store | |
| `TestGraceLSAv6Originated` | `internal/plugins/ospf/gr_restarter_test.go` | A-12, AC-4: (IPv6) one Grace-LSA per interface via `OriginateLinkSelf`, LS ID = Interface ID, LS age 0 | |
| `TestGraceLSAAgeNotResetOnRetransmit` | `internal/plugins/ospf/gr_restarter_test.go` | AC-5, R-5: (both) retransmit keeps LS age, never DoNotAge | |
| `TestRestarterSuppressesSelfLSAsV4` | `internal/plugins/ospf/gr_restarter_test.go` | AC-7, A-7, R-2: (IPv4) no types 1-5/7 self-LSA re-origination while in restart | |
| `TestRestarterSuppressesSelfLSAsV6` | `internal/plugins/ospf/gr_restarter_test.go` | AC-7, A-7, R-2: (IPv6) no v3 self-LSA re-origination (all types) while in restart | |
| `TestRestarterRunsSPFNoInstall` | `internal/plugins/ospf/gr_restarter_test.go` | AC-8, A-8: (both) SPF computed, `Installer.Apply` not called, FIB retained | |
| `TestInterfaceIDPreservedAcrossRestart` | `internal/plugins/ospf/gr_preserve_test.go` | AC-9, A-12, R-6: (IPv6) re-originated Link/Network/Router LSAs carry the same Interface IDs | |
| `TestLSAIDPrefixCorrespondencePreserved` | `internal/plugins/ospf/gr_preserve_test.go` | AC-10, A-12, R-7: (IPv6) each Inter-Area-Prefix / External prefix re-originated under the same LSA ID | |
| `TestRestarterReElectsSelfDR` | `internal/plugins/ospf/gr_restarter_test.go` | AC-11, A-14: (both) re-elect self DR when a Waiting-state Hello lists self as DR | |
| `TestRestarterExitAllAdjacencies` / `TestRestarterExitInconsistentLSA` / `TestRestarterExitGraceExpiry` | `internal/plugins/ospf/gr_restarter_test.go` | AC-12/13/14: (both) the three exit triggers | |
| `TestRestarterExitActions` | `internal/plugins/ospf/gr_restarter_test.go` | AC-15: (both) re-originate self-LSAs, re-install routes, flush stale self-LSAs, flush own Grace-LSAs | |
| `TestRestartFactPersistsAcrossRestart` | `internal/plugins/ospf/gr_nvs_test.go` | AC-6, A-11: (both) write, re-open, read grace-end (+ IPv6 preserved maps) via the ZeFS blob | |
| `TestStaleRestartFactIgnored` | `internal/plugins/ospf/gr_nvs_test.go` | AC-6, R-10: (both) an expired restart-fact is ignored on resume | |
| `TestHelperEntryAllChecksPass` | `internal/plugins/ospf/gr_helper_test.go` | AC-16: (both) enter helper when all §3.1 checks pass; X identified per family | |
| `TestHelperEntryRejectedPerCheck` | `internal/plugins/ospf/gr_helper_test.go` | AC-17: (both) each failing check blocks entry (table-driven) | |
| `TestHelperAlreadyHelpingUpdatesGrace` | `internal/plugins/ospf/gr_helper_test.go` | AC-18: (both) re-receipt updates the grace period, no churn | |
| `TestHelperKeepsAdjacencyAdvertisedV4` / `TestHelperKeepsAdjacencyAdvertisedV6` | `internal/plugins/ospf/gr_helper_test.go` | AC-16, A-9, R-3: (per family) Router-LSA keeps X's link while helping | |
| `TestHelperKeepsXAsDR` | `internal/plugins/ospf/gr_helper_test.go` | AC-16, A-14: (both) X stays DR while helping | |
| `TestHelperExitOnGraceExpiry` / `TestHelperExitOnFlush` / `TestHelperStrictExitOnTopologyChange` | `internal/plugins/ospf/gr_helper_test.go` | AC-19: (both) the three exit triggers + DR recalc + Router-LSA re-origination | |
| `TestHelperStubAreaExternalDoesNotExit` | `internal/plugins/ospf/gr_helper_test.go` | AC-20, R-4, A-10: (both) stub-area external change does not terminate helping | |
| `TestHelperRejectsGraceLSAMissingTLV` | `internal/plugins/ospf/gr_helper_test.go` | AC-21: (IPv6) a Grace-LSA missing a mandatory TLV is ignored, no crash | |
| `TestUnplannedDisabledByDefault` | `internal/plugins/ospf/gr_unplanned_test.go` | AC-22, R-11: (both) no Grace-LSA on cold boot when unplanned support is off | |
| `TestUnplannedGraceBeforeHello` | `internal/plugins/ospf/gr_unplanned_test.go` | AC-23: (both) Grace-LSAs before any Hello, reason 0 or 3 | |
| `TestGRDisabledNoGraceLSA` | `internal/plugins/ospf/gr_config_test.go` | AC-25, A-13: (both) GR off -> no Grace-LSA, normal restart | |
| `TestGracePeriodRangeRejectsAbove1800` | `internal/plugins/ospf/config_test.go` | AC-24, R-12: (both) YANG range 1-1800 enforced | |
| `TestGRShowState` | `internal/plugins/ospf/gr_show_test.go` | AC-26: (both) restarter + helper state rendered for each command | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Grace Period / RestartInterval (s) (both) | 1-1800 | 1800 | 0 | 1801 |
| Grace Period TLV value length (both) | 4 bytes (fixed) | 4 | a shorter type-1 TLV is malformed (ignore the Grace-LSA) | N/A |
| Restart Reason TLV value (both) | 0-3 | 3 | N/A | a value >3 is treated as 0 (unknown) on receive |
| Restart Reason (unplanned, on send) (both) | {0, 3} | 3 | N/A | reasons 1/2 are rejected for the unplanned path (§5) |
| Opaque Type (IPv4 Grace-LSA) | 3 (fixed) | 3 | N/A | N/A |
| Opaque ID (IPv4 Grace-LSA) | 0 (fixed) | 0 | N/A | N/A |
| Grace-LSA LS Type (IPv6) | 0x000B (fixed) | 0x000B | N/A | N/A |
| Grace-LSA Link State ID = Interface ID (IPv6) | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A (0 reserved/inactive) | N/A (32-bit) |
| TLV padded value length (IPv6) | value + (0-3) pad | value+3 | N/A | a length past the LSA Length is an iterator error |
| Helper grace remaining (LS age vs Grace Period) (both) | 0-1800 | 1800 | expired -> no entry | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-gr-register` | `test/ospf/ospf-gr-register.ci` | (IPv4) GR enabled; the consumer + `show ospf graceful-restart` are present | |
| `ospf-v6-gr-register` | `test/ospf/ospf-v6-gr-register.ci` | (IPv6) GR enabled; the Grace-LSA type + `show ospf ipv6 graceful-restart` are present | |
| `ospf-gr-prepare` | `test/ospf/ospf-gr-prepare.ci` | (IPv4) a planned restart originates one Opaque-3 Grace-LSA per interface; the NVS restart-fact is written | |
| `ospf-v6-gr-prepare` | `test/ospf/ospf-v6-gr-prepare.ci` | (IPv6) a planned restart originates one 0x000B Grace-LSA per interface; the NVS restart-fact is written | |
| `ospf-gr-helper` | `test/ospf/ospf-gr-helper.ci` | (IPv4) a received Grace-LSA enters helper mode; the adjacency to X stays advertised; exit on grace expiry | |
| `ospf-v6-gr-helper` | `test/ospf/ospf-v6-gr-helper.ci` | (IPv6) a received Grace-LSA enters helper mode; the adjacency to X stays advertised; exit on grace expiry | |
| `ospf-gr-show` | `test/ospf/ospf-gr-show.ci` | (IPv4) `show ospf graceful-restart` reports restarter + helper state | |
| `ospf-v6-gr-show` | `test/ospf/ospf-v6-gr-show.ci` | (IPv6) `show ospf ipv6 graceful-restart` reports restarter + helper state | |
| `ospf-gr-decode` | `test/ospf/ospf-gr-decode.ci` | (IPv4) decode of a Grace-LSA hex shows Opaque Type 3 + three TLVs | |
| `ospf-v6-gr-decode` | `test/ospf/ospf-v6-gr-decode.ci` | (IPv6) decode of a Grace-LSA hex shows LS Type 0x000B + LS ID = Interface ID + two TLVs | |
| `ospf-gr-disabled` | `test/ospf/ospf-gr-disabled.ci` | (IPv4) GR off: no Grace-LSA, normal restart, routes reconverge | |
| `ospf-v6-gr-disabled` | `test/ospf/ospf-v6-gr-disabled.ci` | (IPv6) GR off: no Grace-LSA, normal restart, IPv6 routes reconverge | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-gr-frr` | `test/interop/scenarios/ospf-gr-frr/` | FRR `ospfd` (graceful-restart + helper) | (IPv4) Ze-helper holds the adjacency while FRR restarts (no flap); Ze-restarter is helped by FRR and exits GR cleanly; the Opaque-3 Grace-LSA TLVs interop both directions | |
| `ospf-v6-gr-frr` | `test/interop/scenarios/ospf-v6-gr-frr/` | FRR `ospf6d` (graceful-restart + helper) | (IPv6) as above with the native 0x000B Grace-LSA (2 TLVs, LS ID = Interface ID), both directions | |
| `ospf-gr-fib-retention` | `test/interop/scenarios/ospf-gr-fib-retention/` | FRR `ospfd` helper + a traffic probe | (IPv4) across a Ze planned restart the RTPROT_ZE kernel routes stay programmed and forwarding continues; routes are refreshed (not swept) on GR exit | |
| `ospf-v6-gr-fib-retention` | `test/interop/scenarios/ospf-v6-gr-fib-retention/` | FRR `ospf6d` helper + an IPv6 traffic probe | (IPv6) as above for IPv6 routes; Interface IDs / LSA-IDs preserved (no churn) | |

> Interop is required: this changes wire behaviour (Grace-LSA origination + helper
> reaction) and forwarding behaviour (FIB retention), in both families. The raw
> IP/IPv6 multicast flood and the real kernel-route retention are Linux-only and
> run as QEMU integration tests (`ai/rules/qemu-testing.md`), consistent with the
> rest of the OSPF interop set (`ospf-p2p-frr`, `ospf-broadcast-frr`, `ospf-v6-frr`,
> `ospf-v6-broadcast-frr`, ...). The IPv4 peer is FRR `ospfd`; the IPv6 peer is FRR
> `ospf6d`.

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
**Shared (both families):**
- `internal/plugins/ospf/instance.go` -- the in-restart flag + the helper map on the engine; gate `originateSelfLSAs`; the pre-restart / exit orchestration; the restart-fact load on `runOSPFEngine` resume; distinguish a graceful stop from `shutdown()`; the IPv6 Grace-LSA reception dispatch from `handleLSUpdate`/`ReceiveUpdate`; carry the per-neighbour helping flag + "X was DR" into `lsdbTopology`/`NeighborInfo`
- `internal/plugins/ospf/spf/install.go` -- a "suppress install while in restart" gate around `Apply` (skip insert/remove) and a guard so a graceful stop does not call `RemoveAll`
- `internal/plugins/ospf/lsdb/origination.go` -- skip `handleSelfReceived`'s flush of the restarter's own pre-restart self-LSAs while in restart; `FlushStaleSelfLSAs` reused on exit
- `internal/plugins/ospf/lsdb/flooding.go` -- surface the post-install content-change signal (already raised for `notifyChange`) to the helper strict-checking exit and the restarter inconsistent-LSA exit
- `internal/plugins/ospf/neighbor/neighbor.go` -- a per-neighbour helping flag on the topology snapshot (`FloodNeighbor` / `NeighborInfo`); record X's DR role at helper entry
- `internal/plugins/ospf/register.go` -- register (IPv4) the Opaque-Type-3 consumer via ext-1; the `graceful-restart` config resolution; the two show commands; the GR doctor check; the GR metrics
- `internal/plugins/ospf/config.go` -- resolve the `graceful-restart` config (restarter support/interval/unplanned, helper support/strict-checking) into `ospfConfig`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the family-neutral `graceful-restart` container (mirrors the `max-metric` precedent)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `show ospf graceful-restart` and `show ospf ipv6 graceful-restart` commands
- `internal/plugins/ospf/cmd_show.go` -- the two `graceful-restart` show handlers
- `internal/plugins/ospf/doctor.go` -- a doctor check for the GR NVS blob path / unplanned-support sanity (the NVS path is the new runtime dependency)

**IPv6-only wire:**
- `internal/plugins/ospf/v3/types/lsa.go` -- add `LSTypeGrace LSType = 0x000B` (function code 11, link-local) + include it in `Known()`
- `internal/plugins/ospf/v3/packet/lsa.go` -- add the typed `Grace *GraceLSA` field + `DecodeGrace()`
- `internal/plugins/ospf/lsdb/link_scope.go` -- broaden the link-scope predicate (`isLinkLSAType` or a sibling) so function code 11 routes through `installLink`/`OriginateLinkSelf`/`LinkLSAs`
- `internal/plugins/ospf/origination_v6.go` -- the IPv6 GR origination glue (`v6OriginateGraceLSAs` reusing `v6OriginateLinkLSA`'s pattern); gate `v6OriginateSelf` behind the in-restart flag; `v6OriginateRouter` keeps X's link advertised while helping; restore Interface-IDs/LSA-IDs on resume

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- family-neutral `graceful-restart` container; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `restart-interval` `range "1..1800"`; `support` enumeration {disabled, planned, planned-and-unplanned}; `helper` `strict-lsa-checking` boolean |
| YANG custom validators | [ ] no | native range + enumeration + boolean suffice |
| CLI commands/flags | [ ] yes | `show ospf graceful-restart` and `show ospf ipv6 graceful-restart` in `ze-ospf-cmd.yang` + `cmd_show.go`; an operator `ospf graceful-restart prepare` action (managed-reload hook) |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ospf graceful-restart` / `show ospf ipv6 graceful-restart` |
| Editor autocomplete | [ ] yes | automatic for the YANG enumeration/boolean leaves + the new show subcommands |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-gr-*.ci` + `test/ospf/ospf-v6-gr-*.ci` |
| Pipe completeness | [ ] yes | both show commands route through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | GR is operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] yes | the GR NVS blob path is a new file dependency -> a doctor check + `internal/core/diagnostic/codes.go` code + unit + functional test (see `ai/rules/doctor-checks.md`) |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_gr_restarter_active` | gauge | `family` (ipv4 / ipv6); 1 while in graceful restart |
| `ze_ospf_gr_restarter_exits_total` | counter | `family`, `reason` (adjacencies / inconsistent-lsa / grace-expiry) |
| `ze_ospf_gr_helper_sessions` | gauge | `family`, `interface` |
| `ze_ospf_gr_helper_exits_total` | counter | `family`, `reason` (flushed / grace-expiry / topology-change) |
| `ze_ospf_gr_grace_lsas` | gauge | `family`, `direction` (originated / received) |

> One metric set with a `family` label (mirroring how the unified engine reports
> per-family state), not two metric families. They use the `ze_ospf_gr_*` prefix
> and are registered by this spec's owner code. The OSPF umbrella "Metrics" table
> must gain these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF Graceful Restart (restarter + helper, both families) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `graceful-restart` container |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ospf graceful-restart` + `show ospf ipv6 graceful-restart` |
| 4 | API/RPC added/changed? | [ ] yes | `docs/architecture/api/commands.md` -- the `prepare` action / managed-reload hook |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains GR (IPv4 opaque consumer + IPv6 Grace-LSA type) |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- a Graceful Restart section covering both families |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` (IPv4 Grace-LSA, Opaque Type 3 + TLVs) and `docs/architecture/wire/ospfv3.md` (IPv6 Grace-LSA, LS Type 0x000B, LS ID = Interface ID + two TLVs) |
| 8 | Plugin SDK/protocol changed? | [ ] no | IPv4 uses the ext-1 `RegisterOpaqueConsumer` API; IPv6 adds a native v3 LSA; no new SDK surface |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc3623.md` and `rfc/short/rfc5187.md` -- flip the Compliance Checklist items to implemented |
| 10 | Test infrastructure changed? | [ ] yes (four interop scenarios) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF GR parity with FRR (`ospfd` + `ospf6d`, restarter + helper) |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- GR state machines + FIB-retention coupling + the IPv6 preservation rules |
| 13 | Route metadata keys added/changed? | [ ] no | GR does not add route metadata keys |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the five `ze_ospf_gr_*` series with the `family` label |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the OSPF umbrella metrics table |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF / v3 / fib-kernel files |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF config/CLI examples against the new `graceful-restart` container |

## Files to Create
**Shared (both families):**
- `internal/plugins/ospf/gr.go` -- the GR feature glue: (IPv4) `RegisterOpaqueConsumer(3, link, ...)`; the engine-side orchestration (pre-restart, in-restart gating, exit); the helper map; the IPv6 Grace-LSA origination/reception dispatch
- `internal/plugins/ospf/gr_restarter.go` -- the restarter state machine (pre-restart, in-restart suppression, the three exit triggers, exit actions; IPv6 §3.1/§3.2 preservation restore)
- `internal/plugins/ospf/gr_helper.go` -- the helper state machine (§3.1 entry checks, while-helping advertisement, §3.2 exit incl. strict checking + stub-area exception)
- `internal/plugins/ospf/gr_nvs.go` -- the restart-fact ZeFS blob (write on prepare, read/validate on resume; carries grace-end + IPv6 §3.2 Interface-ID map + §3.1 prefix->LSA-ID map), reusing the `openBootCountStore` seam
- `internal/plugins/ospf/gr_show.go` -- the `show ospf graceful-restart` + `show ospf ipv6 graceful-restart` state reporter
- `internal/plugins/ospf/gr_test.go`, `gr_register_test.go`, `gr_restarter_test.go`, `gr_helper_test.go`, `gr_nvs_test.go`, `gr_preserve_test.go`, `gr_unplanned_test.go`, `gr_config_test.go`, `gr_show_test.go`

**IPv4-only wire:**
- `internal/plugins/ospf/gr_lsa.go` -- the IPv4 Grace-LSA body build/parse (the three TLVs) over the ext-1 TLV builder/iterator
- `internal/plugins/ospf/gr_lsa_test.go`

**IPv6-only wire:**
- `internal/plugins/ospf/v3/packet/lsa_grace.go` -- the typed `GraceLSA` body (the two TLVs), `DecodeGrace()`, `EncodedLen`, `WriteTo` (buffer-first, 4-octet aligned)
- `internal/plugins/ospf/v3/packet/tlv.go` -- the v3 4-octet-aligned TLV iterator + builder (RFC 3630 §2.3.2 format)
- `internal/plugins/ospf/v3/packet/lsa_grace_test.go`, `internal/plugins/ospf/v3/packet/tlv_test.go`, `internal/plugins/ospf/v3/types/lsa_test.go` (Grace cases)

**Functional + interop (both families):**
- `test/ospf/ospf-gr-register.ci`, `ospf-gr-prepare.ci`, `ospf-gr-helper.ci`, `ospf-gr-show.ci`, `ospf-gr-decode.ci`, `ospf-gr-disabled.ci`
- `test/ospf/ospf-v6-gr-register.ci`, `ospf-v6-gr-prepare.ci`, `ospf-v6-gr-helper.ci`, `ospf-v6-gr-show.ci`, `ospf-v6-gr-decode.ci`, `ospf-v6-gr-disabled.ci`
- `test/interop/scenarios/ospf-gr-frr/` and `ospf-v6-gr-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospf-gr-fib-retention/` and `ospf-v6-gr-fib-retention/` -- `ze.conf`, `frr.conf`, `check.py` (traffic probe across the restart)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the ext-1 carrier (IPv4), the v3 codec verbatim passthrough + link store (IPv6), and the fib-kernel `RTPROT_ZE`/`sweepDelay` retention exist |
| 3. Wiring phase | Wiring Test table -- consumer registration (IPv4) + Grace-LSA type registration (IPv6) + failing wiring tests |
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

<!-- Phase 1 is ALWAYS wiring: create the entry point and a failing wiring test. -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- the IPv4 Opaque-Type-3 consumer registration + the IPv6 Grace-LSA type registration + failing wiring tests
   - Tests: `TestGraceLSAConsumerRegistered`, `TestGraceLSATypeRegistered`, `TestGraceLSALinkScopeRouting`, `test/ospf/ospf-gr-register.ci`, `test/ospf/ospf-v6-gr-register.ci`
   - Files: `gr.go` (the consumer + GR skeleton with stub callbacks/state machines), `v3/types/lsa.go` (`LSTypeGrace` + `Known()`), `lsdb/link_scope.go` (broaden the link predicate), `register.go` (wire the consumer + the `graceful-restart` config skeleton + the show stubs)
   - Verify: the IPv4 consumer registers; the IPv6 Grace-LSA type is recognised and routes to the link store; origination/reception/state machines are stubs so the deeper tests still fail
2. **Phase: Grace-LSA wire (both families)** -- IPv4 body over ext-1; IPv6 type + body + v3 TLV codec
   - Tests: `TestGraceLSAv4BodyBuild`, `TestGraceLSAv4TLVRoundTrip`, `TestGraceLSAv6RoundTrip`, `TestGraceLSAv6BodyBuild`, `TestGraceLSAv6TLVRoundTrip`, `TestGraceLSAv6TLVIteratorMalformed`, `TestGraceLSAAgeNotResetOnRetransmit`, `ospf-gr-decode.ci`, `ospf-v6-gr-decode.ci`
   - Files: `gr_lsa.go` (IPv4), `v3/packet/lsa_grace.go` + `v3/packet/tlv.go` + `v3/packet/lsa.go` (IPv6)
   - Verify: each body round-trips with 4-octet padding; LS age semantics hold; the IPv6 iterator never panics; decode renders the per-family Grace-LSA
3. **Phase: Helper state machine (shared)** -- receive-side first (lower risk, per the guide ordering)
   - Tests: `TestHelperEntryAllChecksPass`, `TestHelperEntryRejectedPerCheck`, `TestHelperAlreadyHelpingUpdatesGrace`, `TestHelperKeepsAdjacencyAdvertisedV4`, `TestHelperKeepsAdjacencyAdvertisedV6`, `TestHelperKeepsXAsDR`, `TestHelperExitOnGraceExpiry`, `TestHelperExitOnFlush`, `TestHelperStrictExitOnTopologyChange`, `TestHelperStubAreaExternalDoesNotExit`, `TestHelperRejectsGraceLSAMissingTLV`, `ospf-gr-helper.ci`, `ospf-v6-gr-helper.ci`
   - Files: `gr_helper.go`, `gr.go` (helper map + IPv6 reception dispatch), `instance.go`/`neighbor.go` (helping flag), `origination_v6.go` `v6OriginateRouter` (keep X's link), `lsdb/flooding.go` (content-change signal)
   - Verify: entry/exit checks per §3.1/§3.2 (both families); the adjacency to X stays advertised; the stub-area exception holds; a malformed IPv6 Grace-LSA is ignored
4. **Phase: Restarter NVS + preservation + in-restart gating (shared + IPv6 preservation)** -- the suppression core
   - Tests: `TestRestartFactPersistsAcrossRestart`, `TestStaleRestartFactIgnored`, `TestRestarterSuppressesSelfLSAsV4`, `TestRestarterSuppressesSelfLSAsV6`, `TestRestarterRunsSPFNoInstall`, `TestInterfaceIDPreservedAcrossRestart`, `TestLSAIDPrefixCorrespondencePreserved`
   - Files: `gr_nvs.go`, `gr_restarter.go`, `gr_preserve` logic, `instance.go` (gate `originateSelfLSAs`), `origination_v6.go` (gate `v6OriginateSelf`; restore Interface-IDs/LSA-IDs), `spf/install.go` (gate `Apply`, guard `RemoveAll`), `lsdb/origination.go` (skip own-self-LSA flush)
   - Verify: the restart-fact (+ IPv6 maps) survives a restart; in-restart suppresses origination + install for both families; SPF still computes; FIB not touched; Interface IDs / LSA-IDs preserved
5. **Phase: Restarter origination + DR re-election + exit triggers + exit actions (shared)**
   - Tests: `TestGraceLSAv4OriginatedViaCarrier`, `TestGraceLSAv6Originated`, `TestRestarterReElectsSelfDR`, `TestRestarterExitAllAdjacencies`, `TestRestarterExitInconsistentLSA`, `TestRestarterExitGraceExpiry`, `TestRestarterExitActions`, `TestGRControlPlaneSharedAcrossFamilies`, `ospf-gr-prepare.ci`, `ospf-v6-gr-prepare.ci`
   - Files: `gr_restarter.go`, `gr.go` (`v6OriginateGraceLSAs` + the `prepare` orchestration + graceful stop), `lsdb/origination.go` (`FlushStaleSelfLSAs`/`FlushStaleLinkSelfLSAs` on exit)
   - Verify: one Grace-LSA per interface per family; retransmit keeps LS age; DR continuity; the three exit triggers; the exit re-originates LSAs, re-installs routes, flushes stale self-LSAs + own Grace-LSAs; the control plane drives both families
6. **Phase: Unplanned outage (config-gated, shared)** -- §5
   - Tests: `TestUnplannedDisabledByDefault`, `TestUnplannedGraceBeforeHello`, `TestGRDisabledNoGraceLSA`
   - Files: `gr_restarter.go`, `config.go`
   - Verify: unplanned off by default; when on, Grace-LSAs before Hello, reason 0/3 only
7. **Phase: Config + CLI + metrics + doctor (shared)** -- user surface
   - Tests: `TestGracePeriodRangeRejectsAbove1800`, `TestGRShowState`, `ospf-gr-register.ci`, `ospf-v6-gr-register.ci`, `ospf-gr-show.ci`, `ospf-v6-gr-show.ci`, `ospf-gr-disabled.ci`, `ospf-v6-gr-disabled.ci`
   - Files: `yang/ze-ospf-conf.yang`, `yang/ze-ospf-cmd.yang`, `config.go`, `gr_show.go`, `cmd_show.go`, `doctor.go`, metric registration in `register.go`
   - Verify: the family-neutral `graceful-restart` container, the two show commands, the five `family`-labelled metric series, the NVS-path doctor check
8. **Functional tests** -> the twelve `.ci` cover the user-visible behaviour across both families
9. **RFC refs** -> add `// RFC 3623 Section X` comments on the shared control plane and the IPv4 wire; add `// RFC 5187 Section X` comments on the IPv6 Grace-LSA type/body, the §2.2 LS-ID = Interface ID, the §3.1 LSA-ID preservation, and the §3.2 Interface-ID preservation
10. **Interop** -> `ospf-gr-frr` + `ospf-gr-fib-retention` (FRR `ospfd`) and `ospf-v6-gr-frr` + `ospf-v6-gr-fib-retention` (FRR `ospf6d`) QEMU scenarios
11. **Full verification** -> `make ze-verify`
12. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation; family-tagged ACs implemented for the named family |
| Feature completeness | each user story has a working path; GR parity with FRR `ospfd` (IPv4) and `ospf6d` (IPv6), restarter + helper, planned + helper-strict-checking; FIB retention proven end-to-end for both families |
| One feature, not two | the restarter/helper state machines are shared; `codec.IsV6()` branches appear only at the Grace-LSA origination/decode wire seam, never inside the state machines (R-13) |
| Correctness | LS age = grace clock (never reset, both families); IPv4 three TLVs + Opaque Type 3 / ID 0; IPv6 two TLVs + LS Type 0x000B + LS ID = Interface ID + 4-octet pad (Reason Length 1 -> 4 octets); §3.1 entry checks all enforced; §3.2 exit incl. stub-area exception; restarter suppresses ALL self-LSA types (both); IPv6 Interface IDs + LSA-IDs preserved; `RemoveAll` not called on graceful stop; grace window vs `sweepDelay` reconciled |
| Naming | `ze_ospf_gr_*` metrics with a `family` label; YANG `graceful-restart` / `restart-interval` kebab-case; `show ospf graceful-restart` / `show ospf ipv6 graceful-restart`; `OnOriginate`/`OnReceive` (IPv4); `LSTypeGrace` (IPv6) |
| Data flow | IPv4 GR is an ext-1 consumer (no new flooding); IPv6 Grace-LSA flows v3 codec -> link store -> helper dispatch; fib-kernel read-only; the in-restart flag gates origination + install only; no shared v2/v3 wire code |
| CLI grammar | both show commands action-before-identifier |
| Doctor checks | the GR NVS blob path has a `ze doctor` check per `ai/rules/doctor-checks.md` |
| YANG validation | `restart-interval` range 1-1800; `support`/`helper` enumerations/booleans; no bare `type string` |
| Prometheus counters | the five `ze_ospf_gr_*` series (with the `family` label) defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | removing GR removes the IPv4 consumer + the IPv6 `LSTypeGrace`, config, both commands, doctor, metrics; no GR spelling in ext-1 or generic OSPF packages |
| Rule: buffer-first | both Grace-LSA bodies built via a TLV builder into a caller buffer; parse is a zero-copy iterator |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| IPv4 Opaque-Type-3 consumer registered | `grep -rn 'RegisterOpaqueConsumer' internal/plugins/ospf/gr.go` |
| IPv6 Grace-LSA type recognised | `grep -rn 'LSTypeGrace' internal/plugins/ospf/v3/types` |
| IPv4 Grace-LSA body build/parse | `go test ./internal/plugins/ospf -run 'GraceLSAv4'` |
| IPv6 Grace-LSA body build/parse | `go test ./internal/plugins/ospf/v3/packet -run 'Grace'` |
| IPv6 link-scope routing for 0x000B | `go test ./internal/plugins/ospf/lsdb -run 'GraceLSALinkScope'` |
| Restarter suppression + exit (both) | `go test ./internal/plugins/ospf -run 'Restarter'` |
| IPv6 preservation (Interface-ID / LSA-ID) | `go test ./internal/plugins/ospf -run 'Preserved'` |
| Helper entry/exit (both) | `go test ./internal/plugins/ospf -run 'Helper'` |
| NVS restart-fact | `go test ./internal/plugins/ospf -run 'RestartFact'` |
| Shared control plane | `go test ./internal/plugins/ospf -run 'GRControlPlaneShared'` |
| Five metric series | `grep -rn 'ze_ospf_gr_' internal/plugins/ospf` |
| FIB-retention interop (both) | `ls test/interop/scenarios/ospf-gr-fib-retention/ test/interop/scenarios/ospf-v6-gr-fib-retention/` |
| GR interop (both) | `ls test/interop/scenarios/ospf-gr-frr/ test/interop/scenarios/ospf-v6-gr-frr/` |
| Functional tests | `ls test/ospf/ospf-gr-*.ci test/ospf/ospf-v6-gr-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | both Grace-LSA TLV parses are bound-checked (the ext-1 iterator and the v3 iterator never panic); a missing/short mandatory TLV makes the Grace-LSA malformed and ignored, not a crash; the existing v3 LSA fuzz target is extended with Grace-LSA bodies |
| Spoofed Grace-LSA | a forged Grace-LSA can hold a withdrawn router adjacent (R-9); the helper still requires a prior Full adjacency with X; Grace-LSAs ride the existing OSPF auth (IPv4: ospf-12; IPv6: RFC 7166 trailer); document the residual risk |
| Resource exhaustion | the helper map is bounded by the neighbour count; the link store shares the existing per-area cap; a flood of Grace-LSAs cannot grow memory unbounded; the acceptance rate limit applies |
| FIB safety | unplanned support is config-gated (default off) because a crashed router cannot guarantee FIB sanity (§5, R-11); the planned path ensures the FIB is current before reload |
| Preservation integrity (IPv6) | the persisted Interface-ID / LSA-ID maps are read back and validated on resume; a corrupt/missing map ignores the restart-fact and boots normally (no silent mismatch) |
| Error leakage | a malformed Grace-LSA is counted, not surfaced to peers; GR state in `show` does not leak secrets |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
| Shared v2/v3 wire code introduced | STOP; keep the IPv6 Grace-LSA codec under `internal/plugins/ospf/v3/` and the IPv4 body in the ext-1 consumer glue |
| `IsV6()` branch inside a state machine | STOP; move the branch to the wire seam; the control plane stays family-neutral (R-13) |
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
OSPF Graceful Restart is ONE feature: two control-plane state machines layered on
an EXISTING FIB-retention substrate (the fib-kernel `RTPROT_ZE`
stale-mark-then-sweep), shared across both address families because the OSPF
engine's FSM, flooding, DR election, SPF, and LSDB sequencing are
address-family-neutral. The only material per-family difference is the Grace-LSA
wire object: IPv4 carries it as an Opaque-Type-3 LSA (owned by ext-1, this spec is
a consumer), IPv6 carries it as a native link-scope LS Type 0x000B (added in the
v3 codec, this spec is a producer). The hard part is not the wire and not the FIB;
it is the disciplined suppression (while in restart the router must touch nothing
it normally re-originates via `originateSelfLSAs` or re-installs via
`Installer.Apply`) and the helper's adjacency freeze (keep advertising an
adjacency the NSM might tear down) -- both reduce to a single per-engine /
per-neighbour flag read at the shared origination and topology-build chokepoints.
IPv6 adds two preservation rules (Interface ID, arbitrary 32-bit LSA-IDs) that
keep the inherited RFC 3623 FSM from terminating early.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One spec / one control plane for both families | two version-split specs (the prior `ospf-ext-9` IPv4 + `ospfv3-ext-4` IPv6) | OSPF is one unified engine; the restarter/helper procedures, FIB-retention coupling, NVS, config/CLI/metrics are family-neutral; RFC 5187 is explicitly "RFC 3623 except the wire"; duplicating the control plane across two specs invites divergence |
| IPv4 Grace-LSA via the ext-1 opaque carrier (consumer of Opaque Type 3) | re-implement link-local Grace-LSA flooding | RFC 3623 §A defines the Grace-LSA AS an Opaque Type 3 LSA; ext-1 owns the carrier; no duplicated flooding |
| IPv6 Grace-LSA as a native v3 LS type (0x000B) routed through the link store | a v3 opaque carrier mirroring ext-1 | RFC 5340 has no opaque LSAs; the Grace-LSA is a first-class link-scope LSA reusing the delivered Link-LSA store + origination |
| `codec.IsV6()` only at the Grace-LSA origination/decode seam | branch per family inside the state machines | keeps the "one feature" goal honest; the state machines stay family-neutral and auditable |
| Reuse the fib-kernel `RTPROT_ZE` + `sweepDelay` for FIB retention (both families) | a new GR-specific FIB-freeze mechanism | the stale-mark-then-sweep already keeps kernel routes across a process restart; GR's job is only to not `RemoveAll` and to re-install before the sweep deadline |
| In-restart is a per-engine flag gating `originateSelfLSAs` + `Installer.Apply` | a parallel "restart-mode" engine | the shared chokepoints are the only places self-LSAs are originated and routes installed; a flag is the minimal, auditable suppression for both families |
| Helper keeps X's link via a per-neighbour flag in the topology builder, NOT by freezing the NSM | freeze the neighbor FSM at Full | RFC 3623 §3 freezes the ADVERTISED view, not the NSM; the topology builders (v4 + `v6OriginateRouter`) are the right seam |
| Restart-fact (+ IPv6 preserved maps) in a ZeFS blob (sibling of the boot-count blob) | a new NVS subsystem; in-memory only | `auth_keystore.go` already persists a blob across restarts via `pkg/zefs` |
| IPv6 persists Interface-ID + prefix->LSA-ID maps | re-derive them on resume | RFC 5187 §3.1/§3.2 require STABLE values; re-deriving risks a renumber that silently terminates the restart |
| One `ze_ospf_gr_*` metric set with a `family` label | separate `ze_ospfv3_gr_*` series | matches how the unified engine reports per-family state; one feature, one metric namespace |

## Known Limitations
- Per-segment partial helping is not offered (the partial-segment pitfall, §3.1); helper support is all-or-nothing per the configured policy (both families).
- Virtual-link GR is constrained by the virtual-links feature's own scope; the restarter runs SPF for virtual-link restore but does not add virtual links here.
- The grace window must complete within (or be reconciled with) the fib-kernel `sweepDelay`; a grace period far larger than the sweep delay requires a coordinated sweep-delay extension, documented as a deployment constraint.
- A forged Grace-LSA can hold a withdrawn router adjacent; mitigated by OSPF cryptographic auth (IPv4 ospf-12; IPv6 RFC 7166 trailer) and the prior-Full-adjacency requirement, not eliminated.
- (IPv6) The §3.2 Interface-ID stability depends on a stable interface-id source across reboots (RFC 5187 §3.2 / Errata 1453 reference RFC 2863 §3.1.5); Ze persists the assignments in the restart-fact NVS rather than relying on the OS.
- OSPFv3 SR / RI / extended LSAs are not preserved by name here; any arbitrary-32-bit-LSA-ID LSA a future OSPFv3 SR spec adds follows the same §3.1 preservation rule, validated when it lands.

## RFC Documentation

Add `// RFC 3623 Section X.Y: "<quoted requirement>"` above the SHARED control
plane and the IPv4 wire:
- §A IPv4 Grace-LSA = LS type 9 / Opaque Type 3 / Opaque ID 0; the three TLVs; LS age 0 at origination, never reset, DoNotAge never set
- §2 in-restart suppression (no self-LSA types, no FIB install, keep received self-LSAs); DR re-election
- §2.1 FIB current before reload; grace period <= LSRefreshTime
- §2.2 the three exit triggers; §2.3 the exit actions
- §3 / §3.1 helper entry checks; keep X's link advertised; keep X as DR
- §3.2 helper exit triggers incl. strict checking + the stub-area exception
- §5 the unplanned-outage path (Grace-LSA before Hello, reason 0/3, operator opt-in)
- RFC 5250 §3.1 the Type 9 "discard if received off the target interface" scope rule (enforced by the ext-1 carrier the IPv4 path relies on)

Add `// RFC 5187 Section X.Y: "<quoted requirement>"` above the IPv6 wire + the
two preservation rules:
- §2.1 the IPv6 Grace-LSA LS Type 0x000B / function code 11 / link-local scope bits
- §2.2 the Link State ID = Interface ID and the two mandatory TLVs (Grace Period, Restart Reason)
- §2 the dropped router-address TLV (helper keys X by Advertising Router)
- §3.1 the LSA-ID->prefix correspondence preservation
- §3.2 the Interface ID preservation

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
| Non-stop forwarding across a planned restart (IPv4) | interop (QEMU) | `ospf-gr-fib-retention` (RTPROT_ZE routes retained, refreshed on exit) |
| Non-stop forwarding across a planned restart (IPv6) | interop (QEMU) | `ospf-v6-gr-fib-retention` (RTPROT_ZE IPv6 routes retained, refreshed on exit) |
| One shared control plane drives both families | unit | `TestGRControlPlaneSharedAcrossFamilies` |
| IPv4 Grace-LSA (Opaque Type 3, three TLVs) interops with FRR `ospfd` | interop | `ospf-gr-frr` (both directions) |
| IPv6 Grace-LSA (0x000B, LS ID = Interface ID, two TLVs) interops with FRR `ospf6d` | interop | `ospf-v6-gr-frr` (both directions) |
| Restarter suppresses all self-LSA origination + route install during restart | unit | `TestRestarterSuppressesSelfLSAsV4`, `TestRestarterSuppressesSelfLSAsV6`, `TestRestarterRunsSPFNoInstall` |
| Helper holds the adjacency at Full for the grace window | unit + interop | `TestHelperKeepsAdjacencyAdvertisedV4`/`V6`, the two `*-frr` scenarios (Ze helper) |
| (IPv6) Interface ID + LSA-ID->prefix correspondence preserved across restart | unit | `TestInterfaceIDPreservedAcrossRestart`, `TestLSAIDPrefixCorrespondencePreserved` |
| GR disabled is fully backward compatible (both families) | unit + suite | `TestGRDisabledNoGraceLSA` + existing OSPF suite green |

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
- [ ] AC-1..AC-26 all demonstrated (family-tagged ACs for the named family)
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/*`, `internal/plugins/ospf/v3/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC 3623 + RFC 5187 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (both roles + both families needed; the helper map / in-restart flag are minimal and shared)
- [ ] No speculative features (only RFC 3623 + RFC 5187 restarter + helper)
- [ ] Single responsibility per component (body / restarter / helper / NVS / show separated; wire forks per family)
- [ ] Explicit > implicit behavior (GR off by default; unplanned opt-in)
- [ ] Minimal coupling (IPv4 ext-1 consumer; IPv6 native v3 LSA; fib-kernel read-only; one shared control plane)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-gr-frr`, `ospf-v6-gr-frr`, `ospf-gr-fib-retention`, `ospf-v6-gr-fib-retention`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-9-graceful-restart.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-9-graceful-restart.md`
