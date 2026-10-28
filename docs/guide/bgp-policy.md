# BGP Policy

Ze applies ordered import and export filter chains to BGP updates. Define named policy objects once, attach them globally, to a group, or to one peer, then verify both accepted routes and the routes sent onward.

## Policy model

Policy has two parts:

1. Named filter instances under `bgp { policy { ... } }`.
2. Direction policies under `filter`, each with ordered `filters` and a required `final accept` or `final reject`.

```text
bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }
    policy {
        loop-detection no-self-as {
            allow-own-as 0
        }
    }

    filter {
        import { filters [ no-self-as rpki:validate ]; final accept; }
    }

    group customers {
        filter {
            import { filters [ community:scrub ]; final accept; }
            export { filters [ prefix:customer-export ]; final accept; }
        }
    }
}
```

The effective references are cumulative: automatic safety filters run first,
followed by global, group, and peer references. The most-specific configured
direction supplies its final decision; references inherited from less-specific
levels are not replaced. Each configured direction must state its own final,
even if it has no references. Use `insert` in the editor when order matters.

For example, `filter { import { filters [ name ]; final accept; } export {
final reject; } }` runs `name` on import and rejects every export that reaches
the policy final. With no explicit policy at any level, EBGP rejects routes
in that direction (RFC 8212 Section 3). IBGP keeps its accepting default,
including sessions using the migration AS. Automatic and inactive references
do not count as an explicit policy.
<!-- source: internal/component/bgp/filterapi/policy.go -- Policy.AcceptsAtEnd -->

Named `loop-detection` instances remain visible in the effective import policy,
including when the direction has no operator `final`. Plain `no-self-as`,
`loop-detection:no-self-as`, and canonical `loop:no-self-as` references select
the same instance. Its settings control the existing protocol-stage loop check;
the instance is not an external plugin and is not executed a second time in
the policy RPC chain. Deactivating its import reference disables that check
for the peer. Earlier named automatic references could incorrectly attempt an
RPC to the instance name and fail with `unknown plugin`.
An active named internal reference on export is refused: these instances
configure ingress checks, not an export policy. The refusal names the location
where the reference was written and prints the exact editor command to
deactivate it. Use that original location and spelling, not the effective
peer's canonical reference. For example, a reference written on `group customers`
as `loop-detection:no-self-as` is corrected in the configuration editor with:

```text
top
deactivate bgp group customers filter export filters loop-detection:no-self-as
commit now
```

Deactivation keeps that member as a deliberate no-check decision; it does not
remove other filters or change the final. Removing the reference at its original
location is also an option. Automatic import references never populate export.

A reload compares the whole effective policy. Changing only the export `final`
therefore reconciles advertisements on the existing session without a source
UPDATE. Rejecting withdraws prior advertisements. Returning to acceptance
regenerates configured static, default and plugin routes and replays retained
received routes through the configured replay
owner. Ordinary received-route restoration requires the receive store and that
owner's existing send permission. FlowSpec instead restores the selecting RIB's
currently authorized paths through the same owner. A destination negotiating only
FlowSpec does not require the optional receive store; negotiated ordinary families
still require their retained source.
Removing the last explicit export policy rejects on EBGP. On IBGP, removing an
accepting direction needs no replay or static-route reset. Routes removed in the
same reload are still withdrawn.
If required restoration fails, reload returns the error and rolls back the policy
instead of committing an accepting configuration with stale rejected state.

One-shot API announcements have no retained desired source. They are withdrawn
on rejection, but their producer must reannounce them after acceptance returns.
This preserves the existing API ownership contract rather than introducing a
second route store. Explicit withdrawals remain effective under `final reject`.
Reloading a filter definition without changing the final is a separate operation.
<!-- source: internal/component/bgp/reactor/policy_reload.go -- reconcileExportFinal -->

## Filter outcomes

A filter can:

- accept the update unchanged;
- reject it and stop the chain;
- modify selected attributes and pass the changed update onward.

A filter's `accept` means continue, not bypass the remaining policy. After the
last active reference, Ze applies `final`. An empty policy, or one whose
references are all inactive, still applies it. A raw full-UPDATE rewrite stops
later filters, but still receives the policy final. An earlier reject,
fail-closed error, or teardown cannot be turned into acceptance by `final accept`.

Each plugin declares whether an internal error fails open or closed. Security filters should normally reject on failure. Operational policy should make that choice explicit rather than depending on an unavailable external service.

## Common policy building blocks

Ze ships filters for:

- AS path matching and length;
- transit-leak rejection by ASN and position (`reject-asn`);
- prefix lists;
- standard, large, and extended communities;
- route attribute modification;
- private AS removal;
- address-family filtering;
- RPKI and IRR validation;
- RFC 4271 AS loop and RFC 4456 cluster-list loop detection.

The [plugin guide](plugins.md) documents the registered filter types and their current configuration. The generated [plugin catalogue](https://ze-software.net/reference/plugins/) shows which plugin owns each filter.

## Rejecting a path through a transit provider

A `reject-asn` list names the ASNs that must not appear in a path exchanged with
a peer, and states WHERE in the AS_PATH each one is unacceptable. A peer that
leaks its transit sends paths that run through its upstream, and this list drops
those routes while the session stays up.

```
bgp {
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }
    policy {
        reject-asn NO-TRANSIT {
            indirect [ 174 701 3356 ];
        }
    }
    peer ix-member {
        role   { import peer; }
        filter {
            import { filters [ NO-TRANSIT ]; final accept; }
            export { filters [ NO-TRANSIT ]; final accept; }
        }
    }
}
```

Seven keywords say where: `direct`, `indirect`, `transit`, `origin`, `anywhere`,
`nth <n>` and `regex`. `indirect` is the one most operators want, because it
excludes the sending peer's own leading run and catches the ASN everywhere else.
The [configuration reference](../config-reference.md) carries the whole table.

Declaring an RFC 9234 role makes this check mandatory on the chains that role
binds: the config is refused unless each bound chain names a `reject-asn`
filter, or names one prefixed with `inactive:` to record the decision to run
without it. See [BGP roles](bgp-role.md).

`show bgp reject-asn` prints every list with the effective position set per ASN.
`show bgp reject-asn known transit-free` prints a pasteable block of well-known
transit ASNs; Ze suggests them and never applies them on its own.

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

A refused announcement produces a withdrawal of its destinations as well. The
forwarding rails do not retain a per-destination advertised inventory, so they
cannot prove that the peer never held an earlier generation. Withdrawing it
prevents a newly restricted route from remaining usable. This is not an
advertisement: wire assertions must distinguish reachable NLRI from Withdrawn
Routes and MP_UNREACH_NLRI, rather than reject a prefix anywhere in an UPDATE.
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- forwardUpdateCore -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRSSection -->

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
    router-id 192.0.2.1;
    session {
        asn {
            local 65000;
        }
    }
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
`NO_ADVERTISE` and `BLACKHOLE` always survive. On a session with `rs-client
true`, the scrub keeps the route-server control communities the client sends
to the route server: `RS:X` and `RS:0` in the standard form, and
`RS:101:X` to `RS:103:X` in the large form (see "Route-server control
communities" below). They are the signalling Section 11 tells you to allow, and
the route server reads them after the ingress filters, so scrubbing them would
silently cancel every announce, do-not-announce and prepend request. The route
server removes them before any client receives the route. `RS` is the route
server's AS: the router's global `session asn local`, never a per-peer
local-as override, because that is the AS the route server reads and removes
them by. On a client session with a local-as override the scrub's own AS is the
override, so a value under it shaped like a control is no request the route
server reads, and it is scrubbed. The real controls carry the global AS and are
outside the scrub's reach. Every other own-AS value from that client is still
scrubbed. The `relation-function` number is
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

**A received BLACKHOLE route reaches an eBGP peer only if that peer agreed.**
RFC 7999 Section 3.1: "In a bilateral peering relationship, use of the BLACKHOLE
community MUST be agreed upon by the two networks before advertising it." The
agreement is the peer's `blackhole` container listing `blackhole` (65535:666).
On egress, a route carrying BLACKHOLE is withheld from an eBGP peer whose
container does not list it, and when the peer held an earlier generation of the
route it is withdrawn. iBGP sessions (an RFC 7705 migration session included)
and route-server clients still receive it: an iBGP session is one network, and
Section 3.1 hands the multilateral case to the operator's policy. A reload that
changes a `blackhole` container takes effect on the next UPDATE of the route;
it does not re-judge what a peer already holds.

<!-- source: internal/component/bgp/plugins/filter_community/blackhole.go -- blackholeReadvertiseRefused -->
<!-- source: internal/component/bgp/plugins/filter_community/filter_community.go -- egressFilter, newCommunityConfigTx, installCommunityConfig -->
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
    router-id 192.0.2.1
    session {
        asn {
            local 65000
        }
    }
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
| `<rs-asn>:101:<asn>` | Large community. Prepend the route server's ASN once to the AS_PATH sent to `<asn>`. |
| `<rs-asn>:102:<asn>`, `<rs-asn>:103:<asn>` | Large community. The same, two or three times. |

A whitelist wins over a blacklist: if a route carries any `<rs-asn>:<asn>` tag, only the listed ASNs receive it and `0:<asn>` tags on the same route are not consulted.

A prepend request whose `<asn>` is `0` applies to every client. When a route carries a request for a client and one for every client, the client's own wins; when it carries two for the same client, the larger count wins.

Without a prepend request a client receives the AS_PATH exactly as the route server received it, as RFC 7947 Section 2.2.2.1 recommends. That section makes the rule a SHOULD NOT rather than a MUST NOT only to stay compatible with older clients that still reject a route whose first AS is not the route server's. A route that carries the request is its sender asking for the change, so the prepend is applied only to such a route, and only toward the client it names.

These are control tags, not attributes to pass on. The route server strips every one of them from the route before forwarding it, and with them every large community whose first field (the Global Administrator) is its own ASN, whatever the other two fields hold, so a client never sees another client's steering instructions or the route server's own. A route carrying several control communities has all of them removed, not the first one. Communities the route server does not recognise are forwarded untouched and in their original order, so a client's own tagging survives.

`65535:666` is NOT one of these. RFC 7999 BLACKHOLE steers no route-server forwarding and is never stripped, so a client's blackhole request reaches the other clients as the client wrote it. Ze's own answer to a received BLACKHOLE is the per-session `blackhole` container and the ingress `blackhole-propagation` leaf, both above.

```
# Client tags a route: keep it away from AS 64998, and let AS 65002 have it.
# The route server removes both tags; 65001:100 reaches AS 65002 unchanged.
community [ 0:64998 65000:65002 65001:100 ]
```

A control community decides about a ROUTE, not about a MESSAGE. One UPDATE can carry withdrawn routes and an announcement together. The withdrawn routes carry no attribute and were tagged by nobody, so a client the announcement excludes still receives the withdrawals in that UPDATE. Without that, an excluded client would keep a prefix an earlier UPDATE did advertise to it, and ze could not take it back until the session reset.

Two limits are worth knowing:

- A standard community's high half is 16 bits (RFC 1997), so a route server with a 4-octet ASN matches `<rs-asn>:<asn>` on the low 16 bits of its ASN only. Large communities match the full ASN, but only the prepend requests have a large form today: there is no large-community spelling of `0:<asn>`, `<rs-asn>:<asn>` or `<rs-asn>:0`.
- Stripping is ze's own forwarding behaviour. RFC 7947 expects per-client import and export policy but prescribes no mechanism for it (Section 2.3.2.1 offers per-client RIBs as "the most portable method"), so a different route-server implementation may leave these tags in place.

Behaviour change: earlier versions of ze read the prepend requests and then ignored them, and forwarded the route server's large communities to every client. A route that carries a prepend request now has the prepend applied, which changes path selection at the client it names, and a client policy that matched the route server's large communities no longer sees them.

`ze_bgp_attr_mod_remove_buffer_refused_total{attribute}` counts removal buffers a
handler refused because they were not a whole number of wire values. The refusal
is silent on the wire, so a non-zero value is the one signal that a producer is
violating the arity contract and that an attribute went out unchanged.

<!-- source: internal/component/bgp/wireu/community.go — ParseCommunityPolicy, ShouldForwardTo, StripControlCommunities, StripLargeControlCommunities, RSPrepend -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go — general forwarding rail; forward_rs.go — route-server fast path -->

## Redistribution

The top-level `redistribute` block moves selected routes between protocol RIBs. Redistribution is separate from ordinary per-peer BGP filtering, but the resulting routes still pass through the destination protocol's policy and loop-prevention rules.

Use named destinations and specify the source protocols deliberately. Avoid broad two-way redistribution unless the topology and loop controls have been proven. Connected, static, kernel, OSPF, IS-IS, and BGP route sources have different ownership and withdrawal behaviour.

See [Route filters and redistribution](redistribution.md) for the current chain rules and configuration syntax.

## Verify policy

`show policy chain peer <selector> [import|export]` reports each effective direction as an object containing
`filters` and `final`, including a configured direction with no filters.
Peer detail reports the same policy under `import-policy` and `export-policy`.
Automatic-only filters have no configured `final`; an entirely absent policy
is omitted. These status objects describe configured presence, not the default.
Policy dry-run applies the live session-aware final: absence rejects on EBGP and
accepts on IBGP. With a rejecting final, a pure withdrawal remains `accept` with unchanged resulting
text. A mixed announcement and withdrawal reports `modify`, with only its explicit
withdrawals in the resulting text. A filter execution failure or teardown still
reports `reject` without resulting text; neither is a successful withdrawal.

The dry-run covers the per-peer external-policy stage, not the full ingress
protocol pipeline. An active named in-process check appears in `trace` with
`action: not-evaluated` and empty `text-after`; it is not an accepting filter
invocation. The overall `action` describes the external-policy stage and its
final only, not whether that unevaluated guard would pass on a live session.
A single-filter override selecting an in-process check is refused with an
explanation; exercise it with an UPDATE on the peer session instead. Inactive
references do not execute and have no trace entry.
When the remote AS is unresolved, the overall action is `undetermined` and
`reason` tells you to wait for establishment and repeat `show policy test`.
No filter executes, including a single-filter override. Trace and text are empty.
<!-- source: internal/component/bgp/reactor/policy_dryrun.go -- PolicyDryRun -->

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
