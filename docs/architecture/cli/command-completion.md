# Plugin Commands in Interactive Completion

The interactive completion tree was built from YANG only. The commands that
plugins register through the plugin `CommandRegistry` worked when typed and were
invisible to Tab. The registry already carried `Hidden` and `Complete()`. The
gap was that the interactive tree never consulted it.

## The producers

| Surface | Producer | How it stays live |
|---------|----------|-------------------|
| entry source | `VisibleCommandEntries()` returns every non-`Hidden` command as a `command.CommandEntry` | RLock over the registry map, mirroring `All()` and `Complete()` |
| injection | `MergeCommandPaths` splits each command name on spaces and inserts tree nodes | non-destructive: an existing node is never mutated. Each of the two help fields is set only on a leaf the call creates, or on one that holds nothing in THAT field |
| SSH | `mergePluginCommands` merges into the per-session tree | the tree is rebuilt per session, so a plugin that exited is absent next session |
| Web | `pluginAwareCommandCompleter` builds a throwaway overlay per `/cli/complete` request and composites it, YANG winning a name collision | the shared YANG tree stays immutable and is never mutated |

<!-- source: internal/component/plugin/server/command_registry.go -- VisibleCommandEntries -->
<!-- source: internal/component/command/node.go -- MergeCommandPaths -->
<!-- source: internal/component/cli/client/inject.go -- injectPluginCommands, the client-side tree -->

The `ze cli` client runs the same rule against the command list it fetches from
the daemon: `injectPluginCommands` skips a hidden command, and skips a path that
already exists in the tree from a YANG proxy RPC.

A plugin command can never overwrite a builtin's `WireMethod`, its summary, or
its long help. `MergeCommandPaths` writes each of the two help fields only on a
leaf it creates, or on one whose own copy of that field is empty. The completer
offers a node on name prefix and `backendAllowed` alone and never reads
`WireMethod`, so a completion-only node surfaces.

A `command.CommandEntry` carries both fields. `Description` is the one-line
summary and `Help` is the long explanation the command's own help page prints.
They are decided one at a time, so a plugin that states a summary and no
explanation fills the summary alone.

## Why web sources at request time

`runYANGConfig` waits for plugin startup before `buildServices` constructs and
binds the optional looking-glass, web, and MCP management services.
`signalStartupComplete` freezes the dispatcher command registry before
`WaitForStartupComplete` returns. The MCP `tools/list` endpoint therefore cannot <!-- doc-links: ignore (JSON-RPC method name, not a repository path) -->
answer before the initial registry freeze.

Web still builds a live per-request overlay so plugins that a reload adds or
removes are visible. A one-time build snapshot would become stale after a reload.
SSH has the same liveness because it rebuilds its whole tree per session.

<!-- source: cmd/ze/hub/main.go -- runYANGConfig -->
<!-- source: internal/component/plugin/server/startup.go -- signalStartupComplete, WaitForStartupComplete -->

## The three completion paths are distinct

1. The interactive SSH and web tree. This is the path plugin commands were
   missing from.
2. `ze completion words`. A standalone CLI process with no daemon. It stays
   YANG-only and cannot see live plugin commands.
3. The daemon's `system command complete` RPC. It already completed plugin
   commands through `Registry().Complete()`.
