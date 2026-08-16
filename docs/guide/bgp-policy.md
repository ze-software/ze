# BGP Policy

Ze applies ordered import and export filter chains to BGP updates. Define named policy objects once, attach them globally, to a group, or to one peer, then verify both accepted routes and the routes sent onward.

## Policy model

Policy has two parts:

1. Named filter instances under `bgp { policy { ... } }`.
2. Ordered references under `filter { import [...] export [...] }`.

```text
bgp {
    policy {
        loop-detection no-self-as {
            allow-own-as 0
        }
    }

    filter {
        import [ no-self-as rpki:validate ]
    }

    group customers {
        filter {
            import [ community:scrub ]
            export [ prefix:customer-export ]
        }
    }
}
```

The effective chain is cumulative: automatic safety filters run first, followed by global, group, and peer references. Use `insert` in the editor when order matters.

## Filter outcomes

A filter can:

- accept the update unchanged;
- reject it and stop the chain;
- modify selected attributes and pass the changed update onward.

Each plugin declares whether an internal error fails open or closed. Security filters should normally reject on failure. Operational policy should make that choice explicit rather than depending on an unavailable external service.

## Common policy building blocks

Ze ships filters for:

- AS path matching and length;
- prefix lists;
- standard, large, and extended communities;
- route attribute modification;
- private AS removal;
- address-family filtering;
- RPKI and IRR validation;
- RFC 4271 AS loop and RFC 4456 cluster-list loop detection.

The [plugin guide](plugins.md) documents the registered filter types and their current configuration. The generated [plugin catalogue](https://ze-software.net/reference/plugins/) shows which plugin owns each filter.

## IRR and RPKI

IRR filters build allowed prefix sets from routing registry data. RPKI validates route origin against ROAs. They answer different questions and can be used together:

- IRR: is this prefix documented for this network or AS-SET?
- RPKI: is the origin AS authorised by a cryptographically validated ROA?

Use the [IRR filtering](irr-filtering.md) and [RPKI](rpki.md) guides for cache configuration, refresh, failure behaviour, and operator commands.

## Attribute changes

Attribute modification belongs in an explicit export or import chain. Keep each named modifier narrow, then compose it with other filters. Typical operations include setting local preference or MED, changing next hop, and adding or removing communities.

Inspect the live plugin documentation before using increment or decrement operations. Attribute ranges and missing-attribute behaviour are validated by the owning plugin.

## Well-known communities (RFC 1997)

Ze honors three community values on egress with no configuration, for every peer and every deployment:

| Community | Value | Ze sends the route to |
|-----------|-------|-----------------------|
| `NO_EXPORT` | 0xFFFFFF01 | internal peers only |
| `NO_ADVERTISE` | 0xFFFFFF02 | no peer at all |
| `NO_EXPORT_SUBCONFED` | 0xFFFFFF03 | internal peers only |

The rule applies to a route ze RECEIVED from a peer. A route ze originates and tags itself is still advertised, because tagging your own route is how the community is meant to be used.

Two properties are worth knowing before you write policy around this.

**The check reads the received route, so an egress policy that strips these values does not restore advertisement.** The value ze reads is the one the route arrived with. Removing `NO_EXPORT` in an export chain changes the bytes on the wire. It does not change what the peer asked for, and RFC 1997 states the prohibition as a MUST NOT. FRR and BIRD apply the route-map first, so a strip there does restore advertisement. Ze diverges from both, deliberately: an operator policy cannot grant what the RFC refuses. If a route must leave the AS, ask the peer to stop tagging it.

**Withdrawals are not affected.** All three clauses forbid ADVERTISING the route. One UPDATE can carry withdrawn routes and an announcement together. A peer refused the announcement still receives the withdrawals, so it never keeps a prefix ze can no longer take back.

Each suppression increments `ze_bgp_wellknown_community_suppressed_total` with the community as its label. Nothing else reports it: there is no per-route log line and no configuration switch, because a switch here would be a switch to violate the RFC.

<!-- source: internal/component/bgp/wireu/wellknown.go — ScanWellKnown, WellKnown.AllowsEgressTo -->
<!-- source: internal/component/bgp/reactor/forward_wellknown.go — wellKnownAllowsEgress -->

## Ingress community rules

The `bgp-filter-community` plugin adds three RFC-derived rules to the ingress
direction, beside the named `tag` and `strip` sets. Each one is a leaf under
`filter { ingress { community { } } }`, and the container is available at the
`bgp`, group, group-peer, and standalone-peer levels.

| Leaf | Type | Default | Meaning |
|------|------|---------|---------|
| `relation-tag` | boolean | `false` | Write the RFC 8195 Section 3.2 relation-to-origin large community on each route this peer sends. |
| `relation-function` | uint32 | `3` | Function number of that large community. RFC 8195 leaves the number to each AS and gives 3 as its example. |
| `scrub-own-ga` | boolean | `false` | Remove inbound communities whose Global Administrator is the local AS. RFC 7454 Section 11. |
| `scrub-keep-function` | leaf-list (uint32) | empty | Function numbers a peer is allowed to send with the local AS as the Global Administrator. |
| `blackhole-propagation` | enum | `none` | Add `no-export` or `no-advertise` to a route that carries BLACKHOLE. RFC 7999 Section 3.2. |

```text
bgp {
    group customers {
        filter {
            ingress {
                community {
                    scrub-own-ga        true;
                    scrub-keep-function [ 100 200 ];
                    relation-tag        true;
                    blackhole-propagation no-export;
                }
            }
        }
    }
}
```

The four steps run in one fixed order on each received route: the named `strip`
sets, then the scrub, then the blackhole guard, then the named `tag` sets. The
order is load-bearing. The guard adds a well-known community, so a scrub running
after it would delete what it just added.

**The scrub is a keep-list, not a denylist.** RFC 7454 Section 11 asks an
operator to scrub inbound communities carrying their own AS number and to allow
only the ones customers and peers use for signaling. An empty
`scrub-keep-function` list therefore keeps nothing. It does not mean "keep
everything". The scrub covers standard and large communities, and it runs on
eBGP sessions only: on an iBGP session an own-AS value is this network's own
signal to itself. Well-known communities are never reached, so `NO_EXPORT`,
`NO_ADVERTISE` and `BLACKHOLE` always survive. The `relation-function` number is
never kept, whatever the list says, because a kept relation function would let a
peer state its own relation to this AS.

**The relation tag reads the resolved peer role.** The parameter says what the
peer IS to this AS: 2 for a customer, 3 for a peer, 4 for a provider. It comes
from the role the `bgp-role` plugin resolved, which prefers a Role capability the
peer announced over the local configuration. A session whose role does not
resolve gets no tag. An iBGP session gets none, because its sender is inside the
local AS. A route-server or route-server-client session gets none, because RFC
7947 requires transparency there. The value is a large community
`<local-AS>:<function>:<parameter>`. Any own-AS large community already carrying
that function number is removed first, so a peer cannot state its own relation to
you.

**The blackhole guard adds a community. It does not discard traffic.** RFC 7999
Section 3.2 asks a receiver to stop a BLACKHOLE-tagged prefix propagating outside
the local AS, and leaves the choice between `NO_EXPORT` and `NO_ADVERTISE` to the
operator. Discarding the traffic is a different feature: see [blackhole
honoring](configuration.md#blackhole-honoring-rfc-7999). RFC 7999 Section 3.1
makes ignoring the community conformant, so `none` is a supported state.

<!-- source: internal/component/bgp/plugins/filter_community/yang/ze-filter-community.yang -- community-filter-fields -->
<!-- source: internal/component/bgp/plugins/filter_community/filter.go -- applyIngressFilter, applyRelationTag -->
<!-- source: internal/component/bgp/plugins/filter_community/scrub.go -- scrubOwnGACommunities -->
<!-- source: internal/component/bgp/plugins/filter_community/blackhole.go -- blackholePropagationGuard -->

## Suppressing communities on egress

`session { community { send ... } }` selects which community attribute types
leave on a session. The list takes `standard`, `large`, `extended`, `all`, or
`none`. Unset is `all`.

```text
bgp {
    peer transit-a {
        session { community { send [ standard ]; } }
    }
}
```

A type that is not selected is removed from every UPDATE sent to that peer, on
both forward rails. Two attributes are the exception and they are refused out
loud rather than silently: `OTC` (RFC 9234 Section 5) and `ORIGINATOR_ID` (RFC
4456 Section 8) must be preserved unchanged once set, so a suppression of either
is logged and not applied.

<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- session/community/send -->
<!-- source: internal/component/bgp/reactor/peer_forward_facts.go -- applyFactsSendCommunity -->

## Route-server control communities

When ze runs as a route server, a client can steer its own routes by tagging them with communities the route server reads and then removes. A peer becomes a route-server client with `rs-client true` under its `session` block; the communities below are only interpreted for such peers.

| Community | Meaning |
|-----------|---------|
| `0:<asn>` | Do not advertise this route to `<asn>`. |
| `<rs-asn>:<asn>` | Advertise this route ONLY to the listed ASNs. |
| `<rs-asn>:0` | Do not advertise this route to any client. |

A whitelist wins over a blacklist: if a route carries any `<rs-asn>:<asn>` tag, only the listed ASNs receive it and `0:<asn>` tags on the same route are not consulted.

These are control tags, not attributes to pass on. The route server strips every one of them from the route before forwarding it, so a client never sees another client's steering instructions or the route server's own. A route carrying several control communities has all of them removed, not the first one. Communities the route server does not recognise are forwarded untouched and in their original order, so a client's own tagging survives.

`65535:666` is NOT one of these. RFC 7999 BLACKHOLE steers no route-server forwarding and is never stripped, so a client's blackhole request reaches the other clients as the client wrote it. Ze's own answer to a received BLACKHOLE is the per-session `blackhole` container and the ingress `blackhole-propagation` leaf, both above.

```
# Client tags a route: keep it away from AS 64998, and let AS 65002 have it.
# The route server removes both tags; 65001:100 reaches AS 65002 unchanged.
community [ 0:64998 65000:65002 65001:100 ]
```

A control community decides about a ROUTE, not about a MESSAGE. One UPDATE can carry withdrawn routes and an announcement together. The withdrawn routes carry no attribute and were tagged by nobody, so a client the announcement excludes still receives the withdrawals in that UPDATE. Without that, an excluded client would keep a prefix an earlier UPDATE did advertise to it, and ze could not take it back until the session reset.

Two limits are worth knowing:

- A standard community's high half is 16 bits (RFC 1997), so a route server with a 4-octet ASN matches `<rs-asn>:<asn>` on the low 16 bits of its ASN only. Use large communities where that is ambiguous.
- Stripping is ze's own forwarding behaviour. RFC 7947 expects per-client import and export policy but prescribes no mechanism for it (Section 2.3.2.1 offers per-client RIBs as "the most portable method"), so a different route-server implementation may leave these tags in place.

`ze_bgp_attr_mod_remove_buffer_refused_total{attribute}` counts removal buffers a
handler refused because they were not a whole number of wire values. The refusal
is silent on the wire, so a non-zero value is the one signal that a producer is
violating the arity contract and that an attribute went out unchanged.

<!-- source: internal/component/bgp/wireu/community.go — ParseCommunityPolicy, ShouldForwardTo, StripControlCommunities -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go — general forwarding rail; forward_rs.go — route-server fast path -->

## Redistribution

The top-level `redistribute` block moves selected routes between protocol RIBs. Redistribution is separate from ordinary per-peer BGP filtering, but the resulting routes still pass through the destination protocol's policy and loop-prevention rules.

Use named destinations and specify the source protocols deliberately. Avoid broad two-way redistribution unless the topology and loop controls have been proven. Connected, static, kernel, OSPF, IS-IS, and BGP route sources have different ownership and withdrawal behaviour.

See [Route filters and redistribution](redistribution.md) for the current chain rules and configuration syntax.

## Verify policy

Use the same route at each boundary:

```console
ze cli -c "show bgp rib peer transit-a"
ze cli -c "show bgp rib best"
ze cli -c "show bgp irr check peer transit-a prefix 192.0.2.0/24"
ze cli -c "show bgp peer transit-b rib"
```

Confirm:

1. The route is present or absent at the expected import boundary.
2. The best-path result carries the intended local attributes.
3. Exported attributes match the destination peer's policy.
4. A rejected route does not reappear through redistribution.
5. A policy reload completes without warnings or partial application.

## Related guides

- [BGP peering](bgp-peering.md)
- [BGP resilience](bgp-resilience.md)
- [IRR filtering](irr-filtering.md)
- [RPKI origin validation](rpki.md)
- [Route injection](route-injection.md)
