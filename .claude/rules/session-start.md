# Session Start Rules

**BLOCKING:** Complete these checks at the start of EVERY session before doing any work.

## TOP 6 RULES

| # | Rule | Why | Check |
|---|------|-----|-------|
| 1 | **Read selected spec FIRST** | Spec contains decisions already made | `cat .claude/selected-spec` → `docs/plan/<name>` |
| 2 | **Read source before writing code** | You will invent conflicting designs without seeing existing code | Read files you are about to modify |
| 3 | **No code without understanding** | Duplicate code, wrong patterns, broken integrations | Can you name 3 related files? |
| 4 | **TDD: Test must FAIL first** | Proves test actually validates something | `go test` shows RED before implementation |
| 5 | **Preserve existing behavior** | Breaking changes waste debugging time | Document current output format BEFORE changing |
| 6 | **Confirm file paths before editing** | Wrong-file edits waste correction cycles | Use Glob/Grep to verify target exists and is correct |

## Session Start Checklist

```
1. [ ] Read .claude/selected-spec
2. [ ] Read docs/plan/<spec-name> (if selected)
3. [ ] Read .claude/session-state.md (if exists)
4. [ ] Check git status for modified files
5. [ ] ONLY THEN start working
```

## Why These Rules Exist

These six rules prevent the most expensive mistakes:

- **Rules 1-2** prevent redesigning decisions already made in the spec and writing code that conflicts with existing patterns.
- **Rule 3** prevents duplicate code — if you can't name 3 related files, you don't understand the codebase well enough to change it.
- **Rule 4** (TDD) catches the case where a test passes immediately — proving it validates nothing.
- **Rule 5** prevents inventing new formats when existing ones work fine. Historical example: invented a new JSON format instead of reading `decode.go` and preserving the existing one.
- **Rule 6** prevents editing the wrong file — a common failure mode that wastes entire correction cycles.
