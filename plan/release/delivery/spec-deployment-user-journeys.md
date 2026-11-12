# Spec: deployment-user-journeys

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli, tooling, docs |
| Priority | P2 |
| Release reason | Essential installation, appliance boot, and onboarding paths must work before users can evaluate a release. |
| RFC gaps | - |
| Depends | - |
| Phase | Completed-path docs/site committed; complete source-bound preview passed; live netlab snapshot correction and extended OTA health proof in progress |
| Handoff | `tmp/session/2026-11-10-225434/scratch/deployment-runtime-handoff.md` |
| Updated | 2026-11-12 |

## Historical verification blocker

Actual own-kernel rerun `68552` finished before the topology could start. The
kernel check and fresh Linux Ze build passed, then `runContainerLab` reached
`interoplab.StageBinaries` and cross-compilation of its Linux LE failed on
concurrently changing foreign IKE code. Exact invocation, under native job
`deployment-netlab-initial-fixed`:

```bash
ZE_LE_BUILD_NAME=deployment-user-journeys CGO_ENABLED=0 ./le --name deployment-user-journeys test qemu docker-lab lab "test netlab run" timeout 3600s
```

Retained log
`tmp/session/2026-11-10-225434/scratch/job-deployment-netlab-initial-fixed-611d4b33.log`
names undefined `CodeDoctorIPsecESPFormFastPath` and
`CodeDoctorIPsecESPFormLink` in `espform_fastpath_linux.go` and
`espform_linux.go`. Since that run, main's source read found qualified
`diagnostic.Code…` references in `espFormFastPath.unavailable`,
`espFormFastPath.refuse` and `espFormReceiver.watchFastPath`; the old
unqualified fault is not asserted to remain current. Other IKE edits also
interrupted the earlier source-bound preview run. These repeated changes
prevent dependable remaining proof, not a failure of the already completed
native, ISO, OTA or Compose evidence.

The blocker clears when the owning IKE refactor and its callers are coherent,
then an actual fresh netlab run proves the topology/protocols/routes/traffic
and cleanup, and a fresh source-bound bundle preparation/check passes. Do not
start another blind compile loop or change foreign code to force this proof.
The committed `b914eb1a0b` initial-method fix has unit and post-commit compile
PASS but remains unverified live. Final local canonical site build `53669`
and check `13463` passed after that commit's guide paragraph. No own jobs
remain pending from that attempt. The owner has since requested documentation
and website updates for completed paths, followed by continued verification.
The recorded failures remain historical evidence until fresh runs establish
the current result.

## Documentation continuation, 2026-11-12

The resumed documentation phase added the deployment chooser and completed-proof
limits in `docs/guide/ze-install.md`, device-targeted SSH checks after updates
and recovery in `docs/guide/appliance.md`, and matching homepage/navigation copy
in `website/data/audience.json` and `website/data/nav.json`. Selected patches
exclude other sessions' edits in the shared guide files. Independent review
found one missing systemd producer reference; that reference was added and the
narrow re-review was clear.

JSON validation passed. The fresh named tool build passed in `16648` after the
earlier activity-options and IKE declarations appeared. Canonical site build
`67925` and check `38826` passed with 24 producers and 1,107 declared/written
routes. The generated HTML and Markdown contain the chooser, explicit appliance
SSH checks, updated deployment descriptions and pending netlab status.

`doc check all` ran twice: the final run `31368` still failed on four digest
references to the missing generated `ai/PACKAGE-MAP.md`. Native regeneration
had succeeded, but the file was absent again by the final check. Eight reported
symbol warnings belong to other documentation; none names these two changed
guides. No global documentation pass, public publication or release is claimed.

## Netlab startup continuation, 2026-11-12

Documentation and website changes landed in `2b7ffc1e78`. Actual run `38376`
then built Linux Ze/LE and both images, passed the own-kernel check and reached
topology creation. Initial configuration used the corrected `docker exec`
method, but all three containers had already stopped. Teardown and outer VM
cleanup passed. The failed run did not retain node exit causes.

Source review established a startup-ordering defect: the image starts Ze before
`netlab initial` assigns the local addresses, while BGP listeners use ordinary
address binding. The new declaration uses netlab's existing bounded
`dataplane-wait` wrapper and an initial completion marker. It preserves the
Linux interface setup and starts Ze through `exec`. Failure capture now keeps
private, unique diagnostics with generated inputs and bounded log tails before
teardown; capture errors cannot bypass cleanup.

Main observed semantic RED for diagnostic retention (`73100`) and startup
ordering (`55493`) before production edits. Focused three-regression GREEN
`71442` and full netlab-package GREEN `2346` followed. Independent review
confirmed the upstream metadata/template chain and cleanup preservation. The
actual cause of the old exits remains unobserved, and the fixed live topology
is not yet a pass. Fresh live proof and the current source-bound bundle remain
required.

## Netlab template correction, 2026-11-12

Actual `57720` failed before container creation: the Ze initial template found
the upstream Linux wrapper, but that wrapper's nested relative include could
not resolve in the daemon template search paths. Private diagnostics survived;
outer VM cleanup left nothing behind. No fixed live topology PASS was produced.

The same error was reproduced without a VM by the existing native netlab check,
using private pinned netlab 26.10/Ansible 2.20 tools (`95514`). The corrected
template supplies its own shell header and `set -e`, then includes the same
upstream Linux setup through its package-qualified path. Actual render check
`1416` passed: all three node configurations match goldens and validate with Ze.
The stronger regression failed before this correction (`21559`) and passed
after it (`14762`); the full netlab package also passed (`82091`). An independent
test judge approved and personally recorded the updated assertion, preserving
all YAML/startup checks while adding explicit shell and failure-stop checks.
The next actual topology run remains required.

Actual `53628` subsequently built both binaries/images, passed the own-kernel
check, created all three running nodes and completed initial configuration.
The verifier then failed because netlab 26.10 writes a pickle snapshot by
default, while the verifier needs YAML. Teardown stopped the VM with no
leftovers. The corrected invocation adds the YAML output through the existing
CLI setting, preserving normal outputs. Existing pickle-only deployments are
also refused before any command or teardown.

Main observed snapshot-retention RED `96653` and pickle-ownership RED `17190`,
then focused GREEN `65222` and full netlab-package GREEN `54464`. Actual pinned
netlab create `69738` generated both snapshots, node configuration, provider
configuration and Ansible inventory with the same output setting. Independent
source/documentation review was clear; an independent judge approved and
recorded the exact-argv fixture update after formatting. The live protocol,
route, traffic and cleanup proof remains required.

## Current acceptance evidence, 2026-11-12

This section records the latest results reported by the main thread and
supersedes the historical execution notes below. The specification remains
in progress with the full scope open. Passing source, unit or filesystem-context checks is not a
substitute for the outstanding actual deployment proofs.

| Criterion | Current evidence | Remaining proof or limit |
|-----------|------------------|--------------------------|
| AC-1: native installation and removal | Native implementation landed in `2213186915`. Actual systemd installation from fresh parents, authenticated management, repeat installation, retained state/account on ordinary removal and confined purge passed in `cd7be821`. | This is real distribution/systemd evidence. It is separate from the tmpfs functional stress evidence below. |
| AC-2: upgrade, retention and refusal | Native trust, effective-unit, conflict, ownership and busy-store controls passed; real repeat installation retained configuration and unit identity. A different-version upgrade installed the selected bytes and retained configuration. | The earlier combined changed-version script later timed out in its additional foreground bootstrap; the complete combined script is not a pass. |
| AC-3: source build and named executables | The patched-toolchain build path is implemented. All five shipped flavors compiled after the preview, OTA, container and phase-binding commits; committed-build job `98028` exited 0 with all five OK at HEAD `94e8a7de6383`. Kernel/build documentation landed in `be5381db9c`. | Each compilation result belongs to its recorded source revision. Recurrent foreign API changes caused earlier failed builds; a passed committed build does not establish the pending source-bound artifact bundle. |
| AC-4: supplied two-peer quickstart | Real basic route-exchange/authentication checks passed. The quickstart completed all 80 unchanged stress invocations in the approved private tmpfs context. Functional fixtures landed in `4c05449c18`; all three deployment tests also passed the live CI-path run associated with `eb28da8d95`. | This proves the supplied control-plane tutorial, with its explicit forwarding limits; it does not add production forwarding evidence. The whole-UI run was not green, as recorded below. |
| AC-5: deployment container persistence | Container/netlab code landed in `ef6e1c9806`; runtime correction `adf359f08c` includes noninteractive import and `/tmp` mode `1777`. Actual Compose portion of finished job `84344` printed `DEPLOYMENT-COMPOSE: PASS authenticated edit retained across a new container on the same persistent volume`, including the corrected metadata-aware subtree oracle. Final outer cleanup stopped the VM and left nothing behind. | This proves the authenticated edit/recreation path for the explicitly source-unbound runtime image. The combined job's separate netlab portion failed; it is not an overall runtime PASS. |
| AC-6: live containerlab/netlab | Actual `53628` created all three running nodes and completed initial configuration, then exposed the missing YAML snapshot. Explicit YAML output and pickle-only ownership refusal passed separate RED/GREEN regressions, full netlab package, actual pinned create and independent review. | Declared adjacencies, kernel routes and six directed sourced pings remain unverified. Actual VM cleanup left nothing behind; no protocol/traffic PASS is claimed. |
| AC-7: installer ISO and installed disk | `66cc2f7abb` landed the ISO corrections. Actual native amd64/KVM proof passed optical boot, selected-disk installation, clean poweroff, unchanged extra disk, installed-disk boot and authenticated SSH/version. Final module-bound ISO assembly passed. | This is amd64/KVM evidence; arm64 boot was not run. The retained ISO image does not establish OTA credentials or update recovery. |
| AC-8: appliance image update and recovery | OTA implementation landed in `d5ac4d3249`. Actual run `19822` passed two images, changed runtime, retained persistent UUID/configuration/login/hostname, previous-slot recovery and cleanup. The extended health assertions now passed semantic RED/GREEN checks, full QEMU-package race tests, independent source review and fixture judgment. | The health-bearing actual VM proof remains required. The original identity/persistence/recovery evidence remains valid for amd64 only and does not establish the new health assertion. |
| AC-9: website deployment journeys | Resumed completed-path documentation, homepage and navigation updates passed canonical local build `67925` and check `38826`: 24 producers and 1,107 declared/written routes in sibling `../gh-pages`. Generated HTML and Markdown were inspected. The appliance decommissioning section now passed independent source/link review. | Regenerate the site after the new decommissioning and artifact-status guidance. No remote publication or push occurred; the broad doc gate's generated-index failure is recorded above. |
| AC-10: identified preview delivery | Complete fresh preparation `52283` and separate retained checker `27968` passed: static Linux amd64/arm64 binaries and archives, native amd64 deployment/lab exports, manifest and checksums. Identity is `preview-c4e0e1d13892-ddebbea4677b-dirty`, source digest `ddebbea4677b8d475cfad0c910a62833bb3f89f739b79ca42ed99141adc8a000`; both actual images were version-probed by the producer. | This private bundle is source-bound at its captured revision. Earlier Compose proof used a different source-unbound runtime image; no current-image recreation proof, public artifact, release, push or publication is claimed. |
| AC-11: production-evaluator guidance | The resumed chooser and update guidance distinguish demonstrated paths, tested architecture, source-unbound container evidence, pending live lab proof and unavailable public images. Independent source review cleared the selected guide and website changes. | Outstanding actual deployment evidence remains required; the work does not declare Ze production-ready. |
| Source/runtime troubleshooting | Historical actual `68552` failed staging Linux LE during foreign IKE edits, with successful cleanup. In the resumed phase, a fresh named tool build first failed on activity/IKE declarations, then passed after those producers changed. | Documentation and site updates are complete locally. Fresh actual netlab and source-bound bundle checks remain required; older failures do not establish the current source result. |

Cross-cutting changes also landed: dependency-gate correction `b3da8d6b01`,
coordination-lock correction `9e3c0ab9c0`, and contention-factor correction
`e660eff82e`. The lock change removes only redundant coordination-directory
sync; executable, data-publication, import and ownership durability barriers
remain. The contention correction carries the existing factor into stress
children without changing authored lifecycle or command limits.

The committed all-flavor compilation receipts are under the session's
scratch directory, not its state directory:
`tmp/session/2026-11-10-225434/scratch/job-deployment-preview-committed-compiles-f679c837.log`,
`tmp/session/2026-11-10-225434/scratch/job-deployment-ota-committed-compiles-f679c837.log`
and
`tmp/session/2026-11-10-225434/scratch/job-deployment-container-committed-compiles-f679c837.log`,
plus phase-binding receipt
`tmp/session/2026-11-10-225434/scratch/job-deployment-phase-committed-compiles-f679c837.log`.
The subsequent initial-method commit also passed all five flavors at
`b914eb1a0b0f` in
`tmp/session/2026-11-10-225434/scratch/job-deployment-initial-committed-compiles-7b943cb1.log`.

The whole-UI run selected 323 cases and reported 295 passes, 22 failures,
five timeouts and one skip. All three tests owned by this specification passed.
The `NoBuild` DUT pinned to `97e4aed9` lacked `ze-stripped`, so part of that
whole-suite environment was invalid. No whole-UI green result is claimed.

The first two optimized image-bundle attempts built both images but refused
foreign selected-source changes. A third fresh-launcher attempt, `81255`, failed
on a foreign `yang.EnumValuesOf` compile defect. Later attempts, including
the next phase-bound build, progressed beyond that defect but failed as
foreign IKE/config producers changed. No complete current bundle passed.
Fresh source-bound run `69132` then failed on the foreign IKE `echoRequestType`
reference. Its failure log is retained; the producer removed its exclusively
owned failed output directory, confirmed absent by main. The separate finished
runtime job built fresh Linux Ze/LE and both images, passed Compose persistence,
then exposed the netlab namespace failure. Do not repeat source-bound builds
blindly. Since that failure, the IKE producing function has changed and no
longer references `echoRequestType`. That does not prove the changing IKE tree
is coherent. The resumed phase subsequently built the current named tool and
updated the site; the fresh source-bound bundle result remains owed.
The committed phase binding validates the full selected source after native
binary/archive work, before image work, then compares each staged runtime
recipe with its captured hash before Docker builds it. Later source edits do
not relabel completed native outputs. These source and unit results do not
replace the pending source-bound bundle or live netlab proof. The later
Compose recreation PASS is recorded in the current acceptance matrix.

The runtime-only deployment receipt is
`tmp/session/2026-11-10-225434/scratch/preview/runtime-proof-tmpfix/receipt.json`:
image `sha256:252f5936e98f2a0c1995891a581c7660a6d6fc0b5af9e43d468a8ed1ed58d81d`,
archive hash `11211ed59268c7b9a0c41443b975a978cd298258d8712b3ec5aac6aa897d0638`.
It records preserved existing binary bytes and the current runtime recipe,
not current-source compilation. Actual image inspection found 118 CA
certificates in the retained image lineage and verified `/tmp` mode `1777`.
These checks establish runtime packaging properties only.

All three deployment lifecycle tests completed 80 invocations each, 240 total,
in the independently approved private tmpfs run context against the exact
pinned DUT SHA256
`97e4aed911b572894adf24cbb23e05144710ecb6a87f2f7def6e68a6e3257bf0`.
The native ext4 failure at its correct factor-three 135-second command limit
is retained as a failed run. The tmpfs results do not prove ext4 latency fixed
or persistent-media crash durability; the actual ext4 systemd and ISO proofs
remain separate evidence.

Smaller runtime-kernel profiles and unified ISO/disk delivery remain design
research only. No implementation or measured improvement is claimed. Existing
review records remain history; no final clean Review Gate or overall completion
is recorded here.

## Task

Owner continuation, 2026-11-12:

> ok update the documentation and website for the part we have completed and then continue

Owner request, 2026-11-11:

> I would like you to review how users can deploy our software (installation on a normal distribution, docker, vm, containerlab, gokrazy). Review the features Ze has to satisfy each type of users as well as how well the website present and allow users to test and use in production Ze. We want to reduce the friction to usage.

Owner implementation direction, 2026-11-11:

> we should be able to install/uninstall on linux with the ze command line easily. So please resolve all the issues you have identified. We have an installation via ISO make sure it works too.

Owner one-off work authorization, 2026-11-11:

> allow to start this work, use as many agents as you need

Main claimed this spec with a one-off WIP cap of 14 under that authorization. This exception does not change the standing WIP rule.

Resolve the reviewed gaps from obtaining Ze through the first working peer and routine operations. Deliver native CLI installation and removal, correct container and lab setup, a complete local tutorial, and real appliance installation and update evidence. Prepare preview delivery locally and in CI. Publishing, pushing, or creating a public release requires the owner's separate instruction; no page may claim an unpublished artifact is downloadable.

### Related work and ownership

| Existing work | Relationship |
|---------------|--------------|
| `plan/release/delivery/spec-release-build.md` | Owns the complete authenticated release build program. Reuse its artifact contracts; this spec prepares and checks usable preview artifacts without duplicating signing or dispatch. |
| `plan/release/delivery/spec-release-distribution.md` | Owns package/repository trust, retention and package lifecycle. Preserve those obligations; this spec makes the existing standalone CLI path usable. |
| `plan/release/delivery/spec-finish-appliance-qemu-evidence.md` | Retains its historical L2TP proof obligations. This work proves the general user ISO, installed target, persistent configuration and OTA cycle. |
| `plan/release/validation/spec-release-audit-8-docs-onboarding.md` | Existing audit remains an audit. This spec implements the deployment findings identified in this session. |

## Required Reading

- [ ] `docs/guide/ze-install.md`, local and systemd installation.
  → Constraint: extend the existing install/uninstall commands and preserve stores on ordinary removal.
- [ ] `docs/guide/appliance.md`, build and day-2 operations; `ai/doors/appliance.md`.
  → Constraint: build-host and target binaries have different roles; use the host driver for appliance commands and preserve A/B updates.
- [ ] `docs/guide/docker.md` and `docs/guide/netlab.md`.
  → Constraint: deployment and lab images have different bases but the same daemon grammar; prove the running topology rather than only template rendering.
- [ ] `docs/contributing/gh-pages.md` and `website/AI.md`.
  → Decision: change site sources and native renderers, then regenerate the canonical sibling artifact with the existing site builder.
- [ ] `docs/contributing/patching-the-standard-library.md`.
  → Constraint: public build instructions and Docker builds must select the patched Go tree through existing tooling.
- [ ] `ai/doors/system.md`, `ai/doors/system-plugins.md`, `ai/doors/commands-cli.md`, `ai/doors/le-tooling.md`.
  → Constraint: commands stay with their existing owners; product code never imports development tooling.

**Key insights:** The product already has local installation, systemd setup, persistence, image building, an ISO installer, A/B updates, Dockerfiles, netlab templates and a site manifest. The gaps are incomplete connected journeys and missing end-to-end proof, not a need for a second installation framework.

## Current Behavior

**Historical initial assessment:** the source observations and dated execution
notes in this section describe earlier points in the work. Their then-pending
or unproven statements are retained as history, not current verdicts. The
current acceptance table above takes precedence; agreed behavior constraints
remain applicable.

**Source files read:**
- [ ] `internal/le/site/homebody.go`, `homeTemplate`: homepage invokes a nonexistent Makefile and imports an unexplained config.
- [ ] `internal/le/site/home.go`, `homeBody`: structured audience cards and the template produce HTML and its Markdown mirror.
- [ ] `internal/le/site/docsmanifest.go`, `docsManifest`: publishes netlab under labs; omits the Docker guide.
- [ ] `internal/le/site/releasefacts.go`, `factsFromReleases`: release information is absent while the committed forge snapshot lists none.
- [ ] `internal/le/test/integration/gates.go`, `Table`: current interop command is under test integration.
- [ ] `internal/le/interoplab/bgp/run.go`, `suiteFor`, and `internal/le/interoplab/kernelcheck.go`, `DockerKernelRoute`: the real lab probes the Docker kernel and provides platform-specific remediation.
- [ ] `website/data/audience.json` and `website/labs/bgp-interop/index.html`: appliance availability and lab prerequisites overstate the supplied journey.
- [ ] Root `ze` launcher: executes `bin/ze`, building it when absent. The cloned-checkout quickstart's `./ze` form exists; its separate checkout-free `go install` branch cannot assume that launcher, and the local sink example assumes an unbuilt `bin/le`.

**Behavior to preserve:** restrictive store permissions and ownership; authenticated management; explicit export policy; graceful protocol drain; existing registered commands and feature gates; separate deployment/lab images; safe install target selection; checksum verification; immutable root and persistent data; A/B update and recovery; website information-preservation rules.

**Behavior to change:** native installation and removal require no manual file/unit surgery; examples use existing executables and supplied inputs; container state survives recreation; labs prove route exchange; VM and ISO paths reach an authenticated installed target; update succeeds across reboot; previews have immutable identities and accurate availability labels.

### Agreed phase constraints

| Phase | Constraint |
|-------|------------|
| Native | Bare `ze install` and `ze uninstall` use defaults registered by the existing local plugin. Explicit targets remain supported. Help and unknown targets never mutate the system. |
| Native | Fresh installation initializes only absent state, then starts through the existing systemd owner when available. A host without systemd receives the exact foreground command. Repeat installation preserves compatible administrator units, drop-ins, credentials and config; every conflict check runs before binary replacement. |
| Native | The fresh bootstrap callback publishes a loopback-only SSH configuration. Installing the daemon on an existing distribution does not take over host interfaces. |
| Native | Default removal stops/removes the service before the executable. Stop failure retains both. Ordinary removal retains state and account; purge is explicit and confined to the selected installation. |
| OTA | Partition the existing GPT image into bounded root and boot payloads. Never send the complete disk image to the updater's root endpoint. Preserve target PARTUUID/cmdline handling and follow the pinned upstream updater sequence. Validate a malformed/truncated image before contacting or switching the target. |
| OTA | Wire existing updater credentials and TLS configuration through to the host command; prove the real installed appliance returns with the new version and its prior config/login. |
| OTA | Main verified three existing defects: updater Basic authentication uses the wrong username instead of `gokrazy`; prepared images omit the configured update token/TLS values; issued certificates omit `Device.Address` from their SANs. Correct all three in the existing producers. Partition handling remains a design to prove with focused tests, not a completed fix. |
| ISO | Existing installed-target proof already builds media, boots UEFI, checks GPT and authenticates after disk boot. Extend that path with version/management assertions and an untargeted-disk preservation check. Recognize the common Debian OVMF 4M firmware pair. |

### Historical baseline execution reported by main

| Command / observation | Result and next step |
|-----------------------|----------------------|
| `ZE_INSTALL_KERNEL=$PWD/build/kernel/Image ZE_INSTALL_QEMU_ACCEL=kvm ZE_INSTALL_KEEP=1 ./le test qemu installer-iso` | Exited 1 on 2026-11-11: GRUB executable exists, but `/usr/lib/grub/x86_64-efi/modinfo.sh` is missing. Main is installing `grub-efi-amd64-bin` before rerunning. |
| Baseline build progress | Installer initrd and complete appliance image were built. ISO creation and ISO/installed-disk boot have not yet passed; this is not installation evidence. |
| GRUB prerequisite repair | Main installed `grub-efi-amd64-bin` successfully. The second run passed the missing-module step. |
| `ZE_INSTALL_KERNEL=$PWD/build/kernel/Image ZE_INSTALL_IMAGE=/home/thomas/.cache/tmp/ze-installer-iso-2845175884/appliances/ze-install-qemu/ze-20261111-134239.img ZE_INSTALL_X86_UEFI_BIOS=/usr/share/OVMF/OVMF_CODE_4M.fd ZE_INSTALL_QEMU_ACCEL=kvm ZE_INSTALL_KEEP=1 ./le test qemu installer-iso` | Exited 1: ISO built and its contained image hash matched the source image. QEMU then reported `could not load PC BIOS` for the supplied firmware. ISO and installed-disk boot remain unproven. |
| Second-run firmware diagnosis | Main observed split OVMF CODE/VARS firmware while `ISOArgv` supplies `-bios`. Paired pflash handling is a hypothesis until the firmware producer and boot tests confirm it. |
| Third baseline with complete OVMF code and variable-store firmware | Main reached GRUB, the kernel and installer init. Installation then failed to find media: `internal/install/disk/validate.go` `isSkippedDisk` excludes optical `sr` devices, and media discovery reused that destination-disk exclusion. Installed-disk boot is still unproven. |
| ISO phase repairs in progress | The ISO agent has separated media discovery, repaired firmware handling and improved the GRUB prerequisite check. Main is running the focused installer tests; a new real ISO/installed-disk run remains owed. Written fixes are not recorded as verified boot evidence. |

The retained directories `/home/thomas/.cache/tmp/ze-installer-iso-2845175884` (baseline image) and `/home/thomas/.cache/tmp/ze-installer-iso-1070727973` (second-run failed ISO input) are temporary inputs for the successor proof, not durable proof. Main owns their cleanup after successful rerun or handoff. Retain the eventual command result and canonical source identity in the session's proof record.

### Historical implementation verification, 2026-11-11

These are directly observed scoped results, not a completed deployment verdict.
The named `deployment-user-journeys` launcher evaluates the current checkout.

| Canonical source and command | Observed result / remaining proof |
|-----------------------------|-----------------------------------|
| `internal/core/subdispatch`, `internal/plugins/local`, `internal/plugins/systemd`; `./le --name deployment-user-journeys test unit pkg path ./internal/core/subdispatch/ path ./internal/plugins/local/ path ./internal/plugins/systemd/ path ./cmd/ze/install/ path ./cmd/ze/uninstall/` | Passed after executable ownership, effective systemd policy and busy-store removal corrections. The command wrapper packages contain no tests. Independent review accepted these corrections; actual systemd lifecycle remains owed. |
| `internal/test/fixture/register_native_install.go`, readiness controls and draft `native-install-lifecycle`; `./le --name deployment-user-journeys test ui --draft --pattern native-install-lifecycle` | Five readiness unit controls and the basic draft lifecycle passed after correcting context handling and failure diagnostics. The draft covers install, authenticated management, repeat install without bootstrap input, retained configuration/login, busy-daemon refusal, ordinary removal and confined purge. The third 80-run stress attempt then failed on invocation 77 after 76 passes (`tmp/stress-repro/ui-draft-native-install-lifecycle-68df71f1df467e9af810a9fb3f8af3688ac5cc67ac96891f6e9e0012eaf7b536-20261111-174501-2569526058.log`): the whole 90-second lifecycle expired during authenticated readiness. No product cause is proven. Phase-only diagnostics were added without changing assertions or deadlines; trace replay remains owed. No stress pass or promotion is claimed. |
| `internal/component/cli/sshclient`, `internal/component/cli/client`, `internal/plugins/completion`, `internal/component/doctor`; main's full race suite, `job-deployment-ssh-full-9298bd8c.log` | Passed the full selected population with no skips, including actual IPv6/OpenSSH trust-file connections, standalone credentials without a local store, and typed CLI errors. The credential/host-key commit has not landed: a false dependency-gate rejection is being repaired under the owner's standing decision. |
| `internal/le/commit`, `internal/le/deps`; focused dependency-policy regressions | The false-gate repair now reads selected and HEAD module metadata and checks the exact provider plus package containment. Existing focused controls passed in `job-deployment-packaging-regressions-2c6f62f0.log`; the new unrelated-module borrowing control failed there, then passed in `job-deployment-iso-selection-red-2c6594ba.log`. The full commit/deps core-profile run is still running (main session `17839`), and no commit has landed. |
| `internal/install/disk`; `./le --name deployment-user-journeys test unit installer` | Passed after optical media discovery and truthful ISO write finalization changes. |
| `internal/le/test/qemu/install_poweroff_test.go`; `./le --name deployment-user-journeys test unit pkg path ./internal/le/test/qemu/ run '^TestISOCapture'` | Passed clean guest exit, failure after completion text and timeout for a guest that remains running. A fixture lookup error was corrected without changing its assertions. |
| `internal/appliance/iso_diagnostics_test.go`, `internal/appliance/iso_grub_selection_test.go`; host-driver profile `ze_core ze_setup` | Five ISO diagnostic tests passed in `job-deployment-iso-selection-red-2c6594ba.log`. Four named foreign-target complete-builder fallback cases observed RED in that same run; their appliance-name/directory source fix is written, but GREEN remains owed. The Debian foreign-target apt hint is still being corrected. These are scoped host-driver results, not a full-profile pass. |
| `ZE_INSTALL_KERNEL=$PWD/build/kernel/Image ZE_INSTALL_IMAGE=<retained-baseline-image> ZE_INSTALL_QEMU_ACCEL=kvm ZE_INSTALL_KEEP=1 ./le --name deployment-user-journeys test qemu installer-iso`; current `internal/install/disk` and `internal/le/test/qemu` sources | Passed on native amd64/KVM without a firmware override after a fresh initrd build: default OVMF CODE/VARS boot, optical media discovery, ISO-contained image hash equality, explicit selected-disk installation, actual clean poweroff, unchanged untargeted disk, matching installed GPT, installed-disk boot and authenticated SSH version command. Final GRUB module-directory binding controls subsequently passed (`job-unit-pkg-dbe92bee.log`), and main's real ISO assembly passed (session `94951`). The retained source image predates the updater identity fix, so this proves ISO installation, not the pending OTA cycle. |
| `internal/le/build/compile`, `internal/le/interoplab`, `internal/le/test/netlab` package runs | Earlier compile and interoplab runs passed. The full netlab package passed after owned-image-tag retention, Dockerfile path, protocol-state and exclusive-lease corrections (`job-unit-pkg-8e65c965.log`). The actual live topology, route and traffic proof remains owed. |
| `internal/le/build/preview`, `internal/le/workflowcheck` package runs and actual binary-bundle check | The full preview package passed after Docker archive-to-record binding and result-presentation corrections (`job-unit-pkg-ae2b3144.log`). Earlier workflow checks passed. Main rechecked the previously produced real binary bundle successfully (session `47454`). Final Docker image exports and their actual bundle check remain owed. |
| `internal/appliance`, `internal/le/build/gokrazy`, `internal/le/vendorpatch`, actual embedded packer failure regression | The affected appliance and image-build population passed (`job-unit-pkg-a341ffd3.log`), and the pinned upstream vendor checks passed (`job-unit-pkg-66d19b99.log`) after preserving successful privileged-helper completion separately from returned errors. These checks cover the earlier prepared-identity cleanup defect. The real OTA update/recovery cycle remains owed. |
| `internal/le/test/qemu/install_ota_protocol_test.go`; full-runner mock TLS/SSH regressions | Both the public configuration-upload and testboot-recovery controls passed in `job-deployment-packaging-regressions-2c6f62f0.log`. Main previously observed the unknown-config RED (`job-unit-pkg-8787a421.log`) and wrong-slot RED (`job-deployment-ota-recovery-red-f940f8fb.log`). These mock protocol results do not replace the owed real VM OTA cycle. The packaging-regressions job was not globally green: its dependency-policy control failed and its appliance population could not compile. |
| Default-profile tool/runtime builds and `./le doc check all` | Unrelated IKE edits leave `discardUnauthenticatedHandshakeRequest` undefined, observed in `job-unit-pkg-3d38c3a1.log`; earlier unrelated MPLS and runner compile failures were observed in `job-unit-pkg-bfc7b873.log`. Main's document check (session `14089`) failed on that compilation blocker and four unrelated `ai/digests` PACKAGE-MAP links. No default-profile build or global documentation pass is claimed. |

### Historical native runtime follow-up, 2026-11-11

Native installation landed in `2213186915`. Main directly observed fresh
systemd installation from a new prefix, authenticated version and negative
password checks, repeat installation retaining configuration and unit inode,
active-service removal retaining state/account, confined purge, and foreground
installation/removal. That complete fresh-prefix proof passed with the fixed
production candidate. A separate different-version upgrade installed the
selected bytes/version and retained configuration; its combined script later
timed out in the additional fresh foreground bootstrap. The combined upgrade
script is therefore not a complete pass.

The focused root execution of
`TestMaintenanceRootLocksServiceOwnedFolderWithoutDataAccess` passed without a
skip, proving root maintenance of a service-owned folder while its data opener
still refuses root. The post-commit `./le repo compiles check` passed all five
shipped flavors for descendant `7fc9c654ff8d`.

With an explicit private `ZE_SESSION_ID` and a freshly built default test
profile, `test ui --draft native-install-lifecycle deployment-quickstart
deployment-remote-trust` passed remote trust and failed native initialization
and passive-peer readiness at their existing deadlines. Correcting private
test paths did not clear those failures. No timeout increase is proposed.

A separate diagnostic-only strace and Go stack capture found native setup
inside `storage.lockStoreFile`, blocked in its first folder `Sync`. The kernel
reported `jbd2_log_wait_commit` and the `fsync` syscall. This observation locates
one wait; it does not establish that every remaining startup wait has the same
cause.

The scoped correction under investigation is to remove directory durability
from coordination-lock acquisition while retaining atomic create/open,
descriptor checks, stable lock identity and nonblocking exclusion. A host
crash ends the old kernel locks; no data publication depends on preserving
that lock filename beforehand. `ownershipLock` already creates maintenance
locks without this sync. Frame, directory-publication, import-recovery and
ownership-transfer durability barriers must remain unchanged. A Linux
`O_PATH` directory regression will prove that descriptor-relative lock
acquisition requires naming capability without requiring folder flushes.
The old creation-sync count oracle requires independent judgment before any
change. Actual lifecycle and stress results remain owed after the correction.

Docker/Compose, live netlab, real OTA, final Docker bundle exports, website
checks and draft-test completion remain open. Smaller runtime kernels and
unified ISO bundle selection remain research, as scoped below.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Native binary and service installation | `docs/guide/ze-install.md`, Local Installation and Systemd Service | Copies the running binary, initializes state separately, installs a restrictive service | Extends the existing CLI workflow and proves ordinary removal retains state. |
| Patched builds | `docs/contributing/patching-the-standard-library.md`; existing launcher and host-driver build | Selects the standard library required by Ze | Reuses for every supplied source-build path. |
| Container images and Compose | `docker/Dockerfile`, `docker/Dockerfile.lab`, `docker/compose.yaml`; Docker guide | Static deployment image and shell-equipped lab image | Corrects bootstrap/persistence and packages reproducible examples. |
| netlab integration | `contrib/netlab/ze.yml`, templates, reference topology; netlab guide | Renders Ze configurations for containerlab | Extends proof through running nodes, routing state and traffic. |
| Appliance build, ISO and updater | Appliance guide, structured workflow and remote operations | Builds seeded images, installer media and update payloads | Proves complete native-architecture install and OTA/reboot cycle. |
| Website data and native renderer | `home.go` `homeBody`, `docsmanifest.go` `docsManifest`, audience/nav JSON | Shared entry points and generated guide pages | Extends with deployment choice, accurate prerequisites and complete examples. |
| Release delivery contracts | Existing release-build and release-distribution specs | Define versioned artifacts, identity, manifests and trust | Reuses for preview preparation; does not invent an unsigned stable channel. |

## Data Flow

### Entry Point

Users choose a deployment from the website, then run existing Ze install/start commands or the existing appliance builder and installer. Inputs are a selected binary/image, explicit credentials, configuration and target platform.

### Transformation Path

1. Existing build tooling produces identified daemon, lab or appliance artifacts.
2. Native install or the ISO installer places the runtime and initializes its private persistent store.
3. The daemon loads the supplied config, starts authenticated management, establishes a peer and exposes route state.
4. Existing reload/update backends apply changes; reboot and recovery preserve the chosen configuration.
5. Website renderers publish the same source-backed steps and artifact availability.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Operator to Linux service | Existing install/init/systemd commands and private store | Actual fresh-parent systemd lifecycle passed; separate approved tmpfs stress passed |
| Host to container | Existing image entrypoint, mounted state and topology links | Actual Compose authenticated edit/recreation and final owned cleanup passed in `84344`; netlab initial configuration failed on missing node namespaces |
| ISO to installed disk | Existing pure-Go installer, checksum and target selection | Native amd64/KVM selected-disk/extra-disk/installed-SSH proof passed |
| Host to appliance updater | Existing authenticated updater protocol and A/B slots | Actual `19822` passed changed runtime, persistent configuration/login/UUID and previous-slot recovery |
| Source docs to public pages | Existing site manifest and renderers | Final local build `53669` and check `13463` passed after the latest guide change: 24 producers and 1,107 declared/written routes; baseline warnings retained, no remote publication |

### Changed-Wire Census

No protocol, plugin wire format or YANG config format is changed. Source searches of site templates, Dockerfiles/Compose, netlab YAML/templates and deployment guides identify the affected command carriers; known controls are the start entrypoint in both Dockerfiles and `Table`'s interop action. Preserve daemon start/config grammar and update protocol. Any implementation that needs a new command argument records its writer, parser and all guide/shell consumers before changing it.

### Integration Points

Use existing native install/systemd registrations, Docker image entrypoints, netlab daemon declarations, appliance commands, updater backends and site producers. Keep new checks with their owning package.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Design yes | Existing CLI and installer entrypoints remain the user path. |
| No unintended coupling | Design yes | Product does not import le. |
| No duplicated functionality | Design yes | Existing mechanisms table assigns each responsibility. |
| Zero-copy preserved | N-A | No route encoding changes. |
| Registration outbound | Design yes | Any added command uses its owning registry. |
| Registration inbound | Design yes | No core feature-name switches or secondary command catalog. |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Current native command owners can supply the complete lifecycle | Installation guide | Extend the existing command owner rather than add a shell installer | Actual fresh-parent systemd lifecycle and scoped native controls | validated |
| A-2 | Native QEMU resources permit complete ISO and update proof | Existing appliance test entrypoints | Repair prerequisite or test/runtime defect before closure | Actual amd64/KVM ISO and actual OTA/recovery run `19822` passed | validated for the exercised amd64/KVM path |
| A-3 | Public delivery is not authorized yet | Owner asked for implementation, not push/release | Require explicit publication instruction at final delivery | Prepared local artifacts and workflow inspection | validated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Install/removal damages an existing store or unrelated file | Ownership/symlink/conflict regression fails | Preserve existing data; refuse unsafe paths before mutation. |
| R-2 | Container bootstrap and daemon choose different store paths | Recreated container loses login/config | One persistent state mount used by init and start. |
| R-3 | An ISO test proves installer completion but not the installed system | Missing SSH/login/route assertion after disk boot | Boot the written disk and query the actual daemon. |
| R-4 | OTA mock passes while real update fails | Installed system does not return after reboot | Exercise real updater and retain recovery path. |
| R-5 | A source or website update includes another session's work | Unrelated hunks in named files | Preserve concurrent hunks; review exact diff before commit. |
| R-6 | Availability claims outrun publication | A download link names no published artifact | Label local previews and build instructions honestly. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if wrong? | Installation, operator login, persisted config, appliance boot or routing during update. |
| How recovered? | Preserve previous binary/state or previous appliance slot; use existing recovery console. |
| Who else touches it? | Native, appliance and onboarding agents have disjoint phase ownership; main serializes all execution. |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Native Ze install/uninstall CLI | → | Existing local/systemd command owners | Native installation lifecycle functional proof |
| Homepage deployment links | → | `homeBody` and `docsManifest` | `TestDeploymentJourneysReachPublishedGuides` |
| Docker Compose up | → | Existing Docker entrypoint and persistent mount | Compose bootstrap/recreate proof |
| netlab up | → | Existing daemon templates | Reference topology route and ping proof |
| Boot installer ISO | → | Existing ISO builder and disk installer | `installer-iso` installed-target proof |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Fresh supported Linux host and Ze binary | Ze CLI installs binary, initializes protected state and starts a usable service; ordinary uninstall removes owned runtime files and retains state; explicit purge removes only selected Ze state | Existing unmanaged configurations and unrelated files remain, because install/removal owns only declared paths. |
| AC-2 | Existing native installation, path conflicts or package ownership | Upgrade retains state and credentials; unsafe/conflicting or package-managed replacement refuses with an actionable fix | Existing compatible installations remain usable; package manager ownership remains authoritative. |
| AC-3 | Fresh source checkout | Documented build selects the patched toolchain and produces the named executable without requiring an unexplained Makefile, binary or config | Explicit custom feature builds remain possible because feature gates are supported. |
| AC-4 | Local quickstart on a clean Linux host | Supplied commands and two supplied configs establish a peer and demonstrate a received route, then stop cleanly | Real-peer use remains available as a separate guided path; no sink acceptance is counted as route verification. |
| AC-5 | Deployment container first start and recreation | Bootstrap and daemon use one writable persistent store; authenticated operator changes persist after recreation; documented port/capability/network settings match the chosen role | Scratch deployment and Alpine lab images remain separate because lab drivers require a shell and ip. |
| AC-6 | Reference containerlab/netlab topology | Running nodes establish expected adjacencies, hold the declared routes and pass a traffic check; users can tear down the lab | Experimental protocol labels remain because a lab success does not imply production maturity. |
| AC-7 | Native-architecture installer ISO | ISO boots, installs the selected disk, then the written disk boots and accepts authenticated SSH with persistent config and a working daemon | Explicit documented hardware/architecture limits remain; no skipped boot counts as proof. |
| AC-8 | Installed appliance receives a new image | Authenticated OTA completes, the target reboots into the new runtime, retains config/login and exposes healthy state; previous-slot recovery remains usable | No in-place daemon replacement on gokrazy, because updates are A/B images. |
| AC-9 | Website chooser and five deployment interests | Every path leads to a complete tutorial with prerequisites, acquisition, success check, persistence, update/recovery and removal guidance; Docker is published on-site | Gokrazy is explained as the appliance implementation, not a separate incompatible Ze product. |
| AC-10 | No public release currently exists | Local preview artifacts and immutable image identities/manifests are prepared and checked; CI publication path is reviewable; pages accurately describe what users can obtain now | Actual push/publication waits for explicit owner instruction; no unavailable download is presented as live. |
| AC-11 | Production evaluator | Guides distinguish pre-release support from demonstrated capabilities and explain backup, graceful maintenance, update verification, rollback and support | Honest experimental gaps remain visible; the task does not declare the product production-ready. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Installs and removes Ze on Linux | Ze install/init/systemd, management login, uninstall | Native lifecycle functional proof |
| 2 | Tries BGP locally | Supplied peer configs, two daemon starts, route query | Local two-peer tutorial proof |
| 3 | Recreates a container | Init store, start, edit, recreate, query | Compose persistence proof |
| 4 | Boots and updates a router VM | ISO, disk installer, disk boot, login, OTA, reboot | Installed-target ISO and OTA proof |
| 5 | Chooses a deployment online | Site chooser, complete guide, success check | Canonical site build/check and link regression |

## 🧪 TDD Test Plan

| Test | File or route | Validates | Status |
|------|---------------|-----------|--------|
| Native command regression tests | Existing local/systemd test packages | Ownership, removal safety, lifecycle entrypoints | Scoped controls, actual systemd lifecycle, 80-run stress and live CI case passed |
| `TestDeploymentJourneysReachPublishedGuides` | `internal/le/site/` | Useful native/container/lab/appliance destinations are published | Site wiring implemented; scoped site evidence and current local canonical build/check passed |
| Two-peer tutorial proof | Supplied tutorial configs and existing functional runner | Established session and received prefix | Actual route/authentication, 80-run stress and live CI case passed |
| Compose bootstrap/recreate | Existing Docker tooling and Compose | Persistent authenticated state | Actual Compose portion of `84344` printed PASS; source-unbound runtime-image limitation retained |
| Reference topology route/ping | Existing netlab/containerlab tooling | Real protocol and forwarding outcome | Initial-method correction passed RED/GREEN/full units, narrow review and compilation; actual `68552` failed before topology startup on changing foreign IKE code, with successful outer cleanup; live correction remains unverified |
| Installed-target ISO and OTA | Existing native QEMU actions | Booted installed system and update/reboot cycle | Actual amd64/KVM installed-target ISO and OTA/recovery `19822` passed |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Install target and state ownership | Existing safe native path contract | Compatible owned file/store | Symlink, conflicting owner or unsafe path | Outside selected prefix/state root |
| Artifact identity | Exact build/image digest | Exact supplied bytes | Missing identity | Mismatched bytes or architecture |

### Functional Tests

Use existing native install, appliance and installer functional suites; extend with focused lifecycle cases where current coverage lacks the user entrypoint. The main thread runs every build, test, formatter and VM. No full-tree verification is owed.

## Files to Modify

- Existing `internal/plugins/local/`, `internal/plugins/systemd/` and init owners: native lifecycle phase owns required command fixes and tests.
- Existing `internal/appliance/` and native QEMU test owners: appliance phase owns install/update proof and related fixes.
- `docs/guide/quickstart.md`, `docs/guide/ubuntu-build-install.md`, `docs/guide/docker.md`, `docs/guide/netlab.md`: onboarding phase owns complete reproducible instructions.
- `docs/guide/ze-install.md`, `docs/guide/appliance.md`: appliance phase owns image and native install integration with native agent review.
- `docker/compose.yaml`, existing Docker recipes and `contrib/netlab/`: onboarding phase corrects persistence, startup and lab packaging.
- `internal/le/site/homebody.go`, `internal/le/site/home_test.go`, `internal/le/site/docsmanifest.go`, audience/nav JSON, guides index and lab pages: onboarding phase fixes discovery and instructions.

## Files to Create

- Focused tutorial configs and their validation/proof entry, next to existing examples.
- Preview artifact preparation or workflow files only where existing release tooling lacks a reusable entrypoint; record exact owners before implementation.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema and validation | No | Preserve current command/config grammar unless native audit demonstrates a required addition. |
| YANG custom validators | No | No new config type. |
| CLI commands/flags and grammar | Yes | Existing local/systemd owners and registered offline grammar. |
| Editor autocomplete | No | No editor grammar change. |
| Functional test for new RPC/API | No | No new RPC/API; existing local CLI paths receive functional proof. |
| Pipe completeness | Yes | Existing command output renderers remain the path. |
| Env var registration | No | Reuse registered config directory, credentials and build variables. |
| Doctor check for dependencies | Yes | Native/systemd and appliance owners retain existing checks and add missing problem classes with registered diagnostic codes. |
| Doctor code per problem class | Yes | Each new dependency refusal must name its existing or new owner check and phase before implementation. |
| Prometheus metrics | No | No new runtime metric. |
| BGP family surface | No | No new family/capability/attribute. |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | Feature scope/evidence changed | Yes | Existing deployment/readiness feature declarations if proof level changes. |
| 2 | Config syntax changed | No | Existing syntax preserved. |
| 3 | CLI changed | Yes | Installation guide and command reference for native additions. |
| 4 | API/RPC changed | No | No new API contract. |
| 5 | Plugin changed | No | Ownership retained; update plugin guide only if audit finds a changed operator contract. |
| 6 | User guide | Yes | Installation, appliance, Docker, netlab, quickstart and Ubuntu pages. |
| 7 | Wire format | No | Existing formats preserved. |
| 8 | SDK/protocol | No | Existing process protocol preserved. |
| 9 | RFC behavior | No | Demonstrate existing BGP behavior without claiming new RFC conformance. |
| 10 | Test infrastructure | Yes | Existing functional/QEMU and lab docs for any added proof action. |
| 11 | Comparison | No | No competitive capability change. |
| 12 | Architecture | Yes | Existing Design-header pages where implementation changes their contract. |
| 13 | Route metadata | No | No route metadata change. |
| 14 | Metrics | No | No metrics change. |
| 15 | Registry/inventory | Yes | Existing site manifest/nav, command inventory if native grammar changes. |
| 16 | Source anchors | Yes | Main runs spec citation anchors and doc checks; update each affected Design-header page in its phase. |
| 17 | Existing examples | Yes | Verify all affected install/start/build/lab/update examples at their producers. |

## Implementation Steps

1. **Phase: Wiring.** Main records the design judgment and claims this spec; each owner audits its existing entrypoints and writes focused failing regressions before changing code.
2. **Phase: Native Linux lifecycle.** Native agent owns install/uninstall/bootstrap/systemd and scoped tests; main runs native proof and coordinates installation-guide examples.
3. **Phase: Appliance installation and update.** Appliance agent owns ISO/build/OTA repairs and proof changes; main boots ISO, installed disk and updated image, and records exact evidence.
4. **Phase: Container, lab and website onboarding.** Onboarding agent owns two-peer tutorial, Docker/Compose, netlab packaging and site entrypoints; main runs the tutorial/lab/container proofs and canonical site build/check.
5. **Phase: Preview delivery.** Main assigns an owner after inspecting release-build/distribution mechanisms; prepare identified artifacts, pinned images and reviewable CI delivery, without publishing. This phase consumes all earlier build/proof results.

## Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Native lifecycle | Ordinary uninstall preserves private state; purge cannot reach another installation. |
| Tutorial | Starts from supplied files and ends with a received route; no undeclared helper executable. |
| Container | Init and start share one mounted state directory; recreation retains operator changes. |
| Lab | Real route and traffic assertion, not render-only acceptance. |
| Appliance | Proof boots the installed disk and completes an actual OTA/reboot. |
| Public claims | Every artifact availability/maturity statement matches observed publication and proof. |
| Website | Existing information remains available and the canonical generated artifact passes its checks. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Easy native lifecycle | Scoped local/systemd tests and native command proof |
| Reproducible local/container/lab examples | Main-thread route/persistence/traffic proofs |
| Working ISO and OTA cycle | Native QEMU installed-target and reboot evidence |
| Accurate deployment chooser and guides | Site package tests, canonical site build/check and doc checks |
| Preview delivery ready to publish | Artifact manifest/digest inspection and prepared workflow validation |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Installation | Symlink and package ownership conflicts; state permissions; unrelated file preservation. |
| Bootstrap | No fixed production passwords, secret argv or readable credential artifacts. |
| Containers | Required privileges only; persistent state and explicit management bindings. |
| Images and updates | Architecture, digests, authenticated transport and recovery remain enforced. |
| Documentation | Lab credentials are explicitly lab-only; production readiness is not overstated. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Build/test/boot fails | Owning phase fixes producer; main reruns exact failing scope. |
| Shared checkout interferes | Preserve concurrent work; coordinate through main. |
| External publication required | Prepare reviewable artifacts and report explicit owner publication step. |

## Mechanical Discovery

| Need | Existing surface | Proof |
|------|------------------|-------|
| Find deployment help | Site nav, guides index, AI index deployment rows | Published guide/link regression and site check |
| Prevent stale command examples | Existing command/action registrations and scoped regression | Command path and tutorial proof |
| Keep feature/build inventory current | Feature-gate and release declarations | Existing feature-tag check and artifact identity |

## Owned Journal Rows

No existing journal row is claimed by this specification at creation. The reviewed deployment findings are the Task and ACs above.

## Review Gate

### Current independent source and documentation reviews, 2026-11-12

These are bounded source-review results reported by the main thread. They do
not convert an outstanding runtime or artifact requirement into a pass; the
dated execution history and failed runs above remain evidence.

| Reviewer / scope | Producers inspected | Result and limit |
|------------------|---------------------|------------------|
| `deployment_final_review`: native trust and purge | `nativeRuntime.install`, `nativeRuntime.uninstall`, `Preflight`, `AcquireMaintenance` | No additional source issues found in the selected ownership, refusal, maintenance and cleanup paths. |
| `deployment_final_review`: OTA authentication, partitions and prepared-secret cleanup | `authTransport.RoundTrip`, `doPushUpdater`, `readPushPartitions`, `Prepare` | No additional source issues found; actual OTA/recovery evidence remains the separate passing `19822` run. |
| `deployment_final_review`: preview provenance, checksums and archive binding | `prepare`, `captureSource`, `checkDockerArchive` | No additional source issues found; a complete current source-bound bundle still must be prepared and checked. |
| `deployment_final_review`: netlab ownership and image binding | `acquireLiveDirectory`, `bindNodeImage`, `runContainerLab` | No additional source issues found in leases/image identity; known actual initial-method failure was not treated as a clean runtime gate. |
| `deployment_final_review`: minimal initial-method correction `b914eb1a0b` | `contrib/netlab/ze.yml`, production buildLab/export regression, guide override explanation | Narrow review clear, tests strengthened and no existing assertions weakened. Actual fixed runtime proof remains pending. |
| `deployment_site_scope`: final owned documentation and acquisition links | Report `tmp/session/2026-11-10-225434/scratch/deployment-doc-final-review.md` | No related substantive discrepancy found across native, quickstart, Docker, appliance, netlab/site acquisition and preview carriers. Source review only; main separately passed final post-guide canonical site build `53669` and check `13463`, with 24 producers and 1,107 declared/written routes. |
| `deployment_final_gap_review`: remaining requirements across completed paths | `executeApplianceOTA`, `otaGuest.await`, `otaGuest.assertHost`, `docsRenderer.renderSource` and the appliance guide | Two blockers: AC-8 lacks a health assertion after update and recovery; AC-9 lacks appliance decommissioning guidance. Both fixes are in progress. The known netlab and complete preview proof gaps remain open. |
| `ota_health_review`: extended health proof and original protocol assertions | `executeApplianceOTA`, `otaGuest.assertHealth`, `validateOTAHealth`, `otaInactiveHealth`, `otaProtocolFixture.run` and health command producers | No actionable findings. Health checks reject unexpected failures and permit only exact inactive outcomes for protocols absent from the parsed seed. Existing hostname probes remain independently required. Actual extended VM proof remains owed. |

### Fixes and scoped reruns

The actual netlab initial-configuration failure in `84344` led to the minimal
explicit shell-method correction in `b914eb1a0b`. Main observed semantic RED
`2438`, focused GREEN `49686`, full netlab-package GREEN `61909` and committed
all-five-flavor PASS `67612` at HEAD `b914eb1a0b0f`. Upstream method mapping
confirmed the routing daemon already overrides inherited namespace mode with
`ignore`; no additional routing override was required. These controls do not
substitute for the running actual own-kernel rerun.

The OTA health extension separately failed for missing health queries (`63291`),
configured protocols incorrectly receiving inactive exceptions (`60575`), and
transient readiness being refused immediately (`77420`). Focused health GREEN
`67333` and full verbose QEMU-package race run `31325` then passed, including
every original OTA test. The fixture judge personally recorded approvals after
formatting; extra health logins cannot satisfy the six original hostname-query
requirements. No new actual health-bearing VM PASS has yet occurred.

### Remaining acceptance evidence

| Requirement | Current state | Required result |
|-------------|---------------|-----------------|
| AC-6 live topology | Actual `53628` reached running nodes and initial configuration; missing YAML inventory was then corrected and passed scoped tests and actual create. | Actual declared adjacencies, kernel routes, sourced traffic and successful owned teardown through the corrected invocation. |
| AC-8 post-update health | Extended runner and health regressions passed scoped/full package tests and independent reviews, preserving all original assertions. | Pass the extended actual VM proof after both updated and recovered boots. |
| AC-9 appliance removal | Canonical guide now covers live backup, separate appliance export, graceful shutdown, VM disk retention/deletion, hardware retirement and retained secrets; independent review was clear. | Regenerate and inspect the canonical website output for the updated guide. |
| AC-10 current source-bound preview | Fresh complete preparation `52283` and retained checker `27968` passed with all identity checks retained. | Preserve the private bundle and its exact scope; no publication is authorized. |
| Final closure review | Bounded source/doc reviews above are clear; required runtime/artifact evidence remains open. | Reconcile actual results before declaring a final clean gate or overall completion. |

### Final status

- [ ] Independent review shows zero blockers and zero issues.
- [ ] Notes recorded or explicitly none.

These final closure boxes remain unchecked while live netlab and extended OTA
health proofs remain open. The complete source-bound preview and its retained
checker passed. Canonical site regeneration for the latest guidance remains
required. No feature-proof refresh or remote publication is authorized.

## Build and kernel research, 2026-11-11

Owner request, 2026-11-11:

> could we have easier way to build and deliver iso, binary, etc. also the kernl options we need to QEMU should be much smaller than a real kernel which needs more driver support. Can you please consider how we could achieve this in an agent ?

This request adds research while the existing deployment fixes and proof runs continue. It does not change the acceptance criteria or mark the kernel design approved.

| Finding | Source and consequence |
|---------|------------------------|
| One bundle producer already exists | `internal/le/build/preview/preview.go` `prepare` owns binary archives, optional Docker exports and their manifest. Extend its artifact selection and checker; call existing appliance producers for disk and ISO outputs. |
| Installer profile and runtime profile are separate | `internal/appliance/cmd_kernel.go` `runKernel` applies `image.kernel-profile` only to the installer. `internal/appliance/runtimekernel.go` `RuntimeKernelTree` always selects `runtime`; `internal/appliance/kernelargs.go` `resolveBuildParentDir` passes architecture alone. A VM runtime needs explicit profile selection. |
| Removing fragment entries alone cannot make a minimal kernel | `internal/appliance/kernelbuilder/worker.go` `RunWorker` starts from `defconfig`; `copyRuntimeOutputs` also packages modules. A QEMU variant needs explicit exclusions checked against the emitted config, or a separately validated minimal seed. |
| Existing resolution and cache machinery can carry the split | `internal/appliance/cmd_kernel.go` `resolveKernel` accepts purpose and profile. `internal/appliance/cache.go` `kernelCacheVariantFor` already keys purpose, architecture, profile, backend and input hashes. Extend these paths and image provenance instead of adding another builder. |
| A private installed image is not a public evaluation image | `internal/appliance/cmd_build.go` `runGokBuild` prepares the named appliance and `injectZeFS` injects its database. Keep these bundles private until a credential-free image has a per-device provisioning path. Existing release signing, source and publication obligations still apply. |

Recommended design for review: extend `build preview run` with an appliance and a format selector that can prepare binary, Docker, disk and ISO artifacts in one request. Add `image.runtime-kernel-profile` with explicit `qemu` and `hardware` variants while preserving the existing installer selector. Reuse common networking fragments and their built-in requirements for both variants. Give QEMU only its tested virtual boot, storage and NIC drivers; keep physical NIC, storage, console, sensor and firmware support in hardware fragments. Preserve hardware fan-control coverage when separating the current compiled runtime floor.

First use `defconfig` plus explicit QEMU exclusions and emitted-config checks. Compare its kernel bytes, module-tree bytes, compressed image bytes and bounded build costs with the current profile. Consider `allnoconfig` plus complete architecture/boot/capability fragments only if the first approach misses an agreed size target. Neither size nor speed improvement has been measured by this research.

Verification must cover each native architecture, cold and warm profile caches, ISO optical/UEFI boot and installed-disk boot, A/B updates, the unchanged networking capabilities, and Docker-host kernel-byte/PID-1/root-filesystem identity. Detailed producer evidence, alternatives and implementation order are in `tmp/session/2026-11-10-225434/state/session-state-deployment-user-journeys-build-kernel.md`.
