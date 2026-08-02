# 1320 -- wire-edit-4-api-origin

## Context

An announce reached the wire through one of two builders, and which one ran was
decided by scheduling rather than by the route: the batch rail if the peer had
finished its initial sync, the queued rail if it had not. The batch rail copied
the caller's attribute block verbatim and APPENDED, so one route went out as
1,8,32,2,3,5 there and 1,2,3,5,8,32 through the drain. Child 4 makes an
API-originated route an edit set over an empty base, so both rails share one
writer and cannot disagree about bytes.

## Decisions

- **An API route is an edit set over an empty base**, over keeping a separate
  announce encoder and testing the two against each other. Testing two encoders
  against each other is what the tree did, and it cost a permanent test pair plus
  a class of timing-dependent defects.
- **Keep the `Builder` setter chain, delete only its emission half.** The setters
  are a good intent-collection surface and are used widely; `WriteTo` and
  `CheckedWriteTo` were the duplicated work, and `Build` is reimplemented over
  `AppendAttributes`.
- **The NLRI region bound is an explicit writer argument**, over giving the
  queued rail its own bounded wrapper. The bound is a real property of the call,
  not a rail quirk, so making it explicit means a future third caller cannot
  misuse the writer.
- **Share child 3's AS_PATH resolver**, over keeping the announce-side AS4_PATH
  insertion. RFC 6793 derivation implemented twice is two chances to be wrong,
  and the announce rail originates rather than relays.
- **Keep the rail-agreement tests** rather than deleting them once the encoder is
  shared. They are cheap and they are the only thing that would catch a future
  re-divergence.

## Consequences

- `findAttrInsertPosition` and `insertAttrOrdered` are gone. Ordering machinery
  added to one rail was a symptom of the divergence, not a fix for it.
- Two rails still exist and `ShouldQueue` still selects between them; they simply
  cannot disagree about bytes any more.
- The `Builder` raw-wire escape hatch remains, which is how flowspec and other
  pre-encoded attributes pass through untouched.
- The announce carries one allocation the forward path does not: its
  `*message.Update` return value. Recorded rather than hidden.

## Gotchas

- **Convergence found a defect neither rail's tests covered.** LOCAL_PREF reached
  eBGP peers on the batch rail, an RFC 4271 Section 5.1.5 MUST. Forcing both
  rails through one writer forced a single answer to a question neither had been
  asked. Expect a convergence to be a fix as well as a deletion, and widen the
  blast radius accordingly.
- **Two green rail-agreement tests do not mean the rails are right.** They meant
  the rails agreed on every case someone had thought to test.
- **A single-peer convergence benchmark cannot see this work.** `ze-perf-bench`
  has almost no fan-out; the per-route encoder cost was measured with
  `BenchmarkAPIOriginVsForward` instead.

## Files

- `internal/component/bgp/reactor/reactor_api_batch.go`, `reactor_api_forward.go`, `announce_build.go`
- `internal/component/bgp/reactor/reactor_api_origin_test.go`, `reactor_api_origin_bench_test.go` (new)
- `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go`
- `internal/core/bgp/attribute/builder.go`, `attribute.go`
- `test/plugin/wire-edit-api-origin-order.ci` (new)
- `docs/features/rfc-status.md`
