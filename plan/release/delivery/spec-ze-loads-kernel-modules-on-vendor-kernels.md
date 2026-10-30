# Spec: ze-loads-kernel-modules-on-vendor-kernels

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Priority | P2 |
| Release reason | Delivery: `ze install systemd` and the planned DEB and RPM packages install ze on Debian, Ubuntu, Rocky and Fedora, whose kernels ship the features ze needs as modules. Today a feature whose module is not loaded either refuses to start with no load path, or (conntrack helpers, the flowtable, the ingress policer) is accepted and does nothing. Installation is not usable until ze loads what its configuration needs and says what it could not load |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner Questions

Each row is a choice no RFC, agreed design or `ai/META-DECISIONS.md` rule answers. The design below is written to the recommended answer and names what changes under the other.

→ Decision: Thomas approved every recommendation on 2026-10-29: Q-1 (a), Q-2 (a), Q-3 (a), Q-4 (a). The design below stands as written.

| # | Question | Options | Recommendation and why | What the other answer changes |
|---|----------|---------|------------------------|-------------------------------|
| Q-1 | A vendor kernel whose `/etc/os-release` names none of Debian, Ubuntu, Fedora, Red Hat (and rebuilds), Arch Linux or Alpine | (a) attempt the load anyway, log a WARN naming the ID, report it in `ze doctor`; (b) refuse to load, with what, why and the fix | (a). Loading is `modprobe -- <name>`, which behaves the same on every kmod system, and the start gate still refuses if the module did not arrive. A refusal turns a host that would work (openSUSE, Gentoo, Void) into one that refuses to start, and its only fix is `sudo modprobe` by hand, the same command ze would have run | AC-11 expects a refusal naming the unrecognised ID and the manual `sudo modprobe -- <module>` lines; the loader step 3 stops there |
| Q-2 | Does the feature need an operator switch, now that it acts only on vendor kernels? | (a) YANG leaf `environment daemon load-kernel-modules`, boolean, default `true`; (b) no switch at all; (c) the earlier proposal: leaf default `false` | (a). `ai/rules/config.md` makes a tunable a YANG leaf. Default on, because ze loads only modules the vendor ships and signs, only for features the operator's own configuration uses, and only off Ze's kernel; with it off, the start refusal's fix is the operator typing the same `modprobe`. The leaf keeps the feature optional (the META rule says "optional feature") for hosts that manage modules themselves. Default off would make every distro install refuse to start until the operator finds the leaf | (b): AC-12 and the env plumbing row go. (c): AC-3 and AC-14 need the leaf set first, and the refusal text names the leaf before `modprobe` |
| Q-3 | The shipped unit runs ze as user `ze` with only `CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE` (`capabilities`, `internal/plugins/systemd/unit.go`), so the daemon itself can never load a module. How does a systemd-run ze load? | (a) the unit gains `ExecStartPre=+<ze> kernel-module load` and the same line before the `kill -HUP` in `ExecReload`: a short-lived root step, the daemon's privileges unchanged; (b) add `CAP_SYS_MODULE` to the unit's ambient and bounding sets; (c) neither: the daemon refuses with the `sudo modprobe` lines and the operator loads by hand | (a). `CAP_SYS_MODULE` lets a compromised daemon load any module the host carries, which is root in all but name; (a) holds root only for the one bounded step that reads the configuration and runs modprobe. `spec-release-distribution.md` makes the packaged unit keep "the existing capability and hardening contract from `buildUnitFile`", so the choice reaches the packages through that one function | (b): no new command; AC-15 checks the capability sets instead, and the reload path loads in-daemon (AC-13 becomes a load). (c): AC-14 and AC-15 go, and the distro install path always ends in a manual step |
| Q-4 | Proof on a vendor kernel. `ai/META-DECISIONS.md` says VMs and test guests run Ze's kernel, never a stock distribution kernel, and that a test needing a kernel feature runs on Ze's kernel. This feature exists only off Ze's kernel, so no allowed guest can prove it | (a) run the vendor proof in the booted Debian 12, Ubuntu 24.04, Rocky 9 and Fedora VMs that `spec-release-distribution.md` AC-21 already needs for package evidence, as an exception for this feature; (b) a GitHub Actions `ubuntu-latest` runner (Ubuntu kernel, passwordless sudo); (c) no vendor-kernel proof: fixture tests only | (a). Those VMs must exist for the package matrix anyway, they cover three of the six families, and the proof runs the packaged unit end to end. (b) proves Ubuntu only and on a cloud kernel flavour. (c) leaves the real `modprobe`, signing and packaging behaviour (A-6, A-7, A-9) unproven, which `ai/rules/principles.md` does not allow for a claim of support | (b): AC-24 runs in a workflow job on the runner. (c): AC-24 is dropped, and the support claim for every distro stays "implemented, unverified" in `docs/guide/ze-install.md` |

## Task

Ze running on a standard distribution kernel loads the kernel modules its configuration needs with `modprobe`, as a feature of `ze` itself (`ai/META-DECISIONS.md`, "Test systems and labs"). `le` never carries it.

Owner decision built in (Thomas, relayed 2026-10-29): the loader detects the running kernel and loads only when it is NOT Ze's kernel. Ze's kernel builds the features in, so on it ze loads nothing. Supported vendor kernels: Debian, Ubuntu, Fedora, Red Hat (RHEL and its rebuilds), Arch Linux and Alpine.

What the work settles:

1. How ze tells its own kernel from a vendor kernel, from a marker the kernel build already guarantees.
2. How it identifies the vendor, and what it does on an unrecognised one (Q-1).
3. Which modules each feature needs, declared by the owning subsystem on its own `kernelcap.Capability` (registration, no central list).
4. When loading runs: daemon start before the capability gate and before the privilege drop; the systemd pre-step (Q-3); what a reload does with no privilege.
5. Whether an operator switch exists (Q-2).
6. `ze doctor` reporting.
7. The enrolment gap: the firewall flowtable and the traffic ingress policer register no kernel capability today.
8. Replacing, not layering: four product `modprobe` call sites exist today and each moves onto the one loader (`ai/rules/no-layering.md`).
9. Tests: unit tests over fixture inputs, functional tests through the real entry point, and a vendor-kernel proof (Q-4).

The starting proposal was `tmp/session/2026-10-29-105ec564-b63f-4d37-8f4c-87229450eb11/scratch/kmod/ze-modprobe-design.md`. Its citations were re-read; the corrections are in Design Insights.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/doctor-and-health-checks.md` - the kernel capability tier, "Four callers, one verdict", the test-only force variable
  → Decision: one enrolment yields four readers (doctor, daemon start, reload, `ze config validate`); the loader is a fifth action on the same enrolment, not a second registry
  → Constraint: "There is no operator override, by owner decision (2026-08-14)". The `load-kernel-modules` leaf is not an override: with it off the refusal stands
  → Constraint: a probe MUST NOT exec a binary (`Capability.Probe` comment, `internal/component/kernelcap/kernelcap.go`); the loader is the only exec, and it is not a probe
- [ ] `docs/architecture/appliance/kernel-profiles.md` - how the runtime profile is assembled from `kernel.config` plus `runtime.config`, and what `.require` enforces
  → Constraint: a `.require` entry must resolve to `=y` (`internal/appliance/kernelreq.go`, the "did not resolve to =y" refusal), so the manifest can hold only boolean symbols. `CONFIG_LOCALVERSION` is a string and cannot be required there
- [ ] `ai/doors/hub-engine.md` - start order and the one reload path
  → Constraint: "`ze` never links `internal/le`" (`TestNormalZeLinksNoInternalLe`), so the `-ze` suffix constant in `internal/le/interoplab/kernelcheck.go` cannot be reused by ze; the fact moves to a core package both import
  → Constraint: "One reload path": SIGHUP and a managed push both reach `doReload`
- [ ] `ai/doors/appliance.md` - the appliance boots Ze's kernel only
  → Constraint: gokrazy ships no `modprobe` (`internal/appliance/kernelreq.go` comment: "gokrazy ships no modprobe, so a transform built as a module is never loaded")
- [ ] `ai/rules/config.md` - leaf before env var; names derivable across YANG, env, Go and CLI
  → Decision: the switch is a leaf (Q-2), overridden by `ze.daemon.load-kernel-modules`
- [ ] `ai/rules/user-facing-errors.md` - what, why, the exact command; refuse before the work starts
  → Constraint: every refusal below ends in a command on its own line
- [ ] `ai/rules/platform-linux.md` - QEMU proofs boot Ze's kernel; `needs-linux` and `caps=` markers
  → Constraint: the functional tests here do not need a real kernel feature (the kernel identity, the modprobe path and the module lists come from fixtures), so they run natively; only AC-24 needs a vendor kernel (Q-4)

### RFC Summaries (Scope: protocol)
- N-A: no protocol behaviour changes. The capabilities named here serve protocols, but this work changes only whether their kernel support is present.

**Key insights:**
- The marker is the release suffix `-ze`, set by `CONFIG_LOCALVERSION="-ze"` in `gokrazy/kernel/runtime.config` line 4, observed as `lib/modules/7.2.9-ze` in the cached runtime build. Two consumers already rely on it (`internal/le/interoplab/kernelcheck.go`, `zeKernelReleaseSuffix`; `internal/le/setup/docker/kernel/dockerkernel.go`, `kernelLocalVersion`). Nothing makes the build guarantee it yet.
- The shipped systemd unit never runs ze as root, so "load before the privilege drop" alone never fires on a distro install (Q-3).
- Four product modprobe call sites exist, not one.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/kernelcap/kernelcap.go` (Capability, MustRegister, Evaluate, ProbeAll, Refuse) - the enrolment: `Subsystem`, `Component`, `Kernel` (the `CONFIG_` symbol), `ConfigLeaf`, `Degrades`, `CodeAbsent`, `CodeUnknown`, `Order`, `InUse`, `Probe`. No module field
- [ ] `internal/component/kernelcap/modules_linux.go` (LoadedModules) - reads `/proc/modules`, nil for unreadable, empty for none; overridable by `ze.test.doctor.modules-file`
- [ ] `internal/component/kernelcap/probe.go` (ProcPath, forceEnv, ForceAnswersForTest) - every procfs read goes through `ProcPath`, overridable by the private proc-root variable; `ze.test.kernelcap.force` exists only in a `zetest` build
- [ ] `cmd/ze/hub/main.go` (runYANGConfig) - `kernelcap.Refuse(loadResult.Tree)` at line 477 on the post-evolution tree; `dropPrivileges()` at line 1045, after the PID file
- [ ] `cmd/ze/hub/main_reload.go` (runReloadContext) - the same gate at line 473, before `ReloadConfig`, with the running configuration still serving
- [ ] `cmd/ze/hub/main_system.go` (dropPrivileges, applyConntrackConfig) - drop is a no-op when `ze.user` is unset; conntrack helpers are loaded at start and at reload
- [ ] `internal/core/privilege/drop_unix.go` (Drop) - `Setgid` then `Setuid`; no capability is kept
- [ ] `internal/plugins/systemd/unit.go` (buildUnitFile, capabilities) - `User=ze`, `Group=ze`, `AmbientCapabilities=` and `CapabilityBoundingSet=` both `CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE`, `NoNewPrivileges=true`, `ExecReload` sends SIGHUP through the shell
- [ ] `internal/component/config/system/conntrack_linux.go` (LoadConntrackModules) - modprobe of `nf_conntrack_<helper>`; returns `nil, nil` when no modprobe is on PATH; `readLoadedModules` returns an empty map on a read error
- [ ] `internal/component/l2tp/kernel_linux.go` (probeKernelModules) and `internal/component/l2tp/subsystem.go` - modprobe `l2tp_ppp` then `pppol2tp`, 10 s each, failing the subsystem start; bypassed by `ze.l2tp.skip-kernel-probe` (54 files under `test/`, `internal/`, `cmd/` name that variable)
- [ ] `internal/component/vpp/dpdk_linux.go` (loadModuleLinux) and `internal/component/vpp/dpdk.go` (vfioModules, loadVFIOModules) - modprobe of `vfio`, `vfio_pci`, `vfio_iommu_type1`, no timeout
- [ ] `internal/plugins/flowexport/conntrack_setup_appliance_linux.go` (loadConntrackModules) - `ze_appliance` build only; modprobe of `nf_conntrack`, `nf_conntrack_netlink`, failures at debug
- [ ] `internal/plugins/firewall/nft/doctor_linux.go` - a doctor check that `nf_tables` is in `/proc/modules`; no kernelcap enrolment
- [ ] `internal/plugins/traffic/netlink/policer_linux.go` - installs `act_police` on the shared ingress hook; no kernelcap enrolment
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` line 914 - `list flowtable`, nft backend only
- [ ] `internal/le/interoplab/kernelcheck.go` (zeKernelReleaseSuffix) and its test `TestZeKernelReleaseSuffixIsTheRuntimeConfigs`
- [ ] `internal/component/host/kernel_linux.go` (DetectKernel) - already reads `/proc/sys/kernel/osrelease` for the host inventory
- [ ] `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`, `gokrazy/kernel/patches/0002-mpls-ip-mtu.patch`

**Behavior to preserve:**
- The four-caller verdict and its texts for a capability that is absent on Ze's kernel.
- `ze doctor` and `ze config validate` never change the host.
- A reload refused by the capability gate leaves the running configuration serving.
- The force variable stays `zetest`-only; no shipped build reads a test override.
- `system conntrack module` keeps loading helpers on a distro kernel.

**Behavior to change:**
- A capability enrolment declares where its kernel support comes from: named modules, built in only, or Ze's kernel only.
- Off Ze's kernel, an in-use capability's modules that are neither loaded nor built in are loaded before the start gate judges.
- The l2tp, vpp, conntrack-helper and flow-export modprobe calls are deleted; their modules become declarations on enrolments.
- The firewall flowtable, the traffic ingress policer, the traffic qdiscs and classifiers, `nf_tables`, flow sampling, mirroring, flow-export conntrack, the conntrack helpers and VPP's VFIO get enrolments.
- The unit `ze install systemd` writes gains the root pre-step (Q-3).
- `system conntrack module` stops offering `nfs` and `sqlnet`, which no Linux helper provides, and Ze's kernel builds in the helpers it still offers.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Daemon start: `ze start`, in `runYANGConfig`, on the post-evolution tree, before `kernelcap.Refuse`.
- The systemd pre-step: `ze kernel-module load` (new local command, Q-3), run as root by `ExecStartPre=+` and in `ExecReload` before the SIGHUP.
- Read-only readers that report and never load: `ze doctor`, `ze config validate`, the reload gate.

### Transformation Path
1. Kernel identity: read `/proc/sys/kernel/osrelease` through `ProcPath`. A release ending in the core suffix constant (`-ze`) is Ze's kernel: stop, load nothing.
2. Vendor identity: read `/etc/os-release`, else `/usr/lib/os-release` (the os-release(5) search order), fields `ID` and `ID_LIKE`. Classify into one family: `debian` (ID `debian`, or `debian` in ID_LIKE: Ubuntu), `rhel` (ID `rhel`, `fedora`, or `rhel`/`fedora` in ID_LIKE: Rocky, AlmaLinux, CentOS Stream, Oracle), `arch`, `alpine`, or unrecognised (Q-1). The family chooses only the package names in a fix line; it never changes which modules load.
3. Loader preconditions, each a refusal reason when it fails: the leaf is on (Q-2); the process holds `CAP_SYS_MODULE` in its effective set (`/proc/self/status`, `CapEff`); `/proc/sys/kernel/modules_disabled` reads `0`; `/lib/modules/<release>/` exists (absent in a container image); the modprobe path read from `/proc/sys/kernel/modprobe` exists and is executable.
4. Needed set: every enrolled capability whose `InUse(tree)` answers true and whose source is "modules"; collect its declared module names in declaration order.
5. Skip what is present: a name found in `/proc/modules` (`LoadedModules`) or in `/lib/modules/<release>/modules.builtin` is skipped. Names compare with `-` and `_` treated as equal, as modprobe and `/proc/modules` do.
6. Load: for each remaining name, exec `<modprobe path> -- <name>` with a 10 s bound per call (the bound l2tp and flow-export use today), capture stderr, log INFO on success with subsystem, module and config leaf, WARN on failure with the same and the stderr.
7. Judge: `kernelcap.Refuse` runs as today. Its absent and unknown texts gain a module line built from the loader's record: loaded, failed with stderr, or not attempted and why (step 3's reason), with the family's fix.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ze ↔ kernel | procfs and sysfs reads; `finit_module` happens inside the vendor's `modprobe`, never in ze | No |
| ze ↔ host userland | exec of the kernel's configured modprobe path, the one exec in the feature | No |
| systemd ↔ ze | `ExecStartPre=+` and `ExecReload` lines in the generated unit | No |
| Subsystem ↔ kernelcap | each owner's `register_kernelcap_linux.go` declares its modules on its own enrolment | No |

### Integration Points
- `kernelcap.Capability` gains a module-source declaration; `MustRegister` refuses an enrolment that leaves it unspecified.
- `kernelcap.Refuse` and the doctor check render the loader's record.
- `runYANGConfig` calls the loader once before `Refuse`.
- `buildUnitFile` writes the pre-step lines; `checkServiceExecutable` and the unit doctor checks read them back.
- `env.MustRegister` for `ze.daemon.load-kernel-modules`; the env plumbing table maps the leaf.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the loader acts only through the enrolment the four readers already share; no subsystem execs modprobe after this |
| No unintended coupling (components stay isolated) | Yes | kernelcap knows no subsystem; each subsystem declares its own modules from its own `init()` |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses `LoadedModules`, `ProcPath`, `Refuse`; deletes four ad-hoc modprobe paths |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire path |
| Registration over hardcoding, outbound | Yes | the module list lives on each `Capability`, registered by its owner |
| Registration over hardcoding, inbound | Yes | searched: `vfioModules` (vpp), the `l2tp_ppp`/`pppol2tp` array (l2tp), the flow-export literal pair, `toKernelModName` (conntrack); each becomes a declaration on that owner's enrolment, and the loader iterates `capabilities.entries` only |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every kernel Ze builds that runs the daemon ends its release in `-ze` | `gokrazy/kernel/runtime.config` line 4; cached build `tmp/kernel/build/lib/modules/7.2.9-ze`; installer profiles under `tools/installer-kernel/` set no suffix but run `ze-installer`, never the daemon | ze on its own kernel would load modules, against the owner decision | AC-23's build-time check plus `TestZeKernelReleaseSuffixIsTheRuntimeConfigs` moved to the core package | unvalidated |
| A-2 | No supported vendor kernel's release ends in `-ze` | vendor release forms (Debian `-amd64`, Ubuntu `-generic`, Fedora `.fcNN.`, RHEL `.elN`, Arch `-archN-N`, Alpine `-lts`/`-virt`): from memory, unverified | a vendor kernel would be treated as Ze's and load nothing; the start gate still refuses, so the failure is visible | read `uname -r` in each AC-24 VM and in each family's kernel package name | unvalidated |
| A-3 | `CONFIG_LOCALVERSION_AUTO=y` in the runtime build adds nothing to the release, because the source is a release tarball and not a git tree | cached build config sets `LOCALVERSION_AUTO=y`; `kernel.version` names `source-url=https://cdn.kernel.org/.../linux-7.2.9.tar.xz`; observed release `7.2.9-ze` | a git-tree build would yield `7.2.9-ze-gHASH` or `7.2.9-ze+` and fail the suffix match | AC-23's build-time check reads the built release | unvalidated |
| A-4 | os-release IDs | containers read 2026-10-29: Debian 13 `ID=debian`; Ubuntu 24.04 `ID=ubuntu ID_LIKE=debian`; Fedora 44 `ID=fedora`; UBI 9.8 `ID="rhel" ID_LIKE="fedora"`; Rocky 9.3 `ID="rocky" ID_LIKE="rhel centos fedora"`; Alpine 3.24.3 `ID=alpine`; Arch `ID=arch`, no ID_LIKE (archlinux `filesystem` package, `os-release` at `main`). AlmaLinux, CentOS Stream and Oracle Linux not read | a rebuild would fall into Q-1's branch | unit fixtures copied from these files; read AlmaLinux and Oracle images | validated (read), except AlmaLinux, CentOS Stream, Oracle |
| A-5 | modprobe lives at `/sbin/modprobe`, reached through `/proc/sys/kernel/modprobe` | Debian 13, Ubuntu 24.04, Fedora 44, UBI 9.8, Rocky 9.3: `/sbin` is a symlink to `usr/sbin`; Alpine 3.24.3: `/sbin/modprobe -> /bin/busybox`. None of the non-Alpine images installs kmod at all. The `/proc/sys/kernel/modprobe` default and its meaning are from memory of `Documentation/admin-guide/sysctl/kernel.rst`, unverified; Arch's layout unverified (no arm64 image) | ze would exec a wrong path; AC-9's refusal covers absence but not a wrong binary | read the kernel doc at 7.2; read `/proc/sys/kernel/modprobe` in each AC-24 VM | unvalidated |
| A-6 | Vendor kernels ship the modules ze needs as `=m` | primary configs fetched 2026-10-29, table in Design Insights: Debian `debian/latest`, Ubuntu noble annotations, Fedora rawhide, CentOS Stream c9s and c10s RHEL configs, Arch `main`, Alpine `linux-lts` lts and virt | a feature refuses on that distro with "not shipped" | AC-24 per distro | validated (config level) |
| A-7 | A module built `=m` may still be absent from the default install: RHEL 9 packages `l2tp`, `mpls` (netprots) and `ppp` (netdrvs) into `kernel-modules-extra` | `filter-modules.sh.rhel` on CentOS Stream `c9s`, lines 24 and 38. Fedora, RHEL 10 and Ubuntu's `linux-modules-extra` split: unverified (files not found at the paths tried) | the fix line names the wrong package | read the c10s and Fedora filter scripts; `dpkg -S act_police` on Ubuntu 24.04 | unvalidated (RHEL 9 read) |
| A-8 | Alpine's busybox modprobe loads the modules Alpine's kernel ships, including any compression it uses | unverified: the Alpine config files set no `MODULE_COMPRESS_*`, and the APKBUILD may compress at package time | ze on Alpine without `kmod` fails to load | `apk add linux-lts` in an Alpine VM, list `/lib/modules/*/kernel/net/sched/act_police.ko*`, run busybox `modprobe act_police` | unvalidated |
| A-9 | Secure Boot does not block vendor modules: every non-Alpine family sets `MODULE_SIG=y`, `MODULE_SIG_FORCE=n`, `SECURITY_LOCKDOWN_LSM=y`, `LOCK_DOWN_KERNEL_FORCE_INTEGRITY=n` (configs above); vendor modules are signed with the vendor key, so lockdown's integrity mode accepts them | config values read; that these distros raise lockdown when booted under Secure Boot, and that their in-tree modules carry signatures, is from memory, unverified | loading fails under Secure Boot with an `EKEYREJECTED`-style stderr | AC-24 on one VM booted with Secure Boot (OVMF with enrolled keys) | unvalidated |
| A-10 | A process with `CAP_NET_ADMIN` alone can trigger kernel module autoload for some modules (tc action kinds, generic netlink families), so some modules arrive without the loader | unverified: from memory of `request_module` calls in `net/sched/act_api.c` and `net/netlink/genetlink.c` | none for correctness: the loader acts on what is absent at its run and the gate re-judges; it matters only for which AC-24 assertions are discriminating | AC-24 records `/proc/modules` before and after; read both files at 7.2 | unvalidated |
| A-11 | A container image has no `/lib/modules/<release>` for the host kernel, and Docker's default capability set excludes `CAP_SYS_MODULE` | the slim images above carry no kmod; Docker's default set is from memory, unverified | ze in a container would attempt and fail; the refusal text still holds | read moby `oci/caps/defaults.go`; run AC-10 in a container | unvalidated |
| A-12 | `/proc/sys/kernel/modules_disabled` set to 1 cannot be cleared until reboot | from memory of `Documentation/admin-guide/sysctl/kernel.rst`, unverified | the fix line would offer a wrong step | read the kernel doc at 7.2 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The pre-step judges a different tree from the daemon's (pre-evolution, or a different config dir) | AC-14 loads nothing yet the start passes, or loads and the start refuses | the pre-step calls the same tree loader `runYANGConfig` uses, post-evolution; AC-14 asserts the loaded list equals the start gate's in-use list |
| R-2 | A hung modprobe stalls the start | start exceeds the systemd `TimeoutStartSec` | 10 s bound per call; the record says "timed out after 10 s" |
| R-3 | A module name differs across vendors (`pppol2tp` vs `l2tp_ppp`, `-` vs `_`) | AC-24 "not shipped" on one distro with the feature built | names come from Ze kernel's `modules.builtin`; `-`/`_` normalisation; l2tp's old two-name fallback is checked against each AC-24 distro before deletion |
| R-4 | Deleting `ze.l2tp.skip-kernel-probe` touches 54 files | a wide diff with test edits | keep the variable's job where it is still needed (it also bypasses CAP_NET_ADMIN needs); only the modprobe goes. Any `.ci` edit that changes what a test proves goes to `ze-test-judge` |
| R-5 | The unit change collides with `spec-release-distribution.md`'s packaged unit | that spec's AC on unit equality fails | the packaged unit derives from `buildUnitFile` by that spec's own rule; this spec edits only that function |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | On a distro: ze loads a module it did not need (harmless, vendor-shipped) or fails to load one (start refused with the fix, as today). On Ze's kernel: nothing, if A-1 holds; if it fails, ze attempts a modprobe gokrazy does not ship and the refusal names it |
| How is it reverted? | Single commit per phase. The YANG leaf removal and the conntrack enumeration change are config-visible; Ze is pre-release, so no migration is owed |
| Who else touches this path? | `spec-release-distribution.md` (packaged unit from `buildUnitFile`); `spec-privileged-tests-run-in-a-user-namespace.md` (le setup's `modprobe ppp_generic`, dev tooling, unaffected); `plan/post-release/engineering/spec-lab-kernel-features-in-setup.md` (le setup for labs, dev tooling, unaffected) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze start` on a vendor kernel, policer configured | → | loader before `kernelcap.Refuse` in `runYANGConfig` | `test/plugin/kernel-module-load-vendor-kernel.ci` |
| `ze start` on Ze's kernel | → | identity check stops the loader | `test/plugin/kernel-module-ze-kernel-loads-nothing.ci` |
| `ze start` without `CAP_SYS_MODULE` | → | precondition record rendered by `Refuse` | `test/plugin/kernel-module-unprivileged-refuses.ci` |
| reload into a config newly using the policer | → | `runReloadContext` gate with the loader's record | `test/plugin/kernel-module-reload-refuses.ci` |
| `ze kernel-module load` | → | the same loader, run as the pre-step | `test/plugin/kernel-module-load-command.ci` |
| `ze install systemd` | → | `buildUnitFile` pre-step lines | `TestUnitFileLoadsKernelModulesAsRootBeforeStart` (`internal/plugins/systemd/unit_test.go`) |
| `ze doctor` | → | doctor check rendering identity, loader and module states | `test/ui/doctor-kernel-modules.ci` |
| `ze start` on Debian 12's own kernel, as packaged | → | real `modprobe -- act_police` | `TestLoaderLoadsActPoliceOnVendorKernel` (`internal/component/kernelcap/loader_vendor_integration_linux_test.go`), run per Q-4 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Release ends in `-ze`; a configured feature's capability is absent | no exec of any binary; the refusal is today's text, plus a line saying Ze's kernel builds `<CONFIG_X>` in and this build lacks it, and the existing kernel-rebuild fix |
| AC-2 | Release ends in `-ze`; `/etc/os-release` says `ID=debian` (Ze's kernel on a Debian userland, as after `le setup docker kernel install`) | no exec; the kernel identity decides, not os-release |
| AC-3 | Vendor kernel (Debian fixture), root, leaf on, an interface carries an ingress policer, `act_police` neither loaded nor built in | exactly one exec, `<modprobe path> -- act_police`, before the gate; INFO `kernel module loaded` with subsystem, module and config leaf; the start proceeds when the probe then answers present |
| AC-4 | Same, `act_police` listed in `/proc/modules`, or in `modules.builtin` | no exec |
| AC-5 | Vendor kernel, no configured feature declares modules | no exec, no log line |
| AC-6 | Vendor kernel, modprobe exits non-zero (module not shipped) | start refused; the text names the subsystem, the config leaf, the module, the release, modprobe's stderr, and the family's fix on its own line (RHEL family: `sudo dnf install kernel-modules-extra-<release>`; Debian family: the kernel-package line; Arch and Alpine: the kernel package name) |
| AC-7 | Vendor kernel, process lacks `CAP_SYS_MODULE` at start | no exec; the refusal lists `sudo modprobe -- <module>` for each missing module, one per line, and `sudo systemctl restart ze` |
| AC-8 | `/proc/sys/kernel/modules_disabled` is `1` | no exec; the refusal says module loading is disabled on this host until reboot, and names the modules to add to the host's boot-time module list |
| AC-9 | The path in `/proc/sys/kernel/modprobe` does not exist | no exec; the refusal names the family's kmod install command (`apt install kmod`, `dnf install kmod`, `pacman -S kmod`, `apk add kmod`) |
| AC-10 | `/lib/modules/<release>/` absent | no exec; the refusal says ze runs where the host's modules are not visible (a container), and names `sudo modprobe -- <module>` to run on the host |
| AC-11 | `/etc/os-release` names an unrecognised ID with no recognised ID_LIKE | per Q-1 (recommended): the load is attempted, WARN `unrecognised distribution` names the ID, and the fix lines are the family-neutral `sudo modprobe` form |
| AC-12 | Leaf `environment daemon load-kernel-modules false`, or `ze.daemon.load-kernel-modules=false` | no exec; the refusal names the leaf and the `sudo modprobe` lines |
| AC-13 | Running daemon without `CAP_SYS_MODULE`; reload into a config newly using the policer; `act_police` not loaded | reload refused, running configuration still serving; the text names `sudo modprobe -- act_police` and `sudo systemctl reload ze` |
| AC-14 | `ze kernel-module load` as root against the stored config | loads exactly the modules the start gate's in-use set declares, prints one line per module (loaded, present, failed with stderr), exits 0 whether or not a module failed (the daemon's gate is the refusal), exits non-zero only on a usage or config-read error |
| AC-15 | `ze install systemd` (Q-3 recommended) | the unit carries `ExecStartPre=+<binary> kernel-module load` and the same command before the SIGHUP in `ExecReload`; `User`, `Group`, the capability sets and `NoNewPrivileges` are unchanged; `ze doctor`'s unit checks accept it |
| AC-16 | `ze doctor` on a vendor kernel | one row per in-use capability that declares modules: built in, loaded, loadable (named in `modules.dep`), or not shipped; plus kernel identity, family and modprobe path; never execs |
| AC-17 | `ze config validate` | never execs; judges the host as today |
| AC-18 | A firewall table declares a flowtable; kernel lacks `NF_FLOW_TABLE_INET` or `NFT_FLOW_OFFLOAD` (forced absent) | start refused, doctor error, validate fails, naming `firewall-flowtable` and the flowtable's config path |
| AC-19 | An interface carries an ingress policer; `NET_ACT_POLICE` forced absent | same four readers refuse, naming `traffic-ingress-policer` |
| AC-20 | Vendor kernel, `rsvp-te` on the kernel FIB (needs `CONFIG_MPLS_IP_MTU`, a Ze kernel patch) | no exec for it; the refusal says only Ze's kernel provides `CONFIG_MPLS_IP_MTU`, and names running on Ze's kernel as the fix |
| AC-21 | Tree after the change | no `exec` of `modprobe` remains in `internal/` or `cmd/` outside `internal/le/` and the kernelcap loader: l2tp `probeKernelModules`, vpp `loadModuleLinux`, conntrack `LoadConntrackModules` and flow-export `loadConntrackModules` are deleted, and their modules are declarations |
| AC-22 | `system conntrack module ftp` on Ze's kernel | the helper is built in (`runtime.config` and `runtime.require`); no exec; the capability reads present. `nfs` and `sqlnet` are no longer accepted, and the parse error names the closest valid helper |
| AC-23 | The `-ze` suffix | declared once in a core package, consumed by kernelcap, `internal/le/interoplab` and `internal/le/setup/docker/kernel`; a test pins it to `runtime.config`; the runtime kernel build refuses an output whose release does not end in it |
| AC-24 | Debian 12, Ubuntu 24.04 and Rocky 9 VMs on their own kernels (Q-4), ze installed by package, config with an ingress policer and a flowtable | the unit starts; the journal shows the pre-step loading `act_police`, `nf_flow_table`, `nf_flow_table_inet`, `nft_flow_offload` (those absent before, per A-10's record); `tc filter show dev <if> ingress` lists the police action; the nft ruleset lists the flowtable. With the pre-step line removed, the same start is refused with AC-7's text |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Installs ze on Debian with `ze install systemd`, configures an upload policer, starts | systemd pre-step → loader → modprobe → daemon start → gate present → policer installed | AC-24, `TestLoaderLoadsActPoliceOnVendorKernel` |
| 2 | Runs ze on the appliance with the same config | identity `-ze` → loader stops → gate present (built in) | `test/plugin/kernel-module-ze-kernel-loads-nothing.ci` |
| 3 | Adds a flowtable and commits over SSH while ze runs unprivileged | reload gate → absent → refusal with `sudo modprobe` lines → operator runs them → commit succeeds | `test/plugin/kernel-module-reload-refuses.ci` |
| 4 | Asks why ze will not start | `ze doctor` row: vendor kernel, family, modprobe path, module not shipped, package to install | `test/ui/doctor-kernel-modules.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestKernelIdentityFromRelease` | `internal/core/kernelrelease/kernelrelease_test.go` | `7.2.9-ze` is Ze's; `6.12.48+deb13-amd64`, `6.8.0-45-generic`, `6.11.4-301.fc41.x86_64`, `5.14.0-427.el9.x86_64`, `6.11.5-arch1-1`, `6.6.58-0-lts` are not; empty and unreadable are "cannot tell", never "vendor" | |
| `TestZeKernelReleaseSuffixIsTheRuntimeConfigs` (moved) | `internal/core/kernelrelease/kernelrelease_test.go` | the constant equals `CONFIG_LOCALVERSION` in `gokrazy/kernel/runtime.config` | |
| `TestVendorFamilyFromOSRelease` | `internal/component/kernelcap/vendor_test.go` | fixtures copied from the A-4 reads classify to debian, debian (Ubuntu), rhel (Fedora, UBI, Rocky), arch, alpine; quoted and unquoted values; `/usr/lib/os-release` fallback; an unknown ID with an unknown ID_LIKE is unrecognised | |
| `TestLoaderLoadsOnlyWhatIsMissing` | `internal/component/kernelcap/loader_test.go` | injected runner: absent and declared → one call per module in declaration order; loaded or built-in → none; not in use → none | |
| `TestLoaderNeverRunsOnZeKernel` | same | release `-ze` → zero runner calls, whatever is absent | |
| `TestLoaderPreconditionsRefuseWithTheirFix` | same | each of: leaf off, no `CAP_SYS_MODULE`, `modules_disabled=1`, no modprobe path, no `/lib/modules/<release>` → zero calls and a record naming that reason and its fix line | |
| `TestLoaderRecordsFailureStderr` | same | non-zero exit and timeout produce a record carrying module, stderr and the family's package line | |
| `TestModuleNamesCompareDashAndUnderscore` | same | `vfio-pci` matches `vfio_pci` in `/proc/modules` | |
| `TestEnrolmentMustDeclareModuleSource` | `internal/component/kernelcap/kernelcap_test.go` | `MustRegister` panics on an unspecified source, on "modules" with an empty list, and on a list with an empty name | |
| `TestDeclaredModulesAreZeKernelModuleNames` | `internal/component/kernelcap/modules_names_linux_test.go` | every declared name appears in the cached runtime kernel's `modules.builtin`; skips with the build command when no cache exists | |
| `TestRefusalCarriesTheLoaderRecord` | `internal/component/kernelcap/kernelcap_test.go` | absent + record renders module lines and the fix command on its own line | |
| `TestFlowtableInUse`, `TestIngressPolicerInUse` | owners' `kernelcap_linux_test.go` | predicates over config trees: true only when a flowtable is declared on the nft backend, or a policer is configured on an interface | |
| `TestUnitFileLoadsKernelModulesAsRootBeforeStart` | `internal/plugins/systemd/unit_test.go` | the two lines exist, carry the `+` prefix and the binary path, and the capability sets are unchanged | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| modprobe bound | 10 s per call, fixed | 10 s | N/A | a call still running at 10 s is killed and recorded as timed out |
| `modules_disabled` | 0 or 1 | 0 loads, 1 refuses | N/A | any other content is "cannot tell", recorded, no exec |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `kernel-module-load-vendor-kernel` | `test/plugin/kernel-module-load-vendor-kernel.ci` | zetest daemon over a fixture proc root: `sys/kernel/osrelease` a Debian release, `sys/kernel/modprobe` naming a stub script that appends its argv to a file and the module to the fixture `modules` file; os-release and modules-root fixtures through zetest-only variables; policer configured. Expects the stub's recorded `-- act_police` and the INFO line | |
| `kernel-module-ze-kernel-loads-nothing` | `test/plugin/kernel-module-ze-kernel-loads-nothing.ci` | same fixtures with release `7.2.9-ze`; the stub's record stays empty | |
| `kernel-module-unprivileged-refuses` | `test/plugin/kernel-module-unprivileged-refuses.ci` | fixture `self/status` without `CAP_SYS_MODULE`; the start is refused with the `sudo modprobe` lines; the stub is never run | |
| `kernel-module-reload-refuses` | `test/plugin/kernel-module-reload-refuses.ci` | commit adding a policer; refused; the old configuration still answers `show` | |
| `kernel-module-load-command` | `test/plugin/kernel-module-load-command.ci` | `ze kernel-module load` prints one line per module and exits 0 | |
| `doctor-kernel-modules` | `test/ui/doctor-kernel-modules.ci` | the doctor rows for loaded, built in, loadable and not shipped | |
| `kernel-capability-refuses-start-flowtable`, `kernel-capability-refuses-start-policer` | `test/plugin/` | AC-18 and AC-19 through the force variable | |

The fixture overrides (os-release path, modules root, `self/status`) are registered and read only in a `zetest` build, as `ze.test.kernelcap.force` is (`probe_force_zetest.go`). The proc root already covers `sys/kernel/osrelease`, `sys/kernel/modprobe`, `sys/kernel/modules_disabled` and `modules`.

### Vendor-kernel proof (Q-4)
| Test | Where | What it proves | Status |
|------|-------|----------------|--------|
| `TestLoaderLoadsActPoliceOnVendorKernel` | `internal/component/kernelcap/loader_vendor_integration_linux_test.go`, tags `integration && linux`, run as root in each AC-24 VM | the real modprobe, signing and packaging on Debian 12, Ubuntu 24.04, Rocky 9; records `/proc/modules` before and after (A-10); skips on a `-ze` release | |
| AC-24 package run | the `spec-release-distribution.md` AC-21 VM matrix | the packaged unit's pre-step and the policer and flowtable installed | |

### Interop Tests (Scope: protocol)
N-A: no wire-visible behaviour changes.

## Files to Modify
- `internal/component/kernelcap/kernelcap.go` - module-source declaration on `Capability`; `MustRegister` validation; record rendering in `Refuse`, `Evaluate`, `ProbeAll`
- `internal/component/kernelcap/modules_linux.go` - read `modules.builtin` beside `/proc/modules`
- `internal/component/kernelcap/probe_force_zetest.go` - the fixture overrides
- `cmd/ze/hub/main.go` - one loader call before `kernelcap.Refuse` in `runYANGConfig`
- `cmd/ze/hub/main_system.go` - delete the modprobe half of `applyConntrackConfig`
- `internal/component/config/system/conntrack_linux.go`, `conntrack_other.go`, `conntrack.go` - delete `LoadConntrackModules`; the enumeration owner declares the helper modules on an enrolment
- `internal/component/config/system/yang/ze-system-conf.yang` - drop `nfs` and `sqlnet`; description says ze loads the helper on a distro kernel
- `internal/component/hub/yang/ze-hub-conf.yang` - leaf `load-kernel-modules` in `daemon`, `ze:help` and `description` naming `ze.daemon.load-kernel-modules`
- `internal/component/config/apply_env.go` - the env plumbing row
- `internal/component/l2tp/kernel_linux.go`, `kernel_other.go`, `subsystem.go`, `kernelcap_linux.go` - delete `probeKernelModules`; declare `l2tp_core`, `l2tp_netlink`, `l2tp_ppp`
- `internal/component/l2tp/pppoe/kernelcap_linux.go` - declare `pppoe`
- `internal/component/vpp/dpdk.go`, `dpdk_linux.go` - delete `loadModule`; a VFIO enrolment declares `vfio`, `vfio_pci`, `vfio_iommu_type1`
- `internal/plugins/flowexport/conntrack_setup_appliance_linux.go` - delete `loadConntrackModules`; an enrolment declares `nf_conntrack`, `nf_conntrack_netlink`
- `internal/plugins/fib/kernel/kernelcap_linux.go` - `mpls` declares `mpls_router` (and `mpls_iptunnel` if the FIB installs MPLS encap routes; confirm at the producer); `mpls-transit-mtu` declares Ze kernel only
- `internal/component/ike/engine/kernelcap_linux.go`, `internal/component/ike/dataplane/kernelcap_linux.go` - `xfrm_user`, `esp4`, `esp6`, `ah4`, `ah6`; `XFRM_MIGRATE` built in only; the CRYPTO transforms declare per A-10's result
- `internal/plugins/iface/netlink/kernelcap_linux.go` - `wireguard`, `xfrm_interface`
- `internal/plugins/firewall/nft/doctor_linux.go` - replaced by the enrolment's doctor row (no-layering)
- `internal/plugins/systemd/unit.go`, `doctor.go` - the pre-step lines and their read-back
- `internal/le/interoplab/kernelcheck.go`, `internal/le/setup/docker/kernel/dockerkernel.go` - import the core constant
- `internal/appliance/kernelreq.go` or the runtime kernel resolver - refuse an output release without the suffix
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - the conntrack helpers the enumeration keeps
- `docs/architecture/doctor-and-health-checks.md` - the loader as the enrolment's fifth action
- `docs/guide/ze-install.md` - what the unit's pre-step does, per-distro notes
- `docs/architecture/config/syntax.md` - conntrack helper loading (the `// Design:` target of `conntrack_linux.go`)
- `internal/core/diagnostic/codes.go` - the new codes

## Files to Create
- `internal/core/kernelrelease/kernelrelease.go` - the `-ze` suffix and the identity predicate
- `internal/component/kernelcap/loader_linux.go`, `loader_other.go` - the loader
- `internal/component/kernelcap/vendor.go` - os-release reading and family classification
- `internal/plugins/firewall/nft/kernelcap_linux.go`, `register_kernelcap_linux.go` - `firewall-nftables`, `firewall-flowtable`
- `internal/plugins/traffic/netlink/kernelcap_linux.go`, `register_kernelcap_linux.go` - `traffic-ingress-policer`, one enrolment per qdisc kind and classifier, flow sampling, mirroring
- the `ze kernel-module load` local command, registered from kernelcap's owner (CLI grammar per `ai/rules/cli.md`)
- the test files named in the TDD plan

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-hub-conf.yang` leaf; `ze-system-conf.yang` enumeration |
| YANG validation constraints | Yes | boolean leaf; enumeration narrowed |
| YANG custom validators | No | native types suffice |
| CLI commands/flags | Yes | `ze kernel-module load` |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`; the command takes no value beyond `--config` |
| Editor autocomplete | Yes | automatic for the boolean and the enumeration |
| Functional test for new RPC/API | Yes | `kernel-module-load-command.ci` |
| Pipe completeness | No | the command prints plain lines, no table |
| Env var registration | Yes | `ze.daemon.load-kernel-modules`; the zetest-only fixture variables |
| Doctor check for runtime dependencies | Yes | kernel modules row and codes `doctor-kernel-module-not-shipped`, `doctor-kernel-module-loader-missing`, `doctor-kernel-modules-disabled`, `doctor-kernel-unrecognised-distribution` |
| Prometheus counters/metrics | No | a start-time action, logged |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | the install feature's `features/<id>.md` entry |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | local command only |
| 5 | Plugin added/changed? | No | enrolments only |
| 6 | Has a user guide page? | Yes | `docs/guide/ze-install.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior changed? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the fixture variables |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered capability or inventory changed? | Yes | `docs/guide/status.md` if it lists kernel capabilities |
| 16 | Changed source files referenced by doc anchors? | Yes | the `// Design:` targets of the files this spec changes, each judged below; re-derive with `./le spec citation anchors spec plan/release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` at implementation |
| 17 | Existing examples for this area? | Yes | `system conntrack module` examples naming `nfs` or `sqlnet` |

### Design documents declared by the changed files

| Document | Declared by | Affected? |
|----------|-------------|-----------|
| `ai/digests/flow-ddos.md` | `internal/plugins/flowexport/conntrack_setup_appliance_linux.go` | Yes: the appliance conntrack init no longer runs modprobe; the conntrack modules become an enrolment |
| `docs/architecture/cli/plugin-modes.md` | `internal/plugins/systemd/unit.go` | Yes: the generated unit gains the root pre-step lines |
| `docs/architecture/core-design.md` | `internal/plugins/traffic/netlink/policer_linux.go` | Yes: the tc ingress policer section names its kernel capability enrolment |
| `docs/architecture/hub-architecture.md` | `cmd/ze/hub/main.go`, `main_system.go` | Yes: start order gains the loader before the kernel capability gate; conntrack apply loses its modprobe |
| `docs/architecture/testing/interop.md` | `internal/le/interoplab/kernelcheck.go` | No behaviour change: the suffix constant moves to the core package; the page is checked for a statement naming where it lives |
| `docs/features/ai-first.md` | a changed file's `// Design:` header (found by the spec hook) | Read at implementation; expected unaffected, the reason recorded then |
| `docs/research/l2tpv2-ze-integration.md` | `internal/component/l2tp/kernel_linux.go`, `subsystem.go` | Yes: the module probe at subsystem start is replaced by the enrolment's declaration and the loader |
| `docs/research/vpp-deployment-reference.md` | `internal/component/vpp/dpdk_linux.go` | Yes: VFIO module loading moves to the loader |

## Implementation Steps

1. **Phase: Wiring** - core suffix package and identity; module-source field with `MustRegister` validation; empty loader called from `runYANGConfig`; the zetest fixture variables. Tests: `TestKernelIdentityFromRelease`, `TestEnrolmentMustDeclareModuleSource`, the two `.ci` load tests failing on the stub record.
2. **Phase: Loader** - vendor family, preconditions, needed set, skip, exec, record, refusal text. Tests: the loader unit tests; `kernel-module-unprivileged-refuses.ci`.
3. **Phase: Declarations** - every existing enrolment declares its source; the new enrolments (flowtable, policer, qdiscs, classifiers, sampling, mirroring, nftables, flow-export conntrack, helpers, VFIO). Tests: predicates, `TestDeclaredModulesAreZeKernelModuleNames`, AC-18 and AC-19 `.ci`.
4. **Phase: Replace** - delete the four modprobe call sites and the nft module doctor check; conntrack enumeration and Ze kernel helpers. Test: AC-21 grep in a Go test over the tree (`TestNoModprobeOutsideTheLoader`); AC-22.
5. **Phase: Unit and command** - `ze kernel-module load`; `buildUnitFile`; unit doctor read-back. Tests: AC-14, AC-15.
6. **Phase: Doctor and reload** - doctor rows and codes; reload refusal text. Tests: `doctor-kernel-modules.ci`, `kernel-module-reload-refuses.ci`.
7. **Phase: Vendor proof** - per Q-4.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the identity check runs before os-release; "cannot tell" never reads as vendor; no exec on `-ze` |
| Correctness | the loader reads the post-evolution tree, the same one `Refuse` reads |
| Naming | leaf `load-kernel-modules`, env `ze.daemon.load-kernel-modules`, Go field derivable |
| Rule: no-layering | no modprobe call remains outside the loader |
| Rule: user-facing-errors | every refusal ends in a command on its own line |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One loader, no other modprobe | `TestNoModprobeOutsideTheLoader` |
| Suffix declared once | `rg '"-ze"' internal cmd` finds only the core package and tests |
| Unit pre-step | `TestUnitFileLoadsKernelModulesAsRootBeforeStart` |
| Vendor proof | AC-24 log and `TestLoaderLoadsActPoliceOnVendorKernel` output |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | module names come only from compiled enrolments, never from config text; the conntrack enumeration maps to fixed names |
| Exec | absolute path from `/proc/sys/kernel/modprobe`, `--` before the name, bounded, no shell |
| Privilege | the daemon gains no capability; the root step reads config and runs modprobe only |
| os-release | treated as untrusted text: a value only selects a fix line, never a path or a command word |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

**Kernel identity marker.** The candidates the brief named, checked against what the build guarantees:

| Candidate | Finding | Verdict |
|-----------|---------|---------|
| Release suffix `-ze` | Set by `CONFIG_LOCALVERSION="-ze"` (`runtime.config` line 4); observed in the cached build; relied on by `zeKernelReleaseSuffix` and `kernelLocalVersion`; pinned by `TestZeKernelReleaseSuffixIsTheRuntimeConfigs`. Not in `runtime.require`, which can only require `=y` | chosen; AC-23 adds the build guarantee |
| `/proc/config.gz` | `CONFIG_IKCONFIG_PROC=y` in `gokrazy/kernel/kernel.config` line 115, but Arch and Alpine set it too (configs above) | not unique |
| `/proc/version` build string | builder-dependent, nothing pins it | rejected |
| kernel cmdline or an appliance file | absent when Ze's kernel runs a non-appliance userland (the Docker lab host after `le setup docker kernel install`) | rejected: misses AC-2 |

**Per-distro facts** (primary configs fetched 2026-10-29 into the session scratch `kmod/cfg/`; `-` means the symbol is absent from that file):

| Symbol | Debian | Ubuntu | Fedora | RHEL 9 | RHEL 10 | Arch | Alpine lts | Alpine virt | Ze runtime |
|--------|--------|--------|--------|--------|---------|------|-----------|-------------|-----------|
| NET_ACT_POLICE | m | m | m | m | m | m | m | m | y |
| NET_ACT_GACT | m | m | m | m | m | m | m | m | y (test prerequisite only, `runtime.config` comment) |
| NF_FLOW_TABLE, NF_FLOW_TABLE_INET, NFT_FLOW_OFFLOAD | m | m | m | m | m | m | m | m | y |
| NF_TABLES, NF_CONNTRACK, NF_CT_NETLINK | m | m | m | m | m | m | m | m | y |
| MPLS_ROUTING, MPLS_IPTUNNEL | m | m | m | m | m | m | m | m | y |
| MPLS_IP_MTU | - | - | - | - | - | - | - | - | y (Ze patch `0002-mpls-ip-mtu.patch`) |
| L2TP, PPPOL2TP | m | m | m | m | m | m | m | m | y |
| PPPOE | m | m | m | m | m | m | m | - | y |
| WIREGUARD, XFRM_INTERFACE, INET_ESP, INET6_ESP, INET_AH, INET6_AH | m | m | m | m | m | m | m | m | y |
| XFRM_USER | m | m | y | y | y | m | m | m | y |
| XFRM_MIGRATE (bool) | y | n | y | y | y | y | - | - | y |
| NET_SCH_HTB/TBF/FQ/SFQ/PRIO/NETEM/INGRESS, NET_CLS_U32/FW/MATCHALL, NET_ACT_MIRRED | m | m | m | m | m | m | m | m | y |
| NET_ACT_SAMPLE, PSAMPLE | m | m | m | m | m | m | - | - | y |
| VFIO_PCI | m | m | m | m | m | m | m | - | builtin in the cached build |
| MODULE_SIG / MODULE_SIG_FORCE | y / n | y / n | y / n | y / n | y / n | y / n | - | - | not set |
| Module compression | xz | zstd | xz | none set | xz | zstd | - | - | n/a |

Consequences the design carries: `CONFIG_MPLS_IP_MTU` is Ze-kernel-only (AC-20); Ubuntu cannot do MOBIKE (`XFRM_MIGRATE=n`, a bool, no module helps; refuse); Alpine virt cannot do PPPoE and neither Alpine kernel can do flow sampling (refuse, "not shipped"); RHEL 9 needs `kernel-modules-extra` for L2TP, MPLS and PPPoE (A-7). Module names are the basenames in the Ze runtime build's `modules.builtin` (`act_police`, `nf_flow_table`, `nf_flow_table_inet`, `nft_flow_offload`, `mpls_router`, `mpls_iptunnel`, `l2tp_core`, `l2tp_netlink`, `l2tp_ppp`, `pppoe`, `wireguard`, `xfrm_interface`, `xfrm_user`, `esp4`, `esp6`, `ah4`, `ah6`, `sch_*`, `cls_*`, `act_mirred`, `act_sample`, `nf_tables`, `nf_conntrack`, `nf_conntrack_netlink`, `vfio`, `vfio-pci`, `vfio_iommu_type1`). That build predates the `NET_ACT_GACT=y` edit (it still carries `=m`), so it is stale for options, not for names.

**Corrections to the proposal.**
- It found one product modprobe; there are four (l2tp, vpp, conntrack helpers, flow-export).
- "Load at start, before the privilege drop" never fires under the shipped unit, which runs as `ze` from the first instruction (Q-3).
- `ze.user` drop and the unit are two routes: the in-daemon load serves a root start; the pre-step serves systemd.
- `act_gact` is not a feature need; only the policer coexistence test installs it.
- The leaf it proposed would be a second opt-in on top of the vendor-kernel gate; Q-2 re-evaluates its default.

**Defects walked into** (journal row in `plan/journal/silent-fall-through.md`, owned below): the conntrack helper load is a silent no-op on Ze's appliance and fails under the shipped unit; `nfs` and `sqlnet` name helpers Linux does not have; the helper comment claims they are built in on gokrazy.

**Out of scope here, already homed.** le's own module loading for development hosts and labs: `plan/post-release/engineering/spec-lab-kernel-features-in-setup.md` and `spec-privileged-tests-run-in-a-user-namespace` (closed; its `ppp-access` setup check in `internal/le/setup/ppp.go` runs `modprobe ppp_generic`). `le` never carries this product feature.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/silent-fall-through.md` | 2026-10-29 | `system conntrack module`, `LoadConntrackModules` |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The release suffix is the Ze-kernel marker | `/proc/config.gz`, `/proc/version`, cmdline, an appliance file | the only marker already relied on and tied to the build config; it also covers Ze's kernel under a distro userland |
| Kernel identity before os-release | os-release first | os-release describes the userland, which in a container or on the Docker lab host is not the kernel's vendor |
| os-release selects fix text only | os-release gates loading | modprobe is the same interface everywhere; gating on a name list refuses working hosts (Q-1) |
| Exec the vendor's modprobe | `finit_module` from Go with ze's own `modules.dep` resolution | the owner rule names `modprobe`; re-implementing dependency, alias, blacklist and decompression handling duplicates kmod |
| Modprobe path from `/proc/sys/kernel/modprobe` | `exec.LookPath` | the kernel's own configured helper, independent of the unit's PATH, and fixture-able through the proc root |
| Module source is an explicit three-way declaration | an optional list where empty means none | an empty list read as "no module" is the zero-as-answer shape `ai/rules/principles.md` bans |
| Load modules absent from `/proc/modules` and `modules.builtin`, then let the gate judge | load only after a probe answers absent (the proposal) | the flowtable and policer have no probe yet, and a probe that autoloads (A-10) would hide the need; the gate still judges presence |
| Root pre-step in the unit | `CAP_SYS_MODULE` on the daemon; manual only | least privilege for the long-lived process (Q-3) |
| The loader never runs at reload inside the daemon | keep `CAP_SYS_MODULE` across the drop | the daemon holds no module privilege; `systemctl reload` runs the pre-step, other reload routes get the fix text |

## Known Limitations

- A distro not in the supported list is attempted (Q-1) but carries no tested evidence; `docs/guide/ze-install.md` says so.
- Features whose kernel symbol a vendor builds `n` or omits (MOBIKE on Ubuntu, flow sampling on Alpine, PPPoE on Alpine virt, MPLS transit MTU everywhere but Ze's kernel) are refused there; no module can supply them.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no wire change)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` only, in the same `./le commit create` script
