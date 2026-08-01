# 747: Per-Service SSH Credential Storage and ze remote CLI

**Spec:** `spec-arch-3-remote-creds`
**Date:** 2026-05-20
**Status:** complete

## What Changed

Restructured zefs SSH credential storage from flat `meta/ssh/{username,password,host,port}`
to per-service `meta/ssh/<host>/<port>/{username,password}` with a `meta/ssh/default`
pointer for the active target. Added `ze remote` CLI (add, list, remove, default) and
`--remote host:port` flag on `ze cli`.

## Key Decisions

1. **Composite key hierarchy (host+port):** same host may run multiple daemons on
   different ports. Using `meta/ssh/<host>/<port>/` as the key prefix avoids collisions.

2. **Default pointer (`meta/ssh/default`):** stores `<host>/<port>` string. All CLI tools
   without `--remote` follow this pointer. `ze init` sets it automatically.

3. **Env overrides as a pair:** setting either `ze.ssh.host` or `ze.ssh.port` bypasses
   the default pointer entirely (the unset var gets the built-in default, not the
   pointer's value). This prevents confusing cross-contamination between env and pointer.

4. **Input validation rejects `/` in host/port:** prevents key path corruption since
   `/` is the zefs key separator.

5. **Removing a remote clears stale default pointer:** if the removed remote was the
   default, the pointer is deleted to avoid dangling references.

6. **loader.go no longer reads meta/ssh directly:** SSH auth credentials flow entirely
   through `LoadCredentials` / `LoadCredentialsForRemote`. The config loader's old
   direct `meta/ssh/username` and `meta/ssh/password` reads were removed.

## Files

| File | Change |
|------|--------|
| `internal/plugins/connect/main.go` | New: add, list, remove, default subcommands |
| `internal/plugins/connect/main_test.go` | New: unit tests |
| `cmd/ze/internal/ssh/client/client.go` | Rewrite: per-service credential resolution |
| `internal/core/ssh/client/client_test.go` | Updated: per-service key paths |
| `internal/plugins/init/main.go` | Updated: writes per-service keys + default pointer |
| `internal/plugins/init/main_test.go` | Updated: per-service assertions |
| `internal/component/cli/client/main.go` | Updated: --remote flag |
| `cmd/ze/main.go` | Updated: wired `ze remote` subcommand |
| `pkg/zefs/keys.go` | Updated: parameterized key patterns |

## Gotchas

- The `KeyEntry.Key(host, port)` method replaces `{host}` and `{port}` placeholders
  in the pattern. The `.Pattern` field is still used directly for non-parameterized
  keys like `meta/ssh/default`.
- `ListRemotes` must skip `meta/ssh/default` and `meta/ssh/authorized-keys` when
  enumerating host/port entries.
