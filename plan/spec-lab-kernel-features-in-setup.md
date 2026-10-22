# Spec: lab-kernel-features-in-setup

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The Docker labs and the deployment proofs need kernel features on the Docker
host: device nodes they pass to a container (`/dev/vhost-net`, `/dev/net/tun`,
`/dev/ppp`) and kernel modules their preflights probe (`l2tp_ppp`,
`l2tp_netlink`, `pppoe`, `mpls_router`, `mpls_iptunnel`). A lab container never
loads a module (`plan/pre-release/spec-lab-containers-least-privilege.md`, D-7:
the host is set up for Ze). Today nothing sets the host up. Each lab names its
modules in a refusal string, and nothing in `./le setup` or in
`./le setup docker-kernel install` knows what any lab needs.

The symptom, on this Linux dev host (2026-10-20): it runs Ze's runtime kernel
`7.2.9-ze`, installed by `./le setup docker-kernel install`. `/proc/config.gz`
shows `# CONFIG_VHOST_NET is not set`, and `sudo modprobe vhost_net` fails with
"Module vhost_net not found". `./le test deployment vpp-test` and
`./le test deployment vpp-iface-test` fail at container start with a raw Docker
error ("no such file or directory") that names neither the kernel option nor
the fix.

Owner decisions, 2026-10-20 ("ok"):

| ID | Decision |
|----|----------|
| D-1 | `CONFIG_VHOST_NET` goes into Ze's runtime kernel (`gokrazy/kernel/runtime.config` and `runtime.require`), because VPP on the appliance needs it too |
| D-2 | This work has its own spec. Owner: "we should have all the modprobe in the le setup for linux" |

-> Pointer (2026-10-21, spec-ze-image-docker-host, closed 2026-10-22): D-1 landed in 81fab69967. `gokrazy/kernel/runtime.config` sets `CONFIG_VHOST_NET=y` and `runtime.require` lists it, with the reason line naming this spec's AC-4. Found there: Kconfig caps `VHOST_NET` at `TAP`, which the arm64 defconfig sets to `m` through `MACVTAP=m`, so `VHOST_NET` resolved to `m` and the arm64 build refused until `CONFIG_MACVTAP=y` was added too (also in `runtime.require`). A-2 is therefore broken as written on arm64 (`TUN=y` alone does not lift it) and holds with `MACVTAP=y`. The kernel cache entry `7.2.9-runtime-arm64-runtime-docker-387d6b76-34cba457` carries both `=y`. That kernel serves the `ze` Colima profile, where AC-15 of that spec ran `vpp-test` and `vpp-iface-test`. This spec's AC-4 kernel build is met by that commit; its other ACs (the setup modprobe route) stay this spec's.

| ID | Goal |
|----|------|
| G-1 | Each kernel feature a Docker lab or a deployment proof needs is declared once, by the lab that needs it, and every consumer derives from that declaration |
| G-2 | `./le setup docker-kernel install` refuses a Ze kernel that lacks a feature a lab needs, naming the lab, the option and the fix |
| G-3 | On a stock distro Linux kernel, `./le setup` loads the modules the labs need, persists them, and names each one the running kernel cannot provide |
| G-4 | The runtime kernel carries `CONFIG_VHOST_NET` |
| G-5 | A VPP proof on a host without `/dev/vhost-net` refuses before its container, naming the option and the fix |
| G-6 | After the owner rebuilds and reinstalls the host kernel and reboots, `vpp-test` and `vpp-iface-test` start their containers on this host |

**Owner action (G-6).** An agent cannot do these steps. After AC-4 lands, the
owner rebuilds the runtime kernel cache entry (`./le build host-driver`, then
`./ze-host appliance kernel --target runtime --arch amd64`), installs it
(`./le setup docker-kernel install confirm 7.2.9-ze`), and reboots. AC-6 runs
after the reboot.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/appliance/kernel-profiles.md` - the runtime profile and what a Docker host relies on it for
  → Decision: an option the host or a lab relies on is built in and declared in `runtime.require`; `TestHostDependencyOptionsAreInTheRuntimeProfile` fails when a dependency table names an option the fragment does not set. The lab needs join that test.
  → Constraint: the runtime release carries `CONFIG_LOCALVERSION="-ze"`; install refuses another suffix.
- [ ] `docs/architecture/testing/interop.md` - "The Docker host kernel check" and the `/dev/ppp` preflight
  → Constraint: the Docker kernel check probes what `kernelcap` enrolls inside a container that holds no device grant, so it cannot see a character device. The device half is a lab preflight (`interoplab/pppdevice.go`).
  → Constraint: a refusal names the next step for the platform through `DockerKernelRoute`, its command on a line of its own.
- [ ] `docs/guide/developer-setup.md` - "Check Mode", "Docker labs", "Linux" (how setup reaches root)
  → Constraint: `./le setup check` changes nothing; `./le setup install` reaches root through `runPrivileged`, always `sudo -n`, asks once with `sudo -v` only with a terminal, and with no route to root prints the command and exits nonzero.
- [ ] `ai/rules/principles.md` - one declaration per fact; registration over a central list
  → Constraint: setup MUST NOT hold a list of lab modules. A lab added later reaches setup, install and its own preflight with no edit outside its package.
- [ ] `ai/rules/platform-linux.md` - Linux-only code and appliance dependencies
  → Constraint: the module step is Linux-only; on macOS the labs run in the Ze-kernel QEMU guest and setup does nothing for them.
- [ ] `ai/rules/user-facing-errors.md` - every refusal names what, why and the exact fix

**Key insights:**
- `internal/le/interoplab/apparmor.go` `RegisterAppArmorProfile` is the precedent: a lab registers what it needs in its `register.go`, and the setup action, the refusal and the container start read one declaration.
- `internal/test/runner/devices.go` already maps `/dev/ppp` to `ppp_generic` for the evidence container. That fact is the same one the PPP labs state, so it joins the one declaration.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/setup/dockerkernel/hostdeps.go` - `readHostFacts` reads `/sys/class/net`, netplan's generated network files, `/proc/modules` and `/etc/ufw/ufw.conf`; `hostDependencies` turns them into `hostDependency` rows (what the host relies on, what breaks, the options); `zeKernelServesHost` refuses a kernel config that builds none of a row's options as `y` or `m`, naming each row. It knows only the host's own network and firewall, nothing a lab needs.
- [ ] `internal/le/setup/dockerkernel/dockerkernel.go` - `runDockerKernelInstall` runs `cachedKernelServesHost` before any root step; without `confirm <release>` it prints the `installStep` list (`printInstallPlan`) and runs none; each step runs through sudo and is printed first; it never reboots.
- [ ] `internal/le/setup/setup.go` - `Setup.Run` visits tools, then on Linux the machine-state rows `visitUserns` and `visitKvm`, then `visitLoopback`; check mode returns the verdict before any change.
- [ ] `internal/le/setup/system.go` - `privilegedStep` (argv, stdin, shown line) and `usernsSteps`: a drop-in written through `tee` plus a command for the running kernel, the pattern for "fix now and after reboot".
- [ ] `internal/le/setup/proc.go` - `Shell.runPrivileged` prints each root command before it runs it and reports the failure detail.
- [ ] `internal/le/test/deployment/vppiface.go` - `vppIface.containerSpec` grants `NET_ADMIN` and the devices `/dev/net/tun` and `/dev/vhost-net`; `vpp-test` (`vppevidence.go` `newVPP`) uses the same spec. No preflight checks the devices, so Docker's start error is what the operator reads.
- [ ] `internal/le/test/deployment/actions.go` - the `vpp-test` and `vpp-iface-test` help text repeats "/dev/net/tun and /dev/vhost-net" as a literal.
- [ ] `internal/le/test/deployment/hostkernel.go` - the on-host L2TP proof loads `l2tpModules` (`ppp_generic`, `l2tp_core`, `l2tp_netlink`, `pppox`, `l2tp_ppp`) itself when root, then reads the evidence paths.
- [ ] `internal/le/test/deployment/l2tp.go` - its container grants `/dev/ppp`.
- [ ] `internal/le/interoplab/pppdevice.go` - `PPPDevice`, `PPPDeviceProbe`, `PPPDeviceMissing` ("module ppp_generic" as text) and `PPPPreflightRunRefusal` ("sudo modprobe ppp_generic" as text).
- [ ] `internal/le/interoplab/l2tp/l2tp.go`, `pppoe/pppoe.go`, `radiussubscriber/radiussubscriber.go`, `rsvpte/rsvpte.go` - each preflight is one shell probe line per feature and a refusal whose `sudo modprobe` line lists the module names as literals.
- [ ] `internal/test/runner/devices.go` and `internal/le/verify/evidence/evidence.go` - `containerDevices` (`/dev/ppp` to `ppp_generic`) and `deviceRefusal`, which maps Docker's "error gathering device information" line to `sudo modprobe <module>`.
- [ ] `gokrazy/kernel/runtime.config`, `runtime.require` - `CONFIG_TUN`, `CONFIG_PPP`, `CONFIG_PPPOL2TP`, `CONFIG_PPPOE`, `CONFIG_L2TP`, `CONFIG_MPLS_ROUTING`, `CONFIG_MPLS_IPTUNNEL` are set; `CONFIG_VHOST_NET` is not.

Today's sites that state a lab kernel fact, as found on 2026-10-20 (an inventory of the tree, not the list the implementation keeps):

| Fact | Where it is stated today |
|------|--------------------------|
| `/dev/ppp`, module `ppp_generic`, `CONFIG_PPP` | `pppdevice.go`, `runner/devices.go`, `deployment/hostkernel.go`, the L2TP, PPPoE and RADIUS subscriber refusals |
| PPP over L2TP, module `l2tp_ppp`, `CONFIG_PPPOL2TP` | `interoplab/l2tp/l2tp.go`, `deployment/hostkernel.go` |
| L2TP Generic Netlink, module `l2tp_netlink`, `CONFIG_L2TP` | `interoplab/l2tp/l2tp.go`, `deployment/hostkernel.go` |
| PPPoE, module `pppoe`, `CONFIG_PPPOE` | `interoplab/pppoe/pppoe.go` |
| MPLS, modules `mpls_router` and `mpls_iptunnel`, `CONFIG_MPLS_ROUTING` and `CONFIG_MPLS_IPTUNNEL` | `interoplab/rsvpte/rsvpte.go` |
| `/dev/net/tun`, module `tun`, `CONFIG_TUN` | `deployment/vppiface.go`, `deployment/actions.go` |
| `/dev/vhost-net`, module `vhost_net`, `CONFIG_VHOST_NET` | `deployment/vppiface.go`, `deployment/actions.go` |

**Behavior to preserve:**
- `hostDependencies` and its refusal for the host's own network and firewall.
- `install` runs no root step without `confirm <release>`, prints each step first, never reboots.
- `./le setup check` changes nothing; setup's root route and its four states.
- Each lab's preflight still runs before any image build or cross-compile, and still refuses rather than skips.
- The lab containers' exact grant sets (`grantcheck` tests).

**Behavior to change:**
- Every row of the inventory above is declared once, by the lab package that needs it; the preflight probe, the refusal text, the `--device` argv, the help text, setup's module step and install's check derive from it.
- `./le setup` gains a Linux machine-state row that loads and persists the lab modules.
- `./le setup docker-kernel install` also refuses a kernel lacking a lab feature.
- The VPP proofs gain a device preflight.
- The runtime kernel gains `CONFIG_VHOST_NET`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le setup install` and `./le setup check` (Linux).
- `./le setup docker-kernel install [builder <b>] [confirm <release>]`.
- `./le test deployment vpp-test`, `vpp-iface-test`, `l2tp-test`, `docker-l2tp-ppp-test`, and each `./le test integration interop-*` lab with a preflight.

### Transformation Path
1. Each lab and deployment proof package registers its kernel needs from its `register.go` init into one registry (fields below).
2. The lab's container spec takes its `--device` list from its registered records, and its preflight builds its probe lines and its refusal from them.
3. `./le setup` on Linux reads the whole registry: for each record it asks whether the running kernel provides the feature (the record's evidence), loads the module through `runPrivileged` when it does not, writes the persistent list, and reports a module the kernel cannot provide.
4. `./le setup docker-kernel install` turns each record into a dependency row and refuses a cached kernel config that sets none of its options.

One record (a table, not a type):

| Field | Meaning |
|-------|---------|
| Lab | the verb that needs it (`vpp-test`), for every refusal |
| Feature | what the lab uses, in words ("vhost-net, the backend of VPP's tap interfaces") |
| Device | the device node passed to the container, or empty when the feature has none |
| Evidence | the path whose presence proves the feature (the device node, `/sys/module/<m>`, `/proc/net/pppol2tp`, `/proc/sys/net/mpls/platform_labels`) |
| Module | the module that provides it when built as a module |
| Options | the kernel options that provide it |

Two labs that need one feature name one record, owned by the package that declares it first in dependency order (`/dev/ppp` lives in `interoplab`, where `pppdevice.go` is today).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Lab package → registry | init registration, read without a lock after init | No, AC-1 |
| le → host root | `runPrivileged`, `sudo -n` | No, AC-3 |
| le → Docker daemon | the preflight one-shot container with the registered `--device` list | No, AC-5 |

### Integration Points
- `interoplab.RegisterAppArmorProfile` - the registration shape the new registry follows.
- `hostDependency` and `zeKernelServesHost` - install's lab rows are `hostDependency` rows; no second refusal path.
- `privilegedStep`, `Shell.runPrivileged`, `Report` - setup's module row.
- `interoplab.DockerKernelRoute` - the platform fix line in each refusal.
- `runner.ContainerDevicePaths`, `runner.DeviceHostModule` - keep their selection of devices, and take the module from the record.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | setup and install read the registry; no lab package is imported by setup by name |
| No unintended coupling (components stay isolated) | Yes | the registry lives in a leaf package under `internal/le` that `interoplab`, `test/deployment`, `test/runner`, `setup` and `setup/dockerkernel` import; `./le arch tier check` passes |
| No duplicated functionality (extends existing, does not recreate) | Yes | install reuses `hostDependency`; setup reuses `privilegedStep`; the refusal reuses `DockerKernelRoute` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no wire path |
| Registration over hardcoding, outbound | Yes | each lab registers its records in its own `register.go` |
| Registration over hardcoding, inbound | Yes | lists searched for the names this work touches: the literal module names in the four `interoplab` refusals and `pppdevice.go`, `deployment/hostkernel.go` `l2tpModules`, `runner/devices.go` `containerDevices`, the `vppiface.go` device list and the `actions.go` help text; each derives from the registry after this spec |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `le` binary links every lab and deployment package, so their init registrations exist when `./le setup` runs | `internal/le/register.go` composes the le areas | setup would miss a lab's need with no error | a test in the `le` composition asserting the registry holds the `vpp-test` and the L2TP records | unvalidated |
| A-2 | `CONFIG_VHOST_NET=y` resolves in the runtime fragment (it selects `VHOST` and needs `EVENTFD`; `TUN` is `=y`) | Linux `drivers/vhost/Kconfig` | the kernel build's `runtime.require` check refuses with "did not resolve to =y" | the kernel build in AC-4 | unvalidated |
| A-3 | On a kernel that builds a feature in, `modprobe <module>` exits 0 (modules.builtin), and the evidence path is present anyway | kmod behavior; `deployment/hostkernel.go` already relies on the evidence over the modprobe exit | setup would report a built-in feature as unloadable | AC-3 unit test, and `modprobe -n tun` on this host | unvalidated |
| A-4 | VPP's tap backend needs exactly `/dev/net/tun` and `/dev/vhost-net` | least-privilege spec A-4 (settled on an amd64 Ubuntu host, 2026-10-10) | AC-6 fails on another device | AC-6 | unvalidated |
| A-5 | Docker refuses a missing `--device` before the container starts and names the path on stderr | `evidence.go` `deviceRefusal`; this host's raw error, 2026-10-20 | the preflight cannot tell a missing device from another start failure | AC-5 run on this host before the reboot | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The rebuilt kernel changes another option and the host does not come up | the host does not boot `7.2.9-ze` | install's host-dependency check still runs first; the previous GRUB entry stays in the menu; the owner picks it |
| R-2 | `DOCKER_HOST` names a remote daemon, so setup loads modules on the wrong machine | the lab still refuses after setup | setup states it acts on this machine; each refusal says "on the Docker host" and keeps `DockerKernelRoute` |
| R-3 | A module written to `/etc/modules-load.d` is absent from a later kernel | `systemd-modules-load` logs a failure at boot | only modules the running kernel provides as modules are written; setup re-reads the file on each run and reports an entry the kernel lacks |
| R-4 | A built-in feature has no `/sys/module/<m>` entry | setup loads a module that is not there | the record's Evidence path is the answer, never `/sys/module` alone (least-privilege spec A-2) |
| R-5 | A feature is needed by a lab that registers nothing, so the gap stays silent | a lab's `--device` grant with no record | `TestEveryLabDeviceGrantIsRegistered` refuses a grant with no record |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A developer host: setup loads a wrong module, or install refuses a kernel it should accept. On the appliance, a kernel that fails its build. Nothing a Ze user runs |
| How is it reverted? | Single-commit revert per phase; the kernel option is one line in each of two files |
| Who else touches this path? | `plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md` (install), `plan/pre-release/spec-lab-containers-least-privilege.md` (lab grants), `plan/spec-lab-docker-cleanup-and-go-test-containers.md` (deployment verbs) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a lab's `register.go` init | → | the registry, then every consumer | `TestLabKernelNeedReachesEveryConsumer` (a test record reaches the container argv, the preflight probe, the refusal, setup's step list and install's rows with no other edit) |
| `./le setup install` on Linux | → | `Setup.Run` → the lab-kernel row → `runPrivileged` | `TestSetupLoadsLabKernelModules` (`internal/le/setup`) |
| `./le setup docker-kernel install` | → | `cachedKernelServesHost` → `zeKernelServesHost` with the lab rows | `TestInstallRefusesKernelWithoutLabFeature` (`internal/le/setup/dockerkernel`) |
| `./le test deployment vpp-test`, `vpp-iface-test` | → | the VPP device preflight | `TestVPPPreflightNamesTheKernelOption` (`internal/le/test/deployment`) |
| every lab and deployment container spec | → | the registered devices | `TestEveryLabDeviceGrantIsRegistered` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The kernel features the labs and deployment proofs need (the inventory under Current Behavior) | Each is declared once, by the package that needs it. The container `--device` argv, the preflight probe lines, the refusal's module names and `sudo modprobe` line, the `vpp-*` help text, setup's module step and install's rows all derive from it. Proof: `TestLabKernelNeedReachesEveryConsumer` passes; `TestEveryLabDeviceGrantIsRegistered` fails when a test adds a `--device` with no record; a grep over `internal/le/setup` and the lab packages for `modprobe` followed by a literal module name finds none outside the registry's renderer |
| AC-2 | `./le setup docker-kernel install` (no `confirm`) with a cached kernel config that lacks `CONFIG_VHOST_NET` | exit 1 before any root step; the refusal names `vpp-test` and `vpp-iface-test`, `/dev/vhost-net`, `CONFIG_VHOST_NET`, and the fix (enable it in `gokrazy/kernel/runtime.config` and `runtime.require`, then the rebuild command `runtimeKernelRebuild` prints for the backend). Proof: `TestInstallRefusesKernelWithoutLabFeature`, and the real dry run on this host before AC-4's kernel is cached, output pasted |
| AC-3 | `./le setup install` on Linux on a stock distro kernel where a lab module is not loaded | each missing module is loaded through setup's root route, each command printed before it runs; `/etc/modules-load.d/ze-labs.conf` lists every lab module the running kernel builds as a module; a feature already present (loaded or built in) gets no step. `./le setup check` prints the same commands, runs none, and exits nonzero while one is owed. A module the running kernel cannot provide is a FAIL row naming the lab, the module, the option and the fix (a kernel with the option, or `./le setup docker-kernel install`), and the run exits nonzero. Proof: `TestSetupLoadsLabKernelModules` over an injected shell and host root; a real run on a stock-kernel Linux host with output pasted; a real `./le setup check` on this host before the reboot, naming `vhost_net` and `CONFIG_VHOST_NET` |
| AC-4 | `gokrazy/kernel/runtime.config` and `runtime.require` | both carry `CONFIG_VHOST_NET`, set `=y`; `./ze-host appliance kernel --target runtime --arch amd64` exits 0 and its emitted config shows `CONFIG_VHOST_NET=y`; `TestHostDependencyOptionsAreInTheRuntimeProfile` (or its successor) also covers every registered lab option and fails when one is missing from the fragment |
| AC-5 | `./le test deployment vpp-test` and `./le test deployment vpp-iface-test` on a Docker host without `/dev/vhost-net` | each exits 1 before the image pull and the cross-compile, with no verdict; the message names the proof, `/dev/vhost-net`, `CONFIG_VHOST_NET`, the module `vhost_net`, and the fix on a line of its own (stock kernel: `./le setup install`; Ze kernel: the runtime option, the rebuild, `./le setup docker-kernel install`, a reboot; through `DockerKernelRoute` for the platform); the Docker error is kept as the cause, never shown alone. Proof: `TestVPPPreflightNamesTheKernelOption`, and both real runs on this host before the reboot, output pasted |
| AC-6 | After the owner action (rebuild, `./le setup docker-kernel install confirm 7.2.9-ze`, reboot) on this host | `uname -r` prints `7.2.9-ze`; `/proc/config.gz` shows `CONFIG_VHOST_NET=y`; `/dev/vhost-net` is a character device; `./le setup check` reports the lab-kernel row present; `./le test deployment vpp-test` and `./le test deployment vpp-iface-test` each start their container (`docker events --filter label=ze.lab` shows the start) and reach their verdict with no device refusal, output pasted |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLabKernelNeedReachesEveryConsumer` | the registry package's `_test.go` | AC-1: one record reaches all six consumers | |
| `TestEveryLabDeviceGrantIsRegistered` | `internal/le/interoplab/grantcheck` or the registry package | AC-1, R-5: every `--device` in a lab or deployment argv has a record | |
| `TestRegistryRefusesDuplicateAndIncompleteRecords` | the registry package | a record with no lab, no evidence or no option, or a second record for one device, panics at init | |
| `TestInstallRefusesKernelWithoutLabFeature` | `internal/le/setup/dockerkernel/hostdeps_test.go` | AC-2: the refusal text and that no step ran | |
| `TestSetupLoadsLabKernelModules` | `internal/le/setup/labkernel_test.go` | AC-3: load, persist, skip present, refuse absent, check mode runs nothing | |
| `TestSetupReportsModuleTheKernelLacks` | `internal/le/setup/labkernel_test.go` | AC-3: the FAIL row names lab, module, option, fix | |
| `TestHostDependencyOptionsAreInTheRuntimeProfile` (extended) | `internal/le/setup/dockerkernel/hostdeps_test.go` | AC-4: lab options are in `runtime.config` and `runtime.require` | |
| `TestVPPPreflightNamesTheKernelOption` | `internal/le/test/deployment/vppiface_test.go` | AC-5: the refusal over a Docker device error | |
| existing grant tests (`TestVPPDeploymentContainerRunsWithoutPrivilege` and the lab grant tests) | their packages | the grant sets do not change | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N-A: no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le setup docker-kernel install` dry run | this host, output pasted here | AC-2 | |
| `./le setup check` before the reboot | this host, output pasted here | AC-3 (absent module) | |
| `./le setup install` on a stock distro kernel | a Linux host on its distro kernel, output pasted here | AC-3 (load and persist) | |
| `./le test deployment vpp-test`, `vpp-iface-test` before the reboot | this host, output pasted here | AC-5 | |
| `./le test deployment vpp-test`, `vpp-iface-test` after the reboot | this host, output pasted here | AC-6 | |

### Interop Tests (Scope: protocol)
N-A: tooling with no protocol peer. The goal validation is AC-6: the VPP proofs run against a real VPP daemon on this host.

## Files to Modify
- `internal/le/setup/dockerkernel/hostdeps.go` - the lab rows join `hostDependencies`.
- `internal/le/setup/dockerkernel/dockerkernel.go` - `cachedKernelServesHost` passes the lab rows.
- `internal/le/setup/setup.go` - the Linux machine-state block visits the lab-kernel row.
- `internal/le/setup/system.go` or a new file beside it - the row's state, steps and apply.
- `internal/le/interoplab/pppdevice.go` - `/dev/ppp` becomes a record; probe and refusal derive.
- `internal/le/interoplab/l2tp/l2tp.go`, `pppoe/pppoe.go`, `radiussubscriber/radiussubscriber.go`, `rsvpte/rsvpte.go`, and each `register_catalog.go` - records, derived probes and refusals.
- `internal/le/test/deployment/vppiface.go`, `vppevidence.go`, `l2tp.go`, `hostkernel.go`, `actions.go`, and its register file - records, the VPP preflight, derived devices, modules and help text.
- `internal/test/runner/devices.go`, `internal/le/verify/evidence/evidence.go` - the module comes from the record.
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - `CONFIG_VHOST_NET`.
- `docs/architecture/appliance/kernel-profiles.md`, `docs/architecture/testing/interop.md`, `docs/guide/developer-setup.md`, `docs/guide/vpp.md`, `docs/functional-tests.md` - see the Documentation checklist.

## Files to Create
- The registry package under `internal/le` (name chosen in the Wiring phase) and its test.
- `internal/le/setup/labkernel_test.go` (and the source file if the row does not fit `system.go`).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no Ze config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | N-A | no new verb or keyword; existing `./le setup` and `./le setup docker-kernel install` gain a row and a check |
| CLI grammar (keyword before value) | N-A | no new keyword |
| Editor autocomplete | N-A | not a ze command |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no new output format |
| Env var registration | N-A | none added |
| Doctor check for runtime dependencies | N-A | development host only; `ze doctor` is for the product. `CONFIG_VHOST_NET` on the appliance is a kernel option the build's `runtime.require` already enforces |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | development tooling |
| 2 | Config syntax changed? | N-A | none |
| 3 | CLI command added/changed? | N-A | no `ze` command |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md` "Docker labs" and "Linux": the lab-kernel row, the drop-in, the check-mode output; `docs/guide/vpp.md`: the deployment proofs' device preflight |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | none |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`: "The Docker host kernel check" (install's lab rows), the `/dev/ppp` preflight paragraph (the registry); `docs/functional-tests.md` VPP rows (the preflight) |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/appliance/kernel-profiles.md`: `CONFIG_VHOST_NET` and why (VPP tap backend, on the appliance and the Docker host), and the lab rows in the profile test. `docs/architecture/core-design.md` (declared by `setup.go` and `system.go`) is checked: it describes setup's install and machine-state model, which this row extends without changing; edit it only if the implementation changes that model |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | no plugin or command |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/spec-lab-kernel-features-in-setup.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `./le setup docker-kernel install` refusal and the `sudo modprobe` lines quoted in `interop.md` are checked against the new output |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the registry package, its registration and its refusal of bad records; one record registered from `test/deployment` for `/dev/vhost-net`; consumers stubbed.
   - Tests: `TestLabKernelNeedReachesEveryConsumer` (fails: consumers stubbed), `TestRegistryRefusesDuplicateAndIncompleteRecords`
   - Files: the registry package, `test/deployment` register file
   - Verify: the registry answers the record; the wiring test fails on the consumers
2. **Phase: Labs declare** - every inventory row becomes a record in its package; container argv, preflight lines, refusals, help text and `runner/devices.go` derive.
   - Tests: `TestEveryLabDeviceGrantIsRegistered`, the existing grant tests, the existing preflight refusal tests
   - Files: the `interoplab` lab packages, `test/deployment`, `test/runner`, `verify/evidence`
   - Verify: grant sets unchanged; refusal texts name the same modules as before
3. **Phase: VPP preflight** - the device preflight before the pull and the cross-compile.
   - Tests: `TestVPPPreflightNamesTheKernelOption`
   - Files: `vppiface.go`, `vppevidence.go`
   - Verify: AC-5 real runs on this host
4. **Phase: Install check** - the lab rows in `hostDependencies`.
   - Tests: `TestInstallRefusesKernelWithoutLabFeature`
   - Files: `hostdeps.go`, `dockerkernel.go`
   - Verify: AC-2 real dry run
5. **Phase: Setup row** - load, persist, report.
   - Tests: `TestSetupLoadsLabKernelModules`, `TestSetupReportsModuleTheKernelLacks`
   - Files: `setup.go`, `system.go` or a sibling
   - Verify: AC-3 real `./le setup check` here; the stock-kernel run
6. **Phase: Runtime kernel** - `CONFIG_VHOST_NET`.
   - Tests: the extended profile test
   - Files: `runtime.config`, `runtime.require`
   - Verify: AC-4 kernel build; then the owner action; then AC-6

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | a built-in feature is never reported missing (Evidence, not `/sys/module` alone) |
| Correctness | setup writes to `/etc/modules-load.d` only modules the running kernel builds as modules |
| Correctness | the VPP preflight runs before the pull and the cross-compile, and keeps the Docker error as its cause |
| Rule: principles | no module name, device path or option is spelled outside its one record, except in tests and the inventory in this spec |
| Rule: user-facing-errors | each refusal names lab, feature, option, module and the fix command on its own line |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One record per feature | `TestLabKernelNeedReachesEveryConsumer`; grep for literal `modprobe` module lists finds none outside the renderer |
| Install refuses a kernel without a lab feature | `TestInstallRefusesKernelWithoutLabFeature` plus the pasted dry run |
| Setup loads and persists | `TestSetupLoadsLabKernelModules` plus the pasted stock-kernel run |
| `CONFIG_VHOST_NET=y` in the runtime kernel | `grep CONFIG_VHOST_NET gokrazy/kernel/runtime.config gokrazy/kernel/runtime.require` and the emitted config |
| VPP preflight | `TestVPPPreflightNamesTheKernelOption` plus the pasted runs |
| VPP proofs run here | AC-6 pasted output |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Root commands | setup runs only `modprobe <registered module>` and the one `tee` of the drop-in; no module name comes from outside the registry |
| Drop-in content | the file holds only registered module names, one per line; an existing file is rewritten whole, never appended |
| Grant widening | no lab container gains a device or a capability; `grantcheck` exact tests stay green |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| The kernel build refuses `CONFIG_VHOST_NET` (A-2) | add the option it names to the fragment, rebuild; never drop `runtime.require` |
| AC-6 reaches a verdict that fails a VPP scenario | a VPP defect, not this spec's: one row in `plan/journal/<class>.md`, the AC still passes on "started and reached a verdict" |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The Docker kernel check cannot see a device node, because its probe container holds no device grant. That is why the device half lives in each lab's preflight, and why `CONFIG_VHOST_NET` is a lab record rather than a `kernelcap` enrolment.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Labs register records; setup and install read the registry | a list of modules in `internal/le/setup` | owner D-2 and `principles.md`: a list beside the labs drifts the first time a lab changes |
| Install's lab rows are `hostDependency` rows | a second refusal in `dockerkernel.go` | one refusal path, one message shape |
| Setup's module step uses `privilegedStep` and `runPrivileged`: check mode prints the plan and runs nothing, install mode prints each command before it runs it | a `confirm <name>` keyword as `docker-kernel install` has | `./le setup install` is already the operator's consent to root steps (apt, the userns drop-in, the kvm group); a confirm keyword on one row would differ from every other setup row. The owner may still ask for one |
| Persist through `/etc/modules-load.d/ze-labs.conf` | a systemd unit, `/etc/modules` | `systemd-modules-load` reads the drop-in at boot; it mirrors the userns drop-in pattern |
| `CONFIG_VHOST_NET` as a lab record, not a `kernelcap` enrolment | enrol it in `kernelcap` so the Docker kernel check refuses | the check's probe container cannot see the device; the appliance build already enforces the option through `runtime.require` |
| The VPP preflight asks the Docker daemon (a one-shot with the registered devices), never the local `/dev` | `os.Stat("/dev/vhost-net")` | the daemon can be remote (`DOCKER_HOST`, the QEMU guest) |

## Known Limitations
- The QEMU guest scripts (`internal/le/test/qemu/run.go` and its `*_linux.go` helpers) load modules inside the guest they boot. They do not run on the Docker host, so no setup step reaches them, and they are outside this spec.
- AC-6 needs the owner's kernel rebuild, reinstall and reboot. The agent prepares the commands and runs AC-6 after the owner reports the reboot.

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
- [ ] AC-1..AC-6 all demonstrated
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
- [ ] Interop tests for protocol features (N-A: tooling, no protocol peer)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-lab-kernel-features-in-setup.md` only, in the same `./le commit create` script
