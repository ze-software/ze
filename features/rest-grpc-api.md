# REST/gRPC API

## Meta

| Field | Value |
|-------|-------|
| Name | REST/gRPC API |
| Kind | daemon |
| Scope | partial |
| Scope gaps | API completion |
| Level | experimental |
| Components | internal/component/api |
| Real-path tests | test/plugin/rest-execute.ci, test/plugin/grpc-execute.ci, test/plugin/rest-api-commands.ci, test/plugin/rest-no-auth-readonly.ci, test/parse/api-rest-multi-listener.ci, test/parse/api-grpc-multi-listener.ci |
| Docs | docs/guide/api.md |
| Doc review | 2026-10-10: OpenAPI 3.1 generation read in internal/component/api/schema.go OpenAPISchema |
| Defect review | 2026-10-07: spec-yang-rpc-declarations-with-no-handler (closed 2026-10-10) and journal 2026-09-03 name ze-bgp-api.yang handlers, not the transports |
| Extra criteria | supported: SSE and gRPC streaming of monitor event = test/plugin/api-stream-monitor-event.ci |

## Description

Programmatic API with OpenAPI 3.1 spec, config sessions. Both transports accept multiple named listen endpoints via `environment.api-server.rest.server <name>` / `.grpc.server <name>`. REST is plaintext and therefore loopback-only; expose it remotely only behind a TLS terminator. Non-loopback authenticated gRPC listeners require TLS. Bearer token auth, per-user auth, CORS support. Both transports share one engine for identical command output. SSE and gRPC streaming are wired to registered streaming commands such as `monitor event`, using the same authorization and accounting path as SSH monitor commands. Completion remains future work. Each transport is independently compile-out-able: `ze_rest` gates `internal/component/api/rest` and `ze_grpc` gates `internal/component/api/grpc`, so a build can ship gRPC-without-REST or vice-versa. normal and appliance builds include both; `ze-stripped` and bare `ze_core` drop both. With a transport compiled out its server code and its config container (`rest{}`/`grpc{}`) are absent, so that block is rejected as unknown; the shared `api-server { token }` base and the parent `internal/component/api` engine stay always-on (gNMI uses the parent). <!-- source: feature-gates.txt -- ze_rest, ze_grpc --> <!-- source: cmd/ze/hub/service_rest.go -- ze_rest REST factory --> <!-- source: cmd/ze/hub/service_grpc.go -- ze_grpc gRPC factory --> <!-- source: internal/component/api/rest/yang/ze-rest-conf.yang -- gated rest{} schema --> <!-- source: internal/component/api/engine.go -- APIEngine, Execute, Stream, ListCommands --> <!-- source: internal/component/api/rest/server.go -- RESTServer, all HTTP handlers, multi-listener Serve --> <!-- source: internal/component/api/grpc/server.go -- GRPCServer, ZeService, ZeConfigService, multi-listener Serve -->
