---
kind: directive
level: MUST NOT
stage:
---
**QEMU evidence is scheduled and advisory. You MUST NOT treat it as a blocking push gate, and you MUST NOT skip the focused QEMU proof for your change.** Which workflow runs which suite, and which of them block, is `docs/architecture/testing/ci-workflows.md`. Advisory means only that the job does not block a push: a nightly job that dies before its first test is broken and MUST be fixed (`ai/rules/repo-maintenance.md`).
