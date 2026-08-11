---
kind: directive
level: MUST
stage:
---
**Detection:** `/ze-review` step 0 (`audit-test-relaxation.py`) flags structural
changes. For semantic replacement, `/ze-review` step 7 (removed-behavior audit)
MUST verify that every assertion the diff replaces still has coverage elsewhere.
When reviewing a test edit that changes WHAT is asserted (not just adding new
assertions), ask: "is the old behavior still tested?"

**A `// test-relax:` token MUST be written for the ONE relaxation in hand, and
the stock MUST stay under the ceiling in `test/relax-ceiling.txt`
(`make ze-relax-census`).** The token is self-service: the agent that weakened
the test writes its own justification, so the only thing that ever made it safe
was a human reading it. 751 accumulated unread by 2026-08-10
(`TEST-RELAX-AUDIT.md`), and a token never expires -- three `.ci` tests carried
one whose claim had been refuted in-place four months earlier. Raising the
ceiling MUST happen in the same commit as the token that needs it.
