# 648 -- SSH Public Key Authentication

## Context

Ze only supported password-based SSH authentication (bcrypt). YANG-configured users and the zefs super-admin both used passwords exclusively. Operators wanted key-based SSH login, similar to VyOS's `system login user <name> authentication public-keys` model, so they could use standard SSH key pairs instead of managing passwords for every session.

## Decisions

- Chose VyOS-style config model (separate `type` + `key` leaves per named key entry) over a single authorized_keys-format string, because it maps cleanly to YANG and provides enumeration-based type validation.
- Public key auth is a parallel wish handler (`wish.WithPublicKeyAuth`) alongside existing password auth, not a new AAA backend, because SSH key verification happens at the transport layer before the AAA chain runs. The AAA `Authenticator` interface is password-oriented (`AuthRequest` carries username+password).
- No `options` leaf (unlike VyOS) because wish handles auth callbacks directly; there are no `authorized_keys` files to write.
- Scoped to YANG-configured users only. Zefs super-admin stays password-only to avoid blob store schema changes.
- `SSHPublicKey` struct lives in `aaa/types.go` (alongside `UserCredential`) with a type alias in `authz`, following the existing re-export pattern.

## Consequences

- Users can now have both password and public keys; either works independently for SSH. Web UI remains password-only.
- Adding FIDO/U2F key types (sk-ecdsa, sk-ssh-ed25519) later requires only adding enum values to the YANG schema; no Go changes needed since `ssh.ParseAuthorizedKey` already handles them.
- `ze cli` still only supports password auth. Users must use a standard `ssh` client for key-based login.

## Gotchas

- `ExtractSSHConfig` returns early when `environment.ssh` is absent from config. Tests must include an `environment { ssh { ... } }` block or the user list comes back empty.
- The `charmbracelet/ssh.PublicKey` interface wraps `golang.org/x/crypto/ssh.PublicKey`. `ssh.KeysEqual` does constant-time comparison internally via marshaled bytes.
- Pre-existing build breakages in unrelated packages (iface, config/system) can block the test suite through transitive `plugin/all` imports. The SSH package tests run independently.

## Files

- `internal/component/ssh/schema/ze-ssh-conf.yang` - YANG `list public-keys`
- `internal/component/aaa/types.go` - `SSHPublicKey` struct
- `internal/component/ssh/pubkey.go` - key matching logic
- `internal/component/ssh/ssh.go` - wish handler wiring
- `internal/component/bgp/config/loader.go` - config extraction
- `docs/guide/authentication.md` - user guide
