# Route Filters

Route filters let plugins act as route filters on import (ingress) and export
(egress). Filters are configured per peer, group, or globally using named
references in a `filter {}` config block. Named filter types are defined under
`bgp { policy { } }`.

<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- filter container -->

## Quick Start

```
bgp {
    policy {
        loop-detection no-self-as {
            allow-own-as 0;
        }
    }
    filter {
        import [ no-self-as rpki:validate ];
    }
    group customers {
        filter {
            import [ community:scrub ];
            export [ aspath:prepend ];
        }
    }
}
```

<!-- source: internal/component/bgp/reactor/filter/schema/ze-loop-detection.yang -- loop-detection type -->

## Filter Types

Filter types are YANG lists under `bgp/policy`, each marked with `ze:filter`.
Plugins add new filter types via YANG `augment`. Each list entry is a named
filter instance referenced by name in peer `filter { import/export }` chains.

### Built-in: loop-detection

Facade over the in-process `LoopIngress` wire-bytes filter. Configures AS loop
detection (RFC 4271 Section 9) and cluster-list loop detection (RFC 4456 Section 8).

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| allow-own-as | uint8 (0-10) | 0 | Own-AS occurrences to tolerate before rejecting |
| cluster-id | ipv4-address | (router-id) | Override Router ID for CLUSTER_LIST loop check |

<!-- source: internal/component/bgp/reactor/filter/loop.go -- LoopIngress -->

### External plugin filters

External plugins declare filters at startup using `<plugin>:<filter>` names.
Example: `rpki:validate`, `community:scrub`.

## How It Works

1. Filter types are defined in `bgp { policy { } }` as named instances.
2. Per-peer `filter { import/export [ names ] }` references filter instances.
3. Default filters (e.g., loop-detection) auto-populate in every peer's import chain.
4. On each received UPDATE, the engine runs the import filter chain.
5. On each forwarded UPDATE, the engine runs the export chain per destination peer.
6. Each filter responds accept, reject, or modify (delta-only).

<!-- source: internal/component/bgp/config/peers.go -- prependDefaultFilters, extractFilterChain -->

## Deactivating Filters

Default filters can be deactivated per-peer using the `inactive:` prefix:

```
bgp {
    peer special {
        filter {
            import [ inactive:no-self-as ];
        }
    }
}
```

In the CLI editor, use `deactivate` and `activate`:

```
deactivate bgp peer special filter import no-self-as
activate bgp peer special filter import no-self-as
```

<!-- source: internal/component/bgp/reactor/filter_chain.go -- inactive: prefix skipping -->

## Chain Order

Chains are cumulative across config levels:

| Level | Merge rule |
|-------|-----------|
| Default | Auto-populated first (loop-detection) |
| bgp | Base user chain |
| group | Appended to bgp chain |
| peer | Appended to group chain |

Use the `insert` command in the CLI editor to control position:

```
insert filter import reject-bogons before no-self-as
insert filter import new-filter last
```

## Filter Responses

| Response | Meaning |
|----------|---------|
| accept | Pass update through unchanged |
| reject | Drop update, short-circuit chain |
| modify | Change specific attributes (delta-only) |

## Failure Handling

Each filter declares its own failure mode at startup:

| Mode | Behavior on IPC error/timeout |
|------|-------------------------------|
| `reject` | Fail-closed: drop the update |
| `accept` | Fail-open: pass the update through |

## Writing a Filter Plugin

A filter plugin is a normal ze plugin that includes `filters` in its stage 1
`declare-registration`. See [Plugin Guide](plugins.md) for general plugin
development and `docs/architecture/api/process-protocol.md` for the wire protocol.
