# Spec: appliance-install-robust

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/7 |
| Updated | 2026-06-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. Architecture docs: `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `plan/learned/813-install-6-installer-initrd.md`
4. Source: `internal/appliance/cmd_build.go` (injectZeFS, findLastPartition), `cmd/ze/ze_core_start.go` (blob gate), `tools/installer-initrd/init`
5. Session state: `tmp/session/session-state-35627.md` (file digests)

## Task

Make the ze appliance build and install path robust and Go-native. One spec, two slices.

**Slice 1 — boot robustness (urgent; appliance must never silently brick):**
- Build-side: `injectZeFS` (internal/appliance/cmd_build.go) trusts `debugfs` exit codes, but `debugfs -R` exits 0 even when `mkdir ze` / `write ze/database.zefs` fail (`runExternal` uses `cmd.Output()`, which only flags non-zero exits). A broken image ships silently with an empty `/perm`; on boot `zefs.Create("/perm/ze/database.zefs")` fails because `/perm/ze/` is absent, `NewBlob` errors, `resolveStorage` falls back to filesystem, `ze start` trips the blob-storage gate (ze_core_start.go:156), gokrazy restart-loops, the box bricks. Fix: verify the DB landed after injection (read back via `debugfs -R "stat ze/database.zefs"` + size/hash) and hard-fail the build if missing/zero. Guard `findLastPartition` against non-standard layouts.
- Runtime-side: gokrazy first-boot auto-init fallback in `ze start`. When on gokrazy and blob storage is unavailable, MkdirAll the config dir, create the DB, write a minimal identity, fall through to the existing bootstrap (ze_core_start.go:177-192). TRADEOFF: un-bricks the box to a reachable state but WITHOUT the provisioned prod credentials/TLS cert/seed config; it does not restore the intended identity.

**Slice 2 — Go-native install (supersedes learned 813 knowingly):**
- Move the installer's detect → write → inject → verify logic out of `tools/installer-initrd/init` (884 lines busybox shell) into a `ze`/`ze-setup` subcommand the initrd calls, sharing ONE Go inject/verify code path with slice 1's build-side. ze already ships a Go binary, so the 813 "no Go in initrd" rationale no longer holds. e2fsprogs (mkfs/debugfs) stay as external invocations; the orchestration/control flow moves to Go.

Origin: a real bricked Intel N150 appliance. User is verifying on-device which failure mode (build-side empty image vs runtime /perm reformat/mount) actually occurred.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `docs/guide/appliance.md` - appliance build/init/iso pipeline, ze-setup binary
  → Constraint: ISO install "does not... fetch a separate ZeFS database, or mutate /perm after writing the disk image. The installed disk receives... the /perm/ze/database.zefs that build already injected" (appliance.md:546-555). So build-side verify is the ONLY guard for ISO installs.
  → Decision: ISO ships the raw image gzip-compressed; injectZeFS + WriteImageChecksum run on the raw .img BEFORE gzip; installer gunzips + verifies sha256. Verify must run inside injectZeFS, before checksum (cmd_build.go:149 before 156).
  → Constraint: appliance.md:185 documents existing fail-open philosophy ("If the credentials database is missing or unreadable, access is granted without authentication for emergency recovery"). The auto-init fallback aligns with this; it is not a new philosophy.
- [ ] `docs/guide/ze-install.md` - install paths (ISO vs PXE), kernel/initrd
  → Constraint: PXE/HTTP install downloads image AND database.zefs separately (ze-install.md:225; init:843-850). ISO carries DB in-image. A Go installer subcommand (slice 2) must cover both: ISO (image carries DB) and PXE (download + inject DB).
  → Decision: `ze install local` already "creates the config directory" when no database.zefs exists (ze-install.md:42) — reusable precedent for the fallback's MkdirAll.
- [ ] `plan/learned/813-install-6-installer-initrd.md` - busybox-vs-Go decision being superseded
  → Constraint: 813 chose busybox over u-root to avoid a Go-compilation dependency IN the initrd build, keep image small, allow drop-to-shell. Slice 2 supersedes this because ze already ships a Go binary; record the supersede explicitly in the learned summary.
- [ ] `plan/learned/675-appliance-1-builder.md` - appliance builder, injectZeFS origin
  → Constraint: gok/ext4 inject was originally a stub pending the external binary; injectZeFS is the realized inject. `runExternalFn`/`gokBuildFn`/`e2fsDir` are test seams to preserve.
- [ ] `plan/learned/579-gokrazy-4-resilience.md` - gokrazy resilience patterns, NamespaceSystem events
  → Decision: gokrazy resilience uses `NamespaceSystem` + `EventClockSynced` (pub/sub) for system-level readiness; the auto-init fallback is a startup concern, not an event subscriber — no new event needed.
- [ ] `ai/rules/config-surface.md` - YANG vs env var decision for the gokrazy fallback toggle
  → Decision: the fallback gate is a BOOTSTRAP-time setting (needed before config loads) → env-only is correct per the decision table (row "needed before config loads"). Reuse the existing `ze.gokrazy.enabled` bool env var (environment.go:94, read via env.IsEnabled). No new config surface. Update its description to note the auto-init meaning.

### RFC Summaries
- N/A (no protocol work)

**Key insights:** (filled during research)
- Gate at ze_core_start.go:156 trips only when `NewBlob` ERRORS, not merely when DB is absent. `NewBlob` creates the DB if the file is missing; it errors when `zefs.Create` fails because `/perm/ze/` parent dir does not exist (`os.CreateTemp` does not MkdirAll).
- `injectZeFS` has no read-back verification; `debugfs -R` exits 0 on internal failure, so a failed inject ships silently.
- ISO install path does NOT inject the DB separately (image must carry it); HTTP path injects DB post-write. So build-side verify is the only guard for ISO.
- `ze.gokrazy.enabled=true` and `ze.config.dir=/perm/ze` are set in `gokrazy/ze/config.json`.
- **gokrazy does NOT reformat /perm.** `mountfs()` (gokrazy modcache mount.go:156-164) tries ext4/vfat/bcachefs and, if all fail, logs and continues with /perm UNMOUNTED (read-only SquashFS root). So "gokrazy wipes /perm" is false.
- **gok `overwrite --full <file>` does NOT format /perm** (overwriteFile path, packerwrite.go:321-381 only logs an mkfs hint). Only the device path formats. So `injectZeFS`'s mkfs is necessary; it cannot be replaced by gok's format.
- **Two distinct runtime failure modes:** (X) valid ext4 mounts but DB write silently failed → auto-init fallback self-heals; (Y) injected ext4 unmountable by runtime kernel → /perm read-only → fallback's MkdirAll ALSO fails → still bricks. Distinguished by whether `mount -t ext4 /dev/nvme0n1p4` succeeds on the NVMe (Step 1).
- `injectZeFS` mkfs uses `-O ^metadata_csum` (diverges from gok device-path default features). Slice 1c must ensure runtime-mountability, not prevent a (non-existent) reformat.

## Current Behavior (MANDATORY)

**Source files read:** (digests in `tmp/session/session-state-35627.md`)
- [ ] `internal/appliance/cmd_build.go` - buildOne, runGokBuild, injectZeFS (no verify), findLastPartition (GPT, highest startLBA), runExternal (cmd.Output, only non-zero exit = err)
- [ ] `internal/appliance/cmd_assemble.go` - assembleZeFS builds provisioned DB (creds, cert, seed config)
- [ ] `cmd/ze/ze_core_start.go` - resolveStorage, blob gate (156), bootstrap (177-192)
- [ ] `internal/core/resolve/resolve.go` - Storage() resolution, filesystem fallback on err
- [ ] `internal/component/config/storage/blob.go` - NewBlob creates DB if missing, errors otherwise
- [ ] `pkg/zefs/store.go` - Create → atomicWrite → os.CreateTemp (no MkdirAll of parent)
- [ ] `internal/plugins/init/main.go` - runInit (interactive; reference for non-interactive auto-init)
- [ ] `tools/installer-initrd/init` - on-target installer shell (HTTP injects DB; ISO does not)
- [ ] `gokrazy/ze/config.json` - env: ze.config.dir=/perm/ze, ze.gokrazy.enabled=true

**Behavior to preserve:**
- `injectZeFS` success path unchanged when the DB genuinely lands (only adds a verify step).
- `assembleZeFS` provisioned-identity DB format unchanged.
- HTTP installer DB-injection behavior unchanged by slice 1; slice 2 reimplements it in Go with identical on-disk result.
- ISO installer: image written byte-for-byte unchanged, checksum verified, poweroff semantics preserved.
- `runExternalFn` / `gokBuildFn` / `e2fsDir` test-override seams.

**Behavior to change:**
- `injectZeFS` gains post-inject verification → hard build failure on missing/zero DB.
- `ze start` gains a gokrazy first-boot auto-init fallback instead of exiting at the blob gate.
- (Slice 2) installer `init` shrinks: detect/write/inject/verify move to a Go subcommand.

## Data Flow (MANDATORY)

### Entry Point
- Build: `ze-setup appliance build <name>` → `buildOne()`. Data = the appliance config + `database.zefs` produced by `assembleZeFS`.
- Runtime: gokrazy init exec's `ze start` with env (`ze.config.dir=/perm/ze`, `ze.gokrazy.enabled=true`). Data = the on-disk `/perm` filesystem state.
- Install: initrd PID 1 (`tools/installer-initrd/init`) parses the kernel cmdline (`ze.source`, `ze.server`, `ze.image`, `ze.target`, `ze.media-id`, ...).

### Transformation Path
1. **Build inject+verify:** `assembleZeFS` → `runGokBuild` (raw .img, /perm unformatted) → `injectZeFS` (mkfs → debugfs write DB → **V3 verify**) → `WriteImageChecksum`.
2. **Runtime resolve+fallback:** `resolveStorage` → `internalresolve.Storage` → `NewBlob`; on error + gokrazy → MkdirAll + create DB + write identity → existing bootstrap (177-192) → daemon run.
3. **Install:** detect disk/partition → download/locate image → sha256 verify → write (dd/gunzip) → (HTTP) mount + inject DB + verify → reboot/poweroff.

### Build-side inject (slice 1a)
`appliance build` → `buildOne()` → `assembleZeFS()` writes `database.zefs` (creds, cert, seed) → `runGokBuild()` produces raw `.img` with empty `/perm` → `injectZeFS(img, db)`: `findLastPartition` (GPT) → `mkfs.ext4 -E offset` → `dd` extract perm → `debugfs mkdir ze` → `debugfs write db ze/database.zefs` → `dd` write-back. **Gap:** no read-back. **Fix:** after write-back, re-extract perm (or operate on the in-place region) and `debugfs -R "stat ze/database.zefs"`; assert present + size matches source; non-zero → `exitError` + remove image. Then `WriteImageChecksum` (already post-inject).

### Runtime gate + fallback (slice 1b)
boot → gokrazy runs `ze start` (env `ze.config.dir=/perm/ze`, `ze.gokrazy.enabled=true`) → `resolveStorage()` → `internalresolve.Storage()` → `NewBlob("/perm/ze/database.zefs", "/perm/ze")`. If `/perm/ze/` missing → `zefs.Create` fails (`os.CreateTemp` no MkdirAll) → `NewBlob` err → filesystem fallback → gate `!IsBlobStorage` (ze_core_start.go:156) → exit 1 → gokrazy restart loop. **Fix:** before the gate exits, if `env.IsEnabled("ze.gokrazy.enabled")`: `os.MkdirAll(configDir, 0o700)` → re-resolve storage (now `NewBlob`→`zefs.Create` succeeds) → write minimal identity → fall through to existing bootstrap (177-192: `bootstrapConfigFromTemplate` else `bootstrapFromDiscovery`). Bootstrap writes NETWORK config only (KeyFileActive), NOT SSH/web creds.

### On-target install (slice 2)
PXE/HTTP: initrd → detect disk → download image → `dd` → mount p4 → mkdir ze → download `database.zefs` → inject. ISO: initrd → find media → verify sha256 → gunzip|dd → poweroff (no inject). **Slice 2:** initrd calls `ze install-disk` (Go) that absorbs detect/write/inject/**verify**, sharing the inject/verify code path with slice 1a. e2fsprogs stay external invocations from Go.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| build host ↔ image `/perm` | mkfs/debugfs/dd on .img file | [ ] |
| gokrazy init ↔ ze start | exec + env vars + exit code (restart loop) | [ ] |
| initrd ↔ ze binary (slice 2) | exec ze subcommand + kernel cmdline args | [ ] |
| ze start ↔ blob storage | resolveStorage → NewBlob → zefs | [ ] |

### Integration Points
- `injectZeFS` verify reuses `runExternalFn` (debugfs stat) — same test seam.
- Fallback reuses existing bootstrap (`bootstrapConfigFromTemplate`/`bootstrapFromDiscovery`) at 177-192 — no new config-gen code.
- Slice-2 subcommand reuses `injectZeFS`/`findLastPartition` logic (factored into a shared package callable from both `internal/appliance` build and the on-device install path).

### Architectural Verification
- [ ] No bypassed layers (fallback goes through NewBlob/zefs, not direct file writes)
- [ ] No duplicated functionality (slice 2 shares inject/verify with slice 1a; fallback reuses bootstrap)
- [ ] Fallback only triggers on gokrazy (env-gated), never in dev/server use

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The N150 brick is caused by an empty `/perm/ze/` (build-side inject failure or runtime reformat), not a different fault | Code chain: gate trips only when NewBlob errors → zefs.Create fails → parent dir missing | If wrong, slice 1 fixes the wrong layer | On-device Step 1 (inspect built .img perm + NVMe p4) | unvalidated |
| A-2 | `debugfs -R` exits 0 on internal subcommand failure | Known e2fsprogs behavior; `runExternal` uses cmd.Output() | If debugfs DID fail loudly, build would already error — root cause is elsewhere | Reproduce: inject with a bad dbPath, observe exit 0 | unvalidated |
| A-3 | `ze.gokrazy.enabled` env var is the right signal to gate the auto-init fallback | `gokrazy/ze/config.json:32` sets it; registered bool (environment.go:94); read via `env.IsEnabled` (main_servers.go:518) | Fallback could trigger off-appliance and create unwanted DBs | grep confirmed it is set only on the appliance | confirmed |
| A-4 | The existing bootstrap (177-192) brings the box up reachable enough for recovery (DHCP + discovered config) without provisioned SSH/web creds | `bootstrapConfigFromTemplate`/`bootstrapFromDiscovery` write only network config (KeyFileActive); SSH/web auth keys absent | Box may be reachable but with no/insecure auth (R-1), or not reachable at all | DESIGN must decide the auth posture; functional test on a creds-less DB | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Auto-init fallback creates a box with default/no credentials, a security regression vs provisioned identity | Box reachable but prod creds absent | Document loudly; emit a warning to kmsg; only trigger on gokrazy; build-side verify makes this path rare |
| R-2 | Slice 2 Go binary in initrd bloats the image or breaks the minimal initrd build | initrd size jump; build failure | Measure size; keep busybox fallback path; gate behind a separate implementation phase |
| R-3 | findLastPartition guard rejects a valid but non-standard gok layout | Build fails on a real image | Guard only adds explicit errors for impossible geometry, not stricter-than-gokrazy checks |
| R-4 | Case Y (unmountable injected ext4): /perm stays read-only, so the auto-init fallback's MkdirAll ALSO fails — box still bricks despite the fallback | `mount -t ext4` fails on NVMe (Step 1); kmsg "Could not mount permanent storage" | Build-side mountability verify (loopback mount the perm region at build, fail the build if it won't mount); match mkfs flags to runtime kernel; fallback logs a clear read-only-/perm diagnostic to kmsg instead of a silent exit |
| R-5 | Build-host loopback mount proves build-host kernel mountability, not the appliance runtime kernel's — a feature the build host has but the runtime lacks slips through | Box bricks despite a green build | Use the most conservative mkfs feature set (match gok/runtime); long-term, a QEMU boot test with the actual runtime kernel |

## Design Decisions (locked at DESIGN gate)

| Area | Decision | Rejected | Rationale |
|------|----------|----------|-----------|
| Build verify (1a/1c) | **V3**: debugfs `dump` read-back + size/sha compare (mandatory, no root) + `e2fsck -fn` structural check (best-effort) + loopback mount when privileged | V1 (read-back only), V2 (mandatory mount) | V1 misses malformed-fs; V2 breaks macOS/non-root builds. V3 closes the silent-write hole everywhere and adds integrity/mountability where possible. |
| Fallback auth (1b) | **A1**: connectivity-only, SSH/CLI closed, serial-console fail-open recovery | A2 (random printed password) | Minimum; no surprise-open SSH; matches existing recovery philosophy (`login.go`, `appliance.md:185`). Re-provisioning is the correct response to a failed inject. |
| Installer scope (2) | **S2a**: full Go installer; `init` shell becomes a thin wrapper (early-boot + exec + drop-to-shell) | S2b (core only), S2c (inject only) | User chose maximum de-shelling; supersedes 813. |
| Shared code reality | Build inject (mkfs/debugfs into image-file offset) and on-device inject (mount block device + write file) are DIFFERENT mechanisms. Shared = verification logic + input validators + one Go codebase + GPT geometry. NOT one literal inject function. | "one inject function" framing | Honest: the targets differ (file offset vs real device); over-unifying would force loopback/root on-device for no gain. |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-setup appliance build` with a forced debugfs-write failure | → | `injectZeFS` verify branch returns `exitError`, image removed | `TestInjectZeFSFailsWhenWriteSilentlyDropped` |
| `ze-setup appliance build` happy path | → | `injectZeFS` verify passes, checksum written | `TestInjectZeFSVerifyPassesOnGoodImage` |
| `ze start` on gokrazy, `/perm/ze` missing, `/perm` writable | → | gokrazy auto-init fallback creates DB + bootstraps, daemon runs | `TestStartGokrazyAutoInitFallback` (functional `.ci`) |
| `ze start` NOT on gokrazy, no blob storage | → | unchanged: exits 1 with blob-storage error | `TestStartNonGokrazyStillExits` |
| `ze start` on gokrazy, `/perm` read-only (Case Y) | → | distinct kmsg diagnostic, no silent loop | `TestStartGokrazyReadOnlyPermDiagnostic` |
| initrd `init` (PXE) | → | execs `ze install disk` Go subcommand which writes+injects+verifies | `test/install/installer-go-http.ci` |
| initrd `init` (ISO) | → | execs `ze install disk` which writes image + powers off | `test/install/installer-go-iso.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `injectZeFS` runs and the debugfs write of `ze/database.zefs` silently fails (file absent or zero) | Read-back via `debugfs dump` detects absence/size-zero; `injectZeFS` returns `exitError` and removes the image; build fails non-zero |
| AC-2 | Read-back succeeds but bytes differ from source `dbPath` | size/sha mismatch detected; build fails |
| AC-3 | Injected perm region has ext4 structural errors | `e2fsck -fn` reports errors; build fails with a clear message |
| AC-4 | Build host has root + loop devices | `injectZeFS` additionally loopback-mounts the perm partition and confirms `ze/database.zefs` present; non-root host skips this step without failing |
| AC-5 | mkfs invocation | feature flags are an explicit, documented, conservative set; a test asserts the exact `mkfs.ext4` argument vector |
| AC-6 | `ze start`, `ze.gokrazy.enabled=true`, `/perm` writable but `/perm/ze` absent | fallback creates `/perm/ze`, creates DB, writes instance identity, bootstraps config from discovery; daemon starts (exit not 1) |
| AC-7 | Same as AC-6 | NO SSH/web credential keys written (connectivity-only, A1); CLI/SSH auth remains closed |
| AC-8 | `ze start`, `ze.gokrazy.enabled` unset/false, no blob storage | unchanged: exits 1 with the existing blob-storage error; fallback does NOT run |
| AC-9 | `ze start`, `ze.gokrazy.enabled=true`, `/perm` read-only (cannot MkdirAll) | distinct diagnostic to kmsg/stderr naming read-only-/perm + ext4 mountability; no silent restart loop |
| AC-10 | initrd boots in HTTP/PXE mode | the Go installer subcommand detects the target disk, downloads + sha256-verifies the image, writes it, injects + verifies `database.zefs`, reboots |
| AC-11 | initrd boots in ISO mode | the Go installer finds + verifies the ISO image, writes it to disk, powers off |
| AC-12 | Malformed kernel cmdline inputs (bad disk path, image name, IPv4, port, media-id, sha256) | Go installer rejects them with the same validation parity as the shell validators; invalid input never reaches a URL/dd/path |
| AC-13 | `wget`/download stream dies mid-transfer | Go installer detects the partial transfer (does not treat a truncated body as success), retries, and fails closed after retries — parity with the shell `wget\|dd` exit-status guard |
| AC-14 | initrd `init` after S2a | shell reduced to early-boot mounts + busybox install + `exec ze install disk`; drop-to-shell-on-error preserved |

## End-to-End User Stories (MANDATORY)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | builds an appliance whose DB inject silently fails | `appliance build` → `injectZeFS` → verify catches it → build fails (no broken ISO ships) | `TestInjectZeFSFailsWhenWriteSilentlyDropped` |
| 2 | boots an appliance whose `/perm/ze` is missing but `/perm` is writable | gokrazy → `ze start` → fallback → bootstrap → daemon up, pingable | `TestStartGokrazyAutoInitFallback` (.ci) |
| 3 | boots an appliance whose `/perm` won't mount | gokrazy → `ze start` → fallback can't write → loud kmsg diagnostic | `TestStartGokrazyReadOnlyPermDiagnostic` |
| 4 | PXE-installs via the Go installer | initrd → `ze install disk` → download+verify+write+inject+verify → reboot | `test/install/installer-go-http.ci` |
| 5 | ISO-installs via the Go installer | initrd → `ze install disk` → verify+write → poweroff | `test/install/installer-go-iso.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInjectZeFSFailsWhenWriteSilentlyDropped` | `internal/appliance/cmd_build_test.go` | AC-1: verify catches silent debugfs failure | |
| `TestInjectZeFSDetectsSizeMismatch` | `internal/appliance/cmd_build_test.go` | AC-2 | |
| `TestInjectZeFSRunsE2fsck` | `internal/appliance/cmd_build_test.go` | AC-3 via runExternalFn seam | |
| `TestInjectZeFSMkfsArgsPinned` | `internal/appliance/cmd_build_test.go` | AC-5: exact mkfs argv | |
| `TestStartGokrazyAutoInitCreatesDB` | `cmd/ze/ze_core_start_test.go` | AC-6/AC-7: DB created, no creds | |
| `TestStartNonGokrazyNoFallback` | `cmd/ze/ze_core_start_test.go` | AC-8 | |
| `TestStartReadOnlyPermDiagnostic` | `cmd/ze/ze_core_start_test.go` | AC-9 | |
| `TestInstallDiskValidators` | `internal/install/.../disk_test.go` | AC-12 validator parity | |
| `TestInstallDiskPartialTransferFailsClosed` | `internal/install/.../disk_test.go` | AC-13 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| install port (ze.port) | 1-65535 | 65535 | 0 | 65536 |
| media-id length | 32 hex | 32 chars | 31 | 33 |
| sha256 length | 64 hex | 64 chars | 63 | 65 |
| perm partition size | >= 100MB (gok min) | n/a | <100MB rejected | n/a |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `installer-go-http` | `test/install/installer-go-http.ci` | PXE install via Go installer, DB present after | |
| `installer-go-iso` | `test/install/installer-go-iso.ci` | ISO install via Go installer, image written | |
| `start-gokrazy-autoinit` | `test/install/start-gokrazy-autoinit.ci` | boot with empty /perm self-heals | |
| `build-verify-catches-empty` | `test/install/build-verify.ci` | build fails when inject produces empty /perm | |

### Interop Tests
- N/A (no wire-protocol behavior). Justification recorded.

### Future (if deferring any tests)
- QEMU boot test with the actual runtime kernel to close R-5 (runtime-kernel mountability). Deferred unless Step 1 shows Case Y; requires user approval.

## Files to Modify
- `internal/appliance/cmd_build.go` — `injectZeFS` gains verify (V3); pin mkfs flags. `// Design:` none currently; add one referencing this area.
- `cmd/ze/ze_core_start.go` — gokrazy auto-init fallback before the blob gate (156); read-only-/perm diagnostic.
- `internal/component/config/environment.go` — extend `ze.gokrazy.enabled` description to note the auto-init meaning.
- `tools/installer-initrd/init` — reduce to early-boot + `exec ze install disk` + drop-to-shell (slice 2).
- `tools/installer-initrd/Makefile` — bake the ze installer binary into the initrd (slice 2).
- `docs/guide/appliance.md`, `docs/guide/ze-install.md` — document the verify, the fallback, the Go installer.
- `gokrazy/ze/config.json` — no change expected (env already present); confirm.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | fallback gate is bootstrap env-only per config-surface.md |
| Env var registration | Reuse | `ze.gokrazy.enabled` already registered (environment.go:94); update description only |
| CLI command (`ze install disk`) | Yes (slice 2) | `internal/install/` + dispatch in `cmd/ze/` |
| Doctor check | Yes | build-host: e2fsprogs/loop availability already partly covered; add a check that build verify ran |
| Functional test for new subcommand | Yes | `test/install/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/appliance.md` (build verify), `docs/guide/ze-install.md` (Go installer) |
| 3 | CLI command added/changed? | Yes (slice 2) | `docs/guide/command-reference.md` (`ze install disk`) |
| 6 | Has a user guide page? | Yes | `docs/guide/ze-install.md`, `docs/guide/appliance.md` |
| 12 | Internal architecture changed? | Yes | note the build-verify + fallback in the appliance/install arch docs |
| 16 | Changed file referenced by doc source anchors? | Check | grep `docs/` for `source: internal/appliance/cmd_build.go`, `tools/installer-initrd/init` |

## Files to Create
- `internal/appliance/diskverify.go` — V3 verify helpers (debugfs dump compare, e2fsck wrapper, optional loop-mount), used by build; unit-tested via `runExternalFn` seam.
- `internal/install/disk/` (package) — on-device installer: cmdline parse, validators, disk/partition detect, download+sha256, ISO/Ventoy media discovery, write, mount+inject (block device, pure Go), verify, reboot/poweroff. Shares validators + verify logic with build where the mechanism allows.
- `cmd/ze/install/disk.go` (or extend existing install dispatch) — wires `ze install disk` into the on-device binary baked in the initrd.
- `test/install/installer-go-http.ci`, `test/install/installer-go-iso.ci`, `test/install/start-gokrazy-autoinit.ci`, `test/install/build-verify.ci`.

## Implementation Steps

### Phasing principle
**Slice 1 (urgent, independently shippable) lands first.** Slice 2 (S2a port) is later phases behind it. The boot fix must not be blocked by the installer rewrite even though they share this spec.

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables | Deliverables Checklist |
| 12. Security review | Security Review Checklist |

### Implementation Phases

1. **Phase 1 — Build-side verify (slice 1a/1c) [SHIPPABLE].** Add V3 verify + pinned mkfs flags to `injectZeFS`.
   - Tests: `TestInjectZeFSFailsWhenWriteSilentlyDropped`, `TestInjectZeFSDetectsSizeMismatch`, `TestInjectZeFSRunsE2fsck`, `TestInjectZeFSMkfsArgsPinned`
   - Files: `internal/appliance/cmd_build.go`, `internal/appliance/diskverify.go`
   - Verify: build fails on forced inject failure; happy path unchanged.
2. **Phase 2 — Runtime fallback + diagnostic (slice 1b) [SHIPPABLE].** gokrazy auto-init fallback before the gate; read-only-/perm diagnostic.
   - Tests: `TestStartGokrazyAutoInitCreatesDB`, `TestStartNonGokrazyNoFallback`, `TestStartReadOnlyPermDiagnostic`, `test/install/start-gokrazy-autoinit.ci`
   - Files: `cmd/ze/ze_core_start.go`, `internal/component/config/environment.go`
   - Verify: empty-/perm boots; off-appliance unchanged; read-only emits diagnostic.
3. **Phase 3 — Slice 2 spike.** Bounded research: which binary/tag the initrd bakes, initrd size budget, on-device mount-inject in pure Go, install-subcommand placement. Output: a short design note appended here. (S2a is large; de-risk before the port.)
4. **Phase 4 — Go installer core (slice 2).** `internal/install/disk` + `ze install disk`: cmdline parse, validators (parity w/ shell), disk/partition detect, write, mount+inject, verify, reboot/poweroff. Functional parity tests.
   - Tests: `TestInstallDiskValidators`, `TestInstallDiskPartialTransferFailsClosed`, `test/install/installer-go-http.ci`, `test/install/installer-go-iso.ci`
5. **Phase 5 — initrd integration (slice 2).** Reduce `init` to wrapper + exec; bake binary via Makefile; preserve drop-to-shell. Supersede 813 in the learned summary.
6. **Functional tests** → all `.ci` above.
7. **Full verification** → `make ze-verify`.
8. **Complete spec** → audit tables, learned summary `plan/learned/NNN-appliance-install-robust.md` (record the 813 supersede + the gokrazy-no-reformat correction). Two commits (A: code+spec+learned; B: `git rm` spec).

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-14 has implementation with file:line |
| Correctness | Verify catches silent failure (forced) AND passes happy path; fallback only on gokrazy; off-appliance unchanged |
| Security | Fallback writes NO creds (A1); installer validators reject malformed cmdline before use; no creds logged |
| No-layering | Slice 2: shell logic fully removed where ported, not left dead alongside Go |
| Uniformity | Reuses runExternalFn seam, existing bootstrap, install subcommand pattern |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| injectZeFS verify | `go test ./internal/appliance -run InjectZeFS` |
| gokrazy fallback | `go test ./cmd/ze -run StartGokrazy` + `.ci` |
| Go installer | `go test ./internal/install/...` + `.ci` |
| initrd reduced | `wc -l tools/installer-initrd/init` materially smaller; drop-to-shell preserved |
| docs updated | `make ze-doc-test` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Fallback auth posture | No SSH/web credential keys written; no default password; serial fail-open still the only recovery |
| Installer input validation | disk path / image name / IPv4 / port / media-id / sha256 validated before reaching dd/URL/path (parity with shell) |
| No secret leakage | database.zefs bytes / hashes not logged; verify compares hashes without printing them |
| Image integrity | sha256 verified before booting written disk (parity with shell guards) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Verify false-negative (build green, runtime won't mount) | R-5: feature-flag alignment / QEMU runtime-kernel test |
| Fallback can't write /perm | R-4: diagnostic + Case-Y handling, not silent |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| gokrazy reformats /perm at first boot (audit Finding #2) | gokrazy never reformats; it mounts-or-skips, leaving /perm read-only on failure (mount.go:156-164) | Reading gokrazy modcache mountfs | Reframed slice 1c from "prevent reformat" to "ensure runtime-mountability" |
| gok formats /perm during `overwrite --full` | gok `overwriteFile` only logs an mkfs hint; injectZeFS's mkfs is necessary | packerwrite.go:321-381 | injectZeFS mkfs cannot be removed |
| build & on-device share one inject function | mechanisms differ (file offset vs block device); only verify/validators/codebase shared | on-device mounts a real device (pure Go), no debugfs needed | Honest slice-2 framing |

## Design Insights
- The auto-init fallback is a strict superset-safety over the current brick, but only for Case X (writable /perm). Case Y (unmountable fs) needs build-side mountability verification + a runtime diagnostic; the fallback alone cannot fix a read-only /perm.
- `ze.gokrazy.enabled` is the natural appliance-mode signal; it is bootstrap-time so env-only is correct (config-surface.md).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| V3 verify | V1, V2 | coverage without breaking non-root/macOS builds |
| A1 fallback auth | A2, A3 | minimum, matches existing serial recovery, no surprise SSH |
| S2a full Go installer | S2b, S2c | user-chosen maximum de-shelling |
| Phase slice 1 before slice 2 | one big bang | urgent boot fix must ship independently |

## Known Limitations
- Build-host verify cannot fully prove appliance-runtime-kernel mountability (R-5); a QEMU runtime-kernel boot test is the only complete proof and is deferred.
- Auto-init fallback yields a reachable-but-unprovisioned box; operator must re-provision (by design, A1).

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | (pending /ze-review) | | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| (filled at implement time) | | |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| (filled at implement time) | | |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | (resolve via Step 1) | |
| A-2 | (resolve via repro) | |
| A-3 | confirmed | grep: set only on appliance |
| A-4 | (resolve via design/test) | |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-appliance-install-robust.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary
- [ ] **Commit B:** `git rm plan/spec-appliance-install-robust.md` only
