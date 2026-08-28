---
kind: directive
level: MUST NOT
stage:
---
- **`go test`, lint analysis, and a `ze-test` runner MUST NOT start raw from Bash.** The hook names the registered native action.
- **Use a registered `./le` action when one owns the work.** For otherwise raw work, the exact generic grammar is `./le job run label <label> command <argv...>`.
- **Admission preserves the child exit status.** One job runs and peers queue or attach; the command inside remains the command being judged.
- **A one-off that MUST NOT queue states its reason in the command: `ZE_ADMIT_RAW="<reason>" <command>`.** An empty reason admits nothing, and the reason that is there lands in the transcript, which is what makes the escape auditable by reading the session.
- **A cheap subcommand of a heavy tool stays available: `golangci-lint config verify` runs no analysis and is not refused.**
