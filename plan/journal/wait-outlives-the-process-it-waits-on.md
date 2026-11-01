# A wait outlives the process whose output it waits for

The waiter polls for a file or a line that one process writes, and it does not
watch that process. When the process exits without writing it, the wait runs to
its deadline anyway, so the exit is reported late, as a timeout, or not at all.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-01 | filter-wire-0-umbrella (walked into) | `waitReady` (`internal/test/runner/runner_exec_util.go`) | A foreground ze that exits at startup (exit 1, refused chain) never writes daemon.ready; waitReady polls for it for 5s times the parallel headroom and never looks at the process, so `filter-wire-name-pattern` and `filter-wire-external-fields-refused` each take 15s for a ze that exited in under 1s | open: end the wait when the foreground process exits (select on its exit beside the file poll), and prove it with a runner .ci whose ze exits at once |
