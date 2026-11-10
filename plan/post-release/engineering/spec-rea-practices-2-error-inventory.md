# Spec: rea-practices-2-error-inventory

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Measures compliance with `ai/rules/user-facing-errors.md`; it changes no shipped behaviour by itself |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

Parent: `plan/post-release/engineering/spec-rea-practices-0-umbrella.md`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

REA keeps an inventory of every user-visible error message (REA
`docs/user-visible-errors.csv`): location, trigger, current copy, user risk,
harm rank, proposed replacement, implementation status and verification
result. Ze has the rule every message must meet (`ai/rules/user-facing-errors.md`:
what went wrong, why, and the exact command that fixes it) but nothing that
shows which messages meet it.

Goal: an inventory Ze derives from the code rather than keeps by hand, so it
cannot drift from the messages it lists (`ai/rules/principles.md`, "Every
fact MUST be declared once"). It lists each place a message reaches a user,
its surface, and whether it meets the rule, and it becomes the worklist for
fixing the ones that do not. A change-set check stops new non-conforming
sites on the surfaces no other gate covers.

Decided at the research gate (Thomas, 2026-11-10): option A. This spec builds
no scanner beside the two release specs that already cover most of the
population. Both have landed and closed (`spec-operator-errors-name-the-fix`,
closure 41540585bb and dc5b80f1b6; `spec-doctor-coverage-1-audit-and-gate`,
closure c49b0b858a and 974cff10be). It reads their site lists, adds only the surface
writers neither covers, and joins everything into one report. Harm is a
derived pre-rank, not a judged field (Thomas confirmed, 2026-11-10). A judged
harm field, if ever wanted, is a declaration on the operator error type and
belongs to `spec-operator-errors-name-the-fix.md`.

Goals:

| # | Goal |
|---|------|
| G-1 | `./le arch error-inventory report` lists every user-visible message site in the tree, once, with surface, location, owner package, door area, status and pre-rank, and a count per surface |
| G-2 | Status is read from the code, never judged: a site that hands the user the operator error type with a fix meets the rule; a plain string does not |
| G-3 | `./le arch error-inventory check` refuses a commit that adds a plain message at a surface writer neither release spec gates |

Out of scope: rewriting any message (the operator-errors spec and its children
do that), a committed inventory file (the report is the inventory; a copy
would drift), and judging "what" and "why" prose (no tool can).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/user-facing-errors.md` - the rule the inventory measures
  → Constraint: a message meets the rule only when it says what went wrong in the reader's terms, why it matters, and the exact fix command; "what" and "why" are judgments of prose, only "names a fix" is a property a tool can read
- [ ] `internal/core/opererr` (`New`, `Wrap`) and `internal/le/arch/operatorerror` (`Sites`, `Scan`), built by spec-operator-errors-name-the-fix (closed 2026-11-10); `docs/architecture/cli/error-surface.md`, "The operator error" - one operator error type with problem, why, fix and cause, rendered by each boundary, plus a change-set check that a refusal from a registered operator handler carries the type
  → Constraint: that type and check own the error shape and the boundary check; this spec must consume the check's site list, never scan beside it
  → Constraint: `archoperatorerror.Sites(tree)` exposes the full site list as a Go function: every return site of every registered command handler, RPC handler and config verifier, and (since 605500cf11) every `plugin.Response{Status: StatusError}` answer a registered handler body builds, each with a Form (returned error or response), a Kind and a Status typed, plain or pass-through. The typed route into a response is `plugin.OperatorErrorResponse(*opererr.Error)`. Not read: an answer a helper function builds, a `Status` computed at run time, an `Error` assigned after the literal (`docs/architecture/cli/error-surface.md`, "The operator error")
- [ ] `internal/le/arch/doctorcoverage` (`Scan`, `./le arch doctor-coverage check` and `report`), built by spec-doctor-coverage-1-audit-and-gate (closed); `docs/architecture/doctor-and-health-checks.md`; the problem inventory `plan/audits/doctor-problem-inventory.md` with its 13 area files under `plan/audits/doctor-problem-inventory/`, whose area children are `plan/release/validation/spec-doctor-coverage-3-access-security.md` through `spec-doctor-coverage-15-system.md`
  → Constraint: its sites are every Warn or Error log line, report raise and health row; this spec must not count them a second time
  → Constraint: its covered set is read from the doctor check declarations in the parsed source (beb3a7e136), never from what le links; it does not judge a `diagnostic.Diagnostic` literal or its `Fix`, so no gate judges the message a doctor check prints
  → Decision: its `report` groups uncovered sites by door area and owner package (`DoorGroup`); this report reuses that grouping and its site records through `archdoctorcoverage.Scan`, which answers an `Inventory` of `Site` records
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`, `ai/doors/core.md`, `ai/doors/plugin-infra.md`
  → Constraint: the check lives in `le`, and product code never imports `internal/le`; a registry is the only list, so an inventory derived from the code is refused if it copies what a registry holds into a literal (`./le arch enumeration check`); `plugin.Response.Error` is wire, so a typed error reaches a plugin answer only through a declared field, never a renamed one
  → Constraint: `le arch error-inventory` lives at `internal/le/arch/errorinventory` (`TestEveryCommandIsFoundAtThePathItsNamePredicts`); an action stays callable as Go and answers through the shared renderer, never a private JSON mode
- [ ] `internal/le/arch/enumeration` `runCheck`, `runReport`, `inChangeSet`; `internal/le/commit/changecheck` `Register`, `Check`, `Change`
  → Decision: the check form judges only sites the change set adds; a site already in the tree is reported, never refused
  → Constraint: enumeration scopes by package through `repochanged.ScopeReport`; operator-error and doctor-coverage instead register a `changecheck.Check` from `init()` whose judge scans the commit's parent side and its carried side and refuses only what the carried side holds more of, so `./le commit create` runs them
- [ ] `internal/le/register.go` - the `le` composition root; its design page is `docs/architecture/core-design.md`, "Command engine and composition roots"
  → Constraint: a new arch package is one blank import beside `internal/le/arch/doctorcoverage`, `enumeration` and `operatorerror`; nothing else in the root changes, so `docs/architecture/core-design.md` needs no edit

**Key insights (research, 2026-11-10, current-state rows rechecked the same day after both release specs closed):**
- Both overlapping specs are implemented: `internal/core/opererr` holds the operator error type, `diagnostic.Diagnostic` carries a `Fix` field (`internal/core/diagnostic/types.go`), and `internal/le/arch/operatorerror` and `internal/le/arch/doctorcoverage` both exist, both register into `internal/le/commit/changecheck`, and both declare the `user-facing-errors/directives/an-error-names-the-cause-and-the-fix` point
- `./le arch operator-error report` (605500cf11) counts the handlers' error answers as 0 typed, 187 plain, 43 pass-through; `./le arch doctor-coverage report` (beb3a7e136) counts 2413 problem sites, 1 covered, against 191 declared doctor codes
- A `Diagnostic` literal's `Fix` is judged by neither package, so the doctor message surface has no gate; the design below still names doctor-coverage as its population owner, which A-5 records as partly broken
- A user-visible message is not one construction site: it is a surface writer, plus a wrap chain of two to four literals from different functions, plus a leaf from the OS or a library. About 57% of `fmt.Errorf` calls wrap with `%w`, so a site scan finds fragments, not the text a user reads. The status is therefore judged at the writer, by the type it receives
- About 2% of product error construction lines and 4% of `le` lines name a command; structured cause-and-fix forms exist only ad hoc (`remedy`, `Fix`, 75 `"fix"` log attributes, 3 `remedy`, 2 `hint`)
- No existing signal ranks harm: severity, log level, gRPC and HTTP codes and doctor exit codes classify, they do not say outage, data loss or silent misconfiguration

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `ai/rules/user-facing-errors.md` - states the message rule; no inventory exists
- [ ] `internal/core/diagnostic/types.go` - `Diagnostic` carries Code, Severity, Message, Help, and `Fix` (the command or config statement the operator runs, never a repair id) beside FixSafety and Repair (an automated repair)
- [ ] `internal/core/report/report.go` - the report bus; an `Issue` is keyed by Source, Code and Subject with a declared severity; its raise sites are in doctor-coverage's site list
- [ ] `internal/component/plugin/types.go` - `Response.Error` is a plain string; `OperatorErrorResponse(*opererr.Error)` is the one typed route into it; operator-error's report counts the handlers' `StatusError` answers it reads as 0 typed, 187 plain, 43 pass-through
- [ ] `internal/le/arch/operatorerror` `Sites`, `Scan`, `responseSites` - handler return sites and error answers, typed, plain or pass-through
- [ ] `internal/le/arch/doctorcoverage` `Scan`, `Inventory`, `Site` - log, report-bus and health-row problem sites, covered or not against the doctor codes declared in source
- [ ] `cmd/ze/ze_core_dispatch.go` - `reportDispatchError` prints `error: ` and the whole Go error chain, then exits 1
- [ ] `internal/le/doc/ste/extract.go` - `unitsGo` reads Go comment blocks only, never string literals, so no prose gate reads a message today

**Population, by surface (2026-11-10; rows owned by the release specs are their tools' counts, the rest are grep counts over non-test Go):**

| Surface | Writer | Sites | Population owner |
|---------|--------|-------|------------------|
| Daemon commands (CLI, SSH, MCP, REST) | returned errors and `plugin.Response{Status: StatusError}` answers from a registered handler | error answers 0 typed, 187 plain, 43 pass-through (`./le arch operator-error report`); returned errors in the same `Sites` list | operator-errors spec, `archoperatorerror.Sites` |
| Config verify | registered config verifiers (`KindConfigVerifier`, `KindSDKConfigVerify`) | in the same `Sites` list | operator-errors spec |
| Logs, report bus, health rows | slog Warn and Error, `report.RaiseWarning`, `RaiseError`, health rows | 2413 sites, 1 covered, 191 declared codes (`./le arch doctor-coverage report`) | doctor-coverage spec, `archdoctorcoverage.Scan` |
| `ze doctor` | `Diagnostic` literals a doctor check returns | not counted by either tool | doctor-coverage reads the checks' declared codes only; no gate judges a literal's `Fix` (A-5) |
| Local `ze` CLI | `reportDispatchError`, `command.LocalRefusal` | 112 refusals, about 1,250 stderr writes | this spec |
| MCP | `fail`, `ErrResult`, `refusalResult` | 69 | this spec |
| REST | `writeError`, `writeJSONError` | 74 | this spec |
| Web UI | `http.Error`, `internal/core/errorfragment` | about 217 | this spec |
| gNMI | `status.Error`, `status.Errorf` | 66 | this spec |
| `./le` tooling | `human.Refusal`, the `Finding` report types | about 20 report types | this spec |

**Behavior to preserve:**
- No product message, exit code or answer shape changes: this spec reads code and refuses commits, it ships nothing in the `ze` binary
- The operator-error and doctor-coverage checks keep their own refusals; this spec's check never refuses a site theirs judge

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Operator error type and boundary check (built, 728fb3b57e; error answers added 605500cf11) | `internal/core/opererr` `New`, `Wrap`; `internal/component/plugin` `OperatorErrorResponse`; `internal/le/arch/operatorerror` `Sites`, `Scan` | One error type with problem, why, fix and cause, and `./le arch operator-error check` over registered handler boundaries, judging returned errors and `StatusError` answers, also run by `./le commit create` through `internal/le/commit/changecheck` | Extends: its site list is the inventory's command, RPC and verifier population, and its type is what "meets the rule" means at every writer |
| Doctor fix field and problem-site scan (built; spec closed c49b0b858a, 974cff10be) | `internal/core/diagnostic/types.go` `Diagnostic.Fix`; `internal/le/arch/doctorcoverage` `Scan`, `./le arch doctor-coverage check` and `report`, registered into `internal/le/commit/changecheck`; `plan/audits/doctor-problem-inventory.md` and its 13 area files; area children `plan/release/validation/spec-doctor-coverage-3` to `-15` | `Fix` on the diagnostic record; a scan of log lines, report raises and health rows judged against the doctor codes declared in source (beb3a7e136), its report grouped by door area and owner package | Extends: its `Scan` is the inventory's log, report-bus and health-row population; not re-scanned here. It does not cover `Diagnostic` literals (A-5) |
| Diagnostic code registry | `internal/core/diagnostic/registry.go` `CodeMeta`, `Register`; `codes.go` `builtinCodes` | A code declares title, description and examples once; `ze show diagnostic` renders it | Extends: a coded row names its code, so the reader follows it to its declaration; nothing is copied |
| Diagnostic record | `internal/core/diagnostic/types.go` `Diagnostic` | Shared by doctor, config validation and `ze config fix --plan`; carries `Fix` | Extends, through its `Fix` field; its severity feeds the pre-rank |
| Report bus | `internal/core/report/report.go` `Issue`, `RaiseWarning`, `RaiseError` | Coded operator issues with a declared severity | Extends: its severity feeds the pre-rank |
| Web error fragment | `internal/core/errorfragment` | One renderer for refused htmx requests on the three HTTP UIs | Extends: one of the surface writers this spec scans |
| STE prose checker | `internal/le/doc/ste/ste.go`, `extract.go` `unitsGo` | Reads Markdown, Go comments and YANG descriptions | Does not apply: it reads no string literal, and "what" and "why" are out of scope |
| Enumeration gate | `internal/le/arch/enumeration` `runCheck`, `runReport`, `inChangeSet` | AST check and report forms, change-set scoped by package | Extends as the model: the same check and report pair |
| Commit change-set seam | `internal/le/commit/changecheck` `Register`, `Check`, `Change` | Runs every registered check's judge on what a commit carries against its parent; operator-error and doctor-coverage register here | Extends: the check registers here, as its two siblings do |
| Rule points | `ai/rules/points/user-facing-errors/` | Three directives; `internal/le/arch/operatorerror` and `internal/le/arch/doctorcoverage` both declare `// ze point: user-facing-errors/directives/an-error-names-the-cause-and-the-fix` | Extends: the check binds the same cause-and-fix directive |
| Journal classes | `plan/journal/error-names-no-fix-command.md` (10 rows, all fixed, none open), `failure-report-names-a-command-that-does-not-exist.md`, `failing-gate-prints-no-cause.md` | Rows found by hand | Extends: every open row whose site still exists must appear in the report with status `plain`, which is an acceptance test |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le arch error-inventory report` or `check`, run on the checkout by a developer, an agent or `./le commit create`

### Transformation Path
1. Parse the tree's non-test Go files once (`go/ast`, the same loader the other `arch` checks use)
2. Ask `operatorerror` for its handler-return sites and `doctorcoverage` for its log, report-bus and doctor sites, each as typed site records
3. Find every call to a surface writer this spec owns (table above); for each, classify the message argument: the operator error type carrying a fix is `typed`; a string or plain error whose literal fragments hold a `ze `, `./le ` or config-path command is `plain-names-command`; anything else is `plain`
4. Merge the three lists, keyed by file and line, so a site two sources name appears once (source precedence: operator-errors, then doctor-coverage, then this spec)
5. Give each site its owner package and door area (the door-base map `ai/doors/README.md` declares), and its pre-rank from declared facts: operator surface with error severity or a refusal = 1, operator surface with warning severity = 2, `./le` tooling = 3
6. `report` renders the rows sorted by pre-rank, surface, file and line, then one count line per surface; `check` keeps only rows the change set adds on this spec's own writers with status `plain` or `plain-names-command`, and refuses when any remain

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `le` arch packages | Go calls into `operatorerror` (`Sites`) and `doctorcoverage` (`Scan`) for their site records | Yes for existence and signature (both exported, A-5); the call itself is unbuilt |
| `le` to the source tree | Read-only AST parse | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `pkg/`, `.github/`, `le` script, Go and shell; terms `error-inventory`, `errorinventory`. No producer or reader exists today: the report and check are new `le` output, and no other program parses them. Known control: `internal/le/arch/enumeration` `runReport` output, which nothing outside `le` reads either.

No existing wire changes. The report's text and its JSON form (through the shared renderer) are new, read only by people and by `./le commit create` through the check's exit code.

### Integration Points
- `internal/le/register.go` - one blank import for the new package
- `./le commit create` - runs the check through `internal/le/commit/changecheck`, which the operator-error and doctor-coverage checks register into from `init()`
- `internal/le/arch/operatorerror` `Sites`, `internal/le/arch/doctorcoverage` `Scan` - their exported site lists

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Reads the other checks' site lists instead of re-scanning their surfaces |
| No unintended coupling (components stay isolated) | Yes | `le` only; no product import |
| No duplicated functionality (extends existing, does not recreate) | Yes | One scan per surface across the three packages; merge by file and line |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Development tooling, not a wire path |
| Registration over hardcoding, outbound | Yes | The package registers its actions with `leroot.RegisterActions` from `init()` |
| Registration over hardcoding, inbound | Partly | The surface writer list (table above) is this spec's own population, declared once in the package; a new writer kind needs a row there (R-3) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Most user-visible messages can be found mechanically from the code | REA builds its inventory by hand; Ze's rule forbids a hand copy | The inventory needs a registration point for messages, a larger change | Research survey of error construction sites | partly broken (2026-11-10): every writer and about 97% of construction sites are found by AST with their literal text, but the text a user reads is a wrap chain no site scan resolves; only a typed error at the writer makes "meets the rule" mechanical, so status is judged by type at the writer |
| A-2 | The operator-errors and doctor-coverage specs land before this one is implemented | Both are release work (P1, P2); this is P3 post-release; Depends names both | This spec would build its own scanner beside theirs | Their status when this spec is claimed | validated (2026-11-10): both landed and closed; `spec-operator-errors-name-the-fix` closure 41540585bb, dc5b80f1b6; `spec-doctor-coverage-1-audit-and-gate` closure c49b0b858a, 974cff10be |
| A-3 | Harm cannot be derived; at most a pre-rank from surface tier and declared severity | No `CodeMeta`, `Issue` or exit-code field encodes operator impact | A judged field is needed, declared once on the operator error type in that spec | Thomas, 2026-11-10: the derived pre-rank is enough | confirmed |
| A-4 | A message argument at a writer can be classified by its static type and literal fragments | `go/types` resolves the argument's type; literals are AST constants | A writer that receives a value built far away classifies as `plain` even when it carries a fix | Unit tests over testdata, then the first report run | unvalidated |
| A-5 | `operatorerror` and `doctorcoverage` expose their site lists as Go functions | Both specs name `report` or check actions; Ze actions stay callable as Go (`le-tooling.md`) | Phase 1 adds the exported function to that package, inside its own spec's contract | Reading both packages when this spec is claimed | partly broken (2026-11-10): `archoperatorerror.Sites(tree)` answers `[]Site` and `archdoctorcoverage.Scan(tree, codesFloor, doorsOf)` answers an `Inventory` of `Site` records, so both lists exist as Go functions; but doctor-coverage's sites are log lines, report raises and health rows only, and neither package lists a `Diagnostic` literal, so the `ze doctor` message surface the design assigns to doctor-coverage has no producer of its site list |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A third scanner grows beside the two release specs | This package parses log lines, report raises or handler returns | Those surfaces come only from the two packages' functions; the review checklist row refuses any second scan |
| R-2 | The `plain-names-command` heuristic reads a command where none fixes the problem | A row marked `plain-names-command` whose command is unrelated | It is never counted as meeting the rule; only `typed` meets it, so the heuristic only sorts the worklist |
| R-3 | A new surface writer kind is added and the inventory never sees it | A new HTTP, RPC or CLI answer helper with no row | The writer list is one declaration in the package; the docs page says a new writer helper adds its row, and the reviewer of that helper checks it |
| R-4 | The report is thousands of rows and unusable as a worklist | Row count far above the per-surface counts | Rows sort by pre-rank first; the count line per surface is the summary; filters by surface and door area come through the shared renderer's pipe operators |
| R-5 | A site both release specs and this spec name is counted twice | Total above the sum of distinct file and line pairs | Merge by file and line with a fixed precedence (AC-2) |
| R-6 | The check refuses a commit for a site another gate already judges | A refusal naming a handler-return or log site | The check judges only this spec's own writers (AC-5) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A false refusal at commit time on a web, REST, gNMI, MCP, local CLI or `le` error site, or a wrong row in a report |
| How is it reverted? | Single commit revert; nothing in the shipped binary depends on it |
| Who else touches this path? | The operator-errors and doctor-coverage specs, whose packages this one calls |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch error-inventory report` | → | `errorinventory` report action over a testdata tree | `TestErrorInventoryReportThroughAction` |
| `./le arch error-inventory check` on a change set adding a plain `http.Error` | → | `errorinventory` check action | `TestErrorInventoryCheckRefusesAddedPlainSite` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le arch error-inventory report` over a testdata tree holding one site per surface | Prints one row per site with surface, file:line, owner package, door area, status (`typed`, `plain-names-command` or `plain`) and pre-rank, then one line per surface `surface=<name> sites=<n> typed=<n> plain-names-command=<n> plain=<n>` | None |
| AC-2 | A testdata site that both the operator-error list and this spec's writer scan name | It appears once, with the operator-error source's status | None |
| AC-3 | A writer handed the operator error type with a fix; the same writer handed `errors.New("bad")`; handed a literal that contains `ze config validate` | Statuses `typed`, `plain`, `plain-names-command` | `plain-names-command` survives as its own status because the heuristic cannot prove the command fixes the problem (R-2) |
| AC-4 | The report run on the real tree | Every open row of `plan/journal/error-names-no-fix-command.md` whose cited site still exists appears with status `plain` or `plain-names-command` | A row whose site was deleted or already fixed is absent, and the docs page says why |
| AC-5 | A change set adding `http.Error(w, "unauthorized", 401)` in a web package; adding a plain `fmt.Errorf` return from a registered command handler; adding a log line | `check` refuses the first, naming file:line, the status and the fix (hand the writer the operator error type with a fix); it does not refuse the second or third, which the operator-error and doctor-coverage checks judge | Sites already in the tree are reported, never refused, so the check can land on a tree full of plain messages |
| AC-6 | `./le arch error-inventory report \| json` | The same rows as JSON through the shared renderer, no private JSON mode | None |
| AC-7 | `./le commit create` with a staged change matching AC-5's first case | Refused with the check's message | None |
| AC-8 | The check's source | Declares the cause-and-fix directive of `ai/rules/points/user-facing-errors/`; `./le ai rules report gate-map` lists it | None |
| AC-9 | Docs | A page under `docs/contributing/` explains the report, the three statuses, the pre-rank, the writer list and how to add a writer; `ai/rules/user-facing-errors.md` and `ai/INDEX.md` point at it | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Thomas asks which messages to fix first | `./le arch error-inventory report` → rows by pre-rank | `TestErrorInventoryReportThroughAction`, AC-4 run |
| 2 | A developer adds a bare web refusal | `./le commit create` → check refuses with the fix | `TestErrorInventoryCheckRefusesAddedPlainSite` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestErrorInventoryReportThroughAction` | `internal/le/arch/errorinventory/actions_test.go` | AC-1, AC-6: rows and count lines through the registered action | |
| `TestErrorInventoryMergesSourcesOnce` | `internal/le/arch/errorinventory/inventory_test.go` | AC-2 | |
| `TestErrorInventoryClassifiesWriterArgument` | `internal/le/arch/errorinventory/classify_test.go` | AC-3, A-4: typed, plain, plain-names-command | |
| `TestErrorInventoryPreRank` | `internal/le/arch/errorinventory/inventory_test.go` | Pre-rank from tier and severity | |
| `TestErrorInventoryCheckRefusesAddedPlainSite` | `internal/le/arch/errorinventory/actions_test.go` | AC-5 first case | |
| `TestErrorInventoryCheckLeavesOtherGatesSites` | `internal/le/arch/errorinventory/actions_test.go` | AC-5 second and third cases, R-6 | |
| `TestErrorInventoryCheckIgnoresExistingSites` | `internal/le/arch/errorinventory/actions_test.go` | AC-5 survivor: a site not in the change set is reported, not refused | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Pre-rank | 1-3 | 3 | 0 (refused as a defect) | 4 (refused as a defect) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| AC-4 real-tree run | `./le arch error-inventory report` in the main checkout, recorded in this spec | The journal's hand-found rows reappear mechanically | |
| AC-7 | a `./le commit create` dry-run test in `internal/le/commit` beside the other change-set gate tests | The gate is wired into commits | |

### Interop Tests (Scope: protocol)
- N-A: tooling with no protocol peer.

## Files to Modify
- `internal/le/register.go` - blank import of the new package
- none in `internal/le/commit`: the check joins the commit through `changecheck.Register` from the new package's `init()`, as operator-error and doctor-coverage do
- `internal/le/arch/operatorerror`, `internal/le/arch/doctorcoverage` - none for their existing lists, which are exported (A-5); the `Diagnostic` literal gap A-5 records is unassigned
- `ai/rules/user-facing-errors.md` - point at the inventory page (through `ai/rules/rule-format.md`, then `./le ai rules update condensed`)
- `ai/INDEX.md` - keyword row
- `docs/contributing/README.md` - list the new page

## Files to Create
- `internal/le/arch/errorinventory/` - `register.go`, the writer table, classifier, merge, report and check actions, tests, `testdata/`
- `docs/contributing/error-inventory.md` - AC-9

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `le` tooling, no product config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | Yes | `le arch error-inventory report` and `check`, registered with `leroot` |
| CLI grammar (keyword before value) | Yes | Verb then keyword; checked by `./le cli grammar` |
| Editor autocomplete | N-A | No YANG change |
| Functional test for new RPC/API | N-A | No RPC; the action tests and the AC-4 run cover the entry point |
| Pipe completeness | Yes | Output through the shared renderer, so `\| json` and `\| table` apply (AC-6) |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | Development tooling, no runtime dependency of `ze` |
| Prometheus counters/metrics | N-A | No runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Development tooling |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/contributing/error-inventory.md` (an `le` command) |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | Contributor tooling |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/error-inventory.md` names the check in the commit gate list |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | An `le` command registers; `./le help` and completion derive it |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec` on this spec at implementation and re-read each page it names |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/rules/user-facing-errors.md` gains the pointer |

### Discovery (`ai/rules/repo-maintenance.md`)
| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` keyword row "error inventory, user-visible messages, which errors name a fix" |
| Rule that prevents regression | `ai/rules/user-facing-errors.md`, bound by the check (AC-8) |
| Registry that prevents drift | The writer table in the package, plus the two release specs' registries of handlers and doctor codes |
| Verification that proves it | The action tests and the AC-4 real-tree run |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register `le arch error-inventory report` and `check` returning an empty inventory; failing action tests
   - Tests: `TestErrorInventoryReportThroughAction`, `TestErrorInventoryCheckRefusesAddedPlainSite`
   - Files: `internal/le/arch/errorinventory/register.go`, `internal/le/register.go`
   - First: A-2 holds (both specs landed); re-read `operatorerror` `Sites` and `doctorcoverage` `Scan` for any change since 2026-11-10, and settle the `Diagnostic` literal gap A-5 records before Phase 3
2. **Phase: Writers and classification** -- the writer table, the classifier (AC-3), pre-rank
   - Tests: `TestErrorInventoryClassifiesWriterArgument`, `TestErrorInventoryPreRank`
3. **Phase: Merge and report** -- consume both packages' site lists, merge (AC-2), render (AC-1, AC-6), real-tree run (AC-4) recorded in this spec
   - Tests: `TestErrorInventoryMergesSourcesOnce`, `TestErrorInventoryReportThroughAction`
4. **Phase: Check and commit gate** -- change-set scoping (AC-5), commit wiring (AC-7), point binding (AC-8)
   - Tests: the three check tests, the commit dry-run test
5. **Phase: Docs** -- AC-9, in the same phase as the code it describes where possible

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| No second scanner | No code in `errorinventory` parses log calls, report raises, doctor literals or handler returns; those come only from the two packages (R-1) |
| Correctness | Status `typed` only for the operator error type carrying a non-empty fix; `plain-names-command` never counts as meeting the rule |
| Merge | One row per file and line, with the stated precedence |
| Check scope | Refuses only this spec's own writers, only for sites the change set adds |
| Rule: `ai/rules/principles.md` | No committed copy of the inventory; the writer table is the only list of writers |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Report action | `TestErrorInventoryReportThroughAction`, AC-4 run |
| Check action and commit gate | The check tests and the commit dry-run test |
| Docs page | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The scan reads source files of the checkout only; a file that does not parse is reported by name, never a panic |
| Error leakage | N-A: the report prints source locations and literal text already in the repository |
| Resource exhaustion | One parse of the tree, shared with the other arch checks' loader where it exists |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A release spec has not landed (A-2) | Stop; report; this spec waits |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Depend on the two release specs and report only (option A, Thomas, 2026-11-10) | B: fold into the operator-errors spec as one criterion; C: own scanner now | C builds beside two designed mechanisms; B was acceptable but A keeps the umbrella's goal in its own spec |
| Status judged by the type the writer receives | Judge the text of the wrap chain | The text a user reads is a chain no static scan resolves; the type is a fact the code declares |
| Harm as a derived pre-rank (Thomas, 2026-11-10) | A judged harm field | Nothing in Ze records impact; a judged field would be a new declaration on the operator error type |
| No committed inventory file | A CSV like REA's | A copy drifts from the code it lists |
| Check refuses only new sites on this spec's writers | Refuse every plain site | The tree is full of plain messages today; the worklist fixes them, the check stops new ones |

## Known Limitations
- "What went wrong" and "why" are not judged; only the presence of a typed fix is
- A message built far from its writer and handed over as a plain error classifies as `plain` even when its text names a fix (A-4)

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted runs pass in the main checkout: the unit tests of `internal/le/arch/errorinventory` and the touched commit tests, through `./le job run label unit-pkg command go test -tags ...`
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for the pre-rank range
- [ ] Targeted runs: no `./le test unit <group>` covers `internal/le`, so the evidence is `./le job run label unit-pkg command go test -race -tags ... ./internal/le/arch/errorinventory/ ./internal/le/commit/`, plus `./le cli grammar` and `./le doc check all`

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-rea-practices-2-error-inventory.md` only
