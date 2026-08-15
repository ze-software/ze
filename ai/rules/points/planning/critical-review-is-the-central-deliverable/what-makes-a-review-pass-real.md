---
kind: directive
level: MUST NOT
stage:
---
1. **Independent, and independence is a property of the CONTEXT, not of the agent
   count.** The reviewer MUST NOT be the context that wrote the code. A fresh
   session satisfies that, and so does a phase agent spawned after the
   implementing phase ended. What is required is ≥2 distinct LENSES over the diff
   (logic/wiring/removed-behavior; security/edge-cases/test-quality; the
   feature's own risk area), each reading the PRODUCER rather than the caller and
   verifying claims against source (`ai/rules/evidence.md`). One independent
   context MAY run every lens itself.
   **`/ze-close` MUST run them itself and MUST NOT spawn (owner directive,
   2026-08-15).** Its closure agent already satisfies the independence condition,
   so a spawned reader adds a hop and a full startup cost without adding a lens.
   Spawn reviewers only from a main thread invoking `/ze-review` directly, where
   the alternative is reviewing inline in the context that authored the diff.
2. **Adversarial.** The question is "what can go wrong that nobody planned for?"
   Default findings PLAUSIBLE, not dismissed. MUST NOT discard wiring, removed-guard,
   logic, or vacuous-test findings.
3. **Verify the reviewers too.** A reviewer can be wrong. Before acting on a
   finding, reproduce it (an empirical check beats an argument: a `.ci` exit
   assertion that "SHOULD fire" either fires or does not; run it).
4. **Looped to zero over a SHRINKING scope.** Every fix is new code and earns a fresh pass. Each pass reviews less than the one before it. There is no cap on the NUMBER of passes, and a hard bound on what each one covers. See "Bounding the loop" below.
5. **Evidenced by an artifact, not narrated.** Record the pass with
   `scripts/dev/review_gate.py record` → `tmp/review/<spec-stem>-<session-id>.md`
   (session-scoped, so concurrent same-spec sessions never clobber each other). It pins the
   SHA-256 of every code/test file the reviewers examined. The spec's Review Gate
   section pastes the reviewers' actual findings and each fix.
