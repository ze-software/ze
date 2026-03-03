# 209 — YANG IPC Dispatch

## Objective

Replace the text `RegisterBuiltin()` dispatch pattern with YANG-driven RPC dispatch: extract RPC metadata from YANG modules, build a bidirectional CLI-text-to-wire-method lookup table, and add `ze schema methods` / `ze schema events` CLI commands.

## Decisions

- `WireModule()` strips the `-api`/`-conf` suffix for wire method prefix (e.g., `ze-bgp-api` → `ze-bgp`) — the suffix is a file-organisation convention, not part of the protocol identity.
- handler.go refactored into domain files per YANG module (`bgp.go`, `system.go`, `rib_handler.go`, `session.go`, `plugin.go`) — one file per YANG module scales; monolithic handler.go does not.
- Server `clientLoop` NUL replacement deferred to Spec 3 — text protocol still needed for plugin communication; clientLoop and plugin protocol must migrate together.
- YANG parameter validation (runtime type-checking against YANG `input` types) deferred to Spec 3 — types extracted but not enforced yet.
- Test thresholds are exact counts (e.g., `== 26 RPCs`) not `>= N` — loose thresholds hide YANG RPC count regressions.

## Patterns

- YANG is authoritative API definition: every YANG RPC must have a matching handler; `TestRPCRegistrationTable` enforces this alignment mechanically.
- `RPCRegistration` as a flat struct (WireMethod, CLICommand, Handler, Help) is simpler than interface-based registration.
- `filterPeersBySelector()` extracted as shared helper to eliminate duplicate peer-filtering logic across peer-list and peer-show handlers.

## Gotchas

- Text protocol code (`parseSerial`, `isComment`, etc.) cannot be deleted while plugin protocol still uses it — Spec 2 and Spec 3 share the same `server.go`; removing text protocol breaks plugins before they are migrated.
- IPC functional tests (`.ci` files) cannot be written until both the `ze-ipc` CLI tool and the NUL protocol server exist — both come in Spec 3.

## Files

- `internal/plugin/schema.go` — extended with `RegisterRPCs`, `RegisterNotifications`, `FindRPCByCommand`
- `internal/yang/rpc.go` — `RPCs()`, `Notifications()`, `WireModule()` extractions from YANG entry tree
- `internal/ipc/dispatch.go` — wire-method-based RPC dispatch (coexists with text dispatch until Spec 3)
- `internal/plugin/bgp.go`, `system.go`, `rib_handler.go`, `session.go`, `plugin.go` — domain handler files
- `internal/plugin/handler.go` — reduced to `RPCRegistration` struct + constants
- `test/parse/cli-schema-methods.ci`, `test/parse/cli-schema-events.ci` — functional tests
