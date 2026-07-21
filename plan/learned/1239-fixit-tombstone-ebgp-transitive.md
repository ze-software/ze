# 1239 -- fixit-tombstone-ebgp-transitive

## Context

Two deliverables, both landed in commit `706b77b7d` ("fix(bgp): clear the tombstone Transitive
bit at the eBGP boundary"): (1) honour the ATTR_TOMBSTONE forwarding-policy MUST
(draft-mangin-idr-attr-tombstone Section 5.3: an optional-transitive tombstone marker must have
its Transitive bit CLEARED when forwarded across an eBGP boundary, so a NEW speaker downstream
does not treat ze's in-place discard marker as a normal transitive attribute); and (2) repair
NINE dead normative citations in `internal/component/bgp/message/attr_discard.go` naming the
obsolete draft `draft-mangin-idr-attr-discard-00`, one citing a phantom `Section 5.10`. This
summary records the work at closure (the sibling `spec-fixit-tombstone-code-point-split`, learned
1237, later unified the wire code point to 252, which retires this spec's AC-2).

## Decisions

- **Clear only the Transitive bit, per-destination, on the eBGP re-encode.**
  `clearTombstoneTransitive` (`wireu/tombstone.go:38`) masks only `FlagTransitive` (Optional +
  Extended-Length preserved); it is called from the attribute-copy loop's `default:` case in
  `aspath_rewrite.go:528` guarded by `code == attribute.AttrTombstone`. IBGP peers keep the
  shared received wire zero-copy (marker stays 0xC0), because "forwarded within the AS via IBGP"
  must not alter it.
- **AC-2 (code 253) RETIRED, not implemented as a second case.** The sibling code-point-split
  unified the tombstone spelling to 252 and deleted 253/`isTombstoneCode`; a 253 attribute is now
  a generic optional-transitive attribute forwarded verbatim. AC-1 (code 252) is the sole
  tombstone-transitive-clear case.
- **Citations corrected to the surviving draft with verified section numbers.** All 9
  `draft-mangin-idr-attr-discard-00` -> `draft-mangin-idr-attr-tombstone-00`; the phantom
  `Section 5.10` -> `5.7` (Section 5 ends at 5.7). Section numbers were checked against the draft,
  not invented.

## Consequences

- The MUST is honoured for ordinary EBGP peers (per-destination re-encode clears the bit).
- **NOT honoured for EBGP route-server clients** (deferred Known Limitation): ze hands RS clients
  the received wire with no per-destination buffer, so the marker keeps its Transitive bit; fixing
  it costs RS zero-copy and needs a design decision.
- Deferred Known Limitations (not ACs): a configurable inherit/strip/propagate policy; the RS-client
  gap above; non-transitive markers still occupying the EBGP wire under "inherit"; the Partial bit.

## Gotchas

- Draft `Section 5.10` never existed (Section 5 ends at 5.7). When repairing normative citations,
  verify each section number exists in the referenced draft rather than trusting the stale text.
- `isTombstoneCode` was deleted by the code-point unification; the eBGP-boundary clear now gates on
  `code == attribute.AttrTombstone`. A spec/audit written before the unification names the deleted
  symbol and code 253 -- reconcile at closure.
- The implementing commit (`706b77b7d`) missed Documentation Update Checklist #7:
  `docs/architecture/wire/attributes.md` had NO ATTR_TOMBSTONE (code 252) entry at all. Filled at
  closure (a code-252 table row + an `## ATTR_TOMBSTONE` section). A wire-format change is not done
  until the wire-format doc records it.

## Files

- internal/component/bgp/wireu/tombstone.go (`clearTombstoneTransitive`)
- internal/component/bgp/wireu/aspath_rewrite.go (call the clear at the eBGP funnel for `AttrTombstone`)
- internal/component/bgp/message/attr_discard.go, rfc7606.go, reactor/session_validation.go (citation repairs)
- internal/component/bgp/wireu/tombstone_forward_test.go (boundary/IBGP/ext-length tests)
- docs/architecture/wire/attributes.md (ATTR_TOMBSTONE code-252 entry -- added at closure)
