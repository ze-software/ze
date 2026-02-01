# CLI Patterns

**BLOCKING:** All CLI commands MUST follow these patterns for consistency.

## Command Dispatch Structure

**File:** `cmd/ze/<domain>/main.go`

Every domain follows this dispatch pattern:

```go
func Run(args []string) int {
    if len(args) < 1 {
        usage()
        return 1
    }

    // Handle help first
    switch args[0] {
    case "help", "-h", "--help":
        usage()
        return 0
    }

    // Dispatch to subcommands
    switch args[0] {
    case "subcmd1":
        return cmdSubcmd1(args[1:])
    case "subcmd2":
        return cmdSubcmd2(args[1:])
    default:
        fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
        usage()
        return 1
    }
}
```

### Alternative: Map-Based Dispatch

For many subcommands, use a map:

```go
var subcommandHandlers = map[string]func([]string) int{
    "edit":    cmdEdit,
    "check":   cmdCheck,
    "migrate": cmdMigrate,
}

func Run(args []string) int {
    if handler, ok := subcommandHandlers[args[0]]; ok {
        return handler(args[1:])
    }
    // ... error handling
}
```

## Flag Definition Pattern (MANDATORY)

Each subcommand creates its own `flag.FlagSet`:

```go
func cmdExample(args []string) int {
    fs := flag.NewFlagSet("example", flag.ExitOnError)

    // Define flags - always use descriptive help text
    verbose := fs.Bool("v", false, "Enable verbose output")
    output := fs.String("o", "", "Output file path")
    count := fs.Uint("count", 1, "Number of iterations")
    jsonOutput := fs.Bool("json", false, "Output JSON instead of text")

    // Custom usage function
    fs.Usage = func() {
        fmt.Fprintf(os.Stderr, `Usage: ze example [options] <required-arg>

Description of what the command does.

Options:
`)
        fs.PrintDefaults()
        fmt.Fprintf(os.Stderr, `
Examples:
  ze example -v file.txt
  ze example --json input.conf
`)
    }

    // Parse flags
    if err := fs.Parse(args); err != nil {
        return 1
    }

    // Check required positional arguments
    if fs.NArg() < 1 {
        fmt.Fprintf(os.Stderr, "error: missing required argument\n")
        fs.Usage()
        return 1
    }

    filename := fs.Arg(0)
    // ... implementation
    return 0
}
```

## Standard Flag Conventions

### Common Short Flags

| Flag | Meaning | Type |
|------|---------|------|
| `-v` | Verbose output | bool |
| `-q` | Quiet mode | bool |
| `-n` | Dry run / count | bool/int |
| `-o` | Output file | string |
| `-f` | Address family / file | string |
| `-i` | Enable feature (e.g., ADD-PATH) | bool |
| `-a` | Local AS | int |
| `-z` | Peer AS | int |

### Common Long Flags

| Flag | Meaning | Type |
|------|---------|------|
| `--help`, `-h` | Show help | bool |
| `--json` | JSON output | bool |
| `--text` | Human-readable output | bool |
| `--dry-run` | Preview without executing | bool |
| `--no-header` | Exclude headers | bool |
| `--socket` | Unix socket path | string |
| `--log-level` | Logging verbosity | string |

### Repeatable Flags

For flags that can appear multiple times:

```go
type stringSlice []string

func (s *stringSlice) String() string {
    return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
    *s = append(*s, value)
    return nil
}

// Usage
var plugins stringSlice
fs.Var(&plugins, "plugin", "Plugin for decoding (repeatable)")
// ze decode --plugin a --plugin b --plugin c
```

## Exit Code Conventions

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error, validation failure, usage error |
| `2` | File not found, unreadable (config commands) |

## Error Output Pattern

Always write errors to stderr:

```go
// Missing argument
fmt.Fprintf(os.Stderr, "error: missing <argument>\n")

// Unknown command
fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])

// Operation failed
fmt.Fprintf(os.Stderr, "error: %v\n", err)

// JSON error response (when --json is used)
errJSON := map[string]any{
    "error":  err.Error(),
    "parsed": false,
}
data, _ := json.Marshal(errJSON)
fmt.Println(string(data))
```

## Stdin Handling Pattern

### TTY Detection

```go
func isTTY() bool {
    fi, err := os.Stdin.Stat()
    if err != nil {
        return false
    }
    return fi.Mode()&os.ModeCharDevice != 0
}
```

### Stdin Fallback

```go
// Check if stdin is a terminal (no pipe)
if isTTY() {
    fmt.Fprintf(os.Stderr, "error: missing argument or stdin\n")
    return 1
}

// Read from stdin
input, err := io.ReadAll(os.Stdin)
if err != nil {
    fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
    return 1
}
```

### Hex Input with `-` for Stdin

```go
hexValue := *nlriHex
if hexValue == "-" {
    hex, ok := readHexFromStdin()
    if !ok {
        fmt.Fprintf(os.Stderr, "error: no input on stdin\n")
        return 1
    }
    hexValue = hex
}
```

## Usage Text Structure

```go
fs.Usage = func() {
    fmt.Fprintf(os.Stderr, `Usage: ze <domain> <command> [options] <args>

Brief description of the command.

Options:
`)
    fs.PrintDefaults()
    fmt.Fprintf(os.Stderr, `
Examples:
  ze bgp decode --open FFFF...     # Decode OPEN message
  ze bgp decode --update FFFF...   # Decode UPDATE message
  ze config check config.conf      # Validate config file

Notes:
  Additional information if needed.
`)
}
```

## Output Modes

Commands often support multiple output formats:

```go
if *jsonOutput {
    data, _ := json.Marshal(result)
    fmt.Println(string(data))
} else {
    // Human-readable output
    fmt.Printf("Result: %s\n", result.Summary())
}
```

## Command Naming

| Pattern | Example | Use Case |
|---------|---------|----------|
| Verb | `validate`, `decode`, `encode` | Actions |
| Noun | `server`, `plugin`, `config` | Subdomains |
| Verb-Object | `check-config` | Avoid; use `config check` |

## Subcommand Hierarchy

```
ze <domain> <command> [subcommand] [options] [args]

ze bgp server config.conf          # domain=bgp, command=server
ze bgp decode --open FFFF...       # domain=bgp, command=decode
ze bgp plugin gr                   # domain=bgp, command=plugin, subcommand=gr
ze config check config.conf        # domain=config, command=check
```

## New Command Checklist

When adding a CLI command:

```
[ ] Create handler function: cmd<CommandName>(args []string) int
[ ] Use flag.NewFlagSet with descriptive name
[ ] Define flags with clear help text
[ ] Implement custom fs.Usage with examples
[ ] Handle --help/-h at parent level
[ ] Check required positional arguments
[ ] Write errors to stderr
[ ] Return appropriate exit codes (0, 1, 2)
[ ] Support --json for JSON output if applicable
[ ] Handle stdin with - convention if applicable
[ ] Register in parent dispatch switch/map
[ ] Add functional tests
```

## Anti-Patterns

| Avoid | Instead |
|-------|---------|
| Global flags | Flags per subcommand |
| `os.Exit()` in handlers | Return exit code |
| Printing help to stdout | Use stderr for usage |
| Silent failures | Always report errors |
| Magic defaults | Require explicit values |
