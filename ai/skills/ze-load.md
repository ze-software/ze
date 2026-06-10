# Load Rules

Force-read all project rules, patterns, and context into the current session.
Use at session start or when Claude is missing context after compaction.

## Steps

1. **Read all ai/rules/ files:**

```
ai/rules/anti-rationalization.md
ai/rules/api-contracts.md
ai/rules/architecture-summary.md
ai/rules/bash-output.md
ai/rules/before-writing-code.md
ai/rules/buffer-first.md
ai/rules/canonical-sources.md
ai/rules/cli-grammar.md
ai/rules/cli-patterns.md
ai/rules/compatibility.md
ai/rules/config-design.md
ai/rules/config-manipulation.md
ai/rules/data-flow-tracing.md
ai/rules/deferral-tracking.md
ai/rules/derive-not-hardcode.md
ai/rules/design-context.md
ai/rules/design-doc-references.md
ai/rules/design-principles.md
ai/rules/doctor-checks.md
ai/rules/documentation.md
ai/rules/enum-over-string.md
ai/rules/exact-or-reject.md
ai/rules/file-modularity.md
ai/rules/friction-reporting.md
ai/rules/functional-test-gate.md
ai/rules/git-safety.md
ai/rules/go-standards.md
ai/rules/goroutine-lifecycle.md
ai/rules/handoff.md
ai/rules/hook-mapping.md
ai/rules/impact-analysis.md
ai/rules/implementation-audit.md
ai/rules/integration-completeness.md
ai/rules/interop-and-goal-validation.md
ai/rules/json-format.md
ai/rules/memory-architecture.md
ai/rules/naming.md
ai/rules/never-destroy-work.md
ai/rules/no-asking.md
ai/rules/no-layering.md
ai/rules/no-partial-completion.md
ai/rules/no-sprintf-alloc.md
ai/rules/no-test-deletion.md
ai/rules/pipe-completeness.md
ai/rules/planning.md
ai/rules/plugin-design.md
ai/rules/quality.md
ai/rules/qemu-testing.md
ai/rules/rfc-compliance.md
ai/rules/rfc-reading.md
ai/rules/testing.md
ai/rules/tdd.md
ai/rules/wiring-completeness.md
ai/rules/ze-divergences.md
```

2. **Read all .claude/rules/ files:**

```
.claude/rules/hook-errors.md
.claude/rules/memory.md
.claude/rules/planning.md
.claude/rules/post-compaction.md
.claude/rules/session-start.md
```

3. **Read key context files:**

```
ai/INDEX.md
ai/LEARNED-INDEX.md
docs/architecture/core-design.md
plan/learned/DESIGN-HISTORY.md
plan/learned/RECURRING-PATTERNS.md
plan/learned/HOOK-FRICTION.md
```

4. **Read all patterns:**

```
ai/patterns/registration.md
ai/patterns/cli-command.md
ai/patterns/web-endpoint.md
ai/patterns/plugin.md
ai/patterns/config-option.md
ai/patterns/functional-test.md
```

5. **Confirm:** Report how many files were read and list any that were missing.

## Rules

- Read every file listed above. Do not skip any.
- If a file does not exist, note it as missing but continue.
- Do not summarize the content. Just read it silently and confirm completion.
- This skill exists because rules only work when they are in context.
