# 1161 -- BGP Export Filter Applied Twice to Forwarded Routes

## Context

`aaefef8ce` made a check-mode ze-peer govern its own verdict, unmasking 69 `.ci`
reds that had been passing on their exit code while their peer assertion never
ran. `plan/spec-fixit-redistribute-establishment-stall.md` classified 6 of
them as "possible PRODUCT BUGS, highest value" and fanned them out one agent per
test. Five turned out to be bad tests. This one was real: a forwarded route ran
its peer's export filter chain TWICE, and where the second pass did damage it
did so silently, on the wire, only for peers whose local AS happened to be in
the RFC 6996 private range.

## Decisions

- **Split `writeUpdate` rather than gate inside the filter.** `writeUpdate` keeps
  its semantics for originated / injected / replayed routes (which have NOT been
  filtered); `writeUpdatePreFiltered` skips the egress gate for forwarded routes
  (which have); `writeUpdateGated` is the shared body. Chosen over a flag on the
  filter or a caller-side check because the exemption is a property of the CALLER
  (has this wire already been through the chain?), not of the filter.
- **This is the same exemption `rawBodies` always had.** The forward pool's
  rawBodies half already bypassed the gate via `writeRawUpdateBody`. The bug was
  that the `updates` re-encode half did not. The fix makes the two halves agree
  rather than inventing a new rule.
- **Fixed the input, not the expectation, where a test fed RFC-invalid data.**
  Two of the six tests sent LOCAL_PREF on an eBGP session (RFC 4271 Section 5.1.5
  forbids it) and then asserted it survived. `bgp-rs-fastpath-ebgp-shared` and
  `remove-private-as-replace-peer` removed the invalid attribute from the input;
  `remove-private-as-export` instead blessed the resulting ATTR_DISCARD marker in
  its expectation. **The repo is now inconsistent on this point** -- see Gotchas.

## Consequences

- Any export filter is now applied exactly once to a forwarded route. Before this,
  `as-path-prepend` would have prepended twice and `remove-private-as` rewrote ze's
  own local AS. The class is "any export filter, any forwarded route", not just
  remove-private-as.
- A private local AS is no longer self-destructive. Because RFC 6996 reserves
  64512-65534 and lab configs routinely sit there, the second pass could not tell
  ze's own just-prepended AS from a genuine private ASN in the path.
- `remove-private-as-export`'s committed AS_PATH expectation (`[65000 64496 64497]`)
  was UNSATISFIABLE while the bug stood -- strip mode removed the 65000 it demanded.
  The test was right for two years and never ran.

## Gotchas

- **A doc comment stated the invariant that the code broke.** `egress_inject_filter.go`
  already said "the already-filtered forwarded path ... excluded by the callers". It
  was true for one of the two forward-pool halves. A comment describing an exemption
  is not the same as an exemption; grep every caller before trusting it.
- **The bug needed THREE conditions to show**: a forwarded (not originated) route, an
  export filter configured on the destination peer, and a local AS that the filter
  itself would act on. Miss any one and the double-application is invisible. That is
  why it survived: `exportFilterForBody` returns at `:40` when `ExportFilters` is
  empty, so most tests could never see it.
- **`.ci` test numbers are unstable in a shared tree.** Discovery indices shift as
  other sessions' edits change `.ci` parseability -- one run of "365" executed 368.
  Verify by NAME, never by number.
- **Inconsistent handling of RFC-invalid LOCAL_PREF now exists in the repo.**
  `remove-private-as-export.ci` expects the ATTR_DISCARD marker (`C0FD0405010000`);
  its siblings remove the invalid attribute from the input instead. Both pass. The
  input-side fix is the better precedent: it keeps the test's subject (AS_PATH
  rewriting) free of an orthogonal RFC-7606 concern. Reconcile when next touched.
- **UNVERIFIED, worth a look**: whether ze should propagate the optional-transitive
  ATTR_DISCARD marker onward to an eBGP peer with the Transitive bit set. Two agents
  independently flagged it and neither could resolve it from the draft.
- **UNVERIFIED, and ze is self-inconsistent**: `message/attr_discard.go` uses code
  point 253 citing `draft-mangin-idr-attr-discard-00`, which does NOT exist in
  `rfc/drafts/`; `core/bgp/attribute/attribute.go` + `wireu/tombstone.go` use
  252 citing `draft-mangin-idr-attr-tombstone-00`, which does. Same mechanism, two
  code points. The draft marks it "TBD (IANA pending)" so neither is authoritative,
  but they must agree with each other.

## Files

- Modified: `internal/component/bgp/reactor/session_write.go` -- `writeUpdate` /
  `writeUpdatePreFiltered` / `writeUpdateGated` split
- Modified: `internal/component/bgp/reactor/{forward_pool,forward_rs}.go` -- use the
  pre-filtered write for the re-encode half
- Modified: `internal/component/bgp/reactor/egress_inject_filter.go` -- corrected the
  stale exemption comment
- Pinned by: `test/plugin/remove-private-as-{export,replace-peer}.ci` (committed
  separately in `e4178535f` by a concurrent session)
