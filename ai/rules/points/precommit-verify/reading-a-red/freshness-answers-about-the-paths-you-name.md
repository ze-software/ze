---
kind: directive
level: MUST
stage:
---
**A verify verdict answers about the paths it was ASKED about, and `./le commit create` asks about the commit's own `--file` list. You MUST NOT read a FRESH as a verdict on the whole checkout.** `verificationState` (`internal/le/commit/verification.go`) passes that list to `./le verify status check <PATH>...`, and `CheckCertificate` (`internal/le/verify/engine/status.go`) compares only the named rows. An edit another session makes to a path your commit does not carry no longer makes your evidence STALE.
**A run that FAILED is STALE for every path list, so scoping MUST NOT be used as a route around a red run, and it is no route around a red structural gate either.** That is `structuralGateReds` (`internal/le/commit/verification.go`), which still reads every red the run recorded.
**`check` with no path arguments keeps its whole-tree meaning, and you MUST use that form when the question is about the tree rather than about one commit.** The other limits of the narrower question are `docs/architecture/testing/verify-freshness-scope.md`.
