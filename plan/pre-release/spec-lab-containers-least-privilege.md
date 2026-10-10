# Spec: lab-containers-least-privilege

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md` (D-7, the AppArmor profile registry at a1636c2223) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every Docker container a Ze lab or proof starts that leaves Docker's default confinement (`--privileged`, `seccomp=unconfined`, `apparmor=unconfined`) gets the treatment the VRRP lab got at a1636c2223: it is granted exactly what its daemon needs (capabilities, devices, namespaced sysctls, a Ze AppArmor profile registered in `interoplab`), and broad privilege stays only where a justified verdict says the need is unbounded.

Owner instruction, 2026-10-10 (recorded as the approval, design gates waived by the owner's "implement"): the remaining containers in the D-7 verdict table of `plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md` get the same least-privilege treatment as VRRP, as their own spec, and the spec is implemented.

Owner principle D-7, restated at his request on 2026-10-10 in corrected English and made generic rather than specific to AppArmor: "The host should be set up correctly for Ze. A user will meet a misconfigured host sooner or later; when they do, Ze should report it without friction, and the path from the symptom to understanding the problem and its fix should be as smooth as possible. Where tooling can make sure the admin does the right thing, we should have the tooling." The owner's clarification, same day: the point is user friendliness, on every user interface Ze has (CLI, web, `ze doctor`, logs, the `./le` tooling), not the CLI alone, so every refusal this spec writes names what is wrong, why, and the exact command that fixes it.

-> Decision (owner, 2026-10-10, "make it generic"): one generic lab profile, not one per lab. A peer that writes its own network namespace's sysctls at run time runs under `ze-lab-net-sysctl` (docker-default with `/proc/sys` writes denied everywhere but under `net/`), registered once in `interoplab`; a lab names it, never defines its own copy.
-> Decision (owner, confirmed 2026-10-10): "make it generic" means ONE shared AppArmor profile, `ze-lab-net-sysctl`, for every lab container that writes network sysctls, granting only what the labs together need; no per-lab profiles. The IPsec strongSwan peer and the RSVP-TE Ze nodes use it; VRRP moves onto it and `ze-lab-vrrp` is deleted (docker-host spec's work).

Goals.

| ID | Goal |
|----|------|
| G-1 | No lab container runs `--privileged` unless this spec records a verdict that its need is unbounded |
| G-2 | Every grant a container keeps is proven necessary: removing it makes the lab fail |
| G-3 | A container that needs more than docker-default's AppArmor rules runs under a registered Ze profile, loaded by `./le setup docker-kernel apparmor confirm <name>`, never unconfined |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - the Docker host kernel check and the AppArmor profile registry
  → Decision: a lab's profile registers with `interoplab.RegisterAppArmorProfile` from a `register_apparmor.go` in the lab's own package; a peer names it in `PeerConfig.AppArmorProfile`; `runContainer` adds `--security-opt apparmor=<name>` only on a daemon that reports AppArmor and refuses on a Linux client whose `/sys/kernel/security/apparmor/profiles` lacks it, naming `./le setup docker-kernel apparmor confirm <name>`. One registry, no list.
  → Constraint: a profile is built with `kernelcap.DockerDefaultProfile(name, procSysDenies, mounts)` and `kernelcap.ProcSysWriteDenies(keep)`; `keep` is ONE path, a trailing slash grants the directory, any other keep is one file and everything longer is denied. Two disjoint keeps cannot be expressed, because each deny chain refuses the other.
- [ ] `docs/architecture/core-design.md` - declared by the deployment and terminal-demo files
  → Constraint: unaffected; this spec changes container argv only, no package boundary or registration
- [ ] `ai/rules/platform-linux.md` - labs run on Ze's kernel
  → Constraint: a lab runs through `./le test qemu docker-lab` (the Alpine guest on Ze's runtime kernel); that guest carries no AppArmor, so a profile is exercised only on an AppArmor Linux host, and its parse is proven with `apparmor_parser -Q` in `ubuntu:24.04`.
- [ ] `gokrazy/kernel/runtime.config` - what Ze's kernel builds in
  → Constraint: `CONFIG_PPP`, `CONFIG_PPPOE`, `CONFIG_L2TP`, `CONFIG_PPPOL2TP`, `CONFIG_MPLS_ROUTING`, `CONFIG_MPLS_IPTUNNEL`, `CONFIG_XFRM_USER`, `CONFIG_IP_NF_NAT`, `CONFIG_TUN`, `CONFIG_HUGETLBFS` are all `=y`, so no container on the lab's kernel needs `modprobe` or `CAP_SYS_MODULE`. A host with these as modules loads them itself (D-7: the host is set up for Ze); a container never loads a host kernel module.
- [ ] Docker's defaults (moby `oci/defaults.go`, `profiles/seccomp/default.json`)
  → Constraint: the default capability set holds `NET_RAW`, `SETUID`, `SETGID`, `NET_BIND_SERVICE`, `DAC_OVERRIDE`, `CHOWN`, `KILL`, `MKNOD` but not `NET_ADMIN`, `SYS_ADMIN`, `IPC_LOCK`, `SYS_MODULE`. Docker mounts `/proc/sys` read-only (lifted only by `--security-opt systempaths=unconfined`), shows no host device but the defaults (`--device` adds one), and its default seccomp profile admits `mount`, `unshare`, `setns` only when `CAP_SYS_ADMIN` is granted. `--sysctl net.*=v` is written by the runtime into the container's own network namespace before the process starts.

**Key insights:**
- A sysctl written once at start becomes `--sysctl`; a sysctl written at run time needs `systempaths=unconfined` and a Ze profile narrowing `/proc/sys` writes.
- PPP needs `--device /dev/ppp` and `NET_ADMIN` (`PPPIOCNEWUNIT` checks `CAP_NET_ADMIN` in the network namespace); XFRM, L2TP genl, MPLS routes, addresses, routes and iptables need `NET_ADMIN`; raw sockets (nping, tcpdump, PPPoE discovery, RSVP protocol 46) need `NET_RAW`, which Docker grants by default.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/interoplab/apparmor.go` - the profile registry and `appArmorSecurityOption`
- [ ] `internal/component/kernelcap/apparmor.go` - `DockerDefaultProfile`, `ProcSysWriteDenies`
- [ ] `internal/le/interoplab/docker.go` - `PeerConfig`, `runContainer` argv
- [ ] `internal/le/interoplab/bgp/register_apparmor.go` - the VRRP precedent
- [ ] `internal/le/interoplab/ipsec/ipsec.go` - IPsec peers
- [ ] `internal/le/interoplab/ipsec/nat.go` - the NAT script
- [ ] `internal/le/interoplab/ipsec/helpers.go` - the run-time sysctl writes on strongSwan
- [ ] `internal/le/interoplab/l2tp/l2tp.go` - L2TP peers and preflight
- [ ] `internal/le/interoplab/pppoe/scenarios.go` - PPPoE peers
- [ ] `internal/le/interoplab/rsvpte/rsvpte.go` - RSVP-TE ze and preflight
- [ ] `internal/le/test/deployment/vppiface.go` - the VPP deployment container
- [ ] `internal/le/verify/evidence/evidence.go` - the evidence container
- [ ] `internal/le/site/terminaldemo/render.go` - the demo renderer
- [ ] `internal/component/l2tp/kernel_linux.go` - `probeKernelModules`

**Behavior to preserve:** every scenario's verdict; each lab's preflight still refuses a host lacking the kernel feature.

**Behavior to change:** the containers' privileges, per the table below.

`git grep` of `--privileged`, `seccomp=unconfined`, `apparmor=unconfined` in non-test Go, 2026-10-10:

| # | Site | Container | Runs | What it does that docker-default refuses |
|---|------|-----------|------|------------------------------------------|
| 1 | `internal/le/interoplab/ipsec/ipsec.go` `dockerPrivileged` | NAT box (`nat.go` `natSetupScript`) | `sysctl -w` ip_forward, send_redirects, accept_redirects at start; `ip addr add`; iptables nat; later `ip route`, `ip rule`, iptables OUTPUT via `docker exec` (`helpers.go`) | the three sysctl writes (read-only `/proc/sys`), every netlink and iptables write (`NET_ADMIN`) |
| 1 | same | strongSwan charon (`test/interop-ipsec/Dockerfile.strongswan`) | XFRM SAs and policies, routes; `nping` raw sockets; the lab's `dropFragmentsAtPeer` (`helpers.go`) writes `net.ipv4.ipfrag_low_thresh` and `ipfrag_high_thresh` at run time | XFRM and route writes (`NET_ADMIN`); the two run-time sysctl writes (read-only `/proc/sys`, docker-default's `deny @{PROC}/sys/[^k]** w`) |
| 1 | same | ze IKE (`Dockerfile.ze`) | XFRM SAs, policies, `XFRM_MSG_MIGRATE_STATE`, MOBIKE address changes via `docker exec ip addr` | XFRM and address writes (`NET_ADMIN`) |
| 2 | `internal/le/interoplab/l2tp/l2tp.go` `privilegedArgument` | ze LNS, xl2tpd LAC, preflight alpine | L2TP genl tunnels and sessions, PPPoL2TP sockets, `/dev/ppp` units; preflight `modprobe` with `/lib/modules` mounted | `NET_ADMIN`, the `/dev/ppp` device; `modprobe` (`SYS_MODULE`) |
| 3 | `internal/le/interoplab/pppoe/pppoe.go`, `scenarios.go` | ze, accel-ppp, pppd client, preflight | PPPoE discovery (AF_PACKET), PPPoX sockets, `/dev/ppp`; images' entrypoints `modprobe` | `NET_ADMIN`, `/dev/ppp`; `modprobe` |
| 4 | `internal/le/interoplab/rsvpte/rsvpte.go` | ze RSVP-TE node, MPLS preflight | `<role>-setup.sh` writes `net.mpls.platform_labels`, `net.mpls.conf.<dev>.input` (including `prot0`, created by the script at run time) and `net.ipv4.ip_forward`; MPLS routes; raw protocol-46 socket; tcpdump | `NET_ADMIN`; the run-time sysctl writes; preflight `modprobe mpls_router` |
| 5 | `internal/le/test/deployment/l2tp.go` `containerArgs` | xl2tpd and ze in one container | `/dev/ppp`, L2TP, PPP | `NET_ADMIN`, `/dev/ppp` |
| 5 | `internal/le/test/deployment/vppiface.go` `containerArgs` | VPP (dpdk disabled, `VPPStartupConfig`) and ze | tap interfaces (`/dev/net/tun`), linux-cp, buffer memory, `mlock` | `NET_ADMIN`, `/dev/net/tun`, `IPC_LOCK`; hugepages: see A-4 |
| 6 | `internal/le/verify/evidence/evidence.go` `dockerArgs` | the clean-clone verification gate (`ContainerScript`: `./le verify current mode full`) | every test the gate runs, including those `caps=` admits | open-ended: see Verdicts |
| 7 | `internal/le/site/terminaldemo/render.go` `containerCommand` | the website terminal-demo renderer | `NET_ADMIN`, `NET_RAW`, `SYS_ADMIN` always, `seccomp=unconfined` always, `--privileged` for a demo whose manifest says `privileged` | see Verdicts |

`internal/le/interoplab/ipsec/ipsec.go` FRR peer already runs with `NET_ADMIN`, `SYS_ADMIN` and no privilege: out of scope.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`./le test integration interop-ipsec` (and `interop-l2tp` via `interop`, `interop-pppoe`, `interop-rsvpte`), run on Ze's kernel by `./le test qemu docker-lab lab "<le words>"`; `./le test deployment l2tp-ppp-test`, `vpp-iface-test`; `./le verify evidence ...`; the terminal-demo render.

### Transformation Path
1. A lab's `prepareScenario` builds `interoplab.PeerConfig` per container: `Capabilities` (rendered `--cap-add`), `Arguments` (verbatim argv: `--device`, `--sysctl`, `--security-opt systempaths=unconfined`), `AppArmorProfile`.
2. `Docker.runContainer` (`internal/le/interoplab/docker.go`) renders the argv; `appArmorSecurityOption` adds the profile on an AppArmor daemon or refuses naming the load command.
3. A deployment proof builds its own `docker run` argv in `containerArgs`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le ↔ Docker daemon | `docker run` argv | unit tests over the recorded argv |
| container ↔ host kernel | capabilities, devices, sysctls, AppArmor | lab runs on Ze's kernel through `docker-lab` |

### Integration Points
- `interoplab.RegisterAppArmorProfile` and `./le setup docker-kernel apparmor confirm <name>`: a new profile is loadable with no edit to the setup action.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | grants go through `PeerConfig` and the existing argv builders |
| No unintended coupling (components stay isolated) | Yes | each lab owns its grants and its profile |
| No duplicated functionality (extends existing, does not recreate) | Yes | profiles use `kernelcap.DockerDefaultProfile` and `ProcSysWriteDenies` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling argv |
| Registration over hardcoding, outbound | Yes | a profile registers from the lab's `register_apparmor.go` |
| Registration over hardcoding, inbound | Yes | `dockerkernel_apparmor.go` iterates `AppArmorProfileNames()`; no list names the new profiles |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ze in the IPsec, L2TP and PPPoE labs writes no sysctl | `git grep /proc/sys` in `internal/component/l2tp`, the IKE dataplane: none; the scenarios' `ze.conf` carry no `sysctl` block | ze logs a write failure or exits; the lab says which | lab run through `docker-lab` | unvalidated |
| A-2 | ze's L2TP kernel probe (`probeKernelModules`, `internal/component/l2tp/kernel_linux.go`) passes on Ze's kernel without `modprobe`: it returns early only when `/sys/module/l2tp_ppp` exists, and a built-in module with no parameter may not appear there | `moduleBuiltIn` reads `/sys/module/<name>` only | ze refuses to start in the L2TP and PPPoE labs: the probe must also accept `/proc/net/pppol2tp` (the preflight's own test), fixed in ze, never by granting `SYS_MODULE` | lab run | resolved at the producer, lab run owed: `net/l2tp/l2tp_ppp.c` declares `MODULE_VERSION(PPPOL2TP_DRV_VERSION)`; built in with `CONFIG_SYSFS`, `MODULE_VERSION` places a `module_version_attribute` in `__modver` (`include/linux/module.h`), and `version_sysfs_builtin` (`kernel/params.c`) creates `/sys/module/l2tp_ppp` through `lookup_or_create_module_kobject`, so ze's probe passes on Ze's kernel with no fix |
| A-3 | accel-ppp and pppd need no run-time sysctl write that fails fatally | accel-ppp warns on a failed write | a scenario fails naming the write; a profile is added as for strongSwan | lab run | unvalidated |
| A-4 | VPP without dpdk runs its buffers and heap on 4K pages when hugepages cannot be reserved | VPP's physmem falls back to the default page size | VPP refuses to start; the container gets `/dev/hugepages` mounted and the host reserves pages | `./le test deployment vpp-iface-test` | unvalidated |
| A-5 | `--sysctl net.ipv4.conf.all.send_redirects=0` and the two others are accepted by Docker for a container on a user bridge | Docker admits `net.*` for a container with its own network namespace | `docker run` refuses with the key named | lab run | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a grant is missing and a scenario fails on a host fact, read as a Ze defect | the failure names EPERM, EACCES, `Operation not permitted`, `Read-only file system` | the lab's log carries the daemon's error; add the one grant with its reason |
| R-2 | the guest carries no AppArmor, so a profile's grant is never exercised there | none in the guest | the profile's text is unit tested against the paths the lab writes, and it parses under `apparmor_parser -Q`; an AppArmor host run is named as owed |
| R-3 | `systempaths=unconfined` without AppArmor (the guest) lets container root write host-global `kernel.*` sysctls | none | used only where a run-time write is required (strongSwan, rsvpte ze), as VRRP does; on an AppArmor host the Ze profile confines it |
| R-4 | a host with L2TP/PPPoE/MPLS as modules not loaded fails the preflight where it used to load them | preflight refusal | the refusal names the modules and the host command; the module is the host's setup (D-7) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a lab or proof fails on the lab host; nothing user-visible ships |
| How is it reverted? | one commit per container |
| Who else touches this path? | the docker-host session (kernelcap, doctor, `interoplab` kernel check) edits `interoplab` too: this spec adds files and touches only each lab's own package |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration interop-ipsec` | → | `prepareScenario` peers | `TestIPsecLabPeersRunWithoutPrivilege` (`internal/le/interoplab/ipsec/privilege_test.go`) |
| `./le test integration interop` (L2TP scenarios) | → | `zePeer`, `lacPeer`, `preflight` | `TestL2TPLabPeersRunWithoutPrivilege` (`internal/le/interoplab/l2tp/privilege_test.go`) |
| `./le test integration` PPPoE lab | → | `prepare*` peers, `preflight` | `TestPPPoELabPeersRunWithoutPrivilege` (`internal/le/interoplab/pppoe/privilege_test.go`) |
| `./le test integration interop-rsvpte` | → | `zePeer`, `mplsPreflight` | `TestRSVPTELabZeRunsWithoutPrivilege` (`internal/le/interoplab/rsvpte/privilege_test.go`) |
| `./le test deployment l2tp-ppp-test` | → | `L2TP.containerArgs` | `TestL2TPDeploymentContainerRunsWithoutPrivilege` (`internal/le/test/deployment/privilege_test.go`) |
| `./le test deployment vpp-iface-test` | → | `vppIface.containerArgs` | `TestVPPDeploymentContainerRunsWithoutPrivilege` (same file) |
| `./le setup docker-kernel apparmor confirm <name>` | → | `appArmorLoadConfirmed` over the registry | existing `internal/le/setup/dockerkernel_apparmor_test.go`, which iterates every registered profile |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IPsec lab peers | no argv carries `--privileged`; nat, strongSwan and ze carry `--cap-add NET_ADMIN`; the NAT box carries `--sysctl` for `net.ipv4.ip_forward=1`, `net.ipv4.conf.all.send_redirects=0`, `net.ipv4.conf.all.accept_redirects=0` and its script writes no sysctl; strongSwan carries `systempaths=unconfined` and `AppArmorProfile` `ze-lab-net-sysctl`, and both `peerReassemblyMarks` lie under the profile's grant (derived in the test) |
| AC-2 | `ze-lab-net-sysctl` | registered once in `interoplab`; docker-default with `/proc/sys` writes denied except under `net/`, `deny mount,` kept, no `mount options` or `remount`; parses under `apparmor_parser -Q` in `ubuntu:24.04` |
| AC-3 | L2TP lab peers and preflight | no `--privileged`, no `/lib/modules` mount, no `modprobe`; ze and LAC carry `NET_ADMIN` and `--device /dev/ppp`; the preflight carries the same and refuses naming the missing module or device |
| AC-4 | PPPoE lab peers and preflight, and the images' entrypoints | as AC-3 for ze, accel-ppp, the pppd client and the preflight; no entrypoint runs `modprobe` |
| AC-5 | RSVP-TE ze and the MPLS preflight | no `--privileged`, no `modprobe`; ze carries `NET_ADMIN`, `systempaths=unconfined` and `AppArmorProfile` `ze-lab-net-sysctl` (every key the scenarios' setup scripts write lies under its grant, derived in the test from the scripts); the preflight runs unprivileged and reads `/proc/sys/net/mpls/platform_labels` |
| AC-6 | deployment L2TP container | no `--privileged`; `NET_ADMIN`, `--device /dev/ppp` |
| AC-7 | deployment VPP container | no `--privileged`; the grants A-4's run settles (`NET_ADMIN`, `IPC_LOCK`, `--device /dev/net/tun`, plus hugepages only if the run shows VPP needs them) |
| AC-8 | each lab and proof above, run through `./le test qemu docker-lab` on Ze's kernel (or its deployment action) | passes as it did with `--privileged` |
| AC-9 | each grant (each capability, device, sysctl, systempaths) removed in turn | the lab fails, naming the refused operation; the red is recorded in this spec |
| AC-10 | evidence and terminal-demo | the verdict in Verdicts is implemented or recorded with its justification; the D-7 table row is updated |

## Verdicts

| Site | Verdict |
|------|---------|
| 1 IPsec | REPLACE: AC-1, AC-2 |
| 2 L2TP | REPLACE: AC-3 |
| 3 PPPoE | REPLACE: AC-4 |
| 4 RSVP-TE | REPLACE: AC-5. `--sysctl` cannot name `net.mpls.conf.prot0.input` because `prot0` is created by the setup script, so the run-time write stays and the profile confines it |
| 5 deployment | REPLACE: AC-6, AC-7 |
| 6 evidence | the container runs `./le verify current mode full`, whose population is every test in the tree, and `caps=` admits a capability-gated test where the host holds the capability. The need is therefore the union of the `caps=` vocabulary, which a registry already declares. Design: derive the `--cap-add` list from that vocabulary, never a hand list; until implemented `--privileged` stays, recorded here |
| 7 terminal demo | `seccomp=unconfined` is redundant beside `--cap-add SYS_ADMIN` (Docker's default seccomp admits `mount`, `unshare`, `setns` with that capability) and is removed; the per-demo `--privileged` stays a manifest decision per demo, each to be justified in the manifest |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIPsecLabPeersRunWithoutPrivilege` | `internal/le/interoplab/ipsec/privilege_test.go` | AC-1 | |
| `TestNetSysctlAppArmorProfileGrantsOnlyNetWrites` | `internal/le/interoplab/apparmor_netsysctl_test.go` | AC-2 | |
| `TestNATSetupScriptWritesNoSysctl` | same | AC-1 | |
| `TestL2TPLabPeersRunWithoutPrivilege` | `internal/le/interoplab/l2tp/privilege_test.go` | AC-3 | |
| `TestPPPoELabPeersRunWithoutPrivilege` | `internal/le/interoplab/pppoe/privilege_test.go` | AC-4 | |
| `TestRSVPTELabZeRunsWithoutPrivilege`, `TestRSVPTEAppArmorProfileAdmitsEverySetupWrite` | `internal/le/interoplab/rsvpte/privilege_test.go` | AC-5 | |
| `TestL2TPDeploymentContainerRunsWithoutPrivilege`, `TestVPPDeploymentContainerRunsWithoutPrivilege` | `internal/le/test/deployment/privilege_test.go` | AC-6, AC-7 | |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| interop-ipsec on Ze's kernel | `./le test qemu docker-lab lab "test integration interop-ipsec"` | AC-8, AC-9 | owed |
| L2TP, PPPoE, RSVP-TE labs | `./le test qemu docker-lab lab "<lab action>"` | AC-8, AC-9 | owed |
| deployment proofs | `./le test deployment l2tp-ppp-test`, `vpp-iface-test` | AC-8, AC-9 | owed |
| profile parse | `apparmor_parser -Q` in `ubuntu:24.04` | AC-2, AC-5 | owed |

### Interop Tests (Scope: protocol)
N-A as new scenarios: no protocol behavior changes; the existing scenarios are the proof (AC-8).

## Files to Modify
- `internal/le/interoplab/ipsec/ipsec.go`, `nat.go` - grants, `--sysctl`, profile
- `internal/le/interoplab/l2tp/l2tp.go`, `test/interop-l2tp/*` - grants, preflight
- `internal/le/interoplab/pppoe/pppoe.go`, `scenarios.go`, `test/interop-pppoe/Dockerfile.*` - grants, preflight, entrypoints
- `internal/le/interoplab/rsvpte/rsvpte.go` - grants, preflight
- `internal/le/test/deployment/l2tp.go`, `vppiface.go` - grants
- `internal/le/site/terminaldemo/render.go` - seccomp
- `internal/test/fixture/ui_fixture_le_evidence_answers.go`, `ui_fixture_le_evidence_vpp_answers.go` - assert the grants, not `--privileged`
- `docs/architecture/testing/interop.md` - the profile table; `plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md` D-7 table rows

## Files to Create
- `internal/le/interoplab/apparmor_netsysctl.go`, `internal/le/interoplab/register_apparmor_netsysctl.go` - the generic `ze-lab-net-sysctl` profile and its registration
- the `privilege_test.go` files above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | tooling |
| YANG validation constraints | No | tooling |
| YANG custom validators | No | tooling |
| CLI commands/flags | No | the existing `apparmor confirm <name>` takes any registered name |
| CLI grammar (keyword before value) | No | unchanged |
| Editor autocomplete | No | unchanged |
| Functional test for new RPC/API | No | no RPC |
| Pipe completeness | No | no output change |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | lab containers, not a ze runtime dependency; the host check is `./le setup docker-kernel check` |
| Prometheus counters/metrics | No | none |
| BGP family surface (new SAFI / capability / attribute) | No | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | lab tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md` names the AppArmor load action; it derives the names from the registry listing |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (lab containers' privileges and profiles) |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md`, declared by `internal/le/test/deployment` and `internal/le/site/terminaldemo` files, is unaffected: no architecture changes, only container argv |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/pre-release/spec-lab-containers-least-privilege.md` per commit |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/testing/interop.md` prose naming `--privileged` per lab |

## Implementation Steps

One container site per commit, test first:
1. IPsec (AC-1, AC-2): write the tests, see them red, change `ipsec.go`, `nat.go`, add the profile.
2. L2TP (AC-3), 3. PPPoE (AC-4), 4. RSVP-TE (AC-5), 5. deployment (AC-6, AC-7), 6. terminal demo seccomp (AC-10).
7. Each commit owes its lab run on Ze's kernel (AC-8) and one grant-removal red per grant (AC-9), run serially, at low host load.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | each lab still passes on Ze's kernel |
| Correctness | no grant without a named operation that needs it |
| Naming | profile names `ze-lab-<lab>[-<peer>]` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] every assumption row carries basis and validation
- [ ] every AC is testable

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
