# 533 -- BGP Boundary Cleanup

## Context

After making BGP a config-driven plugin (learned/530), the hub still hard-imported 4 BGP packages (`bgp/config`, `bgp/server`, `bgp/grmarker`, `bgp/transaction`). This meant deleting the BGP component would break the hub, web, interface -- everything. The goal was to achieve zero BGP imports in the hub and main entry points so components remain truly independent.

## Decisions

- Chose type alias (`reactor.PluginConfig = plugin.PluginConfig`) over separate types, eliminating the conversion that forced `bgp/reactor` imports.
- Chose registration-based RPC handlers (`registry.AddRPCHandlers` + lazy `CollectRPCHandlers`) over hardcoded `RPCFallback` function pointer, because `bgp/server` can't be imported from `plugin/all` (cycle via `plugin/server`). The `bgp/server/register.go` init() registers handlers; server collects lazily on first dispatch via `sync.Once`.
- Chose `config.RegisterPluginExtractor` hook over moving `ExtractPluginsFromTree` wholesale, because the inline BGP plugin extraction (`ResolveBGPTree` + peer process bindings) is deeply BGP-specific. BGP registers its extractor at init time; the generic `config.LoadConfig` calls all registered extractors.
- Chose `config.RegisterMigrateFunc` callback over direct import, because `config/migration` imports `config` (parent), creating a cycle if `config` imports `config/migration`. The migration package registers itself at init time.
- Chose `registry.ReactorFactoryFunc` over coordinator closure for the reactor factory, because the factory needs `bgp/config.CreateReactor` + `grmarker` + chaos injection -- all BGP-specific. Hub stores plain values via `coordinator.SetExtra`; BGP plugin retrieves them and builds the reactor.
- Chose `FatalOnConfigError` registration field over `proc.Name() == "bgp"` hardcoding.
- Chose `SetCommitManager` on `PluginServerAccessor` over `ServerConfig.CommitManager`, because `transaction.NewCommitManager` imports `bgp/nlri` and `bgp/rib`.

## Consequences

- Hub and `cmd/ze/main.go` have zero `bgp/*` imports. Deleting BGP does not break them.
- `cmd/ze/config/cmd_validate.go` retains `bgpconfig` intentionally (validates BGP config content).
- Three new init()-based registration hooks: `RegisterPluginExtractor`, `RegisterMigrateFunc`, `RegisterReactorFactory`. All set-once-at-init, read-at-runtime.
- `config/loader.go` is the canonical config loading entry point. `bgp/config` retains reactor creation, GR markers, chaos injection, authz/SSH extraction.
- Any future protocol component (e.g., OSPF) can register its own plugin extractor and reactor factory using the same hooks.

## Gotchas

- Import cycles blocked every "obvious" approach. `bgp/plugin` can't import `bgp/config` (test cycle via `plugin/all`). `bgp/plugin` can't import `bgp/server` (same). `config` can't import `config/migration` (parent-child cycle). Each required a registration-based indirection.
- The duplicate-name hook (`check-existing-patterns.sh`) blocks creating types that share names with types in other packages (caught `WebConfig` collision with `web.WebConfig`). Renamed to `WebListenConfig`/`MCPListenConfig`/`LGListenConfig`.
- `append(nil, slice...)` shares the backing array -- caught by review. Fixed with `slices.Clone`.
- `Reset`/`Snapshot`/`Restore` in the registry must include every new global (`rpcHandlers`). Missed on first pass, caught by review.

## Files

- `internal/component/config/loader.go` (new) -- LoadConfig, ParseTreeWithYANG, ExtractPluginsFromTree, plugin extractors
- `internal/component/config/loader_extract.go` (new) -- WebListenConfig, MCPListenConfig, LGListenConfig, ExtractHubConfig
- `internal/component/config/migration/register.go` (new) -- migration function registration
- `internal/component/bgp/config/register.go` (new) -- inline plugin extractor + reactor factory
- `internal/component/bgp/server/register.go` (new) -- codec RPC handler registration
- `internal/component/plugin/registry/registry.go` -- RPCHandlers, FatalOnConfigError, AddRPCHandlers, CollectRPCHandlers
- `internal/component/plugin/registry/interfaces.go` -- ReactorFactoryFunc, SetCommitManager
- `internal/component/plugin/server/server.go` -- lazy getRPCHandlers, SetCommitManager
- `cmd/ze/hub/main.go` -- zero BGP imports
- `cmd/ze/main.go` -- zero BGP imports
