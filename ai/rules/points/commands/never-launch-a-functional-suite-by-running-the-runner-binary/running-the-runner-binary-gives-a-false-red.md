---
kind: directive
level: MUST NOT
stage:
---
**A functional suite MUST NOT be launched by running a `ze-test` binary
directly. MUST use `./le functional <suite>`.** A raw runner rebuilds a daemon
without the test-only surface, so it produces a convincing false red. The
`--server` and `--client` hints the runner prints on failure repeat that same
launch and MUST NOT be followed either. The mechanism, and the table of how to
run one suite, one test, or a VM suite, is
`docs/contributing/running-commands.md`.
