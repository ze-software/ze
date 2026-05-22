# Spec: install-6-installer-initrd

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-install-4 |
| Phase | - |
| Updated | 2026-05-21 |
| Parent | spec-install-0-umbrella.md |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-install-0-umbrella.md` - umbrella spec (Component 6 section)
4. `tools/installer-initrd/` - build artifacts (once created)

## Task

Build system for a minimal Linux initrd that PXE-boots on a target machine,
downloads a gokrazy disk image from ze-install's imageserver via HTTP,
writes it to the first non-removable disk, injects a pre-provisioned zefs
database onto partition 4 (/perm), and reboots. This is a build artifact,
not Go code inside ze.

The initrd is the final step in the PXE provisioning chain: PXE ROM loads
bootloader via TFTP, bootloader fetches installer kernel+initrd via HTTP,
kernel boots with initrd, initrd performs the installation.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-install-0-umbrella.md` - Component 6, Gokrazy Partition Layout, SSH Credential Flow
  -> Decision: u-root, Alpine busybox, or custom minimal initrd
  -> Constraint: gokrazy owns the partition table; clean write is safer than live migration
- [ ] `plan/spec-install-3-image-server.md` - HTTP endpoints the initrd downloads from
  -> Constraint: `/install/image/<name>` for disk image, `/install/database.zefs` for credentials
- [ ] `plan/spec-install-4-ze-install-binary.md` - ze-install server that serves the files
  -> Constraint: server IP passed via kernel cmdline or DHCP

### RFC Summaries (MUST for protocol work)
No RFC summaries needed. HTTP downloads use standard wget/curl, not custom protocol code.

**Key insights:**
- The initrd is a Linux build artifact, not Go plugin code
- Gokrazy partition 4 is ext4, mounted at /perm, holds database.zefs
- Server IP comes from kernel cmdline `ze.server=<ip>` (set by bootloader from DHCP)
- Image name from kernel cmdline `ze.image=<name>` (defaults to "ze.img")
- Disk target: first non-removable block device (/dev/sda, /dev/nvme0n1, or /dev/mmcblk0). Detection via sysfs `removable` attribute.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] No existing installer initrd code exists - this is entirely new

**Behavior to preserve:**
- Gokrazy partition layout (4 partitions: boot, rootA, rootB, perm) must not be altered
- ze-install imageserver HTTP endpoint paths (`/install/image/<name>`, `/install/database.zefs`)
- zefs database format (written as-is to `/perm/ze/database.zefs`)

**Behavior to change:**
- New build system for installer initrd (nothing exists today)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Linux kernel boots with this initrd as the root filesystem
- Kernel cmdline contains `ze.server=<ip>` and optionally `ze.image=<name>`

### Transformation Path
1. Init script parses kernel cmdline for `ze.server` and `ze.image` parameters
2. Network configured via Linux kernel `ip=dhcp` cmdline parameter (kernel-level DHCP auto-configuration, no userspace DHCP client needed in initrd)
3. HTTP GET `http://<server>/install/image/<name>` streams disk image
4. Disk image written directly to target block device (`dd` or direct write)
5. Re-read partition table via `blockdev --rereadpt /dev/sdX` (kernel may not detect new partitions after full-disk write). Mount partition 4 as ext4 at /mnt/perm
6. HTTP GET `http://<server>/install/database.zefs` downloads zefs database
7. database.zefs written to /mnt/perm/ze/database.zefs (mkdir -p /mnt/perm/ze first)
8. Unmount, sync, reboot

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel cmdline -> init script | `/proc/cmdline` parsing | [ ] |
| Network -> HTTP download | wget/curl to imageserver | [ ] |
| HTTP response -> block device | Pipe or temp file + dd | [ ] |
| Block device partition -> ext4 mount | `blockdev --rereadpt /dev/sdX` then `mount /dev/sdX4 /mnt/perm` | [ ] |

### Integration Points
- ze-install imageserver HTTP endpoints (spec-install-3)
- PXE bootloader iPXE script that sets kernel cmdline (spec-install-4)
- Gokrazy partition layout (partition 4 = /perm = ext4)
- zefs database format (opaque binary, written as-is)

### Architectural Verification
- [ ] No bypassed layers (initrd downloads from imageserver, not direct disk copy)
- [ ] No unintended coupling (initrd only depends on HTTP endpoints, not ze internals)
- [ ] No duplicated functionality (single-purpose build artifact)
- [ ] Zero-copy preserved where applicable (stream image directly to disk when possible)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| QEMU PXE boot with ze-install serving | -> | initrd downloads image, writes disk, injects zefs | `test-install-qemu-full` |
| Init script with mock HTTP server | -> | Image download + disk write + zefs injection | `test-initrd-install-flow` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Initrd boots in QEMU with `ze.server=<ip>` on cmdline | Init script parses server IP from `/proc/cmdline` |
| AC-2 | `ze.image` not set on cmdline | Defaults to "ze.img" |
| AC-3 | HTTP server at `ze.server` serves `/install/image/ze.img` | Image downloaded and written to first non-removable disk (/dev/sda, /dev/nvme0n1, or /dev/mmcblk0) |
| AC-4 | Image written to disk | Gokrazy partition table intact (4 partitions) |
| AC-5 | HTTP server serves `/install/database.zefs` | database.zefs downloaded and written to `/perm/ze/database.zefs` on partition 4 |
| AC-6 | No non-removable disk found | Init script prints error, drops to shell (does not reboot) |
| AC-7 | HTTP download fails (server unreachable) | Init script retries 3 times with backoff, then drops to shell with error |
| AC-8 | Installation completes successfully | System reboots |
| AC-9 | Build system invoked | Produces initrd image file suitable for PXE boot |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test-cmdline-parse` | `tools/installer-initrd/test/test-cmdline-parse.sh` | Kernel cmdline parsing extracts ze.server and ze.image | |
| `test-disk-detect` | `tools/installer-initrd/test/test-disk-detect.sh` | First non-removable disk detection logic | |
| `test-default-image-name` | `tools/installer-initrd/test/test-cmdline-parse.sh` | Missing ze.image defaults to "ze.img" | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ze.server IP | valid IPv4 | 255.255.255.254 | empty string | malformed IP |
| HTTP retry count | 1-3 | 3 | N/A (always retries at least once) | N/A (capped at 3) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-install-qemu-full` | `test/install/qemu-full.ci` | Full PXE boot -> image write -> zefs inject -> reboot in QEMU | |
| `test-initrd-install-flow` | `test/install/initrd-flow.ci` | Initrd with mock HTTP: download, write, verify partition layout | |

### Future (if deferring any tests)
- Full PXE chain test (DHCP + TFTP + HTTP + initrd) requires QEMU with PXE ROM -- may need dedicated CI infrastructure
- Multi-disk test (verify correct disk selected when multiple disks present)
- SHA256 image integrity verification test (v2 feature)

## Files to Modify

No existing files are modified. This spec creates a new build system.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A - build artifact, not ze plugin |
| CLI commands/flags | No | N/A |
| CLI grammar (action before identifier) | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |
| Doctor check for runtime dependencies | No | N/A - build-time only |
| Makefile target | Yes | `Makefile` - add `ze-installer-initrd` target |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - installer initrd |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/ze-install.md` - building the installer initrd |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create

- `tools/installer-initrd/Makefile` - build system for initrd image
- `tools/installer-initrd/init` - init script (PID 1) that performs the installation
- `tools/installer-initrd/README.md` - build instructions and usage
- `tools/installer-initrd/test/test-cmdline-parse.sh` - cmdline parsing tests
- `tools/installer-initrd/test/test-disk-detect.sh` - disk detection tests
- `test/install/qemu-full.ci` - QEMU-based full PXE boot functional test
- `test/install/initrd-flow.ci` - initrd install flow functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella Component 6 |
| 2. Audit | Files to Create - verify none exist yet |
| 3. Wiring phase | Build system: Makefile target produces initrd file |
| 4. Implement (TDD) | Init script, disk detection, download logic |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | Build initrd, run QEMU test |
| 7. Critical review | Critical Review Checklist below |
| 8-13. Fix/verify loop | Per finding |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- Makefile + skeleton init script
   - Tests: `make ze-installer-initrd` produces a file
   - Files: `tools/installer-initrd/Makefile`, `tools/installer-initrd/init`
   - Verify: Makefile target runs, produces an initrd image (even if init is a no-op)

2. **Phase: Cmdline parsing** -- init script reads ze.server and ze.image from /proc/cmdline
   - Tests: `test-cmdline-parse.sh`
   - Files: `tools/installer-initrd/init`
   - Verify: Parsing handles present, absent, and default cases

3. **Phase: Disk detection** -- find first non-removable block device
   - Tests: `test-disk-detect.sh`
   - Files: `tools/installer-initrd/init`
   - Verify: Correctly identifies /dev/sda, /dev/nvme0n1, or /dev/mmcblk0 via sysfs `removable` attribute; skips removable devices

4. **Phase: Download and write** -- HTTP download of image, write to disk, zefs injection
   - Tests: `test-initrd-install-flow` with mock HTTP server
   - Files: `tools/installer-initrd/init`
   - Verify: Image written, partition 4 mounted, database.zefs placed correctly

5. **Phase: QEMU integration** -- full PXE boot test
   - Tests: `test-install-qemu-full`
   - Files: `test/install/qemu-full.ci`
   - Verify: QEMU boots initrd, downloads image, writes disk, target boots into gokrazy+ze

6. **Phase: Documentation** -- README and guide
   - Files: `tools/installer-initrd/README.md`, `docs/guide/ze-install.md`
   - Verify: Build instructions are complete and accurate

7. **Full verification** -- all tests pass
8. **Complete spec** -- fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation in init script |
| Correctness | Partition 4 is ext4, /perm/ze/database.zefs path is correct |
| Error handling | HTTP failure retries, missing disk drops to shell |
| Disk safety | Only writes to first non-removable disk, never to removable |
| Idempotency | Re-running init script on same hardware produces same result |
| Build reproducibility | Makefile produces consistent initrd from same inputs |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `tools/installer-initrd/init` exists and is executable | `ls -la tools/installer-initrd/init` |
| `tools/installer-initrd/Makefile` builds initrd | `make -C tools/installer-initrd` |
| Initrd boots in QEMU | QEMU test output |
| database.zefs lands at /perm/ze/database.zefs | QEMU test verifies file presence |
| Cmdline parsing handles defaults | `test-cmdline-parse.sh` passes |
| Disk detection skips removable | `test-disk-detect.sh` passes |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | ze.server must be valid IPv4; reject malformed values |
| Disk target | Verify block device is non-removable before writing; never overwrite removable media |
| Download source | Only download from ze.server specified on cmdline; no DNS resolution in v1 |
| No secrets in initrd | Initrd image itself contains no credentials; database.zefs is downloaded at runtime |
| Image integrity | v1: no verification (documented limitation). v2: SHA256 checksum |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Build fails | Fix Makefile or missing build dependencies |
| Cmdline parse fails | Fix init script parsing logic |
| Disk detection wrong | Fix sysfs/procfs enumeration logic |
| HTTP download fails | Check imageserver endpoints (spec-install-3) |
| Partition mount fails | Check partition detection after image write |
| QEMU test fails | Check QEMU config, network setup, PXE chain |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

No RFC work needed. HTTP downloads use standard clients, not custom protocol code.

## Implementation Summary

### What Was Implemented
- [Not started]

### Bugs Found/Fixed
- [None yet]

### Documentation Updates
- [None yet]

### Deviations from Plan
- [None yet]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] Build system produces initrd image
- [ ] QEMU test passes end-to-end
- [ ] Documentation complete (README + guide)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
