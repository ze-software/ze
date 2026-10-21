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
-> Constraint (owner, M1 in `ai/META-DECISIONS.md`, 2026-10-20): this spec designs an Alpine guest (D-6 (b), A-3, A-5, AC-8, AC-10, the `platform-linux.md` and `qemu-integration.md` rows), and M1 rejects Alpine for VMs and test guests: the guest runs Ze's own system (Ze's kernel and base, as the appliance builds it). Every Alpine guest named in this spec moves to Ze's system before implementation resumes; an Alpine-only problem is out of scope. Recorded 2026-10-21 as a pointer, not yet a redesign: the affected ACs and assumptions are re-written in the next design pass.
-> Pointer (2026-10-21, `plan/spec-ze-image-docker-host.md`): the M1 move is done for the Docker guest. One image, Ze's docker-host image (gokrazy, Ze's runtime kernel `7.2.9-ze`, static dockerd and containerd), now serves both routes. D-6's Mac route is the `ze` Colima profile booting that image (`./le setup docker-host start`, 22e1b9e8e7; macOS labs use its daemon or refuse, af4c0708ed), with the QEMU `docker-lab` guest beside it. AC-8 and AC-10's guest is that image under QEMU (`./le test qemu docker-lab`, ca386b6ae3): the Alpine ISO, apk, the guest le and `ZE_DOCKER_LAB_GUEST` left the docker-lab path. Proof there: AC-9 (boot and check, `verdict pass`, kernel `7.2.9-ze`) and AC-10 (one Ctrl-C, `left behind: none`, 63d1b8561d). D-6 (b) and A-3's Alpine guest are superseded by it. The other Alpine `Run` guests (`run`, `stress`, `mobike`, `all-tests`, the kernel builder's QEMU backend) are a journal row in `plan/journal/gate-verdict-depends-on-the-machine.md` (M1).
-> Constraint (D-5): the Docker option list is derived from moby's `contrib/check-config.sh` "Generally Necessary" section, read at moby commit `9fabd6dfbb926381278c436a866bc8d6ac222274` (master, 2026-10-10), never from memory. For a kernel at 5.3 or later its `check_flags` there name: `NAMESPACES NET_NS PID_NS IPC_NS UTS_NS CGROUPS CGROUP_CPUACCT CGROUP_DEVICE CGROUP_FREEZER CGROUP_SCHED CPUSETS MEMCG KEYS VETH BRIDGE BRIDGE_NETFILTER IP_NF_FILTER IP_NF_MANGLE IP_NF_TARGET_MASQUERADE IP6_NF_FILTER IP6_NF_MANGLE IP6_NF_TARGET_MASQUERADE NETFILTER_XT_MATCH_ADDRTYPE NETFILTER_XT_MATCH_CONNTRACK NETFILTER_XT_MATCH_IPVS NETFILTER_XT_MARK IP_NF_RAW IP_NF_NAT NF_NAT IP6_NF_RAW IP6_NF_NAT POSIX_MQUEUE CGROUP_BPF`, and a cgroup v2 hierarchy with the `cpu cpuset io memory pids` controllers. The owner also named overlayfs, the user namespace and seccomp, which the same script lists under "Optional Features" (`OVERLAY_FS`, `USER_NS`, `SECCOMP SECCOMP_FILTER`, `CGROUP_PIDS`); those are added too, because Docker's default storage driver and default seccomp profile use them.
-> Decision (owner, D-7, 2026-10-10), verbatim: "The host should be setup correctly for ze, if apparmor is mis-setup ze doctor should report it and we should make sure to have it right. if tooling can ensure admin does the right thing let's have the tooling". The probe is never unconfined silently and `unknown` is never accepted. `ze doctor`, on the appliance and on any host running ze, tells "the host's security policy denied the probe" apart from "the kernel lacks the feature" and reports it as its own diagnostic naming the policy and the fix; the Docker kernel check still refuses a denied row, naming that cause. On Linux `./le setup docker-kernel check` detects the daemon's AppArmor and names the exact command, and a confirmed setup action loads a Ze profile that grants the probe exactly what it needs.
-> Constraint (D-7, research): Docker's `docker-default` profile is generated from `github.com/moby/profiles/apparmor` `template.go`, which moby master (`9fabd6dfbb926381278c436a866bc8d6ac222274`, 2026-10-10) pins at `apparmor/v0.2.2` (commit `61eaf32614c7c71b60bd8927d3e6a4ffc8ff1f31`). It denies the probe twice: `deny mount,` (the private mount namespace and the `/proc/sys` bind, observed on colima as `make the probe's mounts private: permission denied`) and `deny @{PROC}/sys/[^k]** w,  # deny /proc/sys except /proc/sys/k* (effectively /proc/sys/kernel)` (the `platform_labels` write, which even an allowed mount would still meet). An AppArmor `deny` rule outranks every allow rule, so no profile that keeps that line can grant the write. A profile is compiled and loaded with `apparmor_parser -r -W <file>` (replace, write the cache) and survives a reboot when the file lives in `/etc/apparmor.d/`; a container runs under it with `docker run --security-opt apparmor=<name>`, and `docker info` lists `name=apparmor` in `SecurityOptions` when the daemon applies AppArmor.
-> Decision (D-7 design, least privilege): a new `kernelcap.StateDenied` ("denied"): a probe step that fails with EACCES while `/proc/self/attr` names a confining label other than `unconfined` is denied by that policy, and its reason names the label, the denied step and the grant. Ze's profile `ze-kernel-probe` is one embedded file in `internal/component/kernelcap`, next to the probe whose needs it grants, so the grant and the need are declared once and a unit test compares them. It keeps docker-default's network, signal, ptrace and capability rules, replaces `file,` with reads, the exec of `/ze` and the ONE write `/proc/sys/net/mpls/platform_labels`, and replaces `deny mount,` with exactly the three mounts `remountSysctlWritable` makes (`rprivate` on `/`, the bind of `/proc/sys` onto itself, its read-write remount). The Docker kernel check runs the probe under `--security-opt apparmor=ze-kernel-probe` when the daemon reports AppArmor, refuses before the run when a local Linux host has not loaded it, and names `./le setup docker-kernel apparmor confirm ze-kernel-probe`. That action is Linux-only, prints its root steps and runs none without `confirm`, and is never run by an agent against a real host. The QEMU guest's kernel carries no AppArmor, so the guest route is unchanged; were it to gain AppArmor, the same check path applies. The VRRP lab's `apparmor=unconfined` (`internal/le/interoplab/bgp/prepare.go`) is the other unconfined container and stays outside this step.
-> Decision (D-7, lab containers, 2026-10-10): every Ze AppArmor profile registers in `interoplab` (`RegisterAppArmorProfile`: name, what runs under it, what docker-default denies it, text, next command); a peer names its profile in `PeerConfig.AppArmorProfile`, the container starts with `--security-opt apparmor=<name>` on a daemon applying AppArmor, a Linux client lacking it refuses naming `./le setup docker-kernel apparmor confirm <name>`, and that action loads any registered profile and lists them all. The VRRP scenarios' ze runs under the generic `ze-lab-net-sysctl` (docker-default with `/proc/sys` writes denied outside `net/`, `deny mount,` kept) instead of `apparmor=unconfined`; the per-lab `ze-lab-vrrp` was deleted in review round 2, because it denied `net/ipv4/icmp_errors_use_inbound_ifaddr`, which VRRP writes for every IPv4 group. `ze-kernel-probe` and the deleted `ze-lab-vrrp` parsed under `apparmor_parser -Q` 4.0.1 in `ubuntu:24.04` (`scratch/aa2/parse.log`). The parse of `ze-lab-net-sysctl`, which the VRRP lab now depends on, is owed by `plan/pre-release/spec-lab-containers-least-privilege.md` AC-2 (its "profile parse" functional row), not by this spec. Verdicts for every container that leaves docker-default (`git grep` of `apparmor=unconfined`, `--privileged`, `seccomp=unconfined` in non-test Go; `--privileged` also drops AppArmor confinement):
-> Decision (owner, D-7, 2026-10-10): "make it generic" means ONE shared AppArmor profile, `ze-lab-net-sysctl`, for every lab container that writes network sysctls, granting only what the labs together need; no per-lab profiles. `TestVRRPWritesOnlyLabGrantedSysctls` (`internal/plugins/vrrp/lab_profile_linux_test.go`) records every sysctl `applyDataplaneSysctls` writes for both families and checks the profile grants each.

| Site | Use | Verdict |
|------|-----|---------|
| `internal/le/interoplab/bgp/prepare.go` `scenarioPeers` (VRRP ze) | was `apparmor=unconfined` + `systempaths=unconfined` | REPLACED by `ze-lab-net-sysctl` (was `ze-lab-vrrp`, deleted in review round 2); `systempaths=unconfined` stays because Docker mounts `/proc/sys` read-only, and AppArmor confines the writes |
| `internal/le/interoplab/ipsec/ipsec.go` `dockerPrivileged` (strongSwan and ze) | was `--privileged` | REPLACED (`dba10b4c75`): `NET_ADMIN` for nat, strongSwan and ze, `--sysctl` for the NAT box's keys, and the shared `ze-lab-net-sysctl` profile (`spec-lab-containers-least-privilege` AC-1, AC-2) |
| `privilegedArgument` in `internal/le/interoplab/l2tp/l2tp.go`, `pppoe/pppoe.go`, `rsvpte/rsvpte.go` | was `--privileged` | REPLACED: L2TP (`6d6f2fb308`) and PPPoE (`bdee1599a3`) carry `NET_ADMIN` and `--device /dev/ppp`, no `modprobe`, no `/lib/modules`; RSVP-TE (`9cf91dc469`) carries `NET_ADMIN`, `systempaths=unconfined` and `ze-lab-net-sysctl` (AC-3, AC-4, AC-5) |
| `internal/le/test/deployment/l2tp.go`, `vppiface.go` (their `docker run` argv) | was `--privileged` | REPLACED: L2TP (`dc39259013`) `NET_ADMIN` and `--device /dev/ppp`; VPP (`6ac9aa5b10`, `28df8df562`) `NET_ADMIN`, `--device /dev/net/tun` and `--device /dev/vhost-net`, no hugepages and no `IPC_LOCK`, settled by a run on an amd64 Linux host (AC-6, AC-7) |
| `internal/le/verify/evidence/evidence.go` (asserted by `internal/test/fixture/ui_fixture_le_evidence_answers.go`) | was `--privileged` | REPLACED (`e0d84318e9`, owner decision 2026-10-10): one `--cap-add` per capability of the `caps=` vocabulary, derived by `runner.ContainerCapsDockerNames` (SYS_TIME left out), and one `--device` per node of the `option=opens-device` vocabulary (`runner.ContainerDevicePaths`); Docker's default seccomp and AppArmor stay on. A full verify inside it is owed |
| `internal/le/site/terminaldemo/render.go` | was `seccomp=unconfined`; `--privileged` per demo | `seccomp=unconfined` REMOVED (`bf2d59fa9e`): `SYS_ADMIN` already admits `mount`, `unshare` and `setns` under the default profile. `--privileged` stays per demo, and each privileged demo states its `privileged-reason` in the manifest, refused when missing (`2d429ec8be`, owner decision 2026-10-10) |

Every row except the VRRP one (replaced here, `84d07bf11c`) is owned by `plan/pre-release/spec-lab-containers-least-privilege.md`, where the owner said "implement"; it carries the evidence, the owner's decisions and what is still owed there: the IPsec lab run and its grant-removal reds, the `ze-lab-net-sysctl` parse, a full verify inside the evidence container, and the terminal-demo render run. This spec owes no scope answer on those rows.

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
| A-5 | Docker runs inside the Alpine QEMU guest booted on Ze's amd64 kernel under KVM on `ubuntu-latest`, through the same guest path as A-3 | Alpine packages `docker`; `qemu-nightly.yml` boots Ze's kernel there with `/dev/kvm` | nightly route fails; owner decides between a self-hosted runner on the Linux host and dropping the hosted runner | AC-10 | unvalidated (four jobs wired at 8bd1742581, which is on origin/main; the last nightly, run 38043229048, built 1d0e7a0228, which predates it, so no run has booted the guest yet) |

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
| AC-15 | a probe step fails with EACCES and the process is confined by a security label other than `unconfined` (D-7) | the row reads `denied`, never `unknown` or `absent`; its reason names the label (`AppArmor profile docker-default (enforce)`), the denied step, and the grant (`ze-kernel-probe`); EACCES with no confining label, and EPERM, stay `unknown` |
| AC-16 | `ze doctor` with a config using a subsystem whose probe answered denied | its own diagnostic `doctor-kernel-capability-denied`, WARNING (ze starts, as for unknown), message naming the policy and the fix; `ze doctor --json kernel-capabilities` exits 1 on a denied row |
| AC-17 | the Docker kernel check with a denied row | refuses as AC-3, the row's reason in the text, plus the exact command that loads Ze's profile |
| AC-18 | the Docker kernel check, daemon reporting `name=apparmor` | runs the probe container under `--security-opt apparmor=ze-kernel-probe`; on a Linux client whose `/sys/kernel/security/apparmor/profiles` lacks the profile, refuses before the run naming `./le setup docker-kernel apparmor confirm ze-kernel-probe`; with no AppArmor, the argv is unchanged |
| AC-19 | `./le setup docker-kernel apparmor` | Linux only; refuses where AppArmor is not enabled or `apparmor_parser` is absent; `confirm <name>` loads any profile registered in `interoplab` (`ze-kernel-probe`, `ze-lab-net-sysctl`); without a registered name prints every profile's root steps and the command for each (install the profile into `/etc/apparmor.d/`, `apparmor_parser -r -W`) and runs none; never run by an agent against a real host |
| AC-20 | the embedded `ze-kernel-probe` profile | names itself, carries no `deny mount` and no `file,`, grants the three mounts and the one write the probe makes (derived from `ProcPath("sys")` and `MPLSPlatformLabelsPath()` in a test), keeps docker-default's read denies, and parses under `apparmor_parser -Q` |
| AC-21 | `gokrazy/kernel/runtime.config` and `runtime.require` (owner, 2026-10-14: "fix the kernel config") | carry what a Linux Docker host that boots the Ze kernel relies on: `HOTPLUG_PCI`, `HOTPLUG_PCI_ACPI`, `HOTPLUG_PCI_PCIE` (slot-named interfaces such as `ens18` keep their name) and Ubuntu's stock UFW matches and targets (`NETFILTER_XT_MATCH_LIMIT`, `_RECENT`, `_HL`, `NETFILTER_XT_TARGET_LOG`, `IP_NF_TARGET_REJECT`, `IP6_NF_TARGET_REJECT`, `IP6_NF_MATCH_HL`, `IP6_NF_MATCH_RT`), plus `IP6_NF_MATCH_IPV6HEADER` and `NETFILTER_XT_MATCH_STATE`, which hand-written rule sets load and only the defconfig set, each `=y` and declared once |
| AC-22 | `./le setup docker-kernel install`, cached kernel lacking an option the host relies on (owner, 2026-10-14: "add the check") | refuses before any step, naming each dependency (a slot-named interface in `/sys/class/net` or netplan's generated networkd file names; the iptables modules in `/proc/modules`; UFW installed), what the kernel would break, the missing options and the rebuild; a loaded netfilter module the one table does not map is refused by name. `TestZeKernelServesHostRefusesWhatBrokeTheOwnersHost` refuses the `7.2.9-ze` config and accepts the fixed one; `TestHostDependencyOptionsAreInTheRuntimeProfile` keeps the table inside `runtime.config` |
| AC-23 | `install confirm` on a host whose `/boot/grub/grub.cfg` is root-only | reads it through `sudo cat` after the plain read is refused for permission (`TestReadGrubConfigFallsBackToSudo`; journal row in `plan/journal/gate-verdict-depends-on-the-machine.md`) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoctorAllCapabilitiesIgnoresConfig` | `internal/component/kernelcap/kernelcap_test.go` | AC-1 | |
| `TestSuiteRefusesDockerHostMissingAKernelFeature` | `internal/le/interoplab/lab_test.go` | AC-3, AC-5; no `docker build` in the recorded argv after a refusal | |
| `TestDockerKernelUnknownRefuses` | `internal/le/interoplab/kernelcheck_test.go` | AC-4 | |
| `TestDockerKernelRefusesNamingEveryMissingFeature` (AC-11 is asserted inside it: the check holds no feature names of its own) | `internal/le/interoplab/kernelcheck_test.go` | AC-11 | |
| `TestEnrolledKernelSymbolsAreInRuntimeRequire` | `internal/appliance/kernelreq_test.go` | AC-7 | |
| `TestMOBIKEMigrationIsEnrolled` and one enrolment test per AC-2 capability | each owning package | AC-2 | |
| `TestXFRMAlgorithmEnrolmentFollowsTheTable` | the IKE dataplane package | AC-2: every algorithm the table installs is enrolled | |
| `TestForcedDeniedReachesBothReaders` | `internal/component/kernelcap/kernelcap_test.go` | AC-16: a forced denied reaches `ProbeAll` and `Evaluate` alike, each naming the variable | red 2026-10-10 with the `denied` arm of `forcedState` removed, green restored |
| `TestKernelCapabilitiesAnswer/one_denied` | `internal/component/doctor/kernel_capabilities_test.go` | AC-16: a denied row is not ready, so kernel-capabilities exits 1 | red 2026-10-10 with denied counted as a pass, green restored |

### Boundary Tests (numeric inputs)
N-A: the check takes no numeric input; it compares states, not versions.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| failure on today's colima | recorded run of `./le test integration interop-ipsec` before AC-8 | developer gets one failure naming `XFRM_MSG_MIGRATE_STATE` in seconds, not 15 minutes of scenario failures | |
| MOBIKE on the Mac | recorded run after AC-8 | AC-12 | |
| MOBIKE on the nightly | `evidence-nightly` run | AC-13 | job wired at 8bd1742581 (on origin/main; the last nightly, run 38043229048, built 1d0e7a0228, which predates it): `ipsec-interop` runs `./le test qemu docker-lab lab "test integration interop-ipsec"` on the cached amd64 runtime kernel; no run yet |
| AC-10 workflow shape | `internal/le/workflowcheck/workflowcheck_test.go` `TestEvidenceNightlyScheduleActionsAndPrivileges` | the four Docker lab jobs restore, build on a miss and save the amd64 runtime kernel cache under qemu-nightly's key, run their lab through `docker-lab`, and run the host check as information; `radius-interop` stayed native at 8bd1742581 and runs through `docker-lab` too since 3ac4057f32 | red on HEAD's workflow (no docker-lab, no cache, no host check), green at 8bd1742581. Record correction (review r1 NOTE 11): 8bd1742581's body says the narrowing of this test's qemu ban (`test qemu/` refused, now `test qemu/docker-lab` admitted alone) has a row in `test/weakened/d1987d95.md`; the commit carried none, because `./le commit create` pruned the row at create time (the journal row committed in e663493f91). The justification stands here instead: D-6 makes `docker-lab` the route every Docker lab takes on Ze's kernel, and every other `test qemu/` action stays refused |
| AC-10 RFC tier survives the guest | `internal/le/rfc/tags_test.go` `TestALabRunInsideTheDockerLabGuestIsScheduled`, `TestNativeActionsInWorkflowCommands` | a lab run as `docker-lab lab "<le words>"` credits its interop tree to the scheduled workflow, read from the `le-words` keyword the registry declares (`leaction.ValueLeWords`) | red under a cut forcing no nested keyword, green at 8bd1742581; the stale shared `bin/le` refused `RFC8654-4-1` as unrun until `./le --update` rebuilt it |
| `ze doctor config <file>` grammar | `test/ui/doctor-refuses-bare-config-path.ci` | a bare path after `ze doctor` is refused, naming `ze doctor config <file>` | red observed 2026-10-10: the `.ci` copied into a `git archive` export of `c5ee8bf12f` (1ecb27591e^), run there with `./le test ui doctor-refuses-bare-config-path`, failed `exit_code_mismatch`: "cmd seq=1 (ze doctor --json empty.conf): expected exit code 1, got 0". Green at 1ecb27591e |

| AC-15, AC-16 from both entry points | `test/ui/doctor-kernelcap-denied.ci` (`option=needs-linux`) | with `l2tp-ppp` forced denied, `ze doctor --json config` carries `doctor-kernel-capability-denied` and exits 0, and `ze doctor --json kernel-capabilities` spells the row denied and exits 1 | 2026-10-10 in the QEMU guest on 7.2.9-ze, ze cross-built from a `git archive` of HEAD fc58bf8932 WITH `zetest`: PASS; the same build with the `denied` arm of `forcedState` removed: FAIL, "cmd seq=2 (ze doctor --json kernel-capabilities): expected exit code 1, got 0". The earlier "kernel mode ignores the force" red was a guest ze built WITHOUT `zetest`, which reads no force by design (`plan/journal/feature-test-missing-build-tag.md`, 2026-10-10) |

### Interop Tests (Scope: protocol)
N-A as a new scenario: no protocol behavior changes. The existing `mobike-initiator` and `mobike-responder` go from red (6.8) to green on a passing kernel; that pair is the discrimination record for AC-12.

## Files to Modify
- `internal/component/kernelcap/kernelcap.go` - evaluation of every enrolment with `InUse` ignored, for the doctor mode
- `internal/component/doctor/` - the all-capabilities mode and its JSON answer
- `internal/component/ike/dataplane/` - enrol `XFRM_MSG_MIGRATE_STATE`, ESP, the XFRM algorithm set, xfrm interface (sequenced with the IKE sibling session)
- the WireGuard, L2TP and PPPoE owning packages - their enrolments
- `internal/le/interoplab/lab.go`, `internal/le/interoplab/docker.go` - the check after `Preflight`
- `internal/le/test/deployment/`, `internal/le/site/terminaldemo/render.go` - call the same check
- `internal/le/setup/dockerkernel/dockerkernel.go` - `docker-kernel install` action (Linux hosts only, D-6) and a setup check that runs the same probe
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
| derived requirement | `TestDockerKernelRefusesNamingEveryMissingFeature`, `TestEnrolledKernelSymbolsAreInRuntimeRequire` |
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

## Host the Ze kernel replaces (2026-10-14)

The owner's Ubuntu host booted `7.2.9-ze` (installed with `./le setup docker-kernel install confirm 7.2.9-ze`) and lost two things its distro kernel gave it. The NIC came up as `enp0s18` instead of `ens18`: `/boot/config-7.2.9-ze` reads `# CONFIG_HOTPLUG_PCI_ACPI is not set` and `# CONFIG_HOTPLUG_PCI_PCIE is not set`, so no `/sys/bus/pci/slots` exists, udev has no slot name, and netplan's `/run/systemd/network/10-netplan-ens18.network` matched nothing. `ufw.service` failed with "Extension limit revision 0 not supported" and "Extension REJECT revision 0 not supported"; `/boot/config-6.8.0-147-generic` carries each AC-21 netfilter symbol as `=m`, and `7.2.9-ze` left them unset (`IP6_NF_MATCH_IPV6HEADER` and `NETFILTER_XT_TARGET_LOG` were already `=y` and `=m`). `/etc/ufw/*.rules` and `/proc/net/ip_tables_*` are root-only on this host, so the UFW list follows the ufw-init errors and Ubuntu's stock rule set.
-> Decision (owner, 2026-10-14): "fix the kernel config and add the check, then recompile the kernel and reinstall it."
Rebuild (2026-10-14, after commits 7bd12b1e80 and af8cc93d2f): the Docker builder refused (`/var/lib/docker` 31.4G free, 40G needed); `ze-host appliance kernel --builder qemu --target runtime --arch amd64 --profile runtime` built `~/.cache/ze/runtime-kernel/7.2.9-runtime-amd64-runtime-qemu-fc13b746-34cba457` (release `7.2.9-ze`, `runtime.require` enforced; log `scratch/kernel-rebuild/kernel-qemu.log`). The host check over this host (`ens18` from netplan's file name, `xt_LOG` loaded, UFW installed) refuses the old `docker-7c0d8f79` config naming both and passes the new one. `install` resolves the `runtime-docker-fc13b746` path only, so it cannot place the QEMU-built entry (`plan/journal/stale-artifact-reused.md`); owed: owner frees Docker space and reruns the Docker build, or the resolver gains the QEMU entry.
-> Constraint: `HOTPLUG_PCI_ACPI` depends on `ACPI`, which arm64's defconfig sets; the arm64 runtime build on the Mac has not been run with AC-21, and `runtime.require` refuses it if kconfig drops a symbol there.

## Known Limitations

- Ze's own build, its pin and its digest belong to `plan/pre-release/spec-appliance-ships-ze-kernel.md`; this spec uses whatever kernel passes.
- `ipsec-bgp-redistribute-frr` fails for a separate, journalled cause (`plan/journal/unwired-feature.md`, `plan/journal/test-against-broken-path.md`).
- nftables, policy routing, traffic shaping, eBPF and other interface kinds stay unenrolled unless a Docker lab exercises them; `plan/immediate/spec-kernel-capability-gate.md` keeps them.
- D-5 closed (option B): no separate kernel profile. A distro kernel still passes the derived check when it has every feature, but Ze's own hosts install the appliance kernel.

## Blocked (2026-10-10)

| Item | Blocked on | What unblocks it |
|------|-----------|------------------|
| AC-8, A-3, AC-12, AC-14: Docker lab in the guest under HVF | the Docker kernel check refuses `mpls-transit-mtu (CONFIG_MPLS_IP_MTU): unknown: the MPLS label space holds no unreserved label to address`. Measured 2026-10-10 on the Mac (HVF, arm64) with `./le test qemu docker-lab timeout 1800s`, log `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/docker-lab-nolab-4.log`: the guest booted cache entry `7.2.9-runtime-arm64-runtime-7c0d8f79-bf3ac1c9` ("Runtime kernel confirmed in the guest: 7.2.9", daemon kernel `7.2.9-ze`), dockerd started under OpenRC, and that row was the only one not present (ipsec-mobike, l2tp, l2tp-ppp and mpls present). Cause: `kernelcap.MPLSIPMTU` answers unknown while `net.mpls.platform_labels` is 16 or less, and the probe container's fresh network namespace starts at 0 under every kernel, so no kernel passes the check as built (R-1). Each lab's preflight runs the same check, so no lab runs either; the IPsec and BGP labs were not started | RESOLVED 2026-10-10: owner picked (b), committed 7d209a5d9e. `kernelcap.MPLSIPMTU` unshares a throwaway network namespace, makes `/proc/sys` writable in a private mount namespace, sizes the label space to 17 there and asks; the forwarding owner asks in its own namespace (`MPLSIPMTUInThisNamespace`). Guest proof on `7.2.9-ze` (`scratch/job-qemu-mplsprobe-9498e129.log`): `TestMPLSIntegration_ProbeAnswersFromAFreshNamespace` present, caller label space 0 before and after, red with the fix reverted (unknown, no unreserved label). Reruns on the Mac (HVF, arm64, same cache entry, host load 20 to 40), logs under the same scratch: `./le test qemu docker-lab timeout 1800s` `verdict pass` (`docker-lab-nolab-5.log`); `lab "test integration interop-ipsec" env "IPSEC_INTEROP_SCENARIO=mobike-initiator"` `PASS 1 scenario(s)` (`docker-lab-ipsec-mobike.log`); `lab "test integration interop" env "INTEROP_SCENARIO=bgp-ebgp-ipv4-frr"` `interop: 1 passed, 0 failed` (`docker-lab-bgp-ebgp-ipv4-frr.log`). Not run: `mobike-responder`, the full IPsec and BGP suites in the guest, `TestDockerLabGuestBootsZeKernel` via go test. Limit: a Docker host whose AppArmor `docker-default` profile denies `mount` (colima: `make the probe's mounts private: permission denied`) still reads this row unknown |
| Answered by the first guest boot (2026-10-10) | dockerd under OpenRC on the live Alpine ISO: starts. Go in the guest: `/usr/local/go` (run.go). The guest le as the probing `ze`: NO, a `ze_le` build started as `ze` answers "unknown command: doctor" (`docker-lab-nolab.log`); fixed, the host cross-builds `tmp/qemu/linux-<arch>/ze` with the daemon tags, and the guest check and the nightly's informational step probe with it. A suite's one-scenario selector could not reach the guest; fixed with `docker-lab env "NAME=value"`. Docker images on the 16G tmpfs root: not reached | the first lab run after AC-8 answers the last one |
| AC-12: MOBIKE on the Mac | not blocked on AC-8 any more (resolved above). `mobike-initiator` passed in the guest, and `mobike-responder` passed 2026-10-10 (`docker-lab-ipsec-mobike-responder-2.log`, round 5 owed table). The full IPsec and BGP suites in the guest are still owed ON THE MAC | `./le test qemu docker-lab lab "test integration interop-ipsec" timeout 5400s`, and the same with `lab "test integration interop"` |
| Owed on the Linux host, after the owner loads the profiles (`./le setup docker-kernel apparmor confirm ze-kernel-probe`, `... confirm ze-lab-net-sysctl`), installs the amd64 kernel (`./le setup docker-kernel install`, never run by an agent: HARD STOP) and reboots | AC-9 (the Docker kernel check passes on the host's own Docker); real-host proof of AC-15 (a `denied` row under docker-default, and `present` under `ze-kernel-probe`), AC-17, AC-18 and AC-19 (the AppArmor check, the refusal and the load action against a real AppArmor daemon; today unit tests and an `apparmor_parser -Q` parse only); the VRRP lab under `ze-lab-net-sysctl` | owner runs the three steps on the Linux host, then an agent runs `./le setup docker-kernel check` and the VRRP lab |
| AC-10, A-5, AC-13: the nightly | 8bd1742581 is on origin/main, but the last nightly (run 38043229048) built 1d0e7a0228, which predates it; no run has booted the guest under KVM on `ubuntu-latest`; a cold cache builds the amd64 kernel in each of the four jobs | the next scheduled or `workflow_dispatch` run of `evidence-nightly.yml` on a commit carrying 8bd1742581 |
| AC-14: a Docker daemon starts on the appliance kernel | not blocked (corrected in review round 4, NOTE 1): proven under HVF on the Mac. `docker-lab-nolab-5.log` (same scratch) shows "Runtime kernel confirmed in the guest: 7.2.9" (line 9), "Starting Docker Daemon ... [ ok ]" (line 80) and `verdict  pass` (line 81). The daemon kernel string `7.2.9-ze` is in the earlier `docker-lab-nolab-4.log` (line 80, the refusal that run printed), not in nolab-5 (corrected in review round 5, R5-N1) | nothing on the Mac; the amd64 boot is AC-10's nightly run |

## Review Gate

### Round 2: ready for review round 2 (2026-10-10)

Round 1's fixes start at `c4bd4708ed`. Commits since then, for an independent reviewer over the actual diff:

| Commit | Subject |
|--------|---------|
| `c4bd4708ed` | kernelcap: the forced answer is a zetest-only switch that always says so |
| `1f9ed323c3` | deployment: the kernel refusal test expects the route for its own OS |
| `3ac4057f32` | nightly: radius-interop runs in the Ze-kernel guest like every lab |
| `765430a9cf` | terminal-demo: validate mode checks the Docker kernel like a render |
| `1f56b6efd3` | perf: a run that tests the ze DUT checks the Docker kernel first |
| `c48964b4c2` | verify evidence: release-candidate checks the Docker kernel first |
| `0f85ddc0c8` | setup docker-kernel: install replaces Ze's own release, after confirm |
| `c060535622` | interoplab: the Docker kernel check honours the probe's exit and ready |
| `3f284a82b2` | deployment: daemon proofs check the kernel before pulling their image |
| `569ba6785e` | ike: state why EINVAL from the MOBIKE probe is absence |
| `ecfa32e1fd` | ike: enrol MOBIKE migration, ESP and every XFRM transform in kernelcap |
| `5e3bc193b0` | spec: correct 8bd1742581's claim of a weakening row |
| `f9f024eb42` | qemu: docker-lab probes with a real linux ze, and takes env |
| `83fdff68b1` | plan: first HVF Docker lab guest boot, check refuses mpls-transit-mtu |
| `7d209a5d9e` | kernelcap: mpls-transit-mtu asks from a throwaway namespace |
| `0d9bbe2890` | plan: Docker kernel check passes in the guest, mobike-initiator green |
| `dcc4216fd7` | kernelcap: a probe the host's security policy refuses reads denied |
| `1e1d012115` | le: Docker kernel check probes under Ze's AppArmor profile |
| `99ceb35b77` | interoplab: an unapplied AppArmor profile refuses naming its fix |
| `65d32b7906` | setup: docker-kernel check probes with a linux ze for the daemon |
| `a1636c2223` | interoplab: the VRRP lab's ze runs under ze-lab-vrrp, never unconfined |
| `ba7af683c6` | kernelcap, doctor: a forced denied reaches both doctor modes |

Gates at hand-off: `./le cli grammar` fails on five R1 rows (`explain`, `generate wireguard keypair`, `skills`, `support`, `validate config`), none a verb this spec added (`plan/journal/gate-fires-outside-its-population.md`, 2026-10-10). Scoped lint (kernelcap, doctor, interoplab, interoplab/bgp, setup) produced no verdict twice: "lint inputs changed during planning", other sessions editing the tree. Owed: that scoped lint when edits stop, `./le go lint run`, `./le test unit all`, and the owner's scope answer on the six `--privileged`/`seccomp=unconfined` rows (D-7 verdict table). Corrected in round 3: the owner answered "implement" and `spec-lab-containers-least-privilege` owns those rows (see the D-7 table); this spec owes no answer on them.

Round 2 verdict (independent reviewer, 2026-10-10): 1 BLOCKER, 2 ISSUE, 8 NOTE over `c4bd4708ed..ba7af683c6`.

| # | Severity | Finding | Resolution |
|---|----------|---------|------------|
| 1 | BLOCKER | `ze-lab-vrrp` denied `net/ipv4/icmp_errors_use_inbound_ifaddr`, which VRRP writes | `84d07bf11c`: VRRP lab ze runs under the shared `ze-lab-net-sysctl`; `ze-lab-vrrp` deleted; `TestVRRPWritesOnlyLabGrantedSysctls` derives the grant from VRRP's own writes |
| 2 | ISSUE | fake dockers answered the security-options query empty, so four D-4 entry-point tests were red | `4147096555` |
| 3 | ISSUE | perf probed an unstaged `test/interop/ze-linux` | `6d423d8c60`: `Bench.Run` stages first, `checkDockerKernel` refuses a missing ze path |
| 4 | NOTE | an SELinux denial named the AppArmor fix | `3aad367c33` |
| 5 | NOTE | profile list read on the local host with a remote daemon | journal `gate-verdict-depends-on-the-machine.md` |
| 6 | NOTE | only runc's unapplied-profile phrase recognised | journal `failing-gate-prints-no-cause.md` |
| 7 | NOTE | force-variable description omitted `denied` | round 3 commit: `forcedSpellings` is the one declaration, description built from it, `TestForceGrammarNamesEveryForcibleState` |
| 8 | NOTE | `ze-kernel-probe` lacks docker-default's daemon-profile signal rule | journal `helper-bypassed-by-an-open-coded-copy.md` |
| 9 | NOTE | fix command mid-sentence in three refusals | round 3 commit: command on a line of its own in `appArmorRefusal`, the denied-row text, `labAppArmorProfileMissing`, `installConfirmed`; tests assert the line |
| 10 | NOTE | proof.json under the package dir | already journalled, `test-artifacts-land-in-the-repository-root.md` |
| 11 | NOTE | non-zetest `ZE_BIN` accepted | already journalled, `feature-test-missing-build-tag.md` |
| perf | NOTE | a build-only perf run builds the ze image with nothing staged | journal `guard-added-to-one-half-of-a-pair.md` |

### Round 3: ready for review round 3 (2026-10-10)

Scope for the reviewer: round 2's fixes only, `84d07bf11c`, `4147096555`, `6d423d8c60`, `3aad367c33`, `50eed92a29` (note 7) and `966c9cb302` (note 9 and the lint findings in the same files).

Gates at hand-off: scoped lint over kernelcap, interoplab, setup, perf, plugins/vrrp: clean for this spec's files; one finding left, `internal/le/setup/system.go` goconst `lo0`, from `0b3c569cac` (another session). The wider scope (interoplab/bgp, pppoe, rsvpte, test/deployment, setup tools/claude_server) carries findings in files this spec did not write. Package tests green with gate tags and -race: kernelcap (with and without `zetest`), interoplab, interoplab/bgp, setup. Owed: `./le go lint run`, `./le test unit all`, `./le cli grammar`, and every AC that needs the Linux host after the owner loads the profiles and boots the amd64 kernel.

Round 3 verdict (independent reviewer, 2026-10-10): 0 BLOCKER, 2 ISSUE, 7 NOTE over round 2's fixes (`84d07bf11c..966c9cb302`, `373768ca38`).

| # | Severity | Finding | Resolution |
|---|----------|---------|------------|
| 1 | ISSUE | a denied row on a host without AppArmor still named the AppArmor load command, and `TestDockerKernelDeniedRowNamesTheProfileCommand` pinned it | `c563df412a`: `deniedFix` prints the load command only when the daemon applies AppArmor, otherwise names the policy grant, plus `sudo ausearch -m avc -ts recent` for SELinux; `TestDockerKernelDeniedRowWithoutAppArmorNamesThePolicyGrant` |
| 2 | ISSUE | the kernel-route command sat mid-sentence, and on macOS was the template `<le words>` | `c563df412a`: `DockerKernelRoute(goos, lab)` prints the command on its own line, built on darwin from the run's own le words; `TestDockerKernelRouteNamesEachPlatformsNextStep`, `TestInvokedLeWordsReadTheCommandLine`, `TestDockerKernelRefusalNamesTheExactRouteCommand` |
| 3 | NOTE | force spellings were a second copy of `State.String()` | `d78a6aed92`: `forcedStates [...]State`, spellings from `String()`; the doctor page reads `<subsystem>=<state>` (its hunk landed in `c28bb84e96`) |
| 4 | NOTE | the missing-staged-ze fix was inline and did not fit perf | `c563df412a`: `stageZeAgain` prints `env -u NO_BUILD ./le <words>` or `./le <words>` on its own line; `TestStagedZeMissingNamesTheCommandThatStagesIt` |
| 5 | NOTE | a lab container's unapplied profile surfaced runc's raw error | `c563df412a`: `runContainer` maps it through `labAppArmorUnapplied` to the load command; `TestRunContainerMapsAnUnappliedProfileToItsLoadCommand` |
| 6 | NOTE | `classifyLabel` blamed EACCES under `unconfined_t` on SELinux | `c28bb84e96`: unconfined SELinux domains answer the zero confinement; `TestClassifyLabelUnconfinedSELinuxDomain`, `TestMPLSIPMTUUnconfinedSELinuxEACCESIsUnknown` (linux/arm64 in alpine, red mutated) |
| 7 | NOTE | the deployment mirror linked the checkout's `tmp/` | `c563df412a`: `mirrorPrivateDirs`; `TestMirrorKeepsTmpPrivate` |
| 8 | NOTE | D-7 record stale on ownership of the `--privileged` rows | this spec-record commit: D-7 table and round-2 hand-off name `spec-lab-containers-least-privilege` as owner; the `ze-lab-net-sysctl` parse is owed there (AC-2) |
| 9 | NOTE | "not pushed", "blocked on AC-8" and two nonexistent test names | this spec-record commit: A-5, the AC-13 row and the Blocked table corrected; TDD rows name `TestDockerKernelUnknownRefuses` and `TestDockerKernelRefusesNamingEveryMissingFeature`; the Blocked table lists every AC owed on the Linux host, on the nightly and on the Mac |
| add-on | NOTE | six refusals printed `./ze appliance kernel ...`, which the `./ze` wrapper cannot run | `c563df412a`: `buildhostdriver.RuntimeKernelBuild`; `TestRuntimeKernelBuildResolvesToRegisteredCommands`; journal row |

### Round 4: ready for review round 4 (2026-10-10)

Scope for the reviewer: round 3's fixes, `c563df412a`, `d78a6aed92`, `c28bb84e96` and the spec-record commit after them.

Owed elsewhere, not reviewable here: on the Linux host after the owner's profile load, amd64 kernel install and reboot, AC-9 and the real-host proof of AC-15, AC-17, AC-18, AC-19 and the VRRP lab; on the GitHub nightly, AC-10 and AC-13; on the Mac, AC-12 `mobike-responder` and the full suites in the guest. Gates owed: `./le go lint run`, `./le test unit all`, `./le cli grammar`.

Round 4 verdict (independent reviewer, 2026-10-10): 0 BLOCKER, 3 ISSUE, 4 NOTE.

| # | Severity | Finding | Resolution |
|---|----------|---------|------------|
| 1 | ISSUE | scoped lint: godot on the `unconfinedSELinuxTypes` comment in `policy_linux.go`; goconst `"linux"` x3 in `testqemu` (`dockerlab.go`, `guestle.go`, `mobike.go`) | `acdd48c266` (godot), `49697ff613` (`guestOS`, one constant in `hugepages.go`). Scoped lint over kernelcap, interoplab, interoplab/l2tp, setup, test/deployment, test/qemu, build: one finding left, goconst `lo0` in `internal/le/setup/system.go` (another session's) |
| 2 | ISSUE | `spc_t` read as a confining SELinux label, though container-selinux's `container.te` declares `unconfined_domain_noaudit(spc_t)` | `acdd48c266`: `spc_t` joins `unconfinedSELinuxTypes`, quote and source (container-selinux commit `464dd1655e`) above it; the row moves to `TestClassifyLabelUnconfinedSELinuxDomain`. Red before the fix in a linux/arm64 test binary under alpine: `classifyLabel("system_u:system_r:spc_t:s0") = SELinux context ..., want no confining policy` (`scratch/kernelcap-r4-red.txt`), green after (`kernelcap-r4-green.txt`); doctor page updated |
| 3 | ISSUE | on darwin the route rebuilt the command from `os.Args` alone, so a run a selector variable chose reran the whole suite | `49697ff613`: every suite declares its selector to `ReadEnvironment` (L2TP now too), and `DockerKernelRoute(goos, lab, environment)` appends `env 'NAME=value'`, or falls back to the check-only command when an assignment cannot be carried; `TestDockerKernelRouteNamesEachPlatformsNextStep`, `TestSelectorEnvironmentCarriesTheDeclaredSelectors`, `TestDockerKernelRefusalNamesTheExactRouteCommand` |
| 4 | NOTE 1 | AC-14 Blocked row still "blocked on AC-8 or AC-10" | this spec-record commit: proven under HVF by `docker-lab-nolab-5.log` |
| 5 | NOTE 2 | a refusal inside the QEMU guest prescribed `./le setup docker-kernel install` and a reboot | `49697ff613`: the guest exports `ZE_DOCKER_LAB_GUEST=1` before its check, and the route there names `gokrazy/kernel/runtime.config`, `runtime.require` and the host's cache rebuild; `TestDockerKernelRouteInsideTheDockerLabGuest`, `TestDockerLabCommandChecksTheKernelBeforeTheLab` |
| 6 | NOTE 3 | darwin route rerunning a host path: if the refused run was `./le setup docker-kernel check ze <host path>`, the darwin route reruns it in the guest with a host path that does not exist there. Edge case | journalled 2026-10-10 in `plan/journal/failure-report-names-a-command-that-does-not-exist.md` |
| 7 | NOTE 4 | untracked `proof.json` under `internal/le/interoplab/bgp/tmp/session/` | already journalled 2026-10-09 in `test-artifacts-land-in-the-repository-root.md`; that row now records the 2026-10-10 recurrence (18 files). The files are left in place |

### Round 5: ready for review round 5 (2026-10-10)

Scope for the reviewer: round 4's fixes, `acdd48c266`, `49697ff613` and the spec-record commit after them.

Owed elsewhere, not reviewable here: as round 4 (NOTE 3 is now journalled). AC-12 `mobike-responder` attempt, 2026-10-10: failed before boot, not on the kernel. A concurrent docker-lab run held host port 2222, so QEMU refused "Could not set up host forwarding rule 'tcp::2222-:22'" (the `freeRunSSHPort` probe-then-release race, journalled in `plan/journal/parallel-copies-collide-on-a-deterministic-port.md`; log `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/docker-lab-ipsec-mobike-responder.log`). Gates owed: `./le go lint run`, `./le test unit all`, `./le cli grammar`.

Round 5 verdict (independent reviewer, 2026-10-10): 0 BLOCKER, 0 ISSUE, 4 NOTE.

| # | Severity | Finding | Resolution |
|---|----------|---------|------------|
| 1 | R5-N1 | the AC-14 Blocked row cited `7.2.9-ze` from `docker-lab-nolab-5.log`; that string is in nolab-4 | corrected in the AC-14 row: nolab-5 lines 9, 80 and 81 quoted, `7.2.9-ze` attributed to nolab-4 line 80 |
| 2 | R5-N2 | round-4 NOTE 3 not yet journalled | journalled in `plan/journal/failure-report-names-a-command-that-does-not-exist.md` (round-4 table row 6) |
| 3 | R5-N3 | `quotedGuestWords` refused only space, tab and newline, while the guest splits its `lab` and `env` values with `strings.Fields` (`\r`, `\v`, `\f`, Unicode spaces) | fixed: a word holding any `unicode.IsSpace` rune falls back to the check-only command. Five new rows in `TestDockerKernelRouteNamesEachPlatformsNextStep` (`\r`, `\v`, `\f`, U+00A0, U+2003 in an assignment), all red before the fix, `interoplab` green with `-race` after |
| 4 | R5-N4 | inside the guest the route blames the kernel config for an `unknown` row the probe itself caused | journalled in `plan/journal/error-answered-with-another-errors-code.md` |

The review loop is closed: round 5 has 0 BLOCKER and 0 ISSUE. The spec stays in-progress because these remain owed:

| Where | What |
|-------|------|
| Gates | `./le go lint run`, `./le test unit all`, `./le cli grammar` |
| Linux host, after the owner loads the profiles (`./le setup docker-kernel apparmor confirm ze-kernel-probe`, `... confirm ze-lab-net-sysctl`), installs the amd64 kernel and reboots | AC-9, and the real-host proof of AC-15, AC-17, AC-18, AC-19 and the VRRP lab |
| GitHub nightly (after a push the owner orders) | AC-10, AC-13 |
| Mac | the full IPsec and BGP suites in the guest. AC-12 `mobike-responder` itself PASSED 2026-10-10 19:49 (HVF, arm64, guest kernel 7.2.9): `./le test qemu docker-lab lab "test integration interop-ipsec" env "IPSEC_INTEROP_SCENARIO=mobike-responder" timeout 3600s` printed `verdict  pass` for the kernel check (line 81), `── mobike-responder ── ✓ PASS`, `PASS  1 scenario(s)` and `QEMU VM: PASS` (lines 87 to 94), log `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/docker-lab-ipsec-mobike-responder-2.log`. With `mobike-initiator` (`docker-lab-ipsec-mobike.log`), both AC-12 scenarios have passed on the Mac |

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
