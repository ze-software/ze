# 1115 — command-completion

Plugin-registered commands now appear in **interactive** CLI tab-completion
(SSH and web). Before, the completion tree was built from YANG only, so the ~51
commands that plugins register via the plugin `CommandRegistry` worked when typed
but were invisible to Tab. The registry already carried `Hidden` (on both
`CommandDef` and `CommandDecl`) and a `Complete()` method — the only gap was that
the interactive tree never consulted the registry.

Producers (all read/verified first-hand at implementation):
- **Source of entries:** `internal/component/plugin/server/command_registry.go`
  `VisibleCommandEntries()` returns every non-`Hidden` command as a
  `command.CommandEntry{Name, Description}` (RLock over `r.commands`, mirroring
  `All()`/`Complete()`).
- **Injection primitive:** `internal/component/command/node.go` `MergeCommandPaths`
  splits each command name on spaces and inserts nodes into the tree. It is
  **non-destructive**: an existing node (a YANG-backed command or grouping node)
  is never mutated — `Description` is set only on a leaf this call creates, so a
  plugin command can never overwrite a builtin's `WireMethod`/description. The
  completer offers a node purely on name/prefix + `backendAllowed` — it never
  reads `WireMethod` (`completer.go:262-273`), so completion-only nodes surface.
- **SSH:** `cmd/ze/hub/session_factory.go` `mergePluginCommands` merges eagerly
  into the per-session tree (`buildCommandTree()` is called inside the per-session
  factory closure), sourcing entries lazily via
  `params.APIServer().Dispatcher().Registry()`. Each session reflects the current
  registry, so a plugin that has exited is simply absent next session (AC-3).
- **Web:** `cmd/ze/hub/web_completer.go` `pluginAwareCommandCompleter` overlays
  plugin commands **live on every `/cli/complete` request**: the YANG tree stays
  immutable, and a throwaway overlay tree is built per request from the current
  registry, then composited (YANG wins on name collision). The source is a
  `func() []command.CommandEntry` closure built in `main.go` from
  `apiServer.Dispatcher().Registry().VisibleCommandEntries()`, threaded through
  `ServiceDeps.WebCommands` → `startWebServer` (mirrors the MCP
  `commandMetaSource(apiServer)` precedent).

## GOTCHAS
- **Web must source at request time, not build time.** `buildServices`
  (main.go:729) runs BEFORE `apiServer.WaitForStartupComplete` (main.go:918), so
  plugins have not registered their commands when `startWebServer` builds the
  tree. A build-time snapshot would be empty, and a first-request `sync.Once`
  snapshot would then never reflect a later register/unregister (AC-3). The live
  per-request overlay handles both the startup race (R-2) and hot
  reload/unregister (R-1: "clear and rebuild"). SSH gets liveness for free by
  rebuilding its whole tree per session (always post-start).
- **The three completion paths are distinct.** (1) interactive SSH/web tree
  (this change); (2) `ze completion words` — a standalone CLI process with no
  daemon, so it stays YANG-only and cannot see live plugin commands; (3) the
  daemon's `system command complete` RPC — already completed plugin commands via
  `Registry().Complete()` (`system.go:436`). Only (1) was missing plugin commands.
- **Web is race-free by never mutating the shared tree.** The YANG `commandTree`
  is read-only after build (`AdminTreeFromYANG` snapshots it at setup); each
  request builds its OWN small overlay tree, so there is no shared mutable state.
  (An earlier `sync.Once`-mutate-in-place design was race-free too but snapshotted
  the registry at first request, failing AC-3 for web — the live overlay replaced
  it.)
- Landed alongside a concurrent netlink/firewall session's uncommitted files in
  the shared working tree — the commit staged only the command-completion files.

## Files

None recorded.
