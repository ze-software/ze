# 1319 -- wire-edit-3-aspath-fold

## Context

Two rewrite mechanisms lived in the forward path and did not compose: the edit
set from child 2, and a separate AS-path rewrite pass. Every eBGP destination
with a policy therefore paid two full payload copies, three on the route-server
rail. Child 3 makes AS_PATH, AS4_PATH, AGGREGATOR and AS4_AGGREGATOR generate
slots on the edit set, so one writer pass produces the whole UPDATE.

## Decisions

- **One resolver owns all four attributes**, over one slot per code declared by
  the producer. RFC 6793 makes them a single decision; splitting them would put
  protocol knowledge in every producer.
- **Ordered prepend INTENT, not pre-encoded bytes.** The destination's ASN width
  is not known to every producer, and AS_TRANS substitution depends on it.
- **Delete the eBGP wire caches** rather than re-key them. With no intermediate
  payload there is nothing to cache; fan-out sharing moves to child 5, which
  caches the plan rather than the intermediate.
- **Keep the old implementation as the oracle until the transform matrix was
  byte-identical.** The AS-path slow path is the most intricate code in the
  package, and a reference implementation is the only trustworthy oracle.
- **AC-1 admits one exception, approved by Thomas on 2026-08-01:** a derived
  AS4_PATH (17) or AS4_AGGREGATOR (18) is merge-inserted at its ascending
  position instead of appended after every source attribute.

## Consequences

- One payload copy per destination instead of two (three to one on the
  route-server rail), measured as read-pool borrow deltas.
- `BenchmarkFanoutRebuildOnly` puts what remains at 416 ns of rebuild against
  2.1 ns of copy, which is the measurement that shaped child 5's design.
- `adoptFwdHandle` survives with one production caller, the cross-context
  transcode buffer. That is correct, and its release ORDERING is now proven by
  `TestForwardAdoptedHandleHeldUntilLastWrite` rather than assumed.
- The unreachable eBGP wire cache is dead but not deleted; the removal is homed
  in `plan/spec-wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal.md`.

## Gotchas

- **Translating a slow path branch by branch is how you find its bugs.** Two live
  defects had survived because no test enumerated the branches: AGGREGATOR was
  destroyed on every same-width slow-path prepend, and `clearTombstoneTransitive`
  fired on one prepend path of three. Both are now named after the branch they
  cover.
- **"Delete the cache" and "delete the code that reads the cache" are different
  jobs.** The slots became unreachable the moment the rewrite moved; deleting
  them touches read-pool lifetime in `recent_cache.go`, which is a separate risk
  budget from the rewrite itself.
- **`grep` for the symbol, not for the feature.** `EBGPWire` looks alive: it is
  exported, it has a doc comment, and ten tests call it. It has zero production
  callers.

## Files

- `internal/component/bgp/wireu/aspath_slot.go`, `aspath_slot_test.go` (new)
- `internal/component/bgp/wireu/aspath_aggregator_probe_test.go` (new)
- `internal/component/bgp/wireu/aspath_rewrite.go`, `aspath_transcode.go`, `tombstone.go`
- `internal/component/bgp/reactor/reactor_api_forward.go`, `forward_rs.go`
- `internal/core/bgp/attribute/builder.go`, `builder_parse.go`
- `test/plugin/asn4-transcode-pooled-buffer.ci`, `test/plugin/bgp-rs-relay-aspath-transparency.ci`
