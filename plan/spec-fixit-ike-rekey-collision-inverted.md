# Spec: fixit-ike-rekey-collision-inverted

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-ike-rekey-collision-inverted.md` |
| Updated | 2026-07-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-30 during phase 2b of `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`. An agent
reported that the repository convention looked inverted against the RFC. The supervising
session then read the RFC text and all four layers below and confirmed it.

**Owner authorization, 2026-07-30.** Thomas ruled "fix it now, in its own fixit spec", and
that ruling is the authority for editing an RFC-tagged test. `ai/rules/testing.md` reserves
that to the user, and the `rfc-tagged-test` hook refuses it without a
`// rfc-test-change-approved:` marker naming what the user approved.

## Task

**Ze resolves a simultaneous rekey collision the wrong way round, and a green enrolled test
certifies the inversion.**

RFC 7296 section 2.8.1 states it twice. The lowest nonce LOSES:

<!-- ste: ignore -->
> If redundant SAs are created through such a collision, the SA created with the lowest of the four nonces used in the two exchanges SHOULD be closed by the endpoint that created it.

<!-- ste: ignore -->
> The new IKE SA containing the lowest nonce SHOULD be deleted by the node that created it, and the other surviving new IKE SA MUST inherit all the Child SAs.

The RFC's own worked example agrees:

<!-- ste: ignore -->
> Suppose that the lowest nonce was Nr1 in message resp2; in this case, B (the sender of req2) deletes the redundant new SA.

Ze says the opposite at every layer:

| Layer | What it says | Where |
|-------|--------------|-------|
| The checklist row | "(lower nonce wins)" | `rfc/short/rfc7296.md`, row `RFC7296-2.8-1` |
| The function doc | "the exchange with the lowest nonce wins" | `internal/component/ike/engine/rekey.go` |
| The caller | logs "we win (lower nonce), ignoring peer request" and keeps our exchange | `internal/component/ike/engine/inbound.go` |
| The tagged test | asserts that the lower local nonce wins | `internal/component/ike/engine/rekey_test.go` |

The comparison itself is correct. `bytes.Compare` is octet-by-octet, which is what the RFC
demands ("Lowest" means an octet-by-octet comparison). Only the direction is wrong.

**Why nobody noticed.** The inversion is symmetric. Two Ze peers therefore agree with each
other, and every test passes. The extraction wrote the gloss "(lower nonce wins)" into the
row. The test was then written from the row, and nobody checked the row against the RFC.
A wrong extraction produced a wrong test that certifies wrong behaviour as conformant.

## The interoperability failure

Against a conforming peer the collision never resolves, in either direction.

| Ze holds | Ze does | The conforming peer does | Result |
|----------|---------|--------------------------|--------|
| the LOWER nonce | keeps its exchange, ignores the peer | keeps its own, because its nonce is higher | BOTH survive. Redundant SAs persist, and nothing deletes them |
| the HIGHER nonce | abandons its own exchange | closes its own, because its nonce is lower | BOTH abandon. No rekey happens at all |

The second case is the more dangerous. A rekey that silently does not happen leaves the SA
to reach its hard lifetime and drop the traffic it carries.

## Required Reading

| Document | Why |
|----------|-----|
| `rfc/full/rfc7296.txt` section 2.8.1 | The two sentences and the worked example, verbatim |
| `ai/rules/testing.md` | RFC-tagged tests: the test IS the requirement, and only the user authorizes a change |
| `ai/rules/rfc-compliance.md` | Conformance is not negotiable |
| `internal/component/ike/engine/rekey.go` | The comparison and its doc |
| `internal/component/ike/engine/inbound.go` | Both callers |

## Current Behavior (MANDATORY)

Source files read on 2026-07-30:

- [ ] `internal/component/ike/engine/rekey.go`
- [ ] `internal/component/ike/engine/inbound.go`
- [ ] `internal/component/ike/engine/rekey_test.go`
- [ ] `rfc/short/rfc7296.md`
- [ ] `rfc/full/rfc7296.txt`

`resolveRekeyCollision(localNonce, remoteNonce)` returns
`bytes.Compare(localNonce, remoteNonce) < 0`, so it is true when OUR nonce is the lower.
Both callers treat true as "we win": the child branch at `inbound.go` and the IKE
branch at `inbound.go`, the second added the same day by
`plan/spec-fixit-ike-negotiation-conformance.md`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

An inbound CREATE_CHILD_SA request that arrives while our own rekey is in flight, reaching
`handleCreateChildSAOwned` in `internal/component/ike/engine/inbound.go`.

### Transformation Path

`nonceFromPayloads` extracts the peer's Ni. `resolveRekeyCollision` compares it with
`p.localNonce`. The boolean decides whether Ze abandons its own exchange or ignores the
peer's request. The loser's exchange is dropped and its request window is freed.

### Boundaries Crossed

| Boundary | Direction | Carries |
|----------|-----------|---------|
| peer -> engine | in | The peer's Ni, inside an encrypted CREATE_CHILD_SA |
| engine -> peer | out | Either our response, or nothing when we abandon |

### Integration Points

`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md` owns rows `RFC7296-2.8-1` and `RFC7296-2.8.2-1`.
Both are already enrolled and tagged, so this spec CHANGES an existing proof rather than
adding one.

## Key Design Decisions

| Decision | Over | Because |
|----------|------|---------|
| Invert the decision at the two call sites, and keep `resolveRekeyCollision` returning "our nonce is lower" | Inverting the function itself | The function name states a comparison, not a verdict. A caller that reads "is ours lower" and then decides is clearer than a helper whose truth value silently means "win" |
| Rename the helper to state the comparison | Leaving the name | `resolveRekeyCollision` promises a verdict. `localNonceIsLower` promises a fact, and a fact cannot be inverted by accident |
| Correct the row text as well as the code | Fixing only the code | The row is what the next extraction reads. Leaving the gloss would regenerate this bug |

## Blast Radius

`internal/component/ike/engine` only. The change is wire-visible in exactly one scenario,
which is a simultaneous rekey. It moves that scenario from broken to conformant against any
peer that is not Ze. Two Ze peers keep agreeing, because the rule stays symmetric.

## Risks & Assumptions

| Id | Statement | Basis | Validation |
|----|-----------|-------|------------|
| A-1 | Both callers use the boolean the same way | Read on 2026-07-30 at `inbound.go` and `:216` | Re-read both, and prove each separately |
| R-1 | An interop suite pins the current direction | The ipsec suite drives real rekeys | Run `make ze-ipsec-interop-test` and read any failure as evidence about strongSwan's direction, not as a reason to revert |

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | The endpoint whose exchange carries the LOWER nonce abandons that exchange |
| AC-2 | The endpoint with the higher nonce keeps its exchange and answers as usual |
| AC-3 | Both call sites are corrected, the child branch and the IKE branch |
| AC-4 | Row `RFC7296-2.8-1` in `rfc/short/rfc7296.md` states the RFC's direction |
| AC-5 | `rekey_test.go` asserts the corrected direction, carrying the `rfc-test-change-approved` marker |
| AC-6 | The tagged pair for `RFC7296-2.8-1` still gates: breaking the comparison reddens both polarities |
| AC-7 | `RFC7296-2.8.2-1` still holds, since exactly one new SA survives and it inherits the Child SAs |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| Inbound child rekey with our rekey in flight (`inbound.go`) | -> | the corrected decision | `TestRekeyCollisionLowestNonceAbandons` |
| Inbound IKE rekey with our rekey in flight (`inbound.go`) | -> | the corrected decision | `TestRekeyCollisionIKEBranchLowestNonceAbandons` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Proves |
|------|--------|
| `TestRekeyCollision`, corrected | AC-1, AC-2, AC-5 |
| `TestRekeyCollisionLowestNonceAbandons` | AC-1, AC-3 |
| `TestRekeyCollisionIKEBranchLowestNonceAbandons` | AC-3 |

### Functional Tests

| Test | Role |
|------|------|
| `test/ipsec/ipsec-child-rekey.ci` | The regression net. It drives a real Child SA rekey between two daemons, so it proves the corrected decision did not break the ordinary, uncontested rekey |
| `test/ipsec/ipsec-sa-installed.ci` | Proves an SA still reaches the dataplane after the change |
| `test/ipsec/ipsec-clear-reestablish.ci` | Proves teardown and re-establishment still work, which is the path a losing exchange takes |

A simultaneous collision needs both ends to start inside one round trip. No `.ci` test can
force that today. `make ze-ipsec-interop-test` is where Ze meets strongSwan. It is the right
home for a collision scenario when somebody writes one. Until then the three tests above
are the daemon-level proof that the change is safe. The Go tests prove the direction.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/ike/engine/rekey.go` | Rename the helper and correct its doc |
| `internal/component/ike/engine/inbound.go` | Invert the decision at both call sites |
| `internal/component/ike/engine/rekey_test.go` | Correct the assertions, with the approval marker |
| `rfc/short/rfc7296.md` | Correct the `RFC7296-2.8-1` row text |

## Implementation Steps

1. Write the corrected assertions first, and record the red output.
2. Rename the helper and correct its doc comment.
3. Invert the decision at both call sites.
4. Correct the row text.
5. Mutation-verify that the tagged pair still gates.

## Goal Gates

`make ze-verify`.

## Quality Gates

`make ze-lint-changed`, `make ze-rfc-check`, `python3 scripts/dev/ste_check.py --check`.

## RFC Documentation (Scope: protocol)

`rfc/short/rfc7296.md` row `RFC7296-2.8-1` is corrected. No other summary changes.

## Checklist

- [ ] Tests written first, run, and Tests FAIL output pasted into the spec
- [ ] Both call sites corrected
- [ ] Row text corrected
- [ ] Tests PASS output pasted into the spec
- [ ] The tagged pair mutation-verified
- [ ] `make ze-verify` green

## Test Evidence (2026-07-30)

Tests FAIL, before any production change. Both new call-site tests, both orderings:

```
=== RUN   TestRekeyCollisionLowestNonceAbandons
    rekey_test.go:89: our exchange carries the lower nonce, so it must be abandoned
    rekey_test.go:92: the abandoned exchange still holds the request window
    rekey_test.go:98: our exchange carries the higher nonce, so it must survive
    rekey_test.go:101: our surviving exchange released the request window it holds
--- FAIL: TestRekeyCollisionLowestNonceAbandons (0.00s)
=== RUN   TestRekeyCollisionIKEBranchLowestNonceAbandons
    rekey_test.go:125: our exchange carries the lower nonce, so it must be abandoned
    rekey_test.go:128: the abandoned exchange still holds the request window
    rekey_test.go:131: the surviving peer exchange built no new IKE SA
    rekey_test.go:134: the surviving peer exchange drew no answer
    rekey_test.go:148: our exchange carries the higher nonce, so it must survive
    rekey_test.go:151: our surviving exchange released the request window it holds
    rekey_test.go:154: the losing peer exchange still built a new IKE SA
    rekey_test.go:156: peer IKE rekey that lost the collision: ze wrote 432 unexpected bytes before the sentinel
--- FAIL: TestRekeyCollisionIKEBranchLowestNonceAbandons (5.09s)
FAIL	github.com/ze-software/ze/internal/component/ike/engine	5.114s
```

Tests PASS, after the correction:

```
=== RUN   TestRekeyCollision
--- PASS: TestRekeyCollision (0.00s)
=== RUN   TestRekeyCollisionLowestNonceAbandons
--- PASS: TestRekeyCollisionLowestNonceAbandons (0.00s)
=== RUN   TestRekeyCollisionIKEBranchLowestNonceAbandons
--- PASS: TestRekeyCollisionIKEBranchLowestNonceAbandons (0.09s)
ok  	github.com/ze-software/ze/internal/component/ike/engine	0.103s
```

Mutation, `bytes.Compare(...) < 0` changed to `> 0`. All three tests that carry
`RFC7296-2.8-1` go red, on both polarities, and green again on revert:

```
--- FAIL: TestRekeyCollision (0.00s)
    rekey_test.go:39: our nonce sorts below the peer nonce, so the comparison must read true
    rekey_test.go:42: our nonce sorts above the peer nonce, so the comparison must read false
--- FAIL: TestRekeyCollisionLowestNonceAbandons (0.00s)
--- FAIL: TestRekeyCollisionIKEBranchLowestNonceAbandons (5.09s)
```

Daemon-level regression net, the whole ipsec `.ci` suite:

```
pass  8/8  100.0%  14.8s
```

## Deviations

| What | Why |
|------|-----|
| Three more tests corrected than the spec named: `TestChildRekeyCollisionWeWin` (`rekey_wire_test.go`), `TestNegIKERekeyCollisionResolves` and `TestNegSurvivingSAInheritsChildren` (`rfc7296_negotiation_test.go`) | All three pinned the inverted direction and went red. `ai/rules/rfc-compliance.md` states that a test which pins non-conformant behaviour is the wrong artifact. Each keeps every assertion. Only the nonce each side sends changed, so the peer that must close its exchange is the peer that holds the lowest nonce |
| Two more prose sites corrected: `docs/guide/ipsec.md` rekeying paragraph, and the summary row for section 2.8 in `rfc/short/rfc7296.md` | Both carried the same "(lower nonce wins)" gloss the spec identifies as the source of the bug. A correct checklist row beside a wrong summary row regenerates it |
| `RFC7296-2.8-1` now has three tagged bindings per polarity, up from one | The one binding was a helper-level comparison test, and the comparison was never wrong. The direction lives at the two call sites, so the tags now cover them |

## Known Limitations

The RFC states the deletion as a SHOULD, so the inversion is not a MUST violation on its
own. `RFC7296-2.8.2-1`'s MUST is the inheritance, and that holds either way. The reason to
fix it anyway is interoperability: the direction has to match the peer's, and every other
implementation follows the RFC.
