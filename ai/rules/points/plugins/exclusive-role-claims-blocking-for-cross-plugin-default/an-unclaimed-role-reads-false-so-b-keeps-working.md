---
kind: directive
level: MUST NOT
stage:
---
**Fail closed:** an unclaimed or unresolvable role reads `false`, so B keeps doing
the work. That MUST NOT be inverted: standing down for an owner that never runs
is worse than both running. If a claimant never reaches Running, the engine logs it
(`verifyAdvertisedClaims`) but does not fail startup.
