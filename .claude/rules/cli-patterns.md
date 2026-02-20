# CLI Patterns

**BLOCKING:** All CLI commands MUST follow these patterns.
Rationale: `.claude/rationale/cli-patterns.md`

## Dispatch

Each domain: `cmd/ze/<domain>/main.go` with `func Run(args []string) int`.
Handle `help`/`-h`/`--help` first, then dispatch via switch or map.

## Flags

- Each subcommand: own `flag.NewFlagSet` with custom `fs.Usage`
- Parse flags, check required positional args, return exit codes

### Common Short Flags

| Flag | Meaning | Flag | Meaning |
|------|---------|------|---------|
| `-v` | Verbose | `-q` | Quiet |
| `-o` | Output file | `-f` | Family/file |
| `-i` | Enable feature | `-a` | Local AS |
| `-z` | Peer AS | `-n` | Dry run/count |

### Common Long Flags

| Flag | Meaning | Flag | Meaning |
|------|---------|------|---------|
| `--json` | JSON output | `--text` | Human-readable |
| `--dry-run` | Preview | `--socket` | Unix socket path |
| `--log-level` | Logging level | `--no-header` | Exclude headers |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error, validation failure, usage error |
| 2 | File not found, unreadable |

## Rules

- Errors to stderr: `fmt.Fprintf(os.Stderr, "error: %v\n", err)`
- Return exit codes, never `os.Exit()` in handlers
- Use `-` convention for stdin
- Support `--json` for JSON output where applicable
- Repeatable flags: implement `stringSlice` with `String()` + `Set()`

## New Command Checklist

```
[ ] Handler function: cmd<Name>(args []string) int
[ ] flag.NewFlagSet with fs.Usage including examples
[ ] Handle --help/-h at parent level
[ ] Check required positional args
[ ] Errors to stderr, proper exit codes
[ ] Register in parent dispatch
[ ] Add functional tests
```
