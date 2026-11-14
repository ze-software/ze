---
kind: directive
level: MUST
stage:
---
**Every in-scope item a spec does not do MUST be homed in a spec, in the bucket that item belongs to (`plan/README.md`): an existing spec whose scope can be amended to take it, and a new spec only when none fits. The source spec's `Work Not Done` table MUST name that spec by path.** Reducing scope stays the owner's decision and MUST NOT be taken by the author. A destination that is prose is a deletion with a polite name. `plan/known-failures/` is never one, and no row holds an item in place of a spec. `docs/contributing/spec-workflow.md` carries the bucket test and the order in which a destination is chosen.
**A new spec MUST NOT be created, for a found problem, an unfinished item or a new request, until the existing specs have been searched for one whose scope can be amended to take the work (`ai/META-DECISIONS.md`).** Every spec is claimed, reviewed and closed on its own, so each one that an amendment would have avoided is cost with nothing bought. Where an existing spec changes the same mechanism, amend its Task section and acceptance criteria instead.
