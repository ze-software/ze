---
kind: directive
level: MUST NOT
stage:
---
**A scratch file MUST go under the project's gitignored `tmp/`, and the system
`/tmp` MUST NOT be used.** A subfolder per debugging task
(`tmp/watchdog-debug/`) isolates artifacts from each other but not from a sibling
session, so it goes under this session's own directory unless the artifact has to
outlive the session.
