---
name: ze-load
description: Load Rules
---

# Load Rules

Force-read all project rules, patterns, and context into the current session.
Use at session start or when Claude is missing context after compaction.

## Steps

1. **Read all ai/rules/ files:**

```
ai/rules/completion.md
ai/rules/go-standards.md
ai/rules/architecture.md
ai/rules/commands.md
ai/rules/architecture.md
ai/rules/performance.md
ai/rules/repo-maintenance.md
ai/rules/cli.md
ai/rules/cli.md
ai/rules/go-standards.md
ai/rules/config.md
ai/rules/config.md
ai/rules/architecture.md
ai/rules/planning.md
ai/rules/evidence.md
ai/rules/architecture.md
ai/rules/go-standards.md
ai/rules/architecture.md
ai/rules/repo-maintenance.md
ai/rules/writing.md
ai/rules/go-standards.md
ai/rules/protocol.md
ai/rules/go-standards.md
ai/rules/repo-maintenance.md
ai/rules/testing.md
ai/rules/git-safety.md
ai/rules/go-standards.md
ai/rules/goroutine-lifecycle.md
ai/rules/planning.md
ai/rules/repo-maintenance.md
ai/rules/architecture.md
ai/rules/completion.md
ai/rules/completion.md
ai/rules/interop-and-goal-validation.md
ai/rules/cli.md
ai/rules/performance.md
ai/rules/go-standards.md
ai/rules/never-destroy-work.md
ai/rules/completion.md
ai/rules/no-layering.md
ai/rules/completion.md
ai/rules/performance.md
ai/rules/testing.md
ai/rules/cli.md
ai/rules/planning.md
ai/rules/plugins.md
ai/rules/quality.md
ai/rules/platform-linux.md
ai/rules/rfc-compliance.md
ai/rules/protocol.md
ai/rules/testing.md
ai/rules/testing.md
ai/rules/completion.md
ai/rules/architecture.md
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
