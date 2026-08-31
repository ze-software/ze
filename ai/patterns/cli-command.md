# Pattern: CLI Command

Structural template for adding CLI commands to Ze.
Rules: `ai/rules/cli.md`. Architecture: `docs/architecture/cli/plugin-modes.md`.

**BLOCKING:** `ai/rules/cli.md` -- action keyword before identifier, IDs as strings.
Every command grammar must place the action keyword before any user-supplied identifier.
Read the grammar rule before designing any new command.

## Also Read

| Rule | When it applies |
|------|----------------|
| `ai/rules/cli.md` | Every command producing output MUST support all pipe operators. Its response payload MUST be structured data, never text a renderer already formatted, so `\| json`, `\| yaml` and `\| table` each render it |
| `ai/rules/cli.md` | If the command emits JSON: kebab-case keys and the envelope conventions |
| `ai/rules/evidence.md` | If the command lists or enumerates things (help, show, status) |
| `ai/rules/goroutine-lifecycle.md` | If the command launches background work (monitor, streaming) |
| Full navigation: `ai/INDEX.md` | |

## Two Types of Commands

| Type | Location | When to use |
|------|----------|-------------|
| **Offline** | the owner package, e.g. `internal/component/<owner>/cli/` (or `internal/plugins/<owner>/cli/`, `internal/core/<owner>/cli/`) | Tools that don't need a running daemon (config, decode, validate, yang). The command lives with the component that owns the behaviour, not under `cmd/ze`. |
| **Online** | `internal/component/cmd/<verb>/` | Commands that interact with the running daemon via RPC |

**Command ownership (`ai/rules/plugins.md`):** a command's offline
CLI, root registration, daemon RPC, schema, and doctor check live in the package
that owns the behaviour. `cmd/ze` is the process entry point only -- it consumes
registrations and keeps no-owner / process-global commands (see the no-owner
allowlist). Removing an owner package removes all its commands with no dangling
node elsewhere.


## Ownership Check Before Grammar Work

Before changing any command syntax, answer two questions from source:

1. **Who owns the operation?**
   - Config tree mutation -> engine `set` / `delete`
   - Runtime operational action -> RPC/CLI command
   - Read/query -> `show ...`
2. **Which YANG module owns the command family?**
   - Find the existing `ze:command`
   - Find the matching `RPCRegistration`
   - Edit that same module unless a broader architectural move is explicitly intended

Do not do grammar cleanup before this ownership pass.
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
Selector values should usually be introduced by selector keywords such as
`name`, `id`, `index`, `address`, or `type`.

**Peer commands are the explicit exception.** Their public syntax keeps the
peer selector immediately after `peer`:

- `show bgp peer <name|address> detail`
- `show bgp peer <name|address> rib`

Do not write mutating peer examples in this pattern unless the exact grammar
was explicitly agreed in source or by the user.
Do not expose a user-facing `selector` keyword for peer commands.

#### Peer Selector Mechanism (current implementation, provisional)

The current dispatcher special-cases `peer <value>`.

`peer` is a selector slot in the public grammar, and the current
implementation extracts that raw selector value from the token stream,
removes it, and matches the remaining tokens against the YANG path.

```
show bgp peer 192.168.1.1
  tokens:  ["show", "bgp", "peer", "192.168.1.1"]
  extract: peer selector = "192.168.1.1" (removed from tokens)
  match:   "show bgp peer" -> YANG path -> handler(ctx.Peer="192.168.1.1")

show bgp peer 10.0.0.1 rib
  tokens:  ["show", "bgp", "peer", "10.0.0.1", "rib"]
  extract: peer selector = "10.0.0.1" (removed)
  match:   "show bgp peer rib" -> handler(ctx.Peer="10.0.0.1")

show bgp peer edge1 detail
  tokens:  ["show", "bgp", "peer", "edge1", "detail"]
  extract: peer selector = "edge1" (removed)
  match:   "show bgp peer detail" -> handler(ctx.Peer="edge1")
```

This is an implementation detail of the current codepath, not a license to
introduce `selector` into the public grammar.

Current selector values accepted by the dispatcher: `*` (all), `192.168.1.1`
(IPv4), `10.0.0.*` (glob), `2001:db8::1` (IPv6), `10.0.0.1,10.0.0.2`
(comma-separated), `as65001` (ASN), or a named peer (validated against the
reactor peer list).

Commands with `RequiresSelector: true` reject invocation without an explicit selector.

#### Command Classes

| Class | Pattern | Examples |
|-------|---------|----------|
| **Simple query** | `VERB COMPONENT RESOURCE [ARGS]` | `show version`, `show env list`, `show data ls` |
| **Typed selector** | `VERB COMPONENT RESOURCE SELECTOR-KIND <value> [VIEW] [ARGS]` | `show interface type dummy`, `show interface name eth0 detail`, `show sysctl key net.ipv4.ip_forward` |
| **Peer-scoped** | `VERB bgp peer <name|address> <view> [ARGS]` | `show bgp peer 192.0.2.1 detail`, `show bgp peer edge1 rib` |
| **Named-resource** | `RESOURCE ACTION <id> [ARGS]` | `cache forward 123 *`, `commit start tx1`, `commit withdraw tx1 route 10.0.0.0/24` |
| **Subscription** | `VERB [ARGS]` | `subscribe update`, `unsubscribe` |
| **Meta** | `RESOURCE ACTION [ARGS]` | `command list`, `help`, `plugin encoding` |

#### Full Command Inventory

**show (read-only):**
```
show version
show bgp peer <name|address> detail
show bgp peer <name|address> rib
show bgp warnings
show bgp decode                  show bgp encode
show env list                    show env get <key>           show env registered
show schema list                 show schema methods          show schema events
show schema handlers             show schema protocol
show yang tree [module]          show yang completion          show yang doc
show data ls                     show data cat <key>          show data registered
show config dump                 show config diff             show config history
show config ls                   show config cat              show config fmt
show interface type <type>       show interface name <name> detail
show interface name <name> counters
```

Do not add mutating peer examples here until the exact grammar is agreed.
```

**cache/commit (named-resource, action before ID):**
```
cache list                       cache retain <id>            cache release <id>
cache expire <id>                cache forward <id> <sel>
commit list                      commit start <name>          commit end <name>
commit eor <name>                commit rollback <name>       commit show <name>
commit withdraw <name> route <prefix>
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
internal/component/<owner>/cli/      # the owner package, NOT cmd/ze
  register.go      # init() -> registry.MustRegisterRootHandler + show-shortcuts
  main.go          # Run() + dispatch + usage()
  cmd_<sub>.go     # One file per subcommand handler
```

### main.go Template

```go
package cli

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

The central `cmd/<verb>/` layout below is for **generic** cross-system verbs only.

```
internal/component/cmd/<verb>/
  <verb>.go                    # init() -> pluginserver.RegisterRPCs()
  schema/ze-cli-<verb>-cmd.yang  # CLI tree definition
```

Handler implementation lives in `internal/component/bgp/plugins/cmd/<noun>/`.

**Owner-specific commands do NOT use the central `cmd/<verb>/` layout.** Put the
handler `init()` + `RegisterRPCs` and a container-merge schema module in the
package that owns the behaviour (the one whose code the handler calls), per
`ai/rules/plugins.md` ("Finding the Owner", "How to carve a
command into its owner"). The owner schema lives in `<owner>/yang/ze-<x>-cmd.yang`
(top level, sibling of `cli`/`cmd`), re-declaring `container <verb> { container <x> {...} }`
so the loader merges it onto the verb tree with no central edit. Determine the
owner from the handler's dependencies, not from the `ze-<ns>:` WireMethod prefix
(that prefix is a label, often a legacy misnomer).

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
| `ze-show:bgp-peer` | `show bgp peer` | Yes (`RequiresSelector: true`) |
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
| Stdin/stdout | `-` means stdin (read) / stdout (write). Read/write a user-supplied path through `internal/core/cliio` (`ReadFile`/`OpenReader`/`Create`/`WriteFile`), NEVER a raw `os` call -- `./le dash-stdio check` enforces it |
| JSON output | `\| json` over a structured payload. A `--json` flag is legitimate only on a tool that reaches no pipe layer -- see `ai/rules/cli.md` "`--flag` or Keyword" |

### Flag spellings

A flag that already has a meaning here keeps it. A new flag takes a spelling no
row below uses.

| Short | Meaning | Short | Meaning |
|-------|---------|-------|---------|
| `-v` | Verbose | `-q` | Quiet |
| `-o` | Output file | `-f` | Family/file |
| `-i` | Enable feature | `-a` | Local AS |
| `-z` | Peer AS | `-n` | Dry run/count |

| Long | Meaning | Long | Meaning |
|------|---------|------|---------|
| `--dry-run` | Preview | `--socket` | Unix socket path |
| `--log-level` | Logging level | `--no-header` | Exclude headers |

A rendering flag (`--json`, `--text`, `--yaml`, `--format`, `--no-header`) is a
pipe operator's second spelling and exists on no command. Register the answer
and the pipe layer renders it.

## Command Registration (BLOCKING)

The owner package owns a `register.go` whose `init()` registers its root
command + any `show X` offline shortcuts with the importable command registry
`internal/component/command/registry`. The registry **dispatches** owner-backed
roots, so the owner can live anywhere under `internal/` without `cmd/ze`
importing it directly. **Do not** add a dispatch case to `cmd/ze/main.go`'s
static switch for an owner-backed command, and do not register from `main.go`.

### Why a dedicated leaf registry package

`internal/component/command/registry` imports only the standard library, so any
owner (`internal/component/*`, `internal/plugins/*`, `internal/core/*`) can
import it from `init()` with no import cycle. It must never import a concrete
owner, storage, the plugin server, the CLI package, or the hub package;
dispatch-time dependencies (storage, plugin list, process flags) flow through
`RuntimeContext`, whose heavy types are exposed as function values so the
package stays leaf-like.

All callers import `internal/component/command/registry` directly.

### Per-owner `register.go` template

```go
// internal/component/<owner>/cli/register.go
package cli

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
    // 1. Root command: handler + metadata. The registry dispatches it.
    registry.MustRegisterRootHandler("<name>", func(_ *registry.RuntimeContext, args []string) int {
        return Run(args)
    }, registry.Meta{
        Description: "<short one-liner>",
        Mode:        "offline",            // or "daemon", "setup", "read-only"
        Section:     registry.SectionConfiguration,
        Subs:        "<example sub-paths>", // shown in help
    })

    // 2. `show X` offline shortcuts dispatched by path.
    registry.MustRegisterLocal("show <owner> <op>", func(args []string) int {
        return Run(append([]string{"<op>"}, args...))
    })
}
```

The owner package needs its `init()` linked into the binary: until the Phase 7
generated command-provider aggregator lands, add a blank import to
`cmd/ze/main.go` (`_ "github.com/ze-software/ze/internal/component/<owner>/cli"`).

### Commands with subcommands: `subdispatch.Dispatcher`

When a root command has its own subcommands (install, uninstall, analyze, perf),
use `internal/core/subdispatch.Dispatcher` instead of a hand-rolled switch/case.
The dispatcher provides map-based lookup, derived usage text via `helpfmt`, and
typo suggestions.

See `ai/patterns/registration.md` "Subcommand Dispatch" section for the full template.

Existing examples: `cmd/ze/install/dispatch.go`, `cmd/ze/uninstall/dispatch.go`.

### Binary personalities (ze-test, ze-perf, etc.)

Binary personalities are build-tagged variants of `cmd/ze/`. Their domain code
belongs in `internal/`, not in `cmd/ze/`. The cmd/ze file is a build-tagged
blank import only:

```go
//go:build ze_analyze
package main
import _ "github.com/ze-software/ze/internal/analyze"
```

See `ai/patterns/registration.md` "Binary Personality Registration" section.

### Registry API (`internal/component/command/registry`)

| Call | Use |
|------|-----|
| `registry.RegisterRootHandler(name, handler, meta)` / `MustRegisterRootHandler` | **Owner-backed** `ze <name>`: handler + metadata, **dispatched by the registry**. Rejects empty name / nil handler / duplicate owner |
| `registry.RegisterRoot(name, meta)` | **No-owner / process-global** `ze <name>` metadata only; dispatch stays in `cmd/ze/main.go` (start, version, help, ...) |
| `registry.RootHandler` = `func(rctx *RuntimeContext, args []string) int` | Owner root handler signature; ignore `rctx` if no process deps needed |
| `registry.RuntimeContext` / `StorageAs[T](rctx)` | Process-entry deps built by `main.go` (storage resolver, plugin list, version printer, web/MCP flags). `StorageAs` type-asserts the storage value |
| `registry.RegisterLocal` / `MustRegisterLocal` / `RegisterLocalMeta` / `MustRegisterLocalMeta` | Path-keyed local handler (`"show bgp decode"`) for offline shortcuts. The handler returns an exit code and prints for itself |
| `registry.RegisterLocalData` / `MustRegisterLocalData` | Path-keyed local handler that returns DATA (`func(args []string) (any, int)`) and a renderer, normally `command.RenderLocalAnswer`. Use it whenever the command answers rows or an object, because the pipe layer then renders `\| json`, `\| yaml` and `\| table` from one payload |
| `registry.SetRuntimeStorage(fn)` / `RuntimeStorage()` | Storage resolver for local shortcuts (their `func(args)int` signature gets no context). `main.go` installs it; shortcuts read it lazily |
| `registry.LookupRoot(name)` / `LookupLocal(words)` | Dispatch lookups used by `main.go` |
| `registry.ListLocal()` / `ListRoot()` / `ListRootBySection()` | Enumerate everything; used by `help ai` |
| `registry.HasLocal` / `HasRootHandler` / `ResetForTest` | Test helpers (do not `ResetForTest` from `cmd/ze` tests -- it wipes init-registered roots; use sentinel names) |

### Registration shape per command class

| Command shape | Example | Register |
|---------------|---------|----------|
| Root `ze <name> ...` (owner-backed) | `ze bgp decode` | `MustRegisterRootHandler("bgp", wrap(Run), Meta)` in `internal/component/bgp/cli/register.go`; **registry-dispatched** |
| Root `ze <name> ...` (no-owner) | `ze start`, `ze version` | `RegisterRoot("start", Meta)` from `cmd/ze`; dispatched by `main.go` static switch (allowlisted) |
| `show X` offline shortcut | `ze show bgp decode` | `MustRegisterLocal("show bgp decode", wrapper)` in the owner package; reached via YANG tree or `LookupLocal` |
| `show X` offline shortcut answering DATA | `ze show plugins`, `ze show module list` | `MustRegisterLocalData("show plugins", handler, Meta{Mode: "offline"}, command.RenderLocalAnswer)`, plus `command.RegisterShape` and `command.RegisterColumns`, in the owner package. Template: `internal/component/plugin/register.go` |
| Online RPC | `show interface name <name> detail` | `pluginserver.RegisterRPCs(...)` in the plugin's `init()` (see Online Command section). Independent of the command registry |

### Local-data commands: what the route owes

A local-data command is served in any `ze` process with no daemon, because its
handler reads a registry that `init()` already filled. Four declarations go
together, and the last one is enforced by a gate.

| Declaration | Call | Why |
|-------------|------|-----|
| The command and its handler | `cmdregistry.MustRegisterLocalData(path, handler, meta, command.RenderLocalAnswer)` | The path MUST be a string literal at the call: `./le docvalid command-contract` parses this file and reads literals, so a `const` identifier reaches it as no path at all |
| The answer shape | `command.RegisterShape([]string{path}, command.ShapeTab)` | The published pipe catalog can then say which operators apply before the command runs |
| The column order | `command.RegisterColumns([]string{path}, command.ColumnOrder{...})` | Without it `\| table` orders columns alphabetically |
| The runtime evidence | One row in `internal/test/localdatacoverage.Evidence()`, plus an assertion in the same package's walk | `TestEveryLocalDataRegistrationHasAFunctionalCase` (`internal/component/command/registry`) derives production registrations from the Go AST and fails when one has no row. The row's command MUST carry a real pipe |

Adding a row also moves three counts that are declared beside it: the two
totals in `TestLocalDataCoverageEvidenceIsNonVacuousAndComplete`,
`localdatacoverage.CompletionMarker`, and the ordered marker list in
`test/ui/pipe-local-command.ci`.

A local-data command registers no RPC, so it owes NO `wire-methods.snapshot`
row and no `RequiresSelector`.

### Storage-dependent commands

Storage is opened only after global flag parsing, so never open it from
`init()`. The root **handler** receives the resolver through `RuntimeContext`;
**local shortcuts** (no context) read it lazily from the registry, which
`cmd/ze/main.go` installs once via `registry.SetRuntimeStorage(...)`.

```go
// internal/component/config/cli/register.go
registry.MustRegisterRootHandler("config", func(rctx *registry.RuntimeContext, args []string) int {
    store, ok := registry.StorageAs[storage.Storage](rctx)
    if !ok { /* error */ return 1 }
    defer store.Close() //nolint:errcheck
    return RunWithStorage(store, args)
}, registry.Meta{ /* ... */ })

registry.MustRegisterLocalMeta("show config history", func(args []string) int {
    store, ok := registry.RuntimeStorage().(storage.Storage)
    if !ok { /* error */ return 1 }
    defer store.Close() //nolint:errcheck
    return RunWithStorage(store, append([]string{"history"}, args...))
}, registry.Meta{Description: "..."})
```

### How `help ai` consumes the registry

`cmd/ze/help_ai.go:cliSubcommands()` enumerates:

1. YANG verb subtree (show, set, del, update, ...).
2. `registry.ListRoot()` for top-level subcommands.

De-dupe on root-name collisions. No static list. Adding a new
subcommand package means adding its `register.go`; help picks it up
automatically.

## Reference Implementations

| Variant | File | Notes |
|---------|------|-------|
| Switch dispatch | `internal/component/bgp/cli/main.go` | Standard pattern |
| Map dispatch | `internal/component/config/cli/main.go` | Many subcommands, storage-aware |
| Map dispatch (simple) | `internal/component/config/storage/cli/main.go` | Stateless subcommands (`ze data`) |
| **Root handler registration** | `internal/component/bgp/cli/register.go` | Canonical owner `register.go` (RegisterRootHandler + `show` shortcuts) |
| **Root + owner schema** | `internal/component/bgp/cli/register.go` + `internal/component/bgp/cli/yang/` | Owner-owned YANG tools schema, blank-imported by the owner |
| **Storage-bound** | `internal/component/config/cli/register.go` | `StorageAs` (root) + `RuntimeStorage` (local shortcuts) |
| **No-owner / process-global** | `internal/plugins/skills/register.go` | `RegisterRoot` metadata + `MustRegisterLocalMeta` for commands with no component owner |
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
[ ] Owner package: register.go in internal/component/<owner>/cli (NOT cmd/ze) -- owner is cmd/ze-free
[ ] register.go: registry.MustRegisterRootHandler(<name>, wrap(Run), Meta{...}) for `ze <name>` (registry-dispatched)
[ ] register.go: registry.MustRegisterLocal(<path>, handler) for every `show X` shortcut
[ ] Owner init() linked: blank import in cmd/ze/main.go (until the Phase 7 generated aggregator)
[ ] If storage-dependent: root handler uses StorageAs(rctx); local shortcuts use registry.RuntimeStorage()
[ ] No-owner / process-global only: stays in cmd/ze with RegisterRoot + main.go switch (allowlist)
[ ] If online: YANG tree with ze:command extension
[ ] If online: WireMethod in kebab-case matching YANG
[ ] If online: RequiresSelector set correctly
[ ] Functional tests (test/parse/ for offline, test/plugin/ for online)
[ ] If local-data: MustRegisterLocalData + RegisterShape + RegisterColumns
[ ] If local-data: an Evidence row and a walk assertion in internal/test/localdatacoverage,
    and the three counts that move with it (see "Local-data commands: what the route owes")
```
