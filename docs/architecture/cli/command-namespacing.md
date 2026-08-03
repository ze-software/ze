# CLI Command Namespacing

How operational and config commands are rooted (`show ip route` vs `show router
route-table` vs `show route`), why vendors diverge, where Ze sits, and the rule
that filters are keyword grammar in YANG, never `--flags`.

## The question

A Layer-3 lookup such as "show the routing table" has to be reached through some
keyword path. Vendors disagree on what the *first* keyword after the verb should
encode:

- the **address family** (`show ip route`, `show ipv6 route`)
- the **routing instance** (`show router route-table`)
- the **object** being inspected (`show route`)

The choice is not cosmetic. It decides whether the command tree is duplicated per
family, scoped per VRF, or flat, and it decides what becomes a *filter* versus
what becomes *structure*.

## Three philosophies

| Root encodes | Vendors | Family is | Instance is | Cost |
|---|---|---|---|---|
| Address family (`ip` / `ipv6`) | Cisco IOS/IOS-XE, Arista EOS, FRR | the namespace root | a filter (`vrf X`) | every L3 command duplicated across `ip` and `ipv6` |
| Routing instance (`router`) | Nokia SR OS | a filter (`ipv6`) | the namespace root | verbose for the single-instance case |
| Object / function | Juniper JunOS, Linux iproute2 | a filter / table name | a filter / qualifier | grouping is implicit, less "discoverable by family" |

### Cisco: address-family-rooted (`ip` / `ipv6`)

`show ip route`, `show ip arp`, `show ip ospf`, `show ip bgp`; parallel `show
ipv6 route`, `show ipv6 neighbors`, `show ipv6 ospf`. Config mirrors it: `ip
route`, `ip name-server`, `ip access-list`, `ipv6 route`.

The logic is historical. IOS grew up when IPv4 was *one Layer-3 protocol among
several competing ones*: IPX (`ipx route`), AppleTalk (`appletalk`), CLNS
(`clns`). The `ip` keyword delimited the IPv4 protocol stack from those peers.
When IPv6 arrived it was modelled the same way, as another protocol, so it got
its own parallel `ipv6` root. VRF support was added later and bolted on as a
filter: `show ip route vrf RED`.

Consequence: the namespace encodes *address family*; routing instance is
secondary; and every L3 command exists twice (`ip` and `ipv6`).

### Nokia SR OS: routing-instance-rooted (`router`)

SR OS is a service-provider OS designed so one node hosts **many independent
routing contexts at once**. The namespace is ordered by what disambiguates a
lookup, and on a provider edge box that is "which context?", not "which family?".

The grammar is `show router [<instance>] <object> [<family/filter>]`:

```
show router route-table          # defaults to the Base instance
show router "Base" route-table   # explicit Base
show router 100 route-table      # the VPRN with service-id 100
show router "management" interface
show router route-table ipv6     # family is a trailing qualifier
```

The contexts are the **Base** instance (the provider's own infrastructure: IGP,
LDP/RSVP, the global BGP table; literally named `"Base"`), the **management**
instance (out-of-band port), and one **VPRN** instance per L3VPN, each with its
own RIB and FIB. `<object>` is `route-table`, `arp`, `neighbor`, `bgp`, `ospf`,
`isis`, `fib`, `interface`, `tunnel-table`, `static-route`, ... A parallel `show
service ...` view exists for the L2/service angle; a VPRN appears in both because
it is a *service* that contains a *router* instance.

The logic is overlapping address space. With thousands of customer VPRNs,
`10.0.0.0/8` can exist in customer A and customer B simultaneously, so "show me
route 10.0.0.0/8" is meaningless until the instance is named. The instance is the
**mandatory disambiguator**, so it goes first; family is secondary because every
instance has the same families. This is the exact inversion of Cisco's
family-first reasoning (one global table, several competing L3 protocols), which
is why Cisco appends VRF as a late filter while Nokia elevates it to the root.
Nokia still splits `arp` (IPv4) from `neighbor` (IPv6 ND) by name, because ARP is
inherently IPv4; family-as-filter does not erase that.

This maps cleanly onto a tree. Modern SR OS uses **MD-CLI**, a YANG-modeled CLI
where `configure router "Base" ...` and `configure service vprn "X" ...` are
sibling branches: the instance is a literal tree node, the object hangs under it,
the family is a leaf/filter. Same tree-plus-filter philosophy as Ze, except SR OS
roots the tree at the *instance* and Ze roots it at the *object* (Ze has one
instance).

Consequence: one command grammar, reused per instance; family is a parameter;
scales to thousands of VRFs without duplicating the tree; overlapping address
space is always unambiguous. The price is verbosity when there is only one router
(you carry the `router` word even with a single context), and `router` does
double duty as both the instance-introducing keyword and the path to Base.

### Juniper / Linux: object-rooted

JunOS: `show route`, `show arp`, `show ospf neighbor`, `show bgp summary`. Both
family and instance are qualifiers: `show route table inet.0` (IPv4 unicast),
`inet6.0` (IPv6), `<instance>.inet.0`. The root encodes the *thing inspected*,
not a protocol or a context.

Linux iproute2: `ip route`, `ip neigh`, `ip addr`, `ip link`. Here `ip` is the
*binary name*, the object (`route`, `neigh`) is the subcommand, and family is a
global flag (`ip -6 route`) or filter (`ip route show table main`). One unified
neighbour table covers both ARP and IPv6 ND (`ip neigh`).

## Where Ze sits

Ze is **object-rooted with family-as-filter** (the Juniper / Linux camp), not the
Cisco address-family-rooted camp. Every operational command roots at the **object**
it inspects, and the object *is* the owning component or plugin: `route` and
`neighbor` belong to the iface component, `ospf` to the ospf plugin, `bgp` to the
BGP engine, and so on. This is the same registration model the rest of the system
uses, so the command tree mirrors the plugin tree. There is no shared `ip`
container and no parallel `show ipv6 ...` tree.

Address family is a positional keyword filter on the object, never a namespace:

| Command | Meaning |
|---|---|
| `show route` / `show route <cidr>` / `show route default` | kernel routing table (iface) |
| `show route lookup <ip>` | longest-prefix-match lookup |
| `show neighbor` / `show neighbor ipv4` / `show neighbor ipv6` | ARP + ND table; family filter |
| `show arp` | IPv4-only shortcut for `show neighbor ipv4` |
| `show ospf ...` / `show ospf ipv6 ...` / `clear ospf ...` | OSPF (one unified engine); bare = OSPFv2/IPv4, `ipv6` selector = OSPFv3 |

Three points this settles:

1. **No `show ip` / `show ipv6` split.** That is the Cisco model Ze deliberately
   did not adopt. One tree, one `family` filter.
2. **`arp` is IPv4-only, by design.** ARP is IPv4 by definition (RFC 826); IPv6
   uses Neighbor Discovery. So `show neighbor` is the honest name for the unified
   both-families table (the Linux `ip neigh` model), and `show arp` is kept as the
   familiar IPv4-only shortcut (`= show neighbor ipv4`).
3. **OSPFv2 and OSPFv3 share one `ospf` object.** Ze runs a single unified OSPF
   engine (IPv4 + IPv6 via an address-family strategy), so the family is a
   selector on `ospf`, not a separate object: bare `show ospf <noun>` is IPv4
   (OSPFv2), `show ospf ipv6 <noun>` is IPv6 (OSPFv3), `show ospf ipv4 <noun>` is
   explicit IPv4. This rejects both Cisco's `show ipv6 ospf` family-prefix and the
   Juniper/Nokia `ospf3` second object: one engine, one object, family as filter.
   (Only OSPFv2 ships today; the `ipv6` selector lands with OSPFv3 show.)

This replaced an earlier transitional layout that grouped a few reads under a
shared `show ip` container (`show ip route`, `show ip arp`, `show ip ospf`). That
container conflated "address family" with "object group" and was the one place the
command tree broke plugin self-containment (iface and ospf both reached into a
shared `ip` parent). Object-rooting removes the shared parent: each plugin owns its
root outright.

### Why the shipped CLI isn't `router`-rooted, and what VRF changes

Because Ze is single-instance today. An instance-rooted namespace (`show router
...`) only pays off once there are multiple routing instances / VRFs to
disambiguate. Adopting it now would be verbosity with no instance to select.

Forward-looking: VRF / routing-instance support is designed in
[`plan/spec-vrf-0-umbrella.md`](../../../plan/spec-vrf-0-umbrella.md) (agreed
with user), and it **does adopt the instance-first form**: `show vrf <name>
<object>` (e.g. `show vrf surfprotect route`), with the default VRF keeping the
bare, unwrapped command (`show route`). This is deliberately the Nokia-shaped
instance-first ordering rather than a trailing `show route vrf <name>` filter,
because the VRF design makes each instance a *full replicated stack* (its own
reactor, RIB, hub, and TCP listeners), which is exactly the multi-context
situation that elevates the instance to the root. A hub-of-hubs orchestrator
intercepts the leading `vrf <name>` token and forwards the remainder verbatim to
that instance, so the instance must be a prefix, not a trailing qualifier: a
trailing filter would force either the VRF layer to parse every object's grammar
or every object handler to reach across instances. The YANG of each child module
is wrapped in `vrf <name> { ... }`, so the config tree is instance-rooted and the
operational tree matches it (the MD-CLI symmetry above). Two things stay put:
the keyword is `vrf`, not Nokia's `router`, and **family remains a trailing
filter** (`show vrf red ospf ipv6`). So Ze lands on instance-first prefix ->
object -> family-as-filter, and the default VRF keeps the bare object-rooted form
so the single-instance case pays no verbosity tax. This does not reopen the `ip`
namespace problem: the `vrf` node is owned by the VRF orchestrator plugin and
*wraps* child trees (children never reach up into it), so plugin
self-containment holds.

## Token naming: hyphen for one name, space for a namespace

Object-rooting decides the *shape* of the tree; it also decides how a single token is
spelled. A hyphen inside a command token joins words that name **one indivisible
thing** (a term of art like `as-set` or `graceful-restart`, an LSA/object name like
`opaque-area`, a single attribute like `max-prefix`). When the left part is really an
**object with members**, it is a namespace, so it gets its own token and becomes a
container node, exactly like every other object root here: `show traffic stat`, not
`show traffic-stat`; `show bgp health`, not `show bgp-health`. That keeps completion
able to enumerate the members and keeps the command tree mirroring the plugin tree.

Two traps this closes:

- A shared prefix is not a namespace. `flow-export` (NetFlow/IPFIX) and `flow-recent`
  (conntrack ring) share "flow" by coincidence; there is no `flow` object, so they stay
  compound names, not `show flow {export,recent}`.
- A split namespace needs one owning module. When two components share the prefix, one
  owns the container and the others augment it (`trafficusage` augments `traffic`),
  never a shared parent multiple plugins reach up into: that is the self-containment
  break the retired `show ip` grouping caused.

The rule and its automated check (R9, sibling-collision) live in
[`ai/rules/cli.md`](../../../ai/rules/cli.md) ("Compound Token vs
Namespace Split").

## Filters are keyword grammar, never `--flags`

The through-line across every vendor above: the family / instance / table
qualifier is a **keyword in the command language** (`vrf RED`, `family ipv6`,
`table inet.0`, `-6`). It is part of the grammar, not a typed-out option string.

In Ze that grammar is defined in YANG. A filter is therefore a YANG keyword
selector (a leaf or container consumed as a keyword, per
[`ai/rules/cli.md`](../../../ai/rules/cli.md)). For example, the
address-family filter on the neighbor table is `show neighbor ipv6`, modelled as a
`family` enum leaf surfaced as a positional keyword in the YANG tree.

The `--flag` form (`--family ipv6`, `--limit 50`) is a **presentation artifact of
the offline `cmd/ze/` Go flag tooling** (`flag.NewFlagSet`). It is how that one
front-end renders options. It is not part of the command model and **must never
appear in a `.yang` file**, including in `description` text or comments.

Why this separation matters:

- YANG is the single source of the command grammar, completion tree, and RPC
  dispatch. If a filter is real grammar, it must be a node in the tree so
  completion and dispatch see it. A `--flag` baked into a description is invisible
  to all of that; it is documentation lying about structure.
- The same command is reached from multiple front-ends (SSH CLI, RPC, offline
  `ze` subcommands). Only the offline flag tooling uses `--flags`. Putting a
  `--flag` spelling in YANG wrongly couples the shared model to one front-end.
- Filters belong to the model; rendering belongs to the front-end. Keep them
  apart.

### Correct vs incorrect

| Incorrect (in YANG) | Correct (in YANG) |
|---|---|
| description: "Filter with `--family` ipv4 or ipv6" | `family` selector node; description: "Filter by address family (ipv4, ipv6, any)" |
| description: "Use `--limit` N for large tables" | `limit` leaf (`uint32`); description: "Maximum number of rows" |

### Mechanical check (must return nothing)

```
grep -rnE '\-\-[a-z]' internal --include='*.yang' | grep -vE 'urn:|http|xml'
```

The rule and this check live in [`ai/rules/cli.md`](../../../ai/rules/cli.md)
("No Flag Syntax in YANG"). `--flags` are legitimate only in the offline
`cmd/ze/` flag tooling described in [`ai/rules/cli.md`](../../../ai/rules/cli.md).
