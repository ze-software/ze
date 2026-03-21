# CLI Patterns Rationale

Why: `.claude/rules/cli-patterns.md`

## Why FlagSet Per Subcommand

Global flags create coupling. Each subcommand has its own flag namespace, preventing conflicts and making help text self-contained.

## Why Custom Usage Functions

Default `flag.PrintDefaults()` lacks context. Custom usage includes description, examples, and notes — making each command self-documenting.

## TTY Detection Pattern

```go
func isTTY() bool {
    fi, err := os.Stdin.Stat()
    if err != nil { return false }
    return fi.Mode()&os.ModeCharDevice != 0
}
```

## Stdin Handling

```go
if isTTY() {
    fmt.Fprintf(os.Stderr, "error: missing argument or stdin\n")
    return 1
}
input, err := io.ReadAll(os.Stdin)
```

## Usage Text Structure

```
Usage: ze <domain> <command> [options] <args>

Brief description.

Options:
  (flag.PrintDefaults())

Examples:
  ze bgp decode --open FFFF...
```

## Subcommand Hierarchy

`ze <domain> <command> [subcommand] [options] [args]`
Examples: `ze bgp server`, `ze bgp decode`, `ze config validate`, `ze plugin bgp-gr`

## Repeatable Flag Pattern

Implement `stringSlice` with `String()` and `Set(value)` methods, use `fs.Var()`.
