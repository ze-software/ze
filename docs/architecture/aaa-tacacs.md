# AAA Chain and TACACS+

Ze delegates authentication, command accounting and command authorization to a
central TACACS+ server (RFC 8907), behind a pluggable AAA chain. Local bcrypt
stays as the fallback backend.

<!-- source: internal/component/aaa/aaa.go -- Authenticator, Authorizer, Accountant, Default registry -->
<!-- source: internal/component/tacacs/client.go -- TACACS+ client -->
<!-- source: internal/component/tacacs/cli/main.go -- ze tacacs show reachability probe -->

## A registry, not a hardcoded chain

Each backend implements `Authenticator`, `Authorizer` or `Accountant` and its
`Build()` reads the YANG tree to return its contributions. The hub composes them
in priority order: TACACS+ at 100, local bcrypt at 200. A later RADIUS, LDAP or
OIDC backend is a new package plus a blank import in
`internal/component/aaa/all/all.go`, with no change to the SSH server, the
dispatch hook or the bundle lifecycle.

## Reject and unreachable are different, and the chain owns the difference

`aaa.ErrAuthRejected` stops the chain. Any other error tries the next backend.

Without that distinction, a wrong TACACS+ password falls through to the local
bcrypt hash. That is a security regression wearing the costume of a resilience
feature: the central server said no, and the box says yes.

The same rule is what keeps the recovery path open. An operator who makes
TACACS+ the only backend can still be rescued by the ZeFS super-admin, which
sits outside the AAA chain by design.

## An unmapped privilege level is a denial

A TACACS+ user whose priv-lvl has no `tacacs-profile` entry is denied. This
differs from a local user, where the absence of a profile means admin. Adding a
new level on the upstream server must never silently grant access on the box.

## Own the wire code

The TACACS+ implementation is native. `nwaples/tacplus` is unmaintained and has
known buffer-allocation defects, and `facebookincubator/tacquito` is
server-focused. RFC 8907 is a 12-byte header, an MD5 pseudo-pad XOR and three
message types, so ze owns it and follows ze naming.

<!-- source: internal/component/tacacs/packet.go -- PacketHeader, Encrypt, UnmarshalPacket -->
<!-- source: internal/component/tacacs/authen.go -- authentication exchange -->
<!-- source: internal/component/tacacs/author.go -- authorization exchange -->
<!-- source: internal/component/tacacs/acct.go -- accounting exchange -->

Single-connect (RFC 8907 section 4.4) keeps a per-server connection pool under
a mutex. The first packet on a fresh TCP connection sets the single-connect
flag (0x04). The connection is pooled only when the server echoes the flag on
its reply. A dead pooled connection is evicted on a read or write error and the
caller retries once.

## Accounting hangs off one dispatch point

Every dispatched command, from SSH exec, the interactive TUI, the local CLI and
the API, converges on `Dispatcher.Dispatch()`. START is sent after
authorization passes and STOP through a `defer` after the handler returns. One
hook covers every entry point, and an accounting failure is logged and never
blocks the command.

<!-- source: internal/component/plugin/server/command.go -- accountant hook in Dispatcher -->

Every config reload swaps the bundle atomically and closes the previous one, so
the accounting workers drain. A test that counts "N enqueued, N sent" must
tolerate a dropped tail during the stop.

## The schema merge defect this work uncovered

`internal/component/config/schema.go` `Define` merged only the top-level
container children. A second YANG module extending an already-registered nested
container silently lost its children: `ze-tacacs-conf` extended
`system.authentication`, which `ze-ssh-conf` already owned. `ze schema show`
listed neither module, so nothing pointed at the cause. The fix is a recursive
`mergeContainer` and `mergeNode`, and it protects any future module that
extends a shared container.

<!-- source: internal/component/config/schema.go -- Define, mergeContainer, mergeNode -->

## Assert on the SSH log, not the component log

The daemon log level defaults to WARN, and the TACACS+ authenticator logs its
success at Info in the `hub.infra` subsystem. The SSH-side line
`SSH auth success ... source=tacacs profiles=[...]` is in the `ssh` subsystem
and is on by default, so it is the robust wiring assertion for a functional
test and it carries the mapped profiles as proof of the priv-lvl mapping.

<!-- source: internal/component/ssh/ssh.go -- SSH auth success log -->

## Config surface constraints

- The leaves are `type boolean default false`, not presence-only `type empty`.
  The ze config parser expects a value for a leaf, so a presence leaf would
  need a parser change to ship this feature. The boolean form also keeps the
  verb explicit: `set ... accounting true`.
- `flag.Parse` stops at the first non-flag argument, so `ze tacacs show
  <config> --json` treats `--json` as garbage. The flag goes first. This
  matches the rest of the ze CLI and is stated in the command help.
- Two identical server addresses in `tacacs.server` produce disambiguated YANG
  list keys (`127.0.0.1` and `127.0.0.1#1`).
