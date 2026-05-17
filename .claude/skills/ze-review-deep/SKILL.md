# Deep Review

Multi-agent code review. Spawns parallel focused agents, each reviewing a different aspect of the code changes. Use before merge or commit of significant work.

See also: `/ze-review` (quick single-pass), `/ze-review-spec` (spec completeness), `/ze-review-docs` (documentation only)

The user may optionally specify a scope and/or agent selection:
- `/ze-review-deep` -- all uncommitted changes, ask which agents to run
- `/ze-review-deep internal/plugin/` -- path scope
- `/ze-review-deep for security and logic only` -- run only named agents (skip selection prompt)
- `/ze-review-deep branch` -- current branch vs main

When the argument contains agent names (e.g., "security", "logic", "concurrency"), run only those agents without prompting. Otherwise, present the agent menu and wait for selection.

## Model Selection

The orchestrator (this skill) runs at the session's model. Spawned agents use different models based on task complexity:

| Model | Agents | Why |
|-------|--------|-----|
| **sonnet** | Security (#1), Concurrency (#2), Logic (#5), Data Flow (#6) | Reasoning-heavy: exploit paths, race analysis, subtle bugs, cross-boundary tracing |
| **haiku** | Error Handling (#3), Test Coverage (#4), API Compat (#7), Project Rules (#8), Documentation (#9) | Mechanical: checklist matching, grep callers, compare docs to code |

## Steps

### 1. Determine scope

Determine what code to review based on the argument:
- No arg: `git diff HEAD --name-only` for changed files
- Path: files under that path with changes
- `branch`: `git diff main...HEAD --name-only`

Read the diff to understand the full changeset. Build a file list.

### 2. Select agents

If the user's argument names specific agents (keywords: security, concurrency, error, test, logic, data, api, rules, docs/documentation), run only those. Otherwise, present this menu and **wait for the user to choose**:

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

Enter numbers (e.g., 1,5), "all", or names (e.g., "security, logic"):
```

**Do NOT launch agents before the user responds.** Only skip the prompt when the original `/ze-review-deep` argument already specifies which agents to run.

### 3. Launch selected agents

Launch the selected agents simultaneously using the Agent tool. Use `model: sonnet` for agents 1, 2, 5, 6 and `model: haiku` for agents 3, 4, 7, 8, 9 (see Model Selection table). Each agent gets the file list, diff context, and the Agent Preamble above. Each agent MUST:
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

For every changed function/method:
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

1. **buffer-first.md**: Wire encoding uses WriteTo(buf, off), no append/make in encoding paths
2. **goroutine-lifecycle.md**: No per-event goroutines in hot paths
3. **design-principles.md**: No identity wrappers, no premature abstraction, lazy over eager
4. **json-format.md**: kebab-case JSON keys, correct envelope format
5. **naming.md**: ze- prefix conventions, correct YANG suffixes
6. **plugin-design.md**: Proximity principle, YANG required for RPCs, import rules
7. **cli-patterns.md**: flag.NewFlagSet, exit codes, stderr for errors
8. **config-design.md**: Fail on unknown keys, no version numbers
9. **design-doc-references.md**: // Design: comment present in every .go file
10. **related-refs.md**: // Detail: / // Overview: / // Related: cross-references are bidirectional
11. **file-modularity.md**: Files under 600 lines, single concern per file

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

---

### 4. Consolidate results

After all selected agents complete, consolidate their findings into a single report:

#### Report Format

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
| **Total** | **N** | **highest** |

### Verdict

- **BLOCK:** [count] critical/high issues must be fixed before merge
- **FIX:** [count] medium issues should be fixed
- **CONSIDER:** [count] low issues worth reviewing
```

### 5. Deduplicate

Multiple agents may find the same issue from different angles. Merge duplicates, keeping the most specific description and the highest severity.

## Rules

- Do NOT fix anything. Report findings only.
- Do NOT make any changes to code -- no Edit, Write, or Bash commands that modify files.
- After presenting the report, ask the user which findings to fix.
- Each agent runs in the background -- launch all selected agents simultaneously.
- If an agent finds nothing, that's fine -- report "clean" for that category.
- If an agent times out, report "timed out" -- do not block the review.
- False positive filter: discard findings on unmodified lines, linter-catchable issues, and intentional changes clearly visible in the diff.
