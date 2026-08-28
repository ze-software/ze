---
kind: directive
level: MUST
stage:
---
**A gate that refuses a COMMIT MUST derive its verdict from the paths that
commit names, never from the state of the working tree or of a file on disk. A
gate that reads the repository answers a question nobody asked: it refuses work
that is correct, for a fact about somebody else's work, and it does so in a
checkout several sessions share.**

The failure is not a false positive about the subject. It is a verdict about the
wrong subject, and the author it blocks usually has no permitted action: the
offending state is not theirs to commit, not theirs to delete, and not theirs to
carry.

Two readings of the same kind of ledger, measured on 2026-08-24, show the whole
distinction:

| Reader | Derives from | Effect |
|--------|--------------|--------|
| `check_weakened_tests` | recomputes the weakenings of the paths the commit NAMES | refuses only a commit that actually weakens a test |
| `rfc_changed_problems` | reads `test/rfc-changed.md` from DISK | one open row refuses every commit in the repository until its author lands it |

Both files are per-commit by their own contract. Only the second turns that
contract into a repository-wide lock. On the day it was measured it held a
227-path change hostage to a five-line assertion in another session's package,
and the blocked author's three available moves were all forbidden: adding a
foreign hunk, deleting an owner-approved row, or waiting.

**The same defect wears a second face: a gate that infers INTENT from a
by-product instead of reading the act.** `spec_audit_problems`
(`internal/le/commit`) asks whether a journal row names the claimed
spec, and treats that as the spec's closure. A row naming a spec is mandated for
every defect an agent finds, and an agent finds most of them inside its own
spec, so the ordinary mid-spec commit is read as a closure and refused for
lacking a closure section. The act itself is in the same function's arguments:
`remove_paths` already says whether the commit removes `plan/<spec>.md`, and a
commit that adds no learned summary and removes no spec closes nothing.

**Before adding or changing a commit gate, its doc comment MUST answer one
question: what does this refuse that the commit did not do?** A gate that cannot
answer it is reading the world in place of the diff.

Three consequences follow, and each MUST hold:

- **A shared per-commit ledger MUST be keyed to the commit that carries it.** A
  row about file A MUST NOT refuse a commit touching only file B.
- **An escape hatch MUST reach the gate that fires.** `--review-override`
  clears `review_gate_problems` and `spec_audit_problems` fires afterwards, so
  the documented escape does not reach the refusal an author actually meets.
  A gate with no reachable escape and no permitted action is a stop, not a gate.
- **A gate MUST NOT be satisfiable only by a statement its author knows to be
  false.** Where the routes past a refusal are gaming it (releasing a spec claim
  so the gate returns early) or asserting something untrue (filling a
  verification section for a spec that is not verified), the gate is asking the
  wrong question, and the author's correct move is to leave the work
  uncommitted and say so (`ai/rules/completion.md`).
