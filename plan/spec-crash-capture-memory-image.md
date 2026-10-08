# Spec: crash-capture-memory-image -- opt-in full kernel memory image (crash capture Phase 2)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | spec-crash-capture.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Split out of `plan/spec-crash-capture.md` by the owner on 2026-10-08. The parent
keeps Phase 1 (panic message and backtrace through a reserved pstore region) and
closes after its step 7 QEMU labs.

-> Decision (owner, 2026-10-08): crash capture Phase 2, with the lockdown signing-key and full-memory-image (A-7) questions it carries, is this spec. `spec-crash-capture` closes after its Phase 7 QEMU labs.

Phase 2 adds a full memory image behind an opt-in leaf, for the rare kernel fault a
backtrace does not explain: a capture kernel staged through kexec, a writer that reads
`/proc/vmcore` into the crash storage, and an unconditional reboot back to service.
It is amd64-only, because gokrazy `reboot.go` never kexecs on `!amd64`
(`plan/spec-kernel-lockdown-hardening.md` C-2).

Two open questions travel with it, and neither is answered here:

| Question | Where it stands |
|----------|-----------------|
| Lockdown signing key | Lockdown integrity mode blocks UNSIGNED kexec. `plan/spec-kernel-lockdown-hardening.md` C-1 establishes that cross-build kexec needs a stable, long-lived image-signing key and leaves adopting it as a cost the owner may decline. This spec MUST NOT adopt the key on its own authority (parent R-1) |
| A-7, full image sized to RAM | Parent A-7, moved verbatim below. Owner confirmation is owed before Phase 2 starts |

## Acceptance Criteria

Moved verbatim from `plan/spec-crash-capture.md`:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-15 | Supported amd64 appliance with memory-image capture explicitly enabled, sufficient space, and the approved signing prerequisites satisfied; a kernel panic occurs | Readiness reports the capture kernel armed before the panic. The capture writer persists a usable memory image in the configured crash storage, the appliance returns to normal service, and an offline reader can open the image with the matching kernel symbols and recover the panic context |

The parent's AC-12 (memory-image leaf refused on non-amd64) and AC-13 (readiness
reports the space shortfall) stay in the parent, which implements them in its config
and readiness steps. The parent's step 8 also required them to hold with Phase 2
staging active, so this spec re-proves both once a capture kernel is staged.

## Required Reading

### Related Specs
- [ ] `plan/spec-crash-capture.md` - Phase 1, the reservation, readiness and `show crashes` surfaces this builds on
- [ ] `plan/spec-kernel-lockdown-hardening.md` - C-1 (image-signing key) and C-2 (amd64-only kexec)

## Current Behavior (MANDATORY)

**Source files read:** (the design phase reads each before writing this section)
- [ ] `gokrazy/kernel/kernel.config` - [crash-dump symbols absent; read at design time]
- [ ] `internal/appliance/kernelreq.go` - [kernel requirement floor; read at design time]

## Data Flow (MANDATORY)

### Entry Point
- [Where data enters: written at design time]

### Transformation Path
1. [written at design time]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Capture kernel ↔ storage | writer reads `/proc/vmcore`, writes the resolved target | No |

### Integration Points
- [written at design time]

## Risks & Assumptions

### Assumptions
Moved verbatim from the parent:

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-7 | Operators accept a full memory image sized to RAM when they opt into Phase 2 | Owner statement during design: full dumps accepted, gated on available space | Phase 2 should filter by default rather than capture everything | Owner confirmation before Phase 2 starts | unvalidated |

### Risks
Copied verbatim from the parent, which keeps them as history:

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Lockdown integrity mode blocks unsigned kexec, so Phase 2 cannot stage a capture kernel on a hardened profile | The staging call is refused with a permission error on a lockdown build | Phase 1 is unaffected because it uses no kexec. For Phase 2, either sign the capture kernel with the stable image key in `plan/spec-kernel-lockdown-hardening.md` C-1, or ship the hardened profile without the memory image and say so on the profile page. This spec does NOT adopt that key |
| R-2 | A Phase 2 memory image does not fit: sized by RAM, written to a `/perm` that may be far smaller | Readiness reports insufficient space before any panic | Check free space against the estimate plus a reserve at commit, at boot, and again before the write opens. Report not-armed with the shortfall rather than failing at panic time |
| R-3 | Writing a multi-gigabyte image extends the outage while the router is down | Write duration measured in the QEMU lab | Phase 2 is opt-in and off by default; document the downtime cost on the guide page. Never begin a write that cannot complete |
| R-5 | Phase 2 staging disturbs the normal-reboot kexec path gokrazy uses for OTA | An OTA update fails to reboot after the feature lands | Crash staging targets the reserved region through the crash-entry path, a separate slot from the normal kexec image. A QEMU lab asserts an OTA-style reboot still works with crash staging active |
| R-7 | Phase 2 runs code in the capture kernel, the least debuggable place, where a bug costs the reboot as well as the image | A box that panics and then does not come back | The writer does the minimum: read, write, reboot. It reboots unconditionally on every error path, so a failed capture still restores service. Phase 1 has no code on this path at all |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `system crash-dump memory-image enabled true`, then a kernel panic | → | capture-kernel staging and writer | `qemu-crash-capture-memory-image` |

## End-to-End User Stories

Moved verbatim from the parent:

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 6 | Opts into full-memory capture and investigates a panic after service returns | config → reservation and capture-kernel staging → panic → image persistence → normal boot → offline image inspection | `qemu-crash-capture-memory-image` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReadinessReportsSpaceShortfall` (extended to the image estimate) | `internal/plugins/crashes/readiness_test.go` | AC-13 and R-2 with Phase 2 staging | |

### QEMU Tests (Linux kernel behavior)
Moved verbatim from the parent:

| Test | Registration | What it proves | Status |
|------|--------------|----------------|--------|
| `qemu-crash-capture-memory-image` | `internal/le/qemu/actions.go`, amd64 runtime-kernel lab | AC-15: opt-in configuration, armed staging, induced panic, persisted image readable with matching kernel symbols, and return to normal service | not written |

## Files to Modify
- [written at design time; the parent named the crash-dump kernel symbols, the capture writer and the memory-image leaves]

### Integration Checklist
- [written at design time]

### Documentation Update Checklist (BLOCKING)
- [written at design time]

## Implementation Steps

The parent's step 8, moved verbatim:

1. **Phase: Phase 2, memory image (opt-in)** -- only after 1 to 7 are closed and A-7 is confirmed
   - Tests: `TestReadinessReportsSpaceShortfall` extended to the image estimate, `qemu-crash-capture-ota-unaffected` re-run with staging active, and `qemu-crash-capture-memory-image`
   - Files: crash-dump kernel symbols, the capture writer, the memory-image leaves
   - Verify: AC-12, AC-13, AC-15, R-5 and R-7 hold. The writer reboots unconditionally on every error path; the positive lab must prove that a usable image survives and service returns

"1 to 7" are the parent's implementation steps.

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
