# Spec: data-store-maintenance-through-the-daemon

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Priority | P1 |
| Release reason | A shipped `ze doctor` report tells the operator to remove a store key with `ze data rm`, and that command is refused while the daemon that answered the report runs, so the warning cannot be cleared on a running box. The owner rule that every operator command runs through the daemon makes the daemon route owed work. |
| RFC gaps | - |
| Depends | `spec-single-owner-lifecycle-5-session-end-locals` (closed) (its fix (a) rewrites the same FlowSpec report for the configured case; this spec edits the removed-section text after that lands). The FlowSpec record and its two checks landed in bec9096156. |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The DDoS FlowSpec plugin records the rule it announces under the store key
`meta/ddos-flowspec/announcement` (bec9096156). While the `ddos flowspec`
section is not configured, the registered check `orphanRecordCheck`
(`internal/plugins/ddos/flowspec/doctor.go` `checkOrphanRecord`) reports a
stored record as `doctor-ddos-flowspec-orphan-announcement`. Its fix withdraws
the rule at the peers, and its message tells the operator to remove the record
with `ze data rm meta/ddos-flowspec/announcement`.

What the operator meets: on a box where the daemon runs, `ze data rm` is
refused with "store is owned by another process: `<dir>`; close its owner
before maintenance". No other command removes the key, so the warning stays
until the operator stops the daemon. A record removed while the daemon is
stopped clears the report, but the operator has to take the box down to clear
a warning.

Thomas decided (2026-11-12, "ok a + c"), on decision 1 of
`plan/handover/38-single-owner-lifecycle-children-2-and-5.md`:

- (a) while the section is configured, the fix is the plugin restart, built in
  `spec-single-owner-lifecycle-5-session-end-locals` (closed).
  Not this spec.
- (c) when the section has been removed, the operator withdraws with the
  command the report prints, then removes the record with `ze data rm`
  through the running daemon. This spec builds (c). The removed-section half
  of that spec's AC-16 ("The report clears when the record is removed") moves
  here.

The owner rule it is owed by (`ai/META-DECISIONS.md`, "Goal and scope"):
"Every operator command runs through the daemon on every command surface
(MCP, REST, gRPC, SSH, web) with the same output as the local `ze` command,
including the ones that answer without a daemon (`ze doctor`, `show config`,
`show yang` and the rest), so a remote operator or AI can debug the box. The
local route stays for when the daemon is unreachable. A command whose result
lives on the caller's side gets a daemon form designed for it, never an
exclusion."

Where the daemon route lives, its name, and how it is authorized are the
design pass's. This skeleton chooses no mechanism.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - where a command is served; "Served by the daemon too" and "Local commands already served by the daemon under the same path"
  → Decision: [design pass]
  → Constraint: [design pass]
- [ ] `docs/architecture/zefs-format.md` - the store and the `ze data` CLI
- [ ] `docs/architecture/hub-architecture.md` - the `request data` RPCs beside `request reload`
- [ ] `docs/guide/ddos-mitigation.md` - the FlowSpec record and the orphan report
- [ ] `ai/doors/system.md`, `ai/doors/config-yang.md`, `ai/doors/commands-cli.md`, `ai/doors/system-plugins.md`

**Key insights:**
- The store has one writer. The daemon holds the owner lock for its whole life, so any second writer is refused by design. A daemon route acts through the daemon's own handle; it does not lift the lock.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/storage/cli/register.go` - registers the `data` root (`registry.MustRegisterRootHandler`, Mode `offline`) and the local-data commands `show data list`, `show data registered`, `show data cat`
- [ ] `internal/component/config/storage/cli/main.go` - `subcommandHandlers`: `backup`, `restore`, `import`, `write`, `rm`, `list`, `cat`, `registered`, `check`, `repair`, `encode`; `cmdRm` opens the store with `openStore(path, true)`
- [ ] `internal/component/config/storage/open.go` - `OpenTree` sends the live tree to `Open`, then `openLive`, whose `lockOwner` calls `lockStoreFile`; `lockStoreFile` returns `ErrBusy` ("close its owner before maintenance") when the owner lock is held
- [ ] `internal/component/config/storage/store.go` - `ErrBusy` = "store is owned by another process"
- [ ] `internal/component/config/storage/ownership.go` - `ownershipLock` returns the other `ErrBusy` text ("stop its daemon before maintenance"), reached from `TransferOwnership` and `AcquireMaintenance`, not from `ze data rm`
- [ ] `internal/component/config/storage/cli/data_rpc.go`, `register_data_rpc.go` - the daemon RPCs `ze-config:data-backup` and `ze-config:data-restore` act on the live store through `DataRPCTarget.Store`
- [ ] `internal/component/config/storage/cli/cmd_backup.go` - `cmdBackup` refused with `ErrBusy` prints a hint naming the live form `request data backup path <absolute-file>`
- [ ] `internal/plugins/ddos/flowspec/doctor.go` - `orphanDiagnostic` (message names `ze data rm`, fix is the withdrawal), `corruptRecordDiagnostic` (fix is `ze data rm`), `checkOrphanRecord`

What each `ze data` verb does under a running daemon, as read:

| Verb | How it opens the live store | Under a running daemon | Daemon route today |
|------|-----------------------------|------------------------|--------------------|
| `cat`, `list`, `registered` | reader (`openStore(path, false)`) | answers | yes: `show data cat`, `show data list`, `show data registered` are local-data commands the daemon also serves |
| `check` | reader (`checkStoreHistory`) | not run; read only | not read |
| `rm` | writer (`cmdRm`) | refused, `ErrBusy` | none |
| `write` | writer (`cmdWrite`, `openOrCreateStore`) | refused, `ErrBusy` | none |
| `import` | writer (`cmdImport`, `openOrCreateStore`) | refused, `ErrBusy` | none |
| `repair` | writer (`repairStoreHistory`) | refused, `ErrBusy` | none |
| `backup` | writer (`cmdBackup`) | refused, with a hint to the live form | `request data backup` |
| `restore` | writer for `config` (`cmdRestoreConfig`); a full restore replaces the store and is offline only | refused | `request data restore ... config` |
| `encode` | not read | not read | not read |

`write`, `import` and `repair` share the gap `rm` has under the same owner
rule. This spec is owed `rm`, which AC-1 needs. Whether the other three join
this spec or each get a spec of their own is the design pass's to decide; by
the owner rule each is owed a daemon route either way, and none of them is
dropped.

**Behavior to preserve:**
- The local `ze data rm` with the daemon stopped, unchanged in output and exit code.
- One writer: the daemon's owner lock is never lifted or bypassed.

**Behavior to change:**
- `ze data rm` reaches a running daemon and removes the key through the daemon's own store.
- The removed-section orphan report names the two steps in order and why the order matters.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `request data` daemon RPCs | `internal/component/config/storage/cli/data_rpc.go` `DataRPCTarget`, `InstallDataRPC`, `handleDataBackup` | Act on the daemon's live store for backup and config restore | [design pass: the likely place a removal joins] |
| Local-data commands served by the daemon | `docs/architecture/api/commands.md`, "Served by the daemon too"; `command.MustRegisterLocalData` | `show data cat`, `list`, `registered` answer locally and through the daemon with one handler | [design pass] |
| The offline refusal hint | `internal/component/config/storage/cli/cmd_backup.go` `cmdBackup`, `daemonAddress` | A refused offline backup names the live form and the daemon's address | [design pass: the refused `rm` owes the same kind of hint] |
| The single-writer lock | `internal/component/config/storage/open.go` `openLive`, `lockStoreFile` | Refuses a second writer while the daemon owns the store | does not change: the daemon route writes through the owner |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [design pass]

### Transformation Path
1. [design pass]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ daemon | [design pass] | No |

### Changed-Wire Census

**Search evidence:** [design pass]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [finding] | [disposition] |

### Integration Points
- [design pass]

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
| A-1 | Removing a key through the daemon's handle is safe for a key a running plugin may also write | [design pass] | A plugin rewrites the record after the removal | [design pass] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A removal on every remote surface reaches keys that hold secrets or the active config | [design pass] | [design pass: authorization, as `show data cat` is `NotReadOnly`] |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [design pass] |
| How is it reverted? | [design pass] |
| Who else touches this path? | `spec-single-owner-lifecycle-5-session-end-locals` edits the same FlowSpec report |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze data rm <key>` with the daemon running | → | [design pass] | [design pass] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The daemon runs, the `ddos flowspec` section has been removed, and the store holds a record under `meta/ddos-flowspec/announcement` (moved from AC-16 of `spec-single-owner-lifecycle-5-session-end-locals`) | `ze doctor` reports `doctor-ddos-flowspec-orphan-announcement`. The operator runs the withdrawal the report prints, then removes the record through the running daemon, and the next `ze doctor` run reports no `doctor-ddos-flowspec-orphan-announcement` finding | Every other doctor finding stays: the removal clears only this record's report |
| AC-2 | The same condition as AC-1 | The report's text names the two steps in that order: withdraw the rule with the printed command, then remove the record through the daemon, with the exact command for each. It says why the order matters: removing the record first leaves the rule at the peers with no trace in the store and no report to find it | The configured-section text that `spec-single-owner-lifecycle-5-session-end-locals` writes (the plugin restart) is not changed here |
| AC-3 | The daemon runs and the operator removes a key with `ze data rm` | The key is removed through the daemon's own store and the command reports what it removed, as the local route does | The local route with the daemon stopped answers as it does today |
| AC-4 | A record that does not decode (`corruptRecordDiagnostic`), daemon running, section removed | The fix the report prints removes the record through the running daemon, and the next `ze doctor` run no longer reports it | The step that finds the rule at the peers stays manual: nothing in the store can name it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Removes `ddos flowspec`, sees the orphan report, follows its two steps | [design pass] | [design pass] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [design pass] | | | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [design pass] | `test/.../*.ci` | AC-1 over the real daemon | |

### Interop Tests (Scope: protocol)
N/A: Scope is cli. The withdrawal itself is the FlowSpec spec's.

## Files to Modify
- `internal/component/config/storage/cli/` - [design pass: the daemon route for `rm`]
- `internal/plugins/ddos/flowspec/doctor.go` - `orphanDiagnostic`, `corruptRecordDiagnostic`: the removed-section text and fix (AC-2, AC-4)

## Files to Create
- [design pass]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | [design pass] |
| YANG validation constraints | | [design pass] |
| YANG custom validators | | [design pass] |
| CLI commands/flags | | [design pass] |
| CLI grammar (keyword before value) | | [design pass] |
| Editor autocomplete | | [design pass] |
| Functional test for new RPC/API | | [design pass] |
| Pipe completeness | | [design pass] |
| Env var registration | | [design pass] |
| Doctor check for runtime dependencies | | [design pass] |
| Doctor code per problem class | | [design pass] |
| Prometheus counters/metrics | | [design pass] |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | `docs/guide/ddos-mitigation.md` |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | | |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | `docs/architecture/zefs-format.md` |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [design pass]
2. **Phase: [name]** -- [design pass]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The daemon route never opens a second writer; it acts through the owner's handle |
| Operator errors and doctor codes | A refused local `ze data rm` names the daemon form, as `cmdBackup` does for backup |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| [design pass] | [design pass] |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authorization | A remote removal of a key that holds a secret or the active config is not open to the read-only profile |

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
| `fix-command-refused-in-the-state-it-reports.md` | 2026-11-12 | FlowSpec orphan report names `ze data rm`, refused under the running daemon |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Clear a removed section's record with the withdrawal, then `ze data rm` through the daemon (Thomas, 2026-11-12, option (c)) | (a) plugin restart, which cannot run without the section; (b) a plugin command that withdraws and forgets in one step, a new surface | Owner decision; the owner rule already owes the daemon route |

## Known Limitations
- [design pass]

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
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit pkg path ./internal/component/config/storage/cli/`, `./le test unit pkg path ./internal/plugins/ddos/flowspec/`, and the functional test the design names for AC-1. No agent runs any `./le verify` command
- [ ] Feature code integrated, not library-only
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
- [ ] `/ze-review` gate clean, recorded with `./le spec review record`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-data-store-maintenance-through-the-daemon.md` only, in the same `./le commit create` script
