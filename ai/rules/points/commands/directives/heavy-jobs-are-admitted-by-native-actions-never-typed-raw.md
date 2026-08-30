---
kind: directive
level: MUST NOT
stage:
---
- **`go test`, lint analysis and a `ze-test` runner MUST NOT start raw from Bash.** The generic admission grammar is `./le job run label <label> command <argv...>`: one job runs while peers queue, and the child exit status is preserved.
- **A one-off that MUST NOT queue states its reason in the command: `ZE_ADMIT_RAW="<reason>" <command>`.** An empty reason admits nothing, and the reason lands in the transcript, which is what makes the escape auditable by reading the session.
