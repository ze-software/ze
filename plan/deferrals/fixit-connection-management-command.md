# Deferrals: fixit-connection-management-command

One issue, recorded not fixed. The aggregate live backlog is folded on read from
`plan/deferrals/` by `/ze-status`. Nothing stores it (`ai/rules/planning.md`).

**Issue:** an operator command to list, re-check and close the connections a
daemon is currently serving.

**Owner ruling, 2026-08-08, verbatim:** "removing a user should only affect new
SSH connection as you may be editing your own user, we should instead have a
command to management existing connections and close them/re-check them and
allowing to close them and this should be a different spec".

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | Thomas's ruling on B-14, given while closing `spec-fixit-web-auth-deleted-user-survives-reload` | An operator command that lists the connections the daemon is serving, re-checks them against the running configuration, and closes the ones the operator chooses. It covers SSH sessions and SSE streams. Today a connection open at the moment a user is removed outlives the removal on both: `(*ssh.Server).Reload` (`internal/component/ssh/ssh.go`) tears down no session and `(*authz.Store).Authorize`'s assignments map is not rebuilt by a reload, while `(*EventBroker).ServeHTTP` (`internal/component/web/sse.go`) authenticates at connect and then blocks on the request context for the life of the connection | This is the ruling, not a defect: Thomas decided removal affects NEW connections only, because an operator may be editing their own user and a reload that cut their session would be worse than the window it closes. A new request after removal is already refused on every surface. What is owed is the TOOL that makes cutting a live connection a deliberate operator act. The SSE half is the real work: the broker has no per-subscriber identity today, so it cannot drop one subscriber's stream | needs its own spec | deferred |
