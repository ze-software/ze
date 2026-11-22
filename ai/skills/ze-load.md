---
name: ze-load
description: Force-read every project rule, pattern and context file into the session. Use when rules seem missing, such as after compaction. Costly in context.
---

# Load Rules

Force-read all project rules, patterns, and context into the current session.
Use at session start or when context is missing after compaction.

## Steps

1. **Read all `ai/rules/*.md` files (one read per file; skip generated `CORE.md` and `TRIGGERS.md`):**

```
ai/rules/architecture.md
ai/rules/cli.md
ai/rules/commands.md
ai/rules/completion.md
ai/rules/config.md
ai/rules/context-economy.md
ai/rules/documentation.md
ai/rules/evidence.md
ai/rules/git-safety.md
ai/rules/go-standards.md
ai/rules/goroutine-lifecycle.md
ai/rules/interop-and-goal-validation.md
ai/rules/never-destroy-work.md
ai/rules/no-layering.md
ai/rules/performance.md
ai/rules/planning.md
ai/rules/platform-linux.md
ai/rules/plugins.md
ai/rules/pre-release.md
ai/rules/precommit-verify.md
ai/rules/principles.md
ai/rules/protocol.md
ai/rules/quality.md
ai/rules/repo-maintenance.md
ai/rules/rfc-compliance.md
ai/rules/rule-format.md
ai/rules/rule-precedence.md
ai/rules/simplicity.md
ai/rules/spec-no-code.md
ai/rules/stale-comments.md
ai/rules/terminal-demos.md
ai/rules/testing.md
ai/rules/user-facing-errors.md
ai/rules/writing.md
```

2. **Read all `.claude/rules/` files (when running in Claude Code):**

```
.claude/rules/hook-errors.md
.claude/rules/memory.md
.claude/rules/planning.md
.claude/rules/post-compaction.md
.claude/rules/session-start.md
```

3. **Read key context files:**

```
ai/META-DECISIONS.md
ai/doors/system.md
ai/INDEX.md
docs/architecture/core-design.md
```

4. **Read all patterns:**

```
ai/patterns/bgp-family.md
ai/patterns/cli-command.md
ai/patterns/config-option.md
ai/patterns/debug-registration.md
ai/patterns/functional-test.md
ai/patterns/plugin.md
ai/patterns/registration.md
ai/patterns/web-endpoint.md
```

5. **Confirm:** Report how many files were read and list any that were missing.

## Rules

- Read every file listed above once. Do not skip any and do not re-read duplicates.
- If a file does not exist, note it as missing but continue.
- Do not summarize the content. Just read it silently and confirm completion.
- This skill exists because rules only work when they are in context.

## Done when

Done when the report states how many files were read and lists any that were missing.
