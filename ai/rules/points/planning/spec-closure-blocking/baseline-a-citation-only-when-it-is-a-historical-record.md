---
kind: directive
level: MUST
stage:
---
**A spec-to-spec citation has three repairs, and the baseline is the last of them.**
Repoint the citation at the durable document that replaced the spec. Restate the
fact inline. Add the stem to `plan/.citation-baseline` when the citation is a
historical record of the closed spec. All three ride on commit A, because commit
B removes a spec and adds nothing. A citation that still has a live source MUST be
repointed or restated; the baseline MUST NOT absorb it. `./le spec citation`
MUST pass after the repair.
