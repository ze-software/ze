# Spec: fixit-ike-rekey-collision-inverted

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-07-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-30 during phase 2b of the rfcgate-1b RFC 7296 pilot spec. An agent
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
`spec-fixit-ike-negotiation-conformance` (closed 2026-08-12).

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

The rfcgate-1b RFC 7296 pilot spec owns rows `RFC7296-2.8-1` and `RFC7296-2.8.2-1`.
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

**Deferral shard row corrected to `-` on 2026-08-03 (bookkeeping audit): it named
`plan/deferrals/fixit-ike-rekey-collision-inverted.md`, which never existed. This spec
deferred nothing. Its Deviations section records work done BEYOND the plan (three more tests
and two more prose sites corrected than the spec named), and the limitation below is a
rationale for fixing a SHOULD, not postponed work.**

The RFC states the deletion as a SHOULD, so the inversion is not a MUST violation on its
own. `RFC7296-2.8.2-1`'s MUST is the inheritance, and that holds either way. The reason to
fix it anyway is interoperability: the direction has to match the peer's, and every other
implementation follows the RFC.

---

## CLOSURE REFUSED 2026-08-03 -- the direction was corrected, the algorithm is still wrong

`/ze-close` re-verified AC-1 through AC-7 against their producing functions and put the
changeset to three independent reviewer subagents. **This spec is NOT closed.** The RFC
lens read Section 2.8.1 verbatim and found two MUST-level defects that this spec's own
tagged tests now PIN as correct behaviour. Both were reproduced against
`rfc/full/rfc7296.txt` and against source before being graded.

What this spec DID achieve stands: the direction is corrected at both call sites, the helper
states a fact rather than a verdict, and the "(lower nonce wins)" gloss is gone from the
checklist row, the summary row and `docs/guide/ipsec.md`. The findings below are about the
comparison this spec deliberately did not touch ("The comparison itself is correct ... Only
the direction is wrong").

### BLOCKER 1 -- a nonce collision leaves an authenticated peer request unanswered

Section 2.8.1, describing the moment a node receives the colliding request:

<!-- ste: ignore -->
> At this point, A knows there is a simultaneous rekeying happening. However, it cannot yet know which of the exchanges will have the lowest nonce, so it will just note the situation and respond as usual.

Section 2.21.3:

<!-- ste: ignore -->
> After the IKE SA is authenticated, all requests having errors MUST result in a response notifying the other end of the error.

Both "our nonce is higher, ignoring peer request" arms of `handleCreateChildSAOwned`
(`internal/component/ike/engine/inbound.go`, the Child branch and the IKE branch) return
`ownedOutcome{}` with no `respondChildRekey`, no `respondError` and no `cacheResponse`. The
peer's authenticated request draws nothing at all, `sa.ExpectedMsgID` never advances, and
each retransmission re-enters the same arm. Against strongSwan a simultaneous rekey then
kills the tunnel about half the time: our nonce is higher, we say nothing, the peer
retransmits its budget and declares the SA dead.

**The violation carries a green bar.** `TestRekeyCollisionIKEBranchLowestNonceAbandons`
(`internal/component/ike/engine/rekey_test.go`), the `RFC7296-2.8-1 negative` binding,
asserts it in words -- "It writes no answer" -- and enforces it with
`rtxExpectSilence(...)`. Per `ai/rules/rfc-compliance.md` the test is the violation with a
green bar on top, and it must be corrected together with the code.

### BLOCKER 2 -- the collision is resolved from two nonces of four, and decided too early

<!-- ste: ignore -->
> If redundant SAs are created through such a collision, the SA created with the lowest of the four nonces used in the two exchanges SHOULD be closed by the endpoint that created it.

The RFC's worked example makes the point decisive: "Suppose that the lowest nonce was Nr1 in
message resp2; in this case, B (the sender of req2) deletes the redundant new SA". **Nr1 is
a RESPONDER nonce**, and it decides the outcome. The comparison happens after both responses
have arrived, when "there are three Child SA pairs between A and B ... A and B can now
compare the nonces".

`localNonceIsLower` (`internal/component/ike/engine/rekey.go`) is called from both sites with
`p.localNonce` (our Ni) against `nonceFromPayloads(inner)` (the peer's Ni). Two of the four
nonces, at request-receipt time, which is the moment the RFC says is too early to know.

Two Ze peers agree with each other, because the rule stays symmetric. A conforming peer
disagrees whenever the global minimum is a responder nonce sitting in the exchange with the
higher Ni, which is roughly a quarter of collisions, leaving either zero surviving new SAs
or two redundant ones. This is the same class of failure the spec's own "interoperability
failure" table describes, one layer down.

### ISSUE 3 -- row `RFC7296-2.8-1` is enrolled at the wrong requirement level

The row is tagged `[MUST]`. Section 2.8.1 says "SHOULD be closed by the endpoint that
created it" and Section 2.8.2 says "SHOULD be deleted by the node that created it". The MUST
in Section 2.8.1 is a different sentence -- "a node MUST accept incoming packets through
either SA" -- and `RFC7296-2.8.1-1` already holds it. The row's TEXT also states the
four-nonce rule correctly, which the code does not implement, so the row currently describes
behaviour Ze does not have and grades it at a level the RFC does not ask for.

### ISSUE 4 -- `TestRekeyCollision` is thin evidence for the row it is tagged with

`TestRekeyCollision` asserts that `localNonceIsLower` behaves like `bytes.Compare`. It DOES
discriminate an inversion of the helper, which this spec's own mutation table records, so it
is not vacuous. It cannot discriminate an inversion of the `!` at either call site, which is
where the RFC's direction actually lives, and it carries a full `RFC requirement:` tag as
though it could. `TestRekeyCollisionLowestNonceAbandons` and
`TestRekeyCollisionIKEBranchLowestNonceAbandons` are the bindings that carry the rule.

### Owner decision needed before this can be fixed

BLOCKER 1 and BLOCKER 2 cannot be fixed without editing RFC-tagged tests, which
`ai/rules/testing.md` reserves to the user and the `rfc-tagged-test` hook refuses without a
`// rfc-test-change-approved:` marker naming what was approved. The 2026-07-30 authorization
in this spec covers correcting the DIRECTION; it does not cover replacing the algorithm.

BLOCKER 2 is also a redesign rather than an edit: responding as usual and resolving after
both responses arrive means holding both exchanges open, keeping our own Nr and the peer's,
and deleting the redundant SA at a point the code has no state for today. It touches
`pendingRekey`, `pendingIKESwap`, the responder path, and three enrolled tagged tests.

The question owed is which way to fix it, never whether to skip it
(`ai/rules/rfc-compliance.md`). No interop scenario in `test/ipsec-interop/` exercises a
simultaneous rekey collision, which is why both defects survived to HEAD, and a scenario
that does is part of the fix rather than an extra.
