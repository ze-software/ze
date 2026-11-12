# Spec: virtual-machine-kernel-profiles

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling, appliance, docs |
| Priority | P3 |
| Release reason | Proposal for smaller VM artifacts and faster native builds; existing deployment correctness remains owned by deployment-user-journeys. |
| RFC gaps | - |
| Depends | - |
| Phase | Proposal drafted; design approval and pinned-source Xen audit outstanding |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

## Task

Build Linux kernels for the virtual devices Ze actually uses in QEMU and Xen.
Preserve the networking, security, storage and recovery behavior of the Network OS.
Keep physical hardware support available through the existing broad runtime profile.

Owner request, 2026-11-12: "Please write a spec about the need to create a kernel dedicated to running in QeMU, and/or xen."

This is an unapproved implementation proposal. Creating this document does not
authorize replacing the current kernel or claim Xen support. Continue the existing
deployment work under `plan/release/delivery/spec-deployment-user-journeys.md`.
That spec's kernel research supplies context, not boot evidence for this proposal.

### Proposed target contract

| Target | Intended deployment | Required proof before advertising support |
|--------|---------------------|-------------------------------------------|
| QEMU amd64 | KVM guest, virtual NICs and disks, serial console, disk and ISO installation | Native amd64 build; optical install and installed boot; direct boot used by labs; networking, update and recovery |
| QEMU arm64 | Native accelerated QEMU guest with the repository's declared machine and firmware | Native arm64 build and boot; same applicable runtime contract, with explicit firmware and device-tree inputs |
| Xen amd64 domU | Unprivileged PVH guest; HVM with Xen PV I/O as a second tested boot path | Actual Xen host, pinned toolstack, frontend block/network/console, guest lifecycle, networking and persistent updates |
| Physical hardware | Existing broad runtime kernel and hardware installer profiles | Existing hardware obligations remain, including NIC/storage drivers and PWM fan support |

The recommendation is separate QEMU and Xen runtime profiles over shared NOS
requirements. A combined VM profile remains an alternative if measured artifact
cost is negligible and both hypervisors pass their tests. Xen dom0, legacy x86 PV,
Xen arm64 and PCI passthrough are design choices outside this proposed initial
matrix. They must not appear in a support claim without a separate approved
contract and proof. A VM kernel does not replace a distribution kernel when a
user installs only the Ze binary. A Docker container uses its host's kernel.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/appliance/kernel-profiles.md`
  → Decision: extend the open fragment/requirement registry; do not create a second kernel builder.
  → Constraint: enforce requirements on the emitted configuration after Kconfig resolves dependencies.
- [ ] `docs/architecture/appliance/build-artifacts.md`
  → Constraint: runtime cache resolution uses the selected source root and native architecture; retain cold-cache refusals.
- [ ] `docs/architecture/appliance/builder.md` and `docs/architecture/appliance/docker-host.md`
  → Constraint: the build host stays native; target selection must reach both appliance and Docker-host image assembly.
- [ ] `docs/architecture/testing/qemu-integration.md`
  → Constraint: boot Ze's own system and verify the actual kernel, PID 1 and root filesystem; a staged file alone proves nothing.
- [ ] `ai/rules/platform-linux.md`
  → Constraint: native kernel builds only; arm64 evidence requires an arm64 host, not foreign-architecture emulation here.

### External platform references

- [ ] [Xen xl.cfg manual, 4.21-testing](https://xenbits.xenproject.org/docs/4.21-testing/man/xl.cfg.5.html), guest type and direct kernel boot.
  → Constraint: PVH has a distinct boot contract without the HVM device model; a QEMU guest test cannot establish Xen support.
- [ ] [Xen Project guest handbook](https://handbook.xenproject.org/users/guests/index.html).
  → Decision: propose PVH for Linux domU and test HVM with PV drivers separately. Record the exact Xen release used for proof.
- [ ] Linux Kconfig and boot code at the release in `internal/appliance/kernel.version`.
  → Constraint: verify Xen symbols and dependencies at that exact source pin before design approval. Older Linux examples are not the specification.

## Current Behavior

**Source files read:**

- [ ] `internal/appliance/runtimekernel.go`, `RuntimeKernelTree`: selects the fixed runtime profile by architecture.
- [ ] `internal/appliance/kernelargs.go`, `resolveBuildParentDir`: passes architecture, not the image's installer profile, to runtime resolution.
- [ ] `internal/appliance/cmd_kernel.go`, `runKernel`: the named image's kernel-profile selection applies to the installer target.
- [ ] `internal/appliance/kernelreg.go`, `resolveKernelProfileFrom` and `RuntimeKernelLayers`: resolves profile pairs, one optional base and shared includes.
- [ ] `internal/appliance/kernelbuilder/worker.go`, `RunWorker`: starts with defconfig, merges fragments, runs olddefconfig and checks requirements.
- [ ] `internal/appliance/kernelreq.go`, `runtimeKernelRequirements`: combines NOS requirements with hardware obligations, including PWM fan support.
- [ ] `internal/appliance/cache.go`, `kernelCacheVariantFor`: keys artifacts by target, architecture, profile, backend and tracked inputs.
- [ ] `internal/appliance/manifest.go`, `BuildManifest`: records image checksum and architecture, without explicit runtime-profile/configuration identity.
- [ ] `internal/appliance/config.go`, `ImageConfig`: installer profile is an appliance JSON field; it is not a daemon YANG setting.

**Preserve:** common networking requirements, kernel patches, authenticated access,
durable configuration, A/B updates, recovery, native builds, source-root isolation,
owned build cleanup and hardware drivers. Existing installer profile selection
keeps its meaning.

**Change:** allow a runtime profile to be selected explicitly throughout the build
and delivery path. Remove irrelevant drivers from VM artifacts through checked
configuration policy. Adding fewer requests to a fragment is insufficient:
defconfig can retain drivers the fragment never mentions.

## Existing Mechanisms

| Mechanism | Where | Today | Proposal |
|-----------|-------|-------|----------|
| Open profile registry | `kernelreg.go`, `resolveKernelProfileFrom` | Fragment/require pairs and shared includes | Extend with runtime targets and declared architecture/role applicability |
| Native kernel worker | `kernelbuilder/worker.go`, `RunWorker` | One build path with emitted-config checks | Retain; add explicit exclusion checks alongside required symbols |
| Runtime resolver | `runtimekernel.go`, `RuntimeKernelTree` | Fixed runtime profile | Carry a validated selected profile through existing callers |
| Cache identity | `cache.go`, `kernelCacheVariantFor` | Separates targets and tracked inputs | Include all profile policy, patch and effective builder inputs |
| Kernel provenance | `kernelbuilder/recipe.go`, `writeProvenance` | Records release, target, profile, architecture, module setting, backend and source digest | Extend and propagate this record into image manifests |
| Image assembly | `kernelargs.go`, `resolveBuildParentDir` | Packages the resolved kernel for gokrazy | Package the selected kernel, matching modules and boot inputs together |
| Build manifest | `manifest.go`, `BuildManifest` | Image and configuration identity | Extend with kernel identity and target contract |
| Existing preview delivery | `internal/le/build/preview/` | Binary/archive and optional Docker artifacts | Reuse its provenance/checksum contract when VM artifacts join delivery; no parallel release tool |

## Data Flow

### Entry Point

The existing appliance JSON image configuration selects architecture and a new
runtime profile. Proposed field: `image.runtime-kernel-profile`. Keep
`image.kernel-profile` as the installer profile. The current runtime profile
remains the default until an explicit repository-wide conversion is approved.

### Transformation Path

1. Validate profile name, registry membership, target role and architecture before building.
2. Resolve the common NOS floor, target boot/device requirements and explicit driver exclusions.
3. Hash resolved policy, Linux source digest, patches and builder identity; resolve the matching cache entry.
4. Build natively when needed; validate the emitted configuration, including forbidden symbols and Kconfig dependency results.
5. Assemble kernel, modules, firmware/device tree when applicable, image and installer media from those resolved inputs.
6. Write provenance and checksums. Boot that artifact in its declared hypervisor and verify its identity inside the guest.

### Boundaries Crossed

| Boundary | Contract | Verified for new profiles |
|----------|----------|---------------------------|
| Appliance configuration → resolver | Explicit installer and runtime selections | No; planned wiring tests |
| Resolver → worker/cache | Same architecture, role, fragments, requirements and patches | No; planned identity tests |
| Kernel outputs → image/ISO | Kernel, modules and boot inputs match the manifest | No; planned artifact tests |
| Image → QEMU/Xen | Declared machine, firmware, boot mode and virtual devices | No; actual boot evidence required |

### Changed-Wire Census

This proposal extends appliance JSON configuration and artifact manifests. It
changes neither protocol packets nor the plugin SDK. Preliminary source searches
covered runtime profile resolution, kernel-profile fields and BuildManifest in
`internal/appliance` and `internal/le`; the fixed-profile resolver and image
assembly are positive controls. Complete the caller census before `design`.

| Wire | Role | Carrier | Source evidence | Disposition |
|------|------|---------|-----------------|-------------|
| Appliance JSON | Writer | `internal/appliance/cmd_init.go`, `runInit` | Existing appliance initialization | Emit an explicit runtime target in VM examples |
| Appliance JSON | Reader | `internal/appliance/config.go`, `ImageConfig` | Existing kernel-profile field | Validate separate runtime selection; convert repository examples directly |
| Runtime selection | Writer | `kernelargs.go`, `resolveBuildParentDir`; `cmd_kernel.go`, `runKernel` | Architecture-only runtime path | Propagate profile instead of dropping it |
| Runtime selection | Reader | `runtimekernel.go`, `RuntimeKernelTree`; `kernelreg.go`, `RuntimeKernelLayers` | Hardcoded runtime selection | Resolve the same declared target for build, capability checks and tests |
| Image manifest JSON | Writer | `manifest.go`, `writeManifest` | JSON serialization of BuildManifest | Include kernel/profile/configuration identity |
| Image manifest JSON | Reader | Artifact checker and external image consumer | Reader population still to audit | Update every repository reader and document the external format before approval |

### Architectural Verification

| Check | Design constraint |
|-------|-------------------|
| No bypassed layers | All producers use the existing resolver and builder |
| No unintended coupling | Hypervisor policy stays in appliance/tooling profiles, outside protocol engines |
| No duplicated functionality | Extend profile registration, artifact validation and delivery checks |
| Zero-copy preserved | N-A: build artifacts are outside the packet path |
| Registration, outbound | Profile data registers available targets; no core per-profile switches |
| Registration, inbound | Help, validation and inventories derive supported names from the same registry |

## Proposed Kernel Policy

| Layer | Keep | Exclude only with emitted-config and boot proof |
|-------|------|-----------------------------------------------|
| Common NOS | IPv4/IPv6 routing, namespaces, virtual interfaces, nftables/conntrack, BPF, traffic control, XFRM/crypto, WireGuard, MPLS, PPP/L2TP/PPPoE and existing required patches | No protocol reduction to obtain a smaller file |
| Common system | gokrazy boot, squashfs root, persistent filesystem, entropy, clocks, cgroups, diagnostics and recovery | Facilities no approved role uses |
| QEMU | Actual virtio transports and devices, machine-specific PCI/ACPI/UEFI or device tree, serial console, ISO storage path | Physical vendor NIC/storage/GPU/USB/sensor families absent from the declared virtual device matrix |
| Xen domU | PVH/HVM guest entry, Xen frontend block/network/console, xenbus, event/grant and lifecycle facilities required by pinned Linux | dom0/backend services and unused physical drivers |
| Hardware | Existing vendor NIC/storage, console, firmware, sensors and PWM fan obligations | No hardware removal authorized by this proposal |

Audit QEMU launch sites before removing e1000 support: existing producers use it.
Either convert those producers to the declared virtio contract with boot proof,
or retain e1000 as a tested compatibility device. Installer kernels need their
own optical/storage/network contract; a runtime kernel is not automatically a
valid ISO installer kernel. Keep module support until all consumers, including
gokrazy startup, are audited. Do not make CONFIG_MODULES=n a shortcut.

Start with the existing defconfig build and explicit exclusions. An allnoconfig
baseline requires a complete dependency closure and is a later design alternative.
Do not use localmodconfig: the build host's loaded drivers do not describe a Ze guest.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | VM drivers are a measurable part of delivery/build cost | Owner's request; broad runtime configuration | Added variants cost more than they save | Same-pin size/build measurements | Unvalidated |
| A-2 | Shared NOS requirements can be separated from hardware-only requirements | Current floor includes PWM fan | VM profile retains unnecessary hardware or loses required behavior | Floor census and capability tests for each profile | Unvalidated |
| A-3 | gokrazy boot/update works with Xen frontend devices | Proposed domU role; no Ze Xen proof recorded here | Additional boot/disk discovery work required | Actual Xen boot, A/B update and previous-slot recovery | Unvalidated |
| A-4 | A native Xen test host is available | No host identified by this proposal | Xen remains unverified and cannot be advertised | Record host/toolstack identity and successful run | Unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | A Kconfig dependency restores an excluded driver | Emitted configuration violates profile policy | Fail artifact validation; fix dependency policy |
| R-2 | A VM profile loses networking features | Common capability or interop proof fails | Preserve shared built-in floor and patches; fix the profile |
| R-3 | Cache serves another profile or modules release | Identity mismatch in manifest/guest | Hash full inputs; refuse mismatched kernel/modules/configuration |
| R-4 | Xen boots but cannot update or recover | Frontend disk/console absent after reboot | Exercise complete lifecycle before support claims |
| R-5 | Hardware loses fan, NIC or storage support | Hardware manifest differs unexpectedly | Preserve broad default and hardware requirements explicitly |
| R-6 | Seeded images expose credentials | Release inspection finds private state | Reuse existing credential and private/public artifact boundaries |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if wrong? | Guest boot, routing, persistence or recovery; a misselected hardware image can lose device access |
| Recovery | Retain an image proven on the same hypervisor and boot mode, with its domain boot inputs and console/rescue path. Use a broad-profile fallback only where it has boot evidence. Preserve state and installed artifacts |
| Concurrent work | deployment-user-journeys and docker-hosts-run-the-ze-kernel touch these producers; coordinate edits before implementation |

## Wiring Test

The following names are planned tests, not existing evidence.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Named appliance build | → | Configuration → runtime resolver → image assembly | `TestBuildUsesSelectedRuntimeKernelProfile` |
| Named appliance kernel target | → | Installer/runtime selection | `TestKernelTargetsUseIndependentProfiles` |
| Docker-host/lab image build | → | Runtime resolver → packaged image | `TestDockerHostUsesSelectedRuntimeKernelProfile` |
| Profile selected in a real image | → | Booted kernel and operator access | `vm-kernel-profile-boot.ci`, plus native QEMU/Xen lifecycle runs |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Select a registered runtime profile | Build, capability checks and image assembly use the same profile and architecture; unsupported pairs refuse before build | Existing broad default remains for hardware |
| AC-2 | Build a VM profile from a cold cache | Emitted configuration satisfies common NOS and target boot requirements and explicit exclusions | Required dependency symbols survive with a recorded reason |
| AC-3 | Build/reuse different profiles at the same Linux pin | Correctly isolated cache entries and matching kernel/module/configuration hashes | Identical content can be reused only when identity is verified |
| AC-4 | Boot each declared QEMU target | Authenticated access, persistent configuration, forwarding and diagnostics succeed on the declared devices; ISO installation boots the installed image | Only documented, tested device combinations are supported |
| AC-5 | Boot Xen amd64 PVH and HVM domU | Actual Xen guest reaches authenticated access, frontend disk/network/console and forwarding; update and recovery preserve configuration | Each mode needs its own evidence; an untested mode stays unverified |
| AC-6 | Exercise existing networking on VM kernels | Representative routing interop and kernel-dependent capability proofs pass with required features built in | Hardware-only capabilities remain owned by hardware profiles |
| AC-7 | Compare broad and VM builds on the same host/source/toolchain | Publish kernel, module and compressed artifact sizes, enabled-driver counts and cold/warm build times; demonstrate a reduction in driver/artifact footprint | No invented percentage or latency promise; report measured tradeoffs |
| AC-8 | Consume a delivered VM artifact | Manifest identifies target, boot mode, profile, Linux release/source digest, patch/config hashes, kernel/modules hashes and builder inputs | Private seeded artifacts remain private; existing binary archives keep their deployment contract |
| AC-9 | Retain the hardware profile | Hardware-specific requirements and existing hardware outputs remain selectable and validated | This proposal does not claim new physical-device coverage |

## 🧪 TDD Test Plan

| Test | Location | Validates | Status |
|------|----------|-----------|--------|
| Wiring names above | Owning appliance/tooling packages | Real selection reaches consumers | Planned |
| `TestRuntimeProfileRejectsUnknownOrUnsupportedTarget` | `internal/appliance/` | Registry/architecture/role validation, including malformed names | Planned |
| `TestVMKernelPolicyChecksEmittedConfig` | `internal/appliance/` | Required=y, missing/modular/forbidden outcomes and positive valid result | Planned |
| `TestRuntimeProfileCacheIdentity` | `internal/appliance/` | Profile, fragment, require, exclusion, patch and builder changes invalidate cache | Planned |
| `TestKernelManifestMatchesPackagedOutputs` | `internal/appliance/` | Actual files match manifest, including wrong-profile/module negative controls | Planned |
| `vm-kernel-profile-boot.ci` | `test/appliance/` | CLI-selected configuration reaches a real image build and guest access | Planned |
| Native QEMU and Xen runs | Existing QEMU harness; registered Xen proof entry | Boot, routing, persistence, shutdown, update, recovery and owned cleanup | Planned |

Boundary tests cover malformed/unknown profile names, valid-but-unsupported
architecture/role pairs, absent config/manifests, missing or modular required
symbols, forbidden symbols, stale hashes and mismatched modules. No new numeric
tuning option is proposed. Each new test must observe a semantic failure under
the relevant deliberately broken selection or requirement before the fixed pass.

## Files to Modify

| Area | Intended changes |
|------|------------------|
| `gokrazy/kernel/` and shared profile fragments | Add declared VM profiles; separate common requirements from hardware policy |
| `internal/appliance/config.go`, `cmd_init.go`, `cmd_kernel.go`, `runtimekernel.go`, `kernelreg.go`, `kernelargs.go` | Validate and carry separate runtime selection |
| `internal/appliance/kernelreq.go`, `kernelbuilder/worker.go`, `cache.go` | Enforce required/excluded configuration and complete identity |
| `internal/appliance/manifest.go`, `cmd_build.go`, `cmd_iso.go` | Record actual packaged kernel identity and preserve installer/runtime distinction |
| `internal/le/setup/docker/host/`, `internal/le/build/gokrazy/`, `internal/le/test/qemu/` | Propagate selection, declare devices and verify booted identity |
| `internal/appliance/doctor_checks.go` | Profile/cache/dependency readiness through existing checks |
| `internal/le/build/preview/` | Extend existing artifact validation when VM outputs are delivered |

## Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | Build-host appliance JSON; no daemon configuration leaf |
| YANG validation constraints | N-A | Validate JSON profile token and registry applicability in appliance configuration |
| YANG custom validators | N-A | No daemon leaf |
| CLI commands/flags | Yes | Extend existing kernel/build selection; audit all registered callers before design approval |
| CLI grammar | Yes | Preserve existing registered command grammar; no ad hoc shell pipeline |
| Editor autocomplete | N-A | No daemon editor surface; build help derives profile names from registry |
| Functional test for new RPC/API | N-A | No new RPC; appliance CLI functional test is required above |
| Pipe completeness | N-A | No new command answer type proposed |
| Env var registration | No | No new environment variable proposed |
| Doctor dependencies | Yes | Existing appliance checks: selected profile, cache identity, native build fallback and hypervisor prerequisite |
| Doctor code per problem class | Yes | Audit/reuse existing codes for unknown profile, unsupported target, incomplete artifact, missing builder/accelerator; add owning codes for uncovered classes before implementation |
| Prometheus metrics | N-A | Build measurements are artifact evidence, not daemon counters |
| BGP family surface | N-A | No protocol or family change |

## Documentation Update Checklist

| # | Question | Applies? | File / reason |
|---|----------|----------|---------------|
| 1 | Feature scope/evidence changed? | Yes | Existing appliance feature declaration: name only targets proven by boot evidence |
| 2 | Config syntax changed? | Yes | Appliance JSON examples in `docs/guide/appliance.md`; no daemon syntax change |
| 3 | CLI changed? | Yes | Existing appliance command reference and generated help |
| 4 | API/RPC changed? | N-A | No daemon API change |
| 5 | Plugin changed? | N-A | No plugin change |
| 6 | User guide? | Yes | `docs/guide/appliance.md`: choose VM/hardware target and install/update it |
| 7 | Wire format changed? | Yes | Appliance builder page documents JSON manifest fields; no protocol packet change |
| 8 | Plugin SDK changed? | N-A | No SDK change |
| 9 | RFC behavior/proof changed? | No | Preserve capabilities; update any affected evidence only if implementation actually changes it |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md`; document Xen runner and evidence scope |
| 11 | Daemon comparison affected? | N-A | Kernel packaging changes no routing feature parity |
| 12 | Architecture changed? | Yes | `kernel-profiles.md`, `build-artifacts.md`, `builder.md`, `docker-host.md` under appliance architecture |
| 13 | Route metadata? | N-A | No route metadata change |
| 14 | Metrics? | N-A | No daemon metric change |
| 15 | Registered inventory changed? | Yes | Profile discovery/help and `ai/INDEX.md` kernel row |
| 16 | Source anchors? | Yes | Current Design headers identify appliance kernel/build pages; complete native citation-anchor census once edit set is approved |
| 17 | Existing examples? | Yes | Audit named appliances, gokrazy builds, Docker-host and QEMU launch examples together |

The discovery surface is the existing kernel-profile row in `ai/INDEX.md`.
The registry owns supported names; emitted-config checks prevent policy drift;
artifact checks plus native boot proofs establish the delivered result. Update
the kernel-profile architecture page in each implementation phase that changes
its contract. Regenerate the website from those pages without claiming unproven
Xen support.

## Implementation Steps

1. **Wiring:** add failing entry-point selection tests and carry a validated runtime profile through existing resolvers and consumers. Update configuration examples and builder documentation with that change.
2. **Policy:** audit every common/hardware requirement, add VM profiles and exclusions, and check the emitted configuration. Validate each Xen symbol against the pinned Linux source before committing its policy.
3. **Artifacts:** extend cache/provenance checks and package matching kernel/modules/boot inputs. Extend existing delivery producers without another build tool.
4. **QEMU proof:** execute native target builds, actual ISO installation and runtime/lab tests. Measure the same-pin broad-profile comparison.
5. **Xen proof:** implement the registered native Xen proof path, then execute PVH and HVM lifecycle tests on an identified host. Fix frontend boot/update/recovery defects before making a support claim.
6. **Delivery and documentation:** validate all supported artifact combinations, publish the measured matrix in repository docs and regenerate the website. Public release/upload requires its existing authorization.

## Critical Review Checklist

| Check | Required judgment |
|-------|-------------------|
| Selection | No producer silently falls back to the broad runtime after a VM target was requested |
| Configuration | Every exclusion is checked after Kconfig; common NOS requirements remain built in |
| Boot evidence | Each architecture/hypervisor/mode claim has its own actual guest evidence |
| Identity | Kernel, module tree, patches, emitted config and image manifest agree |
| Lifecycle | Updates, failed test boot, previous-slot recovery and shutdown use the same target contract |
| Hardware | Fan and physical-driver requirements remain enforced in hardware profiles |

## Deliverables Checklist

| Deliverable | Verification |
|-------------|--------------|
| Selectable VM profiles | Entry-point and registry tests |
| Smaller artifacts with retained NOS capabilities | Emitted-config checks, measured comparison and native capability/interop runs |
| Reusable delivery artifacts | Cache/manifest/checksum tests over actual packaged files |
| Accurate support matrix | Native boot/lifecycle logs for each advertised row; docs/site checks |

## Security Review Checklist

| Check | Required judgment |
|-------|-------------------|
| Source integrity | Pinned Linux digest, patches and builder identity survive caching and delivery |
| Guest boundaries | Xen profile supplies domU frontends; no accidental dom0/backend privilege contract |
| Reduced drivers | Driver removal does not weaken hardening, crypto, firewall or recovery |
| Credentials | Seeded images and logs retain current private-artifact restrictions |
| Cleanup | Build/test cleanup targets exact owned workers, domains and disks |

## Design Decisions Still Required

| Decision | Recommendation | Evidence needed before approval |
|----------|----------------|---------------------------------|
| Separate or combined VM kernels | Separate QEMU/Xen profiles over a common NOS layer | Compare configuration, delivered size and maintenance cost |
| Initial Xen modes | amd64 PVH plus separately verified HVM/PV I/O | Pinned Linux audit, gokrazy frontend boot feasibility and native Xen test host |
| Default selection | Keep broad runtime default; VM builds choose explicitly | Complete caller/example census and migration plan |
| Size target | Demonstrate a measured reduction, then set an evidence-based budget | Same-source/host/toolchain cold and warm measurements |

## Owned journal rows

None. This proposal introduces no claim that existing deployment defects are closed.
