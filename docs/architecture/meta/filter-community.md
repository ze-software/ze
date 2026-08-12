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
RFC 7947 requires route-server transparency, and a route server is not in the
customer/peer/provider lattice RFC 8195 Section 3.2 describes.

<!-- source: internal/component/bgp/plugins/filter_community/relation.go -- relationParameterFor, relationPeerRoleFromMeta -->

## Ordering

Ordering is declared, not implied by code position. The plugin registers TWO
ingress filters:

| Filter | Stage | Priority | Work |
|--------|-------|----------|------|
| `bgp-filter-community` | `FilterStagePolicy` (100) | 0 | named `strip` sets, RFC 7454 Section 11 scrub, RFC 7999 Section 3.2 blackhole guard, named `tag` sets |
| `bgp-filter-community-relation` | `FilterStageAnnotation` (200) | 1 | RFC 8195 Section 3.2 relation tag |

Two properties follow from the table and neither is a convention:

- **Scrub before tag.** The policy stage sorts before the annotation stage, so
  Section 11 scrub always runs over a payload the relation tag has not
  touched. Reversing them would delete the value Ze just derived, because the
  scrub's keep-list never keeps the relation function.
- **Role before relation.** `bgp-role` registers at the annotation stage with
  priority 0, so it writes `src-peer-role` before priority 1 reads it. A single
  filter at the policy stage would read a key nothing had written yet.

<!-- source: internal/component/bgp/plugins/filter_community/register.go -- both filterapi.Register calls -->
<!-- source: internal/component/bgp/filterapi/filterapi.go -- LessOrder, the stage constants -->

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

<!-- source: internal/component/bgp/plugins/filter_community/filter.go -- applyRelationTag, applyIngressFilter -->
<!-- source: internal/component/bgp/plugins/filter_community/blackhole.go -- blackholePropagationGuard -->
