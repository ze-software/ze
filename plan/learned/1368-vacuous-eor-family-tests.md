# 1368 -- A Test That Re-Implements The Logic It Names

## Context

Three tests in `internal/component/bgp/reactor/peer_test.go` claimed to cover RFC
4724 End-of-RIB family tracking. Each built a local `familiesSent` map, filled it
inline with the algorithm it was named after, and asserted on its own fill. No
production function was called. `familiesSent` existed nowhere in ze.

They were green against every implementation, correct or broken, and they read as
coverage: names, `VALIDATES:` comments, `PREVENTS:` comments. A session read
`TestFamiliesSentEmpty` ("no routes means no EOR families"), concluded ze deviates
from RFC 4724 by sending End-of-RIB only for families that carried routes, and was
about to escalate a conformance violation to the owner. `sendInitialRoutes` is
conformant: its loop walks `nc.Families()`, and its comment already quoted the RFC
line the tests appeared to contradict.

## Decisions

- **No gate catches this class, and we said so rather than building one.** The
  assert-nothing detector looks for a missing oracle. This defect HAS an oracle,
  pointed at itself. The spec's A-3 assumed the sensitivity ratchet could widen;
  it cannot, because a local fixture asserted against is also what every correct
  table-driven test looks like, so the detector would fire on hundreds of good
  tests and be switched off within a week. Recorded in `ai/rules/testing.md`
  instead, with the one reliable tell: **the test names a function it never calls.**
- **Replaced, not deleted.** `TestInitialSyncEORReachesTheSilentFamilyToo` drives
  `sendInitialRoutes` and asserts the bytes that reach the socket, with one family
  carrying a route and the other silent. That fixture is the one that tells the two
  readings apart; the pre-existing all-silent test cannot, because both readings
  agree when nothing carried a route.
- **Two mutations, not one.** Deleting the loop proves the test sees the loop.
  Narrowing the loop to route-carrying families proves the test sees the SPECIFIC
  wrong belief the deleted tests encoded. The second is the one worth running.
- **The functional test's limits are written into the functional test.** Under the
  narrowing mutation, `test/encode/eor-silent-family.ci` stays GREEN across three
  consecutive runs: `AnnounceEOR` (`reactor_api_forward.go`) is a second path to
  the same wire and its suppression lifts once the drain finishes. The `.ci` proves
  ze's output; only the unit test pins the producer. Its header says so.

## Consequences

**Four review rounds, and the code was clean after one.** Pass 1 confirmed the
tests, the hex and the mutations, and every round after it corrected the RECORD:
a mutation block pasted with the wrong test's numbers, a grep cited as evidence
that had never been run, a "whole output" claim over condensed text, and a
deferral row closed on filing against `ai/rules/planning.md`. Not one was in the
product. A spec about a test that asserted on its own fill produced four
statements that asserted on their author's memory, which is the same move at a
different layer. The defence is the same too: name the command, run it, paste what
it printed. Writing a closure section from recollection while the terminal is one
call away is where the cost lands.

Redundant producers make a daemon robust and make functional tests blunt. Where two
code paths can emit the same frame, a `.ci` cannot attribute it, so a claim about
one producer needs a test at that producer's level. Neither test is the whole
claim, and each now names the other.

The spec cited "RFC 4724 Section 2" for "including the case when there is no update
to send". That clause is Section 4 (`RFC4724-4-1`); Section 2 defines the marker's
encoding and recommends its use (`RFC4724-2-1`). Both `RFC4724-4-1` and
`RFC4724-4.2-9` now carry a reactor-level positive beside their existing
message-level one, so the ledger distinguishes "the frame encodes correctly" from
"the daemon emits one per negotiated family".

## Files

- `internal/component/bgp/reactor/peer_initial_sync.go` -- `sendInitialRoutes`, the
  conformant producer. Unchanged by this work.
- `internal/component/bgp/reactor/peer_initial_sync_test.go` -- the replacement
  tests and the guard.
- `internal/component/bgp/reactor/peer_test.go` -- the `test-relax:` gravestone
  where the three vacuous tests stood.
- `internal/component/bgp/reactor/reactor_api_forward.go` -- `AnnounceEOR`, the
  second producer that makes the functional test blunt.
- `test/encode/eor-silent-family.ci` -- the daemon-level half.
- `test/plugin/eor.ci` -- the pre-existing all-silent case.
- `ai/rules/points/testing/test-sensitivity-ratchets-blocking/a-test-that-re-implements-the-logic-it-names.md`
  -- the recorded class.

| Gotcha | Why |
|--------|-----|
| A guard that greps its own package for an identifier must not contain that identifier | `TestNoTestBuildsItsOwnFamiliesSentMap` splits the needle in two, or it fails on its own source |
| `assert.NotContains` over a file body prints the whole file | Use `assert.False(t, strings.Contains(...))` with a message that names the file |
| `grep -rn <x> --include='*.go' .` is NOT evidence about this repository | It walks `.claude/worktrees/`, where other sessions keep full checkouts. Searching for the deleted identifier returned 19 hits from a sibling session's copy and zero from the tracked tree. `git grep` reads what git tracks, and `.git/info/exclude` already keeps the worktrees out of it |
| A guard whose needle is split will not match its own file with EITHER grep | So "the grep finds only the guard's own literal" is never a true sentence about it. The true sentence is that the grep finds nothing |
