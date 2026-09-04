# Route Filters and Redistribution

This page covers two things an operator writes for routes. Route FILTERS decide
what a BGP session accepts and advertises. Route REDISTRIBUTION moves a route
from one protocol into another. They are separate config roots and separate
mechanisms. The filters come first, and
[Route Redistribution](#route-redistribution) is at the end.

## Route Filters

Route filters let plugins act as route filters on import (ingress) and export
(egress). Filters are configured per peer, group, or globally using named
references in a `filter {}` config block. Named filter types are defined under
`bgp { policy { } }`.

<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- filter container -->

### Quick Start

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

<!-- source: internal/component/bgp/reactor/filter/yang/ze-loop-detection.yang -- loop-detection type -->

### Filter Types

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

### How It Works

1. Filter types are defined in `bgp { policy { } }` as named instances.
2. Per-peer `filter { import/export [ names ] }` references filter instances.
3. Default filters (e.g., loop-detection) auto-populate in every peer's import chain.
4. On each received UPDATE, the engine runs the import filter chain.
5. On each forwarded UPDATE, the engine runs the export chain per destination peer.
6. Each filter responds accept, reject, or modify (delta-only).

<!-- source: internal/component/bgp/config/peers.go -- prependDefaultFilters, extractFilterChain -->

### Deactivating Filters

Default filters can be deactivated per-peer using the inline `inactive:` prefix:

```
bgp {
    peer special {
        filter {
            import [ inactive:no-self-as ];
        }
    }
}
```

The `inactive:` prefix is an input shorthand: it is normalized at parse time into
an out-of-band per-member deactivation marker (the stored filter name stays
clean), so on serialize it round-trips to the canonical `inactive: import
no-self-as` statement form. The deactivated ref stays in the chain but is
skipped at runtime.

In the CLI editor, use `deactivate` and `activate`:

```
deactivate bgp peer special filter import no-self-as
activate bgp peer special filter import no-self-as
```

<!-- source: internal/component/config/parser_list.go -- stripInactiveMemberPrefix (inline inactive: normalization) -->
<!-- source: internal/component/bgp/config/redistribution.go -- extractFilterChain builds []FilterRef with Inactive -->
<!-- source: internal/component/bgp/reactor/filter_chain.go -- PolicyFilterChain skips FilterRef.Inactive -->

### Chain Order

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

### Filter Responses

| Response | Meaning |
|----------|---------|
| accept | Pass update through unchanged |
| reject | Drop update, short-circuit chain |
| modify | Change specific attributes (delta-only) |

### Conditioning a modifier without dropping the rest

A chain is a pipe in which a reject DROPS the route, so a match filter placed
before a modifier can only express "modify these and discard everything else".
When the routes the modifier does not touch must keep flowing, put the condition
on the modifier itself with a `match` block. A route that meets none of the
stated values returns accept and passes through unchanged.

```
bgp {
    policy {
        modify blackhole-to-discard {
            match { community [ blackhole ]; }
            set   { next-hop 192.0.2.1; }
        }
    }
    peer customer {
        filter { import [ blackhole-to-discard ]; }
    }
}
```

A definition with no `match` block applies to every route that reaches it. See
`internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang` for the
full leaf reference.
<!-- source: internal/component/bgp/plugins/filter_modify/match.go -- matchCond -->
<!-- source: internal/component/bgp/plugins/filter_modify/filter_modify.go -- handleFilterUpdate -->
<!-- source: internal/component/bgp/filtertext/community.go -- HasCommunity -->

### Filters Cannot Override the RFC 4271 Egress Rules

Four Section 5 rules are asked after the export chain has run, so no filter can
grant what the RFC refuses: LOCAL_PREF is removed toward an external peer
(5.1.5), a relayed MULTI_EXIT_DISC is removed toward another neighboring AS
(5.1.4), a route whose final next hop is the destination peer's own address is
withheld (5.1.3), and an UPDATE advertising no reachable NLRI keeps no attribute
a filter would have created on it (4.3 and 6.3). See
[BGP protocol features](../features/bgp-protocol.md) for the full table.

A modification that cannot be applied suppresses the route for that destination
rather than forwarding it unmodified, and increments
`ze_bgp_update_modify_failed_total{reason}`.

### Failure Handling

Each filter declares its own failure mode at startup:

| Mode | Behavior on IPC error/timeout |
|------|-------------------------------|
| `reject` | Fail-closed: drop the update |
| `accept` | Fail-open: pass the update through |

### Writing a Filter Plugin

A filter plugin is a normal ze plugin that includes `filters` in its stage 1
`declare-registration`. See [Plugin Guide](plugins.md) for general plugin
development and `docs/architecture/api/process-protocol.md` for the wire protocol.

## Route Redistribution

Redistribution moves a route a protocol already holds into another protocol.
The `redistribute` root is a separate config root from `bgp { filter }`, and
nothing in this section changes what a BGP session accepts or advertises.

```
redistribute {
    destination ospf {
        import bgp {
            family [ ipv6/unicast ];
        }
    }
    destination isis {
        import connected
        import static
    }
}
```

A `destination <protocol>` block names the protocol that RECEIVES the routes.
Each `import <source>` under it names where they come from. The optional
`family` leaf-list narrows which address families that source contributes. An
empty list imports every family.

That block is the whole configuration. It needs no `plugin` block and no
`attach process` block. The orchestrator that dispatches the routes auto-loads
because the `redistribute` root is present. The two peer bindings the rules
depend on are derived from the rules (below).

<!-- source: internal/component/config/redistribute/yang/ze-redistribute-conf.yang -- redistribute container -->
<!-- source: internal/component/config/loader_redistribute.go -- ExtractRedistributeRules -->

### Sources and Destinations

A source is a name a component registers at startup, so which names this build
accepts depends on which components it carries. No command prints the set. A
name this build does not carry is refused at load, and `ze doctor` names the
token first.

| Source | Registered by | What it contributes |
|--------|---------------|---------------------|
| `bgp`, `ibgp`, `ebgp` | the BGP engine | the Loc-RIB's best paths, all sessions or internal or external only |
| `connected` | the interface component | prefixes of configured interface addresses |
| `static` | the static plugin | forward routes in the default table |
| `kernel` | the kernel plugin | routes another program installed in the OS FIB |
| `isis`, `ospf` | the IGP plugins | that protocol's SPF-selected routes |
| `as112`, `l2tp` | those plugins | the covering prefixes and the per-session routes they own |
| `ipsec` | the IKE engine | the remote traffic selector of each established Child SA |

A destination is the name of a protocol whose consumer registered: `bgp`, `ospf`
and `isis` today.

**A protocol is never redistributed into itself.** `destination isis { import
isis }` parses and moves nothing, at every guard: the config evaluator, the
dispatch fan-out and the replay all ask one predicate.

<!-- source: internal/component/config/redistribute/registry.go -- RegisterSource, LookupSource -->
<!-- source: internal/core/redistevents/registry.go -- WouldLoop, the one loop definition -->

### What the Rules Wire For You

A `redistribute` rule needs no `attach process` block. Ze derives the two it
implies, and each one only when a rule implies it.

| You write | Ze grants every peer | Because |
|-----------|----------------------|---------|
| `import bgp` (or `ibgp`, `ebgp`) | `receive [ update state refresh ]` toward `bgp-rib` | the source is the daemon's own Loc-RIB, and a plugin sees a peer's UPDATEs only where that peer grants them |
| `destination bgp` | `receive [ state ]` and `send [ update ]` toward `redistribute-orchestrator` | that plugin puts the route on the peer's wire, and `send` is the permission to do so |

Two things follow, and both are deliberate:

- A peer that already names one of those processes keeps its own binding,
  exactly as written. A narrower grant you typed stays narrow, and you never get
  a second binding for one process.
- A rule implies only its own binding. A router that only imports IS-IS into
  OSPF gains neither.

<!-- source: internal/component/bgp/config/redistribute_binding.go -- wireRedistributeDelivery, processNameFor -->
<!-- source: internal/component/bgp/reactor/send_permission.go -- Peer.maySend -->

### Startup Order Does Not Decide the Outcome

A producer emits its routes once, when it has them. A destination protocol
registers its consumer when its plugin starts. Nothing orders the two, so a
consumer can register after the routes were already sent.

When that happens the consumer asks for them again. Becoming registered fires a
replay request, every producer re-emits its CURRENT set, and the answer is
injected through that consumer alone.

A replay reflects what the producer holds NOW rather than what it once sent. A
route withdrawn before the consumer registered is therefore absent.

The same mechanism covers a BGP peer that establishes after an injection, where
the replay reaches that one peer.

<!-- source: internal/component/bgp/plugins/redistribute_egress/replay.go -- onPeerUp, onConsumerRegistered -->
<!-- source: internal/component/bgp/plugins/redistribute_egress/redistribute.go -- watchConsumers -->

### When Redistribution Cannot Work

The daemon REFUSES to start on a `redistribute` block it cannot turn into rules,
and the error names what you typed:

| The config says | What happens |
|-----------------|--------------|
| `import rip`, and no component registers `rip` | the load fails: `redistribute: unknown source "rip" under destination "ospf"` |
| `family [ ipv9/unicast ]` | the load fails naming the family |
| `destination ospf { }` with no import | the load fails: the destination imports nothing |

A refusal is deliberate. Ze used to warn once and carry on with redistribution
disabled for the WHOLE file. One mistyped word silently stopped rules that were
written correctly.

One fault is not refusable, because no code can judge it at load time. It is a
`destination` naming a protocol whose consumer never registers. `ze doctor`
reports it, under `doctor-redistribute-unknown-destination`, together with an
unknown source under `doctor-redistribute-unknown-source`.

```
ze doctor
```

<!-- source: internal/component/doctor/checks_redistribute.go -- checkRedistributeRules -->
<!-- source: internal/core/diagnostic/codes.go -- doctor-redistribute-unknown-source, doctor-redistribute-unknown-destination -->

### Counters

| Counter | What it counts |
|---------|----------------|
| `ze_bgp_redistribute_events_received` | route-change batches the orchestrator received |
| `ze_bgp_redistribute_announcements` | accepted add entries dispatched to a consumer |
| `ze_bgp_redistribute_withdrawals` | accepted remove entries dispatched to a consumer |
| `ze_bgp_redistribute_filtered_protocol_total` | batches skipped by the loop guard |
| `ze_bgp_redistribute_filtered_rule_total` | entries the evaluator rejected |
| `ze_bgp_redistribute_replay_total{source}` | routes replayed, to a new peer or to a consumer that registered late |

<!-- source: internal/component/bgp/plugins/redistribute_egress/redistribute.go -- setMetricsRegistry -->
