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

## A backend that will not BUILD is dropped, and the chain carries on

The section above is about a backend that ANSWERS. This one is about a backend
that never starts. A TACACS+ server declared with no shared secret is one case,
and any error out of `Build` is another.

`Build` DROPS such a backend, logs it at ERROR, and composes the chain from what
is left.

It used to return on the first error instead. That took the LOCAL backend down
with the broken one, because local sits at priority 200 and was never reached. A
keyless TACACS+ server left the daemon with no authenticator at all.

So the rule an operator needs is the chain's own, and the section above states
it. A user who exists in both places is asked of the remote backend first. The
local account answers only where the remote one failed to ANSWER. A reject is an
answer.

| What broke | Who authenticates | Who authorizes |
|------------|-------------------|----------------|
| A backend will not build | the backends that did, in priority order | the first authorizer they contributed, which is the local RBAC store where local built |
| Every backend will not build | nobody. There is no bundle, and ssh is not started | nothing, and no session exists to ask |

That second row is what "no user, no login" means. A listener that can
authenticate nobody is a port rather than a service, so `infraSetup` does not
start one. `liveAAABundleAuthorizer` refuses every command for the same reason:
no bundle means no policy was ever installed to consult.

**A reload refuses rather than dropping anything.** `ze config commit` rebuilds
the chain, and `Build` returns the dropped backend's error beside the composed
bundle. The reload path treats that error as a refusal and keeps the running
chain, while boot logs it and runs with what composed. One build, two callers,
two answers.

That refusal holds only while a bundle is already installed. After a boot whose
build dropped every backend the slot is nil, the rebuild is skipped, and a
corrected config needs a restart.

`ze doctor` names the risk before any of this happens. `doctor-aaa-no-local-fallback`
warns when a remote backend is configured and no `system.authentication.user`
is, because the chain's fallback is an ACCOUNT and that config declares none.

<!-- source: internal/component/aaa/doctor.go -- checkAAALocalFallback -->
<!-- source: internal/component/aaa/types.go -- backendRegistry.Build, which drops and composes -->
<!-- source: cmd/ze/hub/infra_setup.go -- the ssh build condition -->
<!-- source: cmd/ze/hub/aaa_lifecycle.go -- liveAAABundleAuthorizer -->
<!-- source: cmd/ze/hub/main_reload.go -- the reload refusal -->

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

Boot builds the AAA bundle once. A later BGP infrastructure hook reuses the same
bundle, so an open session and its accounting pair keep live backends. Daemon
shutdown closes the bundle and drains its accounting workers.

A config reload builds a replacement and swaps it in. It has to: the local
backend re-reads the accepted credentials on every login. And a RADIUS or
TACACS+ client holds the address, the shared secret and the timeout it was
constructed with. The replacement is built while the reload can still fail, and
it is installed at the same acceptance point that publishes the new credentials.
A reload that fails after that build closes the replacement, so its socket and
its accounting worker do not leak. The swap closes the chain it replaces, which
is what drains the retired TACACS+ accounting worker.

Every management surface reads the installed bundle on each call. ssh receives
`liveAAABundleAuthenticator` and `liveAAABundleAuthorizer` rather than the
fields of the bundle built at boot. As a result, an operator who rotates a
shared secret gets the new one on the next login and needs no restart.
<!-- source: cmd/ze/hub/infra_setup.go -- boot-owned AAA bundle reuse -->
<!-- source: cmd/ze/hub/main_reload.go -- candidate bundle build and acceptance-point swap -->
<!-- source: cmd/ze/hub/aaa_lifecycle.go -- claimAAABundleBoot, swapAAABundle, closeAAABundle -->

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
- The probe carries no rendering flag (`ai/rules/cli.md`). `ze tacacs show
  <config>` prints the rows in the configured default format and adds the
  reachability verdict as its exit code. No daemon is needed: the probe reads
  the config file and dials from the operator's own process.
- Two identical server addresses in `tacacs.server` produce disambiguated YANG
  list keys (`127.0.0.1` and `127.0.0.1#1`).
