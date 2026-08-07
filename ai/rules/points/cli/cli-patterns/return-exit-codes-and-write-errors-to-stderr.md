---
kind: directive
level:
stage:
---
- Errors to stderr: `fmt.Fprintf(os.Stderr, "error: %v\n", err)`
- Return exit codes, never `os.Exit()` in handlers
- `-` means stdin (read) / stdout (write): read/write a user-supplied path through
  `internal/core/cliio` (`ReadFile`/`OpenReader`/`Create`/`WriteFile`), never a raw
  `os` call. `make ze-dash-stdio-check` fails any command that bypasses it. `--json` for JSON output
- Repeatable flags: `stringSlice` with `String()` + `Set()`
