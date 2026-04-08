# 539 -- decouple-0-umbrella

## Context

Components under `internal/component/` had wrong-direction imports: web imported ssh for authentication, config imported iface for MAC completion, bgp/config imported cli/ssh/web for server creation, reactor imported all cmd packages via blank imports, and cmd packages imported bgp for RPC handlers. These couplings meant removing one component would break unrelated ones.

## Decisions

- Moved `UserConfig`, `CheckPassword`, `AuthenticateUser` from ssh to authz (over keeping in ssh with a re-export) because authentication is a shared concern, not SSH-specific.
- Added global `CompleteFn` registry in config/yang (over modifying the ValidatorRegistry API to accept late-bound callbacks) because init()-time registration matches ze's existing pattern and needs zero API changes.
- Used an `InfraHook` callback pattern to extract SSH/CLI wiring from bgp/config (over moving code to a shared package) because it preserves the existing startup flow while breaking the import.
- Moved BGP-specific RPCs from cmd/* to bgp/plugins/cmd/ (over keeping them in cmd with an interface abstraction) because proximity principle -- handlers belong with the domain they serve.
- Kept protocol-agnostic cmd blank imports in reactor (over moving to plugin/all) because `all_import_test.go` files create import cycles with any package that imports `plugin/server`.

## Consequences

- Components can now be removed independently: web has zero ssh imports, config has zero iface imports, bgp/config has zero cli/ssh/web imports, cmd has zero bgp imports.
- The `InfraHook` pattern introduces a global callback in bgpconfig -- if the hook is not set (e.g., tests calling `LoadReactor` directly), SSH/authz wiring is silently skipped.
- AC-3 (zero cmd imports in reactor) remains blocked: `all_import_test.go -> plugin/all -> cmd -> plugin/server` cycles. Fixing this requires extracting `RegisterRPCs` from `plugin/server` into a separate registration package.
- The codegen infrastructure (`discoverRPCPackages` in `scripts/codegen/plugin_imports.go`) is ready but produces zero imports due to the same cycle.
- Phase 2 spec (cli/contract for ssh->cli, web->cli coupling) is deferred as spec-decouple-1-cli-contract.

## Gotchas

- `all_import_test.go` files are the primary cycle blocker. They import `plugin/all` for test setup, but `plugin/all` transitively imports every RPC package, which imports `plugin/server`. Any package that imports `plugin/server` cannot be in `plugin/all`.
- The `webSrv` variable in the old `CreateReactorFromTree` was always nil (web server creation had already moved to hub). The web import and shutdown code were dead code.
- `loadZefsUsers` existed in both bgpconfig and hub with nearly identical implementations. The bgpconfig version became dead code after the hook extraction.
- `cmd/show` still needs a blank import in reactor because it registers non-BGP RPCs (version, warnings, errors, interface). Removing it from reactor would lose those RPCs.

## Files

- `internal/component/authz/auth.go` -- moved auth types from ssh
- `internal/component/config/yang/validator_registry.go` -- global CompleteFn registry
- `internal/component/iface/validators.go` -- MAC CompleteFn registration
- `internal/component/bgp/config/infra_hook.go` -- InfraHook types and callback
- `internal/component/bgp/config/loader_create.go` -- hook call replaces SSH wiring
- `cmd/ze/hub/infra_setup.go` -- hub-side infrastructure wiring
- `internal/component/bgp/plugins/cmd/cache/` -- moved from cmd/cache
- `internal/component/bgp/plugins/cmd/commit/` -- moved from cmd/commit
- `internal/component/bgp/plugins/cmd/peer/peer.go` -- added CLI verb RPCs
- `scripts/codegen/plugin_imports.go` -- RPC discovery infrastructure
- `docs/architecture/core-design.md` -- component boundaries section
