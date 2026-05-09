# Spec: appliance-3-recovery

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | appliance-1-builder |
| Phase | - |
| Updated | 2026-05-09 |
| Split | Split from appliance-1-builder. |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-appliance-1-builder.md` Design section (Export / Import subsection)
3. `cmd/ze/appliance/crypto.go` - encryption primitives (from appliance-1-builder)

## Task

Bastion disaster recovery for ze appliance: export appliance directories to encrypted archives, import from archives to restore on a fresh bastion.

Split from `spec-appliance-1-builder` (design session 2026-05-09).

## Required Reading

### Architecture Docs
- [ ] `plan/spec-appliance-1-builder.md` - Export / Import design, encryption envelope, archive format

### Source Files
- [ ] `cmd/ze/appliance/crypto.go` - Argon2id + ChaCha20-Poly1305 primitives (from appliance-1-builder)
- [ ] `cmd/ze/appliance/main.go` - Dispatch structure (from appliance-1-builder)
- [ ] `cmd/ze/appliance/resolve.go` - Appliance dir resolution (from appliance-1-builder)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/appliance/crypto.go` - encryption/decryption helpers
- [ ] `cmd/ze/appliance/resolve.go` - dir resolution

**Behavior to preserve:**
- Encryption envelope format: `[16B salt][12B nonce][encrypted payload][16B Poly1305 tag]`
- Passphrase resolution: agent > interactive > env var
- Appliance dir resolution: --dir > env > default

**Behavior to change:**
- No export/import capability exists; this spec adds it

## Data Flow (MANDATORY)

### Entry Point
- `ze appliance export <name>` or `ze appliance export --all`
- `ze appliance import <archive>`

### Transformation Path
1. (export) Resolve appliance directory, obtain passphrase
2. (export) Tar appliance dir (appliance.json + secrets/ + ze.conf + build.json; exclude images + database.zefs)
3. (export) Encrypt tar with Argon2id + ChaCha20-Poly1305
4. (export) Write `<name>.ze.enc` or `appliances-<timestamp>.ze.enc`
5. (import) Obtain passphrase, decrypt archive, verify AEAD tag
6. (import) Validate archive structure (must contain appliance.json per directory)
7. (import) Prompt before overwriting existing; extract to target dir

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Appliance dir -> tar archive | archive/tar package | [ ] |
| Tar -> encrypted archive | Same AEAD envelope as secrets encryption | [ ] |
| Encrypted archive -> tar | AEAD decrypt; wrong passphrase = clear error | [ ] |
| Tar -> appliance dir | tar extract with structure validation | [ ] |

### Integration Points
- `cmd/ze/appliance/crypto.go` - encrypt/decrypt primitives
- `cmd/ze/appliance/resolve.go` - appliance dir resolution
- `cmd/ze/appliance/agent.go` - passphrase agent for key retrieval

### Architectural Verification
- [ ] No bypassed layers (reuses crypto primitives from appliance-1-builder)
- [ ] No unintended coupling (export/import are standalone operations)
- [ ] No duplicated functionality (encryption reuses existing helpers)

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

- `cmd/ze/appliance/main.go` - Add export, import dispatch cases
- `cmd/ze/appliance/register.go` - Register new subcommands

## Files to Create

- `cmd/ze/appliance/cmd_export.go` - Export appliance dir to encrypted archive
- `cmd/ze/appliance/cmd_export_test.go` - Export tests
- `cmd/ze/appliance/cmd_import.go` - Import appliance dir from encrypted archive
- `cmd/ze/appliance/cmd_import_test.go` - Import tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phase below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test` |

### Implementation Phases

1. **Phase: Export/import** -- `ze appliance export` creates encrypted archive of appliance dir; `ze appliance import` restores from archive; bastion disaster recovery
   - Tests: `TestExportCreatesArchive`, `TestExportAllCreatesArchive`, `TestExportRequiresPassphrase`, `TestImportRestoresAppliance`, `TestImportWrongPassphraseFails`, `TestImportPromptsBeforeOverwrite`, `TestImportToNewDir`, `TestExportImportRoundtrip`
   - Files: `cmd/ze/appliance/cmd_export.go`, `cmd/ze/appliance/cmd_import.go` + tests
   - Verify: tests fail -> implement -> tests pass

2. **Functional tests** -> Create after feature works.
3. **Full verification** -> `make ze-verify` (lint + all ze tests)
4. **Complete spec** -> Fill audit tables, write learned summary.

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

- Archives are always encrypted (no `--no-encrypt` flag)
- Archive passphrase can differ from the secrets passphrase
- Archive format: `[16B salt][12B nonce][encrypted tar][16B Poly1305 tag]`
- Excludes images (large, rebuildable) and database.zefs (derived artifact)
- Includes: appliance.json, secrets/, ze.conf, build.json
- Import prompts before overwriting; `--force` skips prompt

Full design details are in `spec-appliance-1-builder.md` Design section (Export / Import subsection).

## Resolved Questions

None (inherited from appliance-1-builder Q23).

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

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
