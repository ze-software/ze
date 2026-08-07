---
kind: note
level:
stage:
---
The recurring failure behind "Diagnosis Before Fix" is jumping from symptom to the
nearest edit that silences it: rename the command so it stops being rejected, skip
or relax the test so it stops failing, special-case the one input that breaks. That
fixes where the problem *shows up*, not where it *is*. The cure is to change the
success criterion from "symptom gone" to "root cause named and fixed at the owning
layer", and to produce the diagnosis BEFORE touching code.
