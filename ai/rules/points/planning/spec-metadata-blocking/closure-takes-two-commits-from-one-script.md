---
kind: directive
level: MUST
stage:
---
**A spec that passes its Review Gate is NOT done until it is deleted from `plan/`, and the deletion MUST take TWO commits from ONE script: commit A carries the code, the journal row and the edited spec, commit B removes the spec.** The spec accumulates design notes, corrected assumptions and audit evidence during implementation, and deleting it before that is committed loses them from history forever.
**Every citation of the spec MUST be repointed inside commit A, and every deferral row naming it MUST be resolved there.** Closure deletes the destination those rows point at, and no gate reads deferral destinations. `docs/contributing/spec-workflow.md` carries the commit contents, the repoint procedure, and which shards a closure removes.
