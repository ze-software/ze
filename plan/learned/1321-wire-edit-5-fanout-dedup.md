# 1321 -- wire-edit-5-fanout-dedup

## Context

One received route fanned out to N destinations did per-destination work N times.
The existing forward-body cache keyed on the materialised wire POINTER, so it
could only dedupe destinations that had already produced the same pointer --
sharing downstream of the copy it was meant to avoid. Child 5 fingerprints the
edit set instead, which is upstream of the copy, and confirms every hint with a
full equality check.

## Decisions

- **Share the PLAN, not the BUFFER.** The spec assumed a shared materialisation
  referenced by several forward items and spent two assumptions, one risk and
  half its Blast Radius making that safe. `BenchmarkFanoutRebuildOnly`, run
  before any of that code, measured the rebuild at 416 ns and a flat copy of its
  result at 2.1 ns. Sharing the plan and copying per destination keeps 99.5% of
  the win and leaves the one-buffer-one-item ownership model untouched.
- **The fingerprint is a hint; full equality decides.** No fast-path bypass. A
  collision would send one destination another destination's wire, which is the
  worst failure in the whole umbrella.
- **The identity carries the BASE as well as the digest.** Two destinations in
  one policy group can have identical edit sets over DIFFERENT bases, reachable
  through a filter-supplied raw export override.
- **No adaptive threshold, deliberately.** Any cutoff L silently disables sharing
  for G >= L, which is the silent cap `ai/rules/completion.md` and the umbrella
  both ban. The trade is +3% worst case for -29% best case, and it is recorded
  rather than hidden.

## Consequences

- Measured per-destination cost, interleaved A/B, medians of 6: (2,1) -10.5%,
  (10,2) -14.4%, (100,2) -28.6%. And (2,2) +3.3%, (100,100) +2.8% where no two
  destinations share a group. Allocations per operation identical in both arms.
- The route-server rail's four-slot cache, which silently stopped caching beyond
  four bodies, is gone. There is no cap left to be silent about.
- About 120 ns per destination (`buildFwdBody` plus the wire wrapper) is NOT
  recovered. Taking it needs the shared buffer this design exists to avoid.
  Measured and deliberately left.

## Gotchas

- **Measure the thing you are about to make safe.** Half the spec was risk
  mitigation for a shared buffer worth 0.5% of the win.
- **A guard field that no test exercises is decorative.** `fwdDedupTable.begin`
  guards on `e.fp != fp || e.id != id`, and the entire reactor suite stayed green
  with the base half removed -- only one test built an identity, and it passed
  the same base for both entries. Mutate each half of a compound guard and
  confirm something goes red.
- **Reaching a real read-pool borrow needs an IBGP destination on a 2-byte send
  context.** An eBGP one folds the RFC 6793 width change into the edit set, so
  the wire is rebuilt and nothing is borrowed. A fixture that does not pin all
  three preconditions goes quiet instead of failing.
- **An ordering assertion needs a happens-before, not a timer.** A sentinel batch
  dispatched to the same peer's worker cannot be handled until the forwarded
  batch's `done()` has run, because the worker takes one batch at a time.
- **`reject=bgp:` inside a `stdin=peer` block asserts nothing.** Neither the
  runner's peer-block parser nor the peer expectation reader consumes it. Three
  live sites; homed in `plan/spec-fixit-ci-peer-block-silent-directives.md`.

## Files

- `internal/component/bgp/filterapi/fingerprint.go`, `fingerprint_test.go` (new)
- `internal/component/bgp/reactor/forward_dedup.go`, `forward_dedup_test.go`, `forward_dedup_bench_test.go` (new)
- `internal/component/bgp/reactor/forward_rs.go`, `reactor_api_forward.go`, `reactor.go`
- `internal/component/bgp/reactor/forward_readbuf_leak_test.go`
