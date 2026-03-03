# 234 — Reload: Peer Add/Remove Functional Tests

## Objective
Add the three functional SIGHUP reload tests deferred from spec 230 (peer removal, peer addition, no-change), now feasible with the `action=sighup` / `action=rewrite` test infrastructure.

## Decisions
- Add-peer test uses a background shell trigger script (`tmpfs=trigger.sh`) rather than modifying ze-peer to bind all interfaces — tests the same `reconcilePeers` add path without changing infrastructure.
- No-change test uses `action=rewrite` with identical config content rather than a bare `action=sighup` — required because `daemon.pid` is only written when a tmpfs directory exists.
- No new daemon code: `reconcilePeers()` already handles all three cases; this spec is test-only.

## Patterns
- Any test using `action=sighup` must have at least one `tmpfs=` block — otherwise `daemon.pid` is never written and the SIGHUP cannot be delivered.
- ze-peer processes connections sequentially; for add-peer with no initial connection, an external trigger script provides the rewrite+SIGHUP independently of ze-peer's connection loop.

## Gotchas
- `daemon.pid` dependency on tmpfs is undocumented in the `.ci` format reference — discover it by reading `internal/test/peer/peer.go` NextSighupAction().
- Functional tests for SIGHUP scenarios require full daemon orchestration; unit tests of `reconcilePeers` alone (from spec 230) do not prove the SIGHUP path works end-to-end.

## Files
- `test/reload/reload-remove-peer.ci` — peer removal scenario
- `test/reload/reload-add-peer.ci` — peer addition via shell trigger
- `test/reload/reload-no-change.ci` — identical-config rewrite + SIGHUP
