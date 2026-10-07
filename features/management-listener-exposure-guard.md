# Management Listener Exposure Guard

## Meta

| Field | Value |
|-------|-------|
| Name | Management Listener Exposure Guard |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | cmd/ze/hub/mgmt_guard.go |
| Real-path tests | test/plugin/mgmt-guard-loopback-allowed.ci, test/plugin/mgmt-guard-gnmi-token-allowed.ci, test/plugin/mgmt-guard-gnmi-nonloopback-refused.ci, test/plugin/mgmt-guard-mcp-env-nonloopback-refused.ci, test/plugin/mgmt-guard-web-insecure-env-refused.ci, test/reload/mgmt-guard-reload-refuses-nonloopback.ci, test/reload/mgmt-guard-reload-refuses-unauth.ci |
| Docs | docs/guide/authentication.md, docs/guide/api.md |
| Doc review | 2026-10-07: listenAddrIsNonLoopback in cmd/ze/hub/mgmt_guard.go counts names, empty host and wildcards as non-loopback, as the row says |
| Defect review | 2026-10-07: plan/journal/ipv6-address-built-by-concatenation.md (2026-08-10) IPv6 loopback ::1 classified non-loopback, unfixed |
| Extra criteria | supported: a management listener on ::1 without auth boots = test/plugin/mgmt-guard-ipv6-loopback-allowed.ci |

## Description

One boot-time, fail-closed check refuses to start a management service on a non-loopback address without authentication. It covers the insecure web listener, MCP, gNMI, and the API server (REST and gRPC). It runs once, after every address and credential is resolved and before anything binds, so a refusal leaves nothing bound and the process exits non-zero. Fail-closed on classification too: a wildcard (`0.0.0.0`, `::`), an empty host, and any DNS name including `localhost` all count as non-loopback, so remote reachability cannot be smuggled past it through a name. An unauthenticated surface that declares no resolved address is refused rather than passed by iterating zero times, which is the empty-set trap that once let an insecure web server reach `0.0.0.0:3443`. The refusal names the service, the address, and the fix, and never prints a token. For MCP the guard mirrors the server's own precedence: a token written beside an explicit `auth-mode none` does not authenticate, because the server builds its accept-all authenticator in that case. <!-- source: cmd/ze/hub/mgmt_guard.go -- checkMgmtListeners, listenAddrIsNonLoopback, mcpAuthModeAuthenticates --> <!-- source: cmd/ze/hub/main.go -- the four declaration sites and the call -->
