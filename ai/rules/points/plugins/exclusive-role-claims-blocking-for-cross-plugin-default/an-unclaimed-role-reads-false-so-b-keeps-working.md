---
kind: directive
level:
stage:
---
**Fail closed:** an unclaimed or unresolvable role reads `false`, so B keeps doing
the work. Never invert that: standing down for an owner that never runs is worse
than both running. If a claimant never reaches Running, the engine logs it
(`verifyAdvertisedClaims`) but does not fail startup.
