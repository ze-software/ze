---
kind: directive
level: MUST NOT
stage:
---
- **`go test`, lint analysis and a `ze-test` runner MUST NOT start raw from Bash.** The generic admission grammar is `./le job run label <label> [quiet] command <argv...>`: one job runs while peers queue, and the child exit status is preserved.
- **A run whose output you do not want in the transcript MUST take `quiet`, and MUST NOT be wrapped in a redirect to a scratch path, an exit-code echo and a `grep` over the log.** `quiet` writes the child's output to this session's scratch log and answers the exit code, that log's path and its failure lines. A hand-written wrapper picks its own lines to keep, and the job log it redirects past is removed when the job ends.
- **A one-off that MUST NOT queue states its reason in the command: `ZE_ADMIT_RAW="<reason>" <command>`.** An empty reason admits nothing, and the reason lands in the transcript, which is what makes the escape auditable by reading the session.
