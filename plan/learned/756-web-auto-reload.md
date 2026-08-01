# 756: Web UI Auto-Reload on Config Commit

## Context
After committing config changes in the web UI, a manual SIGHUP was required to apply them. The API path already had auto-reload via `reloadAfterCommit`, but the web path was not wired.

## Decision
Added a commit hook mechanism to `EditorManager` and wired it to the same `doReload` function the API uses. The web commit handler calls `RunCommitHook()` after successful commit and returns 500 with a descriptive message on reload failure.

Key choices:
- **Late-bound hook via `SetCommitHook`/`RunCommitHook`**: avoids import cycle between `web` and `hub` packages. The hook closure is set during startup wiring in `main.go`, not at construction time.
- **Mutex-protected hook**: `SetCommitHook` takes a write lock, `RunCommitHook` takes a read lock, since the hook could theoretically be updated during config reload.
- **Moved `reloadAfterCommit` outside `apiCfgOK` guard**: web-only deployments (no API server configured) also get automatic reload. Previously unreachable code path.
- **`startWebServer` returns `EditorManager`**: changed from `(WebServer, EventBroker)` to `(WebServer, EventBroker, EditorManager)` so the caller can wire the hook.
- **Error handling**: reload failure returns HTTP 500 with "config saved but reload failed: ... send SIGHUP or restart to apply". Config is saved; only the live apply failed.

## Consequences
- Web UI commits take effect immediately without SIGHUP.
- Reload failure is visible to the operator in the web UI (not silently swallowed).
- SSE config-change broadcast happens after successful reload, not before.

## Gotchas
- The commit hook runs synchronously in the HTTP handler. A slow reload blocks the response. Acceptable because reload is fast (<100ms) and the alternative (async with no feedback) is worse.
- `ike/engine` import was added to `main.go` in the same commit (unrelated side-by-side registration fix).

## Files

None recorded.
