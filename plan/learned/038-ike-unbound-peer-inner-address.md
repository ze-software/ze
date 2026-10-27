# Learned: two peers never hold one inner address unless the operator ordered them

`spec-ike-unbound-peer-inner-address` closed the RFC 3948 Section 5.1 gap
for IKE peers bound to no virtual-IP pool. An engine-wide inner-selector
registry (`innerSelectorRegistry`, `internal/component/ike/engine/inner_selectors.go`)
records every Child SA's installed policy pairs by owner. The narrowing filter
(`dropForeignInnerOverlaps`) drops a peer-chosen pair that one packet could
match together with another peer's live pair, and `installChildSA` repeats the
check before any dataplane call. The reference pages are
`docs/architecture/ike/ipsec-7-ikev2-engine.md` and
`docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`.

## What outlives the spec

1. **Judge a selector as the policy the kernel installs.** The installed pair
   is the intersection of both halves, protocol and ports included
   (`childPolicyPair`). Rounds 6 and 7 found two exemptions judged on the
   remote half alone, so a peer narrowing Ze's local half or one port passed
   as operator-configured (fcf2e39dc8, b99c6503a4).
2. **An operator overlap is ordered only by distinct priorities.** At equal
   `policy-priority` the kernel orders by install time, so a packet's tunnel
   depends on which peer came up first: the newcomer is refused
   (`operatorOrdered`, f87c1883ac).
3. **A record outlives a failed removal.** Releasing the record while the
   kernel still holds the policy reopens the hazard; the record stays stranded
   until a retry removes the policy (`retryStranded`, 3c8b9e42de, dd20e0aa1a).
4. **A red proof must defeat only the guard under test.** The first interop
   red used identical selectors, so the dataplane's exact-key
   `policyOwners.claim` refused the second client and the guard was never
   exercised; the scenario now overlaps without equality (aa18d486d6).
