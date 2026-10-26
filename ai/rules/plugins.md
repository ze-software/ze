# Plugins

**When:** creating or changing a plugin: its registration, placement, transport, command surface, process boundary, dispatch table, or a feature gate
**Severity:** blocking
**Related:** repo-maintenance, cli, evidence

## Directives

- **The mechanism behind every directive here is documented, and the page MUST be read before the plugin work it covers:** `docs/architecture/plugin/plugin-system.md` for registration, the engine boundary, the communication patterns, `OnStarted` against `OnAllPluginsReady`, role claims and the peer-up barrier; `docs/architecture/api/commands.md` for answer-shape and pipe-alias declaration; `docs/architecture/plugin/feature-gates.md` for compile-out; `docs/architecture/command-ownership.md` for command placement; `docs/architecture/api/process-protocol.md` for the wire protocol and the accumulator arity; `ai/patterns/plugin.md` for the file template and the new-plugin checklist; and `pkg/plugin/rpc/bridge.go` before any new core-to-plugin plumbing, because DirectBridge carries request and response where the EventBus MUST NOT.

**A plugin MUST own its entire feature surface. Removing the plugin MUST make every one of its features disappear; every other plugin and the core MUST keep working.**

- **Every RPC MUST carry a YANG registration for the CLI, whether it is registered through `registry.Register()` or through `pluginserver.RegisterRPCs()`.** A command handler with no YANG schema is a structural defect to fix, not a different category. There is no "command module": everything with RPCs is a plugin and lives under `plugins/<name>/`.

- **A payload that crosses a plugin or component boundary MUST be a self-contained value type.** It carries no pointer field into data another plugin or component owns, and a shared core package is no exception. The surface-by-surface list is `docs/architecture/plugin/plugin-system.md`, "Cross-boundary value types".

- **A text a plugin declares that reaches an operator MUST be bounded, and MUST be refused when it carries a control character its shape does not allow.** The check runs at Stage 1, before the declaration is stored, because a stored declaration is live for every operator. A one-line text (a command's `short-help`, a pipe alias's `description`) refuses every control character: it is written into the tab-separated shell-completion format and into the one-line terminal candidate, so a newline or a tab breaks the format for every row that follows and an ESC writes an ANSI sequence to the terminal. A paragraph (a command's `description`) keeps its newlines and refuses the rest. `validateHelpDecls` and `validateDeclaredText` (`internal/component/plugin/server/startup.go`) are where the next declared text joins them.

- **A plugin that declares commands, pipes or families MUST be answerable without activating: its declaration MUST be a pure value, and every side effect MUST sit inside the function that a declaration query never calls.** A process started with `ZE_PLUGIN_MODE=declare` writes its Stage 1 `declare-registration` line to stdout and exits 0, so it MUST initialize no data, open no connection, bind no socket, start no listener and start no timer. A plugin binary ze does not carry gets that guarantee only by handing its declaration and its start function to `sdk.RunOrDeclare` as two arguments (`pkg/plugin/sdk/sdk_query.go`); work its `main` does above that call still runs under a query. A plugin the ze binary carries is answered from the compiled-in registration and runs no plugin code at all (`internal/component/plugin/cli.Run`). `docs/plugin-development/protocol.md`, "Query Mode", is the author's route.

## Registration-Based Dispatch

**MUST NOT use switch/case to dispatch subcommands.** All command dispatch MUST use the registration pattern: register handlers into a dispatcher (or sub-dispatcher), then call `Dispatch(args)`. This applies at every level of nesting.

## Feature-Gate Registration

- **A plugin that consumes what only a plugin under another build gate produces MUST declare an edge to that producer, and the edge kind MUST follow whether config makes the consumer depend on it.** A dependency MAY be soft: a build without the provider still runs the owner and refuses only the config that needs it. When a setting makes the owner depend on the provider's output, the provider MUST go in `OptionalDependencies` with a `ConfigNeeds` entry naming that setting, so a build without it refuses the setting with the build tag that would carry it and a full build pulls the provider only when the setting holds. When the owner only listens and works without it, the provider MUST go in `StartAfter`, which orders without loading. A plugin that cannot work at all without another MUST declare it in `Dependencies`, and a build missing it refuses on every load path, naming the tag. An allowlist MUST NOT stand in for an edge. `./le plugin coupling check` refuses an undeclared coupling, and `docs/architecture/plugin/feature-gates.md`, "Declaring a cross-gate coupling", holds the mechanism.
