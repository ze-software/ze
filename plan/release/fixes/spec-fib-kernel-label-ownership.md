# Spec: fib-kernel-label-ownership

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Incorrect shipped behavior in routing: `fib-kernel` replaces or deletes a main-table route its other owner installed, and a new `fib-kernel` refuses and never sweeps the label entries its own previous run left (`plan/README.md`, `release/fixes/`: routing and lifecycle defects). The defects exist on every box that runs a label source over the kernel data plane, with or without `fib dummy` |
| RFC gaps | - |
| Depends | None to start. `plan/release/features/spec-fib-dummy.md` depends on this spec for every outcome that needs a previous run's label entries taken back (its AC-21 on a box with leftover labels: kernel to dummy to kernel with `flush-on-stop` false, and `request plugin restart fib-kernel`). That spec adds `fib-kernel`'s targeted label replay at open; this spec does not need it for its own acceptance criteria, which drive the label sources' live applies and replays through `handleMPLSEntry`. Coordination: both specs edit `internal/plugins/fib/kernel/fibkernel.go`, `mplsentry.go` and `register.go`; whichever lands second re-reads them |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`fib-kernel` is the one writer of the kernel FIB, and two owners feed it: the
system RIB (best-change events) and the label sources (LDP, RSVP-TE, OSPF
segment routing, BGP EPE, through `(mpls-fib, entry)` events). A table-0 push
and a system-RIB route for the same prefix are both protocol-250 IP routes in
the main table, so the kernel cannot tell the two owners apart. `fib-kernel`
must, and today it does not. Six defects follow, all read at the producing
functions on 2026-11-24 and none of them run:

| ID | Symptom | Root cause at the producer |
|----|---------|----------------------------|
| D-1 | A label source's push replaces a live labeled system-RIB route (BGP labeled-unicast) of the same prefix, while the system-RIB record still lists that route as installed | `addMPLSPushLocked` (`mplscontext.go`) chooses replace over exclusive add on `f.mplsInstalled`, and `recordInstalledLocked` (`fibkernel.go`) sets the same map for every system-RIB install that carries labels. One map answers for two owners. The reverse also holds: a system-RIB withdraw or unlabeled replace of the prefix clears the map (`processEvent`, `recordInstalledLocked`), so the push's own relabel is then an exclusive add that meets its own route |
| D-2 | Withdrawing a push that was never installed deletes the system RIB's route for the FEC, on a fresh box too | `delMPLSEntryLocked` (`mplsentry.go`) calls `delRichRoute(FEC, 0)` for every table-0 push removal without asking whether a push was installed. `ldpFIB.ProgramPush` records the FEC as wanted before the writer answers, so `ldpFIB.Remove` withdraws a push whose add was refused. `delRichRoute` deletes by prefix, protocol 250 and table (`ownedRouteKey`), which the system RIB's route matches |
| D-3 | A push relabel deletes the system RIB's route of the same prefix at another metric, and a system-RIB replace, withdraw or stale sweep removes or overwrites the push | `replaceRichRoute` goes through `replaceOwnedRoute` (`backend_linux.go`), which after the write deletes every other protocol-250 route of the prefix in the table. The system RIB installs at its protocol metric (`fibChange`, `internal/component/sysrib/fibimport.go`; `buildRichRoute` sets the kernel priority from it) and a push carries no metric, so the normal LDP case, an IGP route at its metric beside a push at priority 0, is exactly the pair this cleanup breaks. `delRichRoute` and `delListedLocked` name no priority, so a system-RIB withdraw and the stale sweep delete whichever protocol-250 route of the prefix the kernel matches first |
| D-4 | A new `fib-kernel` refuses the swap and pop entries its own previous run left, and never removes the ones no source wants any more | `addMPLSEntryLocked` asks for a replace only for an in-label in `f.mplsSwaps`, which a new process holds empty, so `addMPLSSwap` calls `RouteAdd` and the kernel answers `EEXIST`. `listZeRoutes` skips every route with no IP destination, so `startupSweep` never records an AF_MPLS entry and `sweepStale` never removes one |
| D-5 | A new `fib-kernel` refuses the table-0 push its own previous run left until the IP sweep removes it, and nothing applies it again afterwards | `startupSweep` records the leftover push like any protocol-250 route, but `addMPLSPushLocked` consults only `f.mplsInstalled`, so the re-applied push is an exclusive add over Ze's own leftover |
| D-6 | With `flush-on-stop` true, a stopping `fib-kernel` leaves the label sources' pushes and AF_MPLS entries in the kernel | `flushRoutes` builds its owned set from `f.installed`, `f.retained`, `f.staleTables` and `f.supersededTables`; a label-source push lives in `f.mplsInstalled` alone and a swap or pop in `f.mplsSwaps` alone |

The fix gives each owner a record of its own and makes every kernel write name
the slot it owns:

1. Label-source pushes get their own record, apart from the system RIB's.
   Replace, relabel and removal of a push are gated on that record alone.
2. Every write and delete names the route's kernel slot (table and priority).
   The cleanup after a replace removes only routes the caller's own record
   lists, never "every other protocol-250 route of the prefix".
3. The leftovers of a previous run are recorded with their slot, AF_MPLS
   entries included, and an install into a recorded slot takes Ze's own
   leftover back by replace. What no owner takes back is swept, as today.
4. A push identical to the recorded one is acknowledged without a kernel
   write.
5. The stop flush removes what both records and both leftover records hold.

The design is Thomas's to approve. He has not been asked about it yet; the main
session split this work out of `spec-fib-dummy.md` on 2026-11-24 after its
seventh design judge refused the leftover-label design there.

## Required Reading

- [ ] `ai/doors/system.md` - the design every subsystem shares
  → Constraint: always-on code never imports a gated package; `fib-kernel` is always on and `internal/component/mpls` is gated by `ze_mpls`
- [ ] `ai/doors/forwarding.md` - FIB writers and data planes
  → Constraint: "Ze owns only what it marked. It owns routes by `rtm_protocol`"; `fib-kernel` "reasserts only what it owns"; "A reconciler that reads 'not in config' as 'mine to delete' breaks them"
  → Constraint: MPLS label entries reach a data plane only as `(mpls-fib, entry)` events its writer consumes
- [ ] `ai/doors/routing-protocols.md` - the label sources
  → Constraint: label entries go through the bus, and each producer answers a writer's replay request with `mplsfib.ApplyReplay`
- [ ] `ai/doors/core.md` - `internal/core/mplsfib`
  → Constraint: core imports neither components nor plugins; this spec adds nothing to core
- [ ] `docs/architecture/mpls/mpls-kernel.md` - "Constraint: the push shares the FIB with other writers", "Label routes a previous run left in VPP", "Doctor checks"
  → Decision: "A first install therefore uses `RouteAdd` and fails with `EEXIST` rather than clobbering that route. Only a genuine relabel of Ze's own push uses `RouteReplace`"
  → Decision: "An `AF_MPLS` label is owned by its distribution source ... A replacement or withdrawal must match the recorded source ... The backend also checks kernel ownership before replacing or deleting an installed label"
  → Decision: `fib-vpp` removes a previous run's label routes "with fib-kernel's stale-mark-then-sweep (`startupSweep`, `sweepStale`) applied to the label table": record at open, an in-label a source installs is the source's from then on, sweep the rest after the delay
- [ ] `docs/architecture/core-design.md` - section 16, the FIB pipeline, and `## Design decisions`
  → Constraint: the system RIB is the one arbiter of IP routes; a push bypasses it by design (`mpls-kernel.md`)
- [ ] `rfc/short/rfc4724.md` - Section 4.1, the restarting speaker's forwarding state
  → Constraint: RFC4724-4.1-3 is tagged on `rfc4724_restarting_fib_test.go` with the claim that `startupSweep` "performs zero backend deletes, replacements, or additions"; the label listing this spec adds at open reads and writes nothing

## Current Behavior

**Source files read:**
- [ ] `internal/plugins/fib/kernel/mplscontext.go` - `addMPLSPushLocked`: a table-0 push is `replaceRichRoute` when `f.mplsInstalled` holds the prefix and `addRichRoute` otherwise; success sets `f.mplsInstalled`. The route it builds carries no metric
- [ ] `internal/plugins/fib/kernel/mplsentry.go` - `handleMPLSEntry` takes `f.mu` for the whole batch and acknowledges after releasing it; `addMPLSEntryLocked` passes `installed` from `f.mplsSwaps` as the replace flag and refuses an in-label another source holds; `delMPLSEntryLocked` deletes a table-0 push with `delRichRoute(FEC, 0)` unconditionally, and a swap or pop only when `f.mplsSwaps` holds it for that source; `delMPLSSourceLocked` walks `f.mplsSwaps` only; `mplsCountLocked` adds `f.mplsInstalled` and `f.mplsSwaps`
- [ ] `internal/plugins/fib/kernel/mplsentry_linux.go` - `addMPLSSwap` asks `mplsSwapOwned` only when asked to replace, otherwise `RouteAdd`; `delMPLSSwap` asks `mplsSwapOwned` first and removes nothing it does not own; `mplsSwapOwned` reads the whole AF_MPLS dump and refuses an in-label another protocol holds
- [ ] `internal/plugins/fib/kernel/fibkernel.go` - the `fibKernel` fields `installed`, `forwarding`, `retained`, `staleTables`, `supersededTables`, `mplsInstalled`, `mplsSwaps`; `ownsSlotLocked` (answers by prefix and table, with no priority); `recordInstalledLocked` (sets `f.mplsInstalled` when the change carries labels, clears it on an unlabeled replace); `processEvent` (a withdraw calls `delChange` unconditionally and clears `f.mplsInstalled`); `startupSweep` (records prefix and table of each listed route); `sweepPrefixLocked`, `sweepStale` (holds `f.mu` for its whole body); `flushRoutes`; `sweepFix`, `fibSweepDiagnosticCode`; `run`; `waitAndSweepStale`
- [ ] `internal/plugins/fib/kernel/backend_linux.go` - `replaceOwnedRoute`: lists the routes of the prefix in the table, replaces when a protocol-250 route sits at the same priority, adds exclusively otherwise, then deletes every other protocol-250 route of the prefix; `ownedRouteKey`: prefix, protocol 250, table, any scope, no priority; `listZeRoutes`: every protocol-250 route with an IP destination, with prefix, table and gateway, skipping the private push-context tables
- [ ] `internal/plugins/fib/kernel/nexthop_linux.go` - `addRichRoute` (`RouteAdd`), `replaceRichRoute` (`replaceOwnedRoute`), `delRichRoute` (`ownedRouteKey`, `delOwnedRoute`), `buildRichRoute` (the kernel priority is the route's metric)
- [ ] `internal/plugins/fib/kernel/fibkernel_replay.go`, `backend_replay_linux.go` - `tryReplay`, `reassertRichRoute`: the system RIB's identical re-install is reasserted after a kernel probe, IPv4 only
- [ ] `internal/plugins/fib/kernel/register.go` - `OnStarted` calls `startupSweep`, starts `run`, and starts `waitAndSweepStale` only when the stale record holds a route
- [ ] `internal/component/sysrib/fibimport.go` - `fibChange` sets the change's metric from the protocol route's metric
- [ ] `internal/plugins/ldp/fib.go` - `ProgramPush` records the push as wanted, then emits; `Remove` emits a removal whenever the FEC is wanted, whether or not a writer accepted the add; `replay` runs on the requesting writer's goroutine and takes the LDP lock, and its comment requires that the writer does not hold its own entry-handler lock while it emits the request
- [ ] `internal/plugins/fib/vpp/mplsstale.go` - `markStaleLabelsLocked`, `reclaimStaleLabelLocked`, `sweepStaleLabels`: the label-table form of stale-mark-then-sweep
- [ ] `internal/plugins/fib/kernel/rfc3032_mplsentry_test.go`, `rfc4724_restarting_fib_test.go`, `mplsowner_integration_linux_test.go`, `mplsapply_test.go` - the tests that drive the touched functions; the first two carry `RFC requirement:` tags (RFC3032-2.1-1; RFC4724-4.1-1, -4.1-2, -4.1-3, -4.1-6)

**Behavior to preserve:**
- A first install into a slot `fib-kernel` holds no record for is an exclusive add, and a foreign route there refuses it with `EEXIST` (`mpls-kernel.md`).
- When a push and a system-RIB route want the same slot (main table, same priority), the first one installed holds it and the second is refused with `EEXIST` and reported by its owner (the label source's doctor check, or `fib-sync-failure` and the pending retry).
- A push and a system-RIB route of one prefix at different priorities coexist; the kernel forwards by the lower priority.
- A swap or pop belongs to its source; another source's add or removal of that in-label is refused.
- `flush-on-stop` false keeps every route and label entry in the kernel at stop.
- `startupSweep` writes nothing to the kernel, and the sweep waits for BGP's deferred selection as `waitAndSweepStale` does today.
- The private push contexts (nonzero table) keep their own lifecycle in `mplscontext_linux.go`.
- A push from a second label source for a FEC another source's push holds replaces it, as today: which source wins a FEC two sources bind is not decided here (Known limits).

**Behavior to change:**
- D-1 to D-6 of the Task table.

## Existing Mechanisms

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The system RIB's owner record | `fibkernel.go` `fibKernel.installed`, `forwarding`; `recordInstalledLocked`, `ownsSlotLocked` | Records each prefix the system RIB installed with the change it was installed from, table and metric included; `ownsSlotLocked` answers by prefix and table | Extends: the slot it answers for gains the priority, read from the recorded change's metric, so "the system RIB owns this slot" is true only for the route it installed. No new field: the metric is already in `forwarding` |
| Source ownership of swap and pop | `fibkernel.go` `fibKernel.mplsSwaps`; `mplsentry.go` `addMPLSEntryLocked`, `delMPLSEntryLocked`, `delMPLSSourceLocked`; `mpls-kernel.md`, "the push shares the FIB with other writers" | Records the source of each in-label; replace and removal must match it; the backend rechecks the kernel | Extends: the label-source push gets the same kind of record (the label-push record, keyed by FEC, holding the applied entry and its source), and push replace and removal are gated on it as swap and pop are gated on `mplsSwaps`. The pattern is copied, not invented |
| The shared labeled-prefix set | `fibkernel.go` `fibKernel.mplsInstalled`; set by `recordInstalledLocked` and by `addMPLSPushLocked` | One set that says "this prefix carries labels" for the gauge and also decides whether a push may replace | Replaces its ownership use: the label-push record takes it, and `mplsInstalled` keeps only what `recordInstalledLocked` writes, the labeled system-RIB prefixes the gauge counts. `addMPLSPushLocked` and `delMPLSEntryLocked` stop reading and writing it. `mplsCountLocked` adds the label-push record |
| Check-then-write replace | `backend_linux.go` `replaceOwnedRoute` | Looks up the slot, replaces a protocol-250 route at the same priority or adds exclusively, then deletes every other protocol-250 route of the prefix in the table | Extends: the caller hands it the other priorities its own record lists for the prefix in that table (the previous metric of a metric change, the leftover and superseded listings), and the cleanup deletes those alone. A push hands none. The same-priority check and the exclusive add are kept |
| The owned-route delete key | `backend_linux.go` `ownedRouteKey`, `delOwnedRoute`; `nexthop_linux.go` `delRichRoute` | Deletes by prefix, protocol 250, table and any scope, with no priority, so the kernel removes the first route that matches | Extends: the key carries the priority of the route the caller's record holds, and `delRichRoute` and `delListedLocked` take it. A delete for a prefix the caller's record does not hold writes nothing |
| Stale-mark-then-sweep for IP routes | `fibkernel.go` `startupSweep`, `staleTables`, `retained`, `forgetStaleTableLocked`, `sweepPrefixLocked`, `sweepStale`, `waitAndSweepStale`; `backend_linux.go` `listZeRoutes` | Lists every protocol-250 IP route at open with prefix and table, lets a system-RIB install into a listed table take the slot, and sweeps the rest | Extends: each listing gains the kernel priority `listZeRoutes` already reads off the dump. A system-RIB install and a push add both take a listed slot back by replace, and the sweep deletes each remaining listing by its own table and priority |
| Stale-mark-then-sweep for label routes | `internal/plugins/fib/vpp/mplsstale.go` `markStaleLabelsLocked`, `reclaimStaleLabelLocked`, `sweepStaleLabels`; `mpls-kernel.md`, "Label routes a previous run left in VPP" | `fib-vpp` records the label table at open, forgets an in-label a source installs, sweeps the rest after the delay | Extends to `fib-kernel` for its own AF_MPLS table: a listing of protocol-250 in-labels on `mplsBackend`, over the dump `mplsSwapOwned` reads; a stale in-label record; reclaim in `addMPLSEntryLocked`; removal through `delMPLSSwap` when the IP sweep runs. No second mechanism and no second timer |
| The sweep's report | `fibkernel.go` `sweepStale`, `sweepFix`, `fibSweepDiagnosticCode`, code `doctor-fib-sweep-incomplete` | Raises one error when a stale IP route could not be listed or deleted, with the `ip route` commands that remove it | Extends: a label listing or delete the kernel refuses is raised under the same code, and the fix and the code's description gain the AF_MPLS form (`ip -f mpls route show proto 250`, `ip -f mpls route del <label>`) |
| The stop flush | `fibkernel.go` `flushRoutes`, `flushOneLocked`, `flushResidueLocked`, code `doctor-fib-flush-incomplete` | Removes the system RIB's routes and the leftover and superseded listings; reports what it could not remove | Extends: it also removes the label-push record's routes, the in-labels of `mplsSwaps` and the stale in-label record, and counts their failures in the same report |
| The kernel push recogniser | `internal/component/mpls/forwarding_linux.go` `pushEntryFromRoute` | Tells `show mpls forwarding` that a protocol-250 IP route with an MPLS encap is a push | Does not apply. `spec-fib-dummy.md` planned to move it to `internal/core/mplsfib` and decide leftover ownership with it. It cannot decide ownership: a labeled system-RIB route (BGP labeled-unicast) is also a protocol-250 IP route with an MPLS encap, so the recogniser answers "push" for both owners. Ownership of a leftover is decided by slot instead (Design presentation), and the recogniser stays where it is |
| Targeted label replay | `internal/core/mplsfib/events.go` `ReplayRequest`, `ApplyReplay`; `spec-fib-dummy.md` AC-21 | A writer that opens late asks the sources to re-apply; `fib-kernel` does not ask today | Does not apply here: `spec-fib-dummy.md` adds the request to `fib-kernel`. This spec makes the entries that replay delivers land correctly, and adds no emit of its own |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | always | "Always-on code MUST NOT import a gated package." | agrees | Nothing moves between packages: every edit is inside `internal/plugins/fib/kernel`, and the recogniser in the gated `internal/component/mpls` is left alone |
| `ai/doors/forwarding.md` | Who depends on it from outside | the door covering every `internal/plugins/fib/kernel` file | "Ze owns only what it marked." | agrees | The kernel mark (`rtm_protocol` 250) is one for both owners, so `fib-kernel` keeps the finer ownership itself: one record per owner, and no write or delete outside the slot a record holds. A leftover is Ze's by the mark, and is taken back only in the slot it was listed in |
| `ai/doors/le-tooling.md` | Boundaries | the door covering the two new `test/ldp/*.ci` files | "A `.ci` that needs a Linux kernel feature carries `option=needs-linux`" | agrees | Both new `.ci` files read the kernel's IP and AF_MPLS tables and carry `option=needs-linux` |
| `docs/architecture/mpls/mpls-kernel.md` | Constraint: the push shares the FIB with other writers | the `// Design:` page of `mplscontext.go`; anchored to `mplsentry.go`, `mplsentry_linux.go` and `nexthop_linux.go`; a page this spec modifies | "with `EEXIST` rather than clobbering that route. Only a genuine relabel of Ze's" | departs | For a live route the design holds the sentence more strictly than the code does today: a push replaces only when the label-push record holds the FEC, and a live system-RIB route is never replaced (D-1). It departs in one case the page does not name: a push applied over a route a previous run left in the push's own slot takes that leftover back by replace, although the new process cannot know that the leftover was a push and not the previous run's system-RIB route at that priority. Presented under "Documents departed from"; choice C-1 |
| `docs/architecture/mpls/mpls-kernel.md` | Constraint: the push shares the FIB with other writers | the same page, the `AF_MPLS` paragraph | "Ze protocol. Its first installation uses `RouteAdd` and refuses an existing" | agrees | An in-label no record holds is still an exclusive add, and a foreign label still refuses it. An in-label the writer recorded at open under protocol 250 is Ze's own from its previous run, not a foreign label: the source's add takes it back, and `mplsSwapOwned` checks the kernel again before the replace, as the last sentence of that paragraph requires |
| `docs/architecture/mpls/mpls-kernel.md` | Label routes a previous run left in VPP | the same page | "with fib-kernel's stale-mark-then-sweep (`startupSweep`, `sweepStale`) applied" | agrees | The same three steps are applied to `fib-kernel`'s own AF_MPLS table: record at open, an in-label a source installs is the source's from then on, sweep the rest when the IP sweep runs. The page gains the kernel subsection beside the VPP one |
| `docs/architecture/core-design.md` | FIB Kernel | the `// Design:` page of `fibkernel.go`, `backend.go`, `backend_linux.go`; anchored to them, to `richroute.go`, `nexthop_linux.go` and `register.go`; a page this spec modifies | "delete key names Ze's protocol and the route's table and matches any scope" | agrees | The key keeps protocol, table and any scope and gains the priority of the route the caller's record holds, so `ESRCH` still means absent. The sentence is edited to name the priority. The page already requires the monitor to match "prefix, table and priority"; the delete and the replace cleanup are brought to the same slot |
| `docs/architecture/core-design.md` | FIB Kernel | the same page | "an install of the prefix into that table replaces it instead of being refused" | agrees | Kept for the slot the install targets. A leftover at another priority of that table is still cleaned up by the install, except one at the push's slot, which is left for a label source to take back or for the sweep (Data Flow step 5); the paragraph is edited to say so |
| `docs/architecture/rsvpte/mpls-rsvp-te.md` | Dataplane | the `// Design:` page of `mplsentry.go`, `mplsentry_linux.go` and `rfc3032_mplsentry_test.go` | "`fib-kernel` programs the corresponding IP and `AF_MPLS` routes, `fib-vpp` the" | agrees | `fib-kernel` stays the one programmer of both kinds for the kernel plane; the spec changes which of its records answers for each route, not who programs it |
| `docs/architecture/linux-limitations.md` | LL-MPLS-FORWARD-NO-PMTU | anchored to `internal/plugins/fib/kernel/mplsentry_linux.go` | - | silent | The entry is about the path MTU of a transit swap or pop. The in-label listing reads the dump and builds no route, and the reclaim passes the path MTU to `addMPLSSwap` unchanged |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Current Ze State | anchored to `internal/plugins/fib/kernel/fibkernel.go` | - | silent | The page records that FIB backends use the registry model. No metric is added, renamed or moved; the MPLS gauge counts the same entries from two records |
| `docs/architecture/fib/fib-depth-4-srv6.md` | Kernel backend | anchored to `internal/plugins/fib/kernel/nexthop_linux.go` | - | silent | The anchored paragraph is about `wrapSEG6InstallError` on `addRichRoute` and `replaceRichRoute`. Both keep passing their install error through it; only the cleanup set of the replace and the priority of the delete change |
| `docs/architecture/ospf/ospf-ext-6-ti-lfa.md` | Decisions | anchored to `internal/plugins/fib/kernel/nexthop_linux.go` | - | silent | The anchored decision is that `buildRichRoute` sends the primary path alone. `buildRichRoute` is not changed |
| `plan/release/validation/spec-fib-depth.md` | Existing Mechanisms (MANDATORY) | the `// Design:` page of `richroute.go` and `nexthop_linux.go` | - | silent | That spec proves what a rich route carries (type, metric, table, multipath, MPLS push, SEG6). It states nothing on which owner may replace or delete a route. Its owned row of 2026-11-14 on `replaceOwnedRoute` (`supersededDeleteError`, `supersededTables`) is kept: a cleanup delete the kernel refuses is still recorded as superseded |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | The spec adds no config and changes no verdict; it changes what a running `fib-kernel` writes |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | No refusal is added or moved |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | No firewall code or annotation is touched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | The spec names the plane `kernel` and adds no selector |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | No BGP export code is touched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | No policy chain is touched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | No BGP engine step is touched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | No export chain is touched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | No policy code is touched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | No export chain is touched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | The spec inserts no filter-chain member and nothing implied |

## Data Flow

### Entry Point
- A label source applies an entry: `mplsfib.Apply` or `ApplyReplay` emits `(mpls-fib, entry)`, and `handleMPLSEntry` programs the batch under `f.mu` and acknowledges.
- The system RIB publishes `(system-rib, best-change)`, and `processEvent` programs the batch under `f.mu`.
- `fib-kernel` opens: `OnStarted` calls `startupSweep` before `run` subscribes.
- `fib-kernel` stops: `run` calls `flushRoutes` when `flush-on-stop` is true.

### Transformation Path
1. **Open.** `startupSweep` lists the protocol-250 IP routes as today and keeps each listing's table and priority. It then lists the protocol-250 AF_MPLS in-labels through the new `mplsBackend` listing and records each as a stale in-label. Neither listing writes to the kernel. A listing the kernel refuses is raised under `doctor-fib-sweep-incomplete` and leaves that record empty, so nothing of that kind is reclaimed or swept. The sweep worker starts when either record holds an entry.
2. **Push add** (`addMPLSPushLocked`, table 0). The slot is the main table at the priority the kernel gives a route with no metric (0 for IPv4, 1024 for IPv6, the value `replaceOwnedRoute` already uses). In order:
   - The label-push record holds the FEC with an equal entry (source, out-labels, next hop, path MTU): acknowledged, no kernel write.
   - The label-push record holds the FEC with another entry: a relabel, written by a replace of that slot with no other priority to clean up.
   - The leftover record lists that slot for the prefix: Ze's own leftover is taken back by a replace of that slot; the listing leaves the leftover record, and the prefix leaves `retained` once no listing of it remains.
   - Otherwise: an exclusive add. A route in the slot, the system RIB's live one included, refuses it with `EEXIST`.
   On success the label-push record holds the entry. `f.mplsInstalled` is neither read nor written.
3. **Push removal** (`delMPLSEntryLocked`, table 0). The label-push record holds the FEC for that source: the delete names the push's slot, and the record forgets the FEC on success. Otherwise nothing is written and the removal is acknowledged, as a swap or pop removal of an in-label nobody installed already is.
4. **Swap or pop add** (`addMPLSEntryLocked`). An in-label in `f.mplsSwaps` behaves as today. An in-label in the stale in-label record is taken as tracked: the backend is asked to replace, `mplsSwapOwned` checks the kernel again, and on success the in-label leaves the stale record and belongs to the source. Any other in-label is an exclusive add.
5. **System-RIB install or replace** (`programRouteLocked`). `ownsSlotLocked` answers for the slot (prefix, table, priority): true when the system-RIB record holds the prefix in that table, or the leftover or superseded record lists that table, and false when the label-push record holds exactly that slot. A replace hands `replaceOwnedRoute` the other priorities the system RIB's own records list for the prefix in that table (the previous metric of a metric change, the superseded listings, and the leftover listings at any priority but the push's), and only those are deleted. A leftover at the push's slot is never in that set: it may be the previous run's push, so it waits for a label source to take it back or for the sweep, and the restart stays hitless for it.
6. **System-RIB withdraw** (`processEvent`, `delChange`). The delete names the table and priority of the route the system-RIB record or the leftover record holds. A withdraw of a prefix neither record holds writes nothing. `f.mplsInstalled` is cleared as today, which no longer touches the push.
7. **Sweep** (`sweepStale`). Each remaining IP listing is deleted by its table and priority; each remaining stale in-label is removed through `delMPLSSwap`. Failures of both kinds are raised once under `doctor-fib-sweep-incomplete`. The sweep emits no event: it holds `f.mu` for its whole body, and a label source answers a replay request synchronously into `handleMPLSEntry`, which takes `f.mu`.
8. **Stop flush** (`flushRoutes`). Beside today's set it removes each route of the label-push record by its slot, each in-label of `f.mplsSwaps` and each stale in-label, and clears the records.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Label source to `fib-kernel` | `(mpls-fib, entry)` batch and its synchronous acknowledgement; unchanged | Read: `mplsfib.Apply`, `handleMPLSEntry` |
| `fib-kernel` to the kernel | rtnetlink route add, replace, delete and dump through `netlinkBackend`; the delete and the replace cleanup gain a priority | Read: `backend_linux.go`, `nexthop_linux.go`; the kernel's matching of a delete without a priority is A-1 |
| `fibKernel` to its backend | `routeBackend`, `richRouteBackend`, `mplsBackend`; signatures change for delete, replace and the two listings, and every implementation and test double follows | Read: `backend.go`, `richroute.go`, `mplsentry.go` |

### Integration Points
- `mplsBackend` (`mplsentry.go`) gains the in-label listing; `netlinkBackend` implements it in `mplsentry_linux.go` over the dump `mplsSwapOwned` reads, and the non-Linux stub answers an empty list.
- `installedRoute` (`backend.go`) gains the priority; `listZeRoutes` fills it.
- `richRouteBackend` (`richroute.go`): `delRichRoute` takes the priority, and `replaceRichRoute` takes the priorities to clean up.
- No event, RPC, YANG leaf, command or metric name changes. The gauge `ze_fibkernel_mpls_routes_installed` keeps counting labeled system-RIB prefixes, label-source pushes, swaps, pops and private contexts.

## Risks & Assumptions

| ID | Assumption / Risk | Basis / Early signal | Validation / Mitigation | Status |
|----|-------------------|----------------------|-------------------------|--------|
| A-1 | A kernel delete that names prefix, protocol and table with no priority removes the lowest-priority protocol-250 route of the prefix, so today a system-RIB withdraw of an IGP route removes the LDP push beside it | Read, not run: `ownedRouteKey` sets no priority; Linux `fib_table_delete` skips the priority comparison when the request carries none. Not confirmed on Ze's kernel | `TestOwnershipIntegration_WithdrawAtAnotherMetricKeepsPush` in QEMU, red before the fix. If the kernel matches otherwise, the symptom of D-3 changes and the fix (name the priority) stays right | unvalidated |
| A-2 | An IPv4 delete that names priority 0 cannot be told from one that names none, so a push delete is exact only because the push record proves the push is there, and the push is then the lowest-priority route of its prefix | Read: the rtnetlink attribute is omitted at 0; a foreign route at priority 0 would have refused the push at install | `TestOwnershipIntegration_PushRemovalKeepsRouteAtAnotherMetric` in QEMU: after the removal the system RIB's route at its metric is still there | unvalidated |
| A-3 | Every protocol-250 AF_MPLS entry in the kernel was installed by a `fib-kernel` of this box, so a recorded in-label is Ze's to replace or sweep | `ai/doors/forwarding.md`: Ze owns routes by `rtm_protocol`; `addMPLSSwap` and the private contexts are the only AF_MPLS writers found (`grep` of `rtprotZE` users owed at implementation) | `mplsSwapOwned` rechecks the kernel before every replace and delete; `TestMPLSIntegration_ForeignLabelPreserved` stays green | unvalidated |
| A-4 | No label source applies a push for one FEC from two sources in a way that depends on today's "last applied wins" being a replace of the other source's route | Read: `addMPLSPushLocked` has no source check today | Kept as it is (Behavior to preserve); the record stores the source so a removal by the source that lost the FEC writes nothing | accepted |
| R-1 | A leftover system-RIB route at the push's own slot is taken back by a replayed push that arrives before the system RIB's replay, so the prefix forwards labeled where the previous run forwarded it unlabeled | The slot rule treats a leftover as nobody's live route. On a fresh box the same arrival order gives the same result: the first owner holds the slot and the second is refused with `EEXIST` | `TestFIBKernelLabelReplayBeforeSystemRIBReplay` and `TestFIBKernelSystemRIBReplayBeforeLabelReplay` assert the fresh-box outcome of each order. The contested case needs a system-RIB route at the push's priority for a FEC a label source binds, which an IGP route at its metric is not | open |
| R-2 | The priority added to delete and replace changes a signature every backend double implements; a double that ignores it hides D-3 | 30 test files in the package use backend doubles | The shared double records the priority of each delete and the cleanup set of each replace, and the new unit tests assert on them | open |
| R-3 | An identical push acknowledged without a write hides a push something removed from the kernel behind `fib-kernel`'s back | `handleExternalChange` (`monitor.go`) reasserts system-RIB routes; whether it covers pushes is not read | Out of scope and unchanged: today's replace would repair it only by accident of a re-apply. Named in Known limits | accepted |
| R-4 | Another session edits `internal/plugins/fib/kernel/` while this spec waits | The brief of 2026-11-24 says so | Every cited symbol is re-read before the implementation starts; the spec names symbols, not line numbers | open |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| LDP binds a FEC whose BGP labeled-unicast route `fib-kernel` installed | → | `addMPLSPushLocked`, the label-push record | `test/ldp/push-keeps-labeled-system-rib-route.ci` |
| `ze` restarts with `fib kernel { flush-on-stop false; }` and LDP | → | `startupSweep`, the stale in-label record, `addMPLSEntryLocked` reclaim | `test/ldp/restart-reclaims-leftover-labels.ci` |
| LDP withdraws a push the kernel refused | → | `delMPLSEntryLocked` | `TestOwnershipIntegration_RefusedPushRemovalKeepsSystemRIBRoute` (QEMU) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The system RIB installed a labeled route for a prefix in the main table at the push's priority; a label source then applies a push for that FEC | The push is refused with `EEXIST` through the acknowledgement, the kernel still holds the system RIB's route unchanged, the system-RIB record still lists it, and the source's doctor check reports the push (D-1) |
| AC-2 | The system RIB withdraws, or replaces without labels, a prefix a label source's push holds at another priority; the source then relabels the push | The push is still in the kernel after the system-RIB change, and the relabel is a replace of the push's own slot (D-1, reverse) |
| AC-3 | A label source removes a table-0 push whose add was refused, on a box where the system RIB holds the FEC's route, at the push's priority or at another | No kernel write; the system RIB's route is still there; the removal is acknowledged without error (D-2) |
| AC-4 | The system RIB holds a route for a prefix at its protocol metric and a label source's push holds the same prefix at priority 0; the source relabels the push | Both routes are in the kernel afterwards, the push with the new label (D-3) |
| AC-5 | Same two routes; the system RIB changes the route's metric, then withdraws it | After the metric change the kernel holds the push and the route at the new metric only; after the withdraw it holds the push only (D-3) |
| AC-6 | A label source applies a push equal to the one the label-push record holds | Acknowledged without error and without a kernel write |
| AC-7 | `fib-kernel` opens on a kernel holding protocol-250 AF_MPLS entries a previous run left; a source applies a swap or pop for a recorded in-label | The entry is replaced with what the source applied and acknowledged; the in-label belongs to the source and the sweep leaves it (D-4) |
| AC-8 | Same open; `sweep-delay` passes with a recorded in-label no source applied | The entry is removed from the kernel; an in-label another kernel protocol took since the listing is left alone; a delete the kernel refuses is raised under `doctor-fib-sweep-incomplete` with the AF_MPLS fix (D-4) |
| AC-9 | `fib-kernel` opens on a kernel holding a table-0 push a previous run left; a source applies a push for that FEC | The leftover is replaced in its slot and acknowledged; the listing leaves the leftover record, and the prefix leaves `retained` when no listing of it remains, so a later system-RIB install of the same slot is an exclusive add (D-5) |
| AC-10 | A previous run left an IGP route at its metric and a push at priority 0 for one prefix; the system RIB's replay and the label replay arrive in either order | In both orders the kernel ends with exactly the two routes the owners applied, each in its own slot; neither route is deleted in between, so the push forwards throughout; and nothing is swept that an owner took back |
| AC-11 | A previous run left one protocol-250 route at the push's slot; the system RIB and a label source both apply a route for that slot, in either order | The first to arrive takes the leftover back by replace and the second is refused with `EEXIST`, the outcome of the same order on a box with no leftover (R-1) |
| AC-12 | A previous run left an IGP route at its metric that no protocol announces again, beside a push a source took back | The sweep removes the IGP route by its own priority and the push stays (D-3 in the sweep) |
| AC-13 | `fib-kernel` stops with `flush-on-stop` true while label sources' pushes, swaps and pops are installed and stale in-labels remain | Every one of them is removed from the kernel; a refused delete is counted in the `doctor-fib-flush-incomplete` report (D-6) |
| AC-14 | `fib-kernel` stops with `flush-on-stop` false | No route and no label entry is removed, as today |
| AC-15 | `fib-kernel` opens | `startupSweep` and the label listing write nothing to the kernel; a listing the kernel refuses is raised under `doctor-fib-sweep-incomplete` and nothing of that kind is reclaimed or swept |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPushOverLabeledSystemRIBRouteIsExclusiveAdd` | `internal/plugins/fib/kernel/mplsowner_test.go` | AC-1: after a labeled system-RIB install the push reaches the backend as an add, never a replace; red today (replace) | |
| `TestPushSurvivesSystemRIBWithdraw` | `internal/plugins/fib/kernel/mplsowner_test.go` | AC-2: a system-RIB withdraw and an unlabeled replace leave the label-push record, and the relabel is a replace | |
| `TestRefusedPushRemovalWritesNothing` | `internal/plugins/fib/kernel/mplsowner_test.go` | AC-3: a removal with no recorded push calls no backend delete; red today | |
| `TestPushRelabelCleansNoOtherPriority` | `internal/plugins/fib/kernel/mplsowner_test.go` | AC-4: the replace of a push hands the backend an empty cleanup set | |
| `TestSystemRIBDeleteNamesItsPriority` | `internal/plugins/fib/kernel/mplsowner_test.go` | AC-5: the withdraw and the metric change name the recorded metric; a withdraw of an unrecorded prefix calls no delete | |
| `TestIdenticalPushIsAcknowledgedWithoutWrite` | `internal/plugins/fib/kernel/mplsowner_test.go` | AC-6 | |
| `TestFIBKernelReclaimsLeftoverLabel` | `internal/plugins/fib/kernel/mplsstale_test.go` | AC-7: a recorded in-label is added with the replace flag and leaves the stale record; an unrecorded one is an exclusive add | |
| `TestFIBKernelSweepsUnreclaimedLabels` | `internal/plugins/fib/kernel/mplsstale_test.go` | AC-8: of two recorded in-labels the reclaimed one stays and the other is deleted; a refused delete raises the code with the AF_MPLS fix | |
| `TestFIBKernelReclaimsLeftoverPush` | `internal/plugins/fib/kernel/mplsstale_test.go` | AC-9: replace of the listed slot, listing and retention dropped, later system-RIB install is an add | |
| `TestFIBKernelLabelReplayBeforeSystemRIBReplay` | `internal/plugins/fib/kernel/mplsstale_test.go` | AC-10 and AC-11, label replay first: the plain variant (IGP route at its metric), the labeled-install variant (a labeled system-RIB leftover at the push's slot) and the unlabeled same-slot variant | |
| `TestFIBKernelSystemRIBReplayBeforeLabelReplay` | `internal/plugins/fib/kernel/mplsstale_test.go` | AC-10 and AC-11, system-RIB replay first, the same three variants; the labeled-install variant is the seventh judge's case and asserts an exclusive add for the push | |
| `TestSweepDeletesListingByItsPriority` | `internal/plugins/fib/kernel/mplsstale_test.go` | AC-12 | |
| `TestSweepEmitsNoEvent` | `internal/plugins/fib/kernel/mplsstale_test.go` | Data Flow step 7: with a bus whose subscriber calls back into `handleMPLSEntry`, `sweepStale` returns | |
| `TestFlushRemovesLabelEntries` | `internal/plugins/fib/kernel/fibkernel_flush_test.go` | AC-13, AC-14 | |
| `TestStartupListingsWriteNothing` | `internal/plugins/fib/kernel/mplsstale_test.go` | AC-15, and the claim of RFC4724-4.1-3 with the label listing added | |

### Functional Tests (when operator-facing)
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestOwnershipIntegration_LabeledInstallThenPush` | `internal/plugins/fib/kernel/mplsowner_integration_linux_test.go` (QEMU) | AC-1 against the real kernel, fresh box: labeled system-RIB install, then LDP's push for the FEC | |
| `TestOwnershipIntegration_RefusedPushRemovalKeepsSystemRIBRoute` | same file (QEMU) | AC-3 against the real kernel, fresh box: the LDP-after-labeled-install case, then `Remove` | |
| `TestOwnershipIntegration_PushRemovalKeepsRouteAtAnotherMetric` | same file (QEMU) | AC-4 and A-2: a route at another metric survives the push's relabel and removal | |
| `TestOwnershipIntegration_WithdrawAtAnotherMetricKeepsPush` | same file (QEMU) | AC-5 and A-1: the push survives the system RIB's metric change and withdraw | |
| `TestMPLSIntegration_LeftoverLabelReclaimedByNextWriter` | `internal/plugins/fib/kernel/mplsentry_integration_linux_test.go` (QEMU) | AC-7, AC-8, AC-9 with a second `fibKernel` over the kernel the first one left | |
| `push-keeps-labeled-system-rib-route` | `test/ldp/push-keeps-labeled-system-rib-route.ci` | `option=needs-linux`: a BGP labeled-unicast route and an LDP binding for one prefix; `ip route` shows the BGP route, `ze show doctor` reports the LDP push | |
| `restart-reclaims-leftover-labels` | `test/ldp/restart-reclaims-leftover-labels.ci` | `option=needs-linux`: `fib kernel { flush-on-stop false; }`, LDP up, stop and start `ze`; `ip -f mpls route` shows the same in-labels, `ze show doctor` reports no refused label entry, and a label LDP no longer binds is gone after `sweep-delay` | |

## Files to Modify
- `internal/plugins/fib/kernel/fibkernel.go` - the label-push record and the stale in-label record on `fibKernel`; priorities in the leftover record; `ownsSlotLocked` by slot; `recordInstalledLocked`, `processEvent`, `delChange`, `sweepPrefixLocked`, `sweepStale`, `flushRoutes`, `startupSweep`; `sweepFix` and `fibSweepDiagnosticCode` gain the AF_MPLS form
- `internal/plugins/fib/kernel/mplscontext.go` - `addMPLSPushLocked` by the rules of Data Flow step 2
- `internal/plugins/fib/kernel/mplsentry.go` - `mplsBackend` listing; reclaim in `addMPLSEntryLocked`; record-gated push removal in `delMPLSEntryLocked`; `mplsCountLocked`; the package comment
- `internal/plugins/fib/kernel/mplsentry_linux.go` - the protocol-250 in-label listing
- `internal/plugins/fib/kernel/backend.go`, `backend_other.go`, `richroute.go` - `installedRoute` priority; the delete and replace signatures; the stub listing
- `internal/plugins/fib/kernel/backend_linux.go` - `listZeRoutes` fills the priority; `ownedRouteKey` takes it; `replaceOwnedRoute` deletes only the priorities handed to it
- `internal/plugins/fib/kernel/nexthop_linux.go` - `delRichRoute`, `replaceRichRoute`
- `internal/plugins/fib/kernel/register.go` - the sweep worker starts when either leftover record holds an entry
- `internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go`, `rfc3032_mplsentry_test.go` and the package's backend doubles - follow the signatures; an RFC-tagged test whose body changes owes its discrimination record (`./le rfc record`)
- `docs/architecture/mpls/mpls-kernel.md` - "the push shares the FIB with other writers" states the two records and the slot rule; a new subsection "Label routes a previous run left in the kernel"; "Doctor checks" gains the AF_MPLS form of the sweep
- `docs/architecture/core-design.md` - the FIB pipeline paragraph, if it states ownership by prefix
- `ai/digests/fib-programming.md`, `ai/digests/mpls-signaling.md` - the ownership paragraph

## Files to Create
- `internal/plugins/fib/kernel/mplsowner_test.go`
- `internal/plugins/fib/kernel/mplsstale.go`, `internal/plugins/fib/kernel/mplsstale_test.go` - the stale in-label record, its reclaim and its sweep
- `test/ldp/push-keeps-labeled-system-rib-route.ci`, `test/ldp/restart-reclaims-leftover-labels.ci`

### Integration Checklist
- [ ] Applicable surfaces checked: no YANG, CLI grammar or command changes; the two doctor codes keep their names and gain text; `docs/architecture/mpls/mpls-kernel.md` and the two digests are edited in the same work

### Documentation Update Checklist
- [ ] Updated any doc whose `<!-- source: -->` or `// Design:` claim changed (or verified unaffected via `./le spec citation anchors spec plan/release/fixes/spec-fib-kernel-label-ownership.md`)

## Design presentation

### What changes
- `fib-kernel` keeps one record per owner (the system RIB's, and a new one for label-source pushes beside the existing one for swaps and pops), names table and priority on every kernel write, and takes back or sweeps what its previous run left, label entries included.

### Before and after
- **Before, D-1.** BGP labeled-unicast installs `10.0.0.9/32` with label 300. LDP then binds the same FEC with label 16. `ip route show 10.0.0.9` shows `encap mpls 16 via <LDP peer> proto 250`; `ze show fib kernel` still lists the BGP route.
- **After.** `ip route show 10.0.0.9` still shows the BGP route with label 300, and `ze show doctor` reports LDP's push for `10.0.0.9/32` as refused under `doctor-fib-mpls-no-writer` with the kernel's `file exists`.
- **Before, D-2.** A static route holds `10.0.0.9/32`. LDP's push for it is refused, and LDP later withdraws the binding. `ip route show 10.0.0.9` is empty.
- **After.** The static route is still there.
- **Before, D-3.** OSPF holds `10.0.0.9/32` at metric 20 and LDP's push holds it at priority 0. LDP's peer sends a new label. `ip route show 10.0.0.9` shows the push alone; when LDP later goes down the prefix has no route.
- **After.** Both routes are there after the relabel, and the OSPF route forwards when the push goes.
- **Before, D-4 and D-5.** `ze` restarts with `flush-on-stop false`. `ze show doctor` reports every LDP label as refused with `file exists` while `ip -f mpls route` shows them forwarding; a label LDP dropped during the restart forwards for ever.
- **After.** No refused label entry; the dropped label is gone after `sweep-delay`.

### Alternatives set aside
- **Decide leftover ownership with the push recogniser** (move `pushEntryFromRoute` to `internal/core/mplsfib`, mark each listed route push or IP, reclaim a push only over a push). Set aside: a labeled system-RIB route is also a protocol-250 route with an MPLS encap, so the recogniser cannot tell the owners apart, which is the seventh judge's finding. It also leaves an unlabeled leftover refusing the push until the sweep, which then needs a second label replay.
- **A second label replay after the sweep** (R-16 of `spec-fib-dummy.md`). Set aside: `sweepStale` holds `f.mu` and a source answers a replay synchronously into `handleMPLSEntry`, so the emit would have to move outside the lock, and the slot rule leaves no push refused over a leftover, so nothing is left for it to repair.
- **A different `rtm_protocol` for label-source pushes.** It would let the kernel tell the owners apart and survive a restart. Set aside: `ai/doors/forwarding.md` and `internal/core/rtproto` give `fib-kernel` one protocol number that every reader of the kernel (the monitor, `show mpls forwarding`, the doctor fixes, operators' scripts) filters on, and a second number is a wider contract change than the defects need.
- **Refuse a push from a second label source for a FEC another source holds**, as swap and pop do. Set aside here: it decides which source wins a FEC, which is a preference between label protocols and not a defect of ownership.
- **Keep the kernel write for an identical push.** Simpler, and harmless once the cleanup is exact. Set aside: the targeted replay `spec-fib-dummy.md` adds re-sends every push a writer already holds from the applies that arrived between its subscription and the replay, and each would be a kernel write and a lookup for nothing.

### Documents departed from
- `docs/architecture/mpls/mpls-kernel.md`, "Constraint: the push shares the FIB with other writers": "A first install therefore uses `RouteAdd` and fails with `EEXIST` rather than clobbering that route. Only a genuine relabel of Ze's own push uses `RouteReplace`."
  - **Where the design departs.** A new `fib-kernel` holds no push record, so by the letter every push it applies is a first install. The design lets a push replace a protocol-250 route its previous run left in the push's own slot (main table, the priority of a route with no metric). The new process cannot tell whether that leftover was a push or the previous run's system-RIB route at that priority: both carry Ze's protocol number, and a labeled system-RIB route carries an MPLS encap too.
  - **Before.** `ze` restarts with `flush-on-stop false` and LDP. `ip route show 10.0.0.9` still shows the previous run's `encap mpls 16 ... proto 250`; LDP applies the same push; `ze show doctor` reports it refused with `file exists` until `sweep-delay` removes the leftover, and nothing applies it again afterwards.
  - **After.** The leftover is replaced by the push LDP applied, and `ze show doctor` reports nothing. In the contested case (the previous run's route in that slot was the system RIB's, for example a static route with no metric for a FEC LDP also binds) the first owner to apply after the restart holds the slot and the other is refused, as on a box with no leftover.
  - **Not departed from.** A live route is never replaced by the other owner, and a foreign route (another protocol number) still refuses the push.
  - Once Thomas agrees, the page's section is rewritten in the same work to state the two records and the leftover rule.

### Choices for Thomas

| ID | Choice | Evidence | What each answer changes |
|----|--------|----------|--------------------------|
| C-1 | What a push applied after a restart does over a route the previous run left in the push's own slot, which `fib-kernel` cannot attribute to either owner | `mpls-kernel.md` allows a replace only for "a genuine relabel of Ze's own push"; the kernel keeps one protocol number for both owners (`internal/core/rtproto`, `FIBKernel` 250); a contested slot needs a system-RIB route with no metric for a FEC a label source binds | **(a), recommended: take it back by replace.** The restart is hitless for every push, the result equals a box with no leftover for the same arrival order, and nothing new is persisted. Cost: in the contested case the slot can change owner across a restart. **(b): never replace, exclusive add.** The page stays as written. Every push of a restarted box is refused until `sweep-delay` removes the leftovers, labeled traffic falls back to the IP route for that time, and the pushes then need a second label replay sent after the sweep. **(c): persist the label-push record** through the plugin state RPCs, so the new process knows which leftovers were pushes. Exact attribution; it adds a persisted record that a crash can leave behind the kernel, and a state read before the first apply |

## Implementation Steps

1. **Phase: slot-exact writes.** Red tests for AC-4, AC-5, AC-12; the priority in `installedRoute`, `ownedRouteKey`, `delRichRoute`, `replaceOwnedRoute` and the leftover record; `ownsSlotLocked` by slot.
2. **Phase: the label-push record.** Red tests for AC-1, AC-2, AC-3, AC-6; `addMPLSPushLocked`, `delMPLSEntryLocked`, `mplsCountLocked`; `mplsInstalled` keeps the gauge role only.
3. **Phase: leftovers.** Red tests for AC-7 to AC-11 and AC-15; the in-label listing, the stale in-label record, both reclaims, the sweep and its report.
4. **Phase: the stop flush.** Red tests for AC-13, AC-14; `flushRoutes`.
5. **Phase: kernel proof and pages.** The QEMU tests and the two `.ci` files; `mpls-kernel.md`, the digests; the discrimination records of the RFC-tagged tests whose bodies changed.

## Known limits

- Which label source's push holds a FEC two sources bind stays "last applied", as today.
- A push something else removed from the kernel is not put back by an identical re-apply (R-3).
- The targeted label replay that makes a reload-started `fib-kernel` receive the sources' entries is `plan/release/features/spec-fib-dummy.md` AC-21, not this spec.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `reconciler-claims-state-another-owner-installed.md` | 2026-11-24 | `addMPLSPushLocked` replaces a live labeled system-RIB route (D-1) |
| `reconciler-claims-state-another-owner-installed.md` | 2026-11-24 | `delMPLSEntryLocked` deletes a route no push installed (D-2) |
| `reconciler-claims-state-another-owner-installed.md` | 2026-11-24 | `replaceOwnedRoute` and the priority-less delete remove the other owner's route (D-3) |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-24 | stale-mark-then-sweep skips label entries a previous run left (D-4, D-5) |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-24 | `flushRoutes` leaves label-source pushes and AF_MPLS entries (D-6) |

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Scoped verification run by main thread: `./le test unit pkg path internal/plugins/fib/kernel/` and `./le go lint run scope "internal/plugins/fib/kernel"`
- [ ] Closure: append `plan/templates/TEMPLATE-CLOSURE.md`, complete `/ze-review` gate, land Commit A + Commit B (`remove plan/release/fixes/spec-fib-kernel-label-ownership.md`)
