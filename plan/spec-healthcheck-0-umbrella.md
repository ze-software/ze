# Spec: healthcheck-0-umbrella

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/watchdog/server.go` - watchdog command dispatch
4. `internal/component/bgp/plugins/watchdog/pool.go` - route pool management
5. `internal/component/bgp/plugins/watchdog/config.go` - watchdog config parsing
6. `internal/component/bgp/format.go` - FormatAnnounceCommand/FormatWithdrawCommand
7. `internal/component/iface/manage_linux.go` - AddAddress/RemoveAddress
8. `pkg/plugin/sdk/sdk_engine.go` - DispatchCommand, UpdateRoute
9. `.claude/patterns/plugin.md` - plugin structural template

## Task

Implement a healthcheck component for Ze with full feature parity with ExaBGP's healthcheck program. Healthcheck monitors service availability by running shell commands periodically and controls BGP route announcement/withdrawal via watchdog groups. Supports both withdraw-on-down (route disappears) and metric-based (MED override) modes through a single watchdog tag.

**Reference implementation:** `~/Code/github.com/exa-networks/exabgp/src/exabgp/application/healthcheck.py`

### ExaBGP Feature Mapping

| ExaBGP feature | Ze mapping |
|---------------|------------|
| `--command` | `command` leaf in YANG |
| `--interval` / `--fast-interval` | `interval` / `fast-interval` leaves |
| `--timeout` | `timeout` leaf |
| `--rise` / `--fall` | `rise` / `fall` leaves |
| `--disable` (file path) | `disable` boolean (admin disable via config) |
| `--ip` (multiple VIPs) | `ip-setup { ip <cidr> }` leaf-list |
| `--ip-setup` / `--dynamic-ip-setup` | `ip-setup { dynamic true }` |
| `--label` / `--label-exact-match` | Dropped -- IPs tracked internally, no label discovery needed |
| `--up-metric` / `--down-metric` / `--disabled-metric` | `up-metric` / `down-metric` / `disabled-metric` |
| `--withdraw-on-down` | Implicit: if `down-metric` unset, withdraw; if set, re-announce with MED override |
| `--community` etc. per-state | Route attributes live in BGP config watchdog route definitions |
| `--deaggregate-networks` | User defines individual prefixes in BGP config |
| `--start-ip` / `--increase` | User defines per-route attributes in BGP config |
| `--neighbor` targeting | Implicit: watchdog groups are per-peer in BGP config |
| `--execute` / `--up-execute` / `--down-execute` / `--disabled-execute` | `on-change` / `on-up` / `on-down` / `on-disabled` |
| `--debounce` | `debounce` boolean |
| `--no-ack` | Not applicable (Ze uses synchronous RPC) |
| `--pid` / `--user` / `--group` / `--syslog-facility` | Not applicable (Ze manages its own process/logging) |
| `--sudo` | Not applicable (Ze uses netlink directly) |
| External process model | Both internal (goroutine) and external (TLS connect-back) plugin modes |

### Features NOT in ExaBGP (new in Ze)

| Feature | Description |
|---------|-------------|
| MED override via watchdog | Single tag with `watchdog announce <name> med <N>` -- no need for multiple route definitions per state |
| Internal + external mode | Same plugin code runs as goroutine (with IP management) or external process (without) |
| YANG-modeled config | Schema validation, editor completions, web form support |
| CLI commands | `show healthcheck`, `show healthcheck <name>`, `reset healthcheck <name>` |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall component architecture
  -> Constraint: components register at startup via init(), communicate via bus/RPC
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage plugin protocol
  -> Constraint: all plugins follow 5-stage startup, tier ordering via Dependencies
- [ ] `docs/architecture/api/update-syntax.md` - route command syntax
  -> Constraint: text commands use `update text ... med set N ... nlri ...` format
- [ ] `.claude/patterns/plugin.md` - plugin structural template
  -> Constraint: atomic logger, register.go with init(), schema/ subdir for YANG

### RFC Summaries (MUST for protocol work)
N/A -- healthcheck is operational tooling, not a BGP protocol feature.

**Key insights:**
- Healthcheck is a root-level component (like iface, web), not a BGP plugin
- It participates in the plugin protocol to dispatch watchdog commands via DispatchCommand
- Route attributes (next-hop, community, as-path) are defined in BGP config on watchdog routes
- Healthcheck only controls announce/withdraw lifecycle and optional MED override
- iface.AddAddress/RemoveAddress use netlink (no subprocess) for IP management

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/watchdog/server.go` - handles `watchdog announce <name>` and `watchdog withdraw <name>` commands. Dispatches to per-peer route pools. Supports `*` wildcard for all peers. Sends pre-computed AnnounceCmd/WithdrawCmd strings.
  -> Constraint: announce command does NOT currently support MED override -- AnnounceCmd is pre-computed at config load time. Must extend to accept optional `med <N>` argument.
- [ ] `internal/component/bgp/plugins/watchdog/pool.go` - PoolEntry stores pre-computed AnnounceCmd and WithdrawCmd strings. Per-peer announced/withdrawn state tracking. Thread-safe via RWMutex.
  -> Constraint: MED override must modify the pre-computed command at dispatch time, not at pool creation time.
- [ ] `internal/component/bgp/plugins/watchdog/config.go` - parses BGP config JSON tree, builds Route structs, calls FormatAnnounceCommand/FormatWithdrawCommand to pre-compute commands.
  -> Constraint: no changes needed to config parsing -- watchdog routes are defined as-is.
- [ ] `internal/component/bgp/format.go` - FormatAnnounceCommand builds `update text origin set ... med set N ... nhop ... nlri ...` strings. MED is written as `med set N` if Route.MED is non-nil.
  -> Decision: MED override in watchdog announce replaces or injects `med set N` in the pre-computed command string.
- [ ] `internal/component/iface/manage_linux.go:194-234` - AddAddress(ifaceName, cidr) and RemoveAddress(ifaceName, cidr). Use netlink.AddrAdd/AddrDel. Validate interface names (1-15 chars). Accept IPv4/IPv6 CIDR.
  -> Constraint: no label support in iface. Healthcheck tracks managed IPs internally.
- [ ] `pkg/plugin/sdk/sdk_engine.go:40-55` - DispatchCommand(ctx, command) dispatches through engine's command dispatcher to any plugin. Returns (status, data, error).
  -> Decision: healthcheck uses DispatchCommand for watchdog commands, not UpdateRoute directly.

**Behavior to preserve:**
- Watchdog announce/withdraw semantics unchanged for existing users
- Pre-computed AnnounceCmd/WithdrawCmd still the default (no MED arg = use pre-computed)
- Peer state tracking (announced/withdrawn per peer) unchanged
- Reconnect resend behavior (announced routes resent on peer up) unchanged

**Behavior to change:**
- Watchdog `announce` command extended to accept optional `med <N>` argument
- New root-level `healthcheck {}` YANG container
- New component under `internal/component/healthcheck/`

## Data Flow (MANDATORY)

### Entry Point
- Shell command execution result (exit code 0 = success, non-zero = failure)
- Timer tick (interval / fast-interval)
- Config change (YANG healthcheck container)

### Transformation Path
1. Timer fires -> probe goroutine runs shell command with timeout
2. Exit code -> FSM state transition (INIT/RISING/UP/FALLING/DOWN/DISABLED/EXIT)
3. State change -> determine action (announce with MED / withdraw)
4. Action -> DispatchCommand("watchdog announce hc-dns med 100") or DispatchCommand("watchdog withdraw hc-dns")
5. Watchdog plugin handles command -> modifies pool state -> sends UpdateRoute to engine
6. (If ip-setup) State change -> AddAddress/RemoveAddress on configured interface

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Healthcheck -> Watchdog | DispatchCommand("watchdog announce/withdraw ...") | [ ] |
| Watchdog -> Engine | UpdateRoute(peer, "update text ...") | [ ] |
| Healthcheck -> Iface | iface.AddAddress/RemoveAddress (direct function call, internal mode only) | [ ] |
| Config -> Healthcheck | OnConfigure callback with healthcheck config section | [ ] |

### Integration Points
- `watchdog.handleCommand` - must be extended to parse optional `med <N>` on announce
- `iface.AddAddress` / `iface.RemoveAddress` - called directly for IP management
- Plugin startup coordinator - healthcheck must start after watchdog (Dependencies field)
- YANG schema registry - new ze-healthcheck-conf.yang module

### Architectural Verification
- [ ] No bypassed layers -- healthcheck dispatches to watchdog, watchdog dispatches to engine
- [ ] No unintended coupling -- healthcheck only knows watchdog command syntax, not internals
- [ ] No duplicated functionality -- reuses watchdog route management, iface IP management
- [ ] Zero-copy preserved where applicable -- N/A (config/command path, not wire encoding)

## Design

### YANG Configuration (root level)

```
healthcheck {
    probe <name> {
        command "<shell command>"
        interval 5                     // seconds between checks (0 = single check then exit)
        fast-interval 1                // seconds during state transitions
        timeout 5                      // command timeout in seconds
        rise 3                         // consecutive successes before UP
        fall 3                         // consecutive failures before DOWN
        disable false                  // admin disable (immediate DISABLED state)
        group <watchdog-tag>           // single watchdog group name
        debounce false                 // only dispatch on state change (not every interval)

        up-metric <uint32>             // optional: MED when UP
        down-metric <uint32>           // optional: MED when DOWN (if set, re-announce instead of withdraw)
        disabled-metric <uint32>       // optional: MED when DISABLED (if set, re-announce instead of withdraw)

        ip-setup {
            interface <name>           // target interface (e.g., "lo", "dummy0")
            dynamic false              // true = remove IPs on DOWN/DISABLED, restore on UP
            ip <cidr>                  // leaf-list: VIPs to manage (e.g., "10.0.0.1/32")
        }

        on-up "<command>"              // execute on transition to UP
        on-down "<command>"            // execute on transition to DOWN
        on-disabled "<command>"        // execute on transition to DISABLED
        on-change "<command>"          // execute on any state transition
    }
}
```

### FSM States (8 states, matching ExaBGP)

| State | Description |
|-------|-------------|
| INIT | Initial state at startup |
| RISING | Consecutive successes accumulating (count < rise) |
| UP | Service healthy, routes announced |
| FALLING | Consecutive failures accumulating (count < fall) |
| DOWN | Service unhealthy, routes withdrawn or re-announced with high MED |
| DISABLED | Admin disabled via config boolean |
| EXIT | Shutdown requested, routes withdrawn, IPs removed |
| END | Exit without route changes (single-check mode) |

### FSM Transition Table

| From | Event | To | Condition |
|------|-------|----|-----------|
| INIT | check success | UP | rise <= 1 |
| INIT | check success | RISING | rise > 1, count = 1 |
| INIT | check failure | FALLING | count = 1 |
| RISING | check success | UP | count >= rise |
| RISING | check success | RISING | count < rise, count++ |
| RISING | check failure | FALLING | count = 1 (reset) |
| FALLING | check failure | DOWN | count >= fall |
| FALLING | check failure | FALLING | count < fall, count++ |
| FALLING | check success | RISING | count = 1 (reset) |
| UP | check failure | FALLING | count = 1 |
| DOWN | check success | RISING | count = 1 |
| any | disable = true | DISABLED | immediate |
| DISABLED | disable = false | INIT | re-enter FSM |
| any | shutdown | EXIT | withdraw all, remove IPs |

### State Actions

| State entered | Watchdog action | IP action | Hook |
|--------------|-----------------|-----------|------|
| UP | `watchdog announce <group> [med <up-metric>]` | restore IPs (if dynamic) | on-up, on-change |
| DOWN (down-metric set) | `watchdog announce <group> med <down-metric>` | remove IPs (if dynamic) | on-down, on-change |
| DOWN (down-metric unset) | `watchdog withdraw <group>` | remove IPs (if dynamic) | on-down, on-change |
| DISABLED (disabled-metric set) | `watchdog announce <group> med <disabled-metric>` | remove IPs (if dynamic) | on-disabled, on-change |
| DISABLED (disabled-metric unset) | `watchdog withdraw <group>` | remove IPs (if dynamic) | on-disabled, on-change |
| EXIT | `watchdog withdraw <group>` | remove all IPs | - |
| RISING | - | - | - |
| FALLING | - | - | - |

### Watchdog MED Override Extension

Current watchdog `announce` command: `watchdog announce <name> [peer]`

Extended: `watchdog announce <name> [med <N>] [peer]`

When `med <N>` is present, the watchdog plugin patches the pre-computed AnnounceCmd:
- If AnnounceCmd contains `med set <M>`: replace with `med set <N>`
- If AnnounceCmd has no `med set`: inject `med set <N>` before `nhop`

This is a string operation on the pre-computed command. The pool entry's stored AnnounceCmd is not modified -- the override is applied at dispatch time.

### Probe Execution

- Shell command via `exec.CommandContext` with context deadline (timeout)
- Process group isolation: `Setpgid: true` in SysProcAttr
- On timeout: kill entire process group `syscall.Kill(-pid, syscall.SIGKILL)`
- Exit code 0 = success, non-zero = failure
- Runs in a per-probe goroutine with ticker

### Hook Execution

- Shell command via `exec.Command` (fire-and-forget, no timeout enforcement)
- Environment variable `STATE=<current_state>` set for the command
- Hooks do NOT block the FSM -- failures are logged but do not affect state

### IP Management (internal mode only)

- Uses `iface.AddAddress(ifaceName, cidr)` and `iface.RemoveAddress(ifaceName, cidr)` directly
- Tracks managed IPs internally (set of CIDRs this probe added)
- On restore: only add IPs not already present on interface
- On remove: only remove IPs that this probe added
- External plugin mode: `ip-setup` block rejected at config validation (runtime Go check)

### Internal vs External Plugin Mode

| Aspect | Internal | External |
|--------|----------|----------|
| Launch | Goroutine + net.Pipe | Fork + TLS connect-back |
| SDK | `sdk.NewWithConn` | `sdk.NewFromTLSEnv` |
| IP management | iface.AddAddress/RemoveAddress | Not available |
| Config root | `healthcheck` | `healthcheck` |
| Dependencies | `["bgp-watchdog"]` | `["bgp-watchdog"]` |

### CLI Commands

| Command | Description | Response format |
|---------|-------------|-----------------|
| `show healthcheck` | All probes: name, state, group, checks count, last check time | JSON table |
| `show healthcheck <name>` | Single probe detail: FSM state, consecutive count, thresholds, IPs, metrics | JSON detail |
| `reset healthcheck <name>` | Force immediate re-check, reset FSM to INIT | JSON status |

### Component File Structure

```
internal/component/healthcheck/
    register.go              # init() -> registry.Register(), ConfigRoots: ["healthcheck"]
    healthcheck.go           # Package doc, logger, RunHealthcheckPlugin()
    config.go                # Parse YANG config tree into probe definitions
    fsm.go                   # 8-state FSM with transition logic
    probe.go                 # Shell command execution (process group, timeout)
    announce.go              # Watchdog command dispatch (DispatchCommand)
    ip.go                    # IP management via iface (internal mode only)
    hooks.go                 # on-up/on-down/on-disabled/on-change hook execution
    schema/
        register.go          # yang.RegisterModule("ze-healthcheck-conf.yang", ...)
        embed.go             # //go:embed ze-healthcheck-conf.yang
        ze-healthcheck-conf.yang
    healthcheck_test.go
    fsm_test.go
    probe_test.go
    config_test.go
```

### Watchdog Extension Files

```
internal/component/bgp/plugins/watchdog/
    server.go                # Extend handleCommand to parse "med <N>" on announce
    server_test.go           # Tests for MED override
```

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config `healthcheck { probe dns { ... } }` | -> | config.go parses probe | `test/parse/healthcheck-basic.ci` |
| Probe check success (rise met) | -> | FSM transitions to UP, dispatches `watchdog announce` | `TestFSM_RiseToUp` |
| Probe check failure (fall met) | -> | FSM transitions to DOWN, dispatches `watchdog withdraw` or `announce med` | `TestFSM_FallToDown` |
| `show healthcheck` CLI command | -> | Returns probe status table | `test/plugin/healthcheck-show.ci` |
| `watchdog announce <name> med <N>` | -> | Watchdog patches MED in AnnounceCmd | `TestWatchdogAnnounceMedOverride` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Probe command exits 0, rise consecutive successes met | FSM transitions to UP, `watchdog announce <group>` dispatched |
| AC-2 | Probe command exits non-zero, fall consecutive failures met | FSM transitions to DOWN, `watchdog withdraw <group>` dispatched (no down-metric) |
| AC-3 | Probe command exits non-zero, fall met, down-metric set | FSM transitions to DOWN, `watchdog announce <group> med <down-metric>` dispatched |
| AC-4 | Probe command times out | Treated as failure, process group killed |
| AC-5 | `disable true` in config | Immediate DISABLED state, routes withdrawn or announced with disabled-metric |
| AC-6 | State transitions from DOWN to RISING to UP | Consecutive success count resets on failure, increments on success |
| AC-7 | `debounce true`, state unchanged | No watchdog command dispatched |
| AC-8 | `debounce false`, state unchanged (UP) | `watchdog announce` re-dispatched every interval |
| AC-9 | `ip-setup { dynamic true }`, state UP | IPs present on interface |
| AC-10 | `ip-setup { dynamic true }`, state DOWN | IPs removed from interface |
| AC-11 | `on-up` hook defined, state transitions to UP | Hook command executed with STATE=UP |
| AC-12 | `on-change` hook defined, any state transition | Hook command executed with STATE=<new-state> |
| AC-13 | `interval 0` (single check mode) | One check, announce/withdraw, then exit |
| AC-14 | `watchdog announce <name> med 500` command | Watchdog patches pre-computed AnnounceCmd with `med set 500` |
| AC-15 | `show healthcheck` | Returns JSON with all probe states |
| AC-16 | `reset healthcheck <name>` | Probe FSM reset to INIT, immediate re-check |
| AC-17 | External plugin mode with `ip-setup` block | Config validation error at startup |
| AC-18 | Graceful shutdown (SIGTERM) | All probes transition to EXIT, routes withdrawn, IPs removed |
| AC-19 | `fast-interval` during RISING/FALLING states | Check interval uses fast-interval, not interval |
| AC-20 | Peer reconnect after healthcheck UP | Watchdog resends announced routes (existing watchdog behavior, no healthcheck change) |

## Phased Implementation

This is a large feature. Recommended phase split:

| Phase | Spec | Content |
|-------|------|---------|
| 1 | `spec-healthcheck-1-watchdog-med` | Extend watchdog announce with optional `med <N>` argument |
| 2 | `spec-healthcheck-2-fsm` | FSM implementation + probe execution (no wiring) |
| 3 | `spec-healthcheck-3-component` | Component registration, YANG schema, config parsing, plugin lifecycle |
| 4 | `spec-healthcheck-4-ip-mgmt` | IP management via iface (internal mode) |
| 5 | `spec-healthcheck-5-cli` | CLI commands (show healthcheck, reset healthcheck) |
| 6 | `spec-healthcheck-6-external` | External plugin mode support |

Phase 1 is a prerequisite -- it extends watchdog independently, with its own tests and commit.
Phases 2-3 are the core. Phase 4-6 can be deferred if needed.

## Design Insights

- Healthcheck is operationally simpler in Ze than ExaBGP because route attributes (communities, as-path, etc.) are defined once in BGP config, not per-state in healthcheck config. The MED override via watchdog is the only per-state attribute control needed.
- Single watchdog tag with MED override gives both modes (withdraw and metric) without the complexity of multiple route definitions per probe.
- Labels dropped from ip-setup because Ze's iface uses netlink (not `ip` command), and healthcheck tracks managed IPs internally. ExaBGP needs labels because it discovers existing IPs; Ze explicitly configures them.
- The dependency on watchdog plugin creates a tier ordering constraint. Healthcheck MUST start after watchdog completes all 5 stages.
