# 1065 -- ownership-3-reactor-modes

## Context

The BGP reactor inferred its plugin-server ownership at runtime from `r.api != nil`
inside `startAPIServer` (`externalServer = r.api != nil`). In production the hub
injects its server (borrow); when nothing was injected the reactor silently
self-hosted (created its own server, ran its own signal handler, waited for plugin
startup, started peers inline). The self-hosting regime is a real shipped mode
(ze-chaos in-process sim + integration harness + `ze bgp --child`), not cruft, but
the *selection* was implicit -- so a production wiring bug (missing SetPluginServer)
would silently start a second server instead of erroring. DESIGN-REVIEW finding #1
(reactor dual-mode) remnant.

## Decisions

- Added `Config.Standalone bool` (default false = borrow) over inverting to a
  `Borrow` flag: the default must be production-safe. A new caller that forgets to
  set the mode gets borrow, which ERRORS without an injected server (loud) rather
  than silently self-hosting (the bug). `externalServer` is derived once in `New`
  as `!config.Standalone`; the runtime inference at `startAPIServer` is replaced by
  a guard `if r.externalServer && r.api == nil { return errBorrowModeNoServer }`.
- Threaded the flag through `bgpconfig` only (no hub->bgp import): added a
  `standalone bool` param to the internal-only `CreateReactor`/`CreateReactorFromTree`,
  kept the public `LoadReactor*` at borrow-default, and added
  `LoadReactorWithPluginsStandalone` (ze-chaos) + `LoadReactorFileStandalone`
  (`ze bgp --child`). Production `createReactorFromCoordinator` passes false.
- Migrated `ze bgp --child` to standalone (user decision) rather than leaving it to
  break; it is un-spawned today but still compiles and self-hosts.

## Consequences

- The six `!externalServer` gates (own signals, plugin-startup waits, inline
  validatePeerFamilies, self-host server creation, owned-server StartWithContext,
  abort guard) needed NO logic change -- only the *source* of `externalServer` moved
  from runtime to construction. They run during StartWithContext (after New), so the
  earlier assignment is always available.
- Any future reactor consumer must now state its mode. The borrow guard converts a
  missed standalone opt-in from a silent second-server bug into an immediate startup
  error -- exactly the invariant the spec wanted.

## Gotchas

- The blueprint estimated ~3 reactor tests needed migration; in reality **38** reactor
  unit tests self-host (they construct `New(&Config{...})` and Start without a hub),
  so all needed explicit `Standalone: true`. The borrow-default default surfaces how
  pervasively the reactor's own suite self-hosts. Migration is safe *only* for tests
  that do NOT inject a server: a test that calls SetPluginServer must stay borrow
  (adding Standalone:true there would make the self-host branch create a second
  server over the injected one). The failing-test set (the borrow error) precisely
  identifies the self-hosting tests; injecting tests never failed.
- A borrow reactor with a `ListenAddr` binds its listener BEFORE `startAPIServer`, so
  the borrow guard errors *after* a listener exists -- `abortStartup` must (and does)
  release it. `TestReactorBorrowModeErrorsWithoutServer` proves not-Running after the
  error.
- Assumption A-1 (two non-production self-hosting consumers) was wrong: there are
  three -- ze-chaos in-process, the integration harness (4 reactor.New sites), and
  `ze bgp --child`.

## Files

- `internal/component/bgp/reactor/reactor.go` -- `Config.Standalone`, `errBorrowModeNoServer`, `externalServer: !config.Standalone` in New, borrow guard in startAPIServer
- `internal/component/bgp/reactor/reactor_startup_test.go` -- 3 new AC tests (AC-1/2/3) + 4 startup tests migrated to Standalone
- `internal/component/bgp/reactor/{reactor_test.go,reload_test.go,panic_recovery_test.go}` -- 34 self-hosting tests set Standalone: true
- `internal/component/bgp/config/loader.go`, `loader_create.go` -- `standalone` param threaded; `LoadReactorWithPluginsStandalone` + `LoadReactorFileStandalone`
- `internal/component/bgp/config/register.go` -- production `createReactorFromCoordinator` passes borrow (false)
- `internal/chaos/inprocess/runner.go`, `internal/component/bgp/cli/childmode.go` -- migrated to the standalone loaders
- `test/integration/integration_test.go` -- 4 reactor.New sites set Standalone: true
