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

## Why the Section 9.1.2.2 restricted comparison does not bind ze

The same paragraph states an obligation: "If an implementation chooses to remove
MULTI_EXIT_DISC, then the optional comparison on MULTI_EXIT_DISC, if performed,
MUST be performed only among EBGP-learned routes." Ze offers a removal and does
not restrict its comparison, so the sentence needs an answer rather than a shrug.

**Read the paragraph from its first sentence.** It opens with its own antecedent:
"If a MULTI_EXIT_DISC attribute is removed before re-advertising a route into
IBGP, then comparison based on the received EBGP MULTI_EXIT_DISC attribute MAY
still be performed." Every later sentence refers to THAT removal, the one that
takes the value off the wire while the speaker keeps it for its own decision.

Three things in the text settle which reading governs.

| Ground | Text |
|--------|------|
| Section 5.1.4 already fixes the order for the configured mechanism | "this removal MUST be done prior to determining the degree of preference of the route and prior to performing route selection (Decision Process phases 1 and 2)" |
| Section 5.1.4 makes that mechanism compulsory for every speaker, so a universal restriction would contradict the sentence three lines below the MUST | "For IBGP-learned routes, the MULTI_EXIT_DISC MUST be used in route comparisons that reach this step in the Decision Process" |
| The paragraph blesses the state an ingress removal leaves behind | "The best EBGP-learned route may then be compared with IBGP-learned routes after the removal of the MULTI_EXIT_DISC attribute" |

**Ze cannot execute the sequence the hazard names, and the hazard names it in
order: include in the comparison, then remove, then advertise.** Every removal ze
performs happens before the comparison or not at all. `ExtractMEDRemoveOps`
converts the directive at the ingress site alone, `appendMEDRemove`
(`internal/component/bgp/plugins/filter_modify/filter_modify.go`) refuses it on an
export chain and logs the reason, and `medPropagationAllowedTo`
(`internal/component/bgp/reactor/forward_med.go`) returns true for every non-eBGP
destination, so the automatic strip never touches an internal peer. The announce
rails write a metric ze set and remove none.

**No later RFC changed the clause.** Of RFC 4271's twelve updaters, ten never
mention MULTI_EXIT_DISC. RFC 7606 Section 7.4 governs a malformed length. RFC
6793 Section 7 adds an AS_TRANS warning. No erratum touches Section 9.1.2.2.

**The invariant is what to hold, not the argument.** For every route ze
advertises, the metric used at the comparison step must equal the metric on the
advertised route, two absences counting as equal. An export-side or reflection-
side removal breaks it whatever the reading, and the obligation then binds.

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
predicted, and neither guards it. FRR escapes the obligation by not offering the
feature, which is a consequence of its feature set rather than a conformance
decision. BIRD offers the removal on the export side, which is the side that
reopens the hazard, and leaves its comparison unrestricted.

That is the corroboration, and it is not agreement. Neither daemon reasoned about
the clause; what they show is that placing the removal is the whole question, and
that an implementation which places it on export owes the restricted comparison
and does not pay it. Ze places it on ingress, which is why the compared value and
the advertised value cannot diverge here.

## Proof

| Rule | Unit | Functional | Interop |
|------|------|-----------|---------|
| LOCAL_PREF | `forward_local_pref_test.go` | `test/plugin/local-pref-strip-ebgp.ci` | `test/interop/scenarios/54-local-pref-strip-gobgp/` |
| MULTI_EXIT_DISC, the propagation rule | `forward_med_test.go` | `test/plugin/med-not-propagated-across-as.ci`, `test/plugin/med-locally-set-reaches-peer.ci` | `test/interop/scenarios/60-med-across-as-gobgp/` |
| MULTI_EXIT_DISC, the configured removal | `forward_med_test.go`, `filter_delta_test.go`, `filter_modify/modify_test.go` | `test/plugin/med-removal-configured.ci`, `test/plugin/med-removal-before-decision.ci`, `test/plugin/med-removal-export-refused.ci` | `test/interop/scenarios/61-med-remove-configured-gobgp/` |

Each functional test carries its confining negative in the same run: one
UPDATE, several destinations, opposite transforms, so neither half can pass by
accident. Both interop scenarios carry the same pair against a foreign RIB.
Scenario 60: the relayed metric is gone from GoBGP's view and ze's own metric is
in it. Scenario 61: the destination is INTERNAL, so the automatic rule never
fires there, and of two routes on one session the one the policy names arrives
with no metric while the one it does not keeps 100.

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
