# 110 — Consolidate Route Commands to `update text`

## Objective

Remove all redundant `announce`/`withdraw` command handlers (24 handlers) and add missing EOR, VPLS, and EVPN support to `update text` so it becomes the single route announcement interface.

## Decisions

- EOR is implemented as an action keyword within the NLRI section (`update text nlri ipv4/unicast eor`) rather than as a separate top-level command — consistent with `add`/`del` pattern.
- `watchdog announce`/`watchdog withdraw` kept — they control watchdog groups, not routes.
- MUP support deferred to a future spec (complex route-type handling, `bgp-prefix-sid-srv6`).
- Local VPLS/L2VPN parsers moved to `cmd/ze/bgp/encode.go` for the CLI encode command (decoupled from plugin registration).

## Patterns

- 929 lines removed from `internal/component/plugin/route.go` and 808 from `handler_test.go`. After removal, `RegisterRouteHandlers()` only registers `update` and the two watchdog handlers.

## Gotchas

- MUP functional tests were simplified to unicast-only since MUP was not yet in `update text`.
- EOR syntax went through one fix commit: initially `update text eor <family>`, corrected to `update text nlri <family> eor` for consistency with the NLRI section pattern.

## Files

- `internal/component/plugin/route.go` — 929 lines removed; only `update` + watchdog remain
- `internal/component/plugin/update_text.go` — EOR action, VPLS/EVPN family parsers added
- `cmd/ze/bgp/encode.go` — local VPLS/EVPN parsers for CLI
