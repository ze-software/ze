# Spec: appliance-3-recovery

| Field | Value |
|-------|-------|
| Status | done |
| Depends | appliance-1-builder |
| Phase | 3/3 |
| Updated | 2026-05-10 |
| Split | Split from appliance-1-builder. |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/675-appliance-1-builder.md` - learned summary (builder spec deleted after completion)
4. `cmd/ze/appliance/crypto.go` - Encrypt/Decrypt/ResolvePassphrase (reused for archive encryption)
5. `cmd/ze/appliance/resolve.go` - ResolveDir/AppliancePath (reused for directory resolution)
6. `cmd/ze/appliance/main.go` - dispatch map and usage (add export/import entries)

## Task

Bastion disaster recovery for ze appliance: export appliance directories to encrypted archives, import from archives to restore on a fresh bastion.

Split from `spec-appliance-1-builder` (design session 2026-05-09).

## Required Reading

### Architecture Docs
- [ ] `plan/learned/675-appliance-1-builder.md` - Learned summary from builder spec
  -> Decision: archive format `[16B salt][24B nonce][encrypted tar][16B Poly1305 tag]` (XChaCha20-Poly1305 uses 24B nonce, not 12B)
  -> Constraint: archives always encrypted, even if appliance secrets are not; archive passphrase can differ from secrets passphrase

### Source Files
- [ ] `cmd/ze/appliance/crypto.go` (195L) - Encrypt/Decrypt with AEAD envelope, DeriveKey (Argon2id), ResolvePassphrase (agent > env > prompt), ZeroBytes
  -> Decision: Encrypt() returns `[16B salt][24B nonce][ciphertext+tag]`; uses XChaCha20-Poly1305 (NewX, 24B nonce)
  -> Constraint: reuse Encrypt/Decrypt directly for archive encryption; do not duplicate KDF parameters
- [ ] `cmd/ze/appliance/main.go` (144L) - Dispatch map `handlers`, extractDirFlag, getBaseDir(), stub pattern for unimplemented commands
  -> Decision: add "export" and "import" to handlers map; add to usage() help entries
- [ ] `cmd/ze/appliance/resolve.go` (60L) - ResolveDir (--dir > env > XDG), AppliancePath, ConfigPath, SecretsDir, TLSDir, DatabasePath
  -> Constraint: export must use ResolveDir + AppliancePath for directory resolution; import --dir uses ResolveDir for target
- [ ] `cmd/ze/appliance/config.go` (198L) - ApplianceConfig struct, LoadConfig, SaveConfig, Validate
  -> Constraint: import must validate appliance.json via LoadConfig after extraction
- [ ] `cmd/ze/appliance/cmd_build.go` (142L) - buildAll() pattern: iterate dir entries, skip _shared and dotfiles, check LoadConfig
  -> Decision: export --all should use the same directory iteration pattern as buildAll()

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/appliance/crypto.go` (195L) - Encrypt(plaintext, passphrase) returns `[16B salt][24B nonce][ciphertext+Poly1305 tag]`. Decrypt(envelope, passphrase) reverses. DeriveKey uses Argon2id (time=3, mem=64MiB, threads=4, keyLen=32). ResolvePassphrase: agent > env > prompt callback.
- [ ] `cmd/ze/appliance/resolve.go` (60L) - ResolveDir: flagDir > env(ze.appliance.dir) > XDG_CONFIG_HOME/ze/appliances. Path helpers: AppliancePath, ConfigPath, SecretsDir, TLSDir, DatabasePath.
- [ ] `cmd/ze/appliance/main.go` (144L) - handlers map dispatches subcommands. extractDirFlag parses --dir before dispatch. getBaseDir() returns resolved dir. usage() lists all commands.
- [ ] `cmd/ze/appliance/cmd_build.go` (142L) - buildAll() iterates baseDir entries, skips _shared and dotfiles, checks LoadConfig to identify valid appliances. Per-appliance pattern: resolve dir, load config, resolve passphrase if encrypted, operate.
- [ ] `cmd/ze/appliance/config.go` (198L) - ApplianceConfig with LoadConfig(path) and Validate(). configFileName = "appliance.json".

**Appliance directory layout** (verified from init, assemble, build):

| Path | Description | Export? |
|------|-------------|--------|
| appliance.json | Config (always present) | Yes |
| ze.conf | Per-appliance config overlay (optional) | Yes |
| build.json | Last build manifest (optional, created by build) | Yes |
| database.zefs | ZeFS database (transient, created by assemble) | No (derived) |
| ze-*.img | Disk images (created by build) | No (large, rebuildable) |
| ze-*.img.sha256 | Image checksums (created by build) | No (tied to images) |
| secrets/.encrypted | Marker if encryption enabled | Yes |
| secrets/password.hash | Bcrypt hash (may be encrypted) | Yes |
| secrets/update.token | Base64 token (may be encrypted) | Yes |
| secrets/authorized_keys | SSH public keys (plaintext) | Yes |
| secrets/tls/cert.pem | TLS certificate (plaintext) | Yes |
| secrets/tls/key.pem | TLS private key (may be encrypted) | Yes |

**Behavior to preserve:**
- Encryption envelope format: `[16B salt][24B nonce][encrypted payload][16B Poly1305 tag]` (XChaCha20-Poly1305)
- Passphrase resolution order: agent > env var > interactive prompt
- Appliance dir resolution: --dir > env > XDG default
- buildAll() iteration pattern: skip _shared, skip dotfiles, verify LoadConfig

**Behavior to change:**
- No export/import capability exists; this spec adds it
- Add "export" and "import" to handlers map and usage()

## Data Flow (MANDATORY)

### Entry Point
- `ze appliance export <name>` dispatched via handlers map in main.go
- `ze appliance export --all` iterates directory (same pattern as buildAll)
- `ze appliance import <archive>` dispatched via handlers map in main.go

### Transformation Path
1. (export single) getBaseDir() + AppliancePath(dir, name) to resolve appliance directory
2. (export single) Verify appliance exists via LoadConfig(ConfigPath(dir, name))
3. (export) ResolvePassphrase(terminalPrompt) to obtain archive encryption passphrase
4. (export) Walk appliance dir, add to tar: appliance.json, secrets/ (recursive), ze.conf (if exists), build.json (if exists)
5. (export) Skip: *.img, *.img.sha256, database.zefs
6. (export) Encrypt(tarBytes, passphrase) produces AEAD envelope
7. (export single) Write `<name>.ze.enc` to current directory
8. (export --all) Iterate baseDir entries (skip _shared, dotfiles, non-appliance dirs), tar all into single archive, write `appliances-<timestamp>.ze.enc`
9. (import) ResolvePassphrase(terminalPrompt) for archive decryption
10. (import) Decrypt(envelope, passphrase) verifies AEAD tag; wrong passphrase returns "decryption failed"
11. (import) Validate archive structure: each top-level directory must contain appliance.json
12. (import) For each appliance in archive: check if AppliancePath(targetDir, name) exists; if so, prompt for overwrite (or --force to skip)
13. (import) Extract tar to target dir (ResolveDir(flagDir))
14. (import) Verify extracted appliance via LoadConfig

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Appliance dir -> tar bytes | archive/tar + bytes.Buffer in memory | [ ] |
| Tar bytes -> encrypted archive | Encrypt() from crypto.go (same AEAD envelope as secrets) | [ ] |
| Encrypted archive -> tar bytes | Decrypt() from crypto.go; AEAD tag verifies integrity | [ ] |
| Tar bytes -> appliance dir | archive/tar extract with path traversal protection | [ ] |
| Archive passphrase -> derived key | DeriveKey() via ResolvePassphrase() from crypto.go | [ ] |

### Integration Points
- `cmd/ze/appliance/crypto.go:Encrypt` - encrypt tar payload; same envelope as secrets
- `cmd/ze/appliance/crypto.go:Decrypt` - decrypt archive; wrong passphrase = clear "decryption failed" error
- `cmd/ze/appliance/crypto.go:ResolvePassphrase` - agent > env > prompt for passphrase
- `cmd/ze/appliance/crypto.go:ZeroBytes` - zero passphrase after use
- `cmd/ze/appliance/resolve.go:ResolveDir` - resolve target directory for import
- `cmd/ze/appliance/resolve.go:AppliancePath` - check if target appliance exists before overwriting
- `cmd/ze/appliance/config.go:LoadConfig` - validate appliance.json in archive and after extraction

### Architectural Verification
- [ ] No bypassed layers (reuses Encrypt/Decrypt from crypto.go, not raw AEAD calls)
- [ ] No unintended coupling (export/import are standalone, no dependency on assemble or build)
- [ ] No duplicated functionality (directory iteration reuses buildAll pattern; encryption reuses existing helpers)
- [ ] Path traversal protection (tar extraction validates no `..` components)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze appliance export lab` CLI | -> | `cmd/ze/appliance/cmd_export.go:cmdExport()` | `TestExportCreatesArchive` |
| `ze appliance import archive.ze` CLI | -> | `cmd/ze/appliance/cmd_import.go:cmdImport()` | `TestImportRestoresAppliance` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-64 | `ze appliance export lab` | Creates `lab.ze.enc` archive containing appliance.json + secrets/ + ze.conf; encrypted with passphrase |
| AC-65 | `ze appliance export --all` | Creates `appliances-<timestamp>.ze.enc` archive containing all appliance directories |
| AC-66 | `ze appliance import lab.ze.enc` | Restores appliance directory from archive; prompts before overwriting existing |
| AC-67 | `ze appliance import lab.ze.enc` with wrong passphrase | Exit code 1, stderr "error: decryption failed" |
| AC-68 | `ze appliance import lab.ze.enc --dir /new/bastion` | Restores to specified directory (bastion migration) |
| AC-69 | `ze appliance export lab` without passphrase set | Exit code 1, stderr "error: export requires encryption passphrase (archives always encrypted)" |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExportCreatesArchive` | `cmd/ze/appliance/cmd_export_test.go` | Export produces encrypted .ze.enc file containing appliance dir | |
| `TestExportAllCreatesArchive` | `cmd/ze/appliance/cmd_export_test.go` | --all exports all appliances into single archive | |
| `TestExportRequiresPassphrase` | `cmd/ze/appliance/cmd_export_test.go` | Export without passphrase fails (archives always encrypted) | |
| `TestImportRestoresAppliance` | `cmd/ze/appliance/cmd_import_test.go` | Import decrypts and restores appliance directory | |
| `TestImportWrongPassphraseFails` | `cmd/ze/appliance/cmd_import_test.go` | Wrong passphrase -> AEAD auth error, exit 1 | |
| `TestImportPromptsBeforeOverwrite` | `cmd/ze/appliance/cmd_import_test.go` | Existing appliance dir -> prompt for confirmation | |
| `TestImportToNewDir` | `cmd/ze/appliance/cmd_import_test.go` | --dir flag restores to specified directory | |
| `TestExportImportRoundtrip` | `cmd/ze/appliance/cmd_export_test.go` | Export then import produces identical directory tree | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-appliance-export-import` | `test/appliance/export-import.ci` | Export appliance, import to new dir, assemble from imported copy | |

## Files to Modify

- `cmd/ze/appliance/main.go` - Add "export" and "import" to handlers map (line 21); add entries to usage() Sections (line 118)
- `cmd/ze/appliance/register.go` - Add "export, import" to Subs string (line 11)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (offline command) |
| CLI commands/flags | Yes | `cmd/ze/appliance/main.go` |
| Editor autocomplete | No | N/A (offline command) |
| Functional test for new RPC/API | No | N/A (offline command) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/appliance.md` - export/import commands |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/appliance.md` - CLI reference |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |

## Files to Create

- `cmd/ze/appliance/cmd_export.go` - Export appliance dir to encrypted archive (tar + Encrypt)
- `cmd/ze/appliance/cmd_export_test.go` - Export tests (single, --all, requires passphrase, roundtrip)
- `cmd/ze/appliance/cmd_import.go` - Import appliance dir from encrypted archive (Decrypt + untar)
- `cmd/ze/appliance/cmd_import_test.go` - Import tests (restore, wrong passphrase, overwrite prompt, --dir)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Export** -- `ze appliance export <name>` and `ze appliance export --all` create encrypted archives
   - Tests: `TestExportCreatesArchive`, `TestExportAllCreatesArchive`, `TestExportRequiresPassphrase`
   - Files: `cmd/ze/appliance/cmd_export.go`, `cmd/ze/appliance/cmd_export_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Import** -- `ze appliance import <archive>` restores from encrypted archive with overwrite protection
   - Tests: `TestImportRestoresAppliance`, `TestImportWrongPassphraseFails`, `TestImportPromptsBeforeOverwrite`, `TestImportToNewDir`
   - Files: `cmd/ze/appliance/cmd_import.go`, `cmd/ze/appliance/cmd_import_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Roundtrip + wiring** -- Export/import roundtrip test, dispatch wiring in main.go, register.go
   - Tests: `TestExportImportRoundtrip`
   - Files: update `cmd/ze/appliance/main.go`, `cmd/ze/appliance/register.go`
   - Verify: tests fail -> implement -> tests pass

4. **Functional tests** -> Create after feature works.
5. **Full verification** -> `make ze-verify` (lint + all ze tests)
6. **Complete spec** -> Fill audit tables, write learned summary.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Export single | `export <name>` creates encrypted .ze.enc archive |
| Export all | `export --all` creates single archive with all appliances |
| Export requires passphrase | Archives are always encrypted; no unencrypted export |
| Import restore | `import <archive>` decrypts and restores appliance dir |
| Import to new dir | `--dir` flag allows import to different bastion |
| Export/import roundtrip | Export then import produces byte-identical appliance dir |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Export | `ze appliance export <name>` produces .ze.enc archive |
| Import | `ze appliance import <archive>` restores appliance dir from archive |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| **Export archive encryption** | Archives always encrypted (no `--no-encrypt`); uses same AEAD as secrets; archive passphrase can differ from secrets passphrase |
| **Export archive integrity** | Archive includes HMAC of contents; import verifies before extracting |
| **Import overwrite protection** | Import prompts before overwriting existing appliance dir; `--force` skips prompt |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Archives always encrypted (no `--no-encrypt` flag) | Archives contain secrets (password hash, TLS key, update token); unencrypted backup defeats the purpose of encrypting secrets at rest |
| Archive passphrase can differ from secrets passphrase | Operator may use a stronger/different passphrase for offsite backup vs daily operations |
| Archive format: `[16B salt][24B nonce][encrypted tar][16B Poly1305 tag]` | Reuses Encrypt() from crypto.go directly; XChaCha20-Poly1305 uses 24B nonce (not 12B as originally written in skeleton) |
| Exclude images (*.img, *.img.sha256) and database.zefs | Images are large (2 GiB default) and rebuildable; database.zefs is a derived artifact from assemble |
| Include: appliance.json, secrets/, ze.conf, build.json | These are the non-rebuildable state: config, encrypted credentials, TLS keys, build metadata |
| Import prompts before overwriting; `--force` skips prompt | Prevents accidental loss of existing appliance config |
| Tar in memory, then encrypt | Appliance dirs are small (a few KB without images); streaming encrypt would require a different envelope format |
| Path traversal protection on import | Tar entries must not contain `..` components; prevents archive-based directory escape |
| Export --all uses single archive | Simpler for offsite storage and transfer; individual export available for selective backup |
| Export writes to current working directory | Not to the appliance base dir; avoids cluttering appliance directories with archives |

### Crypto Integration

Export/import reuse `Encrypt()` and `Decrypt()` from crypto.go unchanged. The archive is treated as one large plaintext: tar the included files into a `bytes.Buffer`, pass the buffer contents to `Encrypt()`, write the envelope to disk.

This means:
- Same Argon2id parameters (time=3, mem=64MiB, threads=4, keyLen=32)
- Same AEAD (XChaCha20-Poly1305, 24B nonce, 16B tag)
- Wrong passphrase on import returns "decryption failed" (AEAD verification fails)
- No partial extraction possible (entire archive decrypted atomically)

For large fleets (many appliances, each with several KB of secrets), the entire tar will be at most a few hundred KB. Memory is not a concern.

### Directory Iteration (--all)

Reuse the same pattern as `buildAll()` in cmd_build.go (line 102):
1. ReadDir(baseDir)
2. Skip entries that are not directories
3. Skip `_shared` and dotfile directories
4. Verify each is a valid appliance via LoadConfig(ConfigPath(dir, name))
5. Collect valid appliance names

### Passphrase Handling

- Export: ResolvePassphrase with terminal prompt callback (agent > env > prompt)
- Import: ResolvePassphrase with terminal prompt callback
- `--all` export: single passphrase for the entire archive
- ZeroBytes(passphrase) after use via defer

Note: for export, the passphrase is for the archive encryption, not for reading the appliance's secrets. Export does not need to decrypt any secrets; it copies the secrets/ directory as-is (already encrypted or plaintext, depending on the appliance's `.encrypted` marker).

## Resolved Questions

| # | Question | Answer |
|---|----------|--------|
| Q1 (inherited) | Should export decrypt and re-encrypt secrets? | No. Export copies secrets/ as-is. The archive encryption wraps the entire directory. |
| Q2 | What if crypto.go's Encrypt() can't handle tar sizes? | Not a concern: without images, archive is a few hundred KB. Encrypt() operates on a byte slice in memory. |
| Q3 | Should import validate extracted secrets? | No. Import validates appliance.json (via LoadConfig). Secrets are opaque encrypted blobs; their validity is checked at assemble/build time. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | extractTar silently drops tar entries with Typeflag other than TypeDir and TypeReg (symlinks, hardlinks). Secure default for import. | cmd_import.go:185 | No action (intentional) |
| 2 | NOTE | tarApplianceInto uses filepath.Walk which follows symlinks. Operator controls appliance dir; not an external attack vector. | cmd_export.go:199 | No action (standard behavior) |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-64..AC-69 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed
