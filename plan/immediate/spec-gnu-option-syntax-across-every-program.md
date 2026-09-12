# Spec: gnu-option-syntax-across-every-program

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner directive, 2026-09-12: every program this repository ships uses GNU option
syntax, and no value begins with a dash. Two dashes introduce one long option,
so `--help` is help. One dash introduces a cluster of short options, so `-help`
is `-h -e -l -p`, a help request followed by three options that do not exist.
`-help` is not a second spelling of `--help`.

The rule is now written at
`ai/rules/points/cli/cli-grammar-keywords-before-values/gnu-option-syntax-binds-every-ze-program.md`
and rendered into `ai/rules/cli.md`. Nothing enforces it, and the tree does not
obey it.

**Go's `flag` package accepts `-name` and `--name` identically for every flag it
declares.** So a `flag.FlagSet` obeys this rule nowhere by default. Measured
2026-09-12: 110 `flag.NewFlagSet` call sites in shipped code, none of them
sharing a parser.

| Area | Sites |
|------|-------|
| `internal/component/config/cli` | 16 |
| `internal/test/cli` | 15 |
| `internal/appliance` | 13 |
| `internal/component/iface/cli` | 7 |
| `internal/component/resolve/cli` | 4 |
| `internal/perf/cli`, `internal/le/cligrammar`, `internal/component/bgp/cli` | 3 each |
| `internal/test/fixture`, `internal/plugins/local` | 2 each |
| the remainder | 1 each |

Two operator-visible consequences follow, and the second is the one that costs.

**A long option answers to its short register.** `ze appliance build -verbose`
is accepted where the rule says that token is `-v -e -r -b -o -s -e`. An
operator who learns the single-dash form on one command carries it everywhere,
and the day a real short-option cluster is declared, their habit breaks.

**A dash-leading token is swallowed as a value.** Where a flag takes a value,
`--output -verbose` takes `-verbose` as the path. Nothing refuses it, because
nothing knows a value may not begin with a dash. This is the class that cost a
shared development machine about 943% CPU for twenty minutes on 2026-09-11:
`./le stress-repro run suite -help` read the cluster as the value of `suite` and
started a load generator. `plan/spec-le-publishes-its-command-surface.md` closes
that one instance inside `le`. Every other program still carries it.

## Work this spec inherits

`plan/spec-le-publishes-its-command-surface.md` applies the rule to `le`'s own
`leaction` grammar only: a dash-leading token is refused as a value, and a
trailing one asks the question rather than starting work. It does not touch any
`flag.FlagSet`, and `le` is one of nine programs.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/cli.md` - carries the new directive and the flag register it extends
  → Constraint: [fill during research]
- [ ] `docs/architecture/cli/command-namespacing.md` - states which register a token belongs to
  → Constraint: [fill during research]
- [ ] `docs/architecture/cli/root-namespace-grammar.md` - the R1-R9 ruleset and its seven feeders
  → Constraint: [fill during research]

**Key insights:** (minimal context to resume after compaction)
- `./le cli-grammar` already reports flag-register violations as F1 to F4 (`FlagRegisterHit`, `internal/le/cligrammar/report.go`), so the gate has a home and a report shape already.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/appliance/cmd_initrd.go` - one `flag.NewFlagSet` per command, no shared parser
- [ ] `internal/le/cligrammar/report.go` - the existing flag-register findings

**Behavior to preserve:**
- Every `--long` spelling an operator types today keeps working.

**Behavior to change:**
- `-long` stops being accepted as `--long`.
- A dash-leading token is refused where a value is expected.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `argv` reaching each program's `main`, then each `flag.FlagSet.Parse`.

### Transformation Path
1. [fill during research]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator ↔ program | argv | No |

### Integration Points
- [fill during research: the shared parser or wrapper every call site adopts]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No shipped command legitimately takes a dash-leading value | measured across `internal/le`: every dash-leading string is either a test proving refusal or argv for an external command | A command breaks when the refusal lands | repeat the measurement over `cmd/` and every `internal/` CLI package | unvalidated |
| A-2 | One shared parser can serve all 110 sites | they all construct a bare `flag.FlagSet` today | The change is 110 edits rather than one seam plus 110 adoptions | read a sample across the ten areas | unvalidated |
| A-3 | No programmatic caller passes `-long` to a ze binary | nothing measured yet | A script or test breaks silently | grep every `exec.Command` naming a ze binary, and every `.ci` fixture | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An internal caller already uses `-long` and starts failing | a test or `.ci` fixture going red | Validate A-3 before changing any parser |
| R-2 | Refusing a dash-leading value breaks a path that legitimately starts with a dash | a command refusing an operator's real input | A-1 measures it; `--` as an end-of-options marker is the GNU answer if one turns up |
| R-3 | 110 sites is wide enough that a partial landing leaves two grammars live | two commands disagreeing about `-long` | Land the seam and the gate together, so an unconverted site is visible rather than silent |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator's flag stops being accepted, or a value they meant is refused. `ze appliance` is on the image build path |
| How is it reverted? | A single commit revert per area if the seam lands first |
| Who else touches this path? | `plan/spec-le-publishes-its-command-surface.md` applies the same rule inside `le` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze appliance build -verbose` | → | the shared option parser | `TestASingleDashLongNameIsRefused` |
| a flag value given `-something` | → | the same parser's value check | `TestADashLeadingTokenIsNotAValue` |
| a new `flag.NewFlagSet` added anywhere | → | the `./le cli-grammar` feeder | `TestEveryFlagSetGoesThroughTheSharedParser` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `-long` where `--long` is declared, in any shipped program | Refused, naming the token and the two registers |
| AC-2 | `--long` | Accepted exactly as today |
| AC-3 | A dash-leading token where a value is expected | Refused by name, in every program |
| AC-4 | `-help` at any program | Treated as the malformed cluster it is, and no work starts |
| AC-5 | A `flag.NewFlagSet` constructed without the shared parser | `./le cli-grammar` names it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestASingleDashLongNameIsRefused` | [fill during design] | AC-1 | |
| `TestADashLeadingTokenIsNotAValue` | [fill during design] | AC-3 | |
| `TestEveryFlagSetGoesThroughTheSharedParser` | `internal/le/cligrammar/` | AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | this spec adds no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill during design] | `test/` | an operator types a single-dash long name and is told which register it belongs to | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | Scope is cli, no wire-visible change | N-A | N-A | |

## Files to Modify
- the 110 `flag.NewFlagSet` call sites, or the seam they adopt
- `internal/le/cligrammar/` - the feeder that keeps a new site from regressing
- `docs/architecture/cli/command-namespacing.md` - state the two registers

## Files to Create
- [fill during design: the shared option parser]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI commands/flags | Yes | every shipped program |
| CLI grammar (keyword before value) | Yes | `./le cli-grammar` is the gate |
| Pipe completeness | N-A | no answer shape changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/command-namespacing.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/immediate/spec-gnu-option-syntax-across-every-program.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the gate names every unconverted site
   - Tests: `TestEveryFlagSetGoesThroughTheSharedParser`
   - Files: `internal/le/cligrammar/`
   - Verify: the gate is red, naming all 110
2. **Phase: the seam** -- one parser that refuses a single-dash long name and a dash-leading value
3. **Phase: adoption** -- area by area, largest first, the gate shrinking each time
4. **Phase: the documentation states the two registers**

## Known Limitations
- [fill during design]

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
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
