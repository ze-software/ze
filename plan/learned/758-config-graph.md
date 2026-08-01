# 758: Config Dependency Graph for Agent Impact Analysis

## Context
AI agents editing config sections had no way to understand which other sections would be affected. A change to a peer-group could affect all peers inheriting from it; a listener change could conflict with another service's port.

## Decision
Added `ze config graph` that outputs a JSON dependency graph derived from the same code paths validation uses. Graph has typed nodes (section, peer, group, plugin, profile, user, listener) and typed edges (contains, inherits, references, config-root, listens-on, depends-on, process-binds).

Key choices:
- **Derived from validation, not declared**: the graph walks `config.Tree` and `config.Schema` using the same lookups that commit validation uses. No separate dependency declaration to keep in sync.
- **Seven edge kinds**: `contains` (section nesting), `inherits` (BGP peer -> group), `references` (authz user -> profile), `config-root` (plugin registry entry), `listens-on` (service -> endpoint), `depends-on` (plugin declared deps), `process-binds` (process -> plugin).
- **Plugin registry integration**: `addPluginRegistryEdges` reads config roots and dependencies from the live plugin registry, so dynamically loaded plugins appear in the graph.
- **Sorted output**: nodes and edges sorted for deterministic JSON, enabling diff-based testing.
- **Offline command**: reads config file, does not require running daemon.

## Consequences
- Agents can query impact radius before proposing config edits.
- 638 new lines: graph builder (353), CLI command (72), tests (208).
- Registered via `cmd/ze/config/register.go` alongside existing config subcommands.

## Gotchas
- Listener nodes extract addresses from the same schema annotation used by `ValidateListenerConflicts`, not from config values directly. If the schema annotation is missing, the listener won't appear in the graph.
- Plugin config roots are discovered via the global `registry.All()` iterator. Plugins that register after graph construction (theoretically possible but not currently done) would be missing.

## Files

None recorded.
