# Bastion disaster recovery

The bastion is the operator workstation, and it is a single point of failure for
fleet management. If its disk dies, every appliance config and every encrypted
secret dies with it. Running devices keep working, and the operator can no
longer rebuild, push, or rotate credentials. Export and import give an encrypted
backup and a migration path.

<!-- source: internal/appliance/cmd_export.go -- export one appliance or --all -->
<!-- source: internal/appliance/cmd_import.go -- import with validation, overwrite protection, and path-traversal checks -->

## Decisions

- The archive is tarred in memory, then encrypted as one unit. The encryption
  envelope needs the full plaintext up front, because it writes salt, nonce, and
  an AEAD seal. Streaming was rejected for that reason, and an appliance
  directory without images is a few hundred kilobytes at most.
- `Encrypt` and `Decrypt` are reused unchanged from the builder's crypto. No
  second encryption layer exists.
- **Archives are always encrypted. There is no plaintext export flag.** An
  archive holds the password hash, the TLS key, and the update token.
- Export writes to the current working directory, not the appliance base
  directory, so archives do not accumulate inside appliance directories.
- Import accepts `--dir`, which is what makes bastion migration to a new machine
  one command.

## Constraint the code does not state

Tar extraction checks for path traversal before writing any member. That check
is the pattern for any future archive handling in this package.

## Related

- `builder.md` for the crypto envelope this reuses
