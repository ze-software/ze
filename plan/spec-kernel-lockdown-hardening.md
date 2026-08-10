# Spec: kernel-lockdown-hardening

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-06-30 |

> NOTE (2026-06-30): This spec is a **design deliverable, not scheduled for implementation**.
> It is written to be implementation-ready and has been critically reviewed (see
> "Critical Review" near the end). Do not treat unchecked boxes as in-progress work.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. The kernel build-consolidation and build-convergence records (retired with the learned corpus) - kernel build system
4. `tools/kernel-builder/run.py`, `tools/kernel-builder/build.py` - build driver + engine (signing hooks here)
5. `gokrazy/kernel/{kernel,runtime}.{config,require}`, `tools/installer-kernel/hardware.config` - config fragments + floor enforcement

## Task

Harden the ze appliance kernel against a **post-exploitation attacker who has already gained
root** (e.g. via a bug in ze's network-facing Go code), using Linux **kernel lockdown in
integrity mode** plus the Cloudflare kernel-hardening set. Lockdown integrity blocks the
attacker from modifying the running kernel: loading unsigned modules, unsigned `kexec`,
`/dev/mem` and `/dev/kmem`, direct PCI BAR access, `ioperm`/`iopl`, MSR writes, hibernation,
ACPI table override, and debugfs. (`/proc/kcore` is a *confidentiality*-mode restriction,
not integrity — see C-10.)

This is the **in-memory / runtime half** of an integrity story. The **at-rest half** (UEFI
Secure Boot, signed boot chain, dm-verity root) is an explicit follow-on (see Known
Limitations). Lockdown alone is defense-in-depth: it raises the bar substantially but does
not, by itself, stop an attacker who can rewrite the boot media.

### Reference Material (authoritative sources for this spec)

| Ref | Source | What we take from it |
|-----|--------|----------------------|
| R-man | `kernel_lockdown.7` — https://man7.org/linux/man-pages/man7/kernel_lockdown.7.html | The exhaustive list of what integrity vs confidentiality blocks; that lockdown is one LSM enabled by `CONFIG_SECURITY_LOCKDOWN_LSM` and present in the active LSM list. |
| R-cf | Cloudflare "Linux kernel hardening" — https://blog.cloudflare.com/linux-kernel-hardening/ | Production recipe: integrity mode (not confidentiality), `CONFIG_MODULE_SIG_FORCE`, drop legacy `CONFIG_KEXEC`, `CONFIG_KEXEC_FILE` + `KEXEC_SIG_FORCE` + `KEXEC_BZIMAGE_VERIFY_SIG`, `RANDOMIZE_BASE` (KASLR), `SECURITY_DMESG_RESTRICT`, and **ephemeral per-build module-signing keys** (sign at build, destroy the private key). They run XDP at scale and deliberately stay in integrity mode because confidentiality "blocks all runtime debugging capabilities, like perf or eBPF." |
| R-lpc | LPC 2020 "eBPF in kernel lockdown mode", A. Carvalho de Melo — https://lpc.events/event/7/contributions/678/attachments/580/1177/eBPF-in-kernel-lockdown-mode-short-paper.pdf | Integrity mode requires only a signature; **confidentiality** additionally restricts `bpf_probe_read()` (tracing programs reading kernel memory). Networking (XDP/TCX) programs are restricted only to the packet being processed and keep working. Signed-BPF is a future direction, not required for integrity. |

### Confirmed scope decisions (SCOPE gate, user-approved 2026-06-30)

| Fork | Decision | Rationale |
|------|----------|-----------|
| Module strategy | Ephemeral per-build module key: `CONFIG_MODULE_SIG_FORCE`, per-build keypair auto-generated inside the build container, modules signed, pubkey embedded in vmlinux, privkey destroyed | Chosen over config-only "build the remaining modules in" (which lockdown integrity already enforces). User chose defense-in-depth. **Ephemeral is correct here: signer = verifier = same build.** |
| kexec OTA (amd64 only) | Sign the kernel **image** with a **stable release key** (NOT ephemeral): `CONFIG_KEXEC_SIG` + `CONFIG_KEXEC_BZIMAGE_VERIFY_SIG`, cert embedded via `CONFIG_SYSTEM_TRUSTED_KEYS` in every build | Chosen over the full-reboot fallback to preserve fast OTA. **Corrected after review (C-1): ephemeral keys CANNOT do cross-build kexec — the old running kernel must already trust the new image's signer. This forces a stable, managed image-signing key (a key-management cost the user should weigh; see "Key strategy" + Known Limitations).** arm64 is excluded (C-2): gokrazy `reboot.go` never kexecs on `!amd64`. |
| Coverage | Lockdown integrity + module signing on the **runtime** kernel (both arches); image/kexec signing **amd64 only**; KASLR (`RANDOMIZE_BASE`) + `DMESG_RESTRICT` on **both** runtime and installer | Installer is ephemeral; lockdown there adds little. KASLR/dmesg are cheap everywhere. arm64 gets lockdown+KASLR+dmesg+module-signing but no image signing (it cannot kexec). |

### Mode decision (integrity, never confidentiality)

Integrity mode only, forced at compile time via `CONFIG_LOCK_DOWN_KERNEL_FORCE_INTEGRITY=y`.
Confidentiality mode would restrict `bpf_probe_read`/perf and likely `/proc/config.gz`
(`IKCONFIG_PROC`, `kernel.config:77-78`). Per R-lpc, the trafficusage TCX plugin is networking
BPF and is **not** restricted by integrity mode.

### Key strategy (TWO distinct keys — corrected after independent review, C-1)

The single biggest correction to the original scope framing: "ephemeral key for everything"
does not work, because kexec OTA verification crosses a build boundary.

| Key | Lifecycle | Why | Embedded via | Used to sign |
|-----|-----------|-----|--------------|--------------|
| **Module key** | Ephemeral, per-build, auto-generated in-container, **destroyed after use** | Modules are verified by the *same* build's embedded pubkey (signer = verifier). Cloudflare's actual module approach. | auto into `vmlinux` `.builtin_trusted_keys` during the vmlinux build | the kernel modules (during `modules_install`, `MODULE_SIG_ALL=y`) |
| **Image key** (amd64 only) | **Stable, long-lived release secret**, kept in CI/build infra, NOT destroyed | `kexec_file_load` verifies the *new* image against the *running (old)* kernel's keyring. An old kernel only trusts a key whose cert it embedded at *its* build time, so every build must embed the *same* cert and sign with the *same* key. | `CONFIG_SYSTEM_TRUSTED_KEYS` in **every** build | the `bzImage` (sbsign PE/COFF Authenticode) |

**Consequence the user should weigh:** preserving fast kexec OTA under lockdown requires
managing a stable image-signing private key NOW (rotation, storage, CI access). That is the
same key material the Secure Boot follow-on needs, so it is a bridge, not waste — but it is a
key-management cost that the declined "accept full-reboot fallback" option avoided. If that
cost is unwanted, the fallback path (no image signing; OTA full-reboots under lockdown) is the
escape hatch and defers ALL image signing to the Secure Boot track. This spec documents the
stable-key path as primary per the gate decision; the fallback remains a one-line reversal
(set the updater default to `kexec=off`, `updater.go`).

## Required Reading

### Architecture Docs
- [ ] The kernel build-consolidation and build-convergence records (retired with the learned corpus) — kernel build system
  → Constraint: one driver `tools/kernel-builder/run.py` resolves fragments and calls `build.py` in docker/qemu; both Makefiles AND `ze appliance kernel` go through it. Any signing step lives in `build.py` (runs in-container), not on the host.
  → Constraint: config floors enforced by paired `.require` files; the Go path `internal/appliance/kernelreg.go` resolves fragments+manifests, the actual `=y` assertion is `build.py:enforce_required_symbols` (143-160). A missing required `=y` symbol fails the build.
  → Constraint: resolved `.config` (defconfig + merge_config + olddefconfig) is the cheaply-diffable artifact; unrelated symbols must stay byte-identical.
- [ ] `ai/rules/repo-maintenance.md` — runtime-dependency readiness checks
  → Constraint: a new runtime dependency (here: the `/sys/kernel/security/lockdown` procfs/securityfs path) requires a registered `ze doctor` check. The appliance is a component, not a plugin, so register via `diagnostic.RegisterDoctorCheck()` from the owning package init(); add a `doctor-<component>-<condition>` code to `internal/core/diagnostic/codes.go`, explainable via `ze explain`. Needs a Linux-tagged unit test + a `ze doctor --json` functional test + QEMU coverage.

### Source Files Read (Current Behavior)
- [ ] `tools/kernel-builder/build.py` (407L)
  → Constraint: `enforce_required_symbols` (143-160) and `required_symbols_for_fragments` (119-140) only understand `CONFIG_X` / `CONFIG_X=y`. They CANNOT enforce string-valued (`CONFIG_LSM="..."`) or `=n` symbols. The floor can mandate `LOCKDOWN_LSM=y`, `MODULE_SIG_FORCE=y`, `KEXEC_SIG=y`, `RANDOMIZE_BASE=y`, `DMESG_RESTRICT=y` but NOT that lockdown is in `CONFIG_LSM` nor that confidentiality is not forced.
  → Constraint: build order is download → extract/restore tree → patches → `merge_config` → `olddefconfig` → `enforce_required_symbols` → `build_kernel` → copy outputs (353-401). Module/image signing must hook AFTER `build_kernel` (384) and BEFORE `copy_runtime_outputs` (389). `modules_install` (copy_runtime_outputs, 287-292) is where `MODULE_SIG_ALL=y` would auto-sign.
  → Constraint: `validate_arch` (51-56) maps amd64→`bzImage`, arm64→`Image`. KEXEC bzImage verification is x86-only; arm64 image-sig differs. Signing is arch-asymmetric.
- [ ] `tools/kernel-builder/run.py` (498L)
  → Constraint: docker run mounts named volumes `ze-kernel-work:/build` and `ze-kernel-build:/tmp/kbuild` (273-276); `build.py:restore_or_extract_tree` (192-219) REUSES an existing build tree. A kernel-auto-generated `certs/signing_key.pem` would PERSIST in the volume across builds — NOT ephemeral, and present on the build host. "Destroy privkey" must be explicit.
- [ ] `gokrazy/kernel/kernel.config` (91L) — runtime base. `CONFIG_KEXEC_FILE=y` (81), `CONFIG_SECURITY_LANDLOCK=y` (90), `IKCONFIG_PROC` (77-78). No lockdown/sig/secureboot. No `CONFIG_KEXEC=y` (legacy kexec_load absent — good).
- [ ] `gokrazy/kernel/runtime.config` (129L) — `CONFIG_MODULES=y` (2-3); all NIC/L2TP/PPP/BPF built-in (`=y`). `CONFIG_BPF_SYSCALL=y`/`BPF_JIT=y` (76,80).
- [ ] `gokrazy/kernel/runtime.require` — current floor (CONFIG_MODULES, PPP/L2TP set, BPF_SYSCALL/JIT, ...). Extension point for the new mandatory `=y` symbols.
- [ ] `tools/installer-kernel/hardware.config` (72L) — installer hw profile, `CONFIG_EFI=y`/`EFI_STUB=y`. No hardening flags.
- [ ] `internal/appliance/kernelreg.go` (233L) — Go-side fragment+manifest resolver mirroring run.py; the `.require` floor is resolved here, enforced in build.py.
- [ ] `internal/component/vpp/dpdk.go` (37) / `dpdk_linux.go` (17-22) — VFIO modprobe (`vfio`, `vfio_pci`, `vfio_iommu_type1`), only when DPDK interfaces configured (`vpp.go,167`; skipped in External mode `vpp.go`).
- [ ] `internal/plugins/trafficusage/attach_linux.go` (46-64) — `link.AttachTCX` + `AttachTCXIngress/Egress`; pure networking BPF, no `bpf_probe_read`. Only privileged step is `rlimit.RemoveMemlock` (33), not lockdown-gated.
- [ ] `gokrazy/modcache/.../modules.go` (21-45) — gokrazy boot loads exactly one module, `pwm-fan.ko` (Pi 5 fan); `FinitModule` swallows EEXIST/EBUSY/ENODEV/ENOENT.
- [ ] `gokrazy/modcache/.../gokrazy.go` (232-234) — `loadModules()` error is logged, NOT fatal. A refused pwm-fan under lockdown does not block boot.
- [ ] `gokrazy/modcache/.../reboot_amd64.go` (14-50) — OTA reboot uses `KexecFileLoad`; on ANY error logs + falls back to `LINUX_REBOOT_CMD_RESTART`. Graceful degrade already present.
- [ ] `internal/appliance/updater/updater.go` (188-227) — OTA default `kexec:true`; `kexec=off` query forces full reboot. Caller `cmd_push.go` uses the default.

**Key insights:**
- Nothing in ze code uses `/dev/mem`, `iopl`, or `ioperm` (grep: only vendored `x/sys`). Lockdown's hardware-poke restrictions affect nothing we do.
- The appliance already follows a "build it in, load nothing at runtime" posture, so lockdown integrity is *close to free*; the only real runtime module loads are VFIO (DPDK, opt-in) and pwm-fan (Pi, non-fatal).
- The build-time floor (`=y` symbols) and a runtime doctor check (`/sys/kernel/security/lockdown` == integrity) are complementary: the floor catches config regressions cheaply, the doctor check is the only thing that proves lockdown is actually *active and in integrity mode* (the floor can't, see build.py constraint).

## Current Behavior (MANDATORY)

**Source files read:** (full annotations in Required Reading → Source Files Read)
- [ ] `tools/kernel-builder/build.py` — build engine; floor enforces `=y` only; signing must hook after `build_kernel`
- [ ] `tools/kernel-builder/run.py` — docker/qemu driver; named-volume build-tree cache (key-persistence hazard)
- [ ] `gokrazy/kernel/runtime.config` — `CONFIG_MODULES=y`; NIC/L2TP/PPP/BPF all built-in
- [ ] `gokrazy/kernel/kernel.config` — `KEXEC_FILE=y`, `LANDLOCK=y`, no lockdown/sig
- [ ] `internal/appliance/kernelreg.go` — Go-side fragment + `.require` manifest resolver
- [ ] `internal/plugins/trafficusage/attach_linux.go` — TCX networking BPF (A-1)
- [ ] `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/gokrazy.go` — `loadModules` error non-fatal (A-2)
- [ ] `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/reboot_amd64.go` — `KexecFileLoad` + full-reboot fallback
- [ ] `internal/appliance/updater/updater.go` — OTA default `kexec:true`

**Behavior to preserve:**
- OTA update still succeeds (signed-kexec when the image verifies; otherwise the existing full-reboot fallback).
- trafficusage TCX eBPF byte accounting keeps working (integrity preserves networking BPF — A-1, confirmed).
- DPDK NIC binding keeps working when configured (VFIO available without an unsigned modprobe).
- gokrazy boot module load (pwm-fan on Pi) does not hard-fail the boot (A-2, confirmed at gokrazy.go).
- Resolved `.config` for unrelated symbols stays byte-identical (no incidental kernel behaviour change).
- Installer still installs (its disk/partition/mount ops are normal syscalls, unaffected by KASLR/dmesg, and the installer has no lockdown).

**Behavior to change:**
- Runtime kernel runs with lockdown integrity active from early boot.
- Any kernel module that loads at runtime must be signed by the per-build key (or built in).
- Runtime kernel image is signed; kexec verifies the signature.
- Both kernels get KASLR + dmesg restriction.

## Data Flow (MANDATORY)

### Entry Point
- **Build-time entry:** `internal/appliance/kernel.version` + the profile config fragments + the `.require` floor manifest enter `run.py`, the single kernel build driver. Format at entry: Kconfig fragment lines plus `CONFIG_X` / `CONFIG_X=y` floor entries.
- **Runtime entry:** the lockdown LSM initializes at early boot; the `FinitModule` and `KexecFileLoad` syscalls are the points lockdown gates; `ze doctor` enters via a securityfs read of `/sys/kernel/security/lockdown`.

### Transformation Path
1. `run.py:resolve_fragments` (109-158): kernel.version + profile → ordered fragment list.
2. docker/qemu → `build.py:main` (353-401): `merge_config` → `olddefconfig` → `enforce_required_symbols` (the `=y` floor) → `build_kernel` (384).
3. **NEW signing setup — BEFORE `build_kernel`:384** (corrected, C-3): provision the **stable image-signing cert** at `CONFIG_SYSTEM_TRUSTED_KEYS` and let the kernel auto-generate (or pre-place) the **ephemeral module key** at `CONFIG_MODULE_SIG_KEY`. Both pubkeys/certs MUST exist before `build_kernel` so they are compiled into `vmlinux`'s `.builtin_trusted_keys`. A key generated *after* `build_kernel` is never embedded.
4. `build_kernel` (384) compiles vmlinux with both certs embedded.
5. **Signing — DURING/AFTER `copy_runtime_outputs`** (C-3): modules signed by the ephemeral key during `modules_install` (`MODULE_SIG_ALL=y`, build.py, reached at build.py); then (amd64) the `bzImage` is sbsigned with the stable image key. **Last:** the ephemeral module private key is destroyed; the stable image private key is NOT in the container (it is injected for the sign step only, or signing happens in CI).
6. `copy_runtime_outputs` (283-308): signed `vmlinuz` + signed modules → gok pack → appliance image.
7. **Runtime:** lockdown active early (`LOCKDOWN_LSM_EARLY`, `FORCE_INTEGRITY`) → unsigned `FinitModule` (gokrazy `modules.go`) refused, logged, non-fatal → on **amd64**, `KexecFileLoad` (`reboot_amd64.go`) of a stable-key-signed image accepted, unsigned refused → full-reboot fallback; on **arm64**, `reboot.go` always full-reboots (no kexec) → `ze doctor` reports the active mode.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build host ↔ build container | signing key generated + destroyed entirely inside the container; only signed artifacts leave | [ ] grep built image for key material (A-4) |
| Kernel config floor ↔ resolved .config | build.py enforce_required_symbols (`=y` only) | [ ] new floor symbols assert; string LSM cannot (R-7) |
| Running kernel ↔ ze doctor | securityfs read of `/sys/kernel/security/lockdown` | [ ] doctor check unit + functional + QEMU |

### Integration Points
- `build.py:enforce_required_symbols` (143-160) — the extended `=y` floor (new hardening symbols).
- `internal/appliance/doctor_checks.go` + `internal/core/diagnostic/codes.go` — new `doctor-appliance-lockdown` check. **Registration over hardcoding:** registered via `diagnostic.RegisterDoctorCheck()` and discovered by the doctor runner; no per-feature switch/case is added to a core or shared struct.
- gokrazy reboot + module-load paths (`reboot_amd64.go`, `modules.go`) — code unchanged; the new kernel policy gates them.

### Architectural Verification
- [ ] No bypassed layers (build floor + runtime doctor check are complementary, not redundant)
- [ ] Registration over hardcoding — the doctor check registers and is core-discovered; no new per-feature field/switch/factory added to a core/shared package (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Lockdown integrity does NOT block the trafficusage TCX program load | R-lpc; `attach_linux.go` uses TCX networking hooks, no `bpf_probe_read` | trafficusage breaks on hardened appliance | QEMU evidence: attach TCX with lockdown=integrity active | **confirmed (code+source)** |
| A-2 | gokrazy boot does not hard-fail when pwm-fan.ko load is refused/absent under lockdown | `gokrazy.go` logs and continues; `modules.go` swallows ENOENT | appliance fails to boot | code read | **confirmed (code)** |
| A-3 | `KexecFileLoad` with `KEXEC_SIG` accepts our build-signed image and the gokrazy OTA path still works | `reboot_amd64.go` | OTA kexec always fails → silent full-reboot fallback masks a real break | QEMU OTA evidence: signed kexec succeeds, fallback NOT taken | unvalidated |
| A-4 | The **ephemeral module** key can be generated, used, and destroyed in-container with no key material in the image; the **stable image** key is a managed secret kept OUT of the container/image | `build.py` runs in-container; named-volume caching of `/tmp/kbuild` is the hazard (run.py) | module privkey leaks into image/volume, or stable image privkey ends up embedded | review signing stage; grep image + `ze-kernel-build` volume for PEM/key | unvalidated |
| A-4b | A stable image-signing key whose cert is embedded via `CONFIG_SYSTEM_TRUSTED_KEYS` lets an OLD running kernel verify a NEW build's signed image (kexec_file_load checks `.builtin_trusted_keys`; Secure Boot `.platform` keyring is an *additional*, not *required*, source) | independent review (keyrings patchwork) | kexec OTA fails every update → silent full-reboot | amd64 QEMU OTA across two builds signed by the same key | unvalidated |
| A-5 | Installer disk/partition/mount ops are unaffected by KASLR + DMESG_RESTRICT (no lockdown there) | `hardware.config`; install ops are normal syscalls | install breaks | existing install QEMU evidence still passes | unvalidated |
| A-6 | With `LOCKDOWN_LSM_EARLY=y` + `FORCE_INTEGRITY=y`, lockdown is active in integrity mode regardless of `CONFIG_LSM` string ordering | kernel lockdown design (R-man); default `CONFIG_LSM` includes lockdown | lockdown silently inactive; floor can't catch it | QEMU: assert `/sys/kernel/security/lockdown` shows `[integrity]` (AC-4) | unvalidated |
| A-7 | arm64 (Pi) gokrazy does **NOT** kexec — OTA always full-reboots — so arm64 needs lockdown + KASLR + dmesg + module signing but **no image signing** | `gokrazy/.../reboot.go` (`!amd64`) ignores `tryKexec`, always `LINUX_REBOOT_CMD_RESTART` | wasted arm64 image-signing work; dead `KEXEC_*` config on arm64 | code read | **confirmed (code)** — image/kexec signing is amd64-only |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Lockdown silently disables a runtime feature we did not enumerate (perf, debugfs, a future /dev poke) | feature fails only on the appliance, not in unit tests | QEMU evidence exercising trafficusage + DPDK + OTA + install end-to-end with lockdown active; document the enumerated allowed surface |
| R-2 | Signed-kexec verification fails on the appliance → every OTA silently full-reboots, hiding the regression | OTA works but slower; nothing surfaced | Doctor/telemetry signal that kexec succeeded; gokrazy already logs "kexec reboot failed" (reboot_amd64.go) — surface that log |
| R-3 | Ephemeral module key not actually destroyed / persists in the docker named volume or squashfs image | key file present in image or the `ze-kernel-build` volume (`/tmp/kbuild`, where the reused build tree + `certs/` live — C-9) | build.py deletes key in same container invocation; test greps image + `ze-kernel-build` + asserts no `certs/signing_key.pem` in the cached tree |
| R-8 | Stable image-signing private key is mishandled (committed, baked into the image, or lost) | key in git/image, or OTA verification suddenly fails after a key rotation | keep the privkey in CI secret storage; sign the image in CI (not in the shared build container); rotate via dual-trust (embed old+new cert) |
| R-4 | `DMESG_RESTRICT` hides boot diagnostics field engineers rely on | support bundle missing dmesg for non-root | ze support tooling runs as root (verify); document the change |
| R-5 | Image-signing symbols are amd64-only; if put in the shared `runtime.require` floor, the arm64 build aborts in `enforce_required_symbols` (the floor is arch-agnostic and only understands `=y`) | arm64 kernel build fails on a missing x86 symbol | keep `KEXEC_SIG`/`BZIMAGE_VERIFY_SIG`/`SYSTEM_TRUSTED_KEYS` OUT of the shared floor; put them in an amd64-only config fragment, OR teach the floor per-arch manifests (larger change, call out explicitly) |
| R-6 | `MODULE_SIG_FORCE` blocks a module we actually need at runtime that is NOT built in (a future addition, or VFIO if not built in) | DPDK bind fails: "modprobe vfio: Key was rejected" | build VFIO in (`CONFIG_VFIO{,_PCI,_IOMMU_TYPE1}=y`) so nothing relies on a signed module; signing is the backstop, not the path |
| R-7 | The `=y`-only floor cannot assert lockdown is in `CONFIG_LSM` nor that confidentiality is not forced; a regression to confidentiality or to lockdown-absent would pass the build | unit tests green, appliance behaves wrong | runtime doctor check (AC-4/AC-8) is the real guarantee; consider a dedicated build-time check that greps the resolved `.config` for the string symbols (extends enforce_required_symbols) |

## Wiring Test (MANDATORY)

The "entry point" is the build pipeline and the running kernel. Reachability = the floor
mandates the `=y` symbols, a built kernel actually has lockdown active, and a doctor check
observes it.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `runtime.require` floor | → | `build.py:enforce_required_symbols` fails build if a hardening `=y` symbol is missing | `test/install/kernel-hardening-floor.ci` (new) |
| Resolved runtime `.config` | → | lockdown + sig + KASLR + dmesg symbols present | `test/install/kernel-lockdown-config.ci` (new) |
| Booted runtime kernel (QEMU) | → | `/sys/kernel/security/lockdown` == `[integrity]` | QEMU evidence `ze-qemu-lockdown-active` (new) |
| `ze doctor` | → | `appliance` lockdown check reads securityfs, emits `doctor-appliance-lockdown` | unit test + `ze doctor --json` functional test |
| OTA under lockdown (QEMU) | → | gokrazy `KexecFileLoad` accepts signed image | QEMU evidence `ze-qemu-lockdown-ota-kexec` (new) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Resolved **runtime** `.config` (amd64) | Contains `CONFIG_SECURITY_LOCKDOWN_LSM=y`, `CONFIG_SECURITY_LOCKDOWN_LSM_EARLY=y`, `CONFIG_LOCK_DOWN_KERNEL_FORCE_INTEGRITY=y`, `CONFIG_MODULE_SIG=y`, `CONFIG_MODULE_SIG_FORCE=y`, `CONFIG_MODULE_SIG_ALL=y`, `CONFIG_KEXEC_FILE=y`, `CONFIG_KEXEC_SIG=y`, **`CONFIG_KEXEC_BZIMAGE_VERIFY_SIG=y`** (x86 image-verify — C-4), `CONFIG_SYSTEM_TRUSTED_KEYS` non-empty, `CONFIG_RANDOMIZE_BASE=y`, `CONFIG_SECURITY_DMESG_RESTRICT=y`; **`# CONFIG_KEXEC is not set`** (legacy kexec_load dropped — C-5, comes from defconfig); and lockdown present in the active LSM set |
| AC-1b | Resolved **runtime** `.config` (arm64) | Same lockdown + module-sig + KASLR + dmesg symbols, but **no image-signing symbols** (`KEXEC_SIG`/`BZIMAGE_VERIFY_SIG` not required) — arm64 gokrazy never kexecs (C-2) |
| AC-2 | Resolved **installer** `.config` | Contains `CONFIG_RANDOMIZE_BASE=y` + `CONFIG_SECURITY_DMESG_RESTRICT=y`; does NOT contain `CONFIG_SECURITY_LOCKDOWN_LSM` |
| AC-3 | `runtime.require` floor | Mandates the new runtime hardening `=y` symbols so a regression fails `build.py:enforce_required_symbols` |
| AC-4 | Booted runtime kernel (QEMU) | `/sys/kernel/security/lockdown` shows `[integrity]` (not `[none]`, not `[confidentiality]`) |
| AC-5 | Booted runtime kernel (QEMU) | trafficusage TCX program attaches successfully (A-1) |
| AC-6 | OTA update under lockdown (QEMU, **amd64**) | Stable-key-signed kexec path succeeds; the full-reboot fallback is NOT silently taken. (arm64 always full-reboots by design — not an AC failure.) |
| AC-7 | Built runtime image + `ze-kernel-build` volume | No **ephemeral module** private key present: `certs/signing_key.pem` absent from the image AND from the in-tree `linux-<ver>-yes/certs/` inside the `ze-kernel-build` named volume (`/tmp/kbuild`, run.py — C-9). The stable image private key is intentionally never placed in the image. |
| AC-8 | `ze doctor` on the appliance | Reports lockdown integrity active; flags a clear `doctor-appliance-lockdown` diagnostic if mode is none/confidentiality |
| AC-9 | DPDK configured (QEMU, if in scope for the evidence host) | VFIO bind works without an unsigned-module rejection (VFIO built in) |
| AC-10 | Installer kernel boot (QEMU) | Install completes; KASLR/dmesg active; no lockdown present |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Builds the runtime kernel | run.py → build.py → floor enforces hardening symbols → signed vmlinuz+modules | `kernel-hardening-floor.ci`, `kernel-lockdown-config.ci` |
| 2 | Boots the hardened appliance | lockdown integrity active early; trafficusage + routing protocols run | `ze-qemu-lockdown-active` |
| 3 | Runs `ze doctor` on a hardened box | securityfs read → `doctor-appliance-lockdown` OK | doctor unit + functional |
| 4 | Performs an OTA update | updater → gokrazy KexecFileLoad of signed image → boots new kernel | `ze-qemu-lockdown-ota-kexec` |
| 5 | Operator on an UNhardened/older box runs `ze doctor` | check reports lockdown not active (informational, not a hard failure on non-appliance) | doctor unit (none-mode case) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRuntimeRequireMandatesLockdownSymbols` | `internal/appliance/kernelreg_test.go` | runtime.require lists the hardening `=y` symbols | |
| `TestEnforceRequiredSymbolsFailsWithoutLockdown` | python build test (existing harness) | floor fails a config missing a hardening symbol | |
| `TestDoctorLockdownCheckParsesSecurityfs` | `internal/appliance/doctor_checks_test.go` (Linux-tagged) | parse the active (single-bracketed) token from securityfs, e.g. `none [integrity] confidentiality` → integrity (C-11) | |
| `TestDoctorLockdownCodeRegistered` | `internal/component/doctor/...` | `doctor-appliance-lockdown` registered + explainable | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| securityfs mode token | none / integrity / confidentiality | integrity (active) | none (flag) | confidentiality (flag) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `kernel-hardening-floor` | `test/install/*.ci` | floor rejects a runtime config missing lockdown/sig | |
| `kernel-lockdown-config` | `test/install/*.ci` | resolved runtime .config has the full symbol set; installer has KASLR+dmesg but no lockdown | |
| `ze doctor --json` lockdown | `internal/component/doctor` functional suite | doctor surfaces `doctor-appliance-lockdown` | |

### QEMU Evidence Tests (MANDATORY — Linux-only kernel behavior, `ai/rules/platform-linux.md`)
| Scenario | Target | What it proves | Status |
|----------|--------|----------------|--------|
| `ze-qemu-lockdown-active` | runtime kernel (amd64) | `/sys/kernel/security/lockdown` == `[integrity]`; trafficusage TCX attaches | |
| `ze-qemu-lockdown-ota-kexec` | runtime kernel (amd64) | stable-key-signed kexec OTA across two builds succeeds (fallback not taken) | |
| `ze-qemu-lockdown-arm64` | runtime kernel (arm64) | lockdown active + module signing on arm64; OTA full-reboots (NO kexec — A-7) | |
| existing install QEMU | installer kernel | install still completes with KASLR+dmesg (A-5) | |

### Interop Tests
N/A — no wire-protocol change. Justification: this is a kernel build/config + doctor-check feature.

## Files to Modify
- `gokrazy/kernel/kernel.config` — add lockdown + module-sig + KASLR + dmesg symbols (arch-common); add `# CONFIG_KEXEC is not set` to drop legacy kexec_load from the resolved config (C-5)
- `gokrazy/kernel/runtime.config` — build VFIO in (`CONFIG_VFIO`, `CONFIG_VFIO_PCI`, `CONFIG_VFIO_IOMMU_TYPE1`) so nothing relies on a signed module (R-6); optionally `CONFIG_SENSORS_PWM_FAN=y` for Pi (A-2 mitigation)
- **NEW** amd64-only image-signing fragment (e.g. `gokrazy/kernel/amd64.config` pulled in for the amd64 build only) — `CONFIG_KEXEC_SIG`, `CONFIG_KEXEC_BZIMAGE_VERIFY_SIG`, `CONFIG_SYSTEM_TRUSTED_KEYS` (kept OUT of the arch-agnostic floor — R-5/C-2). Requires understanding how `run.py`/`build.py` select per-arch fragments (today fragments are per-profile, not per-arch — a real mechanism gap to design).
- `gokrazy/kernel/runtime.require` — add ONLY the arch-common mandatory `=y` symbols (lockdown, module-sig, KASLR, dmesg); NOT the amd64 image-sign symbols (AC-3, R-5)
- `tools/installer-kernel/hardware.config` + `kernel.config` — add `RANDOMIZE_BASE` + `DMESG_RESTRICT` (AC-2)
- `tools/installer-kernel/hardware.require` (and/or base require) — mandate KASLR+dmesg on installer
- `tools/kernel-builder/build.py` — signing: provision both certs BEFORE `build_kernel` (384); sign modules via `MODULE_SIG_ALL` during `modules_install` (in `copy_runtime_outputs`); sbsign the amd64 `bzImage` with the stable key; destroy the ephemeral module key LAST; never let it persist in the `/tmp/kbuild` tree (C-3, R-3)
- `tools/kernel-builder/Dockerfile` — add `sbsigntool` + `openssl` for image + key handling
- `internal/appliance/doctor_checks.go` — register `doctor-appliance-lockdown`; gate it on a **running-on-appliance** signal so it is a no-op (not a warning) on a dev host where `/sys/kernel/security/lockdown` is absent or `[none]` (C-8)
- `internal/appliance/dev_setup_drift_test.go` — add `appliance-lockdown` to the `buildArtifactChecks` skip-set; it is a runtime check, not a host build prerequisite, so it does NOT belong in `dev-setup.py` (C-7, `dev_setup_drift_test.go`)
- `internal/core/diagnostic/codes.go` — register the `doctor-appliance-lockdown` code (title/description/examples, `ze explain`)
- `internal/appliance/kernelreg.go` (+ `_test.go`) — if the floor gains a string-symbol / `=n` check for `CONFIG_LSM` and `# CONFIG_KEXEC is not set` (R-7, C-5), it lands here and in build.py

## Files to Create
- `test/install/kernel-hardening-floor.ci` — floor rejects missing hardening symbols
- `test/install/kernel-lockdown-config.ci` — resolved configs match AC-1/AC-2
- QEMU evidence scripts for `ze-qemu-lockdown-active`, `ze-qemu-lockdown-ota-kexec`, `ze-qemu-lockdown-arm64`
- `internal/appliance/doctor_checks_test.go` additions (Linux-tagged securityfs parse test)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Kernel build config, not runtime YANG config |
| YANG validation | No | — |
| CLI commands/flags | No | No new verb; `ze doctor` already exists |
| Editor autocomplete | No | — |
| Functional test for new behavior | Yes | `test/install/kernel-*.ci`, doctor functional |
| Env var registration | No | — |
| Doctor check for runtime dependencies | **Yes** | `internal/appliance/doctor_checks.go` + `internal/core/diagnostic/codes.go` — the securityfs `/sys/kernel/security/lockdown` dependency (`ai/rules/repo-maintenance.md`) |
| Prometheus counters | No (optional) | Could expose a `lockdown_active` gauge; deferred unless wanted |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — appliance kernel hardening |
| 2 | Config syntax changed? | No | — |
| 3 | CLI command added/changed? | No | `ze doctor` output gains a line; no new command |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | No | — |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` — hardening section + the at-rest follow-on note |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented? | No | — |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` — new QEMU evidence + install tests |
| 11 | Affects daemon comparison? | No | — |
| 12 | Internal architecture changed? | Yes | kernel build doc / `tools/installer-kernel/PROFILES.md` — signing stage + floor symbols |
| 13 | Route metadata keys? | No | — |
| 14 | Prometheus counters? | No (unless gauge added) | — |
| 15 | Registered diagnostic/code/inventory changed? | Yes | diagnostic code list / `ze explain` corpus for `doctor-appliance-lockdown` |
| 16 | Changed source referenced by doc anchors? | Check | grep `docs/` for anchors to build.py / kernel configs |
| 17 | Existing docs show config examples for this area? | Check | appliance kernel docs |

## Implementation Steps (for a future implementer — NOT scheduled)

1. **Phase: Floor + config fragments (cheap, no compile)** — add the `=y` symbols to runtime/installer config + require fragments; build VFIO in. Verify via resolved-`.config` diff (config-only docker run, no `-j` compile) per 988 gotcha. Tests: `kernel-hardening-floor.ci`, `kernel-lockdown-config.ci`.
2. **Phase: Signing in build.py** — provision both certs BEFORE `build_kernel`; `MODULE_SIG_ALL` module signing (ephemeral key) during `modules_install`; sbsign the amd64 `bzImage` with the stable release key after; destroy the ephemeral key LAST (C-3). Dockerfile gains `sbsigntool`/`openssl`. Test: image + `ze-kernel-build` volume have no ephemeral key (AC-7).
3. **Phase: Doctor check** — securityfs parse + `diagnostic.RegisterDoctorCheck()` + code registration + `ze explain`. Tests: unit (Linux) + `ze doctor --json` functional.
4. **Phase: QEMU evidence** — `ze-qemu-lockdown-active`, `-ota-kexec`, `-arm64`; confirm install QEMU still green.
5. **Docs** — features.md, appliance guide (incl. at-rest follow-on note), functional-tests.md, PROFILES.md.

## Known Limitations
- **At-rest integrity is a separate follow-on.** Without UEFI Secure Boot + signed GRUB/kernel + dm-verity root, a root attacker who can write the boot media can ship an image with lockdown disabled. Lockdown defends the running kernel only. The stable image-signing key added here (`KEXEC_SIG` + `SYSTEM_TRUSTED_KEYS`) is a building block the Secure Boot track reuses.
- **Stable image-signing key = key management now.** Preserving kexec OTA forces a long-lived image key (storage, rotation, CI access), which the "accept full-reboot fallback" option avoided (C-1). This is the cost of the gate decision; it is reversible (set updater default `kexec=off`).
- **arm64 OTA always full-reboots.** gokrazy has no arm64 kexec (`reboot.go`), so image signing is amd64-only; arm64 still gets lockdown + KASLR + dmesg + module signing (C-2).
- Confidentiality mode is deliberately out of scope (would break perf / tracing-BPF and likely `/proc/config.gz`).
- On Raspberry Pi 5, pwm-fan must be built in or signed under lockdown; the upstream gokrazy ability to unload it for custom fan control is lost on hardened images.
- The `.require` floor cannot assert string-valued config (`CONFIG_LSM`), `=n` (`# CONFIG_KEXEC is not set`), nor "confidentiality not forced"; the runtime doctor check + a dedicated resolved-`.config` grep test are the authoritative guarantees (R-7, C-5).
- `KEXEC_SIG` (not `KEXEC_SIG_FORCE`) is used; signature enforcement relies on lockdown being active. If lockdown ever regressed to `none`, unsigned kexec would be permitted (C-12). Consider `KEXEC_SIG_FORCE` for belt-and-suspenders.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Ephemeral per-build key for MODULES only | Build all needed modules in (`=y`) + rely on lockdown's own unsigned-module block | User chose module signing for defense-in-depth; ephemeral is valid because signer = verifier = same build |
| Stable release key for the kernel IMAGE (amd64) | Ephemeral image key (impossible — C-1); or accept full-reboot fallback | kexec_file_load verifies the new image against the OLD running kernel's keyring, so the image signer must be stable across builds |
| Lockdown runtime-only | Lockdown on both kernels | Installer is ephemeral; lockdown adds little there |
| Build VFIO in even though we also force module signing | Sign the VFIO module | A built-in driver has nothing to sign and cannot be rejected; signing stays a backstop, not the functional path (R-6) |
| Runtime doctor check as the real "lockdown active" guarantee | Rely on the build-time floor | The `=y`-only floor cannot see the LSM string or the forced mode (R-7) |

## Critical Review

Two rounds: a self-review and an independent, web-verified review (kernel.org docs,
`kernel_lockdown.7`, keyrings patchwork, Cloudflare blog) cross-checked against the codebase.
Severity: BLOCKER (would mislead an implementer into a broken/no-op build), ISSUE (a real
gap), NOTE (worth stating). The C-N numbering here is the one referenced throughout the spec.

### Verified correct (load-bearing claims confirmed, not problems)
- **kexec verification works WITHOUT Secure Boot:** `kexec_file_load` verifies against the running kernel's `.builtin_trusted_keys`/`.secondary_trusted_keys`; the `.platform` (Secure Boot) keyring is an *additional*, not *required*, source. A cert embedded via `CONFIG_SYSTEM_TRUSTED_KEYS` suffices. The "sign now, Secure Boot later" split is sound (A-4b).
- **sbsign is correct for the kernel image** (`KEXEC_BZIMAGE_VERIFY_SIG` verifies a PE/COFF Authenticode signature; `scripts/sign-file` is modules-only).
- **eBPF under integrity (A-1), `LOCKDOWN_LSM_EARLY`+`FORCE_INTEGRITY` forcing integrity (A-6), module-key auto-generation + `MODULE_SIG_ALL` semantics, and the Kconfig symbol names** check out.

### Findings and resolutions
| # | Severity | Finding | Resolution in spec |
|---|----------|---------|--------------------|
| C-1 | BLOCKER | Ephemeral per-build key is incompatible with signed-kexec OTA: kexec verifies the NEW image against the OLD running kernel's keyring, which only trusts a key it embedded at its own build. A throwaway key makes every OTA fail → silent full-reboot; AC-6 unsatisfiable. | Split keys: ephemeral for MODULES, **stable release key for the IMAGE** (see "Key strategy"); scope decision table + A-4b + Known Limitations updated; image no longer "signed with the same key." |
| C-2 | BLOCKER | arm64 gokrazy never kexecs (`reboot.go`, `!amd64`, ignores `tryKexec`), so the entire arm64 image-signing track is dead code. | Image/kexec signing scoped **amd64-only**; arm64 gets lockdown+KASLR+dmesg+module-signing; A-7 confirmed-by-code; AC-1b, arm64 QEMU test, Coverage row updated. |
| C-3 | BLOCKER | The documented signing order is impossible: the module pubkey is embedded into `vmlinux` DURING `build_kernel`, and `MODULE_SIG_ALL` signs DURING `modules_install` (inside `copy_runtime_outputs`, build.py) — so "keygen after 384, destroy before 389" yields an unembedded key and destroys it before signing. | Data Flow rewritten: provision certs BEFORE `build_kernel`; sign during/after `modules_install`; destroy ephemeral key LAST. Implementation step 2 + Files-to-Modify build.py line corrected. |
| C-4 | ISSUE | AC-1 listed `KEXEC_SIG` but omitted `KEXEC_BZIMAGE_VERIFY_SIG`; on x86, `KEXEC_SIG` alone gives no bzImage verify method, so kexec is refused under lockdown. | AC-1 now requires `KEXEC_BZIMAGE_VERIFY_SIG=y` (amd64) + `SYSTEM_TRUSTED_KEYS`; `kernel-lockdown-config.ci` asserts it. |
| C-5 | ISSUE | "No `CONFIG_KEXEC=y`" is true of the fragment but FALSE of the resolved config — `make defconfig` (x86_64_defconfig) sets `CONFIG_KEXEC=y` (legacy kexec_load). Same fragment-vs-resolved trap the spec warns about. | Add `# CONFIG_KEXEC is not set` to the runtime fragment; note the `=y`-only floor can't enforce a `=n` (R-7). |
| C-6 | ISSUE | arm64 image-sign prerequisites (`CONFIG_EFI`, `EFI_STUB`, `SIGNED_PE_FILE_VERIFICATION`) were unstated. | Moot after C-2 (arm64 image signing dropped); recorded so the arm64 track is not silently underspecified if ever revisited. |
| C-7 | ISSUE | A new `appliance-lockdown` doctor check breaks `TestDevSetupMatchesDoctor` (`dev_setup_drift_test.go`) unless it is in `dev-setup.py` or the `buildArtifactChecks` skip-set. | Added `dev_setup_drift_test.go` to Files-to-Modify: classify `appliance-lockdown` as a runtime (not build-prereq) check via the skip-set. |
| C-8 | ISSUE | Existing appliance doctor checks are build-HOST prerequisites; a lockdown check reads securityfs and would warn on every dev run (absent on macOS, `[none]` on plain Linux). | Gate the check on a running-on-appliance signal; no-op (not warn) off-appliance. Files-to-Modify doctor line updated. |
| C-9 | ISSUE | AC-7/R-3 grepped the wrong volume: the reused build tree + `certs/` live in `/tmp/kbuild` → `ze-kernel-build` volume, not `ze-kernel-work` (`/build`). | AC-7 + R-3 now target `ze-kernel-build` + the in-tree `linux-<ver>-yes/certs/`. |
| C-10 | NOTE | `/proc/kcore` is a confidentiality restriction, not integrity (only `/dev/mem`+`/dev/kmem` are integrity). | Task wording corrected. |
| C-11 | NOTE | The securityfs parse example `[none] integrity [confidentiality]` is malformed (the file brackets only the ACTIVE token). | Unit-test row corrected to `none [integrity] confidentiality`. |
| C-12 | NOTE | Spec uses `KEXEC_SIG`, not Cloudflare's `KEXEC_SIG_FORCE`; equivalent only while lockdown is active. | Recorded in Known Limitations as a deliberate, reversible deviation. |

### Earlier self-review items (still tracked as risks, not duplicated above)
- `=y`-only floor cannot see the LSM string / forced mode → **R-7** (doctor check is authoritative).
- `MODULE_SIG_FORCE` vs VFIO → **R-6** (build VFIO in; signing is a backstop).
- Silent OTA degradation if kexec fails → **R-2** (AC-6 asserts fallback not taken; surface gokrazy's log).
- `DMESG_RESTRICT` hiding field diagnostics → **R-4**.
- Repo kernel version is `7.x` (unusual); lockdown/sig stable since ~5.4 but confirm exact symbol names at implementation.
- This spec does NOT deliver at-rest integrity → **Known Limitations** (Secure Boot / dm-verity follow-on).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

> This spec is a **design deliverable, not scheduled for implementation**. The gates below
> are the contract a future implementer must satisfy. None are ticked.

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Doctor check `doctor-appliance-lockdown` registered + explainable; QEMU evidence green
- [ ] Registration over hardcoding — doctor check registers and is core-discovered; no core/shared switch added
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Quality Gates (SHOULD pass)
- [ ] Resolved `.config` byte-diff shows only the intended hardening symbol changes
- [ ] Built image AND `ze-kernel-work` volume contain no private signing key (AC-7)
- [ ] Risks & Assumptions: every A-N confirmed or broken (A-3..A-7 require QEMU evidence)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] QEMU evidence for lockdown-active + OTA-kexec + arm64
- [ ] Functional `.ci` tests for the floor + resolved-config assertions

### Post-wave corrections (2026-07-10)

Registry-context refresh only, no design change. Re-verified after the followup-vpp-iface wave:

- `internal/core/diagnostic/codes.go` grew: the wave registered `doctor-vpp-wireguard` (codes.go) and `doctor-vpp-lcp-netns` (codes.go). The planned `doctor-appliance-lockdown` code therefore joins a registry that keeps growing by append; the registration mechanism this spec relies on (add a code entry to codes.go, register the check via `diagnostic.RegisterDoctorCheck()` from the owning package) is unchanged and freshly exercised by those two additions, which are current working examples to copy.
- DPDK/VFIO citations hold: `vfioModules` (`vfio`, `vfio_pci`, `vfio_iommu_type1`) still at `internal/component/vpp/dpdk.go` and `loadModuleLinux` modprobe still at `internal/component/vpp/dpdk_linux.go`; the wave did not touch either file, so R-6 and the build-VFIO-in decision stand as written.
