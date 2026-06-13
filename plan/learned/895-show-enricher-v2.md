# 895 -- Show Enricher v2: External Plugin + Web + Test Infrastructure

## Context

The v1 show enricher registry (spec 894) supported only in-process plugins registering enrichers via init(). External (out-of-process) plugins had no way to contribute data to show commands, and web service-locator pages that bypassed CLI dispatch couldn't leverage enrichment. This spec extends the enricher registry to three new surfaces: external plugin enrichment via the SDK protocol, web service-locator enrichment via explicit show.Enrich() calls, and permanent test infrastructure (fakeenrich in-process plugin + Python .ci test plugin).

## Decisions

- Chose declaring enrichers at Stage 1 registration over plugin-initiated registration after ready, following the doctor-check pattern and avoiding a race window where show commands run without external enrichers.
- Chose 2s timeout on external enricher callback over no timeout or configurable timeout, because a show command enricher should be fast and a hung plugin must not block the CLI.
- Chose proxy enricher in the server package over core/show, because core/show is a leaf package (stdlib only) and the proxy needs IPC access which requires server imports.
- Chose simple top-level key merge (maps.Copy) for external enricher responses over nested merge, matching in-process enricher behavior (mutate base in place).
- Chose sync.Once for RegisterProcessCleanup over init() in enricher.go, because the hook blocks init() with registration calls in non-register files.
- Chose Go fakeenrich + Python .ci for testing over one language only: Go plugin guards in-process regression permanently, Python .ci exercises external SDK path end-to-end.

## Consequences

- External plugins can now declare enrichers at registration and the server registers a proxy enricher that calls back via IPC. This completes the enricher story: any plugin (in-process, internal, or external) can contribute data to any show command.
- The Unregister function enables per-key removal for plugin exit cleanup, replacing the coarse ResetForTest wipe.
- Web service-locator pages (e.g., L2TP session detail) can now include enriched data by constructing a side map and calling show.Enrich() explicitly.
- The fakeenrich test plugin provides a permanent in-process enrichment regression guard.

## Gotchas

- The proxy enricher captures the PluginConn pointer in a closure. If the plugin exits, the connection is closed and SendEnrichShow returns an error, which the proxy enricher logs as a warning. The show command continues with partial data. This is safe because show.Enrich() recovers from panics.
- The fakeenrich test plugin needed to be a full registry-registered plugin (with RunEngine and OnExecuteCommand) so that the .ci test could dispatch `show test enrich` through the standard command pipeline. A simple enricher-only plugin without a command handler would not be dispatchable.
- JSON round-trip through the external enricher path converts uint16 values to float64. Enrichers receiving external data must handle float64 for numeric fields.

## Files

- `internal/core/show/show.go` -- added Unregister(command, key)
- `internal/core/show/show_test.go` -- 3 new Unregister tests
- `pkg/plugin/rpc/types.go` -- added EnricherDecl, EnrichShowInput, EnrichShowOutput, Enrichers field
- `pkg/plugin/rpc/types_test.go` -- 4 new JSON round-trip tests
- `pkg/plugin/sdk/sdk_callbacks.go` -- added OnEnrichShow callback, EnrichShowHandler type, default handler
- `pkg/plugin/sdk/sdk_callbacks_test.go` -- 2 new SDK callback tests
- `pkg/plugin/sdk/sdk_dispatch.go` -- added callbackEnrichShow constant
- `pkg/plugin/sdk/sdk_types.go` -- added EnricherDecl, EnrichShowInput, EnrichShowOutput aliases
- `internal/component/plugin/ipc/rpc.go` -- added SendEnrichShow method
- `internal/component/plugin/server/enricher.go` -- proxy enricher registration, cleanup, makeProxyCall
- `internal/component/plugin/server/enricher_test.go` -- 3 proxy enricher tests
- `internal/component/plugin/server/startup.go` -- register proxy enrichers during Stage 1
- `internal/component/web/handler_l2tp.go` -- added show.Enrich() in HandleL2TPDetail
- `internal/test/plugins/fakeenrich/fakeenrich.go` -- test plugin with command handler
- `internal/test/plugins/fakeenrich/register.go` -- show.MustRegister + registry.Register
- `internal/test/plugins/fakeenrich/fakeenrich_test.go` -- 2 enricher registration tests
- `internal/test/plugins/all/all.go` -- added fakeenrich blank import
- `test/plugin/show-enricher-external.ci` -- external enrichment .ci test
- `test/plugin/show-enricher-fakeenrich.ci` -- fakeenrich .ci test
- `test/scripts/ze_api.py` -- added declare_enricher, on_enrich_show, enrich-show dispatch
- `docs/features.md` -- updated enricher description
- `ai/patterns/registration.md` -- added external enricher section
- `ai/INDEX.md` -- updated enricher keywords
