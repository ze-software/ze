# 1334 -- rfc7606-54-typed-nlri-review-fixes

## Context

`spec-fixit-rfc7606-5-4-discard-unrecognized-nlri` landed the Section 5.4
discard of unrecognized typed NLRI in a1aec5e6c: a per-family recognizer
registry (`internal/core/bgp/nlri/nlritype`), an ingress filter in
`enforceRFC7606`, six tagged unit tests and two `.ci` relay tests, all green and
mutation-verified. An independent review then found four blockers in it. Three
were wire-visible defects the green bar had not touched, and the fourth was that
the family the RFC names FIRST was never covered. Every one is a shape that
recurs, so they are written down as shapes rather than as a changelog.

## Decisions

- **Read the RFC's own example list as the scope, not the family in hand.**
  RFC 7606 Section 5.4 names MCAST-VPN, MCAST-VPLS and EVPN in one sentence. The
  implementation proved EVPN. `ipv4|ipv6/mvpn` and `ipv4|ipv6/mup` still retained
  and relayed unrecognized types, and nothing was red: `nlritype.Register`
  refuses a family with no `nlrisplit` splitter, and `nlrisplit/register.go`
  bound splitters for unicast, multicast, EVPN and labeled only. A precondition
  that fails closed at REGISTRATION time is invisible from the family that has
  it. When a requirement enumerates its own subjects, enumerate them back.
- **Treated "the rewrite produced an UPDATE that says nothing" as a wire
  forgery, not as a no-op.** With every route discarded and MP_UNREACH the only
  attribute, `rewriteMPNLRISections` drops the attribute, `RebuildUpdateBody`
  emits four zero octets, and that IS the RFC 4724 Section 2 End-of-RIB marker.
  Relaying it ends a restarting neighbor's route deferral on a withdrawal the
  peer never meant as an EoR. There is no encoding of "an UPDATE that says
  nothing" that is not an EoR, so the only correct answer is not to relay it.
- **Did not believe a comment that said another check had already run.**
  `typedNLRIEdit` relayed the whole NLRI section on a split error, justified by
  "Section 5.3 has already run on this attribute". `validateMPNLRISyntax`
  returns nil for every family outside IPv4/IPv6 unicast and multicast, and says
  so in its own doc comment. One appended truncated NLRI therefore bypassed the
  MUST on demand. Section 5.3 plus Section 3(j) make that a session reset, which
  is what ze already does for the families it did walk.
- **Fixed the pre-existing reactor test hang rather than working around it.**
  `TestRefusedOpenIncrementsCounter` started a permanent drain on its `net.Pipe`
  client BEFORE calling `acceptWithReader`, whose contract is that it does the
  one read itself. net.Pipe delivers a write to exactly one reader, so the
  helper's reader blocked and `wg.Wait` never returned: the whole reactor
  package timed out at 600s instead of failing. Not this spec's code, but this
  spec's evidence depended on that package.
- **Delegated the RFC audit re-judgement instead of re-stamping it.** Adding
  tagged tests makes `rfc/audit/rfc7606.json` stale by design. The author of the
  tests is the wrong judge of whether they would fail on non-compliance, so the
  re-audit ran as a separate agent under `/ze-rfc-audit`.

## Consequences

- **An `RFC requirement:` tag inside a Python docstring is invisible to the
  ledger.** `scan_python_tags` (`scripts/dev/rfc_requirements.py`) tokenizes and
  reads COMMENT tokens only, so a tag written inside a string literal is string
  content. The interop scenario ran, passed, and was counted as evidence for
  nothing. The independent audit is what caught it; `make ze-rfc-check` cannot,
  because a tag it never sees is indistinguishable from a tag nobody wrote. Put
  the tag on its own `#` line below the docstring.
- **The speaker harness could not have proven an EVPN claim before this.**
  `test/interop/speaker/engine.py` hardcoded an IPv4-unicast-only OPEN, and ze
  gates every announce on the negotiated family set, so an EVPN check against it
  would have passed on an empty session. `--family AFI:SAFI` now names them,
  defaulting to today's IPv4 unicast. Its `route-bearing-updates` counter read
  the legacy NLRI and Withdrawn fields only, so it reported 0 for a perfectly
  good MP-family relay. `carries_routes` now reads MP_REACH and MP_UNREACH too,
  and still excludes both End-of-RIB encodings so `--stop-after-updates` cannot
  trip before the route arrives.
- **Registering a splitter is not free.** `nlrisplit.Supported` also gates RIB
  installation (`RIBManager.handleReceivedStructured`, `insertPoolNLRIs`), so
  binding MVPN and MUP splitters makes ze install those families as opaque RIB
  entries, the way it already installs EVPN. That is unavoidable: judging a
  route type requires carving the section, and one registry owns that carve.
  Say it out loud in the spec rather than discovering it in production.
- Mutation evidence is the unit of trust here, not the green bar. Eight
  production lines were reverted one at a time and each turned its owning test
  red; the interop scenario was rebuilt against a disabled discard and reported
  `evpn-nlri: 2` with route type 99 on the wire. The six tests that existed
  before this pass were also green before these four defects were found, which
  is the whole reason the mutation step is not optional.

## Files

- `internal/core/bgp/nlri/nlrisplit/typelen.go` -- the one type-and-length framing walk EVPN, MCAST-VPN and BGP-MUP share
- `internal/core/bgp/nlri/nlrisplit/mvpn.go` -- RFC 6514 Section 4 splitter
- `internal/core/bgp/nlri/nlrisplit/mup.go` -- draft-ietf-bess-mup-safi Section 3.1 splitter
- `internal/core/bgp/nlri/nlrisplit/register.go` -- binds both, which is the precondition `nlritype.Register` enforces
- `internal/component/bgp/plugins/nlri/mvpn/rfc7606.go` -- the MCAST-VPN Section 5.4 ruling
- `internal/component/bgp/plugins/nlri/mup/rfc7606.go` -- the BGP-MUP Section 5.4 ruling
- `internal/component/bgp/reactor/session_validation_nlritype.go` -- the emptied-UPDATE drop, the Section 5.3 session reset, the header-octet rewind
- `internal/component/bgp/reactor/session_validation.go` -- `enforceRFC7606` acts on the two whole-UPDATE outcomes
- `internal/component/bgp/reactor/reactor_metrics_behavioral_test.go` -- the net.Pipe double-reader hang
- `test/interop/speaker/engine.py` -- `--family` and `carries_routes`
- `test/interop/speaker/plugins/no_unrecognized_evpn_type.py` -- the independent peer's judgement
- `test/interop/scenarios/53-rfc7606-54-typed-nlri-discard/check.py` -- where the docstring tag was invisible
- `scripts/dev/rfc_requirements.py` -- `scan_python_tags`, which reads COMMENT tokens only

## Reusable

- When a requirement enumerates its subjects, enumerate them back. A
  registration precondition that fails closed hides the ones you skipped.
- A rebuilt message that conveys nothing is not empty; it is whatever "nothing"
  already means on that wire. Ask what the bytes decode to before relaying them.
- A comment claiming an earlier check ran is a hypothesis. Open the function it
  names (`ai/rules/evidence.md`).
- RFC tags in Python go in `#` comments, never in a docstring.
