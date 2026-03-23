# 416 -- Login Warnings

## Context

Operators connecting to ze via SSH had no way to see conditions requiring attention. Stale prefix data (older than 6 months) went unnoticed until a peer exceeded its maximum and was torn down. The goal was a generic warning system that displays actionable messages in the SSH welcome area, with prefix staleness as the first provider.

## Decisions

- Chose closure injection via `SetLoginWarnings` over a WarningProvider registry on the plugin server, because the SSH server has no reference to the plugin server. The closure pattern matches the established `SetExecutorFactory` / `SetStreamingExecutorFactory` / `SetShutdownFunc` / `SetRestartFunc` pattern used for all SSH-to-daemon wiring.
- Placed `LoginWarning` type in `cli` package (where rendering happens) over a shared package, because `ssh/session.go` already imports `cli`.
- Added panic recovery wrapper (`collectWarnings`) so a faulty provider never crashes SSH sessions. This is a deferred-call pattern rather than inline recover.
- Rendered warnings above the message area in `View()`, consuming padding space, rather than in the 2-line message area or the viewport. This supports any number of warnings without changing the `messageLines()` contract.
- Skipped empty `Message` warnings in rendering to avoid bare "warning:" lines.

## Consequences

- Future warning providers (RPKI cache, software version) add checks inside the same closure in `loader.go`. No registry infrastructure needed unless providers exceed 3-4.
- `PeerInfo.PrefixUpdated` is now exposed through the `ReactorLifecycle` interface, available to any consumer of `Peers()`.
- SSH test binaries must not import `reactor` package -- its `init()` chain registers YANG modules that conflict with the editor's `yang.DefaultLoader()`. Staleness logic should be inlined or tested via a separate package.

## Gotchas

- The original spec assumed the SSH server could access the plugin server directly. It cannot -- they are deliberately decoupled. Required redesign from registry to closure pattern.
- Importing `reactor` in SSH tests caused `TestSSHSessionGetsEditor` to fail with "failed to load YANG schema". The YANG `init()` side effects from reactor's import chain conflicted with the editor's schema loading. Fixed by inlining staleness logic in tests.
- The spec contained Go code snippets, violating `spec-no-code.md`. Had to be rewritten with tables and prose before implementation.
- `spec-update-verb` was listed as a dependency but was already implemented (code done, spec still marked `design`). The `ze update bgp peer * prefix` command exists and works.

## Files

- `internal/component/cli/warnings.go` -- LoginWarning type
- `internal/component/ssh/warnings.go` -- LoginWarningsFunc type, collectWarnings panic recovery
- `internal/component/ssh/ssh.go` -- SetLoginWarnings method, loginWarningsFunc field
- `internal/component/ssh/session.go` -- createSessionModel wiring
- `internal/component/cli/model.go` -- loginWarnings field, SetLoginWarnings method
- `internal/component/cli/model_render.go` -- warning rendering in View()
- `internal/component/bgp/config/loader.go` -- staleness closure in post-start hook
- `internal/component/bgp/reactor/reactor_api.go` -- PrefixUpdated in Peers() adapter
- `internal/component/plugin/types.go` -- PrefixUpdated field on PeerInfo
- `docs/features.md` -- login warnings documentation
