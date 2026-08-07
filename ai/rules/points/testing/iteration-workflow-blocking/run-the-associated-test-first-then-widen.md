---
kind: note
level:
stage:
---
If a changed file has an associated test file, feature test, or suite test, run
that first. After it passes, run the next broader relevant scope, then the
remaining gate. Order is: direct test -> file/feature test -> package ->
component group -> whole suite or `ze-verify`.
