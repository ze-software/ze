# Healthcheck

Ze's healthcheck plugin monitors service availability by running shell commands and controls BGP route announcement/withdrawal via watchdog groups. It is the successor to ExaBGP's healthcheck program.
<!-- source: internal/component/bgp/plugins/healthcheck/healthcheck.go -- RunHealthcheckPlugin -->

## Quick Start

```
bgp {
    healthcheck {
        probe dns {
            command "dig @127.0.0.1 example.com +short"
            group hc-dns
        }
    }
    peer upstream {
        # ... peer config ...
        update {
            attribute { origin igp; next-hop self; }
            nlri { ipv4/unicast add 10.0.0.1/32; }
            watchdog { name hc-dns; withdraw true; }
        }
    }
}
```

The `bgp-healthcheck` plugin depends on `bgp-watchdog` -- both are loaded automatically when healthcheck config is present.

The probe runs `dig` every 5 seconds (default). After 3 consecutive successes, the route `10.0.0.1/32` is announced. After 3 consecutive failures, the route is withdrawn.

## Configuration Reference

All leaves go under `bgp { healthcheck { probe <name> { ... } } }`.

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `command` | string | (required) | Shell command. Exit 0 = success, non-zero = failure. |
| `group` | string | (required) | Watchdog group name. Must be unique across all probes. |
| `interval` | uint32 | 5 | Seconds between checks. 0 = single check then dormant. |
| `fast-interval` | uint32 | 1 | Seconds between checks during RISING/FALLING states. |
| `timeout` | uint32 | 5 | Command timeout in seconds. Process group killed on timeout. |
| `rise` | uint32 | 3 | Consecutive successes before UP. |
| `fall` | uint32 | 3 | Consecutive failures before DOWN. |
| `withdraw-on-down` | boolean | false | When true, withdraw route on DOWN/DISABLED. When false, re-announce with down-metric/disabled-metric. |
| `disable` | boolean | false | Admin disable. Probe enters DISABLED immediately. |
| `debounce` | boolean | false | When true, only dispatch watchdog commands on state changes. |
| `up-metric` | uint32 | 100 | MED value when UP. |
| `down-metric` | uint32 | 1000 | MED value when DOWN (when withdraw-on-down is false). |
| `disabled-metric` | uint32 | 500 | MED value when DISABLED (when withdraw-on-down is false). |
<!-- source: internal/component/bgp/plugins/healthcheck/schema/ze-healthcheck-conf.yang -- YANG leaves -->

### IP Management (internal mode only)

```
ip-setup {
    interface lo
    dynamic false
    ip 10.0.0.1/32
    ip 10.0.0.2/32
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `interface` | string | - | Target interface for VIPs. |
| `dynamic` | boolean | false | When true, remove IPs on DOWN/DISABLED, restore on UP. |
| `ip` | leaf-list | - | VIP addresses in CIDR notation. |

IPs are added at probe startup (before the first check), regardless of the `dynamic` setting. Non-dynamic probes keep IPs present through all states except EXIT. Not available in external plugin mode.
<!-- source: internal/component/bgp/plugins/healthcheck/ip.go -- ipTracker -->

### Hooks

```
on-up "curl -X POST http://notify/up"
on-down "curl -X POST http://notify/down"
on-disabled "curl -X POST http://notify/disabled"
on-change "logger -t healthcheck state=$STATE"
```

Each hook is a leaf-list (multiple commands per event). Hooks execute asynchronously in goroutines with a 30-second timeout and process group kill. The `STATE` environment variable is set to the current state name (UP, DOWN, DISABLED, etc.). State-specific hooks fire before on-change hooks.
<!-- source: internal/component/bgp/plugins/healthcheck/hooks.go -- runHooks -->

## FSM States

| State | Description |
|-------|-------------|
| INIT | Initial state at startup. |
| RISING | Consecutive successes accumulating (count < rise). |
| UP | Service healthy. Routes announced with up-metric. |
| FALLING | Consecutive failures accumulating (count < fall). |
| DOWN | Service unhealthy. Routes withdrawn or re-announced with down-metric. |
| DISABLED | Admin disabled via config. Check command not executed. |
| EXIT | Shutdown. Routes withdrawn, IPs removed. |
| END | Single-check complete (interval=0). Routes and IPs left in place. |

## Modes

**Withdraw mode** (`withdraw-on-down true`): Routes are announced when UP, withdrawn when DOWN or DISABLED.

**MED mode** (`withdraw-on-down false`, default): Routes are always announced. MED is set to `up-metric` when UP, `down-metric` when DOWN, `disabled-metric` when DISABLED. Upstream routers prefer the path with the lowest MED.

## CLI Commands

| Command | Description |
|---------|-------------|
| `healthcheck show` | JSON summary of all probes. |
| `healthcheck show <name>` | Detailed status of a single probe. |
| `healthcheck reset <name>` | Withdraw current route, reset FSM to INIT, re-check immediately. Returns error if probe is DISABLED. |

## Migration from ExaBGP

Ze's healthcheck replaces ExaBGP's `healthcheck.py` program. Key differences:

- Route attributes (communities, as-path, next-hop) are defined in BGP config, not in healthcheck config.
- MED override via a single watchdog group replaces per-state route definitions.
- Disable via `ze config set ... disable true` replaces file-poll mechanism.
- Hooks have a 30-second timeout (ExaBGP hooks can hang forever).
- Per-state community and as-path variation is not supported. Use separate watchdog groups if needed.
<!-- source: plan/spec-healthcheck-0-umbrella.md -- ExaBGP Feature Mapping -->
