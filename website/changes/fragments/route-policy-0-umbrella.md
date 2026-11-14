---
spec: route-policy-0-umbrella
topic: Redistribution
feature: route-policy
---

## Before

Route policy belonged to BGP alone. Filters were defined under
`bgp { policy { } }` and judged only BGP sessions. Routes from IS-IS, OSPF,
static and connected entered the RIB with no filter at all, so there was no way
to say "install only these IS-IS prefixes", "never install a bogon from any
protocol" or "advertise only loopbacks into IS-IS".

Redistribution had a small matcher of its own, on family and tag, and
`import ebgp` and `import ibgp` matched no route. The metric, metric type and
tag of a route redistributed into OSPF were per-source settings under
`ospf { redistribute { } }`.

## After

Filters are defined once, under the top-level `policy { }` root, which they
share with policy routing's `policy { route ... }`. A configuration that still
has `bgp { policy { } }` is refused at load: move the block to `policy { }`.

The same chain, `filter { import { filters [ ... ]; final accept; } }` (or
`final reject`), now attaches to `isis`, `ospf`, `static` and `connected`, and
to `rib { filter { import { } } }`, which judges every source's route after
that source's own chain, BGP's best path included. A route a chain rejects stays
in the RIB as a hidden path: it is never selected or written to the kernel,
`show rib candidates` lists it with the chain that hid it, and relaxing the
filter re-admits it at the next commit with no protocol event. An IS-IS or OSPF
import filter decides only what this router installs: the link-state database
and flooding are untouched.

`isis` and `ospf` gain `filter { export { } }` for what they redistribute, and
an OSPF export chain sets the external metric, metric type and tag in place of
the old per-source settings. A redistribute `import` takes `filters` and
`final` in place of `family` and `tag`, and `import ebgp` and `import ibgp` now
move only the best paths learned from that kind of peer. At an import, a
`match` filter can test the tag, the protocol, the IS-IS level or the OSPF route
type, and a `modify` filter can set the administrative distance or the tag of
the routes it matches.

A chain is refused at load when one of its filters reads a field the attachment
does not carry, and the error names the filter, the field and the attachment:
for example a community match on IS-IS import, as IS-IS routes carry no
communities. Route policy is Experimental.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
