# 154 — Hub Phase 3: YANG Integration

## Objective

Integrate YANG validation into the Config Reader so parsed config is type-checked against the combined YANG schema before being sent as verify requests to the Hub.

## Decisions

- Chose goyang over libyang — pure Go, no cgo, sufficient for type and range validation without needing a complete YANG implementation.
- Path-based validation: `Validate("bgp.local-as", "65001")` rather than validating entire document at once — matches how Config Reader sends block-by-block verify requests.
- Permissive on unknown types: unrecognized YANG types pass through validation rather than failing. Enables forward compatibility as new YANG types are added.
- Range expressions parsed directly from YANG string format (`"0 | 3..65535"`) — avoids pre-processing step.

## Patterns

- `checkRangeString(num, rangeExpr)` handles pipe-separated disjoint ranges, enabling hold-time's non-contiguous valid range (0 or 3..65535).

## Gotchas

- Leafref validation deferred — requires runtime state (what peer-groups actually exist) which isn't available at parse/validate time without a live query mechanism. Marked as future work.
- Config Reader integration deferred to Phase 4 — the verify/apply routing infrastructure is needed first.

## Files

- `internal/yang/validator.go` — YANG validator with type, range, pattern, enum validation
- `internal/yang/loader.go` — extended (goyang already added in spec 151)
