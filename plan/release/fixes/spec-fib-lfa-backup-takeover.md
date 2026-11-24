# Spec: fib-lfa-backup-takeover -- Fast-reroute LFA / TI-LFA backup takeover and FIB reporting

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P2 |
| Release reason | OSPF accepts `fast-reroute` configuration and computes RFC 5286 LFA and TI-LFA backups that neither FIB writer activates on primary failure; `fib-vpp` silently ignores `Backup`, and `ze_ospf_fast_reroute_backups_installed` counts SPF computations rather than FIB installs. |
| RFC gaps | RFC5286-x-6, RFC5286-x-18, RFC5286-x-21, RFC5286-x-22 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Spun out of `spec-fib-depth` on 2026-11-24 by owner decision so the last open owned journal row (`plan/journal/kernel-refuses-what-the-installer-sends.md`, 2026-11-13) has a dedicated design and implementation home while `spec-fib-depth` closes once its remaining proofs finish.

## Task

OSPF fast-reroute (`fast-reroute` config) computes per-primary RFC 5286 LFA and TI-LFA backup next hops (with optional MPLS repair label stacks) in `internal/plugins/ospf/spf/lfa.go`, attaches them to `locrib.Path`, and `sysrib` forwards them on `BestChangeEntry.Backup`. Today, however, no backup path ever forwards traffic when a primary link or neighbor fails, and two observability surfaces disagree with reality:

1. **Linux kernel (`fib-kernel`) has no backup takeover.** Previously `buildBackupNexthops` sent LFA backups as `RTA_MULTIPATH` members flagged `RTNH_F_LINKDOWN`, which Linux IPv4 (`fib_get_nhs`) refused with `EINVAL` for the entire route (primary included) and IPv6 silently dropped the flag on (turning the backup into a live ECMP member). `buildRichRoute` (`internal/plugins/fib/kernel/nexthop_linux.go`) now installs only the primary path(s) and `backup_report.go` raises one `doctor-fib-backup-not-programmed` warning on the report bus. Traffic recovers only when OSPF reconverges, not via precomputed local repair. Once a backup is activated on primary failure, RFC 5286 Section 4.1 (`RFC5286-x-6`, MUST) also requires bounding how long an alternate remains active.
2. **VPP (`fib-vpp`) silently ignores `BestChangeEntry.Backup`.** `hasRichFields`, `changeToRichRoute` (`internal/plugins/fib/vpp/fibvpp.go`), `vppRichRoute`, and `addRichRouteInTable` (`internal/plugins/fib/vpp/backend.go`) neither inspect nor program `c.Backup` and raise no warning when a route carries an unprogrammed backup. Note from source: vendored GoVPP `fib_types.FibPathFlags` (`vendor/go.fd.io/govpp/binapi/fib_types/fib_types.ba.go`, VPP 25.10 bindings) exposes no `FIB_API_PATH_FLAG_BACKUP` constant; `fib_types.FibPath` carries `Preference uint8` (lower preference wins; higher preference acts as backup/fallback in VPP FIB path lists, to be verified in VPP v26.06 source at design time).
3. **OSPF FRR metric overstates FIB installation.** `ze_ospf_fast_reroute_backups_installed{kind}` (`internal/plugins/ospf/spf/computer.go:276-280`, updated in `publishFRRMetrics` / `attachAreaBackups` in `internal/plugins/ospf/spf/lfa.go:131-154,225,233`) increments during SPF computation before `Installer.Apply`, so it counts SPF-computed backups even when no FIB writer programmed or activated them.

The goal of this spec is to settle the design choices with Thomas and deliver real fast-reroute backup takeover on Linux, honest native programming or reporting on VPP, and an accurate `ze_ospf_fast_reroute_backups_installed` metric (or FIB-level install metric alongside an accurately named SPF-computed metric) backed by real kernel and VPP proof.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform boundaries, process boundary, registration over enumeration
  → Constraint: FIB writers and routing protocols communicate only across boundaries that survive a process split (EventBus / RPC), never by plain in-process package calls into another plugin's state.
- [ ] `ai/doors/forwarding.md` - FIB ownership, exact-or-reject, report bus, Linux vs VPP semantics
  → Decision: one FIB writer per data plane; protocols never write the kernel; report-bus warnings from a plugin go through the process-safe path.
- [ ] `ai/doors/routing-protocols.md` - OSPF SPF/Loc-RIB/FIB split and BFD client boundary
  → Constraint: OSPF inserts `locrib.Path` values and never writes the kernel; BFD client lookup (`internal/component/bfd/api/registry.go`) is in-process only and returns nil in a forked plugin.
- [ ] `docs/architecture/ospf/ospf-ext-6-ti-lfa.md` - OSPF LFA / TI-LFA design and current kernel gap
  → Decision: backup is per-primary, carried parallel to primary next hops through Loc-RIB and `sysrib` on `BestChangeEntry.Backup`, never merged into `ECMPPaths`.
- [ ] `docs/guide/ospf.md` - operator documentation of `fast-reroute`, metrics, and `doctor-fib-backup-not-programmed`
  → Constraint: public claims must match proven data-plane behavior immediately (`ai/rules/rfc-compliance.md`).
- [ ] `ai/digests/fib-programming.md` - sysrib -> FIB writer flow for kernel and VPP
  → Constraint: preserve `BestChangeEntry` JSON compatibility (`omitempty`) and `rtprotZE` route ownership.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5286.md` - Basic Specification for IP Fast Reroute: Loop-Free Alternates
  → Constraint: `RFC5286-x-6` (§4.1, MUST) requires bounding how long an alternate next hop is used after primary failure; `RFC5286-x-18` (§4, SHOULD) removes the failed primary and installs the precomputed LFA; `RFC5286-x-21` and `RFC5286-x-22` (§4.1, SHOULD) specify hold-down / termination criteria.
- [ ] `rfc/short/rfc8665.md` - OSPF Extensions for Segment Routing (TI-LFA repair label stacks)
  → Constraint: TI-LFA repair labels carried on `Backup[i].Labels` must be pushed in order when the backup activates.

**Key insights:**
- Linux `RTA_MULTIPATH` has no standby member; `fib_get_nhs` rejects `RTNH_F_LINKDOWN` from userspace with `EINVAL` on IPv4 and strips it on IPv6.
- GoVPP `fib_types` (`vendor/go.fd.io/govpp/binapi/fib_types/fib_types.ba.go`) has no `FIB_API_PATH_FLAG_BACKUP` flag; VPP `FibPath` has `Preference uint8`, whose exact VPP v26.06 failover semantics must be verified against VPP source during design.
- `fib-kernel` already retains the full last-installed `BestChangeEntry` (including `Backup`) in `f.forwarding[pfx]` (`internal/plugins/fib/kernel/fibkernel.go:202,300-306,536`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/fib/kernel/nexthop_linux.go` - `buildRichRoute` (lines 139-220) explicitly omits `r.Backup` (comment lines 211-216) because Linux refuses `RTNH_F_LINKDOWN` in `RTA_MULTIPATH`; sends primary path(s) alone. `buildBackupNexthops` was already removed when fixing the route refusal.
- [ ] `internal/plugins/fib/kernel/backup_report.go` - `trackBackupLocked` and `reportBackupsLocked` track installed prefixes with non-empty `c.Backup` and raise/clear one report-bus warning `doctor-fib-backup-not-programmed` under source `fib`, subject `fast-reroute`.
- [ ] `internal/plugins/fib/kernel/fibkernel.go` - `hasRichFields` (line 277) checks `len(c.Backup) > 0`; `changeToRichRoute` (line 283) copies `Backup`; `recordInstalledLocked` (line 536-539) clones the full `incomingChange` (including `Backup`) into `f.forwarding[pfx]` and calls `f.trackBackupLocked(pfx, c)`.
- [ ] `internal/plugins/fib/kernel/richroute.go` - `RichRoute.Backup` carries `[]sysribevents.ECMPPath` for replay identity only.
- [ ] `internal/plugins/fib/kernel/backup_install_integration_linux_test.go` - `TestFIBBackupRouteInstallsItsPrimary` proves on a real Linux kernel that a route carrying `Backup` (with MPLS repair labels) installs its primary path alone without `EINVAL`.
- [ ] `internal/plugins/fib/vpp/fibvpp.go` - `hasRichFields` (line 274) and `changeToRichRoute` (line 282) do not reference `c.Backup`; `fibVPP` holds no backup state and raises no warning when `Backup` is present.
- [ ] `internal/plugins/fib/vpp/backend.go` - `vppRichRoute` (line 24) has no `Backup` field; `addRichRouteInTable` (line 106) builds `FibPath` slices from primary and `ECMPPaths` only, leaving `FibPath.Preference` zero.
- [ ] `vendor/go.fd.io/govpp/binapi/fib_types/fib_types.ba.go` - `FibPathFlags` defines only `NONE`, `RESOLVE_VIA_ATTACHED`, `RESOLVE_VIA_HOST`, and `POP_PW_CW` (no `FIB_API_PATH_FLAG_BACKUP`); `FibPath` carries `Preference uint8` (line 189).
- [ ] `internal/plugins/ospf/spf/computer.go` - `SetMetrics` (lines 276-280) registers `ze_ospf_fast_reroute_backups_installed` (`kind` label: `lfa` / `ti-lfa`); `Run` (lines 525-540) runs `attachAllBackups` and calls `publishFRRMetrics` before `c.installer.Apply(selected)`.
- [ ] `internal/plugins/ospf/spf/lfa.go` - `attachAreaBackups` (lines 225, 233) increments `st.installed["lfa"]` and `st.installed["ti-lfa"]` whenever SPF selects an alternate; `publishFRRMetrics` (lines 131-154) publishes those SPF counts to `ze_ospf_fast_reroute_backups_installed`.
- [ ] `internal/component/sysrib/events/events.go` - `BestChangeEntry.Backup` (lines 89-94) carries `[]ECMPPath` (`json:"backup,omitempty"`), populated by `backupPaths` in `internal/component/sysrib/ecmp.go` (lines 254-265).
- [ ] `internal/core/rib/locrib/candidate.go` - `Path` carries `BackupNextHop`, `BackupInterface`, `BackupOnLink`, `BackupRepairLabels` (lines 147-170); included in `Path.Equal` (lines 277-288) and excluded from best-path arbitration.

**Behavior to preserve:**
- Primary route installation (single-path, ECMP, MPLS push, SRv6) continues to succeed on stock Linux and Ze's kernel without `EINVAL`, and backups never load-share with primaries in steady state.
- `BestChangeEntry` JSON wire compatibility (`backup` field stays `omitempty`, `ECMPPaths` remains strictly load-sharing).
- `locrib.Path` arbitration remains distance-then-metric with backup excluded from selection and included in change detection.
- `show ospf route fast-reroute` continues to display computed LFA/TI-LFA alternates and protection classes.

**Behavior to change:**
- Settled during design after Thomas resolves Choices C-1, C-2, and C-3 below:
  - Linux kernel backup takeover on primary failure plus RFC 5286 Section 4.1 hold-down/termination bound, and updated `doctor-fib-backup-not-programmed` behavior.
  - VPP handling of `BestChangeEntry.Backup` (native VPP backup path vs explicit warning/reporting).
  - Accurate `ze_ospf_fast_reroute_backups_installed` accounting (measuring actual FIB programming rather than SPF computation alone).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| System-RIB backup carriage | `internal/component/sysrib/events/events.go` `BestChangeEntry.Backup`; `internal/component/sysrib/ecmp.go` `backupPaths` | Delivers precomputed per-prefix LFA/TI-LFA backup path(s) (with `Interface`, `OnLink`, `Labels`) separately from `ECMPPaths` | extends: FIB writers consume `Backup` from this existing event without adding a second route bus |
| `fib-kernel` forwarding cache & backup tracking | `internal/plugins/fib/kernel/fibkernel.go` `f.forwarding`, `f.backups`; `backup_report.go` `reportBackupsLocked` | Retains full `BestChangeEntry` (including `Backup`) per installed prefix and raises `doctor-fib-backup-not-programmed` | extends: userspace standby takeover (if C-1a) indexes `f.forwarding` by primary interface/next-hop and updates/retires `doctor-fib-backup-not-programmed` accordingly |
| Interface link-state events | `internal/component/iface/register.go` (`ifaceevents.EventDown`, `ifaceevents.EventUp`) | Publishes link carrier down/up transitions on the EventBus for both kernel and VPP interfaces | extends (if C-1a): candidate local link-failure trigger for `fib-kernel` backup takeover without polling |
| BFD session state events | `internal/component/bfd/api/events.go` `StateChange` (`Initial`, `RemoteAdminDown`, `StateDown`) | Notifies in-process BFD clients on path failure (RFC 5882 Section 4.2.1) while ignoring snapshots and remote AdminDown | extends (if C-1a includes BFD): fast next-hop failure signal when carrier stays up; note in-process boundary in `ai/doors/routing-protocols.md` |
| `fib-vpp` rich route programming | `internal/plugins/fib/vpp/fibvpp.go` `changeToRichRoute`; `internal/plugins/fib/vpp/backend.go` `addRichRouteInTable`, `toFibPath` | Programs `FibPath` list under VPP FIB source `ze-fib-vpp` via `ip_route_add_del_v2` | extends: either encodes backup `FibPath` entries (e.g., via `Preference` after VPP v26.06 source verification) or raises a backup-not-programmed warning (per C-2) |
| Report bus & diagnostic codes | `internal/core/report`, `internal/core/diagnostic`, `internal/plugins/fib/kernel/backup_report.go` | Surfaces `doctor-fib-backup-not-programmed` in `show warnings` and `show diagnostic` | extends: reflects real unprogrammed-backup state on whichever data plane cannot program backups |
| OSPF SPF FRR metrics | `internal/plugins/ospf/spf/computer.go` `mFRRInstalled`; `internal/plugins/ospf/spf/lfa.go` `publishFRRMetrics` | Publishes `ze_ospf_fast_reroute_backups_installed{kind}` from SPF counts | replaces/extends (per C-3): separates SPF-computed count from FIB-installed backup count so the metric does not claim an uninstalled backup is installed |

## Design documents

Skeleton: run `./le spec design documents spec plan/release/fixes/spec-fib-lfa-backup-takeover.md` once design choices C-1 through C-3 are resolved and `Files to Modify` is finalized, then populate every derived row and check with `./le spec design check spec plan/release/fixes/spec-fib-lfa-backup-takeover.md`.

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| [derived at design time] | [section heading] | [what derived it] | "[verbatim span]" | [agrees / departs / silent] | [why] |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- OSPF SPF computes per-primary LFA / TI-LFA backups (`internal/plugins/ospf/spf/lfa.go`) and installs `locrib.Path` entries carrying `BackupNextHop`, `BackupInterface`, `BackupOnLink`, and `BackupRepairLabels` (`internal/plugins/ospf/spf/install.go`).
- `sysrib` publishes `(system-rib, best-change)` carrying `BestChangeEntry.Backup`.
- Failure triggers (if userspace takeover is selected under C-1): interface link-down (`ifaceevents.EventDown` or netlink `RTM_DELLINK`/`IFF_RUNNING` drop) and/or BFD `StateDown` transitions.

### Transformation Path
1. OSPF SPF attaches LFA/TI-LFA backups and inserts into Loc-RIB; `sysrib` resolves and emits `BestChangeEntry` with `Backup`.
2. Target FIB writer (`fib-kernel` or `fib-vpp`) receives `BestChangeEntry`:
   - `fib-kernel`: installs primary path(s) and retains standby backup metadata (or programs kernel standby state if C-1 selects a kernel patch); on primary link/BFD failure, swings affected prefixes to their precomputed backup (including MPLS repair labels for TI-LFA) and starts the RFC 5286 Section 4.1 hold-down/termination bound until reconvergence or timer expiry.
   - `fib-vpp`: programs backup paths natively in VPP or records and warns on unprogrammed backups (per C-2).
3. Metrics reflect actual FIB backup installation rather than SPF computation alone (per C-3).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OSPF → Loc-RIB → sysrib | `locrib.Path` / `route-install` RPC, `(system-rib, best-change)` EventBus typed handle | No |
| sysrib → `fib-kernel` / `fib-vpp` | `(system-rib, best-change)` carrying `BestChangeEntry.Backup` | No |
| `iface` / `bfd` → `fib-kernel` (if C-1a) | EventBus `iface` link-down events / BFD state notification (process boundary checked at design time) | No |
| `fib-kernel` → Linux kernel | netlink `RTM_NEWROUTE` / `RTM_DELROUTE` (or patched kernel primitive if C-1b) | No |
| `fib-vpp` → VPP | GoVPP `ip_route_add_del_v2` (if C-2a) | No |

### Changed-Wire Census

Skeleton: complete full search evidence at design time. Preliminary read shows `BestChangeEntry.Backup` (`internal/component/sysrib/events/events.go`) already carries `NextHop`, `Interface`, `OnLink`, `Weight`, and `Labels` (`json:"backup,omitempty"`). Whether a per-primary-to-backup mapping is needed for ECMP primaries (today `backupPaths` in `internal/component/sysrib/ecmp.go` emits a single backup from the winning `protocolRoute`) or whether a FIB install-feedback event is needed for C-3 will be settled at design time.

**Search evidence:** [completed at design time]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [completed at design time] | [role] | [source site] | [search finding] | [disposition] |

### Integration Points
- `internal/plugins/fib/kernel/nexthop_linux.go` (`buildRichRoute`) and `internal/plugins/fib/kernel/fibkernel.go` (`f.forwarding`, `processEvent`) - primary/backup state and failover swap
- `internal/plugins/fib/kernel/backup_report.go` (`reportBackupsLocked`, `doctor-fib-backup-not-programmed`) - warning lifecycle
- `internal/plugins/fib/vpp/fibvpp.go` (`hasRichFields`, `changeToRichRoute`, `processEvent`) and `internal/plugins/fib/vpp/backend.go` (`vppRichRoute`, `addRichRouteInTable`) - VPP backup programming or warning
- `internal/plugins/ospf/spf/computer.go` (`mFRRInstalled`) and `internal/plugins/ospf/spf/lfa.go` (`publishFRRMetrics`, `attachAreaBackups`) - accurate FRR backup installation metric

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | [verified at design/implementation] |
| No unintended coupling (components stay isolated) | No | [verified at design/implementation] |
| No duplicated functionality (extends existing, does not recreate) | No | [verified at design/implementation] |
| Zero-copy preserved where applicable (refs, not copies) | No | [verified at design/implementation] |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | [verified at design/implementation] |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name (`ai/rules/principles.md`) | No | [verified at design/implementation] |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Linux IPv4 `fib_get_nhs` still refuses `RTNH_F_LINKDOWN` from userspace and IPv6 strips it | `plan/journal/kernel-refuses-what-the-installer-sends.md` (2026-11-13), `internal/plugins/fib/kernel/nexthop_linux.go:211-216` | Kernel-native standby multipath would exist without userspace takeover or a kernel patch | Real Linux kernel integration test (`TestFIBBackupRouteInstallsItsPrimary`) | unvalidated |
| A-2 | VPP v26.06 exposes no `FIB_API_PATH_FLAG_BACKUP` in GoVPP `fib_types`, and uses `FibPath.Preference` (or another FIB path mechanism) for backup paths | `vendor/go.fd.io/govpp/binapi/fib_types/fib_types.ba.go:33-56,189` (VPP 25.10 binapi) | VPP v26.06 may require regenerated binapi or a different FIB API for FRR/PIC backups | Read VPP v26.06 `src/vnet/fib/` source at design time | unvalidated |
| A-3 | `sysrib` currently forwards at most one backup per prefix (`backupPaths`, `internal/component/sysrib/ecmp.go:254-265`), not a per-ECMP-primary backup map | `internal/component/sysrib/ecmp.go:254-265`, `internal/core/rib/locrib/candidate.go:147-170` | ECMP primaries with distinct per-primary LFAs would need a richer mapping on `BestChangeEntry.Backup` | Inspect `internal/plugins/ospf/spf/install.go` + `internal/component/sysrib/ecmp.go` at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Userspace takeover races with OSPF SPF reconvergence, overwriting the newly converged route with a stale backup or leaving a backup active indefinitely (violating RFC 5286 Section 4.1) | Route stays on backup after primary link recovers or after SPF installs a new primary | Enforce generation/sequence ordering with `BestChangeEntry` updates and a bounded RFC 5286 Section 4.1 hold-down/termination timer |
| R-2 | Backup path becomes a live ECMP load-sharing member in steady state on Linux or VPP | Packets traverse backup link while primary is up | Assert steady-state single-primary forwarding on real kernel and real VPP before link down |
| R-3 | External (forked) `fib-kernel` cannot reach in-process BFD registry (`internal/component/bfd/api/registry.go`) | BFD takeover works in-process but never fires when `fib-kernel` runs forked | Route failure signals over process-safe EventBus events or explicitly scope BFD integration across the plugin boundary |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Protected prefixes could fail to install (`EINVAL`), load-share across backup links in steady state, fail to swing on link down, or stay pinned on a stale backup after convergence (micro-loop). |
| How is it reverted? | Single commit revert restores primary-only programming plus `doctor-fib-backup-not-programmed` warning (unless a kernel patch is chosen under C-1b, which also requires reverting the kernel patch). |
| Who else touches this path? | `fib-kernel`, `fib-vpp`, `sysrib`, `ospf/spf`, `iface` link monitor, and `bfd` (if BFD trigger is chosen). |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `(system-rib, best-change)` with `Backup` + primary link failure | → | `fib-kernel` backup takeover + hold-down termination | `TestFIBKernelBackupTakeoverOnLinkDown` (tentative; confirmed at design time) |
| `(system-rib, best-change)` with `Backup` under `fib-vpp` | → | `fib-vpp` native backup programming or warning (per C-2) | `TestFibVPPBackupHandling` (tentative; confirmed at design time) |
| OSPF `fast-reroute` SPF + FIB install | → | `ze_ospf_fast_reroute_backups_installed` / FIB backup metric (per C-3) | `TestOSPFBackupsInstalledReflectsFIBState` (tentative; confirmed at design time) |

## Acceptance Criteria

Tentative ACs pending Thomas's design choices (C-1, C-2, C-3); finalized before moving from `skeleton` to `design`:

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Steady state: route carrying `Backup` (plain LFA and TI-LFA with MPLS repair labels) is delivered to `fib-kernel` on a real Linux kernel | Primary path(s) are programmed and forward traffic; backup path does not receive steady-state traffic; no `EINVAL` error occurs | None: steady-state traffic must follow primary path(s) only |
| AC-2 | Primary link or neighbor failure occurs for a protected prefix installed in `fib-kernel` | `fib-kernel` activates the precomputed backup next hop (including MPLS repair label stack for TI-LFA) in the kernel FIB without waiting for IGP SPF reconvergence, and forwards packets over the backup path | Prefixes with no valid backup remain unprotected until SPF reconvergence, because no loop-free alternate exists |
| AC-3 | Activated backup stays in place after primary failure (`RFC5286-x-6`, §4.1) | Use of the alternate is strictly time-bounded by a hold-down / termination rule (new loop-free primary, hold-down expiry, or unrelated topology change), returning the prefix to the converged primary (or withdrawing it if unreachable) | None: an unbounded alternate violates RFC 5286 Section 4.1 MUST |
| AC-4 | Route carrying `Backup` is delivered to `fib-vpp` | Per C-2: either VPP programs the backup path using VPP's native backup/preference mechanism so primary-down swings in VPP without steady-state load-sharing, OR `fib-vpp` installs primary only and raises an operator warning matching `doctor-fib-backup-not-programmed` | Per C-2 decision once settled |
| AC-5 | OSPF `fast-reroute` is enabled and computes backups | `ze_ospf_fast_reroute_backups_installed` (or the metric pair settled in C-3) reports zero when no FIB writer has programmed/armed the backups and equals the actually programmed/armed backup count when a FIB writer has | SPF-computed backup count remains observable via `show ospf route fast-reroute` and/or a dedicated computed metric so operators can distinguish computed from FIB-programmed |
| AC-6 | Operator visibility (`show warnings`, `docs/guide/ospf.md`, `rfc/short/rfc5286.md`) | `doctor-fib-backup-not-programmed` is raised only when installed routes hold backups the active FIB writer cannot arm/program, and cleared when backups are armed/programmed; docs and `rfc/short/rfc5286.md` match actual behavior | None: public claims and warnings must match live forwarding behavior |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables OSPF `fast-reroute` on Linux and brings down the primary link | OSPF SPF -> Loc-RIB -> sysrib `Backup` -> `fib-kernel` standby -> link-down takeover -> backup forwarding -> hold-down termination on convergence | `test/plugin/fib-lfa-takeover.ci` (tentative, `needs-linux`) |
| 2 | Enables OSPF `fast-reroute` on VPP | OSPF SPF -> Loc-RIB -> sysrib `Backup` -> `fib-vpp` (native backup or explicit warning per C-2) | `test/vpp/vpp-fib-lfa-backup.ci` (tentative) |
| 3 | Scrapes Prometheus metrics for OSPF fast-reroute | `ze_ospf_fast_reroute_*` accurately distinguishes SPF-computed vs FIB-installed backups | Unit + functional test (confirmed at design time) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFIBKernelBackupTakeoverOnLinkDown` | `internal/plugins/fib/kernel/backup_takeover_test.go` | Primary link-down swings affected routes to backup (plain + TI-LFA labels) and hold-down terminates backup | |
| `TestFibVPPBackupHandling` | `internal/plugins/fib/vpp/fibvpp_test.go` | VPP backup encoding or warning lifecycle per C-2 | |
| `TestOSPFBackupsInstalledReflectsFIBState` | `internal/plugins/ospf/spf/computer_lfa_test.go` | Metric accuracy per C-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Hold-down timer (if configurable under RFC 5286 §4.1) | [settled at design time] | [tbd] | [tbd] | [tbd] |
| VPP `FibPath.Preference` (if C-2a) | 0-255 | 255 | N/A (`uint8`) | N/A (`uint8`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fib-lfa-takeover` | `test/plugin/fib-lfa-takeover.ci` | Real Linux kernel (`needs-linux`): steady-state primary forwarding, link down activates backup, reconvergence/hold-down restores primary | |
| `vpp-fib-lfa-backup` | `test/vpp/vpp-fib-lfa-backup.ci` | VPP FIB writer handling of `BestChangeEntry.Backup` per C-2 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| [confirm at design time against existing `ospf-lfa-frr` / `ospf-ti-lfa-frr` scenarios] | `test/interop/scenarios/` | FRR | Local repair on primary link failure before OSPF reconvergence | |

## Files to Modify
- `internal/plugins/fib/kernel/nexthop_linux.go` - primary/backup route programming or kernel standby support (per C-1)
- `internal/plugins/fib/kernel/fibkernel.go` - standby backup index, failure-event takeover, RFC 5286 §4.1 hold-down/termination
- `internal/plugins/fib/kernel/backup_report.go` - update `doctor-fib-backup-not-programmed` lifecycle to reflect actual armed/unarmed state
- `internal/plugins/fib/vpp/fibvpp.go` - inspect `c.Backup` in `hasRichFields`, `changeToRichRoute`, and event/warning handling (per C-2)
- `internal/plugins/fib/vpp/backend.go` - carry `Backup` on `vppRichRoute` and program VPP backup paths if C-2a
- `internal/plugins/ospf/spf/computer.go` - `ze_ospf_fast_reroute_backups_installed` definition/accounting (per C-3)
- `internal/plugins/ospf/spf/lfa.go` - separate SPF-computed stats from FIB-installed counts (per C-3)
- `docs/guide/ospf.md` - update fast-reroute gap note and metrics description
- `docs/architecture/ospf/ospf-ext-6-ti-lfa.md` - update FIB backup programming decision
- `rfc/short/rfc5286.md` - update `RFC5286-x-6` (and related SHOULD rows) once proven

## Files to Create
- `internal/plugins/fib/kernel/backup_takeover_test.go` - unit tests for takeover, TI-LFA label push on failover, and hold-down termination
- `test/plugin/fib-lfa-takeover.ci` - Linux functional test (`needs-linux`) proving data-plane failover
- `test/vpp/vpp-fib-lfa-backup.ci` - VPP functional test proving C-2 behavior

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | [design time] | Depends on whether RFC 5286 §4.1 hold-down timer is operator-configurable |
| YANG validation constraints | [design time] | Applies if a hold-down leaf is added |
| YANG custom validators | [design time] | Assessed at design time |
| CLI commands/flags | [design time] | Assess whether `show fib kernel` / `show fib vpp` surfaces armed backups |
| CLI grammar (keyword before value) | [design time] | Applies if any show output/flag changes |
| Editor autocomplete | [design time] | Automatic for YANG leaves |
| Functional test for new RPC/API | [design time] | Assessed at design time |
| Pipe completeness | [design time] | Assessed at design time |
| Env var registration | No | No env var planned |
| Doctor check for runtime dependencies | [design time] | Applies if C-1b requires a kernel capability probe |
| Doctor code per problem class | Yes | `doctor-fib-backup-not-programmed` in `internal/plugins/fib/kernel/backup_report.go` (and VPP equivalent if C-2b) |
| Prometheus counters/metrics | Yes | `ze_ospf_fast_reroute_backups_installed` and any FIB backup takeover/installed metrics (per C-3) |
| BGP family surface (new SAFI / capability / attribute) | No | No BGP family changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | Check `features/` and `docs/features/rfc-status.md` (RFC 5286 row) |
| 2 | Config syntax changed? | [design time] | `docs/guide/configuration.md` if hold-down leaf added |
| 3 | CLI command added/changed? | [design time] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | None planned |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if `fib-kernel`/`fib-vpp` behavior changes |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md` (fast-reroute section) |
| 7 | Wire format changed? | No | RFC 5286 defines no wire format |
| 8 | Plugin SDK/protocol changed? | No | Uses existing EventBus handles |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5286.md` and `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | Uses existing QEMU and VPP harnesses |
| 11 | Affects daemon comparison? | [design time] | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ospf/ospf-ext-6-ti-lfa.md`, `ai/digests/fib-programming.md` |
| 13 | Route metadata keys added/changed? | No | Uses existing `BestChangeEntry.Backup` |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/ospf.md` and metrics docs (per C-3) |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | [design time] | Checked at design time |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/release/fixes/spec-fib-lfa-backup-takeover.md` at design time |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify `docs/guide/ospf.md` fast-reroute section |

## Design presentation

### What changes

[Written once Thomas settles C-1, C-2, and C-3 below.]

### Before and after

[Written once Thomas settles C-1, C-2, and C-3 below.]

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Sending `RTNH_F_LINKDOWN` in Linux `RTA_MULTIPATH` | Already attempted and reverted: Linux IPv4 `fib_get_nhs` rejects the whole route with `EINVAL` and IPv6 strips the flag so the backup load-shares in steady state (`plan/journal/kernel-refuses-what-the-installer-sends.md`, 2026-11-13). |

### Documents departed from

[Evaluated at design time; `none` if all derived rows agree.]

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Linux kernel LFA/TI-LFA backup takeover mechanism | (a) Userspace standby takeover in `fib-kernel` (index `f.forwarding` by primary interface / next hop, react immediately to interface link-down and/or BFD down by replacing affected kernel routes with their precomputed `Backup` + repair labels, plus an RFC 5286 §4.1 hold-down/termination bound; works on both stock Linux and Ze's kernel); (b) Kernel patch on Ze's own kernel adding a standby/backup multipath nexthop flag (with `kernelcap` probe and warning/userspace fallback on stock Linux) | [pending Thomas] |
| C-2 | `fib-vpp` handling of `BestChangeEntry.Backup` | (a) Program VPP native backup paths (read VPP v26.06 source first: vendored `fib_types` has no `FIB_API_PATH_FLAG_BACKUP`, so confirm whether `FibPath.Preference` or a newer VPP 26.06 path flag gives true primary/standby failover without steady-state load-sharing); (b) Install primary only in `fib-vpp` and raise an explicit backup-not-programmed warning matching `fib-kernel`, deferring VPP native backup programming | [pending Thomas] |
| C-3 | `ze_ospf_fast_reroute_backups_installed` metric semantics | (a) Report actual FIB-programmed/armed backups (either by having FIB writers expose a FIB backup-armed metric and renaming/splitting OSPF's SPF gauge to `ze_ospf_fast_reroute_backups_computed`, or by feeding FIB install acknowledgements back to OSPF); (b) Keep `ze_ospf_fast_reroute_backups_installed` on OSPF only if backed by real FIB install confirmation | [pending Thomas] |

## Implementation Steps

1. **Phase: Design resolution** -- settle C-1, C-2, and C-3 with Thomas, read VPP v26.06 FIB backup source if C-2a, run `./le spec design documents` and `./le spec design check`, obtain design judge + owner approval, and move spec to `ready`.
2. **Phase: Wiring (MANDATORY FIRST)** -- register entry points and write failing wiring tests for `fib-kernel` takeover, `fib-vpp` backup handling, and FRR metric accounting.
3. **Phase: Linux backup takeover + RFC 5286 §4.1 hold-down** -- implement C-1, update `doctor-fib-backup-not-programmed`, prove in unit tests and real-kernel QEMU `needs-linux` test.
4. **Phase: VPP backup handling** -- implement C-2 and prove in unit + VPP functional / real-VPP tests.
5. **Phase: FRR metrics & docs** -- implement C-3, update `docs/guide/ospf.md`, `docs/architecture/ospf/ospf-ext-6-ti-lfa.md`, and `rfc/short/rfc5286.md` (+ `./le rfc record` for newly proven MUST tags).

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `kernel-refuses-what-the-installer-sends.md` | 2026-11-13 | `buildBackupNexthops` (`internal/plugins/fib/kernel/nexthop_linux.go`): LFA backup takeover on link failure, `ze_ospf_fast_reroute_backups_installed` counts SPF not FIB installs, `fib-vpp` ignores `Backup` |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Spun out of `spec-fib-depth.md` into its own `release/fixes/` spec | Keep inside `spec-fib-depth.md` | `spec-fib-depth.md` is a validation spec awaiting its last parity proofs; LFA backup takeover requires fresh owner design choices across Linux, VPP, and OSPF metrics |

## Known Limitations
- IS-IS LFA (`RFC5286-x-4`) and OSPFv3 multicast-AF backup exclusion (`RFC5286-x-5`) are separate RFC 5286 gaps recorded in `rfc/short/rfc5286.md` and are not silently folded into this FIB takeover spec unless widened at design time.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (`rfc/short/rfc5286.md`, `rfc/short/rfc8665.md`)
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist answered Yes/No/N-A

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass in the main checkout: `./le test unit pkg path ./internal/plugins/fib/kernel/`, `./le test unit pkg path ./internal/plugins/fib/vpp/`, `./le test unit pkg path ./internal/plugins/ospf/spf/`, `./le test vpp`, and real-kernel QEMU proof for `test/plugin/fib-lfa-takeover.ci`
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + journal row marked `fixed:`
- [ ] **Commit B:** `remove plan/release/fixes/spec-fib-lfa-backup-takeover.md` only, in the same `./le commit create` script
