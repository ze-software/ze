# 920 -- mpls-ldp

## Context
Spec `mpls-2-ldp` implements LDP (RFC 5036). The audit found discovery, session
FSM, LIB, FIB integration, wire codec and an FRR-ldpd interop test already built,
but closure surfaced that the component **never actually worked through real
config** and its show commands crashed -- because the only coverage was unit tests
that drove the engine directly, bypassing the config parser and the CLI dispatch.
Remaining feature gap: AC-9 dynamic interface reload.

## Decisions
- Dynamic reload via a `discoveryManager.reconcile(cfg)` that diffs the desired
  interface set against running per-interface goroutines (start added, cancel
  removed) -- chosen over restarting the whole engine so sessions on unchanged
  links are undisturbed. Stopping discovery is enough: with no Hellos, the
  neighbor's adjacency ages out through the existing hold-timer path and its
  session tears down, so no explicit per-interface session teardown is needed.
- The discovery `startFn` is a seam injected by the manager, so reconcile is
  unit-testable without real multicast I/O.
- Doctor `ldp-port` check probes binding UDP+TCP 646 (privileged port) via a test
  seam, registered through `Registration.DoctorChecks` (self-contained), not the
  central `runChecks` list.
- `.ci` functional tests are single-daemon (config→engine→show); full session
  establishment needs a mock LDP peer, deferred to the FRR integration test.

## Consequences
- Adding/removing an LDP interface in config now takes effect without a restart.
- `show ldp neighbor`/`binding` now actually dispatch (were recursive before).
- LDP is now exercised end-to-end through the real binary by `ze-test ldp` (3/3).
- AC-10 cross-vendor interop is QEMU-green: `make ze-qemu-ldp-frr-test` runs
  `frr_interop_integration_linux_test.go` against a real FRR ldpd (10.2.1) in an
  Alpine VM (session up, label exchange) -- the closure evidence for live interop.

## Gotchas
- **Plugin config delivery shape is the trap.** `Tree.ToMap` + `BuildPluginConfigSections`
  deliver config as **root-wrapped** (`{"ldp":{...}}`), with **string-typed numbers**
  (`"5"`), **scalar-or-array leaf-lists**, and **keyed-map YANG lists**. A parser that
  expects unwrapped/numeric/array data silently produces an empty config → engine logs
  "no lsr-id configured, engine idle" and does nothing. Unit tests that build the engine
  config struct directly will NOT catch this -- pin the delivered shape in a parser test.
- A builtin show-proxy that re-`Dispatch`es its own command string recurses until the
  stack overflows; use `RPCRegistration.PluginCommand` + `Dispatcher.ForwardToPlugin`
  (canonical pattern in `bgp/plugins/cmd/rib/rib.go`).
- Two `.ci` files referenced a `protocol { ldp { interface X } }` schema that the YANG
  never defined; the real form is top-level `ldp { ... interfaces <name> }`.

## Files
- `internal/plugins/ldp/discovery_manager.go` (+test), `register.go`, `doctor.go` (+test), `cmd_show.go` (+test), `config_test.go`
- `internal/core/diagnostic/codes.go` (`doctor-ldp-port-unavailable`)
- `internal/test/cli/register.go` (`ze-test ldp`); `test/ldp/{ldp-session,ldp-convergence,ldp-reload}.ci`
