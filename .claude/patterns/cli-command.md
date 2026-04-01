# Pattern: CLI Command

Structural template for adding CLI commands to Ze.
Rules: `rules/cli-patterns.md`. Architecture: `docs/architecture/cli/plugin-modes.md`.

## Two Types of Commands

| Type | Location | When to use |
|------|----------|-------------|
| **Offline** | `cmd/ze/<domain>/` | Tools that don't need a running daemon (config, decode, validate, yang) |
| **Online** | `internal/component/cmd/<verb>/` | Commands that interact with the running daemon via RPC |

## Command Grammar

### Offline Commands

`ze <domain> <subcommand> [flags] [args]`

```
ze config set --dry-run config.conf bgp local-as 65000
ze bgp decode FFFF...
ze yang tree bgp
ze data ls
```

### Online Commands (daemon)

The grammar has several classes. The YANG tree defines the dispatch path.
The `peer <selector>` mechanism is handled specially by the dispatcher.

#### Peer Selector Mechanism

`peer` is a **selector keyword**, not a tree node in the grammar sense.
The dispatcher extracts `peer <value>` from the token stream (position-independent),
removes the selector value, and matches the remaining tokens against the YANG path.

```
show bgp peer 192.168.1.1
  tokens:  ["show", "bgp", "peer", "192.168.1.1"]
  extract: peer selector = "192.168.1.1" (removed from tokens)
  match:   "show bgp peer" -> YANG path -> handler(ctx.Peer="192.168.1.1")

set bgp peer 10.0.0.1 with as 65000
  tokens:  ["set", "bgp", "peer", "10.0.0.1", "with", "as", "65000"]
  extract: peer selector = "10.0.0.1" (removed)
  match:   "set bgp peer with" -> handler(ctx.Peer="10.0.0.1", args=["as","65000"])

peer * show bgp peer
  tokens:  ["peer", "*", "show", "bgp", "peer"]
  extract: peer selector = "*" (keyword + value both removed)
  match:   "show bgp peer" -> handler(ctx.Peer="*")
```

Valid selector formats: `*` (all), `192.168.1.1` (IPv4), `10.0.0.*` (glob),
`2001:db8::1` (IPv6), `10.0.0.1,10.0.0.2` (comma-separated), `as65001` (ASN),
or a named peer (validated against reactor peer list).

Commands with `RequiresSelector: true` reject invocation without an explicit selector.

#### Command Classes

| Class | Pattern | Examples |
|-------|---------|----------|
| **Simple query** | `VERB COMPONENT RESOURCE [ARGS]` | `show version`, `show env list`, `show data ls` |
| **Peer-scoped** | `VERB bgp peer [<sel>] [SUBACTION] [ARGS]` | `show bgp peer *`, `set bgp peer 10.0.0.1 with as 65000`, `del bgp peer upstream1` |
| **Named-resource** | `RESOURCE <id> ACTION [ARGS]` | `cache 123 forward *`, `commit tx1 start`, `commit tx1 withdraw route 10.0.0.0/24` |
| **Subscription** | `VERB [ARGS]` | `subscribe update`, `unsubscribe` |
| **Meta** | `RESOURCE ACTION [ARGS]` | `command list`, `help`, `plugin encoding` |

#### Full Command Inventory

**show (read-only):**
```
show version
show bgp peer <sel>              show bgp warnings
show bgp decode                  show bgp encode
show env list                    show env get <key>           show env registered
show schema list                 show schema methods          show schema events
show schema handlers             show schema protocol
show yang tree [module]          show yang completion          show yang doc
show data ls                     show data cat <key>          show data registered
show config dump                 show config diff             show config history
show config ls                   show config cat              show config fmt
show interface
```

**set/del/update (write):**
```
set bgp peer <sel> with <args>   set bgp peer <sel> save
del bgp peer <sel>
update bgp peer <sel> prefix <args>
```

**cache/commit (named-resource):**
```
cache list                       cache <id> retain            cache <id> release
cache <id> expire                cache <id> forward <sel>
commit list                      commit <name> start          commit <name> end
commit <name> eor                commit <name> rollback       commit <name> show
commit <name> withdraw route <prefix>
```

**meta/subscription:**
```
help                             command list                 command help <cmd>
command complete <prefix>        event list
plugin encoding                  plugin format                plugin ack
log levels                       log set <logger> <level>
metrics values                   metrics list
subscribe <type>                 unsubscribe
```

## Offline Command: File Structure

```
cmd/ze/<domain>/
  main.go          # Run() + dispatch + usage()
  cmd_<sub>.go     # One file per subcommand handler
```

### main.go Template

```go
package <domain>

func Run(args []string) int {
    if len(args) < 1 { usage(); return 1 }
    switch args[0] {
    case "help", "-h", "--help":
        usage(); return 0
    case "sub1":
        return cmdSub1(args[1:])
    // ...
    default:
        if s := suggest.Command(args[0], candidates); s != "" {
            fmt.Fprintf(os.Stderr, "hint: did you mean '%s'?\n", s)
        }
        usage(); return 1
    }
}
```

**Map-based dispatch** (>5 subcommands, e.g., `config/`, `data/`):

```go
var handlers = map[string]func([]string) int{
    "list": cmdList, "edit": cmdEdit, "show": cmdShow,
}
// In Run(): if h, ok := handlers[args[0]]; ok { return h(args[1:]) }
```

### cmd_<sub>.go Template

```go
func cmd<Name>(args []string) int {
    fs := flag.NewFlagSet("<domain> <sub>", flag.ExitOnError)
    dryRun := fs.Bool("dry-run", false, "preview changes")

    fs.Usage = func() {
        fmt.Fprintf(os.Stderr, "Usage: ze <domain> <sub> [options] <required-arg>\n\n")
        fmt.Fprintf(os.Stderr, "Options:\n")
        fs.PrintDefaults()
        fmt.Fprintf(os.Stderr, "\nExamples:\n  ze <domain> <sub> example.conf\n")
    }

    if err := fs.Parse(args); err != nil { return exitError }
    if fs.NArg() < 1 {
        fmt.Fprintf(os.Stderr, "error: requires <file>\n")
        fs.Usage()
        return exitError
    }

    // Implementation...
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        return exitError
    }
    return exitOK
}
```

## Online Command: File Structure

```
internal/component/cmd/<verb>/
  <verb>.go                    # init() -> pluginserver.RegisterRPCs()
  schema/ze-cli-<verb>-cmd.yang  # CLI tree definition
```

Handler implementation lives in `internal/component/bgp/plugins/cmd/<noun>/`.

### RPC Registration

```go
func init() {
    pluginserver.RegisterRPCs(
        pluginserver.RPCRegistration{
            WireMethod:       "ze-<verb>:<noun>-<action>",  // kebab-case
            Handler:          handler.HandleMyCommand,
            RequiresSelector: true,  // needs IP/glob selector
        },
    )
}
```

### YANG Tree Definition

```yang
container <verb> {
    config false;
    container bgp {
        config false;
        container peer {
            config false;
            ze:command "ze-<verb>:bgp-peer";
            description "Description for CLI help";
        }
    }
}
```

**Invariant:** Container nesting mirrors the CLI path.
`show bgp peer` = `container show > container bgp > container peer`.

### WireMethod Naming

Format: `ze-<verb>:<resource>-<action>` (kebab-case throughout).
The YANG path maps directly: `show bgp peer` = container nesting = WireMethod `ze-show:bgp-peer`.

| WireMethod | YANG path | Selector? |
|------------|-----------|-----------|
| `ze-show:bgp-peer` | `show bgp peer` | Yes (`RequiresSelector: true`) |
| `ze-show:bgp-warnings` | `show bgp warnings` | No |
| `ze-show:version` | `show version` | No |
| `ze-set:bgp-peer-with` | `set bgp peer with` | Yes |
| `ze-set:bgp-peer-save` | `set bgp peer save` | Yes |
| `ze-del:bgp-peer` | `del bgp peer` | Yes |
| `ze-show:env-list` | `show env list` | No |

## Conventions

| Rule | Detail |
|------|--------|
| Exit codes | `0` = success, `1` = general error, `2` = file not found |
| Exit constants | `const exitOK = 0; const exitError = 1` (or 2 for file I/O) |
| Errors | Always to stderr: `fmt.Fprintf(os.Stderr, "error: %v\n", err)` |
| No os.Exit() | Return exit code from handler. Never call `os.Exit()` in a handler |
| Suggest | Unknown subcommand: `suggest.Command(arg, candidates)` + hint to stderr |
| Help | Handle `help`, `-h`, `--help` at parent level BEFORE dispatch |
| Stdin | `-` means stdin. Use `os.Stdin` when filename is `-` |
| JSON output | `--json` flag. Default is human-readable text |

## Local Command Registration (daemon startup)

Some offline commands are also available inside the daemon. Registered at startup:

```go
cmdutil.RegisterLocalCommand("show version", func(_ []string) int {
    printVersion()
    return 0
})
cmdutil.RegisterLocalCommand("show bgp decode", func(args []string) int {
    return bgp.Run(append([]string{"decode"}, args...))
})
```

## Reference Implementations

| Variant | File | Notes |
|---------|------|-------|
| Switch dispatch | `cmd/ze/bgp/main.go` | Standard pattern |
| Map dispatch | `cmd/ze/config/main.go` | Many subcommands, storage-aware |
| Map dispatch (simple) | `cmd/ze/data/main.go` | Stateless subcommands |
| Registry dispatch | `cmd/ze/plugin/main.go` | Plugin CLI routing |
| Online RPC | `internal/component/cmd/show/show.go` | Read-only verb |
| Online RPC | `internal/component/cmd/set/set.go` | Write verb |

## Checklist

```
[ ] Handler: cmd<Name>(args []string) int
[ ] flag.NewFlagSet with fs.Usage including examples
[ ] Handle help/-h/--help at parent level
[ ] Check required positional args after fs.Parse()
[ ] Errors to stderr, proper exit codes (0/1/2)
[ ] Register in parent dispatch (switch/map/registry)
[ ] Unknown subcommand: suggest + usage + return 1
[ ] If online: YANG tree with ze:command extension
[ ] If online: WireMethod in kebab-case matching YANG
[ ] If online: RequiresSelector set correctly
[ ] Functional tests (test/parse/ for offline, test/plugin/ for online)
```
