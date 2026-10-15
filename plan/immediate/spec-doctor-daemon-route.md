# Spec: doctor-daemon-route

| Field | Value |
|-------|-------|
| Status | design |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The offline `ze doctor` runs in the operator's own process and reaches no
daemon (`internal/component/doctor/register.go`, `Mode: "offline"`), so a
plugin's runtime doctor check, the Stage-1 `DoctorChecks` declaration answered
by `OnDoctorCheck` in the plugin's own process (`ospf-state` today), is seen
only by the daemon-side `show doctor`. An operator who runs `ze doctor` on a
router whose daemon holds a runtime problem is told the box is fine.

Owner decision, Thomas 2026-10-15, answering VRRP Q9 ("route to the daemon if
running"): in its default mode the offline `ze doctor` also asks a running
daemon for the plugin runtime checks `show doctor` already runs, merges their
diagnostics after its own, and when no daemon answers prints its offline results
with one line saying the daemon checks were skipped, why, and the fix.

The design was written as phase D0 of
`plan/immediate/spec-vrrp-owner-arp-rfc-defects.md` and split out by the main
thread on 2026-10-15, because it changes shared surfaces (the `ze doctor` output
contract, every doctor `.ci`, the `doctor-check` sentence of
`docs/architecture/api/process-protocol.md`) and every plugin's runtime check
uses it. VRRP only consumes it: that spec's phase D3 depends on this one, and
its AC-31 (the offline `ze doctor` shows `doctor-vrrp-neigh-request-lost` when
the daemon answers) is proven through this route.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/doctor-and-health-checks.md` - the three self-diagnosis tiers, and where each owner registers its check
  → Constraint: read 2026-10-15: `ze doctor` is "Offline readiness ... in the operator's own process" (tier table, line 7); `show doctor` "reaches the same checks through a provider" (`diagnostic.RegisterDoctorProvider`)
  → Decision (owner, 2026-10-15, "route to the daemon if running"): the tier table gains the sentence that `ze doctor` adds a running daemon's plugin runtime checks when one answers, and says so when none does
- [ ] `docs/architecture/api/process-protocol.md` - the `doctor-check` callback
  → Constraint: line 913 reads "Only invoked at runtime via `show doctor`; offline `ze doctor` does not reach plugins"; this spec makes it false, so its edit lands with the route (`ai/rules/documentation.md`)
- [ ] `docs/guide/health-checks.md` - the operator page for both verbs, "External plugins (RPC)", and the code table
  → Constraint: the `doctor-gtsm-kernel-state` row is the precedent for a row stating two tiers
- [ ] `docs/features/ai-first.md` - declared by `internal/component/doctor/doctor.go` and `internal/core/diagnostic/types.go`; the agent-facing doctor output contract the `runtime` object joins
- [ ] `docs/architecture/api/commands.md` - the `show doctor` RPC handler the `runtime` keyword changes
- [ ] `docs/guide/command-reference.md` - named by `ai/CODE-TO-DOCS.md` for `internal/component/cli/sshclient/client.go`; the `ze doctor` and `show doctor` entries
- [ ] `docs/architecture/core-design.md` - declared by `internal/component/cli/client/main.go`, whose unauthorized recognition moves to the shared helper
  → Constraint: components reach each other through registries and the CLI's SSH session, never by import; the route crosses to the daemon only over SSH
- [ ] `ai/rules/cli.md` and `ai/patterns/` - keyword-before-value grammar and the YANG shape of a command keyword, read at implementation for the `runtime` keyword
- [ ] `ai/rules/user-facing-errors.md` - each skip line names the problem, why, and the exact fix command

**Key insights:**
- One fact, one producer: the daemon's `collectPluginDoctorChecks` already answers `show doctor`; the route adds a second reader, never a second producer (`ai/rules/principles.md`, declared once).
- The daemon is asked for its plugin runtime checks only, because a plain `show doctor` re-runs every offline check in the daemon's process and each would print twice.
- The route's outcome is a field of the result, never a diagnostic, so `ready` and the exit code keep today's meaning.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/doctor/doctor.go` `Run`, `runChecks`, `outputJSON`, `outputText` - read 2026-10-15: `Run` parses `--json`, `kernel-capabilities` and `config <file>`; kernel-capabilities returns `runKernelCapabilities`; otherwise `runChecks(configPath)`, then `ready` is false exactly when a diagnostic has `SeverityError`, and both output forms exit 1 only then; `diagnostic.Severity` has only `error` and `warning`
- [ ] `internal/component/doctor/cmd/show.go` `HandleShowDoctor`, `collectPluginDoctorChecks` - read 2026-10-15: registered as `ze-doctor:show-doctor` (`cmd/register.go`); runs `diagnostic.RunDoctorChecks(configPath)` IN THE DAEMON, then, when a server is present, `collectPluginDoctorChecks`, which walks `Server.DoctorCheckPlugins()`, skips the calling plugin, and calls each declared check one after another in a loop, each under `perCheckTimeout` (5 s), a failed call reported as `doctor-plugin-unreachable`; the handler reads `args[0]` as a config path
- [ ] `internal/component/doctor/yang/ze-doctor-cmd.yang` - read 2026-10-15: `show doctor` is a container with `ze:command "ze-doctor:show-doctor"` and no argument leaf; revision 2026-06-06
- [ ] `internal/core/diagnostic/types.go` - `SchemaVersion = 1`; `DoctorResult` carries `schema-version`, `ready` and the diagnostics
- [ ] `internal/component/cli/sshclient/client.go` `LoadCredentialsNoPrompt`, `ExecCommand`, `ExecCommandRaw`; `answer.go` `ExecCommandStream`, `dialDaemon` - read 2026-10-15: credentials come from the zefs store `ResolveStoreDir("")` names; `LoadCredentialsNoPrompt` never blocks on a password prompt and fails with "no password source" instead; `dialDaemon` bounds the dial with `dialTimeout` (10 s) and nothing bounds the command after it
- [ ] `internal/component/cli/client/main.go` `Run` - the `show version` reachability probe and the recognition of `plugin.UnauthorizedMessage`, which proves the daemon runs
- [ ] `internal/plugins/signal/main.go`, `internal/component/config/cli/reload_notify.go` - the precedents of a local verb sending one command to the daemon
- [ ] `test/ui/doctor-*.ci` (52 files), `test/vrrp/vrrp-doctor*.ci` (3 files) - the expectations an added `daemon checks:` line or `runtime` key may break; `test/ui/doctor-without-store.ci` runs `ze doctor --json config probe.conf`, config mode

**Behavior to preserve:**
- Every offline diagnostic, its order, its text and its JSON shape
- `ready` and the exit code: 1 exactly when a diagnostic shown is an error
- `ze doctor kernel-capabilities` output, unchanged
- `show doctor` without the keyword returns today's offline plus plugin set; the per-check 5 s timeout and `doctor-plugin-unreachable` carry over unchanged
- `ze doctor` never prompts: it runs in scripts and provisioning

**Behavior to change:**
- `ze doctor` default mode (no config file, not kernel-capabilities) asks a running daemon for its plugin runtime checks and merges them
- the result gains a `runtime` status (ran, skipped, not-run) in both output forms
- `show doctor` gains the `runtime` keyword, which returns only the plugin runtime checks

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze doctor [--json]` typed by an operator or a script, with no config file named
- `show doctor runtime` arriving at the daemon over the CLI's SSH session

### Transformation Path
1. `Run` runs `runChecks` as today and keeps its diagnostics
2. `Run` loads CLI credentials without a prompt from the store the run resolved
3. an exec seam (the tests replace it) sends `show doctor runtime` through `sshclient`, one bound covering dial and answer
4. the daemon's `HandleShowDoctor`, seeing the keyword, skips `diagnostic.RunDoctorChecks` and returns only `collectPluginDoctorChecks`'s diagnostics, with `runtime.status` `ran`
5. `Run` decodes the `DoctorResult`, appends its diagnostics after its own, and records the runtime status; a failure at steps 2 to 5 records `skipped` with its cause, reason and fix
6. `ready` is computed over the merged diagnostics; `outputText` prints one `daemon checks:` line before the `ready` line, `outputJSON` writes the `runtime` object

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| operator process ↔ daemon | SSH exec of `show doctor runtime` with CLI credentials, `DoctorResult` JSON back | No |
| daemon ↔ plugin | the existing `ze-plugin-callback:doctor-check` callback, unchanged | No |

### Integration Points
- `sshclient.LoadCredentialsNoPrompt` - the only credential path, so no prompt
- `sshclient` exec - a bounded form, if none exists, so one bound covers dial and command
- `plugin.UnauthorizedMessage` - recognized through one helper the CLI client and the route both call, never a second string match
- `collectPluginDoctorChecks` - the one producer of plugin runtime diagnostics

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the route reaches plugins only through the daemon's existing callback path |
| No unintended coupling (components stay isolated) | Yes | the doctor component calls `sshclient`, as `signal` and `reload_notify` do; no plugin is imported |
| No duplicated functionality (extends existing, does not recreate) | Yes | `collectPluginDoctorChecks` is reused; `DoctorCheckContext` gains no field |
| Zero-copy preserved where applicable (refs, not copies) | N-A | not a wire path |
| Registration over hardcoding, outbound | Yes | the `runtime` keyword is a YANG node of the doctor's own command module |
| Registration over hardcoding, inbound | Yes | a plugin's runtime check reaches `ze doctor` through its existing Stage-1 declaration; the route names no plugin and no check |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No caller passes a config path to `show doctor` as `args[0]`, so the `runtime` keyword replaces that positional reading rather than living beside it | `HandleShowDoctor` reads `args[0]` as a config path while `ze-doctor-cmd.yang` declares no argument | the keyword and a path collide; the positional reading is removed first (`ai/rules/no-layering.md`) and its callers named here | grep of `show doctor` callers and the YANG-validated argument path at implementation | unvalidated |
| A-2 | `sshclient` can carry one bound over dial and answer without a new transport | `dialDaemon` bounds only the dial; `ExecCommandStream` exists | a bounded exec is added to `sshclient` (Files to Modify) | read at implementation | unvalidated |
| A-3 | The store `ze doctor` resolves is the store the CLI credentials live in | `LoadCredentialsNoPrompt` reads the store `ResolveStoreDir("")` names | the route reads the wrong store; the skip reason names the directory it read | `TestDoctorDaemonRouteSkipped` case (1) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The daemon runs plugin checks one after another, each up to 5 s, so its answer can exceed the route's bound on a host with many declared checks | the bound case of `TestDoctorDaemonRouteSkipped` passes while a real daemon with several slow checks is reported skipped | O-1 decided (b): checks of different plugins run concurrently (AC-5), so the answer takes one per-check bound however many plugins declare checks; checks of ONE plugin stay serial (the SDK answers callbacks one at a time), so a plugin declaring several slow checks can still exceed the route's bound (Known Limitations) |
| R-2a | A per-request goroutine fan-out in `collectPluginDoctorChecks` breaks `ai/rules/goroutine-lifecycle.md` (a goroutine per message is forbidden, and `writeGoPatterns` refuses `go func(`) | the pre-write hook or review refuses the handler | one long-lived doctor worker per plugin process, started and stopped with that process (Key Design Decisions, O-1 row) |
| R-2 | An added `daemon checks:` line or `runtime` key breaks an existing doctor `.ci` | the functional suite reds on a doctor test | each affected `.ci` updated to expect the line, never loosened (`ai/rules/testing.md`) |
| R-3 | A test host runs a real ze daemon, so a `.ci` meant to see "skipped" sees "ran" | a skip-path `.ci` flakes depending on the host | the skip `.ci` points the store at its own temporary directory with credentials naming a port nothing listens on |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `ze doctor` output and exit code, which scripts and provisioning read; a hang if the bound fails |
| How is it reverted? | single commit revert |
| Who else touches this path? | `plan/immediate/spec-vrrp-owner-arp-rfc-defects.md` (its AC-31 and phase D3 consume the route); every plugin declaring a Stage-1 doctor check (`ospf-state` today) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze doctor` (no config file) on a host whose daemon answers | → | doctor `Run`, the route (credentials, exec of `show doctor runtime`), daemon `HandleShowDoctor` runtime mode, `collectPluginDoctorChecks` | `TestDoctorDaemonRouteMergesRuntimeChecks` (unit) and `test/ui/doctor-daemon-runtime.ci` (functional) |
| `ze doctor` (no config file) on a host where no daemon answers | → | doctor `Run`, the route's skip path | `TestDoctorDaemonRouteSkipped` (unit) and `test/ui/doctor-daemon-skipped.ci` (functional) |
| `show doctor runtime` on the daemon | → | `HandleShowDoctor` runtime mode | `TestShowDoctorRuntimeOnly` (unit) |
| `show doctor` and `show doctor runtime` on a daemon with several plugins declaring checks | → | `collectPluginDoctorChecks` handing each plugin's checks to that plugin's doctor worker | `TestCollectPluginDoctorChecksConcurrent` (unit) and `test/ui/doctor-daemon-runtime.ci` (functional, the ran path through the workers) |
| `ze doctor config <file>`, `ze doctor kernel-capabilities` | → | doctor `Run`, no route | `TestDoctorConfigModeTakesNoRoute` (unit) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The offline `ze doctor` and `ze doctor --json`, with no config file named, on a host whose daemon runs and holds a plugin whose Stage-1 runtime check reports a diagnostic (moved from the generic half of VRRP AC-31) | the output carries that diagnostic once, after the offline diagnostics, with the message `show doctor` carries; the offline diagnostics appear once each (the daemon is asked for its plugin runtime checks only, never for its own run of the offline checks); text output prints `daemon checks: ran` and JSON carries `"runtime": {"status": "ran"}`; the exit code is 1 exactly when some diagnostic, offline or runtime, is an error, so a runtime warning alone leaves it 0; a runtime check the daemon reports as `doctor-plugin-unreachable` passes through unchanged |
| AC-2 | (was VRRP AC-32) the offline `ze doctor` and `ze doctor --json`, no config file, in each case the route cannot be taken: (1) no store at the resolved store directory; (2) a store with no CLI credentials, or a password only a prompt could supply; (3) credentials, but nothing answers at the daemon address (connection refused or dial timeout); (4) the daemon answers and refuses `show doctor runtime` for the user (`plugin.UnauthorizedMessage`); (5) the daemon accepts and does not answer within the route's bound | the offline diagnostics exactly as today; no runtime diagnostic; ONE line, text `daemon checks: skipped: <why>; <fix>`, JSON `"runtime": {"status": "skipped", "reason": "<why>", "fix": "<fix>"}`, where each case names its own why and an exact fix command: (1) and (2) no daemon credentials in `<store dir>`, fix `ze init` (or `--user`, read at implementation); (3) no daemon answered at `<host>:<port>`, fix the command that starts ze on that host (O-2: `sudo systemctl start ze.service` when the `ze.service` unit is installed, `sudo ze install systemd --start` on a systemd host without it, `ze start` otherwise) or run `ze show doctor` on the box that runs it; (4) user `<name>` is not authorised for `show doctor`, fix naming the authz profile command; (5) the daemon did not answer within `<bound>`, fix `ze show doctor`; the skip is not a diagnostic, so it never changes `ready` and the exit code keeps today's meaning; `ze doctor` never blocks on a password prompt (`LoadCredentialsNoPrompt`) |
| AC-3 | (was VRRP AC-33) `ze doctor config <file>`, `ze doctor --json config <file>` and `ze doctor kernel-capabilities`, with a daemon running | no route is taken: the file named need not be the configuration the daemon runs, so its judgement and the daemon's live state are not mixed; config mode prints `daemon checks: not run: a config file was named; run ze doctor with no config file, or ze show doctor, to include them` and JSON carries `"runtime": {"status": "not-run", ...}` with that reason and fix; the kernel-capabilities output is unchanged |
| AC-4 | `show doctor runtime` and `show doctor` on the daemon | with the keyword, only the plugin runtime checks' diagnostics, no offline check's code, and `"runtime": {"status": "ran"}`; without it, today's offline plus plugin set, also carrying `"runtime": {"status": "ran"}` and otherwise unchanged |
| AC-5 | (O-1 (b), main thread 2026-10-15) `show doctor` or `show doctor runtime` on a daemon where several plugins declare Stage-1 doctor checks, some slow, one never answering | checks of different plugins run concurrently, each still under its own `perCheckTimeout` (5 s); checks of one plugin run one after another in their `Order`, because the SDK answers a plugin's callbacks serially (`pkg/plugin/sdk/sdk_dispatch.go`, `bridgeEventLoop`), so a check's timeout never runs while it waits behind its sibling; two plugins each taking most of one bound answer together within one bound, not the sum; a plugin that never answers yields `doctor-plugin-unreachable` for its check at its own bound and delays no other plugin's result; the diagnostics come out in one deterministic order: by `Order`, then plugin name, then check name (today's `sort.Slice` over map iteration leaves equal `Order` in random order), whatever order the answers arrive in; the route's bound (`ze doctor`, AC-2 cause (5)) is the SSH dial timeout plus one `perCheckTimeout` plus a small margin; no goroutine is started per request: each plugin process that declares checks owns one long-lived doctor worker, started with the process and stopped when it stops |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator runs `ze doctor` on a router whose daemon holds a plugin runtime problem and sees it among the offline results | `ze doctor` → doctor `Run` → route → daemon `HandleShowDoctor` runtime mode → `doctor-check` callback → plugin handler | `test/ui/doctor-daemon-runtime.ci` (AC-1); for VRRP, the `ze doctor` step of the `vrrp-keepalived` lab leg (VRRP AC-31) |
| 2 | An operator runs `ze doctor` on a host where no daemon answers and learns the runtime checks did not run, why, and the command that fixes it | `ze doctor` → doctor `Run` → route skip | `test/ui/doctor-daemon-skipped.ci` (AC-2) |
| 3 | An operator checks a config file with `ze doctor config <file>` and is told the daemon's checks were not mixed in | `ze doctor config` → doctor `Run`, no route | the updated config-mode `test/ui/doctor-*.ci` files (AC-3) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoctorDaemonRouteMergesRuntimeChecks` (new, untagged: `Run` with the route's exec seam answering a `show doctor runtime` result holding one warning and one error; the output holds the offline diagnostics once and the two runtime ones once, after them; `runtime.status` is `ran`; exit 1 because of the runtime error, exit 0 when the seam returns only the warning; red with the route not called, red with the daemon asked for plain `show doctor` (each offline code appears twice)) | `internal/component/doctor/doctor_test.go` | AC-1 | [ ] |
| `TestDoctorDaemonRouteSkipped` (new, untagged, one case per AC-2 cause: no store, no credentials, password needing a prompt, connection refused, unauthorized answer, no answer within the bound on a seam that never returns; each yields the offline diagnostics unchanged, no runtime diagnostic, `runtime.status` `skipped` with that cause's reason and fix, and the exit code the offline diagnostics alone give; the timeout case returns within the bound plus a margin; red with the skip rendered as a warning diagnostic (the `ready` and count line change), red with the reason empty) | `internal/component/doctor/doctor_test.go` | AC-2 | [ ] |
| `TestDoctorConfigModeTakesNoRoute` (new, untagged: `config <file>` and `kernel-capabilities` with a seam that fails the test if called; config mode carries `runtime.status` `not-run` with its reason and fix) | `internal/component/doctor/doctor_test.go` | AC-3 | [ ] |
| `TestShowDoctorRuntimeOnly` (new, untagged: `HandleShowDoctor` with the `runtime` keyword over a fake server with one plugin check returns only that check's diagnostics, no offline check's code, and `runtime.status` `ran`; without the keyword the result is today's offline plus plugin set) | `internal/component/doctor/cmd/show_test.go` | AC-1, AC-4 | [ ] |
| a test of the shared unauthorized helper, both callers (CLI client and route) reaching it | `internal/component/cli/sshclient/` | AC-2 (4) | [ ] |
| `TestCollectPluginDoctorChecksConcurrent` (new, untagged, over a fake server whose check calls block on a controllable clock or channel, with `perCheckTimeout` reachable from the test as a shortened value): (1) two plugins each taking 0.8 of one bound both answer and the whole call returns within one bound plus a margin, not two; (2) one plugin that never answers yields exactly one `doctor-plugin-unreachable` for its check, returned at its own bound, while a fast plugin's diagnostics arrive unchanged; (3) one plugin with two checks runs them one after another (the fake records that no two calls to the same plugin overlap), each under its own bound; (4) answers released in reverse order still come out by `Order`, then plugin name, then check name, and two plugins with equal `Order` come out in the same order across repeated runs; red with the serial loop restored (case 1 takes two bounds), red with one shared deadline across a plugin's checks (case 3's second check times out) | `internal/component/doctor/cmd/show_test.go` | AC-5 | [ ] |
| `TestDoctorWorkerLifecycle` (new, untagged: a plugin process declaring checks gets one doctor worker when it starts and none when it declares none; after the process stops the worker has exited (a goroutine count or done channel the test reads, not a sleep); a request queued to a stopped or busy worker is answered `doctor-plugin-unreachable` at once, never blocks the collector; a late answer from a check that already timed out never blocks the worker; red with the worker left running after stop) | the package that owns the worker, chosen at implementation (Files to Modify) | AC-5 | [ ] |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| route bound | SSH dial timeout + one `perCheckTimeout` + a small margin (O-1 (b)) | an answer just inside the bound is `ran` | N/A | an answer just past the bound is `skipped` cause (5) |
| daemon answer time with N plugins, one check each | each check up to one `perCheckTimeout` | N plugins each just inside one bound answer within one bound | N/A | a plugin past its bound is `doctor-plugin-unreachable`, the others unchanged |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/ui/doctor-daemon-skipped.ci` (new, AC-2): `ze doctor` and `ze doctor --json` over a store with credentials and no daemon listening print the offline results, one `daemon checks: skipped: no daemon answered at` line naming the address and its fix, and exit as the offline results decide; red with the route removed (no line) | `test/ui/` | an operator on a host with no daemon learns the runtime checks did not run and how to run them | [ ] |
| `test/ui/doctor-daemon-runtime.ci` (new, AC-1; added at the split, 2026-10-15, because D0 proved the ran path only through the VRRP guest lab): a daemon started in the test with an SSH block and a plugin declaring a Stage-1 doctor check that reports a diagnostic (which plugin, read at implementation from the `ospf-state` precedent and the harness's daemon support), then `ze doctor` and `ze doctor --json` show that diagnostic once after the offline ones and `daemon checks: ran`; red with the route removed | `test/ui/` | an operator sees a runtime problem from `ze doctor` | [ ] |
| every existing `test/ui/doctor-*.ci` and `test/vrrp/vrrp-doctor*.ci` whose `ze doctor` default-mode or config-mode output an added `daemon checks:` line or `runtime` key would break, re-read at implementation and updated to expect it, never loosened (`ai/rules/testing.md`) | `test/ui/`, `test/vrrp/` | AC-2, AC-3 | [ ] |

### Interop Tests (Scope: protocol)
N-A: Scope is cli; no wire protocol peer.

## Files to Modify
- `internal/component/doctor/doctor.go` - `Run` in its default mode (no config file, not kernel-capabilities) takes the route after `runChecks`: load credentials without a prompt from the store the run resolved, send `show doctor runtime` through an exec seam the tests replace, under one bound for the whole exchange, decode the `DoctorResult`, append its diagnostics, and record the runtime status (ran, skipped with the AC-2 cause, not-run in config mode); `outputText` prints the one `daemon checks:` line before the `ready` line, and `outputJSON` writes the `runtime` object; `ready` and the exit code are computed over the merged diagnostics only
- `internal/component/doctor/cmd/show.go` - a second handler, registered as `ze-doctor:show-doctor-runtime` beside `ze-doctor:show-doctor` (`cmd/register.go`), returns only `collectPluginDoctorChecks`'s diagnostics, skipping `diagnostic.RunDoctorChecks`; both forms set `runtime.status` `ran`; the `args[0]` config-path reading is resolved under A-1; `collectPluginDoctorChecks` stops looping: it hands each plugin's checks, in `Order`, to that plugin's doctor worker without blocking, waits for each plugin's answers under that plugin's per-check bounds, reports an unanswered or refused hand-off as `doctor-plugin-unreachable`, and emits the diagnostics sorted by `Order`, plugin name, check name (AC-5)
- the doctor worker (AC-5) - one long-lived worker per plugin process that declares doctor checks, reading a buffered request channel, running that plugin's checks one after another each under its own `perCheckTimeout`, answering on a reply channel buffered so a late answer never blocks it; started with the process and stopped by closing its channel when the process stops, the sequence stated in the owning type's doc comment (`docs/contributing/ze-go-style.md`, "Goroutines"). Placement read at implementation: the plugin server beside `Server.CallDoctorCheck` (`internal/component/plugin/server/server.go`), which already holds the per-process handle, is the expected owner, because the doctor component holds no process lifecycle to bind a worker to
- `internal/component/doctor/yang/ze-doctor-cmd.yang` - `container doctor` gains a nested `container runtime { config false; ze:command "ze-doctor:show-doctor-runtime"; ... }` with description and `ze:help`, the shape `show ospfv3 interface detail` uses (`internal/plugins/ospf/yang/ze-ospf-cmd.yang`, the nested `container detail` carrying its own `ze:command`), with a revision statement (O-3)
- `internal/core/diagnostic/types.go` - `DoctorResult` gains the `runtime` object (status, reason, fix); `SchemaVersion` stays 1 (O-4)
- `internal/component/cli/sshclient/` - a bounded form of `ExecCommand` if none exists, so the route's single bound covers the dial and the command; the unauthorized refusal recognized as the CLI client recognizes it (`plugin.UnauthorizedMessage`), from one helper both callers use rather than a second string match
- `internal/component/cli/client/main.go` - its unauthorized recognition moves to the shared helper
- `docs/architecture/doctor-and-health-checks.md` - the tier table: `ze doctor` adds a running daemon's plugin runtime checks when one answers, and says so when none does; the route's design paragraph (why only the runtime checks are asked for, the bound, the three statuses, the exit code); the daemon's plugin checks run concurrently across plugins and serially within one, through one doctor worker per plugin process, in a deterministic output order (AC-5)
- `docs/architecture/api/process-protocol.md` - the `doctor-check` text "Only invoked at runtime via `show doctor`; offline `ze doctor` does not reach plugins" becomes false: it says a plugin's runtime check is invoked by the daemon for `show doctor` and for `show doctor runtime`, which the offline `ze doctor` sends when a daemon answers
- `docs/guide/health-checks.md` - a paragraph on the `daemon checks:` line and the `runtime` JSON object (ran, skipped, not-run, each reason and fix)
- `docs/guide/command-reference.md` - the `runtime` keyword and the `daemon checks:` line
- `docs/architecture/api/commands.md` - `show doctor runtime`
- `docs/features/ai-first.md` - the `runtime` object in the agent-facing doctor contract
- the existing `test/ui/doctor-*.ci` and `test/vrrp/vrrp-doctor*.ci` files whose expectations the new line or key changes (TDD plan)

## Files to Create
- `test/ui/doctor-daemon-skipped.ci` - AC-2
- `test/ui/doctor-daemon-runtime.ci` - AC-1
- `internal/component/doctor/cmd/show_test.go` - if absent, for `TestShowDoctorRuntimeOnly`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/doctor/yang/ze-doctor-cmd.yang`, the `runtime` keyword |
| YANG validation constraints | N-A | a keyword, no value |
| YANG custom validators | N-A | no value to validate |
| CLI commands/flags | Yes | `ze doctor` output gains the `daemon checks:` line; `show doctor` gains `runtime` |
| CLI grammar (keyword before value) | Yes | `runtime` is a keyword with no value (`ai/rules/cli.md`) |
| Editor autocomplete | Yes | automatic for the YANG node |
| Functional test for new RPC/API | Yes | `test/ui/doctor-daemon-runtime.ci`, `test/ui/doctor-daemon-skipped.ci` |
| Pipe completeness | Yes | `show doctor runtime` output passes through the same pipe path as `show doctor`, read at implementation |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | the route is the doctor itself; it adds no check |
| Prometheus counters/metrics | N-A | no observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `docs/features/ai-first.md` (the doctor output contract) |
| 2 | Config syntax changed? | No | no config leaf |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, `docs/guide/health-checks.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | no plugin changes |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md` |
| 7 | Wire format changed? | No | no wire format |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, the `doctor-check` sentence |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no RFC |
| 10 | Test infrastructure changed? | No | existing `.ci` harness, unless the runtime `.ci` needs daemon support it lacks (read at implementation) |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the `show doctor runtime` command: `docs/guide/status.md` if it lists doctor verbs, read at implementation |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/immediate/spec-doctor-daemon-route.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `ze doctor` example output in the pages above gains the `daemon checks:` line |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - validate A-1 and A-2; register the `runtime` keyword in YANG; the exec seam in `Run`; the four unit tests written and failing
   - Tests: `TestShowDoctorRuntimeOnly`, `TestDoctorDaemonRouteMergesRuntimeChecks`, `TestDoctorDaemonRouteSkipped`, `TestDoctorConfigModeTakesNoRoute`
   - Files: `ze-doctor-cmd.yang`, `doctor.go`, `cmd/show.go`
   - Verify: the entry points exist; the tests fail because the route is a stub
2. **Phase: Concurrent show doctor** - the per-plugin doctor worker and its lifecycle; `collectPluginDoctorChecks` handing checks to the workers and sorting the answers
   - Tests: `TestCollectPluginDoctorChecksConcurrent`, `TestDoctorWorkerLifecycle`
   - Files: `cmd/show.go`, the worker's owning package
   - Verify: tests fail against the serial loop → implement → tests pass; the existing `show doctor` tests stay green
3. **Phase: Route** - `DoctorResult.runtime`; the daemon's runtime mode; the bounded exec in `sshclient` with the shared unauthorized helper; the route in doctor `Run` with its three statuses and the `daemon checks:` line
   - Tests: the four unit tests and the helper test
   - Files: Files to Modify, Go rows
   - Verify: tests fail → implement → tests pass
4. **Phase: Functional and docs** - `test/ui/doctor-daemon-skipped.ci`, `test/ui/doctor-daemon-runtime.ci`, the existing doctor `.ci` expectations; the doc edits of Files to Modify
   - Tests: the functional rows
   - Verify: each new `.ci` red with the route removed, green with it

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | offline diagnostics appear once; the skip never changes `ready`; the bound covers dial and answer |
| Naming | JSON keys kebab-case: `runtime`, `status`, `reason`, `fix` |
| Data flow | plugin runtime diagnostics come only from `collectPluginDoctorChecks`; `DoctorCheckContext` unchanged |
| Rule: user-facing-errors | every skip line names the problem, why, and an exact fix command |
| Rule: no-layering | the `args[0]` config path reading is removed or proven unused, never kept beside the keyword; the serial loop is replaced by the workers, never kept as a fallback |
| Rule: goroutine-lifecycle | no `go func(`, no goroutine per request; each doctor worker has an owner, a start, and a stop path stated in its type's doc comment |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the route in `Run` | `TestDoctorDaemonRouteMergesRuntimeChecks` |
| five skip causes | `TestDoctorDaemonRouteSkipped`, `test/ui/doctor-daemon-skipped.ci` |
| `show doctor runtime` | `TestShowDoctorRuntimeOnly` |
| concurrent plugin checks, deterministic order | `TestCollectPluginDoctorChecksConcurrent`, `TestDoctorWorkerLifecycle` |
| process-protocol sentence corrected | grep `docs/architecture/api/process-protocol.md` for "does not reach plugins" returns nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Credentials | read only through `LoadCredentialsNoPrompt`; never printed in a skip reason |
| Authorization | `show doctor runtime` is authorized as `show doctor` is; a refusal is a skip, never a silent empty `ran` |
| Resource exhaustion | one bound on the whole exchange; a daemon that never answers cannot hang `ze doctor` |
| Error leakage | the skip reason names the store directory and address, never a password or key |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A-1 or A-2 broken | DESIGN, before Phase Route |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Open Items

Decided at implementation from their producers and recorded in Key Design
Decisions then, unless the row says the owner decides.

| ID | Item | Options | Recommendation |
|----|------|---------|----------------|
| O-1 | The route's bound. The daemon runs plugin checks one after another, each up to 5 s (`collectPluginDoctorChecks`, read 2026-10-15), and `dialDaemon` alone may take 10 s (`dialTimeout`) | (a) one fixed bound, the dial plus a margin over the per-check timeout, a slow daemon reported as skipped cause (5); (b) the daemon runs its plugin checks concurrently so its answer stays within one `perCheckTimeout`, and the route's bound is the dial plus that | DECIDED (b), main thread 2026-10-15: AC-5, Key Design Decisions |
| O-2 | The per-platform fix command for skip cause (3) | read from `ze start` and `ze install systemd` | ANSWERED 2026-10-15 from source: `ze start` is the root handler that starts the daemon from the stored active config (`cmd/ze/ze_core_dispatch.go`, `registerLocalCommands`); `ze install systemd` writes `/etc/systemd/system/ze.service` and takes `--start` (`internal/plugins/systemd/unit.go` `defaultUnitPath`, `cmd_install.go` flags), and the existing `doctor` fix text already names `sudo systemctl ...` for that unit (`internal/core/diagnostic/codes.go`). So the fix is `sudo systemctl start ze.service` when that unit file exists, `sudo ze install systemd --start` on a host with systemd and no unit, `ze start` otherwise. How "a host with systemd" is detected, and what the gokrazy appliance (whose daemon a supervisor runs) prints, is read at implementation |
| O-3 | The YANG shape of `runtime` | a presence container, an empty leaf, or a keyword node per `ai/patterns/` | ANSWERED 2026-10-15: a nested `container runtime` under `container doctor`, `config false`, carrying its own `ze:command "ze-doctor:show-doctor-runtime"`, the shape `internal/plugins/ospf/yang/ze-ospf-cmd.yang` uses for `show ospfv3 interface detail` and three other `detail` forms; a `type empty` leaf is an RPC input flag (`ze-data-cmd.yang`), not a show form. The keyword is then dispatched by the command tree to its own handler, so no handler reads `args[0]` for it, which also settles how A-1's positional reading is removed (`ai/rules/cli.md`, keywords before values; `ai/rules/no-layering.md`) |
| O-4 | Whether `SchemaVersion` moves for the added `runtime` object | per the contract's own rule on an added field | ANSWERED 2026-10-15: it stays 1. No rule on added fields exists: `internal/core/diagnostic/types.go` says only "the current diagnostic JSON contract version", and a grep of `docs/` and `ai/` for `schema-version` finds no versioning rule; no Go reader in the tree compares the value. The `runtime` object is an added key, which a reader that ignores unknown keys already accepts, and Ze is pre-release (`ai/rules/pre-release.md`). Writing a versioning rule into `docs/features/ai-first.md` is not in this spec's scope |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| ze doctor reaches the daemon (owner, 2026-10-15, "route to the daemon if running"): in its default mode the offline `ze doctor` runs its offline checks as today, then loads CLI credentials without a prompt from the store the run resolved and sends `show doctor runtime` to the daemon over SSH, under one bound covering dial and answer; the daemon answers with only its plugin runtime checks (`collectPluginDoctorChecks`, every Stage-1 `DoctorChecks` declaration of every running plugin, each under the existing 5 s `perCheckTimeout`); `ze doctor` appends those diagnostics after its own and records a runtime status. When the route cannot be taken (no store, no credentials, a password only a prompt could give, nothing listening, an unauthorized refusal, no answer within the bound) the offline results stand alone and ONE line says the daemon checks were skipped, why, and the exact fix (AC-2). The status is a field of the result (`runtime`: ran, skipped, not-run), never a diagnostic, so `ready` and the exit code keep today's meaning: exit 1 exactly when a diagnostic shown is an error. `config <file>` mode and `kernel-capabilities` take no route (AC-3). The route is generic and belongs to the doctor component | (1) ask the daemon for plain `show doctor` and merge: the daemon re-runs every offline check in its own process, so each would print twice, judged by two processes that can disagree (permissions, environment); (2) dedupe a plain `show doctor` answer by code: hides a disagreement between the two runs and leaves the work done twice; (3) the skip as a warning diagnostic: a host before its first start, the main use of `ze doctor`, would always warn, and the warning count would mean two things; (4) the skip printed nowhere: a silent absence reads as "no runtime problem" (`ai/rules/principles.md`, a value that is silently wrong); (5) route in config mode too: the file named is often not what the daemon runs, so its verdict and the daemon's live state would describe two different subjects in one report; (6) prompt for a password: `ze doctor` runs in scripts and provisioning, where a prompt hangs; (7) a `DoctorCheckContext` field carrying runtime state | the route reuses the producer `show doctor` already uses, so one fact has one producer and two readers (`ai/rules/principles.md`, declared once); the CLI's SSH credentials and the `signal` and `reload_notify` precedents mean no new transport; the daemon's per-check timeout and `doctor-plugin-unreachable` carry over unchanged |
| Concurrent `show doctor` (O-1 (b), main thread 2026-10-15): checks of different plugins run at the same time, each still under its own 5 s `perCheckTimeout`; checks of one plugin run in `Order` one after another; the output order is `Order`, plugin name, check name; the route's bound is the SSH dial timeout plus one `perCheckTimeout` plus a small margin. Concurrency comes from one long-lived doctor worker per plugin process that declares checks, started and stopped with that process | (a) one fixed route bound over the serial loop: the answer grows with the number of plugins, and cause (5) would report a busy daemon as if it were stuck; (c) one goroutine per check per request: `ai/rules/goroutine-lifecycle.md` forbids a goroutine per message, and `writeGoPatterns` refuses `go func(`; (d) concurrency within one plugin: the SDK answers a plugin's callbacks serially (`pkg/plugin/sdk/sdk_dispatch.go`, `bridgeEventLoop`), so a second check's timeout would expire while it queues behind the first; (e) a fixed pool of N workers owned by the doctor component: more plugins than workers brings the sum back, and the pool has no lifecycle of its own to bind to | the daemon's answer takes one bound however many plugins declare checks, so cause (5) means a stuck daemon; a worker per process is the "one goroutine for one lifecycle" shape the style guide permits; serial within a plugin matches what the SDK does anyway; the deterministic order fixes today's random order for equal `Order` (`sort.Slice` over map iteration) |
| Split from VRRP (main thread, 2026-10-15) | keep as phase D0 of `spec-vrrp-owner-arp-rfc-defects.md` | the route changes shared surfaces VRRP's review rounds should not carry; it lands and is reviewed while VRRP's D1 and D2 proceed; its blast radius is the doctor component |

## Known Limitations
- The route reports the daemon's state at the moment it answers; a runtime problem the plugin already repaired leaves no diagnostic.
- A host running no daemon gets the offline checks only, by design, with the skip line saying so.
- A plugin declaring several slow checks answers in up to their sum, because its checks run serially (AC-5), so the route can report that daemon as skipped cause (5). Every plugin in the tree declares one check today (`ospf-state` in `internal/plugins/ospf/register.go`, one in `internal/test/fixture/plugin_fixture_01_api.go`), read 2026-10-15.

## Design Review Log

| Round | Date | Scope | Result | Disposition |
|-------|------|-------|--------|-------------|
| - | 2026-10-15 | split | design moved from phase D0 of `plan/immediate/spec-vrrp-owner-arp-rfc-defects.md` (written in 6121017ede): AC-32 and AC-33 there become AC-2 and AC-3 here; AC-1 is the generic half of VRRP AC-31; AC-4 states the `show doctor` half AC-33 carried; `test/ui/doctor-daemon-runtime.ci`, the helper test, A-1 (the handler's `args[0]` config path) and O-1's serial-loop finding are new at the split | - |
| - | 2026-10-15 | O-1..O-4 | O-1 decided (b) by the main thread: AC-5, `TestCollectPluginDoctorChecksConcurrent`, `TestDoctorWorkerLifecycle`, the per-plugin doctor worker, phase "Concurrent show doctor"; O-2, O-3 and O-4 answered from source (Open Items) | - |
| r1 (review) | owed | the whole design (was VRRP r13), including AC-5 | - | owed |

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
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
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
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: Scope cli, no protocol peer)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/immediate/spec-doctor-daemon-route.md` only, in the same `./le commit create` script
