---
kind: directive
level:
stage:
---
1. **Independent.** Spawn ≥2 reviewer subagents over the diff, each a distinct
   lens (logic/wiring/removed-behavior; security/edge-cases/test-quality; the
   feature's own risk area). They read the PRODUCER, not the caller,
   and verify claims against source (`ai/rules/evidence.md`).
2. **Adversarial.** The question is "what can go wrong that nobody planned for?"
   Default findings PLAUSIBLE, not dismissed. Never discard wiring, removed-guard,
   logic, or vacuous-test findings.
3. **Verify the reviewers too.** A reviewer can be wrong. Before acting on a
   finding, reproduce it (an empirical check beats an argument: a `.ci` exit
   assertion that "should fire" either fires or does not; run it).
4. **Looped to zero over a SHRINKING scope.** Every fix is new code and earns a fresh pass. Each pass reviews less than the one before it. There is no cap on the NUMBER of passes, and a hard bound on what each one covers. See "Bounding the loop" below.
5. **Evidenced by an artifact, not narrated.** Record the pass with
   `scripts/dev/review_gate.py record` → `tmp/review/<spec-stem>-<session-id>.md`
   (session-scoped, so concurrent same-spec sessions never clobber each other). It pins the
   SHA-256 of every code/test file the reviewers examined. The spec's Review Gate
   section pastes the reviewers' actual findings and each fix.
