# 1338 - one gate where the handler runs, not one guard per producer

The other four producers of `plan/learned/1336-withdraw-only-relay-shape.md`,
closed 2026-08-04 under `plan/spec-rfc7606-5-1-2-relay-shape.md`. 1336 fixed the
AS_PATH prepend. This fixed everything else that stamps an attribute on a relayed
UPDATE carrying no route.

## The defect

Five per-destination rules add a path attribute during a relay. Each asked about
the DESTINATION. None asked about the UPDATE. So a withdrawal and an RFC 4724
Section 2 End-of-RIB each came out carrying one lone attribute:

| Rule | Producer |
|------|----------|
| next-hop rewrite | `applyFactsNextHop` (`reactor/peer_forward_facts.go`) |
| RFC 4456 reflection | `forwardUpdateCore`, `reactorForwardRS`, landing on `originatorIDHandler` / `clusterListHandler` |
| egress community tag | `filter_community`'s egress step, landing on `genericCommunityHandler` |
| policy-chain text delta | `runIngressPolicyChain` / `runEgressPolicyChain`, landing on `genericAttrSetHandler` |

RFC 4271 Section 6.3 makes that a Missing Well-known Attribute error. Measured
against FRR 10.3.1: with `next-hop self` configured, `0004180a00000000` left ze as
`0004180a00000007400304c0000201`, and the End-of-RIB `00000000` left as
`00000007400304c0000201`.

## The reusable lesson: gate where the thing runs, not where it is asked for

The spec, the review and the handoff all proposed hoisting one `advertises` bit
into `forwardUpdateCore` and threading it to the recording sites. Reading the code
said otherwise, and the four reasons generalize past this bug:

1. **Count the drivers before you pick one.** `buildModifiedPayload` is reached
   from FIVE call sites (`reactor_api_forward.go`, `forward_rs.go`,
   `filter_ordered.go` twice, `reactor_api_batch.go`). A guard at the driver you
   happen to be reading covers a fifth of the surface.
2. **A driver cannot tell CREATE from MODIFY.** Refusing to RECORD the operation
   also cancels the legitimate rewrite of an attribute that rode along on the
   withdrawal, which 1336 was careful to keep. `src == nil` exists only inside
   `planAttr`.
3. **A producer in another package cannot read your flag.** `genericCommunityHandler`
   lives in a plugin. Threading a bit to it means a new public API. Gating where
   the handler is CALLED costs nothing, and it covers handlers nobody has written
   yet.
4. **Ask about the bytes you are WRITING, not the bytes you read.** An export
   chain that denies every prefix passes `nlriOverride = []byte{}`. A predicate
   over the source payload calls that body an advertisement, so the guard must
   see the override too.

The rule: when N producers can reach one sink, put the invariant at the sink. Four
guards is four places to forget the fifth, and the fifth is always the one in
someone else's package.

## Traps

- **Fifteen more fixtures had the code's hole.** Same shape 1336 found in
  `aspath_slot_test.go`: a relayed UPDATE with no NLRI, asserting that an egress
  rule ADDED an attribute to it. They were withdraw-only UPDATEs claiming to be
  advertisements. Each gained a prefix, and no assertion moved.

  `TestProgressiveBuildWithdrawnPreserved` was the exception worth naming. It is a
  real withdrawal whose modification CREATED an OTC attribute, so its correction
  was to rewrite an attribute the fixture already carried. When a fix reddens a
  test, ask whether the fixture ever matched the prose above it.

- **The minimal fixture was a marker.** Four tests in `filter_ordered_test.go`
  drove a missing-handler fail-closed guard over `{0, 0, 0, 0}`, which is a legacy
  End-of-RIB. The gate refused the filter's operation before the guard under test
  was ever reached, so those tests went green for the wrong reason. A minimal
  fixture is not a neutral one.

- **Refusing every operation must return "nothing to apply", not a copy.** Without
  the `!planned` early return the rebuild produced a byte-identical payload. That
  is correct, and it costs the relay its zero-copy path on every withdrawal. The
  mutant is invisible to any assertion that only compares bytes. It takes
  `assert.Nil(result)`.

- **An interop scenario needs a witness that CAN SEE the property.** Scenario 54's
  first draft put FRR on the receiving side and asserted it raised no attribute
  error over the reflected withdrawal. Measured, that mutant SURVIVED. FRR checks
  mandatory attributes only once NEXT_HOP or MP_REACH_NLRI is present, so
  ORIGINATOR_ID and CLUSTER_LIST on a withdrawal draw no complaint from it. The
  rebuild moved FRR to the SOURCE side and made the receiving witness a raw
  `ze-test peer` that asserts the bytes. Scenario 53 keeps FRR as the receiver,
  where a stamped NEXT_HOP does make it speak.

- **A wrong `RFC requirement:` tag is worse than none.** `RFC4271-4.3-1` reads like
  the withdraw-only shape. It is the Transitive-bit rule. Scenario 53 carries no
  tag and says why. RFC 4271 Section 4.3's "will not include path attributes" is
  indicative prose with no checklist row, and Section 6.3's extracted row is a
  RECEIVER obligation the scenario does not drive. Scenario 54 IS tagged, to
  RFC4456-8-1 and RFC4456-8-2. Their "when an RR reflects a route" condition is
  extracted, and it is exactly what the pair proves.

## Files

- `internal/component/bgp/reactor/forward_build.go` -- `advertiseGate`, `planAttr`
- `internal/component/bgp/wireu/advertise.go` -- `PayloadAdvertisesNLRI`, the one definition
- `internal/component/bgp/reactor/forward_build_withdraw_shape_test.go`
- `test/interop/scenarios/53-relay-withdraw-nexthop-self-frr`, `54-relay-withdraw-reflector-frr`
