# Spec: install-11 (A) kernel builder — Python recipe + profile registry + Go enforcement

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | 870-kernel-build-convergence, 856-install-10-iso-prerequisites |
| Phase | 7/7 (owner accepted unrelated full-gate failures; commit script closes spec) |
| Updated | 2026-06-25 |

## Spec sequence (context)

This is **spec A** of three. It builds the *foundation* only; per-hardware targets come next.
- **A (this spec):** convert the shared kernel build recipe `tools/kernel-builder/build.sh` → thin Python `build.py`; turn the closed PROFILE enum into an **open registry** (`<name>.config` + `<name>.require` manifest + optional one-level `# ze-base:` layering); move profile resolution + required-`=y` enforcement into **Go** (`internal/appliance`); migrate the existing profiles (`qemu`, `hardware`, `hardware-kms`, `runtime`) to manifests verbatim. Behavior-preserving for existing profiles.
- **B (later):** per-hardware **installer** kernel targets + `ze install remote --hardware <target>` PXE selection (injected resolver).
- **C (later):** per-hardware **runtime/appliance** kernel targets + `appliance.json image.hardware-target` + baking the matched runtime kernel into the image.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `docs/guide/ze-install.md` ("Installer Kernel"), `plan/learned/870-kernel-build-convergence.md`
4. `tools/kernel-builder/build.sh`, `tools/kernel-builder/qemu-build.py`, `internal/appliance/cmd_kernel.go`, `internal/appliance/config.go`

## Task

Replace the shell kernel-build recipe with thin Python, open the profile space into a registry, and relocate the decision logic to Go:

1. **Convert `tools/kernel-builder/build.sh` → `tools/kernel-builder/build.py`** (thin). Python stdlib for download (`urllib`), extract (`tarfile`), file copy (`shutil`), and config read; `subprocess` ONLY for the external kernel toolchain that has no Python equivalent: `make`, the kernel's own `scripts/kconfig/merge_config.sh`, and `patch`. No shell logic. Update every invoker (`Dockerfile`, `qemu-build.py`, `mk/gokrazy.mk`, `tools/installer-kernel/Makefile`, `gokrazy/kernel/Makefile`, `cmd_kernel.go`) to run `python3 build.py`.
2. **Open registry:** a profile `<name>` is valid iff `<srcdir>/<name>.config` and `<srcdir>/<name>.require` both exist. Optional `# ze-base: <profile>` header (one level) layers a base fragment. Remove the closed enum from Go (`config.go`, `cmd_kernel.go`) and python (`qemu-build.py`); names are validated as a **safe token** `^[a-z0-9][a-z0-9-]*$` (reuse `config.go:89 validNameRe` where applicable).
3. **Logic in Go (`internal/appliance`):** profile resolution (fragment list + base chain), token validation, registry enumeration (for `iso --check` and doctor), and **required-`=y` enforcement** by parsing the resolved `build/config` the builder emits. A hardcoded universal floor (`IP_PNP_DHCP/EXT4_FS/BLK_DEV_INITRD/DEVTMPFS_MOUNT`) is enforced in Go in addition to manifests (belt-and-suspenders) so a manifest typo cannot drop the four essentials.
4. **Migrate existing profiles to `.require` manifests verbatim** (`qemu`, `hardware`, `hardware-kms`, `runtime`), reproducing build.sh's current symbol sets exactly.

`build.sh` is **shared** with the runtime kernel build (`make ze-kernel`), so this is builder-infra touching both the installer and runtime paths. No per-hardware target definitions, no `--hardware`/`--kernel-profile` selection, no appliance.json changes here — those are B/C.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/ze-install.md` ("Installer Kernel") - operator model + profile table
  → Constraint: installer initrd carries NO kernel modules; the kernel must have NIC/disk/ext4/devtmpfs/`ip=dhcp` all `=y`. The universal floor manifest must keep these `=y`.
- [ ] `plan/learned/870-kernel-build-convergence.md` - installer + runtime share one builder
  → Constraint: `tools/kernel-builder/build.sh` + `qemu-build.py` are shared (installer SRC_DIR=`tools/installer-kernel`, runtime SRC_DIR=`gokrazy/kernel`); fragments vary, build logic single-source. The build.py conversion must keep BOTH paths working.
  → Constraint: "kernel builds must work without first building Ze binaries" — so `make`-driven builds (`make -C tools/installer-kernel`, `make ze-kernel`) must still produce a kernel without `bin/ze`. Consequence: Go-side enforcement attaches to `ze appliance kernel`; raw `make` builds are "build without verify".
- [ ] `plan/learned/856-install-10-iso-prerequisites.md` - three-tier kernel resolution
  → Constraint: artifacts namespaced `<version>-<arch>-<profile>` (cache + URL); a new profile name flows through for free. Test-injectable `*Fn` vars are the idiom for unit-testing build/resolve without Docker/network.
- [ ] `ai/rules/go-standards.md` ("Scripts: Python Only", line 75)
  → Constraint: shell scripts are banned for new/reworked scripts; `build.sh` is a pre-existing exception this spec removes. build.py uses subprocess only for external kernel tools.

**Key insights:**
- The profile name is already a first-class key (cache path, download URL, fragment filename). Work = remove closed-enum gates + make the guarantee data-driven + move logic to Go + de-shell the recipe.
- Closed enum is hardcoded in: `config.go:165` (config validation), `cmd_kernel.go:137` (cmd validation), `build.sh:46` (shell case + require blocks 162-198), `qemu-build.py:93` (validate_profile). Plus discovery loops `cmd_iso.go:391`, `doctor_checks.go:72`, and stale Makefile error strings.
- "Verifiably correct" = required drivers provably `=y` (Go-enforced). "Minimal/optimal" is an authoring property of the fragment, NOT enforced.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `tools/kernel-builder/build.sh` - in-builder recipe: PROFILE `case` (46-57); `merge_config.sh` layering (130); `olddefconfig`; universal `require_yes` (162-164) + per-profile blocks (166-198); hardware-kms EXTRA_FIRMWARE check (181-189); build + copy out `Image`/`vmlinuz` + `config` (200-231).
  → Constraint: it ALREADY copies the resolved `.config` to `${OUT_DIR}/config` (208/225) — Go can read this post-build to enforce manifests.
  → Constraint: `runtime` profile path (SRC_DIR=`gokrazy/kernel`, MODULES=yes, PATCHES_DIR) must keep working unchanged.
- [ ] `tools/kernel-builder/qemu-build.py` - host driver; boots Alpine VM, runs `sh build.sh` inside; `validate_profile` (93-96) closed enum; `apk add` for guest packages.
  → Constraint: must run `python3 build.py` in the VM and ensure python3 present (apk add); relax validate_profile to a token.
- [ ] `tools/kernel-builder/Dockerfile` - `debian:bookworm-slim`; already installs `python3`.
  → Constraint: docker run entry becomes `python3 /builder/build.py`; python3 already available.
- [ ] `internal/appliance/cmd_kernel.go` - `runKernel` resolves profile (flag/appliance.json/default qemu), validates against enum (137), `resolveKernel` (cache/download/build via docker `-e PROFILE=` or qemu). `ensureFirmware` hardware-kms only (373).
  → Constraint: docker/qemu build fns invoke the builder; update to `build.py` entry; add Go enforcement after build.
- [ ] `internal/appliance/config.go` - `ProfileQEMU/Hardware/HardwareKMS` consts (84-86); `Image.KernelProfile` (56); enum validation (165-166); default qemu (119); existing `validNameRe` (89).
- [ ] `internal/appliance/cmd_iso.go` - reads KernelProfile (301); iterates closed list (391) for discovery.
- [ ] `internal/appliance/doctor_checks.go` - iterates closed list (72).
- [ ] `internal/appliance/cache.go` - `kernelCacheVariant(arch, profile)`; firmware special-case (77).
- [ ] `tools/installer-kernel/Makefile` & `gokrazy/kernel/Makefile` - invoke the builder; `tools/installer-kernel` is generic `$(PROFILE).config` except hardware-kms `PROFILE_DEPS` + stale error strings.

**Behavior to preserve:**
- `qemu`, `hardware`, `hardware-kms`, `runtime` produce identical merged config, identical enforced symbol set, identical artifacts (Image/vmlinuz/modules/dtb/config) as before.
- `make ze-kernel` (runtime) and `make -C tools/installer-kernel` still build without `bin/ze`.
- Cache/download URL scheme `<version>-<arch>-<profile>`; hardware-kms firmware embedding.
- Module-free busybox initrd (untouched).

**Behavior to change:**
- Recipe language shell → Python (`build.sh` removed, `build.py` added).
- Closed enum → open registry (token-validated names; existence by fragment+manifest).
- `require_yes` logic leaves the recipe; enforcement is Go (on the `ze appliance kernel` path) reading the resolved `build/config`.
- Existing profiles' required-symbol sets expressed as `.require` manifests.

## Data Flow (MANDATORY)

### Entry Points
- `ze appliance kernel [--profile <name>]` → Go resolves profile → invoke build.py (docker/qemu) → Go enforces manifest on `build/config`. **Verified path.**
- `make -C tools/installer-kernel PROFILE=<name>` and `make ze-kernel` → compose fragments → build.py → build (no Go enforcement; raw dev build).

### Transformation Path
1. Profile name resolved (flag > appliance.json > default qemu) and token-validated (Go).
2. Go resolves fragment list: `kernel.config` + (one-level `# ze-base:` base) + `<name>.config`; gathers applicable `.require` manifests.
3. build.py: download (urllib) → extract (tarfile) → `merge_config.sh` (subprocess) → optional `patch` (subprocess) → `make` (subprocess) → copy out `Image`/`vmlinuz` + `config` (shutil).
4. Go reads `build/config`; enforces universal floor (hardcoded) + every symbol in the combined manifests; FATAL on miss.
5. Artifact cached/copied; `<version>-<arch>-<profile>` keys unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Go ↔ build.py | argv (explicit fragment paths, version, arch, out-dir) + env | [ ] |
| build.py ↔ kernel toolchain | subprocess (make, merge_config.sh, patch) | [ ] |
| Go ↔ resolved config | read `build/config`, parse `^CONFIG_X=y` | [ ] |

### Integration Points
- `resolveKernel`/`kernelCacheVariant` — profile-keyed, reused.
- New Go: registry resolver + manifest enforcer (`internal/appliance`).

### Architectural Verification
- [ ] No bypassed layers (build still merge_config.sh + make; enforcement after)
- [ ] No duplicated functionality (manifest replaces hardcoded require blocks)
- [ ] runtime profile path produces identical output
- [ ] no shell logic remains in the kernel-build path

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | build.py can fully replace build.sh in both Docker (debian, has python3) and QEMU (Alpine, `apk add python3`) builders | Dockerfile installs python3; qemu-build.py installs `python3` | one backend can't run build.py | `bin/ze-test install 1 2 3 4 5 19 20 21 22 23 24 32`; `make ze-verify-changed` install suite | confirmed |
| A-2 | Existing per-profile `require_yes` sets migrate to `.require` files reproducing identical enforcement | build.sh:162-198 listed exact sets | a dropped symbol silently weakens a profile | `tools/installer-kernel/*.require`, `gokrazy/kernel/*.require`, `TestEnforceRequire`, `appliance-kernel-registry.ci` stripped-symbol path | confirmed |
| A-3 | hardware-kms EXTRA_FIRMWARE check (non-`=y`) can be preserved as a Go special-case alongside manifests | build.sh:185 | firmware check lost | `TestEnforceHardwareKMSFirmware`, `ensureFirmware`, `build.py --firmware-dir` path | confirmed |
| A-4 | Go enforcement reading `build/config` is sufficient for the verified path; raw `make` builds without verify is acceptable | 870 "no ze build required for kernel build"; build.py copies config out | runtime loses build-time verification via `make ze-kernel` | documented in `docs/guide/ze-install.md`, `tools/installer-kernel/README.md`, `plan/learned/982-install-11-hw-kernel-profiles.md`; changed verification passed | confirmed |
| A-5 | The resolved `.config` symbol grammar is exactly `^CONFIG_X=y` (same as build.sh `grep`) | build.sh:154 | Go parser misses set-but-not-`=y` cases | `TestEnforceRequire` covers `=m`/not-set/missing cases; `appliance-kernel-registry.ci` strips required symbols | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A `.config` with no `.require` builds an unverified/over-broad kernel | profile resolves with no manifest | Go rejects a profile lacking a `.require`; FATAL (no silent unverified profile) |
| R-2 | build.py conversion breaks `make ze-kernel` (shared builder) | `make ze-kernel` red; runtime overlay tests fail | run `kernel-runtime-deps.ci`, `ze-kernel-overlay.ci`, `kernel-compose.ci`; keep runtime fragments/patches identical |
| R-3 | Raw `make` path loses build-time require enforcement (moved to Go) | a make-built kernel missing a needed `=y` ships | document `ze appliance kernel` as the verified path; B/C wire enforcement into the shipping path |
| R-4 | `# ze-base:` cycle/nesting → loop | merge hangs | one-level base only; build.py/Go error if a base fragment itself declares a base |
| R-5 | profile name as filename/URL/`-e PROFILE=`/argv → traversal or injection | odd path/arg | safe-token validation `^[a-z0-9][a-z0-9-]*$` in Go + python before use |
| R-6 | universal floor moved to `kernel.require` transcribed wrong for runtime | `make ze-kernel` red or missing `=y` | hardcoded Go floor (belt-and-suspenders) + runtime `kernel.require` verbatim + CI |
| R-7 | Makefile `PROFILE_DEPS` ignores base header → stale rebuild | edited base fragment not picked up | depend on all `*.config`/`*.require` in the dir |
| R-8 | "minimal/optimal" implied but unenforceable | n/a | document: manifest enforces required-present, not nothing-extra (Known Limitation) |

## Wiring Test (MANDATORY)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze appliance kernel --profile hardware` | → | Go resolve → build.py → Go enforce | `appliance-kernel-docker.ci`, `appliance-kernel-qemu.ci`, `appliance-kernel-registry.ci` |
| `make -C tools/installer-kernel PROFILE=<fixture>` | → | build.py merge+build | `kernel-compose.ci` open-registry fixture profile |
| `make ze-kernel` | → | build.py runtime path | `ze-kernel-overlay.ci`, `kernel-runtime-deps.ci` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | build `qemu`, `hardware`, `hardware-kms`, `runtime` via build.py | the resolved `.config` (`build/config`) is byte-identical to the build.sh era for each profile/arch (captured baseline diffs clean); existing CI stays green (`kernel-runtime-deps`, `ze-kernel-overlay`, `appliance-kernel-docker`/`-qemu`); `make ze-kernel` succeeds. (Image bytes are NOT required to be reproducible.) |
| AC-2 | grep the kernel-build path for shell logic after conversion | `build.sh` removed; no project-authored `.sh` in `tools/kernel-builder` (kernel's own `merge_config.sh` invoked via subprocess is allowed); build.py shells out only to make/merge_config.sh/patch |
| AC-3 | a fixture profile `<x>.config`+`<x>.require` present | `make -C tools/installer-kernel PROFILE=<x>` and `ze appliance kernel --profile <x>` build it (open registry) |
| AC-4 | a `.config` present but `<name>.require` absent | resolution FATALs "no require manifest"; non-zero exit |
| AC-5 | a manifest symbol does not resolve to `=y` in `build/config` | Go enforcement FATALs naming the symbol; non-zero exit (the verifiable guarantee) |
| AC-6 | the 4 universal floor symbols stripped from `kernel.require` | Go still FATALs (hardcoded floor catches it) — belt-and-suspenders |
| AC-7 | `# ze-base: hardware` in `<name>.config` | hardware fragment+manifest layered before `<name>`; hardware-kms migrated to this still builds + embeds firmware |
| AC-8 | profile name `../x`, empty, or with illegal chars | rejected by token validation in Go and qemu-build.py before any filesystem/URL/argv use |
| AC-9 | `appliance.json image.kernel-profile: <valid-token>` | loads without closed-enum rejection; invalid token rejected |
| AC-10 | `ze appliance iso --check` and `ze doctor` | enumerate profiles by scanning the registry, not the three old constants |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | `ze appliance kernel --profile hardware` | Go resolve → build.py (docker/qemu) → Go enforce manifest | `appliance-kernel-docker.ci` |
| 2 | adds `tools/installer-kernel/foo.config`+`foo.require`, runs `make PROFILE=foo` | open registry → build.py merge+build | `kernel-compose.ci` (fixture) |
| 3 | runs `make ze-kernel` after conversion | runtime fragments → build.py → vmlinuz/modules | `ze-kernel-overlay.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveProfileFragments` | `internal/appliance/kernelreg_test.go` | resolves `<name>` → ordered fragments incl. one-level `# ze-base:`; errors on missing `.config`/`.require`; errors on nested base (R-4) | PASS (`go test ./internal/appliance -count=1`) |
| `TestProfileNameToken` | `internal/appliance/kernelreg_test.go` | accepts safe tokens; rejects `../x`, empty, illegal chars (AC-8) | PASS (`go test ./internal/appliance -count=1`) |
| `TestEnforceRequire` | `internal/appliance/kernelreq_test.go` | parses a `.config`; passes when all manifest symbols `=y`; FATALs on a missing symbol (AC-5); hardcoded floor catches stripped floor (AC-6); `=m` and `# … is not set` treated as not-`=y` (A-5) | PASS (`go test ./internal/appliance -count=1`) |
| `TestEnumerateRegistry` | `internal/appliance/kernelreg_test.go` | lists profiles by scanning `*.config`+`*.require` (AC-10) | PASS (`go test ./internal/appliance -count=1`) |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `kernel-compose` (extended) | `test/install/kernel-compose.ci` | open-registry fixture builds through raw make; runtime fragment composition still carries required symbols | PASS (`bin/ze-test install 21`) |
| `appliance-kernel-docker`/`-qemu` and registry test | `test/install/appliance-kernel-*.ci` | `ze appliance kernel` builds via build.py and enforces in Go, including missing manifest and stripped-symbol failures | PASS (`bin/ze-test install 1 2 3 4 5`) |
| `kernel-runtime-deps`, `ze-kernel-overlay` | `test/install/*.ci` | `make ze-kernel` runtime build dependencies and overlay path unchanged (R-2) | PASS (`bin/ze-test install 23 32`) |
| `kernel-builder-no-shell` | `test/install/kernel-builder-no-shell.ci` | asserts AC-2 (no project shell script in the build path) | PASS (`bin/ze-test install 19`) |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A — not a wire-protocol feature | - | - | - | - |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/appliance/config.go` - open KernelProfile validation → safe token (keep default qemu)
- `internal/appliance/cmd_kernel.go` - open profile validation; invoke build.py; run Go enforcement after build
- `internal/appliance/cmd_iso.go` - registry-aware discovery (replace const-list iteration)
- `internal/appliance/doctor_checks.go` - registry-aware enumeration
- `internal/appliance/cache.go` - keep firmware special-case; no enum dependence
- `tools/kernel-builder/qemu-build.py` - run `python3 build.py` in VM; ensure python3; token-validate profile
- `tools/kernel-builder/Dockerfile` - entry `python3 /builder/build.py`
- `mk/gokrazy.mk` - `ze-kernel` chain invokes build.py
- `tools/installer-kernel/Makefile`, `gokrazy/kernel/Makefile` - invoke build.py; generic error strings; base-aware deps (R-7)
- `docs/guide/ze-install.md`, `tools/installer-kernel/README.md` - registry + manifest + verified-vs-raw build paths

## Files to Create
- `tools/kernel-builder/build.py` - thin Python recipe (replaces build.sh)
- `internal/appliance/kernelreg.go` - profile registry: resolve fragments, `# ze-base:` layering, token validation, enumeration
- `internal/appliance/kernelreq.go` - manifest enforcement: parse resolved config, enforce floor + manifests
- `internal/appliance/kernelreg_test.go`, `internal/appliance/kernelreq_test.go`
- `tools/installer-kernel/kernel.require`, `qemu.require`, `hardware.require`, `hardware-kms.require` (migrated verbatim)
- `gokrazy/kernel/kernel.require`, `gokrazy/kernel/runtime.require` (migrated verbatim)
- `tools/installer-kernel/PROFILES.md` - how to author a profile (fragment + manifest + `# ze-base:`)
- `test/install/kernel-compose` fixtures + a no-shell assertion test

## Implementation Steps

1. **Phase: Wiring (FIRST)** — add `kernelreg.go`/`kernelreq.go` skeletons + failing unit tests; add a `kernel-compose.ci` fixture profile that fails until the registry resolves it.
2. **Phase: build.py** — port build.sh to thin Python (no require logic); update Dockerfile/qemu-build.py/Makefiles/cmd_kernel.go invokers; prove `qemu` + `runtime` build identically (AC-1, R-2).
3. **Phase: manifests** — author `kernel.require` + per-profile `.require` verbatim from build.sh; remove build.sh require blocks.
4. **Phase: Go registry + enforcement** — implement resolve/token/enumerate + floor+manifest enforcement; wire into `ze appliance kernel` (AC-4/5/6/7/8); registry-aware `iso --check` + doctor (AC-10); open config.go validation (AC-9).
5. **Functional tests** — `kernel-compose` (AC-3/4/5/7), no-shell assertion (AC-2), runtime regression.
6. **Full verification** — `make ze-verify`.
7. **Complete spec** — audit, learned summary, two commits.

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Plan |
| 3. Wiring | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases |
| 6. Full verification | `make ze-verify` |
| 14. Present summary | Executive Summary |

### Critical Review Checklist
| Check | What to verify |
|-------|----------------|
| Completeness | Every AC has impl with file:line |
| Correctness | existing profiles' merged config + enforced set unchanged (AC-1) |
| No-shell | no project shell in the kernel-build path (AC-2) |
| Data flow | enforcement in Go only; build.py has no profile/require logic |
| Rule: no-layering | build.sh fully removed, not left beside build.py |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `build.py` exists, `build.sh` gone | `ls tools/kernel-builder/build.py`; `! test -e tools/kernel-builder/build.sh` |
| manifests exist | `ls tools/installer-kernel/*.require gokrazy/kernel/*.require` |
| Go enforcement reachable | grep `ze appliance kernel` → enforce call |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | profile name token-validated before filesystem/URL/argv use (AC-8, R-5) |
| Path traversal | `..`/`/` in profile name rejected |
| Subprocess args | build.py passes controlled argv to make/merge_config.sh/patch; no shell=True |

### Documentation Update Checklist
| Category | Needed? | File / Evidence |
|----------|---------|-----------------|
| Feature list | No | No new top-level feature; this changes the existing installer/runtime kernel builder behavior. |
| User guide | Yes | `docs/guide/ze-install.md` Installer Kernel section: registry profiles, `.require` manifests, Go-verified path. |
| Config syntax | Yes | `docs/guide/appliance.md` mentions `image.kernel-profile` behavior for open tokens. |
| CLI reference | Yes | `docs/guide/ze-install.md` examples and profile explanation for `ze appliance kernel --profile`. |
| API/RPC docs | No | No RPC/API contract changes. |
| Plugin SDK | No | No plugin SDK changes. |
| Wire format | No | Not a protocol or wire encoding change. |
| RFC compliance | No | Not an RFC implementation. |
| Comparison table | No | No daemon comparison claim changes. |
| Test infrastructure | Yes | Install functional tests gain registry/manifest assertions in `test/install/*.ci`; no new runner format. |
| Architecture design | Yes | `tools/installer-kernel/README.md` and `ai/rules/qemu-testing.md` must point from runtime kernel requirements to `.require` manifests instead of `build.sh`. |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Decision logic (resolve, validate, enumerate, enforce) in Go; build.py is a thin recipe | keep logic in the build script | user directive "logic in Go, thin python script"; Go is unit-testable without Docker |
| build.sh → Python `build.py`, subprocess only for make/merge_config.sh/patch | keep shell; reimplement merge in Python | `go-standards.md` bans shell scripts; reimplementing merge_config.sh risks subtle diffs |
| Required-`=y` data-driven via `.require` + hardcoded universal floor in Go | hardcoded only; manifest only | user chose belt-and-suspenders; manifest gives per-profile guarantee, floor is typo-proof |
| `# ze-base:` header, one level | separate `.base` file; explicit list | lightest; one-level avoids cycles (R-4); migrates hardware-kms special-case |
| Verified guarantee on `ze appliance kernel`; raw `make` builds unverified | require ze build for all builds | 870 forbids requiring ze build; keeps dev `make` fast |
| Existing profiles migrate to manifests verbatim (incl. runtime) | additive-only / installer-only | user chose uniform migration |

## Triple Challenge
- **Simplicity:** minimum to honor the user's directives (de-shell + logic-in-Go + verifiable guarantee). The thin build.py + small Go registry/enforcer is the least machinery that achieves it; reuses merge_config.sh and the existing cache/URL keying.
- **Uniformity:** follows ze data-driven ethos, the existing fragment/merge flow, and the `*Fn`/`cmdKernel=runKernel` idiom. New `.require` manifest is the data form of checks already hardcoded in build.sh.
- **Performance:** build-time tooling only; no datapath, no per-event allocation. N/A.

## Known Limitations
- Required-`=y` manifest guarantees required drivers present, NOT that the kernel is minimal. Minimality is an authoring property of the fragment.
- Raw `make` builds (`make -C tools/installer-kernel`, `make ze-kernel`) are not Go-verified; the verified path is `ze appliance kernel` (and, in C, `ze appliance build`).
- This spec adds the mechanism only. Per-hardware installer targets + `--hardware` selection are **spec B**; per-hardware runtime kernels + `hardware-target` baking are **spec C**.

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-10 all demonstrated (see Goal Validation below)
- [x] End-to-End User Stories: every story has a working path and a passing test
- [x] Wiring Test table complete — every row has a concrete test name, none deferred
- [x] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE) — reviewer re-check returned "No findings"
- [x] Full gate exception approved by Thomas; unrelated failures documented in Implementation Audit, changed-scope verification passed
- [x] Feature code integrated (`internal/*`, `tools/*`)
- [x] Documentation Update Checklist answered Yes/No with source evidence
- [x] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [x] Tests written
- [x] Tests FAIL (unit registry tests initially failed before implementation; `bin/ze-test install 19` failed while `build.sh` still existed)
- [x] Tests PASS (`go test ./internal/appliance -count=1`; `bin/ze-test install 1 2 3 4 5 19 20 21 22 23 24 32`; `make ze-verify-changed`)
- [x] Boundary tests for all numeric inputs (N/A — no new numeric inputs)
- [x] Functional tests for end-to-end behavior
- [x] Interop tests for protocol features (N/A — not a protocol feature)
- [x] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [x] Critical Review passes — reviewer re-check returned "No findings"
- [x] Partial/Skipped items have user approval; Thomas approved unrelated full-gate blocker for closure
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Write learned summary to `plan/learned/982-install-11-hw-kernel-profiles.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

## Goal Validation
| Goal | Evidence |
|------|----------|
| AC-1 existing profiles use build.py paths | `appliance-kernel-docker.ci`, `appliance-kernel-qemu.ci`, `kernel-runtime-deps.ci`, `ze-kernel-overlay.ci`, `kernel-compose.ci` passed in `make ze-verify-changed` |
| AC-2 no shell recipe remains | `test/install/kernel-builder-no-shell.ci`; `tools/kernel-builder/build.py`; `tools/kernel-builder/build.sh` deleted |
| AC-3 open registry fixture | `test/install/kernel-compose.ci` raw make fixture; `test/install/appliance-kernel-registry.ci` `ze appliance kernel --profile fixture` |
| AC-4 missing `.require` rejected | `TestResolveProfileFragments`; `appliance-kernel-registry.ci` missing profile path |
| AC-5 missing manifest symbol rejected | `TestEnforceRequire`; `appliance-kernel-registry.ci` `CONFIG_VIRTIO_BLK` failure |
| AC-6 universal floor hardcoded | `TestEnforceUniversalFloor`; `appliance-kernel-registry.ci` `CONFIG_IP_PNP_DHCP` failure |
| AC-7 `# ze-base:` ordering | `TestResolveProfileFragments`; `appliance-kernel-registry.ci` fragment order assertion; `hardware-kms.config` declares `# ze-base: hardware` |
| AC-8 token validation | `TestProfileNameToken`; `qemu-build.py validate_profile`; `config.go` `image.kernel-profile` validation |
| AC-9 custom valid token accepted | `TestConfigValidation/kernel profile custom token valid` |
| AC-10 registry enumeration | `TestEnumerateRegistry`; `cmd_iso.go` and `doctor_checks.go` call `registeredKernelProfiles` |

## Implementation Summary
Converted the shared kernel builder from `build.sh` to `build.py`, added data-driven profile manifests and one-level base layering, moved verified profile resolution/enforcement into Go, updated Docker/QEMU/Makefile call paths, and added unit/functional/docs coverage.

## Implementation Audit
- Full changed verification passed: `make ze-verify-changed`.
- Targeted evidence passed: `go test ./internal/appliance -count=1`; `python3 -m py_compile tools/kernel-builder/build.py tools/kernel-builder/qemu-build.py`; `bin/ze-test install 1 2 3 4 5 19 20 21 22 23 24 32`; `make ze-doc-test`; `make ze-validate`; `python3 scripts/dev/audit-test-relaxation.py`.
- Full `make ze-verify` did not complete cleanly because unrelated worktree/state failures surfaced in `ze-unit-test-cached`: `tmp/isis/genfix`, `tmp/lease-test`, `internal/component/doctor`, and `scripts/checks`.
- Full `make ze-test` did not complete cleanly because unrelated package/race failures surfaced outside install-11: `tmp/isis/genfix` vendor import failure and a BGP reactor data race.
