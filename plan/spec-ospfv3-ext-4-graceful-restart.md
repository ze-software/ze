# Spec: ospfv3-ext-4 -- OSPFv3 Graceful Restart restarter + helper (RFC 5187)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospfv3-0-umbrella.md (delivered) |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc5187.md` -- OSPFv3 Graceful Restart: the Grace-LSA wire format (§2.1 LS type 0x000b / function code 11, link-local S2=0/S1=0; §2.2 Link State ID = Interface ID, the two mandatory TLVs, the dropped router-address TLV), the two OSPFv3-specific preservation rules (§3.1 LSA-ID->prefix correspondence, §3.2 Interface ID), and the "identical to RFC 3623 except the differences" framing (Abstract)
4. `rfc/short/rfc5340.md` -- OSPFv3 base: the 16-bit scope-bearing LS Type (§A.4.2.1 U/S2/S1 + 13-bit function code), the 20-byte LSA header, link-local flooding scope, the Interface ID model (§A.3.2), and the Router/Network/Link/Intra-Area-Prefix/Inter-Area-Prefix/External LSA registry the restarter must preserve across restart
5. `plan/spec-ospfv3-ext-0-umbrella.md` -- this child's coordinating umbrella; the "no opaque carrier in v3" decision (the Grace-LSA is a NATIVE link-scope LSA, not an opaque Type 9), the "no shared v2/v3 wire code" mandate, and the base-only dependency (ext-4 depends on the delivered base, not on any other ext-N)
6. `internal/plugins/ospf/v3/types/lsa.go` -- the OSPFv3 `LSType` (scope embedded in the top 3 bits, 13-bit function code), `Known()`, and `LSAKey` = (LSType, LinkStateID, AdvertisingRouter); the Grace-LSA adds function code 11 (LS Type 0x000B) here
7. `internal/plugins/ospf/v3/packet/lsa.go` + `lsa_link.go` -- the v3 LSA codec: `LSAHeader` (no Options byte), the typed-body `LSA` with verbatim `RawBytes` passthrough, `DecodeLSA`/`WriteTo`/`VerifyChecksum` (Fletcher), and the link-local Link-LSA precedent the Grace-LSA body mirrors
8. `internal/plugins/ospf/origination_v6_link.go` + `origination_v6.go` -- `v6OriginateLinkLSA` (the link-scope self-origination via `e.lsdb.OriginateLinkSelf` + `v6OriginHeader` + a typed body), `v6OriginateSelf` (the v3 self-LSA origination chokepoint the restarter must suppress), `v6ManagedSelfTypes`
9. `internal/plugins/ospf/lsdb/link_scope.go` -- the link-local store (`d.links`), `OriginateLinkSelf`, `installLink`, `LinkLSAs`, and `isLinkLSAType` (currently matches ONLY `LSTypeLink` 0x0008 -- must be broadened so the Grace-LSA function code 11 routes through the link store)
10. `internal/plugins/ospf/spf/install.go` -- the `locrib.Path` install seam (`Apply`/`RemoveAll`); the restarter must NOT install/remove OSPF routes during restart (FIB retention)
11. `internal/plugins/fib/kernel/backend.go` + `backend_linux.go` -- `RTPROT_ZE` marking + `sweepDelay` stale-mark-then-sweep: the existing mechanism that keeps kernel routes alive across a control-plane restart, which the restarter relies on for non-stop forwarding
12. `internal/plugins/ospf/instance.go` -- the shared engine: `originateSelfLSAs` (dispatches to `v6OriginateSelf` for IPv6), `handleLSUpdate` -> `lsdb.ReceiveUpdate`, `shutdown()`; `internal/plugins/ospf/auth_keystore.go` -- the ZeFS `bootCountStore` NVS blob pattern (`loadOSPFBootCount`/`openBootCountStore`) the restart-fact persistence reuses

## Task

Add **OSPFv3 Graceful Restart (RFC 5187)** to the native OSPF plugin's IPv6
family at `internal/plugins/ospf/` (with the v3 wire encodings under
`internal/plugins/ospf/v3/`), implementing **both roles**: the **restarting
router** (restarter) and the **helper neighbor**. Graceful Restart lets a Ze
router restart or reload its OSPFv3 control software while staying on the
forwarding path ("non-stop forwarding"): the restarting router floods link-scope
**Grace-LSAs** asking neighbours to keep advertising it as fully adjacent for a
bounded grace period, preserves its FIB across the control-plane restart, and
re-acquires adjacencies and re-syncs the LSDB without flapping IPv6 routes;
neighbours that receive a Grace-LSA enter **helper mode**, hold the adjacency at
Full and suppress LSDB churn until the grace period ends or a topology change
forces an early exit.

RFC 5187 is deliberately a delta on RFC 3623: "the OSPFv3 graceful restart is
identical to that of OSPFv2 except for the differences described in this
document" (Abstract). The restarter and helper *procedures* are inherited
unchanged from RFC 3623; this spec re-implements only the **v3-specific** parts:
the Grace-LSA encode/decode (a NATIVE link-scope LSA, NOT an opaque LSA), the
Link State ID semantics, the dropped router-address TLV, and the two OSPFv3
preservation rules. The OSPFv3 extension umbrella records that v3 has **no
opaque carrier** (RFC 5340 carries extensions as native, scope-aware LSAs), so
unlike the OSPFv2 GR spec (`spec-ospf-ext-9-graceful-restart.md`, which is a
consumer of the v2 opaque framework) this spec adds the Grace-LSA as a first-
class v3 LSA type: a new function code (11, LS Type **0x000B**, link-local scope
U=0/S2=0/S1=0), a typed body, link-store routing, and the two control-plane
state machines.

The Grace-LSA body carries **two** TLVs (RFC 5187 §2.2): **type 1 Grace Period**
(4 bytes, always present) and **type 2 Graceful Restart Reason** (1 byte, always
present, padded to 4 octets). The RFC 3623 **type 3 IP Interface Address TLV is
NOT used** in OSPFv3 (§2): OSPFv3 identifies all neighbours by Router ID, so the
helper keys the restart on the Grace-LSA's Advertising Router and needs no
interface-address TLV. The **Link State ID of the Grace-LSA is the Interface ID**
of the originating interface (§2.2), NOT an opaque type/ID pair. The grace period
is measured by the Grace-LSA's LS age (which MUST start at 0 and MUST NOT be
reset on retransmit), so the helper's expiry timer reads LS age against the Grace
Period TLV, not a separate clock.

**Restarter behaviour (RFC 3623 §2/§2.1/§2.2/§2.3/§5, inherited; plus RFC 5187
§3.1/§3.2):** on an operator-triggered (planned) restart, before reload the
router ensures its FIB is current and will persist across the control-plane
restart, originates one Grace-LSA per OSPFv3 interface (LS age 0, requested grace
period, reason, LS ID = the interface's Interface ID), reliably floods them
(retransmit until acked), and records the restart fact + grace-period end in NVS.
After the software resumes, while in graceful restart the router does NOT
originate OSPFv3 self-LSA types (Router / Network / Link / Intra-Area-Prefix /
Inter-Area-Prefix / Inter-Area-Router / External / NSSA -- it relies on its
pre-restart LSAs), does NOT modify or flush received self-originated LSAs, runs
SPF (RFC 5340 §4.8) to restore virtual links but does NOT install OSPFv3 routes
(it relies on the pre-restart FIB preserved by the fib-kernel `RTPROT_ZE`
stale-mark-then-sweep), and re-elects itself DR on a segment if it was DR before
the restart. **OSPFv3-specific (RFC 5187):** it MUST preserve the LSA-ID->prefix
correspondence for Inter-Area-Prefix and External LSAs (§3.1, arbitrary 32-bit
LSA IDs), and MUST preserve the OSPFv3 Interface ID across the restart (§3.2) so
that pre-restart Link-LSAs, Network-LSAs, and Router-LSA link descriptions still
match neighbour adjacency state. It exits graceful restart when any of: all
adjacencies are re-established, an LSA inconsistent with the pre-restart
Router-LSA is received, or the grace period expires; on exit it re-originates its
self-LSAs, re-runs SPF and installs IPv6 routes, removes stale pre-restart FIB
entries, flushes stale received self-LSAs, and flushes its own Grace-LSAs.

**Helper behaviour (RFC 3623 §3/§3.1/§3.2, inherited):** on receiving a Grace-LSA
from neighbour X on a segment, the router enters helper mode for X **only if all
checks pass** -- Full adjacency with X, no content change in the LSDB since X
restarted, the grace period not yet expired (LS age < Grace Period), local policy
permits, and the helper is not itself restarting. While helping, the router
continues to advertise the adjacency to X in its Router-LSA (and Network-LSA if
DR) regardless of the adjacency's synchronisation state, keeps X as DR if X was
DR, and suppresses the LSDB churn that would otherwise drop the adjacency. It
exits helper mode for X on any of: the Grace-LSA is flushed, the grace period
expires, or (with strict LSA checking enabled, the default) an LSA is installed
with changed content that would have been flooded to X. On exit it recalculates
the DR, re-originates its Router-LSA (and Network-LSA if DR) so the now-stale
"frozen" adjacency view is corrected.

Both roles, the grace timers, the v3 Grace-LSA carriage, the two preservation
rules, and the FIB-retention coordination with the fib-kernel sweep are in scope.
OSPFv2 Graceful Restart (RFC 3623, carried as an opaque Type-9 LSA) is OUT OF
SCOPE and belongs to `spec-ospf-ext-9-graceful-restart.md`; this spec shares no
packet / LSA code with it (RFC 5340 mandate).

### In scope (this spec)

| Item | Detail |
|------|--------|
| Grace-LSA wire type | A NATIVE OSPFv3 link-scope LSA: function code 11, LS Type 0x000B, U=0/S2=0/S1=0 (RFC 5187 §2.1); a new `LSTypeGrace` in `v3/types`, a typed `GraceLSA` body in `v3/packet`, and verbatim passthrough via the existing `LSA.RawBytes` path |
| Grace-LSA body | The **two** TLVs (Grace Period type 1 / Restart Reason type 2) built and parsed with a 4-octet-aligned v3 TLV codec; the RFC 3623 type-3 IP Interface Address TLV is NOT emitted or required (RFC 5187 §2) |
| Link State ID = Interface ID | The 32-bit Link State ID is the originating interface's OSPFv3 Interface ID (§2.2), associating the Grace-LSA with the link's neighbour adjacency; NOT an opaque type/ID split |
| Link-scope store routing | Broaden the LSDB link-scope predicate (`isLinkLSAType`) so function code 11 routes through the link store / `installLink` / link flooding, mirroring the existing Link-LSA (0x0008) precedent |
| Restarter: pre-restart | Ensure FIB current/persistent; originate one Grace-LSA per OSPFv3 interface (LS age 0, LS ID = Interface ID); reliably flood; persist {restarting, grace-end, reason} to NVS (ZeFS) (RFC 3623 §2.1) |
| Restarter: in-restart suppression | Suppress v3 self-LSA origination (all OSPFv3 self types) and IPv6 route install while the grace window is open; run SPF without installing; keep pre-restart received self-LSAs (RFC 3623 §2) |
| Restarter: OSPFv3 preservation | Preserve the LSA-ID->prefix correspondence for Inter-Area-Prefix / External LSAs (§3.1) and preserve the Interface ID across restart (§3.2); persist both so pre-restart LSAs match neighbour state on resume |
| Restarter: DR re-election | Re-elect self DR on a segment if a Hello in Waiting state lists self as DR (was DR before restart) (RFC 3623 §2) |
| Restarter: exit | Exit on all-adjacencies-up / inconsistent-LSA / grace-expiry; re-originate self-LSAs, install routes, flush stale self-LSAs, flush own Grace-LSAs (RFC 3623 §2.2, §2.3) |
| Restarter: unplanned outage | Config-gated: on cold/unplanned start, send Grace-LSAs before any Hello, reason restricted to 0 or 3, operator can disable (RFC 3623 §5) |
| Helper: entry checks | Full adjacency, LSDB unchanged since restart, grace period not expired, policy permits, helper not restarting (RFC 3623 §3.1) |
| Helper: while-helping | Continue advertising adjacency to X (Router-LSA / Network-LSA), keep X as DR, suppress LSDB churn for the grace window (RFC 3623 §3) |
| Helper: strict LSA checking | Default-on: a changed LSA that would flood to X terminates helping; config-gated relaxation (RFC 3623 §3.2) |
| Helper: exit | Grace-LSA flushed / grace expiry / topology change -> DR recalc + Router/Network-LSA re-origination (RFC 3623 §3.2) |
| FIB retention coordination | The restarter relies on the existing fib-kernel `RTPROT_ZE` stale-mark-then-sweep; this spec ensures the grace window closes (routes re-installed) before the sweep deadline, and the `RemoveAll` on engine stop is NOT invoked on a graceful restart (RFC 3623 §2.1) |
| Grace timers | Grace Period (1-1800 s, suggested default 120 s) measured by Grace-LSA LS age; helper expiry timer; restarter exit timer (RFC 3623 §2.1, RFC 5187 §2.2) |
| Config + CLI + metrics | `graceful-restart` config (restarter support/interval, helper support/strict-checking), `show ipv6 ospf graceful-restart`, Prometheus series |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| OSPFv2 Graceful Restart (RFC 3623; opaque Type-9 Grace-LSA) | `spec-ospf-ext-9-graceful-restart.md` (no shared wire code -- RFC 5340 mandate) |
| Any shared OSPFv2/OSPFv3 packet or LSA package | forbidden by the umbrella; the Grace-LSA codec lives entirely under `internal/plugins/ospf/v3/` |
| New FIB backend or kernel-route mechanism | fib-kernel already provides `RTPROT_ZE` + `sweepDelay`; this spec consumes it, does not extend it |
| BFD-coordinated GR | OSPFv3 BFD is ospfv3-ext-5; a BFD-down during restart degrades to normal restart (no special handling here) |
| OSPFv3 SR / RI / extended-LSA preservation specifics | ospfv3-ext-6 owns those LSAs; if present they follow the same §3.1 arbitrary-LSA-ID preservation rule, validated when ext-6 lands |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/research/ospf-implementation-guide.md` §"Graceful Restart and Helper (RFC 3623)" (~1538-1541) and the RFC 5187 references (~1654 `0x000B` Grace-LSA, ~1873-1874) -- the FRR landscape: grace-LSAs announce the restart window; helpers suppress LSDB churn; helper mode is strictly receive-side; FRR splits GR into `ospf_gr.c` / `ospf_gr_helper.c`
  -> Decision: implement BOTH roles in one spec, but build the wire + helper first (receive-side, lower risk) then the restarter on top, matching the guide's "support helper mode first, defer the restarter" ordering as the phase order, not as a scope cut
  -> Constraint: the v3 Grace-LSA is a NATIVE link-scope LSA (function code 11, LS Type 0x000B), NOT an opaque LSA; this spec adds the v3 type + body + link-store routing, with no opaque carrier (the umbrella's "no opaque carrier in v3" decision)
- [ ] `plan/spec-ospfv3-ext-0-umbrella.md` "Child Decomposition" (ext-4 row) + "Dependency / Build Order" + "Out of scope (rested)" -- this child's coordinating umbrella
  -> Constraint: ext-4 depends only on the delivered OSPFv3 base; it shares no packet / LSA / SPF code with the OSPFv2 ext set (RFC 5340 mandate); the Grace-LSA is a native LSA because v3 has no opaque carrier
  -> Decision: mirror the STRUCTURE of `spec-ospf-ext-9-graceful-restart.md` (the v2 GR spec) -- the two state machines, the FIB-retention coupling, the config/CLI/metrics shape -- but with v3 encodings; never reuse v2 wire code
- [ ] `plan/spec-ospf-ext-9-graceful-restart.md` -- the delivered-shape OSPFv2 GR spec this mirrors
  -> Constraint: reuse the control-plane PATTERNS (in-restart suppression flag gating `originateSelfLSAs` + the route install, the per-neighbour helping flag in the topology builder, the LS-age grace clock, the NVS restart-fact via the boot-count store, the fib-kernel retention coupling); the v2 spec is a CONSUMER of the opaque carrier, this v3 spec is a NATIVE-LSA producer -- the wire half differs
  -> Constraint: the v2 Grace-LSA has THREE TLVs and an opaque LS-ID; the v3 Grace-LSA has TWO TLVs (no IP Interface Address) and LS-ID = Interface ID (RFC 5187 §2/§2.2) -- do not copy the v2 body
- [ ] `ai/rules/plugin-self-containment.md` -- the GR feature must be self-contained
  -> Constraint: removing GR removes the `LSTypeGrace` registration in the v3 codec dispatch, the `graceful-restart` config, the `show ipv6 ospf graceful-restart` command, the doctor check, and all GR metrics; no GR spelling appears in the OSPFv2 plugin or in generic OSPF packages
- [ ] `ai/rules/buffer-first.md` -- the Grace-LSA body encode is buffer-first
  -> Constraint: the two TLVs are emitted through a v3 TLV builder into a caller-owned buffer (`WriteTo(buf, off) int`), and the 4-octet pad is written explicitly; no `+`/`fmt` string building of the body; the TLV iterator returns zero-copy views over the caller's bytes
- [ ] `ai/rules/qemu-testing.md` -- GR is Linux-only (raw IPv6 / multicast flood, real FIB retention)
  -> Constraint: the FIB-retention and FRR `ospf6d` interop validation run as QEMU integration tests; "needs hardware / needs a real restart" is not a reason to skip

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5187.md` -- OSPFv3 Graceful Restart (the feature spec)
  -> Constraint: §2.1 -- the Grace-LSA is LS Type 0x000B (function code 11), flooding-scope bits U=0/S2=0/S1=0 (link-local); never flood it beyond the originating link
  -> Constraint: §2.2 -- the Link State ID is the Interface ID of the originating interface; the body is TLV-encoded per RFC 3630 §2.3.2 (4-octet aligned, Length is the unpadded value length); both the Grace Period (type 1, Length 4) and the Graceful Restart Reason (type 2, Length 1) TLVs MUST always be present
  -> Constraint: §2 -- the RFC 3623 router-address (IP Interface Address, type 3) TLV is NOT required and is not emitted; OSPFv3 keys neighbours by Router ID
  -> Constraint: §3.1 -- the restarting router MUST preserve the LSA-ID->prefix correspondence across restarts for Inter-Area-Prefix and External LSAs whose LSA ID is an arbitrary 32-bit integer (avoids network churn)
  -> Constraint: §3.2 -- the OSPFv3 Interface ID MUST be preserved across restarts; a renumbered Interface ID makes pre-restart Link/Network/Router LSAs mismatch neighbour state and terminates the restart prematurely
  -> Constraint: §State Machine -- RFC 5187 defines no new FSM; the restarter and helper procedures are inherited unchanged from RFC 3623; only the wire (§2) and the two preservation rules (§3) are v3-specific
  -> Constraint: §Pitfalls -- LS age is the grace clock (MUST be 0 at first origination, MUST NOT be reset on retransmit); the Restart Reason TLV declares Length 1 but occupies 4 padded octets; a decoder advancing by Length rather than padded length misparses
- [ ] `rfc/short/rfc5340.md` -- OSPFv3 base (the wire substrate the Grace-LSA plugs into)
  -> Constraint: §A.4.2.1 -- the 16-bit LS Type carries scope in the top 3 bits (U/S2/S1) + a 13-bit function code; the LSDB keys by (LS Type, Link State ID, Advertising Router); a link-local LSA MUST never be flooded beyond its originating link
  -> Constraint: §A.3.2 / §A.4.3 -- the Interface ID is a router-local 32-bit identifier; it appears in Hello, Router-LSA, Network-LSA, Link-LSA, and the SPF graph, and is the value the Grace-LSA's Link State ID carries (§3.2 makes its stability load-bearing)
  -> Constraint: the 20-byte LSA header (no Options byte) and the Fletcher LS checksum are shared by every v3 LSA including the Grace-LSA; the codec already retains unknown LSAs verbatim (`LSA.RawBytes`) so a received Grace-LSA round-trips byte-for-byte

**Key insights:** (minimal context to resume after compaction)
- The v3 Grace-LSA is a NATIVE link-scope LSA (function code 11), not an opaque LSA -- the single biggest difference from the v2 GR spec. The work splits into (a) a small wire half (new LS type + 2-TLV body + link-store routing) and (b) the two control-plane state machines (the hard part, inherited unchanged from RFC 3623).
- The two OSPFv3-only rules are §3.1 (preserve arbitrary 32-bit LSA-IDs for Inter-Area-Prefix / External LSAs) and §3.2 (preserve the Interface ID). Both are preservation/persistence requirements, not wire fields; if the restarting router re-numbers an Interface ID or re-assigns a prefix's LSA-ID, the restart silently terminates early.
- FIB retention is NOT new code: the fib-kernel `RTPROT_ZE` + `sweepDelay` stale-mark-then-sweep already keeps kernel routes across a process restart. The restarter's job is to (a) NOT call `Installer.RemoveAll` on a graceful stop, and (b) close the grace window (re-install routes) before the fib sweep deadline.
- "Suppress self-LSA origination during restart" = gate `v6OriginateSelf` (via `originateSelfLSAs` in instance.go) and the SPF route install (`spf.Installer.Apply`) behind a per-engine "in graceful restart" flag.
- "Helper freezes the adjacency at Full" = the helper continues to advertise the link to X in its Router-LSA even though the NSM may regress; it does NOT mean the NSM is frozen. The v3 Router-LSA topology builder (`v6OriginateRouter`) must keep X's link while helping.
- LS age is the grace clock: the helper's expiry timer reads the Grace-LSA's LS age vs the Grace Period TLV; never a separate wall clock for the window length.
- Backward compatibility is automatic: no capability negotiation; a non-helper neighbour reverts the restart to a normal restart with no loops.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/v3/types/lsa.go` -- the OSPFv3 `LSType` (16-bit, scope embedded), the eight `Known()` base types, `Scope()` (S2/S1 -> floodScope), and `LSAKey` = (LSType, LinkStateID, AdvertisingRouter); there is NO Grace function code yet
  -> Constraint: add `LSTypeGrace LSType = 0x000B` (U=0/S2=0/S1=0 link-local, function code 11) and include it in `Known()` so the codec/LSDB treat it as a recognised link-local LSA; the LS-ID stays a plain 4-byte `LinkStateID` (the Interface ID), no opaque split
- [ ] `internal/plugins/ospf/v3/packet/lsa.go` -- `LSAHeader` (no Options byte), the typed-body `LSA` with `RawBytes` verbatim passthrough (`WriteTo` re-emits `RawBytes` when no typed body is set), `DecodeLSA`/`EncodedLen`/`VerifyChecksum` (Fletcher), `LSAIterator` (bound-checked)
  -> Constraint: the codec already round-trips an unknown link-scope LSA verbatim; add a typed `Grace *GraceLSA` body field + a `DecodeGrace()` method, mirroring the existing `Link *LinkLSA` precedent, so a self-originated Grace-LSA encodes through `WriteTo` (Length + Fletcher recomputed) and a received one re-floods via `RawBytes`
- [ ] `internal/plugins/ospf/v3/packet/lsa_link.go` -- the Link-LSA body codec (`LinkLSA`, `decodeLinkLSA`, `EncodedLen`, `WriteTo`): the link-local body precedent the Grace-LSA body mirrors (decode is bound-checked, encode is buffer-first, body must consume exactly)
  -> Constraint: the `GraceLSA` body + the v3 TLV iterator/builder live alongside this file under `v3/packet`; follow the same bound-checked-decode / buffer-first-encode / consume-exactly discipline
- [ ] `internal/plugins/ospf/origination_v6_link.go` -- `v6OriginateLinkLSA(router, iface)` originates a Link-LSA via `e.lsdb.OriginateLinkSelf(iface.Name, areaID, key, bodyBytes, enc)` with a `v6OriginHeader(LSTypeLink, LinkStateID, router, seq, purge)` + a typed body; `v6LinkKey(router, ifaceID)` builds the link-store key
  -> Constraint: Grace-LSA origination reuses this exact path -- `OriginateLinkSelf` + a `v6OriginHeader(LSTypeGrace, InterfaceID-as-LSID, router, seq, purge)` + the typed `GraceLSA` body; the LS-ID is the Interface ID (`v6SummaryLSID(iface.InterfaceID)` already maps a uint32 to a `LinkStateID`)
- [ ] `internal/plugins/ospf/origination_v6.go` -- `v6OriginateSelf(router, maxMetric)` is the v3 self-LSA origination chokepoint (per-link Link-LSAs, Router-LSA, Intra-Area-Prefix, DR Network-LSA, stale-flush via `FlushStaleSelfLSAs`/`FlushStaleLinkSelfLSAs`); `v6ManagedSelfTypes`; `v6OriginHeader`/`v6SelfLSA`
  -> Constraint: the restarter's "suppress origination" gate wraps `v6OriginateSelf` (return early while the grace window is open, like the v2 GR spec gates `originateSelfLSAs`); the §3.1/§3.2 preservation requires that the LSA-IDs and Interface IDs `v6OriginateSelf` would assign are STABLE across restart (the Interface ID already comes from `iface.InterfaceID`; the preservation rule pins that source across reboots)
- [ ] `internal/plugins/ospf/instance.go` -- `originateSelfLSAs()` dispatches to `v6OriginateSelf` when `codec.IsV6()`; `handleLSUpdate` -> `e.lsdb.ReceiveUpdate(...)`; `shutdown()` stops SPF, cancels the context, closes the transport, waits the WG
  -> Constraint: the in-restart flag gates `originateSelfLSAs` for the v6 path; helper reception hangs off `ReceiveUpdate` (after the link-store install, the engine inspects the installed LSA type and dispatches a Grace-LSA to the helper); the graceful stop is a variant of `shutdown()` that skips the FIB `RemoveAll` and persists the restart-fact
- [ ] `internal/plugins/ospf/lsdb/link_scope.go` -- the link-local store (`d.links`, `linkForLocked`), `OriginateLinkSelf` (gated on `isLinkLSAType(key.Type)`), `installLink`/`installLinkLocked`, `LinkLSAs(iface)`, `FlushStaleLinkSelfLSAs`; `isLinkLSAType` currently returns `t == types.LSTypeLink` ONLY
  -> Constraint: broaden `isLinkLSAType` (or add a sibling link-scope predicate) so function code 11 (`LSTypeGrace`) also routes through `installLink`/`OriginateLinkSelf`/`LinkLSAs`, mirroring the Type-8 Link-LSA precedent; this is the single store-routing chokepoint, like the v2 ext-1 opaque framework broadened the v2 link predicate for Type-9
- [ ] `internal/plugins/ospf/lsdb/flooding.go` -- `ReceiveUpdate` runs the §13 receive and the link-store path for link-scope LSAs; `notifyChange` signals a content change; the helper's "LSDB unchanged since restart" check and the strict-checking "changed LSA that would flood to X" exit trigger read this path
  -> Constraint: the helper hooks the post-install content-change signal (the same signal `notifyChange` raises) to evaluate the §3.2 strict-checking exit; it does NOT change §13 receive semantics; the Grace-LSA reception itself is delivered to the helper from the engine after a Newer link-store install
- [ ] `internal/plugins/ospf/spf/install.go` -- `Installer.Apply` diffs computed routes and inserts/removes `locrib.Path`; `RemoveAll` withdraws every OSPF path; `loc` may be nil in a forked subprocess
  -> Constraint: during restart the restarter must NOT call `Apply` (no route churn) and must NOT call `RemoveAll` on the graceful stop; on exit it resumes `Apply` so the IPv6 Loc-RIB is reconciled and the fib-kernel sweep refreshes routes instead of deleting them (the same seam the v2 GR spec gates -- shared across families)
- [ ] `internal/plugins/fib/kernel/backend.go` + `backend_linux.go` -- routes marked `RTPROT_ZE` (250); startup stale-mark-then-sweep refreshes ZE routes re-installed within `sweepDelay` and sweeps the rest
  -> Constraint: this is exactly the FIB-retention substrate -- kernel IPv6 routes survive the OSPF subprocess restart and are refreshed when SPF re-installs them on GR exit; the grace period and the restarter exit MUST complete within `sweepDelay` (or the relationship must be reconciled) so non-stop forwarding holds; this coupling is a design constraint, not new code
- [ ] `internal/plugins/ospf/neighbor/neighbor.go` + `nsm.go` + `table.go` -- the `Neighbor` record (State, Options, RouterID, Address), the NSM (Down..Full), `FloodNeighbor`/`Snapshot`, the neighbor table; OSPFv3 keys neighbours by Router ID
  -> Constraint: the helper does NOT add a neighbor state; it adds a per-neighbour "helping (restart-in-progress)" flag consulted by the v3 Router-LSA topology builder and the LSDB-churn suppressor; the NSM stays RFC 5340; the restarting neighbour is identified by the Grace-LSA's Advertising Router (Router ID), no interface-address lookup (RFC 5187 §2)
- [ ] `internal/plugins/ospf/auth_keystore.go` -- `loadOSPFBootCount(store)` reads a ZeFS blob, increments, writes back; `openBootCountStore()` opens the `pkg/zefs` blob store; `bootCountStore` interface
  -> Constraint: the restart-fact NVS ({restarting, grace-end, reason, preserved Interface-ID + LSA-ID maps}) reuses this ZeFS blob-store pattern (a sibling blob), so a planned restart survives the process restart without a new persistence subsystem; the §3.1/§3.2 preserved state persists here too
- [ ] `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` -- `max-metric/router-lsa/on-shutdown` (RFC 6987 stub-router seconds) is the precedent for a restart/shutdown timer config; `ospfConfig` carries `MaxMetric maxMetricConfig`
  -> Constraint: the `graceful-restart` config follows the `max-metric` shape (a sibling container with restarter + helper sub-containers); GR is a distinct mechanism (Grace-LSA + FIB retention), not stub-router; the same YANG file serves both v2 and v3 (the config is family-neutral, the codec path selects the behaviour)

**Behavior to preserve:**
- The delivered OSPFv3 base: the RFC 5340 v3 codec (`v3/packet`, `v3/types`), the link-local Link-LSA origination (`v6OriginateLinkLSA`), the v3 self-LSA chokepoint (`v6OriginateSelf`), the shared LSDB link store, the SPF route table + `Installer.Apply` insert/remove shape, the fib-kernel `RTPROT_ZE` + `sweepDelay` reconciliation, the shared neighbor NSM.
- The `LSA.RawBytes` verbatim passthrough for unknown / received link-scope LSAs and the `(LSType, LinkStateID, AdvertisingRouter)` LSDB key.
- All existing OSPFv3 functional / FRR `ospf6d` interop tests: a router with GR disabled (the default) behaves exactly as today -- it originates no Grace-LSA, never enters helper mode, and restarts normally.
- No OSPFv2 code is touched: the v2 GR spec (`spec-ospf-ext-9`) and its opaque-carried Grace-LSA are independent (RFC 5340 mandate).

**Behavior to change:** (all RFC-5187 / inherited-RFC-3623-required, gated behind GR config)
- `internal/plugins/ospf/v3/types/lsa.go`: add `LSTypeGrace` (0x000B) + include it in `Known()`.
- `internal/plugins/ospf/v3/packet`: add the typed `GraceLSA` body + `DecodeGrace()` + the v3 TLV iterator/builder.
- `internal/plugins/ospf/lsdb/link_scope.go`: broaden the link-scope predicate so function code 11 routes through the link store.
- `v6OriginateSelf` (via `originateSelfLSAs`): return early (suppress) while this engine is in graceful restart.
- The SPF route install: skip `Installer.Apply` while in graceful restart; do NOT `RemoveAll` on a graceful stop.
- The v3 Router-LSA topology builder (`v6OriginateRouter`): while helping X, keep X's link advertised even if the NSM regressed; keep X as DR if X was DR.
- Engine stop: distinguish a graceful restart (preserve FIB, persist restart-fact + Interface-ID/LSA-ID maps, originate Grace-LSAs) from a normal shutdown.
- LSDB receive: surface the post-install content-change signal to the helper's strict-checking exit and to the restarter's inconsistent-LSA exit.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Restarter trigger:** an operator command (`ospf graceful-restart prepare`, or a managed reload) -> the engine enters the pre-restart phase -> originate Grace-LSAs (one per OSPFv3 interface, LS ID = Interface ID) -> persist NVS (restart-fact + preserved Interface-ID / LSA-ID maps) -> stop without `RemoveAll`. After the subprocess restarts, the persisted restart-fact puts the engine into the in-restart phase.
- **Helper trigger:** an LS Update carrying a link-scope Grace-LSA (LS Type 0x000B) arrives -> the LSDB link-store install -> the engine inspects the installed LSA, decodes the Grace-LSA, and dispatches it to the helper -> the helper evaluates the §3.1 entry checks for the originating neighbour (identified by the Advertising Router).
- **Exit triggers:** restarter -- all-adjacencies-up / inconsistent-LSA / grace-timer; helper -- Grace-LSA flushed / grace-timer / strict-checking topology change.

### Transformation Path
1. **Pre-restart (restarter):** ensure the FIB is current (the last `Installer.Apply` has settled); for each OSPFv3 interface build a `GraceLSA` body via the v3 TLV builder (type 1 Grace Period = configured interval, type 2 Reason); originate via `e.lsdb.OriginateLinkSelf(iface.Name, area, key{LSTypeGrace, InterfaceID, router}, body, enc)` with LS age 0 (the carrier reliably floods); persist {restarting, grace-end, reason} plus the §3.1 LSA-ID->prefix map and the §3.2 Interface-ID assignments to the ZeFS NVS blob.
2. **Graceful stop (restarter):** the engine stops WITHOUT `Installer.RemoveAll`; kernel IPv6 routes (RTPROT_ZE) remain; the subprocess exits.
3. **In-restart (restarter, after resume):** the persisted restart-fact (grace-end not yet passed) sets the in-restart flag; the §3.2 Interface IDs and §3.1 LSA-IDs are restored from NVS so re-originated LSAs match pre-restart values; `v6OriginateSelf` is suppressed; `Installer.Apply` is suppressed; SPF runs (virtual-link restore) without installing; received self-LSAs are NOT flushed; DR re-election runs if a Hello in Waiting state lists self as DR.
4. **Helper entry (helper):** the engine decodes the Grace-LSA's two TLVs via the v3 iterator; identifies X by the Advertising Router (Router ID -- no interface-address TLV in v3); runs the §3.1 checks; on pass, sets the per-neighbour helping flag, records grace-end = now + min(Grace Period TLV - LS age, remaining), keeps X as DR if X was DR; if already helping X, just updates the grace period.
5. **While helping (helper):** the v3 Router-LSA topology builder (`v6OriginateRouter`) keeps X's link advertised (and the Network-LSA if DR) regardless of NSM state; the LSDB-churn suppressor holds; the strict-checking watcher evaluates each installed LSA's content-change-that-would-flood-to-X signal.
6. **Restarter exit:** on the earliest of the three triggers, clear the in-restart flag, re-run `v6OriginateSelf` (Router-LSAs all areas, Link-LSAs, Intra-Area-Prefix, Network-LSAs where DR -- with preserved Interface-IDs / LSA-IDs), re-run SPF + `Installer.Apply` (routes re-installed -> fib-kernel sweep refreshes them), `FlushStaleSelfLSAs` / `FlushStaleLinkSelfLSAs` for now-stale self-LSAs, and originate the Grace-LSAs at MaxAge to flush them; clear the NVS restart-fact.
7. **Helper exit:** on the earliest trigger, clear the per-neighbour helping flag, recalc DR for the segment, re-originate the Router-LSA (and Network-LSA if DR) so the frozen adjacency view is corrected.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator / managed reload <-> restarter | `ospf graceful-restart prepare` RPC (or a managed-reload hook) sets the pre-restart phase | [ ] |
| Grace-LSA body <-> v3 codec | a typed `GraceLSA` body in `v3/packet`; built/parsed with the v3 4-octet TLV builder/iterator; LS Type 0x000B, LS ID = Interface ID | [ ] |
| Grace-LSA <-> LSDB link store | function code 11 routed through `installLink` / `OriginateLinkSelf` / `LinkLSAs` via the broadened link-scope predicate | [ ] |
| Restarter <-> self-LSA origination | the in-restart flag gates `v6OriginateSelf` (via `originateSelfLSAs`) | [ ] |
| Restarter <-> route install | the in-restart flag gates `spf.Installer.Apply`; graceful stop skips `RemoveAll` | [ ] |
| Restarter <-> FIB retention | fib-kernel `RTPROT_ZE` routes persist; SPF re-install on exit refreshes them within `sweepDelay` | [ ] |
| Restarter <-> NVS | `{restarting, grace-end, reason}` + §3.1 LSA-ID map + §3.2 Interface-ID map persisted via the `pkg/zefs` blob store (sibling of the boot-count blob) | [ ] |
| Helper <-> Router-LSA builder | the per-neighbour helping flag keeps X's link advertised in `v6OriginateRouter` topology | [ ] |
| Helper <-> LSDB churn | the helping flag + the post-install content-change signal drive the §3.2 strict-checking exit | [ ] |

### Integration Points
- `internal/plugins/ospf/v3/types` -- the new `LSTypeGrace` (0x000B) + `Known()` inclusion.
- `internal/plugins/ospf/v3/packet` -- the typed `GraceLSA` body, `DecodeGrace()`, and the v3 TLV iterator/builder.
- `internal/plugins/ospf/lsdb` -- the broadened link-scope predicate; `OriginateLinkSelf` (Grace-LSA origination), `LinkLSAs` (lookup), `FlushStaleLinkSelfLSAs` (exit cleanup), the post-install content-change signal (helper strict checking).
- `internal/plugins/ospf` (engine, v6 path) -- the in-restart flag, the helper map, the pre-restart/exit orchestration; gates `v6OriginateSelf` and the route install; the restart-fact NVS; the Grace-LSA origination glue (`v6OriginateGraceLSAs`) reusing `v6OriginateLinkLSA`'s pattern; the Grace-LSA reception dispatch from `handleLSUpdate`/`ReceiveUpdate`.
- `internal/plugins/ospf/neighbor` -- the per-neighbour helping flag surfaced into the topology snapshot; DR-was-X preservation.
- `internal/plugins/ospf/spf` -- READ/gate: `Installer.Apply` suppressed during restart, resumed on exit; SPF still computed (virtual-link restore) but not installed.
- `internal/plugins/fib/kernel` -- READ ONLY: the `RTPROT_ZE` + `sweepDelay` retention substrate (no code change; the coupling is verified by the FIB-retention QEMU test).
- `internal/plugins/ospf/auth_keystore.go` -- the ZeFS NVS blob-store pattern reused for the restart-fact + the preserved Interface-ID / LSA-ID maps.
- `internal/plugins/ospf/config.go` + `yang/` -- the `graceful-restart` config and the `show ipv6 ospf graceful-restart` command.

### Architectural Verification
- [ ] No bypassed layers (Grace-LSA flows wire -> v3 codec -> link-store install -> engine helper dispatch; origination flows the GR glue -> `OriginateLinkSelf` link-store self-origination; no direct LSDB poke)
- [ ] No unintended coupling (no GR spelling in the OSPFv2 plugin or generic OSPF packages; fib-kernel read-only; the v3 GR code shares no packet/LSA code with the v2 GR spec)
- [ ] No duplicated functionality (reuses `v6OriginateSelf`, `OriginateLinkSelf`, `FlushStaleSelfLSAs`/`FlushStaleLinkSelfLSAs`, `Installer.Apply`/`RemoveAll`, the fib-kernel sweep, the ZeFS blob store, the link-local store; adds only the Grace-LSA type/body, the link-predicate broadening, the two state machines, the gates, and the config/CLI/metrics)
- [ ] Zero-copy preserved (Grace-LSA body built buffer-first via the v3 TLV builder; TLV parse is a zero-copy iterator view; received Grace-LSAs re-flood from `RawBytes`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The v3 codec round-trips an unknown / new link-scope LSA verbatim via `LSA.RawBytes`, so a received Grace-LSA re-floods byte-for-byte and a self one encodes through `WriteTo` (Length + Fletcher recomputed) | `internal/plugins/ospf/v3/packet/lsa.go` `WriteTo`/`DecodeLSA`/`VerifyChecksum`; the `Link *LinkLSA` typed-body precedent | the codec needs new passthrough work; scope creep | `TestGraceLSARoundTrip` (decode then re-encode a 0x000B LSA byte-for-byte) + decode an FRR `ospf6d` Grace-LSA capture | unvalidated |
| A-2 | Broadening `isLinkLSAType` (or a sibling predicate) to include function code 11 routes the Grace-LSA through `installLink` / `OriginateLinkSelf` / `LinkLSAs` with no new store | `internal/plugins/ospf/lsdb/link_scope.go` `isLinkLSAType` (matches only `LSTypeLink`); the Type-8 precedent; v2 ext-1 broadened the v2 link predicate identically | a new link-scope store/key is needed; larger change | `TestGraceLSALinkScopeRouting` (a 0x000B LSA lands in the link store, never an area store) | unvalidated |
| A-3 | `e.lsdb.OriginateLinkSelf` + a `v6OriginHeader(LSTypeGrace, InterfaceID, ...)` + a typed `GraceLSA` body originates a link-local Grace-LSA at LS age 0, reusing `v6OriginateLinkLSA`'s pattern with no new origination code | `internal/plugins/ospf/origination_v6_link.go` `v6OriginateLinkLSA`; `lsdb/link_scope.go` `OriginateLinkSelf` (gated on `isLinkLSAType`) | a new link origination path is needed | `TestGraceLSAOriginated` (one Grace-LSA per OSPFv3 interface, LS ID = Interface ID) | unvalidated |
| A-4 | The v3 TLV codec (RFC 3630 §2.3.2, 4-octet aligned) round-trips the two Grace-LSA TLVs against FRR `ospf6d`, with the Restart Reason declaring Length 1 but occupying 4 padded octets | `rfc/short/rfc5187.md` §2.2 (TLV format, padding); the absence of any existing v3 TLV codec (must be added) | the body is mis-padded; FRR rejects the Grace-LSA | `TestGraceLSATLVRoundTrip` (type-1 / type-2 values, 4-octet pad) + `ospf-v6-gr-frr` interop | unvalidated |
| A-5 | The fib-kernel `RTPROT_ZE` routes survive the OSPF subprocess restart and the `sweepDelay` window is long enough for a default grace period IF the restarter re-installs routes early; if not, the sweep deletes them before GR exit | `internal/plugins/fib/kernel/backend.go` `sweepDelay`; `backend_linux.go` stale-mark-then-sweep | the FIB is swept mid-restart -> black hole (the exact thing GR must prevent) | `ospf-v6-gr-fib-retention` QEMU test: IPv6 routes stay programmed across a graceful restart; design reconciles grace period vs `sweepDelay` | unvalidated |
| A-6 | `v6OriginateSelf` is the single v3 self-LSA origination chokepoint, so gating it suppresses ALL OSPFv3 self-LSA types during restart | `internal/plugins/ospf/origination_v6.go` `v6OriginateSelf`; `instance.go` `originateSelfLSAs` dispatch | a v3 self-LSA leaks during restart (violates RFC 3623 §2) | `TestRestarterSuppressesV6SelfLSAs` (no Router/Network/Link/Intra-Area-Prefix/Summary/External self-LSA re-origination while in restart) | unvalidated |
| A-7 | The restarter can run SPF for virtual-link restore without installing IPv6 routes by suppressing `Installer.Apply` while leaving the SPF computation running | `internal/plugins/ospf/spf/install.go` `Apply` is the only install seam; the SPF compute is separate | SPF compute and install are entangled | `TestRestarterRunsSPFNoInstall` (SPF table populated, `loc` unchanged, RTPROT_ZE routes retained) | unvalidated |
| A-8 | A helper can keep X's link advertised in its v3 Router-LSA across a transient NSM regression by consulting a per-neighbour helping flag in `v6OriginateRouter`, without freezing the NSM | `internal/plugins/ospf/origination_v6.go` `v6OriginateRouter` builds the Router-LSA from the topology; `neighbor.go` `FloodNeighbor` | the helper drops X's link, prematurely terminating X's restart (and X's Interface-ID match breaks per §3.2) | `TestHelperKeepsAdjacencyAdvertised` (Router-LSA still lists X while helping) | unvalidated |
| A-9 | The post-install content-change signal (`notifyChange` in `lsdb/flooding.go`) is sufficient to drive the §3.2 strict-checking exit without a new flooding hook | `internal/plugins/ospf/lsdb/flooding.go` `ReceiveUpdate` -> `notifyChange` | strict checking needs new per-LSA "would-flood-to-X" plumbing | `TestHelperStrictExitOnTopologyChange` + the stub-area exception test | unvalidated |
| A-10 | The ZeFS boot-count blob-store seam can persist a second small blob (restart-fact + §3.2 Interface-ID map + §3.1 LSA-ID->prefix map) for planned-restart survival | `internal/plugins/ospf/auth_keystore.go` `bootCountStore` + `pkg/zefs` | a new NVS subsystem is needed | `TestRestartFactPersistsAcrossRestart` (write, re-open, read grace-end + the preserved maps) | unvalidated |
| A-11 | The OSPFv3 Interface ID source (`iface.InterfaceID`, today derived per interface) is stable enough to preserve across a planned restart by persisting and restoring it, satisfying §3.2 | `internal/plugins/ospf/origination_v6_link.go` uses `iface.InterfaceID`; RFC 5187 §3.2 | a renumbered Interface ID silently terminates the restart (pre-restart Link/Network/Router LSAs mismatch) | `TestInterfaceIDPreservedAcrossRestart` (the resumed router re-originates Link/Network/Router LSAs with the SAME Interface IDs) | unvalidated |
| A-12 | The arbitrary 32-bit LSA-ID assigned to Inter-Area-Prefix / External LSAs (`v6SummaryLSID` / the external origination LS-ID allocator) can be preserved across restart by persisting the prefix->LSA-ID map, satisfying §3.1 | `internal/plugins/ospf/origination_v6_summary.go` `v6SummaryLSID`; `origination_v6_external.go` | a re-assigned LSA-ID churns the network on restart (the §3.1 failure) | `TestLSAIDPrefixCorrespondencePreserved` (a resumed ABR/ASBR re-originates each prefix with the same LSA ID) | unvalidated |
| A-13 | GR with default config OFF is fully backward compatible: no Grace-LSA originated, no helper entry, normal restart; existing OSPFv3 tests stay green | RFC 5187 inherits RFC 3623 §4 (automatic backward compat); the default-off config | enabling the feature changes default behaviour; interop regression | existing OSPFv3 suite green with GR disabled; `TestGRDisabledNoGraceLSA` | unvalidated |
| A-14 | The DR-was-X state needed for "keep X as DR while helping" / "re-elect self DR" is recoverable from the pre-restart Hello/election state (Hello in Waiting state lists self as DR for the restarter; the helper records X's DR role at entry) | `internal/plugins/ospf/iface` election + `neighbor.go`; the shared DR machinery | DR continuity is lost across restart -> election churn defeats GR | `TestRestarterReElectsSelfDR`, `TestHelperKeepsXAsDR` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The fib-kernel sweep deletes OSPFv3 routes mid-restart (grace period > `sweepDelay`) -> the exact black hole GR must prevent | `ospf-v6-gr-fib-retention` shows a forwarding gap; kernel loses RTPROT_ZE IPv6 routes during the window | re-install routes on the FIRST SPF after resume (not only on full GR exit); pin the relationship between the grace window, the restarter exit, and `sweepDelay`; the FIB-retention QEMU test is the gate |
| R-2 | A v3 self-LSA leaks during restart (a code path other than `v6OriginateSelf` originates) -> violates RFC 3623 §2, peers see a changed Router/Link-LSA, helpers exit early | a peer logs a Router-LSA change for the restarting router during the window | audit ALL v3 origination call sites; the in-restart flag gates `v6OriginateSelf` AND the stale-flush; `TestRestarterSuppressesV6SelfLSAs` asserts no leak across all v3 self types |
| R-3 | The Interface ID is renumbered across restart (the §3.2 pitfall) -> pre-restart Link/Network/Router LSAs mismatch neighbour state -> helpers terminate early, silently | the restart fails over to a normal reconvergence even though helpers were present; X exits GR early | persist and restore the Interface-ID assignments in the restart-fact NVS; `TestInterfaceIDPreservedAcrossRestart`; document the IfIndex-stability dependency (RFC 5187 §3.2 / Errata 1453, RFC 2863 §3.1.5) |
| R-4 | A prefix's arbitrary 32-bit LSA-ID is re-assigned across restart (the §3.1 pitfall) -> network churn for Inter-Area-Prefix / External routes | peers withdraw + re-learn the same prefix under a different LSA-ID during the restart | persist and restore the prefix->LSA-ID map in the restart-fact NVS; `TestLSAIDPrefixCorrespondencePreserved` |
| R-5 | The helper drops X's link on a transient NSM regression -> premature termination of X's restart | X exits GR early; the adjacency flaps | the per-neighbour helping flag keeps X's link advertised in `v6OriginateRouter` for the whole grace window; `TestHelperKeepsAdjacencyAdvertised` |
| R-6 | Strict LSA checking fires on a benign change (an AS-External-LSA change that would NOT flood to X in a stub area) -> the helper exits early for no reason | helping ends right after an unrelated external change | implement the §3.2 stub-area exception: only LSAs that WOULD flood to X count; `TestHelperStubAreaExternalDoesNotExit` |
| R-7 | LS age reset on Grace-LSA retransmit extends the grace period indefinitely (the RFC 5187 §Pitfalls / RFC 3623 §A pitfall) | the helper's window never closes; a stuck restarter holds the adjacency forever | originate the Grace-LSA once at LS age 0 and retransmit the SAME instance (reliable flooding); never re-stamp LS age; `TestGraceLSAAgeNotResetOnRetransmit` |
| R-8 | The Restart Reason TLV padding is mishandled (Length 1, 4 padded octets) -> a decoder advancing by Length misparses the body / FRR rejects it | the type-2 TLV parse misaligns; FRR logs a malformed Grace-LSA | the v3 TLV iterator advances by the 4-octet-padded length, not the raw Length; `TestGraceLSATLVRoundTrip` covers the 1-byte reason value |
| R-9 | A forged Grace-LSA spoofs a restart for a router that was actually withdrawn (the §4 security risk) | a router that is really down is held adjacent by helpers | Grace-LSAs ride the existing OSPFv3 auth (RFC 7166 trailer, delivered base); the helper still requires a prior Full adjacency with X; document the residual risk (RFC 5187 §4: manual keying precludes replay protection) |
| R-10 | A planned restart's NVS restart-fact is stale (the process restarted for an unrelated reason after the grace window) -> the engine wrongly suppresses origination on a normal boot | a cold boot starts in in-restart mode with no real GR in flight | the restart-fact records grace-end; on resume, if grace-end has passed, ignore it and boot normally; `TestStaleRestartFactIgnored` |
| R-11 | The unplanned-outage path (§5) sends Grace-LSAs after a crash without a guaranteed-sane FIB -> forwarding on stale entries | a crashed router resumes forwarding on routes that no longer match the topology | unplanned support is config-gated (default off), reason restricted to 0/3, Grace-LSAs before any Hello; the operator opt-in is mandatory; `TestUnplannedDisabledByDefault` |
| R-12 | The grace period exceeds the LSRefreshTime so the restarting router's own LSAs age out mid-restart, defeating GR | the restarter's pre-restart LSAs disappear during the window | YANG `range "1..1800"` on the interval; a doctor/validation warning if the configured interval approaches 1800; `TestGracePeriodRangeRejectsAbove1800` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| GR enabled; the engine registers the Grace-LSA type + state machines on the v6 path | -> | `LSTypeGrace` recognised by the v3 codec/LSDB; the GR feature wired into the v6 engine | `TestGraceLSATypeRegistered` (unit) + `test/ospf/ospf-v6-gr-register.ci` |
| `ospf graceful-restart prepare` (operator/managed) | -> | the engine enters pre-restart, originates one Grace-LSA per OSPFv3 interface (LS ID = Interface ID) via `OriginateLinkSelf`, persists the NVS restart-fact + Interface-ID/LSA-ID maps | `test/ospf/ospf-v6-gr-prepare.ci` |
| An LS Update carrying a link-scope Grace-LSA (0x000B) arrives from X | -> | link-store install -> engine Grace-LSA dispatch -> helper entry checks -> helping flag set; X's link stays advertised | `test/ospf/ospf-v6-gr-helper.ci` |
| The OSPF subprocess restarts with a valid NVS restart-fact | -> | the in-restart flag is set; `v6OriginateSelf` and `Installer.Apply` suppressed; Interface-IDs/LSA-IDs restored; FIB retained | `ospf-v6-gr-fib-retention` (QEMU) |
| The grace period expires (helper) | -> | the helper exits, recalcs DR, re-originates its Router-LSA | `TestHelperExitOnGraceExpiry` (unit) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GR enabled; the v3 codec/LSDB sees an LS Type 0x000B LSA | it is recognised (`Known()` true), routed to the link store, and re-flooded link-local only; an LS Type 0x000B received and stored never appears in an area or AS store (§2.1) |
| AC-2 | A planned restart is requested; one or more OSPFv3 interfaces are up | one Grace-LSA is originated per interface, LS age 0, Link State ID = that interface's Interface ID, with type-1 Grace Period and type-2 Restart Reason TLVs always present, and NO type-3 IP Interface Address TLV (RFC 5187 §2, §2.2) |
| AC-3 | A Grace-LSA is retransmitted (reliable flooding) before it is acked | the SAME instance is retransmitted; LS age is NOT reset (§Pitfalls, R-7) |
| AC-4 | The restart-fact is persisted, then the OSPF subprocess restarts within the grace window | on resume the engine enters in-restart mode (grace-end not passed); a restart-fact whose grace-end has passed is ignored and the engine boots normally (RFC 3623 §2.1, R-10) |
| AC-5 | The router is in graceful restart | it does NOT originate any OSPFv3 self-LSA type (Router / Network / Link / Intra-Area-Prefix / Inter-Area-Prefix / Inter-Area-Router / External / NSSA) and does NOT modify/flush its received self-originated LSAs (RFC 3623 §2) |
| AC-6 | The router is in graceful restart and SPF runs | SPF is computed (virtual-link restore) but NO OSPFv3 routes are installed/removed; the pre-restart FIB (RTPROT_ZE kernel routes) remains programmed (RFC 3623 §2, A-5, A-7) |
| AC-7 | The router restarts | the OSPFv3 Interface ID of every interface is preserved across the restart, so re-originated Link/Network/Router LSAs carry the same Interface IDs as before (RFC 5187 §3.2, R-3) |
| AC-8 | The router restarts as an ABR/ASBR with Inter-Area-Prefix / External LSAs | the LSA-ID->prefix correspondence is preserved across the restart; each prefix is re-originated under the same arbitrary 32-bit LSA ID (RFC 5187 §3.1, R-4) |
| AC-9 | The router was DR on a segment before restart and a Hello in Waiting state lists it as DR | it re-elects itself DR on that segment (RFC 3623 §2, A-14) |
| AC-10 | All adjacencies are re-established (pre-restart Router/Network-LSA reflected by helpers) | the restarter exits graceful restart and runs the exit actions (RFC 3623 §2.2) |
| AC-11 | An LSA inconsistent with the pre-restart Router-LSA is received during restart | the restarter exits graceful restart immediately and runs the exit actions (RFC 3623 §2.2) |
| AC-12 | The grace period expires before all adjacencies are up | the restarter exits graceful restart and runs the exit actions (RFC 3623 §2.2) |
| AC-13 | The restarter exits graceful restart (any trigger) | it re-originates its self-LSAs (preserved Interface-IDs / LSA-IDs), re-runs SPF and installs IPv6 routes (refreshing the RTPROT_ZE routes within `sweepDelay`), flushes stale received self-LSAs, and flushes its own Grace-LSAs at MaxAge (RFC 3623 §2.3) |
| AC-14 | A Grace-LSA is received from X and ALL §3.1 checks pass (Full adjacency, LSDB unchanged since X restarted, grace not expired, policy permits, helper not restarting) | the router enters helper mode for X (identified by the Advertising Router), advertises the adjacency to X (Router-LSA, and Network-LSA if DR) regardless of synchronisation state, and keeps X as DR if X was DR (RFC 3623 §3, §3.1) |
| AC-15 | A Grace-LSA is received from X but at least one §3.1 check fails | the router does NOT enter helper mode (RFC 3623 §3.1) |
| AC-16 | A new Grace-LSA arrives from X while already helping X on the segment | the existing helper relationship is kept and the grace period is updated; no re-entry churn (RFC 3623 §3.1) |
| AC-17 | While helping X: the Grace-LSA is flushed, OR the grace period expires (LS age >= Grace Period), OR (strict checking on) an LSA is installed with changed content that would have flooded to X | the router exits helper mode for X, recalcs the DR, and re-originates its Router-LSA (and Network-LSA if DR) (RFC 3623 §3.2) |
| AC-18 | While helping X in a stub area: a changed AS-External-LSA is installed that would NOT flood to X | helping for X does NOT terminate (the §3.2 stub-area exception, R-6) |
| AC-19 | Unplanned-outage support is disabled (the default) and the router cold-boots without a planned restart-fact | no Grace-LSA is originated; the router boots normally (RFC 3623 §5, R-11) |
| AC-20 | Unplanned-outage support is enabled by the operator | on a cold/unplanned start Grace-LSAs are sent BEFORE any Hello, with reason restricted to 0 (unknown) or 3 (switch to redundant CP) (RFC 3623 §5) |
| AC-21 | The configured grace period (RestartInterval) | accepts 1-1800 s, default 120 s; a value above 1800 is rejected by YANG validation (RFC 3623 §2.1, R-12) |
| AC-22 | GR is disabled (the default) | no Grace-LSA is originated, helper mode is never entered, and a restart behaves exactly as today (backward compatibility, A-13) |
| AC-23 | A received Grace-LSA missing the mandatory Grace Period or Restart Reason TLV | the Grace-LSA is treated as malformed and ignored (no helper entry, no crash); the engine continues (RFC 5187 §2.2 Validation) |
| AC-24 | `show ipv6 ospf graceful-restart` | reports the restarter state (in-restart / not, grace-end, reason) and the per-neighbour helper state (helping which neighbours, remaining grace) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables OSPFv3 GR and triggers a planned restart; IPv6 routes keep forwarding across the restart | `graceful-restart` config -> `prepare` -> Grace-LSA per interface (LS ID = Interface ID) + NVS persist (incl. Interface-ID/LSA-ID maps) -> graceful stop (no `RemoveAll`) -> subprocess restart -> in-restart suppression -> RTPROT_ZE IPv6 routes retained -> GR exit re-installs -> sweep refreshes | `ospf-v6-gr-fib-retention` (QEMU) |
| 2 | A Ze router is a helper for a restarting FRR `ospf6d` neighbour | FRR floods a Grace-LSA -> wire -> v3 codec -> link-store install -> engine helper dispatch -> §3.1 checks -> helping; Ze keeps the adjacency to FRR advertised; FRR completes GR without flapping | `ospf-v6-gr-frr` interop (Ze helper, FRR restarter) |
| 3 | An FRR `ospf6d` router is a helper for a restarting Ze neighbour | Ze originates Grace-LSAs, restarts (preserving Interface-IDs/LSA-IDs), re-acquires adjacencies; FRR holds the adjacency; Ze exits GR cleanly with no IPv6 route flap | `ospf-v6-gr-frr` interop (Ze restarter, FRR helper) |
| 4 | Runs `show ipv6 ospf graceful-restart` during a restart | CLI -> the GR state reporter -> restarter/helper state rendered | `test/ospf/ospf-v6-gr-show.ci` |
| 5 | Leaves OSPFv3 GR disabled (default) and restarts the router | no Grace-LSA, normal restart, IPv6 routes reconverge normally | `TestGRDisabledNoGraceLSA` + existing OSPFv3 suite green |
| 6 | Decodes a Grace-LSA hex capture | CLI decode -> v3 LSA decode -> LS Type 0x000B + LS ID = Interface ID + the two TLVs rendered | `test/ospf/ospf-v6-gr-decode.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGraceLSATypeRegistered` | `internal/plugins/ospf/v3/types/lsa_test.go` | AC-1: `LSTypeGrace` (0x000B) recognised by `Known()`, scope link-local | |
| `TestGraceLSARoundTrip` | `internal/plugins/ospf/v3/packet/lsa_grace_test.go` | A-1, AC-1: decode then re-encode a 0x000B LSA byte-for-byte; Fletcher checksum valid | |
| `TestGraceLSABodyBuild` | `internal/plugins/ospf/v3/packet/lsa_grace_test.go` | AC-2: body has type-1 Grace Period + type-2 Reason always; NO type-3 IP-address TLV | |
| `TestGraceLSATLVRoundTrip` | `internal/plugins/ospf/v3/packet/tlv_test.go` | A-4, R-8: the two TLVs encode/parse byte-for-byte with 4-octet padding (Reason Length 1, 4 padded octets) | |
| `TestGraceLSATLVIteratorMalformed` | `internal/plugins/ospf/v3/packet/tlv_test.go` | AC-23, R-8: truncated/over-length TLV never panics, reports an error | |
| `TestGraceLSALinkScopeRouting` | `internal/plugins/ospf/lsdb/lsdb_linkscope_test.go` | A-2, AC-1: a 0x000B LSA lands in the link store, never an area/AS store | |
| `TestGraceLSAOriginated` | `internal/plugins/ospf/gr_restarter_test.go` | A-3, AC-2: one Grace-LSA per OSPFv3 interface via `OriginateLinkSelf`, LS ID = Interface ID, LS age 0 | |
| `TestGraceLSAAgeNotResetOnRetransmit` | `internal/plugins/ospf/gr_restarter_test.go` | AC-3, R-7: retransmit keeps LS age | |
| `TestRestarterSuppressesV6SelfLSAs` | `internal/plugins/ospf/gr_restarter_test.go` | AC-5, A-6, R-2: no v3 self-LSA re-origination (all types) while in restart | |
| `TestRestarterRunsSPFNoInstall` | `internal/plugins/ospf/gr_restarter_test.go` | AC-6, A-7: SPF computed, `Installer.Apply` not called, FIB retained | |
| `TestInterfaceIDPreservedAcrossRestart` | `internal/plugins/ospf/gr_preserve_test.go` | AC-7, A-11, R-3: re-originated Link/Network/Router LSAs carry the same Interface IDs | |
| `TestLSAIDPrefixCorrespondencePreserved` | `internal/plugins/ospf/gr_preserve_test.go` | AC-8, A-12, R-4: each Inter-Area-Prefix / External prefix re-originated under the same LSA ID | |
| `TestRestarterReElectsSelfDR` | `internal/plugins/ospf/gr_restarter_test.go` | AC-9, A-14: re-elect self DR when a Waiting-state Hello lists self as DR | |
| `TestRestarterExitAllAdjacencies` / `TestRestarterExitInconsistentLSA` / `TestRestarterExitGraceExpiry` | `internal/plugins/ospf/gr_restarter_test.go` | AC-10/11/12: the three exit triggers | |
| `TestRestarterExitActions` | `internal/plugins/ospf/gr_restarter_test.go` | AC-13: re-originate self-LSAs, re-install routes, flush stale self-LSAs, flush own Grace-LSAs | |
| `TestRestartFactPersistsAcrossRestart` | `internal/plugins/ospf/gr_nvs_test.go` | AC-4, A-10: write, re-open, read grace-end + preserved maps via the ZeFS blob | |
| `TestStaleRestartFactIgnored` | `internal/plugins/ospf/gr_nvs_test.go` | AC-4, R-10: an expired restart-fact is ignored on resume | |
| `TestHelperEntryAllChecksPass` | `internal/plugins/ospf/gr_helper_test.go` | AC-14: enter helper when all §3.1 checks pass; identify X by Advertising Router | |
| `TestHelperEntryRejectedPerCheck` | `internal/plugins/ospf/gr_helper_test.go` | AC-15: each failing check blocks entry (table-driven) | |
| `TestHelperAlreadyHelpingUpdatesGrace` | `internal/plugins/ospf/gr_helper_test.go` | AC-16: re-receipt updates the grace period, no churn | |
| `TestHelperKeepsAdjacencyAdvertised` | `internal/plugins/ospf/gr_helper_test.go` | AC-14, A-8, R-5: v3 Router-LSA keeps X's link while helping | |
| `TestHelperKeepsXAsDR` | `internal/plugins/ospf/gr_helper_test.go` | AC-14, A-14: X stays DR while helping | |
| `TestHelperExitOnGraceExpiry` / `TestHelperExitOnFlush` / `TestHelperStrictExitOnTopologyChange` | `internal/plugins/ospf/gr_helper_test.go` | AC-17: the three exit triggers + DR recalc + Router-LSA re-origination | |
| `TestHelperStubAreaExternalDoesNotExit` | `internal/plugins/ospf/gr_helper_test.go` | AC-18, R-6, A-9: stub-area external change does not terminate helping | |
| `TestHelperRejectsGraceLSAMissingTLV` | `internal/plugins/ospf/gr_helper_test.go` | AC-23: a Grace-LSA missing a mandatory TLV is ignored, no crash | |
| `TestUnplannedDisabledByDefault` | `internal/plugins/ospf/gr_unplanned_test.go` | AC-19, R-11: no Grace-LSA on cold boot when unplanned support is off | |
| `TestUnplannedGraceBeforeHello` | `internal/plugins/ospf/gr_unplanned_test.go` | AC-20: Grace-LSAs before any Hello, reason 0 or 3 | |
| `TestGRDisabledNoGraceLSA` | `internal/plugins/ospf/gr_config_test.go` | AC-22, A-13: GR off -> no Grace-LSA, normal restart | |
| `TestGracePeriodRangeRejectsAbove1800` | `internal/plugins/ospf/config_test.go` | AC-21, R-12: YANG range 1-1800 enforced | |
| `TestGRShowState` | `internal/plugins/ospf/gr_show_test.go` | AC-24: restarter + helper state rendered | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Grace Period / RestartInterval (s) | 1-1800 | 1800 | 0 | 1801 |
| Grace Period TLV value length | 4 bytes (fixed) | 4 | a shorter type-1 TLV is malformed (ignore the Grace-LSA) | N/A |
| Restart Reason TLV value | 0-3 | 3 | N/A | a value >3 is treated as 0 (unknown) on receive |
| Restart Reason (unplanned, on send) | {0, 3} | 3 | N/A | reasons 1/2 are rejected for the unplanned path (§5) |
| Grace-LSA LS Type | 0x000B (fixed) | 0x000B | N/A | N/A |
| Grace-LSA Link State ID (Interface ID) | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A (0 reserved/inactive) | N/A (32-bit) |
| TLV padded value length | value + (0-3) pad | value+3 | N/A | a length past the LSA Length is an iterator error |
| Helper grace remaining (LS age vs Grace Period) | 0-1800 | 1800 | expired -> no entry | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-v6-gr-register` | `test/ospf/ospf-v6-gr-register.ci` | OSPFv3 GR enabled; the feature + `show ipv6 ospf graceful-restart` are present | |
| `ospf-v6-gr-prepare` | `test/ospf/ospf-v6-gr-prepare.ci` | a planned restart originates one Grace-LSA per OSPFv3 interface; the NVS restart-fact is written | |
| `ospf-v6-gr-helper` | `test/ospf/ospf-v6-gr-helper.ci` | a received Grace-LSA enters helper mode; the adjacency to X stays advertised; exit on grace expiry | |
| `ospf-v6-gr-show` | `test/ospf/ospf-v6-gr-show.ci` | `show ipv6 ospf graceful-restart` reports restarter + helper state | |
| `ospf-v6-gr-decode` | `test/ospf/ospf-v6-gr-decode.ci` | decode of a Grace-LSA hex shows LS Type 0x000B + LS ID = Interface ID + the two TLVs | |
| `ospf-v6-gr-disabled` | `test/ospf/ospf-v6-gr-disabled.ci` | GR off: no Grace-LSA, normal restart, IPv6 routes reconverge | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-v6-gr-frr` | `test/interop/scenarios/ospf-v6-gr-frr/` | FRR `ospf6d` (graceful-restart + helper) | Ze-helper holds the adjacency while FRR restarts (no flap); Ze-restarter is helped by FRR and exits GR cleanly; the v3 Grace-LSA (0x000B, 2 TLVs, LS ID = Interface ID) interops both directions | |
| `ospf-v6-gr-fib-retention` | `test/interop/scenarios/ospf-v6-gr-fib-retention/` | FRR `ospf6d` helper + an IPv6 traffic probe | across a Ze planned restart the RTPROT_ZE kernel IPv6 routes stay programmed and forwarding continues (non-stop forwarding); routes are refreshed (not swept) on GR exit; Interface IDs / LSA-IDs preserved (no churn) | |

> Interop is required: this changes wire behaviour (Grace-LSA origination + helper
> reaction) and forwarding behaviour (FIB retention). The raw-IPv6 / multicast
> flood and the real kernel-route retention are Linux-only and run as QEMU
> integration tests (`ai/rules/qemu-testing.md`), consistent with the rest of the
> OSPFv3 interop set (`ospf-v6-frr`, `ospf-v6-broadcast-frr`, ...). The peer is FRR
> `ospf6d` (the OSPFv3 daemon), not `ospfd`.

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/v3/types/lsa.go` -- add `LSTypeGrace LSType = 0x000B` (function code 11, link-local) + include it in `Known()`
- `internal/plugins/ospf/lsdb/link_scope.go` -- broaden the link-scope predicate (`isLinkLSAType` or a sibling) so function code 11 routes through `installLink` / `OriginateLinkSelf` / `LinkLSAs`
- `internal/plugins/ospf/origination_v6.go` -- the GR origination glue (`v6OriginateGraceLSAs` reusing `v6OriginateLinkLSA`'s `OriginateLinkSelf` + `v6OriginHeader` pattern); gate `v6OriginateSelf` behind the in-restart flag
- `internal/plugins/ospf/instance.go` -- the in-restart flag + the helper map on the engine; gate `originateSelfLSAs` (v6 path); the pre-restart / exit orchestration; the Grace-LSA reception dispatch from `handleLSUpdate`/`ReceiveUpdate`; distinguish graceful stop from `shutdown()`; the restart-fact load on resume
- `internal/plugins/ospf/spf/install.go` -- a "suppress install while in restart" gate around `Apply` (skip insert/remove) and a guard so a graceful stop does not call `RemoveAll`
- `internal/plugins/ospf/lsdb/origination.go` / `internal/plugins/ospf/lsdb/flooding.go` -- skip the restarter's own pre-restart self-LSA flush while in restart; surface the post-install content-change signal to the helper strict-checking exit and the restarter inconsistent-LSA exit
- `internal/plugins/ospf/origination_v6.go` `v6OriginateRouter` -- while helping X, keep X's link advertised (and the Network-LSA if DR) regardless of NSM state; keep X as DR
- `internal/plugins/ospf/neighbor/neighbor.go` -- a per-neighbour helping flag on the topology snapshot (`FloodNeighbor` / `NeighborInfo`); record X's DR role at helper entry
- `internal/plugins/ospf/register.go` -- register the `graceful-restart` config resolution (v6-gated), the `show ipv6 ospf graceful-restart` command, the GR doctor check, and the GR metrics
- `internal/plugins/ospf/config.go` -- resolve the `graceful-restart` config (restarter support/interval/unplanned, helper support/strict-checking) into `ospfConfig`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `graceful-restart` container (mirrors the `max-metric` precedent; family-neutral)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `show ipv6 ospf graceful-restart` command
- `internal/plugins/ospf/cmd_show.go` -- the `show ipv6 ospf graceful-restart` handler
- `internal/plugins/ospf/doctor.go` -- a doctor check for the GR NVS blob path / unplanned-support sanity (the NVS path is the new runtime dependency)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `graceful-restart` container; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `restart-interval` `range "1..1800"`; `support` enumeration {disabled, planned, planned-and-unplanned}; `helper` `strict-lsa-checking` boolean |
| YANG custom validators | [ ] no | native range + enumeration + boolean suffice |
| CLI commands/flags | [ ] yes | `show ipv6 ospf graceful-restart` in `ze-ospf-cmd.yang` + `cmd_show.go`; an operator `ospf graceful-restart prepare` action (managed-reload hook) |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ipv6 ospf graceful-restart` |
| Editor autocomplete | [ ] yes | automatic for the YANG enumeration/boolean leaves + the new show subcommand |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-v6-gr-*.ci` |
| Pipe completeness | [ ] yes | `show ipv6 ospf graceful-restart` routes through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | GR is operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] yes | the GR NVS blob path is a new file dependency -> a doctor check + `internal/core/diagnostic/codes.go` code + unit + functional test (see `ai/rules/doctor-checks.md`) |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospfv3_gr_restarter_active` | gauge | (none; 1 while in graceful restart) |
| `ze_ospfv3_gr_restarter_exits_total` | counter | `reason` (adjacencies / inconsistent-lsa / grace-expiry) |
| `ze_ospfv3_gr_helper_sessions` | gauge | `interface` |
| `ze_ospfv3_gr_helper_exits_total` | counter | `reason` (flushed / grace-expiry / topology-change) |
| `ze_ospfv3_gr_grace_lsas` | gauge | `direction` (originated / received) |

> These use the `ze_ospfv3_gr_*` prefix (per the umbrella's `ze_ospfv3_<ext>_*`
> naming) and are registered by this spec's owner code. The OSPFv3 umbrella
> "Metrics" mapping must gain these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPFv3 Graceful Restart (restarter + helper) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `graceful-restart` container |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ipv6 ospf graceful-restart` |
| 4 | API/RPC added/changed? | [ ] yes | `docs/architecture/api/commands.md` -- the `prepare` action / managed-reload hook |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPFv3 gains a Grace-LSA type + GR state machines |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- an OSPFv3 Graceful Restart section |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` -- the v3 Grace-LSA (LS Type 0x000B, LS ID = Interface ID) + its two TLVs |
| 8 | Plugin SDK/protocol changed? | [ ] no | no new SDK surface; the Grace-LSA is a native v3 LSA |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5187.md` -- flip the Compliance Checklist items to implemented |
| 10 | Test infrastructure changed? | [ ] yes (two interop scenarios) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPFv3 GR parity with FRR `ospf6d` (restarter + helper) |
| 12 | Internal architecture changed? | [ ] yes | the OSPFv3 subsystem doc -- GR state machines + FIB-retention coupling + the two preservation rules |
| 13 | Route metadata keys added/changed? | [ ] no | GR does not add route metadata keys |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPFv3 telemetry doc -- the five `ze_ospfv3_gr_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the OSPFv3 umbrella metrics table |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF / v3 / fib-kernel files |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPFv3 config/CLI examples against the new `graceful-restart` container |

## Files to Create
- `internal/plugins/ospf/v3/packet/lsa_grace.go` -- the typed `GraceLSA` body (the two TLVs), `DecodeGrace()`, `EncodedLen`, `WriteTo` (buffer-first, 4-octet aligned)
- `internal/plugins/ospf/v3/packet/tlv.go` -- the v3 4-octet-aligned TLV iterator + builder (RFC 3630 §2.3.2 format)
- `internal/plugins/ospf/gr.go` -- the GR feature glue: the engine-side orchestration (pre-restart, in-restart gating, exit), the helper map, the Grace-LSA origination/reception dispatch (v6 path)
- `internal/plugins/ospf/gr_restarter.go` -- the restarter state machine (pre-restart, in-restart suppression, §3.1/§3.2 preservation restore, the three exit triggers, exit actions)
- `internal/plugins/ospf/gr_helper.go` -- the helper state machine (§3.1 entry checks, while-helping advertisement, §3.2 exit incl. strict checking + stub-area exception)
- `internal/plugins/ospf/gr_nvs.go` -- the restart-fact ZeFS blob (write on prepare, read/validate on resume; carries grace-end + the §3.2 Interface-ID map + the §3.1 prefix->LSA-ID map), reusing the `openBootCountStore` seam
- `internal/plugins/ospf/gr_show.go` -- the `show ipv6 ospf graceful-restart` state reporter
- `internal/plugins/ospf/v3/packet/lsa_grace_test.go`, `internal/plugins/ospf/v3/packet/tlv_test.go`, `internal/plugins/ospf/v3/types/lsa_test.go` (Grace cases)
- `internal/plugins/ospf/gr_restarter_test.go`, `gr_helper_test.go`, `gr_nvs_test.go`, `gr_preserve_test.go`, `gr_unplanned_test.go`, `gr_config_test.go`, `gr_show_test.go`
- `test/ospf/ospf-v6-gr-register.ci`, `ospf-v6-gr-prepare.ci`, `ospf-v6-gr-helper.ci`, `ospf-v6-gr-show.ci`, `ospf-v6-gr-decode.ci`, `ospf-v6-gr-disabled.ci`
- `test/interop/scenarios/ospf-v6-gr-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospf-v6-gr-fib-retention/` -- `ze.conf`, `frr.conf`, `check.py` (IPv6 traffic probe across the restart)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the v3 codec verbatim passthrough + the link store + the fib-kernel `RTPROT_ZE`/`sweepDelay` retention exist |
| 3. Wiring phase | Wiring Test table -- the Grace-LSA type + GR feature wiring + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- the Grace-LSA type + GR feature wiring + failing wiring tests
   - Tests: `TestGraceLSATypeRegistered`, `TestGraceLSALinkScopeRouting`, `test/ospf/ospf-v6-gr-register.ci`
   - Files: `v3/types/lsa.go` (`LSTypeGrace` + `Known()`), `lsdb/link_scope.go` (broaden the link predicate), `gr.go` (the GR feature skeleton wired into the v6 engine with stub origination/reception/state machines), `register.go` (the `graceful-restart` config skeleton + the show command stub)
   - Verify: the Grace-LSA type is recognised and routes to the link store; the GR feature is reachable from the v6 engine; origination/reception/state machines are stubs so the deeper tests still fail
2. **Phase: Grace-LSA wire (type + body + TLV codec)** -- the v3 wire half
   - Tests: `TestGraceLSARoundTrip`, `TestGraceLSABodyBuild`, `TestGraceLSATLVRoundTrip`, `TestGraceLSATLVIteratorMalformed`, `ospf-v6-gr-decode.ci`
   - Files: `v3/packet/lsa_grace.go`, `v3/packet/tlv.go`, `v3/packet/lsa.go` (the `Grace *GraceLSA` field + `DecodeGrace()`)
   - Verify: the body round-trips with 4-octet padding (Reason Length 1 -> 4 octets); the iterator never panics; decode renders LS Type 0x000B + LS ID = Interface ID + the two TLVs
3. **Phase: Helper state machine** -- receive-side first (lower risk, per the guide ordering)
   - Tests: `TestHelperEntryAllChecksPass`, `TestHelperEntryRejectedPerCheck`, `TestHelperAlreadyHelpingUpdatesGrace`, `TestHelperKeepsAdjacencyAdvertised`, `TestHelperKeepsXAsDR`, `TestHelperExitOnGraceExpiry`, `TestHelperExitOnFlush`, `TestHelperStrictExitOnTopologyChange`, `TestHelperStubAreaExternalDoesNotExit`, `TestHelperRejectsGraceLSAMissingTLV`, `ospf-v6-gr-helper.ci`
   - Files: `gr_helper.go`, `gr.go` (helper map + reception dispatch), `instance.go`/`neighbor.go` (helping flag), `origination_v6.go` `v6OriginateRouter` (keep X's link), `lsdb/flooding.go` (content-change signal)
   - Verify: entry/exit checks per §3.1/§3.2; X identified by Advertising Router; the adjacency to X stays advertised; the stub-area exception holds; a malformed Grace-LSA is ignored
4. **Phase: Restarter NVS + preservation + in-restart gating** -- the suppression + §3.1/§3.2 core
   - Tests: `TestRestartFactPersistsAcrossRestart`, `TestStaleRestartFactIgnored`, `TestRestarterSuppressesV6SelfLSAs`, `TestRestarterRunsSPFNoInstall`, `TestInterfaceIDPreservedAcrossRestart`, `TestLSAIDPrefixCorrespondencePreserved`
   - Files: `gr_nvs.go`, `gr_restarter.go`, `gr_preserve` logic, `instance.go` (gate `originateSelfLSAs`), `origination_v6.go` (gate `v6OriginateSelf`; restore Interface-IDs/LSA-IDs), `spf/install.go` (gate `Apply`, guard `RemoveAll`)
   - Verify: the restart-fact + preserved maps survive a restart; in-restart suppresses origination + install; SPF still computes; FIB not touched; Interface IDs and LSA-IDs preserved
5. **Phase: Restarter origination + DR re-election + exit triggers + exit actions**
   - Tests: `TestGraceLSAOriginated`, `TestGraceLSAAgeNotResetOnRetransmit`, `TestRestarterReElectsSelfDR`, `TestRestarterExitAllAdjacencies`, `TestRestarterExitInconsistentLSA`, `TestRestarterExitGraceExpiry`, `TestRestarterExitActions`, `ospf-v6-gr-prepare.ci`
   - Files: `gr_restarter.go`, `gr.go` (`v6OriginateGraceLSAs` + the `prepare` orchestration + graceful stop), `origination_v6.go` (`FlushStaleSelfLSAs`/`FlushStaleLinkSelfLSAs` on exit)
   - Verify: one Grace-LSA per interface (LS age 0, LS ID = Interface ID), retransmit keeps LS age; DR continuity; the three exit triggers; the exit re-originates LSAs, re-installs routes, flushes stale self-LSAs + own Grace-LSAs
6. **Phase: Unplanned outage (config-gated)** -- §5
   - Tests: `TestUnplannedDisabledByDefault`, `TestUnplannedGraceBeforeHello`, `TestGRDisabledNoGraceLSA`
   - Files: `gr_restarter.go`, `config.go`
   - Verify: unplanned off by default; when on, Grace-LSAs before Hello, reason 0/3 only
7. **Phase: Config + CLI + metrics + doctor** -- user surface
   - Tests: `TestGracePeriodRangeRejectsAbove1800`, `TestGRShowState`, `ospf-v6-gr-register.ci`, `ospf-v6-gr-show.ci`, `ospf-v6-gr-disabled.ci`
   - Files: `yang/ze-ospf-conf.yang`, `yang/ze-ospf-cmd.yang`, `config.go`, `gr_show.go`, `cmd_show.go`, `doctor.go`, metric registration in `register.go`
   - Verify: the `graceful-restart` container, the show command, the five metric series, the NVS-path doctor check
8. **Functional tests** -> the six `.ci` cover the user-visible behaviour
9. **RFC refs** -> add `// RFC 5187 Section X` comments on the Grace-LSA type/body, the §2.2 LS-ID = Interface ID, the §3.1 LSA-ID preservation, the §3.2 Interface-ID preservation, and `// RFC 3623 Section X` on the inherited in-restart suppression, the §2.2 exit triggers, the §3.1 entry checks, the §3.2 exit, and the §5 unplanned path
10. **Interop** -> `ospf-v6-gr-frr` + `ospf-v6-gr-fib-retention` QEMU scenarios (FRR `ospf6d`)
11. **Full verification** -> `make ze-verify`
12. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; GR parity with FRR `ospf6d` (restarter + helper, planned + helper-strict-checking); FIB retention + the two preservation rules proven end-to-end |
| Correctness | the v3 Grace-LSA is a NATIVE link LSA (0x000B), LS ID = Interface ID, two TLVs (no IP-address TLV), 4-octet padding (Reason Length 1 -> 4 octets); LS age = grace clock (never reset); §3.1 entry checks all enforced; §3.2 exit incl. stub-area exception; restarter suppresses ALL v3 self-LSA types; Interface IDs + LSA-IDs preserved across restart; `RemoveAll` not called on graceful stop; grace window vs `sweepDelay` reconciled |
| Naming | `ze_ospfv3_gr_*` metrics; YANG `graceful-restart` / `restart-interval` kebab-case; `show ipv6 ospf graceful-restart`; `LSTypeGrace` |
| Data flow | the Grace-LSA flows v3 codec -> link store -> helper dispatch; fib-kernel read-only; the in-restart flag gates origination + install only; no shared v2/v3 wire code |
| CLI grammar | `show ipv6 ospf graceful-restart` action-before-identifier |
| Doctor checks | the GR NVS blob path has a `ze doctor` check per `ai/rules/doctor-checks.md` |
| YANG validation | `restart-interval` range 1-1800; `support`/`helper` enumerations/booleans; no bare `type string` |
| Prometheus counters | the five `ze_ospfv3_gr_*` series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | removing GR removes the `LSTypeGrace` registration, config, command, doctor, metrics; no GR spelling in the OSPFv2 plugin or generic OSPF packages |
| Rule: buffer-first | the Grace-LSA body is built via the v3 TLV builder into a caller buffer; parse is a zero-copy iterator |
| Rule: no shared v2/v3 wire | the Grace-LSA codec lives entirely under `internal/plugins/ospf/v3/`; no v2 GR code referenced |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Grace-LSA type recognised | `grep -rn 'LSTypeGrace' internal/plugins/ospf/v3/types` |
| Grace-LSA body build/parse | `go test ./internal/plugins/ospf/v3/packet -run 'Grace'` |
| Link-scope routing for 0x000B | `go test ./internal/plugins/ospf/lsdb -run 'GraceLSALinkScope'` |
| Restarter suppression + exit | `go test ./internal/plugins/ospf -run 'Restarter'` |
| Preservation (Interface-ID / LSA-ID) | `go test ./internal/plugins/ospf -run 'Preserved'` |
| Helper entry/exit | `go test ./internal/plugins/ospf -run 'Helper'` |
| NVS restart-fact | `go test ./internal/plugins/ospf -run 'RestartFact'` |
| Five metric series | `grep -rn 'ze_ospfv3_gr_' internal/plugins/ospf` |
| FIB-retention interop | `ls test/interop/scenarios/ospf-v6-gr-fib-retention/` |
| GR interop | `ls test/interop/scenarios/ospf-v6-gr-frr/` |
| Functional tests | `ls test/ospf/ospf-v6-gr-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the Grace-LSA TLV parse is bound-checked (the v3 iterator never panics); a missing/short mandatory TLV makes the Grace-LSA malformed and ignored, not a crash; the existing v3 LSA fuzz target is extended with Grace-LSA bodies |
| Spoofed Grace-LSA | a forged Grace-LSA can hold a withdrawn router adjacent (R-9); the helper still requires a prior Full adjacency with X; Grace-LSAs ride the delivered OSPFv3 RFC 7166 auth trailer; document the residual risk (RFC 5187 §4: manual keying precludes replay protection) |
| Resource exhaustion | the helper map is bounded by the neighbour count; the link store shares the existing `MaxLSAsPerArea` cap; a flood of Grace-LSAs cannot grow memory unbounded; the `MinLSArrival` rate limit applies |
| FIB safety | unplanned support is config-gated (default off) because a crashed router cannot guarantee FIB sanity (§5, R-11); the planned path ensures the FIB is current before reload |
| Preservation integrity | the persisted Interface-ID / LSA-ID maps are read back and validated on resume; a corrupt/missing map ignores the restart-fact and boots normally (no silent mismatch) |
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
| Shared v2/v3 wire code introduced | STOP; RFC 5340 forbids it; keep the Grace-LSA codec under `internal/plugins/ospf/v3/` |
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
OSPFv3 Graceful Restart is the OSPFv2 procedure (two control-plane state
machines on an EXISTING FIB-retention substrate) with a different wire half: the
Grace-LSA is a NATIVE link-scope LSA (function code 11, LS Type 0x000B) rather
than an opaque LSA, its Link State ID is the Interface ID rather than an opaque
type/ID, it drops the router-address TLV (OSPFv3 keys neighbours by Router ID),
and it adds two preservation requirements (the Interface ID and the arbitrary
32-bit LSA-IDs) that keep the inherited RFC 3623 FSM from terminating early. The
hard part is the disciplined suppression and the two preservation rules, not the
wire; the wire reuses the delivered v3 link-LSA origination and verbatim
passthrough.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Grace-LSA as a native v3 LS type (0x000B) routed through the link store | a v3 opaque carrier mirroring v2 ext-1 | RFC 5340 has no opaque LSAs; the umbrella forbids inventing one; the Grace-LSA is a first-class link-scope LSA, so it reuses the delivered Link-LSA store + origination |
| Link State ID = Interface ID, two TLVs, no router-address TLV | copy the v2 three-TLV / opaque-LS-ID body | RFC 5187 §2/§2.2 explicitly: LS ID = Interface ID, the IP-address TLV is unnecessary because v3 keys neighbours by Router ID |
| Reuse the shared in-restart gate (`v6OriginateSelf` + `Installer.Apply`) and the fib-kernel retention | a v3-specific restart path | the engine, LSDB install/flood, SPF install seam, and fib-kernel are family-shared; only the v3 origination + wire differ; mirror the v2 GR control plane |
| Persist Interface-ID + prefix->LSA-ID maps in the restart-fact NVS | re-derive them on resume | RFC 5187 §3.1/§3.2 require STABLE values across restart; re-deriving risks a renumber that silently terminates the restart |
| Helper identifies X by the Advertising Router | carry/parse an interface-address TLV like v2 | RFC 5187 §2: OSPFv3 keys neighbours by Router ID; no interface-address TLV exists in the v3 Grace-LSA |
| No shared v2/v3 GR wire code | a unified Grace-LSA codec | RFC 5340 mandate (umbrella); the v2 Grace-LSA is opaque-carried with a different LS-ID and three TLVs; sharing would leak version branches |

## Known Limitations
- The Grace-LSA is the only new v3 wire object; SR / RI / extended LSAs (ospfv3-ext-6) are not preserved by name here, but any arbitrary-32-bit-LSA-ID LSA they add follows the same §3.1 preservation rule, validated when ext-6 lands.
- OSPFv3 BFD-coordinated GR is out of scope (ospfv3-ext-5); a BFD-down during restart degrades to a normal restart.
- The §3.2 Interface-ID stability depends on a stable interface-id source across reboots (RFC 5187 §3.2 / Errata 1453 reference RFC 2863 §3.1.5 IfIndex persistence); Ze persists the assignments in the restart-fact NVS rather than relying on the OS.
- OSPFv2 GR (RFC 3623, opaque-carried) is a separate spec (`spec-ospf-ext-9-graceful-restart.md`); no wire code is shared (RFC 5340 mandate).

## RFC Documentation

Add `// RFC 5187 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §2.1 the Grace-LSA LS Type 0x000B / function code 11 / link-local scope bits
- §2.2 the Link State ID = Interface ID and the two mandatory TLVs (Grace Period, Restart Reason)
- §2 the dropped router-address TLV (helper keys X by Advertising Router)
- §3.1 the LSA-ID->prefix correspondence preservation
- §3.2 the Interface ID preservation

Add `// RFC 3623 Section X.Y: "<quoted requirement>"` above the inherited
control-plane code:
- §2 in-restart self-LSA suppression + run-SPF-no-install + do-not-flush-received-self-LSAs
- §2.1 ensure-FIB-current + grace period not exceeding LSRefreshTime
- §2.2 the three exit triggers; §2.3 the exit actions
- §3.1 the helper entry checks; §3.2 the helper exit + the stub-area exception
- §5 the config-gated unplanned-outage path

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
| Non-stop forwarding across a planned OSPFv3 restart | interop (QEMU) | `ospf-v6-gr-fib-retention` (RTPROT_ZE IPv6 routes retained, refreshed on exit) |
| Native v3 Grace-LSA (0x000B, LS ID = Interface ID, two TLVs) interops with FRR | interop | `ospf-v6-gr-frr` (both directions) |
| Restarter suppresses all v3 self-LSA origination + route install during restart | unit | `TestRestarterSuppressesV6SelfLSAs`, `TestRestarterRunsSPFNoInstall` |
| Helper holds the adjacency at Full for the grace window | unit + functional | `TestHelperKeepsAdjacencyAdvertised`, `ospf-v6-gr-helper.ci` |
| §3.2 Interface ID preserved across restart | unit | `TestInterfaceIDPreservedAcrossRestart` |
| §3.1 LSA-ID->prefix correspondence preserved | unit | `TestLSAIDPrefixCorrespondencePreserved` |

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
- [ ] AC-1..AC-24 all demonstrated
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
- [ ] RFC 5187 + RFC 3623 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the v3 TLV codec serves the Grace-LSA now; reuse if ospfv3-ext-6 needs it)
- [ ] No speculative features (only GR; no SR/RI bodies)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (no GR spelling in the OSPFv2 plugin or generic packages)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-v6-gr-frr` + `ospf-v6-gr-fib-retention`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospfv3-ext-4-graceful-restart.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospfv3-ext-4-graceful-restart.md`
