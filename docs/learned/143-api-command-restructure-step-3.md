# 143 — API Command Restructure Step 3: System Namespace

## Objective

Enhance the `system` namespace with `system version software` (renamed from `system version`), `system version api`, `system shutdown`, and `system subsystem list`.

## Decisions

- `system command list` scope change (list only system commands) was deferred — still lists all commands. Dynamic subsystem detection deferred: `system subsystem list` hardcodes `["bgp"]`.
- `APIVersion = "0.1.0"` constant added to `handler.go`.
- `system shutdown` calls `reactor.Stop()` — application-level shutdown, not just BGP subsystem (distinguished from a planned `bgp daemon shutdown`).

## Patterns

- Handlers return `*Response`; `WrapResponse()` from Step 1 wraps at serialization time. Handlers need no knowledge of the outer `type`/`response` wrapper.

## Gotchas

None.

## Files

- `internal/plugin/handler.go` — `APIVersion` const, `handleSystemVersionSoftware()`, `handleSystemVersionAPI()`, `handleSystemShutdown()`, `handleSystemSubsystemList()`, updated registrations
- `internal/plugin/handler_test.go` — 6 new tests
