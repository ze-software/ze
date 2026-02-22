# CLI Patterns

**BLOCKING:** All CLI commands MUST follow these patterns.
Rationale: `.claude/rationale/cli-patterns.md`

## Dispatch

Each domain: `cmd/ze/<domain>/main.go` with `func Run(args []string) int`.
Handle `help`/`-h`/`--help` first, then dispatch.

## Flags

Each subcommand: own `flag.NewFlagSet` with custom `fs.Usage`. Parse flags, check required positional args, return exit codes.

### Short Flags

| Flag | Meaning | Flag | Meaning |
|------|---------|------|---------|
| `-v` | Verbose | `-q` | Quiet |
| `-o` | Output file | `-f` | Family/file |
| `-i` | Enable feature | `-a` | Local AS |
| `-z` | Peer AS | `-n` | Dry run/count |

### Long Flags

| Flag | Meaning | Flag | Meaning |
|------|---------|------|---------|
| `--json` | JSON output | `--text` | Human-readable |
| `--dry-run` | Preview | `--socket` | Unix socket path |
| `--log-level` | Logging level | `--no-header` | Exclude headers |

## Exit Codes

0 = success, 1 = general/validation/usage error, 2 = file not found/unreadable.

## Rules

- Errors to stderr: `fmt.Fprintf(os.Stderr, "error: %v\n", err)`
- Return exit codes, never `os.Exit()` in handlers
- `-` for stdin, `--json` for JSON output
- Repeatable flags: `stringSlice` with `String()` + `Set()`

## New Command Checklist

```
[ ] Handler: cmd<Name>(args []string) int
[ ] flag.NewFlagSet with fs.Usage including examples
[ ] Handle --help/-h at parent level
[ ] Check required positional args
[ ] Errors to stderr, proper exit codes
[ ] Register in parent dispatch
[ ] Functional tests
```
