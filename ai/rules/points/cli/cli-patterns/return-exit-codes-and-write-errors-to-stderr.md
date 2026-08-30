---
kind: directive
level: MUST
stage:
---
- MUST send errors to stderr: `fmt.Fprintf(os.Stderr, "error: %v\n", err)`
- MUST return exit codes; MUST NOT call `os.Exit()` in handlers
- `-` means stdin (read) / stdout (write): MUST read/write a user-supplied path through
  `internal/core/cliio` (`ReadFile`/`OpenReader`/`Create`/`WriteFile`), MUST NOT make a raw
  `os` call. `./le dash-stdio check` fails any command that bypasses it
- Repeatable flags MUST use `stringSlice` with `String()` + `Set()`
