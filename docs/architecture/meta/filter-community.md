# Community Filter Plugin -- Meta Keys

<!-- source: internal/component/bgp/plugins/filter_community/filter_community.go -- ingressFilter, relationIngressFilter, egressFilter -->

## Keys Set

None. The plugin writes its results into the UPDATE payload, never into route
metadata.

## Keys Read

| Key | Type | Stage | How Used | Description |
|-----|------|-------|----------|-------------|
| `src-peer-role` | `string` | Ingress filter (`bgp-filter-community-relation`) | `relationPeerRoleFromMeta`, then `relationParameterFor` | What the source peer IS to us. Maps to RFC 8195 Section 3.2 parameter: `customer` gives 2, `peer` gives 3, `provider` gives 4. `rs` and `rs-client` give 0, which writes nothing. |

## Absence

An absent key writes no relation tag. So does a key of the wrong type, which
`relationPeerRoleFromMeta` treats as absent per the type contract in
`README.md`. So does any token the mapping does not recognize.

This is the closed branch and it is the whole design of the reader. A guessed
relation would be stored on the route, and every policy keyed on it would then
act on the guess. Three states reach it: the role plugin is not linked, the
peer has neither a role capability nor a role config. The peer is a route
server or a route-server client. The last is a decision rather than a failure:
RFC 7947 recommends route-server transparency (Section 2.2.2.1 says in terms
"This is a recommendation rather than a requirement"), and a route server is not
in the customer/peer/provider lattice RFC 8195 Section 3.2 describes.

<!-- source: internal/component/bgp/plugins/filter_community/relation.go -- relationParameterFor, relationPeerRoleFromMeta -->

## Ordering

Ordering is declared, not implied by code position. The plugin serves four
filters through one evaluator (`evaluateCommunityFilter`), on the typed slot
internal and `filter-update` forked:

| Member | List | Placed by | Work |
|--------|------|-----------|------|
| `bgp-filter-community:ingress` | import | the operator | named `strip` sets, RFC 7454 Section 11 scrub, RFC 7999 Section 3.2 blackhole guard, named `tag` sets |
| `bgp-filter-community:relation` | import | the operator | RFC 8195 Section 3.2 relation tag |
| `bgp-filter-community:egress` | export | the operator | named egress `strip` and `tag` sets, after the RFC 7999 Section 3.1 agreement check |
| `bgp-filter-community:blackhole-agreement` | export | inserted, required, on every eBGP session that is not a route-server client | RFC 7999 Section 3.1 agreement check |

The three operator members run where the operator names them in the peer's
chain. A peer whose community settings no member runs is refused at verify,
naming the member to add (`checkCommunityMembers`). Three properties follow,
and none is a convention:

- **Procedures before policy.** The config loader places every inserted
  member ahead of the operator's, so `bgp-gr`'s `llgr-ingress` marks a route
  carrying LLGR_STALE before an operator strip can remove it, and an operator
  tag of LLGR_STALE marks nothing (RFC 9494 Section 4.3).
- **Scrub before tag.** `ingress` runs before `relation`: the scrub then runs
  over a body the relation tag has not touched. Reversed, it would delete the
  value Ze just derived, because the scrub's keep-list never keeps the
  relation function, so a peer that both scrubs and tags with `relation`
  named first is refused at verify, naming the order to write
  (`checkCommunityMembers`).
- **Scrub before the route server reads.** The whole ingress pass runs before
  the UPDATE is cached, and both route-server forward rails parse the cached
  payload (`wireu.ParseCommunityPolicy`). So on a route-server client's
  session the scrub keeps the values `wireu.IsRSControlCommunity` and
  `wireu.IsRSControlLargeCommunity` match against
  `PeerFilterInfo.RouteServerAS`; scrubbing them would cancel the client's
  request before the route server saw it. The route server strips them itself
  on egress. The reactor fills `RouteServerAS` from the source's
  `GlobalLocalAS`, the AS both forward rails read and strip by, and leaves it
  zero on any other session, which keeps nothing. The scrub's own Global
  Administrator stays the session's `LocalAS`, so with a per-peer local-as
  override a control-shaped value under the override is scrubbed (the route
  server neither acts on nor strips it), and a true control under the global
  AS is outside the scrub's reach. The set kept is the set the route server
  acts on and strips.
- **Role before relation.** `bgp-role` writes `src-peer-role` from its OTC
  procedure, an inserted member, so it runs before the operator's `relation`
  member reads the key.

<!-- source: internal/component/bgp/plugins/filter_community/community_filter.go -- communityFilterDecls, communityInsertions, evaluateCommunityFilter -->
<!-- source: internal/component/bgp/plugins/filter_community/chain_member.go -- checkCommunityMembers -->

## Coupling

One key, read from the role plugin. The coupling is the key spelling and
nothing else: neither plugin imports the other, and the role name tokens are
RFC 9234 Table 1 names that this package spells itself (`relation.go`).
Deleting either plugin leaves the other building.

## Performance

The relation filter costs one map lookup per received UPDATE for a peer that
has no config. One more for a peer whose `relation-tag` leaf is off. A peer
with the tag on pays one wire scan for the de-forge and one attribute rebuild.

The tag is written ONCE, on ingress, per received route. It does not vary per
destination, so it costs the fan-out dedup nothing. A per-destination tag
would give every destination a distinct edit set and defeat that dedup
(`docs/architecture/core-design.md`).

RFC 7999 guard costs nothing at all when its leaf is `none`: the leaf is read
before the payload is, so a peer that did not ask for it pays no wire scan.

The egress filter also refuses one route class, for RFC 7999 Section 3.1: a
route carrying BLACKHOLE towards an eBGP destination that is not a route-server
client and whose `blackhole` container does not list the community
(`blackholeReadvertiseRefused`). It runs before the filter-config lookup, since
a destination with no community policy still owes the agreement, and in the
order cheapest first: the session type (`PeerFilterInfo.IsIBGP`, `RSClient`),
then the 4-octet COMMUNITIES scan, then the lookup. The lookup reads the
`blackholecfg.Parse` map re-keyed once at config time into an address map and a
group-template map, so no string is built per route. The cache is one part of
the plugin's config value (`communityConfig`: definitions, per-peer filter
configs, agreements), which `sdk.ConfigTx` installs at Stage 2 and replaces
on a reload: verify parses, apply swaps all three under `mu`, rollback
restores the previous value. A miss, the empty cache included, means "not
agreed".

<!-- source: internal/component/bgp/plugins/filter_community/blackhole.go -- blackholeReadvertiseRefused, blackholeAgreements -->
<!-- source: internal/component/bgp/plugins/filter_community/filter_community.go -- newCommunityConfigTx, installCommunityConfig, egressFilter -->

<!-- source: internal/component/bgp/plugins/filter_community/filter.go -- applyRelationTag, applyIngressFilter -->
<!-- source: internal/component/bgp/plugins/filter_community/blackhole.go -- blackholePropagationGuard -->
