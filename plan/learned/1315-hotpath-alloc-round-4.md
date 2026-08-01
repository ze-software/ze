# 1315 -- hotpath-alloc-round-4

## Context

Research for `spec-wire-edit-0-umbrella` produced eleven findings on the BGP
forward path. Seven are removed by construction when that umbrella lands, so
fixing them here would have been done twice and would have conflicted with a
spec that rewrites the same functions. This spec split the list in two and fixed
only the first tier: a fail-open modification path, an unpooled transcode
buffer, a per-destination accumulator, a duplicate receive-time attribute walk,
and (found while testing the fourth) a wire-visible RFC 8669 Section 4
violation where a second Prefix-SID attribute survived onto the wire.

## Decisions

- **T1-2 was implemented with an adopt-handle, not the `defer Put` the spec
  specified.** A-3 assumed the transcode buffer fit `acquireModBuf`'s tiering.
  It does not, and the reason is LIFETIME rather than size: `message.UnpackUpdate`
  is zero-copy, and both re-encode helpers return their input unchanged in the
  ordinary case, so every field of the returned `*message.Update` points into the
  transcode buffer, which a forward-pool worker then writes to TCP
  asynchronously. A pooled buffer released at function exit would have put
  another route's bytes on the wire. It is released at cache eviction instead,
  via `adoptFwdHandle`, the shape three existing sites already use.
- **The pool class is chosen from the REQUIRED size, not the payload size**,
  because `wireu.TranscodeASPath` never bounds-checks its destination: it
  truncates through `copy` and panics through `PutUint32`. Above
  `message.ExtMsgLen` no class fits and the site deliberately keeps a
  collector-owned `make`, which needs no handle.
- **Making the return value mandatory beat auditing call sites by hand.** The
  spec's A-2 named three callers of `buildModifiedPayload`. There are five:
  `forward_rs.go` and `reactor_api_batch.go` were never listed. Changing the
  signature so the failure could not be ignored turned the compiler into the
  enumerator and found both.
- **T1-3 landed on a precondition argument, not a measured win**, and this
  spec claims no throughput improvement from it.

## Consequences

- `ModAccumulator.Reset` is now an isolation boundary, not a micro-optimization.
  The inline arena is shared across destinations, so the first `OpCopy` caller
  wired into either forward rail inherits an obligation that did not exist
  before the hoist. That obligation is stated on `Reset` rather than left
  implicit, and `TestAccumulatorResetClearsEverything` sweeps every struct field
  by reflection so a field added later without a clear fails it.
- `Reset` was deliberately NOT made to clear more. `a.ops = a.ops[:0]` leaves
  dropped entries beyond `len`, unreachable through the accessors, and clearing
  them would not close the real leak vector, which is a CONSUMER retaining an op
  slice. It would only add per-destination work against a value the umbrella
  grows.
- Umbrella child 2 is unblocked: it consumed T1-1 and T1-3 as preconditions.

## Gotchas

- **`ze-perf-bench` cannot answer any question about the per-destination loop.**
  It is single-peer with almost no fan-out, so `forwardUpdateCore`,
  `ModAccumulator`, `buildModifiedPayload` and `groupOpsByCode` appear NOWHERE
  in a 300-node profile. Total samples were 1.41s of 30s. The profiling gate in
  this spec's R-5 is therefore UNANSWERABLE as written, and dropping an item on
  that absence would be dropping it on absence of evidence. A benchmark with
  fan-out is owed; it belongs to `spec-perf-next-0-umbrella`, which owns
  methodology. This is the most reusable finding of the effort.
- The spec said three exit paths in `buildModifiedPayload` return nil after work
  has begun. There are ten.
- T1-4 saves a walk only on eBGP sessions with Prefix-SID acceptance off. The
  gate was always there; the umbrella states the saving unconditionally.
- `test/draft/*` is gitignored, so a `.ci` left there is invisible to CI AND
  absent from every commit. Two of this spec's four wiring tests sat there and
  would have been destroyed at closure rather than merely unproven. Promote
  before closing, or the deliverable does not exist.

## Files

- `internal/component/bgp/reactor/forward_modify_failure.go` (new), `forward_build.go`,
  `filter_ordered.go`, `reactor_api_forward.go`, `forward_rs.go`, `reactor_api_batch.go`
- `internal/component/bgp/reactor/forward_body.go`, `session_validation.go`
- `internal/component/bgp/filterapi/filterapi.go`
- `internal/component/bgp/message/attr_discard.go`, `rfc7606.go`
- `test/plugin/modify-oversize-suppress.ci`, `modify-accumulator-per-peer-isolation.ci`,
  `asn4-transcode-pooled-buffer.ci`, `prefixsid-ebgp-discard-single-walk.ci`
