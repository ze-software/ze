# 1181 - fixit-bcrypt-hash-credential (restrict + mask)

## Context

The zefs-stored bcrypt password hash was accepted as a credential itself (constant-time
`sha256(hash)` compare in `authz.CheckPassword`), intended for the on-box `ze` CLI which
sends the zefs hash. But that branch was reached over EVERY transport: remote SSH, web
Basic/form auth, and the REST/gRPC bearer path. It was also exported unmasked (`show config`,
web views, `ze config dump`, `GET /config/download`). So any config read = a remote
read-only-to-admin escalation. Chosen fix (Thomas): restrict hash-as-token to local
transports + mask `ze:bcrypt` on display + gate the raw download behind edit-authz +
redact SSH exec credential logs.

## Decisions

- **Transport signal = explicit `AuthRequest.Local bool`, zero value = remote (fail-closed).**
  Only the SSH password callback sets it, from the accepted socket peer
  (`ssh.isLocalTransport`: unix-socket OR loopback TCP). Web/API never set it, so they always
  reject hash-as-token. `CheckPassword`/`AuthenticateUser` gained an explicit `allowHashToken`
  param (not optional) so a future caller must state the transport class. Rationale: the
  socket peer is the only non-spoofable signal, and it lives at the transport layer, not in authz.
- **Mask on a display CLONE, never in the shared serializers.** `config.MaskBcrypt(tree,schema)`
  clones + replaces every non-empty `ze:bcrypt` leaf value with `SecretDataPlaceholder`
  ("/* SECRET-DATA */"); `MaskBcryptInPlace` for callers that already hold a private clone.
  Masking is applied at each display choke point (CLI show/annotated/diff/search, the
  `DisplayContentAtPath`/`DisplayOriginalContentAtPath` twins of the unmasked accessors, the
  web CLI terminal serializers, the web per-leaf builders, `ze config dump`). The persistence
  and validation serializers keep running on the unmasked live tree.
- **Placeholder is NEVER $9$.** `ze:sensitive` uses reversible `$9$`; bcrypt is one-way and the
  parser refuses `$9$` on a bcrypt leaf. In `ze config dump` the sensitive `$9$` encoder skips a
  value already equal to the placeholder (a bcrypt leaf name can also be `ze:sensitive` in
  another module; `SensitiveKeys` collects by name).
- **Fail-closed commit/upload guard.** `config.RejectMaskedBcryptLeaves` rejects (never silently
  resolves) a bcrypt leaf holding the placeholder, wired at every commit/validate entry point
  (`editor_commit.go` x2, `editor_commands.go` commitContent, `validator.go`, `cmd_validate.go`
  runValidation which also backs the web upload `ValidateContent`). Web `SetValue` no-ops a
  resubmitted placeholder on a bcrypt leaf as a UX backstop.
- **Redaction regex has one home.** `internal/core/redact` owns the bcrypt-shape regex;
  `config.IsBcryptHash` delegates. `redact.Command` scrubs bcrypt-shaped tokens and
  password-family key values BEFORE `truncateForLog` so a secret straddling the 256-byte cut
  cannot half-leak.

## Consequences / Gotchas

- **Masking must be line-preserving.** Only the value token changes, so validation line numbers
  computed on the unmasked `ContentAtPath(nil)` still align with the masked config view. Do NOT
  mask `ContentAtPath`/`WorkingContent` themselves - they feed validation (`model.go`,
  `model_commands_commit.go`) and persistence (`commitContent`, `CommitSession`). Only the
  `Display*` twins mask. Getting this wrong either leaks the hash or makes commits fail.
- **The web CLI has TWO show paths.** `/cli/terminal` (`serializeTreeAtPath`) AND the older
  `/cli` dispatcher (`handleCLIShow` -> `EditorManager.ContentAtPath`). The second is only
  `authWrap+RequireSameOrigin` (no edit gate), so a read-only session could read the raw hash
  through it. Fixed by routing `EditorManager.ContentAtPath` through `DisplayContentAtPath`
  (added to the `contract.Editor` interface + all implementers/fakes). Lesson: grep ALL
  whole-subtree serialize call sites, not just the obvious one.
- **The `.et` editor harness cannot load `system authentication` configs.** Both file-load and
  `set system authentication user ...` return "unknown path" even though the same binary's
  `ze config validate` accepts it and the daemon loads it. AC-5 display masking is proven
  end-to-end instead by `test/parse/config-dump-masks-bcrypt.ci` + the unit tests. See DECISION.md.
- **R-3 blast radius was empty.** Audit of every `KeySSHPassword` writer/consumer found NO real
  flow breaks: `ze cli --remote` with a stored super-admin hash is the only theoretical break and
  it requires `ze connect add`, which is already non-functional (fresh salt, so the stored hash
  can never byte-match the remote daemon's). QEMU/install tooling uses plaintext SSH + the
  separate `ze.appliance.ssh.password` build var. No migration needed; just wire the SSH `Local`
  classification.
- **Residuals (documented, not fixed here):** gNMI `Get` still returns raw config leaf values
  unmasked (separate component, its own auth); an operator-installed local TCP proxy in front of
  127.0.0.1:2222 is outside ze's control (a dedicated unix-socket listener would close it,
  raised as a follow-up). Redaction is scoped to password-family + bcrypt tokens (not
  `secret`/`community`/`-key`) to avoid false-positive redaction of BGP communities / host-key
  file paths in the operational log.
