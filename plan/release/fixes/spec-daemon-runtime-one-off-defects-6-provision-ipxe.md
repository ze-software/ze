# Spec: Daemon runtime one-off defects (6) - Provision staging refuses iPXE placeholder files

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Stops `ze install remote` from silently staging 28-byte ASCII placeholder files (`tools/ipxe-binaries/ipxe.efi` and `ipxe.pxe`) into the TFTP boot directory where firmware fails to boot |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-daemon-runtime-one-off-defects.md`. This child owns the one open `internal/plugins/provision/` journal row routed from the umbrella.

## Task

Fix iPXE placeholder detection in `internal/plugins/provision/staging.go` (`placeholder-shipped-as-the-real-artifact`, 2026-08-28):

`tools/ipxe-binaries/ipxe.efi` and `tools/ipxe-binaries/ipxe.pxe` are 28-byte ASCII placeholder files (`iPXE UEFI binary placeholder` / BIOS equivalent). Today `stageArtifacts` and `validateStaging` in `internal/plugins/provision/staging.go` check only `os.Stat` and `info.Mode().IsRegular()`, so `ze install remote` copies those 28-byte text placeholders into `build/pxe/tftp/` and serves them over TFTP, causing PXE firmware to fail silently at chainload.

Validate each iPXE artifact in BOTH `stageArtifacts` (before copying from `cfg.IPXEDir`) and `validateStaging` (when checking staged files already in `tftpDir`, so a previously copied placeholder is also refused):
- `ipxe.efi` MUST start with the PE/COFF UEFI magic bytes (`MZ`, `0x4d 0x5a`) and exceed a minimum plausible binary size (at least 4096 bytes).
- `ipxe.pxe` MUST exceed a minimum plausible binary size (at least 4096 bytes) and MUST NOT be plain ASCII placeholder text (for example starting with `iPXE `).
- On failure, return an actionable `*opererr.Error` (`ai/rules/user-facing-errors.md`) naming the exact placeholder file path, stating why PXE boot would fail (it is a placeholder text file, not a bootable iPXE image), and giving the exact fix command from `tools/ipxe-binaries/README.md` to obtain real iPXE binaries.

Note: `ze install remote` is an explicit operator provisioning CLI command, not `ze start` daemon boot, so refusing to stage a placeholder binary directly satisfies fail-closed operator safety without violating the 2026-11-26 "Ze always starts" daemon-boot rule.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, operator error contract
- [ ] `ai/doors/system-plugins.md` - provisioning plugins and PXE boot contracts
- [ ] `docs/architecture/provisioning/pxe-staging.md` - PXE artifact staging and validation
- [ ] `tools/ipxe-binaries/README.md` - documented steps to fetch real iPXE binaries
- [ ] `ai/rules/user-facing-errors.md` - problem, why, and exact fix command

## Current Behavior (MANDATORY)

Verified at HEAD on 2026-11-26:

- [ ] `tools/ipxe-binaries/ipxe.efi` and `tools/ipxe-binaries/ipxe.pxe` - both are 28-byte ASCII placeholder files on disk.
- [ ] `internal/plugins/provision/staging.go` - `stageArtifacts` (L68-102) calls `copyFileIfRegular` without inspecting file contents or size; `validateStaging` (L104-127) calls `os.Stat` only, accepting 28-byte placeholders as valid staged bootloaders.

**Behavior to preserve:**
- Real UEFI PE (`MZ` header, >= 4096 bytes) `ipxe.efi` and real BIOS (`>= 4096` bytes, non-ASCII placeholder) `ipxe.pxe` files stage and validate unchanged.
- Existing valid files already present in `tftpDir` are not overwritten by `stageArtifacts` (unless they are invalid placeholders, which `validateStaging` refuses clearly).
- Kernel (`vmlinuz`) and initrd (`initrd.img.gz`) staging stay unchanged.

**Behavior to change:**
- `stageArtifacts` refuses to copy a placeholder `ipxe.efi` or `ipxe.pxe` into `tftpDir`, leaving no broken file staged.
- `validateStaging` refuses any placeholder `ipxe.efi` or `ipxe.pxe` already present in `tftpDir` with an `*opererr.Error` naming the file path, the reason, and the fetch command from `tools/ipxe-binaries/README.md`.

## Existing Mechanisms (MANDATORY)

Verified at HEAD on 2026-11-26.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| PXE staging and validation | `internal/plugins/provision/staging.go` `stageArtifacts`, `validateStaging`, `copyFileIfRegular` | Copies and stats `ipxe.pxe` and `ipxe.efi` with no content or size check | Extends: adds `validateIPXEBinary(name, path)` called from both `stageArtifacts` and `validateStaging` |
| Structured operator errors | `internal/core/opererr` `opererr.New` | Formats Problem / Why / Fix messages consistently across CLI commands | Extends: formats the placeholder refusal with exact path, reason, and fetch instructions |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "**Every message a user sees names the problem, why, and the exact fix**, on every interface" | agrees | `validateIPXEBinary` returns an `*opererr.Error` naming the placeholder path, why firmware cannot boot it, and the exact fetch command from `tools/ipxe-binaries/README.md`. |
| `ai/doors/system-plugins.md` | Who depends on it from outside | the door covering `internal/plugins/provision/staging.go` and `staging_test.go` | "DHCP, TFTP and HTTP clients during PXE" | agrees | Refusing 28-byte ASCII placeholders before TFTP staging prevents serving broken text files to PXE boot firmware. |
| `docs/architecture/provisioning/pxe-staging.md` | The two-stage path | `// Design:` page of `internal/plugins/provision/staging.go` and `staging_test.go`; modified by this spec | "The firmware ROM and iPXE are different clients and get different answers:" | agrees | Ensures the staged `ipxe.efi` and `ipxe.pxe` files are real bootable images rather than 28-byte ASCII placeholders, and updates this page to document the refusal. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze install remote` invoking `stageArtifacts` and `validateStaging` in `internal/plugins/provision/staging.go`.

### Transformation Path
1. `stageArtifacts` locates `src` in `cfg.IPXEDir` -> calls `validateIPXEBinary(name, src)` before copying -> refuses placeholders with `*opererr.Error` and writes nothing to `tftpDir`.
2. `validateStaging` stats each required boot and iPXE file -> for each `ipxeBinaries` entry in `tftpDir`, calls `validateIPXEBinary(name, path)` -> refuses any placeholder file already on disk with `*opererr.Error`.

### Boundaries Crossed
- Stays inside `internal/plugins/provision`. No new component imports or cross-plugin imports (`./le arch tier check` clean).

### Integration Points
- `internal/plugins/provision/staging.go` and `internal/core/opererr`.

### Changed-Wire Census

No wire format, RPC payload, event schema, or external protocol changes in this spec. Validation runs locally before TFTP files are staged or served.

### Architectural Verification
- [ ] Staging the repository's 28-byte `tools/ipxe-binaries/ipxe.efi` or `ipxe.pxe` fails closed with an `*opererr.Error` naming the file and fetch instructions
- [ ] Pre-existing placeholder files in `tftpDir` are also caught by `validateStaging`
- [ ] Valid PE/MZ `ipxe.efi` and binary `ipxe.pxe` (>= 4096 bytes) pass both `stageArtifacts` and `validateStaging`

## Risks & Assumptions

### Assumptions
- Real `ipxe.efi` binaries are UEFI PE/COFF executables starting with `MZ` (`0x4d, 0x5a`) and well above 4 KiB (typically > 100 KiB), and real `ipxe.pxe` UNDI images are binary payloads well above 4 KiB (typically > 60 KiB).

### Risks
- Existing unit tests in `internal/plugins/provision/` might create tiny dummy `ipxe.efi` / `ipxe.pxe` fixture files (for example `[]byte("dummy")`). Any such test fixture must be updated to use a realistic synthetic fixture (4096+ bytes with `MZ` header for `.efi`, non-ASCII binary for `.pxe`) with `ze-test-judge` approval if any existing test assertion changes.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `stageArtifacts` and `validateStaging` given `tools/ipxe-binaries/ipxe.efi` / `ipxe.pxe` | → | `validateIPXEBinary` (`internal/plugins/provision/staging.go`) | `TestStageArtifactsRefusesIPXEPlaceholder` and `TestValidateStagingRefusesIPXEPlaceholder` (`internal/plugins/provision/staging_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Calling `stageArtifacts` with `cfg.IPXEDir` pointing to the 28-byte ASCII placeholder `ipxe.efi` or `ipxe.pxe` (`placeholder-shipped-as-the-real-artifact`, 2026-08-28) | `stageArtifacts` returns an `*opererr.Error` naming the placeholder file path, explaining why PXE firmware cannot boot it, and giving the fetch command from `tools/ipxe-binaries/README.md`, leaving `tftpDir` free of the placeholder file | Kernel (`vmlinuz`) and initrd (`initrd.img.gz`) staging continue operating unchanged |
| AC-2 | Calling `validateStaging` when a 28-byte placeholder `ipxe.efi` or `ipxe.pxe` is already present in `tftpDir` (`placeholder-shipped-as-the-real-artifact`, 2026-08-28) | `validateStaging` returns the same actionable `*opererr.Error` naming the staged placeholder file path, cause, and fetch command | None; pre-existing placeholder files in `tftpDir` are never accepted as valid bootloaders |
| AC-3 | Calling `stageArtifacts` and `validateStaging` with a real PE `MZ` (`0x4d 0x5a`, >= 4096 bytes) `ipxe.efi` and a binary (>= 4096 bytes, non-ASCII placeholder) `ipxe.pxe` (`placeholder-shipped-as-the-real-artifact`, 2026-08-28) | Both `stageArtifacts` and `validateStaging` succeed with nil error and stage both binaries into `tftpDir` | Existing valid iPXE binaries already present in `tftpDir` remain untouched by `stageArtifacts` rather than being overwritten |

## TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestStageArtifactsRefusesIPXEPlaceholder` | `internal/plugins/provision/staging_test.go` | AC-1 |
| `TestValidateStagingRefusesIPXEPlaceholder` | `internal/plugins/provision/staging_test.go` | AC-2 |
| `TestStageAndValidateAcceptRealIPXEBinaries` | `internal/plugins/provision/staging_test.go` | AC-3 |

### Functional Tests

| Test | AC |
|------|----|
| Unit tests drive `stageArtifacts` and `validateStaging` directly against real `tools/ipxe-binaries/` placeholders and synthetic valid images | AC-1, AC-2, AC-3 |

## Files to Modify
- `internal/plugins/provision/staging.go`
- `internal/plugins/provision/staging_test.go`
- `docs/architecture/provisioning/pxe-staging.md`

## Files to Create
- None.

### Integration Checklist
- [ ] `stageArtifacts` rejects placeholder files before copying
- [ ] `validateStaging` rejects placeholder files already present in `tftpDir`
- [ ] Error message satisfies `ai/rules/user-facing-errors.md` (Problem, Why, Fix)

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/provisioning/pxe-staging.md` - document iPXE binary validation and placeholder refusal

## Design presentation

### What changes
- Adds `validateIPXEBinary` in `internal/plugins/provision/staging.go`, called from both `stageArtifacts` and `validateStaging`, rejecting placeholder `ipxe.efi` (missing PE `MZ` magic or < 4096 bytes) and `ipxe.pxe` (< 4096 bytes or plain ASCII placeholder) with an actionable `*opererr.Error`.

### Before and after
- **Before:** `ze install remote` copied 28-byte ASCII placeholder files from `tools/ipxe-binaries/` into `build/pxe/tftp/` and reported staging valid, failing at PXE firmware boot.
- **After:** Both staging and validation fail closed with an `*opererr.Error` naming the placeholder file and the exact instructions to fetch real iPXE binaries.

### Alternatives set aside
- **Checking in real third-party binary blobs into git under `tools/ipxe-binaries/`:** Rejected because binary redistribution and licensing in git is an owner decision (`ai/META-DECISIONS-SITUATIONAL.md`); refusing placeholders at staging closes the silent-failure defect immediately without committing third-party binaries.

### Documents departed from
None.

### Choices for Thomas
none

## Implementation Steps
1. Read `tools/ipxe-binaries/README.md` and `internal/plugins/provision/staging_test.go` to align exact remedy wording and existing test fixtures.
2. Add red unit tests `TestStageArtifactsRefusesIPXEPlaceholder`, `TestValidateStagingRefusesIPXEPlaceholder`, and `TestStageAndValidateAcceptRealIPXEBinaries` and confirm red at HEAD.
3. Implement `validateIPXEBinary` in `internal/plugins/provision/staging.go` and call it from `stageArtifacts` and `validateStaging`.
4. Update `docs/architecture/provisioning/pxe-staging.md`, verify `go test -race ./internal/plugins/provision/...` passes, and mark the owned journal row `fixed:` in `plan/journal/placeholder-shipped-as-the-real-artifact.md`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `placeholder-shipped-as-the-real-artifact` | 2026-08-28 | `tools/ipxe-binaries/ipxe.efi` |

## Work Not Done

Sibling runtime defects routed from `plan/release/fixes/spec-daemon-runtime-one-off-defects.md` live in Children 1 through 5 and Child 7.

## Review Gate

- [ ] Code review clean (`/ze-review`)
- [ ] Spec compliance verified (`/ze-review-spec`)
- [ ] Owned journal row marked `fixed:` in `plan/journal/placeholder-shipped-as-the-real-artifact.md`

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run label provision-ipxe command go test -race ./internal/plugins/provision/...`, `./le arch tier check`
