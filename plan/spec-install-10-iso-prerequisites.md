# Spec: install-10-iso-prerequisites

| Field | Value |
|-------|-------|
| Status | complete |
| Depends | spec-install-8-appliance-iso (implemented) |
| Phase | 10/10 |
| Updated | 2026-06-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/appliance/main.go` - appliance command dispatch and help
4. `internal/appliance/cmd_iso.go` - ISO command, artifact resolution, builder invocation
5. `internal/appliance/cmd_build.go` - image build, e2fsprogs resolution
6. `internal/appliance/resolve.go` - XDG_CONFIG_HOME resolution pattern
7. `internal/core/diagnostic/doctor_registry.go` - external doctor check registration API
8. `internal/component/plugin/doctor/register.go` - example of external doctor check registration
9. `tools/installer-kernel/build.sh` - kernel build script (Docker container)
10. `tools/installer-kernel/Makefile` - kernel build Makefile (Docker invocation)
11. `tools/installer-initrd/Makefile` - initrd build (busybox-static + cpio + gzip)

## Task

Add CLI commands and doctor checks for ISO build prerequisites so operators can build bootable Ze appliance ISOs without manually invoking Makefiles or knowing which system packages to install.

Today, building an ISO requires the operator to: (1) manually run `make -C tools/installer-kernel ARCH=amd64` (needs Docker), (2) manually run `make -C tools/installer-initrd` (needs busybox-static, cpio, gzip), (3) know to install grub-mkstandalone and xorriso, and (4) know the right flags for each step. A third-party shell script automates this but lives outside Ze.

This spec adds:

1. **`ze appliance kernel [--arch amd64|arm64] [--version 7.0.11]`** -- build or download the installer kernel. Checks XDG cache first, tries downloading a pre-built artifact from a configurable URL (environment variable with compiled-in default), falls back to local Docker build if download fails.

2. **`ze appliance initrd`** -- build or download the installer initrd. Same cache/download/fallback pattern.

3. **`ze appliance iso --check`** -- report readiness of all ISO prerequisites (kernel, initrd, grub, xorriso) without building anything.

4. **Doctor checks** registered via `diagnostic.RegisterDoctorCheck()` from the appliance package for artifact and tool availability. These run as warnings in `ze doctor` so operators discover what is missing before attempting a build.

### Build tag placement

The new code lives in `internal/appliance/` and compiles under the `ze_setup` build tag. The `ze-setup` binary (`bin/ze-setup`, built with `-tags ze_setup`) becomes the "build the appliance" tool, containing all features needed for the full ISO pipeline. `setup_features_setup.go` must import `internal/appliance` so `ze-setup` includes `ze appliance init/build/iso/kernel/initrd` alongside `ze install remote`.

The `ze-appliance` binary (`bin/ze-appliance`, built with `-tags ze_appliance`) continues to include the appliance commands as it does today. Both binaries get the new kernel/initrd commands since both import `internal/appliance`.

There is no reason for the main `ze` binary (built with `ze_distro`) to carry this build-host tooling.

## Required Reading

### Architecture Docs

- [ ] `docs/guide/appliance.md` - appliance workflow, build process, gokrazy image pipeline
  → Decision: appliance commands are offline shell commands for build-host tooling, no daemon or bus presence.
  → Constraint: gokrazy module cache download is a one-time step documented here; kernel/initrd download follows the same pattern.
- [ ] `docs/guide/ze-install.md` - install flows (local, PXE, ISO)
  → Decision: ISO install is distinct from PXE; ISO embeds the image, PXE fetches it.
  → Constraint: kernel and initrd are shared between PXE and ISO modes; a single cached artifact serves both.
- [ ] `ai/rules/doctor-checks.md` - when to register doctor checks
  → Decision: external runtime dependencies (files, binaries, sockets) require doctor checks.
  → Constraint: checks registered via `diagnostic.RegisterDoctorCheck()` from the owning package's `init()`.
- [ ] `ai/rules/cli-grammar.md` - command grammar rules
  → Constraint: closed keywords before free-form values. `ze appliance kernel --arch amd64` follows action-first pattern.

### Rules and Patterns

- [ ] `ai/patterns/cli-command.md` - CLI command registration pattern
  → Constraint: commands register via dispatch table in `main.go`, handlers assigned from `init()` in their own files.
- [ ] `ai/rules/plugin-self-containment.md` - self-contained feature ownership
  → Constraint: appliance package owns its doctor checks; removing the appliance package removes its checks.

**Key insights:**
- The appliance package already uses `XDG_CONFIG_HOME` for directory resolution (`resolve.go`). XDG_CACHE_HOME follows the same pattern.
- Doctor checks can be registered externally via `diagnostic.RegisterDoctorCheck()` from any package's `init()`, exactly as `internal/component/plugin/doctor/register.go` does for plugin binary checks.
- The kernel build is fully containerized (Dockerfile + build.sh); the `ze appliance kernel` command wraps the same Docker invocation the Makefile does.
- The initrd build uses busybox-static, cpio, and gzip; these are only needed when the download path fails.
- `ze appliance iso` already resolves kernel at `tools/installer-kernel/build/Image` and initrd at `tools/installer-initrd/build/initrd.img.gz` with clear error messages when missing. The new commands place artifacts where `ze appliance iso` already looks.
- `ze appliance build` already resolves e2fsprogs at startup (`resolveE2FSDir()` in `cmd_build.go`).
- Doctor checks test for artifact existence (the result), not build-tool presence (one path to the result). An operator who downloads pre-built artifacts sees clean doctor output without Docker installed.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/appliance/main.go` - dispatches appliance subcommands via `dispatchTable()` built at call time from `applianceCommands()`. Handler vars (`cmdInit`, `cmdBuild`, `cmdIso`, etc.) assigned from `init()` in individual cmd files. Help via `helpfmt.Page`.
  → Constraint: add `kernel` and `initrd` entries to `applianceCommands()` and corresponding `cmdKernel`/`cmdInitrd` handler vars with `stub` default.
- [ ] `internal/appliance/cmd_iso.go` - ISO command resolves kernel (`defaultISOKernelPath()` returns `tools/installer-kernel/build/Image`), initrd (flag default `tools/installer-initrd/build/initrd.img.gz`), grub (`grub-mkstandalone` or `grub2-mkstandalone` via `isoLookPathFn`), and xorriso (via `resolveExecutable`). Fails with actionable error messages when missing.
  → Constraint: `--check` flag must reuse `resolveISOInput()` but stop before `buildISO()`. The resolution logic is the single source of truth.
  → Constraint: kernel path resolution must check XDG cache before the `tools/` fallback so `ze appliance iso` finds downloaded kernels without `--kernel`.
- [ ] `internal/appliance/cmd_build.go` - `resolveE2FSDir()` searches `/opt/homebrew/sbin`, `/usr/sbin`, `/sbin`, and Homebrew Cellar paths for `mkfs.ext4` and `debugfs`. Returns empty string when not found.
  → Constraint: doctor check for e2fsprogs should call `resolveE2FSDir()` directly, not duplicate the search logic.
- [ ] `internal/appliance/resolve.go` - `ResolveDir()` checks flag, then `ze.appliance.dir` env, then `XDG_CONFIG_HOME/ze/appliances`. Uses `os.Getenv("XDG_CONFIG_HOME")` with `~/.config` fallback.
  → Constraint: cache resolution must follow the same pattern: `os.Getenv("XDG_CACHE_HOME")` with `~/.cache` fallback.
- [ ] `internal/appliance/config.go` - `ApplianceConfig` has `Image.Arch` (amd64 or arm64). Constants `archAMD64 = "amd64"` and `archARM64 = "arm64"` defined.
  → Constraint: kernel `--arch` flag uses the same arch constants and validation.
- [ ] `internal/core/diagnostic/doctor_registry.go` - `RegisterDoctorCheck()` takes `DoctorCheck{Name, Phase, Order, Component, Dependencies, Platforms, Codes, Check}`. Phase is `pre-config`, `missing-config`, or `post-config`. Platforms must be valid strings from `host.Platform*`. Codes must start with `doctor-`. Name must be lower-kebab-case. Dependencies must be non-empty lower-kebab list.
  → Constraint: appliance checks use `pre-config` phase (no config needed to check if artifacts/binaries exist), component `appliance`, platform `any`.
- [ ] `internal/component/plugin/doctor/register.go` - registers `plugin-binaries` check at `post-config` phase, order 700, component `plugin`, dependencies `["external-binary"]`, platform `any`, code `doctor-plugin-missing`. Registration in `init()` with fatal exit on error.
  → Constraint: follow this exact pattern for appliance doctor check registration.
- [ ] `tools/installer-kernel/build.sh` - downloads kernel source from `cdn.kernel.org`, merges `kernel.config`, verifies built-in options (`CONFIG_IP_PNP_DHCP`, `CONFIG_VIRTIO_NET`, etc.), builds Image. Inputs: `LINUX_VERSION` (default 7.0.11), `ARCH` (arm64, amd64, x86_64), `JOBS`.
  → Constraint: `ze appliance kernel` Docker fallback must pass the same env vars and mount the same paths as the Makefile.
- [ ] `tools/installer-kernel/Makefile` - runs `docker build -t ze-installer-kernel-builder .` then `docker run --rm` with `-v $(CURDIR):/src:ro` and `-v $(CURDIR)/build:/out`. Default `ARCH=arm64`, `LINUX_VERSION=7.0.11`.
  → Constraint: default arch for `ze appliance kernel` should detect host architecture via `runtime.GOARCH`, not hardcode arm64.
- [ ] `tools/installer-initrd/Makefile` - needs busybox (statically linked), cpio, gzip. Creates rootfs with busybox applets, copies `init` script, creates cpio archive. Output: `build/initrd.img.gz`.
  → Constraint: `ze appliance initrd` local build fallback invokes `make -C tools/installer-initrd` or replicates its steps.

**Behavior to preserve:**
- `ze appliance iso` existing kernel/initrd/grub/xorriso resolution and error messages.
- `ze appliance build` existing e2fsprogs resolution.
- `ze doctor` existing check structure, output format, and JSON output.
- Existing `tools/installer-kernel/Makefile` and `tools/installer-initrd/Makefile` must continue to work standalone.
- `ze appliance iso --kernel <path>` explicit flag must override any cached artifact.

**Behavior to change:**
- `ze appliance iso` gains `--check` flag that reports readiness without building.
- `ze appliance iso` kernel path resolution gains XDG cache lookup before the `tools/` fallback.
- `ze appliance` gains `kernel` and `initrd` subcommands.
- `ze doctor` gains appliance prerequisite checks (artifact existence and tool availability).

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point

| Entry | Format | Notes |
|-------|--------|-------|
| `ze appliance kernel [--arch amd64] [--version 7.0.11]` | CLI arguments | Resolves cache, downloads, or builds kernel |
| `ze appliance initrd` | CLI arguments | Resolves cache, downloads, or builds initrd |
| `ze appliance iso --check` | CLI arguments | Runs resolution logic, reports status, exits |
| `ze doctor` | CLI arguments | Runs registered checks including appliance prerequisites |
| `ze.appliance.kernel.url` | Environment variable | Base URL for pre-built kernel downloads |
| `ze.appliance.initrd.url` | Environment variable | Base URL for pre-built initrd downloads |

### Transformation Path

**`ze appliance kernel`:**
1. Parse flags: `--arch` (default: `runtime.GOARCH`), `--version` (default: compiled-in, currently 7.0.11).
2. Resolve XDG cache path: `$XDG_CACHE_HOME/ze/installer-kernel/<version>-<arch>/Image` (fallback `~/.cache/ze/...`).
3. If cached artifact exists: report path, exit success.
4. Try download: GET `<base-url>/<version>-<arch>/Image` and `<base-url>/<version>-<arch>/Image.sha256`. Verify checksum. Write to cache path atomically (temp file + rename).
5. If download fails: check Docker is available (`docker info`). If not: error with install hint.
6. Docker build: run the same `docker build` + `docker run` as `tools/installer-kernel/Makefile`. Copy output to cache path.
7. Copy to `tools/installer-kernel/build/Image` so `ze appliance iso` finds it without extra flags.

**`ze appliance initrd`:**
1. Resolve XDG cache path: `$XDG_CACHE_HOME/ze/installer-initrd/v1/initrd.img.gz`.
2. If cached: report path, exit success.
3. Try download: GET `<base-url>/v1/initrd.img.gz` and checksum. Verify. Write to cache atomically.
4. If download fails: check for busybox-static, cpio, gzip in PATH. If missing: error with install hints.
5. Local build: invoke `make -C tools/installer-initrd` or replicate its steps. Copy output to cache.
6. Copy to `tools/installer-initrd/build/initrd.img.gz`.

**`ze appliance iso --check`:**
1. Run artifact resolution logic (kernel, initrd, grub, xorriso) without proceeding to build.
2. Print status table for each prerequisite: name, status (ready/missing), path or install hint.
3. Exit 0 if all ready, exit 1 if any missing.

**Doctor checks:**
1. At `init()` time in `doctor_checks.go`, register checks via `diagnostic.RegisterDoctorCheck()`.
2. At `ze doctor` runtime, each check runs `os.Stat` (for artifacts) or `exec.LookPath` (for binaries).
3. Emit `diagnostic.Diagnostic` with severity warning and actionable message when missing.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI args to cache resolution | Flag parsing, XDG env lookup | Unit test for cache path resolution |
| Network to cache | HTTPS GET with SHA-256 verification | Unit test with mock HTTP server |
| Docker to cache | Docker build + run, copy output | Unit test with mock external runner |
| Cache to `tools/` path | File copy | Unit test for artifact placement |
| Appliance package to diagnostic registry | `diagnostic.RegisterDoctorCheck()` from `init()` | Unit test for check registration |

### Integration Points

- `applianceCommands()` in `internal/appliance/main.go` -- add `kernel` and `initrd` entries.
- `defaultISOKernelPath()` in `internal/appliance/cmd_iso.go` -- extend to check XDG cache first.
- `resolveISOArtifact()` in `internal/appliance/cmd_iso.go` -- reuse for `--check` status reporting.
- `resolveE2FSDir()` in `internal/appliance/cmd_build.go` -- reuse for doctor e2fsprogs check.
- `diagnostic.RegisterDoctorCheck()` in `internal/core/diagnostic/doctor_registry.go` -- registration API.
- `env.MustRegister()` in `internal/core/env/` -- register download URL env vars.

### Architectural Verification

- [ ] No bypassed layers: kernel/initrd commands use the same artifacts `ze appliance iso` consumes.
- [ ] No unintended coupling: doctor checks are registered from the appliance package, not hardcoded in the doctor package.
- [ ] No duplicated functionality: `--check` reuses ISO resolution logic, does not reimplement it.
- [ ] No silent approximation: download artifacts are SHA-256 verified before caching.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze appliance kernel` | -> | `cmdKernel` handler | `TestRunDispatchesKernel` |
| `ze appliance initrd` | -> | `cmdInitrd` handler | `TestRunDispatchesInitrd` |
| `ze appliance iso --check` | -> | check-only exit in `runIso` | `TestIsoCheckReportsStatus` |
| `ze doctor` | -> | appliance doctor checks | `TestApplianceDoctorChecksRegistered` |
| `ze-setup` binary (ze_setup tag) | -> | appliance commands available | `TestZeSetupBinaryCommands` (update existing) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze appliance kernel --arch amd64` with no cached kernel and download available | Downloads kernel, verifies checksum, caches to XDG cache dir, copies to tools path, prints path. |
| AC-2 | `ze appliance kernel` with cached kernel already present | Reports cached path immediately, no download or build. |
| AC-3 | `ze appliance kernel` with download unavailable and Docker available | Falls back to Docker build, caches result, copies to tools path. |
| AC-4 | `ze appliance kernel` with download unavailable and no Docker | Fails with actionable error: install Docker or provide a download URL. |
| AC-5 | `ze appliance kernel --version 6.12.9 --arch arm64` | Downloads or builds kernel for specified version and architecture. |
| AC-6 | `ze appliance initrd` with no cached initrd and download available | Downloads initrd, verifies checksum, caches, copies to tools path. |
| AC-7 | `ze appliance initrd` with download unavailable and build tools present | Falls back to local build using busybox-static, cpio, gzip. |
| AC-8 | `ze appliance initrd` with download unavailable and busybox-static missing | Fails with actionable error listing missing tools. |
| AC-9 | `ze appliance iso --check` with all prerequisites present | Prints status table showing all items ready, exits 0. |
| AC-10 | `ze appliance iso --check` with kernel missing | Prints status table showing kernel missing with hint to run `ze appliance kernel`, exits 1. |
| AC-11 | `ze doctor` runs with appliance package linked | Doctor output includes appliance prerequisite checks (kernel, initrd, grub, xorriso, e2fsprogs). |
| AC-12 | `ze doctor` runs with kernel artifact missing | Emits warning diagnostic `doctor-appliance-kernel` with hint to run `ze appliance kernel`. |
| AC-13 | `ze doctor` runs with grub-mkstandalone missing | Emits warning diagnostic `doctor-appliance-grub` with platform-appropriate install hint. |
| AC-14 | `ze.appliance.kernel.url` environment variable set | Kernel download uses the specified URL instead of compiled-in default. |
| AC-15 | `ze.appliance.initrd.url` environment variable set | Initrd download uses the specified URL instead of compiled-in default. |
| AC-16 | Cached kernel at `$XDG_CACHE_HOME/ze/installer-kernel/<ver>-<arch>/Image` | `ze appliance iso` finds it without `--kernel` flag. |
| AC-17 | Download succeeds but checksum verification fails | Artifact is not cached, error reports checksum mismatch, temp file cleaned up. |
| AC-18 | `XDG_CACHE_HOME` is set to a custom path | Cache dir uses the custom path, not `~/.cache`. |
| AC-19 | Existing `make -C tools/installer-kernel` still works standalone | Makefile is not modified, existing build path unchanged. |
| AC-20 | `ze appliance help` or `ze appliance --help` | Help output lists `kernel` and `initrd` commands with descriptions. |
| AC-21 | `go build -tags ze_setup ./cmd/ze` | Binary includes `appliance` root command with `kernel` and `initrd` subcommands. |
| AC-22 | `go build ./cmd/ze` (no tags) | Binary does NOT include `appliance`, `kernel`, or `initrd` commands. |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunDispatchesKernel` | `internal/appliance/main_test.go` | `kernel` registered in dispatch table | |
| `TestRunDispatchesInitrd` | `internal/appliance/main_test.go` | `initrd` registered in dispatch table | |
| `TestKernelResolvesCache` | `internal/appliance/cmd_kernel_test.go` | Returns cached artifact when present | |
| `TestKernelDownloadsAndCaches` | `internal/appliance/cmd_kernel_test.go` | Downloads, verifies checksum, writes to cache | |
| `TestKernelDownloadChecksumMismatch` | `internal/appliance/cmd_kernel_test.go` | Rejects artifact with bad checksum | |
| `TestKernelFallsBackToDocker` | `internal/appliance/cmd_kernel_test.go` | Falls back to Docker when download fails | |
| `TestKernelFailsWithoutDocker` | `internal/appliance/cmd_kernel_test.go` | Clear error when both download and Docker unavailable | |
| `TestKernelArchFlag` | `internal/appliance/cmd_kernel_test.go` | `--arch` selects architecture, validates value | |
| `TestKernelVersionFlag` | `internal/appliance/cmd_kernel_test.go` | `--version` selects kernel version | |
| `TestKernelCopiesToToolsPath` | `internal/appliance/cmd_kernel_test.go` | Artifact placed at `tools/installer-kernel/build/Image` | |
| `TestKernelEnvURL` | `internal/appliance/cmd_kernel_test.go` | `ze.appliance.kernel.url` overrides default URL | |
| `TestInitrdResolvesCache` | `internal/appliance/cmd_initrd_test.go` | Returns cached artifact when present | |
| `TestInitrdDownloadsAndCaches` | `internal/appliance/cmd_initrd_test.go` | Downloads, verifies checksum, writes to cache | |
| `TestInitrdFallsBackToLocalBuild` | `internal/appliance/cmd_initrd_test.go` | Falls back to make when download fails | |
| `TestInitrdFailsWithoutBuildTools` | `internal/appliance/cmd_initrd_test.go` | Clear error listing missing tools | |
| `TestInitrdEnvURL` | `internal/appliance/cmd_initrd_test.go` | `ze.appliance.initrd.url` overrides default URL | |
| `TestIsoCheckAllReady` | `internal/appliance/cmd_iso_test.go` | `--check` reports all ready, exits 0 | |
| `TestIsoCheckMissing` | `internal/appliance/cmd_iso_test.go` | `--check` reports missing items, exits 1 | |
| `TestResolveCacheDir` | `internal/appliance/cache_test.go` | XDG_CACHE_HOME respected, fallback to ~/.cache/ze | |
| `TestResolveCacheDirCustom` | `internal/appliance/cache_test.go` | Custom XDG_CACHE_HOME used | |
| `TestApplianceDoctorChecksRegistered` | `internal/appliance/doctor_checks_test.go` | All expected checks appear in `diagnostic.DoctorCheckNames()` | |
| `TestDoctorKernelPresent` | `internal/appliance/doctor_checks_test.go` | No diagnostic when kernel artifact exists | |
| `TestDoctorKernelMissing` | `internal/appliance/doctor_checks_test.go` | Warning diagnostic with hint when kernel absent | |
| `TestDoctorGrubPresent` | `internal/appliance/doctor_checks_test.go` | No diagnostic when grub-mkstandalone found | |
| `TestDoctorGrubMissing` | `internal/appliance/doctor_checks_test.go` | Warning diagnostic with install hint | |
| `TestDoctorXorrisoMissing` | `internal/appliance/doctor_checks_test.go` | Warning diagnostic with install hint | |
| `TestDoctorE2fsprogsMissing` | `internal/appliance/doctor_checks_test.go` | Warning diagnostic with install hint | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `--arch` | amd64, arm64 | arm64 | "x86" (rejected) | "riscv64" (rejected) |
| `--version` | Semantic version string | "7.0.11" | "" (uses default) | N/A (any version string accepted, server/build validates) |
| Download content length | >0 bytes | 1 byte | 0 bytes (rejected) | N/A |
| Checksum length | 64 hex chars | 64 chars | 63 chars (rejected) | 65 chars (rejected) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Doctor includes appliance checks | Existing `ze doctor` test suite | `ze doctor --json` output includes appliance codes when prerequisites missing | |

### Interop Tests

Not applicable. This spec does not add or change wire protocol behavior.

### Future (if deferring any tests)

No tests deferred.

## Files to Modify

- `internal/appliance/main.go` - add `kernel` and `initrd` to `applianceCommands()` and `cmdKernel`/`cmdInitrd` handler vars with `stub` default.
- `internal/appliance/cmd_iso.go` - add `--check` flag to `runIso`, extend `defaultISOKernelPath()` to check XDG cache.
- `cmd/ze/setup_features_setup.go` - add `_ "codeberg.org/thomas-mangin/ze/internal/appliance"` import so `ze-setup` binary includes all appliance commands.
- `cmd/ze/build_tag_setup_test.go` - update to expect `appliance` root command present in `ze_setup` build.
- `docs/guide/appliance.md` - document `ze appliance kernel`, `ze appliance initrd`, `ze appliance iso --check`.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | - |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | Yes | `internal/appliance/main.go` (kernel, initrd entries) |
| CLI grammar (action before identifier) | Yes | `kernel` and `initrd` are closed keywords, `--arch`/`--version` are flags |
| Editor autocomplete | No | Offline command |
| Functional test for new RPC/API | No | No RPC/API |
| Pipe completeness | No | Operational output, not pipeable data |
| Env var registration | Yes | `ze.appliance.kernel.url`, `ze.appliance.initrd.url` via `env.MustRegister()` |
| Doctor check for runtime dependencies | Yes | Artifact existence (kernel, initrd) and build tools (grub, xorriso, e2fsprogs) |
| Prometheus counters/metrics | No | Build-time tooling |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature? | Yes | `docs/guide/appliance.md` -- new commands section |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/appliance.md` -- command table |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | `docs/guide/appliance.md` -- command list |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | Check `docs/` for anchors naming `cmd_iso.go`, `main.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/appliance.md` examples section |

## Files to Create

- `internal/appliance/cmd_kernel.go` - kernel download/build command.
- `internal/appliance/cmd_kernel_test.go` - kernel command tests.
- `internal/appliance/cmd_initrd.go` - initrd download/build command.
- `internal/appliance/cmd_initrd_test.go` - initrd command tests.
- `internal/appliance/cache.go` - XDG cache resolution and download/verify helper.
- `internal/appliance/cache_test.go` - cache resolution tests.
- `internal/appliance/doctor_checks.go` - doctor check registration for appliance prerequisites.
- `internal/appliance/doctor_checks_test.go` - doctor check tests.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | Targeted tests, lint, verify |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run affected tests |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run affected tests |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** -- register kernel and initrd commands, add --check to iso
   - Tests: `TestRunDispatchesKernel`, `TestRunDispatchesInitrd`, `TestIsoCheckReportsStatus`
   - Files: `internal/appliance/main.go`, `cmd_kernel.go`, `cmd_initrd.go`, `cmd_iso.go`
   - Verify: dispatch reaches handlers; `--check` flag parsed; all three are stubs returning error.

2. **Phase: Cache resolution** -- XDG cache dir lookup and artifact path helpers
   - Tests: `TestResolveCacheDir`, `TestResolveCacheDirCustom`
   - Files: `internal/appliance/cache.go`, `cache_test.go`
   - Verify: cache paths respect XDG_CACHE_HOME, fallback to ~/.cache/ze.

3. **Phase: Download and verify** -- HTTP download with SHA-256 checksum verification
   - Tests: `TestKernelDownloadsAndCaches`, `TestKernelDownloadChecksumMismatch`, `TestKernelEnvURL`, `TestInitrdDownloadsAndCaches`, `TestInitrdEnvURL`
   - Files: `cache.go` (download helper), `cmd_kernel.go`, `cmd_initrd.go`
   - Verify: downloads to temp file, verifies checksum, atomically moves to cache path.

4. **Phase: Kernel Docker fallback** -- Docker build when download fails
   - Tests: `TestKernelFallsBackToDocker`, `TestKernelFailsWithoutDocker`, `TestKernelArchFlag`, `TestKernelVersionFlag`
   - Files: `cmd_kernel.go`, `cmd_kernel_test.go`
   - Verify: Docker invocation matches Makefile pattern; clear error without Docker.

5. **Phase: Initrd local build fallback** -- local build when download fails
   - Tests: `TestInitrdFallsBackToLocalBuild`, `TestInitrdFailsWithoutBuildTools`
   - Files: `cmd_initrd.go`, `cmd_initrd_test.go`
   - Verify: make invocation or equivalent; clear error listing missing tools.

6. **Phase: Artifact placement** -- copy to tools/ paths for ze appliance iso
   - Tests: `TestKernelCopiesToToolsPath`, `TestKernelResolvesCache`, `TestInitrdResolvesCache`
   - Files: `cmd_kernel.go`, `cmd_initrd.go`
   - Verify: `ze appliance iso` finds the artifacts without `--kernel`/`--initrd` flags.

7. **Phase: ISO --check** -- readiness report
   - Tests: `TestIsoCheckAllReady`, `TestIsoCheckMissing`
   - Files: `cmd_iso.go`
   - Verify: status table printed, exit code reflects readiness.

8. **Phase: Doctor checks** -- register appliance prerequisite checks
   - Tests: `TestApplianceDoctorChecksRegistered`, `TestDoctorKernelPresent`, `TestDoctorKernelMissing`, `TestDoctorGrubPresent`, `TestDoctorGrubMissing`, `TestDoctorXorrisoMissing`, `TestDoctorE2fsprogsMissing`
   - Files: `doctor_checks.go`, `doctor_checks_test.go`
   - Verify: checks appear in `ze doctor` output; correct severity and hints.

9. **Phase: Documentation** -- update guides
   - Files: `docs/guide/appliance.md`
   - Verify: command table, workflow section, prerequisites section updated.

10. **Full verification** -- lint, targeted tests, verify gate.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-20 has code and tests. |
| Correctness | Downloaded artifacts match checksum; cache paths follow XDG spec. |
| Naming | Env vars use `ze.appliance.*` pattern; doctor codes use `doctor-appliance-*`; JSON keys kebab-case. |
| Data flow | Download -> verify -> cache -> copy to tools/; no partial files left on failure. |
| CLI grammar | `kernel` and `initrd` are closed keywords before flags. |
| Doctor checks | Registered via `diagnostic.RegisterDoctorCheck()`, not hardcoded in doctor package. |
| Rule: no-layering | Reuses existing resolution logic in `cmd_iso.go`, does not duplicate. |
| Rule: plugin-self-containment | All checks owned by appliance package; removing appliance removes checks. |
| Build tags | `ze_setup` includes appliance commands; no-tag build excludes them; `ze_appliance` still works. |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `kernel` command registered | `grep "kernel" internal/appliance/main.go` |
| `initrd` command registered | `grep "initrd" internal/appliance/main.go` |
| `--check` flag on iso | `grep "check" internal/appliance/cmd_iso.go` |
| Doctor checks registered | `grep "RegisterDoctorCheck" internal/appliance/doctor_checks.go` |
| XDG cache resolution | `grep "XDG_CACHE_HOME" internal/appliance/cache.go` |
| Env vars registered | `grep "ze.appliance.kernel.url" internal/appliance/` |
| Download with checksum | `grep "sha256" internal/appliance/cache.go` |
| Tests pass | `go test ./internal/appliance/ -run "Kernel\|Initrd\|IsoCheck\|Doctor"` |
| Docs updated | `grep "ze appliance kernel" docs/guide/appliance.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Download URL validation | URL must be HTTPS; reject plain HTTP unless overridden for local testing. |
| Checksum enforcement | Downloaded artifact must match SHA-256 checksum; no `--skip-checksum` override. |
| Temp file cleanup | Partial downloads cleaned up on failure; no stale temp files left. |
| Path traversal | Downloaded file names validated against expected patterns; no directory traversal from server response. |
| Docker command injection | Docker args built as string slices, not shell strings; no user input interpolated into commands. |
| Cache dir permissions | Cache directory created with 0755 (standard for cache, readable by user). |
| Env var content | URL from env var validated (scheme check) before HTTP request. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Command dispatch missing | Phase 1 wiring |
| Cache path wrong on platform | Phase 2 cache resolution |
| Download fails silently | Phase 3 download/verify |
| Docker build produces wrong arch | Phase 4 Docker fallback |
| Initrd build missing applets | Phase 5 initrd local build |
| `ze appliance iso` doesn't find cached kernel | Phase 6 artifact placement |
| `--check` output incomplete | Phase 7 ISO check |
| Doctor checks not appearing | Phase 8 doctor registration |
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

- Doctor checks should test for artifact existence (the result), not build-tool presence (one of several paths to the result). An operator who downloads pre-built artifacts should see a clean doctor output even without Docker installed.
- XDG_CACHE_HOME is the correct location per freedesktop.org spec: cached artifacts are expendable, re-downloadable data that does not belong in XDG_CONFIG_HOME alongside appliance secrets.
- The three-tier resolution (cache, download, build) makes the commands useful in different environments: CI with pre-seeded cache, development with internet access, air-gapped build hosts with Docker.

## Core Insight

The ISO prerequisite pipeline has three tiers: cache hit (instant), download (fast, no local tools), and local build (self-contained but needs Docker or build tools). Each command tries them in order and stops at the first success. The doctor checks only care whether the artifact exists, not how it got there.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Download-first, build-as-fallback | Build-first with optional download; download-only | Download is fastest and needs no local tools; fallback to build handles air-gapped and custom-version scenarios. |
| XDG_CACHE_HOME for artifacts | tools/ directory; ~/.config/ze/cache; /var/cache/ze | XDG spec designates ~/.cache for expendable cached data; tools/ is source tree, not user cache; /var/cache needs root. |
| Doctor checks for artifact existence, not tool presence | Check for Docker, busybox-static, etc. | Operator may have downloaded artifacts; checking tools would false-alarm. Tools are checked at build time by the commands themselves. |
| Env vars for download URLs with compiled-in defaults | Config file; CLI flags only; hardcoded only | Env vars follow existing `ze.appliance.*` pattern; compiled-in defaults work out of the box; CI/air-gap can override. |
| Copy artifact to tools/ path after caching | Symlink; modify ze appliance iso resolution only | Copy is simplest and works across filesystems; symlinks break when cache is cleaned; modifying only ISO resolution would leave PXE make targets broken. |
| `--check` on existing `ze appliance iso` | New `ze appliance check` command | `--check` is discoverable from the command it validates; no new dispatch entry needed for a dry-run of an existing command. |
| Default arch from `runtime.GOARCH` | Hardcode arm64 as Makefile does; require explicit flag | Host arch is the most common target; Makefile default is arm64 because it was written on Apple Silicon, not because arm64 is universally preferred. |

## Known Limitations

- Pre-built artifact hosting must exist at the configured URL for download to succeed. Until artifacts are published, the download path will fall back to local build.
- The kernel Docker build requires the host Docker daemon architecture to match the target. Building arm64 on amd64 requires `docker buildx` or similar, which is not handled.
- Cache invalidation is manual: the operator must run `ze appliance kernel --version <new>` to fetch a new version. No automatic cache expiry or staleness check.
- The initrd version scheme (`v1`) is simple; if the initrd format changes incompatibly, the version must be bumped manually in the code.

## RFC Documentation

No RFC comments required. This is build tooling, not protocol implementation.

## Implementation Summary

### What Was Implemented

Not implemented yet.

### Bugs Found/Fixed

None yet.

### Documentation Updates

Pending implementation.

### Deviations from Plan

None yet.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `ze appliance kernel` command | Planned | `cmd_kernel.go` | |
| `ze appliance initrd` command | Planned | `cmd_initrd.go` | |
| `ze appliance iso --check` | Planned | `cmd_iso.go` | |
| Doctor checks for prerequisites | Planned | `doctor_checks.go` | |
| XDG cache for artifacts | Planned | `cache.go` | |
| Env-configurable download URLs | Planned | `cmd_kernel.go`, `cmd_initrd.go` | |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 through AC-20 | Planned | Unit tests | |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All tests | Planned | `internal/appliance/*_test.go` | |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `cmd_kernel.go` | Planned create | |
| `cmd_kernel_test.go` | Planned create | |
| `cmd_initrd.go` | Planned create | |
| `cmd_initrd_test.go` | Planned create | |
| `cache.go` | Planned create | |
| `cache_test.go` | Planned create | |
| `doctor_checks.go` | Planned create | |
| `doctor_checks_test.go` | Planned create | |

### Audit Summary

- **Total items:** Pending implementation
- **Done:** 0
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Operator can obtain kernel without manual Makefile | Unit test | `TestKernelDownloadsAndCaches`, `TestKernelFallsBackToDocker` |
| Operator can obtain initrd without manual Makefile | Unit test | `TestInitrdDownloadsAndCaches`, `TestInitrdFallsBackToLocalBuild` |
| Operator can check ISO readiness | Unit test | `TestIsoCheckAllReady`, `TestIsoCheckMissing` |
| ze doctor reports missing prerequisites | Unit test | `TestDoctorKernelMissing`, `TestDoctorGrubMissing` |
| Artifacts cached per XDG spec | Unit test | `TestResolveCacheDir`, `TestResolveCacheDirCustom` |
| Download URLs configurable via environment | Unit test | `TestKernelEnvURL`, `TestInitrdEnvURL` |

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| | | Pending implementation review | | |

### Fixes applied

Pending implementation.

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| | | Pending implementation review | | |

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above or explicitly none

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| Pending implementation | Pending | Pending |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| Pending implementation | Pending | Pending |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Pending implementation | Pending | Pending |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Pending implementation | Pending | Pending |

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-22 all demonstrated
- [ ] Wiring Test table complete, every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean, Review Gate section filled, 0 BLOCKER, 0 ISSUE
- [ ] `make ze-test` passes
- [ ] Feature code integrated under `internal/appliance/`
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass, defer with user approval)

- [ ] RFC constraint comments added or documented N/A
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
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] Boundary tests for arch and version inputs
- [ ] Functional tests for doctor integration
- [ ] Interop tests documented N/A

### Completion (BLOCKING, before ANY commit)

- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-install-10-iso-prerequisites.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-install-10-iso-prerequisites.md`
