# 047 — MUP API Fixes

## Objective

Fix post-implementation issues in the MUP API: add family mismatch validation, add unit tests for helper functions, and document deferred work (T1ST/T2ST encoding, code dedup).

## Decisions

- Family mismatch validation (IPv6 prefix with IPv4 AFI) added after prefix/address parsing in all four route types (ISD, DSD, T1ST, T2ST). Produces a clear error message.
- Code duplication between `parseAPIPrefixSIDSRv6()` (reactor) and `ParsePrefixSIDSRv6()` (config/routeattr) was accepted rather than refactored — the two callers have different string-argument conventions, and extracting shared logic adds complexity without clear benefit.
- T1ST/T2ST TEID/QFI/endpoint encoding deferred: the draft spec format is not clearly specified in available documentation.

## Patterns

None new — follows established patterns from MUP static route encoding.

## Gotchas

- `parseAPIPrefixSIDSRv6()` in reactor.go and `ParsePrefixSIDSRv6()` in config/routeattr.go are near-duplicates but have subtly different argument formats. Any future refactoring must account for this.
- The family mismatch produces malformed wire format (IPv6 bytes with AFI=1) with no error — critical silent bug before this fix.

## Files

- `internal/reactor/reactor.go` — family validation in `buildAPIMUPNLRI()`
- `internal/reactor/mup_test.go` — 24 unit tests for helper functions (new file)
