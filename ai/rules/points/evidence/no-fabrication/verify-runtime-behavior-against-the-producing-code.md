---
kind: note
level:
stage:
---
A claim about what code *does* at runtime is not verified until you have read the
code that produces the behavior. Reading a value's consumer and assuming its
producer is inference, not evidence. This bites hardest when recommending work:
"there is a gap, we should fix it" is itself a claim, and its premise must be
traced to source *before* you propose the work, or you spec a non-problem.
