# Requirement met on the rails the spec planned

A protocol rule is centralized at one site so no two producers can disagree, and
every producer the spec enumerated asks there. The requirement is then recorded
as met. The set that was enumerated is the set the spec set out to change, not
the set that WRITES the artifact the rule governs, so a producer nobody listed
keeps emitting the thing the rule forbids.

The centralization is what hides it. One site answering the question reads as
completeness, and the tags on the tests that drive the listed producers are each
honest about what they assert. Nothing in the tree compares the set of askers
against the set of writers, so the gap lives entirely in the sentence the
summary writes, and that sentence is what the public ledger publishes.

`ai/rules/principles.md` states the general form: the work a change owes is
measured by what the change can now REACH, never by the files it edited. Here
the measure to use is different again, and narrower: before recording a MUST as
met, enumerate every function that WRITES the artifact the requirement governs,
and show that each one reaches the site that answers.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-05 | srv6-ebgp-egress-filter | bgp egress, the BGP Prefix-SID attribute against RFC 8669 Section 8 | `prefixSIDAllowedTo` (`internal/component/bgp/reactor/forward_prefix_sid.go`) is the single site that answers "may this destination be sent attribute 40", and four producers ask it: `forwardUpdateCore`, `reactorForwardRS`, `buildStaticRouteUpdateNew` and `toPluginParams`. A fifth writes UPDATEs and does not. `buildBatchAnnounceUpdate` (`internal/component/bgp/reactor/reactor_api_batch.go`) copies the caller's attribute block verbatim into `plan.emit` and its only removal is `plan.drop` of LOCAL_PREF, so attribute 40 in that block reaches an external peer with no leaf set. It carries the API announce, the grouped announce and `sendStaleReadvertise`, the RFC 9494 stale readvertise, whose base is a route's received attribute block. The spec knew the rail was ungated and still took the `{gap}` annotation off RFC8669-8-1 and wrote `docs/features/srv6.md` as "removed from every UPDATE sent to an EBGP peer", so the ledger reported a proven MUST for a week. The same package produced this shape once before, which is what `localPrefAllowedTo` was written to end | not fixed: the rail needs one bool on `announceBuildKey` and on the parameter list of `buildBatchAnnounceUpdate`, and widening it edits `TestAnnounceStripsLocalPrefTowardExternalPeer`, an `RFC requirement: RFC4271-5.1.5` carrier, which only the owner approves through `test/rfc-changed.md`. Closure measured the leak rather than inferring it, with `TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain` (`internal/component/bgp/reactor/zzprobe_prefixsid_announce_test.go`), red at HEAD; restored the `{gap}` and the Meta count ten to eleven in `rfc/short/rfc8669.md`; corrected two rows of `docs/features/srv6.md`; and homed the remainder at `plan/immediate/spec-prefix-sid-announce-rail-boundary.md` |
