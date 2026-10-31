# Route Policy at the Loc-RIB

Ze holds one Loc-RIB for every route source. An import chain judges each route
as it enters that Loc-RIB. This page shows where you attach a chain, how the
chain decides, and what happens to a route it rejects. BGP sessions keep their
own import and export chains: [BGP Policy](bgp-policy.md) covers them.

## The shared `policy` root

Every filter instance is defined once, under the top-level `policy {}` root.
Each filter type is a list there, and an entry of that list is a named
instance. A chain names instances. One name belongs to one filter type only.
<!-- source: internal/component/policy/yang/ze-policy-conf.yang -- container policy -->
<!-- source: internal/component/policy/rib_compile.go -- registryFromPolicy -->

Four filter types can run at a Loc-RIB attachment today:

| Filter type | What it does to a route |
|-------------|-------------------------|
| `match` | Tests the route against the conditions the instance names. With `action accept`, the default, a route that meets every condition passes on and any other is rejected. With `action reject`, a route that meets them is rejected and any other passes on |
| `modify` | A route that meets its `match` block gets the values its `set` block names. At a Loc-RIB attachment, `set` takes `distance` and `tag`. The route always passes on |
| `prefix-list` | The first entry that matches decides: `accept` passes the route on, `reject` stops the chain. A route that matches no entry is rejected |
| `family-filter` | `action remove` rejects a route of the instance's family and passes every other route on. `action tear-down` closes a BGP session, so it is refused at a Loc-RIB attachment |
<!-- source: internal/component/policy/match.go -- compileMatchEvaluator, RouteMatch.Rejects -->
<!-- source: internal/component/bgp/plugins/filter_modify/rib_evaluator.go -- compileRouteEvaluator -->
<!-- source: internal/component/bgp/plugins/filter_prefix/rib_evaluator.go -- compileRouteEvaluator -->
<!-- source: internal/component/bgp/plugins/filter_family/rib_evaluator.go -- compileRouteEvaluator -->

`match` belongs to the policy component and is in every build. The other three
come from BGP plugins, so a build without the `ze_bgp` tag refuses a Loc-RIB
chain that names one of them. Their Loc-RIB evaluators are linked into the
daemon and run inside the policy plugin, so a Loc-RIB chain needs no BGP
plugin process and no `bgp { }` block: a config with only `policy` and an
IS-IS, OSPF, static or connected import chain starts without BGP. The BGP
filter plugins themselves load only when the config holds a `bgp { }` block.
<!-- source: feature-gates.txt -- ze_bgp -->
<!-- source: internal/component/bgp/plugins/filter_prefix/register.go -- MustRegisterRouteEvaluator, ConfigRoots -->
<!-- source: internal/component/policy/register.go -- MatchFilterType -->
<!-- source: internal/component/policy/rib_compile.go -- compileRIBChain -->

### The `match` conditions

A `match` instance names one or more conditions. A route must meet every
condition it names, and a condition with several values is met by any one of
them. A route that does not carry the field a condition reads never meets it:
an untagged route fails `tag [ 0 ]`.

| Condition | Met when |
|-----------|----------|
| `prefix-list <name>` | The named `prefix-list` instance accepts the prefix. The editor completes the configured prefix-lists, and a commit naming any other is refused |
| `family [ ... ]` | The route's address family is listed, such as `ipv4/unicast` |
| `protocol [ ... ]` | The route's source is listed: a registered source such as `static`, or `ebgp` or `ibgp` for a BGP path |
| `next-hop [ ... ]` | The next hop lies in a listed prefix. A connected route has no next hop |
| `interface [ ... ]` | The outgoing interface is listed |
| `metric { minimum; maximum; }` | The metric lies in the range. Either bound may be left out |
| `tag [ ... ]` | The route carries one of the tags |
| `level [ ... ]` | The IS-IS level is listed, 1 or 2 |
| `route-type [ ... ]` | The OSPF path type is listed: `intra-area`, `inter-area`, `external-1`, `external-2`, `nssa-1`, `nssa-2` |
<!-- source: internal/component/policy/match.go -- matchConditions, RouteMatch.Matches -->
<!-- source: internal/component/policy/yang/ze-policy-conf.yang -- list match -->

This hides every static route tagged 4242 and keeps the others:

```
policy {
    match NO-4242 {
        action reject;
        tag [ 4242 ];
    }
}
rib {
    filter {
        import {
            filters [ NO-4242 ];
            final accept;
        }
    }
}
```

`modify` takes the source conditions in its `match` block: `protocol`, `tag`,
`level` and `route-type`. A `modify` with `match { tag [ 100 ]; }` and
`set { distance 90; }` on `ospf filter import` ranks OSPF routes tagged 100 at
distance 90 and leaves the others at the OSPF distance.
<!-- source: internal/component/policy/match.go -- IsSourceConditionKey -->
<!-- source: internal/component/bgp/plugins/filter_modify/rib_evaluator.go -- parseRIBSets -->

### The fields each attachment carries

Ze checks each instance against the fields it reads and sets. A route carries
only what its source knows, so a filter reading a field the source never sets
is refused rather than matching nothing.

| Attachment | Routes carry | A filter may set |
|------------|--------------|------------------|
| `connected filter import` | prefix, family, protocol, interface | distance, tag |
| `static filter import` | prefix, family, protocol, next-hop, interface, metric, tag | distance, tag |
| `ospf filter import` | prefix, family, protocol, next-hop, interface, metric, tag, route-type | distance, tag |
| `isis filter import` | prefix, family, protocol, next-hop, interface, metric, level | distance, tag |
| `rib filter import` | prefix, family, protocol, next-hop, interface, metric, tag, level, route-type | distance, tag |
<!-- source: internal/plugins/connected/register.go -- MustRegisterAttachment -->
<!-- source: internal/plugins/static/register.go -- MustRegisterAttachment -->
<!-- source: internal/plugins/ospf/register.go -- MustRegisterAttachment -->
<!-- source: internal/plugins/isis/register.go -- MustRegisterAttachment -->
<!-- source: internal/component/sysrib/register.go -- MustRegisterAttachment -->

IS-IS routes carry no tag, because Ze does not implement the IS-IS
administrative tag (RFC 5130). A static route carries a tag only when its `tag`
leaf is set, and `tag 0` is a real tag. An OSPF external route carries the tag
of its LSA.

## Where you attach a chain

Five config roots carry `filter { import { ... } }`:

| Root | Chain | Routes it judges |
|------|-------|------------------|
| `isis` | source | Routes IS-IS installs |
| `ospf` | source | Routes OSPF installs |
| `static` | source | Static routes |
| `connected` | source | Connected routes |
| `rib` | central | Routes from every source, BGP included |
<!-- source: internal/component/policy/yang/ze-policy-conf.yang -- grouping rib-import-filter -->
<!-- source: internal/plugins/isis/register.go -- MustRegisterRIBImport -->
<!-- source: internal/plugins/ospf/register.go -- MustRegisterRIBImport -->
<!-- source: internal/plugins/static/register.go -- MustRegisterRIBImport -->
<!-- source: internal/plugins/connected/register.go -- MustRegisterRIBImport -->
<!-- source: internal/component/sysrib/register.go -- MustRegisterRIBImport -->

A route meets its source's chain first, then the `rib` chain. A route is
selected only when both accept it.
<!-- source: internal/core/rib/policy/policy.go -- Snapshot.Evaluate -->

## How a chain decides

A chain is an ordered `filters` list and a mandatory `final`. Each filter
either rejects the route, which stops the chain, or passes it on to the next
filter. A filter's accept means "continue", not "install". After the last
filter, `final accept` accepts the route and `final reject` rejects it. So
`final` decides what survives the chain.
<!-- source: internal/component/policy/rib_compile.go -- ribChain.Evaluate -->

This example installs 10.0.0.0/24 from IS-IS and hides 10.1.0.0/24:

```
policy {
    prefix-list ISIS-IN {
        entry 10.0.0.0/24 { action accept; }
    }
}
isis {
    filter {
        import {
            filters [ ISIS-IN ];
            final accept;
        }
    }
}
```

10.0.0.0/24 matches the entry and passes on, and `final accept` accepts it.
10.1.0.0/24 matches no entry, so the prefix-list rejects it. With
`final reject` instead, the chain would reject both prefixes, because every
route that passes the filters then meets the reject.
<!-- source: internal/component/bgp/plugins/filter_prefix/match.go -- evaluatePrefix -->
<!-- source: internal/component/policy/rib_compile.go -- ribChain.Evaluate -->

## What a rejected route becomes

**The protocol's own database is never filtered.** The chain judges the route
as it enters the Loc-RIB. IS-IS keeps the prefix in its LSDB and floods it to
its neighbors, and OSPF keeps its LSDB the same way.
<!-- source: internal/component/policy/yang/ze-policy-conf.yang -- grouping rib-import-filter -->

**A rejected route stays in the Loc-RIB as a hidden path.** A hidden path is
never selected, so it is not the best path and it does not reach the FIB. When
every path for a prefix is hidden, the prefix has no best path and Ze removes
its FIB entry.
<!-- source: internal/core/rib/locrib/policy.go -- admit -->
<!-- source: internal/core/rib/locrib/entry.go -- selectBest -->

**A reload re-judges every stored path.** When you commit a changed chain, Ze
judges each path again from the values its protocol offered, hidden paths
included. Relax the filter and the hidden path is installed, with no IS-IS or
OSPF event and no SPF run. Tighten it and the path is withdrawn.
<!-- source: internal/component/policy/rib_compile.go -- installRIBImports -->
<!-- source: internal/core/rib/locrib/policy.go -- PublishPolicy -->

## Seeing the hidden paths

`show rib` lists only selected paths, so a hidden path never appears there.
`show rib candidates` lists every path each protocol offered, hidden ones
included. Add `prefix <prefix>` to see one prefix:

```
show rib candidates prefix 192.0.2.0/24
```

Each path carries its source, next hop, interface, distance, metric, and the
tag, IS-IS level or OSPF route type its protocol states. `selected` marks the
path the Loc-RIB chose. A hidden path has `hidden` true, `hidden-by` set to
`source` (its own protocol's chain) or `central` (the `rib` chain), and
`chain` naming the attachment, such as `isis import` or `rib import`. The
chain shown is the one whose verdict the Loc-RIB acted on, not a fresh
evaluation, so a reload in flight cannot make the answer disagree with what
was installed. The answer is structured, so `| json`, `| table` and `| match`
apply. Anything after `candidates` other than `prefix <prefix>` is refused with
the correct form.
<!-- source: internal/component/sysrib/candidates.go -- showRIBCandidates, candidatesPrefix -->
<!-- source: internal/core/rib/locrib/policy.go -- admit, rejectingChain -->

## When an IGP filter blackholes transit traffic

Filtering an IS-IS or OSPF prefix locally does not stop the IGP from flooding
it, so the routers around you keep sending its traffic here. When the hidden
prefix is the route a selected next hop resolves through, this router resolves
that next hop through a shorter prefix, or not at all, and the traffic is
blackholed or loops. `show doctor` warns with
`doctor-policy-igp-import-hides-nexthop` when a hidden IS-IS or OSPF path is
longer than the cover a selected route's next hop resolves through now. The
warning names the chain, the hidden prefix, the next hop and the route, and
points at `show rib candidates prefix <prefix>`. Let the chain accept that
prefix, or stop originating it in the IGP. The check reads the running
daemon's Loc-RIB, so `ze doctor` before a start does not report it.
<!-- source: internal/component/policy/doctor.go -- igpImportDoctorCheck, igpImportHidesNextHop -->
<!-- source: internal/component/policy/rib.go -- RIBImport.Floods -->

## Metrics

| Metric | Labels | Counts |
|--------|--------|--------|
| `ze_rib_policy_hidden_paths` | `source` | Paths of that source the Loc-RIB holds hidden now |
| `ze_rib_policy_rejected_total` | `source`, `chain` (`source` or `central`) | Rejections. Every route a protocol offers that a chain rejects counts once. A reload counts a stored path again only when its verdict changes, so recommitting the same chain adds nothing |
<!-- source: internal/core/rib/locrib/policy_metrics.go -- SetPolicyMetricsRegistry, accountStored, accountRemoved -->

## What Ze refuses

Ze checks every chain when it verifies the config. These are refused, and the
message names the chain and the fix:

| Refused | Why |
|---------|-----|
| An `import` with no `final` | Every import chain needs `final accept` or `final reject`, even with no filters |
| A `<plugin>:<filter>` reference | The Loc-RIB gate runs in the engine and never calls a plugin process. Name an instance from the `policy` root instead |
| A name that is not a filter instance | The name must be an entry of a filter type list under `policy` |
| A filter that reads a field the routes do not carry | For example, a community match on IS-IS routes |
| A filter type with no Loc-RIB route evaluator | The message lists the filter types that have one |
| A `family-filter` with `action tear-down` | A Loc-RIB route has no session to close |
| A `match` instance on a BGP peer chain | No BGP filter serves `match`, so it could never run there. A `modify` that sets `distance` or `tag` on a BGP chain is refused the same way, naming the field |
<!-- source: internal/component/bgp/config/peers.go -- validateChainNames, bgpServesFilterType -->
<!-- source: internal/component/policy/rib_compile.go -- readRIBChain, compileRIBChain -->
<!-- source: internal/component/policy/fields.go -- CheckChain -->
<!-- source: internal/component/bgp/plugins/filter_family/rib_evaluator.go -- compileRouteEvaluator -->

Tab on `filters` offers the filter instances this chain accepts, and only
those: every entry of a filter type list under `policy` whose fields fit the
attachment, so `isis filter import filters [ ` offers a `prefix-list` and not a
`community-match`, and a BGP chain offers no `match`. A `policy route` entry is
never offered. Commit asks the same question of every name, member by member,
with the refusals in the table above, so a name the editor did not offer still
commits when the attachment accepts it: a qualified `prefix-list:NAME`, or an
external plugin's `<plugin>:<filter>` on a BGP chain.
<!-- source: internal/component/policy/reference_source.go -- filterReference, chainLeafAt, MustRegisterChainLeaf -->
<!-- source: internal/component/bgp/config/register.go -- MustRegisterChainLeaf -->
<!-- source: internal/component/cli/completer.go -- referenceCompletions -->
