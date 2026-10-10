# Spec: docker-hosts-run-the-ze-kernel

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 3/7 |
| Handoff | - |
| Updated | 2026-10-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner instruction (Thomas, 2026-10-09): "we should NEVER use docker image without the latest 7.x kernel".

Owner answers to the first design (2026-10-09), which replace its exact-version pin:

-> Decision (owner, D-1 and D-2): "either our own build or a recent kernel supporting all the features we need for Ze". The requirement is FEATURE-based. A Docker host is acceptable when its kernel provides every kernel feature Ze needs, and the required list derives from the declaration Ze already uses, never from a second hand-written list. Ze's own build is one way to meet it; a distro kernel that has everything is equally fine.
-> Decision (owner, D-3): "if the kernel of github is recent enough, good enough, check". Checked below: it is not, so the QEMU-guest fallback is the nightly route.
-> Decision (owner, D-4): "how could we have wrong kernel, it should not be possible - fail". Every Docker run that runs Ze fails on a host lacking a required feature, with no exemption. The check sits where Ze-running Docker work starts; the kernel build runs no Ze, so it never reaches the check and needs no exemption.
-> Decision (owner, D-5, 2026-10-10): option B, "docker needs our kernel". Every Docker host runs EXACTLY the appliance kernel, so every interop result comes from the kernel the appliance ships. There is no separate docker-host profile: the options Docker itself needs are added to `gokrazy/kernel/runtime.config` and declared once in `runtime.require` (AC-14). D-1's "a distro kernel that has everything is equally fine" still describes what the derived check accepts; D-5 fixes which kernel Ze's own hosts install.
-> Constraint (D-5): the Docker option list is derived from moby's `contrib/check-config.sh` "Generally Necessary" section, read at moby commit `9fabd6dfbb926381278c436a866bc8d6ac222274` (master, 2026-10-10), never from memory. For a kernel at 5.3 or later its `check_flags` there name: `NAMESPACES NET_NS PID_NS IPC_NS UTS_NS CGROUPS CGROUP_CPUACCT CGROUP_DEVICE CGROUP_FREEZER CGROUP_SCHED CPUSETS MEMCG KEYS VETH BRIDGE BRIDGE_NETFILTER IP_NF_FILTER IP_NF_MANGLE IP_NF_TARGET_MASQUERADE IP6_NF_FILTER IP6_NF_MANGLE IP6_NF_TARGET_MASQUERADE NETFILTER_XT_MATCH_ADDRTYPE NETFILTER_XT_MATCH_CONNTRACK NETFILTER_XT_MATCH_IPVS NETFILTER_XT_MARK IP_NF_RAW IP_NF_NAT NF_NAT IP6_NF_RAW IP6_NF_NAT POSIX_MQUEUE CGROUP_BPF`, and a cgroup v2 hierarchy with the `cpu cpuset io memory pids` controllers. The owner also named overlayfs, the user namespace and seccomp, which the same script lists under "Optional Features" (`OVERLAY_FS`, `USER_NS`, `SECCOMP SECCOMP_FILTER`, `CGROUP_PIDS`); those are added too, because Docker's default storage driver and default seccomp profile use them.

The symptom. `mobike-initiator` and `mobike-responder` fail on every nightly and on the Mac (`plan/spec-interop-image-copies-a-prebuilt-ze.md`, AC-9: `34 passed, 3 failed`). Ze offers MOBIKE only when the kernel answers `XFRM_MSG_MIGRATE_STATE` (`xfrmMigrationAvailable`, `internal/component/ike/dataplane/xfrm_migrate_linux.go`), which first appears in Linux 7.2. The suite spends 15 minutes and reports two protocol failures that are a host fact.

Goals.

| ID | Goal |
|----|------|
| G-1 | The set of kernel features a Docker host must provide is DERIVED from Ze's kernel capability enrolment (`internal/component/kernelcap`), and that enrolment covers every feature the Docker labs exercise |
| G-2 | Every Docker run that runs Ze (interop labs, `docker-*` deployment proofs, terminal-demo render) probes the daemon's kernel for that whole set before any image build, and fails naming each missing feature |
| G-3 | Every Docker host Ze's labs use (colima on the Mac, the Linux amd64 host, the scheduled nightly) has a kernel that passes, by its own kernel or by Ze's build |
| G-4 | `mobike-initiator` and `mobike-responder` pass on the Mac and on the nightly |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/doctor-and-health-checks.md` - the kernel capability tier `kernelcap` implements
  → Decision: a subsystem enrols its kernel feature with `kernelcap.MustRegister` from its own `init()` (`Capability`: `Subsystem`, `Kernel` spelled as the `CONFIG_` symbol, `Degrades`, `InUse`, `Probe`). No shared package enumerates subsystems. That registry is the one declaration the Docker-host check derives from.
  → Constraint: a `Probe` uses netlink, procfs or a syscall and MUST NOT execute an external binary (`kernelcap.go`, `Capability.Probe`).
- [ ] `plan/immediate/spec-kernel-capability-gate.md` (blocked) - the enrolment's own spec
  → Constraint: "Only MPLS and XFRM enrol" (its Known Limitations): L2TP, PPPoE, nftables and interface kinds "each joins the enrolment later". This spec enrols the features the Docker labs exercise; it does not enrol the rest.
  → Constraint: "ESP has no unprivileged runtime probe" (same table): `CONFIG_XFRM_USER` without `CONFIG_INET_ESP` accepts every SA and drops every ESP packet. The ESP enrolment needs a probe design (A-2).
- [ ] `docs/architecture/testing/interop.md` - labs, preflights, "IPsec address movement"
  → Constraint: "The Docker host must support `XFRM_MSG_MIGRATE_STATE`" is prose with no check today; this spec makes it a derived refusal and rewrites that paragraph.
- [ ] `docs/architecture/appliance/kernel-profiles.md` - profile registry (`internal/appliance/kernelreg.go`)
  → Decision: a profile is discovered from `<profile>.config` + `<profile>.require`; `# ze-base: <profile>` stacks one base. D-5: Ze's Docker-host kernel IS the `runtime` profile, with Docker's options added to it; no new profile.
- [ ] `docs/architecture/testing/qemu-integration.md` - `./le test qemu run kernel <vmlinuz> packages ...` boots Alpine on Ze's kernel under KVM or HVF
- [ ] `docs/architecture/testing/ci-workflows.md` - `evidence-nightly.yml` runs `interop`, `interop-ipsec`, `docker-l2tp-ppp-test`, `docker-pppoe-accel-test` on `ubuntu-latest`
- [ ] `ai/rules/platform-linux.md` - its sentence "the Alpine QEMU VM has no Docker" becomes false under the nightly route and is edited in the same change

**Key insights:**
- GitHub `ubuntu-latest` fails the feature set. `actions/runner-images` README: "`ubuntu-latest` or `ubuntu-24.04`" maps to Ubuntu 24.04 x64. `images/ubuntu/Ubuntu2404-Readme.md`, read 2026-10-10: "Kernel Version: 6.17.0-1022-azure", "Image Version: 20261004.327.1". 6.17 predates 7.2, so it has no `XFRM_MSG_MIGRATE_STATE` and MOBIKE cannot run there. The nightly therefore keeps the QEMU-guest route (D-3 fallback). The README also warns `-latest` moves; the derived check, not this note, is what decides on each run.
- The Mac fails too: colima's VM runs Ubuntu `6.8.0-117-generic`, and its `/boot/config-6.8.0-117-generic` reads `# CONFIG_XFRM_MIGRATE is not set`.
- The colima VM (`vmType: vz`, Ubuntu 24.04, grub-efi-arm64 2.12) boots `/boot/vmlinuz-<release>` through EFI grub with no initramfs (`root=PARTUUID=...`). Installing Ze's arm64 build there is the ordinary Linux route: files in `/boot` and `/lib/modules`, `update-grub`, restart. Ze's arm64 build emits the uncompressed `Image` as `vmlinuz` (`workerArch`, `internal/appliance/kernelbuilder/worker.go`).
- Tools installed 2026-10-09: colima 0.10.3, limactl 2.2.0, docker client 29.8.2, server 29.5.2.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/kernelcap/kernelcap.go` - `MustRegister`, `Enrolled`, `Evaluate(tree)`, `Refuse(tree)`. Evaluation is config-driven: `InUse` false means nothing is probed. An undetermined probe is a warning, never a refusal, for the daemon.
- [ ] `internal/component/ike/engine/kernelcap_linux.go` - enrols `ipsec`, `CONFIG_XFRM_USER`, probe `kernelcap.XFRM`.
- [ ] `internal/plugins/fib/kernel/kernelcap_linux.go` - enrols `mpls` (`CONFIG_MPLS_ROUTING`) and `mpls-transit-mtu` (Degrades).
- [ ] `internal/component/ike/dataplane/xfrm_migrate_linux.go` - `xfrmMigrationAvailable`: sends `XFRM_MSG_MIGRATE_STATE` with `AF_UNSPEC`, so `ESRCH` proves the handler exists without mutation. It is not enrolled in `kernelcap`.
- [ ] `internal/le/interoplab/lab.go` - `Suite.Run`: `Docker.Probe`, then `Suite.Preflight` (where `StageBinaries` cross-compiles ze for the daemon's arch), then image builds. A setup failure sets `SetupError`, `Code = 1` and runs nothing.
- [ ] `internal/le/interoplab/docker.go` - `ServerArchitecture` asks the daemon, never the host, and refuses an unreadable answer; `RunOneShot` runs a throwaway container.
- [ ] `internal/le/test/deployment/`, `internal/le/site/terminaldemo/render.go` - the other two places that start Ze in Docker.
- [ ] `.github/workflows/evidence-nightly.yml`, `.github/workflows/qemu-nightly.yml` - Docker jobs on `ubuntu-latest`; `qemu-nightly` already caches and boots the amd64 runtime kernel under KVM.

**Behavior to preserve:**
- The daemon's own gate: `kernelcap.Refuse(tree)` stays config-driven, and an undetermined probe stays a warning for the daemon.
- `SuiteReport` JSON and text shape; a refusal is `interop: setup: <reason>`, exit 1.

**Behavior to change:**
- A Docker run that runs Ze probes the daemon's kernel for EVERY enrolled capability, regardless of configuration, and fails when any is absent or undetermined.
- The enrolment gains the capabilities the Docker labs exercise.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test integration <lab>`, `./le test deployment docker-*`, the terminal-demo render.

### Transformation Path
1. The caller probes the daemon and stages the ze binary for the daemon's arch (`StageBinaries`).
2. It runs that staged ze in a throwaway container on the daemon (`RunOneShot`, with the network capability the probes need and its own network namespace), invoking a `ze doctor` mode that evaluates every enrolled capability with `InUse` ignored and answers JSON: one row per capability with subsystem, `CONFIG_` symbol, state and reason.
3. le reads the rows. Any `absent` or `unknown` row fails the run, naming every failing row, the daemon's kernel release, and the routes to a passing kernel.
4. Only then do image builds start.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le ↔ Docker daemon | `docker run --rm` of the staged ze | No |
| ze in container ↔ host kernel | the same native probes the daemon uses (netlink, procfs, syscalls); a container shares the daemon's kernel | No |
| ze ↔ le | JSON rows on stdout | No |

### Integration Points
- `kernelcap` registry: the source of the required set; no le-side list.
- `Suite.Run` after `Preflight`, the `docker-*` deployment proofs, the terminal-demo render: the three callers of one le function.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the kernel that matters is the daemon's, probed from a container on it; the le host (macOS) is never asked |
| No unintended coupling (components stay isolated) | Yes | le reads JSON from ze; it imports neither `kernelcap` nor any enrolling package |
| No duplicated functionality (extends existing, does not recreate) | Yes | the probes are the daemon's own enrolled probes; `xfrmMigrationAvailable` becomes an enrolment, not a copy |
| Zero-copy preserved where applicable | N-A | tooling |
| Registration over hardcoding, outbound | Yes | each new capability enrols from its owning package's `init()` |
| Registration over hardcoding, inbound | Yes | the le check holds no feature names; a capability enrolled tomorrow is required of every Docker host with no le edit. Searched: `internal/le/interoplab/` (no kernel feature list today), each lab's `Preflights(...)` (the L2TP and RSVP-TE labs load modules by name: those stay as lab setup, not as the requirement) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | a probe run in a container sees the daemon host's kernel features | containers share the kernel; module autoload requests reach the host kernel | a feature present on the host reads absent, or the reverse | AC-3 and AC-5 on colima before and after the kernel install | unvalidated |
| A-2 | each feature the labs exercise has a native, mutation-free or namespace-confined probe: `XFRM_MSG_MIGRATE_STATE` (exists), xfrm interface (rtnetlink link kind), WireGuard (generic netlink family), L2TP (generic netlink family), PPPoE (socket family), ESP and every AEAD and cipher Ze's XFRM algorithm table installs | `xfrmMigrationAvailable`; the gate spec's "ESP has no unprivileged runtime probe" | an unprobeable feature cannot be required; it is named as a gap and the owner decides | each enrolment's probe test | partly confirmed 2026-10-10: MOBIKE, ESP v4/v6 and all nine XFRM transforms probe mutation-free (`XFRM_MSG_UPDSA` on reserved SPI 1 answers `ESRCH` after the kernel built the state); on colima 6.8 MOBIKE reads absent (`EINVAL`), the rest present, a bogus cipher absent, no SA left (`TestXFRMKernelProbesOnThisHost`, commit ecfa32e1fd). Open: WireGuard and L2TP (generic netlink family lookup), PPPoE (`AF_PPPOX` socket), xfrm interface (rtnetlink create needs a netns-confined probe and `CAP_SYS_ADMIN`) |
| A-3 | colima's vz VM boots a kernel installed in its own `/boot` through grub, with no initramfs | `/proc/cmdline` `BOOT_IMAGE=/vmlinuz-6.8.0-117-generic root=PARTUUID=...`; no initrd in `/boot` | install route fails on the Mac; fall back to lima `images[].kernel` direct boot | AC-8 | unvalidated |
| A-4 | the Linux amd64 host either passes with its own kernel or boots through grub | owner statement of a Linux host; not inspected | the install step differs | AC-9 run on the host | unvalidated |
| A-5 | Docker runs inside the Alpine QEMU guest booted on Ze's amd64 kernel under KVM on `ubuntu-latest` | Alpine packages `docker`; `qemu-nightly.yml` boots Ze's kernel there with `/dev/kvm` | nightly route fails; owner decides between a self-hosted runner on the Linux host and dropping the hosted runner | AC-10 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a capability's probe answers `unknown` in a container (missing privilege, seccomp) | every run fails on that row | the container is granted what the labs' own containers are granted; an `unknown` still fails (D-4) and names the reason |
| R-2 | `colima delete` or an upgrade re-creates the VM and Ze's kernel is gone | next run fails naming `XFRM_MSG_MIGRATE_STATE` | the failure names `./le setup docker-kernel install` |
| R-3 | the colima VM's Ubuntu installs a newer distro kernel that grub ranks first and that lacks a feature | run fails naming the feature | the install action sets `GRUB_DEFAULT` to Ze's entry by title |
| R-4 | GitHub moves `ubuntu-latest` to a kernel that passes | the QEMU-guest job still runs | AC-10's job prints the hosted runner's own verdict too, so the move is seen and the route can be simplified |
| R-5 | enrolling a capability changes `ze doctor` and the daemon gate for operators | a new doctor row on an appliance | each enrolment carries `InUse` and, where the subsystem still works without it, `Degrades`, so operators see a row only for configured subsystems |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | every Docker lab, Docker deployment proof and demo render fails on a host missing an enrolled feature; new enrolments add doctor rows and, for a non-degrading capability, a startup refusal on an appliance whose config uses that subsystem |
| How is it reverted? | single commit revert per phase; an installed kernel stays in the VM and is harmless |
| Who else touches this path? | `plan/immediate/spec-kernel-capability-gate.md` (owns `kernelcap`; this spec extends its enrolment), `plan/immediate/spec-appliance-kernel-vpn-modules.md` (adds ESP, xfrm interface and WireGuard to `runtime.config`), `plan/pre-release/spec-appliance-ships-ze-kernel.md` (the 7.2.9 pin Ze's own build uses), `plan/spec-terminal-demo-showcase.md`, `plan/spec-interop-image-copies-a-prebuilt-ze.md` (AC-9 depends on this), and the IKE sibling session working under `internal/component/ike/` (the MOBIKE enrolment lands there and must be sequenced with it) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration interop-ipsec` on a daemon whose kernel lacks a feature | → | the le kernel-feature check in `Suite.Run` | `TestSuiteRefusesDockerHostMissingAKernelFeature` (`internal/le/interoplab/lab_test.go`, recording `docker` on PATH answering the ze JSON) |
| `./le test deployment docker-l2tp-ppp-test` on the same daemon | → | same check | `TestDockerDeploymentRefusesMissingKernelFeature` (`internal/le/test/deployment/`) |
| terminal-demo render on the same daemon | → | same check | `TestRenderRefusesMissingKernelFeature` (`internal/le/site/terminaldemo/render_test.go`) |
| the `ze doctor` all-capabilities mode | → | `kernelcap` evaluation with `InUse` ignored | `TestDoctorAllCapabilitiesIgnoresConfig` (`internal/component/kernelcap/kernelcap_test.go`) |
| `ike` dataplane `init()` | → | `XFRM_MSG_MIGRATE_STATE` enrolment | `TestMOBIKEMigrationIsEnrolled` (`internal/component/ike/dataplane/`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | the `ze doctor` all-capabilities mode, any host | answers one JSON row per enrolled capability (subsystem, `CONFIG_` symbol, state, reason), probing each regardless of configuration; the row set equals `kernelcap.Enrolled()` |
| AC-2 | the enrolment after this spec | includes `XFRM_MSG_MIGRATE_STATE` (MOBIKE, Degrades), xfrm interface, ESP, each AEAD and cipher Ze's XFRM algorithm table installs (derived from that table, not listed), WireGuard, L2TP and PPPoE, each from its owning package; any of them A-2 finds unprobeable is named in this spec as a gap for the owner, not silently dropped |
| AC-3 | a Docker run that runs Ze, daemon kernel lacking any enrolled feature | exits 1 before any image build or container start, naming every absent feature by subsystem and `CONFIG_` symbol, the daemon's kernel release, and the two routes (a kernel with the feature, or `./le setup docker-kernel install`); no scenario counted as passed, failed or skipped |
| AC-4 | a row is `unknown`, the probe container fails, or the JSON is unreadable | fails as AC-3, naming what it ran and what it read; never proceeds |
| AC-5 | every row `present` | proceeds to image builds unchanged |
| AC-6 | the kernel build (`ze appliance kernel`, Docker backend) | runs no Ze, never calls the check, and builds on any daemon: this is not an exemption, it is outside the check's callers |
| AC-7 | every `Capability.Kernel` symbol enrolled | appears in `gokrazy/kernel/runtime.require` (a test compares them), so Ze's own build passes the check by construction |
| AC-8 | Mac: `./le setup docker-kernel install` with Ze's arm64 kernel cached | installs it into colima's VM, sets grub's default by title, restarts colima; the next Docker run passes AC-5 |
| AC-9 | Linux amd64 host | the check passes with the host's own kernel, or after the same install action and an operator reboot; the action never reboots the host itself |
| AC-10 | `evidence-nightly.yml` jobs `interop`, `ipsec-interop`, `l2tp-interop`, `pppoe-interop` | run inside the Alpine QEMU guest booted on Ze's cached amd64 kernel under KVM; the log shows the check passing in the guest and, as information, the hosted runner's own failing rows |
| AC-11 | a capability enrolled later by any package | is required of every Docker host with no le change (unit test enrolling a fake capability) |
| AC-12 | `./le test integration interop-ipsec` on the Mac after AC-8 | `mobike-initiator` and `mobike-responder` pass; this is the evidence `plan/spec-interop-image-copies-a-prebuilt-ze.md` AC-9 cites |
| AC-13 | the same suite on the nightly after AC-10 | `mobike-initiator` and `mobike-responder` pass |
| AC-14 | `gokrazy/kernel/runtime.config` and `runtime.require` (D-5) | carry every symbol of the D-5 constraint list, each once, with a comment citing moby `contrib/check-config.sh` at the recorded commit; the appliance kernel's own build then refuses a config that loses one, and a Docker daemon starts on the appliance kernel (the colima VM after AC-8, the QEMU guest of AC-10). No kernel build is started by the config edit itself |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoctorAllCapabilitiesIgnoresConfig` | `internal/component/kernelcap/kernelcap_test.go` | AC-1 | |
| `TestSuiteRefusesDockerHostMissingAKernelFeature` | `internal/le/interoplab/lab_test.go` | AC-3, AC-5; no `docker build` in the recorded argv after a refusal | |
| `TestDockerHostKernelUnknownRefuses` | `internal/le/interoplab/lab_test.go` | AC-4 | |
| `TestDockerHostCheckHoldsNoFeatureNames` | `internal/le/interoplab/lab_test.go` | AC-11 | |
| `TestEnrolledKernelSymbolsAreInRuntimeRequire` | `internal/appliance/kernelreq_test.go` | AC-7 | |
| `TestMOBIKEMigrationIsEnrolled` and one enrolment test per AC-2 capability | each owning package | AC-2 | |
| `TestXFRMAlgorithmEnrolmentFollowsTheTable` | the IKE dataplane package | AC-2: every algorithm the table installs is enrolled | |

### Boundary Tests (numeric inputs)
N-A: the check takes no numeric input; it compares states, not versions.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| failure on today's colima | recorded run of `./le test integration interop-ipsec` before AC-8 | developer gets one failure naming `XFRM_MSG_MIGRATE_STATE` in seconds, not 15 minutes of scenario failures | |
| MOBIKE on the Mac | recorded run after AC-8 | AC-12 | |
| MOBIKE on the nightly | `evidence-nightly` run | AC-13 | |

### Interop Tests (Scope: protocol)
N-A as a new scenario: no protocol behavior changes. The existing `mobike-initiator` and `mobike-responder` go from red (6.8) to green on a passing kernel; that pair is the discrimination record for AC-12.

## Files to Modify
- `internal/component/kernelcap/kernelcap.go` - evaluation of every enrolment with `InUse` ignored, for the doctor mode
- `internal/component/doctor/` - the all-capabilities mode and its JSON answer
- `internal/component/ike/dataplane/` - enrol `XFRM_MSG_MIGRATE_STATE`, ESP, the XFRM algorithm set, xfrm interface (sequenced with the IKE sibling session)
- the WireGuard, L2TP and PPPoE owning packages - their enrolments
- `internal/le/interoplab/lab.go`, `internal/le/interoplab/docker.go` - the check after `Preflight`
- `internal/le/test/deployment/`, `internal/le/site/terminaldemo/render.go` - call the same check
- `internal/le/setup/setup.go` - `docker-kernel install` action and a setup check that runs the same probe
- `internal/core/diagnostic/codes.go` - codes for each new enrolment
- `.github/workflows/evidence-nightly.yml` - the four Docker jobs inside the QEMU guest
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - Docker's own kernel options (D-5, AC-14)
- `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/testing/interop.md`, `docs/architecture/testing/qemu-integration.md`, `docs/architecture/testing/ci-workflows.md`, `docs/guide/developer-setup.md`
- `ai/rules/platform-linux.md` - "the Alpine QEMU VM has no Docker" is no longer true

## Files to Create
- None. D-5 put the Docker options into `gokrazy/kernel/runtime.config` and `runtime.require` (Files to Modify), so no `docker-host` profile exists

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | doctor mode and le tooling, no config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | a `ze doctor` all-capabilities mode; `./le setup docker-kernel install` |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md` for both; the doctor mode's JSON envelope follows the CLI contract |
| Editor autocomplete | N-A | |
| Functional test for new RPC/API | Yes | a `.ci` for the doctor mode's JSON under a stand-in proc root, as the doctor tier's fixtures do |
| Pipe completeness | Yes | the doctor mode's output through the existing pipes |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | each new enrolment is a doctor row with its codes |
| Prometheus counters/metrics | N-A | |
| BGP family surface | N-A | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | new doctor rows for enrolled subsystems and the all-capabilities JSON mode: `docs/features/ai-first.md` (declared by `internal/component/doctor/`) describes ze doctor's machine-readable answers and gains the mode |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` for the `ze doctor` mode |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | MOBIKE (RFC 4555) interop evidence becomes runnable; `rfc/short/rfc4555.md` carriers cite the passing run |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`, `qemu-integration.md`, `ci-workflows.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md` at implementation; `lab.go` declares `interop.md` and `docs/features/interoperability-testing.md`; `render.go` and `internal/le/dockerhost/dockerhost.go` declare `docs/architecture/core-design.md`, unaffected: it states le's one-import-per-tool composition and Docker as the pinned demo renderer, neither of which changes |
| 17 | Existing docs show examples for this area? | Yes | doctor examples; `developer-setup.md` colima section |

## Implementation Steps

1. **Phase: Wiring** -- the le check in `Suite.Run` after `Preflight`, the deployment proofs and the render, over a recorded ze JSON answer; failing wiring tests
2. **Phase: Doctor mode** -- every enrolment evaluated with `InUse` ignored, JSON rows; AC-1
3. **Phase: Enrolments** -- MOBIKE migration first (it unblocks AC-12), then ESP and the algorithm set, xfrm interface, WireGuard, L2TP, PPPoE; AC-2, AC-7
4. **Phase: Mac** -- record today's failure, then Ze's arm64 kernel into colima (`./le setup docker-kernel install`); AC-8, AC-12
5. **Phase: Linux host** -- AC-9
6. **Phase: Nightly** -- the four jobs in the QEMU guest; AC-10, AC-13
7. **Phase: Docs and rule**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N at file:line or a recorded run |
| Correctness | the probe runs on the DAEMON's kernel, never on the le host |
| Derivation | le holds no feature name; the required set is `kernelcap`'s enrolment, and the enrolled symbols are checked against `runtime.require` |
| Rule: principles (silent value) | `unknown`, a failed container and unreadable JSON each fail; none reads as present |
| Rule: no-layering | no version check beside the feature check, no warn-and-continue mode |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| derived requirement | `TestDockerHostCheckHoldsNoFeatureNames`, `TestEnrolledKernelSymbolsAreInRuntimeRequire` |
| failure on 6.8 | recorded run naming `XFRM_MSG_MIGRATE_STATE` |
| Mac passes | recorded `interop-ipsec` run with both MOBIKE scenarios green |
| nightly passes | `evidence-nightly` log |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the JSON from the container is untrusted text: bounded size, known fields only, quoted in the failure |
| Privilege | the probe container gets only what the labs' own containers get; the install action runs root steps inside colima via `colima ssh`, and on the Linux host states each `sudo` step and never reboots |
| Mutation | every probe is mutation-free or confined to the probe container's own network namespace |

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

- A version names a kernel; a feature names what Ze needs. Ubuntu's 6.8 ships `# CONFIG_XFRM_MIGRATE is not set`, so even a recent distro version proves nothing until its features are probed.
- The daemon's gate asks "does this config's subsystem have its feature"; the Docker-host check asks "does this kernel have every feature any Ze lab may exercise". Same enrolment, two questions, so `InUse` is the only switch between them.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Required set = `kernelcap` enrolment, probed by the staged ze in a container on the daemon | an le-side list; reading `/boot/config-<release>` or `/proc/config.gz` against `runtime.require` | a list is a second declaration (owner: never); kernel config files are absent on some hosts (Ze's own kernel, no `/proc/config.gz` on the colima Ubuntu) and a symbol says nothing about a module that fails to load |
| Check runs after `Preflight` (ze staged), before image builds, in the three Ze-running callers | before staging; inside each lab's own preflight list | the probe needs the ze binary for the daemon's arch; a per-lab list is an enumeration each new lab must remember |
| `unknown` fails | warn as the daemon does | owner D-4: a wrong kernel "should not be possible - fail" |
| Kernel build is not a caller | an exemption flag | it runs no Ze, so it never reaches the check (owner D-4) |
| Nightly: QEMU guest on Ze's amd64 kernel | Docker directly on `ubuntu-latest` | its kernel `6.17.0-1022-azure` lacks `XFRM_MSG_MIGRATE_STATE` (runner-images Ubuntu2404-Readme, 2026-10-10) |
| Mac: install Ze's kernel into colima's own VM through grub | lima `images[].kernel` direct boot; a distro 7.x kernel | colima rewrites `lima.yaml` on start; a distro kernel is acceptable whenever it passes the check, but none is in hand that does |

## Known Limitations

- Ze's own build, its pin and its digest belong to `plan/pre-release/spec-appliance-ships-ze-kernel.md`; this spec uses whatever kernel passes.
- `ipsec-bgp-redistribute-frr` fails for a separate, journalled cause (`plan/journal/unwired-feature.md`, `plan/journal/test-against-broken-path.md`).
- nftables, policy routing, traffic shaping, eBPF and other interface kinds stay unenrolled unless a Docker lab exercises them; `plan/immediate/spec-kernel-capability-gate.md` keeps them.
- D-5 closed (option B): no separate kernel profile. A distro kernel still passes the derived check when it has every feature, but Ze's own hosts install the appliance kernel.

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
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test (N-A: Scope is tooling)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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
- [ ] Boundary tests for all numeric inputs (N-A: none)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no protocol change; MOBIKE red before and green after is recorded under AC-12)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md` only, in the same `./le commit create` script
