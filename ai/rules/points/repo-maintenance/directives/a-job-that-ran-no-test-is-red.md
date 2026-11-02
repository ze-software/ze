---
kind: directive
level: MUST
stage:
---
**A CI job or a suite that ran zero tests MUST report red, whether or not it blocks a push.** "Advisory" says the job does not block a push. It never says that a job can die before its first test unseen: that job hides every defect in what it would have run, for as long as nobody reads it.
**The setup a CI job depends on (the binaries it builds, the restriction it lifts, the tool it installs) MUST be pinned by a test in `internal/le/workflowcheck`, and MUST be built through the native action that owns the list, never a hand-written step.** `TestQEMUNightlyBuildsEveryGuestDaemon` and `TestDockerPeersLiftsTheUsernsRestrictionBeforeItsTests` are the shape: each goes red when the step is missing from the workflow.
