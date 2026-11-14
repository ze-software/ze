# Learned: an interop checker is a claim until a red under the fix's revert proves it

`spec-rfc-verdict-fix-ospf`, child 3 of `spec-rfc-verdict-test-fix-pass`, took
the OSPF group's weak and wrong RFC verdicts to `enforced`: 130 verdicts in
scope over `internal/plugins/ospf/...` and `internal/core/probe`, each unit
recorded by `./le rfc record` and re-judged through `./le rfc audit-stamp ...
mode rejudge` by a judge that wrote none of the tests. The one verdict left,
RFC3101-3.2-2, waits on a producer fix and is AC-3 of
`plan/release/fixes/spec-ospf-nssa-translator-reachability.md`. The test work
exposed product defects, fixed failing-test-first: the RFC 3623 GR helper
identifies the restarter by the Grace-LSA interface address on shared media
(`graceRestarterV4`), the RFC 7474 AuType 2 replay mark is kept per neighbour
(`replaySlotFor`), a next hop's own SR flags apply off the best path
(`(*srInstaller).forwarding`), an unwanted self-originated LSA is flushed at
MaxAge (`handleSelfReceived`), and the RFC 5709 last key stays usable
(`lastKeyExtendedIndex`). Three interop scenarios against FRR 10.7.1 went red
with those fixes reverted and green on the real build:
`ospf-sr-anycast-nexthop-frr`, `ospf-self-lsa-flush-frr` and
`ospf-accept-lifetime-last-key-frr`. The last code of the closure landed in
`3c984d6cc4`, before the closure commit: the R5-N1 case in
`check_ospf_ext_prefix_link_test.go`, two literals in `check_ospf_self_flush.go`
and `check_ospf_sr_anycast.go` replaced by existing constants, and the SPF
timer defaults in `internal/plugins/ospf/config.go`, now derived from
`spf.DefaultSPFDelay`, `DefaultSPFHold` and `DefaultSPFMaxHold`, with the
journal row for the YANG copy that remains.

## What outlives the spec

1. **Every scenario failure met was a fixture or a checker, never Ze.** Six
   OSPF scenarios failed on 2026-11-14. Each cause was in the lab: an FRR
   config without `capability opaque`, `segment-routing on` at the top level
   where FRR refuses it (`ospf_sr.c` registers it on `OSPF_NODE`), a Prefix-SID
   for a prefix on no loopback, which FRR defers without an error, a missing
   MPLS label space (`ze-sysctls`), and a probe that could not read FRR's empty
   route table. Read the peer's source for the config line before suspecting
   the product.
2. **A substring check passes on the local side's own state.** The
   `ospf-ext-prefix-link-frr` Ze-side check matched Ze's own Extended LSAs, and
   its FRR-side check matched FRR's own Extended Link LSA, whose Link ID is
   Ze's router id. The rewritten checker (`check_ospf_ext_prefix_link.go`)
   decodes the JSON and binds each opaque type, read from the first octet of
   the Link State ID (RFC 5250 Section 3), to its advertising router. Round 5
   found two more of the same shape, `ospf-te-frr` and `ospf-debug-inject-frr`
   (`plan/journal/green-that-could-not-have-been-red.md`, 2026-11-14, owned by
   `plan/release/validation/spec-interop-suite-red.md`).
3. **A first-sample read races any scheduler between the stimulus and the
   state.** The anycast checker judged the first pop it saw, installed before
   the second advertiser's Prefix-SID reached Ze's SR table, so the pre-fix
   code passed too. Ze builds the SR table in the SPF post-run hook, debounced
   up to the SPF hold ceiling, so `check_ospf_sr_anycast.go` holds the pop for
   three times that ceiling and fails on the first read that is not the pop.
   With that hold the revert went red at assertion 3.
4. **The revert is cheap through a Go build overlay.** Every red was built with
   `GOFLAGS=-overlay=<json>` passed to the lab's host build, so no tracked file
   changed, and the staged binary hash and the image id differed between each
   red and its green. A green that the overlay leaves green proves the checker,
   not the fix, is wrong.
5. **A whole-body revert proves reach, not the clause.** The first record sweep
   replaced each producer's body with a panic, and the non-author judges found
   33 of 64 requirement ids whose record named a setup writer, a fixture or a
   decoder. Records were re-targeted at the producer of the clause, and two of
   those units were hiding the GR identity and NSSA-external scope defects.
