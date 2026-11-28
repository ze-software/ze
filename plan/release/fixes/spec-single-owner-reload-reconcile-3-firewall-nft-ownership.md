# Spec: single-owner-reload-reconcile-3-firewall-nft-ownership

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | After a crash or daemon restart, the nftables backend only deletes a `ze_*` table if its process-local `applied` map holds it or the new config still requests it; any Ze-owned table whose former owner is no longer configured survives in the kernel enforcing stale rules indefinitely. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-single-owner-reload-reconcile.md` (Family S3, part 2 of 3).

## Task

**Root cause.** `(*backend).shouldDeleteTable` (`internal/plugins/firewall/nft/backend_linux.go`) deletes a kernel nftables table only when its name begins with `ze_` AND either the new desired set names it or this backend instance's in-memory `b.applied` map holds it. On daemon restart, `b.applied` starts empty. If a previous process crashed (or exited with `flush-on-shutdown false`) leaving a Ze-owned table (for example a removed operator table, `ze_copp`, `ze_pr`, `ze_ddos-local`, or `ze_anomaly-shape`) and the new config no longer configures that owner, the owner never starts, `desiredNames` does not list the table, and `b.applied` is empty, so `shouldDeleteTable` leaves the table enforcing in the kernel forever.

The backend could not simply delete every `ze_*` table outside `desiredNames`, because `ze_iface_rpf` is written directly over netlink by `internal/plugins/iface/netlink/rpf_linux.go` outside the firewall table registry, and an operator could also have created a foreign table starting with `ze_`.

**Goal.** Stamp every nftables table created by `internal/plugins/firewall/nft` with a kernel-persisted ownership marker in `NFTA_TABLE_USERDATA` (`type = 7`, payload `ze:firewall:v1`) using `github.com/mdlayher/netlink` (already in `go.mod`, zero new third-party dependencies). Read `NFTA_TABLE_USERDATA` on every `Apply`, reclaim any marked table outside `desiredNames` across restarts regardless of `b.applied`, preserve atomic rule replacement for already-marked desired tables, generalize the one-shot startup sweep so an empty desired set still reclaims orphaned marked tables at boot, and leave unmarked foreign tables and `ze_iface_rpf` untouched.

**Why it matters for shipping.** A router that crashes or restarts with a tightened or removed firewall/CoPP/DDoS/FlowSpec config no longer leaves orphaned nftables drop or redirect rules enforcing in the kernel with no owner.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, boundaries
- [ ] `ai/doors/forwarding.md` - firewall backend ownership contract and traps
- [ ] `ai/doors/le-tooling.md` - Linux integration test conventions (`//go:build linux`, `userns.Enter`)
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - Rules 1, 2, 2a, 2b, 3 and 4 (edited in this same work)
- [ ] `docs/architecture/core-design.md` - Section 14b (Firewall and Traffic Control)

## Current Behavior (MANDATORY)

Verified at the producers on 2026-11-26.

**Source files read:**
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - `Apply` (lines 44-108), `shouldDeleteTable` (lines 146-155), `applyTable` (lines 165-231). `shouldDeleteTable` returns true only for `ze_` tables in `desiredNames` or `b.applied`.
- [ ] `internal/plugins/firewall/nft/readback_linux.go` - `readTables` reads `ze_` tables from the kernel.
- [ ] `internal/component/firewall/registry.go` - `RegisterTables` (lines 102-120), `ApplyAll` (lines 131-274, including the `LegacySweepPending()` one-shot autoload at line 197).
- [ ] `internal/component/firewall/legacy_tables.go` - one-shot legacy sweep flag (`LegacySweepPending`, `legacySweepReached`).
- [ ] `internal/component/firewall/reservation.go` - `MustReserveTableNames`, `refuseForeignReservations`, `refuseReservedTableNames` (AC-7 already landed at HEAD and proven by `TestOperatorTableRefusedOnAReservedName` and `test/parse/firewall-reserved-table-name-rejected.ci`).
- [ ] `internal/plugins/flowspec-firewall/state.go` - already uses `const tableName = "ze_flowspec"` (rows 2026-08-18 and 2026-08-22 already landed at HEAD).
- [ ] `github.com/google/nftables v0.3.0` (`table.go`, `conn.go`) - `nftables.Table` exposes `Name`, `Use`, `Flags`, `Family` and does not expose `NFTA_TABLE_USERDATA`; `github.com/mdlayher/netlink v1.11.2` is already a direct dependency in `go.mod`.

**Behavior to preserve:**
- `RegisterTables` continues enforcing the `ze_` prefix and `MustReserveTableNames` collision refusals (`docs/architecture/firewall/table-ownership-and-shutdown-flush.md` Rules 2 and 2b).
- One-time legacy table sweep (`legacy_tables.go`, Rule 2a) stays intact for upgrades from earlier builds.
- `flush-on-shutdown` (Rule 3) continues controlling orderly shutdown flush; keep-on-crash remains automatic.
- Operator tables without the `ze:firewall:v1` marker (even if named `ze_*`) and `ze_iface_rpf` (written outside `firewall/nft`, carrying no `ze:firewall:v1` marker) are never deleted.
- Rule replacement on an existing marked table remains atomic in one netfilter batch (zero unfiltered window).

**Behavior to change:**
- `firewall/nft` writes `NFTA_TABLE_USERDATA` (`ze:firewall:v1`) on every table it creates, reads `NFTA_TABLE_USERDATA` during `Apply`, and reclaims every marked table outside `desiredNames` even when `b.applied` is empty (fresh process after restart).
- When replacing an already-marked desired table, `Apply` clears and repopulates its objects/chains/sets/flowtables inside the single atomic `b.conn.Flush()` batch while keeping the table shell (preserving its kernel `NFTA_TABLE_USERDATA`, because Linux `nf_tables_newtable` only sets `udata` on table creation and ignores it on update).
- `firewall.StartupSweepPending()` (generalizing `LegacySweepPending()`) ensures the first startup reconcile reaches the kernel backend once even when `desired` is empty so orphaned marked tables are reclaimed when no firewall producer is configured.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| nftables backend `Apply` and `shouldDeleteTable` | `internal/plugins/firewall/nft/backend_linux.go` `backend.Apply`, `shouldDeleteTable`, `isLegacyTable` | Deletes `ze_` tables in `desiredNames` or process-local `b.applied`; calls `b.conn.Flush()` atomically | Replaces `b.applied` as the cross-restart ownership proof with kernel `NFTA_TABLE_USERDATA` (`ze:firewall:v1`); keeps `b.applied` and `isLegacyTable` for one-time migration of pre-marker tables |
| Firewall table registry and one-shot startup sweep | `internal/component/firewall/registry.go` `RegisterTables`, `ApplyAll`; `legacy_tables.go` `LegacySweepPending`, `legacySweepReached` | Merges owner tables, auto-loads OS default backend on first reconcile while `LegacySweepPending()` is true, calls `Backend.Apply` | Extends: keeps the one-shot startup sweep active for reclaiming orphaned `ze:firewall:v1` tables on boot even when `desired` is empty |
| Firewall table name reservation | `internal/component/firewall/reservation.go` `MustReserveTableNames`, `refuseForeignReservations`, `refuseReservedTableNames` | Refuses colliding owner registrations and operator tables colliding with reserved producer table names (already fixed at HEAD; `test/parse/firewall-reserved-table-name-rejected.ci`) | Preserves unchanged; AC-7 of the umbrella is already satisfied by this mechanism |
| Per-plugin startup sweeps (`ddos-local`, `flowspec-firewall`) | `internal/plugins/ddos/local/register.go` `clearStaleDropRule`; `internal/plugins/flowspec-firewall/engine.go` `clearStaleRules` | Two-reconcile claim+withdraw workaround for stale tables when the plugin restarts | Does not break: remains harmless and is superseded at the firewall layer for any producer that no longer starts |
| `mdlayher/netlink` netlink transport | `go.mod` `github.com/mdlayher/netlink v1.11.2` | Already linked as a direct dependency (used by `google/nftables` and other kernel code) | Extends: used by a small Linux-only `userdata_linux.go` helper in `internal/plugins/firewall/nft/` to send/read `NFTA_TABLE_USERDATA` (`type = 7`) over `NETLINK_NETFILTER` with zero new dependencies |

## Data Flow (MANDATORY)

### Entry Point
- Daemon startup or config reload calling `firewall.ApplyAll()` -> `(*backend).Apply(desired)`.

### Transformation Path
1. `(*backend).Apply(desired)` passes the host-netns safety gate (`refuseHostNetnsFirewall`).
2. `listTableUserData()` opens a `NETLINK_NETFILTER` socket via `mdlayher/netlink` (with `netlinkTimeout()`) and dumps `NFT_MSG_GETTABLE` (`NFNL_SUBSYS_NFTABLES = 10`, `NFT_MSG_GETTABLE = 1`), decoding `(Name, Family) -> UserData` from `NFTA_TABLE_NAME` (`1`) and `NFTA_TABLE_USERDATA` (`7`).
3. `b.conn.ListTables()` lists current tables. For each kernel table `ct`:
   - `marked := bytes.Equal(userData[key], zeTableUserDataMarker)` where `zeTableUserDataMarker = []byte("ze:firewall:v1")`.
   - If `sweepLegacy && b.isLegacyTable(ct)`: queue `b.conn.DelTable(ct)`.
   - Else if `marked`:
     - If `ct` is NOT in `desired` (by name+family): queue `b.conn.DelTable(ct)` (reclaims orphan from this or any previous process!).
     - If `ct` IS in `desired`: keep the table shell (preserving `NFTA_TABLE_USERDATA`), and queue deletion of its existing objects (rules/chains/sets/flowtables/ct-helpers) inside the same `b.conn` batch so replacement stays atomic and keeps its marker.
   - Else (unmarked table):
     - If `strings.HasPrefix(ct.Name, zeTablePrefix)` and (`ct` is in `desired` or in `b.applied`): queue `b.conn.DelTable(ct)` so pre-marker Ze tables migrate cleanly on upgrade.
     - Otherwise (foreign/operator table or `ze_iface_rpf`): leave completely untouched.
4. For every desired table that does NOT already exist as a marked table in the kernel:
   - First delete any unmarked same-name/family table queued in step 3 (or flush orphan deletes first if migrating an unmarked table), then create the table shell carrying `NFTA_TABLE_USERDATA = ze:firewall:v1` via a single batched `NFNL_MSG_BATCH_BEGIN` + `NFT_MSG_NEWTABLE(NLM_F_CREATE, NFTA_TABLE_NAME, NFTA_TABLE_USERDATA)` + `NFNL_MSG_BATCH_END` message on `NETLINK_NETFILTER`. An empty table with no base chains filters zero packets.
5. `applyTable` populates sets, flowtables, ct helpers, chains and rules onto every desired table (for an existing marked table, `b.conn.AddTable` is a kernel no-op that preserves `NFTA_TABLE_USERDATA`), and `b.conn.Flush()` commits all deletions and replacements atomically in one kernel transaction.
6. Update `b.applied = desiredNames`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `firewall/nft` -> Linux kernel `nf_tables` | `NETLINK_NETFILTER` (`NFT_MSG_GETTABLE` dump and `NFT_MSG_NEWTABLE` with `NFTA_TABLE_USERDATA = 7`) via `mdlayher/netlink`, plus `google/nftables` atomic `Flush()` | Linux UAPI `include/uapi/linux/netfilter/nf_tables.h` (`NFTA_TABLE_USERDATA = 7`, kernel >= 5.10; Ze ships Linux 7.x) |

### Changed-Wire Census

No network protocol wire format, RPC message, EventBus payload, YANG leaf, or persisted file format changes in this spec.

**Search evidence:** `NFTA_TABLE_USERDATA` (`ze:firewall:v1`) is a Linux kernel netfilter table attribute private to `internal/plugins/firewall/nft/` (written by `createMarkedTables` and read by `listTableUserData` in the same package). No other package or external plugin reads or writes it.

### Integration Points
- `internal/plugins/firewall/nft/userdata_linux.go` (new): `NFTA_TABLE_USERDATA` read/write helpers over `mdlayher/netlink` with `netlinkTimeout()`.
- `internal/plugins/firewall/nft/backend_linux.go`: `Apply`, `shouldDeleteTable`, in-place atomic replace of already-marked desired tables.
- `internal/plugins/firewall/nft/readback_linux.go`: recognize tables carrying `zeTableUserDataMarker` alongside `ze_` prefix.
- `internal/component/firewall/registry.go`, `legacy_tables.go`: ensure the one-shot startup sweep triggers even after legacy names are retired, so orphaned `ze:firewall:v1` tables are swept on startup when desired state is empty.
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`: document the `NFTA_TABLE_USERDATA` ownership marker alongside Rules 2, 2a, 2b and 4.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | All table programming still flows through `firewall.RegisterTables` -> `ApplyAll` -> `firewall/nft` |
| No unintended coupling | Yes | `ze_iface_rpf` and foreign tables carry no `ze:firewall:v1` marker and remain completely isolated |
| No duplicated functionality | Yes | Reuses `ApplyAll`'s one-shot startup sweep and `firewall/nft`'s netlink timeout and host-netns guard |
| Zero-copy preserved where applicable | Yes | Control-plane config apply only |
| Registration over hardcoding, outbound | Yes | Ownership is carried on the kernel table itself, never by a hardcoded table-name list |
| Registration over hardcoding, inbound | Yes | `shouldDeleteTable` reads kernel `NFTA_TABLE_USERDATA` rather than enumerating producers |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Fresh `firewallnft.newBackend()` `Apply(newDesired)` after a previous backend left a marked table | -> | `listTableUserData` + `shouldDeleteTable` | `TestRestartReclaimsMarkedTable` (`internal/plugins/firewall/nft/ownership_integration_linux_test.go`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Process 1 creates a firewall table through `firewall/nft` `Apply`, then exits without flushing; Process 2 starts with a fresh `backend` (empty `b.applied`) and calls `Apply` with a desired set that omits that table (including an empty desired set) | Process 1 stamps `NFTA_TABLE_USERDATA = ze:firewall:v1` on the kernel table, and Process 2 deletes that orphaned marked table from the kernel during `Apply` | Marked tables present in Process 2's `desired` set remain in the kernel with their updated rules; no orphaned `ze:firewall:v1`-marked table survives |
| AC-2 | An operator or external tool creates a table without the `ze:firewall:v1` user-data marker (both an unprefixed table `operator_tbl` and a `ze_`-prefixed table such as `ze_iface_rpf` or `ze_custom`) and `Apply` runs | `Apply` installs the desired `ze:firewall:v1` tables and preserves both `operator_tbl` and `ze_iface_rpf` / `ze_custom` intact in the kernel with their existing chains and rules unchanged | Every unmarked table (operator tables, external tools' tables, and `ze_iface_rpf`) remains permitted and untouched in the kernel because Ze owns only tables carrying `ze:firewall:v1` (plus one-time legacy sweep names) |
| AC-3 | A desired table already exists with the `ze:firewall:v1` marker and `Apply` updates its rules | The table retains its kernel `NFTA_TABLE_USERDATA = ze:firewall:v1` marker and its chains, sets, and rules are replaced atomically in one `Flush()` batch | The marked table shell itself survives across updates so kernel `NFTA_TABLE_USERDATA` is preserved and packet filtering has no unfiltered gap; obsolete chains, sets, and rules inside it do not survive |
| AC-4 | A pre-marker `ze_*` table from an older build is present in the kernel and is in the new `desired` set | `Apply` recreates the table carrying `NFTA_TABLE_USERDATA = ze:firewall:v1` with the desired chains and rules so subsequent restarts can reclaim it | Unmarked `ze_*` tables outside `desired` and `b.applied` (and not listed in `legacy_tables.go`) remain permitted untouched so foreign `ze_*` tables such as `ze_iface_rpf` are never reclaimed |
| AC-5 | `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | The page documents the `NFTA_TABLE_USERDATA = ze:firewall:v1` ownership marker, cross-restart reclaim contract, and in-place atomic rule replacement alongside Rules 2, 2a, 2b, and 4, and passes `./le doc check all` | Existing Rules 2, 2a, 2b, 3, and 4 (prefix check, one-time legacy sweep, name reservations, `flush-on-shutdown`) remain documented alongside the new marker because they remain enforced |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShouldDeleteTableOwnershipDecision` | `internal/plugins/firewall/nft/ownership_linux_test.go` | AC-1, AC-2, AC-4 (marked orphan deleted, unmarked foreign and `ze_iface_rpf` preserved, marked desired kept in-place, unmarked desired migrated) | |
| `TestDecodeTableUserData` | `internal/plugins/firewall/nft/ownership_linux_test.go` | AC-1, AC-2 (netlink attribute decode for `NFTA_TABLE_NAME` and `NFTA_TABLE_USERDATA`) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `NFTA_TABLE_USERDATA` payload | exact byte match `ze:firewall:v1` (14 bytes) | `ze:firewall:v1` | empty / shorter bytes (treated as unmarked) | extra trailing bytes or different version `ze:firewall:v2` (treated as unmarked) |

### Functional / Integration Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestRestartReclaimsMarkedTable` (`//go:build linux`, `userns.Enter`) | `internal/plugins/firewall/nft/ownership_integration_linux_test.go` | Real kernel netns: backend 1 applies `ze_stale` + `ze_keep`; foreign `operator_tbl` and `ze_iface_rpf` are created unmarked; fresh backend 2 applies only `ze_keep` (and then empty set) -> `ze_stale` is deleted, `ze_keep` retains its marker and updated rules, `operator_tbl` and `ze_iface_rpf` survive untouched (AC-1, AC-2, AC-3, AC-4) | |

### Interop Tests (Scope: config)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | Host Linux kernel netfilter ownership only; no network protocol peer | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add failing unit tests (`ownership_linux_test.go`) and Linux `userns.Enter` integration test `TestRestartReclaimsMarkedTable` (`ownership_integration_linux_test.go`).
2. **Phase: `NFTA_TABLE_USERDATA` helper** -- implement `internal/plugins/firewall/nft/userdata_linux.go` (`listTableUserData`, `createMarkedTables`, `hasZeTableMarker`) using `mdlayher/netlink` with `netlinkTimeout()`.
3. **Phase: `Apply` and startup sweep** -- update `backend_linux.go` (`Apply`, `shouldDeleteTable`, atomic in-place object replacement on marked tables), `readback_linux.go`, and `internal/component/firewall/registry.go` / `legacy_tables.go` one-shot startup sweep.
4. **Phase: Docs** -- update `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` and verify `./le doc check all`.

## Files to Modify
- `internal/plugins/firewall/nft/backend_linux.go`
- `internal/plugins/firewall/nft/readback_linux.go`
- `internal/component/firewall/registry.go`
- `internal/component/firewall/legacy_tables.go`
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`

## Files to Create
- `internal/plugins/firewall/nft/userdata_linux.go`
- `internal/plugins/firewall/nft/ownership_linux_test.go`
- `internal/plugins/firewall/nft/ownership_integration_linux_test.go`

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| gate-excludes-part-of-its-population | 2026-09-09 | `shouldDeleteTable` never reclaims a previous process's table |

## Work Not Done

Sibling children of `plan/release/fixes/spec-single-owner-reload-reconcile.md`:
- `plan/release/fixes/spec-single-owner-reload-reconcile-1-tx-conformance.md` (reload-wide pre-autoload verification and registered conformance suite)
- `plan/release/fixes/spec-single-owner-reload-reconcile-2-iface-reconcile.md` (iface owed-apply guard, VPP learned-address proof, carrier resync epoch, DHCPv6 `IFLA_CARRIER_CHANGES`)
- `plan/release/fixes/spec-single-owner-reload-reconcile-4-web-form-atomic-save.md` (atomic web form save)
- `plan/release/fixes/spec-single-owner-reload-reconcile-5-isis-identity-purge.md` (IS-IS old-SystemID LSP purge on `SystemID` change)

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Shared machinery you use rather than rebuild | the door every change reads | "Encode bytes" | agrees | Netlink `NFTA_TABLE_USERDATA` messages use `github.com/mdlayher/netlink` (already in `go.mod`) and existing nft backend batching without adding dependencies or breaking external contracts. |
| `docs/architecture/core-design.md` | 14b. Firewall and Traffic Control | the `// Design:` page of `backend_linux.go`, `readback_linux.go`, `registry.go`; anchored to `registry.go` | "Ze manages nftables firewall tables and Linux traffic control through the same pluggable backend pattern as interfaces." | agrees | Ownership marking stays encapsulated inside the Linux `firewall/nft` backend (`userdata_linux.go`, `backend_linux.go`) without changing the pluggable `firewall.Backend` interface or VPP backend. |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 2: a producer's table name carries the `ze_` prefix | anchored to `backend_linux.go`, `registry.go`; the `// Design:` page of `legacy_tables.go`; a page this spec modifies | "The kernel firewall backend uses `firewall.Table.Name` verbatim and adds nothing. It reconciles and deletes only kernel tables whose name starts with `ze_`." | agrees | Preserves `ze_` prefix enforcement and `legacy_tables.go` one-shot sweep, and updates this page to add kernel `NFTA_TABLE_USERDATA = ze:firewall:v1` so a restarted process reclaims previously marked `ze_` tables while leaving unmarked `ze_` tables (`ze_iface_rpf`, foreign tables) untouched. |
| `ai/doors/forwarding.md` | Who depends on it from outside | the door covering `internal/plugins/firewall/nft/*`, `internal/component/firewall/*` | "Ze owns only what it marked." | agrees | Storing `ze:firewall:v1` in `NFTA_TABLE_USERDATA` makes nft table ownership an explicit kernel mark that survives process restarts without claiming unmarked tables. |
| `docs/architecture/firewall/firewall-irr.md` | Decisions | anchored to `internal/component/firewall/registry.go` | — | silent | Covers IRR prefix-set firewall generation; IRR tables enter through `RegisterTables` and automatically inherit `NFTA_TABLE_USERDATA` marking with no IRR changes. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Kernel/VPP backend mismatch checks are untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Superseded for daemon start by the 2026-11-26 row below; this spec adds no startup refusal. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | VPP firewall verification is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Firewall backend selector naming (`kernel` / `vpp`) is preserved untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | BGP export next-hop rewriting is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | BGP export next-hop policy authoring is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Built-in BGP export chain compilation is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | BGP export chain next-hop folding is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | BGP explicit next-hop support is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | BGP originated vs forwarded next-hop policy is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Implied filter seam is untouched by this spec. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | No startup refusal is added; startup orphan sweep runs inside the normal non-fatal nft reconcile path. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Startup dataplane mismatch handling is untouched by this spec. |

## Design presentation

### What changes
- `internal/plugins/firewall/nft` writes `NFTA_TABLE_USERDATA = ze:firewall:v1` onto every table it creates (via `mdlayher/netlink`, zero new dependencies) and reads table user data on every `Apply`.
- `shouldDeleteTable` reclaims any `ze:firewall:v1`-marked table outside `desiredNames`, even on a fresh process with an empty `b.applied` map, while leaving unmarked foreign tables and `ze_iface_rpf` untouched.
- Already-marked desired tables are updated in place inside one atomic `b.conn.Flush()` batch so their `NFTA_TABLE_USERDATA` survives and packet filtering has no gap.
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` is updated to document the kernel user-data marker.

### Before and after
- **Before:** A `ze_*` table left by a crashed process whose owner plugin is no longer configured was skipped by `shouldDeleteTable` because the new process's `b.applied` map was empty.
- **After:** Every table created by `firewall/nft` carries `NFTA_TABLE_USERDATA = ze:firewall:v1` in the kernel, so a fresh process reclaims orphaned tables on startup while leaving unmarked operator tables and `ze_iface_rpf` untouched.

### Alternatives set aside
- **Sweeping every `ze_*` table by prefix alone:** Rejected because `ze_iface_rpf` is written directly by `internal/plugins/iface/netlink/rpf_linux.go` outside the firewall registry, and an operator can also own a foreign table prefixed `ze_`.
- **Upgrading or forking `github.com/google/nftables` to add `Table.UserData`:** Rejected because `github.com/mdlayher/netlink` is already a direct dependency in `go.mod`, avoiding third-party dependency churn (`ai/META-DECISIONS-SITUATIONAL.md`, "Third-party dependencies and licensing").
- **Re-creating every desired table in two separate netfilter batches:** Rejected because deleting a base-chain table in one batch and adding rules in a second batch opens a brief unfiltered packet window; keeping the marked table shell and replacing its chains/sets/rules inside one `b.conn.Flush()` keeps rule updates atomic.

### Documents departed from
None. (`docs/architecture/firewall/table-ownership-and-shutdown-flush.md` is updated in the same work to extend Rule 2 with the durable `NFTA_TABLE_USERDATA` ownership marker.)

### Choices for Thomas
none

## Review Gate

- [ ] `/ze-review` passed on diff
- [ ] `/ze-review-spec` confirmed all acceptance criteria and files

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Producer evidence re-read for every row
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-5 each proven by a named test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass in the main checkout: `./le test unit pkg path ./internal/plugins/firewall/nft/`, `./le test unit pkg path ./internal/component/firewall/`, and `./le test integration firewall`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Linux integration test using `//go:build linux` + `userns.Enter`

### Closure
- [ ] Owned journal row marked `fixed:` with commit and test evidence
