# 888 -- l2tp-env-promote

## Context

Five L2TP env vars (auth timeout, reauth interval, NCP enable-ipcp, enable-ipv6cp,
NCP timeout) were operator-facing settings that belonged in YANG config but were
implemented as env-only during initial L2TP development. They were invisible in
`show configuration`, had no validation, and required restarts to change.

## Decisions

- Promoted to YANG containers `authentication {}` and `ncp {}` inside `l2tp {}`,
  over flat leaves at the l2tp root, because the grouping mirrors the PPP phase
  structure and avoids name collisions (both containers have a `timeout` leaf).
- Removed env vars entirely (no backward compatibility) over keeping env as override,
  because the user explicitly requested it and eliminating dual config surfaces removes
  the precedence confusion risk.
- Used YANG `range "0 | 5..86400"` for reauth-interval over runtime clamping,
  because YANG validation rejects invalid values at config commit rather than silently
  clamping at session creation. This deleted `clampReauthInterval` and its test.
- Used `uint16` for auth/NCP timeout (1-3600) and `uint32` for reauth-interval
  (0-86400) because 86400 exceeds uint16 max (65535).

## Consequences

- Operators must move from env vars to `l2tp { authentication { ... } ncp { ... } }`
  in config. No migration shim exists.
- The reauth safety floor (5s minimum) is now a YANG constraint. Any future change
  to the floor requires a YANG schema revision.
- ReactorParams now carries all PPP session config; reactor_kernel.go no longer
  imports the env package.

## Gotchas

- `newUnstartedReactor` test helper had zero-value ReactorParams, which meant
  EnableIPCP/EnableIPv6CP defaulted to false and AuthTimeout to 0. Had to update
  both `newUnstartedReactor` and `newUnstartedReactorWithLogs` to set production
  defaults, or existing tests would break.

## Files

- `internal/component/l2tp/yang/ze-l2tp-conf.yang` -- +2 containers, +5 leaves
- `internal/component/l2tp/config.go` -- Parameters, ExtractParameters, constants
- `internal/component/l2tp/reactor.go` -- ReactorParams, removed clampReauthInterval
- `internal/component/l2tp/reactor_kernel.go` -- uses r.params, removed env import
- `internal/component/l2tp/subsystem.go` -- wires new ReactorParams fields
- `internal/component/config/environment.go` -- removed 2 auth env registrations
- `internal/component/l2tp/config_test.go` -- +14 tests
- `internal/component/l2tp/reactor_ppp_linux_test.go` -- converted 4 tests
- `internal/component/l2tp/reactor_kernel_linux_test.go` -- updated test helper
- `test/parse/l2tp-auth-ncp.ci` -- new functional test
- `docs/guide/l2tp.md` -- updated config examples
