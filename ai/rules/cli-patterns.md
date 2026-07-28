# CLI Patterns

**When:** adding or changing a CLI subcommand, flag, or exit code
**Severity:** blocking

## Directives

All CLI commands MUST follow these patterns.
Rationale: `ai/rationale/cli-patterns.md`
Structural template: `ai/patterns/cli-command.md`

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

- **Grammar: closed keywords before free-form values** (`ai/rules/cli-grammar.md`, BLOCKING).
  First token after the noun must be a keyword. Member selection uses typed selectors (`name`, `id`, `index`, `address`, `type`, ...). IDs are strings.
- Errors to stderr: `fmt.Fprintf(os.Stderr, "error: %v\n", err)`
- Return exit codes, never `os.Exit()` in handlers
- `-` means stdin (read) / stdout (write): read/write a user-supplied path through
  `internal/core/cliio` (`ReadFile`/`OpenReader`/`Create`/`WriteFile`), never a raw
  `os` call. `make ze-dash-stdio-check` fails any command that bypasses it. `--json` for JSON output
- Repeatable flags: `stringSlice` with `String()` + `Set()`

## Command Completion (BLOCKING)

**Every user-facing command MUST have tab-completion.** No exceptions by default.

The completion tree is built from two sources:
1. **YANG command schemas** for built-in commands (via `BuildCommandTree`)
2. **Plugin command registry** for SDK plugin commands (via `CommandRegistry`)

Both feed the same completion tree. A plugin that registers a `CommandDecl` gets
completion automatically without writing a YANG file.

**Opt-out:** Set `Hidden: true` on a `CommandDecl` to suppress a command from
completion and help. The command still works when typed in full. Use this only
for internal/diagnostic commands that operators should not discover through
tab-completion. Hidden is the exception, not the default.

**Runtime vs offline tree:** the runtime completion tree DOES inject plugin
`CommandRegistry` entries after startup (`internal/component/cli/client/inject.go`
`injectPluginCommands`), so plugin commands complete in the live CLI. The static
offline tree (`BuildCommandTree`, used when no daemon is reachable, and
`ze help command`) still sees only YANG-backed commands; a plugin whose commands
must complete offline should ship a `-cmd` YANG module.

## New Command Checklist

```
[ ] Grammar: action keyword before identifier (ai/rules/cli-grammar.md)
[ ] Handler: cmd<Name>(args []string) int
[ ] flag.NewFlagSet with fs.Usage including examples
[ ] Handle --help/-h at parent level
[ ] Check required positional args
[ ] Errors to stderr, proper exit codes
[ ] Register in parent dispatch
[ ] Tab-completion works (verify with tab in CLI)
[ ] Colors follow semantic roles (docs/architecture/cli/color-system.md)
[ ] Functional tests
```
