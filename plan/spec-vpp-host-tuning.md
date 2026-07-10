# Spec: vpp-host-tuning

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-vpp-isolated-cpus |
| Phase | 9/10 (code complete + tested; QEMU/kernel-build evidence infra-gated) |
| Updated | 2026-07-10 |

**Notes:** Designed to ready 2026-07-10 in an autonomous phase-1 design session
(user instruction: run the full /ze-spec DESIGN methodology, record gate
decisions as annotations instead of asking). The NUMA/SMT item was split out to
`plan/spec-vpp-numa-smt.md` (skeleton, created by this session). The Depends
row is coordination-soft: nothing here needs spec-vpp-isolated-cpus implemented
first (the gokrazy KernelExtraArgs path exists today); the two specs share one
kernel-argument assembly seam, and whichever is implemented first creates it.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/spec-vpp-isolated-cpus.md` - the prerequisite (isolcpus sourcing + CPU validation, `ready`)
4. `internal/component/vpp/startupconf.go` - startup.conf generation
5. `internal/component/vpp/config.go` - VPP settings model
6. `internal/appliance/cmd_build.go` - gok build invocation (`--parent_dir gokrazy -i ze`, cmd_build.go:262, :286-292)
7. `internal/appliance/config.go` - `ImageConfig` + `applianceConfig.Validate` (config.go:53-57, :130-172)
8. `gokrazy/ze/config.json` - `KernelExtraArgs` (line 37) consumed by the gokrazy packer
9. `tmp/session/session-state-spec-vpp-host-tuning-phase1.md` - design-session file digests

## Task

**Skeleton created from the osvbng comparison refresh (2026-07-10). Full design not started.**

Ze generates VPP's startup.conf (cores, hugepage page-size, buffers-per-numa,
rx-queues) but leaves the HOST side of a performant VPP deployment to the
operator, and omits the idle-behaviour knob entirely. `plan/spec-vpp-isolated-cpus.md`
(ready) already owns isolated-core sourcing, CPU validation, and requesting
isolcpus on the appliance boot cmdline. This spec covers what remains on top of
it:

1. **`poll-sleep-usec` exposure.** VPP's idle-worker sleep is not configurable
   from Ze (verified: no `poll-sleep` match under `internal/component/vpp/`).
   Default VPP behaviour busy-polls workers at 100% CPU; a sleep value trades
   latency for idle CPU. Expose it in the YANG cpu/dataplane settings with a
   documented recommendation (0 or unset for production latency, non-zero for
   dev/shared hosts). Reference: osvbng 5fc360e documents exactly this trade-off.
2. **Hugepage reservation at boot.** startup.conf consumes a hugepage page-size,
   but nothing reserves hugepages on the host. On the gokrazy appliance Ze owns
   the kernel cmdline, so it can emit `default_hugepagesz`/`hugepagesz`/
   `hugepages` (and per-NUMA-node reservation where topology calls for it)
   alongside the isolcpus request that spec-vpp-isolated-cpus already plans.
3. **NUMA/SMT awareness (design to scope).** Candidates: derive or validate
   worker placement against the NIC's NUMA node; avoid splitting a physical core
   between VPP and the host (SMT sibling awareness); disable automatic NUMA
   balancing where it fights pinning. Reference: osvbng 88064b7 (KVM deploy
   rewrite) does per-node hugepages, SMT-aware pinning, and NUMA-node-derived
   core selection on the HOST; Ze's equivalent surface is the appliance image
   and doctor checks, not a deploy script.

Ze-shape note: osvbng tunes a KVM host that runs the VM. Ze's unit of deployment
is the gokrazy appliance image itself (see memory: Ze owns full process
lifecycle, no systemd), so items land in the image build/boot config, the VPP
component's startup.conf generation, validation, and `ze doctor`, not in a
shell script.

### Design outcome (2026-07-10)

- Item 1 stands, with one correction: VPP accepts `poll-sleep-usec` in the
  **unix** section, not the cpu section (VPP 25.02 configuration reference:
  "Add a fixed-sleep between main loop poll. Default is 0, which is not to
  sleep"; the cpu section accepts only main-core, corelist-workers, skip-cores,
  workers, scheduler-policy, scheduler-priority). The YANG leaf stays under the
  operator-facing `cpu` container (Ze's YANG groups by operator concern, not by
  startup.conf section: the `memory` container already feeds the buffers,
  heapsize, and statseg sections, startupconf.go:55-59 and :107-117), and the
  emitted directive goes into the unix section.
  → Decision: YANG leaf `vpp/cpu/poll-sleep-microseconds` (uint32, range 0..100000,
  no default), emitted as `poll-sleep-usec <n>` in the unix section only when
  the leaf is present. Name follows `ai/rules/config-naming.md`: no
  abbreviations ("usec" is not on the exception list), unit suffix required.
- Item 2 stands: per-appliance `image.hugepages` block in the appliance
  config.json, assembled into kernel arguments at `ze appliance build` through
  gokrazy's `KernelExtraArgs`, plus a `CONFIG_HUGETLBFS` requirement in the
  runtime kernel profile and a `doctor-vpp-hugepages` runtime check.
- Item 3 is **split out** to `plan/spec-vpp-numa-smt.md` (skeleton). Reasons,
  each verified against source: (a) the host inventory has none of the needed
  facts today -- `CoreInfo` carries CPU/CoreID/PhysicalPackage but no NUMA node
  (internal/component/host/inventory.go:203-212), `readNICSysfs` reads no
  `numa_node` (internal/component/host/nic_linux.go:75-95), and there is no
  `/sys/devices/system/node` reader anywhere under `internal/` (grep `numa`,
  2026-07-10); (b) NUMA-aware worker placement validates the worker-core
  *selection*, which spec-vpp-isolated-cpus owns and has not implemented, so
  concrete ACs here would bind to a helper that does not exist; (c) the
  `kernel.numa_balancing` sysctl is a global key, and the sysctl profile
  surface is interface-scoped (`<iface>` placeholders,
  internal/core/sysctl/profiles.go:14-15, :91-100), so it needs its own design.
  Global (non-per-node) hugepage reservation is sufficient for the documented
  reference hardware, which is all single-socket
  (docs/research/vpp-deployment-reference.md:242-247).
  → Decision: split NUMA/SMT into `plan/spec-vpp-numa-smt.md`; this spec's
  reservation is host-global (the kernel splits `hugepages=N` evenly across
  nodes on multi-node systems; per-node counts move to the split spec).

## Required Reading

### Architecture Docs
- [ ] `docs/research/vpp-deployment-reference.md` - startup.conf syntax + production values.
  → Constraint: keep generated startup.conf valid for the pinned VPP version.
  → Constraint: there is no hard VPP version pin in the repo. The doc targets
  VPP 25.02+ (vpp-deployment-reference.md:158, :231), evidence runs
  `ligato/vpp-base:latest` (scripts/evidence/effective-vpp.py:18), GoVPP is
  v0.13.0 (go.mod:24). `poll-sleep-usec` verified against the VPP 25.02
  configuration reference (unix section).
  → Constraint: hugepage sizing formula is `main_heap + buffers * data_size +
  statseg + overhead` (vpp-deployment-reference.md:89); the doctor sufficiency
  check reuses it. System-prerequisites table documents the manual sysfs echo
  this spec replaces on the appliance (vpp-deployment-reference.md:75).
- [ ] `plan/spec-vpp-isolated-cpus.md` - prerequisite scope boundary.
  → Constraint: isolcpus request + CPU validation belong THERE; this spec must not duplicate them.
  → Decision: the shared surface is one kernel-argument assembly function in
  `internal/appliance` (new `kernelargs.go`) that turns appliance config into
  extra kernel arguments. This spec implements it emitting hugepage arguments;
  spec-vpp-isolated-cpus Phase "Boot isolation" adds its isolcpus argument to
  the same function. Whichever is implemented first creates the seam; the
  second rebases (merge awareness, not a design conflict).
- [ ] `ai/rules/config-surface.md` - YANG vs env var for the new knobs.
  → Decision: poll-sleep is operator tuning (capacity/latency trade-off) ->
  YANG config, no env var (not under `environment/`). Hugepage reservation is
  image-build configuration -> appliance config.json (the existing non-YANG
  surface for image settings, `ImageConfig`, internal/appliance/config.go:53-57),
  because the value is consumed on the build host at image-assembly time,
  before any appliance YANG config exists on the target.
- [ ] `ai/rules/config-naming.md` - leaf naming.
  → Constraint: kebab-case, no abbreviations, unit suffix when ambiguous ->
  `poll-sleep-microseconds` (not `poll-sleep-usec`); Go field
  `PollSleepMicroseconds`; appliance JSON keys kebab-case (`page-size`,
  `memory-bytes`) matching existing `size-bytes`/`kernel-profile` keys.
- [ ] `ai/patterns/config-option.md` - structural template for the YANG leaf.
  → Constraint: leaf needs type + range + description; native `range
  "0..100000"` is sufficient, no custom validator; config parse test lives in
  `test/parse/` (precedent: fib-vpp-config-valid.ci).
- [ ] `ai/rules/doctor-checks.md` - runtime dependency checks.
  → Constraint: hugepages are a procfs/sysfs runtime dependency; the owning
  component (vpp) registers the check via `diagnostic.RegisterDoctorCheck()`
  from a linux-tagged file (rule table row "Kernel module, procfs, sysctl,
  netlink, VPP, or platform-specific backend"). Code `doctor-vpp-hugepages`
  registered in `internal/core/diagnostic/codes.go`; one code may carry
  multiple failure messages (precedent: doctor-vpp-dpdk,
  internal/component/doctor/checks_linux.go:639 and :651). Unit test +
  functional test through `ze doctor --json` both required; package with new
  `//go:build linux` files must be added to `ze-qemu-integration-test`
  (grep 2026-07-10: `internal/component/vpp` is not in the target's run list).
- [ ] `ai/rules/qemu-testing.md` - boot-cmdline behaviour needs QEMU evidence.
  → Decision: evidence script `scripts/evidence/effective-vpp-hugepages-qemu.py`
  modeled on `effective-install-qemu.py` (build image via `ze appliance`, boot
  the image directly in QEMU, SSH in, assert), make target
  `ze-vpp-hugepages-qemu-test` in `mk/test-integration.mk`, `.ci` wrapper
  `test/appliance/vpp-hugepages-qemu.ci` that self-skips without artifacts
  (pattern: test/install/qemu-full.ci over effective-install-qemu.py).
  → Constraint: the doctor functional test boots a daemon and reads real
  sysfs -> `option=needs-linux`.
- [ ] `ai/rules/discovery-updates.md` - discovery surfaces for the new target/check.
  → Constraint: new make target needs an `ai/INDEX.md` row + mk help-block
  line; new doctor code must pass `TestDoctorCoverageCodesRegistered` and be
  explainable via `ze explain`.

**Key insights:**
- The dependency ordering matters: isolated-CPU sourcing (prerequisite spec)
  decides WHICH cores VPP gets; this spec decides how the host is prepared
  around that choice (hugepages, idle behaviour, NUMA fit).
- The whole boot-reservation chain already exists outside Ze: gokrazy's packer
  appends `KernelExtraArgs` to /cmdline.txt
  (gokrazy/modcache/github.com/gokrazy/tools@v0.0.0-20260406155313-5861e2403dc8/internal/packer/write.go:71-135,
  field defined in gokrazy/internal config.go:217). Ze only has to compute
  per-appliance arguments and hand them to gok -- via a derived instance
  config, because the checked-in `gokrazy/ze/config.json` is shared by all
  appliances and must not be edited in place.
- The runtime kernel is built from `make defconfig` + repo fragments with a
  require manifest that fails the build if a symbol does not resolve to =y
  (tools/kernel-builder/build.py:248-252 merge, :143-160 enforcement). Whether
  defconfig enables HUGETLBFS is irrelevant once `CONFIG_HUGETLBFS` is added to
  `gokrazy/kernel/runtime.config` + `runtime.require` -- the build then
  guarantees it.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-07-10 at survey depth; re-read fully at design time)
- [ ] `internal/component/vpp/startupconf.go` - emits `main-core`, `corelist-workers`, `page-size` from hugepage-size, `buffers-per-numa`, per-interface `num-rx-queues`. No `poll-sleep-usec` (grep verified 2026-07-10: zero matches in `internal/component/vpp/`).
  → Constraint: the unix section is fixed today: nodaemon, cli-listen, log,
  full-coredump (startupconf.go:25-30). The new directive appends inside this
  section; all other output stays byte-identical when the leaf is unset.
- [ ] `internal/component/vpp/config.go` - `CPUSettings` (main-core/workers), `MemorySettings` (hugepage-size/buffers); no idle/poll knob, no host hugepage reservation.
  → Constraint: `CPUSettings{MainCore, Workers *uint8}` (config.go:51-54);
  `parseCPU` whitelists keys main-core/workers via `unknownKeys`
  (config.go:323-346) -- the new key must join that list or parsing rejects it.
  Parse-time numeric bounds precedent: stats poll-interval 1..3600
  (config.go:445-447). `ParseSettings` defaults block is config.go:204-222.
- [ ] `internal/appliance/` image build - confirm where kernel cmdline is assembled (gokrazy) and whether any hugepage parameters are set today (expected: none; validate A-1).
  → Constraint: verified. `runGokBuild` invokes gok with `--parent_dir gokrazy
  -i ze` (cmd_build.go:262, :286-292); the only cmdline input Ze controls is
  `KernelExtraArgs: ["loglevel=8"]` in the checked-in `gokrazy/ze/config.json:37`.
  No hugepage/isolcpus tokens anywhere (grep `nr_hugepages|hugepagesz|
  default_hugepagesz|hugepages=` over internal/, scripts/, cmd/, gokrazy/:
  zero matches, 2026-07-10). The installer's grub cmdline (cmd_iso.go:754-761)
  and `/proc/cmdline` parser (internal/install/disk/cmdline.go) are the
  install-time chain, not the appliance runtime cmdline; not covered here.
  → Constraint: `buildOne` loads the appliance config WITHOUT calling
  `Validate()` (cmd_build.go:114-118; Validate is only called at cmd_init.go:107,
  :420 and cmd_iso.go:283). Hugepage validation at build requires wiring a
  Validate call into the build path.
  → Constraint: `ze appliance run` boots QEMU with hardcoded `-smp 2 -m 512`
  (cmd_run.go:154, :164).
- [ ] `internal/core/sysctl/profiles.go` - whether a performance profile exists that should carry `kernel.numa_balancing` (validate at design).
  → Constraint: verified: profiles are interface-scoped (`<iface>` placeholder
  substitution, profiles.go:14-15, :91-100); no global-key profile exists.
  `kernel.numa_balancing` therefore moves to the split NUMA spec.

### Design-time verification (2026-07-10)

- `internal/component/vpp/register.go` - config verify path: `verifyVPPConfig`
  (register.go:65-79) is both the `InProcessConfigVerifier` (register.go:40)
  and the `OnConfigVerify` hook (register.go:116); it parses and calls
  `VPPSettings.Validate()`. New parse/validate rules are automatically
  enforced at `ze config validate` and commit.
- `internal/component/vpp/vpp.go` - `Run` writes startup.conf only when NOT
  external (vpp.go:185-199: external mode logs "skipping startup.conf + DPDK
  bind"). Consequence: a functional `.ci` test cannot observe startup.conf
  through the daemon; the parse->emit chain is proven by a unit test that
  feeds config JSON through `ParseSettings` into `GenerateStartupConf`.
  Note: the YANG `external` description (ze-vpp-conf.yang:29-37) still claims
  "Ze still generates startup.conf" -- stale vs vpp.go:185-199; fixed as a
  drive-by in the same YANG edit. `docs/guide/vpp.md:171` already documents
  the correct (skip) behaviour.
- `internal/appliance/config.go` - `ImageConfig{Arch, SizeBytes,
  KernelProfile}` (config.go:53-57) is the per-appliance image-settings home;
  `applianceConfig.Validate` (config.go:130-172) is where the new bounds go.
  `LoadConfig` uses `DisallowUnknownFields` (config.go:195), so new JSON keys
  are additive and typo-safe.
- gokrazy packer (vendored modcache, tools@v0.0.0-20260406155313): `writeCmdline`
  reads the kernel package's cmdline.txt, prepends console args, appends
  `KernelExtraArgs` joined by spaces, writes /cmdline.txt on the boot
  partition, plus a systemd-boot entry when UseGPTPartuuid (write.go:71-135).
  gok resolves the instance config at `<parent_dir>/<instance>/config.json`
  (gokrazy/internal instanceflag.go:83-85), which enables a derived temp
  parent dir for per-appliance args. `GOMODCACHE` is set independently of
  parent_dir (cmd_build.go:239-245), so a temp parent dir does not disturb
  module resolution.
- Kernel profile: runtime profile fragments are
  `gokrazy/kernel/{kernel,runtime}.{config,require}`; base is `make defconfig`
  (tools/kernel-builder/build.py:248), fragments merged via merge_config.sh
  (build.py:249-252), require manifests enforced to =y (build.py:143-160).
  Neither fragment mentions HUGETLB/NUMA today (grep 2026-07-10). Precedent
  for adding a feature symbol: CONFIG_PPPOE in runtime.config per
  `ai/rules/qemu-testing.md` step 3.
- Host facts available to doctor: meminfo reader with overridable proc root
  (internal/component/host/memory_linux.go:44-68; no HugePages_* fields
  parsed today, meminfoFields memory_linux.go:21-29); cpuinfo flags via
  `hasFlag` (internal/component/host/cpu_linux.go:227) -- usable for the
  pdpe1gb warning.
- Doctor registration precedent: `applianceDoctorChecks` returns
  `diagnostic.DoctorCheck` entries registered with name/phase/order/component/
  dependencies/platforms/codes/check (internal/appliance/doctor_checks.go:15-60);
  `diagnostic.RegisterDoctorCheck` (internal/core/diagnostic/doctor_registry.go:65-78).
  Existing codes doctor-vpp-unreachable/-version/-wireguard/-lcp-netns/-dpdk
  (internal/core/diagnostic/codes.go:145, :277, :283, :289, :563).

**Behavior to preserve:**
- Generated startup.conf remains valid; absent new config, output is byte-identical to today.
- spec-vpp-isolated-cpus semantics untouched.
- Absent `image.hugepages`, the built image's /cmdline.txt is identical to
  today (base `KernelExtraArgs` `loglevel=8` preserved, gokrazy/ze/config.json:37).
- `ze appliance run` still defaults to `-m 512` when `image.memory-bytes` is unset.

**Behavior to change:**
- New knobs (poll-sleep-usec; hugepage reservation; ~~NUMA checks per design outcome~~ NUMA moved to spec-vpp-numa-smt), all additive.
- `ze appliance build` starts validating the appliance config (it currently
  loads without Validate, cmd_build.go:114-118); existing valid configs are
  unaffected (all current fields already pass the init-time validation).

## Data Flow (MANDATORY)

### Entry Point
- Config: new leaves in the VPP YANG (idle/poll behaviour, hugepage reservation) and/or appliance image settings.
  → refined: (a) `vpp/cpu/poll-sleep-microseconds` in `ze-vpp-conf.yang`;
  (b) `image.hugepages` (`page-size`, `count`) and optional `image.memory-bytes`
  in the per-appliance config.json parsed by `LoadConfig`
  (internal/appliance/config.go:188-200).
- Host facts: NUMA topology, SMT siblings, NIC `numa_node` (read paths decided at design).
  → refined: NUMA/SMT facts moved to spec-vpp-numa-smt. This spec reads, at
  doctor time only: `/sys/kernel/mm/hugepages/hugepages-<size>kB/nr_hugepages`
  (+ `free_hugepages`), `/proc/cmdline`, `/proc/meminfo`, and the cpuinfo flag
  set already parsed by the host detector.

### Transformation Path
1. New leaves parsed into VPP settings / appliance image config.
   → `parseCPU` gains `poll-sleep-microseconds` -> `CPUSettings.PollSleepMicroseconds`
   (pointer, absent = unset); `LoadConfig` decodes `image.hugepages` /
   `image.memory-bytes` into `ImageConfig`; `applianceConfig.Validate` bounds them.
2. startup.conf generation emits `poll-sleep-usec` when configured.
   → in the unix section (startupconf.go:25-30), value printed as decimal.
3. Image build emits hugepage (and existing isolcpus) kernel cmdline parameters.
   → new kernel-argument assembly in `internal/appliance/kernelargs.go`:
   computes `default_hugepagesz=<size> hugepagesz=<size> hugepages=<count>`
   from `ImageConfig`; when non-empty, `buildOne` materialises a derived
   gokrazy instance config (raw-JSON read of gokrazy/ze/config.json preserving
   unknown fields, computed args appended to `KernelExtraArgs`) under a build
   temp parent dir and passes that dir to gok; when empty, the checked-in
   parent dir is used unchanged. gokrazy packer bakes /cmdline.txt
   (write.go:71-135); kernel reserves pages at boot.
4. Doctor/verify checks compare requested resources against host topology.
   → `doctor-vpp-hugepages` (linux-only, vpp component): fires only when a
   vpp config block with enabled=true is present; reads sysfs/procfs behind
   an overridable root; conditions listed under Acceptance Criteria AC-9..AC-12.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ VPP settings | YANG leaves → settings structs | [ ] |
| Settings ↔ startup.conf | poll-sleep-usec emission | [ ] |
| Appliance config ↔ kernel cmdline | hugepage parameters at image build | [ ] |
| Doctor ↔ host topology | ~~NUMA/SMT/hugepage sanity checks~~ hugepage sanity checks (NUMA/SMT → spec-vpp-numa-smt) | [ ] |
| Ze build ↔ gok | derived instance config (temp parent dir), KernelExtraArgs | [ ] |
| Kernel profile ↔ boot | CONFIG_HUGETLBFS required in runtime profile | [ ] |

### Integration Points
- `internal/component/vpp/startupconf.go` - new emission.
- `internal/component/vpp/config.go` + YANG - new leaves.
- `internal/appliance/` image build - cmdline parameters (coordinate with spec-vpp-isolated-cpus Phase "Boot isolation").
- `ze doctor` - host-topology checks (`ai/rules/doctor-checks.md`).
- `internal/appliance/cmd_build.go` - Validate call + derived parent dir hand-off to gok (cmd_build.go:257-298).
- `internal/appliance/cmd_run.go` - QEMU `-m` from `image.memory-bytes` (cmd_run.go:154, :164).
- `gokrazy/kernel/runtime.config` + `runtime.require` - CONFIG_HUGETLBFS.

### Architectural Verification
- [ ] No bypassed layers (config through parse/validate; no side-channel host writes)
- [ ] No unintended coupling (topology reads behind testable helpers)
- [ ] No duplicated functionality (isolcpus stays in the prerequisite spec)
- [ ] Registration over hardcoding - doctor checks register in the owning package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No hugepage reservation exists anywhere today | grep `nr_hugepages`/`hugepagesz` over `internal/` and `scripts/` (2026-07-10 survey) found nothing | scope shrinks | re-grep + read appliance cmdline assembly | confirmed (2026-07-10: grep `nr_hugepages|hugepagesz|default_hugepagesz|hugepages=` over internal/, scripts/, cmd/, gokrazy/ = zero matches; cmdline inputs are only kernel-package cmdline.txt + console + `KernelExtraArgs ["loglevel=8"]`, gokrazy/ze/config.json:37, packer write.go:71-135; docs prescribe a manual sysfs echo, docs/guide/vpp.md:225-229, vpp-deployment-reference.md:75) |
| A-2 | gokrazy image build lets Ze append arbitrary kernel cmdline parameters | gokrazy owns cmdline; spec-vpp-isolated-cpus A-2 makes the same bet | hugepages via sysctl `vm.nr_hugepages` fallback (no 1G pages then) | read `internal/appliance/` cmd_build cmdline path | confirmed (2026-07-10: `KernelExtraArgs []string` field, gokrazy/internal config.go:217; consumed by packer `writeCmdline`, tools write.go:91, appended verbatim to /cmdline.txt; instance config resolved at `<parent_dir>/<instance>/config.json`, instanceflag.go:83-85, enabling a per-appliance derived copy) |
| A-3 | The pinned VPP version accepts `poll-sleep-usec` in the cpu section | osvbng doc + VPP upstream docs | emit under the correct section per version | check VPP version docs during design | broken (2026-07-10: VPP 25.02 configuration reference places `poll-sleep-usec` in the **unix** section, "Add a fixed-sleep between main loop poll. Default is 0"; the cpu section takes only main-core/corelist-workers/skip-cores/workers/scheduler-*. Also there is no VPP pin in the repo: evidence uses `ligato/vpp-base:latest`, effective-vpp.py:18. Design corrected: emit in unix section; see Mistake Log) |
| A-4 | The baked /cmdline.txt governs the kernel cmdline when the built image is booted directly in QEMU (BIOS/MBR loader path) | packer writes /cmdline.txt + loader entries (write.go:71-135); effective-install-qemu.py already re-boots a written disk and reaches SSH | QEMU evidence asserts /proc/cmdline; if the loader ignored it the test fails loudly | `effective-vpp-hugepages-qemu.py` asserting /proc/cmdline content | unvalidated (validated by the new QEMU evidence at implementation) |
| A-5 | With `CONFIG_HUGETLBFS` in runtime.config + runtime.require, the runtime kernel supports hugetlb reservation regardless of defconfig defaults | require manifest enforcement fails the kernel build otherwise (build.py:143-160) | kernel build fails visibly at `make ze-kernel`; add missing dependent symbols | `make ze-kernel` after the fragment edit | unvalidated (validated mechanically by the kernel build at implementation) |
| A-6 | Over-requesting `hugepages=N` at boot clamps to what fits (kernel reserves fewer pages) rather than failing the boot | Linux hugetlb boot-alloc behaviour; the doctor shortfall check (AC-11) is designed around it | if boot can fail instead, the QEMU negative case moves from doctor-warning to boot-failure assertion | QEMU evidence with an over-sized request on a small VM | unvalidated (validated by QEMU evidence at implementation) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Non-zero poll-sleep default silently costs latency | throughput/latency regression in ze-perf | default unset (VPP default); document the trade-off; no silent default |
| R-2 | Hugepage reservation starves general memory on small appliances | OOM at boot in QEMU test | validate reservation against total memory at verify |
| R-3 | NUMA scope creep (this spec grows a deploy-tool) | design keeps expanding | keep host prep = image build + doctor checks; split anything else (done: spec-vpp-numa-smt) |
| R-4 | Derived instance config drops fields it does not know (upstream config.json gains fields; loglevel=8 lost) | gok build behaves differently only for hugepage-configured appliances; loglevel missing from /proc/cmdline | patch raw JSON generically (decode to a generic map, append to `KernelExtraArgs`, re-encode); unit test asserts unknown fields and base args survive round-trip |
| R-5 | `internal/appliance/cmd_build.go` is concurrently modified by the fixit-appliance-evidence-config session (git status 2026-07-10) | merge conflicts on cmd_build.go / mk/gokrazy.mk at implementation | rebase awareness; the new assembly lives in a new file (kernelargs.go) with a minimal call-site diff in buildOne |
| R-6 | Kernel without HUGETLBFS silently ignores `hugepages=` and VPP fails at runtime | HugePages_Total absent from /proc/meminfo in QEMU evidence | runtime.require entry makes the kernel build fail instead (A-5); doctor error when the sysfs size directory is missing (AC-9) |
| R-7 | 1G pages configured on amd64 CPU without `pdpe1gb` are silently not reserved | doctor warning (AC-12); QEMU evidence uses 2M pages to stay deterministic | doctor check on cpuinfo flags (hasFlag, cpu_linux.go:227) |
| R-8 | QEMU default 512 MiB RAM too small for a meaningful reservation test | evidence flaky or trivially small | evidence script boots QEMU with explicit `-m 1024`; `ze appliance run` honours `image.memory-bytes` (AC-13) so operators reproduce the same shape |
| R-9 | Emitting `poll-sleep-usec 0` vs omitting could differ in VPP behaviour | none expected (0 is the documented default) | emit whenever the leaf is present, including explicit 0; AC-2 pins the unset case to byte-identical output |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| poll-sleep configured in VPP settings | → | startup.conf contains `poll-sleep-usec` | ~~`test/plugin/vpp-poll-sleep.ci`~~ superseded: external mode skips startup.conf generation (vpp.go:185-199) so no `.ci` can observe the file; chain proven by `TestStartupConfPollSleep` (config JSON → ParseSettings → GenerateStartupConf) + `test/parse/vpp-poll-sleep.ci` (validate accepts/rejects) |
| hugepage reservation configured | → | image kernel cmdline carries hugepage parameters | `TestKernelArgsHugepages` (assembly) + `make ze-vpp-hugepages-qemu-test` (`scripts/evidence/effective-vpp-hugepages-qemu.py`, wrapped by `test/appliance/vpp-hugepages-qemu.ci`) |
| invalid hugepage config in appliance config.json | → | `applianceConfig.Validate` rejects at init/build | `TestHugepageConfigValidate` + `test/appliance/appliance-hugepages-validate.ci` |
| vpp.enabled on a host without reserved hugepages | → | `doctor-vpp-hugepages` diagnostic | `TestDoctorVPPHugepages` + `test/plugin/vpp-doctor-hugepages.ci` (`option=needs-linux`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | poll-sleep leaf set | `poll-sleep-usec` emitted in startup.conf with the value → refined: `cpu poll-sleep-microseconds 100` yields `poll-sleep-usec 100` inside the unix section (explicit 0 is also emitted) |
| AC-2 | poll-sleep leaf unset | startup.conf byte-identical to today (VPP default behaviour) |
| AC-3 | hugepage reservation configured | boot cmdline reserves the pages; VPP starts with them available (QEMU evidence) → refined: `image.hugepages {page-size 2M, count 512}` yields `/proc/cmdline` containing `default_hugepagesz=2M hugepagesz=2M hugepages=512` and `/proc/meminfo` `HugePages_Total: 512` on the booted image |
| AC-4 | reservation exceeds appliance memory | config verify rejects → refined: when `image.memory-bytes` is set, `count × page-size > 50%` of it is rejected by `applianceConfig.Validate` at `ze appliance init`/`build`; when `memory-bytes` is unset, the static cap (total ≤ 512 GiB) applies and runtime starvation is covered by doctor (AC-11) |
| AC-5 | ~~NUMA checks (per design scope): mismatched NIC/worker NUMA placement surfaces as doctor warning~~ | ~~doctor warning~~ moved to `plan/spec-vpp-numa-smt.md` (split decision, see Design outcome) |
| AC-6 | poll-sleep-microseconds 100001 | `ze config validate` rejects (YANG range 0..100000; parse-time bound gives the same rejection through `verifyVPPConfig`, register.go:65-79) |
| AC-7 | `image.hugepages` count 0, or page-size not 2M/1G, or total > 512 GiB | appliance config validation rejects with an actionable message |
| AC-8 | no `image.hugepages` configured | built image /cmdline.txt identical to today; base `KernelExtraArgs` (`loglevel=8`) preserved; checked-in `gokrazy/ze/config.json` never modified in place |
| AC-9 | vpp.enabled true, memory hugepage-size 2M, host sysfs shows 0 (or missing) 2M hugepages | `ze doctor` reports `doctor-vpp-hugepages` error |
| AC-10 | reserved hugepage bytes < estimated VPP need (main-heap + buffers × 2048 + statseg, vpp-deployment-reference.md:89) | `ze doctor` reports `doctor-vpp-hugepages` warning naming both numbers |
| AC-11 | `/proc/cmdline` requests `hugepages=N` but `HugePages_Total < N` (kernel clamped) | `ze doctor` reports `doctor-vpp-hugepages` warning (shortfall) |
| AC-12 | amd64 host without `pdpe1gb` cpuinfo flag and vpp hugepage-size 1G | `ze doctor` reports `doctor-vpp-hugepages` warning (1G pages unavailable on this CPU) |
| AC-13 | `image.memory-bytes` set | `ze appliance run` boots QEMU with `-m` derived from it; unset keeps today's `-m 512` (cmd_run.go:154, :164) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | sets poll-sleep for a shared dev host | config → startupconf → idle workers sleep | ~~`test/plugin/vpp-poll-sleep.ci`~~ `TestStartupConfPollSleep` (parse→emit) + `test/parse/vpp-poll-sleep.ci` (config surface) |
| 2 | builds an appliance image with hugepages + isolated cores | image build → cmdline → boot → VPP consumes pages | `make ze-vpp-hugepages-qemu-test` (`effective-vpp-hugepages-qemu.py`; isolated cores land later via spec-vpp-isolated-cpus in the same kernel-args seam) |
| 3 | runs `ze doctor` on an appliance whose VPP config outgrew the boot reservation | vpp config + sysfs/procfs → doctor-vpp-hugepages warning | `TestDoctorVPPHugepages` + `test/plugin/vpp-doctor-hugepages.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStartupConfPollSleep` | `internal/component/vpp/startupconf_test.go` | emission + absence when unset → refined: config JSON through `ParseSettings` into `GenerateStartupConf`; directive in unix section; explicit 0 emitted; unset = byte-identical golden | |
| `TestHugepageReservationValidate` | `internal/component/vpp/config_test.go` | reservation vs memory bounds → superseded: reservation validation lives in the appliance config, renamed `TestHugepageConfigValidate` in `internal/appliance/config_test.go` (count/page-size/total/memory-bytes bounds) | |
| `TestParseCPUPollSleepBounds` | `internal/component/vpp/config_test.go` | parse accepts 0 and 100000, rejects 100001 and non-numeric; unknown-key whitelist extended | |
| `TestKernelArgsHugepages` | `internal/appliance/kernelargs_test.go` | assembly emits the three tokens in deterministic order; empty when unconfigured | |
| `TestDerivedInstanceConfigPreservesFields` | `internal/appliance/kernelargs_test.go` | raw-JSON patch keeps unknown fields and base `KernelExtraArgs` (R-4) | |
| `TestBuildValidatesConfig` | `internal/appliance/cmd_build_test.go` | buildOne rejects an invalid hugepage config before assembling (Validate wired into build) | |
| `TestRunQEMUMemoryBytes` | `internal/appliance/cmd_run_test.go` | `-m` derived from `image.memory-bytes`; 512 default when unset | |
| `TestDoctorVPPHugepages` | `internal/component/vpp/doctor_linux_test.go` | fixture sysfs/procfs roots: error on zero/missing pages (AC-9), warnings on insufficiency/clamp/pdpe1gb (AC-10..12), silent when vpp disabled or absent | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| poll-sleep-usec → poll-sleep-microseconds | ~~design (candidate 0-100000)~~ 0..100000 | 100000 | N/A (0 valid) | 100001 |
| hugepage count | 1-(memory/page-size) → 2M pages, no memory-bytes: 1..262144 (total ≤ 512 GiB) | 262144 | 0 | 262145 |
| hugepage count (1G pages, no memory-bytes) | 1..512 | 512 | 0 | 513 |
| hugepage count (2M pages, memory-bytes = 8 GiB) | 1..2048 (total ≤ 50% of memory-bytes) | 2048 | 0 | 2049 |
| image.memory-bytes | 268435456..1099511627776 (256 MiB..1 TiB) | 1099511627776 | 268435455 | 1099511627777 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-poll-sleep` | ~~`test/plugin/vpp-poll-sleep.ci`~~ `test/parse/vpp-poll-sleep.ci` | operator tunes idle behaviour: validate accepts 0/100/100000, rejects 100001 (runs natively; startup.conf content proven by unit test, see Wiring Test) | |
| `vpp-doctor-hugepages` | `test/plugin/vpp-doctor-hugepages.ci` (`option=needs-linux`) | `ze doctor --json` surfaces `doctor-vpp-hugepages` on a host with no reservation when vpp is enabled; silent when disabled | |
| `appliance-hugepages-validate` | `test/appliance/appliance-hugepages-validate.ci` | `ze appliance init`/`build` rejects count 0, bad page-size, over-50% reservation | |
| `vpp-hugepages-qemu` | `test/appliance/vpp-hugepages-qemu.ci` wrapping `scripts/evidence/effective-vpp-hugepages-qemu.py` (make `ze-vpp-hugepages-qemu-test`) | image with `image.hugepages` boots; /proc/cmdline + /proc/meminfo prove the reservation (AC-3); self-skips without QEMU/artifacts | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - dataplane/appliance config, no wire protocol | - | - | QEMU appliance test covers boot behaviour | - |

### Future (if deferring any tests)
- None planned (skeleton; refine at design). → refined at design: none pending.

## Files to Modify
- `internal/component/vpp/startupconf.go` - poll-sleep-usec emission → in the unix section (startupconf.go:25-30)
- `internal/component/vpp/config.go` + `internal/component/vpp/yang/ze-vpp-conf.yang` - new leaves → `CPUSettings.PollSleepMicroseconds` (*uint32), `parseCPU` key + bounds, YANG leaf `poll-sleep-microseconds` with `range "0..100000"`; drive-by: correct the stale `external` description (ze-vpp-conf.yang:29-37 vs vpp.go:185-199)
- `internal/appliance/` image build - hugepage kernel cmdline (coordinate with spec-vpp-isolated-cpus) → concretely: `internal/appliance/config.go` (`ImageConfig` gains `Hugepages{PageSize, Count}` + `MemoryBytes`; bounds in `Validate`, config.go:130-172), `internal/appliance/cmd_build.go` (Validate call in buildOne; derived parent dir hand-off in runGokBuild, cmd_build.go:257-298), `internal/appliance/cmd_run.go` (`-m` from memory-bytes, cmd_run.go:154, :164)
- `gokrazy/kernel/runtime.config` + `gokrazy/kernel/runtime.require` - `CONFIG_HUGETLBFS` (require manifest enforces =y, build.py:143-160)
- `internal/core/diagnostic/codes.go` - register `doctor-vpp-hugepages` (title, description, examples)
- `mk/test-integration.mk` - `ze-vpp-hugepages-qemu-test` target + help line; add `./internal/component/vpp/...` to the `ze-qemu-integration-test` run list (new linux-tagged files)
- Docs (see Documentation Update Checklist): `docs/features.md`, `docs/guide/vpp.md`, `docs/guide/appliance.md`, `docs/research/vpp-deployment-reference.md`, `ai/INDEX.md`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | Yes | `internal/component/vpp/yang/ze-vpp-conf.yang`: `cpu/poll-sleep-microseconds`. Hugepage settings are deliberately NOT YANG: they are consumed on the build host at image-assembly time, so they live in the appliance config.json next to `size-bytes`/`kernel-profile` (`ImageConfig`, internal/appliance/config.go:53-57) |
| YANG validation constraints | Yes | native `range "0..100000"` on the new leaf; hugepage bounds in `applianceConfig.Validate` (JSON config surface) |
| YANG custom validators | N/A | native range is sufficient; no cross-field constraint on the YANG side |
| CLI commands/flags | N/A | no new CLI verb; appliance config is file-edited and validated at init/build |
| CLI grammar (action before identifier) | N/A | no CLI change |
| Editor autocomplete | N/A | automatic for a numeric YANG leaf; no dynamic completion needed |
| Functional test for new RPC/API | Yes | `test/parse/vpp-poll-sleep.ci`, `test/plugin/vpp-doctor-hugepages.ci`, `test/appliance/appliance-hugepages-validate.ci`, `test/appliance/vpp-hugepages-qemu.ci` |
| Pipe completeness | N/A | no command output added |
| Env var registration | N/A | no `environment/` leaves; `ai/rules/config-surface.md` decision table puts both knobs in config (operator tuning), not env |
| Doctor check for runtime dependencies | Yes | `internal/component/vpp/doctor_linux.go` (new, `diagnostic.RegisterDoctorCheck`), code `doctor-vpp-hugepages` in `internal/core/diagnostic/codes.go`, unit test `TestDoctorVPPHugepages`, functional `test/plugin/vpp-doctor-hugepages.ci` |
| Prometheus counters/metrics | N/A | boot reservation is static state surfaced by doctor and /proc; VPP's own memory behaviour is already exported by the vpp stats-segment telemetry (internal/component/vpp/telemetry.go). No new time-series |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` VPP Data Plane row (:83): poll-sleep + boot hugepage reservation; System Readiness row (:49): doctor-vpp-hugepages |
| 2 | Config syntax changed? | Yes | `docs/guide/vpp.md` config table (:171-176): new leaf row; `docs/guide/appliance.md`: `image.hugepages` + `image.memory-bytes`. `docs/architecture/config/syntax.md` N/A (no grammar change, one added leaf) |
| 3 | CLI command added/changed? | N/A | none |
| 4 | API/RPC added/changed? | N/A | none |
| 5 | Plugin added/changed? | N/A | no plugin inventory change (vpp component gains a leaf; covered by rows 1-2) |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` prerequisites section (:225-229): manual sysfs echo replaced by the appliance path on gokrazy (manual path remains for non-appliance hosts); `docs/guide/appliance.md` |
| 7 | Wire format changed? | N/A | none |
| 8 | Plugin SDK/protocol changed? | N/A | none |
| 9 | RFC behavior implemented/changed? | N/A | no protocol work |
| 10 | Test infrastructure changed? | Yes | new make target + evidence script: `ai/INDEX.md` dev-tools row, `mk/test-integration.mk` help block |
| 11 | Affects daemon comparison? | N/A | no comparison-table claim changes |
| 12 | Internal architecture changed? | Yes | `docs/research/vpp-deployment-reference.md`: poll-sleep-usec row (unix section) + appliance hugepage-reservation path in System Prerequisites |
| 13 | Route metadata keys added/changed? | N/A | none |
| 14 | Prometheus counters added/changed? | N/A | none |
| 15 | Registered plugin/event/command/capability inventory changed? | Yes | doctor check inventory: `docs/features.md` System Readiness row (:49) lists checks; add hugepages |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep 2026-07-10: anchors on `internal/component/vpp/{startupconf,config}.go` and `internal/appliance/cmd_build.go` exist in `docs/features.md`, `docs/guide/vpp.md`, `docs/guide/appliance.md`, `docs/guide/ze-install.md`; re-verify each anchored claim after the change |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/vpp.md` hugepage echo example (:229) and worker-core allocation text (:174, becomes stale only via spec-vpp-isolated-cpus); update the hugepage example to the appliance config path |

## Files to Create
- `test/plugin/vpp-poll-sleep.ci` - functional test → superseded (see Wiring Test): created as `test/parse/vpp-poll-sleep.ci`
- doctor check file(s) in the owning package (names at design) → named: `internal/component/vpp/doctor_linux.go` + `internal/component/vpp/doctor_linux_test.go` (sysfs/procfs root override for tests, following the host Detector procPath/sysfsPath precedent, internal/component/host/memory_linux.go:44-68)
- `internal/appliance/kernelargs.go` + `internal/appliance/kernelargs_test.go` - kernel-argument assembly seam (shared with spec-vpp-isolated-cpus) + derived instance config writer
- `test/plugin/vpp-doctor-hugepages.ci` (`option=needs-linux`)
- `test/appliance/appliance-hugepages-validate.ci`
- `test/appliance/vpp-hugepages-qemu.ci` - wrapper over the evidence script (self-skips without artifacts)
- `scripts/evidence/effective-vpp-hugepages-qemu.py` - builds an appliance with `image.hugepages`, boots the image directly in QEMU (`-m 1024`), SSHes in, asserts /proc/cmdline + /proc/meminfo
- `plan/spec-vpp-numa-smt.md` - split skeleton (created during this design session)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (~~skeleton - run `/ze-spec` RESEARCH/DESIGN first; implement AFTER spec-vpp-isolated-cpus~~ design complete 2026-07-10; implementation order vs spec-vpp-isolated-cpus is free -- whichever lands first creates the kernel-args seam, the other rebases) |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | every issue from critical review |
| 8. Re-verify | re-run stage 5 |
| 9. Repeat 6-8 | until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | two-commit closure per `ai/rules/planning.md` |

### Implementation Phases
1. **RESEARCH/DESIGN (not started)** - run the `/ze-spec` workflow: confirm A-1..A-3, scope the NUMA/SMT item (may split out), then fill ACs/tests above. Coordinate the boot-cmdline work with spec-vpp-isolated-cpus so both specs touch the appliance cmdline assembly once, not twice. → done 2026-07-10 (this session); phases below are the implementation plan.
2. **Phase: Wiring (MANDATORY FIRST)** - YANG leaf + `parseCPU` key + stub emission; `ImageConfig` fields + `Validate` bounds + Validate call wired into `buildOne`; `kernelargs.go` seam returning only base args. Failing tests: `TestStartupConfPollSleep`, `TestHugepageConfigValidate`, `TestKernelArgsHugepages`, `TestBuildValidatesConfig`.
3. **Phase: poll-sleep emission** - unix-section emission incl. explicit 0; parse bounds. Tests: `TestStartupConfPollSleep`, `TestParseCPUPollSleepBounds`, `test/parse/vpp-poll-sleep.ci`. AC-1, AC-2, AC-6.
4. **Phase: hugepage cmdline** - derived instance config (raw-JSON patch, temp parent dir), gok hand-off, `cmd_run` memory. Tests: `TestDerivedInstanceConfigPreservesFields`, `TestRunQEMUMemoryBytes`. AC-4, AC-7, AC-8, AC-13. Note: the derived parent dir starts with a cold gok builddir; correctness first, reuse (copy/symlink of `gokrazy/ze/builddir`) only if build time regresses noticeably.
5. **Phase: kernel profile** - `CONFIG_HUGETLBFS` in `runtime.config` + `runtime.require`; `make ze-kernel` proves resolution (A-5).
6. **Phase: doctor** - `doctor_linux.go` conditions AC-9..AC-12, code registration, `TestDoctorVPPHugepages`, `test/plugin/vpp-doctor-hugepages.ci`, add `./internal/component/vpp/...` to `ze-qemu-integration-test`.
7. **Phase: QEMU evidence** - `effective-vpp-hugepages-qemu.py` + make target + `.ci` wrapper. AC-3; validates A-4/A-6.
8. **Functional tests** - remaining `.ci` from the table.
9. **Docs + discovery** - Documentation Update Checklist rows; `ai/INDEX.md`.
10. **Full verification** → `make ze-verify`.
11. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N (1-4, 6-13) implemented with file:line; AC-5 documented as moved to spec-vpp-numa-smt |
| Feature completeness | all three End-to-End User Stories walk config → image → boot → doctor with no broken link |
| Correctness | `poll-sleep-usec` lands in the unix section (NOT cpu); kernel-arg order deterministic (`default_hugepagesz`, `hugepagesz`, `hugepages`); derived config preserves base `KernelExtraArgs` + unknown JSON fields; zero-config paths byte-identical (AC-2, AC-8) |
| Naming | YANG `poll-sleep-microseconds` (full words + unit suffix); JSON keys kebab-case (`page-size`, `memory-bytes`); Go `PollSleepMicroseconds` |
| Data flow | hugepage knowledge flows appliance config → build → cmdline only; doctor reads sysfs/procfs behind overridable roots; no runtime writes to hugepage sysfs (no side-channel host writes) |
| Registration over hardcoding | doctor check via `diagnostic.RegisterDoctorCheck` from the vpp component, not appended to central runChecks |
| Doctor checks | `doctor-vpp-hugepages` registered, explainable (`ze explain`), `TestDoctorCoverageCodesRegistered` passes |
| YANG validation | new leaf carries `range`; no bare `type string` added anywhere |
| Rule: qemu-testing | new `//go:build linux` files covered: vpp package in `ze-qemu-integration-test`; doctor `.ci` marked `needs-linux` |
| Rule: scope boundary | no isolcpus emission and no CPU validation added here (spec-vpp-isolated-cpus owns both); the kernelargs seam carries a comment pointing at that spec |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| poll-sleep emission | `go test ./internal/component/vpp -run TestStartupConfPollSleep` |
| parse bounds | `go test ./internal/component/vpp -run TestParseCPUPollSleepBounds` |
| hugepage config validation | `go test ./internal/appliance -run TestHugepageConfigValidate` |
| kernel-args assembly + JSON round-trip | `go test ./internal/appliance -run 'TestKernelArgs|TestDerivedInstanceConfig'` |
| build validates config | `go test ./internal/appliance -run TestBuildValidatesConfig` |
| QEMU memory honoured | `go test ./internal/appliance -run TestRunQEMUMemoryBytes` |
| doctor check + code registered | `go test ./internal/component/vpp -run TestDoctorVPPHugepages`; `go test ./internal/component/doctor -run TestDoctorCoverageCodesRegistered` |
| kernel symbol guaranteed | `make ze-kernel` succeeds with `CONFIG_HUGETLBFS` in runtime.require |
| boot evidence | `make ze-vpp-hugepages-qemu-test` prints PASS (or documented SKIP on hosts without artifacts) |
| functional coverage | `test/parse/vpp-poll-sleep.ci`, `test/plugin/vpp-doctor-hugepages.ci`, `test/appliance/appliance-hugepages-validate.ci` pass in their suites |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | poll-sleep bounded 0..100000 in both YANG and parse; hugepage count/page-size/memory-bytes bounded in `applianceConfig.Validate` BEFORE any value flows toward the kernel cmdline |
| Command/argument injection | computed kernel args are built exclusively from a validated enum (page-size) and integers (count); no operator-controlled string reaches /cmdline.txt; appliance name already re-validated in buildOne (cmd_build.go:109-111) |
| Resource exhaustion | 50%-of-declared-memory cap + 512 GiB hard cap + doctor shortfall warning prevent reserving the host into OOM |
| File handling | derived instance config written under a build temp dir and cleaned up; checked-in `gokrazy/ze/config.json` never modified in place |
| Secrets | none touched; the instance config carries no secret material |

### Discovery Updates (per `ai/rules/discovery-updates.md` Mechanical Checklist)
1. Where would an agent look first? `ai/INDEX.md`: dev-tools row for `ze-vpp-hugepages-qemu-test`; keyword "hugepages" → `docs/guide/vpp.md`.
2. What rule prevents regression? Existing rules own it: `ai/rules/qemu-testing.md` (QEMU evidence), `ai/rules/doctor-checks.md` (readiness). No new rule needed.
3. What source of truth prevents drift? `applianceConfig.Validate` (single validation point) and the diagnostic code registry; no static lists copied.
4. What verification proves it? `make ze-vpp-hugepages-qemu-test`, `TestDoctorCoverageCodesRegistered`, the `.ci` suites, `make ze-kernel` (require manifest).
5. What docs explain usage? Named in the Documentation Update Checklist (features.md, guide/vpp.md, guide/appliance.md, vpp-deployment-reference.md).
6. What learned record preserves the decision? Learned summary at closure; the unix-vs-cpu correction and the derived-instance-config mechanism are the structural lessons.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-3: VPP accepts `poll-sleep-usec` in the cpu section (skeleton wording, osvbng-derived) | It is a unix-section parameter ("Add a fixed-sleep between main loop poll", default 0); the cpu section takes only thread-placement keys | VPP 25.02 configuration reference checked during design (2026-07-10) | design-time only: emission targets the unix section; YANG placement stays under `cpu` (operator-logical grouping precedent: `memory` container feeds three different startup.conf sections) |
| Design constraint: add `./internal/component/vpp/...` to `ze-qemu-integration-test` because it gained `//go:build linux` files | That target runs `go test -tags integration` for tests needing a real kernel; my doctor files are NOT integration-tagged and already run under `make ze-unit-test` (`go test -race ./...`) on any linux host (GOOS=linux here). Adding the package would run zero of my tests. | Read the target (mk/test-integration.mk) + confirmed GOOS=linux runs the linux-tagged tests at implementation | dropped the no-op make change; the doctor tests are covered by the standard unit suite on linux |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| YANG leaf `poll-sleep-microseconds` under the existing `cpu` container, emitted into the unix section | (a) new `unix` YANG container mirroring VPP's file layout; (b) leaf named `poll-sleep-usec` matching VPP | Ze YANG groups by operator concern, not by startup.conf section (memory container → buffers/heapsize/statseg, startupconf.go:55-59, :107-117); `ai/rules/config-naming.md` bans abbreviations and wants unit suffixes |
| Hugepage reservation configured per-appliance in config.json (`image.hugepages`), not in ze.conf YANG | (a) derive from the seed ze.conf VPP settings at build; (b) runtime sysfs/sysctl reservation by Ze at start; (c) operator hand-edits `gokrazy/ze/config.json` | (a) bakes a runtime-mutable config into an immutable boot artifact -- config drift makes the cmdline silently stale, and the seed ze.conf is optional; (b) 1G pages are unreliable post-boot (fragmentation) and Ze would write host state outside the commit path; (c) global to all appliances, unvalidated, invisible to doctor. Drift between runtime VPP needs and boot reservation is exactly what the doctor check (AC-10) is for |
| Per-appliance args injected via a derived gokrazy instance config in a temp parent dir | patch `gokrazy/ze/config.json` in place and restore | in-place patching races concurrent builds, dirties the tree, and risks committing generated args; gok resolves `<parent_dir>/<instance>/config.json` (instanceflag.go:83-85) so a temp parent dir is the supported seam |
| One shared kernel-args assembly function (`internal/appliance/kernelargs.go`) for this spec and spec-vpp-isolated-cpus | each spec wires its own args into cmd_build | the user requirement is that both specs touch the cmdline assembly once; a single function with per-feature inputs keeps the second implementation a pure addition |
| Optional `image.memory-bytes` with a 50% reservation cap; static 512 GiB cap otherwise | no declared memory (runtime doctor only) | the build host cannot know target RAM; without a declared value AC-4 ("reservation exceeds appliance memory → verify rejects") is unimplementable at build time. Optional keeps zero-config behaviour; the same value fixes the hardcoded QEMU `-m 512` (AC-13) so evidence and operators share one path |
| `CONFIG_HUGETLBFS` added to runtime.config + runtime.require rather than trusting defconfig | assume defconfig enables it | whether defconfig sets HUGETLBFS is unverifiable in-repo; the require manifest turns the unknown into a build-time guarantee (build.py:143-160) |
| Doctor check owned by the vpp component (linux-tagged), single code `doctor-vpp-hugepages` for all conditions | central check in `internal/component/doctor/checks_linux.go` (like doctor-vpp-dpdk); one code per condition | `ai/rules/doctor-checks.md` ownership table explicitly routes VPP/procfs checks to the owning component; multi-message single code follows the doctor-vpp-dpdk precedent (checks_linux.go:639, :651) |
| NUMA/SMT split into `plan/spec-vpp-numa-smt.md` | include with concrete ACs + doctor check here | required host facts do not exist (no NUMA node on CoreInfo, no NIC numa_node, no node reader); placement validation depends on the unimplemented isolated-cpus selection helper; global sysctl surface missing. Including it would produce ACs bound to nonexistent surfaces |

## Design Insights
<!-- LIVE -->
- The gokrazy cmdline is three concatenated layers: kernel-package cmdline.txt,
  console args from `SerialConsole`, then `KernelExtraArgs` (write.go:71-135).
  Ze-side per-appliance args therefore need no gokrazy changes at all.
- `external` YANG description drift (says "still generates startup.conf",
  code skips at vpp.go:185-199) is why the poll-sleep functional test moved to
  the parse surface: no daemon path writes the file in a test-observable place.

## Known Limitations
- Skeleton only: acceptance criteria and tests above are provisional placeholders to be refined during DESIGN. → resolved: refined 2026-07-10, this design session.
- isolcpus sourcing/validation is owned by `plan/spec-vpp-isolated-cpus.md`, not here.
- NUMA/SMT awareness (per-node hugepage reservation, NIC-NUMA alignment, SMT sibling avoidance, `kernel.numa_balancing`) is owned by `plan/spec-vpp-numa-smt.md`.
- Hugepage reservation applies to gokrazy appliance deployments only; non-appliance hosts (external/systemd VPP) keep the documented manual path (docs/guide/vpp.md:225-229).
- Reservation changes require an image rebuild + reboot by design (boot-deterministic); the doctor check is the runtime drift detector, not a mutator.

## Implementation Summary
### What Was Implemented (2026-07-10, Opus)
- **poll-sleep (AC-1/2/6):** `CPUSettings.PollSleepMicroseconds *uint32` (config.go:55),
  `parseCPU` whitelist+bound 0..100000 (config.go:328,347), unix-section emission
  (startupconf.go:31), YANG leaf `poll-sleep-microseconds` + corrected stale `external`
  description (ze-vpp-conf.yang). Tests: `TestStartupConfPollSleep`,
  `TestParseCPUPollSleepBounds`, `test/parse/vpp-poll-sleep.ci` — all PASS.
- **hugepage config + cmdline (AC-4/7/8/13):** `ImageConfig.Hugepages`+`MemoryBytes`
  (config.go:53), `validateImageMemory` bounds (config.go), `kernelargs.go`
  (`hugepageKernelArgs`, `deriveInstanceConfigJSON`, `materializeDerivedParent`,
  `resolveBuildParentDir`), Validate wired into `buildOne` (cmd_build.go:114),
  derived parent dir in `runGokBuild`, `qemuMemoryMiB` in cmd_run.go. Tests:
  `TestKernelArgsHugepages`, `TestDerivedInstanceConfig*`, `TestMaterializeDerivedParent`,
  `TestHugepageConfigValidate`, `TestBuildValidatesConfig`, `TestRunQEMUMemoryBytes`,
  `test/appliance/appliance-hugepages-validate.ci` — all PASS.
- **doctor (AC-9..12):** `internal/component/vpp/doctor_linux.go` (`checkVPPHugepages`,
  `evaluateVPPHugepages`, sysfs/procfs readers behind overridable roots +
  `ze.test.doctor.hugepages-root` env override), `register_linux.go` registration,
  code `doctor-vpp-hugepages` in diagnostic/codes.go. Tests: `TestDoctorVPPHugepages`
  (9 cases), `test/plugin/vpp-doctor-hugepages.ci` (end-to-end via `ze doctor --json`)
  — all PASS.
- **kernel profile:** `CONFIG_HUGETLBFS` in gokrazy/kernel/runtime.config + runtime.require.
- **QEMU evidence (AC-3):** `scripts/evidence/effective-vpp-hugepages-qemu.py` +
  `ze-vpp-hugepages-qemu-test` target + `test/appliance/vpp-hugepages-qemu.ci` (self-skips).
- **docs:** vpp.md (leaf + prereq), appliance.md (image.hugepages), features.md,
  ai/INDEX.md. `make ze-lint-changed` = 0 issues; `make ze-doc-test` PASSED.

### Verified in this session vs infra-gated
- Verified (passing): AC-1, AC-2, AC-4, AC-6, AC-7, AC-8, AC-9, AC-10, AC-11, AC-12, AC-13
  (unit + functional), plus `-race` on vpp/appliance/diagnostic packages.
- Infra-gated (not empirically run in this env): AC-3 QEMU boot evidence self-SKIPs
  (sshpass absent; qemu/kvm/e2fsprogs/go present) and A-5 `make ze-kernel` (heavy kernel
  build) — both are by-design self-skipping / heavy; the mechanism is unit-tested.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial) — `make ze-validate` pre-check (2026-07-10)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | source anchor points to non-existent file `.claude/memory/project_gokrazy_appliance.md` | docs/guide/vpp.md:107 | fixed — repointed to `internal/plugins/init/main.go` (pre-existing broken anchor, surfaced by editing the file) |
| 2 | ISSUE (pre-existing) | exported `VPPSettings` has no cross-package non-test caller | internal/component/vpp/config.go:31 | pre-existing (in HEAD); same-package-only symbol unrelated to this spec; unexporting is a separate VPP-internals refactor beyond this spec — NOT done, flagged to user |
| 3 | ISSUE (pre-existing) | exported `ValidatePCIAddress` no cross-package non-test caller | internal/component/vpp/config.go:163 | pre-existing; same as #2 |
| 4 | ISSUE (pre-existing) | exported `ParseConfigSection` no cross-package non-test caller | internal/component/vpp/config.go:179 | pre-existing; same as #2 |
| 5 | ISSUE (pre-existing) | exported `ParseSettings` no cross-package non-test caller | internal/component/vpp/config.go:197 | pre-existing; same as #2 |
| 6 | ISSUE (pre-existing) | exported `GenerateStartupConf` no cross-package non-test caller | internal/component/vpp/startupconf.go:22 | pre-existing; same as #2 |

### Fixes applied
- docs/guide/vpp.md:107 — replaced the broken `.claude/memory/...` source anchor with `internal/plugins/init/main.go` (gokrazy PID-1 lifecycle).
- The 5 exported-symbol findings pre-date this spec (all present in HEAD, used same-package: `verifyVPPConfig`→`ParseSettings`, `writeStartupConf`→`GenerateStartupConf`, etc.). My necessary edits to config.go/startupconf.go merely pulled those files into `ze-validate`'s changed-file scope. Resolving them means unexporting VPP-internal symbols (a separate VPP-internals refactor beyond this spec, with cross-package-test risk) so they are surfaced to the user rather than fixed under this spec. The generic adversarial `/ze-review` (multi-agent) has NOT yet been run; that step and closure await the user's decision on the pre-existing findings and the infra-gated QEMU/kernel evidence.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Full `/ze-spec` DESIGN completed and approved before implementation
- [ ] `make ze-test` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)
- [ ] QEMU evidence for boot-cmdline behaviour
- [ ] AC-1..AC-4, AC-6..AC-13 all demonstrated (AC-5 moved to spec-vpp-numa-smt)
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Documentation Update Checklist rows executed with source evidence

### Quality Gates (SHOULD pass)
- [ ] `docs/research/vpp-deployment-reference.md` updated with the new knobs

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
