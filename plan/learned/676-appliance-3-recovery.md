# 676: appliance-3-recovery

Bastion disaster recovery for gokrazy-based Ze appliance images.

## Context

The bastion (operator workstation) is a single point of failure for fleet management. If the bastion disk fails or is destroyed, all appliance configs and encrypted secrets are lost. Running devices continue to work, but the operator cannot rebuild, push updates, or rotate credentials. Export/import provides encrypted backup and bastion migration.

## Decisions

- Reuse `Encrypt()`/`Decrypt()` from crypto.go unchanged over building a streaming encryption layer, because appliance dirs without images are a few hundred KB at most.
- Tar in memory then encrypt atomically, over streaming tar-then-encrypt, because the Encrypt() envelope format requires the full plaintext upfront (salt + nonce + AEAD seal).
- Export writes to the current working directory over the appliance base dir, to avoid cluttering appliance directories with archive files.
- Archives are always encrypted (no `--no-encrypt` flag) over allowing plaintext export, because archives contain secrets (password hash, TLS key, update token).
- Extracted `sharedDirName` constant from three repeated `"_shared"` string literals across cmd_build.go, cmd_list.go, and cmd_export.go.

## Consequences

- Bastion recovery is now a single command: `ze appliance export --all` + offsite storage.
- Import with `--dir` enables bastion migration to a new machine.
- The `listAppliances()` helper can be reused by appliance-2-remote for `--all` operations (push, config-push).
- Path traversal protection in tar extraction sets the pattern for any future archive handling.

## Gotchas

- `cmdExport`/`cmdImport` vars must be declared in main.go (alongside the other stub vars) before the command files can set them via `init()`. Adding the vars without also adding handler map entries triggers an unused-var lint error.
- The `"_shared"` string hit the goconst threshold (3 occurrences) when the third user (listAppliances) was added.

## Files

- `cmd/ze/appliance/cmd_export.go` (new): export single + export --all
- `cmd/ze/appliance/cmd_export_test.go` (new): 4 tests (single, all, passphrase required, roundtrip)
- `cmd/ze/appliance/cmd_import.go` (new): import with validation, overwrite protection, path traversal check
- `cmd/ze/appliance/cmd_import_test.go` (new): 4 tests (restore, wrong passphrase, overwrite, --dir)
- `cmd/ze/appliance/main.go` (modified): added export/import to handlers map, stub vars, usage entries
- `cmd/ze/appliance/register.go` (modified): added export/import to Subs
- `cmd/ze/appliance/resolve.go` (modified): added sharedDirName constant
- `cmd/ze/appliance/cmd_build.go` (modified): use sharedDirName constant
- `cmd/ze/appliance/cmd_list.go` (modified): use sharedDirName constant
- `docs/guide/appliance.md` (modified): added disaster recovery section and command reference entries
