# Learned: pick RFC backfill work by the ORACLE, not by the protocol

2922 gated MUST-level requirements are enrolled. 1536 of them were proven only
by Go unit tests, and the question this record answers is which of those 1536
are worth a second test at a higher tier.

The intuitive answer is "the wire-facing ones". A keyword classifier over
requirement text, calibrated on a 40-item sample, puts at least 76% of gated
MUSTs in that class, which is another way of saying the class is nearly all of
them for a protocol RFC. **The answer that survived measurement is different:
rank by whether the test's expected value comes from anywhere other than the
code under test.**

## The selection rule

A gated unit-only requirement is a backfill candidate when no test bound to it
carries an oracle independent of the code under test, AND its obligation is
observable at a boundary a runnable suite can reach.

| # | Test | Fails when |
|---|------|-----------|
| 1 | **Oracle independence.** Read every test tagged with the requirement id. Does at least one derive its expected value from the RFC text rather than by calling the production helper that computes it? | Every bound test computes `want` by calling the code the assertion exercises. `TestBuildCHAPResponse` (`internal/component/l2tp/pppoeclient/session_test.go`) writes `want := chapMD5Response(...)`, the helper `buildCHAPResponse` itself calls |
| 2 | **Boundary observability.** Can the obligation be seen at a socket, a file, or a command output rather than only inside a function? | The obligation is an internal invariant with no external form |
| 3 | **Tier reachability.** Does a suite named in `internal/le/functional.Gating` already boot the owning subsystem? | No suite runs it. `CarrierFor` (`internal/le/rfc/carriers.go`) answers `functional-unrun` and the scanner refuses the tag |

**The unit of analysis is the REQUIREMENT, not the test.** A self-oracled test
beside a known-answer test is harmless. `TestBuildCHAPResponse` is vacuous
alone, but `TestCHAPAuthenticationKnownVector`
(`internal/component/l2tp/pppoeclient/auth_test.go`) pins the same digest to a
hardcoded hex string, so RFC 1994's digest obligation is honestly proven and is
not a candidate. Judging test by test manufactures work.

## Why the oracle beats the protocol family

Five IKE defects shipped on 2026-08-01. Every one had been unit-green for
months, and every one was found only by running against strongSwan. What they
shared was not that they were wire obligations. It was that ze's producer and
ze's verifier were the same code path, so an assertion about one was satisfied
by construction from the other. `VerifyChallengeResponse` CALLING
`ChallengeResponse` (`internal/component/l2tp/auth.go`) is that shape written
plainly.

The ranking below was built from defect density and then corrected by
measurement.

| Rank | Class | Unit-only | State |
|------|-------|-----------|-------|
| 1 | IKE and EAP (rfc7296, rfc3748, rfc5216, rfc2759, rfc7427, rfc4301, rfc4303) | 254 | highest measured defect density; owned by `plan/pre-release/spec-rfc-evidence-deferred-ike-eap-tranche.md` |
| 2 | Subsystems with no runnable suite (BFD 98, VRRP 80, dhcpserver 28, geodns 18, dnsserver 18) | 242 | carrier now exists for BFD, DHCP and VRRP; owned by `plan/pre-release/spec-rfc-evidence-deferred-unbootable-suite-musts.md` |
| 3 | L2TP, PPP, PPPoE | 113 | worked as the tranche; better covered than predicted |
| 4 | IS-IS 52, RSVP-TE 25, LDP 7 | 84 | suites exist; owned by `plan/pre-release/spec-rfc-evidence-deferred-isis-rsvpte-ldp-tranche.md` |

**The tranche broke its own headline claim, and that is the useful part.** Five
mutations were applied to the L2TP producers through a Go overlay. The landed
`.ci` reddened on all four that touch it. The unit suite reddened on all five,
because the L2TP crypto already carried known-answer vectors that re-derive the
RFC formula in the test body. So the `.ci` bought altitude and oracle diversity
and bought no discrimination the unit tests lacked. A future tranche runs test 1
as a cheap scan across a whole RFC BEFORE picking it.

## Two constraints that are easy to miss

**A tag count is not a requirement count.** The parent spec quoted 2571 and that
figure was tags. Fold `CarrierFor` over the tags and count requirements. Import
the package rather than rendering `ai/RFC-REQUIREMENTS.md`: several sessions own
that file at once and a render is a write.

**Tier reachability is a hard gate.** `internal/le/functional.Gating` is the only
input `carriers` (`internal/le/rfc/carriers.go`) reads to grant a verify tier. A
`.ci` outside a gating suite earns `functional-unrun` and the scanner refuses
the tag, however good the test is.

## The owner's answer on the unbootable subsystems (2026-09-05)

Asked whether to add suites, accept nightly-only tier, or leave 242 MUSTs
unit-only, Thomas answered: *"bfd (98 MUSTs), vrrp (80) and dhcp (28) have no
suite: not acceptable, all RFC MUSTs need tests, so we need to add them."*

`bfd` and `dhcp` were declared new and `vrrp` gained the `Gating` membership it
lacked, so `CarrierFor` answers `functional-bfd`, `functional-dhcp` and
`functional-vrrp` at `verify` where it answered `functional-unrun` before.
`TestTheBFDDHCPAndVRRPSuitesCarryAVerifyTier` (`internal/le/rfc/tags_test.go`)
pins that. The 206 tagged tests those three carriers now make possible are
follow-on work. `geodns` and `dnsserver` are outside the answer and their 36
MUSTs stay unit-only.

## The tranche's own evidence

`test/l2tp/rfc2661-emitted-control-shape.ci` binds RFC2661-4.1-1, RFC2661-4.1-2
and RFC2661-x-1 at `functional/verify`. Each carries its own assertion line and
its own revert record in `rfc/discrimination/rfc2661.json`, observed red against
`WriteAVPHeader`, `writeSCCRPBody` and `WriteControlHeader` in turn. One
whole-suite assertion covering three requirements is refused by
`./le rfc discriminate-record` with `citation-gone`: a functional record must
cite the single assertion its red was observed at.
