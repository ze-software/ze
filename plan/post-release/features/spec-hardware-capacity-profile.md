# Spec: hardware-capacity-profile

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P2 |
| Release reason | Post-release by owner decision (2026-11-23): "We should have it after release. It's not something already holding the first release." Nothing here holds the first release; the supported release keeps its existing obligations |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-vpp-numa-smt.md` (NUMA node and SMT sibling facts, needed before sizing is right on a multi-socket host); `plan/release/validation/spec-doctor-coverage-15-system.md` (the `host-hardware` and `host-tuning` checks this spec's doctor rows sit beside); `plan/release/validation/spec-host-resource-trend-alerts.md` (the per-metric trend rules a high-load rule would join); `plan/release/validation/spec-installed-count-memory-magnitude-undisclosed.md` (a measured bytes-per-prefix cost, one input of the memory estimate) |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Skeleton. Design not started.**

Thomas's request, as dictated (2026-11-23):

> Ze init should be able to look at the CPU in memory and from it using
> Heuristic provide maximum performance guaranteed, being safe on the device so
> someone can understand what to expect. For example for both running with
> canal routing as well as vpp we can effectively have a database of known
> configuration, and if the hardware is unknown then have a guess estimate and
> say it's an estimate. It is important because effectively from the step he
> should set for example how many of the core we assign, for example for VPP in
> the VPP configuration we're going to start or thing like that. Also, if we
> believe there's not enough memory to start the process on it in it, we should
> raise it. Same thing for check, ze doctor should allow to tell you there is
> what we think is a misconfiguration or a problem, probably we could append
> under high load.

("canal routing" is dictation for kernel routing.)

The work has four parts, each to be designed:

| Part | What the operator gets |
|------|------------------------|
| P1 Capacity profile | From the detected CPU, memory and NICs, an expected capacity for the kernel data plane and for VPP (forwarding rate, route scale, peer scale), taken from the trusted database when the hardware matches an entry (an observed value), and otherwise computed by a heuristic (an estimate, information only). The answer says which it is. It is a guarantee only on the appliance, where Ze knows the full machine state; on a general Linux install the same figure is informational |
| P2 Sizing at init | `ze init` uses the profile to write the VPP settings it can choose (main core, worker count, main heap, buffers) into the config it writes, so the first start runs with settings that fit the box |
| P3 Not enough resources | At every start, against the machine as it is then, Ze checks whether the resources are enough for what is configured: VPP heap and buffers against reserved hugepages, workers against cores, and the RIB for the configured peers and families against RAM. A shortfall is a warning that names the number Ze computed, says the operator is taking a risk, and gives the fix; Ze still starts. It never refuses: the requirement changes with the configuration, Ze cannot know how the operator uses the box (a peer may send only a partial view), and on a general Linux install other programs take memory too. A measurement the operator recorded on their own hardware stops the warning there, and the operator has a way to silence the warning (Thomas, 2026-11-23: "as we warn a user should have a way to silence the warning"). No general way to silence a warning exists in Ze today (grep of the YANG and of `internal/core/report`, `internal/component/doctor`, `internal/core/diagnostic`, 2026-11-23), so the design adds one. Thomas (2026-11-23): "Probably an environment value in the configuration or os", that is a registered `ze.*` environment key (`env.MustRegister`), set in the config's `environment` block or in the OS environment, which already name the same key |
| P4 Doctor | `ze doctor` reports a configuration that contradicts the hardware (more workers than cores, workers on a NIC's remote NUMA node, VPP memory beyond the hugepages) and the problems that show only under load (CPU saturation, softirq starvation, NIC drops, conntrack fill) |
| P5 Record mode and built-in database | A mode of `ze` records a performance run on the box, tied to the hardware it ran on, into a record of the database's own format. The record carries all the technical information about the machine that lets another box be matched to it (CPU model and cores, memory, NIC driver, PCI ids and speed, platform and DMI model, kernel, Ze version, data plane, whether the host is a VM). The format is JSON, carries a version, and stays backward compatible: a new Ze reads every record an older Ze wrote, so data gathered over time is kept. The known-platform table P1 reads is that database: a JSON file in the repository, embedded in the binary, updated by a commit after a test on known hardware or from a record a user reports |

Thomas's addition (2026-11-23), on how the known-platform table is filled:

> We should have a mode in Ze to record performance so when we do a test, we
> can use that data to update our system. So we should have a "built-in
> database" for example, like JSON file embedded or the like, we can update
> after test or have users report their testing

And on how a user reports (2026-11-23):

> they run a lab with ze in "record perf mode" and it produces an artifact
> they can share with us

So record mode is a way to run Ze, not a load generator: the user drives
their own lab traffic through a Ze in record perf mode, Ze records what it
observes (forwarding rate, route and peer scale reached, CPU and memory use,
drops, convergence) against the hardware it runs on, and at the end of the
run writes one cleaned artifact the user shares with the project. The same
mode serves the project's own lab runs.

### Defects found while researching, owned here

| Defect | Consequence | Why here |
|--------|-------------|----------|
| The `vpp-hugepages` doctor check re-declares the VPP memory defaults (2M, 1G, 128000, 512M) in `hugepageParamsFromTree` instead of reading them from `ParseSettings` | A change to a default in YANG or `ParseSettings` leaves the doctor estimating against the old value, so the warning is wrong with no test red | The memory estimator this spec builds replaces that arithmetic; one estimator serves init, commit and doctor |
| The same estimate counts buffers once, while VPP's `buffers-per-numa` reserves them on every NUMA node, and it ignores buffer metadata | On a multi-socket host the doctor reports enough hugepages when VPP will fail to allocate | Same estimator; needs the NUMA facts from `spec-vpp-numa-smt` |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/vpp-host-tuning.md` - worker placement and hugepages today
  → Constraint: hugepages and isolated CPUs are image facts; a change needs an image rebuild and a reboot, and the doctor only detects drift. A running device can report a mismatch but not fix it
  → Decision: worker cores come from the kernel's isolated set, not from arithmetic; `resolveWorkerCores` is the one producer of the core list for validation and startup.conf
- [ ] `docs/research/vpp-deployment-reference.md` - the only per-hardware numbers in the tree (IPng: Xeon D-1518, 4 cores, 1.5G heap for a full table; hugepage table lab 2 GB, 10G production 6 GB, DFZ 8 GB)
  → Constraint: these are third-party figures and cannot be published as Ze measurements
- [ ] `ai/META-DECISIONS.md`, "Test systems and labs"
  → Constraint: a performance figure that is reported, compared or recorded comes only from a machine whose performance is known (real hardware, the Mac, or a VM on a known host). The known-platform table is filled only from such runs; the Linux development VM fills nothing; every other figure is published as an estimate
- [ ] `ai/META-DECISIONS.md`, "CLI, config, and daemon surfaces"
  → Constraint: nothing Ze does on the operator's behalf is hidden, so a value init chooses is written into the config as an ordinary leaf, never applied silently at runtime
  → Constraint: every problem an operator can hit is a doctor check with its cause and fixing command; every operator command runs through the daemon on every surface
- [ ] `ai/rules/principles.md`
  → Constraint: every value is declared once; the VPP defaults live in one place and init, commit and doctor read them

**Key insights:**
- Ze already has a full read-only hardware inventory and already places VPP workers from the host; what is missing is any step that chooses a size from the hardware, any memory refusal, and any expected-capacity answer.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/init/main.go` - `writeInitialConfig` writes the SSH listener and the interfaces `discoverInterfaces` returns, nothing else: no CPU, memory, hugepage or VPP fact is read and no VPP or CPU setting is written
- [ ] `internal/component/vpp/config.go` - `ParseSettings` holds the fixed defaults: main heap 1G, hugepage size 2M, buffers 128000, stats segment 512M; no worker count is derived from the host
- [ ] `internal/component/vpp/cpuset.go` - `resolveWorkerCores` draws an operator-given worker count from the isolated set (lowest first, main core excluded), or the contiguous block after main core when nothing is isolated; `hostCPUInventory` reads online and isolated CPUs from sysfs
- [ ] `internal/component/vpp/doctor_linux.go` - `evaluateVPPHugepages` estimates main heap + buffers x 2048 + stats segment against reserved hugepages, a warning at doctor time only; `hugepageParamsFromTree` carries its own copy of the defaults
- [ ] `internal/appliance/kernelargs.go` - `hugepageKernelArgs` and `isolatedCPUKernelArgs` turn `image.hugepages` and `image.isolated-cpus` into boot arguments at image build
- [ ] `internal/component/host/cpu_linux.go`, `memory_linux.go` - `DetectCPU` reads model, logical CPUs, physical cores, threads per core, hybrid roles, frequencies; `DetectMemory` reads meminfo (total, available, hugepages) and ECC. No NUMA node, no SMT sibling map

Further verified facts:
- Nothing refuses to start for lack of memory: a grep for `GOMEMLIMIT` and `SetMemoryLimit` under `internal/` and `cmd/` finds only `le` tooling, and no per-route or per-peer memory estimate exists.
- No recorded Ze performance figure exists for any hardware: `test/perf/history/` holds only `.gitkeep`.

**Behavior to preserve:**
- An operator-set `vpp cpu` or `vpp memory` leaf always wins over anything the profile proposes
- `resolveWorkerCores` stays the one producer of the worker core list
- `ze init` keeps its current output for a host where the profile has nothing to add

**Behavior to change:**
- To be designed (Task parts P1 to P4)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Host hardware inventory | `internal/component/host` `Detector.Detect` | Reads CPU, memory, NIC and DMI facts for `show host`, `show system`, doctor and metrics | Extends: the profile's only input; NUMA and SMT facts come from `spec-vpp-numa-smt` |
| VPP worker placement | `internal/component/vpp/cpuset.go` `resolveWorkerCores` | Turns a worker count or list into cores from the isolated set | Extends: the profile proposes the count; placement stays here |
| VPP settings and defaults | `internal/component/vpp/config.go` `ParseSettings`, `vpp` YANG | Holds the operator values and the fixed defaults | Extends: init writes the chosen values as ordinary leaves the operator can see and edit |
| Image kernel arguments | `internal/appliance/kernelargs.go` `hugepageKernelArgs` | Emits hugepage and isolated-CPU arguments from the image config | To decide (C-3): whether `ze appliance init` sizes them from a declared platform, since the build host cannot see the target |
| `vpp-hugepages`, `vpp-cpu-isolation` doctor checks | `internal/component/vpp/doctor_linux.go` `evaluateVPPHugepages` | Warn on missing or short hugepages and unisolated workers | Replaces the estimate arithmetic with the shared estimator (the two owned defects) |
| Host trend rules | `plan/release/validation/spec-host-resource-trend-alerts.md` | Slow-exhaustion rules per metric | Extends: a high-load rule lands there under its per-metric ruling, not as a parallel monitor |
| `ze init` config writer | `internal/plugins/init/main.go` `writeInitialConfig` | Writes the listener and discovered interfaces | Extends: adds the sized VPP block |
| `./le perf` benchmark harness | `docs/performance.md`, `internal/le/perf` | Measures BGP convergence, throughput and withdrawal against DUTs in Docker, writes result JSON with the run's environment (platform, virtualisation, commit), and refuses a lossy run in comparisons; `le perf record` stores only a baseline commit (`Report.Recorded`), no figure | Extends as a producer: a project run on known hardware emits a database record through the P5 format. It cannot be the user-facing mode: `le` is development tooling and never carries a product feature (`ai/META-DECISIONS.md`, "Test systems and labs"), so record mode lives in `ze` |
| Support bundle sanitizer | `internal/component/support/sanitize.go` `sanitizeConfig` | Replaces the values of sensitive config leaves with REDACTED before a bundle is shared | Extends: a user-reported record carries a config shape and host facts, so it is cleaned by the same producer; it must also leave out identifying host facts (serial numbers, MAC addresses, hostname) the bundle keeps |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| [page] | [section heading] | [what derived it] | "[verbatim span]" | [agrees / departs / silent] | Filled at design from `./le spec design documents` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [To be designed: `ze init`, the commit verify of the `vpp` block, `ze doctor`, and possibly a capacity command]

### Transformation Path
1. [To be designed]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [to be designed] | [ ] | No |

### Changed-Wire Census

**Search evidence:** [to be filled at design]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [finding] | [disposition] |

### Integration Points
- [To be designed]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The feature can land with an empty trusted database: every answer is then an estimate, labelled information only, and the database fills as Thomas adds records by the documented procedure | Thomas (2026-11-23), answer to C-2 | - | Thomas's answer to C-2 | confirmed |
| A-3 | A user's record on real hardware satisfies the owner rule that a figure comes only from a machine whose performance is known, while a record from a VM on an unknown host does not | `ai/META-DECISIONS.md`, "Test systems and labs": real hardware is a known host | User reports either cannot enter the table at all, or enter it under a label that is not "measured" | Thomas's answer to C-6 | unvalidated |
| A-2 | 2M hugepages can be reserved at runtime on first boot early enough to succeed, while 1G pages and isolated CPUs need the boot command line | Kernel behaviour; `docs/architecture/vpp-host-tuning.md` | First-boot sizing cannot cover hugepages and must go through the image build | A QEMU first-boot run | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An estimate is read as a promise, and an operator sizes a deployment on it | An operator report quoting an estimate as a measured figure | Every answer carries its kind (measured on a named platform, or estimate) in the same line as the figure |
| R-3 | A user-submitted record is wrong (a VM reported as hardware, a lossy run, an edited file) and skews the table for everyone on that platform; and once users submit records, the record format is a contract with people outside the repository | A submitted record whose figures sit far from the project's own run on the same platform, or a record that fails to parse after a format change | Record mode applies the `./le perf` publication rule (a lossy run is not a figure) and records the platform detection it saw; a user record enters only by reviewed commit (C-6); the format carries a version so an older report still parses |
| R-4 | In a user's lab Ze sees what it forwarded, not what the traffic generator could offer, so a run limited by the generator records a figure below the box's capacity and is read as its maximum | A record whose CPU, worker and NIC counters show headroom (no saturated worker, no rx ring or buffer drops) | The artifact states whether a limit was reached and which (CPU, a worker, a NIC queue, memory): a run that reached one records a maximum, a run that reached none records "at least" that figure, and only a maximum fills the table's capacity value |
| R-2 | Init-chosen values drift from what the hardware supports after a hardware change or image move | Doctor sees workers beyond the isolated set, or hugepages short of the estimate | The P4 doctor rows judge the running config against the current hardware, not against what init saw |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A first start with VPP settings that do not fit the box, or a commit refused for a memory shortfall that is not real |
| How is it reverted? | Single commit revert; the written leaves are ordinary config the operator can edit |
| Who else touches this path? | `spec-vpp-numa-smt`, `spec-doctor-coverage-15-system`, `spec-host-resource-trend-alerts`, `spec-vpp-embedded-runtime` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [to be designed] | → | [ ] | [ ] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | [to be designed] | [ ] | [ ] |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze init` on a new box with VPP selected | [to be designed] | [ ] |
| 2 | Runs `ze doctor` on a box whose VPP config exceeds its hardware | [to be designed] | [ ] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to be designed] | [ ] | [ ] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [to be designed] | [ ] | [ ] | [ ] | [ ] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [to be designed] | [ ] | [ ] | |

## Files to Modify
- `internal/plugins/init/main.go` - write the sized VPP block (P2)
- `internal/component/vpp/doctor_linux.go` - use the shared estimator (owned defects)
- [rest to be designed]

## Files to Create
- A guide page, under `docs/guide/`, on how to run Ze in record perf mode and add the artifact to the trusted database: the deliverable C-2 asks for, so Thomas can fill the database himself
- [rest to be designed]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | To be answered at design |
| YANG validation constraints | | To be answered at design |
| YANG custom validators | | To be answered at design |
| CLI commands/flags | | To be answered at design |
| CLI grammar (keyword before value) | | To be answered at design |
| Editor autocomplete | | To be answered at design |
| Functional test for new RPC/API | | To be answered at design |
| Pipe completeness | | To be answered at design |
| Env var registration | | To be answered at design |
| Doctor check for runtime dependencies | | To be answered at design |
| Doctor code per problem class | | To be answered at design |
| Prometheus counters/metrics | | To be answered at design |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP family is added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | To be answered at design |
| 2 | Config syntax changed? | | To be answered at design |
| 3 | CLI command added/changed? | | To be answered at design |
| 4 | API/RPC added/changed? | | To be answered at design |
| 5 | Plugin added/changed? | | To be answered at design |
| 6 | Has a user guide page? | | `docs/guide/vpp.md`, `docs/guide/quickstart.md` likely |
| 7 | Wire format changed? | | To be answered at design |
| 8 | Plugin SDK/protocol changed? | | To be answered at design |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No protocol behaviour |
| 10 | Test infrastructure changed? | | To be answered at design |
| 11 | Affects daemon comparison? | | To be answered at design |
| 12 | Internal architecture changed? | | `docs/architecture/vpp-host-tuning.md` likely |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | | To be answered at design |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | To be answered at design |
| 16 | Any changed source file referenced by existing doc source anchors? | | Derived at design by `./le spec citation anchors` |
| 17 | Existing docs show config/CLI/API examples for this area? | | To be answered at design |

## Design presentation

### What changes

[To be written at design]

### Before and after

[To be written at design]

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| [to be designed] | [ ] |

### Documents departed from

[To be written at design]

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | What a capacity answer promises | (a) a measured figure only for a known platform, an estimate labelled as such otherwise, never the word "guaranteed"; (b) a conservative floor presented as guaranteed | Thomas (2026-11-23): every answer says which source it comes from. A value from the trusted database is an observed value; an estimate is information only. Performance is guaranteed only when Ze runs as an appliance, where it has full knowledge of the machine state, and the hardware matches a trusted entry. On a general Linux install, even on the same hardware, the same figure is informational, not guaranteed |
| C-2 | Which platforms are measured first for the known-platform table, on which known hardware | (a) the N100-class appliance target and one server class, each recorded with P5 record mode; (b) other | Filled by record mode into the embedded JSON database (Thomas, 2026-11-23). Thomas (2026-11-23): the spec does not plan which platforms are measured or when; he adds data once documentation says how to do it, and the table fills when it fills. The spec owes that documentation, not a first entry |
| C-6 | How a user-reported record enters the shipped database | (a) through a reviewed commit, labelled "user-reported" beside "project-measured", and only when the record shows real hardware, so an estimate, a user report and a project measurement are three different answers; (b) user reports are kept apart and never shipped in the table | Thomas (2026-11-23): how users send data back is decided later (Discord, email, a collecting daemon); it may be non-technical. What this spec owes now is a versioned, backward-compatible JSON format that keeps data across versions |
| C-7 | Whether a box uses its own recorded run | (a) yes: a run recorded on this box is kept in its store and beats both the embedded table and the estimate for this box only, labelled "measured on this device"; (b) no, only the embedded table is read | (a), Thomas (2026-11-23): an operator can add their own measurement, so Ze does not warn on hardware they have tested themselves |
| C-8 | How a user reports a record | (a) record mode writes a cleaned file the user chooses to submit (an issue or a pull request); Ze never sends it anywhere by itself; (b) an opt-in upload command | (a), refined by Thomas (2026-11-23): the user runs a lab with Ze in "record perf mode", and it produces an artifact they share with the project; the channel for sharing it is decided later (C-6) |
| C-3 | Where sizing applies on the appliance | (a) both: `ze appliance init` takes a declared platform for hugepages and isolated CPUs, and first boot writes the YANG leaves from detection; (b) first boot only, reporting hugepage and isolation mismatches through doctor | |
| C-4 | Whether the profile is also a command (`show host capacity`) served on every surface | (a) yes, so an operator re-reads it after a hardware change; (b) init output only | |
| C-5 | Memory shortfall: refuse or warn | (a) refuse a VPP heap and buffers that cannot be allocated, warn on an estimated RIB shortfall; (b) warn on both | Thomas (2026-11-23), replacing his earlier refusal answer: warn on both, never refuse. The requirement changes with configuration, Ze cannot know how the box is used (peers may send partial views), and on Linux other programs take memory, so the check runs at every start and warns that the operator is taking a risk (recorded in `docs/architecture/vpp-host-tuning.md`, "Design decisions") |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- to be designed
2. **Phase: [name]** -- to be designed

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Every capacity figure names whether it is measured or estimated |
| Operator errors and doctor codes | Every refusal and doctor row names the computed number and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [to be designed] | [ ] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Host facts read from `/proc` and `/sys` that are missing or malformed are reported as unknown, never as zero |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [To be written at design]

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: at least `./le test unit pkg path ./internal/component/vpp/`, `./le test unit pkg path ./internal/plugins/init/`, `./le test unit pkg path ./internal/component/host/` and `./le test vpp`, the full set fixed at design
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
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
