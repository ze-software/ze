# Spec: test-sandbox

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Test tooling: a red that judges the host instead of the code wastes the evidence the release depends on, but no shipped behaviour changes |
| Depends | `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` (the Ze-image guest this spec reuses) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Sibling: `plan/post-release/engineering/spec-test-verdicts-independent-of-load.md`
owns the timing half of root cause T1 (a verdict that depends on how busy the
host is). This spec owns the state half (a verdict that depends on what the host
holds). The two were split because their fixes touch different producers and can
land in either order.

## Task

**Root cause.** Every session shares one `main` checkout and one host, by the
owner rule in `ai/META-DECISIONS.md` ("Sessions share the one `main` checkout").
That rule is the cause of this family and this spec does not change it. What it
changes is that a test inherits whatever the host happens to hold: the working
directory (the checkout root), the process environment, the config store, fixed
ports and addresses, process-global registries, the host's operating system and
kernel, and the live git repository. Nothing gives a test its own copy, so its
verdict is a function of the machine and of the sibling tests and sessions on
it.

**Why it matters.** 60 open journal rows record this one mechanism under ten
class names. Each red was chased as its own flake, and each was attributed to
"somebody else", because the test could not say what it owned. A gate that can
be red for host reasons cannot be trusted green either.

**The fix in one sentence.** The test runner and the Go test helpers hand every
test a sandbox it owns (working directory, environment, config dir, leased
host names, registry scope, and for Linux-dependent work the Ze-image guest),
and a check after each test refuses one that touched anything outside it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/runner-architecture.md` - port leases, timeout resolution, netns mode
  -> Constraint: `.ci` `$PORT`/`$PORT2` are already leased through `LeaseTestPorts` with per-port advisory locks under `/tmp/le-port-locks-<uid>`. The sandbox extends that registry; it does not add a second one.
- [ ] `docs/architecture/testing/qemu-integration.md` - guest kinds, Alpine ISO path
- [ ] `docs/architecture/appliance/docker-host.md` - the Ze docker-host image the lab guest boots
- [ ] `plan/release/delivery/spec-docker-hosts-run-the-ze-kernel.md` - Ze-kernel requirement for Docker hosts
  -> Constraint: only the docker-lab guest moved to Ze's image; `run`, `all-tests`, `stress` and `mobike` still boot Alpine (`internal/le/test/qemu/run_iso.go`, `alpineISOName`). The owner rule rejects Alpine for test guests, so this spec routes them to Ze's image.
- [ ] `plan/post-release/engineering/spec-lab-docker-cleanup-and-go-test-containers.md` - run-owned Docker labels and cleanup
  -> Decision: orphaned lab networks are that spec's cleanup; this spec only leases the subnet so two live runs cannot pick the same one.
- [ ] `plan/post-release/engineering/spec-fixit-netns-test-dut-tags.md` - the netns-test vehicle
- [ ] `plan/release/fixes/spec-fixit-ca-root-location.md` - product use of `DefaultConfigDir`; not changed here

## Current Behavior (MANDATORY)

Verified at the producer on 2026-10-23.

- [ ] `internal/test/runner/runner_exec.go` - sets `ze.config.dir=<rec.WorkDir>` for client and daemon children (since `ba67a8b89c`, 2026-09-17), and `Dir = rec.WorkDir` for peer and client. This already fixed two store rows (below).
- [ ] `internal/test/runner/decoding.go` - the `ze bgp decode` spawn sets no `Dir`, so it runs in the checkout root.
- [ ] `internal/test/runner/runner.go` - one spawn sets `cmd.Dir = r.baseDir`; `setupBinShims` builds the shim directory.
- [ ] `internal/test/fixture/plugin_fixture_01_answer.go` - writes `daemon.ready` with a relative path; so do `misc_fixture_managed_legacy.go`, `misc_fixture_shellports.go` and `plugin_fixture_06_shell.go`. The flag lands wherever the fixture process runs.
- [ ] `internal/test/fixture/bounded.json` - still TRACKED and still rewritten by the fixture at run time.
- [ ] `internal/core/paths/paths.go` - `DefaultConfigDir` reads `ze.config.dir`, else resolves the running binary with `EvalSymlinks`; a child started without the variable reaches the developer's store.
- [ ] `internal/core/env/env.go` - `ensureCache`: an inherited dotted `ze.config.dir` beats the undotted spelling a fixture sets.
- [ ] `internal/test/runner/loopback_bsd.go` - `ensureLoopbackAlias` refuses when the host lacks the alias, so the verdict is the host's `./le setup` state.
- [ ] `internal/test/runner/ports.go` - `LeaseTestPorts`, `ReservePorts`: leases cover `$PORT`/`$PORT2` only. Fixed ports a test or daemon binds itself (REST 18081, IKE UDP 4500), loopback addresses, and lab /24s are not leased.
- [ ] `internal/component/command/registry/registry.go` - `ResetForTest` empties every root command; no `_test.go` calls it any more (only a comment in `cmd/ze/root_dispatch_test.go`).
- [ ] `internal/le/test/qemu/run_iso.go` - `alpineISOName`: four guest kinds still boot Alpine.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A test starts from one of three entries: the `.ci` runner (`./le test <suite>`), a Go test (`go test`, under `./le test unit`), or a lab or guest action (`./le test integration ...`, `./le test qemu ...`).

### Transformation Path
1. The entry asks the sandbox provider for a sandbox, naming what the test declares it needs (ports, addresses, subnets, kernel features, privileges).
2. The provider creates a private directory outside the checkout, a scrubbed environment built from an allow-list, a config dir, and leases every host-scoped name from the one lease registry.
3. A test whose declared needs the host cannot meet runs in the Ze-image guest, or is reported `not-run: <reason>`; it is never reported passed.
4. The test runs with that directory as its working directory and that environment as its whole environment.
5. On exit the provider reaps the process group, releases the leases, and compares the checkout's untracked and modified set with the set before the test.

### Boundaries Crossed
Runner to child process (environment, cwd); runner to host kernel (addresses, ports); runner to Docker or QEMU (guest, subnets); Go test to process-global registry.

### Integration Points
`LeaseTestPorts` (extended, not duplicated); the docker-host image from `spec-docker-hosts-run-the-ze-kernel`; the `needs-linux` and kernel-capability declarations the runner already reads (`internal/test/runner/caps.go`).

### Architectural Verification
- [ ] One lease registry for every host-scoped name
- [ ] No test helper reads the checkout root as scratch
- [ ] Linux-dependent suites run on Ze's kernel, never on whatever the host runs

## Risks & Assumptions

### Assumptions
| Id | Assumption | Evidence | If wrong |
|----|------------|----------|----------|
| A-1 | The per-port advisory lock mechanism scales to addresses and subnets | it already serializes `$PORT` across sessions and users | add a lock file per name class under the same directory |
| A-2 | The Ze docker-host guest can run the `.ci` suites that need Linux | docker-lab already runs in it (`ca386b6ae3`) | keep those suites `not-run` on darwin with the reason, never green |

### Risks
- Scrubbing the environment can drop a variable a test silently relied on. The allow-list is derived from `env.MustRegister` declarations, so a missing one is a registration defect the test names.

## Blast Radius
`internal/test/runner`, `internal/test/fixture`, `internal/test/cli`, `internal/le/test/qemu`, `internal/le/interoplab`, and the Go test helpers of packages that register into process globals (iface, reactor bufmux, config transaction, Loc-RIB, IKE test port).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `./le test plugin <case>` | -> | runner sandbox: cwd, env, config dir, leases | a fixture that writes a relative file and reads an unregistered env var; the runner reports the file outside the sandbox and the variable absent |
| `go test` of a registering package | -> | per-test registry scope | the package passes under `-count=2 -shuffle=on` |
| `./le test qemu run` | -> | Ze-image guest | the guest boots Ze's kernel; `run_iso.go` is gone |

## Acceptance Criteria

| AC | Condition | Observable |
|----|-----------|------------|
| AC-1 | any `.ci` case runs | its child processes' cwd is a directory outside the checkout; after the run `git status --porcelain` lists nothing the case created |
| AC-2 | a case writes into the checkout root or a tracked file | the runner fails the case, naming the path and the step that wrote it |
| AC-3 | a child process starts | its environment holds only allow-listed variables plus the sandbox's own; an inherited `ze.config.dir` (either spelling) never reaches it |
| AC-4 | two copies of one selector run at once (`./le test stress`) | no `address already in use`, no shared store, no shared /24: every port, loopback address and lab subnet came from a lease |
| AC-5 | a run ends, killed or not | no process it started survives, and no address it added stays on loopback |
| AC-6 | a case declares a kernel feature, privilege or OS the host lacks | it runs in the Ze-image guest, or is reported `not-run: <reason>` and the suite verdict is not green; it is never `pass` |
| AC-7 | `./le test qemu run`, `all-tests`, `stress`, `mobike` | the guest boots Ze's image and kernel; `alpineISOName` and `run_iso.go` are deleted |
| AC-8 | every Go package that registers into a process global | passes `go test -count=2 -shuffle=on`; the unit gate runs every package that way |
| AC-9 | a test reads the live wiki, the live git HEAD, or the host's `lo` name | it reads a fixture the sandbox provides instead |
| AC-10 | every row in "Owned journal rows" | re-run or re-read after the change: green, or still red with a cause that is not host state |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Proves |
|------|------|--------|
| `TestSandboxCwdIsOutsideTheCheckout` | `internal/test/runner/sandbox_test.go` | AC-1 |
| `TestSandboxRefusesAWriteIntoTheCheckout` | `internal/test/runner/sandbox_test.go` | AC-2 |
| `TestSandboxEnvironmentIsTheAllowList` | `internal/test/runner/sandbox_test.go` | AC-3 |
| `TestLeaseCoversAddressesAndSubnets` | `internal/test/runner/ports_test.go` | AC-4 |
| `TestSandboxReapsItsProcessGroup` | `internal/test/runner/sandbox_test.go` | AC-5 |
| `TestUnmetNeedIsNotRunNeverPass` | `internal/test/runner/caps_test.go` | AC-6 |

### Functional Tests
| Test | Proves |
|------|--------|
| `test/plugin/sandbox-refuses-checkout-write.ci` | AC-2 end to end |
| two concurrent `./le test stress` copies of `test/ipsec/ipsec-status-show.ci` | AC-4 |

## Files to Modify
- `internal/test/runner/runner.go`, `runner_exec.go`, `decoding.go`, `parsing.go`, `ports.go`, `caps.go`
- `internal/test/fixture/*` writers of `daemon.ready`, `bounded.json`
- `internal/test/cli/cmd_exabgp_process.go`, `cmd_web.go`, `cmd_mcp.go`
- `internal/le/test/qemu/run.go`; delete `internal/le/test/qemu/run_iso.go`
- `internal/le/interoplab/` subnet choice and `kernelcheck.go` (`appArmorProfiles` reads the daemon's host)
- the unit-test runner (`internal/le/testunit/groups.go`) for `-count=2 -shuffle=on`

## Files to Create
- `internal/test/runner/sandbox.go`, `sandbox_test.go`

### Integration Checklist
- [ ] Every child-process spawn in `internal/test/` goes through the sandbox
- [ ] Every registry with a `Register` exposes the unregister a test needs

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/testing/runner-architecture.md` - sandbox and leases
- [ ] `docs/architecture/testing/qemu-integration.md` - guests boot Ze's image
- [ ] `docs/functional-tests.md` - a case owns nothing outside its sandbox
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` declarations a case uses to name what it needs (ports, addresses, kernel features)

## Implementation Steps
1. Extend the lease registry to addresses and subnets; move fixed ports (REST, IKE NAT-T) onto leases.
2. Add the sandbox (cwd, env allow-list, config dir, process group); route every spawn through it.
3. Add the after-test checkout comparison (AC-2).
4. Route unmet needs to the Ze-image guest or `not-run`; move the four Alpine guest kinds to Ze's image and delete `run_iso.go`.
5. Give each registering package a per-test scope; turn on `-count=2 -shuffle=on` in the unit gate.
6. Re-check every owned row (AC-10).

### Failure Routing
| Failure | Route |
|---------|-------|
| a test needs a variable the allow-list drops | register it with `env.MustRegister`; never widen the allow-list by hand |

## Design Insights
- The store rows show the shape of the fix: `ba67a8b89c` gave each test its own `ze.config.dir` and two rows stopped. The sandbox generalizes that one step to every inherited thing.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| one sandbox provider for every test entry | per-suite fixes, one per row | 60 rows, one mechanism; per-row fixes are how the backlog grew |
| unmet need is `not-run`, never `pass` | skip silently | a skip read as green is the `./le integration traffic` row |
| keep the shared checkout | a worktree per session | owner rule; the sandbox removes what tests take from the checkout instead |

## Known Limitations
- Toolchain-version rows (a host `golangci-lint` built by an older Go, the patched GOROOT cache) are host state too, but their fix is pinning tools, not sandboxing tests. They are not owned here.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| parallel-copies-collide-on-a-deterministic-port.md | 2026-10-04 | check_labeled_withdraw.go and bgp-labeled-withdraw-compatibility-frr/inject |
| parallel-copies-collide-on-a-deterministic-port.md | 2026-08-19 | test/plugin/rest-api-commands.ci |
| parallel-copies-collide-on-a-deterministic-port.md | 2026-08-19 | test/static, test/plugin, QEMU VM |
| parallel-copies-collide-on-a-deterministic-port.md | 2026-08-10 | test tooling |
| parallel-copies-collide-on-a-deterministic-port.md | 2026-08-24 | global_cleanup (test/interop-ipsec/lab.py (retired; now internal/le/interop |
| parallel-copies-collide-on-a-deterministic-port.md | 2026-09-17 | test/ipsec/ipsec-status-show.ci, the IKE NAT-T listener on UDP 4500 |
| parallel-copies-collide-on-a-deterministic-port.md | 2026-09-20 | the exabgp functional runner, its ze start client (internal/test/cli/cmd_ex |
| test-artifacts-land-in-the-repository-root.md | 2026-10-08 | demos/terminal/database.lock, native job input fingerprint |
| test-artifacts-land-in-the-repository-root.md | 2026-09-07 | (*Runner).runTest and (*parsingRunner).runLegacyTest (internal/test/runner/ |
| test-artifacts-land-in-the-repository-root.md | 2026-09-03 | cmd/ze/hub/meta/ca/{key,cert} |
| test-artifacts-land-in-the-repository-root.md | 2026-08-29 | internal/test/fixture/bounded.json and full.json |
| test-artifacts-land-in-the-repository-root.md | 2026-09-13 | the plugin fixtures, os.WriteFile("daemon.ready", nil, 0o600) in internal/t |
| registry-contamination.md | 2026-08-10 | bgp reactor tests |
| registry-contamination.md | 2026-09-12 | RegisterOperationDecomposer (internal/component/config/transaction/operatio |
| registry-contamination.md | 2026-10-18 | internal/component/iface tests under go test -count=2 |
| registry-contamination.md | 2026-10-21 | reactor tests sharing bufMuxStd / bufMuxExt and one byte budget |
| suite-shares-one-persistent-store.md | 2026-08-24 | cmd/ze/ze_core_start.go startConfigPath branch, with resolveKey (internal/c |
| suite-shares-one-persistent-store.md | 2026-10-03 | plugin/clamped-path far daemons (startFarDaemons, internal/test/fixture/plu |
| test-shares-a-process-global.md | 2026-10-21 | TestRFC7311ExtendedNextHopCostRecovery (internal/component/bgp/reactor/rfc7 |
| test-shares-a-process-global.md | 2026-10-21 | TestImplicitDiscardReadsWhatTheLastApplyInstalled (internal/component/ike/e |
| container-build-leaves-root-owned-scratch.md | 2026-08-24 | tmp/kernel/build, written by the containerized kernel build and cleaned by |
| container-build-leaves-root-owned-scratch.md | 2026-09-05 | setupBinShims (internal/test/runner/runner.go), reached by every .ci te |
| check-mode-mutates-the-tree.md | 2026-09-14 | TestAMissingLLMSInputStopsTheBuild (internal/le/site/derived_test.go) |
| concurrent-session-corruption.md | 2026-10-05 | cmd/ze/ze_le_personality_test.go, assertInvocationPair |
| concurrent-session-corruption.md | 2026-10-05 | Native site race, roadmap history |
| concurrent-session-corruption.md | 2026-08-26 | compiled le UI fixtures, beginning with leDiscoveryAnswers (internal/test/f |
| concurrent-session-corruption.md | 2026-10-05 | Browser.agentEnv, internal/component/web/testing/runner.go |
| concurrent-session-corruption.md | 2026-10-09 | checkNoExportBoundary assertion 1 (internal/le/interoplab/bgp/check_rfc.go) |
| false-synchronization-claim.md | 2026-10-05 | reactor.TestReceiveCollapseLogsDiscardedMalformedAS4Path |
| package-red-not-reproduced.md | 2026-10-15 | test/ipsec/ipsec-peer-reload-applies-selectors.ci in the QEMU arm64 guest |
| gate-verdict-depends-on-the-machine.md | 2026-10-21 | ./le test qemu run, all-tests, stress, mobike (run.go, run_iso.go); ze appl |
| gate-verdict-depends-on-the-machine.md | 2026-10-20 | the Ze host kernel (7.2.9-ze) |
| gate-verdict-depends-on-the-machine.md | 2026-10-20 | ./le test exabgp api run directly (internal/test/cli), api-healthcheck-modu |
| gate-verdict-depends-on-the-machine.md | 2026-10-18 | publishPluginRegistry (internal/le/site/build.go), ./le site build |
| gate-verdict-depends-on-the-machine.md | 2026-10-12 | TestInstallNativeKernelFixtures, kernelQEMUArchAliasFixture (internal/test/ |
| gate-verdict-depends-on-the-machine.md | 2026-10-10 | appArmorProfiles (internal/le/interoplab/kernelcheck.go) |
| gate-verdict-depends-on-the-machine.md | 2026-10-10 | TestWebTemplPortFidelity (internal/component/web/port_check_test.go, via go |
| gate-verdict-depends-on-the-machine.md | 2026-10-09 | mobike-initiator, mobike-responder (test/interop-ipsec/) |
| gate-verdict-depends-on-the-machine.md | 2026-10-05 | internal/le/scratch/storetrim_test.go, TestTrimWriterRace |
| gate-verdict-depends-on-the-machine.md | 2026-10-03 | TestRFC5187InterfaceIDPreservedAcrossRestart (internal/plugins/ospf/rfc5187 |
| gate-verdict-depends-on-the-machine.md | 2026-09-15 | TestTheCommittedIndexStatesTheLiveWiki (internal/le/site/wiki/index_test.go |
| gate-verdict-depends-on-the-machine.md | 2026-09-13 | TestExtractCrashDumpReadsTheSubtree (internal/component/config/system/crash |
| gate-verdict-depends-on-the-machine.md | 2026-09-07 | ./le integration traffic (internal/le/integration/gates.go), against the CA |
| gate-verdict-depends-on-the-machine.md | 2026-09-03 | ./le functional vpp on macOS |
| gate-verdict-depends-on-the-machine.md | 2026-08-31 | test/ui/le-ste-answers.ci on this workstation |
| gate-verdict-depends-on-the-machine.md | 2026-08-29 | ./le integration interop-ipsec on macOS |
| gate-verdict-depends-on-the-machine.md | 2026-08-11 | qemu integration |
| gate-verdict-depends-on-the-machine.md | 2026-08-14 | ipsec functional suite |
| gate-verdict-depends-on-the-machine.md | 2026-08-18 | the retired ze-functional-appliance-test (current: ./le functional applianc |
| gate-verdict-depends-on-the-machine.md | 2026-08-22 | test/exabgp-compat/bin/bgp, ./le functional exabgp-test |
| gate-verdict-depends-on-the-machine.md | 2026-09-06 | internal/le/qemu/hugepages.go, the driver behind test/appliance/vpp-hugepag |
| gate-verdict-depends-on-the-machine.md | 2026-09-08 | TestReclaimRunsOnlyForAVMBackedRuntime (internal/appliance/kernelbuilder/sp |
| gate-verdict-depends-on-the-machine.md | 2026-09-17 | internal/le/qemu/hugepages.go behind test/appliance/vpp-hugepages-qemu.ci |
| gate-verdict-depends-on-the-machine.md | 2026-09-25 | ./le verify worktree over 0c2be06943 on darwin |
| gate-verdict-depends-on-the-machine.md | 2026-09-26 | test/plugin/evpn-config-self.ci on darwin |
| gate-verdict-depends-on-the-machine.md | 2026-09-28 | all 5 of test/rsvpte/; test/reload/ rsvpte-reload, config-apply-ordering-co |
| gate-verdict-depends-on-the-machine.md | 2026-09-28 | test/ui/display-fill-completion.ci, displayFill read loop (internal/test/fi |
| gate-verdict-depends-on-the-machine.md | 2026-10-03 | internal/plugins/geodns/rfc2181_clarifications_test.go TestRFC2181ReplySour |
| gate-verdict-depends-on-the-machine.md | 2026-10-10 | test/plugin/redistribute-as112-announce.ci |
| gate-verdict-depends-on-the-machine.md | 2026-10-21 | ensureCache (internal/core/env/env.go) |

## Rows found already fixed

Another agent closes these in the journal; this spec does not edit `plan/journal/`.

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| registry-contamination.md | 2026-09-13 | TestRegisteredLocalCommandsStayReachable (cmd/ze/internal/cmdutil) | the synthetic subtest with `defer registry.ResetForTest()` is gone; no `_test.go` calls `command/registry.ResetForTest` |
| suite-shares-one-persistent-store.md | 2026-08-20 | functional runner | `runner_exec.go` sets `ze.config.dir=<rec.WorkDir>` for clients and daemons (`ba67a8b89c`) |
| suite-shares-one-persistent-store.md | 2026-08-22 | DefaultConfigDir under the functional runner | same: the variable is now set, so `EvalSymlinks` never reaches the developer store from a runner child |

## Checklist

### Goal Gates (MUST pass)
- [ ] Tests written
- [ ] Tests FAIL before the sandbox
- [ ] Tests PASS after it
- [ ] `./le test unit` over `internal/test/runner` and the registering packages, under `./le job run`
- [ ] `./le test plugin` and `./le test ipsec` in the Ze-image guest, run by the main thread or a fresh agent
