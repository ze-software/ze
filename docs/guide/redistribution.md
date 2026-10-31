# Route Filters and Redistribution

This page covers two things an operator writes for routes. Route FILTERS decide
what a BGP session accepts and advertises. Route REDISTRIBUTION moves a route
from one protocol into another. They are separate config roots and separate
mechanisms. The filters come first, and
[Route Redistribution](#route-redistribution) is at the end.

## Route Filters

Route filters let plugins act as route filters on import (ingress) and export
(egress). Filters are configured per peer, group, or globally using named
references in a `filter {}` config block. Named filter instances are defined
in the top-level `policy { }` block, which every protocol shares with policy
routing's `policy route`.

<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- filter container -->
<!-- source: internal/component/policy/registry.go -- BuildRegistry, ValidateName -->

### Quick Start

```
policy {
    loop-detection no-self-as {
        allow-own-as 0;
    }
}

bgp {
    router-id 192.0.2.1;
    session {
        asn {
            local 65000;
        }
    }
    filter {
        import {
            filters [ no-self-as rpki:validate ];
            final accept;
        }
    }
    group customers {
        filter {
            import {
                filters [ community:scrub ];
                final accept;
            }
            export {
                filters [ aspath:prepend ];
                final accept;
            }
        }
    }
}
```
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- filter-policy -->

<!-- source: internal/component/bgp/reactor/filter/yang/ze-loop-detection.yang -- loop-detection type -->

### Filter Types

Filter types are YANG lists under the top-level `policy` root, each marked with `ze:filter`.
A list without the marker, such as policy routing's `route`, is not a filter
type, and a chain that names one of its entries is refused.
Plugins add new filter types via YANG `augment "/policy:policy"`. Each list entry is a named
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

1. Filter types are defined in `policy { }` as named instances.
2. Each `filter` direction block lists filter instances under `filters` and states `final accept;` or `final reject;`.
3. Default filters (e.g., loop-detection) auto-populate in every peer's import chain.
4. On each received UPDATE, the engine runs the import filter chain.
5. On each forwarded UPDATE, the engine runs the export chain per destination peer.
6. Each filter responds accept, reject, or modify (delta-only).

<!-- source: internal/component/policy/chain.go -- ExtractChain, ConcatChains -->
<!-- source: internal/component/bgp/config/peers.go -- prependDefaultFilters -->

### Deactivating Filters

Default filters can be deactivated per-peer using the inline `inactive:` prefix:

```
bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }
    peer special {
        filter {
            import {
                filters [ inactive:no-self-as ];
                final accept;
            }
        }
    }
}
```

The `inactive:` prefix is an input shorthand: it is normalized at parse time into
an out-of-band per-member deactivation marker (the stored filter name stays
clean), so on serialize it round-trips to the canonical `inactive: filters
no-self-as;` statement inside the direction block. The deactivated ref stays in the chain but is
skipped at runtime.

In the CLI editor, use `deactivate` and `activate`:

```
deactivate bgp peer special filter import filters no-self-as
activate bgp peer special filter import filters no-self-as
```

<!-- source: internal/component/config/parser_list.go -- stripInactiveMemberPrefix (inline inactive: normalization) -->
<!-- source: internal/component/policy/chain.go -- RefsFromMembers retains Inactive -->
<!-- source: internal/component/bgp/reactor/filter_chain.go -- PolicyFilterChain skips policy.Ref.Inactive -->

### Chain Order

Chains are cumulative across config levels:

| Level | Merge rule |
|-------|-----------|
| Default | Auto-populated first (loop-detection) |
| bgp | Base user chain |
| group | Appended to bgp chain |
| peer | Appended to group chain |

Each configured direction requires its own `final`, even with no active filters.
The most specific configured direction supplies the effective final decision.
An absent direction inherits its parent's policy without creating a new policy.
With no configured policy at any level, operator policy stays absent: EBGP
rejects at exhaustion under RFC 8212 Section 3, while IBGP accepts. Automatic
and inactive references do not supply an operator policy.
<!-- source: internal/component/policy/chain.go -- ExtractChain, ConcatChains -->
<!-- source: internal/component/bgp/filterapi/policy.go -- Policy.Configured, Policy.AcceptsAtEnd -->

Use the `insert` command in the CLI editor to control position:

```
insert filter import filters reject-bogons before no-self-as
insert filter import filters new-filter last
```

### Filter Responses

| Response | Meaning |
|----------|---------|
| accept | Continue to the next filter with the current update |
| reject | Drop the update immediately and stop the chain |
| modify | Apply attribute changes and continue to the next filter |
<!-- source: internal/component/bgp/reactor/filter_chain.go -- PolicyFilterChain -->

Neither `accept` nor `modify` grants final acceptance. After the chain completes,
`final accept;` permits the route and `final reject;` drops it. A raw payload
rewrite stops later filters, but the final decision still applies.
<!-- source: internal/component/bgp/reactor/filter_chain.go -- PolicyFilterChain -->

### Conditioning a modifier without dropping the rest

A chain is a pipe in which a reject DROPS the route, so a match filter placed
before a modifier can only express "modify these and discard everything else".
When the routes the modifier does not touch must keep flowing, put the condition
on the modifier itself with a `match` block. A route that meets none of the
stated values returns accept and passes through unchanged.

```
policy {
    modify blackhole-to-discard {
        match {
            community [ blackhole ];
        }
        set {
            next-hop 192.0.2.1;
        }
    }
}

bgp {
    router-id 192.0.2.254
    session {
        asn {
            local 65000
        }
    }
    peer customer {
        filter {
            import {
                filters [ blackhole-to-discard ];
                final accept;
            }
        }
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
| `accept` | Fail-open: continue to the next filter, then apply the policy's final decision |
<!-- source: internal/component/bgp/reactor/filter_chain.go -- policyFilterFunc, PolicyFilterChain -->

### Writing a Filter Plugin

A filter plugin is a normal ze plugin that includes `filters` in its stage 1
`declare-registration`. See [Plugin Guide](plugins.md) for general plugin
development and `docs/architecture/api/process-protocol.md` for the wire protocol.

## Route Redistribution

Redistribution moves a route a protocol already holds into another protocol.
The `redistribute` root is a separate config root from `bgp { filter }`, and
nothing in this section changes what a BGP session accepts or advertises.

```
policy {
    match redistribute-ospf-bgp {
        family [ ipv6/unicast ];
    }
}
redistribute {
    destination ospf {
        import bgp {
            filters [ redistribute-ospf-bgp ];
            final accept;
        }
    }
    destination isis {
        import connected
        import static
    }
}
```

A `destination <protocol>` block names the protocol that RECEIVES the routes.
Each `import <source>` under it names where they come from. A rule with no
`filters` imports every route the source holds. To narrow it, name filter
instances from the top-level `policy` root in `filters [ ... ]` and end the chain
with `final accept` or `final reject`: a route moves only when every filter
passes it and the final decision accepts it. `filters` without a `final` is
refused. The chain sits at the `redistribute import` attachment, so the editor
completes the instances that fit it and `ze config validate` refuses a name the
`policy` root does not define. The [route policy guide](route-policy.md) lists
the filter types and the fields they read.

A `match` instance narrows by address family (`family [ ... ]`) and by route tag
(`tag [ ... ]`), among its other conditions. The route tag is the value the
source protocol attached to the route, for example the `tag` leaf of a static
route.
<!-- source: internal/component/config/loader_redistribute.go -- importChain -->
<!-- source: internal/plugins/redistribute/config.go -- compileRuleChains -->
<!-- source: internal/plugins/redistribute/register_reference.go -- importFiltersPattern -->
<!-- source: internal/plugins/redistribute/redistribute.go -- redistRoute -->

A file written before the chain, with `family [ ... ]` or `tag N` directly under
`import`, no longer loads: the refusal names the fix,
`ze config migrate -o <new-file> <file>`, which moves the two leaves into a
`match redistribute-<destination>-<source>` instance named in the rule's
`filters` with `final accept`. `tag 0` becomes `tag [ 0 ]` and keeps selecting
the untagged routes. A rule that names neither leaf, a bare `import <source>`
included, is already current and is left as written.
<!-- source: internal/plugins/redistribute/migrate.go -- migrateImportLeaves, importNeedsMigration -->

```
policy {
    match redistribute-ospf-static {
        tag [ 4242 ];
    }
}
redistribute {
    destination ospf {
        import static {
            filters [ redistribute-ospf-static ];
            final accept;
        }
    }
}
```

The value 0 is a tag like any other. A redistributed route with no tag carries
0, so `tag [ 0 ]` selects the untagged routes alone. The filter runs for each route, not for each
batch, so one source can feed two destinations with two different tag sets.

Changing a static route's tag announces the route again with the new tag, and
nothing is withdrawn first. Each destination then treats the new announcement as
a replacement, the way a BGP import policy does. A destination whose rule
accepts the new tag updates the route in place, so it does not flap. A
destination whose rule rejects the new tag removes the route it held. A
destination that never held the route sees nothing.

The rule only decides what may enter a destination, never what must leave it.
When a source withdraws a route a destination holds, the withdrawal reaches
that destination even if its rule would reject the withdrawal itself, for
instance because the rules changed since the route was announced.
<!-- source: internal/plugins/redistribute/held.go -- removeReplacedRoute -->
<!-- source: internal/plugins/static/inject.go -- applyRouteLocked -->

A reload that changes an import's `filters` or `final` judges the routes each
destination already holds again, at once, rather than when each source next
announces them: a route the new chain rejects is withdrawn, a route it now
accepts is dispatched, and a route whose chain now sets a different tag or
metric is sent again with the new value. The orchestrator withdraws first, then
asks every producer to re-send its current set, which the new rules judge (the
replay described under "Startup Order Does Not Decide the Outcome"). When
another plugin refuses the same transaction and it rolls back, the previous
rules return and the same reconciliation puts back what the apply changed. A
route that was being dispatched while the reload applied is judged again under
the new rules before the destination keeps it.
<!-- source: internal/plugins/redistribute/config.go -- reconcileConsumers -->
<!-- source: internal/plugins/redistribute/held.go -- withdrawRejectedHeld, heldRoutes.settleAdd -->
<!-- source: internal/plugins/redistribute/redistribute.go -- replayConsumers, settleDispatchedAdd -->

### Setting the metric

A `modify` instance in the import chain can `set metric`. What the destination
does with that metric depends on the protocol:

| Destination | A `set metric` becomes | With no `set metric` |
|-------------|------------------------|----------------------|
| `bgp` | the route's MULTI_EXIT_DISC | the route is sent with no MULTI_EXIT_DISC |
| `isis` | the TLV 135 or TLV 236 prefix metric | the default prefix metric, 100 |

BGP takes the metric as MED the way FRR does: `bgp_redistribute_add` makes the
redistributed metric the route's MED, and `redistribute <source> metric N`
replaces it. Ze sends a MED only when a filter set one, the way BIRD sends a MED
only when an export filter sets `bgp_med`, because a producer's own metric (an
IGP cost, a static route's zero) is not a figure a BGP neighbour can compare.
<!-- source: internal/component/bgp/redistribute/consumer.go -- formatAnnounce -->
<!-- source: internal/plugins/redistribute/redistribute.go -- dispatchEntryToConsumer, applySets -->

```
policy {
    modify redistribute-bgp-static {
        set { metric 50; }
    }
}
redistribute {
    destination bgp {
        import static {
            filters [ redistribute-bgp-static ];
            final accept;
        }
    }
}
```

### The export chain of an IGP

The import chain decides what may leave the source. IS-IS also judges every
route it receives with its own chain, `isis filter export`, before the route
enters its LSP. A route that chain rejects is not advertised, and a prefix it
advertised before is withdrawn. A `modify` instance's `set metric` there is the
prefix metric, and it replaces any metric the import chain set. The export chain
cannot match on `level`: the level is where IS-IS injects the route, at every
level the router originates, not a property the route arrives with, so a
`match level` there is refused at load. `ze config validate` refuses a name in
`filters` that the `policy` root does not define.

```
policy {
    prefix-list loopbacks {
        entry 0.0.0.0/0 { ge 32; le 32; action accept; }
    }
    modify loopback-metric {
        set { metric 10; }
    }
}
isis {
    filter {
        export {
            filters [ loopbacks loopback-metric ];
            final accept;
        }
    }
}
```

A reload that changes the export chain judges the routes the IGP already holds
again: a route the new chain rejects leaves the LSP or has its AS-external LSA
flushed, a route it now accepts is advertised, and a changed set replaces what
was advertised.
<!-- source: internal/plugins/isis/redistribute/export.go -- exportVerdict -->
<!-- source: internal/plugins/isis/redistribute/consumer.go -- Rejudge -->
<!-- source: internal/plugins/ospf/redistribute/consumer.go -- Rejudge -->
<!-- source: internal/plugins/isis/register.go -- init, MustRegisterAttachment exportAttachment -->

That block is the whole configuration. It needs no `plugin` block and no
`attach process` block. The orchestrator that dispatches the routes auto-loads
because the `redistribute` root is present, and it installs the rules from that
root itself. It needs no `bgp` block either: a router that runs IS-IS or OSPF
without BGP redistributes static, connected and IGP routes between them. When a
rule names BGP, BGP must be configured (below). The two peer bindings the rules
depend on are derived from the rules (below).

<!-- source: internal/component/config/redistribute/yang/ze-redistribute-conf.yang -- redistribute container -->
<!-- source: internal/component/config/loader_redistribute.go -- ExtractRedistributeRules -->
<!-- source: internal/plugins/redistribute/register.go -- StartAfter bgp, no hard dependency -->
<!-- source: internal/plugins/redistribute/config.go -- ruleState -->

### Sources and Destinations

A source is a name a component registers at startup, so which names this build
accepts depends on which components it carries. No command prints the set. A
name this build does not carry is refused at load, and `ze doctor` names the
token first.

| Source | Registered by | What it contributes |
|--------|---------------|---------------------|
| `bgp`, `ibgp`, `ebgp` | the BGP engine | the Loc-RIB's best paths, all sessions or internal or external only. Each best path carries the class of the peer it was learned from, so `ebgp` moves only eBGP-learned paths and `ibgp` only iBGP-learned ones; a locally originated path has no peer class and moves through `bgp` alone |
| `connected` | the connected plugin | prefixes of configured interface addresses, except the unroutable ones below |
| `static` | the static plugin | forward routes in the default table |
| `kernel` | the kernel plugin | routes another program installed in the OS FIB |
| `isis`, `ospf` | the IGP plugins | that protocol's SPF-selected routes |
| `as112`, `l2tp` | those plugins | the covering prefixes and the per-session routes they own |
| `ipsec` | the IKE engine | the remote traffic selector of each established Child SA |

`connected` never offers a prefix whose network address is loopback
(127.0.0.0/8, ::1/128), link-local (169.254.0.0/16, fe80::/10), multicast
(224.0.0.0/4) or in the zero network (0.0.0.0/8). The address `lo` carries
would otherwise reach every OSPF, IS-IS or BGP neighbor, which then routes its
own loopback range through this router. The prefix still enters the Loc-RIB,
because the kernel holds its route, so `show rib` lists it; only the
redistribution feed leaves it out. FRR's zebra refuses the same set on its
redistribute path (`zebra_check_addr`), and BIRD's `direct` protocol skips any
address of host or link scope.
<!-- source: internal/plugins/connected/connected.go -- redistributable, emitID -->

A destination is the name of a protocol that receives routes: `bgp`, `ospf`
and `isis` today. Each protocol declares its destination when its package
loads, with the config block it needs, so the config check knows `destination
isis` needs an `isis { }` block before any plugin starts. The consumer that
receives the routes comes later. OSPF and IS-IS register theirs when their
plugins start. BGP declares a consumer factory, which the orchestrator builds
over its own connection to the engine. A build without a protocol has neither,
so a build without BGP has no `bgp` destination, and a config naming one is
refused at load with the list of destinations the build does declare.

**A protocol is never redistributed into itself.** `destination isis { import
isis }` parses and moves nothing, at every guard: the config evaluator, the
dispatch fan-out and the replay all ask one predicate.

<!-- source: internal/component/config/redistribute/registry.go -- RegisterSource, LookupSource -->
<!-- source: internal/component/config/redistribute/destination.go -- Destination, RegisterDestination -->
<!-- source: internal/component/config/redistribute/consumer_factory.go -- ConsumerFactory -->
<!-- source: internal/component/bgp/redistribute/register.go -- the BGP destination and consumer factory -->
<!-- source: internal/plugins/isis/register.go -- registerISIS, the isis destination -->
<!-- source: internal/plugins/ospf/register.go -- registerOSPF, the ospf destination -->
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
the replay reaches that one peer, and a reload of the import rules, where every
consumer is replayed under the new rules.

<!-- source: internal/plugins/redistribute/replay.go -- onPeerUp, onConsumerRegistered -->
<!-- source: internal/plugins/redistribute/redistribute.go -- watchConsumers -->

### When Redistribution Cannot Work

The daemon REFUSES to start on a `redistribute` block it cannot turn into rules,
and the error names what you typed:

| The config says | What happens |
|-----------------|--------------|
| `import rip`, and no component registers `rip` | the load fails: `redistribute: unknown source "rip" under destination "ospf"` |
| `family [ ipv9/unicast ]` | the load fails naming the family |
| `destination ospf { }` with no import | the load fails: the destination imports nothing |
| `import bgp` or `destination bgp`, and no `bgp { }` block | the load, `ze config validate` and a commit fail: `redistribute: "destination bgp" needs bgp configured, and this config has no bgp { } block`, with the fix on its own line |
| `import isis` or `destination isis`, and no `isis { }` block | the same refusal: `redistribute: "destination isis" needs isis configured, and this config has no isis { } block` |
| `import ospf` or `destination ospf`, and no `ospf { }` block | the same refusal: `redistribute: "destination ospf" needs ospf configured, and this config has no ospf { } block` |

| `destination ospv3 { import connected }`, a name no protocol in this build declares | the load, `ze config validate` and a commit fail: `redistribute: "destination ospv3" names no protocol that receives redistributed routes`, then a line listing the declared destinations (read from the registry, so it is what this build carries) and the fix |

A refusal is deliberate. Ze used to warn once and carry on with redistribution
disabled for the WHOLE file. One mistyped word silently stopped rules that were
written correctly.

`ze doctor` on such a config reports the same refusal, as its config-parse
diagnostic `doctor-config-parse`. Every `ze doctor` entry point (the command,
`show doctor` on a running daemon, and the support bundle) loads the config
through the same door the daemon boots through, so the refusal is the whole
report and no separate redistribution check exists.

```
ze doctor
```

<!-- source: internal/component/config/loader_redistribute.go -- RefuseRedistributeRules, refuseUndeclaredDestination -->
<!-- source: internal/component/config/redistribute/destination.go -- DestinationNames -->
<!-- source: internal/component/doctor/doctor.go -- runChecks -->

### Counters

| Counter | What it counts |
|---------|----------------|
| `ze_bgp_redistribute_events_received` | route-change batches the orchestrator received |
| `ze_bgp_redistribute_announcements` | accepted add entries dispatched to a consumer |
| `ze_bgp_redistribute_withdrawals` | withdrawals dispatched to a consumer: accepted remove entries, and held routes a rejected entry or a reload removes |
| `ze_bgp_redistribute_filtered_protocol_total` | batches skipped by the loop guard |
| `ze_redistribute_chain_rejected_total{destination,source}` | routes an import rule covers whose `filters` chain rejected them, by destination and source protocol. A route no import rule names is not counted |
| `ze_bgp_redistribute_replay_total{source}` | routes replayed, to a new peer, to a consumer that registered late, or to every consumer after a reload changed the import rules |

<!-- source: internal/plugins/redistribute/redistribute.go -- setMetricsRegistry -->
