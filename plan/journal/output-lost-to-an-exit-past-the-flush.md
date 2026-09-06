# Output is lost to an exit that skips the flush

A process replaces its own stderr with a pipe and drains that pipe on one path
only, the normal return from `main`. Anything that calls `os.Exit` in between
writes into a pipe nobody reads, so the diagnostic a person was meant to read
never reaches the terminal. The exit code still arrives, which is what makes it
hard to see: the command answers, and answers with nothing.

The tell is a library that exits for you -- `flag.ExitOnError`, a `log.Fatal`,
an `os.Exit` in a helper -- inside a process whose startup installed a stderr
capture.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-06 | ze-test-dns-stub | every `ze-test <mock>` subcommand's `--help`, through `crashlog.Init` (`internal/core/crashlog/crashlog.go`) and `flushCrashlog` (`cmd/ze/dispatch.go`) | `crashlog.Init` dup2s a pipe over fd 2 and replaces `os.Stderr` with it, and `crashlog.Flush` is the only drain. `main` (`cmd/ze/main.go`) calls it after `dispatchMain` returns. A mock server's flag set is built with `flag.ExitOnError`, so `--help` writes the usage to the pipe and then calls `os.Exit(0)` from inside the flag package, past the flush. MEASURED on the built binary: `./bin/ze-test dns --help`, `./bin/ze-test irr --help` and `./bin/ze-test rpki --help` each exit 0 and write 0 bytes to both streams; `./bin/ze-test dns --nosuch` exits 2 and writes 0 bytes, so the "flag provided but not defined" line is lost too. The serving path is unaffected, because it never exits: `./bin/ze-test dns --port 0` prints its readiness line | FIXED for `ze-test dns` only, which is where it blocked an acceptance criterion: its flag set is `flag.ContinueOnError` and `Run` returns a code instead of exiting, so the usage survives the flush (`TestHelpPrintsTheZone`, `internal/test/mock/dns/dns_test.go`). NOT fixed for `irr`, `cymru`, `rpki`, `peeringdb`, `radius-mock`, `rtr-mock` and `tacacs-mock`, and not fixed at the source. The source repair is a decision rather than an edit: either the process drains the pipe on every exit path, which means `crashlog` owning an exit function every caller uses, or no code reachable from `dispatchMain` calls `os.Exit` at all. Both touch the shipped `ze` daemon's crash capture, so neither belongs to a `ze-test` mock server |
