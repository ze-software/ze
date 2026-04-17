# 620 -- l2tp-7 subsystem wiring

## Context

The L2TPv2 subsystem had core protocol code (handshake, session FSM,
kernel/PPP drivers) in place from spec-l2tp-1 through spec-l2tp-6c,
but three external touchpoints were still missing: the SIGHUP-driven
Reload was a stub; operators had no CLI to list tunnels/sessions or
force teardown; and the `redistribute l2tp` source did not exist, so
subscriber routes were invisible to the protocol RIB. spec-l2tp-7
wires all three. Events (`l2tp.*`) and Prometheus metrics moved to
spec-l2tp-9 / spec-l2tp-10 so this spec stayed focused.

## Decisions

- **Diff-apply Reload policy** (agreed with user): hot-apply
  `shared-secret`, `hello-interval`, `max-tunnels`, `max-sessions`;
  reject `enabled` flip and listener-endpoint changes with WARN. The
  tunnel FSM carries per-tunnel state (sequence numbers, kernel fds)
  that would be invalidated by pushing new values onto live tunnels;
  listener rebind needs a full driver teardown that is safer as an
  explicit restart. Live tunnels are never disturbed just because
  config text changed.
- **Two-tree CLI**: `show l2tp ...` augments the existing `show` tree
  (BFD precedent) for 8 read operations; destructive operations live
  under a new top-level `l2tp` tree (BGP `peer teardown` precedent)
  for 4 teardown variants. Keeping read and write trees separate lets
  the `config false` read path stay cleanly in the YANG schema.
- **Service locator in the `l2tp` package itself**, not a
  sub-package. A `l2tp/api` sub-package would need to import `l2tp`
  types, creating the `l2tp -> l2tp/api -> l2tp` cycle. Placing
  `Service` + `PublishService` / `LookupService` directly in the
  `l2tp` package is the cleanest workaround; CLI handlers in
  `internal/component/cmd/l2tp/` import the runtime package for
  both types and locator, matching the shape of BFD's
  `internal/plugins/bfd/api` but without the extra indirection.
- **Kernel-probe env gate** (`ze.l2tp.skip-kernel-probe`): the real
  `modprobe l2tp_ppp / pppol2tp` probe requires CAP_NET_ADMIN, which
  the dev CI environment does not grant. A Private env var flipped
  only by the `.ci` test lets the wiring test boot ze without
  privileges; production leaves the var unset so the real probe
  runs.
- **Redistribute split** (agreed with user, reinforced by code
  review): spec-l2tp-7 registers the `l2tp` source and tracks
  subscriber routes via a RouteObserver with in-memory state and
  counters, but the actual BGP RIB write path is deferred to
  spec-l2tp-7c-rib-inject. The `bgp rib inject` surface today is
  CLI-text-only; a non-CLI programmatic inject entry is larger than
  spec-l2tp-7 can absorb and will serve future non-BGP sources
  (static, connected, OSPF).
- **`.ci` coverage scoped pragmatically**: 16 of 17 planned `.ci`
  tests moved to spec-l2tp-7b-ci-coverage; all require a working L2TP
  handshake (kernel modules) or SIGHUP routing that the dev-test
  environment does not support. One representative wiring test
  (`show-l2tp-empty.ci`) proves end-to-end dispatch from observer ->
  CLI handler -> service locator -> subsystem -> reactor. Unit tests
  (18 new) cover handler logic, reload diff-apply, snapshot
  formatting, and route observer behaviour.

## Consequences

- Operators now have a first-class way to inspect and control L2TP
  state at runtime without reading logs or restarting. `ze l2tp show
  ...` works from any shell; `ze cli` (or direct SSH exec) dispatches
  the daemon-side handler when interactive.
- SIGHUP reload is no longer a no-op. Operators iterating on
  `shared-secret` or `hello-interval` can apply changes without
  dropping live tunnels. They must still restart for listener rebind
  and enable/disable flips, which is documented and logged.
- `redistribute l2tp` is now a valid config keyword. Future specs
  that wire actual RIB injection (spec-l2tp-7c) can do so without
  changing any L2TP-side code; the RouteObserver hands off the
  tuple (session-id, username, addr, family) via a narrow interface.
- The `internal/component/l2tp/Service` publication pattern
  (via `PublishService` + `atomic.Pointer`) is the template for any
  future subsystem that wants to expose a CLI surface without
  importing into the cmd/ package -- same shape as bfd, minus the
  sub-package indirection.
- The `ze.l2tp.skip-kernel-probe` env var is test-only, but it does
  expose a bypass hatch to operators. Marked `Private: true` so it's
  hidden from `ze env list` and autocomplete. Future specs should
  respect that pattern: test-only gates are Private and documented
  in-line, not user-facing knobs.

## Gotchas

- Range-val-copy lint fires for `l2tp.TunnelSnapshot` (176 bytes) and
  `l2tp.SessionSnapshot`. The CLI helpers take pointers
  (`tunnelJSON(*TunnelSnapshot, ...)`, `sessionJSON(*SessionSnapshot, ...)`)
  to avoid the copy; every caller has to pass `&snap.Tunnels[i]`
  instead of the range-value form. Test code outside the hot path
  copies freely.
- `plugin.Response.Data` is `any`, not `string`. Tests must type-assert
  via a helper (`responseString(t, r)`) before `json.Unmarshal`.
  Passing `[]byte(resp.Data)` directly is a compile error.
- The `require-related-refs.sh` hook blocks writing a new file whose
  `// Related:` points at a file that does not yet exist. Two
  workarounds: write the target file first, or drop the forward-ref
  until the pair lands. Forgetting this wastes ~3 edit cycles.
- `check-existing-patterns.sh` blocks Write of a new `.go` whose
  first declared function name matches an existing function anywhere
  in `internal/`. `SetService` / `GetService` collide with
  `internal/plugins/bfd/api/registry.go`. Resolved by using
  `PublishService` / `LookupService` instead. Generic names
  (`Service`, `State`, `Manager`) will collide; pick unique or
  package-qualified names before the hook rejects the file.
- The BFD-as-plugin `.ci` precedent in `test/plugin/` implicitly
  configures the TLS plugin hub via the BGP peer block's
  `process ...` directive; an L2TP-only test still needs a BGP peer
  declaration to trigger hub setup. This is a known oddity of the
  test runner, documented in spec-l2tp-7b deferral notes.
- `reactor.handlePPPEvent` previously returned silently for
  `EventSessionIPAssigned` -- the switch explicitly ignored it. Adding
  route-observer routing required special-casing that event before
  the fallthrough `tid == 0 && sid == 0` "unknown event" guard, or
  the guard fires first and logs a false warning.
- `newTunnel` gained a `time.Time` parameter for `createdAt`. Three
  test call sites (session_fsm_test.go, reactor_kernel_linux_test.go)
  needed updating in lockstep. The builder would catch it but every
  call site is in a test file, so `go vet` + the existing test run
  is sufficient.

## Files

- `internal/component/l2tp/` — 8 new files
  (`snapshot.go`, `teardown.go`, `service_locator.go`,
  `subsystem_reload.go`, `subsystem_snapshot.go`,
  `reactor_setters.go`, `route_observer.go`, `redistribute.go`)
  plus modifications to `subsystem.go`, `reactor.go`, `session.go`,
  `tunnel.go`, `session_fsm.go`, `config.go`
- `internal/component/l2tp/schema/ze-l2tp-api.yang` — 12 RPC defs
- `internal/component/cmd/l2tp/` — new package (`l2tp.go`, `schema/`)
- `internal/component/plugin/all/all.go` — blank imports
- `cmd/ze/l2tp/show.go` — offline forwarder
- `test/plugin/show-l2tp-empty.ci` — wiring test
- `docs/guide/l2tp.md` — operator guide
- `plan/spec-l2tp-7b-ci-coverage.md`, `plan/spec-l2tp-7c-rib-inject.md`
  — follow-up specs for deferrals
- `plan/deferrals.md` — 3 new deferral rows
