# Redistribution Filters

> **Status:** Planned. See `plan/spec-redistribution-filter.md` for the design spec.

Redistribution filters let external plugins act as route filters on import (ingress)
and export (egress). Filters are configured per peer, group, or globally using
`<plugin>:<filter>` references in a `redistribution {}` config block.

## Quick Start

```
bgp {
    redistribution {
        import [ rpki:validate ]
    }
    group customers {
        redistribution {
            import [ community:scrub ]
            export [ aspath:prepend ]
        }
    }
}
```

<!-- source: plan/spec-redistribution-filter.md -- config design -->

## How It Works

1. A plugin declares named filters at startup (stage 1), specifying direction,
   requested attributes, and failure mode.
2. The user references filters in config as `<plugin>:<filter>`.
3. On each received UPDATE, the engine runs the import filter chain. On each
   forwarded UPDATE, the engine runs the export filter chain per destination peer.
4. Each filter receives only its declared attributes as text and responds
   accept, reject, or modify (delta-only).
5. Filters chain as piped transforms: each sees the previous filter's output.
   Reject short-circuits the chain.

## Filter Categories

| Category | Behavior | Config visible | Example |
|----------|----------|---------------|---------|
| Mandatory | Always on, cannot be overridden | No | `rfc:otc` (RFC 9234 role filtering) |
| Default | On by default, can be overridden | No (override via user filter) | `rfc:no-self-as` (loop prevention) |
| User | Only when explicitly configured | Yes | `rpki:validate`, `community:scrub` |

Mandatory and default filters are invisible in config. A future extensive/verbose
view may display them for debugging.

## Config Hierarchy

Chains are cumulative across config levels:

| Level | Merge rule |
|-------|-----------|
| Mandatory | Always first, implicit |
| Default | After mandatory, implicit |
| bgp | Base user chain |
| group | Appended to bgp chain |
| peer | Appended to group chain |

Example: bgp declares `rpki:validate`, group declares `community:scrub`, peer
declares `aspath:prepend`. Effective import chain for that peer:
[rfc:otc] -> [rfc:no-self-as] -> rpki:validate -> community:scrub -> aspath:prepend.

## Overriding Default Filters

A filter can declare that it overrides a default filter. When configured on a
peer (or group, or globally), the overridden default is removed from that peer's chain.

```
peer special {
    remote { ip 10.0.0.2; as 65002; }
    redistribution {
        import [ allow-own-as:relaxed ]
    }
}
```

If `allow-own-as:relaxed` declares `overrides: ["rfc:no-self-as"]`, then
`rfc:no-self-as` is removed for this peer. Mandatory filters (like `rfc:otc`)
cannot be overridden.

## Filter Responses

| Response | Meaning |
|----------|---------|
| accept | Pass update through unchanged |
| reject | Drop update, short-circuit chain |
| modify | Change specific attributes (delta-only) |

On modify, the filter returns only changed fields. The engine uses dirty tracking
to re-encode only modified attributes into the wire bytes.

## Failure Handling

Each filter declares its own failure mode at stage 1:

| Mode | Behavior on IPC error/timeout |
|------|-------------------------------|
| `reject` | Fail-closed: drop the update |
| `accept` | Fail-open: pass the update through |

The plugin author chooses based on the security semantics of their filter.

## Writing a Filter Plugin

A filter plugin is a normal ze plugin that includes `filters` in its stage 1
`declare-registration`. See [Plugin Guide](plugins.md) for general plugin
development and `docs/architecture/api/process-protocol.md` for the wire protocol.

<!-- source: plan/spec-redistribution-filter.md -- redistribution filter design -->
