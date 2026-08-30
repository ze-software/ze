---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/git-safety.md
---
**The 2026-08-17 directive requiring a full `./le verify worktree` before every Go commit is SUPERSEDED and MUST NOT be followed.** Two later owner directives replace it: the gate gates PUSHING rather than committing (2026-08-21, `ai/rules/git-safety.md`), and ze is pre-release so a commit owes no green (2026-08-30, `ai/rules/pre-release.md`).
**A Go commit owes the FOCUSED test for what it changed, run once, and nothing more.** A missing full run records verification debt, which `./le commit create` writes and a push refuses to carry. You MUST NOT re-run the gate to watch somebody else's red clear, and you MUST NOT hold finished work waiting for one.
