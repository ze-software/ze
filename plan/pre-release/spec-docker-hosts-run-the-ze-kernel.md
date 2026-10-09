# Spec: docker-hosts-run-the-ze-kernel

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | `plan/pre-release/spec-appliance-ships-ze-kernel.md` (AC-19: `internal/appliance/kernel.version` pinned to the latest stable 7.x, 7.2.9 on 2026-10-09, with its SHA-256 pin, AC-8/AC-9) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner instruction (Thomas, 2026-10-09): "we should NEVER use docker image without the latest 7.x kernel".
Read for meaning: every Docker daemon that runs Ze's interop, deployment or demo containers runs on
the latest stable 7.x kernel, which is the one `internal/appliance/kernel.version` pins, and the
harness REFUSES to run on any other kernel instead of producing failures.

The symptom. `mobike-initiator` and `mobike-responder` fail on every nightly and on the Mac
(`plan/spec-interop-image-copies-a-prebuilt-ze.md`, AC-9: `34 passed, 3 failed`). Ze offers MOBIKE
only when the kernel serves `XFRM_MSG_MIGRATE_STATE` (`xfrmMigrationAvailable`,
`internal/component/ike/dataplane/xfrm_linux.go`), which first appears in Linux 7.2. The Mac's
colima VM runs Ubuntu `6.8.0-117-generic`; the nightly runs on GitHub's `ubuntu-latest`. The suite
spends 15 minutes and reports two protocol failures that are really a host fact.

A distro 7.x kernel is not enough. Found 2026-10-09 in the colima VM:
`/boot/config-6.8.0-117-generic` reads `# CONFIG_XFRM_MIGRATE is not set`, so Ubuntu's generic
config omits the migration code entirely, and a mainline build packaged with Ubuntu's config would
carry the same omission. Ze's `gokrazy/kernel/runtime.config` carries `CONFIG_XFRM_MIGRATE=y`.
The Docker host therefore runs Ze's own kernel, built from the same declaration the appliance uses.

Goals.

| ID | Goal |
|----|------|
| G-1 | One kernel declaration: the Docker-host kernel is built from `internal/appliance/kernel.version`, its digest pin and `gokrazy/kernel/patches/`, per arch, natively, cached |
| G-2 | Every Docker host Ze's labs use (colima on the Mac, the Linux amd64 host, the scheduled nightly) runs that kernel |
| G-3 | A lab, deployment proof or demo render started on any other kernel stops before it builds an image, naming the required and the found kernel |
| G-4 | `mobike-initiator` and `mobike-responder` pass on the Mac and on the nightly |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - the labs, their preflights, "IPsec address movement"
  → Constraint: "The Docker host must support `XFRM_MSG_MIGRATE_STATE`" (IPsec address movement) is today prose with no check; this spec makes it a refusal and rewrites that paragraph.
  → Decision: the refusal sits in `Suite.Run` (`internal/le/interoplab/lab.go`) right after `Docker.Probe`, before `Suite.Preflight`, so no lab can omit it; a per-lab `PreflightCheck` would be a list every new lab must remember.
- [ ] `docs/architecture/appliance/kernel-profiles.md` - profile registry (`internal/appliance/kernelreg.go`)
  → Decision: a profile is discovered from `<profile>.config` + `<profile>.require` in the target's config dir; `# ze-base: <profile>` stacks one base profile; `# ze-include:` pulls shared fragments from `tools/kernel-builder/common/`. A `docker-host` profile is two new files and no Go edit.
- [ ] `docs/architecture/testing/qemu-integration.md` - `./le test qemu run kernel <vmlinuz> packages ...` boots Alpine userland on Ze's kernel under KVM or HVF
  → Constraint: `Run.assertRuntimeKernel` (`internal/le/test/qemu/run_exec.go`) already compares the guest's `uname -r` with `internal/appliance/kernel.version` (equal, or the pin followed by `.`). The Docker-host refusal is the same comparison; it MUST become one shared function with two callers, not a second copy.
- [ ] `docs/architecture/testing/ci-workflows.md` - `evidence-nightly.yml` runs `interop`, `interop-ipsec`, `docker-l2tp-ppp-test`, `docker-pppoe-accel-test` on `ubuntu-latest`
  → Constraint: a GitHub-hosted runner cannot boot another kernel; the job must move its Docker daemon into a VM it boots, or move to a self-hosted runner.
- [ ] `ai/rules/platform-linux.md` - QEMU proofs boot Ze's runtime kernel; Docker labs ship a QEMU path
  → Constraint: its sentence "the Alpine QEMU VM has no Docker" becomes false under the nightly route below and is edited in the same change.
- [ ] `plan/pre-release/spec-appliance-ships-ze-kernel.md` - the pin, the cache, the no-emulation decision
  → Decision: every kernel is built on a host of its own arch (arm64 on the Mac, amd64 on the Linux host); the cache under `~/.cache/ze/runtime-kernel/` is never a reclamation target.

**Key insights:**
- The colima VM (`vmType: vz`, Ubuntu 24.04, grub-efi-arm64 2.12) boots `/boot/vmlinuz-<release>` through EFI grub with no initramfs (`root=PARTUUID=...`, no `initrd.img` in `/boot`). Installing a kernel into it is the ordinary Linux route: files in `/boot` and `/lib/modules`, `update-grub`, restart. Grub orders entries by version, so 7.2.9 outranks 6.8.0 without pinning a default.
- Ze's arm64 runtime build emits the uncompressed `arch/arm64/boot/Image` as `vmlinuz` (`workerArch`, `internal/appliance/kernelbuilder/worker.go`); grub boots it through the EFI stub that arm64 defconfig enables.
- Installed tools 2026-10-09: colima 0.10.3, limactl 2.2.0, docker client 29.8.2, server 29.5.2.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/interoplab/lab.go` - `Suite.Run`: no scenarios, then `Docker.Probe`, then `Suite.Preflight`, then image builds. A setup failure sets `SetupError`, `Code = 1` and runs nothing.
- [ ] `internal/le/interoplab/docker.go` - `Probe` runs `docker info`; `ServerArchitecture` asks the daemon, never the host, and refuses an answer it cannot read.
- [ ] `internal/le/dockerhost/dockerhost.go` - chooses the Docker socket (colima's `~/.colima/default/docker.sock` when the default endpoint is absent); imported by `internal/le/test/qemu` and `internal/le/site/terminaldemo/render.go`.
- [ ] `internal/le/test/qemu/run_exec.go` - `assertRuntimeKernel` reads `internal/appliance/kernel.version` from the checkout and accepts `uname -r` equal to it or beginning with it and a dot.
- [ ] `internal/appliance/cmd_kernel.go` - `ze appliance kernel --target runtime --profile <p> --arch <a>`; the runtime target defaults to profile `runtime` and accepts any registered profile.
- [ ] `internal/appliance/kernelreg.go` - `resolveKernelProfile`: base `kernel.config`, optional `ze-base`, the profile, then `ze-include` fragments.
- [ ] `internal/le/setup/setup.go` - `./le setup install` host checks, for example `kvm-access`.
- [ ] `.github/workflows/evidence-nightly.yml`, `.github/workflows/qemu-nightly.yml` - interop jobs on `ubuntu-latest`; `qemu-nightly` already restores and builds the amd64 runtime kernel cache keyed on `kernel.version`, `gokrazy/kernel/**`, `tools/kernel-builder/**`, and boots it under KVM.

**Behavior to preserve:**
- `SuiteReport` JSON and text shape; a setup refusal stays `interop: setup: <reason>` with exit 1.
- The kernel builder's Docker backend (`ze-kernel-builder`) keeps working on a Docker host that does not yet run the pinned kernel, because it is how that kernel is built (decision D-4).
- `assertRuntimeKernel`'s verdict for the `runtime` profile.

**Behavior to change:**
- A lab, a Docker deployment proof or a terminal-demo render on a Docker daemon whose kernel is not Ze's pinned docker-host kernel stops before any image build.
- The Mac, the Linux host and the nightly gain a documented, scripted route to that kernel.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test integration <lab>`, `./le test deployment docker-*`, `./le site terminal-demo ...`: each reaches a Docker daemon.
- The daemon's kernel release, as the daemon reports it: `docker info --format {{.KernelVersion}}` (for example `6.8.0-117-generic`).

### Transformation Path
1. The caller probes the daemon (`Docker.Probe` for labs).
2. The shared kernel check reads the pin from `internal/appliance/kernel.version` in the checkout and the release from the daemon.
3. It compares them with the one comparison `assertRuntimeKernel` also uses, plus the Docker-host identity suffix (D-2).
4. A mismatch returns an error; the caller records it as its setup failure and runs nothing.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le ↔ Docker daemon | `docker info` over the socket `dockerhost` selects | No |
| le ↔ checkout | reads the tracked pin file, as `assertRuntimeKernel` does | No |
| host ↔ Docker VM (Mac, Linux host) | `./le setup docker-kernel install` copies the cached kernel tree in and updates grub | No |

### Integration Points
- `Suite.Run` in `internal/le/interoplab/lab.go` - the refusal runs after `Probe`.
- The Docker deployment proofs in `internal/le/test/deployment/` - same refusal before their first `docker build`.
- `internal/le/site/terminaldemo/render.go` - same refusal before the renderer image build.
- `ze appliance kernel --target runtime --profile docker-host` - builds and caches the kernel.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the daemon is asked, as `ServerArchitecture` does; the host is never assumed |
| No unintended coupling (components stay isolated) | Yes | le reads the tracked pin file; it does not import `internal/appliance` into `interoplab` |
| No duplicated functionality (extends existing, does not recreate) | Yes | the comparison is lifted out of `assertRuntimeKernel` into one function both callers use; the kernel build is the existing runtime target with a new profile |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no wire path |
| Registration over hardcoding, outbound | Yes | the profile is two files discovered by `resolveKernelProfile`; no switch learns `docker-host` |
| Registration over hardcoding, inbound | Yes | searched `kernelTargetFor` (switch over targets, not profiles: untouched), `validateProfile` (pattern only), `qemu-nightly.yml` cache key (`gokrazy/kernel/**` already covers the new files); the refusal lives in `Suite.Run`, so no lab list learns it |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | colima's vz VM boots any kernel installed in its own `/boot` through grub, with no initramfs | `/proc/cmdline` `BOOT_IMAGE=/vmlinuz-6.8.0-117-generic root=PARTUUID=...`; `/boot` holds no initrd; `vz-efi` exists under `~/.colima/_lima/colima/` | the install route fails on the Mac; fall back to lima's `images[].kernel` direct boot (Key Design Decisions) | AC-6: `docker info` reports the pinned release after `colima restart` | unvalidated |
| A-2 | the docker-host profile carries every symbol Docker, the lima guest and the labs need built in (virtio-blk, ext4, virtio-net, virtiofs, vsock, overlayfs, bridge netfilter, veth, nftables and iptables compatibility, cgroup v2, namespaces, EFI stub) | `runtime.config` has `VIRTIO_BLK`, `EXT4_FS`, `OVERLAY_FS`, `VETH`, `BRIDGE`, `NF_NAT`, `XFRM_MIGRATE` = y; it lacks `BRIDGE_NETFILTER`, `VIRTIO_FS`, `VSOCKETS`, `NETFILTER_XT_MATCH_ADDRTYPE`, `IPVLAN`; arm64 defconfig supplies EFI and cgroups | Docker or the VM fails to start | `docker-host.require` lists each symbol; the builder's `enforceRequiredSymbols` refuses a build missing one; moby's `check-config.sh` run inside the booted VM reports no missing required option | unvalidated |
| A-3 | the Linux amd64 host boots through grub with `update-grub` (Debian or Ubuntu) | owner statement of a Linux host; not inspected from here | the install action needs that distro's loader step | owner runs AC-7 on the host | unvalidated |
| A-4 | Docker runs inside the Alpine QEMU guest booted on the docker-host kernel, under KVM on `ubuntu-latest` | Alpine packages `docker`; `qemu-nightly.yml` already grants `/dev/kvm` and boots Ze's kernel there | the nightly route fails; fall back to a self-hosted runner on the Linux host (D-3) | AC-9 run in `evidence-nightly` | unvalidated |
| A-5 | the 7.2.9 pin of AC-19 lands before this spec's ACs are demonstrated | the depended-on spec is `ready` | the refusal demands 7.2.0 instead; MOBIKE still works (7.2) but the owner's "latest" is not met | `cat internal/appliance/kernel.version` reads `7.2.9` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `colima delete` or a colima upgrade re-creates the VM from its stock image and the kernel is gone | the next lab refuses naming `6.8...` | the refusal message names `./le setup docker-kernel install`; the `./le setup install` check reports it too |
| R-2 | Ubuntu inside colima installs a newer distro kernel that grub ranks above 7.2.9 (an 8.x or a later 7.x) | refusal names that release | the install action sets `GRUB_DEFAULT` to the Ze entry by its title, not by rank |
| R-3 | the refusal blocks the kernel build it is meant to enforce | `ze appliance kernel` refuses on a 6.8 daemon | the kernel builder is not a caller (D-4) and AC-10 holds it |
| R-4 | a pin bump makes every host refuse until reinstalled | refusal names the new pin | intended: that is "latest"; the cold build is about 30 minutes per arch and cached |
| R-5 | the ephemeral nightly VM rebuilds every lab image each night | job wall time | acceptable inside the 180-minute budget `qemu-nightly` already uses; measured by AC-9 |
| R-6 | `docker info` answers an empty or malformed `KernelVersion` (remote context, rootless) | empty field | the check refuses with what it asked and what it got, never passes (AC-4) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | every Docker lab, Docker deployment proof and demo render refuses on a host that has not installed the kernel; no product behavior changes |
| How is it reverted? | single commit revert; the installed kernel stays in the VM and is harmless |
| Who else touches this path? | `plan/pre-release/spec-appliance-ships-ze-kernel.md` (the pin), `plan/immediate/spec-appliance-kernel-vpn-modules.md` (adds ESP, xfrm interface and WireGuard symbols to `runtime.config`, which `docker-host` inherits through `ze-base`), `plan/spec-terminal-demo-showcase.md` (renders in Docker), `plan/spec-interop-image-copies-a-prebuilt-ze.md` (AC-9 depends on this), `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md` (its gap narrows once the lab host runs Ze's kernel; this spec does not close it) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration interop-ipsec` on a daemon reporting `6.8.0-117-generic` | → | `Suite.Run` kernel refusal | `TestSuiteRefusesDockerHostOffThePinnedKernel` (`internal/le/interoplab/lab_test.go`, recording `docker` on PATH) |
| `./le test deployment docker-l2tp-ppp-test` on the same daemon | → | deployment refusal | `TestDockerDeploymentRefusesOffThePinnedKernel` (`internal/le/test/deployment/`) |
| terminal-demo render on the same daemon | → | render refusal | `TestRenderRefusesOffThePinnedKernel` (`internal/le/site/terminaldemo/render_test.go`) |
| `ze appliance kernel --target runtime --profile docker-host --arch arm64` | → | `resolveKernelProfile` over `docker-host.config` | `TestDockerHostProfileResolvesOverRuntime` (`internal/appliance/kernelreg_test.go`) |
| `./le setup install` | → | `docker-host-kernel` check | `TestSetupReportsDockerHostKernel` (`internal/le/setup/setup_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze appliance kernel --target runtime --profile docker-host --arch arm64` on the Mac, and `--arch amd64` on the Linux host | builds natively from `kernel.version`, its digest pin and the patch series; the resolved fragments are `kernel.config`, `runtime.config` (via `ze-base`), `docker-host.config`; the build enforces `runtime.require` and `docker-host.require`; the tree is cached under `~/.cache/ze/runtime-kernel/` beside the `runtime` variant without evicting it |
| AC-2 | the built tree's kernel release | equals the pin followed by the docker-host suffix decided in D-2 (for example `7.2.9-ze`), read from the tree's `lib/modules/<release>` directory name |
| AC-3 | a lab, a Docker deployment proof or a demo render, daemon `KernelVersion` is anything other than the pinned docker-host release | exits 1 before any image build or container start, with one line naming the required release, the found release, the pin file, and the install command; the report counts no scenario as passed, failed or skipped |
| AC-4 | daemon `KernelVersion` empty or not a release string | refuses as AC-3, naming the command it ran and the answer it read; never proceeds |
| AC-5 | daemon `KernelVersion` equals the pinned docker-host release | the suite proceeds to `Preflight` and image builds unchanged |
| AC-6 | Mac: `./le setup docker-kernel install` with the arm64 tree cached | copies `vmlinuz` and `lib/modules/<release>` into the colima VM, makes the Ze entry grub's default by title, restarts colima; afterwards `docker info` reports the pinned release |
| AC-7 | Linux amd64 host: the same command with the amd64 tree cached | installs into the host's `/boot` and `/lib/modules`, sets the grub default, and tells the operator a reboot is owed (it does not reboot the host itself); after reboot `docker info` reports the pinned release |
| AC-8 | `./le setup install` on a host whose daemon is off the pin | reports `docker-host-kernel` failing with found and required releases and the install command, beside `kvm-access` |
| AC-9 | `evidence-nightly.yml` jobs `interop`, `ipsec-interop`, `l2tp-interop`, `pppoe-interop` | each runs its lab against a Docker daemon inside the QEMU guest booted on the cached amd64 docker-host kernel (route per D-3); the job log shows the refusal check passing with the pinned release |
| AC-10 | `ze appliance kernel --builder docker` on a daemon off the pin | builds as today: the kernel builder is not refused |
| AC-11 | `kernel.version` bumped while a host still runs the previous pin | that host's next lab refuses naming both releases (unit test with a fake pin and a recording daemon) |
| AC-12 | `./le test integration interop-ipsec` on the Mac after AC-6 | `mobike-initiator` and `mobike-responder` pass; this run is the evidence `plan/spec-interop-image-copies-a-prebuilt-ze.md` AC-9 cites |
| AC-13 | the same suite on the nightly after AC-9 | `mobike-initiator` and `mobike-responder` pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSuiteRefusesDockerHostOffThePinnedKernel` | `internal/le/interoplab/lab_test.go` | AC-3, AC-5: refusal text names required, found, pin file, command; no `docker build` in the recorded argv | |
| `TestDockerHostKernelUnreadableRefuses` | `internal/le/interoplab/lab_test.go` | AC-4 | |
| `TestDockerHostKernelFollowsThePinFile` | `internal/le/interoplab/lab_test.go` | AC-11 | |
| `TestKernelReleaseMatchesPin` | the package that owns the shared comparison | the comparison both callers use: equal, pin plus dot, pin plus docker-host suffix; `7.2.90` does not match `7.2.9` | |
| `TestAssertRuntimeKernelUsesSharedComparison` | `internal/le/test/qemu/run_exec_test.go` | the QEMU guest check keeps its verdicts on the shared function | |
| `TestDockerHostProfileResolvesOverRuntime` | `internal/appliance/kernelreg_test.go` | AC-1 fragment and manifest order | |
| `TestKernelBuilderIsNotRefused` | `internal/appliance/kernelbuilder/driver_test.go` | AC-10 | |
| `TestSetupReportsDockerHostKernel` | `internal/le/setup/setup_test.go` | AC-8 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| kernel release patch level against pin `7.2.9` | exact | `7.2.9-ze` | `7.2.8-ze` | `7.2.10-ze`, `7.3.0-ze` (refused: the pin is exact, D-2) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| refusal on today's colima | recorded run of `./le test integration interop-ipsec` before AC-6 | developer on 6.8 gets one refusal line and exit 1 in seconds, not 15 minutes of failures | |
| MOBIKE on the Mac | recorded run after AC-6 | AC-12 | |
| MOBIKE on the nightly | `evidence-nightly` run | AC-13 | |

### Interop Tests (Scope: protocol)
N-A as a new scenario: this spec adds no protocol behavior. It makes the existing `mobike-initiator` and `mobike-responder` scenarios runnable; their red before (6.8) and green after (7.2.9) is the discrimination record for AC-12.

## Files to Modify
- `internal/le/interoplab/lab.go` - kernel refusal after `Docker.Probe`
- `internal/le/interoplab/docker.go` - daemon kernel release query beside `ServerArchitecture`
- `internal/le/test/qemu/run_exec.go` - `assertRuntimeKernel` calls the shared comparison
- `internal/le/test/deployment/` - the Docker proofs call the refusal before their first build
- `internal/le/site/terminaldemo/render.go` - same
- `internal/le/setup/setup.go` - `docker-host-kernel` check and the `docker-kernel install` action
- `.github/workflows/evidence-nightly.yml` - the four Docker jobs run inside the docker-host QEMU guest (D-3), restoring the amd64 kernel cache as `qemu-nightly.yml` does
- `docs/architecture/testing/interop.md` - host kernel requirement and refusal; rewrite the `XFRM_MSG_MIGRATE_STATE` paragraph of "IPsec address movement"
- `docs/architecture/appliance/kernel-profiles.md` - the `docker-host` profile
- `docs/architecture/testing/qemu-integration.md` - shared kernel comparison; Docker inside the guest
- `docs/architecture/testing/ci-workflows.md` - where the Docker jobs now run
- `docs/guide/developer-setup.md` - installing the Docker-host kernel on the Mac and the Linux host
- `ai/rules/platform-linux.md` - "the Alpine QEMU VM has no Docker" is no longer true

## Files to Create
- `gokrazy/kernel/docker-host.config` - `ze-base: runtime`, the Docker and lima guest symbols of A-2, the release suffix of D-2
- `gokrazy/kernel/docker-host.require` - each of those symbols, enforced by the builder

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling only |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `./le setup docker-kernel install`; `--profile docker-host` already accepted by `ze appliance kernel` |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`; the action takes `arch <a>` keyword-value if an override is needed |
| Editor autocomplete | N-A | le action table |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no ze CLI output |
| Env var registration | N-A | none added |
| Doctor check for runtime dependencies | Yes | `./le setup install` check `docker-host-kernel` (development host, not the appliance doctor) |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | development tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | `ze` CLI unchanged; the le action is documented in `docs/guide/developer-setup.md` |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | MOBIKE (RFC 4555) interop evidence becomes runnable; the `rfc/short/rfc4555.md` carriers cite the passing run |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`, `docs/architecture/testing/qemu-integration.md`, `docs/architecture/testing/ci-workflows.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/appliance/kernel-profiles.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, command, capability or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md` at implementation; `lab.go` declares `interop.md` and `docs/features/interoperability-testing.md`; `render.go` and `dockerhost.go` declare `docs/architecture/core-design.md`, unaffected: it states le's one-import-per-tool composition and Docker as the pinned demo renderer, neither of which changes |
| 17 | Existing docs show examples for this area? | Yes | `qemu-integration.md` kernel examples; `developer-setup.md` colima section |

## Implementation Steps

1. **Phase: Wiring** -- the refusal in `Suite.Run`, the deployment proofs and the render, with the failing wiring tests
   - Tests: the three refusal tests of the Wiring Test table
   - Files: `lab.go`, `docker.go`, `deployment/`, `render.go`
   - Verify: on today's colima the three refuse naming `6.8.0-117-generic`; record that run
2. **Phase: Shared comparison** -- lift the comparison out of `assertRuntimeKernel`, give it the suffix rule of D-2
   - Tests: `TestKernelReleaseMatchesPin`, `TestAssertRuntimeKernelUsesSharedComparison`
3. **Phase: Profile** -- `docker-host.config` and `.require`; build arm64 on the Mac
   - Tests: `TestDockerHostProfileResolvesOverRuntime`; AC-1, AC-2 by build
4. **Phase: Install** -- `./le setup docker-kernel install` and the setup check; run on the Mac
   - Tests: `TestSetupReportsDockerHostKernel`; AC-6, AC-8, AC-12 recorded
5. **Phase: Linux host** -- amd64 build and install, AC-7 (owner runs or grants the host)
6. **Phase: Nightly** -- the four Docker jobs in the guest; AC-9, AC-13 from a dispatched run
7. **Phase: Docs and rule** -- the pages and the `platform-linux.md` sentence

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has an implementation at file:line or a recorded run |
| Correctness | the refusal compares the DAEMON's kernel, never `uname -r` of the machine running le |
| Correctness | one comparison function, two callers; no second reading of `kernel.version` with different rules |
| Data flow | the refusal precedes `Preflight`, so `StageBinaries` never cross-compiles for a host that cannot run the lab |
| Rule: principles (silent value) | an unreadable `KernelVersion` refuses; it never reads as a match |
| Rule: no-layering | no "warn and continue" mode beside the refusal |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| docker-host profile | `ls gokrazy/kernel/docker-host.config gokrazy/kernel/docker-host.require` and a cached tree for each arch |
| refusal | the recorded 6.8 run printing the refusal line |
| colima on the pin | `docker info --format {{.KernelVersion}}` on the Mac |
| nightly on the pin | the `evidence-nightly` log line |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the daemon's `KernelVersion` is untrusted text: bounded length, release-token characters only, quoted in the refusal |
| Privilege | the install action runs root steps inside the colima VM through `colima ssh`; on the Linux host it states each `sudo` step and never reboots on its own |

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

- The owner's sentence names a version, but the failure is a configuration: Ubuntu's 6.8 has `# CONFIG_XFRM_MIGRATE is not set`, so a distro 7.x is not evidence the labs can run. Ze's own build is the only kernel whose config this repository controls.

## Key Design Decisions

Owner decisions owed (recommendation first):

| ID | Decision | Recommendation | Alternative |
|----|----------|----------------|-------------|
| D-1 | Which 7.x kernel the Docker hosts run | Ze's own build, profile `docker-host` (`ze-base: runtime`), from the one pin | a mainline or distro 7.2.9 package: no Ze declaration behind it, and distro configs omit `XFRM_MIGRATE` |
| D-2 | What the refusal accepts | exactly the pin plus the docker-host release suffix (`CONFIG_LOCALVERSION`, for example `-ze`), so it identifies Ze's kernel and a pin bump forces every host forward | a floor (pin or newer, any build): accepts a distro kernel missing the symbols the labs need |
| D-3 | Nightly route | keep GitHub-hosted runners; run each Docker lab inside the Alpine QEMU guest booted on the cached amd64 docker-host kernel under KVM, as `qemu-nightly` already boots the runtime kernel | a self-hosted runner on the Linux amd64 host with the kernel installed: faster, warm image cache, but a machine to secure and keep online |
| D-4 | Who is refused | labs, Docker deployment proofs, terminal-demo renders: everything that runs Ze in a container | also the kernel builder: it would refuse to build the kernel it requires |
| D-5 | Docker-host symbols in their own profile or in `runtime.config` | own profile: the appliance does not gain the Docker-host surface (`plan/spec-kernel-lockdown-hardening.md`) | one profile: one tree to cache, but bridge netfilter, virtiofs and vsock ship on every appliance |

Decisions taken in design:

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Mac: install the kernel into colima's own VM disk and let its EFI grub boot it | lima `images[].kernel` direct boot (supported by lima 2.2.0's template, and vz needs an uncompressed Image, which Ze's arm64 build emits); a separate lima instance; booting Docker inside `./le test qemu run` on the Mac | colima regenerates `lima.yaml` on start and 0.10.3 exposes no kernel field, so direct boot rides an override colima does not own; the grub route is the ordinary Linux mechanism, survives `colima restart`, and keeps the developer's image cache warm |
| Linux host: the same install action, host `/boot` plus grub, operator reboots | a VM on the host | the host's daemon is the Docker host; the kernel goes where the daemon runs |
| Refusal in `Suite.Run`, not a per-lab `PreflightCheck` | adding the check to each lab's `Preflights(...)` | a per-lab list is a central enumeration every new lab must remember (`ai/rules/principles.md`) |
| le reads `internal/appliance/kernel.version` from the checkout | importing `internal/appliance` into `interoplab` | `assertRuntimeKernel` already reads the file this way; the file is the declaration and `go:embed` reads the same bytes |
| Ask the daemon (`docker info`) | `uname -r` on the le host, or in a throwaway container | the le host is macOS; a container needs an image pull before the check; the daemon answers its own kernel, as `ServerArchitecture` answers its own arch |

## Known Limitations

- The appliance's own kernel bump, digest pin and GPLv2 notice belong to `plan/pre-release/spec-appliance-ships-ze-kernel.md`; this spec consumes the pin.
- `ipsec-bgp-redistribute-frr` fails for a separate, journalled cause (`plan/journal/unwired-feature.md`, `plan/journal/test-against-broken-path.md`); this spec does not touch it.
- A lab run on the docker-host kernel is evidence about `runtime` plus Docker plumbing, not about the exact appliance tree; `plan/pre-release/spec-ipsec-nat-transport-runs-on-the-runtime-kernel.md` keeps owning the QEMU proof on the shipped kernel.

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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no protocol change; MOBIKE red before and green after is recorded under AC-12)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md` only, in the same `./le commit create` script
