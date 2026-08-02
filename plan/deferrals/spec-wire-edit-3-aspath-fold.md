# Deferrals: spec-wire-edit-3-aspath-fold

Source: `plan/spec-wire-edit-3-aspath-fold.md`. Format: `ai/rules/deferral-tracking.md`.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-02 | `spec-wire-edit-3-aspath-fold` | AC-9 is not met: `ReceivedUpdate.EBGPWire`, `ebgpSlotASN4`, `ebgpSlotASN2`, `ebgpWireSlot` and their two release branches in `recent_cache.go` are unreachable but not deleted. `grep -rn "\.EBGPWire(" internal/ cmd/ pkg/` returns test files only, so the slots are never populated in production and the release branches are no-ops. | No behavioural effect: the cache is correct because nothing stores a handle in the slots. The deletion touches read-pool buffer lifetime in `recent_cache.go`, which is this spec's own R-2 and R-3 area, so it wants an implementation phase with the poison-read soak rather than a closure-time edit. The goal this child exists for -- one payload copy per destination instead of two -- is met (AC-2). | `plan/spec-wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal.md` | deferred |
