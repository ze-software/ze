# Egress attribute rules: provenance and final wire admission

Two RFC 4271 sections, one apart, constrain what an UPDATE carries when ze
relays it. They share one mechanism and they answer an operator filter in
opposite ways. Reading either as the template for the other produces a defect
that no compiler and no type sees.

<!-- source: internal/component/bgp/reactor/forward_local_pref.go -- Section 5.1.5, LOCAL_PREF -->
<!-- source: internal/component/bgp/reactor/forward_med.go -- Section 5.1.4, MULTI_EXIT_DISC -->

RFC 7607 Section 2 adds an unconditional final-wire prohibition: no originated
or propagated route may carry AS zero in AS_PATH, AS4_PATH, AGGREGATOR or
AS4_AGGREGATOR. `Session.writeUpdateBody` applies it after export transformation
on ordinary origination, explicit raw injection and both forwarding rails.
`transmittedASZeroAttribute` visits every attribute occurrence at the recipient's
negotiated width, sharing the receive validators' ASN walkers while leaving
NLRI and internal unknown-AS state untouched.

An offending announcement leaves as a withdrawal through the existing family
synthesizer; original withdrawals survive too. The ordinary ownership filter
prevents a withdrawal for a route the peer never held, while explicit raw writes
retain their raw ownership semantics. A forwarded replacement first projects
the original section identities into withdrawal spans. This keeps the source's
identity and the recipient's assigned ADD-PATH identifier distinct, including
when export policy replaced the complete payload or sizing splits the output.
The projection lasts for that write; the original manifest is restored on
success and failure. The refusal appears in `ze show doctor` and `ze show errors`
under `doctor-bgp-as-zero-outbound`, rate limited per session. Remove AS zero
from the route source, plugin or export policy, then send the corrected route.
This outbound admission does not change
the receive-side distinction: RFC 7606 treats malformed AS_PATH as withdrawn
and discards AGGREGATOR, while RFC 6793 discards malformed AS4 attributes.
The final-socket matrix in `rfc7607_egress_test.go` pairs each zero case with a
real-AS case that must still reach the recipient.
<!-- source: internal/component/bgp/reactor/session_rfc7607.go -- transmittedASZeroAttribute, writeASZeroAsWithdrawal -->
<!-- source: internal/component/bgp/reactor/forward_provenance_withdraw.go -- projectFwdWithdrawals, bindWithdrawalChunk -->
<!-- source: internal/component/bgp/message/rfc7607.go -- AttributeHasASZero -->

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

Section 5.1.5 also states the other side: LOCAL_PREF "SHALL be included in all
UPDATE messages that a given BGP speaker sends to other internal peers". A route
learned from an external peer arrives with no LOCAL_PREF, so the forward rails
add LOCAL_PREF 100 toward an internal peer, the same value the announce rail
writes (`planBatchAttrs`). A received value is kept, and so is a value that an
egress filter set. `applyFactsLocalPref` records both operations.

Section 5.1.4 prohibits relaying somebody else's metric. A metric ze sets
toward a peer is what MULTI_EXIT_DISC is for, and an operator who steers
inbound traffic by advertising different metrics to two providers is using the
attribute as designed. So the rule targets the RECEIVED value and leaves an
originated one alone. A blanket suppression of attribute 4 on external egress
would satisfy the letter of Section 5.1.4 and delete the feature.

## How provenance is established

Both forward rails relay an UPDATE another speaker sent, after import policy
has had a chance to rewrite it. Their source payload is that post-import
admitted body, not necessarily the exact received bytes. A MULTI_EXIT_DISC in
it is the received forwarding baseline for this egress comparison.
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyOneMessage -->
<!-- source: internal/component/bgp/reactor/forward_med.go -- applyFactsMED -->

A metric ze originates reaches a peer by two other routes, and both are visible
at the egress transform:

- the announce rails (`writeAnnounceUpdate`, `buildRIBRouteUpdate`), which
  encode a route ze produced and never call the forward transform at all.
- an egress filter, either as a Set operation on the destination's accumulator,
  or as the export policy chain's whole-payload override. The override carries
  no provenance, so the base is compared against the source: a metric that
  differs from the one received was written here.

A policy that rewrites the metric to the value it already held is
indistinguishable on the wire from the propagation Section 5.1.4 forbids, and
the MUST NOT decides that tie.

## ATOMIC_AGGREGATE policy admission

RFC 4271 Section 5.1.6 binds the marked received route even if a filter removes
its output marker. Ze therefore restricts modifications of marked message-policy
inputs on both import and export. If output still announces routes, it must
retain a valid ATOMIC_AGGREGATE and announce only an exact native-key subset
of its original chain input. Attribute changes, reordering and route removal
remain supported. Pure withdrawal and EOR output need no marker.
<!-- source: internal/component/bgp/reactor/filter_library.go -- runLibraryChain -->
<!-- source: internal/component/bgp/reactor/filter_atomic_aggregate.go -- atomicAggregatePolicyAllowed -->

The RFC assigns SHOULD NOT to marker removal and MUST NOT to making NLRI more
specific. This release boundary is stronger than those requirements. New marked native
keys are unavailable even when a raw rewrite is less specific or disjoint.
That optional facility would require an explicit relationship between raw
output routes and their received sources. A raw UPDATE supplies none.
<!-- source: internal/component/bgp/reactor/filter_library.go -- runLibraryChain -->
<!-- source: internal/component/bgp/reactor/filter_atomic_aggregate.go -- atomicAggregatePolicyAllowed -->
<!-- source: rfc/full/rfc4271.txt -- Section 5.1.6 -->

The restriction preserves the received condition in existing cache/store/replay
bytes, so no extra source envelope or parallel route store is needed. An
originally unmarked chain input retains its rewrite behavior, including when
the result adds ATOMIC_AGGREGATE.
<!-- source: internal/component/bgp/reactor/filter_library.go -- runLibraryChain -->
<!-- source: internal/component/bgp/reactor/filter_atomic_aggregate.go -- atomicAggregatePolicyAllowed -->
<!-- source: rfc/full/rfc4271.txt -- Section 5.1.6 -->

`runLibraryChain` checks every accepting modified result, raw and typed alike.
Final-judge and stale passes use `intent.chainInput` as the original input.
The helper uses registered native framing and keys, with per-family ADD-PATH
identity. It does not treat labels or native length fields as CIDR bits. Later
encoding and splitting preserve the admitted destinations or withdraw them.
<!-- source: internal/component/bgp/reactor/filter_library.go -- runLibraryChain, runFinalJudges -->
<!-- source: internal/component/bgp/reactor/stale_readvertise_chain.go -- runStaleChain -->
<!-- source: internal/component/bgp/reactor/filter_atomic_aggregate.go -- atomicAggregatePolicyAllowed -->
<!-- source: internal/component/bgp/reactor/forward_body.go -- buildFwdBody, fwdReencodeNLRIs -->

A refusal becomes `OutcomeWithdraw` for the original source routes. It does
not withdraw the rejected output's new prefixes. Import publishes source
removals before the effective replacement. Both forwarding rails use existing
recipient ownership admission to preserve foreign owners and omit withdrawals
for paths the recipient never held.
<!-- source: internal/component/bgp/reactor/filter_ordered.go -- runIngressPolicyChainMeta, runEgressPolicyChainASN4, ingressChainWithdrawal -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- forwardUpdateSection -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRSSection -->
<!-- source: internal/component/bgp/reactor/session_ownership.go -- allow -->

The refusal is a
bounded historical event under `doctor-bgp-atomic-aggregate-policy`, visible in
`ze show doctor` and `ze show errors`. Edit the offending filter so it keeps
the marker and only original destinations. Commit the correction with
the configuration editor (`ze config edit`). A repaired route leaves no active
warning. The bounded event history records what happened.
<!-- source: internal/component/bgp/reactor/filter_atomic_diagnostic.go -- reportAtomicAggregatePolicyRefusal -->

An accepted marked subset must also retire source keys the final output leaves
out. The difference includes original explicit withdrawals omitted by a raw
answer, and excludes keys the final answer announces or withdraws. It is
computed against the original chain input after each effective rewrite. A
final pass that changes nothing retains the preceding result and its removals.
The owned removal result can contain two bodies when the source's MP_REACH
and MP_UNREACH name different families. Each body uses the registered native
withdrawal writer and original received Path Identifier.
<!-- source: internal/component/bgp/reactor/filter_atomic_subset.go -- atomicAggregatePolicyRemovals -->
<!-- source: internal/component/bgp/reactor/filter_library.go -- runLibraryChain, runFinalJudges -->
<!-- source: internal/component/bgp/reactor/stale_readvertise_chain.go -- runStaleChain -->

Forwarding keeps those removal bodies under the same queued item and write
fence as the retained output, with separate source ownership manifests. The
children add no destination count. Source-removal admission precedes
announcements or EOR. Immediate withdrawals retain that wire order.
<!-- source: internal/component/bgp/reactor/forward_policy_removal.go -- prepareFwdPolicyRemovals, writePolicyRemovals -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- fwdBatchHandler, releaseItem -->
<!-- source: internal/component/bgp/reactor/forward_rs_direct.go -- tryDirectWriteNoFlush -->

If a RIB recovery owner receives the source, a non-ADD-PATH destination can
instead owe a later route decision. That decision withdraws the key or installs
an export-permitted replacement. Kept output and unrelated prefixes do not wait
for that decision.
<!-- source: internal/component/bgp/reactor/forward_policy_removal.go -- prepareFwdPolicyRemovals, writePolicyRemovals -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- fwdBatchHandler, releaseItem -->
<!-- source: internal/component/bgp/reactor/forward_rs_direct.go -- tryDirectWriteNoFlush -->
<!-- source: internal/component/bgp/reactor/session_ownership.go -- owesWithdrawal -->
<!-- source: internal/component/bgp/reactor/recovery_owed.go -- recoveryOwedFor, run, decide -->

Import publishes owned, already-judged continuations in that order with monotonic
message IDs and the original admission epoch and metadata. Only the effective
replacement takes the original receive buffer, once. Derived bodies are not
physical receipts and do not increment the peer's receive or EOR counters.
<!-- source: internal/component/bgp/reactor/forward_policy_removal.go -- prepareFwdPolicyRemovals, writePolicyRemovals -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- fwdBatchHandler, releaseItem -->
<!-- source: internal/component/bgp/reactor/forward_rs_direct.go -- tryDirectWriteNoFlush -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyMessageReceiver, notifyOneMessage -->
<!-- source: internal/component/bgp/reactor/session_ownership.go -- owesWithdrawal -->
<!-- source: internal/component/bgp/reactor/recovery_owed.go -- recoveryOwedFor, run, decide -->

Accepted received-source retirements are staged before survivor construction.
If later AS-path resolution, attribute rebuilding or destination-size encoding
fails, the same pending item submits only the owed source removals.
Those removals pass through the same ownership and recovery admission. The failed
survivor remains a forwarding failure and adds no successful destination count.
<!-- source: internal/component/bgp/reactor/forward_policy_removal.go -- prepareFwdPolicyRemovals -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- forwardUpdateSection -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRSSection -->

Before materialization, removal sections are projected onto actual negotiated
families, preserving eligible legacy bytes beside an omitted MP sibling.
An unsupported indivisible native key
cannot suppress a useful supported route, and an empty projection emits no
generic End-of-RIB. Final source and exact-receipt admission still applies.
<!-- source: internal/component/bgp/reactor/forward_policy_removal.go -- prepareFwdPolicyRemovals, projectFwdPolicyRemoval -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- forwardUpdateSection -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRSSection -->

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
(`docs/guide/plugins.md`) requests this removal. The plugin answers it as a
`med` remove in its sets, and `Sets.ApplyTo`
(`internal/component/bgp/policychain/sets.go`) writes the same
`filterapi.AttrModSuppress` on attribute 4 that `applyFactsMED` emits. One
suppression has two triggers: a rule ze derives, and a removal an operator requests.

It does not run on this rail. `removeMED`
(`internal/component/bgp/policychain/external_attributes.go`) honors it on an
import chain alone, because the same section requires the removal
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
import chain only. `bgp-filter-modify` answers the removal as a `med` remove
in its Sets on an import chain, and `removeMED`
(`internal/component/bgp/policychain/external_attributes.go`) applies it there.
On an export chain `medRemoveFor`
(`internal/component/bgp/plugins/filter_modify/filter_modify.go`) withholds the
removal and logs a warning, and `removeMED` fails an export route whose filter
answers one. `medPropagationAllowedTo`
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
| LOCAL_PREF | `rfc4271_forward_local_pref_test.go` | `test/plugin/local-pref-strip-ebgp.ci` | `test/interop/scenarios/bgp-local-pref-strip-gobgp/` |
| MULTI_EXIT_DISC, the propagation rule | `forward_med_test.go` | `test/plugin/med-not-propagated-across-as.ci`, `test/plugin/med-locally-set-reaches-peer.ci` | `test/interop/scenarios/bgp-med-across-as-gobgp/` |
| MULTI_EXIT_DISC, the configured removal | `forward_med_test.go`, `filter_modify/rfc4271_modify_test.go` | `test/plugin/med-removal-configured.ci`, `test/plugin/med-removal-before-decision.ci`, `test/plugin/med-removal-export-refused.ci` | `test/interop/scenarios/bgp-med-remove-configured-gobgp/` |
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

The configured removal has work only when the route carries a metric, from the
peer or from an earlier member of the chain: `Chain.overlayMEDRemove` forgets a
removal from a route with no MULTI_EXIT_DISC, so `Sets.ApplyTo` writes the
suppression only where there is one to remove.

<!-- source: internal/component/bgp/policychain/chain.go -- overlayMEDRemove -->
<!-- source: internal/component/bgp/policychain/sets.go -- Sets.ApplyTo, the med suppression -->
<!-- source: internal/component/bgp/reactor/forward_med.go -- medPropagationAllowedTo, the internal-destination exemption Section 9.1.2.2 rests on -->
<!-- source: internal/component/bgp/plugins/filter_modify/filter_modify.go -- medRemoveFor, the export-chain warning -->
<!-- source: internal/component/bgp/policychain/external_attributes.go -- removeMED, the export-chain refusal -->
