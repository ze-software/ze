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

The ZeFS super-admin uses the separately assigned reserved recovery profile.
On web login, only a successful local result carrying that profile can override
a central rejection. Ordinary configuration users cannot use the web fallback
to bypass it; an unavailable backend still permits local authentication.

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

A successful PAP reply is followed by a session authorization request carrying
`service=shell` and an empty `cmd`. Its `priv-lvl` argument selects the configured
`tacacs-profile` entries. Authentication reply data never supplies privileges.
A level with no mapped profiles is denied, including level zero. An absent
`priv-lvl` uses level one; a mandatory malformed or oversized value denies login.

## Own the wire code

The TACACS+ implementation is native. `nwaples/tacplus` is unmaintained and has
known buffer-allocation defects, and `facebookincubator/tacquito` is
server-focused. RFC 8907 is a 12-byte header, an MD5 pseudo-pad XOR and three
message types, so ze owns it and follows ze naming.

<!-- source: internal/component/tacacs/packet.go -- PacketHeader, Encrypt, UnmarshalPacket -->
<!-- source: internal/component/tacacs/authen.go -- authentication exchange -->
<!-- source: internal/component/tacacs/author.go -- authorization exchange -->
<!-- source: internal/component/tacacs/acct.go -- accounting exchange -->

Single-connect (RFC 8907 section 4.3) keeps a per-server connection pool under
a mutex. The first packet on a fresh TCP connection sets the single-connect
flag (0x04). The connection is pooled only when the server echoes the flag on
its reply. A dead pooled connection is evicted on a read or write error and the
caller retries once.

The receive path checks that the decrypted component lengths consume the entire
body before it retains a connection. Trailing bytes, truncated fields and invalid
display text close that connection. Unknown header flag bits are ignored, while
an unexpected unencrypted flag closes the connection and returns a terminal
denial. An ERROR status tries the next configured server; an explicit FAIL,
RESTART or FOLLOW cannot fall through to a backup decision.

Usernames use the RFC 8265 UsernameCasePreserved profile: width mapping and NFC
normalization preserve case, and disallowed Unicode or invalid bidirectional
text is refused. Other text fields, including PAP passwords, use printable
US-ASCII. Authentication reply data remains binary. Reply fields own their
storage, and the client clears wire scratch before returning it to the pool.

Synthetic dispatch actors stay in AAA's NUL-prefixed local namespace. The wire
codec represents an internal plugin/RPC actor or shared-API actor as
`~ze~r:<base64url(raw identity bytes)>`, without padding. Human usernames first
undergo PRECIS preparation; a resulting name beginning `~ze~` becomes
`~ze~u:<base64url(prepared username bytes)>` in PAP, AUTHOR and ACCT. These
disjoint encodings prevent a printable lookalike, including a width-mapped one,
from becoming a synthetic actor. No wire value grants local reserved authority.
Unknown reserved names and NUL-prefixed authentication inputs remain invalid.
The encoded username must fit 255 bytes: at most 186 input bytes fit either
encoded branch. Longer identities are refused, never hashed or shortened.
Local authorization and unreachable-server fallback still receive the original
local identity.
<!-- source: internal/component/tacacs/text.go -- prepareWireUsername -->

## Authorization arguments are enforced

Command authorization uses `service=shell`, `cmd` and ordered `cmd-arg` values.
PASS_ADD retains the request arguments and applies the response; PASS_REPL
replaces them. The dispatcher executes the original command, so a response
that removes, reorders or changes that command's arguments is denied.

Command values use Go quoted-string escapes without surrounding quotes:
`café` becomes `caf\u00e9`, and literal backslashes are doubled. Ordinary
printable text without quotes or backslashes is unchanged. The encoding is
lossless; execution and local fallback use the original arguments. An
authorization request that cannot fit the wire limits is denied, never shortened.
The server's command policy must interpret this display encoding consistently.
<!-- source: internal/component/tacacs/command_text.go -- asciiCommandValue -->

Ze rejects mandatory attributes it cannot enforce, including session timeouts,
ACL assignment and privilege changes on an individual command. Unsupported
optional attributes may be ignored. Session authorization additionally supports
decimal `priv-lvl` values from zero through fifteen. Servers must use this
dictionary consistently; a PASS status alone cannot grant an unsupported policy.

## Accounting hangs off one dispatch point

Every dispatched command, from SSH exec, the interactive TUI, the local CLI and
the API, is accounted before lookup and authorization. Denied and unknown
commands therefore produce records as well. Streaming entry points use the
same accounting hook and retain the STOP callback until the stream finishes.
Accounting failures are logged and never refuse command execution; a full queue
waits for capacity instead of discarding commands.

Accounting receives the redacted display tokens from dispatch. After ASCII
escaping, each AV is at most 255 bytes and each request has at most 255 AVs.
An oversized display carries `ze-command-truncated=1` and
`ze-command-sha256=<hex digest>` before the service/command arguments. The
digest covers the complete redacted AV sequence, each preceded by its unsigned
64-bit big-endian byte length. Long fields end with `...` at an escape boundary;
excess arguments are omitted. This is explicitly a bounded display, not a
reconstructable full command, and its digest never includes the redacted secrets.
Both START and STOP use the same task ID and display digest. The original command
still reaches execution; this accounting-only bound never approves lossy policy.
<!-- source: internal/component/tacacs/command_text.go -- accountingArguments -->

<!-- source: internal/component/plugin/server/command.go -- accountant hook in Dispatcher -->

Boot builds the AAA bundle once. A later BGP infrastructure hook reuses the same
bundle, so an open session and its accounting pair keep live backends. Daemon
shutdown closes the bundle and drains accepted accounting records.

A config reload builds a replacement and swaps it in. It has to: the local
backend re-reads the accepted credentials on every login. And a RADIUS or
TACACS+ client holds the address, the shared secret and the timeout it was
constructed with. The replacement is built while the reload can still fail, and
it is installed at the same acceptance point that publishes the new credentials.
A reload that fails after that build closes the replacement, so its socket and
its accounting worker do not leak. Outstanding commands retain their original
accountant through STOP; retirement closes that bundle after its last command
finishes. Task identifiers remain unique across configuration generations.

Every management surface reads the installed bundle on each call. SSH password
results bind their login-resolved profile names to `liveAAABundleAuthorizer`,
so an existing session's next command uses the newly configured TACACS+ server.
New logins use `liveAAABundleAuthenticator` and the replacement shared secret.
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
- Active duplicate server addresses are rejected by the configuration parser.
  Each address is the YANG list key.

Shared-secret fields are redacted in diagnostic rendering, structured logs and
JSON output. The configuration's `$9$` representation is reversible obfuscation,
so the configuration file needs the same access protection as credentials.
`ze doctor` warns about keys shorter than sixteen characters. Keys of at least
thirty-two characters are supported without truncation.

RFC 8907 section 10.5 requires a deployment with transport privacy, integrity and
separation from other traffic. MD5 body obfuscation supplies none of those
guarantees; a reachable server or configured source address does not establish
that the deployment meets them.
