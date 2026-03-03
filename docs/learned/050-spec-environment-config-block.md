# 050 — Environment Configuration Block

## Objective

Add `environment { }` block support to ZeBGP config files, allowing environment variables to be set from within the config file instead of the shell, with strict fail-fast validation replacing the prior silent-ignore behaviour.

## Decisions

- Chose config-embedded block over ExaBGP's separate INI file approach — keeps all config in one file
- Priority order: OS env (dot notation) > OS env (underscore notation) > config block > defaults
- Strict validation everywhere: unknown sections, unknown options, type errors, range violations, and invalid OS env vars all cause startup failure (breaking change from prior silent ignore)
- Table-driven option setters (`envOptions map[string]map[string]envOption`) avoid a switch-case explosion per section/option combination
- `LoadEnvironment()` signature changed to return `(*Environment, error)` — callers must handle the error

## Patterns

- Table-driven validators with separate setter and validate funcs allows fail-fast at both parse time and OS env load time using the same code path
- `ze bgp config check --env` provided as migration aid to validate env vars before upgrading to strict validation

## Gotchas

- BREAKING CHANGE: prior code silently ignored invalid OS env vars (e.g., `ze.bgp.tcp.port=abc` would use default); after this change, startup fails
- Environment block can appear anywhere in config but is processed first (position agnostic but single occurrence only)

## Files

- `internal/config/environment.go` — `SetConfigValue()`, `LoadEnvironmentWithConfig()`, strict parsing helpers
- `internal/config/schema.go` — `parseEnvironment()` block parser
- `cmd/ze/bgp/config_check.go` — `--env` flag
