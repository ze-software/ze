---
kind: note
level:
stage:
---
Ze is already de-facto US English (the large majority of files use the US forms).
A small amount of UK spelling has leaked into `docs/` over time. Do not run a blind
global find/replace: some occurrences are quoted RFC/BIRD text or proper nouns that
must stay verbatim. Fix drift opportunistically when you touch a file, matching the
surrounding US convention, and leave quoted external text untouched.
