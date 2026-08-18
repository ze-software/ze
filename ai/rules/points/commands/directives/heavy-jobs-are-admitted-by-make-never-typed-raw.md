---
kind: directive
level: MUST NOT
stage:
---
- **`go test`, `golangci-lint`, the `ze-test` runner, and a Python test file MUST NOT be started raw from Bash.** `check_raw_test_invocation` in `.claude/hooks/pretool-bash.py` refuses each one and names the `make` target that runs the same work.
- **Every heavy job MUST reach the machine through `make`, which routes it to `scripts/dev/ze-run.sh`: one job runs, the others queue behind it.** One machine carries several sessions, every heavy target is sized for the whole box, and a job typed raw arrives with nothing in front of it.
- **Work that no `make` target expresses MUST be queued by hand: `scripts/dev/ze-run.sh <label> <command>`.** The wrapper IS the queue, so the raw command inside it is admitted.
- **A one-off that MUST NOT queue states its reason in the command: `ZE_ADMIT_RAW="<reason>" <command>`.** An empty reason admits nothing, and the reason that is there lands in the transcript, which is what makes the escape auditable by reading the session.
- **A cheap subcommand of a heavy tool stays available: `golangci-lint config verify` runs no analysis and is not refused.**
