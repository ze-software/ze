# Spec: ospf-ext-9 -- OSPFv2 Graceful Restart restarter + helper (RFC 3623)

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
3. `rfc/short/rfc3623.md` -- Graceful OSPF Restart: grace-LSA wire format (§A), restarting-router Tx/state/exit (§2, §2.1, §2.2, §2.3, §5), helper Rx/state/exit (§3, §3.1, §3.2), timers/constants, the two state-machine tables (restarter, helper), the unplanned-outage rules (§5)
4. `rfc/short/rfc5250.md` -- Opaque-LSA framework: Type 9 link-local scope rule (§3.1, "received on an interface other than the target interface MUST be discarded and not acknowledged"), the Opaque Type / Opaque ID split (§3 / App A.2), the O-bit DD gate (§3.1)
5. `plan/spec-ospf-ext-1-opaque-framework.md` -- the opaque carrier this consumer plugs into: `RegisterOpaqueConsumer(opaqueType, scope, OnOriginate, OnReceive)`, the link-store (Type 9) origination/flood path, the 4-byte-aligned TLV iterator + builder, the O-bit DD negotiation, and the verbatim re-flood guarantee
6. `internal/plugins/ospf/instance.go` -- the engine: `originateSelfLSAs` (the self-LSA origination chokepoint the restarter must suppress), `shutdown()`, the boot-count NVS seam (`loadOSPFBootCount`/`openBootCountStore`), `runOSPFEngine` lifecycle
7. `internal/plugins/ospf/spf/install.go` -- `Installer.Apply`/`insert`/`remove`/`RemoveAll`: OSPF inserts `locrib.Path` into the shared Loc-RIB; the restarter must NOT install/remove routes during restart (FIB retention)
8. `internal/plugins/fib/kernel/backend.go` + `backend_linux.go` -- `RTPROT_ZE` (250) marking + `sweepDelay` startup stale-mark-then-sweep: the existing mechanism that keeps kernel routes alive across a control-plane restart, which the restarter relies on for non-stop forwarding
9. `internal/plugins/ospf/neighbor/neighbor.go` + `nsm.go` + `table.go` -- the neighbor record, the NSM, and the table the helper freezes at Full; `FloodNeighbor`/`Snapshot`
10. `internal/plugins/ospf/lsdb/origination.go` -- `OriginateLinkSelf`/`OriginateSelf`/`SelfLSAEncoder` (grace-LSA origination reuses the link-scope self-origination), `flushReceivedSelfLSA`/`handleSelfReceived` (the "do not flush received self-LSAs during restart" requirement, §2)
11. `internal/plugins/ospf/auth_keystore.go` -- the ZeFS (`pkg/zefs`) NVS persistence pattern (`loadOSPFBootCount` reads/increments/writes a blob) that the restart-fact NVS storage mirrors (§2.1)

## Task

Add **OSPFv2 Graceful Restart (RFC 3623)** to the native OSPFv2 plugin at
`internal/plugins/ospf/`, implementing **both roles**: the **restarting router**
(restarter) and the **helper neighbor**. Graceful Restart lets a Ze router
restart or reload its OSPF control software while staying on the forwarding path
("non-stop forwarding"): the restarting router floods link-local **Grace-LSAs**
asking neighbours to keep advertising it as fully adjacent for a bounded grace
period, preserves its FIB across the control-plane restart, and re-acquires
adjacencies and re-syncs the LSDB without flapping routes; neighbours that
receive a Grace-LSA enter **helper mode**, hold the adjacency at Full and
suppress LSDB churn until the grace period ends or a topology change forces an
early exit.

The only new wire object is the Grace-LSA, which is a **link-local Type 9 Opaque
LSA, Opaque Type 3, Opaque ID 0** (RFC 3623 §A), carrying a TLV body. This spec
is therefore a **consumer of the ext-1 opaque carrier**: it claims Opaque Type 3
through `RegisterOpaqueConsumer(scope=link)`, originates Grace-LSAs through the
carrier's link-store self-origination, and receives them through the carrier's
`OnReceive` hook. It does NOT re-implement opaque flooding, the LS-ID split, the
O-bit DD gate, or the TLV iterator/builder; those are ext-1 (this spec's
dependency). It defines only the Grace-LSA body (three TLVs) and the two
control-plane state machines layered on top.

The Grace-LSA body carries three TLVs (RFC 3623 §A): **type 1 Grace Period**
(4 bytes, always present), **type 2 Graceful Restart Reason** (1 byte, always
present), and **type 3 IP Interface Address** (4 bytes, required on
broadcast/NBMA/Point-to-MultiPoint segments). The grace period is measured by the
Grace-LSA's LS age (which MUST start at 0 and MUST NOT be reset on retransmit,
and DoNotAge MUST NOT be set), so the helper's expiry timer reads LS age, not a
separate clock.

**Restarter behaviour (§2, §2.1, §2.2, §2.3, §5):** on an operator-triggered
(planned) restart, before reload the router ensures its FIB is current and will
persist across the control-plane restart, originates one Grace-LSA per OSPF
interface (LS age 0, requested grace period, reason), reliably floods them
(retransmit until acked), and records the restart fact + grace-period end in NVS.
After the software resumes, while in graceful restart the router does NOT
originate LSA types 1-5/7 (it relies on its pre-restart LSAs), does NOT modify or
flush received self-originated LSAs, runs SPF (RFC 2328 §16) to restore virtual
links but does NOT install OSPF routes (it relies on the pre-restart FIB
preserved by the fib-kernel `RTPROT_ZE` stale-mark-then-sweep), and re-elects
itself DR on a segment if it was DR before the restart. It exits graceful restart
when any of: all adjacencies are re-established, an LSA inconsistent with the
pre-restart Router-LSA is received, or the grace period expires; on exit it
re-originates its Router-LSAs (all areas) and Network-LSAs (segments where DR),
re-runs SPF and installs routes, removes stale pre-restart FIB entries, flushes
stale received self-LSAs, and flushes its own Grace-LSAs.

**Helper behaviour (§3, §3.1, §3.2):** on receiving a Grace-LSA from neighbour X
on a segment, the router enters helper mode for X **only if all checks pass** --
Full adjacency with X, no content change in the LSDB (types 1-5/7) since X
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

Both roles, the grace timers, and the FIB-retention coordination with the
fib-kernel sweep are in scope. OSPFv3 Graceful Restart (RFC 5187, carried as a
native LSA, not opaque) is OUT OF SCOPE and belongs to ospfv3-ext-4.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Grace-LSA body | The three TLVs (Grace Period / Reason / IP Interface Address) built and parsed via the ext-1 TLV builder/iterator; Opaque Type 3, Opaque ID 0, scope link (Type 9) |
| Opaque-consumer registration | `RegisterOpaqueConsumer(opaqueType=3, scope=link, OnOriginate, OnReceive)` from this plugin's `init()`; origination + reception driven by ext-1 |
| Restarter: pre-restart | Ensure FIB current/persistent; originate one Grace-LSA per interface (LS age 0); reliably flood; persist {restarting, grace-end, reason} to NVS (ZeFS) (§2.1) |
| Restarter: in-restart suppression | Suppress self-LSA origination (types 1-5/7) and route install while the grace window is open; run SPF without installing; keep pre-restart received self-LSAs (§2) |
| Restarter: DR re-election | Re-elect self DR on a segment if a Hello in Waiting state lists self as DR (was DR before restart) (§2) |
| Restarter: exit | Exit on all-adjacencies-up / inconsistent-LSA / grace-expiry; re-originate Router/Network-LSAs, install routes, flush stale self-LSAs, flush own Grace-LSAs (§2.2, §2.3) |
| Restarter: unplanned outage | Config-gated: on cold/unplanned start, send Grace-LSAs before any Hello, reason restricted to 0 or 3, operator can disable (§5) |
| Helper: entry checks | Full adjacency, LSDB unchanged since restart, grace period not expired, policy permits, helper not restarting (§3.1) |
| Helper: while-helping | Continue advertising adjacency to X (Router-LSA / Network-LSA), keep X as DR, suppress LSDB churn for the grace window (§3) |
| Helper: strict LSA checking | Default-on: a changed LSA that would flood to X terminates helping; config-gated relaxation (§3.2) |
| Helper: exit | Grace-LSA flushed / grace expiry / topology change -> DR recalc + Router/Network-LSA re-origination (§3.2) |
| FIB retention coordination | The restarter relies on the existing fib-kernel `RTPROT_ZE` stale-mark-then-sweep; this spec ensures the grace window closes (routes re-installed) before the sweep deadline, and the `RemoveAll` on engine stop is NOT invoked on a graceful restart (§2.1) |
| Grace timers | Grace Period (1-1800 s, suggested default 120 s) measured by Grace-LSA LS age; helper expiry timer; restarter exit timer (§2.1, §A, §B.1) |
| Config + CLI + metrics | `graceful-restart` config (restarter support/interval, helper support/strict-checking), `show ip ospf graceful-restart`, Prometheus series |

### Out of scope (noted so it is not silently assumed done)

| Item | Where |
|------|-------|
| Opaque carrier (link-store flooding, LS-ID split, O-bit DD gate, TLV iterator/builder) | spec-ospf-ext-1 (this spec's dependency) |
| OSPFv3 Graceful Restart (RFC 5187; native Grace-LSA, not opaque) | ospfv3-ext-4 |
| TE/RI/SR opaque consumers | spec-ospf-ext-2 / ext-3 / ext-4 |
| New FIB backend or kernel-route mechanism | fib-kernel already provides `RTPROT_ZE` + `sweepDelay`; this spec consumes it, does not extend it |
| Cross-vendor "GR + BFD" interaction | BFD is not yet in the OSPF plugin; a BFD-down during restart degrades to normal restart (no special handling) |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/research/ospf-implementation-guide.md` §"Graceful Restart and Helper (RFC 3623)" (~1538-1541) -- the FRR landscape: grace-LSAs (Opaque Type 3) announce the restart window; helpers suppress LSDB churn and adjacency tear-downs; helper mode is strictly receive-side
  -> Decision: implement BOTH roles in one spec, but build helper first (receive-side, lower risk) then the restarter on top, matching the guide's "support helper mode first, defer the restarter" ordering as the phase order, not as a scope cut
  -> Constraint: Grace-LSA is Opaque Type 3 carried by the ext-1 carrier; this spec adds NO new flooding, only the body + the two state machines
- [ ] `plan/spec-ospf-ext-1-opaque-framework.md` "In scope" + "Data Flow" -- the carrier API this consumer uses
  -> Constraint: register with `RegisterOpaqueConsumer(opaqueType=3, scope=link, OnOriginate, OnReceive)`; origination returns `(opaqueID=0, scope=link, body, withdraw)` and the carrier assigns sequence/age, installs into the link store, and floods only to opaque-capable neighbours; reception delivers `OnReceive(opaqueID, body, scope, advRouter, reachable)` after a Newer install
  -> Constraint: the Grace-LSA body is built with the ext-1 4-byte-aligned TLV builder and parsed with the ext-1 TLV iterator; this spec interprets TLV types 1/2/3 and ignores unrecognised types (§A)
- [ ] `ai/rules/plugin-self-containment.md` -- the GR consumer must be self-contained
  -> Constraint: removing this consumer removes Opaque Type 3 registration, the `graceful-restart` config, the `show ip ospf graceful-restart` command, the doctor check, and all GR metrics; no GR spelling appears in the ext-1 carrier or in generic OSPF packages
- [ ] `ai/rules/buffer-first.md` -- Grace-LSA body encode is buffer-first
  -> Constraint: the three TLVs are emitted through the ext-1 TLV builder into a caller-owned buffer (`WriteTo(buf, off) int`); no `+`/`fmt` string building of the body
- [ ] `ai/rules/qemu-testing.md` -- GR is Linux-only (raw IP / multicast flood, real FIB retention)
  -> Constraint: the FIB-retention and interop validation run as QEMU integration tests; "needs hardware / needs a real restart" is not a reason to skip

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc3623.md` -- Graceful OSPF Restart (the feature spec)
  -> Constraint: §A -- Grace-LSA is LS type 9, Opaque Type 3, Opaque ID 0; body is TLV-encoded (RFC 3630 TLV format); LS age MUST be 0 at first origination, MUST NOT be reset on retransmit, DoNotAge MUST NOT be set (LS age is the grace clock)
  -> Constraint: §A -- type 1 Grace Period (4 bytes) and type 2 Restart Reason (1 byte) MUST always be present; type 3 IP Interface Address (4 bytes) is required on broadcast/NBMA/P2MP segments; unrecognised TLV types are ignored
  -> Constraint: §2 -- during restart the restarter MUST NOT originate LSA types 1-5/7, MUST NOT modify/flush received self-originated LSAs, runs SPF without installing routes (relies on the pre-restart FIB), and re-elects self DR if it was DR (Hello in Waiting state lists self as DR)
  -> Constraint: §2.1 -- before reload, ensure forwarding table(s) are current and remain in place across the restart; grace period SHOULD NOT exceed LSRefreshTime (1800 s) or the router's own LSAs age out
  -> Constraint: §2.2 -- exit on any of: all adjacencies re-established (pre-restart Router/Network-LSA reflected by helpers), an LSA inconsistent with the pre-restart Router-LSA received, or grace period expires
  -> Constraint: §2.3 -- on exit, re-originate Router-LSAs (all areas), re-originate Network-LSAs (segments where DR), re-run SPF + install routes, remove stale pre-restart FIB entries, flush stale received self-LSAs, flush own Grace-LSAs
  -> Constraint: §3.1 -- enter helper mode only if ALL: Full adjacency with X; no content change in the LSDB (types 1-5/7) since X restarted (only periodic refreshes on X's retransmission list); LS age < Grace Period; policy permits; helper not itself restarting. If already helping X, accept the new Grace-LSA and update the grace period
  -> Constraint: §3.2 -- exit helper on: Grace-LSA flushed, grace expiry, or (strict checking) an installed LSA with changed content that would have flooded to X; on exit recalc DR + re-originate Router-LSA (and Network-LSA if DR). A changed AS-external-LSA must NOT terminate helping for a neighbour in a stub area (it would never flood there)
  -> Constraint: §5 -- unplanned outage is config-gated: Grace-LSAs sent before any Hello, reason restricted to 0 (unknown) or 3 (switch to redundant CP), operator can disable
  -> Constraint: §B.1 -- RestartInterval range 1-1800 s, suggested default 120 s; RestartSupport = none / planned / planned+unplanned; RestartHelperSupport and RestartHelperStrictLSAChecking (default enabled) per §B.2
- [ ] `rfc/short/rfc5250.md` -- Opaque-LSA framework (carrier reference for the Type 9 scope rule the carrier enforces)
  -> Constraint: §3.1 -- a Type 9 LSA received on an interface other than the target interface MUST be discarded and not acknowledged; the Grace-LSA is bound to the single link it arrived on, so a helper keys it to that interface's neighbour
  -> Constraint: §3 / App A.2 -- the Opaque Type / Opaque ID split (3 / 0 for Grace-LSA) is owned by the ext-1 carrier; this consumer passes opaqueType=3, opaqueID=0 and never re-derives the LS-ID layout

**Key insights:** (minimal context to resume after compaction)
- Grace-LSA is just an Opaque Type 3 body; ext-1 owns the carrier, so this spec is a CONSUMER, not a flooding change. The hard part is the two control-plane state machines, not the wire.
- FIB retention is NOT new code: the fib-kernel `RTPROT_ZE` + `sweepDelay` stale-mark-then-sweep already keeps kernel routes across a process restart. The restarter's job is to (a) NOT call `Installer.RemoveAll` on a graceful stop, and (b) close the grace window (re-install routes) before the fib sweep deadline.
- "Suppress self-LSA origination during restart" = gate `originateSelfLSAs` (instance.go) and the SPF route install (`spf.Installer.Apply`) behind a per-engine "in graceful restart" flag.
- "Helper freezes the adjacency at Full" = the helper continues to advertise the link to X in its Router-LSA even though the NSM may regress; it does NOT mean the NSM is frozen. The Router-LSA topology builder must keep X's link while helping.
- LS age is the grace clock: the helper's expiry timer reads the Grace-LSA's LS age vs the Grace Period TLV; never a separate wall clock for the window length.
- Backward compatibility is automatic (§4): no capability negotiation beyond the O-bit (ext-1). A non-helper neighbour reverts the restart to a normal restart with no loops.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/instance.go` -- `originateSelfLSAs()` is the single self-LSA origination chokepoint (calls `lsdb.OriginateFromTopology` / `v6OriginateSelf`); `shutdown()` cancels the context, closes transport, waits the WG; the boot-count NVS seam is `loadOSPFBootCount(openBootCountStore())` seeded once at construction; `runOSPFEngine` is the forked-subprocess lifecycle
  -> Constraint: the restarter's "suppress origination" gate wraps `originateSelfLSAs` (return early while the grace window is open); the restart-fact NVS persistence reuses the `openBootCountStore()` ZeFS blob-store seam, not a new store
- [ ] `internal/plugins/ospf/spf/install.go` -- `Installer.Apply` diffs computed routes and inserts/removes `locrib.Path` via `loc.InsertForward`/`loc.Remove`; `RemoveAll` withdraws every OSPF path; `loc` may be nil in a forked subprocess
  -> Constraint: during restart the restarter must NOT call `Apply` (no route churn) and must NOT call `RemoveAll` on the graceful stop; on exit it resumes `Apply` so the Loc-RIB is reconciled and the fib-kernel sweep refreshes matching routes instead of deleting them
- [ ] `internal/plugins/fib/kernel/backend.go` + `backend_linux.go` -- `sweepDelay = 30s`; routes are marked `RTPROT_ZE` (250); startup `startupSweep` marks existing ZE routes stale, refreshes the ones re-installed within `sweepDelay`, and sweeps the rest (crash-recovery stale-mark-then-sweep)
  -> Constraint: this is exactly the FIB-retention substrate -- kernel routes survive the OSPF subprocess restart and are refreshed when SPF re-installs them on GR exit; the GR grace period and the restarter exit MUST complete within `sweepDelay` (or the sweep delay must be reconciled with the grace period) so non-stop forwarding holds; this coupling is a design constraint, not new code
- [ ] `internal/plugins/ospf/neighbor/neighbor.go` -- the `Neighbor` record (State, Options, RouterID, Address), `FloodNeighbor` value snapshot used by flooding, `Snapshot` for `show`; `state` enum Down..Full; `EventSink.NeighborUp/Down`
  -> Constraint: the helper does NOT add a neighbor state; it adds a per-neighbour "helping (restart-in-progress)" flag consulted by the Router-LSA topology builder and the LSDB-churn suppressor; the NSM stays RFC 2328
- [ ] `internal/plugins/ospf/neighbor/nsm.go` + `table.go` -- the NSM is event-driven (`shouldAdj`, `startExchange`); `table.go` owns the neighbor map, `NeighborDown`, `AdjOK`, `FloodNeighbors`
  -> Constraint: helper entry/exit hangs off Grace-LSA reception and the helper timer, not off NSM events; the NSM is unaware of GR except that the helper keeps X's link advertised across a transient regression
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `OriginateLinkSelf`/`OriginateSelf` + `SelfLSAEncoder` (the link-scope self-origination the Grace-LSA reuses via ext-1); `flushReceivedSelfLSA` / `handleSelfReceived` (the §13.4 "neighbour restarted -> flush my received self-LSA" path); `FlushStaleSelfLSAs` (MaxAge flush of stale self-LSAs on exit)
  -> Constraint: during restart the restarter must NOT run `handleSelfReceived`'s flush of its own pre-restart self-LSAs (§2 "do not modify/flush received self-originated LSAs"); on exit it uses `FlushStaleSelfLSAs` to purge the ones that are now stale (§2.3)
- [ ] `internal/plugins/ospf/lsdb/flooding.go` -- `ReceiveUpdate` runs §13 receive and `notifyChange` on a content change; the helper's "LSDB unchanged since restart" check and the strict-checking "changed LSA that would flood to X" exit trigger read this path
  -> Constraint: the helper hooks the post-install content-change signal (the same signal `notifyChange` raises) to evaluate the §3.2 strict-checking exit; it does NOT change §13 receive semantics
- [ ] `internal/plugins/ospf/auth_keystore.go` -- `loadOSPFBootCount(store)` reads a ZeFS blob, increments, writes back; `openBootCountStore()` opens the `pkg/zefs` blob store under `internal/core/paths`
  -> Constraint: the restart-fact NVS ({restarting, grace-end, reason}) reuses this ZeFS blob-store pattern (a sibling blob), so a planned restart survives the process restart without a new persistence subsystem
- [ ] `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` -- `max-metric` (RFC 6987 stub-router) is the precedent for a restart/shutdown timer config: `max-metric/router-lsa/{always,on-startup,on-shutdown}`; `ospfConfig` carries `MaxMetric maxMetricConfig`
  -> Constraint: `graceful-restart` config follows the `max-metric` shape (a sibling container with restarter + helper sub-containers); `on-shutdown` already exists as the "graceful shutdown seconds" precedent, but GR is a distinct mechanism (Grace-LSA + FIB retention), not stub-router

**Behavior to preserve:**
- The ext-1 opaque carrier (link-store flooding, LS-ID split, O-bit DD gate, TLV iterator/builder, verbatim re-flood) -- this spec adds only a consumer; the carrier is unchanged.
- The OSPFv2 NSM, the `originateSelfLSAs` topology builder, the SPF route table + `Installer.Apply` insert/remove shape, the fib-kernel `RTPROT_ZE` + `sweepDelay` reconciliation.
- All existing OSPFv2 functional/interop tests: a router with GR disabled (the default) behaves exactly as today -- it originates no Grace-LSA, never enters helper mode, and restarts normally.
- The §13.4 "neighbour restarted -> flush my received self-LSA" path for the non-GR case (only the GR restarter suppresses its OWN self-LSA flush during restart).

**Behavior to change:** (all RFC-3623-required, gated behind GR config)
- `originateSelfLSAs`: return early (suppress) while this engine is in graceful restart (§2).
- The SPF route install: skip `Installer.Apply` while in graceful restart; do NOT `RemoveAll` on a graceful stop (§2, §2.1).
- The Router-LSA topology builder: while helping X, keep X's link advertised even if the NSM regressed; keep X as DR if X was DR (§3).
- Engine stop: distinguish a graceful restart (preserve FIB, persist restart-fact, originate Grace-LSAs) from a normal shutdown (existing path).
- LSDB receive: surface the post-install content-change signal to the helper's strict-checking exit (§3.2) and to the restarter's inconsistent-LSA exit (§2.2).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Restarter trigger:** an operator command (`ospf graceful-restart prepare`, or a managed reload) -> the engine enters the pre-restart phase -> originate Grace-LSAs -> persist NVS -> stop without `RemoveAll`. After the subprocess restarts, the persisted restart-fact puts the engine into the in-restart phase.
- **Helper trigger:** an LS Update carrying a Type 9 / Opaque Type 3 Grace-LSA arrives -> ext-1 carrier installs it in the link store and calls this consumer's `OnReceive` -> the helper evaluates the §3.1 entry checks for the originating neighbour.
- **Exit triggers:** restarter -- all-adjacencies-up / inconsistent-LSA / grace-timer; helper -- Grace-LSA flushed / grace-timer / strict-checking topology change.

### Transformation Path
1. **Pre-restart (restarter):** ensure the FIB is current (the last `Installer.Apply` has settled); for each interface build a Grace-LSA body via the ext-1 TLV builder (type 1 Grace Period = configured interval, type 2 Reason, type 3 IP Interface Address on shared media); call `RegisterOpaqueConsumer`'s origination path (opaqueType 3, opaqueID 0, scope link) so the carrier originates with LS age 0 and reliably floods; persist {restarting, grace-end, reason} to the ZeFS NVS blob.
2. **Graceful stop (restarter):** the engine stops WITHOUT `Installer.RemoveAll`; kernel routes (RTPROT_ZE) remain; the subprocess exits.
3. **In-restart (restarter, after resume):** the persisted restart-fact (grace-end not yet passed) sets the in-restart flag; `originateSelfLSAs` is suppressed; `Installer.Apply` is suppressed; SPF runs (virtual-link restore) without installing; received self-LSAs are NOT flushed; DR re-election runs if a Hello in Waiting state lists self as DR.
4. **Helper entry (helper):** `OnReceive` parses the three TLVs via the ext-1 iterator; identifies X (type 3 IP address on broadcast/NBMA/P2MP, else the Advertising Router); runs the §3.1 checks; on pass, sets the per-neighbour helping flag, records grace-end = now + min(Grace Period TLV, remaining), keeps X as DR if X was DR; if already helping X, just updates the grace period.
5. **While helping (helper):** the Router-LSA topology builder keeps X's link advertised (and the Network-LSA if DR) regardless of NSM state; the LSDB-churn suppressor holds; the strict-checking watcher evaluates each installed LSA's content-change-that-would-flood-to-X signal.
6. **Restarter exit:** on the earliest of the three triggers, clear the in-restart flag, re-run `originateSelfLSAs` (Router-LSAs all areas, Network-LSAs where DR), re-run SPF + `Installer.Apply` (routes re-installed -> fib-kernel sweep refreshes them), `FlushStaleSelfLSAs` for now-stale received self-LSAs, and originate the Grace-LSAs at MaxAge to flush them; clear the NVS restart-fact.
7. **Helper exit:** on the earliest trigger, clear the per-neighbour helping flag, recalc DR for the segment, re-originate the Router-LSA (and Network-LSA if DR) so the frozen adjacency view is corrected.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator / managed reload <-> restarter | `ospf graceful-restart prepare` RPC (or a managed-reload hook) sets the pre-restart phase | [ ] |
| Grace-LSA body <-> ext-1 carrier | `RegisterOpaqueConsumer(3, link, OnOriginate, OnReceive)`; body built/parsed with the ext-1 TLV builder/iterator | [ ] |
| Restarter <-> self-LSA origination | the in-restart flag gates `originateSelfLSAs` (instance.go) | [ ] |
| Restarter <-> route install | the in-restart flag gates `spf.Installer.Apply`; graceful stop skips `RemoveAll` | [ ] |
| Restarter <-> FIB retention | fib-kernel `RTPROT_ZE` routes persist; SPF re-install on exit refreshes them within `sweepDelay` | [ ] |
| Restarter <-> NVS | `{restarting, grace-end, reason}` persisted via the `pkg/zefs` blob store (sibling of the boot-count blob) | [ ] |
| Helper <-> Router-LSA builder | the per-neighbour helping flag keeps X's link advertised in `originateSelfLSAs` topology | [ ] |
| Helper <-> LSDB churn | the helping flag + the post-install content-change signal drive the §3.2 strict-checking exit | [ ] |

### Integration Points
- `internal/plugins/ospf` (engine) -- the in-restart flag, the helper map, the pre-restart/exit orchestration; gates `originateSelfLSAs` and the route install; the restart-fact NVS.
- `internal/plugins/ospf` opaque consumer registration (ext-1) -- `RegisterOpaqueConsumer(3, link, ...)`.
- `internal/plugins/ospf/lsdb` -- `OriginateLinkSelf` (Grace-LSA origination via ext-1), `FlushStaleSelfLSAs` (exit cleanup), the post-install content-change signal (helper strict checking).
- `internal/plugins/ospf/neighbor` -- the per-neighbour helping flag surfaced into the topology snapshot (`NeighborInfo`/`FloodNeighbor`); DR-was-X preservation.
- `internal/plugins/ospf/spf` -- READ/gate: `Installer.Apply` suppressed during restart, resumed on exit; SPF still computed (virtual-link restore) but not installed.
- `internal/plugins/fib/kernel` -- READ ONLY: the `RTPROT_ZE` + `sweepDelay` retention substrate the restarter relies on (no code change here; the coupling is verified by the FIB-retention QEMU test).
- `internal/plugins/ospf/auth_keystore.go` -- the ZeFS NVS blob-store pattern reused for the restart-fact.
- `internal/plugins/ospf/config.go` + `yang/` -- the `graceful-restart` config and the `show ip ospf graceful-restart` command.

### Architectural Verification
- [ ] No bypassed layers (Grace-LSA flows wire -> ext-1 carrier -> `OnReceive`; origination flows the GR consumer -> ext-1 link-store self-origination; no direct LSDB poke)
- [ ] No unintended coupling (the ext-1 carrier names no GR; the GR consumer depends on the carrier, not vice-versa; fib-kernel is read-only)
- [ ] No duplicated functionality (reuses `originateSelfLSAs`, `FlushStaleSelfLSAs`, `Installer.Apply`/`RemoveAll`, the fib-kernel sweep, the ZeFS blob store; adds only the two state machines, the body, the gates, and the config/CLI/metrics)
- [ ] Zero-copy preserved (Grace-LSA body built buffer-first via the ext-1 builder; TLV parse is a zero-copy iterator view)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ext-1 exposes `RegisterOpaqueConsumer(opaqueType, scope, OnOriginate, OnReceive)` with link scope, and originating with opaqueID 0 / scope link produces a Type 9 Grace-LSA flooded only to opaque-capable neighbours | `plan/spec-ospf-ext-1-opaque-framework.md` "Consumer registry" + "Data Flow" steps 5-6 | the GR consumer must add its own opaque flooding; large scope creep | `TestGraceLSAOriginatedViaCarrier` originates and observes a Type 9/Opaque-3 LSA in the link store | unvalidated |
| A-2 | The ext-1 TLV builder/iterator pad to 4-octet alignment exactly as RFC 3623 §A requires (RFC 3630 TLV format), so the three Grace-LSA TLVs round-trip against FRR | ext-1 "Generic TLV carriage" + `rfc/short/rfc3623.md` §A length/padding rule | the body is mis-padded; FRR rejects the Grace-LSA | `TestGraceLSATLVRoundTrip` (1/2/3-byte values padded) + `ospf-gr-frr` interop | unvalidated |
| A-3 | The fib-kernel `RTPROT_ZE` routes survive the OSPF subprocess restart and the `sweepDelay` (30 s) window is long enough for a default 120 s grace period IF the restarter re-installs routes early; if not, the sweep deletes them before GR exit | `internal/plugins/fib/kernel/backend.go` `sweepDelay`; `backend_linux.go` startup stale-mark-then-sweep | the FIB is swept mid-restart -> black hole (the exact thing GR must prevent) | `ospf-gr-fib-retention` QEMU test: routes stay programmed across a graceful restart; design reconciles grace period vs `sweepDelay` | unvalidated |
| A-4 | `originateSelfLSAs` is the single self-LSA origination chokepoint, so gating it suppresses ALL types 1-5/7 self-origination during restart | `internal/plugins/ospf/instance.go` `originateSelfLSAs` -> `OriginateFromTopology` | a self-LSA leaks during restart (violates §2) | `TestRestarterSuppressesSelfLSAs` asserts no Router/Network/Summary/External self-LSA re-origination while in restart | unvalidated |
| A-5 | The restarter can run SPF for virtual-link restore without installing routes by suppressing `Installer.Apply` while leaving the SPF computation running | `internal/plugins/ospf/spf/install.go` `Apply` is the only install seam; the SPF compute is separate | SPF compute and install are entangled; suppressing one breaks the other | `TestRestarterRunsSPFNoInstall` (SPF table populated, `loc` unchanged) | unvalidated |
| A-6 | A helper can keep X's link advertised in its Router-LSA across a transient NSM regression by consulting a per-neighbour helping flag in the topology builder, without freezing the NSM | `internal/plugins/ospf/instance.go` `lsdbTopology` builds `NeighborInfo` from `FloodNeighbors`; `neighbor/neighbor.go` `FloodNeighbor` | the helper drops X's link, prematurely terminating X's restart | `TestHelperKeepsAdjacencyAdvertised` (Router-LSA still lists X while helping) | unvalidated |
| A-7 | The post-install content-change signal (the one `notifyChange` raises in `lsdb/flooding.go`) is sufficient to drive the §3.2 strict-checking exit ("changed LSA that would flood to X") without a new flooding hook | `internal/plugins/ospf/lsdb/flooding.go` `ReceiveUpdate` -> `notifyChange` | strict checking needs a new per-LSA "would-flood-to-X" signal; more LSDB plumbing | `TestHelperStrictExitOnTopologyChange` + the stub-area exception test | unvalidated |
| A-8 | The ZeFS blob-store seam used for the boot count (`openBootCountStore` / `loadOSPFBootCount`) can persist a second small blob (restart-fact) for planned-restart survival | `internal/plugins/ospf/auth_keystore.go` `bootCountStore` interface + `pkg/zefs` | a new NVS subsystem is needed | `TestRestartFactPersistsAcrossRestart` (write, re-open, read grace-end) | unvalidated |
| A-9 | GR with default config OFF is fully backward compatible: no Grace-LSA originated, no helper entry, normal restart; existing OSPF tests stay green | `rfc/short/rfc3623.md` §4 (automatic backward compat, no negotiation); the default-off config | enabling the consumer changes default behaviour; interop regression | existing OSPF suite green with GR disabled; `TestGRDisabledNoGraceLSA` | unvalidated |
| A-10 | The DR-was-X state needed for "keep X as DR while helping" / "re-elect self DR" is recoverable from the pre-restart Hello/election state (Hello in Waiting state lists self as DR for the restarter; the helper records X's DR role at helper entry) | `internal/plugins/ospf/iface/election.go`, `neighbor.go` `DeclaredDR` | DR continuity is lost across restart -> election churn defeats GR | `TestRestarterReElectsSelfDR`, `TestHelperKeepsXAsDR` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The fib-kernel sweep deletes OSPF routes mid-restart (grace period > `sweepDelay`) -> the exact black hole GR must prevent | `ospf-gr-fib-retention` shows a forwarding gap; kernel loses RTPROT_ZE routes during the window | the restarter re-installs routes on the FIRST SPF after resume (not only on full GR exit), and the design pins the relationship between the grace window, the restarter exit, and `sweepDelay`; the FIB-retention QEMU test is the gate |
| R-2 | A self-LSA leaks during restart (a code path other than `originateSelfLSAs` originates) -> violates §2, peers see a changed Router-LSA, helpers exit early | a peer logs a Router-LSA change for the restarting router during the window | audit ALL origination call sites; the in-restart flag gates the topology builder AND `FlushStaleSelfLSAs`; `TestRestarterSuppressesSelfLSAs` asserts no leak across all self-LSA types |
| R-3 | The helper drops X's link on a transient NSM regression -> premature termination of X's restart (the partial-segment-helping pitfall) | X exits GR early; the adjacency flaps | the per-neighbour helping flag keeps X's link advertised in the topology builder for the whole grace window; `TestHelperKeepsAdjacencyAdvertised` |
| R-4 | Strict LSA checking fires on a benign change (e.g. an AS-external-LSA change that would NOT flood to X in a stub area) -> the helper exits early for no reason | helping ends right after an unrelated external change | implement the §3.2 stub-area exception: only LSAs that WOULD flood to X count; `TestHelperStubAreaExternalDoesNotExit` |
| R-5 | LS age reset on Grace-LSA retransmit extends the grace period indefinitely (the §A pitfall) | the helper's window never closes; a stuck restarter holds the adjacency forever | originate the Grace-LSA once at LS age 0 and retransmit the SAME instance (ext-1 reliable flooding); never re-stamp LS age; `TestGraceLSAAgeNotResetOnRetransmit` |
| R-6 | A forged Grace-LSA spoofs a restart for a router that was actually withdrawn (the §Security spoofing risk) | a router that is really down is held adjacent by helpers | Grace-LSAs ride the existing OSPF cryptographic auth (ospf-12); the helper still requires a prior Full adjacency with X; document the residual risk |
| R-7 | A planned restart's NVS restart-fact is stale (the process restarted for an unrelated reason after the grace window) -> the engine wrongly suppresses origination on a normal boot | a cold boot starts in in-restart mode with no real GR in flight | the restart-fact records grace-end; on resume, if grace-end has passed, ignore it and boot normally; `TestStaleRestartFactIgnored` |
| R-8 | The unplanned-outage path (§5) sends Grace-LSAs after a crash without a guaranteed-sane FIB -> forwarding on stale entries | a crashed router resumes forwarding on routes that no longer match the topology | unplanned support is config-gated (default off), reason restricted to 0/3, Grace-LSAs before any Hello; the operator opt-in is mandatory (§5); `TestUnplannedDisabledByDefault` |
| R-9 | A helper that helps on some segments but not others causes premature termination (the partial-segment pitfall) | X's restart ends as soon as one helper drops a segment | helper policy is all-or-nothing per the configured support; document that per-segment partial helping is not offered; aggregate-adjacency exit (§3.2) is honoured |
| R-10 | The grace period exceeds LSRefreshTime (1800 s) -> the restarting router's own LSAs age out mid-restart, defeating GR | the restarter's pre-restart LSAs disappear during the window | YANG `range "1..1800"` on the interval; a doctor/validation warning if the configured interval approaches 1800; `TestGracePeriodRangeRejectsAbove1800` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| The GR consumer's `init()` calls `RegisterOpaqueConsumer(3, link, OnOriginate, OnReceive)` | -> | the ext-1 carrier stores the Opaque-Type-3 registration; the engine discovers it | `TestGraceLSAConsumerRegistered` (unit) + `test/ospf/ospf-gr-register.ci` |
| `ospf graceful-restart prepare` (operator/managed) | -> | the engine enters pre-restart, originates one Grace-LSA per interface via the carrier, persists the NVS restart-fact | `test/ospf/ospf-gr-prepare.ci` |
| An LS Update carrying a Type 9 / Opaque-3 Grace-LSA arrives from X | -> | ext-1 `OnReceive` -> helper entry checks -> helping flag set; X's link stays advertised | `test/ospf/ospf-gr-helper.ci` |
| The OSPF subprocess restarts with a valid NVS restart-fact | -> | the in-restart flag is set; `originateSelfLSAs` and `Installer.Apply` are suppressed; FIB retained | `ospf-gr-fib-retention` (QEMU) |
| The grace period expires (helper) | -> | the helper exits, recalcs DR, re-originates its Router-LSA | `TestHelperExitOnGraceExpiry` (unit) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GR enabled; the GR consumer registers | `RegisterOpaqueConsumer(3, link, ...)` is stored; the engine invokes `OnOriginate`/`OnReceive` for Opaque Type 3 (§A) |
| AC-2 | A planned restart is requested; one or more OSPF interfaces are up | one Grace-LSA is originated per interface, LS age 0, with type-1 Grace Period and type-2 Reason TLVs always present, and a type-3 IP Interface Address TLV on broadcast/NBMA/P2MP segments (§A) |
| AC-3 | A Grace-LSA is retransmitted (reliable flooding) before it is acked | the SAME instance is retransmitted; LS age is NOT reset and DoNotAge is NOT set (§A, R-5) |
| AC-4 | The restart-fact is persisted, then the OSPF subprocess restarts within the grace window | on resume the engine enters in-restart mode (grace-end not passed); a restart-fact whose grace-end has passed is ignored and the engine boots normally (§2.1, R-7) |
| AC-5 | The router is in graceful restart | it does NOT originate LSA types 1-5/7 (relies on pre-restart LSAs) and does NOT modify/flush its received self-originated LSAs (§2) |
| AC-6 | The router is in graceful restart and SPF runs | SPF is computed (virtual-link restore) but NO OSPF routes are installed/removed; the pre-restart FIB (RTPROT_ZE kernel routes) remains programmed (§2, A-3, A-5) |
| AC-7 | The router was DR on a segment before restart and a Hello in Waiting state lists it as DR | it re-elects itself DR on that segment (§2, A-10) |
| AC-8 | All adjacencies are re-established (pre-restart Router/Network-LSA reflected by helpers) | the restarter exits graceful restart and runs the exit actions (§2.2) |
| AC-9 | An LSA inconsistent with the pre-restart Router-LSA is received during restart | the restarter exits graceful restart immediately and runs the exit actions (§2.2) |
| AC-10 | The grace period expires before all adjacencies are up | the restarter exits graceful restart and runs the exit actions (§2.2) |
| AC-11 | The restarter exits graceful restart (any trigger) | it re-originates Router-LSAs (all areas) and Network-LSAs (segments where DR), re-runs SPF and installs routes (refreshing the RTPROT_ZE routes within `sweepDelay`), flushes stale received self-LSAs, and flushes its own Grace-LSAs at MaxAge (§2.3) |
| AC-12 | A Grace-LSA is received from X and ALL §3.1 checks pass (Full adjacency, LSDB unchanged since X restarted, grace not expired, policy permits, helper not restarting) | the router enters helper mode for X, advertises the adjacency to X (Router-LSA, and Network-LSA if DR) regardless of synchronisation state, and keeps X as DR if X was DR (§3, §3.1) |
| AC-13 | A Grace-LSA is received from X but at least one §3.1 check fails | the router does NOT enter helper mode (§3.1) |
| AC-14 | A new Grace-LSA arrives from X while already helping X on the segment | the existing helper relationship is kept and the grace period is updated; no re-entry churn (§3.1) |
| AC-15 | While helping X: the Grace-LSA is flushed, OR the grace period expires, OR (strict checking on) an LSA is installed with changed content that would have flooded to X | the router exits helper mode for X, recalcs the DR, and re-originates its Router-LSA (and Network-LSA if DR) (§3.2) |
| AC-16 | While helping X in a stub area: a changed AS-external-LSA (type 5) is installed that would NOT flood to X | helping for X does NOT terminate (the §3.2 stub-area exception, R-4) |
| AC-17 | Unplanned-outage support is disabled (the default) and the router cold-boots without a planned restart-fact | no Grace-LSA is originated; the router boots normally (§5, R-8) |
| AC-18 | Unplanned-outage support is enabled by the operator | on a cold/unplanned start Grace-LSAs are sent BEFORE any Hello, with reason restricted to 0 (unknown) or 3 (switch to redundant CP) (§5) |
| AC-19 | The configured grace period (RestartInterval) | accepts 1-1800 s, default 120 s; a value above 1800 is rejected by YANG validation (§2.1, §B.1, R-10) |
| AC-20 | GR is disabled (the default) | no Grace-LSA is originated, helper mode is never entered, and a restart behaves exactly as today (§4 backward compatibility, A-9) |
| AC-21 | `show ip ospf graceful-restart` | reports the restarter state (in-restart / not, grace-end, reason) and the per-neighbour helper state (helping which neighbours, remaining grace) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables GR and triggers a planned restart; routes keep forwarding across the restart | `graceful-restart` config -> `prepare` -> Grace-LSA per interface + NVS persist -> graceful stop (no `RemoveAll`) -> subprocess restart -> in-restart suppression -> RTPROT_ZE routes retained -> GR exit re-installs -> sweep refreshes | `ospf-gr-fib-retention` (QEMU) |
| 2 | A Ze router is a helper for a restarting FRR neighbour | FRR floods a Grace-LSA -> wire -> ext-1 carrier -> `OnReceive` -> §3.1 checks -> helping; Ze keeps the adjacency to FRR advertised; FRR completes GR without flapping | `ospf-gr-frr` interop (Ze helper, FRR restarter) |
| 3 | An FRR router is a helper for a restarting Ze neighbour | Ze originates Grace-LSAs, restarts, re-acquires adjacencies; FRR holds the adjacency; Ze exits GR cleanly with no route flap | `ospf-gr-frr` interop (Ze restarter, FRR helper) |
| 4 | Runs `show ip ospf graceful-restart` during a restart | CLI -> the GR state reporter -> restarter/helper state rendered | `test/ospf/ospf-gr-show.ci` |
| 5 | Leaves GR disabled (default) and restarts the router | no Grace-LSA, normal restart, routes reconverge normally | `TestGRDisabledNoGraceLSA` + existing OSPF suite green |
| 6 | Decodes a Grace-LSA hex capture | CLI decode -> ext-1 opaque decode -> Opaque Type 3 + the three TLVs rendered | `test/ospf/ospf-gr-decode.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGraceLSAConsumerRegistered` | `internal/plugins/ospf/gr_register_test.go` | AC-1: `RegisterOpaqueConsumer(3, link, ...)` stored; engine invokes the callbacks | |
| `TestGraceLSABodyBuild` | `internal/plugins/ospf/gr_lsa_test.go` | AC-2: body has type-1 Grace Period + type-2 Reason always; type-3 IP addr on shared media | |
| `TestGraceLSATLVRoundTrip` | `internal/plugins/ospf/gr_lsa_test.go` | A-2: the three TLVs encode/parse byte-for-byte via the ext-1 builder/iterator with 4-octet padding | |
| `TestGraceLSAAgeNotResetOnRetransmit` | `internal/plugins/ospf/gr_lsa_test.go` | AC-3, R-5: retransmit keeps LS age, never DoNotAge | |
| `TestGraceLSAOriginatedViaCarrier` | `internal/plugins/ospf/gr_restarter_test.go` | A-1: origination produces a Type 9 / Opaque-3 LSA in the link store | |
| `TestRestarterSuppressesSelfLSAs` | `internal/plugins/ospf/gr_restarter_test.go` | AC-5, A-4, R-2: no Router/Network/Summary/External self-LSA re-origination while in restart | |
| `TestRestarterRunsSPFNoInstall` | `internal/plugins/ospf/gr_restarter_test.go` | AC-6, A-5: SPF computed, `Installer.Apply` not called, FIB retained | |
| `TestRestarterReElectsSelfDR` | `internal/plugins/ospf/gr_restarter_test.go` | AC-7, A-10: re-elect self DR when a Waiting-state Hello lists self as DR | |
| `TestRestarterExitAllAdjacencies` / `TestRestarterExitInconsistentLSA` / `TestRestarterExitGraceExpiry` | `internal/plugins/ospf/gr_restarter_test.go` | AC-8/9/10: the three exit triggers | |
| `TestRestarterExitActions` | `internal/plugins/ospf/gr_restarter_test.go` | AC-11: re-originate Router/Network-LSAs, re-install routes, flush stale self-LSAs, flush own Grace-LSAs | |
| `TestRestartFactPersistsAcrossRestart` | `internal/plugins/ospf/gr_nvs_test.go` | AC-4, A-8: write, re-open, read grace-end via the ZeFS blob | |
| `TestStaleRestartFactIgnored` | `internal/plugins/ospf/gr_nvs_test.go` | AC-4, R-7: an expired restart-fact is ignored on resume | |
| `TestHelperEntryAllChecksPass` | `internal/plugins/ospf/gr_helper_test.go` | AC-12: enter helper when all §3.1 checks pass | |
| `TestHelperEntryRejectedPerCheck` | `internal/plugins/ospf/gr_helper_test.go` | AC-13: each failing check blocks entry (table-driven) | |
| `TestHelperAlreadyHelpingUpdatesGrace` | `internal/plugins/ospf/gr_helper_test.go` | AC-14: re-receipt updates the grace period, no churn | |
| `TestHelperKeepsAdjacencyAdvertised` | `internal/plugins/ospf/gr_helper_test.go` | AC-12, A-6, R-3: Router-LSA keeps X's link while helping | |
| `TestHelperKeepsXAsDR` | `internal/plugins/ospf/gr_helper_test.go` | AC-12, A-10: X stays DR while helping | |
| `TestHelperExitOnGraceExpiry` / `TestHelperExitOnFlush` / `TestHelperStrictExitOnTopologyChange` | `internal/plugins/ospf/gr_helper_test.go` | AC-15: the three exit triggers + DR recalc + Router-LSA re-origination | |
| `TestHelperStubAreaExternalDoesNotExit` | `internal/plugins/ospf/gr_helper_test.go` | AC-16, R-4, A-7: stub-area external change does not terminate helping | |
| `TestUnplannedDisabledByDefault` | `internal/plugins/ospf/gr_unplanned_test.go` | AC-17, R-8: no Grace-LSA on cold boot when unplanned support is off | |
| `TestUnplannedGraceBeforeHello` | `internal/plugins/ospf/gr_unplanned_test.go` | AC-18: Grace-LSAs before any Hello, reason 0 or 3 | |
| `TestGRDisabledNoGraceLSA` | `internal/plugins/ospf/gr_config_test.go` | AC-20, A-9: GR off -> no Grace-LSA, normal restart | |
| `TestGracePeriodRangeRejectsAbove1800` | `internal/plugins/ospf/config_test.go` | AC-19, R-10: YANG range 1-1800 enforced | |
| `TestGRShowState` | `internal/plugins/ospf/gr_show_test.go` | AC-21: restarter + helper state rendered | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Grace Period / RestartInterval (s) | 1-1800 | 1800 | 0 | 1801 |
| Grace Period TLV value length | 4 bytes (fixed) | 4 | a shorter type-1 TLV is malformed (ignore the Grace-LSA) | N/A |
| Restart Reason TLV value | 0-3 | 3 | N/A | a value >3 is treated as 0 (unknown) on receive |
| Restart Reason (unplanned, on send) | {0, 3} | 3 | N/A | reasons 1/2 are rejected for the unplanned path (§5) |
| Opaque Type (Grace-LSA) | 3 (fixed) | 3 | N/A | N/A |
| Opaque ID (Grace-LSA) | 0 (fixed) | 0 | N/A | N/A |
| Helper grace remaining (LS age vs Grace Period) | 0-1800 | 1800 | expired -> no entry | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-gr-register` | `test/ospf/ospf-gr-register.ci` | GR enabled; the consumer + `show ip ospf graceful-restart` are present | |
| `ospf-gr-prepare` | `test/ospf/ospf-gr-prepare.ci` | a planned restart originates one Grace-LSA per interface; the NVS restart-fact is written | |
| `ospf-gr-helper` | `test/ospf/ospf-gr-helper.ci` | a received Grace-LSA enters helper mode; the adjacency to X stays advertised; exit on grace expiry | |
| `ospf-gr-show` | `test/ospf/ospf-gr-show.ci` | `show ip ospf graceful-restart` reports restarter + helper state | |
| `ospf-gr-decode` | `test/ospf/ospf-gr-decode.ci` | decode of a Grace-LSA hex shows Opaque Type 3 + the three TLVs | |
| `ospf-gr-disabled` | `test/ospf/ospf-gr-disabled.ci` | GR off: no Grace-LSA, normal restart, routes reconverge | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-gr-frr` | `test/interop/scenarios/ospf-gr-frr/` | FRR `ospfd` (graceful-restart + helper) | Ze-helper holds the adjacency while FRR restarts (no flap); Ze-restarter is helped by FRR and exits GR cleanly; Grace-LSA TLVs interop both directions | |
| `ospf-gr-fib-retention` | `test/interop/scenarios/ospf-gr-fib-retention/` | FRR `ospfd` helper + a traffic probe | across a Ze planned restart the RTPROT_ZE kernel routes stay programmed and forwarding continues (non-stop forwarding); routes are refreshed (not swept) on GR exit | |

> Interop is required: this changes wire behaviour (Grace-LSA origination + helper
> reaction) and forwarding behaviour (FIB retention). The raw-IP / multicast flood
> and the real kernel-route retention are Linux-only and run as QEMU integration
> tests (`ai/rules/qemu-testing.md`), consistent with the rest of the OSPF interop
> set (`ospf-p2p-frr`, `ospf-broadcast-frr`, ...).

### Future (if deferring any tests)
- None. Every AC is covered by a unit, functional, or interop test above.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/instance.go` -- the in-restart flag + the helper map on the engine; gate `originateSelfLSAs`; the pre-restart / exit orchestration; the restart-fact load on `runOSPFEngine` resume; distinguish graceful stop from `shutdown()`
- `internal/plugins/ospf/spf/install.go` -- a "suppress install while in restart" gate around `Apply` (skip insert/remove) and a guard so a graceful stop does not call `RemoveAll`
- `internal/plugins/ospf/lsdb/origination.go` -- skip `handleSelfReceived`'s flush of the restarter's own pre-restart self-LSAs while in restart; `FlushStaleSelfLSAs` reused on exit
- `internal/plugins/ospf/lsdb/flooding.go` -- surface the post-install content-change signal (already raised for `notifyChange`) to the helper strict-checking exit and the restarter inconsistent-LSA exit
- `internal/plugins/ospf/instance.go` `lsdbTopology` / `NeighborInfo` -- carry the per-neighbour helping flag and "X was DR" so the topology builder keeps X's link advertised
- `internal/plugins/ospf/neighbor/neighbor.go` -- a per-neighbour helping flag on the topology snapshot (`FloodNeighbor` / `NeighborInfo`); record X's DR role at helper entry
- `internal/plugins/ospf/register.go` -- register the Opaque-Type-3 consumer (via ext-1), the `graceful-restart` config resolution, the `show ip ospf graceful-restart` command, the GR doctor check, and the GR metrics
- `internal/plugins/ospf/config.go` -- resolve the `graceful-restart` config (restarter support/interval/unplanned, helper support/strict-checking) into `ospfConfig`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `graceful-restart` container (mirrors the `max-metric` precedent)
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `show ip ospf graceful-restart` command
- `internal/plugins/ospf/cmd_show.go` -- the `show ip ospf graceful-restart` handler
- `internal/plugins/ospf/doctor.go` -- a doctor check for the GR NVS blob path / unplanned-support sanity (only the NVS path is a new runtime dependency)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `graceful-restart` container; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `restart-interval` `range "1..1800"`; `support` enumeration {disabled, planned, planned-and-unplanned}; `helper` `strict-lsa-checking` boolean |
| YANG custom validators | [ ] no | native range + enumeration + boolean suffice |
| CLI commands/flags | [ ] yes | `show ip ospf graceful-restart` in `ze-ospf-cmd.yang` + `cmd_show.go`; an operator `ospf graceful-restart prepare` action (managed-reload hook) |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ip ospf graceful-restart` |
| Editor autocomplete | [ ] yes | automatic for the YANG enumeration/boolean leaves + the new show subcommand |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-gr-*.ci` |
| Pipe completeness | [ ] yes | `show ip ospf graceful-restart` routes through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | GR is operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] yes | the GR NVS blob path is a new file dependency -> a doctor check + `internal/core/diagnostic/codes.go` code + unit + functional test (see `ai/rules/doctor-checks.md`) |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_gr_restarter_active` | gauge | (none; 1 while in graceful restart) |
| `ze_ospf_gr_restarter_exits_total` | counter | `reason` (adjacencies / inconsistent-lsa / grace-expiry) |
| `ze_ospf_gr_helper_sessions` | gauge | `interface` |
| `ze_ospf_gr_helper_exits_total` | counter | `reason` (flushed / grace-expiry / topology-change) |
| `ze_ospf_gr_grace_lsas` | gauge | `direction` (originated / received) |

> These extend the umbrella's canonical OSPF metric set; they use the
> `ze_ospf_gr_*` prefix and are registered by this spec's owner code. The umbrella
> "Metrics" table must gain these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF Graceful Restart (restarter + helper) |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `graceful-restart` container |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ip ospf graceful-restart` |
| 4 | API/RPC added/changed? | [ ] yes | `docs/architecture/api/commands.md` -- the `prepare` action / managed-reload hook |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains a GR opaque consumer |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- a Graceful Restart section |
| 7 | Wire format changed? | [ ] yes | `docs/architecture/wire/ospf.md` -- the Grace-LSA (Opaque Type 3) + its TLVs |
| 8 | Plugin SDK/protocol changed? | [ ] no | uses the ext-1 `RegisterOpaqueConsumer` API; no new SDK surface |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc3623.md` -- flip the Compliance Checklist items to implemented |
| 10 | Test infrastructure changed? | [ ] yes (two interop scenarios) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF GR parity with FRR (restarter + helper) |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- GR state machines + FIB-retention coupling |
| 13 | Route metadata keys added/changed? | [ ] no | GR does not add route metadata keys |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the five `ze_ospf_gr_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF / fib-kernel files |
| 17 | Existing docs show examples for this area? | [ ] check | verify any OSPF config/CLI examples against the new `graceful-restart` container |

## Files to Create
- `internal/plugins/ospf/gr.go` -- the GR consumer: `RegisterOpaqueConsumer(3, link, ...)`, the engine-side orchestration glue (pre-restart, in-restart gating, exit), the helper map
- `internal/plugins/ospf/gr_lsa.go` -- the Grace-LSA body build/parse (the three TLVs) over the ext-1 TLV builder/iterator
- `internal/plugins/ospf/gr_restarter.go` -- the restarter state machine (pre-restart, in-restart suppression, the three exit triggers, exit actions)
- `internal/plugins/ospf/gr_helper.go` -- the helper state machine (§3.1 entry checks, while-helping advertisement, §3.2 exit incl. strict checking + stub-area exception)
- `internal/plugins/ospf/gr_nvs.go` -- the restart-fact ZeFS blob (write on prepare, read/validate on resume), reusing the `openBootCountStore` seam
- `internal/plugins/ospf/gr_show.go` -- the `show ip ospf graceful-restart` state reporter
- `internal/plugins/ospf/gr_register_test.go`, `gr_lsa_test.go`, `gr_restarter_test.go`, `gr_helper_test.go`, `gr_nvs_test.go`, `gr_unplanned_test.go`, `gr_config_test.go`, `gr_show_test.go`
- `test/ospf/ospf-gr-register.ci`, `ospf-gr-prepare.ci`, `ospf-gr-helper.ci`, `ospf-gr-show.ci`, `ospf-gr-decode.ci`, `ospf-gr-disabled.ci`
- `test/interop/scenarios/ospf-gr-frr/` -- `ze.conf`, `frr.conf`, `check.py`
- `test/interop/scenarios/ospf-gr-fib-retention/` -- `ze.conf`, `frr.conf`, `check.py` (traffic probe across the restart)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm ext-1 carrier + the fib-kernel `RTPROT_ZE`/`sweepDelay` retention exist |
| 3. Wiring phase | Wiring Test table -- consumer registration + failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- the Opaque-Type-3 consumer registration + failing wiring tests
   - Tests: `TestGraceLSAConsumerRegistered`, `test/ospf/ospf-gr-register.ci`
   - Files: `gr.go` (`RegisterOpaqueConsumer(3, link, OnOriginate, OnReceive)` with stub callbacks), `register.go` (wire the consumer + the `graceful-restart` config skeleton)
   - Verify: the consumer registers and the engine discovers it; origination/reception/state machines are stubs so the deeper tests still fail
2. **Phase: Grace-LSA body** -- build/parse the three TLVs over the ext-1 helpers
   - Tests: `TestGraceLSABodyBuild`, `TestGraceLSATLVRoundTrip`, `TestGraceLSAAgeNotResetOnRetransmit`, `ospf-gr-decode.ci`
   - Files: `gr_lsa.go`
   - Verify: the body round-trips with 4-octet padding; LS age semantics hold; decode renders Opaque Type 3 + TLVs
3. **Phase: Helper state machine** -- receive-side first (lower risk, per the guide ordering)
   - Tests: `TestHelperEntryAllChecksPass`, `TestHelperEntryRejectedPerCheck`, `TestHelperAlreadyHelpingUpdatesGrace`, `TestHelperKeepsAdjacencyAdvertised`, `TestHelperKeepsXAsDR`, `TestHelperExitOnGraceExpiry`, `TestHelperExitOnFlush`, `TestHelperStrictExitOnTopologyChange`, `TestHelperStubAreaExternalDoesNotExit`, `ospf-gr-helper.ci`
   - Files: `gr_helper.go`, `gr.go` (helper map), `instance.go`/`neighbor.go` (helping flag in the topology builder), `lsdb/flooding.go` (content-change signal)
   - Verify: entry/exit checks per §3.1/§3.2; the adjacency to X stays advertised; the stub-area exception holds
4. **Phase: Restarter NVS + in-restart gating** -- the suppression core
   - Tests: `TestRestartFactPersistsAcrossRestart`, `TestStaleRestartFactIgnored`, `TestRestarterSuppressesSelfLSAs`, `TestRestarterRunsSPFNoInstall`
   - Files: `gr_nvs.go`, `gr_restarter.go`, `instance.go` (gate `originateSelfLSAs`), `spf/install.go` (gate `Apply`, guard `RemoveAll`), `lsdb/origination.go` (skip own-self-LSA flush)
   - Verify: the restart-fact survives a restart; in-restart suppresses origination + install; SPF still computes; FIB not touched
5. **Phase: Restarter DR re-election + exit triggers + exit actions**
   - Tests: `TestRestarterReElectsSelfDR`, `TestRestarterExitAllAdjacencies`, `TestRestarterExitInconsistentLSA`, `TestRestarterExitGraceExpiry`, `TestRestarterExitActions`, `ospf-gr-prepare.ci`
   - Files: `gr_restarter.go`, `instance.go` (the `prepare` orchestration + graceful stop), `lsdb/origination.go` (`FlushStaleSelfLSAs` on exit)
   - Verify: DR continuity; the three exit triggers; the exit re-originates LSAs, re-installs routes, flushes stale self-LSAs + own Grace-LSAs
6. **Phase: Unplanned outage (config-gated)** -- §5
   - Tests: `TestUnplannedDisabledByDefault`, `TestUnplannedGraceBeforeHello`, `TestGRDisabledNoGraceLSA`
   - Files: `gr_restarter.go`, `config.go`
   - Verify: unplanned off by default; when on, Grace-LSAs before Hello, reason 0/3 only
7. **Phase: Config + CLI + metrics + doctor** -- user surface
   - Tests: `TestGracePeriodRangeRejectsAbove1800`, `TestGRShowState`, `ospf-gr-register.ci`, `ospf-gr-show.ci`, `ospf-gr-disabled.ci`
   - Files: `yang/ze-ospf-conf.yang`, `yang/ze-ospf-cmd.yang`, `config.go`, `gr_show.go`, `cmd_show.go`, `doctor.go`, metric registration in `register.go`
   - Verify: the `graceful-restart` container, the show command, the five metric series, the NVS-path doctor check
8. **Functional tests** -> the six `.ci` cover the user-visible behaviour
9. **RFC refs** -> add `// RFC 3623 Section X` comments on the Grace-LSA build, the in-restart suppression, the §2.2 exit triggers, the §3.1 entry checks, the §3.2 exit, and the §5 unplanned path
10. **Interop** -> `ospf-gr-frr` + `ospf-gr-fib-retention` QEMU scenarios
11. **Full verification** -> `make ze-verify`
12. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; GR parity with FRR (restarter + helper, planned + helper-strict-checking); FIB retention proven end-to-end |
| Correctness | LS age = grace clock (never reset); the three TLVs + padding; §3.1 entry checks all enforced; §3.2 exit incl. stub-area exception; restarter suppresses ALL self-LSA types; `RemoveAll` not called on graceful stop; grace window vs `sweepDelay` reconciled |
| Naming | `ze_ospf_gr_*` metrics; YANG `graceful-restart` / `restart-interval` kebab-case; `OnOriginate`/`OnReceive` per ext-1 |
| Data flow | GR is an ext-1 consumer (no new flooding); fib-kernel read-only; the in-restart flag gates origination + install only |
| CLI grammar | `show ip ospf graceful-restart` action-before-identifier |
| Doctor checks | the GR NVS blob path has a `ze doctor` check per `ai/rules/doctor-checks.md` |
| YANG validation | `restart-interval` range 1-1800; `support`/`helper` enumerations/booleans; no bare `type string` |
| Prometheus counters | the five `ze_ospf_gr_*` series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | removing the GR consumer removes Opaque-Type-3 registration, config, command, doctor, metrics; no GR spelling in ext-1 or generic OSPF packages |
| Rule: buffer-first | the Grace-LSA body is built via the ext-1 TLV builder into a caller buffer; parse is a zero-copy iterator |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Opaque-Type-3 consumer registered | `grep -rn 'RegisterOpaqueConsumer' internal/plugins/ospf/gr.go` |
| Grace-LSA body build/parse | `go test ./internal/plugins/ospf -run 'GraceLSA'` |
| Restarter suppression + exit | `go test ./internal/plugins/ospf -run 'Restarter'` |
| Helper entry/exit | `go test ./internal/plugins/ospf -run 'Helper'` |
| NVS restart-fact | `go test ./internal/plugins/ospf -run 'RestartFact'` |
| Five metric series | `grep -rn 'ze_ospf_gr_' internal/plugins/ospf` |
| FIB-retention interop | `ls test/interop/scenarios/ospf-gr-fib-retention/` |
| GR interop | `ls test/interop/scenarios/ospf-gr-frr/` |
| Functional tests | `ls test/ospf/ospf-gr-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the Grace-LSA TLV parse is bound-checked (ext-1 iterator never panics); a missing/short mandatory TLV makes the Grace-LSA malformed and ignored, not a crash |
| Spoofed Grace-LSA | a forged Grace-LSA can hold a withdrawn router adjacent (R-6); the helper still requires a prior Full adjacency with X; Grace-LSAs ride the existing OSPF cryptographic auth (ospf-12); document the residual risk |
| Resource exhaustion | the helper map is bounded by the neighbour count; a flood of Grace-LSAs cannot grow memory unbounded; ext-1 acceptance rate limit (>= 1 s) applies |
| FIB safety | unplanned support is config-gated (default off) because a crashed router cannot guarantee FIB sanity (§5, R-8); the planned path ensures the FIB is current before reload |
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
Graceful Restart is two control-plane state machines layered on a single new wire
object (the Opaque-Type-3 Grace-LSA, owned by ext-1) and an EXISTING FIB-retention
substrate (the fib-kernel `RTPROT_ZE` stale-mark-then-sweep). The hard part is not
the wire and not the FIB; it is the disciplined suppression: while in restart the
router must touch nothing it normally re-originates (`originateSelfLSAs`) or
re-installs (`Installer.Apply`), and while helping it must keep advertising an
adjacency the NSM might otherwise tear down. Both reduce to a single per-engine /
per-neighbour flag read at the existing origination and topology-build chokepoints.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build on the ext-1 opaque carrier (consumer of Opaque Type 3) | re-implement link-local Grace-LSA flooding inside the GR module | RFC 3623 §A defines the Grace-LSA AS an Opaque Type 3 LSA; ext-1 already owns the carrier; plugin-self-containment + no duplicated flooding |
| Reuse the fib-kernel `RTPROT_ZE` + `sweepDelay` for FIB retention | a new GR-specific FIB-freeze mechanism | the stale-mark-then-sweep already keeps kernel routes across a process restart; GR's job is only to not `RemoveAll` and to re-install before the sweep deadline |
| In-restart is a per-engine flag gating `originateSelfLSAs` + `Installer.Apply` | a parallel "restart-mode" engine | the two chokepoints are the only places self-LSAs are originated and routes installed; a flag is the minimal, auditable suppression |
| Helper keeps X's link via a per-neighbour flag in the topology builder, NOT by freezing the NSM | freeze the neighbor FSM at Full | RFC 3623 §3 freezes the ADVERTISED view, not the NSM; freezing the NSM would break re-sync; the topology builder is the right seam |
| Restart-fact in a ZeFS blob (sibling of the boot-count blob) | a new NVS subsystem; in-memory only | `auth_keystore.go` already persists a blob across restarts via `pkg/zefs`; planned-restart survival needs exactly that pattern |
| Build helper first, then restarter (phase order, not scope cut) | restarter first | the guide notes helper is strictly receive-side and lower risk; both ship in this spec |
| Unplanned outage config-gated, default off | always-on unplanned recovery | §5 requires the operator be able to disable it; a crashed router cannot guarantee FIB sanity |

## Known Limitations
- OSPFv3 Graceful Restart (RFC 5187, native Grace-LSA) is out of scope (ospfv3-ext-4).
- Per-segment partial helping is not offered (the partial-segment pitfall, §3.1); helper support is all-or-nothing per the configured policy.
- Virtual-link GR is constrained by the v1 "no virtual links" limitation; the restarter runs SPF for virtual-link restore but the umbrella defers virtual links.
- The grace window must complete within (or be reconciled with) the fib-kernel `sweepDelay`; a grace period far larger than the sweep delay requires a coordinated sweep-delay extension, documented as a deployment constraint.
- A forged Grace-LSA can hold a withdrawn router adjacent; mitigated by OSPF cryptographic auth (ospf-12) and the prior-Full-adjacency requirement, not eliminated (§Security).

## RFC Documentation

Add `// RFC 3623 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §A Grace-LSA = LS type 9 / Opaque Type 3 / Opaque ID 0; the three TLVs; LS age 0 at origination, never reset, DoNotAge never set
- §2 in-restart suppression (no self-LSA types 1-5/7, no FIB install, keep received self-LSAs); DR re-election
- §2.1 FIB current before reload; grace period <= LSRefreshTime
- §2.2 the three exit triggers
- §2.3 the exit actions
- §3 / §3.1 helper entry checks; keep X's link advertised; keep X as DR
- §3.2 helper exit triggers incl. strict checking + the stub-area exception
- §5 the unplanned-outage path (Grace-LSA before Hello, reason 0/3, operator opt-in)
- RFC 5250 §3.1 the Type 9 "discard if received off the target interface" scope rule (enforced by the ext-1 carrier this consumer relies on)

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
| Restarter preserves the FIB across the control-plane restart | interop (QEMU) | `ospf-gr-fib-retention` (routes stay programmed; forwarding continues) |
| Restarter floods Grace-LSAs + re-acquires adjacencies without flapping routes | functional + interop | `ospf-gr-prepare.ci`, `ospf-gr-frr` (Ze restarter) |
| Helper holds the adjacency Full and suppresses LSDB churn for the grace period | unit + interop | `TestHelperKeepsAdjacencyAdvertised`, `ospf-gr-frr` (Ze helper) |
| Grace-LSA = Opaque Type 3 link-local with the three TLVs | unit + interop | `TestGraceLSATLVRoundTrip`, `ospf-gr-frr` |
| Helper exits on topology change / timer; restarter exits on the three triggers | unit | `TestHelperStrictExitOnTopologyChange`, `TestRestarterExit*` |
| GR disabled is fully backward compatible | unit + suite | `TestGRDisabledNoGraceLSA` + existing OSPF suite green |

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
- [ ] AC-1..AC-21 all demonstrated
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
- [ ] RFC 3623 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (both roles needed; the helper map / in-restart flag are minimal)
- [ ] No speculative features (only RFC 3623 restarter + helper; OSPFv3 GR excluded)
- [ ] Single responsibility per component (body / restarter / helper / NVS / show separated)
- [ ] Explicit > implicit behavior (GR off by default; unplanned opt-in)
- [ ] Minimal coupling (ext-1 consumer; fib-kernel read-only)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-gr-frr`, `ospf-gr-fib-retention`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-9-graceful-restart.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-9-graceful-restart.md`
