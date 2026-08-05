# 1336 - withdraw-only relay shape

FRR rejected every withdrawal ze relayed to an eBGP peer, fixed 2026-08-04 under
`plan/spec-rfc7606-5-1-2-relay-shape.md`. Found while building an OTC interop
scenario, not while looking for it.

## The defect

`(*ASPathEdit).recordPrepend` (`internal/component/bgp/wireu/aspath_slot.go`)
built `&attribute.ASPath{}` from nothing when the forwarded payload carried no
AS_PATH, and emitted it. `forwardUpdateCore`
(`internal/component/bgp/reactor/reactor_api_forward.go`) drove that on
`facts.isEBGP` alone. A source withdrawal of `attrLen=0000` therefore reached an
eBGP peer as `attrLen=0009`: AS_PATH present, ORIGIN and NEXT_HOP absent.

RFC 4271 Section 6.3 makes a missing well-known mandatory attribute a Missing
Well-known Attribute error. FRR 10.3.1 answers with `Missing well-known attribute
NEXT_HOP` and `rcvd UPDATE with errors in attr(s)!! Withdrawing route`, so the
withdrawn route stayed live at the peer.

The fix reads the RFC's own condition: Section 5.1.2 obliges the prepend "when a
given BGP speaker advertises the route to an external peer". `Record` now routes
a payload with no reachable NLRI to `recordTranscode`, so `recordPrepend` -- the
only frame that can create an AS_PATH -- is unreachable for a withdraw-only
UPDATE.

## Traps

- **A comment cited a real RFC sentence and was still wrong.** "RFC 4271 Section
  5: AS_PATH is well-known mandatory" is true, and it is true only of an UPDATE
  that ADVERTISES. The comment quoted the obligation and dropped its condition,
  which is the shape a rationalization takes when it is sincere
  (`ai/rules/evidence.md`).

- **Eight test fixtures had the same hole as the code.** They asserted a prepend
  over a payload with no NLRI, so each was a withdraw-only UPDATE claiming to be
  an advertisement. They stayed green for years because the code shared their
  misreading. When a fix reddens a test, ask whether the FIXTURE ever matched the
  behavior its prose named, before reaching for the assertion.

- **A tag inside a Python docstring is invisible to the ledger.**
  `scan_python_tags` (`scripts/dev/rfc_requirements.py`) tokenizes and reads
  COMMENT tokens only, so `RFC requirement:` lines in a `check.py` module
  docstring bind nothing. `make ze-rfc-check` stays green, because a tag that is
  never read is not a tag that fails. Interop scenario 51 shipped that way and was
  counted as evidence for nothing. One line settles it:
  `python3 -c "import rfc_requirements as R; print(R.scan_tree())"`.

- **`Message.IsEOR` classified by LENGTH** (`internal/test/peer/message.go`):
  body 4 or 11. A legacy End-of-RIB (body 4) with a 7-byte OTC attribute stamped
  on it is exactly 11, and `Checker.ExpectedOrKeepalive` silently accepts an
  unmatched EoR. A `.ci` asserting that a relayed marker arrives UNSTAMPED could
  therefore only fail by TIMING OUT: it never saw the message it existed to
  refuse. Classification in test infrastructure decides what a test can fail on.

## Choosing an interop witness

The first version of this proof used the route-server rail, where RFC 7947
transparency means ze records no prepend at all. A stamped and an unstamped
withdrawal are indistinguishable there. The rail that carries the defect is the
one the scenario must use, and "which rail is convenient" is not the question.

Two mutants, not one, because the first mutant reddened the WRONG assertion.
Deleting the guard outright also stamps the relayed End-of-RIB, which stops being
a marker, so the injector's readiness barrier never releases and the run dies at
the POSITIVE. Narrowing the guard to `len(payload) == 4` leaves the barrier
intact and puts the failure where the design intended, with FRR's two error lines
as the evidence. **A mutant that reddens the test is not yet proof that the
assertion under test discriminates.** Read WHICH assertion failed.

## Files

- `internal/component/bgp/wireu/advertise.go`
- `internal/component/bgp/wireu/advertise_test.go`
- `internal/component/bgp/wireu/aspath_slot.go`
- `internal/component/bgp/wireu/aspath_slot_test.go`
- `internal/component/bgp/plugins/role/otc.go`
- `internal/component/bgp/reactor/forward_rs_test.go`
- `internal/test/peer/message.go`
- `internal/test/peer/message_eor_test.go`
- `test/plugin/role-otc-fwd-withdraw.ci`
- `test/plugin/role-otc-rs-withdraw-eor.ci`
- `test/interop/scenarios/51-role-otc-withdraw-frr`
- `test/interop/scenarios/52-relay-withdraw-shape-frr`
- `plan/deferrals/fixit-otc-src-role-meta-fallback.md`

## Where the same defect still lives

`forwardUpdateCore` injects ORIGINATOR_ID and CLUSTER_LIST for RFC 4456
reflection, and `applyFactsNextHop` (`reactor/peer_forward_facts.go`) SETs
NEXT_HOP, both without asking whether the UPDATE advertises anything. Same
incomplete well-known set, on the iBGP and next-hop-self rails. Not reached on
the eBGP default path, homed in the spec, and it wants the "advertises" bit
hoisted to the driver rather than three separate guards.
