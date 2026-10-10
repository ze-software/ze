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
-> Decision (owner, D-6, 2026-10-10): "we can use qemu, there is no reason to not have docker use on linux and qemu on mac". On Linux, Ze's Docker labs run natively on a host whose kernel has every enrolled feature (its own kernel, or Ze's after `./le setup docker-kernel install` and an operator reboot). On macOS, Ze's Docker-based labs (interop suites, `docker-*` and the other Docker deployment proofs, the terminal-demo render) run inside a QEMU guest under HVF booted on Ze's own arm64 runtime kernel, with Docker inside that guest. colima is no longer the Ze lab host, and the colima kernel-install route is dropped: `./le setup docker-kernel install` is Linux-only. The Mac and the nightly (Ze's amd64 kernel under KVM) share ONE "run the Docker lab inside the Ze-kernel QEMU guest" path. The derived kernel check still runs, inside the guest, and must pass there.
-> Constraint (D-6): booting that guest needs (a) Ze's runtime kernel for the guest's architecture from the kernel cache (`ze appliance kernel` writes `tmp/kernel/build/vmlinuz`, cache under `~/.cache/ze/runtime-kernel/<version>-runtime-<arch>-...`; on 2026-10-10 the Mac holds arm64 entries `d1de5ab8` (2026-10-02) and `8e843adc` (2026-09-27), both built before the D-5 Docker options landed in `runtime.config` on 2026-10-10, so the key has changed and a fresh build is owed before the guest can be trusted to start Docker), and (b) a guest image that carries Docker (Alpine's `docker` package through `./le test qemu run ... packages`, A-5). No kernel build is started by this spec's agents; the build is the owner's to run.
-> Constraint (D-5): the Docker option list is derived from moby's `contrib/check-config.sh` "Generally Necessary" section, read at moby commit `9fabd6dfbb926381278c436a866bc8d6ac222274` (master, 2026-10-10), never from memory. For a kernel at 5.3 or later its `check_flags` there name: `NAMESPACES NET_NS PID_NS IPC_NS UTS_NS CGROUPS CGROUP_CPUACCT CGROUP_DEVICE CGROUP_FREEZER CGROUP_SCHED CPUSETS MEMCG KEYS VETH BRIDGE BRIDGE_NETFILTER IP_NF_FILTER IP_NF_MANGLE IP_NF_TARGET_MASQUERADE IP6_NF_FILTER IP6_NF_MANGLE IP6_NF_TARGET_MASQUERADE NETFILTER_XT_MATCH_ADDRTYPE NETFILTER_XT_MATCH_CONNTRACK NETFILTER_XT_MATCH_IPVS NETFILTER_XT_MARK IP_NF_RAW IP_NF_NAT NF_NAT IP6_NF_RAW IP6_NF_NAT POSIX_MQUEUE CGROUP_BPF`, and a cgroup v2 hierarchy with the `cpu cpuset io memory pids` controllers. The owner also named overlayfs, the user namespace and seccomp, which the same script lists under "Optional Features" (`OVERLAY_FS`, `USER_NS`, `SECCOMP SECCOMP_FILTER`, `CGROUP_PIDS`); those are added too, because Docker's default storage driver and default seccomp profile use them.

The symptom. `mobike-initiator` and `mobike-responder` fail on every nightly and on the Mac (`plan/spec-interop-image-copies-a-prebuilt-ze.md`, AC-9: `34 passed, 3 failed`). Ze offers MOBIKE only when the kernel answers `XFRM_MSG_MIGRATE_STATE` (`xfrmMigrationAvailable`, `internal/component/ike/dataplane/xfrm_migrate_linux.go`), which first appears in Linux 7.2. The suite spends 15 minutes and reports two protocol failures that are a host fact.

Goals.

| ID | Goal |
|----|------|
| G-1 | The set of kernel features a Docker host must provide is DERIVED from Ze's kernel capability enrolment (`internal/component/kernelcap`), and that enrolment covers every feature the Docker labs exercise |
| G-2 | Every Docker run that runs Ze (interop labs, `docker-*` deployment proofs, terminal-demo render) probes the daemon's kernel for that whole set before any image build, and fails naming each missing feature |
| G-3 | Every Docker host Ze's labs use has a kernel that passes: a Linux host natively (its own kernel or Ze's build), and on the Mac and the scheduled nightly the Ze-kernel QEMU guest (D-6) |
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
- (Superseded by D-6: colima is no longer the Ze lab host, so this route is not taken.) The colima VM (`vmType: vz`, Ubuntu 24.04, grub-efi-arm64 2.12) boots `/boot/vmlinuz-<release>` through EFI grub with no initramfs. Ze's arm64 build emits the uncompressed `Image` as `vmlinuz` (`workerArch`, `internal/appliance/kernelbuilder/worker.go`), which is what `./le test qemu run kernel <vmlinuz>` boots under HVF.
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
| A-1 | a probe run in a container sees the daemon host's kernel features | containers share the kernel; module autoload requests reach the host kernel | a feature present on the host reads absent, or the reverse | AC-3 on colima (6.8, before D-6) and AC-5 in the Ze-kernel QEMU guest (D-6) | unvalidated |
| A-2 | each feature the labs exercise has a native, mutation-free or namespace-confined probe: `XFRM_MSG_MIGRATE_STATE` (exists), xfrm interface (rtnetlink link kind), WireGuard (generic netlink family), L2TP (generic netlink family), PPPoE (socket family), ESP and every AEAD and cipher Ze's XFRM algorithm table installs | `xfrmMigrationAvailable`; the gate spec's "ESP has no unprivileged runtime probe" | an unprobeable feature cannot be required; it is named as a gap and the owner decides | each enrolment's probe test | partly confirmed 2026-10-10: MOBIKE, ESP v4/v6 and all nine XFRM transforms probe mutation-free (`XFRM_MSG_UPDSA` on reserved SPI 1 answers `ESRCH` after the kernel built the state); on colima 6.8 MOBIKE reads absent (`EINVAL`), the rest present, a bogus cipher absent, no SA left (`TestXFRMKernelProbesOnThisHost`, commit ecfa32e1fd). Confirmed 2026-10-10 for the rest: L2TP and WireGuard by generic netlink `CTRL_CMD_GETFAMILY` (`ENOENT` absent; a name over `GENL_NAMSIZ` answers `EINVAL`, so the negative control is `zenosuchfam`), L2TP sessions and PPPoE by an `AF_PPPOX` socket open (`pppox_create` requests the module, no privilege check), xfrm interface by `RTM_NEWLINK`+`NLM_F_CREATE` of kind `xfrm` with if_id 0 inside a throwaway netns (`xfrmi_newlink` answers `EINVAL` "if_id must be non zero" before registering; `EOPNOTSUPP` "Unknown device type" is absent; needs `CAP_SYS_ADMIN`, so the probe container is granted it). Kernel source read: torvalds/linux master `net/core/rtnetlink.c`, `net/xfrm/xfrm_interface_core.c`, `drivers/net/ppp/pppox.c`, `pppoe.c`, `net/l2tp/l2tp_ppp.c`. On colima 6.8 in a container with NET_ADMIN+SYS_ADMIN: wireguard present, PPPoE present, xfrm interface present, l2tp ABSENT (no l2tp family), PPPoL2TP ABSENT (`EPROTONOSUPPORT`), bogus link kind absent |
| A-3 | the Alpine QEMU guest under HVF boots Ze's arm64 runtime kernel and runs a Docker daemon in it (D-6) | `./le test qemu run kernel <vmlinuz>` already boots Ze's arm64 kernel under HVF (`docs/architecture/testing/qemu-integration.md`); Alpine packages `docker`; D-5 put Docker's kernel options into `runtime.config` | the Mac route fails; the owner decides the next route | AC-8 | unvalidated (needs a kernel built after the D-5 config change; no build started) |
| A-4 | the Linux amd64 host either passes with its own kernel or boots through grub | owner statement of a Linux host; not inspected | the install step differs | AC-9 run on the host | unvalidated |
| A-5 | Docker runs inside the Alpine QEMU guest booted on Ze's amd64 kernel under KVM on `ubuntu-latest`, through the same guest path as A-3 | Alpine packages `docker`; `qemu-nightly.yml` boots Ze's kernel there with `/dev/kvm` | nightly route fails; owner decides between a self-hosted runner on the Linux host and dropping the hosted runner | AC-10 | unvalidated (four jobs wired at 8bd1742581, not pushed, never run) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a capability's probe answers `unknown` in a container (missing privilege, seccomp) | every run fails on that row | the container is granted what the labs' own containers are granted; an `unknown` still fails (D-4) and names the reason |
| R-2 | the Mac's cached arm64 runtime kernel predates a `runtime.config` change (true on 2026-10-10: the D-5 options landed after both cache entries) | the guest's Docker daemon fails to start, or the check names a missing feature | the failure names the kernel build (`ze appliance kernel`) as the route; the guest never boots a stock Alpine kernel (`Run.assertRuntimeKernel`) |
| R-3 | a Linux host's distro installs a newer kernel that grub ranks first and that lacks a feature | run fails naming the feature | the Linux install action sets `GRUB_DEFAULT` to Ze's entry by title |
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
| AC-3 | a Docker run that runs Ze, daemon kernel lacking any enrolled feature | exits 1 before any image build or container start, naming every absent feature by subsystem and `CONFIG_` symbol, the daemon's kernel release, and the routes to a passing kernel (D-6: on Linux a kernel with the feature or `./le setup docker-kernel install`; on macOS the Ze-kernel QEMU guest); no scenario counted as passed, failed or skipped |
| AC-4 | a row is `unknown`, the probe container fails, or the JSON is unreadable | fails as AC-3, naming what it ran and what it read; never proceeds |
| AC-5 | every row `present` | proceeds to image builds unchanged |
| AC-6 | the kernel build (`ze appliance kernel`, Docker backend) | runs no Ze, never calls the check, and builds on any daemon: this is not an exemption, it is outside the check's callers |
| AC-7 | every `Capability.Kernel` symbol enrolled | appears in `gokrazy/kernel/runtime.require` (a test compares them), so Ze's own build passes the check by construction |
| AC-8 | Mac (D-6): a Docker lab run with Ze's arm64 runtime kernel cached | runs inside the Alpine QEMU guest under HVF booted on that kernel, with Docker in the guest; the kernel check runs in the guest and passes AC-5; colima is never the lab host |
| AC-9 | Linux amd64 host | the check passes with the host's own kernel, or after the same install action and an operator reboot; the action never reboots the host itself |
| AC-10 | `evidence-nightly.yml` jobs `interop`, `ipsec-interop`, `l2tp-interop`, `pppoe-interop` | run inside the Alpine QEMU guest booted on Ze's cached amd64 kernel under KVM, through the same guest path as AC-8; the log shows the check passing in the guest and, as information, the hosted runner's own failing rows |
| AC-11 | a capability enrolled later by any package | is required of every Docker host with no le change (unit test enrolling a fake capability) |
| AC-12 | `./le test integration interop-ipsec` on the Mac after AC-8 | `mobike-initiator` and `mobike-responder` pass; this is the evidence `plan/spec-interop-image-copies-a-prebuilt-ze.md` AC-9 cites |
| AC-13 | the same suite on the nightly after AC-10 | `mobike-initiator` and `mobike-responder` pass |
| AC-14 | `gokrazy/kernel/runtime.config` and `runtime.require` (D-5) | carry every symbol of the D-5 constraint list, each once, with a comment citing moby `contrib/check-config.sh` at the recorded commit; the appliance kernel's own build then refuses a config that loses one, and a Docker daemon starts on the appliance kernel (the QEMU guest of AC-8 and AC-10). No kernel build is started by the config edit itself |

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
| MOBIKE on the nightly | `evidence-nightly` run | AC-13 | job wired at 8bd1742581 (local, not pushed): `ipsec-interop` runs `./le test qemu docker-lab lab "test integration interop-ipsec"` on the cached amd64 runtime kernel; no run yet |
| AC-10 workflow shape | `internal/le/workflowcheck/workflowcheck_test.go` `TestEvidenceNightlyScheduleActionsAndPrivileges` | the four Docker lab jobs restore, build on a miss and save the amd64 runtime kernel cache under qemu-nightly's key, run their lab through `docker-lab`, and run the host check as information; `radius-interop` stayed native at 8bd1742581 and runs through `docker-lab` too since 3ac4057f32 | red on HEAD's workflow (no docker-lab, no cache, no host check), green at 8bd1742581. Record correction (review r1 NOTE 11): 8bd1742581's body says the narrowing of this test's qemu ban (`test qemu/` refused, now `test qemu/docker-lab` admitted alone) has a row in `test/weakened/d1987d95.md`; the commit carried none, because `./le commit create` pruned the row at create time (the journal row committed in e663493f91). The justification stands here instead: D-6 makes `docker-lab` the route every Docker lab takes on Ze's kernel, and every other `test qemu/` action stays refused |
| AC-10 RFC tier survives the guest | `internal/le/rfc/tags_test.go` `TestALabRunInsideTheDockerLabGuestIsScheduled`, `TestNativeActionsInWorkflowCommands` | a lab run as `docker-lab lab "<le words>"` credits its interop tree to the scheduled workflow, read from the `le-words` keyword the registry declares (`leaction.ValueLeWords`) | red under a cut forcing no nested keyword, green at 8bd1742581; the stale shared `bin/le` refused `RFC8654-4-1` as unrun until `./le --update` rebuilt it |
| `ze doctor config <file>` grammar | `test/ui/doctor-refuses-bare-config-path.ci` | a bare path after `ze doctor` is refused, naming `ze doctor config <file>` | red observed 2026-10-10: the `.ci` copied into a `git archive` export of `c5ee8bf12f` (1ecb27591e^), run there with `./le test ui doctor-refuses-bare-config-path`, failed `exit_code_mismatch`: "cmd seq=1 (ze doctor --json empty.conf): expected exit code 1, got 0". Green at 1ecb27591e |

### Interop Tests (Scope: protocol)
N-A as a new scenario: no protocol behavior changes. The existing `mobike-initiator` and `mobike-responder` go from red (6.8) to green on a passing kernel; that pair is the discrimination record for AC-12.

## Files to Modify
- `internal/component/kernelcap/kernelcap.go` - evaluation of every enrolment with `InUse` ignored, for the doctor mode
- `internal/component/doctor/` - the all-capabilities mode and its JSON answer
- `internal/component/ike/dataplane/` - enrol `XFRM_MSG_MIGRATE_STATE`, ESP, the XFRM algorithm set, xfrm interface (sequenced with the IKE sibling session)
- the WireGuard, L2TP and PPPoE owning packages - their enrolments
- `internal/le/interoplab/lab.go`, `internal/le/interoplab/docker.go` - the check after `Preflight`
- `internal/le/test/deployment/`, `internal/le/site/terminaldemo/render.go` - call the same check
- `internal/le/setup/setup.go` - `docker-kernel install` action (Linux hosts only, D-6) and a setup check that runs the same probe
- `internal/le/test/qemu/` - the one "Docker lab inside the Ze-kernel guest" path the Mac and the nightly share (D-6)
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
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md` (its colima section changes: colima is no longer the Ze lab host, D-6) |
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
| 17 | Existing docs show examples for this area? | Yes | doctor examples; `developer-setup.md` colima section (rewritten for D-6) |

## Implementation Steps

1. **Phase: Wiring** -- the le check in `Suite.Run` after `Preflight`, the deployment proofs and the render, over a recorded ze JSON answer; failing wiring tests
2. **Phase: Doctor mode** -- every enrolment evaluated with `InUse` ignored, JSON rows; AC-1
3. **Phase: Enrolments** -- MOBIKE migration first (it unblocks AC-12), then ESP and the algorithm set, xfrm interface, WireGuard, L2TP, PPPoE; AC-2, AC-7
4. **Phase: Mac** -- record today's failure, then the Docker labs inside the Ze-kernel QEMU guest under HVF, the path the nightly shares (D-6); AC-8, AC-12
5. **Phase: Linux host** -- `./le setup docker-kernel install`, Linux only (D-6); AC-9
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
| Privilege | the probe container gets only what the labs' own containers get; the install action is Linux-only (D-6): it states each `sudo` step and never reboots; the Mac guest runs in QEMU with no host root step |
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
| Mac: Docker labs inside the Alpine QEMU guest under HVF on Ze's arm64 kernel, one path shared with the nightly (owner D-6) | install Ze's kernel into colima's VM through grub; lima `images[].kernel` direct boot | owner: "we can use qemu, there is no reason to not have docker use on linux and qemu on mac"; the QEMU runner already boots Ze's kernel and refuses any other (`Run.assertRuntimeKernel`) |

## Known Limitations

- Ze's own build, its pin and its digest belong to `plan/pre-release/spec-appliance-ships-ze-kernel.md`; this spec uses whatever kernel passes.
- `ipsec-bgp-redistribute-frr` fails for a separate, journalled cause (`plan/journal/unwired-feature.md`, `plan/journal/test-against-broken-path.md`).
- nftables, policy routing, traffic shaping, eBPF and other interface kinds stay unenrolled unless a Docker lab exercises them; `plan/immediate/spec-kernel-capability-gate.md` keeps them.
- D-5 closed (option B): no separate kernel profile. A distro kernel still passes the derived check when it has every feature, but Ze's own hosts install the appliance kernel.

## Blocked (2026-10-10)

| Item | Blocked on | What unblocks it |
|------|-----------|------------------|
| AC-8, A-3, AC-12, AC-14: Docker lab in the guest under HVF | the Docker kernel check refuses `mpls-transit-mtu (CONFIG_MPLS_IP_MTU): unknown: the MPLS label space holds no unreserved label to address`. Measured 2026-10-10 on the Mac (HVF, arm64) with `./le test qemu docker-lab timeout 1800s`, log `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/docker-lab-nolab-4.log`: the guest booted cache entry `7.2.9-runtime-arm64-runtime-7c0d8f79-bf3ac1c9` ("Runtime kernel confirmed in the guest: 7.2.9", daemon kernel `7.2.9-ze`), dockerd started under OpenRC, and that row was the only one not present (ipsec-mobike, l2tp, l2tp-ppp and mpls present). Cause: `kernelcap.MPLSIPMTU` answers unknown while `net.mpls.platform_labels` is 16 or less, and the probe container's fresh network namespace starts at 0 under every kernel, so no kernel passes the check as built (R-1). Each lab's preflight runs the same check, so no lab runs either; the IPsec and BGP labs were not started | owner picks the fix: (a) the probe container runs `--privileged` and sizes the label space in its own namespace before `ze doctor` (an MPLS name in the check, against AC-11); (b) `MPLSIPMTU` asks from a private network namespace it unshares and sizes itself, never changing the caller's (changes `ze doctor` on hosts); (c) unknown on this row stops failing the check (against D-4) |
| Answered by the first guest boot (2026-10-10) | dockerd under OpenRC on the live Alpine ISO: starts. Go in the guest: `/usr/local/go` (run.go). The guest le as the probing `ze`: NO, a `ze_le` build started as `ze` answers "unknown command: doctor" (`docker-lab-nolab.log`); fixed, the host cross-builds `tmp/qemu/linux-<arch>/ze` with the daemon tags, and the guest check and the nightly's informational step probe with it. A suite's one-scenario selector could not reach the guest; fixed with `docker-lab env "NAME=value"`. Docker images on the 16G tmpfs root: not reached | the first lab run after AC-8 answers the last one |
| AC-12: MOBIKE on the Mac | AC-8 | `./le test qemu docker-lab lab "test integration interop-ipsec"` |
| AC-9: Linux amd64 host | the amd64 runtime kernel build on the Linux host, then `./le setup docker-kernel install` (never run: HARD STOP) and an operator reboot | owner runs the install on the Linux host |
| AC-10, A-5, AC-13: the nightly | the workflow is committed locally (8bd1742581) and not pushed; no run has booted the guest under KVM on `ubuntu-latest`; a cold cache builds the amd64 kernel in each of the four jobs | owner push, then the first scheduled or `workflow_dispatch` run |
| AC-14: a Docker daemon starts on the appliance kernel | AC-8 or AC-10 | the first guest boot of either |

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
