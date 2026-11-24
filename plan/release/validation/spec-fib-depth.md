# Spec: fib-depth -- Routing Decision and FIB Programming Depth

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P2 |
| Release reason | Rich-route implementation remains subject to functional, interop, cost-change and backend-parity proof. |
| Depends | - |
| Phase | 13/13 |
| Updated | 2026-11-24 |

Set aside 2026-10-09 (owner: "fib depth can be picked when we are back to two specs in dev"): pick it up when only two specs are in development. Depends no longer names `spec-vrf-0-umbrella.md`, because the 2026-10-08 owner decision moved AC-4, AC-8, AC-9 and the TableID producer to `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md` and no remaining AC needs a VRF table. Next: the functional and interop evidence for AC-1, AC-2, AC-3, AC-5, AC-6, AC-7 and AC-10 to AC-15, notably AC-13 cost-change reselection and AC-15 kernel/VPP backend parity.

The July 22 review recorded SRv6 (learned 1113), ECMP (learned 774) and VPP
parity (learned 798) as delivered while the header still said `7/12`.
The current producers also carry the IGP-cost lookup and comparison, so the
old claim that `lookupIGPCost` always returns zero is obsolete. The phase
accounting below separates code that exists from requirements and evidence
still owed; no fresh test or closure result is claimed.


<!-- Note added 2026-09-07 by spec-connected-static-reach-the-locrib: this spec's
`ECMPPath.Weight` item is SATISFIED. The field landed here and `ecmpCollect`
hardcoded 1; it is now populated from the producer through
`nexthop.NextHop.Weight`, and `ecmpWeight` keeps 1 for a producer that states
none, so nothing an unweighted BGP, OSPF or IS-IS group programs changed.
`ECMPPath` also gained `Interface`. `BestChangeEntry.TableID` is untouched and
belongs, since 2026-10-08, to `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md`,
which is why a named-table static route is kept out of the Loc-RIB. -->

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/plugins/fib/kernel/fibkernel.go` -- current kernel FIB plugin
4. `internal/plugins/fib/kernel/backend_linux.go` -- current netlink backend
5. `internal/plugins/fib/vpp/fibvpp.go` -- current VPP FIB plugin
6. `internal/plugins/fib/vpp/backend.go` -- current VPP backend
7. `internal/component/bgp/plugins/rib/bestpath.go` -- best-path decision
8. `internal/plugins/sysrib/events/events.go` -- BestChangeEntry struct
9. `internal/core/rib/locrib/candidate.go` -- Loc-RIB Path struct

## Task

Finish the remaining routing and FIB requirements while preserving the
implemented rich-route path. Both backends already accept more than
prefix/gateway tuples, and the BGP best-path comparison already reads IGP
cost through the shared resolver seam.

Two original requirements differed from the current implementation: AC-4
required Linux nexthop objects, where `buildRichRoute` emits per-route
`MultiPath`, and AC-8 required VPP metric mapping, where `richRouteAddDel`
uses explicit next-hop weights. Both, with the end-to-end VRF/table production
they need (`fibChange` does not populate `BestChangeEntry.TableID`, although
both backends can consume a supplied value), now belong to
`plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md`, which depends on
`plan/post-release/features/spec-vrf-0-umbrella.md`. All other ACs retain their functional and
interop evidence obligations, including cost-change reselection under AC-13
and backend parity under AC-15.

-> Decision (owner, 2026-10-08): AC-4 (Linux nexthop objects) and AC-8 (VPP metric), plus the TableID production they need, move to a new spec, `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md`, which depends on `plan/post-release/features/spec-vrf-0-umbrella.md`. They are removed from this spec's acceptance criteria.

### Competitive Context

| Daemon | Recursive NH | ECMP | Nexthop Groups | VRF/table | Route type | Metric | MPLS push | SRv6 |
|--------|-------------|------|----------------|-----------|------------|--------|-----------|------|
| FRR | yes (zebra) | yes (nhg) | yes (kernel nhg) | yes | yes | yes | yes | yes |
| BIRD | yes (recursive) | yes (merge paths) | no (expanded) | yes (tables) | yes | yes | yes | partial |
| Junos | yes | yes | yes (nhg) | yes | yes | yes | yes | yes |
| Nokia SR OS | yes | yes | yes (nhg) | yes | yes | yes | yes | yes |
| Arista EOS | yes | yes | yes (nhg) | yes | yes | yes | yes | yes |
| Ze (current source) | yes | yes (multipath) | no (AC-4, moved to `spec-fib-nexthop-objects-vpp-metric.md`) | backend input supported; sysrib TableID absent | yes | kernel Priority; VPP AC-8 open | yes | yes |

### Design Decisions (proposed, pending approval)

| Decision | Detail | Rationale |
|----------|--------|-----------|
| Recursive NH tracker lives in sysrib | sysrib already resolves cross-protocol best. NH resolution is a second phase after prefix best-path. Keeps RIB plugins (bgp-rib) unchanged | Same pattern as FRR zebra NHT / BIRD recursive |
| BestChangeEntry gains rich fields | Add RouteType, Metric, Weight, TableID, Labels (already present), SRv6SID, ECMPGroup fields | FIB backends need this data; wire it through the event, not via side-channel lookups |
| ECMP via nexthop groups | Multiple equal-cost paths publish as a single change with an ECMPGroup containing []NextHop rather than N separate changes | Matches Linux kernel nexthop group API (5.3+), VPP multi-path FibPath, and avoids transient single-path states |
| IGP cost comes from Loc-RIB metric | bgp-rib queries sysrib for the IGP metric of the resolved next-hop prefix. No OSPF/IS-IS internal coupling | Loc-RIB Path.Metric is exactly this: the IGP cost for internal next-hops |
| Route types: unicast, blackhole, unreachable, prohibit | Static plugin already models blackhole/unreachable. Extend to FIB event so backends handle it | Linux RTN_BLACKHOLE/RTN_UNREACHABLE/RTN_PROHIBIT, VPP drop/unreach adjacencies |
| Consistent Linux/VPP semantics | Both backends must produce identical forwarding behavior for the same BestChangeEntry | Test via le test functional comparisons |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `fixture-encodes-an-impossible-state.md` | 2026-10-31 | `connectedDistanceWinner02` injects a next hop the kernel cannot install (fixed: 26da0eef6e, journal closed in d7b434442b) |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-11 | fib-vpp consumed no `(mpls-fib, entry)` event, so LDP and RSVP-TE swap and pop entries never reached VPP (fixed in Phase 13: 01250d9b63) |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-11 | `show mpls forwarding` read only the kernel AF_MPLS table, so on VPP it answered empty (fixed: 9318a687d8, the reader registry in `internal/core/mplsfib/forwarding.go`) |
| `zero-value-as-valid-answer.md` | 2026-11-11 | static routes on VPP send `SwIfIndex` 0 (`local0`) for a next-hop-address path; Phase 13 fixes it as it fixed fib-vpp `toFibPath` |
| `announced-state-never-replayed.md` | 2026-11-11 | OSPF-SR label entries used a bare `Emit`, waited for no acknowledgement and answered no `(mpls-fib, replay-request)` (fixed in Phase 13) |
| `announced-state-never-replayed.md` | 2026-11-11 | BGP EPE peer-SID pops answered no `(mpls-fib, replay-request)` (fixed in Phase 13) |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-11 | fib-vpp swept no stale IP routes or labeled pushes at writer open, as fib-kernel's `startupSweep` does (fixed: b1d5f8aabf, FIB source `ze-fib-vpp` and the stale IP route sweep; real VPP v26.06 run 2026-11-12 proves the source and install/withdraw, the removal of an actual stale route is unit-proven only) |
| `config-accepts-a-value-that-breaks-an-rfc-must.md` | 2026-11-11 | `net.mpls.ip_ttl_propagate` 0 accepted by the sysctl registry, so pushed labels carry `net.mpls.default_ttl`, against RFC 3032 Section 2.4.3 (fixed: the key's `KeyDef.Constraint` refuses 0; `TestBuildRichRoutePushCarriesNoTTL` proves the push carries no tunnel TTL) |
| `config-accepts-a-value-that-breaks-an-rfc-must.md` | 2026-11-11 | kernel `net.mpls.ip_ttl_propagate` 0 written outside Ze, which no doctor check reported (fixed: 5d3d7e3a6d, fib-kernel check `mpls-ttl-propagate`) |
| `suite-shares-one-persistent-store.md` | 2026-11-11 | `./le test deployment vpp` traffic scenario started its daemon while the previous one still held the store (fixed: ab87047c60, a VPP scenario stop waits for its daemon to exit; the 2026-11-12 real-VPP run continued past the traffic scenario's restart) |
| `plugin-list-hardcoded.md` | 2026-09-07 | OSPF and IS-IS `Dependencies` named `fib-kernel` (Phase 13 AC-16; also listed by `spec-one-owner-per-domain-fact.md`) |
| `plugin-list-hardcoded.md` | 2026-10-20 | LDP `Dependencies` named `fib-kernel` (Phase 13 AC-16; also listed by `spec-one-owner-per-domain-fact.md`) |
| `plugin-list-hardcoded.md` | 2026-11-11 | geodns, dnsresolver and as112 doctor checks named `fib-kernel` in `DoctorCheckDef.Dependencies` (fixed: f097ae5e30, `TestNoDoctorCheckDependsOnAFIBWriter`) |
| `read-fails-on-a-fact-the-caller-does-not-need.md` | 2026-11-12 | fib-vpp `processEvent` calling `restoreSRv6`: with no persistent store fib-vpp programs no IP route at all (fixed: 4e3e704cff, `TestProcessEventProgramsIPAndMPLSRoutesWithoutStore`, 8/8 `test vpp`) |
| `kernel-refuses-what-the-installer-sends.md` | 2026-11-13 | fib-kernel sent fast-reroute backups as RTNH_F_LINKDOWN multipath members, refused by Linux; no backup takes over on primary failure, fib-vpp ignores `Backup`, `ze_ospf_fast_reroute_backups_installed` counts SPF not FIB installs (ownership moved 2026-11-24 to `plan/release/fixes/spec-fib-lfa-backup-takeover.md`) |
| `record-written-before-the-operation-succeeds.md` | 2026-11-13 | fib-kernel never re-attempts a route the kernel refused, while sysrib holds it as installed; `fib-sync-failure` is never cleared (fixed: 20d642984c, `pending_retry.go`, `fibkernel_journal_fixes_test.go`) |
| `counter-counts-the-wrong-packets.md` | 2026-11-14 | fib-kernel's startup sweep counts a delete the kernel answered "absent" (ESRCH) as a removed orphan (fixed: 20d642984c, `delOwnedRoute` bool return) |
| `rollback-forgets-partial-apply.md` | 2026-11-14 | fib-kernel `replaceOwnedRoute` adds the new route, fails to delete a superseded one, and the new route is not recorded as installed (fixed: 20d642984c, `supersededDeleteError` + `supersededTables`) |
| `key-omits-a-fact-the-builder-uses.md` | 2026-11-14 | fib-kernel records one table per prefix, so an update moving a prefix to another table leaves the old table's route behind (fixed: 20d642984c, table-aware `ownsSlotLocked` + old-table delete) |
| `silent-fall-through.md` | 2026-11-14 | fib-kernel on a backend without the rich interface ignores the table of an add, replace or delete (fixed: 20d642984c, nonzero `TableID` refused when `rb == nil`) |

## Required Reading

### Architecture Docs
- [ ] `docs/research/vpp-deployment-reference.md` - the VPP deployment reference drawn from 83 ipng.ch articles
- [ ] `docs/architecture/core-design.md` -- FIB plugin architecture
  → Constraint: FIB plugins subscribe to sysrib events; they do not poll
- [ ] `docs/architecture/plugin/rib-storage-design.md` -- RIB best-path
  → Decision: best-path runs in bgp-rib plugin; sysrib arbitrates across protocols

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- BGP Decision Process Section 9.1.2.2 (step 6: IGP cost)
  → Constraint: IGP cost comparison applies only between iBGP paths after eBGP-over-iBGP
- [ ] `rfc/short/rfc7911.md` -- ADD-PATH (multiple paths per prefix)
  → Constraint: ECMP selection happens post-ADD-PATH receive, not before
- [ ] `rfc/short/rfc8277.md` -- MPLS label encoding in BGP
  → Constraint: Label stack in BestChangeEntry must preserve order
- [ ] `rfc/full/rfc3031.txt` Section 3.10 (NHLFE) and Section 3.13 (Label Swapping) -- Phase 13
  → Constraint: Section 3.10 names the NHLFE operations "replace the label at the top of the label stack with a specified new label" and "pop the label stack"; Section 3.13: "It uses the ILM to map this label to an NHLFE." The next hop comes from the NHLFE, never from an IP lookup of the payload
- [ ] `rfc/full/rfc3032.txt` Sections 2.1, 2.2, 2.4.2, 2.4.3 -- Phase 13
  → Constraint: Section 2.1, the S bit: "This bit is set to one for the last entry in the label stack (i.e., for the bottom of the stack), and zero for all other label stack entries." One incoming label is met both at the bottom of the stack and above it, and VPP keys its MPLS FIB on (label, S), so one ze entry is two VPP entries
  → Constraint: Section 2.2: "The LSR which pops the last label off the stack must therefore be able to identify the packet's network layer protocol."
  → Constraint: Section 2.1, label 3: "When an LSR would otherwise replace the label at the top of the stack with a new label, but the new label is "Implicit NULL", the LSR will pop the stack instead of doing the replacement."
  → Constraint: Section 2.4.2: "When a labeled packet is forwarded, the TTL field of the label stack entry at the top of the label stack MUST be set to the outgoing TTL value." Section 2.4.3: "When an IP packet is first labeled, the TTL field of the label stack entry MUST BE set to the value of the IP TTL field."

**Key insights:**
- IGP cost comparison (step 6) needs a next-hop resolution table (NH -> IGP metric)
- ECMP is orthogonal to best-path: select N equal-cost bests, group them
- FIB programming is a projection of the resolved RIB state, not a 1:1 copy

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` -- `lookupIGPCost` calls `igpcost.Lookup`; the comparator uses `IGPCost` at step 6
- [ ] `internal/component/sysrib/events/events.go` -- `BestChangeEntry` already carries route type, metric, TableID, labels, SRv6 SID and weighted ECMP paths
- [ ] `internal/component/sysrib/nhresolver.go` -- `Resolve` walks the Loc-RIB; `IGPMetric` returns the accumulated metric
- [ ] `internal/component/sysrib/sysrib.go` -- `SetLocRIB` registers `resolver.IGPMetric` with `igpcost`; `ecmpCollectResolved` filters and weights members
- [ ] `internal/component/sysrib/fibimport.go` -- `fibChange` emits the rich fields but leaves TableID zero
- [ ] `internal/plugins/fib/kernel/nexthop_linux.go` -- `buildRichRoute` sets type, metric and supplied table; emits `MultiPath`, MPLS and SEG6 encapsulation
- [ ] `internal/plugins/fib/vpp/backend.go` -- `richRouteAddDel` builds weighted `FibPath` entries and honors a supplied TableID; it does not read `r.Metric`
- [ ] `internal/plugins/fib/vpp/srv6.go` -- `processSRv6Change` programs and removes SRv6 steering
- [ ] `docs/architecture/fib/fib-depth-4-srv6.md` -- the recorded SRv6 implementation and Prefix-SID input path

**Source files read for Phase 13 (label-switching entries on VPP), 2026-11-11:**
- [ ] `internal/core/mplsfib/events.go` -- `Entry` (Action, Op, InLabel, FEC, TableID, OutLabels, NextHop, PathMTU, Source), `EntryBatch` with an in-process `Acknowledge` callback (`json:"-"`); `Apply` returns `errNoOwnerAcknowledgement` when no subscriber called it; `RemoveLabelSource`
  → Constraint: the acknowledgement is an in-process callback, so a label writer must run in the daemon's process; both FIB writers do (fib-vpp holds a `*vppcomp.Connector`)
- [ ] `internal/plugins/fib/kernel/mplsentry.go` -- `handleMPLSEntry` acks every batch, refuses a nonzero TableID on swap/pop and a private table outside the bypass namespace, validates the 20-bit in-label and the out stack, keeps `mplsSwaps` (in-label to Source) and refuses another source's label with EEXIST; `delMPLSSourceLocked` retires one source's labels; `fibkernel.go` `run` subscribes it beside the best-change handler
- [ ] `internal/plugins/fib/kernel/mplsentry_linux.go` `addMPLSSwap` -- a swap carries `NewDst` with the whole out stack and a pop carries none; a valid next hop is `RTA_VIA` in the next hop's own family; a pop without a next hop goes out loopback, which re-injects the payload into IP lookup
- [ ] `internal/plugins/fib/vpp/mpls.go` -- `govppMPLSBackend` already calls `IPRouteAddDel` (push), `MplsRouteAddDel` (`addMPLSSwap`, `delMPLSSwap`) and `SwInterfaceSetMplsEnable` (`enableMPLS`, `disableMPLS`); only the push has a caller. `addMPLSSwap` takes ONE out-label, programs only the S=1 entry (`MrEos` 1), sets no `MrEosProto`, uses `fib.vpp.table-id` as the MPLS table, and imposes TTL 64 in pipe mode (`IsUniform` 0); `addMPLSRoute` (push) imposes the same fixed TTL. Nothing calls `MplsTableAddDel`
- [ ] `internal/plugins/fib/vpp/register.go` -- `ProgramsFIB` true, `DataPlane` "vpp"; `subscribeFibVPP` subscribes best changes only; `restart` opens a writer at start and on `vpp connected` / `vpp reconnected`, and holds none while VPP is unreachable
- [ ] `internal/plugins/fib/vpp/fibvpp.go` `processMPLSChange` -- the labeled best-change (push) path, the only MPLS path fib-vpp runs today
- [ ] `internal/plugins/fib/vpp/apply_test.go` `testChannel` -- the fake `api.Channel` seam: it records the last request and sets `Retval` only on an `IPRouteAddDelReply`
- [ ] `internal/plugins/ldp/register.go`, `internal/plugins/rsvpte/register.go` -- `Dependencies` name "fib-kernel" (LDP line 201, RSVP-TE line 465) and so do their doctor checks (`ldp-port` line 217, `rsvp-te-rawsock` line 481). `DoctorCheckDef.Dependencies` is validated non-empty (`internal/component/plugin/registry/doctor.go` `validateDoctorCheckDef`) and read nowhere else
- [ ] `internal/plugins/ldp/fib.go` -- `emit` publishes with `EntryChange.Emit` and no acknowledgement, so a refused or unconsumed entry is lost without a log line; `OnStarted` programs an egress pop with no next hop for every local FEC before any session exists
- [ ] `internal/plugins/rsvpte/fib.go` -- `busFIB.emit` uses `mplsfib.Apply`; a swap carries one label, `programBackup` a stack; a push may carry a private `TableID` (`bypassTableID`, facility-backup control contexts)
- [ ] `internal/component/plugin/server/startup_autoload.go` `appendDataPlaneWriter`, `fibPluginChosen`; `internal/component/plugin/registry/registry.go` `ProgramsFIB`, `DataPlane`, `NeedsDataPlane`, `PluginForDataPlane`
- [ ] `internal/component/iface/config_sysctl.go` -- `mpls { enable }` becomes only `net.mpls.conf.<os-name>.input`; `internal/component/iface/backend.go` `ReplaceIPv4RPF` is the whole-set precedent for a leaf a data plane outside the kernel enforces itself (`internal/plugins/iface/vpp/rpf.go`, `internal/plugins/iface/netlink/rpf_linux.go`)
- [ ] `internal/core/replay/replay.go` -- the shared replay token vocabulary: 0 incremental, `Broadcast`, or a targeted token the emitter maps back to one consumer
- [ ] `internal/le/test/deployment/vppevidencerun.go` `runFIB` -- the `mpls-fib` scenario checks a labeled IP route in `show ip fib`; nothing reads `show mpls fib`

**Behavior to preserve:**
- Single-path route installation still works (ECMP is additive)
- Existing BestChangeEntry JSON format (add new fields with omitempty)
- fib-kernel rtprotZE identification (stale-mark-then-sweep)
- fib-vpp tableID from config remains the fallback when a change carries no override
- MPLS labels on best changes (push) already reach both backends; label-switching entries (swap, pop) reach fib-kernel only (Existing Mechanisms)
- Existing route-type handling, including blackhole, unreachable and prohibit
- Metrics: route install/update/remove counters unchanged

**Behavior to change:**
- TableID production, AC-4 (Linux nexthop objects), AC-8 (VPP metric) and AC-9 (table install) moved to `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md` (owner, 2026-10-08).
- Establish the remaining AC evidence against the implemented resolver, cost comparison, ECMP and backend paths, especially AC-13 cost-change reselection.
- Phase 13 (AC-16 to AC-29): label-switching entries reach the writer of the selected data plane. fib-vpp consumes `(mpls-fib, entry)`, LDP and RSVP-TE stop naming fib-kernel and declare `NeedsDataPlane`, `mpls { enable }` is enforced in VPP, and VPP label routes and pushes carry uniform TTL. fib-kernel's label behaviour is preserved, except that it ignores a replay batch addressed to another writer.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|---------------------------------------------------------------|
| System-RIB best change | `internal/component/sysrib/events/events.go` `BestChangeEntry`, published by `internal/component/sysrib/sysrib.go` | Carries route type, metric, TableID, labels, SRv6 SID and weighted ECMP paths to every FIB writer | extends: the rich fields this spec proves are its fields; no second event |
| Recursive next-hop and IGP cost | `internal/component/sysrib/nhresolver.go` `Resolve`, `IGPMetric`; `internal/component/bgp/plugins/rib/bestpath.go` `lookupIGPCost` | Resolves over the Loc-RIB and feeds the comparator's IGP cost step | extends: AC evidence is gathered against it |
| Kernel rich route | `internal/plugins/fib/kernel/nexthop_linux.go` `buildRichRoute`, `backend_linux.go` | Programs type, metric, table, multipath, MPLS push and SEG6 encap | extends: the kernel half of backend parity |
| VPP rich route and SRv6 | `internal/plugins/fib/vpp/backend.go` `richRouteAddDel`, `srv6.go` `processSRv6Change`, `fibvpp.go` `processMPLSChange` | Programs weighted paths, a supplied table, SRv6 steering and labeled (push) routes from best changes | extends: the VPP half of backend parity |
| MPLS label-switching entries | `internal/core/mplsfib/events.go`, consumed by `internal/plugins/fib/kernel/mplsentry.go` only | LDP and RSVP-TE swap and pop entries reach fib-kernel; fib-vpp subscribes only to best changes (`subscribeFibVPP`, `register.go`) | extends (Phase 13): fib-vpp subscribes the same `EntryChange` handle and acknowledges through the same callback; no second event, no VPP-specific payload (owned row `guard-added-to-one-half-of-a-pair.md`, 2026-11-11) |
| Owner acknowledgement | `internal/core/mplsfib/events.go` `Apply`, `errNoOwnerAcknowledgement`; `mplsentry.go` `handleMPLSEntry` | Apply fails when no in-process owner acknowledged; every running owner acks and `errors.Join` keeps any refusal | extends: fib-vpp acks every batch, with an error while it holds no writer, so a producer never advertises a label VPP does not hold. LDP moves from a bare `Emit` to `Apply` so a refusal is logged rather than lost |
| Kernel label consumer semantics | `mplsentry.go`, `mplsentry_linux.go`, `mplscontext*.go`, `internal/core/mplsfib/pathmtu.go` | Validation, source-owned in-label map, swap stack, pop via next hop or loopback, private push contexts, transit MTU by probe | extends: fib-vpp matches each rule (Data Flow, Phase 13); the private push context is kernel-only and is refused by fib-vpp (Work Not Done) |
| VPP MPLS backend | `internal/plugins/fib/vpp/mpls.go` `govppMPLSBackend` | Push wired; swap, pop and interface enable written but unwired, swap single-label and S=1 only | extends: the swap and pop methods are reshaped (out stack, both S entries, payload protocol, MPLS table 0, uniform TTL) and wired; nothing is added beside them |
| One writer per data plane | `internal/component/plugin/registry/registry.go` `ProgramsFIB`, `DataPlane`, `NeedsDataPlane`; `startup_autoload.go` `appendDataPlaneWriter` | Loads the writer for the configured data plane for a producer that declares `NeedsDataPlane` (static, EPE) | extends: LDP and RSVP-TE declare `NeedsDataPlane` and drop "fib-kernel" from `Dependencies`; the writer of the data plane owns all of its forwarding state, labels included, so no second declaration is added |
| Late-join replay | `internal/core/replay/replay.go`; `sysribevents.ReplayRequest`; the redistribute orchestrator's targeted tokens | A FIB writer that starts late asks sysrib to replay; a targeted token routes the answer to one consumer | extends: `mplsfib` gains its own replay-request handle bound to `*replay.Request`; a label writer that opens asks with a targeted token and the producers re-apply their wanted entries tagged with it |
| Data-plane enforcement of an iface leaf | `internal/component/iface/backend.go` `ReplaceIPv4RPF`; `iface/vpp/rpf.go`; `iface/netlink/rpf_linux.go` | Whole-set call per apply; netlink accepts and installs nothing because the sysctl covers it, VPP enforces itself | extends: the same shape carries `mpls { enable }` (VPP `sw_interface_set_mpls_enable`); the sysctl write stays for the kernel |
| VPP deployment proof | `internal/le/test/deployment/vppevidencerun.go` `runFIB`, scenario `mpls-fib` | Real VPP in Docker; asserts a labeled IP route | extends: a new scenario beside it, same harness |
| Doctor label-writer check | `plan/release/validation/spec-doctor-coverage-10-forwarding.md` AC-108 (`doctor-fib-mpls-no-writer`), AC-83 | Planned, not built | does not apply here: that child builds the check; this phase makes its fix command true (edit owed there, Work Not Done) |

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE received, parsed, stored in bgp-rib
- bgp-rib runs best-path selection (step 6 now queries IGP cost)
- Winner published as (rib, best-change) to EventBus

### Transformation Path
1. **bgp-rib best-path** -- RFC 4271 decision process including step 6 IGP cost
2. **sysrib arbitration** -- cross-protocol best by AdminDistance, emits (system-rib, best-change)
3. **sysrib NH resolution** -- resolves recursive next-hops to directly-connected next-hops
4. **sysrib ECMP grouping** -- gathers equal-cost paths into ECMPPaths[]
5. **BestChangeEntry enrichment** -- adds RouteType, Metric, Weight, TableID, SRv6SID
6. **FIB backend programming** -- kernel netlink or VPP binary API

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| bgp-rib → sysrib | EventBus typed handle (rib, best-change) | [ ] |
| sysrib → fib-kernel/vpp | EventBus typed handle (system-rib, best-change) | [ ] |
| sysrib → bgp-rib (IGP cost query) | EventBus request-reply or direct lookup | [ ] |
| fib-kernel → Linux kernel | netlink RTM_NEWROUTE/RTM_NEWNEXTHOP | [ ] |
| fib-vpp → VPP | GoVPP IPRouteAddDel/MplsRouteAddDel | [ ] |

### Integration Points
- `sysribevents.BestChangeEntry` -- extended struct, all FIB consumers adapt
- `locrib.Path` -- NH resolution uses Loc-RIB to find IGP metric for a next-hop
- `bestpath.comparePair` -- step 6 calls into NH resolution table
- `netlinkBackend` -- replaces string-based API with structured route object
- `govppBackend` -- multi-path and route-type support

### Architectural Verification
- [ ] No bypassed layers (data flows through sysrib, not direct bgp-rib → FIB)
- [ ] No unintended coupling (bgp-rib does not import fib packages)
- [ ] No duplicated functionality (ECMP grouping in sysrib only, not per-backend)
- [ ] Zero-copy preserved where applicable (BestChangeEntry is value-typed)

### Phase 13: label-switching entries reach the selected data plane

**Entry points.** An operator config holding `ldp { }` or `rsvp-te { }` with `interface { backend vpp; }` and no `fib { }` block. A labeled frame arriving on a VPP interface whose `mpls { enable true; }` is set.

**Selection, and how no plugin is named.** LDP and RSVP-TE declare `Registration.NeedsDataPlane` and drop "fib-kernel" from `Dependencies` (keeping "sysctl", and "interface" for RSVP-TE). `appendDataPlaneWriter` then asks `registry.PluginForDataPlane` for the writer of the data plane `interface { backend }` names, exactly as it does for static routes. The writer DECLARES its data plane (`DataPlane` "netlink" or "vpp"); the producer declares only that it needs one; the engine reads both declarations and names neither plugin. No code, test helper or doc table adds a list of FIB writers, and `fib-vpp` and `fib-kernel` never name LDP or RSVP-TE: they consume a core event any producer may emit. The doctor checks `ldp-port` and `rsvp-te-rawsock` name their own plugin in `DoctorCheckDef.Dependencies` instead of "fib-kernel". Reusing `NeedsDataPlane` rather than declaring a second need is deliberate: the writer of a data plane owns all of its forwarding state, labels included (`docs/architecture/mpls/mpls-kernel.md`), so a label producer's need and a route producer's need resolve to the same plugin. A chosen writer that reaches no data plane (`fib { p4 { } }`) consumes no label entry, so `Apply` fails with `errNoOwnerAcknowledgement` and RSVP-TE refuses its LSP as it does today.

**Transformation path, one entry.**
1. The producer builds an `mplsfib.Entry` and calls `mplsfib.Apply`. LDP stops using a bare `EntryChange.Emit` and logs a refused batch with the entry and the cause.
2. Every in-process subscriber of `(mpls-fib, entry)` receives the batch. fib-vpp's subscription is plugin-lifetime, made in `OnStarted` beside the best-change one, and routes the batch under the lifecycle lock to the current writer.
3. With no writer (VPP unreachable, channel refused) fib-vpp acknowledges with an error that names the cause and `ze doctor`, so `Apply` fails and the producer advertises nothing it does not forward.
4. The writer validates the entry with the same rules as fib-kernel: 20-bit in-label, out stack of 1 to `mplsfib.MaxLabelStack` labels for a swap, empty for a pop, swap/pop at table zero, an in-label another source owns refused with EEXIST. The two copies of the label validation (`fib/kernel/mpls.go`, `fib/vpp/mpls.go` `validateLabels`) become one function in `internal/core/mplsfib`, so the rule is declared once.
5. The writer programs VPP MPLS table 0, the label space every MPLS-enabled interface looks up, never `fib.vpp.table-id` (which is an IP table). Before its first label entry on a connection it creates table 0 with `MplsTableAddDel`; it never deletes it.
6. One entry becomes two VPP routes, one per value of the S bit (RFC 3032 Section 2.1), each with one path toward the entry's next hop, with the next-hop protocol of the next hop's family:

| Entry | VPP route, S=1 (`MrEos` 1) | VPP route, S=0 (`MrEos` 0) |
|-------|-----------------------------|-----------------------------|
| Swap, out stack L1..Ln, next hop N | payload protocol: N's family; path via N imposing L1..Ln | payload protocol MPLS; path via N imposing L1..Ln |
| Pop, next hop N (penultimate hop, Implicit NULL downstream) | payload protocol: N's family; path via N with the single label Implicit NULL (3), which VPP treats as a pop | payload protocol MPLS; path via N with Implicit NULL, so the remaining stack leaves as MPLS |
| Pop, no next hop (egress disposition) | payload protocol: the FEC's family; path is a lookup in the IP table `fib.vpp.table-id` of that family, with Implicit NULL | payload protocol MPLS; path is a lookup in MPLS table 0, with Implicit NULL |
| Pop, no next hop and no FEC | refused: RFC 3032 Section 2.2, the LSR that pops the last label must identify the network layer protocol, and the entry names none | refused with it |

→ Constraint: three VPP behaviours this table relies on are read in VPP's source at the release the lab runs (upstream v26.06) before the code is written, and the function is named in the implementing comment (`ai/META-DECISIONS.md`, read the peer's source): that the route handler `vl_api_mpls_route_add_del_t_handler` (`src/vnet/mpls/mpls_api.c`) takes `mr_eos_proto` as the payload protocol of an S=1 route and MPLS for S=0; that a path label of Implicit NULL imposes nothing and pops (`fib_path_ext`, `src/vnet/fib/fib_path_ext.c`), with `is_uniform` selecting the uniform disposition; and that `sw_interface_set_mpls_enable` and a route in table 0 fail until `mpls_table_add_del` has created table 0. If one of them reads otherwise, the table is corrected in this spec before code, never approximated.
-> Decision (2026-11-11, VPP v26.06): (1) confirmed. (2) Implicit NULL imposes nothing and mpls-lookup pops; the uniform disposition is inserted only for ATTACHED_NEXT_HOP and DEAG paths, so a penultimate pop via a bare next hop (sw_if_index ~0, RECURSIVE) pops without copying the TTL; egress pops are uniform. Every label path is sent with sw_if_index ~0, and toFibPath is corrected from 0 (local0). (3) confirmed; mpls_table_create is idempotent for the API. Addition: sw_interface_set_mpls_enable is reference-counted, so the iface VPP backend enables only what MplsInterfaceDump does not report and disables a stale interface once.
→ Constraint: an egress pop must name its payload family. LDP already sets `FEC` on its pops. RSVP-TE's egress pop (`engine.go`, `programPop(label, netip.Addr{}, ...)`) sets `FEC` to the session's tunnel endpoint host prefix, the IPv4 FEC its LSP carries. The `Entry.FEC` comment in `mplsfib/events.go` changes from "Unused for swap/pop" to name this use; fib-kernel keeps ignoring it.

7. Every label the writer imposes or uses for a pop is uniform mode (`IsUniform` 1), never a fixed TTL, so the top entry carries the outgoing TTL (RFC 3032 Section 2.4.2) and a popped IP packet gets it back. This matches Linux, whose `net.mpls.ip_ttl_propagate` default is on. The existing labeled best-change push (`addMPLSRoute`) moves from fixed TTL 64 to uniform for the same reason (Section 2.4.3).
8. `PathMTU` has no per-route form in VPP. The writer installs the entry without it and logs once, as a warning, that labeled frames are bounded by the interface MTU alone: the behaviour fib-kernel already has on a kernel without the MTU patch (`mpls-kernel.md`, "On a kernel without the patch").
9. A push with a nonzero `TableID` (an RSVP-TE facility-backup control context selected by packet mark) has no VPP form: the writer refuses it with an error naming the context and the reason (Work Not Done). A push at table zero takes the existing `addMPLSRoute` path, an empty stack the plain-route path, as fib-kernel does.
10. On success the writer records in-label to source, acknowledges nil, and counts the install. Remove deletes both VPP routes; `ActionRemoveLabelSource` deletes every label the source owns and keeps each failed deletion owned for the next call, as fib-kernel does. Flush on stop removes the label routes the writer installed.

**Late join and VPP restart.** A writer that opens after the producers started (VPP connects after LDP's `OnStarted`, which programs its egress pops immediately) or after a VPP restart (VPP lost its FIB) holds none of the entries. `mplsfib` gains a `(mpls-fib, replay-request)` handle bound to `*replay.Request` and a `ReplayID` on `EntryBatch` (`json:"-"`), the same vocabulary sysrib's replay uses. The writer, once its subscription exists and table 0 is created, emits a replay request with a fresh targeted token (`mplsfib` hands out tokens from one process-wide counter starting at 1, the redistribute orchestrator's pattern). LDP and RSVP-TE answer by re-applying every entry they currently want installed, tagged with that token. A batch carrying a nonzero token is consumed and acknowledged only by the writer that holds it; every other writer, fib-kernel included, ignores it without acknowledging, so a kernel writer running beside the VPP one never re-installs what it already holds. A producer that has to wait for a writer that is not yet there gets the error of step 3 and retries through its own existing path.
-> Decision (2026-11-11, implementation): the replay producers are LDP, RSVP-TE, OSPF-SR (`srFIB.replay`, entries applied with `mplsfib.Apply`, refusals logged) and BGP EPE (`Source.ReplayLabels`). IS-IS emits no `(mpls-fib, entry)`.
-> Decision (2026-11-11, implementation, being coded): a stale-label sweep at writer open. At writer open fib-vpp dumps MPLS table 0 and removes the label routes no source re-claimed after the targeted replay; a failure of the sweep is reported by the runtime doctor check `fib-vpp-label-entries`.

**Doctor codes (Phase 13).** Phase 13 declares `doctor-fib-mpls-no-writer`, the name `plan/release/validation/spec-doctor-coverage-10-forwarding.md` AC-108 assigns, through the producers' runtime doctor checks `ldp-label-entries` and `rsvp-te-label-entries` (a refused or unreplayed label entry), and `doctor-fib-vpp-mpls-path-mtu` through fib-vpp's runtime check `fib-vpp-label-entries` (an entry whose `PathMTU` VPP cannot apply), and `doctor-fib-vpp-mpls-stale-label` through the same check (a label route a previous run left that the stale-label sweep could not remove or could not look for). OSPF-SR and BGP EPE answer `doctor-fib-mpls-no-writer` through their own runtime checks `ospf-sr-label-entries` and `bgp-epe-label-entries`; every label source keeps its refused entries in the one record `mplsfib.Refusals` (`internal/core/mplsfib/refusal.go`). The code constant lives in `internal/core/mplsfib` (`CodeDoctorNoWriter`), so producers and the writer name one declaration.

**Boundaries crossed.**

| Boundary | How | Verified |
|----------|-----|----------|
| LDP / RSVP-TE to the writer | `(mpls-fib, entry)` typed handle, in-process acknowledgement | [ ] |
| Writer to producers | `(mpls-fib, replay-request)` typed handle, targeted token | [ ] |
| Engine to writer selection | `NeedsDataPlane` and `DataPlane` declarations, `appendDataPlaneWriter` | [ ] |
| fib-vpp to VPP | GoVPP `MplsTableAddDel`, `MplsRouteAddDel`, `IPRouteAddDel` | [ ] |
| iface component to VPP | `iface.Backend` whole-set MPLS input call, GoVPP `SwInterfaceSetMplsEnable` | [ ] |

**MPLS input on a VPP interface.** `mpls { enable }` reaches the kernel only as a sysctl, which judges nothing VPP forwards, the same gap `ReplaceIPv4RPF` closed for `rp-filter`. `iface.Backend` gains a whole-set call carrying every interface whose `mpls { enable }` is true, passed on every apply, never a delta. The netlink backend accepts it and installs nothing, because the sysctl already enables the kernel. The VPP backend creates MPLS table 0 if absent, enables each named interface through the shared name resolver, reads back the enabled set (`MplsInterfaceDump`) and disables every interface the set no longer names. The non-Linux stub refuses a non-empty set. The sysctl write stays.

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE with iBGP recursive NH | → | sysrib NH resolution + IGP cost lookup | `TestRecursiveNHResolution` |
| Two equal-cost iBGP paths | → | sysrib ECMP grouping | `TestECMPGroupEmission` |
| BestChangeEntry with RouteType=blackhole | → | kernel backend RTN_BLACKHOLE | `TestKernelBlackhole` |
| BestChangeEntry with ECMPPaths | → | kernel backend nexthop group | `TestKernelECMPGroup` |
| BestChangeEntry with ECMPPaths | → | VPP backend multi-path | `TestVPPMultiPath` |
| BestChangeEntry with Labels | → | kernel backend MPLS push | `TestKernelMPLSPush` |
| `ldp { }` with `interface { backend vpp }`, no `fib { }` | → | `appendDataPlaneWriter` loads fib-vpp; LDP egress pops reach `MplsRouteAddDel` | `test/vpp/vpp-mpls-ldp-pop.ci` (stub), `./le test deployment vpp` scenario `mpls-label-pop` (real VPP) |
| `(mpls-fib, entry)` swap from RSVP-TE | → | fib-vpp label consumer, both S entries | `TestFibVPPLabelSwapProgramsBothStackPositions` |
| VPP reconnect | → | targeted `(mpls-fib, replay-request)`, producers re-apply | `TestFibVPPLabelReplayOnWriterOpen`, `TestLDPReplayReappliesWantedEntries`, `TestRSVPTEReplayReappliesInstalledEntries` |
| `mpls { enable true }` under `backend vpp` | → | iface VPP backend `SwInterfaceSetMplsEnable` | `TestVPPReplaceMPLSInput` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two iBGP paths to same prefix, different IGP cost to NH | Best-path selects lower IGP cost (step 6 no longer deferred) |
| AC-2 | Recursive next-hop (NH not directly connected) | sysrib resolves to directly-connected NH before emitting to FIB |
| AC-3 | N equal-cost paths after best-path | BestChangeEntry.ECMPPaths contains all N next-hops |
| AC-5 | ECMP change emitted to VPP backend | IPRouteAddDel with NPaths=N and N FibPath entries |
| AC-6 | BestChangeEntry with RouteType=blackhole | Kernel: RTN_BLACKHOLE, VPP: drop adjacency |
| AC-7 | BestChangeEntry with RouteType=unreachable | Kernel: RTN_UNREACHABLE, VPP: unreach adjacency |
| AC-10 | BestChangeEntry with Labels (kernel) | Kernel: MPLS encap via lwtunnel (ip route ... encap mpls) |
| AC-11 | BestChangeEntry with SRv6SID | Kernel: SRv6 encap via seg6, VPP: SR policy |
| AC-12 | NH becomes unreachable | All routes using that NH withdrawn from FIB |
| AC-13 | NH cost changes | Best-path re-evaluated for all prefixes using that NH |
| AC-14 | ECMP member fails (NH unreachable) | Nexthop group updated (member removed), not full withdrawal |
| AC-15 | Kernel/VPP produce identical forwarding for same input | Functional test with both backends shows same reachability |
| AC-16 | `ldp { }`, `rsvp-te { }`, `ospf { }` or `isis { }` with `interface { backend vpp }` and no `fib { }` block | fib-vpp is auto-loaded and fib-kernel is not; with `backend netlink` fib-kernel is loaded. LDP, RSVP-TE, OSPF and IS-IS `Dependencies` name no plugin whose registration declares `ProgramsFIB`, and all four declare `NeedsDataPlane` (`TestNoRegistrationDependsOnAFIBWriter` over the whole composition root). No engine, registry or producer source names `fib-kernel` or `fib-vpp` to bring this about |
| AC-17 | An RSVP-TE transit swap entry (one out-label, and the facility-backup stack of `programBackup`) reaches fib-vpp | Two `MplsRouteAddDel` adds in MPLS table 0 for the in-label, S=1 with payload protocol of the next hop's family and S=0 with payload protocol MPLS, each one path via the next hop imposing the whole out stack in order; `Apply` returns nil |
| AC-18 | A penultimate-hop pop (next hop set, no out-label) reaches fib-vpp | Two routes as AC-17 whose path, toward the bare next hop (`sw_if_index` ~0), carries the single label Implicit NULL (3) in uniform mode, so VPP pops (RFC 3032 Section 2.1). The pop does not copy the TTL into the IP header: VPP v26.06 inserts the uniform disposition only for attached-next-hop and DEAG paths (Phase 13 Decision). That is recorded as a VPP limit against the SHOULD of RFC 3032 Section 2.4.3 ("the value of the IP TTL field SHOULD BE replaced with the outgoing TTL value"), row RFC3032-2.4.3-2 `{gap}`, and this AC does not claim the copy |
| AC-19 | An egress pop with no next hop (LDP local FEC, RSVP-TE egress) reaches fib-vpp | S=1 route whose path is a lookup in IP table `fib.vpp.table-id` of the FEC's family; S=0 route whose path is a lookup in MPLS table 0; an egress pop with neither next hop nor FEC is refused with an error naming RFC 3032 Section 2.2. RSVP-TE's egress pop carries its session endpoint as `FEC` |
| AC-20 | fib-vpp configured with `table-id 5` receives a swap | The MPLS routes go in MPLS table 0; table 0 is created with `MplsTableAddDel` before the first label route of a connection and is never deleted |
| AC-21 | A swap, pop, or labeled best-change push is programmed into VPP | Every imposed and pop label Ze sends carries `IsUniform` 1 and no fixed TTL (RFC 3032 Sections 2.4.2, 2.4.3); the existing push path no longer imposes TTL 64. This AC claims the values Ze writes. What VPP does with them on a penultimate pop is AC-18's limit; on an egress pop VPP applies the uniform disposition |
| AC-22 | An entry arrives while fib-vpp holds no writer, or VPP answers a nonzero `Retval` | fib-vpp acknowledges with an error naming the cause (and `ze doctor` when VPP is unreachable); `Apply` returns it; no in-label is recorded as owned. LDP logs the refused entry and its cause instead of dropping it |
| AC-23 | Validation and ownership | fib-vpp refuses an in-label above 2^20-1, an empty swap stack, a stack deeper than `mplsfib.MaxLabelStack`, a swap or pop at nonzero `TableID`, and another source's in-label (EEXIST), with the same rules as fib-kernel, from one validation function in `internal/core/mplsfib`. Remove deletes both S routes. `RemoveLabelSource` deletes only that source's labels and keeps a failed deletion owned for the next call |
| AC-24 | A push entry with a nonzero `TableID` (RSVP-TE facility-backup control context) reaches fib-vpp | Refused with an error that names the private table and that VPP has no packet-mark context; nothing is programmed |
| AC-25 | fib-vpp opens a writer after the label producers started, or after VPP reconnects | fib-vpp emits one `(mpls-fib, replay-request)` with a fresh targeted token after its subscription exists and table 0 is created; LDP, RSVP-TE and OSPF-SR (`srFIB.replay`) re-apply every entry they want installed tagged with it; only that writer consumes and acknowledges the batch; fib-kernel ignores a batch carrying a token it does not hold and does not acknowledge it. OSPF-SR applies its entries with `mplsfib.Apply` and logs a refused entry with its cause, as LDP does. IS-IS emits no label entry, so it owes no answer. BGP EPE (`Source.ReplayLabels`) answers the same way. |
| AC-26 | An entry carries a nonzero `PathMTU` | fib-vpp installs it without an MTU and logs once, as a warning, that labeled frames are bounded by the interface MTU alone |
| AC-27 | `mpls { enable true }` on an interface under `backend vpp` | VPP reports the interface MPLS-enabled (`SwInterfaceSetMplsEnable`), after table 0 exists; removing the leaf disables it on the next apply, including for an interface enabled while ze was down; the netlink backend accepts the set and installs nothing, and the sysctl write is unchanged |
| AC-28 | Real VPP in Docker, `ldp { }` on a VPP interface with `mpls { enable true }`, no peer (`./le test deployment vpp`, scenario `mpls-label-pop`) | `show mpls fib` lists the LDP local label of the LSR-ID FEC as both `eos` and `neos` with the disposition paths of AC-19; a packet-generator frame carrying that label is traced through `mpls-lookup` to IP lookup, and the control frame with an unprogrammed label is dropped at `mpls-lookup`; removing `ldp { }` by reload removes both entries. Reverting the fib-vpp subscription turns the scenario red (recorded) |
| AC-29 | fib-vpp stops with flush | The label routes it installed are removed with the IP routes |

AC-4 and AC-8 moved verbatim to `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md` (owner, 2026-10-08), together with the `BestChangeEntry.TableID` producer.

AC-9 (BestChangeEntry with TableID != 0: route installed in table N on both backends) moved verbatim to the same spec, with its `TestKernelVRFTable` wiring row.
-> Decision (main thread, 2026-10-08): AC-9 cannot be proven end to end without the TableID producer, which now lives in `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md`, so the AC moves with it.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestComparePairIGPCost` | `internal/component/bgp/plugins/rib/bestpath_test.go` | Step 6 selects lower IGP cost | |
| `TestRecursiveNHResolve` | `internal/plugins/sysrib/nhresolver_test.go` | Recursive NH resolved to direct NH | |
| `TestRecursiveNHUnreachable` | `internal/plugins/sysrib/nhresolver_test.go` | Unresolvable NH returns error | |
| `TestECMPGrouping` | `internal/plugins/sysrib/ecmp_test.go` | Equal-cost paths grouped into one entry | |
| `TestECMPMemberWithdraw` | `internal/plugins/sysrib/ecmp_test.go` | NH failure removes from group, not prefix | |
| `TestKernelRouteType` | `internal/plugins/fib/kernel/fibkernel_test.go` | Blackhole/unreachable/prohibit mapped | |
| `TestKernelMPLSEncap` | `internal/plugins/fib/kernel/fibkernel_test.go` | MPLS lwtunnel encap | |
| `TestVPPMultiPath` | `internal/plugins/fib/vpp/fibvpp_test.go` | NPaths > 1 with correct FibPaths | |
| `TestVPPRouteType` | `internal/plugins/fib/vpp/fibvpp_test.go` | Drop/unreach adjacency | |
| `TestBestChangeEntryJSON` | `internal/plugins/sysrib/events/events_test.go` | New fields serialize with omitempty | |
| `TestFibVPPLabelSwapProgramsBothStackPositions` | `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go` | AC-17: one-label swap and a two-label backup stack give S=1 (payload IP4) and S=0 (payload MPLS) routes in table 0, stack order kept | |
| `TestFibVPPLabelPenultimatePop` | `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go` | AC-18: Implicit NULL uniform path via the next hop, both S routes | |
| `TestFibVPPLabelEgressPopLooksUp` | `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go` | AC-19: IPv4 and IPv6 FEC select the IP lookup table and payload; S=0 looks up MPLS table 0; no FEC and no next hop refused | |
| `TestFibVPPLabelUsesMPLSTableZero` | `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go` | AC-20: `table-id 5` still programs table 0; `MplsTableAddDel` precedes the first route once per connection | |
| `TestFibVPPLabelTTLUniform` | `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go` | AC-21: swap, pop and labeled push labels carry `IsUniform` 1 | |
| `TestFibVPPLabelAckWithoutWriter` | `internal/plugins/fib/vpp/register_test.go` | AC-22: with no writer, `mplsfib.Apply` through a real bus returns the cause | |
| `TestFibVPPLabelAckVPPRefusal` | `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go` | AC-22: nonzero `Retval` and a transport error reach the producer; no ownership recorded | |
| `TestFibVPPLabelValidationAndOwnership` | `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go` | AC-23: each refusal, EEXIST for a foreign source, remove of both routes, `RemoveLabelSource` keeps a failed deletion | |
| `TestFibVPPLabelPrivateTableRefused` | `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go` | AC-24 | |
| `TestFibVPPLabelReplayOnWriterOpen` | `internal/plugins/fib/vpp/register_test.go` | AC-25: one targeted request per writer open; foreign-token batch neither consumed nor acknowledged | |
| `TestFibVPPLabelPathMTUWarnsOnce` | `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go` | AC-26 | |
| `TestFibVPPFlushRemovesLabelEntries` | `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go` | AC-29 | |
| `TestApplyReplayTargetsOneWriter`, `TestReplayTokensAreDistinct`, `TestValidateOutLabels` | `internal/core/mplsfib/events_test.go` | AC-23, AC-25: targeted acknowledgement, token counter, the one validator | |
| `TestHandleMPLSEntryIgnoresForeignReplay` | `internal/plugins/fib/kernel/mplsapply_test.go` | AC-25: fib-kernel neither programs nor acknowledges a foreign-token batch | |
| `TestLDPRegistrationNeedsDataPlane`, `TestRSVPTERegistrationNeedsDataPlane` | `internal/plugins/ldp/register_test.go`, `internal/plugins/rsvpte/register_test.go` | AC-16: `NeedsDataPlane` set, no `Dependencies` entry whose registration declares `ProgramsFIB` (asked of the registry, not a name) | |
| `TestLDPEmitLogsRefusal`, `TestLDPReplayReappliesWantedEntries` | `internal/plugins/ldp/fib_test.go` | AC-22, AC-25 | |
| `TestRSVPTEReplayReappliesInstalledEntries`, `TestRSVPTEEgressPopCarriesFEC` | `internal/plugins/rsvpte/fib_test.go` | AC-25, AC-19 | |
| `TestNoRegistrationDependsOnAFIBWriter` | `internal/component/plugin/all/all_test.go` | AC-16: across the composition root, no `Registration.Dependencies` names a plugin that declares `ProgramsFIB` (OSPF and IS-IS converted in Phase 13) | |
| `TestOSPFRegistrationNeedsDataPlane`, `TestISISRegistrationNeedsDataPlane` | `internal/plugins/ospf/register_dataplane_test.go`, `internal/plugins/isis/register_dataplane_test.go` | AC-16: OSPF and IS-IS declare `NeedsDataPlane` and name no FIB writer in `Dependencies` | |
| `TestSRFIBEmitLogsRefusal`, `TestSRFIBEmitLogsMissingWriter`, `TestSRFIBReplayReappliesWantedEntries` | `internal/plugins/ospf/sr_fib_replay_test.go` | AC-25: OSPF-SR applies with `mplsfib.Apply`, logs a refusal and a missing writer, and re-applies its wanted entries on a targeted replay | |
| `TestEPEReplayReappliesInstalledLabels` | `internal/component/bgp/plugins/epe/epe_replay_test.go` | AC-25: BGP EPE re-applies its installed peer-SID labels on a targeted replay | |
| `TestLDPLabelDoctorReportsRefusedEntry` | `internal/plugins/ldp/doctor_label_test.go` | AC-22: runtime check `ldp-label-entries` reports a refused entry under `doctor-fib-mpls-no-writer` | |
| `TestRSVPTELabelDoctorReportsUnreplayedEntries` | `internal/plugins/rsvpte/doctor_label_test.go` | AC-22, AC-25: runtime check `rsvp-te-label-entries` reports entries a replay could not re-apply | |
| `TestSRLabelDoctorReportsRefusedEntry`, `TestSRLabelDoctorReportsUnreplayedEntries`, `TestSRLabelDoctorReportsNoEventBus` | `internal/plugins/ospf/sr_doctor_label_test.go` | AC-22, AC-25: runtime check `ospf-sr-label-entries` reports a refused entry, an entry a replay did not restore, and a missing event bus | |
| `TestEPELabelDoctorReportsRefusedLabels`, `TestEPELabelDoctorForgetsUnwantedRefusal`, `TestEPELabelDoctorReportsUnreplayedLabels` | `internal/component/bgp/plugins/epe/epe_doctor_label_test.go` | AC-22, AC-25: runtime check `bgp-epe-label-entries` reports refused and unreplayed labels and forgets a refusal for a label no longer wanted | |
| `TestRefusalsReportsAndClears`, `TestRefusalsBoundsFindings` | `internal/core/mplsfib/refusal_test.go` | AC-22: the shared `Refusals` record reports, clears on a later success, and bounds its findings | |
| `TestFibVPPSweepRemovesOnlyUnclaimedLabels`, `TestFibVPPSweepSparesLabelClaimedAfterMark`, `TestFibVPPSweepFailureReachesDoctor`, `TestFibVPPSweepFindingsBounded` | `internal/plugins/fib/vpp/mplsstale_test.go` | AC-25: the stale-label sweep removes only table 0 label routes no source re-claimed, spares one claimed after the mark, and reports a failure under `doctor-fib-vpp-mpls-stale-label`, bounded | |
| `TestFibVPPRegistrationDeclaresLabelCheck`, `TestFibVPPDoctorCallbackAnswers` | `internal/plugins/fib/vpp/register_doctor_test.go` | AC-22, AC-26: fib-vpp declares `fib-vpp-label-entries` and its doctor callback answers it | |
| `TestFibVPPLabelDoctorReportsPathMTU` | `internal/plugins/fib/vpp/doctor_label_test.go` | AC-26: runtime check `fib-vpp-label-entries` reports an entry installed without its `PathMTU` under `doctor-fib-vpp-mpls-path-mtu` | |
| `TestVPPStaticNextHopAddressSendsNoInterface` | `internal/plugins/static/backend_vpp_linux_test.go` | Owned row `zero-value-as-valid-answer.md` 2026-11-11: a static next-hop-address path on VPP carries `NoInterface` (~0), never `SwIfIndex` 0 | |
| `TestDesiredMPLSInput` | `internal/component/iface/mplsinput_test.go` | AC-27: the whole set from the `mpls { enable }` leaves | |
| `TestVPPReplaceMPLSInput` | `internal/plugins/iface/vpp/mplsinput_test.go` | AC-27: table 0 first, enable named, disable stale read back from `MplsInterfaceDump` | |
| `TestNetlinkReplaceMPLSInputInstallsNothing` | `internal/plugins/iface/netlink/mplsinput_linux_test.go` | AC-27 | |

The fib-vpp tests drive the label consumer through its subscription callback with the fake `api.Channel` (`testChannel`, `apply_test.go`). That seam records only the last request and fills `Retval` only for `IPRouteAddDelReply`; it gains a recorded request list and replies for `MplsRouteAddDelReply`, `MplsTableAddDelReply` and `SwInterfaceSetMplsEnableReply`, which changes no existing assertion. The RFC-quoting tests (`rfc3032_*` naming, as `internal/plugins/fib/kernel/rfc3032_mplsentry_test.go` does for the kernel) carry `RFC requirement:` tags for RFC 3032 Sections 2.1, 2.2 and 2.4.2 and owe a discrimination record each (`./le rfc record`).

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Weight | 1-256 | 256 | 0 (invalid) | 257 |
| ECMPPaths count | 1-128 | 128 | 0 (single path) | 129 (Linux ECMP limit) |
| Labels (kernel) | 1-3 | 3 | 0 (no MPLS) | 4 (kernel limit) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-fib-ecmp` | `test/bgp/fib-ecmp.ci` | Two eBGP peers, same prefix, ECMP installed | |
| `test-fib-recursive` | `test/bgp/fib-recursive.ci` | iBGP with recursive NH, route installed with resolved NH | |
| `test-fib-blackhole` | `test/bgp/fib-blackhole.ci` | Blackhole route programmed correctly | |
| `test-fib-mpls-kernel` | `test/bgp/fib-mpls-kernel.ci` | MPLS encap route in Linux kernel | |
| `vpp-mpls-ldp-pop` | `test/vpp/vpp-mpls-ldp-pop.ci` | AC-16, AC-19, AC-20, AC-27: `ldp { }` with `backend vpp` and no `fib { }` against the VPP stub (`internal/test/cli/cmd_vpp_stub.go`, which already logs `mpls_route_add_del` and `sw_interface_set_mpls_enable`); the stub log holds `mpls_table_add_del` table 0, then two `mpls_route_add_del` for LDP's local label (eos 1 and 0, table 0), and the daemon log names fib-vpp as the auto-loaded writer | |
| `mpls-label-pop` | `./le test deployment vpp` scenario, `internal/le/test/deployment/vppevidencerun.go`, `vppevidencescenarios.go` | AC-28, against real VPP: `show mpls fib`, `show mpls interface`, a packet-generator frame traced through `mpls-lookup` (the pattern of `vpp_srv6_pg_integration_linux_test.go`), the unprogrammed-label control, and removal on reload | |

Swap needs a signaling peer on both sides, which the single-container harness does not hold; swap and the facility-backup stack are proven by the unit tests through the subscription and by the stub `.ci` form of AC-17 only if a scripted RSVP-TE peer exists. No such peer exists in the tree (`docs/architecture/rsvpte/mpls-rsvp-te.md`, "Interop evidence"), so real-VPP swap forwarding is Work Not Done.

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ecmp-frr` | `test/interop/scenarios/` | FRR | Both install same ECMP set for same UPDATEs | |
| `recursive-nh-frr` | `test/interop/scenarios/` | FRR | Recursive NH resolves identically | |
| `igp-cost-bird` | `test/interop/scenarios/` | BIRD | IGP cost tiebreaker produces same winner | |

### Future (if deferring any tests)
- SRv6 is recorded delivered and has backend producers in the current tree. The old NLRI-family and VPP-steering prerequisites are retired; preserve its existing evidence and include AC-11 in the final acceptance accounting.

## Files to Modify
- `internal/plugins/sysrib/events/events.go` -- extend BestChangeEntry struct
- `internal/plugins/sysrib/sysrib.go` -- NH resolution phase, ECMP grouping
- `internal/component/bgp/plugins/rib/bestpath.go` -- implement step 6
- `internal/plugins/fib/kernel/fibkernel.go` -- rich route processing
- `internal/plugins/fib/kernel/backend_linux.go` -- route types, MPLS (nexthop objects, table and metric moved with AC-4, AC-8 and AC-9 to `spec-fib-nexthop-objects-vpp-metric.md`)
- `internal/plugins/fib/vpp/fibvpp.go` -- multi-path processing, route types
- `internal/plugins/fib/vpp/backend.go` -- multi-path FibPath (table override moved with AC-9 to `spec-fib-nexthop-objects-vpp-metric.md`)
- Phase 13: `internal/core/mplsfib/events.go` -- replay-request handle, `EntryBatch.ReplayID`, the targeted apply, the token counter, the `FEC` comment for egress pops, the package comment (fib-kernel is no longer the only owner)
- Phase 13: `internal/plugins/fib/vpp/mpls.go` -- swap/pop reshaped (stack, both S routes, payload protocol, table 0, uniform), table 0 creation; push to uniform; `validateLabels` replaced by the core validator
- Phase 13: `internal/plugins/fib/vpp/register.go` -- plugin-lifetime `EntryChange` subscription, ack without a writer, replay request on writer open
- Phase 13: `internal/plugins/fib/vpp/fibvpp.go` -- label ownership map, flush of label routes
- Phase 13: `internal/plugins/fib/vpp/apply_test.go` -- `testChannel` records every request and answers the MPLS replies
- Phase 13: `internal/plugins/fib/kernel/mplsentry.go`, `mpls.go` -- ignore a foreign replay token; validation from the core function
- Phase 13: `internal/plugins/ldp/register.go`, `fib.go` -- `NeedsDataPlane`, no "fib-kernel", doctor check dependency; `Apply` with a logged refusal; wanted-entry set and replay answer
- Phase 13: `internal/plugins/rsvpte/register.go`, `fib.go`, `engine.go` -- `NeedsDataPlane`, no "fib-kernel", doctor check dependency; replay answer; egress pop `FEC`
- Phase 13: `internal/component/iface/backend.go` and the iface apply that calls `ReplaceIPv4RPF` -- the whole-set MPLS input call; its implementations in `internal/plugins/iface/vpp/`, `internal/plugins/iface/netlink/` (beside `rpf_linux.go`) and the stub in `internal/plugins/iface/netlink/backend_other.go`
- Phase 13: `internal/test/cli/cmd_vpp_stub.go` -- answer and log `mpls_table_add_del` beside the existing `mpls_route_add_del` and `sw_interface_set_mpls_enable` arms
- Phase 13: `internal/le/test/deployment/vppevidencempls.go` -- the `mpls-label-pop` scenario producer (moved here from `vppevidencerun.go`)
- Phase 13: `internal/plugins/ospf/sr_doctor_label.go` (runtime check `ospf-sr-label-entries`), `internal/component/bgp/plugins/epe/epe_doctor_label.go` (runtime check `bgp-epe-label-entries`), `internal/core/mplsfib/refusal.go` (`Refusals`, the record every label source keeps), `internal/plugins/fib/vpp/mplsstale.go` and `doctor_label.go` (the stale-label sweep, reported by `fib-vpp-label-entries` under code `doctor-fib-vpp-mpls-stale-label`)
- Phase 13: `internal/le/test/deployment/vppevidencerun.go`, `vppevidencescenarios.go`, `vppevidencereport.go` -- the `mpls-label-pop` scenario
- Phase 13: `internal/component/plugin/all/all_test.go` -- `TestNoRegistrationDependsOnAFIBWriter`
- Phase 13 follow-up (2026-11-12), ab87047c60 (a VPP scenario stop waits for its daemon to exit): `internal/le/test/deployment/deployment.go`, `vpp_srv6_integration_test.go`, `vpp_srv6_managed_integration_test.go`, `vppevidence.go`, `vppevidencempls.go`, `vppevidencerun.go`, `vppiface.go`, `vppifaceqos.go`; `docs/guide/vpp.md`
- Phase 13 follow-up (2026-11-12), 5d3d7e3a6d (doctor `mpls-ttl-propagate`): `internal/core/diagnostic/codes.go`, `internal/plugins/fib/kernel/register_linux.go`, `docs/architecture/mpls/mpls-kernel.md`
- Phase 13 follow-up (2026-11-12), f097ae5e30 (doctor checks name their own plugin): `internal/component/plugin/all/all_test.go` (`TestNoDoctorCheckDependsOnAFIBWriter`), `internal/plugins/as112/register.go`, `internal/plugins/dnsresolver/doctor.go`, `internal/plugins/dnsresolver/register.go`, `internal/plugins/geodns/register.go`
- Phase 13 follow-up (2026-11-12), e8cb8ef87c (GoVPP fib binapi): `vendor/modules.txt`
- Phase 13 follow-up (2026-11-12), b1d5f8aabf (fib-vpp FIB source and stale IP route sweep): `internal/plugins/fib/vpp/apply_test.go`, `backend.go`, `doctor_label.go`, `fibvpp.go`, `mpls.go`, `register.go`, `register_doctor.go`, `register_doctor_test.go`, `rfc3032_mplsentry_test.go`, `srv6_lifecycle_repair_test.go`; `internal/test/cli/cmd_vpp_stub.go`, `internal/test/fixture/misc_fixture_vpp.go`, `test/vpp/vpp-fib-route.ci`, `test/vpp/vpp-mpls-push.ci`, `docs/architecture/doctor-and-health-checks.md`
- Phase 13 follow-up (2026-11-12), 9318a687d8 (`show mpls forwarding` reads the selected data plane): `internal/component/mpls/doc.go`, `forwarding_linux.go`, `forwarding_linux_test.go`, `forwarding_other.go`, `forwarding_other_test.go`, `show_forwarding.go`, `show_forwarding_test.go`; `internal/plugins/mpls-cmd/yang/ze-mpls-cmd.yang`; `ai/doors/routing-protocols.md`, `docs/guide/command-reference.md`, `docs/guide/mpls.md`

### Files to Create
- `internal/plugins/sysrib/nhresolver.go` -- recursive NH resolution and tracking
- `internal/plugins/sysrib/nhresolver_test.go` -- unit tests
- `internal/plugins/sysrib/ecmp.go` -- ECMP grouping logic
- `internal/plugins/sysrib/ecmp_test.go` -- unit tests
- `test/bgp/fib-ecmp.ci` -- functional test
- `test/bgp/fib-recursive.ci` -- functional test
- `test/bgp/fib-blackhole.ci` -- functional test
- `test/bgp/fib-mpls-kernel.ci` -- functional test
- `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go` -- Phase 13 unit tests
- `internal/core/mplsfib/events_test.go` -- Phase 13 unit tests
- `internal/component/iface/mplsinput_test.go` -- Phase 13 unit tests
- `internal/plugins/iface/vpp/mplsinput_test.go` -- Phase 13 unit tests
- `internal/plugins/iface/netlink/mplsinput_linux_test.go` -- Phase 13 unit tests
- `test/vpp/vpp-mpls-ldp-pop.ci` -- Phase 13: functional test against the VPP stub
- Phase 13 source: the fib-vpp label consumer (dispatch, validation, ownership, ack) is a new file in `internal/plugins/fib/vpp/` beside `mpls.go`; the desired MPLS input set is a new file in `internal/component/iface/` beside `rpf4.go`; its VPP and netlink implementations are new files in `internal/plugins/iface/vpp/` beside `rpf.go` and in `internal/plugins/iface/netlink/` beside `rpf_linux.go`. Each carries its `// Design:` line to the page it implements
- Phase 13 follow-up (2026-11-12), ab87047c60: `internal/le/test/deployment/vppevidencestop_test.go`
- Phase 13 follow-up (2026-11-12), 5d3d7e3a6d: `internal/plugins/fib/kernel/doctor_mplsttl_linux.go`, `internal/plugins/fib/kernel/doctor_mplsttl_linux_test.go`
- Phase 13 follow-up (2026-11-12), e8cb8ef87c: `vendor/go.fd.io/govpp/binapi/fib/fib.ba.go`, `vendor/go.fd.io/govpp/binapi/fib/fib_rpc.ba.go`
- Phase 13 follow-up (2026-11-12), b1d5f8aabf: `internal/plugins/fib/vpp/routesource.go`, `internal/plugins/fib/vpp/routesource_test.go`, `internal/plugins/fib/vpp/doctor_route.go`, `internal/test/cli/cmd_vpp_stub_mpls.go`, `docs/architecture/fib/fib-vpp-route-ownership.md`
- Phase 13 follow-up (2026-11-12), 9318a687d8: `internal/core/mplsfib/forwarding.go`, `internal/core/mplsfib/forwarding_test.go`, `internal/component/mpls/register.go`, `internal/plugins/fib/vpp/mplsforwarding.go`, `internal/plugins/fib/vpp/mplsforwarding_test.go`, `internal/plugins/fib/vpp/register_mpls_forwarding.go`, `internal/test/fixture/register_vpp_mpls_forwarding_show.go`, `test/vpp/vpp-mpls-forwarding-show.ci`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/plugins/sysrib/yang/` (NH resolution config) |
| CLI commands/flags | [x] | `show system-rib nexthop-table`, `show system-rib ecmp-groups` |
| CLI grammar (action before identifier) | [x] | action-first: `show nexthop table`, `show ecmp groups` |
| Editor autocomplete | [x] | YANG-driven |
| Functional test for new RPC/API | [x] | `test/bgp/fib-*.ci` |
| Doctor check for runtime dependencies | [ ] | N/A (no new external deps) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/route-installation.md` (updated Defect review in closure; `docs/features.md` is generated by `./le feature update`) |
| 2 | Config syntax changed? | No | `git show` across all 10 commits shows no YANG config syntax change (`docs/guide/configuration.md` unchanged; existing `rib`/`fib`/`bgp multipath` syntax holds) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`9318a687d8` for `show mpls forwarding`), `docs/architecture/api/commands.md` (`d7b434442b` for `show rib`, `show nexthop-table`, `show ecmp-groups`) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (`d7b434442b`, System RIB show commands anchored to `internal/component/sysrib/register.go`) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (`20d642984c`: `fib-kernel` superseded cleanup + bounded backoff retry + `fib-sync-failure`), `docs/guide/redistribution.md` (`beb889e840`: `connected` unconditional `OptionalDependencies: ["interface"]`) |
| 6 | Has a user guide page? | Yes | `docs/guide/mpls.md` (`9318a687d8`, `d7b434442b`), `docs/guide/vpp.md` (`ab87047c60`, Phase 13), `docs/guide/plugins.md` (`20d642984c`), `docs/guide/redistribution.md` (`beb889e840`) |
| 7 | Wire format changed? | No | No BGP/MPLS wire format change; `BestChangeEntry` JSON fields are additive with `omitempty` (`internal/component/sysrib/events/events.go`) |
| 8 | Plugin SDK/protocol changed? | No | `git show` confirms no `pkg/plugin/sdk` or `pkg/plugin/rpc` wire change |
| 9 | RFC behavior implemented? | Yes | `internal/component/bgp/plugins/rib/bestpath.go` quotes RFC 4271 Section 9.1.2.2(e) verbatim; RFC 3032 tests and `rfc/short/rfc3032.md` updated in Phase 13 (`2f988033a8`, `10b16fedb2`) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (`6d04ee6acb` rows 34/70/71; closure updates lines 867-938 for eleven VPP deployment scenarios, `scenario <name>`, `mpls-label-pop` 7 checks, `fib-ecmp`, `fib-stale-sweep`) |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` already tracks ECMP, recursive NH, MPLS, and VPP; no feature level changed |
| 12 | Internal architecture changed? | Yes | `docs/architecture/route-selection.md` (2026-10-09), `docs/architecture/fib/fib-depth-4-srv6.md` (`4e3e704cff`), `docs/architecture/fib/fib-vpp-route-ownership.md` (`b1d5f8aabf`), `docs/architecture/doctor-and-health-checks.md` (`4e3e704cff`, `b1d5f8aabf`), `docs/architecture/core-design.md` (closure updates `### FIB VPP` for `ip_route_add_del_v2`, MPLS table 0, labeled ECMP, no-store IP/MPLS) |
| 13 | Phase 13: label entries no longer kernel-only | Yes | `docs/architecture/mpls/mpls-kernel.md` (`01250d9b63`, `5d3d7e3a6d`), `docs/guide/mpls.md` (`9318a687d8`, `d7b434442b`), `ai/doors/forwarding.md`, `ai/doors/routing-protocols.md` (`9318a687d8`), `ai/digests/mpls-signaling.md` (`d7b434442b`), `ai/digests/fib-programming.md` (closure repoint) |
| 14 | Phase 13: plugin registration changed? | Yes | `internal/plugins/{ldp,rsvpte,ospf,isis}/register.go` (`NeedsDataPlane`, `01250d9b63`), `internal/plugins/connected/register.go` (`OptionalDependencies: ["interface"]`, `beb889e840`, documented in `docs/guide/redistribution.md`) |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `./le go lint run && ./le test unit  && ./le test functional` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

The original sequence below is retained as an implementation inventory.
Its numbers are not a current completion fraction.

| Original phase | Current state and remaining obligation |
|----------------|----------------------------------------|
| 1, event contract | Rich fields exist; the TableID producer moved to `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md` |
| 2, NH resolver | `nhResolver.Resolve` and dependency tracking exist; retain AC-2/12 evidence |
| 3, IGP cost | Lookup registration and step-6 comparison exist; AC-1 and AC-13 require end-to-end evidence, including reselection when cost changes |
| 4, ECMP grouping | Grouping and producer weights exist; AC-3/14 evidence remains part of closure |
| 5, kernel depth | Rich routes, MPLS, SEG6 and per-route multipath exist; AC-4 nexthop objects and AC-9 table install moved to `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md` |
| 6, VPP depth | Multipath, route types, supplied table overrides and SRv6 steering exist; AC-8 metric mapping and AC-9 table install moved to `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md` |
| 7, NH cascade | `cascadeRecompute` exists; retain AC-12/14 failure-transition evidence |
| 8 through 12 | Functional and interop evidence, RFC review, final verification and closure remain required; this reconciliation supplies none of those results |
| 13, label entries on VPP | Designed 2026-11-11 (AC-16 to AC-29): fib-vpp consumes `(mpls-fib, entry)`, LDP and RSVP-TE declare `NeedsDataPlane`, iface enforces `mpls { enable }` in VPP. Implemented and proven 2026-11-11 (commits 01250d9b63, 2f988033a8, 748b173c1b; evidence in Goal Validation). It is part of AC-15 backend parity and closes owned row `guard-added-to-one-half-of-a-pair.md` (2026-11-11) |

The separately recorded SRv6 closure covers AC-11's implementation. The
September 7 weight amendment above is also satisfied in source. Neither
record discharges the unresolved requirements in this table.

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: BestChangeEntry extension (event contract)** -- add fields to the shared event struct
   - Tests: `TestBestChangeEntryJSON`
   - Files: `internal/plugins/sysrib/events/events.go`, `events_test.go`
   - Verify: existing tests still pass (omitempty preserves wire compat)

2. **Phase: NH Resolver** -- recursive next-hop tracking and resolution
   - Tests: `TestRecursiveNHResolve`, `TestRecursiveNHUnreachable`
   - Files: `internal/plugins/sysrib/nhresolver.go`, `nhresolver_test.go`
   - Verify: resolver maps recursive NH to direct NH using Loc-RIB

3. **Phase: IGP cost in best-path (step 6)** -- query NH resolver for IGP metric
   - Tests: `TestComparePairIGPCost`
   - Files: `internal/component/bgp/plugins/rib/bestpath.go`
   - Verify: iBGP paths with different IGP costs produce correct winner

4. **Phase: ECMP grouping** -- collect equal-cost paths into groups
   - Tests: `TestECMPGrouping`, `TestECMPMemberWithdraw`
   - Files: `internal/plugins/sysrib/ecmp.go`, `ecmp_test.go`
   - Verify: N equal paths produce one BestChangeEntry with ECMPPaths

5. **Phase: Kernel backend depth** -- route type, MPLS (nexthop objects, metric and table moved to `spec-fib-nexthop-objects-vpp-metric.md`)
   - Tests: `TestKernelRouteType`, `TestKernelMPLSEncap`
   - Files: `internal/plugins/fib/kernel/backend_linux.go`, `fibkernel.go`
   - Verify: mock netlink handle receives correct route structures

6. **Phase: VPP backend depth** -- multi-path, route type, table override
   - Tests: `TestVPPMultiPath`, `TestVPPRouteType`, `TestVPPTable`
   - Files: `internal/plugins/fib/vpp/backend.go`, `fibvpp.go`
   - Verify: mock backend receives multi-path and route-type calls

7. **Phase: NH unreachability cascade** -- NH down triggers route withdrawal
   - Tests: from nhresolver_test.go + fibkernel_test.go
   - Files: sysrib, fibkernel, fibvpp
   - Verify: NH removal cascades to all dependent routes

8. **Functional tests** -- end-to-end with le test
9. **Interop tests** -- FRR/BIRD comparison
10. **RFC refs** -- `// RFC 4271 Section 9.1.2.2 Step 6` comments
11. **Full verification** -- `./le verify current mode full`
12. **Complete spec** -- learned summary, delete spec

13. **Phase: label-switching entries reach the selected data plane** (AC-16 to AC-29; Data Flow, "Phase 13")
   - Order, each step its own commit once its package tests pass:
     1. `internal/core/mplsfib`: the one out-stack validator, the replay-request handle, `EntryBatch.ReplayID`, the targeted apply and the token counter; fib-kernel ignores a foreign token and uses the core validator. Tests: `TestApplyReplayTargetsOneWriter`, `TestReplayTokensAreDistinct`, `TestValidateOutLabels`, `TestHandleMPLSEntryIgnoresForeignReplay`
     2. `fib-vpp`: the reshaped swap/pop backend, table 0, uniform TTL (push included), the plugin-lifetime subscription, ack without a writer, ownership, flush, replay request on writer open. Tests: every `TestFibVPPLabel*` row and `TestFibVPPFlushRemovesLabelEntries`
     3. Producers: LDP `Apply` with a logged refusal and the replay answer, RSVP-TE replay answer and egress-pop `FEC`; then both registrations switch to `NeedsDataPlane` and drop "fib-kernel" from `Dependencies` and their doctor checks. Tests: the LDP and RSVP-TE rows, `TestNoRegistrationDependsOnAFIBWriter`
     4. iface: the whole-set MPLS input call and its three implementations. Tests: `TestDesiredMPLSInput`, `TestVPPReplaceMPLSInput`, `TestNetlinkReplaceMPLSInputInstallsNothing`
     5. The stub `.ci` and the real-VPP `mpls-label-pop` scenario, with its recorded red (fib-vpp subscription reverted, image rebuilt)
     6. The pages in the Documentation Update Checklist rows marked Phase 13, in the same commit as the behaviour each describes
   - RFC comments: each implementing function quotes the RFC 3031 / RFC 3032 sentence it performs (Required Reading, Phase 13 constraints), and each caller names the section
   - Scoped evidence the main thread runs (agents do not run tests): `./le test unit pkg path ./internal/core/mplsfib/`, `./internal/plugins/fib/vpp/`, `./internal/plugins/fib/kernel/`, `./internal/plugins/ldp/`, `./internal/plugins/rsvpte/`, `./internal/component/iface/`, `./internal/plugins/iface/vpp/`, `./internal/plugins/iface/netlink/`, `./internal/component/plugin/server/`, `./internal/component/plugin/all/` (the launcher supplies `ze_vpp`, `ze_ldp`, `ze_rsvpte`); `./le test vpp` for `vpp-mpls-ldp-pop.ci` and the existing `vpp-mpls-push.ci`; `./le test ldp` and `./le test rsvpte` for the producers' unchanged `.ci` behaviour; `./le test deployment vpp` where Docker VPP is available; `./le rfc check` for the new tags. `./le repo changed scope print` names the full set from the diff once code exists

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-15 has implementation with file:line |
| Correctness | ECMP group atomicity (no transient single-path), NH resolution convergence |
| Naming | RouteType enum values match Linux RTN_ names (lowercase), VPP adjacency types |
| Data flow | NH resolution in sysrib only; FIB backends never resolve recursively |
| CLI grammar | `show nexthop table` (action first), `show ecmp groups` (action first) |
| Rule: buffer-first | BestChangeEntry remains value-typed, no heap allocation per-change |
| Rule: enum-over-string | RouteType is numeric enum, not string; parse at boundary |
| Rule: no-sprintf-alloc | Hot-path route processing uses no fmt.Sprintf |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method | Status / Evidence |
|-------------|---------------------|-------------------|
| NH resolver resolves recursive NH | `./le test unit pkg path ./internal/component/sysrib/ run TestRecursiveNH` | Verified: `nhResolver.Resolve` (`internal/component/sysrib/nhresolver.go`), `TestRecursiveNHResolve_*` (`internal/component/sysrib/nhresolver_test.go`) |
| ECMP grouping produces multi-path entries | `./le test unit pkg path ./internal/component/sysrib/ run TestECMP` | Verified: `ecmpCollect` (`internal/component/sysrib/ecmp.go`), `TestECMPCollect_*` (`internal/component/sysrib/ecmp_test.go`) |
| bestpath step 6 implemented | `grep -n "BestStepIGPCost\|RFC 4271 Section 9.1.2.2(e)" internal/component/bgp/plugins/rib/bestpath.go` | Verified: `comparePair` (`internal/component/bgp/plugins/rib/bestpath.go`) compares `a.IGPCost` vs `b.IGPCost` and returns `BestStepIGPCost` |
| VPP backend multi-path | `grep -n "ECMPPaths\|applyPathLabels" internal/plugins/fib/vpp/backend.go` | Verified: `addRichRouteInTable` (`internal/plugins/fib/vpp/backend.go`) builds multi-path `[]fib_types.FibPath` with per-path uniform MPLS labels (`applyPathLabels`); `TestGovppBackendECMPMultipathWire` (`d7b434442b`) + real-VPP `fib-ecmp` (`fbd80152a9`) |
| Functional tests exist | `ls test/plugin/fib-*.ci test/static/static-kernel-igp-cost-reselect.ci test/vpp/vpp-fib-ecmp.ci test/vpp/vpp-mpls-*.ci` | Verified: `fib-ecmp.ci`, `fib-recursive.ci`, `fib-blackhole.ci`, `fib-mpls-kernel.ci`, `fib-srv6-kernel.ci`, `static-kernel-igp-cost-reselect.ci`, `vpp-fib-ecmp.ci`, `vpp-mpls-ldp-pop.ci`, `vpp-mpls-forwarding-show.ci` all present |
| BestChangeEntry has new fields | `grep -n "RouteType\|ECMPPaths\|TableID\|Labels\|SRv6SID" internal/component/sysrib/events/events.go` | Verified: `BestChangeEntry` (`internal/component/sysrib/events/events.go`: `RouteType`, `TableID`, `Labels`, `SRv6SID`, `ECMPPaths`, `MaxECMPPaths = 128`) |
| Phase 13 VPP label switching & parity | `ls internal/plugins/fib/vpp/mplsentry.go internal/plugins/fib/vpp/routesource.go internal/plugins/fib/vpp/doctor_srv6.go internal/le/test/deployment/vppevidencefib.go` | Verified: all files present; real-VPP v26.06 `mpls-label-pop` (7/7), `fib-ecmp` (3/3), `fib-stale-sweep` (5/5) PASS (`fbd80152a9`) |
| Journal fixes (fib-kernel 5 + fib-vpp no-store + connected) | `git log --oneline 4e3e704cff^..1197f0a16c` | Verified: `4e3e704cff` (`doctor_srv6.go`, `srv6_state.go`), `20d642984c` (`pending_retry.go`, `fibkernel.go`, `backend_linux.go`), `beb889e840` (`connected/register.go`) |

### Security Review Checklist (/implement stage 11)

| Check | What to look for | Status / Evidence |
|-------|-----------------|-------------------|
| Resource exhaustion (NH recursion) | NH resolution depth limit (prevent infinite recursion) | Verified: `maxRecursionDepth = 8` (`internal/component/sysrib/nhresolver.go`); tested by `TestRecursiveNHResolve_InfiniteRecursion` and `TestRecursiveNHResolve_SelfReferencing` (`internal/component/sysrib/nhresolver_test.go`) |
| Resource exhaustion (ECMP group) | ECMP group size bounded (max 128 paths) | Verified: `MaxECMPPaths = 128` (`internal/component/sysrib/events/events.go`), capped in `ecmpCollect` (`internal/component/sysrib/ecmp.go`) |
| Label validation | MPLS labels validated (20-bit range, stack depth) before kernel/VPP programming | Verified: `MaxLabel = 1<<20 - 1` and `ValidateOutLabels` (`internal/core/mplsfib/label.go`), called by both `fib-kernel` (`mplsentry.go`) and `fib-vpp` (`applyPathLabels` in `backend.go`, `mplsentry.go`) |
| Resource exhaustion (pending retry & doctor findings) | Retry queue, retry attempts, doctor findings, and SRv6 state list bounded | Verified: `maxPendingEntries = 10000` (`fibkernel.go`), `defaultRetryMaxAttempts = 10` / `defaultRetryMaxDelay = 30s` (`pending_retry.go`), `labelFindingsMax` cap in `doctor_srv6.go` / `doctor_route.go` / `doctor_label.go`, `rpc.StateListMax` (4096) in `srv6_state.go` |
| Input validation / integer bounds / panic safety | No peer-reachable `panic()`, bounded netlink int conversions, no shell injection in interop/deployment harnesses | Verified: `maxNetlinkInt` guards `Metric`/`TableID`/`PathMTU` (`nexthop_linux.go`); `panic("BUG: invalid normalized route verb")` in `fibkernel.go`/`fibvpp.go` only reachable from closed internal `routeaction.Verb()`; interop/deployment exec calls pass `argv` slices (no shell interpolation); `decodeIPRoute` bounds `maxPaths` by `len(body)` (`cmd_vpp_stub.go`) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, back to DESIGN |
| Functional test fails | Check AC; if AC wrong, back to DESIGN |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

## Original Phasing Proposal

This proposed split predates the recorded implementations. It is retained for
scope provenance, not as a list of new implementations to schedule. The
current accounting above governs the remainder and retains every original AC
except AC-4, AC-8 and AC-9, which moved to
`plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md` (owner, 2026-10-08).

| Sub-spec | Scope | Depends |
|----------|-------|---------|
| `spec-fib-depth-1-nhresolver` | NH resolution + IGP cost (AC-1, AC-2, AC-12, AC-13) | none |
| `spec-fib-depth-2-ecmp` | ECMP grouping + nexthop groups (AC-3, AC-4, AC-5, AC-14) | sub-1 |
| `spec-fib-depth-3-richroute` | Route type, metric, table, MPLS kernel (AC-6-10, AC-15) | sub-1 |
| `spec-fib-depth-4-srv6` | SRv6 encap (AC-11) | sub-3 + bgp-nlri-srv6 |

## Work Not Done

| Item | Why not here | Destination |
|------|--------------|-------------|
| A doctor check for a kernel `net.mpls.ip_ttl_propagate` of 0 written outside Ze | The sysctl registry refuses 0 on every write Ze makes, but a value another program writes is never read back, so fib-kernel would push labels carrying `net.mpls.default_ttl` against RFC 3032 Section 2.4.3 with nothing reporting it. Met 2026-11-12 by 5d3d7e3a6d: the fib-kernel `post-config` check `mpls-ttl-propagate` reads the key while MPLS is in use (`kernelcap.MPLSInUse`) and reports `doctor-mpls-ttl-propagate-off` or `doctor-mpls-ttl-propagate-unknown`; recorded as `plan/release/validation/spec-doctor-coverage-10-forwarding.md` AC-109 | met, no destination |
| fib-vpp sweeps no stale IP routes or labeled pushes at writer open | fib-kernel's `startupSweep` lists its routes by `rtm_protocol` and `sweepStale` removes the leftovers after `sweepDelay`; fib-vpp restores only SRv6 ownership (`restoreSRv6`) and `run(runCtx, false)` flushes nothing on stop, so after a ze restart with VPP running the IP routes and labeled pushes a previous run installed keep forwarding. The Phase 13 sweep (`mplsstale.go`) covers MPLS table 0 label routes only. Met 2026-11-12 by b1d5f8aabf: fib-vpp installs its IP routes under its own VPP FIB source `ze-fib-vpp` (sent as `ip_route_add_del_v2`), sweeps that source's unclaimed routes after writer open (`routesource.go`), and reports a route it could not sweep through the doctor check `fib-vpp-stale-routes`, code `doctor-fib-vpp-stale-route`. Unit proof and red walk done; on 2026-11-12 `./le test deployment vpp` passed against real VPP v26.06 with the writer opening under its own FIB source (Goal Validation) | met |
| Real-VPP proof that the sweep removes an actual stale IP route across a ze restart | Met 2026-11-24 by `fbd80152a9` (`internal/le/test/deployment/vppevidencefib.go`, `fib-stale-sweep` + `fib-ecmp`): passed live against real VPP v26.06-release (`./le --name session test deployment vpp scenario mpls-label-pop scenario fib-ecmp scenario fib-stale-sweep`, exit 0, 5/5 stale-sweep checks + 3/3 ECMP checks OK) and `test/ui/le-evidence-vpp-answers.ci` PASS (1/1, 31.0s) | met, no destination |
| Real-VPP proof of `show mpls forwarding` on VPP (9318a687d8) | Met 2026-11-24 by `fbd80152a9` (`internal/le/test/deployment/vppevidencempls.go`, `mpls-forwarding-show` / `mpls-forwarding-removed`): passed live against real VPP v26.06-release (`mpls-label-pop` 7/7 checks OK, `ze show mpls forwarding` reported LDP label 16 as pop and confirmed removal on reload) | met, no destination |
| RSVP-TE facility-backup control contexts (push with a nonzero `TableID`) on a VPP data plane | They are packet-mark tables in the kernel with no VPP form; Phase 13 refuses them (AC-24) and the design of where Ze's own RSVP traffic is routed on a VPP box is its own question | `plan/release/fixes/spec-rsvpte-vpp-dataplane.md` |
| Real-VPP transit swap forwarding proof | The single-container harness holds no RSVP-TE or LDP peer on both sides; Phase 13 proves swap through the subscription with a fake channel and pop on real VPP (AC-28) | `plan/release/fixes/spec-rsvpte-vpp-dataplane.md` |
| OSPF and IS-IS name "fib-kernel" in `Dependencies` | Met in Phase 13 (2026-11-11): OSPF and IS-IS declare `NeedsDataPlane` and drop "fib-kernel", AC-16 covers them, and `TestNoRegistrationDependsOnAFIBWriter` judges the whole composition root. This meets `plan/release/fixes/spec-one-owner-per-domain-fact.md` AC-9 and fixes rows `plugin-list-hardcoded.md` 2026-09-07 and 2026-10-20 | met here, no destination |
| geodns, dnsresolver and as112 doctor checks name "fib-kernel" in `DoctorCheckDef.Dependencies` | Not a route or label producer, so AC-16's registration test does not reach it; LDP's and RSVP-TE's checks of the same shape were re-pointed in Phase 13. Met 2026-11-12 by f097ae5e30: the checks name their own plugin, and `TestNoDoctorCheckDependsOnAFIBWriter` refuses a doctor check naming a FIB writer other than its owner | met, no destination |
| fib-vpp programs no IP route when no persistent store exists | Met 2026-11-24 by 4e3e704cff (`srv6_state.go`, `fibvpp.go`, `srv6.go`, `doctor_srv6.go`): IP and MPLS routes program without a persistent SRv6 store; `vpp-fib-route`, `vpp-fib-route-lookup`, `vpp-mpls-push` all PASS (9/9 `test vpp`) | met, no destination |
| `doctor-fib-mpls-no-writer` and the label install check | Doctor coverage is its own child; Phase 13 makes its fix command true | `plan/release/validation/spec-doctor-coverage-10-forwarding.md` AC-108, AC-83 |
| LFA / TI-LFA fast-reroute backup takeover on Linux, `fib-vpp` `Backup` handling, and `ze_ospf_fast_reroute_backups_installed` FIB accounting (`plan/journal/kernel-refuses-what-the-installer-sends.md`, 2026-11-13) | Spun out by owner decision on 2026-11-24: Linux has no standby `RTA_MULTIPATH` member (`RTNH_F_LINKDOWN` rejected by IPv4 `fib_get_nhs`), `fib-vpp` ignores `Backup`, and the OSPF metric counts SPF rather than FIB installs; requires dedicated design choices across Linux, VPP, and OSPF | `plan/release/fixes/spec-fib-lfa-backup-takeover.md` |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `fib-vpp` always had a persistent store before programming any route | `restoreSRv6` ran unconditionally before plain IP/MPLS route handling, blocking IP/MPLS routes on deployments without `ze init` | `test/vpp/vpp-fib-route.ci` and `vpp-mpls-push.ci` failures (2026-11-12) | Fixed in `4e3e704cff` (`srv6_state.go`, `fibvpp.go`, `doctor_srv6.go`); plain IP/MPLS program without a store, SRv6 refuses and reports via `fib-vpp-srv6` |
| `connected {}` + `bgp {}` without `interface {}` or `static {}` still started the interface monitor | `connected/register.go` omitted `OptionalDependencies: ["interface"]`, leaving connected subnets out of Loc-RIB and direct BGP NHs unresolvable | Observed bringing up `bgp-ecmp-frr` interop scenario (2026-11-24) | Fixed in `beb889e840` (`connected/register.go`, `docs/guide/redistribution.md`, journal row) |
| Documentation matched `fbd80152a9` and `d7b434442b` after implementation | `docs/architecture/testing/interop.md` still said "nine scenarios" / 5 `mpls-label-pop` checks, and `docs/architecture/core-design.md` (`### FIB VPP`) still described `IPRouteAddDel` v1 and implicit MPLS table 0 | Closure documentation audit (2026-11-24) | Fixed in closure commit A (`docs/architecture/testing/interop.md`, `docs/architecture/core-design.md`) |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Running live revert-and-rebuild interop discrimination against the shared working tree on 2026-11-24 | Another active session's uncommitted `plugin {} -> bus {}` refactor in `internal/component/plugin/server/` breaks fresh `ze` builds (`unknown top-level keyword: plugin` and `send_permission.go:270` compile error) | Recorded unit + `.ci` discrimination and passing live Docker interop runs on `6d04ee6acb`; flagged live interop revert-and-rebuild discrimination as owed once `bus {}` lands (`ai/rules/interop-and-goal-validation.md`) |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| none | - | - | - |

## Design Insights

- Separating `errSRv6StoreUnavailable` from corrupt/ambiguous SRv6 ownership in `fib-vpp` (`srv6_state.go`) keeps plain IP and MPLS forwarding operational without a persistent store while preserving fail-closed safety for SRv6 steering.
- Modeling `fib-kernel` partial applies via `*supersededDeleteError` + `supersededTables` (`backend.go`, `fibkernel.go`) and bounded retry (`pending_retry.go`) ensures make-before-break table moves and priority replacements never lose ownership tracking or leave silent kernel divergence.

## RFC Documentation

- `internal/component/bgp/plugins/rib/bestpath.go` (`comparePair`) quotes RFC 4271 Section 9.1.2.2(e) verbatim above the IGP-cost tiebreaker (`BestStepIGPCost`).
- `internal/plugins/fib/vpp/mpls.go`, `mplsentry.go`, `backend.go`, and `rfc3032_mplsentry_test.go` quote RFC 3031 Sections 3.10/3.13 and RFC 3032 Sections 2.1, 2.2, 2.4.2, and 2.4.3.

## Implementation Summary

### What Was Implemented
- **Routing decision & sysrib depth:** IGP-cost step-6 comparison (`internal/component/bgp/plugins/rib/bestpath.go:451-468`), recursive next-hop resolution and dependency cascade (`internal/component/sysrib/nhresolver.go`, `sysrib.go`), ECMP grouping and weighted members (`internal/component/sysrib/ecmp.go`, `events/events.go`), and rich kernel/VPP route programming (`nexthop_linux.go`, `richroute.go`, `fib/vpp/backend.go`, `fib/vpp/srv6.go`).
- **Scope splits approved by owner:** AC-4 (Linux nexthop objects), AC-8 (VPP metric), AC-9, and `BestChangeEntry.TableID` production moved to `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md` (2026-10-08); LFA backup takeover moved to `plan/release/fixes/spec-fib-lfa-backup-takeover.md` (2026-11-24, `ca33a974e7`).
- **Phase 13 VPP label switching & backend parity (`01250d9b63`, `2f988033a8`, `748b173c1b`, `ab87047c60`, `5d3d7e3a6d`, `f097ae5e30`, `e8cb8ef87c`, `b1d5f8aabf`, `9318a687d8`):** `fib-vpp` consumes `(mpls-fib, entry)` swap/pop entries in MPLS table 0 (both S=1/S=0, uniform TTL), sweeps stale MPLS labels and stale `ze-fib-vpp` IP routes, serves VPP `show mpls forwarding`; LDP/RSVP-TE/OSPF/IS-IS declare `NeedsDataPlane` instead of `fib-kernel`; VPP iface enforces `mpls { enable true }`.
- **10 closing-wave commits (`4e3e704cff`..`1197f0a16c`):**
  1. `4e3e704cff` -- `fib-vpp` programs plain IP/MPLS without a persistent SRv6 store; adds `fib-vpp-srv6` runtime doctor check (`doctor-fib-vpp-srv6-refused`).
  2. `20d642984c` -- `fib-kernel` fixes 5 journal defects: ESRCH sweep counting, partial-apply `supersededTables`, table-aware `ownsSlotLocked` + old-table delete, non-rich nonzero `TableID` refusal, and bounded exponential-backoff retry (`pending_retry.go`) + `fib-sync-failure` lifecycle.
  3. `d7b434442b` -- `fib-vpp` fixes labeled ECMP dispatch (`isSinglePathMPLS`, `applyPathLabels`), adds `TestGovppBackendECMPMultipathWire`, extends VPP stub multipath decoding, and adds `test/vpp/vpp-fib-ecmp.ci` (80/80 stress clean).
  4. `6d04ee6acb` -- adds and proves 3 live Docker BGP/FIB interop scenarios (`bgp-ecmp-frr` vs FRR 10.7.1, `bgp-recursive-nh-frr` vs FRR 10.7.1, `bgp-igp-cost-bird` vs BIRD 3.4.0).
  5. `beb889e840` -- `connected` declares `OptionalDependencies: ["interface"]`.
  6. `625c9a2a5e` -- shortens `le spec design carry` Summary under 60 chars.
  7. `fbd80152a9` -- adds live real-VPP v26.06 proofs (`fib-ecmp`, `fib-stale-sweep`, `show mpls forwarding` in `mpls-label-pop`), repeatable `scenario <name>` selector, and `KernelCapabilitiesNone` scoping for VPP userspace proofs.
  8. `70c470b45c`, `ca33a974e7`, `1197f0a16c` -- spec bookkeeping and LFA spin-out.

### Bugs Found/Fixed
- `fib-vpp` blocked plain IP/MPLS routes without a persistent store (`4e3e704cff`; tests: `internal/plugins/fib/vpp/doctor_srv6_test.go`, `test/vpp/` 9/9 PASS).
- `fib-kernel` ESRCH miscounted in sweep, partial apply lost ownership, table move leaked old table, plain backend ignored nonzero `TableID`, refused routes never retried (`20d642984c`; tests: `fibkernel_journal_fixes_test.go`, `backend_journal_fixes_linux_test.go`).
- `fib-vpp` dropped `ECMPPaths` on labeled ECMP routes via `processMPLSChange` (`d7b434442b`; tests: `TestGovppBackendECMPMultipathWire`, `test/vpp/vpp-fib-ecmp.ci`).
- `connected` did not pull `interface` without an explicit `interface {}` or `static {}` stanza (`beb889e840`; tests: `TestConnectedDeclaresOptionalInterfaceDependency`, `TestOptionalEdgesWithoutNeedsPullAsBefore/connected`).
- `le spec design carry` Summary exceeded 60-char startup registration check (`625c9a2a5e`).

### Documentation Updates
- `docs/architecture/route-selection.md`: in-process reselection on Loc-RIB changes (2026-10-09).
- `docs/architecture/doctor-and-health-checks.md` (`4e3e704cff`, `b1d5f8aabf`): `fib-vpp-srv6` and `fib-vpp-stale-routes` checks (`<!-- source: internal/plugins/fib/vpp/doctor_srv6.go -- srv6CheckDecl, srv6DoctorFindings -->`).
- `docs/architecture/fib/fib-depth-4-srv6.md` (`4e3e704cff`): no-store IP/MPLS programming vs SRv6 refusal (`<!-- source: internal/plugins/fib/vpp/srv6_state.go -- errSRv6StoreUnavailable, isSRv6StoreUnavailable -->`).
- `docs/architecture/fib/fib-vpp-route-ownership.md` (`b1d5f8aabf`): `ze-fib-vpp` FIB source and stale IP route sweep.
- `docs/guide/plugins.md` (`20d642984c`): `fib-kernel` superseded cleanup and bounded backoff retry (`<!-- source: internal/plugins/fib/kernel/pending_retry.go -- runRetryLoop, retryDuePending, retryPendingAfterProgressLocked -->`).
- `docs/guide/mpls.md`, `ai/digests/mpls-signaling.md`, `docs/architecture/api/commands.md` (`9318a687d8`, `d7b434442b`): data-plane `show mpls forwarding` readers and System RIB show commands.
- `docs/guide/redistribution.md` (`beb889e840`): `connected` optional `interface` dependency (`<!-- source: internal/plugins/connected/register.go -- init -->`).
- `docs/architecture/testing/interop.md` (`6d04ee6acb` rows 34/70/71; closure updates lines 867-938 for eleven VPP deployment scenarios, `scenario <name>`, `mpls-label-pop` 7 checks, `fib-ecmp`, `fib-stale-sweep`).
- `docs/architecture/core-design.md` (closure updates `### FIB VPP` for `ip_route_add_del_v2`, explicit MPLS table 0 creation, labeled ECMP, and no-store IP/MPLS programming; repointed 4 Go `// Design:` headers to this page).
- `website/changes/fragments/fib-depth.md` (closure operator-visible change fragment).

### Deviations from Plan
- AC-4 (Linux nexthop objects), AC-8 (VPP metric), AC-9, and `BestChangeEntry.TableID` production were moved by owner decision (2026-10-08) to `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md`.
- LFA backup takeover (`plan/journal/kernel-refuses-what-the-installer-sends.md`, 2026-11-13) was spun out by owner decision (2026-11-24, `ca33a974e7`) to `plan/release/fixes/spec-fib-lfa-backup-takeover.md`.
- Original functional test paths listed as `test/bgp/fib-*.ci` live under `test/plugin/fib-*.ci` (and `test/static/static-kernel-igp-cost-reselect.ci`, `test/vpp/vpp-fib-ecmp.ci`), matching the plugin test taxonomy.
- Original unit paths listed as `internal/plugins/sysrib/...` moved to `internal/component/sysrib/...` when `sysrib` was promoted to a component.
- Uncommitted 12-line AC-14 zero-withdrawal check in `internal/test/fixture/sysrib_ecmp_daemon_fixture.go` is held out of Commit A (untested in QEMU because cached 7.3 kernel `1fc606ec` is absent and `gokrazy/kernel/kernel.config` is dirty); AC-14 is already satisfied by unit + VPP stub (80/80 zero-withdrawal) + real-VPP `fib-ecmp` + live `bgp-ecmp-frr`.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Step-6 IGP cost comparison & cost-change reselection (AC-1, AC-13) | Done | `internal/component/bgp/plugins/rib/bestpath.go` (`comparePair`), `rib_aigp.go` | Unit + `.ci` discriminated; live `bgp-igp-cost-bird` PASS (`6d04ee6acb`, live revert discrimination blocked by working-tree `bus {}` refactor) |
| Recursive NH resolution & withdrawal cascade (AC-2, AC-12) | Done | `internal/component/sysrib/nhresolver.go` (`Resolve`), `sysrib.go` | Unit + `.ci` discriminated; live `bgp-recursive-nh-frr` PASS (`6d04ee6acb`, live revert discrimination blocked by working-tree `bus {}` refactor) |
| ECMP grouping & member-failure update without full withdraw (AC-3, AC-5, AC-14) | Done | `internal/component/sysrib/ecmp.go` (`ecmpCollect`), `internal/plugins/fib/vpp/backend.go` (`addRichRouteInTable`, `applyPathLabels`), `fibvpp.go` (`isSinglePathMPLS`) | Unit + `.ci` + VPP stub (80/80) + real-VPP `fib-ecmp` (3/3) + live `bgp-ecmp-frr` PASS |
| Rich route attributes in kernel & VPP (AC-6, AC-7, AC-10, AC-11) | Done | `internal/plugins/fib/kernel/nexthop_linux.go`, `internal/plugins/fib/vpp/backend.go`, `srv6.go` | Kernel blackhole/unreachable/prohibit, MPLS push, SRv6 verified |
| Kernel/VPP backend parity & Phase 13 label switching (AC-15..AC-29) | Done | `internal/plugins/fib/vpp/mplsentry.go`, `mpls.go`, `routesource.go`, `doctor_srv6.go`, `mplsforwarding.go`, `internal/le/test/deployment/vppevidencefib.go`, `vppevidencempls.go` | Real-VPP v26.06 `mpls-label-pop` (7/7), `fib-ecmp` (3/3), `fib-stale-sweep` (5/5) PASS (`fbd80152a9`) |
| AC-4, AC-8, AC-9, TableID producer | Changed (moved by owner) | `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md` | Owner decision 2026-10-08 |

### Acceptance Criteria
Audit 2026-10-09, updated 2026-10-10 and 2026-11-24 (closure audit). "Unit" means a Go test over the producer;
"needs-linux" means it runs only in the QEMU guest (`./le test qemu run ... all-tests test <path>`,
arm64 7.2 kernel `tmp/kernel/build/vmlinuz`). Each `.ci` red was one producer break, with the
daemons rebuilt from the broken tree. Live Docker interop scenarios (`bgp-ecmp-frr`, `bgp-recursive-nh-frr`,
`bgp-igp-cost-bird`) all passed on `6d04ee6acb`; live revert-and-rebuild interop discrimination could not
run on 2026-11-24 against the shared working tree because another session's uncommitted `plugin {} -> bus {}`
refactor in `internal/component/plugin/server/` causes fresh `ze` builds to fail (`unknown top-level keyword: plugin`).

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | unit, functional, and live interop green (live interop revert discrimination pending `bus {}` tree) | `TestIGPCostChangeReselectsWithoutAIGP`, `TestComparePairIGPCost`; `test/static/static-kernel-igp-cost-reselect.ci`; live Docker interop `bgp-igp-cost-bird` (`6d04ee6acb`, PASS 1m21s vs BIRD 3.4.0) | Unit and `.ci` discriminated; live interop proves anti-vacuous IGP-cost step parity with BIRD 3.4.0; live revert discrimination blocked by mid-refactor working tree |
| AC-13 | unit, functional, and live interop green (live interop revert discrimination pending `bus {}` tree) | same unit & `.ci` + live Docker interop `bgp-igp-cost-bird` Phase 2 (`6d04ee6acb`, PASS 1m21s: SIGHUP metric 10->30 flips winner on Ze and BIRD without BGP session reset) | Unit and `.ci` discriminated; live interop proves end-to-end reload reselection parity with BIRD 3.4.0; live revert discrimination blocked by mid-refactor working tree |
| AC-2 | unit, functional, and live interop green (live interop revert discrimination pending `bus {}` tree) | `TestRecursiveNHResolve_*`, `TestRFC4271BGPNextHopResolvedToTheImmediateNextHop`, `TestFIBRecursiveIPv4ViaIPv6`; `test/plugin/fib-recursive.ci`; live Docker interop `bgp-recursive-nh-frr` (`6d04ee6acb`, PASS 2m6s vs FRR 10.7.1) | `.ci` discriminated on 3-level chain; live interop proves off-link NH `10.250.0.1 -> .15` parity with FRR + unresolvable negative control; live revert discrimination blocked by mid-refactor working tree |
| AC-3 | unit, functional, and live interop green (live interop revert discrimination pending `bus {}` tree) | `TestECMPCollect_*`, `test/plugin/fib-ecmp.ci`, live Docker interop `bgp-ecmp-frr` (`6d04ee6acb`, PASS 1m9s vs FRR 10.7.1) | `.ci` discriminated; live interop proves symmetric eBGP ECMP kernel FIB proto 250 parity `{.5, .15}` + `show ecmp-groups` + unequal-cost control `{.5}`; live revert discrimination blocked by mid-refactor working tree |
| AC-5 | unit, VPP stub, and live real-VPP green | `TestVPPMultiPath`, `TestGovppBackendECMPMultipathWire` (`d7b434442b`), `TestVPPStubDecodesMultipathIPRouteV2`, `test/vpp/vpp-fib-ecmp.ci` (80/80 stress clean, 9/9 `test vpp`), real-VPP `fib-ecmp` (`fbd80152a9`, PASS live vs VPP v26.06: 2-path `dpo-load-balance` buckets:2 under `ze-fib-vpp`, member withdrawal buckets:1, full withdrawal) | Fixed labeled ECMP dispatch bug in `fib-vpp` (`backend.go`, `fibvpp.go`) and proved on real VPP v26.06 |
| AC-6 | unit and functional green, `.ci` discriminated | `TestKernelRouteType`, `TestNetlinkIntegration_BlackholeRouteWithNextHop`, `TestVPPRouteType`, `test/plugin/fib-blackhole.ci` (needs-linux) | reads kernel type of blackhole, unreachable and prohibit (`ip -N` prints RTN 6, 7, 8). PASS green, FAIL with `routeTypeToLinux` mapping prohibit to RTN_UNREACHABLE (got 7) |
| AC-7 | unit and functional green, discriminated | `TestKernelRouteType`, `TestVPPRouteType`, `test/plugin/fib-blackhole.ci` | same `.ci` reads the installed `unreachable` (7) and `prohibit` (8) routes |
| AC-10 | integration and functional green, `.ci` discriminated | `TestMPLSIntegration_Push`, `TestKernelMPLSPush`, `test/plugin/fib-mpls-kernel.ci` (needs-linux) | `.ci` reads `encap mpls 100/200 via 192.0.2.1 ... proto 250` from the kernel. PASS green, FAIL with `buildMPLSEncap` truncating the stack to its first label (got `encap mpls 100`) |
| AC-11 | needs-linux integration and VPP unit exist | `srv6_integration_linux_test.go`, `TestSRv6SteerAdd`, `TestSRv6SteerWithdraw`, `doctor_srv6_test.go` (`4e3e704cff`) | recorded delivered (learned 1113); no-store refusal and doctor check added in `4e3e704cff` |
| AC-12 | unit, functional, and live interop green (live interop revert discrimination pending `bus {}` tree) | `TestRFC4271BGPRouteWithAnUnresolvedNextHopLeavesTheFIB`, `TestNHResolver_Tracking`, `test/plugin/rfc4271-next-hop-kernel.ci` (+ forked), `test/plugin/sysrib-recursive-nexthop.ci` (+ forked), live Docker interop `bgp-recursive-nh-frr` Phase 2 (`6d04ee6acb`, PASS 2m6s) | Cover removal withdraws `10.200.0.0/24` from both Ze and FRR kernel FIBs while direct control and iBGP session survive; live revert discrimination blocked by mid-refactor working tree |
| AC-14 | unit, functional, VPP stub, real-VPP, and live interop green | `TestECMPMemberFail`, `TestECMPRecursiveMemberLifecycle`, `TestGovppBackendECMPMultipathWire` (`d7b434442b`), `test/vpp/vpp-fib-ecmp.ci` (80/80 stress, zero intervening withdraw), real-VPP `fib-ecmp` (`fbd80152a9`, PASS live vs VPP v26.06), `test/plugin/sysrib-ecmp-replay.ci` (+ forked), live Docker interop `bgp-ecmp-frr` (`6d04ee6acb`, PASS 1m9s) | Satisfied by unit + VPP stub (80/80 zero-withdrawal) + real-VPP `fib-ecmp` + live `bgp-ecmp-frr`; uncommitted 12-line `sysrib_ecmp_daemon_fixture.go` check kept out of Commit A |
| AC-15 | unit, VPP stub, UI fixture, and live real-VPP green | `test/vpp/` (9/9 PASS including `vpp-fib-ecmp`), `test/ui/le-evidence-vpp-answers.ci` PASS (`1/1`, 31.0s), live real VPP v26.06 (`fbd80152a9`): `mpls-label-pop` (7/7 incl. `ze show mpls forwarding`), `fib-ecmp` (3/3), `fib-stale-sweep` (5/5 across SIGKILL restart + 30s sweep + control preservation) | `vpp`/`vpp-iface` scoped to `KernelCapabilitiesNone` per `plan/records/ruling-interop-scoped-kernel-capabilities.md` item 3 (`ze-test-judge` approved); `connected` optional-dep fix landed in `beb889e840` |
| AC-16 | Done (unit + stub + real VPP) | `TestNoRegistrationDependsOnAFIBWriter` (`all_test.go`), `TestLDPRegistrationNeedsDataPlane`, `TestRSVPTERegistrationNeedsDataPlane`, `TestOSPFRegistrationNeedsDataPlane`, `TestISISRegistrationNeedsDataPlane`, `test/vpp/vpp-mpls-ldp-pop.ci`, `mpls-label-pop` | All four producers declare `NeedsDataPlane` and name no FIB writer in `Dependencies` (`01250d9b63`) |
| AC-17 | Done (unit) | `TestFibVPPLabelSwapProgramsBothStackPositions` (`internal/plugins/fib/vpp/rfc3032_mplsentry_test.go:110`) | Both S=1 (IP payload) and S=0 (MPLS payload) routes programmed in table 0; real-VPP swap proof homed in `plan/release/fixes/spec-rsvpte-vpp-dataplane.md` |
| AC-18 | Done (unit) | `TestFibVPPLabelPenultimatePop` (`internal/plugins/fib/vpp/rfc3032_mplsentry_test.go:136`) | Implicit NULL (3) uniform path via bare next hop (`sw_if_index ~0`); VPP v26.06 TTL limit recorded under RFC3032-2.4.3-2 `{gap}` |
| AC-19 | Done (unit + stub + real VPP) | `TestFibVPPLabelEgressPopLooksUp` (`rfc3032_mplsentry_test.go:158`), `TestRSVPTEEgressPopCarriesFEC` (`rsvpte/fib_test.go`), `test/vpp/vpp-mpls-ldp-pop.ci`, `mpls-label-pop` | S=1 IP table lookup, S=0 MPLS table 0 lookup; missing NH+FEC refused naming RFC 3032 Section 2.2 |
| AC-20 | Done (unit + stub + real VPP) | `TestFibVPPLabelUsesMPLSTableZero` (`rfc3032_mplsentry_test.go:189`), `test/vpp/vpp-mpls-ldp-pop.ci` | Always programs MPLS table 0 after explicit `MplsTableAddDel`, never `fib.vpp.table-id` |
| AC-21 | Done (unit + RFC record) | `TestFibVPPLabelTTLUniform` (`rfc3032_mplsentry_test.go:212`), `TestGovppBackendECMPMultipathWire` (`ecmp_wire_test.go:51`) | Uniform mode (`IsUniform=1`) on swap, pop, single-path push, and labeled ECMP push (`2f988033a8`, `d7b434442b`) |
| AC-22 | Done (unit) | `TestFibVPPLabelAckWithoutWriter` (`rfc3032_mplsentry_test.go:368`), `TestFibVPPLabelAckVPPRefusal` (`rfc3032_mplsentry_test.go:242`), `TestLDPEmitLogsRefusal`, `TestLDPLabelDoctorReportsRefusedEntry`, `TestRefusalsReportsAndClears` | Refusals acknowledged with cause and `ze doctor` fix; logged and reported via runtime doctor checks |
| AC-23 | Done (unit) | `TestFibVPPLabelValidationAndOwnership` (`rfc3032_mplsentry_test.go:264`), `TestValidateOutLabels` (`internal/core/mplsfib/events_test.go`) | Shared validator in `internal/core/mplsfib/label.go`; EEXIST on foreign source; `RemoveLabelSource` retains failed deletes |
| AC-24 | Done (unit) | `TestFibVPPLabelPrivateTableRefused` (`rfc3032_mplsentry_test.go:311`) | Nonzero `TableID` push refused; VPP control-context design homed in `plan/release/fixes/spec-rsvpte-vpp-dataplane.md` |
| AC-25 | Done (unit) | `TestFibVPPLabelReplayOnWriterOpen` (`rfc3032_mplsentry_test.go:353`), `TestFibVPPLabelReplayTargetsThisWriter` (`rfc3032_mplsentry_test.go:339`), `TestApplyReplayTargetsOneWriter`, `TestHandleMPLSEntryIgnoresForeignReplay`, `TestLDPReplayReappliesWantedEntries`, `TestRSVPTEReplayReappliesInstalledEntries`, `TestSRFIBReplayReappliesWantedEntries`, `TestEPEReplayReappliesInstalledLabels`, `TestFibVPPSweep*` (`mplsstale_test.go`) | Targeted replay token + stale-label sweep on writer open; fib-kernel ignores foreign replay tokens |
| AC-26 | Done (unit) | `TestFibVPPLabelPathMTUWarnsOnce` (`rfc3032_mplsentry_test.go:324`), `TestFibVPPLabelDoctorReportsPathMTU` (`doctor_label_test.go`) | Installs without per-route MTU, warns once, reports under `doctor-fib-vpp-mpls-path-mtu` |
| AC-27 | Done (unit + real VPP) | `TestDesiredMPLSInput` (`internal/component/iface/mplsinput_test.go`), `TestVPPReplaceMPLSInput` (`internal/plugins/iface/vpp/mplsinput_test.go`), `TestNetlinkReplaceMPLSInputInstallsNothing` (`internal/plugins/iface/netlink/mplsinput_linux_test.go`), real-VPP `mpls-label-pop` (`mpls-input`) | Whole-set MPLS input enforced on VPP; kernel backend accepts and installs nothing while sysctl stays |
| AC-28 | Done (real VPP, discriminated) | `./le test deployment vpp scenario mpls-label-pop` (`01250d9b63`, `fbd80152a9` 7/7 checks PASS live on VPP v26.06) | `installed`, `mpls-input`, `mpls-forwarding-show`, `popped`, `unprogrammed-drop`, `removed`, `mpls-forwarding-removed`; red recorded with fib-vpp subscription removed |
| AC-29 | Done (unit) | `TestFibVPPFlushRemovesLabelEntries` (`rfc3032_mplsentry_test.go:384`) | Stop with flush removes label routes alongside IP routes |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestComparePairIGPCost`, `TestIGPCostChangeReselectsWithoutAIGP` | Done | `internal/component/bgp/plugins/rib/bestpath_test.go`, `rfc4271_igp_cost_reselect_test.go` | AC-1, AC-13 |
| `TestRecursiveNHResolve_*`, `TestNHResolver_IGPMetric`, `TestNHResolver_Tracking` | Done | `internal/component/sysrib/nhresolver_test.go` | AC-2, AC-12; moved with `sysrib` from `plugins/` to `component/` |
| `TestECMPCollect_*`, `TestECMPChanged_*` | Done | `internal/component/sysrib/ecmp_test.go` | AC-3, AC-14 |
| `TestBestChangeEntryJSON_*` | Done | `internal/component/sysrib/events/events_test.go` | Event wire compatibility + rich fields |
| `TestKernelRouteType`, `TestKernelMPLSPush`, `TestKernelECMPGroup`, `TestKernelSRv6Encap` | Done | `internal/plugins/fib/kernel/fibkernel_test.go` | AC-6, AC-7, AC-10, AC-11 |
| `TestFIBKernelSweepDoesNotCountESRCHAsRemovedOrphan`, `TestFIBKernelTableMove*`, `TestFIBKernelPlainBackendRefusesNonzeroTable*`, `TestFIBKernelPendingInstallRetries*` | Done | `internal/plugins/fib/kernel/fibkernel_journal_fixes_test.go`, `backend_journal_fixes_linux_test.go` | 5 `fib-kernel` journal fixes (`20d642984c`) |
| `TestVPPMultiPath`, `TestVPPRouteType`, `TestGovppBackendECMPMultipathWire` | Done | `internal/plugins/fib/vpp/apply_test.go`, `ecmp_wire_test.go` | AC-5, AC-6, AC-7, AC-14 (`d7b434442b`) |
| `TestFibVPPPlainAndMPLSRoutesSucceedWithoutStore`, `TestFibVPPMixedBatchRefusesSRv6AndProgramsPlainIP`, `TestFibVPPSRv6Doctor*` | Done | `internal/plugins/fib/vpp/doctor_srv6_test.go` | `fib-vpp` no-store fix (`4e3e704cff`) |
| `TestFibVPPLabel*`, `TestFibVPPFlushRemovesLabelEntries`, `TestFibVPPSweep*` | Done | `internal/plugins/fib/vpp/rfc3032_mplsentry_test.go`, `mplsstale_test.go`, `routesource_test.go`, `mplsforwarding_test.go` | Phase 13 AC-17..AC-29 |
| `TestNoRegistrationDependsOnAFIBWriter`, `TestNoDoctorCheckDependsOnAFIBWriter`, `TestConnectedDeclaresOptionalInterfaceDependency` | Done | `internal/component/plugin/all/all_test.go`, `internal/plugins/connected/register_test.go` | AC-16 (`01250d9b63`, `f097ae5e30`), `connected` fix (`beb889e840`) |
| `TestDesiredMPLSInput`, `TestVPPReplaceMPLSInput`, `TestNetlinkReplaceMPLSInputInstallsNothing` | Done | `internal/component/iface/mplsinput_test.go`, `internal/plugins/iface/vpp/mplsinput_test.go`, `internal/plugins/iface/netlink/mplsinput_linux_test.go` | AC-27 |
| Functional `.ci` tests (`fib-ecmp`, `fib-recursive`, `fib-blackhole`, `fib-mpls-kernel`, `fib-srv6-kernel`, `static-kernel-igp-cost-reselect`, `vpp-fib-ecmp`, `vpp-mpls-ldp-pop`, `vpp-mpls-forwarding-show`, `le-evidence-vpp-answers`) | Done | `test/plugin/`, `test/static/`, `test/vpp/`, `test/ui/` | All present and verified |
| Interop scenarios (`bgp-ecmp-frr`, `bgp-recursive-nh-frr`, `bgp-igp-cost-bird`) | Passed live; revert discrimination pending `bus {}` tree | `test/interop/scenarios/bgp-{ecmp-frr,recursive-nh-frr,igp-cost-bird}/`, `internal/le/interoplab/bgp/check_fib_depth.go` | Passed live on `6d04ee6acb` (1m9s, 2m6s, 1m21s); unit parser tests in `check_fib_depth_test.go` PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/sysrib/{nhresolver,nhresolver_test,ecmp,ecmp_test}.go` | Done | Present under `internal/component/sysrib/`; `// Design:` headers repointed to `docs/architecture/core-design.md` |
| `internal/component/sysrib/events/{events,events_test}.go` | Done | Rich fields (`RouteType`, `TableID`, `Labels`, `SRv6SID`, `ECMPPaths`) |
| `internal/component/bgp/plugins/rib/bestpath.go` | Done | Step-6 IGP cost with verbatim RFC 4271 Section 9.1.2.2(e) quote |
| `internal/plugins/fib/kernel/{fibkernel,backend,backend_linux,nexthop_linux,richroute,pending_retry,doctor_mplsttl_linux}.go` | Done | Rich routes, MPLS, 5 journal fixes (`20d642984c`), `// Design:` repointed |
| `internal/plugins/fib/vpp/{fibvpp,backend,mpls,mplsentry,mplsstale,routesource,mplsforwarding,doctor_route,doctor_srv6,srv6,srv6_state,register,register_doctor}.go` | Done | Multi-path + labeled ECMP (`d7b434442b`), no-store IP/MPLS (`4e3e704cff`), Phase 13 swap/pop/sweeps/show |
| `internal/core/mplsfib/{events,label,refusal,forwarding}.go` | Done | Shared validator, targeted replay, refusals, forwarding reader registry |
| `internal/component/iface/mplsinput.go`, `internal/plugins/iface/{vpp,netlink}/mplsinput*.go` | Done | Whole-set MPLS input enforcement (AC-27) |
| `internal/plugins/connected/register.go` | Done | `OptionalDependencies: ["interface"]` (`beb889e840`) |
| `internal/le/interoplab/bgp/{check_fib_depth,check_fib_depth_test,register_fib_depth}.go`, `test/interop/scenarios/bgp-{ecmp-frr,recursive-nh-frr,igp-cost-bird}/` | Done | Live Docker interop scenarios (`6d04ee6acb`) |
| `internal/le/test/deployment/{vppevidencefib,vppevidencempls,vppevidence,daemonkernel,actions}.go` | Done | Real-VPP `fib-ecmp`, `fib-stale-sweep`, `show mpls forwarding`, `scenario <name>`, `KernelCapabilitiesNone` (`fbd80152a9`) |
| `test/plugin/fib-{ecmp,recursive,blackhole,mpls-kernel,srv6-kernel}.ci`, `test/static/static-kernel-igp-cost-reselect.ci`, `test/vpp/vpp-{fib-ecmp,mpls-ldp-pop,mpls-forwarding-show}.ci` | Done | All present and verified |

### Audit Summary
- **Total items:** 26 active ACs (AC-1..AC-3, AC-5..AC-7, AC-10..AC-29; AC-4, AC-8, AC-9 moved by owner decision)
- **Done:** 26/26 implemented and unit/functional/live-verified (6 ACs — AC-1, AC-2, AC-3, AC-12, AC-13, AC-14 — have unit + `.ci` discrimination + passing live Docker interop runs on `6d04ee6acb`, with live revert-and-rebuild interop discrimination blocked externally by another session's mid-refactor `plugin {} -> bus {}` working tree)
- **Partial:** 0 (none)
- **Skipped:** 0 (none)
- **Changed:** 4 owner-approved scope splits (AC-4, AC-8, AC-9/TableID -> `spec-fib-nexthop-objects-vpp-metric.md`; LFA backup takeover -> `spec-fib-lfa-backup-takeover.md`)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| IGP cost tiebreaker works | unit + functional + live interop | `TestIGPCostChangeReselectsWithoutAIGP` + `test/static/static-kernel-igp-cost-reselect.ci` (discriminated) + `bgp-igp-cost-bird` (`6d04ee6acb`, 2026-11-24 PASS 1m21s vs BIRD 3.4.0, Phase 1 + Phase 2 reload; live revert discrimination blocked by mid-refactor `bus {}` working tree) |
| Recursive NH resolution | unit + functional + live interop | `TestRecursiveNHResolve_*` + `test/plugin/fib-recursive.ci` + `test/plugin/rfc4271-next-hop-kernel.ci` (discriminated) + `bgp-recursive-nh-frr` (`6d04ee6acb`, 2026-11-24 PASS 2m6s vs FRR 10.7.1, Phase 1 + Phase 2 cascade; live revert discrimination blocked by mid-refactor `bus {}` working tree) |
| ECMP installed in FIB | unit + functional + VPP stub + live real-VPP + live interop | `test/plugin/fib-ecmp.ci` (discriminated) + `test/plugin/sysrib-ecmp-replay.ci` + `test/vpp/vpp-fib-ecmp.ci` (`d7b434442b`, 80/80 stress zero-withdrawal) + live real-VPP `fib-ecmp` (`fbd80152a9`, 3/3 PASS vs VPP v26.06) + `bgp-ecmp-frr` (`6d04ee6acb`, 2026-11-24 PASS 1m9s vs FRR 10.7.1; live revert discrimination blocked by mid-refactor `bus {}` working tree) |
| Rich route attributes in kernel | functional + integration | `test/plugin/fib-blackhole.ci` (discriminated) + `test/plugin/fib-mpls-kernel.ci` (discriminated) + `srv6_integration_linux_test.go` + `fib-kernel` 5 journal fixes (`20d642984c`, `fibkernel_journal_fixes_test.go` + `backend_journal_fixes_linux_test.go`) |
| Linux/VPP parity | stub functional + UI fixture + live real-VPP v26.06 | `test/vpp/` 9/9 PASS (`4e3e704cff`, `d7b434442b`); `test/ui/le-evidence-vpp-answers.ci` PASS (`1/1`, 31.0s); live real VPP v26.06 (`fbd80152a9`, 2026-11-24 PASS exit 0): `mpls-label-pop` 7/7 (incl. `show mpls forwarding`), `fib-ecmp` 3/3 (2-path ECMP + member withdraw + full withdraw), `fib-stale-sweep` 5/5 (SIGKILL orphan swept after restart while live control survives) |
| Label-switching entries reach VPP (Phase 13) | real-VPP deployment scenario + stub functional test | AC-28: `mpls-label-pop` passed against real VPP v26.06 (`ligato/vpp-base:26.06-release`) in `./le test deployment vpp` (2026-11-11 5/5 + 2026-11-24 `fbd80152a9` 7/7 incl. `ze show mpls forwarding`). `test/vpp/vpp-mpls-ldp-pop.ci` committed: red with fib-vpp's `mpls-fib` subscription removed, green restored, 80 stress invocations clean. Commits `01250d9b63`, `2f988033a8`, `748b173c1b` |
| VPP holds only what the current run wants, and an operator sees it (Phase 13 follow-up) | unit tests + stub functional test + live real-VPP v26.06 | `b1d5f8aabf`: `ze-fib-vpp` FIB source + stale IP sweep (`routesource.go`), doctor `fib-vpp-stale-routes`; `9318a687d8`: data-plane `show mpls forwarding` reader (`vpp-mpls-forwarding-show.ci` discriminated, 80/80 stress); `4e3e704cff`: plain IP/MPLS without persistent store (`doctor_srv6_test.go`, `test/vpp/` 9/9 PASS); `fbd80152a9`: live real-VPP v26.06 `fib-stale-sweep` (5/5: SIGKILL orphan removed after 30s sweep while live control survives) and `mpls-label-pop` (`mpls-forwarding-show` + `mpls-forwarding-removed` PASS) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | Pending main-thread decision on Rung-2 interop discrimination blocker (`./le spec review record` owed once live interop revert discrimination runs or owner waives) |
| `./le spec review check` | Pending artifact recording |
| Rounds | 1 (code review CLEAN: 0 code BLOCKER, 0 code ISSUE; 2 doc ISSUEs fixed inline; 1 external evidence BLOCKER flagged) |
| Reviewer lenses used | logic+wiring, security+allocation+edge-cases, Go style 7-question pass (`docs/contributing/ze-go-style.md`), spec completeness (`/ze-review-spec`), design documents, automated pre-checks (`./le repo check`, `./le test weakened check base origin/main`, `./le doc check links`, `./le spec citation`) |

### Run 1 (initial -- independent closure review of all 10 commits `4e3e704cff`..`1197f0a16c`)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER (evidence / Rung 2) | Live revert-and-rebuild discrimination (`ai/rules/interop-and-goal-validation.md`) has not been run on the 3 newly added Docker BGP interop scenarios (`bgp-ecmp-frr`, `bgp-recursive-nh-frr`, `bgp-igp-cost-bird`; all passed live on `6d04ee6acb`, unit + `.ci` discrimination recorded). Currently blocked by another session's uncommitted `plugin {} -> bus {}` refactor in `internal/component/plugin/server/` breaking fresh `ze` builds (`unknown top-level keyword: plugin`). | `docs/architecture/testing/interop.md:5300,5338,5339`, `test/interop/scenarios/bgp-{ecmp-frr,recursive-nh-frr,igp-cost-bird}/ze.conf` | Flagged honestly to main thread: wait for `bus {}` refactor to land, run the 3 revert-red/restore-green checks, or obtain owner waiver before Commit B |
| 2 | ISSUE (docs) | `docs/architecture/testing/interop.md` still stated `./le test deployment vpp` runs "nine scenarios" and listed only 5 `mpls-label-pop` checks after `fbd80152a9` added `scenario <name>`, `mpls-forwarding-show`/`mpls-forwarding-removed`, `fib-ecmp`, and `fib-stale-sweep`. | `docs/architecture/testing/interop.md` | Fixed in closure commit A |
| 3 | ISSUE (docs) | `docs/architecture/core-design.md` (`### FIB VPP`) still described `IPRouteAddDel` v1, implicit MPLS table 0 creation, and single-path MPLS push dispatch instead of `ip_route_add_del_v2` (`ze-fib-vpp`), explicit `MplsTableAddDel`, labeled ECMP via `addRichRoute`/`applyPathLabels`, `(mpls-fib, entry)`, and no-store IP/MPLS programming. | `docs/architecture/core-design.md` | Fixed in closure commit A |
| 4 | NOTE | AC-15 triggered a `./le spec citation` line-token drift warning on a line-numbered reference to `plan/records/ruling-interop-scoped-kernel-capabilities.md`. | AC-15 row in this spec | Fixed in closure commit A (cited item 3 without line number) |
| 5 | NOTE | Go style 7-question pass (`docs/contributing/ze-go-style.md`) ran over all changed Go files across all 10 commits + 4 repointed Go files: 0 peer-reachable `panic()` (closed `routeaction.Verb()` only), all loops/retries/queues/caches bounded (`maxPendingEntries=10000`, `defaultRetryMaxAttempts=10`, `maxRecursionDepth=8`, `MaxECMPPaths=128`, `labelFindingsMax`, `rpc.StateListMax`), 0 unwired exported symbols introduced by our commits, 0 test weakenings in our commits, `gopls check` and `./le go style check path` clean on all 4 touched Go files. | All 10 commits `4e3e704cff`..`1197f0a16c` + 4 `// Design:` repoints | Recorded (no code fix needed) |

### Fixes applied
- Updated `docs/architecture/testing/interop.md:867-938` (eleven VPP scenarios, `scenario <name>`, 7 `mpls-label-pop` checks, `fib-ecmp`, `fib-stale-sweep`, verified source anchors).
- Updated `docs/architecture/core-design.md:3647-3676` (`### FIB VPP`, verified source anchors).
- Repointed 4 Go `// Design:` headers (`ecmp.go`, `nhresolver.go`, `richroute.go`, `nexthop_linux.go`) to `docs/architecture/core-design.md`.
- Cleared all 14 remaining tracked prose citers of `plan/release/validation/spec-fib-depth.md` (zero hits in `git grep`).
- Fixed line 735 `spec citation` drift warning.
- Created `website/changes/fragments/fib-depth.md`.

### Final status
- [x] Code review across all 10 commits shows 0 code BLOCKER, 0 code ISSUE (style pass + security pass + wiring pass + `/ze-review-spec` complete)
- [ ] Evidence gate: live revert-and-rebuild interop discrimination on `bgp-ecmp-frr`, `bgp-recursive-nh-frr`, `bgp-igp-cost-bird` blocked by another session's mid-refactor `plugin {} -> bus {}` working tree (`ai/rules/interop-and-goal-validation.md`)
- [x] All NOTEs recorded above

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/sysrib/{nhresolver,nhresolver_test,ecmp,ecmp_test}.go` | Yes | `ls -1` confirmed all 4 files present |
| `internal/plugins/fib/kernel/{pending_retry,fibkernel_journal_fixes_test,backend_journal_fixes_linux_test,doctor_mplsttl_linux,doctor_mplsttl_linux_test}.go` | Yes | `ls -1` confirmed all 5 files present |
| `internal/plugins/fib/vpp/{mplsentry,mplsstale,routesource,routesource_test,doctor_route,doctor_srv6,doctor_srv6_test,ecmp_wire_test,mplsforwarding,mplsforwarding_test,register_mpls_forwarding,rfc3032_mplsentry_test}.go` | Yes | `ls -1` confirmed all 12 files present |
| `internal/core/mplsfib/{events_test,forwarding,forwarding_test,refusal,refusal_test}.go` | Yes | `ls -1` confirmed all 5 files present |
| `internal/component/iface/mplsinput{,_test}.go`, `internal/plugins/iface/{vpp/mplsinput,netlink/mplsinput_linux}{,_test}.go` | Yes | `ls -1` confirmed all 6 files present |
| `internal/le/interoplab/bgp/check_fib_depth{,_test}.go`, `test/interop/scenarios/bgp-{ecmp-frr,recursive-nh-frr,igp-cost-bird}/ze.conf` | Yes | `ls -1` confirmed all 5 files present |
| `internal/le/test/deployment/{vppevidencefib,vppevidencefib_test,vppevidencempls,vppevidencempls_test,vppevidencestop_test}.go` | Yes | `ls -1` confirmed all 5 files present |
| `test/plugin/fib-{blackhole,ecmp,mpls-kernel,recursive,srv6-kernel}.ci`, `test/plugin/{rfc4271-next-hop-kernel,sysrib-ecmp-replay}.ci`, `test/static/static-kernel-igp-cost-reselect.ci`, `test/vpp/vpp-{fib-ecmp,mpls-forwarding-show,mpls-ldp-pop}.ci`, `test/ui/le-evidence-vpp-answers.ci` | Yes | `ls -1` confirmed all 12 `.ci` files present |
| `website/changes/fragments/fib-depth.md` | Yes | Written in closure per `website/changes/fragments/README.md` and `website/data/topics.json` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-13 | Step-6 IGP cost tiebreaker & cost-change reselection | `bestpath.go:451-468` (`BestStepIGPCost`), `rfc4271_igp_cost_reselect_test.go:53`, `test/static/static-kernel-igp-cost-reselect.ci`, live `bgp-igp-cost-bird` PASS (`6d04ee6acb`) |
| AC-2, AC-12 | Recursive NH resolution (max depth 8) & withdrawal cascade | `nhresolver.go:15,78` (`maxRecursionDepth = 8`), `nhresolver_test.go:38-211`, `test/plugin/fib-recursive.ci`, `rfc4271-next-hop-kernel.ci`, live `bgp-recursive-nh-frr` PASS (`6d04ee6acb`) |
| AC-3, AC-5, AC-14 | ECMP grouping (max 128), VPP multi-path + labeled ECMP, member-fail without withdraw | `events.go:65` (`MaxECMPPaths = 128`), `ecmp.go:245`, `fib/vpp/backend.go:120-165` (`applyPathLabels`), `ecmp_wire_test.go:51`, `test/vpp/vpp-fib-ecmp.ci` (80/80), real-VPP `fib-ecmp` (3/3, `fbd80152a9`), live `bgp-ecmp-frr` PASS (`6d04ee6acb`) |
| AC-6, AC-7, AC-10, AC-11 | Rich route types, MPLS push, SRv6 encap & no-store IP/MPLS | `nexthop_linux.go`, `fibkernel_test.go:660-778`, `test/plugin/fib-blackhole.ci`, `test/plugin/fib-mpls-kernel.ci`, `fib/vpp/doctor_srv6_test.go:84` (`4e3e704cff`) |
| AC-15 | Kernel/VPP backend parity + `connected` optional `interface` + `fib-kernel` 5 fixes | `test/vpp/` (9/9), `test/ui/le-evidence-vpp-answers.ci` (1/1), live real-VPP `mpls-label-pop` (7/7), `fib-ecmp` (3/3), `fib-stale-sweep` (5/5, `fbd80152a9`); `fibkernel_journal_fixes_test.go` (`20d642984c`); `connected/register.go` (`beb889e840`) |
| AC-16..AC-29 | Phase 13 VPP label switching, `NeedsDataPlane`, uniform TTL, replay, MPLS input, real-VPP pop | `rfc3032_mplsentry_test.go:110-384`, `mplsstale_test.go:77-177`, `all_test.go:350`, `iface/vpp/mplsinput_test.go`, `test/vpp/vpp-mpls-ldp-pop.ci`, `test/vpp/vpp-mpls-forwarding-show.ci`, live real-VPP `mpls-label-pop` (7/7) |

### Wiring Verified (end-to-end)
| Entry Point | .ci / Proof File | Verified |
|-------------|------------------|----------|
| BGP UPDATE -> sysrib NH resolution + IGP cost | `test/static/static-kernel-igp-cost-reselect.ci`, `test/plugin/fib-recursive.ci`, `test/interop/scenarios/bgp-igp-cost-bird/ze.conf`, `bgp-recursive-nh-frr/ze.conf` | Yes |
| Equal-cost BGP paths -> sysrib ECMP -> kernel & VPP FIB | `test/plugin/fib-ecmp.ci`, `test/plugin/sysrib-ecmp-replay.ci`, `test/vpp/vpp-fib-ecmp.ci`, `bgp-ecmp-frr/ze.conf`, `vppevidencefib.go` (`fib-ecmp`) | Yes |
| Rich routes (blackhole/unreachable/prohibit, MPLS push, SRv6) | `test/plugin/fib-blackhole.ci`, `test/plugin/fib-mpls-kernel.ci`, `test/plugin/fib-srv6-kernel.ci` | Yes |
| `ldp {}` + `backend vpp` (no `fib {}`) -> `fib-vpp` label pop + `show mpls forwarding` | `test/vpp/vpp-mpls-ldp-pop.ci`, `test/vpp/vpp-mpls-forwarding-show.ci`, `vppevidencempls.go` (`mpls-label-pop` 7/7) | Yes |
| `connected {}` + `bgp {}` without `interface {}` | `internal/plugins/connected/register.go` (`OptionalDependencies: ["interface"]`), `optional_edges_without_needs_test.go` | Yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 (NH recursion & ECMP bounds) | confirmed | `maxRecursionDepth = 8` (`nhresolver.go:15`) and `MaxECMPPaths = 128` (`events.go:65`, `ecmp.go:245`) hold in code and unit tests |
| A-2 (VPP v26.06 MPLS API behavior: `mr_eos_proto`, Implicit NULL, table 0 creation, refcounted `sw_interface_set_mpls_enable`) | confirmed (with TTL limit recorded) | Confirmed against FDio/vpp v26.06 source (2026-11-11 Decision) and live real-VPP `mpls-label-pop` 7/7 (`fbd80152a9`); penultimate-pop TTL limit recorded as RFC3032-2.4.3-2 `{gap}` |
| A-3 (AC-4 nexthop objects, AC-8 VPP metric, AC-9/TableID producer) | broken / moved by owner | Moved 2026-10-08 to `plan/post-release/features/spec-fib-nexthop-objects-vpp-metric.md` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/core-design.md` (`### FIB Kernel`, `### FIB VPP`, `### System RIB`) | Anchors verified against `nhresolver.go`, `ecmp.go`, `fibkernel.go`, `nexthop_linux.go`, `richroute.go`, `fib/vpp/{fibvpp,backend,routesource,mpls,doctor_srv6,register}.go`; 4 Go `// Design:` headers repointed here | Yes |
| `docs/architecture/testing/interop.md` (rows 34/70/71 + real VPP proofs) | Updated for eleven VPP scenarios, `scenario <name>`, 7 `mpls-label-pop` checks, `fib-ecmp`, `fib-stale-sweep` anchored to `vppevidencempls.go`, `vppevidencefib.go`, `vppevidence.go`, `vppevidencescenarios.go` | Yes |
| `docs/architecture/fib/{fib-depth-4-srv6,fib-vpp-route-ownership}.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/guide/{plugins,mpls,redistribution}.md`, `docs/architecture/api/commands.md` | Verified against commits `4e3e704cff`, `20d642984c`, `d7b434442b`, `beb889e840`, `b1d5f8aabf`, `9318a687d8` | Yes |
| Tracked citations of `plan/release/validation/spec-fib-depth.md` | `git grep -n 'plan/release/validation/spec-fib-depth.md'` returned ZERO hits after repointing 4 Go headers and 14 prose files | Yes |
| Operator news fragment | `website/changes/fragments/fib-depth.md` created per `website/changes/fragments/README.md` (`topic: Kernel`, `feature: route-installation`) | Yes |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-3, AC-5..AC-7, AC-10..AC-29 all implemented and unit/functional/live-verified (AC-4/8/9 moved by owner)
- [x] Wiring Test table complete
- [ ] `/ze-review` gate clean (code review CLEAN: 0 code BLOCKER/ISSUE; live interop revert discrimination on 3 BGP scenarios blocked by working-tree `bus {}` refactor)
- [ ] `./le verify worktree` passes (GitHub CI release check; agents never run `./le verify` per `ai/META-DECISIONS.md`)
- [x] Feature code integrated
- [x] Integration completeness proven end-to-end
- [x] Architecture docs updated
- [x] Critical Review passes (0 code BLOCKER/ISSUE)

### Quality Gates (SHOULD pass)
- [x] RFC constraint comments added
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction (3+ use cases?)
- [x] No speculative features (needed NOW?)
- [x] Single responsibility per component
- [x] Explicit > implicit behavior
- [x] Minimal coupling

### TDD
- [x] Tests written
- [x] Tests FAIL (unit + `.ci` discriminated; live BGP interop revert discrimination pending `bus {}` tree)
- [x] Tests PASS
- [x] Boundary tests for all numeric inputs
- [x] Functional tests for end-to-end behavior
- [x] Interop tests for protocol features
- [x] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [x] Critical Review passes (code CLEAN)
- [x] Partial/Skipped items have user approval (AC-4/8/9 and LFA spin-out owner-approved)
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Lessons routed to governing surfaces (journal rows + docs; no separate `plan/learned/` summary required per `ai/rules/planning.md`)
- [x] Operator change fragment written (`website/changes/fragments/fib-depth.md`)
