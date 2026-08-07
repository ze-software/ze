---
kind: note
level:
stage:
---
`TestPrepareRealInstanceCarriesEveryModule` and
`TestPreparedModulesResolveIdenticallyToTracked` gate it against the real
eight-module instance, the latter by comparing `go list -m all` before and after
preparation. **A reappearance is a regression in whatever new path prepares an instance: find that path, do not just delete the directory.**
