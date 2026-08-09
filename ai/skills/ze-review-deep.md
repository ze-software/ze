---
name: ze-review-deep
description: Deep Review
---

# Deep Review

Multi-agent code review. Spawns parallel focused agents, each reviewing a different aspect of the code changes. Use before merge or commit of significant work.

See also: `/ze-review` (quick single-pass), `/ze-review-spec` (spec completeness), `/ze-review-docs` (documentation only)

## Delegation

`ai/rules/planning.md`: this skill runs in the MAIN THREAD and does its
own fan-out. Do not wrap the whole skill in a single agent. That buries the
parallel lenses one level down and costs exactly the independence they exist to
provide (`ai/rules/planning.md`).

Launch the agents this skill defines, all in ONE message, on `model: opus`.
**Start every agent prompt with `Serving /ze-review-deep:`.** The gate in
`.claude/hooks/pretool-agent-skill.py` blocks a raw agent when a skill covers
the ask, and these fan-out prompts ask for exactly that. The prefix says the
routing already happened.
Never trade their model down for cost; cut their NUMBER instead
(`ai/rules/planning.md`). You do not need to ask permission to spawn them
(`ai/INSTRUCTIONS.md`, STANDING REQUEST).

The user may optionally specify a scope and/or agent selection:
- `/ze-review-deep` -- all uncommitted changes, ask which agents to run
- `/ze-review-deep internal/plugin/` -- path scope
- `/ze-review-deep for security and logic only` -- run only named agents (skip selection prompt)
- `/ze-review-deep branch` -- current branch vs main

When the argument contains agent names (e.g., "security", "logic", "concurrency"), run only those agents without prompting. Otherwise, present the agent menu and wait for selection.

## Model Selection

Review is a review-phase workload, so it runs on the review model throughout
(`ai/rules/planning.md`: review on Opus 5). The orchestrator (this skill) runs
at the session's model.

| Model | Agents | Why |
|-------|--------|-----|
| **opus** | All agents (#1-#11) and the verification agent | Review is the judgment-heavy phase. A missed exploit path, race, or vacuous test costs more than the cheaper model saves, and a mechanical-looking lens (docs, project rules, test coverage) still needs judgment to tell a real gap from a false positive |

Do not downgrade an individual agent to `sonnet` or `haiku` because its lens
looks mechanical. If cost forces a reduction, cut the number of agents, never
the model they run on.

## Steps

### 1. Determine scope

Determine what code to review based on the argument:
- No arg: `git diff HEAD --name-only` for changed files
- Path: files under that path with changes
- `branch`: `git diff main...HEAD --name-only`

Read the diff to understand the full changeset. Build a file list.

Then run the deterministic test-relaxation audit and keep its output for the Test Coverage agent (and the final report):
- `python3 scripts/dev/audit-test-relaxation.py` (uncommitted), or `python3 scripts/dev/audit-test-relaxation.py origin/main` to also cover committed-but-unpushed work. Since this repo commits directly to main, `main` is normally the same commit as HEAD; the tool refuses that base (exit 2) instead of auditing an empty range and reporting clean. Use `main` only from an actual feature branch.
- It reports tests that were `[DELETED]`, `[WEAKENED]` (assertions removed, `t.Skip` added, `require`->`assert` downgrade, commented-out asserts, `ignore` build tag), or `[RELAXED]` (a documented `// test-relax:` token). This is read-only. Hand its output to Agent 4 and surface every finding in the report.

### 2. Select agents

Three lenses is this skill's floor on round 1. `ai/rules/planning.md`,
"State the review effort before you spend it", sets two as the universal floor
and three or more for an "audit this" ask. This skill is what that ask reaches.
Round 1 is the only pass that ever sees the whole diff, so its lens count is the
whole change's coverage. If the user names fewer, run those AND enough lenses of
your own choosing to reach three, in the same message. Then say which you added
and why. Spawning fewer plus a note that more are owed is still a round below
the floor, and the gate cannot be called clean from it.

If the user's argument names specific agents (keywords: security, concurrency, error, test, logic, data, api, rules, docs/documentation, performance/perf/alloc, completeness/feature), run those. Otherwise, present this menu and **wait for the user to choose**:

```
Which review agents should I run?

1. Security & Input Validation
2. Concurrency & Race Conditions
3. Error Handling & Edge Cases
4. Test Coverage Gaps
5. Logic & Correctness Bugs
6. Data Flow & Boundary Violations
7. API Compatibility & Contract Violations
8. Project Rules Compliance
9. Documentation Accuracy
10. Performance & Allocations
11. Feature Completeness (missing code)

Enter numbers (e.g., 1,5), "all", or names (e.g., "security, logic"):
```

**Do NOT launch agents before the user responds.** Only skip the prompt when the original `/ze-review-deep` argument already specifies which agents to run.

### 3. Launch selected agents

Launch the selected agents simultaneously using the Agent tool. Use `model: opus` for every agent (see Model Selection table). Each agent gets the file list, diff context, and the Agent Preamble above. Each agent MUST:
- Read the actual changed files (not just the diff)
- Apply its specific lens exhaustively
- Return findings in the structured format below

**IMPORTANT:** Launch all selected agents in a SINGLE message with parallel Agent tool calls.

**Fallback:** If an agent times out or fails, note it in the report as "[Agent] -- timed out, not reviewed" rather than blocking the entire review. Proceed with results from agents that completed.

---

**Agent 1 -- Security & Input Validation**
```
You are a security researcher performing a bug bounty review. Your payout depends on finding exploitable vulnerabilities.

{AGENT_PREAMBLE}

SCOPE: Review these changed files: {file_list}

For every user-controlled input (config values, CLI args, network data, JSON fields, socket data):
1. What characters does validation ACTUALLY accept? Read the code, not comments.
2. Is there a length limit? What happens at 1MB?
3. Does this string flow into shell commands, SQL, JSON formatting, log formatting, or file paths?
4. Can a malicious input cause unbounded allocation, CPU usage, or output size?
5. Can two different inputs produce the same internal representation? (confusion attacks)
6. For every make()/append() -- is the size derived from trusted or untrusted data?

Also check:
- Authentication/authorization bypass paths
- Sensitive data in logs or error messages
- Path traversal in any file operations
- Injection in any string interpolation

For each finding report:
FILE:LINE | SEVERITY (critical/high/medium/low) | CATEGORY | EVIDENCE (the specific input/path) | EXPLOIT scenario | FIX (specific code change)

If no issues found, say "No security issues found" with a brief explanation of what was checked.
```

**Agent 2 -- Concurrency & Race Conditions**
```
You are a concurrency expert looking for race conditions, deadlocks, and goroutine leaks.

{AGENT_PREAMBLE}

SCOPE: Review these changed files: {file_list}

For every goroutine, channel, mutex, and shared variable in the changed code:
1. What data is shared between goroutines? What synchronization protects it?
2. For every channel: where is the sender? Where is the receiver? Can the receiver exit before the sender sends?
3. For every mutex: what is the lock ordering? Can two goroutines acquire locks in different orders?
4. For every goroutine launched: what ensures it terminates? Is there a leak path?
5. For every type assertion: is comma-ok used? Could it panic?
6. For every select statement: is there a default case? Can it block forever?
7. For every goroutine in a loop: is the loop variable captured by reference?

Ze project rules: goroutines must be long-lived workers reading from channels, never per-event. Check compliance.

For each finding report:
FILE:LINE | SEVERITY (critical/high/medium/low) | CATEGORY | EVIDENCE (the specific race/deadlock scenario) | TRIGGER (how to reproduce) | FIX

If no issues found, say "No concurrency issues found" with a brief explanation of what was checked.
```

**Agent 3 -- Error Handling & Edge Cases**
```
You are an error handling auditor. Your job is to find every path where errors are lost, mishandled, or create unexpected behavior.

{AGENT_PREAMBLE}

SCOPE: Review these changed files: {file_list}

For every error return in the changed code:
1. Is the error checked by the caller? Trace up the call chain.
2. Is the error wrapped with context or silently swallowed?
3. Are resources cleaned up on the error path? (files, connections, locks, buffers)
4. Does the error message help debugging? Does it contain enough context?
5. Is there appropriate distinction between retryable and terminal errors?

For every function:
6. What happens with nil input? Zero-value input? Empty slice/map?
7. What happens at integer boundaries (0, -1, MaxInt, overflow)?
8. What happens with empty string? String with only whitespace? String with null bytes?

For every deferred Close():
9. Is the error return ignored on a writer? (data loss on flush failure)

For each finding report:
FILE:LINE | SEVERITY (critical/high/medium/low) | CATEGORY | EVIDENCE (the specific error path) | TRIGGER (input that causes it) | FIX

If no issues found, say "No error handling issues found" with a brief explanation of what was checked.
```

**Agent 4 -- Test Coverage Gaps**
```
You are a test coverage auditor. Your job is to find untested code paths, weak assertions, and missing edge case tests.

{AGENT_PREAMBLE}

SCOPE: Review these changed files: {file_list}

FIRST, the test-relaxation audit. Run `python3 scripts/dev/audit-test-relaxation.py` (use `origin/main` as the base to include committed-but-unpushed work; a base of `main` is refused with exit 2 when it is the same commit as HEAD) — read-only. For every entry it reports:
- `[DELETED]` or `[WEAKENED]`: a test was removed or neutered (assertions dropped, `t.Skip` added, `require`->`assert` downgrade, commented-out asserts, `ignore` build tag). Report as HIGH severity unless you can prove from the diff that the code was genuinely fixed and the test legitimately no longer applies. "The test was failing" is never a valid reason.
- `[RELAXED]`: a documented `// test-relax:` token. Report as MEDIUM and quote the reason; it is valid ONLY for a removed feature or replaced coverage.
Also watch for weakening the audit cannot see: an expected value changed in place to match new (possibly wrong) output. If a golden/expected literal changed, verify the new value is correct, not just convenient.

Then, for every changed function/method:
1. Does a test exist that exercises it? Name the test.
2. Does the test verify BEHAVIOR (correct output for given input) or just EXECUTION (no crash)?
3. Are error paths tested? What happens when dependencies fail?
4. Are edge cases tested? (empty input, boundary values, nil, concurrent access)
5. If the function has branches (if/switch/select), are all branches covered?

For every test in the changed code:
6. Are assertions specific enough? Would a wrong implementation also pass? (e.g., assert count==2 on a map that deduplicates -- wrong parsing could also give 2)
7. Are test inputs realistic? Do they represent actual production scenarios?
8. Is there a functional .ci test proving the feature works end-to-end? (Ze project requires this)

For each finding report:
FILE:LINE | SEVERITY (high/medium/low) | What is not tested | What test should be written (specific inputs and expected outputs)

If coverage is complete, say "Test coverage is adequate" with a brief summary of what's covered.
```

**Agent 5 -- Logic & Correctness Bugs**
```
You are a formal verification expert looking for logical errors in the code.

{AGENT_PREAMBLE}

SCOPE: Review these changed files: {file_list}

Read every changed function. For each one:
1. Does the code actually implement what the function name/comment says?
2. Are loop bounds correct? Off-by-one? Can the loop run zero times when it shouldn't?
3. Are comparison operators correct? (< vs <=, == vs !=)
4. Are boolean conditions correct? (AND vs OR, negation errors)
5. Is the return value correct in every path? Are there paths that return the wrong thing?
6. For switch/select: are all cases handled? Is there a missing default?
7. For map operations: is there a check for key existence before access?
8. For slice operations: are indices bounds-checked before access?
9. Does the code match its git history intent? (Use git blame/log to understand WHY old code existed -- flag if a guard or workaround is being removed)
10. Removed-behavior audit: for every line the diff DELETES or replaces, name the invariant or behavior it enforced. Search the new code for where that invariant is re-established. If you cannot find it, that is a finding: a removed guard, a dropped error path, a narrowed validation, a deleted test that covered a real case.
11. Test rewrite check: for every test file where the diff changes assertions (not just adds new ones), verify the OLD behavior is still tested. A test rewritten to cover a new issue while dropping the old assertion is a coverage regression even when the assertion count stays the same. Ask: "what did the old assertion prove, and where is that proof now?" Report as CRITICAL if coverage is lost. Rule: `ai/rules/testing.md` "Test Rewrite as Replacement."

Specifically check for:
- Inverted conditions
- Wrong variable used (copy-paste errors)
- Missing break/continue/return
- Integer truncation or overflow
- String comparison that should be case-insensitive (or vice versa)

For each finding report:
FILE:LINE | SEVERITY (critical/high/medium/low) | BUG DESCRIPTION | EVIDENCE (specific input that triggers wrong behavior) | EXPECTED vs ACTUAL | FIX

If no bugs found, say "No logic bugs found" with a brief explanation of what was checked.
```

**Agent 6 -- Data Flow & Boundary Violations**
```
You are a data flow analyst. Trace data from entry to exit and find where boundaries are violated.

{AGENT_PREAMBLE}

SCOPE: Review these changed files: {file_list}

For every data entry point in changed code (function parameters, config values, network input, file reads):
1. Trace the data through every transformation until it exits the system or is stored
2. At each boundary crossing (package boundary, goroutine boundary, serialization), verify the data contract is maintained
3. Check: is data validated at the RIGHT layer? (too early = revalidation, too late = use-before-check)
4. Check: can data be modified between validation and use? (TOCTOU)
5. For any type conversion or cast: can information be lost?

Ze-specific checks:
- Wire encoding: does data go through WriteTo(buf, off), not append/make? (buffer-first rule)
- Plugin boundary: JSON events over pipes -- is serialization/deserialization symmetric?
- Config pipeline: File -> Tree -> ResolveBGPTree -> map -> PeersFromTree -- is the chain preserved?
- PackContext: do capabilities affect encoding correctly?

For each finding report:
FILE:LINE | SEVERITY (critical/high/medium/low) | BOUNDARY | EVIDENCE (the specific data flow path) | VIOLATION (what goes wrong) | FIX

If no issues found, say "No data flow issues found" with a brief explanation of what was traced.
```

**Agent 7 -- API Compatibility & Contract Violations**
```
You are an API compatibility reviewer. Check that changes don't break callers, consumers, or documented contracts.

{AGENT_PREAMBLE}

SCOPE: Review these changed files: {file_list}

For every changed function signature, struct field, interface, or exported symbol:
1. Find ALL callers/consumers using grep/references. List them.
2. Does any caller pass arguments that no longer match?
3. Does any consumer read fields that were removed or renamed?
4. Does any interface implementation now miss a method?
5. For JSON output: did field names, types, or nesting change? What parses this JSON?
6. For config: did config keys change? What reads them?
7. For CLI: did flags, exit codes, or output format change?

Also check:
- Are deprecated features still working or silently broken?
- Do error messages that scripts might parse still match?
- Are YANG schemas updated if the data model changed?

For each finding report:
FILE:LINE | SEVERITY (critical/high/medium/low) | BREAKING CHANGE | WHO IS AFFECTED (list specific callers/files) | FIX

If no breaking changes, say "No API compatibility issues found" with a brief summary of what was checked.
```

**Agent 8 -- Project Rules Compliance**
```
You are a project standards auditor for the Ze BGP daemon. Check changed code against project rules.

{AGENT_PREAMBLE}

SCOPE: Review these changed files: {file_list}

Read the project's .claude/rules/ directory to understand all rules. Then check each changed file:

1. **performance.md**: Wire encoding uses WriteTo(buf, off), no append/make in encoding paths
2. **goroutine-lifecycle.md**: No per-event goroutines in hot paths
3. **architecture.md**: No identity wrappers, abstract when you can (2+ use cases), lazy over eager
4. **cli.md**: kebab-case JSON keys, correct envelope format
5. **go-standards.md**: ze- prefix conventions, correct YANG suffixes
6. **plugins.md**: Proximity principle, YANG required for RPCs, import rules
7. **cli.md**: flag.NewFlagSet, exit codes, stderr for errors, tab-completion (YANG command tree or plugin `CommandDecl` without `Hidden: true`)
8. **config.md**: Fail on unknown keys, no version numbers
9. **go-standards.md**: // Design: comment present in every .go file
10. **go-standards.md**: // Detail: / // Overview: / // Related: cross-references are bidirectional
11. **go-standards.md**: Files under 1000 lines, single concern per file
12. **rfc-compliance.md**: If the diff touches protocol code (wire, message, capability, FSM, NLRI, attributes), read the relevant `rfc/short/` summaries and verify: (a) every MUST/MUST NOT is enforced, (b) every MUST enforcement has a `// RFC NNNN Section X.Y: "quoted requirement"` comment, (c) no SHOULD is ignored without justification. A MUST violation is critical severity.
13. **Altitude check**: Is each change at the right depth? A special case layered on shared infrastructure is a sign the underlying mechanism should be generalized instead. Prefer deepening the shared abstraction over adding per-caller workarounds. Flag bandaid fixes with the deeper alternative named.

For each violation report:
FILE:LINE | RULE | VIOLATION | FIX

If all rules are followed, say "All project rules satisfied" with a brief summary.
```

**Agent 9 -- Documentation Accuracy**
```
You are a documentation accuracy auditor. Your job is to verify that documentation matches the changed code and that doc updates were made where required.

{AGENT_PREAMBLE}

SCOPE: Review these changed files: {file_list}

For every changed function, struct, config option, CLI flag, or RPC:
1. Does `docs/` contain documentation for this feature? Search for mentions.
2. If docs exist: do they match the current code? Check field names, syntax, behavior descriptions.
3. If the change modifies documented behavior: was the doc updated in this diff?
4. For every `<!-- source: path -- symbol -->` anchor in related docs: does the anchor still point to valid code?

Check these specific doc locations against changes:
- CLI changes -> `docs/guide/command-reference.md`
- Config changes -> `docs/guide/configuration.md`, `docs/architecture/config/syntax.md`
- Wire format changes -> `docs/architecture/wire/`
- Plugin changes -> `docs/guide/plugins.md`, `docs/plugin-development/`
- API/RPC changes -> `docs/architecture/api/commands.md`
- New features -> `docs/features.md`

Also check:
- `// Design:` comments in changed .go files: do they reference correct architecture docs?
- `// Related:` / `// Detail:` / `// Overview:` cross-references: are they bidirectional and accurate?
- Source anchors in docs that reference changed files: are the claims still correct?
- Code examples in docs that reference changed functions/types: are they still valid?

For each finding report:
FILE:LINE | SEVERITY (critical/high/medium/low) | CATEGORY (stale-doc/missing-doc/broken-anchor/wrong-example/missing-update) | EVIDENCE (what the doc says vs what the code does) | FIX

If documentation is accurate and complete, say "Documentation is accurate" with a brief summary of what was checked.
```

**Agent 10 -- Performance & Allocations**
```
You are a performance engineer reviewing Go code for a high-throughput daemon. Every allocation on a hot path adds GC pressure and latency.

{AGENT_PREAMBLE}

SCOPE: Review these changed files: {file_list}

The project has strict allocation rules. Read these files for context:
- ai/rules/performance.md (banned fmt patterns, textbuf.Buffer usage, hot path list)
- ai/rules/performance.md (buffer ownership, pool lifecycle, data lifecycle)
- ai/rules/performance.md (WriteTo pattern for wire encoding)

Identify hot paths from performance.md "Hot Path Rule" table. Any code in those
directories is per-message, high-frequency code.

For every changed function, check:

1. **fmt on hot path:** Any fmt.Sprintf, fmt.Fprintf, fmt.Errorf where a zero-alloc
   alternative exists? (textbuf.Buffer, errors.New, strconv.Append*, AppendTo)
2. **.String() on hot path:** Any .String() call whose result is concatenated or
   immediately discarded? Use AppendTo or textbuf.Buffer instead.
3. **Allocation in loop:** Any make(), append(), or string building per iteration
   when a buffer outside the loop would suffice?
4. **Heap escape via interface:** Passing a concrete value through any/interface{}
   on a hot path forces heap allocation. Check with: what is the receiver type?
5. **Heap escape via closure:** Closure capturing a local variable forces it to heap.
   On hot paths, prefer passing the variable as a parameter.
6. **Redundant computation:** Same derivation (string format, map lookup, type assertion)
   computed multiple times when it could be computed once and reused.
7. **Missing precomputation:** A value derived from configuration or negotiated state
   that does not change per-request but is recomputed on every call. These should be
   computed once at setup/config-load time, stored on the struct, and reused on the
   hot path. Examples: flag sets, capability masks, pre-formatted identifiers,
   serialized header templates.
8. **Algorithmic complexity:** O(n^2) patterns -- nested loops, linear scan inside a loop
   when a map or pre-sorted slice would work. Check: what is n at scale?
9. **Callee allocates what caller could provide:** Function does make([]byte, n)
   internally when the caller has a buffer in scope. Should use WriteTo(buf, off) pattern.
10. **Map with string key from known set:** map[string]V where a numeric/typed key
    would avoid per-lookup string hashing and GC scanning.
11. **string([]byte) for comparison:** Converting bytes to string just to compare.
    Compare bytes directly.
12. **Pool misuse:** sync.Pool Get without Put, or holding a pool buffer past its
    intended lifecycle (across goroutine boundaries without tracking).
13. **Unnecessary copy:** Copying data that could be referenced. Check against the
    project's "When Copies Happen" list in performance.md.

For each finding report:
FILE:LINE | SEVERITY (critical/high/medium/low) | CATEGORY (fmt-hot-path/heap-escape/loop-alloc/redundant-compute/missing-precompute/complexity/pool-misuse/unnecessary-copy) | EVIDENCE (the specific allocation or pattern) | IMPACT (estimated allocs/op or complexity) | FIX (specific code change using project patterns)

Cold paths (startup, config load, CLI, web rendering) are exempt unless the allocation
is unbounded. Focus severity on hot-path issues.

If no performance issues found, say "No performance issues found" with a brief explanation of what was checked.
```

**Agent 11 -- Feature Completeness (Missing Code)**
```
You are a feature completeness auditor. Your job is NOT to review code quality. Your job is to find what's MISSING -- code that should exist but was never written.

{AGENT_PREAMBLE}

SCOPE: Review these changed files: {file_list}

The other agents review the code that exists. You review what DOESN'T exist. This is a fundamentally different lens. A feature can have all its new code properly wired, tested, and correct -- and still be non-functional because essential connecting pieces were never written.

**Step 1: Identify the feature.**
From the diff, determine what feature is being added (new NLRI family, new plugin, new protocol support, new command, etc.). Read the spec if one exists (run `scripts/dev/spec-session.sh current`).

**Step 2: Enumerate every user story.**
List every operation a user would expect this feature to support:
- Can a user RECEIVE/DECODE this? (wire -> struct -> display)
- Can a user SEND/ENCODE this? (command/config -> struct -> wire)
- Can a user DISPLAY this? (`ze bgp decode`, `show` commands, web UI, JSON events)
- Can a user CONFIGURE this? (YANG config, CLI, bridge commands)
- Can a user FILTER this? (route filtering, policy)
- Can Ze FORWARD this? (receive from one peer, send to another)

For each user story, trace the full path from entry point through every component. Name every link. If any link has no implementation, report it.

**Step 3: Reference comparison.**
Find the most similar existing feature in the codebase. For NLRI families, compare against MUP (`internal/component/bgp/plugins/nlri/mup/`). For plugins, find one of the same shape. For bridge commands, compare against an existing bridge command family.

Read the reference feature's registration, handlers, and tests. Build a checklist of everything it has. Then check whether the new feature has each item. Report anything missing.

For NLRI families specifically, the reference checklist is:
| Component | Reference location | Purpose |
|-----------|--------------------|---------|
| Family registration | `types.go` MustRegister | AFI/SAFI known to the system |
| Splitter | `split.go` + register in nlrisplit | Extract individual NLRIs from wire |
| NLRI type with Parse/WriteTo | `types.go` | Struct representing the NLRI |
| registry.Registration{} | `register.go` | Hooks into the BGP engine |
| - RunEngine | Registration field | Engine can process this NLRI |
| - InProcessNLRIDecoder | Registration field | `ze bgp decode` works |
| - InProcessNLRIEncoder | Registration field | Engine can construct wire bytes |
| - InProcessRouteEncoder | Registration field | Route display/JSON works |
| Command handler | Plugin or bridge | User can announce/withdraw |
| Functional test (.ci) | test/decode/ or test/plugin/ | End-to-end proof |

**Step 4: Check the spec's Task vs ACs.**
If a spec exists, read the Task description (what the feature is supposed to do) and the Acceptance Criteria (what is tested). Report any operation promised by the Task that has no corresponding AC. A spec with narrow ACs can pass review-spec while the feature is incomplete.

For each finding report:
SEVERITY (critical/high/medium/low) | CATEGORY (missing-handler/missing-registration/missing-test/broken-chain/spec-gap) | WHAT IS MISSING | WHY IT MATTERS (what user operation fails) | REFERENCE (where the similar feature has it)

If the feature is complete, say "Feature is complete" with the reference comparison summary.
```

---

### 4. Collect and deduplicate

After all selected agents complete, collect their raw findings into a flat list. Deduplicate: when multiple agents report the same defect at the same location for the same reason, merge into one entry keeping the most specific description and the highest severity.

### 5. Verify findings

For each remaining finding, classify it as one of:

- **CONFIRMED:** The defect is provable from the code. A specific input, state, or sequence produces wrong behavior.
- **PLAUSIBLE:** The scenario is realistic but depends on runtime state. Keep these.
- **REFUTED:** Provably impossible from the code. Quote the guard, cite the type constraint, or show the invariant that prevents it. Or: factually wrong about what the code does.

**A finding in TEST-ONLY code that cannot reach the product is a Low/Note, whatever it would score in shipped code.** Helpers, fixture builders, `.ci` and `.et` scripts and the runners under `test/` ship in no binary. Such a finding keeps its severity when it leads to NO TESTING (the test never runs, the harness never reaches the code, the assertion is swallowed, the fixture builds the wrong scenario), when it changes what the test PROVES, or when it stops a gate refusing what that gate exists to refuse (`ai/rules/planning.md`, "A defect in test-only code is not a finding in the product").

**PLAUSIBLE is the default.** Do not refute a finding for being "speculative" or "depends on runtime state" when the state is realistic: concurrency races, nil on a rare-but-reachable path (error handler, cold cache, missing optional field), falsy-zero treated as missing, off-by-one on a boundary the code does not exclude, retry storms, partial failures, regex that lost an anchor. Only refute when you can construct the proof from the code itself.

Drop REFUTED findings. Keep CONFIRMED and PLAUSIBLE.

When under 20 findings remain after dedup, verify each one yourself by reading the relevant code. When 20 or more, spawn a verification agent (model: opus) with the diff, relevant files, and the candidate list.

### 6. Format report

Format the surviving findings into the report:

```
## Deep Review: [scope description]

**Files Reviewed:** [count] | **Agents:** N/N complete

### Critical & High Findings

| # | File:Line | Category | Severity | Finding | Fix |
|---|-----------|----------|----------|---------|-----|
(sorted by severity, then by file)

### Medium Findings

| # | File:Line | Category | Severity | Finding | Fix |
|---|-----------|----------|----------|---------|-----|

### Low Findings & Notes

| # | File:Line | Category | Finding |
|---|-----------|----------|---------|

### Coverage Summary

| Agent | Findings | Top Severity |
|-------|----------|-------------|
| Security | N | critical/high/medium/low/clean |
| Concurrency | N | ... |
| Error Handling | N | ... |
| Test Gaps | N | ... |
| Logic Bugs | N | ... |
| Data Flow | N | ... |
| API Compat | N | ... |
| Project Rules | N | ... |
| Documentation | N | ... |
| Performance | N | ... |
| Feature Completeness | N | ... |
| **Total** | **N** | **highest** |

### Verdict

- **BLOCK:** [count] critical/high issues must be fixed before merge
- **FIX:** [count] medium issues should be fixed
- **CONSIDER:** [count] low issues worth reviewing
```

## Review Integrity

**Verify claims against source code, not docs.** Each spawned agent must read
the actual implementation, not rely on documentation or comments. Treat docs
as potentially stale. Every capability claim in the report must cite a source
file and line.

**Fresh eyes, and a fresh LENS, on every pass. A fresh full diff only on the
first.** Round 1 examines the whole diff. This skill is the one an "audit this"
ask reaches, so it earns three lenses or more there. Later rounds change the
lens but shrink the scope. Round N+1 covers only round N's fixes and what
they touched. Re-reading the whole diff every round never stops, because a
diff of any size always yields something new, and the deep review is where that
bites hardest. A finding outside the round's scope is still fixed when the goal
depends on it, when you are unsure whether it does, or when it is one of the
eight always-in-scope classes: `ai/rules/planning.md`, "Bounding the
loop", governs, and it is the only place those tests are written.

## Rules

- Do NOT fix anything. Report findings only.
- Do NOT make any changes to code -- no Edit, Write, or Bash commands that modify files.
- After presenting the report, ask the user which findings to fix.
- **Regression test required per fix:** When fixing an issue found by this review, add a test that would have caught the problem during development. The issue exists because a test was missing; the fix is incomplete without one. If a regression test is genuinely impossible (e.g., the finding is a naming convention violation), note why in the fix. Otherwise, no test = not fixed.
- Each agent runs in the background -- launch all selected agents simultaneously.
- If an agent finds nothing, that's fine -- report "clean" for that category.
- If an agent times out, report "timed out" -- do not block the review.
- False positive filter: discard linter-catchable issues and intentional changes clearly visible in the diff. Discard a finding on unmodified lines ONLY when the goal does not depend on it, and it is not one of the eight always-in-scope classes (`ai/rules/planning.md`, "Bounding the loop"). Those classes are never NOTEs. An unqualified unmodified-lines filter deletes exactly what they are. An absence sits on no changed line, so it would discard this skill's own "what DOESN'T exist" lens in full.
