# 1351 -- A Comment That Asserted Safety Kept A Remote Denial Of Service Open

## Context

The community egress handler answered "does a Remove operation name this value"
by scanning the removal operations once per source value. Both operands are
peer-controlled on the route-server forward path, because
`StripControlCommunities` (`internal/component/bgp/wireu/community.go`) derives
the removal buffer from the forwarded route's OWN COMMUNITY attribute rather
than from configuration. One route tagged with 16383 control communities
therefore sized both sides together and measured 874 to 889 ms of CPU per
destination peer, multiplied by the fan-out. The cost cap had been an unrelated
arity guard, `len(toRemove) != valueSize`; removing that defect removed the cap
with it. The row sat open from 2026-07-28 to 2026-08-05.

## Decisions

- Chose one membership structure per attribute, selected above the loop, over a
  faster per-value comparison: the defect is the PLACEMENT of the decision, not
  the speed of the answer. Anything chosen inside the loop is quadratic again.
- Chose a map keyed by the value bytes over a sorted set with binary search: the
  handler receives operation buffers from any producer and cannot assume they
  arrive sorted, and sorting per destination reintroduces per-destination cost.
- Thresholded on `min(source values, removal values)` over the removal count
  alone: the scan costs n*m, so either operand being small already keeps it
  cheap, and thresholding on one side indexes a large set against a two-value
  attribute and pays for a map nothing reads.
- Set the threshold at 32 from the crossover arithmetic (a map operation is
  roughly twenty times a `bytes.Equal` over 4 to 12 bytes, so indexing pays once
  `n*m > 20*(n+m)`, which for n equal to m is n above 40), over picking a round
  number: the common shapes, one configured strip value or a route carrying
  three control communities, must stay on the scan and allocate nothing.
- Let the map collapse duplicates rather than deduplicating at the producer as
  the deferral row proposed: the index does it for free. An earlier candidate
  fix built a set per destination with no deduplication and was 326 times better
  on the worst shape but 16.5 times worse, and up to a megabyte, on a
  duplicate-heavy one. It was reverted.

## Consequences

- The per-destination cost is now O(n + m) instead of O(n * m). The fan-out
  multiplier survives, because the handler still runs per destination; removing
  that too would mean hoisting the structure into the reactor, which is a wider
  change than the defect needs.
- A new producer of `AttrModRemove` buffers inherits the protection without
  knowing it exists. Nothing at the call site has to opt in.
- `removalSet.indexed()` exists only for the regression guard. Deleting it as
  "unused" deletes the only thing that can witness the fix.

## Gotchas

- **A comment can assert a safety property, be wrong, and read exactly like a
  measurement.** `containsValue` carried "the sets here hold a handful of values
  (the control communities on one route, or one configured strip value), so
  building a map per attribute would cost more than it saves". The premise is
  false on the route-server path and the conclusion was a remote denial of
  service. The original comment is quoted in the file rather than deleted,
  because it is what kept the defect open (`ai/rules/evidence.md`: a comment is
  its author's belief, not a decision record).
- **A rewrite can delete the function a deferral row names and keep the defect.**
  The row pointed at `removeValues`. That helper no longer exists. Searching for
  it returns nothing and the row looks stale, but the per-value call had simply
  moved into `removedByAny` and stayed inside the loop. Verify a row against the
  BEHAVIOR it describes, never against the identifier it happens to name.
- **An owner direction can expire on its own premise.** The 2026-07-28 direction
  was "this handler is being replaced wholesale, so a half-finished cost trade is
  not worth shipping". The replacing spec closed on 2026-08-02 without touching
  this code, so the premise was gone and the work was owed again. A direction
  conditioned on future work needs re-reading when that work lands.
- **A guard on elapsed time is the wrong guard, twice over.** It flakes on a
  loaded host, and it passes on a quiet one with the fix deleted. The guard here
  asserts which REPRESENTATION `newRemovalSet` picked, which is deterministic.
  An assertion on the retained values could not work at all: the two
  representations agree on every answer, which is the point of the change, so
  only the representation itself can witness it.
- **Mutation-verify the threshold expression, not just the branch.** Forcing the
  scan and thresholding on the removal count alone are different defects, and
  they fail different subtests. One mutant per claim.

## Files

- `internal/component/bgp/plugins/filter_community/handler.go` -- `removalSet`,
  `newRemovalSet`, `removalIndexThreshold`, and the corrected `containsValue` comment
- `internal/component/bgp/plugins/filter_community/handler_test.go` -- four tests
  pinning the representation, the agreement between representations, deduplication,
  and the malformed-buffer exclusion
