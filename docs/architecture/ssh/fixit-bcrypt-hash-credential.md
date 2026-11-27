# The bcrypt hash is a local-only credential, and it is masked on display

The stored bcrypt password hash was accepted as a credential in its own right.
That branch existed for the on-box CLI, which sends the stored hash, but it was
reachable over EVERY transport: remote SSH, web basic and form authentication,
and the REST and gRPC bearer path. The same hash was exported unmasked by the
show commands, the web views, the config dump and the config download. Any
config read was therefore a remote escalation from read-only to administrator.

The fix has four parts: restrict hash-as-token to local transports, mask the
bcrypt leaf on display, gate the raw download behind the edit authorization, and
redact credential tokens in the command log.

<!-- source: internal/component/ssh/passwordauth.go -- authenticatePasswordResult, isLocalTransport, loggedCommand -->
<!-- source: internal/component/authz/auth.go -- CheckPassword, authenticateUser -->
<!-- source: internal/component/config/mask.go -- LeafHoldsSecret, MaskBcrypt, MaskSecrets, MaskSecretsInPlace, SecretKeys, DisplayValueAtPath, DisplayMessageAtPath, MaskSecretInMessage, RejectMaskedSecretLeaves -->
<!-- source: internal/component/config/schema.go -- Schema.LookupTokenPath -->
<!-- source: internal/component/cli/editor_mask.go -- DisplayContentAtPath, DisplayOriginalContentAtPath -->
<!-- source: internal/core/redact/redact.go -- IsBcryptHash, IsPasswordHash, PrivateKeyPEM, Command, JSON, URL, URLError, Placeholder -->
<!-- source: internal/component/cli/transcript.go -- TranscriptWriter.Record -->

## Decisions

**The transport signal is an explicit boolean whose zero value means remote.**
Only the SSH password callback sets it, from the accepted socket peer: a unix
socket or a loopback TCP connection is local. Web and API never set it, so they
always reject the hash as a token. `CheckPassword` and `authenticateUser` take
the permission as a required parameter, not an option, so a future caller has to
state the transport class. The socket peer is the only signal that cannot be
spoofed, and it lives at the transport layer, not inside the authorization
package.

**Masking happens on a display CLONE, never in the shared serializers.**
`MaskSecrets` clones the tree and replaces every non-empty secret leaf value
with the secret-data placeholder. It reads `LeafHoldsSecret`, which is the one
answer to "does this leaf hold a secret". `MaskSecretsInPlace` serves a caller
that already holds a private clone. `MaskBcrypt` stays narrow for the config
dump, which writes a sensitive value back in its reversible form. The mask is
applied at each display choke point: the CLI show, annotated, diff and search
paths, the display twins of the unmasked accessors, the web CLI terminal
serializers, the web per-leaf builders, and the config dump.

Command recall uses `DisplayCommandAtPath` before publication and when projecting
legacy records. It resolves relative configuration addresses and leading
`top`/`run` words through the same schema-aware display mask. The executed
command retains its original bytes; the history display copy may use an
absolute `top set` address to preserve a relative assignment's target.
Editor-owned history binds `Editor.DisplayCommandAtPath` before either mode is
loaded or recorded. It supplies the editor's actual schema to
`DisplayCommandWithSchema`; an operational owner without an editor uses the
process-wide producer. An unavailable schema still masks assignments closed.
<!-- source: internal/component/config/mask.go -- DisplayCommand, DisplayCommandAtPath, DisplayCommandWithSchema -->
<!-- source: internal/component/cli/editor_mask.go -- DisplayCommandAtPath -->
<!-- source: internal/component/cli/history.go -- Snapshot, Record -->

**Map-shaped data is masked by leaf NAME.** `SecretKeys` answers that set from
the same predicate. The BGP resolver flattens group and peer inheritance, so a
path in the resolved map addresses no schema node. The config dump and the
config diff both read a name there.

**What a command echoes back is masked by PATH.** `DisplayValueAtPath` answers
the text a command may print for the value the operator supplied, and
`DisplayMessageAtPath` answers the same for the sentence that refuses it. This
is the third shape, and no tree mask fits it: an acknowledgement is written from
the operator's own tokens before any tree is read. `set %s %s`, the dry-run
line, the SSH CLI status line, the web terminal answer, the adoption prompt of
`ze config edit` and both sides of a commit conflict each wrote the raw value.
Both functions read the same predicate through `Schema.LookupTokenPath`, which
resolves a path that interleaves a list name with one entry's key.

**The path mask fails CLOSED.** A nil schema resolves nothing and an unknown
path resolves nothing, so neither can be told from a path that names a
credential, and both answer the placeholder. A node that is not a leaf answers
its value: `ze:sensitive` and `ze:bcrypt` are fields of `LeafNode`, so no other
node kind can carry the marking.

**The placeholder is never the reversible sensitive-value marker.** A sensitive
leaf uses a reversible encoding; bcrypt is one way and the parser refuses the
reversible marker on a bcrypt leaf. In the config dump the sensitive encoder
skips a value that already equals the placeholder, because a leaf name can be
both bcrypt in one module and sensitive in another.

**The commit guard fails closed.** `RejectMaskedSecretLeaves` rejects a secret
leaf that holds either the tree display placeholder `SECRET-DATA` or command
recall's `<redacted>` rather than silently resolving it. Ordinary leaves may
retain those strings as literal values. It reads
`LeafHoldsSecret`, so it covers a `ze:sensitive` leaf as well as a `ze:bcrypt`
one, and it answers the one predicate the display mask reads. It is wired
at every commit and validate entry point, which is also what backs the web
upload. The web value setter no-ops a resubmitted placeholder as a
user-interface backstop.

**The redaction regex has one home.** The core redact package owns the
bcrypt-shape pattern and the config helper delegates to it. `redact.IsPasswordHash`
answers for every stored format Ze accepts: bcrypt, and the SHA-crypt `$5$` and
`$6$` hashes of migrated accounts. Its SHA-crypt shape is looser than the
verifier's parser (`internal/core/shacrypt`), so a near-valid hash is hidden too
and redaction fails closed. Command and JSON redaction scrub every such token and password-family key
values BEFORE the log truncation, so a secret straddling the cut cannot
half-leak.

**Four shapes of input, four entry points, and each surface asks the one that
fits it.** A surface leaks because it did not ask, never because the answer was
missing, so the vocabulary is short and each shape has exactly one home.

| The input | The entry point | Where it lives |
|-----------|-----------------|----------------|
| A command line the operator typed | `redact.Command` | `internal/core/redact` |
| A captured JSON payload | `redact.JSON` | `internal/core/redact` |
| A URL the operator configured, and the transport failure carrying one | `redact.URL`, `redact.URLError` | `internal/core/redact` |
| A config leaf value | `config.DisplayValueAtPath`, `config.DisplayMessageAtPath` | `internal/component/config` |

The split is not arbitrary. The first three read the STRING, so they live in a
leaf package any tier may import. The fourth reads the YANG SCHEMA to decide
whether the leaf at that path is marked, so it cannot live below the config
component.

The config command-display producer judges a `set` line by the schema first
and hands every other line, and a `set` line whose leaf holds no secret, to
`redact.Command`. History uses the keyword producer directly for operational
CLI-format settings, which are not configuration assignments. Other command
records reach it through `DisplayCommand`: the accounting record
(`Dispatcher.BeginAccounting`, and `beginAccountingArgs` for a typed
inter-plugin call), the audit log entry (`recordCommandAudit`, which records
builtins whose registration declares an `AuditAction`, and records denied and
error outcomes for `RequireExplicitAuthz` commands), the
SSH exec log (`loggedCommand`), the CLI session transcript
(`TranscriptWriter.Record`), the unauthorized refusal, the dispatch failure log
lines and the TACACS+ authorization log lines. The transcript is the one that
writes to a FILE, so a credential typed at the prompt outlived the session until
`Record` was routed through it.

**Private-key PEM blocks are scrubbed fail-closed.** `redact.PrivateKeyPEM`
replaces any `-----BEGIN ... PRIVATE KEY-----` block (PKCS#8, RSA, EC, or
OpenSSH) with `<redacted>` while `CERTIFICATE` blocks stay readable. If an end
marker is missing or truncated, redaction continues to the end of the string.
`redact.Command`, `redact.JSON`, and `TranscriptWriter.Record` (on command
output) all call `redact.PrivateKeyPEM` so private-key bytes never reach logs,
command history, or transcript files.

<!-- source: internal/component/config/mask.go -- DisplayCommand -->
<!-- source: internal/component/plugin/server/command.go -- BeginAccounting, beginAccountingArgs, recordCommandAudit, recordCommandAuditOutcome -->
<!-- source: internal/component/ssh/passwordauth.go -- loggedCommand -->

**A daemon command that carries caller bytes names them `content`, and
`content` is a command-line credential keyword.** `update data key <key>
content <base64>` and `update data config name <name> content <base64>` carry a
file from the caller's side, and that file can be a credential: a host key, a
login record, a config holding secrets. No record can tell which, so the value
after `content` is redacted on every command line, by the keyword's name, as
the value after `password` is. The keyword is in the command-line set only
(`isCredentialKey`), never in the config-name set `redact.JSON` reads, so a
captured payload keyed `content` keeps its value.

**A refusal is a fifth place a typed value surfaces, and it asks the same
predicate.** A command argument the validator refuses is quoted back in the
problem and the fix, and the refusal reaches the terminal, the web notice and
any log of either, none of which `redact.Command` sees. So the validator asks
`redact.IsCredentialKeyword`, the exported form of `isCredentialKey`, with no
list of its own: a refused value of a leaf named by a credential keyword is
quoted as the placeholder (`command.newValueFault`), and the whole-command fix
writes the placeholder for a valid value typed after such a keyword
(`ArgumentFaults.wholeCommand`). The route the local `ze data` and
`ze config import` commands take to the daemon passes the line through
`redact.Command` before it enters the reason or the fix it prints when the
daemon gives no answer (`daemonroute.CommandFix`, `daemonroute.Send`,
`daemonroute.OverSSH`). A value bound to a credential leaf by position, with no
keyword before it, is quoted as the placeholder in its own fault, but is
written back as typed in the fix of a line whose other leaf is refused, as
`redact.Command` keeps it in a record.

`ValidateArgString` also refuses an exact `<redacted>` value for those same
credential keywords before a handler can run. The typed value fault asks for
the real value; ordinary arguments remain free to contain the literal marker.

<!-- source: internal/core/redact/redact.go -- IsCredentialKeyword -->
<!-- source: internal/component/command/argvalue.go -- newValueFault -->
<!-- source: internal/component/command/argvalidate.go -- ValidateArgString -->
<!-- source: internal/component/command/argfaults.go -- wholeCommand -->
<!-- source: internal/component/config/storage/daemonroute/daemonroute.go -- CommandFix, Send, OverSSH -->


**A URL's secret is its userinfo, and the WHOLE userinfo goes.** `redact.URL`
replaces `user:password@` with the placeholder, and it replaces `token@` too.
`(*url.URL).Redacted` and net/http's own `stripPassword` keep the username and
blank the password, so a bare-token userinfo survives both, and a bare token is
a credential. `redact.URLError` rebuilds the `*url.Error` that net/url and
net/http attach to a failure, because that wrapper carries the URL into any
error text a caller prints. Both fail CLOSED: a string that does not parse
cannot be searched for its userinfo, so the answer carries none of the input.

## Traps this code exists to avoid

**Masking must be line-preserving.** Only the value token changes, so validation
line numbers computed on the unmasked content still align with the masked view.
Do NOT mask the content accessors themselves: they feed validation and
persistence. Only the display twins mask. Getting this wrong either leaks the
hash or makes commits fail.

**The web CLI has TWO show paths.** The terminal endpoint serializes the tree at
a path, and the older CLI dispatcher reaches the editor manager's content
accessor. The second carried only the authentication wrapper and the same-origin
check, with no edit gate, so a read-only session could read the raw hash through
it. It now routes through the display accessor, which was added to the editor
contract interface and to every implementer and fake. Grep ALL whole-subtree
serialize call sites, not only the obvious one.

**The editor test harness cannot load a system authentication config.** Both the
file load and the equivalent set command return an unknown path, even though the
same binary's config validate accepts the file and the daemon loads it. Display
masking is proven end to end by a functional config-dump test plus the unit
tests instead.

## Residual exposure, recorded and not closed here

- gNMI `Get` answers a `ze:sensitive` leaf in its reversible `$9$` form
  (`docs/guide/gnmi.md`, "Sensitive values") and every other leaf as stored, a
  bcrypt hash included, never masked. It is a separate component with its own
  authorization.
- An operator-installed local TCP proxy in front of the loopback SSH port is
  outside Ze's control. A dedicated unix-socket listener would close it.
- Redaction is scoped to the names `isSecretConfigKey` answers for: the words
  `password`, `secret`, `passphrase`, `psk`, `md5` and `token`, the suffixes
  `-password`, `-secret`, `-passphrase` and `-key`, and on a command line the
  `plaintext-` prefix and the word `content` as well. The bare word `key` is
  deliberately absent, because it names a key-chain entry id and a YANG list key
  far more often than it names a secret, and every real secret leaf spells the
  suffix.
- `content` is judged by its name alone, so it also matches two config path
  elements that hold no secret: the BGP update `nlri` entry's `content` leaf and
  a process binding's `content` container. In the record of a line that names
  either and reaches `redact.Command` (`... nlri <family> content <value>`,
  `... process <name> content <leaf> <value>`), the one token after `content`
  reads as the placeholder: the first word of the nlri value, or the name of the
  child leaf. Execution keeps the whole line; only the record loses that token.
- A store key named like a credential keyword (`meta/ssh/host-key` ends in
  `-key`) is itself a keyword, so in the record of `update data key
  meta/ssh/host-key content <base64>` the word `content` reads as the
  placeholder too. The content is still redacted, because the next keyword is
  judged from the original token: the record loses the word, never the
  protection.
- `cmd/ze/hub/main_system.go` logs the configured update-check URL at daemon
  start without `redact.URL`, so a mirror credential still reaches that one
  line.
