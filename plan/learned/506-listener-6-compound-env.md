# 506 -- listener-6-compound-env

## Context

Per-field listener env vars (`ze.web.host` + `ze.web.port`) could not represent multi-endpoint configurations. This was deferred from spec-listener-2-env (AC-3 through AC-6) and tracked in plan/learned/503-listener-0-umbrella.md. The goal was to replace six per-field vars with three compound `ip:port` vars plus three `enabled` vars, matching how network listeners naturally express endpoints.

## Decisions

- Chose compound `ip:port,ip:port` format over separate host/port because it naturally maps to `net.Listen()` arguments and supports multi-endpoint in a single var.
- Added `ParseCompoundListen` as a standalone function in `environment.go` over creating a new file, because it is small (60 lines) and directly related to env var processing.
- Added `ze.<service>.enabled` vars alongside `.listen` because enabled-without-explicit-endpoint is a valid use case (uses service default).
- Moved `.ci` tests from `test/parse/` to `test/ui/` because they test `ze env list` output (no config parsing needed), and the parse test runner requires stdin config blocks.
- MCP enabled defaults to `127.0.0.1:8080` for security (localhost-only by default).

## Consequences

- Multi-endpoint listener configuration is now possible via a single env var.
- Any code using the old `ze.web.host`/`ze.web.port` keys will abort at startup (`env.Get` on unregistered key).
- The `help_ai.go` dynamic env var discovery automatically picks up new key names.
- The `spec-port-defaults.md` spec references old var names and will need updating if implemented.

## Gotchas

- Env var registrations exist in TWO files: `environment.go` (centralized) and `hub/main.go` (consumer). Both must be updated in lockstep. `MustRegister` silently overwrites duplicates.
- The `test/parse/` test runner requires stdin config blocks. Tests using `cmd=foreground:exec=ze env list` must go in `test/ui/`.
- Some docs referenced `ze.looking-glass.ip` (not `.host`), indicating prior inconsistency. Now unified as `.listen`.

## Files

- `internal/component/config/environment.go` -- added ListenEndpoint, ParseCompoundListen; replaced registrations
- `cmd/ze/hub/main.go` -- updated registrations and consumers
- `cmd/ze/help_ai.go` -- updated comment
- `internal/component/config/environment_test.go` -- 4 new test functions
- `test/ui/env-compound-listen.ci` -- functional test
- `test/ui/env-service-enabled.ci` -- functional test
- `docs/architecture/config/environment.md` -- added Listener Service Variables section
- `docs/guide/looking-glass.md` -- updated env var references
- `docs/architecture/web-interface.md` -- updated env var references
- `docs/features/mcp-integration.md` -- updated env var references
