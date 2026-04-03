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
7. `internal/component/iface/manage_linux.go` - AddAddress/RemoveAddress (standalone functions, no interface type)
8. `pkg/plugin/sdk/sdk_engine.go` - DispatchCommand, UpdateRoute
9. `.claude/patterns/plugin.md` - plugin structural template
10. `~/Code/github.com/exa-networks/exabgp/src/exabgp/application/healthcheck.py` - reference implementation

## Task

Implement a healthcheck BGP plugin (`bgp-healthcheck`) for Ze with full feature parity with ExaBGP's healthcheck program. Healthcheck monitors service availability by running shell commands periodically and controls BGP route announcement/withdrawal via watchdog groups. Supports both withdraw-on-down (route disappears) and metric-based (MED override) modes through a single watchdog tag.

**Reference implementation:** `~/Code/github.com/exa-networks/exabgp/src/exabgp/application/healthcheck.py`

### ExaBGP Feature Mapping

| ExaBGP feature | Ze mapping |
|---------------|------------|
| `--command` | `command` leaf in YANG |
| `--interval` / `--fast-interval` | `interval` / `fast-interval` leaves |
| `--timeout` | `timeout` leaf |
| `--rise` / `--fall` | `rise` / `fall` leaves |
| `--disable` (file path) | Config change via `ze config set ... bgp healthcheck probe <name> disable true/false`. Plugin detects `disable: true` during config reload and transitions to DISABLED immediately. Differs from ExaBGP's file-poll mechanism but fits Ze's config-driven model. |
| `--ip` (multiple VIPs) | `ip-setup { ip <cidr> }` leaf-list |
| `--ip-setup` / `--dynamic-ip-setup` | `ip-setup { dynamic true }` |
| `--label` / `--label-exact-match` | Dropped -- IPs tracked internally, no label discovery needed |
| `--up-metric` / `--down-metric` / `--disabled-metric` | `up-metric` / `down-metric` / `disabled-metric` (defaults: 100 / 1000 / 500) |
| `--withdraw-on-down` | Explicit boolean leaf (default false). When true, withdraw on DOWN/DISABLED regardless of metric settings. When false (default), re-announce with MED override using down-metric/disabled-metric. Matches ExaBGP default: metric mode, not withdraw mode. |
| `--community` / `--disabled-community` | Route attributes live in BGP config watchdog route definitions. **Per-state community variation dropped** -- ExaBGP allows different communities when UP vs DISABLED; Ze uses one route definition per watchdog group. Users needing per-state communities must define separate watchdog groups. |
| `--as-path` / `--up-as-path` / `--down-as-path` / `--disabled-as-path` | Route attributes live in BGP config watchdog route definitions. **Per-state as-path variation dropped** -- same rationale as communities above. |
| `--extended-community` / `--large-community` | Route attributes live in BGP config watchdog route definitions (single definition, no per-state variation) |
| `--local-preference` | Route attributes live in BGP config watchdog route definitions (BGP config sets local-pref on the route directly) |
| `--path-id` | Route attributes live in BGP config watchdog route definitions (add-path path-id set on the route directly) |
| `--next-hop` | Route attributes live in BGP config watchdog route definitions (next-hop set on the route directly) |
| `--deaggregate-networks` | User defines individual prefixes in BGP config |
| `--start-ip` / `--increase` | User defines per-route attributes in BGP config. **Explicit per-route MED required** -- ExaBGP auto-increments MED across IPs; Ze requires each route's MED defined explicitly in BGP config. |
| `--ip-ifname` | Dropped -- Ze's `ip-setup` uses a single `interface` leaf for all IPs. ExaBGP allows per-IP interface binding via `IP%IFNAME` syntax. Users needing per-IP interfaces must define separate probes. |
| `--neighbor` targeting | Implicit: watchdog groups are per-peer in BGP config. Without a peer argument, watchdog targets all peers (wildcard `*`). |
| `--execute` / `--up-execute` / `--down-execute` / `--disabled-execute` | `on-change` / `on-up` / `on-down` / `on-disabled` leaf-lists (multiple hooks per event, matching ExaBGP `action='append'`). 30s timeout + process group kill per hook. Execution order: list order, state-specific hooks before on-change. |
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
| CLI commands | `show bgp healthcheck`, `show bgp healthcheck <name>`, `reset bgp healthcheck <name>` |
| Exclusive watchdog group | One `group` value maps to exactly one probe. Config validation rejects duplicates |
| Hook timeout | 30-second timeout with process group kill (ExaBGP hooks can hang forever) |
| Config reload | Probes support deconfigure/reconfigure/kill/restart lifecycle |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall component architecture
  -> Constraint: components register at startup via init(), communicate via bus/RPC
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage plugin protocol
  -> Constraint: all plugins follow 5-stage startup, tier ordering via Dependencies
- [ ] `docs/architecture/api/update-syntax.md` - route command syntax
  -> Constraint: text commands use `update text ... med N ... nhop <addr> nlri ...` format. See Design Insights for stale `set` keyword note.
- [ ] `.claude/patterns/plugin.md` - plugin structural template
  -> Constraint: atomic logger, register.go with init(), schema/ subdir for YANG

### RFC Summaries (MUST for protocol work)
N/A -- healthcheck is operational tooling, not a BGP protocol feature.

**Key insights:**
- Healthcheck is a BGP plugin (`bgp-healthcheck`) under `internal/component/bgp/plugins/healthcheck/`
- It dispatches watchdog commands via DispatchCommand through the plugin SDK
- Route attributes (next-hop, community, as-path) are defined in BGP config on watchdog routes
- Healthcheck only controls announce/withdraw lifecycle and optional MED override
- iface.AddAddress/RemoveAddress are standalone package-level functions (no interface type, no receiver) using netlink directly -- direct import is appropriate for two function calls
- Each probe has exclusive ownership of its watchdog group (enforced at config validation)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/watchdog/server.go` - handles `watchdog announce <name>` and `watchdog withdraw <name>` commands. Dispatches to per-peer route pools. Supports `*` wildcard for all peers. Sends pre-computed AnnounceCmd/WithdrawCmd strings.
  -> Constraint: announce command does NOT currently support MED override -- AnnounceCmd is pre-computed at config load time. Must extend to accept optional `med <N>` argument.
- [ ] `internal/component/bgp/plugins/watchdog/pool.go` - PoolEntry stores pre-computed AnnounceCmd and WithdrawCmd strings. Per-peer announced/withdrawn state tracking. Thread-safe via RWMutex.
  -> Decision: PoolEntry extended with Route field. MED override clones Route with new MED and calls FormatAnnounceCommand. Pre-computed AnnounceCmd used when no override.
- [ ] `internal/component/bgp/plugins/watchdog/config.go` - parses BGP config JSON tree, builds Route structs, calls FormatAnnounceCommand/FormatWithdrawCommand to pre-compute commands.
  -> Constraint: no changes needed to config parsing -- watchdog routes are defined as-is.
- [ ] `internal/component/bgp/format.go` - FormatAnnounceCommand builds `update text origin <val> ... med <N> ... nhop <addr> nlri ...` strings. MED is written as `med N` (no `set` keyword) if Route.MED is non-nil.
  -> Decision: MED override clones the Route with the new MED value and calls FormatAnnounceCommand to produce a one-off command string. No string manipulation.
- [ ] `internal/component/iface/manage_linux.go:194-234` - AddAddress(ifaceName, cidr) and RemoveAddress(ifaceName, cidr). Standalone package-level functions (no receiver, no interface type). Use netlink.AddrAdd/AddrDel. Validate interface names (1-15 chars). Accept IPv4/IPv6 CIDR. Non-Linux variant returns "platform not supported" error.
  -> Constraint: no label support in iface. Healthcheck tracks managed IPs internally.
  -> Decision: direct import from plugin to iface package -- these are stateless utility functions, not component lifecycle coupling. Testing: define local interface type in plugin for test injection.
- [ ] `pkg/plugin/sdk/sdk_engine.go:40-55` - DispatchCommand(ctx, command) dispatches through engine's command dispatcher to any plugin. Returns (status, data, error).
  -> Decision: healthcheck uses DispatchCommand for watchdog commands, not UpdateRoute directly.

**Behavior to preserve:**
- Watchdog announce/withdraw semantics unchanged for existing users
- Pre-computed AnnounceCmd/WithdrawCmd still the default (no MED arg = use pre-computed). Route stored in PoolEntry for MED override path only.
- Peer state tracking (announced/withdrawn per peer) unchanged
- Reconnect resend behavior (announced routes resent on peer up) unchanged

**Behavior to change:**
- Watchdog `announce` command extended to accept optional `med <N>` argument (literal `med` keyword required for disambiguation)
- New `bgp { healthcheck {} }` YANG container
- New BGP plugin under `internal/component/bgp/plugins/healthcheck/`

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
| Healthcheck -> Iface | iface.AddAddress/RemoveAddress (direct import, standalone functions, internal mode only) | [ ] |
| Config -> Healthcheck | OnConfigure callback with bgp.healthcheck config section | [ ] |
| Config reload -> Healthcheck | OnConfigure with changed config: deconfigure removed probes, reconfigure changed probes, start new probes | [ ] |
| CLI -> Healthcheck | Command dispatch (show/reset RPC) | [ ] |

### Integration Points
- `watchdog.handleCommand` - must be extended to parse optional `med <N>` on announce
- `iface.AddAddress` / `iface.RemoveAddress` - called directly for IP management
- Plugin startup coordinator - healthcheck must start after watchdog (Dependencies field)
- YANG schema registry - new ze-healthcheck-conf.yang module (augments ze-bgp-conf)

### Architectural Verification
- [ ] No bypassed layers -- healthcheck dispatches to watchdog, watchdog dispatches to engine
- [ ] No unintended coupling -- healthcheck only knows watchdog command syntax, not internals
- [ ] No duplicated functionality -- reuses watchdog route management, iface IP management
- [ ] Zero-copy preserved where applicable -- N/A (config/command path, not wire encoding)

## Design

### YANG Configuration (under bgp)

```
bgp {
  healthcheck {
    probe <name> {
        command "<shell command>"       // MANDATORY
        group <watchdog-tag>            // MANDATORY, unique across all probes (exclusive ownership)
        interval 5                      // default 5 -- seconds between checks (0 = single check, then dormant)
        fast-interval 1                 // default 1 -- seconds during RISING/FALLING states
        timeout 5                       // default 5 -- command timeout in seconds
        rise 3                          // default 3 -- consecutive successes before UP
        fall 3                          // default 3 -- consecutive failures before DOWN
        disable false                   // admin disable via `ze config set ... bgp healthcheck probe <name> disable true` (config reload)
        withdraw-on-down false          // default false -- true = withdraw on DOWN/DISABLED instead of MED override
        debounce false                  // only dispatch on state change (not every interval)

        up-metric 100                   // default 100 -- MED when UP
        down-metric 1000                // default 1000 -- MED when DOWN (used when withdraw-on-down is false)
        disabled-metric 500             // default 500 -- MED when DISABLED (used when withdraw-on-down is false)

        ip-setup {                      // Internal mode only. External mode rejected at Stage 2 configure callback (startup) / config-verify callback (reload).
            interface <name>            // target interface (e.g., "lo", "dummy0")
            dynamic false               // true = remove IPs on DOWN/DISABLED, restore on UP
            ip <cidr>                   // leaf-list: VIPs to manage (e.g., "10.0.0.1/32")
        }

        on-up "<command>"               // leaf-list: execute on transition to UP (30s timeout, process group kill)
        on-down "<command>"             // leaf-list: execute on transition to DOWN
        on-disabled "<command>"         // leaf-list: execute on transition to DISABLED
        on-change "<command>"           // leaf-list: execute on any state transition (runs after state-specific hooks)
    }
  }
}
```

All defaults match ExaBGP. Default behavior is metric mode (MED override), not withdraw mode.
The `group` leaf must be unique across all probes -- YANG `unique` constraint enforces exclusive ownership.
The value `med` is rejected as a group name because it would be ambiguous with the `med <N>` argument in the extended watchdog announce command. Enforced in Go-level config validation (config.go) with a clear error message, not via YANG pattern -- a readable validation error is better than a cryptic regex rejection. Purely numeric group names (e.g., `100`) are allowed -- the watchdog parser uses the literal `med` keyword for disambiguation, so numeric names are unambiguous syntactically. Operators who find them confusing can choose descriptive names.

### FSM States (8 states, matching ExaBGP)

| State | Description |
|-------|-------------|
| INIT | Initial state at startup |
| RISING | Consecutive successes accumulating (count < rise) |
| UP | Service healthy, routes announced |
| FALLING | Consecutive failures accumulating (count < fall) |
| DOWN | Service unhealthy, routes withdrawn or re-announced with high MED |
| DISABLED | Admin disabled via config reload (`disable true`). Check command is NOT executed while DISABLED (matches ExaBGP short-circuit). Probe goroutine stays alive to detect config reload re-enabling. |
| EXIT | Shutdown requested, routes withdrawn, IPs removed |
| END | Single-check complete (interval=0). Routes and IPs left in place. Probe goroutine stops. Watchdog handles resend on peer reconnect. No further checks. No hooks fire (matches ExaBGP: END causes early return before hook dispatch). |

### FSM Transition Table

| From | Event | To | Condition |
|------|-------|----|-----------|
| INIT | check success | UP | rise <= 1 (shortcut) |
| INIT | check success | RISING | rise > 1, count = 1 |
| INIT | check failure | DOWN | fall <= 1 (shortcut) |
| INIT | check failure | FALLING | fall > 1, count = 1 |
| RISING | check success | UP | count >= rise |
| RISING | check success | RISING | count < rise, count++ |
| RISING | check failure | DOWN | fall <= 1 (shortcut) |
| RISING | check failure | FALLING | fall > 1, count = 1 (reset) |
| FALLING | check failure | DOWN | count >= fall |
| FALLING | check failure | FALLING | count < fall, count++ |
| FALLING | check success | UP | rise <= 1 (shortcut) |
| FALLING | check success | RISING | rise > 1, count = 1 (reset) |
| UP | check failure | DOWN | fall <= 1 (shortcut) |
| UP | check failure | FALLING | fall > 1, count = 1 |
| DOWN | check success | UP | rise <= 1 (shortcut) |
| DOWN | check success | RISING | rise > 1, count = 1 |
| any | config reload disable = true | DISABLED | immediate |
| any | initial config with disable = true | DISABLED | probe starts in DISABLED, skips INIT (check `disable` before entering FSM loop) |
| DISABLED | config reload disable = false | INIT | re-enter FSM |
| any | shutdown | EXIT | withdraw all, remove IPs |

**Shortcut note:** When `rise <= 1` or `fall <= 1`, the intermediate state (RISING/FALLING) is skipped entirely via trigger() shortcut, matching ExaBGP behavior. Both `rise=1` and `fall=1` require exactly one check result. The transition table shows final states after shortcut application. Hooks fire for the final state only (e.g., INIT->DOWN when fall<=1 fires on-down, not on a transient FALLING).

**Implementation note:** All transitions go through `trigger()`. The table shows post-shortcut results. Implementation should always call `trigger()` with the intermediate state (RISING/FALLING) and let `trigger()` apply the shortcut -- do not write separate branches per rise/fall threshold in each state handler.

### State Actions

| State entered | Watchdog action | IP action | Hook |
|--------------|-----------------|-----------|------|
| (probe start) | - | add all IPs (if ip-setup configured) | - |
| UP | `watchdog announce <group> med <up-metric>` | restore IPs (if dynamic) | on-up, on-change |
| DOWN (withdraw-on-down false) | `watchdog announce <group> med <down-metric>` | remove IPs (if dynamic) | on-down, on-change |
| DOWN (withdraw-on-down true) | `watchdog withdraw <group>` | remove IPs (if dynamic) | on-down, on-change |
| DISABLED (withdraw-on-down false) | `watchdog announce <group> med <disabled-metric>` | remove IPs (if dynamic) | on-disabled, on-change |
| DISABLED (withdraw-on-down true) | `watchdog withdraw <group>` | remove IPs (if dynamic) | on-disabled, on-change |
| EXIT | `watchdog withdraw <group>` | remove all IPs | - |
| END | (no action -- routes/IPs left in place) | (no action) | - |
| RISING | - | - | on-change (only on actual state change, not count increment) |
| FALLING | - | - | on-change (only on actual state change, not count increment) |

**Hook trigger rule:** Hooks fire on **state changes only**, not on every check tick. RISING->RISING (count increment, same state) does NOT fire on-change. INIT->RISING, UP->FALLING, FALLING->DOWN, etc. DO fire on-change. This matches ExaBGP's trigger() which only dispatches hooks when `target != state`.

**IP startup:** When `ip-setup` is configured, all IPs are added at probe startup (before first check), regardless of `dynamic` setting. This matches ExaBGP which calls `setup_ips()` before the main loop. The `dynamic` flag only controls whether IPs are removed on DOWN/DISABLED and restored on UP. Non-dynamic probes keep IPs present through all states except EXIT.

**Hook order:** State-specific hooks (on-up, on-down, on-disabled) execute first, then on-change. Within each leaf-list, hooks execute in config order.

**Debounce + withdraw-on-down interaction:** Healthcheck always dispatches via the `med <N>` path for announce commands, regardless of `withdraw-on-down` setting. When UP with `withdraw-on-down true`, the announce is still `watchdog announce <group> med <up-metric>`. The `withdraw-on-down` flag only controls whether DOWN/DISABLED states withdraw or re-announce with a different MED. This means `debounce false` works consistently: repeated UP checks re-dispatch `watchdog announce <group> med <up-metric>` every interval because the MED path bypasses the watchdog pool's announced boolean dedup. This is harmless -- identical BGP UPDATEs are no-ops at the peer.

### Watchdog MED Override Extension

Current watchdog `announce` command: `watchdog announce <name> [peer]`

Extended: `watchdog announce <name> [med <N>] [peer]`

The literal keyword `med` is required for disambiguation (without it, a bare number would be ambiguous between MED value and peer name). Parser: after `<name>`, check if next token is `med`; if yes, consume `med <N>`; remaining token (if any) is peer.

When `med <N>` is present, the watchdog plugin produces an overridden announce command:
1. Clone the PoolEntry's stored Route struct
2. Set `clone.MED = &N`
3. Call `FormatAnnounceCommand(clone)` to produce a one-off command string
4. **Bypass the `announced` dedup check** -- always dispatch the one-off command via UpdateRoute
5. Set `announced[peer] = true` (so a subsequent non-MED announce is deduped as before)

The PoolEntry's stored AnnounceCmd and Route are not modified -- the override is transient. When no `med` argument is present, the pre-computed AnnounceCmd is used with normal dedup (zero cost, existing behavior unchanged).

**Why bypass dedup:** The pool tracks `announced` as a boolean (announced/withdrawn), not command content. Without bypass, transitioning from UP (MED 100) to DOWN (MED 1000) would be silently dropped because the route is already "announced." MED override must always dispatch because the whole point is changing the MED on an already-announced route. Same-MED redundancy (e.g., debounce=false re-dispatching MED 100 while UP) is harmless -- BGP UPDATE with identical attributes is a no-op at the peer.

This requires PoolEntry to store the Route alongside the pre-computed commands. The Route struct is small (a few pointers and slices). The clone+reformat only happens on the MED override path.

### Probe Execution

- Shell command via `exec.CommandContext` with context deadline (timeout)
- Process group isolation: `Setpgid: true` in SysProcAttr
- On timeout: kill entire process group `syscall.Kill(-pid, syscall.SIGKILL)`
- Exit code 0 = success, non-zero = failure
- Stdout and stderr captured together. On failure: logged at warning level. On success: logged at debug level. Not exposed via CLI.
- Runs in a per-probe goroutine with ticker

### Hook Execution

- Each hook leaf (on-up, on-down, on-disabled, on-change) is a leaf-list -- multiple hooks per event
- Shell command via `exec.CommandContext` with 30-second timeout
- Process group isolation: `Setpgid: true` in SysProcAttr (same as probe)
- On timeout: kill entire process group, log warning
- Environment variable `STATE=<current_state>` set for the command
- Execution order: state-specific hooks first (in config order), then on-change hooks (in config order)
- Hooks do NOT block the FSM -- run in separate goroutines, failures are logged but do not affect state
- Hook stdout/stderr discarded (same as ExaBGP)

### IP Management (internal mode only)

- Imports `iface.AddAddress(ifaceName, cidr)` and `iface.RemoveAddress(ifaceName, cidr)` directly -- standalone functions, no component lifecycle coupling
- Plugin defines local `IPManager` interface for test injection (two methods: AddAddress, RemoveAddress)
- Tracks managed IPs internally (set of CIDRs this probe added)
- **Startup:** when `ip-setup` is configured, add all IPs at probe startup (before first check), regardless of `dynamic` setting. Matches ExaBGP which calls `setup_ips()` in `main()` before the loop.
- **Dynamic mode (`dynamic true`):** IPs removed on DOWN/DISABLED, restored on UP. Non-dynamic probes keep IPs through all states except EXIT.
- On restore: only add IPs not already present on interface
- On remove: only remove IPs that this probe added
- External plugin mode: `ip-setup` block rejected at Stage 2 configure callback (startup) / config-verify callback (reload) -- checks plugin mode, returns error if external + ip-setup present

### Internal vs External Plugin Mode

| Aspect | Internal | External |
|--------|----------|----------|
| Launch | Goroutine + net.Pipe | Fork + TLS connect-back |
| SDK | `sdk.NewWithConn` | `sdk.NewFromTLSEnv` |
| IP management | iface.AddAddress/RemoveAddress (direct import) | Not available (rejected at Stage 2 configure / config-verify callback) |
| Config root | `bgp.healthcheck` | `bgp.healthcheck` |
| Dependencies | `["bgp-watchdog"]` | `["bgp-watchdog"]` |

### Config Reload Lifecycle

When `OnConfigure` delivers a new healthcheck config:

| Scenario | Action |
|----------|--------|
| Probe removed from config | Deconfigure: transition to EXIT (withdraw routes, remove IPs), stop goroutine |
| Probe config changed | Reconfigure: deconfigure old probe, start new probe from INIT |
| New probe added | Start new probe from INIT |
| Probe `disable` toggled (via config reload) | Immediate DISABLED or re-enter INIT (no deconfigure/reconfigure -- probe keeps running, only state changes) |
| Kill (shutdown signal) | All probes transition to EXIT, withdraw routes, remove IPs |
| Restart | Kill + start all probes from INIT |

Config changes are detected by comparing the new config tree against the running probe set. Changed probes are identified by comparing all config leaves (not just name). Comparison uses Go struct equality on the parsed probe config struct -- leaf-list ordering matters (reordering IPs triggers reconfigure). This is intentional: struct equality is simple and deterministic, and config reordering is rare enough that a spurious reconfigure is acceptable.

### CLI Commands

| Command | Description | Response format |
|---------|-------------|-----------------|
| `show bgp healthcheck` | All probes: name, state, group, checks count, last check time | JSON table |
| `show bgp healthcheck <name>` | Single probe detail: FSM state, consecutive count, thresholds, IPs, metrics | JSON detail |
| `reset bgp healthcheck <name>` | Withdraw current route (if announced), reset FSM to INIT, immediate re-check. Does not override DISABLED -- returns error if probe is DISABLED (use `ze config set ... disable false` instead). | JSON status |

### Plugin File Structure

```
internal/component/bgp/plugins/healthcheck/
    register.go              # init() -> registry.Register(), ConfigRoots: ["bgp"], Dependencies: ["bgp-watchdog"]. config.go extracts healthcheck subtree from full BGP tree.
    healthcheck.go           # Package doc, logger, RunHealthcheckPlugin()
    config.go                # Parse YANG config tree into probe definitions
    fsm.go                   # 8-state FSM with transition logic
    probe.go                 # Shell command execution (process group, timeout)
    announce.go              # Watchdog command dispatch (DispatchCommand)
    ip.go                    # IP management via iface (internal mode only), local IPManager interface
    hooks.go                 # on-up/on-down/on-disabled/on-change hook execution (30s timeout)
    lifecycle.go             # Config reload: deconfigure/reconfigure/kill/restart
    schema/
        register.go          # yang.RegisterModule("ze-healthcheck-conf.yang", ...)
        embed.go             # //go:embed ze-healthcheck-conf.yang
        ze-healthcheck-conf.yang
    healthcheck_test.go
    fsm_test.go
    probe_test.go
    config_test.go
    lifecycle_test.go
```

### Watchdog Extension Files

```
internal/component/bgp/plugins/watchdog/
    server.go                # Extend handleCommand to parse "med <N>" on announce. MED path: clone Route, set MED, FormatAnnounceCommand, bypass announced dedup, dispatch.
    pool.go                  # Add Route field to PoolEntry. MED override path bypasses announced dedup (always dispatches). Non-MED path unchanged.
    config.go                # Store Route in PoolEntry during config parsing
    server_test.go           # Tests for MED override + dedup bypass + dedup preserved for non-MED
```

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config `bgp { healthcheck { probe dns { ... } } }` | -> | config.go parses probe | `test/parse/healthcheck-basic.ci` |
| Probe check success (rise met) | -> | FSM transitions to UP, dispatches `watchdog announce` | `test/plugin/healthcheck-announce.ci` |
| Probe check failure (fall met) | -> | FSM transitions to DOWN, dispatches `watchdog withdraw` or `announce med` | `test/plugin/healthcheck-withdraw.ci` |
| `show bgp healthcheck` CLI command | -> | Returns probe status table | `test/plugin/healthcheck-show.ci` |
| `watchdog announce <name> med <N>` | -> | Watchdog bypasses dedup, dispatches with overridden MED | `test/plugin/watchdog-med-override.ci` |
| `watchdog announce <name> med 100` then `med 1000` | -> | Both dispatch (dedup bypassed for MED path) | `test/plugin/watchdog-med-override.ci` |
| `watchdog announce <name>` (no med, already announced) | -> | Dedup skips (existing behavior preserved) | `test/plugin/watchdog-med-override.ci` |
| Config reload removes probe | -> | Probe deconfigured, routes withdrawn | `test/plugin/healthcheck-deconfigure.ci` |
| Config reload `disable false` (was true) | -> | Probe transitions DISABLED -> INIT | `test/plugin/healthcheck-deconfigure.ci` |
| Duplicate `group` across probes | -> | YANG validation error | `test/parse/healthcheck-duplicate-group.ci` |
| `group` value is literal `med` | -> | Config validation error | `test/parse/healthcheck-duplicate-group.ci` |
| `ip-setup` in external mode | -> | Config validation error | `test/parse/healthcheck-ip-external.ci` |
| `reset bgp healthcheck <name>` while DISABLED | -> | Returns error, stays DISABLED | `test/plugin/healthcheck-show.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Probe command exits 0, rise consecutive successes met | FSM transitions to UP, `watchdog announce <group> med <up-metric>` dispatched |
| AC-2 | Probe command exits non-zero, fall met, `withdraw-on-down true` | FSM transitions to DOWN, `watchdog withdraw <group>` dispatched |
| AC-3 | Probe command exits non-zero, fall met, `withdraw-on-down false` (default) | FSM transitions to DOWN, `watchdog announce <group> med <down-metric>` dispatched |
| AC-4 | Probe command times out | Treated as failure, process group killed |
| AC-5 | Config reload with `disable true`, `withdraw-on-down false` | Immediate DISABLED state, `watchdog announce <group> med <disabled-metric>` |
| AC-6 | Config reload with `disable true`, `withdraw-on-down true` | Immediate DISABLED state, `watchdog withdraw <group>` |
| AC-7 | State transitions from DOWN to RISING to UP | Consecutive success count resets on failure, increments on success |
| AC-8 | `debounce true`, state unchanged | No watchdog command dispatched |
| AC-9 | `debounce false`, state unchanged (UP) | `watchdog announce` re-dispatched every interval |
| AC-10 | `ip-setup { dynamic true }`, state UP | IPs present on interface |
| AC-11 | `ip-setup { dynamic true }`, state DOWN | IPs removed from interface |
| AC-12 | `on-up` hook defined, state transitions to UP | Hook command executed with STATE=UP |
| AC-13 | `on-change` hook defined, any state transition | Hook command executed with STATE=<new-state> |
| AC-14 | `interval 0` (single check mode) | One check, announce/withdraw, transition to END. Routes and IPs left in place. Probe dormant. |
| AC-15 | `watchdog announce <name> med 500` command | Watchdog clones Route, overrides MED to 500, dispatches via FormatAnnounceCommand (bypasses dedup) |
| AC-16 | `show bgp healthcheck` | Returns JSON with all probe states |
| AC-17 | `reset bgp healthcheck <name>` | Current route withdrawn (if announced), FSM reset to INIT, immediate re-check. Returns error if probe is DISABLED. |
| AC-18 | External plugin mode with `ip-setup` block | Go-level configure/config-verify validation error |
| AC-19 | Graceful shutdown (SIGTERM) | All probes transition to EXIT, routes withdrawn, IPs removed |
| AC-20 | `fast-interval` during RISING/FALLING states | Check interval uses fast-interval (default 1s), not interval |
| AC-21 | Peer reconnect after healthcheck UP | Watchdog resends announced routes (existing watchdog behavior, no healthcheck change) |
| AC-22 | Two probes with same `group` value | YANG validation rejects config (unique constraint) |
| AC-23 | Config reload removes a probe | Probe transitions to EXIT, routes withdrawn, IPs removed, goroutine stopped |
| AC-24 | Config reload changes probe `command` | Old probe deconfigured, new probe starts from INIT |
| AC-25 | Hook command hangs >30 seconds | Process group killed, warning logged, FSM not blocked |
| AC-26 | Probe check failure, stdout/stderr non-empty | Combined output logged at warning level |
| AC-27 | Config reload with `disable false` (was `disable true`) | Probe transitions from DISABLED to INIT, resumes checking |
| AC-28 | `ip-setup` configured (non-dynamic), probe starts | All IPs added to interface at probe startup, before first check |
| AC-29 | `ip-setup { dynamic false }`, state DOWN | IPs remain on interface (not removed) |
| AC-30 | Probe in DISABLED state | Check command is NOT executed (probe sleeps on interval timer only) |
| AC-31 | `group` value set to literal `med` | Config validation error with clear message |
| AC-32 | `interval 0`, state transitions to END | No hooks fire (END causes early return before hook dispatch) |
| AC-33 | `ip-setup` with multiple IPs in leaf-list | All IPs added/removed as a group |
| AC-34 | `on-up` has multiple entries (leaf-list) | All hook commands execute in config order on UP transition |
| AC-35 | `watchdog announce X med 100` then `watchdog announce X med 1000` | Both produce UpdateRoute (MED override bypasses dedup) |
| AC-36 | `watchdog announce X` (no med) when already announced | No UpdateRoute (existing dedup preserved) |
| AC-37 | `reset bgp healthcheck <name>` while probe is DISABLED | Returns error, probe stays DISABLED (reset does not override admin disable) |
| AC-38 | `debounce false`, `withdraw-on-down true`, state unchanged (UP) | `watchdog announce <group> med <up-metric>` re-dispatched every interval (MED path bypasses dedup) |

## Phased Implementation

This is a large feature. Every phase produces a wired, testable, user-reachable feature with `.ci` tests. No "library only" phases.

| Phase | Spec | Content | Wired via |
|-------|------|---------|-----------|
| 1 | `spec-healthcheck-1-watchdog-med` | Extend watchdog announce with optional `med <N>` argument | `test/plugin/watchdog-med-override.ci` |
| 2 | `spec-healthcheck-2-core` | Plugin registration + YANG + config + FSM + probe + announce dispatch. Full end-to-end: config -> plugin starts -> probe runs -> watchdog command dispatched. Basic announce (UP) and withdraw (DOWN with withdraw-on-down true). | `test/parse/healthcheck-basic.ci`, `test/plugin/healthcheck-announce.ci`, `test/plugin/healthcheck-withdraw.ci` |
| 3 | `spec-healthcheck-3-modes` | MED mode (withdraw-on-down false, down-metric/disabled-metric), debounce, fast-interval, exclusive group validation, config reload lifecycle, disable toggle. | `test/plugin/healthcheck-med-mode.ci`, `test/plugin/healthcheck-deconfigure.ci` |
| 4 | `spec-healthcheck-4-ip-hooks-cli` | IP management + hooks (with 30s timeout) + CLI commands (show/reset) + single-check mode (interval=0) | `test/plugin/healthcheck-show.ci` |
| 5 | `spec-healthcheck-5-external` | External plugin mode (ip-setup rejection mechanism) | `test/parse/healthcheck-ip-external.ci` |

Phase 1 is a prerequisite -- extends watchdog independently with its own tests and commit.
Phase 2 is the core -- minimal end-to-end path (config -> FSM -> announce/withdraw).
Phase 3 adds MED mode, debounce, fast-interval, group validation, config reload, disable.
Phases 4-5 add IP management, hooks, CLI, and external mode.

**Umbrella scope:** This spec is a design document. It is never implemented directly. The following template sections are intentionally omitted (they live in child specs, Phases 1-5): TDD Test Plan, Boundary Tests, Implementation Steps/Phases, Critical Review Checklist, Deliverables Checklist, Failure Routing, Mistake Log, Implementation Audit, Pre-Commit Verification, Implementation Summary, and Goal/Quality/TDD/Completion Checklists. Files to Modify and Files to Create are represented by the Plugin File Structure and Watchdog Extension Files sections above. The Documentation Update Checklist and Security Review below apply across all child specs.

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add healthcheck section |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- add `bgp { healthcheck {} }` syntax |
| 3 | CLI command added/changed? | Yes (Phase 4) | `docs/guide/command-reference.md` -- add `show bgp healthcheck`, `reset bgp healthcheck` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- add watchdog MED override syntax |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- add bgp-healthcheck plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/healthcheck.md` -- new page, migration guide from ExaBGP |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- healthcheck feature comparison |
| 12 | Internal architecture changed? | No | - |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Shell injection via `command` | Config value passed to `/bin/sh -c <command>`. Admin-controlled (config file or `ze config set`), not user-facing input. Risk: compromised config file = arbitrary code execution. Acceptable: same threat model as ExaBGP. Mitigation: process group isolation + timeout. |
| Shell injection via hooks | `on-up`, `on-down`, `on-disabled`, `on-change` -- same analysis as `command`. All are admin-controlled config values. |
| Resource exhaustion (probe) | Probe goroutine per healthcheck entry. Bounded by config validation (reasonable limit on probe count). Timeout prevents hung processes. |
| Resource exhaustion (hooks) | 30s timeout with process group kill. Hooks do not block FSM. |
| IP management privilege | `iface.AddAddress`/`RemoveAddress` require CAP_NET_ADMIN. Ze already runs with this capability for interface management. No privilege escalation. |
| Config validation bypass | YANG schema validates all leaves. Go-level validation catches `med` as group name and duplicate groups. No path to inject malformed config past both layers. |

## Design Insights

- Healthcheck lives under `bgp {}` in the config tree because it only makes sense in the context of BGP -- it dispatches watchdog commands to BGP peers. Root-level placement would imply standalone capability.
- Disable via config reload (`ze config set ... disable true`) differs from ExaBGP's file-poll mechanism but fits Ze's config-driven model. The probe reacts immediately on receiving the updated config. If subsecond latency matters later, a dedicated RPC following the `ze-set:bgp-peer-with` pattern can be added.
- Healthcheck is operationally simpler in Ze than ExaBGP because route attributes (communities, as-path, next-hop, local-pref, path-id, etc.) are defined once in BGP config, not per-state in healthcheck config. The MED override via watchdog is the only per-state attribute control needed. This is a deliberate simplification: ExaBGP supports per-state communities (`--disabled-community`) and per-state as-paths (`--up-as-path`, `--down-as-path`, `--disabled-as-path`). Ze trades that flexibility for a cleaner model. Users needing per-state route attributes must define separate watchdog groups with distinct route definitions.
- Single watchdog tag with MED override gives both modes (withdraw and metric) without the complexity of multiple route definitions per probe. MED override uses Route clone + FormatAnnounceCommand (no string manipulation).
- Default behavior is metric mode (MED override), matching ExaBGP defaults. Withdraw-on-down requires explicit opt-in via `withdraw-on-down true`.
- Labels dropped from ip-setup because Ze's iface uses netlink (not `ip` command), and healthcheck tracks managed IPs internally. ExaBGP needs labels because it discovers existing IPs; Ze explicitly configures them.
- The dependency on watchdog plugin creates a tier ordering constraint. Healthcheck MUST start after watchdog completes all 5 stages. This also guarantees that watchdog has received its config (including route pool entries) before healthcheck starts dispatching commands. The 5-stage protocol ensures OnConfigure runs before RunEngine, so by the time healthcheck's RunEngine fires its first probe, watchdog's pool is populated.
- iface.AddAddress/RemoveAddress are standalone functions (no receiver, no state, no interface type). Direct import from the plugin is appropriate -- no abstraction needed for two stateless utility calls. Local interface type in plugin enables test injection.
- Exclusive watchdog group ownership prevents conflicting announce/withdraw from multiple probes. Enforced at YANG level via `unique` constraint on the `group` leaf.
- ExaBGP's END state (interval=0) leaves routes and IPs in place -- the process exits but routes persist. Ze replicates this: probe goes dormant, watchdog handles resend on peer reconnect.
- ExaBGP hooks have no timeout (known issue). Ze adds 30-second timeout with process group kill.
- `docs/architecture/api/update-syntax.md` still documents `med set <value>` syntax, but the parser rejects `set` (migration completed, doc is stale). Child specs should not reference that doc for command format -- use `format.go` as source of truth. Same applies to `nhop` -- format.go produces `nhop <addr>`, not `nhop set <addr>`. The full formatted command pattern is: `update text origin <val> med <N> ... nhop <addr> nlri <family> add <prefix>`.
- Debounce + MED override dedup: when `med <N>` is present, the watchdog pool always dispatches (bypasses the `announced` boolean dedup). This is required because the pool tracks announced/withdrawn state, not command content -- without bypass, a MED change (UP 100 -> DOWN 1000) would be silently dropped. Same-MED re-dispatches (debounce=false, stable UP state) produce repeated UpdateRoute calls, but these are harmless: BGP UPDATE with identical attributes is a no-op at the peer. The no-MED path (pre-computed AnnounceCmd) retains the existing dedup. Child specs should verify: (a) two consecutive `watchdog announce X med 100` both produce UpdateRoute, (b) `watchdog announce X med 100` followed by `watchdog announce X med 1000` produces two distinct UpdateRoutes, (c) `watchdog announce X` (no med) when already announced produces zero UpdateRoute calls (dedup preserved).
- Watchdog reconfigure while healthcheck is running: if BGP config changes the routes behind a watchdog group, the watchdog plugin rebuilds its PoolEntry (new Route, new pre-computed AnnounceCmd/WithdrawCmd). The healthcheck probe's next dispatch will use the new pool state automatically -- it sends `watchdog announce <group> med <N>`, and the watchdog re-evaluates against the current PoolEntry. No healthcheck-side awareness of route changes is needed. The pool rebuild handles it.
- Shell commands (`command`, `on-up`, `on-down`, `on-disabled`, `on-change`) execute with ze's privileges. This is by design (same as ExaBGP). Process group isolation + timeout provide containment. See Security Review Checklist above.
- `interval 0` is a magic value meaning "single check, then dormant" (matching ExaBGP). A separate `single-check` boolean was considered but rejected: the ExaBGP convention is well-established, and adding a boolean creates two ways to express the same thing.
- ExaBGP has a bug in per-state as-path: `trigger()` resolves the correct per-state as-path into a local variable `as_path`, but `exabgp()` uses `options.as_path` (the base value) instead of the resolved variable. This silently ignores `--up-as-path`/`--down-as-path`/`--disabled-as-path`. Ze's decision to drop per-state as-path variation is validated by the fact that the ExaBGP implementation is buggy.
- ExaBGP hooks are synchronous (subprocess.call blocks the main loop). Ze hooks are asynchronous (goroutine, don't block FSM). Combined with 30s timeout + process group kill, this prevents a hung hook from stalling healthcheck state transitions.
- `reset bgp healthcheck <name>` withdraws the current route before resetting to INIT. Without this, the route stays announced with a stale MED while the FSM re-evaluates. Reset does not override DISABLED -- admin disable is intentional and should only be lifted by config change. ExaBGP has no equivalent command, so this is Ze-specific. The withdraw-then-INIT sequence means the probe behaves identically to a freshly started probe.
- Config change detection uses Go struct equality on parsed probe config structs. This is simpler than hashing or field-by-field comparison, and means leaf-list reordering (e.g., IPs in different order) triggers a reconfigure. This is acceptable: config reordering is rare, and a spurious deconfigure/reconfigure is safe (just restarts the probe from INIT).
