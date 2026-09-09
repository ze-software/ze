---
kind: directive
level: MUST
stage:
---
- **A plugin that DECLARES commands, pipes or families MUST be answerable without activating: its declaration MUST be a pure value, and every side effect MUST sit inside the function that a declaration query never calls.** A process started with `ZE_PLUGIN_MODE=declare` writes its Stage 1 `declare-registration` line to stdout and exits 0, so it MUST initialize no data, open no connection, bind no socket, start no listener and start no timer. A plugin binary ze does not carry gets that guarantee only by handing its declaration and its start function to `sdk.RunOrDeclare` as two arguments (`pkg/plugin/sdk/sdk_query.go`); work its `main` does above that call still runs under a query. A plugin the ze binary carries is answered from the compiled-in registration and runs no plugin code at all (`internal/component/plugin/cli.Run`). `docs/plugin-development/protocol.md`, "Query Mode", is the author's route.
