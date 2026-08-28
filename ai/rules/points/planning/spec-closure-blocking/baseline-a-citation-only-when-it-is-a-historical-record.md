---
kind: directive
level: MUST
stage:
---
**A spec-to-spec citation has three repairs, and the baseline is the last of them.**
Repoint the citation at the durable document that replaced the spec. Restate the
fact inline. Add the stem to `plan/.citation-baseline` when the citation is a
historical record of the closed spec. All three ride on commit A, because commit
B removes a spec and adds nothing. Editing `plan/.citation-baseline` to absorb
the dangling reference is banned at closure: `./le spec-citation` must pass
after the citation is repointed to a live source.
