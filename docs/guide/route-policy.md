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

Two filter types can run at a Loc-RIB attachment today:

| Filter type | What it does to a route |
|-------------|-------------------------|
| `prefix-list` | The first entry that matches decides: `accept` passes the route on, `reject` stops the chain. A route that matches no entry is rejected |
| `family-filter` | `action remove` rejects a route of the instance's family and passes every other route on. `action tear-down` closes a BGP session, so it is refused at a Loc-RIB attachment |
<!-- source: internal/component/bgp/plugins/filter_prefix/rib_evaluator.go -- compileRouteEvaluator -->
<!-- source: internal/component/bgp/plugins/filter_family/rib_evaluator.go -- compileRouteEvaluator -->

Both types come from BGP plugins. A build without the `ze_bgp` tag has neither,
so it refuses every Loc-RIB chain that names one.
<!-- source: feature-gates.txt -- ze_bgp -->
<!-- source: internal/component/policy/rib_compile.go -- compileRIBChain -->

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
<!-- source: internal/component/policy/rib_compile.go -- readRIBChain, compileRIBChain -->
<!-- source: internal/component/policy/fields.go -- CheckChain -->
<!-- source: internal/component/bgp/plugins/filter_family/rib_evaluator.go -- compileRouteEvaluator -->

The editor does not offer instance names when you complete `filters`. The check
at verify is what refuses a wrong name.
<!-- source: internal/component/cli/completer.go -- valueCompletions -->
