# 048 — Labeled-Unicast API Completeness

## Objective

Complete labeled-unicast API to match the `AnnounceRoute` pattern: create `nlri.LabeledUnicast` type, add Adj-RIB-Out tracking, transaction mode, non-established peer queuing, and ADD-PATH PathID support.

## Decisions

- `nlri.LabeledUnicast` was the blocking prerequisite — `rib.Route` requires an `nlri.NLRI` implementation. Without this type, `buildRIBRouteUpdate` could not replay queued labeled-unicast routes.
- Wire format consistency test between `nlri.LabeledUnicast.Pack(ctx)` and `buildLabeledUnicastNLRIBytes()` was added to guarantee the two code paths produce identical bytes.
- Pre-existing `AnnounceRoute` bug: stores only `OriginIGP` in `rib.Route` attrs, losing MED/Communities/etc on queued replay. Labeled-unicast was explicitly NOT allowed to repeat this bug — all attributes stored.
- `LocalPref` was discovered missing from `buildLabeledUnicastRIBRoute` during implementation; added.
- ADD-PATH bug in `buildLabeledUnicastNLRIBytes`: `pathID=0` was not encoded when ADD-PATH is negotiated — fixed as part of this spec (RFC 7911 compliance).

## Patterns

- Three-way switch pattern for route injection: `InTransaction → QueueAnnounce`, `Established → SendUpdate + MarkSent`, `default → peer.QueueAnnounce`.
- RFC 8277: Label NLRI length field counts label bits (24 per label) + prefix bits — not bytes. BOS bit (S=1) set on last label in stack. NO TTL in BGP label encoding (unlike MPLS data plane).

## Gotchas

- `LabeledUnicastParams` only supports a single label (not a label stack) for immediate send; queued replay via `nlri.LabeledUnicast` preserves the full stack.
- Transaction mode, Adj-RIB-Out tracking, and non-established queuing were implemented in code but not covered by unit tests — documented as "code complete, no unit test".
- The `AnnounceRoute` attribute-loss bug (stores only `OriginIGP`) is pre-existing and was not fixed here — documented as a separate known issue.

## Files

- `internal/bgp/nlri/labeled.go` — `LabeledUnicast` type (new file)
- `internal/bgp/nlri/labeled_test.go` — NLRI tests
- `internal/bgp/message/labeled_wire_test.go` — wire consistency tests
- `internal/bgp/message/update_build.go` — fixed ADD-PATH pathID=0 bug
- `internal/plugin/types.go` — `PathID` field added to `LabeledUnicastRoute`
- `internal/reactor/reactor.go` — `AnnounceLabeledUnicast` refactored with 3-way switch
