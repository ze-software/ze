---
kind: directive
level:
stage:
---
**Each round reviews less than the last, and the loop is required to end.**
Round 1 covers the whole diff. Round N+1 covers only round N's fixes. A gate
that cannot stop is a gate that gets bypassed. One place settles what happens
to a finding outside the round's scope, which classes are always in scope, and
where a homed finding goes: "Bounding the loop", below. Do not restate those
tests here. A second copy is how the corrected rule and the defective one
become one hop apart.
