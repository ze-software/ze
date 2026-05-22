# Spec: zefs-integrity

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-05-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/zefs-format.md` - disk format
4. `pkg/zefs/store.go` - store implementation
5. `pkg/zefs/netcapstring.go` - netcapstring encoding/decoding

## Task

ZeFS has no tool to detect or repair store corruption. The store validates structure on decode (magic, netcapstring framing, key validity) but cannot pinpoint which entry is corrupt, cannot recover valid entries from a partially damaged file, and has no mechanism to detect silent bit rot between writes.

This spec adds per-record CRC32c checksums to the netcapstring format, an in-place write path that avoids full re-encode when capacity allows, and CLI tooling for integrity checking and best-effort repair.

### Three coupled changes

1. **Per-record CRC32c in netcapstring format**: each netcapstring (entry keys, entry values, and the container itself) gets a CRC32c in its header. The container CRC covers all encoded entries, giving whole-file structural verification for free.
2. **In-place write path**: when a value change fits within its slot capacity and no layout shift is needed, write only the changed entry header+data and the container header CRC via pwrite. Full re-encode only when capacity changes force a layout shift.
3. **Check and repair CLI**: `ze data check` for corruption detection, `ze data repair` for best-effort entry-by-entry recovery.

These are coupled: the CRC makes partial writes trustworthy (you can verify no corruption after an in-place update), and the check/repair tooling consumes the CRC to pinpoint corrupt entries.

4. **Encode CLI**: `ze data encode` for inspecting netcapstring encoding of arbitrary strings. Outputs CRC only, header only, or full encoded line depending on flags.

No backward compatibility requirement. Ze has not seen production.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - disk format, netcapstring spec, container layout, key namespaces
  -> Decision: netcapstring is the universal framing unit; container is itself a netcapstring
  -> Constraint: the `<number>` field determines field widths; header size is deterministic from capacity
- [ ] `docs/architecture/core-design.md` - registration patterns, component isolation

### RFC Summaries (MUST for protocol work)
None. This is internal storage, not protocol work.

**Key insights:**
- The container is a netcapstring wrapping entry netcapstrings. Adding CRC to the format gives per-entry and whole-file coverage in one mechanism.
- Netcapstring capacity allows in-place writes. Currently wasted: `flush()` does full re-encode every time.
- mmap is `PROT_READ` + `MAP_PRIVATE`. In-place writes use pwrite on the file descriptor, then re-mmap for the read path.
- Single-process ownership (daemon owns the blob). No cross-process locking needed.
- `writeAt` and `writeData` methods on `netcapSlot` already exist for in-place buffer manipulation but are not used by `flush()`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/zefs/store.go` - BlobStore: Create/Open/Close, ReadFile/WriteFile/Remove, encode/decode, flush (full re-encode + atomic temp+rename), recoverFromEncoded
  -> Constraint: flush() snapshots previous on-disk state for recovery; must preserve this safety property
  -> Constraint: atomicWrite does temp+rename+fsync+dir-fsync; in-place writes need equivalent durability
- [ ] `pkg/zefs/netcapstring.go` - format: `<number>:<cap>:<used>\n<data><padding>\n`, encode/decode, writeNetcapstring/writeNetcapstringHeader, decodeNetcapstringRef (zero-copy), netcapSlot with writeData/writeAt
  -> Constraint: `<number>` field is self-describing width; adding CRC adds a fixed-width field after `<used>`
- [ ] `pkg/zefs/tree.go` - in-memory tree: set/get/remove/has/walk/collect on nodes
- [ ] `pkg/zefs/guard.go` - WriteGuard/ReadGuard interfaces for concurrent access
- [ ] `pkg/zefs/lock.go` - WriteLock (exclusive, batched writes, flush on Release), ReadLock (shared, zero-copy)
  -> Constraint: WriteLock.Release() calls flush(); in-place path must integrate here
- [ ] `pkg/zefs/mmap_unix.go` - mmap PROT_READ MAP_PRIVATE; loadBacking/unloadBacking
  -> Constraint: MAP_PRIVATE means writes to the fd are not visible via mmap; must re-mmap after pwrite
- [ ] `pkg/zefs/mmap_other.go` - heap fallback: os.ReadFile, no fd
  -> Constraint: in-place write path must fall back to full rewrite on non-unix (no pwrite target)
- [ ] `pkg/zefs/keys.go` - registered key patterns via MustRegister
- [ ] `pkg/zefs/registry.go` - KeyEntry with Pattern/Description/Private, Key()/Prefix()/Dir()
- [ ] `pkg/zefs/file.go` - fs.File/DirEntry wrappers for fs.FS interface
- [ ] `cmd/ze/data/main.go` - subcommand handlers: import, write, rm, ls, cat, registered
- [ ] `cmd/ze/doctor/doctor.go` - system readiness checks
- [ ] `cmd/ze/health_revert.go` - health-based config revert using store
- [ ] `cmd/ze/pushed_config.go` - pushed config loading using store, sha256 for config hashing

**Behavior to preserve:**
- All existing public API signatures (ReadFile, WriteFile, Remove, Open, ReadDir, List, Has, Export, Import, Lock, RLock)
- fs.FS, fs.ReadFileFS, fs.ReadDirFS interface satisfaction
- WriteGuard/ReadGuard interface contracts
- Atomic write safety: failed writes must not leave corrupt state
- Zero-copy reads via mmap for lock-scoped access
- Caller-owned copies from unlocked ReadFile/Open
- Entry count limit (maxEntryCount = 100,000)
- Import size limit (maxImportSize = 256 MB)
- Key validation via fs.ValidPath
- Path conflict detection (file vs directory)

**Behavior to change:**
- Netcapstring header format: add CRC32c field
- `flush()`: use in-place pwrite path when layout unchanged, full rewrite only when layout shifts
- `decodeNetcapstringRef`: verify CRC on decode
- `writeNetcapstring`/`writeNetcapstringHeader`: compute and write CRC

## Data Flow (MANDATORY)

### Entry Point: Write Path (in-place)
- WriteLock.WriteFile() is called with key + data
- Data is stored in-memory tree via writeFileNoFlush()
- On WriteLock.Release(), flush() decides: in-place or full rewrite

### Transformation Path (in-place write)
1. flush() checks: did any entry exceed its slot capacity? Were entries added or removed?
2. If no layout change: compute dirty set (entries whose data changed)
3. For each dirty entry: compute CRC32c of new data, pwrite entry header (used + CRC) and data region
4. Recompute container CRC32c over the full container data region, pwrite container header
5. fsync the file descriptor
6. Re-mmap for the read path (MAP_PRIVATE means pwrite changes are not visible via existing mmap)

### Transformation Path (full rewrite)
1. flush() detects layout change (capacity exceeded, entry added/removed)
2. Full encode() as today, but with CRC32c in each netcapstring header
3. Atomic write via temp+rename+fsync (existing path)
4. Re-mmap new file

### Entry Point: Check Path
- `zefs.Check(path)` opens the file read-only, decodes all netcapstrings, verifies every CRC
- Reports per-entry status and container-level status

### Entry Point: Repair Path
- `zefs.Repair(src, dst)` scans forward entry-by-entry, skips unparseable or CRC-mismatched entries, writes valid entries to new file

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> pkg/zefs | `ze data check`/`repair` call Check()/Repair() | [ ] |
| ze doctor -> pkg/zefs | Doctor calls Check() on default store path | [ ] |
| WriteLock -> flush | Release() triggers in-place or full write | [ ] |

### Integration Points
- `cmd/ze/data/main.go` subcommandHandlers map: add `check` and `repair` entries
- `cmd/ze/doctor/doctor.go`: add zefs store health check
- `pkg/zefs/store.go` flush(): decision logic for in-place vs full rewrite
- `pkg/zefs/netcapstring.go`: CRC32c computation and verification

### Architectural Verification
- [ ] No bypassed layers (check/repair use the same decode path as Open)
- [ ] No unintended coupling (check/repair are read-only, do not import daemon components)
- [ ] No duplicated functionality (in-place write reuses existing writeData/writeAt on netcapSlot)
- [ ] Zero-copy preserved where applicable (read path unchanged; CRC verified during decode)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze data check` CLI | -> | `zefs.Check()` | `TestCheckCLICallsCheck` |
| `ze data repair` CLI | -> | `zefs.Repair()` | `TestRepairCLICallsRepair` |
| `ze data encode` CLI | -> | `zefs.encodeNetcapstring()` | `TestEncodeCLIFull` |
| `ze doctor` | -> | `zefs.Check()` on default store | `TestDoctorChecksZeFS` |
| `WriteLock.Release()` | -> | in-place flush path | `TestWriteLockInPlaceFlush` |
| `decodeNetcapstringRef` | -> | CRC verification | `TestDecodeCRCVerification` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Write a netcapstring, read it back | CRC32c is present in header and verified on decode |
| AC-2 | Flip a bit in a netcapstring data region, decode it | Decode returns CRC mismatch error naming the offset |
| AC-3 | Flip a bit in a netcapstring header CRC field, decode it | Decode returns CRC mismatch error |
| AC-4 | `ze data check` on a valid store | Exit 0, report shows all entries OK and container CRC OK |
| AC-5 | `ze data check` on a store with one bit-flipped entry | Exit 1, report names the corrupt entry key |
| AC-6 | `ze data check` on a truncated file | Exit 1, report identifies truncation point |
| AC-7 | `ze data repair --output new.zefs` on a store with one corrupt entry | New file contains all valid entries, report lists the skipped entry |
| AC-8 | `ze data repair --output new.zefs` on a valid store | New file is functionally identical (same keys and values) |
| AC-9 | `ze doctor` on system with valid default store | Doctor reports store health OK |
| AC-10 | `ze doctor` on system with corrupt default store | Doctor reports store corruption |
| AC-11 | WriteLock: write a value that fits existing capacity, Release() | Only entry header+data and container header are rewritten (no temp file created) |
| AC-12 | WriteLock: write a value that exceeds slot capacity, Release() | Full rewrite via temp+rename (existing atomic path) |
| AC-13 | WriteLock: add a new entry within container capacity, Release() | Append + container header update (no full rewrite) |
| AC-14 | WriteLock: add a new entry that exceeds container capacity, Release() | Full rewrite via temp+rename |
| AC-15 | WriteLock: remove an entry, Release() | Full rewrite (entries shift) |
| AC-16 | In-place write on non-unix platform | Falls back to full rewrite (no pwrite available) |
| AC-17 | Power loss during in-place write | CRC detects inconsistency on next Open; `ze data check` reports it; `ze data repair` recovers unaffected entries |
| AC-18 | `ze data encode "hello"` | Outputs the full encoded netcapstring line (header + data + padding + terminator) |
| AC-19 | `ze data encode --crc "hello"` | Outputs only the CRC32c hex value (8 lowercase hex chars) |
| AC-20 | `ze data encode --header "hello"` | Outputs only the header portion (`<number>:<cap>:<used>:<crc>`) |
| AC-21 | `ze data encode --cap 100 "hello"` | Encodes with explicit capacity 100 instead of default (used + 10%) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNetcapstringCRC` | `pkg/zefs/netcapstring_test.go` | CRC32c written in header, round-trips correctly | |
| `TestNetcapstringCRCMismatch` | `pkg/zefs/netcapstring_test.go` | Decode rejects data with wrong CRC | |
| `TestNetcapstringHeaderLen` | `pkg/zefs/netcapstring_test.go` | Header length calculation includes CRC field | |
| `TestCheckValidStore` | `pkg/zefs/check_test.go` | Check returns clean report for valid store | |
| `TestCheckTruncatedFile` | `pkg/zefs/check_test.go` | Check reports truncation | |
| `TestCheckBitFlippedEntry` | `pkg/zefs/check_test.go` | Check identifies corrupt entry by key | |
| `TestCheckBitFlippedContainer` | `pkg/zefs/check_test.go` | Check identifies container CRC failure | |
| `TestRepairPartialCorruption` | `pkg/zefs/check_test.go` | Repair recovers valid entries, skips corrupt | |
| `TestRepairValidStore` | `pkg/zefs/check_test.go` | Repair produces identical copy | |
| `TestFlushInPlace` | `pkg/zefs/store_test.go` | Value update within capacity uses pwrite, no temp file | |
| `TestFlushFullRewriteOnGrowth` | `pkg/zefs/store_test.go` | Value exceeding capacity triggers full rewrite | |
| `TestFlushFullRewriteOnRemove` | `pkg/zefs/store_test.go` | Entry removal triggers full rewrite | |
| `TestFlushAppendWithinContainer` | `pkg/zefs/store_test.go` | New entry within container capacity appends in-place | |
| `TestFlushFullRewriteOnContainerGrowth` | `pkg/zefs/store_test.go` | New entry exceeding container triggers full rewrite | |
| `TestInPlaceMultipleEntries` | `pkg/zefs/store_test.go` | Multiple dirty entries updated in one flush | |
| `TestInPlaceFallbackNonUnix` | `pkg/zefs/store_test.go` | Non-unix path falls back to full rewrite | |
| `TestCRCVerifiedOnOpen` | `pkg/zefs/store_test.go` | Open rejects store with corrupt CRC | |
| `TestInPlaceCrashRecovery` | `pkg/zefs/store_test.go` | Partial pwrite detected by CRC on next open | |
| `TestEncodeCLIFull` | `cmd/ze/data/main_test.go` | `ze data encode` outputs full netcapstring | |
| `TestEncodeCLICRC` | `cmd/ze/data/main_test.go` | `ze data encode --crc` outputs only CRC hex | |
| `TestEncodeCLIHeader` | `cmd/ze/data/main_test.go` | `ze data encode --header` outputs only header | |
| `TestEncodeCLICustomCap` | `cmd/ze/data/main_test.go` | `ze data encode --cap N` uses explicit capacity | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| CRC hex field | 8 chars, 00000000-ffffffff | ffffffff | N/A (unsigned) | N/A (unsigned) |
| Number field | 1-19 | 19 | 0 | 20 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-data-check-valid` | `test/data/*.ci` | User runs `ze data check` on healthy store | |
| `test-data-check-corrupt` | `test/data/*.ci` | User runs `ze data check` on damaged store | |
| `test-data-repair` | `test/data/*.ci` | User runs `ze data repair` to recover entries | |
| `test-data-encode` | `test/data/*.ci` | User runs `ze data encode` to inspect netcapstring encoding | |

## Files to Modify

- `pkg/zefs/netcapstring.go` - add CRC32c to header format (write + read + verify)
- `pkg/zefs/netcapstring_test.go` - update all tests for new header format, add CRC tests
- `pkg/zefs/store.go` - in-place flush path, dirty tracking, pwrite logic
- `pkg/zefs/store_test.go` - update all tests for new format, add in-place flush tests
- `pkg/zefs/mmap_unix.go` - add writable fd management for pwrite
- `pkg/zefs/mmap_other.go` - in-place write falls back to full rewrite
- `cmd/ze/data/main.go` - add `check`, `repair`, and `encode` subcommand handlers
- `cmd/ze/doctor/doctor.go` - add zefs store health check

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (offline CLI only) |
| CLI commands/flags | Yes | `cmd/ze/data/main.go` |
| CLI grammar (action before identifier) | Yes | `ze data check`, `ze data repair`, `ze data encode` -- action first, correct |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | Functional tests for CLI |
| Doctor check for runtime dependencies | Yes | `cmd/ze/doctor/doctor.go` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add zefs integrity checking |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - add `ze data check` and `ze data repair` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | Yes | `docs/architecture/zefs-format.md` - update netcapstring format with CRC field |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/zefs-format.md` - in-place write path, CRC verification |

## Files to Create

- `pkg/zefs/check.go` - Check(), Repair(), CheckReport, RepairReport, EntryStatus types
- `pkg/zefs/check_test.go` - corruption injection tests for check and repair
- `pkg/zefs/pwrite_unix.go` - pwrite wrapper for in-place writes (build tag: unix)
- `pkg/zefs/pwrite_other.go` - stub that signals "full rewrite required" (build tag: !unix)
- `test/data/test-data-check-valid.ci` - functional test for ze data check on valid store
- `test/data/test-data-check-corrupt.ci` - functional test for ze data check on corrupt store
- `test/data/test-data-repair.ci` - functional test for ze data repair

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- register entry points, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Netcapstring CRC** -- add CRC32c to netcapstring header format
   - Tests: `TestNetcapstringCRC`, `TestNetcapstringCRCMismatch`, `TestNetcapstringHeaderLen`
   - Files: `pkg/zefs/netcapstring.go`, `pkg/zefs/netcapstring_test.go`
   - Format change: header becomes `<number>:<cap>:<used>:<crc>\n` where `<crc>` is 8-char zero-padded hex CRC32c of the `<used>` bytes
   - Update writeNetcapstring, writeNetcapstringHeader to compute CRC32c via `hash/crc32` (Castagnoli polynomial, hardware-accelerated)
   - Update decodeNetcapstringRef to parse and verify CRC; return error on mismatch naming the offset
   - Update netcapstringHeaderLen and netcapstringTotalLen for the 9 extra bytes (colon + 8 hex chars)
   - Update all existing netcapstring tests to use new format
   - Verify: all existing store tests still pass (they use the netcapstring functions)

2. **Phase: Dirty tracking** -- track which entries changed since last flush
   - Tests: none standalone (tested via flush tests in phase 4)
   - Files: `pkg/zefs/store.go`
   - Add dirty set to BlobStore (map or bitset tracking keys modified since last flush)
   - writeFileNoFlush marks entry as dirty
   - removeNoFlush sets a "layout changed" flag
   - New entry addition: if within container capacity, mark as "append"; if exceeding, mark "layout changed"
   - Verify: existing tests pass with dirty tracking (tracking only, no behavior change yet)

3. **Phase: Pwrite support** -- platform-specific in-place write
   - Tests: `TestInPlaceFallbackNonUnix`
   - Files: `pkg/zefs/pwrite_unix.go`, `pkg/zefs/pwrite_other.go`, `pkg/zefs/mmap_unix.go`
   - Unix: open file O_RDWR for pwrite; keep separate O_RDONLY fd for PROT_READ MAP_PRIVATE mmap
   - pwrite_unix.go: pwrite wrapper that writes regions and fsyncs
   - pwrite_other.go: returns sentinel error "pwrite not supported" to trigger full rewrite fallback
   - mmap_unix.go: extend loadBacking to also return a writable fd (or open separately in flush)
   - Verify: pwrite writes correct bytes at correct offsets

4. **Phase: In-place flush** -- flush() uses pwrite when layout unchanged
   - Tests: `TestFlushInPlace`, `TestFlushFullRewriteOnGrowth`, `TestFlushFullRewriteOnRemove`, `TestFlushAppendWithinContainer`, `TestFlushFullRewriteOnContainerGrowth`, `TestInPlaceMultipleEntries`, `TestWriteLockInPlaceFlush`
   - Files: `pkg/zefs/store.go`
   - flush() decision tree: see "In-Place Write Decision Tree" section below
   - After pwrite: re-mmap (munmap old, mmap new) so read path sees changes
   - Fallback: if pwrite returns "not supported", do full rewrite
   - Container CRC: recompute by reading the container data region from the file after entry pwrites, compute CRC, pwrite container header
   - Verify: verify via file size (in-place write does not change file size for updates), verify via temp file detection (in-place does not create temp files)

5. **Phase: Check and Repair** -- integrity verification and recovery
   - Tests: `TestCheckValidStore`, `TestCheckTruncatedFile`, `TestCheckBitFlippedEntry`, `TestCheckBitFlippedContainer`, `TestRepairPartialCorruption`, `TestRepairValidStore`, `TestCRCVerifiedOnOpen`, `TestInPlaceCrashRecovery`
   - Files: `pkg/zefs/check.go`, `pkg/zefs/check_test.go`
   - Check(path) opens file read-only, decodes all netcapstrings with CRC verification, returns CheckReport with: magic status, container CRC status, per-entry list (key, CRC status, data size)
   - Repair(src, dst) scans entry-by-entry, skips entries with parse errors or CRC mismatches, writes valid entries to new store via Create+WriteFile, returns RepairReport with recovered and skipped entry lists
   - Verify: corruption injection tests (bit flip at known offsets, truncation at various points)

6. **Phase: CLI + Doctor** -- wire check/repair/encode to CLI, add doctor integration
   - Tests: `TestCheckCLICallsCheck`, `TestRepairCLICallsRepair`, `TestDoctorChecksZeFS`, `TestEncodeCLIFull`, `TestEncodeCLICRC`, `TestEncodeCLIHeader`, `TestEncodeCLICustomCap`
   - Files: `cmd/ze/data/main.go`, `cmd/ze/doctor/doctor.go`
   - `ze data check`: call Check(), print report, exit 0 or 1
   - `ze data repair --output <path>`: call Repair(), print report
   - `ze data encode [--crc|--header] [--cap N] <string>`: encode string as netcapstring, output depends on flags
   - `ze doctor`: call Check() on default store path, report as pass/fail
   - Verify: functional tests exercise the full CLI path

7. **Functional tests** -- create after feature works
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | CRC32c computed over correct byte range (used bytes only, not padding) |
| Correctness | In-place write targets correct file offsets (entry offsets relative to container start) |
| Correctness | Container CRC recomputed after all entry pwrites, not before |
| Correctness | Re-mmap after pwrite so read path sees changes |
| Data flow | In-place path does not create temp files; full rewrite path still does |
| Data flow | Dirty tracking reset after successful flush |
| Data flow | Non-unix fallback to full rewrite is exercised |
| Durability | pwrite path does fsync before re-mmap |
| Durability | Partial pwrite failure leaves CRC-detectable state |
| CLI grammar | `ze data check`, `ze data repair` -- action before identifier |
| Doctor checks | Default store path checked in ze doctor |
| Rule: buffer-first | CRC computed during encode, not as a separate pass |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Netcapstring format includes CRC32c | `grep -n "crc32" pkg/zefs/netcapstring.go` |
| CRC verified on decode | `grep -n "crc.*mismatch\|crc.*verify" pkg/zefs/netcapstring.go` |
| In-place flush path exists | `grep -n "pwrite\|inPlace\|in-place" pkg/zefs/store.go` |
| Full rewrite fallback on layout change | `grep -n "layoutChanged\|fullRewrite" pkg/zefs/store.go` |
| Check function exists | `grep -n "func Check" pkg/zefs/check.go` |
| Repair function exists | `grep -n "func Repair" pkg/zefs/check.go` |
| ze data check subcommand | `grep -n '"check"' cmd/ze/data/main.go` |
| ze data repair subcommand | `grep -n '"repair"' cmd/ze/data/main.go` |
| ze data encode subcommand | `grep -n '"encode"' cmd/ze/data/main.go` |
| ze doctor zefs check | `grep -n "zefs\|Check\|store" cmd/ze/doctor/doctor.go` |
| zefs-format.md updated | `grep -n "crc\|CRC" docs/architecture/zefs-format.md` |
| pwrite_unix.go exists | `ls pkg/zefs/pwrite_unix.go` |
| pwrite_other.go exists | `ls pkg/zefs/pwrite_other.go` |
| check.go exists | `ls pkg/zefs/check.go` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Check/Repair operate on user-provided file paths: validate path, use existing maxImportSize limit for reads |
| Resource exhaustion | Repair reads corrupt input: apply same size and entry count limits as Import |
| CRC is not cryptographic | CRC32c detects accidental corruption, not adversarial tampering; do not present it as a security feature |
| Repair output path | Repair writes to user-specified output path: validate it is not the same as input (prevent overwriting evidence) |
| pwrite bounds | Verify pwrite offsets and lengths are within file size (no write past EOF unless appending) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| Existing tests fail after format change | Phase 1 must update all existing tests for new header format |
| pwrite offset miscalculation | Add offset validation assertion in pwrite wrapper |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Netcapstring Format Change

### Current format
| Field | Content |
|-------|---------|
| `<number>` | Digit count of `<cap>` (decimal ASCII) |
| `:` | Separator |
| `<cap>` | Capacity (zero-padded to `<number>` digits) |
| `:` | Separator |
| `<used>` | Used bytes (zero-padded to `<number>` digits) |
| `\n` | Header terminator |
| `<data>` | Content (`<used>` bytes) |
| `<padding>` | Space bytes (`<cap>` - `<used>`) |
| `\n` | Section terminator |

### New format
| Field | Content |
|-------|---------|
| `<number>` | Digit count of `<cap>` (decimal ASCII) |
| `:` | Separator |
| `<cap>` | Capacity (zero-padded to `<number>` digits) |
| `:` | Separator |
| `<used>` | Used bytes (zero-padded to `<number>` digits) |
| `:` | Separator |
| `<crc>` | CRC32c of the `<used>` data bytes, 8-char zero-padded lowercase hex |
| `\n` | Header terminator |
| `<data>` | Content (`<used>` bytes) |
| `<padding>` | Space bytes (`<cap>` - `<used>`) |
| `\n` | Section terminator |

### Header length change
Old: `3 + digitCount(digitCount(cap)) + 2 * digitCount(cap)`
New: `3 + digitCount(digitCount(cap)) + 2 * digitCount(cap) + 1 + 8` (add 9 bytes: colon + 8 hex digits)

### CRC scope
CRC32c is computed over the `<used>` data bytes only. Not over padding, not over the header. This means:
- Changing data changes the CRC (corruption detection)
- Padding changes (irrelevant) do not change the CRC
- Header corruption is detected by parse failure (malformed fields), not CRC

### Container CRC semantics
The container is itself a netcapstring. Its data region contains all encoded entries. Its CRC covers all entry bytes (headers + data + padding of all entries + end marker). A valid container CRC means the overall structure is intact. Individual entry CRCs pinpoint which entry is damaged if the container CRC fails.

## Encode CLI

`ze data encode` is a debugging/inspection tool for the netcapstring format. It takes a string argument and outputs its netcapstring encoding.

### Usage
`ze data encode [--crc|--header] [--cap N] <string>`

### Output modes

| Flag | Output | Example for "hello" with cap 5 |
|------|--------|------|
| (none) | Full encoded netcapstring (header + data + padding + terminator) | `1:5:5:3610a686\nhello\n` |
| `--crc` | CRC32c hex value only | `3610a686` |
| `--header` | Header portion only (no newline terminator) | `1:5:5:3610a686` |

### Capacity

Default capacity is `used + 10%` (same as growCapacity). `--cap N` overrides to an explicit value. Error if `N < len(data)`.

### Input

The `<string>` argument is the data to encode. If `-` is passed, read from stdin (for binary data or multi-line input).

## In-Place Write Decision Tree

### flush() decision logic

| Condition | Write strategy | Reason |
|-----------|---------------|--------|
| Entry removed | Full rewrite | Entries after deletion shift |
| Entry value exceeds slot capacity | Full rewrite from changed entry onward | Entries after growth shift |
| New entry exceeds container capacity | Full rewrite | Container needs larger capacity |
| New entry fits container capacity | pwrite: append entry + update container header (used + CRC) | No shifting needed |
| Existing entry value fits slot capacity | pwrite: entry header + data + container header CRC | No shifting needed |
| Platform lacks pwrite (non-unix) | Full rewrite | No in-place write mechanism |

### Dirty tracking

BlobStore gains:
| Field | Type | Purpose |
|-------|------|---------|
| dirty | set of keys | Entries modified since last flush |
| added | list of keys | New entries since last flush (ordered) |
| layoutChanged | boolean | Set on remove or capacity overflow |

Reset all after successful flush.

### pwrite sequence for in-place update (value change within capacity)

1. Open file O_RDWR (or use existing writable fd)
2. For each dirty entry: compute CRC32c of new data, write entry header (number:cap:used:crc\n) and data region at slot offset via pwrite
3. Read full container data region, compute container CRC32c
4. Write container header at its fixed offset via pwrite
5. fsync
6. Close writable fd
7. munmap old mapping, mmap new (PROT_READ MAP_PRIVATE)
8. Rebuild tree references to new backing

### Crash safety of in-place writes

In-place pwrite is not atomic. A crash mid-write can leave:
- Entry header updated but data partially written: entry CRC will mismatch on next decode
- Entry data written but container CRC not updated: container CRC will mismatch but individual entry CRCs may be valid
- All entry writes done but fsync not completed: OS buffer cache may or may not have persisted

In all cases: CRC mismatches are detectable by `ze data check`. Recovery via `ze data repair` scans entry-by-entry and recovers entries whose individual CRCs are valid.

This is a weaker durability guarantee than atomic temp+rename. The trade-off is acceptable because:
- The common case (config edit within capacity) avoids a full file rewrite
- CRC detection ensures corruption is never silent
- `ze data repair` provides a recovery path
- Critical operations (entry add/remove, capacity growth) still use atomic write

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

- Per-entry CRC in the netcapstring format is superior to a whole-file checksum stored as a key: gives corruption pinpointing for free, and the container CRC provides whole-file coverage since the container is itself a netcapstring.
- In-place writes and CRC are coupled: CRC makes partial writes trustworthy (detectable if incomplete), and in-place writes make CRC recomputation cheap (only dirty entries + container header).
- The netcapstring capacity system was designed for in-place updates but never used that way. This spec finally delivers on that design intent.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

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
- [ ] AC-1..AC-21 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`pkg/zefs/*`, `cmd/ze/data/*`, `cmd/ze/doctor/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (`docs/architecture/zefs-format.md`)
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-zefs-integrity.md`
- [ ] Summary included in commit
