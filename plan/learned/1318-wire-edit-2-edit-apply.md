# 1318 -- wire-edit-2-edit-apply

## Context

`ModAccumulator` recorded "what to change" but had no word for "how the new value
relates to the old one", so every handler that kept most of an attribute rebuilt
the whole value in an intermediate buffer. No exact size was ever known, so the
output buffer was over-sized by a fixed 256-byte slack and overflow was handled
by abandoning every modification -- which the caller could not distinguish from
"nothing to modify", and so forwarded the route UNMODIFIED. Child 2 gives the
accumulator the missing word: slots, fragments, an arena, and an exact size.

## Decisions

- **Fragment lists as the slot value**, over composing each touched attribute's
  final value into the arena. Composing copies the MP_REACH NLRI tail twice.
  `mpReachNextHopHandler` already avoided that by hand; fragments generalise it.
- **A handler PLANS, it does not write.** `AttrPlan` answers "how many bytes will
  you write" before the buffer exists. That is what makes the size exact.
- **Three slot kinds, including a generate resolver**, over fragments only. An
  ASN4 transcode re-encodes every ASN and cannot be a fragment list; staging it
  through the arena would restore the double move. The kind is defined here so
  child 3 needs no writer change.
- **Merge-insert a new attribute at its ascending type-code position**, over
  appending at the end. This is a deliberate WIRE CHANGE, approved by Thomas on
  2026-08-01: RFC 4271 Section 5 orders attributes ascending on emission, so the
  previous append order was the defect.
- **Oversize suppresses the route**, over forwarding it unmodified with a log
  line. Forwarding unmodified leaks exactly what the policy exists to strip.
- **Constant-time reset with the accumulator hoisted above the destination
  loop.** A 500-byte inline value re-zeroed per destination would have made this
  child a regression, which is why the hoist was a precondition and not an
  optimisation.

## Consequences

- The fail-open overflow branch is gone rather than merely louder. Exact sizing
  is what made deleting it possible.
- `EditSet.Spilled()` reports slot, fragment and arena spill separately, so the
  static census behind the three inline capacities is answerable from a running
  daemon.
- Child 3 could fold the AS-path family in without touching the writer, and child
  4 could make an API announce an edit set over an empty base.
- A registered `AttrModHandler` written against the old write-directly contract
  no longer compiles. That break is deliberate and documented.

## Gotchas

- **The handler contract change reaches RFC-tagged tests.** Three call sites in
  `rfc8277_test.go` and `otc_test.go` carry `RFC requirement:` tags, and the edit
  hook refuses them without Thomas's approval. While they stayed on the old
  shape, `reactor` and `role` did not compile their test binaries, so all 95
  tagged requirements in those two packages proved NOTHING. Prove such an edit
  safe with `go test -overlay` before asking, then ask.
- **A route-reflector `.ci` cannot discriminate merge-insert from append.** RR
  adds ORIGINATOR_ID (9) and CLUSTER_LIST (10) to a base of 1, 2, 3, 5, so
  appending is already ascending. The announce case discriminates, because it
  injects 2, 3 and 5 before the caller's 8 and 32.
- **Exactly one golden moved**, `set-local-pref-and-add-med`. Any future golden
  that moves for a reason other than merge-insert ordering is a stop-and-report.
- **A reactor path must read the injected clock.** The suppression rate limiter
  was written with `time.Now()` and reddened `TestNoDirectTimeCalls`. The textual
  gate is a grep, so the behavioural test is the one that matters.

## Files

- `internal/component/bgp/filterapi/editset.go`, `editset_test.go` (new)
- `internal/component/bgp/filterapi/filterapi.go`, `metrics.go`
- `internal/component/bgp/reactor/forward_build.go`, `forward_build_merge_test.go` (new)
- `internal/component/bgp/reactor/forward_modify_failure.go`, `filter_delta_handlers.go`, `filter_ordered.go`
- `internal/component/bgp/plugins/filter_community/handler.go`, `internal/component/bgp/plugins/role/otc.go`
- `test/plugin/modify-oversize-suppress.ci`
