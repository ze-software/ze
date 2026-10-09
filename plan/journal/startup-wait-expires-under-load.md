| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-09 | - | terminal-demo `startDaemon` against `ze start` in the render container | zefs-config validation failed twice at host load 72: daemon.log stayed EMPTY, not even `Starting ze`, until the `SSH server listening` wait expired. Same store started by hand in the container: ready in under 1s. Passed at load 26. Seen first 2026-10-08 | not fixed, cause unknown: whether ze blocks before its first write or is only starved. Next: SIGQUIT the daemon when the wait expires and keep the dump |
