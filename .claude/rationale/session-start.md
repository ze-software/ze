# Session Start Rationale

Why: `.claude/rules/session-start.md`

## Why Each TOP 6 Rule Exists

- **Rules 1-2** (Read spec, know source) -- Prevent redesigning decisions already made in the spec and writing code that conflicts with existing patterns.
- **Rule 3** (No code without understanding) -- Prevents duplicate code. If you can't name 3 related files, you don't understand the codebase well enough to change it.
- **Rule 4** (TDD: test must FAIL first) -- Catches the case where a test passes immediately, proving it validates nothing.
- **Rule 5** (Preserve existing behavior) -- Prevents inventing new formats when existing ones work fine. Historical example: invented a new JSON format instead of reading `decode.go` and preserving the existing one.
- **Rule 6** (Confirm file paths) -- Prevents editing the wrong file, a common failure mode that wastes entire correction cycles.

## Verification Checks Per Rule

| Rule | Concrete Check |
|------|----------------|
| 1 | `cat tmp/session/selected-spec` -> read `plan/<name>` |
| 2 | Check file digests in per-spec session state; re-read full file only when digest insufficient |
| 3 | Can you name 3 related files? |
| 4 | `go test` shows RED before implementation |
| 5 | Document current output format BEFORE changing |
| 6 | Use Glob/Grep to verify target exists and is correct |
