# Spec: fixit-agent-tooling-misleads

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Task

Four agent-facing surfaces tell agents something untrue, or demand something they cannot
do. Each was verified at the producer on 2026-07-16 and filed in
`plan/learned/HOOK-FRICTION.md` (F2, F3, F4, plus T-4 below found while writing this
spec). The friction is RECORDED; the fixes are owed, which is why this spec exists.

They share one cause: a gate or rule states a requirement its own implementation does not
honour, so an agent that obeys the text is punished by the tool, and an agent that obeys
the tool violates the text. Each is a separate task item. None is urgent. All four cost
real time on 2026-07-16.

### T-1: the spec validator rejects the citation form the rules mandate

`ai/rules/no-fabrication.md` REQUIRES citing a producer as a path plus a line number. The
validator's Current Behavior check rejects exactly that: its regex requires the backticked
path to END in one of `go|py|rs|ts|js`, so a trailing line number after the extension
makes the match fail. `.sh` and `Makefile` are absent from the extension set entirely, so
a spec about shell code cannot cite the shell file it is about. The rule and its gate
demand opposite things, and the gate wins.

Same check, second half: it reads only the first 30 lines of the section, so a long
section's `Behavior to preserve` heading falls outside the window and warns. Several
specs written on 2026-07-16 carry that warning permanently, which trains readers to
ignore validator output. A warning everyone ignores is worse than no warning.

-> Constraint: the check exists so a spec's Current Behavior cites REAL producers. A fix
that widens the regex until it accepts prose is the `ai/rules/fail-closed-guards.md`
mistake in a new place. Accept the mandated form; do not accept everything.

### T-2: session-start mandates a tool subagents cannot load

`.claude/rules/session-start.md` makes an LSP `ToolSearch` the unconditional first action
and carries a table of "banned excuses" for skipping it. LSP is genuinely absent for
subagents: a filing agent's own first action on 2026-07-16 returned "No matching deferred
tools found".

-> Constraint: unsatisfiable in the CAPABILITY sense, not the GATE sense. The gate
reportedly lifts on the QUERY TEXT rather than on a successful load, so an agent that
issues the query passes while having no LSP. That mechanism is UNVERIFIED here and must be
read at the producer before anything changes. The rule is not blocking anyone; it is lying
to them, and its table blames them for noticing. Decide whether the rule gains a subagent
carve-out, or the gate learns to tell a load from a query.

### T-3: the commit gate's remediation does not work

`scripts/dev/commit_helper.py`'s structural-gate refusal tells the reader to re-run
`make ze-lint-changed` to refresh `tmp/ze-verify-failures.json`. That target does not
write the file; only a full `make ze-verify` does. Following the instruction leaves the
agent stuck with no signal.

-> Constraint: same class as `doctor-vpp-lcp-netns` telling operators to set a namespace
value that breaks LCP, fixed in `287aa411e`. A remediation that cannot work is worse than
none: the reader trusts it and loses the time twice. `ai/rules/error-messages.md` already
requires an error to say what to do next. It does not yet require the advice to be TRUE.

### T-4: the spec-write gate only accepts Go as evidence

`.claude/hooks/pretool-writeedit.py` blocks a spec write unless implementation was
investigated recently, and its own message states the rule: "Reading any .go under
internal/ pkg/ cmd/, or using the LSP tool, satisfies this."

So a spec about Python tooling or shell hooks CANNOT satisfy it by reading its own
subject. Writing this spec required reading an unrelated Go file purely to satisfy the
gate. Found while writing this spec: the gate blocked it twice, and reading
`scripts/dev/commit_helper.py` (the very producer T-3 describes) did not count.

-> Constraint: the gate's intent is RIGHT and its evidence set is too narrow. It exists
because specs were written from inference rather than from producing code, which is the
failure `ai/rules/no-fabrication.md` names and which cost this repo ten false spec
premises on 2026-07-16. Widen the evidence to the file types a spec can legitimately be
about (`.py` under `scripts/`, `.sh` under `.claude/hooks/`, `Makefile`, `mk/`), do not
weaken the requirement itself. A gate that forces a meaningless action to pass is training
agents to perform the ritual rather than the investigation.

## Required Reading

- `plan/learned/HOOK-FRICTION.md` (F2, F3, F4 carry the evidence and citations)
- `ai/rules/friction-reporting.md`, `ai/rules/fail-closed-guards.md`
- `ai/rules/no-fabrication.md` (T-1's mandated citation form; T-4's rationale)
- `ai/rules/error-messages.md` (T-3)

## Current Behavior

Source files read during investigation:

- [ ] `scripts/dev/commit_helper.py` (T-3: the refusal text, and which target writes the failure record)
- [ ] `scripts/dev/verify_wiring_docs.py` (T-3: what refreshing a gate's record actually requires)

Behavior to preserve: every one of these gates is CORRECT in what it enforces. The spec
validator SHOULD demand real citations; the LSP rule SHOULD push agents toward LSP; the
commit gate SHOULD refuse a structurally broken tree; the spec-write gate SHOULD demand
that a spec be grounded in producing code. Only the mechanics lie or over-narrow. A fix
that relaxes what any of them enforces has made things worse, not better.

The hook and shell sources are named inline in the Task section rather than listed here,
because the validator's own regex (T-1) rejects `.sh` paths in this list. The bug
demonstrates itself in the spec that describes it.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point

An agent writes a spec, cites a producer, or hits a refusing gate. Each of the four
surfaces is reached by an agent trying to do the RIGHT thing: cite a real file, load LSP
as instructed, follow a remediation, or ground a spec in producing code.

### Transformation Path

T-1: the agent writes a path plus a line number, as `no-fabrication.md` mandates. The
validator's Current Behavior regex requires the backticked path to END in a known
extension, so the trailing line number defeats the match, and the check errors. The agent
then writes a form the rules forbid, or drops the citation.

T-2: the subagent issues the mandated LSP query. No tool exists. The gate lifts on the
query text anyway, so the agent proceeds without LSP believing the rule was satisfied.

T-3: the agent hits a structural-gate refusal and follows its advice. The named target
does not rewrite the failure record, so the gate refuses identically and the agent has no
signal and no next step.

T-4: the agent reads the Python or shell its spec is ABOUT. The marker records only Go
under internal, pkg and cmd, so the spec write is blocked and the agent reads an unrelated
Go file to pass.

### Boundaries Crossed

| From | To | Shared point |
|------|----|--------------|
| the mandated citation form | the validator's regex | the spec's Current Behavior list |
| the LSP mandate | the subagent tool environment | the ToolSearch call |
| the commit gate's refusal text | the verify record writer | the failure record file |
| the spec-write gate's evidence set | the file types a spec can be about | the source-read marker |

### Integration Points

Every one is a rule and its enforcing mechanism disagreeing. The integration point IS the
disagreement: no code path is broken, the two artifacts simply state different things and
the agent is caught between them.

### Architectural Verification

None of the four needs an architectural change. Each is a narrowing or a stale string
inside an existing, correct gate. If a fix appears to need new architecture, the fix is
wrong: re-read the gate's intent first.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A spec citing a path with a line number is written | → | the validator's Current Behavior check | fixture: line-numbered citation ACCEPTED |
| A spec citing a shell path is written | → | the validator's Current Behavior check | fixture: shell path ACCEPTED |
| A spec about a Python file is written after reading it | → | the spec-write staleness check | fixture: Python read SATISFIES the gate |
| A structural gate refuses a commit | → | the commit gate's refusal text | `commit_helper_test.py`: the named command writes the record |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Agent cites a producer as the rules mandate | spec write, validator, accepted | fixture: line-numbered citation accepted |
| 2 | Agent specs a shell hook, citing that hook | spec write, validator, accepted | fixture: shell path accepted |
| 3 | Agent reads the Python it is specing, then writes | Read, marker, spec-write gate, allowed | fixture: Python read satisfies the gate |
| 4 | Agent follows a gate's remediation | refusal, named command, record refreshed, gate passes | `commit_helper_test.py` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| line-numbered citation accepted | `scripts/dev/hook-fixture-check.py` | AC-1 | |
| shell and Makefile paths accepted | `scripts/dev/hook-fixture-check.py` | AC-2 | |
| whole Current Behavior section read | `scripts/dev/hook-fixture-check.py` | AC-3 | |
| Python or shell read satisfies the spec-write gate | `scripts/dev/hook-fixture-check.py` | AC-6 | |
| the remediation names a record-writing command | `scripts/dev/commit_helper_test.py` | AC-5 | |
| prose is STILL rejected as a citation | `scripts/dev/hook-fixture-check.py` | AC-7, the must-not-fire case | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Current Behavior scan window | whole section | N/A: the fix removes the 30-line bound rather than tuning it | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the hook fixture suite | `scripts/dev/hook-fixture-check.py` | every fixture drives a real hook over a real spec | |

-> Constraint: each new fixture MUST drive a structurally INVALID spec, so that a pass can
only mean no check ran. A fixture that passes for the wrong reason is the defect this
whole spec is about.

## Files to Modify

- `.claude/hooks/validate-spec.sh` - T-1: the Current Behavior regex and the 30-line window
- `.claude/hooks/pretool-writeedit.py` - T-4: the staleness check's evidence set
- `.claude/rules/session-start.md` - T-2: the LSP mandate, only if the decision is a carve-out
- `.claude/hooks/block-until-lsp.sh` - T-2: only if the decision is to make the gate tell a load from a query
- `scripts/dev/commit_helper.py` - T-3: the refusal text
- `scripts/dev/hook-fixture-check.py` - fixtures for T-1 and T-4
- `scripts/dev/commit_helper_test.py` - the T-3 case
- `ai/rules/error-messages.md` - only if the decision is that a remediation MUST be verified true

## Implementation Steps

1. Read each gate's intent before touching it. All four are correct in what they enforce.
2. T-1 and T-4 cost time daily; do them first.
3. Each fix needs a must-not-fire test proving the gate still rejects what it should.

## Acceptance Criteria

- AC-1: a spec citing a path with a line number passes the validator (T-1)
- AC-2: `.sh` and `Makefile` paths are citable in a spec's Current Behavior (T-1)
- AC-3: the `Behavior to preserve` check reads the whole section, not 30 lines (T-1)
- AC-4: the LSP rule and its gate agree with what a subagent can actually do (T-2)
- AC-5: the commit gate's remediation names a command that refreshes the record (T-3)
- AC-6: reading the `.py` or `.sh` a spec is ABOUT satisfies the spec-write gate (T-4)
- AC-7: no gate enforces LESS than it did before

## Risks & Assumptions

| ID | Assumption | Basis | Validation |
|----|-----------|-------|------------|
| A-1 | The four are independent and can land separately | Different files, different gates | unvalidated |
| A-2 | T-1's regex can accept a line-numbered path without accepting prose | The form is narrow and anchored | unvalidated |
| A-3 | LSP's absence for subagents is a harness property, not a repo one | Observed 2026-07-16 | unvalidated: it may change without notice, so T-2 must not hard-code today's behaviour |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | A check is relaxed to make a warning go away | Every AC reads "accepts the correct form", never "warns less" |
| R-2 | T-2 is "fixed" by deleting the mandate | LSP is genuinely useful; the rule's intent is right and only its mechanism is wrong |
| R-3 | T-4 is "fixed" by widening the evidence to anything | The gate exists because inference-written specs cost ten false premises in one day. Widen to what a spec can be ABOUT, nothing more |

## Checklist

- [ ] T-1 validator accepts the mandated citation form, `.sh`/`Makefile` paths, and reads the whole section (AC-1, AC-2, AC-3)
- [ ] T-2 the LSP rule and its gate agree with subagent reality (AC-4)
- [ ] T-3 the commit gate's remediation is true (AC-5)
- [ ] T-4 the spec-write gate accepts the file types a spec can be about (AC-6)
- [ ] No gate enforces less than before (AC-7)
- [ ] Tests written per item
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Review Gate: `/ze-review` clean (0 BLOCKER, 0 ISSUE)

### T-5: the spec validator demands a `.ci` from specs that cannot have one

Found while writing THIS spec, which is why it is listed last. The validator rejects a
spec whose Functional Tests section names no `.ci` file: "A Go unit test is NOT a
substitute for a .ci functional test". That rule is correct for daemon features and
impossible for agent tooling. A `.ci` drives the ze daemon; hooks, `commit_helper.py` and
the spec validator itself never touch it. Their functional test IS the hook fixture suite
(`scripts/dev/hook-fixture-check.py`), which drives each real hook over a real spec.

This spec cannot pass its own validator without either naming a `.ci` that does not and
cannot exist, or being granted an exemption. It is the fourth instance of one assumption:
the tooling believes every spec is about Go code running in the daemon. T-1 rejects `.sh`
paths, T-4 accepts only Go as evidence, and T-5 demands a daemon test.

-> Constraint: do NOT fix this by dropping the `.ci` requirement. It exists because unit
tests alone let 30 tests ship that never bound a peer (`aaefef8ce`, 2026-07-16). Scope the
requirement to specs that touch daemon code, and let a tooling spec name its own real
functional surface instead. Whatever replaces it must still be a test that DRIVES the
thing, not one that reimplements it.
