# 601 -- TACACS+ AAA (RFC 8907)

## Context

Ze authenticated SSH logins against local bcrypt only. Operating an Exa-scale
network fleet from a central TACACS+ server requires every device to delegate
authentication, log every dispatched command (accounting), and optionally
authorize commands per priv-lvl. The challenge was layering this in without
locking operators out when the central server is unreachable, and without
silently letting a wrong-password reply against TACACS+ fall through to a stale
local hash. The work also had to land alongside the existing zefs super-admin
without disturbing recovery paths.

## Decisions

- **Pluggable AAA registry over a hardcoded chain.** Built `internal/component/aaa`
  with `Authenticator`/`Authorizer`/`Accountant` interfaces and a `Default`
  registry. Each backend's `Build()` reads the YANG tree and returns its
  contributions; the hub composes them in priority order (TACACS+ = 100, local
  bcrypt = 200). Future RADIUS/LDAP backends slot in without touching SSH or
  hub wiring.
- **Native TACACS+ implementation, not a Go library.** `nwaples/tacplus` is
  unmaintained (5 years, known buffer-allocation bugs); `facebookincubator/tacquito`
  is server-focused. RFC 8907 is short -- 12-byte header, MD5 pseudo-pad XOR,
  three message types -- so we own the wire code and follow ze patterns
  (exported `PacketHeader`, `Encrypt`, `UnmarshalPacket`).
- **Reject vs unreachable distinction is a chain primitive, not a backend
  concern.** `aaa.ErrAuthRejected` short-circuits the chain on FAIL; any other
  error tries the next backend. Without this, a wrong TACACS+ password would
  fall through to local bcrypt -- a security regression masquerading as a
  resilience feature.
- **Unmapped priv-lvl rejects (AC-18).** TACACS+ users with a priv-lvl not in
  `tacacs-profile` are denied access. Differs from local users (no profile =
  admin); chosen because adding new levels in the upstream server should never
  silently grant access.
- **Accounting through `Dispatcher.Dispatch()`.** All dispatched commands (SSH
  exec, interactive TUI, local CLI, API) converge at one point; START is fired
  after authorization passes, STOP via `defer` after the handler returns. Single
  hook covers every entry point. Failures are logged and never block the
  command.
- **External mock binary (`ze-test tacacs-mock`) over an internal test helper.**
  `.ci` functional tests need a server reachable via `$PATH`, not an in-package
  `_test.go` helper. Reuses exported tacacs primitives so wire bugs surface in
  both layers.
- **Boolean leaves (`type boolean default false`) over presence-only `type empty`.**
  The ze config parser does not yet handle empty-leaf presence syntax. Switching
  to boolean keeps the verb (`set ... accounting true`) explicit and avoids a
  parser change to ship the feature.

## Consequences

- Adding a new AAA backend (RADIUS, LDAP, OIDC) is a new package + blank-import
  in `internal/component/aaa/all/all.go`; the SSH server, dispatch hook, and
  bundle lifecycle do not change.
- Atomic bundle swap on every config reload + `Close()` of the previous bundle
  means TACACS+ accounting workers drain cleanly across reloads. Tests that
  observe "N enqueued, expect N sent" must tolerate dropped tail messages
  during stop -- documented in `accounting.go`.
- The chain's reject vs unreachable rule means an operator who wants TACACS+ to
  be the *only* path can still get rescued by zefs super-admin (which lives
  outside the AAA chain by design). Security review accepted this as a feature,
  not a hole.
- TACACS+ support adds a row to `docs/comparison.md` Security: only Ze, FRR, and
  freeRtr offer TACACS+ AAA among compared daemons.

## Gotchas

- **Schema merge was shallow** for nearly two years. `internal/component/config/schema.go::Define`
  only merged top-level container children, so when a second YANG module
  (`ze-tacacs-conf`) extended an already-registered nested container
  (`system.authentication` already owned by `ze-ssh-conf`), its children were
  silently dropped. `ze schema show` did not list ssh-conf or authz-conf either,
  so the symptom was easy to miss. Recursive `mergeContainer`/`mergeNode` fix
  also benefits any future module that extends a shared container.
- **`#!/bin/sh` + `set -e` + trap behavior.** The `.ci` scripts run under dash
  (Debian's `/bin/sh`). With `set -e`, a non-zero command in a trap (here `wait
  $PID` returning 143 after killing the daemon) propagates as the script's exit
  status, ignoring the explicit `exit 0`. Fix: `|| true` after every command
  in the trap, not just the last one.
- **`type empty` YANG leaves not yet supported by ze parser.** Spec proposed
  presence-only `type empty`; parser expects a value. Either fix the parser or
  use `type boolean default false` -- chose the latter for scope.
- **`process substitution` (`>(tee ...)`) is bash-only.** First attempt at
  streaming daemon stderr live used `2> >(tee daemon.log >&2)`; dash rejects
  with `Syntax error: redirection unexpected`. Replaced with explicit
  `2>daemon.log` then `cat daemon.log >&2` at the end.
- **slog Info filtered by default.** Daemon defaults to WARN. The tacacs
  authenticator logs `TACACS+ auth success` at Info, which is invisible without
  `ze.log.tacacs=info`. The SSH-side log `SSH auth success ... source=tacacs` is
  always visible (separate slog subsystem) and serves as the wiring proof in
  `.ci` tests -- prefer it over component-internal logs for assertion patterns.

## Files

- `internal/component/aaa/{aaa,types}.go` -- interfaces + chain
- `internal/component/aaa/all/all.go` -- backend blank-imports
- `internal/component/tacacs/{client,packet,authen,author,acct,authenticator,authorizer,accounting,config,register}.go`
- `internal/component/tacacs/schema/{embed,register,ze-tacacs-conf.yang}`
- `internal/component/config/schema.go` -- recursive merge fix
- `cmd/ze/hub/{aaa_lifecycle,infra_setup,main}.go` -- bundle lifecycle, RemoteAddr wiring
- `internal/component/plugin/server/command.go` -- accountant hook in Dispatcher
- `cmd/ze-test/{main,tacacs_mock}.go` -- mock server
- `test/plugin/tacacs-{auth,fallback,local-only,acct}.ci` -- wiring tests
- `rfc/short/rfc8907.md` -- RFC summary
- `docs/{features,comparison}.md`, `docs/guide/{tacacs,configuration}.md`,
  `docs/architecture/core-design.md` -- documentation
