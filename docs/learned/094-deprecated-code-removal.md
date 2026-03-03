# 094 — Deprecated Code Removal

## Objective

Remove functions superseded by zero-allocation alternatives, eliminating dead code and the tests that compared old to new.

## Decisions

- Mechanical refactor, no design decisions. The zero-allocation replacements already existed and were verified equivalent; removing the deprecated originals was purely cleanup.

## Patterns

- Comparison tests (`TestWriteX_MatchesBuildX`) exist only during a transition period — once the new path is proven correct, delete both the old function and the comparison test together.
- Test helper `mustBuildGrouped` replaces direct `BuildGroupedUnicast` calls in tests that need grouped UPDATE construction; routes through `BuildGroupedUnicastWithLimit(routes, 65535)`.

## Gotchas

- None. Straightforward removal with no production callers.

## Files

- `internal/bgp/message/update_build.go` — `BuildGroupedUnicast` removed
- `internal/reactor/reactor.go` — `buildAnnounceUpdate`, `buildWithdrawUpdate`, `buildAnnounceUpdateFromStatic` removed
- `internal/reactor/reactor_test.go` — six comparison tests and one benchmark removed
