---
kind: directive
level: MUST
stage:
---
- **Every test, build, lint or verify command MUST run through the registered `./le` action that owns it.** A bare `go test` omits Ze's feature build tags, so plugins never register and unrelated packages fail with a phantom red; a `ze-test` runner launched directly rebuilds a daemon without its test-only surface and gives a convincing false red; a bare `golangci-lint` inherits host defaults and reports an environment failure as a code finding. The tag list, the suite table and the log paths are `docs/contributing/running-commands.md`.
- **A command started with `run_in_background` re-invokes the session when it exits, so that notification IS the wait: a polling loop MUST NOT be written.** No `while` or `until` around `sleep`, no `pgrep` in a loop condition, and no shell loop that forks one process per input where one invocation takes them all.
- **The harness's own Bash and Monitor examples show an unbounded `until ... sleep` loop, and this repository overrides them.** A poll that is genuinely the only signal MUST be wrapped in `timeout <seconds>`, MUST NOT wake faster than once every 30 seconds, and MUST be stopped the moment its reason changes. Each wake competes with QEMU, Docker and the verify gate for the same cores, so a watcher can corrupt the run it watches.
