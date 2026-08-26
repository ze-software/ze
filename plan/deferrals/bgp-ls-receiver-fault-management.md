# Deferrals -- spec-bgp-ls-receiver-fault-management

Rows the spec names in its Known Limitations, homed here so they outlive it. The
spec closes four MUST-level rows of `rfc/short/rfc9552.md`; these are the
SHOULD-level neighbours it deliberately leaves, and a SHOULD is not gated by
`make ze-rfc-check`, so nothing else would record them.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-26 | spec-bgp-ls-receiver-fault-management | RFC 9552 §8.2.3's receiver-side SHOULDs: let the operator specify the neighbors from which Link-State NLRIs are ACCEPTED, and the maximum number of Link-State NLRIs stored in the RIB. The declared BGP-LS role this spec adds is an all-or-nothing import drop, which is what §8.2.6's MUST asks for and less than §8.2.3 suggests. | The MUST is what the gate holds ze to and what Thomas asked for on 2026-08-26. A per-neighbour accept list and a per-family RIB ceiling are each their own config surface, and the RIB ceiling overlaps `RFC9552-8.2.6-1`, whose own `{gap}` says the bgp-ls prefix maximum is compared against a count `countPrefixEntries` derives from a CIDR walk that never parses a type-length Link-State NLRI. Folding them in would put three config surfaces and one counting defect into a spec whose subject is fault management. | a spec of their own, once `RFC9552-8.2.6-1`'s counting defect is scoped | deferred |
| 2026-08-26 | spec-bgp-ls-receiver-fault-management | RFC 9552 §8.2.3's producer-side SHOULDs: the advertisement rate limit, the abstracted-topology controls, and the 4096-byte BGP-LS UPDATE size limit. | Ze originates no BGP-LS, so none of the three has a producer to limit. They become live with `plan/spec-bgp-ls-origination-and-the-scheduled-marker.md` Phase 2 and not before. | plan/spec-bgp-ls-origination-and-the-scheduled-marker.md | deferred |
