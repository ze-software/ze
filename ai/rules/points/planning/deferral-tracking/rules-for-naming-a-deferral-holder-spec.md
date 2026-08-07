---
kind: directive
level:
stage:
---
- One subtask per file. Two deferrals from the same source spec are two files, not one file with two tasks.
- The name carries the provenance: a reader knows what dropped it and why the file exists without opening it.
- For ad-hoc deferrals with no source spec, `<source>` is the subsystem (`plan/spec-l2tp-deferred-session-teardown-race.md`). <!-- doc-links: ignore (illustrative naming example, not a live spec) -->
- **A source spec does not outlive the deferral.** Spec closure `git rm`s the spec ("Spec Closure", above), so `<source>` will usually name a file that no longer exists by the time someone picks the work up. That is correct and intended: the provenance lives in git history, and the deferral spec is the tracker now. But when the source spec is ALREADY closed at the moment you write the deferral spec (homing an old row), name `<source>` for the subsystem instead: a filename pointing at a spec nobody can open reads as a broken reference rather than as provenance. Record the closed source spec in the `## Task` section either way.
- This naming applies only to deferral holders. Specs written as intended work keep the normal `spec-<task>.md` / `spec-<prefix>-<N>-<name>.md` names ("Spec Sets", above).
