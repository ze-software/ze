# Spec: dependency-approval-ledger

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | Every third-party package linked into the shipped binary carries supply-chain, safety, performance and install cost, and Thomas approves each use. Without a gate an agent can add a use nobody approved. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas's rule (`ai/META-DECISIONS.md`, "Code and dependencies"): a third-party
module is approved only for its approved use, and each new use goes to him
first. Today nothing enforces it: `ze-go-style.md` says "No new third-party
import enters until Thomas agrees", and the only check is a reader.

Build three things:

1. A ledger: one tracked file listing, per third-party package, the Ze
   directories allowed to import it, seeded from today's tree.
2. A gate in `./le commit create` that refuses a commit whose Go adds a
   third-party import the ledger does not allow for that importer, naming the
   package, the importer and the exact owner command.
3. An owner-only command, `./le deps approve package <path> importer <dir>
   reason "<why>"`, that refuses without an interactive terminal and writes the
   ledger row.

Plus `./le deps check`, which compares the whole tree with the ledger, so the
seed and every later state can be proved.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - le is one area per package, registered from `init()`
  → Constraint: `le deps` lives in `internal/le/deps` (`TestEveryCommandIsFoundAtThePathItsNamePredicts`)
  → Constraint: an action answers through the shared renderer and stays callable as Go
- [ ] `docs/contributing/committing.md` - the gates `./le commit create` runs and their exit codes
  → Decision: a refusal is exit 1 (a gate ran and refused), a file the gate cannot read is exit 3 (`couldNotRun`)
- [ ] `docs/contributing/ze-go-style.md`, "Simplicity and debt" - the third-party import rule this spec enforces
- [ ] `docs/architecture/core-design.md`, "Command engine and composition roots" - how an le area registers (`internal/le/register.go`)
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the stage list `deps check` joins (`internal/le/verify/engine/stages.go`)
- [ ] `docs/features/ai-first.md` - the agent-governance features this gate joins

**Key insights:**
- A gate that judges what the commit itself adds is not a red and may refuse (`ai/META-DECISIONS.md`, "Ze is pre-release").
- Owner-only issuance already has one shared gate: `human.RequireAuthorization`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/prepare.go` - `Create` runs `checkSourceGates`, `checkVerificationGates`, then `specMechanismsRefusal` for a native checkout
- [ ] `internal/le/commit/specmechanisms.go` - a content gate over `workingTreeSides` (HEAD text and selected blob)
- [ ] `internal/le/le/human/human.go` - `RequireAuthorization` refuses unless stdin and stdout are terminals, and prints the command for Thomas
- [ ] `internal/le/rfc/approve.go` - `approveAnswer`, the shape of an approval action that admits before it writes
- [ ] `internal/le/arch/tier/ownership.go` - import-edge gates over the whole source graph
- [ ] `internal/le/register.go` - one blank import per le area
- [ ] `internal/le/verify/engine/stages.go` - `fullStages`, the whole-tree release gates

**Behavior to preserve:**
- Every existing gate of `./le commit create`, its exit codes and its output lines.
- Every third-party import in the tree today stays admitted (the seed covers it).

**Behavior to change:**
- A commit adding an unapproved third-party import, or a ledger row no owner approval recorded, is refused.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Commit content gates | `internal/le/commit/prepare.go` `Create`, `specMechanismsRefusal` | Judge the blobs a prepared commit carries against HEAD before the script is written | Extends: the dependency gate is one more content gate in the same `native` branch, reading the same `workingTreeSides` |
| Tier and ownership import gates | `internal/le/arch/tier/ownership.go` `pluginOwnershipGate` | Judge Ze-internal import edges over the whole tree at verify time | Does not apply: it judges Ze-to-Ze edges, and a third-party edge has a different authority (Thomas's approval, not the package layout). The new gate follows its idea (judge an edge, importer by directory) but judges only the diff, so it stays cheap at commit time |
| Owner-only terminal gate | `internal/le/le/human/human.go` `RequireAuthorization` (used by `commit create attest`, `commit push`, `rfc approve` rulings, `spec review` authorization) | Refuses issuance unless stdin and stdout are terminals, and prints the exact command for Thomas | Extends: `deps approve` calls it before it reads or writes anything |
| Approval files under `tmp/` | `internal/le/rfc/approve.go` `ApprovalPath` | An approval command writes an untracked record the commit gate reads | Extends: `deps approve` writes `tmp/dependency-approvals.txt`, and the gate admits a ledger row a commit adds only when that record holds it |
| Vendor tree and `go.mod` | `vendor/modules.txt`, `go.mod` | Declare which modules are vendored, not who may import them | Does not apply: it holds a different fact (what is present); the ledger holds who may use it. A new module still lands in its own vendor commit first |
| Verify stage list | `internal/le/verify/engine/stages.go` `fullStages` | Lists every whole-tree gate the release check runs | Extends: `deps check` joins `fullStages` as a whole stage |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le commit create ... file <path.go>`: the staged Go blob and HEAD's blob.
- `./le deps approve package <path> importer <dir> reason "<why>"` from Thomas's terminal.

### Transformation Path
1. The commit gate parses the imports of each carried `.go` file on both sides (`go/parser`, imports only, build tags ignored).
2. It keeps the third-party paths the after side adds, and asks the ledger whether the file's directory may import each.
3. It reads the ledger as the commit carries it, and refuses a row the commit adds that `tmp/dependency-approvals.txt` does not hold.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le commit gate to le deps | Go call into `internal/le/deps` | Yes, unit tests |

### Changed-Wire Census
None: tooling only, no wire format.

### Integration Points
- `Create` calls `dependencyRefusal` after `specMechanismsRefusal`.

### Architectural Verification
- [ ] No product package imports `internal/le` (`TestNormalZeLinksNoInternalLe`).

## Risks & Assumptions

### Assumptions
- A third-party import path is one whose first element holds a dot and which is not under the Ze module path read from `go.mod`.
- The importer is the file's repository directory; a `_test.go` file is judged like any other file of its directory.

### Risks
- An agent can write the ledger or the approval record by hand. The gate stops a tool call, not a determined wrapper, as `human` says of itself. An edit-hook refusal of both paths is an owner decision (hook pins are his).

## Blast Radius

Every `./le commit create` in the Ze checkout runs the new gate. It reads only the `.go` files the commit carries and one small file.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le commit create` with a Go file adding an import | → | `Create`, `dependencyRefusal` | `TestDependencyRefusalRefusesUnapprovedImport` |
| `./le deps approve` without a terminal | → | `admit`, `human.RequireAuthorization` | `TestApproveRefusesWithoutTerminal` |
| `./le deps approve` from a terminal | → | `admit`, `Approve` | `TestApproveWritesRowAndRecord` |
| `./le deps check` | → | `deps.Check` | `TestCheckReportsUncoveredImport` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A commit carries a Go file that adds a third-party import the ledger does not list for that file's directory | Refused, exit 1, naming the package, the importer, that Thomas approves it, and the exact `./le deps approve` command | None: that is the rule |
| AC-2 | The same import, with a ledger row for that package and directory | Admitted | Admitted because the owner approved it |
| AC-3 | A commit keeps or removes an existing third-party import | Admitted | A diff that adds nothing asks nothing |
| AC-4 | A commit adds a ledger row that `tmp/dependency-approvals.txt` does not hold | Refused, naming the row and the owner command | Removing a row is admitted: it narrows approval |
| AC-5 | `./le deps approve` run without interactive stdin and stdout | Refused, exit 2, before any write, printing the command for Thomas | None |
| AC-6 | `./le deps approve` from a terminal | The ledger gains the row (sorted, once) and the approval record gains it | Re-approving an existing row replaces its reason |
| AC-7 | `./le deps check` over the seeded tree | Exit 0; an import the ledger lacks is exit 1 naming it; an unused row is listed as stale without failing | Stale rows survive: removing a use does not need the owner |
| AC-8 | `le help`, `le help ai`, completion | List `deps approve` and `deps check` | Derived from the action table |
| AC-9 | A commit that deletes the ledger | Refused | None: an absent ledger would admit any row |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseLedger` | `internal/le/deps/deps_test.go` | rows, comments, malformed rows refused | |
| `TestAddedThirdParty` | `internal/le/deps/deps_test.go` | only added third-party paths, Ze and stdlib ignored | |
| `TestApproveRefusesWithoutTerminal` | `internal/le/deps/approve_test.go` | AC-5 | |
| `TestApproveWritesRowAndRecord` | `internal/le/deps/approve_test.go` | AC-6 | |
| `TestCheckReportsUncoveredImport` | `internal/le/deps/check_test.go` | AC-7 | |
| `TestDependencyRefusalRefusesUnapprovedImport` | `internal/le/commit/dependencies_test.go` | AC-1, a test file judged like any other | |
| `TestDependencyRefusalAdmitsApprovedKeptAndDroppedImports` | `internal/le/commit/dependencies_test.go` | AC-2, AC-3, vendor not judged | |
| `TestDependencyRefusalRefusesUnrecordedLedgerRow` | `internal/le/commit/dependencies_test.go` | AC-4, AC-9, the seed admitted | |
| `TestJudgedSkipsOtherModules` | `internal/le/deps/deps_test.go` | vendor, testdata and nested modules not judged | |
| `TestModuleOf` | `internal/le/deps/deps_test.go` | the refusal names the vendored module | |
| `TestLedgerWithRendersSortedOnce` | `internal/le/deps/deps_test.go` | AC-6 re-approval replaces the reason | |
| `TestValidApprovalRefusesFirstParty` | `internal/le/deps/approve_test.go` | stdlib and Ze paths need no row | |
| `TestFullStagesMatchesNativeActionPopulation` | `internal/le/verify/engine/verifyengine_test.go` | `deps check` in the release check | |

### Boundary Tests (numeric inputs)
N-A: the feature takes no numeric input.

### Functional Tests
- Tooling: the commit gate is reached through `Create`, the real entry point of `./le commit create`, in the commit package tests.

## Files to Modify
- `internal/le/commit/prepare.go` - call the gate
- `internal/le/register.go` - blank import of `internal/le/deps`
- `internal/le/verify/engine/stages.go` - `deps check` in `fullStages`
- `docs/contributing/committing.md` - the new refusal
- `docs/contributing/ze-go-style.md` - point the third-party rule at the gate

## Files to Create
- `dependency-ledger.txt` - the ledger, seeded
- `internal/le/deps/` - `deps.go`, `approve.go`, `check.go`, `actions.go`, `register.go`, tests
- `internal/le/commit/dependencies.go` and its test
- `docs/contributing/dependency-approval.md`

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/contributing/dependency-approval.md` written
- [ ] `docs/contributing/committing.md` names the refusal

## Implementation Steps
1. `internal/le/deps` with its tests.
2. The commit gate with its tests.
3. Seed the ledger from the tree, prove it with `./le deps check`.
4. Docs.

## Key Design Decisions
- The ledger is a root file beside `go.mod` and `feature-gates.txt`, one row per package and importer, tab separated: package, importer directory, reason. It is the only declaration of who may import what.
- Test files are judged like production files: a test-only dependency is still vendored, still supply chain, and still Thomas's to approve.
- Files under `vendor/`, `testdata/`, and any directory holding its own `go.mod` are not part of the Ze module and are not judged.
- The approved unit is the package path, not the module: approving `github.com/cloudflare/circl/sign/ed448` approves nothing else of circl.

## Known Limitations
- The ledger and the approval record are plain files an agent could write. An edit-hook refusal of both is Thomas's decision.

## Owned journal rows
| Row | Class |
|-----|-------|
| - | - |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le job run label unit-pkg command go test ./internal/le/deps/ ./internal/le/commit/ ./internal/le/verify/engine/`, then `./le cli grammar` and `./le test weakened selftest`; no `./le test unit` group covers `internal/le`
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence

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
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/delivery/spec-dependency-approval-ledger.md` only, in the same `./le commit create` script
