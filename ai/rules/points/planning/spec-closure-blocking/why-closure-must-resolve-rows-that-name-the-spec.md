---
kind: note
level:
stage:
---
Why: closure DELETES the spec, and `deferral_unassigned_problems`
(`internal/le/commit`) checks that every live row's destination exists on
disk. A row left pointing at a closed spec can therefore never be satisfied: it
dangles forever, is reported on every future commit (as a WARNING: that gate is
advisory and does not block), and the next reader cannot tell whether the work was
done or silently lost. Closure must enforce both compatible rules:
"destination must exist" and "closure deletes the spec".
