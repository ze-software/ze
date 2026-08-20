---
kind: directive
level: MUST
stage:
---
**Verification debt MUST be cleared by RUNNING the gate: `make ze-verify-debt-clear`. You MUST NOT edit a row to `cleared`.** Every override on `commit_helper.py create` writes a row into `plan/verification-debt/<session>.md`, and `create --push` refuses while one is open (`ai/rules/completion.md`). The pass re-runs each DISTINCT gate the open rows name, once per pass whatever the row count, and writes `cleared` only on exit 0 (`clear_debt`, `scripts/dev/commit_helper.py`). A gate that exits non-zero leaves its rows open and prints its output. The pass runs whatever the named gates cost, `make ze-precommit-verify` included, so run it in the foreground and let it finish.

**A cleared row says the gate was green in THIS CHECKOUT, not that it is green over the commit alone.** Three of the five runnable gates are plain make targets over the working tree, which carries several sessions' uncommitted files (`DEBT_GATE_RUNNERS`, `scripts/dev/commit_helper.py`). Two answer about what git holds: `index_head_gate` materializes HEAD, and `ze-repository-tracked-build-check` compiles the tracked tree. You MUST read a `cleared` as that narrower claim. Clearing every row over HEAD is the stronger check and is separable work: each gate needs a HEAD-scoped spelling of the kind `discovery_index_head_status` already has.
