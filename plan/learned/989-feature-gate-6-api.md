# 989 -- feature-gate child 6: REST/gRPC compile-out (per-encoding ze_rest + ze_grpc)

## Context

Child 6 of the feature-gate umbrella (`plan/spec-feature-gate-0-umbrella.md`): make
the REST and gRPC API servers compile-out-able. The spec proposed a combined `ze_api`
tag (rest+grpc together) because they share one `startAPIServers` and one
`ze-api-conf.yang` module. Mid-implementation the user asked to split them so a build
can ship **gRPC-without-REST or vice-versa** (the spec's R-1 risk). REST (HTTP/JSON/
OpenAPI) and gRPC (protobuf/grpc-go) are independent encodings with independent server
packages -- worth dropping one without the other.

## Decisions

- **Two independent tags, dedicated seam.** `ze_rest` gates `internal/component/api/rest`,
  `ze_grpc` gates `internal/component/api/grpc`. `api_infra.go` (always-on) declares two
  nil-able hooks (`restBuild`/`grpcBuild`) + `apiBuildInputs` (generic) + `apiShared`
  (parent-api engine/sessions/authenticator). `service_rest.go`/`service_grpc.go`
  (gated) install the hooks via `register_rest.go`/`register_grpc.go`. Chose a seam
  over the construction registry because the two transports share one engine/sessions
  and wire to two migrator slots (same reasoning as the spec's combined seam).
- **YANG split via Ze's container-merge, not augment.** `ze-api-conf.yang` keeps the
  always-on base `environment { api-server { token } }`; the `rest{}`/`grpc{}`
  containers move to `internal/component/api/rest/yang/ze-rest-conf.yang` (ze_rest) and
  `internal/component/api/grpc/yang/ze-grpc-conf.yang` (ze_grpc), each declaring
  `environment { api-server { <transport> } }`. Ze merges same-named containers across
  modules (10 modules declare `container environment`; the merge recurses into nested
  `api-server`), so the full tree reassembles when both are compiled in, and a
  compiled-out transport's container is unregistered -> its config block is rejected as
  unknown. Chose merge over `augment` (both supported) because merge needs no
  cross-module import / target-path.
- **Shared engine/sessions built always-on** (`buildAPIShared` in api.go) using only the
  parent `internal/component/api` package; each gated transport builds only its server
  from the shared state. `SetREST`/`SetGRPC` widened to `Reconfigurable` (drop the
  apigrpc/rest imports from listener_migrate.go). Parent `api` package + base api/yang
  stay always-on (gNMI uses the parent's ConfigSessionManager).
- **The generator gates the transport schemas for free.** Manifest entries
  `ze_rest internal/component/api/rest` + `ze_grpc internal/component/api/grpc` make
  `loadFeatureTags` map `api/rest/yang -> ze_rest` and `api/grpc/yang -> ze_grpc`, so
  the schemas land in `all_ze_rest.go`/`all_ze_grpc.go`. Base api/yang stays in all.go.

## Consequences

- nm matrix: `ze_core` rest=0 grpc=0; `ze_core ze_rest` rest=119 grpc=**0**;
  `ze_core ze_grpc` rest=**0** grpc=79. All four combos compile. `ze`/`ze-appliance`
  link both (ZE_FEATURES); `ze-stripped`/`ze_core` drop both.
- A neutral "shared parent always-on, transport children gated" YANG pattern: when one
  config container has independently-disableable children, keep the parent in an
  always-on base module and contribute each child from a gated module via container-merge.

## Gotchas

- **The YANG split breaks committed config tests that import only the base `api/yang`**
  while testing rest/grpc config (the rest/grpc schemas relocated to gated packages).
  Fix: add blank imports of `api/rest/yang` + `api/grpc/yang` to those tests
  (`loader_extract_test.go`, `api_schema_check_test.go`, `all_schemas_test.go`). Schemas
  moved, not removed -- not test weakening.
- **doctor + config tests are build-tag-dependent.** They rely on schemas registered via
  build tags (not direct imports), so they fail under bare `ze_core` (ssh gated since
  981; api now gated). Not a regression: the unit suite runs shipped tags
  (`GO_TEST_TAGS` = manifest gates) where they pass. Test under the all-gates set, not
  bare ze_core.
- **A unit seam test must not call `buildAPIShared` with an unstarted server.**
  `buildAPIShared` does `go sessions.RunCleanup(server.Context())`; an unstarted
  `pluginserver.NewServer` has a nil Context() -> panic in the goroutine. Test the
  "transport not enabled" path (returns a zero handle before touching the shared state).
- **`ze-doc-test` caught spec-5 debt:** 3 stale `cmd/ze/hub/mcp.go` source anchors
  (mcp.go was renamed to service_mcp.go in 987) -> repointed to service_mcp.go. Run
  `make ze-doc-test`, not just the source-anchors you add.

## Files

- Created: `cmd/ze/hub/api_infra.go`, `service_rest.go`+`register_rest.go` (ze_rest),
  `service_grpc.go`+`register_grpc.go` (ze_grpc), build_tag_rest/grpc_present+absent
  tests, service_rest/grpc_test.go;
  `internal/component/api/rest/yang/ze-rest-conf.yang`,
  `internal/component/api/grpc/yang/ze-grpc-conf.yang` (+generated glue);
  `internal/component/plugin/all/all_ze_rest.go`, `all_ze_grpc.go` (generated)
- Modified: `cmd/ze/hub/{api.go,listener_migrate.go,main.go}` (seam + widen + buildAPIShared),
  `internal/component/api/yang/ze-api-conf.yang` (base only),
  `internal/component/config/{loader_extract,api_schema_check,all_schemas}_test.go`
  (relocated schema imports), `feature-gates.txt`, `.golangci.yml`,
  `internal/component/plugin/all/all.go`, `docs/features.md`, `ai/rules/architecture.md`,
  `ai/rules/plugins.md`, `docs/architecture/mcp/overview.md` +
  `docs/guide/configuration.md` (stale mcp.go anchors)
