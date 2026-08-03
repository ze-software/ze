# 1330 -- rfcgate-2-deferred-nonunit-evidence-backfill

## Context

`plan/learned/1296-rfcgate-2-evidence.md` built the machinery for non-unit RFC
evidence: four declared carriers, an execution tier per carrier, a `kind/tier`
cell on every ledger link, and a per-tier monotonic ratchet. It re-bound none of
the existing corpus, so 1536 gated MUST-level requirements were still proven
only by Go table tests. This spec had to answer a prior question before draining
anything: which unit-only requirements are actually proven badly. It produced a
selection rule, a measured ranking, and one worked tranche of three RFC 2661
requirements now bound at `functional/verify`.

## Decisions

- Keyed the selection rule on the TEST'S ORACLE over a keyword classifier on
  requirement text. The classifier answers "is this about the wire", which is
  nearly always yes for a protocol RFC and is not a reason to write a test. What
  separated the five IKE defects of 2026-08-01 was that Ze's producer and Ze's
  verifier were the same code path, so an assertion about one was satisfied by
  construction from the other.
- Made the REQUIREMENT the unit of analysis over the test. A self-oracled test
  beside a known-answer test is harmless. Judging test by test would have
  manufactured work on RFC 1994, where `TestBuildCHAPResponse` is vacuous but
  `TestCHAPAuthenticationKnownVector` pins the same digest to hardcoded hex.
- Took L2TP as the tranche over IKE, which the ranking put first. A concurrent
  session was reviewing `internal/component/ike/**`, and editing under a live
  review costs more than it buys.
- Landed only positive assertions over also landing the zero-tunnel-id and
  reserved-bit-on-receive negatives. Both would have asserted an ABSENCE, which
  is the vacuity trap in `ai/rules/interop-and-goal-validation.md`. Both became
  owner questions instead.
- Wrote no `{gap}` for either conformance question. `ai/rules/rfc-compliance.md`
  reserves that call for the owner, and an annotation would have lowered what Ze
  owes without anyone deciding to.

## Consequences

- `RFC2661-4.1-1`, `RFC2661-4.1-2` and `RFC2661-x-1` now carry
  `functional/verify` evidence from `test/l2tp/rfc2661-emitted-control-shape.ci`
  alongside their unit tags. The evidence ratchet
  (`check_evidence_ratchet`) will fire if that `.ci` is downgraded to a unit
  test, so the tier is now held, not merely reached.
- The remaining corpus is homed rather than implied. `plan/spec-rfc-evidence-deferred-ike-eap-tranche.md`
  owns the 254 IKE and EAP requirements,
  `plan/spec-rfc-evidence-deferred-isis-rsvpte-ldp-tranche.md` owns the 84
  IS-IS, RSVP-TE and LDP requirements, and
  `plan/spec-rfc-evidence-deferred-unbootable-suite-musts.md` owns the 242 whose
  subsystem no suite boots. The two L2TP conformance questions this tranche
  raised are homed at `plan/spec-fixit-l2tp-sccrq-tunnel-id-zero.md` (the
  StopCCN reply) and `plan/spec-finish-l2tp.md` (the AVP reserved-bit receive
  probe).
- A future tranche runs the rule's oracle test as a cheap scan across a whole
  RFC BEFORE picking it. Ranking by protocol family is weaker than ranking by
  oracle, and this session measured that rather than assuming it.
- Tier reachability is a hard constraint on any backfill plan. 242 gated MUSTs
  (BFD 98, VRRP 80, dhcpserver 28, geodns 18, dnsserver 18) cannot be bound at
  verify tier at all until a suite exists, no matter how good the test is.

## Gotchas

- The spec's own headline figure, `unit/verify 2571`, was a TAG count read as a
  REQUIREMENT count. The requirement number is 1536. Fold `carrier_for` over
  tags and count requirements; do not quote the tag total.
- `functional_suites()` refuses a `# RFC requirement:` tag in a `.ci` whose
  suite `mk/test-functional.mk` `all_suites` does not name, resolving it to
  `TIER_UNRUN`. A perfectly written test in an unlisted suite earns no evidence
  at all, and the refusal is the correct interim state.
- Assumption A-1 was BROKEN by measurement. Four of five mutations were caught
  by the existing unit suite, because the L2TP crypto already carries
  known-answer vectors. The `.ci` bought altitude and oracle diversity, not
  discrimination the unit tests lacked. Report that honestly: a backfill that
  raises tier without raising discrimination is a greener ledger for nothing.
- `RFC2661-4.2-1` (tunnel authentication) is a `[MAY]`, so it is not gated and
  can carry no binding. The strongest assertion in the landed test, a digest the
  Python peer recomputes from RFC 2661 section 4.2, binds no requirement id.
- `ai/RFC-REQUIREMENTS.md` is generated and several sessions can own it in one
  week. Measure by importing `scripts/dev/rfc_requirements.py` and folding in
  memory. Rendering the ledger to read a number sweeps other sessions'
  uncommitted tags into your commit.

## Files

- `test/l2tp/rfc2661-emitted-control-shape.ci` (created; three gated bindings
  plus the independent digest oracle)
- `ai/RFC-REQUIREMENTS.md` (regenerated; the three `functional/verify` links)
- `plan/deferrals/rfcgate-2-deferred-nonunit-evidence-backfill.md` (the ranked
  remainder, re-homed at closure)
