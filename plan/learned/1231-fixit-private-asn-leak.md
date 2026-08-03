# 1231 -- fixit-private-asn-leak

## Context

A configured `export [ remove-private-as:STRIP | replace-with peer-as ]` policy did not
strip/replace the private ASN before an EBGP advertisement of a route ORIGINATED via the
API/plugin path, leaking a private ASN on the wire in violation of RFC 6996 S4 (MUST).
The forwarded egress path was correct; the originated path had drifted. The goal was to make
the operator's export policy actually fire on originated routes, and to remove the structural
cause of the drift so it cannot recur. Root cause was two independent fail-open defects in
`exportFilterForBody`, both instances of "a permissive default standing in for an answer
nobody asked for" (the zero-value trap, `ai/rules/evidence.md`).

## Decisions

- **Collapse, don't patch.** `exportFilterForBody` now delegates to the same shared body as
  the forwarded path (`runEgressPolicyChainASN4`), chosen over patching the originated copy:
  a patch would have fixed remove-private-as only and left every other text filter broken on
  that path, and left the two copies free to drift again.
- **Parameterize only `asn4`.** It is the sole legitimate difference between the two callers
  (forwarded wires are in the SOURCE peer's encoding; originated bodies are already in the
  DESTINATION's send encoding), so `runEgressPolicyChain` became a thin ctx-resolving wrapper
  over `runEgressPolicyChainASN4(..., asn4)`.
- **Copy the override unconditionally** in `buildModifiedPayload`: the raw branch stages
  through the same `writeBuf` the body may alias, so branching the copy on which path produced
  the slice would be a latent aliasing bug.
- **No unit test at this seam, by design.** `Reactor.api` is a concrete `*pluginserver.Server`,
  not an interface, so the guard is driven from the real entry point via two deterministic
  functional tests (`ai/rules/evidence.md`: drive the guard from the entry point).

## Consequences

- One shared chain body means a future outcome added to the chain is honored by BOTH egress
  paths automatically. This is the mechanism the EBGP auto-filters design spends: prepend our
  ASN + strip LOCAL_PREF as auto-added export-chain entries reach the originated path for free.
- Three items were surfaced and deliberately homed to child specs, NOT fixed here:
  - `r.api == nil` silent fail-open (R1) -> `spec-fixit-private-asn-leak-deferred-nil-api-fail-open`
    (trace reachability first, then fix vs fail-closed).
  - EBGP prepend-on-originate + LOCAL_PREF strip (auto-added filters) ->
    `spec-fixit-private-asn-leak-deferred-ebgp-auto-filters`.
  - A live RFC 4271 S5.1.5 violation (LOCAL_PREF survives IBGP-source -> EBGP-destination
    forwarding; no strip on the forwarded egress path) -> AC-1 of the auto-filters child.

## Gotchas

- Fixing the dropped-text-delta defect ALONE did not stop the leak: a second fail-open
  (ctxID 0, so the filter was handed a text with no attributes and truthfully answered "no
  change") sat in front of it. The measurement contradicted the one-defect story; the story
  lost. Always re-measure after a "complete" fix.
- The reproducer is DETERMINISTIC (100% leak pre-fix, 100% green post-fix), not the 3/18 race
  the originating investigation measured -- that race was in the test TOPOLOGY (which egress
  path a forwarded route took), not in the defect. Drive the leaking path directly.
- Two claims in the spec's own Known Limitations were found WRONG while homing the deferrals
  (the `update text` vs `rib/commit` prepend disagreement; `attributes.ci` is an IBGP baseline,
  not EBGP evidence). Corrected in the child spec so it inherits the correction, not the error.

## Files

- `internal/component/bgp/reactor/egress_inject_filter.go` (delegate to shared chain; pass
  `facts.sendCtxID`, `facts.sendASN4`)
- `internal/component/bgp/reactor/filter_ordered.go` (`runEgressPolicyChain` wrapper +
  shared `runEgressPolicyChainASN4`)
- `test/plugin/remove-private-as-export-originated.ci`, `test/plugin/remove-private-as-replace-originated.ci` (new)
