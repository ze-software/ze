# Spec: fixit-agent-tooling-misleads

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Task

Six agent-facing surfaces tell agents something untrue, or demand something they cannot
do. Each was verified at the producer on 2026-07-16 and filed in
`plan/learned/HOOK-FRICTION.md` (F2, F3, F4, plus T-4 below found while writing this
spec). The friction is RECORDED; the fixes are owed, which is why this spec exists.

They share one cause: a gate or rule states a requirement its own implementation does not
honour, so an agent that obeys the text is punished by the tool, and an agent that obeys
the tool violates the text. Each is a separate task item. None is urgent. All six cost
real time on 2026-07-16.

T-5 and T-6 were each found by this spec's own tooling while writing or committing it, and
both are appended after the Checklist in discovery order. That is not tidy, and it is the
point: the count keeps growing because the surfaces are used, not audited. T-6 in
particular was found by the gate firing on the commit that filed the previous finding.

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

-> AUTONOMOUS DEFAULT (2026-07-17): give the RULE a subagent carve-out in
`.claude/rules/session-start.md`; do NOT change `block-until-lsp.sh`. Mechanism now
VERIFIED at the producer (was UNVERIFIED above): `.claude/hooks/block-until-lsp.sh:36`
lifts the gate whenever the ToolSearch query text matches `LSP` (`grep -qi "LSP"`),
regardless of whether a tool actually loaded; the comment at `:32-35` makes this
deliberate ("false-positive unblock is small; false-negative block is a stuck session").
So the gate ALREADY passes for a subagent that issues the query and gets "No matching
deferred tools found" — nothing is broken at the gate. The only fix owed is to stop the
rule's "banned excuses" table from telling subagents they must LOAD a tool that their
harness does not expose. Rationale: (1) a PreToolUse hook cannot observe ToolSearch
RESULTS, only the query, so it structurally cannot "tell a load from a query" without new
post-tool signalling — the larger, riskier option; (2) A-3/R-2 warn LSP absence is a
harness property that may change without notice, so a doc carve-out is reversible while a
gate rewrite bakes in today's behaviour; (3) the carve-out weakens no gate (the gate is
unchanged and still fires for main-session agents who skip the query). This DROPS
`.claude/hooks/block-until-lsp.sh` from Files to Modify and keeps
`.claude/rules/session-start.md`. Thomas: override if you want the gate itself to
distinguish a real load.

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

**Registration over hardcoding**: N/A for this spec — it adds no new command, view,
family, or handler that a core/shared package would need to discover. Every change edits
an existing gate (hook or dev script) in place. The one new piece of DATA (T-6's per-index
source map) belongs inside `scripts/dev/discovery_sources.py`, the module that already owns
the source-to-index relationship, so it is not a hardcoded switch bolted onto a core
package — it is the same module gaining the structure its own docstring already describes.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A spec citing a path with a line number is written | → | the validator's Current Behavior check | fixture: line-numbered citation ACCEPTED |
| A spec citing a shell path is written | → | the validator's Current Behavior check | fixture: shell path ACCEPTED |
| A spec about a Python file is written after reading it | → | the spec-write staleness check | fixture: Python read SATISFIES the gate |
| A structural gate refuses a commit | → | the commit gate's refusal text | `commit_helper_test.py`: the named command writes the record |
| T-5: a tooling spec names its own functional surface | → | `validate-spec.sh` Functional Tests check | fixture: daemon-code spec still requires a `.ci`; tooling spec accepted |
| T-6: a commit feeds one index while another is dirty | → | `commit_helper.py` per-index staleness gate | `commit_helper_test.py`: unrelated-dirty PASSES, genuine-stale still REFUSES |

-> Wiring opt-out: this is **cosmetic/internal** agent-tooling with **no user-facing**
daemon feature, so no row references a `.ci` (N/A here — a `.ci` drives the daemon, which
none of these gates touch; see the Functional Tests opt-out and T-5). Every row names a
concrete hook-fixture or `commit_helper_test.py` test that DRIVES the real gate.

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

-> AUTONOMOUS DEFAULT (2026-07-17): `.ci` opt-out — this spec has **no user-facing**
daemon feature; it is **cosmetic/internal** agent-tooling only. All six items (T-1..T-6)
change hooks (`validate-spec.sh`, `pretool-writeedit.py`, `block-until-lsp.sh`) and dev
scripts (`commit_helper.py`, `discovery_sources.py`) that the ze daemon never loads. A
`.ci` drives the ze daemon, so none of these can have one, exactly as T-5 argues. The real
functional surface is the hook fixture suite above (`scripts/dev/hook-fixture-check.py`),
which drives each real hook over a real spec — that is a DRIVING test, not a
reimplementation, so the requirement is honoured, not weakened. Rationale: fabricating a
`.ci` that cannot exercise anything would be the exact "test that passes for the wrong
reason" this spec is about. Thomas: override if you want a `.ci` added anyway (there is no
daemon path for one to touch).

-> Constraint: each new fixture MUST drive a structurally INVALID spec, so that a pass can
only mean no check ran. A fixture that passes for the wrong reason is the defect this
whole spec is about.

## Files to Modify

- `.claude/hooks/validate-spec.sh` - T-1: the Current Behavior regex and the 30-line window
- `.claude/hooks/pretool-writeedit.py` - T-4: the staleness check's evidence set
- `.claude/rules/session-start.md` - T-2: the LSP mandate, ~~only if the decision is a carve-out~~ → RESOLVED 2026-07-17: the decision IS a carve-out, so this file IS in scope. Add a subagent carve-out to the "banned excuses" table (a subagent whose harness returns "No matching deferred tools found" to `ToolSearch query="select:LSP"` has satisfied step 1)
- ~~`.claude/hooks/block-until-lsp.sh` - T-2: only if the decision is to make the gate tell a load from a query~~ → DROPPED 2026-07-17: the decision is the carve-out, not a gate rewrite. The gate already lifts on the query text by design (`:36`, comment `:32-35`) and needs no change; a PreToolUse hook cannot see ToolSearch results anyway. Do NOT modify this file
- `scripts/dev/commit_helper.py` - T-3: the refusal text; T-6: the index-staleness check, which must consult a per-index source map instead of demanding all three
- `scripts/dev/discovery_sources.py` - T-6: where the per-index source map must be built. It does NOT exist today: `OUTPUTS` (`:26-29`) is a flat tuple of the three index paths and `is_discovery_source` (`:35`) returns a bare bool, so the gate can only ask "is this a source of ANY index". The knowledge is present as prose in the module docstring (`:13-14`) and nowhere as data
- `.claude/hooks/validate-spec.sh` - T-5: scope the `.ci` requirement to specs that touch daemon code
- `scripts/dev/hook-fixture-check.py` - fixtures for T-1, T-4 and T-5
- `scripts/dev/commit_helper_test.py` - the T-3 case; T-6: a commit feeding one index while another is dirty must pass, and a commit feeding a genuinely stale index must still refuse
- `ai/rules/error-messages.md` - ~~only if the decision is that a remediation MUST be verified true~~ → RESOLVED 2026-07-17: IN scope. Add the principle that a remediation an error prints MUST be verifiably true (name a command that actually produces the promised effect). Rationale: this is the rule that owns error-message quality, the T-3 constraint flags the gap explicitly ("does not yet require the advice to be TRUE"), and generalising the fix (`ai/rules/discovery-updates.md`) prevents the next false remediation. Low-risk, reversible doc addition. Thomas: override to keep T-3 a one-off string fix in `commit_helper.py` if you prefer minimal scope

## Implementation Steps

1. Read each gate's intent before touching it. All six are correct in what they enforce.
2. T-1 and T-4 cost time daily; do them first.
3. Each fix needs a must-not-fire test proving the gate still rejects what it should.
4. T-6 needs BOTH directions tested. A per-index map that demands nothing is the fail-open
   this spec is about: the passing test must be paired with one proving a real staleness
   still refuses (AC-9).

## Acceptance Criteria

- AC-1: a spec citing a path with a line number passes the validator (T-1)
- AC-2: `.sh` and `Makefile` paths are citable in a spec's Current Behavior (T-1)
- AC-3: the `Behavior to preserve` check reads the whole section, not 30 lines (T-1)
- AC-4: the LSP rule and its gate agree with what a subagent can actually do (T-2)
- AC-5: the commit gate's remediation names a command that refreshes the record (T-3)
- AC-6: reading the `.py` or `.sh` a spec is ABOUT satisfies the spec-write gate (T-4)
- AC-7: no gate enforces LESS than it did before
- AC-8: a spec about agent tooling can name its real functional surface and pass, while a spec touching daemon code still must name a `.ci` (T-5)
- AC-9: the index-freshness gate demands only the indexes the commit's sources actually feed, and still refuses a genuinely stale one (T-6)

## Risks & Assumptions

| ID | Assumption | Basis | Validation |
|----|-----------|-------|------------|
| A-1 | The six are independent and can land separately | Different files, different gates | unvalidated |
| A-2 | T-1's regex can accept a line-numbered path without accepting prose | The form is narrow and anchored | unvalidated |
| A-3 | LSP's absence for subagents is a harness property, not a repo one | Observed 2026-07-16 | unvalidated: it may change without notice, so T-2 must not hard-code today's behaviour |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | A check is relaxed to make a warning go away | Every AC reads "accepts the correct form", never "warns less" |
| R-2 | T-2 is "fixed" by deleting the mandate | LSP is genuinely useful; the rule's intent is right and only its mechanism is wrong |
| R-3 | T-4 is "fixed" by widening the evidence to anything | The gate exists because inference-written specs cost ten false premises in one day. Widen to what a spec can be ABOUT, nothing more |
| R-4 | T-6 is "fixed" by making the index gate warn-only, or by widening `--stale-index-ok` | With no CI, this gate is the only enforcement of index freshness. AC-9 requires it to still refuse a genuinely stale index; per-index scoping is the fix, not a softer refusal |
| R-5 | The Files to Modify list stays scoped to T-1..T-4 and T-5/T-6 land untested | Both were found by tooling firing on this spec's own commits; whoever implements must add `scripts/dev/commit_helper_test.py` coverage for T-6 and a fixture for T-5 |

## Checklist

- [ ] T-1 validator accepts the mandated citation form, `.sh`/`Makefile` paths, and reads the whole section (AC-1, AC-2, AC-3)
- [ ] T-2 the LSP rule and its gate agree with subagent reality (AC-4)
- [ ] T-3 the commit gate's remediation is true (AC-5)
- [ ] T-4 the spec-write gate accepts the file types a spec can be about (AC-6)
- [ ] T-5 a tooling spec can name its real functional surface; daemon specs still need a `.ci` (AC-8)
- [ ] T-6 the index gate demands only the indexes the commit feeds (AC-9)
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

### T-6: the commit gate demands indexes the commit does not feed

Found on 2026-07-16 committing `plan/learned/1166-rfc-clause-map-needs-producers.md`. The
commit carried its own regenerated index (`ai/LEARNED-FULL-INDEX.md`, written by
`scripts/dev/learned_index.py`). The gate refused anyway, demanding
`ai/DOCS-TO-CODE.md` — an index the commit does not feed.

Verified at the producer: `ze-discovery-index` (`Makefile:424` -> the target at
`Makefile`) runs three independent generators — `package_map.py`, `docs_to_code.py`,
`learned_index.py`. They have disjoint inputs: `ai/DOCS-TO-CODE.md` is built from
`// Design:` headers in `.go` files (its own preamble states this), and carries no
`plan/learned/` dependency. Its dirty hunk at the time was one row for another session's
`internal/component/bgp/message/rfc7606_withdraw.go`. The learned summary appears nowhere
in it.

-> CITATION NOTE (2026-07-17): re-verified at the producer. The three-generator body is
real but lives at `mk/inventory.mk:122-125` (`ze-discovery-index:` runs `package_map.py`,
`docs_to_code.py`, `learned_index.py`), not at ~~`Makefile:424`~~ — `Makefile:424` is the
`ze-ai-check:` target; the top-level `Makefile` only references `ze-discovery-index` inside
`ze-regen` at `:427`. The gate code is `discovery_index_problems` /
`feeds_discovery_index` in `scripts/dev/commit_helper.py:545`, over
`discovery_sources.OUTPUTS` (`scripts/dev/discovery_sources.py:26-30`). The substance of
T-6 (three generators, disjoint inputs, gate demands all three) is confirmed.

The gate treats "this commit touches an index-feeding source" as "every index must be
fresh", rather than "the indexes THIS source feeds must be fresh". The two remediations it
leaves are both wrong:

| Option | Consequence |
|--------|-------------|
| Add `--file ai/DOCS-TO-CODE.md` as instructed | Cross-commits another session's index row. Exactly the failure `git-safety.md` documents and `84f9f2d1f` committed on 2026-07-16 |
| Pass `--stale-index-ok` | Correct here, but it is the same escape hatch for a real staleness, and nothing distinguishes the two |

Cost: with concurrent sessions in one working tree, some generated index is nearly always
dirty, so any commit adding a learned summary hits this. The rule's own text (workflow
step 4) tells agents to include a learned summary whenever a commit changes agent
workflow, rules, tooling, verification, or discovery — so the gate fires most reliably on
the commits the rules most want.

Root cause, verified: `scripts/dev/discovery_sources.py` cannot express the distinction.
`OUTPUTS` (`:26-29`) is a flat tuple of the three index paths, and `is_discovery_source`
(`:35`) answers a bare boolean — "is this a source of ANY index". Given only that, the
gate has no way to demand a subset, so it demands all three. The per-index mapping is
stated as prose in the module docstring (`:13-14`) and exists nowhere as data.

-> Constraint: do NOT fix this by weakening the gate to warn-only, and do NOT widen
`--stale-index-ok`. The gate is the only place index freshness is enforced (there is no
CI). Fix it by making the staleness check per-index: map each index to the sources it
actually reads, and demand only those. That map must be BUILT — the honest cost of T-6 is
turning the docstring's prose into data, then having the gate consult it.

-> Decision needed: whether an override that is right for one index and wrong for another
should stay a single flag. An `--stale-index-ok` that means "I checked, this index is
someone else's" is a different claim from "I know it is stale and am shipping anyway".
Habitual use of the second spelling for the first reason is how the override stops
meaning anything.

-> AUTONOMOUS DEFAULT (2026-07-17): keep ONE flag; do NOT add a second spelling. The
per-index fix (map each index to the sources it reads; demand only those) DELETES the
"someone else's index is dirty" case at the gate: once `discovery_index_problems`
(`scripts/dev/commit_helper.py:545` `feeds_discovery_index` -> `:613` the per-source
loop, over `discovery_sources.OUTPUTS`) demands only the indexes THIS commit feeds, an
unrelated dirty index no longer triggers a refusal at all, so no override is needed to
excuse it. That leaves `--stale-index-ok` with its single honest meaning: "I know this
index is genuinely stale and am shipping anyway." Rationale: this is the smaller,
self-contained option (no new flag surface); R-4 forbids widening/softening the override,
and a second flag would be a new place for the escape hatch to erode. AC-9 is satisfied by
the per-index scoping, not by flag semantics. Thomas: override if you later want the two
claims spelled separately.
