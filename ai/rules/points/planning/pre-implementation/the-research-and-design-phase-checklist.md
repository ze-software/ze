---
kind: fence
level:
stage:
---
```
── RESEARCH ── (read, search, understand -- no code)
   Gate: Name 3 related files + describe current behavior.

[ ] 1. Check existing spec: plan/spec-<task>.md
[ ] 2. Read ai/INDEX.md for doc navigation
[ ] 3. Scan plan/spec-*.md for related specs
[ ] 4. Match keywords → docs (INDEX.md tables)
[ ] 5. Read identified architecture docs
[ ] 6. RFC check: verify rfc/short/rfcNNNN.md exists; create if missing
[ ] 7. Read docs/contributing/rfc-implementation-guide.md (protocol work)
[ ] 7. Read ACTUAL source files -- document current behavior
      BLOCKING: cannot write spec without "what does existing code do?"
[ ] 7. Trace data flow (rules/data-flow-tracing.md)

── DESIGN ── (write spec, get approval)
[ ] 7. Document existing behavior (preserve unless user says change)
[ ] 7. TDD planning -- identify tests BEFORE implementation
[ ] 7. Present plan -- WAIT for approval
[ ] 7. Write spec using plan/TEMPLATE.md -- complete Pre-Spec Verification first

── IMPLEMENT ── (TDD cycle)
[ ] 14. Test fails → implement → test passes. Log mistakes immediately.

── SELF-REVIEW ── (adversarial, BEFORE presenting to user)
   Gate: Adversarial Self-Review (rules/quality.md) -- all 5 questions answered, fixes applied.
[ ] 14. Run adversarial self-review. Fix what it reveals. Do NOT present work yet.
[ ] 14. Check for unanswered questions from earlier in conversation. Re-state them.

── VERIFY ── (complete checklist, present evidence)
[ ] 14. Complete Completion Checklist -- all 12 steps, in order, no skipping.
[ ] 14. Present work with evidence. Do NOT suggest committing.
```
