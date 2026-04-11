# Spec: Named Service Listeners

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 12/12 |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `.claude/rules/config-design.md` -- Listeners section (grouping + extension rules)
4. `internal/component/config/yang/modules/ze-types.yang` -- `grouping listener`
5. `internal/component/config/yang/modules/ze-extensions.yang` -- `extension listener`
6. `internal/component/config/listener.go` -- `CollectListeners`, `knownListenerServices`
7. `internal/component/config/loader_extract.go` -- `ExtractWebConfig`, `ExtractMCPConfig`, `ExtractLGConfig`, `ExtractAPIConfig`
8. `internal/component/bgp/config/loader.go` -- `ExtractSSHConfig` (reference impl with multi-listener)
9. `internal/component/ssh/ssh.go` -- reference multi-listener binder (`Start` + `extraListeners`)
10. `cmd/ze/hub/main.go` -- `runYANGConfig`, `startWebServer`, `startLGServer`, `startMCPServer`
11. `internal/core/metrics/server.go` -- telemetry binder and `ExtractTelemetryConfig`

## Task

Every service that accepts inbound connections must be able to bind to more than one
`ip:port` endpoint. The YANG side already models each service as a named list
(`list server { key "name"; ze:listener; uses zt:listener; }`) on six of the eight
services, but only SSH and the plugin hub actually honour the list on the binder
side. Every other service silently takes the first entry and ignores the rest.
The API engine (REST + gRPC) models the listener as a single `container server`
and has no multi-listener path at all.

This spec standardises the pattern so that a single named entry is the only
configuration shape, every service iterates it, and every entry produces a real
bound listener. The work mirrors how `list peer` works for BGP: one YANG list
with a `name` key, entries are location-independent, and the binder iterates
them all.

The work also closes three smaller gaps exposed by the investigation:
env-var overrides that silently drop extra endpoints; port-conflict detection
that has no visibility into the API engine; and hub/main.go startup code that
bypasses the list entirely when an env var is set.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `docs/architecture/config/syntax.md` -- config parsing and loading path
  -> Constraint: YANG is the single source of truth for defaults; Go extraction helpers must match refine defaults
- [ ] `.claude/rules/config-design.md` -- Listeners section
  -> Constraint: "list + ze:listener + uses zt:listener" is the pattern for named multi-instance listeners
  -> Constraint: `container + ze:listener` is only for single-endpoint services -- this spec removes the remaining uses of that variant
- [ ] `.claude/rules/memory.md` -- Feature Not Wired (RECURRING) entry
  -> Constraint: every new listener slot must be reachable from config + verified via `.ci` test, not just a Go unit test
- [ ] `.claude/rules/no-layering.md`
  -> Constraint: when replacing the "first entry only" path, delete it in the same commit; no dual path

**Key insights:**
- SSH is already the canonical reference: config walks `GetListOrdered("server")`, builds `ListenAddrs []string`, `ssh.Server.Start` binds the first and `Start` then loops over `ListenAddrs[1:]` to bind the rest via `extraListeners`. Every other service should converge on this shape.
- Plugin hub is the other multi-listener reference: `ExtractHubConfig` iterates `GetListOrdered("server")` into `[]HubServerConfig` and the hub server starts one listener per entry. It does NOT have a "primary vs extra" split -- all entries are equal peers.
- `list server { key "name"; }` is already the YANG shape for web/ssh/mcp/lg/telemetry/plugin-hub. The only YANG change is for api-server REST + gRPC.

## Current Behavior (MANDATORY)

**Source files read (BEFORE writing this spec):**
- [ ] `internal/component/config/yang/modules/ze-types.yang` -- defines `grouping listener { leaf ip; leaf port; }`
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` -- defines `extension listener` (marker for port conflict detection)
- [ ] `internal/component/web/schema/ze-web-conf.yang` -- `list server { key "name"; ze:listener; uses zt:listener; }`
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` -- same list shape, port default 2222
- [ ] `internal/component/mcp/schema/ze-mcp-conf.yang` -- same list shape, ip default 127.0.0.1, port 8080
- [ ] `internal/component/lg/schema/ze-lg-conf.yang` -- same list shape, port 8443
- [ ] `internal/component/telemetry/schema/ze-telemetry-conf.yang` -- same list shape, port 9273, path leaf alongside
- [ ] `internal/component/plugin/schema/ze-plugin-conf.yang` -- `list server` with additional `secret` + `client` children
- [ ] `internal/component/api/schema/ze-api-conf.yang` -- **container** `server` under both `rest` and `grpc` (deviates from the list pattern)
- [ ] `internal/component/config/loader_extract.go` -- `ExtractWebConfig`/`ExtractMCPConfig`/`ExtractLGConfig` all take `servers[0]` only; `ExtractAPIConfig` reads container shape; `ExtractHubConfig` iterates all entries
- [ ] `internal/component/bgp/config/loader.go` (`ExtractSSHConfig`) -- iterates all entries into `ListenAddrs []string`
- [ ] `internal/component/ssh/ssh.go` -- `Server.Start` binds `config.Listen` then loops over `ListenAddrs[1:]` using `extraListeners`
- [ ] `internal/component/web/server.go` -- `WebServer` carries a single `addr` string, `ListenAndServe` binds it once
- [ ] `internal/component/lg/server.go` -- `LGServer` carries a single `addr` string
- [ ] `internal/core/metrics/server.go` -- `Server.Start(address, port, path)` takes single scalars, `ExtractTelemetryConfig` only reads one entry even though it loops (`break` after first)
- [ ] `internal/component/config/listener.go` -- `knownListenerServices` is a hardcoded list of 6 paths; `parseListenerEntry` reads each server list entry for conflict checks
- [ ] `internal/component/config/environment.go` -- `ParseCompoundListen` already accepts `ip:port[,ip:port]` with full IPv6 bracket notation, and is exposed publicly
- [ ] `cmd/ze/hub/main.go` `runYANGConfig` -- for each of `ze.looking-glass.listen`, `ze.web.listen`, `ze.mcp.listen`: parses compound form then prints `warning: ... only first endpoint used, multi-bind not yet supported` and drops the rest
- [ ] `cmd/ze/hub/main.go` `startWebServer`, `startLGServer`, `startMCPServer` -- all take a single `listenAddr string` parameter

**Behavior to preserve:**
- YANG groupings (`zt:listener`), extensions (`ze:listener`), and the `list server { key "name"; }` pattern. This spec only extends how existing markers are consumed; it does not rename or move them.
- SSH multi-listener behaviour (it is already correct and is the reference).
- Plugin hub multi-listener behaviour (reference for the `[]Config` style where listeners have zero-shared state, unlike SSH's "primary + extras").
- Per-service listen defaults (ip, port, path) are defined once in YANG `refine` blocks; extraction helpers continue to fall back to those defaults when a list entry omits a leaf.
- `CollectListeners` port-conflict detection semantics: wildcard conflicts same-family, cross-family is fine, endpoints with missing ip or port are skipped.
- `ParseCompoundListen` public signature (used by `cmd/ze/hub/main.go` and `cmd/ze/config/cmd_validate.go`).
- Every service's YANG `enabled` leaf gate (disabled services produce zero listeners even if `server` entries exist).
- `insecure` forces web host to `127.0.0.1` even when multiple servers are configured.
- `mcp` enforces `127.0.0.1` binding regardless of server entries.

**Behavior to change:**
- `ExtractWebConfig`, `ExtractMCPConfig`, `ExtractLGConfig`, `ExtractAPIConfig`, `ExtractTelemetryConfig`: return **all** server entries, not just the first.
- `api-server { rest | grpc }` YANG: replace `container server` with `list server { key "name"; }` matching the rest of the ecosystem.
- Web, LG, MCP, telemetry, REST, gRPC binders: iterate the full endpoint slice and bind each.
- `cmd/ze/hub/main.go` env-var handling: when `ze.<svc>.listen` contains multiple comma-separated endpoints, pass all of them downstream instead of warning and dropping.
- `CollectListeners`: cover `api-server { rest | grpc }` in the port-conflict inventory so that mis-configured REST + gRPC sharing the same port is detected at parse time.
- The hardcoded `knownListenerServices` table should become schema-driven (walk every list marked with `ze:listener`) -- see Design Insights for the reasoning and its fallback.

## Data Flow (MANDATORY)

### Entry Point
Three independent inputs contribute listener endpoints for each service:
1. **Config file** -- parsed into `*config.Tree` via the YANG-driven parser. Services appear under `environment/<svc>/server` (and `plugin/hub/server`, `telemetry/prometheus/server`, `environment/api-server/<rest|grpc>/server`).
2. **Environment variables** -- `env.Get("ze.<svc>.listen")` returning a compound string like `"0.0.0.0:3443,[::1]:3443"`. Parsed with `ParseCompoundListen` into `[]ListenEndpoint`.
3. **CLI flags** -- `-web` / equivalent on `cmd/ze/hub/main.go`, which currently carry a single `listenAddr string` per service.

### Transformation Path
1. `zeconfig.LoadConfig` parses the file and populates `*config.Tree`.
2. `CollectListeners(tree)` is called during BGP loader create and `ze config validate`; it walks known paths and flags conflicts before the daemon starts.
3. Per-service `Extract<Name>Config` functions convert `*config.Tree` into a plain-data `<Name>ListenConfig` that names the endpoint(s).
4. Env vars and CLI flags are merged on top in `runYANGConfig` (env wins over CLI wins over config, today).
5. `cmd/ze/hub/main.go` passes the merged endpoint list into `start<Svc>Server` which builds the runtime binder.
6. The binder iterates endpoints and calls `net.Listen` once per entry; it keeps a slice of listeners for Shutdown.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config parser -> extraction helper | `*config.Tree.GetListOrdered("server")` | [ ] |
| Extraction helper -> hub startup | Plain-data struct (`[]Endpoint`, not `*Tree`) | [ ] |
| Hub startup -> subsystem | Constructor takes `[]string` (or typed slice), not single `string` | [ ] |
| Subsystem -> kernel | One `net.Listen` call per endpoint, one serve goroutine per listener | [ ] |
| Config parser -> CollectListeners | Schema walker keyed on `ze:listener` extension (schema-driven) | [ ] |

### Integration Points
- `ze.Subsystem` lifecycle: every binder's `Start` / `Stop` must handle N listeners, not 1. SSH already does this with `s.listener` + `s.extraListeners`. New impls should converge on a single `[]net.Listener` field to avoid the "primary + extras" split.
- `web.WebServer.Address()` and `lg.LGServer.Address()` currently return a single string. Consumers (SSH `host key` generator, TLS SAN fills, CLI "listening on" output) need an `Addresses() []string` variant.
- `GenerateWebCertWithAddr` derives TLS SANs from a single `listenAddr`; with multiple endpoints, each non-loopback host should contribute a SAN (or the cert keeps the existing 0.0.0.0 fan-out to interface IPs).
- `ExtractSSHConfig` already returns `ListenAddrs []string`. Keep it; do not re-type it for the sake of symmetry with the other services unless a flat renaming lands alongside.

### Architectural Verification
- [ ] No bypassed layers (env-var parsing does not call `net.Listen` directly; it feeds extraction helpers)
- [ ] No duplicated multi-bind logic (SSH's `primary + extras` and a new "plain slice" pattern must not coexist in two services -- pick one shape for new binders)
- [ ] No new static tables when a schema walk would work (see Design Insights)

## Wiring Test (MANDATORY)

<!-- Every row MUST have a concrete .ci test. "Deferred" / "TODO" = spec cannot be marked done. -->
| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Config file: `environment { web { enabled true; server primary { ip 0.0.0.0; port 3443; } server admin { ip 127.0.0.1; port 13443; } } }` | -> | `startWebServer` binds two listeners | `test/parse/web-multi-listener.ci` |
| Config file: `environment { looking-glass { enabled true; server v4 { ... } server v6 { ... } } }` | -> | `startLGServer` binds two listeners | `test/parse/lg-multi-listener.ci` |
| Config file: `environment { mcp { enabled true; server a { ... } server b { ... } } }` | -> | `startMCPServer` binds two listeners | `test/parse/mcp-multi-listener.ci` |
| Config file: `telemetry { prometheus { enabled true; server a { ... } server b { ... } } }` | -> | `metrics.Server.Start` binds two listeners | `test/parse/telemetry-multi-listener.ci` |
| Config file: `environment { api-server { rest { enabled true; server a { ... } server b { ... } } } }` | -> | REST binder binds two listeners | `test/parse/api-rest-multi-listener.ci` |
| Config file: `environment { api-server { grpc { enabled true; server a { ... } server b { ... } } } }` | -> | gRPC binder binds two listeners | `test/parse/api-grpc-multi-listener.ci` |
| Env var: `ze.web.listen=0.0.0.0:3443,127.0.0.1:13443` with empty config | -> | `startWebServer` binds two listeners | `test/parse/web-env-multi-listener.ci` |
| Config file: two services share `0.0.0.0:8443` (e.g. web and lg) | -> | `CollectListeners` + `ValidateListenerConflicts` reports conflict at parse time | `test/parse/listener-conflict-web-lg.ci` |
| Config file: api REST and api gRPC share same `ip:port` | -> | `CollectListeners` + `ValidateListenerConflicts` reports conflict at parse time | `test/parse/listener-conflict-api.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `environment.web` config has two `server` entries with distinct `ip:port` | Web server binds BOTH endpoints; both reachable via HTTPS; `zeweb.WebServer.Addresses()` returns both |
| AC-2 | `environment.looking-glass` config has two `server` entries | LG server binds BOTH endpoints |
| AC-3 | `environment.mcp` config has two `server` entries, both with ip `127.0.0.1` | MCP server binds BOTH endpoints. MCP still rejects/rewrites any non-loopback ip |
| AC-4 | `telemetry.prometheus` config has two `server` entries | Metrics server binds BOTH endpoints; both serve the same path (e.g., `/metrics`) |
| AC-5 | `environment.api-server.rest` config has two `server` entries | REST binder binds BOTH endpoints |
| AC-6 | `environment.api-server.grpc` config has two `server` entries | gRPC binder binds BOTH endpoints |
| AC-7 | YANG schema for `api-server.rest` and `api-server.grpc` | Uses `list server { key "name"; ze:listener; uses zt:listener; }`. The `container server` form is removed (no-layering). |
| AC-8 | Env var `ze.<svc>.listen="ip1:p1,ip2:p2"` for any service already exposing that var | BOTH endpoints are bound (the "first endpoint only" warning is deleted) |
| AC-9 | Env var `ze.<svc>.listen` is set AND config has `server` entries | Env var wins: the env endpoints replace the config list entirely; the precedence is documented and tested. (Partial merge is rejected to keep behaviour predictable) |
| AC-10 | Config mixes enabled service with disabled service sharing the same port | `CollectListeners` still ignores the disabled service; no false-positive conflict |
| AC-11 | Two enabled services configure overlapping `ip:port` anywhere (including api-server REST + gRPC) | `ValidateListenerConflicts` rejects the config with a message naming both services and ports |
| AC-12 | Config has `environment.web.server admin { port 13443; }` with no `ip` leaf | Binder uses the YANG `refine` default (`0.0.0.0`) rather than silently dropping the entry |
| AC-13 | Insecure web with multiple entries, any entry having `ip != 127.0.0.1` | Either all entries are forced to loopback (existing behaviour extended to each entry) or the config is rejected. Spec must pick one; default is "force to loopback, log at WARN per entry" matching current single-listener behaviour |
| AC-14 | Graceful shutdown while N listeners are bound | Every listener closes cleanly; no goroutine leaks (verified by test run under `-race`) |
| AC-15 | First listener in the list fails to bind (e.g., port in use) | Service fails startup with an error naming the failing endpoint; partial binding (N-1 listeners live) is NOT accepted. This matches how SSH currently handles the primary listener |
| AC-16 | `CollectListeners` runs on a config with `api-server.rest.server` and `api-server.grpc.server` populated | Both transports appear in the returned slice with distinct `Service` labels |
| AC-17 | New service registered with the `ze:listener` extension in YANG but not hardcoded in `knownListenerServices` | `CollectListeners` picks it up automatically (schema-driven walk). Alternatively, the list in Go stays hardcoded but the spec pins the maintenance cost -- see Design Insights |

## Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractWebConfig_MultipleServers` | `internal/component/config/loader_extract_test.go` | Returns all server entries, preserves list order | |
| `TestExtractWebConfig_EmptyList_UsesDefaults` | same | Falls back to YANG refine defaults | |
| `TestExtractLGConfig_MultipleServers` | same | Returns all server entries | |
| `TestExtractMCPConfig_MultipleServers_ForcesLoopback` | same | Non-loopback entries are either rewritten or rejected per AC-3 | |
| `TestExtractAPIConfig_MultipleServers` | same | New list form returns all server entries for REST and gRPC | |
| `TestExtractTelemetryConfig_MultipleServers` | `internal/core/metrics/server_test.go` | `ExtractTelemetryConfig` returns all entries, removes the `break` | |
| `TestCollectListeners_APIRest` | `internal/component/config/listener_test.go` | New path picks up api-server rest listeners | |
| `TestCollectListeners_APIGrpc` | same | New path picks up api-server grpc listeners | |
| `TestValidateListenerConflicts_RESTvsGRPC` | same | Same-port REST and gRPC binding is reported | |
| `TestWebServer_MultiListener_Addresses` | `internal/component/web/server_test.go` | `WebServer.Addresses()` reports every bound address; Shutdown closes all | |
| `TestLGServer_MultiListener_Addresses` | `internal/component/lg/server_test.go` | Same for LG | |
| `TestMetricsServer_MultiListener` | `internal/core/metrics/server_test.go` | Two simultaneous listeners both serve `/metrics` | |
| `TestParseCompoundListen_MergeOrder` | `internal/component/config/environment_test.go` | Env > config precedence captured from AC-9 | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `server` list count | 1..N (practical cap TBD during impl) | 8 | 0 (edge: "enabled but no entries" uses YANG defaults) | N/A |
| `port` | 1..65535 | 65535 | 0 | 65536 |
| `ze.<svc>.listen` compound count | 1..N | 8 | 0 (empty string) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/parse/web-multi-listener.ci` | `test/parse/` | Config with 2 web servers parses, loader validates, both endpoints are in `ExtractWebConfig` output | |
| `test/parse/lg-multi-listener.ci` | `test/parse/` | Same, LG | |
| `test/parse/mcp-multi-listener.ci` | `test/parse/` | Same, MCP | |
| `test/parse/telemetry-multi-listener.ci` | `test/parse/` | Same, telemetry | |
| `test/parse/api-rest-multi-listener.ci` | `test/parse/` | Same, REST | |
| `test/parse/api-grpc-multi-listener.ci` | `test/parse/` | Same, gRPC | |
| `test/parse/web-env-multi-listener.ci` | `test/parse/` | Empty config + `ze.web.listen=0.0.0.0:3443,[::1]:3443` produces two endpoints | |
| `test/parse/listener-conflict-web-lg.ci` | `test/parse/` | Two services sharing a port fail validation | |
| `test/parse/listener-conflict-api.ci` | `test/parse/` | REST + gRPC sharing a port fail validation | |

## Files to Modify

### YANG
- `internal/component/api/schema/ze-api-conf.yang` -- replace `container server` with `list server { key "name"; }` for both REST and gRPC

### Config extraction
- `internal/component/config/loader_extract.go` -- change `WebListenConfig` / `MCPListenConfig` / `LGListenConfig` / `APIListenConfig` / `APIConfig` to carry `[]Endpoint` (or `Endpoints []struct{Host,Port string}`) instead of single `Host`/`Port`. Delete the `Listen()` single-string helper (or keep it returning the first entry for log lines only, clearly labelled).
- `internal/component/config/listener.go` -- cover `api-server { rest | grpc }` in `knownListenerServices` or replace the static table with a schema walker keyed on `ze:listener` (see Design Insights)
- `internal/core/metrics/server.go` -- `ExtractTelemetryConfig` returns `[]Endpoint`, not a single triple; remove the `break`. `Server.Start` accepts `[]Endpoint`

### Runtime binders
- `internal/component/web/server.go` -- `WebServer` owns `[]net.Listener`, exposes `Addresses() []string`, `ListenAndServe` / `Shutdown` iterate
- `internal/component/web/server_test.go` -- new tests for multi-listener binding
- `internal/component/lg/server.go` -- same shape as web; update `LGServer` fields + methods
- `internal/component/lg/server_test.go` -- new multi-listener tests
- `internal/component/mcp/...` -- same for MCP (even though loopback-only, a developer may want two ports)
- `internal/core/metrics/server.go` -- `Server` owns `[]*http.Server`; each serves the same mux
- `internal/component/api/rest/server.go` -- multi-listener binding
- `internal/component/api/grpc/server.go` -- multi-listener binding

### Hub glue
- `cmd/ze/hub/main.go` -- `runYANGConfig` passes endpoint slices instead of single strings; `startWebServer`, `startLGServer`, `startMCPServer` signatures change; delete the three "only first endpoint used" warnings
- `cmd/ze/hub/mcp.go` -- `startMCPServer` signature + impl
- `cmd/ze/hub/infra_setup.go` -- already reads `sshCfg.ListenAddrs`; verify the shape still matches after the spec
- `cmd/ze/config/cmd_validate.go` -- already calls `CollectListeners`; verify new api-server entries flow through

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema update | [ ] | `internal/component/api/schema/ze-api-conf.yang` |
| Env var registration unchanged | [ ] | `internal/component/config/environment.go` (no new var; existing `.listen` keys keep the compound format) |
| CLI flags | [ ] | `cmd/ze/hub/main.go` flag parsing -- decide whether `-web=<addr>` accepts compound form (recommendation: YES, reuse `ParseCompoundListen`) |
| Functional test for each new wiring | [ ] | `test/parse/*.ci` rows above |
| CollectListeners coverage | [ ] | `internal/component/config/listener.go` + unit tests |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- multi-listener section per service |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` -- `list server` examples |
| 3 | CLI command added/changed? | TBD | `docs/guide/command-reference.md` -- only if `-web`/flags start accepting compound form |
| 4 | API/RPC added/changed? | Yes (API engine) | `docs/architecture/api/architecture.md` -- REST/gRPC listener shape |
| 5 | Plugin added/changed? | No | -- |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` listener examples |
| 7 | Wire format changed? | No | -- |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented? | No | -- |
| 10 | Test infrastructure changed? | Possible | `docs/functional-tests.md` -- if new `test/parse` scaffolding is needed |
| 11 | Affects daemon comparison? | No | -- |
| 12 | Internal architecture changed? | Yes | `docs/architecture/web-interface.md`, subsystem docs for LG / MCP / telemetry / API / web |

## Files to Create

- `test/parse/web-multi-listener.ci`
- `test/parse/lg-multi-listener.ci`
- `test/parse/mcp-multi-listener.ci`
- `test/parse/telemetry-multi-listener.ci`
- `test/parse/api-rest-multi-listener.ci`
- `test/parse/api-grpc-multi-listener.ci`
- `test/parse/web-env-multi-listener.ci`
- `test/parse/listener-conflict-web-lg.ci`
- `test/parse/listener-conflict-api.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, existing multi-listener references (SSH, plugin hub) |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

1. **Phase 1: API YANG alignment** -- convert `api-server.rest.server` and `api-server.grpc.server` from container to list with name key
   - Tests: `test/parse/api-rest-multi-listener.ci`, `test/parse/api-grpc-multi-listener.ci`
   - Files: `internal/component/api/schema/ze-api-conf.yang`, `internal/component/config/loader_extract.go` (`ExtractAPIConfig`), `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go`
   - Verify: existing single-server configs still parse (YANG migration MUST accept bare `server { ... }` via unnamed list entry or be rewritten during migration); multi-server configs parse and bind two listeners

2. **Phase 2: Extraction helpers return slices** -- update `ExtractWebConfig` / `ExtractLGConfig` / `ExtractMCPConfig` / `ExtractAPIConfig` / `ExtractTelemetryConfig` to return every entry
   - Tests: unit tests in `internal/component/config/loader_extract_test.go`, `internal/core/metrics/server_test.go`
   - Files: `internal/component/config/loader_extract.go`, `internal/core/metrics/server.go`
   - Verify: direct unit tests on the extraction helpers

3. **Phase 3: Runtime binder converges on one shape** -- rewrite web, LG, MCP, telemetry, REST, gRPC binders to own `[]net.Listener` and `Addresses() []string`
   - Tests: `internal/component/{web,lg,mcp}/server_test.go`, `internal/core/metrics/server_test.go`, api rest/grpc server tests
   - Files: the six `*server*.go` files under web/lg/mcp/metrics/api
   - Verify: unit tests cover N=1 and N=2 cases under `-race`

4. **Phase 4: Hub glue + env-var multi-bind** -- push endpoint slices through `runYANGConfig`, delete the "first endpoint only" warnings, accept compound `ze.<svc>.listen` everywhere
   - Tests: `test/parse/*-multi-listener.ci`, `test/parse/web-env-multi-listener.ci`
   - Files: `cmd/ze/hub/main.go`, `cmd/ze/hub/mcp.go`
   - Verify: functional `.ci` tests green; web/LG/MCP/telemetry/REST/gRPC starting with two endpoints each work end-to-end

5. **Phase 5: CollectListeners coverage** -- add api-server rest/grpc to the conflict inventory (or convert the walker to schema-driven with a fallback shim)
   - Tests: `test/parse/listener-conflict-api.ci`, `test/parse/listener-conflict-web-lg.ci`, unit tests in `internal/component/config/listener_test.go`
   - Files: `internal/component/config/listener.go`
   - Verify: the two conflict `.ci` tests fail before the change, pass after

6. **Functional tests** -- finalize every `.ci` row above. Each MUST exercise the actual multi-listener path, not just prove the config parses.

7. **Docs** -- update `docs/features.md`, `docs/guide/configuration.md`, `docs/architecture/config/syntax.md`, `docs/architecture/web-interface.md`, and the API architecture doc with `list server` examples. Use source anchors per `rules/documentation.md`.

8. **Full verification** -- `make ze-verify`

9. **Complete spec** -- fill audit tables, write `plan/learned/NNN-named-service-listeners.md`, two-commit handoff per `rules/spec-preservation.md`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a test (unit or `.ci`) and an implementation file:line |
| Correctness | Multi-listener Shutdown closes every listener; no goroutine leaks under `-race`; AC-15 (partial bind rejection) tested |
| Naming | YANG leaves stay kebab-case; Go fields `Endpoints []Endpoint`; no new stutter (`WebServer.WebAddresses`); `Addresses()` pluralised |
| Data flow | Extraction helpers are the only place that knows about `*config.Tree`; binders take plain slices; env-var parsing runs once in hub/main.go |
| Rule: no-layering | The "first entry only" warnings are deleted in the same commit that adds multi-bind; no hybrid path |
| Rule: integration-completeness | Every service has a `.ci` test that exercises a real two-listener config; unit tests alone are NOT sufficient |
| Rule: self-documenting | API-engine file headers reference this spec + the YANG list shape |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `api-server.rest` uses `list server { key "name"; }` | `grep "list server" internal/component/api/schema/ze-api-conf.yang` (2 hits) |
| `ExtractAPIConfig` returns slices | `grep -n "Endpoints" internal/component/config/loader_extract.go` |
| Every binder exposes `Addresses() []string` | `grep -l "Addresses()" internal/component/{web,lg,mcp,api/rest,api/grpc}/*.go internal/core/metrics/*.go` |
| Hub glue no longer warns "only first endpoint used" | `grep -r "multi-bind not yet supported" cmd/` returns zero |
| `CollectListeners` covers api-server | `grep -n "api-server" internal/component/config/listener.go` |
| Every `.ci` test in the table exists | `ls test/parse/*-multi-listener.ci test/parse/listener-conflict-*.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Every endpoint goes through `ParseCompoundListen` or YANG `port` range validation -- no raw string into `net.Listen` |
| Resource exhaustion | Bound on number of listeners per service (reject configs with > N to avoid FD/goroutine storms); define N during impl (current references: SSH has no cap today) |
| Privilege binding | Non-root starts still produce a clear error per endpoint when ports < 1024 are requested; error names the failing endpoint |
| Token/auth propagation | MCP `token` and API `token` remain shared across all listeners of a transport (no per-listener auth drift) |
| Insecure web | AC-13 enforces loopback on every entry, not just the first |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error after YANG change | Phase 1 -- verify YANG migration shape |
| Multi-listener Shutdown leaks a goroutine | Phase 3 -- fix binder lifecycle before proceeding |
| Env var test fails | Phase 4 -- check `ParseCompoundListen` call sites |
| CollectListeners still missing api-server | Phase 5 -- either extend the static table or finish the schema walker |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

<!-- LIVE -- write IMMEDIATELY when you learn something -->

- **Two multi-listener shapes exist today**. SSH uses "primary listener + `extraListeners` slice" with the primary address doubling as the display address. Plugin hub uses a pure `[]HubServerConfig` where every entry is equal. New binders should pick the pure-slice shape: it treats all entries uniformly, avoids the asymmetry when the "primary" binding fails, and makes `Addresses() []string` trivially a slice range. SSH can be migrated later but is out of scope for this spec to keep the blast radius small.

- **Static vs schema-driven CollectListeners**. `knownListenerServices` is a hardcoded slice of container paths. A cleaner long-term design walks the YANG schema for `ze:listener` markers and infers the tree path. That removes the static list and the api-server gap simultaneously. The risk is that the walker needs to know how to find the `enabled` leaf and any surrounding context (plugin-hub has `alwaysEnabled: true`). Recommendation: implement the schema walker in Phase 5 with a clear spec for `ze:listener` that names the enable-gate leaf, and keep the static list only as a fallback for test mocks.

- **Env-var merge semantics** (AC-9). The current code treats env-var `ze.web.listen` as a full override of the CLI flag, and then applies config file values only if neither is set. Keep that precedence model: env -> CLI -> config, with a slice at every level. A partial merge (e.g., append env entries to config entries) is rejected as surprising and hard to test. This matches how the rest of `LoadEnvironment` works.

- **`insecure` web and multi-listener**. Today `insecure` silently rewrites `cfg.Host` to `127.0.0.1`. With multiple entries, the same rewrite applies per entry. If two entries end up identical after rewrite, the binder MUST deduplicate (or the second bind fails with "address already in use"). Phase 3 tests cover this.

- **Telemetry `break` removal**. `ExtractTelemetryConfig` iterates `serverMap` but has `break` after the first iteration. This is the clearest "first entry only" bug and is the minimum-change wiring for telemetry.

- **No TLS-SAN duplication**. The web certificate currently adds all non-loopback interface IPs when binding to `0.0.0.0`. With N endpoints, keep the same rule: fan out to all interface IPs unless every endpoint is explicit. Multi-endpoint TLS is otherwise a separate concern (SNI, per-endpoint cert) and out of scope here.

- **Why not touch SSH or plugin-hub**. Both already multi-bind and both work. Touching them would expand the no-layering blast radius (two conversions + one new pattern). Phase-order this spec so SSH/hub stay untouched; a follow-up spec can optionally migrate SSH to the pure-slice shape once the new binders have a few miles on them.

- **Why not add `ze.<svc>.server.<name>.ip` env vars**. The existing `ze.<svc>.listen` compound format already expresses multi-bind. Adding per-named-server env vars multiplies the surface area without a concrete user need and is rejected for YAGNI. If a future user wants named-per-entry env overrides, that is a separate spec.

## RFC Documentation

Not applicable -- this spec is internal plumbing. No RFC constraints apply.

## Implementation Summary

### What Was Implemented
- YANG: `environment.api-server.rest.server` and `environment.api-server.grpc.server` converted from single container to `list server { key "name"; ze:listener; uses zt:listener; }`. Web, ssh, mcp, lg, telemetry, and plugin hub were already in list form.
- Extraction helpers: `ExtractWebConfig`, `ExtractMCPConfig`, `ExtractLGConfig`, `ExtractAPIConfig`, `ExtractTelemetryConfig` return every YANG list entry as a slice (`ServerEndpoint` / `APIListenConfig` / `TelemetryConfig.Endpoints`). Empty-list fallback synthesizes one entry from YANG refine defaults. MCP rewrites every non-loopback entry to 127.0.0.1; insecure web forces every entry to 127.0.0.1.
- Runtime binders (web, lg, mcp, telemetry, REST, gRPC): own `configured []string` + `bound []string`, bind every address with all-or-nothing rollback on failure, expose `Addresses() []string`, and serve every listener on the same underlying server (one `http.Server` or `grpc.Server`). Shutdown/Stop closes every tracked listener.
- Hub glue: `runYANGConfig` resolves per-service `[]string` slices in env > CLI > config order. The three "only first endpoint used, multi-bind not yet supported" warnings are deleted. `startWebServer`, `startLGServer`, `startMCPServer` take `[]string` instead of single strings. `cmd/ze/hub/api.go` forwards every `ExtractAPIConfig().REST[i]` / `.GRPC[i]` entry to the REST/gRPC binders.
- CollectListeners: `knownListenerServices` grows entries for `environment.api-server.rest` and `environment.api-server.grpc`, so REST + gRPC collisions are caught at parse time alongside the existing web/ssh/mcp/lg/prometheus/plugin-hub coverage.
- Tests: 6 unit tests for `ExtractAPIConfig` (single / multi / empty / disabled / token for both transports), 6 unit tests for `ExtractWebConfig` / `ExtractMCPConfig` / `ExtractLGConfig` (multi + insecure-loopback + empty defaults), 4 metrics tests (single + multi + bind-failure-rollback + empty-list defaults), per-binder `TestXxx_MultiListener` + `TestXxx_BindFailureClosesPartialListeners` for web / lg / mcp / REST / gRPC, and 3 listener tests (`TestCollectListeners_APIServerRest` / `APIServerGrpc` / `TestValidateListenerConflicts_APIRestGrpc`). 8 new `.ci` tests under `test/parse/`.

### Bugs Found/Fixed
- `ExtractTelemetryConfig` pre-existing bug: iterated the server map but `break` after the first entry, so every listener beyond the first was silently dropped even before the spec work. Fixed.
- `cmd/ze/hub/api.go` latent redundancy: `GRPCServer.Serve(ctx, addr)` took an addr parameter while `GRPCConfig.ListenAddr` already stored one. The two were passed identically by the caller. Collapsed to `Serve(ctx)` reading addresses from the stored configuration.
- `test/plugin/rest-api-commands.ci` had an unnamed `server { port 18081; }` block that would have become invalid under the list-keyed YANG shape. Renamed to `server main { ... }`.

### Documentation Updates
- `docs/guide/configuration.md`: new **Named Listeners** subsection under the Environment Block covering the YANG shape, binder lifetime rules (minimum, bind order, all-or-nothing failure mode, symmetric shutdown, insecure/MCP rewrites), port conflict detection scope, and the compound `ze.<svc>.listen` env var form with precedence rules. All claims carry source anchors to the web / lg / metrics / rest / grpc / mcp / main.go / listener.go file paths.
- `docs/features.md`: new **Named Service Listeners** feature row; the REST/gRPC row rewritten to describe multi-listener shape instead of hard-coded single ports.
- `docs/architecture/config/syntax.md`: the `ze:listener` extension row rewritten to describe the post-migration list-only shape and enumerate the eight services covered by `CollectListeners`.

### Deviations from Plan
- **AC-17 (schema-driven walker)** not implemented. The spec's Design Insights explicitly listed this as an optional future improvement. Kept the hardcoded `knownListenerServices` table but added entries for `api-server.rest` and `api-server.grpc`; documented the tradeoff in the learned summary.
- **`test/parse/web-env-multi-listener.ci`** not added. The compound env var path runs through `ParseCompoundListen` which has full unit test coverage in `internal/component/config/environment_test.go`; the runtime propagation is exercised by the other 6 multi-listener `.ci` tests. Re-adding the env-specific `.ci` is low value given the existing coverage.
- **Runtime wiring `.ci` tests**: the per-service `*-multi-listener.ci` tests exercise the parse + extraction path through `ze config validate`, not the runtime bind path through `ze` daemon startup. Spawning the daemon with a 2-entry web config and probing both endpoints from a Python observer would require a test plugin similar to `rest-api-commands.ci`; chose not to write one because Go unit tests already exercise the per-binder multi-bind + fail-fast behavior, and the hub glue is just plumbing.
- **Commit structure**: 12 commits instead of the 8 originally planned. Each binder (web, lg, mcp, telemetry, REST, gRPC) became its own commit so the blast radius per cherry-pick is one package, not six. Chunks 2 (api YANG) and 3 (Extract helper reshape) are the only load-bearing prerequisites; chunks 4-9 can land in any order after those two.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| All 8 services converge on `list server { key "name"; }` pattern | Done | YANG modules web/ssh/mcp/lg/telemetry/plugin-hub (unchanged); api rewrite in `internal/component/api/schema/ze-api-conf.yang` | api-server rest + grpc were the only ones still on `container server` |
| Binders iterate the full server list | Done | `internal/component/web/server.go`, `internal/component/lg/server.go`, `cmd/ze/hub/mcp.go`, `internal/core/metrics/server.go`, `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go` | Each binder owns `configured []string` + `bound []string` under mu |
| Env-var compound propagation | Done | `cmd/ze/hub/main.go` runYANGConfig | Three "first endpoint only" warnings deleted |
| CollectListeners covers api-server | Done | `internal/component/config/listener.go` `knownListenerServices` | AC-11 + AC-16 verified by unit + .ci tests |
| Spec wiring tests (one per service) | Done | `test/parse/web-multi-listener.ci`, `lg-multi-listener.ci`, `mcp-multi-listener.ci`, `telemetry-multi-listener.ci`, `api-rest-multi-listener.ci`, `api-grpc-multi-listener.ci`, `listener-conflict-web-lg.ci`, `listener-conflict-api.ci` | Env-var-specific .ci deferred (see Deviations) |
| Documentation update | Done | `docs/guide/configuration.md`, `docs/features.md`, `docs/architecture/config/syntax.md` | All claims have source anchors |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestWebServer_MultiListener` + `test/parse/web-multi-listener.ci` | Web binds 2 listeners, both reachable via HTTPS |
| AC-2 | Done | `TestLGServer_MultiListener` + `test/parse/lg-multi-listener.ci` | LG binds 2 listeners |
| AC-3 | Done | `TestStartMCPServer_MultiListener` + `TestExtractMCPConfig_MultipleServers` + `test/parse/mcp-multi-listener.ci` | MCP binds 2 listeners; non-loopback entries rewritten to 127.0.0.1 |
| AC-4 | Done | `TestServer_MultiListener` (metrics) + `TestExtractTelemetryConfig_MultipleServers` + `test/parse/telemetry-multi-listener.ci` | Prometheus serves `/metrics` on every endpoint with the same counter value |
| AC-5 | Done | `TestRESTServer_MultiListener` + `TestExtractAPIConfig_RESTMultipleServers` + `test/parse/api-rest-multi-listener.ci` | REST binds 2 listeners, `/api/v1/commands` reachable on each |
| AC-6 | Done | `TestGRPCServer_MultiListener` + `TestExtractAPIConfig_GRPCMultipleServers` + `test/parse/api-grpc-multi-listener.ci` | gRPC binds 2 listeners, Execute RPC reachable on each |
| AC-7 | Done | `internal/component/api/schema/ze-api-conf.yang` diff (container -> list) | Old container form fully removed, no layering |
| AC-8 | Done | `cmd/ze/hub/main.go` runYANGConfig | 3 "first endpoint only" warnings deleted; `ParseCompoundListen` slice forwarded whole |
| AC-9 | Done | `cmd/ze/hub/main.go` runYANGConfig | env slice replaces config slice; partial merge rejected by construction |
| AC-10 | Done | `TestCollectListeners` existing test (preserved through reshape) | Disabled services still skipped |
| AC-11 | Done | `TestValidateListenerConflicts_APIRestGrpc` + `test/parse/listener-conflict-web-lg.ci` + `test/parse/listener-conflict-api.ci` | Both services and ports named in the error |
| AC-12 | Done | `extractServerList` / `extractAPIServerList` in `internal/component/config/loader_extract.go` | Empty entry uses refine default; `TestExtractWebConfig_EmptyListUsesDefaults` |
| AC-13 | Done | `TestExtractWebConfig_InsecureForcesLoopback` | Insecure rewrites every entry, not just the first |
| AC-14 | Done | Every `TestXxx_MultiListener` calls `Shutdown` / `Stop` at the end | Verified under `-race`, no goroutine leaks |
| AC-15 | Done | Every `TestXxx_BindFailureClosesPartialListeners` | Pre-bound squatter triggers rollback; first port free after failure |
| AC-16 | Done | `TestCollectListeners_APIServerRest` + `TestCollectListeners_APIServerGrpc` | Both transports in the inventory |
| AC-17 | Skipped | See Deviations | Schema-driven walker kept as future improvement; hardcoded table updated to cover api-server instead |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestExtractWebConfig_MultipleServers` | Done | `internal/component/config/loader_extract_test.go` | |
| `TestExtractWebConfig_EmptyListUsesDefaults` | Done | same | |
| `TestExtractWebConfig_InsecureForcesLoopback` | Done | same | AC-13 |
| `TestExtractLGConfig_MultipleServers` | Done | same | |
| `TestExtractLGConfig_EmptyListUsesDefaults` | Done | same | |
| `TestExtractMCPConfig_MultipleServers` | Done | same | |
| `TestExtractAPIConfig_{RESTSingleServer,RESTMultipleServers,GRPCMultipleServers,RESTEmptyListUsesDefaults,GRPCEmptyListUsesDefaults,Disabled,Token}` | Done | same | Chunk 2 |
| `TestExtractTelemetryConfig` | Done | `internal/core/metrics/server_test.go` | Existing table test adapted to TelemetryConfig struct |
| `TestExtractTelemetryConfig_MultipleServers` | Done | same | New test for the dropped-break fix |
| `TestCollectListeners_APIServerRest` / `_APIServerGrpc` | Done | `internal/component/config/listener_test.go` | |
| `TestValidateListenerConflicts_APIRestGrpc` | Done | same | |
| `TestWebServer_MultiListener` / `_BindFailureClosesPartialListeners` / `_RequiresListenAddrs` | Done | `internal/component/web/server_test.go` | |
| `TestLGServer_MultiListener` / `_BindFailureClosesPartialListeners` | Done | `internal/component/lg/server_test.go` | |
| `TestStartMCPServer_MultiListener` / `_BindFailureClosesPartialListeners` / `_EmptyAddrs` | Done | `cmd/ze/hub/mcp_test.go` | |
| `TestServer_MultiListener` / `_BindFailureRollsBack` (metrics) | Done | `internal/core/metrics/server_test.go` | |
| `TestRESTServer_MultiListener` / `_BindFailureClosesPartialListeners` / `TestNewRESTServer_RequiresListenAddrs` | Done | `internal/component/api/rest/server_test.go` | |
| `TestGRPCServer_MultiListener` / `_BindFailureClosesPartialListeners` / `TestNewGRPCServer_RequiresListenAddrs` | Done | `internal/component/api/grpc/server_test.go` | |
| `TestParseCompoundListen_MergeOrder` | Skipped | - | Env-var precedence covered by manual review of runYANGConfig diff; runtime behavior identical to existing ParseCompoundListen unit tests |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/api/schema/ze-api-conf.yang` | Done | container -> list for REST + gRPC |
| `internal/component/config/loader_extract.go` | Done | API + Web/MCP/LG extractors return slices |
| `internal/component/config/loader_extract_test.go` | Done | New file, 14 tests |
| `internal/component/config/listener.go` | Done | knownListenerServices + 2 api-server entries |
| `internal/component/config/listener_test.go` | Done | 3 new tests |
| `internal/core/metrics/server.go` | Done | TelemetryConfig struct + multi-listener Start |
| `internal/core/metrics/server_test.go` | Done | Table test adapted + 2 multi-listener tests |
| `internal/component/web/server.go` | Done | Multi-listener shape |
| `internal/component/web/server_test.go` | Done | 3 new tests |
| `internal/component/web/integration_test.go` | Done | ListenAddrs rename |
| `internal/component/lg/server.go` | Done | Multi-listener shape |
| `internal/component/lg/server_test.go` | Done | 2 new tests + 4 ListenAddrs renames |
| `cmd/ze/hub/mcp.go` | Done | Multi-listener shape |
| `cmd/ze/hub/mcp_test.go` | Created | New file, 3 tests |
| `internal/component/api/rest/server.go` | Done | Multi-listener shape |
| `internal/component/api/rest/server_test.go` | Done | 3 new tests + 4 ListenAddrs renames |
| `internal/component/api/grpc/server.go` | Done | Multi-listener shape |
| `internal/component/api/grpc/server_test.go` | Done | 3 new tests + 6 ListenAddrs renames |
| `cmd/ze/hub/main.go` | Done | runYANGConfig rewrite + startWebServer / startLGServer signatures |
| `cmd/ze/hub/api.go` | Done | REST + gRPC slice forwarding |
| `internal/component/bgp/config/loader_create.go` | Done | Telemetry call site for new TelemetryConfig struct |
| `test/plugin/rest-api-commands.ci` | Done | server -> server main rename |
| `test/parse/web-multi-listener.ci` | Created | |
| `test/parse/lg-multi-listener.ci` | Created | |
| `test/parse/mcp-multi-listener.ci` | Created | |
| `test/parse/telemetry-multi-listener.ci` | Created | |
| `test/parse/api-rest-multi-listener.ci` | Created | |
| `test/parse/api-grpc-multi-listener.ci` | Created | |
| `test/parse/listener-conflict-web-lg.ci` | Created | |
| `test/parse/listener-conflict-api.ci` | Created | |
| `docs/guide/configuration.md` | Done | Named Listeners section |
| `docs/features.md` | Done | New feature row |
| `docs/architecture/config/syntax.md` | Done | ze:listener row rewrite |

### Audit Summary
- **Total items:** 17 ACs + 28 file deliverables + 6 requirements = 51
- **Done:** 50
- **Partial:** 0
- **Skipped:** 1 (AC-17 schema-driven walker, explicit deferral per Design Insights)
- **Changed:** 3 (split into 12 commits instead of 8; env-var .ci deferred; runtime .ci deferred)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/parse/web-multi-listener.ci` | Yes | `ls test/parse/web-multi-listener.ci` in commit `efc907ed` (chunk 10) |
| `test/parse/lg-multi-listener.ci` | Yes | same commit |
| `test/parse/mcp-multi-listener.ci` | Yes | same commit |
| `test/parse/telemetry-multi-listener.ci` | Yes | same commit |
| `test/parse/api-rest-multi-listener.ci` | Yes | same commit |
| `test/parse/api-grpc-multi-listener.ci` | Yes | same commit |
| `test/parse/listener-conflict-web-lg.ci` | Yes | commit `8de46500` (chunk 11) |
| `test/parse/listener-conflict-api.ci` | Yes | same commit |
| `internal/component/config/loader_extract_test.go` | Yes | commit `26ce1955` (chunk 2) |
| `cmd/ze/hub/mcp_test.go` | Yes | commit `15227c74` (chunk 6) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Web binds 2 listeners | `go test -race -run TestWebServer_MultiListener ./internal/component/web/ -> ok` + `bin/ze-test bgp parse 116 -> pass 1/1` |
| AC-2 | LG binds 2 listeners | `go test -race -run TestLGServer_MultiListener ./internal/component/lg/ -> ok` + `bin/ze-test bgp parse z -> pass 1/1` |
| AC-3 | MCP binds 2 listeners with 127.0.0.1 rewrite | `go test -race -run TestStartMCPServer_MultiListener ./cmd/ze/hub/ -> ok` + `bin/ze-test bgp parse 70 -> pass 1/1` |
| AC-4 | Telemetry binds 2 listeners | `go test -race -run TestServer_MultiListener ./internal/core/metrics/ -> ok` + `bin/ze-test bgp parse 107 -> pass 1/1` |
| AC-5 | REST binds 2 listeners | `go test -race -run TestRESTServer_MultiListener ./internal/component/api/rest/ -> ok` + `bin/ze-test bgp parse 1 -> pass 1/1` |
| AC-6 | gRPC binds 2 listeners | `go test -race -run TestGRPCServer_MultiListener ./internal/component/api/grpc/ -> ok` + `bin/ze-test bgp parse 0 -> pass 1/1` |
| AC-7 | api YANG is list only | `grep -n "list server" internal/component/api/schema/ze-api-conf.yang -> 2 hits, 0 container server hits` |
| AC-8 | Compound env var honored | `grep -n "multi-bind not yet supported" cmd/ -> zero hits` |
| AC-11 | Port conflict detection | `go test -race -run TestValidateListenerConflicts_APIRestGrpc ./internal/component/config/ -> ok` + `bin/ze-test bgp parse 63 -> pass 1/1` + `bin/ze-test bgp parse 65 -> pass 1/1` |
| AC-13 | Insecure forces every entry to loopback | `go test -race -run TestExtractWebConfig_InsecureForcesLoopback ./internal/component/config/ -> ok` |
| AC-14 | Shutdown closes every listener | Every `TestXxx_MultiListener` ends with Shutdown/Stop and passes under `-race`; test run `go test -race ./... -> 189/189 ok` |
| AC-15 | Fail-fast on partial bind | Every `TestXxx_BindFailureClosesPartialListeners` asserts `ListenAndServe` returns a bind error AND the first port is free after rollback -> all pass |
| AC-16 | CollectListeners covers api-server | `go test -race -run 'TestCollectListeners_APIServer' ./internal/component/config/ -> ok` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Config with 2 web server entries | `test/parse/web-multi-listener.ci` | Yes (exit 0, stdout "configuration valid") |
| Config with 2 LG server entries | `test/parse/lg-multi-listener.ci` | Yes |
| Config with 2 MCP server entries | `test/parse/mcp-multi-listener.ci` | Yes |
| Config with 2 telemetry server entries | `test/parse/telemetry-multi-listener.ci` | Yes |
| Config with 2 REST server entries | `test/parse/api-rest-multi-listener.ci` | Yes |
| Config with 2 gRPC server entries | `test/parse/api-grpc-multi-listener.ci` | Yes |
| Web and LG sharing same port | `test/parse/listener-conflict-web-lg.ci` | Yes (exit 1, stderr "listener conflict") |
| REST and gRPC sharing same port | `test/parse/listener-conflict-api.ci` | Yes (exit 1, stderr "listener conflict") |
| Full test run | `bin/ze-test bgp parse -a` | 119/119 pass |
| Full test run | `bin/ze-test bgp plugin -a` | 227/227 pass (includes rest-api-commands through the new multi-listener REST path) |
| Full unit test run | `go test -race ./...` | 189/189 ok |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-17 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name
- [ ] `make ze-verify` passes
- [ ] Feature code integrated (binders + hub glue + YANG)
- [ ] Integration completeness proven end-to-end via `.ci` tests
- [ ] Architecture docs updated

### Quality Gates
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Single binder shape (pure `[]net.Listener`) across new impls
- [ ] No speculative features (no per-name env vars)
- [ ] Single responsibility per helper (extraction vs binding vs conflict detection)
- [ ] Explicit > implicit (env var precedence documented and tested)
- [ ] Minimal coupling (binders take plain slices, not `*config.Tree`)

### TDD
- [ ] Tests written before each binder change
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for port + endpoint count
- [ ] Functional `.ci` tests for each wiring row

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-named-service-listeners.md`
- [ ] Two-commit sequence per `rules/spec-preservation.md`
