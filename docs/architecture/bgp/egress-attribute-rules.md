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

## The third trigger, and why it is not on this rail

RFC 4271 Section 5.1.4 asks for one more thing: "A BGP speaker MUST implement a
mechanism (based on local configuration) that allows the MULTI_EXIT_DISC
attribute to be removed from a route." Public `del { med; }` in a modify policy
(`docs/guide/plugins.md`) requests this removal. The plugin converts it to the
private `med-remove` runtime delta token. The reactor converts that token to the
same `filterapi.AttrModSuppress` on attribute 4 that `applyFactsMED` emits. One
suppression has two triggers: a rule ze derives, and a removal an operator requests.

It does not run on this rail. `ExtractMEDRemoveOps`
(`internal/component/bgp/reactor/filter_delta.go`) is called from the INGRESS
site alone (`filter_ordered.go`), because the same section requires the removal
to happen "prior to determining the degree of preference of the route and prior
to performing route selection (Decision Process phases 1 and 2)". The import
chain's rewritten payload replaces the WireUpdate before the UPDATE is cached
and dispatched (`reactor_notify.go`), so the route reaches the RIB plugin with
no metric at all, and every egress rail then has nothing to suppress.

An export-side removal would be a different defect rather than a second option.
RFC 4271 Section 9.1.2.2: "Including the MULTI_EXIT_DISC of an EBGP-learned
route in the comparison with an IBGP-learned route, then removing the
MULTI_EXIT_DISC attribute, and advertising the route has been proven to cause
route loops."

## How the Section 9.1.2.2 boundary is enforced

The same paragraph states an obligation: "If an implementation chooses to remove
MULTI_EXIT_DISC, then the optional comparison on MULTI_EXIT_DISC, if performed,
MUST be performed only among EBGP-learned routes." Ze does not restrict its
comparison, so removal must not occur after selection before an IBGP
readvertisement.

The public `del { med; }` mechanism satisfies Section 5.1.4 by running on the
import chain only. `ExtractMEDRemoveOps` converts that directive at ingress,
`appendMEDRemove` (`internal/component/bgp/plugins/filter_modify/filter_modify.go`)
refuses it on export and logs the reason, and `medPropagationAllowedTo`
(`internal/component/bgp/reactor/forward_med.go`) returns true for every non-eBGP
destination. That path keeps the value used for comparison aligned with the value
advertised to an IBGP peer.

The lower raw egress replacement path needs the same boundary. A policy handler
can return a full UPDATE body on the common IBGP egress rail. `applyFactsMED`
receives the source payload and the post-policy destination base. For an IBGP
destination, it records a MED Set when the source carries MED and the base omits
it. The function records no operation if the base already carries MED, the source
had no MED, or policy already recorded a MED Set.

**No later RFC changed the clause.** Of RFC 4271's twelve updaters, ten never
mention MULTI_EXIT_DISC. RFC 7606 Section 7.4 governs a malformed length. RFC
6793 Section 7 adds an AS_TRANS warning. No erratum touches Section 9.1.2.2.

**The invariant is MED presence at IBGP readvertisement.** A selected route that
used MED in comparison must keep that MED before an IBGP advertisement. The only
exception is a candidate set whose comparison is restricted to EBGP-learned
routes.

### What FRR and BIRD do, and why it corroborates

The reading above was derived from the RFC text with no implementation open, then
checked against FRR 10.5.3 and BIRD 2.19.0. The order was deliberate: a reading
assembled from what daemons happen to do is a description of the installed base,
not of the obligation.

| | A removal driven by configuration | Where it runs | Comparison restricted when a removal is configured |
|---|---|---|---|
| FRR | None. `route_set_metric` (`bgpd/bgp_routemap.c`) only overwrites | The one place it clears the bit is `subgroup_announce_check` (`bgpd/bgp_route.c`), the Section 5.1.4 eBGP strip, gated on `peer->sort == BGP_PEER_EBGP` | No. `bgp_path_info_cmp` gates on AS_PATH shape (`aspath_cmp_left`, plus `always-compare-med`), never on peer sort or removal state |
| BIRD | Yes. `med` is a named filter attribute, so `unset(bgp_med)` is a real removal | Import AND export filters, plus the Section 5.1.4 strip in `bgp_update_attrs` (`proto/bgp/attrs.c`) | No. `bgp_rte_better` gates on `bgp_get_neighbor` equality or `med_metric`, never on removal state |

Neither daemon carries a comment, a log line or a documentation sentence
reproducing this loop. Their loop warnings are about MED oscillation, which is
order-dependence in the decision process and a different failure.

So the two implementations sit on opposite sides of the fault line the text
predicted, and neither guards it. FRR escapes the obligation because it does not
offer the feature. That is a consequence of its feature set, not a conformance
decision. BIRD offers the removal on the export side. That side reopens the
hazard, and BIRD leaves its comparison unrestricted.

That is the corroboration, and it is not agreement. Neither daemon reasoned about
the clause. They show that placement is the whole question. An implementation
that can place removal after selection owes the restricted comparison until that
path is removed, guarded, or proven unreachable.

## Proof

| Rule | Unit | Functional | Interop |
|------|------|-----------|---------|
| LOCAL_PREF | `forward_local_pref_test.go` | `test/plugin/local-pref-strip-ebgp.ci` | `test/interop/scenarios/bgp-local-pref-strip-gobgp/` |
| MULTI_EXIT_DISC, the propagation rule | `forward_med_test.go` | `test/plugin/med-not-propagated-across-as.ci`, `test/plugin/med-locally-set-reaches-peer.ci` | `test/interop/scenarios/bgp-med-across-as-gobgp/` |
| MULTI_EXIT_DISC, the configured removal | `forward_med_test.go`, `filter_delta_test.go`, `filter_modify/modify_test.go` | `test/plugin/med-removal-configured.ci`, `test/plugin/med-removal-before-decision.ci`, `test/plugin/med-removal-export-refused.ci` | `test/interop/scenarios/bgp-med-remove-configured-gobgp/` |
| MULTI_EXIT_DISC, IBGP post-selection preservation | `forward_med_test.go` | `test/plugin/med-ibgp-post-selection-removal.ci` | `test/interop/scenarios/bgp-med-ibgp-post-selection-removal-gobgp/` |

Each functional test carries its confining negative in the same run: one
UPDATE, several destinations, opposite transforms, so neither half can pass by
accident. The interop scenarios carry the same pair against a foreign RIB.
Scenario 60: the relayed metric is gone from GoBGP's view and ze's own metric is
in it. Scenario 61: the destination is INTERNAL, so the automatic rule never
fires there, and of two routes on one session the one the policy names arrives
with no metric while the one it does not keeps 100. Scenario 62: a raw export
filter removes MED from the destination base on the route-server egress rail,
and GoBGP still receives MED.

The configured removal's gate reads the WIRE rather than the filter text.
`appendSingleAttr` (`internal/component/bgp/reactor/filter_format.go`) switches
on `*attribute.MED` while the parser builds the value form `attribute.MED`, so
`med` never reaches the text a filter is handed. `medRemoveHasWork`
(`internal/component/bgp/reactor/filter_delta.go`) is where that is answered, and
scenario 61 is what measured it.

<!-- source: internal/component/bgp/reactor/filter_delta.go -- medRemoveHasWork, ExtractMEDRemoveOps -->
<!-- source: internal/component/bgp/reactor/filter_chain.go -- filterAttrs.merge, the chain order between med and med-remove -->
<!-- source: internal/component/bgp/reactor/forward_med.go -- medPropagationAllowedTo, the internal-destination exemption Section 9.1.2.2 rests on -->
<!-- source: internal/component/bgp/plugins/filter_modify/filter_modify.go -- appendMEDRemove, the export-chain refusal -->
