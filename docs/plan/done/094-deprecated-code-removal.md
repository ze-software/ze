# Spec: Deprecated Code Removal

## Task
Remove deprecated functions that have been superseded by zero-allocation alternatives.

## Removed Code

### internal/bgp/message/update_build.go
| Function | Replacement | Reason |
|----------|-------------|--------|
| `BuildGroupedUnicast` | `BuildGroupedUnicastWithLimit` | No size limiting, can produce oversized UPDATEs |

### internal/reactor/reactor.go
| Function | Replacement | Reason |
|----------|-------------|--------|
| `buildAnnounceUpdate` | `WriteAnnounceUpdate` | Zero-allocation path available |
| `buildWithdrawUpdate` | `WriteWithdrawUpdate` | Zero-allocation path available |
| `buildAnnounceUpdateFromStatic` | `staticRouteToSpec` + `SendAnnounce` | Unused, zero-allocation path available |

### internal/reactor/reactor_test.go
| Test | Reason |
|------|--------|
| `TestBuildAnnounceUpdateIPv6UsesMPReachNLRI` | Covered by `TestWriteAnnounceUpdateIPv6` |
| `TestBuildWithdrawUpdateIPv6UsesMPUnreachNLRI` | Covered by `TestWriteWithdrawUpdateIPv6` |
| `TestWriteAnnounceUpdateMatchesBuildAnnounceUpdate` | Comparison test, no longer needed |
| `TestWriteWithdrawUpdateMatchesBuildWithdrawUpdate` | Comparison test, no longer needed |
| `TestWriteAnnounceUpdateIPv6MatchesBuildAnnounceUpdate` | Comparison test, no longer needed |
| `TestWriteWithdrawUpdateIPv6MatchesBuildWithdrawUpdate` | Comparison test, no longer needed |
| `BenchmarkBuildAnnounceUpdateIPv4` | Benchmarking deprecated function |

## Test Migration

Tests using `BuildGroupedUnicast` migrated to use helper:

```go
func mustBuildGrouped(t *testing.T, ub *UpdateBuilder, routes []UnicastParams) *Update {
    t.Helper()
    updates, err := ub.BuildGroupedUnicastWithLimit(routes, 65535)
    // ... error handling ...
    return updates[0]
}
```

## Verification

- [x] `make test` passes
- [x] `make functional` passes (43/43)
- [x] No production code used removed functions
