---
kind: directive
level: MUST
stage:
---
- **Two weakenings pass the gate and MUST be judged by hand.** `writeWeakening` reads structure, so it sees neither an expected value changed in place (`Equal(t, 1, x)` to `Equal(t, 2, x)`) nor a rewrite that repoints an existing test at new behavior: the function count and the assertion count are unchanged, and the coverage loss is semantic.
- **A new behavior MUST get a NEW case, and an existing test MUST NOT be repurposed to carry it.** The behavior that test verified still needs proving.
