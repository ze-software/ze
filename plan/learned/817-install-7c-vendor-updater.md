# 817: install-7c-vendor-updater

## Context

spec-install-7c: replace raw HTTP PUT push with vendored gokrazy updater library for proper A/B root partition updates.

## Decisions

| Decision | Rationale |
|----------|-----------|
| Copy updater.go locally instead of adding to go.mod | Zero new external deps in ze's main module; updater is stdlib-only (430 lines) |
| authTransport wraps http.RoundTripper | updater.NewTarget makes its own requests (feature probe); auth must be on all requests, not just StreamTo |
| Keep protocolError type | Preserves existing error message differentiation (unreachable vs protocol) |
| Map updater errors to protocolError | Check for "401"/"Unauthorized" or "unexpected HTTP status" in error strings |
| Add --testboot flag | Uses updater.Testboot instead of Switch; auto-reverts on failure |
| Add --no-reboot flag | Streams and switches but skips reboot; useful for batch updates |
| Injectable doPushFn | Matches existing gokBuildFn/runExternalFn pattern for testability |

## Patterns

- **Vendored library copy**: when a library is stdlib-only and small, copying avoids go.mod pollution
- **authTransport wrapper**: inject credentials at transport level for libraries that make multiple requests
- **updaterMux test helper**: builds a full gokrazy update protocol mock (features, root, switch, testboot, reboot) with callback hooks

## Mistakes

| Mistake | Impact | Fix |
|---------|--------|-----|
| Missing resp.Body.Close on vendored updater | bodyclose lint failure | Added defer close to all 5 doer.Do calls |
| nil body instead of http.NoBody | gocritic httpNoBody lint | Replaced nil with http.NoBody on 4 POST/GET requests |
| for loop in Supports instead of slices.Contains | modernize lint | Used slices.Contains |

## Files

- `cmd/ze/install/appliance/updater/updater.go` (~270L): vendored gokrazy updater library

## Files Modified

- `internal/appliance/cmd_push.go`: replaced doPush with doPushUpdater using updater API; added authTransport, pushOpts, --testboot/--no-reboot flags
- `internal/appliance/cmd_push_test.go`: rewrote mock server to speak updater protocol; added TestPushTestboot, TestPushNoReboot, TestPushHashVerification, TestAuthTransport
