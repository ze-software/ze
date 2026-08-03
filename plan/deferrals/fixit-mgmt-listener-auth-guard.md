# Deferrals: fixit-mgmt-listener-auth-guard

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-17 | spec-fixit-mgmt-listener-auth-guard (Known Limitations) | Auth-mode changes on a SIGHUP reload do not take effect: management servers are constructed once, and reload migrates addresses only. Verified at the producer rather than inherited from the spec's prose -- `ReloadListeners` (`cmd/ze/hub/listener_migrate.go`) builds its change set exclusively from addresses (`endpointsToAddrs` for web/lg/mcp `:80-102`, `apiListenToAddrs` for rest/grpc `:104-117`), every path funnels into `buildChange(name, srv, newAddrs)` whose only per-service input is `newAddrs`, and the diff it drives (`listenerDiff`, `:190-213`) compares address lists. No auth field is read, compared, or applied | Not fixed in the source spec: its AC-7 stops a running UNAUTHENTICATED listener migrating onto a non-loopback address, which is the security-relevant half and needs no rebuild. Turning auth on for an already-running server is a lifecycle change (drain/replace or a widened `Reconfigurable`), which is larger and independent. Thomas confirmed AC-7's boot-plus-migration scope as shipped, 2026-07-17 | `plan/spec-fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild.md` (skeleton; the producer trace is recorded there so nobody redoes it. Also flags, without claiming, the source spec's gNMI token-over-plaintext note as a distinct concern -- transport secrecy, not identity -- to be placed when picked up) | deferred |
| 2026-07-19 | spec-fixit-mgmt-listener-auth-guard functional-proof | AC-5 LG TLS + AC-6 gnmi validate wiring + functional/QEMU refusal test deferred | live-server/QEMU constraint, deferred to CI | plan/spec-fixit-mgmt-listener-auth-guard.md | deferred |

