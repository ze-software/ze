# Egress attribute rules: two prohibitions, opposite precedence

Two RFC 4271 sections, one apart, constrain what an UPDATE carries when ze
relays it. They share one mechanism and they answer an operator filter in
opposite ways. Reading either as the template for the other produces a defect
that no compiler and no type sees.

<!-- source: internal/component/bgp/reactor/forward_local_pref.go -- Section 5.1.5, LOCAL_PREF -->
<!-- source: internal/component/bgp/reactor/forward_med.go -- Section 5.1.4, MULTI_EXIT_DISC -->

## The two rules

| | LOCAL_PREF (Section 5.1.5) | MULTI_EXIT_DISC (Section 5.1.4) |
|---|---|---|
| What the RFC forbids | including the attribute in an UPDATE sent to an external peer | propagating a metric RECEIVED from one neighboring AS to another one |
| What decides | the destination alone | the value's provenance AND the destination |
| An egress filter that SETS it | loses. The Suppress is recorded last and `filterapi.LastSetOrSuppress` makes it win | wins. The suppression is not recorded at all |
| Exemption | RFC 3065 confederations, which ze cannot configure, so the prohibition is absolute | RFC 7947 Section 2.2.3, a route server client, gated on `PeerSettings.RSClient` |
| Predicate | `localPrefAllowedTo` | `medPropagationAllowedTo` |

## Why the precedence differs

Section 5.1.5 prohibits the attribute. A policy that puts LOCAL_PREF on an
external peer's wire is asking for a thing the RFC refuses, so the prohibition
is not a policy the operator may override.

Section 5.1.4 prohibits relaying somebody else's metric. A metric ze sets
toward a peer is what MULTI_EXIT_DISC is for, and an operator who steers
inbound traffic by advertising different metrics to two providers is using the
attribute as designed. So the rule targets the RECEIVED value and leaves an
originated one alone. A blanket suppression of attribute 4 on external egress
would satisfy the letter of Section 5.1.4 and delete the feature.

## How provenance is established

Both forward rails relay an UPDATE another speaker sent, so every byte of the
source payload came off that speaker's wire: a MULTI_EXIT_DISC in it is a
received value by construction. A metric ze originates reaches a peer by two
other routes, and both are visible at the egress transform:

- the announce rails (`writeAnnounceUpdate`, `buildRIBRouteUpdate`), which
  encode a route ze produced and never call the forward transform at all.
- an egress filter, either as a Set operation on the destination's accumulator,
  or as the export policy chain's whole-payload override. The override carries
  no provenance, so the base is compared against the source: a metric that
  differs from the one received was written here.

A policy that rewrites the metric to the value it already held is
indistinguishable on the wire from the propagation Section 5.1.4 forbids, and
the MUST NOT decides that tie.

## The cost constraint both rules obey

The presence question is asked ONCE per UPDATE, not once per destination.
Recording the operation unconditionally would be simpler and would put every
route to every external peer on the payload-rebuild path, which is the cost the
route-server fast path exists to avoid. Both predicates also read the attribute
SECTION rather than the payload bytes, so a prefix holding the byte 0x04 or
0x05 in its NLRI is never mistaken for an attribute.

## Proof

| Rule | Unit | Functional |
|------|------|-----------|
| LOCAL_PREF | `forward_local_pref_test.go` | `test/plugin/local-pref-strip-ebgp.ci` |
| MULTI_EXIT_DISC | `forward_med_test.go` | `test/plugin/med-not-propagated-across-as.ci`, `test/plugin/med-locally-set-reaches-peer.ci` |

Each functional test carries its confining negative in the same run: one
UPDATE, several destinations, opposite transforms, so neither half can pass by
accident.
