# 351 — Reactor Lifecycle Interface Split

## Objective

Split the 18-method `ReactorLifecycle` interface into 5 focused sub-interfaces per the interface segregation principle, composed back into `ReactorLifecycle` for backwards compatibility.

## Decisions

- **5 sub-interfaces by consumer cluster:** ReactorIntrospector (4), ReactorPeerController (6), ReactorConfigurator (5), ReactorStartupCoordinator (4), ReactorCacheCoordinator (2)
- **Merged lifecycle + peer config into ReactorPeerController:** Stop/TeardownPeer/PausePeer/ResumePeer and AddDynamicPeer/RemovePeer are always consumed together (handler/bgp.go), so one 6-method interface instead of two tiny ones
- **Composed ReactorLifecycle preserved:** Defined as embedding all 5 sub-interfaces, so all existing code compiles unchanged — zero-risk refactor
- **Phase 1 only:** Sub-interface definitions + composition. Consumer narrowing deferred to arch-0

## Patterns

- Go interface composition (`io.ReadWriteCloser` pattern) enables incremental adoption — consumers narrow their types one at a time
- Consumer-driven interface boundaries beat mechanical concern grouping — group by "who uses these together", not by abstract category
- Compile-time `var _ Interface = (*Type)(nil)` checks are sufficient for pure type-level refactors; runtime tests would be testing the Go compiler

## Gotchas

- **Adapter pattern:** ReactorLifecycle is implemented by unexported `reactorAPIAdapter` wrapping `*Reactor`, not by `*Reactor` directly. Compile-time checks must reference the adapter type. Always grep for the actual method receiver before assuming which type implements an interface.

## Files

- `internal/component/plugin/types.go` — 5 sub-interfaces + composed ReactorLifecycle
- `internal/component/bgp/reactor/reactor_test.go` — compile-time interface satisfaction checks
