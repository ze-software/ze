| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-11 | appliance-ships-ze-kernel | `defaultHTTPGet` (`internal/appliance/cache.go`), read by `downloadAndVerify` and `downloadChecksum` | Unprobed: it `defer cancel()`s its request context before it returns the response, so the caller's `io.Copy` of the body reads under a canceled context and can fail with "context canceled" | not fixed: hand the cancel to the caller with the body, proven by a multi-buffer download through the real `defaultHTTPGet` |
