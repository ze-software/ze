---
kind: directive
level: MUST
stage:
---
**A spec that passes its Review Gate is NOT done until it is deleted from `plan/`, and the deletion MUST take TWO commits from ONE script: commit A carries the code, the journal row and the edited spec, commit B removes the spec.** The spec accumulates design notes, corrected assumptions and audit evidence during implementation, and deleting it before that is committed loses them from history forever.
**Every citation of the spec MUST be repointed inside commit A. A `Work Not Done` row in a LIVE spec that names it MUST be repointed there too.** Closure deletes the file those citations name, and no gate reads them. `docs/contributing/spec-workflow.md` carries the commit contents and the repoint procedure.
