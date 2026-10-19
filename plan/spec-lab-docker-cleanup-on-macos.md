# Spec: lab-docker-cleanup-on-macos

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`plan/spec-lab-docker-cleanup-and-go-test-containers.md` (the parent) added
owner labels to every Docker resource a Ze lab run creates, a sweep that
removes only the resources whose owner is provably gone
(`./le test integration cleanup`), and signal roots so that Ctrl-C runs the
teardown. The parent proved this on a Linux host. It could not prove it on
macOS, because only the owner's Mac can run those checks.

Owner decision, 2026-10-19 ("2 yes"): the Mac checks move out of the parent
into this spec. The owner runs every check here on his Mac. An agent only
prepares the commands and records the results. An agent never measures perf
on another host.

| ID | Goal |
|----|------|
| G-1 | The parent's assumptions A-2, A-3 and A-5 are confirmed or broken on a Mac: the owner labels, the owner identity and the cleanup verdicts work against Colima, against Docker Desktop, and inside the QEMU guest's dockerd that runs the labs on macOS |
| G-2 | `./le perf run dut ze`, stopped with Ctrl-C on the Mac, leaves nothing of its run behind |
| G-3 | The FreeRtr DUT image builds on the Mac through the wrapper's `docker build` with `--build-context` |

Nothing else is in scope. This spec adds no feature. A check that fails is a
defect in the parent's implemented capability, and it is fixed here (Failure
Routing).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - "Crash-safe teardown" (the label table and the "gone" paths)
  → Constraint: on macOS `ze.lab.machine-id` is the IOPlatformUUID, `ze.lab.boot-id` is `kern.bootsessionuuid`, `ze.lab.os` is `darwin`, and `ze.lab.pid-ns` is absent. A Mac resource that carries `ze.lab.pid-ns` is a defect.
  → Constraint: every owner fact is read on the client, never from the daemon. On a Mac the daemon runs in a VM whose pids are not the Mac's.
  → Constraint: "another boot id with the owner started before this machine booted" is judged with `kern.boottime` on macOS. AC-3 exercises it.
- [ ] `docs/architecture/testing/qemu-integration.md` - "Docker labs in the Ze-kernel guest"
  → Constraint: on macOS the labs run inside a QEMU guest booted on Ze's runtime kernel, with Alpine's dockerd, through `./le test qemu docker-lab lab "<lab verb>"`. The action needs HVF and refuses before boot when the runtime kernel cache entry is missing, naming the build that writes it.
  → Decision: the guest le is a linux binary, so a resource created inside the guest carries Linux owner facts (with `ze.lab.pid-ns`). AC-5 checks the guest's dockerd, not the Mac's identity readers.
- [ ] `docs/guide/benchmarking.md` - "Automated Docker Runner"
  → Constraint: Ctrl-C or SIGTERM stops `./le perf run`, which removes the DUT, the sender and the network, then exits 1. The DUT images under their cache tags are kept by design.
  → Constraint: the Ze DUT runs on the daemon's kernel, and the run refuses a kernel that lacks a feature Ze enrolls. AC-6 needs a daemon that passes that check (R-3).
- [ ] `plan/spec-lab-docker-cleanup-and-go-test-containers.md` - Assumptions A-2, A-3, A-5; Goal Validation G-2; Work Not Done
  → Decision: the assumption wording below is the parent's, copied verbatim, so a verdict here is a verdict on the parent's row.

**Key insights:**
- The darwin identity readers (`readDarwinFacts` in `internal/le/interoplab/orphans.go`) run `ioreg -rd1 -c IOPlatformExpertDevice` and `sysctl -n kern.bootsessionuuid`, and lowercase the UUID (`normalizedID`). Only unit tests with injected output cover them today.
- `TestKilledLabRunIsCleanedByNextStart` (tag `docker`) needs no Ze kernel, so it runs against Colima and Docker Desktop directly. It fails if this client's machine id or boot id is unreadable, because the killed run's set is then "unknown" and kept.
- The wrapper builds with plain `docker build`. The buildx path was deleted in the parent. `--build-context` needs BuildKit, so a daemon whose `docker build` runs the legacy builder refuses the FreeRtr build (R-2).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/interoplab/orphans.go` - `readHostFacts` dispatches on GOOS; `readDarwinFacts` reads IOPlatformUUID and `kern.bootsessionuuid`, then `readDarwinBootStarted`; `ownerIdentity.labelPairs` emits `ze.lab.pid-ns` only when a pid namespace was read.
- [ ] `internal/le/dockerhost/dockerhost.go` - `Environment` selects the Colima socket as `DOCKER_HOST` only when the client's default endpoint is absent; an explicit `DOCKER_HOST` wins.
- [ ] `internal/test/perfrunner/run.go` - `Runner.imageBuild` gives the FreeRtr DUT `Context` = the repo root and one named context `freertr` = `FREERTR_DIR`, default `../rtr` beside the repo; `Runner.cleanup` and `Runner.removeContainer` remove on `context.WithoutCancel`.
- [ ] `internal/le/interoplab/docker.go` - `Docker.Build` renders each `NamedBuildContext` as one `--build-context name=path` before the main context.
- [ ] `internal/le/test/qemu/dockerlab.go` - `docker-lab` runs `le <lab>` inside the guest after dockerd answers `docker info` and the kernel check passes.

**Behavior to preserve:**
- Everything. This spec changes no behavior unless a check fails.

**Behavior to change:**
- None. A failing check is fixed at its producer under Failure Routing, and the fix keeps the parent's ACs.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Commands the owner types on his Mac: `./le perf run ...`, `./le test integration cleanup [dry]`, `./le test qemu docker-lab ...`, and one scoped `go test -tags docker` under `./le job run`.

### Transformation Path
1. le on the Mac reads its owner facts (darwin readers) and labels every create argv.
2. The docker CLI sends the argv to the selected daemon: the Colima VM, the Docker Desktop VM, or (through `docker-lab`) the QEMU guest's dockerd, where the guest's linux le is the client.
3. The sweep lists by `--filter label=ze.lab`, judges each owner on the client, and removes the gone set by id.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Mac client ↔ VM daemon (Colima, Docker Desktop) | docker CLI over the selected socket; owner facts stay client-side | No, AC-1..AC-4 |
| Mac host ↔ QEMU guest | `docker-lab` boots the guest and runs the linux le inside it | No, AC-5 |

### Integration Points
- `interoplab.Docker`, `Docker.CleanupOrphans`, `Runner.cleanup`, `Docker.Build`: all exist; nothing new is integrated.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every check runs a registered `./le` verb or the parent's tagged test |
| No unintended coupling (components stay isolated) | Yes | no code change planned |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses the parent's tests and verbs |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no wire path |
| Registration over hardcoding, outbound | N-A | no new command, verb or handler |
| Registration over hardcoding, inbound | N-A | no new name enters any list |

## Risks & Assumptions

### Assumptions
The three rows are the parent's, verbatim. Their status here starts again at `unvalidated` for macOS.

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-2 | Linux boot id (`/proc/sys/kernel/random/boot_id`) and macOS `kern.bootsessionuuid` are readable without privilege and change on every boot | kernel ABI; macOS sysctl | the "same machine, rebooted" verdict is unavailable; resources from before a reboot become "unknown" (skipped, reported) | AC-1, AC-3 on the owner's Mac | Linux confirmed in the parent; macOS unvalidated |
| A-3 | A machine identity is readable on both hosts: Linux `/etc/machine-id` (as `internal/core/identity` reads it), macOS IOPlatformUUID | `identity.go` `readMachineIDFile`; macOS `ioreg -rd1 -c IOPlatformExpertDevice` | without it, "another machine" and "this machine rebooted" cannot be told apart; both become "unknown" (skipped) | AC-1, AC-2 on the owner's Mac | Linux confirmed in the parent; macOS unvalidated |
| A-5 | Every daemon Ze labs use (Linux host, Colima, Docker Desktop, the QEMU guest's Alpine dockerd) supports `--label` and `--filter label=` on `run`, `network create`, `build`, `ps`, `network ls`, `image ls` | Docker CLI and API | the sweep cannot list; it reports a setup error naming the daemon version | AC-4 (Colima, Docker Desktop), AC-5 (the QEMU guest's dockerd) | Linux host confirmed in the parent; Colima, Docker Desktop and the guest dockerd unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The owner's Docker context and `DOCKER_HOST` point at a different daemon than the one a check names | `docker info` names the other VM | every AC selects the daemon by an explicit `DOCKER_HOST` and records `docker info --format '{{.Name}} {{.ServerVersion}}'` beside the result |
| R-2 | Colima's `docker build` runs the legacy builder, which refuses `--build-context` | AC-7 exits non-zero naming the flag | a defect in the parent's build path: fix it in `Docker.Build` under Failure Routing; never drop the named context |
| R-3 | The Mac daemon's kernel fails the Ze DUT kernel check, so `./le perf run dut ze` stops before it creates a DUT | the run's refusal names the missing features | run AC-6 on the daemon the owner already uses for perf; a refusal on every Mac daemon is recorded as the result and reported to the owner, never counted as a pass |
| R-4 | The reboot in AC-3 also removes the VM's stopped containers | `docker ps -a` after the reboot lacks the seeded container | seed again with a network as well as a container (both persist in the VM's storage) and record which survived |
| R-5 | `docker-lab` refuses before boot: no runtime kernel cache entry, or no HVF | the refusal names `./le build host-driver` and the kernel build | run the builds the refusal names, then rerun AC-5; nothing is skipped |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: the checks only read and remove labeled resources of the owner's own runs |
| How is it reverted? | Spec-only today; a fix found here is a single-commit revert |
| Who else touches this path? | the parent spec until it closes; `plan/pre-release/spec-lab-containers-least-privilege.md` edits `runContainer` argv |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration cleanup [dry]` on the Mac | → | `readDarwinFacts` → `Docker.CleanupOrphans` | `TestKilledLabRunIsCleanedByNextStart` (`internal/le/interoplab/orphans_docker_test.go`, tag `docker`), run on the Mac per AC-4 |
| `./le perf run dut ze`, then Ctrl-C | → | `signalRooted` → `Runner.cleanup` | `TestPerfInterruptedRunRemovesWhatItStarted` (`internal/test/perfrunner/run_test.go`); the real run is AC-6 |
| `./le perf run step build dut freertr` | → | `Runner.imageBuild` → `Docker.Build` with `--build-context freertr=<dir>` | `TestBuildRendersNamedContexts` (`internal/le/interoplab/container_test.go`); the real build is AC-7 |
| `./le test qemu docker-lab lab "test integration interop-ipsec"` | → | guest le `Suite.Run` → `Docker.CleanupOrphans` | `TestSuiteRunCleansOrphansBeforePreflight` (`internal/le/interoplab/lab_test.go`); the real run is AC-5 |

## Acceptance Criteria

Every AC runs on the owner's Mac. "Colima" means `DOCKER_HOST=unix://$HOME/.colima/default/docker.sock`, "Docker Desktop" means `DOCKER_HOST=unix:///var/run/docker.sock` with Docker Desktop running. Each result records the daemon (`docker info --format '{{.Name}} {{.ServerVersion}}'`), the commit and the date.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | As the owner's own user, no `sudo`: `sysctl -n kern.bootsessionuuid` and `ioreg -rd1 -c IOPlatformExpertDevice`, piped to `grep IOPlatformUUID` (A-2, A-3) | both exit 0 and print one UUID each; the two values are pasted into this spec |
| AC-2 | `./le perf run dut ze` on the perf daemon; while the DUT runs, `docker ps --filter label=ze.lab --format '{{.ID}}'`, then `docker inspect --format '{{json .Config.Labels}}'` on the DUT container (A-3, A-2) | `ze.lab.machine-id` equals the lowercase AC-1 IOPlatformUUID; `ze.lab.boot-id` equals the lowercase AC-1 `kern.bootsessionuuid`; `ze.lab.os` is `darwin`; `ze.lab.pid` is the pid of the running `le` (`ps -p <pid> -o command=`); `ze.lab.pid-ns` is absent |
| AC-3 | On one daemon: seed a dead owner (start `./le perf run dut ze`, `kill -9` the pid in the DUT's `ze.lab.pid` label once the DUT runs), reboot the Mac, start the same daemon, run `sysctl -n kern.bootsessionuuid`, then `./le test integration cleanup dry` (A-2) | the new boot id differs from the one in the seeded labels; `cleanup dry` lists each seeded container and network as "would remove", with no "unknown" line for them; `./le test integration cleanup` then removes them, exit 0 |
| AC-4 | Against Colima, then against Docker Desktop: `./le job run label mac-orphans command env ZE_REQUIRE_DOCKER=1 go test -tags docker -count=1 -run '^TestKilledLabRunIsCleanedByNextStart$' ./internal/le/interoplab/` (A-5, and A-2, A-3 through the verdict) | PASS on each daemon: the first sweep removes the killed child's container, network and retained tag and keeps the live child's set; the second removes the live set; 0 failed and 0 unknown in both sweeps; no setup error about a listing |
| AC-5 | On the Mac with HVF: `./le test qemu docker-lab lab "test integration interop-ipsec" timeout 3600s` (A-5, the QEMU guest's dockerd) | exit 0; the lab output carries the start sweep's `cleanup:` summary line and no "could not be listed" setup error; the run's labeled creates (`run`, `network create`, `build`) are accepted by the guest dockerd |
| AC-6 | On the perf daemon: `./le perf run dut ze`; once `docker ps --filter label=ze.lab` lists the DUT and the sender, press Ctrl-C once; then, with the run id from the DUT's `ze.lab.run` label, `docker ps -a --filter label=ze.lab.run=<id>`, `docker network ls --filter label=ze.lab.run=<id>` and `docker image ls --filter label=ze.lab.run=<id> --format '{{.Repository}}:{{.Tag}}'` | the run exits 1 after its removals; the first two commands print no resource; the third prints no `ze-interop-retained:` tag (the kept DUT image under its cache tag is allowed); `./le test integration cleanup dry` names no resource of that run |
| AC-7 | Against Colima, then against Docker Desktop, with a FreeRtr checkout at `FREERTR_DIR` (default `../rtr`): `FREERTR_DIR=<dir> ./le perf run step build dut freertr` | exit 0 on each daemon; `docker image inspect freertr-interop --format '{{index .Config.Labels "ze.lab.kind"}}'` prints `image`; no `ze-interop-retained:` tag of that run is left (`docker image ls --filter label=ze.lab.run=<id>`) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOwnerIdentityReadsBootID` (existing) | `internal/le/interoplab/orphans_test.go` | the darwin parser over injected `ioreg` and `sysctl` output; it is the fixture AC-1's real output is compared against | |
| `TestBuildRendersNamedContexts` (existing) | `internal/le/interoplab/container_test.go` | the `--build-context` argv AC-7 sends | |
| `TestPerfInterruptedRunRemovesWhatItStarted` (existing) | `internal/test/perfrunner/run_test.go` | the removals AC-6 expects | |

A defect found by an AC gets a failing unit test in the producer's package first, then the fix (Failure Routing).

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N-A: this spec adds no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestKilledLabRunIsCleanedByNextStart` | `internal/le/interoplab/orphans_docker_test.go` | AC-4 on Colima and Docker Desktop | |
| `./le perf run dut ze` stopped with Ctrl-C | the owner's Mac, output pasted here | AC-6 | |
| `./le perf run step build dut freertr` | the owner's Mac, output pasted here | AC-7 | |
| `./le test qemu docker-lab lab "test integration interop-ipsec"` | the owner's Mac, output pasted here | AC-5 | |

## Files to Modify
- None planned. A failing AC changes its producer: `internal/le/interoplab/orphans.go` (AC-1..AC-4), `internal/le/interoplab/docker.go` (AC-4, AC-5, AC-7), `internal/test/perfrunner/run.go` (AC-6, AC-7), `internal/le/test/qemu/dockerlab.go` (AC-5), and the page that describes it.
- `plan/spec-lab-docker-cleanup-and-go-test-containers.md` - if the parent is still open when a result lands, its A-2, A-3 and A-5 rows name this spec's result; once the parent is closed, nothing.

## Files to Create
- None.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no Ze config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | N-A | no new verb; existing `./le` verbs only |
| CLI grammar (keyword before value) | N-A | no new verb |
| Editor autocomplete | N-A | not a ze CLI command |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | `FREERTR_DIR`, `DOCKER_HOST` and `ZE_REQUIRE_DOCKER` already exist |
| Doctor check for runtime dependencies | N-A | development host only |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | development tooling |
| 2 | Config syntax changed? | N-A | none |
| 3 | CLI command added/changed? | N-A | none |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | Yes, if AC-6 or AC-7 fails | `docs/guide/benchmarking.md` "Automated Docker Runner" states the Ctrl-C removal; a fix that changes it edits that paragraph |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | none |
| 10 | Test infrastructure changed? | Yes, if AC-1..AC-5 fail | `docs/architecture/testing/interop.md` "Crash-safe teardown" states the macOS label sources; a fix that changes them edits that table |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | N-A | none |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | none |
| 16 | Any changed source file referenced by existing doc source anchors? | N-A until a fix | `./le spec citation anchors spec plan/spec-lab-docker-cleanup-on-macos.md` once a fix names a file |
| 17 | Existing docs show config/CLI/API examples for this area? | No | the `docker-lab` and `perf run` examples are used as written |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - nothing to wire: every entry point exists. An agent confirms each Wiring Test row's test passes on Linux at the commit the owner will run, and writes the AC command list for the owner.
   - Tests: the four Wiring Test rows
   - Files: none
   - Verify: each named test exits 0 under `./le job run`
2. **Phase: Mac run (the owner)** - AC-1, AC-2, AC-4, AC-7 on both daemons; AC-6 on the perf daemon; AC-5; AC-3 last, because it reboots the Mac.
   - Tests: AC-1..AC-7
   - Files: this spec (pasted output)
   - Verify: each AC's pass condition, read from the pasted output
3. **Phase: Fixes** - only for a failed AC: failing test first, fix at the producer, the owner reruns that AC.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC has pasted output from the owner's Mac, with the daemon and commit named |
| Correctness | AC-2's label values are compared to AC-1's real output, not to a fixture |
| Correctness | no AC passes on a run that was refused before it created anything (R-3, R-5) |
| Rule: docker.md | no prune and no removal by name pattern in any command the owner is given |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| A-2, A-3, A-5 each confirmed or broken on macOS | the Assumptions table here, each row naming its AC output |
| Ctrl-C leaves nothing | AC-6 output pasted |
| FreeRtr builds with `--build-context` | AC-7 output pasted for both daemons |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Destructive scope | every removal in the AC commands goes through `./le test integration cleanup`, which removes by id after a verdict; no command removes by name or pattern |
| Privilege | AC-1 runs without `sudo`, which is the A-2 claim itself |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| An AC fails on the Mac | a defect in the parent's implemented capability: failing unit test, fix at the producer, the owner reruns the AC |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The Mac checks need no new code: the parent's tests and verbs already reach the darwin readers and the VM daemons. What was missing was a host that runs them.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Prove A-5 on Colima and Docker Desktop with `TestKilledLabRunIsCleanedByNextStart` | a full `interoplab` lab on each VM daemon | the labs run Ze, so the VM kernels may fail the Ze kernel check; the kill test needs no Ze kernel and exercises every labeled create and filtered listing A-5 names |
| Prove the guest dockerd with one real lab through `docker-lab` | a second kill test inside the guest | `docker-lab` runs a lab verb; a lab's start sweep and its labeled creates are the guest's real use |

## Known Limitations
- AC-2 and AC-6 run on one daemon, the one that passes the Ze DUT kernel check (R-3). AC-4 covers both VM daemons for labels and verdicts.

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test (N-A: Scope is tooling)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes, if a fix changed code
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written (only for a fix)
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A: none)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: tooling, no wire change)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-lab-docker-cleanup-on-macos.md` only, in the same `./le commit create` script
