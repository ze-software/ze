# Spec: doctor-coverage-1-audit-and-gate

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md`: `ze doctor` reports every problem an operator can hit, with cause and fix; this child finds every problem class, gives runtime state a phase that cannot read healthy when unseen, and stops new classes landing without a check |
| RFC gaps | - |
| Depends | - |
| Phase | P1a, P1b, P1c, P2a, P2b landed (commit fc96556110 the gate, commit ce58dddf53 the doctor phases); P3 audit section files written, merge and area children in progress. Owner questions O-1, O-2, O-3 decided 2026-11-08 (see "Owner Decisions"); design approved by the owner order of 2026-11-10; phases in "Implementation Phases" |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Owner order, 2026-11-10: Thomas ordered this spec landed ("Land P1 and P2",
then "Complete the work"), which approves the design and permits the claim
above the WIP cap (`ZE_SPEC_WIP_CAP=25`) for this spec only. The audit of
this spec against the tree that day (`plan/handover/doctor-coverage-1-start.md`,
"Audit of the spec against the tree") is folded in below as dated notes and
added rows; no decided content was removed. Evidence is the scoped package
tests and the named `.ci` files only: no agent runs `./le verify` or a whole
functional suite (`ai/META-DECISIONS.md`, "Ze is pre-release").

Claim, 2026-11-10: claimed with `ZE_SPEC_WIP_CAP=25` under Thomas's orders
"Land P1 and P2", "Complete the work" and "Fix the issues found along the way
and continue with P2". P1 and P2 landed in commits fc96556110 (the gate) and
ce58dddf53 (the runtime phase, the fix line, the local route asking the
daemon, the recent-events check). What implementation decided is recorded in
"Implementation Record, 2026-11-10" below; where it changes a step or an AC,
the step or AC carries an amendment line and keeps its original text.

Parent: `plan/release/validation/spec-doctor-coverage-0-umbrella.md`.

## Task

Thomas: "Not only the one you have, but all problems. So we will have to audit
the code for all the things we need to be added to it."

This child owns four things, in this order:

| # | Work | Why first |
|---|------|-----------|
| 1 | The `runtime` doctor phase, the `fix` field on every doctor diagnostic, and the local route's answer when it cannot see daemon state | every later check needs a phase for daemon state and a field for its fix command |
| 2 | The gate: a scan that finds every problem site (a Warn or Error log line, a report-bus raise, a health row) and refuses a commit that adds one with no doctor code | its report is the mechanical half of the audit, and it stops the backlog growing while the audit runs |
| 3 | The audit: every component and plugin walked, the gate's report plus the silent states no log line names, into one inventory file | the area children are written from it |
| 4 | The area children: one spec per door area that holds gap rows | the gaps stay visible as specs |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/doctor-and-health-checks.md` - tiers, phases, owners, tests
  → Decision: the phase set is declared once in `pkg/plugin/rpc` (`DoctorCheckPhase`) and aliased by `internal/core/diagnostic`; a new phase is added there and nowhere else
  → Constraint: `parsed-config` checks must accept invalid semantics and apply nothing; the new `runtime` phase must likewise apply nothing and only read
  → Constraint: `TestDoctorDependencyInventory` holds an expected total; it stays, and the gate is a separate check that names sites
- [ ] `ai/doors/plugin-infra.md` - the rpc package is wire
  → Constraint: "A field or method name in `rpc` is wire, not Go: renaming a JSON tag breaks every plugin written in another language"; the new `fix` key and the `runtime` phase value are a protocol change, made directly before the first release with every in-tree plugin rebuilt (`ai/META-DECISIONS.md`, plugin protocol owes no backward compatibility)
- [ ] `ai/doors/core.md` - core registries
  → Constraint: a registry holds value types and is the only list; the gate reads the doctor registries, never a copied list of codes
- [ ] `ai/doors/le-tooling.md` - le tools
  → Constraint: a tool is one package under `internal/le/` at the path its name predicts (`le arch doctor-coverage` is `internal/le/arch/doctorcoverage`), registered from `init()`; `le` may link `internal/component/plugin/all` to read what `ze` registers
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: the fixing command sits on a line of its own; JSON carries no color
- [ ] `ai/META-DECISIONS.md`
  → Decision: a refusal that judges what the commit itself adds stands, while no gate refuses a commit for a red; the gate judges added sites only
  → Constraint: every operator command runs through the daemon on every surface, and the local route stays for an unreachable daemon
  → Decision (O-1, owner 2026-11-08): local `ze doctor` asks a reachable daemon for the runtime checks, and falls back to the local answer with the `doctor-runtime-not-checked` warning when no daemon answers
  → Decision (O-2, owner 2026-11-08): every Warn and Error log line carries a doctor code, with no transient marker; a transient class shows as a recent-event row read from the report bus
  → Decision (O-3, owner 2026-11-08): every area child is release work (P2, `plan/release/validation/`)

**Key insights:**
- On the daemon route `runChecks` still loads the config FILE from the store (`loadDoctorConfig`), not the running configuration, so a `runtime` check must read its owner's live state and never infer live state from `ctx.Tree`
- `collectPluginDoctorChecks` and `runPluginRegistryChecks` both rebuild each diagnostic from `Code`, `Severity`, `Message` only, so any fix carried elsewhere is dropped at two places

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/types.go` - `DoctorCheckPhase` with four values and `Valid`; `DoctorCheckDiagnostic{Code, Severity, Message}`
- [ ] `internal/core/diagnostic/doctor_registry.go` - `DoctorCheck`, `DoctorCheckContext`, `RegisterDoctorCheck`, `DoctorChecksForPhase`
- [ ] `internal/core/diagnostic/types.go` - `Diagnostic{Code, Severity, Message, Path, ..., Help, FixSafety, Repair, Related}`, `DoctorResult`
- [ ] `internal/component/doctor/answer.go` - `Answer`, `RuntimeChecks` (nil on the local route), `readyOf`
- [ ] `internal/component/doctor/doctor.go` `runChecks` - pre-config, then config load from the store, missing-config or parsed-config and post-config
- [ ] `internal/component/doctor/checks_plugin_registry.go` `runPluginRegistryChecks` - bridges `registry.PluginDoctorChecks()` per phase, three fields
- [ ] `internal/component/doctor/cmd/show.go` `HandleDoctor`, `collectPluginDoctorChecks` - daemon route, per-check timeout, three fields
- [ ] `internal/core/report/report.go` - `Issue{Source, Code, Severity, Subject, Message, Detail}`, `RaiseWarning`, `ClearWarning`, `ClearSource`
- [ ] `internal/core/diagnostic/doctor_daemon.go` (read 2026-11-10) - `DaemonDoctorCheck{Name, Codes, Check}`, `RegisterDaemonDoctorCheck`, `DaemonDoctorChecks`, `RunDaemonDoctorChecks`: a second, context-free registry of checks only the daemon route runs (commit 311177442c, 2026-11-09)
- [ ] `internal/component/config/system/register.go` (read 2026-11-10) - `init()` registers the dns-source selection checks with `RegisterDaemonDoctorCheck` (`dnsSourceDoctorCheckName`, `dnsSourceDoctorCodes`, `checkDNSSourceSelections`); `test/plugin/doctor-dns-source.ci` proves them through `show doctor`
- [ ] `internal/component/doctor/registry.go` (read 2026-11-10) - `runDoctorChecks(phase, ctx)` runs the registry checks of one phase then the plugin registry bridge; it is unexported, so `HandleDoctor` in `doctor/cmd` cannot call it
- [ ] `internal/le/commit/changecheck/changecheck.go` (read 2026-11-10) - `Check{Name, Judge}`, `Register`, `Registered`, `Run`, `Snapshot`: the seam a change-set check registers into so `./le commit create` runs it; the commit code names no check
- [ ] `internal/core/opererr/opererr.go` (read 2026-11-10) - `New(problem, why, fix)`: fix is the command alone, rendered on its own line after a `fix: ` label; a log line uses `Problem` as message and `LogAttrs` (`why`, `fix`) plus a caller's `code`; its doc comment already says fix is not a `Diagnostic.Repair` or `FixSafety`
- [ ] `cmd/ze/internal/cmdutil/cmdutil.go` `RunCommand` and `internal/component/cli/sshclient/client.go` `ExecCommand` (read 2026-11-10) - a daemon command from the local CLI travels over SSH with `sshclient.Credentials`, so "the daemon answers" needs credentials as well as a running daemon

**Corrections, 2026-11-10 (audit of the spec against the tree; the lines above stay as written):**
- `collectPluginDoctorChecks` (`internal/component/doctor/cmd/show.go`) does not filter by phase: it calls every check a running plugin declared, whatever its phase. The "three fields" reading above holds. A `runtime` check declared by a forked plugin therefore already runs on the daemon route; what the local route must not do is run it (`runPluginRegistryChecks` filters by phase and stays the local bridge)
- The text line is drawn by `outputText` (`internal/component/doctor/doctor.go`), which `renderDoctor` (`answer.go`) calls; Transformation Path step 3's "`renderDoctor` prints it" is read as `outputText` prints it, reached through `renderDoctor`
- `runDoctorChecks` (`internal/component/doctor/registry.go`) is unexported, so `HandleDoctor` needs an exported runner of the `runtime` phase in package `doctor`
- `HandleDoctor` today runs `diagnostic.RunDaemonDoctorChecks` (the registry in `doctor_daemon.go`) before the plugin checks; that registry is what the `runtime` phase replaces (Existing Mechanisms)
- `TestDoctorDependencyInventory` lives in `internal/component/doctor/doctor_test.go`, not `doctor_checks_test.go` as the Existing Mechanisms row below first said

**Behavior to preserve:**
- the four existing phases and their order; every existing check and code
- `DoctorResult` keys `schema-version`, `ready`, `diagnostics`; exit 1 when not ready on the local drawing
- a not-ready result on the daemon route is data with code 0

**Behavior to change:**
- a `runtime` phase that runs only inside the daemon
- a `fix` field carried from every check to every surface
- the local route asks a reachable daemon for the runtime checks (O-1); when no daemon answers, it reports that runtime checks did not run, instead of silence
- a commit that adds an uncovered problem site is refused, naming the site and the one way to cover it: a doctor code (O-2, no exempt marker)
- a transient problem class (a peer reset that recovers, one retry) is reported as a recent-event row read from the report bus, so its doctor code has a check that answers for it (O-2)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Doctor phase set | `pkg/plugin/rpc/types.go` `DoctorCheckPhase`, `Valid` | Four phases, one declaration | extends: adds `runtime` in the same declaration |
| Doctor check registry | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck` | In-tree owners register checks with codes | extends: a `runtime` check registers here with the new phase |
| Plugin doctor checks | `internal/component/plugin/registry/doctor.go` `DoctorCheckDef`; `checks_plugin_registry.go` | Plugin-declared checks bridged per phase | extends: carries `fix`; a plugin declares `runtime` checks the same way |
| Daemon doctor route | `internal/component/doctor/cmd/show.go` `HandleDoctor` | Runs `doctor.Answer` with the plugins' checks | extends: also runs `runtime` registry checks |
| Diagnostic record | `internal/core/diagnostic/types.go` `Diagnostic` | Has `Help`, `Repair`, no fix command field | extends: adds `Fix`; `Help` stays for config validation help text, which is not a command |
| Diagnostic code registry | `internal/core/diagnostic/registry.go` `Register`; `ze show diagnostic` | Explains each code | extends: the gate resolves every covered code through it |
| Enumeration gate | `internal/le/arch/enumeration`, `./le arch enumeration check` and `report`, exempt marker | AST scan of the tree with a report and a check form | extends as a model: `./le arch doctor-coverage` copies its check and report shape |
| Report bus | `internal/core/report/report.go` `RaiseWarning`, `RaiseError`, `Warnings`, `Errors` (the error ring) | Coded operator issues: active warnings, and an error ring of recent events | extends: each raise is a problem site whose code must be a doctor code; a transient class raises its event here, and one `runtime` bridge check renders the ring's recent events as doctor rows (O-2) |
| Daemon reach from the local CLI | the client `ze` already uses to send a command to a running daemon (`docs/architecture/api/commands.md`, "Served by the daemon too"; the client symbol is named at implementation) | `ze <verb>` commands answered by the daemon | extends: the local doctor route sends `show doctor` through it when the daemon answers (O-1) |
| Live health registry | `internal/core/health/registry.go` `Register`, `RegisterWhen` | Degraded rows with a reason | extends: each registration is a problem site the inventory maps to a doctor check |
| Dependency inventory test | `internal/component/doctor/doctor_checks_test.go` `TestDoctorDependencyInventory` | Expected total of dependencies | does not apply: kept as is; it counts dependencies, not problem classes |
| Commit-time added-content refusals | `./le commit create` (journal owned-rows refusal, `plan/journal/README.md` "Owned rows") | Refuses what the commit itself adds | extends: the doctor-coverage check joins as a refusal of added sites |
| Audit records | `plan/audits/` | Historical audit reports | extends: the inventory lands there |
| Daemon-only doctor checks (added 2026-11-10) | `internal/core/diagnostic/doctor_daemon.go` `DaemonDoctorCheck`, `RegisterDaemonDoctorCheck`, `RunDaemonDoctorChecks` (commit 311177442c); called from `HandleDoctor`; doctor page, "Daemon doctor checks" | A second registry of context-free checks only `show doctor` runs; its one user is the dns-source selection checks (`internal/component/config/system/register.go`, proven by `test/plugin/doctor-dns-source.ci`) | REPLACES (`ai/rules/no-layering.md`): the `runtime` phase is the same idea inside the one phase set. P1a deletes `doctor_daemon.go` first, then re-registers the dns-source checks through `RegisterDoctorCheck` with `Phase: runtime`, drops the `RunDaemonDoctorChecks` call from `HandleDoctor`, and rewrites the page section as the `runtime` phase. Two registries for one fact would let a check pick the one the local route runs |
| Change-set check seam (added 2026-11-10) | `internal/le/commit/changecheck` `Register`, `Run` (built by spec-operator-errors-name-the-fix) | `./le commit create` runs every registered check over what the commit adds; the commit code names no check | extends: the doctor-coverage gate registers its `Judge` into it from `internal/le/arch/doctorcoverage`'s own `init()`; no file under `internal/le/commit` names it. This replaces the "joins `./le commit create`" wording of the Commit-time added-content refusals row above, which stays for the refusal kind it describes |
| Operator error (added 2026-11-10) | `internal/core/opererr` `New`, `Error()`, `LogAttrs` | problem, why, fix; fix is the command alone on its own line; log attributes `why` and `fix`, plus a caller-supplied `code` | extends: a doctor diagnostic's `Fix` holds the same thing (the command alone, no label), and the gate's A-1 reads the same `code` log key `opererr` passes through, so a problem site logged through `opererr` is covered by the same rule. No second fix convention |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show doctor` on any daemon surface; `ze doctor` locally
- `./le arch doctor-coverage check` and `./le commit create` for the gate

### Transformation Path
1. `HandleDoctor` calls `doctor.Answer` with a runtime closure that runs plugin checks and the `runtime` registry checks
2. `runChecks` never runs `runtime` checks. On the local route (O-1, decided) `ze doctor` first asks the daemon: when the daemon's socket answers and `config` names no file outside the daemon's config directories, the answer is the daemon's `show doctor` answer, drawn locally. When no daemon answers, `Answer` runs the local checks and adds one `doctor-runtime-not-checked` warning whose fix names `ze start` to run the daemon, then `show doctor`
2a. Transient classes (O-2): the `runtime` bridge check `doctor-recent-events` reads `report.Errors` and the active `report.Warnings`, and answers one warning row per recent event under the event's own code, with its subject and time in the message, so a transient problem is reported without making the box read not ready
3. Each diagnostic carries `fix`; `renderDoctor` prints it on its own line after the message; JSON carries the `fix` key
4. Gate: the scan parses non-test Go under `internal/component`, `internal/plugins` and `cmd/ze`, finds problem sites, resolves each site's code against the codes of every registered doctor check (in-tree and plugin), and lists the uncovered ones; the check form refuses only sites the commit adds against `HEAD^`

**Additions, 2026-11-10 (audit of the spec against the tree):**
- 1a. `HandleDoctor`'s runtime closure runs the `runtime` phase through a new exported runner in package `doctor` (`runDoctorChecks` is unexported), then `collectPluginDoctorChecks`. It no longer calls `diagnostic.RunDaemonDoctorChecks`, which is deleted with `doctor_daemon.go`; the dns-source checks reach `show doctor` as `runtime`-phase registry checks
- 2b. O-1, the case "daemon up, operator has no credentials": the local route asks the daemon over SSH (`cmdutil.RunCommand` sends a daemon command through `sshclient.ExecCommand` with `sshclient.Credentials`). When the daemon runs but the SSH call fails (no credentials in zefs, refused login, unreachable listener), no daemon has answered: the local route answers locally and adds the `doctor-runtime-not-checked` warning, its message naming why the daemon did not answer and its fix naming the command that gets an answer (`show doctor` from an account that can log in). The local route never reads a failed ask as a healthy runtime
- 3a. Step 3's line is drawn by `outputText` (`internal/component/doctor/doctor.go`), reached through `renderDoctor`
- 4a. Step 4's check form reaches `./le commit create` by registering into `internal/le/commit/changecheck` from the `doctorcoverage` package's `init()`; the commit code names no check

**Amendments, 2026-11-10 (implementation; the steps above stay as written):**
- 1b. A `runtime` registry check receives only `Platform` in its `DoctorCheckContext`: the daemon route holds no config tree to hand it, so a runtime check reads its owner's live state and never `Tree` or `ConfigDir` (`RunRuntimeChecks`, `internal/component/doctor/registry.go`)
  → Decision: A-3 holds in this shape: a runtime check's context is the platform alone
- 2 (amended). The `doctor-runtime-not-checked` warning names its fix by the cause the daemon did not answer, not always `ze start`: `ze start` when no daemon listens; `ze cli --user <name> -c "show doctor"` when the daemon listens but this account cannot log in; `ze status` when the listener cannot be found; `ze doctor` (no `config`) for R-5, where the named file is outside the daemon's config directories (`internal/component/doctor/local_route.go`)
- 2c. `ze doctor` drawing a daemon answer exits 0 when the answer holds only warnings, as the local answer does; not-ready still exits 1
- 2a (amended). The recent-events check is named `recent-events` and declares the one code `doctor-recent-events` (`internal/component/doctor/checks_recent_events.go`). A row carries the event's own code only when that code's metadata is registered (`ze show diagnostic` can explain it); otherwise the row is `doctor-recent-events` with the bus code named in its message, so no event is dropped and no row carries a code the operator cannot look up. Its fix lines are the bare commands `show errors` (an error event) and `show warnings` (an active warning)
  → Decision: because the check declares only `doctor-recent-events`, a report-bus code counts as covered for the gate only when the check of the code's owner declares it. A transient class therefore needs its owner's check to declare the code; the recent-events row then reports it
- 4b. A health row (`health.Register`, `health.RegisterWhen`) is listed by the gate and never refused: the registration takes no code, so there is nothing to resolve; the audit maps each one to a doctor check by hand
  → Constraint: the gate cannot see a Warn or Error logged through a function value (dhcpserver `warnLimited`), nor a site whose message is not a literal (`internal/plugins/flowexport/register.go`, line 172); and a `code` log attribute that holds a protocol or exit code (not a doctor code) reads as code-dynamic. The audit walks these by hand

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to plugin | `ze-plugin-callback:doctor-check` output gains `fix`; Stage 1 declaration may carry phase `runtime` | No: census below |
| le to product registries | le links `internal/component/plugin/all` and reads `diagnostic.DoctorChecksForPhase` and `registry.PluginDoctorChecks` | No |

### Changed-Wire Census

**Search evidence:** scope `pkg/`, `internal/`, `cmd/`, `test/`; terms `DoctorCheckDiagnostic`, `DoctorCheckOutput`, `DoctorCheckPhase`, `"doctor-check"`, `pre-config`, `post-config`. Positive control: `collectPluginDoctorChecks` reads `out.Diagnostics` (read above). The implementer re-runs the search before phase 1 and adds every carrier found.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| doctor-check output | writer | each plugin check through `pkg/plugin/sdk` doctor callback | `DoctorCheckDiagnostic` is the element type | add `fix` (omitempty); every in-tree plugin rebuilt |
| doctor-check output | reader | `internal/component/doctor/cmd/show.go` `collectPluginDoctorChecks` | maps three fields | map `fix` too |
| registry bridge | writer | each in-tree plugin's `registry.DoctorCheckDef.Check` (`internal/component/plugin/registry/doctor.go` `DoctorCheckFunc`, answering `[]rpc.DoctorCheckDiagnostic`) | `DoctorCheckFunc` returns the rpc element type (read 2026-11-10) | a check that has a fix sets `Fix`; the field rides the rpc type, no second type |
| registry bridge | reader | `internal/component/doctor/checks_plugin_registry.go` `runPluginRegistryChecks` | maps three fields | map `fix` too |
| Stage 1 doctor declaration | writer and reader | `rpc.DoctorCheckDecl.Phase`, validated by `DoctorCheckPhase.Valid` | `Valid` lists four phases | add `runtime` to the declaration and `Valid` |
| `ze doctor --json` / `show doctor` JSON | writer | `diagnostic.Diagnostic` | `DoctorResult.Diagnostics` | add `fix` (omitempty), kebab-case |
| `ze doctor --json` / `show doctor` JSON | reader | the local drawing (`internal/component/doctor/doctor.go` `outputJSON`, `outputText`); every API client of `show doctor` outside the repository (MCP, REST, gRPC, SSH); the `.ci` files that assert doctor output (`test/plugin/api-doctor-check.ci`, `test/plugin/doctor-dns-source.ci` and the other `test/plugin/*doctor*.ci`) | `outputText` draws code and message only (read 2026-11-10); a client that ignores unknown keys keeps working | `outputText` draws the fix line; the new key is additive (omitempty), so an outside reader that ignores unknown keys is unaffected; the implementer re-runs the census search over `test/` before P1a |

### Integration Points
- `doctor.Answer` and the local route - ask a reachable daemon first (O-1); add the runtime-not-checked row only when no daemon answers
- `./le commit create` - runs the doctor-coverage refusal over the staged set

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | runtime checks run through `HandleDoctor` and `doctor.Answer` |
| No unintended coupling (components stay isolated) | Yes | the gate is le tooling; product code gains no le import |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms table |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no hot path |
| Registration over hardcoding, outbound | Yes | runtime checks register with the existing registry |
| Registration over hardcoding, inbound | Yes | the gate derives covered codes from the two doctor registries; it holds no code list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A problem site can name its code statically: a report raise takes the code as an argument, and a log line can carry a `code` attribute whose value is an exported code constant | `report.Issue.Code`; slog attributes are key-value pairs | the scan cannot resolve a code built at run time | gate unit test over fixtures with constant, variable and missing codes | validated 2026-11-10: `TestDoctorCoverageResolvesStaticCodes`, `TestDoctorCoverageRefusesUnparsableFile` (`internal/le/arch/doctorcoverage/doctorcoverage_test.go`); limits in Data Flow 4b |
| A-2 | le can read every registered doctor check's codes by linking the composition root | `ai/doors/le-tooling.md`: le may link `internal/component/plugin/all` | the gate misses plugin codes | `TestDoctorCoverageReadsPluginCodes` | validated 2026-11-10: `TestDoctorCoverageReadsPluginCodes`, `TestDoctorCoverageThinCoveredSetRefused` (`internal/le/arch/doctorcoverage/check_test.go`) |
| A-3 | The daemon's in-process components (L2TP, iface, IKE) run their `runtime` checks in the daemon process, so a registry check there reads live state | L2TP and PPPoE are `Engine.RegisterSubsystem` subsystems (`ai/doors/hub-engine.md`); IKE runs in the daemon through DirectBridge (`ai/doors/plugin-infra.md`, Traps) | a check reads an empty copy | each owner's check test runs against the live owner state; a forked plugin uses the plugin callback instead | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The gate refuses ordinary new log lines and slows every session | refusals on lines that are not problems | O-2 (owner): there is no marker, a Warn or Error line is a problem the operator can see, and it carries a doctor code; the refusal names that cover and, for a transient class, the report-bus raise that `doctor-recent-events` reports; a `plan/learned/HOOK-FRICTION.md` row if friction appears |
| R-2 | A local run with a dead daemon reads ready | no row about runtime checks | `doctor-runtime-not-checked` is a warning, always present on the local fallback when `runtime` checks are registered |
| R-5 | The local route reaches a daemon that answers from a different config than the operator named | `config <file>` outside the daemon's config directories | the local route asks the daemon only when `config` names no such file, otherwise it answers locally with the warning row (O-1) |
| R-6 | Recent-event rows grow without bound | many rows from a flapping peer | the rows come from the bounded report-bus ring (`ze.report.errors.max`); one row per (source, code, subject) with a count |
| R-3 | The inventory is one row per log line and unusable | thousands of rows | group by problem class and owner; the report keeps the per-site list |
| R-4 | A forked plugin's `runtime` check is called while the plugin is stopped | `doctor-plugin-unreachable` | existing behavior: that row already says the plugin did not answer |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a commit refused wrongly (tooling), or a doctor row missing; no forwarding change |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | every session committing new log lines (the gate); every plugin author (`fix` key) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` | → | `HandleDoctor` running a registered `runtime` check | `test/plugin/show-doctor-runtime-checks.ci` |
| `ze doctor --json`, no daemon | → | `Answer` adding `doctor-runtime-not-checked` | `TestDoctorRuntimePhaseNotRunLocally`, `test/ui/doctor-runtime-not-checked.ci` |
| `ze doctor`, daemon running | → | the local route asking the daemon's `show doctor` | `TestDoctorLocalRouteAsksReachableDaemon`, `test/plugin/doctor-local-asks-daemon.ci` |
| a report-bus error event, `show doctor` | → | the `doctor-recent-events` runtime check | `TestDoctorRecentEventsRowsFromReportRing` |
| plugin doctor callback | → | `collectPluginDoctorChecks` mapping `fix` | `TestCollectPluginDoctorChecksCarriesFix` |
| `./le arch doctor-coverage check` | → | the site scan and code resolution | `TestDoctorCoverageRefusesAddedUncoveredSite` |
| `./le commit create` | → | the added-site refusal | `TestCommitCreateRefusesUncoveredProblemSite` |
| `show doctor` with a dns-source fault (added 2026-11-10) | → | the dns-source checks as `runtime` registry checks | `test/plugin/doctor-dns-source.ci` (kept, unchanged expectations), `TestDNSSourceChecksRegisterRuntimePhase` |
| `ze doctor`, daemon up, SSH ask refused (added 2026-11-10) | → | `Answer` falling back with `doctor-runtime-not-checked` | `TestDoctorLocalRouteNoCredentialsAnswersLocally` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A check registered with phase `runtime`, `show doctor` | the daemon runs it and its diagnostics appear with code, severity, message and fix | none |
| AC-2 | The same check, `ze doctor --json` locally with no daemon answering | the check does not run; the local checks run; one warning `doctor-runtime-not-checked` says runtime checks need the running daemon and its fix line names the command that starts it | none |
| AC-10 | The same check, `ze doctor` locally while the daemon runs, no `config` keyword (O-1) | the answer is the daemon's `show doctor` answer, the runtime row included, drawn by the local renderer with the local exit code; no `doctor-runtime-not-checked` row | `config <file>` naming a file outside the daemon's config directories answers locally with the warning row, because the daemon would read another file (R-5) |
| AC-11 | A report-bus error event raised with a doctor code, then `show doctor` (O-2) | `doctor-recent-events` answers one warning row under that code naming subject, count and last time; `ready` is unchanged by it | none |
| AC-12 | A fixture commit adding a Warn log line with an exempt or transient marker comment (O-2) | refused like an uncovered line: no marker is accepted | none |
| AC-3 | A plugin check returning a diagnostic with `fix` | text output prints the fix on its own line after the message; JSON carries `"fix"`; on every surface through `show doctor` | none |
| AC-4 | A fixture commit adding a Warn log line in `internal/plugins/<x>` with no doctor code | `./le arch doctor-coverage check` and `./le commit create` refuse it, naming file and line, and print the cover: a `code` attribute naming a code a registered doctor check declares, and for a transient class the report-bus raise under that code (O-2) | a site the commit did not add is reported by `report`, never refused |
| AC-5 | `./le arch doctor-coverage report` | lists every uncovered site grouped by owner package and door area, with counts | none |
| AC-6 | Audit done | `plan/audits/doctor-problem-inventory.md` holds one row per problem class with columns area, owner package, class, state it reads, route (local or runtime), existing check and code or `gap`, source sites; and one spec per door area with gap rows exists at `plan/release/validation/spec-doctor-coverage-<n>-<area>.md`, each listing its rows as ACs, each with `Priority` P2 as release work whatever release profile its feature sits in (O-3) | an area with no gap row gets no spec |
| AC-7 | Every check added under this umbrella | its unit test asserts a non-empty fix that names a `ze` command or a `set`/`delete` config path | none |
| AC-9 | An existing check that reads daemon state while registered at `pre-config`, `parsed-config` or `post-config` | the audit lists it with its owner; it moves to `runtime` in its area child (IKE's `ipsec-virtual-ip-pool` and `ipsec-udp-encap` move in child 2), so the local route never reads that state as empty and healthy | none |
| AC-8 | Docs | `docs/architecture/doctor-and-health-checks.md` documents the `runtime` phase, the fix field, the local-route row and the gate; `docs/plugin-development/` documents `fix` and `runtime` for plugin authors | none |
| AC-13 (added 2026-11-10) | The dns-source selection checks after P1a, `show doctor` on a daemon whose dns-source selection is empty, silent or unserved | the same `doctor-dns-source-*` rows appear, produced by checks registered through `RegisterDoctorCheck` with `Phase: runtime`; `internal/core/diagnostic/doctor_daemon.go` is gone, and no `RegisterDaemonDoctorCheck` or `RunDaemonDoctorChecks` symbol remains in the tree; the doctor page's "Daemon doctor checks" section describes the `runtime` phase instead | none: two registries for daemon-only checks is the layering this removes |
| AC-14 (added 2026-11-10) | `ze doctor` locally while the daemon runs, but the SSH ask fails (no credentials, refused login) (O-1) | the local checks answer, plus one `doctor-runtime-not-checked` warning whose message says the daemon did not answer and why, and whose fix line names a command that gets the daemon's answer; no runtime check runs in the local process | none |

**AC amendments, 2026-11-10 (implementation; the rows above stay as written):**
- AC-2: the fix line names `ze start` when no daemon listens; the other causes name their own command (Data Flow step 2, amended)
- AC-4: the printed cover reads "a `code` attribute naming a code a registered doctor check declares"; for a transient class that check is the owner's, declaring the code it raises on the report bus, which the `recent-events` check then reports. The `recent-events` check's own declaration (`doctor-recent-events`) covers no other code (Data Flow step 2a, amended)
- AC-11: the row carries the event's own code when its metadata is registered, else `doctor-recent-events` with the bus code in the message; its fix is `show errors` or `show warnings`
- AC-13: the dns-source findings carry their command in `Fix`, not in the message

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoctorPhaseRuntimeValid` | `pkg/plugin/rpc/types_test.go` | `runtime` is a valid phase | |
| `TestDoctorRuntimePhaseNotRunLocally` | `internal/component/doctor/answer_test.go` | AC-2 | |
| `TestDoctorRuntimePhaseRunsOnDaemonRoute` | `internal/component/doctor/cmd/show_test.go` | AC-1 | |
| `TestCollectPluginDoctorChecksCarriesFix` | `internal/component/doctor/cmd/show_test.go` | AC-3 | |
| `TestRunPluginRegistryChecksCarriesFix` | `internal/component/doctor/checks_plugin_registry_test.go` | AC-3, local bridge | |
| `TestRenderDoctorPrintsFixOnItsOwnLine` | `internal/component/doctor/answer_test.go` | AC-3 text shape | |
| `TestDoctorCoverageRefusesAddedUncoveredSite` | `internal/le/arch/doctorcoverage/check_test.go` | AC-4 | |
| `TestDoctorCoverageAcceptsCoveredSite` | `internal/le/arch/doctorcoverage/check_test.go` | a site whose code a registered check declares passes | |
| `TestDoctorCoverageReadsPluginCodes` | `internal/le/arch/doctorcoverage/check_test.go` | A-2 | |
| `TestDoctorCoverageReportGroupsByOwner` | `internal/le/arch/doctorcoverage/report_test.go` | AC-5 | |
| `TestCommitCreateRefusesUncoveredProblemSite` | the `./le commit create` refusal tests | AC-4 through the commit entry | |
| `TestDoctorLocalRouteAsksReachableDaemon` | `internal/component/doctor/answer_test.go` | AC-10: a stub daemon answer is drawn and no warning row is added; an unreachable daemon gives AC-2; a `config` file outside the daemon's directories answers locally | |
| `TestDoctorRecentEventsRowsFromReportRing` | the `doctor-recent-events` owner's test | AC-11: rows per code from `report.Errors` and `report.Warnings`, readiness unchanged | |
| `TestDoctorCoverageRefusesMarkedSite` | `internal/le/arch/doctorcoverage/check_test.go` | AC-12: a marker comment is not a cover | |
| `TestDNSSourceChecksRegisterRuntimePhase` (added 2026-11-10) | `internal/component/config/system/doctor_dns_source_test.go` | AC-13: the dns-source checks are registry checks with `Phase: runtime`, and the local runner skips them | |
| `TestDoctorLocalRouteNoCredentialsAnswersLocally` (added 2026-11-10) | `internal/component/doctor/answer_test.go` | AC-14: a stub ask that fails with a running daemon gives the local answer plus the warning row | |
| `TestDoctorChangeCheckRegistered` (added 2026-11-10) | `internal/le/arch/doctorcoverage/register_test.go` | AC-4 through the seam: `changecheck.Registered` holds the doctor-coverage check | |

**Test locations as implemented, 2026-11-10 (where a row above names another file, this list governs):**

| Test | File | Validates |
|------|------|-----------|
| `TestRenderDoctorPrintsFixOnItsOwnLine` | `internal/component/doctor/render_fix_test.go` | AC-3 text shape |
| `TestCommitCreateRefusesUncoveredProblemSite` | `internal/le/arch/doctorcoverage/register_test.go` (over the real checkout) | AC-4 through the commit entry |
| `TestDoctorRuntimePhaseNotRunLocally` | `internal/component/doctor/answer_test.go` | AC-2 |
| `TestDoctorLocalRouteAsksReachableDaemon` | `internal/component/doctor/answer_test.go` | AC-10, R-5 |
| `TestDoctorLocalRouteNoCredentialsAnswersLocally` | `internal/component/doctor/answer_test.go` | AC-14 |
| `TestDecodeDaemonDoctorRefusesAForeignAnswer` | `internal/component/doctor/answer_test.go` | the local route never draws an answer whose schema it does not know |
| `TestDoctorRecentEventsRowsFromReportRing` | `internal/component/doctor/checks_recent_events_test.go` | AC-11 |
| `TestDoctorRecentEventsSubjectsAreSeparateRows` | `internal/component/doctor/checks_recent_events_test.go` | AC-11, R-6: one row per (source, code, subject) |
| `TestDoctorRecentEventsCodeWithoutMetadata` | `internal/component/doctor/checks_recent_events_test.go` | AC-11: an unregistered bus code reports as `doctor-recent-events` |
| `TestDoctorRecentEventsEmptyBus` | `internal/component/doctor/checks_recent_events_test.go` | AC-11: no event, no row |
| `TestDoctorRecentEventsRegisteredRuntimePhase` | `internal/component/doctor/checks_recent_events_test.go` | AC-11: registered at `runtime` |
| `TestDNSSourceChecksRegisterRuntimePhase` | `internal/component/config/system/doctor_dns_source_test.go` | AC-13 |
| `TestDoctorCoverageResolvesStaticCodes`, `TestDoctorCoverageRefusesUnparsableFile` | `internal/le/arch/doctorcoverage/doctorcoverage_test.go` | A-1 |
| `TestDoctorCoverageThinCoveredSetRefused` | `internal/le/arch/doctorcoverage/check_test.go` | A-2: a covered set too thin to be the registry is refused, not read as "nothing covered" |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-doctor-runtime-checks` | `test/plugin/show-doctor-runtime-checks.ci` | operator runs `show doctor` on a daemon and reads a runtime row with its fix | |
| `doctor-runtime-not-checked` | `test/ui/doctor-runtime-not-checked.ci` | operator runs `ze doctor` with no daemon and is told runtime checks did not run, with the command that starts the daemon | |
| `doctor-local-asks-daemon` | `test/plugin/doctor-local-asks-daemon.ci` | operator runs `ze doctor` on a box whose daemon runs and reads the runtime row (AC-10) | |

## Files to Modify
- `pkg/plugin/rpc/types.go` - `runtime` phase, `Valid`, `DoctorCheckDiagnostic.Fix`
- `internal/core/diagnostic/types.go` - `Diagnostic.Fix`
- `internal/core/diagnostic/doctor_registry.go` - alias `DoctorPhaseRuntime`; phase rank
- `internal/component/doctor/answer.go` - the local route asks a reachable daemon first (O-1), local-route row on fallback, fix rendering
- the owner of the report bus bridge check (`doctor-recent-events`, `runtime` phase, registered from its owner's `init()`; the owning package is chosen at implementation so `internal/core/report` stays a leaf) (O-2)
- `internal/component/doctor/doctor.go` - `runChecks` skips `runtime`
- `internal/component/doctor/checks_plugin_registry.go` - carry `fix`
- `internal/component/doctor/cmd/show.go` - run `runtime` registry checks; carry `fix`
- `docs/architecture/doctor-and-health-checks.md` - the phase, the field, the gate
- `docs/guide/health-checks.md` - reading the fix line

**Additions, 2026-11-10 (audit of the spec against the tree):**
- `internal/core/diagnostic/doctor_daemon.go` - DELETED first in P1a (`remove` through `./le commit create`), replaced by the `runtime` phase (no-layering)
- `internal/component/config/system/register.go` and `doctor_dns_source.go` - the dns-source checks register through `RegisterDoctorCheck` with `Phase: runtime`; their check function takes the registry's `DoctorCheckContext` and reads the live dns-source registry as before; the file comments that name the daemon doctor check are rewritten
- `internal/component/doctor/registry.go` - an exported runner of the `runtime` phase that `HandleDoctor` calls; `runDoctorChecks` stays the local runner and never receives `runtime`
- `internal/component/doctor/cmd/show.go` - drop the `RunDaemonDoctorChecks` call; call the exported runner, then `collectPluginDoctorChecks`
- `internal/component/doctor/doctor.go` `outputText` - draws the fix line (Transformation Path 3a)
- `internal/core/diagnostic/types.go` - the `Fix` doc comment states how `Fix`, `FixSafety` and `Repair` differ (Design Insights, 2026-11-10)
- `internal/core/diagnostic/codes.go` - every new code registered up front in P1a (`doctor-runtime-not-checked`, `doctor-recent-events`); another session carried a one-line hunk in it on 2026-11-10, so the committer checks `git diff` and commits its own hunks with `hunks` if the foreign one is unsafe
- `pkg/plugin/sdk/sdk_types.go`, `pkg/plugin/sdk/sdk_callbacks.go` - the SDK aliases and comments of the `runtime` phase (the phase list comment names four values today); `OnDoctorCheck` passes `fix` through unchanged with the rpc type
- `docs/plugin-development/protocol.md` - `fix` and `runtime` for plugin authors (AC-8)
- `docs/architecture/doctor-and-health-checks.md` "Daemon doctor checks" - rewritten as the `runtime` phase (AC-13)

**Additions, 2026-11-10 (implementation, P1 and P2 as landed):**
- `internal/component/cli/sshclient/client.go` - exported `DaemonAddress`, which tells the local route "no daemon listens" from "this account cannot log in"; the file declares `docs/architecture/system-architecture.md` as its design page
- `internal/component/plugin/registration.go` - the `DoctorCheckRegistration.Phase` comment names `runtime` among the phases
- `docs/architecture/api/process-protocol.md` - `fix` and `runtime` in the doctor-check payload
- `docs/features/dns-resolver.md` - the dns-source doctor rows carry their command as a fix line
- `docs/features/ai-first.md` - the diagnostic record gains `fix` (Documentation row 16)
- `docs/guide/health-checks.md` - reading the fix line
- `internal/component/doctor/answer_test.go` - the local-route tests (AC-2, AC-10, AC-14)

## Files to Create
- `internal/le/arch/doctorcoverage/` - the scan, `check` and `report` actions, registered from `init()`; its `init()` also registers the check into `internal/le/commit/changecheck` (P2b), and nothing under `internal/le/commit` changes for it
- `test/plugin/show-doctor-runtime-checks.ci`
- `test/ui/doctor-runtime-not-checked.ci`
- `test/plugin/doctor-local-asks-daemon.ci`
- `plan/audits/doctor-problem-inventory.md` - the inventory
- `plan/release/validation/spec-doctor-coverage-<n>-<area>.md` - one per door area with gaps, all release work, P2 (O-3)

**Additions, 2026-11-10 (implementation, P1 and P2 as landed):**
- `internal/core/diagnostic/doctor_registry_list.go` - `DoctorChecks`, every registered check of every phase, which the gate reads for its covered codes
- `internal/le/hookruntime/doors_covering.go` - `DoorsCovering`, the door lookup the read gate uses, exposed so the gate's report groups sites by door area by the same rule; the file declares `docs/architecture/core-design.md` as its design page
- `internal/component/doctor/local_route.go` - the local route's ask of the daemon and its fallback row (O-1)
- `internal/component/doctor/checks_recent_events.go` (+ test) - the `recent-events` check (O-2)
- `internal/component/doctor/render_fix_test.go` - the fix-line rendering test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | `show doctor` exists |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | Yes | `le arch doctor-coverage check` and `report` (le tooling) |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`; `./le cli grammar` |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | Yes | `test/plugin/show-doctor-runtime-checks.ci` |
| Pipe completeness | Yes | `show doctor` with the json pipe already works; the `fix` key rides it |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | this child is the doctor contract itself |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the doctor feature page under `features/` (level from `./le feature report`) |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | `ze doctor` output gains a fix line; `docs/guide/health-checks.md` covers it |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (`show doctor` JSON gains `fix`) |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md` |
| 7 | Wire format changed? | No | no protocol wire beyond the plugin protocol (row 8) |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/` doctor sections |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/ai-first.md` (declared by `internal/core/diagnostic/types.go` and `checks_plugin_registry.go`): updated, the diagnostic record gains `fix`; `docs/architecture/api/ipc_protocol.md` (declared by `pkg/plugin/rpc/types.go`): unaffected, answer-line framing does not change, only a JSON field inside a doctor-check payload; the rest derived by `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-1-audit-and-gate.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/health-checks.md` sample output |

## Implementation Steps

1. **Phase: Wiring** -- `runtime` phase, `fix` field through both bridges, local-route row
   - Tests: `TestDoctorPhaseRuntimeValid`, `TestDoctorRuntimePhaseNotRunLocally`, `TestDoctorRuntimePhaseRunsOnDaemonRoute`, `TestCollectPluginDoctorChecksCarriesFix`, `TestRunPluginRegistryChecksCarriesFix`, `TestRenderDoctorPrintsFixOnItsOwnLine`
   - Files: rpc types, diagnostic types and registry, doctor answer, doctor.go, bridges
   - Verify: fail, implement, pass; the `.ci` tests pass
2. **Phase: Gate** -- `./le arch doctor-coverage check` and `report`, joined to `./le commit create`
   - Tests: the four `doctorcoverage` tests, `TestCommitCreateRefusesUncoveredProblemSite`
   - Verify: a fixture adding an uncovered Warn is refused; a covered one passes
3. **Phase: Audit** -- per door area (`ai/doors/README.md` order): run `report` for the area, then walk each component and plugin for silent states: refusals that leave state degraded after the command returns, config accepted but not in effect (skipped, waiting), failed kernel or peer interactions retried without a log line, missing capability, and every health row
   - Output: `plan/audits/doctor-problem-inventory.md`, committed per area as each area finishes
4. **Phase: Area children** -- write one spec per area holding gap rows, each row an AC with owner, state, route and code name

### Implementation Phases (added 2026-11-10)

The four steps above, cut into agent work packages (`plan/handover/doctor-coverage-1-start.md`, "Proposed phases"). One agent per package, each about 60 tool calls; `∥` marks packages that run at once. Agents read and edit only; every test, build and lint runs in the main thread, one at a time.

| Phase | Owns | Covers | Depends on | Runs with |
|-------|------|--------|------------|-----------|
| P1a wire | `pkg/plugin/rpc/types.go` (+ test), `pkg/plugin/sdk/sdk_types.go`, `pkg/plugin/sdk/sdk_callbacks.go`; `internal/core/diagnostic/types.go`, `doctor_registry.go`, `codes.go` (every new code up front); delete `internal/core/diagnostic/doctor_daemon.go`; `internal/component/config/system/register.go`, `doctor_dns_source.go`; `internal/component/doctor/registry.go`, `checks_plugin_registry.go`, `doctor.go`; `internal/component/doctor/cmd/show.go`; the doctor page; `docs/plugin-development/protocol.md` | AC-1, AC-3, AC-7 base, AC-8 docs, AC-13 | - | ∥ P2a |
| P2a gate scan | `internal/le/arch/doctorcoverage/**` (scan, `check`, `report`) | AC-4 check form, AC-5, AC-12, A-1, A-2 | - | ∥ P1a |
| P1b local route (O-1) | `internal/component/doctor/answer.go`, `doctor.go` `Run`; `test/plugin/show-doctor-runtime-checks.ci`, `test/plugin/doctor-local-asks-daemon.ci`, `test/ui/doctor-runtime-not-checked.ci` | AC-2, AC-10, AC-14 | P1a | ∥ P1c |
| P1c recent events (O-2) | new `internal/component/doctor/checks_recent_events.go` (+ test), `internal/component/doctor/register.go` | AC-11 | P1a | ∥ P1b |
| P2b commit registration | the `doctorcoverage` `init()` registering into `internal/le/commit/changecheck` | AC-4 through `./le commit create` | P2a | - |
| P3 audit | about 18 area agents, at most 5 at once (system-plugins split in 3, bgp-plugins in 2, routing-protocols in 2), each walking one door area from the P2a report plus the silent states; area spec numbers 3..N assigned in advance so agents never collide on a name; then one merge agent writes `plan/audits/doctor-problem-inventory.md` and the area children | AC-6, AC-9 | P2a report | - |

Child 2 (`plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md`) waits on P1a only, plus P1b for its R-5.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every door area walked; every inventory row has a state it reads |
| Correctness | no `runtime` check runs in the local process; the local route answers from the daemon when it answers, and the warning row always appears on the local fallback when a `runtime` check exists |
| Naming | JSON key `fix`, phase `runtime`, code `doctor-runtime-not-checked`, kebab-case |
| Data flow | the gate derives codes from registries, never a list |
| Rule: user-facing-errors | the gate's refusal names the site, why, and the two covers on their own lines |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `runtime` phase | `TestDoctorPhaseRuntimeValid` |
| `fix` on every surface | `test/plugin/show-doctor-runtime-checks.ci` asserts the `fix` key |
| Gate | `TestDoctorCoverageRefusesAddedUncoveredSite` |
| Inventory | `plan/audits/doctor-problem-inventory.md` with a row for every door area |
| Area children | listing `plan/release/validation/spec-doctor-coverage-*` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Information exposure | a fix line never carries a secret; `show doctor` stays behind authz |
| Gate input | the scan reads source only, never runs it |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A site's code cannot be resolved statically | DESIGN: the owner names the code with a constant |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The daemon route re-reads the config file, so "running state" and "config on disk" can disagree; a runtime check reports live state and, where it compares against config, compares against the running configuration the owner applied.
- 2026-11-10: `Diagnostic.Fix` (JSON `"fix"`) sits beside the existing `FixSafety` (`"fix-safety"`) and `Repair` (`"repair"`). They answer different questions, and the `Fix` doc comment in `internal/core/diagnostic/types.go` states the difference: `Fix` is the command or config statement the operator runs, alone, as `opererr` holds it, drawn on its own line; `Repair` names a candidate automated repair an agent may apply (an id and a summary); `FixSafety` grades how safe that automated repair is. A diagnostic may carry `Fix` with neither of the others, and `Fix` never holds a repair id
- 2026-11-10: the daemon-only registry in `internal/core/diagnostic/doctor_daemon.go` (commit 311177442c) landed one day after this design and is the `runtime` phase built a second way. It is deleted, not extended, so the phase set stays the one declaration of when a check runs
- 2026-11-10: "a daemon answers" on the local route means the SSH ask succeeded, not that a process exists; a running daemon the operator cannot log in to is the fallback case (AC-14)

### Implementation Record, 2026-11-10

| Topic | Decision taken in implementation | Where |
|-------|----------------------------------|-------|
| Runtime check context | a `runtime` check gets `Platform` only | Data Flow 1b |
| Recent-events check | named `recent-events`, code `doctor-recent-events`; a row carries the event's own code only when its metadata is registered, else `doctor-recent-events` with the bus code in the message; fixes are bare `show errors` / `show warnings` | Data Flow 2a (amended), AC-11 amendment |
| Gate cover of a transient code | the check declares only `doctor-recent-events`, so a transient code counts as covered only when its owner's check declares it | Data Flow 2a (amended), AC-4 amendment |
| Local-route fallback fix | named by cause: `ze start`, `ze cli --user <name> -c "show doctor"`, `ze status`, `ze doctor` for R-5 | Data Flow 2 (amended), AC-2 amendment |
| Health rows | listed by the gate, never refused: `health.Register` takes no code | Data Flow 4b |
| Exit code | `ze doctor` exits 0 when the daemon answers only warnings | Data Flow 2c |
| dns-source findings | the command moves from the message into `Fix` | AC-13 amendment |

→ Decision: the doctor-coverage gate's baseline on 2026-11-10 is 2412 problem sites, 0 covered, 10 health rows, 175 registered doctor codes. The audit (P3) starts from it, and the area children's ACs move the covered count
→ Constraint: the known gate limits (function-value logging, non-literal messages, a `code` attribute holding a protocol or exit code) are walked by hand in the audit, and each is a site the area child of its owner fixes by naming a doctor code

## Owner Decisions

Thomas answered "yes to all recommendations" on 2026-11-08. No owner question remains open.

| # | Question | Decision (owner, 2026-11-08) | Where it lands in this spec |
|---|----------|------------------------------|-----------------------------|
| O-1 | Does local `ze doctor` ask the running daemon for its runtime checks? | Yes (option B): the local route asks the daemon when its socket answers and `config` names no file outside the daemon's config directories; when no daemon answers it falls back to the local checks with the `doctor-runtime-not-checked` warning | Transformation Path step 2, AC-2, AC-10, R-5, `TestDoctorLocalRouteAsksReachableDaemon`, `test/plugin/doctor-local-asks-daemon.ci` |
| O-2 | May a problem site be marked as a transient event that owes no doctor check? | No marker (option A): every Warn and Error log line carries a doctor code, and a transient class is reported as a recent-event row read from the report bus ring | Transformation Path step 2a, AC-4, AC-11, AC-12, R-1, R-6, the `doctor-recent-events` check |
| O-3 | Release timing of the area children | Every area child is release work (option A): P2 in `plan/release/validation/`, whatever release profile its feature sits in | AC-6, Files to Create |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A `runtime` phase in the existing phase set | a flag on `DoctorCheckContext` telling a check which route it runs on | a phase is declared once and the runner skips it; a flag lets every check decide, and one that forgets reads empty state as healthy |
| `Fix` as its own field | put the fix in `Help`, or only in the message | `Help` carries config validation help text, not a command; a separate field lets the renderer put the command on its own line on every surface |
| Gate judges added sites, report lists all | refuse every uncovered site in the tree | `ai/META-DECISIONS.md`: no gate refuses a commit for a red; a refusal that judges what the commit adds stands |
| Gate as an AST scan with registry lookup | a runtime registry of degraded states each mapped to a check | a runtime registry misses a new problem that never registers; the scan sees every new log line and raise |
| Local `ze doctor` asks a reachable daemon, falls back locally with the warning row (O-1) | local stays local and only prints the warning | owner decision: an operator on a broken box expects the box's real state; the local route stays for an unreachable daemon, as `ai/META-DECISIONS.md` requires |
| No transient marker; transient classes are recent-event rows from the report bus (O-2) | an exempt marker comment judged by review | owner decision: doctor reports every problem; the bus ring is already bounded and coded, so one bridge check reports it |
| Every area child is release work (O-3) | split by release profile | owner decision |

## Known Limitations
- The area children's content is the audit's output; they are named by AC-6.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass in the main checkout: `./le job run label doctor command go test -tags <tags> ./internal/component/doctor/... ./pkg/plugin/rpc/...`, `./le job run label le-arch command go test ./internal/le/arch/doctorcoverage/...`, `./le test functional plugin` and `./le test functional ui` for the two `.ci` files, with the tags `./le repo changed scope print both` names

**Goal Gates, corrected 2026-11-10 (the two lines above stay as written; this list governs):** no agent runs a whole functional suite or any `./le verify` command, so neither is evidence here (`ai/META-DECISIONS.md`, "Ze is pre-release"). The evidence is:
- [ ] AC-1..AC-14 all demonstrated
- [ ] Scoped package tests, each under `./le job run` with the tags `./le repo changed scope print both` names: `./pkg/plugin/rpc/`, `./pkg/plugin/sdk/`, `./internal/core/diagnostic/`, `./internal/component/doctor/...`, `./internal/component/config/system/`, `./internal/le/arch/doctorcoverage/`, `./internal/le/commit/changecheck/`, and the package that owns `doctor-recent-events`
- [ ] Named `.ci` files only, each run alone: `test/plugin/show-doctor-runtime-checks.ci`, `test/plugin/doctor-local-asks-daemon.ci`, `test/ui/doctor-runtime-not-checked.ci`, and the kept `test/plugin/doctor-dns-source.ci`, plus any existing `test/plugin/*doctor*.ci` whose expected output the `fix` line changes
- [ ] GitHub CI's `verify` run on the pushed commit is the whole-tree evidence, never a local run
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-1-audit-and-gate.md` only
