# Spec: op-0-umbrella — Operational Commands (VyOS Parity Tier)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/3 |
| Updated | 2026-04-18 |

## Scope Correction (2026-04-18)

After reading the source, five of the ten commands listed below are ALREADY
implemented on the main branch. The spec was written from the VyOS gap
analysis without first verifying ze's current state.

| # | Command | Actual state |
|---|---------|-------------|
| 3 | `show interfaces <type>` | Done -- `handleShowInterface` in `internal/component/cmd/show/show.go:136`, YANG leaf, `showInterfaceByType` filter |
| 6 | `ping` | Done -- `cmd/ze/diag/register.go:27` registers `RunPing` |
| 7 | `show bgp <family> summary` | Done -- `handleBgpSummary` filters on `NegotiatedFamilies`, rejects unknowns, tests in `summary_test.go` |
| 9 | `traceroute` | Done -- `cmd/ze/diag/register.go:28` registers `RunTraceroute` |
| 10 | `show system uptime` | Done -- `handleShowUptime` at `show.go:100`, YANG `ze-show:uptime` registered |

**Remaining scope (five commands):** 1, 2, 4, 5, 8. The spec body below is
kept verbatim for history; the Phases are renumbered to reflect the real
work:

- **Phase 1 (firewall):** #1 `show firewall ruleset` + #8 `show firewall group`
- **Phase 2 (interface/neighbor):** #4 `show ip arp` + #5 `clear interfaces counters`
- **Phase 3 (route):** #2 `show ip route`

**Corrections to claims in the spec body below:**

- Line "VPP has `sw_interface_dump` (already used) for stats" is wrong:
  `ifacevpp.GetStats` returns `errNotSupported` at `ifacevpp.go:435`. VPP
  stats reading is a separate task, out of scope here.
- Line "netlink neighbor table is event-monitored only today" is correct.
- Line "VPP FIB and VPP neighbor are WRITE-only today" is correct.
- Line "firewall nft `GetCounters` ... returns per-rule packet/byte counters
  grouped by chain" is wrong: `GetCounters` returns `ChainCounters` with an
  empty `Terms` slice -- kernel counter reading is stubbed. Wiring it is
  part of Phase 1.
- Line "netlink ResetCounters" is not generically supported by the Linux
  kernel; Phase 2 rejects on netlink with an exact-or-reject message and
  implements it on VPP via `sw_interface_clear_stats`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` and `.claude/rules/exact-or-reject.md`
3. `docs/guide/command-reference.md` (canonical ze command set)
4. `internal/component/iface/backend.go`, `internal/component/firewall/backend.go`
5. `internal/plugins/iface/netlink/`, `internal/plugins/iface/vpp/`,
   `internal/plugins/fib/kernel/`, `internal/plugins/fib/vpp/`,
   `internal/plugins/firewall/nft/`
6. `internal/component/bgp/plugins/cmd/peer/peer.go` (RegisterRPCs pattern)
7. `cmd/ze/main.go` `registerLocalCommands()` (offline CLI table)

## Task

Add the ten operational commands derived from the VyOS gap analysis
(2026-04-18) to ze, with working implementations on BOTH the netlink
(Linux) backend AND the VPP backend where the command is backend-specific.
Commands that do not touch dataplane state are userspace-only and apply
uniformly.

### The ten commands

| # | Command | Backend dependency |
|---|---------|--------------------|
| 1 | `show firewall ruleset <name>` | netlink (nftables) + VPP (ACL/classifier) |
| 2 | `show ip route [prefix]` | kernel FIB + VPP FIB |
| 3 | `show interfaces <type>` | netlink + VPP |
| 4 | `show ip arp` | netlink + VPP neighbor |
| 5 | `clear interfaces counters [name]` | netlink + VPP |
| 6 | `ping <target>` | userspace (no backend split) |
| 7 | `show bgp <family> summary` | pure BGP (no backend split) |
| 8 | `show firewall group <name>` | config-driven (no backend split) |
| 9 | `traceroute <target>` | userspace (no backend split) |
| 10 | `show system uptime` | pure (no backend split) |

### Exact-or-reject constraint

Per `.claude/rules/exact-or-reject.md`: where a command has a
backend-specific read path and the active backend for the relevant
component does NOT yet implement the read, the command MUST reject at
dispatch with a clear message naming the backend. Silent "returns
empty" is banned.

Concrete instances today:

| Command | Missing backend | Dispatch error |
|---------|-----------------|----------------|
| `show firewall ruleset` | VPP (no firewall backend exists yet) | `firewall: backend vpp not supported; configure backend nftables or defer` |
| `show firewall group` | none (config-driven) | n/a |

All other Top-10 commands MUST work on both backends in this spec.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/command-reference.md` - canonical command catalogue. New
      commands land here.
  → Constraint: two-column table shape (`Command` / `Description`) per
    existing sections; no prose.
- [ ] `docs/architecture/api/commands.md` - RPC registration narrative.
  → Constraint: every new runtime command needs a YANG schema under the
    owning plugin's `schema/` directory.
- [ ] `docs/architecture/overview.md` - small-core + registration.
  → Decision: these are operational commands on existing components
    (iface, firewall, bgp); no new component is created.
- [ ] `.claude/patterns/cli-command.md` - CLI/RPC registration pattern.
  → Constraint: RPC handlers live in the component plugin, not the
    top-level component package.
- [ ] `.claude/rules/exact-or-reject.md` - silent approximation banned.
  → Constraint: VPP-missing backends reject at dispatch time, not
    "return empty".
- [ ] `.claude/rules/enum-over-string.md` - typed numeric over string.
  → Constraint: interface type filter for `show interfaces <type>`
    uses the existing iface-type enum, not the YANG name string, on
    the hot path.

### RFC Summaries

None - operational commands are internal surface; no protocol RFCs apply.

**Key insights:**
- Ze's iface and firewall backend abstractions already expose the read
  APIs (`ListInterfaces`, `GetStats`, `ListTables`, `GetCounters`);
  commands 3, 5, and 1-on-netlink mostly reuse existing backend calls.
- VPP has no firewall backend yet: command 1 on VPP rejects at verify.
- VPP FIB and VPP neighbor are WRITE-only today; this spec adds the
  corresponding DUMP calls (`ip_route_v2_dump`, `ip_neighbor_dump`).
- Linux neighbor table is event-monitored only today; this spec adds
  a list call (`netlink.NeighList()`) for one-shot read.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` - Backend interface:
      `ListInterfaces`, `GetInterface`, `GetStats` exist; NO
      `ListNeighbors`, NO `ResetCounters`.
  → Constraint: add `ListNeighbors(ctx, family)` and `ResetCounters(ctx, name)`
    to the Backend interface - both impls must implement; older impls
    that cannot implement reject at Apply time per exact-or-reject.
- [ ] `internal/plugins/iface/netlink/backend_linux.go` - netlink LinkList
      populates stats via RTM_GETLINK; no neighbor list method today;
      counter reset is not implemented.
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - GoVPP `SwInterfaceDump`
      for stats; no neighbor dump call; no counter reset.
- [ ] `internal/plugins/fib/kernel/fibkernel.go` -
      `showInstalled()` returns currently-installed routes as JSON
      (ze-programmed only, RTPROT_ZE=250). For `show ip route` we need
      the full kernel FIB, not only ze-programmed routes.
- [ ] `internal/plugins/fib/vpp/backend.go` - `addRoute/delRoute/replaceRoute`
      only; NO `ListRoutes`. Must be added via `ip_route_v2_dump`.
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - `ListTables`,
      `GetCounters` exist and return per-rule counters grouped by chain;
      no ruleset-by-name API, but the name is accessible via `Table.Name`.
- [ ] `internal/component/firewall/model.go` - `Table` carries `Chains`
      with full rule list; `ChainCounters` pairs with chain by name.
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - `RegisterRPCs`
      pattern with `WireMethod` prefix (`ze-bgp:`, `ze-show:`, etc.) and
      `RequiresSelector` flag. Model for new handlers.
- [ ] `cmd/ze/main.go` `registerLocalCommands` (lines ~96-198) - offline
      CLI path for commands that work without the daemon (ping /
      traceroute / `show interfaces <type>` in read-only offline mode).

**Behavior to preserve:**
- Existing `peer`, `rib`, `commit`, `bgp monitor`, `bgp summary`, `rib routes`
  commands. No rename, no resignature.
- Existing `ze show interface` offline subcommand output (stats + addrs).
  New `<type>` filter MUST not change default (no-type) output.
- Existing firewall JSON format for `firewall show` (model_test.go
  covers it). `show firewall ruleset` uses the same envelope.
- Existing `bgp summary` output (without family arg). `show bgp <family>
  summary` is a parallel command, not a replacement.

**Behavior to change:**
- None requested. All ten commands are additive.

## Data Flow

### Entry Point
- Runtime: `ze cli` / `ze cli -c "<cmd>"` / `ze show <cmd>` over SSH →
  CLI dispatcher parses tokens → matches a `WireMethod` from the
  component plugin registry → calls the registered handler.
- Offline (selected commands only): `ze <plugin> <subcommand>` shell entry
  point; uses local backend read path without a running daemon where
  feasible (ping, traceroute always local; interface show already local).

### Transformation Path
1. CLI tokens → dispatcher → WireMethod lookup.
2. Handler receives `*pluginserver.CommandContext` + args.
3. Handler calls backend read method (`ListNeighbors`, `ListRoutes`,
   `ListInterfaces`, `GetCounters`, etc.) via component's current
   active backend.
4. Backend-specific impl executes:
   - **netlink**: `netlink.NeighList()`, `netlink.RouteList()`,
     `netlink.LinkList()`, `nft.GetCounters()`.
   - **VPP**: `ip_neighbor_dump`, `ip_route_v2_dump`,
     `sw_interface_dump` (already used), no VPP firewall.
5. Handler formats structured reply (JSON envelope identical to every
   other ze surface: `{"cmd":"...","data":{...}}`).
6. Pipe operators (`| json`, `| table`, `| match`) render in-shell.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ Plugin | WireMethod dispatch (existing) | [ ] |
| Plugin ↔ Backend | Component Backend interface (existing + new methods) | [ ] |
| Backend ↔ Kernel/VPP | netlink syscall or GoVPP RPC | [ ] |

### Integration Points
- `internal/component/iface/backend.go`: ADD `ListNeighbors`, `ResetCounters`
  to the `Backend` interface. Both netlink and VPP impls implement.
- `internal/plugins/fib/vpp/backend.go`: ADD `ListRoutes`.
- `internal/plugins/fib/kernel/backend_linux.go`: ADD `ListRoutes` that
  returns the full kernel RIB (not just ze-programmed).
- `internal/component/bgp/reactor/`: expose per-family summary reader for
  `show bgp <family> summary` (reuse existing summary producer with an
  AFI/SAFI filter).
- `internal/component/firewall/backend.go`: `GetRuleset(tableName)` maps
  onto existing `ListTables()` + `GetCounters(tableName)`; no new
  backend method needed.

### Architectural Verification
- [ ] No bypassed layers (commands reach backends through the existing
      Backend interface only)
- [ ] No unintended coupling (no plugin reaches into another plugin's
      Backend; cross-component reads use the component's public API)
- [ ] No duplicated functionality (reuse existing ListInterfaces,
      GetStats, GetCounters; extend only where read is missing)
- [ ] Zero-copy preserved: handlers format output directly, no
      intermediate struct copying of backend replies

## Wiring Test (MANDATORY — NOT deferrable)

Each command has its own entry-point-to-backend wiring test.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze cli -c "show firewall ruleset test"` (nftables) | → | `internal/component/firewall/cmd/show.go:HandleRuleset` | `test/op/firewall-ruleset-nft.ci` |
| `ze cli -c "show firewall ruleset test"` (VPP active) | → | same handler rejects | `test/op/firewall-ruleset-vpp-reject.ci` |
| `ze cli -c "show ip route 10.0.0.0/8"` (netlink) | → | `internal/plugins/fib/kernel/cmd/show.go:HandleRoute` | `test/op/ip-route-netlink.ci` |
| `ze cli -c "show ip route 10.0.0.0/8"` (VPP) | → | `internal/plugins/fib/vpp/cmd/show.go:HandleRoute` | `test/op/ip-route-vpp.ci` |
| `ze cli -c "show interfaces bridge"` (netlink) | → | `internal/component/iface/cmd/show.go:HandleByType` | `test/op/interfaces-by-type-netlink.ci` |
| `ze cli -c "show interfaces bridge"` (VPP) | → | same handler via VPP backend | `test/op/interfaces-by-type-vpp.ci` |
| `ze cli -c "show ip arp"` (netlink) | → | `internal/component/iface/cmd/show.go:HandleArp` | `test/op/arp-netlink.ci` |
| `ze cli -c "show ip arp"` (VPP) | → | same handler via VPP backend | `test/op/arp-vpp.ci` |
| `ze cli -c "clear interfaces counters eth0"` (netlink) | → | `internal/component/iface/cmd/clear.go:HandleCounters` | `test/op/clear-counters-netlink.ci` |
| `ze cli -c "clear interfaces counters eth0"` (VPP) | → | same handler via VPP backend | `test/op/clear-counters-vpp.ci` |
| `ze cli -c "ping 10.0.0.1"` | → | `cmd/ze/diag/ping.go:Run` | `test/op/ping.ci` |
| `ze cli -c "show bgp ipv6 summary"` | → | `internal/component/bgp/plugins/cmd/summary/family.go:Handle` | `test/op/bgp-family-summary.ci` |
| `ze cli -c "show firewall group <name>"` | → | `internal/component/firewall/cmd/group.go:Handle` | `test/op/firewall-group.ci` |
| `ze cli -c "traceroute 10.0.0.1"` | → | `cmd/ze/diag/traceroute.go:Run` | `test/op/traceroute.ci` |
| `ze cli -c "show system uptime"` | → | `internal/component/engine/cmd/uptime.go:Handle` | `test/op/system-uptime.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1  | `show firewall ruleset <name>` with nftables active | JSON envelope lists rules in the named table with per-rule packet/byte counters |
| AC-2  | `show firewall ruleset <name>` with VPP active | Dispatch returns error `firewall: backend vpp not supported` (per exact-or-reject); exit code non-zero |
| AC-3  | `show ip route` on Linux | JSON lists every kernel route (not only RTPROT_ZE) with prefix, nexthop, device, protocol, metric |
| AC-4  | `show ip route <prefix>` on Linux | Filters to exact prefix match; empty result if not present |
| AC-5  | `show ip route` on VPP | Same envelope as netlink; uses `ip_route_v2_dump` |
| AC-6  | `show interfaces <type>` (both backends) | Output filtered to ifaces of given type (`ethernet`, `bridge`, `veth`, `wireguard`, `vxlan`, `tunnel`, `bond`, `loopback`); unknown type rejects with valid list |
| AC-7  | `show ip arp` on Linux | Lists neighbors from `RTM_GETNEIGH` with IP, MAC, device, state |
| AC-8  | `show ip arp` on VPP | Same envelope using `ip_neighbor_dump` |
| AC-9  | `clear interfaces counters` (no name) on both backends | Resets counters on every interface the backend manages; idempotent |
| AC-10 | `clear interfaces counters <name>` on both backends | Resets counters on the named interface only; unknown name returns error with valid list |
| AC-11 | `ping <target>` | ICMP echo via OS; respects `--count`, `--interval`, `--source`, `--interface` flags; exit code reflects success |
| AC-12 | `show bgp ipv4 summary`, `show bgp ipv6 summary` | Same envelope as `bgp summary` filtered by AFI/SAFI; peers not negotiated for the family are omitted |
| AC-13 | `show bgp <invalid-family> summary` | Rejects with valid family list |
| AC-14 | `show firewall group <name>` | Lists group members (address / network / port / interface) from parsed config; unknown group returns error with valid list |
| AC-15 | `traceroute <target>` | Path trace using OS `traceroute` (or equivalent); flags for count and interface |
| AC-16 | `show system uptime` | Returns daemon start time, running duration, host uptime |
| AC-17 | All commands | Respect `\| json`, `\| table`, `\| match`, `\| count` pipe operators |
| AC-18 | `show ip route`, `show ip arp`, `show interfaces <type>` | `--family ipv4\|ipv6` flag narrows output; default is both |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIfaceBackend_ListNeighbors_Linux` | `internal/plugins/iface/netlink/neighbor_linux_test.go` | netlink NeighList returns expected entries from a fake Conn | |
| `TestIfaceBackend_ListNeighbors_VPP` | `internal/plugins/iface/vpp/neighbor_test.go` | VPP ip_neighbor_dump parsing | |
| `TestIfaceBackend_ResetCounters_Linux` | `internal/plugins/iface/netlink/counters_linux_test.go` | netlink writes zero-stats message | |
| `TestIfaceBackend_ResetCounters_VPP` | `internal/plugins/iface/vpp/counters_test.go` | VPP sw_interface_clear_stats call | |
| `TestFibBackend_ListRoutes_Kernel` | `internal/plugins/fib/kernel/list_linux_test.go` | Returns all kernel routes, not only RTPROT_ZE | |
| `TestFibBackend_ListRoutes_VPP` | `internal/plugins/fib/vpp/list_test.go` | ip_route_v2_dump parsing | |
| `TestFirewallHandleRuleset_Reject_VPP` | `internal/component/firewall/cmd/show_test.go` | VPP backend → error naming backend | |
| `TestIfaceFilterByType` | `internal/component/iface/cmd/show_test.go` | Type filter matches registered iface types; unknown rejects | |
| `TestBgpFamilySummary` | `internal/component/bgp/plugins/cmd/summary/family_test.go` | Peer list filtered by AFI/SAFI | |
| `TestFirewallGroupLookup` | `internal/component/firewall/cmd/group_test.go` | Group members returned from config; unknown rejects | |
| `TestEngineUptime` | `internal/component/engine/cmd/uptime_test.go` | Start time + duration + host uptime present | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ping --count` | 1..100000 | 100000 | 0 | 100001 |
| `ping --interval` (ms) | 100..60000 | 60000 | 99 | 60001 |
| `traceroute --hops` | 1..64 | 64 | 0 | 65 |
| prefix for `show ip route` | CIDR | valid | `10.0.0.0` (no mask) | `10.0.0.0/33` |

### Functional Tests (`.ci`)
See Wiring Test table. Each row is a required `.ci`. No deferrals.

### Future (deferred)
- `monitor bandwidth` (VyOS's per-link rate display) - not in this spec.
- `clear ip arp` (per-entry flush) - belongs to a future arp-management spec.

## Files to Modify

### Component abstractions
- `internal/component/iface/backend.go` - add `ListNeighbors`, `ResetCounters`.
- `internal/component/firewall/backend.go` - add `GetRuleset(name)` (helper
  over existing `ListTables`+`GetCounters`; thin wrapper).

### Netlink (Linux) backend
- `internal/plugins/iface/netlink/backend_linux.go` - extend Backend impl.
- `internal/plugins/iface/netlink/neighbor_linux.go` (new).
- `internal/plugins/iface/netlink/counters_linux.go` (new).
- `internal/plugins/fib/kernel/list_linux.go` (new) - full kernel FIB read.
- `internal/plugins/fib/kernel/fibkernel.go` - register `ListRoutes` handler.
- `internal/plugins/firewall/nft/backend_linux.go` - no change (GetCounters
  already sufficient).

### VPP backend
- `internal/plugins/iface/vpp/neighbor.go` (new) - `ip_neighbor_dump`.
- `internal/plugins/iface/vpp/counters.go` (new) - `sw_interface_clear_stats`.
- `internal/plugins/fib/vpp/list.go` (new) - `ip_route_v2_dump`.

### Component handlers (online RPC)
- `internal/component/iface/cmd/show.go` - `HandleByType`, `HandleArp`.
- `internal/component/iface/cmd/clear.go` (new) - `HandleCounters`.
- `internal/component/iface/cmd/register.go` - register new RPCs.
- `internal/component/iface/schema/iface.yang` - YANG for new RPCs.
- `internal/component/firewall/cmd/show.go` - `HandleRuleset`.
- `internal/component/firewall/cmd/group.go` (new) - `Handle`.
- `internal/component/firewall/cmd/register.go` - register RPCs.
- `internal/component/firewall/schema/firewall.yang` - YANG.
- `internal/plugins/fib/kernel/cmd/show.go` (new) - `HandleRoute`.
- `internal/plugins/fib/vpp/cmd/show.go` (new) - `HandleRoute`.
- `internal/component/bgp/plugins/cmd/summary/family.go` (new).
- `internal/component/engine/cmd/uptime.go` (new).

### Offline CLI entry points
- `cmd/ze/diag/ping.go` (new).
- `cmd/ze/diag/traceroute.go` (new).
- `cmd/ze/main.go` - add `ping`, `traceroute`, and new `show ...` paths
  to `registerLocalCommands()`.

### Docs
- `docs/guide/command-reference.md` - add every new command.
- `docs/guide/cli.md` - add new command groups.
- `docs/features/cli-commands.md` - feature-level catalogue.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/{iface,firewall,bgp,engine}/schema/*.yang` |
| CLI commands/flags | [x] | `cmd/ze/main.go` local registrations + `cmd/ze/diag/` new package |
| Editor autocomplete | [x] | YANG-driven (automatic once YANG lands) |
| Functional test for new RPC/API | [x] | `test/op/*.ci` (see Wiring Test) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | n/a |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md`, `docs/guide/cli.md`, `docs/features/cli-commands.md` |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | n/a |
| 6 | Has a user guide page? | [x] | `docs/guide/operations.md` |
| 7 | Wire format changed? | [ ] | n/a |
| 8 | Plugin SDK/protocol changed? | [ ] | n/a |
| 9 | RFC behavior implemented? | [ ] | n/a |
| 10 | Test infrastructure changed? | [x] | `docs/functional-tests.md` (new `test/op/` directory) |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [x] | `docs/architecture/overview.md` (backend iface additions) |

## Files to Create

Listed above inline under "Files to Modify" where (new). Summary paths:

- `internal/plugins/iface/netlink/neighbor_linux.go`
- `internal/plugins/iface/netlink/counters_linux.go`
- `internal/plugins/iface/vpp/neighbor.go`
- `internal/plugins/iface/vpp/counters.go`
- `internal/plugins/fib/kernel/list_linux.go`
- `internal/plugins/fib/vpp/list.go`
- `internal/plugins/fib/kernel/cmd/show.go`
- `internal/plugins/fib/vpp/cmd/show.go`
- `internal/component/iface/cmd/clear.go`
- `internal/component/firewall/cmd/group.go`
- `internal/component/bgp/plugins/cmd/summary/family.go`
- `internal/component/engine/cmd/uptime.go`
- `cmd/ze/diag/ping.go`
- `cmd/ze/diag/traceroute.go`
- `test/op/firewall-ruleset-nft.ci`
- `test/op/firewall-ruleset-vpp-reject.ci`
- `test/op/ip-route-netlink.ci`
- `test/op/ip-route-vpp.ci`
- `test/op/interfaces-by-type-netlink.ci`
- `test/op/interfaces-by-type-vpp.ci`
- `test/op/arp-netlink.ci`
- `test/op/arp-vpp.ci`
- `test/op/clear-counters-netlink.ci`
- `test/op/clear-counters-vpp.ci`
- `test/op/ping.ci`
- `test/op/bgp-family-summary.ci`
- `test/op/firewall-group.ci`
- `test/op/traceroute.ci`
- `test/op/system-uptime.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Implement (TDD) | Phases 1–5 below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-verify-fast` |
| 6. Critical review | Critical Review Checklist |
| 7–12 | Fix / re-verify / present |

### Implementation Phases

Phases are ordered so each lands independently. Any phase MAY be spun
out to its own child spec (`spec-op-N-<name>.md`) if the work grows;
until then, the umbrella carries them.

1. **Phase 1: Backend read-path extensions** (commands 2, 3, 4, 5 precondition)
   - Add `ListNeighbors`, `ResetCounters` to `iface.Backend`. Implement
     on netlink (`neighbor_linux.go`, `counters_linux.go`) and VPP
     (`neighbor.go`, `counters.go`).
   - Add `ListRoutes` to fib-kernel and fib-vpp.
   - Tests: unit tests for each backend impl.
   - Verify: `go test ./internal/plugins/iface/... ./internal/plugins/fib/...` passes.

2. **Phase 2: Firewall commands** (commands 1, 8)
   - Wire `show firewall ruleset <name>` handler on top of existing
     `GetCounters`. Reject on VPP backend per exact-or-reject.
   - Wire `show firewall group <name>` from parsed config.
   - Tests: unit + `test/op/firewall-*.ci`.

3. **Phase 3: Interface + neighbor commands** (commands 3, 4, 5)
   - `show interfaces <type>` filter (enum-typed).
   - `show ip arp` (calls ListNeighbors).
   - `clear interfaces counters [name]`.
   - Tests: unit + `test/op/interfaces-by-type-*.ci`, `test/op/arp-*.ci`,
     `test/op/clear-counters-*.ci`.

4. **Phase 4: Route command** (command 2)
   - `show ip route [prefix]` on both backends.
   - Tests: unit + `test/op/ip-route-*.ci`.

5. **Phase 5: Userspace-only commands** (commands 6, 7, 9, 10)
   - `ping`, `traceroute` (offline subcommand + online passthrough).
   - `show bgp <family> summary` (reuse existing summary producer).
   - `show system uptime`.
   - Tests: unit + `test/op/{ping,bgp-family-summary,traceroute,system-uptime}.ci`.

6. **Functional verification**
   - `make ze-verify-fast` passes.
   - Every `.ci` in `test/op/` green on both backends where applicable.

7. **Docs + learned summary**
   - Update the six doc files above.
   - Write `plan/learned/NNN-op-0-umbrella.md`.
   - Delete spec per two-commit sequence.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a test and a handler file:line |
| Correctness | VPP-reject commands actually reject (don't return empty); netlink ResetCounters resets on kernel side (verify via `/proc/net/dev`) |
| Naming | `show interfaces <type>` uses enum for type, not string |
| Data flow | Handlers call component Backend only; no plugin reaches into another plugin's Backend |
| Rule: exact-or-reject | Missing VPP firewall backend rejects with backend name in the message |
| Rule: no-layering | No legacy "return empty when backend missing" path left in |
| Rule: integration-completeness | Every command has a `.ci` wiring test on every backend it targets |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| All 10 commands reachable via `ze cli -c "<cmd>"` | Each `.ci` exercises the command; grep `test/op/*.ci` for command name |
| `command-reference.md` lists all 10 | `grep -c 'show firewall ruleset\|show ip route\|show interfaces\|show ip arp\|clear interfaces counters\|ping\|show bgp\|show firewall group\|traceroute\|show system uptime' docs/guide/command-reference.md` ≥ 10 |
| YANG schemas land | `ls internal/component/*/schema/*.yang` shows new entries |
| Backend interface additions | `grep 'ListNeighbors\|ResetCounters\|ListRoutes' internal/component/iface/backend.go internal/plugins/fib/kernel/backend_linux.go internal/plugins/fib/vpp/backend.go` matches new methods |
| exact-or-reject enforced | `test/op/firewall-ruleset-vpp-reject.ci` asserts non-zero exit + error string |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `prefix` for `show ip route` validated as CIDR (reject `10.0.0.0` without mask; reject `/33`). Interface name validated against backend list (prevent shelling out with attacker-controlled names). |
| Shell injection | `ping` / `traceroute` wrappers MUST use `exec.Command` with argument list, never `sh -c`; target sanitised (hostname/IP only; no shell metachars) |
| Resource exhaustion | `show ip route` streams large RIBs instead of buffering (FIB may have 1M entries); same for `show ip arp` on large LANs |
| Error leakage | No kernel pointer / raw errno exposed to operator; errors wrapped with command context |
| Privilege | `clear interfaces counters` is write on netlink + VPP; require authz role `operator` via existing authz check |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| `make ze-verify-fast` fails on unrelated tests | Log in `plan/known-failures.md` (per rules/anti-rationalization.md) |
| VPP dump parsing mismatches the vendored govpp | `rules/memory.md` "Vendored Library != Upstream" - add the actual govpp version's dump types |
| Exact-or-reject test fails because command returned empty | Re-dispatch path: must return error, not empty reply |
| 3 fix attempts fail | STOP, report all 3, ask user |

## Mistake Log

### Wrong Assumptions
_(to be filled during implementation)_

### Failed Approaches
_(to be filled)_

### Escalation Candidates
_(to be filled)_

## Design Insights
_(to be filled live during implementation)_

## RFC Documentation

No RFC constraints on operational commands.

## Implementation Summary
_(to be filled at completion)_

## Implementation Audit
_(tables filled at completion)_

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   |          |         |          |        |

### Fixes applied
- _(to be filled)_

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
|      |        |          |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
|       |       |                |

### Wiring Verified
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
|             |          |          |

## Checklist

### Goal Gates
- [ ] AC-1..AC-18 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-test` passes
- [ ] Feature code integrated under `internal/*`, `cmd/*`
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates
- [ ] RFC constraint comments added (n/a for operational commands)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (only new methods where required)
- [ ] No speculative features (exactly the ten commands)
- [ ] Single responsibility per handler
- [ ] Explicit > implicit (enum for interface type, not string)
- [ ] Minimal coupling (handlers read via component Backend)

### TDD
- [ ] Tests written first
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for `ping`, `traceroute`, `show ip route` inputs
- [ ] Functional `.ci` tests on both backends where applicable

### Completion
- [ ] Critical Review documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-op-0-umbrella.md`
- [ ] Summary included in commit
