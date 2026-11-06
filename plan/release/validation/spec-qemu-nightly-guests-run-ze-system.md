# Spec: qemu-nightly-guests-run-ze-system

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | Five nightly labs prove behavior on Alpine, which is not the system Ze ships or the owner's rule for test guests |
| RFC gaps | - |
| Depends | `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` (the Ze-image guest these steps move to) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-06 |

Scope is tooling: no shipped behavior changes. The End-to-End section is deleted
for that reason, and no wire changes.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`.github/workflows/qemu-nightly.yml` still runs five steps through
`./le test qemu run`, which boots Alpine userland on Ze's kernel:

| Job | Step | Tools it installs | Command |
|-----|------|-------------------|---------|
| `protocol-labs` | LDP against FRR | `frr iproute2 kmod` | `go test ... TestLDPInteropFRR` |
| `protocol-labs` | IS-IS against FRR | `frr iproute2 kmod tcpdump` | `go test ... TestISISInteropFRR` |
| `protocol-labs` | VRRP against keepalived | `keepalived tcpdump iproute2 iputils-ping kmod` | `le test qemu vrrp-keepalived` |
| `runtime-kernel-labs` | PPPoE against accel-ppp | `accel-ppp ppp iproute2 iputils-ping kmod` | `le test qemu pppoe-accel` |
| `runtime-kernel-labs` | Traffic-usage eBPF proof | `iproute2 kmod` | `go test ... TestProgram_ ...` |

The owner's rule: "VMs, test guests and the lab Docker host run Ze's own kernel
and system, never Alpine or a stock distribution kernel." The evidence these
steps give is a Linux kernel with Alpine's userland, so
`docs/architecture/testing/qemu-integration.md` already says the route "does not
establish own-system proof".

Ze's docker-host image lacks what the steps call: zebra, ldpd and go for LDP;
zebra, isisd, tcpdump and go for IS-IS; keepalived and tcpdump for VRRP;
accel-pppd for PPPoE; go for eBPF. Busybox's `ping` is enough where the labs call
`ping`: the image's `busybox.config` sets `CONFIG_PING` and
`CONFIG_FEATURE_FANCY_PING`, which carry `-c` and `-W`, and `harness` runs as root.

A second defect on the same route: a plain `docker-lab command` holds no host
admission slot, while its `harness` branch does.

A third, on the guest's population: `86bc403a4e` added integration tests under
`internal/test/pppoeac`, and no runner names the package, so they run nowhere.

Goal: every nightly lab boots Ze's image, the proof each step gives survives
whole, every guest boot holds a slot, and every integration package runs.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/qemu-integration.md` "One command in Ze's own system", "Docker labs on the docker-host image"
  → Decision: `docker-lab` has two branches. `lab` runs le on the host against the guest's Docker daemon; `command` runs a command in the guest without Docker.
  → Constraint: the page states "A plain `docker-lab` command holds none" as today's behavior, so the slot fix edits that sentence.
- [ ] `docs/architecture/testing/ci-workflows.md` - the job table
  → Decision: `evidence-nightly.yml` already runs `./le test integration interop` and `./le test deployment docker-pppoe-accel` inside `docker-lab lab`, on Ze's image and kernel.
  → Constraint: the job table rows for `protocol-labs` and `runtime-kernel-labs` change with this spec.
- [ ] `docs/architecture/appliance/docker-host.md` - what the image holds
  → Constraint: `gokrazy/docker-host/binaries.json` pins what the image builds from source. Each addition is a pin to bump and a build to keep working.
- [ ] `docs/architecture/testing/interop.md` - scenarios and vacuity traps
  → Constraint: revert, rebuild, see red, restore, see green, for each scenario that takes a step's proof.

**Key insights:**
- Most of the missing tools are peers, and the interop labs already run those peers in containers on Ze's kernel. A lab-to-lab move adds nothing to the image.
- Go is already solved for the guest. `needs-linux` copies the patched tree to `tmp/qemu/_guest-go`, because "Ze's image carries no Go".

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `.github/workflows/qemu-nightly.yml` - the five steps above, and the `harness` steps that already boot Ze's image. Its comment says the Alpine neighbours "still need tools it does not carry"
- [ ] `.github/workflows/evidence-nightly.yml` - job `pppoe-interop` runs `docker-lab lab "test deployment docker-pppoe-accel"`
- [ ] `internal/le/test/qemu/dockerlab.go` - the command branch: with no `harness`, it calls `runDockerLabCommand` directly, and with `harness` it goes through `admitQEMURun`
- [ ] `internal/le/test/qemu/dockerlab_command.go` - `runDockerLabCommand`, which ensures the image and boots QEMU
- [ ] `internal/le/test/qemu/run_admit.go` - `admitQEMURun`, the host's job registry, label `qemu-run`
- [ ] `internal/le/test/qemu/vrrp_keepalived_linux.go` - `runVRRPGuest` requires `ip`, `ping`, `tcpdump`, `keepalived` on the guest
- [ ] `internal/le/test/qemu/pppoe_accel_linux.go` - `runPPPoEAccelGuest` requires `ip`, `ping`, `accel-pppd` on the guest
- [ ] `internal/appliance/dockerhost/guest_linux.go` - `shippedLinks`: sudo, sshd, ip, nft, setcap, getcap, docker, iptables; busybox applets otherwise
- [ ] `internal/plugins/ldp/frr_interop_integration_linux_test.go` - starts zebra and ldpd in a namespace, and `t.Skipf` when FRR is absent. It proves adjacency, operational session, Label Mapping decoded
- [ ] `internal/plugins/isis/frr_interop_integration_linux_test.go` - starts zebra and isisd, proves adjacency Up and FRR's LSP stored. Its header says `./le test qemu all` provides FRR, which Ze's image does not
- [ ] `internal/le/workflowcheck/qemu_nightly_own_system_test.go` - pins `needs-linux` and the two namespace steps to `docker-lab harness`
- [ ] `internal/le/test/qemu/alltests.go` - `integrationPackages`, which names `./internal/component/l2tp/pppoeclient` and not `./internal/test/pppoeac`
- [ ] `internal/le/test/qemu/integration_coverage_test.go` - `TestEveryIntegrationPackageIsNamed` derives the population from the tree, and reports the package as named by no runner

**Behavior to preserve:**
- What each step proves: LDP Hello, session and Label Mapping against FRR; IS-IS adjacency and LSP sync against FRR; VRRP against keepalived; PPPoE against accel-ppp; the eBPF program, attach and metrics.
- The peer's version rule: third-party lab software runs its latest stable upstream release (`ai/META-DECISIONS.md`).
- A `harness` guest's stand-in `ZE_RUN_JOB`, which keeps a guest le from admitting itself.
- The steps stay advisory and scheduled.

**Behavior to change:**
- The five steps stop calling `./le test qemu run`.
- A plain `docker-lab command` holds a host slot.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `docker-lab lab` | `internal/le/test/qemu/dockerlab.go`, `ci-workflows.md` | Boots Ze's image, forwards its Docker socket, runs a lab | Extends: LDP, IS-IS, VRRP and PPPoE take this route, with their peer in a container |
| Interop scenarios | `test/interop/scenarios/` (`ldp-keepalive-expiry-frr`, `ldp-init-unknown-tlv-frr`, `isis-*-frr`, `vrrp-mastership-keepalived`, `vrrp-v2-owner-keepalived`), `test/interop-pppoe/scenarios/` | Prove Ze against FRR, keepalived and accel-ppp, on Ze's kernel | Extends: an assertion a Go test or `le test qemu` lab holds and no scenario holds moves into a scenario |
| `docker-lab harness` | `dockerlab_command.go`, `dockerLabHarnessCommand` | Builds the guest le, binds the checkout, runs as root | Extends: the eBPF step |
| Guest Go tree | `qemu-nightly.yml`, "Copy the patched Go tree into the checkout the guest mounts" | Gives the guest a compiler without one in the image | Extends: the eBPF step uses the same copy. Go is not added to the image |
| Host admission | `run_admit.go` `admitQEMURun` | Holds a slot for `test qemu run` and for `harness` | Extends: covers a plain command too |
| Workflow pin | `qemu_nightly_own_system_test.go` | Pins two jobs' steps to Ze's image | Extends: one test reads every step of the file, and replaces the need for a per-step list |
| Image contents | `gokrazy/docker-host/binaries.json`, `shippedLinks` | Pins what the image builds | Does not apply: no peer is added, and a peer in the image would need its own static build and pin beside the lab that already has one |
| Alpine route | `internal/le/test/qemu/run.go`, `run_iso.go` | Boots Alpine | Does not apply here: `plan/post-release/engineering/spec-test-sandbox.md` AC-7 deletes it, and other callers remain (`mobike`, `guestCIArgv`). This spec removes the nightly callers |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The nightly schedule runs a workflow step, which runs `./le test qemu docker-lab`.
- A person or agent runs a plain `./le test qemu docker-lab kernel <vmlinuz> command <words>`.

### Transformation Path
1. A lab step: `docker-lab lab "<le words>"` boots the image, and the host le runs `./le test integration interop` with the named scenario through the forwarded socket.
2. The eBPF step: `docker-lab harness` runs `go test` from the guest Go tree, in the guest, as root.
3. Every command run first takes a host slot through `admitQEMURun`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Host to guest Docker | Forwarded socket (`docker-lab lab`) | Yes: `evidence-nightly.yml` runs it today |
| Host to guest command | ssh, `harness` | Yes: `qemu-nightly.yml` runs it today |
| Host job registry | `job.NewIn`, `AdmitContext` | Yes: `run_admit.go` |

### Changed-Wire Census

No wire changes. The move is between test routes, and no protocol, event or file format changes. The `INTEROP_SCENARIO` selector and the `le` keywords stay as they are.

### Integration Points
- `dockerLabParameters` in `actions.go` - the keywords stay; only the admission changes.
- `TestEveryIntegrationPackageIsNamed` (`internal/le/test/qemu/integration_coverage_test.go`) - stays green while the Go tests live.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Steps use the existing `docker-lab` branches |
| No unintended coupling (components stay isolated) | Yes | No product code changes |
| No duplicated functionality (extends existing, does not recreate) | Yes | Ends a duplicate: `pppoe-accel` under Alpine beside `docker-pppoe-accel` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | No new command or registration |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | N-A | The feature adds no name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The scenarios can hold what the five steps prove | The scenario lists above, and the Go tests' own headers | A step's proof has no home | Phase 2's table, assertion by assertion, and `ze-test-judge` | unvalidated |
| A-2 | The eBPF proof needs nothing else from the image | The step installs only `iproute2 kmod`; `shippedLinks` has `ip`; Ze's kernel carries its options | A missing tool refuses in the guest | The step's first run | unvalidated |
| A-3 | Busybox `ping` answers as iputils does for `-c N -W N` | `busybox.config` `CONFIG_FEATURE_FANCY_PING=y` | Only relevant if a guest lab keeps calling `ping` | `./le test qemu docker-lab` with a one-line ping, once | unvalidated |
| A-4 | A plain command can hold a slot with no guest stand-in | The stand-in exists for a guest `le`, and a plain command is unprivileged and starts none | A plain command that does start le is refused or nested | A row in `dockerlab_command_test.go` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Retiring a Go test drops an assertion no scenario holds | The judge refuses | The assertion moves first, and the refusal stands |
| R-2 | The Go interop tests stay in the package and `t.Skipf` on the image, reading as green | `test qemu all` shows them skipped | The tests go once their assertions moved; a skip is never counted as the proof |
| R-3 | `spec-docker-hosts-run-the-ze-kernel` changes the guest under this work | Boot failures on the new route | It is the dependency, and its lab jobs are the same route |
| R-4 | The workflow file carries another session's uncommitted hunks | `git diff` on the workflow | Commit only this spec's hunks (`hunks <path> <patch-file>`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A nightly lab goes red or proves less. Nothing shipped changes |
| How is it reverted? | Revert the commit per step; the workflow is advisory |
| Who else touches this path? | `spec-docker-hosts-run-the-ze-kernel`, `plan/post-release/engineering/spec-test-sandbox.md`, `plan/release/validation/spec-vrrp-deferred-keepalived-interop-scenarios.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A plain `docker-lab command` | → | `admitQEMURun`, `runDockerLabCommand` | `TestDockerLabPlainCommandHoldsSlot` |
| The nightly workflow file | → | Each step's route | `TestQEMUNightlyBootsOnlyZeSystem` |
| `./le test qemu docker-lab lab "test integration interop"` | → | Scenario selection | `./le test integration interop` scenarios `ldp-session-labels-frr`, `isis-p2p-frr`, `vrrp-mastership-keepalived` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `qemu-nightly.yml` is read | Every step that boots a guest uses `docker-lab`, and none uses `test qemu run` | Other callers of `test qemu run` stay until `spec-test-sandbox` AC-7: `mobike`, `guestCIArgv` |
| AC-2 | LDP in the nightly | A scenario on Ze's image asserts Hello accepted both ways, the session operational, and a Label Mapping decoded from FRR | None: this is what `TestLDPInteropFRR` proves |
| AC-3 | IS-IS in the nightly | A scenario on Ze's image asserts the adjacency Up on a LAN circuit and FRR's LSP in Ze's LSDB | None: this is what `TestISISInteropFRR` proves |
| AC-4 | VRRP in the nightly | A scenario on Ze's image holds every assertion `le test qemu vrrp-keepalived` makes, the capture and the dataplane ping included | None |
| AC-5 | PPPoE against accel-ppp in the nightly | `docker-pppoe-accel` holds what `pppoe-accel` asserted, the dataplane ping and clean teardown included | None |
| AC-6 | eBPF in the nightly | The same `go test` selection runs in Ze's image under `harness`, from the guest Go tree | None |
| AC-7 | A plain `docker-lab command` runs while the machine is at its limit | It waits for a slot like every other guest boot, and reports what it waits on | Inside a parent that holds the slot (`le job run`), it runs at once |
| AC-8 | A Go interop test no step runs any more | It is deleted once its assertions moved, under `ze-test-judge` | A test that still runs somewhere stays |
| AC-9 | Ze's image is read | No FRR, keepalived, accel-ppp, tcpdump or Go was added to it | Their absence is the point: peers live in lab containers |
| AC-10 | `test qemu all` runs in the guest | The `internal/test/pppoeac` integration tests execute and report, and `TestEveryIntegrationPackageIsNamed` no longer lists the package | None: the fix is a named package, not an exclusion |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDockerLabPlainCommandHoldsSlot` | `internal/le/test/qemu/dockerlab_command_test.go` | AC-7 | |
| `TestDockerLabPlainCommandInsideParentRunsAtOnce` | same | AC-7, the survivor | |
| `TestQEMUNightlyBootsOnlyZeSystem` | `internal/le/workflowcheck/qemu_nightly_own_system_test.go` | AC-1, reading every step of the workflow | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| None | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `docker-lab command` plain and `harness` | `internal/le/test/qemu/dockerlab_command_integration_test.go` | Both boot, each under one slot | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ldp-session-labels-frr` | `test/interop/scenarios/` | FRR | AC-2, new unless phase 2 finds a scenario that holds it. Read `ldpd` in FRR at the lab's version before asserting a label | |
| `isis-p2p-frr` and siblings | `test/interop/scenarios/` | FRR | AC-3 | |
| `vrrp-mastership-keepalived` | `test/interop/scenarios/` | keepalived | AC-4 | |
| `docker-pppoe-accel` | `test/interop-pppoe/` | accel-ppp | AC-5 | |

## Files to Modify
- `.github/workflows/qemu-nightly.yml` - the five steps, and the comment that says the neighbours still need Alpine
- `internal/le/test/qemu/dockerlab.go` - a plain command goes through `admitQEMURun`
- `internal/le/test/qemu/run_admit.go` - its comment names the branches
- `internal/le/test/qemu/alltests.go` - add `./internal/test/pppoeac` to `integrationPackages`
- `internal/le/workflowcheck/qemu_nightly_own_system_test.go` - the whole-file check
- `internal/plugins/ldp/frr_interop_integration_linux_test.go` - removed under `ze-test-judge` after the move
- `internal/plugins/isis/frr_interop_integration_linux_test.go` - removed under `ze-test-judge` after the move
- `internal/le/test/qemu/vrrp_keepalived_linux.go`, `internal/le/test/qemu/pppoe_accel_linux.go` - removed with their actions after the move, if nothing else calls them
- `docs/architecture/testing/ci-workflows.md` - the job table
- `docs/architecture/testing/qemu-integration.md` - the slot sentence, and what the Alpine route still serves

## Files to Create
- `test/interop/scenarios/ldp-session-labels-frr/ze.conf` - if phase 2 finds no scenario that holds AC-2

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Tooling |
| YANG validation constraints | No | Tooling |
| YANG custom validators | No | Tooling |
| CLI commands/flags | No | The keywords stay |
| CLI grammar (keyword before value) | No | No new word |
| Editor autocomplete | No | None |
| Functional test for new RPC/API | No | None |
| Pipe completeness | No | None |
| Env var registration | No | None |
| Doctor check for runtime dependencies | No | Tooling |
| Prometheus counters/metrics | No | Tooling |
| BGP family surface (new SAFI / capability / attribute) | No | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | The ledger credits a lab to its workflow: `rfc/discrimination/` records that name a moved carrier are re-recorded |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-workflows.md`, `docs/architecture/testing/qemu-integration.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/ldp/mpls-ldp.md`, `docs/architecture/isis/isis-13-cli-diag-interop.md`: they name the Go tests. Others: `./le spec citation anchors spec plan/release/validation/spec-qemu-nightly-guests-run-ze-system.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/testing/qemu-integration.md`, its `test qemu run` examples |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the slot, the package name and the workflow check
   - Tests: `TestDockerLabPlainCommandHoldsSlot`, `TestDockerLabPlainCommandInsideParentRunsAtOnce`, `TestQEMUNightlyBootsOnlyZeSystem`, `TestEveryIntegrationPackageIsNamed`
   - Files: `dockerlab.go`, `run_admit.go`, `alltests.go`, `qemu_nightly_own_system_test.go`
   - Verify: the workflow check is red for five steps; the package runs once in the guest (AC-10). The one-line package edit may land first, alone
2. **Phase: Proof table** -- read each step's assertions and find each one's scenario, then write the table into this spec. No test changes yet.
   - Tests: none
   - Files: this spec
   - Verify: every assertion has a scenario or a named gap
3. **Phase: Move the proof** -- add each missing assertion to its scenario
   - Tests: the scenarios above, each with its revert-and-red walk
   - Files: `test/interop/scenarios/`, `internal/le/interoplab/`
   - Verify: red on the reverted behavior, green restored, recorded
4. **Phase: Move the steps** -- LDP, IS-IS, VRRP, PPPoE to `docker-lab lab`; eBPF to `harness` with the guest Go tree
   - Tests: `TestQEMUNightlyBootsOnlyZeSystem` goes green; a dispatched run of `qemu-nightly.yml` cited
   - Files: `.github/workflows/qemu-nightly.yml`
   - Verify: the run's log shows Ze's image booted for each step
5. **Phase: Retire** -- Go interop tests and `le test qemu` lab actions no step calls, under `ze-test-judge`
   - Tests: each removal has its approval
   - Files: the listed test and lab files
   - Verify: `TestEveryIntegrationPackageIsNamed` stays green
6. **Phase: Pages** -- `ci-workflows.md`, `qemu-integration.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Each moved step's assertion list matches the proof table, none dropped |
| Correctness | A `harness` run still holds exactly one slot |
| Naming | A new scenario is named, with no numeric prefix (`ai/rules/interop-and-goal-validation.md`) |
| Rule: test changes | Every removal or narrowing has a `ze-test-judge` approval and a commit line |
| Rule: never weaken | A skip on the image is not counted as a pass |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No Alpine step in the nightly | `TestQEMUNightlyBootsOnlyZeSystem` |
| Slot on a plain command | `./le job run label qemu-unit command go test ./internal/le/test/qemu/ ./internal/le/workflowcheck/` |
| Scenarios green | `./le test integration interop` with each scenario |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | None new; the typed words re-execute as today (`dockerLabArgv`) |

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

- The brief listed the tools missing from the image, which is accurate, and that list reads as a shopping list. The code points the other way: four of the five steps have a lab route that runs the same peer on Ze's kernel, and the fifth needs only the guest Go tree `needs-linux` already builds. So nothing is added to the image.
- The slot gap is documented as behavior, so the page changes with the code.
- Both Go interop tests `t.Skipf` when FRR is absent, so on Ze's image they read as a pass and prove nothing. That is why they go, rather than stay beside the scenarios.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `gate-verdict-depends-on-the-machine.md` | 2026-11-06 | `qemu-nightly.yml` steps still on Alpine `test qemu run` |
| `gate-excludes-part-of-its-population.md` | 2026-11-06 | plain `docker-lab command` holds no host slot |
| `gate-excludes-part-of-its-population.md` | 2026-11-06 | `internal/test/pppoeac` integration tests run nowhere |
| `check-probes-a-peer-its-own-teardown-stopped.md` | 2026-11-06 | `TestInjectionConsumerCancellationJoinsDescendants` fails on its own teardown race |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Peers stay in lab containers | Build FRR, keepalived, accel-ppp and tcpdump into the image | The lab already pins and builds them, and a second static build in the image is a second version of each to keep current |
| Go stays out of the image | Add Go to the image | `needs-linux` shows the guest Go tree works, and the image is a Docker host |
| Proof moves before a step moves | Move the steps, then look | A step moved first loses its proof for as long as the gap lasts |
| Slot on every command | Document that a plain command holds none | Every guest boot uses a VM's worth of CPU, which is what the slot counts |

## Known Limitations
- `./le test qemu run` itself remains for its other callers. `plan/post-release/engineering/spec-test-sandbox.md` AC-7 deletes it.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass, run in the main checkout: `./le test integration interop` for the named scenarios and the scoped package tests. GitHub CI verifies the pushed commit
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script
