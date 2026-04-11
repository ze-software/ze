# 544 -- API Engine (Shared Backend)

## Context

Ze had SSH CLI, HTMX web UI, and MCP as interfaces, but no programmatic REST or gRPC API. Network automation tools (Ansible, Terraform, custom scripts) had no way to control ze without SSH scraping. The goal was a shared API engine that both REST and gRPC transports use, keeping all logic in one place so the two transports cannot diverge.

## Decisions

- Chose function-type dependencies (`Executor`, `CommandSource`, `AuthChecker`, `StreamSource`) over importing dispatcher/plugin packages directly, because the api/ component must be independently deletable without breaking compilation
- Chose `CommandMeta`/`ParamMeta`/`ExecResult` names over `CommandInfo`/`ParamInfo`/`Response` to avoid hook-enforced name collisions with existing types in MCP and plugin packages
- Chose to keep MCP's own types independent over extracting shared types, because component independence is the core ze architecture pattern
- Chose lazy OpenAPI generation (`sync.Once` on first request) over eager startup generation, because plugins register commands during startup after the server starts listening
- Chose `api-server` as the YANG container name over `api`, because `ze-hub-conf` already defines `environment.api` for plugin API process settings (ack timeout, chunk size, etc.)

## Consequences

- REST and gRPC transports are guaranteed to produce identical command output because they both call the same engine
- Adding a new command to any plugin automatically exposes it through both REST and gRPC (no per-command wiring needed)
- Config sessions are engine-managed with timeout cleanup, preventing transport-specific session management divergence
- The OpenAPI spec reflects the actual command registry at request time, not a stale startup snapshot

## Gotchas

- The `check-existing-patterns.sh` hook blocks new files with type names that exist anywhere in `internal/`. Had to use unique names (`CommandMeta` not `Command`, `ExecResult` not `Response`)
- YANG schema `Define()` merges only one level deep. Two modules defining the same container name under `environment` shadow each other's children. Discovered when `api` collided with `hub-conf`'s `api` -- children from only one module appeared
- `yang_schema.go` requires a hard-coded block per YANG conf module. Registering via `yang.RegisterModule()` makes the module available to the loader, but the schema builder must explicitly call `GetEntry()` and `yangToNode()` for each module
- The `bodyclose` linter in tests requires every `*http.Response` to have its body closed. Returning `*http.Response` from helper functions triggers false positives. Solution: return a value struct with status/headers/body already read

## Files

- `internal/component/api/` -- engine, types, config sessions, schema generation (6 source + 3 test files)
- `internal/component/api/schema/` -- YANG config, embed, register
- `internal/component/api/rest/` -- REST HTTP transport
- `internal/component/api/grpc/` -- gRPC transport
- `api/proto/` -- proto3 definitions and generated Go code
- `cmd/ze/hub/api.go` -- hub startup wiring
- `internal/component/config/loader_extract.go` -- `ExtractAPIConfig`
- `internal/component/config/yang_schema.go` -- schema loader addition
- `internal/component/config/environment.go` -- env var registration
- `tools.go` -- vendored protoc plugins
- `Makefile` -- `ze-setup` target
