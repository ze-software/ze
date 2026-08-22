---
kind: directive
level: MUST
stage:
---
**Verification debt MUST be cleared by RUNNING the gate: `make ze-verify-debt-clear`. You MUST NOT edit a row to `cleared`.** Every override on `commit_helper.py create` writes a row into `plan/verification-debt/<session>.md`, and `create --push` refuses while one is open (`ai/rules/completion.md`). The pass re-runs each DISTINCT gate the open rows name, once per pass whatever the row count, and writes `cleared` only on exit 0 (`clear_debt`, `scripts/dev/commit_helper.py`). A gate that exits non-zero leaves its rows open and prints its output. The pass runs whatever the named gates cost, `make ze-precommit-verify` included, so run it in the foreground and let it finish.

**A cleared row says the gate was green over the COMMIT.** Since 2026-08-22 every runnable gate runs inside ONE throwaway worktree at HEAD (`clear_debt`, `scripts/dev/commit_helper.py`). A pass no longer judges the uncommitted files several sessions keep in this checkout. Before that change a `cleared` meant only "green HERE". Such a pass CAN go red on work nobody in it owns, and green on work nobody in it wrote.

**When no worktree can be made, NOTHING clears and the pass exits 1.** You MUST NOT read that as a gate failure. The fallback it refuses is to run the gates against the working tree. That fallback is the defect the worktree removes. Taking it writes `cleared` into the artifact that exists to hold verification evidence (`ai/rules/evidence.md`).

**A pass whose every row names an unrunnable gate materializes no worktree.** A worktree is a full checkout. A pass with nothing to run MUST NOT pay for one.
