# 027 — AS Path Length Validation

## Objective

Add RFC 4271/5065 AS_PATH validation (segment type checking, max-length limit) to protect against malformed updates and DoS.

## Decisions

- Max AS_PATH total length set to 1000 ASNs (constant `MaxASPathTotalLength`), which is a practical DoS-protection threshold.
- Confederation types (AS_CONFED_SEQUENCE, AS_CONFED_SET) are excluded from the path-length count, consistent with RFC 5065.

## Patterns

None beyond standard attribute validation.

## Gotchas

- Pre-existing RFC 5065 bug: `ASConfedSet` was 3, `ASConfedSequence` was 4 — values were swapped. Fixed during this spec.
- Discovering the wrong constants required updating existing wire tests that depended on the incorrect values.

## Files

- `internal/bgp/attribute/aspath.go` — validation logic and constants
- `internal/bgp/attribute/aspath_test.go` — 7 new tests
- `internal/bgp/attribute/as4_test.go` — fixed wire data for AS_CONFED_SEQUENCE
