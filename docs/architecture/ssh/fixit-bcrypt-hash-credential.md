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
<!-- source: internal/component/config/mask.go -- LeafHoldsSecret, MaskBcrypt, MaskSecrets, MaskSecretsInPlace, SecretKeys, RejectMaskedSecretLeaves -->
<!-- source: internal/component/cli/editor_mask.go -- DisplayContentAtPath, DisplayOriginalContentAtPath -->
<!-- source: internal/core/redact/redact.go -- IsBcryptHash, Command, JSON, Placeholder -->

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

**Map-shaped data is masked by leaf NAME.** `SecretKeys` answers that set from
the same predicate. The BGP resolver flattens group and peer inheritance, so a
path in the resolved map addresses no schema node. The config dump and the
config diff both read a name there.

**The placeholder is never the reversible sensitive-value marker.** A sensitive
leaf uses a reversible encoding; bcrypt is one way and the parser refuses the
reversible marker on a bcrypt leaf. In the config dump the sensitive encoder
skips a value that already equals the placeholder, because a leaf name can be
both bcrypt in one module and sensitive in another.

**The commit guard fails closed.** `RejectMaskedSecretLeaves` rejects a secret
leaf that holds the placeholder rather than silently resolving it. It reads
`LeafHoldsSecret`, so it covers a `ze:sensitive` leaf as well as a `ze:bcrypt`
one, and it answers the one predicate the display mask reads. It is wired
at every commit and validate entry point, which is also what backs the web
upload. The web value setter no-ops a resubmitted placeholder as a
user-interface backstop.

**The redaction regex has one home.** The core redact package owns the
bcrypt-shape pattern and the config helper delegates to it. Command redaction
scrubs bcrypt-shaped tokens and password-family key values BEFORE the log
truncation, so a secret straddling the cut cannot half-leak.

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

- gNMI `Get` still returns raw config leaf values unmasked. It is a separate
  component with its own authorization.
- An operator-installed local TCP proxy in front of the loopback SSH port is
  outside Ze's control. A dedicated unix-socket listener would close it.
- Redaction is scoped to the password family and to bcrypt tokens. It does not
  cover `secret`, `community` or key-suffixed names, because that would redact
  BGP communities and host-key file paths out of the operational log.
