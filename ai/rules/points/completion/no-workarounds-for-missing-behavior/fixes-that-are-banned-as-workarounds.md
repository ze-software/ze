---
kind: directive
level: MUST NOT
stage:
---
**These MUST NOT be written as a fix. Each is a workaround:**

| Banned | Why |
|--------|-----|
| Weakening or simplifying a test expectation | The test describes the required behavior. The broken code is what changes. |
| Special-casing only the failing fixture | Users can hit the same class of problem outside the fixture. |
| Skipping validation, errors, or unsupported inputs | Silent acceptance hides missing behavior and ships an operator trap. |
| Adding compatibility shims, aliases, or fallbacks instead of clean cutover | Ze has no released compatibility contract. Keep one real path. |
| Bypassing the owning layer from a caller | The next caller will fail the same way. Fix the owner. |
| Hiding a failure behind retries, sleeps, or broad catches | This masks the defect instead of proving the goal works. |
