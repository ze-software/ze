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
| `show ospf ...` / `clear ospf ...` | OSPF views and runtime resets (ospf plugin) |

Two points this settles:

1. **No `show ip` / `show ipv6` split.** That is the Cisco model Ze deliberately
   did not adopt. One tree, one `family` filter.
2. **`arp` is IPv4-only, by design.** ARP is IPv4 by definition (RFC 826); IPv6
   uses Neighbor Discovery. So `show neighbor` is the honest name for the unified
   both-families table (the Linux `ip neigh` model), and `show arp` is kept as the
   familiar IPv4-only shortcut (`= show neighbor ipv4`).

This replaced an earlier transitional layout that grouped a few reads under a
shared `show ip` container (`show ip route`, `show ip arp`, `show ip ospf`). That
container conflated "address family" with "object group" and was the one place the
command tree broke plugin self-containment (iface and ospf both reached into a
shared `ip` parent). Object-rooting removes the shared parent: each plugin owns its
root outright.

### Why not Nokia's `router` root?

Because Ze is single-instance today. An instance-rooted namespace (`show router
...`) only pays off once there are multiple routing instances / VRFs to
disambiguate. Adopting it now would be verbosity with no instance to select.

Forward-looking: if Ze gains VRF / routing-instance support, the instance becomes
the natural scoping keyword, and the choice is between Nokia's root form (`show
router <name> route`) and the Juniper/Cisco filter form (Cisco `show ip route vrf
<name>`, Juniper `show route table <instance>`). Given Ze's family-as-filter
stance, the **filter form** for Ze (`show route vrf <name>`, with `vrf <name>` a
keyword selector) is the consistent extension: instance and family are both
filters, the tree stays flat. Revisit at that point; do not pre-build it.

## Filters are keyword grammar, never `--flags`

The through-line across every vendor above: the family / instance / table
qualifier is a **keyword in the command language** (`vrf RED`, `family ipv6`,
`table inet.0`, `-6`). It is part of the grammar, not a typed-out option string.

In Ze that grammar is defined in YANG. A filter is therefore a YANG keyword
selector (a leaf or container consumed as a keyword, per
[`ai/rules/cli-grammar.md`](../../../ai/rules/cli-grammar.md)). For example, the
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

The rule and this check live in [`ai/rules/cli-grammar.md`](../../../ai/rules/cli-grammar.md)
("No Flag Syntax in YANG"). `--flags` are legitimate only in the offline
`cmd/ze/` flag tooling described in [`ai/rules/cli-patterns.md`](../../../ai/rules/cli-patterns.md).
