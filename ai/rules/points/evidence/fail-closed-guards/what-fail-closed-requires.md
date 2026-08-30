---
kind: directive
level: MUST
stage:
---
**A guard MUST satisfy all three:**

| Requirement | Meaning |
|-------------|---------|
| Fail closed | On a miss, an unmapped input, an empty set, or an error, DENY. Never fall through to the permissive branch |
| Or say something | A guard that genuinely cannot deny MUST log, error, or fail its gate. A guard that neither denies nor speaks does not exist |
| Make the miss explicit at the producer | Do not rely on a downstream layer to notice. The layer that knows it missed is the only one that can say so |
