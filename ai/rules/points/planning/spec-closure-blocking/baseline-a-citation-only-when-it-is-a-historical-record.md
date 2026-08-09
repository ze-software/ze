---
kind: directive
level:
stage:
---
**A spec-to-spec citation has three repairs, and the baseline is the last of them.**
Repoint the citation at the durable document that replaced the spec. Restate the
fact inline. Add the stem to `plan/.citation-baseline` when the citation is a
historical record of the closed spec. All three ride on commit A, because commit
B removes a spec and adds nothing. `spec-citation-check.py --write-baseline` is
banned at closure: it regenerates the whole list from the current tree, so it
grandfathers a citation that a repoint must fix.
