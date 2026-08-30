---
kind: directive
level: MUST
stage:
---
**Verification debt MUST be cleared by RUNNING the gate: `./le commit debt-clear`. You MUST NOT edit a row to `cleared`.** Every override on `./le commit create` writes a row into `plan/verification-debt/<session>.md`, and `create push <remote>` refuses while one is open (`ai/rules/completion.md`). The pass re-runs each DISTINCT gate the open rows name, once per pass whatever the row count, and writes `cleared` only on exit 0 (`clearDebt`, `internal/le/commit/actions.go`). A gate that exits non-zero leaves its rows open and prints its output. The pass runs whatever the named gates cost, `./le verify worktree` included, so run it in the foreground and let it finish.

**A cleared row says the gate was green over the COMMIT**, because every runnable gate runs inside ONE throwaway worktree at HEAD. A pass does not judge the uncommitted files several sessions keep in this checkout.

**When no worktree can be made, NOTHING clears and the pass exits 1. You MUST NOT read that as a gate failure.** The fallback it refuses is to run the gates against the working tree, and taking it would write `cleared` into the artifact that exists to hold verification evidence (`ai/rules/evidence.md`).

**A row the pass leaves open MUST be answered by doing the work the row names, never by editing the row.** What the pass runs, what it refuses to run, and why an unrunnable row keeps the debt open are `docs/architecture/testing/verify-freshness-scope.md`.
