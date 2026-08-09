# Plugin Commands in Interactive Completion

The interactive completion tree was built from YANG only. The commands that
plugins register through the plugin `CommandRegistry` worked when typed and were
invisible to Tab. The registry already carried `Hidden` and `Complete()`. The
gap was that the interactive tree never consulted it.

## The producers

| Surface | Producer | How it stays live |
|---------|----------|-------------------|
| entry source | `VisibleCommandEntries()` returns every non-`Hidden` command as a `command.CommandEntry` | RLock over the registry map, mirroring `All()` and `Complete()` |
| injection | `MergeCommandPaths` splits each command name on spaces and inserts tree nodes | non-destructive: an existing node is never mutated, and `Description` is set only on a leaf the call creates |
| SSH | `mergePluginCommands` merges into the per-session tree | the tree is rebuilt per session, so a plugin that exited is absent next session |
| Web | `pluginAwareCommandCompleter` builds a throwaway overlay per `/cli/complete` request and composites it, YANG winning a name collision | the shared YANG tree stays immutable and is never mutated |

<!-- source: internal/component/plugin/server/command_registry.go -- VisibleCommandEntries -->
<!-- source: internal/component/command/node.go -- MergeCommandPaths -->
<!-- source: internal/component/cli/client/inject.go -- injectPluginCommands, the client-side tree -->

The `ze cli` client runs the same rule against the command list it fetches from
the daemon: `injectPluginCommands` skips a hidden command, and skips a path that
already exists in the tree from a YANG proxy RPC.

A plugin command can never overwrite a builtin's `WireMethod` or description,
because `MergeCommandPaths` writes a description only on a leaf it creates. The
completer offers a node on name prefix and `backendAllowed` alone and never
reads `WireMethod`, so a completion-only node surfaces.

## Why web sources at request time

`buildServices` runs before `WaitForStartupComplete`, so plugins have not
registered their commands when the web server builds its tree. A build-time
snapshot is empty. A `sync.Once` snapshot taken on the first request is not
empty and never reflects a later register or unregister. The live per-request
overlay handles the startup race and hot reload with one mechanism. SSH gets the
same liveness for free, because it rebuilds its whole tree per session and every
session starts after startup completes.

## The three completion paths are distinct

1. The interactive SSH and web tree. This is the path plugin commands were
   missing from.
2. `ze completion words`. A standalone CLI process with no daemon. It stays
   YANG-only and cannot see live plugin commands.
3. The daemon's `system command complete` RPC. It already completed plugin
   commands through `Registry().Complete()`.
