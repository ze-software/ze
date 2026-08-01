# 1224 -- RFC 7606: closing five disclosed gaps, and what "implement the MUST" actually costs

## Context

`plan/spec-rfc7606-close-gaps.md`. `rfc/short/rfc7606.md` carried eight `{gap}`
annotations. The owner excluded §5.1-1 (the deliberate MP_UNREACH-first ordering,
`docs/architecture/wire/mp-nlri-ordering.md`) and, after reading the analysis,
§5.4 (typed-NLRI discard). The remaining six were implemented.

A side finding that framed the whole job: `docs/features/rfc-status.md` claimed
"Seven MUST-level gaps" and enumerated seven, while the summary carried eight.
The undisclosed one was §5.1-2. `check_status_agreement` cannot catch that -- it
only requires the row to be non-"Supported", it cannot count.

## What was built

- `message/rfc7606_optional_attrs.go`: validators for the three attribute codes
  that had none. `attrValidators` is opt-in and `validateAttribute` returns nil
  for an unregistered code, so codes 24, 25 and 128 accepted ANY length.
  - §7.13 (code 24): rejects below one RFC 5543 descriptor (36 octets).
  - §7.15 (code 25): non-zero multiple of 20, reading LENGTH ONLY.
  - §7.16 (code 128): all three RFC 6368 §5 conditions, recursing through
    `validateAttribute`, depth-capped at 4.
- `reactor/session_validation.go`: `rfc7606Diagnostics` logs the NLRI involved and
  the entire UPDATE body at all four enforcement outcomes (§6).
- `wireu/split.go`: `buildCombinedUpdates` restructured from an interleaved
  fill-loop to four sequential per-component drains, so each re-chunked UPDATE
  carries at most one NLRI-bearing field.

## Lessons

1. **A validator must be judged in the context its own RFC specifies, not the
   session's.** The ATTR_SET walker forwarded the enclosing session's `isIBGP`
   and `asn4` to the inner attribute stream. Both are wrong, and both
   over-validate, which means withdrawing conforming routes:
   - RFC 6368 §5 requires inner AS_PATH/AGGREGATOR to use 4-octet ASNs
     "**regardless of the capabilities advertised by the BGP speaker to which the
     ATTR_SET attribute is transmitted**". Passing the session's `asn4` made
     `validateASPathAttr` read a conforming 4-octet inner AS_PATH with a 2-octet
     AS size on any session that had not negotiated RFC 6793.
   - An ATTR_SET exists to carry the CUSTOMER's iBGP attributes across a provider
     (RFC 6368 is "Internal BGP as PE/CE Protocol"), so LOCAL_PREF, ORIGINATOR_ID
     and CLUSTER_LIST are legitimate inside it. The session's eBGP context
     withdrew any route whose ATTR_SET preserved the customer's LOCAL_PREF.
   The clause that gives the game away was sitting in the RFC text I had already
   quoted into the spec. **Reading the RFC is not the same as reading it for the
   question you are currently answering.**

2. **RFC 7606 grades its actions, and a wrapper must not flatten that grade.**
   The walker treated any non-`None` inner result as "malformed", so an inner
   attribute whose own action is `attribute discard` (AGGREGATOR §7.7, LOCAL_PREF
   §7.5, ORIGINATOR_ID §7.9, CLUSTER_LIST §7.10) escalated to a whole-route
   withdraw. RFC 7606 chose discard for those precisely so the route survives;
   escalating inverts the document's central design decision. Gate on
   `>= RFC7606ActionTreatAsWithdraw`, never on `!= None`.

3. **"Implemented" and "the MUST is met" are different claims, and the gate knows.**
   The §5.1-2 work made every UPDATE ze RE-CHUNKS compliant, and ze already
   originated only compliant UPDATEs. But two relay paths still reproduce a
   received mixed shape (`forward_body.go:63-65` verbatim, `:99` whole emit), so
   the MUST is not met. Tagging the new tests as `RFC requirement: RFC7606-5.1-2`
   was an overclaim, and `make ze-rfc-check` caught it within seconds: "annotated
   {gap} but IS tested". The annotation was narrowed and the tags removed.
   **The contradiction between a `{gap}` and a tag is a real safety property, not
   bookkeeping.**

4. **A guard that treats a tag as proof the instant it is typed freezes the author
   out of their own uncommitted work.** `_rfc_tagged_change_err` blocked, in one
   session: a one-line lint fix inside a test written minutes earlier, and the
   removal of the overclaiming tags in lesson 3 (tag REMOVAL is checked first and
   unconditionally, which is right in general and wrong here). Neither file had
   ever been committed, and the gate had never counted their tags. The workaround
   was to write each test file complete in one shot and, when that failed, to get
   an explicit `rfc-test-change-approved:` from the owner. **Worth considering: a
   tag that is not in HEAD is not yet proof of anything.**

5. **A comment-only edit is only exempt if the hunk contains the comment marker.**
   `_behavior_bytes` strips `//...` before comparing, so a hunk that is a
   FRAGMENT of a comment line -- no `//` in it -- reads as code and blocks. Include
   the `//` in `old_string` and the same edit sails through.

6. **An allocation assertion is the right probe for an eager-evaluation guard.**
   The first §6 negative test installed a WARN-level handler and asserted no line
   appeared. That passes with or without the `Enabled()` guard, because the
   handler drops the line either way -- and the audit note claimed it "pins the
   Enabled() guard", which was false. `testing.AllocsPerRun` == 0 is what actually
   discriminates: without the guard, slog's eager argument evaluation builds the
   hex dump and both prefix lists before the handler ever sees them.

7. **Fetch the referenced RFC before deciding what "malformed" means.** §7.13,
   §7.15 and §7.16 all delegate: to RFC 5543 (which "does not detail what
   constitutes malformation" -- so the check must stay minimal, since
   over-validating blackholes valid routes), to RFC 5701, and to RFC 6368 (which
   defines three precise conditions). `rfc/full/` had only 5701; the other two
   were fetched. Guessing any of the three would have produced a plausible,
   wrong validator.

## Consequences

- `make ze-rfc-check`: 2535 tags resolved, up from 2519. RFC 7606 gaps: 8 -> 3.
- `docs/features/rfc-status.md` now states three gaps and describes each honestly,
  correcting the seven-versus-eight under-count.
- Codes 24, 25 and 128 are now validated on every received UPDATE. ze's own
  encoder cannot trip them: `IPv6ExtendedCommunities.Len()` is `len(e)*20`
  (`core/bgp/attribute/community.go:486`), and nothing in ze emits 24 or 128.
- Two pre-existing defects found and NOT fixed here, both reported:
  `message.Splitter.splitUpdateWithMP` silently drops IPv4 withdrawn/NLRI when an
  UPDATE also carries an MP attribute (`update_split.go:344-348`), and
  `EVPNGeneric.Len()` over-reports by 2 versus what `WriteTo` writes
  (`plugins/nlri/evpn/types.go:915,917-919`).
- §5.4 stays open by owner decision. The analysis behind it: ze has no EVPN
  forwarding plane, so it is only ever a relay for those routes; discarding
  unrecognized types would remove function from the PEs on either side while
  improving nothing locally, and RFC 7606 §6 warns that exactly this kind of
  asymmetric dropping inside an AS causes "long-lived forwarding loops and black
  holes". RFC 9552 §5.1 shows the IETF using §5.4's own escape clause for BGP-LS
  ("MUST be preserved and propagated"); RFC 7432 never did for EVPN, so retaining
  is a genuine divergence rather than conformance by another route.

## Files

None recorded.
